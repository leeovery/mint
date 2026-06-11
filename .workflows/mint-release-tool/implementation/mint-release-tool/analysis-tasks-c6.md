---
topic: mint-release-tool
cycle: 6
total_proposed: 2
---
# Analysis Tasks: mint-release-tool (Cycle 6)

## Task 1: Remove the load-dependent timing flake in the AI transport real-deadline test
status: pending
severity: high
sources: standards

**Problem**: `TestTransport_Generate_RealDeadlineKillIsNonRetriedTimeout` in `internal/ai/transport_test.go:266-298` gates a correctness assertion on a subprocess winning a fork-vs-SIGKILL race against a sub-second deadline. The test forks `#!/bin/sh\nprintf x >> marker\nsleep 5` under a fixed 300ms `Config.Timeout`, then asserts the marker file contains exactly one byte to prove the subprocess ran exactly once. This makes the assertion depend on the forked process forking + starting `sh` + executing `printf` within 300ms. REPRODUCED: passes 5/5 in isolation but fails reliably under CPU contention — `go test -race ./internal/... ./cmd/...` produces `reading invocation marker: ... no such file or directory` because the 300ms deadline SIGKILLs the subprocess before it writes its byte. This is the worst class of flake: passes on a quiet dev machine, fails in busy CI, violating the golang-testing determinism rule (tests MUST be independently runnable) and the code-quality determinism principle.

**Solution**: Decouple the assertion from the wall-clock startup race. The `errors.Is(err, ai.ErrTimeout)` check at :284 already proves the real-kill path classifies the deadline correctly and is timing-robust (the 5s sleep guarantees the 300ms deadline fires regardless of load). The "timeout is not retried" behaviour is ALREADY proven deterministically by the sibling `TestTransport_Generate_DoesNotRetryTimeout` (FakeRunner, :226), so the only genuinely-new coverage here is "a REAL `exec.CommandContext` deadline kill classifies as `ErrTimeout`" — which does not require asserting the marker side-effect at all. Drop the marker write/read side-effect: the script becomes just `sleep 5`, and the marker setup (`dir`, `marker`, the `printf x >> %q` in the body) plus the read/count assertion (:291-297) are removed. Keep the `ErrTimeout` / not-`ErrNotesFailure` assertions.

**Outcome**: The test asserts only the timing-robust classification (real deadline kill → `ErrTimeout`, distinguishable from `ErrNotesFailure`) and contains no subprocess-startup race. It passes deterministically both in isolation and under `go test -race` with parallel-package CPU contention. No correctness coverage is lost because the not-retried behaviour remains covered by the deterministic FakeRunner sibling test.

**Do**:
1. In `internal/ai/transport_test.go`, in `TestTransport_Generate_RealDeadlineKillIsNonRetriedTimeout`:
   - Remove the marker plumbing: delete `marker := filepath.Join(dir, "invocations")` and change the script body to `#!/bin/sh\nsleep 5\n` (no `printf x >> %q` and no `marker` arg). Keep `dir := t.TempDir()` and `script := filepath.Join(dir, "ai-command")` for the script file.
   - Remove the marker read/count block at :291-297 (`os.ReadFile(marker)`, `readErr` handling, and the `len(got) != 1` assertion).
   - Keep the `ai.NewTransport(...)` construction with the 300ms `Timeout` against the 5s sleep, and keep both the `errors.Is(err, ai.ErrTimeout)` assertion (:284) and the `errors.Is(err, ai.ErrNotesFailure)` negative assertion (:287-289).
2. Update the test's explanatory comments (:255-265 and :274-277) to reflect that the assertion now proves real-deadline-kill classification only, and that the "exactly one invocation / no retry" guarantee is covered by `TestTransport_Generate_DoesNotRetryTimeout`. Remove the now-stale claim that 300ms is "generous enough... to write its byte".
3. Prune any imports that become unused after removing the marker write/read (e.g. confirm whether `os`/`filepath` are still needed elsewhere in the file before removing).

**Acceptance Criteria**:
- The test no longer reads or writes a marker file and no longer asserts an invocation count.
- The test still asserts `errors.Is(err, ai.ErrTimeout)` and that the error is NOT `ai.ErrNotesFailure` against a real `exec.CommandContext` deadline kill.
- `go test ./internal/ai/...` passes in isolation.
- `go test -race ./internal/... ./cmd/...` passes with the real-deadline test included (no `reading invocation marker` failure), repeatably under CPU contention.
- The not-retried-on-timeout behaviour remains covered by `TestTransport_Generate_DoesNotRetryTimeout`.
- No unused imports remain; the package builds and `go vet ./internal/ai/...` is clean.

**Tests**:
- Run `internal/ai/transport_test.go` in isolation (`go test ./internal/ai/...`) — passes.
- Run the full race suite that previously reproduced the flake (`go test -race ./internal/... ./cmd/...`), ideally a few iterations, to confirm the marker-not-found failure no longer occurs.

## Task 2: Collapse the four near-identical cmd-layer body/regenerator producer closures into two Resolution-keyed producers
status: pending
severity: medium
sources: duplication, architecture

**Problem**: Four closure-builders in the cmd layer encode the same reuse-vs-fresh source-dispatch rule twice per concern. `newRegenerateBodyProducer` (`cmd/mint/regenerate_run.go:59-66`) and `newBatchBodyProducer` (`cmd/mint/regenerate_all.go:95-102`) both branch `if source == engine.RegenerateSourceReuse { return engine.ReadReuseBody(ctx, r, res.Tag) } return engine.RegenerateFreshBody(ctx, r, nil, root, cfg, res)`. `newRegenerateRegeneratorProducer` (`cmd/mint/regenerate_run.go:75-82`) and `newBatchRegeneratorProducer` (`cmd/mint/regenerate_all.go:110-117`) both branch `if source == engine.RegenerateSourceReuse { return nil } return engine.RegenerateFreshRegenerator(r, nil, root, cfg, res)`. The only difference is that the single-version variants close over one fixed `version.Resolution` while the batch variants take the `Resolution` as a parameter. The dispatch rule (reuse→read / fresh→generate; reuse→no regenerator / fresh→regenerator) is the load-bearing decision, authored four times. A third source or a changed reuse/fresh contract would require updating all four sites in lockstep.

**Solution**: Make the batch-shaped, `Resolution`-keyed producers the canonical pair (they are the more general shape), and derive the single-version closures by partially applying the fixed `Resolution`. Concretely, keep one body producer of shape `func(ctx, source, res) (string, error)` and one regenerator producer of shape `func(source, res) engine.Regenerator` that own the dispatch branch in exactly one place each; the single-version `newRegenerateBodyProducer` / `newRegenerateRegeneratorProducer` then become trivial closures that bind their fixed `res` and delegate to the canonical producers. The dispatch rule lives in one place per concern.

**Outcome**: The reuse-vs-fresh dispatch for body production exists in exactly one function, and the dispatch for regenerator production exists in exactly one function. The single-version cmd path and the batch cmd path both route through those shared producers (single by binding its fixed `Resolution`, batch by threading it through). A future change to the source set or the reuse/fresh contract is made in one site per concern. Externally observable behaviour for both `regenerate` (single) and `regenerate --all` (batch) is unchanged.

**Do**:
1. Choose a home for the two canonical producers — keep them in the cmd layer (the existing `cmd/mint` package) since they only orchestrate already-exported engine helpers. Define:
   - a body producer with the batch signature `func(ctx context.Context, source engine.RegenerateSource, res version.Resolution) (string, error)` containing the single reuse/fresh branch currently duplicated (`ReadReuseBody` vs `RegenerateFreshBody`).
   - a regenerator producer with the batch signature `func(source engine.RegenerateSource, res version.Resolution) engine.Regenerator` containing the single reuse/fresh branch (`nil` vs `RegenerateFreshRegenerator`).
   These can be the existing `newBatchBodyProducer` / `newBatchRegeneratorProducer` (each returns a closure capturing `r, cfg, root`), reused as-is by the batch caller.
2. Reimplement `newRegenerateBodyProducer` (`regenerate_run.go`) so that, instead of inlining the branch, it builds the canonical body producer and returns a closure `func(ctx, source) (string, error)` that calls it with the fixed `res` it already closes over.
3. Reimplement `newRegenerateRegeneratorProducer` (`regenerate_run.go`) the same way: build the canonical regenerator producer and return a closure `func(source) engine.Regenerator` that calls it with the fixed `res`.
4. Confirm the batch caller in `regenerate_all.go` continues to use the canonical producers directly (no signature change at the batch call site).
5. Preserve the existing doc comments' intent: note on the single-version builders that they bind the fixed `Resolution` and delegate to the shared producer; keep the comments documenting the 5-5 reuse / 5-6 fresh mapping.
6. Do not change any engine-level functions (`ReadReuseBody`, `RegenerateFreshBody`, `RegenerateFreshRegenerator`) — this is a cmd-layer-only consolidation.

**Acceptance Criteria**:
- The reuse-vs-fresh branch for body production appears in exactly one function; likewise for regenerator production (no duplicated `if source == engine.RegenerateSourceReuse` body/regenerator branch across `regenerate_run.go` and `regenerate_all.go`).
- `newRegenerateBodyProducer` and `newRegenerateRegeneratorProducer` retain their existing exported-to-cmd signatures (single-version `func(context.Context, engine.RegenerateSource) (string, error)` and `func(engine.RegenerateSource) engine.Regenerator`) so their callers are unchanged.
- The batch caller's wiring is unchanged in behaviour.
- `go build ./...` and `go vet ./cmd/...` are clean.
- All existing `cmd/mint` tests pass unchanged (no behavioural difference for single or batch regenerate).

**Tests**:
- Run the existing `cmd/mint` regenerate test suite (`go test ./cmd/mint/...`) — single-version and `--all` body/regenerator selection for both reuse and fresh sources still pass unchanged.
- If existing coverage does not already assert both branches (reuse and fresh) for both the single and batch producers, add a focused cmd-layer test that exercises the shared producer for reuse (expects `ReadReuseBody` path / nil regenerator) and fresh (expects `RegenerateFreshBody` path / non-nil regenerator), proving both single-bound and batch-threaded routes hit the same dispatch.
