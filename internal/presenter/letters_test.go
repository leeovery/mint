package presenter_test

import (
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
	"mint/internal/presenter/frametest"
	"mint/internal/ptytest"
)

// TestRailGate_AWordsLetterAnswersAtAKeypress pins that at a terminal a question
// whose keys are words is answered by a single keypress, its word's first letter, as
// every other question is: no Enter, the keys line gone once it is answered, and the
// word settled as the answer's row.
func TestRailGate_AWordsLetterAnswersAtAKeypress(t *testing.T) {
	t.Parallel()
	pty, tty := ptytest.Open(t)
	term := frametest.NewTerminal(100, 24)
	out := &syncedBuffer{}
	p := presenter.NewRailPresenter(io.MultiWriter(term, out),
		presenter.WithRailInput(tty),
		presenter.WithRailProfile(termenv.Ascii),
		presenter.WithRailSize(func() (int, int) { return 100, 24 }),
	)
	defer presenter.Close(p)
	p.RunStarted(presenter.RunInfo{Header: presenter.Header{Command: "release regenerate", Where: pigeonWhere}, Start: presenter.Station{Name: "v1.4.0"}})

	source := presenter.SourceGate([]presenter.GateChoice{{Key: "fresh", Action: "re-diff + AI"}, {Key: "tag", Action: "tag annotation body"}}, "fresh")
	type answer struct {
		got presenter.Choice
		err error
	}
	got := make(chan answer, 1)
	go func() {
		c, err := p.Prompt(source)
		got <- answer{c, err}
	}()
	waitFor(t, out, "Source?")
	ptytest.Type(t, pty, "t")

	select {
	case a := <-got:
		if a.err != nil || a.got != "tag" {
			t.Fatalf("answered %q, %v, want tag at the keypress t", a.got, a.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the keypress never answered the question")
	}
	if err := term.Err(); err != nil {
		t.Fatalf("the terminal was written what it can't apply: %v", err)
	}
	screen := term.Grid().Text()
	if !slices.Contains(screen, " ●  source        tag") {
		t.Errorf("the terminal shows\n%s\nwant the answer settled as its row", strings.Join(screen, "\n"))
	}
	if slices.ContainsFunc(screen, func(s string) bool { return strings.Contains(s, "tag annotation body") }) {
		t.Errorf("the terminal shows\n%s\nwant the keys line gone once answered", strings.Join(screen, "\n"))
	}
}
