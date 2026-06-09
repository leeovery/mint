// Package presentertest provides test doubles for the presenter seam. It ships
// in its own subpackage (mirroring net/http/httptest) so these helpers stay out
// of the production presenter package's surface, and consequently works only
// against presenter's exported types.
//
// RecordingPresenter is the core double: it satisfies presenter.Presenter by
// capturing every call — in order, with its full payload — so that engine-driven
// tests can assert which events fired and with what data, independent of any
// rendering. It performs no rendering and no I/O.
package presentertest

import "mint/internal/presenter"

// EventKind identifies which Presenter method produced a recorded Event. The
// zero value (KindUnknown) is a sentinel that never corresponds to a real call,
// so an Event read before being populated is self-evidently invalid.
type EventKind int

const (
	KindUnknown EventKind = iota
	KindRunStarted
	KindStageStarted
	KindStageSucceeded
	KindStageFailed
	KindWarn
	KindShowPlan
	KindShowNotes
	KindRunFinished
)

// String renders the kind for readable test failure output.
func (k EventKind) String() string {
	switch k {
	case KindRunStarted:
		return "RunStarted"
	case KindStageStarted:
		return "StageStarted"
	case KindStageSucceeded:
		return "StageSucceeded"
	case KindStageFailed:
		return "StageFailed"
	case KindWarn:
		return "Warn"
	case KindShowPlan:
		return "ShowPlan"
	case KindShowNotes:
		return "ShowNotes"
	case KindRunFinished:
		return "RunFinished"
	default:
		return "Unknown"
	}
}

// Event is one recorded Presenter call. The tagged-struct form (a Kind
// discriminator plus one field per payload type) is chosen over parallel
// per-kind slices because a single ordered slice of these is what preserves
// call order across different event kinds — and assertions read naturally:
// inspect Kind to know which method fired, then read the matching payload field.
// Only the field named by Kind is populated; the rest are zero values.
type Event struct {
	Kind           EventKind
	RunStarted     presenter.RunInfo
	StageStarted   presenter.StageStart
	StageSucceeded presenter.StageSuccess
	StageFailed    presenter.StageFailure
	Warn           presenter.Warning
	ShowPlan       presenter.Plan
	ShowNotes      presenter.Notes
	RunFinished    presenter.RunResult
}

// RecordingPresenter satisfies presenter.Presenter by appending every call to a
// single ordered slice. Its zero value is ready to use: a freshly constructed
// recorder has a nil Events slice, records nothing, and its accessors return
// empty results without panicking.
type RecordingPresenter struct {
	// Events is the ordered log of every recorded call, including interleaving
	// across kinds. Tests may read it directly or via the accessors below.
	Events []Event
}

// Compile-time proof the recorder satisfies the interface it records.
var _ presenter.Presenter = (*RecordingPresenter)(nil)

// RunStarted records the start-of-run event.
func (r *RecordingPresenter) RunStarted(info presenter.RunInfo) {
	r.Events = append(r.Events, Event{Kind: KindRunStarted, RunStarted: info})
}

// StageStarted records the beginning of a stage.
func (r *RecordingPresenter) StageStarted(s presenter.StageStart) {
	r.Events = append(r.Events, Event{Kind: KindStageStarted, StageStarted: s})
}

// StageSucceeded records a stage's successful completion.
func (r *RecordingPresenter) StageSucceeded(s presenter.StageSuccess) {
	r.Events = append(r.Events, Event{Kind: KindStageSucceeded, StageSucceeded: s})
}

// StageFailed records a stage's failure.
func (r *RecordingPresenter) StageFailed(s presenter.StageFailure) {
	r.Events = append(r.Events, Event{Kind: KindStageFailed, StageFailed: s})
}

// Warn records a warning with its full structured payload — label and message —
// so an engine-driven test can round-trip the warning independent of any rendering.
func (r *RecordingPresenter) Warn(w presenter.Warning) {
	r.Events = append(r.Events, Event{Kind: KindWarn, Warn: w})
}

// ShowPlan records the plan event with its full structured payload so an
// engine-driven test can round-trip the steps independent of any rendering.
func (r *RecordingPresenter) ShowPlan(plan presenter.Plan) {
	r.Events = append(r.Events, Event{Kind: KindShowPlan, ShowPlan: plan})
}

// ShowNotes records the notes event with its full payload — version and verbatim
// body — so an engine-driven test can round-trip the notes independent of any
// rendering.
func (r *RecordingPresenter) ShowNotes(notes presenter.Notes) {
	r.Events = append(r.Events, Event{Kind: KindShowNotes, ShowNotes: notes})
}

// RunFinished records the end-of-run event.
func (r *RecordingPresenter) RunFinished(res presenter.RunResult) {
	r.Events = append(r.Events, Event{Kind: KindRunFinished, RunFinished: res})
}

// Kinds returns the ordered list of recorded event kinds — the ergonomic way to
// assert the exact sequence of calls without reaching into payloads. An empty
// recorder returns an empty (nil) slice.
func (r *RecordingPresenter) Kinds() []EventKind {
	kinds := make([]EventKind, len(r.Events))
	for i, ev := range r.Events {
		kinds[i] = ev.Kind
	}
	return kinds
}

// At returns the nth recorded Event and true, or the zero Event and false if n
// is out of range. The comma-ok form lets tests probe positions without
// panicking on an empty or short log.
func (r *RecordingPresenter) At(n int) (Event, bool) {
	if n < 0 || n >= len(r.Events) {
		return Event{}, false
	}
	return r.Events[n], true
}
