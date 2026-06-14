package commit_test

// Black-box proofs for the commit wiring site (commitTransport, task 2-4): a bare
// `mint commit` run must resolve BOTH its AI command and its timeout through the
// [commit] verb chain (cfg.AICommandFor(VerbCommit) / cfg.TimeoutFor(VerbCommit)), so
// a [commit].ai_command override drives the commit-message AI invocation and a
// zero-config run still resolves to the shipped default. These ride the REAL generate
// thread with deps.Transport NIL, so production's ai.Transport is built over the
// FakeRunner and the configured binary (not a scripted fake) is what gets invoked.

import (
	"context"
	"testing"

	"mint/internal/commit"
	"mint/internal/git"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// seedAIDiffThenCommit scripts the bare-commit git thread for a NIL-transport run
// (the production ai.Transport is built, so the AI binary is invoked separately and is
// NOT a git call): the empty-index preflight read (non-empty), the L1 `git diff
// --cached` read, then the `git commit -F -` sink. The AI binary's invocation is keyed
// by its own command name (mybot/sharedbot/claude) and seeded by each test.
func seedAIDiffThenCommit(diff string) *runner.FakeRunner {
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}}, // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},  // git diff --cached
		runner.ScriptedCall{}, // git commit -F -
	)
	return r
}

// realTransportDeps assembles production-shaped Deps with deps.Transport NIL, so Run
// builds the REAL ai.Transport over the FakeRunner and the configured ai_command's
// binary is invoked. The Root seam points config.Load at the test's .mint.toml.
func realTransportDeps(rec *presentertest.RecordingPresenter, r *runner.FakeRunner, root string) commit.Deps {
	return commit.Deps{
		Presenter: rec,
		Runner:    r,
		Mutator:   git.NewMutator(r, git.WithBackoff(func(int) {})),
		// Transport intentionally NIL: Run builds the production ai.Transport.
		Root: root,
	}
}

// aiInvocationStdin returns the recorded stdin of the first invocation whose name+args
// match the given command line, or "" if no such invocation was recorded.
func aiInvocationStdin(r *runner.FakeRunner, name string, args ...string) string {
	for _, inv := range r.Invocations() {
		if inv.Name != name || len(inv.Args) != len(args) {
			continue
		}
		match := true
		for i := range args {
			if inv.Args[i] != args[i] {
				match = false
				break
			}
		}
		if match {
			return inv.Stdin
		}
	}
	return ""
}

// invokedBinary reports whether any recorded invocation matched the given command line
// exactly (name + args).
func invokedBinary(r *runner.FakeRunner, name string, args ...string) bool {
	for _, inv := range r.Invocations() {
		if inv.Name != name || len(inv.Args) != len(args) {
			continue
		}
		match := true
		for i := range args {
			if inv.Args[i] != args[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// TestRun_AICommand_CommitVerbOverrideDrivesTransport proves the per-verb
// [commit].ai_command override (NOT the bare shared top-level key) drives the
// commit-message AI invocation: with a [commit].ai_command set, its binary+args are
// what the production transport runs — sourced through cfg.AICommandFor(VerbCommit).
// A shared top-level ai_command is ALSO present to prove the per-verb override WINS the
// resolution (verb override → shared → floor); seeding the shared binary would never be
// reached, and the unseeded `claude` default is asserted absent.
func TestRun_AICommand_CommitVerbOverrideDrivesTransport(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMintToml(t, root, "ai_command = \"sharedbot gen\"\n[commit]\nai_command = \"mybot gen --json\"\n")
	const body = "feat: per-verb override drove the call"
	r := seedAIDiffThenCommit("diff --git a/x b/x\n+work")
	// The [commit].ai_command override's BINARY (mybot) is the runner key. Seeding the
	// shared `sharedbot` or the default `claude` here would never be reached; an unseeded
	// `mybot` would error the run, so the seed proves the per-verb override drives the call.
	r.Seed("mybot", runner.Result{Stdout: body}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := commit.Run(context.Background(), realTransportDeps(rec, r, root)); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// The [commit].ai_command override — its binary AND its args — drove the AI call,
	// with the composed prompt piped on stdin.
	if got := aiInvocationStdin(r, "mybot", "gen", "--json"); got == "" {
		t.Errorf("[commit].ai_command %q was not invoked with a prompt on stdin", "mybot gen --json")
	}
	// Neither the shared top-level command nor the default was invoked — the per-verb
	// override won the resolution.
	if invokedBinary(r, "sharedbot", "gen") {
		t.Errorf("shared `sharedbot gen` was invoked despite a [commit].ai_command override; per-verb did not win")
	}
	if invokedBinary(r, "claude", "-p", "--model", "sonnet") {
		t.Errorf("default `claude -p --model sonnet` was invoked despite a [commit].ai_command override")
	}

	// The body the configured command returned still reaches the commit sink verbatim.
	commitInv := findCommitInvocation(t, r)
	if commitInv.Stdin != body {
		t.Errorf("commit body = %q, want the AI body %q verbatim", commitInv.Stdin, body)
	}
}

// TestRun_AICommand_NoCommitOverrideFallsToShared proves the resolution chain falls a
// commit run with NO [commit].ai_command to the shared top-level ai_command: with only a
// shared key set, that shared command's binary+args drive the commit-message AI
// invocation (cfg.AICommandFor(VerbCommit) resolves verb-absent → shared).
func TestRun_AICommand_NoCommitOverrideFallsToShared(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMintToml(t, root, "ai_command = \"mybot gen\"\n")
	const body = "feat: shared command drove the call"
	r := seedAIDiffThenCommit("diff --git a/x b/x\n+work")
	r.Seed("mybot", runner.Result{Stdout: body}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := commit.Run(context.Background(), realTransportDeps(rec, r, root)); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// With no [commit].ai_command, the shared `mybot gen` drove the AI call.
	if got := aiInvocationStdin(r, "mybot", "gen"); got == "" {
		t.Errorf("shared ai_command %q was not invoked with a prompt on stdin", "mybot gen")
	}
	if invokedBinary(r, "claude", "-p", "--model", "sonnet") {
		t.Errorf("default `claude -p --model sonnet` was invoked despite a shared ai_command")
	}
}

// TestRun_AICommand_DefaultDrivesTransport proves a zero-config commit (no .mint.toml)
// still resolves to the documented default `claude -p --model sonnet`: with no
// ai_command set, the production transport invokes the pinned default with the composed
// prompt on stdin — the per-verb resolution preserves the floor exactly.
func TestRun_AICommand_DefaultDrivesTransport(t *testing.T) {
	t.Parallel()

	root := t.TempDir() // no .mint.toml → zero-config
	const body = "feat: default command drove the call"
	r := seedAIDiffThenCommit("diff --git a/x b/x\n+work")
	r.Seed("claude", runner.Result{Stdout: body}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := commit.Run(context.Background(), realTransportDeps(rec, r, root)); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if got := aiInvocationStdin(r, "claude", "-p", "--model", "sonnet"); got == "" {
		t.Errorf("default `claude -p --model sonnet` was not invoked with a prompt on stdin")
	}
}
