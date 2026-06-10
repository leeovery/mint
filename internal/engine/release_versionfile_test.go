package engine_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// seedVersionFileFold scripts the git timeline for a happy first-release run whose
// bookkeeping commit FOLDS a changelog change and a version-file change into ONE
// commit: the version-file projection is filesystem-only (no git), so the ONLY
// timeline difference from seedHappyGit is the single folded
// `git -C {root} add CHANGELOG.md {versionFile}` staging BOTH paths before the one
// bookkeeping commit. The caller seeds `gh`.
func seedVersionFileFold(f *runner.FakeRunner, root, releaseBranch, tag, versionFile string) {
	f.SeedSequence("git",
		ScriptedOut(root),                    // rev-parse --show-toplevel
		ScriptedOut("origin/"+releaseBranch), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),                      // tag --list (no tags)
		ScriptedOut(""),                      // fetch --tags
		ScriptedOut(""),                      // status --porcelain (clean)
		ScriptedOut(releaseBranch),           // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedNonZero(),                    // rev-parse -q --verify refs/tags/{tag} (absent)
		ScriptedOut("0\t1"),                  // rev-list left-right count (ahead only)
		ScriptedOut(""),                      // ls-remote --tags (tag free remote)
		ScriptedOut(startingSHA),             // rev-parse HEAD (capture the clean start)
		ScriptedOut(""),                      // -C root add CHANGELOG.md {versionFile} (folded)
		ScriptedOut(""),                      // -C root commit -m {commit_prefix} Release {tag}
		ScriptedOut(""),                      // tag -a {tag} -F -
		ScriptedOut(""),                      // push --atomic origin HEAD {tag}
	)
}

// readFile returns the exact bytes of {root}/{name} as a string, failing the test
// on a read error.
func readFile(t *testing.T, root, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(data)
}

// seedFile writes content to {root}/{name}, failing the test on error.
func seedFile(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
		t.Fatalf("seeding %s: %v", name, err)
	}
}

// countBookkeepingCommits counts how many `git -C {root} commit -m {subject}`
// invocations were recorded, so a test can prove EXACTLY one bookkeeping commit was
// made.
func countBookkeepingCommits(f *runner.FakeRunner, root, subject string) int {
	want := commandLine(runner.Invocation{Name: "git", Args: []string{"-C", root, "commit", "-m", subject}})
	n := 0
	for _, inv := range f.Invocations() {
		if commandLine(inv) == want {
			n++
		}
	}
	return n
}

// TestRelease_VersionFile_FoldsChangelogAndVersionIntoOneCommit proves a plain-mode
// version_file projection folds into the SAME bookkeeping commit as the changelog:
// exactly ONE `{commit_prefix} Release {tag}` commit whose single folded `add`
// stages BOTH CHANGELOG.md and the version file, the version file on disk holds the
// new bare version, and the version file is NEVER given a separate commit.
func TestRelease_VersionFile_FoldsChangelogAndVersionIntoOneCommit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release]\nversion_file = \"release.txt\"\n")
	// Pre-create the version file holding an OLDER version so the projection nets a
	// change (0.0.1 is the first release; seed something different).
	seedFile(t, root, "release.txt", "0.0.0\n")

	f := runner.NewFakeRunner()
	seedVersionFileFold(f, root, "main", "v0.0.1", "release.txt")
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The single folded staging carries BOTH the changelog and the version file.
	if !invokedWith(f, "git", "-C", root, "add", "CHANGELOG.md", "release.txt") {
		t.Errorf("no folded `git -C %s add CHANGELOG.md release.txt`; got %v", root, commandLines(f.Invocations()))
	}
	// The version file is never staged on its own (which would imply a separate commit).
	if invokedWith(f, "git", "-C", root, "add", "release.txt") {
		t.Errorf("version file was staged on its own; it must be folded with the changelog")
	}

	// Exactly ONE bookkeeping commit folds both — not two.
	subject := "🌿 Release v0.0.1"
	if got := countBookkeepingCommits(f, root, subject); got != 1 {
		t.Errorf("bookkeeping commits = %d, want exactly 1 (changelog + version folded)", got)
	}

	// The version file on disk now holds the new bare version (plain-mode canonical).
	if got := readFile(t, root, "release.txt"); got != "0.0.1\n" {
		t.Errorf("version file = %q, want the new bare version %q", got, "0.0.1\n")
	}
}

// TestRelease_VersionFile_VersionUnchangedChangelogChanges_OneCommit proves that
// when the version file ALREADY holds the target version (no net version change) but
// the changelog still changes, the bookkeeping commit is still made ONCE — staging
// only the changelog (the version file is a no-op and is not staged).
func TestRelease_VersionFile_VersionUnchangedChangelogChanges_OneCommit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release]\nversion_file = \"release.txt\"\n")
	// The version file is ALREADY at the target (0.0.1) → version projection no-ops.
	seedFile(t, root, "release.txt", "0.0.1\n")

	f := runner.NewFakeRunner()
	// Version no-op → only CHANGELOG.md is staged, so the timeline matches the plain
	// changelog-only happy path.
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// Only the changelog is staged — the version file is already at target (no-op).
	if !invokedWith(f, "git", "-C", root, "add", "CHANGELOG.md") {
		t.Errorf("no changelog-only staging; got %v", commandLines(f.Invocations()))
	}
	if invokedWith(f, "git", "-C", root, "add", "CHANGELOG.md", "release.txt") {
		t.Errorf("version file was staged despite being already at target (a no-op)")
	}

	// Still exactly ONE bookkeeping commit.
	subject := "🌿 Release v0.0.1"
	if got := countBookkeepingCommits(f, root, subject); got != 1 {
		t.Errorf("bookkeeping commits = %d, want exactly 1 (changelog only, version no-op)", got)
	}

	// The version file is untouched, still at the target version.
	if got := readFile(t, root, "release.txt"); got != "0.0.1\n" {
		t.Errorf("version file = %q, want it untouched at target %q", got, "0.0.1\n")
	}
}

// TestRelease_VersionFile_NothingNetChanged_NoCommit proves the combined no-op: when
// the changelog is disabled (no changelog change) AND the version file already holds
// the target version, NO bookkeeping commit is made (no empty commit), yet the run
// still proceeds to tag and push.
func TestRelease_VersionFile_NothingNetChanged_NoCommit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// changelog disabled (no changelog change) + version file already at target.
	writeConfig(t, root, "[release]\nchangelog = false\nversion_file = \"release.txt\"\n")
	seedFile(t, root, "release.txt", "0.0.1\n")

	f := runner.NewFakeRunner()
	// No staging and no bookkeeping commit: the spine jumps from the startingHEAD
	// capture straight to the tag + push.
	f.SeedSequence("git",
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),            // tag --list (no tags)
		ScriptedOut(""),            // fetch --tags
		ScriptedOut(""),            // status --porcelain (clean)
		ScriptedOut("main"),        // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedNonZero(),          // rev-parse -q --verify refs/tags/v0.0.1 (absent)
		ScriptedOut("0\t1"),        // rev-list left-right count (ahead only)
		ScriptedOut(""),            // ls-remote --tags (tag free remote)
		ScriptedOut(startingSHA),   // rev-parse HEAD (capture the clean start)
		ScriptedOut(""),            // tag -a v0.0.1 -F - (no commit precedes it)
		ScriptedOut(""),            // push --atomic origin HEAD v0.0.1
	)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// NO bookkeeping commit and NO staging — the combined no-op makes no empty commit.
	if invokedWith(f, "git", "-C", root, "commit", "-m", "🌿 Release v0.0.1") {
		t.Errorf("a bookkeeping commit ran despite the combined no-op; no empty commit allowed")
	}
	for _, inv := range f.Invocations() {
		if inv.Name == "git" && len(inv.Args) >= 3 && inv.Args[0] == "-C" && inv.Args[2] == "add" {
			t.Errorf("a staging `git add` ran despite the combined no-op: %v", inv.Args)
		}
	}

	// The run still proceeded to tag and push.
	if !invokedWith(f, "git", "tag", "-a", "v0.0.1", "-F", "-") {
		t.Errorf("combined no-op did not proceed to the tag; got %v", commandLines(f.Invocations()))
	}
	if !invokedWith(f, "git", "push", "--atomic", "origin", "HEAD", "v0.0.1") {
		t.Errorf("combined no-op did not proceed to the push; got %v", commandLines(f.Invocations()))
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("combined no-op run did not finish; last event = %v", fin.Kind)
	}
}

// TestRelease_VersionFile_EmbeddedMismatch_AbortsBeforeTag proves the embedded-mode
// pattern-mismatch abort (3-6) fires DURING the version-file projection, BEFORE the
// changelog write or any commit: the run surfaces a StageFailed naming "record",
// aborts with a non-zero *AbortError, makes NO tag/push, and — because the
// projection runs FIRST — the CHANGELOG file was never written.
func TestRelease_VersionFile_EmbeddedMismatch_AbortsBeforeTag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release]\nversion_file = \"version.go\"\nversion_pattern = \"RELEASE_VERSION=\\\"{version}\\\"\"\n")
	// A source file present but where the pattern matches nothing → fail-loud abort.
	seedFile(t, root, "version.go", "package main\n\nconst OTHER=\"0.0.0\"\n")

	f := runner.NewFakeRunner()
	// The spine reaches the startingHEAD capture, then the projection aborts before any
	// commit. The unwind re-probes HEAD (unchanged — nothing committed yet).
	seedHappyGitThroughGate(f, root, "main", "v0.0.1")
	f.SeedSequence("git", ScriptedOut(startingSHA)) // unwind: rev-parse HEAD (unchanged)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())

	assertAbortNonZero(t, err)
	if name := stageFailedName(t, rec); name != "record" {
		t.Errorf("StageFailed.Name = %q, want %q", name, "record")
	}
	// No tag/push/publish — the abort precedes all of it.
	assertNoMutation(t, f)
	// Project-FIRST ordering: the CHANGELOG was never written (the projection aborted
	// before the changelog write), so no partial dirty changelog is left behind.
	if _, statErr := os.Stat(filepath.Join(root, "CHANGELOG.md")); !os.IsNotExist(statErr) {
		t.Errorf("CHANGELOG.md exists; the version-file mismatch must abort BEFORE the changelog write (stat err = %v)", statErr)
	}
	// The source file was left untouched (no write on a fail-loud abort).
	if got := readFile(t, root, "version.go"); !strings.Contains(got, "0.0.0") {
		t.Errorf("version source = %q, want it untouched (no write on a fail-loud abort)", got)
	}
}
