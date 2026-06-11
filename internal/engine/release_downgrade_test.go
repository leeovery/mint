package engine_test

import (
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// This file pins task 4-10: the safe downgrade to tag + push when the publishing
// provider cannot be resolved. With publish=true (the default) an unresolved
// provider — an unknown recognised value, a non-github.com host, no remote, or an
// unparseable SSH URL — must NOT abort. mint warns LOUDLY (naming the reason) and
// downgrades the run to tag + push ONLY: the annotated tag and the atomic push
// still happen, but the gh install/auth gate is never reached and no provider
// release is created, so the pushed tag is never stranded. publish=false stays a
// SILENT tag + push (the user opted out — not a downgrade), and mint NEVER silently
// assumes GitHub for an unresolved provider.

// downgradeWarnLabel is the Warn label the provider-unresolved downgrade rides.
const downgradeWarnLabel = "publish skipped"

// seedDowngradeGit scripts the publish=true first-release "git" timeline that ends
// at a successful tag + atomic push WITHOUT any gh call — the downgrade shape. It
// scripts the full happy git timeline including the remote read (so provider detection
// still runs); the only difference from a published run is that the caller seeds NO gh,
// because a downgrade must never reach gh.
func seedDowngradeGit(f *runner.FakeRunner, root, releaseBranch, tag, remoteURL string) {
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
		ScriptedOut(""),                      // -C root add CHANGELOG.md
		ScriptedOut(""),                      // -C root commit -m
		ScriptedOut(remoteURL),               // remote get-url origin (provider detection)
		ScriptedOut(""),                      // tag -a {tag} -F -
		ScriptedOut(""),                      // push --atomic origin HEAD {tag}
	)
}

// assertDowngradedTagPush asserts the shared downgrade invariant: NO gh call was
// ever made (no auth gate, no release create), but the annotated tag AND the atomic
// push both ran — so the run reached the point of no return cleanly and the tag is
// never stranded waiting on a release mint could not create.
func assertDowngradedTagPush(t *testing.T, f *runner.FakeRunner, tag string) {
	t.Helper()

	for _, inv := range f.Invocations() {
		if inv.Name == "gh" {
			t.Errorf("gh was invoked on a downgrade (gate or publish must be skipped): %q", commandLine(inv))
		}
	}
	if !invokedWith(f, "git", "tag", "-a", tag, "-F", "-") {
		t.Errorf("downgrade did not create the annotated tag %s\ngot: %v", tag, commandLines(f.Invocations()))
	}
	if !invokedWith(f, "git", "push", "--atomic", "origin", "HEAD", tag) {
		t.Errorf("downgrade did not reach the atomic push for %s\ngot: %v", tag, commandLines(f.Invocations()))
	}
}

// hasDowngradeWarn reports whether a provider-unresolved downgrade Warn was
// recorded carrying the given reason substring (the reason naming WHY publishing
// was skipped).
func hasDowngradeWarn(rec *presentertest.RecordingPresenter, reason string) bool {
	for _, ev := range rec.Events {
		if ev.Kind != presentertest.KindWarn || ev.Warn.Label != downgradeWarnLabel {
			continue
		}
		if reason == "" {
			return true
		}
		if strings.Contains(ev.Warn.Message, reason) || strings.Contains(ev.Warn.Output, reason) {
			return true
		}
	}
	return false
}

// TestRelease_Downgrade_UnknownProviderValue proves an unknown recognised provider
// value (provider=gitlab) with publish=true warns LOUDLY (naming the value) and
// downgrades to tag + push only — no gh gate, no provider release, but the tag and
// push still happen. The remote IS github.com, proving the unsupported value
// governs and mint does NOT silently fall through to GitHub.
func TestRelease_Downgrade_UnknownProviderValue(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release]\nprovider = \"gitlab\"\n")

	f := runner.NewFakeRunner()
	seedDowngradeGit(f, root, "main", "v0.0.1", "https://github.com/acme/widget.git")
	// Deliberately NO gh seed: a gh call on a downgrade is a bug — the FakeRunner
	// errors on an unseeded command, so any gh attempt would fail the run.
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err != nil {
		t.Fatalf("Release returned %v, want nil (an unresolved provider downgrades, it does not abort)", err)
	}

	if !hasDowngradeWarn(rec, `gitlab`) {
		t.Errorf("no loud downgrade warning naming the unsupported provider value; warns = %v", warnMessages(rec))
	}
	assertDowngradedTagPush(t, f, "v0.0.1")

	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("downgrade run did not finish; last event = %v", fin.Kind)
	}
	// A downgrade publishes no provider release, so the footer carries NO URL.
	if fin.RunFinished.URL != "" {
		t.Errorf("RunFinished.URL = %q, want empty on a no-publish downgrade", fin.RunFinished.URL)
	}
}

// TestRelease_Downgrade_NonGitHubHost proves auto-detection with no matching driver
// — a non-github.com host (GHE / GitLab / Gitea), publish=true, no explicit
// provider — warns loudly (naming the host) and downgrades to tag + push only.
func TestRelease_Downgrade_NonGitHubHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		remoteURL string
		wantHost  string
	}{
		{name: "GitHub Enterprise host", remoteURL: "https://github.example.com/acme/widget.git", wantHost: "github.example.com"},
		{name: "GitLab host", remoteURL: "https://gitlab.com/acme/widget.git", wantHost: "gitlab.com"},
		{name: "Gitea host", remoteURL: "https://gitea.acme.io/acme/widget.git", wantHost: "gitea.acme.io"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			seedDowngradeGit(f, root, "main", "v0.0.1", tt.remoteURL)
			rec := &presentertest.RecordingPresenter{}

			err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
			if err != nil {
				t.Fatalf("Release returned %v, want nil (a non-github host downgrades)", err)
			}

			if !hasDowngradeWarn(rec, tt.wantHost) {
				t.Errorf("no loud downgrade warning naming the host %q; warns = %v", tt.wantHost, warnMessages(rec))
			}
			assertDowngradedTagPush(t, f, "v0.0.1")
		})
	}
}

// TestRelease_Downgrade_UnmatchableSSHHost proves an unparseable SSH remote (no
// host could be determined) with publish=true warns loudly and downgrades to tag +
// push only — never a silent GitHub assumption.
func TestRelease_Downgrade_UnmatchableSSHHost(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	// An SCP-like form with no host:path colon — parseHost cannot extract a host.
	seedDowngradeGit(f, root, "main", "v0.0.1", "git@nohostcolon")
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err != nil {
		t.Fatalf("Release returned %v, want nil (an unparseable SSH remote downgrades)", err)
	}

	if !hasDowngradeWarn(rec, "git@nohostcolon") {
		t.Errorf("no loud downgrade warning naming the unparseable remote; warns = %v", warnMessages(rec))
	}
	assertDowngradedTagPush(t, f, "v0.0.1")
}

// TestRelease_Downgrade_NoRemote proves no remote at all (git remote get-url origin
// exits non-zero, treated as an empty remote) with publish=true warns loudly and
// downgrades to tag + push only.
func TestRelease_Downgrade_NoRemote(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
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
		ScriptedOut(""),            // -C root add CHANGELOG.md
		ScriptedOut(""),            // -C root commit -m
		ScriptedNonZero(),          // remote get-url origin (NO origin remote -> empty URL)
		ScriptedOut(""),            // tag -a v0.0.1 -F -
		ScriptedOut(""),            // push --atomic origin HEAD v0.0.1
	)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err != nil {
		t.Fatalf("Release returned %v, want nil (no remote downgrades to tag + push)", err)
	}

	if !hasDowngradeWarn(rec, "no remote") {
		t.Errorf("no loud downgrade warning for the no-remote case; warns = %v", warnMessages(rec))
	}
	assertDowngradedTagPush(t, f, "v0.0.1")
}

// TestRelease_PublishFalse_SilentTagPush proves publish=false is a SILENT tag +
// push: no gh, no provider release, AND crucially NO downgrade warning — the user
// opted out, which is distinct from the warn-and-downgrade case. (The git remote is
// never even read, because resolution is gated behind publish=true.)
func TestRelease_PublishFalse_SilentTagPush(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release]\npublish = false\n")

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
		ScriptedOut(startingSHA),   // rev-parse HEAD (capture the clean start)
		ScriptedOut(""),            // -C root add CHANGELOG.md
		ScriptedOut(""),            // -C root commit -m
		ScriptedOut(""),            // tag -a v0.0.1 -F -
		ScriptedOut(""),            // push --atomic origin HEAD v0.0.1
	)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err != nil {
		t.Fatalf("Release returned %v, want nil (publish=false is a silent tag + push)", err)
	}

	// publish=false must NOT emit the downgrade warning — opting out is not a downgrade.
	if hasDowngradeWarn(rec, "") {
		t.Errorf("publish=false emitted a downgrade warning; it must be SILENT; warns = %v", warnMessages(rec))
	}
	// It must also not read the remote at all (resolution is gated behind publish=true).
	if invokedWith(f, "git", "remote", "get-url", "origin") {
		t.Errorf("publish=false read the remote URL; provider resolution must be gated behind publish=true")
	}
	assertDowngradedTagPush(t, f, "v0.0.1")
}

// TestRelease_Downgrade_NeverAssumesGitHub proves the load-bearing safety property:
// on an unresolved provider mint NEVER silently selects the GitHub driver. Across
// every unresolved cause no `gh release create` is ever issued — the tag is created
// and pushed, but publishing is skipped, not silently retargeted at GitHub.
func TestRelease_Downgrade_NeverAssumesGitHub(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedDowngradeGit(f, root, "main", "v0.0.1", "https://gitlab.com/acme/widget.git")
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err != nil {
		t.Fatalf("Release returned %v, want nil", err)
	}

	if invokedWith(f, "gh", "release", "create", "v0.0.1", "--title", "v0.0.1", "--notes-file", "-", "--verify-tag") {
		t.Errorf("an unresolved provider silently fell through to the GitHub driver (gh release create)")
	}
	assertDowngradedTagPush(t, f, "v0.0.1")
}
