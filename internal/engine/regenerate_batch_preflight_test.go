package engine_test

import (
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
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
