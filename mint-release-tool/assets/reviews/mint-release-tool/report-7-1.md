TASK: mint-release-tool-7-1 — Wire a per-run Regenerator on the regenerate fresh path so the rendered [r] choice works

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented (correct, symmetric w/ forward path). RegenerateFreshRegenerator (regenerate_fresh.go:82-94) builds a per-run Regenerator binding freshGenerator to res.DiffRange() via GenerateFromRangeWithContext, w/ first-release short-circuit (record.FirstReleaseBody, no AI/diff). cmd producers newRegenerateRegeneratorProducer (regenerate_run.go:75) + newBatchRegeneratorProducer (regenerate_all.go:103) return nil for reuse, fresh regenerator otherwise; wired main.go:181 + regenerate_all.go:62. RegenerateRun + gatePerVersion populate RegenerateWriteRequest.Regenerator, consumed by gateRegenerate→regenerateRegenerator w/ deps.Regenerator override precedence matching forward perRunRegenerator. Forward path untouched. One-time context transient (cfg by value).

TESTS:
- Status: Adequate. Engine r-flow single (regenerate_write_test.go:404) + batch (regenerate_batch_test.go:241), both assert regenerated body reaches provider + no abort. Builder unit: resolved vX-1..vX range diff + context-in-prompt + first-release no-AI. cmd producer: reuse→nil/fresh→non-nil. e regression guard. Forward r guards intact.

CODE QUALITY:
- Followed conventions (small focused functions, doc comments, regeneratorFunc adapter reused). SOLID good — clean injected seam mirroring forward design; freshGenerator shared by body + regenerator producers. Low complexity.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/engine/regenerate_interactive_test.go:51 — runReq leaves ProduceRegenerator nil and no test scripts a ChoiceRegen through RegenerateRun, so the RegenerateRun→produceRegenerator→RegenerateWriteRequest.Regenerator threading (regenerate_interactive.go:187/195) is not directly exercised at this layer (covered one layer down + at batch + cmd producers unit-tested). Add a RegenerateRun fresh test setting ProduceRegenerator to a fake + scripting [r]+[y], asserting the fake is consulted — closes the last untested seam in the single-version interactive path.
