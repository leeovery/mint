package notes_test

import (
	"slices"
	"testing"

	"mint/internal/notes"
)

// succeededLog records the sizes reported as succeeded diff-carrying calls. It holds
// no lock: the generator serialises its reports, so the race detector fails a run
// that does not.
type succeededLog struct {
	sizes []int
}

func (sl *succeededLog) PartsDone(int, int, int)       {}
func (sl *succeededLog) Halved(int, int)               {}
func (sl *succeededLog) Merging(int)                   {}
func (sl *succeededLog) Merged(int)                    {}
func (sl *succeededLog) CallSucceeded(promptBytes int) { sl.sizes = append(sl.sizes, promptBytes) }

func sortedSizes(prompts []string) []int {
	sizes := make([]int, len(prompts))
	for i, p := range prompts {
		sizes[i] = len(p)
	}
	slices.Sort(sizes)
	return sizes
}

func (s chunkScenario) generateReporting(t *testing.T, transport notes.Transport) *succeededLog {
	t.Helper()
	log := &succeededLog{}
	r := seedNormalPathGit(t, s.diff(), s.nameStatus(), s.numstat())
	gen := notes.NewGenerator(notes.NewAssembler(r, "/repo", notes.ExcludeConfig{}), transport, s.root).WithProgress(log)
	if _, err := gen.Generate(t.Context(), "v1.0.0", s.cfg); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}
	return log
}

func TestGenerator_SinglePassSucceeds_ReportsItsCallsSize(t *testing.T) {
	t.Parallel()

	s := underCapScenario(t, threeTenKBFiles())

	log := s.generateReporting(t, &chunkTransport{mergeBody: "notes"})

	if want := []int{len(s.singlePass(t))}; !slices.Equal(log.sizes, want) {
		t.Errorf("succeeded call sizes = %v, want the single-pass call's %v", log.sizes, want)
	}
}

func TestGenerator_SinglePassClassifiedSize_ReportsEachHalvedCallNotTheFailedOneTheClassifyOrTheMerge(t *testing.T) {
	t.Parallel()

	s := underCapScenario(t, threeTenKBFiles())
	ot := &overflowTransport{classifyAnswer: "SIZE"}

	log := s.generateReporting(t, ot)

	slices.Sort(log.sizes)
	if want := sortedSizes(ot.chunks); !slices.Equal(log.sizes, want) {
		t.Errorf("succeeded call sizes = %v, want exactly the halved chunk calls' %v", log.sizes, want)
	}
}

func TestGenerator_ChunkClassifiedSize_ReportsEverySucceededChunkCallHalvedOrNot(t *testing.T) {
	t.Parallel()

	s := capOneAreaPerChunk(t, areaFiles(6))
	ht := &halvingTransport{overflows: overflowsOnce(isSlice(areaDiff(3))), mergeBody: "merged"}

	log := s.generateReporting(t, ht)

	var succeeded []string
	for _, prompt := range ht.written {
		succeeded = append(succeeded, prompt)
	}
	if len(succeeded) < 7 {
		t.Fatalf("succeeded chunk calls = %d, want parts 1, 2, 4, 5, 6 and part 3's halved parts", len(succeeded))
	}
	slices.Sort(log.sizes)
	if want := sortedSizes(succeeded); !slices.Equal(log.sizes, want) {
		t.Errorf("succeeded call sizes = %v, want exactly the succeeded chunk calls' %v", log.sizes, want)
	}
}

func TestGenerator_PieceClassifiedSize_ReportsEverySucceededPieceCall(t *testing.T) {
	t.Parallel()

	pieces := 0
	ht := &halvingTransport{mergeBody: "merged"}
	ht.overflows = func(slice string) bool {
		if firstPath(slice) != "big/huge.go" {
			return false
		}
		pieces++
		return pieces == 2
	}
	log := &succeededLog{}
	gen, cfg := piecesGeneration(t, ht, []pieceFile{{"alpha/a.go", fileDiff("alpha/a.go", 5_000)}, {"big/huge.go", hunkedFileDiff("big/huge.go", 12, 5_000)}})

	if _, err := gen.WithProgress(log).Generate(t.Context(), "v1.0.0", cfg); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if len(ht.overflowed) != 1 {
		t.Fatalf("overflowed calls = %d, want one piece of big/huge.go", len(ht.overflowed))
	}
	var succeeded []string
	for _, prompt := range ht.written {
		succeeded = append(succeeded, prompt)
	}
	slices.Sort(log.sizes)
	if want := sortedSizes(succeeded); !slices.Equal(log.sizes, want) {
		t.Errorf("succeeded call sizes = %v, want exactly the succeeded chunk and piece calls' %v", log.sizes, want)
	}
}
