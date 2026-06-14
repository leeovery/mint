package runner

import (
	"context"
	"io"
	"time"
)

// DeadlineRecordingRunner is a CommandRunner spy that records the invoked argv and
// whether the context it was handed carries a deadline — exactly what a transport
// sets (or omits) via context.WithTimeout for a positive (or zero/no-deadline)
// timeout. It exists because FakeRunner discards the context and so cannot observe
// the timeout-vs-no-deadline behaviour the per-verb transport-wiring proofs assert.
// It ships in this production file (alongside FakeRunner) rather than a _test.go
// helper so the aitransport, engine, and commit test packages can all share it: Go
// test helpers in _test.go files are not visible across packages.
type DeadlineRecordingRunner struct {
	name        string
	args        []string
	hadDeadline bool
}

// Compile-time assertion that DeadlineRecordingRunner satisfies the seam.
var _ CommandRunner = (*DeadlineRecordingRunner)(nil)

// Name returns the binary recorded on the most recent call.
func (d *DeadlineRecordingRunner) Name() string { return d.name }

// Args returns the args recorded on the most recent call.
func (d *DeadlineRecordingRunner) Args() []string { return d.args }

// HadDeadline reports whether the context handed to the most recent call carried a
// deadline.
func (d *DeadlineRecordingRunner) HadDeadline() bool { return d.hadDeadline }

// RunWith records the argv and the context's deadline presence, drains any stdin,
// and returns a fixed non-empty body so the calling transport treats the attempt as
// a successful generation.
func (d *DeadlineRecordingRunner) RunWith(ctx context.Context, stdin io.Reader, name string, args ...string) (Result, error) {
	d.name = name
	d.args = args
	_, d.hadDeadline = ctx.Deadline()
	if stdin != nil {
		_, _ = io.ReadAll(stdin)
	}
	return Result{Stdout: "a non-empty body\n"}, nil
}

// Run delegates to RunWith with no stdin.
func (d *DeadlineRecordingRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	return d.RunWith(ctx, nil, name, args...)
}

// RunInteractive is a no-op: the deadline proofs never launch an editor.
func (d *DeadlineRecordingRunner) RunInteractive(context.Context, string, ...string) error {
	return nil
}

// RunInDir is a no-op: the deadline proofs never run a directory-scoped command.
func (d *DeadlineRecordingRunner) RunInDir(context.Context, string, []string, string, ...string) (Result, error) {
	return Result{}, nil
}

// DurationPtr returns a pointer to the given duration. It is the shared *time.Duration
// helper the config-pointer fields (cfg.Timeout, [release]/[commit].timeout) need at their
// test call sites, consolidating the copy the deadline proofs independently re-authored.
func DurationPtr(dur time.Duration) *time.Duration { return &dur }
