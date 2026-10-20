package engine_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// regenOverflowOutput is what an `r` call that overflowed the model printed.
const regenOverflowOutput = "Prompt is too long · the nudged request is over the limit"

// regenOverflowTransport scripts a release's first attempt and its `r`s. A call
// carrying none of the nudges is the first attempt's, answered firstSinglePass. A call
// carrying one of onePassNudges is a single-pass `r` that succeeds, answered
// regenSinglePass. Of the calls carrying nudge, those overflows picks fail with failure,
// or an overflow's output when failure is unset; its chunk and piece calls get notes,
// its merge regenMergeBody, and a single-pass call that does not overflow
// regenSinglePass. Every classify call is answered answer.
type regenOverflowTransport struct {
	nudge         string
	onePassNudges []string
	overflows     func(prompt string) bool
	failure       error
	answer        string

	mu       sync.Mutex
	first    []string
	onePass  []string
	classify []string
	failed   []string
	chunks   []string
	merges   []string
}

func (rt *regenOverflowTransport) Generate(_ context.Context, prompt string) (string, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	switch {
	case isClassifyPrompt(prompt):
		rt.classify = append(rt.classify, prompt)
		return rt.answer, nil
	case slices.ContainsFunc(rt.onePassNudges, func(n string) bool { return strings.HasPrefix(prompt, nudgedInstructions(n)) }):
		rt.onePass = append(rt.onePass, prompt)
		return regenSinglePass, nil
	case !strings.HasPrefix(prompt, nudgedInstructions(rt.nudge)):
		rt.first = append(rt.first, prompt)
		return firstSinglePass, nil
	case !strings.Contains(prompt, "diff --git "):
		rt.merges = append(rt.merges, prompt)
		return regenMergeBody, nil
	case rt.overflows(prompt):
		rt.failed = append(rt.failed, prompt)
		if rt.failure != nil {
			return "", rt.failure
		}
		return "", &ai.GenerationError{Stdout: regenOverflowOutput + "\n", ExitCode: 1}
	case partFraming.MatchString(prompt):
		rt.chunks = append(rt.chunks, prompt)
		return "- r notes for " + partFraming.FindString(prompt), nil
	default:
		return regenSinglePass, nil
	}
}

func singlePassCall(prompt string) bool { return !partFraming.MatchString(prompt) }

// singlePassThenFirstCoreCall overflows the single-pass call and the first chunk call
// carrying core/b.go whole or its first piece.
func singlePassThenFirstCoreCall() func(string) bool {
	coreOverflowed := false
	return func(prompt string) bool {
		if singlePassCall(prompt) {
			return true
		}
		if coreOverflowed || !strings.Contains(prompt, "diff --git a/core/b.go") {
			return false
		}
		if strings.Contains(prompt, ": piece ") && !strings.Contains(prompt, ": piece 1 of ") {
			return false
		}
		coreOverflowed = true
		return true
	}
}

// singlePassAndEveryFirstPart overflows the single-pass call and every chunk call
// framed as part 1.
func singlePassAndEveryFirstPart(prompt string) bool {
	return singlePassCall(prompt) || strings.Contains(prompt, "What follows is part 1 of ")
}

// firstCoreCall overflows the first chunk call carrying core/b.go.
func firstCoreCall() func(string) bool {
	overflows := singlePassThenFirstCoreCall()
	return func(prompt string) bool { return !singlePassCall(prompt) && overflows(prompt) }
}

// seedTwoFileGenerations scripts generations full notes generations' git calls over a
// two-file diff of about fileBytes a file, under the default cap.
func seedTwoFileGenerations(f *runner.FakeRunner, fileBytes, generations int) {
	for range generations {
		f.SeedSequence("git",
			ScriptedOut(twoFileDiff(fileBytes)),
			ScriptedOut("M\tapi/a.go\nM\tcore/b.go\n"),
			ScriptedOut("156\t0\tapi/a.go\n156\t0\tcore/b.go\n"),
		)
	}
}

// releaseWithOneRegen runs a prior-tag release over a two-file diff of about
// fileBytes a file whose review gate takes one `r` with tr's nudge, then accepts.
func releaseWithOneRegen(t *testing.T, tr *regenOverflowTransport, fileBytes int) (*presentertest.RecordingPresenter, *runner.FakeRunner, error) {
	t.Helper()
	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedTwoFileGenerations(f, fileBytes, 2)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen, presenter.ChoiceYes},
		NextLines:   []string{tr.nudge},
	}
	deps := newDeps(rec, f)
	deps.Transport = tr
	return rec, f, engine.Release(t.Context(), deps, priorTagNormalAIOptions())
}

// assertRegenHalved fails unless the r's single-pass call overflowed once, one classify
// call read it, and chunk calls each at most half its size, then one merge, every one
// carrying the nudge, wrote the r's notes.
func (rt *regenOverflowTransport) assertRegenHalved(t *testing.T) {
	t.Helper()
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if len(rt.failed) != 1 || !singlePassCall(rt.failed[0]) || len(rt.classify) != 1 {
		t.Fatalf("overflowed calls = %d, classify calls = %d, want the r's single-pass call and one classify call", len(rt.failed), len(rt.classify))
	}
	if !strings.Contains(rt.classify[0], regenOverflowOutput) {
		t.Errorf("classify prompt does not carry the r call's output %q", regenOverflowOutput)
	}
	if len(rt.chunks) < 2 || len(rt.merges) != 1 {
		t.Fatalf("chunk calls = %d, merge calls = %d, want the r halved into a chunked run ending in one merge", len(rt.chunks), len(rt.merges))
	}
	failedSize := len(rt.failed[0])
	for i, p := range rt.chunks {
		if 2*len(p) > failedSize {
			t.Errorf("chunk call %d is %d bytes, over half the failed r call's %d", i+1, len(p), failedSize)
		}
		if !strings.HasPrefix(p, nudgedInstructions(rt.nudge)) {
			t.Errorf("chunk call %d does not carry the nudge", i+1)
		}
	}
	if !strings.HasPrefix(rt.merges[0], nudgedInstructions(rt.nudge)) {
		t.Errorf("the merge call does not carry the nudge")
	}
}

// assertCoreSliceHalved fails unless a chunk call of the r overflowed and the
// succeeded calls carrying core/b.go, at least two, are each at most half its size and
// carry the nudge.
func (rt *regenOverflowTransport) assertCoreSliceHalved(t *testing.T) {
	t.Helper()
	rt.mu.Lock()
	defer rt.mu.Unlock()
	var overflowedPart string
	for _, p := range rt.failed {
		if !singlePassCall(p) {
			overflowedPart = p
		}
	}
	if overflowedPart == "" {
		t.Fatalf("no chunk call of the r overflowed")
	}
	var halved []string
	for _, p := range rt.chunks {
		if strings.Contains(p, "diff --git a/core/b.go") {
			halved = append(halved, p)
		}
	}
	if len(halved) < 2 {
		t.Fatalf("calls carrying core/b.go after its overflow = %d, want its slice re-sent in smaller calls", len(halved))
	}
	for i, p := range halved {
		if 2*len(p) > len(overflowedPart) {
			t.Errorf("halved call %d is %d bytes, over half the failed call's %d", i+1, len(p), len(overflowedPart))
		}
		if !strings.HasPrefix(p, nudgedInstructions(rt.nudge)) {
			t.Errorf("halved call %d does not carry the nudge", i+1)
		}
	}
	if len(rt.merges) != 1 {
		t.Errorf("merge calls = %d, want one", len(rt.merges))
	}
}

func reviewAsks(rec *presentertest.RecordingPresenter, gate presenter.Gate) int {
	n := 0
	for _, g := range promptGates(rec) {
		if g.Question == gate.Question {
			n++
		}
	}
	return n
}

func TestRelease_RegenCallFailsWithOutput_ClassifyRunsTheReleaseCommandAndTimeoutOnTheRegensOutput(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, perVerbConfig)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	seedNormalAINotes(f)
	failed := runner.ScriptedCall{
		Result: runner.Result{Stdout: regenOverflowOutput + "\n", ExitCode: 1},
		Err:    errors.New("exit status 1"),
	}
	f.SeedSequence("tool-a", ScriptedOut(firstSinglePass), failed, failed, ScriptedOut("OTHER\n"))
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen},
		NextLines:   []string{exportNudge},
	}
	d := &deadlineRunner{FakeRunner: f}
	deps := newDeps(rec, f)
	deps.Runner = d

	err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	assertAbortNonZero(t, err)
	if len(d.calls) != 4 {
		t.Fatalf("AI calls = %d, want the first attempt, the r's two attempts, then one classify call", len(d.calls))
	}
	for i, call := range d.calls {
		if call.name != "tool-a" {
			t.Errorf("AI call %d ran %q, want the [release].ai_command %q", i+1, call.name, "tool-a")
		}
	}
	if !strings.Contains(d.calls[1].stdin, exportNudge) {
		t.Fatalf("fixture: the second AI call is not the r's")
	}
	classify := d.calls[3]
	if !isClassifyPrompt(classify.stdin) || !strings.Contains(classify.stdin, regenOverflowOutput) {
		t.Errorf("fourth AI call stdin =\n%.400s\nwant the classify prompt carrying the r call's output", classify.stdin)
	}
	if classify.remaining > 30*time.Second || classify.remaining < 25*time.Second {
		t.Errorf("classify call deadline remaining = %v, want about the [release].timeout of 30s", classify.remaining)
	}
}

func TestRelease_RegenClassifiedSize_HalvesAtHalfTheNudgedCallAndShowsTheMergeAtTheGateAgain(t *testing.T) {
	t.Parallel()

	tr := &regenOverflowTransport{nudge: exportNudge, overflows: singlePassCall, answer: "SIZE"}

	rec, f, err := releaseWithOneRegen(t, tr, 10_000)
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	tr.assertRegenHalved(t)
	if got, want := shownBodies(rec), []string{firstSinglePass, regenMergeBody}; !slices.Equal(got, want) {
		t.Errorf("notes shown = %q, want the first attempt's then the r's merge output %q", got, want)
	}
	if got := reviewAsks(rec, presenter.ReleaseReviewGate()); got != 2 {
		t.Errorf("review gate asked %d times, want once before the r and once after", got)
	}
	if got := tagAnnotationBody(t, f, nextTag); got != regenMergeBody {
		t.Errorf("tag annotation body = %q, want the r's merge output %q", got, regenMergeBody)
	}
}

func TestRelease_RegensHalvedPartClassifiedSize_ReCutsThatSliceCarryingTheNudgeAndStillMerges(t *testing.T) {
	t.Parallel()

	tr := &regenOverflowTransport{nudge: exportNudge, overflows: singlePassThenFirstCoreCall(), answer: "SIZE"}

	_, f, err := releaseWithOneRegen(t, tr, 100_000)
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	tr.assertCoreSliceHalved(t)
	if got := len(tr.classify); got != 2 {
		t.Errorf("classify calls = %d, want the r's single-pass call's and its core part's", got)
	}
	if got := tagAnnotationBody(t, f, nextTag); got != regenMergeBody {
		t.Errorf("tag annotation body = %q, want the r's merge output %q", got, regenMergeBody)
	}
}

func TestRelease_RegenOverTheCapWhosePartIsClassifiedSize_ReCutsThatSliceCarryingTheNudge(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	release := newNearCapRelease(t, threeFilePaths, nearCapPromptBytes)
	release.seedGeneration(f)
	release.seedGeneration(f)
	tr := &regenOverflowTransport{nudge: overCapNudge, overflows: firstCoreCall(), answer: "SIZE"}

	releaseWithRegens(t, root, f, tr, newDeps, overCapNudge)

	tr.assertCoreSliceHalved(t)
	if got := tagAnnotationBody(t, f, nextTag); got != regenMergeBody {
		t.Errorf("tag annotation body = %q, want the r's merge output %q", got, regenMergeBody)
	}
}

func TestRelease_RegensSliceClassifiedSizeAFourthTime_FailsTheRegen(t *testing.T) {
	t.Parallel()

	tr := &regenOverflowTransport{nudge: exportNudge, overflows: singlePassAndEveryFirstPart, answer: "SIZE"}

	rec, _, err := releaseWithOneRegen(t, tr, 100_000)

	assertAbortNonZero(t, err)
	if !errors.Is(err, notes.ErrTooLargeForModel) {
		t.Errorf("Release error = %v, want the r failed as too large for the model", err)
	}
	if got := len(tr.failed); got != 4 {
		t.Errorf("overflowed r calls = %d, want the single-pass call and its first part in three halved rounds", got)
	}
	if len(tr.merges) != 0 {
		t.Errorf("merge calls = %d, want none", len(tr.merges))
	}
	if got := shownBodies(rec); !slices.Equal(got, []string{firstSinglePass}) {
		t.Errorf("notes shown = %q, want only the first attempt's", got)
	}
}

func TestRelease_RegenThatCannotBeHalved_FailsWithNoChunkCall(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name          string
		answer        string
		failure       error
		wantClassify  int
		wantErrorType error
	}{
		{name: "classified OTHER", answer: "OTHER", wantClassify: 1, wantErrorType: ai.ErrGenerationFailed},
		{name: "timed out", answer: "SIZE", failure: fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout), wantErrorType: ai.ErrTimeout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tr := &regenOverflowTransport{nudge: exportNudge, overflows: singlePassCall, failure: tc.failure, answer: tc.answer}

			rec, f, err := releaseWithOneRegen(t, tr, 10_000)

			assertAbortNonZero(t, err)
			if !errors.Is(err, tc.wantErrorType) {
				t.Errorf("Release error = %v, want one matching %v", err, tc.wantErrorType)
			}
			if got := len(tr.classify); got != tc.wantClassify {
				t.Errorf("classify calls = %d, want %d", got, tc.wantClassify)
			}
			if len(tr.chunks) != 0 || len(tr.merges) != 0 {
				t.Errorf("chunk calls = %d, merge calls = %d, want none", len(tr.chunks), len(tr.merges))
			}
			if got := shownBodies(rec); !slices.Equal(got, []string{firstSinglePass}) {
				t.Errorf("notes shown = %q, want only the first attempt's", got)
			}
			assertNoMutation(t, f)
		})
	}
}

// assertHintAfterRegen fails unless rec shows one lower-the-cap hint naming bytes,
// straight after the r's notes and straight before the gate asks again.
func assertHintAfterRegen(t *testing.T, rec *presentertest.RecordingPresenter, bytes int, gate presenter.Gate) {
	t.Helper()
	assertLowerCapHint(t, rec, bytes, gate)
	at := lowerCapHints(rec)[0]
	if got := rec.Events[at-1].ShowNotes.Body; got != regenMergeBody {
		t.Errorf("the hint follows notes %q, want the r's %q", got, regenMergeBody)
	}
}

func TestRelease_RegenWrittenAfterHalving_HintsTheLargestSucceededCallOfTheRunBeforeTheGateAsksAgain(t *testing.T) {
	t.Parallel()

	tr := &regenOverflowTransport{nudge: exportNudge, overflows: singlePassCall, answer: "SIZE"}

	rec, _, err := releaseWithOneRegen(t, tr, 10_000)
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if len(tr.first) != 1 || len(tr.first[0]) <= largest(tr.chunks) {
		t.Fatalf("fixture: the first attempt's call is not the run's largest succeeded call")
	}
	assertHintAfterRegen(t, rec, len(tr.first[0]), presenter.ReleaseReviewGate())
}

func TestRelease_RegenWrittenAfterItsPartHalved_HintsBeforeTheGateAsksAgain(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	release := newNearCapRelease(t, threeFilePaths, nearCapPromptBytes)
	release.seedGeneration(f)
	release.seedGeneration(f)
	tr := &regenOverflowTransport{nudge: overCapNudge, overflows: firstCoreCall(), answer: "SIZE"}

	rec := releaseWithRegens(t, root, f, tr, newDeps, overCapNudge)

	assertHintAfterRegen(t, rec, nearCapPromptBytes, presenter.ReleaseReviewGate())
}

func TestRelease_RegenNotHalved_NoHint(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		nudge string
		bytes int
	}{
		{name: "one single-pass call", nudge: exportNudge, bytes: 400_000},
		{name: "chunked over the cap", nudge: overCapNudge, bytes: nearCapPromptBytes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			seedPriorTagReadGates(f, root, "main")
			release := newNearCapRelease(t, threeFilePaths, tc.bytes)
			release.seedGeneration(f)
			release.seedGeneration(f)
			tr := &regenOverflowTransport{nudge: tc.nudge, overflows: func(string) bool { return false }, answer: "SIZE"}

			rec := releaseWithRegens(t, root, f, tr, newDeps, tc.nudge)

			assertNoLowerCapHint(t, rec)
		})
	}
}

func TestRelease_ReMergeAfterARegenThatHalved_HintsOnlyAfterTheHalvedRegen(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedTwoFileGenerations(f, 10_000, 2)
	tr := &regenOverflowTransport{nudge: exportNudge, overflows: singlePassCall, answer: "SIZE"}

	rec := releaseWithRegens(t, root, f, tr, newDeps, exportNudge, exportNudge)

	if got := len(tr.merges); got != 2 {
		t.Fatalf("merge calls = %d, want the halved r's and the re-merge", got)
	}
	at := lowerCapHints(rec)
	if len(at) != 1 {
		t.Fatalf("lower-the-cap hints = %d, want one, after the halved r's notes", len(at))
	}
	if shown := shownBodies(rec); len(shown) != 3 {
		t.Fatalf("notes shown = %d, want the first attempt's and each r's", len(shown))
	}
	var notesBefore int
	for _, ev := range rec.Events[:at[0]] {
		if ev.Kind == presentertest.KindShowNotes {
			notesBefore++
		}
	}
	if notesBefore != 2 {
		t.Errorf("the hint follows notes shown %d, want the halved r's, the second shown", notesBefore)
	}
}

func TestRegenerateRun_RegenClassifiedSize_HalvesAndHintsBeforeTheGateAsksAgain(t *testing.T) {
	t.Parallel()

	notesGit := runner.NewFakeRunner()
	seedTwoFileGenerations(notesGit, 10_000, 2)
	tr := &regenOverflowTransport{nudge: exportNudge, overflows: singlePassCall, answer: "SIZE"}

	run := regenerateV140(t, notesGit, tr, freshCfg(), regenChoices([]string{exportNudge}), exportNudge)

	tr.assertRegenHalved(t)
	if got, want := dispatchedBodies(run.pub), []string{regenMergeBody}; !slices.Equal(got, want) {
		t.Errorf("release bodies written = %q, want the r's merge output %q", got, want)
	}
	assertHintAfterRegen(t, run.rec, len(tr.first[0]), presenter.NotesReviewGate())
}

// regenerateAllWithRegens runs `mint release regenerate --all` over v1.4.0 alone,
// fresh, at a terminal without -y, over a two-file diff, answering its review gate
// with one `r` per nudge, then accepting.
func regenerateAllWithRegens(t *testing.T, tr notes.Transport, nudges ...string) (*presentertest.RecordingPresenter, *fakePublisher) {
	t.Helper()
	notesGit := runner.NewFakeRunner()
	seedTwoFileGenerations(notesGit, 10_000, 1+len(nudges))
	res := version.Resolution{Tag: regenRunTag, PreviousTag: "v1.3.0"}
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(res.Tag, true, nil)
	rec := &presentertest.RecordingPresenter{NextChoices: regenChoices(nudges), NextLines: nudges}
	cfg := freshCfg()
	req := batchReq(engine.RegenerateSourceFresh, []version.Resolution{res}, false)
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, res version.Resolution, _ string, sinks engine.NotesSinks) (string, error) {
		return engine.RegenerateFreshBody(ctx, notesGit, tr, "", cfg, res, sinks)
	}
	req.ProduceRegenerator = func(_ engine.RegenerateSource, res version.Resolution, run engine.RegenRun) engine.Regenerator {
		return engine.RegenerateFreshRegenerator(notesGit, tr, "", cfg, res, run)
	}

	if _, err := engine.RegenerateAll(t.Context(), regenWriteDeps(rec, f), pub, t.TempDir(), req); err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}
	return rec, pub
}

func TestRegenerateAll_RegenClassifiedSize_HalvesWithNoHintOfItsOwnAndCountsTowardTheBatchEndHint(t *testing.T) {
	t.Parallel()

	tr := &regenOverflowTransport{nudge: exportNudge, overflows: singlePassCall, answer: "SIZE"}

	rec, pub := regenerateAllWithRegens(t, tr, exportNudge)

	tr.assertRegenHalved(t)
	if got, want := dispatchedBodies(pub), []string{regenMergeBody}; !slices.Equal(got, want) {
		t.Errorf("release bodies written = %q, want the r's merge output %q", got, want)
	}
	assertOneBatchEndHint(t, rec, len(tr.first[0]))
}

// longNudge makes an `r`'s single-pass call bigger than the first attempt's.
var longNudge = strings.Repeat("group the fixes by area and say what each changes, ", 20)

// largestCallIsAnRegens scripts a run whose first `r` succeeds in one call bigger than the
// first attempt's, then whose second `r` overflows and halves.
func largestCallIsAnRegens() *regenOverflowTransport {
	return &regenOverflowTransport{nudge: exportNudge, onePassNudges: []string{longNudge}, overflows: singlePassCall, answer: "SIZE"}
}

// assertOnePassRegenIsLargest fails unless the one-pass r's call is bigger than every
// other call carrying diff that succeeded.
func (rt *regenOverflowTransport) assertOnePassRegenIsLargest(t *testing.T) int {
	t.Helper()
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if len(rt.onePass) != 1 || len(rt.first) != 1 {
		t.Fatalf("one-pass r calls = %d, first attempt calls = %d, want one of each", len(rt.onePass), len(rt.first))
	}
	size := len(rt.onePass[0])
	if size <= len(rt.first[0]) || size <= largest(rt.chunks) {
		t.Fatalf("fixture: the one-pass r's call is not the run's largest succeeded call")
	}
	return size
}

func TestRelease_RegenWrittenAfterHalving_HintNamesAnEarlierRegensCallWhenItIsTheLargest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedTwoFileGenerations(f, 10_000, 3)
	tr := largestCallIsAnRegens()

	rec := releaseWithRegens(t, root, f, tr, newDeps, longNudge, exportNudge)

	tr.assertRegenHalved(t)
	assertHintAfterRegen(t, rec, tr.assertOnePassRegenIsLargest(t), presenter.ReleaseReviewGate())
}

func TestRegenerateAll_RegenWrittenAfterHalving_BatchEndHintNamesAnEarlierRegensCallWhenItIsTheLargest(t *testing.T) {
	t.Parallel()

	tr := largestCallIsAnRegens()

	rec, _ := regenerateAllWithRegens(t, tr, longNudge, exportNudge)

	tr.assertRegenHalved(t)
	assertOneBatchEndHint(t, rec, tr.assertOnePassRegenIsLargest(t))
}
