TASK: Phase 7 / interactive-mint-init-setup-7-1 — Pin the max_diff_lines SoT default to its canonical constant (close the last unpinned shared scalar default)

ACCEPTANCE CRITERIA:
- DefaultMaxDiffLines is an exported constant in internal/config/config.go; no reference to the old unexported defaultMaxDiffLines name remains anywhere in the package.
- The max_diff_lines SoT row in metadata.go sources its Default from strconv.Itoa(DefaultMaxDiffLines), not a re-typed literal.
- A dedicated test pins the (shared, max_diff_lines) SoT Default cell to config.DefaultMaxDiffLines and fails the build if they drift, symmetric with the existing ai_command/timeout pins.
- The value-render test derives the timeout-seconds want as int(config.DefaultTimeout / time.Second) — the single spelling used by both production and the dedicated timeout pin — with no remaining .Seconds() truncation spelling.
- The WHY-comments in metadata.go and the affected test doc comments are updated to be true to as-built (all three shared scalars sourced+pinned).
- All gates pass.

STATUS: Complete

SPEC CONTEXT:
The specification's "initgen scope of change" section (specification.md:71) is explicit: "the SoT `default` column becomes the drift-pinned carrier of those values. No default value is left unpinned by the change." This task makes that invariant TOTAL across all three shared scalar defaults (ai_command, timeout, max_diff_lines), where previously max_diff_lines was the lone exception — its default living in an unexported const, with the SoT cell and SoT value-test each carrying an uncoupled re-typed literal "50000".

IMPLEMENTATION:
- Status: Implemented (matches the plan exactly, drift-free)
- Location:
  - internal/config/config.go:64-72 — const renamed/exported to DefaultMaxDiffLines = 50000; doc comment (lines 64-71) updated to state it is EXPORTED so other sites derive the value, symmetric with DefaultAICommand / DefaultTimeout.
  - internal/config/config.go:313 — defaults() now seeds MaxDiffLines: DefaultMaxDiffLines.
  - internal/config/config.go:645,649 — resolveMaxDiffLines returns DefaultMaxDiffLines.
  - internal/config/metadata.go:128 — max_diff_lines SoT row now Default: strconv.Itoa(DefaultMaxDiffLines) (was the literal "50000"). strconv already imported.
  - internal/config/metadata.go:122-126 — WHY-comment updated: now names all three shared scalar defaults (ai_command, timeout, max_diff_lines) as constant-sourced and pinned against DefaultAICommand / DefaultTimeout / DefaultMaxDiffLines. True to as-built.
- Verification of "no old-name references remain": grep -rn "defaultMaxDiffLines" internal/config/ returns ZERO hits (only the exported DefaultMaxDiffLines appears). Confirmed.
- README.md:205 left as-is: | `max_diff_lines` | `50000` | ... | — human GitHub-browsing surface, value unchanged, spec-sanctioned light duplication. Confirmed unchanged.
- Value-preserving: DefaultMaxDiffLines = 50000 and DefaultTimeout = 60s unchanged. Confirmed.
- Notes: None. The change is a pure single-source consolidation; no behaviour or rendered value changes.

TESTS:
- Status: Adequate
- Coverage:
  - New dedicated pin TestMetadataRows_SharedMaxDiffLinesDefaultEqualsConfigConstant (metadata_test.go:329-340): fetches (LevelShared, "max_diff_lines") via configtest.MustByLevelKey, t.Fatal if missing, asserts row.Default == strconv.Itoa(config.DefaultMaxDiffLines). Modelled exactly on the ai_command pin (309-320) and timeout pin (349-361). WHY doc comment (322-328) names it the subsuming pin and explains the drift-guard role. This is the structural twin of the existing two pins.
  - Drift-guard-by-construction holds: because both the SoT cell (metadata.go:128) and the test want (metadata_test.go:337-338) are sourced from strconv.Itoa(DefaultMaxDiffLines), the test cannot pass against a re-typed divergent literal in the cell — a hand-mutation of the cell to e.g. "50001" makes row.Default != strconv.Itoa(config.DefaultMaxDiffLines) and fails. (Note this is symmetric with the ai_command/timeout pins and is the strongest form available given both sides reference the constant; see non-blocking note.)
  - TestMetadataRows_ConcreteScalarDefaultsRenderVerbatim (metadata_test.go:393-420): max_diff_lines want updated to strconv.Itoa(config.DefaultMaxDiffLines) (line 403); timeout want normalised to strconv.Itoa(int(config.DefaultTimeout / time.Second)) (line 402) — no .Seconds() truncation spelling. Doc comment (384-392) updated true to as-built (all three shared scalars derived from exported constants, timeout-seconds uses the single production spelling).
  - .Seconds() spelling: grep "\.Seconds()" across internal/config/ returns ZERO hits. The divergent float64-truncation spelling is fully removed from the package; the one production spelling int(DefaultTimeout / time.Second) is used by metadata.go:129, the dedicated timeout pin (357), and the value-render test (402).
  - Bijection test premise confirmed: metadata_bijection_test.go projects MetadataRows() to leafKey{Level, Key} (sotLeafKeys, lines 84-91) and metadata_drift_test.go derives schema leaf keys from toml tags — NEITHER inspects the Default column. So the default VALUE was genuinely uncovered before this pin; the new dedicated pin is what now guards it. The bijection's key-presence coverage is unaffected (max_diff_lines pair still present in expectedLeafKeys census, metadata_census_test.go:29).
- Notes: Not over-tested — the new pin is the minimal symmetric guard, not redundant with the bijection (different concern: value vs key-presence) nor with ConcreteScalarDefaultsRenderVerbatim (the latter is a broad render-convention sweep; the dedicated pin is the named single-purpose drift carrier, consistent with the existing ai_command/timeout pins). Not under-tested — the value drift is now guarded, and the existing setup-guide render tests remain green by value-preservation.

CODE QUALITY:
- Project conventions: Followed. External test package (config_test) for the value/pin tests; internal package only where unexported reflection is required (drift/bijection/census), exactly per the documented placement decision. t.Parallel() throughout. Heavy true-to-as-built WHY-comments maintained on the const, the SoT comment block, the new pin, and the amended render-test comment — consistent with CLAUDE.md's comment discipline.
- SOLID principles: Good. Single canonical source for the default value; the SoT cell and tests now depend on the exported constant (dependency-inversion-style single source) rather than re-typed literals.
- Complexity: Low. Mechanical rename + sourcing change + one new table-shaped test.
- Modern idioms: Yes. strconv.Itoa over the constant; consistent with the sibling rows.
- Readability: Good. The new pin reads identically to its two siblings, so the "all three shared scalars are pinned" pattern is now visually uniform.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/config/metadata_test.go:337 and metadata.go:128 — Because BOTH the SoT cell and the dedicated pin's want are sourced from strconv.Itoa(DefaultMaxDiffLines), the pin proves "the cell is sourced from the constant" but cannot independently catch a hand-typed divergent literal swapped into BOTH places at once (an unlikely but theoretically possible co-edit). This is an inherent property of all three shared-scalar pins (ai_command/timeout share it), so it is consistent-by-design, not a regression. If stronger value coverage is ever wanted, decide whether a single test should additionally assert the concrete literal (e.g. row.Default == "50000") to anchor the constant itself — a design call, not a defect, and explicitly out of scope for this value-preserving remediation.
