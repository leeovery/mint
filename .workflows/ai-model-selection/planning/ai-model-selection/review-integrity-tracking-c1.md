---
status: in-progress
created: 2026-06-13
cycle: 1
phase: Plan Integrity Review
topic: AI Model Selection
---

# Review Tracking: AI Model Selection - Integrity

## Findings

### 1. Task 1-7's `TimeoutFor` return type is left open, but four Phase 2 tasks firmly assume `*time.Duration`

**Severity**: Important
**Plan Reference**: Phase 1, Task ai-model-selection-1-7 ("Add the layered `TimeoutFor(verb)` accessor with value semantics"); consumed firmly by Phase 2 Tasks 2-2, 2-3, 2-4, 2-5
**Category**: Dependencies and Ordering / Task Self-Containment (cross-task contract consistency)
**Change Type**: update-task

**Details**:
Task 1-7 produces the `TimeoutFor(verb)` accessor whose return type is the load-bearing contract every Phase 2 wiring task consumes. But 1-7 presents the return type as an open implementer choice — its Do says `func (c Config) TimeoutFor(verb Verb) <ReturnType>` and reads "return `*time.Duration` (or a tiny wrapper) … If a wrapper type is chosen, define it in `config` and expose whether it is the explicit-zero/no-deadline case." The acceptance criteria only pin the behaviour ("the return type distinguishes an explicit-`0`/no-deadline result from a positive/floor result"), not the concrete type.

Meanwhile all four downstream Phase 2 tasks treat `*time.Duration` as already decided and build their own contracts on it:
- Task 2-2 (Context): "`config.TimeoutFor(verb)` returns `*time.Duration`" and changes `ai.Config.Timeout` to `*time.Duration` specifically "so the three wiring sites assign the accessor's return directly with no lossy conversion."
- Task 2-3 (Do): "`cfg.TimeoutFor` returns `*time.Duration` (Phase 1) which matches `ai.Config.Timeout`'s new `*time.Duration` type (Task 2-2) — assign directly, no conversion."
- Task 2-4 (Do): "`cfg.TimeoutFor` returns `*time.Duration` (Phase 1), matching `ai.Config.Timeout`'s `*time.Duration` (Task 2-2) — assign directly."
- Task 2-5 (Do): same firm `*time.Duration` assumption.

If the implementer of 1-7 exercises the offered "tiny wrapper" option, the "assign directly, no conversion" contract in Tasks 2-3/2-4/2-5 and the type-match premise in Task 2-2 are all invalidated, forcing rework across four tasks in a later phase. Because the spec defers only "the mechanism" and the whole plan has already converged on `*time.Duration` everywhere it is consumed, Task 1-7 should pin `*time.Duration` as the decided return type and drop the wrapper alternative, removing the inconsistency. (The spec's "e.g. `*time.Duration` / a small wrapper" remains satisfied: `*time.Duration` is the chosen instance of that mechanism.)

This edits Task 1-7 only — the Do paragraph that names the return type, the matching Acceptance Criterion, and the Tests line — to remove the open choice and state `*time.Duration` as fixed, consistent with the four tasks that already depend on it. No Phase 2 task needs changing (they already assume `*time.Duration`).

**Current** (Task ai-model-selection-1-7, first **Do** bullet):
> - Add `func (c Config) TimeoutFor(verb Verb) <ReturnType>` to `internal/config/config.go`. RETURN TYPE: return `*time.Duration` (or a tiny wrapper) so the Phase 2 `config → ai.Config` boundary can distinguish an explicit `0` ("no deadline") from a positive value and from the floor — the spec mandates "no deadline" be reachable ONLY by an operator's explicit `0`, never by a forgotten/zero-by-omission field. A plain `time.Duration` zero would re-introduce the ambiguity the spec forbids. (If a wrapper type is chosen, define it in `config` and expose whether it is the explicit-zero/no-deadline case.) Document the boundary contract in the accessor comment so Phase 2 wires it correctly.

**Proposed** (Task ai-model-selection-1-7, first **Do** bullet):
> - Add `func (c Config) TimeoutFor(verb Verb) *time.Duration` to `internal/config/config.go`. RETURN TYPE is fixed at `*time.Duration` — the plan commits to it (not a wrapper) because Phase 2 depends on it directly: Task 2-2 changes `ai.Config.Timeout` to `*time.Duration` and Tasks 2-3/2-4/2-5 assign this accessor's return to it with NO conversion, so any other return type (a wrapper struct, a plain `time.Duration`) would break those four tasks' "assign directly" contract. The `*time.Duration` distinguishes the three cases the boundary must keep apart: `nil` is never returned by this accessor (the 60s floor guarantees a non-nil result), a pointer to `0` is the operator's explicit "no deadline", and a pointer to a positive value is a real deadline — so the spec's invariant ("no deadline" reachable ONLY by an explicit `0`, never by a forgotten/zero-by-omission field) holds, and a plain `time.Duration` zero (which would re-introduce the absent-vs-explicit ambiguity the spec forbids) is rejected. This satisfies the spec's deferred mechanism ("e.g. `*time.Duration` / a small wrapper") by choosing the `*time.Duration` instance. Document the boundary contract in the accessor comment so Phase 2 wires it correctly.

**Current** (Task ai-model-selection-1-7, **Acceptance Criteria**, the return-type criterion):
> - [ ] The return type distinguishes an explicit-`0`/no-deadline result from a positive/floor result (so Phase 2's boundary can keep "no deadline" reachable only by explicit `0`).

**Proposed** (Task ai-model-selection-1-7, **Acceptance Criteria**, the return-type criterion):
> - [ ] The accessor returns `*time.Duration` (never a wrapper or a plain `time.Duration`), and that return distinguishes an explicit-`0`/no-deadline result (a pointer to `0`) from a positive/floor result (a pointer to a positive value) — so Phase 2's `ai.Config.Timeout` (also `*time.Duration`, Task 2-2) takes the accessor's return by direct assignment with no conversion, keeping "no deadline" reachable only by an operator's explicit `0`.

**Current** (Task ai-model-selection-1-7, **Tests**, the return-type test):
> - `"its return distinguishes an explicit-zero (no deadline) from a positive/floor value"`

**Proposed** (Task ai-model-selection-1-7, **Tests**, the return-type test):
> - `"its *time.Duration return distinguishes an explicit-zero (no deadline) from a positive/floor value"` — assert the accessor's return type is `*time.Duration` and that an explicit `0` yields a pointer to `0` while a positive/floor case yields a pointer to a positive value (the type Phase 2 assigns directly into `ai.Config.Timeout`).

**Resolution**: Pending
**Notes**:

---
