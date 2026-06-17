# Investigation: Notes-Failure Output Ugly and Uninformative

## Symptoms

### Problem Description

**Expected behavior:**
When AI notes generation fails during `mint release`, the operator should see
claude's actual captured output verbatim (e.g. `Prompt is too long`) under the
✗ line, via a populated `StageFailure.Output`. The top-line message should
collapse to one concise sentence that does not restate the stage name or repeat
"failed" three times. The failure-line layout (whether to keep the `padStage`
gap for failures) should be settled, with the pinned presenter tests updated to
match.

**Actual behavior:**
The failure renders as a single line:

```
✗ notes      notes generation failed (AI returned empty/invalid notes after retry): generating notes: ai generation failed
```

- The message is redundantly self-nesting: "notes generation **failed** … :
  generating notes: ai generation **failed**" — "generation failed" twice and
  "generating notes" once across one line (the seed's "three times" is loose;
  literally "failed" appears twice — corrected here so the acceptance criterion
  "does not repeat 'failed' / does not restate the stage" is testable against the
  actual string).
- The real AI cause (claude's stdout/stderr) is discarded.
- `StageFailure.Output` ends up empty, so there is nothing actionable below
  the ✗ line.
- The message restates the stage name — "notes" appears twice across a wide
  `padStage` gap.

### Manifestation

- Failure line is visually ugly and uninformative.
- Operator is left guessing at the true cause (in the surfacing case,
  `claude -p --model sonnet` exited non-zero with stdout `Prompt is too long`,
  which never reached the screen).

### Reproduction Steps

1. Run `mint release` in a repo where the notes diff is large enough to exceed
   the model context window (the surfacing case: `v0.0.2..HEAD` diff ~867 KB,
   mostly committed `.workflows/` and `.tick/` artifact trees).
2. AI notes generation invokes `claude -p` which exits non-zero with
   `Prompt is too long` on stdout.
3. Observe the rendered failure line.

**Reproducibility:** Always (given an AI command that exits non-zero / returns
empty output).

### Environment

- **Affected environments:** Local CLI (`mint release`).
- **Browser/platform:** Terminal (pretty renderer).
- **User conditions:** Any notes-generation failure path — non-zero exit,
  empty/invalid AI body after retry.

### Impact

- **Severity:** Medium (no data loss; degraded diagnosability — operator can't
  see why notes failed).
- **Scope:** Any operator who hits a notes-generation failure.
- **Business impact:** Trust / usability — the tool looks broken and gives no
  actionable signal.

### References

- Seed: `seeds/2026-06-14-notes-failure-output-ugly-and-uninformative.md`
  (inbox:bug, captured while dogfooding `mint release` on mint itself).
- Discovery session: `discovery/session-001.md`.

### Out of Scope (carried from discovery)

The line-based `max_diff_lines` guard (default 50000) did not catch an
8.6k-line but ~867 KB byte-dense diff, suggesting a byte/token-aware ceiling.
This was **deliberately left out of scope** for this work unit — it's an
enhancement to a guard, not part of how a failure is rendered. Noted in the
seed; can be promoted to its own work later.

---

## Analysis

### Initial Hypotheses

Three facets of one failure-rendering path (from the seed / discovery):

1. **The real cause is discarded.** The transport
   (`internal/ai/transport.go`) maps a non-zero exit or empty body to
   `ErrGenerationFailed` and drops claude's captured stdout/stderr; the notes
   layer (`internal/notes/resolve.go:101`, `internal/notes/generate.go:185`)
   re-wraps it into a redundant chain, and `StageFailure.Output` ends up empty.
2. **The top-line message is redundant** — "failed" appears three times and
   restates the stage name.
3. **The failure line is visually ugly** — `padStage` alignment in
   `internal/presenter/pretty.go` (`StageFailed`) puts "notes" twice across a
   wide gap. The gap is intentional column alignment shared with `✓` success
   lines and is pinned in presenter tests (`pretty_test.go`,
   `gate_forbidden_test.go`, `askline_test.go`), so changing it is a contract
   change to be settled deliberately.

### Code Trace

**Entry point (what the operator sees):** the notes stage fails and the release
spine renders a `StageFailed` line via `surfaceAndUnwind(ctx, deps, "notes", …)`
(`internal/engine/release.go:458`).

**Execution path — the error's construction, bottom-up:**

1. `internal/ai/transport.go:191` `attempt()` — runs `t.runner.RunWith(...)`.
   On a non-zero exit (or any runner error) the runner returns a **fully
   populated** `Result` (`res.Stdout` = claude's `Prompt is too long`,
   `res.Stderr`, `res.ExitCode`) alongside a non-nil `err` — this is the runner's
   documented contract (`internal/runner/runner.go:23-31, 40-43`). But `attempt`
   does `if err != nil { return "", err }` — it **discards `res` entirely**,
   throwing away claude's stdout/stderr. ← **PRIMARY ROOT CAUSE (facet 1).**
2. `internal/ai/transport.go:150` `Generate()` — on bad content after the single
   retry returns the bare sentinel `ErrGenerationFailed` ("ai generation
   failed"). The sentinel carries **no payload** — there is no field for the
   captured output even if `attempt` had kept it.
3. `internal/notes/generate.go:183-186` `generateFromDiffWithContext` — wraps:
   `fmt.Errorf("generating notes: %w", err)` → "generating notes: ai generation
   failed".
4. `internal/notes/select.go:204-205` → `internal/notes/resolve.go:65`
   `ResolveFailure` (abort mode) → `abortError` (`resolve.go:100-102`):
   `fmt.Errorf("notes generation failed (%s): %w", causeText(failure), failure)`,
   where `causeText` (`resolve.go:107-120`) maps `ErrGenerationFailed` →
   "AI returned empty/invalid notes after retry". Final `.Error()` string:
   `notes generation failed (AI returned empty/invalid notes after retry): generating notes: ai generation failed`.
   ← **ROOT CAUSE (facet 2): redundant nested chain.**
5. `internal/engine/release.go:1047-1053` `surfaceAndUnwind` builds
   `presenter.StageFailure{Name: "notes", Message: failureMessage(cause)}` —
   `failureMessage` (`release.go:1615-1621`) returns `cause.Error()` (the full
   nested chain). **`Output` is never set → empty.**
6. `internal/presenter/pretty.go:536-546` `StageFailed` renders
   `✗ <padStage(name)><message>` and, **only when `s.Output != ""`**, writes the
   captured body verbatim below via `writeNotesBody`. Output is empty, so the ✗
   line stands alone. `padStage` (`pretty.go:1123-1128`) right-pads "notes" to
   `stageColumn=11`, so the message — which itself starts with "notes" — sits
   after a wide gap: `✗ notes      notes generation failed …`.
   ← **ROOT CAUSE (facet 3): padStage gap + stage-restating message.**

**Key files involved:**
- `internal/ai/transport.go` — discards the runner `Result` on error; bare
  `ErrGenerationFailed` sentinel with no payload. (Shared by `mint commit` too.)
- `internal/notes/generate.go`, `internal/notes/resolve.go` — `%w` wrapping
  layers stack "generating"/"failed" prefixes; `abortError`/`causeText` produce
  the redundant top-line.
- `internal/engine/release.go` — `surfaceAndUnwind`/`surface` set only
  `Name`+`Message`; `failureMessage` surfaces the whole nested `.Error()`.
- `internal/presenter/pretty.go` — `StageFailed` (already renders `Output` when
  present) + `padStage` (the shared alignment gap).

**Existing in-codebase precedent the fix should mirror (the machinery already
exists — only the notes/AI path doesn't use it):**
- `internal/presenter/pretty.go:536-546` already renders a verbatim captured
  body below the ✗ line when `StageFailure.Output` is non-empty — pinned green by
  `TestPrettyPresenterStageFailedRendersCapturedOutputBelowGlyphLine`
  (`pretty_test.go:1119`, using the `tag/push` case).
- `internal/commit/surface.go:26-33` `surfaceOutput` — passes a failed command's
  captured stderr verbatim as `StageFailure.Output`.
- `internal/commit/run.go:948-958` `pushAfterCommit` — git's stderr travels
  verbatim in `Warning.Output`.
- `internal/engine/release.go:1559,1587-1597` `hookFailureOutput` — extracts a
  `*hooks.HookError`'s captured `Result.Stderr` into `Output`. The notes/AI
  failure needs an analogous typed carrier OR `ErrGenerationFailed` upgraded to
  carry the captured output.

**padStage contract (facet 3 detail):** `padStage` is shared by
`StageSucceeded` (`pretty.go:495`), `StageFailed` (`540`), `Unwound` (`565`), and
`failNotInteractive` (`953`). The pinned tests are: `pretty_test.go` (success +
failure lines), `gate_forbidden_test.go` and `askline_test.go` (the latter two
pin `failNotInteractive`'s `padStage(label)`, NOT the notes stage). So dropping
the gap for `StageFailed` *only* touches `pretty_test.go`; changing `padStage`
itself touches all four. This is the deliberate contract decision flagged in the
seed — settle in spec, update the pinned tests to match.

### Root Cause

The notes-failure render is uninformative and ugly because of three independent
defects on one path, each fixable on its own:

1. **The real cause is discarded at the transport.** `ai.Transport.attempt`
   returns `"", err` on a non-zero exit, dropping the fully-populated runner
   `Result` (claude's `Prompt is too long` on stdout). `ErrGenerationFailed` is a
   bare sentinel with no payload, so nothing downstream can populate
   `StageFailure.Output` — even though `StageFailed` already knows how to render
   it. **This is the load-bearing root cause** — fixing it is what lets the
   operator see the actual message.
2. **The top-line message is a redundant `%w` chain.** Three layers each prepend
   their own "generating"/"generation failed" text
   (`abortError` → `generate.go` wrap → `ErrGenerationFailed`), and the presenter
   shows the entire nested `cause.Error()`. The message should collapse to one
   concise phrase (e.g. `causeText`'s clause alone) that does not restate the
   stage or repeat "failed".
3. **The failure line restates the stage across the `padStage` gap.** `StageFailed`
   pads "notes" to the stage column, then the message *also* leads with "notes",
   so it reads `✗ notes      notes …`. Fixing facet 2 (a message that no longer
   restates the stage) removes most of the visible ugliness; whether to also drop
   the `padStage` gap for failures is a separate, deliberate layout decision.

**Why this happens:** the transport was designed content-agnostic and returns
*typed, distinguishable* sentinels so callers can branch on cause — but it
optimised for "which kind of failure" and never carried "what the tool actually
said". The notes layer then wraps for context (good Go hygiene) without anyone
owning the operator-facing collapse, and the presenter faithfully prints whatever
`.Error()` it's handed.

### Contributing Factors

- The runner *does* preserve stdout/stderr/exit on a non-zero exit (good), but
  the transport's error path throws the `Result` away — the one seam where the
  real cause is available is also where it's dropped.
- `%w` wrapping is correct for `errors.Is` matching but produces a human-hostile
  concatenation when the whole chain is rendered as the display message.
- `StageFailure.Output` and the presenter's verbatim-body rendering already
  exist and are battle-tested (tag/push, hooks, commit) — the notes path simply
  never opted in.

### Why It Wasn't Caught

- The failure-rendering machinery (`StageFailure.Output` → `writeNotesBody`) is
  tested with the `tag/push` case, so the *presenter* contract looks covered —
  but no test asserts the **notes AI-failure** path populates `Output` or that
  the top-line message is concise. The gap is at the engine/notes wiring, not the
  presenter.
- AI-failure rendering is only hit on a genuine `claude` non-zero exit / empty
  body, which fakes in tests script as a bare sentinel — so the discarded-output
  defect never surfaced in the suite (the fakes never had stdout to lose).
- It took real-world dogfeeding (an 867 KB diff → `Prompt is too long`) to expose
  that the actionable message was missing.

### Blast Radius

**Directly affected:**
- `mint release` notes stage failure rendering (the reported bug).

**Potentially affected (shared transport):**
- `mint commit` consumes the SAME `ai.Transport`, so it has the identical
  discard-claude's-output defect at the transport. Its editor-fallback softens
  the symptom, but a transport-level fix improves both verbs. Scope decision for
  the spec: fix the transport once (benefits both) vs. notes-only.
- `mint release regenerate` rides `[release]` and the same generator/transport,
  so it shares facets 1 and 2.
- Any change to `padStage` (facet 3, if chosen) touches every aligned line
  (success, failure, unwound, gate-not-interactive) and their pinned tests.

---

## Fix Direction

_To be filled during Step 8 (Findings Review & Fix Discussion)._

---

## Notes

Investigation initialized from discovery carrier (manifest description + session-001 log + seed).

### Synthesis validation (2026-06-17)

Independent synthesis agent traced the code fresh: **all six root-cause claims
verified to the implementation level**, high confidence, no alternative root
cause. It confirmed the runner provably populates `res.Stdout` with claude's
message on a non-zero exit (`exec_runner.go` `translateRun` builds Stdout before
the `*exec.ExitError` branch and returns the populated `res`) and that `attempt`
provably discards it. Full report:
`.workflows/.cache/notes-failure-output-ugly-and-uninformative/investigation/notes-failure-output-ugly-and-uninformative/synthesis-001.md`.

Two minor gaps raised — both now resolved:

1. **"failed" over-count** — symptom prose said "three times"; literally "failed"
   appears twice in the chain. Corrected in Symptoms above so the acceptance
   criterion is testable against the real string.
2. **Regenerate surfacing untraced** — now traced. `mint release regenerate`
   surfaces notes-generation failures via `surface(p, "notes", err)`
   (`regenerate_batch.go:271`, `regenerate_interactive.go:207`), which builds the
   SAME `StageFailure{Name, Message: failureMessage(cause)}` with **no `Output`**.
   So regenerate shares facets 1 (discarded claude output) and 2 (wrapped
   message), and hits the padStage gap (facet 3) for its "notes" stage too —
   though its fresh path may carry a SHORTER wrap chain (surfaces `GenerateFromRange`'s
   `"generating notes: %w"` directly rather than always re-wrapping through
   `abortError`/`causeText`). **Fix scope note:** the fix must cover BOTH
   surfacing helpers — `surfaceAndUnwind` (forward notes stage) AND `surface`
   (regenerate notes stage, and the generic pre-PONR path) — so regenerate's
   rendering is not left behind. A transport-level fix (carry claude's output on
   the failure) benefits all three verbs (release, regenerate, commit) at once.

### Fix-direction constraints surfaced by validation (for the spec, not decisions)

- **`context.Canceled` must stay a passthrough.** Any change to `attempt`/`Generate`
  that wraps the runner `Result` into a richer error MUST preserve
  `classifyFatal`'s unchanged `context.Canceled` propagation (CLAUDE.md AI-seam
  contract — a cancel is not an AI failure, never routed to a fallback).
- **`padStage` is shared by four call sites.** Dropping the gap for `StageFailed`
  only touches `pretty_test.go`; editing `padStage` itself also shifts
  `StageSucceeded`/`Unwound`/`failNotInteractive` and breaks the exact-line
  contracts pinned in `gate_forbidden_test.go` and `askline_test.go`. Settle the
  layout decision deliberately and update whichever pinned tests it touches.
