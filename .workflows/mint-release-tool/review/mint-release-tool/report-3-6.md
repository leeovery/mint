TASK: mint-release-tool-3-6 — Version-file projection — embedded mode (version_pattern)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/record/versionfile.go:133-158 ProjectVersionFileEmbedded, :166-183 compileVersionPattern, dispatch :48-56; wired before the tag at engine/release.go:493-501 (projection runs FIRST, surgical unwind on error). versionKey = next.String("") bare X.Y.Z (aligned w/ 3-5). QuoteMeta on literal pieces + (\d+\.\d+\.\d+) slot group; ReplaceAllLiteralString (fixed replacement, no $ expansion). Zero-match (absent OR present-without-slot) and malformed-pattern (no {version}) are distinct error paths. No commit here.

TESTS:
- Status: Adequate. versionfile_test.go: single match, multiple matches (no stale 1.3.9), zero-match present-file fail-loud (errors.Is + untouched bytes + stable mtime), absent-file fail-loud (no file created), already-at-target no-op, placeholder substituted, malformed-pattern config error (distinct from zero-match). dispatch test routing + fail-loud propagation. engine/release_versionfile_test.go:225-258 proves abort in Record before tag, StageFailed "record", no mutation, CHANGELOG unwritten.

CODE QUALITY:
- Followed conventions (sentinel + errors.Is, %w w/ domain noun, shared atomic write, stdlib-style tests). SOLID good — single-responsibility helpers, clean dispatcher. Low complexity, thorough doc comments.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/record/versionfile.go:22 — versionSlot (\d+\.\d+\.\d+) matches only bare X.Y.Z, so an embedded source already holding a pre-release/build value (e.g. 1.3.9-rc1) would fail to match its full slot and trip the zero-match abort. Spec scopes versions to bare X.Y.Z so acceptable as scoped; decide whether to broaden the slot or document the X.Y.Z-only constraint at the config boundary.
- [quickfix] internal/record/versionfile_test.go — add one test where the existing slot value is a different bare version with extra surrounding regex-metacharacter source (literal . or $ adjacent) to pin QuoteMeta anchoring; current tests use metachar-free surroundings so the literal-quoting guarantee is asserted only indirectly.
