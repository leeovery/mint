---
status: complete
created: 2026-06-09
cycle: 1
phase: Plan Integrity Review
topic: CLI Presentation
---

# Review Tracking: CLI Presentation - Integrity

## Summary

Integrity review of the CLI Presentation plan as a standalone document: 4 phases, 29 tasks (6 / 8 / 8 / 7). Read end-to-end (planning.md + phase-1..4-tasks.md).

Overall the plan is strong and implementation-ready. Tasks are consistently vertically sliced (one TDD cycle each), the template is fully honoured (Problem / Solution / Outcome / Do / Acceptance Criteria / Tests / Edge Cases / Context / Spec Reference present throughout), acceptance criteria are concrete and pass/fail, tests cover edge cases not just happy paths, and dependencies follow natural intra-phase order with phase boundaries that make architectural sense (seam → run narration → gating → cross-verb/hardening). Convergence points are handled by explicit "reuses cli-presentation-X-Y" prose in the Problem/Do/Context sections rather than a dependency column, which is sufficient for an implementer.

The findings below are the genuine gaps. The one Important finding is a self-consistency defect introduced by the cycle-1 traceability fix (the brand-leaf provenance change to Task 1-5 was not propagated to the two other brand-bearing tasks, 4-3 and 4-4, which still instruct hardcoding the literal leaf — directly contradicting 1-5's new contract). The remaining findings are Minor polish.

## Findings

### 1. Brand-leaf provenance contradicts Task 1-5 in the other brand-bearing tasks (4-3, 4-4)

**Severity**: Important
**Plan Reference**: Tasks cli-presentation-4-3 (`version` pretty form `🌿 mint v{value}`) and cli-presentation-4-4 (release footer `🌿 released …`); originates from cli-presentation-1-5.
**Category**: Task Self-Containment / Dependencies and Ordering (internal consistency)
**Change Type**: update-task

**Details**:
The cycle-1 traceability review changed Task cli-presentation-1-5 to render the brand leaf from an **engine-supplied payload datum** (a `Leaf`/`Brand` field on `RunInfo`/`RunResult`, defaulting to `🌿`), with a matching acceptance criterion ("The brand leaf is rendered from the engine-supplied payload datum (defaulting to `🌿`), not re-derived/hardcoded in the presenter"). That fix was applied to 1-5 only.

The two other tasks that render the brand leaf were not updated and still describe **hardcoding the literal `🌿`**:

- Task 4-3 (`version`): the `Do` and tests instruct rendering `🌿 mint v{value}` with the leaf as a literal, and the Context says "the leaf glyph ties to the brand" but never references the engine-supplied field from 1-5.
- Task 4-4 (end-of-run footer): the `Do`/Acceptance/tests render `🌿 released {project} v{X} · {url}` with the leaf as a literal.

An implementer picking up 4-4 in isolation would hardcode the leaf, directly violating the 1-5 contract that the presenter must not re-derive/hardcode it. This is an internal inconsistency that would surface as either a contract violation or rework when the implementer notices the mismatch. The fix is to make 4-3 and 4-4 consume the same engine-supplied leaf 1-5 established (the `RunResult.Leaf`/`Brand` field for 4-4; an analogous payload-supplied leaf, or an explicit cross-reference to 1-5's decision, for 4-3's `Version` payload), so all three brand-bearing tasks agree.

This is content already in the plan (the 1-5 decision) being propagated for consistency — it does not add scope.

**Current** (Task cli-presentation-4-4, `**Do**` — release-shape bullet):
>      - `release` → pretty `🌿 released {project} v{version} · {url}`; plain `done: {project} v{version} {url}`.

**Proposed** (Task cli-presentation-4-4, `**Do**` — release-shape bullet):
>      - `release` → pretty `🌿 released {project} v{version} · {url}`; plain `done: {project} v{version} {url}`. The leaf glyph (`🌿`) is the engine-supplied brand leaf carried on the end-of-run payload (the `Leaf`/`Brand` field established for `RunResult` in cli-presentation-1-5, defaulting to `🌿`) — render the supplied leaf, do not hardcode it, so a customised `commit_prefix` brand stays consistent across the start-of-run brand line and this footer.

**Notes**: Companion edits (same finding) — apply alongside the Current/Proposed above so all three brand-bearing tasks agree:

Task cli-presentation-4-4 — add to `**Acceptance Criteria**` (after the release-footer criterion):
- [ ] The release footer renders the engine-supplied brand leaf (the `RunResult` `Leaf`/`Brand` field from cli-presentation-1-5, defaulting to `🌿`), not a hardcoded literal.

Task cli-presentation-4-3 — append to the `**Context**` block:
> Brand-leaf provenance: the pretty `🌿 mint v{value}` form uses the engine-supplied brand leaf established in cli-presentation-1-5 (carried on the payload, defaulting to `🌿`), consistent with the event-payload principle and the "leaf ties to `commit_prefix`" note — render the supplied leaf rather than hardcoding it. If the user prefers a fixed constant leaf (see the open decision raised in 1-5), this task follows the same resolution.

**Resolution**: Fixed
**Notes**:

---

### 2. Task 4-3 `version` payload struct omits the brand-leaf field needed for the pretty dressed form

**Severity**: Minor
**Plan Reference**: Task cli-presentation-4-3 (`type Version struct { Value string }`)
**Category**: Task Template Compliance / Task Self-Containment
**Change Type**: update-task

**Details**:
Following finding 1, if the pretty `version` form renders an engine-supplied leaf (consistent with 1-5/4-4), the `Version` payload as currently defined (`{ Value string }`) carries no leaf field, leaving the implementer to either re-introduce a hardcoded `🌿` (contradicting 1-5) or invent a field with no guidance. This is a small self-containment gap downstream of finding 1. If the user resolves finding 1 toward a fixed-constant leaf, this finding is moot. Raised so the payload shape is decided rather than guessed.

**Current** (Task cli-presentation-4-3, `**Do**` — first bullet):
> - In `internal/presenter/presenter.go`, add the payload event. Recommended: `ShowVersion(v Version)` with `type Version struct { Value string }` (the resolved version value, e.g. `1.4.0`). Add the record implementation to `RecordingPresenter`. Document that this is the **payload exception** — the one event whose plain output is a raw value, not `key: value` narration.

**Proposed** (Task cli-presentation-4-3, `**Do**` — first bullet):
> - In `internal/presenter/presenter.go`, add the payload event. Recommended: `ShowVersion(v Version)` with `type Version struct { Value string; Leaf string }` — the resolved version value (e.g. `1.4.0`) plus the engine-supplied brand leaf (defaulting to `🌿`, consistent with cli-presentation-1-5; the plain form ignores it, the pretty form renders it). Add the record implementation to `RecordingPresenter`. Document that this is the **payload exception** — the one event whose plain output is a raw value, not `key: value` narration. (If finding-1's open decision resolves to a fixed-constant leaf, drop the `Leaf` field and render the literal `🌿` in pretty.)

**Resolution**: Fixed
**Notes**:

---

### 3. ShowNotes pretty rule width: 2-5 says "fixed-width rule", 4-7 must "replace" it — the deferred value is never stated

**Severity**: Minor
**Plan Reference**: Task cli-presentation-2-5 (`Do`: "render a fixed-width rule; note the cap is deferred")
**Category**: Acceptance Criteria Quality / Scope and Granularity
**Change Type**: update-task

**Details**:
Task 2-5 instructs rendering "a fixed-width rule" for the pretty notes titled/closing rules, deferring the `min(terminalWidth, ~50)` cap to 4-7, but never states what fixed width to use in the interim. 4-7 then says to "replace the cli-presentation-2-5 fixed-width rule" with `ruleWidth(...)`. An implementer of 2-5 has to pick an arbitrary width with no guidance, and a divergent choice (e.g. a very wide fixed rule) could ship transiently if phases are implemented and merged independently. Naming the interim width as the same cap constant 4-7 will use (`~50`) makes 2-5 self-contained and makes the 4-7 "replacement" a no-op visual change (only the detection/cap logic is added), reducing churn. This is light polish — the spec's `~50` is already cited in 4-7.

**Current** (Task cli-presentation-2-5, `**Do**` — pretty `ShowNotes` bullet, relevant sentence):
> Decorative-rule width capping (`min(terminalWidth, ~50)`) is a Phase 4 concern — for this task render a fixed-width rule; note the cap is deferred.

**Proposed** (Task cli-presentation-2-5, `**Do**` — pretty `ShowNotes` bullet, relevant sentence):
> Decorative-rule width capping (`min(terminalWidth, ~50)`) is a Phase 4 concern — for this task render a **fixed-width rule at the cap constant** (`~50`, the same constant cli-presentation-4-7 will use), not a terminal-derived width; the terminal-width detection and the `min(terminalWidth, cap)` narrowing are deferred to 4-7, which replaces only the width source, not the rule's appearance on a normal-or-wide terminal.

**Resolution**: Fixed
**Notes**:

---

### 4. Task 3-5 leaves the reuse-confirm `-y` echo subject under-specified ("an analogous subject")

**Severity**: Minor
**Plan Reference**: Task cli-presentation-3-5 (`Do` / Acceptance: "an analogous `{subject}: accepted (-y)` for the reuse confirm")
**Category**: Acceptance Criteria Quality
**Category-note**: pass/fail clarity
**Change Type**: update-task

**Details**:
3-5 specifies the exact plain `-y` echo for the notes gate (`notes: accepted (-y)`) but for the reuse confirm repeatedly says "an analogous subject" / "an appropriate subject for `ReuseConfirmGate()`" without naming it. Because the subject is added as a `Gate.Subject` field (the task's own recommendation), the constructor must set a concrete value — leaving it as "analogous" forces the implementer to invent the string, and the corresponding test ("the reuse confirm is auto-accepted under -y ... with its analogous echo") cannot assert an exact line. Naming the subject (e.g. `notes`, since the reuse-confirm gate is also a notes-acceptance gate, per the spec's "reuse confirm rendered in the same `Continue?` vocabulary") makes the criterion pass/fail. The exact word is refinable, but one must be fixed for the constructor and the test.

**Current** (Task cli-presentation-3-5, `**Do**` — `Gate.Subject` bullet):
> - To carry the echo subject without the presenter re-deriving engine knowledge, add a subject/label to the `Gate` model if not already present (e.g. `Gate.Subject string` defaulting to `notes` for `NotesReviewGate()` and an appropriate subject for `ReuseConfirmGate()`), so the presenter renders `{subject}: accepted (-y)` from the payload rather than hardcoding `notes`. Document this addition (it extends 3-1's model).

**Proposed** (Task cli-presentation-3-5, `**Do**` — `Gate.Subject` bullet):
> - To carry the echo subject without the presenter re-deriving engine knowledge, add a subject/label to the `Gate` model if not already present (e.g. `Gate.Subject string`), set by each constructor: `notes` for `NotesReviewGate()` and `notes` for `ReuseConfirmGate()` (the reuse confirm is also a notes-acceptance gate rendered in the same `Continue?` vocabulary, so its echo is `notes: accepted (-y)` — the source/target gates in cli-presentation-3-7 set `source`/`target`). The presenter renders `{subject}: accepted (-y)` from the payload rather than hardcoding `notes`. Document this addition (it extends 3-1's model). (The exact subject word is refinable, but each constructor must set a concrete value so the `-y` echo line is deterministic and testable.)

**Resolution**: Fixed
**Notes**:

---

### 5. Task 3-7 source/target prompts modelled as the choice-`Gate` may not fit free-form value selection

**Severity**: Minor
**Plan Reference**: Task cli-presentation-3-7 (models source/target prompts via the 3-1 `Gate` type with `GateChoice` options)
**Category**: Scope and Granularity / Task Self-Containment
**Change Type**: update-task

**Details**:
3-7 reuses the 3-1 `Gate` model — an ordered set of single-key `GateChoice`s parsed by the 3-3 line-read core (trim → lower-case → exact match against declared keys) — for the regenerate **source** and **target** prompts. That fits when sources/targets are a small enumerated key set, but the task's own wording ("type the value, press Enter") and the constructor signature `SourceGate(options []GateChoice, def Choice)` leave it ambiguous whether a target can be a free-form value (e.g. an arbitrary version string or branch name) rather than a declared single-key choice. If targets can be free-form, the 3-3 exact-match-against-declared-keys parser would re-prompt every valid free-form entry as "unrecognised", which contradicts the task. The task should state explicitly that source/target are **enumerated** (declared choice keys, like the y/n/e/r gate) so the shared line-read/exact-match core applies unchanged — or, if free-form values are in scope, note that a value-accepting parse variant is needed. Given the spec frames them as "selection prompts" with available options the engine supplies, the enumerated reading is most likely; stating it removes the ambiguity an implementer would otherwise have to resolve.

**Current** (Task cli-presentation-3-7, `**Do**` — first bullet):
> - Model the source and target prompts as gates using the 3-1 `Gate` type. Provide constructors, e.g. `func SourceGate(options []GateChoice, def Choice) Gate` and `func TargetGate(options []GateChoice, def Choice) Gate`, with the gate `Prompt` text being the source/target question (e.g. `Source?` / `Target?`) and a `Subject` (3-5) of `source` / `target` for the `-y` echo. The declared options and default come from the engine (the available sources/targets and the flag/default) — the presenter does not invent them.

**Proposed** (Task cli-presentation-3-7, `**Do**` — first bullet):
> - Model the source and target prompts as gates using the 3-1 `Gate` type, treating source/target as an **enumerated declared choice set** (the engine supplies the available sources/targets as `GateChoice` keys, exactly like the y/n/e/r gate), so the shared 3-3 line-read/exact-match-against-declared-keys core applies unchanged — there is no free-form value entry on this path. Provide constructors, e.g. `func SourceGate(options []GateChoice, def Choice) Gate` and `func TargetGate(options []GateChoice, def Choice) Gate`, with the gate `Prompt` text being the source/target question (e.g. `Source?` / `Target?`) and a `Subject` (3-5) of `source` / `target` for the `-y` echo. The declared options and default come from the engine (the available sources/targets and the flag/default) — the presenter does not invent them. (If the engine ever needs free-form value entry rather than a fixed option set, that is a separate parse variant and out of scope for this task.)

**Resolution**: Fixed
**Notes**:

---
