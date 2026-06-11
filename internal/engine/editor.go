package engine

// This file implements the engine's Editor seam (declared in release.go): the
// real $EDITOR hand-off the `e` review-gate choice uses. It lives in the engine
// package because it is tightly coupled to the review-gate flow — it drives the
// presenter's spinner suspend/resume hooks around the terminal hand-off and
// signals "return to the gate" via ErrEditorReturnToGate when no editor can be
// launched, both of which the gate loop in release.go consumes directly.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"mint/internal/presenter"
	"mint/internal/runner"
)

// ErrEditorReturnToGate is the sentinel Edit returns when no editor could be
// LAUNCHED (the resolved binary is missing). It is NOT a failure: the launcher
// has already reported the problem through the presenter, and the gate loop
// branches on it (via errors.Is) to RE-PRESENT the gate with the body unchanged
// rather than aborting the run. Nothing is mutated on this path.
var ErrEditorReturnToGate = errors.New("editor could not be launched; return to gate")

// defaultEditor is the fallback launched when neither $VISUAL nor $EDITOR is set
// — the conventional always-present Unix editor.
const defaultEditor = "vi"

// ResolveEditor resolves which editor to launch for the `e` review-gate choice,
// honouring the conventional precedence: $VISUAL (when set and non-blank), then
// $EDITOR (when set and non-blank), then the "vi" fallback.
//
// A whitespace-only value is treated as unset: strings.Fields would split it
// into an empty slice (no command to launch), so a blank-after-trim $VISUAL or
// $EDITOR falls through to the next candidate exactly as an unset variable does.
//
// The returned value may carry arguments (e.g. "code --wait"): it is an
// operator-controlled command line that the launcher splits on whitespace
// (strings.Fields) at launch time, appending the notes temp-file path as the
// final argument. Resolution reads only the value; splitting is the launcher's
// concern.
func ResolveEditor() string {
	if visual := os.Getenv("VISUAL"); strings.TrimSpace(visual) != "" {
		return visual
	}
	if editor := os.Getenv("EDITOR"); strings.TrimSpace(editor) != "" {
		return editor
	}
	return defaultEditor
}

// EditorLauncher is the production Editor seam: it writes the current notes body
// to a temp file, launches the resolved editor on it through the CommandRunner's
// interactive seam (bracketed by the presenter's spinner suspend/resume hooks),
// and returns the saved text verbatim. It is constructed with the same presenter
// and runner the rest of the spine uses so production wires one of each.
type EditorLauncher struct {
	presenter presenter.Presenter
	runner    runner.CommandRunner
}

// NewEditorLauncher constructs an EditorLauncher from the shared presenter and
// runner. Production wires it once (at the cmd entry point) and the gate's `e`
// choice consults it.
func NewEditorLauncher(p presenter.Presenter, r runner.CommandRunner) *EditorLauncher {
	return &EditorLauncher{presenter: p, runner: r}
}

// Compile-time assertion that EditorLauncher satisfies the engine's Editor seam.
var _ Editor = (*EditorLauncher)(nil)

// Edit presents the current body in the user's editor and returns the saved text
// VERBATIM (no validation, no trimming, no normalisation — a human edit is
// trusted). It writes current to a temp file, resolves and launches the editor
// on that file, and reads the file back on success.
//
// The terminal hand-off is bracketed with the presenter's spinner controls:
// SuspendSpinner before the launch and ResumeSpinner after (deferred, so it runs
// even on a launch failure). Both are safe no-ops in plain mode and when nothing
// is animating.
//
// A MISSING editor (errors.Is(err, runner.ErrCommandNotFound)) is not fatal: the
// launcher reports the problem via the presenter and returns
// ErrEditorReturnToGate so the gate re-presents with the body unchanged. Any
// OTHER launch error (a launched-but-failed editor, an IO failure) is returned
// wrapped for the caller to surface and abort. The temp file is always removed.
func (e *EditorLauncher) Edit(ctx context.Context, current string) (string, error) {
	tmpPath, err := writeNotesTempFile(current)
	if err != nil {
		return "", err
	}
	// Always clean up the temp file, on every return path. The removal error has
	// nowhere useful to go (the edit already succeeded or failed on its own terms)
	// and a leaked temp file is harmless, so it is deliberately discarded.
	defer func() { _ = os.Remove(tmpPath) }()

	editor := ResolveEditor()
	fields := strings.Fields(editor)
	name := fields[0]
	args := append(fields[1:], tmpPath)

	// Bracket the terminal hand-off: stop the spinner before the editor takes the
	// terminal, restart it afterwards (deferred so it runs even on a launch error).
	e.presenter.SuspendSpinner()
	defer e.presenter.ResumeSpinner()

	if launchErr := e.runner.RunInteractive(ctx, name, args...); launchErr != nil {
		// A missing editor is not fatal: report it and signal return-to-gate so the
		// caller re-presents the gate with the body unchanged. Nothing is mutated.
		if errors.Is(launchErr, runner.ErrCommandNotFound) {
			e.presenter.Warn(presenter.Warning{
				Label:   "editor",
				Message: fmt.Sprintf("could not launch editor %q: not found", name),
			})
			return "", ErrEditorReturnToGate
		}
		// Any other launch failure is a genuine error the caller surfaces and aborts on.
		return "", fmt.Errorf("launching editor %q: %w", name, launchErr)
	}

	// Success: the editor saved its work to the temp file. Return those bytes
	// verbatim — no re-parse, no validation.
	saved, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("reading edited notes: %w", err)
	}
	return string(saved), nil
}

// writeNotesTempFile writes body to a fresh "mint-notes-*.md" temp file and
// returns its path. The caller owns cleanup (defer os.Remove). The file is
// closed before returning so the editor opens it cleanly.
func writeNotesTempFile(body string) (string, error) {
	tmp, err := os.CreateTemp("", "mint-notes-*.md")
	if err != nil {
		return "", fmt.Errorf("creating notes temp file: %w", err)
	}
	path := tmp.Name()
	if _, err := tmp.WriteString(body); err != nil {
		_ = tmp.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("writing notes temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("closing notes temp file: %w", err)
	}
	return path, nil
}
