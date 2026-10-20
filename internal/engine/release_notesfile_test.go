package engine_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// suppliedBody is the distinctive finished body a --notes-file test hands mint, so
// the sinks can be checked for it byte for byte.
const suppliedBody = "TL;DR: hand-written notes, supplied finished.\n\n" +
	"## ✨ Added\n" +
	"- **Notes file** shipped exactly as written  \n\n" +
	"trailing spaces and blank lines survive\n\n"

// writeNotesFile writes body to name under dir and returns the file's path.
func writeNotesFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

func notesFileOptions(path string) engine.ReleaseOptions {
	return engine.ReleaseOptions{Bump: version.BumpPatch, Now: fixedClock, NotesFile: path}
}

// seedPriorTagNotesFileGit scripts a prior-tag release whose body is supplied: the
// read gates, then straight to the record/tag/push tail. No notes git call is seeded,
// so any diff assembly would consume a mutation-tail outcome and derail the run.
func seedPriorTagNotesFileGit(f *runner.FakeRunner, root string) {
	seedPriorTagReadGates(f, root, "main")
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
}

// gitDiffInvoked reports whether any `git diff …` ran — the diff assembly every
// AI-path step (exclusion, size check, chunking, generation) starts from.
func gitDiffInvoked(f *runner.FakeRunner) bool {
	for _, inv := range f.Invocations() {
		if inv.Name == "git" && slices.Contains(inv.Args, "diff") {
			return true
		}
	}
	return false
}

func assertSuppliedBodyShipped(t *testing.T, f *runner.FakeRunner, root, tag, versionKey, want string) {
	t.Helper()
	if got := tagAnnotationBody(t, f, tag); got != want {
		t.Errorf("tag annotation body = %q, want %q", got, want)
	}
	if got := changelogSectionBody(t, root, versionKey); got != want {
		t.Errorf("CHANGELOG body = %q, want %q", got, want)
	}
	if got := stdinOf(t, f, "gh", "release", "create", tag, "--title", tag, "--notes-file", "-", "--verify-tag"); got != want {
		t.Errorf("provider release body = %q, want %q", got, want)
	}
}

func assertNoAIWork(t *testing.T, f *runner.FakeRunner) {
	t.Helper()
	if claudeInvoked(f) {
		t.Errorf("the AI was invoked; a supplied body makes no AI call")
	}
	if gitDiffInvoked(f) {
		t.Errorf("a diff was assembled; a supplied body skips the AI path entirely; got %v", commandLines(f.Invocations()))
	}
}

func TestRelease_NotesFile_ShipsFileContentsByteIdenticalWithNoAICall(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := writeNotesFile(t, t.TempDir(), "notes.md", suppliedBody)
	f := runner.NewFakeRunner()
	seedPriorTagNotesFileGit(f, root)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), notesFileOptions(path)); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertNoAIWork(t, f)
	assertSuppliedBodyShipped(t, f, root, nextTag, "1.2.4", suppliedBody)
}

func TestRelease_NotesFile_WinsOverEveryOtherBodySource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		config     string
		firstRel   bool
		seedCache  bool
		tag        string
		versionKey string
	}{
		{name: "first release ships it instead of Initial release.", firstRel: true, tag: "v0.0.1", versionKey: "0.0.1"},
		{name: "prior tag ships it without assembling the diff an empty-diff body needs", tag: nextTag, versionKey: "1.2.4"},
		{name: "on_notes_failure fallback never applies", config: "[release]\non_notes_failure = \"fallback\"\n", tag: nextTag, versionKey: "1.2.4"},
		{name: "a live dry-run cache entry is never offered", seedCache: true, tag: nextTag, versionKey: "1.2.4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			if tt.config != "" {
				writeConfig(t, root, tt.config)
			}
			path := writeNotesFile(t, t.TempDir(), "notes.md", suppliedBody)
			f := runner.NewFakeRunner()
			if tt.firstRel {
				seedHappyGit(f, root, "main", tt.tag)
				f.Seed("gh", runner.Result{}, nil)
			} else {
				seedPriorTagNotesFileGit(f, root)
			}
			rec := &presentertest.RecordingPresenter{}
			deps := newDeps(rec, f)
			if tt.seedCache {
				cacheBase := t.TempDir()
				seedCacheEntry(t, cacheBase, root, priorTagDiff, tt.versionKey, cachedReuseBody)
				deps = newDepsWithRealRunCache(rec, f, cacheBase, func() time.Time { return realRunClock })
			}

			if err := engine.Release(t.Context(), deps, notesFileOptions(path)); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			assertNoAIWork(t, f)
			assertSuppliedBodyShipped(t, f, root, tt.tag, tt.versionKey, suppliedBody)
			if reuseConfirmPrompted(rec) {
				t.Errorf("the cached-notes question was asked; a supplied body never reaches cache reuse")
			}
		})
	}
}

func TestRelease_NotesFile_ReviewGateOffersAcceptAbortEditWithoutRegenerate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := writeNotesFile(t, t.TempDir(), "notes.md", suppliedBody)
	f := runner.NewFakeRunner()
	seedPriorTagNotesFileGit(f, root)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), notesFileOptions(path)); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	gate := promptGate(t, rec)
	if gate.Question != "Release with these notes?" {
		t.Errorf("review gate Question = %q, want %q", gate.Question, "Release with these notes?")
	}
	var keys []presenter.Choice
	for _, c := range gate.Choices {
		keys = append(keys, c.Key)
	}
	want := []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo, presenter.ChoiceEdit}
	if !slices.Equal(keys, want) {
		t.Errorf("review gate choices = %v, want %v (no r)", keys, want)
	}
	if got := lastNotesBody(t, rec); got != suppliedBody {
		t.Errorf("notes shown at the gate = %q, want the supplied body %q", got, suppliedBody)
	}
}

func TestRelease_NotesFile_EditShipsSavedTextAndLeavesFileUnchanged(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := writeNotesFile(t, t.TempDir(), "notes.md", suppliedBody)
	f := runner.NewFakeRunner()
	seedPriorTagNotesFileGit(f, root)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{
		presenter.ChoiceYes,  // version confirmation
		presenter.ChoiceEdit, // review gate: edit
		presenter.ChoiceYes,  // review gate: accept the edited body
	}}
	editor := &fakeEditor{edited: editedBody}
	deps := newDeps(rec, f)
	deps.Editor = editor

	if err := engine.Release(t.Context(), deps, notesFileOptions(path)); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if editor.gotCurrent != suppliedBody {
		t.Errorf("editor opened on %q, want the supplied body %q", editor.gotCurrent, suppliedBody)
	}
	assertSuppliedBodyShipped(t, f, root, nextTag, "1.2.4", editedBody)
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the notes file back: %v", err)
	}
	if string(onDisk) != suppliedBody {
		t.Errorf("notes file on disk = %q, want it unchanged %q", onDisk, suppliedBody)
	}
}

func TestRelease_NotesFile_AbortAtReviewGateEndsBeforeAnyTag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := writeNotesFile(t, t.TempDir(), "notes.md", suppliedBody)
	f := runner.NewFakeRunner()
	seedPriorTagNotesFileGit(f, root)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{
		presenter.ChoiceYes, // version confirmation
		presenter.ChoiceNo,  // review gate: abort
	}}

	err := engine.Release(t.Context(), newDeps(rec, f), notesFileOptions(path))

	assertAbortNonZero(t, err)
	if got := lastNotesBody(t, rec); got != suppliedBody {
		t.Errorf("notes shown at the gate = %q, want the supplied body %q", got, suppliedBody)
	}
	promptGate(t, rec)
	if recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("a StageFailed was recorded; an abort at the review gate surfaces none")
	}
	assertNoMutation(t, f)
	for _, inv := range f.Invocations() {
		if inv.Name == "git" && slices.Contains(inv.Args, "commit") {
			t.Errorf("a commit was made after the abort: %s", commandLine(inv))
		}
	}
}

func TestRelease_NotesFile_YesAutoAcceptsReviewGateInsideThePresenter(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := writeNotesFile(t, t.TempDir(), "notes.md", suppliedBody)
	f := runner.NewFakeRunner()
	seedPriorTagNotesFileGit(f, root)

	streams := t.TempDir()
	out := createFile(t, filepath.Join(streams, "stdout"))
	errOut := createFile(t, filepath.Join(streams, "stderr"))
	in := createFile(t, filepath.Join(streams, "stdin"))
	deps := newDeps(&presentertest.RecordingPresenter{}, f)
	deps.Presenter = presenter.NewForStartup(true, true, out, errOut, in)

	if err := engine.Release(t.Context(), deps, notesFileOptions(path)); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	rendered, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatalf("reading rendered output: %v", err)
	}
	if !slices.Contains(strings.Split(string(rendered), "\n"), "notes: accepted (-y)") {
		t.Errorf("rendered output has no %q line; got:\n%s", "notes: accepted (-y)", rendered)
	}
	assertNoAIWork(t, f)
	assertSuppliedBodyShipped(t, f, root, nextTag, "1.2.4", suppliedBody)
}

func createFile(t *testing.T, path string) *os.File {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating %s: %v", path, err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}

// Not parallel: t.Chdir sets the process working directory.
func TestRelease_NotesFile_RelativePathResolvesAgainstTheRunDirectory(t *testing.T) {
	root := t.TempDir()
	const subBody = "notes from <repo>/sub/notes.md\n"
	writeNotesFile(t, root, "notes.md", "notes from <repo>/notes.md\n")
	writeNotesFile(t, root, filepath.Join("sub", "notes.md"), subBody)
	t.Chdir(filepath.Join(root, "sub"))
	f := runner.NewFakeRunner()
	seedPriorTagNotesFileGit(f, root)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), notesFileOptions("notes.md")); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertSuppliedBodyShipped(t, f, root, nextTag, "1.2.4", subBody)
}

// Not parallel: t.Chdir sets the process working directory, and os.Stdin is swapped.
func TestRelease_NotesFile_DashNamesAFileNeverStdin(t *testing.T) {
	root := t.TempDir()
	runDir := t.TempDir()
	writeNotesFile(t, runDir, "-", suppliedBody)
	t.Chdir(runDir)

	const stdinBody = "stdin must never become the body\n"
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	if _, err := stdinW.WriteString(stdinBody); err != nil {
		t.Fatalf("writing stdin pipe: %v", err)
	}
	_ = stdinW.Close()
	origStdin := os.Stdin
	os.Stdin = stdinR
	t.Cleanup(func() {
		os.Stdin = origStdin
		_ = stdinR.Close()
	})

	f := runner.NewFakeRunner()
	seedPriorTagNotesFileGit(f, root)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), notesFileOptions("-")); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertSuppliedBodyShipped(t, f, root, nextTag, "1.2.4", suppliedBody)
	os.Stdin = origStdin
	left, err := io.ReadAll(stdinR)
	if err != nil {
		t.Fatalf("draining stdin pipe: %v", err)
	}
	if string(left) != stdinBody {
		t.Errorf("stdin left unread = %q, want %q (stdin must not be read)", left, stdinBody)
	}
}

// rewritingHookRunner rewrites a file when the pre_tag hook (`sh`) runs, then
// delegates to the wrapped FakeRunner.
type rewritingHookRunner struct {
	*runner.FakeRunner
	path    string
	rewrite string
}

func (r *rewritingHookRunner) RunInDir(ctx context.Context, dir string, env []string, name string, args ...string) (runner.Result, error) {
	if name == "sh" {
		if err := os.WriteFile(r.path, []byte(r.rewrite), 0o644); err != nil {
			return runner.Result{}, err
		}
	}
	return r.FakeRunner.RunInDir(ctx, dir, env, name, args...)
}

func TestRelease_NotesFile_BodyIsWhatWasReadAtStartupDespiteAPreTagRewrite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")
	const rewritten = "rewritten by the pre_tag hook mid-run\n"
	path := writeNotesFile(t, t.TempDir(), "notes.md", suppliedBody)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git", ScriptedOut("")) // -C root status --porcelain (post-hook: clean)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	f.Seed("sh", runner.Result{}, nil)
	r := &rewritingHookRunner{FakeRunner: f, path: path, rewrite: rewritten}
	rec := &presentertest.RecordingPresenter{}
	deps := newDeps(rec, f)
	deps.Runner = r

	if err := engine.Release(t.Context(), deps, notesFileOptions(path)); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the notes file back: %v", err)
	}
	if string(onDisk) != rewritten {
		t.Fatalf("notes file on disk = %q, want the hook's rewrite %q (the hook did not run)", onDisk, rewritten)
	}
	assertSuppliedBodyShipped(t, f, root, nextTag, "1.2.4", suppliedBody)
}
