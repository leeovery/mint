package engine_test

import (
	"errors"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// This file pins task 5-4: the regenerate preflight SUBSET selector. Regenerate's
// preflight is the forward gate SET run as a subset driven off the resolved
// request, per the spec's general rule (calls gh → gh-auth; commits + pushes →
// clean-tree / branch / remote-sync; cuts a new tag → tag-free). Regenerate NEVER
// cuts a tag, so tag-free NEVER runs, and there is no version compute in any mode.
//
// The four concrete cases (driven off the resolved target):
//   - --reuse / fresh --target release (provider write only) → gh-auth ONLY.
//   - fresh --target changelog (commits + pushes, no provider) → clean-tree +
//     on-branch + remote-sync; NO gh-auth.
//   - fresh --target both → gh-auth + clean-tree + on-branch + remote-sync.
//
// Assertions are by the git/gh argv the FakeRunner records: gh-auth issues
// `gh auth status`, clean-tree `git status --porcelain`, on-branch `git rev-parse
// --abbrev-ref HEAD`, remote-sync `git fetch --tags` then `git rev-list
// --left-right --count @{u}...HEAD`, and the tag-free local probe `git rev-parse
// -q --verify refs/tags/...` (which must NEVER appear).

const regenReleaseBranch = "main"

// ghAuthRan reports whether the gh-auth gate ran (`gh auth status`).
func ghAuthRan(f *runner.FakeRunner) bool {
	return invokedWith(f, "gh", "auth", "status")
}

// cleanTreeRan reports whether the clean-tree gate ran (`git status --porcelain`).
func cleanTreeRan(f *runner.FakeRunner) bool {
	return invokedWith(f, "git", "status", "--porcelain")
}

// onBranchRan reports whether the on-branch gate ran (`git rev-parse --abbrev-ref HEAD`).
func onBranchRan(f *runner.FakeRunner) bool {
	return invokedWith(f, "git", "rev-parse", "--abbrev-ref", "HEAD")
}

// remoteSyncRan reports whether the remote-sync gate ran (`git rev-list
// --left-right --count @{u}...HEAD`).
func remoteSyncRan(f *runner.FakeRunner) bool {
	return invokedWith(f, "git", "rev-list", "--left-right", "--count", "@{u}...HEAD")
}

// tagFreeLocalRan reports whether the tag-free LOCAL gate ran (`git rev-parse -q
// --verify refs/tags/...`). For regenerate this must NEVER be true.
func tagFreeLocalRan(f *runner.FakeRunner) bool {
	for _, inv := range f.Invocations() {
		if inv.Name != "git" || len(inv.Args) < 4 {
			continue
		}
		if inv.Args[0] == "rev-parse" && inv.Args[1] == "-q" && inv.Args[2] == "--verify" {
			return true
		}
	}
	return false
}

// tagFreeRemoteRan reports whether the tag-free REMOTE gate ran (`git ls-remote
// --tags ...`). For regenerate this must NEVER be true.
func tagFreeRemoteRan(f *runner.FakeRunner) bool {
	for _, inv := range f.Invocations() {
		if inv.Name == "git" && len(inv.Args) >= 2 && inv.Args[0] == "ls-remote" {
			return true
		}
	}
	return false
}

// releaseGateSet is the resolved selection for a --reuse / --target release run:
// a provider write only, no git mutation.
func releaseGateSet() engine.RegenerateGateSet {
	return engine.RegenerateGateSet{CallsProvider: true}
}

// changelogGateSet is the resolved selection for a fresh --target changelog run:
// commits + pushes, no provider write.
func changelogGateSet() engine.RegenerateGateSet {
	return engine.RegenerateGateSet{CommitsAndPushes: true}
}

// bothGateSet is the resolved selection for a fresh --target both run: a provider
// write AND commits + pushes.
func bothGateSet() engine.RegenerateGateSet {
	return engine.RegenerateGateSet{CallsProvider: true, CommitsAndPushes: true}
}

// TestRegeneratePreflight_Reuse_GhAuthOnly proves a --reuse / --target release run
// runs gh-auth ONLY — no clean-tree, on-branch, remote-sync, or tag-free.
func TestRegeneratePreflight_Reuse_GhAuthOnly(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("gh", runner.Result{}, nil) // gh auth status (authenticated)
	rec := &presentertest.RecordingPresenter{}

	err := engine.RegeneratePreflight(t.Context(), newDeps(rec, f), regenReleaseBranch, releaseGateSet())
	if err != nil {
		t.Fatalf("RegeneratePreflight returned %v, want nil", err)
	}

	if !ghAuthRan(f) {
		t.Errorf("--reuse did not run gh-auth; it MUST (a dead gh auth is the usual heal reason)")
	}
	if cleanTreeRan(f) {
		t.Errorf("--reuse ran the clean-tree gate; release-only has no git mutation")
	}
	if onBranchRan(f) {
		t.Errorf("--reuse ran the on-branch gate; release-only has no git mutation")
	}
	if remoteSyncRan(f) {
		t.Errorf("--reuse ran the remote-sync gate; release-only has no git mutation")
	}
	assertNoTagFreeGate(t, f)
}

// TestRegeneratePreflight_FreshChangelog_CommitPushGates proves a fresh --target
// changelog run runs clean-tree + on-branch + remote-sync (commits + pushes) and
// NOT gh-auth (no provider write).
func TestRegeneratePreflight_FreshChangelog_CommitPushGates(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(""),                 // status --porcelain (clean)
		ScriptedOut(regenReleaseBranch), // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedOut(""),                 // fetch --tags
		ScriptedOut("0\t1"),             // rev-list left-right count (ahead only)
	)
	rec := &presentertest.RecordingPresenter{}

	err := engine.RegeneratePreflight(t.Context(), newDeps(rec, f), regenReleaseBranch, changelogGateSet())
	if err != nil {
		t.Fatalf("RegeneratePreflight returned %v, want nil", err)
	}

	if !cleanTreeRan(f) {
		t.Errorf("fresh --target changelog did not run the clean-tree gate")
	}
	if !onBranchRan(f) {
		t.Errorf("fresh --target changelog did not run the on-branch gate")
	}
	if !remoteSyncRan(f) {
		t.Errorf("fresh --target changelog did not run the remote-sync gate")
	}
	if ghAuthRan(f) {
		t.Errorf("fresh --target changelog ran gh-auth; no provider write occurs")
	}
	assertNoTagFreeGate(t, f)
}

// TestRegeneratePreflight_FreshBoth_AllApplicableGates proves a fresh --target
// both run runs gh-auth + clean-tree + on-branch + remote-sync (provider write AND
// commits + pushes), still never tag-free.
func TestRegeneratePreflight_FreshBoth_AllApplicableGates(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(""),                 // status --porcelain (clean)
		ScriptedOut(regenReleaseBranch), // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedOut(""),                 // fetch --tags
		ScriptedOut("0\t1"),             // rev-list left-right count (ahead only)
	)
	f.Seed("gh", runner.Result{}, nil) // gh auth status (authenticated)
	rec := &presentertest.RecordingPresenter{}

	err := engine.RegeneratePreflight(t.Context(), newDeps(rec, f), regenReleaseBranch, bothGateSet())
	if err != nil {
		t.Fatalf("RegeneratePreflight returned %v, want nil", err)
	}

	if !ghAuthRan(f) {
		t.Errorf("fresh --target both did not run gh-auth")
	}
	if !cleanTreeRan(f) {
		t.Errorf("fresh --target both did not run the clean-tree gate")
	}
	if !onBranchRan(f) {
		t.Errorf("fresh --target both did not run the on-branch gate")
	}
	if !remoteSyncRan(f) {
		t.Errorf("fresh --target both did not run the remote-sync gate")
	}
	assertNoTagFreeGate(t, f)
}

// TestRegeneratePreflight_FreshRelease_GhAuthOnly proves a fresh --target release
// run (provider write only, no changelog commit) runs gh-auth ONLY — the same
// subset as --reuse.
func TestRegeneratePreflight_FreshRelease_GhAuthOnly(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("gh", runner.Result{}, nil) // gh auth status (authenticated)
	rec := &presentertest.RecordingPresenter{}

	err := engine.RegeneratePreflight(t.Context(), newDeps(rec, f), regenReleaseBranch, releaseGateSet())
	if err != nil {
		t.Fatalf("RegeneratePreflight returned %v, want nil", err)
	}

	if !ghAuthRan(f) {
		t.Errorf("fresh --target release did not run gh-auth")
	}
	if cleanTreeRan(f) || onBranchRan(f) || remoteSyncRan(f) {
		t.Errorf("fresh --target release ran a git-mutation gate; provider-only writes nothing to git")
	}
	assertNoTagFreeGate(t, f)
}

// TestRegeneratePreflight_NeverTagFree proves the tag-free gate NEVER runs in ANY
// regenerate mode: the target tag already exists, so regenerate never cuts a new
// tag.
func TestRegeneratePreflight_NeverTagFree(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		set  engine.RegenerateGateSet
		seed func(*runner.FakeRunner)
	}{
		{
			name: "reuse / release",
			set:  releaseGateSet(),
			seed: func(f *runner.FakeRunner) { f.Seed("gh", runner.Result{}, nil) },
		},
		{
			name: "fresh changelog",
			set:  changelogGateSet(),
			seed: func(f *runner.FakeRunner) {
				f.SeedSequence("git",
					ScriptedOut(""),
					ScriptedOut(regenReleaseBranch),
					ScriptedOut(""),
					ScriptedOut("0\t1"),
				)
			},
		},
		{
			name: "fresh both",
			set:  bothGateSet(),
			seed: func(f *runner.FakeRunner) {
				f.SeedSequence("git",
					ScriptedOut(""),
					ScriptedOut(regenReleaseBranch),
					ScriptedOut(""),
					ScriptedOut("0\t1"),
				)
				f.Seed("gh", runner.Result{}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := runner.NewFakeRunner()
			tt.seed(f)
			rec := &presentertest.RecordingPresenter{}

			if err := engine.RegeneratePreflight(t.Context(), newDeps(rec, f), regenReleaseBranch, tt.set); err != nil {
				t.Fatalf("RegeneratePreflight returned %v, want nil", err)
			}
			assertNoTagFreeGate(t, f)
		})
	}
}

// TestRegeneratePreflight_NoVersionCompute proves no version-computation git probe
// runs in any regenerate mode — regenerate resolves an EXISTING tag, there is no
// bump. The forward path computes the current version via `git tag --list`; that
// probe must NOT appear here.
func TestRegeneratePreflight_NoVersionCompute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		set  engine.RegenerateGateSet
		seed func(*runner.FakeRunner)
	}{
		{
			name: "reuse / release",
			set:  releaseGateSet(),
			seed: func(f *runner.FakeRunner) { f.Seed("gh", runner.Result{}, nil) },
		},
		{
			name: "fresh both",
			set:  bothGateSet(),
			seed: func(f *runner.FakeRunner) {
				f.SeedSequence("git",
					ScriptedOut(""),
					ScriptedOut(regenReleaseBranch),
					ScriptedOut(""),
					ScriptedOut("0\t1"),
				)
				f.Seed("gh", runner.Result{}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := runner.NewFakeRunner()
			tt.seed(f)
			rec := &presentertest.RecordingPresenter{}

			if err := engine.RegeneratePreflight(t.Context(), newDeps(rec, f), regenReleaseBranch, tt.set); err != nil {
				t.Fatalf("RegeneratePreflight returned %v, want nil", err)
			}
			if invokedWith(f, "git", "tag", "--list") {
				t.Errorf("a version-compute probe (`git tag --list`) ran; regenerate resolves an existing tag, no bump")
			}
		})
	}
}

// TestRegeneratePreflight_GhAuthFails_OnReuse_Aborts proves a failing APPLICABLE
// gate aborts cleanly before any work: gh-auth not authenticated on a --reuse run
// surfaces a StageFailed and aborts non-zero.
func TestRegeneratePreflight_GhAuthFails_OnReuse_Aborts(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	// gh ran and reported NOT authenticated (a populated Result with a non-nil error).
	f.Seed("gh", runner.Result{ExitCode: 1}, errors.New("exit status 1"))
	rec := &presentertest.RecordingPresenter{}

	err := engine.RegeneratePreflight(t.Context(), newDeps(rec, f), regenReleaseBranch, releaseGateSet())
	assertAbortNonZero(t, err)

	if !recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("gh-auth failure on --reuse did not surface a StageFailed")
	}
}

// TestRegeneratePreflight_CleanTreeFails_OnFreshChangelog_Aborts proves a failing
// APPLICABLE gate aborts cleanly before any work: a dirty tree on a fresh --target
// changelog run surfaces a StageFailed and aborts non-zero — before any commit or
// push.
func TestRegeneratePreflight_CleanTreeFails_OnFreshChangelog_Aborts(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(" M CHANGELOG.md"), // status --porcelain (DIRTY — clean-tree gate fails)
	)
	rec := &presentertest.RecordingPresenter{}

	err := engine.RegeneratePreflight(t.Context(), newDeps(rec, f), regenReleaseBranch, changelogGateSet())
	assertAbortNonZero(t, err)

	if !recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("dirty tree on fresh --target changelog did not surface a StageFailed")
	}
	// The gate aborts BEFORE the network gates run.
	if remoteSyncRan(f) {
		t.Errorf("clean-tree failure did not short-circuit; the remote-sync gate still ran")
	}
}

// assertNoTagFreeGate fails the test if either tag-free gate ran — regenerate
// never cuts a tag, so neither the local nor the remote tag-free probe is allowed.
func assertNoTagFreeGate(t *testing.T, f *runner.FakeRunner) {
	t.Helper()
	if tagFreeLocalRan(f) {
		t.Errorf("the tag-free LOCAL gate ran; regenerate never cuts a tag")
	}
	if tagFreeRemoteRan(f) {
		t.Errorf("the tag-free REMOTE gate ran; regenerate never cuts a tag")
	}
}
