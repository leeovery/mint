package engine_test

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// regenSplitPromptBytes is the composed single-pass prompt, nudge included, of the `r`
// that goes to chunking at its size check.
const regenSplitPromptBytes = 1_520_000

// longOverCapNudge takes a prompt from under the default cap to regenSplitPromptBytes.
var longOverCapNudge = strings.Repeat("say what each change means for operators. ", 600)

var fourFilePaths = []string{"api/a.go", "core/b.go", "web/c.go", "zz/d.go"}

// splitRegenRelease seeds a release whose first attempt goes in one call and whose
// `r` with longOverCapNudge composes a single-pass prompt of regenSplitPromptBytes.
func splitRegenRelease(t *testing.T, root string, f *runner.FakeRunner) *sizeCheckTransport {
	t.Helper()
	seedPriorTagReadGates(f, root, "main")
	release := newNearCapRelease(t, threeFilePaths, regenSplitPromptBytes-len("\n\n"+longOverCapNudge))
	release.seedGeneration(f)
	release.seedGeneration(f)
	return &sizeCheckTransport{singlePass: []string{firstSinglePass}, merges: []string{regenMergeBody}}
}

// halvedRegenRelease seeds a release over paths, its single-pass prompt 400,000 bytes,
// whose `r` with exportNudge has its single-pass call overflow, and also the first
// call carrying core/b.go when coreOverflows.
func halvedRegenRelease(t *testing.T, root string, f *runner.FakeRunner, paths []string, coreOverflows bool) *regenOverflowTransport {
	t.Helper()
	seedPriorTagReadGates(f, root, "main")
	release := newNearCapRelease(t, paths, 400_000)
	release.seedGeneration(f)
	release.seedGeneration(f)
	overflows := singlePassCall
	if coreOverflows {
		overflows = singlePassThenFirstCoreCall()
	}
	return &regenOverflowTransport{nudge: exportNudge, overflows: overflows, answer: "SIZE"}
}

// regenNotesStage returns the notes stage the run's one `r` opened, failing unless it
// closes straight before that r's notes are shown.
func regenNotesStage(t *testing.T, rec *presentertest.RecordingPresenter) []presentertest.Event {
	t.Helper()
	stages := notesStages(t, rec)
	if len(stages) != 2 {
		t.Fatalf("notes stages = %d, want the first attempt's and the r's", len(stages))
	}
	var closes []int
	for i, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageSucceeded && ev.StageSucceeded.Name == "notes" {
			closes = append(closes, i)
		}
	}
	if after := rec.Events[closes[1]+1]; after.Kind != presentertest.KindShowNotes || after.ShowNotes.Body != regenMergeBody {
		t.Errorf("the r's notes line closes before %v, want the r's notes shown", after.Kind)
	}
	return stages[1]
}

// assertStageNarrates fails unless stage is want, then the completion naming parts.
func assertStageNarrates(t *testing.T, stage, want []presentertest.Event, parts int) {
	t.Helper()
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

func progressEvent(promptBytes, parts, done int) presentertest.Event {
	return presentertest.Event{Kind: presentertest.KindNotesProgress, NotesProgress: presenter.NotesProgress{PromptBytes: promptBytes, Parts: parts, Done: done}}
}

func halvingEvent(part, parts int) presentertest.Event {
	return presentertest.Event{Kind: presentertest.KindNotesHalving, NotesHalving: presenter.NotesHalving{Part: part, Parts: parts}}
}

func mergingEvent(parts int) presentertest.Event {
	return presentertest.Event{Kind: presentertest.KindNotesMerging, NotesMerging: presenter.NotesMerge{Parts: parts}}
}

// halvedRegenStage is the narration of an `r` whose single-pass call of promptBytes
// was re-cut into parts that all went through.
func halvedRegenStage(promptBytes, parts int) []presentertest.Event {
	want := []presentertest.Event{halvingEvent(0, parts)}
	for done := 1; done <= parts; done++ {
		want = append(want, progressEvent(promptBytes, parts, done))
	}
	return append(want, mergingEvent(parts))
}

// regenSinglePassBytes is the size of the r's single-pass call that overflowed.
func (rt *regenOverflowTransport) regenSinglePassBytes(t *testing.T) int {
	t.Helper()
	rt.mu.Lock()
	defer rt.mu.Unlock()
	for _, p := range rt.failed {
		if singlePassCall(p) {
			return len(p)
		}
	}
	t.Fatalf("the r's single-pass call did not overflow")
	return 0
}

// halvedChunks counts the r's chunk calls that went through.
func (rt *regenOverflowTransport) halvedChunks() int {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return len(rt.chunks)
}

// assertRegenNarratesAsBuilt fails unless the r writing its notes in one call runs the
// notes stage again with no line plain shows: after the first notes shown, one Quiet
// blocking start and one Quiet blocking success of the notes stage, and no chunked
// run's update.
func assertRegenNarratesAsBuilt(t *testing.T, rec *presentertest.RecordingPresenter) {
	t.Helper()
	firstShown := slices.IndexFunc(rec.Events, func(ev presentertest.Event) bool { return ev.Kind == presentertest.KindShowNotes })
	var starts, successes int
	for _, ev := range rec.Events[firstShown:] {
		switch {
		case ev.Kind == presentertest.KindNotesProgress, ev.Kind == presentertest.KindNotesHalving, ev.Kind == presentertest.KindNotesMerging:
			t.Errorf("after the first notes shown, got %v, want the r to narrate no chunked run", ev.Kind)
		case ev.Kind == presentertest.KindStageStarted && ev.StageStarted.Name == "notes":
			starts++
			if s := ev.StageStarted; !s.Quiet || !s.Blocking {
				t.Errorf("the r's notes start = %+v, want a Quiet blocking start", s)
			}
		case ev.Kind == presentertest.KindStageSucceeded && ev.StageSucceeded.Name == "notes":
			successes++
			if s := ev.StageSucceeded; !s.Quiet || !s.Blocking {
				t.Errorf("the r's notes success = %+v, want a Quiet blocking success", s)
			}
		}
	}
	if starts != 1 || successes != 1 {
		t.Errorf("the r's notes starts = %d, successes = %d, want one of each", starts, successes)
	}
	if got := len(shownBodies(rec)); got != 2 {
		t.Errorf("notes shown = %d, want the first attempt's and the r's", got)
	}
}

func TestRelease_RegenOverTheCapAtItsSizeCheck_NarratesTheSplitEachPartTheMergeAndThePartCount(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	tr := splitRegenRelease(t, root, f)

	rec := releaseWithRegens(t, root, f, tr, newDeps, longOverCapNudge)

	want := []presentertest.Event{
		progressEvent(regenSplitPromptBytes, 2, 0),
		progressEvent(regenSplitPromptBytes, 2, 1),
		progressEvent(regenSplitPromptBytes, 2, 2),
		mergingEvent(2),
	}
	assertStageNarrates(t, regenNotesStage(t, rec), want, 2)
}

func TestRelease_RegenSinglePassClassifiedSize_NarratesTheRecutEachPartWithItsSizeTheMergeAndThePartCount(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	tr := halvedRegenRelease(t, root, f, threeFilePaths, false)

	rec := releaseWithRegens(t, root, f, tr, newDeps, exportNudge)

	if got := tr.halvedChunks(); got != 3 {
		t.Fatalf("fixture: the r's single-pass call was re-cut into %d parts, want 3", got)
	}
	assertStageNarrates(t, regenNotesStage(t, rec), halvedRegenStage(tr.regenSinglePassBytes(t), 3), 3)
}

func TestRelease_ChunkedRegensPartClassifiedSize_NarratesThePartsRecutAndCountsOutOfTheGrownTotal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	tr := halvedRegenRelease(t, root, f, fourFilePaths, true)

	rec := releaseWithRegens(t, root, f, tr, newDeps, exportNudge)

	stage := regenNotesStage(t, rec)
	if stage[0].Kind != presentertest.KindNotesHalving || stage[0].NotesHalving != (presenter.NotesHalving{Parts: 4}) {
		t.Fatalf("first r event = %v %+v, want its single-pass call re-cut into 4 parts", stage[0].Kind, stage[0].NotesHalving)
	}
	recutAt := slices.IndexFunc(stage[1:], func(ev presentertest.Event) bool { return ev.Kind == presentertest.KindNotesHalving }) + 1
	if recutAt == 0 {
		t.Fatalf("notes stage events = %v, want part 2's re-cut narrated", kindsOf(stage))
	}
	recut := stage[recutAt].NotesHalving
	if recut.Part != 2 || recut.Parts < 2 {
		t.Fatalf("re-cut = %+v, want part 2 re-cut into smaller parts", recut)
	}
	total := 4 + recut.Parts - 1
	size := tr.regenSinglePassBytes(t)
	var want []presentertest.Event
	want = append(want, halvingEvent(0, 4))
	for done := 1; done < recutAt; done++ {
		want = append(want, progressEvent(size, 4, done))
	}
	want = append(want, halvingEvent(2, recut.Parts))
	for done := recutAt; done <= total; done++ {
		want = append(want, progressEvent(size, total, done))
	}
	want = append(want, mergingEvent(total))
	assertStageNarrates(t, stage, want, total)
}

// plainRegenNotesLines runs a release on the plain presenter whose review gate takes
// one `r` with nudge, then accepts, and returns the notes lines printed after the
// first attempt's.
func plainRegenNotesLines(t *testing.T, root string, f *runner.FakeRunner, tr *sizeCheckTransport, rt *regenOverflowTransport, nudge string) []string {
	t.Helper()
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	deps := newDeps(&presentertest.RecordingPresenter{}, f)
	if tr != nil {
		deps.Transport = tr
	} else {
		deps.Transport = rt
	}
	out := &bytes.Buffer{}
	// Accepts the version gate, takes `r` at the review gate, then accepts.
	deps.Presenter = presenter.NewPlainPresenterWithInput(out, &bytes.Buffer{}, strings.NewReader("y\nr\n"+nudge+"\ny\n"))
	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}
	var lines []string
	for line := range strings.Lines(out.String()) {
		// The nudge prompt ends its line only with the operator's echoed answer.
		line = strings.TrimPrefix(line, "Add one-time context for regeneration (optional):: ")
		if strings.HasPrefix(line, "notes: ") {
			lines = append(lines, strings.TrimSuffix(line, "\n"))
		}
	}
	// The first attempt's notes stage is its start, the exclusion check's line and its
	// completion.
	lines = withoutPlainCheckLine(t, lines)
	if len(lines) < 2 || lines[0] != "notes: running..." || !strings.HasPrefix(lines[1], "notes: generated (") {
		t.Fatalf("notes lines =\n%s\nwant the first attempt's start and completion first", strings.Join(lines, "\n"))
	}
	return lines[2:]
}

func assertPlainLines(t *testing.T, got, want []string, parts int) {
	t.Helper()
	if len(got) != len(want)+1 {
		t.Fatalf("r's notes lines =\n%s\nwant\n%s\nthen the completion", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("r's notes line %d = %q, want %q", i+1, got[i], want[i])
		}
	}
	if prefix := fmt.Sprintf("notes: generated in %d parts, merged (", parts); !strings.HasPrefix(got[len(got)-1], prefix) {
		t.Errorf("last line = %q, want the completion naming the part count", got[len(got)-1])
	}
}

func plainProgress(size string, parts, done int) string {
	return fmt.Sprintf("notes: %s release, in %d parts, %d of %d done", size, parts, done, parts)
}

func TestRelease_RegenOverTheCap_PlainMode_PrintsEachUpdateAsItsOwnLine(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	tr := splitRegenRelease(t, root, f)

	lines := plainRegenNotesLines(t, root, f, tr, nil, longOverCapNudge)

	assertPlainLines(t, lines, []string{
		"notes: running...",
		plainProgress("1.5 MB", 2, 0),
		plainProgress("1.5 MB", 2, 1),
		plainProgress("1.5 MB", 2, 2),
		"notes: merging 2 parts",
	}, 2)
}

func TestRelease_RegenSinglePassClassifiedSize_PlainMode_PrintsTheRecutAndEachPartAsLines(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	rt := halvedRegenRelease(t, root, f, threeFilePaths, false)

	lines := plainRegenNotesLines(t, root, f, nil, rt, exportNudge)

	size := presenterSize(rt.regenSinglePassBytes(t))
	assertPlainLines(t, lines, []string{
		"notes: running...",
		"notes: too big for the model in one call, retrying in 3 parts",
		plainProgress(size, 3, 1),
		plainProgress(size, 3, 2),
		plainProgress(size, 3, 3),
		"notes: merging 3 parts",
	}, 3)
}

func TestRelease_ChunkedRegensPartClassifiedSize_PlainMode_PrintsThePartsRecutAsItsOwnLine(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	rt := halvedRegenRelease(t, root, f, fourFilePaths, true)

	lines := plainRegenNotesLines(t, root, f, nil, rt, exportNudge)

	recuts := 0
	for _, line := range lines {
		if _, err := fmt.Sscanf(line, "notes: part 2 was too big for the model, retrying it as %d smaller parts", &recuts); err == nil {
			break
		}
	}
	if recuts < 2 {
		t.Fatalf("r's notes lines =\n%s\nwant part 2's re-cut as its own line", strings.Join(lines, "\n"))
	}
	total := 4 + recuts - 1
	// The start, the re-cut into 4, one line per part done, part 2's re-cut, the merge and the completion.
	if len(lines) != total+5 {
		t.Fatalf("r's notes lines =\n%s\nwant %d", strings.Join(lines, "\n"), total+5)
	}
	if want := "notes: too big for the model in one call, retrying in 4 parts"; lines[1] != want {
		t.Errorf("line 2 = %q, want %q", lines[1], want)
	}
	if want := fmt.Sprintf("notes: merging %d parts", total); lines[len(lines)-2] != want {
		t.Errorf("merge line = %q, want %q", lines[len(lines)-2], want)
	}
	if prefix := fmt.Sprintf("notes: generated in %d parts, merged (", total); !strings.HasPrefix(lines[len(lines)-1], prefix) {
		t.Errorf("last line = %q, want the completion naming the part count", lines[len(lines)-1])
	}
}

// presenterSize writes bytes as the notes line does, for the sizes these fixtures use.
func presenterSize(bytes int) string {
	return fmt.Sprintf("%.0f KB", float64(bytes)/1e3)
}

func TestRelease_RegenThatMakesOneCall_NarratesAsBuilt(t *testing.T) {
	t.Parallel()

	t.Run("merge-only r on chunked notes", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeConfig(t, root, partCapToml)
		f := runner.NewFakeRunner()
		seedPriorTagReadGates(f, root, "main")
		seedSixPartNotes(f)
		seedSixPartAI(f, firstRemergeBody)

		rec := releaseWithRegens(t, root, f, nil, newDeps, exportNudge)

		assertRegenNarratesAsBuilt(t, rec)
	})

	t.Run("single-pass r in one call", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		f := runner.NewFakeRunner()
		seedPriorTagReadGates(f, root, "main")
		release := newNearCapRelease(t, threeFilePaths, 400_000)
		release.seedGeneration(f)
		release.seedGeneration(f)
		tr := &sizeCheckTransport{singlePass: []string{firstSinglePass, regenSinglePass}}

		rec := releaseWithRegens(t, root, f, tr, newDeps, exportNudge)

		assertRegenNarratesAsBuilt(t, rec)
	})
}

func TestRegenerateRun_RegenClassifiedSize_NarratesAsAChunkedRun(t *testing.T) {
	t.Parallel()

	notesGit := runner.NewFakeRunner()
	seedTwoFileGenerations(notesGit, 10_000, 2)
	tr := &regenOverflowTransport{nudge: exportNudge, overflows: singlePassCall, answer: "SIZE"}

	run := regenerateV140(t, notesGit, tr, freshCfg(), regenChoices([]string{exportNudge}), exportNudge)

	parts := tr.halvedChunks()
	assertStageNarrates(t, regenNotesStage(t, run.rec), halvedRegenStage(tr.regenSinglePassBytes(t), parts), parts)
}

func TestRegenerateRun_RegenInOneCall_NarratesAsBuilt(t *testing.T) {
	t.Parallel()

	notesGit := runner.NewFakeRunner()
	seedTwoFileGenerations(notesGit, 10_000, 2)
	tr := &regenOverflowTransport{nudge: exportNudge, onePassNudges: []string{exportNudge}, overflows: singlePassCall, answer: "SIZE"}

	run := regenerateV140(t, notesGit, tr, freshCfg(), regenChoices([]string{exportNudge}), exportNudge)

	assertRegenNarratesAsBuilt(t, run.rec)
}

func TestRegenerateAll_RegenClassifiedSize_NarratesAsAChunkedRun(t *testing.T) {
	t.Parallel()

	tr := &regenOverflowTransport{nudge: exportNudge, overflows: singlePassCall, answer: "SIZE"}

	rec, _ := regenerateAllWithRegens(t, tr, exportNudge)

	parts := tr.halvedChunks()
	assertStageNarrates(t, regenNotesStage(t, rec), halvedRegenStage(tr.regenSinglePassBytes(t), parts), parts)
}

func TestRegenerateAll_RegenInOneCall_NarratesAsBuilt(t *testing.T) {
	t.Parallel()

	tr := &regenOverflowTransport{nudge: exportNudge, onePassNudges: []string{exportNudge}, overflows: singlePassCall, answer: "SIZE"}

	rec, _ := regenerateAllWithRegens(t, tr, exportNudge)

	assertRegenNarratesAsBuilt(t, rec)
}
