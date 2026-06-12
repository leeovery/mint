package presenter_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
)

// TestVersionCarriesValueAndLeaf proves the Version payload carries the resolved
// version value plus the engine-supplied brand leaf as fields — the presenter
// renders what it is handed (plain ignores the leaf; pretty renders it).
func TestVersionCarriesValueAndLeaf(t *testing.T) {
	v := presenter.Version{Value: "1.4.0", Leaf: "🌱"}

	if v.Value != "1.4.0" {
		t.Errorf("Value = %q, want %q", v.Value, "1.4.0")
	}
	if v.Leaf != "🌱" {
		t.Errorf("Leaf = %q, want %q", v.Leaf, "🌱")
	}
}

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

// TestPrettyPresenterShowVersionRendersDressedLine is the core pretty acceptance:
// ShowVersion renders the dressed "🌿 mint v{value}" form with the brand leaf, the
// "mint" brand, and the "v"-prefixed value. Asserted under the no-colour profile so
// the layout/glyph is exact.
func TestPrettyPresenterShowVersionRendersDressedLine(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.ShowVersion(presenter.Version{Value: "1.4.0"})
	})

	want := "🌿 mint v1.4.0\n"
	if got := out.String(); got != want {
		t.Errorf("ShowVersion pretty = %q, want %q", got, want)
	}
}

// TestPrettyPresenterShowVersionColourOnEmitsANSI forces a colour-capable profile
// and asserts the dressed line carries ANSI SGR escapes (additive styling) while the
// load-bearing value and brand glyph survive.
func TestPrettyPresenterShowVersionColourOnEmitsANSI(t *testing.T) {
	out := drivePretty(termenv.TrueColor, func(p *presenter.PrettyPresenter) {
		p.ShowVersion(presenter.Version{Value: "1.4.0"})
	})

	got := out.String()
	if !bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("colour-on version line contains no ESC (0x1b) — expected SGR codes:\n%q", got)
	}
	if !strings.Contains(got, "🌿") {
		t.Errorf("🌿 brand glyph missing from version line:\n%q", got)
	}
	if !strings.Contains(got, "v1.4.0") {
		t.Errorf("v-prefixed value missing from version line:\n%q", got)
	}
}

// TestPrettyPresenterShowVersionLayoutSurvivesColourDowngrade forces the no-colour
// profile and asserts the dressed line survives as bare text — the value present, no
// SGR codes leaking, exact layout.
func TestPrettyPresenterShowVersionLayoutSurvivesColourDowngrade(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.ShowVersion(presenter.Version{Value: "1.4.0"})
	})

	if bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("downgraded version output leaked an SGR code:\n%q", out.String())
	}
	want := "🌿 mint v1.4.0\n"
	if got := out.String(); got != want {
		t.Errorf("downgraded version output = %q, want %q", got, want)
	}
}

// TestPrettyPresenterShowVersionUsesPayloadLeaf proves the pretty brand leaf comes
// from the payload (consistent with RunInfo/RunResult): a supplied leaf is rendered,
// and an empty leaf falls back to the 🌿 default via leafOrDefault.
func TestPrettyPresenterShowVersionUsesPayloadLeaf(t *testing.T) {
	supplied := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.ShowVersion(presenter.Version{Value: "1.4.0", Leaf: "🌱"})
	})
	if got, want := supplied.String(), "🌱 mint v1.4.0\n"; got != want {
		t.Errorf("supplied-leaf version = %q, want %q", got, want)
	}

	defaulted := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.ShowVersion(presenter.Version{Value: "1.4.0", Leaf: ""})
	})
	if got, want := defaulted.String(), "🌿 mint v1.4.0\n"; got != want {
		t.Errorf("empty-leaf version = %q, want %q (🌿 default)", got, want)
	}
}

// TestPrettyPresenterShowVersionWritesToStdoutOnly proves the dressed version line
// is narration → out ONLY and never duplicates to err, in both the colour-on and
// colour-off cases (the err writer is captured via the stderr-split test seam).
func TestPrettyPresenterShowVersionWritesToStdoutOnly(t *testing.T) {
	for _, profile := range []termenv.Profile{termenv.Ascii, termenv.TrueColor} {
		out := &bytes.Buffer{}
		errBuf := &bytes.Buffer{}
		p := presenter.NewPrettyPresenter(out, presenter.WithErr(errBuf), presenter.WithProfile(profile))

		p.ShowVersion(presenter.Version{Value: "1.4.0"})

		if errBuf.Len() != 0 {
			t.Errorf("profile %v: ShowVersion wrote to err: %q", profile, errBuf.String())
		}
		if out.Len() == 0 {
			t.Errorf("profile %v: ShowVersion wrote nothing to out", profile)
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

	prettyOut := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.ShowVersion(presenter.Version{Value: "1.4.0"})
	})
	prettyGot := prettyOut.String()
	if strings.Contains(prettyGot, "released") {
		t.Errorf("pretty version run emitted a release-style \"released\" footer:\n%q", prettyGot)
	}
	for _, marker := range []string{"done:", "Use these notes?", "› ", "y accept"} {
		if strings.Contains(plainGot, marker) || strings.Contains(prettyGot, marker) {
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
// Version payload — value and leaf — so an engine-driven test can round-trip the
// version event independent of any rendering.
func TestRecordingPresenterRecordsShowVersion(t *testing.T) {
	rec := &presentertest.RecordingPresenter{}
	v := presenter.Version{Value: "1.4.0", Leaf: "🌱"}

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
	if ev.ShowVersion.Leaf != "🌱" {
		t.Errorf("Leaf = %q, want %q", ev.ShowVersion.Leaf, "🌱")
	}
}
