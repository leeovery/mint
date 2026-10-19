package commit_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"mint/internal/commit"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// leanClaudeArgs is the argv the shipped default ai_command must reach claude with,
// written out literally so a drift in the constant or in its splitting fails here.
var leanClaudeArgs = []string{
	"-p", "--model", "sonnet",
	"--safe-mode", "--tools", "",
	"--strict-mcp-config", "--no-session-persistence",
	"--system-prompt", "Follow the instructions you are given and output only what they ask for.",
}

// invocationsNamed returns every recorded invocation of the named program, in order.
func invocationsNamed(r *runner.FakeRunner, name string) []runner.Invocation {
	var out []runner.Invocation
	for _, inv := range r.Invocations() {
		if inv.Name == name {
			out = append(out, inv)
		}
	}
	return out
}

// assertEveryArgv fails unless every invocation in invs carried exactly want.
func assertEveryArgv(t *testing.T, invs []runner.Invocation, want []string) {
	t.Helper()
	for i, inv := range invs {
		if !slices.Equal(inv.Args, want) {
			t.Errorf("%s invocation %d args = %q, want exactly %q", inv.Name, i, inv.Args, want)
		}
	}
}

func TestRun_ConfigWithoutAICommand_RunsLeanClaudeCommand(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMintToml(t, root, "[commit]\ncontext = 'house style'\n")
	r := seedAIDiffThenCommit("diff --git a/x b/x\n+work")
	r.Seed("claude", runner.Result{Stdout: "feat: lean default drove the call"}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := commit.Run(context.Background(), realTransportDeps(rec, r, root)); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	invs := invocationsNamed(r, "claude")
	if len(invs) != 1 {
		t.Fatalf("claude invoked %d time(s), want 1", len(invs))
	}
	assertEveryArgv(t, invs, leanClaudeArgs)
	if invs[0].Stdin == "" {
		t.Error("lean claude command was invoked without the composed prompt on stdin")
	}
}

func TestRun_ExplicitCommitAICommand_RunsVerbatimWithoutLeanFlags(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMintToml(t, root, "[commit]\nai_command = 'claude -p --model opus'\n")
	r := seedAIDiffThenCommit("diff --git a/x b/x\n+work")
	r.Seed("claude", runner.Result{Stdout: "feat: opus drove the call"}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := commit.Run(context.Background(), realTransportDeps(rec, r, root)); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	invs := invocationsNamed(r, "claude")
	if len(invs) != 1 {
		t.Fatalf("claude invoked %d time(s), want 1", len(invs))
	}
	assertEveryArgv(t, invs, []string{"-p", "--model", "opus"})
}

func TestRun_ClaudeRejectsLeanFlag_InvokesOnlyLeanAttemptAndRetry(t *testing.T) {
	t.Parallel()

	root := t.TempDir() // no .mint.toml
	r := seedAIDiffThenCommit("diff --git a/x b/x\n+work")
	r.Seed("claude",
		runner.Result{Stderr: "error: unknown option '--safe-mode'\n", ExitCode: 1},
		errors.New("exit status 1"))
	rec := &presentertest.RecordingPresenter{}
	deps := realTransportDeps(rec, r, root)
	deps.Yes = true // unattended: the failure ends the run instead of opening an editor

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil, want the unattended generation failure")
	}

	invs := invocationsNamed(r, "claude")
	if len(invs) != 2 {
		t.Fatalf("claude invoked %d time(s), want 2 (first attempt and the one retry)", len(invs))
	}
	assertEveryArgv(t, invs, leanClaudeArgs)
}
