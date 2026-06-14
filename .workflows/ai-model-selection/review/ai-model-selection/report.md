# Implementation Review: AI Model Selection

**Plan**: ai-model-selection
**QA Verdict**: Approve

## Summary

A clean, fully-realised implementation. All 20 tasks across 5 phases (3 build phases + 2 analysis cycles) were independently verified against their acceptance criteria and the specification; every one is **Complete** with **zero blocking issues**. The feature lands exactly as specified: the shipped default is pinned to `claude -p --model sonnet` as a single canonical constant in `internal/config`; `ai_command` and `timeout` both resolve through the layered `verb → shared → default` chain via typed accessors (`AICommandFor`/`TimeoutFor`) keyed on a closed two-value `Verb` enum with no `regenerate` value; the net-new `timeout` key honours the `0 ⇒ no deadline` semantics with the transport conditionally skipping `context.WithTimeout`; and the absent-vs-explicit-zero invariant is carried across the config→`ai.Config` boundary by a `*time.Duration` so "no deadline" is only ever reachable by an operator's explicit `0`, never by a wiring site omitting the field. The two analysis cycles added genuine quality: the three duplicated transport-construction sites are consolidated into `internal/aitransport` (preserving the `config`↛`ai` decoupling), and the duplicated deadline-recording test spy is extracted into `internal/runner`. Doc-comment migrations (transport WHY-comments, the `Commit` struct contract) were done in the same change per CLAUDE.md. Remaining notes are all non-blocking polish.

## QA Verification

### Specification Compliance

Implementation aligns with the specification across all decided behaviours:

- **Pinned default** — `config.DefaultAICommand == "claude -p --model sonnet"`, exported as the canonical source; alias form (not full versioned ID) as specified.
- **Per-verb override + resolution order** — `[verb].ai_command → top-level ai_command → shipped default`, with blank/whitespace trim-and-skip applied at every layer in one place (the old `resolveAICommand` folded into the accessor); `ai_command` never empties out.
- **`timeout` value semantics** — explicit `0` honoured and stops fall-through; negative/invalid drops through to the 60s floor; positive used as-is; transport skips `WithTimeout` on `0` and never collapses a residual negative into the no-deadline branch.
- **Per-key independence** — proven both directions, both verbs.
- **Regenerate routes through `[release]`** — `regenerate_fresh.go` resolves via `config.VerbRelease`; the closed enum makes any other routing un-writable.
- **Strict decoding** — new keys added to both verb shapes and top-level with `typeErrorMessages`; genuine TOML type mismatches still fail loud at `Load`.
- **Single source of truth** — transport `defaultAICommand`/`defaultTimeout` deleted; `initgen` and README derive the pinned value; no driver/provider pattern, no env layer, no reflection/service-locator; `config` never imports `ai`.
- **Operator surfacing** — initgen template scaffolds the new keys (top-level active, per-verb commented), comments stay model-agnostic; README documents both-level keys, resolution order, the `claude -p --model sonnet` fact, `timeout = 0` trade-off, and the unenforced override-both pattern.

No deviations found.

### Plan Completion

- [x] Phase 1–5 acceptance criteria met (verified per-task)
- [x] All 20 tasks completed and independently verified Complete
- [x] No scope creep — `internal/aitransport` and the `internal/runner` shared spy are planned analysis-cycle tasks (4-1, 5-1), not unplanned additions

### Code Quality

No issues found. Typed enum gives compile-time routing safety; the `*time.Duration` boundary with inverse-polarity internal carrier is a deliberate, well-commented mechanism enforcing the fail-loud invariant; the `aitransport.New` helper is a thin single-responsibility seam justified by three byte-identical call sites and a real drift risk, without over-abstraction. WHY-comments were kept true-to-as-built across the change.

### Test Quality

Tests adequately verify requirements. Acceptance-criteria behaviours map 1:1 to focused, non-redundant tests; exact-argv and exact-rendered-line idioms are honoured; the deadline-recording spy is directly tested on both branches; migration test-pins (`claude -p` → `claude -p --model sonnet`) are complete and bounded, with the sole surviving bare `claude -p` confirmed an unrelated FakeRunner-mechanics fixture. Minor redundancy noted below (one quick-fix).

### Required Changes (if any)

None.

## Recommendations

### Do now

1. Comment / doc wording fixes (zero logic impact)
   - `internal/config/config_test.go:875` — comment reads `must override the "claude -p" default`; update to the current `claude -p --model sonnet` default so it doesn't encode the pre-migration value (Report 2-6)
   - `internal/config/config.go:237` — drop the bare plan task-id `(assembly, 1-2)` from the `Commit` struct doc comment; it means nothing to a future reader of shipped code (Report 3-3)
   - `internal/runner/deadline_recording_runner.go:64` — `DurationPtr` godoc reads "returns a pointer to d"; rename the param (e.g. `dur`) or rephrase to "the given duration" so the doc reads cleanly in isolation (Report 5-1)

### Quick-fixes

2. `internal/config/config_test.go:1992` & `:2036` — fold the two tests that assert the same explicit-`timeout = 0` → non-nil-pointer-to-0 behaviour with identical TOML into one, removing the redundant case (Report 1-5)

### Ideas

3. `internal/initgen/initgen.go` — template literal sourcing & drift pinning
   - `:47` — the top-level `ai_command` is a static literal pinned-equal to `config.DefaultAICommand` by a drift test (initgen deliberately doesn't import config); decide whether that satisfies the spec's "sourced from the config constant, not re-typed" or whether a true compile-time reference is wanted (Report 1-1)
   - `:66,80` — the per-verb commented example value `'claude -p --model sonnet'` is hand-typed and not drift-pinned; decide whether to extend the drift pin to the example lines or accept them as illustrative (Report 3-1)
   - `:67,81` — both per-verb `timeout` override examples reuse `120`; consider whether distinct values communicate "arbitrary example" better, or leave for consistency (Report 3-1)
4. `internal/config/config.go` — read-only contract & comment label
   - `:606,614` — `TimeoutFor` returns the Config's stored override/shared pointers directly (aliasing); safe under the current read-only contract, but decide whether to return a fresh copy for defensive immutability (Report 1-7)
   - `:171` — the comment carries a "Phase 2" phase-origin label that a future reader has no map for; decide whether to drop the qualifier (Report 4-2)
5. `internal/config/config_test.go:2684` — optional symmetric independence case (override the per-verb *timeout* while only a shared `ai_command` is set); the existing four functions already cover both directions, so this is optional symmetry, not a gap (Report 1-8)
6. `internal/runner/deadline_recording_runner.go:17` — the spy overwrites last-call state with no concurrency guard (FakeRunner shares this); if a future test drives one instance from multiple goroutines or asserts full call history, decide whether to add a mutex and/or accumulate an `[]Invocation` slice. No action needed for current single-call-per-subtest usage (Report 5-1)
