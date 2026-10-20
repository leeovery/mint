package engine_test

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

const (
	tooLargePhrase   = "too large for the model, even in smaller parts"
	modelRejectsSize = "Prompt is too long"
)

// firstPartOverflowTransport overflows the model on every single-pass call and every
// chunk call framed as part 1, answering each classify call SIZE, so halving re-cuts
// the first part until it runs out. Every other chunk call gets notes.
type firstPartOverflowTransport struct {
	mu         sync.Mutex
	last       string
	overflowed int
}

func (ft *firstPartOverflowTransport) Generate(_ context.Context, prompt string) (string, error) {
	switch {
	case isClassifyPrompt(prompt):
		return "SIZE", nil
	case !strings.Contains(prompt, "diff --git "):
		return "merged notes", nil
	case strings.Contains(prompt, "What follows is part ") && !strings.Contains(prompt, "What follows is part 1 of "):
		return "- part notes", nil
	}
	ft.mu.Lock()
	defer ft.mu.Unlock()
	ft.last = prompt
	ft.overflowed++
	return "", &ai.GenerationError{Stdout: modelRejectsSize + "\n", ExitCode: 1}
}

var firstPartFraming = regexp.MustCompile(`What follows is part (\d+) of (\d+)`)

// halvingOutCase is a notes generation that halving cannot save.
type halvingOutCase struct {
	name string
	// fileBytes sizes each file of a two-file diff.
	fileBytes int
	// context, when set, is the release's context file.
	context string
	// overflows is how many calls overflow before the generation fails.
	overflows int
}

func halvingOutCases() []halvingOutCase {
	return []halvingOutCase{
		// The single-pass call, then its first part in each of three halved rounds.
		{name: "the first part halved three times", fileBytes: 100_000, overflows: 4},
		// The context fills more than half the single-pass call on its own.
		{name: "the first halving round has no room", fileBytes: 10_000, context: outsizedContext, overflows: 1},
	}
}

// wantPhrase is the ✗ line's phrase: the location of the last overflowed call when it
// was a chunk call, none when it was the single-pass call.
func (tc halvingOutCase) wantPhrase(t *testing.T, tr *firstPartOverflowTransport) string {
	t.Helper()
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tc.context != "" {
		return tooLargePhrase
	}
	m := firstPartFraming.FindStringSubmatch(tr.last)
	if m == nil {
		t.Fatalf("the last overflowed call is not a chunk call:\n%.300s", tr.last)
	}
	n, _ := strconv.Atoi(m[2])
	return fmt.Sprintf("%s (part 1 of %d)", tooLargePhrase, n)
}

func (tc halvingOutCase) seedRelease(t *testing.T, root string, f *runner.FakeRunner, onNotesFailure string) {
	t.Helper()
	config := "[release]\non_notes_failure = \"" + onNotesFailure + "\"\n"
	if tc.context != "" {
		writeFile(t, root, "context.md", tc.context)
		config += "context = \"context.md\"\n"
	}
	writeConfig(t, root, config)
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git",
		ScriptedOut(twoFileDiff(tc.fileBytes)),
		ScriptedOut("M\tapi/a.go\nM\tcore/b.go\n"),
		ScriptedOut("156\t0\tapi/a.go\n156\t0\tcore/b.go\n"),
	)
}

func (tc halvingOutCase) produce(ctx context.Context, t *testing.T, tr notes.Transport, res version.Resolution) (string, error) {
	t.Helper()
	dir := t.TempDir()
	cfg := freshCfg()
	if tc.context != "" {
		writeFile(t, dir, "context.md", tc.context)
		cfg.Release.Context = "context.md"
	}
	notesGit := seedFreshGit(twoFileDiff(tc.fileBytes), "M\tapi/a.go\nM\tcore/b.go\n", "156\t0\tapi/a.go\n156\t0\tcore/b.go\n")
	return engine.RegenerateFreshBody(ctx, notesGit, tr, dir, cfg, res, engine.NotesSinks{})
}

func TestRelease_HalvingRunsOut_Abort_StopsBeforeTheTagWithTheOutputVerbatim(t *testing.T) {
	t.Parallel()

	for _, tc := range halvingOutCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			tc.seedRelease(t, root, f, "abort")
			rec := &presentertest.RecordingPresenter{}
			tr := &firstPartOverflowTransport{}
			deps := newDeps(rec, f)
			deps.Transport = tr

			err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

			assertAbortNonZero(t, err)
			assertNoMutation(t, f)
			if tr.overflowed != tc.overflows {
				t.Errorf("overflowed calls = %d, want %d", tr.overflowed, tc.overflows)
			}
			sf := onlyStageFailureEvent(t, rec)
			if want := tc.wantPhrase(t, tr); sf.Name != "notes" || sf.Message != want {
				t.Errorf("StageFailure = %q %q, want notes %q", sf.Name, sf.Message, want)
			}
			if sf.Output != modelRejectsSize {
				t.Errorf("StageFailure.Output = %q, want the last overflowed call's output verbatim %q", sf.Output, modelRejectsSize)
			}
		})
	}
}

func TestRelease_HalvingRunsOut_Fallback_ShipsTheCommitSubjects(t *testing.T) {
	t.Parallel()

	const subjects = "subject one\nsubject two\n"
	for _, tc := range halvingOutCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			tc.seedRelease(t, root, f, "fallback")
			f.SeedSequence("git", ScriptedOut(subjects)) // log --format=%s
			seedRecordTagPush(f, root)
			f.Seed("gh", runner.Result{}, nil)
			rec := &presentertest.RecordingPresenter{}
			deps := newDeps(rec, f)
			deps.Transport = &firstPartOverflowTransport{}

			if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			if got := tagAnnotationBody(t, f, nextTag); got != subjects {
				t.Errorf("tag annotation body = %q, want the commit-subject fallback %q", got, subjects)
			}
		})
	}
}

func TestRegenerateRun_FreshHalvingRunsOut_StopsWithTheSameLineAndWritesNothing(t *testing.T) {
	t.Parallel()

	for _, tc := range halvingOutCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := runner.NewFakeRunner()
			f.Seed("git", runner.Result{}, nil)
			f.Seed("gh", runner.Result{}, nil)
			pub := newFakePublisher()
			rec := &presentertest.RecordingPresenter{}
			tr := &firstPartOverflowTransport{}
			req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease), false)
			req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, _ engine.NotesSinks) (string, error) {
				return tc.produce(ctx, t, tr, version.Resolution{Tag: regenRunTag, PreviousTag: "v1.3.0"})
			}

			if err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, t.TempDir(), req); err == nil {
				t.Fatal("RegenerateRun returned nil error, want it to stop")
			}

			sf := onlyStageFailureEvent(t, rec)
			if want := tc.wantPhrase(t, tr); sf.Message != want {
				t.Errorf("StageFailure.Message = %q, want %q", sf.Message, want)
			}
			if sf.Output != modelRejectsSize {
				t.Errorf("StageFailure.Output = %q, want the last overflowed call's output verbatim %q", sf.Output, modelRejectsSize)
			}
			if len(pub.dispatched) != 0 {
				t.Errorf("provider writes = %d, want none", len(pub.dispatched))
			}
		})
	}
}

func TestRegenerateAll_FreshHalvingRunsOut_SkipsThatVersionAndCarriesOn(t *testing.T) {
	t.Parallel()

	for _, tc := range halvingOutCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := runner.NewFakeRunner()
			f.Seed("git", runner.Result{}, nil)
			pub := newFakePublisher()
			pub.seedExists(batchV1Tag, true, nil)
			pub.seedExists(batchV2Tag, true, nil)
			pub.seedExists(batchV3Tag, true, nil)
			rec := &presentertest.RecordingPresenter{}
			tr := &firstPartOverflowTransport{}
			req := batchReq(engine.RegenerateSourceFresh, threeVersions(), true)
			req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, res version.Resolution, _ string, _ engine.NotesSinks) (string, error) {
				if res.Tag == batchV2Tag {
					return tc.produce(ctx, t, tr, res)
				}
				return "## fresh " + res.Tag + "\n", nil
			}

			if _, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub, t.TempDir(), req); err != nil {
				t.Fatalf("RegenerateAll returned unexpected error: %v", err)
			}

			if got := dispatchedTags(pub); len(got) != 2 || got[0] != batchV1Tag || got[1] != batchV3Tag {
				t.Errorf("provider writes = %v, want %s and %s with %s skipped", got, batchV1Tag, batchV3Tag, batchV2Tag)
			}
			want := "2 regenerated, 1 skipped: " + batchV2Tag + " (notes generation failed)"
			if got := finishEvent(t, rec).Summary; got != want {
				t.Errorf("RunFinished.Summary = %q, want %q", got, want)
			}
		})
	}
}
