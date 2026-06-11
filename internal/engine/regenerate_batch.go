package engine

// This file is the regenerate batch `--all` LOOP (task 5-11): the orchestration that
// backfills EVERY matching version in one run. The version set is enumerated OLDEST →
// NEWEST upstream (version.ResolveAllRegenerateTargets), and this loop processes each
// in turn: it opens ONE narration block per version (each with its OWN RunStarted,
// emitted in order — the presenter renders blocks linearly in emit order, so block
// ordering is ENGINE-owned), produces the version's notes body (reuse read or fresh
// re-diff+AI, injected as ProduceBody), runs a per-version review gate BY DEFAULT
// (-y opts out to run fully unattended), and dispatches the RELEASE surface via the
// 5-7 create-or-update DISPATCH — so the batch transparently mixes updates and creates
// per version.
//
// SCOPE: this task owns the RELEASE surface dispatch + per-version notes only. It does
// NO per-version changelog commit — in `--all` mode the changelog is a whole-file
// rebuild + single END commit (task 5-13), so the loop COLLECTS each version's produced
// body (oldest → newest) and returns the collection for 5-13 to consume. The
// per-version processing is factored into processOneVersion (returning a per-version
// result + error the loop currently propagates) so task 5-12 can wrap each iteration in
// skip-and-continue without restructuring the loop.
//
// NO RESUME STATE: the loop reads no checkpoint and writes none — a re-run simply
// re-processes from the top. `--reuse --all` is deterministic; `--fresh --all`
// re-generates (stochastic but harmless).

import (
	"context"

	"mint/internal/presenter"
	"mint/internal/publish"
	"mint/internal/version"
)

// BatchRegenerateRequest carries the resolved inputs for one `--all` batch run: the
// chosen source, the OLDEST → NEWEST version set, the project + prefix for narration,
// the -y opt-out, and the injected per-version body producer.
type BatchRegenerateRequest struct {
	// Source selects the per-version gating + production: fresh runs the re-diff+AI
	// producer and the notes-review gate; reuse reads the tag annotation and runs the
	// simple confirm.
	Source RegenerateSource
	// Versions is the matching version set to backfill, ordered OLDEST → NEWEST. The
	// loop processes it in slice order and never reorders.
	Versions []version.Resolution
	// Project is the project name shown in each version's start-of-run header.
	Project string
	// TagPrefix is the configured tag prefix, used to derive each version's bare x.y.z
	// key (for the narration header + notes) from its canonical tag.
	TagPrefix string
	// Yes is the -y opt-out: when true the per-version review gate/confirm is SKIPPED
	// entirely (the engine does not even prompt) so the batch runs fully unattended.
	Yes bool
	// ProduceBody yields one version's notes body for the resolved source. It is
	// injected so the loop stays testable without a real AI transport / tag read;
	// production wires the 5-5 reuse read or the 5-6 fresh re-diff+AI per version.
	ProduceBody func(context.Context, RegenerateSource, version.Resolution) (string, error)
}

// RegeneratedVersion is one version's produced body collected by the batch loop, in
// OLDEST → NEWEST order. Task 5-13 consumes the collection to rebuild CHANGELOG.md
// whole from every version's section; this task collects it as a by-product of the
// per-version release dispatch.
type RegeneratedVersion struct {
	// Resolution is the version this body belongs to (its canonical tag + fresh diff
	// base), carried so 5-13 can render the section header per version.
	Resolution version.Resolution
	// Body is the version's full regenerated notes body, produced for the resolved
	// source and already gate-reviewed (unless -y skipped the gate).
	Body string
}

// RegenerateAll runs the batch `--all` loop over the request's OLDEST → NEWEST version
// set: for each version it opens a narration block, produces the body, runs the
// per-version gate (unless -y), and dispatches the provider release (create-or-update
// per version). It returns the COLLECTED per-version bodies (oldest → newest) for the
// 5-13 whole-file changelog rebuild, or the first per-version error (which task 5-12
// will turn into skip-and-continue). An empty version set is a clean no-op.
//
// The loop holds NO resume state: it reads no checkpoint and writes none, so a re-run
// simply re-processes every version from the top.
func RegenerateAll(ctx context.Context, deps ReleaseDeps, publisher publish.Publisher, req BatchRegenerateRequest) ([]RegeneratedVersion, error) {
	collected := make([]RegeneratedVersion, 0, len(req.Versions))
	for _, res := range req.Versions {
		processed, err := processOneVersion(ctx, deps, publisher, req, res)
		if err != nil {
			return nil, err
		}
		collected = append(collected, processed)
	}
	return collected, nil
}

// processOneVersion runs the per-version batch step for one resolved version: it opens
// the version's narration block (RunStarted + plan), produces the body, runs the
// per-version gate (unless -y), then dispatches the provider release via the 5-7
// create-or-update DISPATCH. It returns the version's collected body (for the 5-13
// rebuild) and any error.
//
// It is factored out as a single per-version unit so task 5-12 can wrap EACH iteration
// in skip-and-continue (collecting the returned error and continuing) without
// restructuring RegenerateAll. The per-version surface is the RELEASE only — no
// per-version changelog commit (that is the 5-13 end-of-batch whole-file write).
func processOneVersion(ctx context.Context, deps ReleaseDeps, publisher publish.Publisher, req BatchRegenerateRequest, res version.Resolution) (RegeneratedVersion, error) {
	p := deps.Presenter
	versionKey := batchVersionKey(res.Tag, req.TagPrefix)

	// Each version opens its OWN narration block: its own RunStarted then a plan. The
	// presenter renders blocks linearly in emit order, so emitting in loop order is
	// what makes the blocks read oldest → newest.
	p.RunStarted(presenter.RunInfo{
		Project: req.Project,
		Version: versionKey,
		Action:  regenerateAction,
	})
	p.ShowPlan(regeneratePlan(req.Source, RegenerateTargetRelease, res.Tag))

	// Produce the version's body for the resolved source BEFORE the gate — the
	// notes-review gate (fresh) reviews this body.
	body, err := req.ProduceBody(ctx, req.Source, res)
	if err != nil {
		return RegeneratedVersion{}, surface(p, "notes", err)
	}

	// Per-version gate BY DEFAULT (fresh → notes-review, reuse → simple confirm). -y
	// opts out: the engine does not even prompt, so the batch runs fully unattended.
	reviewed, err := gatePerVersion(ctx, deps, req, body, versionKey)
	if err != nil {
		return RegeneratedVersion{}, err
	}

	// Dispatch the RELEASE surface for this version: the 5-7 probe resolves
	// create-vs-update per version, so the batch transparently mixes updates and
	// creates. The changelog (whole-file rebuild + single end commit) is task 5-13.
	if err := DispatchRelease(ctx, publisher, res.Tag, res.Tag, reviewed); err != nil {
		return RegeneratedVersion{}, surface(p, "publish", err)
	}

	return RegeneratedVersion{Resolution: res, Body: reviewed}, nil
}

// gatePerVersion runs the source-appropriate per-version gate (fresh → the four-choice
// notes-review gate, reuse → the simple two-choice confirm) and returns the body to
// write. Under -y the gate is SKIPPED entirely — the engine does not prompt — and the
// produced body flows through unchanged so the batch runs fully unattended. This is the
// batch's -y contract: unlike the single-version path (which always prompts and lets the
// presenter skip), the batch suppresses the per-version gate so a large backfill does
// not emit N auto-accepted prompts.
func gatePerVersion(ctx context.Context, deps ReleaseDeps, req BatchRegenerateRequest, body, versionKey string) (string, error) {
	if req.Yes {
		return body, nil
	}
	return gateRegenerate(ctx, deps, RegenerateWriteRequest{
		Source:     req.Source,
		Tag:        "",
		VersionKey: versionKey,
		Body:       body,
	})
}

// batchVersionKey derives a version's bare x.y.z key from its canonical tag by
// stripping the configured prefix, REUSING the Phase 1 parser so the narration header
// and notes carry the same key the single-version path uses. A tag that fails to parse
// (it came from the matching set, so this is unreachable) falls back to the raw tag.
func batchVersionKey(tag, prefix string) string {
	v, err := version.ParseSemVer(tag, prefix)
	if err != nil {
		return tag
	}
	return v.String("")
}
