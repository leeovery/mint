package engine

// White-box proofs for failureMessage's notes/AI branch (Fix 2): the presenter-facing
// display Message for a notes/AI failure must collapse to ONE concise cause phrase
// derived (via notes.CauseText / errors.Is) from the sentinel the cause wraps — NOT the
// verbose nested %w chain rendered by cause.Error(). The %w chain itself is left intact
// for errors.Is/logs; only the display derivation changes. The branch must produce the
// identical phrase for BOTH chain shapes: the forward release abortError chain and
// regenerate's shorter "generating notes: %w" chain.

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/runner"
)

// resolveAbortError builds the forward-release abort chain for a sentinel exactly as
// the spine does: notes.ResolveFailure in abort mode wraps the failure through
// abortError ("notes generation failed (%s): %w"), the longest chain failureMessage must
// collapse. The git runner is never invoked in abort mode.
func resolveAbortError(t *testing.T, sentinel error) error {
	t.Helper()
	_, err := notes.ResolveFailure(t.Context(), runner.NewFakeRunner(), sentinel, "v1.0.0", config.Release{OnNotesFailure: "abort"})
	if err == nil {
		t.Fatalf("ResolveFailure returned nil error in abort mode for %v", sentinel)
	}
	return err
}

// TestFailureMessage_CollapsesForwardAbortChain proves the forward abortError chain
// collapses to the concise phrase: no nested %w chain, no leading stage label, no
// "failed".
func TestFailureMessage_CollapsesForwardAbortChain(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		sentinel error
		want     string
	}{
		{"timeout", ai.ErrTimeout, "AI timed out"},
		{"diff too large", notes.ErrDiffTooLarge, "diff too large"},
		{"missing tool", ai.ErrCommandMissing, "AI tool not installed"},
		{"empty after retry", ai.ErrGenerationFailed, "AI returned empty/invalid notes after retry"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cause := resolveAbortError(t, tc.sentinel)
			got := failureMessage(cause)

			if got != tc.want {
				t.Errorf("failureMessage = %q, want the concise phrase %q", got, tc.want)
			}
			if strings.Contains(got, ":") {
				t.Errorf("failureMessage = %q, want no nested %%w chain (no ':')", got)
			}
			if strings.Contains(got, "failed") {
				t.Errorf("failureMessage = %q, want no 'failed'", got)
			}
			if strings.HasPrefix(got, "notes") {
				t.Errorf("failureMessage = %q, want no leading stage label 'notes'", got)
			}
		})
	}
}

// TestFailureMessage_CollapsesRegenerateShortChain proves regenerate's shorter
// "generating notes: %w" chain (never routed through abortError) collapses to the
// IDENTICAL concise phrase — the derivation matches on the wrapped sentinel, not the
// chain shape.
func TestFailureMessage_CollapsesRegenerateShortChain(t *testing.T) {
	t.Parallel()

	cause := fmt.Errorf("generating notes: %w", ai.ErrGenerationFailed)
	got := failureMessage(cause)

	const want = "AI returned empty/invalid notes after retry"
	if got != want {
		t.Errorf("failureMessage = %q, want the concise phrase %q", got, want)
	}
	if strings.Contains(got, ":") {
		t.Errorf("failureMessage = %q, want no nested %%w chain (no ':')", got)
	}
	if strings.Contains(got, "failed") {
		t.Errorf("failureMessage = %q, want no 'failed'", got)
	}
	if strings.HasPrefix(got, "notes") {
		t.Errorf("failureMessage = %q, want no leading stage label 'notes'", got)
	}
}

// TestFailureMessage_LeavesMatchableChainIntact proves deriving the concise display
// phrase does NOT disturb the %w chain: errors.Is(cause, sentinel) still matches after
// the change.
func TestFailureMessage_LeavesMatchableChainIntact(t *testing.T) {
	t.Parallel()

	cause := resolveAbortError(t, ai.ErrGenerationFailed)
	_ = failureMessage(cause)

	if !errors.Is(cause, ai.ErrGenerationFailed) {
		t.Errorf("errors.Is(cause, ai.ErrGenerationFailed) = false after deriving the message, want it still to match")
	}
}

// TestFailureMessage_FallsBackToCauseErrorForNonAICause proves a non-AI cause (the
// shape resetAndAbort surfaces — a git record/push failure, none of the four sentinels)
// falls through to the defensive cause.Error() fallback rather than being misclassified.
func TestFailureMessage_FallsBackToCauseErrorForNonAICause(t *testing.T) {
	t.Parallel()

	cause := fmt.Errorf("batch rebuild: %w", errors.New("git reset failed: exit status 128"))
	got := failureMessage(cause)

	if got != cause.Error() {
		t.Errorf("failureMessage = %q, want the cause.Error() fallback %q", got, cause.Error())
	}
}
