package presenter_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// drivePretty runs the supplied callback against a PrettyPresenter whose renderer
// is forced to the given colour profile, capturing narration into the returned
// buffer. Forcing the profile makes the colour-on/colour-off assertions
// deterministic regardless of the test runner's own TTY.
func drivePretty(profile termenv.Profile, fn func(p *presenter.PrettyPresenter)) *bytes.Buffer {
	out := &bytes.Buffer{}
	p := presenter.NewPrettyPresenter(out, presenter.WithProfile(profile))
	fn(p)
	return out
}

// TestPrettyPresenterSatisfiesInterface is the compile-time and runtime proof that
// PrettyPresenter is usable wherever a Presenter is required.
func TestPrettyPresenterSatisfiesInterface(t *testing.T) {
	var _ presenter.Presenter = (*presenter.PrettyPresenter)(nil)

	var p presenter.Presenter = presenter.NewPrettyPresenter(&bytes.Buffer{}, presenter.WithErr(&bytes.Buffer{}))
	p.RunStarted(presenter.RunInfo{})
	p.StageStarted(presenter.StageStart{})
	p.StageSucceeded(presenter.StageSuccess{})
	p.StageFailed(presenter.StageFailure{})
	p.RunFinished(presenter.RunResult{})
}

// TestPrettyPresenterAllOptionsCombineInOneCall proves the previously-awkward
// combination — force colour AND capture err AND script input — now constructs in a
// SINGLE NewPrettyPresenter call via three composable options, with no setter
// chaining to backfill a constructor gap. It drives a gate prompt: WithInput feeds
// the scripted choice, WithProfile forces colour on, and WithErr captures the err
// stream. The behaviour must match the old NewPrettyPresenterWithErr(out, err,
// profile).WithInput(reader) form exactly: the styled menu reaches out (ANSI
// present), the scripted choice is read and returned, and a clean accept leaves err
// untouched.
func TestPrettyPresenterAllOptionsCombineInOneCall(t *testing.T) {
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	p := presenter.NewPrettyPresenter(out,
		presenter.WithProfile(termenv.TrueColor),
		presenter.WithErr(errBuf),
		presenter.WithInput(strings.NewReader("y\n")),
	)

	choice, err := p.Prompt(presenter.NotesReviewGate())
	if err != nil {
		t.Fatalf("Prompt returned error = %v, want nil", err)
	}
	if choice != presenter.NotesReviewGate().Default {
		t.Errorf("Prompt choice = %q, want the gate default %q", choice, presenter.NotesReviewGate().Default)
	}
	// WithProfile forced colour on, so the styled menu carries ANSI on out.
	if !bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("colour-forced menu carries no ESC (0x1b) on out:\n%q", out.String())
	}
	// The scripted choice was read from WithInput, and a clean accept writes nothing to err.
	if errBuf.Len() != 0 {
		t.Errorf("clean accept wrote to err = %q, want empty", errBuf.String())
	}
}

// TestPrettyPresenterRendersMinimalSequence drives the walking-skeleton sequence
// (start-of-run brand line -> a check stage success -> end-of-run brand line) and
// asserts the three lines appear, in order, with the brand leaf and ✓ glyph. The
// no-colour profile keeps the assertion on layout/glyphs rather than ANSI bytes.
func TestPrettyPresenterRendersMinimalSequence(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.4.0", Action: "releasing"})
		p.StageSucceeded(presenter.StageSuccess{Name: "version", Detail: "v1.3.2 → v1.4.0 (minor)"})
		p.RunFinished(presenter.RunResult{Project: "acme", Version: "1.4.0", URL: "https://github.com/acme/acme/releases/tag/v1.4.0"})
	})

	got := out.String()
	topBrand := "🌿 mint · acme  ›  releasing v1.4.0"
	stageLine := "  ✓ version"
	bottomBrand := "🌿 released acme v1.4.0 · https://github.com/acme/acme/releases/tag/v1.4.0"

	topIdx := strings.Index(got, topBrand)
	stageIdx := strings.Index(got, stageLine)
	bottomIdx := strings.Index(got, bottomBrand)

	if topIdx < 0 {
		t.Errorf("top brand line not found in output:\n%s", got)
	}
	if stageIdx < 0 {
		t.Errorf("stage success line %q not found in output:\n%s", stageLine, got)
	}
	if bottomIdx < 0 {
		t.Errorf("bottom brand line not found in output:\n%s", got)
	}
	if topIdx >= stageIdx || stageIdx >= bottomIdx {
		t.Errorf("lines out of order: top=%d stage=%d bottom=%d\n%s", topIdx, stageIdx, bottomIdx, got)
	}
	// The stage detail must travel through verbatim, including any engine-supplied →.
	if !strings.Contains(got, "v1.3.2 → v1.4.0 (minor)") {
		t.Errorf("stage detail not rendered verbatim:\n%s", got)
	}
}

// TestPrettyPresenterColourOnEmitsANSI forces a colour-capable profile and asserts
// the narration carries ANSI SGR escapes (ESC 0x1b) around the styled glyph/text,
// while the layout (indent, glyph, padded name) survives.
func TestPrettyPresenterColourOnEmitsANSI(t *testing.T) {
	out := drivePretty(termenv.TrueColor, func(p *presenter.PrettyPresenter) {
		p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.4.0", Action: "releasing"})
		p.StageSucceeded(presenter.StageSuccess{Name: "version", Detail: "v1.3.2 → v1.4.0 (minor)"})
		p.RunFinished(presenter.RunResult{Project: "acme", Version: "1.4.0"})
	})

	if !bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("colour-on output contains no ESC (0x1b) — expected ANSI SGR codes:\n%q", out.String())
	}
	if !strings.Contains(out.String(), "✓") {
		t.Errorf("✓ glyph missing from colour-on output:\n%q", out.String())
	}
	// Under colour the glyph is ANSI-wrapped, so the indent and the padded name
	// are asserted around it rather than as one bare literal: the two-space indent
	// precedes the styled ✓, and the name padded to its column ("version  ")
	// follows. The detail's engine-supplied → must survive the styling too.
	if !strings.Contains(out.String(), "  \x1b[") {
		t.Errorf("two-space indent before the styled glyph missing:\n%q", out.String())
	}
	if !strings.Contains(out.String(), "version    v1.3.2 → v1.4.0 (minor)") {
		t.Errorf("padded stage name + verbatim detail missing:\n%q", out.String())
	}
}

// TestPrettyPresenterColourDowngradeEmitsNoANSI forces the no-colour (Ascii)
// profile and asserts the narration carries no ESC byte at all, while the layout
// and glyphs (✓ and the brand leaf 🌿) are preserved. This is lipgloss's colour
// auto-downgrade behaving correctly — not a third mint mode.
func TestPrettyPresenterColourDowngradeEmitsNoANSI(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.4.0", Action: "releasing"})
		p.StageSucceeded(presenter.StageSuccess{Name: "version", Detail: "v1.3.2 → v1.4.0 (minor)"})
		p.RunFinished(presenter.RunResult{Project: "acme", Version: "1.4.0"})
	})

	if bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("downgraded output contains an ESC (0x1b) — colour codes leaked:\n%q", out.String())
	}
	if !strings.Contains(out.String(), "✓") {
		t.Errorf("✓ glyph not preserved under colour downgrade:\n%q", out.String())
	}
	if !strings.Contains(out.String(), "🌿") {
		t.Errorf("brand leaf 🌿 not preserved under colour downgrade:\n%q", out.String())
	}
	if !strings.Contains(out.String(), "  ✓ version") {
		t.Errorf("stage layout not preserved under colour downgrade:\n%q", out.String())
	}
}

// TestPrettyPresenterElapsedRendersOnlyOnBlockingStages asserts the elapsed
// suffix "({elapsed})" appears for a blocking stage success and is absent for a
// short (non-blocking) one — the engine times the stage; pretty shows elapsed on
// long/blocking stages only.
func TestPrettyPresenterElapsedRendersOnlyOnBlockingStages(t *testing.T) {
	tests := []struct {
		name        string
		blocking    bool
		wantElapsed bool
	}{
		{name: "blocking stage shows elapsed", blocking: true, wantElapsed: true},
		{name: "short stage omits elapsed", blocking: false, wantElapsed: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
				p.StageSucceeded(presenter.StageSuccess{
					Name:     "prep",
					Detail:   "pre_tag: npm ci && npm run build",
					Elapsed:  2300 * time.Millisecond,
					Blocking: tt.blocking,
				})
			})

			gotElapsed := strings.Contains(out.String(), "(2.3s)")
			if gotElapsed != tt.wantElapsed {
				t.Errorf("blocking=%v elapsed-present=%v, want %v\noutput: %q", tt.blocking, gotElapsed, tt.wantElapsed, out.String())
			}
		})
	}
}

// TestPrettyPresenterStageSucceededDetailOnlyHasNoArtefact locks the empty-detail
// edge cases: a stage success carrying no Detail renders the glyph and stage name
// with NO trailing-whitespace / empty-slot artefact, and (when blocking) appends
// the elapsed with no stray leading space before it.
func TestPrettyPresenterStageSucceededDetailOnlyHasNoArtefact(t *testing.T) {
	tests := []struct {
		name     string
		stage    string
		blocking bool
		elapsed  time.Duration
		want     string
	}{
		{
			name:  "short detail-less stage has no trailing whitespace",
			stage: "x",
			want:  "  ✓ x\n",
		},
		{
			name:     "blocking detail-less stage shows elapsed with no leading space",
			stage:    "x",
			blocking: true,
			elapsed:  1100 * time.Millisecond,
			// Two-space indent, ✓, then "x" padded into the 11-wide stage column
			// (one char + ten spaces) and the elapsed placed flush at the column —
			// no stray leading space before "(".
			want: "  ✓ x          (1.1s)\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
				p.StageSucceeded(presenter.StageSuccess{
					Name:     tt.stage,
					Detail:   "",
					Elapsed:  tt.elapsed,
					Blocking: tt.blocking,
				})
			})

			got := out.String()
			if got != tt.want {
				t.Errorf("detail-less stage line = %q, want %q", got, tt.want)
			}
			// Belt-and-braces: no line carries trailing whitespace before its newline.
			for _, line := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
				if strings.TrimRight(line, " ") != line {
					t.Errorf("line carries a trailing-whitespace artefact: %q", line)
				}
			}
		})
	}
}

// TestPrettyPresenterStageNamesPadToCommonColumn renders two stage successes with
// different-length names and asserts their detail text starts at the same column —
// the "padded to a column so successive lines align" rule.
func TestPrettyPresenterStageNamesPadToCommonColumn(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.StageSucceeded(presenter.StageSuccess{Name: "version", Detail: "v1.3.2 → v1.4.0 (minor)"})
		p.StageSucceeded(presenter.StageSuccess{Name: "preflight", Detail: "clean · on main"})
	})

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected two stage lines, got %d:\n%q", len(lines), out.String())
	}

	col0 := strings.Index(lines[0], "v1.3.2")
	col1 := strings.Index(lines[1], "clean")
	if col0 < 0 || col1 < 0 {
		t.Fatalf("detail text not found in stage lines:\n%q", out.String())
	}
	if col0 != col1 {
		t.Errorf("detail columns misaligned: version detail at %d, preflight detail at %d\n%q", col0, col1, out.String())
	}
}

// TestPrettyPresenterStageColumnSurvivesColourDowngrade forces the no-colour
// profile and asserts the ✓ glyph, two-space indent, and the column padding (so
// the detail starts at the aligned column) all survive while no SGR codes leak.
func TestPrettyPresenterStageColumnSurvivesColourDowngrade(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.StageSucceeded(presenter.StageSuccess{Name: "prep", Detail: "pre_tag: npm ci && npm run build", Elapsed: 2300 * time.Millisecond, Blocking: true})
	})

	got := out.String()
	if bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("downgraded stage line leaked an SGR code:\n%q", got)
	}
	// Two-space indent, glyph, name padded into the column, then the detail.
	want := "  ✓ prep       pre_tag: npm ci && npm run build (2.3s)\n"
	if got != want {
		t.Errorf("downgraded stage line = %q, want %q", got, want)
	}
}

// TestPrettyPresenterStartLineUsesEngineAction proves the brand top line renders
// the engine-supplied verb word from RunInfo — never a hardcoded "releasing" — so
// regenerate narrates "… › regenerating v{X}".
func TestPrettyPresenterStartLineUsesEngineAction(t *testing.T) {
	tests := []struct {
		name   string
		action string
		want   string
	}{
		{name: "release verb", action: "releasing", want: "🌿 mint · acme  ›  releasing v1.4.0"},
		{name: "regenerate verb", action: "regenerating", want: "🌿 mint · acme  ›  regenerating v1.4.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
				p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.4.0", Action: tt.action})
			})

			if !strings.Contains(out.String(), tt.want) {
				t.Errorf("brand line for action %q = %q, want it to contain %q", tt.action, out.String(), tt.want)
			}
		})
	}
}

// TestPrettyPresenterStartLineOmitsEmptyVersion proves a version-LESS run (commit
// announces no version) renders the brand line WITHOUT the " v{X}" segment — never
// a dangling bare "v" — per the presenter's no-dangling-segment rule.
func TestPrettyPresenterStartLineOmitsEmptyVersion(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.RunStarted(presenter.RunInfo{Project: "acme", Action: "committing"})
	})

	want := "🌿 mint · acme  ›  committing\n"
	if got := out.String(); got != want {
		t.Errorf("version-less brand line = %q, want exactly %q (no dangling \" v\")", got, want)
	}
}

// TestPrettyPresenterBrandLeafComesFromPayload proves the brand leaf is rendered
// from the engine-supplied payload datum: a supplied leaf is used verbatim, and an
// empty Leaf defaults to 🌿 rather than being re-derived or hardcoded.
func TestPrettyPresenterBrandLeafComesFromPayload(t *testing.T) {
	tests := []struct {
		name string
		leaf string
		want string
	}{
		{name: "supplied leaf used verbatim", leaf: "🌱", want: "🌱 mint · acme"},
		{name: "empty leaf defaults to mint leaf", leaf: "", want: "🌿 mint · acme"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
				p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.4.0", Action: "releasing", Leaf: tt.leaf})
			})

			if !strings.Contains(out.String(), tt.want) {
				t.Errorf("brand top line for leaf %q = %q, want it to contain %q", tt.leaf, out.String(), tt.want)
			}
		})
	}
}

// TestPrettyPresenterBottomBrandLeafComesFromPayload mirrors the top-line leaf
// rule for the closing brand line: supplied leaf used verbatim; empty defaults.
func TestPrettyPresenterBottomBrandLeafComesFromPayload(t *testing.T) {
	tests := []struct {
		name string
		leaf string
		want string
	}{
		{name: "supplied leaf used verbatim", leaf: "🌱", want: "🌱 released acme v1.4.0"},
		{name: "empty leaf defaults to mint leaf", leaf: "", want: "🌿 released acme v1.4.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
				p.RunFinished(presenter.RunResult{Project: "acme", Version: "1.4.0", Leaf: tt.leaf})
			})

			if !strings.Contains(out.String(), tt.want) {
				t.Errorf("brand bottom line for leaf %q = %q, want it to contain %q", tt.leaf, out.String(), tt.want)
			}
		})
	}
}

// TestPrettyPresenterBottomBrandOmitsEmptyURLSeparator covers the empty-URL edge
// case: the closing brand line must not dangle a " · " separator when there is no
// release URL (e.g. regenerate, which publishes nothing).
func TestPrettyPresenterBottomBrandOmitsEmptyURLSeparator(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		notWant string
	}{
		{
			name: "with url",
			url:  "https://github.com/acme/acme/releases/tag/v1.4.0",
			want: "🌿 released acme v1.4.0 · https://github.com/acme/acme/releases/tag/v1.4.0",
		},
		{
			name:    "empty url omits separator",
			url:     "",
			want:    "🌿 released acme v1.4.0",
			notWant: "v1.4.0 · ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
				p.RunFinished(presenter.RunResult{Project: "acme", Version: "1.4.0", URL: tt.url})
			})

			if !strings.Contains(out.String(), tt.want) {
				t.Errorf("bottom brand for url %q = %q, want it to contain %q", tt.url, out.String(), tt.want)
			}
			if tt.notWant != "" && strings.Contains(out.String(), tt.notWant) {
				t.Errorf("bottom brand for empty url = %q, must not contain dangling separator %q", out.String(), tt.notWant)
			}
		})
	}
}

// TestPrettyPresenterRegenerateSingleVersionRendersBlockThenURLlessClose drives a
// single-version regenerate: the per-version block reuses the EXISTING events
// exactly as release does (RunStarted("regenerating") → a stage → ShowNotes →
// Prompt), then RunFinished{Verb:VerbRegenerate, Summary:"v1.4.0"} renders the
// URL-less, verb-shaped closing summary "{leaf} regenerated {project} {Summary}".
// The close must carry NO URL and NO dangling " · " separator.
func TestPrettyPresenterRegenerateSingleVersionRendersBlockThenURLlessClose(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.4.0", Action: "regenerating"})
		p.StageSucceeded(presenter.StageSuccess{Name: "notes", Detail: "generated", Blocking: true})
		p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: "Faster cold starts."})
		_, _ = p.Prompt(presenter.NotesReviewGate())
		p.RunFinished(presenter.RunResult{Project: "acme", Verb: presenter.VerbRegenerate, Summary: "v1.4.0"})
	})

	got := out.String()
	if !strings.Contains(got, "🌿 mint · acme  ›  regenerating v1.4.0") {
		t.Errorf("regenerate block start-of-run brand line missing:\n%q", got)
	}
	if !strings.Contains(got, "🌿 regenerated acme v1.4.0\n") {
		t.Errorf("regenerate close = want it to contain %q:\n%q", "🌿 regenerated acme v1.4.0", got)
	}
	// The close itself must omit the URL field — no " · {url}" tail after the summary.
	// (The brand top line and notes rule legitimately use a middot, so the check is
	// scoped to the summary tail, not the whole output.)
	if strings.Contains(got, "v1.4.0 · ") {
		t.Errorf("regenerate close must carry NO dangling separator after the summary:\n%q", got)
	}
	if strings.Contains(got, "http") {
		t.Errorf("regenerate close must carry NO URL:\n%q", got)
	}
}

// TestPrettyPresenterRegenerateAllRendersBlocksInEmitOrderNoReorder proves --all
// renders ONE block per version in ENGINE EMIT ORDER (oldest→newest) and the
// presenter does NOT reorder: three engine-emitted RunStarted blocks (v1.2.0,
// v1.3.0, v1.4.0) appear in exactly that order. Block ordering is engine-owned —
// the presenter renders linearly in emit order.
func TestPrettyPresenterRegenerateAllRendersBlocksInEmitOrderNoReorder(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.2.0", Action: "regenerating"})
		p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.3.0", Action: "regenerating"})
		p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.4.0", Action: "regenerating"})
		p.RunFinished(presenter.RunResult{Project: "acme", Verb: presenter.VerbRegenerate, Summary: "v1.2.0–v1.4.0 (3 versions)"})
	})

	got := out.String()
	idx120 := strings.Index(got, "regenerating v1.2.0")
	idx130 := strings.Index(got, "regenerating v1.3.0")
	idx140 := strings.Index(got, "regenerating v1.4.0")
	if idx120 < 0 || idx130 < 0 || idx140 < 0 {
		t.Fatalf("one or more regenerate block start lines missing:\n%q", got)
	}
	if idx120 >= idx130 || idx130 >= idx140 {
		t.Errorf("blocks not in engine emit order (oldest→newest): 1.2.0=%d 1.3.0=%d 1.4.0=%d\n%q", idx120, idx130, idx140, got)
	}
}

// TestPrettyPresenterRegenerateBlockStartUsesRegeneratingNotReleasing asserts the
// per-block brand line renders the engine-supplied Action word "regenerating", not
// "releasing" — the presenter renders the supplied action verbatim.
func TestPrettyPresenterRegenerateBlockStartUsesRegeneratingNotReleasing(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.4.0", Action: "regenerating"})
	})

	got := out.String()
	if !strings.Contains(got, "🌿 mint · acme  ›  regenerating v1.4.0") {
		t.Errorf("brand line did not use the engine action 'regenerating':\n%q", got)
	}
	if strings.Contains(got, "releasing") {
		t.Errorf("brand line must NOT say 'releasing' on a regenerate block:\n%q", got)
	}
}

// TestPrettyPresenterRegenerateAllSingleVersionRendersSetSummaryNotReleaseFooter
// proves the --all SINGLE-version case still renders the regenerate arm: a
// set-summary close, NOT a release-style v{X}+url footer. The presenter renders the
// regenerate arm whenever Verb=VerbRegenerate, regardless of block count.
func TestPrettyPresenterRegenerateAllSingleVersionRendersSetSummaryNotReleaseFooter(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.4.0", Action: "regenerating"})
		p.RunFinished(presenter.RunResult{Project: "acme", Verb: presenter.VerbRegenerate, Summary: "1 version (v1.4.0)"})
	})

	got := out.String()
	if !strings.Contains(got, "🌿 regenerated acme 1 version (v1.4.0)\n") {
		t.Errorf("--all single-version close = want it to contain %q:\n%q", "🌿 regenerated acme 1 version (v1.4.0)", got)
	}
	if strings.Contains(got, "released") {
		t.Errorf("--all single-version regenerate must NOT render a release footer:\n%q", got)
	}
	// No " · {url}" tail after the engine-supplied summary.
	if strings.Contains(got, "(v1.4.0) · ") {
		t.Errorf("--all single-version regenerate close must carry NO dangling separator after the summary:\n%q", got)
	}
}

// TestPrettyPresenterRegenerateCloseOmitsURLWithNoDanglingSeparator asserts the
// regenerate close omits the {url} field entirely — no URL, no dangling " · "
// separator — and locks the exact closing line.
func TestPrettyPresenterRegenerateCloseOmitsURLWithNoDanglingSeparator(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.RunFinished(presenter.RunResult{Project: "acme", Verb: presenter.VerbRegenerate, Summary: "v1.4.0"})
	})

	got := out.String()
	if got != "🌿 regenerated acme v1.4.0\n" {
		t.Errorf("regenerate close = %q, want exactly %q (no url, no dangling separator)", got, "🌿 regenerated acme v1.4.0\n")
	}
}

// TestPrettyPresenterFailedRegenerateSuppressesClose proves the Phase-2
// terminalFailure flag suppresses the regenerate closing summary too: a StageFailed
// then RunFinished{Verb:VerbRegenerate} renders the ✗ line but NO closing brand
// summary — the suppression check runs BEFORE the verb dispatch.
func TestPrettyPresenterFailedRegenerateSuppressesClose(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.StageFailed(presenter.StageFailure{Name: "notes", Message: "claude failed"})
		p.RunFinished(presenter.RunResult{Project: "acme", Verb: presenter.VerbRegenerate, Summary: "v1.4.0"})
	})

	got := out.String()
	if !strings.Contains(got, "  ✗ notes") {
		t.Errorf("✗ line missing:\n%q", got)
	}
	if strings.Contains(got, "regenerated") {
		t.Errorf("regenerate success close must be suppressed after a StageFailed, got:\n%q", got)
	}
}

// TestPrettyPresenterReleaseRunFinishedUnchangedByDefaultVerb is the regression
// guard for the additive discriminator: a RunResult with NO Verb set (default
// VerbRelease) still renders the release form WITH the URL.
func TestPrettyPresenterReleaseRunFinishedUnchangedByDefaultVerb(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.RunFinished(presenter.RunResult{Project: "acme", Version: "1.4.0", URL: "https://github.com/acme/acme/releases/tag/v1.4.0"})
	})

	if !strings.Contains(out.String(), "🌿 released acme v1.4.0 · https://github.com/acme/acme/releases/tag/v1.4.0\n") {
		t.Errorf("default-Verb release close changed:\n%q", out.String())
	}
}

// TestPrettyPresenterRegenerateFreshNotesBlockRendersFourChoiceGate drives a
// fresh-notes block: Prompt(NotesReviewGate()) renders the four-choice y/n/e/r
// vertical menu. The presenter renders whichever gate the engine passes — it does
// NOT decide reuse-vs-fresh.
func TestPrettyPresenterRegenerateFreshNotesBlockRendersFourChoiceGate(t *testing.T) {
	out := &bytes.Buffer{}
	p := presenter.NewPrettyPresenter(out, presenter.WithProfile(termenv.Ascii), presenter.WithInput(strings.NewReader("y\n")))
	_, _ = p.Prompt(presenter.NotesReviewGate())

	got := out.String()
	for _, want := range []string{
		"    y  accept & proceed [default]",
		"    n  abort",
		"    e  edit in $EDITOR",
		"    r  regenerate",
		"  Continue? › ",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("fresh-notes block four-choice menu missing %q:\n%q", want, got)
		}
	}
}

// TestPrettyPresenterRegenerateReuseNotesBlockRendersTwoChoiceGate drives a
// reused-notes block: Prompt(ReuseConfirmGate()) renders the two-choice y/n confirm
// — NO e/r lines, since there are no freshly-generated notes to edit or regenerate.
func TestPrettyPresenterRegenerateReuseNotesBlockRendersTwoChoiceGate(t *testing.T) {
	out := &bytes.Buffer{}
	p := presenter.NewPrettyPresenter(out, presenter.WithProfile(termenv.Ascii), presenter.WithInput(strings.NewReader("y\n")))
	_, _ = p.Prompt(presenter.ReuseConfirmGate())

	got := out.String()
	if !strings.Contains(got, "    y  accept & proceed [default]") {
		t.Errorf("reuse confirm y line missing:\n%q", got)
	}
	if !strings.Contains(got, "    n  abort") {
		t.Errorf("reuse confirm n line missing:\n%q", got)
	}
	if strings.Contains(got, "    e  ") {
		t.Errorf("reuse confirm must NOT render an e line:\n%q", got)
	}
	if strings.Contains(got, "    r  ") {
		t.Errorf("reuse confirm must NOT render an r line:\n%q", got)
	}
}

// TestPrettyPresenterShowPlanRendersBulletedBlock is the core pretty acceptance:
// ShowPlan renders a two-space-indented "Plan" header followed by one
// "    • {verb}<pad>{target}" line per step, verbs padded so targets align at the
// (longest verb + 2) column. The no-colour profile keeps the assertion on the
// exact layout/glyphs rather than ANSI bytes.
func TestPrettyPresenterShowPlanRendersBulletedBlock(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.ShowPlan(presenter.Plan{Steps: []presenter.PlanStep{
			{Verb: "commit", Target: "CHANGELOG.md + bin/acme"},
			{Verb: "tag", Target: "v1.4.0 (annotated)"},
			{Verb: "push", Target: "--atomic → origin"},
			{Verb: "publish", Target: "GitHub release"},
		}})
	})

	// Column = longest verb ("publish", 7) + 2 = 9, so every verb pads to width 9
	// and the targets all start at the same column — matching the worked example.
	want := "  Plan\n" +
		"    • commit   CHANGELOG.md + bin/acme\n" +
		"    • tag      v1.4.0 (annotated)\n" +
		"    • push     --atomic → origin\n" +
		"    • publish  GitHub release\n"
	if got := out.String(); got != want {
		t.Errorf("ShowPlan block mismatch\n got: %q\nwant: %q", got, want)
	}
}

// TestPrettyPresenterShowPlanTargetsAlign independently asserts the alignment
// property: regardless of verb length, every target starts at the same column.
func TestPrettyPresenterShowPlanTargetsAlign(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.ShowPlan(presenter.Plan{Steps: []presenter.PlanStep{
			{Verb: "commit", Target: "CHANGELOG.md + bin/acme"},
			{Verb: "tag", Target: "v1.4.0 (annotated)"},
			{Verb: "push", Target: "--atomic → origin"},
			{Verb: "publish", Target: "GitHub release"},
		}})
	})

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	// lines[0] is the "  Plan" header; the bullets follow.
	bullets := lines[1:]
	if len(bullets) != 4 {
		t.Fatalf("expected 4 bullet lines, got %d:\n%q", len(bullets), out.String())
	}
	targets := []string{"CHANGELOG.md", "v1.4.0", "--atomic", "GitHub"}
	col := strings.Index(bullets[0], targets[0])
	if col < 0 {
		t.Fatalf("target %q not found in %q", targets[0], bullets[0])
	}
	for i, b := range bullets {
		got := strings.Index(b, targets[i])
		if got != col {
			t.Errorf("target column misaligned: line %d %q has target at %d, want %d", i, b, got, col)
		}
	}
}

// TestPrettyPresenterShowPlanSingleStep covers the single-step edge: the header
// plus exactly one bullet, with the verb padded to its own (only) verb length + 2.
func TestPrettyPresenterShowPlanSingleStep(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.ShowPlan(presenter.Plan{Steps: []presenter.PlanStep{
			{Verb: "tag", Target: "v1.4.0"},
		}})
	})

	want := "  Plan\n" +
		"    • tag  v1.4.0\n"
	got := out.String()
	if got != want {
		t.Errorf("single-step plan = %q, want %q", got, want)
	}
	if strings.Count(got, "•") != 1 {
		t.Errorf("single-step plan must render exactly one bullet, got %q", got)
	}
}

// TestPrettyPresenterShowPlanEmptyOmitsBlock covers the empty-plan edge: no steps
// renders nothing at all — no "Plan" header, no bullets, no orphan output.
func TestPrettyPresenterShowPlanEmptyOmitsBlock(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.ShowPlan(presenter.Plan{})
	})

	if got := out.String(); got != "" {
		t.Errorf("empty plan must render no block, got %q", got)
	}
}

// TestPrettyPresenterShowPlanEmptyTargetRendersVerbOnly covers the empty-target
// edge: a step whose target is empty renders "    • {verb}" with NO trailing pad
// or space.
func TestPrettyPresenterShowPlanEmptyTargetRendersVerbOnly(t *testing.T) {
	tests := []struct {
		name  string
		steps []presenter.PlanStep
		want  string
	}{
		{
			name:  "lone empty-target step renders just the verb",
			steps: []presenter.PlanStep{{Verb: "publish", Target: ""}},
			want:  "  Plan\n    • publish\n",
		},
		{
			name: "empty-target step among others has no trailing pad",
			steps: []presenter.PlanStep{
				{Verb: "tag", Target: "v1.4.0"},
				{Verb: "publish", Target: ""},
			},
			// Column = longest verb ("publish", 7) + 2 = 9, so "tag" pads to the
			// column before its target; the empty-target "publish" line drops the
			// pad entirely (nothing follows the verb).
			want: "  Plan\n    • tag      v1.4.0\n    • publish\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
				p.ShowPlan(presenter.Plan{Steps: tt.steps})
			})

			got := out.String()
			if got != tt.want {
				t.Errorf("ShowPlan = %q, want %q", got, tt.want)
			}
			// No bullet line may carry a trailing-whitespace artefact.
			for _, line := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
				if strings.TrimRight(line, " ") != line {
					t.Errorf("line carries a trailing-whitespace artefact: %q", line)
				}
			}
		})
	}
}

// TestPrettyPresenterShowPlanLayoutSurvivesColourDowngrade asserts the block's
// layout (header indent, bullet, padding) survives the no-colour profile with no
// SGR codes leaking — the bullet/colour may be styled, but the layout must remain.
func TestPrettyPresenterShowPlanLayoutSurvivesColourDowngrade(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.ShowPlan(presenter.Plan{Steps: []presenter.PlanStep{
			{Verb: "commit", Target: "CHANGELOG.md + bin/acme"},
			{Verb: "publish", Target: "GitHub release"},
		}})
	})

	got := out.String()
	if bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("downgraded plan block leaked an SGR code:\n%q", got)
	}
	want := "  Plan\n" +
		"    • commit   CHANGELOG.md + bin/acme\n" +
		"    • publish  GitHub release\n"
	if got != want {
		t.Errorf("downgraded plan block = %q, want %q", got, want)
	}
}

// TestPrettyPresenterShowPlanColourOnEmitsANSIButKeepsLayout forces a
// colour-capable profile and asserts the block carries ANSI SGR escapes (from the
// styled header/bullet) while the layout text (the "Plan" header, the verbs and
// targets) survives.
func TestPrettyPresenterShowPlanColourOnEmitsANSIButKeepsLayout(t *testing.T) {
	out := drivePretty(termenv.TrueColor, func(p *presenter.PrettyPresenter) {
		p.ShowPlan(presenter.Plan{Steps: []presenter.PlanStep{
			{Verb: "commit", Target: "CHANGELOG.md + bin/acme"},
			{Verb: "publish", Target: "GitHub release"},
		}})
	})

	got := out.String()
	if !bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("colour-on plan block contains no ESC (0x1b) — expected ANSI SGR codes:\n%q", got)
	}
	for _, frag := range []string{"Plan", "•", "commit", "CHANGELOG.md + bin/acme", "publish", "GitHub release"} {
		if !strings.Contains(got, frag) {
			t.Errorf("colour-on plan block missing %q:\n%q", frag, got)
		}
	}
}

// TestPrettyPresenterWarnRendersAmberLineToStdout is the core pretty Warn
// acceptance under colour: the "  ⚠ {label}  {message}" line (two-space indent,
// amber ⚠ glyph, label, two spaces, message) is written to stdout and carries
// ANSI SGR codes (the amber styling) while the layout text survives. Label and
// message arrive as separate fields — never parsed from a single combined string.
func TestPrettyPresenterWarnRendersAmberLineToStdout(t *testing.T) {
	out := drivePretty(termenv.TrueColor, func(p *presenter.PrettyPresenter) {
		p.Warn(presenter.Warning{Label: "post_release", Message: "hook failed (tag is already published): scripts/notify.sh exited 1"})
	})

	got := out.String()
	if !bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("colour-on warn line contains no ESC (0x1b) — expected amber SGR codes:\n%q", got)
	}
	if !strings.Contains(got, "⚠") {
		t.Errorf("⚠ glyph missing from warn line:\n%q", got)
	}
	for _, frag := range []string{"post_release", "hook failed (tag is already published): scripts/notify.sh exited 1"} {
		if !strings.Contains(got, frag) {
			t.Errorf("warn line missing %q:\n%q", frag, got)
		}
	}
}

// TestPrettyPresenterWarnLayoutSurvivesColourDowngrade forces the no-colour profile
// and asserts the exact warn layout — two-space indent, ⚠ glyph, label, two-space
// gap, message — with no SGR codes leaking. The amber styling may colour the line,
// but the layout and glyph survive a downgrade intact.
func TestPrettyPresenterWarnLayoutSurvivesColourDowngrade(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.Warn(presenter.Warning{Label: "post_release", Message: "hook failed (tag is already published): scripts/notify.sh exited 1"})
	})

	got := out.String()
	if bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("downgraded warn line leaked an SGR code:\n%q", got)
	}
	want := "  ⚠ post_release  hook failed (tag is already published): scripts/notify.sh exited 1\n"
	if got != want {
		t.Errorf("downgraded warn line = %q, want %q", got, want)
	}
}

// TestPrettyPresenterWarnEmptyMessageHasNoTrailingWhitespace covers the empty-message
// edge: the line renders "  ⚠ {label}" with NO trailing-whitespace artefact (the
// two-space gap and the message are dropped when there is no message), and no
// invented content.
func TestPrettyPresenterWarnEmptyMessageHasNoTrailingWhitespace(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.Warn(presenter.Warning{Label: "x", Message: ""})
	})

	got := out.String()
	want := "  ⚠ x\n"
	if got != want {
		t.Errorf("empty-message warn line = %q, want %q", got, want)
	}
	for _, line := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
		if strings.TrimRight(line, " ") != line {
			t.Errorf("warn line carries a trailing-whitespace artefact: %q", line)
		}
	}
}

// TestPrettyPresenterWarnWritesToBothStreams proves the stream contract for Warn:
// with colour forced on, the styled amber line reaches stdout (ANSI present) while
// an UNSTYLED copy reaches err (no ANSI) — stderr is a redirect-visibility channel,
// not a styled surface, mirroring StageFailed's err summary.
func TestPrettyPresenterWarnWritesToBothStreams(t *testing.T) {
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	p := presenter.NewPrettyPresenter(out, presenter.WithErr(errBuf), presenter.WithProfile(termenv.TrueColor))
	p.Warn(presenter.Warning{Label: "post_release", Message: "hook failed: scripts/notify.sh exited 1"})

	// out carries the styled amber line (colour forced on), so it has ANSI.
	if !bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("pretty: stdout warn line should be styled (ANSI) under a colour profile:\n%q", out.String())
	}
	// err carries the unstyled warn copy, so it must have no ANSI.
	if bytes.ContainsRune(errBuf.Bytes(), 0x1b) {
		t.Errorf("pretty: stderr warn copy must be unstyled but contains an ESC (0x1b):\n%q", errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "post_release") || !strings.Contains(errBuf.String(), "hook failed: scripts/notify.sh exited 1") {
		t.Errorf("pretty: stderr warn copy missing label/message:\n%q", errBuf.String())
	}
	// The err copy carries the ⚠ glyph too — it is the same text form, just unstyled.
	if !strings.Contains(errBuf.String(), "⚠") {
		t.Errorf("pretty: stderr warn copy missing ⚠ glyph:\n%q", errBuf.String())
	}
}

// TestPrettyPresenterWarnMultipleRenderInSequence covers the multiple-warnings edge:
// two Warn calls produce two independent ⚠ lines, in order, in BOTH streams — no
// collapsing or de-duplication.
func TestPrettyPresenterWarnMultipleRenderInSequence(t *testing.T) {
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	p := presenter.NewPrettyPresenter(out, presenter.WithErr(errBuf), presenter.WithProfile(termenv.Ascii))
	p.Warn(presenter.Warning{Label: "post_release", Message: "hook failed"})
	p.Warn(presenter.Warning{Label: "cleanup", Message: "temp dir left behind"})

	want := "  ⚠ post_release  hook failed\n" +
		"  ⚠ cleanup  temp dir left behind\n"
	if got := out.String(); got != want {
		t.Errorf("multiple warn out = %q, want %q", got, want)
	}
	wantErr := "⚠ post_release  hook failed\n" +
		"⚠ cleanup  temp dir left behind\n"
	if got := errBuf.String(); got != wantErr {
		t.Errorf("multiple warn err = %q, want %q", got, wantErr)
	}
}

// TestPrettyPresenterWarnDoesNotSuppressSuccessEndOfRunLine proves a warning is
// independent of run state in pretty mode: a warn-only run still ends with the
// success brand line. Warn must not flip the run to failure or suppress the
// end-of-run line.
func TestPrettyPresenterWarnDoesNotSuppressSuccessEndOfRunLine(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.Warn(presenter.Warning{Label: "post_release", Message: "hook failed"})
		p.RunFinished(presenter.RunResult{Project: "acme", Version: "1.4.0", URL: "https://example/v1.4.0"})
	})

	if !strings.Contains(out.String(), "released acme v1.4.0") {
		t.Errorf("success end-of-run brand line missing after a warn:\n%q", out.String())
	}
}

// TestPrettyPresenterUnwoundRendersGlyphLineWithVerbatimSummary is the core
// pretty Unwound acceptance: the auto-unwind line mirrors the StageSucceeded line
// shape — two-space indent, the ↩ glyph, the literal "unwound" padded to the stage
// column (7 chars → 4 trailing spaces to reach column 11), then the engine-supplied
// summary VERBATIM, including its own "— repo clean" tail. The no-colour profile
// keeps the assertion on the exact layout; the colour-on profile proves the glyph
// is styled. Both are asserted to lock the exact line and the styling.
func TestPrettyPresenterUnwoundRendersGlyphLineWithVerbatimSummary(t *testing.T) {
	const summary = "removed tag v1.4.0, reset 2 release commit(s) — repo clean"

	t.Run("colour downgrade renders the exact layout", func(t *testing.T) {
		t.Parallel()

		out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
			p.Unwound(presenter.Unwind{Summary: summary})
		})

		want := "  ↩ unwound    " + summary + "\n"
		if got := out.String(); got != want {
			t.Errorf("unwound line = %q, want %q", got, want)
		}
		if bytes.ContainsRune(out.Bytes(), 0x1b) {
			t.Errorf("downgraded unwound line leaked an SGR code:\n%q", out.String())
		}
	})

	t.Run("colour on styles the glyph while layout survives", func(t *testing.T) {
		t.Parallel()

		out := drivePretty(termenv.TrueColor, func(p *presenter.PrettyPresenter) {
			p.Unwound(presenter.Unwind{Summary: summary})
		})

		if !bytes.ContainsRune(out.Bytes(), 0x1b) {
			t.Errorf("colour-on unwound line contains no ESC (0x1b) — expected the ↩ glyph styled:\n%q", out.String())
		}
		if !strings.Contains(out.String(), "↩") {
			t.Errorf("↩ glyph missing from colour-on unwound line:\n%q", out.String())
		}
		// The label padded to the column and the verbatim summary survive the styling.
		if !strings.Contains(out.String(), "unwound    "+summary) {
			t.Errorf("padded label + verbatim summary missing:\n%q", out.String())
		}
	})
}

// TestPrettyPresenterUnwoundWritesToStdoutOnly proves the stream contract for
// Unwound: with colour forced on, the styled line reaches stdout (ANSI present)
// while err stays EMPTY — the auto-unwind line is narration only and is NOT
// duplicated to stderr, unlike the ✗/⚠ summaries.
func TestPrettyPresenterUnwoundWritesToStdoutOnly(t *testing.T) {
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	p := presenter.NewPrettyPresenter(out, presenter.WithErr(errBuf), presenter.WithProfile(termenv.TrueColor))
	p.Unwound(presenter.Unwind{Summary: "removed tag v1.4.0, reset 2 release commit(s) — repo clean"})

	if !strings.Contains(out.String(), "↩") {
		t.Errorf("pretty: unwound line missing from stdout:\n%q", out.String())
	}
	if errBuf.Len() != 0 {
		t.Errorf("pretty: Unwound wrote to stderr = %q, want empty", errBuf.String())
	}
}

// TestPrettyPresenterStageFailedThenRunFinishedSuppressesSuccessLine proves the
// success bottom brand line is SUPPRESSED after a StageFailed: the failure run
// ends after the ✗ line with no closing brand line.
func TestPrettyPresenterStageFailedThenRunFinishedSuppressesSuccessLine(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected: remote moved"})
		p.RunFinished(presenter.RunResult{Project: "acme", Version: "1.4.0", URL: "https://example/v1.4.0"})
	})

	if strings.Contains(out.String(), "released") {
		t.Errorf("success brand line must be suppressed after a StageFailed, got:\n%q", out.String())
	}
}

// TestPrettyPresenterUnwoundAfterFailureSuppressesSuccessLine proves the failure
// path in pretty mode: StageFailed → Unwound → RunFinished renders the ✗ and ↩
// lines but suppresses the closing brand line.
func TestPrettyPresenterUnwoundAfterFailureSuppressesSuccessLine(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected: remote moved"})
		p.Unwound(presenter.Unwind{Summary: "removed tag v1.4.0, reset 2 release commit(s) — repo clean"})
		p.RunFinished(presenter.RunResult{Project: "acme", Version: "1.4.0", URL: "https://example/v1.4.0"})
	})

	got := out.String()
	if !strings.Contains(got, "  ✗ tag/push") {
		t.Errorf("✗ line missing:\n%q", got)
	}
	if !strings.Contains(got, "  ↩ unwound") {
		t.Errorf("↩ line missing:\n%q", got)
	}
	if strings.Contains(got, "released") {
		t.Errorf("success brand line must be suppressed after a failure+unwind, got:\n%q", got)
	}
}

// TestPrettyPresenterUnwoundAfterAbortSuppressesSuccessLine proves the abort path
// (gate-n) in pretty mode: an Unwound with NO prior StageFailed still suppresses
// the closing brand line on the subsequent RunFinished.
func TestPrettyPresenterUnwoundAfterAbortSuppressesSuccessLine(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.Unwound(presenter.Unwind{Summary: "removed tag v1.4.0, reset 2 release commit(s) — repo clean"})
		p.RunFinished(presenter.RunResult{Project: "acme", Version: "1.4.0", URL: "https://example/v1.4.0"})
	})

	got := out.String()
	if !strings.Contains(got, "  ↩ unwound") {
		t.Errorf("↩ line missing:\n%q", got)
	}
	if strings.Contains(got, "released") {
		t.Errorf("success brand line must be suppressed after an abort unwind, got:\n%q", got)
	}
}

// prettyCapturedOutput is the worked-example captured underlying-command output
// shared across the pretty StageFailed body tests: multi-line git chatter with an
// internal blank line.
const prettyCapturedOutput = "fatal: failed to push some refs to 'origin'\n" +
	"\n" +
	"hint: Updates were rejected because the remote contains work\n" +
	"hint: that you do not have locally."

// TestPrettyPresenterStageFailedRendersCapturedOutputBelowGlyphLine is the core
// pretty acceptance for captured output: below the red ✗ line, the captured body
// is rendered to OUT verbatim (no box). The no-colour profile keeps the assertion
// on layout — the ✗ line then each captured-body line, in order.
func TestPrettyPresenterStageFailedRendersCapturedOutputBelowGlyphLine(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected: remote moved", Output: prettyCapturedOutput})
	})

	got := out.String()
	glyphLine := "  ✗ tag/push"
	glyphIdx := strings.Index(got, glyphLine)
	if glyphIdx < 0 {
		t.Fatalf("✗ failure line %q not found:\n%q", glyphLine, got)
	}
	// The captured body follows the ✗ line — verbatim, so each source line appears
	// after the glyph line.
	bodyIdx := strings.Index(got, prettyCapturedOutput)
	if bodyIdx < 0 {
		t.Fatalf("captured body not rendered verbatim below the ✗ line:\n%q", got)
	}
	if bodyIdx <= glyphIdx {
		t.Errorf("captured body must follow the ✗ line: glyph=%d body=%d\n%q", glyphIdx, bodyIdx, got)
	}
}

// TestPrettyPresenterStageFailedSummaryToStderrWithoutBody proves the stream
// contract: the one-line summary reaches err, but the multi-line captured body
// does NOT — only the summary is duplicated to stderr for redirect-visibility.
func TestPrettyPresenterStageFailedSummaryToStderrWithoutBody(t *testing.T) {
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	p := presenter.NewPrettyPresenter(out, presenter.WithErr(errBuf), presenter.WithProfile(termenv.Ascii))
	p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected: remote moved", Output: prettyCapturedOutput})

	// The captured body reaches out (narration) ...
	if !strings.Contains(out.String(), prettyCapturedOutput) {
		t.Errorf("captured body missing from stdout narration:\n%q", out.String())
	}
	// ... but err carries ONLY the one-line summary, never the body.
	wantErr := "✗ tag/push  push rejected: remote moved\n"
	if got := errBuf.String(); got != wantErr {
		t.Errorf("stderr = %q, want exactly the one-line summary %q", got, wantErr)
	}
	for _, line := range strings.Split(prettyCapturedOutput, "\n") {
		if line != "" && strings.Contains(errBuf.String(), line) {
			t.Errorf("captured body line %q leaked to stderr = %q", line, errBuf.String())
		}
	}
}

// TestPrettyPresenterStageFailedEmptyOutputRendersGlyphLineAlone covers the
// empty-output edge: with no captured output the ✗ line stands alone — no body
// block follows it.
func TestPrettyPresenterStageFailedEmptyOutputRendersGlyphLineAlone(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected: remote moved", Output: ""})
	})

	// The name is padStage-padded to the stage column (8-char "tag/push" → three
	// trailing spaces to reach column 11), matching the existing StageFailed line.
	want := "  ✗ tag/push   push rejected: remote moved\n"
	if got := out.String(); got != want {
		t.Errorf("empty-output StageFailed = %q, want exactly the ✗ line alone %q", got, want)
	}
}

// TestPrettyPresenterStageFailedDelimiterLikeBodyLineIsVerbatim covers the
// delimiter-collision edge in pretty mode: a captured-body line that reads like a
// plain closing delimiter is written through verbatim — pretty renders no such
// delimiter of its own, so the line simply survives in the body unchanged.
func TestPrettyPresenterStageFailedDelimiterLikeBodyLineIsVerbatim(t *testing.T) {
	body := "real chatter\n--- end output ---\nstill chatter"
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "boom", Output: body})
	})

	if !strings.Contains(out.String(), body) {
		t.Errorf("delimiter-like captured body not written verbatim in pretty:\n%q", out.String())
	}
}

// TestPrettyPresenterStageFailedMultiLineBlankLinesPreserved covers the
// multi-line edge: a captured body with internal blank lines round-trips exactly
// below the ✗ line — no collapsing, no re-wrapping, no truncation.
func TestPrettyPresenterStageFailedMultiLineBlankLinesPreserved(t *testing.T) {
	body := "line one\n\n\nline four after two blanks"
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "boom", Output: body})
	})

	if !strings.Contains(out.String(), body) {
		t.Errorf("multi-line blank lines not preserved verbatim in pretty:\n%q", out.String())
	}
}

// TestPrettyPresenterStageFailedBodySurvivesColourDowngrade asserts the captured
// body bytes survive the no-colour profile intact: even if the presenter wraps the
// body in a dim style, the colour downgrade leaves the verbatim body text in place
// (styling is additive; the body bytes are load-bearing).
func TestPrettyPresenterStageFailedBodySurvivesColourDowngrade(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "boom", Output: prettyCapturedOutput})
	})

	if bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("downgraded failure block leaked an SGR code:\n%q", out.String())
	}
	if !strings.Contains(out.String(), prettyCapturedOutput) {
		t.Errorf("captured body not preserved verbatim under colour downgrade:\n%q", out.String())
	}
}

// prettyNotesBody mirrors the plain test's worked-example notes body: a lead
// line, a blank line, then the emoji-headed Features/Fixes sections. Shared here
// so the pretty/plain byte-identity assertions render the same source bytes.
const prettyNotesBody = "Faster cold starts and a calmer log.\n" +
	"\n" +
	"✨ Features\n" +
	"- Parallel warm-up halves boot time\n" +
	"🐛 Fixes\n" +
	"- Stop double-flush on SIGTERM"

// The expected titled/closing notes rules (notesTitledRule/notesClosingRule), the
// title-prefix literal, and the rule-width source live in the shared
// pretty_helpers_test.go so the prefix + fill/clamp arithmetic and the cap appear
// exactly once on the test side.

// TestPrettyPresenterShowNotesWrapsBodyInTitledRules is the core pretty
// acceptance: ShowNotes renders a titled opener rule, the body verbatim (flush,
// NOT indented), and a closing rule — and crucially NO box-drawing border
// (╭ ╮ ╰ ╯ │) surrounds the body. The no-colour profile keeps the assertion on
// the exact layout rather than ANSI bytes.
func TestPrettyPresenterShowNotesWrapsBodyInTitledRules(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: prettyNotesBody})
	})

	want := notesTitledRule("1.4.0") + "\n" +
		prettyNotesBody + "\n" +
		notesClosingRule() + "\n"
	if got := out.String(); got != want {
		t.Errorf("ShowNotes titled-rule layout mismatch\n got: %q\nwant: %q", got, want)
	}

	// No rounded-box border characters may surround the body — the box was dropped.
	for _, boxChar := range []string{"╭", "╮", "╰", "╯", "│"} {
		if strings.Contains(out.String(), boxChar) {
			t.Errorf("box-drawing border char %q present — the rounded box was dropped:\n%q", boxChar, out.String())
		}
	}
}

// TestPrettyPresenterShowNotesPreservesEmojiHeaders proves the emoji section
// headers survive verbatim in pretty mode too — the body is written through the
// shared verbatim helper, never stripped or transformed.
func TestPrettyPresenterShowNotesPreservesEmojiHeaders(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: prettyNotesBody})
	})

	got := out.String()
	for _, header := range []string{"✨ Features", "🐛 Fixes"} {
		if !strings.Contains(got, header) {
			t.Errorf("emoji header %q stripped from pretty notes body:\n%q", header, got)
		}
	}
}

// TestPrettyPresenterShowNotesEmptyBodyRendersBareRules covers the empty-body
// edge: the titled rule is immediately followed by the closing rule with NO
// spurious blank line or invented content between them — consistent with plain.
func TestPrettyPresenterShowNotesEmptyBodyRendersBareRules(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: ""})
	})

	want := notesTitledRule("1.4.0") + "\n" +
		notesClosingRule() + "\n"
	if got := out.String(); got != want {
		t.Errorf("empty-body pretty notes = %q, want %q", got, want)
	}
}

// TestPrettyPresenterShowNotesDelimiterLikeBodyLineIsVerbatim covers the
// delimiter-collision edge in pretty mode: a body line that reads like a plain
// closing delimiter is written through verbatim; the REAL closing rule still
// follows. Delimiters/rules are positional, never content-matched.
func TestPrettyPresenterShowNotesDelimiterLikeBodyLineIsVerbatim(t *testing.T) {
	body := "real notes\n--- end notes ---\nstill notes"
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: body})
	})

	want := notesTitledRule("1.4.0") + "\n" +
		body + "\n" +
		notesClosingRule() + "\n"
	if got := out.String(); got != want {
		t.Errorf("delimiter-like body line not written verbatim in pretty\n got: %q\nwant: %q", got, want)
	}
}

// TestPrettyPresenterShowNotesMultiLineBlankLinesPreserved covers the multi-line
// edge: internal blank lines round-trip exactly — no collapsing, no re-wrapping,
// no truncation.
func TestPrettyPresenterShowNotesMultiLineBlankLinesPreserved(t *testing.T) {
	body := "line one\n\n\nline four after two blanks"
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.ShowNotes(presenter.Notes{Version: "2.0.0", Body: body})
	})

	want := notesTitledRule("2.0.0") + "\n" +
		body + "\n" +
		notesClosingRule() + "\n"
	if got := out.String(); got != want {
		t.Errorf("multi-line blank lines not preserved in pretty\n got: %q\nwant: %q", got, want)
	}
}

// TestPrettyPresenterShowNotesRulesSurviveColourDowngrade asserts the rule layout
// (the title text and the U+2500 rule characters) survives the no-colour profile
// with no SGR codes leaking — the rules may be dim-styled, but the layout must
// remain intact under downgrade.
func TestPrettyPresenterShowNotesRulesSurviveColourDowngrade(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: "hi"})
	})

	if bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("downgraded notes block leaked an SGR code:\n%q", out.String())
	}
	if !strings.Contains(out.String(), "release notes · v1.4.0") {
		t.Errorf("titled rule text not preserved under colour downgrade:\n%q", out.String())
	}
	if !strings.Contains(out.String(), "─") {
		t.Errorf("rule character U+2500 not preserved under colour downgrade:\n%q", out.String())
	}
}

// TestPrettyPresenterShowNotesRulesStyledUnderColour forces a colour-capable
// profile and asserts the rules carry ANSI SGR escapes (they are dim-styled)
// while the rule layout text survives — styling is additive, layout is fixed.
func TestPrettyPresenterShowNotesRulesStyledUnderColour(t *testing.T) {
	out := drivePretty(termenv.TrueColor, func(p *presenter.PrettyPresenter) {
		p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: "hi"})
	})

	if !bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("colour-on notes block contains no ESC (0x1b) — expected dim SGR codes:\n%q", out.String())
	}
	if !strings.Contains(out.String(), "release notes · v1.4.0") {
		t.Errorf("titled rule text missing under colour:\n%q", out.String())
	}
}

// TestShowNotesBodyIsByteIdenticalAcrossModes is THE non-negotiable invariant:
// the same Notes rendered in plain and pretty produce a BYTE-IDENTICAL body
// region. The body is extracted from each rendering by stripping the
// mode-specific delimiter/rule lines (the first and last lines) and the two
// presenters' inner bytes are compared byte-for-byte.
func TestShowNotesBodyIsByteIdenticalAcrossModes(t *testing.T) {
	notes := presenter.Notes{Version: "1.4.0", Body: prettyNotesBody}

	plainOut := &bytes.Buffer{}
	presenter.NewPlainPresenter(plainOut, &bytes.Buffer{}).ShowNotes(notes)

	prettyOut := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.ShowNotes(notes)
	})

	plainBody := extractNotesBody(t, plainOut.String())
	prettyBody := extractNotesBody(t, prettyOut.String())

	if plainBody != prettyBody {
		t.Errorf("notes body differs across modes\nplain : %q\npretty: %q", plainBody, prettyBody)
	}
	// And the extracted body must equal the source body byte-for-byte — proving
	// neither mode mutated it.
	if plainBody != prettyNotesBody {
		t.Errorf("plain body mutated the source\n got: %q\nwant: %q", plainBody, prettyNotesBody)
	}
}

// extractNotesBody removes the first line (the opener delimiter/rule) and the
// last line (the closing delimiter/rule) from a rendered notes block, returning
// the inner body region. Delimiters are positional — exactly the first and last
// of the rendered lines — so this slice is mode-agnostic and lets the two
// renderings be compared on body bytes alone.
func extractNotesBody(t *testing.T, rendered string) string {
	t.Helper()
	trimmed := strings.TrimSuffix(rendered, "\n")
	lines := strings.Split(trimmed, "\n")
	if len(lines) < 2 {
		t.Fatalf("rendered notes block has too few lines to extract a body: %q", rendered)
	}
	return strings.Join(lines[1:len(lines)-1], "\n")
}

// TestPrettyPresenterBlockingStageStartedStartsSpinnerNoStaticLine is the updated
// behaviour for this phase (Phase 4): a BLOCKING StageStarted starts a spinner
// rather than printing the Phase-1 placeholder static-dim-line. Driven through the
// spy factory, the spinner is Started and the presenter writes no static "notes"
// start line of its own (the start text is the spinner's suffix, animated by the
// library — here the spy writes nothing). This deliberately replaces the old
// TestPrettyPresenterStageStartedRendersStaticLine: short stages and the spinner now
// own the start-line behaviour. The full spinner lifecycle is exercised in
// pretty_spinner_test.go.
func TestPrettyPresenterBlockingStageStartedStartsSpinnerNoStaticLine(t *testing.T) {
	out, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
		p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
	})

	if len(tr.created) != 1 || !tr.created[0].started {
		t.Fatalf("a blocking StageStarted must start exactly one spinner, got created=%d", len(tr.created))
	}
	// No Phase-1 placeholder static start line is printed by the presenter itself —
	// the spy spinner renders nothing, so out is empty (no carriage-return animation
	// either, since the spy does not animate).
	if got := out.String(); got != "" {
		t.Errorf("blocking StageStarted must print no static start line of its own, got %q", got)
	}
}
