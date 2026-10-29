package engine_test

// This file holds whole init runs, the real orchestrator writing into a repo of its
// own with git scripted, each drawn two ways: on the rail (rail_frames_test.go holds
// those frames) and in plain, whose transcript, stdout then stderr, is held byte for
// byte to testdata/plain beside regenerate's. Plain's transcripts were written before
// init moved onto the rail and pin that it changed nothing plain prints.
// `go test ./internal/engine -run PlainTranscripts -update-plain` writes them again.

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/frametest"
	"mint/internal/runner"
)

// initRun is one whole init run: what the repo holds before it.
type initRun struct {
	present []string
}

var initRuns = map[string]initRun{
	// An empty repo: both files created.
	"init-empty-100": {},
	// A repo with its .mint.toml already there: the release shim created beside it.
	"init-present-100": {present: []string{".mint.toml"}},
}

// initialise runs r into pigeon at ~/Code/pigeon under a home of its own, drawing on p.
func initialise(t *testing.T, p presenter.Presenter, r initRun) error {
	t.Helper()
	home := t.TempDir()
	root := filepath.Join(home, "Code", "pigeon")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range r.present {
		if err := os.WriteFile(filepath.Join(root, name), []byte("# already here\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f := runner.NewFakeRunner()
	f.SeedSequence("git", ScriptedOut(root+"\n")) // rev-parse --show-toplevel
	deps := engine.InitDeps{Presenter: p, Runner: f, MintVersion: "0.1.2", Home: home}
	return engine.Init(t.Context(), deps, engine.InitOptions{})
}

// initFrames draws every init run on the rail, on a terminal set up as the release
// frames' is, by name.
func initFrames(t *testing.T) map[string]string {
	t.Helper()
	drawn := map[string]string{}
	for name, r := range initRuns {
		term := frametest.NewTerminal(railCols, railRows)
		p := presenter.NewRailPresenter(term,
			presenter.WithRailProfile(termenv.Ascii),
			presenter.WithRailSize(func() (int, int) { return railCols, railRows }),
			presenter.WithRailTicker(stillTicker),
		)
		if err := initialise(t, p, r); err != nil {
			t.Errorf("%s: Init returned %v", name, err)
		}
		p.Close()
		if err := term.Err(); err != nil {
			t.Fatalf("the terminal was written what it can't apply: %v", err)
		}
		drawn[name] = term.ANSI()
	}
	return drawn
}

func TestInit_PlainTranscripts(t *testing.T) {
	t.Parallel()

	for name, r := range initRuns {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			if err := initialise(t, presenter.NewPlainPresenter(&stdout, &stderr), r); err != nil {
				t.Fatalf("Init returned %v", err)
			}
			heldPlain(t, name, transcript(stdout.String(), stderr.String()))
		})
	}
}

// heldPlain holds transcript to plain's transcript of the run name, or writes it
// again under -update-plain.
func heldPlain(t *testing.T, name, transcript string) {
	t.Helper()
	path := filepath.Join("testdata", "plain", name+".txt")
	if *updatePlain {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(transcript), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v: write the transcripts with -update-plain", err)
	}
	if transcript != string(want) {
		t.Errorf("plain printed\n%s\nwant, as before the rail\n%s", transcript, want)
	}
}
