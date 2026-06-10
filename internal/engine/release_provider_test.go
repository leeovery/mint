package engine_test

import (
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// seedHappyGitRemote scripts the no-tags first-release "git" timeline like
// seedHappyGit but with a CALLER-SUPPLIED remote URL for the
// `git remote get-url origin` read that drives provider auto-detection. The
// trailing gh calls (auth status, release create) are seeded by the caller.
func seedHappyGitRemote(f *runner.FakeRunner, root, releaseBranch, tag, remoteURL string) {
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

// TestRelease_ProviderDetection_ResolvesGitHubAndPublishes proves the provider is
// auto-detected from the release remote across every github.com URL form (and an
// explicit provider=github override on a non-github host): each resolves to the
// GitHub driver behind the Publisher interface, so the run reaches a `gh release
// create` and the gh auth gate runs only for the selected driver, before the tag.
func TestRelease_ProviderDetection_ResolvesGitHubAndPublishes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		remoteURL string
		config    string
	}{
		{
			name:      "https github.com remote",
			remoteURL: "https://github.com/acme/widget.git",
		},
		{
			name:      "scp-like ssh github.com remote",
			remoteURL: "git@github.com:acme/widget.git",
		},
		{
			name:      "ssh:// github.com remote",
			remoteURL: "ssh://git@github.com/acme/widget.git",
		},
		{
			name:      "explicit provider=github overrides a non-github host",
			remoteURL: "https://example.com/acme/widget.git",
			config:    "[release]\nprovider = \"github\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			if tt.config != "" {
				writeConfig(t, root, tt.config)
			}

			f := runner.NewFakeRunner()
			seedHappyGitRemote(f, root, "main", "v0.0.1", tt.remoteURL)
			f.Seed("gh", runner.Result{}, nil) // gh auth status, then gh release create
			rec := &presentertest.RecordingPresenter{}

			err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
			if err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			// Resolution must read the release remote through the runner — the single
			// git invocation that drives provider auto-detection.
			if !invokedWith(f, "git", "remote", "get-url", "origin") {
				t.Errorf("provider resolution did not read the remote URL through the runner\ngot: %v", commandLines(f.Invocations()))
			}

			// The resolved GitHub driver must have published the release — the run
			// reached `gh release create` for the computed tag through the interface.
			if !invokedWith(f, "gh", "release", "create", "v0.0.1", "--title", "v0.0.1", "--notes-file", "-", "--verify-tag") {
				t.Errorf("provider was not resolved to the GitHub driver: no gh release create for v0.0.1\ngot: %v", commandLines(f.Invocations()))
			}

			// The gh auth gate must still gate the selected driver: it runs before the
			// tag is created (so a missing/unauth gh never strands a pushed tag).
			assertGhAuthBeforeTag(t, f)
		})
	}
}

// assertGhAuthBeforeTag asserts the `gh auth status` gate was invoked and that it
// preceded `git tag -a` — the conditional gh gate still gates the resolved driver
// before any tag is created.
func assertGhAuthBeforeTag(t *testing.T, f *runner.FakeRunner) {
	t.Helper()

	authAt, tagAt := -1, -1
	for i, inv := range f.Invocations() {
		switch commandLine(inv) {
		case "gh auth status":
			if authAt == -1 {
				authAt = i
			}
		case "git tag -a v0.0.1 -F -":
			if tagAt == -1 {
				tagAt = i
			}
		}
	}
	if authAt == -1 {
		t.Fatalf("gh auth gate was not invoked for the resolved driver\ngot: %v", commandLines(f.Invocations()))
	}
	if tagAt == -1 {
		t.Fatalf("tag was never created\ngot: %v", commandLines(f.Invocations()))
	}
	if authAt > tagAt {
		t.Errorf("gh auth gate ran at %d, after the tag at %d; it must gate before the tag", authAt, tagAt)
	}
}
