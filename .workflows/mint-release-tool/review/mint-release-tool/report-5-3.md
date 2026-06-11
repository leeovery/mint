TASK: mint-release-tool-5-3 — Version argument & diff-base resolution (regenerate)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/version/resolve.go (Resolution, DiffRange, ResolveRegenerateTarget, ResolveAllRegenerateTargets, matchingVersions, contains, highestBelow); reuses version.go grammar (ParseSemVer, prefixedPattern, splitTags, parseTag, GreaterThan, SemVer.String). Normalisation, canonical re-prefixing, fail-loud "no tag %s found" (canonical re-prefixed tag), numeric (not lexical) predecessor, first-release marker all correct; single `git tag --list` via runner. No bump/next-version compute. ResolveAllRegenerateTargets (5-11/5-13 enumeration) consumed by regenerate_batch.go — wired, not orphaned.

TESTS:
- Status: Adequate. resolve_test.go: with/without prefix identical, monorepo prefix (strip + re-apply, table), fail-loud canonical message, invalid-version rejection, vX-1..vX base, numeric-vs-lexical predecessor, oldest→first-release (empty PreviousTag + empty DiffRange), non-matching-tag exclusion, runner-seam (one git tag --list). resolveall_test.go: ordering, first-release chaining, monorepo, empty-set, list-failure.

CODE QUALITY:
- Followed conventions (runner seam, FakeRunner, doc comments, strict-SemVer grammar reuse — no second parser/sorter). SOLID good — single-responsibility helpers, grammar centralised. Low complexity, modern idioms.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/version/resolve_test.go — ResolveRegenerateTarget has no test asserting a `git tag --list` failure is surfaced (the single-resolve error branch resolve.go:66-68 is uncovered; sibling ResolveAllRegenerateTargets path tests it). Add TestResolveRegenerateTarget_ListFailureSurfaces seeding Result{ExitCode:1} + error.
- [quickfix] internal/version/resolve.go:151 (highestBelow) — predecessor scan is O(n) full pass per single resolve; could share the sorted-set predecessor-finding with the --all path. Correct and input tiny, so minor dedup tidy.
