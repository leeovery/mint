package engine_test

import (
	"context"
	"slices"
	"testing"

	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// freshGenerations scripts generations full fresh generations' git calls over release.
func freshGenerations(release nearCapRelease, generations int) *runner.FakeRunner {
	f := runner.NewFakeRunner()
	for range generations {
		release.seedGeneration(f)
	}
	return f
}

// regenChoices answers the review gate with one `r` per nudge, then accepts.
func regenChoices(nudges []string) []presenter.Choice {
	var choices []presenter.Choice
	for range nudges {
		choices = append(choices, presenter.ChoiceRegen)
	}
	return append(choices, presenter.ChoiceYes)
}

// regenerateAllV140 runs `mint release regenerate --all` over v1.4.0 alone, fresh, at a
// terminal without -y, answering its review gate with one `r` per nudge.
func regenerateAllV140(t *testing.T, notesGit *runner.FakeRunner, tr notes.Transport, nudges ...string) *fakePublisher {
	t.Helper()
	res := version.Resolution{Tag: regenRunTag, PreviousTag: "v1.3.0"}
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(res.Tag, true, nil)
	rec := &presentertest.RecordingPresenter{
		NextChoices: regenChoices(nudges),
		NextLines:   nudges,
	}
	cfg := freshCfg()
	req := batchReq(engine.RegenerateSourceFresh, []version.Resolution{res}, false)
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, res version.Resolution, _ string, sinks engine.NotesSinks) (string, error) {
		return engine.RegenerateFreshBody(ctx, notesGit, tr, "", cfg, res, sinks)
	}
	req.ProduceRegenerator = func(_ engine.RegenerateSource, res version.Resolution, merged *notes.MergeInputs) engine.Regenerator {
		return engine.RegenerateFreshRegenerator(notesGit, tr, "", cfg, res, merged)
	}
	if _, err := engine.RegenerateAll(t.Context(), regenWriteDeps(rec, f), pub, t.TempDir(), req); err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}
	return pub
}

func TestRegenerateRun_RegenOnSinglePassFreshNotes_WithinTheCap_MakesOneSinglePassCallWithTheNudge(t *testing.T) {
	t.Parallel()

	notesGit := freshGenerations(newNearCapRelease(t, threeFilePaths, 400_000), 2)
	tr := &sizeCheckTransport{singlePass: []string{firstSinglePass, regenSinglePass}}

	run := regenerateV140(t, notesGit, tr, freshCfg(), regenChoices([]string{exportNudge}), exportNudge)

	if got := len(tr.prompts); got != 2 {
		t.Fatalf("AI calls = %d, want the first generation and one single-pass call for the r", got)
	}
	if want := withNudge(t, tr.prompts[0], exportNudge); tr.prompts[1] != want {
		t.Errorf("r's prompt is not the first generation's prompt with the nudge appended to its instructions")
	}
	if got, want := dispatchedBodies(run.pub), []string{regenSinglePass}; !slices.Equal(got, want) {
		t.Errorf("release bodies written = %q, want the r's notes %q", got, want)
	}
}

func TestRegenerateRun_RegenWhoseNudgeTakesThePromptOverTheCap_WritesTheNotesInPartsCarryingTheNudge(t *testing.T) {
	t.Parallel()

	notesGit := freshGenerations(newNearCapRelease(t, threeFilePaths, nearCapPromptBytes), 2)
	tr := &sizeCheckTransport{singlePass: []string{firstSinglePass}, merges: []string{regenMergeBody}}

	run := regenerateV140(t, notesGit, tr, freshCfg(), regenChoices([]string{overCapNudge}), overCapNudge)

	if got := len(tr.prompts[0]); got != nearCapPromptBytes {
		t.Fatalf("first generation's prompt = %d bytes, want %d", got, nearCapPromptBytes)
	}
	assertChunkedRegen(t, tr.regenPrompts(), overCapNudge)
	if got, want := shownBodies(run.rec), []string{firstSinglePass, regenMergeBody}; !slices.Equal(got, want) {
		t.Errorf("notes shown = %q, want the first generation's then the merge output %q", got, want)
	}
	if got, want := dispatchedBodies(run.pub), []string{regenMergeBody}; !slices.Equal(got, want) {
		t.Errorf("release bodies written = %q, want the merge output %q", got, want)
	}
}

func TestRegenerateRun_RegenAfterAnRegenTheLadderChunked_ReMergesThatRegensPartsWithOnlyTheNewNudge(t *testing.T) {
	t.Parallel()

	notesGit := freshGenerations(newNearCapRelease(t, threeFilePaths, nearCapPromptBytes), 2)
	tr := &sizeCheckTransport{singlePass: []string{firstSinglePass}, merges: []string{regenMergeBody, regenRemergeBody}}
	nudges := []string{overCapNudge, fixesNudge}

	run := regenerateV140(t, notesGit, tr, freshCfg(), regenChoices(nudges), nudges...)

	regen := tr.regenPrompts()
	assertChunkedRegen(t, regen[:len(regen)-1], overCapNudge)
	assertReMergeWithOnlyTheNewNudge(t, regen[len(regen)-2], regen[len(regen)-1], overCapNudge, fixesNudge)
	if got := len(notesGit.Invocations()); got != 6 {
		t.Errorf("notes git calls = %d, want the first generation's and the chunked r's only", got)
	}
	if got, want := dispatchedBodies(run.pub), []string{regenRemergeBody}; !slices.Equal(got, want) {
		t.Errorf("release bodies written = %q, want the re-merge output %q", got, want)
	}
}

func TestRegenerateAll_RegenOnSinglePassNotes_WithinTheCap_MakesOneSinglePassCallWithTheNudge(t *testing.T) {
	t.Parallel()

	notesGit := freshGenerations(newNearCapRelease(t, threeFilePaths, 400_000), 2)
	tr := &sizeCheckTransport{singlePass: []string{firstSinglePass, regenSinglePass}}

	pub := regenerateAllV140(t, notesGit, tr, exportNudge)

	if got := len(tr.prompts); got != 2 {
		t.Fatalf("AI calls = %d, want the first generation and one single-pass call for the r", got)
	}
	if want := withNudge(t, tr.prompts[0], exportNudge); tr.prompts[1] != want {
		t.Errorf("r's prompt is not the first generation's prompt with the nudge appended to its instructions")
	}
	if got, want := dispatchedBodies(pub), []string{regenSinglePass}; !slices.Equal(got, want) {
		t.Errorf("release bodies written = %q, want the r's notes %q", got, want)
	}
}

func TestRegenerateAll_RegenWhoseNudgeTakesThePromptOverTheCap_WritesTheNotesInPartsAndAFurtherRegenReMerges(t *testing.T) {
	t.Parallel()

	notesGit := freshGenerations(newNearCapRelease(t, threeFilePaths, nearCapPromptBytes), 2)
	tr := &sizeCheckTransport{singlePass: []string{firstSinglePass}, merges: []string{regenMergeBody, regenRemergeBody}}

	pub := regenerateAllV140(t, notesGit, tr, overCapNudge, fixesNudge)

	regen := tr.regenPrompts()
	assertChunkedRegen(t, regen[:len(regen)-1], overCapNudge)
	assertReMergeWithOnlyTheNewNudge(t, regen[len(regen)-2], regen[len(regen)-1], overCapNudge, fixesNudge)
	if got, want := dispatchedBodies(pub), []string{regenRemergeBody}; !slices.Equal(got, want) {
		t.Errorf("release bodies written = %q, want the re-merge output %q", got, want)
	}
}
