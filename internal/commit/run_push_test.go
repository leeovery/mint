package commit_test

import (
	"context"
	"testing"

	"mint/internal/commit"
	"mint/internal/git"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// pushInvocations returns every recorded `git push` invocation, in order — the
// auto-push step the accept path runs through the git_safe Committer after a
// successful commit when -p is armed.
func pushInvocations(r *runner.FakeRunner) []runner.Invocation {
	var pushes []runner.Invocation
	for _, inv := range r.Invocations() {
		if inv.Name == "git" && len(inv.Args) > 0 && inv.Args[0] == "push" {
			pushes = append(pushes, inv)
		}
	}
	return pushes
}

// seedDiffThenCommitThenPush scripts the gate-accept thread's git invocations IN
// ORDER for an armed -p run: the empty-index preflight read (non-empty), the L1
// `git diff --cached` read, the `git commit -F -` mutation, then the `git push`
// mutation that follows a successful commit.
func seedDiffThenCommitThenPush(diff string) *runner.FakeRunner {
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}}, // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},  // git diff --cached
		runner.ScriptedCall{}, // git commit -F -
		runner.ScriptedCall{}, // git push
	)
	return r
}

// TestRun_PushArmed_PushesAfterGateAcceptCommit proves that with -p armed
// (Deps.Push true) a plain `git push` runs via the git_safe Committer AFTER a
// successful gate-accept commit. The push is recorded on the SAME runner the
// Mutator wraps — proof it flows through the Committer seam, not a side channel.
func TestRun_PushArmed_PushesAfterGateAcceptCommit(t *testing.T) {
	t.Parallel()

	const message = "feat: push after commit"
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommitThenPush("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	pushes := pushInvocations(r)
	if len(pushes) != 1 {
		t.Fatalf("git push invocations = %d (%v), want exactly 1 (a plain push after the commit)", len(pushes), pushes)
	}
}

// TestRun_PushArmed_PlainPushNoUpstreamArgs proves the auto-push is a PLAIN
// `git push` — argv exactly ["push"], with NO upstream/remote/branch arguments and
// no -u/--set-upstream (mint defers all upstream handling to git). The push also
// carries no stdin (push has no body).
func TestRun_PushArmed_PlainPushNoUpstreamArgs(t *testing.T) {
	t.Parallel()

	const message = "feat: plain push"
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommitThenPush("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	pushes := pushInvocations(r)
	if len(pushes) != 1 {
		t.Fatalf("git push invocations = %d (%v), want exactly 1", len(pushes), pushes)
	}
	if got := pushes[0].Args; len(got) != 1 || got[0] != "push" {
		t.Errorf("push argv = %v, want exactly [\"push\"] (no upstream/remote/branch args, no -u/--set-upstream)", got)
	}
	if pushes[0].Stdin != "" {
		t.Errorf("push stdin = %q, want empty (push carries no body)", pushes[0].Stdin)
	}
}

// TestRun_PushArmed_RunsStrictlyAfterCommit proves the push runs strictly AFTER the
// commit succeeds: the recorded git invocation order has the commit before the push.
func TestRun_PushArmed_RunsStrictlyAfterCommit(t *testing.T) {
	t.Parallel()

	const message = "feat: ordered push"
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommitThenPush("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	commitIdx := indexOfGitArgs(r, "commit")
	pushIdx := indexOfGitArgs(r, "push")
	if commitIdx < 0 || pushIdx < 0 {
		t.Fatalf("git calls = %v, want both a commit and a push", gitInvocations(r))
	}
	if commitIdx >= pushIdx {
		t.Errorf("commit at %d, push at %d; the push must run strictly AFTER the commit", commitIdx, pushIdx)
	}
}

// TestRun_PushUnarmed_NoPushAfterCommit proves that with -p UNARMED (Deps.Push
// false, the default) NO push runs after a successful commit — zero push
// invocations.
func TestRun_PushUnarmed_NoPushAfterCommit(t *testing.T) {
	t.Parallel()

	const message = "feat: no push when unarmed"
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommit("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())
	// Push left at the zero value (false): no push.

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// The commit still ran.
	findCommitInvocation(t, r)
	if pushes := pushInvocations(r); len(pushes) != 0 {
		t.Errorf("unarmed run created %d push(es); no push without -p", len(pushes))
	}
}

// TestRun_PushArmedNoCommit_GateNoAbort_NoPush proves -p WITHOUT a successful commit
// performs no push: an `n` gate abort produces no commit, so the push gated on
// commit success never runs.
func TestRun_PushArmedNoCommit_GateNoAbort_NoPush(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceNo}}
	r := runner.NewFakeRunner()
	// Only the staged-diff reads are scripted; neither the commit nor the push is reached.
	r.Seed("git", runner.Result{Stdout: "diff --git a/x b/x\n+work"}, nil)
	deps := newCommitDeps(rec, r, scriptedTransport("feat: declined"), t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil on gate-no; want a non-zero abort")
	}

	if commits := commitInvocations(r); len(commits) != 0 {
		t.Errorf("gate-no created %d commit(s); abort must mutate nothing", len(commits))
	}
	if pushes := pushInvocations(r); len(pushes) != 0 {
		t.Errorf("gate-no with -p created %d push(es); no commit means no push", len(pushes))
	}
}

// TestRun_PushArmedNoCommit_GenerateFailure_NoPush proves -p WITHOUT a successful
// commit performs no push on the failure side too: a generate failure aborts before
// the commit, so no commit means no push.
func TestRun_PushArmedNoCommit_GenerateFailure_NoPush(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	// Only the staged-diff read is scripted; the commit and push must never be reached.
	r.Seed("git", runner.Result{Stdout: "diff --git a/x b/x\n+work"}, nil)
	deps := commit.Deps{
		Presenter: rec,
		Runner:    r,
		Committer: git.NewMutator(r),
		Transport: &recordingTransport{err: errExitOne},
		Root:      t.TempDir(),
		Push:      true,
	}

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil error, want a generate-failure abort")
	}

	if commits := commitInvocations(r); len(commits) != 0 {
		t.Errorf("generate failure created %d commit(s); it must not commit", len(commits))
	}
	if pushes := pushInvocations(r); len(pushes) != 0 {
		t.Errorf("generate failure with -p created %d push(es); no commit means no push", len(pushes))
	}
}

// TestRun_PushArmed_PushesAfterDashYAutoAcceptCommit proves the push fires after a
// -y auto-accept commit exactly as after an interactive accept: an unscripted Prompt
// returns the gate Default (ChoiceYes), modelling the -y skip, and the armed push
// runs after that commit.
func TestRun_PushArmed_PushesAfterDashYAutoAcceptCommit(t *testing.T) {
	t.Parallel()

	const message = "feat: push after -y accept"
	rec := &presentertest.RecordingPresenter{} // unscripted → Default (ChoiceYes), modelling the -y skip
	r := seedDiffThenCommitThenPush("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())
	deps.Yes = true
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// The auto-accepted gate committed, and the armed push followed it.
	findCommitInvocation(t, r)
	pushes := pushInvocations(r)
	if len(pushes) != 1 {
		t.Fatalf("git push invocations = %d (%v), want exactly 1 after the -y auto-accept commit", len(pushes), pushes)
	}
	commitIdx := indexOfGitArgs(r, "commit")
	pushIdx := indexOfGitArgs(r, "push")
	if commitIdx >= pushIdx {
		t.Errorf("commit at %d, push at %d; the push must run strictly AFTER the -y commit", commitIdx, pushIdx)
	}
}

// TestRun_PushViaGitSafe_NotRawRunner proves the auto-push flows through the
// lock-resilient git_safe Committer, not the raw runner: a stale-lock contention on
// the first push attempt is RETRIED (the raw runner would surface the first failure).
// Seeding a lock contention on the first push and a success on the second shows the
// retry — proof the push goes through the same git_safe wrapper as the commit.
func TestRun_PushViaGitSafe_NotRawRunner(t *testing.T) {
	t.Parallel()

	const message = "chore: push via git_safe"
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},                       // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work"}}, // git diff --cached
		runner.ScriptedCall{}, // git commit -F - (succeeds)
		runner.ScriptedCall{ // git push attempt 1: lock contention
			Result: runner.Result{Stderr: "fatal: Unable to create '/nope/.git/index.lock': File exists\nAnother git process seems to be running"},
			Err:    errExitOne,
		},
		runner.ScriptedCall{}, // git push attempt 2: succeeds after the wrapper retries
	)

	deps := commit.Deps{
		Presenter: rec,
		Runner:    r,
		// A no-op backoff keeps the retry deterministic and never sleeps.
		Committer: git.NewMutator(r, git.WithBackoff(func(int) {})),
		Transport: scriptedTransport(message),
		Root:      t.TempDir(),
		Push:      true,
	}

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// Two push attempts prove the lock-resilient retry ran — a raw runner would have
	// surfaced the first lock failure and never retried.
	pushes := pushInvocations(r)
	if len(pushes) != 2 {
		t.Fatalf("git push invocations = %d, want 2 (the lock retry proves git_safe)", len(pushes))
	}
}
