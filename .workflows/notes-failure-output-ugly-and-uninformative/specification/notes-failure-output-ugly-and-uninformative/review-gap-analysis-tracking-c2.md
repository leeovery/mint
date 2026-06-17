---
status: complete
created: 2026-06-17
cycle: 2
phase: Gap Analysis
topic: notes-failure-output-ugly-and-uninformative
---

# Review Tracking: Notes Failure Output Ugly and Uninformative - Gap Analysis

Cycle 2. The 8 cycle-1 findings were verified applied (Seam paragraph in Fix 2, resetAndAbort third-site note, commit-routing rewording, carrier shape/stdout-first, per-cause Output behaviour, concise-message rule wording, stream-split note, worked-example-spacing note). This cycle reads the current spec as a standalone document and validates each new finding against the as-built code. One remaining finding.

## Findings

### 1. The batch `--all` regenerate path surfaces a notes failure as a `Warn` (not `StageFailed`) and is unaddressed by Fix 1, Fix 2, and Acceptance Criteria #3

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Scope & Affected Surfaces ("the two notes surfacing helpers"); Acceptance Criteria #3 ("Both surfacing paths behave identically — forward release (`surfaceAndUnwind`) and regenerate (`surface`)"); Fix 1 ("Release and regenerate gain the rendered `Output`"); Fix 2.

**Details**:
The spec treats "regenerate" as a single surfacing path — `surface(p, "notes", err)` — and Acceptance Criteria #3 asserts exactly two paths behave identically (`surfaceAndUnwind` and `surface`). But `mint release regenerate` has TWO sub-modes with DIFFERENT notes-failure handling, and only one of them routes through `surface`:

- **Single-version / interactive regenerate** (`regenerate_interactive.go:207`) does `surface(p, "notes", err)` → `StageFailed` → `failureMessage`. This is the path the spec covers. It gains the concise Message (Fix 2) and the Output (Fix 1).
- **Batch `--all` regenerate** (`regenerate_batch.go:288`) does NOT reach `StageFailed`. A per-version notes failure is CAUGHT (skip-and-continue contract) and narrated via `reportSkip(p, res.Tag, classifyNotesFailure(err))`, which emits a non-terminal `presenter.Warn` and records a `skippedVersion{Reason}` for the closing batch summary.

Two concrete consequences the spec does not address:

1. **The batch skip reason still contains "failed".** `classifyNotesFailure` (regenerate_batch.go:389) returns `reasonNotesFailed = "notes generation failed"` (regenerate_batch.go:340) for a generation-failed cause — the exact word Fix 2's concise-message rule and Acceptance Criteria #2 forbid in the failure display. This is a SEPARATE cause-phrase mapping from `causeText`/`failureMessage`; it does not route through the Fix 2 seam, so the batch summary and per-version `Warn.Message` keep the verbose "failed" wording even after Fix 2 lands. The spec's "Both surfacing paths behave identically" claim is therefore incomplete: a third regenerate display string is left inconsistent with the rule the spec sets for the other two.

2. **The batch skip carries no captured Output.** `reportSkip` builds `presenter.Warning{Label, Message}` with no `Output`. The captured-claude-output improvement (the spec's load-bearing Fix 1) accrues to single-version regenerate but NOT to batch — even though `Warning.Output` is an existing render site (the spec itself cites `commit/run.go pushAfterCommit`'s `Warning.Output` as a verbatim-output precedent). So a `--all` run whose version trips a real AI error (e.g. `Prompt is too long`) still shows the operator an uninformative one-line skip with the real cause discarded — the precise defect this work unit exists to fix.

An implementer reading Scope ("Release and regenerate gain the rendered `Output`") and Acceptance Criteria #3 ("Both surfacing paths") cannot tell whether the batch path is in or out of scope, or whether its `classifyNotesFailure`/`reportSkip` should be touched. The spec needs an explicit decision: either (a) declare the batch skip out of scope and say why (it is a non-terminal Warn on the skip-and-continue contract, not a terminal `StageFailed`), keeping `classifyNotesFailure`/`reportSkip` untouched; or (b) include it — route the carrier's captured output into `Warning.Output` and align `classifyNotesFailure`'s phrase with the no-"failed" rule. Cycle 1's finding #2 accounted for the THIRD `StageFailed` builder (`resetAndAbort`); this is a FOURTH notes-failure display site that is not a `StageFailed` at all and is wholly unmentioned.

**Proposed Addition**:
Declare the batch `--all` per-version skip `Warn` path **out of scope** (option a), with rationale (deliberate non-terminal skip-and-continue UX, distinct from the terminal `StageFailed` render this spec targets) and two promotable residual limitations recorded explicitly: (1) `classifyNotesFailure`'s "notes generation failed" reason retains "failed" independent of Fix 2's `StageFailed`-only rule; (2) the batch `Warn` carries no captured `Output`, so a `--all` run hitting a real AI error shows a one-line skip without claude's output (routing into `Warning.Output` is the natural future enhancement). Tighten Acceptance #3 to name the in-scope `StageFailed` paths (forward `surfaceAndUnwind` + single-version/interactive `surface`) and exclude the batch skip. Refine the in-scope `surface` bullet to note batch uses it only for the deterministic pre-read body *read* failure, not the notes-production failure.

**Resolution**: Approved
**Notes**: Verified against code — `regenerate_interactive.go:207` `surface(p, "notes", err)` vs `regenerate_batch.go:288` `reportSkip(p, res.Tag, classifyNotesFailure(err))`; `reportSkip` (regenerate_batch.go:381) emits `p.Warn(presenter.Warning{Label, Message})` with no Output and returns `skippedVersion{Reason}` for `batchSummary`; `classifyNotesFailure` (regenerate_batch.go:389) returns `reasonNotesFailed = "notes generation failed"` (regenerate_batch.go:340) — contains "failed"; batch's `surface` call (regenerate_batch.go:271) is the deterministic pre-read body read-failure path, distinct from the notes-production failure at :288. The batch loop intentionally overrides on_notes_failure=abort to skip-and-continue, so the Warn vs StageFailed split is deliberate as-built. **Scope decision (out of scope) made under finding_gate_mode=auto with the sensible default matching the investigation's scope and the max_diff_lines precedent; surfaced to the user at completion sign-off for override.** Applied: Scope "Fourth display site" note, Out of Scope bullet, Acceptance #3 tightening, in-scope `surface` bullet refinement.

---
