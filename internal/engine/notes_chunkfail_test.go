package engine_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

const (
	apiPartNotes  = "- api part notes"
	corePartNotes = "- core part notes"
)

// failingCallTransport answers a two-part chunked run, failing every notes call whose
// prompt fails reports true with failure, and answers a classify call OTHER. calls
// counts the notes calls.
type failingCallTransport struct {
	fails   func(prompt string) bool
	failure error

	mu    sync.Mutex
	calls int
}

func (ft *failingCallTransport) Generate(_ context.Context, prompt string) (string, error) {
	if isCheckPrompt(prompt) {
		return "none", nil
	}
	if isClassifyPrompt(prompt) {
		return "OTHER", nil
	}
	ft.mu.Lock()
	ft.calls++
	ft.mu.Unlock()
	if ft.fails(prompt) {
		return "", ft.failure
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
	name    string
	fails   func(prompt string) bool
	failure error
	// wantMerge reports whether the run makes a merge call.
	wantMerge   bool
	wantMessage string
	wantOutput  string
}

func isSecondPartPrompt(prompt string) bool {
	return strings.Contains(prompt, "What follows is part 2 of")
}

func chunkFailureCases() []chunkFailureCase {
	return []chunkFailureCase{
		{
			name:        "a chunk call fails",
			fails:       isSecondPartPrompt,
			failure:     &ai.GenerationError{Stdout: "API Error: usage limit reached for this account", ExitCode: 1},
			wantMessage: "AI call failed (part 2 of 2)",
			wantOutput:  "API Error: usage limit reached for this account",
		},
		{
			name:        "a chunk call times out",
			fails:       isSecondPartPrompt,
			failure:     fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout),
			wantMessage: "AI timed out (part 2 of 2)",
		},
		{
			name:        "the merge call fails",
			fails:       isMergePrompt,
			failure:     &ai.GenerationError{Stdout: "Invalid API key · Please run /login", ExitCode: 1},
			wantMerge:   true,
			wantMessage: "AI call failed (merging)",
			wantOutput:  "Invalid API key · Please run /login",
		},
		{
			name:        "the merge call returns empty notes",
			fails:       isMergePrompt,
			failure:     &ai.GenerationError{Stderr: " \n"},
			wantMerge:   true,
			wantMessage: "AI returned empty notes (merging)",
		},
	}
}

func (tc chunkFailureCase) transport() *failingCallTransport {
	return &failingCallTransport{fails: tc.fails, failure: tc.failure}
}

// seedOverCapRelease scripts a release over a two-file diff of halvableFileBytes a
// file, which chunks into two parts.
func seedOverCapRelease(t *testing.T, root string, f *runner.FakeRunner, onNotesFailure string) {
	t.Helper()
	writeConfig(t, root, capToml(partCap(halvableFileBytes))+"\n[release]\non_notes_failure = \""+onNotesFailure+"\"\n")
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git",
		ScriptedOut(twoFileDiff(halvableFileBytes)),
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
			if sf.Name != "notes" || sf.Message != tc.wantMessage {
				t.Errorf("StageFailure = %q %q, want notes %q", sf.Name, sf.Message, tc.wantMessage)
			}
			if sf.Output != tc.wantOutput {
				t.Errorf("StageFailure.Output = %q, want the failed call's output %q", sf.Output, tc.wantOutput)
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
	cfg.MaxPromptBytes = partCapBytes
	notesGit := seedFreshGit(twoFileDiff(10_000), "M\tapi/a.go\nM\tcore/b.go\n", "156\t0\tapi/a.go\n156\t0\tcore/b.go\n")
	return engine.RegenerateFreshBody(ctx, notesGit, tc.transport(), t.TempDir(), cfg, res, engine.NotesSinks{})
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
			req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease))
			req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, _ engine.NotesSinks) (string, error) {
				return tc.produceFresh(t, ctx, version.Resolution{Tag: regenRunTag, PreviousTag: "v1.3.0"})
			}

			if err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, t.TempDir(), req); err == nil {
				t.Fatal("RegenerateRun returned nil error, want an abort")
			}

			sf := onlyStageFailureEvent(t, rec)
			if sf.Message != tc.wantMessage || sf.Output != tc.wantOutput {
				t.Errorf("StageFailure = %q with output %q, want %q with %q", sf.Message, sf.Output, tc.wantMessage, tc.wantOutput)
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
			req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, res version.Resolution, _ string, _ engine.NotesSinks) (string, error) {
				if res.Tag == batchV2Tag {
					return tc.produceFresh(t, ctx, res)
				}
				return "## fresh " + res.Tag + "\n", nil
			}

			if _, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub, t.TempDir(), req); err != nil {
				t.Fatalf("RegenerateAll returned unexpected error: %v", err)
			}

			if got := dispatchedTags(pub); len(got) != 2 || got[0] != batchV1Tag || got[1] != batchV3Tag {
				t.Errorf("provider writes = %v, want %s and %s with %s skipped", got, batchV1Tag, batchV3Tag, batchV2Tag)
			}
			assertSkippedV2(t, rec, pub, tc.wantMessage, tc.wantOutput)
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

func (it *interruptingTransport) Generate(ctx context.Context, prompt string) (string, error) {
	if isCheckPrompt(prompt) {
		return "none", nil
	}
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
