---
status: complete
created: 2026-06-17
cycle: 3
phase: Gap Analysis
topic: notes-failure-output-ugly-and-uninformative
---

# Review Tracking: Notes Failure Output Ugly and Uninformative - Gap Analysis

## Findings

### 1. Output composition rule contradicts itself: "stdout-first concatenation" vs. "stderr only when stdout empty"

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Fix 1, step 1 (line 44) and step 3 (line 46); Acceptance Criteria #1 (line 139)

**Details**:
The spec defines the carrier's `Output` composition twice, and the two statements describe different behaviours when BOTH stdout and stderr are non-empty:

- Step 1 (line 44): "The rendered `Output` is composed **stdout-first** (claude's `Prompt is too long` is emitted on **stdout**, not stderr — see step 3)." — "stdout-first" most naturally reads as a concatenation order, i.e. stdout then stderr.
- Step 3 (line 46): the new helper "reads the carrier's stdout (**including stderr when stdout is empty**)." — this reads as a fallback: use stdout, and only fall back to stderr when stdout is empty (never both).
- Acceptance Criteria #1 (line 139): "populated with claude's captured output (stdout-first) verbatim" — restates "stdout-first" without resolving which rule applies.

These are materially different for the realistic case where a non-zero claude exit writes diagnostic text to BOTH streams: the step-1 reading renders stdout then stderr; the step-3 reading renders stdout only (stderr dropped). An implementer cannot tell whether stderr should be appended after stdout, or used only as a fallback when stdout is empty. The presenter renders `StageFailure.Output` verbatim (`writeNotesBody`), so whatever the engine/carrier packs is exactly what ships — there is no downstream layer that resolves this. This is the one composition rule the whole load-bearing Fix 1 hinges on, and it is the value the new wiring test must assert against; leaving it ambiguous forces a design decision at implementation time and risks the test pinning the wrong behaviour.

A secondary, related under-spec: the spec does not state WHERE the composition lives. Step 1 says the carrier "holds the captured `Stdout` and `Stderr` as distinct fields" and that "the rendered `Output` is composed stdout-first"; step 3 says "the new helper ... reads the carrier's stdout (including stderr when stdout is empty)." So the carrier holds the two streams separate, and the engine helper composes — but if both readings of the rule are reconciled, the helper's exact composition expression still needs to be one settled sentence.

**Proposed Addition**:
Settled composition rule stated once in Fix 1 step 3: non-empty streams included stdout-first then stderr, joined by a single newline (BOTH shown when both present — chosen over stderr-only-fallback because it is more informative). Step 1's composition claim reduced to a pointer; Acceptance #1 aligned.

**Resolution**: Approved
**Notes**: Resolved the stdout-first-concatenation vs stderr-fallback contradiction in favour of concatenation (show both, stdout first). Applied under finding_gate_mode=auto.

---

### 2. Trailing-whitespace / blank-output handling of the verbatim captured Output is unstated

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Fix 1, step 4 (line 47); Per-cause `Output` behaviour (lines 49-52); Acceptance Criteria #1 (line 139)

**Details**:
The spec says claude's captured output is rendered "verbatim" in the `Output` block, and that the presenter's existing `writeNotesBody` does the rendering. `writeNotesBody` writes the body bytes unchanged then appends exactly one trailing newline only when the body is non-empty; an empty body writes nothing (the ✗ line stands alone). Captured subprocess stdout very commonly ends in its own trailing newline (and may be only whitespace, e.g. `"\n"`). Two within-scope edge cases are left to the implementer's judgement:

- If the captured stdout is non-empty but whitespace-only (e.g. a lone newline), the spec's "verbatim" rule plus `writeNotesBody`'s "non-empty → emit + one newline" rule yields a stray blank line under the ✗ glyph. The spec's intended render ("the ✗ line stands alone" for no-output causes, lines 49-52) implies a whitespace-only payload should be treated as empty, but the spec does not say whether the helper trims/empties whitespace-only captured output before assigning it to `Output`.
- The worked examples (`  Prompt is too long`) show no trailing blank line, but claude's real stdout for that message would carry a trailing newline; the spec does not state whether the helper trims a trailing newline before assignment (relying on `writeNotesBody` to re-add exactly one) or passes it through unchanged.

This matters because the new wiring test (Testing Requirements) asserts `StageFailure.Output` population, and the existing pinned presenter test asserts the rendered lines — a mismatch on trailing-whitespace policy between the two will surface as a flaky/contradictory pin. Settling "trim trailing whitespace before assignment; a whitespace-only capture is treated as empty" (or the explicit opposite) removes the guess.

**Proposed Addition**:
Folded into the same settled rule (Fix 1 step 3): each stream is whitespace-trimmed for the emptiness check (a whitespace-only capture counts as empty → contributes nothing; if all empty, Output is empty and the ✗ line stands alone); the composed result's trailing whitespace is trimmed before assignment so there is no dangling blank line; internal content is preserved verbatim, with `writeNotesBody` re-adding exactly one trailing newline.

**Resolution**: Approved
**Notes**: Verified `writeNotesBody` behaviour (pretty.go: emits body + one trailing newline when non-empty, nothing when empty). Policy chosen: trim surrounding whitespace, treat whitespace-only as empty. Applied under finding_gate_mode=auto.

---
