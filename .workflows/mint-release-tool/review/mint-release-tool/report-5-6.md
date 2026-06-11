TASK: mint-release-tool-5-6 — Fresh source: re-diff vX-1..vX + AI notes

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented, no drift. RegenerateFreshBody and RegenerateFreshRegenerator (internal/engine/regenerate_fresh.go:56,82) reuse forward notes.Generator via GenerateFromRange/GenerateFromRangeWithContext, driven by res.DiffRange() (vX-1..vX). Exclusion tiers from cfg in freshExcludeConfig:112; AssembleRange/BuildChangeMapForRange share excludePathspecs so diff + Change Map ride identical path-based excludes. First-release guard precedes any assembly (:59); wired CLI at cmd/mint/regenerate_run.go:64,80. Degenerate guard short-circuits the always-bookkeeping-bearing fresh range before any AI call.

TESTS:
- Status: Adequate. regenerate_fresh_test.go: range substitution, CHANGELOG always-excluded, plain-vs-embedded version_file, path-not-commit exclusion (no --not/^ in argv), Change Map prepend + after-exclusion, first-release no-AI/no-diff, max_diff_lines guard, AI-failure surfaced, regenerator one-time context, degenerate stub. range_test.go pins shared range methods.

CODE QUALITY:
- Followed conventions (runner seam + injected Transport, recording transport, %w preserving errors.Is, doc comments). SOLID/DRY good — fresh path is thin range substitution over reused forward engine; no exclusion logic reimplemented. Low complexity.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/notes/changemap.go:22-26 — BuildChangeMap doc comment describes the exclude set as only "CHANGELOG.md plus one :(exclude)<glob> per configured diff_exclude glob", omitting the strategy-aware version_file tier that excludePathspecs now also appends. Update to list the version_file tier.
- [do-now] internal/notes/assemble.go:138-144 — AssembleDiff doc comment lists excludes as "CHANGELOG.md followed by one :(exclude)<glob> per configured diff_exclude glob", not mentioning the version_file pathspec the shared excludePathspecs appends. Add the strategy version_file tier to the comment.
