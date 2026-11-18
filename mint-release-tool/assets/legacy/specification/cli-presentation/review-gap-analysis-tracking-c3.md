---
status: complete
created: 2026-06-08
cycle: 3
phase: Gap Analysis
topic: cli-presentation
---

# Review Tracking: cli-presentation - Gap Analysis

## Findings

### 1. Captured underlying-command output on failure — ownership, event, and plain rendering undefined

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Spinner Lifecycle (pretty only)" (line 265, "On failure, `mint` prints the captured output below the `✗` line"); "The `Presenter` Seam" (Event payload principle; `StageFailed`); "Per-event rendering" table (`StageFailed` row, line 221); "The Plain Layer" / "Output streams"

**Details**:
The spec states that git/claude/gh chatter is captured (not streamed through the spinner) and that "On failure, `mint` prints the captured output below the `✗` line." This is the one failure-path behaviour that is asserted but never reconciled with the rest of the design:

1. **Ownership is unstated.** The phrase "`mint` prints" does not say whether the captured output routes through the `Presenter` (as an event) or is emitted directly by the engine. Every analogous "extra text on a transition" concern in this spec has been deliberately resolved *toward* the presenter — the `Warn` label/message, the `-y` auto-accept echo, and the forbidden-combination failure were all made rendered events under the established principle "narration → presenter, the engine never prints." Captured failure output is the lone exception left open, so an implementer cannot tell whether `StageFailed` carries the captured output in its payload, whether a separate event exists, or whether the engine bypasses the seam and prints directly (which would contradict the render-only-presenter discipline).

2. **Plain rendering is undefined.** The behaviour appears only in the *pretty-only* Spinner Lifecycle section ("below the `✗` line"), but captured command output is not a pretty-only concern — a failing `git push` / `claude` / `gh` call produces the same diagnostic chatter regardless of mode. The per-event `StageFailed` plain row shows only `{stage}: FAILED - {message}` with no captured output, and the plain failure worked example (lines 255–258) shows none either. An implementer cannot tell whether plain mode emits the captured output at all, and if so, how it is introduced/delimited (plain deliberately delimits the notes block so a reader can slice it out — captured multi-line command output has no analogous contract).

3. **Stream placement is unstated for this specific text.** The stream contract sends errors/warnings to stderr. Captured underlying-command output on failure is diagnostic-but-not-itself-an-error-line; whether it follows the `StageFailed` line on stdout (narration), goes to stderr, or both is not addressed.

These are behavioural/ownership questions of the same kind already closed for `Warn`, auto-accept, and the forbidden-combination failure — not deferred wording/signature detail. Left open, an implementer must invent the event shape, the plain-mode behaviour, and the stream placement, and risks violating the render-only-presenter discipline by printing engine-side.

**Proposed Addition**:
Added an Event payload principle bullet: `StageFailed` carries the error message and the captured underlying-command output; the presenter renders it in both modes (pretty below the `✗` line, plain below the `FAILED` line wrapped in a sliceable `--- output --- … --- end output ---` delimiter), never engine-printed; captured body is stdout narration while the one-line FAILED/error summary also goes to stderr (the multi-line body is not duplicated to stderr).

**Resolution**: Approved
**Notes**: Auto-approved. Closes the last ownership/rendering gap, consistent with the Warn/auto-accept/forbidden-combination resolutions.

---
