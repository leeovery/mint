package notes_test

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"mint/internal/notes"
)

// notesMissing is the line a saved chunk-notes file opens with when files went
// without notes, the files listed as given.
func notesMissing(files string) string {
	return "**Notes missing:** no notes were written for " + files + " — add them before releasing.\n\n"
}

func TestGenerator_WaveFailsForGoodAtItsLastPart_SavedNotesOpenWithTheMissingFiles(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, []chunkFile{
		{"admin/users.go", 10_000},
		{"agent/run.go", 10_000},
		{"api/handler.go", 5_000},
		{"api/routes.go", 5_000},
	})
	var mu sync.Mutex
	parts := map[string]string{}
	transport := transportFunc(func(_ context.Context, prompt string) (string, error) {
		switch {
		case isClassifyPrompt(prompt):
			return "OTHER", nil
		case isChunkPrompt(prompt):
			path := firstPath(sliceOf(prompt))
			mu.Lock()
			parts[path] = partFraming.FindString(prompt)
			mu.Unlock()
			if path == "api/handler.go" {
				return "", failedWith("API Error: usage limit reached for this account")
			}
			return chunkNotesFor(path), nil
		default:
			return "merged", nil
		}
	})

	ns, err := s.generateSaving(t, transport)

	if got := parts["api/handler.go"]; got != "part 3 of 3" {
		t.Fatalf("fixture: api/handler.go framed as %q, want part 3 of 3 with api/routes.go", got)
	}
	ns.assertSavedOnce(t, err, notesMissing("`api/handler.go`, `api/routes.go`")+joinedNotes("admin/users.go", "agent/run.go"))
}

func TestGenerator_WaveEnds_MissingFilesIncludeUnstartedPartsAndCallsThatFailedAfterTheEnd(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(6))
	release := make(chan struct{})
	et := &endingTransport{classify: answerBy()}
	et.call = func(path string) error {
		switch path {
		case area(1):
			if err := et.waitUntil("start "+area(2), "start "+area(3), "start "+area(4)); err != nil {
				return err
			}
			return failedWith("area1: usage limit reached")
		case area(4):
			return heldUntil(release, failedWith("area4: usage limit reached"))
		default:
			return heldUntil(release, nil)
		}
	}

	done, ns := s.startSaving(t, t.Context(), et)
	et.waitFor(t, "classify "+area(1))
	time.Sleep(recordGrace)
	close(release)
	err := awaitFailure(t, done)

	if n := et.count("start " + area(5)); n != 0 {
		t.Fatalf("fixture: area 5's part started %d times, want never", n)
	}
	want := notesMissing("`area1/file.go`, `area4/file.go`, `area5/file.go`, `area6/file.go`") + joinedNotes(area(2), area(3))
	ns.assertSavedOnce(t, err, want)
}

// pieceOutcomeTransport answers a chunk call by its slice's first file and piece
// number through fail, nil for notes, and a classify call OTHER.
func pieceOutcomeTransport(fail func(path string, piece int) bool) notes.Transport {
	return transportFunc(func(_ context.Context, prompt string) (string, error) {
		switch {
		case isClassifyPrompt(prompt):
			return "OTHER", nil
		case isChunkPrompt(prompt):
			path := firstPath(sliceOf(prompt))
			piece, _, _ := pieceOf(prompt)
			if fail(path, piece) {
				return "", failedWith("API Error: usage limit reached for this account")
			}
			return chunkNotesFor(path) + " piece " + strconv.Itoa(piece), nil
		default:
			return "merged", nil
		}
	})
}

func TestGenerator_FileSentAsPiecesPartlyFinished_IsListedPartly(t *testing.T) {
	t.Parallel()

	ns := &noteSaver{}
	transport := pieceOutcomeTransport(func(path string, piece int) bool {
		return path == "src/partly.go" && piece == 2 || path == "src/unwritten.go"
	})
	gen, cfg := piecesGeneration(t, transport, []pieceFile{
		{path: "src/partly.go", diff: hunkedFileDiff("src/partly.go", 2, 10_000)},
		{path: "src/unwritten.go", diff: hunkedFileDiff("src/unwritten.go", 2, 10_000)},
	})

	_, err := gen.WithChunkNotes(ns.save).Generate(t.Context(), "v1.0.0", cfg)

	ns.assertSavedOnce(t, err, notesMissing("`src/partly.go` (partly), `src/unwritten.go`")+chunkNotesFor("src/partly.go")+" piece 1")
}

func TestGenerator_SliceHalved_ListsOnlyTheFilesWhoseHalvedPartsDidNotFinish(t *testing.T) {
	t.Parallel()

	s := capOneAreaPerChunk(t, areaFiles(3))
	overflows := overflowsOnce(isSlice(areaDiff(2)))
	var mu sync.Mutex
	sp := &succeededPaths{}
	dStart := make(chan struct{})
	dStarted := sync.OnceFunc(func() { close(dStart) })
	transport := transportFunc(func(_ context.Context, prompt string) (string, error) {
		switch {
		case isClassifyPrompt(prompt):
			return answerBy(overflowOutput)(prompt)
		case isChunkPrompt(prompt):
			slice := sliceOf(prompt)
			mu.Lock()
			overflow := overflows(slice)
			mu.Unlock()
			if overflow {
				return "", failedWith(overflowOutput)
			}
			if strings.Contains(slice, "a/area2/d.go ") {
				dStarted()
			}
			if strings.Contains(slice, "a/area2/c.go ") {
				if err := waitClosed(dStart); err != nil {
					return "", err
				}
				return "", failedWith("API Error: usage limit reached for this account")
			}
			return sp.add(firstPath(slice)), nil
		default:
			return "merged", nil
		}
	})

	ns, err := s.generateSaving(t, transport)

	order := sp.inChunkOrder()
	if want := []string{"area1/a.go", "area2/a.go", "area2/b.go", "area2/d.go", "area3/a.go"}; !slices.Equal(order, want) {
		t.Fatalf("fixture: chunk calls that returned notes open with %v, want %v: area 2 halved a file to a part", order, want)
	}
	ns.assertSavedOnce(t, err, notesMissing("`area2/c.go`")+joinedNotes(order...))
}

func TestGenerator_FileTheReCutSplitIntoPiecesPartlyFinished_IsListedPartly(t *testing.T) {
	t.Parallel()

	s := capOneAreaPerChunk(t, []chunkFile{{"area1/a.go", 10_000}, {"big/one.go", 30_000}, {"zeta/z.go", 10_000}})
	var mu sync.Mutex
	overflowed := false
	pieces := map[int]bool{}
	transport := transportFunc(func(_ context.Context, prompt string) (string, error) {
		switch {
		case isClassifyPrompt(prompt):
			return answerBy(overflowOutput)(prompt)
		case isChunkPrompt(prompt):
			slice := sliceOf(prompt)
			mu.Lock()
			overflow := !overflowed && strings.Contains(slice, "diff --git a/big/one.go ")
			overflowed = overflowed || overflow
			mu.Unlock()
			if overflow {
				return "", failedWith(overflowOutput)
			}
			if piece, _, ok := pieceOf(prompt); ok {
				mu.Lock()
				pieces[piece] = true
				mu.Unlock()
				if piece == 2 {
					return "", failedWith("API Error: usage limit reached for this account")
				}
				return "- big piece " + strconv.Itoa(piece), nil
			}
			return chunkNotesFor(firstPath(slice)), nil
		default:
			return "merged", nil
		}
	})

	ns, err := s.generateSaving(t, transport)

	if len(pieces) != 2 {
		t.Fatalf("fixture: big/one.go's re-cut sent pieces %v, want it cut into two", pieces)
	}
	ns.assertSavedOnce(t, err, notesMissing("`big/one.go` (partly)")+chunkNotesFor("area1/a.go")+"\n\n- big piece 1\n\n"+chunkNotesFor("zeta/z.go"))
}

func TestGenerator_EveryPartFinishedAndTheMergeFailed_SavedNotesCarryNoMarker(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(3))
	transport := transportFunc(func(_ context.Context, prompt string) (string, error) {
		switch {
		case isClassifyPrompt(prompt):
			return "OTHER", nil
		case isChunkPrompt(prompt):
			return chunkNotesFor(firstPath(sliceOf(prompt))), nil
		default:
			return "", failedWith("Invalid API key · Please run /login")
		}
	})

	ns, err := s.generateSaving(t, transport)

	ns.assertSavedOnce(t, err, joinedNotes(area(1), area(2), area(3)))
}

func TestGenerator_PrefixedHeaders_MarkerFramingAndOrderNameTheRepoPath(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var chunks []string
	fail := pieceOutcomeTransport(func(path string, piece int) bool {
		return path == "api/handler.go" && piece == 2
	})
	transport := transportFunc(func(ctx context.Context, prompt string) (string, error) {
		if isChunkPrompt(prompt) {
			mu.Lock()
			chunks = append(chunks, prompt)
			mu.Unlock()
		}
		return fail.Generate(ctx, prompt)
	})
	ns := &noteSaver{}
	gen, cfg := piecesGeneration(t, transport, []pieceFile{
		{path: "zeta/z.go", diff: fileDiff("zeta/z.go", 1_000)},
		{path: "api/handler.go", diff: hunkedFileDiff("api/handler.go", 2, 10_000)},
	})

	_, err := gen.WithChunkNotes(ns.save).Generate(t.Context(), "v1.0.0", cfg)

	saves := ns.saved()
	if len(saves) != 1 || !strings.HasPrefix(saves[0], notesMissing("`api/handler.go` (partly)")) {
		t.Fatalf("saved chunk notes = %q, want once opening with %q", saves, notesMissing("`api/handler.go` (partly)"))
	}
	if got := notes.SavedChunkNotesPath(err); got != savedNotesPath {
		t.Errorf("SavedChunkNotesPath = %q, want %q", got, savedNotesPath)
	}
	partOfFile := map[string][]int{}
	for _, c := range chunks {
		path := firstPath(sliceOf(c))
		k, _ := partOf(t, c)
		partOfFile[path] = append(partOfFile[path], k)
		if _, _, ok := pieceOf(c); ok && !strings.Contains(framingOf(c), "of the diff for api/handler.go, a file") {
			t.Errorf("piece of %s framed without its repo path:\n%s", path, framingOf(c))
		}
	}
	if len(partOfFile["api/handler.go"]) != 2 || len(partOfFile["zeta/z.go"]) != 1 {
		t.Fatalf("fixture: parts per file = %v, want two api/handler.go pieces and one zeta/z.go part", partOfFile)
	}
	for _, k := range partOfFile["api/handler.go"] {
		if k > partOfFile["zeta/z.go"][0] {
			t.Errorf("api/handler.go piece in part %d after zeta/z.go's part %d, want path order", k, partOfFile["zeta/z.go"][0])
		}
	}
}
