package main

import (
	"context"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/publish"
	"mint/internal/runner"
	"mint/internal/version"
)

// regenerateRunAxes maps the validated cmd-layer source/target selection onto the
// engine's optional axis types for the interactive default flow (task 5-10). It hands
// the engine the "ask vs skip" decision for each axis:
//
//   - SourceSet (a supplied --source value) maps to a PRESENT engine source so the
//     source prompt is skipped; an unset source maps to the engine UNSET so the prompt
//     asks. Source alone cannot express this — both --source fresh and "no flag" resolve
//     to sourceFresh — which is why SourceSet exists.
//   - targetUnset maps to the engine UNSET target (ask); any resolved target maps to a
//     present engine target (skip). The axes are orthogonal, so the source never
//     constrains the target — a present source and an unset target simply ask the target
//     prompt.
func regenerateRunAxes(req regenerateRequest) (engine.OptionalRegenerateSource, engine.OptionalRegenerateTarget) {
	return regenerateSourceAxis(req), regenerateTargetAxis(req.Target)
}

// regenerateSourceAxis maps the source selection: a supplied flag is a present engine
// source; no flag is the engine UNSET (ask the source prompt).
func regenerateSourceAxis(req regenerateRequest) engine.OptionalRegenerateSource {
	if !req.SourceSet {
		return engine.SourceUnset()
	}
	switch req.Source {
	case sourceTag:
		return engine.SourceOf(engine.RegenerateSourceTag)
	case sourceRelease:
		return engine.SourceOf(engine.RegenerateSourceRelease)
	default:
		return engine.SourceOf(engine.RegenerateSourceFresh)
	}
}

// regenerateTargetAxis maps the target selection: targetUnset is the engine UNSET (ask
// the target prompt); any other value is a present engine target (skip the question).
func regenerateTargetAxis(target regenerateTarget) engine.OptionalRegenerateTarget {
	switch target {
	case targetRelease:
		return engine.TargetOf(engine.RegenerateTargetRelease)
	case targetChangelog:
		return engine.TargetOf(engine.RegenerateTargetChangelog)
	case targetBoth:
		return engine.TargetOf(engine.RegenerateTargetBoth)
	default:
		return engine.TargetUnset()
	}
}

// newRegenerateBodyProducer builds the engine.RegenerateRun ProduceBody closure for a
// single-version run: it reads the resolved source and dispatches to the matching reuse
// read, provider-release read, or fresh re-diff+AI producer. The closure is invoked
// AFTER the source prompt resolves, so an interactively-chosen source produces the right
// body. The publisher backs the provider-release source's read.
//
// It binds the fixed single-version Resolution and delegates to the canonical, Resolution-
// keyed newBatchBodyProducer so the source dispatch lives in exactly one place; the batch
// path uses the same producer threaded with each version's Resolution.
func newRegenerateBodyProducer(r runner.CommandRunner, cfg config.Config, root string, res version.Resolution, publisher publish.Publisher) func(context.Context, engine.RegenerateSource) (string, error) {
	produce := newBatchBodyProducer(r, cfg, root, publisher)
	return func(ctx context.Context, source engine.RegenerateSource) (string, error) {
		// The single-version path has no batch skip check, so it never pre-reads a
		// deterministic source's body — the producer's reuse/release branch performs the
		// (single) read.
		return produce(ctx, source, res, "")
	}
}

// newRegenerateRegeneratorProducer builds the engine.RegenerateRun ProduceRegenerator
// closure for a single-version run: it binds the per-run fresh regenerator
// (engine.RegenerateFreshRegenerator over the resolved range) for a FRESH source — the
// backing for the notes-review gate's `r` choice — and returns nil for REUSE, which runs
// the simple confirm with no review gate. It is the regenerator counterpart of
// newRegenerateBodyProducer, invoked AFTER the source resolves so an interactively-chosen
// fresh source gets a working `r`.
//
// It binds the fixed single-version Resolution and delegates to the canonical, Resolution-
// keyed newBatchRegeneratorProducer so the reuse/fresh dispatch lives in exactly one place;
// the batch path uses the same producer threaded with each version's Resolution.
func newRegenerateRegeneratorProducer(r runner.CommandRunner, cfg config.Config, root string, res version.Resolution) func(engine.RegenerateSource) engine.Regenerator {
	produce := newBatchRegeneratorProducer(r, cfg, root)
	return func(source engine.RegenerateSource) engine.Regenerator {
		return produce(source, res)
	}
}
