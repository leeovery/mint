package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/publish"
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
func runRegenerateAll(deps engine.ReleaseDeps, r runner.CommandRunner, cfg config.Config, root string, req regenerateRequest) int {
	ctx := context.Background()

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

	// A non-github / no-remote host downgrades to an unresolved publisher; the engine's
	// release-write surfaces that, so pass the resolver result through (nil publisher on
	// an unresolved provider), exactly as the single-version path does.
	publisher, _ := publish.ResolvePublisher(regenerateRemoteURL(ctx, r), cfg.Release.Provider, r)

	batch := engine.BatchRegenerateRequest{
		Source:      source,
		Versions:    versions,
		Project:     filepath.Base(root),
		TagPrefix:   cfg.Release.TagPrefix,
		Yes:         req.Yes,
		Target:      target,
		ProduceBody: newBatchBodyProducer(r, cfg, root),
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

// newBatchBodyProducer builds the engine batch ProduceBody closure: per version it
// dispatches to the matching 5-5 reuse read or 5-6 fresh re-diff+AI producer for the
// resolved source. It is the batch counterpart of newRegenerateBodyProducer, keyed off
// the per-version Resolution rather than a single fixed version.
func newBatchBodyProducer(r runner.CommandRunner, cfg config.Config, root string) func(context.Context, engine.RegenerateSource, version.Resolution) (string, error) {
	return func(ctx context.Context, source engine.RegenerateSource, res version.Resolution) (string, error) {
		if source == engine.RegenerateSourceReuse {
			return engine.ReadReuseBody(ctx, r, res.Tag)
		}
		return engine.RegenerateFreshBody(ctx, r, nil, root, cfg, res)
	}
}

// newBatchRegeneratorProducer builds the engine batch ProduceRegenerator closure: per
// version it binds the per-version fresh regenerator (engine.RegenerateFreshRegenerator
// over that version's resolved range) for a FRESH source — backing each version's
// notes-review gate `r` choice — and returns nil for REUSE (the simple confirm has no
// review gate). It is the batch counterpart of newRegenerateRegeneratorProducer, keyed
// off the per-version Resolution.
func newBatchRegeneratorProducer(r runner.CommandRunner, cfg config.Config, root string) func(engine.RegenerateSource, version.Resolution) engine.Regenerator {
	return func(source engine.RegenerateSource, res version.Resolution) engine.Regenerator {
		if source == engine.RegenerateSourceReuse {
			return nil
		}
		return engine.RegenerateFreshRegenerator(r, nil, root, cfg, res)
	}
}
