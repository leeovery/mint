package presenter_test

import (
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
	reader := &failingReader{t: t}
	p, out, _ := plainGate(reader, gateOpts{yes: true})

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
// rendered under -y: no "[y/n/e/r]" hint and no "Use these notes?" question reach
// out.
func TestPlainPromptUnderYesDrawsNoMenu(t *testing.T) {
	gate := presenter.NotesReviewGate()
	p, out, _ := plainGate(strings.NewReader(""), gateOpts{yes: true})

	if _, err := p.Prompt(gate); err != nil {
		t.Fatalf("plain Prompt under -y returned error: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "Use these notes?") {
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
	p, _, errBuf := plainGate(strings.NewReader(""), gateOpts{yes: true})

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
	reader := &failingReader{t: t}
	p, out, _ := plainGate(reader, gateOpts{yes: true})

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
	if strings.Contains(out.String(), "Use these notes?") {
		t.Errorf("plain reuse -y drew the menu; it must be skipped:\n%q", out.String())
	}
}

// TestPlainPromptInteractivePathUnchangedWhenNotYes is the regression guard: with
// yes=false the plain Prompt still renders the menu and reads input, returning the
// scripted choice — the interactive path is UNCHANGED.
func TestPlainPromptInteractivePathUnchangedWhenNotYes(t *testing.T) {
	gate := presenter.NotesReviewGate()
	p, out, _ := plainGate(strings.NewReader("y\n"), gateOpts{})

	choice, err := p.Prompt(gate)
	if err != nil {
		t.Fatalf("plain Prompt (yes=false) returned error: %v", err)
	}
	if choice != presenter.ChoiceYes {
		t.Errorf("plain Prompt (yes=false) = %q, want %q", choice, presenter.ChoiceYes)
	}
	if !strings.Contains(out.String(), "Use these notes?") {
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
	p, out, _ := plainGate(strings.NewReader(""), gateOpts{yes: true})
	if _, err := p.Prompt(presenter.NotesReviewGate()); err != nil {
		t.Fatalf("plain Prompt under -y returned error: %v", err)
	}

	assertBytePureASCII(t, out, "plain -y echo")
}

// TestRailPromptUnderYesTakesTheDefaultAskingNothing proves the Rail, under -y,
// returns each gate's declared default without reading the input, draws none of its
// keys, writes nothing to err, and records the auto-accept on the rail.
func TestRailPromptUnderYesTakesTheDefaultAskingNothing(t *testing.T) {
	for _, gate := range []presenter.Gate{presenter.NotesReviewGate(), presenter.ReuseConfirmGate()} {
		t.Run(gate.Question+" "+string(gate.Keys()[len(gate.Keys())-1]), func(t *testing.T) {
			reader := &failingReader{t: t}
			p, out, errBuf := railGate(termenv.Ascii, reader, gateOpts{yes: true})
			defer p.Close()

			choice, err := p.Prompt(gate)
			if err != nil {
				t.Fatalf("rail Prompt under -y returned error: %v", err)
			}
			if choice != gate.Default {
				t.Errorf("rail Prompt under -y = %q, want gate default %q", choice, gate.Default)
			}
			if reader.tripped {
				t.Error("rail Prompt under -y read the input reader; it must not")
			}
			if errBuf.Len() != 0 {
				t.Errorf("rail -y wrote to stderr %q; the auto-accept is narration → stdout only", errBuf.String())
			}
			got := out.String()
			if !strings.Contains(got, "accepted (-y)") {
				t.Errorf("rail -y drew no auto-accept:\n%q", got)
			}
			for _, c := range gate.Choices {
				if key := string(c.Key) + " " + c.Action; strings.Contains(got, key) {
					t.Errorf("rail -y drew the key %q; the gate must be skipped:\n%q", key, got)
				}
			}
		})
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
// "github" yields "source: github (-y)" (plain), and the Rail records the answer on a
// node named for the subject, so a renderer hardcoding either word would fail.
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

	plain, out, _ := plainGate(strings.NewReader(""), gateOpts{yes: true})
	if _, err := plain.Prompt(gate); err != nil {
		t.Fatalf("plain Prompt under -y returned error: %v", err)
	}
	if got := out.String(); got != "source: github (-y)\n" {
		t.Errorf("plain subject echo = %q, want %q", got, "source: github (-y)\n")
	}

	rail, railOut, _ := railGate(termenv.Ascii, strings.NewReader(""), gateOpts{yes: true})
	defer rail.Close()
	if _, err := rail.Prompt(gate); err != nil {
		t.Fatalf("rail Prompt under -y returned error: %v", err)
	}
	if got := railOut.String(); !strings.Contains(got, "source") || !strings.Contains(got, "github · accepted (-y)") {
		t.Errorf("rail subject accept = %q, want a source node reading %q", got, "github · accepted (-y)")
	}
}

// assert failingReader implements io.Reader.
var _ io.Reader = (*failingReader)(nil)
