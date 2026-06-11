package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/runner"
	"mint/internal/version"
)

// runRegenerateAll executes the `mint release regenerate --all` batch backfill: it
// resolves the source/target axes interactively ONCE up front (mirroring the
// single-version path), enumerates every matching version OLDEST → NEWEST (task 5-11),
// resolves the publishing driver, builds the per-version body producer (5-5 reuse read /
// 5-6 fresh re-diff+AI), and runs the validated batch (5-12 up-front config check +
// skip-and-continue + end summary) which performs the end-of-batch whole-file CHANGELOG
// rebuild (5-13) for a changelog/both target. It returns the process exit code.
//
// Axis resolution is interactive by default: a bare --all (no source flag, no --target)
// ASKS source THEN target before the batch runs — mint never guesses which live
// surface(s) to rewrite unattended. A supplied flag skips its question; a reuse source
// FORCES release without asking (the 5-2 axis contract). -y is threaded so the per-version
// review gates skip; the axis prompts themselves skip+echo inside the presenter under -y.
func runRegenerateAll(ctx context.Context, deps engine.ReleaseDeps, r runner.CommandRunner, cfg config.Config, root, releaseBranch string, req regenerateRequest) int {
	// Resolve both axes ONCE before the batch, via the shared interactive resolver the
	// single-version path uses (replacing the old silent fresh+release defaulting).
	source, target, err := resolveBatchAxes(deps.Presenter, req, cfg.Release.Changelog)
	if err != nil {
		return exitCode(err)
	}

	versions, err := version.ResolveAllRegenerateTargets(ctx, r, cfg.Release.TagPrefix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return usageExitCode
	}

	// Resolve the publishing driver through the SHARED engine helper, exactly as the
	// single-version path does: an unresolvable provider warns and downgrades (a nil
	// publisher proceeds, every per-version provider write is skipped downstream), any
	// other resolution error aborts. This REPLACES the former `publisher, _ := …` discard
	// that crashed the per-version DispatchRelease on a non-github / no-remote origin.
	publisher, code, proceed := resolveRegeneratePublisher(ctx, deps, cfg)
	if !proceed {
		return code
	}

	batch := engine.BatchRegenerateRequest{
		Source:    source,
		Versions:  versions,
		Project:   filepath.Base(root),
		TagPrefix: cfg.Release.TagPrefix,
		Yes:       req.Yes,
		Target:    target,
		// The resolved release branch backs the batch preflight on-branch / remote-sync
		// gates, which run at the RegenerateAllValidated entry point AFTER the interactive
		// axis prompts above resolve the target — the only point at which a bare `--all`
		// (no --target) knows which surface(s) it writes.
		ReleaseBranch: releaseBranch,
		ProduceBody:   newBatchBodyProducer(r, cfg, root),
		// Each version's fresh notes-review gate `r` choice consults this per-version
		// regenerator, bound to that version's resolved range. Without it the rendered
		// `r` would abort on every interactive fresh `--all` backfill.
		ProduceRegenerator: newBatchRegeneratorProducer(r, cfg, root),
	}

	if err := engine.RegenerateAllValidated(ctx, deps, publisher, root, batch, cfg.Release.Changelog); err != nil {
		return exitCode(err)
	}
	return 0
}

// resolveBatchAxes resolves the --all batch's source/target axes ONCE up front via the
// SHARED engine resolver (engine.ResolveRegenerateAxes), the same gate idiom the
// single-version interactive flow uses. It maps the validated cmd request onto the
// engine's optional-axis types (reusing regenerateRunAxes — a supplied flag skips its
// question, an unset axis is asked), then delegates: an unset source asks SourceGate, an
// unset target on a fresh source asks TargetGate, and a reuse source forces release
// without asking (the 5-2 axis contract). It returns the resolved engine enums or a
// surfaced gate abort.
func resolveBatchAxes(p presenter.Presenter, req regenerateRequest, changelogEnabled bool) (engine.RegenerateSource, engine.RegenerateTarget, error) {
	source, target := regenerateRunAxes(req)
	return engine.ResolveRegenerateAxes(p, source, target, changelogEnabled)
}

// newBatchBodyProducer builds the canonical, Resolution-keyed ProduceBody closure: per
// version it dispatches to the matching 5-5 reuse read or 5-6 fresh re-diff+AI producer
// for the resolved source. This is the single home of the body reuse/fresh dispatch — the
// batch path threads each version's Resolution through it, and newRegenerateBodyProducer
// binds the single-version's fixed Resolution and delegates here.
func newBatchBodyProducer(r runner.CommandRunner, cfg config.Config, root string) func(context.Context, engine.RegenerateSource, version.Resolution, string) (string, error) {
	return func(ctx context.Context, source engine.RegenerateSource, res version.Resolution, reuseBody string) (string, error) {
		if source == engine.RegenerateSourceReuse {
			// The batch loop pre-reads the annotation body for its skip check and
			// threads it through, so each tag is read ONCE; the single-version
			// delegation never pre-reads (empty reuseBody) and reads here instead.
			if reuseBody != "" {
				return reuseBody, nil
			}
			return engine.ReadReuseBody(ctx, r, res.Tag)
		}
		return engine.RegenerateFreshBody(ctx, r, nil, root, cfg, res)
	}
}

// newBatchRegeneratorProducer builds the canonical, Resolution-keyed ProduceRegenerator
// closure: per version it binds the per-version fresh regenerator
// (engine.RegenerateFreshRegenerator over that version's resolved range) for a FRESH
// source — backing each version's notes-review gate `r` choice — and returns nil for REUSE
// (the simple confirm has no review gate). This is the single home of the regenerator
// reuse/fresh dispatch — the batch path threads each version's Resolution through it, and
// newRegenerateRegeneratorProducer binds the single-version's fixed Resolution and
// delegates here.
func newBatchRegeneratorProducer(r runner.CommandRunner, cfg config.Config, root string) func(engine.RegenerateSource, version.Resolution) engine.Regenerator {
	return func(source engine.RegenerateSource, res version.Resolution) engine.Regenerator {
		if source == engine.RegenerateSourceReuse {
			return nil
		}
		return engine.RegenerateFreshRegenerator(r, nil, root, cfg, res)
	}
}
