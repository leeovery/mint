package record_test

import (
	"errors"
	"testing"

	"mint/internal/record"
	"mint/internal/runner"
)

// committedPathsArgs is the read of the paths HEAD's commit changed, in dir.
func committedPathsArgs(dir string) []string {
	return []string{"-C", dir, "diff-tree", "--no-commit-id", "--name-only", "-r", "-z", "HEAD"}
}

// indexToCommitArgs is the restore of the index entries named on stdin to HEAD, in dir.
func indexToCommitArgs(dir string) []string {
	return []string{"-C", dir, "restore", "--source=HEAD", "--staged", "--pathspec-from-file=-", "--pathspec-file-nul"}
}

// refreshArgs is the refresh of the index's stat data in dir.
func refreshArgs(dir string) []string {
	return []string{"-C", dir, "update-index", "-q", "--refresh"}
}

// The commit hooks run against a temporary index, so whatever they staged rides in the
// commit while the real index keeps it as it was: after the commit, every path the
// commit changed, a file a hook added among them, has its index entry matched to HEAD,
// named on stdin, and no work tree is touched; the index's stat data is refreshed after,
// as the restored entries carry none, which reset --keep would refuse over.
func TestCommitPaths_MatchesTheIndexToEveryPathTheCommitChanged(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{}, // commit
		runner.ScriptedCall{Result: runner.Result{Stdout: "CHANGELOG.md\x00VERSION\x00dist/notes.json\x00"}}, // diff-tree
		runner.ScriptedCall{}, // restore --staged
		runner.ScriptedCall{}, // update-index --refresh
	)

	const dir = "/repo/root"
	if err := record.CommitPaths(t.Context(), mutator(r), dir, "subject", []string{"CHANGELOG.md", "VERSION"}); err != nil {
		t.Fatalf("CommitPaths returned unexpected error: %v", err)
	}

	assertCalls(t, r, []wantCall{
		{args: pathspecCommitArgs(dir, "subject"), stdin: ":(top,literal)CHANGELOG.md\x00:(top,literal)VERSION\x00"},
		{args: committedPathsArgs(dir)},
		{args: indexToCommitArgs(dir), stdin: ":(top,literal)CHANGELOG.md\x00:(top,literal)VERSION\x00:(top,literal)dist/notes.json\x00"},
		{args: refreshArgs(dir)},
	})
}

// A commit that changed no path leaves nothing to match, so no restore runs, whose
// empty pathspec would name the whole index.
func TestCommitPaths_ACommitChangingNoPath_RunsNoRestore(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.SeedSequence("git", runner.ScriptedCall{}, runner.ScriptedCall{})

	const dir = "/repo/root"
	if err := record.CommitPaths(t.Context(), mutator(r), dir, "subject", []string{"VERSION"}); err != nil {
		t.Fatalf("CommitPaths returned unexpected error: %v", err)
	}
	assertCalls(t, r, []wantCall{
		{args: pathspecCommitArgs(dir, "subject"), stdin: ":(top,literal)VERSION\x00"},
		{args: committedPathsArgs(dir)},
	})
}

// Once the commit landed, a failure to match the index is an *IndexBehindError carrying
// git's words, never a failed commit: every caller passes it on as such, so the run
// counts the commit.
func TestCommitPaths_AFailureToMatchTheIndex_IsAnIndexBehindErrorAndTheCommitStands(t *testing.T) {
	t.Parallel()

	const stderr = "fatal: Unable to create '/repo/root/.git/index.lock': File exists.\n"
	failed := runner.ScriptedCall{Result: runner.Result{ExitCode: 128, Stderr: stderr}, Err: errors.New("exit status 128")}
	changed := runner.ScriptedCall{Result: runner.Result{Stdout: "CHANGELOG.md\x00"}}
	commits := []struct {
		name   string
		commit func(r *runner.FakeRunner) error
		// calls are git's answers ahead of the commit.
		calls []runner.ScriptedCall
	}{
		{name: "CommitPaths", commit: func(r *runner.FakeRunner) error {
			return record.CommitPaths(t.Context(), mutator(r), "/repo/root", "subject", []string{"CHANGELOG.md"})
		}},
		{name: "CommitFile", commit: func(r *runner.FakeRunner) error {
			return record.CommitFile(t.Context(), mutator(r), "/repo/root", "CHANGELOG.md", "subject")
		}, calls: []runner.ScriptedCall{{}}},
		{name: "CommitBookkeeping", commit: func(r *runner.FakeRunner) error {
			return record.CommitBookkeeping(t.Context(), mutator(r), "/repo/root", "🌿", "v0.0.1", "", true, false)
		}, calls: []runner.ScriptedCall{{}}},
		{name: "CommitDirtyTree", commit: func(r *runner.FakeRunner) error {
			committed, err := record.CommitDirtyTree(t.Context(), mutator(r), "/repo/root", "subject", "NEXT_RELEASE.md")
			if !committed {
				t.Errorf("committed = false, want true: the commit landed")
			}
			return err
		}, calls: []runner.ScriptedCall{
			{Result: runner.Result{Stdout: " M CHANGELOG.md\n"}}, {}, notesStaged,
			{Result: runner.Result{Stdout: "CHANGELOG.md\x00"}},
		}},
	}
	steps := []struct {
		name  string
		after []runner.ScriptedCall
	}{
		{name: "the read of the paths it changed", after: []runner.ScriptedCall{failed}},
		{name: "the restore", after: []runner.ScriptedCall{changed, failed, failed, failed, failed, failed}},
		{name: "the refresh", after: []runner.ScriptedCall{changed, {}, failed, failed, failed, failed, failed}},
	}

	for _, c := range commits {
		for _, step := range steps {
			t.Run(c.name+" "+step.name, func(t *testing.T) {
				t.Parallel()

				r := runner.NewFakeRunner()
				r.SeedSequence("git", c.calls...)
				r.SeedSequence("git", runner.ScriptedCall{}) // the commit
				r.SeedSequence("git", step.after...)

				err := c.commit(r)
				var behind *record.IndexBehindError
				if !errors.As(err, &behind) {
					t.Fatalf("error = %v, want a *record.IndexBehindError", err)
				}
				if behind.Stderr != stderr {
					t.Errorf("Stderr = %q, want %q", behind.Stderr, stderr)
				}
				var uncommitted *record.UncommittedError
				if errors.As(err, &uncommitted) {
					t.Errorf("error = %v, want no *record.UncommittedError: the commit landed", err)
				}
			})
		}
	}
}
