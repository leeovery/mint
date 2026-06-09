package presenter_test

import (
	"bytes"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// subprocessMarkers are import-path fragments that betray an editor/subprocess
// dependency in the prompt path. The render-only Prompt contract is that the
// engine — never the presenter — invokes $EDITOR (edit) or claude (regenerate)
// on the e/r choices; the presenter only re-renders and returns a Choice. The
// guard below scans the package's NON-TEST source imports for these so the
// presenter provably CANNOT spawn an editor or any subprocess. The display
// labels ("edit in $EDITOR") and doc comments mentioning $EDITOR/claude are
// fine — the guard is about IMPORTS, not label or comment text.
var subprocessMarkers = []string{
	"os/exec",
	"syscall",
}

// presenterNonTestSources globs the presenter package's non-test .go files —
// the sources whose imports the subprocess guard scans. Unlike the plain-only
// UI-library guard (which scans just plain.go), the render-only contract is a
// WHOLE-PACKAGE property: neither presenter, nor the shared prompt core, may
// reach for an editor/subprocess, so every non-test source is scanned.
func presenterNonTestSources(t *testing.T) []string {
	t.Helper()

	all, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("globbing *.go: %v", err)
	}
	var sources []string
	for _, path := range all {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		sources = append(sources, path)
	}
	if len(sources) == 0 {
		t.Fatal("no non-test sources found to scan — the guard would be a false positive")
	}
	return sources
}

// TestPromptPathImportsNoSubprocessDependency is the dependency guard that LOCKS
// the render-only Prompt contract: it parses the presenter package's non-test
// sources and asserts NONE of their imports name os/exec (or any other
// subprocess-spawning package). This proves the presenter cannot invoke an
// editor/subprocess — the e/r work is the engine's. Parsing the source (rather
// than go list -deps) keeps the check hermetic and CI-safe while still failing
// loudly if any source ever reaches for os/exec. It MIRRORS the UI-library guard
// (TestPlainPresenterImportsNoUILibrary) and its go/parser ImportsOnly approach.
func TestPromptPathImportsNoSubprocessDependency(t *testing.T) {
	fset := token.NewFileSet()

	scanned := 0
	for _, path := range presenterNonTestSources(t) {
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		scanned++
		for _, imp := range file.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			for _, marker := range subprocessMarkers {
				if p == marker {
					t.Errorf("%s imports %q which matches banned subprocess marker %q — the presenter must never spawn an editor/subprocess; the engine owns the e/r work", filepath.Base(path), p, marker)
				}
			}
		}
	}
	// Defend against a glob/parse regression silently scanning nothing.
	if scanned == 0 {
		t.Fatal("scanned no sources — the subprocess guard never ran")
	}
}

// screenControlSequences are the terminal control sequences a LINEAR,
// append-only presenter must NEVER emit: clear-screen, alt-screen enter/leave,
// and cursor-home-to-overwrite. Their absence is what distinguishes mint's
// print-style scrolling narration from a Bubble Tea / full-screen TUI. SGR
// colour codes (ESC[...m) from lipgloss are deliberately NOT listed — they are
// styling, not screen control, and are expected in pretty mode.
var screenControlSequences = []struct {
	name string
	seq  string
}{
	{"clear-screen (ESC[2J)", "\x1b[2J"},
	{"alt-screen enter (ESC[?1049h)", "\x1b[?1049h"},
	{"alt-screen leave (ESC[?1049l)", "\x1b[?1049l"},
	{"cursor-home (ESC[H)", "\x1b[H"},
	{"cursor-home (ESC[;H)", "\x1b[;H"},
	{"cursor-home (ESC[1;1H)", "\x1b[1;1H"},
}

// assertNoScreenControl fails if out contains any clear-screen/alt-screen/
// cursor-home sequence — the proof the render scrolls linearly and never
// overwrites the terminal in place.
func assertNoScreenControl(t *testing.T, mode, out string) {
	t.Helper()
	for _, ctrl := range screenControlSequences {
		if strings.Contains(out, ctrl.seq) {
			t.Errorf("%s output contains a %s control sequence — render must be linear/append-only, never screen-clearing or alt-screen:\n%q", mode, ctrl.name, out)
		}
	}
}

// promptDriver scripts a single Prompt call against a real presenter of one mode
// and returns the choice, the cumulative out buffer, and any error. The two
// constructors are the injected-reader test seams (NewPlainPresenterWithInput /
// NewPrettyPresenterWithInput) so Prompt is driven without a real terminal; the
// pretty driver pins the colour profile so the screen-control assertions are
// deterministic regardless of the test runner's own TTY.
type promptDriver struct {
	mode string
	run  func(input string, gate presenter.Gate) (presenter.Choice, *bytes.Buffer, error)
}

// promptDrivers returns one driver per render mode so the render-only contract
// is asserted identically against BOTH real presenters. The pretty driver is run
// under a colour-CAPABLE profile (TrueColor) deliberately: the screen-control
// guard must pass even while lipgloss IS emitting SGR colour escapes, proving the
// guard rejects only clear/alt-screen/home sequences and not all ESC bytes.
func promptDrivers() []promptDriver {
	return []promptDriver{
		{
			mode: "plain",
			run: func(input string, gate presenter.Gate) (presenter.Choice, *bytes.Buffer, error) {
				out := &bytes.Buffer{}
				p := presenter.NewPlainPresenterWithInput(out, &bytes.Buffer{}, strings.NewReader(input))
				choice, err := p.Prompt(gate)
				return choice, out, err
			},
		},
		{
			mode: "pretty",
			run: func(input string, gate presenter.Gate) (presenter.Choice, *bytes.Buffer, error) {
				out := &bytes.Buffer{}
				p := presenter.NewPrettyPresenterWithInput(out, termenv.TrueColor, strings.NewReader(input))
				choice, err := p.Prompt(gate)
				return choice, out, err
			},
		},
	}
}

// TestPromptEditHasNoPresenterSideEffect locks the e (edit) render-only contract:
// scripted "e\n" returns ChoiceEdit and the presenter does nothing else — it
// invokes no $EDITOR and spawns no subprocess (the import guard proves the
// mechanism cannot exist; this proves the choice is returned cleanly). Asserted
// in BOTH modes against a real presenter.
func TestPromptEditHasNoPresenterSideEffect(t *testing.T) {
	for _, d := range promptDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			choice, _, err := d.run("e\n", presenter.NotesReviewGate())
			if err != nil {
				t.Fatalf("%s Prompt(e) returned error: %v", d.mode, err)
			}
			if choice != presenter.ChoiceEdit {
				t.Errorf("%s Prompt(e) = %q, want %q (render-only: return the choice, the engine owns the edit)", d.mode, choice, presenter.ChoiceEdit)
			}
		})
	}
}

// TestPromptRegenHasNoPresenterSideEffect locks the r (regenerate) render-only
// contract: scripted "r\n" returns ChoiceRegen and the presenter does nothing
// else — it invokes no claude/regeneration and spawns no subprocess. Asserted in
// BOTH modes against a real presenter.
func TestPromptRegenHasNoPresenterSideEffect(t *testing.T) {
	for _, d := range promptDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			choice, _, err := d.run("r\n", presenter.NotesReviewGate())
			if err != nil {
				t.Fatalf("%s Prompt(r) returned error: %v", d.mode, err)
			}
			if choice != presenter.ChoiceRegen {
				t.Errorf("%s Prompt(r) = %q, want %q (render-only: return the choice, the engine owns the regeneration)", d.mode, choice, presenter.ChoiceRegen)
			}
		})
	}
}

// simulateEngineLoop drives the engine's re-entry loop against a REAL presenter
// of one mode: each pass calls ShowNotes(refreshed body) then Prompt(gate); on a
// returned e or r it "refreshes" the body (changes the per-pass text, as the
// engine would after an edit/regenerate) and loops; it STOPS on y/n. It returns
// the final choice, the per-pass body texts it rendered, and the cumulative out
// buffer — everything the linear-render assertions need. The presenter is
// constructed ONCE so its narration accumulates across passes, exactly as a real
// run scrolls.
func simulateEngineLoop(t *testing.T, mode, input string, gate presenter.Gate) (presenter.Choice, []string, string) {
	t.Helper()

	out := &bytes.Buffer{}
	var p presenter.Presenter
	switch mode {
	case "plain":
		p = presenter.NewPlainPresenterWithInput(out, &bytes.Buffer{}, strings.NewReader(input))
	case "pretty":
		p = presenter.NewPrettyPresenterWithInput(out, termenv.TrueColor, strings.NewReader(input))
	default:
		t.Fatalf("unknown mode %q", mode)
	}

	var bodies []string
	var choice presenter.Choice
	const maxPasses = 16 // guard against a non-terminating loop in a broken impl
	for pass := 0; pass < maxPasses; pass++ {
		// The engine refreshes the body each pass (e.g. after an edit/regenerate);
		// a per-pass marker lets the assertions count appearances in order.
		body := bodyForPass(pass)
		bodies = append(bodies, body)
		p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: body})

		var err error
		choice, err = p.Prompt(gate)
		if err != nil {
			t.Fatalf("%s Prompt on pass %d returned error: %v", mode, pass, err)
		}
		if choice == presenter.ChoiceEdit || choice == presenter.ChoiceRegen {
			continue // engine owns the e/r work; loop again with a refreshed body
		}
		// y or n exits the loop.
		return choice, bodies, out.String()
	}
	t.Fatalf("%s engine loop did not terminate within %d passes — y/n must exit", mode, maxPasses)
	return choice, bodies, out.String()
}

// bodyForPass returns the distinct, per-pass release-notes body the simulated
// engine "refreshes" to on each re-entry, so the linear-render assertions can
// count each pass's body appearing once, in order, in the cumulative output.
func bodyForPass(pass int) string {
	switch pass {
	case 0:
		return "PASS-0 original notes body"
	case 1:
		return "PASS-1 edited notes body"
	case 2:
		return "PASS-2 regenerated notes body"
	default:
		return "PASS-" + strings.Repeat("x", pass) + " refreshed notes body"
	}
}

// TestEngineLoopRendersLinearlyAcrossPasses is the core render-only acceptance:
// the simulated engine loop (e, then r, then y — THREE passes ending on y)
// re-renders linearly. Each pass's refreshed body appears EXACTLY ONCE, in pass
// order, in the cumulative output; the gate/prompt appears once per pass; and the
// output carries NO clear-screen/alt-screen/cursor-home sequence (the render
// scrolls, it never overwrites). Asserted in BOTH modes against a real presenter.
func TestEngineLoopRendersLinearlyAcrossPasses(t *testing.T) {
	for _, d := range promptDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			gate := presenter.NotesReviewGate()
			choice, bodies, out := simulateEngineLoop(t, d.mode, "e\nr\ny\n", gate)

			// Three passes: e (pass 0) -> r (pass 1) -> y (pass 2, exits).
			if len(bodies) != 3 {
				t.Fatalf("%s loop ran %d passes, want 3 (e, r, y):\nbodies=%v", d.mode, len(bodies), bodies)
			}
			if choice != presenter.ChoiceYes {
				t.Errorf("%s loop final choice = %q, want %q (y exits the loop)", d.mode, choice, presenter.ChoiceYes)
			}

			// Each per-pass body appears exactly once, and in pass order — the proof
			// the render is append-only/cumulative, not an in-place overwrite that
			// would leave only the last body.
			lastIdx := -1
			for pass, body := range bodies {
				if n := strings.Count(out, body); n != 1 {
					t.Errorf("%s pass %d body %q appears %d times in cumulative output, want exactly 1 (append-only):\n%q", d.mode, pass, body, n, out)
				}
				idx := strings.Index(out, body)
				if idx <= lastIdx {
					t.Errorf("%s pass %d body %q is out of order (index %d, previous %d) — passes must scroll in order:\n%q", d.mode, pass, body, idx, lastIdx, out)
				}
				lastIdx = idx
			}

			// The gate/prompt is re-rendered once per pass (3 passes -> 3 prompts).
			if n := strings.Count(out, gate.Question); n != len(bodies) {
				t.Errorf("%s gate question %q appears %d times, want %d (once per pass):\n%q", d.mode, gate.Question, n, len(bodies), out)
			}

			// No screen-clearing / alt-screen / cursor-home — even though pretty is
			// emitting SGR colour escapes under TrueColor.
			assertNoScreenControl(t, d.mode, out)
		})
	}
}

// TestEngineLoopEndsOnYes proves y exits the loop: scripted "e\ny\n" loops ONCE
// on e (engine refreshes the body) then exits on y — two passes, final choice y.
// Asserted in BOTH modes.
func TestEngineLoopEndsOnYes(t *testing.T) {
	for _, d := range promptDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			choice, bodies, out := simulateEngineLoop(t, d.mode, "e\ny\n", presenter.NotesReviewGate())

			if choice != presenter.ChoiceYes {
				t.Errorf("%s loop final choice = %q, want %q", d.mode, choice, presenter.ChoiceYes)
			}
			if len(bodies) != 2 {
				t.Errorf("%s loop ran %d passes, want 2 (e then y):\nbodies=%v", d.mode, len(bodies), bodies)
			}
			assertNoScreenControl(t, d.mode, out)
		})
	}
}

// TestEngineLoopEndsOnNo proves n exits the loop immediately: scripted "n\n"
// exits on the first pass with ChoiceNo — one pass, no re-entry. Asserted in BOTH
// modes.
func TestEngineLoopEndsOnNo(t *testing.T) {
	for _, d := range promptDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			choice, bodies, out := simulateEngineLoop(t, d.mode, "n\n", presenter.NotesReviewGate())

			if choice != presenter.ChoiceNo {
				t.Errorf("%s loop final choice = %q, want %q", d.mode, choice, presenter.ChoiceNo)
			}
			if len(bodies) != 1 {
				t.Errorf("%s loop ran %d passes, want 1 (n exits immediately):\nbodies=%v", d.mode, len(bodies), bodies)
			}
			assertNoScreenControl(t, d.mode, out)
		})
	}
}
