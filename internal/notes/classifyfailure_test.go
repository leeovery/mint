package notes_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"mint/internal/ai"
	"mint/internal/notes"
)

func TestClassifyFailure_EachSentinelThroughTheWrapChain_NamesItsCause(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		failure error
		want    notes.Cause
	}{
		{"no room for diff", notes.ErrNoRoomForDiff, notes.CauseNoRoomForDiff},
		{"merge too large", notes.ErrMergeTooLarge, notes.CauseMergeTooLarge},
		{"too large for the model", notes.ErrTooLargeForModel, notes.CauseTooLargeForModel},
		{"timeout", fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout), notes.CauseTimeout},
		{"missing tool", fmt.Errorf("%w: claude not found", ai.ErrCommandMissing), notes.CauseCommandMissing},
		{"generation failure", &ai.GenerationError{Stdout: "boom", ExitCode: 1}, notes.CauseGenerationFailed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, known := notes.ClassifyFailure(fmt.Errorf("generating notes: %w", tc.failure))
			if !known {
				t.Fatalf("ClassifyFailure(%v) known = false, want true", tc.failure)
			}
			if got != tc.want {
				t.Errorf("ClassifyFailure(%v) = %v, want %v", tc.failure, got, tc.want)
			}
		})
	}
}

func TestClassifyFailure_SizeSentinelOverTheCallsGenerationFailure_KeepsTheSizeCause(t *testing.T) {
	t.Parallel()

	genErr := &ai.GenerationError{Stdout: "prompt is too long", ExitCode: 1}
	cases := []struct {
		name     string
		sentinel error
		want     notes.Cause
	}{
		{"too large for the model", notes.ErrTooLargeForModel, notes.CauseTooLargeForModel},
		{"merge too large", notes.ErrMergeTooLarge, notes.CauseMergeTooLarge},
		{"no room for diff", notes.ErrNoRoomForDiff, notes.CauseNoRoomForDiff},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			failure := fmt.Errorf("%w: %w", tc.sentinel, genErr)
			got, known := notes.ClassifyFailure(failure)
			if !known || got != tc.want {
				t.Errorf("ClassifyFailure(%v) = (%v, %t), want (%v, true)", failure, got, known, tc.want)
			}
		})
	}
}

func TestClassifyFailure_ErrorNoSentinelMatches_IsNotANotesFailure(t *testing.T) {
	t.Parallel()

	for _, failure := range []error{
		errors.New("boom"),
		fmt.Errorf("generating notes: %w", notes.ErrDiffTooLarge),
		context.Canceled,
	} {
		if got, known := notes.ClassifyFailure(failure); known {
			t.Errorf("ClassifyFailure(%v) = (%v, true), want not known", failure, got)
		}
	}
}
