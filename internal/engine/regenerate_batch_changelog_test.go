package engine_test

import (
	"errors"
	"slices"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// This file pins task 5-13: the batch `--all` WHOLE-FILE CHANGELOG rebuild + ONE end
// commit. After the per-version loop, a changelog/both `--all` run rebuilds CHANGELOG.md
// WHOLE — KaC preamble + every real version's section newest-on-top (regenerated bodies
// rendered under their HISTORICAL date; skipped-but-real versions' existing sections
// preserved verbatim; stray sections dropped) — and makes EXACTLY ONE commit + plain
// push at the END (subject `docs(changelog): regenerate release notes`). A release-only
// run makes no changelog commit; a byte-identical rebuild makes none.

const (
	batchRebuildSubject = "docs(changelog): regenerate release notes"
	batchV1Date         = "2024-01-01"
	batchV2Date         = "2024-02-02"
	batchV3Date         = "2024-03-03"
)

// freshChangelogBatchReq builds a fresh `--target changelog` `--all` request (the
// per-version body is the canned fresh body keyed off the tag), with -y so the loop
// runs unattended and the rebuild is the only thing under test.
func freshChangelogBatchReq(versions []version.Resolution, target engine.RegenerateTarget) engine.BatchRegenerateRequest {
	req := batchReq(engine.RegenerateSourceFresh, versions, true)
	req.Target = target
	return req
}

// committedSubject returns the -m subject of the single `git commit` the FakeRunner
// recorded, or "" if none committed. It scans for the commit verb and returns the arg
// after -m.
func committedSubject(f *runner.FakeRunner) string {
	for _, inv := range f.Invocations() {
		if inv.Name != "git" || !slices.Contains(inv.Args, "commit") {
			continue
		}
		for i, a := range inv.Args {
			if a == "-m" && i+1 < len(inv.Args) {
				return inv.Args[i+1]
			}
		}
	}
	return ""
}

// commitCount returns how many `git commit` invocations the FakeRunner recorded.
func commitCount(f *runner.FakeRunner) int {
	n := 0
	for _, inv := range f.Invocations() {
		if inv.Name == "git" && slices.Contains(inv.Args, "commit") {
			n++
		}
	}
	return n
}

// seedRebuildGit seeds the FakeRunner for a fresh changelog batch end-rebuild: the
// HEAD capture, one historical-date read per regenerated version (oldest → newest), and
// the add/commit/push. Per-version dispatch is skipped for a changelog-only target, so
// no per-version git fires in the loop.
func seedRebuildGit(f *runner.FakeRunner, dates ...string) {
	calls := []runner.ScriptedCall{ScriptedOut("startHEAD")} // rev-parse HEAD
	for _, d := range dates {
		calls = append(calls, ScriptedOut(d)) // for-each-ref creatordate:short per version
	}
	calls = append(calls,
		ScriptedOut(""), // add CHANGELOG.md
		ScriptedOut(""), // commit
		ScriptedOut(""), // push origin HEAD
	)
	f.SeedSequence("git", calls...)
}

// TestRegenerateAllValidated_Changelog_RebuildsWholeFileNewestOnTop proves a fresh
// `--target changelog` `--all` run rebuilds CHANGELOG.md WHOLE: preamble + every
// version's section newest-on-top, each regenerated body under its HISTORICAL date.
func TestRegenerateAllValidated_Changelog_RebuildsWholeFileNewestOnTop(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A stale file with the sections in the wrong order — the rebuild must repair it.
	seedChangelog(t, dir, kacPreamble+"\n## [1.0.0] - 2024-01-01\n\nstale v1\n")

	f := runner.NewFakeRunner()
	seedRebuildGit(f, batchV1Date, batchV2Date, batchV3Date)
	pub := newFakePublisher()
	rec := &presentertest.RecordingPresenter{}

	err := engine.RegenerateAllValidated(t.Context(), batchDeps(rec, f), pub, dir,
		freshChangelogBatchReq(threeVersions(), engine.RegenerateTargetChangelog), true)
	if err != nil {
		t.Fatalf("RegenerateAllValidated returned unexpected error: %v", err)
	}

	got := readChangelogFile(t, dir)
	want := kacPreamble + "\n" +
		"## [2.0.0] - 2024-03-03\n\n## fresh v2.0.0\n\n" +
		"## [1.1.0] - 2024-02-02\n\n## fresh v1.1.0\n\n" +
		"## [1.0.0] - 2024-01-01\n\n## fresh v1.0.0\n\n"
	if got != want {
		t.Errorf("rebuilt CHANGELOG.md =\n%q\nwant whole-file rebuild newest-on-top with historical dates\n%q", got, want)
	}
}

// TestRegenerateAllValidated_Changelog_DropsStraySection proves the whole-file rebuild
// drops a stray section (one matching NO real version) while repairing order — the
// rebuilt file contains exactly the real versions' sections, no stray.
func TestRegenerateAllValidated_Changelog_DropsStraySection(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A stray 9.9.9 section with no matching version, plus a mis-ordered real section.
	seedChangelog(t, dir, kacPreamble+"\n"+
		"## [9.9.9] - 2024-09-09\n\nstray drift\n\n"+
		"## [1.0.0] - 2024-01-01\n\nstale v1\n")

	f := runner.NewFakeRunner()
	seedRebuildGit(f, batchV1Date, batchV2Date, batchV3Date)
	pub := newFakePublisher()
	rec := &presentertest.RecordingPresenter{}

	if err := engine.RegenerateAllValidated(t.Context(), batchDeps(rec, f), pub, dir,
		freshChangelogBatchReq(threeVersions(), engine.RegenerateTargetChangelog), true); err != nil {
		t.Fatalf("RegenerateAllValidated returned unexpected error: %v", err)
	}

	got := readChangelogFile(t, dir)
	want := kacPreamble + "\n" +
		"## [2.0.0] - 2024-03-03\n\n## fresh v2.0.0\n\n" +
		"## [1.1.0] - 2024-02-02\n\n## fresh v1.1.0\n\n" +
		"## [1.0.0] - 2024-01-01\n\n## fresh v1.0.0\n\n"
	if got != want {
		t.Errorf("rebuilt CHANGELOG.md =\n%q\nwant the stray 9.9.9 section dropped\n%q", got, want)
	}
}

// TestRegenerateAllValidated_Changelog_PreservesSkippedSectionVerbatim proves a SKIPPED
// real version's existing section is PRESERVED verbatim in the rebuild (the user-resolved
// no-data-loss rule): v2 fails (diff too large) and is skipped, but its existing section
// survives untouched while v1 and v3 are regenerated.
func TestRegenerateAllValidated_Changelog_PreservesSkippedSectionVerbatim(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n"+
		"## [2.0.0] - 2024-03-03\n\nstale v3\n\n"+
		"## [1.1.0] - 2024-02-02\n\nPRESERVE THIS skipped v2 body\n\n"+
		"## [1.0.0] - 2024-01-01\n\nstale v1\n")

	f := runner.NewFakeRunner()
	// v2 (batchV2Tag) is skipped → no historical-date read for it; only v1 and v3 are
	// regenerated, so two date reads (oldest → newest of the regenerated set).
	seedRebuildGit(f, batchV1Date, batchV3Date)
	pub := newFakePublisher()
	rec := &presentertest.RecordingPresenter{}

	req := freshChangelogBatchReq(threeVersions(), engine.RegenerateTargetChangelog)
	req.ProduceBody = freshBodyOrDiffTooLarge(batchV2Tag) // v2 fails → skipped

	if err := engine.RegenerateAllValidated(t.Context(), batchDeps(rec, f), pub, dir, req, true); err != nil {
		t.Fatalf("RegenerateAllValidated returned unexpected error: %v", err)
	}

	got := readChangelogFile(t, dir)
	want := kacPreamble + "\n" +
		"## [2.0.0] - 2024-03-03\n\n## fresh v2.0.0\n\n" +
		"## [1.1.0] - 2024-02-02\n\nPRESERVE THIS skipped v2 body\n\n" +
		"## [1.0.0] - 2024-01-01\n\n## fresh v1.0.0\n\n"
	if got != want {
		t.Errorf("rebuilt CHANGELOG.md =\n%q\nwant the skipped v2 section preserved verbatim\n%q", got, want)
	}
}

// TestRegenerateAllValidated_Changelog_OneCommitAtEnd proves the batch makes EXACTLY
// ONE changelog commit, at the END (subject `docs(changelog): regenerate release
// notes`) — not one per version.
func TestRegenerateAllValidated_Changelog_OneCommitAtEnd(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## [1.0.0] - 2024-01-01\n\nstale v1\n")

	f := runner.NewFakeRunner()
	seedRebuildGit(f, batchV1Date, batchV2Date, batchV3Date)
	pub := newFakePublisher()
	rec := &presentertest.RecordingPresenter{}

	if err := engine.RegenerateAllValidated(t.Context(), batchDeps(rec, f), pub, dir,
		freshChangelogBatchReq(threeVersions(), engine.RegenerateTargetChangelog), true); err != nil {
		t.Fatalf("RegenerateAllValidated returned unexpected error: %v", err)
	}

	if got := commitCount(f); got != 1 {
		t.Errorf("recorded %d changelog commits, want exactly 1 (one at the end, not one per version)", got)
	}
	if got := committedSubject(f); got != batchRebuildSubject {
		t.Errorf("commit subject = %q, want the --all form %q", got, batchRebuildSubject)
	}
	if !invokedWith(f, "git", "push", "origin", "HEAD") {
		t.Errorf("no plain `git push origin HEAD` at the end of the batch; got %v", commandLines(f.Invocations()))
	}
}

// TestRegenerateAllValidated_Changelog_PrePushFailure_ResetsCommit proves the batch
// end-of-batch push routes through the SAME shared recovery as the single-version
// path: a pre-push failure resets the rebuilt CHANGELOG commit to the captured
// starting HEAD, surfaces a "push" StageFailed, and emits NO "push" StageSucceeded.
func TestRegenerateAllValidated_Changelog_PrePushFailure_ResetsCommit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## [1.0.0] - 2024-01-01\n\nstale v1\n")

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut("startHEAD"), // rev-parse HEAD (captured start)
		ScriptedOut(batchV1Date), // for-each-ref creatordate:short v1
		ScriptedOut(batchV2Date), // for-each-ref creatordate:short v2
		ScriptedOut(batchV3Date), // for-each-ref creatordate:short v3
		ScriptedOut(""),          // add CHANGELOG.md
		ScriptedOut(""),          // commit (the rebuild commit lands)
		runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("remote rejected")}, // push fails
		ScriptedOut(""), // reset --hard startHEAD (recovery)
	)
	pub := newFakePublisher()
	rec := &presentertest.RecordingPresenter{}

	err := engine.RegenerateAllValidated(t.Context(), batchDeps(rec, f), pub, dir,
		freshChangelogBatchReq(threeVersions(), engine.RegenerateTargetChangelog), true)

	assertAbortNonZero(t, err)
	if !invokedWith(f, "git", "reset", "--hard", "startHEAD") {
		t.Errorf("batch pre-push failure did not reset the rebuild commit to the captured start; got %v", commandLines(f.Invocations()))
	}
	if stageFailedName(t, rec) != "push" {
		t.Errorf("batch pre-push failure surfaced StageFailed %q, want \"push\"", stageFailedName(t, rec))
	}
	if _, ok := stageSucceeded(rec, "push"); ok {
		t.Errorf("a batch pre-push FAILURE emitted a push StageSucceeded; success narration is success-only; kinds = %v", rec.Kinds())
	}
}

// TestRegenerateAllValidated_Release_NoChangelogCommit proves an `--all --target
// release` run makes NO changelog commit and NO push (it touches only provider
// releases).
func TestRegenerateAllValidated_Release_NoChangelogCommit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## [1.0.0] - 2024-01-01\n\nv1\n")

	f := runner.NewFakeRunner()
	seedReuseGit(f) // for-each-ref reuse body reads only
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	rec := &presentertest.RecordingPresenter{}

	req := batchReq(engine.RegenerateSourceReuse, threeVersions(), true)
	req.Target = engine.RegenerateTargetRelease

	if err := engine.RegenerateAllValidated(t.Context(), batchDeps(rec, f), pub, dir, req, true); err != nil {
		t.Fatalf("RegenerateAllValidated returned unexpected error: %v", err)
	}

	for _, inv := range f.Invocations() {
		if inv.Name == "git" && (slices.Contains(inv.Args, "commit") || slices.Contains(inv.Args, "push") || slices.Contains(inv.Args, "add")) {
			t.Errorf("--target release issued a changelog mutation %v; release-only must touch no changelog", inv.Args)
		}
	}
	// The provider releases still went out (release-only writes the provider).
	if len(pub.dispatched) != 3 {
		t.Errorf("dispatched %d, want 3 (release-only writes provider per version)", len(pub.dispatched))
	}
}

// TestRegenerateAllValidated_Changelog_ByteIdenticalNoCommit proves a rebuild that
// yields byte-identical content makes NO commit and NO push.
func TestRegenerateAllValidated_Changelog_ByteIdenticalNoCommit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// The existing file is exactly what the rebuild will emit, so the rebuild is a no-op.
	identical := kacPreamble + "\n" +
		"## [2.0.0] - 2024-03-03\n\n## fresh v2.0.0\n\n" +
		"## [1.1.0] - 2024-02-02\n\n## fresh v1.1.0\n\n" +
		"## [1.0.0] - 2024-01-01\n\n## fresh v1.0.0\n\n"
	seedChangelog(t, dir, identical)

	f := runner.NewFakeRunner()
	// Only the HEAD capture + the three historical-date reads fire; no add/commit/push.
	f.SeedSequence("git",
		ScriptedOut("startHEAD"),
		ScriptedOut(batchV1Date),
		ScriptedOut(batchV2Date),
		ScriptedOut(batchV3Date),
	)
	pub := newFakePublisher()
	rec := &presentertest.RecordingPresenter{}

	if err := engine.RegenerateAllValidated(t.Context(), batchDeps(rec, f), pub, dir,
		freshChangelogBatchReq(threeVersions(), engine.RegenerateTargetChangelog), true); err != nil {
		t.Fatalf("RegenerateAllValidated returned unexpected error: %v", err)
	}

	if commitCount(f) != 0 {
		t.Errorf("a byte-identical rebuild made %d commits, want 0", commitCount(f))
	}
	for _, inv := range f.Invocations() {
		if inv.Name == "git" && (slices.Contains(inv.Args, "push") || slices.Contains(inv.Args, "add")) {
			t.Errorf("a byte-identical rebuild issued %v; it must make no commit/push", inv.Args)
		}
	}
	if got := readChangelogFile(t, dir); got != identical {
		t.Errorf("a byte-identical rebuild rewrote the file:\n%q", got)
	}
}
