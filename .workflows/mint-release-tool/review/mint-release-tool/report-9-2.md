TASK: mint-release-tool-9-2 — Extract shared CHANGELOG stage-and-commit helper for regenerate paths

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. Helper at internal/engine/regenerate_write.go:284-292 stageAndCommitChangelog(ctx, m *git.Mutator, root, subject); single-version caller regenerate_changelog.go:69; batch caller regenerate_batch_changelog.go:147-153 (commitAndPushRebuild). Mutator param *git.Mutator matches both call sites. Helper short-circuits on stage failure (commit never runs) and tags the failed step ("staging %s" / "committing %q"). Single-version preserves tag-wrapping + (bool, error); batch routes the returned error through resetAndAbort(... startingHEAD, false, "record" ...) then calls pushChangelogCommit. Both subjects unchanged. No duplicated regenerate stage/commit pair remains (forward-release stage/commit is separate, out of scope).

TESTS:
- Status: Adequate. Single-version (regenerate_changelog_test.go): exact subject `docs(changelog): regenerate notes for v1.4.0`, exactly-2 invocations (add then commit), no-net-change → 0 invocations, at-most-one commit, never-cuts-tag, stage-fails-before-commit. Batch (regenerate_batch_changelog_test.go): batchRebuildSubject, single end-of-batch commit + plain push, byte-identical → no commit, pre-push failure → reset to startHEAD + "push" StageFailed + no StageSucceeded, stage-failure variant (add fails → "record" StageFailed) exercising the helper's returned error through resetAndAbort.

CODE QUALITY:
- Followed conventions (%w wrapping, named-return discipline, helper beside pushChangelogCommit, doc comment explains shared-idiom rationale). SOLID good — single-responsibility helper parameterised only by subject; callers retain distinct recovery. Low complexity.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
