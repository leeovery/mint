package engine_test

import (
	"context"
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

// mergeOverflowOutput is what a merge call that overflowed the model printed.
const mergeOverflowOutput = "Prompt is too long · the request is ~1300000 tokens (limit 1000000)"

// partOverflowTransport writes a two-part chunked run over api/a.go and core/b.go.
// With overflowCore set, the first chunk call carrying core/b.go fails with an
// overflow's output. Every classify call is answered SIZE, every other chunk call gets
// notes, and the merge halvedNotesBody, or fails with mergeErr when set.
type partOverflowTransport struct {
	overflowCore bool
	mergeErr     error

	mu         sync.Mutex
	calls      int
	overflowed []string
	chunks     []string
	classify   int
	merges     int
}

func (pt *partOverflowTransport) Generate(_ context.Context, prompt string) (string, error) {
	if isCheckPrompt(prompt) {
		return "none", nil
	}
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.calls++
	switch {
	case isClassifyPrompt(prompt):
		pt.classify++
		return "SIZE", nil
	case isMergePrompt(prompt):
		pt.merges++
		if pt.mergeErr != nil {
			return "", pt.mergeErr
		}
		return halvedNotesBody, nil
	case pt.overflowCore && len(pt.overflowed) == 0 && strings.Contains(prompt, "diff --git a/core/b.go"):
		pt.overflowed = append(pt.overflowed, prompt)
		return "", &ai.GenerationError{Stdout: "Prompt is too long\n", ExitCode: 1}
	default:
		pt.chunks = append(pt.chunks, prompt)
		return "- part notes", nil
	}
}

// assertCorePartHalved fails unless the core/b.go part overflowed once, its classify
// call read SIZE, its slice was re-sent in calls each at most half its size, and one
// merge call was made.
func (pt *partOverflowTransport) assertCorePartHalved(t *testing.T) {
	t.Helper()
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if len(pt.overflowed) != 1 || pt.classify != 1 || pt.merges != 1 {
		t.Fatalf("overflowed calls = %d, classify calls = %d, merge calls = %d, want one of each", len(pt.overflowed), pt.classify, pt.merges)
	}
	var halved []string
	for _, p := range pt.chunks {
		if strings.Contains(p, "diff --git a/core/b.go") {
			halved = append(halved, p)
		}
	}
	if len(halved) < 2 {
		t.Fatalf("calls carrying core/b.go after its overflow = %d, want its slice re-sent in smaller calls", len(halved))
	}
	for i, p := range halved {
		if 2*len(p) > len(pt.overflowed[0]) {
			t.Errorf("halved call %d is %d bytes, over half the failed call's %d", i+1, len(p), len(pt.overflowed[0]))
		}
	}
}

func TestRelease_ChunkClassifiedSize_HalvedNotesShipByteIdentical(t *testing.T) {
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
	if got := tagAnnotationBody(t, f, nextTag); got != halvedNotesBody {
		t.Errorf("tag annotation body = %q, want the merge output %q", got, halvedNotesBody)
	}
}

func TestRelease_MergeClassifiedSize_Abort_TooLargeToMergeWithTheMergeOutputVerbatim(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedOverCapRelease(t, root, f, "abort")
	rec := &presentertest.RecordingPresenter{}
	tr := &partOverflowTransport{mergeErr: &ai.GenerationError{Stdout: mergeOverflowOutput + "\n", ExitCode: 1}}
	deps := newDeps(rec, f)
	deps.Transport = tr

	err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	assertAbortNonZero(t, err)
	assertNoMutation(t, f)
	if tr.calls != 4 || tr.merges != 1 || tr.classify != 1 {
		t.Errorf("AI calls = %d (merges %d, classify %d), want the two parts, the merge and its classify call only", tr.calls, tr.merges, tr.classify)
	}
	sf := onlyStageFailureEvent(t, rec)
	if sf.Name != "notes" || sf.Message != mergeTooLargePhrase {
		t.Errorf("StageFailure = %q %q, want notes %q with no location", sf.Name, sf.Message, mergeTooLargePhrase)
	}
	if sf.Output != mergeOverflowOutput {
		t.Errorf("StageFailure.Output = %q, want the merge call's output verbatim %q", sf.Output, mergeOverflowOutput)
	}
}

// produceCoreOverflow runs the fresh producer over res's own range, a two-part
// chunked run whose core/b.go part overflows the model.
func produceCoreOverflow(ctx context.Context, tr notes.Transport, root string, res version.Resolution) (string, *runner.FakeRunner, error) {
	cfg := freshCfg()
	cfg.MaxPromptBytes = 20_000
	notesGit := seedFreshGit(twoFileDiff(10_000), "M\tapi/a.go\nM\tcore/b.go\n", "156\t0\tapi/a.go\n156\t0\tcore/b.go\n")
	body, err := engine.RegenerateFreshBody(ctx, notesGit, tr, root, cfg, res, engine.NotesSinks{})
	return body, notesGit, err
}

func TestRegenerateFreshBody_ChunkClassifiedSize_HalvedOverTheVersionsRange(t *testing.T) {
	t.Parallel()

	tr := &partOverflowTransport{overflowCore: true}

	body, f, err := produceCoreOverflow(t.Context(), tr, t.TempDir(), version.Resolution{Tag: "v1.1.0", PreviousTag: "v1.0.0"})
	if err != nil {
		t.Fatalf("RegenerateFreshBody returned unexpected error: %v", err)
	}

	tr.assertCorePartHalved(t)
	if body != halvedNotesBody {
		t.Errorf("body = %q, want the merge output byte-identical %q", body, halvedNotesBody)
	}
	if !invokedWith(f, "git", "diff", "--src-prefix=a/", "--dst-prefix=b/", "--no-color", "--no-ext-diff", "v1.0.0..v1.1.0", "--", ".", ":(exclude)CHANGELOG.md", ":(exclude).mint.toml") {
		t.Errorf("the halved run did not diff the version's own range; got %v", f.Invocations())
	}
}

func TestRegenerateAll_FreshVersionChunkClassifiedSize_HalvedAndWritten(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	rec := &presentertest.RecordingPresenter{}
	tr := &partOverflowTransport{overflowCore: true}
	req := batchReq(engine.RegenerateSourceFresh, threeVersions(), true)
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, res version.Resolution, _ string, _ engine.NotesSinks) (string, error) {
		if res.Tag == batchV2Tag {
			body, _, err := produceCoreOverflow(ctx, tr, t.TempDir(), res)
			return body, err
		}
		return "## fresh " + res.Tag + "\n", nil
	}

	if _, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub, t.TempDir(), req); err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}

	tr.assertCorePartHalved(t)
	if got := dispatchedTags(pub); len(got) != 3 || got[1] != batchV2Tag {
		t.Fatalf("provider writes = %v, want all three versions with %s written", got, batchV2Tag)
	}
	if got := pub.dispatched[1].body; got != halvedNotesBody {
		t.Errorf("%s provider body = %q, want the halved run's merge output %q", batchV2Tag, got, halvedNotesBody)
	}
}
