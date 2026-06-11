TASK: mint-release-tool-1-3 — Version determination from git tags

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented
- Location: internal/version/version.go
- Notes: SemVer strict 3-part (no pre-release/build fields). CurrentVersion lists tags via runner (`git tag --list`), highestMatching numeric max. Next: major zeroes minor+patch, minor zeroes patch, default patch increments. String(prefix) writes prefix back. prefixedPattern uses regexp.QuoteMeta (component prefixes safe). Empty-prefix matches bare X.Y.Z. All ACs met. Scope drift (non-blocking): package also ships ParseSemVer/BumpExplicit/resolve.go for Phase 4/5; GreaterThan legitimately reused by 1-3.

TESTS:
- Status: Adequate
- Coverage: Each AC has a corresponding test (no-tags→0.0.0, ignores non-conforming shapes, global numeric max not lexical, component/empty prefix, prefix write-back, Next bumps, default-patch, lists via runner). Behaviour-focused.

CODE QUALITY:
- Followed conventions; package doc; black-box tests; FakeRunner seam; %w wrapping. SOLID good, low complexity, modern idioms (QuoteMeta, FindStringSubmatch).

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/version/version_test.go:171-187 — CurrentVersion_ListsTagsViaRunner asserts only the command name; add an assertion on invs[0].Args == ["tag","--list"] so a regression to the wrong git subcommand is caught.
- [quickfix] internal/version/version_test.go (CurrentVersion suite) — add a test seeding `git` with a non-nil error to cover the error path at version.go:56-58 (the "listing git tags: %w" wrap is currently unexecuted).
