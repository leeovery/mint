package commit_test

import (
	"context"
	"testing"

	"mint/internal/commit"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// The flag-aware empty-staging messages the preflight surfaces verbatim when the
// chosen staging mode would stage nothing but changes still exist (spec: Staging
// Model → Empty-staging handling). The em dash is U+2014, matching the spec.
const (
	noChangesStagedMessage  = "no changes staged — use -a/--all, -A/--add-all, or git add"
	noTrackedChangesMessage = "no tracked changes to stage — use -A/--add-all to include untracked files"
)

// TestRun_AddAllOnPristineTree_ReportsCleanTree proves `mint commit -A` on a pristine
// tree fails loud with git's clean-tree line — keyed on the ACTUAL tree state (clean),
// not the -A flag. The would-be-staged probe is empty (no tracked changes, no
// untracked), and `git status --porcelain` reports a clean tree, so the message is the
// clean-tree line. No AI, no add, no commit.
func TestRun_AddAllOnPristineTree_ReportsCleanTree(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	// -A would-be-staged probe: `git diff HEAD --name-only` (empty), then `git ls-files
	// --others --exclude-standard` (empty) → empty would-be-staged set. Then the tree
	// state probe `git status --porcelain` (empty) → genuinely clean tree.
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}}, // git diff HEAD --name-only (no tracked changes)
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}}, // git ls-files --others (no untracked)
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}}, // git status --porcelain (clean tree)
	)
	transport := scriptedTransport("must never be returned (clean tree)")
	deps := newCommitDeps(rec, r, transport, t.TempDir())
	deps.Staging = commit.AddAll

	err := commit.Run(context.Background(), deps)
	if err == nil {
		t.Fatal("Run returned nil for -A on a pristine tree; want a non-zero fail-loud abort")
	}
	if err.Error() != nothingToCommitMessage {
		t.Errorf("error = %q, want the clean-tree line %q (keyed on tree state, not the -A flag)", err.Error(), nothingToCommitMessage)
	}

	idx := indexOfKind(rec.Kinds(), presentertest.KindStageFailed)
	if idx < 0 {
		t.Fatalf("kinds = %v, want a StageFailed narrating the clean-tree abort", rec.Kinds())
	}
	ev, _ := rec.At(idx)
	if ev.StageFailed.Message != nothingToCommitMessage {
		t.Errorf("StageFailed.Message = %q, want the clean-tree line %q", ev.StageFailed.Message, nothingToCommitMessage)
	}
}

// TestRun_AllOnPristineTree_ReportsCleanTree proves `mint commit -a` on a pristine tree
// fails loud with git's clean-tree line: the -a would-be-staged probe (`git diff HEAD
// --name-only`) is empty and `git status --porcelain` reports a clean tree.
func TestRun_AllOnPristineTree_ReportsCleanTree(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}}, // git diff HEAD --name-only (no tracked changes)
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}}, // git status --porcelain (clean tree)
	)
	deps := newCommitDeps(rec, r, scriptedTransport("must never be returned (clean tree)"), t.TempDir())
	deps.Staging = commit.All

	err := commit.Run(context.Background(), deps)
	if err == nil {
		t.Fatal("Run returned nil for -a on a pristine tree; want a non-zero fail-loud abort")
	}
	if err.Error() != nothingToCommitMessage {
		t.Errorf("error = %q, want the clean-tree line %q", err.Error(), nothingToCommitMessage)
	}
}

// TestRun_BareCommitUnstagedChangesNothingStaged_PointsAtStagingFlags proves a bare
// `mint commit` with unstaged changes but nothing staged fails loud with mint's flavour
// of git's "no changes added to commit" — naming the staging modes that would help. The
// staged probe (`git diff --cached --name-only`) is empty, and `git status --porcelain`
// reports changes exist, so the message is the no-changes-staged guidance.
func TestRun_BareCommitUnstagedChangesNothingStaged_PointsAtStagingFlags(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},          // git diff --cached --name-only (nothing staged)
		runner.ScriptedCall{Result: runner.Result{Stdout: " M file\n"}}, // git status --porcelain (unstaged changes exist)
	)
	deps := newCommitDeps(rec, r, scriptedTransport("must never be returned (nothing staged)"), t.TempDir())
	deps.Staging = commit.StagedOnly

	err := commit.Run(context.Background(), deps)
	if err == nil {
		t.Fatal("Run returned nil for a bare commit with unstaged changes; want a non-zero fail-loud abort")
	}
	if err.Error() != noChangesStagedMessage {
		t.Errorf("error = %q, want the no-changes-staged guidance %q", err.Error(), noChangesStagedMessage)
	}

	idx := indexOfKind(rec.Kinds(), presentertest.KindStageFailed)
	if idx < 0 {
		t.Fatalf("kinds = %v, want a StageFailed narrating the no-changes-staged abort", rec.Kinds())
	}
	ev, _ := rec.At(idx)
	if ev.StageFailed.Message != noChangesStagedMessage {
		t.Errorf("StageFailed.Message = %q, want %q", ev.StageFailed.Message, noChangesStagedMessage)
	}
}

// TestRun_AllWithOnlyUntrackedChanges_PointsAtAddAll proves `mint commit -a` when the
// only changes are untracked fails loud pointing specifically at -A/--add-all (the mode
// that would include them). The -a tracked probe (`git diff HEAD --name-only`) is empty
// (untracked files are not tracked changes), and `git status --porcelain` reports the
// untracked changes exist, so the message points at -A.
func TestRun_AllWithOnlyUntrackedChanges_PointsAtAddAll(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},         // git diff HEAD --name-only (no tracked changes)
		runner.ScriptedCall{Result: runner.Result{Stdout: "?? new\n"}}, // git status --porcelain (untracked changes exist)
	)
	deps := newCommitDeps(rec, r, scriptedTransport("must never be returned (only untracked)"), t.TempDir())
	deps.Staging = commit.All

	err := commit.Run(context.Background(), deps)
	if err == nil {
		t.Fatal("Run returned nil for -a with only untracked changes; want a non-zero fail-loud abort")
	}
	if err.Error() != noTrackedChangesMessage {
		t.Errorf("error = %q, want the point-at-add-all guidance %q", err.Error(), noTrackedChangesMessage)
	}
}

// TestRun_EmptyStagingMessageKeyedOnTreeStateNotFlag proves the empty-staging message
// is selected by the ACTUAL post-mode tree state, not by the flag passed: -A on a clean
// tree yields the clean-tree line, NOT a "no changes" guidance — even though -A is the
// everything-including-untracked mode. The discriminator is `git status --porcelain`
// reporting a clean tree.
func TestRun_EmptyStagingMessageKeyedOnTreeStateNotFlag(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}}, // git diff HEAD --name-only (no tracked changes)
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}}, // git ls-files --others (no untracked)
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}}, // git status --porcelain (clean tree)
	)
	deps := newCommitDeps(rec, r, scriptedTransport("must never be returned"), t.TempDir())
	deps.Staging = commit.AddAll

	err := commit.Run(context.Background(), deps)
	if err == nil {
		t.Fatal("Run returned nil for -A on a clean tree; want a non-zero fail-loud abort")
	}
	// Keyed on the tree state: a clean tree gives the clean-tree line regardless of the
	// -A flag, and is distinct from the flag-driven "no changes" guidances.
	if err.Error() != nothingToCommitMessage {
		t.Errorf("error = %q, want the clean-tree line %q (keyed on tree state, not the flag)", err.Error(), nothingToCommitMessage)
	}
	if err.Error() == noChangesStagedMessage || err.Error() == noTrackedChangesMessage {
		t.Errorf("error = %q, must NOT be a flag-driven no-changes guidance for a clean tree", err.Error())
	}
}

// TestRun_EmptyStaging_NoAIInvoked proves the AI is NEVER invoked in an empty-staging
// case: the preflight short-circuits BEFORE generate for every empty mode. Driven via
// the bare (StagedOnly) empty case with changes present.
func TestRun_EmptyStaging_NoAIInvoked(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},          // git diff --cached --name-only (nothing staged)
		runner.ScriptedCall{Result: runner.Result{Stdout: " M file\n"}}, // git status --porcelain (changes exist)
	)
	transport := scriptedTransport("must never be returned (empty staging)")
	deps := newCommitDeps(rec, r, transport, t.TempDir())
	deps.Staging = commit.StagedOnly

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil for an empty-staging case; want a non-zero abort")
	}

	if transport.calls() != 0 {
		t.Errorf("transport called %d times; an empty-staging case must short-circuit before any AI call", transport.calls())
	}
}

// TestRun_EmptyStaging_NoGitAddNoCommit proves no `git add` and no `git commit` run in
// an empty-staging case: the preflight fails loud before any mutation. Driven via the -a
// only-untracked empty case.
func TestRun_EmptyStaging_NoGitAddNoCommit(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},         // git diff HEAD --name-only (no tracked changes)
		runner.ScriptedCall{Result: runner.Result{Stdout: "?? new\n"}}, // git status --porcelain (untracked changes exist)
	)
	deps := newCommitDeps(rec, r, scriptedTransport("must never be returned"), t.TempDir())
	deps.Staging = commit.All

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil for an empty-staging case; want a non-zero abort")
	}

	if adds := addInvocations(r); len(adds) != 0 {
		t.Errorf("empty-staging case ran `git add` %v; it must never stage", adds)
	}
	if commits := commitInvocations(r); len(commits) != 0 {
		t.Errorf("empty-staging case created %d commit(s); it must never commit", len(commits))
	}
}
