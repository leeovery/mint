package engine_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

const preTagHookConfig = "[release.hooks]\npre_tag = \"build.sh\"\n"

// gateAbortTransport answers a release's first attempt and its `r`. A classify call
// gets classify; a call carrying nudge is the `r`'s and gets regen's answer; of the
// rest, a chunk call gets notes, a merge sixPartMergeBody and a single-pass call
// firstSinglePass.
type gateAbortTransport struct {
	nudge    string
	classify string
	regen    func(ctx context.Context, prompt string) (string, error)

	mu       sync.Mutex
	calls    int
	classifs int
}

func (gt *gateAbortTransport) Generate(ctx context.Context, prompt string) (string, error) {
	if isCheckPrompt(prompt) {
		return "none", nil
	}
	gt.mu.Lock()
	defer gt.mu.Unlock()
	gt.calls++
	switch {
	case isClassifyPrompt(prompt):
		gt.classifs++
		return gt.classify, nil
	case strings.HasPrefix(prompt, nudgedInstructions(gt.nudge)):
		return gt.regen(ctx, prompt)
	case !strings.Contains(prompt, "diff --git "):
		return sixPartMergeBody, nil
	case partFraming.MatchString(prompt):
		return "- notes for " + partFraming.FindString(prompt), nil
	default:
		return firstSinglePass, nil
	}
}

func failsWith(err error) func(context.Context, string) (string, error) {
	return func(context.Context, string) (string, error) { return "", err }
}

// seedPreTagArtifactCommit scripts the pre_tag hook dirtying the tree and mint's
// artifact commit.
func seedPreTagArtifactCommit(f *runner.FakeRunner) {
	f.SeedSequence("git",
		ScriptedOut(" M bundle.js\n"), // -C root status --porcelain (post-hook: dirty)
		ScriptedOut(""),               // -C root add -A
		ScriptedOut(""),               // -C root commit -m chore(release): pre-tag artifacts
	)
	f.Seed("sh", runner.Result{}, nil)
}

// gateAbortRelease is a prior-tag release whose pre_tag hook made the artifact commit,
// whose notes git calls seedNotes scripts, ready for the unwind's reset.
func gateAbortRelease(t *testing.T, config string, seedNotes func(*runner.FakeRunner)) (string, *runner.FakeRunner) {
	t.Helper()
	root := t.TempDir()
	writeConfig(t, root, config)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedPreTagArtifactCommit(f)
	seedNotes(f)
	f.SeedSequence("git", ScriptedOut("")) // unwind: reset --hard startingSHA
	return root, f
}

func singlePassNotesAndRegen(f *runner.FakeRunner) {
	seedNormalAINotes(f)
	seedNormalAINotes(f)
}

func regenOnce(nudge string) *presentertest.RecordingPresenter {
	return &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen},
		NextLines:   []string{nudge},
	}
}

// assertUnwoundToStart fails unless the run aborted non-zero, reset to the starting
// commit taking the one artifact commit with it, narrated that, and made no tag.
func assertUnwoundToStart(t *testing.T, rec *presentertest.RecordingPresenter, f *runner.FakeRunner, err error) {
	t.Helper()
	assertAbortNonZero(t, err)
	if !invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("the pre_tag artifact commit was not unwound; got %v", commandLines(f.Invocations()))
	}
	if got, want := unwoundText(t, rec), "reset 1 commit; repo clean"; got != want {
		t.Errorf("Unwound.Text() = %q, want %q", got, want)
	}
	assertNoTag(t, f)
	assertNoMutation(t, f)
	assertClosesUnwound(t, rec)
}

func TestRelease_FailedSinglePassRegen_ShowsTheCallsOutputAndUnwindsThePreTagCommit(t *testing.T) {
	t.Parallel()

	_, f := gateAbortRelease(t, preTagHookConfig, singlePassNotesAndRegen)
	rec := regenOnce(exportNudge)
	deps := attendedDeps(rec, f)
	tr := &gateAbortTransport{nudge: exportNudge, classify: "OTHER", regen: failsWith(&ai.GenerationError{Stdout: loginOutput, ExitCode: 1})}
	deps.Transport = tr

	err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	if tr.classifs != 1 {
		t.Fatalf("classify calls = %d, want the r's failure classified once", tr.classifs)
	}
	sf := onlyStageFailureEvent(t, rec)
	if sf.Message != "AI call failed" || sf.Output != loginOutput {
		t.Errorf("✗ line = %q with output %q, want %q with the r call's output %q verbatim", sf.Message, sf.Output, "AI call failed", loginOutput)
	}
	assertStageFailedThenUnwound(t, rec)
	assertUnwoundToStart(t, rec, f, err)
}

func TestRelease_FailedChunkedRegen_UnwindsThePreTagCommit(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		config    string
		seedNotes func(*runner.FakeRunner)
		classify  string
		regen     func(context.Context, string) (string, error)
	}{
		{
			name:      "re-merge on chunked notes fails",
			config:    partCapToml + "\n" + preTagHookConfig,
			seedNotes: seedSixPartNotes,
			classify:  "OTHER",
			regen:     failsWith(&ai.GenerationError{Stdout: loginOutput, ExitCode: 1}),
		},
		{
			name:      "an r the ladder turned into a chunked run fails",
			config:    preTagHookConfig,
			seedNotes: func(f *runner.FakeRunner) { seedTwoFileGenerations(f, 10_000, 2) },
			classify:  "SIZE",
			regen: func(_ context.Context, prompt string) (string, error) {
				if partFraming.MatchString(prompt) {
					return "", fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout)
				}
				return "", &ai.GenerationError{Stdout: regenOverflowOutput + "\n", ExitCode: 1}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, f := gateAbortRelease(t, tc.config, tc.seedNotes)
			rec := regenOnce(exportNudge)
			deps := attendedDeps(rec, f)
			deps.Transport = &gateAbortTransport{nudge: exportNudge, classify: tc.classify, regen: tc.regen}

			err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

			assertStageFailedThenUnwound(t, rec)
			assertUnwoundToStart(t, rec, f, err)
		})
	}
}

func TestRelease_StdinClosedAtTheRegenNudge_MakesNoCallAndUnwindsThePreTagCommit(t *testing.T) {
	t.Parallel()

	_, f := gateAbortRelease(t, preTagHookConfig, seedNormalAINotes)
	rec := &presentertest.RecordingPresenter{
		NextChoices:   []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen},
		AskLineResult: func(string) (string, error) { return "", presenter.ErrInputClosed },
	}
	deps := attendedDeps(rec, f)
	tr := &freshTransport{body: firstSinglePass}
	deps.Transport = tr

	err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	if !errors.Is(err, presenter.ErrInputClosed) {
		t.Errorf("err = %v, want the closed input", err)
	}
	if got := tr.calls(); got != 1 {
		t.Errorf("AI calls = %d, want only the first attempt's", got)
	}
	assertUnwoundToStart(t, rec, f, err)
}

func TestRelease_FailedEditorAtTheReviewGate_SurfacesAndUnwindsThePreTagCommit(t *testing.T) {
	t.Parallel()

	editorFailure := errors.New(`launching editor "vi": exit status 1`)
	_, f := gateAbortRelease(t, preTagHookConfig, seedNormalAINotes)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceEdit}}
	deps := attendedDeps(rec, f)
	deps.Transport = &freshTransport{body: firstSinglePass}
	deps.Editor = &fakeEditor{err: editorFailure}

	err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	if !errors.Is(err, editorFailure) {
		t.Errorf("err = %v, want the editor's failure", err)
	}
	if sf := onlyStageFailureEvent(t, rec); sf.Name != "edit" {
		t.Errorf("StageFailed name = %q, want the edit failure surfaced", sf.Name)
	}
	assertStageFailedThenUnwound(t, rec)
	assertUnwoundToStart(t, rec, f, err)
}

func TestRelease_StdinClosedAtTheReviewGateKeypress_UnwindsThePreTagCommit(t *testing.T) {
	t.Parallel()

	_, f := gateAbortRelease(t, preTagHookConfig, seedNormalAINotes)
	review := presenter.ReleaseReviewGate()
	rec := &presentertest.RecordingPresenter{PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
		if g.Question == review.Question {
			return "", presenter.ErrInputClosed
		}
		return g.Default, nil
	}}
	deps := attendedDeps(rec, f)
	deps.Transport = &freshTransport{body: firstSinglePass}

	err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	if !errors.Is(err, presenter.ErrInputClosed) {
		t.Errorf("err = %v, want the closed input", err)
	}
	assertUnwoundToStart(t, rec, f, err)
}

// The editor resolves from the process environment, so this test cannot run in
// parallel with others.
func TestRelease_MissingEditorAtTheReviewGate_WarnsAndAsksAgainWithNothingReset(t *testing.T) {
	t.Setenv("VISUAL", "mint-missing-editor")
	t.Setenv("EDITOR", "")

	root := t.TempDir()
	writeConfig(t, root, preTagHookConfig)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedPreTagArtifactCommit(f)
	seedNormalAINotes(f)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	f.SeedNotFound("mint-missing-editor")
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	deps := attendedDeps(rec, f)
	deps.Transport = &freshTransport{body: firstSinglePass}
	deps.Editor = engine.NewEditorLauncher(rec, f)

	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	want := []string{`could not launch editor "mint-missing-editor": not found`}
	if got := warnMessages(rec); !slices.Equal(got, want) {
		t.Errorf("warnings = %q, want %q", got, want)
	}
	if got := reviewAsks(rec, presenter.ReleaseReviewGate()); got != 2 {
		t.Errorf("review gate asks = %d, want the gate asked again after the missing editor", got)
	}
	if got := shownBodies(rec); !slices.Equal(got, []string{firstSinglePass}) {
		t.Errorf("notes shown = %q, want only the first attempt's", got)
	}
	if got := tagAnnotationBody(t, f, nextTag); !strings.Contains(got, firstSinglePass) {
		t.Errorf("tag annotation = %q, want the unchanged body %q", got, firstSinglePass)
	}
	if invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("a missing editor reset the run; got %v", commandLines(f.Invocations()))
	}
	if indexOfKind(rec, presentertest.KindUnwound) != -1 {
		t.Errorf("a missing editor narrated an unwind; kinds = %v", rec.Kinds())
	}
}

func TestRelease_CtrlCDuringAnRegensCall_AbortsAsACancelAndUnwindsThePreTagCommit(t *testing.T) {
	t.Parallel()

	_, f := gateAbortRelease(t, preTagHookConfig, singlePassNotesAndRegen)
	rec := regenOnce(exportNudge)
	deps := attendedDeps(rec, f)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	tr := &gateAbortTransport{nudge: exportNudge, classify: "OTHER", regen: func(ctx context.Context, _ string) (string, error) {
		cancel()
		return "", ctx.Err()
	}}
	deps.Transport = tr

	err := engine.Release(ctx, deps, priorTagNormalAIOptions())

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want a cancel", err)
	}
	if tr.classifs != 0 {
		t.Errorf("classify calls = %d, want a cancel never classified", tr.classifs)
	}
	assertUnwoundToStart(t, rec, f, err)
}

func TestRelease_FailedRegenUnderFallback_UnwindsWithoutTheFallbackOrTheFloor(t *testing.T) {
	t.Parallel()

	config := "[release]\non_notes_failure = \"fallback\"\n\n" + preTagHookConfig
	_, f := gateAbortRelease(t, config, singlePassNotesAndRegen)
	rec := regenOnce(exportNudge)
	deps := attendedDeps(rec, f)
	deps.Transport = &gateAbortTransport{nudge: exportNudge, classify: "OTHER", regen: failsWith(&ai.GenerationError{Stdout: loginOutput, ExitCode: 1})}

	err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	assertUnwoundToStart(t, rec, f, err)
	if countCmd(f, "git", "log", "--format=%s", priorTag+"..HEAD") != 0 {
		t.Errorf("a failed r built the commit-subject list")
	}
	if got := warnMessages(rec); len(got) != 0 {
		t.Errorf("warnings = %q, want no fallback warning", got)
	}
	if gates := floorGates(rec); len(gates) != 0 {
		t.Errorf("a failed r was offered the c / e / a choice: %+v", gates)
	}
}
