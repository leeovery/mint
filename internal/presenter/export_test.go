package presenter

import (
	"io"
	"os"
)

// The chainable With* setters below are TEST-ONLY shims for the gating and echo
// fields. Production threads these signals in exactly one place — the
// converged startup seam NewForStartup, which sets the fields directly — so the
// production API carries a single construction idiom (constructors + functional
// Options). Tests, which drive each axis independently, chain these onto a
// constructor (e.g. NewPlainPresenterWithInput(...).WithYes(true)). Defined here
// (compiled only under `go test`) they add no production surface.

// WithYes sets the -y/--yes gating decision on the plain presenter and returns it
// for chaining. See PlainPresenter.yes.
func (p *PlainPresenter) WithYes(yes bool) *PlainPresenter {
	p.yes = yes
	return p
}

// WithInteractiveStdin sets the stdin-interactive gating signal on the plain
// presenter and returns it for chaining; pass false to arm the
// forbidden-combination fail path. See PlainPresenter.stdinInteractive.
func (p *PlainPresenter) WithInteractiveStdin(interactive bool) *PlainPresenter {
	p.stdinInteractive = interactive
	return p
}

// SingleKeyGate exposes singleKeyGate, the test for whether a gate on the Rail can
// be answered with one keypress, to the external gate tests.
func SingleKeyGate(g Gate) bool { return singleKeyGate(g) }

// KeyChoice exposes keyChoice, what one read of keys answers a gate with, to the
// keypress tests.
func KeyChoice(keys []byte, g Gate) (Choice, bool, error) { return keyChoice(keys, g) }

// WithYes sets the -y gating decision on the rail presenter and returns it for
// chaining. See RailPresenter.yes.
func (p *RailPresenter) WithYes(yes bool) *RailPresenter {
	p.yes = yes
	return p
}

// WithInteractiveStdin sets the stdin-interactive gating signal on the rail
// presenter and returns it for chaining; false arms the forbidden combination.
func (p *RailPresenter) WithInteractiveStdin(interactive bool) *RailPresenter {
	p.stdinInteractive = interactive
	return p
}

// WithEcho sets whether the rail presenter's input echoes a typed line on the
// terminal, as a terminal's does, and returns it for chaining: the scripted readers
// tests answer with echo nothing unless a test says so, which may switch it between
// questions. See RailPresenter.echo.
func (p *RailPresenter) WithEcho(echo bool) *RailPresenter {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.echo = echo
	return p
}

// NewForSignals is NewForStartup's presenter for signals already resolved, for tests
// that can't give it a terminal.
func NewForSignals(signals StartupSignals, yes bool, stdout *os.File, errStream io.Writer, stdin *os.File) Presenter {
	return newForSignals(signals, yes, stdout, errStream, stdin)
}

// LiveRegion is the live region writer, for its tests.
type LiveRegion = liveRegion

// NewLiveRegion is a live region writing to out.
func NewLiveRegion(out io.Writer) *LiveRegion { return newLiveRegion(out) }
