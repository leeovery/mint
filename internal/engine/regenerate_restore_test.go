package engine_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// putBack is the recovery's return of CHANGELOG.md to HEAD in the repo at dir, and
// nothing else.
func putBack(dir string) string {
	return "git -C " + dir + " restore --source=HEAD --staged --worktree --pathspec-from-file=- --pathspec-file-nul" + piped("CHANGELOG.md")
}

// regenRestoreCase is a regenerate whose changelog rewrite no commit holds: CHANGELOG.md
// there before it or created by it, its stage or its commit failing.
type regenRestoreCase struct {
	name string
	// existed reports that CHANGELOG.md was there before the rewrite.
	existed bool
	// commitFails is the commit failing, the rewrite staged; else the stage fails.
	commitFails bool
	// restores reports that git is asked to put the file back; else it is taken out of
	// the index, which a stage that failed part-way may have put it in, and removed from
	// the disk.
	restores bool
	undone   string
}

var regenRestoreCases = []regenRestoreCase{
	{name: "a stage that fails, the file there before", existed: true, restores: true, undone: "restored CHANGELOG.md"},
	{name: "a commit that fails, the file there before", existed: true, commitFails: true, restores: true, undone: "restored CHANGELOG.md"},
	{name: "a stage that fails, the file created", undone: "removed CHANGELOG.md"},
	{name: "a commit that fails, the file created", commitFails: true, restores: true, undone: "removed CHANGELOG.md"},
}

// failingTail scripts the read of whether the user changed CHANGELOG.md, the stage and
// commit, the one failing, then, where the file was never staged, git naming it among
// those it holds, and the put-back where git is asked for it, else the file taken out of
// the index.
func (tc regenRestoreCase) failingTail() []runner.ScriptedCall {
	tail := []runner.ScriptedCall{unchangedFiles, gitFailing(stageStderr)}
	if tc.commitFails {
		tail = []runner.ScriptedCall{unchangedFiles, ScriptedOut(""), gitFailing(commitStderr)}
	}
	if tc.restores && !tc.commitFails {
		tail = append(tail, ScriptedOut("CHANGELOG.md\x00"))
	}
	return append(tail, ScriptedOut(""))
}

// wantTail is the git steps from the read of whether the user changed CHANGELOG.md on:
// that read, the stage, the commit where the stage went through, else the read of
// whether git holds the file it wrote over, then the put-back where git is asked for
// it, else the file taken out of the index, and no reset.
func (tc regenRestoreCase) wantTail(dir string) []string {
	want := []string{userChangesRead(dir, "CHANGELOG.md"), "git -C " + dir + " add CHANGELOG.md"}
	if tc.commitFails {
		want = append(want, "git -C "+dir+" commit -m docs(changelog): regenerate notes for "+regenWriteTag+" --pathspec-from-file=- --pathspec-file-nul"+piped("CHANGELOG.md"))
	}
	if tc.restores && !tc.commitFails {
		want = append(want, knownChangelog(dir))
	}
	if tc.restores {
		return append(want, putBack(dir))
	}
	return append(want, "git -C "+dir+" rm --cached -q --ignore-unmatch --pathspec-from-file=- --pathspec-file-nul"+piped("CHANGELOG.md"))
}

// knownChangelog is the read of whether git holds CHANGELOG.md, in the repo at dir,
// in HEAD or the index.
func knownChangelog(dir string) string {
	return "git -C " + dir + " ls-files -z --cached --with-tree=HEAD -- :(top,literal)CHANGELOG.md"
}

// assertGitTailFrom fails unless the git steps from the first that is want's first are
// exactly want.
func assertGitTailFrom(t *testing.T, f *runner.FakeRunner, want []string) {
	t.Helper()
	got := stepLines(f)
	at := slices.Index(got, want[0])
	if at == -1 || !slices.Equal(got[at:], want) {
		t.Errorf("git steps = %q, want them to end %q", got, want)
	}
}

func TestRegenerateWrite_AStageOrCommitThatFails_PutsBackExactlyTheRewrite(t *testing.T) {
	t.Parallel()

	for _, tc := range regenRestoreCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			if tc.existed {
				seedChangelog(t, dir, regenNotesStaleChangelog)
			}
			f := runner.NewFakeRunner()
			f.SeedSequence("git", ScriptedOut("startHEAD"), ScriptedOut(regenWriteHistorical)) // rev-parse HEAD, for-each-ref
			f.SeedSequence("git", tc.failingTail()...)
			rec := &presentertest.RecordingPresenter{}

			err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), nil, dir, freshWriteReq(engine.RegenerateTargetChangelog))

			assertAbortNonZero(t, err)
			assertGitTailFrom(t, f, tc.wantTail(dir))
			if got := unwoundText(t, rec); got != tc.undone {
				t.Errorf("Unwound = %q, want %q", got, tc.undone)
			}
			_, statErr := os.Stat(filepath.Join(dir, "CHANGELOG.md"))
			if gone := errors.Is(statErr, os.ErrNotExist); gone != (!tc.existed && !tc.restores) {
				t.Errorf("CHANGELOG.md gone = %v, want removed only where the rewrite created it and git never held it", gone)
			}
		})
	}
}

// A notes file in the repo is named by no step of the commit or the put-back: its
// edits and staged state are the user's.
func TestRegenerateWrite_ACommitThatFails_PutsBackTheRewriteLeavingTheNotesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, regenNotesStaleChangelog)
	path := writeNotesFile(t, dir, "NEXT_RELEASE.md", suppliedBody)
	f := runner.NewFakeRunner()
	f.SeedSequence("git", regenChangelogWrite()...)
	f.SeedSequence("git", gitFailing(commitStderr), ScriptedOut("")) // the commit, the put-back
	rec := &presentertest.RecordingPresenter{}

	err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), nil, dir, freshWriteReq(engine.RegenerateTargetChangelog))

	assertAbortNonZero(t, err)
	if got := stepLines(f); got[len(got)-1] != putBack(dir) {
		t.Errorf("git steps = %q, want them to end with the put-back %q", got, putBack(dir))
	}
	assertNoDiscard(t, f)
	assertNotesFileOnlyExcluded(t, f, "NEXT_RELEASE.md")
	assertFileUnchanged(t, path, suppliedBody)
}

func TestRegenerateWrite_APutBackThatFails_SaysSoWithTheManualStep(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, regenNotesStaleChangelog)
	f := runner.NewFakeRunner()
	f.SeedSequence("git", regenChangelogWrite()...)
	f.SeedSequence("git", gitFailing(commitStderr), gitFailing(stageStderr)) // commit, the put-back
	rec := &presentertest.RecordingPresenter{}

	err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), nil, dir, freshWriteReq(engine.RegenerateTargetChangelog))

	assertAbortNonZero(t, err)
	step := "git restore --source=HEAD --staged --worktree -- ':(top,literal)CHANGELOG.md'"
	onlyWarning(t, rec, unwindIncomplete("restore CHANGELOG.md to HEAD (`"+step+"`)", stageStderr))
	if got := unwoundText(t, rec); got != "" {
		t.Errorf("Unwound = %q, want nothing claimed undone", got)
	}
}

// A CHANGELOG.md git holds no copy of, one it ignores, is left as the rewrite left it,
// and said so, with no restore git would refuse.
func TestRegenerateWrite_AStageThatFails_AChangelogGitHoldsNoCopyOf_IsLeftAndSaidSo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, regenNotesStaleChangelog)
	f := runner.NewFakeRunner()
	f.SeedSequence("git", ScriptedOut("startHEAD"), ScriptedOut(regenWriteHistorical)) // rev-parse HEAD, for-each-ref
	f.SeedSequence("git", unchangedFiles, gitFailing(stageStderr), ScriptedOut(""))    // the user-change read, add, ls-files naming nothing
	rec := &presentertest.RecordingPresenter{}

	err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), nil, dir, freshWriteReq(engine.RegenerateTargetChangelog))

	assertAbortNonZero(t, err)
	assertGitTailFrom(t, f, []string{"git -C " + dir + " add CHANGELOG.md", knownChangelog(dir)})
	onlyWarning(t, rec, unwindIncomplete("put back CHANGELOG.md, which mint wrote over and git holds no copy of", ""))
	if got := unwoundText(t, rec); got != "" {
		t.Errorf("Unwound = %q, want nothing claimed undone", got)
	}
}

func TestRegenerateAllValidated_Changelog_ACommitThatFails_PutsBackTheRebuild(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## [1.0.0] - 2024-01-01\n\nstale v1\n")
	f := runner.NewFakeRunner()
	f.SeedSequence("git", append(preflightCommitPushBucket(),
		ScriptedOut("startHEAD"), // rev-parse HEAD (captured start)
		ScriptedOut(batchV1Date), // for-each-ref creatordate:short v1
		ScriptedOut(batchV2Date), // for-each-ref creatordate:short v2
		ScriptedOut(batchV3Date), // for-each-ref creatordate:short v3
		unchangedFiles,           // the read of whether the user changed the files mint writes
		ScriptedOut(""),          // add CHANGELOG.md
		gitFailing(commitStderr), // commit
		ScriptedOut(""),          // the put-back
	)...)
	rec := &presentertest.RecordingPresenter{}

	err := engine.RegenerateAllValidated(t.Context(), batchDeps(rec, f), newFakePublisher(), dir,
		freshChangelogBatchReq(threeVersions(), engine.RegenerateTargetChangelog), true)

	assertAbortNonZero(t, err)
	got := stepLines(f)
	if got[len(got)-1] != putBack(dir) {
		t.Errorf("git steps = %q, want them to end with the put-back %q", got, putBack(dir))
	}
	assertNoReset(t, f)
	if got := unwoundText(t, rec); got != "restored CHANGELOG.md" {
		t.Errorf("Unwound = %q, want %q", got, "restored CHANGELOG.md")
	}
}
