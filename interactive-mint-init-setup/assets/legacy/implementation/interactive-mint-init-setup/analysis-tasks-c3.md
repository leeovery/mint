---
topic: interactive-mint-init-setup
cycle: 3
total_proposed: 1
---
# Analysis Tasks: interactive-mint-init-setup (Cycle 3)

## Task 1: Pin the max_diff_lines SoT default to its canonical constant (close the last unpinned shared scalar default)
status: approved
severity: medium
sources: architecture, duplication

**Problem**: The spec's "initgen scope of change" section is explicit: "The SoT `default` column becomes the drift-pinned carrier of those values. No default value is left unpinned by the change." Two of the three shared scalar defaults honour this — `ai_command` and `timeout` source their SoT cells from the EXPORTED `config.DefaultAICommand` / `config.DefaultTimeout` constants (metadata.go:126, 128), each with a dedicated subsuming drift pin. `max_diff_lines` breaks the pattern: its canonical default lives in the UNEXPORTED `defaultMaxDiffLines = 50000` (config.go:68), so the SoT row hardcodes the literal `"50000"` (metadata.go:127) and the value test pins it against another bare `"50000"` (metadata_test.go:381), with NOTHING coupling them. The value `50000` is now re-typed independently in four places — the const, the SoT cell, the SoT test, and the README table (README.md:205). The bijection test only guards key PRESENCE per (level, key), not default VALUES, so it gives no cover: a future change to `defaultMaxDiffLines` would silently diverge from the agent-facing config reference (and the README) while every test stays green — exactly the drift the SoT was built to prevent. Separately, the same value test (ConcreteScalarDefaultsRenderVerbatim, metadata_test.go:380) derives the timeout seconds as `strconv.Itoa(int(config.DefaultTimeout.Seconds()))` (a float64 truncation) while the production SoT and its dedicated pin derive it as `int(config.DefaultTimeout / time.Second)` (metadata.go:128, metadata_test.go:337) — two spellings of one drift-pin that can quietly diverge (e.g. a sub-second DefaultTimeout would truncate differently), with no signal to a reader which is canonical.

**Solution**: Export the `defaultMaxDiffLines` constant, source the SoT `max_diff_lines` cell from it, add a dedicated subsuming drift pin tying the cell to the constant (symmetric with the existing ai_command/timeout pins), and normalise the divergent timeout-seconds spelling in the value-render test to the single production spelling. This makes the "SoT default column is the drift-pinned carrier" invariant total across all three shared scalars, as the spec requires.

**Outcome**: All three shared scalar defaults (`ai_command`, `timeout`, `max_diff_lines`) source their SoT cell from an exported config constant and each has a subsuming drift pin; no shared default is left as an uncoupled re-typed literal. The one timeout-seconds derivation used in tests matches the production SoT spelling. Changing `defaultMaxDiffLines` in config.go forces the SoT cell (and its pin) to move with it, and the new pin fails the build if they drift apart.

**Do**:
1. In `internal/config/config.go`: rename the unexported `const defaultMaxDiffLines = 50000` (line 68) to the exported `const DefaultMaxDiffLines = 50000`. Update the doc comment above it (lines 64-67) to reflect the exported name. Update its two in-package uses: `MaxDiffLines: defaultMaxDiffLines` (line 309) and `return defaultMaxDiffLines` (line 645) to `DefaultMaxDiffLines`. Confirm with `grep -rn "defaultMaxDiffLines" internal/config/` that no references to the old name remain.
2. In `internal/config/metadata.go` (line 127): change the `max_diff_lines` row's `Default: "50000"` to `Default: strconv.Itoa(DefaultMaxDiffLines)`. `strconv` is already imported (used by the timeout row on line 128). Update the WHY-comment block above the shared rows (lines 122-125, which currently names only ai_command/timeout as constant-sourced) so it states all three shared scalar defaults — ai_command, timeout, and max_diff_lines — are sourced from exported config constants and pinned, keeping the comment true to as-built.
3. In `internal/config/metadata_test.go`: add a new dedicated drift-guard test `TestMetadataRows_SharedMaxDiffLinesDefaultEqualsConfigConstant`, modelled exactly on `TestMetadataRows_SharedAICommandDefaultEqualsConfigConstant` (lines 309-320) — fetch the `(LevelShared, "max_diff_lines")` row via `configtest.MustByLevelKey(t)`, fail if missing, and assert `row.Default == strconv.Itoa(config.DefaultMaxDiffLines)`. Give it a WHY doc comment matching the style of the existing two pins, naming it the subsuming pin tying the SoT cell to `config.DefaultMaxDiffLines`.
4. In `internal/config/metadata_test.go` `TestMetadataRows_ConcreteScalarDefaultsRenderVerbatim` (lines 371-398): change the `max_diff_lines` row's `want` from the literal `"50000"` (line 381) to `strconv.Itoa(config.DefaultMaxDiffLines)`. Change the `timeout` row's `want` (line 380) from `strconv.Itoa(int(config.DefaultTimeout.Seconds()))` to `strconv.Itoa(int(config.DefaultTimeout / time.Second))` so it matches the production SoT spelling (metadata.go:128) and the dedicated timeout pin (line 337); `time` is already imported in the test file (used at line 337). Update the test's doc comment (lines 364-370) so it stays true: max_diff_lines is now pinned against the exported constant, not a bare literal.
5. README.md (line 205): leave the `| max_diff_lines | 50000 | ... |` row as-is — it is the human GitHub-browsing surface, light duplication is spec-sanctioned, and the value `50000` is unchanged. (No edit needed; the coupling guarantee applies to the machine/agent SoT surface.)
6. Run the full gate suite: `go build ./...`, `gofmt -l .` (must print nothing), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues).

**Acceptance Criteria**:
- `DefaultMaxDiffLines` is an exported constant in `internal/config/config.go`; no reference to the old unexported `defaultMaxDiffLines` name remains anywhere in the package.
- The `max_diff_lines` SoT row in `metadata.go` sources its `Default` from `strconv.Itoa(DefaultMaxDiffLines)`, not a re-typed literal.
- A dedicated test pins the `(shared, max_diff_lines)` SoT Default cell to `config.DefaultMaxDiffLines` and fails the build if they drift, symmetric with the existing ai_command/timeout pins.
- The value-render test derives the timeout-seconds `want` as `int(config.DefaultTimeout / time.Second)` — the single spelling used by both production and the dedicated timeout pin — with no remaining `.Seconds()` truncation spelling.
- The WHY-comments in metadata.go and the affected test doc comments are updated to be true to as-built (all three shared scalars sourced+pinned).
- All gates pass: `go build ./...`, `gofmt -l .` prints nothing, `go vet ./...`, `go test -race ./...`, `golangci-lint run` reports 0 issues.

**Tests**:
- New `TestMetadataRows_SharedMaxDiffLinesDefaultEqualsConfigConstant`: asserts `(shared, max_diff_lines)` Default equals `strconv.Itoa(config.DefaultMaxDiffLines)`; a hand-mutation of the SoT cell to a divergent literal must make this test fail (drift guard proven by construction, mirroring the existing two pins).
- Existing `TestMetadataRows_ConcreteScalarDefaultsRenderVerbatim` continues to pass with the constant-sourced `max_diff_lines` want and the normalised timeout-seconds spelling.
- Existing SoT bijection / value tests and the setup-guide render tests stay green (the rename and constant-sourcing are value-preserving — `50000` and `60` are unchanged).
