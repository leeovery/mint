---
status: in-progress
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

**Resolution**: Pending
**Notes**:

---

### 2. `Warn(msg)` event has no `label`, but both renderings require one

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "The `Presenter` Seam" (illustrative method set: `Warn(msg)`); "Per-event rendering" table (`Warn` row); pretty failure example (`⚠ post_release  hook failed …`)

**Details**:
Both renderings of a warning are label-prefixed: pretty `⚠ {label}  {message}` (worked example: `⚠ post_release  hook failed (tag is already published): …`) and plain `{label}: WARN - {message}`. But the illustrative event is `Warn(msg)` — a single message, no label. The implementer cannot produce `{label}` from `{msg}` alone. Either `Warn` needs a label parameter (`Warn(label, msg)`) or the convention is that the caller embeds the label in `msg` and the presenter must not re-format. The two table rows imply structured `{label}`/`{message}` fields, contradicting the single-arg signature. This needs resolving so the presenter and every `Warn` call site agree on the shape.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 3. Auto-unwind is a rendered event with no method in the interface

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "The `Presenter` Seam" (illustrative method set); "The Pretty Layer" (failure example `↩ unwound`); "Per-event rendering" table (auto-unwind row); "Status glyphs" (`↩` auto-unwind)

**Details**:
Auto-unwind has a dedicated glyph (`↩`), a pretty rendering (`↩ unwound  {what it undid} — repo clean`), a plain rendering (`unwound: {what}; repo clean`), and a table row — so it is a first-class presentation event. But it is absent from the illustrative method set (`StageStarted/StageSucceeded/StageFailed/Warn/ShowPlan/ShowNotes/Prompt`), and it is clearly not a `StageFailed` (it follows the failure as a separate `↩` line). The implementer needs to know this is a distinct event (e.g. `Unwound(summary)`) carrying the "what it undid" payload. While "exact surface settled at implementation" covers method names, an entirely missing event with its own glyph and two renderings is a completeness gap, not a naming detail.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 4. The `e`/`r` gate re-entry loop straddles the engine/presenter boundary without an ownership statement

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Gating & `-y` Orthogonality" (Regenerate / edit re-entry); "The `Presenter` Seam" (`Prompt(gate) → choice`); "The Pretty Layer" (Review gate)

**Details**:
`Prompt(gate) → choice` returns a single choice. The spec says after `e` (edit in `$EDITOR`) or `r` (regenerate-with-context) the flow "loops back to the same `Continue?` gate … until `y`/`n`," and rendering "re-prints the notes block + gate below (it scrolls)." But it never states who owns the loop:
- If the presenter loops internally on `e`/`r`, then `Prompt` must itself perform the edit/regenerate (calling `$EDITOR`, triggering regeneration) — which violates "the engine never touches … TTY state … the presenter is render-only" because regenerate-with-context is engine work (a claude call).
- If the engine loops (re-calling `Prompt` after handling `e`/`r` and re-showing notes via `ShowNotes`), then `Prompt` returns `e`/`r` to the engine and the "loops back to the same gate" + "re-prints the notes block" behaviour is an engine orchestration, with the presenter only re-rendering on each pass.

The latter is consistent with the seam's render-only discipline, but the spec asserts the loop and the re-print behaviour without saying which component drives them. An implementer must guess the division, and the guess determines whether `Prompt` blocks on `$EDITOR`/claude or returns immediately. Also: the spinner-stop-around-`$EDITOR` note ("stopped before handing off, resumed after") presumes someone stops the spinner across the edit — this only works if ownership is pinned down.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 5. Elapsed time `({elapsed})` — source unspecified; conflicts with plain's "no start line for short stages"

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Per-event rendering" table (`StageSucceeded` row `({elapsed})`); "The Plain Layer"; worked examples (`prep … (2.3s)`, `notes … (1.1s)`)

**Details**:
Pretty `StageSucceeded` renders `({elapsed})` and the examples show `(2.3s)`/`(1.1s)`. The spec never says whether elapsed is (a) measured by the presenter between `StageStarted` and `StageSucceeded`, or (b) supplied by the engine in the `StageSucceeded` payload. This matters:
- If the presenter measures it, the plain presenter must record a start time even for stages it does *not* print a start line for — workable, but only the long stages in the examples show elapsed, so it's unclear whether short stages (`version`, `preflight`, `record`) intentionally omit elapsed or just happen to be fast.
- If the engine supplies it, `StageSucceeded(name)` needs an elapsed/detail payload.

Either way the rule for *which* stages show `({elapsed})` (only long/blocking? all?) is implied by examples but never stated, so an implementer would guess. The examples consistently show elapsed only on the two long stages (`prep`, `notes`), suggesting elapsed is shown only for long stages — but this is inference, not specification.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 6. Forbidden-combination failure (non-TTY stdin without `-y`) has no specified presentation path

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Gating & `-y` Orthogonality" (Forbidden-combination rule)

**Details**:
The rule states `mint` "fails loud" with the message `"not a TTY — pass -y to run unattended"`. The message text is given, and stderr placement follows the stream contract. But the spec does not say how this surfaces through the presentation layer: is it a `StageFailed`, a `Warn`, or a bare pre-presenter error printed before any `Presenter` is even engaged? Because this failure is fundamentally about input (stdin), and render mode is selected on stdout, it can occur in either pretty or plain output mode — so an implementer needs to know whether it routes through the `Presenter` (and thus gets styled in pretty) or is emitted directly. Minor, but it's the one error the presentation spec itself introduces, so its rendering should be pinned.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 7. `Prompt(gate)` input handling vs. the `-y`/non-TTY skip — what reaches `Prompt`

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Gating & `-y` Orthogonality" (Gate input handling, Forbidden-combination rule); "Per-event rendering" (review gate row: "not shown — non-TTY ⇒ -y required ⇒ gate skipped")

**Details**:
Under `-y`, the gate is skipped and plain emits `notes: accepted (-y)`. The table says the gate is "not shown" in plain. This implies the engine decides to skip `Prompt` entirely when `-y` is set, and instead the *presenter* emits the `notes: accepted (-y)` echo — but there is no event for "gate auto-accepted." Is `notes: accepted (-y)` produced by the presenter (needs an event/method, e.g. the engine calling something) or printed by the engine directly? Given the seam discipline (engine emits semantic events, presenter renders), there should be an event like `GateAutoAccepted` or the `-y` echo is engine-printed text — which would contradict "narration → presenter." The pretty side has no stated equivalent echo line under `-y` (the worked pretty example shows the full menu, i.e. interactive). Clarify: under `-y`, does pretty also skip the menu (and if so does it print an accept echo)? The orthogonality section says a human with `-y` "still gets full styling," but does "full styling" include showing then auto-answering the menu, or skipping it? The `-y alignment` note ("identical outcome to pressing Enter") suggests the gate is skipped, not auto-pressed — leaving the pretty-mode `-y` rendering undefined.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 8. `ShowPlan` plain one-liner derivation is illustrative only

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Per-event rendering" table (`ShowPlan` row); plain example (`plan: commit changelog+version; tag v1.4.0; push --atomic; publish github`)

**Details**:
Pretty renders `ShowPlan(plan)` as a multi-line bulleted block; plain renders `plan: {semicolon-joined one-liner}`. The example shows the plan abbreviated (`commit CHANGELOG.md + bin/acme` becomes `commit changelog+version`) — i.e. the plain one-liner is not a mechanical join of the pretty bullets, it's a separately abbreviated form. The spec doesn't define whether the `plan` payload carries both a verbose and terse form, or whether each presenter abbreviates independently. If the presenter must abbreviate (`CHANGELOG.md + bin/acme` → `changelog+version`), that abbreviation logic is unspecified and would be guessed. This is lower-severity than the event-signature gaps but still forces an implementer decision about what `plan` contains.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---
