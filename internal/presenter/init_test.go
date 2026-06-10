package presenter_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
)

// TestInitActionStringIsReadable proves InitAction has a readable String() so test
// output and any narration that prints the action is legible rather than a bare
// integer. The zero value is InitCreated (a typed action the engine always sets
// explicitly), and InitSkipped is the other member.
func TestInitActionStringIsReadable(t *testing.T) {
	tests := []struct {
		action presenter.InitAction
		want   string
	}{
		{action: presenter.InitCreated, want: "created"},
		{action: presenter.InitSkipped, want: "skipped"},
	}

	for _, tt := range tests {
		if got := tt.action.String(); got != tt.want {
			t.Errorf("InitAction(%d).String() = %q, want %q", tt.action, got, tt.want)
		}
	}
}

// TestInitOutcomeCarriesEngineResolvedFields proves the InitOutcome payload carries
// the engine-resolved action, target, and reason as fields — the presenter renders
// what it is handed and never resolves created-vs-skipped or knows --force.
func TestInitOutcomeCarriesEngineResolvedFields(t *testing.T) {
	o := presenter.InitOutcome{Action: presenter.InitSkipped, Target: "release", Reason: "exists, use --force"}

	if o.Action != presenter.InitSkipped {
		t.Errorf("Action = %v, want %v", o.Action, presenter.InitSkipped)
	}
	if o.Target != "release" {
		t.Errorf("Target = %q, want %q", o.Target, "release")
	}
	if o.Reason != "exists, use --force" {
		t.Errorf("Reason = %q, want %q", o.Reason, "exists, use --force")
	}
}

// TestPlainPresenterInitCreatedRendersCreatedLine is the core plain created
// acceptance: a created outcome renders "{target}: created" — byte-pure ASCII, no
// glyph, the action word after the target.
func TestPlainPresenterInitCreatedRendersCreatedLine(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.InitResult(presenter.InitOutcome{Action: presenter.InitCreated, Target: ".mint.toml"})
	})

	want := ".mint.toml: created\n"
	if got := out.String(); got != want {
		t.Errorf("InitResult created = %q, want %q", got, want)
	}
}

// TestPlainPresenterInitSkippedRendersSkippedLineWithReason is the core plain
// skipped acceptance: a skipped outcome renders "{target}: skipped ({reason})" with
// the engine-supplied reason rendered VERBATIM — the presenter synthesises no reason
// text.
func TestPlainPresenterInitSkippedRendersSkippedLineWithReason(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.InitResult(presenter.InitOutcome{Action: presenter.InitSkipped, Target: "release", Reason: "exists, use --force"})
	})

	want := "release: skipped (exists, use --force)\n"
	if got := out.String(); got != want {
		t.Errorf("InitResult skipped = %q, want %q", got, want)
	}
}

// TestPlainPresenterInitResultWritesToStdoutOnly proves the init narration is
// narration → out only and never duplicates to err (init has no failure/warning
// semantics — its created/skipped lines are plain narration).
func TestPlainPresenterInitResultWritesToStdoutOnly(t *testing.T) {
	out, errBuf := drive(func(p *presenter.PlainPresenter) {
		p.InitResult(presenter.InitOutcome{Action: presenter.InitCreated, Target: ".mint.toml"})
		p.InitResult(presenter.InitOutcome{Action: presenter.InitSkipped, Target: "release", Reason: "exists, use --force"})
	})

	if errBuf.Len() != 0 {
		t.Errorf("InitResult wrote to err: %q", errBuf.String())
	}
	if out.Len() == 0 {
		t.Error("InitResult wrote nothing to out")
	}
}

// TestPlainPresenterInitResultEmitsNoANSIGlyphOrAnimationBytes guards the
// byte-purity contract for the plain init lines: the synthesised parts ("{target}: ",
// "created", "skipped (", ")") are byte-pure ASCII — the pretty "·" middot and the
// "✓" glyph are PRETTY-only and must never appear in plain output. Targets and the
// reason here are ASCII so the whole run is asserted byte-pure.
func TestPlainPresenterInitResultEmitsNoANSIGlyphOrAnimationBytes(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.InitResult(presenter.InitOutcome{Action: presenter.InitCreated, Target: ".mint.toml"})
		p.InitResult(presenter.InitOutcome{Action: presenter.InitSkipped, Target: "release", Reason: "exists, use --force"})
	})

	assertBytePureASCII(t, out, "plain init output")
}

// TestPrettyPresenterInitCreatedRendersCreatedLine is the core pretty created
// acceptance: a created outcome renders "  ✓ created {target}" — two-space indent,
// the green ✓ (success style), then the action word "created", then the target. The
// word order is {glyph} {action-word} {target}, which differs from plain's
// {target}: {action-word}. Asserted under the no-colour profile so the layout/glyph
// is exact.
func TestPrettyPresenterInitCreatedRendersCreatedLine(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.InitResult(presenter.InitOutcome{Action: presenter.InitCreated, Target: ".mint.toml"})
	})

	want := "  ✓ created .mint.toml\n"
	if got := out.String(); got != want {
		t.Errorf("InitResult created = %q, want %q", got, want)
	}
}

// TestPrettyPresenterInitSkippedRendersMiddotNoticeWithReason is the core pretty
// skipped acceptance: a skipped outcome renders "  · skipped {target} ({reason})" —
// two-space indent, the NEUTRAL middot "·" (U+00B7, NOT ✓/✗/⚠/↩ since a skip is
// neither success nor failure), then "skipped", the target, then " ({reason})" with
// the reason rendered VERBATIM.
func TestPrettyPresenterInitSkippedRendersMiddotNoticeWithReason(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.InitResult(presenter.InitOutcome{Action: presenter.InitSkipped, Target: "release", Reason: "exists, use --force"})
	})

	want := "  · skipped release (exists, use --force)\n"
	if got := out.String(); got != want {
		t.Errorf("InitResult skipped = %q, want %q", got, want)
	}
}

// TestPrettyPresenterInitResultLayoutSurvivesColourDowngrade forces the no-colour
// profile and asserts the exact init layout for both outcomes — the glyph, indent,
// and middot survive a colour downgrade as bare text with no SGR codes leaking.
func TestPrettyPresenterInitResultLayoutSurvivesColourDowngrade(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.InitResult(presenter.InitOutcome{Action: presenter.InitCreated, Target: ".mint.toml"})
		p.InitResult(presenter.InitOutcome{Action: presenter.InitSkipped, Target: "release", Reason: "exists, use --force"})
	})

	if bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("downgraded init output leaked an SGR code:\n%q", out.String())
	}
	want := "  ✓ created .mint.toml\n" +
		"  · skipped release (exists, use --force)\n"
	if got := out.String(); got != want {
		t.Errorf("downgraded init output = %q, want %q", got, want)
	}
}

// TestPrettyPresenterInitCreatedColourOnEmitsANSI forces a colour-capable profile
// and asserts the created line carries ANSI SGR escapes (the green ✓ styling) while
// the layout text — the indent, the glyph, the action word, and the target —
// survives.
func TestPrettyPresenterInitCreatedColourOnEmitsANSI(t *testing.T) {
	out := drivePretty(termenv.TrueColor, func(p *presenter.PrettyPresenter) {
		p.InitResult(presenter.InitOutcome{Action: presenter.InitCreated, Target: ".mint.toml"})
	})

	got := out.String()
	if !bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("colour-on init created line contains no ESC (0x1b) — expected green SGR codes:\n%q", got)
	}
	// Strip the colour codes and assert the COMPLETE deterministic line — the whole
	// indent + glyph + action word + target survives the styling, asserted exactly
	// rather than by bare inner fragments.
	if stripped, want := stripANSI(got), "  ✓ created .mint.toml\n"; stripped != want {
		t.Errorf("colour-on init created line (ANSI stripped) = %q, want %q", stripped, want)
	}
	if !strings.Contains(got, "  \x1b[") {
		t.Errorf("two-space indent before the styled glyph missing:\n%q", got)
	}
}

// TestPrettyPresenterInitSkippedColourOnEmitsANSI forces a colour-capable profile
// and asserts the skipped line carries ANSI SGR escapes (the dim middot styling)
// while the middot, action word, target, and verbatim reason survive.
func TestPrettyPresenterInitSkippedColourOnEmitsANSI(t *testing.T) {
	out := drivePretty(termenv.TrueColor, func(p *presenter.PrettyPresenter) {
		p.InitResult(presenter.InitOutcome{Action: presenter.InitSkipped, Target: "release", Reason: "exists, use --force"})
	})

	got := out.String()
	if !bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("colour-on init skipped line contains no ESC (0x1b) — expected dim SGR codes:\n%q", got)
	}
	// Strip the colour codes and assert the COMPLETE deterministic line — the whole
	// indent + middot + action word + target + verbatim reason survives the styling,
	// asserted exactly rather than by bare inner fragments.
	if stripped, want := stripANSI(got), "  · skipped release (exists, use --force)\n"; stripped != want {
		t.Errorf("colour-on init skipped line (ANSI stripped) = %q, want %q", stripped, want)
	}
}

// TestPlainPresenterInitAllCreatedRunRendersOnlyCreatedLines covers the all-created
// edge: two InitCreated outcomes render two created lines and no skipped notice.
func TestPlainPresenterInitAllCreatedRunRendersOnlyCreatedLines(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.InitResult(presenter.InitOutcome{Action: presenter.InitCreated, Target: ".mint.toml"})
		p.InitResult(presenter.InitOutcome{Action: presenter.InitCreated, Target: "release"})
	})

	want := ".mint.toml: created\n" +
		"release: created\n"
	got := out.String()
	if got != want {
		t.Errorf("all-created run = %q, want %q", got, want)
	}
	if strings.Contains(got, "skipped") {
		t.Errorf("all-created run contains a skipped notice: %q", got)
	}
}

// TestPlainPresenterInitAllSkippedRunRendersOnlySkippedNotices covers the
// all-skipped edge: two InitSkipped outcomes render two skipped lines and no created
// line.
func TestPlainPresenterInitAllSkippedRunRendersOnlySkippedNotices(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.InitResult(presenter.InitOutcome{Action: presenter.InitSkipped, Target: ".mint.toml", Reason: "exists, use --force"})
		p.InitResult(presenter.InitOutcome{Action: presenter.InitSkipped, Target: "release", Reason: "exists, use --force"})
	})

	want := ".mint.toml: skipped (exists, use --force)\n" +
		"release: skipped (exists, use --force)\n"
	got := out.String()
	if got != want {
		t.Errorf("all-skipped run = %q, want %q", got, want)
	}
	if strings.Contains(got, "created") {
		t.Errorf("all-skipped run contains a created line: %q", got)
	}
}

// TestPlainPresenterInitMixedRunRendersInEmitOrder covers the mixed edge: a created
// outcome then a skipped outcome render both lines independently, in the order the
// engine emitted them.
func TestPlainPresenterInitMixedRunRendersInEmitOrder(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.InitResult(presenter.InitOutcome{Action: presenter.InitCreated, Target: ".mint.toml"})
		p.InitResult(presenter.InitOutcome{Action: presenter.InitSkipped, Target: "release", Reason: "exists, use --force"})
	})

	want := ".mint.toml: created\n" +
		"release: skipped (exists, use --force)\n"
	if got := out.String(); got != want {
		t.Errorf("mixed run = %q, want %q", got, want)
	}
}

// TestPrettyPresenterInitMixedRunRendersInEmitOrder covers the mixed edge in pretty:
// a created line then a skipped notice, in emit order, under the no-colour profile so
// the exact layout is asserted.
func TestPrettyPresenterInitMixedRunRendersInEmitOrder(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.InitResult(presenter.InitOutcome{Action: presenter.InitCreated, Target: ".mint.toml"})
		p.InitResult(presenter.InitOutcome{Action: presenter.InitSkipped, Target: "release", Reason: "exists, use --force"})
	})

	want := "  ✓ created .mint.toml\n" +
		"  · skipped release (exists, use --force)\n"
	if got := out.String(); got != want {
		t.Errorf("mixed run = %q, want %q", got, want)
	}
}

// TestInitForceOverwriteNarratesAsCreated proves a --force overwrite is narrated as
// a created line: the ENGINE supplies InitCreated for an overwrite (the presenter
// does not special-case --force, has no knowledge of it, and renders the created
// vocabulary). Asserted in both modes from the same payload.
func TestInitForceOverwriteNarratesAsCreated(t *testing.T) {
	// The engine resolved a --force overwrite to a created action; the payload carries
	// no --force marker — the presenter renders the created vocabulary unconditionally.
	overwrite := presenter.InitOutcome{Action: presenter.InitCreated, Target: ".mint.toml"}

	plainOut, _ := drive(func(p *presenter.PlainPresenter) {
		p.InitResult(overwrite)
	})
	if got, want := plainOut.String(), ".mint.toml: created\n"; got != want {
		t.Errorf("plain --force overwrite = %q, want %q", got, want)
	}

	prettyOut := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.InitResult(overwrite)
	})
	if got, want := prettyOut.String(), "  ✓ created .mint.toml\n"; got != want {
		t.Errorf("pretty --force overwrite = %q, want %q", got, want)
	}
}

// TestPlainPresenterInitRunEmitsNoReleaseFooterOrGate drives an init run (InitResult
// only) and asserts init emits NO release-style brand footer / "done:" line and NO
// gate: the run ends on the last outcome line. The engine simply never calls
// RunFinished or Prompt for init — the presenter does not special-case init.
func TestPlainPresenterInitRunEmitsNoReleaseFooterOrGate(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.InitResult(presenter.InitOutcome{Action: presenter.InitCreated, Target: ".mint.toml"})
		p.InitResult(presenter.InitOutcome{Action: presenter.InitSkipped, Target: "release", Reason: "exists, use --force"})
	})

	got := out.String()
	// The run ends on the last outcome line — the skipped notice.
	if !strings.HasSuffix(got, "release: skipped (exists, use --force)\n") {
		t.Errorf("init run does not end on the last outcome line:\n%q", got)
	}
	if strings.Contains(got, "done:") {
		t.Errorf("init run emitted a release-style \"done:\" footer:\n%q", got)
	}
	if strings.Contains(got, "🌿") {
		t.Errorf("init run emitted a release-style brand footer:\n%q", got)
	}
	for _, marker := range []string{"Continue?", "[y/n", "accept & proceed"} {
		if strings.Contains(got, marker) {
			t.Errorf("init run emitted gate marker %q:\n%q", marker, got)
		}
	}
}

// TestPrettyPresenterInitRunEmitsNoReleaseFooterOrGate is the pretty counterpart:
// an init run (InitResult only) ends on the last outcome line with no "🌿 released"
// brand footer and no "Continue? ›" gate/menu.
func TestPrettyPresenterInitRunEmitsNoReleaseFooterOrGate(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.InitResult(presenter.InitOutcome{Action: presenter.InitCreated, Target: ".mint.toml"})
		p.InitResult(presenter.InitOutcome{Action: presenter.InitSkipped, Target: "release", Reason: "exists, use --force"})
	})

	got := out.String()
	if !strings.HasSuffix(got, "  · skipped release (exists, use --force)\n") {
		t.Errorf("init run does not end on the last outcome line:\n%q", got)
	}
	if strings.Contains(got, "released") {
		t.Errorf("init run emitted a release-style \"released\" brand footer:\n%q", got)
	}
	for _, marker := range []string{"Continue?", "›", "accept & proceed"} {
		if strings.Contains(got, marker) {
			t.Errorf("init run emitted gate marker %q:\n%q", marker, got)
		}
	}
}

// TestRecordingPresenterRecordsInitResult proves the recorder captures the full
// InitOutcome payload — action, target, reason — so an engine-driven test can
// round-trip the init outcome independent of any rendering.
func TestRecordingPresenterRecordsInitResult(t *testing.T) {
	rec := &presentertest.RecordingPresenter{}
	o := presenter.InitOutcome{Action: presenter.InitSkipped, Target: "release", Reason: "exists, use --force"}

	rec.InitResult(o)

	ev, ok := rec.At(0)
	if !ok {
		t.Fatal("expected one recorded event, got none")
	}
	if ev.Kind != presentertest.KindInitResult {
		t.Fatalf("Kind = %v, want %v", ev.Kind, presentertest.KindInitResult)
	}
	if ev.InitResult != o {
		t.Errorf("InitResult = %+v, want %+v", ev.InitResult, o)
	}
	if ev.InitResult.Action != presenter.InitSkipped {
		t.Errorf("Action = %v, want %v", ev.InitResult.Action, presenter.InitSkipped)
	}
	if ev.InitResult.Target != "release" {
		t.Errorf("Target = %q, want %q", ev.InitResult.Target, "release")
	}
	if ev.InitResult.Reason != "exists, use --force" {
		t.Errorf("Reason = %q, want %q", ev.InitResult.Reason, "exists, use --force")
	}
}
