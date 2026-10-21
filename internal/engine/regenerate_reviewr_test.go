package engine_test

import (
	"errors"
	"slices"
	"testing"

	"mint/internal/engine"
	"mint/internal/notescache"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

func TestRegenerateWrite_FreshRegeneratorWithoutRunState_FailedRegenSavesNothingNamesNoFileAndHintsNothing(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(regenWriteTag, true, nil)
	const contextLine = "Lead with the new auth package."
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen},
		NextLines:   []string{contextLine},
	}
	userCache := t.TempDir()
	deps := regenWriteDeps(rec, f)
	deps.ChunkNotes = notescache.NewChunkNotes(chunkNotesBaseIn(userCache))
	failure := failsWithLogin()
	regen := &fakeRegenerator{err: failure}
	req := freshWriteReq(engine.RegenerateTargetRelease)
	req.Regenerator = regen

	err := engine.RegenerateWrite(t.Context(), deps, pub, t.TempDir(), req)

	assertAbortExitsOne(t, err)
	if !errors.Is(err, failure) {
		t.Errorf("err = %v, want the regenerator's failure", err)
	}
	if !slices.Equal(regen.gotContexts, []string{contextLine}) {
		t.Errorf("Regenerator contexts = %q, want [%q]", regen.gotContexts, contextLine)
	}
	assertNoSavedChunkNotes(t, chunkNotesBaseIn(userCache))
	if got := nextMovesEvents(rec); len(got) != 0 {
		t.Errorf("next moves = %+v, want no saved file named", got)
	}
	if got := warnings(rec); len(got) != 0 {
		t.Errorf("warnings = %+v, want no hint and no unsaved-notes warning", got)
	}
	if len(pub.dispatched) != 0 {
		t.Errorf("provider writes = %d, want none", len(pub.dispatched))
	}
}

func TestRegenerateRun_DepsRegeneratorWired_RegenUsesTheOverridesBody(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, true, nil)
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceYes},
		NextLines:   []string{exportNudge},
	}
	override := &fakeRegenerator{bodies: []string{regen1Body}}
	perRun := &fakeRegenerator{bodies: []string{regen2Body}}
	deps := regenWriteDeps(rec, f)
	deps.Regenerator = override
	req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease))
	req.ProduceRegenerator = func(engine.RegenerateSource, engine.RegenRun) engine.Regenerator { return perRun }

	if err := engine.RegenerateRun(t.Context(), deps, pub, t.TempDir(), req); err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	if !slices.Equal(override.gotContexts, []string{exportNudge}) {
		t.Errorf("override contexts = %q, want [%q]", override.gotContexts, exportNudge)
	}
	if len(perRun.gotContexts) != 0 {
		t.Errorf("per-run regenerator consulted %d times, want never", len(perRun.gotContexts))
	}
	if len(pub.dispatched) != 1 || pub.dispatched[0].body != regen1Body {
		t.Errorf("provider dispatch = %+v, want the override's body %q", pub.dispatched, regen1Body)
	}
}
