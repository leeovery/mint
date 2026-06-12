package presenter_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/muesli/termenv"

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

// TestPrettyWarnRendersCapturedOutputFlushBelowOutOnly proves the pretty warn
// renders a non-empty Output flush and unstyled below the ⚠ line — mirroring
// StageFailed's boxless captured-body treatment — while err receives only the
// unstyled one-line summary.
func TestPrettyWarnRendersCapturedOutputFlushBelowOutOnly(t *testing.T) {
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	p := presenter.NewPrettyPresenter(out, presenter.WithProfile(termenv.Ascii), presenter.WithErr(errBuf))

	p.Warn(pushWarning())

	wantOut := "⚠ push  commit is in place; re-run the push to retry\n" +
		"remote: permission denied\nfatal: unable to access 'origin'\n"
	if out.String() != wantOut {
		t.Errorf("pretty warn out = %q, want %q", out.String(), wantOut)
	}
	wantErr := "⚠ push  commit is in place; re-run the push to retry\n"
	if errBuf.String() != wantErr {
		t.Errorf("pretty warn err = %q, want only the one-line summary %q", errBuf.String(), wantErr)
	}
}

// TestPrettyWarnEmptyOutputRendersNoBlock proves the pretty Output=="" case stays
// byte-identical to the pre-Output warn rendering.
func TestPrettyWarnEmptyOutputRendersNoBlock(t *testing.T) {
	out := &bytes.Buffer{}
	p := presenter.NewPrettyPresenter(out, presenter.WithProfile(termenv.Ascii))

	p.Warn(presenter.Warning{Label: "post_release", Message: "hook exited 1"})

	want := "⚠ post_release  hook exited 1\n"
	if out.String() != want {
		t.Errorf("pretty warn out = %q, want %q (no output block)", out.String(), want)
	}
}

// TestWarnOutputBodyIsByteIdenticalAcrossModes proves the captured body region is
// the SAME bytes in both modes — both presenters write Warning.Output through the
// shared verbatim helper; only the framing differs.
func TestWarnOutputBodyIsByteIdenticalAcrossModes(t *testing.T) {
	w := pushWarning()

	plainOut := &bytes.Buffer{}
	presenter.NewPlainPresenter(plainOut, &bytes.Buffer{}).Warn(w)
	prettyOut := &bytes.Buffer{}
	presenter.NewPrettyPresenter(prettyOut, presenter.WithProfile(termenv.Ascii)).Warn(w)

	wantBody := w.Output + "\n"
	if !strings.Contains(plainOut.String(), wantBody) {
		t.Errorf("plain warn out %q does not contain the verbatim body %q", plainOut.String(), wantBody)
	}
	if !strings.Contains(prettyOut.String(), wantBody) {
		t.Errorf("pretty warn out %q does not contain the verbatim body %q", prettyOut.String(), wantBody)
	}
}
