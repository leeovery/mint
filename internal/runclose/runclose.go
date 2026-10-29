// Package runclose holds one run between the RunStarted that opens it and the one
// RunFinished that closes it, so each orchestrator closes every way its run can end
// from a single place: the ending the run landed, or, when it stopped short, the
// start station it is back at, with what the run left there.
//
// A release, a single-version regenerate and a commit are each one such run. A
// regenerate --all backfill is not: it opens a block per version with a RunStarted of
// its own and closes the batch once, with one RunFinished, only when it succeeds, so
// it holds no Run.
package runclose

import (
	"time"

	"mint/internal/presenter"
)

// nothingChanged is the detail of the station a run that stopped short closes at when
// it left nothing behind.
const nothingChanged = "nothing changed"

// Run is one run's close. A nil Run does nothing, so a code path shared with a run
// that closes elsewhere (a --all backfill's versions) can hold none.
type Run struct {
	p       presenter.Presenter
	began   time.Time
	info    *presenter.RunInfo
	landed  *presenter.RunResult
	unwound bool
	// left is what a run that stopped short left behind, "" for nothing.
	left     string
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

// Unwound records that the run's unwind undid what it had made, detail saying where
// that left the repo.
func (r *Run) Unwound(detail string) {
	if r == nil {
		return
	}
	r.unwound, r.left = true, detail
}

// Left records what a run that stops short without an unwind leaves behind, the
// detail its close states in place of "nothing changed".
func (r *Run) Left(detail string) {
	if r == nil {
		return
	}
	r.left = detail
}

// Defer registers f to run once the run is over, deferred calls running last first:
// work whose narration has always followed the run's closing line, as the autostash
// pop's does. Unlike the other methods it needs a Run: a nil one has nowhere to keep f.
func (r *Run) Defer(f func()) {
	r.deferred = append(r.deferred, f)
}

// RunDeferred runs what Defer registered and not yet run. An orchestrator defers it
// ahead of its run, so the work runs after Close on the way out, and still runs, as a
// deferred call would, when the run panics and never reaches Close.
func (r *Run) RunDeferred() {
	if r == nil {
		return
	}
	for len(r.deferred) > 0 {
		last := len(r.deferred) - 1
		f := r.deferred[last]
		r.deferred = r.deferred[:last]
		f()
	}
}

// Close emits the run's one RunFinished, timed from Begin, unless the run never
// opened. A run that landed closes with its ending. One that stopped short closes at
// its start station: unwound when its unwind ran, else declined when declined says
// so, else failed, with what it left there, or nothing changed.
func (r *Run) Close(declined bool) {
	if r == nil || r.info == nil {
		return
	}
	res := r.stopped(declined)
	if r.landed != nil {
		res = *r.landed
	}
	res.Elapsed = time.Since(r.began)
	r.p.RunFinished(res)
}

func (r *Run) stopped(declined bool) presenter.RunResult {
	res := presenter.RunResult{Project: r.info.Project, Version: r.info.Version, Leaf: r.info.Leaf}
	switch {
	case r.unwound:
		res.Outcome = presenter.OutcomeUnwound
	case declined:
		res.Outcome = presenter.OutcomeDeclined
	default:
		res.Outcome = presenter.OutcomeFailed
	}
	detail := r.left
	if detail == "" {
		detail = nothingChanged
	}
	res.End = presenter.Station{Name: r.info.Start.Name, Detail: detail}
	return res
}
