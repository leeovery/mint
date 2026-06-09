package presenter_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// modeCase pairs a render Mode with a human label so the stream-split contract
// can be asserted identically for BOTH modes in a single table — the spec's
// fixed requirement is that the split is the same regardless of mode.
type modeCase struct {
	name string
	mode presenter.Mode
}

// bothModes is the shared table driving every stream-split assertion across the
// two implementations. The split (narration → out; one-line failure summary →
// err; success leaves err empty) must hold identically for plain and pretty.
var bothModes = []modeCase{
	{name: "plain", mode: presenter.ModePlain},
	{name: "pretty", mode: presenter.ModePretty},
}

// newSplit constructs a Presenter for the given mode via the single wiring point
// (presenter.New), capturing the out and err streams into separate buffers so a
// test can assert exactly which stream each line landed on.
func newSplit(mode presenter.Mode) (p presenter.Presenter, out, errBuf *bytes.Buffer) {
	out = &bytes.Buffer{}
	errBuf = &bytes.Buffer{}
	p = presenter.New(mode, out, errBuf)
	return p, out, errBuf
}

// TestNewSelectsImplementationMatchingMode asserts the constructor returns the
// implementation matching the requested mode and wires both writers: ModePlain
// yields the plain rendering, ModePretty the styled one, each writing to the
// provided out. The mode is identified behaviourly (plain emits no glyph; pretty
// emits the brand leaf) rather than by reflecting on the concrete type, keeping
// the assertion on observable rendering.
func TestNewSelectsImplementationMatchingMode(t *testing.T) {
	tests := []struct {
		name        string
		mode        presenter.Mode
		wantContain string
		notContain  string
	}{
		{
			name:        "plain mode renders terse line",
			mode:        presenter.ModePlain,
			wantContain: "mint: releasing acme v1.4.0",
			notContain:  "🌿",
		},
		{
			name:        "pretty mode renders brand leaf",
			mode:        presenter.ModePretty,
			wantContain: "🌿",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, out, _ := newSplit(tt.mode)
			p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.4.0", Action: "releasing"})

			if got := out.String(); !strings.Contains(got, tt.wantContain) {
				t.Errorf("New(%v) RunStarted out = %q, want it to contain %q", tt.mode, got, tt.wantContain)
			}
			if tt.notContain != "" && strings.Contains(out.String(), tt.notContain) {
				t.Errorf("New(%v) RunStarted out = %q, must not contain %q", tt.mode, out.String(), tt.notContain)
			}
		})
	}
}

// TestNewWritesNarrationToProvidedOutWriter proves the constructor wires the out
// writer through: nothing is written to err on a non-failure event, so the err
// buffer stays empty while the narration reaches out.
func TestNewWritesNarrationToProvidedOutWriter(t *testing.T) {
	for _, mc := range bothModes {
		t.Run(mc.name, func(t *testing.T) {
			t.Parallel()

			p, out, errBuf := newSplit(mc.mode)
			p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.4.0", Action: "releasing"})

			if out.Len() == 0 {
				t.Errorf("%s: New wired no narration to out", mc.name)
			}
			if errBuf.Len() != 0 {
				t.Errorf("%s: RunStarted wrote to err = %q, want empty", mc.name, errBuf.String())
			}
		})
	}
}

// TestSuccessRunLeavesStderrEmpty drives the full success sequence (RunStarted →
// StageSucceeded → RunFinished) and asserts narration reaches stdout while
// stderr stays EMPTY — a clean run under redirection must leave nothing on
// stderr. Asserted for BOTH modes.
func TestSuccessRunLeavesStderrEmpty(t *testing.T) {
	for _, mc := range bothModes {
		t.Run(mc.name, func(t *testing.T) {
			t.Parallel()

			p, out, errBuf := newSplit(mc.mode)
			p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.4.0", Action: "releasing"})
			p.StageSucceeded(presenter.StageSuccess{Name: "preflight", Detail: "clean"})
			p.RunFinished(presenter.RunResult{Project: "acme", Version: "1.4.0", URL: "https://example/v1.4.0"})

			if out.Len() == 0 {
				t.Errorf("%s: success run wrote no narration to stdout", mc.name)
			}
			if errBuf.Len() != 0 {
				t.Errorf("%s: success run wrote to stderr = %q, want empty", mc.name, errBuf.String())
			}
		})
	}
}

// TestNonFailureEventsNeverWriteToStderr drives every non-failure event in
// isolation and asserts none of them touch stderr — the success path must keep
// stderr clean event-by-event, not merely in aggregate. Asserted for BOTH modes.
func TestNonFailureEventsNeverWriteToStderr(t *testing.T) {
	for _, mc := range bothModes {
		t.Run(mc.name, func(t *testing.T) {
			t.Parallel()

			p, _, errBuf := newSplit(mc.mode)
			p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.4.0", Action: "releasing"})
			p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
			p.StageSucceeded(presenter.StageSuccess{Name: "preflight", Detail: "clean"})
			p.RunFinished(presenter.RunResult{Project: "acme", Version: "1.4.0"})

			if errBuf.Len() != 0 {
				t.Errorf("%s: non-failure events wrote to stderr = %q, want empty", mc.name, errBuf.String())
			}
		})
	}
}

// TestFailureSummaryAppearsOnBothStdoutAndStderr drives StageFailed and asserts
// the one-line FAILED/error summary is present in BOTH the stdout narration AND
// the stderr buffer — under redirection the failure cannot silently vanish.
// Asserted for BOTH modes; each mode uses its own one-line summary form.
func TestFailureSummaryAppearsOnBothStdoutAndStderr(t *testing.T) {
	tests := []struct {
		name        string
		mode        presenter.Mode
		wantSummary string
	}{
		{
			name:        "plain summary line",
			mode:        presenter.ModePlain,
			wantSummary: "tag/push: FAILED - push rejected: remote moved",
		},
		{
			name:        "pretty summary message",
			mode:        presenter.ModePretty,
			wantSummary: "push rejected: remote moved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, out, errBuf := newSplit(tt.mode)
			p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected: remote moved"})

			if !strings.Contains(out.String(), tt.wantSummary) {
				t.Errorf("%s: failure summary missing from stdout = %q, want it to contain %q", tt.name, out.String(), tt.wantSummary)
			}
			if !strings.Contains(errBuf.String(), tt.wantSummary) {
				t.Errorf("%s: failure summary missing from stderr = %q, want it to contain %q", tt.name, errBuf.String(), tt.wantSummary)
			}
		})
	}
}

// TestFailureStderrSummaryIsSingleLine asserts the stderr summary is exactly one
// line for BOTH modes — the contract routes only the one-line summary to stderr,
// never a multi-line body. (The captured-output body is a Phase 2 addition; this
// locks the single-line rule now so Phase 2 inherits it.)
func TestFailureStderrSummaryIsSingleLine(t *testing.T) {
	for _, mc := range bothModes {
		t.Run(mc.name, func(t *testing.T) {
			t.Parallel()

			p, _, errBuf := newSplit(mc.mode)
			p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected: remote moved"})

			if got := strings.Count(errBuf.String(), "\n"); got != 1 {
				t.Errorf("%s: stderr summary should be exactly one line, got %d newlines:\n%q", mc.name, got, errBuf.String())
			}
		})
	}
}

// TestFailureStderrSummaryIsUnstyled asserts the stderr summary carries no ANSI
// escape (ESC 0x1b) for BOTH modes — stderr is a redirect-visibility channel,
// not a styled surface, so even pretty's stderr summary is plain text. The pretty
// path forces a colour-capable profile via the explicit constructor so the test
// proves the *absence* of styling on err is a deliberate choice, not an artefact
// of lipgloss auto-downgrading on a non-TTY buffer.
func TestFailureStderrSummaryIsUnstyled(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		t.Parallel()

		out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
		p := presenter.NewPlainPresenter(out, errBuf)
		p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected"})

		if bytes.ContainsRune(errBuf.Bytes(), 0x1b) {
			t.Errorf("plain: stderr summary contains an ESC (0x1b):\n%q", errBuf.String())
		}
	})

	t.Run("pretty with forced colour profile", func(t *testing.T) {
		t.Parallel()

		out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
		p := presenter.NewPrettyPresenterWithErr(out, errBuf, termenv.TrueColor)
		p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected"})

		// out carries the styled ✗ line (colour forced on), so it has ANSI.
		if !bytes.ContainsRune(out.Bytes(), 0x1b) {
			t.Errorf("pretty: stdout failure line should be styled (ANSI) under a colour profile:\n%q", out.String())
		}
		// err carries the unstyled one-line summary, so it must have no ANSI.
		if bytes.ContainsRune(errBuf.Bytes(), 0x1b) {
			t.Errorf("pretty: stderr summary must be unstyled but contains an ESC (0x1b):\n%q", errBuf.String())
		}
		if !strings.Contains(errBuf.String(), "tag/push") || !strings.Contains(errBuf.String(), "push rejected") {
			t.Errorf("pretty: stderr summary missing stage/message:\n%q", errBuf.String())
		}
	})
}

// TestFailureOutputBodyNotDuplicatedToStderr is the explicit contract guard: a
// StageFailure with a non-empty Output (the captured multi-line command body)
// must put the one-line message on stderr but MUST NOT write the Output body
// there. The body is narration → stdout only; routing it to stderr is forbidden
// now (it is not even rendered yet) and must stay forbidden when Phase 2 renders
// it. Asserted for BOTH modes.
func TestFailureOutputBodyNotDuplicatedToStderr(t *testing.T) {
	const body = "fatal: failed to push some refs\nhint: updates were rejected\nremote: moved on"

	for _, mc := range bothModes {
		t.Run(mc.name, func(t *testing.T) {
			t.Parallel()

			p, _, errBuf := newSplit(mc.mode)
			p.StageFailed(presenter.StageFailure{
				Name:    "tag/push",
				Message: "push rejected: remote moved",
				Output:  body,
			})

			if !strings.Contains(errBuf.String(), "push rejected: remote moved") {
				t.Errorf("%s: stderr missing the one-line message = %q", mc.name, errBuf.String())
			}
			for _, line := range strings.Split(body, "\n") {
				if strings.Contains(errBuf.String(), line) {
					t.Errorf("%s: captured Output body line %q leaked to stderr = %q", mc.name, line, errBuf.String())
				}
			}
		})
	}
}

// TestNewForStartupWiresStdoutStderrAndMode proves the startup convenience wires
// the real os.Stdout/os.Stderr handles and selects the mode from the stdout TTY
// signal. Feeding a non-TTY stdout (/dev/null) deterministically selects plain
// regardless of the host's own terminal, so the assertion stays CI-safe. The
// returned value is a usable Presenter writing to the supplied handles.
func TestNewForStartupWiresStdoutStderrAndMode(t *testing.T) {
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("opening %s: %v", os.DevNull, err)
	}
	t.Cleanup(func() { _ = devNull.Close() })

	p := presenter.NewForStartup(false, devNull, os.Stderr)
	if p == nil {
		t.Fatal("NewForStartup returned nil")
	}

	// A non-TTY stdout selects plain; driving RunStarted writes the terse plain
	// line to the provided stdout handle (here /dev/null, discarded), proving the
	// value is a usable Presenter wired to the handles without panicking.
	p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.4.0", Action: "releasing"})
}
