package engine

// This file is the regenerate batch `--all` LOOP: the orchestration that
// backfills EVERY matching version in one run. The version set is enumerated OLDEST →
// NEWEST upstream (version.ResolveAllRegenerateTargets), and this loop processes each
// in turn. The backfill is ONE run (RegenerateAllRun / RegenerateAllValidated open and
// close it), drawn on the rail as a node per version, oldest first; plain keeps its
// narration block per version (each with its OWN plain RunStarted, emitted in order,
// so block ordering is ENGINE-owned) and its end summary, sent Ahead of the run's own
// close. For each version the loop produces the notes body (reuse read or fresh
// re-diff+AI, injected as ProduceBody), runs a per-version review gate BY DEFAULT
// (-y opts out to run fully unattended), and dispatches the RELEASE surface via the
// create-or-update DISPATCH — so the batch transparently mixes updates and creates
// per version.
//
// SCOPE: this file owns the RELEASE surface dispatch + per-version notes only. It does
// NO per-version changelog commit — in `--all` mode the changelog is a whole-file
// rebuild + single END commit (rebuildBatchChangelog, regenerate_batch_changelog.go), so
// the loop COLLECTS each version's produced body (oldest → newest) and returns the
// collection for that rebuild. Each version runs through processOneVersion, which
// catches a per-version failure and returns it as a skip the loop records before moving
// on to the next version: skip-and-continue is the batch's floor.
//
// NO RESUME STATE: the loop reads no checkpoint and writes none — a re-run simply
// re-processes from the top. `--source tag --all` is deterministic; `--source fresh --all`
// re-generates (stochastic but harmless).

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"mint/internal/notes"
	"mint/internal/presenter"
	"mint/internal/publish"
	"mint/internal/record"
	"mint/internal/runclose"
	"mint/internal/version"
)

// BatchRegenerateRequest carries the resolved inputs for one `--all` batch run: the
// chosen source, the OLDEST → NEWEST version set, the project + prefix for narration,
// the -y opt-out, and the injected per-version body producer.
type BatchRegenerateRequest struct {
	// Source selects the per-version gating + production: fresh runs the re-diff+AI
	// producer and the notes-review gate; the deterministic sources read existing notes
	// (reuse → tag annotation, release → published provider release body) and run the
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
	// ReleaseBranch is the resolved release branch the preflight on-branch and
	// remote-sync gates check, threaded from the cmd layer's read-only branch
	// resolution. It backs the batch preflight that runs inside the run AFTER the target
	// resolves at its axis prompts — the cmd layer cannot run that gate set itself
	// because it dispatches before the interactive batch target is known, exactly as the
	// single-version RegenerateRunRequest.ReleaseBranch.
	ReleaseBranch string
	// ProduceBody yields one version's notes body for the resolved source. It is
	// injected so the loop stays testable without a real AI transport / tag / release
	// read; production wires the reuse read, the provider-release read, or the fresh
	// re-diff+AI per version. preReadBody threads the loop's pre-read body (captured by
	// the deterministic-source skip check — tag annotation or release body) into the
	// producer so a deterministic source consumes it instead of reading the SECOND time;
	// it is empty for a fresh source and for the single-version delegation (which never
	// pre-reads, so the producer reads). A fresh producer reports a chunked run to the
	// sinks of the version's notes stage it runs inside.
	ProduceBody func(ctx context.Context, source RegenerateSource, res version.Resolution, preReadBody string, sinks NotesSinks) (string, error)
	// ProduceRegenerator yields the PER-VERSION regenerator the fresh notes-review
	// gate's `r` choice consults, handed run, whose Merged reads the merge inputs that
	// version's chunked run kept (nil unless a chunked run wrote its body). Production
	// returns nil for a reuse source (no review gate) and binds RegenerateFreshRegenerator
	// over run and that version's fresh AI range for a fresh source. When nil (older
	// tests that never reach `r`) no per-version regenerator is supplied.
	ProduceRegenerator func(source RegenerateSource, res version.Resolution, run RegenRun) Regenerator
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

// RegenerateAllValidated runs a --all backfill whose source and target are already
// resolved, as RegenerateAllRun runs one once its questions are answered: it runs the
// static CONFIG-level check UP FRONT — before any version is touched — then runs the
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
//
// The backfill is one run, opened here at its start station and closed here once.
func RegenerateAllValidated(ctx context.Context, deps ReleaseDeps, publisher publish.Publisher, root string, req BatchRegenerateRequest, changelogEnabled bool) error {
	deps.run = runclose.Begin(deps.Presenter, deps.clock())
	err := regenerateAllValidated(ctx, deps, publisher, root, req, changelogEnabled)
	deps.run.Close(declined(err))
	return err
}

// RegenerateAllRunRequest carries a --all backfill's inputs ahead of its questions:
// the source and target given or to ask, whether the changelog is on, the project,
// and Prepare, which resolves, once both are answered, the batch to run and the
// provider driver that writes its releases (nil where none resolved).
type RegenerateAllRunRequest struct {
	Source           OptionalRegenerateSource
	Target           OptionalRegenerateTarget
	ChangelogEnabled bool
	Project          string
	Prepare          func(ctx context.Context, source RegenerateSource, target RegenerateTarget) (BatchRegenerateRequest, publish.Publisher, error)
}

// RegenerateAllRun runs a --all backfill from its questions: one run, opened with its
// header ahead of Source? and Target?, which are asked as RegenerateRun asks them,
// then prepared and run as RegenerateAllValidated runs it, and closed once.
func RegenerateAllRun(ctx context.Context, deps ReleaseDeps, root string, req RegenerateAllRunRequest) error {
	deps.run = runclose.Begin(deps.Presenter, deps.clock())
	err := regenerateAllRun(ctx, deps, root, req)
	deps.run.Close(declined(err))
	return err
}

// regenerateAllRun is RegenerateAllRun's run; RegenerateAllRun closes it.
func regenerateAllRun(ctx context.Context, deps ReleaseDeps, root string, req RegenerateAllRunRequest) error {
	deps.run.Open(presenter.RunInfo{Project: req.Project, Header: deps.header(regenerateAllCommand, root)})
	source, target, err := ResolveRegenerateAxes(deps.Presenter, req.Source, req.Target, req.ChangelogEnabled)
	if err != nil {
		return err
	}
	batch, publisher, err := req.Prepare(ctx, source, target)
	if err != nil {
		return err
	}
	return regenerateAllValidated(ctx, deps, publisher, root, batch, req.ChangelogEnabled)
}

// regenerateAllValidated is RegenerateAllValidated's run within the run deps holds:
// its start station, its checks, the plan of a node per version, the loop, then the
// changelog rebuild, landing at "done" with what the loop regenerated and skipped.
func regenerateAllValidated(ctx context.Context, deps ReleaseDeps, publisher publish.Publisher, root string, req BatchRegenerateRequest, changelogEnabled bool) error {
	release := releaseName(publisher)
	deps.run.Start(presenter.RunInfo{
		Project: req.Project,
		Header:  deps.header(regenerateAllCommand, root),
		Start:   batchStart(req, release),
		Toward:  batchDone,
		Quiet:   true,
	})
	if err := checkBatchTargetConfig(req.Target, changelogEnabled); err != nil {
		return surface(deps.Presenter, "config", err)
	}
	// The provider-release source reads every version's existing release through the
	// Publisher seam, so it is impossible without a resolved provider. A downgraded batch
	// (non-github / no-remote origin → nil publisher) using this source aborts the WHOLE
	// run up front — before any version — rather than nil-dereferencing per version. This
	// is a static precondition like the changelog-disabled check above, not a per-version
	// skip.
	if req.Source == RegenerateSourceRelease && publisher == nil {
		return surface(deps.Presenter, "publish", errReleaseSourceNoPublisher)
	}
	// Preflight the RESOLVED batch target BEFORE any version is processed, any provider
	// write, or any changelog commit/push. A bare `--all` resolves its target
	// INTERACTIVELY at RegenerateAllRun's axis prompts, AFTER the cmd layer could run
	// preflight, so the gate set is derived from the resolved req.Target HERE —
	// mirroring the single-version RegenerateRun: an interactive changelog/both runs the
	// clean-tree + on-branch + remote-sync bucket before committing+pushing, and an
	// interactive release/both runs gh-auth before the first provider dispatch. The batch
	// resolves ONE target — and ONE publisher — for every version, so gating on req.Target
	// and the batch-level publisher IS the conservative superset of what will be written.
	// The gh-auth gate also requires a RESOLVED publisher (publisher != nil): a downgraded
	// batch (the provider could not be resolved upstream, so a nil publisher is threaded
	// and every per-version provider write is nil-guarded and skipped) does NOT run
	// gh-auth, mirroring the forward spine's `if publisher != nil` guard. A failing
	// applicable gate routes through the same surfaced abort as every other failure,
	// before the loop starts.
	if err := RegeneratePreflight(ctx, deps, req.ReleaseBranch, regenerateGateSet(req.Target, publisher != nil)); err != nil {
		return err
	}
	deps.Presenter.ShowPlan(batchPlan(req, release))
	collected, err := RegenerateAll(ctx, deps, publisher, root, req)
	if err != nil {
		return err
	}
	// With no version in the set there is nothing to rebuild from: a rebuild would cut
	// CHANGELOG.md down to its bare preamble (a tag_prefix that matches no tag gets
	// here), then commit and push that, so the file is left as it is.
	if req.Target.writesChangelog() && len(req.Versions) > 0 {
		if err := rebuildBatchChangelog(ctx, deps, root, req, collected); err != nil {
			return err
		}
	}
	deps.run.Land(presenter.RunResult{
		Project: req.Project,
		Outcome: presenter.OutcomeAllRegenerated,
		End:     presenter.Station{Name: batchDone, Detail: batchDoneDetail(len(req.Versions), len(collected))},
		Quiet:   true,
	})
	return nil
}

// batchDone is the name of the station a --all backfill lands at.
const batchDone = "done"

// batchStart is the station a --all backfill starts from: how many versions, and
// where their notes come from and go, release naming the provider's.
func batchStart(req BatchRegenerateRequest, release string) presenter.Station {
	return presenter.Station{
		Name:   countOf(len(req.Versions), "version"),
		Detail: batchSourceLabel(req.Source) + " → " + batchTargetLabel(req.Target, release),
	}
}

// batchDoneDetail is what a --all backfill of total versions that regenerated some
// did: how many of them, then how many it skipped, where it skipped any; or, with no
// version to regenerate, that it had none.
func batchDoneDetail(total, regenerated int) string {
	if total == 0 {
		return "nothing to regenerate"
	}
	detail := fmt.Sprintf("%d of %s regenerated", regenerated, countOf(total, "version"))
	if skipped := total - regenerated; skipped > 0 {
		detail += fmt.Sprintf(" · %d skipped", skipped)
	}
	return detail
}

// countOf is n things, the noun plural but for one, and none as no things.
func countOf(n int, noun string) string {
	switch n {
	case 0:
		return "no " + noun + "s"
	case 1:
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// batchSourceLabel is where a --all backfill's notes come from, for its station.
func batchSourceLabel(source RegenerateSource) string {
	switch source {
	case RegenerateSourceTag:
		return "the tags' notes"
	case RegenerateSourceRelease:
		return "the releases' notes"
	default:
		return "fresh notes"
	}
}

// batchTargetLabel is where a --all backfill's notes go, for its station: each
// version's release, then the one changelog rebuilt.
func batchTargetLabel(target RegenerateTarget, release string) string {
	switch target {
	case RegenerateTargetChangelog:
		return record.ChangelogFileName
	case RegenerateTargetBoth:
		return release + "s, then " + record.ChangelogFileName
	default:
		return release + "s"
	}
}

// batchPlan is a --all backfill's stages, Quiet as plain's plan comes per version: a
// node per version, oldest first, keyed by its tag, then the changelog rebuilt where
// the target writes it.
func batchPlan(req BatchRegenerateRequest, release string) presenter.Plan {
	detail := versionWrites(req.Target, release)
	stages := make([]presenter.PlanStage, 0, len(req.Versions)+1)
	for _, res := range req.Versions {
		stages = append(stages, presenter.PlanStage{Key: res.Tag, Name: res.Tag, Detail: detail})
	}
	if req.Target.writesChangelog() && len(req.Versions) > 0 {
		stages = append(stages, presenter.PlanStage{Key: stageRecord, Name: record.ChangelogFileName, Detail: "rebuilt, committed and pushed"})
	}
	return presenter.Plan{Stages: stages, Quiet: true}
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
// set: for each version it opens plain's narration block, produces the body, runs the
// per-version gate (unless -y), and dispatches the provider release (create-or-update
// per version), all on the version's own node. A per-version FAILURE is CAUGHT,
// RECORDED with a human reason, and the loop CONTINUES, so one version whose notes
// fail does not stop the others. As the loop ends, plain's closing line goes Ahead,
// carrying the engine-computed end Summary (the regenerated count + each skipped
// version with its reason). It returns the COLLECTED per-version bodies (oldest →
// newest) for the 5-13 whole-file changelog rebuild, or a genuine abort (e.g. a gate
// DECLINE or a cancel) that ends the batch with no end summary.
// Once any version's notes were written after halving, one lower-the-cap hint prints as
// the batch ends, however it ends, and no version prints its own.
// An empty version set is a clean no-op.
//
// The loop holds NO resume state: it reads no checkpoint and writes none, so a re-run
// simply re-processes every version from the top.
func RegenerateAll(ctx context.Context, deps ReleaseDeps, publisher publish.Publisher, root string, req BatchRegenerateRequest) ([]RegeneratedVersion, error) {
	collected := make([]RegeneratedVersion, 0, len(req.Versions))
	var skipped []skippedVersion
	batch := batchRun{release: releaseName(publisher)}
	for _, res := range req.Versions {
		processed, skip, err := processOneVersion(ctx, deps, publisher, root, req, res, &batch)
		if err != nil {
			hintLowerCap(deps.Presenter, batch.hint.lowerCap())
			return nil, err
		}
		if skip != nil {
			skipped = append(skipped, *skip)
			continue
		}
		collected = append(collected, processed)
	}

	// Plain's close, the engine-computed end summary: the regenerated count and each
	// skipped version with its reason, so the user can re-run the stragglers. The URL
	// is omitted ENTIRELY (regenerate publishes no release); the presenter renders the
	// Summary verbatim and never computes the version set.
	hintLowerCap(deps.Presenter, batch.hint.lowerCap())
	deps.Presenter.RunFinished(presenter.RunResult{
		Project: req.Project,
		Outcome: presenter.OutcomeAllRegenerated,
		Summary: batchSummary(len(collected), skipped),
		Ahead:   true,
	})
	return collected, nil
}

// batchRun is what a --all loop carries from one version to the next: the lower-cap
// hint it gathers, the name of the release each version writes, and how many it
// has written.
type batchRun struct {
	hint    batchLowerCap
	release string
	written int
}

// wrote counts one more release written and tells the run's close: no reset takes
// those back, and git is still as the run found it.
func (b *batchRun) wrote(deps ReleaseDeps) {
	b.written++
	deps.run.Wrote(countOf(b.written, b.release) + " written")
	deps.run.Left(noClaim)
}

// skippedVersion records one version the batch skipped: its canonical tag (shown in
// the end summary so the user can re-run it) and the human reason it was skipped.
type skippedVersion struct {
	// Tag is the version's canonical tag (e.g. "v1.1.0"), the re-run key shown in the
	// end summary.
	Tag string
	// Reason is the human-readable skip cause (e.g. "AI call failed (part 3 of 6)").
	Reason string
	// SavedNotes points at the version's saved chunk notes: the path, then the
	// version's own regenerate that puts them on it. Empty when its notes failed with
	// none saved, so the skip names no file. saved is the path alone, "" for none.
	SavedNotes string
	saved      string
	// bodiless is a skip for a deterministic source with no notes to read, which its
	// reason says fresh notes cure.
	bodiless bool
}

// cause is the skip's text in its warning and in the end summary.
func (s skippedVersion) cause() string {
	if s.SavedNotes == "" {
		return s.Reason
	}
	return s.Reason + "; " + s.SavedNotes
}

// savedNotesPointer names the chunk notes failure saved and the regenerate of version
// to target that puts them on it, or "" when failure saved none.
func savedNotesPointer(failure error, version string, target RegenerateTarget) string {
	saved := notes.SavedChunkNotesPath(failure)
	if saved == "" {
		return ""
	}
	return "part notes saved to " + saved + " — tidy them and put them on it with " + notesFileCommand(regenerateRerun(version, target).command, saved)
}

// processOneVersion runs the per-version batch step for one resolved version: it opens
// plain's narration block for the version (RunStarted + plan) and runs the version's
// node, on which it produces the body, runs the per-version gate (unless -y), then
// dispatches the provider release via the 5-7 create-or-update DISPATCH, the node
// settling with what the version wrote. It returns either the version's collected body (for the
// 5-13 rebuild) OR a *skippedVersion (a caught per-version failure the loop records
// and continues past) OR a genuine error that ends the batch (e.g. a gate DECLINE or
// a cancel).
//
// Per-version FAILURES are caught here and returned as a *skippedVersion rather than
// surfaced as a terminal abort: skip-and-continue is the batch's floor. The covered failures are:
//   - reuse against a tag with NO annotation body (lightweight / empty) — detected via
//     ReadTagBody's has-body branch (the --all variant of 5-5's single-mode fail-loud
//     read), never written as an empty provider release;
//   - a notes-production failure (e.g. an AI call that fails) from ProduceBody —
//     EXCEPT a cancel (context.Canceled, bare or wrapped), which is not a skip.
//
// A gate DECLINE is NOT a failure — it is a user choice — so it propagates as an error
// and ends the batch. A cancel (a Ctrl-C) is not a failure either and ends the batch the
// same way, through a failed `notes` stage.
func processOneVersion(ctx context.Context, deps ReleaseDeps, publisher publish.Publisher, root string, req BatchRegenerateRequest, res version.Resolution, batch *batchRun) (RegeneratedVersion, *skippedVersion, error) {
	p := deps.Presenter
	versionKey := batchVersionKey(res.Tag, req.TagPrefix)

	// Plain opens a block per version: its own RunStarted then a plan, emitted in loop
	// order so the blocks read oldest → newest. The run's station and the version's
	// node are drawn already, so on the rail this start and this plan of steps alone
	// draw nothing; the version's node runs from here.
	p.RunStarted(presenter.RunInfo{
		Project: req.Project,
		Version: versionKey,
		Action:  regenerateAction,
		Header:  deps.header(regenerateAllCommand, root),
	})
	p.ShowPlan(presenter.Plan{Steps: regenerateSteps(req.Source, req.Target, res.Tag)})
	node := startQuietStep(p, deps.clock(), stepOf{name: res.Tag, key: res.Tag}, readingText(req.Source, batch.release))

	// A deterministic source against a body-less version is a per-version SKIP (not the
	// single-mode fail-loud error): read the existing body up front — the tag annotation
	// (reuse) or the published release (release) — and skip-and-report when none is
	// present, so an empty body is never written. The body this check read is threaded
	// into ProduceBody below so the producer consumes it rather than reading the source a
	// second time — ONE read per version.
	preReadBody, skipReason, err := preReadDeterministicBody(ctx, deps, publisher, req.Source, res.Tag)
	if err != nil {
		return RegeneratedVersion{}, nil, failVersion(p, node, stageNotes, err)
	}
	if skipReason != "" {
		return reportSkip(p, req, node, skippedVersion{Tag: res.Tag, Reason: skipReason, bodiless: true}, "")
	}

	// Produce the version's body for the resolved source BEFORE the gate — the
	// notes-review gate (fresh) reviews this body. A notes-production failure (e.g. a
	// failed AI call) is CAUGHT and skipped rather than aborting the batch; a cancel is
	// not a failure and ends the batch.
	// Body production is BLOCKING (a fresh source re-diffs + calls the AI per version),
	// so narrate it with a blocking StageStarted (spinner), a chunked run's progress
	// while it runs, and a StageSucceeded carrying the engine-measured Elapsed once the
	// body resolves; a per-version skip (production failure) closes the stage with no
	// StageSucceeded — the skip Warn narrates it instead. On the rail the notes are a
	// step of the version's node, which goes on past them.
	stage := startNotesStep(p, deps.clock(), stepOf{name: stageNotes, key: res.Tag, ongoing: true})
	chunkNotes := newChunkNotesKeeper(deps.ChunkNotes, root, res.Tag)
	body, err := req.ProduceBody(ctx, req.Source, res, preReadBody, chunkNotes.sinks(stage))
	if err != nil {
		batch.hint.tried(stage)
		if errors.Is(err, context.Canceled) {
			return RegeneratedVersion{}, nil, failVersion(p, node, stageNotes, err)
		}
		reason, output := notesSkipCause(err)
		skip := skippedVersion{Tag: res.Tag, Reason: reason, SavedNotes: savedNotesPointer(err, versionKey, req.Target), saved: notes.SavedChunkNotesPath(err)}
		processed, recorded, skipErr := reportSkip(p, req, node, skip, output)
		chunkNotes.warnUnsaved(p)
		return processed, recorded, skipErr
	}
	stage.read(readFrom(req.Source, batch.release, ""))
	batch.hint.written(stage)

	// Per-version gate BY DEFAULT (fresh → notes-review, reuse → simple confirm). -y
	// opts out: the engine does not even prompt, so the batch runs fully unattended. A
	// gate DECLINE is a user choice (not a failure), so it propagates and ends the batch.
	// An `r` prints no hint of its own: its calls and halving fold into the batch's. The
	// version's node goes on past the gate, its time leaving out the waits on its answers
	// and the editor: an `r`'s new notes are mint's work and count.
	regens := stage.regens(batch.hint.regenWritten)
	reviewed, err := gatePerVersion(ctx, node.awaiting(deps), req, res, body, versionKey, chunkNotes, regens)
	batch.hint.tried(stage)
	if err != nil {
		return RegeneratedVersion{}, nil, err
	}

	if err := writeVersionRelease(ctx, deps, publisher, req, res, reviewed, node, batch); err != nil {
		return RegeneratedVersion{}, nil, err
	}
	return RegeneratedVersion{Resolution: res, Body: reviewed}, nil, nil
}

// writeVersionRelease dispatches the RELEASE surface for a version whose notes are
// body when the target writes the provider (release/both), then settles the version's
// node with what it wrote: the 5-7 probe resolves create-vs-update per version, so the
// batch transparently mixes updates and creates. A changelog-only target skips the
// provider entirely — its surface is the end-of-batch whole-file CHANGELOG rebuild
// (task 5-13).
//
// NIL-GUARD: a downgraded batch (the provider could not be resolved — a non-github /
// no-remote origin) carries a nil publisher for every version. Skip the provider
// dispatch with a warn, which ends the node, rather than dereferencing nil in
// DispatchRelease — mirroring the forward path's tag + push only downgrade. This is NOT
// a per-version FAILURE (the version is still collected for the changelog rebuild and
// counted as regenerated): the WHOLE run is downgraded, the same way the single-version
// path skips its one provider write.
func writeVersionRelease(ctx context.Context, deps ReleaseDeps, publisher publish.Publisher, req BatchRegenerateRequest, res version.Resolution, body string, node *blockingStage, batch *batchRun) error {
	p := deps.Presenter
	if req.Target.writesProvider() {
		if publisher == nil {
			warnRegenerateProviderSkipped(p, res.Tag)
			return nil
		}
		p.StageStarted(presenter.StageStart{Name: stagePublish, Key: res.Tag, Blocking: true, Text: "writing the " + batch.release + "…", Quiet: true})
		// The batch's closing Summary carries no URL (regenerate publishes a set, not a
		// single release with a footer URL), so the dispatched release URL is discarded
		// here — only the forward release path threads it into RunResult.URL.
		if _, err := DispatchRelease(ctx, publisher, res.Tag, res.Tag, body); err != nil {
			return failVersion(p, node, stagePublish, err)
		}
		batch.wrote(deps)
	}
	node.succeeded(presenter.StageSuccess{Detail: versionDetail(req, res, batch.release)})
	return nil
}

// failVersion fails the version's node on cause, plain's line naming stage, the step
// it failed at, and ends the batch.
func failVersion(p presenter.Presenter, node *blockingStage, stage string, cause error) error {
	failure := failedStage(stage, cause)
	failure.Key, failure.Elapsed = node.step.key, node.elapsed()
	p.StageFailed(failure)
	return abort(cause)
}

// readingText is what a version's node does first: reading the notes a deterministic
// source names, or generating fresh ones.
func readingText(source RegenerateSource, release string) string {
	switch source {
	case RegenerateSourceTag:
		return "reading the tag's notes…"
	case RegenerateSourceRelease:
		return "reading the " + release + "…"
	default:
		return notesStageText
	}
}

// versionDetail is what a version's node settles with: what it wrote, and where the
// first release's notes are fresh, that they are its default.
func versionDetail(req BatchRegenerateRequest, res version.Resolution, release string) string {
	detail := versionWrites(req.Target, release)
	if res.FirstRelease && req.Source == RegenerateSourceFresh {
		detail += " · " + notesSource(notes.KindFirstRelease, "")
	}
	return detail
}

// versionWrites is what each version of a --all backfill writing target writes: its
// release, or, where the target writes none, its notes for the changelog rebuilt.
func versionWrites(target RegenerateTarget, release string) string {
	if !target.writesProvider() {
		return "notes for " + record.ChangelogFileName
	}
	return release
}

// The human-readable skip reasons shown in the end summary. They are stable strings so
// the summary reads deterministically and the user can recognise re-run candidates.
const (
	// reasonNoAnnotationBody is the `--source tag --all` skip reason for a tag with no
	// annotation body — the --all variant of single-mode's fail-loud "use --source fresh".
	reasonNoAnnotationBody = "no annotation body — use --source fresh"
	// reasonNoReleaseBody is the `--source release --all` skip reason for a version whose
	// published release has no body — the variant of reasonNoAnnotationBody.
	reasonNoReleaseBody = "no release body — use --source fresh"
	// reasonNotesFailed is the skip reason for a notes-production failure that is not
	// the generation's own.
	reasonNotesFailed = "notes generation failed"
)

// notesSkipCause is the reason a version whose notes failed with failure is skipped,
// and the failed call's captured output to render beneath it: the ✗ line's cause
// phrase and output for a generation failure, the generic reason and none otherwise.
func notesSkipCause(failure error) (reason, output string) {
	if _, known := notes.ClassifyFailure(failure); !known {
		return reasonNotesFailed, ""
	}
	phrase, _ := notes.CauseText(failure)
	return phrase, notesFailureOutput(failure)
}

// preReadDeterministicBody pre-reads a DETERMINISTIC source's body for the batch skip
// check, returning the body to thread into ProduceBody (so each version's source is read
// ONCE). A body-less source returns a non-empty skipReason (the loop reports the skip and
// continues): reuse → the tag annotation (ReadTagBody), release → the published provider
// release (Publisher.ReadReleaseBody). A fresh source has no pre-read and returns
// ("", "", nil). A genuine read failure is returned as err for the caller to surface.
//
// The release branch dereferences publisher; RegenerateAllValidated guarantees a non-nil
// publisher for the release source up front (the downgrade precondition), so it is always
// safe here.
func preReadDeterministicBody(ctx context.Context, deps ReleaseDeps, publisher publish.Publisher, source RegenerateSource, tag string) (body, skipReason string, err error) {
	switch source {
	case RegenerateSourceTag:
		b, hasBody, err := ReadTagBody(ctx, deps.Runner, tag)
		if err != nil {
			return "", "", err
		}
		if !hasBody {
			return "", reasonNoAnnotationBody, nil
		}
		return b, "", nil
	case RegenerateSourceRelease:
		b, hasBody, err := publisher.ReadReleaseBody(ctx, tag)
		if err != nil {
			return "", "", err
		}
		if !hasBody {
			return "", reasonNoReleaseBody, nil
		}
		return b, "", nil
	default:
		return "", "", nil
	}
}

// reportSkip narrates a per-version skip as a non-terminal Warn (which does NOT set
// failure state and does NOT suppress the end summary) that ends the version's node in
// its time, its cause naming any saved chunk notes, output verbatim beneath it, and the
// way to try it again for the rail's next; it returns the *skippedVersion the loop
// records — so the version appears in the closing summary for re-run.
func reportSkip(p presenter.Presenter, req BatchRegenerateRequest, node *blockingStage, skip skippedVersion, output string) (RegeneratedVersion, *skippedVersion, error) {
	p.Warn(presenter.Warning{
		Key:     skip.Tag,
		Label:   "skipped " + skip.Tag,
		Message: skip.cause(),
		Row:     "skipped · " + skip.Reason,
		Elapsed: node.elapsed(),
		Output:  output,
	})
	p.NextMoves(presenter.NextMoves{Stage: skip.Tag, Quiet: true, Moves: []presenter.NextMove{skip.move(req.Source, req.Target)}})
	return RegeneratedVersion{}, &skip, nil
}

// move is the way to regenerate the skipped version again, from source to target as
// the batch did: from the part notes it saved, from fresh notes where its source had
// none, or the same again.
func (s skippedVersion) move(source RegenerateSource, target RegenerateTarget) presenter.NextMove {
	switch {
	case s.saved != "":
		return notesFileMove(regenerateRerun(s.Tag, target), s.saved)
	case s.bodiless:
		return presenter.NextMove{Lead: s.Tag + " kept its old notes. To write them fresh:", Command: regenerateMoveCommand(s.Tag, RegenerateSourceFresh, target)}
	}
	return presenter.NextMove{Lead: s.Tag + " kept its old notes. To try it again:", Command: regenerateMoveCommand(s.Tag, source, target)}
}

// batchSummary computes the end-of-run Summary the presenter renders VERBATIM: the
// regenerated count, then — only when versions were skipped — a "{N} skipped:" clause
// listing each skipped version as "tag (cause)" in batch (oldest → newest) order, the
// cause naming any saved chunk notes and their regenerate, so the user can re-run the
// stragglers. With nothing skipped it collapses to the bare
// "{N} regenerated".
func batchSummary(regenerated int, skipped []skippedVersion) string {
	if len(skipped) == 0 {
		return fmt.Sprintf("%d regenerated", regenerated)
	}
	parts := make([]string, len(skipped))
	for i, s := range skipped {
		parts[i] = fmt.Sprintf("%s (%s)", s.Tag, s.cause())
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
func gatePerVersion(ctx context.Context, deps ReleaseDeps, req BatchRegenerateRequest, res version.Resolution, body, versionKey string, chunkNotes *chunkNotesKeeper, regens *regenStage) (string, error) {
	if req.Yes {
		return body, nil
	}
	return gateRegenerate(ctx, deps, RegenerateWriteRequest{
		Source: req.Source,
		// Target is inert here today — gateRegenerate keys off Source only — but is
		// set so the inner request faithfully mirrors the batch's resolved target.
		Target:      req.Target,
		Tag:         "",
		VersionKey:  versionKey,
		Body:        body,
		Regenerator: batchRegenerator(req.ProduceRegenerator, req.Source, res, chunkNotes.regenRun(regens)),
		regens:      regens,
		chunkNotes:  chunkNotes,
		node:        res.Tag,
	})
}

func batchRegenerator(produce func(RegenerateSource, version.Resolution, RegenRun) Regenerator, source RegenerateSource, res version.Resolution, run RegenRun) Regenerator {
	if produce == nil {
		return nil
	}
	return produce(source, res, run)
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
