AGENT: architecture
FINDINGS:
- FINDING: Stale cross-reference comment names a cmd-layer selector that no longer exists
  SEVERITY: low
  FILES: internal/engine/regenerate.go:54
  DESCRIPTION: The doc comment on the engine's regenerateGateSet stated it is "the engine-side analogue of the cmd layer's regenerateGateSet, keyed off the engine's RegenerateTarget rather than the cmd flag enum." No cmd-layer regenerateGateSet exists anymore — Phase 10 (close interactive-regenerate preflight gate bypass) moved all regenerate preflight gating into the engine to run after the interactive target resolves, removing the cmd-layer counterpart. The comment described a mirror-pair relationship that no longer holds. Documentation-accuracy only (runtime selection is correct and well-tested).
  RECOMMENDATION: Reword to describe regenerateGateSet as the single engine-owned selector keyed off the resolved RegenerateTarget. Drop the "engine-side analogue of the cmd layer's regenerateGateSet" clause.
  STATUS: REMEDIATED 2026-06-11 (commit fixing the comment) — applied directly at the user's direction rather than queued as a task.
SUMMARY: Architecture is sound and composes cleanly across the cmd/engine seam; the cycle-6 fixes introduced no seam regressions, and the cycle-5 gh-auth gate (regenerateGateSet(target, publisher != nil), mirroring release.go's `if publisher != nil` guard in both single and batch paths) is correct. The only issue was a stale doc comment referencing a removed cmd-layer selector, now fixed.
