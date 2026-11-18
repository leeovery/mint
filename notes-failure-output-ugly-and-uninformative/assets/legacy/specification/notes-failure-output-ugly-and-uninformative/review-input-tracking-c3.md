---
status: complete
created: 2026-06-17
cycle: 3
phase: Input Review
topic: notes-failure-output-ugly-and-uninformative
---

# Review Tracking: notes-failure-output-ugly-and-uninformative - Input Review

## Findings

### 1. Concise-message guarantee is silent on the unmapped-cause (`causeText` default) path

**Source**: Investigation `Code Trace` step 4 (`internal/notes/resolve.go` `causeText` map) + `Notes / Regenerate surfacing` (line ~381: regenerate's "SHORTER wrap chain … surfaces `GenerateFromRange`'s `"generating notes: %w"` directly rather than always re-wrapping through `abortError`/`causeText`"). The as-built `causeText` `default` branch (`internal/notes/resolve.go:118-119`) returns `failure.Error()` for any cause that is not one of the four known sentinels.

**Category**: Gap/Ambiguity
**Affects**: Fix 2 — Collapse the top-line message (Seam paragraph) and Acceptance Criteria #2

**Details**:
Fix 2 settles the concise-message seam as deriving the phrase "the same mapping `causeText` provides" via an exported `causeText` equivalent / `Message()`-style method. But `causeText` only maps the four known sentinels (`ErrTimeout`, `ErrDiffTooLarge`, `ErrCommandMissing`, `ErrGenerationFailed`); its `default` branch returns `failure.Error()` — i.e. the full wrapped chain. So if a notes/AI failure reaches the surfacing path WITHOUT carrying one of the four recognized sentinels, the chosen seam reproduces the exact human-hostile "render the whole nested `cause.Error()`" symptom Fix 2 is meant to eliminate.

This is not purely hypothetical: the spec itself notes (Scope, "Note on regenerate's wrap chain") that regenerate's fresh path may surface a *shorter* chain that does not always re-wrap through `abortError`/`causeText`, and the investigation flags the same. If that shorter chain ever surfaces a cause that `errors.As`/`errors.Is` does not resolve to a known sentinel, the concise-`Message` guarantee (AC #2: no nested chain, no "failed") would not hold.

The spec enumerates the four causes as a closed set for `Output` behaviour (Fix 1, "Per-cause `Output` behaviour") but never states the display contract for an unmapped/`default` cause. Two clean resolutions are possible and the spec should pick one: (a) state that the four sentinels are exhaustive for any error that can reach the notes surfacing path (so `default` is unreachable — consistent with `ResolveFailure`'s "invoked solely when the generator surfaces a typed failure" contract), making the gap moot by assertion; or (b) if `default` is reachable, define the fallback display behaviour (e.g. still render the short tail, or accept `failure.Error()` only when no sentinel matches) so AC #2 is testable for that case. As written, an implementer extending `failureMessage`/`causeText` has no instruction for the `default` branch and could ship a path that still prints the nested chain.

**Proposed Addition**:
Resolution (a) with a stated defensive-default contract. Added an "Unmapped-cause contract" paragraph to Fix 2: the four sentinels (`ai.ErrGenerationFailed`, `ai.ErrTimeout`, `ai.ErrCommandMissing`, `notes.ErrDiffTooLarge`) are exhaustive for the notes surfacing display (`ResolveFailure` is invoked solely on a typed generator failure; `context.Canceled` propagates separately), so the concise phrase always resolves via one of the four; `causeText`'s `default` branch is a defensive fallback only, not a reachable display path, and AC #2's guarantee is asserted against the four known causes.

**Resolution**: Approved
**Notes**: Verified `ResolveFailure` contract (resolve.go:58-71 "invoked solely when the generator surfaces a typed failure") and `causeText` default (resolve.go:117-119 returns `failure.Error()`). Applied under finding_gate_mode=auto.

---
