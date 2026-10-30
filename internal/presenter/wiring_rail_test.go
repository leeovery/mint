package presenter_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"mint/internal/presenter"
)

// startupFile is a file standing in for a startup handle, read back once written.
func startupFile(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Create(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func written(t *testing.T, f *os.File) string {
	t.Helper()
	data, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

var (
	prettySignals = presenter.StartupSignals{Mode: presenter.ModePretty, StdinInteractive: true}
	plainSignals  = presenter.StartupSignals{Mode: presenter.ModePlain, StdinInteractive: true}
)

func TestNewForStartup_DrawsPrettyAsTheRailAndLeavesPlainAlone(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		signals presenter.StartupSignals
		want    string
	}{
		{name: "pretty", signals: prettySignals, want: " mint ─ release"},
		{name: "plain", signals: plainSignals, want: "mint: releasing pigeon v1.4.0\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := startupFile(t, "out")

			p := presenter.NewForSignals(tc.signals, false, out, &bytes.Buffer{}, startupFile(t, "in"))
			p.RunStarted(releaseStarted(false))
			presenter.Close(p)

			if got := written(t, out); !strings.HasPrefix(got, tc.want) {
				t.Errorf("out = %q, want it to open %q", got, tc.want)
			}
		})
	}
}

func TestNewForStartup_TheRailTakesTheGatingSignals(t *testing.T) {
	t.Parallel()

	t.Run("-y takes the default, unasked", func(t *testing.T) {
		t.Parallel()
		out := startupFile(t, "out")
		p := presenter.NewForSignals(prettySignals, true, out, &bytes.Buffer{}, startupFile(t, "in"))
		p.RunStarted(releaseStarted(false))
		p.ShowPlan(releasePlan())

		c, err := p.Prompt(versionGate())
		presenter.Close(p)

		if err != nil || c != presenter.ChoiceYes {
			t.Fatalf("Prompt = %q, %v, want the default under -y", c, err)
		}
		if got := written(t, out); !strings.Contains(got, "v1.4.0 (minor bump) · accepted (-y)") {
			t.Errorf("out = %q, want the version settled accepted (-y)", got)
		}
	})

	t.Run("stdin no terminal fails loud, its summary on the err stream", func(t *testing.T) {
		t.Parallel()
		errStream := &bytes.Buffer{}
		p := presenter.NewForSignals(presenter.StartupSignals{Mode: presenter.ModePretty}, false, startupFile(t, "out"), errStream, startupFile(t, "in"))

		_, err := p.Prompt(versionGate())
		presenter.Close(p)

		if !errors.Is(err, presenter.ErrNotInteractive) {
			t.Errorf("Prompt err = %v, want ErrNotInteractive", err)
		}
		if !strings.HasPrefix(errStream.String(), "✗ ") {
			t.Errorf("err stream = %q, want the failure's summary", errStream.String())
		}
	})
}

func TestClose_EndsTheRailsDrawingAndWritesNothingWhereNothingIsDrawn(t *testing.T) {
	t.Parallel()

	r := newRailTerm(t, 80, 24, answers())
	commitRun(r)
	r.NextMoves(presenter.NextMoves{Stage: "message", Moves: []presenter.NextMove{{Lead: "write the message yourself:", Command: "mint commit --no-ai"}}})
	r.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeFailed, End: presenter.Station{Name: "main", Detail: "nothing changed"}})
	presenter.Close(r.RailPresenter)
	if screen := r.screen(t); screen[len(screen)-1] != "     mint commit --no-ai" {
		t.Errorf("the rail ends %q, want next settled once its drawing ends", screen[len(screen)-1])
	}

	for _, mode := range bothModes {
		out := &bytes.Buffer{}
		presenter.Close(presenter.New(mode.mode, out, &bytes.Buffer{}))
		if out.Len() != 0 {
			t.Errorf("Close on %s wrote %q, want nothing", mode.name, out.String())
		}
	}
}

// TestNewForStartup_OnADumbTerminalTheRailDrawsNoRegion pins that TERM=dumb, a
// terminal that can't move its cursor, gets the Rail with no live region: its lines
// only settle, and nothing ever moves back up over them.
func TestNewForStartup_OnADumbTerminalTheRailDrawsNoRegion(t *testing.T) {
	cursorUp := regexp.MustCompile(`\x1b\[\d+A`)
	for _, tc := range []struct {
		term    string
		movesUp bool
	}{
		{term: "xterm-256color", movesUp: true},
		{term: "dumb", movesUp: false},
	} {
		t.Run(tc.term, func(t *testing.T) {
			t.Setenv("TERM", tc.term)
			out := startupFile(t, "out")
			p := presenter.NewForSignals(prettySignals, false, out, &bytes.Buffer{}, startupFile(t, "in"))
			p.RunStarted(releaseStarted(false))
			p.ShowPlan(releasePlan())
			p.StageStarted(presenter.StageStart{Name: "pre_tag", Key: "pre_tag", Blocking: true, Text: "running the pre_tag hook…"})
			p.StageSucceeded(presenter.StageSuccess{Name: "pre_tag", Key: "pre_tag", Blocking: true, Detail: "npm ci && npm run build"})
			presenter.Close(p)

			got := written(t, out)
			if moved := cursorUp.MatchString(got); moved != tc.movesUp {
				t.Errorf("TERM=%s moved the cursor up: %v, want %v\n%q", tc.term, moved, tc.movesUp, got)
			}
			if !strings.Contains(got, " ●  pre_tag") {
				t.Errorf("TERM=%s: out = %q, want the pre_tag node settled", tc.term, got)
			}
		})
	}
}
