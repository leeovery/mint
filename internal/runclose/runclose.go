// Package runclose holds one run between the RunStarted that opens it and the one
// RunFinished that closes it, so each orchestrator closes every way its run can end
// from a single place: the ending the run landed, or, when it stopped short, the
// start station it is back at, with what the run left there.
//
// A release, a commit, a single-version regenerate and a regenerate --all backfill are
// each one such run. A regenerate opens its run with its header alone, ahead of the
// questions that decide its start station, and draws the station once they are
// answered; plain's start line, which a backfill prints per version, comes later
// through the presenter, outside the Run.
//
// A stopped run's end detail states only mint's own actions (what it committed, reset,
// wrote, staged, created or kept, and that it pushed nothing) and where HEAD stands,
// never how clean the tree is or what a hook did. It is "nothing changed" only while
// no hook and no write of mint's has run; from then on the orchestrator records, with
// Wrote, Left or Unwound, what mint did.
package runclose

import (
	"strings"
	"time"

	"mint/internal/presenter"
)

// nothingChanged is the detail of the station a run that stopped short closes at when
// it left nothing behind.
const nothingChanged = "nothing changed"

// Run is one run's close. A nil Run does nothing, so a code path driven outside a
// run (a test driving a piece of one alone) can hold none.
type Run struct {
	p presenter.Presenter
	// now is the clock the run is timed by.
	now     func() time.Time
	began   time.Time
	info    *presenter.RunInfo
	landed  *presenter.RunResult
	unwound bool
	// wrote is what the run wrote that its close names first, "" for nothing, and left
	// what a run that stopped short left behind besides, "" for nothing.
	wrote, left string
	// at is the station a run that stopped short closes at in place of its start, ""
	// for the start.
	at       string
	deferred []func()
}

// Begin starts timing, by now, a run whose events go to p.
func Begin(p presenter.Presenter, now func() time.Time) *Run {
	return &Run{p: p, now: now, began: now()}
}

// Open opens the run with its header alone, a Quiet RunStarted carrying info's
// header, ahead of what decides its start station; Start draws the station. A run
// that closes before it has one closes with no end station.
func (r *Run) Open(info presenter.RunInfo) {
	if r == nil {
		return
	}
	opened := presenter.RunInfo{Project: info.Project, Header: info.Header, Quiet: true}
	r.info = &opened
	r.p.RunStarted(opened)
}

// Start opens the run with RunStarted, or, opened already, draws its start station.
func (r *Run) Start(info presenter.RunInfo) {
	if r == nil {
		return
	}
	r.info = &info
	r.p.RunStarted(info)
}

// Wrote records what the run wrote that no recovery takes back, which a close short
// of its ending names ahead of what Left or Unwound record: a --all backfill's
// provider releases. It replaces any recorded before.
func (r *Run) Wrote(detail string) {
	if r == nil {
		return
	}
	r.wrote = detail
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

// StoppedAt names the station a run that stops short closes at, in place of its
// start station, where HEAD never got back there: the branch a failed recovery reset
// left it on. An empty name keeps the start station.
func (r *Run) StoppedAt(name string) {
	if r == nil {
		return
	}
	r.at = name
}

// Left records what a run that stops short without an unwind leaves behind, the
// detail its close states in place of "nothing changed", replacing any recorded
// before.
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
// so, else failed, with the detail Wrote then Left or Unwound recorded, or "nothing
// changed" when none was.
func (r *Run) Close(declined bool) {
	if r == nil || r.info == nil {
		return
	}
	res := r.stopped(declined)
	if r.landed != nil {
		res = *r.landed
	}
	res.Elapsed = r.now().Sub(r.began)
	r.p.RunFinished(res)
}

func (r *Run) stopped(declined bool) presenter.RunResult {
	res := presenter.RunResult{Project: r.info.Project, Version: r.info.Version}
	switch {
	case r.unwound:
		res.Outcome = presenter.OutcomeUnwound
	case declined:
		res.Outcome = presenter.OutcomeDeclined
	default:
		res.Outcome = presenter.OutcomeFailed
	}
	var parts []string
	for _, part := range []string{r.wrote, r.left} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	detail := strings.Join(parts, " · ")
	if detail == "" {
		detail = nothingChanged
	}
	name := r.info.Start.Name
	if r.at != "" {
		name = r.at
	}
	res.End = presenter.Station{Name: name, Detail: detail}
	return res
}
