package engine_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// This file pins task 5-10: the regenerate INTERACTIVE DEFAULT FLOW — resolving the
// source and target axes (asking via SourceGate/TargetGate when the flag is unset),
// showing the plan summary, then delegating to the 5-9 write path (which owns the
// confirm / notes-review gate + the write). The load-bearing facts under test:
//
//   - no flags → ask source THEN target THEN show plan THEN confirm (the gate order);
//   - a supplied source SKIPS the source question; a supplied target SKIPS the target;
//   - flags WITHOUT -y still confirm (the write path's confirm/review gate still runs);
//   - -y → the engine still calls Prompt at every gate (the recorder models the
//     presenter-internal skip by returning the gate default); no extra branching;
//   - fresh → the four-choice notes-review gate runs before writing; reuse → the
//     two-choice simple confirm only (assert which gate keys appear);
//   - a reuse source forces target = release (the axis contract honoured interactively).

const (
	regenRunTag        = "v1.4.0"
	regenRunVersionKey = "1.4.0"
	regenRunProject    = "acme"
	regenRunFreshBody  = "## What's Changed\n\n- Fresh notes\n"
	regenRunReuseBody  = "## What's Changed\n\n- Reused annotation\n"
)

// staticBody returns a ProduceBody closure that yields the source-appropriate canned
// body — fresh vs reuse — so a test can assert which body flowed downstream without a
// real AI/transport or tag read.
func staticBody() func(context.Context, engine.RegenerateSource) (string, error) {
	return func(_ context.Context, src engine.RegenerateSource) (string, error) {
		if src == engine.RegenerateSourceReuse {
			return regenRunReuseBody, nil
		}
		return regenRunFreshBody, nil
	}
}

// runReq builds a RegenerateRunRequest with the canned tag/version/project, the
// static body producer, changelog enabled, and the given axis options + Yes flag.
func runReq(source engine.OptionalRegenerateSource, target engine.OptionalRegenerateTarget, yes bool) engine.RegenerateRunRequest {
	return engine.RegenerateRunRequest{
		Source:           source,
		Target:           target,
		Tag:              regenRunTag,
		VersionKey:       regenRunVersionKey,
		Project:          regenRunProject,
		ChangelogEnabled: true,
		Yes:              yes,
		ProduceBody:      staticBody(),
	}
}

// promptGates returns, in recorded order, every gate the recorder captured — the
// ergonomic way to assert the prompt SEQUENCE across source/target/confirm.
func promptGates(rec *presentertest.RecordingPresenter) []presenter.Gate {
	var gates []presenter.Gate
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindPrompt {
			gates = append(gates, ev.Prompt)
		}
	}
	return gates
}

// gateSubjects returns the Subject of each recorded gate in order ("source",
// "target", "notes") — the load-bearing way to assert the prompt sequence.
func gateSubjects(rec *presentertest.RecordingPresenter) []string {
	gates := promptGates(rec)
	subjects := make([]string, len(gates))
	for i, g := range gates {
		subjects[i] = g.Subject
	}
	return subjects
}

// freshRunDeps builds the ReleaseDeps the run path consumes, reusing the 5-9 write
// deps (recording presenter + FakeRunner + Mutator). A release-only fresh run issues
// no git mutation, so the runner stays unseeded unless the test seeds it.
func freshRunDeps(rec *presentertest.RecordingPresenter, f *runner.FakeRunner) engine.ReleaseDeps {
	return regenWriteDeps(rec, f)
}

// TestRegenerateRun_NoFlags_AsksSourceThenTargetThenPlanThenConfirm proves the fully
// interactive path with no flags: it asks source, THEN target, THEN shows the plan,
// THEN confirms — in that exact order.
func TestRegenerateRun_NoFlags_AsksSourceThenTargetThenPlanThenConfirm(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, true, nil)
	// Script: source=fresh, target=release, confirm=yes.
	rec := &presentertest.RecordingPresenter{
		PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
			switch g.Subject {
			case "source":
				return presenter.Choice("fresh"), nil
			case "target":
				return presenter.Choice("release"), nil
			default:
				return presenter.ChoiceYes, nil
			}
		},
	}

	err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, dir,
		runReq(engine.SourceUnset(), engine.TargetUnset(), false))
	if err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	// The gate SEQUENCE must be source → target → notes(confirm), and the plan must
	// be shown BEFORE the confirm gate.
	if got, want := gateSubjects(rec), []string{"source", "target", "notes"}; !slices.Equal(got, want) {
		t.Errorf("gate subjects = %v, want %v", got, want)
	}
	assertPlanBeforeConfirm(t, rec)
}

// TestRegenerateRun_SourceFlag_SkipsSourceQuestion proves a supplied source flag
// skips the source question: only target then confirm are asked.
func TestRegenerateRun_SourceFlag_SkipsSourceQuestion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, true, nil)
	rec := &presentertest.RecordingPresenter{
		PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
			if g.Subject == "target" {
				return presenter.Choice("release"), nil
			}
			return presenter.ChoiceYes, nil
		},
	}

	err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, dir,
		runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetUnset(), false))
	if err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	if got, want := gateSubjects(rec), []string{"target", "notes"}; !slices.Equal(got, want) {
		t.Errorf("gate subjects = %v, want %v (source flag must skip the source question)", got, want)
	}
}

// TestRegenerateRun_TargetFlag_SkipsTargetQuestion proves a supplied --target skips
// the target question: only source then confirm are asked.
func TestRegenerateRun_TargetFlag_SkipsTargetQuestion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, true, nil)
	rec := &presentertest.RecordingPresenter{
		PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
			if g.Subject == "source" {
				return presenter.Choice("fresh"), nil
			}
			return presenter.ChoiceYes, nil
		},
	}

	err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, dir,
		runReq(engine.SourceUnset(), engine.TargetOf(engine.RegenerateTargetRelease), false))
	if err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	if got, want := gateSubjects(rec), []string{"source", "notes"}; !slices.Equal(got, want) {
		t.Errorf("gate subjects = %v, want %v (target flag must skip the target question)", got, want)
	}
}

// TestRegenerateRun_BothFlags_NoYes_StillConfirms proves flags WITHOUT -y skip both
// questions but the run STILL confirms (one gate fires: the confirm/review gate).
func TestRegenerateRun_BothFlags_NoYes_StillConfirms(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, true, nil)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, dir,
		runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease), false))
	if err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	if got, want := gateSubjects(rec), []string{"notes"}; !slices.Equal(got, want) {
		t.Errorf("gate subjects = %v, want %v (flags skip questions but still confirm)", got, want)
	}
}

// TestRegenerateRun_Yes_AlwaysCallsPrompt proves that under -y the engine STILL calls
// Prompt at every gate point (the presenter-internal skip is modelled by the recorder
// returning the gate default); the run proceeds without any extra branching. With no
// flags this is source → target → confirm — three Prompt calls — all auto-answered.
func TestRegenerateRun_Yes_AlwaysCallsPrompt(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, true, nil)
	// No PromptResult / NextChoices: each Prompt falls back to the gate default, which
	// is exactly how the recorder models the presenter-internal -y skip+echo.
	rec := &presentertest.RecordingPresenter{}

	err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, dir,
		runReq(engine.SourceUnset(), engine.TargetUnset(), true))
	if err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	if got, want := gateSubjects(rec), []string{"source", "target", "notes"}; !slices.Equal(got, want) {
		t.Errorf("under -y the engine must still call Prompt at every gate; subjects = %v, want %v", got, want)
	}
	// The source/target gates carry the chosen value in AcceptEcho — the -y echo
	// payload — so a captured log shows which axis was used. The defaults are
	// reuse-vs-fresh's first option (fresh) and release for target.
	gates := promptGates(rec)
	if gates[0].AcceptEcho != string(gates[0].Default) {
		t.Errorf("source gate AcceptEcho = %q, want the chosen value %q", gates[0].AcceptEcho, gates[0].Default)
	}
}

// TestRegenerateRun_Fresh_RunsNotesReviewGate proves the fresh path runs the
// four-choice notes-review gate (y/n/e/r) as its confirm — backfilled notes are
// reviewable before they overwrite the live surface.
func TestRegenerateRun_Fresh_RunsNotesReviewGate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, true, nil)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, dir,
		runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease), false))
	if err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	confirm := confirmGate(t, rec)
	wantKeys := []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo, presenter.ChoiceEdit, presenter.ChoiceRegen}
	if !slices.Equal(confirm.Keys(), wantKeys) {
		t.Errorf("fresh confirm gate keys = %v, want the four-choice review gate %v", confirm.Keys(), wantKeys)
	}
}

// TestRegenerateRun_Reuse_SimpleConfirmOnly proves a reuse source runs the
// two-choice simple confirm (y/n) only — no e/r review gate — and forces
// target=release (the axis contract honoured interactively: the target question is
// never asked).
func TestRegenerateRun_Reuse_SimpleConfirmOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, false, nil)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	// Reuse source supplied, target UNSET: the axis contract forces release, so the
	// target question must NOT be asked.
	err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, dir,
		runReq(engine.SourceOf(engine.RegenerateSourceReuse), engine.TargetUnset(), false))
	if err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	// Only the confirm gate fires (reuse skips its own source question via the flag,
	// and the forced release target skips the target question).
	if got, want := gateSubjects(rec), []string{"notes"}; !slices.Equal(got, want) {
		t.Errorf("reuse gate subjects = %v, want %v (forced target=release, no target question)", got, want)
	}
	confirm := confirmGate(t, rec)
	wantKeys := []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo}
	if !slices.Equal(confirm.Keys(), wantKeys) {
		t.Errorf("reuse confirm gate keys = %v, want the simple confirm %v (no e/r)", confirm.Keys(), wantKeys)
	}
	// The reuse body flowed to the provider create (target forced to release).
	if len(pub.dispatched) != 1 || pub.dispatched[0].body != regenRunReuseBody {
		t.Errorf("provider dispatch = %+v, want one create with the reuse body", pub.dispatched)
	}
}

// TestRegenerateRun_ReuseChosenInteractively_ForcesTargetRelease proves the axis
// contract is honoured even when reuse is chosen at the SOURCE PROMPT (no source
// flag): once the source resolves to reuse, the target is forced to release and the
// target question is never asked.
func TestRegenerateRun_ReuseChosenInteractively_ForcesTargetRelease(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, true, nil)
	rec := &presentertest.RecordingPresenter{
		PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
			if g.Subject == "source" {
				return presenter.Choice("reuse"), nil
			}
			return presenter.ChoiceYes, nil
		},
	}

	err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, dir,
		runReq(engine.SourceUnset(), engine.TargetUnset(), false))
	if err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	// Source asked (and answered reuse), then NO target question, then the simple
	// confirm — the axis contract forced release.
	if got, want := gateSubjects(rec), []string{"source", "notes"}; !slices.Equal(got, want) {
		t.Errorf("gate subjects = %v, want %v (reuse forces release; no target question)", got, want)
	}
	confirm := confirmGate(t, rec)
	if !slices.Equal(confirm.Keys(), []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo}) {
		t.Errorf("a reuse-chosen run used %v, want the simple confirm", confirm.Keys())
	}
}

// TestRegenerateRun_ConfirmDecline_Aborts proves a declined confirm aborts non-zero
// (the write path owns the abort; the run surfaces it).
func TestRegenerateRun_ConfirmDecline_Aborts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := runner.NewFakeRunner()
	pub := newFakePublisher()
	rec := &presentertest.RecordingPresenter{
		PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
			switch g.Subject {
			case "source":
				return presenter.Choice("fresh"), nil
			case "target":
				return presenter.Choice("release"), nil
			default:
				return presenter.ChoiceNo, nil
			}
		},
	}

	err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, dir,
		runReq(engine.SourceUnset(), engine.TargetUnset(), false))

	assertAbortNonZero(t, err)
	if len(pub.dispatched) != 0 {
		t.Errorf("a declined confirm dispatched a provider write %+v", pub.dispatched)
	}
}

// TestRegenerateRun_BodyProducerError_Surfaces proves a body-production failure
// surfaces as an abort BEFORE the plan/confirm (no notes gate fires).
func TestRegenerateRun_BodyProducerError_Surfaces(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := runner.NewFakeRunner()
	pub := newFakePublisher()
	rec := &presentertest.RecordingPresenter{
		PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
			switch g.Subject {
			case "source":
				return presenter.Choice("fresh"), nil
			case "target":
				return presenter.Choice("release"), nil
			default:
				return presenter.ChoiceYes, nil
			}
		},
	}

	req := runReq(engine.SourceUnset(), engine.TargetUnset(), false)
	req.ProduceBody = func(context.Context, engine.RegenerateSource) (string, error) {
		return "", errors.New("diff too large")
	}

	err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, dir, req)

	if err == nil {
		t.Fatalf("RegenerateRun returned nil, want a surfaced body-production error")
	}
	for _, s := range gateSubjects(rec) {
		if s == "notes" {
			t.Errorf("the confirm gate fired despite a body-production failure; subjects = %v", gateSubjects(rec))
		}
	}
}

// assertPlanBeforeConfirm fails unless a ShowPlan event was recorded before the
// confirm (notes) gate — the plan summary is shown before the confirm.
func assertPlanBeforeConfirm(t *testing.T, rec *presentertest.RecordingPresenter) {
	t.Helper()
	planAt, confirmAt := -1, -1
	for i, ev := range rec.Events {
		if ev.Kind == presentertest.KindShowPlan && planAt == -1 {
			planAt = i
		}
		if ev.Kind == presentertest.KindPrompt && ev.Prompt.Subject == "notes" && confirmAt == -1 {
			confirmAt = i
		}
	}
	if planAt == -1 {
		t.Fatalf("no ShowPlan event recorded; kinds = %v", rec.Kinds())
	}
	if confirmAt == -1 {
		t.Fatalf("no confirm (notes) gate recorded; kinds = %v", rec.Kinds())
	}
	if planAt > confirmAt {
		t.Errorf("ShowPlan (at %d) fired AFTER the confirm gate (at %d); the plan must precede the confirm", planAt, confirmAt)
	}
}

// confirmGate returns the LAST recorded gate — the confirm / notes-review gate the
// write path fires after the source/target questions — failing if none fired.
func confirmGate(t *testing.T, rec *presentertest.RecordingPresenter) presenter.Gate {
	t.Helper()
	gates := promptGates(rec)
	if len(gates) == 0 {
		t.Fatalf("no Prompt gate recorded; kinds = %v", rec.Kinds())
	}
	return gates[len(gates)-1]
}
