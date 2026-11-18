# Implementation Review: CLI Presentation

**Plan**: mint-release-tool
**QA Verdict**: Approve

## Summary

All 42 tasks across the 9 plan phases are fully implemented, correctly tested, and faithful to the specification. The `internal/presenter` package delivers the spec's `Presenter` seam exactly as designed: an event/step-oriented interface where the engine emits semantic events and two implementations (`pretty`, `plain`) decide how they look, selected once at startup by TTY detection with no environment sniffing. Every load-bearing invariant is honoured and guarded by behaviour-level tests — the event-payload principle (no presenter re-derives engine knowledge or hardcodes stage logic), byte-identical notes body across modes, the three orthogonal axes (styling / gating / stream-split), the forbidden-combination fail-loud path, render-only `Prompt` with engine-owned `e`/`r` re-entry, and the success-only verb-shaped end-of-run line. The four analysis cycles (phases 5–9) landed genuine DRY/consistency improvements (shared test helpers, single construction seam, functional-options constructor) without behavioural drift. Where the implementation departs from the spec's worked examples (e.g. plain's stage-agnostic `running...` start verb, flush-left notes rules), each deviation is deliberate, spec-sanctioned, and documented at the source. No blocking issues were found; all recommendations are non-blocking.

## QA Verification

### Specification Compliance

Implementation aligns with the specification throughout. Verified against every major spec section: render-mode detection (TTY-only, `--plain` force-plain, no `--pretty`/`--no-color`, lipgloss colour auto-downgrade), the `Presenter` seam and event-payload principle, gating & `-y` orthogonality (including the forbidden-combination rule and the render-only `Prompt` contract), the pretty and plain layers (brand lines, glyphs, no-box notes rule, vertical gate menu, terse `key: value` plain lines), spinner lifecycle (pretty-only, one-at-a-time, buffered output below `✗`), library selection (lipgloss + standalone spinner, no Bubble Tea), and cross-verb rendering (`init`/`regenerate`/`version` payload exception + verb-shaped footer). The two engine-owed reconciliations the spec records (the `y`/`n`/`e`/`r` gate superseding the stale `[a]`/`[q]` keys; the `--plain` global flag) are correctly reflected on the presentation side. Documented deviations from illustrative worked-example wording are within the spec's stated "wording refinable" latitude and forced by fixed constraints (byte-purity, event-payload principle).

### Plan Completion
- [x] All 9 phase acceptance criteria met (Presenter seam & render-mode skeleton; run narration; interactive gating; cross-verb rendering, spinner & width; analysis cycles 1–5)
- [x] All 42 tasks completed and individually verified
- [x] No scope creep — later-phase additions to shared files (e.g. the growing `Presenter` interface, gate `Subject`/`AcceptEcho` fields) were anticipated by their own task contexts and reviewed against the correct task, not counted against earlier ones

### Code Quality

No issues found. The package consistently applies the project's Go conventions: compile-time interface assertions, functional-options constructors with injectable seams (writer/reader/profile/spinner-factory/term-width), typed `Choice`/`RunVerb`/`InitAction` enums, centralised fire-and-forget `writef`/`errf` write-error discard, and thorough intent-revealing doc comments. The dependency-inversion seam is clean (engine depends on the presenter interface; colour-capability is delegated to lipgloss rather than reimplemented). Complexity is uniformly low. The one structural observation — a 14-method `Presenter` interface exceeding the usual 1–3 method guidance — is an intentional, spec-mandated single lifecycle seam ("consistency is structural, one interface") and splitting it would fight the spec.

### Test Quality

Tests adequately verify requirements. Coverage is behaviour-focused (asserting rendered bytes, stream placement, returned choices, recorded events — not internals), balanced (every acceptance criterion has a direct assertion; no significant over-testing), and uses deterministic seams (forced colour profiles, spy spinners, `/dev/null` handles, scripted readers) so it is CI-safe without real TTYs. Standout guards include the byte-identical notes-body cross-mode comparison, the import-scan dependency guards (no UI library in plain; no subprocess in the prompt path), the screen-control guard that passes under live SGR escapes, the golden full-worked-example transcripts pinning composition, and the `failingReader` proofs that stdin is never read under `-y`/forbidden-combination. The non-blocking test notes below are additive strengthening, not gaps that undermine the verified contracts.

### Required Changes (if any)

None.

## Recommendations

### Do now
1. `internal/presenter/bytepurity_test.go:21` — reword the byte-purity failure messages if exact-message parity with the pre-refactor per-arm strings is desired (cosmetic, failure-path only) (Report 5-1)
2. `internal/presenter/plain_test.go:269` — reword `TestPlainPresenterStageFailedRendersOneLineSummary`'s stale doc comment (it still calls the delimiter block + stderr duplication "later phases"; both are now implemented) to point at the sibling tests (Report 2-7)
3. `internal/presenter/prompt.go` (plain.go:345 / pretty.go:745) — cross-reference the whitespace-only⇒default rule from each `Prompt` doc comment (currently documented only on `parseChoice`) (Report 3-3)

### Quick-fixes
4. `internal/presenter/presentertest/recording_test.go` — add focused unit tests for the recorder hooks that are exercised only indirectly (Report 1-2)
   - Prompt answer-resolution precedence (`PromptResult` > `NextChoices` > `gate.Default`; `NextChoices` FIFO pop with nil error)
   - `SuspendSpinner`/`ResumeSpinner` each append a payload-less Event of the correct Kind in order
5. `internal/presenter/plain_test.go` + `pretty_test.go` — add empty-`Summary` `Unwound` render assertions (plain `Unwound(Unwind{})` → `"unwound: \n"`; pretty → `↩` line with no summary) to lock the documented "empty string is legal" claim (presenter.go:436-438) (Report 2-8)
6. `internal/presenter/prompt_test.go:156` — add a per-mode EOF positive-path case (input `"y"` with no trailing newline → `ChoiceYes`, nil error), currently exercised only indirectly (Report 3-3)
7. `internal/presenter/pretty_suspend_test.go:170` — strengthen `TestStageFailedWhileSuspendedClearsSuspendedState` (and its success twin) to also assert `"✗ notes"` still emits after a suspended-then-failed sequence (Report 4-6)
8. `internal/presenter/pretty_gate_test.go:128` — tighten the positive `Contains(got, "    n  abort")` to `hasExactLine` for consistency with the 9-2 rationale (the line is deterministic) (Report 9-2)
9. `internal/presenter/gate_helpers_test.go:113-115` — confirm `gateDriver.prompt` has a consumer; remove it if it is dead test-helper surface (Report 6-1)

### Ideas
10. `internal/presenter/gate.go` — gate-model contract decisions (Reports 3-1, 3-7)
    - :103-106 — decide whether to enforce the documented "Default must be a member of Choices" invariant (validation helper / constructor check) or leave it as a by-construction contract (Report 3-1)
    - :193 — confirm with the spec author whether the plain source/target prompt should keep the `Source? [github/gitlab]` form or use a literal `source: [github/gitlab]` form (Report 3-7)
11. `internal/presenter/spinner.go` + plain UI-library guard — spinner packaging/payload decisions (Report 4-5)
    - :70-78 — decide whether to extend `StageStart` with an engine-supplied start-detail field for the richer spinner suffix (`generating with claude…`), or accept the payload-faithful bare `Name`
    - plain_test.go:1011-1015 + spinner.go:7 — decide whether to split the spinner into a sub-package so "plain pulls in no UI library" holds at link level (it currently holds at source-import level only, since plain and the briandowns spinner share one package), or document the guard as source-level by design
12. `internal/presenter/wiring.go` — startup-seam convergence (Reports 8-1, 1-6)
    - :21,64 — express the mode-branch + out/err stream-split once (a small unexported helper) to genuinely satisfy the one-place intent rather than the doc-only fallback (Report 8-1)
    - :64-75 — when the `main`/`cmd` package lands in a later work unit, route production startup through `NewForStartup` so the `os.Stdout`/`os.Stderr` default wiring is exercised end-to-end rather than only in tests (Report 1-6)
13. `internal/presenter/bytepurity_test.go:39` — decide whether to keep `streamLabels`' open-ended variadic or tighten the signature to `(out, err *bytes.Buffer)` (the third+ buffer fallback is unused) (Report 5-1)
14. `internal/presenter/pretty_width_test.go:64,67,100,103` — decide whether to route the `want 50` cap assertions through `presenter.RuleCapForTest` for full single-sourcing or keep `50` as a deliberate spec-pin (Report 6-3)
15. `internal/presenter/prompt_render_only_test.go:22-25` — consider an AST-level guard flagging `os.StartProcess` call expressions to close the residual subprocess-guard gap (bare `os` is legitimately imported, so the import scan can't catch it); low value today as no such call exists (Report 3-8)
16. `internal/presenter/golden_transcript_test.go:123-125` — decide whether a future pass should route the standalone full-transcript driver through `plainGate` or leave it deliberately standalone (Report 6-1)
17. `internal/presenter/wiring_test.go:469-530` — if a pty-based test helper is ever introduced, consider one `NewForStartup` case driving a real TTY stdout/stdin end-to-end (the "all four TTY combinations against the seam" bullet is currently satisfied indirectly because real TTYs aren't CI-creatable) (Report 7-2)
