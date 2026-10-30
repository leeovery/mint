package presenter_test

import (
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"mint/internal/presenter"
)

// shellStopAndResume is what the shell prints under the region when Ctrl-Z stops
// mint and fg resumes it, the cursor left on the row under it.
const shellStopAndResume = "^Z\r\n[1]+  Stopped                 mint release -m\r\n$ fg\r\nmint release -m\r\n"

// TestRail_ResumedByTheShellItDrawsAfreshBelowWhatTheShellPrinted pins that once the
// shell resumes mint, Ctrl-Z then fg, the region is drawn again under the shell's
// lines, from where the cursor is, never going back up over them: the shell's
// Stopped and fg lines stay, and the next spinner frame redraws only the new region.
func TestRail_ResumedByTheShellItDrawsAfreshBelowWhatTheShellPrinted(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 40, answers())
	r.RunStarted(releaseStarted(false))
	r.ShowPlan(releasePlan())
	r.StageStarted(notesStarted())
	if _, err := r.term.Write([]byte(shellStopAndResume)); err != nil {
		t.Fatal(err)
	}
	before := len(r.writes.written())

	r.Continued()
	if drawn := r.writes.written()[before:]; cursorUp.MatchString(drawn) || !strings.Contains(drawn, " ⠋  notes") {
		t.Errorf("resumed, the writer wrote %q, want the region drawn where the cursor is, nothing moved up", drawn)
	}
	r.ticks.tick(t)

	screen := r.screen(t)
	stopped := slices.Index(screen, "[1]+  Stopped                 mint release -m")
	fg := slices.Index(screen, "$ fg")
	if stopped < 0 || fg != stopped+1 {
		t.Fatalf("the terminal shows\n%s\nwant the shell's Stopped and fg lines kept", strings.Join(screen, "\n"))
	}
	if n := len(slices.DeleteFunc(slices.Clone(screen[fg:]), func(s string) bool { return !strings.HasPrefix(s, " ⠋  notes") })); n != 1 {
		t.Errorf("under the shell's lines the notes run %d times, want once:\n%s", n, strings.Join(screen, "\n"))
	}
}

// TestRail_ResumedWhileTheEditorHasTheTerminalItDrawsNothing pins that a resume
// while the editor holds the terminal draws nothing over it: the region comes back
// once the editor is done.
func TestRail_ResumedWhileTheEditorHasTheTerminalItDrawsNothing(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 40, answers())
	r.RunStarted(releaseStarted(false))
	r.StageStarted(notesStarted())
	r.SuspendSpinner()
	before := len(r.writes.written())

	r.Continued()

	if after := r.writes.written()[before:]; after != "" {
		t.Errorf("resumed under the editor, the writer wrote %q, want nothing", after)
	}
	r.ResumeSpinner()
	if screen := r.screen(t); !slices.ContainsFunc(screen, func(s string) bool { return strings.HasPrefix(s, " ⠋  notes") }) {
		t.Errorf("the editor done, the terminal shows\n%s\nwant the notes running", strings.Join(screen, "\n"))
	}
}

// TestRail_WatchesTheResumeSignalUntilClosed pins the seam production wires SIGCONT
// through: each signal draws the run afresh, and Close stops the signal, once.
func TestRail_WatchesTheResumeSignalUntilClosed(t *testing.T) {
	t.Parallel()
	signals := make(chan os.Signal)
	var stops atomic.Int32
	out := &syncedBuffer{}
	p := presenter.NewRailPresenter(out,
		presenter.WithRailSize(func() (int, int) { return 80, 24 }),
		presenter.WithRailTicker((&fakeTicker{}).start),
		presenter.WithRailContinued(signals, func() { stops.Add(1) }),
	)
	p.RunStarted(releaseStarted(false))
	p.StageStarted(notesStarted())
	drawn := len(out.String())

	signals <- os.Interrupt
	for deadline := time.Now().Add(5 * time.Second); len(out.String()) == drawn; {
		if time.Now().After(deadline) {
			t.Fatal("a resume signal drew nothing")
		}
		time.Sleep(5 * time.Millisecond)
	}
	p.Close()
	p.Close()

	if n := stops.Load(); n != 1 {
		t.Errorf("the signal was stopped %d times, want once", n)
	}
	select {
	case signals <- os.Interrupt:
		t.Error("a signal was taken once the Rail was closed")
	case <-time.After(50 * time.Millisecond):
	}
}
