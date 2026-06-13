---
status: complete
created: 2026-06-13
cycle: 3
phase: Plan Integrity Review
topic: AI Model Selection
---

# Review Tracking: AI Model Selection - Integrity

## Findings

### 1. Task 2-2's nil-handling Do bullet still endorses "a `deadline` copied from `cfg.Timeout`" — the exact direct pointer copy the cycle-2 fix declared WRONG, leaving the task internally self-contradictory

**Severity**: Critical
**Plan Reference**: Phase 2, Task ai-model-selection-2-2 ("Make `ai.Config.Timeout` carry absent-vs-explicit-zero and apply the per-attempt deadline conditionally"), **Do** — the nil-handling bullet (2nd Do bullet)
**Category**: Task Self-Containment / Acceptance Criteria Quality (internal contract consistency)
**Change Type**: update-task

**Details**:
The cycle-2 fix rewrote Task 2-2's `Transport` struct bullet (Do bullet 4) and `attempt` bullet (Do bullet 5), the Outcome, and AC7 to establish a single, CRITICAL-flagged contract: the internal carrier `Transport.deadline` has the INVERSE polarity of the boundary `Config.Timeout`, so `NewTransport` must **MAP** `cfg.Timeout` into `t.deadline` (`&0` → `t.deadline = nil`; `&positive` → `t.deadline = &value`; `nil` → the wiring-bug case), and "A direct `t.deadline = cfg.Timeout` is WRONG."

But the cycle-2 edit did NOT touch the nil-handling bullet (Do bullet 2), which still reads:

> "store on the Transport a `deadline *time.Duration` **copied from** `cfg.Timeout` and treat nil as 'no deadline ONLY if explicitly nil-by-design'"

That is precisely the direct pointer copy bullet 4 now forbids. The two bullets are in direct contradiction inside the same task:
- Bullet 2 (untouched): recommends `deadline *time.Duration` **copied from** `cfg.Timeout`.
- Bullet 4 (cycle-2): "`NewTransport` must MAP `cfg.Timeout` into `t.deadline`, NOT copy the pointer through. … A direct `t.deadline = cfg.Timeout` is WRONG."

An implementer who follows bullet 2's "copied from `cfg.Timeout`" mechanism ships exactly the regression the cycle-2 finding existed to eliminate: the operator's explicit `&0` (no-deadline) survives as a non-nil `&0` and fires `WithTimeout(ctx, 0)` immediately (instant timeout, violating AC3), and a forbidden nil-by-omission `Config.Timeout` silently becomes `t.deadline == nil` ⇒ no-deadline (unbounded run, violating AC6/AC7). Bullet 2's "treat nil as 'no deadline ONLY if explicitly nil-by-design'" also re-muddles the now-settled contract that a `nil` `Config.Timeout` is the wiring-bug case (NOT a no-deadline trigger). Because the task carries two mutually exclusive mechanisms for the same field, an implementer must guess which one governs — defeating the cycle-2 fix and re-opening the resolved finding. This blocks correct implementation, so it is Critical.

The fix rewrites Do bullet 2 to align with the cycle-2 mapping: keep the "decide how `NewTransport` resolves a nil `Config.Timeout`" intent and the STRICT-preferred contract, but remove the "copied from `cfg.Timeout`" mechanism (which bullet 4 forbids) and the "no deadline ONLY if explicitly nil-by-design" phrasing, and point at bullet 4's explicit mapping as the single mechanism. This edits Task 2-2 only — one Do bullet — and introduces no new contract; it removes the stale wording the partial cycle-2 edit left behind. No other task changes.

**Current** (Task ai-model-selection-2-2, **Do**, the nil-handling bullet — 2nd Do bullet):
> - Decide how `NewTransport` resolves a nil `Config.Timeout`. Per the invariant, a nil here must NOT silently become "no deadline" — that is the zero-by-omission case the spec forbids. Options, in order of preference: (a) keep `NewTransport` STRICT — a nil `Config.Timeout` is a programming error from a wiring site that forgot to thread the value; since all three production sites are migrated in 2-3/2-4/2-5 to pass the accessor's non-nil return, and tests pass an explicit value, nil should not occur. Choose the mechanism that makes the invariant test-pinnable: e.g. store on the Transport a `deadline *time.Duration` copied from `cfg.Timeout` and treat nil as "no deadline ONLY if explicitly nil-by-design" — but prefer making nil unreachable in production by the wiring contract and pinning it with the boundary test below. Document the chosen contract in the `NewTransport` comment so a future caller cannot reintroduce zero-by-omission.

**Proposed** (Task ai-model-selection-2-2, **Do**, the nil-handling bullet — 2nd Do bullet):
> - Decide how `NewTransport` resolves a nil `Config.Timeout`. Per the invariant, a nil here must NOT silently become "no deadline" — that is the zero-by-omission case the spec forbids. Keep `NewTransport` STRICT: a nil `Config.Timeout` is a programming error from a wiring site that forgot to thread the value; since all three production sites are migrated in 2-3/2-4/2-5 to pass the accessor's non-nil return, and tests pass an explicit value, nil should not occur. Do NOT represent this by copying `cfg.Timeout` straight into the internal carrier — the `Transport` struct bullet below MAPS `cfg.Timeout` into `t.deadline` (a direct `t.deadline = cfg.Timeout` is WRONG, because it would both fire an instant `WithTimeout(ctx, 0)` on the explicit-`&0` no-deadline case AND turn a nil-by-omission into a silent unbounded run). In that mapping, the `cfg.Timeout == nil` branch is the wiring-bug case — it is distinct from the explicit-`&0` no-deadline case and must NOT be folded into the parent-context path; choose the strict handling that makes the invariant test-pinnable (e.g. the documented-unreachable-in-production contract proven by the boundary test below, or a guard that distinguishes nil from `&0`). Document the chosen contract in the `NewTransport` comment so a future caller cannot reintroduce zero-by-omission.

**Resolution**: Fixed
**Notes**: Approved via auto. Rewrote Task 2-2 Do bullet 2 (nil-handling) to align with the cycle-2 mapping (no direct `cfg.Timeout` copy; `nil` is the wiring-bug case, distinct from explicit `&0`). Applied to tick-d69d1b and the phase-2-tasks.md mirror.

---
