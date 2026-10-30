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
	"mint/internal/ptytest"
)

// syncedBuffer is a presenter's output, written while the test reads it.
type syncedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncedBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncedBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// waitFor waits until out holds text, failing the test after a while.
func waitFor(t *testing.T, out *syncedBuffer, text string) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !strings.Contains(out.String(), text); {
		if time.Now().After(deadline) {
			t.Fatalf("out never held %q:\n%s", text, out.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestTypedAhead_WhatWasTypedBeforeTheQuestionNeverAnswersIt pins that a question
// asked at a terminal discards what was typed before it was asked: a stray Enter
// pressed while the notes were written never accepts them, and a line typed early
// never answers the line asked for. Only what is typed once the question is up
// answers it, as a keypress or a typed line, on the Rail and in plain alike.
func TestTypedAhead_WhatWasTypedBeforeTheQuestionNeverAnswersIt(t *testing.T) {
	t.Parallel()
	source := presenter.SourceGate([]presenter.GateChoice{{Key: "fresh", Action: "re-diff + AI"}, {Key: "tag", Action: "tag annotation body"}}, "fresh")
	rail := func(out io.Writer, tty io.Reader) presenter.Presenter {
		return presenter.NewRailPresenter(out,
			presenter.WithRailInput(tty),
			presenter.WithRailProfile(termenv.Ascii),
			presenter.WithRailSize(func() (int, int) { return 80, 24 }),
		)
	}
	plain := func(out io.Writer, tty io.Reader) presenter.Presenter {
		return presenter.NewPlainPresenterWithInput(out, io.Discard, tty)
	}
	prompt := func(g presenter.Gate) func(presenter.Presenter) (string, error) {
		return func(p presenter.Presenter) (string, error) {
			c, err := p.Prompt(g)
			return string(c), err
		}
	}
	askLine := func(p presenter.Presenter) (string, error) { return p.AskLine("What should change?") }

	tests := []struct {
		name   string
		build  func(out io.Writer, tty io.Reader) presenter.Presenter
		ask    func(presenter.Presenter) (string, error)
		asked  string
		ahead  string
		answer string
		want   string
	}{
		{name: "the rail's keypress", build: rail, ask: prompt(reviewGate()), asked: "Release with these notes?", ahead: "\r", answer: "n", want: "n"},
		{name: "the rail's typed choice", build: rail, ask: prompt(source), asked: "Source?", ahead: "tag\r", answer: "fresh\r", want: "fresh"},
		{name: "the rail's typed line", build: rail, ask: askLine, asked: "What should change?", ahead: "stale\r", answer: "shorter\r", want: "shorter"},
		{name: "plain's choice", build: plain, ask: prompt(reviewGate()), asked: "Release with these notes?", ahead: "\r", answer: "n\r", want: "n"},
		{name: "plain's line", build: plain, ask: askLine, asked: "What should change?", ahead: "stale\r", answer: "shorter\r", want: "shorter"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pty, tty := ptytest.Open(t)
			out := &syncedBuffer{}
			p := tt.build(out, tty)
			defer presenter.Close(p)

			ptytest.Type(t, pty, tt.ahead)
			type answer struct {
				got string
				err error
			}
			got := make(chan answer, 1)
			go func() {
				a, err := tt.ask(p)
				got <- answer{a, err}
			}()
			waitFor(t, out, tt.asked)
			ptytest.Type(t, pty, tt.answer)

			select {
			case a := <-got:
				if a.err != nil || a.got != tt.want {
					t.Errorf("answered %q, %v, want %q: what was typed once the question was up", a.got, a.err, tt.want)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("the question was never answered")
			}
		})
	}
}
