package runclose_test

import (
	"slices"
	"testing"

	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runclose"
)

var opened = presenter.RunInfo{
	Project: "acme",
	Version: "1.4.0",
	Leaf:    "🌱",
	Start:   presenter.Station{Name: "v1.3.2", Detail: "main @ 4f9c2e1"},
}

// closing returns the one RunFinished rec holds, failing the test unless exactly one
// was recorded and it was the last event.
func closing(t *testing.T, rec *presentertest.RecordingPresenter) presenter.RunResult {
	t.Helper()
	var results []presenter.RunResult
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindRunFinished {
			results = append(results, ev.RunFinished)
		}
	}
	if len(results) != 1 {
		t.Fatalf("RunFinished events = %d, want one; kinds = %v", len(results), rec.Kinds())
	}
	if last := rec.Events[len(rec.Events)-1]; last.Kind != presentertest.KindRunFinished {
		t.Fatalf("last event = %v, want the RunFinished", last.Kind)
	}
	return results[0]
}

func TestClose_ARunThatStoppedShortClosesAtItsStartStation(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		stop     func(*runclose.Run)
		declined bool
		want     presenter.RunResult
	}{
		{
			name: "a clean unwind is back where it started",
			stop: func(r *runclose.Run) { r.Unwound(true) },
			want: presenter.RunResult{Outcome: presenter.OutcomeUnwound, End: presenter.Station{Name: "v1.3.2", Detail: "back where it started · repo clean · nothing pushed"}},
		},
		{
			name: "an unwind that left steps undone needs manual cleanup",
			stop: func(r *runclose.Run) { r.Unwound(false) },
			want: presenter.RunResult{Outcome: presenter.OutcomeUnwound, End: presenter.Station{Name: "v1.3.2", Detail: "manual cleanup required · nothing pushed"}},
		},
		{
			name:     "an unwind outranks the decline that led to it",
			stop:     func(r *runclose.Run) { r.Unwound(true) },
			declined: true,
			want:     presenter.RunResult{Outcome: presenter.OutcomeUnwound, End: presenter.Station{Name: "v1.3.2", Detail: "back where it started · repo clean · nothing pushed"}},
		},
		{
			name:     "a decline changed nothing",
			stop:     func(*runclose.Run) {},
			declined: true,
			want:     presenter.RunResult{Outcome: presenter.OutcomeDeclined, End: presenter.Station{Name: "v1.3.2", Detail: "nothing changed"}},
		},
		{
			name: "a failure with nothing made changed nothing",
			stop: func(*runclose.Run) {},
			want: presenter.RunResult{Outcome: presenter.OutcomeFailed, End: presenter.Station{Name: "v1.3.2", Detail: "nothing changed"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := &presentertest.RecordingPresenter{}
			run := runclose.Begin(rec)
			run.Start(opened)
			tc.stop(run)
			run.Close(tc.declined)

			got := closing(t, rec)
			if got.Elapsed < 0 {
				t.Errorf("Elapsed = %v, want the run's non-negative duration", got.Elapsed)
			}
			got.Elapsed = 0
			want := tc.want
			want.Project, want.Version, want.Leaf = "acme", "1.4.0", "🌱"
			if got != want {
				t.Errorf("RunFinished = %+v, want %+v", got, want)
			}
		})
	}
}

func TestClose_ARunThatLandedClosesWithItsEndingWhateverFollowed(t *testing.T) {
	t.Parallel()

	landed := presenter.RunResult{
		Project: "acme",
		Outcome: presenter.OutcomeCommittedWithWarning,
		End:     presenter.Station{Name: "main", Detail: "a3f9c21"},
	}
	rec := &presentertest.RecordingPresenter{}
	run := runclose.Begin(rec)
	run.Start(opened)
	run.Land(landed)
	run.Close(false)

	got := closing(t, rec)
	got.Elapsed = 0
	if got != landed {
		t.Errorf("RunFinished = %+v, want the landed ending %+v", got, landed)
	}
}

func TestClose_ARunThatNeverOpenedClosesSilentlyButRunsItsDeferredWork(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	ran := false
	run := runclose.Begin(rec)
	run.Defer(func() { ran = true })
	run.Close(false)

	if len(rec.Events) != 0 {
		t.Errorf("events = %v, want none for a run that never opened", rec.Kinds())
	}
	if !ran {
		t.Error("deferred work did not run")
	}
}

func TestClose_DeferredWorkFollowsTheRunFinishedLastFirst(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	run := runclose.Begin(rec)
	var order []string
	run.Defer(func() { order = append(order, "first registered"); rec.Warn(presenter.Warning{Label: "first"}) })
	run.Defer(func() { order = append(order, "last registered") })
	run.Start(opened)
	run.Close(false)

	if want := []string{"last registered", "first registered"}; !slices.Equal(order, want) {
		t.Errorf("deferred order = %q, want %q", order, want)
	}
	want := []presentertest.EventKind{presentertest.KindRunStarted, presentertest.KindRunFinished, presentertest.KindWarn}
	if got := rec.Kinds(); !slices.Equal(got, want) {
		t.Errorf("kinds = %v, want %v (the deferred narration after the close)", got, want)
	}
}

func TestRun_ANilRunDoesNothing(t *testing.T) {
	t.Parallel()

	var run *runclose.Run
	run.Start(opened)
	run.Land(presenter.RunResult{})
	run.Unwound(true)
	run.Close(true)
}
