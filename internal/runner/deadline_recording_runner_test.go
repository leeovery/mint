package runner_test

import (
	"context"
	"testing"
	"time"

	"mint/internal/runner"
)

// TestDeadlineRecordingRunner_RecordsArgvAndDeadlinePresence proves the shared spy
// records the invoked argv, reports HadDeadline per the context it was handed (true
// for a deadline-bearing context, false for a plain one), and always returns the
// fixed non-empty body — the exact behaviour the three former per-package copies
// relied on for the timeout-vs-no-deadline transport proofs.
func TestDeadlineRecordingRunner_RecordsArgvAndDeadlinePresence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		withDead bool
		wantDead bool
	}{
		{name: "deadline-bearing context reports HadDeadline true", withDead: true, wantDead: true},
		{name: "plain context reports HadDeadline false", withDead: false, wantDead: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			if tt.withDead {
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, time.Now().Add(time.Minute))
				defer cancel()
			}

			spy := &runner.DeadlineRecordingRunner{}
			res, err := spy.Run(ctx, "bot", "run", "--json")
			if err != nil {
				t.Fatalf("Run returned unexpected error: %v", err)
			}

			if res.Stdout != "a non-empty body\n" {
				t.Errorf("Stdout = %q, want the fixed non-empty body %q", res.Stdout, "a non-empty body\n")
			}
			if spy.Name() != "bot" {
				t.Errorf("Name() = %q, want %q", spy.Name(), "bot")
			}
			wantArgs := []string{"run", "--json"}
			if len(spy.Args()) != len(wantArgs) || spy.Args()[0] != wantArgs[0] || spy.Args()[1] != wantArgs[1] {
				t.Errorf("Args() = %v, want %v", spy.Args(), wantArgs)
			}
			if spy.HadDeadline() != tt.wantDead {
				t.Errorf("HadDeadline() = %v, want %v", spy.HadDeadline(), tt.wantDead)
			}
		})
	}
}

// TestDurationPtr proves the shared pointer helper returns a pointer to the given
// duration — the one-line helper the three test files independently re-authored.
func TestDurationPtr(t *testing.T) {
	t.Parallel()

	d := 90 * time.Second
	got := runner.DurationPtr(d)
	if got == nil {
		t.Fatal("DurationPtr returned nil")
	}
	if *got != d {
		t.Errorf("*DurationPtr(%v) = %v, want %v", d, *got, d)
	}
}
