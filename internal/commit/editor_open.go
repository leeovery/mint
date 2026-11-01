package commit

// This file holds commit's REUSABLE editor file-roundtrip routine — the way the
// "no AI message" fallback cases and the gate's `e` action open an editor against a
// temp file. It MIRRORS the engine's EditorLauncher.Edit precedent (temp file →
// argv + path → RunInteractive → read-back, bracketed by the
// presenter's spinner suspend/resume) WITHOUT importing the engine's release-coupled
// policy: editor resolution comes from commit's own ResolveEditor (git's full
// precedence via `git var GIT_EDITOR`), and the routine itself NEVER stages or
// commits — it only opens the editor and hands back the result for the caller's
// save-as-accept / empty-save / aborted decision.
//
// The message ALWAYS travels via the FILE, never stdin. The initial buffer is
// caller-supplied (empty/template on the --no-ai path here; Phase 4's `e` pre-fills
// the current message into the SAME routine), so the open is a single shared seam.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"mint/internal/cmdline"
	"mint/internal/presenter"
	"mint/internal/runner"
)

// OpenEditor writes initial to a temp file, resolves the editor (git's full
// precedence via ResolveEditor), launches it on that file through the runner's
// INTERACTIVE seam (the only one that hands the child the real terminal), waits for
// it to exit, and reads the saved file back.
//
// It returns the saved buffer and whether the editor exited NORMALLY (so the caller
// can make the save-as-accept / empty-save / aborted decision):
//
//   - Normal exit → (saved bytes verbatim, true, nil). No trimming or normalisation;
//     a human edit is trusted, and the caller owns the whitespace-only "empty" rule.
//   - Aborted/quit editor (a launched-but-failed run, e.g. `:cq`) → ("", false, nil):
//     a NO-OP signal, NOT an error, so the caller treats it as a true no-op rather
//     than routing it to fail-loud.
//   - Missing/unlaunchable editor (ErrNoEditor from resolution, or an
//     *editorOpenError: a command that can't be split, or a launch failing with
//     runner.ErrCommandNotFound or runner.ErrNotStarted) → ("", false, err):
//     SURFACED to the caller (see editorUnopenable). Routing that to fail-loud is
//     task 3-5, NOT here.
//   - A genuine IO failure (temp-file write/read) → ("", false, err).
//
// The terminal hand-off is bracketed by the presenter's SuspendSpinner before the
// launch and ResumeSpinner after (deferred, so it runs even on a launch failure).
// Both are safe no-ops in plain mode and when nothing is animating. The temp file is
// always removed on every return path. The routine NEVER stages or commits.
func OpenEditor(ctx context.Context, r runner.CommandRunner, p presenter.Presenter, initial string) (string, bool, error) {
	editor, err := ResolveEditor(ctx, r)
	if err != nil {
		// No launchable editor in git's chain: surface the not-launchable signal to the
		// caller (3-5 routes it to fail-loud / graceful-degrade). Nothing is opened.
		return "", false, err
	}
	// The value may carry quoting and args ("'/Applications/Sublime Text.app/…/subl'
	// -w"): it is split as a POSIX shell splits words, but run with no shell, so
	// nothing else in it is expanded. The buffer travels via the file, never stdin.
	words, err := cmdline.Split(editor)
	if err != nil {
		return "", false, &editorOpenError{command: editor, err: err}
	}
	if len(words) == 0 {
		return "", false, ErrNoEditor
	}

	tmpPath, err := writeEditorTempFile(initial)
	if err != nil {
		return "", false, err
	}
	// Always clean up the temp file on every return path. The removal error has nowhere
	// useful to go and a leaked temp file is harmless, so it is deliberately discarded.
	defer func() { _ = os.Remove(tmpPath) }()

	name := words[0]
	args := append(words[1:], tmpPath)

	// Bracket the terminal hand-off: stop the spinner before the editor takes the
	// terminal, restart it afterwards (deferred so it runs even on a launch error).
	p.SuspendSpinner()
	defer p.ResumeSpinner()

	if launchErr := r.RunInteractive(ctx, name, args...); launchErr != nil {
		// An editor that is missing or never started is surfaced to the caller (3-5
		// routes it); it is NOT the same as a quit/abort and must not be swallowed as a
		// no-op.
		if errors.Is(launchErr, runner.ErrCommandNotFound) || errors.Is(launchErr, runner.ErrNotStarted) {
			return "", false, &editorOpenError{command: editor, words: words, err: launchErr}
		}
		// Any other launch failure is a quit/abort (the editor ran and exited non-zero):
		// report a non-normal exit as a NO-OP signal, not an error.
		return "", false, nil
	}

	saved, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", false, fmt.Errorf("reading edited message: %w", err)
	}
	return string(saved), true, nil
}

// editorOpenError is the editor command git named that could not be opened, err
// saying why: it can't be split into words (words nil), or the program it names is
// missing or never started.
type editorOpenError struct {
	command string
	words   []string
	err     error
}

func (e *editorOpenError) Error() string {
	if e.words == nil {
		return fmt.Sprintf("reading editor command %q: %v", e.command, e.err)
	}
	return fmt.Sprintf("launching editor %q: %v", e.words[0], e.err)
}

func (e *editorOpenError) Unwrap() error { return e.err }

// shellOnly reports whether the command leans on a shell mint doesn't run: a word
// opening with ~, or holding a $.
func (e *editorOpenError) shellOnly() bool {
	for _, word := range e.words {
		if strings.HasPrefix(word, "~") || strings.Contains(word, "$") {
			return true
		}
	}
	return false
}

// editorUnopenable reports whether err is OpenEditor saying no editor could be
// opened: git names none, or the command it names can't be split, or names a
// program that is missing or never started.
func editorUnopenable(err error) bool {
	var unopened *editorOpenError
	return errors.Is(err, ErrNoEditor) || errors.As(err, &unopened)
}

// writeEditorTempFile writes body to a fresh "mint-commit-*.txt" temp file and
// returns its path. The caller owns cleanup (defer os.Remove). The file is closed
// before returning so the editor opens it cleanly.
func writeEditorTempFile(body string) (string, error) {
	tmp, err := os.CreateTemp("", "mint-commit-*.txt")
	if err != nil {
		return "", fmt.Errorf("creating commit-message temp file: %w", err)
	}
	path := tmp.Name()
	if _, err := tmp.WriteString(body); err != nil {
		_ = tmp.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("writing commit-message temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("closing commit-message temp file: %w", err)
	}
	return path, nil
}
