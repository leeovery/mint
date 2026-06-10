package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
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

	return translateRun(cmd.Run(), &stdout, &stderr, cmd, name)
}

// RunInDir executes name with args from working directory dir, with env (a slice
// of "KEY=VALUE" entries) layered on top of the inherited environment. It is the
// seam hooks use: each entry runs as `sh -c "<entry>"` from the repo root with
// mint's MINT_* variables injected. Stdout/stderr are captured separately and the
// exit is translated exactly as RunWith does (missing binary → ErrCommandNotFound;
// non-zero exit → populated Result + non-nil error; clean → populated Result + nil
// error).
func (r *ExecRunner) RunInDir(ctx context.Context, dir string, env []string, name string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	// Layer the extra entries on top of the inherited environment so MINT_* is
	// injected without dropping the host's PATH etc.
	cmd.Env = append(os.Environ(), env...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	return translateRun(cmd.Run(), &stdout, &stderr, cmd, name)
}

// translateRun converts a finished exec.Cmd into the seam's Result/error contract,
// shared by RunWith and RunInDir so both report missing binaries, non-zero exits,
// and clean runs identically.
func translateRun(runErr error, stdout, stderr *bytes.Buffer, cmd *exec.Cmd, name string) (Result, error) {
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

// RunInteractive launches name with args wired directly to the real terminal so
// an interactive child (the user's $EDITOR) owns stdin/stdout/stderr for its
// session. Unlike Run/RunWith it captures NO pipes — the editor draws to the
// terminal itself — so it returns only an error. A missing binary is translated
// to ErrCommandNotFound exactly as Run does, so the editor launcher can branch
// "no editor to launch" apart from "the editor ran and failed".
func (r *ExecRunner) RunInteractive(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)

	// Wire the real terminal: the editor owns it for the duration of the session.
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	runErr := cmd.Run()
	if runErr == nil {
		return nil
	}

	// A missing editor surfaces before the process ever starts; report it
	// distinctly so the launcher can return to the gate rather than abort.
	if errors.Is(runErr, exec.ErrNotFound) {
		return fmt.Errorf("running %q: %w", name, ErrCommandNotFound)
	}

	// Any other failure (non-zero exit, context cancellation) propagates wrapped.
	return fmt.Errorf("running %q: %w", name, runErr)
}
