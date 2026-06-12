package presenter_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// commitMessage is the shared specimen for the ShowMessage tests: the
// commit-command shape this event exists to serve — an engine-titled block whose
// body is a generated commit message shown for review.
func commitMessage() presenter.Message {
	return presenter.Message{
		Title: "commit message",
		Body:  "feat: add retry to publish\n\nRetries the gh release call once on a 5xx.",
	}
}

// TestPlainShowMessageWrapsBodyInTitledDelimiters proves the plain block renders
// the engine-supplied title verbatim in BOTH sliceable delimiters and the body
// byte-for-byte between them.
func TestPlainShowMessageWrapsBodyInTitledDelimiters(t *testing.T) {
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	p := presenter.NewPlainPresenter(out, errBuf)

	p.ShowMessage(commitMessage())

	want := "--- commit message ---\n" +
		"feat: add retry to publish\n\nRetries the gh release call once on a 5xx.\n" +
		"--- end commit message ---\n"
	if out.String() != want {
		t.Errorf("plain ShowMessage out = %q, want %q", out.String(), want)
	}
	if errBuf.Len() != 0 {
		t.Errorf("plain ShowMessage wrote to err: %q; messages are narration -> out only", errBuf.String())
	}
	assertBytePureASCIIStreams(t, "plain showmessage", out, errBuf)
}

// TestPlainShowMessageEmptyBodyRendersBareDelimiters proves an empty body renders
// the opener immediately followed by the closer — no spurious blank line, no
// invented content.
func TestPlainShowMessageEmptyBodyRendersBareDelimiters(t *testing.T) {
	out := &bytes.Buffer{}
	p := presenter.NewPlainPresenter(out, &bytes.Buffer{})

	p.ShowMessage(presenter.Message{Title: "commit message"})

	want := "--- commit message ---\n--- end commit message ---\n"
	if out.String() != want {
		t.Errorf("plain ShowMessage empty body = %q, want %q", out.String(), want)
	}
}

// TestPlainShowMessageDelimiterLookalikeBodyPassesThrough proves the delimiters
// are positional, never content-matched: a body line that reads exactly like the
// closing delimiter survives verbatim and the real closer still follows.
func TestPlainShowMessageDelimiterLookalikeBodyPassesThrough(t *testing.T) {
	out := &bytes.Buffer{}
	p := presenter.NewPlainPresenter(out, &bytes.Buffer{})

	p.ShowMessage(presenter.Message{Title: "commit message", Body: "--- end commit message ---"})

	if got := strings.Count(out.String(), "--- end commit message ---\n"); got != 2 {
		t.Errorf("closing delimiter count = %d, want 2 (lookalike body line + real closer)", got)
	}
}

// TestPrettyShowMessageRendersGutterPanel proves the pretty block uses the shared
// gutter-panel treatment ShowNotes uses: a dim "│ {title}" line with the
// engine-supplied title verbatim, a bare "│" spacer, then every body line behind
// the "│ " gutter (empty body lines as a bare "│") — no titled/closing rules, no
// width math. Narration → out only, never err.
func TestPrettyShowMessageRendersGutterPanel(t *testing.T) {
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	p := presenter.NewPrettyPresenter(out, presenter.WithProfile(termenv.Ascii), presenter.WithErr(errBuf))

	p.ShowMessage(commitMessage())

	want := "│ commit message\n" +
		"│\n" +
		"│ feat: add retry to publish\n" +
		"│\n" +
		"│ Retries the gh release call once on a 5xx.\n"
	if out.String() != want {
		t.Errorf("pretty ShowMessage out = %q, want %q", out.String(), want)
	}
	if errBuf.Len() != 0 {
		t.Errorf("pretty ShowMessage wrote to err: %q; messages are narration -> out only", errBuf.String())
	}
}

// TestPrettyShowMessageBodyInFullAtAnyTermWidth proves termWidth no longer
// affects ShowMessage: at a narrow, a huge, and an undetectable (0) width, every
// body line still renders in full behind the gutter — the panel does no width
// math and never truncates.
func TestPrettyShowMessageBodyInFullAtAnyTermWidth(t *testing.T) {
	m := commitMessage()
	for _, width := range []int{30, 2000, 0} {
		out := &bytes.Buffer{}
		p := presenter.NewPrettyPresenter(out, presenter.WithProfile(termenv.Ascii)).WithTermWidth(width)

		p.ShowMessage(m)

		assertGutterPanel(t, out.String(), m.Title, m.Body)
	}
}

// TestShowMessageBodyContentIntactAcrossModes pins the REVISED cross-mode
// contract for message blocks (the old pretty-bytes-identical-to-plain invariant
// was deliberately traded away for the gutter): (a) PLAIN still writes the body
// VERBATIM between its delimiters — the shipped bytes are untouched; (b) PRETTY
// renders every body line's CONTENT intact behind the "│ " gutter — same line
// count, each non-empty line as "│ {line}", each empty line as a bare "│",
// nothing truncated or reordered.
func TestShowMessageBodyContentIntactAcrossModes(t *testing.T) {
	m := commitMessage()

	plainOut := &bytes.Buffer{}
	presenter.NewPlainPresenter(plainOut, &bytes.Buffer{}).ShowMessage(m)
	prettyOut := &bytes.Buffer{}
	presenter.NewPrettyPresenter(prettyOut, presenter.WithProfile(termenv.Ascii)).ShowMessage(m)

	// (a) Plain: the body region between the positional delimiters is the source
	// byte-for-byte.
	extract := func(s, closer string) string {
		afterOpener := s[strings.Index(s, "\n")+1:]
		return strings.TrimSuffix(afterOpener, closer)
	}
	plainBody := extract(plainOut.String(), "--- end commit message ---\n")
	if want := m.Body + "\n"; plainBody != want {
		t.Errorf("plain body region = %q, want the verbatim source %q", plainBody, want)
	}

	// (b) Pretty: title line, bare-"│" spacer, then exactly one gutter line per
	// body line with the content intact.
	assertGutterPanel(t, prettyOut.String(), m.Title, m.Body)
}
