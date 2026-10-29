package engine_test

// This file holds whole regenerate runs, the real orchestrators with their git and gh
// scripted on a FakeRunner, each drawn two ways: on the rail (rail_frames_test.go
// holds those frames) and in plain, whose transcript, stdout then stderr, is held
// byte for byte to testdata/plain. Plain's transcripts were written before the rail
// switch and pin that it changed nothing plain prints.
// `go test ./internal/engine -run PlainTranscripts -update-plain` writes them again.

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/muesli/termenv"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/git"
	"mint/internal/presenter"
	"mint/internal/presenter/frametest"
	"mint/internal/publish"
	"mint/internal/record"
	"mint/internal/release"
	"mint/internal/runner"
	"mint/internal/version"
)

var updatePlain = flag.Bool("update-plain", false, "write the plain transcripts of the regenerate runs again")

const (
	regenRailTag   = "v1.4.0"
	regenRailKey   = "1.4.0"
	ghBadGateway   = "HTTP 502: Bad Gateway (https://api.github.com/repos/acme/pigeon/releases/1)\n"
	tagAnnotation  = "Replay a delivery from the dashboard.\n"
	regenStaleBody = "Retries.\n"
)

// regenWorld is pigeon regenerating its notes: its root at ~/Code/pigeon under a home
// of its own, a CHANGELOG.md with a section for every version, git and gh scripted,
// the GitHub release driver over them, and the deps of a run.
type regenWorld struct {
	root      string
	f         *runner.FakeRunner
	cfg       config.Config
	publisher publish.Publisher
	deps      engine.ReleaseDeps
}

// regenRun is one regenerate run, drawn on the rail and in plain: its answers, a line
// each, whether it runs under -y, whether the terminal echoes what is typed, whether
// it ends in an abort, and the run itself.
type regenRun struct {
	answers string
	yes     bool
	echo    bool
	aborted bool
	run     func(t *testing.T, w *regenWorld) error
}

// newRegenWorld is pigeon with its commands scripted, its run drawing on p.
func newRegenWorld(t *testing.T, p presenter.Presenter, yes bool) *regenWorld {
	t.Helper()
	home := t.TempDir()
	root := filepath.Join(home, "Code", "pigeon")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	seedChangelog(t, root, pigeonChangelog())
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	f := runner.NewFakeRunner()
	mut := git.NewMutator(f)
	return &regenWorld{
		root:      root,
		f:         f,
		cfg:       cfg,
		publisher: publish.NewGitHubPublisher(f),
		deps: engine.ReleaseDeps{
			Presenter:        p,
			Runner:           f,
			Mutator:          mut,
			Releaser:         release.NewReleaser(mut),
			Yes:              yes,
			StdinInteractive: true,
			MintVersion:      "0.1.2",
			Home:             home,
			Clock:            (&steppingClock{now: railEpoch, step: 700 * time.Millisecond}).Now,
		},
	}
}

// pigeonVersions are pigeon's releases, oldest first.
var pigeonVersions = []string{"v1.0.0", "v1.1.0", "v1.2.0", "v1.2.1", "v1.3.0", "v1.3.2", "v1.4.0"}

// pigeonChangelog is pigeon's CHANGELOG.md: a terse section for every release.
func pigeonChangelog() string {
	var b strings.Builder
	b.WriteString(kacPreamble)
	for i := len(pigeonVersions) - 1; i >= 0; i-- {
		fmt.Fprintf(&b, "\n## [%s] - 2026-0%d-01\n\n%s", strings.TrimPrefix(pigeonVersions[i], "v"), i+1, regenStaleBody)
	}
	return b.String()
}

// resolutions are the first n of pigeon's releases, oldest first, as --all resolves
// them.
func resolutions(n int) []version.Resolution {
	res := make([]version.Resolution, n)
	for i, tag := range pigeonVersions[:n] {
		res[i] = version.Resolution{Tag: tag, FirstRelease: i == 0}
		if i > 0 {
			res[i].PreviousTag = pigeonVersions[i-1]
		}
	}
	return res
}

// preflight scripts the changelog's preflight passing: a clean tree on main, in sync
// with origin.
func (w *regenWorld) preflight() {
	w.f.SeedSequence("git",
		ScriptedOut(""),     // status --porcelain
		ScriptedOut("main"), // rev-parse --abbrev-ref HEAD
		ScriptedOut(""),     // fetch
		ScriptedOut("0\t0"), // rev-list --left-right --count
	)
}

// changelogWritten scripts a single version's changelog commit, from the starting
// HEAD's capture to the push, push being the push's outcome.
func (w *regenWorld) changelogWritten(push runner.ScriptedCall) {
	w.f.SeedSequence("git",
		ScriptedOut(startingSHA),  // rev-parse HEAD
		ScriptedOut("2026-07-01"), // for-each-ref creatordate
		ScriptedOut(""),           // add CHANGELOG.md
		ScriptedOut(""),           // commit
		push,                      // push origin HEAD
	)
}

// gh scripts every gh call going through: signed in, each release found and edited.
func (w *regenWorld) gh() {
	w.f.Seed("gh", runner.Result{}, nil)
}

// single is the request of a regenerate of v1.4.0, source and target as given,
// written by the AI as railNotes.
func (w *regenWorld) single(source engine.OptionalRegenerateSource, target engine.OptionalRegenerateTarget) engine.RegenerateRunRequest {
	return engine.RegenerateRunRequest{
		Source:           source,
		Target:           target,
		Tag:              regenRailTag,
		VersionKey:       regenRailKey,
		DiffRange:        "v1.3.2.." + regenRailTag,
		Project:          "pigeon",
		ReleaseBranch:    "main",
		ChangelogEnabled: true,
		Config:           w.cfg,
		ProduceBody: func(ctx context.Context, source engine.RegenerateSource, _ engine.NotesSinks) (string, error) {
			if source == engine.RegenerateSourceTag {
				return engine.RequireTagBody(ctx, w.f, regenRailTag)
			}
			return railNotes, nil
		},
	}
}

// regenerate runs req.
func (w *regenWorld) regenerate(t *testing.T, req engine.RegenerateRunRequest) error {
	t.Helper()
	return engine.RegenerateRun(t.Context(), w.deps, w.publisher, w.root, req)
}

// versionNotes writes each version's notes as the fresh source does: the first
// release's default, no notes at all for the tags in empty, and a line of its own
// for the rest.
func versionNotes(empty ...string) func(context.Context, engine.RegenerateSource, version.Resolution, string, engine.NotesSinks) (string, error) {
	return func(_ context.Context, _ engine.RegenerateSource, res version.Resolution, _ string, _ engine.NotesSinks) (string, error) {
		switch {
		case res.FirstRelease:
			return record.FirstReleaseBody, nil
		case slices.Contains(empty, res.Tag):
			return "", fmt.Errorf("regenerating fresh notes for %s: %w", res.Tag, &ai.GenerationError{})
		}
		return "What changed in " + res.Tag + ", in a line.\n", nil
	}
}

// tallNotes are versionNotes but for tag's, too tall for the terminal to hold.
func tallNotes(tag string) func(context.Context, engine.RegenerateSource, version.Resolution, string, engine.NotesSinks) (string, error) {
	short := versionNotes()
	return func(ctx context.Context, source engine.RegenerateSource, res version.Resolution, pre string, sinks engine.NotesSinks) (string, error) {
		if res.Tag != tag {
			return short(ctx, source, res, pre, sinks)
		}
		var b strings.Builder
		b.WriteString("Deliveries are batched per endpoint now.\n\n## ✨ Added\n")
		for i := 1; i <= railRows; i++ {
			fmt.Fprintf(&b, "- Batched delivery change %d.\n", i)
		}
		return b.String(), nil
	}
}

// all runs --all over the first n of pigeon's releases from fresh notes to target,
// their notes written by notes, and written again at a review's r by the run's
// Regenerator, where it has one.
func (w *regenWorld) all(t *testing.T, n int, target engine.RegenerateTarget, notes func(context.Context, engine.RegenerateSource, version.Resolution, string, engine.NotesSinks) (string, error)) error {
	t.Helper()
	return engine.RegenerateAllRun(t.Context(), w.deps, w.root, engine.RegenerateAllRunRequest{
		Source:           engine.SourceOf(engine.RegenerateSourceFresh),
		Target:           engine.TargetOf(target),
		ChangelogEnabled: true,
		Project:          "pigeon",
		Prepare: func(_ context.Context, source engine.RegenerateSource, target engine.RegenerateTarget) (engine.BatchRegenerateRequest, publish.Publisher, error) {
			return engine.BatchRegenerateRequest{
				Source:        source,
				Versions:      resolutions(n),
				Project:       "pigeon",
				TagPrefix:     "v",
				Yes:           w.deps.Yes,
				Target:        target,
				ReleaseBranch: "main",
				ProduceBody:   notes,
				ProduceRegenerator: func(engine.RegenerateSource, version.Resolution, engine.RegenRun) engine.Regenerator {
					return w.deps.Regenerator
				},
			}, w.publisher, nil
		},
	})
}

// rebuilt scripts the --all changelog rebuild of the first n releases, those in
// skipped kept as they were, from the starting HEAD's capture to the push.
func (w *regenWorld) rebuilt(n int, skipped ...string) {
	w.f.SeedSequence("git", ScriptedOut(startingSHA)) // rev-parse HEAD
	for _, tag := range pigeonVersions[:n] {
		if !slices.Contains(skipped, tag) {
			w.f.SeedSequence("git", ScriptedOut("2026-07-01")) // for-each-ref creatordate
		}
	}
	w.f.SeedSequence("git",
		ScriptedOut(""), // add CHANGELOG.md
		ScriptedOut(""), // commit
		ScriptedOut(""), // push origin HEAD
	)
}

var regenRuns = map[string]regenRun{
	// A version regenerated from fresh notes to its changelog and GitHub release, its
	// review answered y.
	"regenerate-accepted-100": {answers: "y\n", run: func(t *testing.T, w *regenWorld) error {
		w.preflight()
		w.gh()
		w.changelogWritten(ScriptedOut(""))
		return w.regenerate(t, w.single(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetBoth)))
	}},
	// The review answered r, the notes written again in one call, then y.
	"regenerate-regenerated-100": {answers: "r\n\ny\n", run: func(t *testing.T, w *regenWorld) error {
		w.deps.Regenerator = railRegenerator(railRegenNotes)
		w.gh()
		return w.regenerate(t, w.single(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease)))
	}},
	// No source or target given: both asked, typed at the terminal, then the tag's
	// notes confirmed.
	"regenerate-asked-100": {answers: "tag\nrelease\ny\n", echo: true, run: func(t *testing.T, w *regenWorld) error {
		w.gh()
		w.f.SeedSequence("git", ScriptedOut(tagAnnotation)) // for-each-ref contents:body
		return w.regenerate(t, w.single(engine.SourceUnset(), engine.TargetUnset()))
	}},
	// No source or target given: both asked, Enter typed at each for its default, then
	// the notes accepted.
	"regenerate-asked-defaults-100": {answers: "\n\ny\n", echo: true, run: func(t *testing.T, w *regenWorld) error {
		w.gh()
		return w.regenerate(t, w.single(engine.SourceUnset(), engine.TargetUnset()))
	}},
	// The changelog commit's push rejected under -y: the commit is reset.
	"regenerate-push-rejected-100": {yes: true, aborted: true, run: func(t *testing.T, w *regenWorld) error {
		w.preflight()
		w.changelogWritten(gitRejected(pushRejectedStderr))
		w.f.SeedSequence("git", ScriptedOut("")) // reset --hard startsha
		return w.regenerate(t, w.single(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetChangelog)))
	}},
	// --all under -y, every version's GitHub release written and CHANGELOG.md rebuilt,
	// v1.2.0's notes coming back empty, so it is skipped.
	"regenerate-all-skipped-100": {yes: true, run: func(t *testing.T, w *regenWorld) error {
		w.preflight()
		w.gh()
		w.rebuilt(len(pigeonVersions), "v1.2.0")
		return w.all(t, len(pigeonVersions), engine.RegenerateTargetBoth, versionNotes("v1.2.0"))
	}},
	// --all under -y whose third GitHub release can't be written.
	"regenerate-all-failed-100": {yes: true, aborted: true, run: func(t *testing.T, w *regenWorld) error {
		w.f.SeedSequence("gh",
			ScriptedOut(""), // auth status
			ScriptedOut(""), // release view v1.0.0
			ScriptedOut(""), // release edit v1.0.0
			ScriptedOut(""), // release view v1.1.0
			ScriptedOut(""), // release edit v1.1.0
			ScriptedOut(""), // release view v1.2.0
			gitFailing(ghBadGateway),
		)
		return w.all(t, 4, engine.RegenerateTargetRelease, versionNotes())
	}},
	// --all at a terminal, v1.1.0's notes written again at its review's r, the rest
	// answered y.
	"regenerate-all-regenerated-100": {answers: "y\nr\n\ny\ny\n", run: func(t *testing.T, w *regenWorld) error {
		w.deps.Regenerator = railRegenerator(railRegenNotes)
		w.gh()
		return w.all(t, 3, engine.RegenerateTargetRelease, versionNotes())
	}},
	// --all at a terminal, v1.1.0's notes too tall for the terminal, each answered y.
	"regenerate-all-tall-100": {answers: "y\ny\ny\n", run: func(t *testing.T, w *regenWorld) error {
		w.gh()
		return w.all(t, 3, engine.RegenerateTargetRelease, tallNotes("v1.1.0"))
	}},
	// --all at a terminal, v1.1.0's review answered r, the call writing its notes again
	// failing.
	"regenerate-all-regen-failed-100": {answers: "y\nr\n\n", aborted: true, run: func(t *testing.T, w *regenWorld) error {
		w.deps.Regenerator = &fakeRegenerator{err: fmt.Errorf("regenerating notes: %w", &ai.GenerationError{})}
		w.gh()
		return w.all(t, 3, engine.RegenerateTargetRelease, versionNotes())
	}},
	// --all at a terminal: each version's notes reviewed, answered y.
	"regenerate-all-attended-100": {answers: "y\ny\ny\n", run: func(t *testing.T, w *regenWorld) error {
		w.gh()
		return w.all(t, 3, engine.RegenerateTargetRelease, versionNotes())
	}},
}

// railRegenerate runs r on the rail, on a terminal set up as the release frames' is,
// then ends the drawing as the cmd layer does, and returns the run's error and
// everything on the terminal. A run whose terminal echoes has its answers typed at
// it: the terminal shows each line as it is read.
func railRegenerate(t *testing.T, r regenRun) (string, error) {
	t.Helper()
	term := frametest.NewTerminal(railCols, railRows)
	var in io.Reader = strings.NewReader(r.answers)
	if r.echo {
		in = &typedInput{lines: strings.SplitAfter(r.answers, "\n"), term: term}
	}
	p := presenter.NewRailPresenter(term,
		presenter.WithRailInput(in),
		presenter.WithRailEcho(r.echo),
		presenter.WithRailGating(r.yes, true),
		presenter.WithRailProfile(termenv.Ascii),
		presenter.WithRailSize(func() (int, int) { return railCols, railRows }),
		presenter.WithRailTicker(stillTicker),
		presenter.WithRailClock(func() time.Time { return railEpoch }),
	)
	err := r.run(t, newRegenWorld(t, p, r.yes))
	p.Close()
	if terr := term.Err(); terr != nil {
		t.Fatalf("the terminal was written what it can't apply: %v", terr)
	}
	return term.ANSI(), err
}

// typedInput is what is typed at a terminal in cooked mode: each read is one line,
// which the terminal echoes where its cursor stands.
type typedInput struct {
	lines []string
	term  io.Writer
}

func (in *typedInput) Read(p []byte) (int, error) {
	if len(in.lines) == 0 || in.lines[0] == "" {
		return 0, io.EOF
	}
	line := in.lines[0]
	in.lines = in.lines[1:]
	if _, err := io.WriteString(in.term, strings.ReplaceAll(line, "\n", "\r\n")); err != nil {
		return 0, err
	}
	return copy(p, line), nil
}

// regenerateFrames draws every regenerate run on the rail, by name.
func regenerateFrames(t *testing.T) map[string]string {
	t.Helper()
	drawn := map[string]string{}
	for name, r := range regenRuns {
		screen, err := railRegenerate(t, r)
		if aborted := err != nil; aborted != r.aborted {
			t.Errorf("%s: the run returned %v, want aborted %v", name, err, r.aborted)
		}
		drawn[name] = screen
	}
	return drawn
}

// plainRegenerate runs r in plain, and returns its stdout, then its stderr under a
// rule of its own.
func plainRegenerate(t *testing.T, r regenRun) (string, error) {
	t.Helper()
	if r.yes {
		p, read := plainUnderYes(t)
		err := r.run(t, newRegenWorld(t, p, true))
		return read(), err
	}
	var out, errOut bytes.Buffer
	p := presenter.NewPlainPresenterWithInput(&out, &errOut, strings.NewReader(r.answers))
	err := r.run(t, newRegenWorld(t, p, false))
	return transcript(out.String(), errOut.String()), err
}

func transcript(stdout, stderr string) string {
	return stdout + "--- stderr ---\n" + stderr
}

// plainUnderYes is plain under -y as the startup seam builds it, over files, and what
// reads back the transcript it wrote.
func plainUnderYes(t *testing.T) (presenter.Presenter, func() string) {
	t.Helper()
	dir := t.TempDir()
	open := func(name string) *os.File {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = f.Close() })
		return f
	}
	stdout, stderr, stdin := open("stdout"), open("stderr"), open("stdin")
	read := func() string {
		var texts [2]string
		for i, f := range []*os.File{stdout, stderr} {
			data, err := os.ReadFile(f.Name())
			if err != nil {
				t.Fatal(err)
			}
			texts[i] = string(data)
		}
		return transcript(texts[0], texts[1])
	}
	return presenter.NewForStartup(true, true, stdout, stderr, stdin), read
}

func TestRegenerate_PlainTranscripts(t *testing.T) {
	t.Parallel()

	for name, r := range regenRuns {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			transcript, err := plainRegenerate(t, r)
			if aborted := err != nil; aborted != r.aborted {
				t.Errorf("the run returned %v, want aborted %v", err, r.aborted)
			}
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
		})
	}
}
