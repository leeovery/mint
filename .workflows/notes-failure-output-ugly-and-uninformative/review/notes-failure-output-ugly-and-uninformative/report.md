# Implementation Review: Notes Failure Output Ugly and Uninformative

**Plan**: notes-failure-output-ugly-and-uninformative
**QA Verdict**: Approve

## Summary

The bugfix lands cleanly across all three facets the spec targeted. Fix 1 (the load-bearing change) introduces a typed `*ai.GenerationError` carrier that wraps `ErrGenerationFailed` and carries claude's captured `Stdout`/`Stderr`/`ExitCode` from the runner `Result`, populated after the single retry is exhausted on BOTH the non-zero-exit and empty/whitespace-body paths — and only then. Fix 2 collapses the top-line `Message` to one concise cause phrase via an exported `notes.CauseText` derivation matched by `errors.Is` (so it works through both the forward `abortError` chain and regenerate's shorter `"generating notes: %w"` chain) wired into `failureMessage`, leaving the `%w` chain intact for routing/logs. Fix 3 correctly leaves `padStage` untouched. The two notes `StageFailed` surfacing sites (`surfaceAndUnwind`, `surface`) now feed `notesFailureOutput(cause)` into `StageFailure.Output`, and the helper returns `""` for non-carrier causes so timeout / command-missing / diff-too-large render the concise phrase alone. All five load-bearing AI-seam invariants are preserved and pinned with explicit negative-carrier regression tests (including the critical `context.Canceled` passthrough). The two analysis cycles added end-to-end coverage through the real production transport chain, consolidated duplicated test helpers, documented an intentional cross-package twin, and made `GenerationError.Error()` honest about its dual provenance (never "(exit 0)"). All 10 tasks verified Complete with 0 blocking issues; every project gate passes.

## QA Verification

### Specification Compliance

Implementation aligns with the specification on every point:

- **Fix 1 (transport carrier)** — `*ai.GenerationError` mirrors the `*hooks.HookError` pattern, wraps the sentinel (`errors.Is` routing preserved), holds the two streams as distinct fields, and is populated only after the retry. Transport never imports `config` (Invariant 3); single-retry ownership unchanged (Invariant 4); success path byte-identical (Invariant 5).
- **Fix 2 (concise message)** — `notes.CauseText` exposes the four-sentinel mapping; `failureMessage` derives the concise phrase after the `*preflight.GateError` branch and before the `cause.Error()` defensive fallback. The display `Message` is separated from the still-intact matchable chain. No leading "notes" prefix, no "failed".
- **Fix 3 (layout)** — `padStage` and the `StageFailed` column layout untouched; `gate_forbidden_test.go` and `askline_test.go` confirmed byte-for-byte unchanged.
- **Per-cause Output behaviour** — only the generation-failed carrier populates `Output`; the other three causes render the concise phrase with empty `Output` (✗ line stands alone).
- **Scope discipline** — `resetAndAbort` inherits the concise `Message` for free but gets no `Output` population; the batch `--all` `reportSkip`/`classifyNotesFailure` path is left untouched, exactly as the spec's out-of-scope notes require.
- **`context.Canceled`** propagates unchanged with no carrier and no sentinel (Invariant 2) — pinned by an explicit negative-carrier test.

### Plan Completion
- [x] Phase 1 acceptance criteria met (carrier introduced, both bad-content paths populate it, invariants pinned)
- [x] Phase 2 acceptance criteria met (concise phrase, extraction helper, both surfacing paths wired)
- [x] Phase 3 acceptance criteria met (test-helper consolidation, end-to-end release test)
- [x] Phase 4 acceptance criteria met (cross-package twin documented, `Error()`/`ExitCode` honest)
- [x] All 10 tasks completed and individually verified
- [x] No scope creep — out-of-scope sites (`resetAndAbort`, batch skip path, `padStage`, presenter) left untouched

### Code Quality

No issues found. The carrier mirrors the established `*hooks.HookError` precedent; `notesFailureOutput` is placed beside `hookFailureOutput` with a WHY-comment explaining it reads stdout (not stderr) and uses `errors.As` precisely to match through the `%w` chain. The concise-phrase derivation follows the existing `failureMessage` → `gate.Message()` pattern. `GenerationError.Error()` is lowercase, no trailing punctuation, and now branches honestly on `ExitCode`.

### Test Quality

Tests adequately verify requirements. The transport tests now seed `FakeRunner` with stdout on a non-zero exit — the previously-untested shape that let the defect hide — and assert carrier capture, `errors.Is` matching, exactly-two-invocations, and the negative-carrier guards on all four non-carrier paths. The engine wiring test asserts `StageFailure.Output` population (not the rendered stream) across both forward and regenerate paths and all four causes. The Phase 3 end-to-end test drives the real production transport (Transport nil) through the genuine two-wrap chain — non-tautological. Duplicated helpers consolidated to one site; cross-package twin documented where consolidation is impossible. One minor, non-blocking coverage suggestion on the both-streams composition assertion (below).

### Required Changes (if any)

None.

## Recommendations

### Quick-fixes

1. `internal/engine/notesfailureoutput_internal_test.go:59` — `TestNotesFailureOutput_ComposesStdoutThenStderr` uses pre-trimmed literals, so the both-streams-present case proves the stdout-then-stderr join but does not assert interior-verbatim + single-trailing-trim *together*. Add a case with stdout `"out\n\nline\n"` and stderr `"err\n\n"` asserting `"out\n\nline\nerr"` to cover composition clause (d) directly rather than by inference. (Report 2-2)
