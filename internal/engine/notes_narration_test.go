package engine_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// notesStages returns, per notes stage in recorded order, the events between its
// StageStarted and its StageSucceeded, followed by that StageSucceeded.
func notesStages(t *testing.T, rec *presentertest.RecordingPresenter) [][]presentertest.Event {
	t.Helper()
	var stages [][]presentertest.Event
	var current []presentertest.Event
	inStage := false
	for _, ev := range rec.Events {
		switch {
		case ev.Kind == presentertest.KindStageStarted && ev.StageStarted.Name == "notes":
			inStage, current = true, nil
		case !inStage:
		case ev.Kind == presentertest.KindStageSucceeded && ev.StageSucceeded.Name == "notes":
			stages = append(stages, append(current, ev))
			inStage = false
		default:
			current = append(current, ev)
		}
	}
	if inStage {
		t.Fatalf("a notes stage never completed; kinds = %v", rec.Kinds())
	}
	return stages
}

// assertChunkedNotesStage asserts stage narrates a two-part chunked run over a
// single-pass prompt over maxPromptBytes: the split, each part, the merge, then the
// completion naming the part count.
func assertChunkedNotesStage(t *testing.T, stage []presentertest.Event, maxPromptBytes int) {
	t.Helper()
	wantKinds := []presentertest.EventKind{
		presentertest.KindNotesProgress,
		presentertest.KindNotesProgress,
		presentertest.KindNotesProgress,
		presentertest.KindNotesMerging,
		presentertest.KindStageSucceeded,
	}
	if len(stage) != len(wantKinds) {
		t.Fatalf("notes stage events = %v, want %v", kindsOf(stage), wantKinds)
	}
	for i, want := range wantKinds {
		if stage[i].Kind != want {
			t.Fatalf("notes stage events = %v, want %v", kindsOf(stage), wantKinds)
		}
	}
	size := stage[0].NotesProgress.PromptBytes
	if size <= maxPromptBytes {
		t.Errorf("split size = %d bytes, want the single-pass prompt over the %d cap", size, maxPromptBytes)
	}
	for done := range 3 {
		want := presenter.NotesProgress{PromptBytes: size, Parts: 2, Done: done}
		if got := stage[done].NotesProgress; got != want {
			t.Errorf("progress %d = %+v, want %+v", done, got, want)
		}
	}
	if got, want := stage[3].NotesMerging, (presenter.NotesMerge{Parts: 2}); got != want {
		t.Errorf("merge = %+v, want %+v", got, want)
	}
	done := stage[4].StageSucceeded
	if done.Sentence != "Notes written (2 parts, merged)" {
		t.Errorf("completion sentence = %q, want %q", done.Sentence, "Notes written (2 parts, merged)")
	}
	if done.Detail != "generated in 2 parts, merged" {
		t.Errorf("completion detail = %q, want %q", done.Detail, "generated in 2 parts, merged")
	}
	if !done.Blocking {
		t.Errorf("completion Blocking = false, want true so the elapsed time shows")
	}
}

// assertSinglePassNotesStage asserts stage narrates as built: the completion alone.
func assertSinglePassNotesStage(t *testing.T, stage []presentertest.Event) {
	t.Helper()
	if len(stage) != 1 {
		t.Fatalf("notes stage events = %v, want only the completion", kindsOf(stage))
	}
	done := stage[0].StageSucceeded
	if done.Sentence != "Generated release notes" || done.Detail != "generated" || !done.Blocking {
		t.Errorf("completion = %+v, want the as-built blocking %q / %q", done, "Generated release notes", "generated")
	}
}

func kindsOf(events []presentertest.Event) []presentertest.EventKind {
	kinds := make([]presentertest.EventKind, len(events))
	for i, ev := range events {
		kinds[i] = ev.Kind
	}
	return kinds
}

func TestRelease_OverCap_NarratesTheSplitEachPartTheMergeAndThePartCount(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "max_prompt_bytes = 20000\n")
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedOverCapNotes(f)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	stages := notesStages(t, rec)
	if len(stages) != 1 {
		t.Fatalf("notes stages = %d, want one", len(stages))
	}
	assertChunkedNotesStage(t, stages[0], 20_000)
}

func TestRelease_OverCap_PlainMode_PrintsEachUpdateAsItsOwnLine(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "max_prompt_bytes = 20000\n")
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedOverCapNotes(f)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	out := &bytes.Buffer{}
	deps := newDeps(&presentertest.RecordingPresenter{}, f)
	// Accepts the version gate and the review gate.
	deps.Presenter = presenter.NewPlainPresenterWithInput(out, &bytes.Buffer{}, strings.NewReader("y\ny\n"))

	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	var notesLines []string
	for line := range strings.Lines(out.String()) {
		if strings.HasPrefix(line, "notes: ") {
			notesLines = append(notesLines, strings.TrimSuffix(line, "\n"))
		}
	}
	if len(notesLines) != 6 {
		t.Fatalf("notes lines =\n%s\nwant the start, the split, two parts, the merge and the completion", strings.Join(notesLines, "\n"))
	}
	if notesLines[0] != "notes: running..." {
		t.Errorf("line 1 = %q, want the stage start", notesLines[0])
	}
	for i, suffix := range []string{" release, in 2 parts, 0 of 2 done", " release, in 2 parts, 1 of 2 done", " release, in 2 parts, 2 of 2 done"} {
		if !strings.HasSuffix(notesLines[i+1], suffix) {
			t.Errorf("line %d = %q, want it to end %q", i+2, notesLines[i+1], suffix)
		}
	}
	if notesLines[4] != "notes: merging 2 parts" {
		t.Errorf("line 5 = %q, want the merge", notesLines[4])
	}
	if !strings.HasPrefix(notesLines[5], "notes: generated in 2 parts, merged (") {
		t.Errorf("line 6 = %q, want the completion naming the part count", notesLines[5])
	}
}

func TestRelease_UnderCap_NotesStageNarratesAsBuilt(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	// The same diff, under the default cap: one call, answered with the first script.
	seedOverCapNotes(f)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	stages := notesStages(t, rec)
	if len(stages) != 1 {
		t.Fatalf("notes stages = %d, want one", len(stages))
	}
	assertSinglePassNotesStage(t, stages[0])
}

// overCapFreshBody produces a fresh version's notes over a two-file range diff whose
// single-pass prompt is over a 20,000-byte cap.
func overCapFreshBody(ctx context.Context, res version.Resolution, progress notes.Progress) (string, error) {
	notesGit := seedFreshGit(twoFileDiff(10_000), "M\tapi/a.go\nM\tcore/b.go\n", "156\t0\tapi/a.go\n156\t0\tcore/b.go\n")
	cfg := freshCfg()
	cfg.MaxPromptBytes = 20_000
	return engine.RegenerateFreshBody(ctx, notesGit, &splitTransport{mergeBody: "## merged\n"}, "", cfg, res, progress)
}

func TestRegenerateRun_FreshOverCap_NarratesLikeAChunkedRelease(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, true, nil)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease), false)
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, progress notes.Progress) (string, error) {
		return overCapFreshBody(ctx, version.Resolution{Tag: regenRunTag, PreviousTag: "v1.3.0"}, progress)
	}

	if err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, dir, req); err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	stages := notesStages(t, rec)
	if len(stages) != 1 {
		t.Fatalf("notes stages = %d, want one", len(stages))
	}
	assertChunkedNotesStage(t, stages[0], 20_000)
}

func TestRegenerateAll_FreshOverCapVersions_EachNarrateLikeAChunkedRelease(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	rec := &presentertest.RecordingPresenter{}

	req := batchReq(engine.RegenerateSourceFresh, threeVersions(), true)
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, res version.Resolution, _ string, progress notes.Progress) (string, error) {
		return overCapFreshBody(ctx, res, progress)
	}

	if _, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub, req); err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}

	stages := notesStages(t, rec)
	if len(stages) != 3 {
		t.Fatalf("notes stages = %d, want one per version", len(stages))
	}
	// The oldest version is its first release: its notes take no AI call.
	assertSinglePassNotesStage(t, stages[0])
	assertChunkedNotesStage(t, stages[1], 20_000)
	assertChunkedNotesStage(t, stages[2], 20_000)
}

func TestRelease_OverCap_FailedChunkedRunFallsBack_NotesStageClaimsNoMerge(t *testing.T) {
	t.Parallel()

	failed := runner.ScriptedCall{Result: runner.Result{Stdout: "boom", ExitCode: 1}, Err: errors.New("exit status 1")}
	tests := []struct {
		name      string
		succeeded []runner.ScriptedCall
	}{
		{name: "second chunk call fails", succeeded: []runner.ScriptedCall{ScriptedOut("- api notes")}},
		{name: "merge call fails", succeeded: []runner.ScriptedCall{ScriptedOut("- api notes"), ScriptedOut("- core notes")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeConfig(t, root, "max_prompt_bytes = 20000\n\n[release]\non_notes_failure = \"fallback\"\n")
			f := runner.NewFakeRunner()
			seedPriorTagReadGates(f, root, "main")
			f.SeedSequence("git",
				ScriptedOut(twoFileDiff(10_000)),
				ScriptedOut("M\tapi/a.go\nM\tcore/b.go\n"),
				ScriptedOut("156\t0\tapi/a.go\n156\t0\tcore/b.go\n"),
				ScriptedOut("subject one\nsubject two\n"), // log --format=%s (the fallback body)
			)
			f.SeedSequence("claude", tt.succeeded...)
			f.Seed("claude", failed.Result, failed.Err)
			seedRecordTagPush(f, root)
			f.Seed("gh", runner.Result{}, nil)
			rec := &presentertest.RecordingPresenter{}

			if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			stages := notesStages(t, rec)
			if len(stages) != 1 {
				t.Fatalf("notes stages = %d, want one", len(stages))
			}
			done := stages[0][len(stages[0])-1].StageSucceeded
			if done.Sentence != "Generated release notes" || done.Detail != "generated" {
				t.Errorf("completion = %q / %q, want the as-built %q / %q", done.Sentence, done.Detail, "Generated release notes", "generated")
			}
			if got := tagAnnotationBody(t, f, nextTag); got != "subject one\nsubject two\n" {
				t.Errorf("tag annotation body = %q, want the commit-subject fallback", got)
			}
		})
	}
}
