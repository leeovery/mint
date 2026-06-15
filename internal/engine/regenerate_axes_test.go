package engine_test

import (
	"slices"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
)

// This file pins the SHARED axis resolver (ResolveRegenerateAxes): the source-then-target
// resolution the interactive default flow uses, factored out so the --all batch path can
// resolve the axes ONCE up front with the SAME gate idiom rather than silently defaulting.
// The load-bearing facts:
//
//   - an unset source ASKS via SourceGate, then an unset target ASKS via TargetGate —
//     source THEN target, in that order;
//   - a supplied source/target SKIPS its question;
//   - the axes are ORTHOGONAL: no source forces a target, so a reuse/from-release source
//     with an unset target still ASKS the target question (a deterministic source can
//     write the changelog, not just the provider release);
//   - under -y the resolver still calls Prompt at every applicable gate (the presenter
//     models the skip by returning the gate default).

// TestResolveRegenerateAxes_NoFlags_AsksSourceThenTarget proves the fully interactive
// resolution with neither axis supplied: it asks source, THEN target — in that order —
// and returns the chosen enums.
func TestResolveRegenerateAxes_NoFlags_AsksSourceThenTarget(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
			switch g.Subject {
			case "source":
				return presenter.Choice("fresh"), nil
			case "target":
				return presenter.Choice("both"), nil
			default:
				return presenter.ChoiceYes, nil
			}
		},
	}

	source, target, err := engine.ResolveRegenerateAxes(rec, engine.SourceUnset(), engine.TargetUnset(), true)
	if err != nil {
		t.Fatalf("ResolveRegenerateAxes returned unexpected error: %v", err)
	}
	if source != engine.RegenerateSourceFresh {
		t.Errorf("source = %v, want fresh", source)
	}
	if target != engine.RegenerateTargetBoth {
		t.Errorf("target = %v, want both", target)
	}
	if got, want := axisGateSubjects(rec), []string{"source", "target"}; !slices.Equal(got, want) {
		t.Errorf("gate subjects = %v, want %v (source then target)", got, want)
	}
}

// TestResolveRegenerateAxes_SuppliedAxes_SkipBothQuestions proves a supplied source and
// target skip BOTH questions — no gate fires.
func TestResolveRegenerateAxes_SuppliedAxes_SkipBothQuestions(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}

	source, target, err := engine.ResolveRegenerateAxes(rec,
		engine.SourceOf(engine.RegenerateSourceFresh),
		engine.TargetOf(engine.RegenerateTargetChangelog), true)
	if err != nil {
		t.Fatalf("ResolveRegenerateAxes returned unexpected error: %v", err)
	}
	if source != engine.RegenerateSourceFresh {
		t.Errorf("source = %v, want fresh", source)
	}
	if target != engine.RegenerateTargetChangelog {
		t.Errorf("target = %v, want changelog", target)
	}
	if got := axisGateSubjects(rec); len(got) != 0 {
		t.Errorf("gate subjects = %v, want none (both axes supplied)", got)
	}
}

// TestResolveRegenerateAxes_SuppliedTarget_SkipsTargetQuestion proves a supplied target
// skips the target question: only the source question fires.
func TestResolveRegenerateAxes_SuppliedTarget_SkipsTargetQuestion(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
			return presenter.Choice("fresh"), nil
		},
	}

	source, target, err := engine.ResolveRegenerateAxes(rec,
		engine.SourceUnset(), engine.TargetOf(engine.RegenerateTargetBoth), true)
	if err != nil {
		t.Fatalf("ResolveRegenerateAxes returned unexpected error: %v", err)
	}
	if source != engine.RegenerateSourceFresh {
		t.Errorf("source = %v, want fresh", source)
	}
	if target != engine.RegenerateTargetBoth {
		t.Errorf("target = %v, want both (supplied target unchanged)", target)
	}
	if got, want := axisGateSubjects(rec), []string{"source"}; !slices.Equal(got, want) {
		t.Errorf("gate subjects = %v, want %v (supplied target skips the target question)", got, want)
	}
}

// TestResolveRegenerateAxes_ReuseFlag_AsksTargetWhenUnset proves the axes are
// orthogonal: a reuse source flag with an UNSET target still ASKS the target question
// (the source no longer forces release), so reuse can target the changelog.
func TestResolveRegenerateAxes_ReuseFlag_AsksTargetWhenUnset(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
			if g.Subject == "target" {
				return presenter.Choice("changelog"), nil
			}
			return presenter.ChoiceYes, nil
		},
	}

	source, target, err := engine.ResolveRegenerateAxes(rec,
		engine.SourceOf(engine.RegenerateSourceTag), engine.TargetUnset(), true)
	if err != nil {
		t.Fatalf("ResolveRegenerateAxes returned unexpected error: %v", err)
	}
	if source != engine.RegenerateSourceTag {
		t.Errorf("source = %v, want tag", source)
	}
	if target != engine.RegenerateTargetChangelog {
		t.Errorf("target = %v, want changelog (reuse can target the changelog)", target)
	}
	if got, want := axisGateSubjects(rec), []string{"target"}; !slices.Equal(got, want) {
		t.Errorf("gate subjects = %v, want %v (reuse flag skips source but still asks the target)", got, want)
	}
}

// TestResolveRegenerateAxes_ReleaseChosenInteractively_AsksTarget proves that when the
// provider-release source is chosen at the SOURCE PROMPT (no source flag), the target is
// still asked — source THEN target — and a deterministic source can resolve to the
// changelog.
func TestResolveRegenerateAxes_ReleaseChosenInteractively_AsksTarget(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
			switch g.Subject {
			case "source":
				return presenter.Choice("release"), nil
			case "target":
				return presenter.Choice("changelog"), nil
			default:
				return presenter.ChoiceYes, nil
			}
		},
	}

	source, target, err := engine.ResolveRegenerateAxes(rec,
		engine.SourceUnset(), engine.TargetUnset(), true)
	if err != nil {
		t.Fatalf("ResolveRegenerateAxes returned unexpected error: %v", err)
	}
	if source != engine.RegenerateSourceRelease {
		t.Errorf("source = %v, want release", source)
	}
	if target != engine.RegenerateTargetChangelog {
		t.Errorf("target = %v, want changelog (a deterministic source can target the changelog)", target)
	}
	if got, want := axisGateSubjects(rec), []string{"source", "target"}; !slices.Equal(got, want) {
		t.Errorf("gate subjects = %v, want %v (source then target — orthogonal axes)", got, want)
	}
}

// TestResolveRegenerateAxes_Yes_StillCallsPrompt proves that under -y the resolver still
// calls Prompt at every applicable gate (the recorder models the presenter-internal skip
// by returning the gate default): no flags resolves to fresh + release via two prompts.
func TestResolveRegenerateAxes_Yes_StillCallsPrompt(t *testing.T) {
	t.Parallel()

	// No PromptResult / NextChoices: each Prompt falls back to the gate default — exactly
	// how the recorder models the presenter-internal -y skip+echo.
	rec := &presentertest.RecordingPresenter{}

	source, target, err := engine.ResolveRegenerateAxes(rec,
		engine.SourceUnset(), engine.TargetUnset(), true)
	if err != nil {
		t.Fatalf("ResolveRegenerateAxes returned unexpected error: %v", err)
	}
	if source != engine.RegenerateSourceFresh {
		t.Errorf("source = %v, want fresh (the source gate default)", source)
	}
	if target != engine.RegenerateTargetRelease {
		t.Errorf("target = %v, want release (the target gate default)", target)
	}
	if got, want := axisGateSubjects(rec), []string{"source", "target"}; !slices.Equal(got, want) {
		t.Errorf("gate subjects = %v, want %v (every applicable gate still prompted)", got, want)
	}
}

// axisGateSubjects returns the Subject of each recorded gate in order — the load-bearing
// way to assert the axis prompt sequence.
func axisGateSubjects(rec *presentertest.RecordingPresenter) []string {
	var subjects []string
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindPrompt {
			subjects = append(subjects, ev.Prompt.Subject)
		}
	}
	return subjects
}
