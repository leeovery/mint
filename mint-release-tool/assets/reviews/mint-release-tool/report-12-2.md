TASK: Collapse the Four cmd-layer Body/Regenerator Producer Closures into Two Resolution-Keyed Producers (mint-release-tool-12-2, chore)

ACCEPTANCE CRITERIA:
- The reuse-vs-fresh branch for body production appears in exactly one function; likewise for regenerator production (no duplicated 'if source == engine.RegenerateSourceReuse' branch across regenerate_run.go and regenerate_all.go).
- newRegenerateBodyProducer / newRegenerateRegeneratorProducer retain their original single-version signatures (func(context.Context, engine.RegenerateSource) (string, error) and func(engine.RegenerateSource) engine.Regenerator) so callers are unchanged.
- The batch caller's wiring is unchanged in behaviour.
- go build ./... and go vet ./cmd/... are clean.
- All existing cmd/mint tests pass unchanged.

STATUS: Complete

SPEC CONTEXT:
The two-axis (source x target) regenerate contract: a REUSE source reads the verbatim release body from the tag annotation (git-only, no AI); a FRESH source re-diffs the range and re-runs AI notes generation. The regenerator (backing the notes-review gate's `r` re-roll choice) only exists on the fresh path — reuse runs a simple confirm with no review gate, hence nil. This dispatch rule (reuse->read/no-regenerator, fresh->generate/regenerator) is exactly the branch this chore consolidates. Single-version and --all batch paths must remain externally identical.

IMPLEMENTATION:
- Status: Implemented (clean, matches the plan's prescribed shape exactly)
- Location:
  - cmd/mint/regenerate_all.go:91-103 — newBatchBodyProducer: canonical Resolution-keyed body producer; the sole reuse/fresh body branch lives at lines 98-101 (ReadReuseBody vs RegenerateFreshBody).
  - cmd/mint/regenerate_all.go:105-120 — newBatchRegeneratorProducer: canonical Resolution-keyed regenerator producer; the sole reuse/fresh regenerator branch at lines 115-118 (nil vs RegenerateFreshRegenerator).
  - cmd/mint/regenerate_run.go:63-68 — newRegenerateBodyProducer: builds the canonical producer, binds fixed res, returns delegating closure func(ctx, source) (string, error). Original single-version signature preserved.
  - cmd/mint/regenerate_run.go:81-86 — newRegenerateRegeneratorProducer: same pattern; returns func(source) engine.Regenerator delegating to the canonical producer with fixed res.
  - cmd/mint/regenerate_all.go:65,69 — batch caller still wires newBatchBodyProducer/newBatchRegeneratorProducer directly (no signature change).
  - cmd/mint/main.go:198,202 — single-version caller call sites unchanged (verified identical arg list).
- Notes:
  - Dispatch-branch uniqueness verified by grep: `if source == engine.RegenerateSourceReuse` appears in cmd/ at exactly two sites — regenerate_all.go:98 (body) and regenerate_all.go:115 (regenerator) — one per concern, as required. No remaining branch in regenerate_run.go.
  - Engine helpers unchanged: ReadReuseBody (internal/engine/regenerate_reuse.go:58), RegenerateFreshBody (regenerate_fresh.go:56), RegenerateFreshRegenerator (regenerate_fresh.go:82) signatures intact. Task commit 980bfa3 --stat touched only cmd/mint/{regenerate_all.go,regenerate_run.go,regenerate_run_test.go} plus workflow tracking files — no internal/engine changes. Confirms cmd-layer-only consolidation.
  - Behaviour preservation: the delegating closures pass the same arguments to the same engine helpers in the same order the old inline branches did; the only structural change is where `res` enters (closed-over and forwarded vs closed-over and inlined). No observable difference.
  - Doc-comment intent (do step 5) preserved: regenerate_run.go:60-62 and :78-80 explicitly note binding the fixed Resolution and delegating to the canonical producer; the 5-5 reuse / 5-6 fresh mapping comments are retained on both single and batch builders.

TESTS:
- Status: Adequate
- Coverage:
  - Single-version reuse body: TestNewRegenerateBodyProducer_Reuse (regenerate_run_test.go:79-94) — seeds git, asserts verbatim tag-annotation body via the delegating closure. NEW in this commit.
  - Single-version regenerator both branches: TestNewRegenerateRegeneratorProducer (regenerate_run_test.go:100-112) — reuse -> nil, fresh -> non-nil.
  - Batch reuse body: TestNewBatchBodyProducer_Reuse (regenerate_all_test.go:139-153).
  - Batch regenerator both branches: TestNewBatchRegeneratorProducer_Reuse (:158-165) and TestNewBatchRegeneratorProducer_Fresh (:170-177).
  - This satisfies the plan's test ask: the shared producer is exercised for reuse (ReadReuseBody path / nil regenerator) and fresh (non-nil regenerator), proving both single-bound and batch-threaded routes hit the same dispatch for the regenerator concern, and the reuse body for both routes.
- Notes:
  - Not over-tested: each test asserts one branch/route with minimal setup (FakeRunner, TempDir). No redundancy.
  - One genuine coverage gap (minor, non-blocking): the FRESH body branch (RegenerateFreshBody) is not directly exercised by any cmd-layer producer test for either route. The fresh body path needs an AI transport, so it is not cheaply runnable with FakeRunner alone — the fresh regenerator tests only assert non-nil rather than invoking the AI path. So fresh-body parity between the single and batch routes rests on the shared-dispatch structure (provably one function) rather than an executed assertion. Acceptable for a behaviour-preserving consolidation, since the dispatch is now structurally singular and the reuse route is asserted end-to-end on both paths, but worth a focused note.

CODE QUALITY:
- Project conventions: Followed. Matches golang-design-patterns closure/partial-application idiom (the more-general Resolution-keyed producer is canonical; the single-version one is derived by binding the fixed value) and DRY guidance (dispatch authored once). Naming consistent with existing newBatch*/newRegenerate* convention.
- SOLID principles: Good. Single responsibility per producer; the dispatch rule has one owner per concern (open/closed: a third source or a changed reuse/fresh contract now changes exactly one function each instead of four sites).
- Complexity: Low. Each canonical producer is a single two-way branch; the delegators are one-line forwards.
- Modern idioms: Yes. Idiomatic Go closures; t.Context() used in tests.
- Readability: Good. Doc comments clearly explain the canonical-vs-derived relationship and cross-reference both directions (single->batch and batch->single), and preserve the 5-5/5-6 mapping intent.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] cmd/mint/regenerate_run_test.go / regenerate_all_test.go — add a focused fresh-body test (single-bound and/or batch-threaded) that drives RegenerateFreshBody via a fake/stub AI transport and asserts the fresh route is taken, closing the one branch (fresh body) currently proven only by structure rather than an executed assertion. Mirrors the existing reuse-body tests; would make the "both branches for both routes" guarantee fully test-backed.
