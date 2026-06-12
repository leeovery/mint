package presenter_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// TestPlainAskLineReturnsTypedLineVerbatim proves the plain free-text ask renders
// the terse "{prompt}: " framing (no trailing newline — the cursor sits after the
// colon), returns the typed line with only its terminator stripped, and writes
// nothing to err.
func TestPlainAskLineReturnsTypedLineVerbatim(t *testing.T) {
	p, out, errBuf := plainGate(strings.NewReader("fix the flaky retry loop\n"), gateOpts{})

	got, err := p.AskLine("context")
	if err != nil {
		t.Fatalf("plain AskLine returned error: %v", err)
	}
	if got != "fix the flaky retry loop" {
		t.Errorf("plain AskLine = %q, want %q", got, "fix the flaky retry loop")
	}
	if out.String() != "context: " {
		t.Errorf("plain AskLine prompt = %q, want %q", out.String(), "context: ")
	}
	if errBuf.Len() != 0 {
		t.Errorf("plain AskLine wrote to err: %q; the prompt is narration -> out only", errBuf.String())
	}
	assertBytePureASCIIStreams(t, "plain askline", out, errBuf)
}

// TestPlainAskLineEmptyEnterReturnsEmptyString proves a deliberate empty Enter is
// a legal answer: the empty string with a nil error — never an error, never an
// invented default.
func TestPlainAskLineEmptyEnterReturnsEmptyString(t *testing.T) {
	p, _, _ := plainGate(strings.NewReader("\n"), gateOpts{})

	got, err := p.AskLine("context")
	if err != nil {
		t.Fatalf("plain AskLine on empty Enter returned error: %v", err)
	}
	if got != "" {
		t.Errorf("plain AskLine on empty Enter = %q, want empty string", got)
	}
}

// TestPlainAskLinePreservesWhitespaceWithinLine proves the answer is verbatim
// beyond terminator-stripping: leading, inner, and trailing spaces all survive —
// the engine owns interpretation, the presenter does not trim.
func TestPlainAskLinePreservesWhitespaceWithinLine(t *testing.T) {
	p, _, _ := plainGate(strings.NewReader("  spaced  out \n"), gateOpts{})

	got, err := p.AskLine("context")
	if err != nil {
		t.Fatalf("plain AskLine returned error: %v", err)
	}
	if got != "  spaced  out " {
		t.Errorf("plain AskLine = %q, want %q (whitespace preserved)", got, "  spaced  out ")
	}
}

// TestPlainAskLineStripsCRLFTerminator proves a CRLF-terminated line loses both
// terminator bytes — the \r is part of the terminator, not the answer.
func TestPlainAskLineStripsCRLFTerminator(t *testing.T) {
	p, _, _ := plainGate(strings.NewReader("value\r\n"), gateOpts{})

	got, err := p.AskLine("context")
	if err != nil {
		t.Fatalf("plain AskLine returned error: %v", err)
	}
	if got != "value" {
		t.Errorf("plain AskLine = %q, want %q (CRLF stripped)", got, "value")
	}
}

// TestPlainAskLineFinalLineWithoutNewlineIsReturned proves a final line that
// arrives alongside EOF (no trailing newline) is still the answer — mirroring the
// gate loop's "y then EOF" handling.
func TestPlainAskLineFinalLineWithoutNewlineIsReturned(t *testing.T) {
	p, _, _ := plainGate(strings.NewReader("partial"), gateOpts{})

	got, err := p.AskLine("context")
	if err != nil {
		t.Fatalf("plain AskLine on partial final line returned error: %v", err)
	}
	if got != "partial" {
		t.Errorf("plain AskLine = %q, want %q", got, "partial")
	}
}

// TestPlainAskLineEOFWithNoLineFailsWithErrInputClosed proves a closed stream is
// never mistaken for a deliberate empty answer: EOF with no bytes returns the
// exported ErrInputClosed, and (unlike the forbidden combination) renders NO
// failure — the engine owns that failure's surfacing.
func TestPlainAskLineEOFWithNoLineFailsWithErrInputClosed(t *testing.T) {
	p, out, errBuf := plainGate(strings.NewReader(""), gateOpts{})

	_, err := p.AskLine("context")
	if !errors.Is(err, presenter.ErrInputClosed) {
		t.Fatalf("plain AskLine on EOF = %v, want ErrInputClosed", err)
	}
	if out.String() != "context: " {
		t.Errorf("plain AskLine out = %q, want just the prompt %q (EOF renders no failure)", out.String(), "context: ")
	}
	if errBuf.Len() != 0 {
		t.Errorf("plain AskLine on EOF wrote to err: %q; ErrInputClosed is unrendered", errBuf.String())
	}
}

// TestPlainAskLineNonInteractiveStdinFailsLoud proves the forbidden-combination
// rule covers the free-text seam too: a non-interactive stdin fails loud with the
// "input" label — rendered to out AND duplicated to err, exactly like the gate
// failure — returns ErrNotInteractive, and never touches the input stream.
func TestPlainAskLineNonInteractiveStdinFailsLoud(t *testing.T) {
	reader := &failingReader{t: t}
	p, out, errBuf := plainGate(reader, gateOpts{nonInteractiveStdin: true})

	_, err := p.AskLine("context")
	if !errors.Is(err, presenter.ErrNotInteractive) {
		t.Fatalf("plain AskLine non-interactive = %v, want ErrNotInteractive", err)
	}
	want := "input: FAILED - not a TTY; pass -y to run unattended\n"
	if out.String() != want {
		t.Errorf("plain AskLine fail line (out) = %q, want %q", out.String(), want)
	}
	if errBuf.String() != want {
		t.Errorf("plain AskLine fail line (err) = %q, want %q", errBuf.String(), want)
	}
	if reader.tripped {
		t.Error("plain AskLine read the input reader on the fail path; it must not")
	}
	assertBytePureASCIIStreams(t, "plain askline non-interactive", out, errBuf)
}

// TestPlainPromptThenAskLineConsumeConsecutiveLines proves the free-text seam
// shares Prompt's single persistent buffered reader: a gate answer and a context
// line scripted as consecutive lines of one stream are consumed in order — the
// engine's r-then-context flow works against one stdin.
func TestPlainPromptThenAskLineConsumeConsecutiveLines(t *testing.T) {
	p, _, _ := plainGate(strings.NewReader("r\nuse the changelog tone\n"), gateOpts{})

	choice, err := p.Prompt(presenter.NotesReviewGate())
	if err != nil {
		t.Fatalf("plain Prompt returned error: %v", err)
	}
	if choice != presenter.ChoiceRegen {
		t.Fatalf("plain Prompt = %q, want %q", choice, presenter.ChoiceRegen)
	}
	got, err := p.AskLine("context")
	if err != nil {
		t.Fatalf("plain AskLine after Prompt returned error: %v", err)
	}
	if got != "use the changelog tone" {
		t.Errorf("plain AskLine after Prompt = %q, want %q (buffered reader shared)", got, "use the changelog tone")
	}
}

// TestPrettyAskLineRendersPromptMarkerAndReturnsLine proves the pretty free-text
// ask renders the gate-question vocabulary — "{prompt} › " flush-left with no
// trailing newline — and returns the typed line. Under the Ascii profile the dim
// marker downgrades to bare text, so the framing is asserted byte-exactly.
func TestPrettyAskLineRendersPromptMarkerAndReturnsLine(t *testing.T) {
	p, out, errBuf := prettyGate(termenv.Ascii, strings.NewReader("free text answer\n"), gateOpts{})

	got, err := p.AskLine("context")
	if err != nil {
		t.Fatalf("pretty AskLine returned error: %v", err)
	}
	if got != "free text answer" {
		t.Errorf("pretty AskLine = %q, want %q", got, "free text answer")
	}
	if out.String() != "context › " {
		t.Errorf("pretty AskLine prompt = %q, want %q", out.String(), "context › ")
	}
	if errBuf.Len() != 0 {
		t.Errorf("pretty AskLine wrote to err: %q; the prompt is narration -> out only", errBuf.String())
	}
}

// TestPrettyAskLineNonInteractiveStdinFailsLoud proves the pretty fail path
// mirrors the gate's forbidden-combination rendering with the "input" label: the
// styled ✗ line to out, the unstyled one-line summary to err, ErrNotInteractive
// returned, and no read of the input stream.
func TestPrettyAskLineNonInteractiveStdinFailsLoud(t *testing.T) {
	reader := &failingReader{t: t}
	p, out, errBuf := prettyGate(termenv.Ascii, reader, gateOpts{nonInteractiveStdin: true})

	_, err := p.AskLine("context")
	if !errors.Is(err, presenter.ErrNotInteractive) {
		t.Fatalf("pretty AskLine non-interactive = %v, want ErrNotInteractive", err)
	}
	wantOut := "✗ input      not a TTY — pass -y to run unattended\n"
	if out.String() != wantOut {
		t.Errorf("pretty AskLine fail line (out) = %q, want %q", out.String(), wantOut)
	}
	wantErr := "✗ input  not a TTY — pass -y to run unattended\n"
	if errBuf.String() != wantErr {
		t.Errorf("pretty AskLine fail line (err) = %q, want %q", errBuf.String(), wantErr)
	}
	if reader.tripped {
		t.Error("pretty AskLine read the input reader on the fail path; it must not")
	}
}

// TestPrettyAskLineEOFWithNoLineFailsWithErrInputClosed proves the pretty seam
// shares plain's EOF contract: ErrInputClosed, unrendered.
func TestPrettyAskLineEOFWithNoLineFailsWithErrInputClosed(t *testing.T) {
	p, _, errBuf := prettyGate(termenv.Ascii, strings.NewReader(""), gateOpts{})

	_, err := p.AskLine("context")
	if !errors.Is(err, presenter.ErrInputClosed) {
		t.Fatalf("pretty AskLine on EOF = %v, want ErrInputClosed", err)
	}
	if errBuf.Len() != 0 {
		t.Errorf("pretty AskLine on EOF wrote to err: %q; ErrInputClosed is unrendered", errBuf.String())
	}
}
