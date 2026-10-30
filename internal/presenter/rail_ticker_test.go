package presenter_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"mint/internal/presenter"
)

// untaken is how long a tick is offered to a ticker that should be stopped: one
// stopped may still take it, but draws nothing.
const untaken = 20 * time.Millisecond

func messageRunning(p presenter.Presenter) {
	commitRun(p)
	p.StageStarted(presenter.StageStart{Name: "commit message", Key: "message", Blocking: true, Text: "writing…"})
}

func TestRailTicker_EachTickRedrawsTheSpinnerAndTheTimeFromTheClock(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	messageRunning(r)
	for _, step := range []struct {
		advance time.Duration
		want    string
	}{
		{advance: 0, want: row(" ⠋  message       writing…", "0s")},
		{advance: 160 * time.Millisecond, want: row(" ⠹  message       writing…", "0s")},
		{advance: time.Second, want: row(" ⠼  message       writing…", "1s")},
	} {
		r.clock.Advance(step.advance)
		r.ticks.tick(t)
		if got := r.screen(t)[4]; got != step.want {
			t.Errorf("at +%v the running row reads %q, want %q", step.advance, got, step.want)
		}
	}
}

func TestRailTicker_NothingWritesWhileTheEditorHasTheTerminal(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	messageRunning(r)
	if !r.ticks.running() {
		t.Fatal("no ticker runs while a stage runs")
	}
	r.SuspendSpinner()
	if r.ticks.running() {
		t.Error("the ticker runs with the region erased for the editor")
	}
	written := r.writes.count()
	r.clock.Advance(time.Second)
	r.ticks.fire(untaken)
	if r.writes.count() != written {
		t.Error("a tick was drawn while the editor had the terminal")
	}
	r.ResumeSpinner()
	if !r.ticks.running() {
		t.Error("the ticker didn't run again once the editor was done")
	}
	r.ticks.tick(t)
}

// TestRailTicker_AStageThatEndsWhileTheEditorHasTheTerminalRunsNoTickerOnResume pins
// that a resume brings back only a spinner that still has a stage to show: one that
// succeeded or failed while the region was erased runs none.
func TestRailTicker_AStageThatEndsWhileTheEditorHasTheTerminalRunsNoTickerOnResume(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		end  func(r *railTerm)
	}{
		{name: "succeeded", end: func(r *railTerm) {
			r.StageSucceeded(presenter.StageSuccess{Name: "commit message", Key: "message", Detail: "written", Blocking: true})
		}},
		{name: "failed", end: func(r *railTerm) {
			r.StageFailed(presenter.StageFailure{Name: "commit message", Key: "message", Message: "the AI command failed"})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRailTerm(t, 80, 24, answers())
			messageRunning(r)
			r.SuspendSpinner()
			tt.end(r)
			r.ResumeSpinner()
			if r.ticks.running() {
				t.Error("the ticker runs again for a stage that ended while the editor had the terminal")
			}
		})
	}
}

func TestRailTicker_NothingWritesWhileAQuestionWaits(t *testing.T) {
	t.Parallel()
	in := newBlockingInput()
	r := newRailTerm(t, 80, 24, in)
	messageRunning(r)
	answered := make(chan presenter.Choice)
	go func() {
		c, _ := r.Prompt(commitGate())
		answered <- c
	}()
	<-in.reading
	if r.ticks.running() {
		t.Error("the ticker runs while the question waits")
	}
	written := r.writes.count()
	r.clock.Advance(time.Second)
	r.ticks.fire(untaken)
	if r.writes.count() != written {
		t.Error("a tick was drawn while the question waited")
	}
	in.lines <- "y\n"
	if c := <-answered; c != presenter.ChoiceYes {
		t.Fatalf("Prompt = %q, want y", c)
	}
	if !r.ticks.running() {
		t.Error("the ticker didn't run again, the stage still running, once the question was answered")
	}
}

func TestRailTicker_TheEndStopsTheTickerForGood(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		end  func(r *railTerm)
	}{
		{name: "the run finished", end: func(r *railTerm) {
			r.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeFailed, End: presenter.Station{Name: "main", Detail: "nothing changed"}})
		}},
		{name: "the presenter closed", end: func(r *railTerm) { r.Close() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRailTerm(t, 80, 24, answers())
			messageRunning(r)
			tt.end(r)
			if r.ticks.running() {
				t.Error("the ticker runs once the run is over")
			}
			written := r.writes.count()
			r.clock.Advance(time.Second)
			r.ticks.fire(untaken)
			if r.writes.count() != written {
				t.Error("a tick was drawn once the run was over")
			}
			if screen := r.screen(t); strings.Contains(strings.Join(screen, "\n"), "message") {
				t.Errorf("the stage left running is still drawn:\n%s", strings.Join(screen, "\n"))
			}
		})
	}
}

// TestRailTicker_TicksBetweenEveryEventLeaveTheRunAsDrawn runs the tour with the
// spinner ticking as fast as it is taken throughout, through its suspends and its
// questions, under -race: the run ends as its golden frame has it.
func TestRailTicker_TicksBetweenEveryEventLeaveTheRunAsDrawn(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 100, 40, answers("y\n", "e\n", "y\n"))
	stop, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			select {
			case <-stop:
				return
			default:
				r.ticks.fire(time.Millisecond)
			}
		}
	}()
	tour(r, r.clock, func() {})
	close(stop)
	<-stopped

	golden, err := os.ReadFile("testdata/frames/done-100.txt")
	if err != nil {
		t.Fatal(err)
	}
	wantScreen(t, r, strings.Split(strings.TrimSuffix(string(golden), "\n"), "\n")...)
}
