package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

// ExecRunner is the production CommandRunner. It runs commands via
// os/exec.CommandContext so cancellation/timeout flows through ctx, captures
// stdout and stderr into separate buffers, and derives ExitCode from the
// process exit status.
type ExecRunner struct{}

// NewExecRunner returns an ExecRunner. It is stateless; the constructor exists
// so call sites depend on the CommandRunner seam rather than a bare struct
// literal, and so future configuration has a home without churning callers.
func NewExecRunner() *ExecRunner {
	return &ExecRunner{}
}

// Compile-time assertion that ExecRunner satisfies the seam.
var _ CommandRunner = (*ExecRunner)(nil)

// Run executes name with args and no stdin.
func (r *ExecRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	return r.RunWith(ctx, nil, name, args...)
}

// RunWith executes name with args, piping stdin (when non-nil) into the
// command's standard input. It captures stdout/stderr separately and translates
// the process exit into Result.ExitCode plus an error per the seam's contract:
//   - missing binary    -> error wrapping ErrCommandNotFound
//   - non-zero exit      -> populated Result + non-nil error (Stderr/ExitCode readable)
//   - clean (exit 0)     -> populated Result + nil error
func (r *ExecRunner) RunWith(ctx context.Context, stdin io.Reader, name string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if stdin != nil {
		cmd.Stdin = stdin
	}

	runErr := cmd.Run()

	res := Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: cmd.ProcessState.ExitCode(),
	}

	if runErr == nil {
		return res, nil
	}

	// A missing binary surfaces before the process ever starts, so there is no
	// exit status; report it distinctly so callers can tell "not installed" apart
	// from "ran and failed".
	if errors.Is(runErr, exec.ErrNotFound) {
		return Result{}, fmt.Errorf("running %q: %w", name, ErrCommandNotFound)
	}

	// A non-zero exit keeps the populated Result so callers can inspect
	// Stderr/ExitCode while still branching on the error.
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return res, fmt.Errorf("running %q: exited with code %d: %w", name, res.ExitCode, runErr)
	}

	// Any other failure (e.g. context cancellation, pipe setup) propagates wrapped.
	return res, fmt.Errorf("running %q: %w", name, runErr)
}
