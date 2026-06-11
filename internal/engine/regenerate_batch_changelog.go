package engine

// This file is the batch `--all` whole-file CHANGELOG rebuild + single end commit (task
// 5-13): after the per-version loop, an `--all` `--target changelog`/`both` run rebuilds
// CHANGELOG.md WHOLE — the KaC preamble + EVERY real matching version's section,
// newest-on-top — and makes EXACTLY ONE commit + plain push at the END (not one per
// version).
//
// COMPOSITION (the user-resolved no-data-loss rule). The real version set is the batch's
// matching versions (req.Versions, oldest → newest); the rebuild emits one section per
// real version, newest-on-top:
//   - REGENERATED this batch → render the FRESH body under the version's HISTORICAL date
//     (recovered from the tag via for-each-ref %(creatordate:short), exactly as the
//     single-version 5-9 path does — never today's date for a historical release).
//   - SKIPPED but real → PRESERVE the version's EXISTING section verbatim from the
//     current CHANGELOG.md (no data loss for a skipped real release).
//   - STRAY (a section in the file matching NO real version) → DROPPED (it simply has no
//     entry in the rebuilt set).
//
// It REUSES record's rendering via record.RebuildChangelog (no second renderer) and
// REUSES the 5-9 write idioms: the clean-HEAD capture, the historical-date read, the
// plain `git push origin HEAD` point of no return, and the reset-on-abort recovery —
// run ONCE at the end of the batch rather than per version.
//
// COMMIT SUBJECT is the `--all` form, `docs(changelog): regenerate release notes`
// (distinct from the single-version `docs(changelog): regenerate notes for {tag}`) —
// nothing is being released. NO-OP SAFETY: when the rebuilt file is byte-identical to
// the existing one, record.RebuildChangelog reports Changed=false and no commit is made.

import (
	"context"
	"fmt"
	"time"

	"mint/internal/record"
	"mint/internal/version"
)

// batchRebuildSubject is the `--all` whole-file rebuild commit subject — distinct from
// the single-version `docs(changelog): regenerate notes for {tag}` because the batch
// rebuilds the whole file rather than one version's section.
const batchRebuildSubject = "docs(changelog): regenerate release notes"

// rebuildBatchChangelog performs the end-of-batch whole-file CHANGELOG rebuild for a
// changelog/both `--all` run: it composes the file from every real version's section
// (regenerated bodies rendered under their historical date; skipped-but-real sections
// preserved verbatim; stray sections dropped), then — only when the rebuild nets a
// change — stages, commits (the `--all` subject), and plain-pushes it in ONE commit at
// the end, resetting on any pre-push failure. A byte-identical rebuild makes no commit.
//
// req carries the batch's real matching set OLDEST → NEWEST (req.Versions) and the tag
// prefix used to derive each section's bare x.y.z key; collected is the per-version
// bodies produced this batch (a subset — skipped versions are absent).
func rebuildBatchChangelog(ctx context.Context, deps ReleaseDeps, root string, req BatchRegenerateRequest, collected []RegeneratedVersion) error {
	p := deps.Presenter

	// Capture the clean starting HEAD before any mutation so a pre-push failure resets
	// to exactly here — the same clean-start capture the single-version 5-9 path uses.
	startingHEAD, err := resolveHEAD(ctx, deps.Runner)
	if err != nil {
		return surface(p, "record", err)
	}

	sections, err := batchChangelogSections(ctx, deps, req, collected)
	if err != nil {
		return surface(p, "record", err)
	}

	result, err := record.RebuildChangelog(root, sections)
	if err != nil {
		return surface(p, "record", err)
	}
	if !result.Changed {
		// Byte-identical rebuild → no commit. mint never makes an empty commit.
		return nil
	}

	return commitAndPushRebuild(ctx, deps, root, startingHEAD)
}

// batchChangelogSections builds the rebuild's section list NEWEST → ON → TOP (the order
// record.RebuildChangelog emits) from the real version set: a version regenerated this
// batch becomes a RenderedSection under its historical date; a skipped-but-real version
// becomes a PreservedSection (its existing block is copied verbatim).
//
// The historical dates are read in a FIRST pass over req.Versions in batch order (oldest
// → newest) so the per-tag reads happen in a stable, predictable order; the sections are
// then assembled by walking the versions in REVERSE (newest-on-top), reusing the dates.
func batchChangelogSections(ctx context.Context, deps ReleaseDeps, req BatchRegenerateRequest, collected []RegeneratedVersion) ([]record.ChangelogSection, error) {
	regenerated := regeneratedByTag(collected)

	dates, err := readRegeneratedDates(ctx, deps, req.Versions, regenerated)
	if err != nil {
		return nil, err
	}

	sections := make([]record.ChangelogSection, 0, len(req.Versions))
	for i := len(req.Versions) - 1; i >= 0; i-- {
		res := req.Versions[i]
		key := batchVersionKey(res.Tag, req.TagPrefix)
		body, ok := regenerated[res.Tag]
		if !ok {
			// Skipped but real: preserve its existing section verbatim (no data loss).
			// A preserved section needs no readRegeneratedDates entry — its date rides
			// the copied block, which is why that first pass reads regenerated tags only.
			sections = append(sections, record.PreservedSection(key))
			continue
		}
		// Regenerated: render the fresh body under the version's historical date.
		sections = append(sections, record.RenderedSection(key, dates[res.Tag], body))
	}
	return sections, nil
}

// readRegeneratedDates reads the historical date of every REGENERATED version in batch
// order (oldest → newest), keyed by canonical tag — the same for-each-ref creatordate
// read the single-version 5-9 path uses, so each regenerated section keeps its original
// release date. Skipped versions are not read (their dates ride their preserved sections).
func readRegeneratedDates(ctx context.Context, deps ReleaseDeps, versions []version.Resolution, regenerated map[string]string) (map[string]time.Time, error) {
	dates := make(map[string]time.Time, len(regenerated))
	for _, res := range versions {
		if _, ok := regenerated[res.Tag]; !ok {
			continue
		}
		date, err := readHistoricalDate(ctx, deps.Runner, res.Tag)
		if err != nil {
			return nil, err
		}
		dates[res.Tag] = date
	}
	return dates, nil
}

// regeneratedByTag indexes the collected bodies by canonical tag so the rebuild can ask,
// per real version, whether it was regenerated this batch (and with what body).
func regeneratedByTag(collected []RegeneratedVersion) map[string]string {
	byTag := make(map[string]string, len(collected))
	for _, rv := range collected {
		byTag[rv.Resolution.Tag] = rv.Body
	}
	return byTag
}

// commitAndPushRebuild stages, commits (the `--all` subject), and plain-pushes the
// rebuilt CHANGELOG.md in ONE commit at the end of the batch, resetting the local commit
// back to startingHEAD on any pre-push failure — the same recovery the single-version
// 5-9 path uses, run once for the whole batch. The push is the point of no return.
func commitAndPushRebuild(ctx context.Context, deps ReleaseDeps, root, startingHEAD string) error {
	if err := stageAndCommitChangelog(ctx, deps.Mutator, root, batchRebuildSubject); err != nil {
		return resetAndAbort(ctx, deps, startingHEAD, false, "record", fmt.Errorf("batch rebuild: %w", err))
	}
	// The rebuild commit landed: push it (the end-of-batch point of no return) via the
	// SHARED regenerate push tail — the same plain-push form, blocking "push" narration,
	// and reset-on-abort recovery the single-version path uses, run once for the batch.
	return pushChangelogCommit(ctx, deps, startingHEAD)
}
