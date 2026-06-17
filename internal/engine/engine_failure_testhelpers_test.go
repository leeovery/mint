package engine

// Shared white-box test helpers for the notes/AI failure surface (the abort-chain
// wrapping shape and the Acceptance Criterion #2 concise-phrase rule). These were each
// authored three/two times across the notes-failure internal tests; consolidating them
// keeps the magic literals ("v1.0.0", OnNotesFailure: "abort") and the concise-phrase
// rule in exactly one place so a ResolveFailure signature change or a tightening of the
// rule edits one site, not several.

import (
	"strings"
	"testing"

	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/runner"
)

// wrapNotesAbort builds the forward-release abort chain around a cause exactly as the
// spine does: notes.ResolveFailure in abort mode wraps the cause through abortError
// ("notes generation failed (%s): %w"), so a *ai.GenerationError carrier sits behind the
// longest %w chain the notes-failure surface must traverse with errors.As. The git runner
// is never invoked in abort mode.
func wrapNotesAbort(t *testing.T, cause error) error {
	t.Helper()
	_, err := notes.ResolveFailure(t.Context(), runner.NewFakeRunner(), cause, "v1.0.0", config.Release{OnNotesFailure: "abort"})
	if err == nil {
		t.Fatalf("ResolveFailure returned nil error in abort mode for %v", cause)
	}
	return err
}

// assertConcisePhrase pins Acceptance Criterion #2: a notes/AI display Message collapses
// to ONE concise cause phrase — no nested %w chain (no ':'), no "failed", and no leading
// stage label ("notes").
func assertConcisePhrase(t *testing.T, got string) {
	t.Helper()
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
