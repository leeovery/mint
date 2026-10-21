package engine_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// seedHalvedRelease seeds a release whose two-file diff sits under the default cap,
// so its notes go in one call.
func seedHalvedRelease(root string, f *runner.FakeRunner) {
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git",
		ScriptedOut(twoFileDiff(10_000)),
		ScriptedOut("M\tapi/a.go\nM\tcore/b.go\n"),
		ScriptedOut("156\t0\tapi/a.go\n156\t0\tcore/b.go\n"),
	)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
}

// assertHalvedSinglePassStage asserts stage narrates a single-pass call tr re-cut:
// the re-cut, each part finishing counted out of its parts with the failed call's
// size, the merge, then the completion naming the part count.
func assertHalvedSinglePassStage(t *testing.T, stage []presentertest.Event, tr *sizeOverflowTransport) {
	t.Helper()
	tr.mu.Lock()
	parts, size := len(tr.chunks), len(tr.single[0])
	tr.mu.Unlock()

	var want []presentertest.Event
	want = append(want, presentertest.Event{Kind: presentertest.KindNotesHalving, NotesHalving: presenter.NotesHalving{Parts: parts}})
	for done := 1; done <= parts; done++ {
		want = append(want, presentertest.Event{Kind: presentertest.KindNotesProgress, NotesProgress: presenter.NotesProgress{PromptBytes: size, Parts: parts, Done: done}})
	}
	want = append(want, presentertest.Event{Kind: presentertest.KindNotesMerging, NotesMerging: presenter.NotesMerge{Parts: parts}})
	if len(stage) != len(want)+1 {
		t.Fatalf("notes stage events = %v, want %d before the completion", kindsOf(stage), len(want))
	}
	for i, ev := range want {
		got := stage[i]
		if got.Kind != ev.Kind || got.NotesHalving != ev.NotesHalving || got.NotesProgress != ev.NotesProgress || got.NotesMerging != ev.NotesMerging {
			t.Errorf("event %d = %v %+v %+v %+v, want %v %+v %+v %+v", i, got.Kind, got.NotesHalving, got.NotesProgress, got.NotesMerging, ev.Kind, ev.NotesHalving, ev.NotesProgress, ev.NotesMerging)
		}
	}
	assertMergedCompletion(t, stage[len(stage)-1], parts)
}

func assertMergedCompletion(t *testing.T, ev presentertest.Event, parts int) {
	t.Helper()
	done := ev.StageSucceeded
	if ev.Kind != presentertest.KindStageSucceeded {
		t.Fatalf("last notes stage event = %v, want the completion", ev.Kind)
	}
	if want := fmt.Sprintf("Notes written (%d parts, merged)", parts); done.Sentence != want {
		t.Errorf("completion sentence = %q, want %q", done.Sentence, want)
	}
	if want := fmt.Sprintf("generated in %d parts, merged", parts); done.Detail != want {
		t.Errorf("completion detail = %q, want %q", done.Detail, want)
	}
	if !done.Blocking {
		t.Errorf("completion Blocking = false, want true so the elapsed time shows")
	}
}

func onlyNotesStage(t *testing.T, rec *presentertest.RecordingPresenter) []presentertest.Event {
	t.Helper()
	stages := notesStages(t, rec)
	if len(stages) != 1 {
		t.Fatalf("notes stages = %d, want one", len(stages))
	}
	return stages[0]
}

// plainNotesLines runs a release on the plain presenter and returns its notes lines,
// the exclusion check's line left out.
func plainNotesLines(t *testing.T, deps engine.ReleaseDeps) []string {
	t.Helper()
	out := &bytes.Buffer{}
	// Accepts the version gate and the review gate.
	deps.Presenter = presenter.NewPlainPresenterWithInput(out, &bytes.Buffer{}, strings.NewReader("y\ny\n"))
	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}
	var lines []string
	for line := range strings.Lines(out.String()) {
		if strings.HasPrefix(line, "notes: ") {
			lines = append(lines, strings.TrimSuffix(line, "\n"))
		}
	}
	return withoutPlainCheckLine(t, lines)
}

func TestRelease_SinglePassClassifiedSize_NarratesTheRecutEachPartTheMergeAndThePartCount(t *testing.T) {
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

	assertHalvedSinglePassStage(t, releaseNotesStage(t, rec), tr)
}

func TestRelease_SinglePassClassifiedSize_PlainMode_PrintsTheRecutAndEachPartAsLines(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHalvedRelease(root, f)
	tr := &sizeOverflowTransport{}
	deps := newDeps(&presentertest.RecordingPresenter{}, f)
	deps.Transport = tr

	lines := plainNotesLines(t, deps)

	parts := len(tr.chunks)
	if len(lines) != parts+4 {
		t.Fatalf("notes lines =\n%s\nwant the start, the re-cut, one per part, the merge and the completion", strings.Join(lines, "\n"))
	}
	if want := fmt.Sprintf("notes: too big for the model in one call, retrying in %d parts", parts); lines[1] != want {
		t.Errorf("line 2 = %q, want %q", lines[1], want)
	}
	for done := 1; done <= parts; done++ {
		if suffix := fmt.Sprintf(" release, in %d parts, %d of %d done", parts, done, parts); !strings.HasSuffix(lines[done+1], suffix) {
			t.Errorf("line %d = %q, want it to end %q", done+2, lines[done+1], suffix)
		}
	}
	if want := fmt.Sprintf("notes: merging %d parts", parts); lines[parts+2] != want {
		t.Errorf("line %d = %q, want %q", parts+3, lines[parts+2], want)
	}
	if prefix := fmt.Sprintf("notes: generated in %d parts, merged (", parts); !strings.HasPrefix(lines[parts+3], prefix) {
		t.Errorf("last line = %q, want the completion naming the part count", lines[parts+3])
	}
}

// coreRecuts counts the calls core/b.go's overflowed part was re-cut into.
func coreRecuts(t *testing.T, tr *partOverflowTransport) int {
	t.Helper()
	tr.mu.Lock()
	defer tr.mu.Unlock()
	n := 0
	for _, p := range tr.chunks {
		if strings.Contains(p, "diff --git a/core/b.go") {
			n++
		}
	}
	if n < 2 {
		t.Fatalf("calls carrying core/b.go after its overflow = %d, want its part re-cut", n)
	}
	return n
}

func TestRelease_ChunkClassifiedSize_NarratesThePartsRecutAndCountsOutOfTheGrownTotal(t *testing.T) {
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

	stage := releaseNotesStage(t, rec)
	recuts := coreRecuts(t, tr)
	total := 2 + recuts - 1
	halvedAt := -1
	for i, ev := range stage {
		if ev.Kind != presentertest.KindNotesHalving {
			continue
		}
		if halvedAt >= 0 {
			t.Fatalf("notes stage events = %v, want one re-cut", kindsOf(stage))
		}
		halvedAt = i
		if want := (presenter.NotesHalving{Part: 2, Parts: recuts}); ev.NotesHalving != want {
			t.Errorf("re-cut = %+v, want %+v", ev.NotesHalving, want)
		}
	}
	if halvedAt < 0 {
		t.Fatalf("notes stage events = %v, want the re-cut narrated", kindsOf(stage))
	}
	var last presenter.NotesProgress
	for i, ev := range stage {
		if ev.Kind != presentertest.KindNotesProgress {
			continue
		}
		parts := 2
		if i > halvedAt {
			parts = total
		}
		if ev.NotesProgress.Parts != parts {
			t.Errorf("progress %d = %+v, want it counted out of %d parts", i, ev.NotesProgress, parts)
		}
		last = ev.NotesProgress
	}
	if last.Done != total || last.Parts != total {
		t.Errorf("last progress = %+v, want %d of %d done", last, total, total)
	}
	if merge := stage[len(stage)-2]; merge.Kind != presentertest.KindNotesMerging || merge.NotesMerging.Parts != total {
		t.Errorf("event before the completion = %v %+v, want merging %d parts", merge.Kind, merge.NotesMerging, total)
	}
	assertMergedCompletion(t, stage[len(stage)-1], total)
}

func TestRelease_ChunkClassifiedSize_PlainMode_PrintsTheRecutAsItsOwnLine(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedOverCapRelease(t, root, f, "abort")
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	tr := &partOverflowTransport{overflowCore: true}
	deps := newDeps(&presentertest.RecordingPresenter{}, f)
	deps.Transport = tr

	lines := plainNotesLines(t, deps)

	recuts := coreRecuts(t, tr)
	total := 2 + recuts - 1
	// The start, the split, one line per part done, the re-cut, the merge and the completion.
	if len(lines) != total+5 {
		t.Fatalf("notes lines =\n%s\nwant %d", strings.Join(lines, "\n"), total+5)
	}
	recut := fmt.Sprintf("notes: part 2 was too big for the model, retrying it as %d smaller parts", recuts)
	found := false
	for _, line := range lines {
		found = found || line == recut
	}
	if !found {
		t.Errorf("notes lines =\n%s\nwant the line %q", strings.Join(lines, "\n"), recut)
	}
	if want := fmt.Sprintf("notes: merging %d parts", total); lines[len(lines)-2] != want {
		t.Errorf("merge line = %q, want %q", lines[len(lines)-2], want)
	}
	if prefix := fmt.Sprintf("notes: generated in %d parts, merged (", total); !strings.HasPrefix(lines[len(lines)-1], prefix) {
		t.Errorf("last line = %q, want the completion naming the part count", lines[len(lines)-1])
	}
}

// halvedFreshBody produces a fresh version's notes over a two-file range diff under
// the default cap whose single-pass call tr overflows.
func halvedFreshBody(ctx context.Context, tr notes.Transport, res version.Resolution, sinks engine.NotesSinks) (string, error) {
	notesGit := seedFreshGit(twoFileDiff(10_000), "M\tapi/a.go\nM\tcore/b.go\n", "156\t0\tapi/a.go\n156\t0\tcore/b.go\n")
	return engine.RegenerateFreshBody(ctx, notesGit, tr, "", freshCfg(), res, sinks)
}

func TestRegenerateRun_FreshSinglePassClassifiedSize_NarratesLikeAHalvedRelease(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, true, nil)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	tr := &sizeOverflowTransport{}

	req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease))
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, sinks engine.NotesSinks) (string, error) {
		return halvedFreshBody(ctx, tr, version.Resolution{Tag: regenRunTag, PreviousTag: "v1.3.0"}, sinks)
	}

	if err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, dir, req); err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	assertHalvedSinglePassStage(t, onlyNotesStage(t, rec), tr)
}

func TestRegenerateAll_FreshVersionClassifiedSize_ItsNotesLineNarratesTheRecut(t *testing.T) {
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
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, res version.Resolution, _ string, sinks engine.NotesSinks) (string, error) {
		if res.Tag == batchV2Tag {
			return halvedFreshBody(ctx, tr, res, sinks)
		}
		return "## fresh " + res.Tag + "\n", nil
	}

	if _, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub, t.TempDir(), req); err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}

	stages := notesStages(t, rec)
	if len(stages) != 3 {
		t.Fatalf("notes stages = %d, want one per version", len(stages))
	}
	assertSinglePassNotesStage(t, stages[0])
	assertHalvedSinglePassStage(t, stages[1], tr)
	assertSinglePassNotesStage(t, stages[2])
}
