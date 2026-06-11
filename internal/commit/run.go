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
	"mint/internal/presenter"
	"mint/internal/runner"
)

// commitAction is the engine-supplied verb word for the start-of-run header.
const commitAction = "committing"

// commitMessageTitle labels the ShowMessage delimiters for the minted commit
// message. It is ASCII so the plain-mode delimiters stay byte-pure (the same
// convention as the gate Subject/AcceptEcho values).
const commitMessageTitle = "commit message"

// errGateAborted is the clean cause for a user `n` decline at the review gate. It is
// a TRUE no-op cause — nothing was mutated (the gate sits before the commit sink) —
// so the abort path emits NO StageFailed failure narration. It surfaces only so the
// run exits non-zero (mapped to exit 1 by cmd/mint's exitCode), telling a caller the
// commit did not happen; it is distinct from the AI/transport failures that DO
// narrate a StageFailed.
var errGateAborted = errors.New("commit aborted at review gate")

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
//     (reviewMessage) — BOTH before the commit sink, so an accept proceeds and a
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

	// Empty-index preflight (task 1-6) runs HERE — before generate, so the AI is never
	// invoked on an empty staged diff. A genuinely empty index fails loud with git's
	// clean-tree message; a non-empty index proceeds.
	if err := checkStagedIndexNonEmpty(ctx, deps.Runner); err != nil {
		return surface(p, "preflight", err)
	}

	body, err := generateMessage(ctx, deps, cfg, root)
	if err != nil {
		return surface(p, "generate", err)
	}

	// The minted message is shown for review, THEN the Continue? gate is presented —
	// both BEFORE the commit sink, so the core invariant (mutate nothing until accept)
	// holds: a decline aborts with nothing mutated. The render order is verbatim
	// message first, gate second.
	p.ShowMessage(presenter.Message{Title: commitMessageTitle, Body: body})

	accepted, err := reviewMessage(p)
	if err != nil {
		return err
	}
	if !accepted {
		// A clean `n` decline: a TRUE no-op (nothing mutated — the gate precedes the
		// commit). Surface a non-zero abort WITHOUT a StageFailed — a deliberate decline
		// is not a failure, so it emits no failure narration.
		return errGateAborted
	}

	if err := createCommit(ctx, deps, body); err != nil {
		return surface(p, "commit", err)
	}

	p.RunFinished(presenter.RunResult{Project: projectName(root)})
	return nil
}

// reviewMessage presents the Continue? gate through the consumed Presenter and maps
// the outcome to an accept/abort decision. It is commit's decision seam — modelled on
// engine.ReviewDecision: it calls p.Prompt(gate) (the single render-and-read entry
// point — the gate rendering, line-read input, -y auto-accept echo, and
// forbidden-combo fail-loud are ALL consumed from the presenter, never re-implemented
// here) and translates the result:
//
//   - ChoiceYes (also the bare-Enter default, and the value the presenter returns
//     under -y after rendering its auto-accept echo) → accept: return (true, nil) so
//     Run proceeds to the commit sink.
//   - ChoiceNo → decline: return (false, nil) so Run aborts as a true no-op.
//   - A Prompt error is the presenter's machine-readable failure channel:
//     ErrNotInteractive (the forbidden non-TTY-without-`-y` combination, ALREADY
//     rendered as a failure line by the presenter) and ErrInputClosed (EOF mid-gate,
//     NOT rendered by the presenter). ErrNotInteractive is wrapped and returned with
//     NO further narration (the presenter rendered it); ErrInputClosed is SURFACED via
//     the StageFailed helper (the presenter rendered nothing, so commit owns its
//     surfacing). Both preserve the underlying sentinel in the chain (errors.Is) and
//     map to a non-zero exit.
//   - Any other returned choice is a presenter-contract violation (Phase 1 declares
//     only y/n) — surfaced and aborted rather than silently treated as an accept.
func reviewMessage(p presenter.Presenter) (bool, error) {
	choice, err := p.Prompt(commitReviewGate())
	if err != nil {
		// ErrNotInteractive is pre-rendered by the presenter — wrap and return WITHOUT a
		// StageFailed. Every other Prompt error (ErrInputClosed and any future case) is
		// unrendered by contract, so commit surfaces it via the StageFailed helper. Both
		// keep the sentinel in the chain (%w) for errors.Is and map to a non-zero exit.
		if errors.Is(err, presenter.ErrNotInteractive) {
			return false, fmt.Errorf("review gate: %w", err)
		}
		return false, surface(p, "review", err)
	}

	switch choice {
	case presenter.ChoiceYes:
		return true, nil
	case presenter.ChoiceNo:
		return false, nil
	default:
		// The gate declares only y/n and the presenter returns a member of the declared
		// set; any other choice is a contract violation — fail loud rather than treat an
		// unknown answer as an accept.
		return false, surface(p, "review", fmt.Errorf("unexpected review-gate choice %q", choice))
	}
}

// commitReviewGate is commit's HAND-BUILT Continue? gate literal. It is deliberately
// NOT presenter.NotesReviewGate()/ReuseConfirmGate() — those carry Subject "notes",
// which would make the -y auto-accept echo read "notes: accepted (-y)". Commit's
// Subject is "message" (echo "message: accepted (-y)") and AcceptEcho is "accepted".
// Phase 1 offers y/n ONLY (Enter ⇒ y via Default ChoiceYes); the e (edit) and r
// (regenerate) actions are Phase 4. Nothing in the presenter hardcodes the choice
// set, so a two-choice literal renders cleanly. The action labels match the spec's
// gate mapping (accept & proceed / abort).
func commitReviewGate() presenter.Gate {
	return presenter.Gate{
		Question:   "Continue?",
		Subject:    "message",
		AcceptEcho: "accepted",
		Choices: []presenter.GateChoice{
			{Key: presenter.ChoiceYes, Action: "accept & proceed"},
			{Key: presenter.ChoiceNo, Action: "abort"},
		},
		Default: presenter.ChoiceYes,
	}
}

// errNothingToCommit is commit's empty-index preflight failure. Its message is git's
// own clean-tree line VERBATIM ("nothing to commit, working tree clean", per the
// spec's Empty-staging handling): commit's surface helper renders cause.Error(), so
// the user sees this exact git-style line with no mint wrapping. It is returned
// UNWRAPPED so that verbatim text survives to the presenter. Phase 1 covers only the
// bare (staged-only) path — the flag-aware empty-staging variants (-a/-A) are Phase 2.
var errNothingToCommit = errors.New("nothing to commit, working tree clean")

// checkStagedIndexNonEmpty is commit's minimal "something to commit" preflight for
// the bare (staged-only) path: it reads the staged index READ-ONLY via `git diff
// --cached --name-only` and fails loud when the index is empty. An empty stdout means
// nothing is staged → errNothingToCommit (git's exact clean-tree line). Non-empty
// stdout means there is staged content → the check passes and the run proceeds to
// generate. The read goes through the consumed CommandRunner seam (same seam/idiom as
// generate's stagedDiff), so it is fully scriptable via the FakeRunner; a genuine git
// failure is wrapped and surfaced as-is so it is never mistaken for an empty index.
//
// This runs BEFORE message generation and short-circuits it, so the AI is never
// invoked on an empty diff. It is the staged-only check ONLY: the -a/-A would-be-staged
// distinctions and the richer empty-staging messaging are Phase 2.
func checkStagedIndexNonEmpty(ctx context.Context, r runner.CommandRunner) error {
	res, err := r.Run(ctx, "git", "diff", "--cached", "--name-only")
	if err != nil {
		return fmt.Errorf("checking staged index: %w", err)
	}

	if strings.TrimSpace(res.Stdout) == "" {
		return errNothingToCommit
	}
	return nil
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
// transport + root, then generates the conventional-commits body from the staged
// diff. The transport is the injected deps.Transport when set (the test seam), else
// the production ai.Transport over the run's runner driven by the validated
// cfg.AICommand — so production leaves deps.Transport nil and gets the real engine.
func generateMessage(ctx context.Context, deps Deps, cfg config.Config, root string) (string, error) {
	generator := NewGenerator(deps.Runner, commitTransport(deps, cfg), root)
	return generator.Generate(ctx, cfg)
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
