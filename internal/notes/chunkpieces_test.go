package notes_test

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"mint/internal/config"
	"mint/internal/notes"
)

var pieceFraming = regexp.MustCompile(`piece (\d+) of (\d+)`)

// pieceRoomOverFixed is how far the test cap sits above the single-pass prompt's
// fixed part: one ~10 KB file fits a chunk, and a 60 KB file does not.
const pieceRoomOverFixed = 15_000

// pieceHeader is the diff header every hunked test file opens with.
func pieceHeader(path string) string {
	return "diff --git a/" + path + " b/" + path + "\nindex 1111111..2222222 100644\n--- a/" + path + "\n+++ b/" + path + "\n"
}

// hunkedFileDiff renders a file's diff of hunks hunks, each about hunkBytes of
// added lines.
func hunkedFileDiff(path string, hunks, hunkBytes int) string {
	var b strings.Builder
	b.WriteString(pieceHeader(path))
	line := "+" + strings.Repeat("y", 63) + "\n"
	for h := range hunks {
		fmt.Fprintf(&b, "@@ -%d,0 +%d,%d @@\n", h*100+1, h*100+1, hunkBytes/len(line))
		b.WriteString(strings.Repeat(line, hunkBytes/len(line)))
	}
	return b.String()
}

// minifiedFileDiff renders a file's diff whose one hunk adds a single line of about
// lineBytes, lead ASCII bytes then one-, three- and four-byte characters.
func minifiedFileDiff(path string, lineBytes, lead int) string {
	const run = "a€😀"
	return pieceHeader(path) + "@@ -0,0 +1 @@\n+" + strings.Repeat("b", lead) + strings.Repeat(run, lineBytes/len(run)) + "\n"
}

// pieceFile is one file of a pieces scenario, its diff as git emits it.
type pieceFile struct {
	path string
	diff string
}

// piecesGeneration builds a generator over files whose cap sits pieceRoomOverFixed
// above the single-pass prompt's fixed part.
func piecesGeneration(t *testing.T, transport notes.Transport, files []pieceFile) (*notes.Generator, config.Config) {
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
	cfg := config.Config{MaxPromptBytes: len(notes.ComposePrompt(notes.DefaultPrompt, changeMap, "")) + pieceRoomOverFixed}
	r := seedNormalPathGit(t, diff.String(), nameStatus.String(), numstat.String())
	return notes.NewGenerator(notes.NewAssembler(r, "/repo", notes.ExcludeConfig{}), transport, t.TempDir()), cfg
}

// runPieces runs an over-cap Generate over files and returns the cap it ran under.
func runPieces(t *testing.T, transport notes.Transport, files []pieceFile) int {
	t.Helper()
	gen, cfg := piecesGeneration(t, transport, files)
	if _, err := gen.Generate(t.Context(), "v1.0.0", cfg); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}
	return cfg.MaxPromptBytes
}

// pieceOf reads the piece number and total a piece prompt's framing names, ok false
// for a prompt that is not a piece.
func pieceOf(prompt string) (i, m int, ok bool) {
	match := pieceFraming.FindStringSubmatch(framingOf(prompt))
	if match == nil {
		return 0, 0, false
	}
	i, _ = strconv.Atoi(match[1])
	m, _ = strconv.Atoi(match[2])
	return i, m, true
}

// framingOf returns the block a chunk prompt carries just before its slice.
func framingOf(prompt string) string {
	end := strings.Index(prompt, "\n\ndiff --git ")
	if end < 0 {
		return ""
	}
	return prompt[strings.LastIndex(prompt[:end], "\n\n")+2 : end]
}

// piecesOf returns, in part order, the chunk prompts that are pieces of path.
func piecesOf(t *testing.T, ct *chunkTransport, path string) []string {
	t.Helper()
	var out []string
	for _, p := range chunksInPartOrder(t, ct) {
		if _, _, ok := pieceOf(p); ok && firstPath(sliceOf(p)) == path {
			out = append(out, p)
		}
	}
	return out
}

// pieceBodies returns each piece's slice with the file's header cut off, failing
// unless every slice opens with that header.
func pieceBodies(t *testing.T, pieces []string, header string) []string {
	t.Helper()
	var bodies []string
	for i, p := range pieces {
		body, ok := strings.CutPrefix(sliceOf(p), header)
		if !ok {
			t.Fatalf("piece %d does not open with the file's diff header:\n%.300s", i+1, sliceOf(p))
		}
		bodies = append(bodies, body)
	}
	return bodies
}

func TestGenerator_OverCap_FileOverChunkRoom_SentAsPieces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		diff string
	}{
		{name: "many hunks that each fit a piece", diff: hunkedFileDiff("big/huge.go", 12, 5_000)},
		{name: "one hunk bigger than a piece", diff: hunkedFileDiff("big/huge.go", 1, 60_000)},
		{name: "one line longer than a piece", diff: minifiedFileDiff("big/huge.go", 60_000, 0)},
		{name: "pieces enough for ten or more parts", diff: minifiedFileDiff("big/huge.go", 150_000, 0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ct := &chunkTransport{mergeBody: "merged notes"}
			files := []pieceFile{
				{"alpha/a.go", fileDiff("alpha/a.go", 10_000)},
				{"big/huge.go", tt.diff},
				{"gamma/c.go", fileDiff("gamma/c.go", 10_000)},
			}
			maxPromptBytes := runPieces(t, ct, files)

			pieces := piecesOf(t, ct, "big/huge.go")
			if len(pieces) < 2 {
				t.Fatalf("pieces of big/huge.go = %d, want the file sent as pieces", len(pieces))
			}
			for i, p := range pieces {
				if len(p) > maxPromptBytes {
					t.Errorf("piece %d is %d bytes, over the %d cap", i+1, len(p), maxPromptBytes)
				}
				if n := strings.Count(sliceOf(p), "diff --git "); n != 1 {
					t.Errorf("piece %d carries %d file headers, want only big/huge.go's", i+1, n)
				}
				gotI, gotM, _ := pieceOf(p)
				if gotI != i+1 || gotM != len(pieces) {
					t.Errorf("piece %d framing names piece %d of %d, want piece %d of %d", i+1, gotI, gotM, i+1, len(pieces))
				}
				if !strings.Contains(framingOf(p), "big/huge.go") {
					t.Errorf("piece %d framing does not name the file:\n%s", i+1, framingOf(p))
				}
			}
			header := pieceHeader("big/huge.go")
			if got := header + strings.Join(pieceBodies(t, pieces, header), ""); got != tt.diff {
				t.Errorf("the pieces joined, header kept once, do not reproduce the file's diff byte for byte")
			}
			parts := chunksInPartOrder(t, ct)
			if len(parts) != len(pieces)+2 {
				t.Fatalf("parts = %d, want alpha/a.go's chunk, %d pieces, then gamma/c.go's chunk", len(parts), len(pieces))
			}
			if sliceOf(parts[0]) != files[0].diff {
				t.Errorf("part 1 is not alpha/a.go's diff whole")
			}
			for i, p := range parts[1 : len(parts)-1] {
				if p != pieces[i] {
					t.Errorf("part %d is not piece %d of big/huge.go", i+2, i+1)
				}
			}
			if sliceOf(parts[len(parts)-1]) != files[2].diff {
				t.Errorf("part %d is not gamma/c.go's diff whole", len(parts))
			}
		})
	}
}

func TestGenerator_OverCap_PiecesOfHunksThatFit_CutOnlyBetweenHunks(t *testing.T) {
	t.Parallel()

	diff := hunkedFileDiff("big/huge.go", 12, 5_000)
	ct := &chunkTransport{mergeBody: "merged notes"}
	runPieces(t, ct, []pieceFile{{"big/huge.go", diff}})

	bodies := pieceBodies(t, piecesOf(t, ct, "big/huge.go"), pieceHeader("big/huge.go"))
	if len(bodies) < 2 {
		t.Fatalf("pieces = %d, want the file sent as pieces", len(bodies))
	}
	for i, body := range bodies {
		if !strings.HasPrefix(body, "@@ -") {
			t.Errorf("piece %d starts mid-hunk: %.80q", i+1, body)
		}
	}
	if limit := 2*len(diff)/pieceRoomOverFixed + 1; len(bodies) > limit {
		t.Errorf("pieces = %d, want at most %d: each piece filled before the next is started", len(bodies), limit)
	}
}

func TestGenerator_OverCap_HunkOverPieceRoom_CutBetweenLines(t *testing.T) {
	t.Parallel()

	diff := hunkedFileDiff("big/huge.go", 1, 60_000)
	ct := &chunkTransport{mergeBody: "merged notes"}
	runPieces(t, ct, []pieceFile{{"big/huge.go", diff}})

	bodies := pieceBodies(t, piecesOf(t, ct, "big/huge.go"), pieceHeader("big/huge.go"))
	if len(bodies) < 2 {
		t.Fatalf("pieces = %d, want the hunk cut into pieces", len(bodies))
	}
	for i, body := range bodies {
		if !strings.HasSuffix(body, "\n") {
			t.Errorf("piece %d ends mid-line: %.80q", i+1, body[max(len(body)-80, 0):])
		}
	}
	if limit := 2*len(diff)/pieceRoomOverFixed + 1; len(bodies) > limit {
		t.Errorf("pieces = %d, want at most %d: each piece filled before the next is started", len(bodies), limit)
	}
}

func TestGenerator_OverCap_LineOverPieceRoom_CutAtCharacterBoundary(t *testing.T) {
	t.Parallel()

	// Each lead shifts where the cuts land against the characters, so some lead puts
	// a byte-count cut inside a multi-byte character.
	for lead := range 8 {
		t.Run(fmt.Sprintf("lead %d", lead), func(t *testing.T) {
			t.Parallel()

			diff := minifiedFileDiff("big/huge.go", 60_000, lead)
			ct := &chunkTransport{mergeBody: "merged notes"}
			runPieces(t, ct, []pieceFile{{"big/huge.go", diff}})

			header := pieceHeader("big/huge.go")
			bodies := pieceBodies(t, piecesOf(t, ct, "big/huge.go"), header)
			if len(bodies) < 3 {
				t.Fatalf("pieces = %d, want the line cut across pieces", len(bodies))
			}
			for i, body := range bodies {
				if !utf8.ValidString(body) {
					t.Errorf("piece %d is cut inside a multi-byte character", i+1)
				}
			}
			if strings.HasSuffix(bodies[0], "\n") {
				t.Errorf("piece 1 ends at a line end, want the one long line cut inside it")
			}
			if header+strings.Join(bodies, "") != diff {
				t.Errorf("the pieces joined, header kept once, do not reproduce the file's diff byte for byte")
			}
		})
	}
}

// partTransport answers each chunk call with notes naming the part its framing
// names, so the merge shows which part's notes sit where.
type partTransport struct {
	waveTransport
}

func partNotesFor(part int) string {
	return fmt.Sprintf("- notes written for part %d", part)
}

func (pt *partTransport) Generate(ctx context.Context, prompt string) (string, error) {
	notes, err := pt.waveTransport.Generate(ctx, prompt)
	if err != nil || !isChunkPrompt(prompt) {
		return notes, err
	}
	return partNotesFor(partNumber(prompt)), nil
}

func TestGenerator_OverCap_PiecesRunAlongsideChunks_UnderTheBound_NotesInChunkOrder(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	pt := &partTransport{waveTransport{mergeBody: "merged", gate: heldGate(release)}}
	files := []pieceFile{
		{"alpha/a.go", fileDiff("alpha/a.go", 10_000)},
		{"big/huge.go", hunkedFileDiff("big/huge.go", 12, 5_000)},
		{"gamma/c.go", fileDiff("gamma/c.go", 10_000)},
	}
	gen, cfg := piecesGeneration(t, pt, files)
	done := make(chan generation, 1)
	go func() {
		body, err := gen.Generate(t.Context(), "v1.0.0", cfg)
		done <- generation{body: body, err: err}
	}()
	pt.waitInFlight(t, 4)
	close(release)
	awaitGeneration(t, done)

	_, maxInFlight, events, merges := pt.snapshot()
	parts := countPrefix(events, "start ")
	if parts < 6 {
		t.Fatalf("chunk calls = %d, want alpha's chunk, big/huge.go's pieces and gamma's chunk", parts)
	}
	if maxInFlight != 4 {
		t.Errorf("most chunk calls in flight at once = %d, want 4", maxInFlight)
	}
	if len(merges) != 1 {
		t.Fatalf("merge calls = %d, want one", len(merges))
	}
	last := -1
	for k := 1; k <= parts; k++ {
		at := strings.Index(merges[0], fmt.Sprintf("Notes for part %d of %d:\n\n%s", k, parts, partNotesFor(k)))
		if at < 0 {
			t.Fatalf("merge is missing part %d's notes under its heading", k)
		}
		if at < last {
			t.Errorf("merge carries part %d's notes out of chunk order", k)
		}
		last = at
	}
}

var _ notes.Transport = (*partTransport)(nil)
