---
topic: notes-failure-output-ugly-and-uninformative
cycle: 2
total_findings: 6
deduplicated_findings: 2
proposed_tasks: 2
---
# Analysis Report: notes-failure-output-ugly-and-uninformative (Cycle 2)

## Summary
Cycle 2 surfaced two genuinely actionable items against the shipped notes-failure fix: a byte-identical pair of single-StageFailed-extraction test helpers split across the `engine` and `engine_test` packages, and a self-contradicting `GenerationError.Error()` string ("exit 0") on the empty/whitespace-body construction path. The standards agent reported clean; all four lower-severity duplication patterns were independently re-judged this cycle and correctly left as-is. Both proposed tasks are tightly scoped, behaviour-preserving, and independently executable.

## Discarded Findings
- GenerationError carrier literal built in two transport branches (duplication, low) — re-judged this cycle and concurs with c1: the two branches are reached via genuinely different control flow, each carries a load-bearing WHY-comment, and the repeated 3-field literal is smaller than the indirection a shared helper would add. No action by the agent's own recommendation.
- notesFailureOutput / hookFailureOutput parallel extraction helpers (duplication, low) — re-judged and concurs with c1: the parallel is mandated by spec Fix 1 and documented; the carrier types are distinct (no common interface) and the bodies diverge substantively after the ~2-line guard. Two helpers are correct.
- Repeated four-sentinel→concise-phrase test table across test files (duplication, low) — re-judged: this is healthy, intentional test data; each table pins the same contract from a different angle, the tables are not identical, and the project's test idioms explicitly favour self-contained exact-value assertions over a shared cross-package fixture. The single source of truth (notes.CauseText) already exists in production.
