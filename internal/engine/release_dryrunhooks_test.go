package engine_test

import (
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// dryRunPatchOptions is the default-bump options with the fixed clock and DryRun
// active. These 3-11 tests focus on the HOOK skip-and-report dimension; since 4-7a
// the same DryRun flag also skips every mutation (commit/tag/push/provider release),
// so no mutating command is issued here either — but these tests assert only the
// hook behaviour. The dedicated no-mutation/plan assertions live in
// release_dryrun_test.go.
func dryRunPatchOptions() engine.ReleaseOptions {
	return engine.ReleaseOptions{Bump: version.BumpPatch, Now: fixedClock, DryRun: true}
}

// warnWithMessage returns the first recorded Warn event whose Message equals want,
// failing the test if none matched — the dry-run hook-skip notice rides the Warn
// seam, so each skipped point is asserted by its exact message.
func warnWithMessage(t *testing.T, rec *presentertest.RecordingPresenter, want string) presentertest.Event {
	t.Helper()
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindWarn && ev.Warn.Message == want {
			return ev
		}
	}
	t.Fatalf("no Warn event with Message %q; kinds = %v", want, rec.Kinds())
	return presentertest.Event{}
}

// TestRelease_DryRun_SkipsPreflightHookAndReports proves that under --dry-run a
// CONFIGURED preflight hook is NOT invoked (no `sh -c "check.sh"` reaches the
// runner) and the skip is REPORTED via a Warn (label "dry-run", message "skipping
// preflight hook"). The run otherwise proceeds; since 4-7a no mutations occur, but
// this test asserts only the hook-skip behaviour.
func TestRelease_DryRun_SkipsPreflightHookAndReports(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npreflight = \"check.sh\"\n")

	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("sh", runner.Result{}, nil) // seeded but must never run under dry-run
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), dryRunPatchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if invokedWith(f, "sh", "-c", "check.sh") {
		t.Errorf("preflight hook ran under --dry-run; it must be skipped")
	}
	warn := warnWithMessage(t, rec, "skipping preflight hook")
	if warn.Warn.Label != "dry-run" {
		t.Errorf("Warn.Label = %q, want %q", warn.Warn.Label, "dry-run")
	}
}

// TestRelease_DryRun_SkipsPreTagHookAndReports proves that under --dry-run a
// CONFIGURED pre_tag hook is NOT invoked, the skip is reported via a Warn ("skipping
// pre_tag hook"), and NO artifact-commit machinery runs: no post-hook `git status
// --porcelain` probe and no `chore(release): pre-tag artifacts for {tag}` commit,
// because the hook never dirtied the tree.
func TestRelease_DryRun_SkipsPreTagHookAndReports(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")

	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("sh", runner.Result{}, nil) // seeded but must never run under dry-run
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), dryRunPatchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if invokedWith(f, "sh", "-c", "build.sh") {
		t.Errorf("pre_tag hook ran under --dry-run; it must be skipped")
	}
	warn := warnWithMessage(t, rec, "skipping pre_tag hook")
	if warn.Warn.Label != "dry-run" {
		t.Errorf("Warn.Label = %q, want %q", warn.Warn.Label, "dry-run")
	}
	// No artifact-commit machinery: the skipped hook dirtied nothing, so the
	// post-hook porcelain probe is never issued and no artifact commit is made.
	if invokedWith(f, "git", "-C", root, "status", "--porcelain") {
		t.Errorf("a skipped pre_tag hook still ran the post-hook status probe under --dry-run")
	}
	assertNoArtifactCommit(t, f, root)
}

// TestRelease_DryRun_NoPreTagArtifactCommit proves explicitly that under --dry-run
// no `chore(release): pre-tag artifacts for {tag}` commit (and no `git -C {root} add
// -A` staging) is produced — the skipped hook left the tree as mint found it, so the
// artifact commit must not exist.
func TestRelease_DryRun_NoPreTagArtifactCommit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")

	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("sh", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), dryRunPatchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertNoArtifactCommit(t, f, root)
}

// TestRelease_DryRun_SkipsPostReleaseHookAndReports proves that under --dry-run a
// CONFIGURED post_release hook is NOT invoked and the skip is reported via a Warn
// ("skipping post_release hook", label "dry-run").
func TestRelease_DryRun_SkipsPostReleaseHookAndReports(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npost_release = \"notify.sh\"\n")

	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	f.Seed("sh", runner.Result{}, nil) // seeded but must never run under dry-run
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), dryRunPatchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if invokedWith(f, "sh", "-c", "notify.sh") {
		t.Errorf("post_release hook ran under --dry-run; it must be skipped")
	}
	warn := warnWithMessage(t, rec, "skipping post_release hook")
	if warn.Warn.Label != "dry-run" {
		t.Errorf("Warn.Label = %q, want %q", warn.Warn.Label, "dry-run")
	}
}

// TestRelease_DryRun_AllThreeHooksConfigured_NoneRun proves the combined assertion:
// with all three hook points configured, NO hook command reaches the CommandRunner
// under --dry-run (zero `sh` invocations), and each point reports its skip via a
// Warn.
func TestRelease_DryRun_AllThreeHooksConfigured_NoneRun(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npreflight = \"check.sh\"\npre_tag = \"build.sh\"\npost_release = \"notify.sh\"\n")

	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("sh", runner.Result{}, nil) // seeded but must never run under dry-run
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), dryRunPatchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if shInvoked(f) {
		t.Errorf("a hook command ran under --dry-run; no `sh -c …` must reach the runner: %v", commandLines(f.Invocations()))
	}
	warnWithMessage(t, rec, "skipping preflight hook")
	warnWithMessage(t, rec, "skipping pre_tag hook")
	warnWithMessage(t, rec, "skipping post_release hook")
}

// TestRelease_DryRun_AbsentHooks_NoSkipReport proves an ABSENT hook under --dry-run
// produces NO skip report: with no [release.hooks] configured at all, no `sh` runs
// and no "skipping … hook" Warn is emitted for any unconfigured point — the skip
// notice is only for configured-and-skipped hooks.
func TestRelease_DryRun_AbsentHooks_NoSkipReport(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// No .mint.toml at all — no [release.hooks].
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), dryRunPatchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if shInvoked(f) {
		t.Errorf("an absent hook ran `sh` under --dry-run; got %v", commandLines(f.Invocations()))
	}
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindWarn && ev.Warn.Label == "dry-run" {
			t.Errorf("an absent hook emitted a dry-run skip Warn: %q", ev.Warn.Message)
		}
	}
}
