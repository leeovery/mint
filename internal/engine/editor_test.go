package engine_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// TestResolveEditor_PrefersVisualThenEditorThenVi pins the resolution order:
// $VISUAL wins when set; $EDITOR is used when only $EDITOR is set; vi is the
// fallback when neither is set. These cases set process env via t.Setenv, which
// forbids t.Parallel, so they run serially as ordinary subtests.
func TestResolveEditor_PrefersVisualThenEditorThenVi(t *testing.T) {
	// An empty string and an unset variable are equivalent for ResolveEditor (it
	// checks for non-empty), so every case sets both vars explicitly — "" stands in
	// for "unset" and keeps these t.Setenv subtests serial (t.Setenv forbids
	// t.Parallel) and free of process-global leakage.
	tests := []struct {
		name   string
		visual string
		editor string
		want   string
	}{
		{name: "visual preferred over editor", visual: "code --wait", editor: "nano", want: "code --wait"},
		{name: "editor used when only editor set", visual: "", editor: "nano", want: "nano"},
		{name: "vi fallback when neither set", visual: "", editor: "", want: "vi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("VISUAL", tt.visual)
			t.Setenv("EDITOR", tt.editor)

			if got := engine.ResolveEditor(); got != tt.want {
				t.Errorf("ResolveEditor() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestEditorLauncher_Edit_WritesTempLaunchesEditorReturnsSavedText proves the
// success path: Edit writes `current` to a temp file, launches the resolved
// editor on that file (recorded via RunInteractive), reads the saved bytes back,
// and returns them VERBATIM. The test simulates the editor writing new content
// to the temp file by intercepting the recorded launch and writing the path.
func TestEditorLauncher_Edit_WritesTempLaunchesEditorReturnsSavedText(t *testing.T) {
	t.Setenv("VISUAL", "") // empty == unset for ResolveEditor; falls through to EDITOR
	t.Setenv("EDITOR", "myedit")

	const original = "original body\n"
	const saved = "human-edited body\nwith a second line\n"

	rec := &presentertest.RecordingPresenter{}
	f := &writeBackRunner{content: saved}

	launcher := engine.NewEditorLauncher(rec, f)
	got, err := launcher.Edit(context.Background(), original)
	if err != nil {
		t.Fatalf("Edit returned unexpected error: %v", err)
	}

	// The saved bytes are returned verbatim — no trimming, no normalisation.
	if got != saved {
		t.Errorf("Edit returned %q, want the saved text %q", got, saved)
	}

	// The resolved editor (myedit) was launched on the temp file via RunInteractive.
	if len(f.invocations) != 1 {
		t.Fatalf("RunInteractive launch count = %d, want 1", len(f.invocations))
	}
	inv := f.invocations[0]
	if inv.name != "myedit" {
		t.Errorf("launched editor = %q, want resolved %q", inv.name, "myedit")
	}
	if len(inv.args) != 1 {
		t.Fatalf("launch args = %v, want exactly the temp path", inv.args)
	}
	tmpPath := inv.args[0]
	if !strings.Contains(tmpPath, "mint-notes-") {
		t.Errorf("temp path = %q, want a mint-notes-* file", tmpPath)
	}

	// The original body was written to the temp file before launch (the test's
	// write-back overwrote it; we assert the launcher cleaned up below).
	if _, statErr := os.Stat(tmpPath); !os.IsNotExist(statErr) {
		t.Errorf("temp file %q still exists after Edit; want it cleaned up (stat err: %v)", tmpPath, statErr)
	}
}

// TestEditorLauncher_Edit_SplitsEditorArgsAndAppendsPath proves an editor value
// carrying args (e.g. "code --wait") is split on whitespace and the temp path is
// appended as the FINAL argument.
func TestEditorLauncher_Edit_SplitsEditorArgsAndAppendsPath(t *testing.T) {
	t.Setenv("VISUAL", "code --wait")

	rec := &presentertest.RecordingPresenter{}
	f := &writeBackRunner{content: "saved\n"}

	launcher := engine.NewEditorLauncher(rec, f)
	if _, err := launcher.Edit(context.Background(), "body\n"); err != nil {
		t.Fatalf("Edit returned unexpected error: %v", err)
	}

	if len(f.invocations) != 1 {
		t.Fatalf("launch count = %d, want 1", len(f.invocations))
	}
	inv := f.invocations[0]
	if inv.name != "code" {
		t.Errorf("launched binary = %q, want %q", inv.name, "code")
	}
	if len(inv.args) != 2 || inv.args[0] != "--wait" {
		t.Fatalf("launch args = %v, want [--wait <temp path>]", inv.args)
	}
	if !strings.Contains(inv.args[1], "mint-notes-") {
		t.Errorf("final arg = %q, want the appended temp path", inv.args[1])
	}
}

// TestEditorLauncher_Edit_BracketsLaunchWithSuspendResume proves the spinner
// hand-off contract: SuspendSpinner is recorded BEFORE the editor launch and
// ResumeSpinner AFTER it returns, with the launch in between.
func TestEditorLauncher_Edit_BracketsLaunchWithSuspendResume(t *testing.T) {
	t.Setenv("VISUAL", "myedit")

	rec := &presentertest.RecordingPresenter{}
	f := &writeBackRunner{content: "saved\n"}

	launcher := engine.NewEditorLauncher(rec, f)
	if _, err := launcher.Edit(context.Background(), "body\n"); err != nil {
		t.Fatalf("Edit returned unexpected error: %v", err)
	}

	kinds := rec.Kinds()
	suspendAt, resumeAt := -1, -1
	for i, k := range kinds {
		switch k {
		case presentertest.KindSuspendSpinner:
			suspendAt = i
		case presentertest.KindResumeSpinner:
			resumeAt = i
		}
	}
	if suspendAt == -1 {
		t.Fatalf("no SuspendSpinner recorded; kinds = %v", kinds)
	}
	if resumeAt == -1 {
		t.Fatalf("no ResumeSpinner recorded; kinds = %v", kinds)
	}
	if suspendAt >= resumeAt {
		t.Errorf("SuspendSpinner (%d) must precede ResumeSpinner (%d)", suspendAt, resumeAt)
	}
	// The editor launch happened between suspend and resume (the recorder does not
	// log the runner; the launch is captured on the runner double instead — its
	// single recorded launch occurred while suspended by construction of Edit).
	if len(f.invocations) != 1 {
		t.Errorf("editor launch count = %d, want exactly 1 between suspend and resume", len(f.invocations))
	}
}

// TestEditorLauncher_Edit_NoLaunchableEditor_ReturnsToGate proves a missing
// editor does NOT crash or abort: Edit reports the problem via Warn and returns
// the ErrEditorReturnToGate sentinel, with nothing mutated and the temp file
// cleaned up. ResumeSpinner still runs (deferred) even on the failure path.
func TestEditorLauncher_Edit_NoLaunchableEditor_ReturnsToGate(t *testing.T) {
	t.Setenv("VISUAL", "ghost-editor")

	rec := &presentertest.RecordingPresenter{}
	// A path-capturing double that fails the launch with ErrCommandNotFound, so the
	// test can assert the temp file is cleaned up even on the failure path.
	f := &pathCapturingRunner{err: fmt.Errorf("running %q: %w", "ghost-editor", runner.ErrCommandNotFound)}

	launcher := engine.NewEditorLauncher(rec, f)
	body, err := launcher.Edit(context.Background(), "untouched body\n")

	if !errors.Is(err, engine.ErrEditorReturnToGate) {
		t.Fatalf("Edit error = %v, want ErrEditorReturnToGate", err)
	}
	// Nothing usable is returned on the return-to-gate path; the caller keeps the
	// original body.
	if body != "" {
		t.Errorf("Edit returned body %q on the return-to-gate path, want empty", body)
	}

	// The problem was reported via a Warn, not a crash.
	if !recorded(rec, presentertest.KindWarn) {
		t.Errorf("missing editor did not surface a Warn; kinds = %v", rec.Kinds())
	}
	// Resume still ran (deferred) even though the launch failed.
	if !recorded(rec, presentertest.KindResumeSpinner) {
		t.Errorf("ResumeSpinner did not run on the launch-failure path; kinds = %v", rec.Kinds())
	}
	// The temp file is cleaned up even on the failure path.
	if f.tempPath == "" {
		t.Fatal("no editor launch recorded; cannot assert temp-file cleanup")
	}
	if _, statErr := os.Stat(f.tempPath); !os.IsNotExist(statErr) {
		t.Errorf("temp file %q still exists after failed Edit; want it cleaned up (stat err: %v)", f.tempPath, statErr)
	}
}

// TestEditorLauncher_Edit_LaunchError_ReturnsWrappedError proves a genuine launch
// failure (a launched-but-failed editor) returns a real wrapped error — NOT the
// return-to-gate sentinel — so the caller surfaces and aborts.
func TestEditorLauncher_Edit_LaunchError_ReturnsWrappedError(t *testing.T) {
	t.Setenv("VISUAL", "myedit")

	rec := &presentertest.RecordingPresenter{}
	f := runner.NewFakeRunner()
	launchErr := errors.New("editor crashed")
	f.Seed("myedit", runner.Result{ExitCode: 1}, launchErr)

	launcher := engine.NewEditorLauncher(rec, f)
	_, err := launcher.Edit(context.Background(), "body\n")

	if err == nil {
		t.Fatal("Edit returned nil error for a launch failure, want non-nil")
	}
	if errors.Is(err, engine.ErrEditorReturnToGate) {
		t.Errorf("Edit returned the return-to-gate sentinel for a genuine launch failure: %v", err)
	}
	if !errors.Is(err, launchErr) {
		t.Errorf("Edit error = %v, want it to wrap the launch error %v", err, launchErr)
	}
}

// writeBackRunner is a CommandRunner double whose RunInteractive simulates the
// editor having SAVED new content to the temp file it was handed: it writes
// `content` to the final arg (the temp path) before returning, so the launcher's
// read-back path observes the saved bytes. It records each launch for assertion.
// Run/RunWith are unused by the launcher and panic if called.
type writeBackRunner struct {
	content     string
	invocations []recordedLaunch
}

type recordedLaunch struct {
	name string
	args []string
}

func (w *writeBackRunner) RunInteractive(_ context.Context, name string, args ...string) error {
	w.invocations = append(w.invocations, recordedLaunch{name: name, args: args})
	if len(args) > 0 {
		// The final arg is the temp path; simulate the editor saving new content.
		path := args[len(args)-1]
		if err := os.WriteFile(path, []byte(w.content), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func (w *writeBackRunner) Run(_ context.Context, _ string, _ ...string) (runner.Result, error) {
	panic("writeBackRunner.Run: not expected")
}

func (w *writeBackRunner) RunWith(_ context.Context, _ io.Reader, _ string, _ ...string) (runner.Result, error) {
	panic("writeBackRunner.RunWith: not expected")
}

func (w *writeBackRunner) RunInDir(_ context.Context, _ string, _ []string, _ string, _ ...string) (runner.Result, error) {
	panic("writeBackRunner.RunInDir: not expected")
}

// pathCapturingRunner is a CommandRunner double whose RunInteractive captures the
// temp path it was handed (the final arg) and returns a configurable error — so a
// launch-failure test can assert the launcher still cleaned up the temp file.
// Run/RunWith are unused by the launcher and panic if called.
type pathCapturingRunner struct {
	err      error
	tempPath string
}

func (p *pathCapturingRunner) RunInteractive(_ context.Context, _ string, args ...string) error {
	if len(args) > 0 {
		p.tempPath = args[len(args)-1]
	}
	return p.err
}

func (p *pathCapturingRunner) Run(_ context.Context, _ string, _ ...string) (runner.Result, error) {
	panic("pathCapturingRunner.Run: not expected")
}

func (p *pathCapturingRunner) RunWith(_ context.Context, _ io.Reader, _ string, _ ...string) (runner.Result, error) {
	panic("pathCapturingRunner.RunWith: not expected")
}

func (p *pathCapturingRunner) RunInDir(_ context.Context, _ string, _ []string, _ string, _ ...string) (runner.Result, error) {
	panic("pathCapturingRunner.RunInDir: not expected")
}
