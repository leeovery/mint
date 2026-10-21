package engine_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/record"
	"mint/internal/runner"
)

const regenNotesStaleChangelog = kacPreamble + "\n## [1.4.0] - 2024-02-15\n\nStale body.\n"

// noBodyProduction fails the test if the run asks for a body from fresh, tag or
// release: a supplied body takes the place of every source's production.
func noBodyProduction(t *testing.T) func(context.Context, engine.RegenerateSource, engine.NotesSinks) (string, error) {
	t.Helper()
	return func(context.Context, engine.RegenerateSource, engine.NotesSinks) (string, error) {
		t.Errorf("ProduceBody was called; a supplied body produces nothing")
		return regenRunFreshBody, nil
	}
}

// notesFileRunReq reads path as mint does at startup and builds the run request
// that carries its body.
func notesFileRunReq(t *testing.T, path string, target engine.RegenerateTarget) engine.RegenerateRunRequest {
	t.Helper()
	supplied, err := engine.ReadNotesFile(&presentertest.RecordingPresenter{}, path, true)
	if err != nil {
		t.Fatalf("reading --notes-file %s: %v", path, err)
	}
	req := runReq(engine.SourceUnset(), engine.TargetOf(target))
	req.ReleaseBranch = regenReleaseBranch
	req.NotesFile = path
	req.Supplied = supplied
	req.ProduceBody = noBodyProduction(t)
	return req
}

// regenChangelogGates scripts the clean-tree, on-branch, fetch and remote-sync reads
// a changelog-writing regenerate makes. status is the clean-tree probe's output.
func regenChangelogGates(status string) []runner.ScriptedCall {
	return []runner.ScriptedCall{
		ScriptedOut(status),             // status --porcelain
		ScriptedOut(regenReleaseBranch), // rev-parse --abbrev-ref HEAD
		ScriptedOut(""),                 // fetch --tags
		ScriptedOut("0\t1"),             // rev-list left-right count
	}
}

// regenChangelogWrite scripts the changelog write up to its commit.
func regenChangelogWrite() []runner.ScriptedCall {
	return []runner.ScriptedCall{
		ScriptedOut("startHEAD"),          // rev-parse HEAD
		ScriptedOut(regenWriteHistorical), // for-each-ref creatordate:short
		ScriptedOut(""),                   // -C root add CHANGELOG.md
	}
}

// seedRegenNotesFileGit scripts a supplied-body regenerate of a changelog-writing
// target whose notes file lies outside the repo.
func seedRegenNotesFileGit(f *runner.FakeRunner) {
	f.SeedSequence("git", regenChangelogGates("")...)
	f.SeedSequence("git", regenChangelogWrite()...)
	f.SeedSequence("git",
		ScriptedOut(""), // -C root commit -m
		ScriptedOut(""), // push origin HEAD
	)
	f.Seed("gh", runner.Result{}, nil)
}

func regenSectionBody(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatalf("reading CHANGELOG.md: %v", err)
	}
	_, after, found := strings.Cut(string(data), "## [1.4.0] - "+regenWriteHistorical)
	if !found {
		t.Fatalf("CHANGELOG.md has no 1.4.0 section; got:\n%s", data)
	}
	return strings.TrimSuffix(strings.TrimPrefix(after, "\n\n"), "\n")
}

func assertProviderBody(t *testing.T, pub *fakePublisher, want string) {
	t.Helper()
	if len(pub.dispatched) != 1 {
		t.Fatalf("provider dispatched %d times, want exactly 1", len(pub.dispatched))
	}
	if got := pub.dispatched[0].body; got != want {
		t.Errorf("provider release body = %q, want %q", got, want)
	}
}

func assertNoChangelogCommit(t *testing.T, f *runner.FakeRunner, root string) {
	t.Helper()
	for _, inv := range f.Invocations() {
		if inv.Name == "git" && (slices.Contains(inv.Args, "commit") || slices.Contains(inv.Args, "push")) {
			t.Errorf("git step %s ran; this run writes no changelog", commandLine(inv))
		}
	}
	if got := readChangelogFile(t, root); got != regenNotesStaleChangelog {
		t.Errorf("CHANGELOG.md = %q, want it untouched", got)
	}
}

func TestRegenerateRun_NotesFile_BothTargetWritesTheFileWithNoSourceQuestionOrAI(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedChangelog(t, root, regenNotesStaleChangelog)
	path := writeNotesFile(t, t.TempDir(), "notes.md", suppliedBody)
	f := runner.NewFakeRunner()
	seedRegenNotesFileGit(f)
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, true, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, root, notesFileRunReq(t, path, engine.RegenerateTargetBoth)); err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	if got, want := gateSubjects(rec), []string{"notes"}; !slices.Equal(got, want) {
		t.Errorf("gate subjects = %v, want %v (no source question)", got, want)
	}
	if claudeInvoked(f) {
		t.Errorf("the AI was invoked; a supplied body makes no AI call")
	}
	if got := regenSectionBody(t, root); got != suppliedBody {
		t.Errorf("CHANGELOG body = %q, want %q", got, suppliedBody)
	}
	assertProviderBody(t, pub, suppliedBody)
}

func TestRegenerateRun_NotesFile_TargetChoosesTheSurfacesWritten(t *testing.T) {
	t.Parallel()

	t.Run("release writes only the provider release", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		seedChangelog(t, root, regenNotesStaleChangelog)
		path := writeNotesFile(t, t.TempDir(), "notes.md", suppliedBody)
		f := runner.NewFakeRunner()
		f.Seed("gh", runner.Result{}, nil)
		pub := newFakePublisher()
		pub.seedExists(regenRunTag, true, nil)
		rec := &presentertest.RecordingPresenter{}

		if err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, root, notesFileRunReq(t, path, engine.RegenerateTargetRelease)); err != nil {
			t.Fatalf("RegenerateRun returned unexpected error: %v", err)
		}

		assertProviderBody(t, pub, suppliedBody)
		assertNoChangelogCommit(t, f, root)
	})

	t.Run("changelog writes only the changelog", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		seedChangelog(t, root, regenNotesStaleChangelog)
		path := writeNotesFile(t, t.TempDir(), "notes.md", suppliedBody)
		f := runner.NewFakeRunner()
		seedRegenNotesFileGit(f)
		pub := newFakePublisher()
		rec := &presentertest.RecordingPresenter{}

		if err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, root, notesFileRunReq(t, path, engine.RegenerateTargetChangelog)); err != nil {
			t.Fatalf("RegenerateRun returned unexpected error: %v", err)
		}

		if got := regenSectionBody(t, root); got != suppliedBody {
			t.Errorf("CHANGELOG body = %q, want %q", got, suppliedBody)
		}
		if !invokedWith(f, "git", "push", "origin", "HEAD") {
			t.Errorf("the changelog commit was not pushed; got %v", commandLines(f.Invocations()))
		}
		if len(pub.dispatched) != 0 {
			t.Errorf("provider release written %d times; a changelog target writes none", len(pub.dispatched))
		}
	})
}

func TestRegenerateRun_NotesFile_OldestVersionShipsTheFileNotInitialRelease(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedChangelog(t, root, regenNotesStaleChangelog)
	path := writeNotesFile(t, t.TempDir(), "notes.md", suppliedBody)
	f := runner.NewFakeRunner()
	seedRegenNotesFileGit(f)
	pub := newFakePublisher()
	rec := &presentertest.RecordingPresenter{}
	req := notesFileRunReq(t, path, engine.RegenerateTargetBoth)
	req.ProduceBody = func(context.Context, engine.RegenerateSource, engine.NotesSinks) (string, error) {
		return record.FirstReleaseBody, nil
	}

	if err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, root, req); err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	if got := regenSectionBody(t, root); got != suppliedBody {
		t.Errorf("CHANGELOG body = %q, want the supplied body %q", got, suppliedBody)
	}
	assertProviderBody(t, pub, suppliedBody)
}

func TestRegenerateRun_NotesFile_PlanNamesNoOtherSource(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := writeNotesFile(t, t.TempDir(), "notes.md", suppliedBody)
	f := runner.NewFakeRunner()
	f.Seed("gh", runner.Result{}, nil)
	pub := newFakePublisher()
	rec := &presentertest.RecordingPresenter{}

	if err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, root, notesFileRunReq(t, path, engine.RegenerateTargetRelease)); err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	var plan presenter.Plan
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindShowPlan {
			plan = ev.ShowPlan
		}
	}
	want := []presenter.PlanStep{{Verb: "source", Target: "notes file"}, {Verb: "release", Target: regenRunTag}}
	if !slices.Equal(plan.Steps, want) {
		t.Errorf("plan steps = %+v, want %+v", plan.Steps, want)
	}
}

func TestRegenerateRun_NotesFile_ReviewGateOffersAcceptAbortEditWithoutRegenerate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := writeNotesFile(t, t.TempDir(), "notes.md", suppliedBody)
	f := runner.NewFakeRunner()
	f.Seed("gh", runner.Result{}, nil)
	pub := newFakePublisher()
	rec := &presentertest.RecordingPresenter{}

	if err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, root, notesFileRunReq(t, path, engine.RegenerateTargetRelease)); err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	gate := singlePromptGate(t, rec)
	if gate.Question != "Use these notes?" {
		t.Errorf("review gate Question = %q, want regenerate's %q", gate.Question, "Use these notes?")
	}
	want := []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo, presenter.ChoiceEdit}
	if got := gate.Keys(); !slices.Equal(got, want) {
		t.Errorf("review gate choices = %v, want %v (accept, abort, edit; no r)", got, want)
	}
	if got := lastNotesBody(t, rec); got != suppliedBody {
		t.Errorf("notes shown at the gate = %q, want the supplied body %q", got, suppliedBody)
	}
}

func TestRegenerateRun_NotesFile_EditChangesTheBodyWrittenNotTheFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedChangelog(t, root, regenNotesStaleChangelog)
	path := writeNotesFile(t, t.TempDir(), "notes.md", suppliedBody)
	f := runner.NewFakeRunner()
	seedRegenNotesFileGit(f)
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, true, nil)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes}}
	editor := &fakeEditor{edited: editedBody}
	deps := freshRunDeps(rec, f)
	deps.Editor = editor

	if err := engine.RegenerateRun(t.Context(), deps, pub, root, notesFileRunReq(t, path, engine.RegenerateTargetBoth)); err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	if editor.gotCurrent != suppliedBody {
		t.Errorf("editor opened on %q, want the supplied body %q", editor.gotCurrent, suppliedBody)
	}
	if got := regenSectionBody(t, root); got != editedBody {
		t.Errorf("CHANGELOG body = %q, want the edited body %q", got, editedBody)
	}
	assertProviderBody(t, pub, editedBody)
	assertFileUnchanged(t, path, suppliedBody)
}

func TestRegenerateRun_NotesFile_AbortAtTheReviewGateWritesNothing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedChangelog(t, root, regenNotesStaleChangelog)
	path := writeNotesFile(t, t.TempDir(), "notes.md", suppliedBody)
	f := runner.NewFakeRunner()
	f.SeedSequence("git", regenChangelogGates("")...)
	f.Seed("gh", runner.Result{}, nil)
	pub := newFakePublisher()
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceNo}}

	err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, root, notesFileRunReq(t, path, engine.RegenerateTargetBoth))

	assertAbortNonZero(t, err)
	assertNoChangelogCommit(t, f, root)
	if len(pub.dispatched) != 0 {
		t.Errorf("provider release written %d times after an abort, want none", len(pub.dispatched))
	}
}

func TestRegenerateRun_NotesFile_YesAutoAcceptsTheReviewGateInsideThePresenter(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedChangelog(t, root, regenNotesStaleChangelog)
	path := writeNotesFile(t, t.TempDir(), "notes.md", suppliedBody)
	f := runner.NewFakeRunner()
	seedRegenNotesFileGit(f)
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, true, nil)

	streams := t.TempDir()
	out := createFile(t, filepath.Join(streams, "stdout"))
	errOut := createFile(t, filepath.Join(streams, "stderr"))
	in := createFile(t, filepath.Join(streams, "stdin"))
	deps := freshRunDeps(&presentertest.RecordingPresenter{}, f)
	deps.Presenter = presenter.NewForStartup(true, true, out, errOut, in)
	req := notesFileRunReq(t, path, engine.RegenerateTargetBoth)

	if err := engine.RegenerateRun(t.Context(), deps, pub, root, req); err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	rendered, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatalf("reading rendered output: %v", err)
	}
	lines := strings.Split(string(rendered), "\n")
	if !slices.Contains(lines, "notes: accepted (-y)") {
		t.Errorf("rendered output has no %q line; got:\n%s", "notes: accepted (-y)", rendered)
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "source:") {
			t.Errorf("rendered output carries a source answer %q; no source question is asked", line)
		}
	}
	if got := regenSectionBody(t, root); got != suppliedBody {
		t.Errorf("CHANGELOG body = %q, want %q", got, suppliedBody)
	}
	assertProviderBody(t, pub, suppliedBody)
}

func TestRegenerateRun_NotesFile_InRepoFileIsKeptOutOfTheChangelogGitSteps(t *testing.T) {
	t.Parallel()

	const subject = "docs(changelog): regenerate notes for " + regenRunTag
	tests := []struct {
		name string
		// notesStaged is the staged-edit probe's output for the notes file.
		notesStaged string
	}{
		{name: "an untracked file"},
		{name: "a tracked file with unstaged edits"},
		{name: "a tracked file with staged edits", notesStaged: "notes.md\x00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			seedChangelog(t, root, regenNotesStaleChangelog)
			path := writeNotesFile(t, root, "notes.md", suppliedBody)
			f := runner.NewFakeRunner()
			f.SeedSequence("git", ScriptedOut("")) // ls-files --ignored (not ignored)
			f.SeedSequence("git", regenChangelogGates("")...)
			f.SeedSequence("git", regenChangelogWrite()...)
			f.SeedSequence("git", keepOutCommit(tt.notesStaged, stagedChangelog)...)
			f.SeedSequence("git", ScriptedOut("")) // push origin HEAD
			f.Seed("gh", runner.Result{}, nil)
			pub := newFakePublisher()
			pub.seedExists(regenRunTag, true, nil)
			rec := &presentertest.RecordingPresenter{}

			if err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, root, notesFileRunReq(t, path, engine.RegenerateTargetBoth)); err != nil {
				t.Fatalf("RegenerateRun returned unexpected error: %v", err)
			}

			assertGitArgs(t, f, 2, []string{"status", "--porcelain", "--", excludePathspec("notes.md")})
			if tt.notesStaged == "" {
				assertCommitAsBuilt(t, f, root, "notes.md", subject)
			} else {
				assertCommitKeepsNotesOut(t, f, root, "notes.md", subject, committedChangelog)
			}
			assertNotesFileOnlyExcluded(t, f, "notes.md")
			if resetIssued(f) {
				t.Errorf("a git reset ran after a successful regenerate")
			}
			assertFileUnchanged(t, path, suppliedBody)
			if got := regenSectionBody(t, root); got != suppliedBody {
				t.Errorf("CHANGELOG body = %q, want %q", got, suppliedBody)
			}
		})
	}
}

func TestRegenerateRun_NotesFile_AnotherDirtyPathStillFailsTheCleanTreeCheck(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedChangelog(t, root, regenNotesStaleChangelog)
	path := writeNotesFile(t, root, "notes.md", suppliedBody)
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(""),           // ls-files --ignored (not ignored)
		ScriptedOut(" M main.go"), // status --porcelain -- :(exclude)notes.md
	)
	pub := newFakePublisher()
	rec := &presentertest.RecordingPresenter{}

	err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, root, notesFileRunReq(t, path, engine.RegenerateTargetChangelog))

	assertAbortNonZero(t, err)
	if !recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("no StageFailed recorded; the dirty tree must fail the check")
	}
	assertNoChangelogCommit(t, f, root)
}

// rewritingStatusRunner rewrites a file when the clean-tree check runs, then
// delegates to the wrapped FakeRunner.
type rewritingStatusRunner struct {
	*runner.FakeRunner
	path    string
	rewrite string
}

func (r *rewritingStatusRunner) Run(ctx context.Context, name string, args ...string) (runner.Result, error) {
	if name == "git" && slices.Contains(args, "status") {
		if err := os.WriteFile(r.path, []byte(r.rewrite), 0o644); err != nil {
			return runner.Result{}, err
		}
	}
	return r.FakeRunner.Run(ctx, name, args...)
}

// Not parallel: t.Chdir sets the process working directory.
func TestRegenerateRun_NotesFile_RelativePathResolvesAgainstTheRunDirectoryAndIsReadAtStartup(t *testing.T) {
	root := t.TempDir()
	seedChangelog(t, root, regenNotesStaleChangelog)
	runDir := t.TempDir()
	path := writeNotesFile(t, runDir, "notes.md", suppliedBody)
	t.Chdir(runDir)
	f := runner.NewFakeRunner()
	seedRegenNotesFileGit(f)
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, true, nil)
	rec := &presentertest.RecordingPresenter{}
	deps := freshRunDeps(rec, f)
	const rewritten = "rewritten after startup\n"
	deps.Runner = &rewritingStatusRunner{FakeRunner: f, path: path, rewrite: rewritten}

	if err := engine.RegenerateRun(t.Context(), deps, pub, root, notesFileRunReq(t, "notes.md", engine.RegenerateTargetBoth)); err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	assertFileUnchanged(t, path, rewritten)
	if got := regenSectionBody(t, root); got != suppliedBody {
		t.Errorf("CHANGELOG body = %q, want the body read at startup %q", got, suppliedBody)
	}
	assertProviderBody(t, pub, suppliedBody)
}
