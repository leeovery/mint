package engine_test

import (
	"testing"

	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

const (
	answeredRangeWant = "fix: handle empty input\nfeat: add export\n"
	majorTagV200      = "v2.0.0"
)

// answeredFailedNotes is an attended release of v1.3.0 under config that recorded an
// Exclude answer, whose notes call then fails and whose commit subjects since the last
// tag are log, its floor answered with c.
func answeredFailedNotes(t *testing.T, config, log string) *answeredAbort {
	t.Helper()
	rec := &presentertest.RecordingPresenter{PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
		switch {
		case isPauseGate(g):
			return presenter.ChoiceExclude, nil
		case g.Has(presenter.ChoiceCommitSubjects):
			return presenter.ChoiceCommitSubjects, nil
		default:
			return g.Default, nil
		}
	}}
	a := newAnsweredAbort(t, rec, config)
	a.deps.Transport = &answeredTransport{notes: notesFailing}
	a.f.SeedSequence("git", ScriptedOut(log)) // log --format=%s v1.2.3..HEAD
	seedRecordTagPush(a.f, a.root)
	a.f.Seed("gh", runner.Result{}, nil)
	return a
}

func TestRelease_CommitSubjectsAfterAnswering_LeaveOutTheAnswersCommits(t *testing.T) {
	t.Parallel()

	for _, route := range []struct {
		name   string
		config string
	}{
		{name: "on_notes_failure fallback", config: "[release]\non_notes_failure = \"fallback\"\n"},
		{name: "c at the floor", config: "[release]\non_notes_failure = \"abort\"\n"},
	} {
		for _, tc := range []struct {
			name string
			log  string
		}{
			{name: "this run's answers commit", log: answersSubject(minorTagV130) + "\n" + answeredRangeWant},
			{
				name: "a kept answers commit and this run's",
				log:  answersSubject(minorTagV130) + "\nfix: handle empty input\n" + answersSubject(minorTagV130) + "\nfeat: add export\n",
			},
		} {
			t.Run(route.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				a := answeredFailedNotes(t, route.config, tc.log)

				if err := a.release(t.Context()); err != nil {
					t.Fatalf("Release returned unexpected error: %v", err)
				}

				if got := tagAnnotationBody(t, a.f, minorTagV130); got != answeredRangeWant {
					t.Errorf("tag annotation body = %q, want %q", got, answeredRangeWant)
				}
			})
		}
	}
}

func TestRelease_NotesFloor_OnlyAnswersCommitsGiveTheMaintenanceBody(t *testing.T) {
	t.Parallel()

	a := answeredFailedNotes(t, "", answersSubject(minorTagV130)+"\n")

	if err := a.release(t.Context()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got := tagAnnotationBody(t, a.f, minorTagV130); got != terminatedAnnotationBody(notes.StubBody()) {
		t.Errorf("tag annotation body = %q, want the maintenance-release body %q", got, notes.StubBody())
	}
}

func TestRelease_NoAI_LeavesOutAnAnswersCommitAnAbortedAttemptKept(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		log  string
		want string
	}{
		{
			name: "other subjects in the range",
			log:  answersSubject(minorTagV130) + "\n" + answeredRangeWant,
			want: answeredRangeWant,
		},
		{
			name: "nothing else in the range",
			log:  answersSubject(minorTagV130) + "\n",
			want: terminatedAnnotationBody(notes.StubBody()),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			seedPriorTagReadGates(f, root, "main")
			f.SeedSequence("git",
				ScriptedOut(priorTagDiff), // diff priorTag..HEAD
				ScriptedOut(tc.log),       // log --format=%s priorTag..HEAD
			)
			seedRecordTagPush(f, root)
			f.Seed("gh", runner.Result{}, nil)
			opts := engine.ReleaseOptions{Bump: version.BumpMajor, NoAI: true, Now: fixedClock}

			if err := engine.Release(t.Context(), newDeps(&presentertest.RecordingPresenter{}, f), opts); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			if got := tagAnnotationBody(t, f, majorTagV200); got != tc.want {
				t.Errorf("tag annotation body = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRegenerateRun_NotesFloor_CommitSubjectsLeaveOutTheAnswersCommit(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		log  string
		want string
	}{
		{
			name: "the artifact commit and a change in the range",
			log: "🌿 Release " + regenFloorTag + "\n" + answersSubject(regenFloorTag) + "\n" +
				pretagArtifactSubject(regenFloorTag) + "\nfeat: add export\n",
			want: pretagArtifactSubject(regenFloorTag) + "\nfeat: add export\n",
		},
		{
			name: "nothing but the bookkeeping and answers commits",
			log:  "🌿 Release " + regenFloorTag + "\n" + answersSubject(regenFloorTag) + "\n",
			want: notes.StubBody(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := newAttendedRegenFloor(t, &fakeEditor{})
			run.deps.Runner.(*runner.FakeRunner).SeedSequence("git", ScriptedOut(tc.log))
			run.rec.PromptResult = answerFloor(presenter.ChoiceCommitSubjects)

			if err := run.regenerateAttended(t.Context(), singlePassFailsUnder("")); err != nil {
				t.Fatalf("RegenerateRun returned unexpected error: %v", err)
			}

			assertOnlyBodyWritten(t, run, tc.want)
		})
	}
}
