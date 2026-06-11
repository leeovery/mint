TASK: mint-release-tool-5-5 — Reuse source: read tag annotation body

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented, no drift. internal/engine/regenerate_reuse.go:41 ReadTagBody (single `git for-each-ref … contents:body` call, returns (body, hasBody, err), trim-and-check-empty for hasBody, body returned raw/untrimmed) and :58 ReadReuseBody (single-mode fail-loud wrapper, exact spec message incl. em-dash). Consumed at cmd/mint/regenerate_run.go:62 + regenerate_all.go:91; ReadTagBody reused for hasBody branching at regenerate_batch.go:227 (5-12 seam). Genuine git failures wrapped/surfaced, not masked as "no body". No AI, no diff.

TESTS:
- Status: Adequate. regenerate_reuse_test.go: verbatim whole-body return, exact single for-each-ref argv, three no-body shapes for both hasBody=false and fail-loud message+empty-body, no-AI/no-diff (one git call), git-error surfacing. Error message exact equality incl. em-dash.

CODE QUALITY:
- Followed conventions (runner seam, %w + tag context, doc comments, t.Parallel/t.Context). SOLID/DRY good — ReadReuseBody composes ReadTagBody; single responsibility. Low complexity, intent-revealing names.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
