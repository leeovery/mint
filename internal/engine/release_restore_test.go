package engine_test

// This file pins the unwind putting back what a release wrote or staged before any
// commit of its own held it: the version file projected, the changelog written,
// pre_tag's changes staged. It restores exactly those paths to HEAD and removes the
// files mint created, naming every path it touches, so nothing else in the tree, a
// notes file inside the repo or the user's own edits, is touched; with no commit it
// moves no HEAD. The fake runner never moves git's tree, so these runs pin the exact
// git steps the unwind takes and the files mint removes itself.

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"mint/internal/cmdline"
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

// commitPaths is mint's commit of the paths it names under subject, whatever the notes
// file holds, git answering it with outcome.
func (r restoreRepo) commitPaths(subject string, outcome runner.ScriptedCall) gitStep {
	return gitStep{r.git("commit -m " + subject + " --pathspec-from-file=- --pathspec-file-nul"), outcome}
}

// commit is the artifact commit of what is staged under subject, as CommitStaged makes it,
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

// restore is the unwind returning exactly paths to HEAD, git answering with outcome.
func (r restoreRepo) restore(outcome runner.ScriptedCall, paths ...string) gitStep {
	return gitStep{r.git("restore --source=HEAD --staged --worktree -- " + literalPathspecsOf(paths)), outcome}
}

// stagedRead is the unwind reading which paths mint staged, the notes file left out,
// git answering staged.
func (r restoreRepo) stagedRead(staged runner.ScriptedCall) gitStep {
	return gitStep{r.git("diff --cached --name-only -z --no-renames" + r.keepingOut()), staged}
}

func literalPathspecsOf(paths []string) string {
	specs := make([]string, len(paths))
	for i, path := range paths {
		specs[i] = ":(top,literal)" + path
	}
	return strings.Join(specs, " ")
}

// stagedTree is the tree git writes the index as ahead of a reset.
const stagedTree = "5e1f0a7b"

// resetCommits is the unwind's reset of the commits mint made, a reset that keeps every
// change of the user's: a read of what is staged, then the reset, and where the notes
// file has edits staged, the index written as a tree first and the file staged again
// from it after.
func (r restoreRepo) resetCommits() []gitStep {
	read := r.git("diff --cached --name-only -z --no-renames")
	reset := gitStep{"git reset --keep " + startingSHA, ScriptedOut("")}
	if !r.notesStaged {
		return []gitStep{{read, nothingStaged}, reset}
	}
	return []gitStep{
		{read, ScriptedOut(r.notes + "\x00")},
		{"git write-tree", ScriptedOut(stagedTree + "\n")},
		reset,
		{"git restore --source=" + stagedTree + " --staged -- :(top,literal)" + r.notes, ScriptedOut("")},
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
	// restores is the unwind's steps putting back what mint wrote or staged, git
	// answering its restore with outcome.
	restores func(r restoreRepo, outcome runner.ScriptedCall) []gitStep
	// created is a file mint created and never staged, which the unwind removes itself.
	created string
	// undone is what the unwind says it undid with no commit to reset, afterCommit
	// once the pre_tag hook's artifacts were committed first.
	undone, afterCommit string
	// end is the close's detail, and unrestored its detail where the restore fails.
	end, unrestored string
}

var (
	changelogWriteFails = treeFailure{
		name:                "the version file projected, then the changelog write failing",
		config:              plainVersionFile,
		changelogUnreadable: true,
		fails:               func(restoreRepo) []gitStep { return nil },
		restores: func(r restoreRepo, outcome runner.ScriptedCall) []gitStep {
			return []gitStep{r.restore(outcome, "VERSION")}
		},
		undone:      "restored VERSION",
		afterCommit: "reset 1 commit and restored VERSION",
		end:         "back where it started · nothing pushed",
		unrestored:  "manual cleanup required · VERSION written, not committed · nothing pushed",
	}
	stagedBookkeepingFails = treeFailure{
		name:   "the bookkeeping commit failing once its files are staged",
		config: plainVersionFile,
		fails: func(r restoreRepo) []gitStep {
			return append([]gitStep{{r.git("add CHANGELOG.md VERSION"), ScriptedOut("")}},
				r.commitPaths(bookkeepingSubject, gitFailing(commitStderr)))
		},
		restores: func(r restoreRepo, outcome runner.ScriptedCall) []gitStep {
			return []gitStep{r.restore(outcome, "VERSION", "CHANGELOG.md")}
		},
		undone:      "restored VERSION and removed CHANGELOG.md",
		afterCommit: "reset 1 commit, restored VERSION and removed CHANGELOG.md",
		end:         "back where it started · nothing pushed",
		unrestored:  "manual cleanup required · VERSION + CHANGELOG.md written, not committed · nothing pushed",
	}
	bookkeepingStageFails = treeFailure{
		name:   "the bookkeeping commit failing to stage its files",
		config: plainVersionFile,
		fails: func(r restoreRepo) []gitStep {
			return []gitStep{{r.git("add CHANGELOG.md VERSION"), gitFailing(stageStderr)}}
		},
		restores: func(r restoreRepo, outcome runner.ScriptedCall) []gitStep {
			return []gitStep{r.restore(outcome, "VERSION")}
		},
		created:     "CHANGELOG.md",
		undone:      "restored VERSION and removed CHANGELOG.md",
		afterCommit: "reset 1 commit, restored VERSION and removed CHANGELOG.md",
		end:         "back where it started · nothing pushed",
		unrestored:  "manual cleanup required · VERSION + CHANGELOG.md written, not committed · nothing pushed",
	}
	artifactsCommitFails = treeFailure{
		name:   "pre_tag's changes staged, then their commit failing",
		config: preTagHookConfig,
		fails:  func(r restoreRepo) []gitStep { return r.artifacts(gitFailing(commitStderr)) },
		restores: func(r restoreRepo, outcome runner.ScriptedCall) []gitStep {
			return []gitStep{r.stagedRead(ScriptedOut(stagedArtifacts)), r.restore(outcome, "bundle.js")}
		},
		undone: "unstaged and removed pre_tag's changes",
		// The pre_tag hook ran, and could have moved HEAD itself, so the close never
		// claims the run is back where it started.
		end:        "nothing pushed",
		unrestored: "manual cleanup required · pre_tag's changes staged, not committed · nothing pushed",
	}
	treeFailures = []treeFailure{changelogWriteFails, stagedBookkeepingFails, bookkeepingStageFails, artifactsCommitFails}
)

// restoringRun is a first release from repo stopped by failure, the git steps from
// the starting commit's capture on being before, then the failure's, then the
// unwind's restore, git answering it with restoreAnswer, then its reset of the
// commits before made, then after. stash scripts an autostash's push.
type restoringRun struct {
	repo          restoreRepo
	failure       treeFailure
	config        string
	before        []gitStep
	after         []gitStep
	restoreAnswer *runner.ScriptedCall
	stash         []runner.ScriptedCall
	opts          func(*engine.ReleaseOptions)
}

// tail is every git step the run takes once it has captured its starting commit.
func (r restoringRun) tail() []gitStep {
	answer := ScriptedOut("")
	if r.restoreAnswer != nil {
		answer = *r.restoreAnswer
	}
	var reset []gitStep
	if len(r.before) > 0 {
		reset = r.repo.resetCommits()
	}
	return slices.Concat(r.before, r.failure.fails(r.repo), r.failure.restores(r.repo, answer), reset, r.after)
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
// commit are want, exactly and in order: the unwind touched no path but those its
// steps name.
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

// assertNoReset fails if the run reset anything: with no commit of mint's, the unwind
// moves no HEAD and rewrites no path beyond those it names.
func assertNoReset(t *testing.T, f *runner.FakeRunner) {
	t.Helper()
	if got := argsOf(f, "reset"); len(got) != 0 {
		t.Errorf("an unwind with no commit to reset ran git %q", got)
	}
}

// assertNoDiscard fails if the run issued a reset that discards changes of the user's
// rather than refusing over them: a --hard, or a --soft and a restore of the whole tree.
func assertNoDiscard(t *testing.T, f *runner.FakeRunner) {
	t.Helper()
	for _, mode := range []string{"--hard", "--soft", "--mixed", "--merge"} {
		if got := argsOf(f, "reset", mode); len(got) != 0 {
			t.Errorf("the unwind ran git %q, want only reset --keep", got)
		}
	}
}

// assertCreatedRemoved fails unless the file mint created, if any, is gone.
func assertCreatedRemoved(t *testing.T, root, created string) {
	t.Helper()
	if created == "" {
		return
	}
	if _, err := os.Stat(filepath.Join(root, created)); !os.IsNotExist(err) {
		t.Errorf("%s, which mint created, is still there (stat: %v)", created, err)
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

func TestRelease_AFailureBeforeAnyCommit_PutsBackExactlyWhatMintWroteOrStaged(t *testing.T) {
	t.Parallel()

	for _, tf := range treeFailures {
		t.Run(tf.name, func(t *testing.T) {
			t.Parallel()

			run := restoringRun{repo: restoreRepo{root: t.TempDir()}, failure: tf}
			rec, f, err := run.release(t)

			assertTreeRestored(t, rec, err, tf.undone, tf.end)
			assertGitTail(t, f, commandLinesOf(run.tail()))
			assertNoReset(t, f)
			assertCreatedRemoved(t, run.repo.root, tf.created)
		})
	}
}

func TestRelease_AFailureAfterAnArtifactsCommit_PutsBackTheWritesThenResetsTheCommit(t *testing.T) {
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
			assertCreatedRemoved(t, repo.root, tf.created)
		})
	}
}

// The reset after the put-back keeps the notes file's edits, and stages again what the
// index held staged of it: no step discards a change mint did not make.
func TestRelease_AFailureAfterAnArtifactsCommit_LeavesAnInRepoNotesFileAlone(t *testing.T) {
	t.Parallel()

	for _, tf := range []treeFailure{changelogWriteFails, stagedBookkeepingFails} {
		for _, nc := range notesResetCases {
			t.Run(tf.name+"/"+nc.name, func(t *testing.T) {
				t.Parallel()

				repo := restoreRepo{root: t.TempDir(), notes: nc.file, notesStaged: nc.notesStaged != ""}
				path := writeNotesFile(t, repo.root, nc.file, suppliedBody)
				run := restoringRun{repo: repo, failure: tf, config: preTagHookConfig, before: repo.artifacts(ScriptedOut(""))}
				rec, f, err := run.release(t)

				assertTreeRestored(t, rec, err, tf.afterCommit, tf.end)
				assertGitTail(t, f, commandLinesOf(run.tail()))
				assertNoDiscard(t, f)
				assertFileUnchanged(t, path, suppliedBody)
			})
		}
	}
}

func TestRelease_AFailureBeforeAnyCommit_LeavesAnInRepoNotesFileAlone(t *testing.T) {
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
				assertNoReset(t, f)
				assertNotesFileOnlyExcluded(t, f, nc.file)
				assertFileUnchanged(t, path, suppliedBody)
			})
		}
	}
}

func TestRelease_AFailureBeforeAnyCommit_Autostash_PopsOntoThePutBackTree(t *testing.T) {
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
			// The tail ends restore then pop: the WIP goes back onto the put-back tree.
			assertGitTail(t, f, commandLinesOf(run.tail()))
		})
	}
}

func TestRelease_AFailureBeforeAnyCommit_ARestoreThatFails_SaysSoWithTheManualStep(t *testing.T) {
	t.Parallel()

	for _, tf := range []treeFailure{stagedBookkeepingFails, artifactsCommitFails} {
		t.Run(tf.name, func(t *testing.T) {
			t.Parallel()

			repo := restoreRepo{root: t.TempDir()}
			failing := gitFailing(stageStderr)
			run := restoringRun{repo: repo, failure: tf, restoreAnswer: &failing}
			rec, f, err := run.release(t)

			assertAbortNonZero(t, err)
			restoring := tf.restores(repo, failing)
			paths := strings.TrimPrefix(restoring[len(restoring)-1].line, repo.git("restore --source=HEAD --staged --worktree -- "))
			names := strings.ReplaceAll(strings.ReplaceAll(paths, ":(top,literal)", ""), " ", " + ")
			var quoted []string
			for _, spec := range strings.Fields(paths) {
				quoted = append(quoted, cmdline.Quote(spec))
			}
			command := "git restore --source=HEAD --staged --worktree -- " + strings.Join(quoted, " ")
			warn := recoveryWarn(t, rec).Warn
			if want := "automatic recovery failed; manually restore " + names + " to HEAD (`" + command + "`)"; warn.Message != want || warn.Output != stageStderr || !warn.Failed {
				t.Errorf("recovery warning = %+v, want %q with git's output", warn, want)
			}
			if got := unwoundEvent(t, rec); got.Undone != "" || got.Repo != "manual cleanup required" {
				t.Errorf("Unwound = %+v, want nothing claimed undone and manual cleanup required", got)
			}
			if got := nextMoveCommands(rec); !slices.Equal(got, []string{command}) {
				t.Errorf("next moves' commands = %q, want the manual step %q", got, command)
			}
			assertClosesUnwound(t, rec, tf.unrestored)
			assertGitTail(t, f, commandLinesOf(run.tail()))
		})
	}
}

func TestRelease_PreTagsChangesStaged_AnUnreadableIndex_SaysSoAndTouchesNothing(t *testing.T) {
	t.Parallel()

	repo := restoreRepo{root: t.TempDir()}
	failure := artifactsCommitFails
	failure.restores = func(r restoreRepo, _ runner.ScriptedCall) []gitStep {
		return []gitStep{r.stagedRead(gitFailing(stageStderr))}
	}
	run := restoringRun{repo: repo, failure: failure}
	rec, f, err := run.release(t)

	assertAbortNonZero(t, err)
	warn := recoveryWarn(t, rec).Warn
	if want := "automatic recovery failed; manually unstage and remove the pre_tag changes mint staged"; warn.Message != want || warn.Output != stageStderr {
		t.Errorf("recovery warning = %+v, want %q with git's output", warn, want)
	}
	assertClosesUnwound(t, rec, failure.unrestored)
	assertGitTail(t, f, commandLinesOf(run.tail()))
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
			for _, args := range [][]string{{"reset"}, {"-C", root, "restore"}} {
				if got := argsOf(f, args...); len(got) != 0 {
					t.Errorf("an unwind with nothing to take back ran git %q", got)
				}
			}
		})
	}
}

func TestRelease_ARestoreWithNoCommit_PlainTranscripts(t *testing.T) {
	t.Parallel()

	const restoreBundleIncomplete = "unwind incomplete: WARN - automatic recovery failed; manually restore bundle.js to HEAD " +
		"(`git restore --source=HEAD --staged --worktree -- ':(top,literal)bundle.js'`)\n"
	restoreFails := gitFailing(stageStderr)
	tests := []struct {
		name    string
		failure treeFailure
		// restore is git's answer to the restore, success where nil.
		restore *runner.ScriptedCall
		want    string
	}{
		{
			name:    "pre_tag's changes, then a restore that fails",
			failure: artifactsCommitFails,
			restore: &restoreFails,
			want: "mint: releasing pigeon v0.0.1\n" +
				"version: v0.0.1 (-y)\n" +
				"preflight: tree clean, on main, v0.0.1 free, origin in sync, gh signed in\n" +
				"pre_tag: running...\n" +
				"pre_tag: FAILED - committing dirty tree \"chore(release): pre-tag artifacts for v0.0.1\": exit status 1\n" +
				"--- output ---\n" +
				commitStderr + "\n" +
				"--- end output ---\n" +
				restoreBundleIncomplete +
				"--- output ---\n" +
				stageStderr + "\n" +
				"--- end output ---\n" +
				"unwound: manual cleanup required\n" +
				"--- stderr ---\n" +
				"pre_tag: FAILED - committing dirty tree \"chore(release): pre-tag artifacts for v0.0.1\": exit status 1\n" +
				restoreBundleIncomplete,
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
			run := restoringRun{repo: restoreRepo{root: root}, failure: tt.failure, restoreAnswer: tt.restore}
			_, err := run.releaseTo(t, p)

			assertAbortNonZero(t, err)
			if got, want := read(), tt.want; got != want {
				t.Errorf("plain transcript =\n%s\nwant\n%s", got, want)
			}
		})
	}
}
