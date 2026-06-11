package main

import (
	"context"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/runner"
	"mint/internal/version"
)

// regenerateRunAxes maps the validated cmd-layer source/target selection onto the
// engine's optional axis types for the interactive default flow (task 5-10). It hands
// the engine the "ask vs skip" decision for each axis:
//
//   - SourceSet (a supplied --reuse/--fresh) maps to a PRESENT engine source so the
//     source prompt is skipped; an unset source maps to the engine UNSET so the prompt
//     asks. Source alone cannot express this — both --fresh and "no flag" resolve to
//     sourceFresh — which is why SourceSet exists.
//   - targetUnset maps to the engine UNSET target (ask); any resolved target maps to a
//     present engine target (skip). validateRegenerateRequest has already resolved
//     --reuse's implied --target release, so a reuse run arrives here with a present
//     release target either way; the engine's reuse axis contract also forces release.
func regenerateRunAxes(req regenerateRequest) (engine.OptionalRegenerateSource, engine.OptionalRegenerateTarget) {
	return regenerateSourceAxis(req), regenerateTargetAxis(req.Target)
}

// regenerateSourceAxis maps the source selection: a supplied flag is a present engine
// source; no flag is the engine UNSET (ask the source prompt).
func regenerateSourceAxis(req regenerateRequest) engine.OptionalRegenerateSource {
	if !req.SourceSet {
		return engine.SourceUnset()
	}
	if req.Source == sourceReuse {
		return engine.SourceOf(engine.RegenerateSourceReuse)
	}
	return engine.SourceOf(engine.RegenerateSourceFresh)
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
// single-version run: it reads the resolved source and dispatches to the matching 5-5
// reuse read or 5-6 fresh re-diff+AI producer. The closure is invoked AFTER the source
// prompt resolves, so an interactively-chosen source produces the right body.
func newRegenerateBodyProducer(r runner.CommandRunner, cfg config.Config, root string, res version.Resolution) func(context.Context, engine.RegenerateSource) (string, error) {
	return func(ctx context.Context, source engine.RegenerateSource) (string, error) {
		if source == engine.RegenerateSourceReuse {
			return engine.ReadReuseBody(ctx, r, res.Tag)
		}
		return engine.RegenerateFreshBody(ctx, r, nil, root, cfg, res)
	}
}

// newRegenerateRegeneratorProducer builds the engine.RegenerateRun ProduceRegenerator
// closure for a single-version run: it binds the per-run fresh regenerator
// (engine.RegenerateFreshRegenerator over the resolved range) for a FRESH source — the
// backing for the notes-review gate's `r` choice — and returns nil for REUSE, which runs
// the simple confirm with no review gate. It is the regenerator counterpart of
// newRegenerateBodyProducer, invoked AFTER the source resolves so an interactively-chosen
// fresh source gets a working `r`.
func newRegenerateRegeneratorProducer(r runner.CommandRunner, cfg config.Config, root string, res version.Resolution) func(engine.RegenerateSource) engine.Regenerator {
	return func(source engine.RegenerateSource) engine.Regenerator {
		if source == engine.RegenerateSourceReuse {
			return nil
		}
		return engine.RegenerateFreshRegenerator(r, nil, root, cfg, res)
	}
}
