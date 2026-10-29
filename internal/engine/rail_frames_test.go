package engine_test

// This file holds the golden runs of a release on the rail: the real orchestrator,
// its git, AI and gh scripted on a FakeRunner, drawing through a RailPresenter onto a
// virtual terminal 100 columns wide with colour off. Each run's whole screen once it
// is over, scrollback included, is held to its frame in testdata/frames cell for
// cell. The engine is timed by a clock that steps at each read, and the presenter's
// spinner never ticks, so every time and frame is the same in every run.
// `go test ./internal/engine -run Frames -update` writes the frames again.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/muesli/termenv"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/git"
	"mint/internal/presenter"
	"mint/internal/presenter/frametest"
	"mint/internal/release"
	"mint/internal/runner"
	"mint/internal/version"
)

const (
	railCols, railRows = 100, 40
	railURL            = "https://github.com/acme/pigeon/releases/tag/v1.3.0"
	railNotes          = "Webhooks retry on their own now, so a flaky endpoint no longer drops events.\n" +
		"\n" +
		"## ✨ Added\n" +
		"- Failed webhook deliveries retry with backoff for up to an hour.\n" +
		"\n" +
		"## 🐛 Fixed\n" +
		"- Signing secrets with a trailing newline are accepted again.\n"
	railRegenNotes = "Webhooks now retry failed deliveries for up to an hour.\n" +
		"\n" +
		"## ✨ Added\n" +
		"- Backoff retries for failed webhook deliveries.\n"
	pushRejectedStderr = " ! [rejected]        main -> main (fetch first)\n" +
		"error: failed to push some refs to 'https://github.com/acme/pigeon'\n"
	ghFailure = "HTTP 502: Bad Gateway (https://api.github.com/repos/acme/pigeon/releases)\n"
)

// railEpoch is when every golden run starts.
var railEpoch = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

// steppingClock is the engine's clock in the golden runs: each read is step past the
// one before.
type steppingClock struct {
	mu   sync.Mutex
	now  time.Time
	step time.Duration
}

func (c *steppingClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(c.step)
	return c.now
}

// stillTicker is a spinner clock that never ticks.
func stillTicker(time.Duration) (<-chan time.Time, func()) {
	return make(chan time.Time), func() {}
}

// railWorld is pigeon released on the rail: its root at ~/Code/pigeon under a home of
// its own, its commands scripted, and the deps of a run drawing on the terminal.
type railWorld struct {
	root string
	f    *runner.FakeRunner
	term *frametest.Terminal
	p    *presenter.RailPresenter
	deps engine.ReleaseDeps
}

// newRailWorld is pigeon at a terminal under config, answering its questions with
// answers, a line each, under -y where yes says so.
func newRailWorld(t *testing.T, config, answers string, yes bool) *railWorld {
	t.Helper()
	home := t.TempDir()
	root := filepath.Join(home, "Code", "pigeon")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if config != "" {
		writeConfig(t, root, config)
	}
	w := &railWorld{root: root, f: runner.NewFakeRunner(), term: frametest.NewTerminal(railCols, railRows)}
	w.p = presenter.NewRailPresenter(w.term,
		presenter.WithRailInput(strings.NewReader(answers)),
		presenter.WithRailGating(yes, true),
		presenter.WithRailProfile(termenv.Ascii),
		presenter.WithRailSize(func() (int, int) { return railCols, railRows }),
		presenter.WithRailTicker(stillTicker),
		presenter.WithRailClock(func() time.Time { return railEpoch }),
	)
	mut := git.NewMutator(w.f)
	w.deps = engine.ReleaseDeps{
		Presenter:        w.p,
		Runner:           w.f,
		Mutator:          mut,
		Releaser:         release.NewReleaser(mut),
		Yes:              yes,
		StdinInteractive: true,
		MintVersion:      "0.1.2",
		Home:             home,
		Clock:            (&steppingClock{now: railEpoch, step: 700 * time.Millisecond}).Now,
	}
	return w
}

// readGates scripts git from the repo's root to the starting commit's capture: pigeon
// at v1.2.3 on main, every preflight gate passing, its work stashed first under
// --autostash.
func (w *railWorld) readGates(stash bool) {
	w.f.SeedSequence("git",
		ScriptedOut(w.root),        // rev-parse --show-toplevel
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(priorTag+"\n"), // tag --list
		ScriptedHead(),             // rev-parse HEAD --abbrev-ref HEAD (the start station)
	)
	if stash {
		w.f.SeedSequence("git", stashSaved()) // stash push --include-untracked
	}
	w.f.SeedSequence("git",
		ScriptedOut(""),          // fetch --tags
		ScriptedOut(""),          // status --porcelain (clean)
		ScriptedOut("main"),      // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedNonZero(),        // rev-parse -q --verify refs/tags/v1.3.0 (absent)
		ScriptedOut("0\t1"),      // rev-list left-right count (ahead only)
		ScriptedOut(""),          // ls-remote --tags (tag free remote)
		ScriptedOut(startingSHA), // rev-parse HEAD (capture the clean start)
	)
}

// notes scripts the AI notes: the diff and its Change Map, the exclusion check
// finding nothing, then the notes written in one call.
func (w *railWorld) notes() {
	seedNormalAINotes(w.f)
	seedClaudeAfterCheck(w.f, ScriptedOut(railNotes))
}

// recorded scripts the bookkeeping commit, the provider's detection and the tag.
func (w *railWorld) recorded() {
	w.f.SeedSequence("git",
		ScriptedOut(""), // -C root add CHANGELOG.md
		ScriptedOut(""), // -C root commit -m
		ScriptedOut("https://github.com/acme/pigeon.git"), // remote get-url origin
		ScriptedOut(""), // tag -a v1.3.0 -F -
	)
}

// pushed scripts the atomic push going through.
func (w *railWorld) pushed() {
	w.f.SeedSequence("git", ScriptedOut("")) // push --atomic origin HEAD v1.3.0
}

// published scripts gh signed in, and the release's creation answering with publish.
func (w *railWorld) published(publish runner.ScriptedCall) {
	w.f.SeedSequence("gh", ScriptedOut(""), publish) // auth status, release create
}

// release runs a minor release with opts set, then ends the drawing as the cmd layer
// does, and returns the run's error and everything on the terminal.
func (w *railWorld) release(t *testing.T, set func(*engine.ReleaseOptions)) (string, error) {
	t.Helper()
	opts := engine.ReleaseOptions{Bump: version.BumpMinor, Now: fixedClock}
	if set != nil {
		set(&opts)
	}
	err := engine.Release(t.Context(), w.deps, opts)
	w.p.Close()
	if terr := w.term.Err(); terr != nil {
		t.Fatalf("the terminal was written what it can't apply: %v", terr)
	}
	return w.term.ANSI(), err
}

// railRegenerator writes the notes again, in one call, as body.
type railRegenerator string

func (r railRegenerator) Regenerate(context.Context, string) (string, error) { return string(r), nil }

// railRun is one golden run: the run it draws, and whether it ends in an abort.
type railRun struct {
	run     func(t *testing.T) (string, error)
	aborted bool
}

var railRuns = map[string]railRun{
	// A release through its review, accepted.
	"release-accepted-100": {run: func(t *testing.T) (string, error) {
		w := newRailWorld(t, "", "y\ny\n", false)
		w.readGates(false)
		w.notes()
		w.recorded()
		w.pushed()
		w.published(ScriptedOut(railURL + "\n"))
		return w.release(t, nil)
	}},
	// A release whose review is answered r, the notes written again in one call, then y.
	"release-regenerated-100": {run: func(t *testing.T) (string, error) {
		w := newRailWorld(t, "", "y\nr\n\ny\n", false)
		w.deps.Regenerator = railRegenerator(railRegenNotes)
		w.readGates(false)
		w.notes()
		w.recorded()
		w.pushed()
		w.published(ScriptedOut(railURL + "\n"))
		return w.release(t, nil)
	}},
	// A release whose pre_tag hook committed artifacts and whose review is answered r,
	// the r's call failing: the notes asked about stay as answered, the r's own node
	// fails with the call's output, and the artifacts commit unwinds.
	"release-regen-failed-100": {aborted: true, run: func(t *testing.T) (string, error) {
		w := newRailWorld(t, preTagHookConfig, "y\nr\n"+exportNudge+"\n", false)
		w.deps.Transport = &gateAbortTransport{nudge: exportNudge, classify: "OTHER", regen: failsWith(&ai.GenerationError{Stdout: loginOutput, ExitCode: 1})}
		w.readGates(false)
		seedPreTagArtifactCommit(w.f)
		singlePassNotesAndRegen(w.f)
		w.f.SeedSequence("git", ScriptedOut("")) // unwind: reset --hard startsha
		return w.release(t, nil)
	}},
	// A release whose push is rejected, before the point of no return, unwinding.
	"release-unwound-100": {aborted: true, run: func(t *testing.T) (string, error) {
		w := newRailWorld(t, "", "", true)
		w.readGates(false)
		w.notes()
		w.recorded()
		w.f.SeedSequence("gh", ScriptedOut("")) // auth status
		w.f.SeedSequence("git",
			gitRejected(pushRejectedStderr), // push --atomic origin HEAD v1.3.0
			ScriptedOut(""),                 // unwind: tag -d v1.3.0
			ScriptedOut(""),                 // unwind: reset --hard startsha
		)
		return w.release(t, nil)
	}},
	// The same rejected push, its unwind unable to reset the commit it made.
	"release-unwind-incomplete-100": {aborted: true, run: func(t *testing.T) (string, error) {
		w := newRailWorld(t, "", "", true)
		w.readGates(false)
		w.notes()
		w.recorded()
		w.f.SeedSequence("gh", ScriptedOut("")) // auth status
		w.f.SeedSequence("git",
			gitRejected(pushRejectedStderr), // push --atomic origin HEAD v1.3.0
			ScriptedOut(""),                 // unwind: tag -d v1.3.0
			gitFailing(resetStderr),         // unwind: reset --hard startsha
		)
		return w.release(t, nil)
	}},
	// A dry run: its hooks skipped, the notes written, the review previewed, nothing
	// changed.
	"release-dry-run-100": {run: func(t *testing.T) (string, error) {
		w := newRailWorld(t, "[release.hooks]\npreflight = \"make lint\"\npre_tag = \"npm ci && npm run build\"\npost_release = \"./scripts/notify.sh\"\n", "", false)
		w.readGates(false)
		w.notes()
		return w.release(t, func(o *engine.ReleaseOptions) { o.DryRun = true })
	}},
	// A publish failing past the point of no return: it warns, and the run still lands.
	"release-publish-warned-100": {run: func(t *testing.T) (string, error) {
		w := newRailWorld(t, "", "", true)
		w.readGates(false)
		w.notes()
		w.recorded()
		w.pushed()
		w.published(gitFailing(ghFailure))
		return w.release(t, nil)
	}},
	// The exclusion check's pause, answered exclude, its answers committed.
	"release-excluded-100": {run: func(t *testing.T) (string, error) {
		w := newRailWorld(t, "", "y\ne\ny\n", false)
		w.readGates(false)
		w.f.SeedSequence("git", withAnswersCommit(append([]runner.ScriptedCall{
			ScriptedOut(exportGoDiff + designDocDiff), // diff, the check asking about docs/design/
			ScriptedOut(exportGoDiff),                 // diff, docs/design/ excluded
		}, exportOnlyChangeMap...)...)...)
		w.f.SeedSequence("claude", ScriptedOut(designDocAnswer), ScriptedOut(railNotes))
		w.recorded()
		w.pushed()
		w.published(ScriptedOut(railURL + "\n"))
		return w.release(t, nil)
	}},
	// A release under --autostash whose work won't go back cleanly once it is out.
	"release-autostash-conflict-100": {run: func(t *testing.T) (string, error) {
		w := newRailWorld(t, "", "", true)
		w.readGates(true)
		w.notes()
		w.recorded()
		w.pushed()
		w.f.SeedSequence("git", stashPopConflict()) // stash pop
		w.published(ScriptedOut(railURL + "\n"))
		return w.release(t, func(o *engine.ReleaseOptions) { o.AutoStash = true })
	}},
}

// gitRejected is a push git rejected, as production's runner reports it: a non-zero
// exit writing stderr, which the releaser wraps in release.ErrPushRejected.
func gitRejected(stderr string) runner.ScriptedCall {
	return gitFailing(stderr)
}

func TestRelease_RailFrames(t *testing.T) {
	t.Parallel()

	drawn := map[string]string{}
	for name, r := range railRuns {
		screen, err := r.run(t)
		if aborted := err != nil; aborted != r.aborted {
			t.Errorf("%s: Release returned %v, want aborted %v", name, err, r.aborted)
		}
		drawn[name] = screen
	}

	frametest.Check(t, "testdata/frames", drawn)
}
