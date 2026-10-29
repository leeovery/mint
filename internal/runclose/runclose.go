// Package runclose holds one run between the RunStarted that opens it and the one
// RunFinished that closes it, so each orchestrator closes every way its run can end
// from a single place: the ending the run landed, or, when it stopped short, the
// start station it is back at.
package runclose

import (
	"time"

	"mint/internal/presenter"
)

// The details of the station a run that stopped short closes at. Only a release
// unwinds, and only before its push, so an unwound run has pushed nothing.
const (
	nothingChanged       = "nothing changed"
	backWhereItStarted   = "back where it started · repo clean · nothing pushed"
	manualCleanupPending = "manual cleanup required · nothing pushed"
)

// Run is one run's close. A nil Run does nothing, so a code path shared with a run
// that closes elsewhere (a --all backfill's versions) can hold none.
type Run struct {
	p        presenter.Presenter
	began    time.Time
	info     *presenter.RunInfo
	landed   *presenter.RunResult
	unwound  *bool
	deferred []func()
}

// Begin starts timing a run whose events go to p.
func Begin(p presenter.Presenter) *Run {
	return &Run{p: p, began: time.Now()}
}

// Start opens the run with RunStarted.
func (r *Run) Start(info presenter.RunInfo) {
	if r == nil {
		return
	}
	r.info = &info
	r.p.RunStarted(info)
}

// Land records the ending the run reached; Close closes with it whatever error the
// run then returns, as a commit whose push failed does.
func (r *Run) Land(res presenter.RunResult) {
	if r == nil {
		return
	}
	r.landed = &res
}

// Unwound records that the run's unwind undid what it had made, clean reporting
// whether every recovery step succeeded.
func (r *Run) Unwound(clean bool) {
	if r == nil {
		return
	}
	r.unwound = &clean
}

// Defer registers f to run once Close has closed the run, after its RunFinished,
// deferred calls running last first: work whose narration has always followed the
// run's closing line, as the autostash pop's does. Unlike the other methods it needs
// a Run: a nil one has nowhere to keep f.
func (r *Run) Defer(f func()) {
	r.deferred = append(r.deferred, f)
}

// Close emits the run's one RunFinished, timed from Begin, unless the run never
// opened, then runs what Defer registered. A run that landed closes with its ending.
// One that stopped short closes at its start station: unwound when its unwind ran,
// else declined when declined says so, else failed, the last two with nothing
// changed.
func (r *Run) Close(declined bool) {
	if r == nil {
		return
	}
	if r.info != nil {
		res := r.stopped(declined)
		if r.landed != nil {
			res = *r.landed
		}
		res.Elapsed = time.Since(r.began)
		r.p.RunFinished(res)
	}
	for i := len(r.deferred) - 1; i >= 0; i-- {
		r.deferred[i]()
	}
}

func (r *Run) stopped(declined bool) presenter.RunResult {
	res := presenter.RunResult{Project: r.info.Project, Version: r.info.Version, Leaf: r.info.Leaf}
	back := func(detail string) presenter.Station {
		return presenter.Station{Name: r.info.Start.Name, Detail: detail}
	}
	switch {
	case r.unwound != nil && *r.unwound:
		res.Outcome, res.End = presenter.OutcomeUnwound, back(backWhereItStarted)
	case r.unwound != nil:
		res.Outcome, res.End = presenter.OutcomeUnwound, back(manualCleanupPending)
	case declined:
		res.Outcome, res.End = presenter.OutcomeDeclined, back(nothingChanged)
	default:
		res.Outcome, res.End = presenter.OutcomeFailed, back(nothingChanged)
	}
	return res
}
