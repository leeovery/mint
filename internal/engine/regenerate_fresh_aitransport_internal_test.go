package engine

// White-box proofs for the regenerate wiring site `resolveFreshTransport` (task 2-5):
// the EASY-MISS third construction site. With a nil injected transport (the production
// branch) it must source BOTH its command and its per-attempt deadline from the RELEASE
// verb's per-key accessors (cfg.AICommandFor(VerbRelease) / cfg.TimeoutFor(VerbRelease)),
// because regenerate rides on [release] — there is no [regenerate] table. The command is
// observable as the invoked argv; the deadline is observable as the presence/absence of a
// context deadline on the call the transport hands the runner. The FakeRunner discards the
// context, so these timeout proofs reuse the deadlineRunner spy from
// release_aitransport_internal_test.go.

import (
	"testing"
	"time"

	"mint/internal/config"
)

// TestResolveFreshTransport_SourcesCommandFromReleaseAccessor proves the production
// branch threads cfg.AICommandFor(VerbRelease): a [release].ai_command override is the
// binary+args the fresh-regenerate transport invokes — not the bare shared top-level
// command, and certainly not [commit].
func TestResolveFreshTransport_SourcesCommandFromReleaseAccessor(t *testing.T) {
	t.Parallel()

	shared := "shared-bot"
	release := "verbbot run --json"
	commit := "wrongbot"
	cfg := config.Config{
		AICommand: shared,
		Release:   config.Release{AICommand: &release},
		Commit:    config.Commit{AICommand: &commit},
		Timeout:   durationPtr(60 * time.Second),
	}

	spy := &deadlineRunner{}
	transport := resolveFreshTransport(spy, nil, cfg)
	if _, err := transport.Generate(t.Context(), "prompt"); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if spy.name != "verbbot" {
		t.Errorf("invoked binary = %q, want the [release].ai_command override binary %q (regenerate routes through [release], not shared/[commit])", spy.name, "verbbot")
	}
	wantArgs := []string{"run", "--json"}
	if len(spy.args) != len(wantArgs) || spy.args[0] != wantArgs[0] || spy.args[1] != wantArgs[1] {
		t.Errorf("invoked args = %v, want the [release].ai_command override args %v", spy.args, wantArgs)
	}
}

// TestResolveFreshTransport_ExplicitZeroTimeoutThreadsNoDeadline proves the timeout is
// sourced from cfg.TimeoutFor(VerbRelease): a [release].timeout of explicit 0 ("no
// deadline") reaches the transport, so the context it hands the runner carries NO
// deadline. If the wiring site forgot the field (zero-by-omission) NewTransport would
// panic on the nil pointer — so reaching Generate at all already proves the field was
// threaded; the no-deadline context proves it was threaded from the release accessor.
func TestResolveFreshTransport_ExplicitZeroTimeoutThreadsNoDeadline(t *testing.T) {
	t.Parallel()

	zero := time.Duration(0)
	cfg := config.Config{
		AICommand: "claude",
		Release:   config.Release{Timeout: &zero},
		Timeout:   durationPtr(60 * time.Second),
	}

	spy := &deadlineRunner{}
	transport := resolveFreshTransport(spy, nil, cfg)
	if _, err := transport.Generate(t.Context(), "prompt"); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if spy.hadDeadline {
		t.Errorf("context carried a deadline; a [release].timeout of explicit 0 must thread NO deadline through resolveFreshTransport")
	}
}

// TestResolveFreshTransport_PositiveTimeoutThreadsDeadline proves a positive resolved
// release timeout reaches the transport as a per-attempt deadline: the context the
// transport hands the runner carries a deadline. With [release].timeout absent, the
// accessor resolves the shared/floor positive value, which must drive a real deadline.
func TestResolveFreshTransport_PositiveTimeoutThreadsDeadline(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		AICommand: "claude",
		Timeout:   durationPtr(90 * time.Second),
	}

	spy := &deadlineRunner{}
	transport := resolveFreshTransport(spy, nil, cfg)
	if _, err := transport.Generate(t.Context(), "prompt"); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if !spy.hadDeadline {
		t.Errorf("context carried no deadline; a positive resolved release timeout must thread a per-attempt deadline through resolveFreshTransport")
	}
}
