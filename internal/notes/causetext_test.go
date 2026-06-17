package notes_test

import (
	"errors"
	"fmt"
	"testing"

	"mint/internal/ai"
	"mint/internal/notes"
)

// TestCauseText_KnownSentinels_DerivesConcisePhrase proves notes.CauseText maps each
// of the four known notes-failure sentinels to its exact concise phrase and reports
// known=true. The match is errors.Is-based, so it must hold whether the sentinel is
// passed bare or wrapped behind a %w chain (the engine's display-derivation seam).
func TestCauseText_KnownSentinels_DerivesConcisePhrase(t *testing.T) {
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

			// Bare sentinel.
			got, known := notes.CauseText(tc.sentinel)
			if !known {
				t.Fatalf("CauseText(%v) known = false, want true", tc.sentinel)
			}
			if got != tc.want {
				t.Errorf("CauseText(%v) = %q, want %q", tc.sentinel, got, tc.want)
			}

			// Wrapped behind a %w chain — errors.Is must still traverse it.
			wrapped := fmt.Errorf("generating notes: %w", tc.sentinel)
			gotWrapped, knownWrapped := notes.CauseText(wrapped)
			if !knownWrapped {
				t.Fatalf("CauseText(wrapped %v) known = false, want true", tc.sentinel)
			}
			if gotWrapped != tc.want {
				t.Errorf("CauseText(wrapped %v) = %q, want %q", tc.sentinel, gotWrapped, tc.want)
			}
		})
	}
}

// TestCauseText_UnmappedCause_ReportsNotKnown proves an error that wraps none of the
// four sentinels reports known=false (and an empty phrase) so the engine falls back to
// cause.Error().
func TestCauseText_UnmappedCause_ReportsNotKnown(t *testing.T) {
	t.Parallel()

	got, known := notes.CauseText(errors.New("boom"))
	if known {
		t.Errorf("CauseText(unmapped) known = true, want false")
	}
	if got != "" {
		t.Errorf("CauseText(unmapped) phrase = %q, want empty", got)
	}
}
