package engine_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// failedCallPhrase is the ✗ line's cause phrase for an AI call that failed after the
// transport's retry.
const failedCallPhrase = "AI returned empty/invalid notes after retry"

const (
	apiPartNotes  = "- api part notes"
	corePartNotes = "- core part notes"
)

// failingCallTransport answers a two-part chunked run, failing every call whose prompt
// fails reports true with output as the AI's captured stdout.
type failingCallTransport struct {
	fails  func(prompt string) bool
	output string

	mu    sync.Mutex
	calls int
}

func (ft *failingCallTransport) Generate(_ context.Context, prompt string) (string, error) {
	ft.mu.Lock()
	ft.calls++
	ft.mu.Unlock()
	if ft.fails(prompt) {
		return "", &ai.GenerationError{Stdout: ft.output, ExitCode: 1}
	}
	switch {
	case strings.Contains(prompt, "What follows is part 1 of"):
		return apiPartNotes, nil
	case strings.Contains(prompt, "What follows is part 2 of"):
		return corePartNotes, nil
	default:
		return "merged notes", nil
	}
}

func isMergePrompt(prompt string) bool {
	return !strings.Contains(prompt, "What follows is part ")
}

type chunkFailureCase struct {
	name   string
	fails  func(prompt string) bool
	output string
	// wantMerge reports whether the run makes a merge call.
	wantMerge bool
}

func chunkFailureCases() []chunkFailureCase {
	return []chunkFailureCase{
		{
			name:   "a chunk call fails",
			fails:  func(prompt string) bool { return strings.Contains(prompt, "What follows is part 2 of") },
			output: "API Error: usage limit reached for this account",
		},
		{
			name:      "the merge call fails",
			fails:     isMergePrompt,
			output:    "Invalid API key · Please run /login",
			wantMerge: true,
		},
	}
}

func (tc chunkFailureCase) transport() *failingCallTransport {
	return &failingCallTransport{fails: tc.fails, output: tc.output}
}

// seedOverCapRelease scripts a release over a two-file diff of about 10 KB a file
// under a 20,000-byte cap, which chunks into two parts.
func seedOverCapRelease(t *testing.T, root string, f *runner.FakeRunner, onNotesFailure string) {
	t.Helper()
	writeConfig(t, root, "max_prompt_bytes = 20000\n\n[release]\non_notes_failure = \""+onNotesFailure+"\"\n")
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git",
		ScriptedOut(twoFileDiff(10_000)),
		ScriptedOut("M\tapi/a.go\nM\tcore/b.go\n"),
		ScriptedOut("156\t0\tapi/a.go\n156\t0\tcore/b.go\n"),
	)
}

func TestRelease_ChunkedRunFails_Abort_StopsBeforeTheTagWithTheOutputVerbatim(t *testing.T) {
	t.Parallel()

	for _, tc := range chunkFailureCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			seedOverCapRelease(t, root, f, "abort")
			rec := &presentertest.RecordingPresenter{}
			tr := tc.transport()
			deps := newDeps(rec, f)
			deps.Transport = tr

			err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

			assertAbortNonZero(t, err)
			assertNoMutation(t, f)
			wantCalls := 2
			if tc.wantMerge {
				wantCalls = 3
			}
			if tr.calls != wantCalls {
				t.Errorf("AI calls = %d, want %d", tr.calls, wantCalls)
			}
			sf := onlyStageFailureEvent(t, rec)
			if sf.Name != "notes" || sf.Message != failedCallPhrase {
				t.Errorf("StageFailure = %q %q, want notes %q", sf.Name, sf.Message, failedCallPhrase)
			}
			if sf.Output != tc.output {
				t.Errorf("StageFailure.Output = %q, want the failed call's output %q", sf.Output, tc.output)
			}
		})
	}
}

func TestRelease_ChunkedRunFails_Fallback_ShipsTheFallbackBodyNeverThePartNotes(t *testing.T) {
	t.Parallel()

	const subjects = "subject one\nsubject two\n"
	for _, tc := range chunkFailureCases() {
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
			deps.Transport = tc.transport()

			if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			body := tagAnnotationBody(t, f, nextTag)
			if body != subjects {
				t.Errorf("tag annotation body = %q, want the commit-subject fallback %q", body, subjects)
			}
			if got := changelogSectionBody(t, root, "1.2.4"); strings.Contains(got, apiPartNotes) || strings.Contains(got, corePartNotes) {
				t.Errorf("CHANGELOG body = %q carries part notes", got)
			}
		})
	}
}

func (tc chunkFailureCase) produceFresh(t *testing.T, ctx context.Context, res version.Resolution) (string, error) {
	t.Helper()
	cfg := freshCfg()
	cfg.MaxPromptBytes = 20_000
	notesGit := seedFreshGit(twoFileDiff(10_000), "M\tapi/a.go\nM\tcore/b.go\n", "156\t0\tapi/a.go\n156\t0\tcore/b.go\n")
	return engine.RegenerateFreshBody(ctx, notesGit, tc.transport(), t.TempDir(), cfg, res, nil)
}

func TestRegenerateRun_FreshChunkedRunFails_AbortsWithTheOutputVerbatim(t *testing.T) {
	t.Parallel()

	for _, tc := range chunkFailureCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := runner.NewFakeRunner()
			f.Seed("git", runner.Result{}, nil)
			f.Seed("gh", runner.Result{}, nil)
			pub := newFakePublisher()
			rec := &presentertest.RecordingPresenter{}
			req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease), false)
			req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, _ notes.Progress) (string, error) {
				return tc.produceFresh(t, ctx, version.Resolution{Tag: regenRunTag, PreviousTag: "v1.3.0"})
			}

			if err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, t.TempDir(), req); err == nil {
				t.Fatal("RegenerateRun returned nil error, want an abort")
			}

			sf := onlyStageFailureEvent(t, rec)
			if sf.Message != failedCallPhrase || sf.Output != tc.output {
				t.Errorf("StageFailure = %q with output %q, want %q with %q", sf.Message, sf.Output, failedCallPhrase, tc.output)
			}
			if len(pub.dispatched) != 0 {
				t.Errorf("provider writes = %d, want none for an aborted regenerate", len(pub.dispatched))
			}
		})
	}
}

func TestRegenerateAll_FreshChunkedRunFails_SkipsThatVersionAndCarriesOn(t *testing.T) {
	t.Parallel()

	for _, tc := range chunkFailureCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := runner.NewFakeRunner()
			f.Seed("git", runner.Result{}, nil)
			pub := newFakePublisher()
			pub.seedExists(batchV1Tag, true, nil)
			pub.seedExists(batchV2Tag, true, nil)
			pub.seedExists(batchV3Tag, true, nil)
			rec := &presentertest.RecordingPresenter{}
			req := batchReq(engine.RegenerateSourceFresh, threeVersions(), true)
			req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, res version.Resolution, _ string, _ notes.Progress) (string, error) {
				if res.Tag == batchV2Tag {
					return tc.produceFresh(t, ctx, res)
				}
				return "## fresh " + res.Tag + "\n", nil
			}

			if _, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub, req); err != nil {
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

// interruptingTransport stands in for a Ctrl-C landing mid-wave: once both chunk
// calls are in flight it cancels the run, and every call returns its context's error.
type interruptingTransport struct {
	interrupt context.CancelFunc

	mu       sync.Mutex
	inFlight int
	calls    int
}

func (it *interruptingTransport) Generate(ctx context.Context, _ string) (string, error) {
	it.mu.Lock()
	it.calls++
	it.inFlight++
	if it.inFlight == 2 {
		it.interrupt()
	}
	it.mu.Unlock()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(5 * time.Second):
		return "", errors.New("the call was never cancelled")
	}
}

func TestRelease_CtrlCMidWave_Fallback_AbortsCleanlyWithNoFallbackBody(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedOverCapRelease(t, root, f, "fallback")
	f.SeedSequence("git", ScriptedOut("subject one\n")) // log --format=%s, if wrongly reached
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	tr := &interruptingTransport{interrupt: cancel}
	deps := newDeps(rec, f)
	deps.Transport = tr

	err := engine.Release(ctx, deps, priorTagNormalAIOptions())

	assertAbortNonZero(t, err)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want it to carry context.Canceled", err)
	}
	assertNoMutation(t, f)
	if invokedWith(f, "git", "log", "--format=%s", priorTag+"..HEAD") {
		t.Errorf("an interrupted run built the fallback body; on_notes_failure must not apply")
	}
	if tr.calls != 2 {
		t.Errorf("AI calls = %d, want only the 2 cancelled in flight", tr.calls)
	}
	if sf := onlyStageFailureEvent(t, rec); sf.Name != "notes" {
		t.Errorf("StageFailure.Name = %q, want the notes stage", sf.Name)
	}
}
