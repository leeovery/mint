TASK: mint-release-tool-7-4 — Extract a single shared atomic-write helper and have all three sites delegate

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented correctly. Helper at internal/fsutil/atomicwrite.go:26-53 (CreateTemp/Write/Close/Chmod/Rename with os.Remove cleanup on every post-create error branch + step-named error wrapping). Delegations: record/changelog.go:228-233 (0o644, "writing CHANGELOG.md"), record/versionfile.go:205-210 (0o644, "writing version file %s"), notescache/cache.go:220-222 (entryPerm 0o600; Store.Write wraps "writing note cache entry"). No stray copies of the old idiom remain. Final paths + perms unchanged. (notescache now gets an explicit Chmod where it previously relied on CreateTemp's 0o600 default — observable mode identical, more robust.)

TESTS:
- Status: Adequate. atomicwrite_test.go: happy path (content+perm+no leftover temp), overwrite of existing target, rename-failure cleanup (target unchanged, no leftover temp). record writers via public WriteChangelog/version-file tests; notescache via Store.Write round-trip.

CODE QUALITY:
- Followed conventions (fsutil in internal/, %w wrapping, package-style tests). SOLID good — single-responsibility primitive, callers own domain wording. Low complexity, io/fs.FileMode, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/fsutil/atomicwrite.go:43 — the helper now unconditionally Chmods the temp file, incl. notescache which previously relied on CreateTemp's default 0o600. Observable mode unchanged + arguably more robust. Decide whether to note this deliberate behavioral addition in the helper doc or the notescache entryPerm comment.
- [quickfix] internal/fsutil/atomicwrite_test.go — add a test exercising a Write-or-Close failure branch to directly cover the early cleanup paths rather than relying on similarity to the rename branch.
- [quickfix] internal/record/versionfile_test.go + changelog_test.go — add one assertion that a forced write failure (e.g. target path is a directory) surfaces the domain noun ("version file" / "CHANGELOG.md") in the returned error, locking in the per-domain wording the refactor was required to preserve.
