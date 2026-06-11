TASK: mint-release-tool-5-13 — Batch --all whole-file CHANGELOG rebuild (one commit at end)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented (resolved the flagged ambiguity in favour of no-data-loss preserve, which the spec backs — line 495). internal/engine/regenerate_batch_changelog.go (orchestration), internal/record/rebuild.go (whole-file composer), wired from regenerate_batch.go:105-117 RegenerateAllValidated. Reuses record's renderSection/kacPreamble/splitAroundSection/writeAtomic (no second renderer) and shared single-version write idioms (resolveHEAD, readHistoricalDate, stageAndCommitChangelog, pushChangelogCommit, resetAndAbort) run once at batch end. Regenerated sections keep historical tag date; skipped-but-real preserved verbatim; strays dropped; no-op when byte-identical. Section-header matching line-anchored.

TESTS:
- Status: Adequate. regenerate_batch_changelog_test.go: whole-file newest-on-top rebuild w/ historical dates, stray-drop + order-repair, skipped-section preserve-verbatim, exactly-one-commit-at-end + subject `docs(changelog): regenerate release notes` + plain push, pre-push failure → reset + "push" StageFailed, stage(add) failure → "record" StageFailed + no commit/push, release-only → no changelog mutation, byte-identical → no commit/push. rebuild_test.go: compose-order, drop-stray, preserve-verbatim, byte-identical Changed=false, missing-preserved-section loud error.

CODE QUALITY:
- Followed conventions (small focused functions, doc comments name the why, error wrapping w/ domain noun, atomic write, two-kind union constructors). SOLID good — record owns composition, engine owns orchestration; ChangelogSection two-kind union keeps caller declarative. Low complexity, documented two-pass date read.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/record/rebuild.go:111-121 — composeChangelog concatenates each section's pre-terminated text directly, relying on every block carrying its own correct trailing newline(s). A preserved block whose source omitted a trailing blank line, if preserved in a non-last position, could leave the next header without a separating blank line. Decide whether to normalise preserved-block trailing whitespace to one blank line, or document that section spacing is the source file's responsibility. (Latent — only preserve path is for real sections record itself wrote.)
- [do-now] internal/engine/regenerate_batch_changelog.go:117 — readRegeneratedDates doc says "Skipped versions are not read"; add a one-line note at the batchChangelogSections call site that the preserved path needs no dates entry, to keep the two-collaborator contract self-evident at the call site too.
