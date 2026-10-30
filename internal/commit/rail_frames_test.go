package commit_test

// This file holds the commit runs (runs_test.go) to their frames on the rail: the real
// orchestrator drawing through a RailPresenter onto a virtual terminal 100 columns wide
// with colour off. Each run's whole screen once it is over, scrollback included, is
// held to its frame in testdata/frames cell for cell. The commit is timed by a clock
// that steps at each read, and the presenter's spinner never ticks, so every time and
// frame is the same in every run.
// `go test ./internal/commit -run Frames -update` writes the frames again.

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/muesli/termenv"

	"mint/internal/git"
	"mint/internal/presenter"
	"mint/internal/presenter/frametest"
	"mint/internal/runner"
)

const railCols, railRows = 100, 40

// stillTicker is a spinner clock that never ticks.
func stillTicker(time.Duration) (<-chan time.Time, func()) {
	return make(chan time.Time), func() {}
}

// drawCommit runs r on the rail, then ends the drawing as the cmd layer does, and
// returns the run's error and everything on the terminal. asked, where set, is handed
// the terminal as the first question waits for its answer.
func drawCommit(t *testing.T, r commitRun, asked func(*frametest.Terminal)) (string, error) {
	t.Helper()
	return drawCommitAt(t, r, asked, nil)
}

// drawCommitAt is drawCommit, keeping in g, where set, the screen as it stood when the
// run first ran the command g watches for.
func drawCommitAt(t *testing.T, r commitRun, asked func(*frametest.Terminal), g *glimpsing) (string, error) {
	t.Helper()
	term := frametest.NewTerminal(railCols, railRows)
	var in io.Reader = strings.NewReader(r.answers)
	if asked != nil {
		in = &askedInput{r: in, asked: func() { asked(term) }}
	}
	p := presenter.NewRailPresenter(term,
		presenter.WithRailInput(in),
		presenter.WithRailGating(false, true),
		presenter.WithRailProfile(termenv.Ascii),
		presenter.WithRailSize(func() (int, int) { return railCols, railRows }),
		presenter.WithRailTicker(stillTicker),
		presenter.WithRailClock(func() time.Time { return runEpoch }),
	)
	w := newCommitWorld(t, p)
	if g != nil {
		g.CommandRunner, g.term = w.deps.Runner, term
		w.deps.Runner, w.deps.Mutator = g, git.NewMutator(g)
	}
	err := r.run(t, w)
	p.Close()
	if terr := term.Err(); terr != nil {
		t.Fatalf("the terminal was written what it can't apply: %v", terr)
	}
	return term.ANSI(), err
}

// glimpsing is a run's runner keeping the terminal's screen as it stands when the run
// first runs name with arg as its first argument, as a reader sees it while that
// command runs.
type glimpsing struct {
	runner.CommandRunner
	term      *frametest.Terminal
	name, arg string
	screen    string
}

func (g *glimpsing) Run(ctx context.Context, name string, args ...string) (runner.Result, error) {
	g.look(name, args)
	return g.CommandRunner.Run(ctx, name, args...)
}

func (g *glimpsing) RunWith(ctx context.Context, stdin io.Reader, name string, args ...string) (runner.Result, error) {
	g.look(name, args)
	return g.CommandRunner.RunWith(ctx, stdin, name, args...)
}

func (g *glimpsing) RunInDir(ctx context.Context, dir string, env []string, name string, args ...string) (runner.Result, error) {
	g.look(name, args)
	return g.CommandRunner.RunInDir(ctx, dir, env, name, args...)
}

func (g *glimpsing) look(name string, args []string) {
	if g.screen == "" && name == g.name && len(args) > 0 && args[0] == g.arg {
		g.screen = g.term.ANSI()
	}
}

// commitGlimpses are the -Ap run's screens while it runs a command, each by its frame's
// name: the diff counted, everything staged, and the commit going in, its hooks
// running.
var commitGlimpses = map[string]struct{ name, arg string }{
	"commit-diff-running-100":   {"git", "diff"},
	"commit-stage-running-100":  {"git", "add"},
	"commit-commit-running-100": {"git", "commit"},
}

// askedInput is answers that hand over the terminal as the first is read, the first
// question waiting.
type askedInput struct {
	r     io.Reader
	once  sync.Once
	asked func()
}

func (in *askedInput) Read(p []byte) (int, error) {
	in.once.Do(in.asked)
	return in.r.Read(p)
}

// TestCommit_Frames holds each commit run's whole screen once it is over, and the -Ap
// run's screen as its question waits: the message hanging off the rail under the
// question, the stages to come and the end station below.
func TestCommit_Frames(t *testing.T) {
	t.Parallel()

	drawn := map[string]string{}
	for name, r := range commitRuns {
		var asked func(*frametest.Terminal)
		if name == "commit-pushed-100" {
			asked = func(term *frametest.Terminal) { drawn["commit-review-100"] = term.ANSI() }
		}
		screen, err := drawCommit(t, r, asked)
		if aborted := err != nil; aborted != r.aborted {
			t.Errorf("%s: the run returned %v, want aborted %v", name, err, r.aborted)
		}
		drawn[name] = screen
	}
	for name, at := range commitGlimpses {
		g := &glimpsing{name: at.name, arg: at.arg}
		if _, err := drawCommitAt(t, commitRuns["commit-pushed-100"], nil, g); err != nil {
			t.Errorf("%s: the run returned %v", name, err)
		}
		if g.screen == "" {
			t.Fatalf("%s: the run never ran %s %s", name, at.name, at.arg)
		}
		drawn[name] = g.screen
	}

	frametest.Check(t, "testdata/frames", drawn)
}
