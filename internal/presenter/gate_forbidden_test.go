package presenter_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// The forbidden-combination rule: when -y is absent and stdin is NOT a TTY, an
// interactive gate cannot be answered, so Prompt must FAIL LOUD rather than block
// on a stdin read that never returns. The failure surfaces THROUGH the presenter
// (styled in pretty, terse in plain — render mode comes from stdout, independent
// of the stdin problem) and the one-line summary ALSO goes to stderr; Prompt
// returns the ErrNotInteractive sentinel and the presenter sets no exit code.

// TestPlainForbiddenComboFailsWithoutReadingStdin proves the plain presenter, with
// -y absent and stdinInteractive=false, fails the gate WITHOUT reading the input
// stream: the failingReader fails the test on any Read, so a returned error with
// the reader untripped proves no blocking stdin read occurred.
func TestPlainForbiddenComboFailsWithoutReadingStdin(t *testing.T) {
	gate := presenter.NotesReviewGate()
	out := &bytes.Buffer{}
	reader := &failingReader{t: t}
	p := presenter.NewPlainPresenterWithInput(out, &bytes.Buffer{}, reader).WithInteractiveStdin(false)

	choice, err := p.Prompt(gate)
	if err == nil {
		t.Fatalf("plain Prompt (non-TTY stdin, no -y) returned nil error; want fail-loud")
	}
	if choice != "" {
		t.Errorf("plain Prompt forbidden-combo returned choice %q; want zero choice", choice)
	}
	if reader.tripped {
		t.Error("plain Prompt forbidden-combo read the input reader; it must NOT read stdin on this path")
	}
}

// TestPlainForbiddenComboTerseFailureToOut proves the plain presenter renders the
// terse FAILED line "gate: FAILED - not a TTY; pass -y to run unattended" to out,
// reusing the established plain "{label}: FAILED - {message}" vocabulary with the
// fixed "gate" label and the ASCII message form (no em-dash).
func TestPlainForbiddenComboTerseFailureToOut(t *testing.T) {
	gate := presenter.NotesReviewGate()
	out := &bytes.Buffer{}
	p := presenter.NewPlainPresenterWithInput(out, &bytes.Buffer{}, strings.NewReader("")).WithInteractiveStdin(false)

	if _, err := p.Prompt(gate); err == nil {
		t.Fatalf("plain Prompt (non-TTY stdin, no -y) returned nil error; want fail-loud")
	}
	want := "gate: FAILED - not a TTY; pass -y to run unattended\n"
	if got := out.String(); got != want {
		t.Errorf("plain forbidden-combo out = %q, want %q", got, want)
	}
}

// TestPlainForbiddenComboFailureIsBytePureASCII guards the plain byte-purity
// contract for the synthesised forbidden-combination failure: no ESC, no CR,
// nothing above the printable ASCII range (in particular, the message uses the
// ASCII "; pass" form, never the em-dash).
func TestPlainForbiddenComboFailureIsBytePureASCII(t *testing.T) {
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	p := presenter.NewPlainPresenterWithInput(out, errBuf, strings.NewReader("")).WithInteractiveStdin(false)
	if _, err := p.Prompt(presenter.NotesReviewGate()); err == nil {
		t.Fatalf("plain Prompt (non-TTY stdin, no -y) returned nil error; want fail-loud")
	}

	assertBytePureASCIIStreams(t, "plain forbidden-combo failure", out, errBuf)
}

// TestPrettyForbiddenComboStyledFailureToOut proves the pretty presenter renders
// the styled "✗ gate  not a TTY — pass -y to run unattended" line to out under a
// colour-capable profile: ESC present (the ✗ glyph is styled) and the message in
// the spec's em-dash form, mirroring StageFailed's "✗ {label}  {message}" shape.
func TestPrettyForbiddenComboStyledFailureToOut(t *testing.T) {
	gate := presenter.NotesReviewGate()
	out := &bytes.Buffer{}
	reader := &failingReader{t: t}
	p := presenter.NewPrettyPresenter(out, presenter.WithProfile(termenv.TrueColor), presenter.WithInput(reader)).WithInteractiveStdin(false)

	if _, err := p.Prompt(gate); err == nil {
		t.Fatalf("pretty Prompt (non-TTY stdin, no -y) returned nil error; want fail-loud")
	}
	got := out.String()
	if !bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("colour-on forbidden-combo failure emitted no ESC (0x1b) — expected styled ✗:\n%q", got)
	}
	for _, want := range []string{"✗", "gate", "not a TTY — pass -y to run unattended"} {
		if !strings.Contains(got, want) {
			t.Errorf("colour-on forbidden-combo failure missing %q:\n%q", want, got)
		}
	}
	if reader.tripped {
		t.Error("pretty Prompt forbidden-combo read the input reader; it must NOT read stdin on this path")
	}
}

// TestPrettyForbiddenComboFailureShape locks the unstyled structure of the pretty
// failure line under the Ascii profile (no ANSI): the exact
// "  ✗ gate       not a TTY — pass -y to run unattended\n" line, padStage-aligned
// like the established StageFailed rendering.
func TestPrettyForbiddenComboFailureShape(t *testing.T) {
	gate := presenter.NotesReviewGate()
	out := &bytes.Buffer{}
	p := presenter.NewPrettyPresenter(out, presenter.WithProfile(termenv.Ascii), presenter.WithInput(strings.NewReader(""))).WithInteractiveStdin(false)

	if _, err := p.Prompt(gate); err == nil {
		t.Fatalf("pretty Prompt (non-TTY stdin, no -y) returned nil error; want fail-loud")
	}
	// padStage("gate") right-pads "gate" (4) to stageColumn (11) = 7 trailing
	// spaces, exactly like every other StageFailed line.
	want := "  ✗ gate       not a TTY — pass -y to run unattended\n"
	if got := out.String(); got != want {
		t.Errorf("pretty forbidden-combo out = %q, want %q", got, want)
	}
}

// TestForbiddenComboSummaryToStderrBothModes proves the one-line failure summary
// ALSO reaches stderr in BOTH modes per the stream contract.
func TestForbiddenComboSummaryToStderrBothModes(t *testing.T) {
	gate := presenter.NotesReviewGate()

	plainOut := &bytes.Buffer{}
	plainErr := &bytes.Buffer{}
	plain := presenter.NewPlainPresenterWithInput(plainOut, plainErr, strings.NewReader("")).WithInteractiveStdin(false)
	if _, err := plain.Prompt(gate); err == nil {
		t.Fatalf("plain Prompt (non-TTY stdin, no -y) returned nil error; want fail-loud")
	}
	if got := plainErr.String(); got != "gate: FAILED - not a TTY; pass -y to run unattended\n" {
		t.Errorf("plain forbidden-combo stderr = %q, want the one-line FAILED summary", got)
	}

	prettyOut := &bytes.Buffer{}
	prettyErr := &bytes.Buffer{}
	pretty := presenter.NewPrettyPresenter(prettyOut, presenter.WithErr(prettyErr), presenter.WithProfile(termenv.Ascii), presenter.WithInput(strings.NewReader(""))).
		WithInteractiveStdin(false)
	if _, err := pretty.Prompt(gate); err == nil {
		t.Fatalf("pretty Prompt (non-TTY stdin, no -y) returned nil error; want fail-loud")
	}
	if got := prettyErr.String(); got != "✗ gate  not a TTY — pass -y to run unattended\n" {
		t.Errorf("pretty forbidden-combo stderr = %q, want the unstyled one-line ✗ summary", got)
	}
}

// TestForbiddenComboRenderModeIndependentOfStdin proves the axes are orthogonal:
// the render mode is selected from stdout (a colour-capable pretty presenter)
// independently of the non-TTY STDIN that triggers the failure, so the failure
// renders STYLED (ESC present) even though stdin is the non-interactive side.
func TestForbiddenComboRenderModeIndependentOfStdin(t *testing.T) {
	gate := presenter.NotesReviewGate()
	out := &bytes.Buffer{}
	p := presenter.NewPrettyPresenter(out, presenter.WithProfile(termenv.TrueColor), presenter.WithInput(strings.NewReader(""))).WithInteractiveStdin(false)

	if _, err := p.Prompt(gate); err == nil {
		t.Fatalf("pretty Prompt (non-TTY stdin, no -y) returned nil error; want fail-loud")
	}
	if !bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("pretty render mode (from stdout) did not style the failure despite non-TTY stdin:\n%q", out.String())
	}
}

// TestForbiddenComboReturnsErrNotInteractive proves Prompt returns the exported
// ErrNotInteractive sentinel on this path, in both modes, so the engine can map it
// to a non-zero exit.
func TestForbiddenComboReturnsErrNotInteractive(t *testing.T) {
	gate := presenter.NotesReviewGate()

	plain := presenter.NewPlainPresenterWithInput(&bytes.Buffer{}, &bytes.Buffer{}, strings.NewReader("")).WithInteractiveStdin(false)
	if _, err := plain.Prompt(gate); !errors.Is(err, presenter.ErrNotInteractive) {
		t.Errorf("plain forbidden-combo err = %v, want errors.Is(..., ErrNotInteractive)", err)
	}

	pretty := presenter.NewPrettyPresenter(&bytes.Buffer{}, presenter.WithProfile(termenv.Ascii), presenter.WithInput(strings.NewReader(""))).WithInteractiveStdin(false)
	if _, err := pretty.Prompt(gate); !errors.Is(err, presenter.ErrNotInteractive) {
		t.Errorf("pretty forbidden-combo err = %v, want errors.Is(..., ErrNotInteractive)", err)
	}
}

// TestYesBypassesForbiddenComboBothModes proves precedence branch 1: -y present
// auto-accepts even with stdinInteractive=false — no forbidden-combination
// failure, the -y echo is emitted, and Prompt returns the gate default with a nil
// error (the failingReader proves no stdin read on either path).
func TestYesBypassesForbiddenComboBothModes(t *testing.T) {
	gate := presenter.NotesReviewGate()

	plainOut := &bytes.Buffer{}
	plainReader := &failingReader{t: t}
	plain := presenter.NewPlainPresenterWithInput(plainOut, &bytes.Buffer{}, plainReader).
		WithYes(true).WithInteractiveStdin(false)
	choice, err := plain.Prompt(gate)
	if err != nil {
		t.Fatalf("plain Prompt (-y, non-TTY stdin) returned error: %v", err)
	}
	if choice != gate.Default {
		t.Errorf("plain Prompt (-y, non-TTY stdin) = %q, want gate default %q", choice, gate.Default)
	}
	if got := plainOut.String(); got != "notes: accepted (-y)\n" {
		t.Errorf("plain -y echo = %q, want the auto-accept echo, NOT a failure", got)
	}

	prettyOut := &bytes.Buffer{}
	prettyReader := &failingReader{t: t}
	pretty := presenter.NewPrettyPresenter(prettyOut, presenter.WithProfile(termenv.Ascii), presenter.WithInput(prettyReader)).
		WithYes(true).WithInteractiveStdin(false)
	pchoice, perr := pretty.Prompt(gate)
	if perr != nil {
		t.Fatalf("pretty Prompt (-y, non-TTY stdin) returned error: %v", perr)
	}
	if pchoice != gate.Default {
		t.Errorf("pretty Prompt (-y, non-TTY stdin) = %q, want gate default %q", pchoice, gate.Default)
	}
	if got := prettyOut.String(); got != "  ✓ notes  accepted (-y)\n" {
		t.Errorf("pretty -y accept line = %q, want the auto-accept line, NOT a failure", got)
	}
}

// TestInteractiveStdinKeepsInteractivePathBothModes proves precedence branch 3:
// with -y absent and stdinInteractive=true the interactive line-read path is
// UNCHANGED — the scripted "y\n" returns ChoiceYes and the menu is drawn — so the
// new fail branch only fires on the genuine forbidden combination.
func TestInteractiveStdinKeepsInteractivePathBothModes(t *testing.T) {
	gate := presenter.NotesReviewGate()

	plainOut := &bytes.Buffer{}
	plain := presenter.NewPlainPresenterWithInput(plainOut, &bytes.Buffer{}, strings.NewReader("y\n")).
		WithInteractiveStdin(true)
	choice, err := plain.Prompt(gate)
	if err != nil {
		t.Fatalf("plain Prompt (interactive stdin) returned error: %v", err)
	}
	if choice != presenter.ChoiceYes {
		t.Errorf("plain Prompt (interactive stdin) = %q, want %q", choice, presenter.ChoiceYes)
	}
	if !strings.Contains(plainOut.String(), "Continue?") {
		t.Errorf("plain Prompt (interactive stdin) did not render the menu:\n%q", plainOut.String())
	}

	prettyOut := &bytes.Buffer{}
	pretty := presenter.NewPrettyPresenter(prettyOut, presenter.WithProfile(termenv.Ascii), presenter.WithInput(strings.NewReader("y\n"))).
		WithInteractiveStdin(true)
	pchoice, perr := pretty.Prompt(gate)
	if perr != nil {
		t.Fatalf("pretty Prompt (interactive stdin) returned error: %v", perr)
	}
	if pchoice != presenter.ChoiceYes {
		t.Errorf("pretty Prompt (interactive stdin) = %q, want %q", pchoice, presenter.ChoiceYes)
	}
	if !strings.Contains(prettyOut.String(), "Continue? ›") {
		t.Errorf("pretty Prompt (interactive stdin) did not render the menu:\n%q", prettyOut.String())
	}
}

// TestConstructorsDefaultStdinInteractive proves the constructors default
// stdinInteractive=true: a presenter built WITHOUT calling WithInteractiveStdin
// (and without -y) still hits the interactive loop, NOT the new fail path — the
// default that keeps the existing interactive-path tests green.
func TestConstructorsDefaultStdinInteractive(t *testing.T) {
	gate := presenter.NotesReviewGate()

	plainOut := &bytes.Buffer{}
	plain := presenter.NewPlainPresenterWithInput(plainOut, &bytes.Buffer{}, strings.NewReader("y\n"))
	choice, err := plain.Prompt(gate)
	if err != nil {
		t.Fatalf("plain Prompt (default stdinInteractive) returned error: %v", err)
	}
	if choice != presenter.ChoiceYes {
		t.Errorf("plain Prompt (default stdinInteractive) = %q, want %q (interactive path, not fail)", choice, presenter.ChoiceYes)
	}

	prettyOut := &bytes.Buffer{}
	pretty := presenter.NewPrettyPresenter(prettyOut, presenter.WithProfile(termenv.Ascii), presenter.WithInput(strings.NewReader("y\n")))
	pchoice, perr := pretty.Prompt(gate)
	if perr != nil {
		t.Fatalf("pretty Prompt (default stdinInteractive) returned error: %v", perr)
	}
	if pchoice != presenter.ChoiceYes {
		t.Errorf("pretty Prompt (default stdinInteractive) = %q, want %q (interactive path, not fail)", pchoice, presenter.ChoiceYes)
	}
}
