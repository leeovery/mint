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
// tag is already public, so the run does not unwind; it warns and exits 0.
//
// AUTO-UNWIND (Phase 2): a gate-no user abort and any POST-MUTATION pre-PONR failure
// share ONE clean-reset path (unwind): mint resets whatever it made this run back to
// the clean starting state captured before the gate (startingHEAD) and narrates an
// engine-authored Unwound. The two are deliberately identical — a declined gate and
// a rejected push reset the same way. The PRE-mutation / preflight failures (before
// startingHEAD is captured) stay plain surface; there is nothing to unwind. Phase 4
// adds surgical N-commit counting, lock-resilient git wrapping, and --autostash
// ordering on top of this spine reset.

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/gitrepo"
	"mint/internal/hooks"
	"mint/internal/notes"
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

// Regenerator is the engine's regenerate seam: the `r` review-gate choice hands a
// ONE-TIME context line to Regenerate and uses the regenerated body it returns as
// the new notes body. The one-time context is TRANSIENT — it flows into this one
// AI call only and is NEVER persisted to [release].context. The interface is
// intentionally tiny so production (task 2-16) can wire the real AI path
// (notes.Generator.GenerateWithContext over the run's lastTag/cfg) behind it while
// the gate-loop tests drive a scripted fake. It is defined HERE (the consumer)
// rather than where it is implemented, per the accept-interfaces convention.
//
// It is consulted ONLY on `r`, which the gate offers ONLY for notes.KindNormalAI
// (the one path the AI produced the body), so the no-AI paths never reach it and
// may leave the seam nil.
type Regenerator interface {
	// Regenerate re-runs the normal AI path with oneTimeContext appended to the
	// prompt and returns the new body. An empty oneTimeContext is legal (regenerate
	// with no extra context). A non-nil error means the regeneration failed; the
	// engine surfaces it and aborts (fail-loud) rather than looping on a broken AI.
	Regenerate(ctx context.Context, oneTimeContext string) (string, error)
}

// regenContextPrompt is the AskLine label shown when the user chooses `r`: it asks
// for the one-time context nudge appended to the regeneration prompt. An empty
// answer is legal (regenerate with no extra context).
const regenContextPrompt = "Add one-time context for regeneration (optional):"

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
	// Regenerator is the OPTIONAL regenerate seam consulted ONLY on the `r`
	// review-gate choice, which the gate offers ONLY for notes.KindNormalAI. Every
	// other path (the no-AI Kinds, every non-interactive and accept/abort path) may
	// leave it nil. In production it is left nil: Release builds a PER-RUN regenerator
	// closure that binds the run's lastTag + cfg to the resolved Generator (see
	// resolveBody). When non-nil it OVERRIDES that closure — the test-injection seam
	// the gate-loop `r` tests drive with a scripted fake. If `r` is chosen with neither
	// a wired Regenerator nor a Generator (a misconfiguration), the gate surfaces a
	// clear error and aborts rather than panicking.
	Regenerator Regenerator
	// Transport is the OPTIONAL AI transport seam the notes Generator hands its
	// composed prompt to. It exists so the prior-tag end-to-end tests can drive the
	// REAL notes path over the FakeRunner while still injecting a recording transport
	// where they need to script the AI body directly. When nil, Release builds the
	// production ai.Transport (default `claude -p`) over deps.Runner once root is
	// resolved — so production leaves it nil and gets the real transport. Wiring the
	// ai_command / timeout config OVERRIDE is deferred to the Phase 6 schema; the
	// zero-Config default is used now.
	Transport notes.Transport
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
	// NotesBody is a TEST-INJECTION OVERRIDE for the resolved notes body. In
	// PRODUCTION it is empty: Release resolves the body via the notes-path precedence
	// (SelectBody). When NON-EMPTY it bypasses the selector and is used verbatim
	// alongside NotesKind — the seam the body-distribution / gate-loop tests drive to
	// pin a specific body without scripting the whole notes engine. Whatever value
	// resolves (override or selector), it flows WHOLE to the tag annotation, the
	// CHANGELOG section, and the provider release — no parsing, no splitting, no
	// per-sink reassembly.
	NotesBody string
	// NotesKind names which precedence path produced NotesBody — used ONLY alongside
	// the NotesBody test-injection override. When NotesBody is empty (production),
	// Release ignores this and uses the Kind SelectBody returns. It selects the review
	// gate variant: notes.KindNormalAI offers the four-choice y/n/e/r gate (the only
	// path with an AI to regenerate), while EVERY other Kind (first-release,
	// degenerate, --no-ai, fallback) offers the y/n/e variant with no `r`.
	NotesKind notes.Kind
	// NoAI is the --no-ai flag: a DELIBERATE skip of the AI path (after the
	// first-release and degenerate guards). It is threaded into the selector's
	// SelectState so the precedence routes to the commit-subject fallback body and
	// never calls the AI. It is irrelevant when NotesBody overrides the selector.
	NoAI bool
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

	// Stage 2 — project preflight hook. After mint's built-in gates pass and BEFORE
	// any mutation (and before startingHEAD is even captured), the project's optional
	// preflight hook runs for project-specific gates/validation. An absent hook
	// (cfg.Release.Hooks.Preflight == nil) is a no-op — the runner returns nil for a
	// nil/empty value. A non-zero exit (for an array, the first non-zero entry) aborts
	// the whole release cleanly: it is surfaced as a "preflight" StageFailed. Because
	// this precedes all mutation there is nothing to unwind — a plain surface, not the
	// auto-unwind path.
	if err := runPreflightHook(ctx, deps, cfg, root, current, next, tag, opts.Bump); err != nil {
		return surface(p, "preflight", err)
	}

	// Capture the clean starting state NOW: preflight has just confirmed the tree is
	// clean and HEAD is resolvable, and nothing has mutated yet, so this HEAD is the
	// unambiguous reset target the auto-unwind returns to. It is captured BEFORE the
	// gate and any mutation so a gate-abort or any pre-PONR failure can reset back to
	// exactly here. Failing to resolve it is a plain preflight failure (no mutation
	// to unwind yet).
	startingHEAD, err := resolveHEAD(ctx, deps.Runner)
	if err != nil {
		return surface(p, "preflight", err)
	}

	// Stage 3 — project prep. The optional pre_tag hook builds/generates artifacts
	// (e.g. a knowledge bundle) and may dirty the tree; mint then commits whatever it
	// left dirty as its OWN `chore(release): pre-tag artifacts for {tag}` commit, kept
	// distinct from the bookkeeping commit. This runs AFTER startingHEAD so the
	// artifact commit is covered by the auto-unwind, and BEFORE notes so they generate
	// at the post-hook HEAD. An absent hook is a no-op (no prep, no artifact commit); a
	// non-zero exit aborts cleanly before any notes/tag/push — routed through the
	// auto-unwind so a hook that made its own commit before failing is reset back to
	// startingHEAD.
	if err := runPreTagHook(ctx, deps, cfg, root, current, next, tag, opts.Bump); err != nil {
		return surfaceAndUnwind(ctx, deps, "pre_tag", startingHEAD, tag, false, err)
	}

	// Stage 4 — notes body. resolveBody runs the notes-path PRECEDENCE (SelectBody)
	// to pick the body + Kind for this run, building the selector from deps.Runner now
	// that root is resolved. opts.NotesBody is a test-injection override that bypasses
	// the selector. An on_notes_failure=abort failure surfaces and aborts here, BEFORE
	// any mutation (nothing tagged). Whatever resolves flows WHOLE to every active sink
	// below — no parsing, no splitting, no per-sink reassembly.
	body, kind, generator, err := resolveBody(ctx, deps, root, cfg, current, opts)
	if err != nil {
		return surface(p, "notes", err)
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

	// The review gate may EDIT (the `e` choice) or REGENERATE (the `r` choice, only
	// offered for KindNormalAI) the body; capture the returned final body and thread
	// IT to every downstream sink. The gate stays positioned BEFORE any mutation
	// (before Record). The resolved Kind selects the gate variant (y/n/e vs y/n/e/r);
	// the per-run regenerator binds this run's lastTag + cfg to the generator so an `r`
	// re-runs the SAME AI path SelectBody took.
	regenerator := perRunRegenerator(deps, generator, current.String(cfg.Release.TagPrefix), cfg)
	body, err = reviewGate(ctx, p, deps.Editor, regenerator, kind, body, versionKey)
	if err != nil {
		// A gate-no is a clean USER abort: route it through the shared auto-unwind so it
		// is treated identically to a pre-push failure (reset whatever mint made this
		// run, narrate an Unwound). No prior StageFailed — declining the gate is not a
		// stage failure. The gate sits BEFORE the tag, so the tag cannot exist yet
		// (tagMayExist=false). Every OTHER reviewGate error (edit/regenerate/AskLine/
		// unexpected) already surfaced its own StageFailed and occurred at the gate
		// before any mutation, so it keeps its current behaviour — returned as-is.
		if errors.Is(err, errGateAborted) {
			return unwind(ctx, deps, startingHEAD, tag, false, errGateAborted)
		}
		return err
	}

	// Stage 5 — record: write changelog (gated by the changelog toggle), then the
	// bookkeeping commit. When changelog=false WriteChangelog no-ops (Changed:false)
	// so CommitBookkeeping skips the commit — the tag then points at the existing
	// HEAD and STILL carries the full body via TagAndPush. Nothing durable is lost.
	writeResult, err := record.WriteChangelog(root, versionKey, opts.Now, body, cfg.Release.Changelog)
	if err != nil {
		// Post-mutation, pre-PONR: route through the shared auto-unwind so a half-written
		// changelog (and any bookkeeping commit) is reset back to the clean start — the
		// abort path is identical to the pre-push failure path. No tag exists yet.
		return surfaceAndUnwind(ctx, deps, "record", startingHEAD, tag, false, err)
	}
	if err := record.CommitBookkeeping(ctx, deps.Runner, root, cfg.Release.CommitPrefix, tag, writeResult.Changed); err != nil {
		return surfaceAndUnwind(ctx, deps, "record", startingHEAD, tag, false, err)
	}

	// Stage 2 (conditional gate 6) — gh auth, only when publishing, BEFORE the tag.
	if cfg.Release.Publish {
		if err := preflight.CheckGhAuth(ctx, deps.Runner); err != nil {
			// The bookkeeping commit may have moved HEAD; unwind resets it. No tag yet.
			return surfaceAndUnwind(ctx, deps, "preflight", startingHEAD, tag, false, err)
		}
	}

	// Stage 6 — tag + atomic push. A nil error means the atomic push succeeded and
	// PointOfNoReturnCrossed is set: from here the tag is public, so any later
	// failure is warn-only and the run must NOT unwind.
	if _, err := deps.Releaser.TagAndPush(ctx, tag, cfg.Release.CommitPrefix, body); err != nil {
		// Pre-PONR failure: route through the shared auto-unwind. A push REJECTION means
		// the local tag was created and must be deleted (tagMayExist); a tag-CREATION
		// failure means no tag exists, so only the commit is reset.
		tagMayExist := errors.Is(err, release.ErrPushRejected)
		return surfaceAndUnwind(ctx, deps, "tag", startingHEAD, tag, tagMayExist, err)
	}

	// Stage 7 — publish. Post-PONR: a publish failure is WARN-ONLY (the tag is
	// already public); the run does not unwind and exits successfully.
	releaseURL := ""
	if cfg.Release.Publish {
		if err := deps.Publisher.CreateRelease(ctx, tag, tag, body); err != nil {
			warnPublishFailed(p, err)
		}
	}

	// Stage 7 — post_release hook. Post-PONR follow-ups (notifications, tap
	// repository_dispatch). It runs UNCONDITIONALLY: reaching here means the push
	// already crossed the PONR, so the tag is public whether publish=true or
	// publish=false, and post-release follow-ups apply either way. Like the publish
	// failure above, a non-zero exit is WARN-ONLY — it does NOT abort or unwind; the
	// run still reaches RunFinished and returns nil. This is the ONLY hook point whose
	// failure is non-fatal (preflight and pre_tag abort); the array stop-on-first-
	// failure SEQUENCING is identical across points — only the CONSEQUENCE differs.
	if err := runPostReleaseHook(ctx, deps, cfg, root, current, next, tag, opts.Bump); err != nil {
		warnPostReleaseFailed(p, err)
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

// resolveBody resolves this run's notes body + Kind via the notes-path precedence
// (SelectBody), returning the generator it built so the caller can bind a per-run
// regenerator for the gate's `r` choice. It is the single Stage-4 body decision:
//
//   - opts.NotesBody is a TEST-INJECTION OVERRIDE: when non-empty it bypasses the
//     selector and returns (opts.NotesBody, opts.NotesKind, nil generator, nil). The
//     gate's `r` is never reached for the no-AI Kinds these overrides use, so a nil
//     generator is fine; a normal-AI override test supplies its own deps.Regenerator.
//   - otherwise the selector is built from deps.Runner (now that root is resolved) and
//     SelectBody runs the precedence over the run's SelectState. A SelectBody error is
//     an on_notes_failure=abort notes failure: it returns the error for the caller to
//     surface and abort on, BEFORE any mutation.
//
// FirstRelease is detected by comparing current to the zero SemVer — a tagless repo
// resolves to {0,0,0}. An actual v0.0.0 tag is therefore treated as a first release;
// that edge is acceptable for Phase 2 (a real v0.0.0 release is not a meaningful case
// to support, and the selector simply records "Initial release." for it).
func resolveBody(ctx context.Context, deps ReleaseDeps, root string, cfg config.Config, current version.SemVer, opts ReleaseOptions) (string, notes.Kind, *notes.Generator, error) {
	if opts.NotesBody != "" {
		return opts.NotesBody, opts.NotesKind, nil, nil
	}

	// One Assembler (the single git seam) is shared by the Generator and the Selector
	// so the degenerate-check diff and the AI path range over the same git, exactly as
	// NewSelector documents.
	assembler := notes.NewAssembler(deps.Runner)
	generator := notes.NewGenerator(assembler, aiTransport(deps), root)
	selector := notes.NewSelector(generator, assembler, deps.Runner, root)

	state := notes.SelectState{
		FirstRelease: current == version.SemVer{},
		LastTag:      current.String(cfg.Release.TagPrefix),
		NoAI:         opts.NoAI,
	}
	body, kind, err := selector.SelectBody(ctx, state, cfg)
	if err != nil {
		return "", kind, nil, err
	}
	return body, kind, generator, nil
}

// aiTransport resolves the AI transport the notes Generator hands its prompt to:
// the injected deps.Transport when set (the test seam), else the production
// ai.Transport (default `claude -p`, zero Config) over the run's runner — so
// production leaves deps.Transport nil and gets the real transport. Wiring the
// ai_command / timeout config override is deferred to the Phase 6 schema.
func aiTransport(deps ReleaseDeps) notes.Transport {
	if deps.Transport != nil {
		return deps.Transport
	}
	return ai.NewTransport(deps.Runner, ai.Config{})
}

// regeneratorFunc adapts a plain regenerate closure to the Regenerator seam so the
// per-run AI regenerator (which must bind this run's lastTag + cfg, known only at run
// time) can be expressed without a dedicated type at the call site.
type regeneratorFunc func(ctx context.Context, oneTimeContext string) (string, error)

func (f regeneratorFunc) Regenerate(ctx context.Context, oneTimeContext string) (string, error) {
	return f(ctx, oneTimeContext)
}

// perRunRegenerator selects the Regenerator the gate's `r` choice consults. A wired
// deps.Regenerator OVERRIDES everything (the test-injection seam). Otherwise, when a
// generator was built (the normal-AI selector path), it returns a per-run closure
// binding lastTag + cfg to generator.GenerateWithContext — so an `r` re-runs the SAME
// AI path SelectBody took, with the one-time context appended. When neither is present
// (an override body, or a no-AI path with no generator), it returns nil; `r` is only
// offered for KindNormalAI, so the nil is never consulted on the no-AI paths.
func perRunRegenerator(deps ReleaseDeps, generator *notes.Generator, lastTag string, cfg config.Config) Regenerator {
	if deps.Regenerator != nil {
		return deps.Regenerator
	}
	if generator == nil {
		return nil
	}
	return regeneratorFunc(func(ctx context.Context, oneTimeContext string) (string, error) {
		return generator.GenerateWithContext(ctx, lastTag, cfg, oneTimeContext)
	})
}

// resolveHEAD reads the current commit SHA via `git rev-parse HEAD` through the
// runner seam (every git op goes through the seam). It is used BOTH to capture the
// clean starting state before the gate and to probe the current HEAD inside the
// unwind, so the two are compared apples-to-apples.
func resolveHEAD(ctx context.Context, r runner.CommandRunner) (string, error) {
	res, err := r.Run(ctx, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Stdout), nil
}

// unwind is the single shared clean-reset used by BOTH the user-abort (gate-no) and
// the pre-push failure paths, so a declined gate and a rejected push are treated
// identically: mint rolls back whatever it made THIS run, returning the tree to the
// exact clean starting state captured before the gate (startingHEAD).
//
// It probes the current HEAD: if it MOVED, mint committed this run, so a `git reset
// --hard {startingHEAD}` returns the tree to the clean start (also discarding any
// uncommitted churn, e.g. a half-written changelog after a failed commit); if HEAD
// is unchanged there is nothing to reset. When tagMayExist is true it also deletes
// the local tag mint created this run (`git tag -d {tag}`, best-effort — a "tag not
// found" non-zero is not fatal). The engine AUTHORS the whole Unwound Summary
// (ASCII, semicolon-separated, INCLUDING the "repo clean" tail) and the presenter
// renders it verbatim; it returns abort(reason) so the engine owns the non-zero
// exit. It emits NO StageFailed (stage-failure callers emit that first via
// surfaceAndUnwind; the gate-no path emits none).
//
// Phase 4 hardening is DEFERRED and deliberately NOT implemented here: precise
// N-commit surgical counting beyond a simple reset, lock-resilient git wrapping, and
// --autostash stash/restore ordering all land in Phase 4. This is the spine reset.
func unwind(ctx context.Context, deps ReleaseDeps, startingHEAD, tag string, tagMayExist bool, reason error) error {
	commitReset := false
	if current, err := resolveHEAD(ctx, deps.Runner); err != nil || current != startingHEAD {
		// On a probe error the safe move is still to reset to the known-clean start; on a
		// moved HEAD the reset is mandatory. Either way reset to the captured starting
		// state. A reset failure is best-effort here (Phase 4 adds lock-resilient
		// wrapping); the abort still carries the original reason.
		_, _ = deps.Runner.Run(ctx, "git", "reset", "--hard", startingHEAD)
		commitReset = true
	}

	tagDeleted := false
	if tagMayExist {
		// Best-effort: a "tag not found" non-zero is not fatal — the goal is that no tag
		// mint made this run survives the abort.
		_, _ = deps.Runner.Run(ctx, "git", "tag", "-d", tag)
		tagDeleted = true
	}

	deps.Presenter.Unwound(presenter.Unwind{Summary: unwindSummary(commitReset, tagDeleted, tag)})
	return abort(reason)
}

// unwindSummary authors the engine-owned Unwound Summary describing what the unwind
// undid, INCLUDING the trailing "repo clean" tail. It is ASCII and semicolon-joined
// (matching the existing "…; repo clean" style) so the plain presenter stays
// byte-pure; the presenter renders it VERBATIM and never synthesises the tail.
func unwindSummary(commitReset, tagDeleted bool, tag string) string {
	const tail = "; repo clean"
	switch {
	case commitReset && tagDeleted:
		return "reset the release commit and deleted tag " + tag + tail
	case commitReset:
		return "reset the release commit" + tail
	case tagDeleted:
		return "deleted tag " + tag + tail
	default:
		return "nothing to undo" + tail
	}
}

// surfaceAndUnwind handles a post-mutation, pre-PONR STAGE failure: it surfaces the
// StageFailed first (so the failed stage is shown) and then routes through the
// shared unwind, so the abort path after a bookkeeping commit is identical to the
// pre-push failure path (the commit is reset). It is the stage-failure sibling of
// the gate-no path, which calls unwind directly with no StageFailed.
func surfaceAndUnwind(ctx context.Context, deps ReleaseDeps, stage, startingHEAD, tag string, tagMayExist bool, cause error) error {
	deps.Presenter.StageFailed(presenter.StageFailure{
		Name:    stage,
		Message: failureMessage(cause),
	})
	return unwind(ctx, deps, startingHEAD, tag, tagMayExist, cause)
}

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

// runPreflightHook runs the project's optional preflight hook through the shared
// hooks runner. It assembles the MINT_* env from the run's computed versions
// (NewVersion = the bare version being released, PreviousVersion = the bare prior
// latest, VersionTag = the full prefixed tag) and the bump kind, then runs the
// configured [release.hooks].preflight value from the repo root. An absent value
// (nil) is a no-op — the runner returns nil. A non-zero exit (the first non-zero
// entry for an array) returns a non-nil error the caller surfaces and aborts on.
// DryRun is fixed to false here; MINT_DRY_RUN skip behaviour is a later phase.
func runPreflightHook(ctx context.Context, deps ReleaseDeps, cfg config.Config, root string, current, next version.SemVer, tag string, bump version.Bump) error {
	env := buildHookEnv(current, next, tag, bump)
	return hooks.NewRunner(deps.Runner).Run(ctx, cfg.Release.Hooks.Preflight, root, env)
}

// runPreTagHook runs the project's optional Stage-3 pre_tag hook and then applies
// the artifact-commit interplay rule. The hook (build/generate artifacts) runs
// through the shared hooks runner with the same MINT_* env as the preflight hook. An
// absent value (nil) is a no-op — the runner returns nil and no artifact commit is
// considered. A non-zero exit (the first non-zero entry for an array) returns a
// non-nil error the caller routes through the auto-unwind.
//
// On hook SUCCESS, mint commits whatever the hook left dirty as its OWN commit
// (subject `chore(release): pre-tag artifacts for {tag}` — a FIXED chore prefix, NOT
// the configurable commit_prefix), via record.CommitDirtyTree: a clean tree (empty
// `git status --porcelain`) commits nothing — which covers a hook that built nothing,
// a hook that made its own commit, and gitignored-only outputs alike. A commit
// failure is surfaced for the caller to unwind. The interplay rule applies ONLY after
// a hook actually ran: an absent hook skips the artifact-commit probe entirely, so
// the existing no-hook spine is untouched. DryRun is fixed to false here; MINT_DRY_RUN
// skip behaviour is a later phase.
func runPreTagHook(ctx context.Context, deps ReleaseDeps, cfg config.Config, root string, current, next version.SemVer, tag string, bump version.Bump) error {
	if cfg.Release.Hooks.PreTag == nil {
		return nil
	}

	env := buildHookEnv(current, next, tag, bump)
	if err := hooks.NewRunner(deps.Runner).Run(ctx, cfg.Release.Hooks.PreTag, root, env); err != nil {
		return err
	}
	_, err := record.CommitDirtyTree(ctx, deps.Runner, root, pretagArtifactSubject(tag))
	return err
}

// runPostReleaseHook runs the project's optional Stage-7 post_release hook through
// the shared hooks runner with the same MINT_* env as the other points (reusing
// buildHookEnv). An absent value (nil) is a no-op — the runner returns nil. A
// non-zero exit (the first non-zero entry for an array — the stop-on-first-failure
// SEQUENCING is identical to the other points) returns a non-nil error. UNLIKE
// preflight/pre_tag, the CONSEQUENCE here is warn-only: the caller does NOT abort or
// unwind, because by Stage 7 the push has crossed the PONR and the tag is public.
// DryRun is fixed to false here; MINT_DRY_RUN skip behaviour is a later phase.
func runPostReleaseHook(ctx context.Context, deps ReleaseDeps, cfg config.Config, root string, current, next version.SemVer, tag string, bump version.Bump) error {
	env := buildHookEnv(current, next, tag, bump)
	return hooks.NewRunner(deps.Runner).Run(ctx, cfg.Release.Hooks.PostRelease, root, env)
}

// pretagArtifactSubject builds the FIXED subject for the pre_tag artifact commit. It
// uses a constant `chore(release):` prefix — NOT the configurable commit_prefix —
// because the artifact commit is project content (e.g. a rebuilt bundle), distinct
// from the release bookkeeping commit (`{commit_prefix} Release {tag}`).
func pretagArtifactSubject(tag string) string {
	return "chore(release): pre-tag artifacts for " + tag
}

// buildHookEnv assembles the shared MINT_* hook environment from the run's computed
// versions (NewVersion = the bare version being released, PreviousVersion = the bare
// prior latest, VersionTag = the full prefixed tag) and the bump kind. The preflight
// and pre_tag points share it so they inject an identical env. DryRun is fixed to
// false; MINT_DRY_RUN skip behaviour is a later phase.
func buildHookEnv(current, next version.SemVer, tag string, bump version.Bump) hooks.HookEnv {
	return hooks.NewHookEnv(next.String(""), current.String(""), tag, hookBump(bump), false)
}

// hookBump maps the engine's version.Bump onto the hooks package's Bump so the
// MINT_BUMP variable reflects how the version was chosen. --set-version/explicit
// is a later phase and not reachable yet; an unmapped value falls back to patch.
func hookBump(bump version.Bump) hooks.Bump {
	switch bump {
	case version.BumpMinor:
		return hooks.BumpMinor
	case version.BumpMajor:
		return hooks.BumpMajor
	default:
		return hooks.BumpPatch
	}
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

// errRegeneratorUnavailable is the cause surfaced when the `r` choice is taken but
// no Regenerator seam was wired — a misconfiguration that should never reach
// production (the gate offers `r` only for KindNormalAI, which task 2-16 wires with
// a Regenerator). It is surfaced rather than panicked so the spine fails loud and
// clean before any mutation.
var errRegeneratorUnavailable = errors.New("regenerate chosen but no regenerator is configured")

// reviewGate runs the notes review gate as the engine-owned re-entry LOOP and
// returns the (possibly edited or regenerated) FINAL body the caller threads to
// every sink. Rendering stays in the presenter; this owns only the semantics.
//
// The gate VARIANT is selected by kind (gateForKind): notes.KindNormalAI gets the
// four-choice y/n/e/r gate (an AI produced the body, so regenerating is
// meaningful), while every other Kind gets the y/n/e gate with no `r`. The SAME
// selected gate is used on every loop iteration.
//
// Each pass reads one decision and acts on it:
//
//   - a Prompt error (ErrNotInteractive / ErrInputClosed) is already an
//     *AbortError carrying a non-zero exit; it is returned as-is (ErrNotInteractive
//     is pre-rendered by the presenter, ErrInputClosed is surfaced via the abort).
//   - ChoiceYes (also the bare-Enter default) accepts: the loop RETURNS the current
//     body so the spine proceeds to Record with exactly the reviewed text. Under -y
//     the presenter returns the gate Default (ChoiceYes), so the loop accepts
//     immediately and `r` is never reached — regenerate is interactive-only.
//   - ChoiceNo aborts: it returns a clean errGateAborted cause WITHOUT emitting an
//     Unwound — Release routes the gate-no through the shared auto-unwind (which
//     authors the Unwound and resets whatever mint made), so a user-abort and a
//     pre-push failure share one path. The gate sits BEFORE any mutation, so the
//     unwind finds nothing to reset; the run still exits non-zero.
//   - ChoiceEdit edits: the editor seam returns the saved text, which REPLACES the
//     body VERBATIM — no re-parse, no re-validation (a human edit is trusted;
//     structural validation only ever guards untrusted AI output). The notes are
//     re-shown and the loop re-prompts. A return-to-gate signal
//     (ErrEditorReturnToGate — no editor could be launched, already reported by the
//     launcher) re-presents the gate with the body UNCHANGED, no re-render and no
//     abort. Any OTHER editor-seam failure (a launched-but-failed editor, the
//     nil-editor misconfiguration) is surfaced and aborts rather than blocking the
//     spine.
//   - ChoiceRegen regenerates (only reachable on the four-choice gate): see
//     regenerateBody. It reads a one-time context line via AskLine, re-runs the AI,
//     REPLACES the body with the regenerated text, re-shows the notes, and LOOPS so
//     the user may regenerate again or settle on y/n/e.
func reviewGate(ctx context.Context, p presenter.Presenter, editor Editor, regen Regenerator, kind notes.Kind, body, versionKey string) (string, error) {
	gate := gateForKind(kind)
	for {
		choice, err := ReviewDecision(p, gate)
		if err != nil {
			return "", err
		}

		switch choice {
		case presenter.ChoiceYes:
			// Accept (also the bare-Enter default): proceed with the reviewed body.
			return body, nil
		case presenter.ChoiceNo:
			// Abort on gate-no: return a CLEAN cause and let Release route it through the
			// shared auto-unwind (which authors the Unwound and owns the non-zero exit) —
			// the gate no longer emits its own Unwound, so the user-abort and the pre-push
			// failure share one reset/narration path.
			return "", abort(errGateAborted)
		case presenter.ChoiceEdit:
			edited, eerr := editBody(ctx, editor, body)
			switch {
			case errors.Is(eerr, ErrEditorReturnToGate):
				// No editor could be launched: the launcher already reported the problem
				// via the presenter. RE-PRESENT the gate with the body UNCHANGED — this
				// is not a failure, so do not surface or abort, and do not re-render.
				continue
			case eerr != nil:
				// A genuine edit failure (a launched-but-failed editor, an IO error, or
				// the nil-editor misconfiguration): surface and abort.
				return "", surface(p, "edit", eerr)
			}
			// Use the edited text VERBATIM — no re-parse, no re-validation — then
			// re-render the notes and loop back to re-prompt.
			body = edited
			p.ShowNotes(presenter.Notes{Version: versionKey, Body: body})
		case presenter.ChoiceRegen:
			// Regenerate (only reachable on the four-choice gate): re-run the AI with a
			// one-time context line, replace the body, re-show, and loop. Any failure
			// (closed input, nil seam, regenerate error) is surfaced and aborts.
			regenerated, rerr := regenerateBody(ctx, p, regen, versionKey)
			if rerr != nil {
				return "", rerr
			}
			body = regenerated
		default:
			// The gate declares only its own choice set and the presenter returns a
			// member of it; any other choice is a contract violation. Fail loud rather
			// than spin the loop forever.
			return "", surface(p, "review", errUnexpectedChoice(choice))
		}
	}
}

// gateForKind selects the review-gate variant for the precedence path that
// produced the body: notes.KindNormalAI gets the four-choice y/n/e/r gate (the one
// path with an AI to regenerate), and EVERY other Kind (first-release, degenerate,
// --no-ai, fallback) gets the y/n/e gate with no `r` — offering `r` there would be
// meaningless (no AI to nudge) and, under --no-ai, would contradict the flag.
func gateForKind(kind notes.Kind) presenter.Gate {
	if kind == notes.KindNormalAI {
		return presenter.NotesReviewGate()
	}
	return FirstReleaseReviewGate()
}

// regenerateBody runs the `r` regenerate step and returns the regenerated body.
// It reads a one-time context line via the presenter's AskLine input seam (the
// engine NEVER reads stdin directly), an EMPTY answer being legal (regenerate with
// no extra context), then hands that line to the Regenerator. Each failure mode is
// fail-loud and surfaced before any mutation:
//
//   - AskLine's ErrInputClosed / ErrNotInteractive abort (the read failed; the
//     engine owns surfacing ErrInputClosed). `r` is interactive-only, so
//     ErrNotInteractive should be unreachable here — it is defended against anyway.
//   - a nil Regenerator (a misconfiguration that should never reach production for
//     KindNormalAI) surfaces a clean error and aborts rather than panicking.
//   - a Regenerator failure is surfaced and aborts. Richer handling could re-present
//     the gate on a regenerate failure, but surface+abort keeps the path
//     deterministic.
//
// On success the regenerated body is re-shown (so the user reviews it) and
// returned; the caller replaces the body and loops the gate.
func regenerateBody(ctx context.Context, p presenter.Presenter, regen Regenerator, versionKey string) (string, error) {
	line, err := p.AskLine(regenContextPrompt)
	if err != nil {
		// A closed/non-interactive input on the one-time-context read is fail-loud: the
		// presenter leaves ErrInputClosed unrendered, so the abort (and any closing
		// narration) is the engine's to surface — done via abort, mirroring ReviewDecision.
		return "", abort(err)
	}

	if regen == nil {
		// `r` was offered (KindNormalAI) but no Regenerator was wired — a
		// misconfiguration. Surface a clean failure and abort rather than panicking.
		return "", surface(p, "regenerate", errRegeneratorUnavailable)
	}

	regenerated, rerr := regen.Regenerate(ctx, line)
	if rerr != nil {
		// A regenerate failure is fail-loud: surface and abort.
		return "", surface(p, "regenerate", rerr)
	}

	// Re-show the regenerated notes so the user reviews them before the gate re-prompts.
	p.ShowNotes(presenter.Notes{Version: versionKey, Body: regenerated})
	return regenerated, nil
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

// warnPostReleaseFailed emits the post-PONR warn-only event for a failed
// post_release hook: by Stage 7 the push has crossed the point of no return, so the
// tag is public and mint must NOT unwind. It warns (the spec-fixed message) and the
// run still finishes successfully. When the failure carries a *hooks.HookError its
// captured output (the failing entry's stderr) is rendered beneath the warn line, as
// warnPublishFailed does for the provider error; otherwise the Output is empty.
func warnPostReleaseFailed(p presenter.Presenter, cause error) {
	p.Warn(presenter.Warning{
		Label:   "post_release",
		Message: "post_release hook failed; tag is already published",
		Output:  hookFailureOutput(cause),
	})
}

// hookFailureOutput extracts the failing hook entry's captured stderr from a
// *hooks.HookError so the warn can render it verbatim; a cause that is not a
// HookError (or that captured no stderr) yields the empty string — the common case,
// which renders no output block.
func hookFailureOutput(cause error) string {
	var hookErr *hooks.HookError
	if errors.As(cause, &hookErr) {
		return hookErr.Result.Stderr
	}
	return ""
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
