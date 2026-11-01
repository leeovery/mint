package engine_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"mint/internal/cmdline"
	"mint/internal/engine"
	"mint/internal/git"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// notesResetCase is one notes file a reset must leave as it was. notesStaged is the
// staged-edit probe's output, as for keepOutCommit.
type notesResetCase struct {
	name        string
	file        string
	notesStaged string
}

var notesResetCases = []notesResetCase{
	{name: "an untracked notes.md", file: "notes.md"},
	{name: "a tracked NEXT_RELEASE.md with unstaged edits", file: "NEXT_RELEASE.md"},
	{name: "a tracked NEXT_RELEASE.md with staged edits", file: "NEXT_RELEASE.md", notesStaged: "NEXT_RELEASE.md\x00"},
}

// keepingReset scripts a successful reset that leaves the notes file as it was: the read
// of what is staged, answered with the notes file where it has edits staged, then the
// index written as a tree, the reset, and the file staged again.
func keepingReset(tt notesResetCase) []runner.ScriptedCall {
	if tt.notesStaged == "" {
		return []runner.ScriptedCall{nothingStaged, ScriptedOut("")}
	}
	return []runner.ScriptedCall{ScriptedOut(tt.notesStaged), ScriptedOut(stagedTree + "\n"), ScriptedOut(""), ScriptedOut("")}
}

// keepingResetLines are the git steps keepingReset answers, in the repo at root.
func keepingResetLines(tt notesResetCase, root, start string) []string {
	read := "git -C " + root + " diff --cached --name-only -z --no-renames"
	reset := "git reset --keep " + start
	if tt.notesStaged == "" {
		return []string{read, reset}
	}
	return []string{read, "git write-tree", reset, "git restore --source=" + stagedTree + " --staged -- :(top,literal)" + tt.file}
}

// assertResetKeptNotesFile fails unless the run's reset was keepingReset's: HEAD moved
// back to start by a reset that keeps the notes file's edits, then the file's staged
// edits staged again, and no reset that discards them.
func assertResetKeptNotesFile(t *testing.T, f *runner.FakeRunner, tt notesResetCase, root, start string) {
	t.Helper()
	want := keepingResetLines(tt, root, start)
	got := commandLines(f.Invocations())
	at := slices.Index(got, want[0])
	if at == -1 || len(got) < at+len(want) || !slices.Equal(got[at:at+len(want)], want) {
		t.Errorf("git steps = %q, want the reset %q", got, want)
	}
	assertNoDiscard(t, f)
}

func TestRelease_NotesFileInRepo_GateAbortAfterArtifactCommit_ResetLeavesTheNotesFile(t *testing.T) {
	t.Parallel()

	for _, tt := range notesResetCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")
			path := writeNotesFile(t, root, tt.file, suppliedBody)
			f := runner.NewFakeRunner()
			seedProvider(f)
			f.SeedSequence("git", firstReleaseGates(root, "")...)
			f.SeedSequence("git",
				ScriptedOut(" M bundle.js\n"), // -C root status --porcelain (post-hook: dirty)
				ScriptedOut(""),               // -C root add -A
			)
			f.SeedSequence("git", keepOutCommit(tt.notesStaged, stagedArtifacts)...)
			f.SeedSequence("git", keepingReset(tt)...)
			f.Seed("sh", runner.Result{}, nil)
			rec := &presentertest.RecordingPresenter{
				NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo},
			}

			err := engine.Release(t.Context(), newDeps(rec, f), inRepoNotesFileOptions(root, tt.file))
			assertAbortNonZero(t, err)

			assertResetKeptNotesFile(t, f, tt, root, startingSHA)
			if got, want := unwoundText(t, rec), "reset 1 commit; repo clean"; got != want {
				t.Errorf("Unwound.Text() = %q, want %q", got, want)
			}
			assertNoMutation(t, f)
			assertFileUnchanged(t, path, suppliedBody)
		})
	}
}

// seedThroughBookkeepingCommit scripts a first release with an in-repo notes file up
// to and including its bookkeeping commit, with stash as for firstReleaseGates.
func seedThroughBookkeepingCommit(f *runner.FakeRunner, root, notesStaged string, stash ...runner.ScriptedCall) {
	seedProvider(f)
	f.SeedSequence("git", firstReleaseGates(root, "", stash...)...)
	f.SeedSequence("git", ScriptedOut("")) // -C root add CHANGELOG.md
	f.SeedSequence("git", keepOutCommit(notesStaged, stagedChangelog)...)
	f.Seed("gh", runner.Result{}, nil)
}

func TestRelease_NotesFileInRepo_PushRejected_UnwindLeavesTheNotesFile(t *testing.T) {
	t.Parallel()

	for _, tt := range notesResetCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			path := writeNotesFile(t, root, tt.file, suppliedBody)
			f := runner.NewFakeRunner()
			seedThroughBookkeepingCommit(f, root, tt.notesStaged)
			f.SeedSequence("git",
				ScriptedOut(""), // tag -a v0.0.1 -F -
				pushRejected(),  // push --atomic origin HEAD v0.0.1
				ScriptedOut(""), // tag -d v0.0.1
			)
			f.SeedSequence("git", keepingReset(tt)...)
			rec := &presentertest.RecordingPresenter{}

			err := engine.Release(t.Context(), newDeps(rec, f), inRepoNotesFileOptions(root, tt.file))
			assertAbortNonZero(t, err)

			assertGitArgs(t, f, 2, []string{"tag", "-d", "v0.0.1"})
			assertResetKeptNotesFile(t, f, tt, root, startingSHA)
			if got, want := unwoundText(t, rec), "reset 1 commit and deleted tag v0.0.1; repo clean"; got != want {
				t.Errorf("Unwound.Text() = %q, want %q", got, want)
			}
			assertFileUnchanged(t, path, suppliedBody)
		})
	}
}

func TestRelease_NotesFileInRepo_CancelledAfterACommit_UnwindLeavesTheNotesFile(t *testing.T) {
	t.Parallel()

	for _, tt := range notesResetCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			path := writeNotesFile(t, root, tt.file, suppliedBody)
			f := runner.NewFakeRunner()
			seedThroughBookkeepingCommit(f, root, tt.notesStaged)
			f.SeedSequence("git", keepingReset(tt)...)
			rec := &presentertest.RecordingPresenter{}

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			err := engine.Release(ctx, newDeps(rec, f), inRepoNotesFileOptions(root, tt.file))
			assertAbortNonZero(t, err)

			if got := argsOf(f, "push"); len(got) != 0 {
				t.Errorf("a cancelled run pushed: %q", got)
			}
			assertResetKeptNotesFile(t, f, tt, root, startingSHA)
			if got, want := unwoundText(t, rec), "reset 1 commit; repo clean"; got != want {
				t.Errorf("Unwound.Text() = %q, want %q", got, want)
			}
			assertFileUnchanged(t, path, suppliedBody)
		})
	}
}

func TestRelease_NotesFileInRepo_Autostash_PushRejected_UnwindsKeepingTheNotesFileThenPops(t *testing.T) {
	t.Parallel()

	for _, tt := range notesResetCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			path := writeNotesFile(t, root, tt.file, suppliedBody)
			f := runner.NewFakeRunner()
			seedThroughBookkeepingCommit(f, root, tt.notesStaged, stashSaved())
			f.SeedSequence("git",
				ScriptedOut(""), // tag -a v0.0.1 -F -
				pushRejected(),  // push --atomic origin HEAD v0.0.1
				ScriptedOut(""), // tag -d v0.0.1
			)
			f.SeedSequence("git", keepingReset(tt)...)
			f.SeedSequence("git", stashPopped())
			rec := &presentertest.RecordingPresenter{}

			opts := inRepoNotesFileOptions(root, tt.file)
			opts.AutoStash = true
			err := engine.Release(t.Context(), newDeps(rec, f), opts)
			assertAbortNonZero(t, err)

			assertGitArgs(t, f, 2, []string{"stash", "push", "--include-untracked", "--", excludePathspec(tt.file)})
			assertResetKeptNotesFile(t, f, tt, root, startingSHA)
			reset := keepingResetLines(tt, root, startingSHA)
			got := commandLines(f.Invocations())
			if last := slices.Index(got, reset[len(reset)-1]); last == -1 || slices.Index(got, "git stash pop") != last+1 {
				t.Errorf("git steps = %q, want the other changes popped once the reset's %q is done", got, reset)
			}
			assertFileUnchanged(t, path, suppliedBody)
		})
	}
}

func TestRegenerateRun_NotesFileInRepo_PushFails_ResetLeavesTheNotesFile(t *testing.T) {
	t.Parallel()

	for _, tt := range notesResetCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			seedChangelog(t, root, regenNotesStaleChangelog)
			path := writeNotesFile(t, root, tt.file, suppliedBody)
			f := runner.NewFakeRunner()
			f.SeedSequence("git", ScriptedOut("")) // ls-files --ignored (not ignored)
			f.SeedSequence("git", regenChangelogGates("")...)
			f.SeedSequence("git", regenChangelogWrite()...)
			f.SeedSequence("git", keepOutCommit(tt.notesStaged, stagedChangelog)...)
			f.SeedSequence("git", runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("remote rejected")}) // push origin HEAD
			f.SeedSequence("git", keepingReset(tt)...)
			rec := &presentertest.RecordingPresenter{}

			err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), nil, root, notesFileRunReq(t, path, engine.RegenerateTargetChangelog))
			assertAbortNonZero(t, err)

			assertResetKeptNotesFile(t, f, tt, root, "startHEAD")
			if got := argsOf(f, "tag"); len(got) != 0 {
				t.Errorf("regenerate's reset touched a tag: %q", got)
			}
			assertFileUnchanged(t, path, suppliedBody)
		})
	}
}

// The reset keeps every change mint did not make: an unstaged edit of the user's, here
// to any path, is named by no step, and the paths the index holds staged, the notes
// file's among them, are staged again from the tree the index was written as.
func TestUnwind_Reset_KeepsEveryOtherChangeAndStagesAgainWhatWasStaged(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(""), // tag -d v0.0.1
		ScriptedOut("NEXT_RELEASE.md\x00src/app.go\x00"), // diff --cached --name-only
		ScriptedOut(stagedTree+"\n"),                     // write-tree
		ScriptedOut(""),                                  // reset --keep
		ScriptedOut(""),                                  // restore --staged
	)
	rec := &presentertest.RecordingPresenter{}

	start := startState(startingSHA, "v0.0.1")
	start.KeepOut = git.Exclusion("NEXT_RELEASE.md")
	err := engine.Unwind(t.Context(), newDeps(rec, f), start, engine.MadeState{Commits: 2, TagCreated: true}, errUnwindReason)

	assertAbortNonZero(t, err)
	want := []string{
		"git tag -d v0.0.1",
		"git diff --cached --name-only -z --no-renames",
		"git write-tree",
		"git reset --keep " + startingSHA,
		"git restore --source=" + stagedTree + " --staged -- :(top,literal)NEXT_RELEASE.md :(top,literal)src/app.go",
	}
	if got := commandLines(f.Invocations()); !slices.Equal(got, want) {
		t.Errorf("git steps = %q, want %q", got, want)
	}
	if got, want := unwoundText(t, rec), "reset 2 commits and deleted tag v0.0.1; repo clean"; got != want {
		t.Errorf("Unwound.Text() = %q, want %q", got, want)
	}
}

func TestUnwind_KeepOut_NoCommits_IssuesNoReset(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	rec := &presentertest.RecordingPresenter{}

	start := startState(startingSHA, "v0.0.1")
	start.KeepOut = git.Exclusion("NEXT_RELEASE.md")
	err := engine.Unwind(t.Context(), newDeps(rec, f), start, engine.MadeState{}, errUnwindReason)

	assertAbortNonZero(t, err)
	if got := f.Invocations(); len(got) != 0 {
		t.Errorf("an unwind with nothing made ran %v", commandLines(got))
	}
}

// Where the user changed a path mint's commits changed, git refuses the reset and
// changes nothing: the unwind says so with git's words beneath, names the manual step,
// claims no reset and takes no further step, so nothing of the user's is lost.
func TestUnwind_ResetRefusedOverAChangeOfTheUsers_SaysSoWithTheManualStep(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.SeedSequence("git", nothingStaged, gitFailing(keepRefusalStderr))
	rec := &presentertest.RecordingPresenter{}

	err := engine.Unwind(t.Context(), newDeps(rec, f), startState(startingSHA, "v0.0.1"), engine.MadeState{Commits: 1}, errUnwindReason)

	assertAbortNonZero(t, err)
	onlyWarning(t, rec, unwindIncomplete(keepStep, keepRefusalStderr))
	assertManualSteps(t, rec, keepMove)
	if got, want := unwoundText(t, rec), "manual cleanup required"; got != want {
		t.Errorf("Unwound.Text() = %q, want %q", got, want)
	}
	// After the refused reset, only the reads of where HEAD stands, for the close.
	want := []string{
		"git diff --cached --name-only -z --no-renames",
		"git reset --keep " + startingSHA,
		"git rev-parse HEAD --abbrev-ref HEAD",
		"git symbolic-ref --short -q HEAD",
	}
	if got := commandLines(f.Invocations()); !slices.Equal(got, want) {
		t.Errorf("git steps = %q, want only the refused reset %q", got, want)
	}
}

// A restage that fails after the reset went through says so with the step that stages
// the paths again, each pathspec quoted so the shell hands it to git as one word, while
// the prose names the file raw.
func TestUnwind_RestageFails_SaysSoWithAStepThatSplitsBackToThePathspec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		file     string
		pathspec string
	}{
		{name: "a plain name", file: "NEXT_RELEASE.md", pathspec: "':(top,literal)NEXT_RELEASE.md'"},
		{name: "a name holding a single quote", file: "notes/what's-new.md", pathspec: `':(top,literal)notes/what'\''s-new.md'`},
		{name: "a name holding a pair of single quotes", file: "notes/'draft'.md", pathspec: `':(top,literal)notes/'\''draft'\''.md'`},
		{name: "a name holding a space", file: "release notes.md", pathspec: "':(top,literal)release notes.md'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := runner.NewFakeRunner()
			f.SeedSequence("git", ScriptedOut(tt.file+"\x00"), ScriptedOut(stagedTree+"\n"), ScriptedOut(""), gitFailing(stageStderr))
			rec := &presentertest.RecordingPresenter{}

			err := engine.Unwind(t.Context(), newDeps(rec, f), startState(startingSHA, "v0.0.1"), engine.MadeState{Commits: 1}, errUnwindReason)

			assertAbortNonZero(t, err)
			restage := "git restore --source=" + stagedTree + " --staged -- " + tt.pathspec
			onlyWarning(t, rec, unwindIncomplete("stage your changes to "+tt.file+" again (`"+restage+"`)", stageStderr))
			assertManualSteps(t, rec, presenter.NextMove{Lead: "Stage your changes to " + tt.file + " again:", Command: restage})
			if got, want := unwoundText(t, rec), "reset 1 commit; manual cleanup required"; got != want {
				t.Errorf("Unwound.Text() = %q, want %q", got, want)
			}
			words, err := cmdline.Split(restage)
			if err != nil {
				t.Fatalf("Split(restage step) error = %v", err)
			}
			want := []string{"git", "restore", "--source=" + stagedTree, "--staged", "--", ":(top,literal)" + tt.file}
			if !slices.Equal(words, want) {
				t.Errorf("restage step splits to %q, want %q", words, want)
			}
		})
	}
}
