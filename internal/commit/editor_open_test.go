package commit_test

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"mint/internal/commit"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// editorRunner is a CommandRunner double for the editor file-roundtrip routine: it
// delegates the read-only git calls (Run/RunWith) to an embedded FakeRunner so the
// `git var GIT_EDITOR` resolution scripts as usual, while its RunInteractive
// simulates the editor having SAVED `saved` to the temp file it was handed (the
// final arg) before returning — so the routine's read-back observes the saved bytes.
// When launchErr is non-nil RunInteractive returns it WITHOUT writing (the editor
// failed to launch or the user quit), and notFound seeds the resolution itself as a
// missing editor. Each interactive launch is recorded for assertion.
type editorRunner struct {
	fake      *runner.FakeRunner
	saved     string
	launchErr error
	launches  []runner.Invocation
	// onLaunch, when set, is called with the temp-file path at launch time (before
	// the double overwrites it with `saved`), letting a test capture the pre-filled
	// buffer the editor was handed.
	onLaunch func(path string)
}

// newEditorRunner builds an editorRunner whose `git var GIT_EDITOR` resolves to
// editor and whose simulated editor saves saved to the temp file on launch.
func newEditorRunner(editor, saved string) *editorRunner {
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{Stdout: editor + "\n"}, nil)
	return &editorRunner{fake: f, saved: saved}
}

func (e *editorRunner) Run(ctx context.Context, name string, args ...string) (runner.Result, error) {
	return e.fake.Run(ctx, name, args...)
}

func (e *editorRunner) RunWith(ctx context.Context, stdin io.Reader, name string, args ...string) (runner.Result, error) {
	return e.fake.RunWith(ctx, stdin, name, args...)
}

func (e *editorRunner) RunInDir(ctx context.Context, dir string, env []string, name string, args ...string) (runner.Result, error) {
	return e.fake.RunInDir(ctx, dir, env, name, args...)
}

func (e *editorRunner) RunInteractive(_ context.Context, name string, args ...string) error {
	e.launches = append(e.launches, runner.Invocation{Name: name, Args: args})
	if e.launchErr != nil {
		return e.launchErr
	}
	if len(args) > 0 {
		path := args[len(args)-1]
		if e.onLaunch != nil {
			e.onLaunch(path)
		}
		if err := os.WriteFile(path, []byte(e.saved), 0o600); err != nil {
			return err
		}
	}
	return nil
}

// TestOpenEditor_WritesInitialBufferLaunchesResolvedEditorReadsBack proves the
// success path: the routine resolves the editor (via `git var GIT_EDITOR`), writes
// the INITIAL BUFFER to a temp file, launches the resolved editor on that path via
// RunInteractive (NOT stdin), waits for exit, and reads the saved bytes back.
func TestOpenEditor_WritesInitialBufferLaunchesResolvedEditorReadsBack(t *testing.T) {
	t.Parallel()

	const saved = "feat: human-written subject\n\nA body the user typed.\n"
	rec := &presentertest.RecordingPresenter{}
	er := newEditorRunner("myedit", saved)

	got, ok, err := commit.OpenEditor(context.Background(), er, rec, "")
	if err != nil {
		t.Fatalf("OpenEditor returned unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("OpenEditor reported the editor did not exit normally; want a normal exit")
	}
	if got != saved {
		t.Errorf("OpenEditor returned %q, want the saved bytes verbatim %q", got, saved)
	}

	if len(er.launches) != 1 {
		t.Fatalf("RunInteractive launch count = %d, want exactly 1", len(er.launches))
	}
	launch := er.launches[0]
	if launch.Name != "myedit" {
		t.Errorf("launched editor = %q, want the resolved %q", launch.Name, "myedit")
	}
	if len(launch.Args) != 1 {
		t.Fatalf("launch args = %v, want exactly the temp path appended", launch.Args)
	}
	tmpPath := launch.Args[0]
	if !strings.Contains(tmpPath, "mint-") {
		t.Errorf("temp path = %q, want a mint-* temp file", tmpPath)
	}
	// The temp file is cleaned up on the success path.
	if _, statErr := os.Stat(tmpPath); !os.IsNotExist(statErr) {
		t.Errorf("temp file %q still exists after OpenEditor; want it cleaned up (stat err: %v)", tmpPath, statErr)
	}

	// The message went via the FILE, never stdin: no RunWith stdin carried the buffer.
	for _, inv := range er.fake.Invocations() {
		if inv.Stdin != "" {
			t.Errorf("a git call carried stdin %q; the editor buffer must go via the temp file, never stdin", inv.Stdin)
		}
	}
}

// TestOpenEditor_SplitsEditorArgsAndAppendsPath proves a multi-word editor command
// (e.g. "code --wait") is split on whitespace into program + args, with the temp
// path appended as the FINAL arg — never fed via stdin.
func TestOpenEditor_SplitsEditorArgsAndAppendsPath(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	er := newEditorRunner("code --wait", "feat: x\n")

	if _, _, err := commit.OpenEditor(context.Background(), er, rec, ""); err != nil {
		t.Fatalf("OpenEditor returned unexpected error: %v", err)
	}

	if len(er.launches) != 1 {
		t.Fatalf("RunInteractive launch count = %d, want exactly 1", len(er.launches))
	}
	launch := er.launches[0]
	if launch.Name != "code" {
		t.Errorf("launched program = %q, want %q (split from the multi-word command)", launch.Name, "code")
	}
	if len(launch.Args) != 2 || launch.Args[0] != "--wait" {
		t.Fatalf("launch args = %v, want [--wait <temp path>]", launch.Args)
	}
	if !strings.Contains(launch.Args[1], "mint-") {
		t.Errorf("final arg = %q, want the appended temp path", launch.Args[1])
	}
}

// TestOpenEditor_PreFillsCallerSuppliedInitialBuffer proves the routine accepts a
// caller-supplied initial buffer and writes it to the temp file before launch — the
// pre-fill capability 4-1's `e` reuses. The double captures the temp-file contents
// AT LAUNCH (before its own save-back) so the test can assert the pre-filled bytes.
func TestOpenEditor_PreFillsCallerSuppliedInitialBuffer(t *testing.T) {
	t.Parallel()

	const initial = "fix: a message to pre-fill\n\nwith a body\n"
	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: newSeededEditorGit("myedit"), saved: "edited\n"}
	// Capture the buffer the editor was handed: read the temp file at launch time,
	// before the double overwrites it with `saved`.
	var preFilled string
	er.onLaunch = func(path string) {
		b, _ := os.ReadFile(path)
		preFilled = string(b)
	}

	if _, _, err := commit.OpenEditor(context.Background(), er, rec, initial); err != nil {
		t.Fatalf("OpenEditor returned unexpected error: %v", err)
	}

	if preFilled != initial {
		t.Errorf("temp file at launch = %q, want the caller-supplied initial buffer %q", preFilled, initial)
	}
}

// TestOpenEditor_MissingEditor_SurfacesToCaller proves a missing editor surfaces an
// error to the caller (matching runner.ErrCommandNotFound or commit.ErrNoEditor) —
// the routine does NOT route it to fail-loud itself (that is task 3-5). The editor
// is recorded as NOT launched in this case (resolution failed first).
func TestOpenEditor_MissingEditor_SurfacesToCaller(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	f := runner.NewFakeRunner()
	// `git var GIT_EDITOR` fails → ErrNoEditor from ResolveEditor.
	f.Seed("git", runner.Result{Stderr: "terminal is dumb", ExitCode: 128}, errors.New("exit status 128"))
	er := &editorRunner{fake: f}

	_, _, err := commit.OpenEditor(context.Background(), er, rec, "")
	if !errors.Is(err, commit.ErrNoEditor) {
		t.Fatalf("OpenEditor error = %v, want it to match commit.ErrNoEditor for an unresolvable editor", err)
	}
	if len(er.launches) != 0 {
		t.Errorf("RunInteractive launched %d time(s) despite resolution failing; nothing should launch", len(er.launches))
	}
}

// TestOpenEditor_NotFoundBinary_SurfacesCommandNotFound proves a RESOLVED editor
// whose binary is missing on PATH surfaces runner.ErrCommandNotFound to the caller
// (the launch failed with the not-found sentinel), not a no-op signal.
func TestOpenEditor_NotFoundBinary_SurfacesCommandNotFound(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: newSeededEditorGit("ghost-editor"), launchErr: wrapNotFound("ghost-editor")}

	_, _, err := commit.OpenEditor(context.Background(), er, rec, "")
	if !errors.Is(err, runner.ErrCommandNotFound) {
		t.Fatalf("OpenEditor error = %v, want it to match runner.ErrCommandNotFound for a missing binary", err)
	}
}

// TestOpenEditor_AbortedEditor_ReportsNotNormalExit proves an editor that launched
// but exited ABNORMALLY (quit/abort — a non-not-found RunInteractive error) is
// reported as ok=false WITHOUT an error, so the caller treats it as a no-op rather
// than a launch failure to route to fail-loud.
func TestOpenEditor_AbortedEditor_ReportsNotNormalExit(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: newSeededEditorGit("myedit"), launchErr: errors.New("exit status 1")}

	saved, ok, err := commit.OpenEditor(context.Background(), er, rec, "")
	if err != nil {
		t.Fatalf("OpenEditor returned an error for an aborted editor; want a no-op signal: %v", err)
	}
	if ok {
		t.Error("OpenEditor reported a normal exit for an aborted editor; want ok=false")
	}
	if saved != "" {
		t.Errorf("OpenEditor returned %q for an aborted editor; want an empty buffer", saved)
	}
}

// TestOpenEditor_BracketsLaunchWithSuspendResume proves the terminal hand-off is
// bracketed by the presenter's SuspendSpinner / ResumeSpinner around the launch.
func TestOpenEditor_BracketsLaunchWithSuspendResume(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	er := newEditorRunner("myedit", "feat: x\n")

	if _, _, err := commit.OpenEditor(context.Background(), er, rec, ""); err != nil {
		t.Fatalf("OpenEditor returned unexpected error: %v", err)
	}

	kinds := rec.Kinds()
	suspendIdx := indexOfKind(kinds, presentertest.KindSuspendSpinner)
	resumeIdx := indexOfKind(kinds, presentertest.KindResumeSpinner)
	if suspendIdx < 0 || resumeIdx < 0 {
		t.Fatalf("kinds = %v, want both a SuspendSpinner and a ResumeSpinner bracketing the launch", kinds)
	}
	if suspendIdx >= resumeIdx {
		t.Errorf("SuspendSpinner at %d, ResumeSpinner at %d; suspend must precede resume", suspendIdx, resumeIdx)
	}
}

// newSeededEditorGit returns a FakeRunner whose `git var GIT_EDITOR` resolves to
// editor — the read-only resolution scripting shared by the editor-open tests.
func newSeededEditorGit(editor string) *runner.FakeRunner {
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{Stdout: editor + "\n"}, nil)
	return f
}

// wrapNotFound returns an error matching runner.ErrCommandNotFound for name —
// mirroring the runner's missing-binary report so a resolved-but-missing editor can
// be simulated at the launch.
func wrapNotFound(name string) error {
	return &notFoundErr{name: name}
}

type notFoundErr struct{ name string }

func (e *notFoundErr) Error() string { return "running " + e.name + ": command not found" }
func (e *notFoundErr) Unwrap() error { return runner.ErrCommandNotFound }
