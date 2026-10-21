# Review Report: token-aware-diff-sizing (Cycle 1)

## Stats

- Total findings: 23
- Deduplicated findings: 17
- Proposed tasks: 1

## Summary
The review's 23 findings deduplicated into 17 actions. Fourteen were do-now corrections applied in this session (commit 78bfe3d), the one blocking issue (A9) among them. Two were routed out of scope as quick-fixes that predate this work, and one was routed to replan. That one is the chunk-wave deadline test that flakes on wall-clock margins. It is staged as a single task with its direction settled: a deterministic deadline ordering replaces the re-tuned margins. No action indicts the specification.

## Spec Defects

None. §11.6 says "The per-attempt timeout applies to each call on its own" and "There is no overall deadline across the calls". The as-built transport matches: it applies a fresh `context.WithTimeout` per attempt (internal/ai/transport.go:282-290), and the wave sets no deadline of its own (internal/notes/chunk.go:175-197). The defect is the test's timing margins, not the claim.

## Discarded Findings

None were discarded during synthesis. The following were not staged because of their routing:
- A1–A14 (20 source ids, blocking A9 among them) — do-now; applied in this session in commit 78bfe3d
- A16 Batch header describes skip-and-continue as future work — out-of-scope quick-fix; the header is byte-identical to base 6aa345e4
- A17 Docs place pre_tag after notes; code runs it before — out-of-scope quick-fix; all five sites predate this work (base 6aa345e4 had the same order)
