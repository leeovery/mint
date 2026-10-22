package notes_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/notes"
)

const tooLargePhrase = "too large for the model, even in smaller parts"

// opensWith overflows each slice that opens with prefix.
func opensWith(prefix string) func(string) bool {
	return func(slice string) bool { return strings.HasPrefix(slice, prefix) }
}

// assertFailedWithNoMerge fails unless the run failed, made no merge call, and carries
// the overflowed calls' output.
func (ht *halvingTransport) assertFailedWithNoMerge(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Generate returned nil error, want the notes to fail")
	}
	if len(ht.merges) != 0 {
		t.Errorf("merge calls = %d, want none", len(ht.merges))
	}
	var genErr *ai.GenerationError
	if !errors.As(err, &genErr) || genErr.Output() != overflowOutput {
		t.Errorf("error = %v, want it to carry the overflowed call's output verbatim", err)
	}
}

func TestGenerator_SinglePassHalvedThreeTimes_FourthSizeFailsWithNoFurtherRecutOrMerge(t *testing.T) {
	t.Parallel()

	files := make([]chunkFile, 16)
	for i := range files {
		files[i] = chunkFile{path: fmt.Sprintf("area/f%02d.go", i), size: 10_000}
	}
	s := underCapScenario(t, files)
	failedSize := len(s.singlePass(t))
	// Each round's first part, the one opening with area/f00.go, overflows.
	ht := &halvingTransport{overflows: opensWith(fileDiff("area/f00.go", 10_000)), mergeBody: "merged"}

	_, err := s.generateWith(t, ht)

	if len(ht.single) != 1 || len(ht.overflowed) != 3 || ht.classify != 4 {
		t.Fatalf("single-pass calls = %d, overflowed chunk calls = %d, classify calls = %d; want 1, 3, 4",
			len(ht.single), len(ht.overflowed), ht.classify)
	}
	third := ht.halvedFrom(ht.overflowed[1])
	if !slices.Contains(third, ht.sent[ht.overflowed[2]]) {
		t.Fatalf("the last overflow is not one of the third round's calls")
	}
	for i, p := range third {
		if 8*len(p) > failedSize {
			t.Errorf("third-round call %d is %d bytes, over an eighth of the single-pass call's %d", i+1, len(p), failedSize)
		}
	}
	if again := ht.halvedFrom(ht.overflowed[2]); len(again) != 0 {
		t.Errorf("calls re-cut from the fourth SIZE = %d, want none", len(again))
	}
	ht.assertFailedWithNoMerge(t, err)
	k, n := partOf(t, ht.sent[ht.overflowed[2]])
	if want := fmt.Sprintf("%s (part %d of %d)", tooLargePhrase, k, n); causeOf(err) != want {
		t.Errorf("CauseText = %q, want %q", causeOf(err), want)
	}
}

func TestGenerator_ChunkHalvedThreeTimes_FourthSizeFailsTheSliceNamingItsPart(t *testing.T) {
	t.Parallel()

	files := make([]chunkFile, 6)
	for i := range files {
		files[i] = chunkFile{path: fmt.Sprintf("area%d/file.go", i+1), size: 200_000}
	}
	s := chunkScenario{files: files, root: t.TempDir(), cfg: config.Config{MaxPromptBytes: config.DefaultMaxPromptBytes}}
	s.cfg.MaxPromptBytes = s.fixedPart(t) + 205_000
	// Part 3 overflows, then the first piece of each re-cut of it.
	ht := &halvingTransport{overflows: opensWith(pieceHeader("area3/file.go") + "@@"), mergeBody: "merged"}

	_, err := s.generateWith(t, ht)

	if len(ht.overflowed) != 4 || ht.classify != 4 {
		t.Fatalf("overflowed calls = %d, classify calls = %d, want four of each", len(ht.overflowed), ht.classify)
	}
	// Each re-cut yields 3 parts, so each overflow names part 3 of a total 2 larger.
	for round, call := range ht.overflowed {
		if k, n := partOf(t, ht.sent[call]); k != 3 || n != 6+2*round {
			t.Fatalf("overflow %d names part %d of %d, want part 3 of %d", round+1, k, n, 6+2*round)
		}
	}
	for _, p := range ht.sent {
		if _, n := partOf(t, p); n > 12 {
			t.Errorf("a chunk call names a total of %d parts, want no re-cut after the fourth SIZE", n)
		}
	}
	ht.assertFailedWithNoMerge(t, err)
	if want := tooLargePhrase + " (part 3 of 12)"; causeOf(err) != want {
		t.Errorf("CauseText = %q, want %q", causeOf(err), want)
	}
}

// outsizedFixedPart is a scenario whose operator context gives every call a fixed
// part of about 120,000 bytes.
func outsizedFixedPart(t *testing.T, files []chunkFile, maxPromptBytes int) chunkScenario {
	t.Helper()
	root := t.TempDir()
	writeRootFile(t, root, "context.md", strings.Repeat("Project context the operator wrote at length.\n", 2_540))
	s := chunkScenario{files: files, root: root, cfg: config.Config{MaxPromptBytes: maxPromptBytes}}
	s.cfg.Release.Context = "context.md"
	return s
}

func TestGenerator_ChunkHalvingRoundBelowItsFixedPart_SendsNoCallAndFailsNamingItsPart(t *testing.T) {
	t.Parallel()

	files := []chunkFile{{"alpha/a.go", 75_000}, {"beta/b.go", 75_000}, {"gamma/c.go", 75_000}}
	s := outsizedFixedPart(t, files, 0)
	// Room for one whole file per call, so its halved target falls below the fixed part.
	s.cfg.MaxPromptBytes = s.fixedPart(t) + 80_000
	ht := &halvingTransport{overflows: isSlice(fileDiff("beta/b.go", 75_000)), mergeBody: "merged"}

	_, err := s.generateWith(t, ht)

	if len(ht.sent) != 3 || len(ht.overflowed) != 1 || ht.classify != 1 {
		t.Fatalf("chunk calls = %d, overflowed = %d, classify calls = %d; want 3, 1, 1", len(ht.sent), len(ht.overflowed), ht.classify)
	}
	failed := ht.sent[ht.overflowed[0]]
	if fixed := len(failed) - len(sliceOf(failed)); len(failed)/2 >= fixed {
		t.Fatalf("fixture: halved target %d is not below the call's fixed part %d", len(failed)/2, fixed)
	}
	if again := ht.halvedFrom(ht.overflowed[0]); len(again) != 0 {
		t.Errorf("calls re-cut from the overflowed part = %d, want none", len(again))
	}
	ht.assertFailedWithNoMerge(t, err)
	if want := tooLargePhrase + " (part 2 of 3)"; causeOf(err) != want {
		t.Errorf("CauseText = %q, want %q", causeOf(err), want)
	}
}

func TestGenerator_SinglePassFirstHalvingRoundHasNoRoom_SendsNoChunkCallAndFailsWithNoLocation(t *testing.T) {
	t.Parallel()

	s := outsizedFixedPart(t, []chunkFile{{"alpha/a.go", 40_000}, {"beta/b.go", 40_000}}, config.DefaultMaxPromptBytes)
	if singlePass, fixed := len(s.singlePass(t)), s.fixedPart(t); singlePass/2 >= fixed {
		t.Fatalf("fixture: halved target %d is not below the fixed part %d", singlePass/2, fixed)
	}
	ht := &halvingTransport{overflows: func(string) bool { return false }, mergeBody: "merged"}

	_, err := s.generateWith(t, ht)

	if len(ht.single) != 1 || ht.classify != 1 || len(ht.sent) != 0 || len(ht.merges) != 0 {
		t.Errorf("single-pass calls = %d, classify = %d, chunk calls = %d, merges = %d; want 1, 1, 0, 0",
			len(ht.single), ht.classify, len(ht.sent), len(ht.merges))
	}
	if causeOf(err) != tooLargePhrase {
		t.Errorf("CauseText = %q, want %q with no location", causeOf(err), tooLargePhrase)
	}
	var genErr *ai.GenerationError
	if !errors.As(err, &genErr) || genErr.Output() != overflowOutput {
		t.Errorf("error = %v, want it to carry the single-pass call's output verbatim", err)
	}
}

func TestCauseText_TooLargeForTheModel_WinsOverTheCallFailureItCarries(t *testing.T) {
	t.Parallel()

	cause := fmt.Errorf("%w: %w", notes.ErrTooLargeForModel, &ai.GenerationError{Stdout: overflowOutput, ExitCode: 1})

	if got, known := notes.CauseText(cause); !known || got != tooLargePhrase {
		t.Errorf("CauseText = %q, %v; want %q, true", got, known, tooLargePhrase)
	}
}

func causeOf(err error) string {
	phrase, _ := notes.CauseText(err)
	return phrase
}
