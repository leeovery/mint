---
status: complete
created: 2026-06-08
cycle: 1
phase: Gap Analysis
topic: cli-presentation
---

# Review Tracking: cli-presentation - Gap Analysis

## Findings

### 1. `StageStarted` carries no long/blocking signal, yet plain rendering depends on one

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "The Plain Layer" (Contract → "Start line for long/blocking stages only"); "Per-event rendering" table (`StageStarted` row); "The `Presenter` Seam" (illustrative method set)

**Details**:
The plain contract is explicit: short stages emit nothing on start; "long/blocking stages emit a terse start line." The per-event table repeats this. But the illustrative interface is `StageStarted(name)` — name only. An implementer cannot derive "is this stage long/blocking?" from a name string without either (a) a hardcoded list of stage names inside the plain presenter, or (b) an extra parameter on the event (e.g. `StageStarted(name, blocking bool)`).

The spec says the exact surface is settled at implementation, but this is a *behavioural* decision, not just a method signature: where does the long/blocking classification live — in the engine (which knows it blocks on claude/build) or hardcoded in the presenter? Leaving it open forces the implementer to make a design decision that affects both presenter implementations and the engine's call sites. Notably "long" is engine knowledge (the engine knows it's about to shell out to claude or run a build hook), arguing for the event to carry it — but the spec never states this.

**Proposed Addition**:
Added an "Event payload principle" subsection to the `Presenter` Seam stating the engine supplies every datum the renderings consume, with a bullet: `StageStarted` carries the long/blocking flag (engine knowledge); plain uses it to decide the start line, pretty always shows a spinner; no hardcoded stage-name list in the presenter.

**Resolution**: Approved
**Notes**: Resolved via the engine-supplies-payload principle. Auto-approved.

---

### 2. `Warn(msg)` event has no `label`, but both renderings require one

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "The `Presenter` Seam" (illustrative method set: `Warn(msg)`); "Per-event rendering" table (`Warn` row); pretty failure example (`⚠ post_release  hook failed …`)

**Details**:
Both renderings of a warning are label-prefixed: pretty `⚠ {label}  {message}` (worked example: `⚠ post_release  hook failed (tag is already published): …`) and plain `{label}: WARN - {message}`. But the illustrative event is `Warn(msg)` — a single message, no label. The implementer cannot produce `{label}` from `{msg}` alone. Either `Warn` needs a label parameter (`Warn(label, msg)`) or the convention is that the caller embeds the label in `msg` and the presenter must not re-format. The two table rows imply structured `{label}`/`{message}` fields, contradicting the single-arg signature. This needs resolving so the presenter and every `Warn` call site agree on the shape.

**Proposed Addition**:
Bullet under the Event payload principle: `Warn` carries structured `label` + `message`; the presenter does not parse a label out of a single string.

**Resolution**: Approved
**Notes**: Resolved via the engine-supplies-payload principle. Auto-approved.

---

### 3. Auto-unwind is a rendered event with no method in the interface

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "The `Presenter` Seam" (illustrative method set); "The Pretty Layer" (failure example `↩ unwound`); "Per-event rendering" table (auto-unwind row); "Status glyphs" (`↩` auto-unwind)

**Details**:
Auto-unwind has a dedicated glyph (`↩`), a pretty rendering (`↩ unwound  {what it undid} — repo clean`), a plain rendering (`unwound: {what}; repo clean`), and a table row — so it is a first-class presentation event. But it is absent from the illustrative method set (`StageStarted/StageSucceeded/StageFailed/Warn/ShowPlan/ShowNotes/Prompt`), and it is clearly not a `StageFailed` (it follows the failure as a separate `↩` line). The implementer needs to know this is a distinct event (e.g. `Unwound(summary)`) carrying the "what it undid" payload. While "exact surface settled at implementation" covers method names, an entirely missing event with its own glyph and two renderings is a completeness gap, not a naming detail.

**Proposed Addition**:
Added `Unwound(summary)` to the illustrative method set and a payload-principle bullet declaring it a first-class event (not a `StageFailed`) carrying the "what it undid" summary.

**Resolution**: Approved
**Notes**: Auto-approved.

---

### 4. The `e`/`r` gate re-entry loop straddles the engine/presenter boundary without an ownership statement

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Gating & `-y` Orthogonality" (Regenerate / edit re-entry); "The `Presenter` Seam" (`Prompt(gate) → choice`); "The Pretty Layer" (Review gate)

**Details**:
`Prompt(gate) → choice` returns a single choice. The spec says after `e` (edit in `$EDITOR`) or `r` (regenerate-with-context) the flow "loops back to the same `Continue?` gate … until `y`/`n`," and rendering "re-prints the notes block + gate below (it scrolls)." But it never states who owns the loop. The render-only-presenter discipline implies the engine drives the loop, but the spec asserts the loop and re-print behaviour without saying which component drives them.

**Proposed Addition**:
Rewrote the re-entry paragraph: the engine owns the loop; `Prompt` is render-only and returns the choice; on `e`/`r` the engine performs the edit/regenerate work and re-calls `ShowNotes`/`Prompt`; the engine also stops/resumes the pretty spinner around `$EDITOR`.

**Resolution**: Approved
**Notes**: Auto-approved.

---

### 5. Elapsed time `({elapsed})` — source unspecified; conflicts with plain's "no start line for short stages"

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Per-event rendering" table (`StageSucceeded` row `({elapsed})`); "The Plain Layer"; worked examples (`prep … (2.3s)`, `notes … (1.1s)`)

**Details**:
Pretty `StageSucceeded` renders `({elapsed})` and the examples show `(2.3s)`/`(1.1s)`. The spec never says whether elapsed is measured by the presenter or supplied by the engine, nor which stages show it. The examples consistently show elapsed only on the two long stages (`prep`, `notes`), suggesting elapsed is shown only for long stages — but this is inference, not specification.

**Proposed Addition**:
Payload-principle bullet: `StageSucceeded` carries detail + elapsed, both supplied by the engine; pretty renders `({elapsed})` on long/blocking stages only (same flag as `StageStarted`); short stages render detail without elapsed.

**Resolution**: Approved
**Notes**: Auto-approved.

---

### 6. Forbidden-combination failure (non-TTY stdin without `-y`) has no specified presentation path

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Gating & `-y` Orthogonality" (Forbidden-combination rule)

**Details**:
The rule states `mint` "fails loud" with a given message and stderr placement. But the spec does not say how this surfaces through the presentation layer: a `StageFailed`, a `Warn`, or a bare pre-presenter error. Because this failure is about input (stdin) while render mode is selected on stdout, it can occur in either output mode — so an implementer needs to know whether it routes through the `Presenter` or is emitted directly.

**Proposed Addition**:
Extended the forbidden-combination rule: the failure surfaces through the `Presenter` (rendered as a failure — styled in pretty, terse in plain — since render mode is selected on stdout independently of the stdin problem) and also goes to stderr per the stream contract.

**Resolution**: Approved
**Notes**: Auto-approved.

---

### 7. `Prompt(gate)` input handling vs. the `-y`/non-TTY skip — what reaches `Prompt`

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Gating & `-y` Orthogonality" (Gate input handling, Forbidden-combination rule); "Per-event rendering" (review gate row: "not shown — non-TTY ⇒ -y required ⇒ gate skipped")

**Details**:
Under `-y`, the gate is skipped and plain emits `notes: accepted (-y)`. Is that echo produced by the presenter (needs an event) or printed by the engine directly? The pretty side has no stated `-y` echo, and it's unclear whether pretty shows-then-auto-answers the menu or skips it. The `-y alignment` note ("identical outcome to pressing Enter") suggests skip, leaving the pretty-mode `-y` rendering undefined.

**Proposed Addition**:
Payload-principle bullet: gate auto-accept under `-y` is a rendered event, not engine-printed text. Plus a "Pretty under `-y`" paragraph in the Gating section: `-y` skips the gate (menu not drawn); the auto-accept renders via the event (pretty `✓ notes  accepted (-y)`, plain `notes: accepted (-y)`).

**Resolution**: Approved
**Notes**: Auto-approved.

---

### 8. `ShowPlan` plain one-liner derivation is illustrative only

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Per-event rendering" table (`ShowPlan` row); plain example (`plan: commit changelog+version; tag v1.4.0; push --atomic; publish github`)

**Details**:
Pretty renders `ShowPlan(plan)` as a multi-line bulleted block; plain renders `plan: {semicolon-joined one-liner}`. The example abbreviates (`commit CHANGELOG.md + bin/acme` → `commit changelog+version`), so the plain one-liner is not a mechanical join of the pretty bullets. The spec doesn't define whether `plan` carries both forms or whether each presenter abbreviates independently.

**Proposed Addition**:
Payload-principle bullet: `ShowPlan` carries structured plan steps (verb + target), not pre-formatted text; pretty bullets them, plain joins them; no separate verbose/terse payload; the example abbreviations are illustrative wording.

**Resolution**: Approved
**Notes**: Auto-approved.

---
