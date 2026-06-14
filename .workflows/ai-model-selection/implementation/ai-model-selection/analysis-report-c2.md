---
topic: ai-model-selection
cycle: 2
total_findings: 4
deduplicated_findings: 3
proposed_tasks: 1
---
# Analysis Report: AI Model Selection (Cycle 2)

## Summary
Three analysis agents (duplication, standards, architecture) raised four findings, all confined to test scaffolding or already-resolved design choices — the production wiring is well-consolidated (all three transport-construction sites route through `aitransport.New`, per-key resolution lives once in `config.AICommandFor`/`TimeoutFor`, all gates pass clean). The only actionable item is one medium-severity test-support consolidation: a non-trivial context-deadline `CommandRunner` spy was independently re-authored in three packages, with a trivial `durationPtr` helper riding alongside it in the same files; these group into a single shared-test-double extraction. The two remaining low-severity findings are deliberate, documented design decisions with the no-stale-default / no-cycle invariants already preserved, so neither is actionable.

## Discarded Findings
- initgen re-types the pinned default literals (standards, low) — Deliberate, review-approved design decision from plan task 3-1: initgen stays import-free and pins each literal equal to `config.DefaultAICommand`/`config.DefaultTimeout` via build-failing drift tests. The spec explicitly permits "built from the constant OR asserted equal by a drift test," so the no-stale-default invariant is preserved by the chosen mechanism. This is an already-resolved design choice, not an actionable correctness or duplication defect; at most a spec-text/as-built wording reconciliation, which is out of an implementation-improvement task's scope.
- Per-site inject-or-build transport wrappers repeat the short-circuit three times (architecture, low) — Explicitly recorded by the analysis agent as a deliberate, well-documented residual with "no action required." The divergence is structural: the injected-transport seam field lives on three different struct/param shapes (`ReleaseDeps.Transport`, `Deps.Transport`, a bare `notes.Transport` param), so collapsing the inject-or-build branch would require unifying those three seam shapes — out of this work unit's scope and not worth the churn. The construction-expression consolidation that mattered already lives once in `aitransport.New`.
</content>
