TASK: mint-release-tool-3-9 — Configurable diff_exclude globs (on top of built-in CHANGELOG.md)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/notes/assemble.go:111-121 excludePathspecs appends one :(exclude)<glob> per ExcludeConfig.Globs after :(exclude)CHANGELOG.md; shared by AssembleRange and both Change Map git calls (changemap.go). Wired from cfg.DiffExclude → ExcludeConfig.Globs on forward (release.go:728) + regenerate (regenerate_fresh.go:114). Globs passed verbatim to git (no Go-side matching); path-based; force-added tracked files not special-cased. max_diff_lines counts post-exclusion text. Tier ordering fixed (CHANGELOG → globs → version_file); duplicates not de-duped (git tolerates).

TESTS:
- Status: Adequate. assemble_test.go: glob on top of CHANGELOG, multiple globs in order, glob-matching-nothing harmless, force-added tracked file excluded, absent→only CHANGELOG, ordering vs version_file. changemap_test.go: globs ride name-status + numstat. size_test.go: counts only post-exclusion. release_priortag_test.go: two real config globs thread through spine. Exact argv assertions prove git excludes.

CODE QUALITY:
- Followed conventions (runner/FakeRunner seam, errors.Is, table test, doc comments). SOLID good — single shared excludePathspecs; ExcludeConfig keeps constructor stable. Low complexity, preallocated slice, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/notes/changemap.go:49,55 and assemble.go:173 — append([]string{...}, a.excludePathspecs()...) reuses the backing array returned by excludePathspecs; safe today (freshly allocated per call, not retained), but if a future change caches the slice the spread-append could alias/mutate it. Consider making the no-aliasing contract explicit (comment on excludePathspecs). Purely defensive.
