package engine_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// diskIgnoresCase reports whether dir's filesystem takes two spellings of a name in
// different case as one file.
func diskIgnoresCase(t *testing.T, dir string) bool {
	t.Helper()
	probe := filepath.Join(dir, "case-probe")
	if err := os.WriteFile(probe, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(probe) }()
	_, err := os.Stat(filepath.Join(dir, "CASE-PROBE"))
	return err == nil
}

// On a disk that ignores case, a version_file spelled in another case than git tracks
// it is staged, checked and committed under git's spelling, which a literal pathspec
// then matches.
func TestRelease_AVersionFileInAnotherCaseThanGitTracksIt_IsCommittedUnderGitsSpelling(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if !diskIgnoresCase(t, root) {
		t.Skip("the disk keeps the spellings apart, so version.txt is a file of its own")
	}
	writeConfig(t, root, "[release]\nversion_file = \"version.txt\"\n")
	writeRepoFile(t, root, "VERSION.txt", "0.0.0\n")
	f := runner.NewFakeRunner()
	seedHappyGitThroughGate(f, root, "main", "v0.0.1")
	f.SeedSequence("git",
		ScriptedOut("VERSION.txt\x00"), // how git spells version.txt
		unchangedFiles,                 // the read of whether the user changed the files mint writes
		ScriptedOut(""),                // add
		headBeforeCommit,               // HEAD before the commit
		ScriptedOut(""),                // commit
		headAfterCommit,                // HEAD after it
		committedPaths,                 // the paths the commit changed
		ScriptedOut(""),                // tag -a
		ScriptedOut(""),                // push
	)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), engine.ReleaseOptions{Now: fixedClock}); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	lines := stepLines(f)
	for _, want := range []string{
		spellingRead(root, "version.txt"),
		userChangesRead(root, "CHANGELOG.md", "VERSION.txt"),
		"git -C " + root + " add CHANGELOG.md VERSION.txt",
		"git -C " + root + " commit -m 🌿 Release v0.0.1 --pathspec-from-file=- --pathspec-file-nul" + piped("CHANGELOG.md", "VERSION.txt"),
	} {
		if !slices.Contains(lines, want) {
			t.Errorf("git steps = %q, want %q among them", lines, want)
		}
	}
	if got := readFile(t, root, "VERSION.txt"); got != "0.0.1\n" {
		t.Errorf("VERSION.txt = %q, want the release's version", got)
	}
}

// Where git tracks the file under two spellings the disk takes as one, the run stops
// on the record node before writing anything.
func TestRelease_AVersionFileGitTracksUnderTwoSpellings_StopsBeforeAnyWrite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if !diskIgnoresCase(t, root) {
		t.Skip("the disk keeps the spellings apart")
	}
	writeConfig(t, root, "[release]\nversion_file = \"version\"\n")
	writeRepoFile(t, root, "VERSION", "0.0.0\n")
	f := runner.NewFakeRunner()
	seedHappyGitThroughGate(f, root, "main", "v0.0.1")
	f.SeedSequence("git", ScriptedOut("VERSION\x00Version\x00"))
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), engine.ReleaseOptions{Now: fixedClock})

	assertAbortNonZero(t, err)
	assertNothingStagedOrCommitted(t, f)
	const message = "version_file version is the file git tracks as VERSION and Version; set it to one of them"
	if failure := onlyStageFailureEvent(t, rec); failure.Key != "record" || failure.Message != message {
		t.Errorf("StageFailed = %+v, want the record node failing with %q", failure, message)
	}
	if got := readFile(t, root, "VERSION"); got != "0.0.0\n" {
		t.Errorf("VERSION = %q, want it untouched", got)
	}
	if _, err := os.Stat(filepath.Join(root, "CHANGELOG.md")); !os.IsNotExist(err) {
		t.Errorf("CHANGELOG.md stat = %v, want it never written", err)
	}
}
