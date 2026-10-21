package engine_test

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

const (
	fivePartMergeBody = "✨ Added\n- Five parts, merged once   \n"
	regenRemergeOne   = "✨ Added\n- The export feature, re-merged   \n"
	regenRemergeTwo   = "✨ Added\n- Fixes, grouped and re-merged   \n"
)

var fivePartPaths = []string{"a/1.go", "b/2.go", "c/3.go", "d/4.go", "e/5.go"}

// seedFivePartFreshGit scripts a fresh range of five files that chunks into five parts
// under a 20,000-byte cap.
func seedFivePartFreshGit() *runner.FakeRunner {
	var diff, nameStatus, numstat strings.Builder
	for _, path := range fivePartPaths {
		diff.WriteString("diff --git a/" + path + " b/" + path + "\n@@ -1 +1,9 @@\n")
		diff.WriteString(strings.Repeat("+"+strings.Repeat("x", 63)+"\n", 10_000/64))
		nameStatus.WriteString("M\t" + path + "\n")
		numstat.WriteString("156\t0\t" + path + "\n")
	}
	return seedFreshGit(diff.String(), nameStatus.String(), numstat.String())
}

func chunkingCfg() config.Config {
	cfg := freshCfg()
	cfg.MaxPromptBytes = 20_000
	return cfg
}

// mergeTransport answers each chunk call with notes naming label and its part, the
// first merge with firstMerge and each later merge with the next of remerges. It
// records every prompt and every merge prompt in arrival order.
type mergeTransport struct {
	label      string
	firstMerge string
	remerges   []string

	mu      sync.Mutex
	prompts []string
	merges  []string
}

func (mt *mergeTransport) Generate(_ context.Context, prompt string) (string, error) {
	if isCheckPrompt(prompt) {
		return "none", nil
	}
	mt.mu.Lock()
	defer mt.mu.Unlock()
	mt.prompts = append(mt.prompts, prompt)
	if m := partFraming.FindStringSubmatch(prompt); m != nil {
		return "- " + mt.label + " notes for part " + m[1] + " of " + m[2], nil
	}
	mt.merges = append(mt.merges, prompt)
	if len(mt.merges) == 1 {
		return mt.firstMerge, nil
	}
	if i := len(mt.merges) - 2; i < len(mt.remerges) {
		return mt.remerges[i], nil
	}
	return "unscripted merge", nil
}

// freshRunOf wires req's body production and `r` to the fresh path over res, the way
// production does: the regenerator is handed whatever merge inputs the body's run kept.
func freshRunOf(req *engine.RegenerateRunRequest, notesGit *runner.FakeRunner, tr notes.Transport, cfg config.Config, res version.Resolution) {
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, sinks engine.NotesSinks) (string, error) {
		return engine.RegenerateFreshBody(ctx, notesGit, tr, "", cfg, res, sinks)
	}
	req.ProduceRegenerator = func(_ engine.RegenerateSource, run engine.RegenRun) engine.Regenerator {
		return engine.RegenerateFreshRegenerator(notesGit, tr, "", cfg, res, run)
	}
}

type regenRemergeRun struct {
	rec *presentertest.RecordingPresenter
	pub *fakePublisher
}

// regenerateV140 runs `mint release regenerate v1.4.0 --source fresh --target release`
// over tr and notesGit, answering the review gate with choices and its `r` nudges with
// nudges.
func regenerateV140(t *testing.T, notesGit *runner.FakeRunner, tr notes.Transport, cfg config.Config, choices []presenter.Choice, nudges ...string) regenRemergeRun {
	t.Helper()
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	run := regenRemergeRun{
		rec: &presentertest.RecordingPresenter{NextChoices: choices, NextLines: nudges},
		pub: newFakePublisher(),
	}
	run.pub.seedExists(regenRunTag, true, nil)
	req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease))
	freshRunOf(&req, notesGit, tr, cfg, version.Resolution{Tag: regenRunTag, PreviousTag: "v1.3.0"})
	if err := engine.RegenerateRun(t.Context(), regenWriteDeps(run.rec, f), run.pub, t.TempDir(), req); err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}
	return run
}

func shownBodies(rec *presentertest.RecordingPresenter) []string {
	var shown []string
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindShowNotes {
			shown = append(shown, ev.ShowNotes.Body)
		}
	}
	return shown
}

func reviewGateAsks(rec *presentertest.RecordingPresenter) int {
	var n int
	for _, g := range promptGates(rec) {
		if g.Question == presenter.NotesReviewGate().Question {
			n++
		}
	}
	return n
}

func dispatchedBodies(pub *fakePublisher) []string {
	bodies := make([]string, len(pub.dispatched))
	for i, d := range pub.dispatched {
		bodies[i] = d.body
	}
	return bodies
}

func TestRegenerateRun_RegenOnChunkedFreshNotes_MakesOneMergeCallCarryingTheNudge(t *testing.T) {
	t.Parallel()

	notesGit := seedFivePartFreshGit()
	tr := &mergeTransport{label: "v1.4.0", firstMerge: fivePartMergeBody, remerges: []string{regenRemergeOne}}

	regenerateV140(t, notesGit, tr, chunkingCfg(), []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceYes}, exportNudge)

	if got := len(tr.prompts); got != 7 {
		t.Fatalf("AI calls = %d, want five chunk calls, the merge and one re-merge", got)
	}
	if len(tr.merges) != 2 {
		t.Fatalf("merge calls = %d, want the merge and one re-merge", len(tr.merges))
	}
	remerge := tr.merges[1]
	if tr.prompts[6] != remerge {
		t.Errorf("the call after the merge is not the re-merge")
	}
	if want := withNudge(t, tr.merges[0], exportNudge); remerge != want {
		t.Errorf("re-merge prompt = %q, want the first merge's prompt with the nudge in its instructions %q", remerge, want)
	}
	if strings.Contains(remerge, "diff --git ") {
		t.Errorf("the re-merge call carries diff")
	}
	assertPartsInOrder(t, remerge, len(fivePartPaths))
	if got := len(notesGit.Invocations()); got != 3 {
		t.Errorf("notes git calls = %d, want the first generation's diff and Change Map only", got)
	}
}

func TestRegenerateRun_RegenOnChunkedFreshNotes_ReShowsTheReMergeAndWritesItByteIdentical(t *testing.T) {
	t.Parallel()

	tr := &mergeTransport{label: "v1.4.0", firstMerge: fivePartMergeBody, remerges: []string{regenRemergeOne}}

	run := regenerateV140(t, seedFivePartFreshGit(), tr, chunkingCfg(), []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceYes}, exportNudge)

	if got, want := shownBodies(run.rec), []string{fivePartMergeBody, regenRemergeOne}; !slices.Equal(got, want) {
		t.Errorf("notes shown = %q, want the merge output then the re-merge output %q", got, want)
	}
	if got := reviewGateAsks(run.rec); got != 2 {
		t.Errorf("review gate asked %d times, want once before the r and once after", got)
	}
	if got, want := dispatchedBodies(run.pub), []string{regenRemergeOne}; !slices.Equal(got, want) {
		t.Errorf("release bodies written = %q, want the re-merge output %q", got, want)
	}
}

func TestRegenerateRun_SecondRegenOnChunkedFreshNotes_ReMergesTheSamePartsWithOnlyItsOwnNudge(t *testing.T) {
	t.Parallel()

	tr := &mergeTransport{label: "v1.4.0", firstMerge: fivePartMergeBody, remerges: []string{regenRemergeOne, regenRemergeTwo}}

	run := regenerateV140(t, seedFivePartFreshGit(), tr, chunkingCfg(),
		[]presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceRegen, presenter.ChoiceYes}, exportNudge, fixesNudge)

	if got := len(tr.prompts); got != 8 {
		t.Fatalf("AI calls = %d, want five chunk calls, the merge and two re-merges", got)
	}
	if len(tr.merges) != 3 {
		t.Fatalf("merge calls = %d, want the merge and two re-merges", len(tr.merges))
	}
	second := tr.merges[2]
	if want := withNudge(t, tr.merges[0], fixesNudge); second != want {
		t.Errorf("second re-merge prompt = %q, want the first merge's prompt with only its own nudge %q", second, want)
	}
	if strings.Contains(second, exportNudge) {
		t.Errorf("the second re-merge carries the first nudge")
	}
	if strings.Contains(second, regenRemergeOne) {
		t.Errorf("the second re-merge carries the first re-merge's output")
	}
	if got, want := dispatchedBodies(run.pub), []string{regenRemergeTwo}; !slices.Equal(got, want) {
		t.Errorf("release bodies written = %q, want the second re-merge output %q", got, want)
	}
}

func TestRegenerateAll_RegenOnAChunkedVersion_ReMergesOnlyThatVersionsPartNotes(t *testing.T) {
	t.Parallel()

	versions := []version.Resolution{
		{Tag: "v1.2.0", PreviousTag: "v1.1.0"},
		{Tag: "v1.3.0", PreviousTag: "v1.2.0"},
	}
	transports := map[string]*mergeTransport{
		"v1.2.0": {label: "v1.2.0", firstMerge: "✨ Added\n- v1.2.0 merged   \n"},
		"v1.3.0": {label: "v1.3.0", firstMerge: "✨ Added\n- v1.3.0 merged   \n", remerges: []string{regenRemergeOne}},
	}
	notesGits := map[string]*runner.FakeRunner{"v1.2.0": seedFivePartFreshGit(), "v1.3.0": seedFivePartFreshGit()}
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	pub := newFakePublisher()
	for _, res := range versions {
		pub.seedExists(res.Tag, true, nil)
	}
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen, presenter.ChoiceYes},
		NextLines:   []string{exportNudge},
	}
	cfg := chunkingCfg()
	req := batchReq(engine.RegenerateSourceFresh, versions, false)
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, res version.Resolution, _ string, sinks engine.NotesSinks) (string, error) {
		return engine.RegenerateFreshBody(ctx, notesGits[res.Tag], transports[res.Tag], "", cfg, res, sinks)
	}
	req.ProduceRegenerator = func(_ engine.RegenerateSource, res version.Resolution, run engine.RegenRun) engine.Regenerator {
		return engine.RegenerateFreshRegenerator(notesGits[res.Tag], transports[res.Tag], "", cfg, res, run)
	}

	if _, err := engine.RegenerateAll(t.Context(), regenWriteDeps(rec, f), pub, t.TempDir(), req); err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}

	if got := len(transports["v1.2.0"].prompts); got != 6 {
		t.Errorf("v1.2.0 AI calls = %d, want its five chunk calls and its merge", got)
	}
	v13 := transports["v1.3.0"]
	if got := len(v13.prompts); got != 7 {
		t.Fatalf("v1.3.0 AI calls = %d, want five chunk calls, the merge and one re-merge", got)
	}
	if len(v13.merges) != 2 {
		t.Fatalf("v1.3.0 merge calls = %d, want the merge and one re-merge", len(v13.merges))
	}
	remerge := v13.merges[1]
	if want := withNudge(t, v13.merges[0], exportNudge); remerge != want {
		t.Errorf("v1.3.0 re-merge prompt = %q, want its merge's prompt with the nudge %q", remerge, want)
	}
	assertPartsInOrder(t, remerge, len(fivePartPaths))
	if strings.Contains(remerge, "v1.2.0 notes") {
		t.Errorf("v1.3.0's re-merge carries v1.2.0's part notes")
	}
	if got, want := dispatchedBodies(pub), []string{"✨ Added\n- v1.2.0 merged   \n", regenRemergeOne}; !slices.Equal(got, want) {
		t.Errorf("release bodies written = %q, want v1.2.0's merge then v1.3.0's re-merge %q", got, want)
	}
}

func TestRegenerateRun_RegenOnHalvedSinglePassFreshNotes_MakesOneMergeCallOverTheParts(t *testing.T) {
	t.Parallel()

	var diff strings.Builder
	for _, path := range []string{"api/a.go", "core/b.go", "web/c.go"} {
		diff.WriteString("diff --git a/" + path + " b/" + path + "\n@@ -1 +1,9 @@\n")
		diff.WriteString(strings.Repeat("+"+strings.Repeat("x", 63)+"\n", 10_000/64))
	}
	notesGit := seedFreshGit(diff.String(), "M\tapi/a.go\nM\tcore/b.go\nM\tweb/c.go\n", "156\t0\tapi/a.go\n156\t0\tcore/b.go\n156\t0\tweb/c.go\n")
	tr := &halvedToThreeTransport{}

	run := regenerateV140(t, notesGit, tr, freshCfg(), []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceYes}, exportNudge)

	if tr.chunks != 3 || len(tr.merges) != 2 {
		t.Fatalf("chunk calls = %d, merge calls = %d, want a halved run of three parts and one re-merge", tr.chunks, len(tr.merges))
	}
	if got := len(tr.prompts); got != 7 {
		t.Errorf("AI calls = %d, want the single-pass call, its classify call, three chunk calls, the merge and one re-merge", got)
	}
	if want := withNudge(t, tr.merges[0], exportNudge); tr.merges[1] != want {
		t.Errorf("re-merge prompt = %q, want the first merge's prompt with the nudge in its instructions %q", tr.merges[1], want)
	}
	assertPartsInOrder(t, tr.merges[1], 3)
	if got, want := dispatchedBodies(run.pub), []string{halvedRemergeBody}; !slices.Equal(got, want) {
		t.Errorf("release bodies written = %q, want the re-merge output %q", got, want)
	}
}
