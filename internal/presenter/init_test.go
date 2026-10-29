package presenter_test

import (
	"strings"
	"testing"

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
// "created", "skipped (", ")") are byte-pure ASCII — no glyph ever appears in plain
// output. Targets and the reason here are ASCII so the whole run is asserted
// byte-pure.
func TestPlainPresenterInitResultEmitsNoANSIGlyphOrAnimationBytes(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.InitResult(presenter.InitOutcome{Action: presenter.InitCreated, Target: ".mint.toml"})
		p.InitResult(presenter.InitOutcome{Action: presenter.InitSkipped, Target: "release", Reason: "exists, use --force"})
	})

	assertBytePureASCII(t, out, "plain init output")
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

// TestInitForceOverwriteNarratesAsCreated proves a --force overwrite is narrated as
// a created line: the ENGINE supplies InitCreated for an overwrite (the presenter
// does not special-case --force, has no knowledge of it, and renders the created
// vocabulary).
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
	for _, marker := range []string{"Use these notes?", "[y/n"} {
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
