---
topic: interactive-mint-init-setup
cycle: 1
total_proposed: 2
---
# Analysis Tasks: interactive-mint-init-setup (Cycle 1)

## Task 1: Disambiguate the "shared" Level token and single-source the levelCell placeholder
status: pending
severity: medium
sources: architecture, duplication

**Problem**: The config-reference table that `mint setup` emits surfaces the word "shared" with two unrelated meanings in two columns of the SAME table. `levelCell()` (`internal/setupguide/setupguide.go:347-352`) renders the empty `LevelShared` form as the literal word "shared" in the LEVEL column (rationale: a blank markdown cell is ambiguous). Separately, the per-verb override rows (`[release]`/`[commit]` `ai_command`/`timeout`) carry "shared" in the DEFAULT column to mean "inherit the shared top-level value" (`internal/config/metadata.go:135-136,150-151`). So a shared-level row reads `| ai_command | shared | claude ... |` (shared = level) while a `[release]` row reads `| ai_command | [release] | shared | ... |` (shared = inherit). A reading agent sees the same word meaning two different things in adjacent cells. The Default-column "shared" token is a spec-decided, load-bearing convention; the Level-column "shared" placeholder is a render-site invention added in `levelCell` and was not reconciled against the existing meaning. Compounding this, the test re-implements `levelCell` byte-for-byte in `internal/setupguide/setupguide_test.go:17-22`: the test-local copy returns `level.String()` with the same literal "shared" fallback, so `TestGuide_ConfigReferenceRendersLevelViaString` and the level cells inside `TestGuide_ConfigReferenceCarriesDefaultTokensVerbatim` assert against the test's own re-derivation rather than the production behaviour — a self-fulfilling test that tracks the placeholder convention instead of guarding it.

**Solution**: Render the shared level with a token that does not collide with the inherit-default "shared" token — e.g. "top-level" or "(top level)" — keeping the cell DRIVEN by `MetadataLevel.String()` (the placeholder applies only when `String()` is empty). Confine the production change to `levelCell()` in `internal/setupguide/setupguide.go`. Then eliminate the duplicated placeholder logic in the test: remove the test-local `levelCell` and have the test exercise the production placeholder instead, so the test fails when production drifts rather than tracking it. The SoT Default-column convention stays untouched; do not alter `internal/config/metadata.go`'s Default values.

**Outcome**: The emitted config-reference table uses one unambiguous token for the shared level (distinct from the inherit-default "shared"), and the level-column convention has exactly one source: a test that drifts to red if `levelCell` changes, instead of a test that mirrors `levelCell` and silently keeps passing.

**Do**:
1. In `internal/setupguide/setupguide.go`, change `levelCell`'s empty-`String()` fallback from the literal `"shared"` to a non-colliding token (recommend `"top-level"`). Keep the `level.String()` value as the driver — the placeholder still applies only when `String()` returns `""`. Update the `levelCell` doc comment so it states the new placeholder and the WHY (avoids collision with the Default-column inherit token).
2. Remove the test-local `levelCell` helper at `internal/setupguide/setupguide_test.go:17-22` (and its comment). Replace its use in the two tests that call it (`TestGuide_ConfigReferenceCarriesDefaultTokensVerbatim` and `TestGuide_ConfigReferenceRendersLevelViaString`) with an assertion sourced from production behaviour — either export a render seam from `setupguide` (e.g. a `LevelCell`/render helper) and call it, or compute the expected cell inline from `config.MetadataLevel.String()` with the new placeholder applied in ONE place in the test (a single shared expectation function whose placeholder must match production, not a re-implementation of the conditional logic the production owns).
3. Verify no other call site (tests or production) still emits the old literal "shared" for the shared level.

**Acceptance Criteria**:
- The emitted guide's config-reference table renders the shared-level rows with the new token (e.g. "top-level") in the LEVEL column, and the per-verb `[release]`/`[commit]` `ai_command`/`timeout` rows still render "shared" in the DEFAULT column — the two no longer collide.
- The level-column cell value is still derived from `config.MetadataLevel.String()`; the placeholder is applied only when `String()` is empty.
- `internal/config/metadata.go` Default-column tokens are unchanged.
- The test suite no longer contains a verbatim re-implementation of `levelCell`'s conditional; the level-cell expectation has a single source that fails if production's placeholder changes.
- All gates pass: `go build ./...`, `gofmt -l .` prints nothing, `go vet ./...`, `go test -race ./...`, `golangci-lint run` reports 0 issues.

**Tests**:
- A test that asserts a shared-level row line in the emitted guide contains the new level token (e.g. "top-level") and NOT the literal "shared" in its level cell.
- A test that asserts a per-verb override row (e.g. `[release]` `ai_command`) still carries "shared" in its default cell, proving the disambiguation did not disturb the Default convention.
- A test proving the level-cell expectation is single-sourced from production: changing the production placeholder must turn the test red (e.g. by asserting against an exported render seam rather than a test-local copy).

## Task 2: Give the blank-default render a real assertion
status: pending
severity: low
sources: architecture

**Problem**: `defaultCell()` (`internal/setupguide/setupguide.go:358-363`) turns an empty SoT default into a single space so the markdown cell stays well-formed — a deliberate render decision the spec's blank/auto/[]/shared convention rests on. No test pins that behaviour. `containsRowLine()` (`internal/setupguide/setupguide_test.go:43-53`) checks the default via `strings.Contains(line, defaultCell)`, and for a blank-default key (e.g. `[release].context`) the passed `defaultCell` is `""`, so `strings.Contains(line, "")` is unconditionally true. The "empty-string `[release].context` renders blank" subtest in `TestGuide_ConfigReferenceCarriesDefaultTokensVerbatim` (`internal/setupguide/setupguide_test.go:287-317`) therefore proves only that the key/level/description appear on a line — it asserts NOTHING about the blank rendering it claims to cover. The blank-vs-auto-vs-shared distinction is load-bearing for the agent (the SoT side pins it hard via `TestMetadataRows_SentinelAutoDefaultsRenderAuto`'s explicit not-blank assertion), but the seam where it reaches the agent — the rendered table — has no real coverage for the blank case. A regression that emitted "auto" or dropped the cell delimiter for a blank-default key would not be caught.

**Solution**: Give the blank case a real assertion that does not collapse to `Contains(_, "")`. Either assert the rendered line for a blank-default key contains the exact single-space cell sequence `defaultCell` produces (e.g. the `| | ` shape), or add a `defaultCell` unit test pinning `defaultCell("") == " "` plus an assertion on the column count / delimiter shape of a blank row. Keep the non-blank rows driven by `row.Default` as today. Do not change production behaviour — this is a test-coverage fix at the render seam.

**Outcome**: The blank-default render is genuinely pinned at the agent-facing seam: a regression that emitted "auto", an empty token, or a malformed/dropped delimiter for a blank-default key turns the test red.

**Do**:
1. Add a unit test for `defaultCell` proving `defaultCell("") == " "` (a single space) and `defaultCell(x) == x` for a non-empty token — pinning the well-formed-cell decision directly. If `defaultCell` is unexported and not reachable from the external test package, either export a minimal render seam or assert the rendered-line shape instead (next step).
2. Replace the vacuous blank-default assertion in `TestGuide_ConfigReferenceCarriesDefaultTokensVerbatim` for the blank-default key (e.g. `[release].context`): assert the rendered row line contains the exact single-space default cell sequence (the `| | ` delimiter shape) rather than `strings.Contains(line, "")`. Ensure the assertion would fail if the cell carried "auto", an empty string with no space, or a dropped delimiter.
3. Confirm `containsRowLine`'s blank-case path (or any replacement used for the blank row) no longer relies on a `strings.Contains(line, "")` that is unconditionally true.

**Acceptance Criteria**:
- A regression that renders "auto" (or any non-blank token) for a blank-default key (e.g. `[release].context`) causes the test suite to fail.
- A regression that drops the cell delimiter or collapses the blank cell so the row is malformed causes the test suite to fail.
- The blank-default assertion no longer reduces to `strings.Contains(line, "")`.
- Non-blank-default rows remain driven by `row.Default` and still pass.
- All gates pass: `go build ./...`, `gofmt -l .` prints nothing, `go vet ./...`, `go test -race ./...`, `golangci-lint run` reports 0 issues.

**Tests**:
- A `defaultCell` unit test: `defaultCell("")` returns a single space; `defaultCell("auto")` returns `"auto"`.
- A render-seam test asserting a blank-default key's row line carries the exact single-space default cell (e.g. the `| | ` shape) and not "auto" or a missing delimiter.
