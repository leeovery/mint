TASK: mint-release-tool-5-10 — Interactive default flow for regenerate (source/target prompts + plan + confirm)

STATUS: Issues Found

IMPLEMENTATION:
- Status: Implemented for the prompt/plan/confirm acceptance criteria, but with a cross-task preflight gap. internal/engine/regenerate_interactive.go (RegenerateRun, ResolveRegenerateAxes, resolveSource, resolveTarget); wired at cmd/mint/main.go:146-188 (runRegenerateSingle) with axis mapping in cmd/mint/regenerate_run.go. Source→target→plan→confirm order correct; reuse forces release (no target question), incl. reuse chosen interactively; -y always calls Prompt (no branch-around). Body production injected (ProduceBody/ProduceRegenerator).

TESTS:
- Status: Adequate for in-scope ACs; the preflight gap is untested (lives in cmd layer upstream). regenerate_interactive_test.go: gate sequence, flag-skips, flags-without--y-still-confirm, -y-always-prompts + AcceptEcho, fresh four-choice vs reuse two-choice gate, reuse-forces-release (flag + interactive), confirm-decline abort, body-producer-error surface. regenerate_axes_test.go pins shared resolver.

CODE QUALITY:
- Followed conventions (Go naming, small pure mappers, doc comments). SOLID/DRY good — ResolveRegenerateAxes shared with --all batch path; gate constructors reused. Low complexity, OptionalRegenerateSource/Target model asked-via-flag vs ask-the-user cleanly.

BLOCKING ISSUES:
- Interactive fresh regenerate runs the wrong (empty) preflight subset. For a bare `mint release regenerate <ver>` (no -y, no --target, no --reuse/--fresh), validateRegenerateRequest leaves Target = targetUnset (cmd/mint/regenerate_validate.go:30-52), and preflight runs at cmd/mint/main.go:129 as regenerateGateSet(targetUnset) → {CallsProvider:false, CommitsAndPushes:false} (cmd/mint/regenerate_preflight.go:16-21) — ZERO gates. The actual target is resolved AFTER preflight inside RegenerateRun → resolveTarget (internal/engine/regenerate_interactive.go:245-259), and neither RegenerateRun (5-10) nor RegenerateWrite (5-9) re-runs any preflight after the target is known. Against spec lines 547-550: an interactively-chosen changelog/both commits+pushes the CHANGELOG with NO clean-tree/branch/remote-sync gate, and an interactively-chosen release/both writes the provider with NO gh-auth gate. The gap flagged by sibling 5-4 is real and unmitigated for the interactive path. Fix: after ResolveRegenerateAxes resolves the target, run the matching gate subset before the write — e.g. call RegeneratePreflight with regenerateGateSet(resolvedTarget) inside the interactive flow / cmd wiring — or move axis resolution ahead of preflight. Add a test asserting an interactive changelog choice runs clean-tree/branch/remote-sync and an interactive provider choice runs gh-auth.

NON-BLOCKING NOTES:
- [quickfix] internal/engine/regenerate_interactive.go:296-305 targetFromChoice — the default branch silently maps any unrecognised Choice to release. Gate only returns declared keys so unreachable, but make the three declared keys explicit and treat default as programmer-error (panic/explicit case) so a future added target key cannot silently collapse to release.
