---
phase: 2
phase_name: Render verbatim claude output below a concise notes-failure line
total: 3
---

## notes-failure-output-ugly-and-uninformative-2-1 | approved

### Task 2-1: Expose the concise cause phrase and collapse failureMessage for notes/AI failures

**Problem**: The presenter-facing `Message` for a notes/AI failure is the entire nested `%w` chain — `notes generation failed (AI returned empty/invalid notes after retry): generating notes: ai generation failed`. Three layers each prepend their own text (`abortError` → `generate.go`'s `"generating notes: %w"` wrap → the `ai.ErrGenerationFailed` sentinel), and `failureMessage` in `internal/engine/release.go` faithfully renders the whole concatenation via `cause.Error()`. The line restates the stage ("notes" appears twice across the `padStage` gap) and contains "failed" twice. This is Fix 2.

**Solution**: Expose `notes.causeText`'s known-sentinel mapping as a package-public derivation, and extend the engine's single display-derivation helper `failureMessage(cause)` to use it — mirroring `failureMessage`'s existing `*preflight.GateError` → `gate.Message()` branch (a typed/known error exposing a display-ready phrase). The concise phrase is derived from the SENTINEL the cause wraps (matched via `errors.Is`, which traverses the `%w` chain), NOT by rendering the wrapped `cause.Error()`. The full `%w` chain is left intact for `errors.Is`/logs; only the display `Message` changes.

**Outcome**: For any of the four known notes-failure causes, on BOTH the forward release chain (`abortError(...)` wrapping) and regenerate's shorter `"generating notes: %w"` chain, `failureMessage(cause)` returns one concise phrase (e.g. `AI returned empty/invalid notes after retry`) that does not begin with the stage label, does not contain "failed", and contains no nested `%w` chain — while `errors.Is(cause, <sentinel>)` still matches the original error. `resetAndAbort` (its cause is a git record/push failure, not the AI carrier) and `surface`/`surfaceAndUnwind` all inherit the concise `Message` for free because they already funnel through `failureMessage`.

**Do**:
- In `internal/notes/resolve.go`, expose the known-sentinel mapping that `causeText` (~line 107) already performs as a package-public derivation usable from `internal/engine`. Two acceptable shapes (pick one, keep it minimal):
  - (a) Add an exported function `notes.CauseText(failure error) (string, bool)` that returns the concise phrase and `true` for one of the four known sentinels (`ai.ErrTimeout` → `"AI timed out"`, `ErrDiffTooLarge` → `"diff too large"`, `ai.ErrCommandMissing` → `"AI tool not installed"`, `ai.ErrGenerationFailed` → `"AI returned empty/invalid notes after retry"`), and `("", false)` for an unmapped cause. Have the unexported `causeText` (used by `abortError`) delegate to it, falling back to `failure.Error()` when the second return is `false` so `abortError`'s existing message is byte-identical.
  - (b) Alternatively expose it as a `Message()`-style method on the abort/carrier error. Prefer (a): it works uniformly across the forward chain AND regenerate's shorter chain (which never passes through `abortError`), because it matches on the wrapped sentinel via `errors.Is`, not on the error's concrete type.
- Keep the matching strictly `errors.Is`-based against the four sentinels so it traverses the `%w` chain and matches whether the sentinel is wrapped inside `abortError` (forward) or only inside `"generating notes: %w"` (regenerate fresh path, `internal/notes/generate.go` ~line 185).
- In `internal/engine/release.go`, extend `failureMessage` (~line 1615) with a new branch BEFORE the final `cause.Error()` fallback: call the new `notes.CauseText(cause)`; when it reports a known cause, return its concise phrase. Order it after the existing `*preflight.GateError` branch (a gate error is never one of the four AI sentinels, so order is non-conflicting, but keep the gate branch first to preserve its behaviour). Leave the final `return cause.Error()` as the defensive fallback for any unmapped cause (e.g. `resetAndAbort`'s git failure, which is not one of the four — it keeps rendering its own message).
- Do NOT alter `abortError`, the `%w` wrapping, or the unexported `causeText`'s observable output — `internal/notes/resolve_test.go`'s `TestResolveFailure_VariedCauses_RouteThroughBothModes` (~line 220) and the `errors.Is`/`errorContains` assertions there must stay green. Only ADD the exported derivation and have the existing one delegate.
- Do NOT touch the presenter or `padStage` (Fix 3 — no layout change).

**Acceptance Criteria**:
- [ ] `notes.CauseText` (or the chosen exported derivation) returns the exact concise phrase for each of the four sentinels and reports "not a known cause" for any other error, matching via `errors.Is` so it works through both `abortError`'s chain and the shorter `"generating notes: %w"` chain.
- [ ] `failureMessage(cause)` returns the concise phrase (no nested `%w` chain) for all four known causes on BOTH chain shapes; it does not begin with the stage label ("notes") as a prefix; it does not contain "failed". An incidental "notes" INSIDE the phrase (`AI returned empty/invalid notes after retry`) is allowed.
- [ ] `errors.Is(cause, <each sentinel>)` still matches the original wrapped error after the change — the `%w` chain is untouched.
- [ ] `resetAndAbort`'s git-failure cause (not one of the four) still renders via the `cause.Error()` defensive fallback — its concise behaviour is unchanged beyond already routing through `failureMessage`.
- [ ] `internal/notes/resolve_test.go`'s existing abort-message assertions stay green (`abortError`'s output is byte-identical).
- [ ] Gates pass: `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues).

**Tests**:
- `"it derives the concise phrase for each of the four sentinels via errors.Is"` — table over `ai.ErrGenerationFailed`, `ai.ErrTimeout`, `ai.ErrCommandMissing`, `notes.ErrDiffTooLarge`; assert the exact phrase and the known-cause flag (in `internal/notes`).
- `"it reports an unmapped cause as not-known so the engine falls back"` — pass a plain `errors.New("boom")`; assert the not-known result (and, via `failureMessage`, that `cause.Error()` is returned).
- `"failureMessage collapses the forward abortError chain to the concise phrase"` — wrap a sentinel through `notes.ResolveFailure(..., abort)` (the forward chain) and assert `failureMessage` returns the concise phrase with no nested chain, no leading "notes", no "failed" (in `internal/engine`).
- `"failureMessage collapses regenerate's shorter generating-notes chain to the same phrase"` — wrap a sentinel as `fmt.Errorf("generating notes: %w", ai.ErrGenerationFailed)` and assert `failureMessage` returns the identical concise phrase (regenerate's chain is NOT routed through `abortError`).
- `"failureMessage leaves the matchable chain intact"` — after deriving the concise `Message`, assert `errors.Is(cause, sentinel)` still holds.
- `"failureMessage falls back to cause.Error for a non-AI git cause"` — pass `resetAndAbort`'s shape (a wrapped git error, not one of the four) and assert the fallback text, proving the defensive default still works.

**Edge Cases**:
- Forward `abortError` chain vs regenerate's shorter `"generating notes:"` chain — both must yield the identical concise phrase because the derivation matches on the wrapped sentinel, not the chain shape.
- All four sentinels covered; the `default`/unmapped branch retained only as a defensive fallback (not a reachable display path per the spec's unmapped-cause contract).
- No leading "notes notes" prefix; no "failed"; incidental "notes" inside the phrase is acceptable.
- `resetAndAbort`'s git cause must NOT be misclassified as a known AI cause — it falls through to `cause.Error()`.

**Context**:
> Fix 2 (spec "Fix 2 — Collapse the top-line message to one concise cause phrase"): the seam is `failureMessage(cause)` in `internal/engine/release.go`, through which `surface`, `surfaceAndUnwind`, and `resetAndAbort` already funnel their `Message`. It is extended to derive the concise phrase — "the same mapping `causeText` provides; the engine accesses it via an exported derivation (an exported `causeText` equivalent, or a `Message()`-style method) rather than rendering the wrapped `cause.Error()`." This is option (a) — change the engine display helper — chosen over stripping `abortError`'s prefix (which would change the matchable/logged text) and over a brand-new display-only error type. The `%w` chain is RETAINED for `errors.Is`/logs (spec "Sub-decision"). The four sentinels are the exhaustive set that can reach the notes display (spec "Unmapped-cause contract"); the `default` branch is a defensive fallback, not relied on for the concise-`Message` guarantee. Regenerate's fresh path "may carry a shorter wrap chain than forward release (it surfaces `GenerateFromRange`'s `"generating notes: %w"` directly rather than always re-wrapping through `abortError`/`causeText`)" — the derivation must produce a clean phrase for both (spec "Note on regenerate's wrap chain").

As-built anchors:
- `internal/notes/resolve.go`: `causeText` (~line 107) maps the four sentinels; `abortError` (~line 100) wraps with `notes generation failed (%s): %w`.
- `internal/engine/release.go`: `failureMessage` (~line 1615) currently `errors.As`-branches on `*preflight.GateError` → `gate.Message()`, else `cause.Error()`.
- `internal/notes/generate.go` (~line 185): the fresh-path wrap is `fmt.Errorf("generating notes: %w", err)`.

**Spec Reference**: `.workflows/notes-failure-output-ugly-and-uninformative/specification/notes-failure-output-ugly-and-uninformative/specification.md` (Fix 2; Unmapped-cause contract; Note on regenerate's wrap chain; Acceptance Criteria #2)

## notes-failure-output-ugly-and-uninformative-2-2 | approved

### Task 2-2: Add the carrier-output extraction helper composing stdout-then-stderr

**Problem**: The Phase 1 `*ai.GenerationError` carrier now holds claude's captured `Stdout`/`Stderr` from the runner `Result`, but nothing in the engine reads it. The existing `hookFailureOutput` helper (`internal/engine/release.go` ~line 1591) is the right PATTERN but the wrong FIELD: it reads `Result.Stderr`, whereas claude's payload (e.g. `Prompt is too long`) is on STDOUT, so a literal copy would render nothing. A new extraction helper is needed that traverses the `%w` chain to find the carrier and composes both streams by the settled rule.

**Solution**: Add a new engine helper (e.g. `notesFailureOutput(cause error) string`) in `internal/engine/release.go`, mirroring `hookFailureOutput`'s shape but using `errors.As(cause, &genErr)` against `*ai.GenerationError` — which traverses the `%w` chain, so it still matches when the carrier is wrapped inside `abortError` (forward) or the shorter regenerate chain. It composes the captured streams stdout-first then stderr by the settled rule, returning the empty string for any non-carrier cause (the timeout/command-missing/diff-too-large cases) so those render the concise phrase alone.

**Outcome**: `notesFailureOutput(cause)` returns the composed captured output for a generation-failed carrier wherever it sits in the `%w` chain, and the empty string for every other cause — ready for Task 2-3 to feed into `StageFailure.Output`.

**Do**:
- In `internal/engine/release.go`, add `notesFailureOutput(cause error) string` directly beside `hookFailureOutput` (~line 1591) so the parallel pattern is obvious to a future reader; carry a WHY-comment stating it reads STDOUT (not stderr like the hook helper) because claude's message is on stdout, and that `errors.As` is used precisely so it matches the carrier inside `abortError`/the regenerate chain.
- Match the carrier via `var genErr *ai.GenerationError; if !errors.As(cause, &genErr) { return "" }`. (Use the exact carrier type name and `Stdout`/`Stderr` field names introduced in Phase 1 — confirm against `internal/ai`.)
- Compose by the SETTLED rule:
  1. Trim leading/trailing whitespace from each stream for the EMPTINESS check (`strings.TrimSpace`); a whitespace-only stream counts as EMPTY.
  2. Include the non-empty streams in order: stdout first, then stderr. When both are non-empty, join the two ORIGINAL (verbatim, untrimmed-internal) stream values with a single `"\n"`. When only one is non-empty, include just that stream.
  3. Internal content is preserved VERBATIM — do not trim or reflow the kept stream's interior; only the emptiness DECISION uses the trimmed view.
  4. Trim TRAILING whitespace from the composed result (`strings.TrimRight(out, " \t\r\n")` or `strings.TrimRightFunc(out, unicode.IsSpace)`) so there is no dangling blank line — the presenter's `writeNotesBody` re-adds exactly one trailing newline.
  5. When BOTH streams are empty after trimming, return `""` — the ✗ line stands alone.
- Decide carefully whether the joined/kept value preserves internal verbatim content while only the result's trailing whitespace is trimmed: e.g. keep `genErr.Stdout` and `genErr.Stderr` verbatim for inclusion, decide inclusion using their trimmed copies, join included streams with `"\n"`, then trim only the trailing whitespace of the final composed string.
- Do NOT change `hookFailureOutput`, the presenter, or `padStage`.

**Acceptance Criteria**:
- [ ] `notesFailureOutput` returns `""` for a cause that does not wrap `*ai.GenerationError` (timeout, command-missing, diff-too-large, or any plain error).
- [ ] It returns the carrier's captured output when the carrier is wrapped inside `abortError`'s chain AND when wrapped inside the shorter `"generating notes: %w"` chain (proven via `errors.As` traversal).
- [ ] stdout-only carrier → stdout verbatim (trailing whitespace trimmed); stderr-only carrier → stderr verbatim (trailing whitespace trimmed).
- [ ] Both streams non-empty → stdout, then a single newline, then stderr; internal content of each stream verbatim; only the composed result's trailing whitespace trimmed.
- [ ] Whitespace-only stdout AND whitespace-only stderr → `""` (✗ line stands alone).
- [ ] One whitespace-only stream and one real stream → only the real stream, with no leading/joining newline from the empty stream.
- [ ] Gates pass: `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues).

**Tests**:
- `"it extracts stdout when the carrier is wrapped in abortError"` — build `notes.ResolveFailure`'s abort error around a carrier with stdout `"Prompt is too long\n"`; assert `notesFailureOutput` returns `"Prompt is too long"` (trailing newline trimmed).
- `"it extracts through the shorter regenerate chain"` — wrap the carrier as `fmt.Errorf("generating notes: %w", carrier)`; assert the same stdout extraction (proves `errors.As` traversal independent of chain shape).
- `"it composes stdout then stderr joined by a single newline when both present"` — carrier with stdout `"out line"` and stderr `"err line"`; assert `"out line\nerr line"`.
- `"it includes only stdout when stderr is whitespace-only"` — stdout real, stderr `"   \n"`; assert just the stdout value, no joining newline.
- `"it includes only stderr when stdout is whitespace-only"` — stdout `"\t\n"`, stderr real; assert just the stderr value.
- `"it returns empty when both streams are whitespace-only"` — both whitespace; assert `""`.
- `"it preserves internal content verbatim while trimming trailing whitespace"` — stdout `"line1\n\nline2\n\n"`; assert `"line1\n\nline2"` (interior blank line kept, trailing whitespace trimmed).
- `"it returns empty for a non-carrier cause"` — pass `ai.ErrTimeout`, `ai.ErrCommandMissing`, `notes.ErrDiffTooLarge`, and a plain error; assert `""` for each.

**Edge Cases**:
- `errors.As` must match through `abortError`'s `%w` chain AND the shorter regenerate chain — assert both.
- stdout-only / stderr-only / both-present compositions.
- Whitespace-only streams treated as empty (the emptiness check uses the trimmed view; the kept content stays verbatim).
- Internal/interior whitespace preserved verbatim; only the composed result's TRAILING whitespace trimmed.
- Non-carrier causes (the three other sentinels and any plain error) yield `""`.

**Context**:
> Fix 1 consumption (spec "Fix 1 — Carry claude's captured output", step 3): "The engine mirrors the `hookFailureOutput` pattern (a typed-error extraction helper) — but not its field choice. `hookFailureOutput` reads `Result.Stderr`; claude's payload here is on stdout, so a literal copy would render nothing. The new helper uses `errors.As(cause, &genErr)` — which traverses the `%w` chain, so it still matches when the carrier is wrapped inside `abortError`'s chain on the forward path — and composes the captured `Output` by this settled rule:
> - Trim leading/trailing whitespace from each stream for the emptiness check; a whitespace-only stream counts as empty.
> - Include the non-empty streams stdout first, then stderr, joined by a single newline. Both are shown when both are present (most informative); stdout leads because claude's message (e.g. `Prompt is too long`) is on stdout.
> - Trim trailing whitespace from the composed result before assignment, so there is no dangling blank line; internal content is preserved verbatim (the presenter's `writeNotesBody` re-adds exactly one trailing newline).
> - When both streams are empty after trimming, `Output` is empty and the ✗ line stands alone."

The Phase 1 carrier `*ai.GenerationError` wraps `ai.ErrGenerationFailed` and holds the captured `Stdout`/`Stderr` (and optionally `ExitCode`) from the runner `Result`. Precedents this mirrors: `internal/engine/release.go` `hookFailureOutput` (the analogous extraction for `*hooks.HookError`, ~line 1591), `internal/commit/surface.go` `surfaceOutput`. Only the generation-failed cause carries output; timeout/command-missing/diff-too-large have no claude output (spec "Per-cause `Output` behaviour").

**Spec Reference**: `.workflows/notes-failure-output-ugly-and-uninformative/specification/notes-failure-output-ugly-and-uninformative/specification.md` (Fix 1 step 3; Per-cause `Output` behaviour; Acceptance Criteria #1)

## notes-failure-output-ugly-and-uninformative-2-3 | approved

### Task 2-3: Wire the composed Output into both notes StageFailed surfacing paths

**Problem**: `surfaceAndUnwind` (forward release, `internal/engine/release.go` ~line 1047) and `surface` (regenerate's single-version/interactive notes path, ~line 1604) both build `presenter.StageFailure{Name, Message}` with NO `Output` today, so even with the carrier populated (Phase 1) and the extraction helper available (Task 2-2), claude's captured output never reaches the screen below the ✗ line. The two paths must behave identically (Acceptance Criteria #3), and only the generation-failed cause should carry output.

**Solution**: Feed the Task 2-2 helper's result into `StageFailure.Output` at BOTH surfacing sites. Because the helper returns `""` for any non-carrier cause, timeout/command-missing/diff-too-large automatically render the concise phrase with an empty `Output` (✗ line stands alone), and only generation-failed carries the composed body. No presenter or `padStage` change (Fix 3).

**Outcome**: A notes AI generation-failure renders the concise top-line `Message` (from Task 2-1) followed by claude's verbatim captured output below the ✗ line, identically on the forward release (`surfaceAndUnwind`) and regenerate (`surface`) paths; the other three causes render the concise phrase alone; `padStage` and the non-message presenter tests are untouched.

**Do**:
- In `internal/engine/release.go`, set `Output: notesFailureOutput(cause)` on the `presenter.StageFailure{}` built in `surfaceAndUnwind` (~line 1048) and in `surface` (~line 1605), alongside the existing `Name` and `Message: failureMessage(cause)`.
- Note that `surfaceAndUnwind` serves multiple stages (`pre_tag`, `notes`, `record`, `preflight`, `tag` — see the call sites at `release.go` ~lines 438/458/514/...); calling `notesFailureOutput(cause)` for all of them is correct because the helper returns `""` for any non-carrier cause, so only the notes/AI generation-failure populates `Output`. Do NOT special-case the stage name.
- Do NOT change `resetAndAbort` (`internal/engine/regenerate_write.go` ~line 353): its cause is a git record/push failure, never the AI carrier, so it is OUT OF SCOPE for the `Output`-population change — it already inherits the concise `Message` via `failureMessage` (Task 2-1). Leave it building `StageFailure{Name, Message}` with no `Output`. (If `notesFailureOutput` were added there it would always return `""`, so adding it is harmless but unnecessary — leave it untouched to keep the diff scoped.)
- Do NOT touch the batch `--all` skip path — `reportSkip` / `classifyNotesFailure` in `internal/engine/regenerate_batch.go` (it narrates a non-terminal `Warn`, a different UX, explicitly out of scope).
- Do NOT change the presenter (`StageFailed`/`writeNotesBody` in `internal/presenter/pretty.go` already render `Output` verbatim below the ✗ line when non-empty) or `padStage` (Fix 3).
- Update ONLY the `pretty_test.go` failure-line assertion(s) whose message text changes under Fix 2: `TestPrettyPresenterFailedRegenerateSuppressesClose` (`internal/presenter/pretty_test.go` ~line 618) uses `Message: "claude failed"`. The "failed" message text is now a fabricated test literal, not a real notes message; either keep it as an opaque literal (it is not asserting the concise-message rule) or update it to a concise phrase — but ensure no `pretty_test.go` assertion claims the OLD nested-chain message. Leave `TestPrettyPresenterStageFailedRendersCapturedOutputBelowGlyphLine` (~line 1119, the `tag/push` case) untouched — the new engine/notes wiring test COMPLEMENTS it, not duplicates it.
- Keep `gate_forbidden_test.go` and `askline_test.go` UNTOUCHED (they pin `failNotInteractive`'s `padStage(label)`, guaranteed safe by keeping `padStage`).

**Acceptance Criteria**:
- [ ] `surfaceAndUnwind` and `surface` both set `StageFailure.Output = notesFailureOutput(cause)`; a generation-failed carrier populates `Output` with the composed captured output at BOTH sites.
- [ ] The two paths render identically for the same cause (forward release `surfaceAndUnwind` vs regenerate `surface`).
- [ ] timeout / command-missing / diff-too-large produce an empty `StageFailure.Output` (the helper returns `""`) — the ✗ line stands alone with the concise phrase.
- [ ] `resetAndAbort` is unchanged (no `Output` population); the batch `reportSkip`/`classifyNotesFailure` path is unchanged.
- [ ] `padStage` and the `StageFailed` column layout are unchanged; only `pretty_test.go` message-text assertions change; `gate_forbidden_test.go` and `askline_test.go` stay byte-for-byte untouched.
- [ ] The wiring test asserts `StageFailure.Output` POPULATION (via `presentertest.RecordingPresenter`), not the rendered stream — the existing pinned presenter test already covers stream placement.
- [ ] Gates pass: `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues).

**Tests**:
- `"forward release notes failure populates StageFailure.Output with claude's captured stdout"` — drive the release spine (or `surfaceAndUnwind` directly with a `presentertest.RecordingPresenter`) with a generation-failed carrier (carrier wrapped via `abortError`, stdout `"Prompt is too long"`); assert the recorded `StageFailure.Output` contains `"Prompt is too long"` and the `Message` is the concise phrase (no nested chain, no "failed", no leading "notes").
- `"regenerate notes failure populates StageFailure.Output identically"` — drive the `surface(p, "notes", err)` path (regenerate's shorter chain) with the same carrier; assert the recorded `StageFailure.Output` and `Message` match the forward path's exactly (proves both paths behave identically, Acceptance Criteria #3).
- `"timeout renders the concise phrase with empty Output"` — surface `ai.ErrTimeout` (forward and regenerate); assert `Message == "AI timed out"` and `Output == ""`.
- `"command-missing renders the concise phrase with empty Output"` — surface `ai.ErrCommandMissing`; assert `Message == "AI tool not installed"` and `Output == ""`.
- `"diff-too-large renders the concise phrase with empty Output"` — surface `notes.ErrDiffTooLarge`; assert `Message == "diff too large"` and `Output == ""`.
- `"resetAndAbort still surfaces a concise Message with no Output"` — drive `resetAndAbort` with a git push failure; assert `Message` is the concise/`cause.Error()` fallback and `Output == ""` (proving it inherited Fix 2 but was not given Fix 1 output).
- Update `TestPrettyPresenterFailedRegenerateSuppressesClose` if its message literal needs to stop implying the old nested-chain text; assert the `padStage` column is unchanged.

**Edge Cases**:
- Forward release vs regenerate must render IDENTICALLY for the same cause — assert both `Message` and `Output` match across the two paths.
- Only generation-failed carries `Output`; the other three causes (timeout, command-missing, diff-too-large) yield empty `Output` (✗ line stands alone).
- `surfaceAndUnwind` is shared by multiple stages — calling `notesFailureOutput` for all is safe (returns `""` for non-carrier causes); do not special-case stage name.
- `resetAndAbort` and the batch `reportSkip` path remain untouched.
- `padStage` gap unchanged; `gate_forbidden_test.go` / `askline_test.go` untouched.

**Context**:
> Scope & Affected Surfaces (spec): "The `Output`-population change covers the two notes `StageFailed` surfacing helpers — `surfaceAndUnwind(ctx, deps, "notes", …)` (forward release) and `surface(p, "notes", err)` (regenerate single-version/interactive). Both build `presenter.StageFailure{Name, Message}` with no `Output` today. The engine helper that extracts the captured output feeds `StageFailure.Output` at both sites." `resetAndAbort` (third `StageFailure{}` builder) "is out of scope for the `Output`-population change — its `cause` is a git record/push failure, never the AI carrier — but it inherits the concise `Message` for free." The batch `--all` `reportSkip`/`classifyNotesFailure` path (fourth display site) is "out of scope" (non-terminal `Warn`, different UX).
>
> Fix 3 (spec): "keep the gap. No change to `padStage` or the `StageFailed` column layout." Test impact: "the only `pretty_test.go` failure-line assertions that change are those affected by Fix 2's concise-message text. `gate_forbidden_test.go` and `askline_test.go` stay untouched."
>
> Testing Requirements (spec): "Engine/notes wiring — assert the notes AI-failure path populates `StageFailure.Output` with claude's captured output. This is the gap the existing `tag/push`-only presenter test left uncovered; assert at the wiring level, not just the presenter." "Both surfacing paths — cover forward release (`surfaceAndUnwind`) and regenerate (`surface`)." "Stream split (as-built — do not assert otherwise): the captured body (`StageFailure.Output`) renders to stdout only … The new engine/notes wiring test asserts `StageFailure.Output` population, not the rendered stream."

As-built anchors:
- `internal/engine/release.go`: `surfaceAndUnwind` (~line 1047) and `surface` (~line 1604) build `presenter.StageFailure{Name: stage, Message: failureMessage(cause)}`; notes stage call site at ~line 458.
- `internal/presenter/pretty.go`: `StageFailed` (~line 536) renders `s.Output` via `writeNotesBody` when non-empty — NO change needed.
- `internal/presenter/pretty_test.go`: `TestPrettyPresenterFailedRegenerateSuppressesClose` (~line 618, message literal), `TestPrettyPresenterStageFailedRendersCapturedOutputBelowGlyphLine` (~line 1119, the `tag/push` case to leave untouched).
- Test doubles: `presentertest.RecordingPresenter` for the wiring assertion; `runner.FakeRunner` for any spine drive.

**Spec Reference**: `.workflows/notes-failure-output-ugly-and-uninformative/specification/notes-failure-output-ugly-and-uninformative/specification.md` (Scope & Affected Surfaces; Fix 3; Acceptance Criteria #1, #3, #4; Testing Requirements)
