package notes_test

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/notes"
)

// mergeOverflowOutput is what a merge call that overflowed the model printed.
const mergeOverflowOutput = "Prompt is too long · the request is ~1300000 tokens (limit 1000000)"

// halvingTransport writes a chunked run in which some chunk calls overflow the model.
// A chunk call whose slice overflows reports true for fails with an overflow's output;
// every classify call is answered SIZE. Each chunk call that succeeds is held on
// release, when set, then answered with notes unique to it. The merge is answered
// mergeBody, or fails with mergeErr when set.
type halvingTransport struct {
	overflows func(slice string) bool
	release   <-chan struct{}
	mergeBody string
	mergeErr  error

	mu          sync.Mutex
	inFlight    int
	maxInFlight int
	// events logs "send N", "overflow N" and "end N" for chunk call N, counted from 0
	// in send order, and "classify" and "merge".
	events     []string
	sent       []string
	overflowed []int
	written    map[string]string
	merges     []string
	classify   int
}

func (ht *halvingTransport) Generate(_ context.Context, prompt string) (string, error) {
	switch {
	case isClassifyPrompt(prompt):
		ht.record(func() {
			ht.classify++
			ht.events = append(ht.events, "classify")
		})
		return "SIZE", nil
	case isChunkPrompt(prompt):
		return ht.chunk(prompt)
	default:
		ht.record(func() {
			ht.events = append(ht.events, "merge")
			ht.merges = append(ht.merges, prompt)
		})
		if ht.mergeErr != nil {
			return "", ht.mergeErr
		}
		return ht.mergeBody, nil
	}
}

func (ht *halvingTransport) chunk(prompt string) (string, error) {
	var call int
	var overflow bool
	ht.record(func() {
		call = len(ht.sent)
		ht.sent = append(ht.sent, prompt)
		ht.events = append(ht.events, fmt.Sprintf("send %d", call))
		overflow = ht.overflows(sliceOf(prompt))
		if overflow {
			ht.overflowed = append(ht.overflowed, call)
			ht.events = append(ht.events, fmt.Sprintf("overflow %d", call))
			return
		}
		ht.inFlight++
		ht.maxInFlight = max(ht.maxInFlight, ht.inFlight)
	})
	if overflow {
		return "", &ai.GenerationError{Stdout: overflowOutput + "\n", ExitCode: 1}
	}
	if ht.release != nil {
		select {
		case <-ht.release:
		case <-time.After(waitLimit):
		}
	}
	notes := fmt.Sprintf("- notes written by chunk call %d", call)
	ht.record(func() {
		ht.inFlight--
		ht.events = append(ht.events, fmt.Sprintf("end %d", call))
		if ht.written == nil {
			ht.written = map[string]string{}
		}
		ht.written[notes] = prompt
	})
	return notes, nil
}

func (ht *halvingTransport) record(change func()) {
	ht.mu.Lock()
	defer ht.mu.Unlock()
	change()
}

// waitHeld waits until n chunk calls are held in flight together, after one has
// overflowed.
func (ht *halvingTransport) waitHeld(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(waitLimit)
	for {
		var inFlight, overflowed int
		ht.record(func() { inFlight, overflowed = ht.inFlight, len(ht.overflowed) })
		if inFlight == n && overflowed > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("chunk calls held in flight = %d after %d overflows, never reached %d", inFlight, overflowed, n)
		}
		time.Sleep(time.Millisecond)
	}
}

// halvedFrom returns the chunk calls sent after call parent whose slices sit inside
// its slice: the calls its slice was re-cut into, and theirs.
func (ht *halvingTransport) halvedFrom(parent int) []string {
	parentSlice := sliceOf(ht.sent[parent])
	var out []string
	for _, p := range ht.sent[parent+1:] {
		if s := sliceOf(p); s != parentSlice && sliceWithin(parentSlice, s) {
			out = append(out, p)
		}
	}
	return out
}

// sliceWithin reports whether child carries only diff that parent carries: whole
// files of it, or a piece of one of its files under that file's header.
func sliceWithin(parent, child string) bool {
	if strings.Contains(parent, child) {
		return true
	}
	header := pieceHeader(firstPath(child))
	body, ok := strings.CutPrefix(child, header)
	return ok && strings.Contains(parent, header) && strings.Contains(parent, body)
}

var mergeSection = regexp.MustCompile(`(?m)^Notes for part (\d+) of (\d+):\n\n(.*)$`)

// mergedCalls returns the chunk prompts whose notes the merge carries, in the order
// it carries them.
func (ht *halvingTransport) mergedCalls(t *testing.T) []string {
	t.Helper()
	if len(ht.merges) != 1 {
		t.Fatalf("merge calls = %d, want one", len(ht.merges))
	}
	var calls []string
	for _, m := range mergeSection.FindAllStringSubmatch(ht.merges[0], -1) {
		prompt, ok := ht.written[m[3]]
		if !ok {
			t.Fatalf("merge carries notes %q no chunk call wrote", m[3])
		}
		calls = append(calls, prompt)
	}
	return calls
}

// hasRunJoiningTo reports whether two or more consecutive bodies join to want.
func hasRunJoiningTo(bodies []string, want string) bool {
	for start := range bodies {
		joined := ""
		for end := start; end < len(bodies) && len(joined) < len(want); end++ {
			joined += bodies[end]
			if joined == want && end > start {
				return true
			}
		}
	}
	return false
}

// inPartOrder returns chunk prompts ordered by the part each names.
func inPartOrder(t *testing.T, prompts []string) []string {
	t.Helper()
	return chunksInPartOrder(t, &chunkTransport{prompts: prompts})
}

// joinedSlices joins the prompts' slices, keeping a file's header once across its
// consecutive pieces.
func joinedSlices(prompts []string) string {
	var b strings.Builder
	previous := ""
	for _, p := range prompts {
		slice := sliceOf(p)
		path := firstPath(slice)
		_, _, piece := pieceOf(p)
		if piece && path == previous {
			slice = strings.TrimPrefix(slice, pieceHeader(path))
		}
		previous = ""
		if piece {
			previous = path
		}
		b.WriteString(slice)
	}
	return b.String()
}

// areaFiles is n areas of four 10 KB files each.
func areaFiles(n int) []chunkFile {
	var files []chunkFile
	for area := 1; area <= n; area++ {
		for _, name := range []string{"a", "b", "c", "d"} {
			files = append(files, chunkFile{path: fmt.Sprintf("area%d/%s.go", area, name), size: 10_000})
		}
	}
	return files
}

// areaDiff is the diff of one area's four files, the slice its chunk carries.
func areaDiff(area int) string {
	var b strings.Builder
	for _, name := range []string{"a", "b", "c", "d"} {
		b.WriteString(fileDiff(fmt.Sprintf("area%d/%s.go", area, name), 10_000))
	}
	return b.String()
}

// capOneAreaPerChunk is a scenario over files whose cap packs each area's four files
// into one chunk.
func capOneAreaPerChunk(t *testing.T, files []chunkFile) chunkScenario {
	t.Helper()
	s := chunkScenario{files: files, root: t.TempDir(), cfg: config.Config{MaxPromptBytes: config.DefaultMaxPromptBytes}}
	s.cfg.MaxPromptBytes = s.fixedPart(t) + 45_000
	return s
}

// overflowsOnce overflows each slice match reports true for, the first time it is sent.
func overflowsOnce(match func(slice string) bool) func(string) bool {
	seen := map[string]bool{}
	return func(slice string) bool {
		if !match(slice) || seen[slice] {
			return false
		}
		seen[slice] = true
		return true
	}
}

func isSlice(want string) func(string) bool {
	return func(slice string) bool { return slice == want }
}

func (s chunkScenario) generateWith(t *testing.T, transport notes.Transport) (string, error) {
	t.Helper()
	return awaitResult(t, s.startGenerate(t, transport))
}

func awaitResult(t *testing.T, done <-chan generation) (string, error) {
	t.Helper()
	select {
	case g := <-done:
		return g.body, g.err
	case <-time.After(waitLimit):
		t.Fatal("Generate did not finish")
		return "", nil
	}
}

func TestGenerator_ChunkClassifiedSize_SliceResentAsHalvedCallsCarryingTheReleaseAndEveryFileOnce(t *testing.T) {
	t.Parallel()

	s := capOneAreaPerChunk(t, areaFiles(6))
	s.cfg.Release.Context = "Our users are operators."
	head := s.instructions(t) + "\n\n" + s.changeMap(t) + "\n\n"
	ht := &halvingTransport{overflows: overflowsOnce(isSlice(areaDiff(3))), mergeBody: "merged"}

	if _, err := s.generateWith(t, ht); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if len(ht.overflowed) != 1 || ht.classify != 1 {
		t.Fatalf("overflowed calls = %d, classify calls = %d, want part 3's one of each", len(ht.overflowed), ht.classify)
	}
	failed := ht.sent[ht.overflowed[0]]
	if k, n := partOf(t, failed); k != 3 || n != 6 {
		t.Fatalf("the overflowed call names part %d of %d, want part 3 of 6", k, n)
	}
	halved := ht.halvedFrom(ht.overflowed[0])
	if len(halved) < 2 {
		t.Fatalf("calls re-cut from part 3 = %d, want its slice re-sent in smaller calls", len(halved))
	}
	halved = inPartOrder(t, halved)
	for i, p := range halved {
		if 2*len(p) > len(failed) {
			t.Errorf("halved call %d is %d bytes, over half the failed call's %d", i+1, len(p), len(failed))
		}
		if !strings.HasPrefix(p, head) {
			t.Errorf("halved call %d does not open with the release's instructions then the whole-release Change Map", i+1)
		}
		if !strings.HasSuffix(p, "\n\n"+notes.OutputReminder) {
			t.Errorf("halved call %d does not end with the closing reminder", i+1)
		}
		if framing := framingOf(p); !partFraming.MatchString(framing) {
			t.Errorf("halved call %d carries no framing naming its part:\n%s", i+1, framing)
		}
	}
	if joinedSlices(halved) != areaDiff(3) {
		t.Errorf("the halved calls' slices joined in part order are not part 3's files' diffs, each once")
	}
}

func TestGenerator_ChunkClassifiedSize_WaveCarriesOnAndEveryOtherPartIsSentAndFinishes(t *testing.T) {
	t.Parallel()

	s := capOneAreaPerChunk(t, areaFiles(8))
	release := make(chan struct{})
	ht := &halvingTransport{overflows: overflowsOnce(isSlice(areaDiff(3))), release: release, mergeBody: "merged"}

	done := s.startGenerate(t, ht)
	// Parts 1, 2 and 4 are held in flight and parts 6 to 8 not yet started when part 3
	// is classified SIZE; part 5 takes the slot part 3 left.
	ht.waitHeld(t, 4)
	close(release)
	body, err := awaitResult(t, done)

	if err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}
	if body != "merged" {
		t.Errorf("body = %q, want the merge output", body)
	}
	for _, area := range []int{1, 2, 4, 5, 6, 7, 8} {
		sends := 0
		for _, p := range ht.sent {
			if sliceOf(p) == areaDiff(area) {
				sends++
			}
		}
		if sends != 1 {
			t.Errorf("part %d's slice was sent %d times, want once", area, sends)
		}
	}
	if len(ht.overflowed) != 1 {
		t.Errorf("overflowed calls = %d, want only part 3's", len(ht.overflowed))
	}
}

func TestGenerator_ChunkClassifiedSize_HalvedPartsCountTowardFourInFlight(t *testing.T) {
	t.Parallel()

	s := capOneAreaPerChunk(t, areaFiles(6))
	release := make(chan struct{})
	ht := &halvingTransport{overflows: overflowsOnce(isSlice(areaDiff(1))), release: release, mergeBody: "merged"}

	done := s.startGenerate(t, ht)
	ht.waitHeld(t, 4)
	close(release)
	if _, err := awaitResult(t, done); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if halved := ht.halvedFrom(ht.overflowed[0]); len(halved) < 2 {
		t.Fatalf("calls re-cut from part 1 = %d, want its halved parts sent", len(halved))
	}
	if ht.maxInFlight != 4 {
		t.Errorf("most chunk calls in flight at once = %d, want 4, halved parts included", ht.maxInFlight)
	}
}

func TestGenerator_PieceClassifiedSize_RecutIntoSmallerPiecesUnderTheFilesHeader(t *testing.T) {
	t.Parallel()

	big := hunkedFileDiff("big/huge.go", 12, 5_000)
	// The second of big/huge.go's pieces to be sent overflows.
	pieces := 0
	ht := &halvingTransport{mergeBody: "merged"}
	ht.overflows = func(slice string) bool {
		if firstPath(slice) != "big/huge.go" {
			return false
		}
		pieces++
		return pieces == 2
	}
	gen, cfg := piecesGeneration(t, ht, []pieceFile{{"alpha/a.go", fileDiff("alpha/a.go", 5_000)}, {"big/huge.go", big}})

	if _, err := gen.Generate(t.Context(), "v1.0.0", cfg); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if len(ht.overflowed) != 1 {
		t.Fatalf("overflowed calls = %d, want one piece", len(ht.overflowed))
	}
	failed := ht.sent[ht.overflowed[0]]
	if _, _, ok := pieceOf(failed); !ok {
		t.Fatalf("the overflowed call is not a piece:\n%s", framingOf(failed))
	}
	header := pieceHeader("big/huge.go")
	halved := ht.halvedFrom(ht.overflowed[0])
	if len(halved) < 2 {
		t.Fatalf("calls re-cut from the piece = %d, want smaller pieces", len(halved))
	}
	halved = inPartOrder(t, halved)
	for i, p := range halved {
		if _, _, ok := pieceOf(p); !ok {
			t.Errorf("re-cut call %d is not a piece of big/huge.go", i+1)
		}
		if 2*len(p) > len(failed) {
			t.Errorf("re-cut piece %d is %d bytes, over half the failed piece's %d", i+1, len(p), len(failed))
		}
	}
	failedBody := strings.TrimPrefix(sliceOf(failed), header)
	if got := strings.Join(pieceBodies(t, halved, header), ""); got != failedBody {
		t.Errorf("the re-cut pieces joined, header kept once, do not reproduce the failed piece's diff byte for byte")
	}
	failedPiece, originalPieces, _ := pieceOf(failed)
	grown := originalPieces - 1 + len(halved)
	for i, p := range halved {
		if gotI, gotM, _ := pieceOf(p); gotI != failedPiece+i || gotM != grown {
			t.Errorf("re-cut piece %d names piece %d of %d, want piece %d of %d", i+1, gotI, gotM, failedPiece+i, grown)
		}
	}
}

func TestGenerator_TwoPiecesOfOneFileClassifiedSize_EachRecutUnderTheFilesHeader(t *testing.T) {
	t.Parallel()

	big := hunkedFileDiff("big/huge.go", 12, 5_000)
	header := pieceHeader("big/huge.go")
	// The first two of big/huge.go's pieces to be sent overflow.
	seen := map[string]bool{}
	ht := &halvingTransport{mergeBody: "merged"}
	ht.overflows = func(slice string) bool {
		if firstPath(slice) != "big/huge.go" || seen[slice] || len(seen) == 2 {
			return false
		}
		seen[slice] = true
		return true
	}
	gen, cfg := piecesGeneration(t, ht, []pieceFile{{"alpha/a.go", fileDiff("alpha/a.go", 5_000)}, {"big/huge.go", big}})

	body, err := gen.Generate(t.Context(), "v1.0.0", cfg)
	if err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if body != "merged" {
		t.Errorf("body = %q, want the merge output", body)
	}
	if len(ht.overflowed) != 2 || ht.classify != 2 {
		t.Fatalf("overflowed calls = %d, classify calls = %d, want two of each", len(ht.overflowed), ht.classify)
	}
	merged := ht.mergedCalls(t)
	bodies := pieceBodies(t, merged[1:], header)
	for _, call := range ht.overflowed {
		if !hasRunJoiningTo(bodies, strings.TrimPrefix(sliceOf(ht.sent[call]), header)) {
			t.Errorf("no run of two or more merged pieces, in chunk order, reproduces overflowed call %d's diff byte for byte", call)
		}
	}
	if got := joinedSlices(merged[1:]); got != big {
		t.Errorf("the merge's pieces of big/huge.go, in chunk order, do not reproduce the file's diff")
	}
	for i, p := range merged[1:] {
		if _, gotM, _ := pieceOf(p); gotM == 0 {
			t.Errorf("merged part %d is not a piece of big/huge.go", i+2)
		}
	}
}

func TestGenerator_HalvedPartClassifiedSize_HalvedAgainAfterThatAnswer(t *testing.T) {
	t.Parallel()

	s := capOneAreaPerChunk(t, areaFiles(6))
	part3 := areaDiff(3)
	overflowed := 0
	ht := &halvingTransport{mergeBody: "merged"}
	// Part 3 overflows, then the first call cut from it.
	ht.overflows = func(slice string) bool {
		if overflowed < 2 && strings.HasPrefix(part3, slice) {
			overflowed++
			return true
		}
		return false
	}

	if _, err := s.generateWith(t, ht); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if len(ht.overflowed) != 2 || ht.classify != 2 {
		t.Fatalf("overflowed calls = %d, classify calls = %d, want two of each", len(ht.overflowed), ht.classify)
	}
	second := ht.overflowed[1]
	if !slices.Contains(ht.halvedFrom(ht.overflowed[0]), ht.sent[second]) {
		t.Fatalf("the second overflow is not one of part 3's halved parts")
	}
	again := inPartOrder(t, ht.halvedFrom(second))
	if len(again) < 2 {
		t.Fatalf("calls re-cut from the halved part = %d, want it halved in turn", len(again))
	}
	overflowAt := slices.Index(ht.events, fmt.Sprintf("overflow %d", second))
	answerAt := overflowAt + slices.Index(ht.events[overflowAt:], "classify")
	for i, p := range again {
		if 2*len(p) > len(ht.sent[second]) {
			t.Errorf("further-round call %d is %d bytes, over half the halved part's own %d", i+1, len(p), len(ht.sent[second]))
		}
		call := slices.Index(ht.sent, p)
		if at := slices.Index(ht.events, fmt.Sprintf("send %d", call)); at < answerAt {
			t.Errorf("further-round call %d was sent before the halved part's SIZE answer", i+1)
		}
	}
	if joinedSlices(again) != sliceOf(ht.sent[second]) {
		t.Errorf("the further round's slices joined are not the halved part's slice, each file once")
	}
}

func TestGenerator_HalvedRunSucceeds_OneMergeCarryingEveryPartInChunkOrderShippedByteIdentical(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		overflows func(part3 string) func(string) bool
	}{
		{"part 3 halved once", func(part3 string) func(string) bool { return overflowsOnce(isSlice(part3)) }},
		{"a halved part of part 3 halved again", func(part3 string) func(string) bool {
			overflowed := 0
			return func(slice string) bool {
				if overflowed < 2 && strings.HasPrefix(part3, slice) {
					overflowed++
					return true
				}
				return false
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := capOneAreaPerChunk(t, areaFiles(6))
			ht := &halvingTransport{overflows: tc.overflows(areaDiff(3)), mergeBody: halvedMergeBody}

			body, err := s.generateWith(t, ht)
			if err != nil {
				t.Fatalf("Generate returned unexpected error: %v", err)
			}

			if body != halvedMergeBody {
				t.Errorf("body = %q, want the merge output byte-identical %q", body, halvedMergeBody)
			}
			merged := ht.mergedCalls(t)
			if len(merged) != len(ht.written) {
				t.Errorf("merge carries %d parts' notes, want every one of the %d successful chunk calls'", len(merged), len(ht.written))
			}
			if len(merged) < 7 {
				t.Fatalf("merge carries %d parts' notes, want parts 1, 2, 4, 5, 6 and part 3's halved parts", len(merged))
			}
			for i, area := range []int{1, 2} {
				if sliceOf(merged[i]) != areaDiff(area) {
					t.Errorf("merge part %d is not part %d's notes", i+1, area)
				}
			}
			tail := merged[len(merged)-3:]
			for i, area := range []int{4, 5, 6} {
				if sliceOf(tail[i]) != areaDiff(area) {
					t.Errorf("merge part %d is not part %d's notes", len(merged)-3+i+1, area)
				}
			}
			if joinedSlices(merged[2:len(merged)-3]) != areaDiff(3) {
				t.Errorf("the notes between parts 2 and 4 are not part 3's halved parts, in chunk order")
			}
			for k, m := range mergeSection.FindAllStringSubmatch(ht.merges[0], -1) {
				if m[1] != fmt.Sprint(k+1) || m[2] != fmt.Sprint(len(merged)) {
					t.Errorf("merge section %d is headed part %s of %s, want part %d of %d", k+1, m[1], m[2], k+1, len(merged))
				}
			}
		})
	}
}

func TestGenerator_MergeClassifiedSize_FailsAsTooLargeToMergeWithNoFurtherCall(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(3))
	ht := &halvingTransport{
		overflows: func(string) bool { return false },
		mergeErr:  &ai.GenerationError{Stdout: mergeOverflowOutput + "\n", ExitCode: 1},
	}

	_, err := s.generateWith(t, ht)

	if err == nil {
		t.Fatal("Generate returned nil error, want a failed merge")
	}
	if want := []string{"merge", "classify"}; !slices.Equal(ht.events[len(ht.events)-2:], want) {
		t.Errorf("events end %v, want the merge then its classify call and nothing after", ht.events[len(ht.events)-2:])
	}
	if len(ht.sent) != 3 || len(ht.merges) != 1 || ht.classify != 1 {
		t.Errorf("chunk calls = %d, merge calls = %d, classify calls = %d; want 3, 1, 1", len(ht.sent), len(ht.merges), ht.classify)
	}
	if got, _ := notes.CauseText(err); got != "part notes too large to merge" {
		t.Errorf("CauseText = %q, want %q with no location", got, "part notes too large to merge")
	}
	if !errors.Is(err, notes.ErrMergeTooLarge) {
		t.Errorf("error = %v, want it to match ErrMergeTooLarge", err)
	}
	var genErr *ai.GenerationError
	if !errors.As(err, &genErr) || genErr.Output() != mergeOverflowOutput {
		t.Errorf("error = %v, want it to carry the merge call's output verbatim", err)
	}
}
