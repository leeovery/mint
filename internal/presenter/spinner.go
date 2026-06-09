package presenter

import (
	"io"
	"time"

	"github.com/briandowns/spinner"
)

// StageSpinner is the small abstraction the pretty presenter depends on for the
// stage-progress animation. It is the seam that keeps the spinner LIFECYCLE
// deterministically testable: the presenter only ever calls Start()/Stop(), so a
// test can inject a fake that records those calls and tracks the active count,
// asserting "one spinner at a time" and "started-then-stopped" WITHOUT the real
// library's timed goroutine and frame output. It is exported solely so the
// external-package test can supply that spy via WithSpinnerFactory; production code
// only ever uses the default factory wired in the constructors.
//
// The real implementation (briandownsSpinner) wraps github.com/briandowns/spinner
// — a lightweight standalone spinner with explicit Start()/Stop(), NOT Bubble Tea:
// it owns no event loop and draws no alt-screen, so it composes with the linear,
// print-style Presenter seam. Stop() signals the animation goroutine to halt; the
// goroutine then observes the signal and exits, so a stopped spinner leaks no
// goroutine. Presenter usage is single-goroutine (one stage at a time), so the
// active handle is never touched concurrently.
type StageSpinner interface {
	// Start begins the animation (the real one spawns a goroutine writing timed
	// braille frames to the configured writer; a no-op if already running or if the
	// writer is not a real terminal). The presenter calls it once on a blocking
	// StageStarted.
	Start()
	// Stop ends the animation, clearing the spinner's line in place. The real
	// implementation signals its animation goroutine to exit (it does not block-join),
	// and in practice writes no further frame once stopped. The presenter calls it on
	// stage completion before printing the ✓/✗ line in the cleared place.
	Stop()
}

// spinnerFactory builds a StageSpinner that animates the given dim start text to
// out. It is a field on PrettyPresenter so production wires the real briandowns
// wrapper while tests inject a spy — the injection point for deterministic
// lifecycle assertions.
type spinnerFactory func(out io.Writer, text string) StageSpinner

// brailleCharSet is the braille spinner frame set (⠋⠙⠹…) the spec calls for. It is
// the library's CharSets[11].
var brailleCharSet = spinner.CharSets[11]

// spinnerFrameDelay is the per-frame delay of the real spinner. The exact cadence
// is cosmetic and never asserted (the tests drive the injected spy, not real
// timing); ~100ms matches the library's own default and reads as a calm spinner.
const spinnerFrameDelay = 100 * time.Millisecond

// briandownsSpinner is the production StageSpinner: a thin wrapper over the
// briandowns spinner so the presenter depends only on the Start()/Stop() seam, not
// the concrete library type.
type briandownsSpinner struct {
	s *spinner.Spinner
}

// newBriandownsSpinner is the production spinnerFactory. It builds a braille
// spinner writing to out (narration → stdout in production; NEVER stderr), with the
// dim start text as the suffix so a frame renders as "⠋ {text}" — the spec's
// "⠋ notes  generating with claude…" shape, the braille frame supplied by the
// library and the text the presenter's dim start text. The cursor is kept hidden
// while spinning and restored on Stop (the library's default). The library gates
// animation on whether os.Stdout is a terminal; in production pretty mode is only
// selected when out is the real (TTY) stdout, so frames render there and nowhere
// else (a non-TTY run is plain mode, which never constructs a spinner).
func newBriandownsSpinner(out io.Writer, text string) StageSpinner {
	s := spinner.New(
		brailleCharSet,
		spinnerFrameDelay,
		spinner.WithWriter(out),
		spinner.WithSuffix(" "+text),
	)
	return &briandownsSpinner{s: s}
}

// Start delegates to the library's Start (spawns the animation goroutine; a no-op
// if out is not a real terminal).
func (b *briandownsSpinner) Start() { b.s.Start() }

// Stop delegates to the library's Stop, which clears the spinner line in place and
// signals the animation goroutine to exit (it does not block-join); the goroutine
// then terminates, so no goroutine leaks and no frame is written after Stop.
func (b *briandownsSpinner) Stop() { b.s.Stop() }
