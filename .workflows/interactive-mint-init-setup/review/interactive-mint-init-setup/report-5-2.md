TASK: Phase 5 / interactive-mint-init-setup-5-2 — "Give the blank-default render a real assertion"

ACCEPTANCE CRITERIA (from analysis-tasks-c1.md Task 2 + planning row 118):
- A regression that renders "auto" (or any non-blank token) for a blank-default key (e.g. [release].context) causes the test suite to fail.
- A regression that drops the cell delimiter or collapses the blank cell so the row is malformed causes the test suite to fail.
- The blank-default assertion no longer reduces to strings.Contains(line, "").
- Non-blank-default rows remain driven by row.Default and still pass.
- Pin defaultCell("") == " " (single space) and defaultCell(x) == x.
- Test-coverage fix only — no production behaviour change.
- All gates pass.

STATUS: Complete

SPEC CONTEXT:
The config-reference table (mint setup guide) renders one row per config.MetadataRows() SoT row. The default column carries the SoT token VERBATIM under a load-bearing representation convention: empty-string default -> blank cell; sentinel auto -> "auto"; empty collection -> "[]"; per-verb inherit -> "shared". defaultCell() (setupguide.go:366-371) turns an empty default into a single space so the markdown cell stays well-formed without altering the SoT token. The blank-vs-auto-vs-shared distinction is what lets the reading agent judge "is the default fine here?" (minimalism guidance, setupguide.go:230-249). The SoT side pins the auto-vs-blank split hard (config metadata_test); the render seam — where it reaches the agent — previously had no real coverage for the blank case. [release].context (metadata.go:144) carries NO Default field, so row.Default == "" — the canonical genuine blank-default row.

IMPLEMENTATION:
- Status: Implemented (test-only remediation; production unchanged — verified)
- Location:
  - Remediation commit 6534bfb touched ONLY internal/setupguide/setupguide_test.go (+ .tick / manifest bookkeeping). `git show 6534bfb --name-only` shows no production file. Confirms criterion (d) "no production behaviour change".
  - The vacuous assertion was in containsRowLine: the pre-remediation body did `strings.Contains(line, defaultCell)` with defaultCell == "" for the blank key — unconditionally true. The diff replaces it with a guard that routes a blank default (defaultDefault == "") through blankDefaultCellRenders and keeps non-blank defaults on the verbatim row.Default Contains path (setupguide_test.go:39-57).
  - New helper blankDefaultCellRenders (setupguide_test.go:72-83): splits the row on "|" into the fixed six fields, requires len==6, and asserts fields[3] == "   " (three raw spaces = " " separator + defaultCell("") single space + " " separator).
  - New dedicated test TestGuide_ConfigReferenceBlankDefaultRendersSingleSpaceCell (setupguide_test.go:425-454): locates [release].context by exact Key cell via rowLineFor, fatals if row.Default != "" (self-guards the chosen blank key), asserts level+description present, then asserts blankDefaultCellRenders(line).
- Notes: production defaultCell is unexported and unreachable from the external setupguide_test package, so the "defaultCell unit test" listed as Do step 1 was correctly replaced by the render-seam shape assertion — explicitly offered as the alternative in the analysis ("If defaultCell is unexported ... assert the rendered-line shape instead"). The single-space contract is still pinned exactly, just indirectly: the blankDefaultField = "   " constant equals separator + the single space + separator, so a defaultCell that returned "" (no space) yields fields[3] == "  " (2 spaces) and fails. defaultCell(x) == x for non-empty x is exercised by the verbatim-token path (TestGuide_ConfigReferenceCarriesDefaultTokensVerbatim, setupguide_test.go:383-413) — non-blank defaults still go through strings.Contains(line, row.Default).

TESTS:
- Status: Adequate
- Coverage / regression-bite verification (traced against the live renderConfigReference delimiter, setupguide.go:330-338 — leading "| ", " | " between cells, trailing " |"; descriptions carry zero literal "|", confirmed via grep, so the 6-field split is robust):
  - "auto" (or any non-blank token) for the blank key: fields[3] == " auto " != "   " -> RED. Criterion 1 met.
  - Truly-empty cell, defaultCell returns "" : builder emits " | " + "" + " | " -> fields[3] == "  " (2 spaces) != "   " -> RED.
  - Collapsed/dropped delimiter (e.g. "||"): len(fields) != 6 -> RED. Criterion 2 met.
  - Correct single-space cell: fields[3] == "   ", len==6 -> green.
- The assertion no longer reduces to Contains(line, "") — both in the dedicated test and in containsRowLine's blank branch (criterion 3 met). containsRowLine still requires key+level+description present before the shape check, so the blank-row presence proof inside TestGuide_ConfigReferenceCarriesDefaultTokensVerbatim and TestGuide_ConfigReferenceHasLinePerSoTRow is now genuine, not vacuous.
- Not over-tested: blankDefaultCellRenders is shared between the row-presence helper and the dedicated seam test (single source for the shape); the dedicated test adds the explicit, named, agent-facing proof plus a self-guard (fatal if the chosen key stops being blank). No redundant duplication.
- Not under-tested: all four regression classes are covered, plus the non-blank verbatim path is untouched and still green (criterion 4 met).
- The test is drift-resistant: it tracks production output (runs through Guide()/renderConfigReference and locates the row by exact Key cell via rowLineFor) rather than re-deriving the table.
- Notes: build + vet pass for the package (go build/go vet ./internal/setupguide). Did not execute the suite (out of scope); adequacy judged by reading.

CODE QUALITY:
- Project conventions: Followed. External test package (setupguide_test), t.Parallel() throughout, behaviour-level proof at the agent-facing seam, exact rendered-line shape asserted (CLAUDE.md test idioms). Heavy WHY-comments on the new helper and test explain the raw-field-vs-trimmed-cell choice and why it bites each regression — true to as-built.
- SOLID / DRY: Good. blankDefaultCellRenders is the single source of the blank-cell shape, reused by both the helper and the dedicated test (no re-implementation).
- Complexity: Low. Helper is a split + len check + one field compare.
- Modern idioms: Yes.
- Readability: Good. The blankDefaultField constant's inline comment documents the "separator + space + separator" derivation; the dedicated test's comments state the seam intent.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/setupguide/setupguide_test.go:72-83 — blankDefaultCellRenders hard-codes the six-field row shape and the "   " field, which silently depends on (a) the fixed "| ... | ... |" delimiter in renderConfigReference and (b) no description ever containing a literal "|". Both hold today (grep confirms zero pipes in metadata.go descriptions). A future SoT row whose description carried a "|" would make len(fields) != 6 and turn this RED with a shape-mismatch message that does not point at the real cause. Consider deciding whether to (a) split into exactly the 4 logical columns by joining the over-split tail, or (b) add a comment/cross-link asserting the no-pipe-in-descriptions invariant so the dependency is explicit. Pure judgement call; current behaviour is correct and safe.
