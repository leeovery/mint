package commit_test

// Mutation-failure diagnostics: when `git add` or `git commit` fails, git's captured
// stderr must travel VERBATIM as the StageFailure.Output — the same pass-through
// convention the push warn uses. The canonical case is a pre-commit/commit-msg hook
// rejection: the hook's own output is the only actionable explanation of the failure,
// and without the pass-through the user sees a bare exit status.

import (
	"context"
	"strings"
	"testing"

	"mint/internal/commit"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// findStageFailed returns the first StageFailed event named stage, failing the test
// when none was emitted.
func findStageFailed(t *testing.T, rec *presentertest.RecordingPresenter, stage string) presentertest.Event {
	t.Helper()
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageFailed && ev.StageFailed.Name == stage {
			return ev
		}
	}
	t.Fatalf("kinds = %v, want a StageFailed named %q", rec.Kinds(), stage)
	return presentertest.Event{}
}

// TestRun_CommitHookRejection_StderrReachesFailureOutput proves a failing
// `git commit` (a pre-commit hook rejecting, exiting non-zero with its explanation on
// stderr) surfaces that stderr verbatim as the "commit" StageFailure.Output.
func TestRun_CommitHookRejection_StderrReachesFailureOutput(t *testing.T) {
	t.Parallel()

	const hookOutput = "pre-commit hook failed:\nlint: main.go:7: unused variable x\n"
	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},                                    // preflight probe (non-empty)
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work\n"}},            // L1 staged diff
		runner.ScriptedCall{Result: runner.Result{Stderr: hookOutput, ExitCode: 1}, Err: errExitOne}, // git commit -F - (hook rejects)
	)
	deps := newCommitDeps(rec, r, scriptedTransport("feat: rejected by hook"), t.TempDir())

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil for a failing git commit; want a surfaced abort")
	}

	ev := findStageFailed(t, rec, "commit")
	if !strings.Contains(ev.StageFailed.Output, "unused variable x") {
		t.Errorf("StageFailure.Output = %q, want git's stderr (the hook's rejection) passed through verbatim", ev.StageFailed.Output)
	}
}

// TestRun_StagingFailure_StderrReachesFailureOutput proves a failing deferred
// `git add -A` surfaces git's stderr verbatim as the "stage" StageFailure.Output.
func TestRun_StagingFailure_StderrReachesFailureOutput(t *testing.T) {
	t.Parallel()

	const gitStderr = "error: insufficient permission for adding an object to repository database .git/objects\n"
	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},                                     // tracked probe (non-empty, short-circuits)
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work\n"}},             // L1 tracked diff
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},                                        // L1 untracked enumeration (none)
		runner.ScriptedCall{Result: runner.Result{Stderr: gitStderr, ExitCode: 128}, Err: errExitOne}, // git add -A fails
	)
	deps := newCommitDeps(rec, r, scriptedTransport("feat: staging fails"), t.TempDir())
	deps.Staging = commit.AddAll

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil for a failing git add; want a surfaced abort")
	}

	ev := findStageFailed(t, rec, "stage")
	if !strings.Contains(ev.StageFailed.Output, "insufficient permission") {
		t.Errorf("StageFailure.Output = %q, want git's stderr passed through verbatim", ev.StageFailed.Output)
	}
}
