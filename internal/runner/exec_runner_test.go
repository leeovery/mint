package runner_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mint/internal/runner"
)

func TestExecRunner_Run_ReturnsStdout(t *testing.T) {
	t.Parallel()

	r := runner.NewExecRunner()

	res, err := r.Run(t.Context(), "printf", "hello")

	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if res.Stdout != "hello" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "hello")
	}
	if res.Stderr != "" {
		t.Errorf("Stderr = %q, want empty", res.Stderr)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
}

func TestExecRunner_Run_CapturesStderrSeparately(t *testing.T) {
	t.Parallel()

	r := runner.NewExecRunner()

	// sh writes "out" to stdout and "err" to stderr; the two streams must not mix.
	res, err := r.Run(t.Context(), "sh", "-c", "printf out; printf err 1>&2")

	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if res.Stdout != "out" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "out")
	}
	if res.Stderr != "err" {
		t.Errorf("Stderr = %q, want %q", res.Stderr, "err")
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
}

func TestExecRunner_Run_NonZeroExit(t *testing.T) {
	t.Parallel()

	r := runner.NewExecRunner()

	// Convention: non-zero exit returns a populated Result WITH a non-nil error so
	// callers can both branch on err and still read Stderr/ExitCode off the Result.
	res, err := r.Run(t.Context(), "sh", "-c", "printf boom 1>&2; exit 3")

	if err == nil {
		t.Fatal("Run returned nil error for a non-zero exit, want non-nil")
	}
	if res.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", res.ExitCode)
	}
	if res.Stderr != "boom" {
		t.Errorf("Stderr = %q, want %q", res.Stderr, "boom")
	}
	if res.Stdout != "" {
		t.Errorf("Stdout = %q, want empty", res.Stdout)
	}
}

func TestExecRunner_Run_DeadlineKillMatchesDeadlineExceeded(t *testing.T) {
	t.Parallel()

	r := runner.NewExecRunner()

	// A real context-deadline kill: exec.CommandContext SIGKILLs the child when the
	// deadline expires, so cmd.Run() returns an *exec.ExitError ("signal: killed"),
	// NOT context.DeadlineExceeded. The runner must inspect ctx.Err() and wrap the
	// cause so callers (the AI transport's classifyFatal) can detect the timeout via
	// errors.Is — otherwise a production ~60s timeout is misclassified as bad content
	// and retried. This drives the production path with a REAL kill, not an injected
	// DeadlineExceeded wrapper.
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	_, err := r.Run(ctx, "sleep", "5")

	if err == nil {
		t.Fatal("Run returned nil error for a deadline kill, want non-nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want it to match context.DeadlineExceeded", err)
	}
	// A timeout is not a missing binary; keep the two distinguishable.
	if errors.Is(err, runner.ErrCommandNotFound) {
		t.Errorf("error = %v, a deadline kill must NOT match ErrCommandNotFound", err)
	}
}

func TestExecRunner_Run_NonZeroExitDoesNotMatchDeadlineExceeded(t *testing.T) {
	t.Parallel()

	r := runner.NewExecRunner()

	// Guard the inverse: a normal non-zero exit (the deadline never fired) must stay a
	// plain command failure and must NOT be misread as a timeout, or every failing
	// command would be treated as a non-retried timeout.
	_, err := r.Run(t.Context(), "sh", "-c", "exit 1")

	if err == nil {
		t.Fatal("Run returned nil error for a non-zero exit, want non-nil")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, a normal non-zero exit must NOT match context.DeadlineExceeded", err)
	}
	if errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, a normal non-zero exit must NOT match context.Canceled", err)
	}
}

func TestExecRunner_Run_CommandNotFound(t *testing.T) {
	t.Parallel()

	r := runner.NewExecRunner()

	_, err := r.Run(t.Context(), "mint-no-such-binary-xyz")

	if err == nil {
		t.Fatal("Run returned nil error for a missing binary, want non-nil")
	}
	// Missing binary must be distinguishable from a ran-and-failed command (the gh
	// gate in 1-8 relies on telling "missing" apart from "exited non-zero").
	if !errors.Is(err, runner.ErrCommandNotFound) {
		t.Errorf("error = %v, want it to match ErrCommandNotFound", err)
	}
}

func TestExecRunner_Run_NonZeroExitIsNotCommandNotFound(t *testing.T) {
	t.Parallel()

	r := runner.NewExecRunner()

	_, err := r.Run(t.Context(), "sh", "-c", "exit 1")

	if err == nil {
		t.Fatal("Run returned nil error for a non-zero exit, want non-nil")
	}
	if errors.Is(err, runner.ErrCommandNotFound) {
		t.Errorf("error = %v, a ran-and-failed command must NOT match ErrCommandNotFound", err)
	}
}

func TestExecRunner_RunInteractive_SucceedsForRealBinary(t *testing.T) {
	t.Parallel()

	r := runner.NewExecRunner()

	// `true` exits 0 without producing output; RunInteractive wires the real
	// stdio (no captured pipes) and must report a clean run with a nil error.
	if err := r.RunInteractive(t.Context(), "true"); err != nil {
		t.Fatalf("RunInteractive returned unexpected error: %v", err)
	}
}

func TestExecRunner_RunInteractive_CommandNotFound(t *testing.T) {
	t.Parallel()

	r := runner.NewExecRunner()

	err := r.RunInteractive(t.Context(), "mint-no-such-editor-xyz")

	if err == nil {
		t.Fatal("RunInteractive returned nil error for a missing editor, want non-nil")
	}
	// A missing editor must be distinguishable from a launched-but-failed one so
	// the editor launcher can return to the gate instead of aborting.
	if !errors.Is(err, runner.ErrCommandNotFound) {
		t.Errorf("error = %v, want it to match ErrCommandNotFound", err)
	}
}

func TestExecRunner_RunInteractive_NonZeroExitIsNotCommandNotFound(t *testing.T) {
	t.Parallel()

	r := runner.NewExecRunner()

	err := r.RunInteractive(t.Context(), "false")

	if err == nil {
		t.Fatal("RunInteractive returned nil error for a non-zero exit, want non-nil")
	}
	if errors.Is(err, runner.ErrCommandNotFound) {
		t.Errorf("error = %v, a launched-but-failed editor must NOT match ErrCommandNotFound", err)
	}
}

func TestExecRunner_RunInDir_RunsInWorkingDirectory(t *testing.T) {
	t.Parallel()

	r := runner.NewExecRunner()

	// A hook runs from the repo root; RunInDir sets cmd.Dir so `pwd` must report the
	// directory we passed, not the test process's cwd. t.TempDir resolves symlinks
	// on some platforms (macOS /tmp), so compare against the evaluated path.
	dir := t.TempDir()
	wantDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", dir, err)
	}

	res, err := r.RunInDir(t.Context(), dir, nil, "pwd")

	if err != nil {
		t.Fatalf("RunInDir returned unexpected error: %v", err)
	}
	if got := strings.TrimSpace(res.Stdout); got != wantDir {
		t.Errorf("pwd = %q, want %q", got, wantDir)
	}
}

func TestExecRunner_RunInDir_InjectsEnvOnTopOfInherited(t *testing.T) {
	// No t.Parallel(): t.Setenv mutates process env and is incompatible with
	// parallel tests.
	r := runner.NewExecRunner()

	// The runner inherits the host env and layers the extra entries on top. Set a
	// real env var in the test process and assert BOTH it and an injected MINT_*
	// entry reach the child, proving the inherited env is preserved, not replaced.
	t.Setenv("MINT_RUNNER_INHERIT_PROBE", "from-host")

	res, err := r.RunInDir(t.Context(), t.TempDir(), []string{"MINT_NEW_VERSION=1.4.0"},
		"sh", "-c", "printf '%s|%s' \"$MINT_RUNNER_INHERIT_PROBE\" \"$MINT_NEW_VERSION\"")

	if err != nil {
		t.Fatalf("RunInDir returned unexpected error: %v", err)
	}
	if res.Stdout != "from-host|1.4.0" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "from-host|1.4.0")
	}
}

func TestExecRunner_RunInDir_CapturesStderrSeparately(t *testing.T) {
	t.Parallel()

	r := runner.NewExecRunner()

	res, err := r.RunInDir(t.Context(), t.TempDir(), nil, "sh", "-c", "printf out; printf err 1>&2")

	if err != nil {
		t.Fatalf("RunInDir returned unexpected error: %v", err)
	}
	if res.Stdout != "out" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "out")
	}
	if res.Stderr != "err" {
		t.Errorf("Stderr = %q, want %q", res.Stderr, "err")
	}
}

func TestExecRunner_RunInDir_NonZeroExit(t *testing.T) {
	t.Parallel()

	r := runner.NewExecRunner()

	// Same contract as Run: a non-zero exit returns a populated Result WITH a
	// non-nil error so a hook's failing entry stays inspectable (stderr/exit).
	res, err := r.RunInDir(t.Context(), t.TempDir(), nil, "sh", "-c", "printf boom 1>&2; exit 3")

	if err == nil {
		t.Fatal("RunInDir returned nil error for a non-zero exit, want non-nil")
	}
	if res.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", res.ExitCode)
	}
	if res.Stderr != "boom" {
		t.Errorf("Stderr = %q, want %q", res.Stderr, "boom")
	}
	if errors.Is(err, runner.ErrCommandNotFound) {
		t.Errorf("error = %v, a ran-and-failed command must NOT match ErrCommandNotFound", err)
	}
}

func TestExecRunner_RunInDir_CommandNotFound(t *testing.T) {
	t.Parallel()

	r := runner.NewExecRunner()

	_, err := r.RunInDir(t.Context(), t.TempDir(), nil, "mint-no-such-binary-xyz")

	if err == nil {
		t.Fatal("RunInDir returned nil error for a missing binary, want non-nil")
	}
	if !errors.Is(err, runner.ErrCommandNotFound) {
		t.Errorf("error = %v, want it to match ErrCommandNotFound", err)
	}
}

func TestExecRunner_RunWith_PipesStdin(t *testing.T) {
	t.Parallel()

	r := runner.NewExecRunner()

	// Stage 4 pipes the AI prompt to `claude -p` on stdin and reads the body off
	// stdout; cat echoes stdin straight back so we exercise that whole path.
	res, err := r.RunWith(t.Context(), strings.NewReader("prompt-body"), "cat")

	if err != nil {
		t.Fatalf("RunWith returned unexpected error: %v", err)
	}
	if res.Stdout != "prompt-body" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "prompt-body")
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
}
