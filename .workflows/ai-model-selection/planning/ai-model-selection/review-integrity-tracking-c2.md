---
status: complete
created: 2026-06-13
cycle: 2
phase: Plan Integrity Review
topic: AI Model Selection
---

# Review Tracking: AI Model Selection - Integrity

## Findings

### 1. Task 2-2's `Transport.deadline` field inverts the "no deadline" encoding relative to `Config.Timeout`/`TimeoutFor`, so a direct copy reintroduces both bugs the task exists to prevent

**Severity**: Critical
**Plan Reference**: Phase 2, Task ai-model-selection-2-2 ("Make `ai.Config.Timeout` carry absent-vs-explicit-zero and apply the per-attempt deadline conditionally")
**Category**: Acceptance Criteria Quality / Task Self-Containment (internal contract consistency)
**Change Type**: update-task

**Details**:
Task 2-2 defines two `*time.Duration` carriers with OPPOSITE "no deadline" polarities and then instructs the implementer to copy one straight into the other, which silently produces the exact two failures the task is written to forbid.

The boundary type `ai.Config.Timeout` is specified (Do bullet 1, Outcome, AC 1) as: `nil` = not threaded (a wiring bug), `&0` = the operator's explicit "no deadline", `&positive` = a real deadline. Phase 1's `TimeoutFor` returns a pointer-to-`0` for the explicit-zero/no-deadline case and never returns nil (Task 1-7). So in production `ai.Config.Timeout` is `&0` for no-deadline.

The internal carrier `Transport.deadline` is specified (Do bullet 4) as a `*time.Duration` where "`nil` ⇒ no deadline; non-nil positive ⇒ that deadline", and `attempt` (Do bullet 5) keys the no-deadline path off `t.deadline == nil`. That is the OPPOSITE encoding: here no-deadline is `nil`, not `&0`.

Do bullet 4 then says "Set it in `NewTransport` from `cfg.Timeout`." If `NewTransport` copies `t.deadline = cfg.Timeout` directly (both are `*time.Duration`):
- The operator's explicit no-deadline (`cfg.Timeout == &0`) becomes `t.deadline == &0`, which is **non-nil**, so `attempt` takes the `t.deadline != nil` branch and calls `context.WithTimeout(ctx, *t.deadline)` = `context.WithTimeout(ctx, 0)` — firing immediately. That is precisely the "passing a zero duration to `WithTimeout` fires immediately, producing instant timeouts" regression the task is meant to eliminate (and AC 3 forbids).
- A `nil` `cfg.Timeout` (the wiring-bug case the invariant forbids) becomes `t.deadline == nil`, which `attempt` treats as **no deadline** — silently running unbounded. That is precisely the "no deadline reachable only via explicit `0`, never by a forgotten/zero-by-omission field" invariant inversion (AC 6/7 forbid it).

The task never specifies the translation step that bridges the two encodings, and its Do/Outcome/AC are internally contradictory: AC 1 says `&0` is no-deadline, AC 7 says "no deadline reachable only via explicit `0`", but Do bullet 5's `t.deadline == nil` test makes `nil` the no-deadline trigger and `&0` a fire-immediately deadline. An implementer following the Do literally (copy `cfg.Timeout` into `t.deadline`, branch on `t.deadline == nil`) ships both bugs and passes neither AC 3 nor AC 6/7. This blocks correct implementation.

The fix keeps `Transport.deadline` as the internal carrier but makes `NewTransport` MAP `cfg.Timeout` into it rather than copy it, and makes the `attempt` conditional key off the resolved duration VALUE (deadline applies iff a positive value is present), so the explicit-`0`/no-deadline case takes the parent-context path and the nil wiring-bug case does NOT silently become no-deadline. The cleanest internal representation is a `deadline *time.Duration` that is non-nil only when a POSITIVE deadline applies (nil = run on parent context), populated by an explicit mapping: positive `*cfg.Timeout` → `&value`; explicit `&0` → leave `deadline` nil (no-deadline path); `nil` `cfg.Timeout` → the documented wiring-bug contract (not silently no-deadline). This makes the internal `nil ⇒ no deadline` rule correct ONLY because `NewTransport` translates `&0` to nil deliberately — it is no longer the same pointer the boundary carries.

This edits Task 2-2 only — Do bullets 4 and 5, the Outcome, and the affected Acceptance Criteria — to specify the mapping and the value-based conditional. No other task changes.

**Current** (Task ai-model-selection-2-2, **Outcome**):
> **Outcome**: `ai.Config.Timeout` is a `*time.Duration`; a nil pointer means "not set" and an explicit `0` (a pointer to `0`) means "no deadline"; the transport skips `context.WithTimeout` and runs on the parent context when the resolved timeout is the no-deadline case, and uses `context.WithTimeout(ctx, d)` for a positive `d`; `defaultTimeout` is deleted; `context.Canceled` still propagates unchanged on both the deadline and no-deadline paths.

**Proposed** (Task ai-model-selection-2-2, **Outcome**):
> **Outcome**: `ai.Config.Timeout` is a `*time.Duration` whose `&0` is the operator's explicit "no deadline" (`nil` is the forbidden not-threaded case); `NewTransport` MAPS that boundary value into the transport's internal deadline carrier (it does not copy the pointer through), so the explicit-`0`/no-deadline case runs the attempt on the parent context and a positive value runs under `context.WithTimeout(ctx, d)`; `defaultTimeout` is deleted; `context.Canceled` still propagates unchanged on both the deadline and no-deadline paths.

**Current** (Task ai-model-selection-2-2, **Do**, the `Transport` struct bullet):
> - Change the `Transport` struct (~lines 68-72) so it records the per-attempt deadline AND whether one applies. Recommended: a `deadline *time.Duration` field (nil ⇒ no deadline; non-nil positive ⇒ that deadline). Set it in `NewTransport` from `cfg.Timeout`.

**Proposed** (Task ai-model-selection-2-2, **Do**, the `Transport` struct bullet):
> - Change the `Transport` struct (~lines 68-72) so it records the per-attempt deadline AND whether one applies. Recommended: a `deadline *time.Duration` field where **nil ⇒ no per-attempt deadline (run on the parent context)** and **non-nil ⇒ a POSITIVE deadline of that value**. CRITICAL — this internal field's nil-means-no-deadline polarity is the INVERSE of the `Config.Timeout` boundary (where `&0` means no-deadline and `nil` is the forbidden not-threaded case), so `NewTransport` must MAP `cfg.Timeout` into `t.deadline`, NOT copy the pointer through. The mapping in `NewTransport`: if `cfg.Timeout == nil` → the documented wiring-bug case (per the nil-handling bullet — must NOT become a silent no-deadline); if `*cfg.Timeout == 0` (the operator's explicit no-deadline) → set `t.deadline = nil` (the parent-context path); if `*cfg.Timeout > 0` → set `t.deadline` to a pointer to that positive duration. A direct `t.deadline = cfg.Timeout` is WRONG: it would leave the explicit-`0`/no-deadline case as a non-nil `&0` (firing `WithTimeout(ctx, 0)` immediately) and would turn the forbidden nil-by-omission case into a silent unbounded run.

**Current** (Task ai-model-selection-2-2, **Do**, the `attempt` bullet):
> - In `attempt` (~lines 147-156), apply the deadline conditionally: when `t.deadline == nil` (the no-deadline case), run `t.runner.RunWith(ctx, …)` on the PARENT context — do NOT call `context.WithTimeout`. When `t.deadline != nil` (a positive value), call `attemptCtx, cancel := context.WithTimeout(ctx, *t.deadline); defer cancel()` and run on `attemptCtx`. Never pass `0` (or a negative) to `WithTimeout`.

**Proposed** (Task ai-model-selection-2-2, **Do**, the `attempt` bullet):
> - In `attempt` (~lines 147-156), apply the deadline conditionally off the INTERNAL carrier (which `NewTransport` has already mapped so nil = no-deadline, non-nil = a positive deadline): when `t.deadline == nil` (the no-deadline case — i.e. the operator passed an explicit `&0`, which `NewTransport` translated to nil), run `t.runner.RunWith(ctx, …)` on the PARENT context — do NOT call `context.WithTimeout`. When `t.deadline != nil`, call `attemptCtx, cancel := context.WithTimeout(ctx, *t.deadline); defer cancel()` and run on `attemptCtx`. Because the `NewTransport` mapping guarantees `t.deadline` is non-nil only for a strictly positive duration, `0` (and any negative) is never passed to `WithTimeout`.

**Current** (Task ai-model-selection-2-2, **Acceptance Criteria**, AC 6 — "No deadline" reachable only via explicit `0`):
> - [ ] "No deadline" is reachable only via an explicit `0`, never by a nil/forgotten field — the boundary contract is pinned by a test and documented in the comment.

**Proposed** (Task ai-model-selection-2-2, **Acceptance Criteria**, AC 6 — "No deadline" reachable only via explicit `0`):
> - [ ] "No deadline" is reachable only via an explicit boundary `&0` (which `NewTransport` maps to a nil internal `deadline` → parent-context path), never by a nil/forgotten `Config.Timeout` — the boundary→internal mapping is explicit (no direct pointer copy), the contract is pinned by a test, and documented in the comment.

**Resolution**: Fixed
**Notes**: Approved via auto. Applied to Task 2-2 (tick-d69d1b) and the phase-2-tasks.md mirror.

---

### 2. Task 2-2's explicit-zero test constructs `ptrTo(0)`, which yields a `*int` but `ai.Config.Timeout` is `*time.Duration` — the test code will not compile

**Severity**: Minor
**Plan Reference**: Phase 2, Task ai-model-selection-2-2 ("Make `ai.Config.Timeout` carry absent-vs-explicit-zero and apply the per-attempt deadline conditionally"), **Tests** section
**Category**: Acceptance Criteria Quality (concrete test content correctness)
**Change Type**: update-task

**Details**:
Two of Task 2-2's test snippets construct the explicit-zero `Config.Timeout` with `ptrTo(0)`. The task's own helper bullet defines `ptrTo[T any](v T) *T`, so `ptrTo(0)` infers `T = int` and returns a `*int`. But Do bullet 1 changes `ai.Config.Timeout` to `*time.Duration`, so `ai.Config{… Timeout: ptrTo(0)}` is a `*int` assigned to a `*time.Duration` field — a compile error. The no-deadline value must be a typed zero duration: `ptrTo(time.Duration(0))` (or the local `dur := time.Duration(0); &dur`). The plan should carry correct example code since the tracking file's content is applied verbatim.

This edits Task 2-2's Tests section only — the two `ptrTo(0)` occurrences become `ptrTo(time.Duration(0))` — leaving everything else unchanged.

**Current** (Task ai-model-selection-2-2, **Tests**, the explicit-zero no-instant-timeout test):
> - `"it skips context.WithTimeout and runs on the parent context when the timeout is an explicit zero (no instant timeout)"` — construct with `ai.Config{AICommand: <a real script>, Timeout: ptrTo(0)}` where the script sleeps briefly (e.g. an `os.WriteFile` script `#!/bin/sh\nsleep 0.2\necho ok` via `runner.NewExecRunner()`, mirroring `TestTransport_Generate_RealDeadlineKillIsNonRetriedTimeout`'s setup) and assert `Generate` returns the body successfully — proving a zero did NOT fire an immediate deadline. (A pure FakeRunner cannot prove "no instant timeout" because it never blocks; use the exec-runner script for the no-deadline proof, OR assert structurally that `attempt` passed the parent ctx — pick the FakeRunner structural assertion if an exec script is too heavy, but the spec's instant-timeout regression is best caught by the real-runner sleep.)

**Proposed** (Task ai-model-selection-2-2, **Tests**, the explicit-zero no-instant-timeout test):
> - `"it skips context.WithTimeout and runs on the parent context when the timeout is an explicit zero (no instant timeout)"` — construct with `ai.Config{AICommand: <a real script>, Timeout: ptrTo(time.Duration(0))}` (a typed zero duration; `ptrTo(0)` would be a `*int` and not assignable to the `*time.Duration` field) where the script sleeps briefly (e.g. an `os.WriteFile` script `#!/bin/sh\nsleep 0.2\necho ok` via `runner.NewExecRunner()`, mirroring `TestTransport_Generate_RealDeadlineKillIsNonRetriedTimeout`'s setup) and assert `Generate` returns the body successfully — proving a zero did NOT fire an immediate deadline. (A pure FakeRunner cannot prove "no instant timeout" because it never blocks; use the exec-runner script for the no-deadline proof, OR assert structurally that `attempt` passed the parent ctx — pick the FakeRunner structural assertion if an exec script is too heavy, but the spec's instant-timeout regression is best caught by the real-runner sleep.)

**Current** (Task ai-model-selection-2-2, **Tests**, the parent-cancellation test):
> - `"it propagates a parent cancellation unchanged on the no-deadline path"` — with `Timeout: ptrTo(0)`, seed the FakeRunner to return `fmt.Errorf("running claude: %w", context.Canceled)`; assert the error `errors.Is(context.Canceled)` and matches NO transport sentinel, and the command ran exactly once (not retried).

**Proposed** (Task ai-model-selection-2-2, **Tests**, the parent-cancellation test):
> - `"it propagates a parent cancellation unchanged on the no-deadline path"` — with `Timeout: ptrTo(time.Duration(0))` (a typed zero duration, not `ptrTo(0)` which is a `*int`), seed the FakeRunner to return `fmt.Errorf("running claude: %w", context.Canceled)`; assert the error `errors.Is(context.Canceled)` and matches NO transport sentinel, and the command ran exactly once (not retried).

**Resolution**: Fixed
**Notes**: Approved via auto. Applied to Task 2-2 (tick-d69d1b) and the phase-2-tasks.md mirror.

---
