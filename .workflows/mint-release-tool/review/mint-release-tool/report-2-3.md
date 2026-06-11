TASK: mint-release-tool-2-3 — max_diff_lines guard (default 50000)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/notes/size.go: ErrDiffTooLarge sentinel, CheckDiffSize, countDiffLines. Wired in generate.go:167 before transport.Generate:183. Config default 50000. Boundary inclusive (got > maxLines fails). Pure function (diff, maxLines); orchestrator resolves limit from cfg.MaxDiffLines. Empty diff = 0; trailing newline no phantom line; final partial line counts. Excluded-path exclusion structural (input already post-exclusion). Trimmed-diff escalation deferred (documented).

TESTS:
- Status: Adequate. Covers exactly-max passes (50000), over-max ErrDiffTooLarge (50001), message carries both counts + names max_diff_lines, empty=0, trailing partial line counts, trailing-newline no phantom, post-exclusion-only, custom limit at boundary. "AI not called when over" verified at integration layer (generate/range/regenerate tests). Config default/override tested.

CODE QUALITY:
- Followed conventions (sentinel + %w, errors.Is, lowercase error strings, doc comments, pure function w/ injected limit). SOLID good, low complexity, modern idioms, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
