TASK: mint-release-tool-8-3 — Remove the orphaned Phase-1 presenter mappers (EmitPlan/EmitStageFailed/EmitNotes/EmitWarning)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented (clean removal). internal/engine/engine.go — ReviewDecision ends at line 116, flows straight into emitGateSucceeded (line 118); the old 123-154 Emit* block is gone. Adjacent live symbols intact: FirstReleaseReviewGate (line 83), emitGateSucceeded (122), emitBlockingStageStarted (136). Grep for EmitPlan|EmitStageFailed|EmitNotes|EmitWarning across the repo returns zero .go hits — surviving matches only in .workflows/ docs, .tick/tasks.jsonl, task-detail. Orchestrator's direct presenter calls confirmed present/unchanged: release.go:443 ShowPlan, :444/:1348/:1418 ShowNotes, :1531 StageFailed, numerous Warn; plus regenerate_*.go direct calls.

TESTS:
- Status: Adequate. The four dedicated Emit* tests are removed; engine_test.go now ends at line 137 with the ReviewDecision table test. FirstReleaseReviewGate + ReviewDecision retain live tests. Presenter-interaction coverage for plan/stage-failed/notes/warning broad + intact across internal/presenter/*_test.go. No orphaned references in any test.

CODE QUALITY:
- Followed conventions (exported surface trimmed, no new exports, removed-symbol doc comments removed). SOLID good — removes redundant indirection, tightening engine's public boundary. Low complexity, pure deletion.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
