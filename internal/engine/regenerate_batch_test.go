package engine_test

import (
	"context"
	"slices"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// This file pins task 5-11: the batch `--all` single-version regeneration LOOP. It
// enumerates every matching version OLDEST → NEWEST, opens one RunStarted-narration
// block per version, produces notes per version (reuse read or fresh diff+AI), runs a
// per-version review gate by DEFAULT (-y opts out, no per-version gate), resolves
// create-vs-update per version via the 5-7 dispatch, and holds NO resume state.
//
// The loop is shaped so task 5-12 can wrap each iteration in skip-and-continue and
// task 5-13 can consume the COLLECTED per-version bodies (oldest → newest) for the
// whole-file CHANGELOG rebuild + single end commit — so this task does the RELEASE
// surface dispatch + per-version notes only (NO per-version changelog commit).

const (
	batchProject     = "acme"
	batchV1Tag       = "v1.0.0"
	batchV2Tag       = "v1.1.0"
	batchV3Tag       = "v2.0.0"
	batchFreshBodyV1 = "## v1 fresh\n"
	batchReuseBodyV1 = "## v1 reuse\n"
)

// threeVersions returns three matching versions in OLDEST → NEWEST order, the oldest
// marked first-release, each later version chaining off its predecessor.
func threeVersions() []version.Resolution {
	return []version.Resolution{
		{Tag: batchV1Tag, FirstRelease: true},
		{Tag: batchV2Tag, PreviousTag: batchV1Tag},
		{Tag: batchV3Tag, PreviousTag: batchV2Tag},
	}
}

// perVersionBody returns a deterministic body keyed off the version tag and source so
// a test can assert WHICH body flowed downstream per version without a real AI/tag read.
func perVersionBody() func(context.Context, engine.RegenerateSource, version.Resolution) (string, error) {
	return func(_ context.Context, src engine.RegenerateSource, res version.Resolution) (string, error) {
		prefix := "fresh"
		if src == engine.RegenerateSourceReuse {
			prefix = "reuse"
		}
		return "## " + prefix + " " + res.Tag + "\n", nil
	}
}

// batchReq builds a BatchRegenerateRequest for the given source/versions, the canned
// project + per-version body producer, and the -y flag.
func batchReq(source engine.RegenerateSource, versions []version.Resolution, yes bool) engine.BatchRegenerateRequest {
	return engine.BatchRegenerateRequest{
		Source:      source,
		Versions:    versions,
		Project:     batchProject,
		TagPrefix:   "v",
		Yes:         yes,
		ProduceBody: perVersionBody(),
	}
}

// runStartedVersions returns, in recorded order, the Version of every RunStarted event
// — the load-bearing way to assert one narration block opened per version, oldest →
// newest.
func runStartedVersions(rec *presentertest.RecordingPresenter) []string {
	var versions []string
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindRunStarted {
			versions = append(versions, ev.RunStarted.Version)
		}
	}
	return versions
}

// dispatchedTags returns the tags the fake publisher routed, in dispatch order.
func dispatchedTags(pub *fakePublisher) []string {
	tags := make([]string, len(pub.dispatched))
	for i, d := range pub.dispatched {
		tags[i] = d.tag
	}
	return tags
}

// batchDeps builds the ReleaseDeps the batch loop consumes (recording presenter +
// FakeRunner + Mutator), reusing the 5-9 write deps.
func batchDeps(rec *presentertest.RecordingPresenter, f *runner.FakeRunner) engine.ReleaseDeps {
	return regenWriteDeps(rec, f)
}

// TestRegenerateAll_ProcessesOldestToNewest proves the batch processes versions
// oldest → newest: the per-version provider dispatch fires in that order.
func TestRegenerateAll_ProcessesOldestToNewest(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceYes, presenter.ChoiceYes}}

	_, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub,
		batchReq(engine.RegenerateSourceReuse, threeVersions(), false))
	if err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}

	wantTags := []string{batchV1Tag, batchV2Tag, batchV3Tag}
	if got := dispatchedTags(pub); !slices.Equal(got, wantTags) {
		t.Errorf("dispatch order = %v, want oldest → newest %v", got, wantTags)
	}
}

// TestRegenerateAll_OneRunStartedBlockPerVersion proves each version opens its OWN
// narration block with its OWN RunStarted, emitted oldest → newest.
func TestRegenerateAll_OneRunStartedBlockPerVersion(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceYes, presenter.ChoiceYes}}

	if _, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub,
		batchReq(engine.RegenerateSourceReuse, threeVersions(), false)); err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}

	// The version KEY (bare x.y.z) is what the narration header carries.
	wantVersions := []string{"1.0.0", "1.1.0", "2.0.0"}
	if got := runStartedVersions(rec); !slices.Equal(got, wantVersions) {
		t.Errorf("RunStarted sequence = %v, want one block per version oldest → newest %v", got, wantVersions)
	}
	// Every block carries the regenerate action so each renders as its own run.
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindRunStarted && ev.RunStarted.Project != batchProject {
			t.Errorf("RunStarted Project = %q, want %q", ev.RunStarted.Project, batchProject)
		}
	}
}

// TestRegenerateAll_GeneratesNotesPerVersion proves notes are produced per version
// (here reuse read) and each version's OWN body flows to its OWN provider dispatch.
func TestRegenerateAll_GeneratesNotesPerVersion(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceYes, presenter.ChoiceYes}}

	bodies, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub,
		batchReq(engine.RegenerateSourceReuse, threeVersions(), false))
	if err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}

	// Each provider dispatch carries the version's own reuse body.
	if len(pub.dispatched) != 3 {
		t.Fatalf("dispatched %d, want 3 (one per version)", len(pub.dispatched))
	}
	for i, tag := range []string{batchV1Tag, batchV2Tag, batchV3Tag} {
		if want := "## reuse " + tag + "\n"; pub.dispatched[i].body != want {
			t.Errorf("dispatch[%d] body = %q, want %q", i, pub.dispatched[i].body, want)
		}
	}

	// The COLLECTED bodies (for the 5-13 whole-file rebuild) are returned oldest →
	// newest, each carrying the version's tag + body.
	if len(bodies) != 3 {
		t.Fatalf("collected %d bodies, want 3 (one per version, for the 5-13 rebuild)", len(bodies))
	}
	for i, tag := range []string{batchV1Tag, batchV2Tag, batchV3Tag} {
		if bodies[i].Resolution.Tag != tag {
			t.Errorf("collected[%d].Tag = %q, want %q (oldest → newest)", i, bodies[i].Resolution.Tag, tag)
		}
		if want := "## reuse " + tag + "\n"; bodies[i].Body != want {
			t.Errorf("collected[%d].Body = %q, want %q", i, bodies[i].Body, want)
		}
	}
}

// TestRegenerateAll_PerVersionReviewGateByDefault proves a per-version review gate
// runs by DEFAULT (no -y): a Prompt fires for EACH version.
func TestRegenerateAll_PerVersionReviewGateByDefault(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	// One confirm per version.
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceYes, presenter.ChoiceYes}}

	if _, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub,
		batchReq(engine.RegenerateSourceFresh, threeVersions(), false)); err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}

	var prompts int
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindPrompt {
			prompts++
		}
	}
	if prompts != 3 {
		t.Errorf("recorded %d Prompt gates, want one per version (3) by default", prompts)
	}
}

// TestRegenerateAll_YesOptsOutOfPerVersionGate proves -y opts OUT of the per-version
// gate: NO Prompt fires for any version (the batch runs fully unattended).
func TestRegenerateAll_YesOptsOutOfPerVersionGate(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	rec := &presentertest.RecordingPresenter{}

	if _, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub,
		batchReq(engine.RegenerateSourceFresh, threeVersions(), true)); err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}

	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindPrompt {
			t.Errorf("under -y a per-version gate fired (%+v); -y must opt out of all per-version gates", ev.Prompt)
		}
	}
	// All three versions still dispatched (unattended).
	if len(pub.dispatched) != 3 {
		t.Errorf("dispatched %d, want 3 under -y (unattended)", len(pub.dispatched))
	}
}

// TestRegenerateAll_MixesUpdateAndCreate proves create-vs-update is resolved PER
// VERSION via the 5-7 probe: an existing release updates, an absent one creates — the
// batch transparently mixes both.
func TestRegenerateAll_MixesUpdateAndCreate(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)  // exists → update
	pub.seedExists(batchV2Tag, false, nil) // absent → create
	pub.seedExists(batchV3Tag, true, nil)  // exists → update
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceYes, presenter.ChoiceYes}}

	if _, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub,
		batchReq(engine.RegenerateSourceReuse, threeVersions(), false)); err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}

	if len(pub.dispatched) != 3 {
		t.Fatalf("dispatched %d, want 3", len(pub.dispatched))
	}
	wantMethods := []string{"update", "create", "update"}
	for i, want := range wantMethods {
		if pub.dispatched[i].method != want {
			t.Errorf("dispatch[%d] (%s) = %q, want %q (per-version create-vs-update)", i, pub.dispatched[i].tag, pub.dispatched[i].method, want)
		}
	}
}

// TestRegenerateAll_NoResumeState proves the loop holds NO resume state and is
// re-runnable: no checkpoint file is read or written, and a second identical run
// produces the identical dispatch (deterministic for reuse).
func TestRegenerateAll_NoResumeState(t *testing.T) {
	t.Parallel()

	run := func() *fakePublisher {
		f := runner.NewFakeRunner()
		f.Seed("git", runner.Result{}, nil)
		pub := newFakePublisher()
		pub.seedExists(batchV1Tag, true, nil)
		pub.seedExists(batchV2Tag, true, nil)
		pub.seedExists(batchV3Tag, true, nil)
		rec := &presentertest.RecordingPresenter{}
		if _, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub,
			batchReq(engine.RegenerateSourceReuse, threeVersions(), true)); err != nil {
			t.Fatalf("RegenerateAll returned unexpected error: %v", err)
		}
		// No checkpoint/state file read or written: the loop only reads tags and writes
		// provider releases — assert it never touched a file-state command.
		for _, inv := range f.Invocations() {
			for _, arg := range inv.Args {
				if arg == "checkpoint" || arg == "--resume" || arg == ".mint-regenerate-state" {
					t.Errorf("loop used resume/checkpoint state %v; the batch must hold none", inv.Args)
				}
			}
		}
		return pub
	}

	first := dispatchedTags(run())
	second := dispatchedTags(run())
	if !slices.Equal(first, second) {
		t.Errorf("re-run dispatch differs: first %v, second %v; --reuse --all must be deterministic", first, second)
	}
}

// TestRegenerateAll_DeclineAbortsBatch proves a declined per-version gate aborts the
// run non-zero (skip-and-continue is task 5-12; for now a decline propagates).
func TestRegenerateAll_DeclineAbortsBatch(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	// Decline the FIRST version's confirm.
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceNo}}

	_, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub,
		batchReq(engine.RegenerateSourceReuse, threeVersions(), false))

	assertAbortNonZero(t, err)
	if len(pub.dispatched) != 0 {
		t.Errorf("a declined first-version gate still dispatched %+v", pub.dispatched)
	}
}

// TestRegenerateAll_EmptyVersions_NoOp proves an empty version set is a clean no-op:
// no narration block, no dispatch, no error.
func TestRegenerateAll_EmptyVersions_NoOp(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	pub := newFakePublisher()
	rec := &presentertest.RecordingPresenter{}

	bodies, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub,
		batchReq(engine.RegenerateSourceReuse, nil, true))
	if err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}
	if len(bodies) != 0 {
		t.Errorf("collected %d bodies, want 0 for an empty version set", len(bodies))
	}
	if len(pub.dispatched) != 0 {
		t.Errorf("dispatched %d, want 0 for an empty version set", len(pub.dispatched))
	}
	if len(runStartedVersions(rec)) != 0 {
		t.Errorf("emitted %d narration blocks, want 0 for an empty version set", len(runStartedVersions(rec)))
	}
}
