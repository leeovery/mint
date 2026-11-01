package record_test

import (
	"errors"
	"testing"

	"mint/internal/record"
	"mint/internal/runner"
)

// The commits HEAD names either side of a commit of mint's, as the fake answers its
// reads of HEAD.
const (
	beforeSHA = "beforesha"
	afterSHA  = "aftersha"
)

var (
	headBefore = runner.ScriptedCall{Result: runner.Result{Stdout: beforeSHA + "\n"}}
	headAfter  = runner.ScriptedCall{Result: runner.Result{Stdout: afterSHA + "\n"}}
)

// headArgs is the read of the commit HEAD names in dir.
func headArgs(dir string) []string {
	return []string{"-C", dir, "rev-parse", "HEAD"}
}

// committedPathsArgs is the read of the paths changed from beforeSHA to afterSHA in
// dir, less deletions and keepOut's pathspec.
func committedPathsArgs(dir string, keepOut ...string) []string {
	return append([]string{"-C", dir, "diff", "--name-only", "-z", "--no-renames", "--diff-filter=d", beforeSHA, afterSHA}, keepOut...)
}

// indexToCommitArgs is the restore of the index entries named on stdin to afterSHA, in
// dir.
func indexToCommitArgs(dir string) []string {
	return []string{"-C", dir, "restore", "--source=" + afterSHA, "--staged", "--pathspec-from-file=-", "--pathspec-file-nul"}
}

// refreshArgs is the refresh of the index's stat data in dir.
func refreshArgs(dir string) []string {
	return []string{"-C", dir, "update-index", "-q", "--refresh"}
}

// The commit hooks run against a temporary index, so whatever they staged rides in the
// commit while the real index keeps it as it was: after the commit, every path changed
// between HEAD before it and HEAD after, a file a hook added among them, has its index
// entry matched to HEAD after, named on stdin, and no work tree is touched; the
// index's stat data is refreshed after, as the restored entries carry none, which
// reset --keep would refuse over. The read leaves out deletions (--diff-filter=d),
// which the commit already took out of the index and a restore from HEAD would fail
// on, and spans HEAD as it was before the commit, so a post-commit hook's own commit
// on top is matched too.
func TestCommitPaths_MatchesTheIndexToEveryPathChangedSinceHEADBeforeTheCommit(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		headBefore,
		runner.ScriptedCall{}, // commit
		headAfter,
		runner.ScriptedCall{Result: runner.Result{Stdout: "CHANGELOG.md\x00VERSION\x00dist/notes.json\x00"}}, // diff
		runner.ScriptedCall{}, // restore --staged
		runner.ScriptedCall{}, // update-index --refresh
	)

	const dir = "/repo/root"
	if err := record.CommitPaths(t.Context(), mutator(r), dir, "subject", []string{"CHANGELOG.md", "VERSION"}, ""); err != nil {
		t.Fatalf("CommitPaths returned unexpected error: %v", err)
	}

	assertCalls(t, r, []wantCall{
		{args: headArgs(dir)},
		{args: pathspecCommitArgs(dir, "subject"), stdin: ":(top,literal)CHANGELOG.md\x00:(top,literal)VERSION\x00"},
		{args: headArgs(dir)},
		{args: committedPathsArgs(dir)},
		{args: indexToCommitArgs(dir), stdin: ":(top,literal)CHANGELOG.md\x00:(top,literal)VERSION\x00:(top,literal)dist/notes.json\x00"},
		{args: refreshArgs(dir)},
	})
}

// The notes file is left out of the match: a hook that stages every tracked change
// commits it, and a restore would put what the hook committed over its staged-only
// state, which is the user's.
func TestCommitPaths_LeavesTheNotesFileOutOfTheIndexMatch(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.SeedSequence("git", headBefore, runner.ScriptedCall{}, headAfter, runner.ScriptedCall{})

	const dir = "/repo/root"
	if err := record.CommitPaths(t.Context(), mutator(r), dir, "subject", []string{"CHANGELOG.md"}, "NEXT_RELEASE.md"); err != nil {
		t.Fatalf("CommitPaths returned unexpected error: %v", err)
	}
	assertCalls(t, r, []wantCall{
		{args: headArgs(dir)},
		{args: pathspecCommitArgs(dir, "subject"), stdin: ":(top,literal)CHANGELOG.md\x00"},
		{args: headArgs(dir)},
		{args: committedPathsArgs(dir, "--", ":(top,literal,exclude)NEXT_RELEASE.md")},
	})
}

// A commit that changed no path leaves nothing to match, so no restore runs, whose
// empty pathspec would name the whole index.
func TestCommitPaths_ACommitChangingNoPath_RunsNoRestore(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.SeedSequence("git", headBefore, runner.ScriptedCall{}, headAfter, runner.ScriptedCall{})

	const dir = "/repo/root"
	if err := record.CommitPaths(t.Context(), mutator(r), dir, "subject", []string{"VERSION"}, ""); err != nil {
		t.Fatalf("CommitPaths returned unexpected error: %v", err)
	}
	assertCalls(t, r, []wantCall{
		{args: headArgs(dir)},
		{args: pathspecCommitArgs(dir, "subject"), stdin: ":(top,literal)VERSION\x00"},
		{args: headArgs(dir)},
		{args: committedPathsArgs(dir)},
	})
}

// A failure to read HEAD before the commit commits nothing: it is a *GitError carrying
// git's words, never an *IndexBehindError.
func TestCommitPaths_AFailedReadOfHEADBeforeTheCommit_CommitsNothing(t *testing.T) {
	t.Parallel()

	const stderr = "fatal: ambiguous argument 'HEAD'\n"
	r := runner.NewFakeRunner()
	r.SeedSequence("git", runner.ScriptedCall{Result: runner.Result{ExitCode: 128, Stderr: stderr}, Err: errors.New("exit status 128")})

	err := record.CommitPaths(t.Context(), mutator(r), "/repo/root", "subject", []string{"VERSION"}, "")

	var gitErr *record.GitError
	if !errors.As(err, &gitErr) || gitErr.Stderr != stderr {
		t.Fatalf("error = %v, want a *record.GitError carrying %q", err, stderr)
	}
	var behind *record.IndexBehindError
	if errors.As(err, &behind) {
		t.Errorf("error = %v, want no *record.IndexBehindError: nothing was committed", err)
	}
	assertCalls(t, r, []wantCall{{args: headArgs("/repo/root")}})
}

// Once the commit landed, a failure to match the index is an *IndexBehindError carrying
// git's words, the commits either side of it and the path left out, never a failed
// commit: every caller passes it on as such, so the run counts the commit.
func TestCommitPaths_AFailureToMatchTheIndex_IsAnIndexBehindErrorAndTheCommitStands(t *testing.T) {
	t.Parallel()

	const stderr = "fatal: Unable to create '/repo/root/.git/index.lock': File exists.\n"
	failed := runner.ScriptedCall{Result: runner.Result{ExitCode: 128, Stderr: stderr}, Err: errors.New("exit status 128")}
	changed := runner.ScriptedCall{Result: runner.Result{Stdout: "CHANGELOG.md\x00"}}
	commits := []struct {
		name   string
		commit func(r *runner.FakeRunner) error
		// calls are git's answers ahead of the read of HEAD before the commit.
		calls   []runner.ScriptedCall
		keepOut string
	}{
		{name: "CommitPaths", commit: func(r *runner.FakeRunner) error {
			return record.CommitPaths(t.Context(), mutator(r), "/repo/root", "subject", []string{"CHANGELOG.md"}, "")
		}},
		{name: "CommitFile", commit: func(r *runner.FakeRunner) error {
			return record.CommitFile(t.Context(), mutator(r), "/repo/root", "CHANGELOG.md", "subject", "")
		}, calls: []runner.ScriptedCall{{}}},
		{name: "CommitBookkeeping", commit: func(r *runner.FakeRunner) error {
			return record.CommitBookkeeping(t.Context(), mutator(r), "/repo/root", "🌿", "v0.0.1", "", true, false, "")
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
		}, keepOut: "NEXT_RELEASE.md"},
	}
	steps := []struct {
		name  string
		after []runner.ScriptedCall
		// read is whether the read of HEAD after the commit went through, naming it.
		read bool
	}{
		{name: "the read of HEAD after it", after: []runner.ScriptedCall{failed}},
		{name: "the read of the paths it changed", after: []runner.ScriptedCall{headAfter, failed}, read: true},
		{name: "the restore", after: []runner.ScriptedCall{headAfter, changed, failed, failed, failed, failed, failed}, read: true},
		{name: "the refresh", after: []runner.ScriptedCall{headAfter, changed, {}, failed, failed, failed, failed, failed}, read: true},
	}

	for _, c := range commits {
		for _, step := range steps {
			t.Run(c.name+" "+step.name, func(t *testing.T) {
				t.Parallel()

				r := runner.NewFakeRunner()
				r.SeedSequence("git", c.calls...)
				r.SeedSequence("git", headBefore, runner.ScriptedCall{}) // HEAD before, the commit
				r.SeedSequence("git", step.after...)

				err := c.commit(r)
				var behind *record.IndexBehindError
				if !errors.As(err, &behind) {
					t.Fatalf("error = %v, want a *record.IndexBehindError", err)
				}
				after := "HEAD"
				if step.read {
					after = afterSHA
				}
				if behind.Stderr != stderr || behind.Before != beforeSHA || behind.After != after || string(behind.KeepOut) != c.keepOut {
					t.Errorf("IndexBehindError = %+v, want Stderr %q, Before %q, After %q, KeepOut %q", behind, stderr, beforeSHA, after, c.keepOut)
				}
				var uncommitted *record.UncommittedError
				if errors.As(err, &uncommitted) {
					t.Errorf("error = %v, want no *record.UncommittedError: the commit landed", err)
				}
			})
		}
	}
}
