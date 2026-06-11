TASK: mint-release-tool-4-3 — Gate-abort & pre-push failure route through surgical unwind

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented (wiring only, per scope). engine/release.go: gate-n routes to Unwind (only for errGateAborted; other gate errors returned as-is); every pre-push failure routes via surfaceAndUnwind at pre_tag, notes, record×3, preflight/gh-auth, tag/push — with made.TagCreated = errors.Is(err, release.ErrPushRejected) distinguishing created-but-rejected tag from tag-creation failure. Post-PONR publish failure stays warnPublishFailed pointing at regenerate --reuse; no unwind. reviewGate n returns clean errGateAborted with no StageFailed. MadeState tracked not probed. No best-effort/HEAD~N reset helper remains.

TESTS:
- Status: Adequate. release_surgicalunwind_test.go: gate-n after artifact commit (reset, no tag-delete, no HEAD probe), tag-creation failure (reset, no tag-delete), push-rejection (reset + tag-delete), identical-clean-state/summary across n vs pre-push failure, notes failure after artifact commit, post-PONR publish failure (no reset/tag-delete/Unwound, warns regenerate --reuse, finishes nil). Exact git argv, rev-parse HEAD count==1, StageFailed-then-Unwound ordering, byte-identical summaries.

CODE QUALITY:
- Followed conventions (accept-interfaces, runner/Mutator seams, presenter-only, typed AbortError owns exit code). SOLID good — surfaceAndUnwind factors the stage-failure+unwind sibling of the gate path. Low complexity, documented PONR asymmetry.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
