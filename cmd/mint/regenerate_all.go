package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/publish"
	"mint/internal/runner"
	"mint/internal/version"
)

// runRegenerateAll executes the `mint release regenerate --all` batch backfill as one
// run (engine.RegenerateAllRun): it opens with the header, resolves the source/target
// axes interactively ONCE up front (mirroring the single-version path), then prepares
// the batch — enumerating every matching version OLDEST → NEWEST (task 5-11), resolving
// the publishing driver, and building the per-version body producer (5-5 reuse read /
// 5-6 fresh re-diff+AI) — and runs it validated (5-12 up-front config check +
// skip-and-continue + end summary), with the end-of-batch whole-file CHANGELOG rebuild
// (5-13) for a changelog/both target. It returns the process exit code.
//
// Axis resolution is interactive by default: a bare --all (no source flag, no --target)
// ASKS source THEN target before the batch runs — mint never guesses which live
// surface(s) to rewrite unattended. A supplied flag skips its question; a reuse source
// FORCES release without asking (the 5-2 axis contract). -y is threaded so the per-version
// review gates skip; the axis prompts themselves skip+echo inside the presenter under -y.
func runRegenerateAll(ctx context.Context, deps engine.ReleaseDeps, r runner.CommandRunner, cfg config.Config, root, releaseBranch string, req regenerateRequest) int {
	source, target := regenerateRunAxes(req)
	// A version set that can't be read has no stage to fail: its line is the cmd
	// layer's own, printed once the run has closed.
	var unreadable error
	prepare := func(ctx context.Context, source engine.RegenerateSource, target engine.RegenerateTarget) (engine.BatchRegenerateRequest, publish.Publisher, error) {
		versions, err := version.ResolveAllRegenerateTargets(ctx, r, cfg.Release.TagPrefix)
		if err != nil {
			unreadable = err
			return engine.BatchRegenerateRequest{}, nil, err
		}

		// Resolve the publishing driver through the SHARED engine helper, exactly as the
		// single-version path does: an unresolvable provider warns and downgrades (a nil
		// publisher proceeds, every per-version provider write is skipped downstream),
		// any other resolution error aborts.
		publisher, err := engine.ResolvePublisher(ctx, deps, cfg)
		if err != nil {
			return engine.BatchRegenerateRequest{}, nil, err
		}

		return engine.BatchRegenerateRequest{
			Source:    source,
			Versions:  versions,
			Project:   filepath.Base(root),
			TagPrefix: cfg.Release.TagPrefix,
			Yes:       req.Yes,
			Target:    target,
			// The resolved release branch backs the batch preflight on-branch /
			// remote-sync gates, which run AFTER the interactive axis prompts resolve
			// the target — the only point at which a bare `--all` (no --target) knows
			// which surface(s) it writes.
			ReleaseBranch: releaseBranch,
			ProduceBody:   newBatchBodyProducer(r, cfg, root, publisher),
			// Each version's fresh notes-review gate `r` choice consults this
			// per-version regenerator, bound to that version's resolved range. Without
			// it the rendered `r` would abort on every interactive fresh `--all`
			// backfill.
			ProduceRegenerator: newBatchRegeneratorProducer(r, cfg, root),
		}, publisher, nil
	}

	err := engine.RegenerateAllRun(ctx, deps, root, engine.RegenerateAllRunRequest{
		Source:           source,
		Target:           target,
		ChangelogEnabled: cfg.Release.Changelog,
		Project:          filepath.Base(root),
		Prepare:          prepare,
	})
	if unreadable != nil {
		fmt.Fprintf(os.Stderr, "mint: %v\n", unreadable)
	}
	if err != nil {
		return exitCode(err)
	}
	return 0
}

// newBatchBodyProducer builds the canonical, Resolution-keyed ProduceBody closure: per
// version it dispatches to the matching deterministic read (reuse → tag annotation,
// release → provider release body) or the fresh re-diff+AI producer for the resolved
// source. This is the single home of the body source dispatch — the batch path threads
// each version's Resolution through it, and newRegenerateBodyProducer binds the
// single-version's fixed Resolution and delegates here. The publisher backs the
// provider-release read; a fresh version reports a chunked run to its sinks.
func newBatchBodyProducer(r runner.CommandRunner, cfg config.Config, root string, publisher publish.Publisher) func(context.Context, engine.RegenerateSource, version.Resolution, string, engine.NotesSinks) (string, error) {
	return func(ctx context.Context, source engine.RegenerateSource, res version.Resolution, preReadBody string, sinks engine.NotesSinks) (string, error) {
		switch source {
		case engine.RegenerateSourceTag:
			// The batch loop pre-reads the annotation body for its skip check and
			// threads it through, so each tag is read ONCE; the single-version
			// delegation never pre-reads (empty preReadBody) and reads here instead.
			if preReadBody != "" {
				return preReadBody, nil
			}
			return engine.RequireTagBody(ctx, r, res.Tag)
		case engine.RegenerateSourceRelease:
			// As with reuse: the batch pre-reads the release body (one read per version),
			// the single-version delegation reads here through the Publisher seam.
			if preReadBody != "" {
				return preReadBody, nil
			}
			return engine.RequireReleaseBody(ctx, publisher, res.Tag)
		default:
			return engine.RegenerateFreshBody(ctx, r, nil, root, cfg, res, sinks)
		}
	}
}

// newBatchRegeneratorProducer builds the canonical, Resolution-keyed ProduceRegenerator
// closure: per version it binds the per-version fresh regenerator
// (engine.RegenerateFreshRegenerator over the merge inputs that version's chunked run
// kept, else over its resolved range) for a FRESH source — backing each version's
// notes-review gate `r` choice — and returns nil for the deterministic sources (reuse,
// release), whose simple confirm has no review gate. This is the single home of the
// regenerator source dispatch — the batch path threads each version's Resolution
// through it, and newRegenerateRegeneratorProducer binds the single-version's fixed
// Resolution and delegates here.
func newBatchRegeneratorProducer(r runner.CommandRunner, cfg config.Config, root string) func(engine.RegenerateSource, version.Resolution, engine.RegenRun) engine.Regenerator {
	return func(source engine.RegenerateSource, res version.Resolution, run engine.RegenRun) engine.Regenerator {
		if source != engine.RegenerateSourceFresh {
			return nil
		}
		return engine.RegenerateFreshRegenerator(r, nil, root, cfg, res, run)
	}
}
