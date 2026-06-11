TASK: mint-release-tool-5-11 — Batch --all single-version regeneration loop

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/engine/regenerate_batch.go: RegenerateAll (153-180) loops req.Versions in slice order; processOneVersion (209-273) opens per-version block (RunStarted 216, ShowPlan 221), reuse has-body skip check (226-234), ProduceBody (245), gatePerVersion (254), DispatchRelease per version (267). Ordering from version.ResolveAllRegenerateTargets (oldest-first sort). No resume/checkpoint state. Production wiring in cmd/mint/regenerate_all.go (newBatchBodyProducer, newBatchRegeneratorProducer). gatePerVersion passes Tag:"" — gate consumes only Body/VersionKey/Regenerator so harmless.

TESTS:
- Status: Adequate. regenerate_batch_test.go + regenerate_batch_skip_test.go: oldest→newest dispatch order; one RunStarted block per version (oldest→newest); notes per version flow to right dispatch + collected slice; per-version gate by default (3 prompts); -y opts out (0 prompts, still 3 dispatches); mixed update/create via per-version probe; no-resume re-run determinism; decline aborts; empty-set no-op; fresh r-choice consults per-version regenerator.

CODE QUALITY:
- Followed conventions (errors wrapped + surface, table-free focused tests w/ t.Parallel, FakeRunner/RecordingPresenter/fakePublisher). SOLID good — processOneVersion factored so 5-12 wraps it; injected producers keep loop transport-free. Low complexity.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/engine/regenerate_batch.go:226-234 + cmd/mint/regenerate_all.go:90-92 — on the reuse path each version reads the tag annotation body twice: once via ReadTagBody for the has-body skip check, then again inside ProduceBody→ReadReuseBody (re-runs the same git for-each-ref). Decide whether to thread the already-read body from the skip check into the dispatch to halve git calls on large --reuse --all backfills.
- [quickfix] internal/engine/regenerate_batch.go:339 — gatePerVersion builds the inner RegenerateWriteRequest with no Target field set (defaults to RegenerateTargetRelease). Inert today (gateRegenerate never reads Target) but fragile; set Target: req.Target explicitly (or comment that the gate ignores Target).
