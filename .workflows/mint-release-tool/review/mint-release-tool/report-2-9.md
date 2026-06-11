TASK: mint-release-tool-2-9 — Deliberate skip: --no-ai fallback path

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented (correct, no drift). internal/notes/noai.go NoAIBody delegates one-line to shared resolveFallbackBody (resolve.go) — genuinely shares the 2-7 builder so paths can't drift. Imports nothing AI-related — "no AI" enforced by construction. on_notes_failure (rel.OnNotesFailure) correctly not consulted. Empty-log floor to StubBody keeps tag non-empty. Precedence wiring (select.go, 2-10) and CLI flag (2-16) out of scope.

TESTS:
- Status: Adequate. noai_test.go: commit-subject body + exactly-one git-log invocation w/ asserted argv + no AI call; fixed-string path zero git invocations; never-aborts even with OnNotesFailure="abort"; non-empty floor on empty log; shared-builder equivalence vs ResolveFailure(fallback) across list + fixed-string; genuine-git-failure surfacing. Behaviour-focused.

CODE QUALITY:
- Followed conventions (runner/FakeRunner seam, table test, t.Parallel, doc comments, %w). SOLID good — policy-only wrapper over shared selector, DRY via genuine reuse. Low complexity, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
