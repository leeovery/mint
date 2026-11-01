//go:build unix

package runner_test

import (
	"context"
	"errors"
	"testing"

	"mint/internal/runner"
)

// A Ctrl-C reaches the child as well as mint, so the child can die of SIGINT
// while ctx is still live. A child that kills itself with SIGINT stands in for
// that race: the runner must report the user's cancel, never a verdict.
func TestExecRunner_ChildDiedOfSIGINTIsACancel(t *testing.T) {
	t.Parallel()

	r := runner.NewExecRunner()
	runs := map[string]func() (runner.Result, error){
		"Run": func() (runner.Result, error) {
			return r.Run(t.Context(), "sh", "-c", "kill -INT $$")
		},
		"RunInDir": func() (runner.Result, error) {
			return r.RunInDir(t.Context(), t.TempDir(), nil, "sh", "-c", "kill -INT $$")
		},
	}

	for name, run := range runs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := run()

			if !errors.Is(err, context.Canceled) {
				t.Errorf("error = %v, want it to match context.Canceled", err)
			}
		})
	}
}

// Only SIGINT is the user's cancel: a child killed by any other signal still
// reports as an ordinary failed run.
func TestExecRunner_ChildDiedOfOtherSignalIsNotACancel(t *testing.T) {
	t.Parallel()

	r := runner.NewExecRunner()

	res, err := r.Run(t.Context(), "sh", "-c", "kill -TERM $$")

	if err == nil {
		t.Fatal("Run returned nil error for a child killed by SIGTERM, want non-nil")
	}
	if errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, a SIGTERM death must NOT match context.Canceled", err)
	}
	if res.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1 for a signalled child", res.ExitCode)
	}
}
