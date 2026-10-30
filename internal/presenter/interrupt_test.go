package presenter_test

import (
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
	"mint/internal/ptytest"
)

// fakeInterrupts stands in for the interrupt Ctrl-C sends: it hands out one channel
// the test sends on, and counts the watches started and stopped.
type fakeInterrupts struct {
	mu            sync.Mutex
	c             chan os.Signal
	starts, stops int
}

func newFakeInterrupts() *fakeInterrupts { return &fakeInterrupts{c: make(chan os.Signal, 1)} }

func (f *fakeInterrupts) watch() (<-chan os.Signal, func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts++
	return f.c, func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.stops++
	}
}

func (f *fakeInterrupts) watching() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.starts > f.stops
}

// TestInterrupt_CtrlCAtATypedQuestionClosesTheInput pins Ctrl-C at a question read
// as a typed line at a terminal, where the terminal sends SIGINT, not a byte: the read
// ends at once with ErrInputClosed, whatever was half typed, as a keypress read's
// Ctrl-C does, and every question after it is closed too, reading nothing. The watch
// for the interrupt runs only while the line is read. On the Rail and in plain alike.
func TestInterrupt_CtrlCAtATypedQuestionClosesTheInput(t *testing.T) {
	t.Parallel()
	// Two words alike in their first letter, so the Rail reads a typed word.
	source := presenter.SourceGate([]presenter.GateChoice{{Key: "fresh", Action: "re-diff + AI"}, {Key: "file", Action: "a notes file"}}, "fresh")
	rail := func(out io.Writer, tty io.Reader, i presenter.Interrupts) presenter.Presenter {
		return presenter.NewRailPresenter(out,
			presenter.WithRailInput(tty),
			presenter.WithRailInterrupts(i),
			presenter.WithRailProfile(termenv.Ascii),
			presenter.WithRailSize(func() (int, int) { return 80, 24 }),
		)
	}
	plain := func(out io.Writer, tty io.Reader, i presenter.Interrupts) presenter.Presenter {
		return presenter.NewPlainPresenterWithInput(out, io.Discard, tty).WithInterrupts(i)
	}
	prompt := func(g presenter.Gate) func(presenter.Presenter) (string, error) {
		return func(p presenter.Presenter) (string, error) {
			c, err := p.Prompt(g)
			return string(c), err
		}
	}
	askLine := func(p presenter.Presenter) (string, error) { return p.AskLine("What should change?") }

	for _, tt := range []struct {
		name  string
		build func(out io.Writer, tty io.Reader, i presenter.Interrupts) presenter.Presenter
		ask   func(presenter.Presenter) (string, error)
		asked string
		// unasked is whether a question after the interrupt draws nothing, as the
		// Rail's never opens it.
		unasked bool
	}{
		{name: "the rail's typed choice", build: rail, ask: prompt(source), asked: "Source?", unasked: true},
		{name: "the rail's typed line", build: rail, ask: askLine, asked: "What should change?", unasked: true},
		{name: "plain's choice", build: plain, ask: prompt(reviewGate()), asked: "Release with these notes?"},
		{name: "plain's line", build: plain, ask: askLine, asked: "What should change?"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pty, tty := ptytest.Open(t)
			out := &syncedBuffer{}
			interrupts := newFakeInterrupts()
			p := tt.build(out, tty, interrupts.watch)
			defer presenter.Close(p)

			got := make(chan error, 1)
			go func() {
				_, err := tt.ask(p)
				got <- err
			}()
			waitFor(t, out, tt.asked)
			ptytest.Type(t, pty, "fre")
			interrupts.c <- os.Interrupt

			select {
			case err := <-got:
				if !errors.Is(err, presenter.ErrInputClosed) {
					t.Errorf("answered %v, want ErrInputClosed", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("Ctrl-C never ended the read")
			}
			if interrupts.watching() {
				t.Error("the interrupt is still watched once the read is over")
			}
			ptytest.Type(t, pty, "\r")
			before := out.String()
			if a, err := tt.ask(p); !errors.Is(err, presenter.ErrInputClosed) {
				t.Errorf("the next question answered %q, %v, want ErrInputClosed, reading nothing", a, err)
			}
			if drawn := out.String()[len(before):]; tt.unasked && drawn != "" {
				t.Errorf("the next question drew %q, want it never asked", drawn)
			}
		})
	}
}

// TestInterrupt_ALineTypedToItsEnterAnswersAndEndsTheWatch pins that the interrupt's
// watch changes nothing for a line typed to its Enter: it answers, and the watch
// stops with the read.
func TestInterrupt_ALineTypedToItsEnterAnswersAndEndsTheWatch(t *testing.T) {
	t.Parallel()
	pty, tty := ptytest.Open(t)
	out := &syncedBuffer{}
	interrupts := newFakeInterrupts()
	p := presenter.NewPlainPresenterWithInput(out, io.Discard, tty).WithInterrupts(interrupts.watch)

	got := make(chan string, 1)
	go func() {
		line, _ := p.AskLine("What should change?")
		got <- line
	}()
	waitFor(t, out, "What should change?")
	ptytest.Type(t, pty, "shorter\r")

	select {
	case line := <-got:
		if line != "shorter" {
			t.Errorf("answered %q, want shorter", line)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the line never answered")
	}
	if interrupts.watching() || interrupts.starts != 1 {
		t.Errorf("watches started %d, still watching %v; want one, over with the read", interrupts.starts, interrupts.watching())
	}
}

// TestInterrupt_PlainsFailureLineStartsARowOfItsOwnAfterCtrlC pins plain at a
// terminal: the terminal echoes Ctrl-C as ^C with no newline, so plain ends that row
// before the failure that says why, which would otherwise read on from the ^C. A line
// typed to its Enter ends its own row, and plain adds none.
func TestInterrupt_PlainsFailureLineStartsARowOfItsOwnAfterCtrlC(t *testing.T) {
	t.Parallel()
	failed := presenter.StageFailure{Name: "version", Key: "version", Message: presenter.ErrInputClosed.Error()}
	for _, tt := range []struct {
		name  string
		ask   func(presenter.Presenter) error
		asked string
	}{
		{name: "a choice", ask: func(p presenter.Presenter) error { _, err := p.Prompt(reviewGate()); return err }, asked: "Release with these notes?"},
		{name: "a line", ask: func(p presenter.Presenter) error { _, err := p.AskLine("What should change?"); return err }, asked: "What should change?"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pty, tty := ptytest.Open(t)
			interrupts := newFakeInterrupts()
			p := presenter.NewPlainPresenterWithInput(tty, io.Discard, tty).WithInterrupts(interrupts.watch)

			got := make(chan error, 1)
			go func() { got <- tt.ask(p) }()
			if asked := ptytest.Written(t, pty); !strings.Contains(asked, tt.asked) {
				t.Fatalf("the terminal shows %q, want the question asked", asked)
			}
			ptytest.Type(t, pty, "fre\x03")
			interrupts.c <- os.Interrupt
			select {
			case err := <-got:
				if !errors.Is(err, presenter.ErrInputClosed) {
					t.Fatalf("answered %v, want ErrInputClosed", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("Ctrl-C never ended the read")
			}
			p.StageFailed(failed)

			shown := ptytest.Written(t, pty)
			if !strings.Contains(shown, "^C\r\nversion: FAILED - ") {
				t.Errorf("the terminal shows %q, want the failure on the row under the ^C", shown)
			}
		})
	}
}
