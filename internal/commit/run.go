package commit

// This file is commit's ORCHESTRATOR. It owns ORDERING for the whole `mint commit`
// verb and CALLS the shipped pieces — it never re-implements their logic: resolve the
// repo root, load config (the shared engine keys + the [commit] table), run the
// staging-mode-aware empty-staging preflight (preflight.go), GENERATE the
// conventional-commits message from the mode's would-be-committed diff (the L3
// Generator in generate.go), present it at the Continue? review gate (y/n/e/r —
// reviewLoop below), and on accept run the shared stage→commit→push accept tail
// (commitAccept). Three "no AI message" triggers — --no-ai, an AI transport failure,
// and an oversized diff — converge on the SAME $EDITOR fallback (runEditorFallback),
// where the save IS the accept event.
//
// The load-bearing invariant is MUTATE NOTHING UNTIL ACCEPT: every read (preflight
// probes, the per-mode L1 diff) is computed read-only, the gate and the editor sit
// BEFORE the mutation half, and the deferred `git add` (-a/-A) runs only inside the
// accept tail — so any decline/abort leaves the index byte-for-byte untouched. After
// accept the contract flips to never-unwind: a failed -p push warns and keeps the
// commit (pushAfterCommit).
//
// git_safe is non-negotiable: every git mutation (the deferred staging, the commit,
// the push) flows through the Mutator seam (the consumed lock-resilient *git.Mutator
// in production), NEVER the raw runner — a contended/stale .git lock is
// retried/cleared so a background agent or editor briefly holding the index lock
// cannot blow up a commit.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/gitrepo"
	"mint/internal/notes"
	"mint/internal/presenter"
	"mint/internal/runner"
)

// commitAction is the engine-supplied verb word for the start-of-run header.
const commitAction = "committing"

// commitMessageTitle labels the ShowMessage delimiters for the minted commit
// message. It is ASCII so the plain-mode delimiters stay byte-pure (the same
// convention as the gate Subject/AcceptEcho values).
const commitMessageTitle = "commit message"

// The oversized-diff fallback note. When commit's L1 size guard reports the
// (diff_exclude-filtered) diff is over max_diff_lines — a generate-SKIP, distinct from
// a generate-FAILURE — commit emits this note via the Presenter and routes to the SAME
// editor fallback as --no-ai. The message is the spec string VERBATIM (Commit Message
// Format & Prompt → The $EDITOR fallback, case 3), em dash U+2014; the label is a short
// informational tag prefixed onto the warn line.
const (
	oversizedNoteLabel   = "diff"
	oversizedNoteMessage = "diff too large to summarise — opening editor"
)

// The `e` gate-action graceful-degrade warning. When the user presses `e` at the gate
// but NO editor in git's chain is launchable (the 3-1 not-launchable signal), commit
// does NOT fail loud — a message candidate already exists — so it warns that the editor
// could not be launched and re-renders the gate with the UNEDITED message preserved
// (treating `e` as a no-op). This is the MIRROR of runEditorFallback's errNoMessageSource
// fail-loud (the no-message-source path): same 3-1 signal, opposite consumer decision.
// The wording is NOT spec-pinned; it is a clear, lowercase line consistent with the
// codebase's "could not launch editor" warn precedent (internal/engine/editor.go).
const (
	editorNoLaunchWarnLabel   = "editor"
	editorNoLaunchWarnMessage = "could not launch editor, keeping the message"
)

// regenContextPrompt is the short free-text prompt the gate's `r` action renders via the
// presenter's AskLine before regenerating. It cues the user for an optional one-time
// steer; Enter (an empty line) submits a plain re-roll. The wording is NOT spec-pinned —
// it is a clear, lowercase line consistent with the codebase's prompt phrasing.
const regenContextPrompt = "extra context for the regeneration (Enter to skip)"

// errGateAborted is the clean cause for a user `n` decline at the review gate. It is
// a TRUE no-op cause — nothing was mutated (the gate sits before the commit sink) —
// so the abort path emits NO StageFailed failure narration. It surfaces only so the
// run exits non-zero (mapped to exit 1 by cmd/mint's exitCode), telling a caller the
// commit did not happen; it is distinct from the AI/transport failures that DO
// narrate a StageFailed.
var errGateAborted = errors.New("commit aborted at review gate")

// errEditorNoOp is the clean cause for a true no-op on the --no-ai editor fallback: an
// empty (whitespace-only) save or an aborted/quit editor. Like errGateAborted it is a
// TRUE no-op cause — nothing was mutated (the save IS the accept event; an empty/aborted
// editor never reaches staging or the commit) — so it emits NO StageFailed narration. It
// surfaces only so the run exits non-zero (mapped to exit 1 by cmd/mint's exitCode),
// telling a caller the commit did not happen.
var errEditorNoOp = errors.New("commit aborted: editor closed with no message")

// errNoMessageSource is the fail-loud cause when the editor fallback has no way to
// produce a message: an unattended run (-y), a non-TTY stdin (the startup-resolved
// StdinInteractive signal is false), or no launchable editor in git's chain on a TTY.
// The fallback is inherently interactive, so without a human at a terminal there is
// nothing to commit with — mint fails loud rather than hang or commit an empty message.
// It extends the gate's forbidden-combo philosophy to the editor path and applies
// IDENTICALLY across all three converging triggers (--no-ai, AI-failure, oversized),
// which all reach runEditorFallback. The message is the spec string VERBATIM (lowercase,
// no trailing punctuation); it is surfaced (StageFailed) so the cmd layer maps it to a
// non-zero exit. There is NO -m/--message escape — unattended-with-own-message uses
// plain `git commit`.
var errNoMessageSource = errors.New("no AI message and no interactive editor available")

// errPushFailed is the internal sentinel for a FAILED auto-push under warn-don't-unwind.
// The commit already happened and is kept untouched; the Warn (below) narrates the
// failure, so this sentinel carries NO user-facing message and is NEVER surfaced via
// surface/StageFailed. It exists ONLY to drive a non-zero exit: it is a plain error, so
// cmd/mint's exitCode (which matches only *engine.AbortError) falls through to the
// deterministic generic exit 1 — telling scripted/CI callers the push failed while the
// commit stays in place. The post-accept never-unwind invariant is absolute, so this
// path runs NO reset/revert/restore/unstage/amend.
var errPushFailed = errors.New("commit: push failed; commit kept in place")

// The push-failure warn label and generic message. mint does NOT classify the cause —
// rejected/non-fast-forward, remote-moved, no-upstream, and network ALL get this one
// generic warn; git's own specific hint (set-upstream, non-fast-forward, etc.) stays
// visible only via the verbatim Output pass-through of git's stderr. The message is NOT
// spec-pinned exact (the spec's "set an upstream and push" is illustrative of git's
// pass-through, not mint-authored text): it is a clear, lowercase line conveying the
// commit is in place and the push is repeatable. It deliberately omits any per-cause
// phrasing (no "upstream", no "non-fast-forward").
const (
	pushFailWarnLabel   = "push"
	pushFailWarnMessage = "commit is in place; re-run the push to retry"
)

// errRegenerateFallback is an INTERNAL routing sentinel — never user-facing. When the
// gate's `r` regeneration fails after the engine's one retry (an isAITransportFailure),
// reviewLoop returns this cause so Run routes to the SAME $EDITOR fallback a
// first-generation AI failure (3-3) takes. The spec mandates one consistent rule: any
// failed AI generation lands at the editor (no re-show of the pre-r message). The
// regeneration happens INSIDE reviewLoop, but the fallback must be owned by Run (the
// same entry point first-generation failures use) so a successful fallback commit exits 0
// rather than being turned into errGateAborted, and never double-stages/commits. Run
// INTERCEPTS this sentinel before it can surface — it carries no user-facing message.
var errRegenerateFallback = errors.New("commit: regenerate failed, routing to editor fallback")

// Mutator is commit's git-mutation SINK seam: EVERY git mutation commit makes flows
// through it so each is lock-resilient (git_safe), NEVER the raw runner. Three distinct
// mutations route through it — staging (`git add -u`/`git add -A`, stageForMode), the
// commit itself (`git commit -F -`, createCommit), and the auto-push (`git push`,
// pushAfterCommit) — and each one needs the lock wrapper, so a background agent or
// editor briefly holding the `.git` index lock cannot blow up staging, the commit, or
// the push. It is named to mirror engine.ReleaseDeps.Mutator.
//
// It is defined HERE — where it is consumed — so the orchestrator stays decoupled from
// the git package's concretion: *git.Mutator satisfies it in production (carrying the
// retry + stale-lock discrimination), while tests inject a fake/real Mutator over a
// FakeRunner. The signature mirrors git.Mutator.Mutate exactly: a body (the commit
// message) is passed as raw BYTES on stdin (a retry must re-pipe the full payload), and
// the argv follows; staging and push pass nil stdin. Read-only git (commit's L1
// staged-diff and the would-be-staged probes) stays on the plain Runner — only the
// mutations need the wrapper.
type Mutator interface {
	// Mutate runs a git mutation with lock resilience, piping stdin (the commit body for
	// `git commit -F -`; nil for `git add`/`git push`) as raw bytes when non-nil. It
	// returns the runner Result and a non-nil error on a non-lock failure or an exhausted
	// retry budget.
	Mutate(ctx context.Context, stdin []byte, name string, args ...string) (runner.Result, error)
}

// Deps bundles the orchestrator's injected seams so production wires the real
// implementations once (at the cmd entry point) and tests drive the whole thread
// with a RecordingPresenter + a single FakeRunner. It mirrors engine.ReleaseDeps's
// shape (presenter + read runner + a lock-resilient mutation seam) but carries only
// what the bare commit path needs.
type Deps struct {
	// Presenter is mint's single output seam. Run emits its lifecycle events through
	// it and never touches stdout/TTY directly.
	Presenter presenter.Presenter
	// Runner is the external-command seam commit's L1 staged-diff read issues git
	// through unchanged — the read-only `git diff --cached` does NOT go through the
	// lock wrapper.
	Runner runner.CommandRunner
	// Mutator is the lock-resilient git MUTATION sink (git_safe). ALL of commit's git
	// mutations flow through it — staging (`git add`), the commit (`git commit -F -`),
	// and the auto-push (`git push`) — so each is retried/cleared past a contended/stale
	// `.git` lock; production wires *git.Mutator constructed once over the same Runner.
	Mutator Mutator
	// Transport is the OPTIONAL L2 AI seam the Generator hands its composed prompt to.
	// It exists so tests can drive the real generate thread over the FakeRunner while
	// scripting the AI body directly. When nil, Run builds the production ai.Transport
	// over Runner once config is loaded, driving it with the validated cfg.AICommand —
	// so production leaves it nil and gets the real transport.
	Transport Transport
	// Root is the OPTIONAL pre-resolved repo root (the test-injection seam: tests pass
	// a TempDir so config.Load reads no real repo config and ResolveInstructions reads
	// no real override file). When empty, Run resolves it via gitrepo.ResolveRoot —
	// production leaves it empty and gets the real repo root.
	Root string
	// Staging is the resolved staging mode (cmd layer resolves the mutually-exclusive
	// -a/-A flags into it). The zero value StagedOnly is the default — commit the index
	// exactly as staged, running NO `git add`. All/AddAll select the deferred-staging
	// behaviour: the would-be-committed diff is computed read-only for the preview, and
	// the mode's `git add -u`/`-A` runs only on accept (stageForMode, inside the
	// commitAccept tail) — the mutate-nothing-until-accept invariant.
	Staging StagingMode
	// NoAI selects the --no-ai degradation path: skip the L3 generate AND the Continue?
	// gate entirely, opening the editor directly (the editor save IS the accept event).
	// A non-empty save applies the mode's deferred staging then commits; an empty save
	// or an aborted/quit editor is a true no-op. False is the normal AI-generate path.
	NoAI bool
	// Yes is the resolved -y (auto-accept) flag, threaded from the cmd layer. The editor
	// fallback's no-message-source guard reads it: under -y a fallback is unattended with
	// no human to write a message, so it fails loud rather than launch an editor. (The
	// gate's own -y auto-accept is presenter-internal; this is the engine-side guard.)
	Yes bool
	// StdinInteractive is the run-level startup signal — presenter.DetectStartupSignals(
	// plainFlag, stdout, stdin).StdinInteractive — resolved ONCE at startup and threaded
	// here as a boolean (the Presenter interface exposes no accessor for it). The editor
	// fallback's no-message-source guard reads it: a non-interactive stdin cannot host the
	// inherently-interactive editor, so a fallback fails loud rather than hang. It is the
	// SAME determination the gate's non-TTY-without-`-y` fail-loud uses — NOT a separate
	// /dev/tty/stdout probe and NOT a re-implemented isatty.
	StdinInteractive bool
	// Push is the resolved -p/--push armed value, threaded from the cmd layer. ARMED
	// (true) means push after a SUCCESSFUL commit; DISARMED (false, the default) means no
	// push. Push is FLAG-ONLY — "we never push without the -p flag" — so there is
	// deliberately NO push config default; the flag is the sole source of this value and
	// Run never reads a config push key. Both accept paths (gate-accept and the editor's
	// save-as-accept) consume it through the single shared pushAfterCommit step, which
	// warns-don't-unwinds on failure; no-commit runs never reach it.
	Push bool
}

// editorUnavailable is the SINGLE "will the interactive editor refuse to open?" predicate.
// It is true on exactly the unattended runs the $EDITOR fallback cannot serve: an
// auto-accept -y run (no human to write a message) OR a non-TTY stdin (the startup-resolved
// StdinInteractive signal — the SAME determination the gate uses, not a separate /dev/tty
// probe). Defining it ONCE keeps the two consumers in lock-step so they cannot drift:
//
//   - runEditorFallback's no-message-source guard fails loud when it is true; AND
//   - the oversized branch emits the "opening editor" note ONLY when it is false (the editor
//     will actually open) — so an unattended oversized run fails loud with the single spec
//     message and no contradictory preceding note.
func (d Deps) editorUnavailable() bool {
	return d.Yes || !d.StdinInteractive
}

// Run executes the `mint commit` thread, orchestrating the pieces in EXACTLY this
// order:
//
//  1. Resolve the repo root (anchored the same way release resolves it) and load
//     config (for the shared engine keys + the [commit] table).
//  2. Empty-staging preflight: the staging-mode-aware would-be-staged emptiness check
//     runs BEFORE generate so the AI is never invoked on an empty diff.
//  3. Generate the conventional-commits message via the L3 Generator: it assembles
//     the mode's would-be-committed diff (commit's L1, read-only), applies the size
//     guard, composes the prompt, and calls the L2 transport. The AI infers the type;
//     scope is off by default; the body returns verbatim. --no-ai skips this (and the
//     gate) for the $EDITOR fallback; an AI transport failure or an oversized diff
//     routes to the SAME fallback.
//  4. Show the minted message (ShowMessage) and present the Continue? review gate
//     (reviewLoop, with its e edit and r regenerate re-entries) — BOTH before the
//     mutation half, so an accept proceeds and a decline aborts with nothing mutated
//     (the mutate-nothing-until-accept invariant).
//  5. On accept, run the shared accept tail (commitAccept): the mode's deferred
//     staging, then the commit through the git_safe Mutator — piping the final body on
//     stdin (`git commit -F -`) so the commit message lands BYTE-FOR-BYTE (no
//     commit_prefix, no branding, no reformatting) — then the optional -p push.
//
// A failure at any step surfaces through the presenter and aborts with a non-zero exit
// (an *engine-style abort is owned by the cmd layer's exit mapping).
func Run(ctx context.Context, deps Deps) error {
	p := deps.Presenter

	root, err := resolveRoot(ctx, deps)
	if err != nil {
		return surface(p, "preflight", err)
	}

	cfg, err := config.Load(root)
	if err != nil {
		return surface(p, "config", err)
	}

	p.RunStarted(presenter.RunInfo{
		Project: projectName(root),
		Action:  commitAction,
	})

	// Empty-staging preflight runs HERE — before generate, so the AI is never invoked on
	// an empty diff. It is staging-mode aware: it computes the would-be-staged emptiness
	// for deps.Staging READ-ONLY and, when empty, fails loud with the message keyed on the
	// ACTUAL post-mode tree state (clean tree vs changes-the-mode-could-not-stage). A
	// non-empty would-be-staged set proceeds. cfg.DiffExclude is threaded in so the
	// preflight measures the POST-exclusion would-be-staged set — the SAME exclusion-filtered
	// source the AI's L1 diff consumes — so an all-excluded set fails loud here rather than
	// reaching generate with a blank post-exclusion diff.
	if err := checkSomethingToCommit(ctx, deps.Runner, root, deps.Staging, cfg.DiffExclude); err != nil {
		return surface(p, "preflight", err)
	}

	// --no-ai degradation path: skip L3 generate AND the Continue? gate (the gate is
	// AI-path-only) and route straight to the editor fallback. The editor save IS the
	// accept event — a non-empty save applies the deferred staging then commits; an
	// empty/aborted editor is a true no-op. mint opens the editor itself (staging is
	// deferred until the save event), against an EMPTY buffer (no synthetic stub).
	if deps.NoAI {
		return runEditorFallback(ctx, deps, root, "")
	}

	body, err := generateMessage(ctx, deps, cfg, root)
	if err != nil {
		// An over-limit (diff_exclude-filtered) diff is a generate-SKIP, NOT a failure:
		// commit's L1 size guard returned notes.ErrDiffTooLarge BEFORE any L2 call (the
		// transport was never reached). A routine large commit must not abort — route to the
		// SAME editor fallback as --no-ai (empty buffer, save-as-accept). The oversized NOTE
		// ("opening editor") is emitted FIRST, but ONLY when the editor will actually open
		// (an attended run): on an unattended run (editorUnavailable — -y or non-TTY) the
		// fallback fails loud immediately, so emitting the note would promise an editor that
		// never opens. Gating on the SAME predicate the fallback's guard uses keeps the two
		// in lock-step. This branch carries the note; the AI-failure branch below does NOT.
		if errors.Is(err, notes.ErrDiffTooLarge) {
			if !deps.editorUnavailable() {
				p.Warn(presenter.Warning{Label: oversizedNoteLabel, Message: oversizedNoteMessage})
			}
			return runEditorFallback(ctx, deps, root, "")
		}
		// An AI transport failure (bad content after the transport's one retry, a timeout,
		// or a missing binary) is NOT an abort for a routine local commit — it routes to the
		// SAME editor fallback as --no-ai, opened against an EMPTY buffer (no synthetic stub,
		// no re-show of a partial message), but carries NO oversized note (distinct from the
		// oversized-skip above). The transport's own bad-content retry is consumed here,
		// never re-run. Any OTHER generate error keeps the surface abort.
		if isAITransportFailure(err) {
			return runEditorFallback(ctx, deps, root, "")
		}
		return surface(p, "generate", err)
	}

	// The minted message is shown for review and the Continue? gate is presented in the
	// engine-owned review LOOP — both BEFORE the commit sink, so the core invariant
	// (mutate nothing until accept) holds: a decline aborts with nothing mutated. The
	// loop owns the ShowMessage→Prompt render and the gate's e (edit) re-entry: an `e`
	// non-empty save replaces the in-memory body verbatim and re-renders, so the FINAL
	// (possibly edited) body is what proceeds to the commit.
	accepted, finalBody, err := reviewLoop(ctx, deps, cfg, root, body)
	if err != nil {
		// A regeneration failure (after the engine's one retry) routes to the SAME editor
		// fallback as a first-generation AI failure (3-3) — Run owns the call so a successful
		// fallback commit exits 0 (NOT turned into errGateAborted) and never double-stages.
		// The editor opens EMPTY/template (no re-show of the pre-r message), exactly like 3-3.
		if errors.Is(err, errRegenerateFallback) {
			return runEditorFallback(ctx, deps, root, "")
		}
		return err
	}
	if !accepted {
		// A clean `n` decline: a TRUE no-op (nothing mutated — the gate precedes BOTH the
		// deferred staging and the commit). Surface a non-zero abort WITHOUT a StageFailed
		// — a deliberate decline is not a failure, so it emits no failure narration. The
		// return runs BEFORE any `git add`/`git commit`, so abort leaves the index exactly
		// its pre-mint state (any pre-existing user staging untouched).
		return errGateAborted
	}

	// On accept, run the shared stage→commit→push accept tail with the FINAL body from
	// the review loop (the edited text verbatim when the user pressed `e`, else the
	// generated message). commitAccept owns the mutate-nothing-until-accept mutation half
	// (STAGE→COMMIT is the only point the index is touched) and the warn-don't-unwind
	// push contract — see its doc comment.
	return commitAccept(ctx, deps, root, finalBody)
}

// runEditorFallback is the shared $EDITOR degradation path the three "no AI message"
// cases converge on (the --no-ai entry is wired here; AI-failure 3-3 and oversized 3-4
// reuse it unchanged). It opens the editor directly via the reusable OpenEditor
// roundtrip — initial is the caller-supplied buffer (empty on --no-ai; Phase 4's `e`
// pre-fills the current message) — and treats the SAVE as the accept event.
//
// The no-message-source guard runs FIRST, BEFORE any editor launch, so a run with no
// way to produce a message fails loud rather than hangs or commits empty. It fires when:
//
//   - deps.Yes — an unattended -y run has no human to write a message; OR
//   - !deps.StdinInteractive — a non-TTY stdin (the startup-resolved signal) cannot host
//     the inherently-interactive editor; OR
//   - OpenEditor reports no launchable editor in git's chain (ErrNoEditor from 3-1, or
//     runner.ErrCommandNotFound at launch) on a TTY — there is no message to fall back to.
//
// All three collapse to errNoMessageSource (the spec's "no AI message and no interactive
// editor available"), surfaced and aborted — identically across all three converging
// triggers, since they all reach here. There is NO -m/--message escape.
//
// Past the guard, the SAVE is the accept event:
//
//   - Aborted/quit editor (ok=false) → a true no-op: errEditorNoOp, no mutation.
//   - Whitespace-only save ("empty") → a true no-op: errEditorNoOp, no mutation. There
//     is NO synthetic stub/comment scaffolding, so emptiness is purely whitespace-only
//     (no #-comment stripping — downstream 4-2 reuses this rule).
//   - Non-empty save = ACCEPT → apply the mode's deferred staging FIRST, then commit the
//     saved buffer, in that order, through the git_safe Mutator (stage→commit, the
//     same order the gate-accept path uses). Then, when -p is armed (5-1), the SAME
//     single shared push step the gate-accept path uses (pushAfterCommit, 5-2) runs —
//     so a non-empty save is a full accept: `mint commit -Ap --no-ai` stages, commits,
//     AND pushes. The push fires strictly after the stage→commit ordering completes and
//     is a no-op when push is disarmed.
func runEditorFallback(ctx context.Context, deps Deps, root, initial string) error {
	p := deps.Presenter

	// No-message-source guard, BEFORE any editor launch: an unattended -y run or a non-TTY
	// stdin (the startup-resolved StdinInteractive signal — the SAME determination the gate
	// uses, not a separate /dev/tty probe) cannot drive the interactive editor, so there is
	// no message to commit with. Fail loud rather than launch an editor, stage, commit, or
	// hang. This extends the gate's forbidden-combo philosophy to the editor path. The
	// editorUnavailable predicate is the SINGLE definition of this condition, shared with the
	// oversized note gate so "will the editor open?" cannot drift between them.
	if deps.editorUnavailable() {
		return surface(p, "editor", errNoMessageSource)
	}

	saved, ok, err := OpenEditor(ctx, deps.Runner, p, initial)
	if err != nil {
		// No launchable editor in git's chain on a TTY (ErrNoEditor from 3-1, or
		// runner.ErrCommandNotFound at launch) is the no-message-source case too — there is
		// no message to fall back to, so fail loud with the SAME spec message rather than
		// surface the raw resolution/launch error.
		if errors.Is(err, ErrNoEditor) || errors.Is(err, runner.ErrCommandNotFound) {
			return surface(p, "editor", errNoMessageSource)
		}
		// Any OTHER launch error (a genuine IO failure, etc.) keeps the existing surface.
		return surface(p, "editor", err)
	}
	if !ok {
		// Aborted/quit editor: a true no-op — nothing was mutated (the save IS the accept,
		// and there was no save). No StageFailed narration; just a non-zero abort.
		return errEditorNoOp
	}
	if isEmptyEditorBuffer(saved) {
		// Empty (whitespace-only) save: a true no-op. No synthetic stub exists, so "empty"
		// is purely whitespace-only — no #-comment stripping (the single 3-2 rule, shared
		// with the `e` gate action via isEmptyEditorBuffer).
		return errEditorNoOp
	}

	// Non-empty save = ACCEPT: run the SAME shared stage→commit→push accept tail the
	// gate-accept path uses (commitAccept), committing the saved buffer verbatim. Routing
	// through the one helper keeps the STAGE→COMMIT order, the single shared push step,
	// the warn-don't-unwind push handling, and the always-fire RunFinished identical
	// across both accept paths.
	return commitAccept(ctx, deps, root, saved)
}

// isEmptyEditorBuffer is the SINGLE editor-emptiness rule established in 3-2 and shared by
// every editor consumer: a saved buffer is "empty" iff it is whitespace-only (or has no
// content at all). There is NO synthetic stub or #-comment scaffolding — the buffer carries
// only the real message — so emptiness is purely whitespace-only; there is no comment
// stripping. Both editor paths consume this one rule, but ACT on it differently:
//
//   - runEditorFallback (Phase 3, no message exists yet): an empty save = a true no-op
//     abort (errEditorNoOp) — there is nothing to fall back to.
//   - reviewLoop's `e` gate action (Phase 4, a message ALREADY exists): an empty save =
//     a DISCARD of the edit + re-render with the prior message preserved — never an abort,
//     never a commit, so `e` can never produce an empty commit.
func isEmptyEditorBuffer(saved string) bool {
	return strings.TrimSpace(saved) == ""
}

// reviewLoop is commit's engine-owned Continue? review LOOP — the seam the gate's e
// (edit) re-entry needs. It owns the ShowMessage→Prompt render so each iteration shows
// the CURRENT message (the generated body first, the edited body after an `e`) then
// presents the gate, and translates the outcome:
//
//   - ChoiceYes (also the bare-Enter default, and the value the presenter returns
//     under -y after rendering its auto-accept echo) → accept: return (true, body, nil)
//     so Run proceeds to the commit sink with the FINAL body.
//
//   - ChoiceNo → decline: return (false, body, nil) so Run aborts as a true no-op.
//
//   - ChoiceEdit → open the editor pre-filled with the CURRENT body (via the reusable
//     OpenEditor roundtrip — 3-1 resolution + the internal SuspendSpinner/ResumeSpinner
//     bracket). A non-empty save replaces the in-memory body VERBATIM (no ComposePrompt,
//     no Generator, no L2 — the edit is never reprocessed) and LOOPS BACK: re-render and
//     re-prompt. It is NOT save-as-accept — nothing stages, commits, or pushes here.
//
//   - A Prompt error is the presenter's machine-readable failure channel:
//     ErrNotInteractive (the forbidden non-TTY-without-`-y` combination, ALREADY
//     rendered as a failure line by the presenter) and ErrInputClosed (EOF mid-gate,
//     NOT rendered by the presenter). ErrNotInteractive is wrapped and returned with
//     NO further narration (the presenter rendered it); ErrInputClosed is SURFACED via
//     the StageFailed helper (the presenter rendered nothing, so commit owns its
//     surfacing). Both preserve the underlying sentinel in the chain (errors.Is) and
//     map to a non-zero exit.
//
//   - Any other returned choice is a presenter-contract violation — surfaced and
//     aborted rather than silently treated as an accept.
//
//   - ChoiceRegen → regenerate the message with a one-time free-text context line.
//     The line is read via the presenter's AskLine free-text seam (the engine NEVER
//     reads stdin) — Enter submits, and an EMPTY line is legal (a plain re-roll, no
//     injected context). The line is injected ONE-TIME into the regeneration prompt
//     (ON TOP of any [commit].context) via regenerateMessage, which re-runs the
//     consumed generate path (its one retry consumed). On success the regenerated
//     body becomes the new candidate and LOOPS BACK: re-render and re-prompt. Like
//     `e` it is NOT an accept — nothing stages, commits, or pushes here. The line is
//     never persisted, and a subsequent `r` reads a FRESH line. An AskLine
//     ErrNotInteractive is PRE-rendered by the presenter (wrapped, no StageFailed);
//     ErrInputClosed (EOF) and any other AskLine error are surfaced by the engine. A
//     regeneration FAILURE (after the transport's one retry — an isAITransportFailure)
//     is — per the spec — routed to the SAME $EDITOR fallback as any other AI failure
//     (3-3): the loop returns the internal errRegenerateFallback sentinel and Run owns
//     the runEditorFallback call (the editor opens EMPTY/template, no re-show of the
//     pre-r message). A NON-transport regeneration error keeps a defensive surface-abort.
//
// The `e` and `r` actions are interactive-only: under -y the presenter auto-accepts
// (returns the Default ChoiceYes) and on a non-TTY Prompt returns ErrNotInteractive, so
// neither branch is reached in those modes — no special-casing is needed here.
func reviewLoop(ctx context.Context, deps Deps, cfg config.Config, root, body string) (bool, string, error) {
	p := deps.Presenter
	for {
		// Render-only contract: the presenter NEVER re-shows the message itself; re-showing
		// the (possibly edited) message is the engine's ShowMessage call. The gate Prompt
		// renders the MENU only.
		p.ShowMessage(presenter.Message{Title: commitMessageTitle, Body: body})

		choice, err := p.Prompt(commitReviewGate())
		if err != nil {
			// ErrNotInteractive is pre-rendered by the presenter — wrap and return WITHOUT a
			// StageFailed. Every other Prompt error (ErrInputClosed and any future case) is
			// unrendered by contract, so commit surfaces it via the StageFailed helper. Both
			// keep the sentinel in the chain (%w) for errors.Is and map to a non-zero exit.
			if errors.Is(err, presenter.ErrNotInteractive) {
				return false, body, fmt.Errorf("review gate: %w", err)
			}
			return false, body, surface(p, "review", err)
		}

		switch choice {
		case presenter.ChoiceYes:
			return true, body, nil
		case presenter.ChoiceNo:
			return false, body, nil
		case presenter.ChoiceEdit:
			// Open the editor pre-filled with the CURRENT body. OpenEditor consumes 3-1
			// ResolveEditor and brackets the $EDITOR hand-off with SuspendSpinner/ResumeSpinner
			// internally — reused, not re-bracketed here.
			saved, ok, oerr := OpenEditor(ctx, deps.Runner, p, body)
			if oerr != nil {
				// Not-launchable signal from 3-1 (ErrNoEditor from resolution, or
				// runner.ErrCommandNotFound at launch): GRACEFUL DEGRADE. A message candidate
				// already exists at the gate, so — unlike runEditorFallback's no-message-source
				// fail-loud (the MIRROR consumer of the SAME signal) — `e` must never fail loud,
				// abort, or commit. Warn that the editor could not launch and LOOP BACK with body
				// UNCHANGED: the next iteration re-renders ShowMessage(unedited) → Prompt with the
				// same y/n/e gate, so `e` is treated as a no-op. `e` is a refinement step that can
				// never produce an empty commit, so a non-launchable editor simply preserves the
				// existing message.
				if errors.Is(oerr, ErrNoEditor) || errors.Is(oerr, runner.ErrCommandNotFound) {
					p.Warn(presenter.Warning{Label: editorNoLaunchWarnLabel, Message: editorNoLaunchWarnMessage})
					continue
				}
				// Any OTHER OpenEditor error (a genuine IO failure, etc.) is unexpected — surface
				// and abort. OpenEditor's only err != nil cases ARE the not-launchable ones above,
				// so this branch is purely defensive.
				return false, body, surface(p, "editor", oerr)
			}
			// Save outcome under `e`, where a message ALREADY exists:
			//
			//   - Non-empty save (ok && !empty) → ADOPT: replace the in-memory body VERBATIM
			//     (no AI reprocessing) and loop back to re-render the refreshed message.
			//   - Empty/whitespace-only save, OR an aborted/quit editor (ok=false) → DISCARD:
			//     leave body UNCHANGED, so the loop-back re-renders the PRIOR message (the
			//     candidate shown before this `e`) — via ShowMessage(prior) → Prompt — with
			//     y/n/e still offered. The run CONTINUES; this is NOT the Phase 3 fallback's
			//     empty=abort (under `e` a message exists, so an empty save is a discard, not
			//     a no-op). Emptiness is the single 3-2 whitespace-only rule (isEmptyEditorBuffer);
			//     a quit/abort surfaces as ok=false. Because `e` only ever replaces an existing
			//     message on a non-empty save and otherwise preserves it, `e` is a refinement
			//     step that can NEVER produce an empty commit — it is never a message source.
			if ok && !isEmptyEditorBuffer(saved) {
				body = saved
			}
			// Loop back: re-render the (possibly refreshed) message and re-prompt.
		case presenter.ChoiceRegen:
			// Read a single free-text one-time context line via the presenter's AskLine
			// seam (the engine NEVER reads stdin). Enter submits; an EMPTY line is legal
			// (a plain re-roll, no injected context). The line is a FRESH local each
			// iteration, so a subsequent `r` reads a new line and never carries this one
			// forward — and it is never persisted to cfg/[commit].context.
			line, aerr := p.AskLine(regenContextPrompt)
			if aerr != nil {
				// Mirror the Prompt error handling: ErrNotInteractive is PRE-rendered by the
				// presenter (wrap WITHOUT a StageFailed); ErrInputClosed (EOF) and any other
				// AskLine error are unrendered by contract, so the engine surfaces them.
				if errors.Is(aerr, presenter.ErrNotInteractive) {
					return false, body, fmt.Errorf("regenerate context: %w", aerr)
				}
				return false, body, surface(p, "regenerate", aerr)
			}
			// Re-run the consumed generate path with the one-time context injected (line may
			// be "" → a plain re-roll). The transport's one retry is consumed inside
			// regenerateMessage; commit does not re-run it.
			regenerated, gerr := regenerateMessage(ctx, deps, cfg, root, line)
			if gerr != nil {
				// A regeneration FAILURE after the transport's one retry (an AI transport
				// failure: bad content surviving the retry, a timeout, or a missing binary) is
				// — per the spec — routed to the SAME $EDITOR fallback as any other AI failure
				// (3-3), NOT a re-show of the pre-r message. Return the internal routing
				// sentinel so Run owns the fallback call (the same entry point first-generation
				// failures use): a successful fallback commit exits 0 and never double-stages.
				if isAITransportFailure(gerr) {
					return false, body, errRegenerateFallback
				}
				// Any OTHER regeneration error (not a transport failure) keeps the defensive
				// surface-abort — it is unexpected and must fail loud rather than route to the
				// editor.
				return false, body, surface(p, "regenerate", gerr)
			}
			// Adopt the regenerated body as the new candidate and LOOP BACK: re-render
			// ShowMessage(regenerated) → Prompt with the same y/n/e/r gate. `r` is NOT an
			// accept — nothing stages, commits, or pushes here.
			body = regenerated
		default:
			// The gate declares y/n/e/r and the presenter returns a member of the declared set;
			// any other choice is a contract violation — fail loud rather than treat an unknown
			// answer as an accept.
			return false, body, surface(p, "review", fmt.Errorf("unexpected review-gate choice %q", choice))
		}
	}
}

// commitReviewGate is commit's HAND-BUILT Continue? gate literal. It is deliberately
// NOT presenter.NotesReviewGate()/ReuseConfirmGate() — those carry Subject "notes",
// which would make the -y auto-accept echo read "notes: accepted (-y)". Commit's
// Subject is "message" (echo "message: accepted (-y)") and AcceptEcho is "accepted".
// The gate offers y/n/e/r (Enter ⇒ y via Default ChoiceYes): the e (edit) action was
// added in Phase 4 (task 4-1) and the r (regenerate) action in task 4-4. Nothing in
// the presenter hardcodes the choice set, so the literal renders WHATEVER it declares.
// The action labels match the spec's gate mapping (accept & proceed / abort / edit /
// regenerate); the edit and regenerate labels match NotesReviewGate's "edit in
// $EDITOR" / "regenerate".
func commitReviewGate() presenter.Gate {
	return presenter.Gate{
		Question:   "Continue?",
		Subject:    "message",
		AcceptEcho: "accepted",
		Choices: []presenter.GateChoice{
			{Key: presenter.ChoiceYes, Action: "accept & proceed"},
			{Key: presenter.ChoiceNo, Action: "abort"},
			{Key: presenter.ChoiceEdit, Action: "edit in $EDITOR"},
			{Key: presenter.ChoiceRegen, Action: "regenerate"},
		},
		Default: presenter.ChoiceYes,
	}
}

// resolveRoot returns the pre-injected Root when set (the test seam) else resolves
// the repo root via gitrepo.ResolveRoot — production leaves Root empty and gets the
// real root, anchored the same way release resolves it.
func resolveRoot(ctx context.Context, deps Deps) (string, error) {
	if deps.Root != "" {
		return deps.Root, nil
	}
	return gitrepo.ResolveRoot(ctx, deps.Runner)
}

// generateMessage builds the L3 Generator over the run's read runner + resolved
// transport + root + resolved staging mode, then generates the conventional-commits
// body from the would-be-committed diff. The mode (deps.Staging) selects the L1 source:
// StagedOnly reads the index, All/AddAll compute the would-be-staged worktree diff
// read-only. The transport is the injected deps.Transport when set (the test seam),
// else the production ai.Transport over the run's runner driven by the validated
// cfg.AICommand — so production leaves deps.Transport nil and gets the real engine.
func generateMessage(ctx context.Context, deps Deps, cfg config.Config, root string) (string, error) {
	generator := NewGenerator(deps.Runner, commitTransport(deps, cfg), root, deps.Staging)
	return generator.Generate(ctx, cfg)
}

// regenerateMessage is the gate's `r` (regenerate-with-context) re-run: it builds the
// SAME L3 Generator over the run's seams as generateMessage and re-runs the consumed
// L1 → size-guard → compose → L2 path via GenerateWithContext, layering the one-time
// context line onto the prompt (ON TOP of any persisted [commit].context). The line is
// passed verbatim; an EMPTY line is a plain re-roll (no injected block). The transport's
// own one retry is consumed here exactly as the initial generate consumes it — commit
// does NOT re-run the retry. The one-time context is a local string only: it is never
// persisted to cfg/[commit].context, and a subsequent `r` passes a fresh line.
func regenerateMessage(ctx context.Context, deps Deps, cfg config.Config, root, oneTimeContext string) (string, error) {
	generator := NewGenerator(deps.Runner, commitTransport(deps, cfg), root, deps.Staging)
	return generator.GenerateWithContext(ctx, cfg, oneTimeContext)
}

// isAITransportFailure reports whether err is one of the THREE L2 transport-failure
// sentinels the generate step (1-3) surfaces wrapped: ai.ErrGenerationFailed (bad
// content surviving the transport's one retry), ai.ErrTimeout, or ai.ErrCommandMissing
// (both never retried). It is the single routing predicate for "the AI produced no
// usable message" → the shared $EDITOR fallback, deliberately distinct from the
// oversized-skip (notes.ErrDiffTooLarge, task 3-4) which is NOT a transport failure and
// keeps the surface abort. It is a NAMED predicate so Phase 4's `r` (regenerate)
// failure can reuse the SAME routing entry point.
func isAITransportFailure(err error) bool {
	return errors.Is(err, ai.ErrGenerationFailed) ||
		errors.Is(err, ai.ErrTimeout) ||
		errors.Is(err, ai.ErrCommandMissing)
}

// commitTransport resolves the L2 transport: the injected deps.Transport when set
// (the test seam), else the production ai.Transport over the run's runner. The
// validated cfg.AICommand drives the invocation (the documented top-level ai_command
// key): NewTransport whitespace-splits it into name + args and re-defaults an empty
// value to `claude -p`, so a zero-config run still uses the documented default.
func commitTransport(deps Deps, cfg config.Config) Transport {
	if deps.Transport != nil {
		return deps.Transport
	}
	return ai.NewTransport(deps.Runner, ai.Config{AICommand: cfg.AICommand})
}

// stageForMode applies the resolved StagingMode's deferred `git add` on the gate-accept
// path — the mutate-nothing-until-accept invariant deferred this until now, so the read
// phase (preflight + the read-only would-be-staged diff in generateMessage) never touched
// the index. It is the staging half of the STAGE→COMMIT order on accept:
//
//   - All (-a): `git add -u` — stage tracked modifications + deletions, NO untracked
//     (`git commit -a` semantics).
//   - AddAll (-A): `git add -A` — stage everything including untracked (the `git add .`
//     habit in one shot).
//   - StagedOnly (the default): NO `git add` — commit the existing index exactly as
//     staged (the Phase 1 path, unchanged).
//
// The staging `git add` is a MUTATION, so it flows through the SAME git_safe Mutator as
// the commit (never the raw runner): a contended/stale `.git` index lock is retried/cleared,
// so a background agent or editor briefly holding the lock cannot blow up the staging step.
// stdin is nil — `git add` reads no body.
// On failure the captured Result is returned alongside the error so the caller can
// pass git's stderr (e.g. a rejecting hook's output) through to the StageFailure.
func stageForMode(ctx context.Context, deps Deps) (runner.Result, error) {
	var args []string
	switch deps.Staging {
	case All:
		args = []string{"add", "-u"}
	case AddAll:
		args = []string{"add", "-A"}
	default:
		// StagedOnly: commit the index exactly as staged — no `git add`.
		return runner.Result{}, nil
	}

	res, err := deps.Mutator.Mutate(ctx, nil, "git", args...)
	if err != nil {
		return res, fmt.Errorf("staging changes: %w", err)
	}
	return res, nil
}

// createCommit creates the commit through the git_safe Mutator, piping the
// generated body on stdin via `git commit -F -`. The body is passed as raw BYTES so a
// lock-retry re-pipes the FULL payload (a shared io.Reader would be drained after the
// first attempt). Piping via -F - (rather than -m) keeps a multi-line body intact and
// avoids any shell-escaping of the verbatim message. The body reaches git
// BYTE-FOR-BYTE — no commit_prefix, no branding (commit_prefix stays a release-only
// concern). It runs NO `git add` itself: it commits the index as it stands, which the
// accept tail's stageForMode has already prepared for the -a/-A modes.
// On failure the captured Result is returned alongside the error so the caller can pass
// git's stderr — above all a pre-commit/commit-msg hook's rejection output, which is the
// only actionable explanation of the failure — through to the StageFailure.
func createCommit(ctx context.Context, deps Deps, body string) (runner.Result, error) {
	res, err := deps.Mutator.Mutate(ctx, []byte(body), "git", "commit", "-F", "-")
	if err != nil {
		return res, fmt.Errorf("creating commit: %w", err)
	}
	return res, nil
}

// commitAccept is the SINGLE shared accept tail both accept entry points end with — the
// gate-accept branch (Run) and the save-as-accept branch (runEditorFallback). It runs the
// ordered stage→commit→push mutation sequence ONCE so the spec's load-bearing invariant
// (mutate nothing until accept, STAGE→COMMIT ordering, never unwind after) lives in exactly
// one place and cannot drift between the two paths. Only the committed body differs between
// callers (the review loop's final body vs the editor's saved buffer), so it is the sole
// parameter:
//
//  1. stageForMode applies the mode's deferred `git add` FIRST (a no-op under StagedOnly);
//     a staging failure surfaces as a "stage" StageFailed and aborts.
//  2. createCommit pipes the body to `git commit -F -`; a commit failure surfaces as a
//     "commit" StageFailed and aborts.
//  3. pushAfterCommit runs the single shared auto-push (a no-op when -p is disarmed; on a
//     FAILED push it warns-don't-unwind — the commit is kept and it returns errPushFailed).
//  4. RunFinished ALWAYS fires: the commit IS the run's success, so the success close-out
//     runs even when the push failed (the warn-don't-unwind contract).
//  5. The pushErr is returned: nil on a no-push/successful-push run, errPushFailed on a
//     failed push → a non-zero exit with the commit left in place.
func commitAccept(ctx context.Context, deps Deps, root, body string) error {
	p := deps.Presenter
	if res, err := stageForMode(ctx, deps); err != nil {
		// git's captured stderr travels verbatim as the failure Output — the same
		// pass-through convention the push warn uses — so a hook rejection or git's own
		// diagnostic explains the failure instead of a bare exit status.
		return surfaceOutput(p, "stage", err, res.Stderr)
	}
	if res, err := createCommit(ctx, deps, body); err != nil {
		return surfaceOutput(p, "commit", err, res.Stderr)
	}
	pushErr := pushAfterCommit(ctx, deps)
	p.RunFinished(presenter.RunResult{Project: projectName(root)})
	return pushErr
}

// pushAfterCommit is commit's SINGLE shared auto-push step, called by BOTH accept
// paths after a successful commit (the gate-accept path here; the editor save-as-accept
// path in 5-3 reuses this exact routine — not a parallel implementation). It is gated
// on deps.Push: DISARMED (the default) is a no-op, so an unarmed run is unchanged; ARMED
// runs a PLAIN `git push` — current branch → its configured upstream, resolved entirely
// by git. mint adds NO special upstream logic: no -u/--set-upstream, no remote/branch
// arguments, no current-branch detection or upstream inference. The argv is exactly
// `git push`.
//
// The push is a git mutation/network op, so — like the commit and the deferred staging —
// it flows through the git_safe Mutator (NOT the raw runner): a contended/stale `.git`
// lock is retried/cleared. stdin is nil — push reads no body.
//
// Sequencing makes the "push only after a successful commit" guarantee free: callers
// invoke pushAfterCommit AFTER createCommit returns nil, and every no-commit path (a
// gate abort, a generate failure, an empty/aborted editor) returns BEFORE reaching it,
// so a run that produced no commit never pushes.
//
// NO PRE-PUSH / REMOTE-SYNC GATE (the Preflight & Safety drops, 5-5). The dropped gates
// — clean-working-tree, on-release-branch, remote-in-sync (behind/diverged), and any
// pre-push gate — stay dropped: there is deliberately NO `git fetch`, no @{upstream}
// rev-list/--count behind-ahead probe, and no remote-sync precheck before the push. mint
// attempts the plain `git push` DIRECTLY and REPORTS a failure (the warn-don't-unwind
// below), rather than gating the commit on push-ability — which is what lets
// `mint commit -Apy` run unattended end-to-end.
//
// WARN-DON'T-UNWIND on failure (the ONE place — both accept paths reach here). The
// commit already happened, so a push failure must NEVER unwind it: there is NO
// reset/revert/restore/unstage/amend and no destructive cleanup of any kind — the
// post-accept never-unwind invariant is absolute. On ANY push failure (rejected /
// non-fast-forward, remote moved, no upstream, network) mint does NOT classify the
// cause: it emits ONE generic warn via the consumed Presenter (Label "push", the
// generic 'commit is in place; re-run the push' message) with git's OWN captured
// stderr passed through VERBATIM as Warning.Output — so git's specific hint
// (set-upstream, non-fast-forward, …) stays visible without any mint reformatting. The
// Warn does NOT suppress the success close-out and sets no presenter failure state;
// the non-zero exit comes solely from returning errPushFailed (the commit succeeded —
// that IS the run's success — so the caller still fires RunFinished). Empty/aborted
// suppression (5-5) is already natural: push is only reached after a successful commit.
func pushAfterCommit(ctx context.Context, deps Deps) error {
	if !deps.Push {
		return nil
	}
	res, err := deps.Mutator.Mutate(ctx, nil, "git", "push")
	if err != nil {
		// One generic warn for ALL causes; git's stderr travels verbatim in Output (the
		// no-upstream "set an upstream" hint comes from git, never mint-authored text). No
		// surface/StageFailed — the warn narrates; errPushFailed only drives the exit code.
		deps.Presenter.Warn(presenter.Warning{
			Label:   pushFailWarnLabel,
			Message: pushFailWarnMessage,
			Output:  res.Stderr,
		})
		return errPushFailed
	}
	return nil
}
