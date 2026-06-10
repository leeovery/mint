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

// TestPrettyShowMessageRendersTitledRulesAtCapWidth proves the pretty block uses
// the shared titled-rule treatment at the capped width: an opener
// "── {title} ────…" filled to the cap, the body flush between, and a closing
// rule of exactly the cap width — the same width source ShowNotes uses.
func TestPrettyShowMessageRendersTitledRulesAtCapWidth(t *testing.T) {
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	p := presenter.NewPrettyPresenter(out, presenter.WithProfile(termenv.Ascii), presenter.WithErr(errBuf))

	m := commitMessage()
	p.ShowMessage(m)

	lines := strings.Split(out.String(), "\n")
	opener := lines[0]
	wantPrefix := "── commit message "
	if !strings.HasPrefix(opener, wantPrefix) {
		t.Errorf("opener rule = %q, want prefix %q", opener, wantPrefix)
	}
	if got := len([]rune(opener)); got != presenter.RuleCapForTest {
		t.Errorf("opener rule width = %d runes, want the cap %d", got, presenter.RuleCapForTest)
	}
	closer := lines[len(lines)-2] // the final element after Split is the empty tail
	if closer != strings.Repeat("─", presenter.RuleCapForTest) {
		t.Errorf("closing rule = %q, want %d rule chars", closer, presenter.RuleCapForTest)
	}
	wantBody := m.Body + "\n"
	if !strings.Contains(out.String(), wantBody) {
		t.Errorf("pretty ShowMessage out %q does not contain the verbatim body %q", out.String(), wantBody)
	}
	if errBuf.Len() != 0 {
		t.Errorf("pretty ShowMessage wrote to err: %q; messages are narration -> out only", errBuf.String())
	}
}

// TestPrettyShowMessageRespectsNarrowTermWidth proves the rules size to
// min(termWidth, cap) — the same single width concession ShowNotes makes.
func TestPrettyShowMessageRespectsNarrowTermWidth(t *testing.T) {
	out := &bytes.Buffer{}
	p := presenter.NewPrettyPresenter(out, presenter.WithProfile(termenv.Ascii)).WithTermWidth(30)

	p.ShowMessage(commitMessage())

	lines := strings.Split(out.String(), "\n")
	if got := len([]rune(lines[0])); got != 30 {
		t.Errorf("opener rule width = %d runes, want the narrow terminal width 30", got)
	}
	if lines[len(lines)-2] != strings.Repeat("─", 30) {
		t.Errorf("closing rule = %q, want 30 rule chars", lines[len(lines)-2])
	}
}

// TestShowMessageBodyIsByteIdenticalAcrossModes proves the body region between
// the delimiters is the SAME bytes in both modes — both presenters write
// Message.Body through the shared writeNotesBody helper, so only the framing
// differs and "what previews is what ships" holds for message blocks too.
func TestShowMessageBodyIsByteIdenticalAcrossModes(t *testing.T) {
	m := commitMessage()

	plainOut := &bytes.Buffer{}
	presenter.NewPlainPresenter(plainOut, &bytes.Buffer{}).ShowMessage(m)
	prettyOut := &bytes.Buffer{}
	presenter.NewPrettyPresenter(prettyOut, presenter.WithProfile(termenv.Ascii)).ShowMessage(m)

	// extract slices the body region positionally: everything after the first
	// line (the opener) up to the trailing closer line.
	extract := func(s, closer string) string {
		afterOpener := s[strings.Index(s, "\n")+1:]
		return strings.TrimSuffix(afterOpener, closer)
	}
	plainBody := extract(plainOut.String(), "--- end commit message ---\n")
	prettyBody := extract(prettyOut.String(), strings.Repeat("─", presenter.RuleCapForTest)+"\n")

	want := m.Body + "\n"
	if plainBody != want {
		t.Errorf("plain body region = %q, want the verbatim source %q", plainBody, want)
	}
	if prettyBody != want {
		t.Errorf("pretty body region = %q, want the verbatim source %q", prettyBody, want)
	}
	if plainBody != prettyBody {
		t.Errorf("body regions differ across modes: plain %q vs pretty %q", plainBody, prettyBody)
	}
}
