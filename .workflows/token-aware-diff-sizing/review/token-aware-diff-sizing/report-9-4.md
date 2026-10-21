TASK: token-aware-diff-sizing-9-4 — Corrections (make the setup guide's point-of-no-return line and the README's two unwind promises name both ways the exclusion check's answers survive a pre-push unwind)

ACCEPTANCE CRITERIA:
- Running `mint setup` prints, in the pipeline section's point-of-no-return bullet after "nothing after it is.", the sentence "The one exception: after an abort or failure before the push, the exclusion check's answers stay: their commit is re-committed on the starting commit, and where git ignores an untracked .mint.toml, their uncommitted write is left in place." Its `.mint.toml` has no backticks, its lines wrap at the width of the bullet's other lines with the same two-space continuation indent, and the rest of the guide is unchanged.
- README.md's "One atomic point of no return" feature-list bullet ends with the replacement sentence the Solution gives for README.md:30, and its earlier sentences are unchanged.
- README.md's "Surgical unwind before the point of no return" bullet ends with the replacement sentence the Solution gives for README.md:351, and its earlier sentences are unchanged.
- After the change, `grep -rn "exclusion check's answers commit" --include='*.go' --include='*.md' --exclude-dir=.workflows .` returns nothing.
- The five gates in CLAUDE.md pass with no test added, removed or changed.

STATUS: complete

SPEC CONTEXT: §18.1 (setup guide, "The point-of-no-return line") and §18.2 (README, "The unwind promises"), as corrected, require both docs to name the one unwind exception in full: the answers commit is re-committed on the starting commit, and where git ignores an untracked `.mint.toml` the uncommitted write is left in place (§6.6). §18.3 already put the same wording in CLAUDE.md's "Surgical unwind, then PONR" invariant.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/setupguide/setupguide.go:267-270 (inside the `pipelineSection` raw string); README.md:30; README.md:353 (the task's README.md:351, moved down two lines by later README edits)
- Notes: Commit be189f1 touches only README.md and internal/setupguide/setupguide.go, and its three replacements match the Solution word for word. Setup guide: the rendered sentence is exact, `.mint.toml` has no backticks, and the continuation lines keep the two-space indent. The wrap matches the neighbouring lines, which reach 79-80 columns: "re-committed" would take line 268 to 83 columns and ".mint.toml," would take line 269 to 85, so each break falls where the bullet's other lines break. The guide's other lines are unchanged. README: in both bullets only the final sentence changed. The grep criterion holds: re-running the task's grep returns no matches (exit 1). The new text matches the code: internal/engine/release.go:1103-1115 commits the answers only when `record.IsIgnored` is false, then clears `a.edit` in both cases, so the unwind has no edit to revert for an ignored file. internal/record/commit.go:117-131 runs `git check-ignore -q`, which never reports a tracked path as ignored, so the docs' "untracked" qualifier is accurate. The three edited sites now agree with CLAUDE.md:42, README.md:117 and internal/engine/unwind.go:11-14.

TESTS:
- Status: Adequate
- Coverage: This is a documentation-only correction, and the task says no new test is needed. No `_test.go` file pins any of the three sentences (searching tests for "exclusion check's answers", "uncommitted write" and "left in place" finds nothing). internal/setupguide/setupguide_test.go checks section markers and specific content, not the PONR bullet's wording or line widths. The commit changes no test file, so the "no test added, removed or changed" half of the last criterion holds.
- Notes: None

CODE QUALITY:
- Project conventions: Followed. The CLAUDE.md "Comments" contract to keep docs true to the code is met, and the edit is confined to the raw string and README prose.
- SOLID principles: Good (no code-structure change)
- Complexity: Low
- Modern idioms: Yes (N/A, prose only)
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "The five gates in CLAUDE.md pass with no test added, removed or changed." — Reading settles the "no test added, removed or changed" half: commit be189f1 touches only README.md and internal/setupguide/setupguide.go. The gates passing still needs `go build ./...`, `gofmt -l .`, `go vet ./...`, `go test -race ./...` and `golangci-lint run` to be run on the change-set.
