TASK: mint-release-tool-7-2 — Apply the degenerate-diff guard on the regenerate fresh path

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/notes/generate.go:163-165 — IsDegenerate(diff) → return StubBody(), nil inserted at head of shared generateFromDiffWithContext core (before CheckDiffSize/change map/transport). Both fresh producers reach it: single via RegenerateFreshBody → GenerateFromRange, batch/r via GenerateFromRangeWithContext. cmd wires identical engine.RegenerateFreshBody as ProduceBody for single + --all. Guard single-sourced in the shared core; forward AI path reaches same core only after SelectBody already excluded degenerate diffs, so the core check is a harmless never-true re-assertion forward, not a duplicated rule. Non-degenerate untouched (leading short-circuit only).

TESTS:
- Status: Adequate. notes layer: range_test.go:328 (single fresh degenerate → StubBody, 0 transport), :354 (fresh-with-context degenerate, context never reached), :189 non-degenerate still calls transport, :379 empty-context byte-identity. engine: regenerate_fresh_test.go:285 (RegenerateFreshBody degenerate → StubBody, 0 calls), :312 (RegenerateFreshRegenerator/r re-run degenerate → StubBody, 0 calls). Whitespace-only inputs exercised.

CODE QUALITY:
- Followed conventions (testify-free table-light tests, seam-injected fakes). SOLID/DRY good — guard single-sourced; reuses IsDegenerate/StubBody. Low complexity, documented placement.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/engine/regenerate_batch_test.go (new test) — no --all-loop-level test proving a degenerate version is collected (not skipped) and the batch continues to subsequent versions. Per-version outcome is proven + continuation structurally guaranteed by RegenerateAll, but no test pins "degenerate version mid-batch → StubBody collected, later versions still processed." Add a batch test w/ a degenerate version between two normal ones asserting the degenerate body is collected as StubBody (in collected, not skipped) and the following version still runs.
- [do-now] internal/notes/degenerate.go:42 — comment line begins with `/ StubBody` (single slash) instead of `// StubBody`; mid-block so it compiles as a continuation comment but reads as a typo. Fix to `//`.
