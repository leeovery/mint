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
	"errors"
	"fmt"
	"strings"

	"mint/internal/notes"
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
	// Target selects which surface(s) the batch writes: a release/both target dispatches
	// the per-version provider release in the loop; a changelog/both target drives the
	// end-of-batch whole-file CHANGELOG rebuild. The zero value (RegenerateTargetRelease)
	// writes the provider only — the batch's historical default.
	Target RegenerateTarget
	// ProduceBody yields one version's notes body for the resolved source. It is
	// injected so the loop stays testable without a real AI transport / tag read;
	// production wires the 5-5 reuse read or the 5-6 fresh re-diff+AI per version.
	ProduceBody func(context.Context, RegenerateSource, version.Resolution) (string, error)
	// ProduceRegenerator yields the PER-VERSION regenerator the fresh notes-review
	// gate's `r` choice consults, bound to that version's fresh AI range. Production
	// returns nil for a reuse source (no review gate) and binds
	// notes.Generator.GenerateFromRangeWithContext for a fresh source. When nil (older
	// tests that never reach `r`) no per-version regenerator is supplied.
	ProduceRegenerator func(RegenerateSource, version.Resolution) Regenerator
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

// RegenerateAllValidated is the batch `--all` ENTRY POINT: it runs the static
// CONFIG-level check UP FRONT — before any version is touched — then runs the
// per-version loop, and finally (for a changelog/both target) performs the END-of-batch
// whole-file CHANGELOG rebuild + single commit + push. A config-level fact (a
// changelog/both target with the changelog DISABLED in config) is NOT a per-version
// condition, so it aborts the WHOLE batch immediately rather than being skipped per
// version; the abort surfaces through the presenter as a single stage failure and
// carries a non-zero exit.
//
// root is the repo root (the changelog lives at root/CHANGELOG.md). The target is read
// from req.Target: a release-only target makes NO changelog commit (the loop wrote the
// provider releases and there is nothing more to do); a changelog/both target drives
// the whole-file rebuild AFTER the loop so the file is regenerated from every real
// version's section in ONE commit + push at the end (not one per version).
//
// This is the batch counterpart of the cmd-layer validateTargetAgainstChangelog (task
// 5-2): the same static rule, enforced at the engine boundary so the batch aborts
// before it starts rather than emitting N per-version skips for one config mistake.
func RegenerateAllValidated(ctx context.Context, deps ReleaseDeps, publisher publish.Publisher, root string, req BatchRegenerateRequest, changelogEnabled bool) error {
	if err := checkBatchTargetConfig(req.Target, changelogEnabled); err != nil {
		return surface(deps.Presenter, "config", err)
	}
	collected, err := RegenerateAll(ctx, deps, publisher, req)
	if err != nil {
		return err
	}
	if !req.Target.writesChangelog() {
		return nil
	}
	return rebuildBatchChangelog(ctx, deps, root, req, collected)
}

// ErrChangelogDisabled is the single owned sentinel both changelog-disabled validators
// return: the engine's checkBatchTargetConfig (batch) and the cmd-layer
// validateTargetAgainstChangelog (single). The two validators stay SEPARATE functions
// — their concrete-enum signatures differ (cmd's regenerateTarget vs engine's
// RegenerateTarget) and each enforces independently — but they share this one pinned
// message so the wording can never drift across the single and batch paths.
var ErrChangelogDisabled = errors.New("changelog is disabled in config")

// checkBatchTargetConfig rejects a changelog-touching target when the changelog is
// disabled in config — the static config fact that aborts the whole batch up front.
// It mirrors the cmd-layer validateTargetAgainstChangelog so the rule is identical
// across the single and batch paths. A release-only target, or an enabled changelog,
// is a no-op.
func checkBatchTargetConfig(target RegenerateTarget, changelogEnabled bool) error {
	if !changelogEnabled && target.writesChangelog() {
		return ErrChangelogDisabled
	}
	return nil
}

// RegenerateAll runs the batch `--all` loop over the request's OLDEST → NEWEST version
// set: for each version it opens a narration block, produces the body, runs the
// per-version gate (unless -y), and dispatches the provider release (create-or-update
// per version). A per-version FAILURE is CAUGHT, RECORDED with a human reason, and the
// loop CONTINUES — consciously OVERRIDING the single-version on_notes_failure=abort
// default (a single huge release tripping max_diff_lines must not kill the others).
// The run closes with one RunFinished carrying the engine-computed end Summary (the
// regenerated count + each skipped version with its reason). It returns the COLLECTED
// per-version bodies (oldest → newest) for the 5-13 whole-file changelog rebuild, or a
// genuine abort (a gate DECLINE, which is a user choice, not a per-version failure).
// An empty version set is a clean no-op.
//
// The loop holds NO resume state: it reads no checkpoint and writes none, so a re-run
// simply re-processes every version from the top.
func RegenerateAll(ctx context.Context, deps ReleaseDeps, publisher publish.Publisher, req BatchRegenerateRequest) ([]RegeneratedVersion, error) {
	collected := make([]RegeneratedVersion, 0, len(req.Versions))
	var skipped []skippedVersion
	for _, res := range req.Versions {
		processed, skip, err := processOneVersion(ctx, deps, publisher, req, res)
		if err != nil {
			// A genuine abort (a gate DECLINE) is a user choice, not a per-version
			// failure — it propagates and ends the batch with no end summary.
			return nil, err
		}
		if skip != nil {
			skipped = append(skipped, *skip)
			continue
		}
		collected = append(collected, processed)
	}

	// Close the run with the engine-computed end summary: the regenerated count and
	// each skipped version with its reason, so the user can re-run the stragglers. The
	// URL is omitted ENTIRELY (regenerate publishes no release); the presenter renders
	// the Summary verbatim and never computes the version set.
	deps.Presenter.RunFinished(presenter.RunResult{
		Project: req.Project,
		Verb:    presenter.VerbRegenerate,
		Summary: batchSummary(len(collected), skipped),
	})
	return collected, nil
}

// skippedVersion records one version the batch skipped: its canonical tag (shown in
// the end summary so the user can re-run it) and the human reason it was skipped.
type skippedVersion struct {
	// Tag is the version's canonical tag (e.g. "v1.1.0"), the re-run key shown in the
	// end summary.
	Tag string
	// Reason is the human-readable skip cause (e.g. "diff too large").
	Reason string
}

// processOneVersion runs the per-version batch step for one resolved version: it opens
// the version's narration block (RunStarted + plan), produces the body, runs the
// per-version gate (unless -y), then dispatches the provider release via the 5-7
// create-or-update DISPATCH. It returns either the version's collected body (for the
// 5-13 rebuild) OR a *skippedVersion (a caught per-version failure the loop records
// and continues past) OR a genuine error (a gate DECLINE).
//
// Per-version FAILURES are caught here and returned as a *skippedVersion rather than
// surfaced as a terminal abort: this is the skip-and-continue contract that overrides
// the single-version on_notes_failure=abort default. The covered failures are:
//   - reuse against a tag with NO annotation body (lightweight / empty) — detected via
//     ReadTagBody's has-body branch (the --all variant of 5-5's single-mode fail-loud
//     read), never written as an empty provider release;
//   - a notes-production failure (e.g. a diff exceeding max_diff_lines) from ProduceBody.
//
// A gate DECLINE is NOT a failure — it is a user choice — so it propagates as an error
// and ends the batch (preserving the existing decline-aborts behavior).
func processOneVersion(ctx context.Context, deps ReleaseDeps, publisher publish.Publisher, req BatchRegenerateRequest, res version.Resolution) (RegeneratedVersion, *skippedVersion, error) {
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

	// reuse --all against a body-less tag is a per-version SKIP (not the single-mode
	// fail-loud error): read the annotation body up front and skip-and-report when the
	// tag carries none, so an empty provider release body is never written.
	if req.Source == RegenerateSourceReuse {
		_, hasBody, err := ReadTagBody(ctx, deps.Runner, res.Tag)
		if err != nil {
			return RegeneratedVersion{}, nil, surface(p, "notes", err)
		}
		if !hasBody {
			return reportSkip(p, res.Tag, reasonNoAnnotationBody)
		}
	}

	// Produce the version's body for the resolved source BEFORE the gate — the
	// notes-review gate (fresh) reviews this body. A notes-production failure (e.g. a
	// diff over max_diff_lines) is CAUGHT and skipped rather than aborting the batch.
	// Body production is BLOCKING (a fresh source re-diffs + calls the AI per version),
	// so narrate it with a blocking StageStarted (spinner) and a StageSucceeded
	// carrying the engine-measured Elapsed once the body resolves; a per-version skip
	// (production failure) closes the stage with no StageSucceeded — the skip Warn
	// narrates it instead.
	notesDone := emitBlockingStageStarted(p, "notes")
	body, err := req.ProduceBody(ctx, req.Source, res)
	if err != nil {
		return reportSkip(p, res.Tag, classifyNotesFailure(err))
	}
	notesDone()

	// Per-version gate BY DEFAULT (fresh → notes-review, reuse → simple confirm). -y
	// opts out: the engine does not even prompt, so the batch runs fully unattended. A
	// gate DECLINE is a user choice (not a failure), so it propagates and ends the batch.
	reviewed, err := gatePerVersion(ctx, deps, req, res, body, versionKey)
	if err != nil {
		return RegeneratedVersion{}, nil, err
	}

	// Dispatch the RELEASE surface for this version when the target writes the provider
	// (release/both): the 5-7 probe resolves create-vs-update per version, so the batch
	// transparently mixes updates and creates. A changelog-only target skips the provider
	// entirely — its surface is the end-of-batch whole-file CHANGELOG rebuild (task 5-13).
	if req.Target.writesProvider() {
		if err := DispatchRelease(ctx, publisher, res.Tag, res.Tag, reviewed); err != nil {
			return RegeneratedVersion{}, nil, surface(p, "publish", err)
		}
	}

	return RegeneratedVersion{Resolution: res, Body: reviewed}, nil, nil
}

// The human-readable skip reasons shown in the end summary. They are stable strings so
// the summary reads deterministically and the user can recognise re-run candidates.
const (
	// reasonNoAnnotationBody is the reuse --all skip reason for a tag with no
	// annotation body — the --all variant of single-mode's fail-loud "use --fresh".
	reasonNoAnnotationBody = "no annotation body — use --fresh"
	// reasonDiffTooLarge is the skip reason for a notes failure caused by a diff over
	// max_diff_lines (the distinguishable ErrDiffTooLarge case).
	reasonDiffTooLarge = "diff too large"
	// reasonNotesFailed is the fallback skip reason for any other notes-production
	// failure that is not the diff-too-large case.
	reasonNotesFailed = "notes generation failed"
)

// reportSkip narrates a per-version skip as a non-terminal Warn (which does NOT set
// failure state and does NOT suppress the end summary) and returns the *skippedVersion
// the loop records — so the version appears in the closing summary for re-run.
func reportSkip(p presenter.Presenter, tag, reason string) (RegeneratedVersion, *skippedVersion, error) {
	p.Warn(presenter.Warning{Label: "skipped " + tag, Message: reason})
	return RegeneratedVersion{}, &skippedVersion{Tag: tag, Reason: reason}, nil
}

// classifyNotesFailure maps a notes-production error to its human skip reason: the
// distinguishable diff-too-large case (matched with errors.Is) names itself; anything
// else falls back to the generic notes-failed reason.
func classifyNotesFailure(err error) string {
	if errors.Is(err, notes.ErrDiffTooLarge) {
		return reasonDiffTooLarge
	}
	return reasonNotesFailed
}

// batchSummary computes the end-of-run Summary the presenter renders VERBATIM: the
// regenerated count, then — only when versions were skipped — a "{N} skipped:" clause
// listing each skipped version as "tag (reason)" in batch (oldest → newest) order, so
// the user can re-run the stragglers. With nothing skipped it collapses to the bare
// "{N} regenerated".
func batchSummary(regenerated int, skipped []skippedVersion) string {
	if len(skipped) == 0 {
		return fmt.Sprintf("%d regenerated", regenerated)
	}
	parts := make([]string, len(skipped))
	for i, s := range skipped {
		parts[i] = fmt.Sprintf("%s (%s)", s.Tag, s.Reason)
	}
	return fmt.Sprintf("%d regenerated, %d skipped: %s", regenerated, len(skipped), strings.Join(parts, ", "))
}

// gatePerVersion runs the source-appropriate per-version gate (fresh → the four-choice
// notes-review gate, reuse → the simple two-choice confirm) and returns the body to
// write. Under -y the gate is SKIPPED entirely — the engine does not prompt — and the
// produced body flows through unchanged so the batch runs fully unattended. This is the
// batch's -y contract: unlike the single-version path (which always prompts and lets the
// presenter skip), the batch suppresses the per-version gate so a large backfill does
// not emit N auto-accepted prompts.
func gatePerVersion(ctx context.Context, deps ReleaseDeps, req BatchRegenerateRequest, res version.Resolution, body, versionKey string) (string, error) {
	if req.Yes {
		return body, nil
	}
	return gateRegenerate(ctx, deps, RegenerateWriteRequest{
		Source:      req.Source,
		Tag:         "",
		VersionKey:  versionKey,
		Body:        body,
		Regenerator: batchRegenerator(req.ProduceRegenerator, req.Source, res),
	})
}

// batchRegenerator invokes the batch request's per-version regenerator factory for the
// resolved source + version, tolerating a nil factory (older callers/tests that never
// reach the `r` choice) by returning nil. Production wires a factory that returns the
// fresh AI-range regenerator for a fresh source and nil for reuse.
func batchRegenerator(produce func(RegenerateSource, version.Resolution) Regenerator, source RegenerateSource, res version.Resolution) Regenerator {
	if produce == nil {
		return nil
	}
	return produce(source, res)
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
