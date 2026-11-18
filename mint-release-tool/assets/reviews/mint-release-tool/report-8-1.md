TASK: mint-release-tool-8-1 — Extract shared changelog push/recovery tail for the single-version and batch regenerate paths

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented (clean extraction). Helper pushChangelogCommit at internal/engine/regenerate_write.go:306-313; single-version call site :267-270; batch call site regenerate_batch_changelog.go:153 (via commitAndPushRebuild). The `git push origin HEAD` literal, emitBlockingStageStarted(...,"push")/StageSucceeded narration, "pushing regenerated changelog: %w" wrap, and PONR resetAndAbort now live in exactly one function. Both paths reach the helper only after a commit landed (committed=true unconditionally). Commit subjects stay at call sites; single-version !committed no-op short-circuit and batch !result.Changed short-circuit remain at call sites. Contract preserved (single-version pushed=true/false; batch nil on success).

TESTS:
- Status: Adequate. Single-version: plain-push-no-tag, pre-push reset, blocking "push" StageStarted/StageSucceeded, release-only omits push stage. Batch: one-commit-at-end + plain push, pre-push reset + "push" StageFailed + no StageSucceeded, stage-failure → "record" StageFailed, byte-identical no commit/push. Both paths through the shared helper (anti-drift verified end-to-end).

CODE QUALITY:
- Followed conventions (Mutator for mutations, read seam for reads, doc comments). SOLID good — single-responsibility helper; stage/commit (stageAndCommitChangelog) and push/recovery (pushChangelogCommit) mirror each other. Low complexity, precise doc comment.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/engine/regenerate_write_test.go (new test) — add a RegenerateWrite-level no-op assertion: a fresh --target changelog run whose RegenerateChangelog nets no change must make no commit and issue no `git push origin HEAD` (pushed=false). Currently pinned only at the RegenerateChangelog unit boundary, not through the writeAndPushChangelog short-circuit the acceptance criterion names.
