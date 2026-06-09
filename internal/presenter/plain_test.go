package presenter_test

import (
	"bytes"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"mint/internal/presenter"
)

// productionGoFiles globs the presenter package's non-test .go sources, which the
// import guard scans. Test files are excluded: they may import anything; the
// production code is the contract.
func productionGoFiles(t *testing.T) []string {
	t.Helper()

	all, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("globbing package sources: %v", err)
	}

	var production []string
	for _, f := range all {
		if !strings.HasSuffix(f, "_test.go") {
			production = append(production, f)
		}
	}
	if len(production) == 0 {
		t.Fatal("found no non-test .go files to scan — the guard would be vacuous")
	}
	return production
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
		{name: "blocking stage emits a terse start line", blocking: true, want: "notes: ...\n"},
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

// TestPlainPresenterImportsNoUILibrary is the dependency guard: it parses the
// presenter package's own source and asserts none of its imports name a UI or
// animation library. Parsing the source (rather than go list -deps) keeps the
// check hermetic and CI-safe while still failing loudly if plain.go ever reaches
// for lipgloss or a spinner.
func TestPlainPresenterImportsNoUILibrary(t *testing.T) {
	fset := token.NewFileSet()

	for _, path := range productionGoFiles(t) {
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
