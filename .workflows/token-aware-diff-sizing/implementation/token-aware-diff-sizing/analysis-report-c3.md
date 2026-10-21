# Analysis Report: Token Aware Diff Sizing (Cycle 3)

## Stats

- Total findings: 2
- Deduplicated findings: 2
- Proposed tasks: 2

## Summary

The duplication agent found nothing that clears the floor. The standards and architecture agents each raised one low finding, and the two do not overlap. Both become proposals because each fits a pattern this plan already built:
- The setup guide's AI-transport paragraph still sends every failed release call straight to `on_notes_failure`. Cycle 2's Task 2 corrected the same claim at README.md:295, so the two references now disagree. The fix is one sentence, so it goes in the cycle's Corrections bundle.
- `RegenerateRunRequest.Yes` is a field nothing reads. It sits beside `ReleaseDeps.Yes`, which since this feature decides whether a single-version regenerate's notes floor offers its choice. Cycle 1's Task 1 settled that -y and the stdin signal reach the engine through `ReleaseDeps` on a tested seam, and a second, unread home for the same flag is a trap on that path.

The standards agent re-checked every behaviour decision against the specification and its corrigenda and found them all as built, so no spec defect is recorded. One stale comment in internal/notes/assemble.go is collected below for direct application.

## Comment Corrections

- internal/notes/assemble.go:113-115 — "mint does NO Go-side glob matching" is false now that `pathspecMatches` (internal/notes/pathspec.go) matches `diff_keep` globs and the AI's suggested globs in Go, including the Exclude answers that join these pathspecs through `Assembler.Exclude`
  OLD: // diff and Change Map git calls — the UNION of the exclusion tiers. git
// interprets each glob as a pathspec pattern — mint does NO Go-side glob matching — so a
// glob matching nothing is harmless and a force-added tracked file matching a glob is
  NEW: // diff and Change Map git calls — the UNION of the exclusion tiers. git
// interprets each glob as a pathspec pattern, so a glob matching nothing is harmless
// and a force-added tracked file matching a glob is

## Discarded Findings

- None. Both findings clear the floor, neither reverses a settled direction, and each fits a pattern an approved task already set (cycle 2 Task 2's routing correction and cycle 1 Task 1's attended-signal seam).
