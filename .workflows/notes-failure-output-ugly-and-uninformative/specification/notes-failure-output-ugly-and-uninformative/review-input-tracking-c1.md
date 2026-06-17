---
status: complete
created: 2026-06-17
cycle: 1
phase: Input Review
topic: notes-failure-output-ugly-and-uninformative
---

# Review Tracking: Notes Failure Output Ugly and Uninformative - Input Review

## Findings

### 1. Release-vs-hotfix recommendation and risk rating absent

**Source**: investigation `## Fix Direction` → `### Risk Assessment` (lines 340-351)
**Category**: New topic
**Affects**: New section "Risk & Rollout"

**Details**:
The investigation closes with an explicit Risk Assessment the spec drops entirely:
- **Fix complexity: Low** — mirrors the existing `Output`/`hookFailureOutput` precedent; no new presenter mechanism.
- **Regression risk: Low–Medium** — Low if `padStage` is kept (the chosen Fix 3); Medium only if `padStage` were edited globally. The carrier must preserve `errors.Is(ErrGenerationFailed)` matching and the `context.Canceled` passthrough — both load-bearing AI-seam invariants.
- **Recommended approach: Regular release (not a hotfix)** — the bug degrades diagnosability but causes no data loss.

The rollout recommendation (regular release, not hotfix) is a decision that informs how the work unit is shipped and was deliberately reached in the investigation. The risk framing ties the chosen Fix 3 (keep the gap) directly to the "Low" regression rating, which reinforces why that decision matters. None of this survives into the spec.

**Proposed Addition**:
```markdown
## Risk & Rollout

- **Fix complexity: Low.** Mirrors the existing `StageFailure.Output` / `hookFailureOutput` precedent; no new presenter mechanism is introduced.
- **Regression risk: Low–Medium.** Low given the Fix 3 decision to keep `padStage`; it would only rise to Medium if `padStage` were edited globally (which this spec does not do). The carrier must preserve `errors.Is(ErrGenerationFailed)` matching and the `context.Canceled` passthrough — both load-bearing AI-seam invariants (see Invariants to Preserve).
- **Rollout: regular release, not a hotfix.** The bug degrades diagnosability but causes no data loss.
```

**Resolution**: Approved
**Notes**: Added as a new "Risk & Rollout" section after "Out of Scope" (auto mode).

---

### 2. Fix 1 depends on a documented runner contract that the spec leaves implicit

**Source**: investigation `### Code Trace` (lines 113-119, 146-148) and `### Notes → Synthesis validation` (lines 358-366); confirmed in `internal/runner/runner.go:23-31` and `internal/runner/exec_runner.go` `translateRun`
**Category**: Enhancement to existing topic
**Affects**: "Fix 1 — Carry claude's captured output to `StageFailure.Output` (transport-level)"

**Details**:
The spec states claude's output is "taken from the runner `Result`" but never records the load-bearing precondition that makes Fix 1 possible: the runner has a **documented contract** that on a non-zero exit the `Result` is *still fully populated* (`Stdout`/`Stderr`/`ExitCode`) alongside the non-nil error. The investigation grounds this twice — the runner doc comment (`runner.go:23-31, 40-43`) and the synthesis validation, which verified `exec_runner.go translateRun` builds `Stdout` BEFORE the `*exec.ExitError` branch and returns the populated `res`. This is the seam where the real cause is provably available; the whole fix rests on it.

**Current**:
**Root cause:** `ai.Transport.attempt` returns `"", err` on a non-zero exit, discarding the fully-populated runner `Result` (claude's `Prompt is too long` on stdout). `ai.ErrGenerationFailed` is a bare sentinel with no payload, so nothing downstream can populate `StageFailure.Output` — even though the presenter already knows how to render it.

**Proposed Addition**:
A "Precondition (runner contract)" paragraph inserted after the Fix 1 Root cause paragraph, recording that the runner's documented contract guarantees `Result` is fully populated on a non-zero exit (confirmed by synthesis validation of `translateRun`), so the captured output is guaranteed present at the discard seam.

**Resolution**: Approved
**Notes**: Added as a "Precondition (runner contract)" paragraph in Fix 1 (auto mode).

---

### 3. Full set of battle-tested precedents to mirror is reduced to one

**Source**: investigation `### Code Trace` → "Existing in-codebase precedent the fix should mirror" (lines 157-170) and `### Contributing Factors` (lines 219-221)
**Category**: Enhancement to existing topic
**Affects**: "Fix 1 — Carry claude's captured output…" (the "mirrors the existing `hookFailureOutput` precedent" line)

**Details**:
The spec cites only `hookFailureOutput` as the precedent. The investigation grounds the "this is opt-in to existing mechanism, not new mechanism" framing on FOUR battle-tested precedents, three of which the spec omits:
- `internal/presenter/pretty.go:536-546` already renders a verbatim captured body below the ✗ line, pinned green by `TestPrettyPresenterStageFailedRendersCapturedOutputBelowGlyphLine` (`pretty_test.go:1119`, using the `tag/push` case) — this is the existing test the new engine/notes test must complement, not duplicate.
- `internal/commit/surface.go:26-33` `surfaceOutput` — passes a failed command's captured stderr verbatim as `StageFailure.Output`.
- `internal/commit/run.go:944-958` `pushAfterCommit` — git's stderr travels verbatim in `Warning.Output`.
- `internal/engine/release.go:1559,1587-1597` `hookFailureOutput` — the typed-carrier extraction the spec already names.

Naming the existing pinned presenter test in particular matters: the spec's Testing Requirements say the new test fills "the gap the existing tag/push-only presenter test left uncovered" but never names that existing test, so an implementer cannot locate the seam it is complementing.

**Current**:
This is the **load-bearing fix** — it is what lets the operator see the actual message. The other two facets are polish that ride on it.

**Proposed Addition**:
A "Precedents this fix mirrors" list in Fix 1 enumerating all four precedents (presenter `StageFailed` + its pinned test `TestPrettyPresenterStageFailedRendersCapturedOutputBelowGlyphLine`, `commit/surface.go surfaceOutput`, `commit/run.go pushAfterCommit`, `engine/release.go hookFailureOutput`), framed as opt-in to existing mechanism, with the note that the new engine/notes test complements (not duplicates) the existing pinned presenter test.

**Resolution**: Approved
**Notes**: Added as a "Precedents this fix mirrors" list in Fix 1 (auto mode).

---
