package presenter_test

import (
	"bytes"
	"strings"
	"testing"

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

// TestShowMessageBodyContentIntactAcrossModes pins the cross-mode contract for
// message blocks: (a) PLAIN writes the body VERBATIM between its delimiters — the
// shipped bytes are untouched; (b) the RAIL shows every body line's CONTENT intact
// behind its gutter, in order, nothing truncated or reordered.
func TestShowMessageBodyContentIntactAcrossModes(t *testing.T) {
	m := commitMessage()

	plainOut := &bytes.Buffer{}
	presenter.NewPlainPresenter(plainOut, &bytes.Buffer{}).ShowMessage(m)
	r := newRailTerm(t, 100, 24, answers())
	commitRun(r)
	r.StageSucceeded(presenter.StageSuccess{Name: "message", Key: "message", Detail: "written"})
	r.ShowMessage(m)

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

	// (b) The Rail: each body line, behind the gutter, in order.
	screen := r.screen(t)
	at := 0
	for _, line := range strings.Split(m.Body, "\n") {
		if line == "" {
			continue
		}
		for at < len(screen) && !strings.HasSuffix(pane(screen[at]), "    "+line) {
			at++
		}
		if at == len(screen) {
			t.Fatalf("the rail doesn't show body line %q intact, in order, behind its gutter:\n%s", line, strings.Join(screen, "\n"))
		}
		if !strings.HasPrefix(screen[at], " │") {
			t.Errorf("body line %q is not behind the rail's gutter: %q", line, screen[at])
		}
	}
}
