package engine_test

import (
	"context"
	"errors"
	"testing"

	"mint/internal/engine"
	"mint/internal/publish"
)

// fakePublisher is a recording Publisher test double for the create-or-update
// DISPATCH tests: it scripts the ReleaseExists probe outcome PER TAG and records,
// in order, which mutating method (CreateRelease / UpdateRelease) the dispatch
// routed to and with what arguments. Recording the dispatched method — rather than
// shelling gh — is what proves the dispatch decision behind the Publisher interface
// (no driver specifics leak into the orchestration).
type fakePublisher struct {
	// existsByTag scripts ReleaseExists per tag: the bool it returns and an optional
	// probe error. An unscripted tag returns (false, nil) — the absent default.
	existsByTag map[string]existsOutcome
	// dispatched records each routed mutation in call order.
	dispatched []dispatchedCall
	// beforeDispatch, when set, fires at the START of each routed mutation (before
	// the dispatched call is recorded), so an ordering test can snapshot external
	// state (e.g. whether the changelog push already happened) at dispatch time.
	beforeDispatch func()
}

type existsOutcome struct {
	exists bool
	err    error
}

type dispatchedCall struct {
	method string // "create" or "update"
	tag    string
	title  string
	body   string
}

func newFakePublisher() *fakePublisher {
	return &fakePublisher{existsByTag: make(map[string]existsOutcome)}
}

// Compile-time assertion that the double satisfies the seam the dispatch depends on.
var _ publish.Publisher = (*fakePublisher)(nil)

func (f *fakePublisher) seedExists(tag string, exists bool, err error) {
	f.existsByTag[tag] = existsOutcome{exists: exists, err: err}
}

func (f *fakePublisher) ReleaseExists(_ context.Context, tag string) (bool, error) {
	o := f.existsByTag[tag]
	return o.exists, o.err
}

func (f *fakePublisher) CreateRelease(_ context.Context, tag, title, body string) error {
	if f.beforeDispatch != nil {
		f.beforeDispatch()
	}
	f.dispatched = append(f.dispatched, dispatchedCall{method: "create", tag: tag, title: title, body: body})
	return nil
}

func (f *fakePublisher) UpdateRelease(_ context.Context, tag, title, body string) error {
	if f.beforeDispatch != nil {
		f.beforeDispatch()
	}
	f.dispatched = append(f.dispatched, dispatchedCall{method: "update", tag: tag, title: title, body: body})
	return nil
}

// TestDispatchRelease_ExistingReleaseUpdates proves that when a release EXISTS at
// the tag the dispatch routes to UpdateRelease (never CreateRelease), carrying the
// tag/title/body through unchanged.
func TestDispatchRelease_ExistingReleaseUpdates(t *testing.T) {
	t.Parallel()

	f := newFakePublisher()
	f.seedExists("v1.2.3", true, nil)

	if err := engine.DispatchRelease(t.Context(), f, "v1.2.3", "v1.2.3", "the body"); err != nil {
		t.Fatalf("DispatchRelease returned unexpected error: %v", err)
	}

	if len(f.dispatched) != 1 {
		t.Fatalf("dispatched %d calls, want exactly 1", len(f.dispatched))
	}
	got := f.dispatched[0]
	if got.method != "update" {
		t.Errorf("dispatched %q, want update when the release exists", got.method)
	}
	if got.tag != "v1.2.3" || got.title != "v1.2.3" || got.body != "the body" {
		t.Errorf("dispatched call = %+v, want tag/title/body threaded unchanged", got)
	}
}

// TestDispatchRelease_AbsentReleaseCreates proves that when NO release exists at the
// tag the dispatch routes to CreateRelease (never UpdateRelease).
func TestDispatchRelease_AbsentReleaseCreates(t *testing.T) {
	t.Parallel()

	f := newFakePublisher()
	f.seedExists("v1.2.3", false, nil)

	if err := engine.DispatchRelease(t.Context(), f, "v1.2.3", "v1.2.3", "the body"); err != nil {
		t.Fatalf("DispatchRelease returned unexpected error: %v", err)
	}

	if len(f.dispatched) != 1 {
		t.Fatalf("dispatched %d calls, want exactly 1", len(f.dispatched))
	}
	got := f.dispatched[0]
	if got.method != "create" {
		t.Errorf("dispatched %q, want create when the release is absent", got.method)
	}
	if got.tag != "v1.2.3" || got.title != "v1.2.3" || got.body != "the body" {
		t.Errorf("dispatched call = %+v, want tag/title/body threaded unchanged", got)
	}
}

// TestDispatchRelease_ResolvesPerVersion proves the create-vs-update decision is
// made PER VERSION: with two tags — one with an existing release, one absent — a
// batch that dispatches each in turn mixes exactly one update and one create
// transparently (the per-version contract the --all batch relies on).
func TestDispatchRelease_ResolvesPerVersion(t *testing.T) {
	t.Parallel()

	f := newFakePublisher()
	f.seedExists("v1.0.0", true, nil)  // existing release → update
	f.seedExists("v2.0.0", false, nil) // absent release → create

	for _, tag := range []string{"v1.0.0", "v2.0.0"} {
		if err := engine.DispatchRelease(t.Context(), f, tag, tag, "body for "+tag); err != nil {
			t.Fatalf("DispatchRelease(%s) returned unexpected error: %v", tag, err)
		}
	}

	if len(f.dispatched) != 2 {
		t.Fatalf("dispatched %d calls, want exactly 2 (one per version)", len(f.dispatched))
	}
	if got := f.dispatched[0]; got.method != "update" || got.tag != "v1.0.0" {
		t.Errorf("first dispatch = %+v, want update for v1.0.0 (existing release)", got)
	}
	if got := f.dispatched[1]; got.method != "create" || got.tag != "v2.0.0" {
		t.Errorf("second dispatch = %+v, want create for v2.0.0 (absent release)", got)
	}
}

// TestDispatchRelease_ProbeErrorSurfacesWithoutDispatch proves a GENUINE probe
// failure (not just not-found) is surfaced AND that neither CreateRelease nor
// UpdateRelease is dispatched — the dispatch must never silently default to
// create-or-update when the probe itself failed.
func TestDispatchRelease_ProbeErrorSurfacesWithoutDispatch(t *testing.T) {
	t.Parallel()

	probeErr := errors.New("HTTP 401: Bad credentials")
	f := newFakePublisher()
	f.seedExists("v1.2.3", false, probeErr)

	err := engine.DispatchRelease(t.Context(), f, "v1.2.3", "v1.2.3", "the body")
	if err == nil {
		t.Fatal("DispatchRelease returned nil error, want the probe failure surfaced")
	}
	if !errors.Is(err, probeErr) {
		t.Errorf("error = %v, want it to wrap the probe failure %v", err, probeErr)
	}
	if len(f.dispatched) != 0 {
		t.Errorf("dispatched %d calls, want 0 — a probe failure must not create or update", len(f.dispatched))
	}
}

// TestDispatchRelease_AcceptsPublisherInterface proves the dispatch helper depends
// only on the Publisher interface — no GitHub/driver specifics leak into the
// orchestration. The recording fakePublisher (not the GitHub driver) drives the
// decision, which only compiles and runs because the helper takes the interface.
func TestDispatchRelease_AcceptsPublisherInterface(t *testing.T) {
	t.Parallel()

	var p publish.Publisher = newFakePublisher()
	if err := engine.DispatchRelease(t.Context(), p, "v1.0.0", "v1.0.0", "body"); err != nil {
		t.Fatalf("DispatchRelease returned unexpected error: %v", err)
	}
}
