package engine_test

import (
	"errors"
	"testing"
	"time"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// closedAt answers every question with its default but the one asks matches, at
// which the input closes, as Ctrl-D or Ctrl-C leaves it.
func closedAt(asks func(presenter.Gate) bool) *presentertest.RecordingPresenter {
	return &presentertest.RecordingPresenter{PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
		if asks(g) {
			return "", presenter.ErrInputClosed
		}
		return g.Default, nil
	}}
}

func asking(subject string) func(presenter.Gate) bool {
	return func(g presenter.Gate) bool { return g.Subject == subject }
}

func reviewing(g presenter.Gate) bool { return g.Review }

func reusing(g presenter.Gate) bool { return g.Subject == "notes" && !g.Review }

// inputClosed is the failure a question whose input closed leaves, labelled name on
// the node keyed key: the sentinel in its own words, as commit's review says it.
func inputClosed(name, key string) presenter.StageFailure {
	return presenter.StageFailure{Name: name, Key: key, Message: presenter.ErrInputClosed.Error()}
}

// assertClosedOn fails unless err is an abort carrying the closed input and the
// run's last StageFailed, of failures in all, is want, however long its stage ran.
func assertClosedOn(t *testing.T, rec *presentertest.RecordingPresenter, err error, failures int, want presenter.StageFailure) {
	t.Helper()
	assertAbortNonZero(t, err)
	if !errors.Is(err, presenter.ErrInputClosed) {
		t.Errorf("err = %v, want the closed input", err)
	}
	var got []presenter.StageFailure
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageFailed {
			got = append(got, ev.StageFailed)
		}
	}
	if len(got) != failures {
		t.Fatalf("StageFailed events = %+v, want %d", got, failures)
	}
	if last := untimed(got[len(got)-1]); last != want {
		t.Errorf("StageFailed = %+v, want %+v", last, want)
	}
}

func TestRelease_InputClosedAtAQuestion_FailsTheNodeItAsksAbout(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		rec  *presentertest.RecordingPresenter
		want presenter.StageFailure
	}{
		{name: "the version", rec: closedAt(asking("version")), want: inputClosed("version", "version")},
		{name: "the review", rec: closedAt(reviewing), want: inputClosed("review", "notes")},
		{name: "r's context line", rec: &presentertest.RecordingPresenter{
			NextChoices:   []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen},
			AskLineResult: func(string) (string, error) { return "", presenter.ErrInputClosed },
		}, want: inputClosed("regenerate", "notes")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := runner.NewFakeRunner()
			seedHappyGit(f, t.TempDir(), "main", "v0.0.1")
			f.Seed("gh", runner.Result{}, nil)

			err := engine.Release(t.Context(), newDepsWithRegenerator(tc.rec, f, &fakeRegenerator{bodies: []string{regen1Body}}), normalAIOptions(phase2Body))

			assertClosedOn(t, tc.rec, err, 1, tc.want)
			assertNoMutation(t, f)
		})
	}
}

func TestRelease_InputClosedAtTheCachedNotesQuestion_FailsTheNotes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cacheBase := t.TempDir()
	seedCacheEntry(t, cacheBase, root, priorTagDiff, "1.2.4", cachedReuseBody)
	f := runner.NewFakeRunner()
	seedRealRunReuseGit(f, root, priorTagDiff)
	rec := closedAt(reusing)

	err := engine.Release(t.Context(), newDepsWithRealRunCache(rec, f, cacheBase, func() time.Time { return realRunClock }), priorTagNormalAIOptions())

	assertClosedOn(t, rec, err, 1, inputClosed("notes", "notes"))
	assertNoTag(t, f)
}

func TestRelease_InputClosedAtTheNotesFloor_FailsTheNotesAgain(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")
	f := runner.NewFakeRunner()
	seedPreTagArtifactThenNotes(f, root)
	f.Seed("sh", runner.Result{}, nil)
	rec := closedAt(func(g presenter.Gate) bool { return g.Has(presenter.ChoiceCommitSubjects) })
	deps := attendedDeps(rec, f)
	deps.Transport = &freshTransport{err: &ai.GenerationError{Stdout: loginOutput, ExitCode: 1}}

	err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	// The first failure is the notes call's, the one the floor offers a way past.
	assertClosedOn(t, rec, err, 2, inputClosed("notes", "notes"))
	assertNoTag(t, f)
}

// The check has settled what it flagged before its pauses ask, so a pause whose
// input closes fails the notes, the stage the run is in, as every failure the
// selector returns does.
func TestRelease_InputClosedAtAnExclusionPause_FailsTheNotes(t *testing.T) {
	t.Parallel()

	_, f := gateAbortRelease(t, preTagHookConfig, seedNormalAINotes)
	f.SeedSequence("claude", ScriptedOut("auth/: auth code\n"), ScriptedOut(aiBody))
	rec := answeringPauses(func() (presenter.Choice, error) { return "", presenter.ErrInputClosed })

	err := engine.Release(t.Context(), attendedDeps(rec, f), priorTagNormalAIOptions())

	assertClosedOn(t, rec, err, 1, inputClosed("notes", "notes"))
}

func TestResolveRegenerateAxes_InputClosedAtAQuestion_FailsWhatItAsks(t *testing.T) {
	t.Parallel()

	for _, subject := range []string{"source", "target"} {
		t.Run(subject, func(t *testing.T) {
			t.Parallel()

			rec := closedAt(asking(subject))

			_, _, err := engine.ResolveRegenerateAxes(rec, engine.SourceUnset(), engine.TargetUnset(), true)

			assertClosedOn(t, rec, err, 1, inputClosed(subject, ""))
		})
	}
}

func TestRegenerateWrite_InputClosedAtAQuestion_FailsTheReview(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		req  engine.RegenerateWriteRequest
		rec  *presentertest.RecordingPresenter
		want presenter.StageFailure
	}{
		{name: "the review", req: freshWriteReq(engine.RegenerateTargetRelease), rec: closedAt(reviewing), want: inputClosed("review", "review")},
		{name: "the reused notes", req: reuseWriteReq(engine.RegenerateTargetRelease), rec: closedAt(reusing), want: inputClosed("notes", "review")},
		{name: "r's context line", req: freshWriteReq(engine.RegenerateTargetRelease), rec: &presentertest.RecordingPresenter{
			NextChoices:   []presenter.Choice{presenter.ChoiceRegen},
			AskLineResult: func(string) (string, error) { return "", presenter.ErrInputClosed },
		}, want: inputClosed("regenerate", "")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := runner.NewFakeRunner()
			f.Seed("git", runner.Result{}, nil)
			pub := newFakePublisher()
			pub.seedExists(regenWriteTag, true, nil)
			tc.req.Regenerator = &fakeRegenerator{bodies: []string{regen1Body}}

			err := engine.RegenerateWrite(t.Context(), regenWriteDeps(tc.rec, f), pub, t.TempDir(), tc.req)

			assertClosedOn(t, tc.rec, err, 1, tc.want)
			if len(pub.dispatched) != 0 {
				t.Errorf("dispatched %+v after the input closed", pub.dispatched)
			}
		})
	}
}

func TestRegenerateAll_InputClosedAtAVersionsQuestion_FailsThatVersion(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		source engine.RegenerateSource
		rec    *presentertest.RecordingPresenter
		want   presenter.StageFailure
	}{
		{name: "its review", source: engine.RegenerateSourceFresh, rec: closedAt(reviewing), want: inputClosed("review", batchV1Tag)},
		{name: "its reused notes", source: engine.RegenerateSourceTag, rec: closedAt(reusing), want: inputClosed("notes", batchV1Tag)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := runner.NewFakeRunner()
			seedReuseGit(f)
			pub := newFakePublisher()
			for _, tag := range []string{batchV1Tag, batchV2Tag, batchV3Tag} {
				pub.seedExists(tag, true, nil)
			}

			_, err := engine.RegenerateAll(t.Context(), batchDeps(tc.rec, f), pub, t.TempDir(), batchReq(tc.source, threeVersions(), false))

			assertClosedOn(t, tc.rec, err, 1, tc.want)
			if len(pub.dispatched) != 0 {
				t.Errorf("dispatched %+v after the input closed", pub.dispatched)
			}
		})
	}
}
