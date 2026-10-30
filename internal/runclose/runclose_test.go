package runclose_test

import (
	"slices"
	"testing"
	"time"

	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runclose"
)

var opened = presenter.RunInfo{
	Project: "acme",
	Version: "1.4.0",
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

func TestClose_ARunThatStoppedShortClosesAtItsStartStationWithWhatItLeft(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		stop     func(*runclose.Run)
		declined bool
		want     presenter.RunResult
	}{
		{
			name: "an unwind closes with where it left the repo",
			stop: func(r *runclose.Run) { r.Unwound("back where it started · repo clean · nothing pushed") },
			want: presenter.RunResult{Outcome: presenter.OutcomeUnwound, End: presenter.Station{Name: "v1.3.2", Detail: "back where it started · repo clean · nothing pushed"}},
		},
		{
			name:     "an unwind outranks the decline that led to it",
			stop:     func(r *runclose.Run) { r.Unwound("manual cleanup required · nothing pushed") },
			declined: true,
			want:     presenter.RunResult{Outcome: presenter.OutcomeUnwound, End: presenter.Station{Name: "v1.3.2", Detail: "manual cleanup required · nothing pushed"}},
		},
		{
			name: "a reset that never moved HEAD closes where HEAD stands",
			stop: func(r *runclose.Run) {
				r.StoppedAt("main")
				r.Unwound("9f3e2a1 · 2 commits not reset · manual cleanup required · nothing pushed")
			},
			want: presenter.RunResult{Outcome: presenter.OutcomeUnwound, End: presenter.Station{Name: "main", Detail: "9f3e2a1 · 2 commits not reset · manual cleanup required · nothing pushed"}},
		},
		{
			name: "a station git couldn't name keeps the start",
			stop: func(r *runclose.Run) {
				r.StoppedAt("")
				r.Unwound("2 commits not reset · manual cleanup required · nothing pushed")
			},
			want: presenter.RunResult{Outcome: presenter.OutcomeUnwound, End: presenter.Station{Name: "v1.3.2", Detail: "2 commits not reset · manual cleanup required · nothing pushed"}},
		},
		{
			name:     "a decline that left nothing changed nothing",
			stop:     func(*runclose.Run) {},
			declined: true,
			want:     presenter.RunResult{Outcome: presenter.OutcomeDeclined, End: presenter.Station{Name: "v1.3.2", Detail: "nothing changed"}},
		},
		{
			name: "a failure that left nothing changed nothing",
			stop: func(*runclose.Run) {},
			want: presenter.RunResult{Outcome: presenter.OutcomeFailed, End: presenter.Station{Name: "v1.3.2", Detail: "nothing changed"}},
		},
		{
			name: "a failure that left something says what",
			stop: func(r *runclose.Run) { r.Left("changes staged, not committed") },
			want: presenter.RunResult{Outcome: presenter.OutcomeFailed, End: presenter.Station{Name: "v1.3.2", Detail: "changes staged, not committed"}},
		},
		{
			name: "what a run wrote comes first, the latest written standing",
			stop: func(r *runclose.Run) {
				r.Wrote("1 GitHub release written")
				r.Left("nothing committed · nothing pushed")
				r.Wrote("2 GitHub releases written")
			},
			want: presenter.RunResult{Outcome: presenter.OutcomeFailed, End: presenter.Station{Name: "v1.3.2", Detail: "2 GitHub releases written · nothing committed · nothing pushed"}},
		},
		{
			name: "what a run wrote alone is all it says",
			stop: func(r *runclose.Run) { r.Wrote("1 GitHub release written") },
			want: presenter.RunResult{Outcome: presenter.OutcomeFailed, End: presenter.Station{Name: "v1.3.2", Detail: "1 GitHub release written"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := &presentertest.RecordingPresenter{}
			run := runclose.Begin(rec, time.Now)
			run.Start(opened)
			tc.stop(run)
			run.Close(tc.declined)

			got := closing(t, rec)
			if got.Elapsed < 0 {
				t.Errorf("Elapsed = %v, want the run's non-negative duration", got.Elapsed)
			}
			got.Elapsed = 0
			want := tc.want
			want.Project, want.Version = "acme", "1.4.0"
			if got != want {
				t.Errorf("RunFinished = %+v, want %+v", got, want)
			}
		})
	}
}

func TestClose_ARunThatLandedClosesWithItsEndingWhateverItLeftOnTheWay(t *testing.T) {
	t.Parallel()

	landed := presenter.RunResult{
		Project: "acme",
		Outcome: presenter.OutcomeCommittedWithWarning,
		End:     presenter.Station{Name: "main", Detail: "a3f9c21"},
	}
	rec := &presentertest.RecordingPresenter{}
	run := runclose.Begin(rec, time.Now)
	run.Start(opened)
	run.Left("changes staged, not committed")
	run.Land(landed)
	run.Close(false)

	got := closing(t, rec)
	got.Elapsed = 0
	if got != landed {
		t.Errorf("RunFinished = %+v, want the landed ending %+v", got, landed)
	}
}

func TestClose_TimesTheRunByTheClockItBeganWith(t *testing.T) {
	t.Parallel()

	reads := []time.Time{time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC), time.Date(2026, 10, 29, 12, 1, 14, 0, time.UTC)}
	clock := func() time.Time {
		now := reads[0]
		reads = reads[1:]
		return now
	}
	rec := &presentertest.RecordingPresenter{}
	run := runclose.Begin(rec, clock)
	run.Start(opened)
	run.Close(false)

	if got := closing(t, rec).Elapsed; got != 74*time.Second {
		t.Errorf("Elapsed = %v, want the 1m 14s between the clock's reads at Begin and Close", got)
	}
}

func TestOpen_OpensWithTheHeaderAloneAndStartDrawsTheStationLater(t *testing.T) {
	t.Parallel()

	header := presenter.Header{Command: "release regenerate", Where: "~/Code/pigeon"}
	rec := &presentertest.RecordingPresenter{}
	run := runclose.Begin(rec, time.Now)
	run.Open(presenter.RunInfo{Project: "acme", Header: header, Start: opened.Start})

	if got, want := rec.Events[0].RunStarted, (presenter.RunInfo{Project: "acme", Header: header, Quiet: true}); len(rec.Events) != 1 || got != want {
		t.Fatalf("opening = %+v, want the one Quiet start %+v, carrying the header alone", rec.Events, want)
	}
	run.Start(opened)
	run.Close(false)

	if got := closing(t, rec).End; got != (presenter.Station{Name: "v1.3.2", Detail: "nothing changed"}) {
		t.Errorf("close = %+v, want at the station Start drew", got)
	}
}

func TestOpen_ARunThatClosesBeforeItsStationClosesWithNone(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	run := runclose.Begin(rec, time.Now)
	run.Open(presenter.RunInfo{Project: "acme", Header: presenter.Header{Command: "release regenerate"}})
	run.Close(true)

	if got := closing(t, rec); got.Outcome != presenter.OutcomeDeclined || got.End.Name != "" {
		t.Errorf("close = %+v, want declined with no end station", got)
	}
}

func TestClose_ARunThatNeverOpenedClosesSilently(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	run := runclose.Begin(rec, time.Now)
	run.Close(false)

	if len(rec.Events) != 0 {
		t.Errorf("events = %v, want none for a run that never opened", rec.Kinds())
	}
}

func TestRunDeferred_RunsTheDeferredWorkLastFirstAndOnlyOnce(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	run := runclose.Begin(rec, time.Now)
	var order []string
	run.Defer(func() { order = append(order, "first registered"); rec.Warn(presenter.Warning{Label: "first"}) })
	run.Defer(func() { order = append(order, "last registered") })
	run.Start(opened)
	run.Close(false)
	run.RunDeferred()
	run.RunDeferred()

	if want := []string{"last registered", "first registered"}; !slices.Equal(order, want) {
		t.Errorf("deferred order = %q, want %q, each once", order, want)
	}
	want := []presentertest.EventKind{presentertest.KindRunStarted, presentertest.KindRunFinished, presentertest.KindWarn}
	if got := rec.Kinds(); !slices.Equal(got, want) {
		t.Errorf("kinds = %v, want %v (the deferred narration after the close)", got, want)
	}
}

// TestRunDeferred_StillRunsWhenTheRunPanicsBeforeItsClose pins the reason an
// orchestrator defers RunDeferred: a panic that never reaches Close still runs the
// deferred work, and closes nothing.
func TestRunDeferred_StillRunsWhenTheRunPanicsBeforeItsClose(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	ran := false
	func() {
		defer func() { _ = recover() }()
		run := runclose.Begin(rec, time.Now)
		defer run.RunDeferred()
		run.Start(opened)
		run.Defer(func() { ran = true })
		panic("the run panics")
	}()

	if !ran {
		t.Error("deferred work did not run on the panic")
	}
	if want := []presentertest.EventKind{presentertest.KindRunStarted}; !slices.Equal(rec.Kinds(), want) {
		t.Errorf("kinds = %v, want %v (no close on a panic)", rec.Kinds(), want)
	}
}

func TestRun_ANilRunDoesNothing(t *testing.T) {
	t.Parallel()

	var run *runclose.Run
	run.Start(opened)
	run.Land(presenter.RunResult{})
	run.Unwound("back where it started")
	run.Left("changes staged, not committed")
	run.Open(opened)
	run.Wrote("1 GitHub release written")
	run.RunDeferred()
	run.Close(true)
}
