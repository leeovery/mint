package presenter_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// failingReader is an io.Reader that fails the test if it is read from at all. It
// proves the -y skip path NEVER touches the gate input stream: under -y the
// presenter must not render the menu nor read a byte, so any Read here is a bug.
type failingReader struct {
	t       *testing.T
	tripped bool
}

func (r *failingReader) Read(p []byte) (int, error) {
	r.tripped = true
	r.t.Errorf("input reader was read under -y skip; the gate must NOT read stdin")
	return 0, errors.New("failingReader: must not be read under -y")
}

// TestPlainPromptSkipsGateUnderYesEchoesAcceptedToOut proves the plain presenter,
// under -y, emits exactly "notes: accepted (-y)\n" to out, returns the gate's
// declared default, and reads NOTHING from the input reader.
func TestPlainPromptSkipsGateUnderYesEchoesAcceptedToOut(t *testing.T) {
	gate := presenter.NotesReviewGate()
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	reader := &failingReader{t: t}
	p := presenter.NewPlainPresenterWithInput(out, errBuf, reader).WithYes(true)

	choice, err := p.Prompt(gate)
	if err != nil {
		t.Fatalf("plain Prompt under -y returned error: %v", err)
	}
	if choice != gate.Default {
		t.Errorf("plain Prompt under -y = %q, want gate default %q", choice, gate.Default)
	}
	if got := out.String(); got != "notes: accepted (-y)\n" {
		t.Errorf("plain -y echo = %q, want %q", got, "notes: accepted (-y)\n")
	}
	if reader.tripped {
		t.Error("plain Prompt under -y read the input reader; it must not")
	}
}

// TestPlainPromptUnderYesDrawsNoMenu proves the plain terse prompt line is NOT
// rendered under -y: no "[y/n/e/r]" hint and no "Continue?" question reach out.
func TestPlainPromptUnderYesDrawsNoMenu(t *testing.T) {
	gate := presenter.NotesReviewGate()
	out := &bytes.Buffer{}
	p := presenter.NewPlainPresenterWithInput(out, &bytes.Buffer{}, strings.NewReader("")).WithYes(true)

	if _, err := p.Prompt(gate); err != nil {
		t.Fatalf("plain Prompt under -y returned error: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "Continue?") {
		t.Errorf("plain -y output drew the question line; the gate must be skipped:\n%q", got)
	}
	if strings.Contains(got, "[y/n/e/r]") {
		t.Errorf("plain -y output drew the key hint; the gate must be skipped:\n%q", got)
	}
}

// TestPlainPromptUnderYesEchoesStdoutOnly proves the auto-accept echo is narration
// (stdout) only — stderr is EMPTY after a -y Prompt.
func TestPlainPromptUnderYesEchoesStdoutOnly(t *testing.T) {
	gate := presenter.NotesReviewGate()
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	p := presenter.NewPlainPresenterWithInput(out, errBuf, strings.NewReader("")).WithYes(true)

	if _, err := p.Prompt(gate); err != nil {
		t.Fatalf("plain Prompt under -y returned error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Errorf("plain -y echo wrote to stderr %q; it is narration → stdout only", errBuf.String())
	}
}

// TestPlainReuseConfirmAutoAcceptedUnderYes proves the two-choice reuse confirm is
// auto-accepted under -y exactly like the notes gate: returns ChoiceYes, draws no
// menu, and emits the SAME "notes: accepted (-y)" echo (subject "notes").
func TestPlainReuseConfirmAutoAcceptedUnderYes(t *testing.T) {
	gate := presenter.ReuseConfirmGate()
	out := &bytes.Buffer{}
	reader := &failingReader{t: t}
	p := presenter.NewPlainPresenterWithInput(out, &bytes.Buffer{}, reader).WithYes(true)

	choice, err := p.Prompt(gate)
	if err != nil {
		t.Fatalf("plain reuse Prompt under -y returned error: %v", err)
	}
	if choice != presenter.ChoiceYes {
		t.Errorf("plain reuse Prompt under -y = %q, want %q", choice, presenter.ChoiceYes)
	}
	if got := out.String(); got != "notes: accepted (-y)\n" {
		t.Errorf("plain reuse -y echo = %q, want %q", got, "notes: accepted (-y)\n")
	}
	if strings.Contains(out.String(), "Continue?") {
		t.Errorf("plain reuse -y drew the menu; it must be skipped:\n%q", out.String())
	}
}

// TestPlainPromptInteractivePathUnchangedWhenNotYes is the regression guard: with
// yes=false the plain Prompt still renders the menu and reads input, returning the
// scripted choice — the interactive path is UNCHANGED.
func TestPlainPromptInteractivePathUnchangedWhenNotYes(t *testing.T) {
	gate := presenter.NotesReviewGate()
	out := &bytes.Buffer{}
	p := presenter.NewPlainPresenterWithInput(out, &bytes.Buffer{}, strings.NewReader("y\n"))

	choice, err := p.Prompt(gate)
	if err != nil {
		t.Fatalf("plain Prompt (yes=false) returned error: %v", err)
	}
	if choice != presenter.ChoiceYes {
		t.Errorf("plain Prompt (yes=false) = %q, want %q", choice, presenter.ChoiceYes)
	}
	if !strings.Contains(out.String(), "Continue?") {
		t.Errorf("plain Prompt (yes=false) did not render the menu:\n%q", out.String())
	}
	if strings.Contains(out.String(), "accepted (-y)") {
		t.Errorf("plain Prompt (yes=false) wrongly emitted the -y echo:\n%q", out.String())
	}
}

// TestPlainPromptUnderYesEchoIsBytePureASCII guards the byte-purity contract for
// the synthesised "{subject}: accepted (-y)" echo: it must be pure ASCII — no ESC,
// no CR, nothing above the printable range — mirroring the other plain guards.
func TestPlainPromptUnderYesEchoIsBytePureASCII(t *testing.T) {
	out := &bytes.Buffer{}
	p := presenter.NewPlainPresenterWithInput(out, &bytes.Buffer{}, strings.NewReader("")).WithYes(true)
	if _, err := p.Prompt(presenter.NotesReviewGate()); err != nil {
		t.Fatalf("plain Prompt under -y returned error: %v", err)
	}

	for i, b := range out.Bytes() {
		switch {
		case b == 0x1b:
			t.Errorf("byte %d is ESC (0x1b) — ANSI escape leaked into the plain -y echo", i)
		case b == 0x0d:
			t.Errorf("byte %d is CR (0x0d) — carriage-return leaked into the plain -y echo", i)
		case b == '\n':
			// the only permitted control byte: a line terminator
		case b < 0x20 || b > 0x7e:
			t.Errorf("byte %d = 0x%02x is outside the printable ASCII range the plain -y echo uses", i, b)
		}
	}
}

// TestPrettyPromptSkipsGateUnderYesEchoesAcceptLine proves the pretty presenter,
// under -y, emits the concise accept line "  ✓ notes  accepted (-y)\n" to out,
// returns the gate's declared default, and reads NOTHING from the input reader.
func TestPrettyPromptSkipsGateUnderYesEchoesAcceptLine(t *testing.T) {
	gate := presenter.NotesReviewGate()
	out := &bytes.Buffer{}
	reader := &failingReader{t: t}
	p := presenter.NewPrettyPresenterWithInput(out, termenv.Ascii, reader).WithYes(true)

	choice, err := p.Prompt(gate)
	if err != nil {
		t.Fatalf("pretty Prompt under -y returned error: %v", err)
	}
	if choice != gate.Default {
		t.Errorf("pretty Prompt under -y = %q, want gate default %q", choice, gate.Default)
	}
	if got := out.String(); got != "  ✓ notes  accepted (-y)\n" {
		t.Errorf("pretty -y accept line = %q, want %q", got, "  ✓ notes  accepted (-y)\n")
	}
	if reader.tripped {
		t.Error("pretty Prompt under -y read the input reader; it must not")
	}
}

// TestPrettyPromptUnderYesDrawsNoMenu proves the pretty vertical menu is NOT drawn
// under -y: no "Continue? ›" prompt and no option lines reach out.
func TestPrettyPromptUnderYesDrawsNoMenu(t *testing.T) {
	gate := presenter.NotesReviewGate()
	out := &bytes.Buffer{}
	p := presenter.NewPrettyPresenterWithInput(out, termenv.Ascii, strings.NewReader("")).WithYes(true)

	if _, err := p.Prompt(gate); err != nil {
		t.Fatalf("pretty Prompt under -y returned error: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "Continue? ›") {
		t.Errorf("pretty -y drew the prompt line; the gate must be skipped:\n%q", got)
	}
	for _, opt := range []string{"accept & proceed", "abort", "edit in $EDITOR", "regenerate"} {
		if strings.Contains(got, opt) {
			t.Errorf("pretty -y drew option line %q; the gate must be skipped:\n%q", opt, got)
		}
	}
}

// TestPrettyPromptUnderYesEchoesStdoutOnly proves the pretty auto-accept line is
// narration (stdout) only — the err stream is EMPTY after a -y Prompt.
func TestPrettyPromptUnderYesEchoesStdoutOnly(t *testing.T) {
	gate := presenter.NotesReviewGate()
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	p := presenter.NewPrettyPresenterWithErr(out, errBuf, termenv.Ascii).WithYes(true).WithInput(strings.NewReader(""))

	if _, err := p.Prompt(gate); err != nil {
		t.Fatalf("pretty Prompt under -y returned error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Errorf("pretty -y accept line wrote to stderr %q; it is narration → stdout only", errBuf.String())
	}
}

// TestPrettyReuseConfirmAutoAcceptedUnderYes proves the two-choice reuse confirm is
// auto-accepted under -y exactly like the notes gate: returns ChoiceYes, draws no
// menu, and emits the SAME "  ✓ notes  accepted (-y)" accept line (subject "notes").
func TestPrettyReuseConfirmAutoAcceptedUnderYes(t *testing.T) {
	gate := presenter.ReuseConfirmGate()
	out := &bytes.Buffer{}
	reader := &failingReader{t: t}
	p := presenter.NewPrettyPresenterWithInput(out, termenv.Ascii, reader).WithYes(true)

	choice, err := p.Prompt(gate)
	if err != nil {
		t.Fatalf("pretty reuse Prompt under -y returned error: %v", err)
	}
	if choice != presenter.ChoiceYes {
		t.Errorf("pretty reuse Prompt under -y = %q, want %q", choice, presenter.ChoiceYes)
	}
	if got := out.String(); got != "  ✓ notes  accepted (-y)\n" {
		t.Errorf("pretty reuse -y accept line = %q, want %q", got, "  ✓ notes  accepted (-y)\n")
	}
	if strings.Contains(out.String(), "Continue? ›") {
		t.Errorf("pretty reuse -y drew the menu; it must be skipped:\n%q", out.String())
	}
}

// TestPrettyPromptInteractivePathUnchangedWhenNotYes is the regression guard: with
// yes=false the pretty Prompt still renders the vertical menu and reads input,
// returning the scripted choice — the interactive path is UNCHANGED.
func TestPrettyPromptInteractivePathUnchangedWhenNotYes(t *testing.T) {
	gate := presenter.NotesReviewGate()
	out := &bytes.Buffer{}
	p := presenter.NewPrettyPresenterWithInput(out, termenv.Ascii, strings.NewReader("y\n"))

	choice, err := p.Prompt(gate)
	if err != nil {
		t.Fatalf("pretty Prompt (yes=false) returned error: %v", err)
	}
	if choice != presenter.ChoiceYes {
		t.Errorf("pretty Prompt (yes=false) = %q, want %q", choice, presenter.ChoiceYes)
	}
	if !strings.Contains(out.String(), "Continue? ›") {
		t.Errorf("pretty Prompt (yes=false) did not render the menu:\n%q", out.String())
	}
	if strings.Contains(out.String(), "accepted (-y)") {
		t.Errorf("pretty Prompt (yes=false) wrongly emitted the -y accept line:\n%q", out.String())
	}
}

// TestPrettyYesAcceptLineColourOnStylesGlyph proves the pretty accept line under a
// colour-capable profile styles the ✓ glyph (ANSI present) while the "notes
// accepted (-y)" structure survives as contiguous substrings.
func TestPrettyYesAcceptLineColourOnStylesGlyph(t *testing.T) {
	gate := presenter.NotesReviewGate()
	out := &bytes.Buffer{}
	p := presenter.NewPrettyPresenterWithInput(out, termenv.TrueColor, strings.NewReader("")).WithYes(true)

	if _, err := p.Prompt(gate); err != nil {
		t.Fatalf("pretty Prompt under -y returned error: %v", err)
	}
	if !bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("colour-on -y accept line emitted no ESC (0x1b) — expected styled ✓:\n%q", out.String())
	}
	for _, want := range []string{"✓", "notes", "accepted (-y)"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("colour-on -y accept line missing %q:\n%q", want, out.String())
		}
	}
}

// TestGateSubjectSetByConstructors proves both gate constructors set Subject to
// "notes" so the presenter renders "{Subject}: accepted (-y)" from the payload
// rather than hardcoding the word.
func TestGateSubjectSetByConstructors(t *testing.T) {
	if got := presenter.NotesReviewGate().Subject; got != "notes" {
		t.Errorf("NotesReviewGate().Subject = %q, want %q", got, "notes")
	}
	if got := presenter.ReuseConfirmGate().Subject; got != "notes" {
		t.Errorf("ReuseConfirmGate().Subject = %q, want %q", got, "notes")
	}
}

// TestPromptEchoesGateSubjectAndAcceptEchoNotHardcoded proves the presenter renders
// BOTH halves of the -y echo from the gate payload — gate.Subject AND gate.AcceptEcho
// — not hardcoded "notes"/"accepted". A gate carrying subject "source" and echo word
// "github" yields "source: github (-y)" (plain) / "  ✓ source  github (-y)" (pretty),
// so a renderer hardcoding either word would fail.
func TestPromptEchoesGateSubjectAndAcceptEchoNotHardcoded(t *testing.T) {
	gate := presenter.Gate{
		Question:   "Source?",
		Subject:    "source",
		AcceptEcho: "github",
		Choices: []presenter.GateChoice{
			{Key: presenter.Choice("github"), Action: "GitHub"},
			{Key: presenter.Choice("gitlab"), Action: "GitLab"},
		},
		Default: presenter.Choice("github"),
	}

	out := &bytes.Buffer{}
	plain := presenter.NewPlainPresenterWithInput(out, &bytes.Buffer{}, strings.NewReader("")).WithYes(true)
	if _, err := plain.Prompt(gate); err != nil {
		t.Fatalf("plain Prompt under -y returned error: %v", err)
	}
	if got := out.String(); got != "source: github (-y)\n" {
		t.Errorf("plain subject echo = %q, want %q", got, "source: github (-y)\n")
	}

	prettyOut := &bytes.Buffer{}
	pretty := presenter.NewPrettyPresenterWithInput(prettyOut, termenv.Ascii, strings.NewReader("")).WithYes(true)
	if _, err := pretty.Prompt(gate); err != nil {
		t.Fatalf("pretty Prompt under -y returned error: %v", err)
	}
	if got := prettyOut.String(); got != "  ✓ source  github (-y)\n" {
		t.Errorf("pretty subject accept line = %q, want %q", got, "  ✓ source  github (-y)\n")
	}
}

// assert failingReader implements io.Reader.
var _ io.Reader = (*failingReader)(nil)
