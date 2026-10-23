package notes_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/runner"
)

// wantMergeFraming is spelled out, not read from the package: the exact wording is
// the contract. It names no ranking rule, no headline and no maintenance line, and
// gives no nothing-to-list body of its own; the governing format decides all of
// them.
func wantMergeFraming(parts int) string {
	return fmt.Sprintf("This release's diff was too large to read in one pass, so it was read in %d parts and release notes were written for each part. "+
		"Those notes follow, in part order. From them, write one set of release notes for the whole release, in the format above: "+
		"combine related changes across parts into single bullets and drop duplicates. "+
		mergeFramingNothing, parts)
}

const mergeFramingNothing = "Leave out every line in which a part said it holds nothing for the notes. " +
	"Only where every part returned such a line, write what the format above gives a release with nothing to list; never do so while any part listed a change."

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
// instructions and the part notes names a headline, asks for a ranking, or carries
// the default prompt's maintenance line, users-meet test or doubt rule.
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
	for _, defaultOnly := range []string{"maintenance", "users meet", "unsure", "in doubt"} {
		if strings.Contains(lower, defaultOnly) {
			t.Errorf("%s: what mint adds around the instructions carries %q, which belongs to the default prompt alone", label, defaultOnly)
		}
	}
}

const maintenanceLine = "Maintenance release — no notable source changes"

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

func TestGenerator_OverCap_DefaultPrompt_MergeInstructionsCarryTheMaintenanceLineRule(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ct := &chunkTransport{mergeBody: "merged notes"}
	run := newFramedRun(t, ct, chunksAndPieces(), config.Release{}, root)

	if _, err := run.gen.Generate(t.Context(), "v1.0.0", run.cfg); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	merges := ct.mergePrompts()
	if len(merges) != 1 {
		t.Fatalf("merge calls = %d, want exactly one", len(merges))
	}
	if !strings.HasPrefix(merges[0], notes.DefaultPrompt+"\n\n"+run.changeMap+"\n\n") {
		t.Fatalf("merge call does not open with the default prompt as its instructions, then the Change Map")
	}
	rule := "Where every change in the release is one these rules leave out, the notes are\n  ONE plain line saying the release is maintenance with no user-facing changes,\n  in the shape of \"" + maintenanceLine + "\". That line\n  has no headline and no sections"
	if !strings.Contains(notes.DefaultPrompt, rule) {
		t.Errorf("merge call's instructions do not carry the maintenance-line rule %q", rule)
	}
}

// emptyMergeRunner stands in for the AI command behind a real ai.Transport: every
// chunk call answers with the line saying its part holds nothing for the notes, and
// every other call, the merge, with an empty body. It is safe for concurrent calls.
type emptyMergeRunner struct {
	mu     sync.Mutex
	parts  int
	merges int
}

func (r *emptyMergeRunner) RunWith(_ context.Context, stdin io.Reader, _ string, _ ...string) (runner.Result, error) {
	b, err := io.ReadAll(stdin)
	if err != nil {
		return runner.Result{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if isChunkPrompt(string(b)) {
		r.parts++
		return runner.Result{Stdout: "This part holds nothing for the notes."}, nil
	}
	r.merges++
	return runner.Result{}, nil
}

func (r *emptyMergeRunner) Run(context.Context, string, ...string) (runner.Result, error) {
	return runner.Result{}, errors.New("unexpected Run")
}

func (r *emptyMergeRunner) RunInteractive(context.Context, string, ...string) error {
	return errors.New("unexpected RunInteractive")
}

func (r *emptyMergeRunner) RunInDir(context.Context, string, []string, string, ...string) (runner.Result, error) {
	return runner.Result{}, errors.New("unexpected RunInDir")
}

func TestGenerator_OverCap_FullOverride_MergeReturnsEmptyBody_FailsLikeAnyEmptyBody(t *testing.T) {
	t.Parallel()

	aiRunner := &emptyMergeRunner{}
	timeout := waitLimit
	transport := ai.NewTransport(aiRunner, ai.Config{AICommand: "claude -p", Timeout: &timeout})
	s := capOneFilePerChunk(t, tenKBFiles(3))

	err := awaitFailure(t, s.startGenerate(t, transport))

	var genErr *ai.GenerationError
	if !errors.Is(err, ai.ErrGenerationFailed) || !errors.As(err, &genErr) {
		t.Errorf("error = %v, want the empty body's generation failure", err)
	}
	if strings.Contains(err.Error(), "generating notes for part") {
		t.Errorf("error = %q, want the merge's failure, not a part's", err)
	}
	aiRunner.mu.Lock()
	defer aiRunner.mu.Unlock()
	if aiRunner.parts != 3 {
		t.Errorf("part calls = %d, want each of the three parts once", aiRunner.parts)
	}
	if aiRunner.merges != 2 {
		t.Errorf("merge attempts = %d, want the call and the transport's one retry", aiRunner.merges)
	}
}
