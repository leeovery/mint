TASK: mint-release-tool-1-4 — Repo root anchoring & release-branch resolution

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented
- Location: internal/gitrepo/gitrepo.go
- Notes: ResolveRoot runs `git rev-parse --show-toplevel`, wraps ErrNotARepository on failure. ResolveReleaseBranch returns config override verbatim when set (no git call), else deriveFromOriginHead (`git symbolic-ref --short refs/remotes/origin/HEAD`, strips origin/ prefix, ErrOriginHeadUnset on failure). Distinguishable sentinels via errors.Is. Worktree/submodule semantics recorded as intent comment. Wired in cmd/mint/main.go, engine/release.go, engine/init.go; cfg threaded as override source. All git calls via runner seam.

TESTS:
- Status: Adequate
- Coverage: All five ACs + three edge cases have dedicated behaviour-focused tests; error tests via errors.Is on exported sentinels; invocation assertions confirm the seam.

CODE QUALITY:
- Followed conventions (exported sentinels + errors.Is, %w, context-first, runner injection, black-box tests). SOLID good, low complexity, modern idioms, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
