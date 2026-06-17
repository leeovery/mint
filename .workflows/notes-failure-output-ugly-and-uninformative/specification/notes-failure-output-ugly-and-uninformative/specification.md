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

---

## Working Notes
