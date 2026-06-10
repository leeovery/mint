---
topic: cli-presentation
cycle: 2
total_proposed: 4
---
# Analysis Tasks: CLI Presentation (Cycle 2)

## Task 1: Extract shared gate-test construction+capture helpers
status: approved
severity: medium
sources: duplication

**Problem**: The presenter-construction-and-capture idiom (`out := &bytes.Buffer{}; errBuf := &bytes.Buffer{}; p := presenter.NewPlainPresenterWithInput(out, errBuf, strings.NewReader(input)).WithYes(...)` and its `NewPrettyPresenter(out, WithProfile(...), WithInput(...))` twin) is hand-inlined ~30 times across gate_skip_test.go, gate_forbidden_test.go, and gate_sourcetarget_test.go. plain_test.go/pretty_test.go already established `drive`/`drivePretty` and prompt_test.go established `drivePlainPrompt`/`drivePrettyPrompt`, but the gate-* files re-authored construction inline. A construction-API change must be touched at ~30 sites instead of 2, and the `.WithYes(true)` / `.WithInteractiveStdin(false)` arming flag is easy to omit by copy-paste drift.

**Solution**: Add small shared constructors in a single test helper file (e.g. `gate_test_helpers.go` in package `presenter_test`) such as `plainGate(input string, opts...) (p, out, errBuf)` and `prettyGate(input string, profile, opts...) (p, out)` that take the `-y` / interactive-stdin toggles as parameters, reusing/extending the existing prompt drivers. Have the three gate test files call them. Only collapse the repeated construction+capture lines already present; do not invent new behaviour.

**Outcome**: A single construction+capture seam for gate tests; a future presenter-construction-API change touches ~2 helper sites instead of ~30; arming-flag drift is eliminated because the toggle is an explicit parameter.

**Do**:
1. Create `internal/presenter/gate_test_helpers.go` in package `presenter_test`.
2. Add a `plainGate` helper that allocates `out`/`errBuf` buffers, builds a plain presenter from the input string, applies the `-y`/interactive-stdin toggles passed as parameters, and returns the presenter plus both buffers.
3. Add a `prettyGate` helper that allocates `out`, builds a pretty presenter with the supplied profile + input, applies the toggles, and returns the presenter plus the buffer.
4. Replace the inline construction+capture blocks in gate_skip_test.go, gate_forbidden_test.go, and gate_sourcetarget_test.go (FIND the actual sites by grepping; the synthesizer's line numbers are approximate) with calls to the new helpers.
5. Keep each test's assertions unchanged; only the construction/capture lines move into the helpers.

**Acceptance Criteria**:
- One shared helper file provides `plainGate`/`prettyGate` (or equivalently named) constructors parameterised by the `-y`/interactive-stdin toggles.
- gate_skip_test.go, gate_forbidden_test.go, and gate_sourcetarget_test.go no longer hand-inline the buffer-allocation + presenter-construction idiom; they call the helpers.
- No behavioural change to any test assertion; the arming toggles are passed explicitly at every call site.

**Tests**:
- `go test ./internal/presenter/...` passes with the refactored gate tests still exercising the same skip/forbidden/source-target paths.
- `go vet ./internal/presenter/...` is clean and the new helper file compiles under the `presenter_test` package.

## Task 2: Consolidate mode-invariant both-arm assertions through promptDrivers table
status: approved
severity: medium
sources: duplication

**Problem**: Many gate/prompt tests assert "the same outcome holds in BOTH render modes" by writing the plain block then copy-pasting a structurally identical pretty block immediately below. prompt_render_only_test.go already solved this with the `promptDrivers()` table + `t.Run(d.mode, ...)` pattern, but prompt_test.go, gate_skip_test.go, gate_forbidden_test.go, and gate_sourcetarget_test.go hand-duplicate the two arms. The arms drift independently and a new mode-invariant property must be written twice.

**Solution**: Where a property is genuinely mode-invariant, drive it through the existing `promptDrivers()`-style table (one driver per mode) and assert once inside `t.Run(d.mode, ...)`. Keep mode-SPECIFIC rendering tests (exact bytes of the plain hint vs the pretty menu) separate — only consolidate the arms whose assertion body is genuinely identical across modes.

**Outcome**: Mode-invariant properties are asserted once across a driver table, so the two render-mode arms cannot drift apart and a new invariant is written a single time; mode-specific byte-exact rendering tests remain untouched.

**Do**:
1. Identify the copy-pasted plain-then-pretty arm pairs in prompt_test.go, gate_skip_test.go, gate_forbidden_test.go, and gate_sourcetarget_test.go whose assertion bodies are structurally identical across modes (grep/read to find them precisely).
2. For each such pair, replace the two inline arms with a loop over the existing `promptDrivers()`-style driver table, asserting the shared outcome (choice/error) once inside `t.Run(d.mode, ...)`.
3. Leave any arm that asserts mode-specific bytes (plain hint text vs pretty menu rendering) as a separate, dedicated test — do not fold those into the table.
4. Reuse the helpers from Task 1 where the table needs to construct presenters, but do not make this task depend on Task 1 being done first — fall back to inline construction inside the driver if needed.

**Acceptance Criteria**:
- Each consolidated mode-invariant property is asserted exactly once via a `t.Run(d.mode, ...)` loop rather than two copy-pasted arms.
- Mode-specific rendering assertions (exact plain hint vs pretty menu bytes) remain in their own tests and are not merged.
- The set of properties verified before and after is unchanged — no invariant is dropped, none is added.

**Tests**:
- `go test ./internal/presenter/...` passes; both render modes are still covered for every consolidated property (visible via subtest names `t.Run(d.mode, ...)`).
- Running with `-run` against a specific mode subtest shows each invariant executes once per mode.

## Task 3: Consolidate decorative notes-rule expectation and the ANSI-strip helper into one shared pretty test helper
status: approved
severity: medium
sources: duplication

**Problem**: The decorative notes-rule expectation is reconstructed in three places: pretty_test.go defines `notesTitledRule`/`notesClosingRule` (rebuilding the `"── release notes · v"+version+" "` prefix, the `width - len([]rune(prefix))` fill clamp, and the U+2500 repeat), pretty_width_test.go independently defines `notesTitlePrefix` rebuilding the same literal prefix, and both mirror production's `notesTitledRule`/`notesClosingRule` in pretty.go. pretty_test.go also hardcodes `decorativeRuleWidthForTest = 50`, re-encoding production's `ruleCap` (width.go). Separately, the `stripANSI` CSI-SGR stripper lives only in pretty_width_test.go while other pretty tests dodge ANSI by pinning `termenv.Ascii` — a near-duplicate-in-waiting.

**Solution**: Consolidate the test-side rule expectation into one shared helper (prefix string + fill/clamp logic in a single place reused by both pretty_test.go and pretty_width_test.go), reference the production `ruleCap`-equivalent value once instead of hardcoding `50` a second time, and promote `stripANSI` (with its `ruleDisplayWidth` consumer) into the same shared pretty test-helper file so colour-aware assertions draw on one stripper.

**Outcome**: The notes-rule prefix literal and fill arithmetic exist once on the test side and reference production's `ruleCap` once; `stripANSI` is a single shared primitive available to all pretty tests, removing both the third copy of the prefix and the second-stripper risk.

**Do**:
1. Create (or reuse) a shared pretty test-helper file in package `presenter_test`.
2. Move the rule-expectation construction (prefix `"── release notes · v"`, the fill clamp, U+2500 repeat) into one helper there, and have both pretty_test.go and pretty_width_test.go call it (removing pretty_width_test.go's separate `notesTitlePrefix`).
3. Replace the hardcoded `decorativeRuleWidthForTest = 50` with a single reference to production's `ruleCap` value (note: `ruleCap` is package-internal in package `presenter`; the tests are black-box `presenter_test`, so either expose the cap via a tiny test accessor in an internal `export_test.go`, or define the test constant once and document it mirrors `ruleCap` — pick the cleaner option and avoid a second scattered `50`).
4. Move `stripANSI` (and `ruleDisplayWidth`) into the same shared helper file so pretty_test.go / pretty_width_test.go / pretty_gate_test.go all reference one stripper.
5. Update call sites to use the consolidated helpers.

**Acceptance Criteria**:
- The `"── release notes · v"` prefix literal and the fill/clamp arithmetic appear exactly once in the test tree.
- The decorative rule width is sourced once (not a duplicated scattered `50` literal).
- `stripANSI`/`ruleDisplayWidth` live in one shared helper file and are referenced (not re-defined) by the consuming pretty tests.

**Tests**:
- `go test ./internal/presenter/...` passes; the notes-rule and width assertions still verify the exact rendered rule.
- A grep for `"── release notes · v"` and for `func stripANSI` in the test tree each return a single definition site.

## Task 4: Add golden full-worked-example transcript test per render mode
status: approved
severity: medium
sources: architecture

**Problem**: Every cross-task seam is exercised individually, but no test drives the complete worked-example sequence end-to-end and asserts the whole rendered transcript. The richest sequence tests cover only three events. The spec ships two fully-worked ~15-event transcripts (the plain `-y` agent run and the pretty human run). Those transcripts are the contract for how per-event renderings COMPOSE — inter-line spacing, the notes block between plan and gate, the gate echo landing in the stage column, the footer after the last stage. A composition regression would slip through every existing assertion because the pieces are verified in isolation, not their assembly.

**Solution**: Add one golden-transcript test per mode that drives the spec's full worked example (the plain `-y` run and the pretty run) and asserts the output against the composed transcript, pinning the composition of per-event renderings. IMPORTANT: assert the IMPLEMENTATION's actual composed output, grounded in the spec's worked examples for ordering/spacing — do NOT force the spec's exact illustrative bytes where the implementation deliberately differs (e.g. the notes body is verbatim/flush, not indented, per task 2-5; the plain start verb is "running...", per task 5-2). The test must pin genuine composition (block ordering, inter-block spacing, gate echo placement, footer position), not encode a tautology — verify by mutating a composition detail and confirming failure.

**Outcome**: The composed transcript for each mode is locked as a golden contract, so any composition regression in spacing, block ordering, column alignment, or footer placement fails a test.

**Do**:
1. Transcribe the spec's two worked-example event sequences (plain `-y` agent run; pretty human run) into ordered event streams.
2. Add a plain-mode golden test: construct the plain presenter (`-y`), feed the full ~15-event sequence (RunStarted, version/preflight stages, ShowPlan, blocking notes stage, ShowNotes, gate, record/tag/push/publish stages, RunFinished), capture stdout/stderr, and assert the captured transcript equals an expected golden string (the most byte-stable target). Build the golden from the implementation's actual composed output reconciled with the spec's worked example (same ordering/blocks; use the implementation's real wording — `running...`, verbatim notes body, etc.).
3. Add a pretty-mode golden test driving the same sequence; drive the spinner with a spy/no-op factory (`WithSpinnerFactory`) so output is deterministic, and force a colour profile (or Ascii) so the bytes are stable — assert the composed stable lines (strip ANSI if asserting under colour).
4. Store the expected transcripts as in-test constants/golden strings.
5. Reuse the helpers from Tasks 1/3 where convenient, but keep this test self-contained (do not hard-depend on the other tasks).

**Acceptance Criteria**:
- A plain-mode test drives the spec's full plain `-y` worked example and asserts the complete composed transcript.
- A pretty-mode test drives the spec's full pretty worked example with a spy/no-op spinner and a fixed profile, asserting the stable composed lines.
- The transcripts assert composition (inter-block spacing, notes-between-plan-and-gate ordering, gate echo column, trailing footer), not just individual events — and are NOT tautological (a mutated composition detail fails the test).

**Tests**:
- `go test ./internal/presenter/...` passes with the two new golden-transcript tests.
- Deliberately introducing a composition defect (e.g. an extra blank line between the notes block and the gate) causes at least one golden-transcript test to fail (verify during development, then revert the defect).
