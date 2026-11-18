TASK: mint-release-tool-3-7 — Bookkeeping commit folds changelog + version-file projection

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. record.CommitBookkeeping (internal/record/commit.go:69-85) stages only changed paths in ONE `git -C {dir} add {paths…}` via bookkeepingPaths (changelog-then-version order), then one commit with BookkeepingSubject. Combined no-op short-circuits when len(paths)==0; stage-fail short-circuits before commit. Spine (engine/release.go:484-527) projects version FIRST (embedded mismatch aborts before changelog write), then changelog, then folds. Distinct from CommitDirtyTree hook-artifact commit.

TESTS:
- Status: Adequate. commit_test.go: changelog-only, both-folded-one-add, version-only, combined-no-op (zero invocations), subject prefix/tag table, stage-fail-no-commit. release_versionfile_test.go: both-change fold (one commit, version-file never staged alone, disk bytes), version-no-op single commit, combined no-op proceeds to tag+push, embedded mismatch aborts before tag + before changelog write. "No version_file → Phase 1" via baseline seedHappyGit.

CODE QUALITY:
- Followed conventions (CommandRunner/Mutator seam, error wrapping w/ domain noun + path, t.Parallel, exact-argv, doc comments). SOLID/DRY good — BookkeepingSubject single source, bookkeepingPaths isolates stage-order. Low complexity, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/engine/release.go:918-920 — bookkeepingWillCommit re-derives the commit-or-not rule (changelogChanged || (versionChanged && versionFile != "")) that already lives implicitly in record.bookkeepingPaths (commit.go:100-109). The two can silently drift if the staging rule changes. Have CommitBookkeeping return a committed bool (or export record.BookkeepingWillCommit/BookkeepingPaths) and consume that in the spine so made.Commits tracks the one true rule. [Also flagged by 3-8.]
