TASK: Change Map condenses by folder over its share (token-aware-diff-sizing-2-3)

ACCEPTANCE CRITERIA:
- Given a release whose Change Map takes at most a tenth of the cap, the map every call carries is byte-identical to the as-built rendering (§6.3, §8.2)
- Given a release that removes 4,312 files under `vendor/`, whose map would take more than a tenth of the cap, the map carries `Removed: 4,312 files under vendor/` in place of those 4,312 `Removed:` lines (§8.2)
- Given a long run of renamed files under one folder in a map over its share, that run collapses to one summary line for that folder (§8.2)
- Given a map over its share with runs of different lengths, the largest collapse first and condensing stops once the map is under its share (4,312-file `vendor/` run vs 12-file `docs/` run: `docs/` stays listed) (§6.3)
- Given the same release on two runs, the condensed map is byte-identical both times (§6.3)
- Given `max_prompt_bytes` set, the share is a tenth of the configured cap (§6.3)
- Given a release whose single-pass prompt is under the cap and whose map is over its share, the single-pass call carries the condensed map (§8.2)
- Given `mint release regenerate <version>` from a fresh source on a version whose map is over its share, its call carries the map condensed the same way (§8.2)

STATUS: complete

SPEC CONTEXT: §6.3 sets one condensing rule shared by the Change Map and the exclusion-check list: condense once the text takes more than a tenth of max_prompt_bytes (150,000 bytes at the default), replacing long runs under one folder with a summary line, largest runs first, until under the share; deterministic; a text under its share renders as today. §8.2: every call that carries the map (single-pass, chunk, merge) gets the same whole-release map, condensed when over its share, e.g. `Removed: 4,312 files under vendor/`. §8.7 / task 2-9 own the case where even the condensed map leaves no room.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/condense.go:10 (condenseShare = cap/10), :55 (condenseRuns), :85 (rankRuns: count desc, then kind, then folder), :122 (lineOverhead)
  - internal/notes/changemap.go:36,48 (BuildChangeMap / BuildChangeMapForRange take maxPromptBytes), :175-186 (renderChangeMap: full render returned when len <= share, else novelty runs condensed), :202-217 (noveltyLines keys Removed runs by path folder, Renamed runs by destination folder), :219-221 (summary `{kind}: {n} files under {folder}/`)
  - internal/notes/generate.go:215 (the one production call site, passing cfg.MaxPromptBytes; the same changeMap feeds single-pass, generateChunked, generateHalved and the kept MergeInputs used by Remerge)
- Notes: Under-share path renders byte-identically to the pre-change renderer (same section order, same line order, same indentation). Byte accounting in rankRuns is exact: each novelty line contributes len(text) + 2-byte indent + 1 newline, so after saving at least len(full)-share bytes the map is at most the share. Integer division of the share matches "more than a tenth" exactly for integer lengths. Ordering is a total order over unique run keys and output iterates slices, so the condensed map is deterministic. Forward release and regenerate's fresh source both reach generateFromDiffWithContext, so both carry the condensed map. Leading-directory runs and destination-folder keying for renames are implementer's choices the spec leaves open.

TESTS:
- Status: Adequate
- Coverage:
  - AC1: TestBuildChangeMap_UnderShare_RendersAsBuilt pins the exact as-built rendering with renames/removals present (internal/notes/changemap_condense_test.go:62); the pre-existing changemap_test.go exact-output tests now run at the default cap.
  - AC2: TestBuildChangeMap_OverShare_CollapsesVendorRemovalToOneLine (:93), with a fixture guard proving the full map is over the share.
  - AC3: TestBuildChangeMap_OverShare_CollapsesRenameRunToOneLine (:113), exact output.
  - AC4: TestBuildChangeMap_OverShare_CollapsesLargestRunFirstAndStopsUnderShare (:135) would fail on reversed ordering or collapse-everything; SummaryTakesTheRunsPlace (:160) pins placement and stop-after-one.
  - AC5: TestBuildChangeMap_OverShare_TiedRunsCondenseIdentically (:194) pins the tie-break and compares two runs.
  - AC6: TestBuildChangeMap_ConfiguredCap_SetsTheShare (:231) and TestGenerator_Generate_ConfiguredCapSetsTheMapsShare (:321, proves cfg plumbing).
  - AC7: TestGenerator_Generate_SinglePassCarriesCondensedMap (:278).
  - AC8: TestRegenerateFreshBody_OverShareMapIsCondensed (internal/engine/regenerate_fresh_test.go:581).
  - Edges: map still over share after condensing collapses every run (:250); single-entry folder never collapses (:304).
- Notes: Each test targets a distinct criterion or edge; no redundant assertions.

CODE QUALITY:
- Project conventions: Followed (external test package, t.Parallel, FakeRunner scripting, exact rendered output assertions)
- SOLID principles: Good (condensing is a generic line-run helper parameterised by a summary func, reused by the exclusion check)
- Complexity: Low
- Modern idioms: Yes (range-over-int)
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
