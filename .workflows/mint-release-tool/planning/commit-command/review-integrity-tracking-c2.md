---
status: in-progress
created: 2026-06-09
cycle: 2
phase: Plan Integrity Review
topic: Commit Command
---

# Review Tracking: Commit Command - Integrity

## Summary

Cycle-2 re-review with fresh eyes. All 5 cycle-1 findings are confirmed **applied, sound, and complete**, verified in both the task files and the tick store (topic `tick-82909f`, 5 phases + 25 tasks, all priority 2, no dependency edges — consistent with the stated natural-order design):

1. **3-2 reusable editor file-roundtrip routine** (Important) — present: writes a caller-supplied initial buffer to a temp file, launches the resolved 3-1 editor argv against the path (argv, not stdin), waits, reads back; explicitly the routine 3-3/3-4 reuse and 4-1's `e` pre-fills. The caller-supplied-initial-buffer capability that 4-1 needs is now first-class. Sound.
2. **3-2 emptiness tightened to whitespace-only** (Minor) — present: "only whitespace (or no content)"; the `#`-comment-stripping language is removed with an explicit note that there is no scaffolding to strip. Sound.
3. **3-5 editor interactivity pinned to the Presenter stdin determination** (Minor) — present: no separate stdout/`/dev/tty` probe; gated on the same determination the gate uses, with a new AC + test + edge case. Sound.
4. **5-4 deterministic non-zero exit on push failure** (Minor) — present: new AC ("exits non-zero … while the commit remains in place"), new test, and edge-case line; commit stays forward-only. Sound.
5. **1-2 ordering criterion covers the `[commit].prompt` override** (Minor) — present: the prompt-then-diff ordering AC/test/edge now explicitly hold under the override (override replaces the prompt segment only; diff stays in the same trailing position). Sound.

The plan remains strong and implementation-ready: full task-template compliance, single-TDD-cycle vertical slices, logical phase progression (skeleton → staging → fallback → interactive refinement → push), strong self-containment, pass/fail acceptance criteria, and consistently-named consumed-dependency seams. Consumed external dependencies (Presenter, L1/L2 engine, git_safe, verb-namespaced config) are correctly treated as resolved, not re-planned.

One residual finding remains — a wording inconsistency **newly exposed by cycle-1's fix #2**: 4-2 consumes 3-2's emptiness rule but still restates it with the old "non-comment" qualifier that fix #2 deleted from 3-2. Minor.

## Findings

### 1. 4-2 restates the consumed 3-2 emptiness rule with the old "non-comment" qualifier that cycle-1 removed from 3-2

**Severity**: Minor
**Plan Reference**: Phase 4 / commit-command-4-2 (`e` empty-save discards the edit; gate re-renders with the prior message preserved) — Solution paragraph and the first Do bullet.
**Category**: Task Self-Containment / Acceptance Criteria Quality (consistency of a consumed rule)
**Change Type**: update-task

**Details**:
Cycle-1 finding #2 tightened 3-2's emptiness rule to **purely whitespace-only / no-content** and explicitly *removed* the comment-stripping language, adding the note that "there are no `#`-comment lines to strip here … downstream tasks (4-2) reuse this same whitespace-only rule." 4-2 is the task that explicitly **consumes** that rule and instructs the implementer to "Reuse the **3-2 editor-contract emptiness rule** … do NOT introduce a second emptiness definition." But 4-2 still describes the rule it is consuming as *"no non-whitespace/**non-comment** content ⇒ empty"* in **two** places — its Solution paragraph and its first Do bullet. This re-introduces the exact "non-comment" qualifier that cycle-1 deleted from 3-2, leaving the consumer's restatement out of sync with the now-canonical source. An implementer reading 4-2 (which forbids a second emptiness definition while itself describing a slightly different one) could re-introduce comment scaffolding / `#`-comment stripping that 3-2 now forbids. The load-bearing rule on both paths is identical and simple — **whitespace-only (or no content) ⇒ empty**. The fix is to bring 4-2's two restatements into verbatim alignment with the corrected 3-2 wording; no behaviour changes. (4-2's Acceptance Criteria, Tests, and Edge Cases already say "whitespace-only" correctly — only the two prose restatements lag.)

**Current** (Solution paragraph, 4-2):
```
**Solution**: In the `e` branch of the gate (4-1), when the editor save is **empty** — buffer empty, whitespace-only, or quit/abort with no content — **discard** the edited buffer (do NOT adopt it as the new message) and **loop back** to the `Continue?` gate with the **prior** message (the candidate shown before `e`) preserved unchanged, `y`/`n`/`e`/`r` still offered. "Empty" is determined per the editor contract established in 3-2 (no non-whitespace/non-comment content ⇒ empty). Because `e` only ever *replaces* an existing message on a non-empty save and otherwise preserves it, `e` can never yield an empty commit.
```

**Proposed** (Solution paragraph, 4-2):
```
**Solution**: In the `e` branch of the gate (4-1), when the editor save is **empty** — buffer empty, whitespace-only, or quit/abort with no content — **discard** the edited buffer (do NOT adopt it as the new message) and **loop back** to the `Continue?` gate with the **prior** message (the candidate shown before `e`) preserved unchanged, `y`/`n`/`e`/`r` still offered. "Empty" is determined per the editor contract established in 3-2 (content that is **only whitespace, or no content at all** ⇒ empty — there is no `#`-comment scaffolding to strip, since the buffer carries only the real message). Because `e` only ever *replaces* an existing message on a non-empty save and otherwise preserves it, `e` can never yield an empty commit.
```

**Current** (first Do bullet, 4-2):
```
- In the `e` branch of the gate loop (4-1), detect an **empty** save: buffer empty, **whitespace-only**, or quit/abort with no written content. Reuse the **3-2 editor-contract emptiness rule** (no non-whitespace/non-comment content ⇒ empty) — do NOT introduce a second emptiness definition.
```

**Proposed** (first Do bullet, 4-2):
```
- In the `e` branch of the gate loop (4-1), detect an **empty** save: buffer empty, **whitespace-only**, or quit/abort with no written content. Reuse the **3-2 editor-contract emptiness rule** (content that is **only whitespace, or no content at all** ⇒ empty; no `#`-comment stripping — the buffer carries only the real message) — do NOT introduce a second emptiness definition.
```

**Resolution**: Pending
**Notes**:

---
