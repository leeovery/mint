package notes_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mint/internal/config"
	"mint/internal/notes"
)

// generateErr runs the scenario's generation over transport and returns its error.
func (s chunkScenario) generateErr(t *testing.T, transport notes.Transport) error {
	t.Helper()
	r := seedNormalPathGit(t, s.diff(), s.nameStatus(), s.numstat())
	gen := notes.NewGenerator(notes.NewAssembler(r, "/repo", notes.ExcludeConfig{}), transport, s.root)
	body, err := gen.Generate(t.Context(), "v1.0.0", s.cfg)
	if err == nil {
		t.Fatalf("Generate returned body %q and nil error, want a failure", body)
	}
	return err
}

func writeRootFile(t *testing.T, root, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGenerator_OverCap_OperatorTextLeavesNoRoomForDiff_SendsNoCall(t *testing.T) {
	t.Parallel()

	outsized := strings.Repeat("Project context the operator wrote at length.\n", 1_000)
	tests := []struct {
		name string
		set  func(rel *config.Release)
	}{
		{name: "context file", set: func(rel *config.Release) { rel.Context = "context.md" }},
		{name: "prompt override", set: func(rel *config.Release) { rel.Prompt = "context.md" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeRootFile(t, root, "context.md", outsized)
			s := chunkScenario{files: threeTenKBFiles(), root: root}
			s.cfg = config.Config{MaxPromptBytes: 40_000}
			tt.set(&s.cfg.Release)
			if len(s.singlePass(t)) <= s.cfg.MaxPromptBytes || s.fixedPart(t) <= s.cfg.MaxPromptBytes {
				t.Fatalf("fixture: want the single-pass prompt and its fixed part both over the cap")
			}
			ct := &chunkTransport{mergeBody: "merged notes"}

			err := s.generateErr(t, ct)

			if !errors.Is(err, notes.ErrNoRoomForDiff) {
				t.Errorf("Generate error = %v, want ErrNoRoomForDiff", err)
			}
			if len(ct.prompts) != 0 {
				t.Errorf("AI calls = %d, want none sent", len(ct.prompts))
			}
		})
	}
}

func TestGenerator_OverCap_CapBelowMintsOwnInstructions_SendsNoCall(t *testing.T) {
	t.Parallel()

	s := chunkScenario{files: threeTenKBFiles(), root: t.TempDir()}
	s.cfg = config.Config{MaxPromptBytes: len(notes.DefaultPrompt) / 2}
	ct := &chunkTransport{mergeBody: "merged notes"}

	err := s.generateErr(t, ct)

	if !errors.Is(err, notes.ErrNoRoomForDiff) {
		t.Errorf("Generate error = %v, want ErrNoRoomForDiff", err)
	}
	if len(ct.prompts) != 0 {
		t.Errorf("AI calls = %d, want none sent", len(ct.prompts))
	}
}

func TestGenerator_OverCap_NoRoomForDiff_ReportsNoProgress(t *testing.T) {
	t.Parallel()

	s := chunkScenario{files: threeTenKBFiles(), root: t.TempDir()}
	s.cfg = config.Config{MaxPromptBytes: len(notes.DefaultPrompt) / 2}
	ct := &chunkTransport{mergeBody: "merged notes"}
	log := &progressLog{ct: ct}
	r := seedNormalPathGit(t, s.diff(), s.nameStatus(), s.numstat())
	gen := notes.NewGenerator(notes.NewAssembler(r, "/repo", notes.ExcludeConfig{}), ct, s.root).WithProgress(log)

	if _, err := gen.Generate(t.Context(), "v1.0.0", s.cfg); !errors.Is(err, notes.ErrNoRoomForDiff) {
		t.Fatalf("Generate error = %v, want ErrNoRoomForDiff", err)
	}
	if len(log.reports) != 0 {
		t.Errorf("progress reports = %q, want none for a run that sends no call", log.reports)
	}
}

// sizedNotesTransport answers each chunk call with notes of notesSize bytes and
// records every prompt.
type sizedNotesTransport struct {
	chunkTransport
	notesSize int
}

func (st *sizedNotesTransport) Generate(ctx context.Context, prompt string) (string, error) {
	body, err := st.chunkTransport.Generate(ctx, prompt)
	if isChunkPrompt(prompt) {
		return strings.Repeat("n", st.notesSize), err
	}
	return body, err
}

func (s chunkScenario) generateWithNotesOf(t *testing.T, notesSize int) (*sizedNotesTransport, string, error) {
	t.Helper()
	st := &sizedNotesTransport{chunkTransport: chunkTransport{mergeBody: "merged notes"}, notesSize: notesSize}
	r := seedNormalPathGit(t, s.diff(), s.nameStatus(), s.numstat())
	gen := notes.NewGenerator(notes.NewAssembler(r, "/repo", notes.ExcludeConfig{}), st, s.root)
	body, err := gen.Generate(t.Context(), "v1.0.0", s.cfg)
	return st, body, err
}

func TestGenerator_OverCap_MergePromptOverCap_SendsNoMerge(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(2))

	st, _, err := s.generateWithNotesOf(t, 10_000)

	if !errors.Is(err, notes.ErrMergeTooLarge) {
		t.Errorf("Generate error = %v, want ErrMergeTooLarge", err)
	}
	if got := len(st.chunkPrompts()); got != 2 {
		t.Errorf("chunk calls = %d, want both sent", got)
	}
	if got := len(st.mergePrompts()); got != 0 {
		t.Errorf("merge calls = %d, want none sent", got)
	}
}

func TestGenerator_OverCap_MergePromptExactlyAtCap_SendsTheMerge(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(2))
	const probeSize = 1_000
	probe, _, err := s.generateWithNotesOf(t, probeSize)
	if err != nil {
		t.Fatalf("probe Generate returned unexpected error: %v", err)
	}
	merges := probe.mergePrompts()
	if len(merges) != 1 {
		t.Fatalf("probe merge calls = %d, want one", len(merges))
	}
	// The merge prompt grows byte for byte with the notes of each of the two parts.
	slack := s.cfg.MaxPromptBytes - len(merges[0])
	if slack <= 0 || slack%2 != 0 {
		t.Fatalf("fixture: probe merge prompt leaves %d bytes under the cap, want a positive even slack", slack)
	}

	st, body, err := s.generateWithNotesOf(t, probeSize+slack/2)

	if err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}
	merges = st.mergePrompts()
	if len(merges) != 1 || len(merges[0]) != s.cfg.MaxPromptBytes {
		t.Fatalf("merge calls = %d, want one exactly at the %d-byte cap", len(merges), s.cfg.MaxPromptBytes)
	}
	if body != "merged notes" {
		t.Errorf("body = %q, want the merge output", body)
	}
}

func TestGenerator_OverCap_MergePromptOverCap_ReportsNoMerging(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(2))
	st := &sizedNotesTransport{chunkTransport: chunkTransport{mergeBody: "merged notes"}, notesSize: 10_000}
	log := &progressLog{ct: &st.chunkTransport}
	r := seedNormalPathGit(t, s.diff(), s.nameStatus(), s.numstat())
	gen := notes.NewGenerator(notes.NewAssembler(r, "/repo", notes.ExcludeConfig{}), st, s.root).WithProgress(log)

	if _, err := gen.Generate(t.Context(), "v1.0.0", s.cfg); !errors.Is(err, notes.ErrMergeTooLarge) {
		t.Fatalf("Generate error = %v, want ErrMergeTooLarge", err)
	}
	if got := log.reports[len(log.reports)-1]; !strings.HasSuffix(got, ", 2 of 2 done") {
		t.Errorf("last report = %q, want the finished parts and no merge reported", got)
	}
	for _, report := range log.reports {
		if strings.HasPrefix(report, "merging") || strings.HasPrefix(report, "merged") {
			t.Errorf("progress reported %q for a merge that was never sent", report)
		}
	}
}
