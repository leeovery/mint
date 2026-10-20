package notes_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/notes"
)

// overflowOutput is what the single-pass call that overflowed the model printed.
const overflowOutput = "Prompt is too long · the request is ~1162407 tokens (limit 1000000)"

// halvedMergeBody is the merge output a halved run ships, trailing whitespace and all.
const halvedMergeBody = "✨ Added\n- Everything the parts wrote, merged   \n\n"

// overflowTransport fails the single-pass call with its captured output, answers a
// classify call with classifyAnswer (or classifyErr), a chunk call with notes naming
// its slice's first file, and the merge with halvedMergeBody.
type overflowTransport struct {
	classifyAnswer string
	classifyErr    error

	mu       sync.Mutex
	single   []string
	classify []string
	chunks   []string
	merges   []string
}

func (ot *overflowTransport) Generate(_ context.Context, prompt string) (string, error) {
	ot.mu.Lock()
	defer ot.mu.Unlock()
	switch {
	case isClassifyPrompt(prompt):
		ot.classify = append(ot.classify, prompt)
		return ot.classifyAnswer, ot.classifyErr
	case isChunkPrompt(prompt):
		ot.chunks = append(ot.chunks, prompt)
		return chunkNotesFor(firstPath(sliceOf(prompt))), nil
	case strings.Contains(prompt, "diff --git "):
		ot.single = append(ot.single, prompt)
		return "", &ai.GenerationError{Stdout: overflowOutput + "\n", ExitCode: 1}
	default:
		ot.merges = append(ot.merges, prompt)
		return halvedMergeBody, nil
	}
}

// chunksInOrder returns the chunk prompts ordered by the part each names.
func (ot *overflowTransport) chunksInOrder(t *testing.T) []string {
	t.Helper()
	return chunksInPartOrder(t, &chunkTransport{prompts: ot.chunks})
}

// underCapScenario is files under the default cap, so their notes go in one call.
func underCapScenario(t *testing.T, files []chunkFile) chunkScenario {
	t.Helper()
	s := chunkScenario{files: files, root: t.TempDir(), cfg: normalCfg()}
	if size := len(s.singlePass(t)); size > s.cfg.MaxPromptBytes {
		t.Fatalf("fixture: single-pass prompt is %d bytes, over the %d cap", size, s.cfg.MaxPromptBytes)
	}
	return s
}

func (s chunkScenario) generateOver(t *testing.T, transport notes.Transport) (string, error) {
	t.Helper()
	r := seedNormalPathGit(t, s.diff(), s.nameStatus(), s.numstat())
	gen := notes.NewGenerator(notes.NewAssembler(r, notes.ExcludeConfig{}), transport, s.root)
	return gen.Generate(t.Context(), "v1.0.0", s.cfg)
}

func TestGenerator_SinglePassClassifiedSize_WrittenByHalvedChunkCallsThenOneMerge(t *testing.T) {
	t.Parallel()

	for _, answer := range []string{"SIZE", " size ", `"SIZE"`, "`Size`", "SIZE.", "SIZE\n", "'size'"} {
		t.Run(answer, func(t *testing.T) {
			t.Parallel()

			s := underCapScenario(t, threeTenKBFiles())
			failedSize := len(s.singlePass(t))
			ot := &overflowTransport{classifyAnswer: answer}

			body, err := s.generateOver(t, ot)
			if err != nil {
				t.Fatalf("Generate returned unexpected error: %v", err)
			}

			if len(ot.single) != 1 {
				t.Errorf("single-pass calls = %d, want only the one that failed", len(ot.single))
			}
			if len(ot.classify) != 1 {
				t.Errorf("classify calls = %d, want one", len(ot.classify))
			}
			if len(ot.chunks) < 2 {
				t.Fatalf("chunk calls = %d, want the notes written by chunk calls", len(ot.chunks))
			}
			for i, p := range ot.chunks {
				if 2*len(p) > failedSize {
					t.Errorf("chunk call %d is %d bytes, over half the failed call's %d", i, len(p), failedSize)
				}
			}
			if len(ot.merges) != 1 {
				t.Errorf("merge calls = %d, want one", len(ot.merges))
			}
			if body != halvedMergeBody {
				t.Errorf("body = %q, want the merge output byte-identical %q", body, halvedMergeBody)
			}
		})
	}
}

func TestGenerator_SinglePassClassifiedSize_EachHalvedCallCarriesTheReleaseAndEveryFileOnce(t *testing.T) {
	t.Parallel()

	// git emitted the files out of path order; the halved run packs them in it.
	files := []chunkFile{{"core/c.go", 6_000}, {"api/a.go", 6_000}, {"ext/e.go", 6_000}, {"bin/b.go", 6_000}, {"docs/d.go", 6_000}}
	s := underCapScenario(t, files)
	s.cfg.Release.Context = "Our users are operators."
	failedSize := len(s.singlePass(t))
	instructions := s.instructions(t)
	changeMap := s.changeMap(t)
	ot := &overflowTransport{classifyAnswer: "SIZE"}

	if _, err := s.generateOver(t, ot); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	chunks := ot.chunksInOrder(t)
	if len(chunks) < 2 {
		t.Fatalf("chunk calls = %d, want a chunked run", len(chunks))
	}
	var slices strings.Builder
	head := instructions + "\n\n" + changeMap + "\n\n"
	for i, p := range chunks {
		k, n := partOf(t, p)
		if k != i+1 || n != len(chunks) {
			t.Errorf("chunk %d names part %d of %d, want part %d of %d", i, k, n, i+1, len(chunks))
		}
		if !strings.HasPrefix(p, head) {
			t.Errorf("part %d does not open with the release's instructions then the whole-release Change Map", k)
		}
		if !strings.HasSuffix(p, "\n\n"+notes.OutputReminder) {
			t.Errorf("part %d does not end with the closing reminder", k)
		}
		if 2*len(p) > failedSize {
			t.Errorf("part %d is %d bytes, over half the failed call's %d", k, len(p), failedSize)
		}
		slices.WriteString(sliceOf(p))
	}
	want := fileDiff("api/a.go", 6_000) + fileDiff("bin/b.go", 6_000) + fileDiff("core/c.go", 6_000) +
		fileDiff("docs/d.go", 6_000) + fileDiff("ext/e.go", 6_000)
	if slices.String() != want {
		t.Errorf("the parts' slices joined in part order are not every file's diff once, in path order")
	}
}

func TestGenerator_SinglePassClassifiedSize_FileOverHalvedRoomSentAsPieces(t *testing.T) {
	t.Parallel()

	big := hunkedFileDiff("big/huge.go", 12, 5_000)
	files := []pieceFile{{"alpha/a.go", fileDiff("alpha/a.go", 5_000)}, {"big/huge.go", big}}
	ot := &overflowTransport{classifyAnswer: "SIZE"}
	gen, cfg := piecesGeneration(t, ot, files)
	cfg.MaxPromptBytes = config.DefaultMaxPromptBytes

	if _, err := gen.Generate(t.Context(), "v1.0.0", cfg); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if len(ot.single) != 1 {
		t.Fatalf("single-pass calls = %d, want one", len(ot.single))
	}
	failedSize := len(ot.single[0])
	ct := &chunkTransport{prompts: ot.chunks}
	pieces := piecesOf(t, ct, "big/huge.go")
	if len(pieces) < 2 {
		t.Fatalf("pieces of big/huge.go = %d, want the file sent as pieces", len(pieces))
	}
	header := pieceHeader("big/huge.go")
	for i, p := range pieces {
		if 2*len(p) > failedSize {
			t.Errorf("piece %d is %d bytes, over half the failed call's %d", i+1, len(p), failedSize)
		}
		if gotI, gotM, _ := pieceOf(p); gotI != i+1 || gotM != len(pieces) {
			t.Errorf("piece %d framing names piece %d of %d, want piece %d of %d", i+1, gotI, gotM, i+1, len(pieces))
		}
	}
	if got := header + strings.Join(pieceBodies(t, pieces, header), ""); got != big {
		t.Errorf("the pieces joined, header kept once, do not reproduce the file's diff byte for byte")
	}
}

func TestGenerator_SinglePassClassifiedOther_NoChunkCallAndTheCallsOwnFailure(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ot   *overflowTransport
	}{
		{"OTHER", &overflowTransport{classifyAnswer: "OTHER"}},
		{"a sentence naming SIZE", &overflowTransport{classifyAnswer: "This looks like a SIZE problem"}},
		{"both codes", &overflowTransport{classifyAnswer: "SIZE OTHER"}},
		{"an unknown word", &overflowTransport{classifyAnswer: "LIMIT"}},
		{"an empty answer", &overflowTransport{classifyAnswer: ` "". `}},
		{"the classify call fails", &overflowTransport{classifyAnswer: "SIZE", classifyErr: &ai.GenerationError{Stdout: "SIZE", ExitCode: 1}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := underCapScenario(t, threeTenKBFiles())

			_, err := s.generateOver(t, tc.ot)

			if len(tc.ot.chunks) != 0 || len(tc.ot.merges) != 0 {
				t.Errorf("chunk calls = %d, merge calls = %d, want none", len(tc.ot.chunks), len(tc.ot.merges))
			}
			if len(tc.ot.classify) != 1 {
				t.Errorf("classify calls = %d, want exactly one", len(tc.ot.classify))
			}
			if got, _ := notes.CauseText(err); got != "AI call failed" {
				t.Errorf("CauseText = %q, want %q", got, "AI call failed")
			}
			var genErr *ai.GenerationError
			if !errors.As(err, &genErr) || genErr.Output() != overflowOutput {
				t.Errorf("error = %v, want the single-pass call's own failure with its output verbatim", err)
			}
		})
	}
}
