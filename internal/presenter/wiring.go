package presenter

import (
	"io"
	"os"
)

// New is the raw, lower-level wiring seam: given an ALREADY-selected Mode and the
// two writers, it returns the matching Presenter implementation — ModePretty
// yields the Rail, ModePlain (the zero value, so any unrecognised Mode safely lands
// here too) yields the terse one — with both writers wired through. Returning the
// interface type keeps every caller depending on the seam rather than a concrete
// presenter. It takes the writers and Mode directly (no TTY probing, no gating
// axes), so a test can drive either mode deterministically; the production
// construction site that resolves the signals and threads the gating axes is
// NewForStartup, not this.
//
// The stream contract is fixed and identical across modes: out receives all
// narration; err receives the one-line failure summary (per StageFailed) and
// nothing on a successful run.
func New(mode Mode, out, err io.Writer) Presenter {
	if mode == ModePretty {
		return NewRailPresenter(out, WithRailErr(err))
	}
	return NewPlainPresenter(out, err)
}

// NewForStartup is the CONVERGED startup seam: the ONE production construction
// site that resolves BOTH orthogonal axes via DetectStartupSignals and threads all
// three signals — render Mode, the -y gating decision, and the stdin-interactive
// gating signal — onto the returned presenter. It wires stdout for narration and
// stderr for the failure summary (the production defaults of out = os.Stdout, err =
// os.Stderr), with stdin held only to resolve the gating axis. This is the single
// seam where mode selection, the stream split, and BOTH gating axes meet, so
// production never drifts into a parallel path that leaves a gating field at its
// interactive default.
//
// The render Mode is selected from stdout's TTY signal (honouring --plain via
// plainFlag); the stdin-interactive signal is detected INDEPENDENTLY from the
// stdin descriptor (DetectStartupSignals, never re-derived from Mode), so the
// forbidden-combination fail-loud path — non-TTY stdin without -y — is reachable
// through this seam. -y is a PARAMETER the caller supplies (the flag is parsed
// elsewhere); the seam only threads it.
//
// All three handles are taken as *os.File (not io.Writer): stdout and stdin so
// DetectStartupSignals can probe each for a TTY, stderr to mirror them. *os.File
// satisfies io.Writer, so stdout/stderr double as the narration/summary writers.
// Accepting the handles as parameters (rather than reaching for the os globals
// internally) keeps this unit-testable: a test passes non-TTY handles to drive the
// plain path and the forbidden-combination path deterministically.
//
// The stdin handle serves BOTH input roles: it is probed for the
// stdin-interactive gating signal AND wired through as the gate/line-read input
// stream, so the detected signal and the stream actually read can never diverge —
// whatever handle was judged interactive is the handle Prompt/AskLine read.
//
// Pretty output is the RailPresenter's, which reads the terminal's size from stdout
// at each redraw; the caller ends its drawing with Close once the run is over.
func NewForStartup(plainFlag, yes bool, stdout, stderr, stdin *os.File) Presenter {
	return newForSignals(DetectStartupSignals(plainFlag, stdout, stdin), yes, stdout, errWriterFor(stdout, stderr), stdin)
}

// newForSignals is NewForStartup's presenter for the signals it resolved, with the
// failure summaries going to errStream. The Rail on a TERM=dumb terminal, which can't
// move its cursor, draws no live region.
func newForSignals(signals StartupSignals, yes bool, stdout *os.File, errStream io.Writer, stdin *os.File) Presenter {
	if signals.Mode == ModePretty {
		return NewRailPresenter(stdout,
			WithRailErr(errStream),
			WithRailInput(stdin),
			WithRailGating(yes, signals.StdinInteractive),
			WithRailDumb(os.Getenv("TERM") == "dumb"),
		)
	}
	p := NewPlainPresenterWithInput(stdout, errStream, stdin)
	p.yes = yes
	p.stdinInteractive = signals.StdinInteractive
	return p
}

// Close ends p's drawing where it has one to end, the Rail's live region; any other
// presenter has nothing to end.
func Close(p Presenter) {
	if c, ok := p.(interface{ Close() }); ok {
		c.Close()
	}
}

// errWriterFor resolves the failure-summary stream. The err stream exists SOLELY
// for redirect-visibility — keeping failures on screen when the narration stream
// is piped away — so when stderr and stdout point at the SAME file (the everyday
// both-on-one-terminal case, or a `2>&1` merge) the mirror would land right next
// to the narration copy and every failure would print twice. In that case the
// summary stream is discarded; with genuinely split streams (stdout piped, stderr
// on the terminal — or vice versa) the mirror is kept. A Stat failure keeps the
// mirror: a failure shown twice beats a failure lost.
func errWriterFor(stdout, stderr *os.File) io.Writer {
	outInfo, outErr := stdout.Stat()
	errInfo, errErr := stderr.Stat()
	if outErr == nil && errErr == nil && os.SameFile(outInfo, errInfo) {
		return io.Discard
	}
	return stderr
}
