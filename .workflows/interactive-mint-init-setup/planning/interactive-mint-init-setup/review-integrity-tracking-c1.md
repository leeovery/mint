---
status: complete
created: 2026-06-16
cycle: 1
phase: Plan Integrity Review
topic: Interactive Mint Init Setup
---

# Review Tracking: Interactive Mint Init Setup - Integrity

This integrity review read the planning file end-to-end, all four per-phase task
files (`phase-1-tasks.md` … `phase-4-tasks.md`), all 14 authored tick tasks
(`tick list --parent tick-f3d959`, `tick show` on the convergence tasks), and the
two applied dependency edges (3-2 blocked-by 1-5; 4-1 blocked-by 3-1).

Overall the plan is exceptionally strong: every task carries Problem / Solution /
Outcome / Do / Acceptance Criteria / Tests / Edge Cases / Context / Spec Reference;
acceptance criteria are concrete and pass/fail; tests cover edge cases (the
recurse-don't-count inverse, the dual-level per-level matches, the cold-arrival
header, the README-read-failure loud path); phase progression is sound
(SoT foundation → render/wire → strip → reconcile docs); each task is a single TDD
cycle; emission-surface and package-placement decisions are explicitly flagged for
the implementer rather than silently fixed. Vertical slicing, scope/granularity,
self-containment, and acceptance-criteria quality all pass.

The findings below are confined to **cross-phase dependency edges** that the task
graph does not declare. Both flagged dependencies are protected today by tick's
natural creation-order ordering for sequential top-to-bottom execution, but the
integrity criteria explicitly call out missing *cross-phase* dependencies and
*convergence points lacking explicit edges* — and a cross-phase `tick ready`
query (or any reordering) would currently surface a consumer task before its
producer is done, because no edge records the data requirement. There are no
Critical findings; nothing blocks a straight sequential implementation.

## Findings

### 1. Task 2-2 has no edge to the Phase 1 SoT it consumes (missing cross-phase dependency)

**Severity**: Important
**Plan Reference**: Phase 2, task interactive-mint-init-setup-2-2 (tick-fc2454) — "Render the config-reference section from the Phase 1 SoT"
**Category**: Dependencies and Ordering
**Change Type**: update-task

**Details**:
Task 2-2 renders the config-reference table by calling `config.MetadataRows()` and
`MetadataLevel.String()`, and asserts the default-column tokens (`[]`, `auto`,
`shared`, blank) are carried verbatim. Those tokens do not exist until task 1-2
populates them, and `MetadataRows()`/`MetadataLevel` do not exist until task 1-1.
This is a genuine cross-phase data/capability requirement: 2-2 cannot compile or
pass its tests until the Phase 1 SoT rows (1-1) and their default-column
representation (1-2) are in the tree. The task graph records no `blocked_by` edge
for 2-2, so a cross-phase `tick ready --parent tick-f3d959` could return 2-2 (or
2-1, which assembles 2-2's render seam) before Phase 1 lands. Today only tick's
natural creation-order ordering keeps the sequence correct; the criteria flag a
missing cross-phase dependency regardless of natural order. The honest blocker is
task 1-2 (the latest Phase 1 task 2-2 actually needs — the SoT rows WITH their
default tokens applied; 1-3/1-4/1-5 are drift-test/pin tasks 2-2 does not consume).

Add a dependency edge: **2-2 (tick-fc2454) blocked-by 1-2 (tick-bc3a0f)**. The
"Do" / "Context" already note "confirm against the shipped code … Phase 1 is
planned but not yet implemented" — this finding makes that latent ordering
requirement an explicit graph edge rather than a prose caveat.

**Current**:
> task interactive-mint-init-setup-2-2 (tick-fc2454): blocked_by — (none)

**Proposed**:
> task interactive-mint-init-setup-2-2 (tick-fc2454): blocked_by — interactive-mint-init-setup-1-2 (tick-bc3a0f)
>
> Rationale recorded on the edge: task 2-2 renders `config.MetadataRows()` with the
> default-column tokens applied; both the `MetadataRows()`/`MetadataLevel` API
> (task 1-1) and the default-token population (task 1-2) must exist before 2-2 can
> compile or pass its render tests. 1-2 transitively requires 1-1, so a single
> 2-2 → 1-2 edge captures the requirement.

**Resolution**: Fixed
**Notes**: Applied via `tick dep add tick-fc2454 tick-bc3a0f` — 2-2 (tick-fc2454) now blocked-by 1-2 (tick-bc3a0f). Verified acyclic. Approved in auto mode.

---

### 2. Task 4-3 tripwire has no edge to the Phase 1 SoT key-source it derives from (missing cross-phase dependency at a convergence point)

**Severity**: Important
**Plan Reference**: Phase 4, task interactive-mint-init-setup-4-3 (tick-7edb1f) — "Add the optional key-presence tripwire test"
**Category**: Dependencies and Ordering
**Change Type**: update-task

**Details**:
Task 4-3 is a convergence point: it derives the distinct schema key-name set from a
Phase 1 source (the recommended key-source is `config.MetadataRows()`, falling back
to reflecting the decode-shape `toml` tags) AND it must pass against the reconciled
README produced by 4-1 and 4-2. It currently carries zero `blocked_by` edges.

- The README side is protected by natural intra-phase order (4-3 was created after
  4-1/4-2, so sequential `tick ready --parent <phase-4>` yields it last) — no edge
  strictly required there per the intra-phase natural-order rule.
- The Phase 1 side is a cross-phase dependency with no edge. If 4-3 chooses the
  recommended `config.MetadataRows()` key-source, it cannot compile until task 1-1
  ships. Even if it reflects the `toml` tags directly, its own "Context" notes the
  Phase 1 reflection helper "is not yet in the tree" at authoring time. Either way
  the test's key-source is a Phase 1 artifact, and the missing edge means a
  cross-phase `tick ready` could surface 4-3 before Phase 1 is built.

Add a dependency edge: **4-3 (tick-7edb1f) blocked-by 1-1 (tick-15d94b)** (the SoT
table / `MetadataRows()` it derives the key-name set from). 1-1 is the right anchor
because 4-3 needs only the key NAMES (deduped, container tags excluded), not the
default-column tokens — so 1-2 is not required, only the row set and its keys.
(The README-reconciliation predecessors 4-1/4-2 are already ordered correctly by
natural intra-phase order and need no explicit edge.)

**Current**:
> task interactive-mint-init-setup-4-3 (tick-7edb1f): blocked_by — (none)

**Proposed**:
> task interactive-mint-init-setup-4-3 (tick-7edb1f): blocked_by — interactive-mint-init-setup-1-1 (tick-15d94b)
>
> Rationale recorded on the edge: the tripwire derives its distinct schema
> key-name set from the Phase 1 SoT (`config.MetadataRows()`, the recommended
> key-source) — that source must exist before the test can compile. 4-3 needs only
> the key names, so the SoT table task (1-1) is the correct anchor; the
> default-token task (1-2) is not a prerequisite. The within-phase README
> predecessors (4-1, 4-2) are satisfied by natural creation-order ordering.

**Resolution**: Fixed
**Notes**: Applied via `tick dep add tick-7edb1f tick-15d94b` — 4-3 (tick-7edb1f) now blocked-by 1-1 (tick-15d94b). Verified acyclic. Approved in auto mode. If the implementer instead picks the reflect-the-`toml`-tags key-source
(not `MetadataRows()`), the edge to 1-1 is still warranted — 4-3's own Context
states the Phase 1 reflection helper is the canonical chain it should stay inside,
and 1-1/1-3 land together in Phase 1, so 1-1 remains a safe, sufficient anchor.

---
