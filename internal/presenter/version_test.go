package presenter_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
)

// TestPlainPresenterShowVersionEmitsBareValue is the LOAD-BEARING plain contract:
// ShowVersion writes EXACTLY the bare value plus a single trailing newline —
// byte-for-byte. No "version:" prefix, no "v" prefix, no glyph, no ANSI, no extra
// lines or whitespace. This exact framing is what `$(mint version)` consumes.
func TestPlainPresenterShowVersionEmitsBareValue(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.ShowVersion(presenter.Version{Value: "1.4.0"})
	})

	want := "1.4.0\n"
	if got := out.String(); got != want {
		t.Errorf("ShowVersion plain = %q, want %q (byte-for-byte)", got, want)
	}
}

// TestPlainPresenterShowVersionAddsNoFramingBytes guards the plain framing
// contract byte-by-byte: the synthesised framing adds NOTHING around the value —
// no ESC (0x1b) ANSI byte, no 🌿 leaf, no "version:" prefix, no "v" prefix. The
// value itself is engine content (typical semver is byte-pure ASCII).
func TestPlainPresenterShowVersionAddsNoFramingBytes(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.ShowVersion(presenter.Version{Value: "1.4.0"})
	})

	got := out.String()
	if bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("ShowVersion plain leaked an ESC (0x1b) ANSI byte: %q", got)
	}
	if strings.Contains(got, "🌿") {
		t.Errorf("ShowVersion plain leaked the 🌿 leaf: %q", got)
	}
	if strings.Contains(got, "version:") {
		t.Errorf("ShowVersion plain leaked a \"version:\" prefix: %q", got)
	}
	if strings.HasPrefix(got, "v") {
		t.Errorf("ShowVersion plain leaked a \"v\" prefix: %q", got)
	}
}

// TestPlainPresenterShowVersionConsumedCleanlyByCommandSubstitution simulates
// `$(mint version)`: command substitution strips a SINGLE trailing newline. After
// trimming exactly one "\n", the captured result must equal the value with no
// extraneous bytes — no prefix, no ANSI, no second line.
func TestPlainPresenterShowVersionConsumedCleanlyByCommandSubstitution(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.ShowVersion(presenter.Version{Value: "1.4.0"})
	})

	// Command substitution strips exactly one trailing newline.
	captured := strings.TrimSuffix(out.String(), "\n")
	if captured != "1.4.0" {
		t.Errorf("$(mint version) captured = %q, want %q (no extra bytes)", captured, "1.4.0")
	}
}

// TestPlainPresenterShowVersionWritesToStdoutOnly proves the version value is
// narration → out ONLY and never duplicates to err (version carries no
// failure/warning semantics; its value is the product).
func TestPlainPresenterShowVersionWritesToStdoutOnly(t *testing.T) {
	out, errBuf := drive(func(p *presenter.PlainPresenter) {
		p.ShowVersion(presenter.Version{Value: "1.4.0"})
	})

	if errBuf.Len() != 0 {
		t.Errorf("ShowVersion wrote to err: %q", errBuf.String())
	}
	if out.Len() == 0 {
		t.Error("ShowVersion wrote nothing to out")
	}
}

// TestRailShowVersionWritesToStdoutOnly proves the Rail's dressed version line is
// narration → out ONLY and never duplicates to err, in both the colour-on and
// colour-off cases.
func TestRailShowVersionWritesToStdoutOnly(t *testing.T) {
	for _, profile := range []termenv.Profile{termenv.Ascii, termenv.TrueColor} {
		out := &bytes.Buffer{}
		errBuf := &bytes.Buffer{}
		p := presenter.NewRailPresenter(out, presenter.WithRailErr(errBuf), presenter.WithRailProfile(profile))

		p.ShowVersion(presenter.Version{Value: "1.4.0"})
		p.Close()

		if errBuf.Len() != 0 {
			t.Errorf("profile %v: ShowVersion wrote to err: %q", profile, errBuf.String())
		}
		if !strings.Contains(out.String(), "v1.4.0") {
			t.Errorf("profile %v: ShowVersion wrote no version to out: %q", profile, out.String())
		}
	}
}

// TestShowVersionEmitsNoFooterOrGate drives a version path (ShowVersion only) and
// asserts version emits NO release-style footer / "done:" line and draws NO gate:
// the run ends on the version line. The engine simply never calls RunFinished or
// Prompt for version — the presenter does not special-case version.
func TestShowVersionEmitsNoFooterOrGate(t *testing.T) {
	plainOut, _ := drive(func(p *presenter.PlainPresenter) {
		p.ShowVersion(presenter.Version{Value: "1.4.0"})
	})
	plainGot := plainOut.String()
	if plainGot != "1.4.0\n" {
		t.Errorf("plain version run = %q, want exactly the value line", plainGot)
	}
	if strings.Contains(plainGot, "done:") {
		t.Errorf("plain version run emitted a release-style \"done:\" footer:\n%q", plainGot)
	}

	railOut := &bytes.Buffer{}
	rail := presenter.NewRailPresenter(railOut, presenter.WithRailProfile(termenv.Ascii))
	rail.ShowVersion(presenter.Version{Value: "1.4.0"})
	rail.Close()
	railGot := railOut.String()
	for _, marker := range []string{"done:", "Use these notes?", "y accept", "◆", "released"} {
		if strings.Contains(plainGot, marker) || strings.Contains(railGot, marker) {
			t.Errorf("version run emitted gate/footer marker %q", marker)
		}
	}
}

// TestShowVersionDrawsNoGate proves the version path never invokes Prompt: driving
// a version render through the recorder yields a single ShowVersion event and no
// Prompt event. version has no interactive gate (it prints its value).
func TestShowVersionDrawsNoGate(t *testing.T) {
	rec := &presentertest.RecordingPresenter{}

	rec.ShowVersion(presenter.Version{Value: "1.4.0"})

	for _, k := range rec.Kinds() {
		if k == presentertest.KindPrompt {
			t.Errorf("version path recorded a Prompt event — version draws no gate")
		}
	}
	if len(rec.Kinds()) != 1 || rec.Kinds()[0] != presentertest.KindShowVersion {
		t.Errorf("version path kinds = %v, want exactly [ShowVersion]", rec.Kinds())
	}
}

// TestRecordingPresenterRecordsShowVersion proves the recorder captures the full
// Version payload so an engine-driven test can round-trip the
// version event independent of any rendering.
func TestRecordingPresenterRecordsShowVersion(t *testing.T) {
	rec := &presentertest.RecordingPresenter{}
	v := presenter.Version{Value: "1.4.0"}

	rec.ShowVersion(v)

	ev, ok := rec.At(0)
	if !ok {
		t.Fatal("expected one recorded event, got none")
	}
	if ev.Kind != presentertest.KindShowVersion {
		t.Fatalf("Kind = %v, want %v", ev.Kind, presentertest.KindShowVersion)
	}
	if ev.ShowVersion != v {
		t.Errorf("ShowVersion = %+v, want %+v", ev.ShowVersion, v)
	}
	if ev.ShowVersion.Value != "1.4.0" {
		t.Errorf("Value = %q, want %q", ev.ShowVersion.Value, "1.4.0")
	}
}
