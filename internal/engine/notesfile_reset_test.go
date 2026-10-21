package engine_test

import (
	"context"
	"errors"
	"slices"
	"strings"
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

// keepingReset scripts a successful reset that leaves the notes file as it was.
func keepingReset() []runner.ScriptedCall {
	return []runner.ScriptedCall{
		ScriptedOut(""), // reset --soft <start>
		ScriptedOut(""), // restore --source=<start> --staged --worktree -- :(exclude)<notes>
	}
}

// assertResetKeptNotesFile fails unless the run moved HEAD back to start and restored
// the index and work tree to start everywhere but the notes file, and issued no
// `git reset --hard`, which would discard the file's edits and staged state.
func assertResetKeptNotesFile(t *testing.T, f *runner.FakeRunner, start, rel string) {
	t.Helper()
	assertGitArgs(t, f, 1, []string{"reset", "--soft", start})
	assertGitArgs(t, f, 1, []string{"restore", "--source=" + start, "--staged", "--worktree", "--", excludePathspec(rel)})
	if got := argsOf(f, "reset", "--hard"); len(got) != 0 {
		t.Errorf("a reset discarded the notes file's edits: %q", got)
	}
	assertNotesFileOnlyExcluded(t, f, rel)
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
			f.SeedSequence("git", firstReleaseGates(root, "")...)
			f.SeedSequence("git",
				ScriptedOut(" M bundle.js\n"), // -C root status --porcelain (post-hook: dirty)
				ScriptedOut(""),               // -C root add -A
			)
			f.SeedSequence("git", keepOutCommit(tt.notesStaged, stagedArtifacts)...)
			f.SeedSequence("git", keepingReset()...)
			f.Seed("sh", runner.Result{}, nil)
			rec := &presentertest.RecordingPresenter{
				NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo},
			}

			err := engine.Release(t.Context(), newDeps(rec, f), inRepoNotesFileOptions(root, tt.file))
			assertAbortNonZero(t, err)

			assertResetKeptNotesFile(t, f, startingSHA, tt.file)
			if got, want := unwoundSummary(t, rec), "reset 1 commit; repo clean"; got != want {
				t.Errorf("Unwound.Summary = %q, want %q", got, want)
			}
			assertNoMutation(t, f)
			assertFileUnchanged(t, path, suppliedBody)
		})
	}
}

// seedThroughBookkeepingCommit scripts a first release with an in-repo notes file up
// to and including its bookkeeping commit, with stash as for firstReleaseGates.
func seedThroughBookkeepingCommit(f *runner.FakeRunner, root, notesStaged string, stash ...runner.ScriptedCall) {
	f.SeedSequence("git", firstReleaseGates(root, "", stash...)...)
	f.SeedSequence("git", ScriptedOut("")) // -C root add CHANGELOG.md
	f.SeedSequence("git", keepOutCommit(notesStaged, stagedChangelog)...)
	f.SeedSequence("git", ScriptedOut(githubRemoteURL)) // remote get-url origin
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
			f.SeedSequence("git", keepingReset()...)
			rec := &presentertest.RecordingPresenter{}

			err := engine.Release(t.Context(), newDeps(rec, f), inRepoNotesFileOptions(root, tt.file))
			assertAbortNonZero(t, err)

			assertGitArgs(t, f, 2, []string{"tag", "-d", "v0.0.1"})
			assertResetKeptNotesFile(t, f, startingSHA, tt.file)
			if got, want := unwoundSummary(t, rec), "reset 1 commit and deleted tag v0.0.1; repo clean"; got != want {
				t.Errorf("Unwound.Summary = %q, want %q", got, want)
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
			f.SeedSequence("git", keepingReset()...)
			rec := &presentertest.RecordingPresenter{}

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			err := engine.Release(ctx, newDeps(rec, f), inRepoNotesFileOptions(root, tt.file))
			assertAbortNonZero(t, err)

			if got := argsOf(f, "push"); len(got) != 0 {
				t.Errorf("a cancelled run pushed: %q", got)
			}
			assertResetKeptNotesFile(t, f, startingSHA, tt.file)
			if got, want := unwoundSummary(t, rec), "reset 1 commit; repo clean"; got != want {
				t.Errorf("Unwound.Summary = %q, want %q", got, want)
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
			f.SeedSequence("git", keepingReset()...)
			f.SeedSequence("git", stashPopped())
			rec := &presentertest.RecordingPresenter{}

			opts := inRepoNotesFileOptions(root, tt.file)
			opts.AutoStash = true
			err := engine.Release(t.Context(), newDeps(rec, f), opts)
			assertAbortNonZero(t, err)

			assertGitArgs(t, f, 2, []string{"stash", "push", "--include-untracked", "--", excludePathspec(tt.file)})
			assertResetKeptNotesFile(t, f, startingSHA, tt.file)
			restoreAt := indexOfCmd(f, "git", "restore", "--source="+startingSHA, "--staged", "--worktree", "--", excludePathspec(tt.file))
			popAt := indexOfCmd(f, "git", "stash", "pop")
			if popAt == -1 || restoreAt == -1 || popAt < restoreAt {
				t.Errorf("stash pop at %d, reset's restore at %d: the other changes must be popped after the unwind", popAt, restoreAt)
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
			f.SeedSequence("git", keepingReset()...)
			rec := &presentertest.RecordingPresenter{}

			err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), nil, root, notesFileRunReq(t, path, engine.RegenerateTargetChangelog))
			assertAbortNonZero(t, err)

			assertResetKeptNotesFile(t, f, "startHEAD", tt.file)
			if got := argsOf(f, "tag"); len(got) != 0 {
				t.Errorf("regenerate's reset touched a tag: %q", got)
			}
			assertFileUnchanged(t, path, suppliedBody)
		})
	}
}

func TestUnwind_KeepOut_ResetsAroundTheKeptPath(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.SeedSequence("git", ScriptedOut("")) // tag -d v0.0.1
	f.SeedSequence("git", keepingReset()...)
	rec := &presentertest.RecordingPresenter{}

	start := startState(startingSHA, "v0.0.1")
	start.KeepOut = git.Exclusion("NEXT_RELEASE.md")
	err := engine.Unwind(t.Context(), newDeps(rec, f), start, engine.MadeState{Commits: 2, TagCreated: true}, errUnwindReason)

	assertAbortNonZero(t, err)
	assertResetKeptNotesFile(t, f, startingSHA, "NEXT_RELEASE.md")
	if got, want := unwoundSummary(t, rec), "reset 2 commits and deleted tag v0.0.1; repo clean"; got != want {
		t.Errorf("Unwound.Summary = %q, want %q", got, want)
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

func TestUnwind_KeepOut_ResetStepFails_WarnsTheManualStepsLeft(t *testing.T) {
	t.Parallel()

	const (
		soft    = "`git reset --soft " + startingSHA + "`"
		restore = "`git restore --source=" + startingSHA + " --staged --worktree -- ':(top,literal,exclude)NEXT_RELEASE.md'`"
	)
	tests := []struct {
		name  string
		calls []runner.ScriptedCall
		want  string
	}{
		{
			name:  "the soft reset fails",
			calls: []runner.ScriptedCall{scriptedMutateFailure()},
			want:  "automatic recovery failed; manually reset HEAD back to " + startingSHA + ", leaving NEXT_RELEASE.md as it is (" + soft + " then " + restore + ")",
		},
		{
			name:  "the restore fails",
			calls: []runner.ScriptedCall{ScriptedOut(""), scriptedMutateFailure()},
			want:  "automatic recovery failed; manually restore the index and work tree to " + startingSHA + ", leaving NEXT_RELEASE.md as it is (" + restore + ")",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := runner.NewFakeRunner()
			f.SeedSequence("git", tt.calls...)
			rec := &presentertest.RecordingPresenter{}

			start := startState(startingSHA, "v0.0.1")
			start.KeepOut = git.Exclusion("NEXT_RELEASE.md")
			err := engine.Unwind(t.Context(), newDeps(rec, f), start, engine.MadeState{Commits: 1}, errUnwindReason)

			assertAbortNonZero(t, err)
			if got := recoveryWarn(t, rec).Warn.Message; got != tt.want {
				t.Errorf("recovery Warn message = %q, want %q", got, tt.want)
			}
			if got, want := unwoundSummary(t, rec), "reset 1 commit; manual cleanup required"; got != want {
				t.Errorf("Unwound.Summary = %q, want %q", got, want)
			}
		})
	}
}

func TestRegenerateWrite_KeepOut_RecoveryResetFails_WarnsTheManualStepsLeft(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, regenNotesStaleChangelog)
	f := runner.NewFakeRunner()
	f.SeedSequence("git", regenChangelogWrite()...)
	f.SeedSequence("git", keepOutCommit("", stagedChangelog)...)
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("remote rejected")}, // push origin HEAD
		scriptedMutateFailure(), // reset --soft startHEAD
	)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	req := freshWriteReq(engine.RegenerateTargetChangelog)
	req.KeepOut = git.Exclusion("notes.md")
	err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), nil, dir, req)

	assertAbortNonZero(t, err)
	want := "automatic recovery failed; manually reset HEAD back to startHEAD, leaving notes.md as it is " +
		"(`git reset --soft startHEAD` then `git restore --source=startHEAD --staged --worktree -- ':(top,literal,exclude)notes.md'`)" +
		" to drop the leftover changelog commit"
	if got := recoveryWarn(t, rec).Warn.Message; got != want {
		t.Errorf("recovery Warn message = %q, want %q", got, want)
	}
}

// A notes file whose name holds a single quote still gets restore steps the operator can
// paste: the pathspec is written the way cmdline.Quote writes it, while the prose names
// the file raw.
func TestUnwind_KeepOut_ResetStepFails_QuotesThePathspecForTheShell(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		file     string
		pathspec string
	}{
		{name: "a name holding a single quote", file: "notes/what's-new.md", pathspec: `':(top,literal,exclude)notes/what'\''s-new.md'`},
		{name: "a name holding a pair of single quotes", file: "notes/'draft'.md", pathspec: `':(top,literal,exclude)notes/'\''draft'\''.md'`},
		{name: "a name holding a space", file: "release notes.md", pathspec: "':(top,literal,exclude)release notes.md'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			restore := "`git restore --source=" + startingSHA + " --staged --worktree -- " + tt.pathspec + "`"
			steps := []struct {
				name  string
				calls []runner.ScriptedCall
				want  string
			}{
				{
					name:  "the soft reset fails",
					calls: []runner.ScriptedCall{scriptedMutateFailure()},
					want: "automatic recovery failed; manually reset HEAD back to " + startingSHA + ", leaving " + tt.file +
						" as it is (`git reset --soft " + startingSHA + "` then " + restore + ")",
				},
				{
					name:  "the restore fails",
					calls: []runner.ScriptedCall{ScriptedOut(""), scriptedMutateFailure()},
					want: "automatic recovery failed; manually restore the index and work tree to " + startingSHA + ", leaving " + tt.file +
						" as it is (" + restore + ")",
				},
			}
			for _, step := range steps {
				f := runner.NewFakeRunner()
				f.SeedSequence("git", step.calls...)
				rec := &presentertest.RecordingPresenter{}

				start := startState(startingSHA, "v0.0.1")
				start.KeepOut = git.Exclusion(tt.file)
				err := engine.Unwind(t.Context(), newDeps(rec, f), start, engine.MadeState{Commits: 1}, errUnwindReason)

				assertAbortNonZero(t, err)
				if got := recoveryWarn(t, rec).Warn.Message; got != step.want {
					t.Errorf("%s: recovery Warn message = %q, want %q", step.name, got, step.want)
				}
			}
		})
	}
}

// Read back by POSIX shell rules, the printed restore step hands git the keep-out
// pathspec as one whole word, so the restore leaves the notes file out and never names
// a path with the quotes stripped.
func TestUnwind_KeepOut_RestoreStepSplitsBackToTheKeepOutPathspec(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.SeedSequence("git", ScriptedOut(""), scriptedMutateFailure())
	rec := &presentertest.RecordingPresenter{}

	start := startState(startingSHA, "v0.0.1")
	start.KeepOut = git.Exclusion("notes/'draft'.md")
	err := engine.Unwind(t.Context(), newDeps(rec, f), start, engine.MadeState{Commits: 1}, errUnwindReason)

	assertAbortNonZero(t, err)
	msg := recoveryWarn(t, rec).Warn.Message
	open := strings.Index(msg, "`")
	shut := strings.LastIndex(msg, "`")
	if open < 0 || shut <= open {
		t.Fatalf("recovery Warn message %q carries no backticked step", msg)
	}
	words, err := cmdline.Split(msg[open+1 : shut])
	if err != nil {
		t.Fatalf("Split(restore step) error = %v", err)
	}
	want := []string{"git", "restore", "--source=" + startingSHA, "--staged", "--worktree", "--", ":(top,literal,exclude)notes/'draft'.md"}
	if !slices.Equal(words, want) {
		t.Errorf("restore step splits to %q, want %q", words, want)
	}
}

func TestRegenerateWrite_KeepOut_RecoveryResetFails_QuotesThePathspecForTheShell(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, regenNotesStaleChangelog)
	f := runner.NewFakeRunner()
	f.SeedSequence("git", regenChangelogWrite()...)
	f.SeedSequence("git", keepOutCommit("", stagedChangelog)...)
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("remote rejected")}, // push origin HEAD
		scriptedMutateFailure(), // reset --soft startHEAD
	)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	req := freshWriteReq(engine.RegenerateTargetChangelog)
	req.KeepOut = git.Exclusion("notes/what's-new.md")
	err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), nil, dir, req)

	assertAbortNonZero(t, err)
	want := "automatic recovery failed; manually reset HEAD back to startHEAD, leaving notes/what's-new.md as it is " +
		`(` + "`git reset --soft startHEAD` then `git restore --source=startHEAD --staged --worktree -- ':(top,literal,exclude)notes/what'\\''s-new.md'`" + `)` +
		" to drop the leftover changelog commit"
	if got := recoveryWarn(t, rec).Warn.Message; got != want {
		t.Errorf("recovery Warn message = %q, want %q", got, want)
	}
}
