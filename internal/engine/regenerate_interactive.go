package engine

// This file is the regenerate INTERACTIVE DEFAULT FLOW (task 5-10): the orchestration
// that sits BEFORE the 5-9 write path and resolves the two axes — source then target —
// asking the user via SourceGate / TargetGate whenever the corresponding flag was not
// supplied, then showing the plan summary and delegating to RegenerateWrite (which owns
// the confirm / notes-review gate and the write/push/recovery).
//
// The flow mirrors the spec's "asks source, asks target, shows the plan, confirms":
//
//	resolve source (ask iff unset) → produce body for the source →
//	resolve target (ask iff unset, axis-contract aware) → RunStarted + ShowPlan →
//	RegenerateWrite (ShowNotes + confirm/review gate + write)
//
// AXIS CONTRACT (the 5-2 source × target rule, honoured INTERACTIVELY here): once the
// source resolves to REUSE — whether by flag or by the source prompt — the target is
// FORCED to release and the target question is NOT asked (reuse's source IS the notes
// record, so it can only write the provider release). A fresh source with no --target
// asks the full release/changelog/both target question.
//
// -y handling mirrors the forward path: the engine ALWAYS calls Prompt at every gate
// (source, target, confirm); the -y SKIP happens INSIDE the presenter, which renders
// the accept echo and returns the gate Default. So this orchestration never branches
// around a Prompt call on -y — it asks every applicable gate unconditionally, and a
// -y presenter answers each with its default + echo. (Flags pre-fill the source/target
// answers and skip ONLY those questions; the confirm still fires unless -y.)
//
// SCOPE: this owns the source/target RESOLUTION + plan + delegation only. Body
// PRODUCTION (5-5 reuse read / 5-6 fresh re-diff+AI) is injected as ProduceBody so the
// flow stays testable with a RecordingPresenter and a FakeRunner without a real AI
// transport or tag read; the confirm/review gate and the write/push/recovery are 5-9's.

import (
	"context"

	"mint/internal/presenter"
	"mint/internal/publish"
)

// The Choice keys the source/target gates offer. They are the ASCII enumerated option
// values SourceGate / TargetGate render and parse (case-folded by the presenter), and
// they double as the gate AcceptEcho so a -y log reads "source: fresh (-y)" etc.
const (
	choiceSourceFresh = presenter.Choice("fresh")
	choiceSourceReuse = presenter.Choice("reuse")

	choiceTargetRelease   = presenter.Choice("release")
	choiceTargetChangelog = presenter.Choice("changelog")
	choiceTargetBoth      = presenter.Choice("both")
)

// OptionalRegenerateSource carries either a flag-supplied RegenerateSource or the
// UNSET state (no source flag), so the interactive flow can distinguish "asked via
// flag" from "ask the user". It mirrors the cmd layer's sourceUnset distinction at the
// engine boundary without overloading RegenerateSource's zero value.
type OptionalRegenerateSource struct {
	// set reports whether a source was supplied (a flag). When false the source is
	// resolved by the source prompt.
	set bool
	// value is the supplied source, meaningful only when set is true.
	value RegenerateSource
}

// SourceOf wraps a flag-supplied source as a present OptionalRegenerateSource.
func SourceOf(s RegenerateSource) OptionalRegenerateSource {
	return OptionalRegenerateSource{set: true, value: s}
}

// SourceUnset is the no-source-flag state — the source is asked interactively.
func SourceUnset() OptionalRegenerateSource {
	return OptionalRegenerateSource{}
}

// OptionalRegenerateTarget carries either a flag-supplied RegenerateTarget or the
// UNSET state (no --target), so the interactive flow can distinguish a supplied target
// from "ask the user" without overloading RegenerateTarget's zero value (which is the
// load-bearing RegenerateTargetRelease).
type OptionalRegenerateTarget struct {
	// set reports whether a --target was supplied. When false the target is resolved
	// by the target prompt (or forced to release by the reuse axis contract).
	set bool
	// value is the supplied target, meaningful only when set is true.
	value RegenerateTarget
}

// TargetOf wraps a flag-supplied target as a present OptionalRegenerateTarget.
func TargetOf(t RegenerateTarget) OptionalRegenerateTarget {
	return OptionalRegenerateTarget{set: true, value: t}
}

// TargetUnset is the no-target-flag state — the target is asked interactively (or
// forced to release when the source is reuse).
func TargetUnset() OptionalRegenerateTarget {
	return OptionalRegenerateTarget{}
}

// RegenerateRunRequest carries the single-version interactive run inputs: the
// optional axis selections (a supplied flag skips its prompt), the resolved tag /
// version key / project, whether the changelog is enabled (which target options are
// offerable), the -y flag, and the injected body producer.
type RegenerateRunRequest struct {
	// Source is the flag-supplied source or unset (ask the source prompt).
	Source OptionalRegenerateSource
	// Target is the flag-supplied target or unset (ask the target prompt, or force
	// release when the source is reuse).
	Target OptionalRegenerateTarget
	// Tag is the canonical target tag (e.g. "v1.4.0").
	Tag string
	// VersionKey is the bare x.y.z key used in the changelog header and the notes/plan.
	VersionKey string
	// Project is the project name shown in the start-of-run header.
	Project string
	// ChangelogEnabled gates which target options the target prompt offers: when false
	// the changelog/both options are not offerable (the static config check upstream
	// already rejects a flag-supplied changelog/both target).
	ChangelogEnabled bool
	// Yes is the -y flag. It does NOT change which gates the engine calls — every gate
	// is still called and the presenter skips internally — but it threads to the write
	// request for the confirm/review gate's internal skip.
	Yes bool
	// ProduceBody yields the notes body for the RESOLVED source. It is injected so the
	// flow stays testable without a real AI transport / tag read; production wires the
	// 5-5 reuse read or the 5-6 fresh re-diff+AI here.
	ProduceBody func(context.Context, RegenerateSource) (string, error)
}

// RegenerateRun runs the interactive default flow for one resolved version: it asks
// the source (when unset), produces the body for the resolved source, asks the target
// (when unset, honouring the reuse⇒release axis contract), shows the start-of-run
// header + plan summary, then delegates to RegenerateWrite for the confirm/review gate
// and the write/push/recovery. It returns whatever RegenerateWrite returns (nil on
// success, an *AbortError on a confirm decline or pre-push failure, nil-with-warn on a
// post-push provider failure), or a surfaced abort on a body-production failure.
func RegenerateRun(ctx context.Context, deps ReleaseDeps, publisher publish.Publisher, root string, req RegenerateRunRequest) error {
	p := deps.Presenter

	// Axis 1 — resolve the source: a supplied flag skips the question; otherwise ask.
	source, err := resolveSource(p, req.Source)
	if err != nil {
		return err
	}

	// Axis 2 — resolve the target: reuse FORCES release (no question); a supplied flag
	// skips the question; otherwise ask the release/changelog/both target prompt. Both
	// questions are asked back-to-back (source then target) before any slow work.
	target, err := resolveTarget(p, source, req.Target, req.ChangelogEnabled)
	if err != nil {
		return err
	}

	// Produce the body for the resolved source BEFORE the confirm — the notes-review
	// gate (fresh) reviews this body. A production failure aborts before the plan.
	body, err := req.ProduceBody(ctx, source)
	if err != nil {
		return surface(p, "notes", err)
	}

	// Start-of-run header + plan summary, shown BEFORE the confirm (the spec order:
	// asks source, asks target, shows the plan, confirms).
	p.RunStarted(presenter.RunInfo{
		Project: req.Project,
		Version: req.VersionKey,
		Action:  regenerateAction,
	})
	p.ShowPlan(regeneratePlan(source, target, req.Tag))

	// Delegate to the 5-9 write path: it shows the notes, runs the source-appropriate
	// confirm/review gate (fresh → notes-review, reuse → simple confirm), and writes.
	return RegenerateWrite(ctx, deps, publisher, root, RegenerateWriteRequest{
		Source:     source,
		Target:     target,
		Tag:        req.Tag,
		VersionKey: req.VersionKey,
		Body:       body,
	})
}

// regenerateAction is the start-of-run verb word the header renders for a regenerate
// run, the regenerate counterpart of the forward releaseAction.
const regenerateAction = "regenerating"

// resolveSource returns the flag-supplied source unchanged, or asks the source prompt
// (SourceGate, reuse/fresh) when none was supplied. The engine ALWAYS calls Prompt for
// the question when unset — the -y skip+echo happens inside the presenter.
func resolveSource(p presenter.Presenter, opt OptionalRegenerateSource) (RegenerateSource, error) {
	if opt.set {
		return opt.value, nil
	}
	choice, err := ReviewDecision(p, presenter.SourceGate(sourceOptions(), choiceSourceFresh))
	if err != nil {
		return 0, err
	}
	return sourceFromChoice(choice), nil
}

// resolveTarget returns the resolved target: reuse forces release (no question asked —
// the axis contract); a flag-supplied target is returned unchanged; otherwise the
// target prompt (TargetGate) asks among the offerable surfaces. The default offered is
// release.
func resolveTarget(p presenter.Presenter, source RegenerateSource, opt OptionalRegenerateTarget, changelogEnabled bool) (RegenerateTarget, error) {
	// Axis contract: reuse can only write the provider release, so force release and
	// never ask — even when reuse was chosen interactively at the source prompt.
	if source == RegenerateSourceReuse {
		return RegenerateTargetRelease, nil
	}
	if opt.set {
		return opt.value, nil
	}
	choice, err := ReviewDecision(p, presenter.TargetGate(targetOptions(changelogEnabled), choiceTargetRelease))
	if err != nil {
		return 0, err
	}
	return targetFromChoice(choice), nil
}

// sourceOptions is the ordered source-prompt menu: fresh (the default) then reuse.
func sourceOptions() []presenter.GateChoice {
	return []presenter.GateChoice{
		{Key: choiceSourceFresh, Action: "re-diff + AI"},
		{Key: choiceSourceReuse, Action: "tag annotation body"},
	}
}

// targetOptions is the ordered target-prompt menu: release first (the default), then —
// only when the changelog is enabled — changelog and both. mint never offers a
// changelog surface the project opted out of.
func targetOptions(changelogEnabled bool) []presenter.GateChoice {
	options := []presenter.GateChoice{
		{Key: choiceTargetRelease, Action: "provider release"},
	}
	if changelogEnabled {
		options = append(options,
			presenter.GateChoice{Key: choiceTargetChangelog, Action: "CHANGELOG.md"},
			presenter.GateChoice{Key: choiceTargetBoth, Action: "both"},
		)
	}
	return options
}

// sourceFromChoice maps a source-gate Choice to the engine source enum. The gate
// returns only a declared key, so the reuse key is the only non-fresh case.
func sourceFromChoice(c presenter.Choice) RegenerateSource {
	if c == choiceSourceReuse {
		return RegenerateSourceReuse
	}
	return RegenerateSourceFresh
}

// targetFromChoice maps a target-gate Choice to the engine target enum. The gate
// returns only a declared key; release is the default for the (unreachable) other case.
func targetFromChoice(c presenter.Choice) RegenerateTarget {
	switch c {
	case choiceTargetChangelog:
		return RegenerateTargetChangelog
	case choiceTargetBoth:
		return RegenerateTargetBoth
	default:
		return RegenerateTargetRelease
	}
}

// regeneratePlan builds the plan summary shown before the confirm: the source, the
// target surface(s), and the tag. Create-vs-update is resolved later per version (5-7),
// so the plan names the surfaces rather than the dispatch decision.
func regeneratePlan(source RegenerateSource, target RegenerateTarget, tag string) presenter.Plan {
	steps := []presenter.PlanStep{
		{Verb: "source", Target: sourceLabel(source)},
	}
	if target.writesChangelog() {
		steps = append(steps, presenter.PlanStep{Verb: "changelog", Target: tag})
	}
	if target.writesProvider() {
		steps = append(steps, presenter.PlanStep{Verb: "release", Target: tag})
	}
	return presenter.Plan{Steps: steps}
}

// sourceLabel renders the source word for the plan summary.
func sourceLabel(source RegenerateSource) string {
	if source == RegenerateSourceReuse {
		return string(choiceSourceReuse)
	}
	return string(choiceSourceFresh)
}
