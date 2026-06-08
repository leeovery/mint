---
status: in-progress
created: 2026-06-08
cycle: 4
phase: Gap Analysis
topic: mint-release-tool
---

# Review Tracking: mint-release-tool - Gap Analysis

*(Cycle 4. Prior cycles' 21 findings applied and excluded from scope. Systematic re-read found the spec internally complete and planning-ready except for one narrow, not-previously-raised semantic interaction: the review gate's `[r] regenerate with context` choice on a notes path that deliberately never calls the AI. This section explicitly owns "the four semantic choices and their effects," so the collision is in-scope here and not a deferred rendering concern.)*

## Findings

### 1. Review-gate `[r] regenerate with context` is undefined on the no-AI notes paths (first-release / degenerate / `--no-ai`)

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Interactive Confirmation & Notes Review — gate menu + `r` choice (lines 448, 454); Stage 4 — Notes-path precedence (lines 264–273)

**Details**:
The interactive gate (Stage 4 / Interactive Review) presents four choices on the generated body, and the section explicitly states it "owns the four semantic choices and their effects" (rendering is deferred to the Presentation spec). The `[r] regenerate with context` choice is defined as: mint "asks for a one-time context line, appends it to the prompt, **re-runs the AI**, and shows the result again (loops until happy)" (line 454).

But the Notes-path precedence subsection (cycle-1 #9, lines 264–273) establishes three paths that **deliberately never call the AI** and still produce a body shown at the gate:
1. First release → fixed body "Initial release."
2. Degenerate diff → stub entry.
3. `--no-ai` → fallback (commit-subject list).

On any of these runs an interactive (non-`-y`) user still reaches the gate with a body shown, and `[r]` is the one choice whose defined effect ("re-run the AI") directly contradicts the run's deliberate no-AI decision. The spec never says what `[r]` does here — offer-and-no-op, hide the choice, error, or (on the `--no-ai` case specifically) actually invoke the AI despite the flag. The other three choices are unaffected: `y`/`n` are body-agnostic, and `e` edits verbatim text regardless of source. Only `r` collides.

This is a semantic question the section claims to own (which choices are valid and what they do), not a rendering one, so it isn't covered by the deferred Presentation spec. Cycle-1 #9 fixed *which body is produced* across the guards but did not address *which gate choices remain valid* once a no-AI body is in hand. An implementer must currently invent the behaviour (and could, for `--no-ai`, build an `r` that silently defies the flag).

**Proposed Addition**:
{leave blank until discussed — likely: on the three no-AI notes paths the gate offers only `y`/`n`/`e` (the AI-dependent `r` is omitted, since there is no AI invocation to nudge); for `--no-ai` specifically, choosing to regenerate would require the user to re-run without `--no-ai` rather than `r` silently overriding the flag.}

**Resolution**: Pending
**Notes**:

---
