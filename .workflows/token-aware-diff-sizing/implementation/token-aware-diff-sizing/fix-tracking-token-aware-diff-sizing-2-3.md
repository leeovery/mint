## Attempt 1

ISSUES:
- `internal/notes/changemap_condense_test.go:194-215`: `TestBuildChangeMap_OverShare_SameInputCondensesIdentically` does not exercise the only source of nondeterminism. Its three 300-entry runs all collapse under the 50,000-byte cap, so the output does not depend on which tied run collapses first. I removed the kind/folder tie-break at `changemap.go:331-334` (leaving `return false`) and every test still passed. On a fixture where ties matter, that mutated code gives different outputs across builds. AC5's test checks nothing that could fail.
  FIX: Replace the fixture with equal-count runs where one collapse is enough. Example: 100 `D aaa/xNNN.go` and 100 `D bbb/yNNN.go`, with the cap set to `(len(full)-1)*10` as at line 175. Assert the exact expected map (`Removed: 100 files under aaa/` with every `bbb/` line still listed), then build it again and compare. The exact-output assertion pins the tie-break rule on every run. A plain two-build comparison catches map-order randomness only about half the time.
  CONFIDENCE: high
- `internal/notes/generate.go:171`: nothing tests that the configured `max_prompt_bytes` reaches the map that calls carry. `TestBuildChangeMap_ConfiguredCap_SetsTheShare` (`changemap_condense_test.go:217`) passes an int straight to `BuildChangeMap`. The single-pass test (`:264`) and the regenerate test (`internal/engine/regenerate_fresh_test.go:536`) both use the default cap. Hard-coding `config.DefaultMaxPromptBytes` at line 171 passes the whole suite. AC6 ("the share is a tenth of the configured cap") is proven only at the unit level, and task 2-4 is about to rework this call site.
  FIX: Add a Generator-level test, next to `TestGenerator_Generate_SinglePassCarriesCondensedMap`, using `removedRun("vendor", 40)` (about 2.9 KB of map, rendered in full at the default) and `cfg.MaxPromptBytes = 20_000` (share 2,000). Assert the prompt carries `\n  Removed: 40 files under vendor/\n` and no `Removed: vendor/`. The default prompt is under 4 KB, so the whole prompt stays under the 20,000 cap and remains single-pass after 2-4's size check.
  ALTERNATIVE: Put the same assertion through `engine.RegenerateFreshBody` with a non-default `cfg.MaxPromptBytes`. That covers the fresh path as well, but it is further from the wiring line. I recommend the Generator-level test.
  CONFIDENCE: high
- `internal/notes/changemap.go:321-325`: `rankRuns` treats a one-entry folder group as a run. It collapses whenever the long path saves bytes, and then renders `Removed: 1 files under vendor/` or `Renamed: 1 files under pkg/`. I reproduced this: two single deep-path removals under a 10-byte cap give exactly those two lines. The spec says a "long run of entries" is condensed. Collapsing one entry drops its file name for a few bytes saved, and "1 files" is wrong text in the prompt. It happens only after every larger run has collapsed and the map is still over its share, but that is exactly the case §8.7 and task 2-9 inherit.
  FIX: In `rankRuns`, skip keys with `count < 2` before building the summary line (for example `if count < 2 { continue }` at the top of the loop at line 321). Add a case to `TestBuildChangeMap_OverShareEvenCondensed_CollapsesEveryRun`, or a sibling test, with a single long-path removal under its own folder at cap 10. Assert that it stays listed as `Removed: <path>`.
  ALTERNATIVE: Keep one-entry collapses and write `1 file` in the singular. This fixes the text but still condenses something that is not a run, against the spec's wording. I recommend the count floor.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- internal/notes/changemap.go:31-33 — "until it is within that share" is false when every run has collapsed and the map is still over its share. The code returns it anyway.
  OLD: // entries and the single largest file). A map that would take more than a tenth of
// maxPromptBytes condenses its removal and rename runs by folder, largest first,
// until it is within that share.
  NEW: // entries and the single largest file). A map that would take more than a tenth of
// maxPromptBytes condenses its removal and rename runs by folder, largest first,
// until it is within that share or no run left would shrink it.
- internal/notes/changemap.go:13-15 — this doc block was touched by the diff. It carries workflow vocabulary and a claim that is false as built: `ComposePrompt` does put the map ahead of the diff.
  OLD: // salience problem, not a missing-data problem. The map is compact METADATA that a
// later task prepends before the (capped) diff to tell the AI what to prioritize;
// it is NOT a restatement of the diff and this task does NOT do the prepend.
  NEW: // salience problem, not a missing-data problem. The map is compact METADATA that
// precedes the diff in the prompt to tell the AI what to prioritize; it is NOT a
// restatement of the diff.
- internal/notes/changemap.go:176-178 — the added sentence restates the function body (`if len(full) <= share { return full }`) and repeats the exported doc.
  OLD: // only when they have content, so a small change set yields a small map. A map over
// its share of maxPromptBytes has its removal and rename runs condensed by folder;
// one within its share renders unchanged.
  NEW: // only when they have content, so a small change set yields a small map.

NOTES:
- Rename runs are grouped by the folder the files moved into, so `legacy/handlers/* -> pkg/handlers/*` condenses to `Renamed: 50 files under pkg/`. The source folder disappears from the condensed map, and renames within `pkg/` mix with moves into it. §6.3 leaves the grouping to the implementer, so this is not an issue. Grouping on (old folder, new folder) would keep the move's shape if the notes lose it in practice.
- Phase 7 task 7-13 must apply "the same rule as the Change Map" with a different summary line (`vendor/: 4,312 files, 38 MB`). `rankRuns`/`condenseRuns` hard-code `noveltyLine` and the `Kind: N files under folder/` format. When 7-13 lands, the ranking, tie-break and stopping logic should be moved into a shared helper with the summary format as a parameter, not copied. A copy could drift from the "one rule" §6.3 sets.
- A zero `MaxPromptBytes` gives share 0, so tests built on `config.Config{}` literals or `freshCfg()` (`regenerate_fresh_test.go:44`) condense every map as far as it goes. No current test is affected. Task 2-4's prompt-size check will run into the same zero-cap fixtures, loudly.
- Boundary check: changing `len(full) <= share` to `<` at `changemap.go:186` makes no difference to behaviour, because excess 0 breaks before anything collapses. The "at most a tenth" edge is safe without a dedicated test.
- The executor's claims hold as stated: savings are exact (the condensed maps landed within the share in every probe), the summary line takes the run's first position, and every gate passes.
- The BuildChangeMapForRange doc (`changemap.go:42-49`) has pre-existing workflow vocabulary ("Phase 5", "5-3's DiffRange"). Only its signature line was touched, so it is outside this task's scope.
- Nothing beyond the enumerated inputs came with the dispatch.
