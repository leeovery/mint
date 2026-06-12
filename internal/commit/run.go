package commit

// This file is commit's ORCHESTRATOR — the Phase 1 walking-skeleton vertical seam
// that threads the shipped pieces (config [commit] table, the prompt composer, and
// the L3 Generator) into a runnable bare `mint commit`. It owns ORDERING for the
// bare (staged-only, no-flags) path: resolve the repo root, load config, GENERATE the
// conventional-commits message from the staged diff (L3), then CREATE the commit
// carrying that body verbatim through the lock-resilient git_safe sink. The pieces
// themselves are unchanged — Run CALLS them; it never re-implements their logic.
//
// SCOPE (Phase 1 bare path only). The review gate (task 1-5) is now wired between
// Generate and the commit sink: the minted message renders, then the Continue? gate
// is presented through the consumed Presenter — accept on y/Enter proceeds to the
// commit, decline on n aborts as a true no-op, -y auto-accepts (presenter-internal),
// and the non-TTY-without-`-y` forbidden combination fails loud. The remaining
// sequencing points for later tasks are LEFT CLEAN, not built here:
//   - Empty-index preflight (task 1-6) slots before Generate.
//   - -a/-A staging (Phase 2), --no-ai/$EDITOR (Phase 3), gate e/r (Phase 4), and
//     -p push (Phase 5) are NOT implemented. The bare path is STAGED-ONLY — it runs
//     NO `git add`; the only mutation is the commit itself.
//
// git_safe is non-negotiable: the commit mutation flows through the Committer seam
// (the consumed lock-resilient *git.Mutator in production), NEVER the raw runner — a
// contended/stale .git lock is retried/cleared so a background agent or editor
// briefly holding the index lock cannot blow up a commit.

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

// Committer is commit's git-mutation SINK seam: the bare commit's `git commit -F -`
// flows through it so the mutation is lock-resilient (git_safe), NEVER the raw runner.
// It is defined HERE — where it is consumed — so the orchestrator stays decoupled from
// the git package's concretion: *git.Mutator satisfies it in production (carrying the
// retry + stale-lock discrimination), while tests inject a fake/real Mutator over a
// FakeRunner. The signature mirrors git.Mutator.Mutate exactly: the message body is
// passed as raw BYTES on stdin (a retry must re-pipe the full payload), and the argv
// follows. Read-only git (commit's L1 staged-diff) stays on the plain Runner — only
// the mutation needs the wrapper.
type Committer interface {
	// Mutate runs a git mutation with lock resilience, piping stdin (the commit body)
	// as raw bytes when non-nil. It returns the runner Result and a non-nil error on a
	// non-lock failure or an exhausted retry budget.
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
	// Committer is the lock-resilient git MUTATION sink (git_safe). The commit — the
	// bare path's ONLY mutation — flows through it; production wires *git.Mutator
	// constructed once over the same Runner.
	Committer Committer
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
	// -a/-A flags into it). The zero value StagedOnly is the Phase 1 default — commit
	// the index exactly as staged, running NO `git add`. The All/AddAll deferred-staging
	// behaviour (compute the would-be-committed diff read-only, then `git add` only on
	// gate-accept) is Phase 2 (tasks 2-2/2-3) and consumes this field; this task only
	// THREADS the resolved value through, leaving the StagedOnly path byte-identical.
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
	// Run never reads a config push key. This task only THREADS the armed value through;
	// the actual push execution (after the gate-accept commit and the editor-save commit),
	// the push-failure warn-don't-unwind, and the empty/aborted-run suppression are LATER
	// Phase 5 tasks (5-2..5-5) that consume this field — none run here.
	Push bool
}

// Run executes the bare `mint commit` thread, orchestrating the Phase 1 pieces in
// EXACTLY this order:
//
//  1. Resolve the repo root (anchored the same way release resolves it) and load
//     config (for the shared engine keys + the [commit] table). [Empty-index
//     preflight is task 1-6 — its slot is BEFORE generate.]
//  2. Generate the conventional-commits message via the L3 Generator: it assembles
//     the staged diff (commit's L1), applies the size guard, composes the prompt, and
//     calls the L2 transport. The AI infers the type; scope is off by default; the
//     body returns verbatim.
//  3. Show the minted message (ShowMessage) and present the Continue? review gate
//     (reviewLoop) — BOTH before the commit sink, so an accept proceeds and a
//     decline aborts with nothing mutated (the mutate-nothing-until-accept invariant).
//  4. Create the commit through the git_safe Committer, piping the generated body on
//     stdin (`git commit -F -`) so the commit message is the minted body BYTE-FOR-BYTE
//     — no commit_prefix, no branding, no reformatting.
//
// A failure at any step surfaces through the presenter and aborts with a non-zero exit
// (an *engine-style abort is owned by the cmd layer's exit mapping). The bare path is
// staged-only: Run issues NO `git add`.
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
	// non-empty would-be-staged set proceeds.
	if err := checkSomethingToCommit(ctx, deps.Runner, deps.Staging); err != nil {
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
		// transport was never reached). A routine large commit must not abort — emit the
		// oversized NOTE via the Presenter, then route to the SAME editor fallback as
		// --no-ai (empty buffer, save-as-accept). This carries the note; the AI-failure
		// branch below does NOT.
		if errors.Is(err, notes.ErrDiffTooLarge) {
			p.Warn(presenter.Warning{Label: oversizedNoteLabel, Message: oversizedNoteMessage})
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

	// On accept, apply the mode's deferred staging FIRST, then commit — in that order.
	// This is the mutate-nothing-until-accept invariant's mutation half: everything up to
	// here was read-only, so STAGE→COMMIT is the only point the index is touched. The
	// committed body is the FINAL body from the review loop (the edited text verbatim
	// when the user pressed `e`, else the generated message).
	if err := stageForMode(ctx, deps); err != nil {
		return surface(p, "stage", err)
	}

	if err := createCommit(ctx, deps, finalBody); err != nil {
		return surface(p, "commit", err)
	}

	// Auto-push (Phase 5): the commit succeeded, so when -p is armed push it now —
	// AFTER the commit, gated strictly on its success. pushAfterCommit is a no-op when
	// push is disarmed (pushErr nil), and on a FAILED push it warns-don't-unwind (the
	// commit is kept; it emits the generic warn and returns errPushFailed). The commit
	// IS the run's success, so RunFinished ALWAYS fires regardless; a failed push then
	// returns the sentinel → non-zero exit, with the commit left in place.
	pushErr := pushAfterCommit(ctx, deps)
	p.RunFinished(presenter.RunResult{Project: projectName(root)})
	return pushErr
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
//     saved buffer, in that order, through the git_safe Committer (stage→commit, the
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
	// hang. This extends the gate's forbidden-combo philosophy to the editor path.
	if deps.Yes || !deps.StdinInteractive {
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

	// Non-empty save = ACCEPT: apply the mode's deferred staging FIRST, then commit the
	// saved buffer verbatim — the same STAGE→COMMIT order the gate-accept path uses.
	if err := stageForMode(ctx, deps); err != nil {
		return surface(p, "stage", err)
	}
	if err := createCommit(ctx, deps, saved); err != nil {
		return surface(p, "commit", err)
	}

	// Auto-push (Phase 5): the save-as-accept commit succeeded, so — exactly as the
	// gate-accept path does — when -p is armed push it now via the SAME single shared
	// push step (pushAfterCommit, not a parallel push), which handles a failure with the
	// SAME warn-don't-unwind behaviour (the commit is kept; one generic warn; git's
	// stderr verbatim; errPushFailed drives the non-zero exit). The push fires strictly
	// after the stage->commit ordering above. RunFinished ALWAYS fires (the commit is the
	// run's success); a failed push returns the sentinel, leaving the commit in place.
	pushErr := pushAfterCommit(ctx, deps)
	p.RunFinished(presenter.RunResult{Project: projectName(root)})
	return pushErr
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

// The empty-staging preflight failures. Each message is rendered VERBATIM by commit's
// surface helper (it renders cause.Error()), so the user sees the exact git-style line
// with no mint wrapping. All three are returned UNWRAPPED so that verbatim text survives
// to the presenter, and all carry lowercase, punctuation-free messages mirroring git's
// own diagnostics (per the spec's Empty-staging handling). Which one fires is keyed on
// the ACTUAL post-mode tree state, NOT the flag passed:
//
//   - errNothingToCommit — git's own clean-tree line VERBATIM: the tree is genuinely
//     clean (nothing anywhere), so the chosen mode had nothing it could ever stage.
//   - errNoChangesStaged — bare `mint commit` with unstaged changes but nothing staged:
//     guide the user to the staging modes (mint's flavour of git's "no changes added to
//     commit"). The em dash is U+2014.
//   - errNoTrackedChanges — `mint commit -a` when the only changes are untracked, so the
//     tracked-only -a staged nothing: point specifically at -A/--add-all (the mode that
//     would include them). The em dash is U+2014.
var (
	errNothingToCommit  = errors.New("nothing to commit, working tree clean")
	errNoChangesStaged  = errors.New("no changes staged — use -a/--all, -A/--add-all, or git add")
	errNoTrackedChanges = errors.New("no tracked changes to stage — use -A/--add-all to include untracked files")
)

// checkSomethingToCommit is commit's staging-mode-aware "something to commit" preflight.
// It computes the would-be-staged emptiness for the resolved StagingMode READ-ONLY (no
// `git add`, no AI) and fails loud when that set is empty, short-circuiting generation so
// the AI is never invoked on an empty diff. All probes go through the consumed
// CommandRunner seam (the same read-only idiom as generate's source helpers), so they are
// fully scriptable via the FakeRunner.
//
// A NON-empty would-be-staged set returns nil → the run proceeds to generate (as before).
// An EMPTY set selects the failure by the ACTUAL post-mode tree state (probed once with a
// read-only `git status --porcelain`), NOT the flag passed — so `mint commit -A` on a
// pristine tree yields the clean-tree message, because an empty -A set means a clean tree.
func checkSomethingToCommit(ctx context.Context, r runner.CommandRunner, mode StagingMode) error {
	empty, err := wouldStageNothing(ctx, r, mode)
	if err != nil {
		return err
	}
	if !empty {
		return nil
	}
	return emptyStagingError(ctx, r, mode)
}

// wouldStageNothing reports whether the resolved StagingMode would stage nothing,
// computed READ-ONLY from name-only probes (sufficient for emptiness — no diff body is
// needed). Only the source command differs per mode, mirroring generate's sourceDiff:
//
//   - StagedOnly: empty iff `git diff --cached --name-only` is empty (the staged index).
//   - All (-a): empty iff `git diff HEAD --name-only` is empty (tracked mods + deletions).
//   - AddAll (-A): empty iff BOTH `git diff HEAD --name-only` AND `git ls-files --others
//     --exclude-standard` are empty (tracked changes AND untracked files).
//
// A genuine git failure is wrapped and surfaced so it is never mistaken for an empty set.
func wouldStageNothing(ctx context.Context, r runner.CommandRunner, mode StagingMode) (bool, error) {
	switch mode {
	case All:
		return gitOutputEmpty(ctx, r, "diff", "HEAD", "--name-only")
	case AddAll:
		trackedEmpty, err := gitOutputEmpty(ctx, r, "diff", "HEAD", "--name-only")
		if err != nil {
			return false, err
		}
		if !trackedEmpty {
			return false, nil
		}
		return gitOutputEmpty(ctx, r, "ls-files", "--others", "--exclude-standard")
	default:
		return gitOutputEmpty(ctx, r, "diff", "--cached", "--name-only")
	}
}

// emptyStagingError selects the fail-loud cause for an empty would-be-staged set, keyed on
// the ACTUAL tree state (a read-only `git status --porcelain`), NOT the flag passed:
//
//   - Genuinely clean tree (status empty → nothing anywhere) → errNothingToCommit. An
//     empty -A set ALWAYS lands here (if -A staged nothing, the tree is clean).
//   - Changes exist but the chosen mode staged none (status non-empty):
//   - StagedOnly (bare) → errNoChangesStaged.
//   - All (-a) → errNoTrackedChanges (only untracked remain — tracked changes would have
//     been captured by -a, so an empty -a set with changes present means they are
//     untracked; point at -A/--add-all).
//   - AddAll (-A) → unreachable (an empty -A set ⇒ a clean tree); defensively return the
//     clean-tree message.
func emptyStagingError(ctx context.Context, r runner.CommandRunner, mode StagingMode) error {
	clean, err := gitOutputEmpty(ctx, r, "status", "--porcelain")
	if err != nil {
		return err
	}
	if clean {
		return errNothingToCommit
	}

	switch mode {
	case All:
		return errNoTrackedChanges
	case AddAll:
		// Unreachable: an empty -A would-be-staged set implies a clean tree, handled above.
		// Defensive fall-back to the clean-tree message keeps the function total.
		return errNothingToCommit
	default:
		return errNoChangesStaged
	}
}

// gitOutputEmpty runs a READ-ONLY git command and reports whether its trimmed stdout is
// empty. It is the shared probe of the emptiness checks: a genuine git failure is wrapped
// and surfaced (never mistaken for an empty result).
func gitOutputEmpty(ctx context.Context, r runner.CommandRunner, args ...string) (bool, error) {
	res, err := r.Run(ctx, "git", args...)
	if err != nil {
		return false, fmt.Errorf("checking %v: %w", args, err)
	}
	return strings.TrimSpace(res.Stdout) == "", nil
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
// The staging `git add` is a MUTATION, so it flows through the SAME git_safe Committer as
// the commit (never the raw runner): a contended/stale `.git` index lock is retried/cleared,
// so a background agent or editor briefly holding the lock cannot blow up the staging step.
// stdin is nil — `git add` reads no body.
func stageForMode(ctx context.Context, deps Deps) error {
	var args []string
	switch deps.Staging {
	case All:
		args = []string{"add", "-u"}
	case AddAll:
		args = []string{"add", "-A"}
	default:
		// StagedOnly: commit the index exactly as staged — no `git add`.
		return nil
	}

	if _, err := deps.Committer.Mutate(ctx, nil, "git", args...); err != nil {
		return fmt.Errorf("staging changes: %w", err)
	}
	return nil
}

// createCommit creates the commit through the git_safe Committer, piping the
// generated body on stdin via `git commit -F -`. The body is passed as raw BYTES so a
// lock-retry re-pipes the FULL payload (a shared io.Reader would be drained after the
// first attempt). Piping via -F - (rather than -m) keeps a multi-line body intact and
// avoids any shell-escaping of the verbatim message. The body reaches git
// BYTE-FOR-BYTE — no commit_prefix, no branding (commit_prefix stays a release-only
// concern). No `git add` runs: the bare path commits the index exactly as staged.
func createCommit(ctx context.Context, deps Deps, body string) error {
	if _, err := deps.Committer.Mutate(ctx, []byte(body), "git", "commit", "-F", "-"); err != nil {
		return fmt.Errorf("creating commit: %w", err)
	}
	return nil
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
// it flows through the git_safe Committer (NOT the raw runner): a contended/stale `.git`
// lock is retried/cleared. stdin is nil — push reads no body.
//
// Sequencing makes the "push only after a successful commit" guarantee free: callers
// invoke pushAfterCommit AFTER createCommit returns nil, and every no-commit path (a
// gate abort, a generate failure, an empty/aborted editor) returns BEFORE reaching it,
// so a run that produced no commit never pushes.
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
	res, err := deps.Committer.Mutate(ctx, nil, "git", "push")
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
