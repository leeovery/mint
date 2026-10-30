package presenter_test

import (
	"bytes"
	"io"
	"strings"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// gateOpts carries the gate-arming toggles for the plainGate/railGate
// construction helpers. The zero value is the constructor default — no -y and an
// interactive stdin — so a call site that armed neither toggle passes gateOpts{}.
// The fields are named so each call site states its armed state explicitly (no
// copy-paste drift), and they are expressed as the ARMED deviation from the
// default so the zero value needs no field at all:
//   - yes:                 mirrors .WithYes(true)
//   - nonInteractiveStdin: mirrors .WithInteractiveStdin(false), which arms the
//     forbidden-combination fail path
type gateOpts struct {
	yes                 bool
	nonInteractiveStdin bool
}

// applyToPlain and applyToRail thread the armed toggles onto an already-constructed
// presenter via the builder-style setters. Both setters return the presenter, but the helpers
// below keep their own concrete pointer, so the chained return value is unused here.
func (o gateOpts) applyToPlain(p *presenter.PlainPresenter) {
	if o.yes {
		p.WithYes(true)
	}
	if o.nonInteractiveStdin {
		p.WithInteractiveStdin(false)
	}
}

func (o gateOpts) applyToRail(p *presenter.RailPresenter) {
	if o.yes {
		p.WithYes(true)
	}
	if o.nonInteractiveStdin {
		p.WithInteractiveStdin(false)
	}
}

// plainGate is the shared construction+capture seam for the plain gate tests. It
// allocates the out/err capture buffers, builds a PlainPresenter reading from in
// (a strings.Reader script or a failingReader), applies the armed toggles, and
// returns the presenter plus both buffers. Centralising this collapses the
// hand-inlined buffer-allocation + presenter-construction idiom to one site, so a
// construction-API change is a single edit and the arming toggles can never drift.
func plainGate(in io.Reader, opts gateOpts) (p *presenter.PlainPresenter, out, errBuf *bytes.Buffer) {
	out = &bytes.Buffer{}
	errBuf = &bytes.Buffer{}
	p = presenter.NewPlainPresenterWithInput(out, errBuf, in)
	opts.applyToPlain(p)
	return p, out, errBuf
}

// railGate mirrors plainGate for the Rail, pretty mode's presenter, on a terminal
// 80 by 24. It forces the supplied colour profile so colour-on/colour-off assertions
// stay deterministic regardless of the test runner's own TTY, wires the err buffer
// (so the stream-split contract is assertable), reads from in, which is no terminal,
// so every answer is a typed line, applies the armed toggles, and returns the
// presenter plus both buffers.
func railGate(profile termenv.Profile, in io.Reader, opts gateOpts) (p *presenter.RailPresenter, out, errBuf *bytes.Buffer) {
	out = &bytes.Buffer{}
	errBuf = &bytes.Buffer{}
	p = presenter.NewRailPresenter(out,
		presenter.WithRailErr(errBuf),
		presenter.WithRailInput(in),
		presenter.WithRailProfile(profile),
		presenter.WithRailSize(func() (int, int) { return 80, 24 }),
	)
	opts.applyToRail(p)
	return p, out, errBuf
}

// gateResult is what a gateDriver returns from a single Prompt call: the choice,
// the out/err capture buffers, and the error. Mode-invariant gate/prompt
// properties (e.g. "returns ErrNotInteractive in both modes", "empty-Enter selects
// the default in both modes") read whichever fields they assert and ignore the
// rest.
type gateResult struct {
	choice presenter.Choice
	out    *bytes.Buffer
	errBuf *bytes.Buffer
	err    error
}

// gateDriver pairs a render-mode name with the one canonical one-Prompt driver,
// built on the plainGate/railGate construction seams. It lets a GENUINELY
// mode-invariant gate/prompt property be asserted once inside t.Run(d.mode, ...)
// instead of as two hand-duplicated plain-then-rail arms. Mode-SPECIFIC rendering
// (the exact plain "FAILED -" line vs the Rail's ✗ node, the plain [y/n/e/r] hint
// vs the Rail's keys, the distinct -y echo bytes) stays in its own dedicated test
// and is NOT driven through this table. The colour profile is an explicit
// parameter (the plain driver ignores it — plain mode has no colour) so each call
// site states the profile it needs: Ascii for deterministic captured text,
// TrueColor where the property must hold WHILE SGR colour escapes are present (the
// render-only screen-control guard).
type gateDriver struct {
	mode string
	run  func(profile termenv.Profile, in io.Reader, opts gateOpts, gate presenter.Gate) gateResult
}

// prompt is the prompt-test-shaped convenience over run: it scripts a single
// Prompt from a string with the default gate options (no -y, interactive stdin) —
// the shape the prompt tests need — selecting the Rail's colour profile (ignored by
// the plain driver).
func (d gateDriver) prompt(profile termenv.Profile, input string, gate presenter.Gate) gateResult {
	return d.run(profile, strings.NewReader(input), gateOpts{}, gate)
}

// renderCount counts how many times the gate's question was written to out, once
// per pass in both modes: plain prints its question line each pass, and the Rail
// prints the question its typed line is read at, again under each answer that names
// no key. The re-prompt/loop tests count renders through this seam so they assert
// the render-once-per-pass property without hardcoding mode-specific layout at every
// call site.
func (d gateDriver) renderCount(out string, gate presenter.Gate) int {
	return strings.Count(out, gate.Question)
}

// gateDrivers returns one driver per render mode for the mode-invariant
// gate/prompt properties.
func gateDrivers() []gateDriver {
	return []gateDriver{
		{
			mode: "plain",
			run: func(_ termenv.Profile, in io.Reader, opts gateOpts, gate presenter.Gate) gateResult {
				p, out, errBuf := plainGate(in, opts)
				choice, err := p.Prompt(gate)
				return gateResult{choice: choice, out: out, errBuf: errBuf, err: err}
			},
		},
		{
			mode: "rail",
			run: func(profile termenv.Profile, in io.Reader, opts gateOpts, gate presenter.Gate) gateResult {
				p, out, errBuf := railGate(profile, in, opts)
				defer p.Close()
				choice, err := p.Prompt(gate)
				return gateResult{choice: choice, out: out, errBuf: errBuf, err: err}
			},
		},
	}
}
