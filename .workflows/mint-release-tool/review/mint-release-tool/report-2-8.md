TASK: mint-release-tool-2-8 — Degenerate-diff stub path

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented, no drift. internal/notes/degenerate.go: stubBody const, IsDegenerate (strings.TrimSpace(diff)=="" — empty + all-excluded both reduce to "", whitespace-only collapses), StubBody. Consumed in select.go precedence (step 2: after first-release, before --no-ai/AI) and reused as the resolve.go empty-fallback floor. AI-never-invoked structurally enforced — file imports only strings, no transport reachable.

TESTS:
- Status: Adequate. degenerate_test.go: empty, all-excluded→empty, whitespace-only table (6 cases incl. CRLF/mixed), real-diff false branch, real-content-amid-whitespace false branch, exact stub wording, single-line honesty. End-to-end "AI never invoked" verified at precedence boundary (select_test, range_test, engine release_priortag_test) — correct division.

CODE QUALITY:
- Followed conventions (doc comments, table subtests + t.Parallel, external _test package, behaviour-focused). SOLID good — single responsibility, pure dependency-free functions. Low complexity.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
