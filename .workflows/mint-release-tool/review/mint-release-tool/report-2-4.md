TASK: mint-release-tool-2-4 — Change Map salience preamble

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented (with forward-compatible extension into Phase 3/5 present). internal/notes/changemap.go: BuildChangeMap, render pipeline, novelty heuristic (newDirectories flags a leading-dir area novel only when every changed path under it is Added — new auth/ headlines, all-modified area doesn't), magnitude (per-area churn ranked desc + alpha tiebreak), notable (top-level files + single largest file w/ alpha tiebreak). Exclusion shared via excludePathspecs() so map sees post-exclusion view. BuildChangeMapForRange is Phase 5 regenerate entry.

TESTS:
- Status: Adequate. Covers new-package-headlines-above-larger-area, renamed+removed, per-area churn ranking, single largest file (+ negative assertion small file not named), new top-level entries, all-in-one-area no-false-headline, computed-after-CHANGELOG-exclusion (exact argv), glob layering, non-zero git surfaced, command-not-found distinguishable. Index-based ordering assertions behavior-focused.

CODE QUALITY:
- Followed conventions (runner seam, focused tests, doc comments, deterministic sorted output). SOLID good — small single-purpose helpers. Low complexity, modern idioms, intent-rich comments.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/notes/changemap_test.go — add a unit test asserting a binary file (`-\t-\tpath`) contributes 0 churn and a renamed file's numstat two-path row (`0\t0\told\tnew`) resolves to the new path; both branches (parseCount 152-161, numstatPath 166-168) are exercised only indirectly.
- [idea] internal/notes/changemap.go:208-211 — the novelty section silently has no cap; on a very large release with many new dirs/removals/renames the "Structural novelty" list could itself become mush (the failure this preamble fights). Consider whether to bound/summarize the novelty/notable lists.
- [do-now] internal/notes/changemap.go:197 — rendered label is "New package: " while the area is a directory; the as-built doc and tests call these "new directories/packages" interchangeably. Consider "New package/dir: " (or align the doc) so a new non-package directory isn't mislabeled. Pure wording.
