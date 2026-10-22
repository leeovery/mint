package engine_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// skipWarns returns every `skipped <tag>` warning the recorder captured, in order.
func skipWarns(rec *presentertest.RecordingPresenter) []presenter.Warning {
	var warns []presenter.Warning
	for _, w := range warnings(rec) {
		if strings.HasPrefix(w.Label, "skipped ") {
			warns = append(warns, w)
		}
	}
	return warns
}

// failingV2Batch is a fresh `--all` over threeVersions whose v1.1.0 notes come from
// produce and whose other versions' notes come back as they are.
func failingV2Batch(produce freshProducer) engine.BatchRegenerateRequest {
	req := batchReq(engine.RegenerateSourceFresh, threeVersions(), true)
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, res version.Resolution, _ string, sinks engine.NotesSinks) (string, error) {
		if res.Tag == batchV2Tag {
			return produce(ctx, res, sinks)
		}
		return "## fresh " + res.Tag + "\n", nil
	}
	return req
}

func newBatchRun(t *testing.T) (*presentertest.RecordingPresenter, *fakePublisher, engine.ReleaseDeps) {
	t.Helper()
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	rec := &presentertest.RecordingPresenter{}
	return rec, pub, batchDeps(rec, f)
}

// singlePassNotLoggedInUnder fails the one notes call of a fresh run whose
// on_notes_failure is onFailure with claude's expired login on stdout, classified OTHER.
func singlePassNotLoggedInUnder(onFailure string) freshProducer {
	return func(ctx context.Context, res version.Resolution, sinks engine.NotesSinks) (string, error) {
		cfg := freshCfg()
		cfg.Release.OnNotesFailure = onFailure
		notesGit := seedFreshGit(twoFileDiff(1_000), "M\tapi/a.go\nM\tcore/b.go\n", "16\t0\tapi/a.go\n16\t0\tcore/b.go\n")
		notLoggedInCall := func(_ context.Context, prompt string) (string, error) {
			if isClassifyPrompt(prompt) {
				return "OTHER", nil
			}
			return "", &ai.GenerationError{Stdout: notLoggedIn, ExitCode: 1}
		}
		return engine.RegenerateFreshBody(ctx, notesGit, transportFunc(notLoggedInCall), "", cfg, res, sinks)
	}
}

// singlePassFailsWith fails the one notes call of a fresh run with err.
func singlePassFailsWith(err error) freshProducer {
	return func(ctx context.Context, res version.Resolution, sinks engine.NotesSinks) (string, error) {
		return underCapFresh(ctx, func(context.Context, string) (string, error) { return "", err }, res, sinks)
	}
}

// assertSkippedV2 fails the test unless v1.1.0 alone was skipped with a warning reading
// phrase with output beneath it and a summary entry naming phrase, while the other
// versions were regenerated.
func assertSkippedV2(t *testing.T, rec *presentertest.RecordingPresenter, pub *fakePublisher, phrase, output string) {
	t.Helper()
	want := []presenter.Warning{{Label: "skipped " + batchV2Tag, Message: phrase, Output: output}}
	if got := skipWarns(rec); !slices.Equal(got, want) {
		t.Errorf("skip warnings = %+v, want %+v", got, want)
	}
	if got, want := finishEvent(t, rec).Summary, "2 regenerated, 1 skipped: "+batchV2Tag+" ("+phrase+")"; got != want {
		t.Errorf("RunFinished.Summary = %q, want %q", got, want)
	}
	if got, want := dispatchedTags(pub), []string{batchV1Tag, batchV3Tag}; !slices.Equal(got, want) {
		t.Errorf("provider writes = %v, want %v", got, want)
	}
}

func TestRegenerateAll_SinglePassCallFails_SkipNamesTheCauseWithTheAIsOutputBeneath(t *testing.T) {
	t.Parallel()

	rec, pub, deps := newBatchRun(t)

	if _, err := engine.RegenerateAll(t.Context(), deps, pub, t.TempDir(), failingV2Batch(singlePassNotLoggedInUnder(""))); err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}

	assertSkippedV2(t, rec, pub, "AI call failed", notLoggedIn)
}

func TestRegenerateAll_NotesFailure_SkipsWhateverTheModeOrOnNotesFailure(t *testing.T) {
	t.Parallel()

	modes := []struct {
		name             string
		yes              bool
		stdinInteractive bool
	}{
		{name: "at a terminal without -y", stdinInteractive: true},
		{name: "under -y", yes: true, stdinInteractive: true},
	}
	for _, mode := range modes {
		for _, onFailure := range []string{"", "abort", "fallback"} {
			t.Run(fmt.Sprintf("%s, on_notes_failure %q", mode.name, onFailure), func(t *testing.T) {
				t.Parallel()

				rec, pub, deps := newBatchRun(t)
				rec.PromptResult = func(presenter.Gate) (presenter.Choice, error) { return presenter.ChoiceYes, nil }
				deps.Yes = mode.yes
				deps.StdinInteractive = mode.stdinInteractive
				req := failingV2Batch(singlePassNotLoggedInUnder(onFailure))
				req.Yes = mode.yes

				if _, err := engine.RegenerateAll(t.Context(), deps, pub, t.TempDir(), req); err != nil {
					t.Fatalf("RegenerateAll returned unexpected error: %v", err)
				}

				assertSkippedV2(t, rec, pub, "AI call failed", notLoggedIn)
				for _, g := range promptGates(rec) {
					if g.Has(presenter.ChoiceCommitSubjects) {
						t.Errorf("the batch showed the floor's choice: %+v", g)
					}
				}
				if moves := nextMovesEvents(rec); len(moves) != 0 {
					t.Errorf("the batch printed next moves %+v, want the skip alone", moves)
				}
			})
		}
	}
}

func TestRegenerateAll_NotesFailure_EachCauseNamesItsPhrase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		err    error
		phrase string
	}{
		{name: "timeout", err: fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout), phrase: "AI timed out"},
		{name: "missing binary", err: fmt.Errorf("%w: claude not found", ai.ErrCommandMissing), phrase: "AI tool not installed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec, pub, deps := newBatchRun(t)

			if _, err := engine.RegenerateAll(t.Context(), deps, pub, t.TempDir(), failingV2Batch(singlePassFailsWith(tt.err))); err != nil {
				t.Fatalf("RegenerateAll returned unexpected error: %v", err)
			}

			assertSkippedV2(t, rec, pub, tt.phrase, "")
		})
	}
}

// partFiveOutput is what part 5 of a six-part wave prints when it fails.
const partFiveOutput = "API Error: 529 overloaded"

// sixPartGit is the diff, name-status and numstat of six files of about 10 KB, which
// chunk into six parts, one a file, under partCapBytes.
func sixPartGit() (diff, nameStatus, numstat string) {
	var ns, nu strings.Builder
	for _, path := range sixPartPaths {
		ns.WriteString("M\t" + path + "\n")
		nu.WriteString("156\t0\t" + path + "\n")
	}
	return sixFileDiff(10_000), ns.String(), nu.String()
}

// partThreeEndsTheWave writes a six-part chunked run whose part 3 fails for good,
// classified OTHER, once part 5 is in flight; part 5 then fails too, after part 3's
// failure has been classified. Parts 2 and 4 are held until part 5 has failed, so no
// part 6 is sent.
type partThreeEndsTheWave struct {
	fiveStarted, threeClassified, fiveEnded chan struct{}
	once                                    sync.Once
}

func newPartThreeEndsTheWave() *partThreeEndsTheWave {
	return &partThreeEndsTheWave{
		fiveStarted:     make(chan struct{}),
		threeClassified: make(chan struct{}),
		fiveEnded:       make(chan struct{}),
	}
}

func (wt *partThreeEndsTheWave) Generate(_ context.Context, prompt string) (string, error) {
	if isClassifyPrompt(prompt) {
		if strings.Contains(prompt, partThreeOutput) {
			wt.once.Do(func() { close(wt.threeClassified) })
		}
		return "OTHER", nil
	}
	if !isChunkCallPrompt(prompt) {
		return "merged notes", nil
	}
	switch firstPartFraming.FindStringSubmatch(prompt)[1] {
	case "3":
		<-wt.fiveStarted
		return "", &ai.GenerationError{Stdout: partThreeOutput, ExitCode: 1}
	case "5":
		close(wt.fiveStarted)
		defer close(wt.fiveEnded)
		<-wt.threeClassified
		// Lets the wave record part 3's classified failure before part 5's lands.
		time.Sleep(50 * time.Millisecond)
		return "", &ai.GenerationError{Stdout: partFiveOutput, ExitCode: 1}
	case "2", "4":
		<-wt.fiveEnded
	}
	return "- part notes", nil
}

func TestRegenerateAll_WaveFailsForGoodAtPartThree_SkipNamesThePartWithItsOutputBeneath(t *testing.T) {
	t.Parallel()

	rec, pub, deps := newBatchRun(t)
	tr := newPartThreeEndsTheWave()
	produce := func(ctx context.Context, res version.Resolution, _ engine.NotesSinks) (string, error) {
		cfg := freshCfg()
		cfg.MaxPromptBytes = partCapBytes
		return engine.RegenerateFreshBody(ctx, seedFreshGit(sixPartGit()), tr, t.TempDir(), cfg, res, engine.NotesSinks{})
	}

	if _, err := engine.RegenerateAll(t.Context(), deps, pub, t.TempDir(), failingV2Batch(produce)); err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}

	assertSkippedV2(t, rec, pub, "AI call failed (part 3 of 6)", partThreeOutput)
}
