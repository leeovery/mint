---
phase: 2
phase_name: Transport adoption and wiring
total: 6
---

## ai-model-selection-2-1 | approved

### Task ai-model-selection-2-1: Delete the transport's command self-default and correct its comments

**Problem**: The transport (`internal/ai/transport.go`) carries its own `defaultAICommand = "claude -p"` const and re-defaults a blank/whitespace `AICommand` to it inside `NewTransport`. Phase 1 made `config.AICommandFor(verb)` the single source of truth — the config floor (`DefaultAICommand = "claude -p --model sonnet"`) always supplies a valid, non-empty command, and blank-skipping now happens once in the config accessor across all layers. The transport's duplicate default and its blank-re-default path are therefore dead code: they can only ever see an already-resolved, non-empty command in production, and the duplicate const re-introduces exactly the `claude -p` literal the de-duplication target removes. The transport's WHY-comments still hard-encode the deleted "(default `claude -p` when empty)" / "An empty AICommand resolves to `claude -p`" contracts, which CLAUDE.md requires be corrected in the same change.

**Solution**: Delete the `defaultAICommand` const and the `strings.TrimSpace(command) == "" → defaultAICommand` re-default block in `NewTransport`, so the transport runs the concrete command config hands it verbatim. Correct the `Config.AICommand` doc comment, the `NewTransport` doc comment, and the `parseCommand` comment so they no longer claim the transport defaults an empty command — they state instead that config's floor guarantees a non-empty command and the transport never re-defaults. The timeout-side self-default deletion and conditional deadline are Task 2-2; this task is the command-side half only.

**Outcome**: `internal/ai/transport.go` has no `defaultAICommand` const; `NewTransport` assigns `cfg.AICommand` to the Transport's `command` field without trimming or re-defaulting; the literal `"claude -p"` no longer appears anywhere in the transport; the transport's command-side comments match as-built (config owns the default and the blank-skip).

**Do**:
- In `internal/ai/transport.go`, delete the `defaultAICommand` const (~line 45) and its WHY-comment (~lines 43-45).
- In `NewTransport` (~lines 79-89), remove the `command := cfg.AICommand; if strings.TrimSpace(command) == "" { command = defaultAICommand }` block — assign the command straight from `cfg.AICommand`. Leave the timeout handling untouched for now (Task 2-2 owns it). If removing the trim makes `strings` unused, do NOT remove the import yet — `parseCommand`/`isValid`/`attempt` still use `strings.Fields`/`strings.TrimSpace`/`strings.NewReader`, so the import stays; verify with `go build`.
- Rewrite the `Config.AICommand` doc comment (~lines 47-51): drop "(default `claude -p` when empty)"; state that `AICommand` is the resolved command config hands the transport — config's floor (`config.DefaultAICommand`) guarantees it is non-empty, and the multi-layer blank-skip lives in `config.AICommandFor`, so the transport never re-defaults. Keep the "whitespace-split into name + args; see Generate" note.
- Rewrite the `NewTransport` doc comment (~lines 74-78): drop "An empty AICommand resolves to `claude -p` …" — state that `NewTransport` runs the command config resolves and hands it (config guarantees non-empty), with the runner injected for production/test. (The "non-positive Timeout resolves to the ~60s production default" clause is removed in Task 2-2; for THIS task either leave the timeout clause as-is or trim it minimally — do not introduce the conditional-deadline wording yet, which is 2-2's.)
- Update the `parseCommand` comment (~lines 194-198): the parenthetical "(already guarded against in NewTransport)" referring to an all-whitespace command is now false — `NewTransport` no longer guards blank. State instead that config's floor guarantees a non-empty command, so an empty `name` is unreachable from production; the empty-fields branch remains a defensive no-op.

**Acceptance Criteria**:
- [ ] The `defaultAICommand` const is deleted from `internal/ai/transport.go`; the literal `"claude -p"` appears nowhere in the file.
- [ ] `NewTransport` assigns the Transport's `command` field directly from `cfg.AICommand` with no trim/re-default branch.
- [ ] A blank/whitespace `AICommand` is no longer re-defaulted by the transport (it is passed through unchanged — production never hits this because config's floor is non-empty).
- [ ] The `Config.AICommand`, `NewTransport`, and `parseCommand` comments no longer claim the transport defaults an empty command; they state config owns the default and the blank-skip.
- [ ] `go build ./...`, `gofmt -l .` (prints nothing), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues) all pass.

**Tests**:
- `"it runs the passed ai_command verbatim without re-defaulting"` — `ai.NewTransport(r, ai.Config{AICommand: "mybot gen", Timeout: generousTimeout})`, seed `mybot`, assert the recorded invocation is `mybot` with args `["gen"]` and no `claude` invocation occurred.
- `"it passes a blank AICommand through unchanged (transport no longer re-defaults; production never sends blank)"` — construct with `ai.Config{AICommand: "  ", Timeout: generousTimeout}`, seed an empty-name binary path, and assert the transport does NOT substitute `claude` (the old blank-re-default is gone). This proves the dead path is removed; production safety is guaranteed by config's floor, asserted in the config-layer tests.
- Migrate `TestTransport_Generate_DefaultCommandIsClaudeDashP` in this task only insofar as it constructs `ai.Config{Timeout: generousTimeout}` (no `AICommand`) and asserts the transport defaults to `claude -p`. Since the transport no longer defaults, that test's premise is gone — either delete it or repoint it to assert that an explicitly-passed command drives the argv (the canonical default-command coverage now lives in `config`'s `AICommandFor` zero-config test). NOTE: the FULL test-pin migration sweep (engine/commit argv pins) is Task 2-6; here only fix what this file's deletion directly breaks to keep `go test` green.

**Edge Cases**:
- Blank/whitespace `AICommand` no longer re-defaulted in the transport (per the task table) — config's floor guarantees non-empty, so the transport carries the value verbatim.
- The empty-name `parseCommand` path is now unreachable from production (per the task table) — the defensive empty-fields branch stays but the comment no longer claims `NewTransport` guards it.

**Context**:
> Spec — Single source of truth for config defaults: "The transport carries no defaults. It runs the concrete command/timeout that config resolves and hands it. The duplicate `defaultAICommand` in `internal/ai/transport.go` is deleted; since config's floor always supplies a valid command, the transport never re-defaults."
> Spec — Resolution value semantics (`ai_command`): "Consequently the transport's old 'empty → re-default / empty → fail-loud' path becomes unreachable and is removed: config's floor always supplies a valid command. … This multi-layer trim-and-skip replaces the transport's old single blank-re-default … the whitespace-blank detection moves out of the transport into config."
> Spec — De-duplication target: "`defaultAICommand = "claude -p"` is currently duplicated across `internal/config/config.go`, `internal/ai/transport.go`, and `internal/initgen/initgen.go` … After this work the value lives canonically in `internal/config` and the other sites derive from it."
> Spec — Migration & mechanical carry-overs (Transport doc-comment migration, same-change per CLAUDE.md): "`Config.AICommand` — '(default `claude -p` when empty)'" and "`NewTransport` — 'An empty AICommand resolves to `claude -p` …'" must be corrected in the same change. (The `Config.Timeout` and `Generate`/`attempt` comment corrections are Task 2-2's, as they pair with the deadline change.)
> As-built: `NewTransport` (~lines 79-89) currently does `command := cfg.AICommand; if strings.TrimSpace(command) == "" { command = defaultAICommand }`. `parseCommand` (~lines 194-205) returns an empty name for an all-whitespace command and its comment says "(already guarded against in NewTransport)". Phase 1 exported `config.DefaultAICommand = "claude -p --model sonnet"` as the canonical floor.

**Spec Reference**: `.workflows/ai-model-selection/specification/ai-model-selection/specification.md` — Single source of truth for config defaults (transport carries no defaults; de-duplication target); Resolution value semantics (`ai_command`); Migration & mechanical carry-overs (Transport doc-comment migration).

## ai-model-selection-2-2 | approved

### Task ai-model-selection-2-2: Make `ai.Config.Timeout` carry absent-vs-explicit-zero and apply the per-attempt deadline conditionally

**Problem**: The transport applies its per-attempt deadline unconditionally via `context.WithTimeout(ctx, t.timeout)` and self-defaults any non-positive `Config.Timeout` to its own `defaultTimeout = 60s`. Phase 1 made `config.TimeoutFor(verb)` the source of truth and introduced an operator-chosen `timeout = 0` meaning "no time limit" — the transport must learn to SKIP `WithTimeout` entirely on `0` (passing a zero duration to `WithTimeout` fires immediately, producing instant timeouts) and run the attempt on the parent context, while a positive value still uses `WithTimeout`. Critically, `ai.Config.Timeout` is today a plain `time.Duration` that every wiring site leaves at its zero value; once the self-default is deleted, a `time.Duration` zero is ambiguous — it is BOTH the operator's explicit "no deadline" AND the value a wiring site produces by forgetting to thread the resolved timeout. A forgotten field silently running unbounded would invert "fail loud, never hang" by omission. The boundary type must distinguish absent from explicit-`0`.

**Solution**: Change `ai.Config.Timeout` to a type that distinguishes "not set" from an explicit value — `*time.Duration` (matching the `*time.Duration` Phase 1's `TimeoutFor` returns, so the three wiring sites assign the accessor's return directly with no lossy conversion). Delete the transport's `defaultTimeout` const and the `timeout <= 0 → defaultTimeout` re-default. Carry the resolved timeout onto the `Transport` as a value that records BOTH the duration and whether a deadline applies (e.g. a `*time.Duration` field, or a `time.Duration` plus a `deadline bool`). In `attempt`, apply `context.WithTimeout` ONLY when a positive deadline is set; when the timeout is the explicit-`0`/no-deadline case, run on the parent `ctx` directly. Correct the `Config.Timeout`, `NewTransport`, and `Generate`/`attempt` WHY-comments to match.

**Outcome**: `ai.Config.Timeout` is a `*time.Duration`; a nil pointer means "not set" and an explicit `0` (a pointer to `0`) means "no deadline"; the transport skips `context.WithTimeout` and runs on the parent context when the resolved timeout is the no-deadline case, and uses `context.WithTimeout(ctx, d)` for a positive `d`; `defaultTimeout` is deleted; `context.Canceled` still propagates unchanged on both the deadline and no-deadline paths.

**Do**:
- In `internal/ai/transport.go`, change `Config.Timeout` from `time.Duration` to `*time.Duration` (~line 59). This is the absent-vs-explicit-zero boundary: nil = not threaded, `&0` = operator's explicit "no deadline", `&positive` = a real deadline. Phase 1's `config.TimeoutFor(verb)` returns `*time.Duration`, so the wiring sites (Tasks 2-3/2-4/2-5) assign it straight in.
- Decide how `NewTransport` resolves a nil `Config.Timeout`. Per the invariant, a nil here must NOT silently become "no deadline" — that is the zero-by-omission case the spec forbids. Options, in order of preference: (a) keep `NewTransport` STRICT — a nil `Config.Timeout` is a programming error from a wiring site that forgot to thread the value; since all three production sites are migrated in 2-3/2-4/2-5 to pass the accessor's non-nil return, and tests pass an explicit value, nil should not occur. Choose the mechanism that makes the invariant test-pinnable: e.g. store on the Transport a `deadline *time.Duration` copied from `cfg.Timeout` and treat nil as "no deadline ONLY if explicitly nil-by-design" — but prefer making nil unreachable in production by the wiring contract and pinning it with the boundary test below. Document the chosen contract in the `NewTransport` comment so a future caller cannot reintroduce zero-by-omission.
- Delete the `defaultTimeout` const (~lines 62-64) and the `timeout := cfg.Timeout; if timeout <= 0 { timeout = defaultTimeout }` block in `NewTransport` (~lines 84-87). The transport no longer re-defaults; config's floor (`config.DefaultTimeout = 60s`) supplies 60s when the chain is exhausted, and an explicit `0` reaches the transport as the no-deadline case.
- Change the `Transport` struct (~lines 68-72) so it records the per-attempt deadline AND whether one applies. Recommended: a `deadline *time.Duration` field (nil ⇒ no deadline; non-nil positive ⇒ that deadline). Set it in `NewTransport` from `cfg.Timeout`.
- In `attempt` (~lines 147-156), apply the deadline conditionally: when `t.deadline == nil` (the no-deadline case), run `t.runner.RunWith(ctx, …)` on the PARENT context — do NOT call `context.WithTimeout`. When `t.deadline != nil` (a positive value), call `attemptCtx, cancel := context.WithTimeout(ctx, *t.deadline); defer cancel()` and run on `attemptCtx`. Never pass `0` (or a negative) to `WithTimeout`.
- Residual-negative defensiveness: config guarantees the transport receives only a positive value or an explicit `0` (negatives drop through to the 60s floor in `config.TimeoutFor`). If any defensive handling of a stray non-positive-but-nonzero value remains, it must NOT collapse a negative into the `0` no-deadline branch — a negative is not "no deadline". Prefer treating only an explicit `0` as no-deadline and a positive as a deadline; if a representation forces a negative to be handled, it should be coerced to the floor or rejected, never silently disable the deadline.
- Correct the WHY-comments per CLAUDE.md same-change:
  - `Config.Timeout` (~lines 53-56): drop "A zero or negative Timeout falls back to the production default." State the new contract — `Timeout` is a `*time.Duration`: nil means the field was not threaded (a wiring bug; production never produces it because all sites source `config.TimeoutFor`), an explicit `0` (`&0`) means "no per-attempt deadline" (the operator opted into an unbounded AI call), and a positive value is the per-attempt deadline. Note the "no deadline" trade-off (a conscious exception to fail-loud-never-hang, operator-chosen).
  - `NewTransport` (~lines 74-78): state it records the resolved per-attempt deadline (or no-deadline) from `cfg.Timeout`; config owns the default, the transport no longer re-defaults.
  - `Generate`/`attempt` (~lines 99-102 and the `attempt` doc ~lines 144-146): change "Each attempt gets its own deadline via context.WithTimeout(ctx, t.timeout)" to the CONDITIONAL form — each attempt gets its own deadline via `context.WithTimeout` ONLY when a positive timeout is set; when the operator chose `0` (no deadline) the attempt runs on the parent context with no `WithTimeout`. Keep the "prompt re-piped fresh on every attempt" note.
- Preserve cancellation routing: `classifyFatal` already propagates `context.Canceled` unchanged. On the no-deadline path the attempt runs on the parent `ctx`, so a caller cancel still surfaces as `context.Canceled` — confirm with a test that a parent-context cancel on the no-deadline path is NOT swallowed and is NOT mapped to a transport sentinel.

**Acceptance Criteria**:
- [ ] `ai.Config.Timeout` is a `*time.Duration` (nil = not threaded; `&0` = explicit no-deadline; `&positive` = the deadline).
- [ ] The `defaultTimeout` const is deleted; the literal `60 * time.Second` no longer appears in `internal/ai/transport.go`.
- [ ] When `Config.Timeout` is an explicit `0` (no deadline), the transport runs the attempt on the parent context and does NOT call `context.WithTimeout` — a long-running command does NOT produce an instant/immediate timeout.
- [ ] When `Config.Timeout` is a positive value, the transport uses `context.WithTimeout(ctx, value)` and a genuine deadline still fires as `ErrTimeout`.
- [ ] A parent-context cancellation on the no-deadline path propagates as `context.Canceled` unchanged (not swallowed, not mapped to any transport sentinel).
- [ ] No negative is ever collapsed into the `0` no-deadline branch (config guarantees no negative reaches the transport; any residual defensive handling does not silently disable the deadline for a negative).
- [ ] "No deadline" is reachable only via an explicit `0`, never by a nil/forgotten field — the boundary contract is pinned by a test and documented in the comment.
- [ ] The `Config.Timeout`, `NewTransport`, and `Generate`/`attempt` comments are corrected to the conditional-deadline / no-self-default as-built.
- [ ] `go build ./...`, `gofmt -l .` (prints nothing), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues) all pass.

**Tests**:
- `"it skips context.WithTimeout and runs on the parent context when the timeout is an explicit zero (no instant timeout)"` — construct with `ai.Config{AICommand: <a real script>, Timeout: ptrTo(0)}` where the script sleeps briefly (e.g. an `os.WriteFile` script `#!/bin/sh\nsleep 0.2\necho ok` via `runner.NewExecRunner()`, mirroring `TestTransport_Generate_RealDeadlineKillIsNonRetriedTimeout`'s setup) and assert `Generate` returns the body successfully — proving a zero did NOT fire an immediate deadline. (A pure FakeRunner cannot prove "no instant timeout" because it never blocks; use the exec-runner script for the no-deadline proof, OR assert structurally that `attempt` passed the parent ctx — pick the FakeRunner structural assertion if an exec script is too heavy, but the spec's instant-timeout regression is best caught by the real-runner sleep.)
- `"it uses context.WithTimeout with the configured value when the timeout is positive (a real deadline fires as ErrTimeout)"` — reuse the `TestTransport_Generate_RealDeadlineKillIsNonRetriedTimeout` shape: a tiny positive `Timeout` against a long-sleeping script yields `ai.ErrTimeout`.
- `"it propagates a parent cancellation unchanged on the no-deadline path"` — with `Timeout: ptrTo(0)`, seed the FakeRunner to return `fmt.Errorf("running claude: %w", context.Canceled)`; assert the error `errors.Is(context.Canceled)` and matches NO transport sentinel, and the command ran exactly once (not retried).
- `"it does not collapse a negative into the no-deadline branch"` — drive a negative through the transport directly (a unit guard on the transport's deadline handling) OR document that config prevents negatives and pin at the config layer; assert that a negative, if it reached the transport, is treated as a deadline-bearing/floor case, never as no-deadline. (Prefer: assert the only no-deadline trigger is an explicit `0`.)
- `"a nil Config.Timeout is a wiring bug, not a silent no-deadline"` — pin the chosen contract: either `NewTransport` with a nil `Timeout` is documented-unreachable-in-production and the test asserts the wiring sites never produce nil (covered structurally in 2-3/2-4/2-5), or the transport treats nil distinctly from `&0`. Assert that "no deadline" requires `&0`, so a nil does not silently run unbounded.
- Helper: add a tiny `ptrTo[T any](v T) *T` (or a local `dur := 5*time.Second; &dur`) in the test file for constructing `*time.Duration` configs.

**Edge Cases**:
- Explicit `0` skips `context.WithTimeout` (no instant/immediate timeout) — the central behaviour (per the task table).
- Positive uses `WithTimeout` with that value.
- Parent-context cancellation still propagates unchanged on the no-deadline path (per the task table) — running on the parent ctx must not swallow a cancel.
- Residual negative defensive handling must not collapse into the `0` no-deadline branch (per the task table).
- "No deadline" reachable only via explicit `0`, never by a forgotten/zero-by-omission field (per the task table) — the load-bearing invariant: `*time.Duration` distinguishes nil-by-omission from `&0`-by-choice.

**Context**:
> Spec — Resolution value semantics (`timeout`): "Zero is an explicit, honored value meaning 'no time limit' — it disables the per-attempt deadline and stops the fall-through … The transport must learn `timeout = 0` ⇒ no deadline, replacing its current non-positive → 60s re-default. The transport applies the deadline conditionally. When the resolved timeout is `0`, the transport skips `context.WithTimeout` entirely and runs the attempt on the parent context — it must not pass a zero duration to `WithTimeout` (which fires immediately, producing instant timeouts). The current `Timeout <= 0` defensive re-default is therefore split: `== 0` ⇒ no deadline; positive ⇒ `WithTimeout` with that value. Config guarantees the transport receives only a positive value or an explicit `0` (negatives drop through to the 60s floor in config), so no negative reaches the transport; any residual defensive handling of a negative must not collapse it into the `0` no-deadline branch."
> Spec — Resolution value semantics (`timeout`), the load-bearing boundary invariant: "The config→`ai.Config` boundary must preserve absent-vs-explicit-zero for `timeout`. `ai.Config.Timeout` is today a plain `time.Duration` that every wiring site leaves at its zero value … Once that self-default is gone, a `time.Duration` zero is ambiguous — it is both the operator's explicit 'no deadline' and the value a wiring site produces by forgetting to thread the resolved timeout. Invariant: 'no deadline' must only ever be reachable by an operator's explicit `0`, never by a wiring site omitting the field — a forgotten field silently running unbounded would invert 'fail loud, never hang' by omission. Planning picks the mechanism (e.g. give the boundary field a type that distinguishes nil from explicit-`0`, such as `*time.Duration` / a small wrapper …)."
> Spec — Single source of truth: "The transport carries no defaults. … `timeout` is introduced as a net-new config key (today only the transport's `defaultTimeout`, never config-populated). The transport also learns `timeout = 0` ⇒ no deadline."
> Spec — Migration & mechanical carry-overs (Transport doc-comment migration, same-change per CLAUDE.md): "`Config.Timeout` — 'A zero or negative Timeout falls back to the production default.'" and "`Generate`/`attempt` — 'Each attempt gets its own deadline via context.WithTimeout(ctx, t.timeout)' (becomes conditional once `timeout = 0` skips `WithTimeout`)."
> Phase 1 contract (consumed here): `config.TimeoutFor(verb)` returns `*time.Duration` distinguishing the explicit-`0`/no-deadline case from a positive/floor value; `config.DefaultTimeout = 60 * time.Second` is the floor (the transport's old `defaultTimeout` is now deleted and lives canonically in config). Matching `ai.Config.Timeout` to `*time.Duration` lets the wiring sites assign the accessor's return with no lossy conversion.
> As-built: `attempt` (~line 148) does `attemptCtx, cancel := context.WithTimeout(ctx, t.timeout); defer cancel()`. `classifyFatal` (~line 162) routes `context.Canceled` UNCHANGED and `context.DeadlineExceeded` to `ErrTimeout`. `TestTransport_Generate_RealDeadlineKillIsNonRetriedTimeout` (~line 273) shows the exec-runner sleep-script pattern for proving real deadline behaviour deterministically.

**Spec Reference**: `.workflows/ai-model-selection/specification/ai-model-selection/specification.md` — Resolution value semantics (`timeout`, including the conditional-deadline and config→`ai.Config` boundary invariant); Single source of truth for config defaults (transport carries no defaults); Migration & mechanical carry-overs (Transport doc-comment migration).

## ai-model-selection-2-3 | approved

### Task ai-model-selection-2-3: Thread resolved command + timeout through the release wiring site (`internal/engine/release.go`)

**Problem**: The release wiring site `aiTransport` (`internal/engine/release.go` ~line 934) constructs `ai.NewTransport(deps.Runner, ai.Config{AICommand: cfg.AICommand})` — reading the bare shared top-level `cfg.AICommand` and leaving `Timeout` at its zero value, relying on the transport's now-deleted self-defaults. After Phase 1 and Tasks 2-1/2-2, this site must source BOTH the command and the timeout from the per-verb accessors so a `[release].ai_command` / `[release].timeout` override actually drives the release notes AI call, and so the timeout is never zero-by-omission. Its WHY-comment also still claims "NewTransport … re-defaults an empty value to `claude -p`", which is now false.

**Solution**: Change `aiTransport` to construct `ai.Config{AICommand: cfg.AICommandFor(config.VerbRelease), Timeout: cfg.TimeoutFor(config.VerbRelease)}`, threading both resolved per-verb values from Phase 1's accessors. Update the WHY-comment to state it resolves through the release verb's per-key chain (`[release] → shared → floor`) and that config — not the transport — owns the default and the blank-skip/deadline semantics.

**Outcome**: `cfg.AICommandFor(VerbRelease)` drives the release notes AI argv and `cfg.TimeoutFor(VerbRelease)` drives the per-attempt deadline; a `[release].ai_command` override changes the invoked binary/args; the timeout is sourced from the accessor (never zero-by-omission); the comment matches as-built.

**Do**:
- In `internal/engine/release.go`, change the `ai.NewTransport(...)` call in `aiTransport` (~line 934) to pass `ai.Config{AICommand: cfg.AICommandFor(config.VerbRelease), Timeout: cfg.TimeoutFor(config.VerbRelease)}`. `cfg.TimeoutFor` returns `*time.Duration` (Phase 1) which matches `ai.Config.Timeout`'s new `*time.Duration` type (Task 2-2) — assign directly, no conversion. Confirm `config` is already imported (it is — `aiTransport` takes `cfg config.Config`).
- Rewrite the `aiTransport` WHY-comment (~lines 922-929): drop "NewTransport whitespace-splits it into name + args and re-defaults an empty value to `claude -p`". State that the release verb's per-key resolution (`cfg.AICommandFor(config.VerbRelease)` / `cfg.TimeoutFor(config.VerbRelease)`) supplies the concrete command and per-attempt deadline — config owns the floor (`claude -p --model sonnet`, 60s) and the blank-skip/no-deadline semantics; the transport runs what it is handed. Note the timeout is sourced from the accessor (never zero-by-omission), preserving the "no deadline only via explicit `0`" invariant.
- Also update the related comment at `internal/engine/release.go` ~lines 137-138 (the `deps.Transport` doc near the seam, which mentions "re-defaulting an empty value to `claude -p`") if it encodes the same now-false claim — bring it true to as-built (config owns the default).
- Do NOT change the `deps.Transport != nil` test-seam short-circuit — the injected-transport path is unchanged; only the production-construction branch threads the accessors.

**Acceptance Criteria**:
- [ ] `aiTransport` constructs `ai.Config` with `AICommand: cfg.AICommandFor(config.VerbRelease)` and `Timeout: cfg.TimeoutFor(config.VerbRelease)`.
- [ ] A `[release].ai_command` override in `.mint.toml` drives the release notes AI invocation's binary+args (not the bare shared/default).
- [ ] A zero-config run still resolves to `claude -p --model sonnet` for release notes (the pinned default flows through the accessor).
- [ ] The release timeout is sourced from `cfg.TimeoutFor(VerbRelease)` (never left zero-by-omission).
- [ ] The `aiTransport` WHY-comment (and the related seam comment) no longer claim the transport re-defaults an empty value to `claude -p`.
- [ ] `go build ./...`, `gofmt -l .` (prints nothing), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues) all pass.

**Tests**:
- `"it drives the release notes AI invocation from a [release].ai_command override"` — write `[release]\nai_command = "mybot gen --json"`, run the prior-tag normal-AI release path (mirror `TestRelease_AICommand_ConfigValueDrivesTransport` in `release_configconsolidation_test.go`), seed `mybot`, assert `stdinOf(t, f, "mybot", "gen", "--json")` is non-empty and `invokedWith(f, "claude", ...)` is false. This proves the per-verb override (not just the shared top-level) drives the transport.
- `"it falls a release run with no override to the shared ai_command"` — write a top-level `ai_command = "mybot gen"` with no `[release].ai_command`; assert `mybot gen` drives the call (per-verb absent → shared).
- `"it resolves the release notes call to claude -p --model sonnet with no .mint.toml"` — migrate/extend `TestRelease_AICommand_DefaultDrivesTransport`: seed `claude`, assert `stdinOf(t, f, "claude", "-p", "--model", "sonnet")` is non-empty (the new pinned default argv). This is one of the Task 2-6 migrations but is the natural assertion for this site; coordinate the exact-argv string with Task 2-6.
- `"it sources the release timeout from the accessor (a [release].timeout override is threaded)"` — a behaviour-level proof is hard to argv-assert (timeout is not on the command line), so assert at the seam: with `[release].timeout = 0` (no deadline) a long-running seeded AI body still completes (no instant timeout) — OR pin structurally that `aiTransport` passes `cfg.TimeoutFor(VerbRelease)` (a focused unit test calling `aiTransport` directly, asserting the constructed `ai.Config.Timeout` equals the accessor's return). Prefer the focused unit test for determinism.

**Edge Cases**:
- A per-verb `[release]` override drives the argv, not the bare shared/default (per the task table).
- Timeout sourced from the accessor, never zero-by-omission (per the task table) — the `*time.Duration` from `TimeoutFor` is threaded; a forgotten field cannot reach the transport.

**Context**:
> Spec — Migration & mechanical carry-overs (Transport wiring sites): "The resolved per-verb command and timeout must be threaded where today only `ai.Config{AICommand: cfg.AICommand}` is constructed (with `Timeout` left zero): `internal/engine/release.go`, `internal/commit/run.go`, `internal/engine/regenerate_fresh.go`."
> Spec — Config schema: per-verb `ai_command` override (Resolution order): "`[verb].ai_command → top-level shared ai_command → shipped default`." Resolution `[verb].timeout → top-level shared timeout → shipped default`.
> Spec — Acceptance criteria — resolution behaviors: "Pinned default applies for zero-config — with no `.mint.toml`, both verbs resolve to `claude -p --model sonnet` and the 60s timeout."
> Spec — the boundary invariant: "all three wiring sites must source the timeout from the accessor (never zero-by-omission)."
> Phase 1 contracts consumed: `config.VerbRelease`, `cfg.AICommandFor(verb) string`, `cfg.TimeoutFor(verb) *time.Duration`. Task 2-2 makes `ai.Config.Timeout` a `*time.Duration`, so `TimeoutFor`'s return assigns directly.
> As-built: `aiTransport` (~line 930) and the call at `notes.NewGenerator(assembler, aiTransport(deps, cfg), root)` (~line 813). The `deps.Transport != nil` short-circuit is the test seam — leave it. `TestRelease_AICommand_ConfigValueDrivesTransport` / `TestRelease_AICommand_DefaultDrivesTransport` (`release_configconsolidation_test.go`) are the existing argv pins for this site; `invokedWith`/`stdinOf` match the FULL joined command line and FakeRunner `Seed` is keyed by binary name only.

**Spec Reference**: `.workflows/ai-model-selection/specification/ai-model-selection/specification.md` — Migration & mechanical carry-overs (Transport wiring sites); Config schema: per-verb `ai_command` override / `timeout` key (resolution order); Acceptance criteria — resolution behaviors.

## ai-model-selection-2-4 | approved

### Task ai-model-selection-2-4: Thread resolved command + timeout through the commit wiring site (`internal/commit/run.go`)

**Problem**: The commit wiring site `commitTransport` (`internal/commit/run.go` ~line 772) constructs `ai.NewTransport(deps.Runner, ai.Config{AICommand: cfg.AICommand})` — reading the bare shared top-level command and leaving `Timeout` zero. After Phase 1 and Tasks 2-1/2-2, `mint commit` must resolve BOTH its command and timeout through the `[commit]` verb chain so a `[commit].ai_command` / `[commit].timeout` override drives the commit-message AI call, and so the timeout is never zero-by-omission. Its WHY-comment also still claims the transport "re-defaults an empty value to `claude -p`", now false.

**Solution**: Change `commitTransport` to construct `ai.Config{AICommand: cfg.AICommandFor(config.VerbCommit), Timeout: cfg.TimeoutFor(config.VerbCommit)}`, threading both resolved per-verb values. Update the WHY-comment to state it resolves through the commit verb's per-key chain (`[commit] → shared → floor`) and that config owns the default and the blank-skip/deadline semantics.

**Outcome**: `cfg.AICommandFor(VerbCommit)` drives the commit-message AI argv and `cfg.TimeoutFor(VerbCommit)` drives the deadline; a `[commit].ai_command` override changes the invoked binary/args; the timeout is sourced from the accessor (never zero-by-omission); the comment matches as-built.

**Do**:
- In `internal/commit/run.go`, change the `ai.NewTransport(...)` call in `commitTransport` (~line 772) to pass `ai.Config{AICommand: cfg.AICommandFor(config.VerbCommit), Timeout: cfg.TimeoutFor(config.VerbCommit)}`. `cfg.TimeoutFor` returns `*time.Duration` (Phase 1), matching `ai.Config.Timeout`'s `*time.Duration` (Task 2-2) — assign directly. `config` is already imported (`commitTransport` takes `cfg config.Config`).
- Rewrite the `commitTransport` WHY-comment (~lines 763-767): drop "NewTransport whitespace-splits it into name + args and re-defaults an empty value to `claude -p`". State that the commit verb's per-key resolution (`cfg.AICommandFor(config.VerbCommit)` / `cfg.TimeoutFor(config.VerbCommit)`) supplies the concrete command and per-attempt deadline — config owns the floor and semantics; the transport runs what it is handed; the timeout is sourced from the accessor (never zero-by-omission).
- Check the related comment at `internal/commit/run.go` ~line 204 (mentions "driving it with the validated cfg.AICommand") and ~line 707 ("the validated cfg.AICommand — so production leaves deps.Transport nil") — bring any now-false "shared cfg.AICommand drives it" / "re-defaults to claude -p" wording true to as-built (per-verb resolution via `AICommandFor(VerbCommit)`).
- Do NOT change the `deps.Transport != nil` test-seam short-circuit.

**Acceptance Criteria**:
- [ ] `commitTransport` constructs `ai.Config` with `AICommand: cfg.AICommandFor(config.VerbCommit)` and `Timeout: cfg.TimeoutFor(config.VerbCommit)`.
- [ ] A `[commit].ai_command` override drives the commit-message AI invocation's binary+args (not the bare shared/default).
- [ ] A zero-config `mint commit` still resolves to `claude -p --model sonnet`.
- [ ] The commit timeout is sourced from `cfg.TimeoutFor(VerbCommit)` (never left zero-by-omission).
- [ ] The `commitTransport` WHY-comment (and related comments at ~204/~707) no longer claim the transport re-defaults an empty value to `claude -p`.
- [ ] `go build ./...`, `gofmt -l .` (prints nothing), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues) all pass.

**Tests**:
- `"it drives the commit-message AI invocation from a [commit].ai_command override"` — drive `commit.Run` (or the lower `commitTransport`) with a config carrying `[commit].ai_command = "mybot gen --json"` over a FakeRunner; seed `mybot`; assert the recorded AI invocation is `mybot gen --json` and no `claude` invocation occurred. Use the existing commit test harness (FakeRunner + RecordingPresenter); the AI call is the commit-message generation in `generateMessage`/the generator.
- `"it falls a commit run with no override to the shared ai_command"` — top-level `ai_command = "mybot gen"`, no `[commit].ai_command`; assert `mybot gen` drives the call.
- `"it resolves the commit-message call to claude -p --model sonnet with no .mint.toml"` — zero-config commit; assert the AI invocation is `claude -p --model sonnet` (coordinate the exact-argv with Task 2-6, which migrates the `claude -p` pins). The existing `TestGenerator_Generate_ConsumesL2OneRetryNotReimplemented` constructs `ai.NewTransport(r, ai.Config{AICommand: "claude -p"})` explicitly — that stays an explicit-value test (not a default), so it is unaffected by the default change but is part of the Task 2-6 sweep only if its literal needs updating for consistency (it asserts retry behaviour, not the default, so the `"claude -p"` literal may stay).
- `"it sources the commit timeout from the accessor (a [commit].timeout override is threaded)"` — a focused unit test calling `commitTransport(deps, cfg)` with `[commit].timeout` set, asserting the constructed transport carries the accessor's resolved deadline (prefer the structural assertion over a timing-based test, for determinism).

**Edge Cases**:
- A per-verb `[commit]` override drives the argv, not the bare shared/default (per the task table).
- Timeout sourced from the accessor, never zero-by-omission (per the task table).

**Context**:
> Spec — Migration & mechanical carry-overs (Transport wiring sites): the three sites construct `ai.Config{AICommand: cfg.AICommand}` with `Timeout` left zero — the resolved per-verb command and timeout must be threaded. `internal/commit/run.go` is one of the three.
> Spec — Config schema: per-verb `ai_command` override: "`[commit]` simply mirrors `[release]` — same override keys, same resolution, no commit-specific asymmetry."
> Spec — Cross-spec reconciliation (commit spec): promoting per-verb `[commit].ai_command` is the appearance of the "real need" the commit spec said to wait for — this task is the code-level consumption (the `Commit` struct doc-comment reconciliation is Phase 3, NOT here).
> Spec — the boundary invariant: "all three wiring sites must source the timeout from the accessor (never zero-by-omission)."
> Phase 1 contracts consumed: `config.VerbCommit`, `cfg.AICommandFor`, `cfg.TimeoutFor` (`*time.Duration`). Task 2-2's `ai.Config.Timeout` is `*time.Duration`.
> As-built: `commitTransport` (~line 768) is called at `NewGenerator(deps.Runner, commitTransport(deps, cfg), root, deps.Staging)` (~lines 709, 722). The `deps.Transport != nil` short-circuit is the test seam.

**Spec Reference**: `.workflows/ai-model-selection/specification/ai-model-selection/specification.md` — Migration & mechanical carry-overs (Transport wiring sites); Config schema: per-verb `ai_command` override (`[commit]` mirrors `[release]`); Cross-spec reconciliation (commit spec) — code-level consumption.

## ai-model-selection-2-5 | approved

### Task ai-model-selection-2-5: Route the regenerate wiring site through the release verb (`internal/engine/regenerate_fresh.go`)

**Problem**: `resolveFreshTransport` (`internal/engine/regenerate_fresh.go` ~line 130) is the EASY-MISS third construction site: it builds `ai.NewTransport(r, ai.Config{AICommand: cfg.AICommand})` for `mint release regenerate --fresh`. Because `regenerate` re-runs the release-notes task, it MUST resolve through the `[release]` verb table — NOT through its own table (there is none) and NOT through the bare shared/default. A naive edit that forgets this site, or routes it to a non-release verb, would silently make regenerate ignore a `[release]` override. Its WHY-comment also still claims the transport "re-defaults an empty value to `claude -p`".

**Solution**: Change `resolveFreshTransport` to construct `ai.Config{AICommand: cfg.AICommandFor(config.VerbRelease), Timeout: cfg.TimeoutFor(config.VerbRelease)}`, deliberately resolving through `config.VerbRelease` (regenerate rides on `[release]`). Update the WHY-comment to state regenerate routes through the RELEASE verb specifically and why (it re-runs the release-notes task; there is no `[regenerate]` table).

**Outcome**: The fresh-regenerate AI call resolves the `[release]` command and timeout; argv asserted to carry the release values (a `[release].ai_command` override drives the regenerate AI invocation, not the bare shared/default); the comment records the deliberate release routing; the timeout is sourced from the accessor (never zero-by-omission).

**Do**:
- In `internal/engine/regenerate_fresh.go`, change the `ai.NewTransport(...)` call in `resolveFreshTransport` (~line 130) to pass `ai.Config{AICommand: cfg.AICommandFor(config.VerbRelease), Timeout: cfg.TimeoutFor(config.VerbRelease)}`. Use `config.VerbRelease` explicitly — the closed enum has no `regenerate` value, so this is the ONLY correct verb (Phase 1's enum makes a wrong verb a compile error if someone invents one, and there is no regenerate value to mis-route to). `config` is already imported (`resolveFreshTransport` takes `cfg config.Config`).
- Rewrite the `resolveFreshTransport` WHY-comment (~lines 120-125): drop "NewTransport re-defaults an empty value to `claude -p`". State that the fresh-regenerate path resolves through the RELEASE verb (`cfg.AICommandFor(config.VerbRelease)` / `cfg.TimeoutFor(config.VerbRelease)`) because regenerate re-runs the release-notes task and shares release's salience needs and timeout exposure — there is no `[regenerate]` table, so it deliberately reads `[release]`. Config owns the default/semantics; the transport runs what it is handed; the timeout is sourced from the accessor (never zero-by-omission).
- Do NOT change the `transport != nil` test-seam short-circuit (the injected-fake path used by `RegenerateFreshBody`/`RegenerateFreshRegenerator`'s unit tests). Only the production branch (called with a nil transport from `cmd/mint/regenerate_all.go` and `cmd/mint/regenerate_run.go`) threads the accessors.

**Acceptance Criteria**:
- [ ] `resolveFreshTransport` constructs `ai.Config` with `AICommand: cfg.AICommandFor(config.VerbRelease)` and `Timeout: cfg.TimeoutFor(config.VerbRelease)` — the RELEASE verb, not the bare shared/default.
- [ ] A `[release].ai_command` override drives the fresh-regenerate AI invocation's binary+args (argv asserted to carry the release values).
- [ ] A `[commit].ai_command` override does NOT affect the fresh-regenerate call (regenerate reads `[release]`, never `[commit]`).
- [ ] A zero-config fresh-regenerate still resolves to `claude -p --model sonnet`.
- [ ] The fresh-regenerate timeout is sourced from `cfg.TimeoutFor(VerbRelease)` (never left zero-by-omission).
- [ ] The `resolveFreshTransport` WHY-comment records the deliberate release-verb routing (and the no-`[regenerate]`-table rationale) and no longer claims the transport re-defaults an empty value.
- [ ] `go build ./...`, `gofmt -l .` (prints nothing), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues) all pass.

**Tests**:
- `"it drives the fresh-regenerate AI invocation through a [release].ai_command override"` — call `engine.RegenerateFreshBody(ctx, f, nil, root, cfg, res)` with `transport=nil` (the PRODUCTION path, so `resolveFreshTransport` builds the real `ai.Transport` over the FakeRunner) and a `cfg` carrying `Release.AICommand` = `"mybot gen --json"` plus a resolvable, non-first-release `version.Resolution` (set `PreviousTag`/`Tag` so `res.FirstRelease` is false and `DiffRange()` is non-empty); seed the three fresh git calls (`seedFreshGit` shape) and seed `mybot`; assert `stdinOf(t, f, "mybot", "gen", "--json")` is non-empty (the assembled prompt reached the release-resolved command) and `invokedWith(f, "claude", ...)` is false. This is the key proof: the easy-miss site routes through `[release]`. NOTE: existing `regenerate_fresh_test.go` tests inject a recording `freshTransport` (the test seam) so they bypass `resolveFreshTransport` — this NEW test must pass `nil` to exercise the production wiring.
- `"a [commit].ai_command override does not drive the fresh-regenerate call"` — same setup but with `Commit.AICommand` = `"wrongbot"` and NO `[release]` override and NO shared override: assert the call resolves to `claude -p --model sonnet` (the floor), NOT `wrongbot` — proving regenerate reads `[release]`, never `[commit]`.
- `"it resolves the fresh-regenerate call to claude -p --model sonnet with no .mint.toml"` — `transport=nil`, zero-config `cfg` (use `config.Load` on an empty dir or a `defaults()`-equivalent config), seed `claude`, assert the invocation is `claude -p --model sonnet` (coordinate exact-argv with Task 2-6).
- `"it sources the fresh-regenerate timeout from the release accessor"` — a focused assertion that `resolveFreshTransport(r, nil, cfg)` (or the constructed transport) carries `cfg.TimeoutFor(config.VerbRelease)`; prefer a structural/unit assertion over a timing test.

**Edge Cases**:
- Regenerate resolves the `[release]` values — argv asserted to carry release's command/timeout, not the bare shared/default (per the task table). This is the distinct, easy-miss construction site.
- A `[commit]` override must not leak into the regenerate call — regenerate reads `[release]` exclusively (the closed enum has no regenerate value, making mis-routing structurally impossible).

**Context**:
> Spec — Config schema: per-verb `ai_command` override (Verb config space): "`regenerate` is not a separate verb. `mint release regenerate --fresh` re-runs the release-notes task, so it resolves through `[release]`'s `ai_command` — there is no `[regenerate]` table. (Regenerating with a different model than you released with would be odd, and it shares release's salience needs and timeout exposure.)"
> Spec — Single source of truth (typed verb enum): "makes the regenerate routing un-missable: there is no `regenerate` value, so `internal/engine/regenerate_fresh.go` can only pass the release verb."
> Spec — Migration & mechanical carry-overs (Transport wiring sites): "`internal/engine/regenerate_fresh.go` — a distinct construction site that must deliberately resolve through `[release]` (per 'regenerate rides on `[release]`'), not its own table. An easy miss."
> Spec — Acceptance criteria — resolution behaviors: "Regenerate routes through `[release]` — `internal/engine/regenerate_fresh.go` resolves the release command/timeout; argv asserted to carry the release values, not the bare shared/default (the 'easy miss' wiring site)."
> Phase 1 contracts consumed: `config.VerbRelease`, `cfg.AICommandFor`, `cfg.TimeoutFor` (`*time.Duration`). Task 2-2's `ai.Config.Timeout` is `*time.Duration`.
> As-built: `resolveFreshTransport` (~line 126) is called from `freshGenerator` (~line 103), which `RegenerateFreshBody`/`RegenerateFreshRegenerator` build with a `transport` argument — production passes `nil` (from `cmd/mint/regenerate_all.go` ~lines 107/124 and `cmd/mint/regenerate_run.go`), tests inject a recording `freshTransport`. `seedFreshGit(diff, nameStatus, numstat)` (`regenerate_fresh_test.go` ~line 52) scripts the three ordered git calls; `version.Resolution.DiffRange()` produces the `{PreviousTag}..{Tag}` range and `res.FirstRelease` short-circuits to the fixed body with NO AI — so the new production-path test MUST use a non-first-release resolution to reach the AI call.

**Spec Reference**: `.workflows/ai-model-selection/specification/ai-model-selection/specification.md` — Config schema: per-verb `ai_command` override (Verb config space; regenerate rides on `[release]`); Single source of truth (typed verb enum, un-missable regenerate routing); Migration & mechanical carry-overs (Transport wiring sites); Acceptance criteria — resolution behaviors (Regenerate routes through `[release]`).

## ai-model-selection-2-6 | approved

### Task ai-model-selection-2-6: Migrate the old `claude -p` default-argv test pins to `claude -p --model sonnet`

**Problem**: The codebase pins the OLD shipped default `claude -p` (no `--model`) as exact argv across the engine and commit tests. Changing the pinned default (Phase 1) and deleting the transport's `defaultAICommand` (Task 2-1) make those pins assert a default that no longer exists — they will fail or assert the wrong argv. Project test idioms assert exact argv / rendered lines, so these are known, bounded edits enumerated in the spec's Migration section. They must be migrated so the suite proves the NEW pinned default `claude -p --model sonnet` and so the transport's ex-default-command test asserts the PASSED command verbatim rather than a transport-supplied default.

**Solution**: Sweep the enumerated test sites that assert `claude` invoked with exactly `["-p"]` (or `stdinOf(... "claude", "-p")` / `invokedWith(... "claude", "-p")`) for the ZERO-CONFIG / DEFAULT case, and update them to the new pinned argv `["-p", "--model", "sonnet"]`. Leave FakeRunner `Seed("claude", …)` calls untouched (seeds are keyed by binary name only — `claude` still matches). Leave tests that assert an EXPLICIT operator command (e.g. `"mybot gen --json"`, or `ai.Config{AICommand: "claude -p"}` constructed deliberately) unchanged — those assert a passed value, not the default. Repoint the transport's ex-default-command test to assert a passed command verbatim.

**Outcome**: Every test that pins the zero-config/default AI argv asserts `claude -p --model sonnet`; FakeRunner seeds still match (only exact-argv assertions changed); the transport's default-command test no longer asserts a transport default; `go test -race ./...` is green across the repo.

**Do**:
- Enumerate and update the default-argv assertion sites (these assert the DEFAULT, so they migrate; the `Seed("claude", …)` lines on the SAME tests stay because seeds match by binary name only):
  - `internal/engine/release_configconsolidation_test.go` ~line 87-88: `stdinOf(t, f, "claude", "-p")` → `stdinOf(t, f, "claude", "-p", "--model", "sonnet")` in `TestRelease_AICommand_DefaultDrivesTransport`. (The `invokedWith(f, "claude", "-p")` NEGATIVE assertion at ~line 57 in `TestRelease_AICommand_ConfigValueDrivesTransport` asserts the default was NOT invoked when a custom command is configured — update it to the new default argv `invokedWith(f, "claude", "-p", "--model", "sonnet")` so it still means "the default was not used".)
  - `internal/engine/release_priortag_test.go` ~lines 133, 520: `stdinOf(t, f, "claude", "-p")` → `stdinOf(t, f, "claude", "-p", "--model", "sonnet")` (the zero-config prior-tag normal-AI paths that exercise the default).
  - `internal/engine/release_dryrun_test.go` ~line 189: `invokedWith(f, "claude", "-p")` → `invokedWith(f, "claude", "-p", "--model", "sonnet")`.
  - Any other `"claude", "-p"` exact-argv assertion surfaced by the grep below that is a DEFAULT-case assertion (not an explicit-command test).
- `internal/ai/transport_test.go`: handle `TestTransport_Generate_DefaultCommandIsClaudeDashP` (~line 78). The transport no longer defaults (Task 2-1), so this test's premise is gone. Either (a) delete it (the zero-config default now lives in `config`'s `AICommandFor` test), or (b) repoint it to `TestTransport_Generate_RunsPassedCommandVerbatim`: construct `ai.NewTransport(r, ai.Config{AICommand: "claude -p --model sonnet", Timeout: ...})`, seed `claude`, and assert the recorded invocation is `claude` with args `["-p", "--model", "sonnet"]` — proving the transport runs the PASSED command verbatim (the canonical-default proof is the config layer's job). If Task 2-1 already adjusted this test, reconcile so there is exactly one coherent version. Also update the helpers `newTransport`/the `ai.Config{Timeout: generousTimeout}` constructions to pass `*time.Duration` (Task 2-2 changed the field type) — e.g. `Timeout: ptrTo(generousTimeout)`; this is mechanical but required to compile.
- `internal/commit/generate_test.go` ~line 476: `ai.NewTransport(r, ai.Config{AICommand: "claude -p"})` is an EXPLICIT command driving a retry-behaviour test — it does NOT assert the default. Leave the `"claude -p"` literal as-is UNLESS Task 2-2's `*time.Duration` field change forces a `Timeout` adjustment (it does not set Timeout, so a nil `Timeout` would need the no-deadline contract — set it to an explicit value if 2-2's contract requires non-nil; coordinate with 2-2's nil-handling decision). Make the minimal change to keep it compiling and asserting retry behaviour, not the default.
- Run the enumeration grep to catch any site missed: `grep -rn '"claude", "-p"' internal/ --include='*_test.go'` and `grep -rn 'claude -p' internal/ cmd/ --include='*_test.go'`. Triage each hit: DEFAULT-case argv assertion → migrate to `claude -p --model sonnet`; EXPLICIT-command test → leave; `Seed("claude", …)` → leave (binary-keyed).
- Confirm `cmd/mint` and any other package that pins the default argv is covered; if `cmd/mint` has no `claude -p` argv pin, note that and move on.

**Acceptance Criteria**:
- [ ] Every default-case exact-argv assertion of `claude -p` (no `--model`) is migrated to `claude -p --model sonnet`.
- [ ] FakeRunner `Seed("claude", …)` calls are unchanged (seeds match by binary name only; migrating them is unnecessary and out of scope).
- [ ] Explicit-command tests (e.g. `"mybot gen --json"`, the commit retry test's `ai.Config{AICommand: "claude -p"}`) are NOT spuriously changed — they assert a passed value, not the default.
- [ ] The transport's ex-default-command test (`TestTransport_Generate_DefaultCommandIsClaudeDashP`) is deleted or repointed to assert the passed command verbatim — it no longer asserts a transport-supplied default.
- [ ] All `ai.Config{...}` constructions in tests compile against Task 2-2's `*time.Duration` `Timeout` field.
- [ ] `go test -race ./...` is green across the whole repo; `go build ./...`, `gofmt -l .` (prints nothing), `go vet ./...`, `golangci-lint run` (0 issues) all pass.

**Tests**:
- This is the test-pin migration task; its "tests" are the migrated assertions themselves. Concretely the suite must now prove:
  - `"a zero-config release run invokes claude -p --model sonnet"` (migrated `TestRelease_AICommand_DefaultDrivesTransport`).
  - `"a configured ai_command run does not invoke the new default claude -p --model sonnet"` (migrated negative assertion in `TestRelease_AICommand_ConfigValueDrivesTransport`).
  - `"the transport runs the passed command verbatim"` (repointed transport test) — `claude` with args `["-p", "--model", "sonnet"]` when that exact command is passed.
- After migrating, run the grep enumeration once more and assert zero remaining DEFAULT-case `"claude", "-p"` (no `--model`) argv assertions.

**Edge Cases**:
- FakeRunner seeds keyed by binary name still match (per the task table) — only exact-argv assertions change; `Seed("claude", …)` is untouched because the binary name is still `claude`.
- The transport's ex-default-command test now asserts the passed command verbatim rather than a transport default (per the task table) — the default proof moved to the config layer.

**Context**:
> Spec — Migration & mechanical carry-overs (Test-pin migration): "Changing the shipped default and removing the transport's `defaultAICommand` will break every test that asserts the exact default command/argv (`claude -p` with no `--model`) … Project test idioms assert exact argv / rendered lines, so these are known, bounded edits — enumerate them in planning." (The initgen 'full template loads cleanly' test is Phase 3, NOT this task.)
> Spec — Pinned default model: "The shipped default command becomes `claude -p --model sonnet` (today: `claude -p`). … The only real migration cost is internal: mint's own test pins that assert the old `claude -p` default (enumerated in the Migration section)."
> Project test idioms (CLAUDE.md): "Assert exact argv on git invocations and exact rendered lines on presenter output — drift in either is a contract break." FakeRunner `Seed` is keyed by command NAME only; `SeedSequence` for same-binary sequences. `invokedWith`/`stdinOf` (engine tests) match the FULL joined command line.
> As-built sites (from grep): `release_configconsolidation_test.go` (~57, ~87), `release_priortag_test.go` (~133, ~520), `release_dryrun_test.go` (~189) carry `"claude", "-p"` exact-argv assertions for the default path; `transport_test.go` `TestTransport_Generate_DefaultCommandIsClaudeDashP` (~78) asserts the transport's default; `commit/generate_test.go` (~476) constructs an EXPLICIT `ai.Config{AICommand: "claude -p"}` for a retry test (not a default assertion). The many `f.Seed("claude", …)` lines across `release_priortag_test.go`/`release_realruncache_test.go`/etc. are binary-keyed seeds and stay.
> Coordination note: Tasks 2-3/2-4/2-5 each add or migrate the natural default-argv assertion for their own site; this task is the SWEEP that guarantees no orphaned `claude -p` (no `--model`) default pin remains anywhere and that the transport test is reconciled. If 2-1/2-3/2-4/2-5 already migrated a given assertion, this task verifies and de-duplicates rather than re-editing.

**Spec Reference**: `.workflows/ai-model-selection/specification/ai-model-selection/specification.md` — Migration & mechanical carry-overs (Test-pin migration); Pinned default model (the internal migration cost).
