package engine_test

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

const lowerCapLabel = "max_prompt_bytes"

func lowerCapWarning(bytes int) presenter.Warning {
	return presenter.Warning{
		Label:   lowerCapLabel,
		Message: "set higher than this model accepts; set max_prompt_bytes = " + strconv.Itoa(bytes) + ", the largest call that succeeded",
	}
}

func lowerCapHints(rec *presentertest.RecordingPresenter) []int {
	var at []int
	for i, ev := range rec.Events {
		if ev.Kind == presentertest.KindWarn && ev.Warn.Label == lowerCapLabel {
			at = append(at, i)
		}
	}
	return at
}

// assertLowerCapHint fails unless rec shows one lower-the-cap hint naming bytes,
// straight after the notes and straight before the review gate.
func assertLowerCapHint(t *testing.T, rec *presentertest.RecordingPresenter, bytes int, gate presenter.Gate) {
	t.Helper()
	at := lowerCapHints(rec)
	if len(at) != 1 {
		t.Fatalf("lower-the-cap hints = %d, want one", len(at))
	}
	i := at[0]
	if got, want := rec.Events[i].Warn, lowerCapWarning(bytes); got != want {
		t.Errorf("hint = %+v, want %+v", got, want)
	}
	if i == 0 || rec.Events[i-1].Kind != presentertest.KindShowNotes {
		t.Errorf("the event before the hint is not the notes")
	}
	if i+1 == len(rec.Events) || rec.Events[i+1].Kind != presentertest.KindPrompt || rec.Events[i+1].Prompt.Question != gate.Question {
		t.Errorf("the event after the hint is not the review gate")
	}
}

func assertNoLowerCapHint(t *testing.T, rec *presentertest.RecordingPresenter) {
	t.Helper()
	if at := lowerCapHints(rec); len(at) != 0 {
		t.Errorf("lower-the-cap hints = %d, want none: %+v", len(at), rec.Events[at[0]].Warn)
	}
}

func largest(prompts []string) int {
	most := 0
	for _, p := range prompts {
		most = max(most, len(p))
	}
	return most
}

func TestRelease_SinglePassClassifiedSize_HintsTheLargestHalvedCallBetweenTheNotesAndTheReviewGate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHalvedRelease(root, f)
	rec := &presentertest.RecordingPresenter{}
	tr := &sizeOverflowTransport{}
	deps := newDeps(rec, f)
	deps.Transport = tr

	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	tr.assertHalved(t)
	assertLowerCapHint(t, rec, largest(tr.chunks), presenter.ReleaseReviewGate())
}

func TestRelease_ChunkClassifiedSize_HintsTheLargestSucceededCallHalvedOrNot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedOverCapRelease(t, root, f, "abort")
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}
	tr := &partOverflowTransport{overflowCore: true}
	deps := newDeps(rec, f)
	deps.Transport = tr

	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	tr.assertCorePartHalved(t)
	var unhalved []string
	for _, p := range tr.chunks {
		if !strings.Contains(p, "diff --git a/core/b.go") {
			unhalved = append(unhalved, p)
		}
	}
	if len(unhalved) != 1 || len(unhalved[0]) != largest(tr.chunks) {
		t.Fatalf("fixture: the unhalved api/a.go part is not the largest succeeded call")
	}
	assertLowerCapHint(t, rec, largest(tr.chunks), presenter.ReleaseReviewGate())
}

// wordyPartsTransport halves a single-pass call whose classify call reads SIZE, its
// chunk calls each answering notes long enough that the merge call is bigger than
// any of them.
type wordyPartsTransport struct {
	mu       sync.Mutex
	chunks   []string
	classify []string
	merges   []string
}

func (wt *wordyPartsTransport) Generate(_ context.Context, prompt string) (string, error) {
	if isCheckPrompt(prompt) {
		return "none", nil
	}
	wt.mu.Lock()
	defer wt.mu.Unlock()
	switch {
	case isClassifyPrompt(prompt):
		wt.classify = append(wt.classify, prompt)
		return "SIZE", nil
	case strings.Contains(prompt, "What follows is part "):
		wt.chunks = append(wt.chunks, prompt)
		return strings.Repeat("- a long note on this part\n", 1_000), nil
	case strings.Contains(prompt, "diff --git "):
		return "", &ai.GenerationError{Stdout: "Prompt is too long\n", ExitCode: 1}
	default:
		wt.merges = append(wt.merges, prompt)
		return halvedNotesBody, nil
	}
}

func TestRelease_MergeCallBiggerThanEveryPart_HintsTheLargestPartNeverTheMergeOrClassify(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHalvedRelease(root, f)
	rec := &presentertest.RecordingPresenter{}
	tr := &wordyPartsTransport{}
	deps := newDeps(rec, f)
	deps.Transport = tr

	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if len(tr.merges) != 1 || len(tr.merges[0]) <= largest(tr.chunks) {
		t.Fatalf("fixture: the merge call is not bigger than every part")
	}
	if len(tr.classify) != 1 {
		t.Fatalf("classify calls = %d, want one", len(tr.classify))
	}
	assertLowerCapHint(t, rec, largest(tr.chunks), presenter.ReleaseReviewGate())
}

func TestRelease_NotesWrittenWithoutHalving_NoHint(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		seed func(t *testing.T, root string, f *runner.FakeRunner)
	}{
		{"single pass", func(_ *testing.T, root string, f *runner.FakeRunner) { seedHalvedRelease(root, f) }},
		{"chunked", func(t *testing.T, root string, f *runner.FakeRunner) {
			seedOverCapRelease(t, root, f, "abort")
			seedRecordTagPush(f, root)
			f.Seed("gh", runner.Result{}, nil)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			tc.seed(t, root, f)
			rec := &presentertest.RecordingPresenter{}
			tr := &partOverflowTransport{}
			deps := newDeps(rec, f)
			deps.Transport = tr

			if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			if tr.classify != 0 {
				t.Fatalf("fixture: classify calls = %d, want a run with no halving", tr.classify)
			}
			assertNoLowerCapHint(t, rec)
		})
	}
}

// coreOverflowingTransport writes notes for every call carrying api/a.go alone and
// fails every call carrying core/b.go with an overflow's output, answering every
// classify call SIZE, so core/b.go's halving runs out.
type coreOverflowingTransport struct {
	mu        sync.Mutex
	succeeded int
}

func (ct *coreOverflowingTransport) Generate(_ context.Context, prompt string) (string, error) {
	if isCheckPrompt(prompt) {
		return "none", nil
	}
	ct.mu.Lock()
	defer ct.mu.Unlock()
	switch {
	case isClassifyPrompt(prompt):
		return "SIZE", nil
	case strings.Contains(prompt, "diff --git a/core/b.go"):
		return "", &ai.GenerationError{Stdout: "Prompt is too long\n", ExitCode: 1}
	default:
		ct.succeeded++
		return "- part notes", nil
	}
}

func TestRelease_HalvedThenNotesFailed_Fallback_ShipsTheSubjectsWithNoHint(t *testing.T) {
	t.Parallel()

	const subjects = "subject one\nsubject two\n"
	for _, tc := range []struct {
		name      string
		transport func() notes.Transport
	}{
		{"halving runs out", func() notes.Transport { return &coreOverflowingTransport{} }},
		{"the merge fails", func() notes.Transport {
			return &partOverflowTransport{overflowCore: true, mergeErr: &ai.GenerationError{Stdout: "boom\n", ExitCode: 1}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			seedOverCapRelease(t, root, f, "fallback")
			f.SeedSequence("git", ScriptedOut(subjects)) // log --format=%s
			seedRecordTagPush(f, root)
			f.Seed("gh", runner.Result{}, nil)
			rec := &presentertest.RecordingPresenter{}
			deps := newDeps(rec, f)
			tr := tc.transport()
			deps.Transport = tr

			if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			if ct, ok := tr.(*coreOverflowingTransport); ok && ct.succeeded == 0 {
				t.Fatal("fixture: no call carrying diff succeeded")
			}
			if body := tagAnnotationBody(t, f, nextTag); body != subjects {
				t.Errorf("tag annotation body = %q, want the commit-subject fallback %q", body, subjects)
			}
			assertNoLowerCapHint(t, rec)
		})
	}
}
func TestRegenerateRun_FreshSinglePassClassifiedSize_HintsBetweenTheNotesAndTheReviewGate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, true, nil)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	tr := &sizeOverflowTransport{}

	req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease), false)
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, sinks engine.NotesSinks) (string, error) {
		return halvedFreshBody(ctx, tr, version.Resolution{Tag: regenRunTag, PreviousTag: "v1.3.0"}, sinks)
	}

	if err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, dir, req); err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	tr.assertHalved(t)
	assertLowerCapHint(t, rec, largest(tr.chunks), presenter.NotesReviewGate())
}
