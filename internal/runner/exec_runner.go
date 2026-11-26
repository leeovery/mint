package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

// ExecRunner is the production CommandRunner. It runs commands via
// os/exec.CommandContext so cancellation/timeout flows through ctx, captures
// stdout and stderr into separate buffers, and derives ExitCode from the
// process exit status.
type ExecRunner struct {
	// graceful is how a command whose context ends is stopped; nil kills it at once.
	graceful *gracefulStop
}

// NewExecRunner returns an ExecRunner that kills a command at once when its context
// ends, as exec.CommandContext does: what a short command (git, gh, the AI call)
// wants. The constructor exists so call sites depend on the CommandRunner seam
// rather than a bare struct literal.
func NewExecRunner() *ExecRunner {
	return &ExecRunner{}
}

// NewGracefulExecRunner returns an ExecRunner that stops a command whose context ends
// gracefully, as a project's hook needs: SIGTERM, then SIGKILL once grace has passed
// or mint is interrupted again, whichever comes first (see gracefulStop).
func NewGracefulExecRunner(grace time.Duration) *ExecRunner {
	return &ExecRunner{graceful: &gracefulStop{grace: grace, again: againSignals}}
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

	return translateRun(ctx, r.run(cmd), &stdout, &stderr, cmd, name)
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

	return translateRun(ctx, r.run(cmd), &stdout, &stderr, cmd, name)
}

// run runs cmd to its end, stopped as r stops a command whose context ends.
func (r *ExecRunner) run(cmd *exec.Cmd) error {
	if r.graceful == nil {
		return cmd.Run()
	}
	return r.graceful.run(cmd)
}

// translateRun converts a finished exec.Cmd into the seam's Result/error contract,
// shared by RunWith and RunInDir so both report missing binaries, non-zero exits,
// and clean runs identically.
func translateRun(ctx context.Context, runErr error, stdout, stderr *bytes.Buffer, cmd *exec.Cmd, name string) (Result, error) {
	res := Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		// ProcessState.ExitCode() is nil-safe: it returns -1 for an unstarted or
		// not-found process, so reading it unconditionally here (before the runErr
		// branches below) is not a latent nil-deref. The not-found branch discards
		// this Result so the -1 never escapes.
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

	// A context deadline/cancel stops the child, SIGKILLed at once or, by a graceful
	// runner, sent SIGTERM first, so cmd.Run() surfaces whatever the child's end was:
	// an *exec.ExitError ("signal: killed", or the exit it chose on its SIGTERM) that
	// hides the real cause — errors.Is(runErr, DeadlineExceeded) is FALSE on that path.
	// Inspect ctx.Err() first and wrap that cause so callers (the AI transport's
	// classifyFatal) can detect a timeout/cancel via errors.Is and avoid retrying a
	// hung call, however the child ended. This must precede the plain ExitError branch
	// below, otherwise a real timeout is misread as an ordinary non-zero exit.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return res, fmt.Errorf("running %q: %w", name, ctxErr)
	}

	// Only a graceful runner sets a WaitDelay, past which a child that exited 0 but
	// left something running with its output open has that output cut off. Its exit
	// status is its verdict, so the run succeeded.
	if errors.Is(runErr, exec.ErrWaitDelay) {
		return res, nil
	}

	// A non-zero exit keeps the populated Result so callers can inspect
	// Stderr/ExitCode while still branching on the error.
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		// Ctrl-C signals the whole foreground process group, so the child can die
		// of it before mint's signal handler has cancelled ctx. That death is the
		// user's cancel, not the command's verdict (a check reading it as "gh is
		// not authenticated" would be wrong).
		if diedOfInterrupt(exitErr.ProcessState) {
			return res, fmt.Errorf("running %q: interrupted: %w", name, context.Canceled)
		}
		return res, fmt.Errorf("running %q: exited with code %d: %w", name, res.ExitCode, runErr)
	}

	// Any other failure (e.g. pipe setup, a non-ExitError I/O error) propagates
	// wrapped. Context deadline/cancel is handled by the ctx.Err() branch above.
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

	runErr := r.run(cmd)
	if runErr == nil {
		return nil
	}

	// A missing editor surfaces before the process ever starts; report it
	// distinctly so the launcher can return to the gate rather than abort.
	if errors.Is(runErr, exec.ErrNotFound) {
		return fmt.Errorf("running %q: %w", name, ErrCommandNotFound)
	}

	// A child that never started, with ctx still live, is not an exit: the launcher
	// tells it apart from a quit.
	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) && ctx.Err() == nil {
		return fmt.Errorf("running %q: %w: %w", name, ErrNotStarted, runErr)
	}

	// Any other failure (non-zero exit, context cancellation) propagates wrapped.
	return fmt.Errorf("running %q: %w", name, runErr)
}
