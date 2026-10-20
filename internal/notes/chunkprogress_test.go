package notes_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"mint/internal/config"
	"mint/internal/notes"
)

// progressLog records a run's progress reports as lines. It holds no lock: the
// generator serialises its reports, so the race detector fails a run that does not.
type progressLog struct {
	ct      *chunkTransport
	reports []string
}

func (pl *progressLog) PartsDone(promptBytes, parts, done int) {
	pl.reports = append(pl.reports, fmt.Sprintf("%d bytes, %d of %d done", promptBytes, done, parts))
}

func (pl *progressLog) Merged(parts int) {
	pl.reports = append(pl.reports, fmt.Sprintf("merged %d parts", parts))
}

func (pl *progressLog) Merging(parts int) {
	pl.ct.mu.Lock()
	merges := len(pl.ct.mergePrompts())
	pl.ct.mu.Unlock()
	pl.reports = append(pl.reports, fmt.Sprintf("merging %d parts, %d merge calls made", parts, merges))
}

func (s chunkScenario) runWithProgress(t *testing.T) (*chunkTransport, *progressLog) {
	t.Helper()
	ct := &chunkTransport{mergeBody: "merged notes"}
	log := &progressLog{ct: ct}
	r := seedNormalPathGit(t, s.diff(), s.nameStatus(), s.numstat())
	gen := notes.NewGenerator(notes.NewAssembler(r, notes.ExcludeConfig{}), ct, s.root).WithProgress(log)
	if _, err := gen.Generate(t.Context(), "v1.0.0", s.cfg); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}
	return ct, log
}

func TestGenerator_OverCap_ReportsTheSplitEachFinishedPartThenTheMerge(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(6))
	ct, log := s.runWithProgress(t)

	size := len(s.singlePass(t))
	if size <= s.cfg.MaxPromptBytes {
		t.Fatalf("fixture single-pass prompt is %d bytes, want it over the %d cap", size, s.cfg.MaxPromptBytes)
	}
	if got := len(ct.chunkPrompts()); got != 6 {
		t.Fatalf("chunk calls = %d, want 6", got)
	}
	var want []string
	for done := 0; done <= 6; done++ {
		want = append(want, fmt.Sprintf("%d bytes, %d of 6 done", size, done))
	}
	want = append(want, "merging 6 parts, 0 merge calls made", "merged 6 parts")
	if !slices.Equal(log.reports, want) {
		t.Errorf("progress reports =\n%s\nwant\n%s", strings.Join(log.reports, "\n"), strings.Join(want, "\n"))
	}
	if got := len(ct.mergePrompts()); got != 1 {
		t.Errorf("merge calls = %d, want one after the merge was reported", got)
	}
}

func TestGenerator_OverCap_EachPieceCountsAsOnePart(t *testing.T) {
	t.Parallel()

	ct := &chunkTransport{mergeBody: "merged notes"}
	log := &progressLog{ct: ct}
	files := []pieceFile{
		{"alpha/a.go", fileDiff("alpha/a.go", 10_000)},
		{"big/huge.go", hunkedFileDiff("big/huge.go", 12, 5_000)},
		{"gamma/c.go", fileDiff("gamma/c.go", 10_000)},
	}
	gen, cfg := piecesGeneration(t, ct, files)
	if _, err := gen.WithProgress(log).Generate(t.Context(), "v1.0.0", cfg); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	pieces := len(piecesOf(t, ct, "big/huge.go"))
	if pieces < 2 {
		t.Fatalf("pieces of big/huge.go = %d, want the file sent as pieces", pieces)
	}
	parts := pieces + 2
	if got := len(ct.chunkPrompts()); got != parts {
		t.Fatalf("chunk calls = %d, want %d", got, parts)
	}
	if got := len(log.reports); got != parts+3 {
		t.Fatalf("reports = %d, want the split, one per part, the merge and its end", got)
	}
	if got, want := log.reports[parts+1:], []string{fmt.Sprintf("merging %d parts, 0 merge calls made", parts), fmt.Sprintf("merged %d parts", parts)}; !slices.Equal(got, want) {
		t.Errorf("last reports = %q, want %q", got, want)
	}
	for _, report := range log.reports[:parts+1] {
		if !strings.HasSuffix(report, fmt.Sprintf(" of %d done", parts)) {
			t.Errorf("report %q does not count out of %d parts", report, parts)
		}
	}
}

func TestGenerator_UnderCap_ReportsNoProgress(t *testing.T) {
	t.Parallel()

	s := chunkScenario{
		files: threeTenKBFiles(),
		cfg:   config.Config{MaxPromptBytes: config.DefaultMaxPromptBytes},
		root:  t.TempDir(),
	}
	ct, log := s.runWithProgress(t)

	if len(ct.prompts) != 1 {
		t.Fatalf("AI calls = %d, want one single-pass call", len(ct.prompts))
	}
	if len(log.reports) != 0 {
		t.Errorf("progress reports = %q, want none for a single-pass run", log.reports)
	}
}

// mergeFailingTransport answers chunk calls with notes and fails the merge.
type mergeFailingTransport struct {
	chunkTransport
}

func (mt *mergeFailingTransport) Generate(ctx context.Context, prompt string) (string, error) {
	notes, err := mt.chunkTransport.Generate(ctx, prompt)
	if isChunkPrompt(prompt) {
		return notes, err
	}
	return "", errors.New("merge failed")
}

func TestGenerator_OverCap_FailedMerge_ReportsMergingButNotMerged(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(2))
	mt := &mergeFailingTransport{}
	log := &progressLog{ct: &mt.chunkTransport}
	r := seedNormalPathGit(t, s.diff(), s.nameStatus(), s.numstat())
	gen := notes.NewGenerator(notes.NewAssembler(r, notes.ExcludeConfig{}), mt, s.root).WithProgress(log)
	if _, err := gen.Generate(t.Context(), "v1.0.0", s.cfg); err == nil {
		t.Fatal("Generate returned nil error for a failed merge")
	}

	if got := log.reports[len(log.reports)-1]; !strings.HasPrefix(got, "merging 2 parts") {
		t.Errorf("last report = %q, want the merge reported and nothing after it", got)
	}
}
