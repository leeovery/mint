package notes_test

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"mint/internal/ai"
	"mint/internal/notes"
)

const savedNotesPath = "/cache/mint/chunk-notes/-Users-lee-Code-app/v1.4.0.md"

// noteSaver records every set of chunk notes a generation saves.
type noteSaver struct {
	mu    sync.Mutex
	saves []string
}

func (ns *noteSaver) save(notes string) (string, error) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.saves = append(ns.saves, notes)
	return savedNotesPath, nil
}

func (ns *noteSaver) saved() []string {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	return slices.Clone(ns.saves)
}

// assertSavedOnce fails unless the generation saved want once and its failure names
// the saved file.
func (ns *noteSaver) assertSavedOnce(t *testing.T, err error, want string) {
	t.Helper()
	if saves := ns.saved(); len(saves) != 1 || saves[0] != want {
		t.Errorf("saved chunk notes = %q, want once %q", saves, want)
	}
	if got := notes.SavedChunkNotesPath(err); got != savedNotesPath {
		t.Errorf("SavedChunkNotesPath = %q, want %q", got, savedNotesPath)
	}
}

func (ns *noteSaver) assertNothingSaved(t *testing.T, err error) {
	t.Helper()
	if saves := ns.saved(); len(saves) != 0 {
		t.Errorf("saved chunk notes = %q, want none", saves)
	}
	if got := notes.SavedChunkNotesPath(err); got != "" {
		t.Errorf("SavedChunkNotesPath = %q, want none named", got)
	}
}

// startSaving runs the scenario's generation on ctx in the background over transport,
// saving its chunk notes to the returned saver.
func (s chunkScenario) startSaving(t *testing.T, ctx context.Context, transport notes.Transport) (<-chan generation, *noteSaver) {
	t.Helper()
	ns := &noteSaver{}
	r := seedNormalPathGit(t, s.diff(), s.nameStatus(), s.numstat())
	gen := notes.NewGenerator(notes.NewAssembler(r, notes.ExcludeConfig{}), transport, s.root).WithChunkNotes(ns.save)
	done := make(chan generation, 1)
	go func() {
		body, err := gen.Generate(ctx, "v1.0.0", s.cfg)
		done <- generation{body: body, err: err}
	}()
	return done, ns
}

func (s chunkScenario) generateSaving(t *testing.T, transport notes.Transport) (*noteSaver, error) {
	t.Helper()
	done, ns := s.startSaving(t, t.Context(), transport)
	return ns, awaitFailure(t, done)
}

func joinedNotes(paths ...string) string {
	parts := make([]string, len(paths))
	for i, path := range paths {
		parts[i] = chunkNotesFor(path)
	}
	return strings.Join(parts, "\n\n")
}

// waitUntil polls until every event has been logged, then returns nil, or an error
// once waitLimit passes.
func (et *endingTransport) waitUntil(events ...string) error {
	deadline := time.Now().Add(waitLimit)
	for {
		logged := et.snapshot()
		if !slices.ContainsFunc(events, func(e string) bool { return !slices.Contains(logged, e) }) {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("events never logged")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestGenerator_WaveFailsForGood_SavesTheFinishedPartsNotesInChunkOrder(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(6))
	release := make(chan struct{})
	et := &endingTransport{classify: answerBy()}
	et.call = func(path string) error {
		switch path {
		case area(3):
			if err := et.waitUntil("start "+area(5), "start "+area(6)); err != nil {
				return err
			}
			return failedWith("area3: usage limit reached")
		case area(5):
			return heldUntil(release, nil)
		case area(6):
			return heldUntil(release, failedWith("area6: usage limit reached"))
		default:
			return nil
		}
	}

	done, ns := s.startSaving(t, t.Context(), et)
	et.waitFor(t, "classify "+area(3))
	time.Sleep(recordGrace)
	close(release)
	err := awaitFailure(t, done)

	if got := causeOf(err); got != "AI call failed (part 3 of 6)" {
		t.Errorf("CauseText = %q, want part 3's failure", got)
	}
	ns.assertSavedOnce(t, err, joinedNotes(area(1), area(2), area(4), area(5)))
}

func TestGenerator_SavedNotes_KeepEachPartsNotesByteForByteWithOneBlankLineBetween(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(3))
	partNotes := map[string]string{
		area(1): "### Added\n- area one   \n",
		area(2): "- area two, no trailing newline",
		area(3): "- area three\n\n",
	}
	transport := transportFunc(func(_ context.Context, prompt string) (string, error) {
		switch {
		case isClassifyPrompt(prompt):
			return "OTHER", nil
		case isChunkPrompt(prompt):
			return partNotes[firstPath(sliceOf(prompt))], nil
		default:
			return "", failedWith("Invalid API key · Please run /login")
		}
	})

	ns, err := s.generateSaving(t, transport)

	ns.assertSavedOnce(t, err, "### Added\n- area one   \n"+"\n"+"- area two, no trailing newline"+"\n\n"+"- area three\n\n")
}

func TestGenerator_MergeFails_SavesEveryPartsNotesAndNothingElse(t *testing.T) {
	t.Parallel()

	const mergeOutput = "Prompt is too long"
	cases := []struct {
		name      string
		classify  string
		notesSize int
		wantCause string
	}{
		{name: "the merge call fails", classify: "OTHER", wantCause: "AI call failed (merging)"},
		{name: "its classify answer is SIZE", classify: "SIZE", wantCause: notes.ErrMergeTooLarge.Error()},
		{name: "its prompt is over the cap", notesSize: 6_000, wantCause: notes.ErrMergeTooLarge.Error()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := capOneFilePerChunk(t, tenKBFiles(3))
			notesOf := func(path string) string {
				return chunkNotesFor(path) + strings.Repeat("n", tc.notesSize)
			}
			transport := transportFunc(func(_ context.Context, prompt string) (string, error) {
				switch {
				case isClassifyPrompt(prompt):
					return tc.classify, nil
				case isChunkPrompt(prompt):
					return notesOf(firstPath(sliceOf(prompt))), nil
				default:
					return "", failedWith(mergeOutput)
				}
			})

			ns, err := s.generateSaving(t, transport)

			if got := causeOf(err); got != tc.wantCause {
				t.Errorf("CauseText = %q, want %q", got, tc.wantCause)
			}
			ns.assertSavedOnce(t, err, notesOf(area(1))+"\n\n"+notesOf(area(2))+"\n\n"+notesOf(area(3)))
		})
	}
}

// succeededPaths records the first path of each chunk call that returned notes.
type succeededPaths struct {
	mu    sync.Mutex
	paths []string
}

func (sp *succeededPaths) add(path string) string {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.paths = append(sp.paths, path)
	return chunkNotesFor(path)
}

// inChunkOrder is the recorded paths in path order, which is chunk order.
func (sp *succeededPaths) inChunkOrder() []string {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sorted := slices.Clone(sp.paths)
	sort.Strings(sorted)
	return sorted
}

func TestGenerator_HalvedSliceThenMergeFails_SavesTheHalvedPartsNotesInTheSlicesPlace(t *testing.T) {
	t.Parallel()

	s := capOneAreaPerChunk(t, areaFiles(3))
	overflows := overflowsOnce(isSlice(areaDiff(2)))
	var mu sync.Mutex
	sp := &succeededPaths{}
	transport := transportFunc(func(_ context.Context, prompt string) (string, error) {
		switch {
		case isClassifyPrompt(prompt):
			return answerBy(overflowOutput)(prompt)
		case isChunkPrompt(prompt):
			mu.Lock()
			overflow := overflows(sliceOf(prompt))
			mu.Unlock()
			if overflow {
				return "", failedWith(overflowOutput)
			}
			return sp.add(firstPath(sliceOf(prompt))), nil
		default:
			return "", failedWith("Invalid API key · Please run /login")
		}
	})

	ns, err := s.generateSaving(t, transport)

	order := sp.inChunkOrder()
	if len(order) <= 3 {
		t.Fatalf("fixture: chunk calls that returned notes = %v, want area 2's slice halved into more than one", order)
	}
	if order[0] != "area1/a.go" || order[len(order)-1] != "area3/a.go" {
		t.Fatalf("fixture: chunk order %v, want area 2's halved parts between areas 1 and 3", order)
	}
	ns.assertSavedOnce(t, err, joinedNotes(order...))
}

func TestGenerator_HalvedSinglePassPartlyFinishes_SavesTheFinishedPartsNotes(t *testing.T) {
	t.Parallel()

	s := underCapScenario(t, threeTenKBFiles())
	sp := &succeededPaths{}
	transport := transportFunc(func(_ context.Context, prompt string) (string, error) {
		switch {
		case isClassifyPrompt(prompt):
			return answerBy(overflowOutput)(prompt)
		case isChunkPrompt(prompt):
			path := firstPath(sliceOf(prompt))
			if path == "gamma/c.go" {
				return "", failedWith("API Error: usage limit reached for this account")
			}
			return sp.add(path), nil
		case strings.Contains(prompt, "diff --git "):
			return "", failedWith(overflowOutput)
		default:
			return "merged", nil
		}
	})

	ns, err := s.generateSaving(t, transport)

	if got := sp.inChunkOrder(); !slices.Equal(got, []string{"alpha/a.go", "beta/b.go"}) {
		t.Fatalf("fixture: halved parts that returned notes = %v, want alpha's and beta's", got)
	}
	ns.assertSavedOnce(t, err, joinedNotes("alpha/a.go", "beta/b.go"))
}

func TestGenerator_NoPartFinished_SavesNothingAndNamesNoFile(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		scenario func(t *testing.T) chunkScenario
	}{
		{
			name:     "a wave in which no part returned notes",
			scenario: func(t *testing.T) chunkScenario { return capOneFilePerChunk(t, tenKBFiles(2)) },
		},
		{
			name:     "a single-pass call that fails without halving",
			scenario: func(t *testing.T) chunkScenario { return underCapScenario(t, threeTenKBFiles()) },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			transport := transportFunc(func(_ context.Context, prompt string) (string, error) {
				if isClassifyPrompt(prompt) {
					return "OTHER", nil
				}
				return "", failedWith("API Error: usage limit reached for this account")
			})

			ns, err := tc.scenario(t).generateSaving(t, transport)

			if !errors.Is(err, ai.ErrGenerationFailed) {
				t.Errorf("Generate error = %v, want the AI's failure", err)
			}
			ns.assertNothingSaved(t, err)
		})
	}
}

func TestGenerator_CtrlCWhileChunkCallsAreInFlight_SavesNothing(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(3))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	firstReturned := make(chan struct{})
	transport := transportFunc(func(ctx context.Context, prompt string) (string, error) {
		if firstPath(sliceOf(prompt)) == area(1) {
			defer close(firstReturned)
			return chunkNotesFor(area(1)), nil
		}
		<-ctx.Done()
		return "", ctx.Err()
	})

	done, ns := s.startSaving(t, ctx, transport)
	<-firstReturned
	time.Sleep(recordGrace)
	cancel()
	err := awaitFailure(t, done)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Generate error = %v, want context.Canceled", err)
	}
	ns.assertNothingSaved(t, err)
}

func TestGenerator_CtrlCAtTheMerge_SavesNothing(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		// merge answers the merge call, given the run's cancel.
		merge func(cancel context.CancelFunc) (string, error)
		// classify answers the merge failure's classify call, given the run's cancel.
		classify func(cancel context.CancelFunc) (string, error)
	}{
		{
			name: "during the merge call",
			merge: func(cancel context.CancelFunc) (string, error) {
				cancel()
				return "", context.Canceled
			},
		},
		{
			name: "during the merge's classify call",
			merge: func(context.CancelFunc) (string, error) {
				return "", failedWith("Invalid API key · Please run /login")
			},
			classify: func(cancel context.CancelFunc) (string, error) {
				cancel()
				return "", context.Canceled
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := capOneFilePerChunk(t, tenKBFiles(3))
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			transport := transportFunc(func(_ context.Context, prompt string) (string, error) {
				switch {
				case isClassifyPrompt(prompt):
					return tc.classify(cancel)
				case isChunkPrompt(prompt):
					return chunkNotesFor(firstPath(sliceOf(prompt))), nil
				default:
					return tc.merge(cancel)
				}
			})

			done, ns := s.startSaving(t, ctx, transport)
			err := awaitFailure(t, done)

			if !errors.Is(err, context.Canceled) {
				t.Errorf("Generate error = %v, want context.Canceled", err)
			}
			ns.assertNothingSaved(t, err)
		})
	}
}
