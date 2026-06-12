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

// addInvocations returns every recorded `git add` invocation, in order — the deferred
// staging step the accept path runs (under -a/-A) before the commit mutation.
func addInvocations(r *runner.FakeRunner) []runner.Invocation {
	var adds []runner.Invocation
	for _, inv := range r.Invocations() {
		if inv.Name == "git" && len(inv.Args) > 0 && inv.Args[0] == "add" {
			adds = append(adds, inv)
		}
	}
	return adds
}

// indexOfGitArgs returns the position (within the ordered `git` invocations) of the
// first call whose first arg equals verb, or -1 — used to assert the strict
// stage-then-commit ordering.
func indexOfGitArgs(r *runner.FakeRunner, verb string) int {
	for i, inv := range gitInvocations(r) {
		if len(inv.Args) > 0 && inv.Args[0] == verb {
			return i
		}
	}
	return -1
}

// seedAllModeThenStageAndCommit scripts the -a (All) thread's git invocations IN
// ORDER: the empty-index preflight read (non-empty), the L1 read-only `git diff HEAD`,
// then the deferred `git add -u` and the `git commit -F -` on accept.
func seedAllModeThenStageAndCommit(diff string) *runner.FakeRunner {
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}}, // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},  // git diff HEAD -- . (read-only -a source)
		runner.ScriptedCall{}, // git add -u (deferred staging on accept)
		runner.ScriptedCall{}, // git commit -F -
	)
	return r
}

// seedAddAllModeThenStageAndCommit scripts the -A (AddAll) thread's git invocations IN
// ORDER: the empty-index preflight read (non-empty), the L1 read-only `git diff HEAD`,
// the untracked enumeration (empty — no untracked files for these tests), then the
// deferred `git add -A` and the `git commit -F -` on accept.
func seedAddAllModeThenStageAndCommit(diff string) *runner.FakeRunner {
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}}, // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},  // git diff HEAD -- . (read-only tracked source)
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},    // git ls-files --others (no untracked)
		runner.ScriptedCall{}, // git add -A (deferred staging on accept)
		runner.ScriptedCall{}, // git commit -F -
	)
	return r
}

// TestRun_AcceptUnderAll_AddsTrackedThenCommits proves an accept under -a runs
// `git add -u` (tracked modifications + deletions, no untracked) and then the commit,
// in that order — the deferred staging the mutate-nothing-until-accept invariant
// requires.
func TestRun_AcceptUnderAll_AddsTrackedThenCommits(t *testing.T) {
	t.Parallel()

	const message = "feat: stage tracked on accept under -a"
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedAllModeThenStageAndCommit("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())
	deps.Staging = commit.All

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	adds := addInvocations(r)
	if len(adds) != 1 {
		t.Fatalf("git add invocations = %d (%v), want exactly 1 (`git add -u`)", len(adds), adds)
	}
	if got := adds[0].Args; len(got) != 2 || got[0] != "add" || got[1] != "-u" {
		t.Errorf("staging argv = %v, want `git add -u` (tracked mods + deletions, no untracked)", got)
	}

	addIdx := indexOfGitArgs(r, "add")
	commitIdx := indexOfGitArgs(r, "commit")
	if addIdx < 0 || commitIdx < 0 {
		t.Fatalf("git calls = %v, want both an add and a commit", gitInvocations(r))
	}
	if addIdx >= commitIdx {
		t.Errorf("git add at %d, commit at %d; staging must run BEFORE the commit", addIdx, commitIdx)
	}
}

// TestRun_AcceptUnderAddAll_AddsEverythingThenCommits proves an accept under -A runs
// `git add -A` (everything including untracked) and then the commit, in that order.
func TestRun_AcceptUnderAddAll_AddsEverythingThenCommits(t *testing.T) {
	t.Parallel()

	const message = "feat: stage everything on accept under -A"
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedAddAllModeThenStageAndCommit("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())
	deps.Staging = commit.AddAll

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	adds := addInvocations(r)
	if len(adds) != 1 {
		t.Fatalf("git add invocations = %d (%v), want exactly 1 (`git add -A`)", len(adds), adds)
	}
	if got := adds[0].Args; len(got) != 2 || got[0] != "add" || got[1] != "-A" {
		t.Errorf("staging argv = %v, want `git add -A` (everything incl. untracked)", got)
	}

	addIdx := indexOfGitArgs(r, "add")
	commitIdx := indexOfGitArgs(r, "commit")
	if addIdx < 0 || commitIdx < 0 {
		t.Fatalf("git calls = %v, want both an add and a commit", gitInvocations(r))
	}
	if addIdx >= commitIdx {
		t.Errorf("git add at %d, commit at %d; staging must run BEFORE the commit", addIdx, commitIdx)
	}
}

// TestRun_AcceptUnderDefault_NoGitAddCommitsIndex proves an accept under the default
// (StagedOnly) mode runs NO `git add` and commits the existing index — the Phase 1
// path, byte-identical.
func TestRun_AcceptUnderDefault_NoGitAddCommitsIndex(t *testing.T) {
	t.Parallel()

	const message = "feat: default mode commits the index"
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommit("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())
	deps.Staging = commit.StagedOnly

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if adds := addInvocations(r); len(adds) != 0 {
		t.Errorf("default mode ran `git add` %v; StagedOnly stages nothing", adds)
	}
	commitInv := findCommitInvocation(t, r)
	if commitInv.Stdin != message {
		t.Errorf("commit stdin = %q, want the generated body verbatim %q", commitInv.Stdin, message)
	}
}

// TestRun_AbortUnderAll_NoGitAddNoCommit proves an abort (n) under -a runs NO
// `git add` and NO commit — nothing is mutated, so the index is its exact pre-mint
// state.
func TestRun_AbortUnderAll_NoGitAddNoCommit(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceNo}}
	r := runner.NewFakeRunner()
	// Preflight read (non-empty) then the read-only -a L1 diff; staging/commit must
	// never be reached, so nothing past the read is scripted.
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work"}},
	)
	deps := newCommitDeps(rec, r, scriptedTransport("feat: declined under -a"), t.TempDir())
	deps.Staging = commit.All

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil on gate-no under -a; want a non-zero abort")
	}

	if adds := addInvocations(r); len(adds) != 0 {
		t.Errorf("abort under -a ran `git add` %v; abort must mutate nothing", adds)
	}
	if commits := commitInvocations(r); len(commits) != 0 {
		t.Errorf("abort under -a created %d commit(s); abort must mutate nothing", len(commits))
	}
}

// TestRun_AbortUnderAddAll_IndexUntouched proves an abort (n) under -A leaves the
// index exactly its pre-mint state: NO `git add` and NO commit mutation run, so the
// only git calls are the read-only reads (preflight + L1 source), never a mutation
// through the git_safe sink.
func TestRun_AbortUnderAddAll_IndexUntouched(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceNo}}
	r := runner.NewFakeRunner()
	// Preflight read, the read-only -A tracked L1 diff, then the untracked enumeration —
	// all read-only. No staging/commit is scripted: it must never be reached.
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},
	)
	deps := newCommitDeps(rec, r, scriptedTransport("feat: declined under -A"), t.TempDir())
	deps.Staging = commit.AddAll

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil on gate-no under -A; want a non-zero abort")
	}

	if adds := addInvocations(r); len(adds) != 0 {
		t.Errorf("abort under -A ran `git add` %v; abort leaves the index untouched", adds)
	}
	if commits := commitInvocations(r); len(commits) != 0 {
		t.Errorf("abort under -A created %d commit(s); abort leaves the index untouched", len(commits))
	}
	// No mutation flowed through the git_safe sink at all on abort: every git call is a
	// read-only diff/ls-files, none an `add` or `commit`.
	for _, inv := range gitInvocations(r) {
		if len(inv.Args) > 0 && (inv.Args[0] == "add" || inv.Args[0] == "commit") {
			t.Errorf("abort under -A ran a mutation `git %v`; abort mutates nothing", inv.Args)
		}
	}
}

// TestRun_AbortLeavesPreExistingStagingUntouched proves an abort never touches the
// user's pre-existing staging: under -A, an abort runs NO `git add` (the only thing
// that would alter the index), so whatever the user had staged before `mint` ran is
// exactly as they left it. The proof is the absence of ANY index-altering git call on
// the abort path.
func TestRun_AbortLeavesPreExistingStagingUntouched(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceNo}}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},
	)
	deps := newCommitDeps(rec, r, scriptedTransport("feat: declined with prior staging"), t.TempDir())
	deps.Staging = commit.AddAll

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil on gate-no; want a non-zero abort")
	}

	// The user's prior staging is altered ONLY by a `git add`/`git commit`; an abort
	// runs neither, so the pre-existing index is untouched.
	for _, inv := range gitInvocations(r) {
		if len(inv.Args) > 0 && (inv.Args[0] == "add" || inv.Args[0] == "commit") {
			t.Errorf("abort ran an index-altering `git %v`; pre-existing user staging must be left untouched", inv.Args)
		}
	}
}

// TestRun_DashYAutoAcceptUnderAddAll_StagesThenCommits proves -y auto-accept follows
// the accept path under -A: it stages (`git add -A`) then commits, in that order,
// without the gate ever being declined (the -y skip is presenter-internal — the
// recorder returns the gate Default for an unscripted Prompt, modelling the -y echo).
func TestRun_DashYAutoAcceptUnderAddAll_StagesThenCommits(t *testing.T) {
	t.Parallel()

	const message = "feat: unattended add-all"
	rec := &presentertest.RecordingPresenter{} // unscripted → Default (ChoiceYes), modelling the -y skip
	r := seedAddAllModeThenStageAndCommit("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())
	deps.Staging = commit.AddAll

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	adds := addInvocations(r)
	if len(adds) != 1 {
		t.Fatalf("git add invocations = %d (%v), want exactly 1 (`git add -A`)", len(adds), adds)
	}
	if got := adds[0].Args; len(got) != 2 || got[0] != "add" || got[1] != "-A" {
		t.Errorf("staging argv = %v, want `git add -A`", got)
	}

	addIdx := indexOfGitArgs(r, "add")
	commitIdx := indexOfGitArgs(r, "commit")
	if addIdx < 0 || commitIdx < 0 {
		t.Fatalf("git calls = %v, want both an add and a commit under -y", gitInvocations(r))
	}
	if addIdx >= commitIdx {
		t.Errorf("git add at %d, commit at %d; -y auto-accept must stage BEFORE the commit", addIdx, commitIdx)
	}
}

// TestRun_StagingAddRunsViaGitSafe proves the deferred staging `git add` flows through
// the lock-resilient git_safe wrapper, NOT the raw runner: a stale-lock contention on
// the first `git add -u` attempt is RETRIED (a raw runner would surface the first lock
// failure and never stage). Seeding the contention then a success on the staging step
// shows the retry — proof the staging add is a git_safe mutation.
func TestRun_StagingAddRunsViaGitSafe(t *testing.T) {
	t.Parallel()

	const message = "chore: stage via git_safe"
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},                       // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work"}}, // git diff HEAD -- . (read-only -a source)
		runner.ScriptedCall{ // git add -u attempt 1: lock contention
			Result: runner.Result{Stderr: "fatal: Unable to create '/nope/.git/index.lock': File exists\nAnother git process seems to be running"},
			Err:    errExitOne,
		},
		runner.ScriptedCall{}, // git add -u attempt 2: succeeds after the wrapper retries
		runner.ScriptedCall{}, // git commit -F -
	)

	deps := commit.Deps{
		Presenter: rec,
		Runner:    r,
		// A no-op backoff keeps the retry deterministic and never sleeps.
		Mutator:   git.NewMutator(r, git.WithBackoff(func(int) {})),
		Transport: scriptedTransport(message),
		Root:      t.TempDir(),
		Staging:   commit.All,
	}

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// Two `git add` attempts prove the lock-resilient retry ran — a raw runner would
	// have surfaced the first lock failure and never retried.
	adds := addInvocations(r)
	if len(adds) != 2 {
		t.Fatalf("git add invocations = %d, want 2 (the lock retry proves the staging add is git_safe)", len(adds))
	}
	// The commit still runs after the retried staging.
	findCommitInvocation(t, r)
}
