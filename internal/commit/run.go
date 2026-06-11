package commit

// This file is commit's ORCHESTRATOR — the Phase 1 walking-skeleton vertical seam
// that threads the shipped pieces (config [commit] table, the prompt composer, and
// the L3 Generator) into a runnable bare `mint commit`. It owns ORDERING for the
// bare (staged-only, no-flags) path: resolve the repo root, load config, GENERATE the
// conventional-commits message from the staged diff (L3), then CREATE the commit
// carrying that body verbatim through the lock-resilient git_safe sink. The pieces
// themselves are unchanged — Run CALLS them; it never re-implements their logic.
//
// SCOPE (Phase 1 bare path only). The deliberate sequencing points for later tasks
// are LEFT CLEAN, not built here:
//   - Review gate (task 1-5) slots between Generate and the commit sink.
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
	"fmt"

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
//     body returns verbatim. [Review gate is task 1-5 — its slot is HERE, between
//     generate and the commit sink.]
//  3. Create the commit through the git_safe Committer, piping the generated body on
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

	// Empty-index preflight (task 1-6) slots HERE — before generate, so the AI is never
	// invoked on an empty staged diff.

	body, err := generateMessage(ctx, deps, cfg, root)
	if err != nil {
		return surface(p, "generate", err)
	}

	// The minted message is shown for review. The interactive review gate (task 1-5)
	// slots HERE — between showing the message and the commit sink — so an accept
	// proceeds to the commit and a decline aborts before any mutation.
	p.ShowMessage(presenter.Message{Title: commitMessageTitle, Body: body})

	if err := createCommit(ctx, deps, body); err != nil {
		return surface(p, "commit", err)
	}

	p.RunFinished(presenter.RunResult{Project: projectName(root)})
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
