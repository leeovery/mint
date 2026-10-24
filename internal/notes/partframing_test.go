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

// The part framings are spelled out, not read from the package: the exact wording is
// the contract.
const (
	framingOpening   = "This release's diff is too large to read in one pass, so it has been split into parts. "
	framingSummaryTo = "Leave out anything that sums up the release, or this part, as a whole, since the merge writes that from every part's notes."
	framingNothing   = "If this part holds nothing the notes would list, return one line saying this part holds nothing for the notes, in place of an empty reply. That line speaks of this part alone, never of the release."
)

func wantChunkFraming(part, parts int) string {
	return fmt.Sprintf(framingOpening+
		"What follows is part %d of %d. The Change Map above covers the whole release; the diff below covers only this part. "+
		"Write release notes for the changes in this part, in the format above. They will be merged with the other parts' notes. "+
		framingSummaryTo+" "+framingNothing, part, parts)
}

func wantPieceFraming(part, parts, piece, pieces int, file string) string {
	return fmt.Sprintf(framingOpening+
		"What follows is part %d of %d: piece %d of %d of the diff for %s, a file too large to read in one part, so its diff has been cut into pieces. "+
		"The Change Map above covers the whole release; the diff below covers only this piece of that file. "+
		"Write release notes for the changes in this piece, in the format above. They will be merged with the other parts' notes. "+
		framingSummaryTo+" "+framingNothing, part, parts, piece, pieces, file)
}

// wantFraming is the framing a chunk or piece call must carry, read from the part,
// piece and file it names.
func wantFraming(t *testing.T, prompt string) string {
	t.Helper()
	part, parts := partOf(t, prompt)
	if piece, pieces, ok := pieceOf(prompt); ok {
		return wantPieceFraming(part, parts, piece, pieces, firstPath(sliceOf(prompt)))
	}
	return wantChunkFraming(part, parts)
}

// assertPartCall fails unless prompt is the release's instructions, the
// whole-release Change Map, the framing naming its part, its slice and the closing
// reminder, in that order and nothing else, with no headline, maintenance line,
// users-meet test or doubt rule in what mint adds.
func assertPartCall(t *testing.T, label, prompt, instructions, changeMap string) {
	t.Helper()
	framing := framingOf(prompt)
	if want := wantFraming(t, prompt); framing != want {
		t.Errorf("%s framing:\n got: %q\nwant: %q", label, framing, want)
	}
	if strings.Contains(strings.ToLower(framing), "headline") {
		t.Errorf("%s framing names a headline:\n%s", label, framing)
	}
	want := instructions + "\n\n" + changeMap + "\n\n" + framing + "\n\n" + sliceOf(prompt) + "\n\n" + formatNeutralReminder
	if prompt != want {
		t.Errorf("%s is not instructions, Change Map, framing, slice and reminder alone, in that order", label)
	}
	added, ok := strings.CutPrefix(prompt, instructions+"\n\n")
	if !ok {
		t.Fatalf("%s does not open with the release's instructions verbatim", label)
	}
	addedText := strings.ToLower(strings.Replace(added, sliceOf(prompt), "", 1))
	for _, defaultOnly := range []string{"headline", "maintenance", "users meet", "unsure", "in doubt"} {
		if strings.Contains(addedText, defaultOnly) {
			t.Errorf("%s: what mint adds around the instructions carries %q, which belongs to the default prompt alone", label, defaultOnly)
		}
	}
}

// framedRun is an over-cap generation whose cap leaves a part about
// pieceRoomOverFixed bytes of diff, under the release's own instructions.
type framedRun struct {
	gen          *notes.Generator
	cfg          config.Config
	instructions string
	changeMap    string
}

func newFramedRun(t *testing.T, transport notes.Transport, files []pieceFile, release config.Release, root string) framedRun {
	t.Helper()
	var diff, nameStatus, numstat strings.Builder
	for _, f := range files {
		diff.WriteString(f.diff)
		nameStatus.WriteString("M\t" + f.path + "\n")
		fmt.Fprintf(&numstat, "%d\t0\t%s\n", len(f.diff)/64, f.path)
	}
	changeMap, err := notes.NewAssembler(seedChangeMapGit(t, nameStatus.String(), numstat.String()), "/repo", notes.ExcludeConfig{}).
		BuildChangeMap(t.Context(), "v1.0.0", config.DefaultMaxPromptBytes)
	if err != nil {
		t.Fatalf("BuildChangeMap returned unexpected error: %v", err)
	}
	instructions, err := notes.ResolveInstructions(root, release)
	if err != nil {
		t.Fatalf("ResolveInstructions returned unexpected error: %v", err)
	}
	cfg := config.Config{
		MaxPromptBytes: len(notes.ComposePrompt(instructions, changeMap, "")) + pieceRoomOverFixed,
		Release:        release,
	}
	r := seedNormalPathGit(t, diff.String(), nameStatus.String(), numstat.String())
	return framedRun{
		gen:          notes.NewGenerator(notes.NewAssembler(r, "/repo", notes.ExcludeConfig{}), transport, root),
		cfg:          cfg,
		instructions: instructions,
		changeMap:    changeMap,
	}
}

func chunksAndPieces() []pieceFile {
	return []pieceFile{
		{"alpha/a.go", fileDiff("alpha/a.go", 10_000)},
		{"big/huge.go", hunkedFileDiff("big/huge.go", 12, 5_000)},
		{"gamma/c.go", fileDiff("gamma/c.go", 10_000)},
	}
}

const framingOverride = "Write a terse bullet list of what changed. Nothing else."

func TestGenerator_OverCap_EveryChunkAndPieceFramingLeavesTheReleaseSummaryToTheMerge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		release func(t *testing.T, root string) config.Release
	}{
		{name: "default prompt", release: func(*testing.T, string) config.Release { return config.Release{} }},
		{name: "full [release].prompt override", release: func(t *testing.T, root string) config.Release {
			return config.Release{Prompt: writeFile(t, root, ".mint/notes-prompt.md", framingOverride)}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			ct := &chunkTransport{mergeBody: "merged notes"}
			run := newFramedRun(t, ct, chunksAndPieces(), tt.release(t, root), root)
			if run.cfg.Release.Prompt != "" && run.instructions != framingOverride {
				t.Fatalf("override instructions = %q, want the prompt file verbatim", run.instructions)
			}

			if _, err := run.gen.Generate(t.Context(), "v1.0.0", run.cfg); err != nil {
				t.Fatalf("Generate returned unexpected error: %v", err)
			}

			if n := len(piecesOf(t, ct, "big/huge.go")); n < 2 {
				t.Fatalf("pieces of big/huge.go = %d, want the file sent as pieces", n)
			}
			chunks := chunksInPartOrder(t, ct)
			if len(chunks) < 4 {
				t.Fatalf("chunk calls = %d, want whole-file chunks beside the pieces", len(chunks))
			}
			for i, p := range chunks {
				assertPartCall(t, fmt.Sprintf("part call %d", i+1), p, run.instructions, run.changeMap)
			}
		})
	}
}

func TestGenerator_SinglePassClassifiedSize_EveryHalvedCallCarriesTheSummaryFraming(t *testing.T) {
	t.Parallel()

	s := underCapScenario(t, threeTenKBFiles())
	ot := &overflowTransport{classifyAnswer: "SIZE"}

	if _, err := s.generateOver(t, ot); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	chunks := ot.chunksInOrder(t)
	if len(chunks) < 2 {
		t.Fatalf("chunk calls = %d, want the single call re-sent as smaller calls", len(chunks))
	}
	for i, p := range chunks {
		assertPartCall(t, fmt.Sprintf("halved call %d", i+1), p, s.instructions(t), s.changeMap(t))
	}
}

func TestGenerator_ChunkClassifiedSize_EveryHalvedCallCarriesTheSummaryFraming(t *testing.T) {
	t.Parallel()

	s := capOneAreaPerChunk(t, areaFiles(4))
	ht := &halvingTransport{overflows: overflowsOnce(isSlice(areaDiff(2))), mergeBody: "merged"}

	if _, err := s.generateWith(t, ht); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if len(ht.overflowed) != 1 {
		t.Fatalf("overflowed calls = %d, want part 2's one", len(ht.overflowed))
	}
	halved := ht.halvedFrom(ht.overflowed[0])
	if len(halved) < 2 {
		t.Fatalf("calls re-cut from part 2 = %d, want its slice re-sent in smaller calls", len(halved))
	}
	for i, p := range inPartOrder(t, halved) {
		assertPartCall(t, fmt.Sprintf("halved call %d", i+1), p, s.instructions(t), s.changeMap(t))
	}
}

func TestGenerator_PieceClassifiedSize_EveryRecutPieceCarriesTheSummaryFraming(t *testing.T) {
	t.Parallel()

	// A one-line override stands in for the default prompt, so half the failed
	// piece's call still leaves room for diff.
	pieces := 0
	ht := &halvingTransport{mergeBody: "merged"}
	ht.overflows = func(slice string) bool {
		if firstPath(slice) != "big/huge.go" {
			return false
		}
		pieces++
		return pieces == 2
	}
	root := t.TempDir()
	run := newFramedRun(t, ht, []pieceFile{
		{"alpha/a.go", fileDiff("alpha/a.go", 5_000)},
		{"big/huge.go", hunkedFileDiff("big/huge.go", 12, 5_000)},
	}, config.Release{Prompt: writeFile(t, root, "short-prompt.md", framingOverride)}, root)

	if _, err := run.gen.Generate(t.Context(), "v1.0.0", run.cfg); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if len(ht.overflowed) != 1 {
		t.Fatalf("overflowed calls = %d, want one piece", len(ht.overflowed))
	}
	halved := ht.halvedFrom(ht.overflowed[0])
	if len(halved) < 2 {
		t.Fatalf("calls re-cut from the piece = %d, want smaller pieces", len(halved))
	}
	for i, p := range inPartOrder(t, halved) {
		if _, _, ok := pieceOf(p); !ok {
			t.Errorf("re-cut call %d is not a piece", i+1)
		}
		assertPartCall(t, fmt.Sprintf("re-cut piece %d", i+1), p, run.instructions, run.changeMap)
	}
}

// emptyPartRunner stands in for the AI command behind a real ai.Transport: the chunk
// call for emptyPart answers with an empty body, every other chunk call with notes,
// and any other call with the merge's body. It is safe for concurrent calls.
type emptyPartRunner struct {
	emptyPart int

	mu         sync.Mutex
	emptyCalls int
	merges     int
}

func (r *emptyPartRunner) RunWith(_ context.Context, stdin io.Reader, _ string, _ ...string) (runner.Result, error) {
	b, err := io.ReadAll(stdin)
	if err != nil {
		return runner.Result{}, err
	}
	prompt := string(b)
	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case isChunkPrompt(prompt) && partNumber(prompt) == r.emptyPart:
		r.emptyCalls++
		return runner.Result{}, nil
	case isChunkPrompt(prompt):
		return runner.Result{Stdout: chunkNotesFor(firstPath(sliceOf(prompt)))}, nil
	default:
		r.merges++
		return runner.Result{Stdout: "merged"}, nil
	}
}

func (r *emptyPartRunner) Run(context.Context, string, ...string) (runner.Result, error) {
	return runner.Result{}, errors.New("unexpected Run")
}

func (r *emptyPartRunner) RunInteractive(context.Context, string, ...string) error {
	return errors.New("unexpected RunInteractive")
}

func (r *emptyPartRunner) RunInDir(context.Context, string, []string, string, ...string) (runner.Result, error) {
	return runner.Result{}, errors.New("unexpected RunInDir")
}

func TestGenerator_OverCap_PartReturnsEmptyBody_FailsThePartAndTheRun(t *testing.T) {
	t.Parallel()

	aiRunner := &emptyPartRunner{emptyPart: 2}
	timeout := waitLimit
	transport := ai.NewTransport(aiRunner, ai.Config{AICommand: "claude -p", Timeout: &timeout})
	s := capOneFilePerChunk(t, tenKBFiles(3))

	err := awaitFailure(t, s.startGenerate(t, transport))

	if !errors.Is(err, ai.ErrGenerationFailed) {
		t.Errorf("error = %v, want the empty body's generation failure", err)
	}
	if want := "generating notes for part 2 of 3"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to name the failed part (%q)", err, want)
	}
	aiRunner.mu.Lock()
	defer aiRunner.mu.Unlock()
	if aiRunner.emptyCalls != 2 {
		t.Errorf("part 2 attempts = %d, want the call and the transport's one retry", aiRunner.emptyCalls)
	}
	if aiRunner.merges != 0 {
		t.Errorf("merge calls = %d, want none once a part has failed", aiRunner.merges)
	}
}
