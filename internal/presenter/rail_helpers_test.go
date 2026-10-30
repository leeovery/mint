package presenter_test

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
	"mint/internal/presenter/frametest"
)

// railTerm is a rail presenter drawing on a virtual terminal, colour off, at a size,
// time and spinner ticks the test sets.
type railTerm struct {
	*presenter.RailPresenter
	term  *frametest.Terminal
	err   *bytes.Buffer
	clock *fakeClock
	ticks *fakeTicker
	size  *termSize
	// writes counts the writes the presenter made to the terminal.
	writes *countingWriter
}

// newRailTerm is a rail presenter on a terminal cols by rows, reading its answers
// from in.
func newRailTerm(t *testing.T, cols, rows int, in io.Reader) *railTerm {
	t.Helper()
	return newRailTermIn(t, termenv.Ascii, cols, rows, in)
}

// newRailTermIn is newRailTerm drawing in profile's colour.
func newRailTermIn(t *testing.T, profile termenv.Profile, cols, rows int, in io.Reader) *railTerm {
	t.Helper()
	r := &railTerm{
		term:  frametest.NewTerminal(cols, rows),
		err:   &bytes.Buffer{},
		clock: &fakeClock{now: time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)},
		ticks: &fakeTicker{},
		size:  &termSize{cols: cols, rows: rows},
	}
	r.writes = &countingWriter{w: r.term}
	r.RailPresenter = presenter.NewRailPresenter(r.writes,
		presenter.WithRailErr(r.err),
		presenter.WithRailInput(in),
		presenter.WithRailProfile(profile),
		presenter.WithRailSize(r.size.get),
		presenter.WithRailTicker(r.ticks.start),
		presenter.WithRailClock(r.clock.Now),
	)
	t.Cleanup(r.Close)
	return r
}

// screen is everything on the terminal, a line a row, failing on anything the
// terminal couldn't apply.
func (r *railTerm) screen(t *testing.T) []string {
	t.Helper()
	if err := r.term.Err(); err != nil {
		t.Fatalf("the terminal was written what it can't apply: %v", err)
	}
	return r.term.Grid().Text()
}

// frame is everything on the terminal as ANSI, for a golden frame.
func (r *railTerm) frame(t *testing.T) string {
	t.Helper()
	if err := r.term.Err(); err != nil {
		t.Fatalf("the terminal was written what it can't apply: %v", err)
	}
	return r.term.ANSI()
}

type termSize struct {
	mu         sync.Mutex
	cols, rows int
}

func (s *termSize) get() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cols, s.rows
}

func (s *termSize) set(cols, rows int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cols, s.rows = cols, rows
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// fakeTicker is the spinner's ticker as the test drives it: it ticks only when told.
type fakeTicker struct {
	mu            sync.Mutex
	ch            chan time.Time
	starts, stops int
}

func (f *fakeTicker) start(time.Duration) (<-chan time.Time, func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts++
	ch := make(chan time.Time)
	f.ch = ch
	return ch, func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.stops++
	}
}

// running is whether a ticker has started and not stopped.
func (f *fakeTicker) running() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.starts > f.stops
}

// fire sends a tick to the last ticker started, and reports whether it was taken
// within wait.
func (f *fakeTicker) fire(wait time.Duration) bool {
	f.mu.Lock()
	ch := f.ch
	f.mu.Unlock()
	if ch == nil {
		return false
	}
	select {
	case ch <- time.Time{}:
		return true
	case <-time.After(wait):
		return false
	}
}

// tick fires a tick and waits until the frame it draws is on the terminal: a second
// tick is taken only once the first is drawn, and draws the same, the clock unmoved.
func (f *fakeTicker) tick(t *testing.T) {
	t.Helper()
	for range 2 {
		if !f.fire(5 * time.Second) {
			t.Fatal("no spinner ticker took the tick")
		}
	}
}

// countingWriter counts the writes made through it, and keeps what they wrote.
type countingWriter struct {
	mu  sync.Mutex
	w   io.Writer
	n   int
	all strings.Builder
}

func (c *countingWriter) Write(p []byte) (int, error) {
	c.mu.Lock()
	c.n++
	c.all.Write(p)
	c.mu.Unlock()
	return c.w.Write(p)
}

func (c *countingWriter) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// written is every byte written through it.
func (c *countingWriter) written() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.all.String()
}

// scriptedInput answers each question with the next of answers, one line a read,
// first calling asked with how many questions it has been asked, for a test to look
// at the terminal while the question is up. Out of answers, input is closed.
type scriptedInput struct {
	answers []string
	n       int
	asked   func(n int)
}

func answers(lines ...string) *scriptedInput { return &scriptedInput{answers: lines} }

func (s *scriptedInput) Read(p []byte) (int, error) {
	if len(s.answers) == 0 {
		return 0, io.EOF
	}
	s.n++
	if s.asked != nil {
		s.asked(s.n)
	}
	n := copy(p, s.answers[0])
	s.answers = s.answers[1:]
	return n, nil
}

// blockingInput is input whose read waits for the test: reading is closed once a
// read is waiting, and the read takes the line sent on lines.
type blockingInput struct {
	reading chan struct{}
	once    sync.Once
	lines   chan string
}

func newBlockingInput() *blockingInput {
	return &blockingInput{reading: make(chan struct{}), lines: make(chan string)}
}

func (b *blockingInput) Read(p []byte) (int, error) {
	b.once.Do(func() { close(b.reading) })
	line, ok := <-b.lines
	if !ok {
		return 0, io.EOF
	}
	return copy(p, line), nil
}

// wantScreen fails unless the terminal shows want, a line a row.
func wantScreen(t *testing.T, r *railTerm, want ...string) {
	t.Helper()
	if got := r.screen(t); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("the terminal shows\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// pane is one row of a screen, its trailing blanks trimmed, as the terminal's text has it.
func pane(text string) string { return strings.TrimRight(text, " ") }
