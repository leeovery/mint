package runner_test

import (
	"errors"
	"strings"
	"testing"

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
