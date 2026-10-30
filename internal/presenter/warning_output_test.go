package presenter_test

import (
	"bytes"
	"strings"
	"testing"

	"mint/internal/presenter"
)

// pushWarning is the shared specimen for the warn-with-output tests: the
// engine-shaped non-terminal push failure (the commit landed; the push can be
// retried) carrying git's captured stderr verbatim.
func pushWarning() presenter.Warning {
	return presenter.Warning{
		Label:   "push",
		Message: "commit is in place; re-run the push to retry",
		Output:  "remote: permission denied\nfatal: unable to access 'origin'",
	}
}

// TestPlainWarnRendersCapturedOutputBlockOutOnly proves the plain warn renders a
// non-empty Output beneath the WARN line wrapped in the same sliceable
// "--- output ---" delimiters StageFailed uses, body verbatim — while err still
// receives ONLY the one-line WARN summary, never the body.
func TestPlainWarnRendersCapturedOutputBlockOutOnly(t *testing.T) {
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	p := presenter.NewPlainPresenter(out, errBuf)

	p.Warn(pushWarning())

	wantOut := "push: WARN - commit is in place; re-run the push to retry\n" +
		"--- output ---\n" +
		"remote: permission denied\nfatal: unable to access 'origin'\n" +
		"--- end output ---\n"
	if out.String() != wantOut {
		t.Errorf("plain warn out = %q, want %q", out.String(), wantOut)
	}
	wantErr := "push: WARN - commit is in place; re-run the push to retry\n"
	if errBuf.String() != wantErr {
		t.Errorf("plain warn err = %q, want only the one-line summary %q", errBuf.String(), wantErr)
	}
}

// TestPlainWarnEmptyOutputRendersNoBlock proves the common Output=="" case is
// byte-identical to the pre-Output warn rendering: one line, no delimiters.
func TestPlainWarnEmptyOutputRendersNoBlock(t *testing.T) {
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	p := presenter.NewPlainPresenter(out, errBuf)

	p.Warn(presenter.Warning{Label: "post_release", Message: "hook exited 1"})

	want := "post_release: WARN - hook exited 1\n"
	if out.String() != want {
		t.Errorf("plain warn out = %q, want %q (no output block)", out.String(), want)
	}
}

// TestPlainWarnOutputDelimiterLookalikeBodyPassesThrough proves the delimiters
// are positional, never content-matched: a body line that reads exactly like the
// closing delimiter is written verbatim and the real closer still follows.
func TestPlainWarnOutputDelimiterLookalikeBodyPassesThrough(t *testing.T) {
	out := &bytes.Buffer{}
	p := presenter.NewPlainPresenter(out, &bytes.Buffer{})

	p.Warn(presenter.Warning{Label: "push", Message: "retry", Output: "--- end output ---"})

	if got := strings.Count(out.String(), "--- end output ---\n"); got != 2 {
		t.Errorf("closing delimiter count = %d, want 2 (lookalike body line + real closer)", got)
	}
}

// TestPlainWarnWithOutputDoesNotSuppressSuccessFooter proves a warn carrying
// captured output stays NON-terminal: the success end-of-run line still renders
// afterwards (only StageFailed/Unwound suppress it).
func TestPlainWarnWithOutputDoesNotSuppressSuccessFooter(t *testing.T) {
	out := &bytes.Buffer{}
	p := presenter.NewPlainPresenter(out, &bytes.Buffer{})

	p.Warn(pushWarning())
	p.RunFinished(presenter.RunResult{Project: "mint", Version: "1.4.0"})

	if !strings.HasSuffix(out.String(), "done: mint v1.4.0\n") {
		t.Errorf("out = %q, want success footer after warn-with-output", out.String())
	}
}

// TestRailWarnHangsCapturedOutputOnOutOnly proves the Rail hangs a warning's
// captured Output under its node, each line intact behind the gutter, while err
// receives only the unstyled one-line summary.
func TestRailWarnHangsCapturedOutputOnOutOnly(t *testing.T) {
	w := pushWarning()
	r := newRailTerm(t, 100, 24, answers())

	r.Warn(w)

	screen := strings.Join(r.screen(t), "\n")
	for _, line := range strings.Split(w.Output, "\n") {
		if !strings.Contains(screen, "│ "+line) {
			t.Errorf("the rail doesn't hang output line %q behind a gutter:\n%s", line, screen)
		}
	}
	if want := "⚠ push  commit is in place; re-run the push to retry\n"; r.err.String() != want {
		t.Errorf("rail warn err = %q, want only the one-line summary %q", r.err.String(), want)
	}
}
