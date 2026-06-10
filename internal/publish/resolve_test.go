package publish_test

import (
	"errors"
	"testing"

	"mint/internal/publish"
	"mint/internal/runner"
)

// TestResolvePublisher_HTTPSGitHubHost selects the GitHub driver from an HTTPS
// github.com remote — the canonical clone URL form.
func TestResolvePublisher_HTTPSGitHubHost(t *testing.T) {
	t.Parallel()

	pub, err := publish.ResolvePublisher("https://github.com/owner/repo.git", "", runner.NewFakeRunner())
	if err != nil {
		t.Fatalf("ResolvePublisher returned unexpected error: %v", err)
	}
	if _, ok := pub.(*publish.GitHubPublisher); !ok {
		t.Errorf("resolved publisher = %T, want *publish.GitHubPublisher", pub)
	}
}

// TestResolvePublisher_SCPSSHGitHubHost selects the GitHub driver from the
// SCP-like SSH form git@github.com:owner/repo.git (no scheme, host:path).
func TestResolvePublisher_SCPSSHGitHubHost(t *testing.T) {
	t.Parallel()

	pub, err := publish.ResolvePublisher("git@github.com:owner/repo.git", "", runner.NewFakeRunner())
	if err != nil {
		t.Fatalf("ResolvePublisher returned unexpected error: %v", err)
	}
	if _, ok := pub.(*publish.GitHubPublisher); !ok {
		t.Errorf("resolved publisher = %T, want *publish.GitHubPublisher", pub)
	}
}

// TestResolvePublisher_SSHSchemeGitHubHost selects the GitHub driver from the
// ssh:// URL form ssh://git@github.com/owner/repo.git.
func TestResolvePublisher_SSHSchemeGitHubHost(t *testing.T) {
	t.Parallel()

	pub, err := publish.ResolvePublisher("ssh://git@github.com/owner/repo.git", "", runner.NewFakeRunner())
	if err != nil {
		t.Fatalf("ResolvePublisher returned unexpected error: %v", err)
	}
	if _, ok := pub.(*publish.GitHubPublisher); !ok {
		t.Errorf("resolved publisher = %T, want *publish.GitHubPublisher", pub)
	}
}

// TestResolvePublisher_ExplicitGitHubProviderOverridesDetection forces the
// GitHub driver via provider=github even though the remote host would NOT match
// (a non-github.com host) — the explicit override wins over detection.
func TestResolvePublisher_ExplicitGitHubProviderOverridesDetection(t *testing.T) {
	t.Parallel()

	// The remote points at a host that auto-detection would NOT resolve to GitHub;
	// the explicit provider=github must still select the GitHub driver, proving the
	// override path is taken rather than detection.
	pub, err := publish.ResolvePublisher("https://example.com/owner/repo.git", "github", runner.NewFakeRunner())
	if err != nil {
		t.Fatalf("ResolvePublisher returned unexpected error: %v", err)
	}
	if _, ok := pub.(*publish.GitHubPublisher); !ok {
		t.Errorf("resolved publisher = %T, want *publish.GitHubPublisher", pub)
	}
}

// TestResolvePublisher_ResolvedDriverIsBehindInterface proves the resolver's
// return is consumed through the Publisher interface — callers never need the
// concrete type, so a future driver slots in with no caller change.
func TestResolvePublisher_ResolvedDriverIsBehindInterface(t *testing.T) {
	t.Parallel()

	var pub publish.Publisher
	pub, err := publish.ResolvePublisher("https://github.com/owner/repo.git", "", runner.NewFakeRunner())
	if err != nil {
		t.Fatalf("ResolvePublisher returned unexpected error: %v", err)
	}
	if pub == nil {
		t.Fatal("resolved publisher is nil, want a usable Publisher")
	}
}

// TestResolvePublisher_NonGitHubHostUnresolved exposes the unresolved outcome
// (the sentinel) for a non-github.com host with no override — the seam task 4-10
// acts on. 4-9 does NOT implement the downgrade itself.
func TestResolvePublisher_NonGitHubHostUnresolved(t *testing.T) {
	t.Parallel()

	pub, err := publish.ResolvePublisher("https://gitlab.com/owner/repo.git", "", runner.NewFakeRunner())
	if !errors.Is(err, publish.ErrProviderUnresolved) {
		t.Errorf("error = %v, want ErrProviderUnresolved", err)
	}
	if pub != nil {
		t.Errorf("resolved publisher = %v, want nil on an unresolved provider", pub)
	}
}

// TestResolvePublisher_UnknownProviderValueUnresolved exposes the unresolved
// outcome for a recognised-but-unsupported provider value (e.g. gitlab) — the
// override must NOT silently fall through to GitHub; it surfaces the sentinel for
// 4-10 to downgrade on.
func TestResolvePublisher_UnknownProviderValueUnresolved(t *testing.T) {
	t.Parallel()

	// Even though the remote host IS github.com, an explicit unsupported provider
	// value must not silently resolve to GitHub — the explicit value governs.
	pub, err := publish.ResolvePublisher("https://github.com/owner/repo.git", "gitlab", runner.NewFakeRunner())
	if !errors.Is(err, publish.ErrProviderUnresolved) {
		t.Errorf("error = %v, want ErrProviderUnresolved", err)
	}
	if pub != nil {
		t.Errorf("resolved publisher = %v, want nil on an unsupported provider value", pub)
	}
}

// TestResolvePublisher_EmptyRemoteUnresolved exposes the unresolved outcome when
// there is no remote URL to detect from and no override — again the sentinel for
// 4-10, never a silent GitHub assumption.
func TestResolvePublisher_EmptyRemoteUnresolved(t *testing.T) {
	t.Parallel()

	pub, err := publish.ResolvePublisher("", "", runner.NewFakeRunner())
	if !errors.Is(err, publish.ErrProviderUnresolved) {
		t.Errorf("error = %v, want ErrProviderUnresolved", err)
	}
	if pub != nil {
		t.Errorf("resolved publisher = %v, want nil with no remote and no override", pub)
	}
}

// TestResolvePublisher_UnresolvedReason proves the unresolved sentinel carries a
// distinguishable, human-readable REASON across its four causes so the engine's
// downgrade warning (task 4-10) can name WHY publishing was skipped — an unknown
// provider value vs an unmatched host vs no remote vs an unparseable SSH URL. Each
// case still satisfies errors.Is(err, ErrProviderUnresolved) AND exposes a
// *publish.UnresolvedError via errors.As whose Reason() is the named cause.
func TestResolvePublisher_UnresolvedReason(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		remoteURL  string
		provider   string
		wantReason string
	}{
		{
			name:       "unknown provider value",
			remoteURL:  "https://github.com/owner/repo.git",
			provider:   "gitlab",
			wantReason: `unsupported provider "gitlab"`,
		},
		{
			name:       "non-github.com host",
			remoteURL:  "https://gitlab.com/owner/repo.git",
			wantReason: `unrecognised host "gitlab.com"`,
		},
		{
			name:       "no remote",
			remoteURL:  "",
			wantReason: "no remote configured",
		},
		{
			name:       "unparseable ssh url",
			remoteURL:  "git@nohostcolon",
			wantReason: `could not determine host from remote "git@nohostcolon"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pub, err := publish.ResolvePublisher(tt.remoteURL, tt.provider, runner.NewFakeRunner())
			if pub != nil {
				t.Errorf("resolved publisher = %v, want nil on an unresolved provider", pub)
			}
			if !errors.Is(err, publish.ErrProviderUnresolved) {
				t.Fatalf("error = %v, want it to wrap ErrProviderUnresolved", err)
			}

			var unresolved *publish.UnresolvedError
			if !errors.As(err, &unresolved) {
				t.Fatalf("error = %v, want a *publish.UnresolvedError", err)
			}
			if unresolved.Reason() != tt.wantReason {
				t.Errorf("Reason() = %q, want %q", unresolved.Reason(), tt.wantReason)
			}
		})
	}
}
