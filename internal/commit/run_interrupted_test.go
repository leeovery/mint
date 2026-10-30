package commit_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"mint/internal/commit"
	"mint/internal/presenter/presentertest"
)

// TestRun_InterruptedAICall_FailsTheMessageAsInterrupted proves a Ctrl-C during the
// AI call fails the message stage marked interrupted, so the rail says so, while its
// Message, plain's line, stays the error as ever.
func TestRun_InterruptedAICall_FailsTheMessageAsInterrupted(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	cancelled := fmt.Errorf("running %q: %w", "claude", context.Canceled)
	deps := newCommitDeps(rec, seedDiffThenCommit("diff --git a/x b/x\n+work"), &failTransport{err: cancelled}, t.TempDir())

	if err := commit.Run(context.Background(), deps); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run returned %v, want the cancel", err)
	}
	var failed []presentertest.Event
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageFailed {
			failed = append(failed, ev)
		}
	}
	if len(failed) != 1 {
		t.Fatalf("StageFailed events = %d, want 1", len(failed))
	}
	got := failed[0].StageFailed
	if got.Key != "message" || !got.Interrupted || got.Message == "interrupted" {
		t.Errorf("failure = %+v, want the message stage marked interrupted, its Message the error's", got)
	}
}
