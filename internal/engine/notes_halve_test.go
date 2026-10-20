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

// halvedNotesBody is the merge output that ends a halved run.
const halvedNotesBody = "✨ Added\n- The api and core areas, written in halves   \n"

// sizeOverflowTransport fails every single-pass call with an overflow's output and
// answers its classify call SIZE, then writes the halved run: notes for each chunk
// call and halvedNotesBody for the merge.
type sizeOverflowTransport struct {
	mu       sync.Mutex
	single   []string
	classify int
	chunks   []string
	merges   int
}

func (st *sizeOverflowTransport) Generate(_ context.Context, prompt string) (string, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	switch {
	case isClassifyPrompt(prompt):
		st.classify++
		return "SIZE", nil
	case strings.Contains(prompt, "What follows is part "):
		st.chunks = append(st.chunks, prompt)
		return "- part notes", nil
	case strings.Contains(prompt, "diff --git "):
		st.single = append(st.single, prompt)
		return "", &ai.GenerationError{Stdout: "Prompt is too long\n", ExitCode: 1}
	default:
		st.merges++
		return halvedNotesBody, nil
	}
}

// assertHalved fails unless one single-pass call failed, one classify call read it
// as SIZE, and chunk calls each at most half that call's size, then one merge, wrote
// the notes.
func (st *sizeOverflowTransport) assertHalved(t *testing.T) {
	t.Helper()
	st.mu.Lock()
	defer st.mu.Unlock()
	if len(st.single) != 1 || st.classify != 1 {
		t.Fatalf("single-pass calls = %d, classify calls = %d, want one of each", len(st.single), st.classify)
	}
	if len(st.chunks) < 2 || st.merges != 1 {
		t.Fatalf("chunk calls = %d, merge calls = %d, want a chunked run ending in one merge", len(st.chunks), st.merges)
	}
	failedSize := len(st.single[0])
	for i, p := range st.chunks {
		if 2*len(p) > failedSize {
			t.Errorf("chunk call %d is %d bytes, over half the failed call's %d", i, len(p), failedSize)
		}
	}
}

func TestRelease_SinglePassClassifiedSize_HalvedNotesShipByteIdentical(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git",
		ScriptedOut(twoFileDiff(10_000)),
		ScriptedOut("M\tapi/a.go\nM\tcore/b.go\n"),
		ScriptedOut("156\t0\tapi/a.go\n156\t0\tcore/b.go\n"),
	)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}
	tr := &sizeOverflowTransport{}
	deps := newDeps(rec, f)
	deps.Transport = tr

	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	tr.assertHalved(t)
	if got := tagAnnotationBody(t, f, nextTag); got != halvedNotesBody {
		t.Errorf("tag annotation body = %q, want the merge output %q", got, halvedNotesBody)
	}
	if got := changelogSectionBody(t, root, "1.2.4"); got != halvedNotesBody {
		t.Errorf("CHANGELOG body = %q, want the merge output %q", got, halvedNotesBody)
	}
}

// produceHalved runs the fresh producer over res's own range, a two-file diff under
// the default cap whose single-pass call overflows.
func produceHalved(ctx context.Context, tr notes.Transport, root string, res version.Resolution) (string, *runner.FakeRunner, error) {
	notesGit := seedFreshGit(twoFileDiff(10_000), "M\tapi/a.go\nM\tcore/b.go\n", "156\t0\tapi/a.go\n156\t0\tcore/b.go\n")
	body, err := engine.RegenerateFreshBody(ctx, notesGit, tr, root, freshCfg(), res, engine.NotesSinks{})
	return body, notesGit, err
}

func TestRegenerateFreshBody_SinglePassClassifiedSize_HalvedOverTheVersionsRange(t *testing.T) {
	t.Parallel()

	tr := &sizeOverflowTransport{}

	body, f, err := produceHalved(t.Context(), tr, t.TempDir(), version.Resolution{Tag: "v1.1.0", PreviousTag: "v1.0.0"})
	if err != nil {
		t.Fatalf("RegenerateFreshBody returned unexpected error: %v", err)
	}

	tr.assertHalved(t)
	if body != halvedNotesBody {
		t.Errorf("body = %q, want the merge output byte-identical %q", body, halvedNotesBody)
	}
	if !invokedWith(f, "git", "diff", "v1.0.0..v1.1.0", "--", ".", ":(exclude)CHANGELOG.md") {
		t.Errorf("the halved run did not diff the version's own range; got %v", f.Invocations())
	}
}

func TestRegenerateAll_FreshVersionClassifiedSize_HalvedAndWritten(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	rec := &presentertest.RecordingPresenter{}
	tr := &sizeOverflowTransport{}
	req := batchReq(engine.RegenerateSourceFresh, threeVersions(), true)
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, res version.Resolution, _ string, _ engine.NotesSinks) (string, error) {
		if res.Tag == batchV2Tag {
			body, _, err := produceHalved(ctx, tr, t.TempDir(), res)
			return body, err
		}
		return "## fresh " + res.Tag + "\n", nil
	}

	if _, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub, t.TempDir(), req); err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}

	tr.assertHalved(t)
	if got := dispatchedTags(pub); len(got) != 3 || got[1] != batchV2Tag {
		t.Fatalf("provider writes = %v, want all three versions with %s written", got, batchV2Tag)
	}
	if got := pub.dispatched[1].body; got != halvedNotesBody {
		t.Errorf("%s provider body = %q, want the halved run's merge output %q", batchV2Tag, got, halvedNotesBody)
	}
}
