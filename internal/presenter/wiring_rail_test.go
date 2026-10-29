package presenter_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
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

func TestNewForStartup_WithRailDrawsPrettyAsTheRailAndLeavesPlainAndTodaysPrettyAlone(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		signals presenter.StartupSignals
		opts    []presenter.StartupOption
		want    string
	}{
		{name: "pretty on the rail", signals: prettySignals, opts: []presenter.StartupOption{presenter.WithRail()}, want: " mint ─ release"},
		{name: "today's pretty", signals: prettySignals, want: "🌿 mint"},
		{name: "plain, rail asked for", signals: plainSignals, opts: []presenter.StartupOption{presenter.WithRail()}, want: "mint: releasing pigeon v1.4.0\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := startupFile(t, "out")

			p := presenter.NewForSignals(tc.signals, false, out, &bytes.Buffer{}, startupFile(t, "in"), tc.opts...)
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
		p := presenter.NewForSignals(prettySignals, true, out, &bytes.Buffer{}, startupFile(t, "in"), presenter.WithRail())
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
		p := presenter.NewForSignals(presenter.StartupSignals{Mode: presenter.ModePretty}, false, startupFile(t, "out"), errStream, startupFile(t, "in"), presenter.WithRail())

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

func TestClose_EndsTheRailsDrawingAndLeavesOtherPresentersAlone(t *testing.T) {
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
