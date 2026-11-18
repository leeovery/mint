TASK: mint-release-tool-4-1 — Lock-resilient git wrapper (git_safe built-in)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented (no drift). internal/git/mutator.go:146 Mutate, :188 handleLock, :204 lockIsStale, :217 lockContention. Routed at record/commit.go, release/release.go, engine/unwind.go, engine/autostash.go. Constructed once and shared via engine.ReleaseDeps.Mutator. Stale-vs-live via mtime; lock path from git stderr. NewMutator clamps budget ≥1 to prevent zero-value silent success. Reads bypass via Run/RunWith pass-throughs.

TESTS:
- Status: Adequate. mutator_test.go: contended-then-success, stale cleared+removal observed, live not-removed+backoff, exhausted-surfaces-error+lock preserved, read pass-through not retried, non-lock-error-immediate-surface, clamped-budget-runs-once, lock-gone-between-attempts, stdin-via-RunWith. Fixed clock + no-op backoff (deterministic). Call-site routing verified by production code.

CODE QUALITY:
- Followed conventions (package doc, injectable Option seams, table test, std asserts). SOLID good — single responsibility, CommandRunner interface. Low complexity, clear retry loop, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/git/mutator.go:166 — the final attempt deliberately neither clears nor backs off (if attempt < m.retryBudget). With budget N only N-1 lock-handling passes occur, and a stale lock cleared on the (N-1)th pass gets exactly one retry. Correct and intended, but the budget-vs-handling-passes interaction is subtle; add a one-line comment clarifying that budget counts total attempts, not clears.
- [do-now] internal/git/mutator.go:217 — lockContention returns an empty lockPath for the generic "Another git process" signature; handleLock/lockIsStale then os.Stat("") (errors → not-stale → backoff), correct but implicit. Add a brief comment on handleLock noting an empty path falls through to backoff (never a remove).
- [idea] internal/git/mutator_test.go — no test exercises the generic-signature-only branch of lockContention (stderr "Another git process…" without the "Unable to create '<path>'" line). Add a focused test asserting a no-path lock signature is still classified as contention and retried (backoff, never remove).
