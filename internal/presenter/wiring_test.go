package presenter_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
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
// err; success leaves err empty) must hold identically for plain and the Rail.
var bothModes = []modeCase{
	{name: "plain", mode: presenter.ModePlain},
	{name: "pretty", mode: presenter.ModePretty},
}

// newSplit constructs a Presenter for the given mode via the raw wiring seam
// (presenter.New), capturing the out and err streams into separate buffers so a
// test can assert exactly which stream each line landed on. The presenter is
// closed when the test ends, so a Rail's spinner stops with it.
func newSplit(t *testing.T, mode presenter.Mode) (p presenter.Presenter, out, errBuf *bytes.Buffer) {
	t.Helper()
	out = &bytes.Buffer{}
	errBuf = &bytes.Buffer{}
	p = presenter.New(mode, out, errBuf)
	t.Cleanup(func() { presenter.Close(p) })
	return p, out, errBuf
}

// TestNewSelectsImplementationMatchingMode asserts the constructor returns the
// implementation matching the requested mode and wires both writers: ModePlain
// yields the plain rendering, ModePretty the Rail, each writing to the provided
// out. The mode is identified behaviourly (plain's terse line; the Rail's header
// wordmark) rather than by reflecting on the concrete type, keeping the assertion
// on observable rendering.
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
			notContain:  "─",
		},
		{
			name:        "pretty mode draws the rail's header",
			mode:        presenter.ModePretty,
			wantContain: " mint ─ release",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, out, _ := newSplit(t, tt.mode)
			p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.4.0", Action: "releasing", Header: presenter.Header{Command: "release"}})

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

			p, out, errBuf := newSplit(t, mc.mode)
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

			p, out, errBuf := newSplit(t, mc.mode)
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

			p, _, errBuf := newSplit(t, mc.mode)
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

			p, out, errBuf := newSplit(t, tt.mode)
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

// TestWarnAppearsOnBothStdoutAndStderr drives Warn and asserts the warning is
// present in BOTH the stdout narration AND the stderr buffer for BOTH modes — per
// the stream contract, warnings appear in the narration and additionally on stderr
// for redirect-visibility. Each mode uses its own warn text form.
func TestWarnAppearsOnBothStdoutAndStderr(t *testing.T) {
	tests := []struct {
		name    string
		mode    presenter.Mode
		wantOut string
		wantErr string
	}{
		{
			name:    "plain warn line",
			mode:    presenter.ModePlain,
			wantOut: "post_release: WARN - hook failed: scripts/notify.sh exited 1",
			wantErr: "post_release: WARN - hook failed: scripts/notify.sh exited 1",
		},
		{
			name:    "pretty warn node",
			mode:    presenter.ModePretty,
			wantOut: "hook failed: scripts/notify.sh exited 1",
			wantErr: "⚠ post_release  hook failed: scripts/notify.sh exited 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, out, errBuf := newSplit(t, tt.mode)
			p.Warn(presenter.Warning{Label: "post_release", Message: "hook failed: scripts/notify.sh exited 1"})

			if !strings.Contains(out.String(), tt.wantOut) {
				t.Errorf("%s: warn missing from stdout = %q, want it to contain %q", tt.name, out.String(), tt.wantOut)
			}
			if !strings.Contains(errBuf.String(), tt.wantErr) {
				t.Errorf("%s: warn missing from stderr = %q, want it to contain %q", tt.name, errBuf.String(), tt.wantErr)
			}
		})
	}
}

// TestUnwoundAppearsOnStdoutOnly drives Unwound and asserts the auto-unwind line
// reaches the stdout narration but is ABSENT from stderr, for BOTH modes — unlike
// FAILED/WARN, the per-event table lists no stderr copy for the auto-unwind line.
func TestUnwoundAppearsOnStdoutOnly(t *testing.T) {
	tests := []struct {
		name    string
		mode    presenter.Mode
		wantOut string
	}{
		{name: "plain", mode: presenter.ModePlain, wantOut: "unwound: removed tag v1.4.0, reset 2 commits; repo clean"},
		{name: "pretty", mode: presenter.ModePretty, wantOut: "removed tag v1.4.0, reset 2 commits"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, out, errBuf := newSplit(t, tt.mode)
			p.Unwound(presenter.Unwind{Undone: "removed tag v1.4.0, reset 2 commits; repo clean"})

			if !strings.Contains(out.String(), tt.wantOut) {
				t.Errorf("%s: unwound missing from stdout = %q, want it to contain %q", tt.name, out.String(), tt.wantOut)
			}
			if errBuf.Len() != 0 {
				t.Errorf("%s: Unwound wrote to stderr = %q, want empty", tt.name, errBuf.String())
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

			p, _, errBuf := newSplit(t, mc.mode)
			p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected: remote moved"})

			if got := strings.Count(errBuf.String(), "\n"); got != 1 {
				t.Errorf("%s: stderr summary should be exactly one line, got %d newlines:\n%q", mc.name, got, errBuf.String())
			}
		})
	}
}

// TestFailureStderrSummaryIsUnstyled asserts the stderr summary carries no ANSI
// escape (ESC 0x1b) for BOTH modes — stderr is a redirect-visibility channel,
// not a styled surface, so even the Rail's stderr summary is plain text. The Rail
// is built with a colour-capable profile so the test proves the *absence* of
// styling on err is a deliberate choice, not an artefact of colour downgrading on
// a non-TTY buffer.
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

	t.Run("the rail with forced colour profile", func(t *testing.T) {
		t.Parallel()

		out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
		p := presenter.NewRailPresenter(out, presenter.WithRailErr(errBuf), presenter.WithRailProfile(termenv.TrueColor))
		defer p.Close()
		p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected"})

		// out carries the styled ✗ node (colour forced on), so it has SGR colour.
		if !strings.Contains(out.String(), "m✗") {
			t.Errorf("rail: stdout failure node should be styled under a colour profile:\n%q", out.String())
		}
		// err carries the unstyled one-line summary, so it must have no ANSI.
		if bytes.ContainsRune(errBuf.Bytes(), 0x1b) {
			t.Errorf("rail: stderr summary must be unstyled but contains an ESC (0x1b):\n%q", errBuf.String())
		}
		if !strings.Contains(errBuf.String(), "tag/push") || !strings.Contains(errBuf.String(), "push rejected") {
			t.Errorf("rail: stderr summary missing stage/message:\n%q", errBuf.String())
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

			p, _, errBuf := newSplit(t, mc.mode)
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

// openDevNull opens a fresh /dev/null *os.File for use as a CI-safe non-TTY
// handle (stdout selects plain; stdin arms the forbidden-combination path), and
// registers its close. Each caller gets its own handle so a closed handle in one
// subtest never affects another.
func openDevNull(t *testing.T) *os.File {
	t.Helper()
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("opening %s: %v", os.DevNull, err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// TestNewForStartupWiresStdoutStderrAndMode proves the converged startup seam
// wires the real os.Stdout/os.Stderr/os.Stdin handles and selects the mode from
// the stdout TTY signal. Feeding a non-TTY stdout (/dev/null) deterministically
// selects plain regardless of the host's own terminal, so the assertion stays
// CI-safe. The returned value is a usable Presenter writing to the supplied
// handles.
func TestNewForStartupWiresStdoutStderrAndMode(t *testing.T) {
	devNull := openDevNull(t)

	p := presenter.NewForStartup(false, false, devNull, os.Stderr, os.Stdin)
	if p == nil {
		t.Fatal("NewForStartup returned nil")
	}

	// A non-TTY stdout selects plain; driving RunStarted writes the terse plain
	// line to the provided stdout handle (here /dev/null, discarded), proving the
	// value is a usable Presenter wired to the handles without panicking.
	p.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.4.0", Action: "releasing"})
}

// TestNewForStartupThreadsNonInteractiveStdinToForbiddenCombo proves the
// converged seam CONSUMES DetectStartupSignals and threads the stdin-interactive
// signal (NOT the render Mode) into the gating field: built with a non-TTY stdin
// handle and -y unset, the returned presenter's Prompt reaches the
// forbidden-combination fail-loud path — returning ErrNotInteractive and writing
// the one-line failure summary to the err stream. This only fires when
// signals.StdinInteractive=false was threaded into the gating field; if the seam
// left the gating axis at its interactive default the Prompt would block on stdin
// instead of failing loud. (A /dev/null stdout selects ModePlain, which is fine —
// the fail-loud path is mode-independent.)
func TestNewForStartupThreadsNonInteractiveStdinToForbiddenCombo(t *testing.T) {
	stdout := openDevNull(t)
	stdin := openDevNull(t)
	errBuf := &bytes.Buffer{}

	// stderr is taken as *os.File for the TTY-probe-shaped signature, but the err
	// summary is what we assert; route it through a pipe so the buffer captures it.
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(func() { _ = errR.Close(); _ = errW.Close() })

	p := presenter.NewForStartup(false, false, stdout, errW, stdin)

	choice, promptErr := p.Prompt(presenter.NotesReviewGate())
	_ = errW.Close()

	if !errors.Is(promptErr, presenter.ErrNotInteractive) {
		t.Errorf("NewForStartup (non-TTY stdin, -y unset) Prompt err = %v, want errors.Is(..., ErrNotInteractive)", promptErr)
	}
	if choice != "" {
		t.Errorf("NewForStartup forbidden-combo Prompt returned choice %q, want zero choice", choice)
	}

	if _, copyErr := errBuf.ReadFrom(errR); copyErr != nil {
		t.Fatalf("reading err pipe: %v", copyErr)
	}
	if !strings.Contains(errBuf.String(), "FAILED") {
		t.Errorf("NewForStartup forbidden-combo err stream = %q, want the one-line FAILED summary", errBuf.String())
	}
}

// TestNewForStartupThreadsYesAutoConfirmsWithoutReadingStdin proves the seam
// threads the -y parameter onto the returned presenter: built with -y SET and a
// non-TTY stdin handle, Prompt AUTO-CONFIRMS — returning the gate's declared
// default and emitting the auto-accept echo — WITHOUT reaching the
// forbidden-combination path and without reading stdin. -y precedes the
// stdin-interactive check, so the non-TTY stdin is irrelevant here; the assertion
// proves the WithYes axis was threaded (otherwise the non-TTY stdin would fail
// loud instead of auto-confirming).
func TestNewForStartupThreadsYesAutoConfirmsWithoutReadingStdin(t *testing.T) {
	stdin := openDevNull(t)
	gate := presenter.NotesReviewGate()

	// Capture stdout through a pipe so the auto-accept echo is assertable; a
	// non-TTY pipe stdout selects ModePlain, whose echo is the byte-pure ASCII
	// "notes: accepted (-y)" line.
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(func() { _ = outR.Close(); _ = outW.Close() })

	p := presenter.NewForStartup(false, true, outW, os.Stderr, stdin)

	choice, promptErr := p.Prompt(gate)
	_ = outW.Close()

	if promptErr != nil {
		t.Fatalf("NewForStartup (-y set, non-TTY stdin) Prompt returned error: %v", promptErr)
	}
	if choice != gate.Default {
		t.Errorf("NewForStartup (-y set) Prompt = %q, want gate default %q (auto-confirm)", choice, gate.Default)
	}

	out := &bytes.Buffer{}
	if _, copyErr := out.ReadFrom(outR); copyErr != nil {
		t.Fatalf("reading out pipe: %v", copyErr)
	}
	if got := out.String(); !strings.Contains(got, "notes: accepted (-y)") {
		t.Errorf("NewForStartup (-y set) echo = %q, want it to contain the auto-accept echo", got)
	}
}

// TestNewForStartupSameFileStreamsSuppressFailureMirror proves the everyday
// both-streams-on-one-terminal case renders a failure ONCE: when stdout and
// stderr resolve to the SAME file (here, two handles on one temp file — the
// same condition as a shared TTY or a `2>&1` merge), the stderr summary mirror
// is discarded, so a StageFailed lands in the file exactly once.
func TestNewForStartupSameFileStreamsSuppressFailureMirror(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "merged")
	outF, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating merged stream: %v", err)
	}
	t.Cleanup(func() { _ = outF.Close() })
	errF, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("opening second handle: %v", err)
	}
	t.Cleanup(func() { _ = errF.Close() })

	p := presenter.NewForStartup(true, false, outF, errF, os.Stdin)
	p.StageFailed(presenter.StageFailure{Name: "preflight", Message: "nothing to commit, working tree clean"})

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading merged stream: %v", err)
	}
	if n := strings.Count(string(got), "nothing to commit"); n != 1 {
		t.Errorf("failure rendered %d times on merged streams, want exactly 1:\n%q", n, string(got))
	}
}

// TestNewForStartupSplitStreamsKeepFailureMirror proves the redirect case the
// mirror exists FOR still works: with stdout and stderr on DIFFERENT files
// (stdout piped away), the stderr summary is kept, so the failure stays
// visible on the un-redirected stream.
func TestNewForStartupSplitStreamsKeepFailureMirror(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outF, err := os.Create(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatalf("creating out: %v", err)
	}
	t.Cleanup(func() { _ = outF.Close() })
	errF, err := os.Create(filepath.Join(dir, "err"))
	if err != nil {
		t.Fatalf("creating err: %v", err)
	}
	t.Cleanup(func() { _ = errF.Close() })

	p := presenter.NewForStartup(true, false, outF, errF, os.Stdin)
	p.StageFailed(presenter.StageFailure{Name: "preflight", Message: "nothing to commit, working tree clean"})

	errGot, err := os.ReadFile(errF.Name())
	if err != nil {
		t.Fatalf("reading err stream: %v", err)
	}
	if !strings.Contains(string(errGot), "nothing to commit") {
		t.Errorf("split streams must keep the stderr mirror, err stream = %q", string(errGot))
	}
}
