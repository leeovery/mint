# Implementation Review: Interactive Mint Init Setup

**Plan**: interactive-mint-init-setup
**QA Verdict**: Approve

## Summary

The feature is implemented faithfully and completely. All 23 plan tasks across 7 phases (4 build phases + 3 analysis-remediation cycles) were independently verified against their acceptance criteria, the specification, and the project's Go conventions — every task returned **Complete with 0 blocking issues**. The core design lands as specified: a single in-binary config-metadata source of truth (`config.MetadataRows()`) with a typed level enum, drift-tested against the real decode-shape schema via a total per-`(level,key)` bijection derived mechanically from struct `toml` tags; a pure `mint setup` emitter that renders the SoT config reference and carries the required marker-anchored guide sections; `initgen` stripped to a minimal empty-body + dual-pointer header with the scaffold value-drift discipline cleanly subsumed by the SoT pins; and the README reconciled as the human config reference with an any-AI entry point. The three analysis cycles tightened real weaknesses (a "shared" token collision, a vacuous blank-default assertion, duplicated test census/seams, an ambiguous README reference, an invalid-level masking risk, missing end-to-end dispatch coverage, and the last unpinned shared scalar default). All standard gates pass.

## QA Verification

### Specification Compliance

Implementation aligns with the specification. Verified end to end:
- **SoT + drift test** (spec "Config-metadata source of truth", "Drift test"): 25 `(level,key)` rows, tri-level `ai_command`/`timeout`, recurse-don't-count containers, total bijection over leaf keys with synthetic-divergence coverage of the guard mechanism.
- **`default` column representation** (spec "`default` column representation"): blank / `auto` / `[]` / `shared` / hooks-blank applied per the decided convention, blank-vs-`auto` distinction load-bearing and tested.
- **`mint setup`** (spec "The `mint setup` subcommand"): pure stdout emitter, runs unconditionally (no git/cwd guard), stable section markers grepped structurally, `--help` exits 0 via `flag.ErrHelp`, threaded through the curated-help surface with coverage test extended.
- **Strip-to-minimal** (spec "Generated config: strip to minimal", "`initgen` scope of change"): empty body + dual-pointer header, old commented-template assertions removed, scaffold value-drift pins subsumed by the SoT, `ReleaseShim()` untouched.
- **README** (spec "README — entry point / any-AI framing / config reference verification"): Configuration intro + Commands `init` line reconciled, minimal template embedded, any-AI Opus-steer entry point routes to `mint setup` without restating it, optional key-presence tripwire test added.

No deviations from spec decisions found.

### Plan Completion

- [x] Phase 1–7 acceptance criteria met
- [x] All 23 tasks completed and verified
- [x] No scope creep — every changed file traces to a task; analysis-cycle work was scoped remediation, not new feature surface

### Code Quality

No issues found. The code follows the project's non-negotiable seams (cmd-layer stdout for the pure emitter, no `os/exec`, strict config schema) and the Go skill conventions (closed-enum `String()` returning a self-describing sentinel rather than panicking, single-sourced reflection/test primitives, WHY-comments kept true to as-built). Non-blocking polish items are listed under Recommendations.

### Test Quality

Tests adequately verify requirements without over-testing. Highlights: the bijection guard is factored into a pure helper unit-tested on synthetic divergent inputs (the real schema can't be mutated); the structural marker test carries a working negative guard; the blank-default render now has a real raw-shape assertion that bites on `auto`/collapse/dropped-delimiter; the end-to-end `run("setup")` test captures stdout/stderr separately so an argument swap fails. No real `git`/`claude`/editor is spawned. Minor test-balance and guard-tightening suggestions are non-blocking (see Recommendations).

### Required Changes (if any)

None — no blocking issues.

## Recommendations

### Do now

1. Doc staleness — historical pointers in non-code docs
   - `.workflows/interactive-mint-init-setup/planning/interactive-mint-init-setup/phase-1-tasks.md:231-232` — task names `metadata_drift_test.go` as the pins' home, but the pins live in `metadata_test.go`; correct if the plan doc is still maintained (Report 1-5)
   - `internal/configtest/configtest.go:6-17` — package doc references now-removed predecessor names (`rowKey + rowSet`, `rowKey + rowByLevelKey`); mirror `metadata_census_test.go`'s past-tense "previously lived inline" framing (Report 6-3)

### Quick-fixes

2. `internal/config` metadata tests — tighten guards
   - `metadata_drift_test.go:250-256` — add a focused test for the `tomlTag` empty/`-` skip guard so the defensive guard is proven, not just implied (Report 1-3)
   - `metadata_bijection_test.go:305` — remove the redundant `dropped := dropped` loop-var shadow (dead since go 1.22; `copyloopvar` would flag it) (Report 1-4)
   - `metadata_test.go:401-403` — drop or comment the three shared-constant table rows that duplicate the dedicated `*EqualsConfigConstant` pins (Report 1-5)
3. `internal/setupguide` — close marker / markdown-table contract gaps
   - `setupguide_test.go:104` — assert each section marker appears EXACTLY once in `Guide()` (a duplicated/copy-pasted marker currently passes) (Report 2-1)
   - `setupguide.go:337` — `row.Description` is written into a markdown cell unescaped; add a guard test that no `MetadataRow.Description` contains `|` so the four-column contract is enforced at the SoT boundary (Report 2-2)
   - `setupguide_test.go:462` — fold or retitle `TestGuide_ConfigReferenceRendersLevelViaString`, a strict subset of the per-row test (Report 2-2)
4. `cmd/mint/usage_test.go` — add a `run([]string{"setup","--nope"})` assertion that the full dispatch returns the usage exit code and writes the diagnostic to stderr (proven at the runner level, not yet through the `run()` switch) (Report 2-3)

### Ideas

5. `internal/setupguide/setupguide.go` — design calls
   - `:158` — the pipeline/stage prose is flagged drift-sensitive but has no automated tie to the engine's stage ordering; decide whether a lightweight stage-token-in-order pin is worth adding (Report 2-1)
   - `:355` — `LevelCell` is exported solely for cross-package test single-sourcing (no production caller); decide keep-as-documented-render-seam vs. unexport via `configtest`/internal test (Reports 2-2, 5-1)
6. `internal/setupguide/setupguide_test.go` — test-shape judgement calls
   - `:72-83` — `blankDefaultCellRenders` hard-codes a six-field split that silently depends on no description containing `|`; make the dependency explicit or split to the four logical columns (Report 5-2)
   - `:213-221` — `firstNumberedStep`/`numberedStep` overlap; could single-source step extraction (Report 6-4)
   - add a direct `LevelCell(LevelCommit+1)` assertion to pin the out-of-range sentinel at the render seam (currently proven only transitively via `String()`) (Report 6-5)
7. `internal/config` metadata triangulation — value-coverage / maintenance notes
   - `metadata.go:78-91` — consider an exhaustiveness lint/test so a newly-added level missing a `String()` case fails at build time, not only via the drift test (Report 1-1)
   - `metadata_census_test.go:26-56` — triangulation means a schema change edits two hand lists (census + `MetadataRows`); the bijection still bites if only one is updated (confirmed) (Report 1-1)
   - `metadata_bijection_test.go:97-127` — consider a single positive "no divergence" guard for a clearer pass narrative; the `wantPairs = 25` sanity pin is fine as test-only scope (Report 1-4)
   - `metadata_drift_test.go:250-256` — `tomlTag` doesn't split `name,omitempty`-style options; correct today, add the split in this one seam when a tag option first appears (Report 6-2)
   - `metadata_test.go:337` — shared-scalar pins source both cell and `want` from the constant, so a simultaneous co-edit of both wouldn't be caught; inherent to all three pins, optional literal anchor if stronger value coverage is ever wanted (Report 7-1)
8. `internal/initgen` docs-URL pointer — the header URL is the repo root (`github.com/leeovery/mint`), not a dedicated docs path; tests pin presence not the exact token, so align with the Phase 4 canonical docs URL if one is settled (Report 3-1: `initgen.go:43`, `initgen_test.go:38`)
9. `cmd/mint/usage_test.go:213` — `TestRootUsage_CarriesNoConfigReference` forbids a hand-maintained four-key list; consider driving the forbidden-key set from `config.MetadataRows()` keys so it tracks the schema (low value — the marker check is the primary guard) (Report 2-4)
