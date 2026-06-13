package engine_test

// This file holds the Phase 4 DRY-RUN CORE tests (task 4-7a): under --dry-run the
// spine runs the read-only stages NORMALLY (preflight gates, version determination,
// notes generation/preview) and then SKIPS every mutation — no bookkeeping commit,
// no annotated tag, no atomic push, no provider release — while printing the full
// plan (the would-do commits + their subjects, the tag, and the publish target).
//
// The mutation-free guarantee is enforced by SEEDING NO mutation outcomes on the
// FakeRunner: the read path and the read-only provider detection are scripted, but
// the commit/tag/push git calls and every gh call are deliberately LEFT UNSEEDED,
// so any attempt to issue one fails the run (FakeRunner errors on an unseeded
// command). A clean nil return therefore proves no mutation was ever reached.

import (
	"os"
	"path/filepath"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// seedDryRunFirstRelease scripts a FakeRunner's git timeline for a no-tags
// first-release dry run: the ten read-only gates (identical to the real path) plus
// the read-only `git remote get-url origin` provider detection the dry-run plan uses
// to name the publish target. CRUCIALLY it seeds NO mutation tail (no add/commit, no
// tag, no push) — those calls are left unseeded so any attempt fails the run.
func seedDryRunFirstRelease(f *runner.FakeRunner, root, releaseBranch, tag string) {
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
		ScriptedOut(githubRemoteURL),         // remote get-url origin (provider detection for the plan)
	)
}

// dryRunOptions is the default-bump, fixed-clock options with DryRun active — the
// real --dry-run run this task wires (read-only run, all mutations skipped).
func dryRunOptions() engine.ReleaseOptions {
	return engine.ReleaseOptions{Bump: version.BumpPatch, Now: fixedClock, DryRun: true}
}

// TestRelease_DryRun_NoMutation_FirstRelease proves the load-bearing guarantee: a
// --dry-run first release makes NO commit, NO tag, NO push, and NO provider release.
// Every mutating command is left UNSEEDED on the FakeRunner, so the clean nil return
// proves no mutation was ever issued (an attempt would have errored). The run still
// finishes successfully.
func TestRelease_DryRun_NoMutation_FirstRelease(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedDryRunFirstRelease(f, root, "main", "v0.0.1")
	// No mutation seeds: the bookkeeping add/commit, the tag, the push, and every gh
	// call are deliberately unseeded so any attempt fails the run.
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), dryRunOptions()); err != nil {
		t.Fatalf("dry-run Release returned unexpected error: %v", err)
	}

	assertNoMutation(t, f)
	// No bookkeeping commit (no `git -C root commit` and no staging add).
	if invokedWith(f, "git", "-C", root, "commit", "-m", "🌿 Release v0.0.1") {
		t.Errorf("dry-run made the bookkeeping commit")
	}
	if invokedWith(f, "git", "-C", root, "add", "CHANGELOG.md") {
		t.Errorf("dry-run staged the changelog")
	}
	// Not a single gh command may have run (no auth gate, no release create).
	for _, inv := range f.Invocations() {
		if inv.Name == "gh" {
			t.Errorf("dry-run issued a gh command: %q", commandLine(inv))
		}
	}
	// The run still finishes successfully.
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("dry-run did not finish; last event = %v", fin.Kind)
	}
}

// TestRelease_DryRun_RunsReadOnlyPreflightAndComputesVersion proves the read-only
// stages run NORMALLY under --dry-run: the preflight gate chain reads happen (fetch,
// clean-tree, branch, tag-free, remote-sync) and the version is computed (the plan
// and notes carry the resolved v0.0.1).
func TestRelease_DryRun_RunsReadOnlyPreflightAndComputesVersion(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedDryRunFirstRelease(f, root, "main", "v0.0.1")
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), dryRunOptions()); err != nil {
		t.Fatalf("dry-run Release returned unexpected error: %v", err)
	}

	// The read-only preflight gate reads must have happened.
	for _, want := range [][]string{
		{"fetch", "--tags"},
		{"status", "--porcelain"},
		{"rev-parse", "--abbrev-ref", "HEAD"},
		{"rev-parse", "-q", "--verify", "refs/tags/v0.0.1"},
		{"rev-list", "--left-right", "--count", "@{u}...HEAD"},
		{"ls-remote", "--tags", "origin", "refs/tags/v0.0.1"},
	} {
		if !invokedWith(f, "git", want...) {
			t.Errorf("dry-run did not run read-only preflight probe: git %v", want)
		}
	}
	// The version is computed: RunStarted carries the resolved bare version. It now
	// follows the read-only gate completions and the blocking notes stage, so locate it
	// by kind rather than a fixed index.
	start, _ := rec.At(indexOfKind(rec, presentertest.KindRunStarted))
	if start.RunStarted.Version != "0.0.1" {
		t.Errorf("RunStarted.Version = %q, want computed %q", start.RunStarted.Version, "0.0.1")
	}
}

// TestRelease_DryRun_PrintsFullPlan proves --dry-run prints the FULL plan via the
// Presenter: the commit it would make (with its real subject), the tag it would
// create, and the publish target. The dry-run plan is the LAST ShowPlan recorded
// (the gate-preview plan fires first; the would-do plan is layered on top).
func TestRelease_DryRun_PrintsFullPlan(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedDryRunFirstRelease(f, root, "main", "v0.0.1")
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), dryRunOptions()); err != nil {
		t.Fatalf("dry-run Release returned unexpected error: %v", err)
	}

	plan := lastPlan(t, rec)
	wantSteps := map[string]string{
		"commit":  "🌿 Release v0.0.1",
		"tag":     "v0.0.1",
		"publish": "v0.0.1",
	}
	for _, step := range plan.Steps {
		want, ok := wantSteps[step.Verb]
		if !ok {
			continue
		}
		if step.Target != want {
			t.Errorf("plan step %q target = %q, want %q", step.Verb, step.Target, want)
		}
		delete(wantSteps, step.Verb)
	}
	for verb := range wantSteps {
		t.Errorf("dry-run plan missing step %q", verb)
	}
}

// TestRelease_DryRun_GeneratesNotesPreview proves the AI notes preview is generated
// under --dry-run: a prior-tag NORMAL-AI run assembles the diff, runs the AI, and
// the resulting body is PREVIEWED via ShowNotes — even though no mutation lands.
func TestRelease_DryRun_GeneratesNotesPreview(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	f.SeedSequence("git", ScriptedOut(githubRemoteURL)) // remote get-url origin (provider detection)
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
	rec := &presentertest.RecordingPresenter{}

	opts := dryRunOptions()
	if err := engine.Release(t.Context(), newDeps(rec, f), opts); err != nil {
		t.Fatalf("dry-run Release returned unexpected error: %v", err)
	}

	// The AI transport ran (the diff was assembled and handed to the AI).
	if !invokedWith(f, "claude", "-p") {
		t.Errorf("dry-run did not run the AI notes path; got %v", commandLines(f.Invocations()))
	}
	// The generated body was previewed.
	if got := lastNotesBody(t, rec); got != aiBody {
		t.Errorf("dry-run notes preview body = %q, want AI body %q", got, aiBody)
	}
	// No mutation reached the wrapper.
	assertNoMutation(t, f)
}

// TestRelease_DryRun_PublishDisabled_NoPublishStep proves that under --dry-run with
// publish=false the plan carries NO publish step (publishing is disabled), and no gh
// command runs — consistent with the real publish=false run.
func TestRelease_DryRun_PublishDisabled_NoPublishStep(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release]\npublish = false\n")

	f := runner.NewFakeRunner()
	// publish=false skips provider detection entirely, so no `remote get-url origin`.
	f.SeedSequence("git",
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),            // tag --list
		ScriptedOut(""),            // fetch --tags
		ScriptedOut(""),            // status --porcelain
		ScriptedOut("main"),        // rev-parse --abbrev-ref HEAD
		ScriptedNonZero(),          // rev-parse -q --verify refs/tags/v0.0.1
		ScriptedOut("0\t1"),        // rev-list left-right count
		ScriptedOut(""),            // ls-remote --tags
		ScriptedOut(startingSHA),   // rev-parse HEAD
	)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), dryRunOptions()); err != nil {
		t.Fatalf("dry-run Release returned unexpected error: %v", err)
	}

	plan := lastPlan(t, rec)
	for _, step := range plan.Steps {
		if step.Verb == "publish" {
			t.Errorf("dry-run plan includes a publish step despite publish=false: %+v", step)
		}
	}
	for _, inv := range f.Invocations() {
		if inv.Name == "gh" {
			t.Errorf("dry-run with publish=false issued a gh command: %q", commandLine(inv))
		}
	}
	assertNoMutation(t, f)
}

// TestRelease_DryRun_ProviderUnresolved_DowngradesPublishInPlan proves that under
// --dry-run a publish=true run whose provider cannot be resolved (a non-github.com
// remote) does NOT silently assume GitHub: it warns (downgrade) and the plan's
// publish target reflects the downgrade rather than naming a provider release.
func TestRelease_DryRun_ProviderUnresolved_DowngradesPublishInPlan(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),            // tag --list
		ScriptedOut(""),            // fetch --tags
		ScriptedOut(""),            // status --porcelain
		ScriptedOut("main"),        // rev-parse --abbrev-ref HEAD
		ScriptedNonZero(),          // rev-parse -q --verify refs/tags/v0.0.1
		ScriptedOut("0\t1"),        // rev-list left-right count
		ScriptedOut(""),            // ls-remote --tags
		ScriptedOut(startingSHA),   // rev-parse HEAD
		ScriptedOut("https://gitlab.com/acme/widget.git"), // remote get-url origin (unrecognised host)
	)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), dryRunOptions()); err != nil {
		t.Fatalf("dry-run Release returned unexpected error: %v", err)
	}

	// The downgrade is warned (the same loud signal the real path emits).
	if !downgradeWarned(rec) {
		t.Errorf("dry-run did not warn that publishing downgraded for an unresolved provider")
	}
	// The plan's publish step (if present) must NOT name a normal provider release
	// target — it reflects the downgrade.
	plan := lastPlan(t, rec)
	for _, step := range plan.Steps {
		if step.Verb == "publish" && step.Target == "v0.0.1" {
			t.Errorf("dry-run plan names a normal provider release despite an unresolved provider: %+v", step)
		}
	}
	assertNoMutation(t, f)
}

// TestRelease_DryRun_SkipsHooksAndReports re-confirms the 3-11 behaviour STILL HOLDS
// under the full dry-run: all three configured hooks are skipped (no `sh` runs) and
// each skip is reported, AND no mutation lands (the would-do mutations never reach
// the wrapper).
func TestRelease_DryRun_SkipsHooksAndReports(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npreflight = \"check.sh\"\npre_tag = \"build.sh\"\npost_release = \"notify.sh\"\n")

	f := runner.NewFakeRunner()
	seedDryRunFirstRelease(f, root, "main", "v0.0.1")
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), dryRunOptions()); err != nil {
		t.Fatalf("dry-run Release returned unexpected error: %v", err)
	}

	if shInvoked(f) {
		t.Errorf("a hook ran under the full dry-run; no `sh -c …` must reach the runner: %v", commandLines(f.Invocations()))
	}
	warnWithMessage(t, rec, "skipping preflight hook")
	warnWithMessage(t, rec, "skipping pre_tag hook")
	warnWithMessage(t, rec, "skipping post_release hook")
	assertNoMutation(t, f)
}

// TestRelease_DryRun_RepoFilesUnchanged proves the byte-for-byte guarantee on the
// WORKING TREE (not just the git commands): under --dry-run the version-file
// projection and the changelog write are BOTH skipped, so the pre-existing version
// file keeps its exact bytes and NO CHANGELOG.md is created. (assertNoMutation covers
// the git/gh side; this covers the filesystem side.)
func TestRelease_DryRun_RepoFilesUnchanged(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release]\nversion_file = \"release.txt\"\n")
	const beforeVersion = "0.0.0\n"
	seedFile(t, root, "release.txt", beforeVersion)

	f := runner.NewFakeRunner()
	seedDryRunFirstRelease(f, root, "main", "v0.0.1")
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), dryRunOptions()); err != nil {
		t.Fatalf("dry-run Release returned unexpected error: %v", err)
	}

	// The version file is byte-for-byte unchanged (no projection ran).
	if got := readFile(t, root, "release.txt"); got != beforeVersion {
		t.Errorf("version file changed under dry-run: got %q, want %q", got, beforeVersion)
	}
	// No changelog was written.
	if _, err := os.Stat(filepath.Join(root, "CHANGELOG.md")); !os.IsNotExist(err) {
		t.Errorf("dry-run wrote CHANGELOG.md; the working tree must be unchanged (stat err = %v)", err)
	}
	assertNoMutation(t, f)
}

// lastPlan returns the LAST recorded ShowPlan payload — the dry-run would-do plan,
// which is layered after the gate-preview plan — failing the test if none was
// recorded.
func lastPlan(t *testing.T, rec *presentertest.RecordingPresenter) presenter.Plan {
	t.Helper()
	var plan presenter.Plan
	found := false
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindShowPlan {
			plan = ev.ShowPlan
			found = true
		}
	}
	if !found {
		t.Fatalf("no ShowPlan event recorded; kinds = %v", rec.Kinds())
	}
	return plan
}

// lastNotesBody returns the body of the LAST recorded ShowNotes payload, failing
// the test if none was recorded.
func lastNotesBody(t *testing.T, rec *presentertest.RecordingPresenter) string {
	t.Helper()
	body := ""
	found := false
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindShowNotes {
			body = ev.ShowNotes.Body
			found = true
		}
	}
	if !found {
		t.Fatalf("no ShowNotes event recorded; kinds = %v", rec.Kinds())
	}
	return body
}

// downgradeWarned reports whether any recorded Warn carries the publish-downgrade
// label — the loud signal mint emits when the provider cannot be resolved.
func downgradeWarned(rec *presentertest.RecordingPresenter) bool {
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindWarn && ev.Warn.Label == "publish skipped" {
			return true
		}
	}
	return false
}
