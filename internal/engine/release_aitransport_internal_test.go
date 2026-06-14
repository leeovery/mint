package engine

// White-box proofs for the release wiring site `aiTransport` (task 2-3): the
// production transport must source BOTH its command and its per-attempt deadline
// from the release verb's per-key accessors (cfg.AICommandFor(VerbRelease) /
// cfg.TimeoutFor(VerbRelease)). The command is observable as the invoked argv; the
// deadline is observable as the presence/absence of a context deadline on the call
// the transport hands the runner — config's explicit 0 ("no deadline") must reach
// the transport as a deadline-free context, while a positive override must reach it
// as a context carrying a deadline. The FakeRunner discards the context, so these
// timeout proofs use a tiny deadline-recording runner instead.

import (
	"context"
	"io"
	"testing"
	"time"

	"mint/internal/config"
	"mint/internal/runner"
)

// deadlineRunner is a CommandRunner spy that records the argv and whether the
// context it was handed carries a deadline — exactly what the transport sets (or
// omits) via context.WithTimeout for a positive (or zero/no-deadline) timeout.
type deadlineRunner struct {
	name        string
	args        []string
	hadDeadline bool
}

func (d *deadlineRunner) RunWith(ctx context.Context, stdin io.Reader, name string, args ...string) (runner.Result, error) {
	d.name = name
	d.args = args
	_, d.hadDeadline = ctx.Deadline()
	if stdin != nil {
		_, _ = io.ReadAll(stdin)
	}
	return runner.Result{Stdout: "a non-empty body\n"}, nil
}

func (d *deadlineRunner) Run(ctx context.Context, name string, args ...string) (runner.Result, error) {
	return d.RunWith(ctx, nil, name, args...)
}

func (d *deadlineRunner) RunInteractive(context.Context, string, ...string) error { return nil }

func (d *deadlineRunner) RunInDir(context.Context, string, []string, string, ...string) (runner.Result, error) {
	return runner.Result{}, nil
}

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
		Timeout:   durationPtr(60 * time.Second),
	}

	spy := &deadlineRunner{}
	transport := aiTransport(ReleaseDeps{Runner: spy}, cfg)
	if _, err := transport.Generate(t.Context(), "prompt"); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if spy.name != "verbbot" {
		t.Errorf("invoked binary = %q, want the [release].ai_command override binary %q", spy.name, "verbbot")
	}
	wantArgs := []string{"run", "--json"}
	if len(spy.args) != len(wantArgs) || spy.args[0] != wantArgs[0] || spy.args[1] != wantArgs[1] {
		t.Errorf("invoked args = %v, want the [release].ai_command override args %v", spy.args, wantArgs)
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
		Timeout:   durationPtr(60 * time.Second),
	}

	spy := &deadlineRunner{}
	transport := aiTransport(ReleaseDeps{Runner: spy}, cfg)
	if _, err := transport.Generate(t.Context(), "prompt"); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if spy.hadDeadline {
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
		Timeout:   durationPtr(90 * time.Second),
	}

	spy := &deadlineRunner{}
	transport := aiTransport(ReleaseDeps{Runner: spy}, cfg)
	if _, err := transport.Generate(t.Context(), "prompt"); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if !spy.hadDeadline {
		t.Errorf("context carried no deadline; a positive resolved timeout must thread a per-attempt deadline through aiTransport")
	}
}

func durationPtr(d time.Duration) *time.Duration { return &d }
