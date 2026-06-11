TASK: mint-release-tool-7-3 — Emit StageStarted / StageSucceeded around the release and regenerate stages

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented (correctly, no drift). Two shared helpers engine.go:122 emitGateSucceeded + :136 emitBlockingStageStarted (returns a completion closure that times the stage and emits StageSucceeded with time.Since). Wired in release.go:334,378 (version/preflight gate completion), :433/438 (notes), :605/615 (push), :1048/1057 (pre_tag, only when configured), :1329/1337/1344 (edit stage bracketing the editor). Regenerate: regenerate_interactive.go:170/175 (notes), regenerate_batch.go:244/249 (per-version notes), regenerate_write.go:307/311 (push PONR), shared reviewGate reuses edit-stage wiring. Editor SuspendSpinner/ResumeSpinner now wraps a genuinely live spinner. Stage events conditional (pre_tag only when a hook runs; push only when a commit is pushed). Failure paths omit StageSucceeded.

TESTS:
- Status: Adequate. release_stageevents_test.go: Blocking:true + non-negative Elapsed + start-before-success ordering for pre_tag/notes/push, gate completion for version/preflight, spine order, RunStarted-first, hookless pre_tag omission, real-launcher edit (SuspendSpinner/ResumeSpinner inside a live stage). regenerate_stageevents_test.go: interactive notes stage, body-production failure (no StageSucceeded), write-path push stage, release-only no-push omission. priortag/release tests pin full timeline (existing-events-unchanged guard).

CODE QUALITY:
- Followed conventions (accept-interfaces seam, presenter-only output, shared-helper DRY, no time.Now in deterministic paths except the justified stage timer). SOLID/DRY good — one closure shared across five blocking-stage sites + both verbs. Low complexity, clear closure-returns-completion pattern.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/engine/regenerate_batch.go:244 — the batch (--all) per-version notes blocking stage is wired identically to the other paths but has no dedicated stage-event assertion in regenerate_stageevents_test.go (which covers interactive notes + write push only). Add a small batch test asserting the per-version notes StageStarted(Blocking:true)/StageSucceeded fire and that a per-version skip emits no StageSucceeded, to lock the batch half against drift.
