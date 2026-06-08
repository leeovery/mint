---
status: in-progress
created: 2026-06-08
cycle: 2
phase: Gap Analysis
topic: cli-presentation
---

# Review Tracking: cli-presentation - Gap Analysis

## Findings

### 1. Regenerate source/target prompts — rendering and input handling undefined

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Gating & `-y` Orthogonality (gate inventory, line 90); Gate input handling (lines 99–104)

**Details**:
The gate inventory states `regenerate` has "interactive *source* + *target* prompts, then the notes-review gate." These are interactive stops, and the forbidden-combination rule explicitly "applies to **any** interactive gate" — so the source/target prompts are in scope for this spec. But how the `Presenter` renders them (pretty vs plain) and what input handling they use is never specified. The "Gate input handling" section is explicitly scoped to "the `Continue?` prompt" only (line 99: line-read, empty=default, case-insensitive, re-prompt on garbage) — leaving the source/target prompts with no defined rendering, no default behaviour, and no `Presenter` event.

An implementer would have to invent: the menu/prompt shape, whether they are choice-lists or free text, the plain-mode rendering (the per-event table has no row for them), and whether their input handling mirrors `Continue?`. The forbidden-combination interaction is implied (`-y` "uses flags/defaults, auto-accepts") but the auto-accept event/echo for these specific prompts is unspecified, unlike the explicit `notes: accepted (-y)` echo for the notes gate.

**Proposed Addition**:
_(leave blank until discussed)_

**Resolution**: Pending
**Notes**:

---

### 2. Regenerate "simple confirm (reuse)" — choice set and rendering undefined

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Gating & `-y` Orthogonality (gate inventory, line 90)

**Details**:
The gate inventory distinguishes two regenerate gate variants: "the notes-review gate (fresh) / **a simple confirm (reuse)**." The fresh variant maps to the fully-specified `y`/`n`/`e`/`r` `Continue?` menu. The "simple confirm (reuse)" is named but never defined: it is unclear whether it is the same four-choice menu, a reduced set (e.g. `y`/`n` only — `e`/`r` arguably make no sense when reusing existing notes), or some other shape. Its pretty rendering, plain rendering (no per-event table row), default key, and `-y` auto-accept echo are all unspecified.

Because `Prompt(gate)` is documented to return `y`/`n`/`e`/`r` and the engine owns the e/r re-entry loop, the reuse confirm's relationship to that contract needs stating — otherwise an implementer must guess which choices the reuse confirm offers and how the presenter renders a (possibly) different gate.

**Proposed Addition**:
_(leave blank until discussed)_

**Resolution**: Pending
**Notes**:

---

### 3. End-of-run rendering undefined for non-release verbs and for failure runs

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Cross-Verb Rendering (lines 268–274); Per-event rendering table (line 220, "end of run"); The Pretty Layer / The Plain Layer worked examples

**Details**:
The "end of run" event is defined only with a release-shaped, URL-bearing payload: pretty `🌿 released {project} v{X} · {url}` and plain `done: {project} v{X} {url}`. Two in-scope cases are left undefined:

1. **Non-release verbs.** `regenerate` does not publish and has no release URL; `init` produces created/skipped lines, not a versioned release. The Cross-Verb Rendering section specifies their *stage* vocabulary but not their end-of-run line. With `{url}` baked into the only defined end-line format, an implementer cannot render the closing line for `regenerate` (especially `--all`, which narrates multiple version blocks) or `init` without inventing wording.

2. **Failure runs.** Both failure worked examples (lines 186–191, 248–251) end after the `✗`/`unwound`/`warn` lines with no closing brand/`done:` line. This implies the end-of-run line is success-only, but that conditionality is never stated. An implementer needs to know whether `StageFailed` suppresses the end-of-run event entirely, or whether a failure-flavoured closing line should be emitted.

**Proposed Addition**:
_(leave blank until discussed)_

**Resolution**: Pending
**Notes**:

---
