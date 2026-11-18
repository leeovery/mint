---
scope: mint-release-tool
cycle: 1
source: review
total_proposed: 9
gate_mode: auto
---
# Review Tasks: Mint Release Tool (Cycle 1)

## Task 1: Close the interactive-regenerate preflight gate bypass
status: approved
severity: high
sources: report-5-10, report-5-4

**Problem**: For a bare `mint release regenerate <ver>` (no `-y`, no `--target`, no `--reuse`/`--fresh`), `validateRegenerateRequest` leaves `Target = targetUnset` (`cmd/mint/regenerate_validate.go:30-52`); preflight then runs at `cmd/mint/main.go:129` as `regenerateGateSet(targetUnset)` → `{CallsProvider:false, CommitsAndPushes:false}` (`cmd/mint/regenerate_preflight.go:16-21`) — i.e. zero gates. The real target is resolved AFTER preflight inside `RegenerateRun` → `resolveTarget` (`internal/engine/regenerate_interactive.go:245-259`), and neither `RegenerateRun` nor `RegenerateWrite` (`internal/engine/regenerate_write.go:137-179`) re-runs any gate. Against spec lines 547-550: an interactively-chosen `changelog`/`both` commits+pushes the CHANGELOG with NO clean-tree/branch/remote-sync gate, and an interactively-chosen `release`/`both` writes the provider with NO gh-auth gate.
**Solution**: After `ResolveRegenerateAxes` resolves the actual target, run `RegeneratePreflight(regenerateGateSet(resolvedTarget))` before the write — or move axis resolution ahead of preflight so the existing call sees the resolved target.
**Outcome**: An interactively-chosen `changelog`/`both` runs the clean-tree/on-branch/remote-sync gate before committing+pushing; an interactively-chosen `release`/`both` runs the gh-auth gate before writing the provider. No regenerate write path reaches mutation/network with an empty gate set.
**Do**:
1. In the interactive single-version flow, capture the target resolved by `ResolveRegenerateAxes`/`resolveTarget` (`internal/engine/regenerate_interactive.go:245-259`).
2. Before the write step, invoke `RegeneratePreflight` with `regenerateGateSet(resolvedTarget)` (reuse the existing gate-set constructor and preflight entry point used at `cmd/mint/main.go:129`); route any gate failure through the existing abort/exit-code path.
3. Alternatively, restructure so axis resolution happens before the single preflight call in `cmd/mint/main.go` and pass the resolved target into the existing call. Pick one approach; do not leave both an early empty-gate call and a late resolved-gate call that double-prompt or double-run gates.
4. Ensure tag-free and version-compute gates remain excluded (regenerate cuts no tag).
**Acceptance Criteria**:
- A bare interactive `changelog` (or `both`) choice runs clean-tree, on-branch, and remote-sync gates before any CHANGELOG commit/push.
- A bare interactive `release` (or `both`) choice runs the gh-auth gate before any provider write.
- A failing applicable gate aborts cleanly before any mutation or network call.
**Tests**:
- Test asserting an interactive `changelog` choice runs clean-tree/branch/remote-sync (observe the gate runner invocations via the existing FakeRunner seam).
- Test asserting an interactive provider (`release`/`both`) choice runs gh-auth before the provider write.

## Task 2: Fix the nil-Publisher crash on the regenerate paths
status: approved
severity: high
sources: report-5-10, report-7-5, report-4-9
note: Supersedes report Idea #17 (shared ResolvePublisher helper) and quick-fix #13 (cmd-level unresolved-publisher test) — both folded into this task; do not emit them separately.

**Problem**: `cmd/mint/main.go:166` and `cmd/mint/regenerate_all.go:49` discard the `publish.ResolvePublisher` error (`publisher, _ := …`) and pass the nil interface down. `RegenerateWrite` (`internal/engine/regenerate_write.go:166`) → `DispatchRelease` (`internal/engine/regenerate_dispatch.go:41`) then calls `ReleaseExists` on it. Reproduced: `mint release regenerate <ver> --reuse -y` in a repo whose origin is not `github.com` panics with a nil-pointer dereference.
**Solution**: Branch on `publish.ErrProviderUnresolved` exactly as `engine.Release` does (warn-and-downgrade or abort), and nil-guard before provider dispatch in `RegenerateWrite`. Consolidate the two duplicated resolve-and-discard call sites into a single shared helper (e.g. `engine.ResolvePublisher(ctx, r, cfg)`) that performs the real error handling, satisfying the former Idea #17 in the same change.
**Outcome**: Regenerate with an unresolvable provider aborts or downgrades cleanly (matching the forward `engine.Release` behaviour) — never a nil-pointer panic.
**Do**:
1. Add a shared helper (e.g. `engine.ResolvePublisher(ctx, r, cfg)`) wrapping `publish.ResolvePublisher(engine.RemoteURL(…), cfg.Release.Provider, r)` that returns the publisher and the error.
2. Replace the `publisher, _ := …` discards at `cmd/mint/main.go:166` and `cmd/mint/regenerate_all.go:49` with calls to the helper, handling `publish.ErrProviderUnresolved` the same way `engine.Release` does (warn-and-downgrade for the no-provider carve-out, abort otherwise).
3. In `RegenerateWrite` (`internal/engine/regenerate_write.go:166`) / before `DispatchRelease` (`internal/engine/regenerate_dispatch.go:41`), nil-guard the publisher so a downgraded (provider-skipped) run never calls `ReleaseExists` on nil.
**Acceptance Criteria**:
- `mint release regenerate <ver> --reuse -y` against a non-github / no-remote origin does not panic.
- The unresolvable-provider case either aborts with a clear message or downgrades (provider write skipped) consistent with `engine.Release`.
- No regenerate code path dereferences a nil `Publisher`.
**Tests**:
- Test: single-version regenerate with an unresolvable provider aborts or downgrades cleanly — asserts no panic and the expected warn/abort outcome.
- Test: `--all` (batch) regenerate with an unresolvable provider takes the same clean path.

## Task 3: Add SIGINT/SIGTERM handling
status: approved
severity: high
sources: report-5-10, external-audit

**Problem**: No `signal.NotifyContext` exists anywhere; every cmd entry point uses a bare `context.Background()` (`cmd/mint/main.go:92,116,129,147,226`). Ctrl-C during the AI call, a hook, or between the bookkeeping commit and the atomic push kills the process with no unwind and no autostash pop — stray commit(s), tag, and stash survive, contradicting the fail-loud / repo-clean philosophy.
**Solution**: Create a signal-cancellable context once in `run()` via `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`, thread it down through the cmd entry points, and treat pre-PONR context cancellation as a failure routed through the existing `surfaceAndUnwind`.
**Outcome**: Ctrl-C / SIGTERM before the point-of-no-return triggers the existing surgical unwind (resets, tag delete, autostash pop) so the repo is left clean; post-PONR cancellation follows the existing warn-only contract.
**Do**:
1. In `run()`, build `ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` and `defer stop()`.
2. Replace the bare `context.Background()` calls at `cmd/mint/main.go:92,116,129,147,226` (and the per-path `ctx := context.Background()` in the run helpers) with the threaded signal ctx.
3. Ensure pre-PONR `ctx.Err()` cancellation is surfaced as a failure routed through `surfaceAndUnwind` so the existing unwind/autostash-pop runs; confirm post-PONR cancellation keeps the warn-only behaviour.
**Acceptance Criteria**:
- A cancelled context mid-spine (pre-PONR) triggers the surgical unwind path and pops any autostash.
- No cmd entry point uses a bare `context.Background()` for the release/regenerate spine.
- Post-PONR cancellation does not attempt an unwind (warn-only, unchanged).
**Tests**:
- Test: a context cancelled mid-spine (before PONR) triggers the unwind path (assert recovery resets / tag delete / autostash pop via the FakeRunner+Mutator seam).

## Task 4: Fix Mutator.Mutate retrying with a consumed io.Reader
status: approved
severity: high
sources: report-5-10, external-audit

**Problem**: `internal/git/mutator.go:146-172` re-invokes `RunWith` with the same `stdin io.Reader` on a lock-contention retry. For `createAnnotatedTag` (`internal/release/release.go:100`, `git tag -a … -F -`), a retried attempt pipes an already-exhausted reader and writes an empty tag annotation, silently breaking `regenerate --reuse` (whose only source is that annotation). `internal/ai/transport.go` documents and avoids this exact trap (fresh `strings.NewReader` per attempt).
**Solution**: Change `Mutate`'s stdin parameter from `io.Reader` to `[]byte`/`string` (or a `func() io.Reader` factory) so every retry attempt gets fresh bytes; update callers accordingly.
**Outcome**: A lock-retried stdin-bearing mutation pipes the full stdin on every attempt — no empty/truncated tag annotation after a retry.
**Do**:
1. Change `Mutate` (and `invoke`) in `internal/git/mutator.go` to accept stdin as `[]byte`/`string` (or a `func() io.Reader` factory), constructing a fresh reader inside the retry loop per attempt.
2. Update all `Mutate` callers that pass stdin — notably `createAnnotatedTag` (`internal/release/release.go:100`) — to pass the bytes/factory.
3. Keep the non-stdin (`nil`) path delegating to `Run` unchanged.
**Acceptance Criteria**:
- A lock-retried stdin-bearing mutation receives the full stdin (not an empty reader) on the second attempt.
- `git tag -a … -F -` produces the full annotation even when the first attempt hit a lock and retried.
- `regenerate --reuse` reads back the complete annotation body after a retried tag creation.
**Tests**:
- Test: a lock-retried stdin-bearing mutation receives the full stdin on the second attempt (FakeRunner reports the bytes seen per attempt; assert the retried attempt got the complete payload).

## Task 5: Stop reporting "repo clean" when the unwind itself failed
status: approved
severity: high
sources: report-5-10, external-audit

**Problem**: `internal/engine/unwind.go:104,117` and `internal/engine/regenerate_write.go:322` discard the `Mutate` errors from the recovery resets / tag-delete, and `surgicalSummary` (`unwind.go:137-138`) then unconditionally appends `"; repo clean"` — a false success exactly when recovery failed.
**Solution**: Check the recovery `Mutate` results; on failure emit a `Warn` naming the manual cleanup needed and omit/replace the "repo clean" tail of the summary.
**Outcome**: When a recovery reset / tag-delete fails mid-unwind, the user sees a warning describing what to clean up manually, and the summary does not falsely claim "repo clean".
**Do**:
1. In `internal/engine/unwind.go` (lines 104, 117) and `internal/engine/regenerate_write.go:322`, stop discarding the `Mutate` error returns; capture them.
2. On any recovery failure, emit a `Warn` naming the specific manual cleanup (e.g. which reset/tag-delete failed).
3. In `surgicalSummary` (`unwind.go:137-138`), append `"; repo clean"` only when all recovery operations succeeded; otherwise omit or replace it with a "manual cleanup required" tail.
**Acceptance Criteria**:
- A failed mid-unwind recovery `Mutate` produces a `Warn` naming the needed manual cleanup.
- The summary in that case does NOT contain "repo clean".
- A fully-successful unwind still reports "repo clean" as before.
**Tests**:
- Test: a failed mid-unwind `Mutate` yields a warn and a summary without "repo clean".

## Task 6: Parse --plain on the regenerate route
status: approved
severity: high
sources: report-5-10, external-audit

**Problem**: `cmd/mint/main.go:115` hardcodes `plainFlag=false` and `parseRegenerateFlags` (`cmd/mint/regenerate_flags.go`) defines no `--plain` flag, so `mint release regenerate <ver> --plain` is a usage error — contradicting the CLI presentation contract (and `cmd/mint/init.go`'s own doc) that `--plain` is global to every verb.
**Solution**: Add the `--plain` flag to the regenerate flag parser and thread its value into `NewForStartup` instead of the hardcoded `false`.
**Outcome**: `mint release regenerate <ver> --plain` parses and forces plain presentation, matching every other verb.
**Do**:
1. Define a `--plain` flag in `parseRegenerateFlags` (`cmd/mint/regenerate_flags.go`).
2. Thread the parsed value into the `presenter.NewForStartup(...)` call at `cmd/mint/main.go:115`, replacing the hardcoded `false` first argument.
3. Confirm `--plain` works in combination with the existing regenerate flags (`--target`, `--reuse`/`--fresh`, `-y`, `--all`).
**Acceptance Criteria**:
- `mint release regenerate <ver> --plain` is accepted (not a usage error) and selects plain presentation.
- `--plain` composes with the other regenerate flags.
**Tests**:
- Flag-parse test asserting `--plain` is recognised on the regenerate route and propagates into the presenter startup.

## Task 7: Fix the production timeout misclassification (promoted bug)
status: approved
severity: high
sources: report-2-1, external-audit

**Problem**: On a context-deadline kill, `exec.CommandContext` returns an `*exec.ExitError` (process killed by signal), so `translateRun` (`internal/runner/exec_runner.go:97-100`) takes the `errors.As(&exitErr)` branch and wraps the ExitError, NOT `context.DeadlineExceeded`. The AI transport's `classifyFatal` (`internal/ai/transport.go:160`) detects timeouts solely via `errors.Is(err, context.DeadlineExceeded)`, which is therefore false in production — a real ~60s timeout is misclassified as bad content and RETRIED, defeating the spec's "timeout is not retried" guarantee (worst-case latency becomes two timeouts). The transport's own tests pass only because they inject a `DeadlineExceeded`-wrapping error.
**Solution**: In `translateRun`, when `ctx.Err()` is `DeadlineExceeded`/`Canceled`, wrap that cause so `errors.Is(err, context.DeadlineExceeded)` holds for the returned error.
**Outcome**: A real production deadline kill is classified as a (non-retried) timeout end-to-end; the AI transport surfaces `ErrTimeout` without a second attempt.
**Do**:
1. In `translateRun` (`internal/runner/exec_runner.go:97-100`), before/within the `*exec.ExitError` branch, check `ctx.Err()`; if it is `context.DeadlineExceeded` or `context.Canceled`, wrap that cause (e.g. `%w`) so `errors.Is(err, context.DeadlineExceeded)`/`Canceled` holds on the result.
2. Confirm `classifyFatal` (`internal/ai/transport.go:160`) now classifies a real deadline kill as a timeout (not retried).
**Acceptance Criteria**:
- A real context-deadline kill from `exec.CommandContext` produces an error for which `errors.Is(err, context.DeadlineExceeded)` is true.
- The AI transport classifies that as a non-retried timeout (single invocation).
**Tests**:
- End-to-end test that a real deadline kill (not an injected DeadlineExceeded wrapper) is classified as a non-retried timeout; assert a single underlying invocation.

## Task 8: Fix the whitespace-only $EDITOR panic (promoted bug)
status: approved
severity: high
sources: report-2-13, external-audit

**Problem**: `internal/engine/editor.go:99` — `args := append(fields[1:], tmpPath)` then `fields[0]` (line 98) panics when the resolved editor value is whitespace-only (e.g. `EDITOR=" "`), since `strings.Fields` returns an empty slice and `ResolveEditor` only guards against the empty string, not whitespace-only.
**Solution**: Guard `len(fields)==0` after splitting and treat it as "no launchable editor" (Warn + `ErrEditorReturnToGate`), or have `ResolveEditor` fall through to `vi` when the value is blank after trimming.
**Outcome**: A whitespace-only `$EDITOR`/`$VISUAL` no longer panics — the gate either falls through to `vi` or returns to the review gate with a warning, consistent with the missing-editor handling.
**Do**:
1. In `EditorLauncher.Edit` (`internal/engine/editor.go` around line 98-99), after `strings.Fields`, guard `len(fields)==0` and treat it as the no-launchable-editor case (Warn + `ErrEditorReturnToGate`, with temp-file cleanup preserved), OR
2. Alternatively, in `ResolveEditor`, trim the candidate and treat a blank-after-trim value as unset so resolution falls through to `vi`.
3. Pick one approach consistently with the existing missing-editor path.
**Acceptance Criteria**:
- `EDITOR=" "` (whitespace-only) does not panic.
- The run either launches `vi` (fall-through) or returns to the gate with a Warn + `ErrEditorReturnToGate`, with the temp file cleaned up.
**Tests**:
- Test: a whitespace-only resolved editor value is handled without panic (asserts the chosen behaviour — fall-through to vi, or Warn + `ErrEditorReturnToGate` + temp cleanup).

## Task 9: Stop blocking-stage spinner leaks across Warn (promoted bug, extended)
status: approved
severity: high
sources: report-5-12, external-audit

**Problem**: Two same-class cases where a `Warn` (or skip) fires while a blocking stage's spinner is still live: (a) the `--all` notes-production-failure skip path starts the notes spinner but never stops it — `internal/engine/regenerate_batch.go:244-248`; `reportSkip`→`Warn` doesn't stop it (`internal/presenter/pretty.go:520` / `:905`), so for the last/only version hitting diff-too-large the spinner animates over the `⚠ skipped` and end-summary lines; and (b) the real-run cache-reuse / miss / unreadable notices ride the `Warn` seam inside the live blocking notes stage — `internal/engine/release.go:802,810,820` — so the spinner animates over those notices too.
**Solution**: Fix once, generally: stop or suspend the active spinner before any `Warn` emitted inside a blocking stage (not only on the batch-skip path) — e.g. close the stage before the notice, or have the presenter suspend/stop the spinner on `Warn` while a blocking stage is active.
**Outcome**: No spinner animates over a `Warn` or skip line emitted inside a blocking stage, in either the batch-skip path or the real-run cache-reuse/miss/unreadable path.
**Do**:
1. Implement a single general fix: when `Warn` is emitted while a blocking stage's spinner is active, the presenter suspends/stops the spinner first (preferred — covers all sites), or close the stage before emitting the notice.
2. Verify it covers both (a) `internal/engine/regenerate_batch.go:244-248` (notes-failure skip → `reportSkip`/`Warn`) and (b) `internal/engine/release.go:802,810,820` (cache-reuse/miss/unreadable `Warn` inside the live notes stage).
3. Check `internal/presenter/pretty.go:520` (`Warn`) and `:905` (`RunFinished`) so the spinner is reliably stopped before exit.
**Acceptance Criteria**:
- The `--all` notes-failure skip on the last/only version emits `⚠ skipped` and the end summary with no live spinner over them.
- The real-run cache-reuse / miss / unreadable notices appear without a spinner animating over them.
- A normal blocking stage (no Warn/skip) still shows and stops its spinner as before.
**Tests**:
- Test covering the batch-skip case: a notes-production-failure skip stops/suspends the active spinner before the warn/summary.
- Test covering the cache-reuse case: a cache-reuse/miss/unreadable Warn inside the blocking notes stage stops/suspends the spinner first.
