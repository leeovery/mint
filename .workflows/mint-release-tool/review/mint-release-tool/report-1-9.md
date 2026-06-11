TASK: mint-release-tool-1-9 — First-release body & Record (changelog + bookkeeping commit)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. record.FirstReleaseBody constant wired via notes/select.go (returned on FirstRelease, no diff/AI). record.WriteChangelog (date injected), CommitBookkeeping + BookkeepingSubject. engine/release.go injects opts.Now (prod time.Now, tests fixed), skips commit on no-op via bookkeepingWillCommit. Atomic write via fsutil.WriteFile (temp+rename). Newest-on-top insertion line-start anchored (indexOfSectionLine) so a `## [` inside a body can't be mistaken for header.

TESTS:
- Status: Adequate. Every AC has a focused test. Body: exact string + zero runner invocations (no-AI). Changelog: absent-file create (KaC preamble), existing prepend newest-on-top, preamble-only, disabled no-op, idempotent in-place replace, identical-content no-op, injected-date header w/ zero-padding. Commit: changelog-only/version-only/both-folded/neither-no-op/subject-prefix-tag/stage-failure short-circuit, exact argv via `git -C {dir}`.

CODE QUALITY:
- Followed conventions (CommandRunner/Mutator seams, injected date, owned exported symbols ChangelogFileName/DateLayout/BookkeepingSubject shared so sinks can't drift). SOLID good — pure string transforms separated from IO and git. Low complexity, errors.Is(fs.ErrNotExist), good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/record/changelog.go:99-111 — readExisting surfaces only fs.ErrNotExist as the create path; the non-ENOENT read-error branch (directory, permission error) is untested. Add a case asserting WriteChangelog wraps and returns a non-ENOENT read error with the "reading CHANGELOG.md" prefix, to lock the error-wrapping contract.
- [idea] internal/fsutil/atomicwrite.go:48 — atomic write renames without fsync of the temp file or parent dir, so after a crash the rename may not be durable on some filesystems even though never torn. The doc comment claims crash-safety (true for atomicity, not durability). Decide whether mint wants directory-fsync durability and align the comment wording.
