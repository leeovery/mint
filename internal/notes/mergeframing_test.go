package notes_test

import (
	"fmt"
	"strings"
	"testing"

	"mint/internal/config"
	"mint/internal/notes"
)

// wantMergeFraming is spelled out, not read from the package: the exact wording is
// the contract. It names no ranking rule and no headline; the governing format
// decides both.
func wantMergeFraming(parts int) string {
	return fmt.Sprintf("This release's diff was too large to read in one pass, so it was read in %d parts and release notes were written for each part. "+
		"Those notes follow, in part order. From them, write one set of release notes for the whole release, in the format above: "+
		"combine related changes across parts into single bullets and drop duplicates.", parts)
}

// wantMergePrompt is the merge call over partNotes: the instructions, the
// whole-release Change Map, the framing, every part's notes in part order and the
// closing reminder, in that order and nothing else.
func wantMergePrompt(instructions, changeMap string, partNotes []string) string {
	sections := []string{instructions, changeMap, wantMergeFraming(len(partNotes))}
	for i, n := range partNotes {
		sections = append(sections, fmt.Sprintf("Notes for part %d of %d:\n\n%s", i+1, len(partNotes), n))
	}
	return strings.Join(append(sections, formatNeutralReminder), "\n\n")
}

// partNotesOf returns, in part order, the notes chunkTransport answered each chunk
// call with.
func partNotesOf(t *testing.T, ct *chunkTransport) []string {
	t.Helper()
	var out []string
	for _, p := range chunksInPartOrder(t, ct) {
		out = append(out, chunkNotesFor(firstPath(sliceOf(p))))
	}
	return out
}

// assertMergeAddsNoHeadlineOrRanking fails if what mint adds around the
// instructions and the part notes names a headline or asks for a ranking.
func assertMergeAddsNoHeadlineOrRanking(t *testing.T, label, prompt, instructions string, partNotes []string) {
	t.Helper()
	added, ok := strings.CutPrefix(prompt, instructions+"\n\n")
	if !ok {
		t.Fatalf("%s does not open with the release's instructions verbatim", label)
	}
	for _, n := range partNotes {
		added = strings.Replace(added, n, "", 1)
	}
	lower := strings.ToLower(added)
	if strings.Contains(lower, "headline") {
		t.Errorf("%s: what mint adds around the instructions names a headline", label)
	}
	if strings.Contains(lower, "rank") {
		t.Errorf("%s: what mint adds around the instructions asks for a ranking", label)
	}
}

func mergeFramingReleases() []struct {
	name    string
	release func(t *testing.T, root string) config.Release
} {
	return []struct {
		name    string
		release func(t *testing.T, root string) config.Release
	}{
		{name: "default prompt", release: func(*testing.T, string) config.Release { return config.Release{} }},
		{name: "full [release].prompt override", release: func(t *testing.T, root string) config.Release {
			return config.Release{Prompt: writeFile(t, root, ".mint/notes-prompt.md", framingOverride)}
		}},
	}
}

func TestGenerator_OverCap_MergeFramingLeavesRankingAndTheHeadlineToTheGoverningFormat(t *testing.T) {
	t.Parallel()

	for _, tt := range mergeFramingReleases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			ct := &chunkTransport{mergeBody: "merged notes"}
			run := newFramedRun(t, ct, chunksAndPieces(), tt.release(t, root), root)

			if _, err := run.gen.Generate(t.Context(), "v1.0.0", run.cfg); err != nil {
				t.Fatalf("Generate returned unexpected error: %v", err)
			}

			merges := ct.mergePrompts()
			if len(merges) != 1 {
				t.Fatalf("merge calls = %d, want exactly one", len(merges))
			}
			partNotes := partNotesOf(t, ct)
			if len(partNotes) < 2 {
				t.Fatalf("parts = %d, want a chunked run", len(partNotes))
			}
			if want := wantMergePrompt(run.instructions, run.changeMap, partNotes); merges[0] != want {
				t.Errorf("merge prompt:\n got: %q\nwant: %q", merges[0], want)
			}
			assertMergeAddsNoHeadlineOrRanking(t, "merge call", merges[0], run.instructions, partNotes)
		})
	}
}

func TestGenerator_Remerge_CarriesTheNudgeAndTheSameMergeFraming(t *testing.T) {
	t.Parallel()

	const nudge = "lead with the auth work"
	for _, tt := range mergeFramingReleases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			ct := &chunkTransport{mergeBody: "merged notes"}
			var kept []notes.MergeInputs
			run := newFramedRun(t, ct, chunksAndPieces(), tt.release(t, root), root)
			gen := run.gen.WithMergeInputs(func(in notes.MergeInputs) { kept = append(kept, in) })
			if _, err := gen.Generate(t.Context(), "v1.0.0", run.cfg); err != nil {
				t.Fatalf("Generate returned unexpected error: %v", err)
			}
			if len(kept) != 1 {
				t.Fatalf("merge inputs kept = %d, want the chunked run's one", len(kept))
			}
			partNotes := partNotesOf(t, ct)
			calls := len(ct.prompts)

			if _, err := gen.Remerge(t.Context(), kept[0], run.cfg, nudge); err != nil {
				t.Fatalf("Remerge returned unexpected error: %v", err)
			}

			if got := len(ct.prompts) - calls; got != 1 {
				t.Fatalf("re-merge AI calls = %d, want one merge call", got)
			}
			remerge := ct.prompts[len(ct.prompts)-1]
			if isChunkPrompt(remerge) || strings.Contains(remerge, "diff --git ") {
				t.Errorf("the re-merge is not a merge call over the part notes")
			}
			nudged := run.instructions + "\n\n" + nudge
			if want := wantMergePrompt(nudged, run.changeMap, partNotes); remerge != want {
				t.Errorf("re-merge prompt:\n got: %q\nwant: %q", remerge, want)
			}
			assertMergeAddsNoHeadlineOrRanking(t, "re-merge call", remerge, nudged, partNotes)
		})
	}
}
