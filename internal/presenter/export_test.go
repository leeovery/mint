package presenter

import "io"

// The chainable With* setters below are TEST-ONLY shims for the gating, spinner,
// and width fields. Production threads these signals in exactly one place — the
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

// WithYes sets the -y/--yes gating decision on the pretty presenter and returns it
// for chaining. See PrettyPresenter.yes.
func (p *PrettyPresenter) WithYes(yes bool) *PrettyPresenter {
	p.yes = yes
	return p
}

// WithInteractiveStdin sets the stdin-interactive gating signal on the pretty
// presenter and returns it for chaining; pass false to arm the
// forbidden-combination fail path. See PrettyPresenter.stdinInteractive.
func (p *PrettyPresenter) WithInteractiveStdin(interactive bool) *PrettyPresenter {
	p.stdinInteractive = interactive
	return p
}

// WithSpinnerFactory overrides the stage-progress spinner factory and returns the
// presenter for chaining — the spy seam that keeps the spinner lifecycle
// deterministic in tests (no timed goroutine, no frame output). Production never
// overrides the factory; the constructor defaults it to the real briandowns
// wrapper. See PrettyPresenter.newSpinner.
func (p *PrettyPresenter) WithSpinnerFactory(factory func(out io.Writer, text string) StageSpinner) *PrettyPresenter {
	p.newSpinner = factory
	return p
}

// WithTermWidth sets the detected terminal width feeding the decorative notes
// rules and returns the presenter for chaining — the width-axis seam for
// narrow/wide/tiny/undetectable cases. Production sets the field in NewForStartup
// from detectTermWidth(stdout). See PrettyPresenter.termWidth.
func (p *PrettyPresenter) WithTermWidth(w int) *PrettyPresenter {
	p.termWidth = w
	return p
}
