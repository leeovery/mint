package engine

// This file is the engine's release ORCHESTRATOR — the Phase 1 walking-skeleton
// spine that threads every shipped unit (gitrepo, config, version, preflight,
// record, release, publish) into one runnable `mint release`. It owns ORDERING:
// the sequence in which the units run, when the presenter events fire, and the
// load-bearing point-of-no-return (PONR) asymmetry. The units themselves are
// unchanged — Release CALLS them; it never re-implements their logic.
//
// PONR ASYMMETRY (load-bearing): every failure in stages 1–8 BEFORE the atomic
// push succeeds aborts the run (surfaced via the presenter, non-zero exit) with
// nothing published. A publish failure AFTER a successful push is WARN-ONLY — the
// tag is already public, so the run does not unwind; it warns and exits 0. The
// full surgical auto-unwind is a later phase; Phase 1 surfaces and stops.

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"mint/internal/config"
	"mint/internal/gitrepo"
	"mint/internal/preflight"
	"mint/internal/presenter"
	"mint/internal/publish"
	"mint/internal/record"
	"mint/internal/release"
	"mint/internal/runner"
	"mint/internal/version"
)

// releaseAction is the engine-supplied verb word for the start-of-run header.
const releaseAction = "releasing"

// Editor is the engine's edit seam: the `e` review-gate choice hands the current
// notes body to Edit and uses whatever it returns VERBATIM (no re-parse, no
// re-validation — a human edit is trusted). The interface is intentionally tiny so
// production can wire the real $EDITOR resolution (task 2-13) behind it while the
// gate-loop tests drive a scripted fake. It is defined HERE (the consumer) rather
// than where it is implemented, per the accept-interfaces convention.
type Editor interface {
	// Edit presents the current body to the user's editor and returns the saved
	// text. A non-nil error means no edit could be obtained (e.g. no editor on PATH);
	// the engine surfaces it and aborts rather than blocking on an unwired editor.
	Edit(ctx context.Context, current string) (string, error)
}

// ReleaseDeps bundles the orchestrator's injected seams so production wires the
// real implementations once (at the cmd entry point) and tests drive the whole
// spine with a RecordingPresenter + a single FakeRunner. The Releaser and
// Publisher each already hold the same runner; bundling them here keeps Release's
// signature short and the wiring explicit.
type ReleaseDeps struct {
	// Presenter is mint's single output/gate seam. Release emits lifecycle events
	// through it and never touches stdout/TTY directly.
	Presenter presenter.Presenter
	// Runner is the external-command seam the read-side units (gitrepo, version,
	// preflight, record) issue git through.
	Runner runner.CommandRunner
	// Releaser performs the point-of-no-return tag + atomic push.
	Releaser *release.Releaser
	// Publisher publishes the provider release after the push crosses the PONR.
	Publisher publish.Publisher
	// Editor is the OPTIONAL edit seam consulted ONLY on the `e` review-gate choice.
	// Runs that never reach `e` (every non-interactive and accept/abort path) may
	// leave it nil; production wires the real $EDITOR resolution (task 2-13). If `e`
	// is chosen with a nil Editor (a misconfiguration), the gate surfaces a clear
	// error and aborts rather than panicking.
	Editor Editor
}

// ReleaseOptions carries the per-run parsed inputs. Bump selects the version
// segment to increment; Now is the injected release date (the production caller
// passes time.Now(), tests pass a fixed time) so the changelog header is
// deterministic — Release never calls time.Now() itself.
type ReleaseOptions struct {
	// Bump selects which version segment Next increments (default BumpPatch).
	Bump version.Bump
	// Now is the injected release timestamp used for the changelog date.
	Now time.Time
	// NotesBody is the SELECTED notes body to distribute to every sink — the
	// Phase-2 seam the SelectBody wiring (task 2-16) fills. Empty means "no
	// override": Release falls back to the Phase-1 first-release default body,
	// preserving current behaviour. Whatever value resolves, it flows WHOLE to the
	// tag annotation, the CHANGELOG section, and the provider release — no parsing,
	// no splitting, no per-sink reassembly.
	NotesBody string
}

// Release runs the Phase 1 first-release spine in strict order and returns nil on
// success. Any pre-push failure returns an *AbortError carrying a non-zero exit
// code (the failure is surfaced through the presenter first). A publish failure
// AFTER a successful push is warn-only — it surfaces a Warn and returns nil,
// because the tag is already public and mint never unwinds post-PONR.
func Release(ctx context.Context, deps ReleaseDeps, opts ReleaseOptions) error {
	p := deps.Presenter

	// Stage 1 — resolve root, load config, resolve branch, compute the tag.
	root, err := gitrepo.ResolveRoot(ctx, deps.Runner)
	if err != nil {
		return surface(p, "preflight", err)
	}

	cfg, err := config.Load(root)
	if err != nil {
		return surface(p, "config", err)
	}

	releaseBranch, err := gitrepo.ResolveReleaseBranch(ctx, deps.Runner, cfg)
	if err != nil {
		return surface(p, "preflight", err)
	}

	current, err := version.CurrentVersion(ctx, deps.Runner, cfg.Release.TagPrefix)
	if err != nil {
		return surface(p, "version", err)
	}
	next := version.Next(current, opts.Bump)
	tag := next.String(cfg.Release.TagPrefix)

	// Stage 2 — preflight. Fetch first, then cheap local gates, then network gates.
	if err := runPreflight(ctx, deps.Runner, releaseBranch, tag); err != nil {
		return surface(p, "preflight", err)
	}

	// Stage 4 — notes body. The injected NotesBody is the selected body (Phase-2
	// SelectBody seam); an empty override falls back to the Phase-1 first-release
	// fixed no-AI body, preserving current behaviour. Whatever resolves here flows
	// WHOLE to every active sink below — no parsing, no splitting, no per-sink
	// reassembly.
	body := opts.NotesBody
	if body == "" {
		body = record.FirstReleaseBody
	}
	versionKey := next.String("")

	// Emit in SPEC ORDER: RunStarted, ShowPlan, ShowNotes — then the review gate.
	p.RunStarted(presenter.RunInfo{
		Project: projectName(root),
		Version: versionKey,
		Action:  releaseAction,
		Leaf:    cfg.Release.CommitPrefix,
	})
	p.ShowPlan(buildPlan(tag, cfg.Release.Publish))
	p.ShowNotes(presenter.Notes{Version: versionKey, Body: body})

	// The review gate may EDIT the body (the `e` choice); capture the returned
	// final body and thread IT to every downstream sink. The gate stays positioned
	// BEFORE any mutation (before Record).
	body, err = reviewGate(ctx, p, deps.Editor, body, versionKey)
	if err != nil {
		return err
	}

	// Stage 5 — record: write changelog (gated by the changelog toggle), then the
	// bookkeeping commit. When changelog=false WriteChangelog no-ops (Changed:false)
	// so CommitBookkeeping skips the commit — the tag then points at the existing
	// HEAD and STILL carries the full body via TagAndPush. Nothing durable is lost.
	writeResult, err := record.WriteChangelog(root, versionKey, opts.Now, body, cfg.Release.Changelog)
	if err != nil {
		return surface(p, "record", err)
	}
	if err := record.CommitBookkeeping(ctx, deps.Runner, root, cfg.Release.CommitPrefix, tag, writeResult.Changed); err != nil {
		return surface(p, "record", err)
	}

	// Stage 2 (conditional gate 6) — gh auth, only when publishing, BEFORE the tag.
	if cfg.Release.Publish {
		if err := preflight.CheckGhAuth(ctx, deps.Runner); err != nil {
			return surface(p, "preflight", err)
		}
	}

	// Stage 6 — tag + atomic push. A nil error means the atomic push succeeded and
	// PointOfNoReturnCrossed is set: from here the tag is public, so any later
	// failure is warn-only and the run must NOT unwind.
	if _, err := deps.Releaser.TagAndPush(ctx, tag, cfg.Release.CommitPrefix, body); err != nil {
		return surface(p, "tag", err)
	}

	// Stage 7 — publish. Post-PONR: a publish failure is WARN-ONLY (the tag is
	// already public); the run does not unwind and exits successfully.
	releaseURL := ""
	if cfg.Release.Publish {
		if err := deps.Publisher.CreateRelease(ctx, tag, tag, body); err != nil {
			warnPublishFailed(p, err)
		}
	}

	p.RunFinished(presenter.RunResult{
		Project: projectName(root),
		Version: versionKey,
		URL:     releaseURL,
		Leaf:    cfg.Release.CommitPrefix,
	})
	return nil
}

// errGateAborted is the cause for a clean gate-no abort: the user declined the
// review gate, so the run stops with a non-zero exit but no underlying failure.
var errGateAborted = errors.New("release aborted at the review gate")

// runPreflight runs the Stage 2 gate chain in the spec's order: fetch (read-only,
// refreshes tags + upstream refs), then the cheap local gates, then the network
// gates. The first failure short-circuits and is returned for the caller to
// surface and abort on. The conditional gh gate is run separately by Release
// (only when publishing, after the bookkeeping commit and before the tag).
func runPreflight(ctx context.Context, r runner.CommandRunner, releaseBranch, tag string) error {
	if err := preflight.Fetch(ctx, r); err != nil {
		return err
	}
	if err := preflight.RunLocalGates(ctx, r, releaseBranch, tag); err != nil {
		return err
	}
	if err := preflight.CheckRemoteSync(ctx, r, releaseBranch); err != nil {
		return err
	}
	return preflight.CheckTagFreeRemote(ctx, r, tag)
}

// buildPlan assembles the Phase 1 plan steps in execution order — commit, tag,
// push, and publish when publishing. The presenter renders these verbatim; the
// engine owns the verbs and targets.
func buildPlan(tag string, publish bool) presenter.Plan {
	steps := []presenter.PlanStep{
		{Verb: "commit", Target: "bookkeeping"},
		{Verb: "tag", Target: tag},
		{Verb: "push", Target: "--atomic → origin"},
	}
	if publish {
		steps = append(steps, presenter.PlanStep{Verb: "publish", Target: tag})
	}
	return presenter.Plan{Steps: steps}
}

// errEditorUnavailable is the cause surfaced when the `e` choice is taken but no
// Editor seam was wired — a misconfiguration that should never reach production
// (task 2-13 wires the editor). It is surfaced rather than panicked so the spine
// fails loud and clean before any mutation.
var errEditorUnavailable = errors.New("edit chosen but no editor is configured")

// reviewGate runs the first-release y/n/e review gate as the engine-owned
// re-entry LOOP and returns the (possibly edited) FINAL body the caller threads
// to every sink. Rendering stays in the presenter; this owns only the semantics.
//
// Each pass reads one decision and acts on it:
//
//   - a Prompt error (ErrNotInteractive / ErrInputClosed) is already an
//     *AbortError carrying a non-zero exit; it is returned as-is (ErrNotInteractive
//     is pre-rendered by the presenter, ErrInputClosed is surfaced via the abort).
//   - ChoiceYes (also the bare-Enter default) accepts: the loop RETURNS the current
//     body so the spine proceeds to Record with exactly the reviewed text.
//   - ChoiceNo aborts: Phase 2 surfaces an Unwound and stops BEFORE any mutation
//     (the full surgical auto-unwind lands in task 2-15); the run exits non-zero.
//   - ChoiceEdit edits: the editor seam returns the saved text, which REPLACES the
//     body VERBATIM — no re-parse, no re-validation (a human edit is trusted;
//     structural validation only ever guards untrusted AI output). The notes are
//     re-shown and the loop re-prompts. An editor-seam failure (including a missing
//     editor) is surfaced and aborts rather than blocking the spine.
func reviewGate(ctx context.Context, p presenter.Presenter, editor Editor, body, versionKey string) (string, error) {
	for {
		choice, err := ReviewDecision(p, FirstReleaseReviewGate())
		if err != nil {
			return "", err
		}

		switch choice {
		case presenter.ChoiceYes:
			// Accept (also the bare-Enter default): proceed with the reviewed body.
			return body, nil
		case presenter.ChoiceNo:
			// Abort on gate-no: surface and stop (full surgical unwind is task 2-15).
			p.Unwound(presenter.Unwind{Summary: "release aborted at review gate; repo clean"})
			return "", abort(errGateAborted)
		case presenter.ChoiceEdit:
			edited, eerr := editBody(ctx, editor, body)
			if eerr != nil {
				return "", surface(p, "edit", eerr)
			}
			// Use the edited text VERBATIM — no re-parse, no re-validation — then
			// re-render the notes and loop back to re-prompt.
			body = edited
			p.ShowNotes(presenter.Notes{Version: versionKey, Body: body})
		default:
			// The gate declares only y/n/e and the presenter returns a member of the
			// declared set; any other choice is a contract violation. Fail loud rather
			// than spin the loop forever (r regenerate is task 2-14, not handled here).
			return "", surface(p, "review", errUnexpectedChoice(choice))
		}
	}
}

// errUnexpectedChoice builds the cause for a review-gate choice outside the gate's
// declared y/n/e set — a presenter-contract violation the loop refuses to ignore.
func errUnexpectedChoice(choice presenter.Choice) error {
	return fmt.Errorf("unexpected review-gate choice %q", choice)
}

// editBody obtains the edited notes from the editor seam. A nil editor on the `e`
// path is a misconfiguration (task 2-13 wires the real one); rather than
// panicking it returns errEditorUnavailable so the gate surfaces a clear failure.
func editBody(ctx context.Context, editor Editor, current string) (string, error) {
	if editor == nil {
		return "", errEditorUnavailable
	}
	return editor.Edit(ctx, current)
}

// warnPublishFailed emits the post-PONR warn-only event: by the time it runs the
// push has already crossed the point of no return, so the tag is public and mint
// must NOT unwind. It warns and points at the heal path; the run still finishes
// successfully.
func warnPublishFailed(p presenter.Presenter, cause error) {
	p.Warn(presenter.Warning{
		Label:   "publish failed",
		Message: "tag is already published; heal with regenerate --reuse",
		Output:  cause.Error(),
	})
}

// surface renders a stage failure through the presenter and returns the engine's
// typed abort carrying a non-zero exit code. It is the single pre-PONR failure
// path: every stage-1–8 error flows through here so the failure is both shown and
// terminal. A *GateError surfaces its actionable Message; any other error
// surfaces its Error() text.
func surface(p presenter.Presenter, stage string, cause error) error {
	p.StageFailed(presenter.StageFailure{
		Name:    stage,
		Message: failureMessage(cause),
	})
	return abort(cause)
}

// failureMessage extracts the display message for a stage failure: a preflight
// *GateError carries an actionable, display-ready Message; everything else falls
// back to the wrapped error text.
func failureMessage(cause error) string {
	var gate *preflight.GateError
	if errors.As(cause, &gate) {
		return gate.Message()
	}
	return cause.Error()
}

// projectName derives the project label from the repo root's final path segment.
// Phase 1 has no configured project name, so the directory basename is the natural
// stand-in for the brand/header lines.
func projectName(root string) string {
	return filepath.Base(root)
}
