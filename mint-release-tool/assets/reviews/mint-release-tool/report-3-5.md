TASK: mint-release-tool-3-5 — Version-file projection — plain mode (whole file is the version)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. ProjectVersionFilePlain (internal/record/versionfile.go:83-99). Bare semver + single trailing newline canonical (version+"\n"); no-op via read+compare; atomic write via fsutil.WriteFile; create-if-absent via readVersionFileContent found=false on ErrNotExist. No commit created (folded in 3-7). Write-only mirror (read only for no-op detection, documented). Bare-version + single-newline convention documented in doc comment.

TESTS:
- Status: Adequate. versionfile_test.go:45-136 — absent→created (verbatim "1.4.0\n"), older→overwritten, already-at-target→no-op (mtime-stability), target-without-newline→rewritten. dispatcher test proves routing by absent pattern. Behaviour-focused (bytes, changed flag, mtime).

CODE QUALITY:
- Followed conventions (%w, named returns, sentinel pattern, shared atomic write delegated to fsutil — DRY). SOLID good — single-responsibility helpers, clean dispatcher. Low complexity, modern idioms, thorough doc comments.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
