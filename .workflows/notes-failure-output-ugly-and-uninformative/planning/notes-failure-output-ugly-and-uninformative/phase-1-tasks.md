---
phase: 1
phase_name: Carry claude's captured output through the AI transport carrier
total: 3
---

## notes-failure-output-ugly-and-uninformative-1-1 | approved

### Task 1-1: Introduce the GenerationError carrier and populate it on a non-zero exit

**Problem**: `ai.Transport.attempt` returns `"", err` on a non-zero command exit, discarding the fully-populated runner `Result` (claude's actual message — e.g. `Prompt is too long` — on stdout). `ai.ErrGenerationFailed` is a bare sentinel with no payload, so when a non-zero exit survives the single retry there is nothing downstream can use to populate `StageFailure.Output`. The captured output never reaches the screen, which is the load-bearing defect this whole work unit fixes (Fix 1).

**Solution**: Introduce a typed carrier error `*ai.GenerationError` that wraps the `ErrGenerationFailed` sentinel (so `errors.Is` still matches) and carries claude's captured `Stdout` and `Stderr` as distinct fields (and `ExitCode`), mirroring how `*hooks.HookError` holds the failing entry's `runner.Result`. Stop discarding `res` on the non-zero-exit bad-content path so the captured output is available when `Generate` packs it into the carrier after the retry is exhausted. `Generate`'s signature is unchanged — the captured output travels on the returned error.

**Outcome**: A `Generate` call whose command exits non-zero on BOTH attempts returns an error that (a) still satisfies `errors.Is(err, ai.ErrGenerationFailed)` and (b) yields a `*ai.GenerationError` via `errors.As` carrying the runner's captured stdout and stderr as distinct fields. No sentinel routing or success-path behaviour changes.

**Do**:
- In `internal/ai/transport.go`, add a `GenerationError` struct carrying claude's captured output as distinct fields — `Stdout string`, `Stderr string`, `ExitCode int` — mirroring `*hooks.HookError` (which holds the whole `runner.Result`). Give it an `Error() string` method (a concise lowercase summary, no trailing punctuation, per the project error idioms) and an `Unwrap() error` method that returns `ErrGenerationFailed` so `errors.Is(err, ErrGenerationFailed)` matches the carrier. Keep `ErrGenerationFailed` as the existing bare sentinel — it is now the wrapped target, not removed.
- Change `attempt` (currently `internal/ai/transport.go` lines 191-204) so the bad-content path can surface the captured `res`. The minimal as-built change: on the `t.runner.RunWith` error branch (lines 200-202), return the populated `res` to the caller alongside the error rather than discarding it (e.g. return the `res` and `err`, or thread `res` so `Generate` can read `res.Stdout`/`res.Stderr`/`res.ExitCode` on the non-zero-exit path). The captured output is guaranteed present here by the runner contract — `internal/runner/runner.go` documents that on a non-zero exit `Result` is still fully populated alongside the non-nil error (`translateRun` builds `Stdout` before the `*exec.ExitError` branch).
- In `Generate` (lines 150-177), on the path where the retry's `attempt` returns a non-zero-exit bad-content error that survives `classifyFatal` returning nil, build and return `&GenerationError{Stdout: ..., Stderr: ..., ExitCode: ...}` from the retry attempt's captured `res` instead of the bare `return "", ErrGenerationFailed` at line 171. Do NOT change the timeout / missing-tool / cancel short-circuits routed by `classifyFatal` (lines 157-159, 167-169) — those return before the carrier path.
- Keep the carrier population AFTER the retry is exhausted only — the captured output from the SECOND (retry) attempt is what is packed (the retry is the last word). Do not populate a carrier on the first attempt's bad content (it falls through to the retry).
- Do not import `config` (the transport stays content-agnostic — Invariant 3). The carrier holds raw captured output, not notes/commit framing.

**Acceptance Criteria**:
- [ ] A `*ai.GenerationError` type exists in `internal/ai`, exported, carrying claude's captured `Stdout` and `Stderr` as distinct exported fields (plus `ExitCode`).
- [ ] `errors.Is(err, ai.ErrGenerationFailed)` returns true for the carrier (the carrier's `Unwrap` returns the sentinel) — sentinel routing (release `on_notes_failure`, commit editor fallback) is unaffected.
- [ ] `errors.As(err, &genErr)` retrieves the carrier from a `Generate` call whose command exits non-zero on both attempts, with `genErr.Stdout`/`genErr.Stderr` equal to the runner's captured streams and `genErr.ExitCode` equal to the captured exit code.
- [ ] Both stdout and stderr are carried when both are present on the non-zero exit (the carrier holds the two streams separately; composition into one rendered block is Phase 2's concern, not here).
- [ ] `errors.Is(err, ai.ErrTimeout)` and `errors.Is(err, ai.ErrCommandMissing)` are both false for the non-zero-exit carrier (it stays distinguishable from the other causes).
- [ ] The command is invoked exactly twice on the non-zero-exit-on-both-attempts case (single-retry ownership unchanged).
- [ ] `internal/ai` does not import `config`.
- [ ] All project gates pass: `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues).

**Tests** (in `internal/ai/transport_test.go`, package `ai_test`, `t.Parallel()`):
- `"it carries the runner's captured stdout and stderr on a non-zero exit surviving the retry"` — seed `FakeRunner` with `runner.Result{Stdout: "Prompt is too long", Stderr: "some stderr", ExitCode: 1}` and a non-nil error (e.g. `errors.New("exit status 1")`) on `claude`; call `Generate`; assert `errors.As(err, &genErr)` succeeds and `genErr.Stdout == "Prompt is too long"`, `genErr.Stderr == "some stderr"`, `genErr.ExitCode == 1`. (Seeding stdout on a non-zero exit is the case the prior fakes never exercised — they had no stdout to lose, which is why the defect never surfaced.)
- `"it still matches ErrGenerationFailed through the carrier wrap"` — same seed; assert `errors.Is(err, ai.ErrGenerationFailed)` is true and `errors.Is(err, ai.ErrTimeout)` / `errors.Is(err, ai.ErrCommandMissing)` are both false.
- `"it carries stdout-only when stderr is empty on a non-zero exit"` — seed `runner.Result{Stdout: "Prompt is too long", Stderr: "", ExitCode: 1}`; assert `genErr.Stdout == "Prompt is too long"` and `genErr.Stderr == ""` (the carrier holds both streams faithfully, including an empty one).
- `"it invokes the command exactly twice on a non-zero exit surviving the retry"` — same non-zero-exit seed; assert `len(r.Invocations()) == 2`.

**Edge Cases**:
- FakeRunner seeded with stdout on a non-zero exit — the previously-untested shape that exposes the defect.
- Both stdout and stderr present — the carrier must keep them distinct (Phase 2 composes them; Phase 1 must not pre-merge).
- `errors.Is(ErrGenerationFailed)` must still match through the wrap — the carrier's `Unwrap` returns the sentinel.

**Context**:
> The runner contract (`internal/runner/runner.go`): "On a non-zero exit the Result is still fully populated alongside a non-nil error, so callers can both branch on err and read Stderr/ExitCode." Synthesis confirmed `exec_runner.go`'s `translateRun` builds `Stdout` BEFORE the `*exec.ExitError` branch, so the captured output is guaranteed present (not best-effort) at the seam `attempt` currently discards it.
>
> Mirror `*hooks.HookError` (`internal/hooks/hooks.go` lines 27-41): a struct holding the captured `runner.Result`, an `Error()` method, and an `Unwrap()` returning the underlying error so `errors.Is` matches. The difference here: the carrier wraps the `ErrGenerationFailed` SENTINEL (so sentinel routing survives), and it holds `Stdout`/`Stderr`/`ExitCode` as distinct fields rather than the whole `Result` (the spec leaves either shape acceptable but requires the two streams be distinct fields — claude's payload is on stdout, and Phase 2 composes stdout-first-then-stderr).
>
> Option chosen (spec Fix 1): typed carrier error over a separate return value — keeps `Generate`'s signature and `errors.Is` routing intact, avoids churning every call site.
>
> Phase 1 is confined to `internal/ai`. Do NOT author the engine/notes extraction helper, the `errors.As` composition rule, or any presenter wiring — those are Phase 2.

**Spec Reference**: `.workflows/notes-failure-output-ugly-and-uninformative/specification/notes-failure-output-ugly-and-uninformative/specification.md` — "Fix 1 — Carry claude's captured output to `StageFailure.Output` (transport-level)", "Invariants to Preserve" (1, 3, 4), "Testing Requirements" (Transport bullet).

## notes-failure-output-ugly-and-uninformative-1-2 | approved

### Task 1-2: Populate the carrier on an empty/whitespace body surviving the retry

**Problem**: An empty or whitespace-only body is the OTHER bad-content path (alongside the non-zero exit covered in Task 1-1) that survives the single retry and becomes `ErrGenerationFailed`. The carrier must be populated here too: claude can write a real message on stdout AND still return a body that fails the `isValid` whitespace check (a zero-exit call whose stdout is whitespace-only, or whose informative text landed on stderr). If this path returns the bare sentinel, that captured output is again discarded.

**Solution**: On the empty/whitespace-survives-the-retry path in `Generate`, return the same `*ai.GenerationError` carrier populated from the retry attempt's captured `res` (its `Stdout`/`Stderr`/`ExitCode`), so a whitespace-only-body failure carries whatever claude actually wrote — exactly as the non-zero-exit path does. The single-retry ownership and exactly-two-invocations behaviour are unchanged.

**Outcome**: A `Generate` call whose body is empty or whitespace-only on BOTH attempts returns a `*ai.GenerationError` (still `errors.Is`-matching `ErrGenerationFailed`) carrying the retry attempt's captured streams, and the command is invoked exactly twice.

**Do**:
- In `Generate` (`internal/ai/transport.go`, lines 150-177), unify the two bad-content-survives-the-retry exits onto the carrier. The retry attempt has two non-fatal-failure shapes after `classifyFatal` returns nil:
  - the retry's `attempt` returned an error that is NOT a fatal cause (the non-zero-exit path from Task 1-1, line 171), and
  - the retry's `attempt` returned a clean body that fails `isValid` (the `!isValid(body)` branch, lines 173-175).
  Both must now return `&GenerationError{...}` populated from the retry attempt's captured `res` rather than the bare `return "", ErrGenerationFailed`. For the `!isValid` branch, `attempt` succeeded (nil error) but the body is empty/whitespace — thread the retry's captured `res` (its `Stdout`/`Stderr`/`ExitCode`) through so the carrier still reflects what claude wrote (e.g. a real message on stderr, or whitespace on stdout).
- Ensure `attempt` surfaces the captured `res` on its SUCCESS branch too (currently it returns only `res.Stdout` at line 203), so `Generate` can read `res.Stderr`/`res.ExitCode` when a zero-exit body is whitespace-only. Keep the change minimal and internal to the package; do not change `Generate`'s public signature.
- The carrier is populated ONLY after the retry is exhausted — a first-attempt whitespace body still falls through to the retry unchanged (lines 161-163 keep `isValid(body)` returning early only on a GOOD body).
- Do not alter the timeout / missing-tool / cancel short-circuits (`classifyFatal` paths) — those never reach the carrier.

**Acceptance Criteria**:
- [ ] A `Generate` call returning an empty body (`Stdout: ""`) on both attempts returns a `*ai.GenerationError` retrievable via `errors.As`, still `errors.Is`-matching `ErrGenerationFailed`.
- [ ] A `Generate` call returning a whitespace-only body (`Stdout: "   \n\t\n"`) on both attempts returns the carrier; the carrier's `Stdout` field holds that whitespace verbatim (the carrier preserves it; emptiness trimming/composition is Phase 2).
- [ ] When the body is whitespace-only on stdout but claude wrote a real message on stderr, the carrier's `Stderr` field holds that message (the informative output is still carried).
- [ ] The command is invoked exactly twice on the empty/whitespace-survives-the-retry case (single-retry ownership unchanged).
- [ ] The non-zero-exit carrier path (Task 1-1) and the empty/whitespace carrier path return the same `*ai.GenerationError` type — they are unified, not two divergent error shapes.
- [ ] All project gates pass: `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues).

**Tests** (in `internal/ai/transport_test.go`, package `ai_test`, `t.Parallel()`):
- `"it carries the captured output on an empty body surviving the retry"` — seed `FakeRunner` with `runner.Result{Stdout: ""}` and nil error on `claude`; assert `errors.Is(err, ai.ErrGenerationFailed)` and `errors.As(err, &genErr)` succeed; assert `genErr.Stdout == ""`.
- `"it carries the whitespace body verbatim on a whitespace-only body surviving the retry"` — seed `runner.Result{Stdout: "   \n\t\n"}` nil error; assert the carrier's `Stdout` holds `"   \n\t\n"` verbatim (no trimming at the transport — Phase 2 owns emptiness composition).
- `"it carries stderr when a whitespace-only stdout body hides a real stderr message"` — seed `runner.Result{Stdout: "   ", Stderr: "Prompt is too long"}` nil error; assert `genErr.Stderr == "Prompt is too long"` so the informative output is not lost.
- `"it invokes the command exactly twice on an empty body surviving the retry"` — seed `runner.Result{Stdout: ""}`; assert `len(r.Invocations()) == 2`.
- Consider extending the existing table-driven `TestTransport_Generate_RetriesOnceThenFailsOnBadContent` (lines 174-215) to additionally assert `errors.As(err, &genErr)` for its `empty body` / `whitespace-only body` / `non-zero exit` rows, OR add the cases above as dedicated tests — keep the existing `errors.Is(ErrGenerationFailed)` + exactly-two-invocations assertions green either way.

**Edge Cases**:
- Empty body (`Stdout: ""`) surviving the retry — carrier with empty stdout.
- Whitespace-only body surviving the retry — carrier holds the whitespace verbatim (the transport does not trim; emptiness composition is Phase 2).
- Captured stdout/stderr still carried when the body is whitespace-only — the informative message (often on stderr for a zero-exit refusal) is not discarded.
- Exactly two invocations — single-retry ownership unchanged (Invariant 4).

**Context**:
> `isValid` (`internal/ai/transport.go` lines 238-240) trims whitespace for the emptiness check: `strings.TrimSpace(body) != ""`. A whitespace-only body therefore fails validation and falls through to the retry, then to the carrier path. The carrier itself does NOT trim — it preserves the captured streams verbatim; Phase 2's settled composition rule (trim each stream for emptiness, stdout-first-then-stderr, trim the composed result) governs how the carrier's fields become the rendered `Output`.
>
> Invariant 4 (Single retry ownership unchanged): the transport still owns validation and the single bad-content retry; the carrier is populated ONLY after the retry is exhausted. Consumers never re-retry.
>
> This task builds directly on Task 1-1's carrier type — it is the same `*ai.GenerationError`, just reached via the empty/whitespace exit rather than the non-zero-exit exit. Land 1-1 first.

**Spec Reference**: `.workflows/notes-failure-output-ugly-and-uninformative/specification/notes-failure-output-ugly-and-uninformative/specification.md` — "Fix 1" (step 2: "`Generate` packs the captured output into the carrier when a non-zero exit / empty body survives the single retry"), "Invariants to Preserve" (4), "Fix 1" settled composition rule (whitespace handling, deferred to Phase 2).

## notes-failure-output-ugly-and-uninformative-1-3 | approved

### Task 1-3: Pin the load-bearing AI-seam invariants against the carrier change

**Problem**: The carrier upgrade rewrites the transport's bad-content failure path, which sits on load-bearing AI-seam contracts (CLAUDE.md non-negotiable seam 5). A regression here is silent and dangerous: if `context.Canceled` were accidentally wrapped into or swallowed by the carrier, a Ctrl-C would be misrouted to a fallback (commit's editor) or treated as an AI failure; if the timeout/missing-tool short-circuits picked up a carrier, the causes would stop being distinguishable; if a valid body were touched, byte-identical output would break. These must be pinned as explicit regression guards against the carrier change, not left to the pre-existing tests by assumption.

**Solution**: Add (and where appropriate strengthen) the regression tests that prove the four non-carrier paths are untouched by the carrier change — `context.Canceled` propagates UNCHANGED with no carrier and no sentinel; `ErrTimeout` carries no carrier; `ErrCommandMissing` carries no carrier; a valid body is returned verbatim with no carrier. These assert the invariants directly against `*ai.GenerationError` (via `errors.As` returning false on the non-carrier paths), complementing the existing sentinel/invocation-count tests rather than duplicating them.

**Outcome**: The test suite explicitly guarantees that only the bad-content-survives-the-retry path carries a `*ai.GenerationError`; cancel, timeout, missing-tool, and the success path are provably carrier-free and behave exactly as before, with `context.Canceled` still a verbatim passthrough.

**Do**:
- In `internal/ai/transport_test.go` (package `ai_test`, `t.Parallel()`), add a negative-carrier assertion to the cancel path. Either extend `TestTransport_Generate_DoesNotRetryCancel` (lines 294-317) or add a focused test: assert `errors.Is(err, context.Canceled)` is true, the three sentinels do NOT match (already asserted), AND `errors.As(err, &genErr)` returns false (no `*ai.GenerationError` was constructed — the cancel propagates UNCHANGED, never wrapped into the carrier). This is the most load-bearing guard: a cancel must never be routed to a fallback or swallowed by the carrier (Invariant 2, CLAUDE.md AI-seam contract).
- Add the same negative-carrier assertion to the timeout path (`TestTransport_Generate_DoesNotRetryTimeout`, lines 272-292): assert `errors.As(err, &genErr)` returns false alongside the existing `errors.Is(err, ai.ErrTimeout)` + exactly-one-invocation assertions — a missing/timed-out call has no captured output to carry (spec "Per-cause `Output` behaviour").
- Add the same negative-carrier assertion to the missing-tool path (`TestTransport_Generate_DoesNotRetryMissingTool`, lines 480-503): assert `errors.As(err, &genErr)` returns false alongside the existing `errors.Is(err, ai.ErrCommandMissing)` assertion.
- Add a negative-carrier assertion to the success path (`TestTransport_Generate_ReturnsValidBodyUnchanged`, lines 41-64): the existing test already asserts the body is returned verbatim with `err == nil`; confirm no behavioural change is needed beyond keeping it green (a nil error trivially yields no carrier). If a dedicated guard reads cleaner, add a one-liner asserting `err == nil` so the success path's carrier-free contract is explicit.
- Confirm the no-deadline cancel path test (`TestTransport_Generate_NoDeadlinePathPropagatesParentCancellationUnchanged`, lines 402-426) still holds after the carrier change — extend it with the same `errors.As(err, &genErr) == false` assertion so the parent-context cancel route is also pinned carrier-free.
- Do NOT modify the timeout/cancel/missing-tool production logic in `classifyFatal` (`internal/ai/transport.go` lines 210-229) — these tests pin that it stays unchanged. This task is test-only over the Task 1-1/1-2 production change.

**Acceptance Criteria**:
- [ ] `context.Canceled` propagates UNCHANGED from `Generate`: `errors.Is(err, context.Canceled)` is true, none of the three transport sentinels match, AND `errors.As(err, &genErr)` returns false (no carrier).
- [ ] The timeout path returns `ErrTimeout` with no `*ai.GenerationError` carrier (`errors.As` false) and exactly one invocation.
- [ ] The missing-tool path returns `ErrCommandMissing` with no `*ai.GenerationError` carrier (`errors.As` false) and exactly one invocation.
- [ ] A valid body is returned verbatim with `err == nil` and no carrier (the byte-identical success path is untouched — Invariant 5).
- [ ] The no-deadline (`Timeout: &0`) parent-context cancel path also propagates `context.Canceled` unchanged with no carrier.
- [ ] All project gates pass: `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues).

**Tests** (in `internal/ai/transport_test.go`, package `ai_test`, `t.Parallel()`):
- `"it propagates context.Canceled unchanged with no carrier and no sentinel"` — seed `FakeRunner` with `fmt.Errorf("running claude: %w", context.Canceled)`; assert `errors.Is(err, context.Canceled)`, all three sentinels false, `errors.As(err, &genErr) == false`, and exactly one invocation.
- `"it returns ErrTimeout with no carrier"` — seed `fmt.Errorf("running claude: %w", context.DeadlineExceeded)`; assert `errors.Is(err, ai.ErrTimeout)`, `errors.As(err, &genErr) == false`, exactly one invocation.
- `"it returns ErrCommandMissing with no carrier"` — `SeedNotFound("claude")`; assert `errors.Is(err, ai.ErrCommandMissing)`, `errors.As(err, &genErr) == false`, exactly one invocation.
- `"it returns a valid body verbatim with no carrier"` — seed a good `runner.Result{Stdout: body}`; assert the body is returned unchanged, `err == nil`, and (trivially) no carrier.
- `"it propagates a parent-context cancellation unchanged with no carrier on the no-deadline path"` — `Timeout: &0`, seed `context.Canceled`; assert `errors.Is(err, context.Canceled)`, `errors.As(err, &genErr) == false`.

**Edge Cases**:
- `context.Canceled` propagates unchanged with no carrier and no sentinel — the single most load-bearing invariant; a cancel is never an AI failure (Invariant 2).
- `ErrTimeout` carries no carrier — a hung call's partial output is not captured by this fix (spec "Per-cause `Output` behaviour").
- `ErrCommandMissing` carries no carrier — a missing binary has no output to carry.
- Valid body returned verbatim with no carrier — the success path is untouched (Invariant 5, byte-identical bodies).
- The no-deadline (`&0`) parent-context cancel route is pinned carrier-free too, not just the with-deadline route.

**Context**:
> CLAUDE.md non-negotiable seam 5: "`context.Canceled` propagates unchanged (a cancel is not an AI failure — never route it to a fallback)." Invariant 2 in the spec: "Any change to `attempt`/`Generate` that wraps the runner `Result` into a richer error MUST preserve `classifyFatal`'s unchanged `context.Canceled` propagation."
>
> Spec "Per-cause `Output` behaviour": only the generation-failed cause carries captured output. `ErrTimeout` and `ErrCommandMissing` short-circuit via `classifyFatal` and do not populate output — a missing binary has no output; a timed-out call's partial output is not captured by this fix.
>
> The existing tests (`TestTransport_Generate_DoesNotRetryCancel`, `...DoesNotRetryTimeout`, `...DoesNotRetryMissingTool`, `...ReturnsValidBodyUnchanged`, `...NoDeadlinePathPropagatesParentCancellationUnchanged`) already pin sentinel matching and invocation counts. This task ADDS the negative-carrier assertion (`errors.As(err, &genErr) == false`) to each, so the carrier change is proven not to leak onto these paths. It is test-only — no production change beyond what Tasks 1-1 and 1-2 introduce.

**Spec Reference**: `.workflows/notes-failure-output-ugly-and-uninformative/specification/notes-failure-output-ugly-and-uninformative/specification.md` — "Invariants to Preserve" (2 context.Canceled passthrough, 5 byte-identical success), "Fix 1" Per-cause `Output` behaviour, "Testing Requirements" (Transport bullet: "`context.Canceled` still propagates UNCHANGED — no carrier-swallowing").
