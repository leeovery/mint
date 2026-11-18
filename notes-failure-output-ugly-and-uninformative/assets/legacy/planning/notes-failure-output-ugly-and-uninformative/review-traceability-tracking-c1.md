---
status: complete
created: 2026-06-17
cycle: 1
phase: Traceability Review
topic: Notes Failure Output Ugly and Uninformative
---

# Review Tracking: Notes Failure Output Ugly and Uninformative - Traceability

## Result: CLEAN

No findings. The plan is a faithful, complete translation of the specification in both directions.

## Direction 1: Specification → Plan (completeness)

Every specification element has plan coverage with sufficient depth:

| Spec element | Plan coverage |
|--------------|---------------|
| Fix 1 — typed carrier error wrapping `ErrGenerationFailed`, carrying `Stdout`/`Stderr`/`ExitCode` | Task 1-1 (non-zero exit path), Task 1-2 (empty/whitespace body path) |
| Fix 1 — runner-contract precondition (Result populated before `*exec.ExitError` branch) | Task 1-1 Do + Context |
| Fix 1 — `transport.attempt` stops discarding `res`; `Generate` packs carrier after retry | Tasks 1-1, 1-2 Do |
| Fix 1 step 3 — `errors.As` extraction helper + settled composition rule (trim-for-emptiness, stdout-then-stderr single-newline join, trailing-trim, verbatim interior, both-empty→"") | Task 2-2 Do/Acceptance/Tests |
| Fix 1 — no presenter change (StageFailed/writeNotesBody already render Output) | Task 2-3 (explicit no-change directive) |
| Fix 1 — per-cause Output behaviour (only generation-failed carries; timeout/command-missing/diff-too-large empty) | Tasks 1-3, 2-2, 2-3 |
| Fix 2 — concise `Message` via `failureMessage`/exported `causeText` derivation; no leading "notes" prefix; no "failed" | Task 2-1 |
| Fix 2 — unmapped-cause contract (four exhaustive sentinels; default branch defensive only) | Task 2-1 Do/Edge Cases/Context |
| Fix 2 — sub-decision: retain `%w` chain for `errors.Is`/logs | Task 2-1 (explicit "do NOT tear out chain") |
| Fix 3 — keep `padStage` gap; no layout/presenter change | Task 2-3 no-change directives; Phase 2 acceptance |
| Scope — both surfacing sites (`surfaceAndUnwind`, `surface`) fed Output | Task 2-3 |
| Scope — `resetAndAbort` inherits concise Message, out of scope for Output | Task 2-1, Task 2-3 |
| Scope — batch `--all` `reportSkip`/`classifyNotesFailure` out of scope | Task 2-3 (explicit no-touch) |
| Scope — commit carrier preserves `errors.Is` routing, no commit-side render | Task 1-1 Acceptance; Phase 1 Goal |
| Regenerate's shorter wrap chain produces clean phrase + extraction matches | Tasks 2-1, 2-2 |
| Invariant 1 — `errors.Is(ErrGenerationFailed)` still matches | Task 1-1 |
| Invariant 2 — `context.Canceled` passthrough, never carrier-swallowed | Task 1-3 |
| Invariant 3 — transport never imports `config` | Tasks 1-1, 1-2 |
| Invariant 4 — single-retry ownership; carrier only after retry exhausted | Tasks 1-1, 1-2 |
| Invariant 5 — byte-identical success path | Task 1-3 |
| Acceptance Criteria #1–#4 | Phase 2 acceptance + Task 2-3 acceptance |
| Testing Requirements (engine/notes wiring, concise message, both paths, transport, updated pretty_test.go, stream-split) | Tasks 1-1, 1-3, 2-1, 2-3 |
| Out of Scope (byte/token ceiling; batch skip rendering) | Correctly excluded — no task |

Coverage depth verified: each task contains enough detail (settled composition rule spelled out, exact sentinel→phrase mapping, as-built code anchors, invariant assertions) that an implementer would not need to return to the spec.

## Direction 2: Plan → Specification (fidelity / anti-hallucination)

Every piece of plan content traces to a specific specification section:

- All task Problem/Solution/Outcome statements tie to the spec's Fix 1/Fix 2/Fix 3 root-cause and change descriptions.
- The settled composition rule, the four-sentinel exhaustive set, the `padStage`-keep decision, and the per-cause Output behaviour are quoted/paraphrased directly from the spec — none invented.
- Acceptance criteria verify spec requirements (carrier fields, `errors.Is`/`errors.As` behaviour, concise-phrase rule, empty-Output cases, padStage-unchanged), not made-up ones.
- Test names map to the spec's Testing Requirements (transport stdout-on-non-zero-exit, concise message, both surfacing paths, context.Canceled passthrough, updated pretty_test.go assertions, stream-split note).
- As-built code anchors (file paths, function names, approximate line numbers) are implementation scaffolding pointing at real code (verified: `surface`/`surfaceAndUnwind` call sites, `regenerate_batch.go:271` pre-read body-read `surface` vs `:288` `reportSkip` production-failure skip, `classifyNotesFailure`/`reportSkip` skip path) — they constrain implementation toward the spec's named seams, not new requirements.
- No technical approach, behaviour, edge case, or acceptance criterion appears in the plan that is absent from the spec.

## Findings

None.

**Resolution**: Complete — clean traceability, no changes required.
