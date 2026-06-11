TASK: mint-release-tool-3-11 — Dry-run skips all hooks and reports skipped

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. engine/release.go: runPreflightHook/runPreTagHook/runPostReleaseHook guard on dryRun, report skip only for a CONFIGURED point (absent stays silent no-op), return before invoking runner. reportHookSkipped/dryRunLabel; buildHookEnv threads dryRun into NewHookEnv → MINT_DRY_RUN=1. runPreTagHook returns before porcelain probe / CommitDirtyTree so no artifact commit, committed=false. Deferred-caching comment present; actual 4-7 cache write lives in finishDryRun (outside this task).

TESTS:
- Status: Adequate. release_dryrunhooks_test.go: all three points skipped+reported, no-sh invocation, no porcelain probe / no artifact commit (pre_tag), all-three combined no-run, absent-hook no-report negative case. release_hookenv_internal_test.go asserts MINT_DRY_RUN=1 (dry)/=0 (wet) on the builder directly.

CODE QUALITY:
- Followed conventions (accept-interfaces, single Warn seam reused, no new presenter event). SOLID good — single shared reportHookSkipped keeps skip convention DRY across three sites. Low complexity, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
