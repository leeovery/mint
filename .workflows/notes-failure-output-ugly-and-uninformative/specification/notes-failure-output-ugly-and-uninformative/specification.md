# Specification: Notes Failure Output Ugly and Uninformative

## Specification

## Problem Statement

When AI notes generation fails during `mint release` (and `mint release regenerate`), the rendered failure is both **uninformative** and **ugly**:

```
✗ notes      notes generation failed (AI returned empty/invalid notes after retry): generating notes: ai generation failed
```

Three independent defects converge on this one line:

1. **The real cause is discarded.** `claude`'s actual captured output (e.g. `Prompt is too long` on stdout from a non-zero exit) never reaches the screen. `StageFailure.Output` ends up empty, so there is nothing actionable below the ✗ line.
2. **The top-line message is a redundant `%w` chain.** "generation failed" appears twice and "generating notes" once across one line, and the message restates the stage name ("notes" appears twice).
3. **The failure line restates the stage across the `padStage` gap** — `✗ notes      notes …`.

## Target Behaviour

The failure should render claude's verbatim captured output below the ✗ line, with a single concise top-line message that does not restate the stage name or repeat "failed":

```
✗ notes  AI returned empty/invalid notes after retry
  Prompt is too long
```

- The top-line `Message` is one concise cause phrase.
- `claude`'s captured stdout/stderr is shown verbatim in the `Output` block below.
- The failure-line column layout (`padStage` gap) is settled deliberately, with pinned presenter tests updated to match.

## Fix 1 — Carry claude's captured output to `StageFailure.Output` (transport-level)

This is the **load-bearing fix** — it is what lets the operator see the actual message. The other two facets are polish that ride on it.

**Root cause:** `ai.Transport.attempt` returns `"", err` on a non-zero exit, discarding the fully-populated runner `Result` (claude's `Prompt is too long` on stdout). `ai.ErrGenerationFailed` is a bare sentinel with no payload, so nothing downstream can populate `StageFailure.Output` — even though the presenter already knows how to render it.

**Change:**

1. **Upgrade `ai.ErrGenerationFailed` into a typed carrier error** (e.g. `*ai.GenerationError`) that:
   - **wraps** the sentinel, so `errors.Is(err, ErrGenerationFailed)` still matches (callers that branch on the three sentinels are unaffected); and
   - **carries** claude's captured stdout/stderr taken from the runner `Result`.
2. **`transport.attempt` stops discarding `res`** on the error path; **`Generate` packs the captured output** into the carrier. The `Generate` signature is unchanged — the captured output travels on the returned error, not via a new return value.
3. **The engine mirrors the existing `hookFailureOutput` precedent**: a helper extracts the captured output from the error, and **both** notes surfacing sites set `StageFailure.Output` to it — `surfaceAndUnwind("notes", …)` (forward release) **and** `surface("notes", …)` (regenerate).
4. **No presenter change is needed for this facet** — `StageFailed` (`pretty.go`) already renders a verbatim captured body below the ✗ line via `writeNotesBody` when `StageFailure.Output != ""`.

**Why transport-level:** fixing the discard at the transport means `mint release`, `mint release regenerate`, **and** `mint commit` all benefit from one seam — they share the same `ai.Transport`. The transport stays content-agnostic (never imports `config`).

**Option chosen:** typed carrier error (mirrors `*hooks.HookError`, keeps `Generate`'s signature and `errors.Is` routing intact) **over** returning the captured output as a separate return value (which would churn every call site).

## Fix 2 — Collapse the top-line message to one concise cause phrase

**Root cause:** the presenter-facing `Message` is the entire nested `%w` chain. Three layers each prepend their own text (`abortError` → `generate.go` wrap → `ErrGenerationFailed`), and the presenter faithfully renders the whole concatenation as the display string.

**Change:** the presenter-facing `Message` shows only the short cause phrase that `causeText` already produces (e.g. `ErrGenerationFailed` → "AI returned empty/invalid notes after retry"). The message must **not** restate the stage name and must **not** repeat "failed". The verbose detail (claude's captured output) lives in the Fix-1 `Output` block, not in the top line.

Target render for the reported case:

```
✗ notes  AI returned empty/invalid notes after retry
  Prompt is too long
```

**Sub-decision (settled here):** the `%w` wrapping chain (`abortError`/`generate.go` wrap) is **retained** for `errors.Is` matching and logs — it is correct Go hygiene and load-bearing for sentinel routing. What changes is that the **display `Message` is separated from the matchable error**: the surfacing path derives the concise display phrase rather than rendering the full nested `cause.Error()`. We do **not** tear out the error chain.

**Option chosen:** concise phrase from `causeText` (CHOSEN) **over** continuing to render the full nested chain. The nested chain is correct for `errors.Is`/logs but human-hostile as a display string; separating the matchable error from the display message is the fix.

---

## Working Notes
