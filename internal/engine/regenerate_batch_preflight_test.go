package engine_test

import (
	"errors"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/publish"
	"mint/internal/runner"
)

// This file pins the batch `--all` preflight gate (task 10-1, review remediation): the
// batch path must preflight the RESOLVED target subset the SAME way the single-version
// interactive path does. A bare `mint release regenerate --all` resolves its target
// INTERACTIVELY (at the cmd-layer resolveBatchAxes) AFTER the cmd layer can preflight, so
// the engine entry point RegenerateAllValidated runs the resolved gate set itself —
// otherwise an interactively-chosen changelog/both commits+pushes with no clean-tree /
// branch / remote-sync gate, and an interactively-chosen release/both dispatches provider
// writes with no gh-auth gate (the bypass the single-version fix closed, mirrored here).
//
// The bar matches the single-version tests: assert gate-BEFORE-mutation ORDERING, not
// mere presence.

// TestRegenerateAllValidated_InteractiveChangelog_RunsCommitPushGatesBeforeRebuild proves
// a bare `--all` run whose TARGET resolved to changelog runs the commits+pushes preflight
// bucket — clean-tree, on-branch, remote-sync — BEFORE the end-of-batch CHANGELOG rebuild
// commit/push. The gate set is derived from the resolved batch target, not the empty
// pre-resolution target.
func TestRegenerateAllValidated_InteractiveChangelog_RunsCommitPushGatesBeforeRebuild(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## [1.0.0] - 2024-01-01\n\nstale v1\n")

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		// Preflight commits+pushes bucket (resolved from the interactive changelog target).
		ScriptedOut(""),                 // status --porcelain (clean)
		ScriptedOut(regenReleaseBranch), // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedOut(""),                 // fetch --tags
		ScriptedOut("0\t1"),             // rev-list left-right count (ahead only)
		// End-of-batch rebuild.
		ScriptedOut("startHEAD"), // rev-parse HEAD (capture clean start)
		ScriptedOut(batchV1Date), // for-each-ref creatordate:short v1
		ScriptedOut(batchV2Date), // for-each-ref creatordate:short v2
		ScriptedOut(batchV3Date), // for-each-ref creatordate:short v3
		ScriptedOut(""),          // add CHANGELOG.md
		ScriptedOut(""),          // commit
		ScriptedOut(""),          // push origin HEAD
	)
	pub := newFakePublisher()
	rec := &presentertest.RecordingPresenter{}

	req := freshChangelogBatchReq(threeVersions(), engine.RegenerateTargetChangelog)
	req.ReleaseBranch = regenReleaseBranch
	if err := engine.RegenerateAllValidated(t.Context(), batchDeps(rec, f), pub, dir, req, true); err != nil {
		t.Fatalf("RegenerateAllValidated returned unexpected error: %v", err)
	}

	if !cleanTreeRan(f) {
		t.Errorf("interactive changelog --all did not run the clean-tree gate")
	}
	if !onBranchRan(f) {
		t.Errorf("interactive changelog --all did not run the on-branch gate")
	}
	if !remoteSyncRan(f) {
		t.Errorf("interactive changelog --all did not run the remote-sync gate")
	}
	// The gates must precede the rebuild commit/push (all gates pass here and the rebuild
	// proceeds; a failing gate would short-circuit before any mutation).
	assertCommitPushGatesBeforeCommit(t, f)
}

// TestRegenerateAllValidated_InteractiveRelease_RunsGhAuthBeforeFirstDispatch proves a
// bare `--all` run whose TARGET resolved to release runs the gh-auth gate BEFORE the FIRST
// per-version provider dispatch. The gate set is derived from the resolved batch target.
func TestRegenerateAllValidated_InteractiveRelease_RunsGhAuthBeforeFirstDispatch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	f := runner.NewFakeRunner()
	seedReuseGit(f)                    // for-each-ref reuse body reads per version
	f.Seed("gh", runner.Result{}, nil) // gh auth status (authenticated)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	// Snapshot whether gh-auth had run at the moment the FIRST provider write dispatches.
	ghAuthBeforeFirstDispatch := false
	dispatches := 0
	pub.beforeDispatch = func() {
		if dispatches == 0 {
			ghAuthBeforeFirstDispatch = ghAuthRan(f)
		}
		dispatches++
	}
	rec := &presentertest.RecordingPresenter{}

	req := batchReq(engine.RegenerateSourceReuse, threeVersions(), true)
	req.Target = engine.RegenerateTargetRelease
	req.ReleaseBranch = regenReleaseBranch
	if err := engine.RegenerateAllValidated(t.Context(), batchDeps(rec, f), pub, dir, req, true); err != nil {
		t.Fatalf("RegenerateAllValidated returned unexpected error: %v", err)
	}

	if !ghAuthRan(f) {
		t.Errorf("interactive release --all did not run the gh-auth gate")
	}
	if len(pub.dispatched) != 3 {
		t.Fatalf("provider dispatched %d times, want exactly 3 (one per version)", len(pub.dispatched))
	}
	if !ghAuthBeforeFirstDispatch {
		t.Errorf("the first provider write dispatched BEFORE gh-auth ran; the gate must precede the write")
	}
}

// TestRegenerateAllValidated_DowngradedReuse_SkipsGhAuthGate proves a downgraded
// `regenerate --reuse --all` / `--target release` batch (the provider could not be
// resolved on a non-github / no-remote origin, so a NIL publisher is threaded for the
// WHOLE batch) does NOT run the gh-auth preflight gate: the gate is selected from the
// resolved publisher, not the bare provider-writing target. A FAILING gh-auth recorder
// is seeded — if the gate were still selected it would abort the whole batch — so the
// batch completing proves the gate was skipped, mirroring the forward path's
// `if publisher != nil` guard. Each version's provider write is nil-guarded downstream
// (warn + skip), so the whole batch is a clean downgrade.
func TestRegenerateAllValidated_DowngradedReuse_SkipsGhAuthGate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	f := runner.NewFakeRunner()
	seedReuseGit(f) // for-each-ref reuse body reads per version
	// gh ran would report NOT authenticated — but on a downgrade the gate must NEVER be
	// reached, so this failing seed must not abort the batch.
	f.Seed("gh", runner.Result{ExitCode: 1}, errors.New("exit status 1"))
	// A genuinely-nil publish.Publisher interface — the downgrade value the cmd layer
	// threads (NOT a typed-nil concrete pointer).
	var pub publish.Publisher
	rec := &presentertest.RecordingPresenter{}

	req := batchReq(engine.RegenerateSourceReuse, threeVersions(), true)
	req.Target = engine.RegenerateTargetRelease
	req.ReleaseBranch = regenReleaseBranch
	if err := engine.RegenerateAllValidated(t.Context(), batchDeps(rec, f), pub, dir, req, true); err != nil {
		t.Fatalf("downgraded --reuse --all batch aborted: %v; the gh-auth gate must be skipped on a nil publisher", err)
	}

	if ghAuthRan(f) {
		t.Errorf("a downgraded batch ran the gh-auth gate; a nil publisher must skip it (CallsProvider == false)")
	}
}

// TestRegenerateAllValidated_ResolvedRelease_RunsGhAuthGate proves a NON-downgraded
// `regenerate --reuse --all` / `--target release` batch (a resolved, non-nil
// publisher) STILL runs the gh-auth preflight gate exactly as before — the
// publisher-presence guard only suppresses the gate on a downgrade.
func TestRegenerateAllValidated_ResolvedRelease_RunsGhAuthGate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	f := runner.NewFakeRunner()
	seedReuseGit(f)                    // for-each-ref reuse body reads per version
	f.Seed("gh", runner.Result{}, nil) // gh auth status (authenticated)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	rec := &presentertest.RecordingPresenter{}

	req := batchReq(engine.RegenerateSourceReuse, threeVersions(), true)
	req.Target = engine.RegenerateTargetRelease
	req.ReleaseBranch = regenReleaseBranch
	if err := engine.RegenerateAllValidated(t.Context(), batchDeps(rec, f), pub, dir, req, true); err != nil {
		t.Fatalf("RegenerateAllValidated returned unexpected error: %v", err)
	}

	if !ghAuthRan(f) {
		t.Errorf("a resolved-publisher release batch did not run the gh-auth gate; it must (CallsProvider == true)")
	}
}

// TestRegenerateAllValidated_ChangelogOnly_GateSelectionUnaffectedByPublisher proves
// a changelog-only (`--target changelog`) batch's gate selection is UNCHANGED by the
// publisher-presence guard: with a NIL publisher it STILL runs the full commit/push
// bucket (clean-tree + on-branch + remote-sync) and NEVER gh-auth (a changelog target
// writes no provider, so CallsProvider is false regardless of publisher presence). The
// CommitsAndPushes bucket is selected SOLELY from target.writesChangelog().
func TestRegenerateAllValidated_ChangelogOnly_GateSelectionUnaffectedByPublisher(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## [1.0.0] - 2024-01-01\n\nstale v1\n")

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		// Preflight commits+pushes bucket (resolved from the changelog target).
		ScriptedOut(""),                 // status --porcelain (clean)
		ScriptedOut(regenReleaseBranch), // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedOut(""),                 // fetch --tags
		ScriptedOut("0\t1"),             // rev-list left-right count (ahead only)
		// End-of-batch rebuild.
		ScriptedOut("startHEAD"), // rev-parse HEAD (capture clean start)
		ScriptedOut(batchV1Date), // for-each-ref creatordate:short v1
		ScriptedOut(batchV2Date), // for-each-ref creatordate:short v2
		ScriptedOut(batchV3Date), // for-each-ref creatordate:short v3
		ScriptedOut(""),          // add CHANGELOG.md
		ScriptedOut(""),          // commit
		ScriptedOut(""),          // push origin HEAD
	)
	// A genuinely-nil publish.Publisher interface: a changelog-only run writes no
	// provider, so the nil publisher is never dereferenced and the selection is the
	// same as with a resolved publisher.
	var pub publish.Publisher
	rec := &presentertest.RecordingPresenter{}

	req := freshChangelogBatchReq(threeVersions(), engine.RegenerateTargetChangelog)
	req.ReleaseBranch = regenReleaseBranch
	if err := engine.RegenerateAllValidated(t.Context(), batchDeps(rec, f), pub, dir, req, true); err != nil {
		t.Fatalf("RegenerateAllValidated returned unexpected error: %v", err)
	}

	if !cleanTreeRan(f) || !onBranchRan(f) || !remoteSyncRan(f) {
		t.Errorf("changelog-only --all did not run the full commit/push bucket with a nil publisher; the bucket is selected solely from the target")
	}
	if ghAuthRan(f) {
		t.Errorf("changelog-only --all ran gh-auth; a changelog target writes no provider (CallsProvider == false regardless of publisher)")
	}
}

// TestRegenerateAllValidated_FailingGate_AbortsBeforeAnyWork proves a failing APPLICABLE
// gate (a dirty tree on an interactive changelog --all) aborts non-zero BEFORE any version
// is processed, any provider write, or any changelog commit/push — the gate set
// short-circuits before mutation, identical to the single-version path.
func TestRegenerateAllValidated_FailingGate_AbortsBeforeAnyWork(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## [1.0.0] - 2024-01-01\n\nstale v1\n")

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(" M CHANGELOG.md"), // status --porcelain (DIRTY → clean-tree fails)
	)
	pub := newFakePublisher()
	rec := &presentertest.RecordingPresenter{}

	req := freshChangelogBatchReq(threeVersions(), engine.RegenerateTargetChangelog)
	req.ReleaseBranch = regenReleaseBranch
	err := engine.RegenerateAllValidated(t.Context(), batchDeps(rec, f), pub, dir, req, true)

	assertAbortNonZero(t, err)
	if remoteSyncRan(f) {
		t.Errorf("clean-tree failure did not short-circuit; the remote-sync gate still ran")
	}
	if invokedWith(f, "git", "push", "origin", "HEAD") {
		t.Errorf("a failing gate pushed; the gate must abort before any mutation")
	}
	if len(pub.dispatched) != 0 {
		t.Errorf("a failing gate dispatched a provider write %+v; it must abort before any work", pub.dispatched)
	}
	// No version narration block should have opened (the batch never started its loop).
	if len(runStartedVersions(rec)) != 0 {
		t.Errorf("a failing gate opened %d version blocks; it must abort before the loop", len(runStartedVersions(rec)))
	}
}
