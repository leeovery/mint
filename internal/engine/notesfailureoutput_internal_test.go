package engine

// White-box proofs for notesFailureOutput (Fix 1 step 3): the engine extraction helper
// that pulls claude's captured output off the *ai.GenerationError carrier and composes
// it for StageFailure.Output. It mirrors hookFailureOutput's PATTERN but NOT its field
// choice — claude's payload arrives on STDOUT (e.g. "Prompt is too long"), not stderr —
// and uses errors.As so it matches the carrier wherever it sits in the %w chain (inside
// abortError's forward chain or regenerate's shorter "generating notes: %w" chain).
//
// Composition rule (settled): trim each stream for the EMPTINESS check only (a
// whitespace-only stream counts as empty); include the non-empty streams stdout-first
// then stderr, joining two present streams with a single newline; keep the included
// content VERBATIM; trim only the composed result's TRAILING whitespace. Both empty → "".
// Non-carrier causes (timeout / command-missing / diff-too-large / any plain error) → "".

import (
	"errors"
	"fmt"
	"testing"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/runner"
)

// resolveAbortAround builds the forward-release abort chain around a *ai.GenerationError
// carrier exactly as the spine does: notes.ResolveFailure in abort mode wraps the carrier
// through abortError ("notes generation failed (%s): %w"), so the carrier sits behind the
// longest %w chain notesFailureOutput must traverse with errors.As. The git runner is
// never invoked in abort mode.
func resolveAbortAround(t *testing.T, carrier error) error {
	t.Helper()
	_, err := notes.ResolveFailure(t.Context(), runner.NewFakeRunner(), carrier, "v1.0.0", config.Release{OnNotesFailure: "abort"})
	if err == nil {
		t.Fatalf("ResolveFailure returned nil error in abort mode for %v", carrier)
	}
	return err
}

// TestNotesFailureOutput_ExtractsStdoutThroughAbortChain proves the helper finds the
// carrier inside abortError's forward chain (errors.As traversal) and reads STDOUT — not
// stderr — returning claude's message with trailing whitespace trimmed.
func TestNotesFailureOutput_ExtractsStdoutThroughAbortChain(t *testing.T) {
	t.Parallel()

	carrier := &ai.GenerationError{Stdout: "Prompt is too long\n", ExitCode: 1}
	cause := resolveAbortAround(t, carrier)

	got := notesFailureOutput(cause)
	const want = "Prompt is too long"
	if got != want {
		t.Errorf("notesFailureOutput = %q, want %q", got, want)
	}
}

// TestNotesFailureOutput_ExtractsThroughRegenerateShortChain proves the helper also
// matches the carrier inside regenerate's shorter "generating notes: %w" chain — the
// derivation keys on the carrier type via errors.As, not on the chain shape.
func TestNotesFailureOutput_ExtractsThroughRegenerateShortChain(t *testing.T) {
	t.Parallel()

	carrier := &ai.GenerationError{Stdout: "Prompt is too long\n", ExitCode: 1}
	cause := fmt.Errorf("generating notes: %w", carrier)

	got := notesFailureOutput(cause)
	const want = "Prompt is too long"
	if got != want {
		t.Errorf("notesFailureOutput = %q, want %q", got, want)
	}
}

// TestNotesFailureOutput_ComposesStdoutThenStderr proves both non-empty streams are
// included stdout-first, joined by a single newline.
func TestNotesFailureOutput_ComposesStdoutThenStderr(t *testing.T) {
	t.Parallel()

	carrier := &ai.GenerationError{Stdout: "out line", Stderr: "err line", ExitCode: 1}

	got := notesFailureOutput(carrier)
	const want = "out line\nerr line"
	if got != want {
		t.Errorf("notesFailureOutput = %q, want %q", got, want)
	}
}

// TestNotesFailureOutput_IncludesOnlyStdoutWhenStderrWhitespace proves a whitespace-only
// stderr counts as empty for the inclusion decision: only stdout is included, with no
// joining newline from the empty stream.
func TestNotesFailureOutput_IncludesOnlyStdoutWhenStderrWhitespace(t *testing.T) {
	t.Parallel()

	carrier := &ai.GenerationError{Stdout: "real stdout", Stderr: "   \n", ExitCode: 1}

	got := notesFailureOutput(carrier)
	const want = "real stdout"
	if got != want {
		t.Errorf("notesFailureOutput = %q, want %q", got, want)
	}
}

// TestNotesFailureOutput_IncludesOnlyStderrWhenStdoutWhitespace proves a whitespace-only
// stdout counts as empty: only stderr is included, with no leading newline.
func TestNotesFailureOutput_IncludesOnlyStderrWhenStdoutWhitespace(t *testing.T) {
	t.Parallel()

	carrier := &ai.GenerationError{Stdout: "\t\n", Stderr: "real stderr", ExitCode: 1}

	got := notesFailureOutput(carrier)
	const want = "real stderr"
	if got != want {
		t.Errorf("notesFailureOutput = %q, want %q", got, want)
	}
}

// TestNotesFailureOutput_EmptyWhenBothStreamsWhitespace proves both whitespace-only
// streams compose to "" so the ✗ line stands alone.
func TestNotesFailureOutput_EmptyWhenBothStreamsWhitespace(t *testing.T) {
	t.Parallel()

	carrier := &ai.GenerationError{Stdout: "  \n", Stderr: "\t\t\n", ExitCode: 1}

	got := notesFailureOutput(carrier)
	if got != "" {
		t.Errorf("notesFailureOutput = %q, want \"\" for two whitespace-only streams", got)
	}
}

// TestNotesFailureOutput_PreservesInteriorTrimsTrailing proves interior content
// (including a blank line) is preserved VERBATIM while only the composed result's
// TRAILING whitespace is trimmed.
func TestNotesFailureOutput_PreservesInteriorTrimsTrailing(t *testing.T) {
	t.Parallel()

	carrier := &ai.GenerationError{Stdout: "line1\n\nline2\n\n", ExitCode: 1}

	got := notesFailureOutput(carrier)
	const want = "line1\n\nline2"
	if got != want {
		t.Errorf("notesFailureOutput = %q, want %q (interior blank kept, trailing trimmed)", got, want)
	}
}

// TestNotesFailureOutput_EmptyForNonCarrierCause proves every non-carrier cause yields ""
// — the other sentinels carry no claude output, so the ✗ line stands alone.
func TestNotesFailureOutput_EmptyForNonCarrierCause(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		cause error
	}{
		{"timeout", ai.ErrTimeout},
		{"command missing", ai.ErrCommandMissing},
		{"diff too large", notes.ErrDiffTooLarge},
		{"plain error", errors.New("git reset failed: exit status 128")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := notesFailureOutput(tc.cause); got != "" {
				t.Errorf("notesFailureOutput(%v) = %q, want \"\"", tc.cause, got)
			}
		})
	}
}
