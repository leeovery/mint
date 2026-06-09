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
	p := presenter.NewPrettyPresenterWithProfile(out, profile)
	fn(p)
	return out
}

// TestPrettyPresenterSatisfiesInterface is the compile-time and runtime proof that
// PrettyPresenter is usable wherever a Presenter is required.
func TestPrettyPresenterSatisfiesInterface(t *testing.T) {
	var _ presenter.Presenter = (*presenter.PrettyPresenter)(nil)

	var p presenter.Presenter = presenter.NewPrettyPresenter(&bytes.Buffer{}, &bytes.Buffer{})
	p.RunStarted(presenter.RunInfo{})
	p.StageStarted(presenter.StageStart{})
	p.StageSucceeded(presenter.StageSuccess{})
	p.StageFailed(presenter.StageFailure{})
	p.RunFinished(presenter.RunResult{})
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

// TestPrettyPresenterStageStartedRendersStaticLine asserts StageStarted prints a
// single static (non-animated) stage line — no spinner lifecycle this phase. It is
// rendered dim under colour and as a plain glyph-less line under downgrade.
func TestPrettyPresenterStageStartedRendersStaticLine(t *testing.T) {
	out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
		p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
	})

	got := out.String()
	if !strings.Contains(got, "notes") {
		t.Errorf("StageStarted line missing the stage name:\n%q", got)
	}
	// A single printed line keeps the flow linear: no carriage-return animation.
	if bytes.ContainsRune(out.Bytes(), 0x0d) {
		t.Errorf("StageStarted emitted a carriage return (0x0d) — spinner animation is out of scope this phase:\n%q", got)
	}
	if strings.Count(got, "\n") != 1 {
		t.Errorf("StageStarted should print exactly one line, got %d newlines:\n%q", strings.Count(got, "\n"), got)
	}
}
