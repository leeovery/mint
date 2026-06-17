# Specification: Notes Failure Output Ugly and Uninformative

## Specification

## Problem Statement

When AI notes generation fails during `mint release` (and `mint release regenerate`), the rendered failure is both **uninformative** and **ugly**:

```
✗ notes      notes generation failed (AI returned empty/invalid notes after retry): generating notes: ai generation failed
```

Three independent defects converge on this one line:

1. **The real cause is discarded.** `claude`'s actual captured output (e.g. `Prompt is too long` on stdout from a non-zero exit) never reaches the screen. `StageFailure.Output` ends up empty, so there is nothing actionable below the ✗ line.
2. **The top-line message is a redundant `%w` chain.** "generation failed" appears twice and "generating notes" once across one line, and the message restates the stage name ("notes" appears twice).
3. **The failure line restates the stage across the `padStage` gap** — `✗ notes      notes …`.

## Target Behaviour

The failure should render claude's verbatim captured output below the ✗ line, with a single concise top-line message that does not restate the stage name or repeat "failed":

```
✗ notes  AI returned empty/invalid notes after retry
  Prompt is too long
```

- The top-line `Message` is one concise cause phrase.
- `claude`'s captured stdout/stderr is shown verbatim in the `Output` block below.
- The failure-line column layout (`padStage` gap) is settled deliberately, with pinned presenter tests updated to match.

## Fix 1 — Carry claude's captured output to `StageFailure.Output` (transport-level)

This is the **load-bearing fix** — it is what lets the operator see the actual message. The other two facets are polish that ride on it.

**Root cause:** `ai.Transport.attempt` returns `"", err` on a non-zero exit, discarding the fully-populated runner `Result` (claude's `Prompt is too long` on stdout). `ai.ErrGenerationFailed` is a bare sentinel with no payload, so nothing downstream can populate `StageFailure.Output` — even though the presenter already knows how to render it.

**Precondition (runner contract):** the fix rests on the runner's *documented* guarantee that on a non-zero exit the `Result` is **still fully populated** (`Stdout`/`Stderr`/`ExitCode`) alongside the non-nil error (`internal/runner/runner.go`). Synthesis validation confirmed `exec_runner.go`'s `translateRun` builds `Stdout` **before** the `*exec.ExitError` branch and returns the populated `res`. The captured output is therefore guaranteed present (not best-effort) at the one seam where it is currently discarded — so `transport.attempt` "stops discarding `res`" has something real to capture.

**Change:**

1. **Upgrade `ai.ErrGenerationFailed` into a typed carrier error** (e.g. `*ai.GenerationError`) that:
   - **wraps** the sentinel, so `errors.Is(err, ErrGenerationFailed)` still matches (callers that branch on the three sentinels are unaffected); and
   - **carries** claude's captured output from the runner `Result`. It holds the captured `Stdout` and `Stderr` as distinct fields (it may also keep `ExitCode`) — mirroring how `*hooks.HookError` holds the whole `runner.Result`. The rendered `Output` is composed **stdout-first** (claude's `Prompt is too long` is emitted on **stdout**, not stderr — see step 3).
2. **`transport.attempt` stops discarding `res`** on the bad-content error path; **`Generate` packs the captured output** into the carrier when a non-zero exit / empty body survives the single retry. The `Generate` signature is unchanged — the captured output travels on the returned error, not via a new return value.
3. **The engine mirrors the `hookFailureOutput` *pattern*** (a typed-error extraction helper) — but **not its field choice**. `hookFailureOutput` reads `Result.Stderr`; claude's payload here is on **stdout**, so a literal copy would render nothing. The new helper uses `errors.As(cause, &genErr)` — which traverses the `%w` chain, so it still matches when the carrier is wrapped inside `abortError`'s chain on the forward path — and reads the carrier's stdout (including stderr when stdout is empty). The notes surfacing sites set `StageFailure.Output` to the extracted output (exact site list in Scope & Affected Surfaces).
4. **No presenter change is needed for this facet** — `StageFailed` (`pretty.go`) already renders a verbatim captured body below the ✗ line via `writeNotesBody` when `StageFailure.Output != ""`.

**Per-cause `Output` behaviour:** only the **generation-failed** cause (a non-zero exit / empty body that survived the retry) carries captured output. The other `causeText` causes render the concise phrase with an **empty `Output`** — the ✗ line stands alone, per the presenter contract:

- `ai.ErrTimeout` ("AI timed out") and `ai.ErrCommandMissing` ("AI tool not installed") short-circuit via `classifyFatal` and are not bad-content failures; they do not populate `Output` (a missing binary has no output; a timed-out call's partial output is not captured by this fix).
- `notes.ErrDiffTooLarge` ("diff too large") originates in the `CheckDiffSize` guard and never reaches the transport, so it has no claude output and renders the concise phrase alone.

**Precedents this fix mirrors (opt-in to existing mechanism, not new mechanism):**

- `internal/presenter/pretty.go` `StageFailed` already renders a verbatim captured body below the ✗ line, pinned green by `TestPrettyPresenterStageFailedRendersCapturedOutputBelowGlyphLine` (using the `tag/push` case). The new engine/notes test **complements** this existing test (which the notes path simply never opted into) — it does not duplicate it.
- `internal/commit/surface.go` `surfaceOutput` — passes a failed command's captured stderr verbatim as `StageFailure.Output`.
- `internal/commit/run.go` `pushAfterCommit` — git's stderr travels verbatim in `Warning.Output`.
- `internal/engine/release.go` `hookFailureOutput` — extracts a typed carrier error's captured `Result.Stderr` into `Output`; the analogous extraction helper for the AI carrier mirrors this.

**Why transport-level:** the discard is at the transport, which is shared by all three verbs, so fixing it once is the natural home. Release and regenerate gain the rendered `Output`; commit gains a carrier that does not break its `errors.Is`-based editor-fallback routing (its AI failure never reaches `StageFailed`, so it gains no rendered output — see Scope & Affected Surfaces). The transport stays content-agnostic (never imports `config`).

**Option chosen:** typed carrier error (mirrors `*hooks.HookError`, keeps `Generate`'s signature and `errors.Is` routing intact) **over** returning the captured output as a separate return value (which would churn every call site).

## Fix 2 — Collapse the top-line message to one concise cause phrase

**Root cause:** the presenter-facing `Message` is the entire nested `%w` chain. Three layers each prepend their own text (`abortError` → `generate.go` wrap → `ErrGenerationFailed`), and the presenter faithfully renders the whole concatenation as the display string.

**Change:** the presenter-facing `Message` shows only the short cause phrase that `causeText` already produces (e.g. `ErrGenerationFailed` → "AI returned empty/invalid notes after retry"). The message must **not** begin with or duplicate the stage label as a leading prefix (no `notes  notes …`) and must **not** contain "failed". An incidental "notes" *inside* the descriptive cause phrase is acceptable — "AI returned empty/invalid notes after retry" is valid even though it contains the word "notes". The verbose detail (claude's captured output) lives in the Fix-1 `Output` block, not in the top line.

Target render for the reported case:

```
✗ notes  AI returned empty/invalid notes after retry
  Prompt is too long
```

**Seam (settled here):** the concise phrase is produced by the engine's single display-derivation helper, `failureMessage(cause)` in `internal/engine/release.go`, through which all `StageFailed` sites already funnel their `Message` (`surface`, `surfaceAndUnwind`, and `resetAndAbort`). `failureMessage` is extended to derive the concise phrase for notes/AI failures — mirroring its existing `*preflight.GateError` → `gate.Message()` branch (a typed error exposing a display-ready message). The phrase is the same mapping `causeText` provides; the engine accesses it via an exported derivation (an exported `causeText` equivalent, or a `Message()`-style method on the carrier / abort error) rather than rendering the wrapped `cause.Error()`. Centralising on `failureMessage` covers the forward chain (`abortError` → `causeText`), regenerate's shorter chain, AND `resetAndAbort` in one place. This is **option (a)** — change the engine display helper — chosen over (b) stripping `abortError`'s prefix (which would change the `errors.Is`-matchable / logged text) and (c) a brand-new display-only error type (largest change).

**Unmapped-cause contract:** the four sentinels — `ai.ErrGenerationFailed`, `ai.ErrTimeout`, `ai.ErrCommandMissing`, `notes.ErrDiffTooLarge` — are the **exhaustive** set of typed failures that can reach the notes surfacing display. `ResolveFailure` is invoked solely when the generator surfaces one of these typed failures, and `context.Canceled` propagates separately (it is never routed here as an AI failure). The concise phrase therefore always resolves via one of the four. `causeText`'s `default` branch (which returns the cause's own `failure.Error()`) is retained only as a **defensive** fallback to keep an unexpected cause informative — it is not a reachable display path in practice, so it is not relied on for the concise-`Message` guarantee. Accordingly, Acceptance Criteria #2's "no nested chain / no 'failed'" guarantee is asserted against the four known causes (the realistic failure universe), which is what the concise-message test exercises.

**Sub-decision (settled here):** the `%w` wrapping chain (`abortError`/`generate.go` wrap) is **retained** for `errors.Is` matching and logs — it is correct Go hygiene and load-bearing for sentinel routing. What changes is that the **display `Message` is separated from the matchable error**: the surfacing path derives the concise display phrase rather than rendering the full nested `cause.Error()`. We do **not** tear out the error chain.

**Option chosen:** concise phrase from `causeText` (CHOSEN) **over** continuing to render the full nested chain. The nested chain is correct for `errors.Is`/logs but human-hostile as a display string; separating the matchable error from the display message is the fix.

## Fix 3 — Keep the `padStage` gap for failure lines

**Decision: keep the gap. No change to `padStage` or the `StageFailed` column layout.**

**Rationale:** once Fix 2 lands, the "notes … notes" duplication is gone regardless of the gap, which removes the only functional driver to change the layout. Keeping the gap is column-consistent with the `✓` (success) and `↩` (unwound) lines and is the lowest-risk option.

**Why not drop the gap:** `padStage` is shared by four call sites — `StageSucceeded`, `StageFailed`, `Unwound`, and `failNotInteractive`. Dropping the gap for `StageFailed` alone would touch only `pretty_test.go`, but editing `padStage` itself would ripple to the other three aligned lines and break the exact-line contracts pinned in `gate_forbidden_test.go` and `askline_test.go`. With no remaining functional driver after Fix 2, that contract churn is unwarranted.

**Test impact:** the only `pretty_test.go` failure-line assertions that change are those affected by Fix 2's concise-message text. `gate_forbidden_test.go` and `askline_test.go` stay untouched (they pin `failNotInteractive`'s `padStage(label)`, not the notes stage).

**Note on the worked-example spacing:** the `✗ notes  …` examples throughout this spec use illustrative two-space spacing; the real failure line uses `padStage("notes")` column padding (the gap preserved per this decision). The updated `pretty_test.go` assertion replaces only the message text *after* the padded stage column — the padding itself is unchanged.

**Option chosen:** keep the gap (CHOSEN) **over** dropping the `padStage` gap for failures.

## Scope & Affected Surfaces

**Fix the transport once; the shared seam serves all three verbs.** `mint release`, `mint release regenerate`, and `mint commit` all consume the same `ai.Transport`, which has the identical discard-claude's-output defect, so the carrier upgrade (Fix 1) lands once. **The user-visible improvement (verbatim claude output below a ✗ line) accrues to release and single-version/interactive regenerate only.** A `mint commit` AI failure routes to the `$EDITOR` fallback (`isAIFallback` / `runEditorFallback` in `internal/commit/run.go`), never to `StageFailed`, so it has no `Output` render site. For commit the carrier is still correct and necessary: it MUST preserve `errors.Is(err, ErrGenerationFailed)` so the fallback still triggers (see Invariants). What commit gains is a non-broken seam, not new rendered output — no commit-side rendering or tests are in scope.

**The `Output`-population change covers the two notes `StageFailed` surfacing helpers** so regenerate's rendering is not left behind:

- `surfaceAndUnwind(ctx, deps, "notes", …)` — the **forward release** notes stage (`internal/engine/release.go`).
- `surface(p, "notes", err)` — the regenerate **`StageFailed`** path: single-version/interactive regenerate's notes-production failure (`regenerate_interactive.go`) and the generic pre-PONR path (including batch's deterministic pre-read body *read* failure at `regenerate_batch.go`). The batch `--all` per-version notes-**production** failure does NOT use this path — see the fourth-site note below.

Both build `presenter.StageFailure{Name, Message}` with no `Output` today. The engine helper that extracts the captured output feeds `StageFailure.Output` at **both** sites.

**Third `StageFailed` site — `resetAndAbort` (`internal/engine/regenerate_write.go`):** it also builds `presenter.StageFailure{Name, Message: failureMessage(cause)}` directly, for the changelog-regenerate record/push failure path. It is **out of scope for the `Output`-population change** — its `cause` is a git record/push failure, never the AI carrier — but it **inherits the concise `Message` for free** because it routes through the same `failureMessage` helper (Fix 2). It is called out here so an implementer auditing the spec against the code finds the third `StageFailure{}` builder accounted for, not unaddressed.

**Fourth display site — batch `--all` regenerate per-version skip (`reportSkip` / `internal/engine/regenerate_batch.go`): out of scope.** Batch regenerate deliberately does NOT abort on a per-version notes-production failure — it catches it (the skip-and-continue contract), narrates a **non-terminal `presenter.Warn`** via `reportSkip(p, tag, classifyNotesFailure(err))`, records a `skippedVersion` for the closing batch summary, and moves to the next version. This is a different UX from the terminal `StageFailed` ✗-line render this spec targets, so it is **out of scope**: `classifyNotesFailure` / `reportSkip` are not touched. Two deliberate residual limitations follow, each promotable to its own work unit if desired:

- The batch skip reason keeps its own vocabulary — `classifyNotesFailure` returns `"notes generation failed"` (which contains "failed") — independent of Fix 2's concise-`Message` rule, which governs the `StageFailed` display only.
- The batch per-version `Warn` carries no captured `Output`, so a `--all` run that trips a real AI error still shows a one-line skip without claude's output. (Routing the carrier's captured output into `Warning.Output` here — the same render site `commit/run.go pushAfterCommit` uses — is the natural future enhancement.)

**Note on regenerate's wrap chain:** regenerate's fresh path may carry a *shorter* wrap chain than forward release (it surfaces `GenerateFromRange`'s `"generating notes: %w"` directly rather than always re-wrapping through `abortError`/`causeText`). The concise-`Message` derivation (Fix 2) must therefore produce a clean phrase for both the forward and regenerate chains — not assume the forward-release chain shape.

## Invariants to Preserve

The carrier upgrade (Fix 1) touches the AI seam, which carries load-bearing contracts. The change MUST preserve all of them:

1. **`errors.Is(err, ErrGenerationFailed)` still matches.** The new carrier error wraps the sentinel; callers that branch on the three sentinels (`ErrGenerationFailed` / `ErrTimeout` / `ErrCommandMissing`) are unaffected.
2. **`context.Canceled` stays a passthrough.** Any change to `attempt`/`Generate` that wraps the runner `Result` into a richer error MUST preserve `classifyFatal`'s unchanged `context.Canceled` propagation — a cancel is not an AI failure and must never be routed to a fallback or swallowed by the carrier (CLAUDE.md AI-seam contract).
3. **The transport stays content-agnostic.** It continues never to import `config`; the carrier holds raw captured output, not notes/commit-specific framing.
4. **Single retry ownership is unchanged.** The transport still owns validation and the single bad-content retry; consumers never re-retry. The carrier is populated only after the retry is exhausted.
5. **Byte-identical bodies on success.** The success path is untouched — generated notes/commit bodies still pass through verbatim.

## Acceptance Criteria

A notes-generation AI failure (non-zero exit or empty/invalid body after retry) renders as:

```
✗ notes  AI returned empty/invalid notes after retry
  Prompt is too long
```

1. **`StageFailure.Output` is populated** with claude's captured output (stdout-first) verbatim and rendered below the ✗ line.
2. **The top-line `Message` is the concise cause phrase** — it does not contain the nested `%w` chain, does not begin with the stage label as a prefix (no `notes notes …`), and does not contain "failed". (An incidental "notes" inside the cause phrase is allowed; the concise-message test asserts absence of the leading-label duplication and of "failed", not absence of the substring "notes".)
3. **Both `StageFailed` surfacing paths behave identically** — forward release (`surfaceAndUnwind`) and single-version/interactive regenerate (`surface`). The batch `--all` per-version skip `Warn` is out of scope (see Scope & Affected Surfaces).
4. **The `padStage` gap is unchanged** for all aligned lines.

## Testing Requirements

**New tests:**

- **Engine/notes wiring** — assert the notes AI-failure path populates `StageFailure.Output` with claude's captured output. This is the gap the existing `tag/push`-only presenter test left uncovered; assert at the wiring level, not just the presenter.
- **Concise message** — assert the top-line `Message` is the concise cause phrase and does NOT contain the nested chain and does NOT restate the stage name.
- **Both surfacing paths** — cover forward release (`surfaceAndUnwind`) and regenerate (`surface`) so regenerate's rendering is not left behind.
- **Transport** — a non-zero-exit `Generate` carries the runner's captured stdout/stderr on the returned error, while `errors.Is(err, ErrGenerationFailed)` still holds; and `context.Canceled` still propagates UNCHANGED (no carrier-swallowing). Tests must seed `FakeRunner` with stdout on a non-zero exit (the prior fakes had no stdout to lose, which is why the defect never surfaced).

**Updated tests:**

- Update the `pretty_test.go` failure-line assertions that the concise-`Message` change touches. Keep `gate_forbidden_test.go` and `askline_test.go` untouched (guaranteed by keeping `padStage`).

**Stream split (as-built — do not assert otherwise):** the ✗ one-line summary is written to BOTH stdout and stderr; the captured body (`StageFailure.Output`) renders to **stdout only** (never duplicated to stderr). The new engine/notes wiring test asserts `StageFailure.Output` **population**, not the rendered stream; the existing pinned presenter test (`TestPrettyPresenterStageFailedRendersCapturedOutputBelowGlyphLine`) already covers stream placement.

All changes pass the project gates: `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues).

## Out of Scope

- **Byte/token-aware diff ceiling.** The line-based `max_diff_lines` guard (default 50000) did not catch an 8.6k-line but ~867 KB byte-dense diff, which suggests a byte/token-aware ceiling would be valuable. This is **deliberately out of scope** — it is an enhancement to an input guard, not part of how a failure is *rendered*. It can be promoted to its own work unit later. This specification covers only the failure-rendering path: carrying the captured output, the concise message, and the layout decision.
- **Batch `--all` regenerate per-version skip rendering.** The non-terminal `Warn` path (`reportSkip` / `classifyNotesFailure`) is deliberately not touched — see "Fourth display site" under Scope & Affected Surfaces for the rationale and the two promotable residual limitations (the "failed" skip-reason vocabulary and the absent `Warning.Output`).

## Risk & Rollout

- **Fix complexity: Low.** Mirrors the existing `StageFailure.Output` / `hookFailureOutput` precedent; no new presenter mechanism is introduced.
- **Regression risk: Low–Medium.** Low given the Fix 3 decision to keep `padStage`; it would only rise to Medium if `padStage` were edited globally (which this spec does not do). The carrier must preserve `errors.Is(ErrGenerationFailed)` matching and the `context.Canceled` passthrough — both load-bearing AI-seam invariants (see Invariants to Preserve).
- **Rollout: regular release, not a hotfix.** The bug degrades diagnosability but causes no data loss.

---

## Working Notes
