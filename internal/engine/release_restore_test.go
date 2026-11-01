package engine_test

// This file pins the unwind's restore of a tree a release wrote or staged in before
// any commit of its own held it: the version file projected, the changelog written,
// pre_tag's changes staged. The unwind resets to the starting commit as it does once
// mint has committed, a notes file inside the repo keeping its content and staged
// state, and the autostash popping after it. The fake runner never moves git's tree,
// so these runs pin the exact git steps the unwind takes and the bytes of what mint
// must leave alone.

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// gitStep is one git call a run makes: its command line, and git's answer.
type gitStep struct {
	line string
	call runner.ScriptedCall
}

func scripted(steps []gitStep) []runner.ScriptedCall {
	out := make([]runner.ScriptedCall, len(steps))
	for i, s := range steps {
		out[i] = s.call
	}
	return out
}

func commandLinesOf(steps []gitStep) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.line
	}
	return out
}

// restoreRepo is the repo a restoring run releases from: its root, and the notes file
// it keeps out, "" for none, with whether that file has edits staged.
type restoreRepo struct {
	root        string
	notes       string
	notesStaged bool
}

func (r restoreRepo) git(args string) string { return "git -C " + r.root + " " + args }

// keepingOut is the exclusion pathspec a step ends with while a notes file is kept out.
func (r restoreRepo) keepingOut() string {
	if r.notes == "" {
		return ""
	}
	return " -- " + excludePathspec(r.notes)
}

// commit is the commit of what is staged under subject, as CommitStaged makes it,
// git answering it with outcome; staged is the staged-path list it reads where the
// notes file has staged edits.
func (r restoreRepo) commit(subject, staged string, outcome runner.ScriptedCall) []gitStep {
	if r.notes == "" {
		return []gitStep{{r.git("commit -m " + subject), outcome}}
	}
	probe := r.git("diff --cached --name-only -z --no-renames -- :(top,literal)" + r.notes)
	if !r.notesStaged {
		return []gitStep{{probe, ScriptedOut("")}, {r.git("commit -m " + subject), outcome}}
	}
	return []gitStep{
		{probe, ScriptedOut(r.notes + "\x00")},
		{r.git("diff --cached --name-only -z --no-renames" + r.keepingOut()), ScriptedOut(staged)},
		{r.git("commit -m " + subject + " --pathspec-from-file=- --pathspec-file-nul"), outcome},
	}
}

// artifacts is the pre_tag hook leaving bundle.js changed and mint committing it,
// git answering the commit with outcome.
func (r restoreRepo) artifacts(outcome runner.ScriptedCall) []gitStep {
	return append([]gitStep{
		{r.git("status --porcelain" + r.keepingOut()), ScriptedOut(" M bundle.js\n")},
		{r.git("add -A" + r.keepingOut()), ScriptedOut("")},
	}, r.commit(pretagArtifactSubject("v0.0.1"), stagedArtifacts, outcome)...)
}

// reset is the unwind's reset to the starting commit, around the notes file where one
// is kept out.
func (r restoreRepo) reset() []gitStep {
	if r.notes == "" {
		return []gitStep{{"git reset --hard " + startingSHA, ScriptedOut("")}}
	}
	return []gitStep{
		{"git reset --soft " + startingSHA, ScriptedOut("")},
		{"git restore --source=" + startingSHA + " --staged --worktree" + r.keepingOut(), ScriptedOut("")},
	}
}

const (
	bookkeepingSubject = "🌿 Release v0.0.1"
	// previousVersion is what VERSION holds at the start, the version file the record
	// stage writes over.
	previousVersion = "0.0.0\n"
)

// treeFailure is a release stopping once mint wrote or staged in the tree and before
// a commit held it.
type treeFailure struct {
	name   string
	config string
	// changelogUnreadable makes CHANGELOG.md a directory, so the changelog write fails.
	changelogUnreadable bool
	// fails is the git steps from the stage's start to its failure.
	fails func(restoreRepo) []gitStep
	// undone is what the unwind says it undid with no commit to reset, afterCommit
	// once the pre_tag hook's artifacts were committed first.
	undone, afterCommit string
	// end is the close's detail.
	end string
}

var (
	changelogWriteFails = treeFailure{
		name:                "the version file projected, then the changelog write failing",
		config:              plainVersionFile,
		changelogUnreadable: true,
		fails:               func(restoreRepo) []gitStep { return nil },
		undone:              "restored VERSION",
		afterCommit:         "reset 1 commit and restored VERSION",
		end:                 "back where it started · nothing pushed",
	}
	stagedBookkeepingFails = treeFailure{
		name:   "the bookkeeping commit failing once its files are staged",
		config: plainVersionFile,
		fails: func(r restoreRepo) []gitStep {
			return append([]gitStep{{r.git("add CHANGELOG.md VERSION"), ScriptedOut("")}},
				r.commit(bookkeepingSubject, "CHANGELOG.md\x00VERSION\x00", gitFailing(commitStderr))...)
		},
		undone:      "restored VERSION and removed CHANGELOG.md",
		afterCommit: "reset 1 commit, restored VERSION and removed CHANGELOG.md",
		end:         "back where it started · nothing pushed",
	}
	bookkeepingStageFails = treeFailure{
		name:   "the bookkeeping commit failing to stage its files",
		config: plainVersionFile,
		fails: func(r restoreRepo) []gitStep {
			return []gitStep{{r.git("add CHANGELOG.md VERSION"), gitFailing(stageStderr)}}
		},
		undone:      "restored VERSION",
		afterCommit: "reset 1 commit and restored VERSION",
		end:         "back where it started · CHANGELOG.md left untracked · nothing pushed",
	}
	artifactsCommitFails = treeFailure{
		name:   "pre_tag's changes staged, then their commit failing",
		config: preTagHookConfig,
		fails:  func(r restoreRepo) []gitStep { return r.artifacts(gitFailing(commitStderr)) },
		undone: "unstaged and removed pre_tag's changes",
		end:    "back where it started · nothing pushed",
	}
	treeFailures = []treeFailure{changelogWriteFails, stagedBookkeepingFails, bookkeepingStageFails, artifactsCommitFails}
)

// restoringRun is a first release from repo stopped by failure, the git steps from
// the starting commit's capture on being before, then the failure's, then the reset,
// then after. resetAnswers answers the reset's steps in place of success, and stash
// scripts an autostash's push.
type restoringRun struct {
	repo         restoreRepo
	failure      treeFailure
	config       string
	before       []gitStep
	after        []gitStep
	resetAnswers []runner.ScriptedCall
	stash        []runner.ScriptedCall
	opts         func(*engine.ReleaseOptions)
}

// tail is every git step the run takes once it has captured its starting commit.
func (r restoringRun) tail() []gitStep {
	reset := r.repo.reset()
	for i, answer := range r.resetAnswers {
		reset[i].call = answer
	}
	return slices.Concat(r.before, r.failure.fails(r.repo), reset, r.after)
}

// release runs r, returning what it recorded, its runner and its error.
func (r restoringRun) release(t *testing.T) (*presentertest.RecordingPresenter, *runner.FakeRunner, error) {
	t.Helper()
	rec := &presentertest.RecordingPresenter{}
	f, err := r.releaseTo(t, rec)
	return rec, f, err
}

// releaseTo runs r narrating to p, returning its runner and its error.
func (r restoringRun) releaseTo(t *testing.T, p presenter.Presenter) (*runner.FakeRunner, error) {
	t.Helper()
	root := r.repo.root
	writeConfig(t, root, r.failure.config+r.config)
	if strings.Contains(r.failure.config, "version_file") {
		writeRepoFile(t, root, "VERSION", previousVersion)
	}
	if r.failure.changelogUnreadable {
		if err := os.Mkdir(filepath.Join(root, "CHANGELOG.md"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f := runner.NewFakeRunner()
	seedProvider(f)
	if r.repo.notes != "" {
		f.SeedSequence("git", firstReleaseGates(root, "", r.stash...)...)
	} else {
		f.SeedSequence("git", bareFirstReleaseGates(root, r.stash...)...)
	}
	f.SeedSequence("git", scripted(r.tail())...)
	f.Seed("sh", runner.Result{}, nil)
	opts := patchOptions()
	if r.repo.notes != "" {
		opts = inRepoNotesFileOptions(root, r.repo.notes)
	}
	if r.opts != nil {
		r.opts(&opts)
	}
	deps := newDeps(nil, f)
	deps.Presenter = p
	deps.Clock = func() time.Time { return railEpoch }
	err := engine.Release(t.Context(), deps, opts)
	return f, err
}

// bareFirstReleaseGates is firstReleaseGates for a release with no notes file kept
// out, which probes no ignore rule.
func bareFirstReleaseGates(root string, stash ...runner.ScriptedCall) []runner.ScriptedCall {
	gates := firstReleaseGates(root, "", stash...)
	return slices.Delete(gates, 1, 2)
}

func writeRepoFile(t *testing.T, root, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

// assertGitTail fails unless the git steps the run took after capturing its starting
// commit are want, exactly and in order.
func assertGitTail(t *testing.T, f *runner.FakeRunner, want []string) {
	t.Helper()
	var got []string
	captured := false
	for _, inv := range invocationsNamed(f, "git") {
		line := commandLine(inv)
		if captured {
			got = append(got, line)
		}
		captured = captured || line == "git rev-parse HEAD"
	}
	if !slices.Equal(got, want) {
		t.Errorf("git after the start's capture =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// assertTreeRestored fails unless the run stopped, unwound saying undone and closed
// unwound at its start station saying end.
func assertTreeRestored(t *testing.T, rec *presentertest.RecordingPresenter, err error, undone, end string) {
	t.Helper()
	assertAbortNonZero(t, err)
	assertStageFailedThenUnwound(t, rec)
	if got := unwoundEvent(t, rec); got.Undone != undone || got.Repo != "repo clean" {
		t.Errorf("Unwound = %+v, want %q with the repo clean", got, undone)
	}
	assertClosesUnwound(t, rec, end)
	if fin := closingEvent(t, rec); fin.End.Name != startStation(t, rec).Name {
		t.Errorf("closed at %q, want the start station %q", fin.End.Name, startStation(t, rec).Name)
	}
}

func unwoundEvent(t *testing.T, rec *presentertest.RecordingPresenter) presenter.Unwind {
	t.Helper()
	n := indexOfKind(rec, presentertest.KindUnwound)
	if n == -1 {
		t.Fatalf("no Unwound recorded; kinds = %v", rec.Kinds())
	}
	return rec.Events[n].Unwound
}

func TestRelease_AFailureBeforeAnyCommit_ResetsTheTreeMintWroteOrStagedIn(t *testing.T) {
	t.Parallel()

	for _, tf := range treeFailures {
		t.Run(tf.name, func(t *testing.T) {
			t.Parallel()

			run := restoringRun{repo: restoreRepo{root: t.TempDir()}, failure: tf}
			rec, f, err := run.release(t)

			assertTreeRestored(t, rec, err, tf.undone, tf.end)
			assertGitTail(t, f, commandLinesOf(run.tail()))
		})
	}
}

func TestRelease_AFailureAfterAnArtifactsCommit_ResetsTheCommitAndTheTreeTogether(t *testing.T) {
	t.Parallel()

	for _, tf := range treeFailures {
		if tf.afterCommit == "" {
			// pre_tag's artifacts commit is the run's first: nothing commits before it.
			continue
		}
		t.Run(tf.name, func(t *testing.T) {
			t.Parallel()

			repo := restoreRepo{root: t.TempDir()}
			run := restoringRun{repo: repo, failure: tf, config: preTagHookConfig, before: repo.artifacts(ScriptedOut(""))}
			rec, f, err := run.release(t)

			assertTreeRestored(t, rec, err, tf.afterCommit, tf.end)
			assertGitTail(t, f, commandLinesOf(run.tail()))
		})
	}
}

func TestRelease_AFailureBeforeAnyCommit_ResetsAroundAnInRepoNotesFile(t *testing.T) {
	t.Parallel()

	for _, tf := range treeFailures {
		for _, nc := range notesResetCases {
			t.Run(tf.name+"/"+nc.name, func(t *testing.T) {
				t.Parallel()

				repo := restoreRepo{root: t.TempDir(), notes: nc.file, notesStaged: nc.notesStaged != ""}
				path := writeNotesFile(t, repo.root, nc.file, suppliedBody)
				run := restoringRun{repo: repo, failure: tf}
				rec, f, err := run.release(t)

				assertTreeRestored(t, rec, err, tf.undone, tf.end)
				assertGitTail(t, f, commandLinesOf(run.tail()))
				assertResetKeptNotesFile(t, f, startingSHA, nc.file)
				assertFileUnchanged(t, path, suppliedBody)
			})
		}
	}
}

func TestRelease_AFailureBeforeAnyCommit_Autostash_PopsOntoTheRestoredTree(t *testing.T) {
	t.Parallel()

	for _, tf := range treeFailures {
		t.Run(tf.name, func(t *testing.T) {
			t.Parallel()

			run := restoringRun{
				repo:    restoreRepo{root: t.TempDir()},
				failure: tf,
				stash:   []runner.ScriptedCall{stashSaved()},
				after:   []gitStep{{"git stash pop", stashPopped()}},
				opts:    func(o *engine.ReleaseOptions) { o.AutoStash = true },
			}
			rec, f, err := run.release(t)

			assertTreeRestored(t, rec, err, tf.undone, tf.end)
			assertGitArgs(t, f, 2, []string{"stash", "push", "--include-untracked"})
			// The tail ends reset then pop: the WIP goes back onto the restored tree.
			assertGitTail(t, f, commandLinesOf(run.tail()))
		})
	}
}

func TestRelease_AFailureBeforeAnyCommit_ARestoreThatFails_SaysSoWithTheManualStep(t *testing.T) {
	t.Parallel()

	const (
		hardReset = "automatic recovery failed; manually reset HEAD back to " + startingSHA + " (`git reset --hard " + startingSHA + "`)"
		softReset = "automatic recovery failed; manually restore the index and work tree to " + startingSHA +
			", leaving NEXT_RELEASE.md as it is (`git restore --source=" + startingSHA +
			" --staged --worktree -- ':(top,literal,exclude)NEXT_RELEASE.md'`)"
	)

	tests := []struct {
		name    string
		failure treeFailure
		notes   string
		// reset is git's answers to the reset's steps.
		reset   []runner.ScriptedCall
		warn    string
		command string
		end     string
	}{
		{
			name:    "the record's files",
			failure: stagedBookkeepingFails,
			reset:   []runner.ScriptedCall{gitFailing(resetStderr)},
			warn:    hardReset,
			command: "git reset --hard " + startingSHA,
			end:     "manual cleanup required · VERSION + CHANGELOG.md written, not committed · nothing pushed",
		},
		{
			name:    "pre_tag's changes",
			failure: artifactsCommitFails,
			reset:   []runner.ScriptedCall{gitFailing(resetStderr)},
			warn:    hardReset,
			command: "git reset --hard " + startingSHA,
			end:     "manual cleanup required · pre_tag's changes staged, not committed · nothing pushed",
		},
		{
			name:    "the record's files, around an in-repo notes file",
			failure: stagedBookkeepingFails,
			notes:   "NEXT_RELEASE.md",
			reset:   []runner.ScriptedCall{ScriptedOut(""), gitFailing(resetStderr)},
			warn:    softReset,
			command: "git restore --source=" + startingSHA + " --staged --worktree -- ':(top,literal,exclude)NEXT_RELEASE.md'",
			end:     "manual cleanup required · VERSION + CHANGELOG.md written, not committed · nothing pushed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := restoreRepo{root: t.TempDir(), notes: tt.notes}
			if tt.notes != "" {
				writeNotesFile(t, repo.root, tt.notes, suppliedBody)
			}
			run := restoringRun{repo: repo, failure: tt.failure, resetAnswers: tt.reset}
			rec, f, err := run.release(t)

			assertAbortNonZero(t, err)
			warn := recoveryWarn(t, rec).Warn
			if warn.Message != tt.warn || warn.Output != resetStderr || !warn.Failed {
				t.Errorf("recovery warning = %+v, want the failed step %q with git's output", warn, tt.warn)
			}
			if got := unwoundEvent(t, rec); got.Undone != "" || got.Repo != "manual cleanup required" {
				t.Errorf("Unwound = %+v, want nothing claimed undone and manual cleanup required", got)
			}
			if got := nextMoveCommands(rec); !slices.Equal(got, []string{tt.command}) {
				t.Errorf("next moves' commands = %q, want the manual step %q", got, tt.command)
			}
			assertClosesUnwound(t, rec, tt.end)
			assertGitTail(t, f, commandLinesOf(run.tail()))
		})
	}
}

// nextMoveCommands is the command of every next move the run recorded.
func nextMoveCommands(rec *presentertest.RecordingPresenter) []string {
	var commands []string
	for _, ev := range rec.Events {
		if ev.Kind != presentertest.KindNextMoves {
			continue
		}
		for _, move := range ev.NextMoves.Moves {
			commands = append(commands, move.Command)
		}
	}
	return commands
}

func TestRelease_AFailureWithNothingWrittenOrStaged_IssuesNoResetAndNoUnwound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  string
		version string
		hook    runner.ScriptedCall
		git     []runner.ScriptedCall
	}{
		{name: "a pre_tag hook that fails", config: preTagHookConfig, hook: hookFails},
		{name: "pre_tag's changes failing to stage", config: preTagHookConfig, git: []runner.ScriptedCall{dirtyAfterHook, gitFailing(stageStderr)}},
		{
			name:    "a version pattern matching nothing in the version file",
			config:  "[release]\nversion_file = \"version.go\"\nversion_pattern = 'const Version = \"(.*)\"'\n",
			version: "package main\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeConfig(t, root, tt.config)
			if tt.version != "" {
				writeRepoFile(t, root, "version.go", tt.version)
			}
			f := runner.NewFakeRunner()
			seedProvider(f)
			f.SeedSequence("git", bareFirstReleaseGates(root)...)
			f.SeedSequence("git", tt.git...)
			f.SeedSequence("sh", tt.hook)
			rec := &presentertest.RecordingPresenter{}

			err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())

			assertAbortNonZero(t, err)
			if recorded(rec, presentertest.KindUnwound) {
				t.Error("an unwind with nothing to take back emitted an Unwound")
			}
			for _, args := range [][]string{{"reset"}, {"restore"}} {
				if got := argsOf(f, args...); len(got) != 0 {
					t.Errorf("an unwind with nothing to take back ran git %q", got)
				}
			}
		})
	}
}

func TestRelease_ARestoreWithNoCommit_PlainTranscripts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		failure treeFailure
		// reset is git's answers to the reset's steps, each succeeding where nil.
		reset []runner.ScriptedCall
		want  string
	}{
		{
			name:    "pre_tag's changes, then a restore that fails",
			failure: artifactsCommitFails,
			reset:   []runner.ScriptedCall{gitFailing(resetStderr)},
			want: "mint: releasing pigeon v0.0.1\n" +
				"version: v0.0.1 (-y)\n" +
				"preflight: tree clean, on main, v0.0.1 free, origin in sync, gh signed in\n" +
				"pre_tag: running...\n" +
				"pre_tag: FAILED - committing dirty tree \"chore(release): pre-tag artifacts for v0.0.1\": exit status 1\n" +
				"--- output ---\n" +
				commitStderr + "\n" +
				"--- end output ---\n" +
				"unwind incomplete: WARN - automatic recovery failed; manually reset HEAD back to " + startingSHA + " (`git reset --hard " + startingSHA + "`)\n" +
				"--- output ---\n" +
				resetStderr + "\n" +
				"--- end output ---\n" +
				"unwound: manual cleanup required\n" +
				"--- stderr ---\n" +
				"pre_tag: FAILED - committing dirty tree \"chore(release): pre-tag artifacts for v0.0.1\": exit status 1\n" +
				"unwind incomplete: WARN - automatic recovery failed; manually reset HEAD back to " + startingSHA + " (`git reset --hard " + startingSHA + "`)\n",
		},
		{
			name:    "the record's files",
			failure: stagedBookkeepingFails,
			want: "mint: releasing pigeon v0.0.1\n" +
				"version: v0.0.1 (-y)\n" +
				"preflight: tree clean, on main, v0.0.1 free, origin in sync, gh signed in\n" +
				"notes: running...\n" +
				"notes: generated (0.0s)\n" +
				"plan: commit bookkeeping; tag v0.0.1; publish v0.0.1\n" +
				"--- release notes v0.0.1 ---\n" +
				"Initial release.\n" +
				"--- end notes ---\n" +
				"notes: accepted (-y)\n" +
				"record: FAILED - committing release bookkeeping \"🌿 Release v0.0.1\": exit status 1\n" +
				"--- output ---\n" +
				commitStderr + "\n" +
				"--- end output ---\n" +
				"unwound: restored VERSION and removed CHANGELOG.md; repo clean\n" +
				"--- stderr ---\n" +
				"record: FAILED - committing release bookkeeping \"🌿 Release v0.0.1\": exit status 1\n",
		},
		{
			name:    "pre_tag's changes",
			failure: artifactsCommitFails,
			want: "mint: releasing pigeon v0.0.1\n" +
				"version: v0.0.1 (-y)\n" +
				"preflight: tree clean, on main, v0.0.1 free, origin in sync, gh signed in\n" +
				"pre_tag: running...\n" +
				"pre_tag: FAILED - committing dirty tree \"chore(release): pre-tag artifacts for v0.0.1\": exit status 1\n" +
				"--- output ---\n" +
				commitStderr + "\n" +
				"--- end output ---\n" +
				"unwound: unstaged and removed pre_tag's changes; repo clean\n" +
				"--- stderr ---\n" +
				"pre_tag: FAILED - committing dirty tree \"chore(release): pre-tag artifacts for v0.0.1\": exit status 1\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := filepath.Join(t.TempDir(), "pigeon")
			if err := os.Mkdir(root, 0o755); err != nil {
				t.Fatal(err)
			}
			p, read := plainUnderYes(t)
			run := restoringRun{repo: restoreRepo{root: root}, failure: tt.failure, resetAnswers: tt.reset}
			_, err := run.releaseTo(t, p)

			assertAbortNonZero(t, err)
			if got := read(); got != tt.want {
				t.Errorf("plain transcript =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}
