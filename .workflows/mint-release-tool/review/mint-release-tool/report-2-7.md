TASK: mint-release-tool-2-7 — on_notes_failure resolution (abort default / fallback)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/notes/resolve.go: ResolveFailure (mode-only: abort default for ""/abort/unknown; fallback otherwise), resolveFallbackBody (fixed rel.Fallback verbatim → no git; else FallbackBody commit-subject list via `git log --format=%s {lastTag}..HEAD`; else StubBody floor), abortError (wraps cause w/ %w, names via causeText). Precedence scoping in select.go: branches 1-3 (first-release/degenerate/--no-ai) return before resolveNormalPathFailure (branch 4 only). Commit-subject list via runner seam, never to transport.

TESTS:
- Status: Adequate. resolve_test.go: abort default (no body, errors.Is, zero git), explicit abort, fallback commit-subject, fixed-string short-circuit (no git), unknown-mode→abort, empty-log→floor, all four varied causes through both modes (table), unknown-cause fallback, FallbackBody git-failure surfaced. select_test.go proves branches 1-3 never route through on_notes_failure. noai_test.go byte-identical bodies across both fallback paths.

CODE QUALITY:
- Followed conventions (errors.Is, %w, runner seam, t.Parallel, FakeRunner). SOLID good — single shared fallback selector prevents drift; ResolveFailure pure. Low complexity, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/notes/resolve.go:91 / internal/notes/degenerate.go:14 — the fallback empty-log floor reuses StubBody() ("Maintenance release — no notable source changes"), semantically a degenerate-diff message, not a "no commits since last tag" message. Consider whether the fallback floor warrants its own honest phrase distinct from the degenerate stub (cosmetic, no behavioural impact).
