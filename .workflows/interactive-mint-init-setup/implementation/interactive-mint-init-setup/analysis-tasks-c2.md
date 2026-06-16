---
topic: interactive-mint-init-setup
cycle: 2
total_proposed: 6
---
# Analysis Tasks: Interactive Mint Init Setup (Cycle 2)

## Task 1: Consolidate the 25-pair (level, key) expected census to one shared list
status: pending
severity: medium
sources: duplication

**Problem**: `TestMetadataRows_OneRowPerLevelKeyPair` (internal/config/metadata_test.go:43-73) and `TestSchemaLeafKeys_DerivesAllLeafPairs` (internal/config/metadata_drift_test.go:125-155) each carry the SAME ~30-line literal enumeration of all 25 (level, key) pairs — identical ordering, identical grouped comment structure (Shared / [release] / [release.hooks] / [commit]), differing only in element type (`config.rowKey{config.LevelShared, "ai_command"}` vs the internal `leafKey{LevelShared, "ai_command"}`). A schema key added, renamed, or removed must be edited in BOTH lists by hand, and the two will silently diverge if an editor touches only one. This is a hand-maintained third copy of the census — neither the SoT rows nor the reflection-derived schema tags.

**Solution**: Extract the single ordered expected-pair census to one shared internal slice in the config package (an unexported `expectedLeafKeys` var, or a same-package helper returning `[]leafKey`, reusable because metadata_drift_test.go is `package config`). Have both naming tests assert against that one list — metadata_test.go's `config_test` rows can be projected from it. Keep the SoT-side derivation (`MetadataRows()`) and the schema-side derivation (`schemaLeafKeys()`) independent; only the hand-written expected census is consolidated, not the two drift sides.

**Outcome**: One ordered census exists in exactly one place. Adding/renaming/removing a schema key requires editing the census once, and the two naming tests stay in lockstep automatically. The deliberately-independent SoT vs reflected-schema bijection sides remain unmerged, preserving the design's anti-drift value.

**Do**:
1. In package config (test scope), define one ordered source of the 25 expected (level, key) pairs — an unexported `expectedLeafKeys` slice of `[]leafKey` (or a helper returning it), carrying the existing grouped comments verbatim.
2. Rewrite `TestSchemaLeafKeys_DerivesAllLeafPairs` in metadata_drift_test.go to assert the schema-derived set against this single census instead of its inline literal.
3. Rewrite `TestMetadataRows_OneRowPerLevelKeyPair` in metadata_test.go to derive its expected `config.rowKey` set by projecting the shared census (mapping each `leafKey` to the `config_test` rowKey type), removing its inline literal.
4. Confirm `MetadataRows()` and `schemaLeafKeys()` are NOT touched — the two derivations stay independent.

**Acceptance Criteria**:
- The full 25-pair (level, key) enumeration appears exactly once in the config package test sources.
- Both naming tests reference the shared census; neither carries an inline 25-pair literal.
- `MetadataRows()` and `schemaLeafKeys()` remain separate, independent derivations.

**Tests**:
- Both `TestMetadataRows_OneRowPerLevelKeyPair` and `TestSchemaLeafKeys_DerivesAllLeafPairs` pass against the shared census.
- A deliberate single-pair removal from `MetadataRows()` (verified locally, then reverted) still fails the bijection drift test, proving the consolidation did not weaken the bijection.

## Task 2: Reuse a single toml-tag-reading primitive in tomlTagsOf and schemaLeafKeysInto
status: pending
severity: low
sources: duplication

**Problem**: `tomlTagsOf` (internal/config/metadata_drift_test.go:261-269) re-implements the same per-field `field.Tag.Get("toml")` read with the identical `tag == "" || tag == "-"` skip guard that `schemaLeafKeysInto` (internal/config/metadata_drift_test.go:72-100) already performs, flattened into a single-level name set for the independence test. Two functions in the same file own the toml-tag-extraction rule; a change to the skip-token convention (or a future tag-option suffix like `,omitempty`) must be updated in both.

**Solution**: Introduce one small tag-reading primitive (e.g. `tomlTag(field) (name string, ok bool)`) that encapsulates the `Tag.Get("toml")` read plus the skip-token guard, and have both `tomlTagsOf` and `schemaLeafKeysInto` call it so the parse-and-skip rule lives in one place.

**Outcome**: The toml-tag parse/skip convention is single-sourced. A future change to skip tokens or tag-option handling is made once and both callers inherit it.

**Do**:
1. Add an unexported `tomlTag(field reflect.StructField) (name string, ok bool)` helper in metadata_drift_test.go that reads `field.Tag.Get("toml")` and returns `ok=false` for the `"" `/ `"-"` skip cases.
2. Refactor `schemaLeafKeysInto` to obtain its tag via `tomlTag` instead of its inline read+guard.
3. Refactor `tomlTagsOf` to obtain its tag via `tomlTag`, preserving its single-level name-set behaviour.

**Acceptance Criteria**:
- The `Tag.Get("toml")` read and the `"" `/ `"-"` skip guard appear in exactly one helper.
- Both `tomlTagsOf` and `schemaLeafKeysInto` consume that helper; neither carries its own inline tag read.

**Tests**:
- The bijection/independence tests in metadata_drift_test.go continue to pass unchanged.

## Task 3: Single-source the rowKey struct and (level,key) index map across the config and setupguide test packages
status: pending
severity: low
sources: duplication

**Problem**: internal/config/metadata_test.go:14-33 (rowKey + rowSet) and internal/setupguide/setupguide_test.go:477-492 (rowKey + rowByLevelKey) both define an identical `rowKey` struct (`{level config.MetadataLevel; key string}`) and a near-identical builder folding `config.MetadataRows()` into a `map[rowKey]config.MetadataRow` keyed on that pair. The struct is byte-for-byte the same; the builders differ only in rowSet's extra duplicate-collision Fatalf. Both were independently written to solve the same "look up the SoT row for a (level, key) pair" need across the task boundary. A change to the SoT row identity model would need touching both builders.

**Solution**: Provide one exported test-support seam in the config package — e.g. `config.MetadataByLevelKey()` returning the indexed `map` keyed on a `(level, key)` pair (or a small `config/configtest` support file) — and have both test packages consume it, keeping the (level, key) indexing single-sourced alongside the SoT it indexes.

**Outcome**: The rowKey identity and the SoT-indexing builder live in one place next to the SoT. Both the config and setupguide tests look up rows through that single seam; a change to the SoT row identity model is made once.

**Do**:
1. Add an exported indexing seam in package config (a small support file or an exported `MetadataByLevelKey()` returning the indexed map keyed on the exported pair type) that folds `MetadataRows()` and detects duplicate (level, key) collisions.
2. Replace metadata_test.go's local `rowKey`/`rowSet` usage with the shared seam, preserving its duplicate-collision assertion.
3. Replace setupguide_test.go's local `rowKey`/`rowByLevelKey` usage with the shared seam.
4. Keep the seam test-support-only; do not add it to the production render path.

**Acceptance Criteria**:
- The `rowKey` struct and the (level, key) index-map builder are defined exactly once, in the config package.
- Both metadata_test.go and setupguide_test.go consume that single seam; neither re-declares the struct or re-authors the builder.
- The duplicate-(level,key) collision detection is preserved.

**Tests**:
- The existing config and setupguide tests that previously used the local builders pass against the shared seam.
- A test proves the seam reports a collision when fed duplicate (level, key) rows (preserving rowSet's original Fatalf intent).

## Task 4: Reword emitted-guide procedure step 2 to name mint's own README unambiguously
status: pending
severity: low
sources: standards

**Problem**: The spec's setup procedure step 2 ("Learn mint — read the README and internalise mint's minimalist philosophy") refers to mint's OWN README — the human config-reference surface and the source of mint's commands/philosophy. The emitted guide (internal/setupguide/setupguide.go:112-114) renders this as "Read the project's README for mint's commands and surface". "the project's README" most naturally reads as the TARGET project's README, which would not document mint's commands or philosophy. Because the guide prose is the deliverable's load-bearing content — it drives the AI's behaviour and is the one part with no compiler per the spec's "Acceptance — prose quality" note — an agent could follow the instruction to the wrong README. No structural test catches this (the tests key on markers, not this prose).

**Solution**: Reword step 2 to name mint's README unambiguously, e.g. "Read mint's README (the human config reference)", so the agent does not conflate it with the target project's README. Keep the minimalism-philosophy clause intact.

**Outcome**: The emitted guide directs the agent to mint's own README for mint's commands/surface/philosophy with no ambiguity, faithfully matching the spec's decided intent for procedure step 2.

**Do**:
1. Edit the procedure step 2 prose at internal/setupguide/setupguide.go:112-114 to name mint's README explicitly (e.g. "Read mint's README (the human config reference) and internalise mint's minimalist philosophy").
2. Preserve the existing section markers and the minimalism-philosophy clause; change only the README-disambiguation wording.

**Acceptance Criteria**:
- Procedure step 2 names mint's own README explicitly and is not readable as the target project's README.
- The minimalism-philosophy clause is retained.
- The guide's structural markers are unchanged.

**Tests**:
- The existing structural/marker tests for the emitted guide pass unchanged.
- Add or extend an assertion that the rendered procedure step 2 contains the disambiguated "mint's README" wording.

## Task 5: Make MetadataLevel.String() distinguish an invalid level from the shared scope
status: pending
severity: low
sources: architecture

**Problem**: `MetadataLevel.String()` (internal/config/metadata.go:64-77) returns "" for both `LevelShared` (a legitimate, expected scope) and the default branch (any int outside the four declared constants — i.e. a corrupted/uninitialised level). Two semantically opposite cases produce one identical output. The render seam (internal/setupguide/setupguide.go:354-359) then maps that empty string to the "top-level" placeholder, so an out-of-range level would silently render as a valid shared-level cell in the agent-facing config table rather than being caught. The level identity is described as a closed typed identity, but the closure is not enforced: an unrecognised level reads as shared. Latent today (every row uses a declared constant), it becomes a real masking bug if a fifth level is added without a String() case.

**Solution**: Make the default branch distinguishable from `LevelShared` — have the default case panic (matching the "fail loud on an impossible enum value" posture the bijection walk already takes when it Fatalf's on an uncovered container), or return a sentinel like "?invalid-level?" that render and any future caller cannot mistake for the shared scope. Keep `LevelShared` returning "" (its real TOML form), so only the genuinely-invalid case changes behaviour.

**Outcome**: The closed-enum claim is enforced: a real `MetadataLevel` value renders as before, while an undeclared/out-of-range level fails loud (or renders as an unmistakable sentinel) instead of silently masquerading as the shared scope in the agent-facing table.

**Do**:
1. In `MetadataLevel.String()`, keep the `LevelShared` case returning "" and the other declared cases unchanged.
2. Change the default branch to fail loud — panic on the impossible enum value (consistent with the bijection walk's posture) or return a clearly-invalid sentinel string that cannot collide with any legitimate scope output.
3. Confirm the render seam at setupguide.go:354-359 no longer maps an out-of-range level onto the shared/top-level cell.

**Acceptance Criteria**:
- Each of the four declared `MetadataLevel` constants produces its existing String() output (`LevelShared` still ""); behaviour for valid levels is unchanged.
- An out-of-range `MetadataLevel` value no longer produces the same output as `LevelShared` — it panics or yields an unmistakable sentinel.
- The render path cannot turn an invalid level into a valid shared-level cell.

**Tests**:
- Table test asserting each declared level's String() output, including `LevelShared` == "".
- A test exercising an out-of-range `MetadataLevel` proves the fail-loud behaviour (panic recovered/asserted, or sentinel string), confirming it is distinguishable from `LevelShared`.

## Task 6: Add an end-to-end run("setup") dispatch test
status: pending
severity: low
sources: architecture

**Problem**: The setup feature is covered by three independent seams — `classifyCommand` routing ("setup" -> commandSetup, cmd/mint/dispatch_test.go:80-91), the `runSetup` emitter (cmd/mint/setup_test.go:16-29), and the `run()` switch wiring (cmd/mint/main.go:105-108). Each is tested alone, but nothing exercises `run([]string{"setup"})` all the way through to stdout, so the composition — that the commandSetup arm calls runSetup with os.Stdout/os.Stderr in the right order and returns its code unchanged — is unproven. A future edit that swapped the stdout/stderr arguments or wired runSetup to the wrong route would pass every existing test. `TestRun_TopLevelHelp_ExitsZero` already drives `run()` for help paths, so run()-level tests are within the suite's idiom.

**Solution**: Add one `run([]string{"setup"})` assertion (in the spirit of the existing run()-level help test) proving the dispatched route emits `setupguide.Guide()` to stdout and exits 0 — closing the composition seam between classifyCommand, the switch arm, and runSetup without re-testing the emitter internals.

**Outcome**: The full setup dispatch path is proven end to end: routing, the commandSetup switch arm, and runSetup compose correctly and exit 0. An argument-order swap or mis-wired route is caught by a test.

**Do**:
1. Add a test (alongside the existing run()-level help test in cmd/mint) that calls `run([]string{"setup"})` with captured stdout/stderr writers.
2. Assert the captured stdout contains the emitted guide (e.g. via a stable marker from `setupguide.Guide()`) and that the returned exit code is 0.
3. Keep the assertion at the composition level — do not duplicate the emitter's internal content checks already covered by setup_test.go.

**Acceptance Criteria**:
- A test drives `run([]string{"setup"})` end to end and asserts stdout carries the emitted guide and the exit code is 0.
- The test would fail if the commandSetup arm's stdout/stderr arguments were swapped or the route were mis-wired.
- The test does not re-test runSetup's internal rendering already covered elsewhere.

**Tests**:
- New end-to-end `run([]string{"setup"})` test passing, asserting guide-on-stdout and exit 0.
