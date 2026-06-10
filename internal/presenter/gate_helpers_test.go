package presenter_test

import (
	"bytes"
	"io"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// gateOpts carries the gate-arming toggles for the plainGate/prettyGate
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

// applyTo threads the armed toggles onto an already-constructed presenter via the
// builder-style setters. Both setters return the presenter, but the helpers below
// keep their own concrete pointer, so the chained return value is unused here.
func (o gateOpts) applyToPlain(p *presenter.PlainPresenter) {
	if o.yes {
		p.WithYes(true)
	}
	if o.nonInteractiveStdin {
		p.WithInteractiveStdin(false)
	}
}

func (o gateOpts) applyToPretty(p *presenter.PrettyPresenter) {
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

// prettyGate mirrors plainGate for the pretty presenter. It forces the supplied
// colour profile so colour-on/colour-off assertions stay deterministic regardless
// of the test runner's own TTY, wires the err buffer (so the stream-split contract
// is assertable), reads from in, applies the armed toggles, and returns the
// presenter plus both buffers. Call sites that only assert on out ignore errBuf
// with _.
func prettyGate(profile termenv.Profile, in io.Reader, opts gateOpts) (p *presenter.PrettyPresenter, out, errBuf *bytes.Buffer) {
	out = &bytes.Buffer{}
	errBuf = &bytes.Buffer{}
	p = presenter.NewPrettyPresenter(
		out,
		presenter.WithProfile(profile),
		presenter.WithErr(errBuf),
		presenter.WithInput(in),
	)
	opts.applyToPretty(p)
	return p, out, errBuf
}
