TASK: mint-release-tool-5-8 — Single-version changelog write (in-place idempotent replace)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/engine/regenerate_changelog.go:57 RegenerateChangelog; reuses record/changelog.go:76 WriteChangelog (in-place replace via splitAroundSection/insertSection); stage+commit via shared regenerate_write.go:284 stageAndCommitChangelog; wired into 5-9 caller at regenerate_write.go:251. Reuses forward writer unchanged — no second mutator. No-op short-circuits before staging (Changed=false → no commit). Subject `docs(changelog): regenerate notes for {tag}` (not forward Release subject). No git tag anywhere. Failed git add short-circuits before commit, wrapped w/ tag. All git via lock-resilient Mutator.

TESTS:
- Status: Adequate. regenerate_changelog_test.go: in-place replace no-duplicate (count==1), exact regenerate subject + negative (not forward Release subject), no-net-change → zero invocations + unchanged file, never cuts a tag (scans all args), at-most-one commit, create-if-absent, failed-stage-before-commit. Reused writer covered in record/changelog_test.go.

CODE QUALITY:
- Followed conventions (injected date, record.ChangelogFileName single owned symbol, FakeRunner, error wrapping names failing step). SOLID good — single responsibility; shared helpers eliminate single/batch drift. Low complexity.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/engine/regenerate_write.go:299 — fix the malformed comment line: it begins with a single `/` (`/ is narrated as a BLOCKING stage`) instead of `//`. Syntactically valid only because line 298 ends with a line comment, but on its own reads as a stray `/`. Change to `// is narrated`.
