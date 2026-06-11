TASK: mint-release-tool-9-1 — Extract single release-bookkeeping commit-subject builder

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented (no drift). Builder at internal/record/commit.go:87-94 record.BookkeepingSubject(commitPrefix, tag); call sites at internal/record/commit.go:80, internal/release/release.go:126, internal/engine/release.go:1244. The literal `Release %s` now exists in exactly one file (commit.go, repo-wide grep). The old engine bookkeepingSubject helper was removed entirely — dry-run plan calls record.BookkeepingSubject directly inline. record import in release.go:28 retained. Doc comment names it the single source.

TESTS:
- Status: Adequate. New focused unit test internal/record/bookkeeping_subject_test.go pins the exact `🌿 Release v0.0.1` format. Three downstream behaviours covered by existing integration tests asserting the byte-identical literal: commit subject (release_test.go:148, release_versionfile_test.go, release_commitgraph_test.go), tag annotation (release/release_test.go:44), dry-run plan step (release_dryrun_test.go:150-161).

CODE QUALITY:
- Followed conventions (exported func beside ChangelogFileName/CommitBookkeeping, package-idiom test). SOLID good — single owner, DRY drift risk eliminated. Low complexity (one-line builder).

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
