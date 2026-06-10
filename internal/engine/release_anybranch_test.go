package engine_test

import (
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// This file pins task 4-5: the --any-branch escape hatch. When set, mint SKIPS the
// on-release-branch gate entirely (it is not evaluated — no `git rev-parse
// --abbrev-ref HEAD` is issued) so a deliberate off-branch release proceeds; every
// OTHER gate (clean tree, tag-free, remote sync, gh auth) still runs unchanged.
// Without the flag the branch gate runs exactly as before and aborts off-branch.
// The bypass is observable via the Presenter (a Warn). The flag composes with
// --autostash and the rest without interaction.

// anyBranchOptions is patchOptions with --any-branch set.
func anyBranchOptions() engine.ReleaseOptions {
	return engine.ReleaseOptions{Bump: version.BumpPatch, Now: fixedClock, AnyBranch: true}
}

// seedAnyBranchHappyGit scripts the full happy first-release git timeline with the
// on-branch gate SKIPPED — there is NO `git rev-parse --abbrev-ref HEAD` call,
// because --any-branch bypasses it without evaluating it. Every other gate still
// runs. The caller seeds the trailing gh calls.
func seedAnyBranchHappyGit(f *runner.FakeRunner, root, releaseBranch, tag string) {
	f.SeedSequence("git",
		ScriptedOut(root),                    // rev-parse --show-toplevel
		ScriptedOut("origin/"+releaseBranch), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),                      // tag --list (no tags)
		ScriptedOut(""),                      // fetch --tags
		ScriptedOut(""),                      // status --porcelain (clean)
		// NO rev-parse --abbrev-ref HEAD — the on-branch gate is skipped.
		ScriptedNonZero(),        // rev-parse -q --verify refs/tags/{tag} (absent)
		ScriptedOut("0\t1"),      // rev-list left-right count (ahead only)
		ScriptedOut(""),          // ls-remote --tags (tag free remote)
		ScriptedOut(startingSHA), // rev-parse HEAD (capture the clean start)
		ScriptedOut(""),          // -C root add CHANGELOG.md
		ScriptedOut(""),          // -C root commit -m
		ScriptedOut(""),          // tag -a {tag} -F -
		ScriptedOut(""),          // push --atomic origin HEAD {tag}
	)
}

// TestRelease_AnyBranch_OffBranch_SkipsGateAndProceeds proves that with --any-branch
// the on-release-branch gate is BYPASSED (not evaluated — no abbrev-ref probe) and an
// off-branch release runs through to a successful GitHub release. The bypass is
// reported via the Presenter.
func TestRelease_AnyBranch_OffBranch_SkipsGateAndProceeds(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedAnyBranchHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil) // gh auth status, then gh release create
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), anyBranchOptions())
	if err != nil {
		t.Fatalf("Release returned %v, want nil (off-branch passes under --any-branch)", err)
	}

	// The on-branch gate was NOT evaluated — no abbrev-ref probe was issued.
	if invokedWith(f, "git", "rev-parse", "--abbrev-ref", "HEAD") {
		t.Errorf("--any-branch evaluated the on-branch gate (`git rev-parse --abbrev-ref HEAD`); it must be skipped, not run")
	}
	// The run finished successfully.
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("run did not finish under --any-branch; last event = %v", fin.Kind)
	}
	// The bypass is observable via the Presenter.
	if !anyBranchBypassWarnRecorded(rec) {
		t.Errorf("--any-branch did not report the branch-gate bypass via the Presenter; warns = %v", warnMessages(rec))
	}
}

// TestRelease_NoAnyBranch_OffBranch_StillAborts proves the Phase 1 default is
// preserved: WITHOUT --any-branch an off-branch HEAD aborts at the on-branch gate.
func TestRelease_NoAnyBranch_OffBranch_StillAborts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),            // tag --list (no tags)
		ScriptedOut(""),            // fetch --tags
		ScriptedOut(""),            // status --porcelain (clean)
		ScriptedOut("feature/x"),   // rev-parse --abbrev-ref HEAD (OFF branch — gate fails)
	)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	assertAbortNonZero(t, err)

	if !recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("off-branch without --any-branch did not surface a StageFailed")
	}
	// The branch gate WAS evaluated (the default behaviour) and no mutation crossed.
	if !invokedWith(f, "git", "rev-parse", "--abbrev-ref", "HEAD") {
		t.Errorf("the on-branch gate was not evaluated without --any-branch; it must run by default")
	}
	assertNoMutation(t, f)
}

// TestRelease_AnyBranch_OnBranch_NoEffect proves --any-branch is inert when already on
// the release branch: the release succeeds exactly as it would without the flag. The
// gate is skipped (the abbrev-ref probe is still absent), but the run reaches the same
// successful end state.
func TestRelease_AnyBranch_OnBranch_NoEffect(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	// HEAD happens to be on main, but --any-branch still skips the gate (it is never
	// evaluated regardless of the current branch). The spine reaches a release.
	seedAnyBranchHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), anyBranchOptions())
	if err != nil {
		t.Fatalf("Release returned %v, want nil (on-branch + --any-branch is a no-op on the outcome)", err)
	}

	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("run did not finish on-branch under --any-branch; last event = %v", fin.Kind)
	}
}

// TestRelease_AnyBranch_DoesNotWeakenCleanTreeGate proves --any-branch does NOT relax
// the clean-tree gate: a dirty tree still aborts under --any-branch.
func TestRelease_AnyBranch_DoesNotWeakenCleanTreeGate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),            // tag --list (no tags)
		ScriptedOut(""),            // fetch --tags
		ScriptedOut(dirtyStatus),   // status --porcelain (DIRTY — clean-tree gate fails)
	)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), anyBranchOptions())
	assertAbortNonZero(t, err)

	if !recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("dirty tree under --any-branch did not surface a StageFailed; clean-tree gate must still run")
	}
	assertNoMutation(t, f)
}

// TestRelease_AnyBranch_DoesNotWeakenTagFreeGate proves --any-branch does NOT relax the
// tag-free-local gate: an already-existing local tag still aborts under --any-branch.
func TestRelease_AnyBranch_DoesNotWeakenTagFreeGate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),            // tag --list (no tags)
		ScriptedOut(""),            // fetch --tags
		ScriptedOut(""),            // status --porcelain (clean)
		// on-branch gate skipped (--any-branch)
		ScriptedOut("abc1234"), // rev-parse -q --verify refs/tags/v0.0.1 (EXISTS — gate fails)
	)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), anyBranchOptions())
	assertAbortNonZero(t, err)

	if !recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("existing local tag under --any-branch did not surface a StageFailed; tag-free gate must still run")
	}
	assertNoMutation(t, f)
}

// TestRelease_AnyBranch_DoesNotWeakenRemoteSyncGate proves --any-branch does NOT relax
// the remote-sync gate: being behind the upstream still aborts under --any-branch.
func TestRelease_AnyBranch_DoesNotWeakenRemoteSyncGate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),            // tag --list (no tags)
		ScriptedOut(""),            // fetch --tags
		ScriptedOut(""),            // status --porcelain (clean)
		// on-branch gate skipped (--any-branch)
		ScriptedNonZero(),   // rev-parse -q --verify refs/tags/v0.0.1 (absent)
		ScriptedOut("2\t0"), // rev-list left-right count (2 BEHIND — gate fails)
	)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), anyBranchOptions())
	assertAbortNonZero(t, err)

	if !recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("behind-upstream under --any-branch did not surface a StageFailed; remote-sync gate must still run")
	}
	assertNoMutation(t, f)
}

// anyBranchBypassWarnRecorded reports whether any recorded Warn announces the
// release-branch gate bypass — the --any-branch observable signal.
func anyBranchBypassWarnRecorded(rec *presentertest.RecordingPresenter) bool {
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindWarn && strings.Contains(ev.Warn.Message, "release-branch gate") {
			return true
		}
	}
	return false
}
