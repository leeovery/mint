package commit

// White-box proofs for the commit wiring site `commitTransport` (task 2-4): the
// production transport must source BOTH its command and its per-attempt deadline from
// the commit verb's per-key accessors (cfg.AICommandFor(VerbCommit) /
// cfg.TimeoutFor(VerbCommit)). The command is observable as the invoked argv; the
// deadline is observable as the presence/absence of a context deadline on the call the
// transport hands the runner — config's explicit 0 ("no deadline") must reach the
// transport as a deadline-free context, while a positive override must reach it as a
// context carrying a deadline. The FakeRunner discards the context, so these timeout
// proofs use a tiny deadline-recording runner instead. This mirrors the release site's
// white-box aiTransport proofs (task 2-3).

import (
	"context"
	"io"
	"testing"
	"time"

	"mint/internal/config"
	"mint/internal/runner"
)

// deadlineCommitRunner is a CommandRunner spy that records the argv and whether the
// context it was handed carries a deadline — exactly what the transport sets (or omits)
// via context.WithTimeout for a positive (or zero/no-deadline) timeout.
type deadlineCommitRunner struct {
	name        string
	args        []string
	hadDeadline bool
}

func (d *deadlineCommitRunner) RunWith(ctx context.Context, stdin io.Reader, name string, args ...string) (runner.Result, error) {
	d.name = name
	d.args = args
	_, d.hadDeadline = ctx.Deadline()
	if stdin != nil {
		_, _ = io.ReadAll(stdin)
	}
	return runner.Result{Stdout: "a non-empty body\n"}, nil
}

func (d *deadlineCommitRunner) Run(ctx context.Context, name string, args ...string) (runner.Result, error) {
	return d.RunWith(ctx, nil, name, args...)
}

func (d *deadlineCommitRunner) RunInteractive(context.Context, string, ...string) error { return nil }

func (d *deadlineCommitRunner) RunInDir(context.Context, string, []string, string, ...string) (runner.Result, error) {
	return runner.Result{}, nil
}

func durationPtr(d time.Duration) *time.Duration { return &d }

// TestCommitTransport_SourcesCommandFromCommitAccessor proves commitTransport threads
// cfg.AICommandFor(VerbCommit): a [commit].ai_command override is the binary+args the
// production transport invokes — not the bare shared top-level command.
func TestCommitTransport_SourcesCommandFromCommitAccessor(t *testing.T) {
	t.Parallel()

	shared := "shared-bot"
	override := "verbbot run --json"
	cfg := config.Config{
		AICommand: shared,
		Commit:    config.Commit{AICommand: &override},
		Timeout:   durationPtr(60 * time.Second),
	}

	spy := &deadlineCommitRunner{}
	transport := commitTransport(Deps{Runner: spy}, cfg)
	if _, err := transport.Generate(t.Context(), "prompt"); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if spy.name != "verbbot" {
		t.Errorf("invoked binary = %q, want the [commit].ai_command override binary %q", spy.name, "verbbot")
	}
	wantArgs := []string{"run", "--json"}
	if len(spy.args) != len(wantArgs) || spy.args[0] != wantArgs[0] || spy.args[1] != wantArgs[1] {
		t.Errorf("invoked args = %v, want the [commit].ai_command override args %v", spy.args, wantArgs)
	}
}

// TestCommitTransport_PositiveTimeoutThreadsDeadline proves the timeout is sourced from
// cfg.TimeoutFor(VerbCommit): a [commit].timeout override of a positive value reaches the
// transport as a per-attempt deadline, so the context the transport hands the runner
// carries a deadline. If the wiring site forgot the field (zero-by-omission) NewTransport
// would panic on the nil pointer — so reaching Generate at all already proves the field
// was threaded; the deadline proves it was threaded from the accessor.
func TestCommitTransport_PositiveTimeoutThreadsDeadline(t *testing.T) {
	t.Parallel()

	override := 90 * time.Second
	cfg := config.Config{
		AICommand: "claude",
		Commit:    config.Commit{Timeout: &override},
		Timeout:   durationPtr(60 * time.Second),
	}

	spy := &deadlineCommitRunner{}
	transport := commitTransport(Deps{Runner: spy}, cfg)
	if _, err := transport.Generate(t.Context(), "prompt"); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if !spy.hadDeadline {
		t.Errorf("context carried no deadline; a [commit].timeout override must thread a per-attempt deadline through commitTransport")
	}
}

// TestCommitTransport_ExplicitZeroTimeoutThreadsNoDeadline proves a [commit].timeout of
// explicit 0 ("no deadline") reaches the transport, so the context the transport hands
// the runner carries NO deadline. This is reachable ONLY by sourcing the accessor's
// *time.Duration directly (never zero-by-omission, which would panic at NewTransport).
func TestCommitTransport_ExplicitZeroTimeoutThreadsNoDeadline(t *testing.T) {
	t.Parallel()

	zero := time.Duration(0)
	cfg := config.Config{
		AICommand: "claude",
		Commit:    config.Commit{Timeout: &zero},
		Timeout:   durationPtr(60 * time.Second),
	}

	spy := &deadlineCommitRunner{}
	transport := commitTransport(Deps{Runner: spy}, cfg)
	if _, err := transport.Generate(t.Context(), "prompt"); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if spy.hadDeadline {
		t.Errorf("context carried a deadline; a [commit].timeout of explicit 0 must thread NO deadline through commitTransport")
	}
}
