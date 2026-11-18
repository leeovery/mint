TASK: mint-release-tool-3-10 — Strategy-aware version_file diff exclusion (plain excludes, embedded doesn't)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/notes/assemble.go:48-58 versionFileExcludePathspec (pure decision), wired at :111-121 (excludePathspecs appends the strategy entry after CHANGELOG + globs), consumed by forward AssembleDiff and regenerate AssembleRange via shared union builder. ExcludeConfig carries VersionFile/VersionPattern. Three branches: plain→exclude, embedded→don't, none→nothing. No de-dup (git tolerates).

TESTS:
- Status: Adequate. White-box table test of pure decision (assemble_internal_test.go) covers all three branches. Forward-path argv tests (assemble_test.go) assert plain excludes, embedded doesn't, no-version-file nothing, order, embedded-also-in-globs union. Regenerate equivalents in range_test.go + Change Map carrying same excludes. Forward-path "inert" exercised structurally (decision lands in argv).

CODE QUALITY:
- Followed conventions (table-driven t.Parallel tests, runner seam, doc comments). SOLID good — pure decision function isolated from argv assembler. Low complexity, named returns, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
