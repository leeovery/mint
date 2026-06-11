package engine_test

import (
	"errors"
	"testing"

	"mint/internal/engine"
	"mint/internal/runner"
)

// TestRemoteURL verifies the exported remote-URL reader's contract: it reads
// `git remote get-url origin` through the runner seam, returns "" on a non-zero
// exit (no `origin` remote — the publisher resolver treats empty as unresolved
// and downgrades rather than fails), and TrimSpaces the stdout otherwise. This is
// the single owned reader both the forward engine path and the cmd regenerate path
// now share, so its behaviour is pinned here.
func TestRemoteURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		result   runner.Result
		runErr   error
		expected string
	}{
		{
			name:     "trims trailing newline from stdout",
			result:   runner.Result{Stdout: "https://github.com/acme/widget.git\n"},
			expected: "https://github.com/acme/widget.git",
		},
		{
			name:     "non-zero exit yields empty string",
			result:   runner.Result{ExitCode: 1},
			runErr:   errors.New("exit status 1"),
			expected: "",
		},
		{
			name:     "empty stdout yields empty string",
			result:   runner.Result{Stdout: "   \n"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := runner.NewFakeRunner()
			f.Seed("git", tt.result, tt.runErr)

			got := engine.RemoteURL(t.Context(), f)
			if got != tt.expected {
				t.Errorf("RemoteURL() = %q, want %q", got, tt.expected)
			}
		})
	}
}
