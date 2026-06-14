package engine

// White-box proofs for the release wiring site `aiTransport` (task 2-3): the
// production transport must source BOTH its command and its per-attempt deadline
// from the release verb's per-key accessors (cfg.AICommandFor(VerbRelease) /
// cfg.TimeoutFor(VerbRelease)). The command is observable as the invoked argv; the
// deadline is observable as the presence/absence of a context deadline on the call
// the transport hands the runner — config's explicit 0 ("no deadline") must reach
// the transport as a deadline-free context, while a positive override must reach it
// as a context carrying a deadline. The FakeRunner discards the context, so these
// timeout proofs use runner.DeadlineRecordingRunner instead.

import (
	"testing"
	"time"

	"mint/internal/config"
	"mint/internal/runner"
)

// TestAITransport_SourcesCommandFromReleaseAccessor proves aiTransport threads
// cfg.AICommandFor(VerbRelease): a [release].ai_command override is the binary+args
// the production transport invokes — not the bare shared top-level command.
func TestAITransport_SourcesCommandFromReleaseAccessor(t *testing.T) {
	t.Parallel()

	shared := "shared-bot"
	override := "verbbot run --json"
	cfg := config.Config{
		AICommand: shared,
		Release:   config.Release{AICommand: &override},
		Timeout:   runner.DurationPtr(60 * time.Second),
	}

	spy := &runner.DeadlineRecordingRunner{}
	transport := aiTransport(ReleaseDeps{Runner: spy}, cfg)
	if _, err := transport.Generate(t.Context(), "prompt"); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if spy.Name() != "verbbot" {
		t.Errorf("invoked binary = %q, want the [release].ai_command override binary %q", spy.Name(), "verbbot")
	}
	wantArgs := []string{"run", "--json"}
	if len(spy.Args()) != len(wantArgs) || spy.Args()[0] != wantArgs[0] || spy.Args()[1] != wantArgs[1] {
		t.Errorf("invoked args = %v, want the [release].ai_command override args %v", spy.Args(), wantArgs)
	}
}

// TestAITransport_ExplicitZeroTimeoutThreadsNoDeadline proves the timeout is sourced
// from cfg.TimeoutFor(VerbRelease): a [release].timeout of explicit 0 ("no deadline")
// reaches the transport, so the context the transport hands the runner carries NO
// deadline. If the wiring site forgot the field (zero-by-omission) NewTransport would
// panic on the nil pointer — so reaching Generate at all already proves the field was
// threaded; the no-deadline context proves it was threaded from the accessor.
func TestAITransport_ExplicitZeroTimeoutThreadsNoDeadline(t *testing.T) {
	t.Parallel()

	zero := time.Duration(0)
	cfg := config.Config{
		AICommand: "claude",
		Release:   config.Release{Timeout: &zero},
		Timeout:   runner.DurationPtr(60 * time.Second),
	}

	spy := &runner.DeadlineRecordingRunner{}
	transport := aiTransport(ReleaseDeps{Runner: spy}, cfg)
	if _, err := transport.Generate(t.Context(), "prompt"); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if spy.HadDeadline() {
		t.Errorf("context carried a deadline; a [release].timeout of explicit 0 must thread NO deadline through aiTransport")
	}
}

// TestAITransport_PositiveTimeoutThreadsDeadline proves a positive resolved timeout
// reaches the transport as a per-attempt deadline: the context the transport hands the
// runner carries a deadline. With the [release].timeout absent, the accessor resolves
// the shared/floor positive value, which must drive a real per-attempt deadline.
func TestAITransport_PositiveTimeoutThreadsDeadline(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		AICommand: "claude",
		Timeout:   runner.DurationPtr(90 * time.Second),
	}

	spy := &runner.DeadlineRecordingRunner{}
	transport := aiTransport(ReleaseDeps{Runner: spy}, cfg)
	if _, err := transport.Generate(t.Context(), "prompt"); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if !spy.HadDeadline() {
		t.Errorf("context carried no deadline; a positive resolved timeout must thread a per-attempt deadline through aiTransport")
	}
}
