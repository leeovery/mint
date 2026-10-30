package presenter_test

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"mint/internal/presenter"
	"mint/internal/presenter/frametest"
)

// shellStopAndResume is what the shell prints under the settled lines when Ctrl-Z
// stops mint and fg resumes it, the cursor left on the row under it.
const shellStopAndResume = "^Z\r\n[1]+  Stopped                 mint release -m\r\n$ fg\r\nmint release -m\r\n"

// stoppedRail is a rail presenter on a terminal 80 by 40 whose stops the test sends,
// each carried out by stop.
type stoppedRail struct {
	*presenter.RailPresenter
	term  *frametest.Terminal
	out   *countingWriter
	ticks *fakeTicker
	stops chan os.Signal
}

func newStoppedRail(t *testing.T, stop func(r *stoppedRail)) *stoppedRail {
	t.Helper()
	r := &stoppedRail{term: frametest.NewTerminal(80, 40), ticks: &fakeTicker{}, stops: make(chan os.Signal)}
	r.out = &countingWriter{w: r.term}
	r.RailPresenter = presenter.NewRailPresenter(r.out,
		presenter.WithRailInput(answers()),
		presenter.WithRailSize(func() (int, int) { return 80, 40 }),
		presenter.WithRailTicker(r.ticks.start),
		presenter.WithRailStops(r.stops, func() { stop(r) }),
	)
	t.Cleanup(func() { close(r.stops) })
	return r
}

// awaitDrawn waits until the presenter has written past the first n bytes and its
// spinner runs again: the region drawn again once the process is resumed.
func (r *stoppedRail) awaitDrawn(t *testing.T, n int) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); len(r.out.written()) <= n || !r.ticks.running(); {
		if time.Now().After(deadline) {
			t.Fatalf("nothing was drawn once resumed; written %q", r.out.written())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// whileStopped is what a stop saw while the process was stopped: what was written
// when it stopped, whether the spinner ran then, and what was written after the
// shell's lines and a spinner frame fired after them.
type whileStopped struct {
	atStop, after string
	spinning      bool
}

func notesRunning(s string) bool { return strings.Contains(s, "  notes         writing the notes…") }

// TestRail_StoppedByTheShellItTakesTheRegionOffAndDrawsItAfreshAfterFg pins Ctrl-Z
// then fg mid-stage. The region comes off the terminal and the spinner stops before
// the process stops, so the shell prints under settled lines alone; nothing draws
// while it is stopped, not even a spinner frame due then, which Go's runtime fires the
// moment fg resumes it; then the region is drawn afresh under the shell's lines, from
// where the cursor is, and the spinner runs again.
func TestRail_StoppedByTheShellItTakesTheRegionOffAndDrawsItAfreshAfterFg(t *testing.T) {
	t.Parallel()

	seen := make(chan whileStopped, 1)
	r := newStoppedRail(t, func(r *stoppedRail) {
		s := whileStopped{atStop: r.out.written(), spinning: r.ticks.running()}
		if _, err := r.term.Write([]byte(shellStopAndResume)); err != nil {
			t.Error(err)
		}
		// The fake's channel outlives its stop, so a spinner goroutine not yet gone
		// may still take the frame; whether it draws is what counts.
		r.ticks.fire(50 * time.Millisecond)
		time.Sleep(20 * time.Millisecond)
		s.after = r.out.written()[len(s.atStop):]
		seen <- s
	})
	r.RunStarted(releaseStarted(false))
	r.ShowPlan(releasePlan())
	r.StageStarted(notesStarted())

	r.stops <- os.Interrupt
	s := <-seen
	atStop := s.atStop
	r.awaitDrawn(t, len(atStop))

	if !strings.HasSuffix(atStop, "\x1b[J") {
		t.Errorf("before the process stopped the writer ended %q, want the region erased", atStop[max(len(atStop)-40, 0):])
	}
	if s.spinning || s.after != "" {
		t.Errorf("while stopped: spinner running %v, written %q; want nothing drawn", s.spinning, s.after)
	}
	if again := r.out.written()[len(atStop):]; cursorUp.MatchString(again) {
		t.Errorf("resumed, the writer wrote %q, want the region drawn where the cursor is, nothing moved up", again)
	}
	r.ticks.tick(t)
	screen := r.term.Grid().Text()
	stopped := slices.Index(screen, "[1]+  Stopped                 mint release -m")
	fg := slices.Index(screen, "$ fg")
	if stopped < 0 || fg != stopped+1 {
		t.Fatalf("the terminal shows\n%s\nwant the shell's Stopped and fg lines kept", strings.Join(screen, "\n"))
	}
	if n := len(slices.DeleteFunc(slices.Clone(screen), func(s string) bool { return !notesRunning(s) })); n != 1 || !slices.ContainsFunc(screen[fg:], notesRunning) {
		t.Errorf("the notes run %d times, want once, under the shell's lines:\n%s", n, strings.Join(screen, "\n"))
	}
}

// TestRail_StoppedWithNoRegionDrawnItOnlyStops pins a stop while the editor holds the
// terminal, or once the run is closed: there is no region to take off, so nothing is
// written, and the process stops all the same.
func TestRail_StoppedWithNoRegionDrawnItOnlyStops(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		hold func(*stoppedRail)
	}{
		{name: "the editor holding the terminal", hold: func(r *stoppedRail) { r.SuspendSpinner() }},
		{name: "the run closed", hold: func(r *stoppedRail) { r.Close() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stopped := make(chan string, 1)
			var before string
			r := newStoppedRail(t, func(r *stoppedRail) { stopped <- r.out.written() })
			r.RunStarted(releaseStarted(false))
			r.StageStarted(notesStarted())
			tc.hold(r)
			before = r.out.written()

			r.stops <- os.Interrupt
			select {
			case atStop := <-stopped:
				if atStop != before {
					t.Errorf("before the process stopped the writer wrote %q, want nothing", atStop[len(before):])
				}
			case <-time.After(5 * time.Second):
				t.Fatal("the process was never stopped")
			}
		})
	}
}
