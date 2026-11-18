---
status: complete
created: 2026-06-17
cycle: 1
phase: Plan Integrity Review
topic: Notes Failure Output Ugly and Uninformative
---

# Review Tracking: Notes Failure Output Ugly and Uninformative - Integrity

## Findings

No findings. The plan meets structural quality and implementation-readiness standards.

## Review Notes

Reviewed the plan end-to-end (planning.md, phase-1-tasks.md, phase-2-tasks.md, and the
six tick tasks) against every integrity criterion, and cross-checked the as-built source
anchors the tasks cite. Summary of the assessment:

1. **Task Template Compliance** — Pass. All six tasks carry Problem, Solution, Outcome,
   Do, Acceptance Criteria, Tests, Edge Cases, Context, and Spec Reference. Problems
   state WHY (e.g. 1-1: the runner `Result` is discarded at the seam so nothing can
   populate `StageFailure.Output`); Solutions state WHAT (typed `*ai.GenerationError`
   carrier; concise-phrase derivation on `failureMessage`; `notesFailureOutput`
   extraction helper). Acceptance criteria are concrete and pass/fail throughout, and
   the Tests sections name edge cases (whitespace-only body, stderr-only, both-streams,
   non-carrier causes), not just happy paths.

2. **Vertical Slicing** — Pass. Phase 1 slices by failure path (1-1 non-zero exit,
   1-2 empty/whitespace body, 1-3 regression pins for the four non-carrier paths);
   Phase 2 slices by display concern (2-1 concise message, 2-2 extraction/composition
   helper, 2-3 wiring into both surfacing sites). Each task is independently testable.
   Task 1-3 is test-only but is a legitimate behavior-pinning cycle, not housekeeping:
   it guards the load-bearing `context.Canceled`-passthrough and per-cause carrier-free
   invariants that the carrier rewrite could silently break (called out explicitly in
   the spec's testing requirements).

3. **Phase Structure** — Pass. Logical progression: foundation (transport carrier) →
   consumption (engine/presenter display). Each phase has clear, verifiable acceptance
   criteria and the gate block. Phase boundary (transport seam vs engine display seam)
   is principled, not arbitrary.

4. **Dependencies and Ordering** — Pass. The graph carries one explicit edge: 2-2
   (extraction helper) depends on 1-1 (carrier type). This is the genuine cross-phase
   capability requirement — 2-2 references `*ai.GenerationError` and its `Stdout`/
   `Stderr` fields, introduced in 1-1. 1-2 adds no new fields, so it is correctly NOT
   a hard predecessor of 2-2. 2-3 is a convergence point (needs 2-1's `failureMessage`
   change and 2-2's helper), but both predecessors sit earlier in the same phase by
   creation order, so natural intra-phase ordering already produces the correct
   sequence — not flagged per the criterion's sequential-intra-phase guidance. 2-3's
   transitive dependence on the Phase 1 carrier type is satisfied by sequential phase
   execution anchored through the 2-2→1-1 edge. No circular dependencies. All tasks
   priority 2, which is uniform and consistent with a short linear-ish chain.

5. **Task Self-Containment** — Pass. Each task pulls in the relevant spec decisions
   (settled composition rule, per-cause Output behaviour, unmapped-cause contract,
   regenerate's shorter wrap chain) and cites precise as-built anchors. Verified the
   anchors against the codebase: `internal/ai/transport.go` `Generate` lines 150-177 /
   `attempt` 191-204 / bare `return "", ErrGenerationFailed` at 171 and 174;
   `internal/notes/resolve.go` `causeText` line 107 / `abortError` line 100 mapping the
   four sentinels; `internal/notes/generate.go` line 185 `"generating notes: %w"`;
   `internal/engine/release.go` `failureMessage` line 1615 (with its existing
   `*preflight.GateError` branch), `hookFailureOutput` line 1591, `surfaceAndUnwind`
   builder line 1048, `surface` builder line 1605; `internal/presenter/pretty.go`
   `StageFailed` line 536 rendering `s.Output` via `writeNotesBody`;
   `internal/hooks/hooks.go` `HookError` Error/Unwrap at 36/41. Every cited anchor is
   accurate. An implementer can execute any single task without reading the others.

6. **Scope and Granularity** — Pass. No task's Do exceeds the scope signal; each maps to
   one describable test surface and one architectural boundary (Phase 1 inside
   `internal/ai`; Phase 2 across `internal/notes` + `internal/engine`, with the
   notes-side derivation and engine-side helper kept as separate cycles 2-1 and 2-2).
   Nothing is mechanical boilerplate.

7. **Acceptance Criteria Quality** — Pass. Criteria are pass/fail and target the actual
   requirement (e.g. "errors.As(err, &genErr) returns false" on the cancel path;
   "stdout, then a single newline, then stderr; only the composed result's trailing
   whitespace trimmed"; "Message ... does not contain 'failed'"). Boundary values are
   specified (whitespace-only streams treated as empty; exactly two invocations;
   interior blank line kept while trailing whitespace trimmed).

8. **External Dependencies** — N/A (bugfix-scoped work unit; no external dependencies in
   the plan, none expected).

Overall: the plan is implementation-ready as a standalone document. No structural,
template, slicing, dependency, self-containment, scope, or acceptance-criteria issues
warrant a change.
