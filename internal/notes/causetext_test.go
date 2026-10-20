package notes_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"mint/internal/ai"
	"mint/internal/notes"
)

func TestCauseText_SinglePassFailure_NamesTheCauseWithNoLocation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		cause error
		want  string
	}{
		{"output on stdout", &ai.GenerationError{Stdout: "Not logged in · Please run /login", ExitCode: 1}, "AI call failed"},
		{"output on stderr only", &ai.GenerationError{Stderr: "error: rate limited\n", ExitCode: 1}, "AI call failed"},
		{"whitespace on both streams", &ai.GenerationError{Stdout: " \n\t", Stderr: "\n  \n"}, "AI returned empty notes"},
		{"both streams empty", &ai.GenerationError{}, "AI returned empty notes"},
		{"generation failure with no carrier", ai.ErrGenerationFailed, "AI returned empty notes"},
		{"timeout", fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout), "AI timed out"},
		{"missing tool", fmt.Errorf("%w: claude not found", ai.ErrCommandMissing), "AI tool not installed"},
		{"no room for diff", notes.ErrNoRoomForDiff, "no room for the diff under max_prompt_bytes"},
		{"merge too large", notes.ErrMergeTooLarge, "part notes too large to merge"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, failure := range []error{tc.cause, fmt.Errorf("generating notes: %w", tc.cause)} {
				got, known := notes.CauseText(failure)
				if !known {
					t.Fatalf("CauseText(%v) known = false, want true", failure)
				}
				if got != tc.want {
					t.Errorf("CauseText(%v) = %q, want %q", failure, got, tc.want)
				}
			}
		})
	}
}

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

func TestCauseText_NoOldPhraseForAnyGenerationFailure(t *testing.T) {
	t.Parallel()

	for _, cause := range []error{
		ai.ErrGenerationFailed,
		&ai.GenerationError{Stdout: "Prompt is too long", ExitCode: 1},
		&ai.GenerationError{},
	} {
		if got, _ := notes.CauseText(cause); got == "AI returned empty/invalid notes after retry" {
			t.Errorf("CauseText(%v) = the retired phrase %q", cause, got)
		}
	}
}

// failingPartGate fails part's chunk call with cause and lets every other call
// through.
func failingPartGate(part int, cause error) func(int) error {
	return func(p int) error {
		if p == part {
			return cause
		}
		return nil
	}
}

func TestCauseText_ChunkCallFails_EndsWithItsPart(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		cause error
		want  string
	}{
		{"with output", &ai.GenerationError{Stdout: "API Error: 529 overloaded", ExitCode: 1}, "AI call failed (part 3 of 6)"},
		{"by timeout", fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout), "AI timed out (part 3 of 6)"},
		{"with empty notes", &ai.GenerationError{Stderr: "  \n"}, "AI returned empty notes (part 3 of 6)"},
		{"by a missing binary", fmt.Errorf("%w: claude not found", ai.ErrCommandMissing), "AI tool not installed (part 3 of 6)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := capOneFilePerChunk(t, tenKBFiles(6))
			wt := &waveTransport{mergeBody: "merged", gate: failingPartGate(3, tc.cause)}

			err := s.generateErr(t, wt)

			if got, _ := notes.CauseText(err); got != tc.want {
				t.Errorf("CauseText = %q, want %q", got, tc.want)
			}
			if !errors.Is(err, tc.cause) {
				t.Errorf("error = %v, want it to carry the part's cause", err)
			}
		})
	}
}

// failingPieceTransport fails the chunk call carrying piece 2 of a file's pieces,
// recording the part its framing names.
type failingPieceTransport struct {
	chunkTransport
	failedPart string
}

func (ft *failingPieceTransport) Generate(ctx context.Context, prompt string) (string, error) {
	if piece, _, ok := pieceOf(prompt); ok && piece == 2 {
		ft.failedPart = partFraming.FindString(prompt)
		return "", &ai.GenerationError{Stdout: "API Error: 500", ExitCode: 1}
	}
	return ft.chunkTransport.Generate(ctx, prompt)
}

func TestCauseText_PieceCallFails_EndsWithItsOwnPart(t *testing.T) {
	t.Parallel()

	ft := &failingPieceTransport{chunkTransport: chunkTransport{mergeBody: "merged"}}
	gen, cfg := piecesGeneration(t, ft, []pieceFile{
		{"alpha/a.go", fileDiff("alpha/a.go", 10_000)},
		{"big/huge.go", hunkedFileDiff("big/huge.go", 12, 5_000)},
	})

	_, err := gen.Generate(t.Context(), "v1.0.0", cfg)

	if ft.failedPart == "" || strings.HasPrefix(ft.failedPart, "part 2 of") {
		t.Fatalf("fixture: want piece 2's call made under a part number other than 2, got %q (err %v)", ft.failedPart, err)
	}
	want := "AI call failed (" + ft.failedPart + ")"
	if got, _ := notes.CauseText(err); got != want {
		t.Errorf("CauseText = %q, want %q", got, want)
	}
}

func TestCauseText_MergeCallFails_EndsWithMerging(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		cause error
		want  string
	}{
		{"with output", &ai.GenerationError{Stdout: "Prompt is too long", ExitCode: 1}, "AI call failed (merging)"},
		{"by timeout", fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout), "AI timed out (merging)"},
		{"with empty notes", &ai.GenerationError{}, "AI returned empty notes (merging)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := capOneFilePerChunk(t, tenKBFiles(3))

			err := s.generateErr(t, &failingMergeTransport{cause: tc.cause})

			if got, _ := notes.CauseText(err); got != tc.want {
				t.Errorf("CauseText = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCauseText_ChunkedRunNoSingleCallOwns_TakesNoLocation(t *testing.T) {
	t.Parallel()

	t.Run("no room for the diff", func(t *testing.T) {
		t.Parallel()

		s := chunkScenario{files: threeTenKBFiles(), root: t.TempDir()}
		s.cfg.MaxPromptBytes = len(notes.DefaultPrompt) / 2

		err := s.generateErr(t, &chunkTransport{mergeBody: "merged"})

		if got, _ := notes.CauseText(err); got != "no room for the diff under max_prompt_bytes" {
			t.Errorf("CauseText = %q, want the no-room phrase alone", got)
		}
	})

	t.Run("merge prompt over the cap", func(t *testing.T) {
		t.Parallel()

		s := capOneFilePerChunk(t, tenKBFiles(3))

		_, _, err := s.generateWithNotesOf(t, s.cfg.MaxPromptBytes)

		if got, _ := notes.CauseText(err); got != "part notes too large to merge" {
			t.Errorf("CauseText = %q, want the merge-too-large phrase alone", got)
		}
	})
}
