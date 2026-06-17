# Investigation: Notes-Failure Output Ugly and Uninformative

## Symptoms

### Problem Description

**Expected behavior:**
When AI notes generation fails during `mint release`, the operator should see
claude's actual captured output verbatim (e.g. `Prompt is too long`) under the
✗ line, via a populated `StageFailure.Output`. The top-line message should
collapse to one concise sentence that does not restate the stage name or repeat
"failed" three times. The failure-line layout (whether to keep the `padStage`
gap for failures) should be settled, with the pinned presenter tests updated to
match.

**Actual behavior:**
The failure renders as a single line:

```
✗ notes      notes generation failed (AI returned empty/invalid notes after retry): generating notes: ai generation failed
```

- "failed" appears three times.
- The real AI cause (claude's stdout/stderr) is discarded.
- `StageFailure.Output` ends up empty, so there is nothing actionable below
  the ✗ line.
- The message restates the stage name — "notes" appears twice across a wide
  `padStage` gap.

### Manifestation

- Failure line is visually ugly and uninformative.
- Operator is left guessing at the true cause (in the surfacing case,
  `claude -p --model sonnet` exited non-zero with stdout `Prompt is too long`,
  which never reached the screen).

### Reproduction Steps

1. Run `mint release` in a repo where the notes diff is large enough to exceed
   the model context window (the surfacing case: `v0.0.2..HEAD` diff ~867 KB,
   mostly committed `.workflows/` and `.tick/` artifact trees).
2. AI notes generation invokes `claude -p` which exits non-zero with
   `Prompt is too long` on stdout.
3. Observe the rendered failure line.

**Reproducibility:** Always (given an AI command that exits non-zero / returns
empty output).

### Environment

- **Affected environments:** Local CLI (`mint release`).
- **Browser/platform:** Terminal (pretty renderer).
- **User conditions:** Any notes-generation failure path — non-zero exit,
  empty/invalid AI body after retry.

### Impact

- **Severity:** Medium (no data loss; degraded diagnosability — operator can't
  see why notes failed).
- **Scope:** Any operator who hits a notes-generation failure.
- **Business impact:** Trust / usability — the tool looks broken and gives no
  actionable signal.

### References

- Seed: `seeds/2026-06-14-notes-failure-output-ugly-and-uninformative.md`
  (inbox:bug, captured while dogfooding `mint release` on mint itself).
- Discovery session: `discovery/session-001.md`.

### Out of Scope (carried from discovery)

The line-based `max_diff_lines` guard (default 50000) did not catch an
8.6k-line but ~867 KB byte-dense diff, suggesting a byte/token-aware ceiling.
This was **deliberately left out of scope** for this work unit — it's an
enhancement to a guard, not part of how a failure is rendered. Noted in the
seed; can be promoted to its own work later.

---

## Analysis

### Initial Hypotheses

Three facets of one failure-rendering path (from the seed / discovery):

1. **The real cause is discarded.** The transport
   (`internal/ai/transport.go`) maps a non-zero exit or empty body to
   `ErrGenerationFailed` and drops claude's captured stdout/stderr; the notes
   layer (`internal/notes/resolve.go:101`, `internal/notes/generate.go:185`)
   re-wraps it into a redundant chain, and `StageFailure.Output` ends up empty.
2. **The top-line message is redundant** — "failed" appears three times and
   restates the stage name.
3. **The failure line is visually ugly** — `padStage` alignment in
   `internal/presenter/pretty.go` (`StageFailed`) puts "notes" twice across a
   wide gap. The gap is intentional column alignment shared with `✓` success
   lines and is pinned in presenter tests (`pretty_test.go`,
   `gate_forbidden_test.go`, `askline_test.go`), so changing it is a contract
   change to be settled deliberately.

### Code Trace

_To be filled during Step 5 (Code Analysis)._

### Root Cause

_To be filled during Step 6._

---

## Fix Direction

_To be filled during Step 8 (Findings Review & Fix Discussion)._

---

## Notes

Investigation initialized from discovery carrier (manifest description + session-001 log + seed).
