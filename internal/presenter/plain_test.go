package presenter_test

import (
	"bytes"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mint/internal/presenter"
)

// plainPresenterSources are the non-test sources whose imports the UI-library
// guard scans. The guard protects the *plain* presenter's defining property —
// token-efficiency via zero UI dependencies — so it scans only that presenter's
// own source, not the whole package. The pretty presenter (pretty.go)
// legitimately imports lipgloss, so it is deliberately excluded; mixing it in
// would assert a package-wide property the spec never makes.
func plainPresenterSources(t *testing.T) []string {
	t.Helper()

	const src = "plain.go"
	if _, err := filepath.Glob(src); err != nil {
		t.Fatalf("globbing %s: %v", src, err)
	}
	return []string{src}
}

// uiLibraryMarkers are import-path fragments that betray a UI/animation library.
// The plain presenter's whole point is token-efficiency: it must pull in none of
// these. The guard test below scans the package's source imports for them.
var uiLibraryMarkers = []string{
	"lipgloss",
	"charmbracelet",
	"briandowns/spinner",
	"spinner",
	"bubbletea",
	"fatih/color",
}

// drive runs the supplied callback against a freshly constructed PlainPresenter
// whose out and err writers are captured into the returned buffers. Centralising
// construction keeps each test focused on the event sequence it exercises.
func drive(fn func(p *presenter.PlainPresenter)) (out, errBuf *bytes.Buffer) {
	out = &bytes.Buffer{}
	errBuf = &bytes.Buffer{}
	p := presenter.NewPlainPresenter(out, errBuf)
	fn(p)
	return out, errBuf
}

// TestPlainPresenterSatisfiesInterface is the compile-time and runtime proof that
// PlainPresenter is usable wherever a Presenter is required.
func TestPlainPresenterSatisfiesInterface(t *testing.T) {
	var _ presenter.Presenter = (*presenter.PlainPresenter)(nil)

	var p presenter.Presenter = presenter.NewPlainPresenter(&bytes.Buffer{}, &bytes.Buffer{})
	p.RunStarted(presenter.RunInfo{})
	p.StageStarted(presenter.StageStart{})
	p.StageSucceeded(presenter.StageSuccess{})
	p.StageFailed(presenter.StageFailure{})
	p.RunFinished(presenter.RunResult{})
}

// TestPlainPresenterRendersMinimalSequence drives the walking-skeleton sequence
// (start-of-run -> stage success -> end-of-run) and asserts the exact terse lines
// in order — the core acceptance criterion.
func TestPlainPresenterRendersMinimalSequence(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.4.0", Action: "releasing"})
		p.StageSucceeded(presenter.StageSuccess{Name: "preflight", Detail: "clean, on main, tag free, in sync"})
		p.RunFinished(presenter.RunResult{Project: "acme", Version: "1.4.0", URL: "https://github.com/acme/acme/releases/tag/v1.4.0"})
	})

	want := "mint: releasing acme v1.4.0\n" +
		"preflight: clean, on main, tag free, in sync\n" +
		"done: acme v1.4.0 https://github.com/acme/acme/releases/tag/v1.4.0\n"

	if got := out.String(); got != want {
		t.Errorf("output mismatch\n got: %q\nwant: %q", got, want)
	}
}

// TestPlainPresenterStartLineUsesEngineAction proves the start-of-run line renders
// the engine-supplied verb word — never a hardcoded "releasing" — so every verb
// (release, regenerate, …) narrates correctly through the same presenter.
func TestPlainPresenterStartLineUsesEngineAction(t *testing.T) {
	tests := []struct {
		name   string
		action string
		want   string
	}{
		{name: "release verb", action: "releasing", want: "mint: releasing acme v1.4.0\n"},
		{name: "regenerate verb", action: "regenerating", want: "mint: regenerating acme v1.4.0\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, _ := drive(func(p *presenter.PlainPresenter) {
				p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.4.0", Action: tt.action})
			})

			if got := out.String(); got != tt.want {
				t.Errorf("RunStarted action %q = %q, want %q", tt.action, got, tt.want)
			}
		})
	}
}

// TestPlainPresenterStageSucceededFallsBackToOk asserts the detail-less success
// line renders the "ok" floor rather than a dangling "{stage}: ".
func TestPlainPresenterStageSucceededFallsBackToOk(t *testing.T) {
	tests := []struct {
		name   string
		detail string
		want   string
	}{
		{name: "with detail", detail: "pre_tag ok (2.3s)", want: "prep: pre_tag ok (2.3s)\n"},
		{name: "empty detail", detail: "", want: "prep: ok\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, _ := drive(func(p *presenter.PlainPresenter) {
				p.StageSucceeded(presenter.StageSuccess{Name: "prep", Detail: tt.detail})
			})

			if got := out.String(); got != tt.want {
				t.Errorf("StageSucceeded detail %q = %q, want %q", tt.detail, got, tt.want)
			}
		})
	}
}

// TestPlainPresenterStageStartedHonoursBlocking proves plain's spinner-equivalent:
// a blocking (long) stage emits a terse start line so a live-tail consumer isn't
// staring at silence, while a short stage stays silent until completion.
func TestPlainPresenterStageStartedHonoursBlocking(t *testing.T) {
	tests := []struct {
		name     string
		blocking bool
		want     string
	}{
		{name: "short stage emits nothing on start", blocking: false, want: ""},
		{name: "blocking stage emits a terse start line", blocking: true, want: "notes: generating...\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, _ := drive(func(p *presenter.PlainPresenter) {
				p.StageStarted(presenter.StageStart{Name: "notes", Blocking: tt.blocking})
			})

			if got := out.String(); got != tt.want {
				t.Errorf("StageStarted blocking=%v = %q, want %q", tt.blocking, got, tt.want)
			}
		})
	}
}

// TestPlainPresenterStageSucceededElapsedSuffix proves the elapsed suffix is gated
// on Blocking, not on the Elapsed value: a blocking stage's completion line carries
// ({elapsed}); a short stage's never does — even when an Elapsed travels with it.
func TestPlainPresenterStageSucceededElapsedSuffix(t *testing.T) {
	tests := []struct {
		name    string
		success presenter.StageSuccess
		want    string
	}{
		{
			name:    "blocking with detail appends elapsed after detail",
			success: presenter.StageSuccess{Name: "notes", Detail: "generated", Elapsed: 1100 * time.Millisecond, Blocking: true},
			want:    "notes: generated (1.1s)\n",
		},
		{
			name:    "blocking without detail appends elapsed after ok",
			success: presenter.StageSuccess{Name: "preflight", Detail: "", Elapsed: 2300 * time.Millisecond, Blocking: true},
			want:    "preflight: ok (2.3s)\n",
		},
		{
			name:    "short stage carries no elapsed even when Elapsed is set",
			success: presenter.StageSuccess{Name: "preflight", Detail: "clean", Elapsed: 5 * time.Second, Blocking: false},
			want:    "preflight: clean\n",
		},
		{
			name:    "blocking with zero elapsed still renders an elapsed suffix",
			success: presenter.StageSuccess{Name: "notes", Detail: "generated", Elapsed: 0, Blocking: true},
			want:    "notes: generated (0.0s)\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, _ := drive(func(p *presenter.PlainPresenter) {
				p.StageSucceeded(tt.success)
			})

			if got := out.String(); got != tt.want {
				t.Errorf("StageSucceeded(%+v) = %q, want %q", tt.success, got, tt.want)
			}
		})
	}
}

// TestPlainPresenterShortStageEmitsOnlyCompletionLine drives a full short-stage
// transition (start then success, both Blocking==false) and asserts exactly one
// line — the completion — with no start line and no elapsed suffix.
func TestPlainPresenterShortStageEmitsOnlyCompletionLine(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.StageStarted(presenter.StageStart{Name: "preflight", Blocking: false})
		p.StageSucceeded(presenter.StageSuccess{Name: "preflight", Detail: "clean", Blocking: false})
	})

	want := "preflight: clean\n"
	if got := out.String(); got != want {
		t.Errorf("short stage output = %q, want %q", got, want)
	}
}

// TestPlainPresenterLongStageEmitsStartThenCompletion drives a full blocking-stage
// transition and asserts two lines in order — the terse start line then the
// completion line — with the completion carrying the elapsed suffix.
func TestPlainPresenterLongStageEmitsStartThenCompletion(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
		p.StageSucceeded(presenter.StageSuccess{Name: "notes", Detail: "generated", Elapsed: 1100 * time.Millisecond, Blocking: true})
	})

	want := "notes: generating...\n" +
		"notes: generated (1.1s)\n"
	if got := out.String(); got != want {
		t.Errorf("long stage output = %q, want %q", got, want)
	}
}

// TestPlainPresenterStageFailedRendersOneLineSummary asserts the one-line failure
// summary. The captured-output delimiter block and stderr duplication are later
// phases — this task owns only the single summary line.
func TestPlainPresenterStageFailedRendersOneLineSummary(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected: remote moved"})
	})

	want := "tag/push: FAILED - push rejected: remote moved\n"
	if got := out.String(); got != want {
		t.Errorf("StageFailed = %q, want %q", got, want)
	}
}

// TestPlainPresenterRunFinishedOmitsTrailingSpaceWhenURLEmpty covers the empty-URL
// edge case (e.g. regenerate, which publishes no release): the done line must not
// dangle a trailing space where the URL would be.
func TestPlainPresenterRunFinishedOmitsTrailingSpaceWhenURLEmpty(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "with url",
			url:  "https://github.com/acme/acme/releases/tag/v1.4.0",
			want: "done: acme v1.4.0 https://github.com/acme/acme/releases/tag/v1.4.0\n",
		},
		{
			name: "empty url has no trailing space",
			url:  "",
			want: "done: acme v1.4.0\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, _ := drive(func(p *presenter.PlainPresenter) {
				p.RunFinished(presenter.RunResult{Project: "acme", Version: "1.4.0", URL: tt.url})
			})

			if got := out.String(); got != tt.want {
				t.Errorf("RunFinished url %q = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

// TestPlainPresenterEmitsNoANSIGlyphOrAnimationBytes scans every byte of a full
// run's narration: no ESC (0x1b) for ANSI, no carriage return (0x0d) for in-place
// animation, and nothing above the basic ASCII range the plain contract uses.
func TestPlainPresenterEmitsNoANSIGlyphOrAnimationBytes(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.4.0", Action: "releasing"})
		p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
		p.StageSucceeded(presenter.StageSuccess{Name: "preflight", Detail: "clean, on main"})
		p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected"})
		p.RunFinished(presenter.RunResult{Project: "acme", Version: "1.4.0", URL: "https://example/v1.4.0"})
	})

	for i, b := range out.Bytes() {
		switch {
		case b == 0x1b:
			t.Errorf("byte %d is ESC (0x1b) — ANSI escape leaked into plain output", i)
		case b == 0x0d:
			t.Errorf("byte %d is CR (0x0d) — carriage-return animation leaked into plain output", i)
		case b == '\n':
			// the only permitted control byte: a line terminator
		case b < 0x20 || b > 0x7e:
			t.Errorf("byte %d = 0x%02x is outside the printable ASCII range the plain contract uses", i, b)
		}
	}
}

// TestPlainPresenterShowPlanJoinsStepsIntoOneLiner is the core plain acceptance:
// ShowPlan renders one "plan: …" line with each step rendered "{verb} {target}"
// and joined by "; " — derived from the SAME structured steps, with no separate
// pre-formatted/terse field.
func TestPlainPresenterShowPlanJoinsStepsIntoOneLiner(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.ShowPlan(presenter.Plan{Steps: []presenter.PlanStep{
			{Verb: "commit", Target: "changelog+version"},
			{Verb: "tag", Target: "v1.4.0"},
			{Verb: "push", Target: "--atomic"},
			{Verb: "publish", Target: "github"},
		}})
	})

	want := "plan: commit changelog+version; tag v1.4.0; push --atomic; publish github\n"
	if got := out.String(); got != want {
		t.Errorf("ShowPlan = %q, want %q", got, want)
	}
}

// TestPlainPresenterShowPlanSingleStepHasNoSeparator covers the single-step edge:
// exactly one "{verb} {target}" with no "; " separator dangling on either side.
func TestPlainPresenterShowPlanSingleStepHasNoSeparator(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.ShowPlan(presenter.Plan{Steps: []presenter.PlanStep{
			{Verb: "tag", Target: "v1.4.0"},
		}})
	})

	want := "plan: tag v1.4.0\n"
	got := out.String()
	if got != want {
		t.Errorf("ShowPlan = %q, want %q", got, want)
	}
	if strings.Contains(got, ";") {
		t.Errorf("single-step plan must contain no separator, got %q", got)
	}
}

// TestPlainPresenterShowPlanEmptyHasNoDanglingSeparator covers the empty-plan
// edge: no steps renders exactly "plan:" — no trailing space, no "; ".
func TestPlainPresenterShowPlanEmptyHasNoDanglingSeparator(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.ShowPlan(presenter.Plan{})
	})

	want := "plan:\n"
	got := out.String()
	if got != want {
		t.Errorf("empty ShowPlan = %q, want %q", got, want)
	}
	if strings.Contains(got, ";") {
		t.Errorf("empty plan must contain no separator, got %q", got)
	}
}

// TestPlainPresenterShowPlanEmptyTargetRendersVerbOnly covers the empty-target
// edge: a step with no target contributes just "{verb}" — no trailing space.
func TestPlainPresenterShowPlanEmptyTargetRendersVerbOnly(t *testing.T) {
	tests := []struct {
		name  string
		steps []presenter.PlanStep
		want  string
	}{
		{
			name:  "single empty-target step renders just the verb",
			steps: []presenter.PlanStep{{Verb: "publish", Target: ""}},
			want:  "plan: publish\n",
		},
		{
			name: "empty-target step joined with others has no stray space",
			steps: []presenter.PlanStep{
				{Verb: "tag", Target: "v1.4.0"},
				{Verb: "publish", Target: ""},
			},
			want: "plan: tag v1.4.0; publish\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, _ := drive(func(p *presenter.PlainPresenter) {
				p.ShowPlan(presenter.Plan{Steps: tt.steps})
			})

			if got := out.String(); got != tt.want {
				t.Errorf("ShowPlan = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestPlainPresenterShowPlanEmitsNoANSIGlyphOrAnimationBytes guards the
// byte-purity contract for the plan one-liner specifically: the synthesised
// parts ("plan: ", "; ", spaces) are byte-pure ASCII — the "•" bullet is a
// PRETTY-only glyph and must never appear in plain output. (The targets are
// engine-supplied and rendered verbatim; this test uses ASCII targets so the
// whole line is asserted byte-pure.)
func TestPlainPresenterShowPlanEmitsNoANSIGlyphOrAnimationBytes(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.ShowPlan(presenter.Plan{Steps: []presenter.PlanStep{
			{Verb: "commit", Target: "changelog+version"},
			{Verb: "tag", Target: "v1.4.0"},
			{Verb: "publish", Target: ""},
		}})
	})

	for i, b := range out.Bytes() {
		switch {
		case b == 0x1b:
			t.Errorf("byte %d is ESC (0x1b) — ANSI escape leaked into plain plan output", i)
		case b == 0x0d:
			t.Errorf("byte %d is CR (0x0d) — carriage-return animation leaked into plain plan output", i)
		case b == '\n':
			// the only permitted control byte: a line terminator
		case b < 0x20 || b > 0x7e:
			t.Errorf("byte %d = 0x%02x is outside the printable ASCII range the plain plan contract uses", i, b)
		}
	}
}

// notesBody is the worked-example release-notes body shared across the
// byte-identity tests: a lead line, a blank line, then the emoji-headed
// Features/Fixes sections. It deliberately carries the ✨/🐛 emoji headers (which
// must survive verbatim in both modes) and an internal blank line.
const notesBody = "Faster cold starts and a calmer log.\n" +
	"\n" +
	"✨ Features\n" +
	"- Parallel warm-up halves boot time\n" +
	"🐛 Fixes\n" +
	"- Stop double-flush on SIGTERM"

// TestPlainPresenterShowNotesWrapsBodyInPlainDelimiters is the core plain
// acceptance: ShowNotes wraps the verbatim body in the sliceable plain rules
// "--- release notes v{X} ---" … "--- end notes ---", with the body bytes
// untouched between them.
func TestPlainPresenterShowNotesWrapsBodyInPlainDelimiters(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: notesBody})
	})

	want := "--- release notes v1.4.0 ---\n" +
		notesBody + "\n" +
		"--- end notes ---\n"
	if got := out.String(); got != want {
		t.Errorf("ShowNotes plain delimiters mismatch\n got: %q\nwant: %q", got, want)
	}
}

// TestPlainPresenterShowNotesPreservesEmojiHeaders proves the emoji section
// headers (✨ Features / 🐛 Fixes) are written byte-for-byte — no stripping or
// transforming — even though plain mode is otherwise byte-pure ASCII. The body is
// engine content, not synthesised narration, so it carries its emoji through.
func TestPlainPresenterShowNotesPreservesEmojiHeaders(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: notesBody})
	})

	got := out.String()
	for _, header := range []string{"✨ Features", "🐛 Fixes"} {
		if !strings.Contains(got, header) {
			t.Errorf("emoji header %q stripped from plain notes body:\n%q", header, got)
		}
	}
}

// TestPlainPresenterShowNotesEmptyBodyRendersBareDelimiters covers the empty-body
// edge: the opener line is immediately followed by the closer line with NO
// spurious blank line or invented content between them.
func TestPlainPresenterShowNotesEmptyBodyRendersBareDelimiters(t *testing.T) {
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: ""})
	})

	want := "--- release notes v1.4.0 ---\n" +
		"--- end notes ---\n"
	if got := out.String(); got != want {
		t.Errorf("empty-body plain notes = %q, want %q", got, want)
	}
}

// TestPlainPresenterShowNotesDelimiterLikeBodyLineIsVerbatim covers the
// delimiter-collision edge: a body line that itself reads like a closing
// delimiter is written through verbatim, and the REAL closing delimiter still
// follows it. Delimiters are positional, never content-matched.
func TestPlainPresenterShowNotesDelimiterLikeBodyLineIsVerbatim(t *testing.T) {
	body := "real notes\n--- end notes ---\nstill notes"
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: body})
	})

	want := "--- release notes v1.4.0 ---\n" +
		body + "\n" +
		"--- end notes ---\n"
	if got := out.String(); got != want {
		t.Errorf("delimiter-like body line not written verbatim\n got: %q\nwant: %q", got, want)
	}
	// The real closing delimiter is the LAST line — the body's lookalike does not
	// short-circuit it.
	if !strings.HasSuffix(out.String(), "still notes\n--- end notes ---\n") {
		t.Errorf("real closing delimiter must follow the verbatim body, got %q", out.String())
	}
}

// TestPlainPresenterShowNotesMultiLineBlankLinesPreserved covers the multi-line
// edge: a body with internal blank lines round-trips exactly — no collapsing, no
// re-wrapping.
func TestPlainPresenterShowNotesMultiLineBlankLinesPreserved(t *testing.T) {
	body := "line one\n\n\nline four after two blanks"
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.ShowNotes(presenter.Notes{Version: "2.0.0", Body: body})
	})

	want := "--- release notes v2.0.0 ---\n" +
		body + "\n" +
		"--- end notes ---\n"
	if got := out.String(); got != want {
		t.Errorf("multi-line blank lines not preserved\n got: %q\nwant: %q", got, want)
	}
}

// TestPlainPresenterImportsNoUILibrary is the dependency guard: it parses the
// plain presenter's own source and asserts none of its imports name a UI or
// animation library. Parsing the source (rather than go list -deps) keeps the
// check hermetic and CI-safe while still failing loudly if plain.go ever reaches
// for lipgloss or a spinner. It scans only plain.go — the pretty presenter is
// expected to import lipgloss, so a package-wide scan would be wrong.
func TestPlainPresenterImportsNoUILibrary(t *testing.T) {
	fset := token.NewFileSet()

	for _, path := range plainPresenterSources(t) {
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		for _, imp := range file.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			for _, marker := range uiLibraryMarkers {
				if strings.Contains(p, marker) {
					t.Errorf("%s imports %q which matches banned UI-library marker %q", filepath.Base(path), p, marker)
				}
			}
		}
	}
}
