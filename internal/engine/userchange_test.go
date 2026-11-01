package engine_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// userChanged is git's answer to the read of whether the files mint is about to write
// hold a change it did not make: entries, each an XY status then its path.
func userChanged(entries ...string) runner.ScriptedCall {
	var out string
	for i := 0; i+1 < len(entries); i += 2 {
		out += entries[i] + " " + entries[i+1] + "\x00"
	}
	return ScriptedOut(out)
}

// assertNothingStagedOrCommitted fails if the run staged or committed anything after
// the read that stopped it.
func assertNothingStagedOrCommitted(t *testing.T, f *runner.FakeRunner) {
	t.Helper()
	for _, inv := range invocationsNamed(f, "git") {
		args := inv.Args
		if len(args) > 2 && args[0] == "-C" {
			args = args[2:]
		}
		if len(args) > 0 && (args[0] == "add" || args[0] == "commit" || slices.Equal(args[:min(2, len(args))], []string{"tag", "-a"})) {
			t.Errorf("a refused write ran git %q", inv.Args)
		}
	}
}

// A change the user made to a file the record stage writes, while the run waited, stops
// it on the record node before either write: the version file and the changelog are
// left as the user has them, nothing is staged or committed, and the message names the
// file and says to commit or stash the change.
func TestRelease_AChangeOfTheUsersToAFileTheRecordWrites_StopsItBeforeAnyWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		answer  runner.ScriptedCall
		message string
	}{
		{name: "an unstaged edit to the changelog", answer: userChanged(" M", "CHANGELOG.md"),
			message: "CHANGELOG.md changed while mint ran; commit or stash the change and run again"},
		{name: "a staged edit to the version file", answer: userChanged("M ", "VERSION"),
			message: "VERSION changed while mint ran; commit or stash the change and run again"},
		{name: "a changelog created and the version file edited", answer: userChanged("??", "CHANGELOG.md", " M", "VERSION"),
			message: "CHANGELOG.md and VERSION changed while mint ran; commit or stash the changes and run again"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeConfig(t, root, plainVersionFile)
			writeRepoFile(t, root, "VERSION", "mine\n")
			writeRepoFile(t, root, "CHANGELOG.md", "# mine\n")
			f := runner.NewFakeRunner()
			seedHappyGitThroughGate(f, root, "main", "v0.0.1")
			f.SeedSequence("git", tt.answer)
			rec := &presentertest.RecordingPresenter{}

			err := engine.Release(t.Context(), newDeps(rec, f), engine.ReleaseOptions{Now: fixedClock})

			assertAbortNonZero(t, err)
			if got := stepLines(f); got[len(got)-1] != userChangesRead(root, "CHANGELOG.md", "VERSION") {
				t.Errorf("git steps = %q, want them to end with the read of the files record writes", got)
			}
			assertNothingStagedOrCommitted(t, f)
			failure := onlyStageFailureEvent(t, rec)
			if failure.Key != "record" || failure.Message != tt.message {
				t.Errorf("StageFailed = %+v, want the record node failing with %q", failure, tt.message)
			}
			if got := readFile(t, root, "VERSION"); got != "mine\n" {
				t.Errorf("VERSION = %q, want the user's", got)
			}
			if got := readFile(t, root, "CHANGELOG.md"); got != "# mine\n" {
				t.Errorf("CHANGELOG.md = %q, want the user's", got)
			}
		})
	}
}

// With the changelog off and no version file, the record stage writes nothing, so it
// reads nothing.
func TestRelease_ARecordThatWritesNothing_ReadsNoFileForChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release]\nchangelog = false\n")
	f := runner.NewFakeRunner()
	seedHappyGitThroughGate(f, root, "main", "v0.0.1")
	f.SeedSequence("git", ScriptedOut(""), ScriptedOut("")) // tag -a, push
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), engine.ReleaseOptions{Now: fixedClock}); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}
	for _, inv := range invocationsNamed(f, "git") {
		if slices.Contains(inv.Args, "--untracked-files=all") {
			t.Errorf("a record writing nothing read %q", inv.Args)
		}
	}
}

// A change of the user's to .mint.toml stops the answers' write on the notes node, the
// file left as the user has it, nothing staged or committed.
func TestRelease_AChangeOfTheUsersToMintToml_StopsTheAnswersBeforeTheirWrite(t *testing.T) {
	t.Parallel()

	const config = "[release]\n# mine\n"
	root := t.TempDir()
	writeConfig(t, root, config)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git", ScriptedOut(exportGoDiff+designDocDiff), userChanged(" M", ".mint.toml"))
	f.SeedSequence("claude", ScriptedOut(designDocAnswer), ScriptedOut(aiBody))
	f.Seed("gh", runner.Result{}, nil)
	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil)

	err := engine.Release(t.Context(), attendedDeps(rec, f), priorTagNormalAIOptions())

	assertAbortNonZero(t, err)
	if got := stepLines(f); got[len(got)-1] != userChangesRead(root, ".mint.toml") {
		t.Errorf("git steps = %q, want them to end with the read of .mint.toml", got)
	}
	assertNothingStagedOrCommitted(t, f)
	failure := notesFailure(t, rec)
	const message = ".mint.toml changed while mint ran; commit or stash the change and run again"
	if failure.Message != "recording the exclusion answers" || failure.Output != message {
		t.Errorf("notes StageFailed = %+v, want the answers' write failing with %q beneath", failure, message)
	}
	if got := readFile(t, root, ".mint.toml"); got != config {
		t.Errorf(".mint.toml = %q, want the user's %q", got, config)
	}
}

// A change of the user's to CHANGELOG.md stops regenerate's rewrite on the CHANGELOG.md
// node, the file left as the user has it.
func TestRegenerateRun_AChangeOfTheUsersToTheChangelog_StopsItBeforeTheRewrite(t *testing.T) {
	t.Parallel()

	dir := staleChangelogDir(t, "Stale body.\n")
	before := readFile(t, dir, "CHANGELOG.md")
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(""),                    // status --porcelain (clean)
		ScriptedOut(regenRunReleaseBranch), // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedOut(""),                    // fetch --tags
		ScriptedOut("0\t1"),                // rev-list left-right count (ahead only)
		ScriptedOut("startHEAD"),           // rev-parse HEAD (capture clean start)
		ScriptedOut("2024-02-15"),          // for-each-ref creatordate:short (historical date)
		userChanged(" M", "CHANGELOG.md"),  // the read of whether the user changed CHANGELOG.md
	)
	rec := &presentertest.RecordingPresenter{}
	req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetChangelog))
	req.ReleaseBranch = regenRunReleaseBranch

	err := engine.RegenerateRun(t.Context(), railDeps(rec, f, dir), nil, dir, req)

	assertAbortNonZero(t, err)
	if got := stepLines(f); got[len(got)-1] != userChangesRead(dir, "CHANGELOG.md") {
		t.Errorf("git steps = %q, want them to end with the read of CHANGELOG.md", got)
	}
	assertNothingStagedOrCommitted(t, f)
	failure := onlyStageFailureEvent(t, rec)
	if want := "CHANGELOG.md changed while mint ran; commit or stash the change and run again"; failure.Key != "record" || failure.Message != want {
		t.Errorf("StageFailed = %+v, want the CHANGELOG.md node failing with %q", failure, want)
	}
	if got := readFile(t, dir, "CHANGELOG.md"); got != before {
		t.Errorf("CHANGELOG.md = %q, want it untouched", got)
	}
}

// regenerate --all's rebuild is stopped the same way.
func TestRegenerateAllValidated_AChangeOfTheUsersToTheChangelog_StopsItBeforeTheRebuild(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	const stale = kacPreamble + "\n## [1.0.0] - 2024-01-01\n\nstale v1\n"
	seedChangelog(t, dir, stale)
	f := runner.NewFakeRunner()
	f.SeedSequence("git", append(preflightCommitPushBucket(),
		ScriptedOut("startHEAD"),          // rev-parse HEAD (captured start)
		ScriptedOut(batchV1Date),          // for-each-ref creatordate:short v1
		ScriptedOut(batchV2Date),          // for-each-ref creatordate:short v2
		ScriptedOut(batchV3Date),          // for-each-ref creatordate:short v3
		userChanged("M ", "CHANGELOG.md"), // the read of whether the user changed CHANGELOG.md
	)...)
	rec := &presentertest.RecordingPresenter{}

	err := engine.RegenerateAllValidated(t.Context(), batchDeps(rec, f), newFakePublisher(), dir,
		freshChangelogBatchReq(threeVersions(), engine.RegenerateTargetChangelog), true)

	assertAbortNonZero(t, err)
	assertNothingStagedOrCommitted(t, f)
	if got := stageFailedName(t, rec); got != "record" {
		t.Errorf("StageFailed = %q, want record", got)
	}
	if got := readFile(t, dir, "CHANGELOG.md"); got != stale {
		t.Errorf("CHANGELOG.md = %q, want it untouched", got)
	}
}

// A read that fails stops the run too, git's words beneath, as mint can't tell whether
// the write would take a change of the user's.
func TestRelease_TheReadForAChangeOfTheUsersFailing_StopsItBeforeAnyWrite(t *testing.T) {
	t.Parallel()

	const stderr = "fatal: unable to read index\n"
	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGitThroughGate(f, root, "main", "v0.0.1")
	f.SeedSequence("git", gitFailing(stderr))
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), engine.ReleaseOptions{Now: fixedClock})

	assertAbortNonZero(t, err)
	assertNothingStagedOrCommitted(t, f)
	failure := onlyStageFailureEvent(t, rec)
	if failure.Key != "record" || failure.Output != stderr {
		t.Errorf("StageFailed = %+v, want the record node failing with git's words beneath", failure)
	}
	if _, err := os.Stat(filepath.Join(root, "CHANGELOG.md")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("CHANGELOG.md stat = %v, want it never written", err)
	}
}
