package presenter_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// The regenerate source/target prompts are expressed as GATES so they flow through
// the SAME Prompt(gate) method as the notes gate: the shared line-read loop (3-3),
// the -y skip+echo (3-5, generalised to echo the chosen value), and the
// forbidden-combination fail-loud path (3-6). These tests prove the wiring + the
// -y value echo + the plain key:value rendering — NOT a parallel implementation.

// sourceOptions and targetOptions are the illustrative enumerated choice sets the
// engine supplies as GateChoice keys (no free-form value entry).
func sourceOptions() []presenter.GateChoice {
	return []presenter.GateChoice{
		{Key: presenter.Choice("github"), Action: "GitHub"},
		{Key: presenter.Choice("gitlab"), Action: "GitLab"},
	}
}

func targetOptions() []presenter.GateChoice {
	return []presenter.GateChoice{
		{Key: presenter.Choice("stable"), Action: "stable channel"},
		{Key: presenter.Choice("beta"), Action: "beta channel"},
	}
}

// TestSourceGateCarriesEngineSuppliedOptionsAndDefault proves SourceGate builds a
// Gate from the engine-supplied options and default — the presenter does not invent
// them. Subject is "source"; AcceptEcho is the chosen default value.
func TestSourceGateCarriesEngineSuppliedOptionsAndDefault(t *testing.T) {
	gate := presenter.SourceGate(sourceOptions(), presenter.Choice("github"))

	if gate.Subject != "source" {
		t.Errorf("SourceGate().Subject = %q, want %q", gate.Subject, "source")
	}
	if gate.Default != presenter.Choice("github") {
		t.Errorf("SourceGate().Default = %q, want %q", gate.Default, "github")
	}
	if gate.AcceptEcho != "github" {
		t.Errorf("SourceGate().AcceptEcho = %q, want the chosen default %q", gate.AcceptEcho, "github")
	}
	wantKeys := []presenter.Choice{"github", "gitlab"}
	if got := gate.Keys(); len(got) != len(wantKeys) || got[0] != wantKeys[0] || got[1] != wantKeys[1] {
		t.Errorf("SourceGate().Keys() = %v, want %v (engine-supplied options in order)", got, wantKeys)
	}
}

// TestTargetGateCarriesEngineSuppliedOptionsAndDefault proves TargetGate builds a
// Gate from the engine-supplied options and default — Subject "target", AcceptEcho
// the chosen default value.
func TestTargetGateCarriesEngineSuppliedOptionsAndDefault(t *testing.T) {
	gate := presenter.TargetGate(targetOptions(), presenter.Choice("stable"))

	if gate.Subject != "target" {
		t.Errorf("TargetGate().Subject = %q, want %q", gate.Subject, "target")
	}
	if gate.Default != presenter.Choice("stable") {
		t.Errorf("TargetGate().Default = %q, want %q", gate.Default, "stable")
	}
	if gate.AcceptEcho != "stable" {
		t.Errorf("TargetGate().AcceptEcho = %q, want the chosen default %q", gate.AcceptEcho, "stable")
	}
	wantKeys := []presenter.Choice{"stable", "beta"}
	if got := gate.Keys(); len(got) != len(wantKeys) || got[0] != wantKeys[0] || got[1] != wantKeys[1] {
		t.Errorf("TargetGate().Keys() = %v, want %v (engine-supplied options in order)", got, wantKeys)
	}
}

// TestSourceGateUnrecognisedInputRePromptsThenAccepts proves the source prompt
// reuses the shared line-read loop: scripted "bogus\ngithub\n" re-prompts once on
// the unrecognised "bogus" then accepts the valid "github" — the prompt is rendered
// TWICE (once before the bogus read, once before the valid read).
func TestSourceGateUnrecognisedInputRePromptsThenAccepts(t *testing.T) {
	gate := presenter.SourceGate(sourceOptions(), presenter.Choice("github"))
	out := &bytes.Buffer{}
	p := presenter.NewPlainPresenterWithInput(out, &bytes.Buffer{}, strings.NewReader("bogus\ngithub\n"))

	choice, err := p.Prompt(gate)
	if err != nil {
		t.Fatalf("plain source Prompt returned error: %v", err)
	}
	if choice != presenter.Choice("github") {
		t.Errorf("source Prompt after re-prompt = %q, want %q", choice, "github")
	}
	if n := strings.Count(out.String(), "Source?"); n != 2 {
		t.Errorf("source prompt rendered %d times, want 2 (one re-prompt after the unrecognised input):\n%q", n, out.String())
	}
}

// TestPlainSourceTargetRenderTerseKeyValueLines proves plain renders the source and
// target prompts as terse key:value-style prompt lines with the option-key hint —
// NOT the pretty vertical menu. The hint is built from the declared option keys.
func TestPlainSourceTargetRenderTerseKeyValueLines(t *testing.T) {
	srcOut := &bytes.Buffer{}
	src := presenter.NewPlainPresenterWithInput(srcOut, &bytes.Buffer{}, strings.NewReader("github\n"))
	if _, err := src.Prompt(presenter.SourceGate(sourceOptions(), presenter.Choice("github"))); err != nil {
		t.Fatalf("plain source Prompt returned error: %v", err)
	}
	if got := srcOut.String(); got != "Source? [github/gitlab]\n" {
		t.Errorf("plain source prompt = %q, want terse key:value line %q", got, "Source? [github/gitlab]\n")
	}

	tgtOut := &bytes.Buffer{}
	tgt := presenter.NewPlainPresenterWithInput(tgtOut, &bytes.Buffer{}, strings.NewReader("stable\n"))
	if _, err := tgt.Prompt(presenter.TargetGate(targetOptions(), presenter.Choice("stable"))); err != nil {
		t.Fatalf("plain target Prompt returned error: %v", err)
	}
	if got := tgtOut.String(); got != "Target? [stable/beta]\n" {
		t.Errorf("plain target prompt = %q, want terse key:value line %q", got, "Target? [stable/beta]\n")
	}
	// No pretty vertical-menu artefacts in plain (no four-space option indent, no ›).
	for _, buf := range []*bytes.Buffer{srcOut, tgtOut} {
		if strings.Contains(buf.String(), "›") {
			t.Errorf("plain prompt drew the pretty cursor marker:\n%q", buf.String())
		}
		if strings.Contains(buf.String(), "    github") || strings.Contains(buf.String(), "    stable") {
			t.Errorf("plain prompt drew an indented option line (pretty menu) in plain:\n%q", buf.String())
		}
	}
}

// TestYesEchoesChosenSourcePlainAndPretty proves the -y echo shows the CHOSEN value
// (not "accepted") for the source gate: plain "source: github (-y)" and pretty
// "  ✓ source  github (-y)"; the gate's declared default is returned WITHOUT reading
// stdin (the failingReader fails the test on any Read).
func TestYesEchoesChosenSourcePlainAndPretty(t *testing.T) {
	gate := presenter.SourceGate(sourceOptions(), presenter.Choice("github"))

	plainOut := &bytes.Buffer{}
	plainReader := &failingReader{t: t}
	plain := presenter.NewPlainPresenterWithInput(plainOut, &bytes.Buffer{}, plainReader).WithYes(true)
	choice, err := plain.Prompt(gate)
	if err != nil {
		t.Fatalf("plain source Prompt under -y returned error: %v", err)
	}
	if choice != presenter.Choice("github") {
		t.Errorf("plain source Prompt under -y = %q, want declared default %q", choice, "github")
	}
	if got := plainOut.String(); got != "source: github (-y)\n" {
		t.Errorf("plain source -y echo = %q, want %q", got, "source: github (-y)\n")
	}

	prettyOut := &bytes.Buffer{}
	prettyReader := &failingReader{t: t}
	pretty := presenter.NewPrettyPresenter(prettyOut, presenter.WithProfile(termenv.Ascii), presenter.WithInput(prettyReader)).WithYes(true)
	pchoice, perr := pretty.Prompt(gate)
	if perr != nil {
		t.Fatalf("pretty source Prompt under -y returned error: %v", perr)
	}
	if pchoice != presenter.Choice("github") {
		t.Errorf("pretty source Prompt under -y = %q, want declared default %q", pchoice, "github")
	}
	if got := prettyOut.String(); got != "  ✓ source  github (-y)\n" {
		t.Errorf("pretty source -y accept line = %q, want %q", got, "  ✓ source  github (-y)\n")
	}
}

// TestYesEchoesChosenTargetPlain proves the -y echo for the target gate shows the
// chosen target value: plain "target: stable (-y)".
func TestYesEchoesChosenTargetPlain(t *testing.T) {
	gate := presenter.TargetGate(targetOptions(), presenter.Choice("stable"))
	out := &bytes.Buffer{}
	reader := &failingReader{t: t}
	p := presenter.NewPlainPresenterWithInput(out, &bytes.Buffer{}, reader).WithYes(true)

	choice, err := p.Prompt(gate)
	if err != nil {
		t.Fatalf("plain target Prompt under -y returned error: %v", err)
	}
	if choice != presenter.Choice("stable") {
		t.Errorf("plain target Prompt under -y = %q, want %q", choice, "stable")
	}
	if got := out.String(); got != "target: stable (-y)\n" {
		t.Errorf("plain target -y echo = %q, want %q", got, "target: stable (-y)\n")
	}
}

// TestYesSourceEchoIsBytePureASCII guards that the source -y echo stays byte-pure
// ASCII (the option keys are ASCII identifiers).
func TestYesSourceEchoIsBytePureASCII(t *testing.T) {
	out := &bytes.Buffer{}
	p := presenter.NewPlainPresenterWithInput(out, &bytes.Buffer{}, strings.NewReader("")).WithYes(true)
	if _, err := p.Prompt(presenter.SourceGate(sourceOptions(), presenter.Choice("github"))); err != nil {
		t.Fatalf("plain source Prompt under -y returned error: %v", err)
	}

	assertBytePureASCII(t, out, "plain source -y echo")
}

// TestSourceGateForbiddenComboFailsLoud proves the source gate reuses the
// forbidden-combination path: -y absent and stdinInteractive=false fails loud
// (terse FAILED to out, the one-line summary to stderr, ErrNotInteractive) WITHOUT
// reading stdin.
func TestSourceGateForbiddenComboFailsLoud(t *testing.T) {
	gate := presenter.SourceGate(sourceOptions(), presenter.Choice("github"))
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	reader := &failingReader{t: t}
	p := presenter.NewPlainPresenterWithInput(out, errBuf, reader).WithInteractiveStdin(false)

	choice, err := p.Prompt(gate)
	if !errors.Is(err, presenter.ErrNotInteractive) {
		t.Fatalf("source forbidden-combo err = %v, want errors.Is(..., ErrNotInteractive)", err)
	}
	if choice != "" {
		t.Errorf("source forbidden-combo returned choice %q; want zero choice", choice)
	}
	if reader.tripped {
		t.Error("source forbidden-combo read the input reader; it must NOT read stdin on this path")
	}
	want := "gate: FAILED - not a TTY; pass -y to run unattended\n"
	if got := out.String(); got != want {
		t.Errorf("source forbidden-combo out = %q, want %q", got, want)
	}
	if got := errBuf.String(); got != want {
		t.Errorf("source forbidden-combo stderr = %q, want the one-line FAILED summary %q", got, want)
	}
}

// TestSourceGateFlagDefaultUsedWhenSkipped proves a DIFFERENT engine-supplied
// default is used when skipped under -y: SourceGate(opts, "gitlab") returns
// Choice("gitlab") and echoes "source: gitlab (-y)" — the flag/default value, not a
// hardcoded one.
func TestSourceGateFlagDefaultUsedWhenSkipped(t *testing.T) {
	gate := presenter.SourceGate(sourceOptions(), presenter.Choice("gitlab"))
	out := &bytes.Buffer{}
	reader := &failingReader{t: t}
	p := presenter.NewPlainPresenterWithInput(out, &bytes.Buffer{}, reader).WithYes(true)

	choice, err := p.Prompt(gate)
	if err != nil {
		t.Fatalf("plain source Prompt under -y returned error: %v", err)
	}
	if choice != presenter.Choice("gitlab") {
		t.Errorf("source Prompt under -y = %q, want the engine-supplied default %q", choice, "gitlab")
	}
	if got := out.String(); got != "source: gitlab (-y)\n" {
		t.Errorf("source -y echo = %q, want %q", got, "source: gitlab (-y)\n")
	}
}

// TestSourceGateCaseInsensitiveAndEmptyEnterDefault proves the shared parse applies:
// "GITHUB\n" maps case-insensitively to Choice("github"); a bare "\n" selects the
// declared default.
func TestSourceGateCaseInsensitiveAndEmptyEnterDefault(t *testing.T) {
	upperOut := &bytes.Buffer{}
	upper := presenter.NewPlainPresenterWithInput(upperOut, &bytes.Buffer{}, strings.NewReader("GITHUB\n"))
	choice, err := upper.Prompt(presenter.SourceGate(sourceOptions(), presenter.Choice("github")))
	if err != nil {
		t.Fatalf("plain source Prompt (uppercase) returned error: %v", err)
	}
	if choice != presenter.Choice("github") {
		t.Errorf("source Prompt(\"GITHUB\") = %q, want case-insensitive %q", choice, "github")
	}

	emptyOut := &bytes.Buffer{}
	empty := presenter.NewPlainPresenterWithInput(emptyOut, &bytes.Buffer{}, strings.NewReader("\n"))
	dchoice, derr := empty.Prompt(presenter.SourceGate(sourceOptions(), presenter.Choice("gitlab")))
	if derr != nil {
		t.Fatalf("plain source Prompt (empty Enter) returned error: %v", derr)
	}
	if dchoice != presenter.Choice("gitlab") {
		t.Errorf("source Prompt(empty Enter) = %q, want the declared default %q", dchoice, "gitlab")
	}
}

// TestNotesReuseEchoUnchangedAfterGeneralisation is the regression guard for the
// generalised echo: NotesReviewGate and ReuseConfirmGate set AcceptEcho="accepted",
// so under -y they still emit "notes: accepted (-y)" (plain) — the 3-5 behaviour is
// unchanged.
func TestNotesReuseEchoUnchangedAfterGeneralisation(t *testing.T) {
	if got := presenter.NotesReviewGate().AcceptEcho; got != "accepted" {
		t.Errorf("NotesReviewGate().AcceptEcho = %q, want %q", got, "accepted")
	}
	if got := presenter.ReuseConfirmGate().AcceptEcho; got != "accepted" {
		t.Errorf("ReuseConfirmGate().AcceptEcho = %q, want %q", got, "accepted")
	}

	out := &bytes.Buffer{}
	p := presenter.NewPlainPresenterWithInput(out, &bytes.Buffer{}, strings.NewReader("")).WithYes(true)
	if _, err := p.Prompt(presenter.NotesReviewGate()); err != nil {
		t.Fatalf("plain notes Prompt under -y returned error: %v", err)
	}
	if got := out.String(); got != "notes: accepted (-y)\n" {
		t.Errorf("notes -y echo after generalisation = %q, want unchanged %q", got, "notes: accepted (-y)\n")
	}
}
