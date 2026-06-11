TASK: mint-release-tool-4-2 — Surgical pre-PONR auto-unwind (delete tag + reset N commits + report)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/engine/unwind.go:86 Unwind(ctx, deps, start, made, reason); tag-delete `git tag -d {start.Tag}` (gated on made.TagCreated), reset `git reset --hard {start.HEAD}` (gated on made.Commits>0), both via deps.Mutator.Mutate (lock-resilient 4-1). Resets to captured sha, not HEAD~N. Zero-mutation no-op (no Unwound, no git). Returns abort(reason) so engine owns non-zero exit; no StageFailed. Engine-authored summary with "; repo clean" tail, no prefix. No push/publish path — post-PONR invariant structural. StartState/MadeState are explicit captured/tracked state.

TESTS:
- Status: Adequate. unwind_test.go: 2-commits+tag, 1-commit+tag, commits-but-no-tag, tag-only-no-commits, zero-mutation no-op, exact-captured-sha (no HEAD~N), reports-each-undone (no "Reverted" prefix), never-pushes/publishes. Exact git argv for tag-delete + reset; summary strings verbatim incl. tail. Post-PONR-never-unwinds proven end-to-end in release_surgicalunwind_test.go. Singular/plural grammar exercised.

CODE QUALITY:
- Followed conventions (Mutator seam, exact-argv tests, t.Parallel, doc comments, ASCII-pure summary). SOLID/DRY good — single-responsibility helpers, summary authored once. Low complexity, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/engine/unwind.go:104,117 — Mutate errors deliberately discarded (_, _ =) as best-effort (doc-justified), but no test asserts a tag-delete or reset failure mid-unwind is non-fatal (abort still returned, run still terminates). Add a unit test seeding a non-zero git tag -d / git reset to lock in the best-effort contract.
- [do-now] internal/engine/unwind.go:146-148 — the surgicalSummary default branch is reached only when deleted && !reset, but the switch default silently also covers the impossible !reset && !deleted case; consider an explicit case deleted: plus a default panic/guard to make the impossible branch obvious. Pure clarity.
