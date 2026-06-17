TASK: interactive-mint-init-setup-2-4 — Thread `mint setup` through the curated-help surface and extend the coverage test

ACCEPTANCE CRITERIA:
1. `rootUsage` lists `setup` with a curated one-line description stating the command PRINTS/EMITS the guide (not that it writes files), column-aligned to the existing entries.
2. A curated `setupUsage` const exists, opens with the `usage: mint setup` synopsis line, and notes no flags beyond `--help` and runs-anywhere/no-repo.
3. `mint setup --help` prints `setupUsage` to stdout and exits 0 (pinned via the extended `TestRunVerb_Help_ExitsZero` and `TestParseFlags_HelpSurfacesErrHelp`).
4. `TestUsageTexts_CoverTheirFlagSets` is extended: a setup row (synopsis pinned) and `setup` added to the `rootUsage`-command coverage slice, so dropping the `setup` line fails the test.
5. `mint help`/`rootUsage` carries NO config reference and is otherwise the frozen curated text plus the one `setup` line; a guard test pins the no-config-reference fact.
6. All standard gates pass.

STATUS: Complete

SPEC CONTEXT:
Spec section "The `mint setup` subcommand" → "Help-surface wiring (the curated-help contract)" decides: a `rootUsage` command-list line for `setup` with the curated description "print the AI-assisted setup guide for configuring mint" (states it PRINTS/EMITS, not writes files); a curated `setupUsage` for `mint setup --help` (stdout, exit 0, via `flag.ErrHelp`; notes no flags beyond `--help` and runs anywhere/no repo); dispatch + the help-contract coverage test extended to pin `mint setup`. Spec "Render targets and layering" decides `mint help` → curated text, NO config reference (the agent gets config from `mint setup`); "Definition of done" → "Help-contract coverage test — the existing usage-coverage test extended to pin `mint setup` (rootUsage line + curated `setupUsage`)." `mint help` stays frozen, NOT retrofitted into a dynamic renderer.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - `cmd/mint/usage.go:22` — `setup` line added to `rootUsage` command list: "print the AI-assisted setup guide for configuring mint" (spec's exact wording). Verified column-aligned: descriptions all start at the same column as the existing entries (checked with visible-whitespace dump; `release regenerate` is the widest label and the gutter is consistent).
  - `cmd/mint/usage.go:87-96` — `setupUsage` const, mirrors `initUsage`'s synopsis-first shape; doc comment in the heavy-WHY register. Opens with `usage: mint setup [--help]`; body states "Takes no flags beyond --help and runs anywhere — no git repository required," and that it prints the guide "to stdout for an AI agent to follow." Both spec facts (no-flags-beyond-help, runs-anywhere/no-repo) are present.
- Notes:
  - AC2 ("notes no flags beyond `--help`"): the synopsis carries the literal `--help` token, which satisfies the coverage test's `{"setup", setupUsage, []string{"--help"}}` row, and the body states it in prose too. Good — both the test's substring check and the human-readable contract are covered.
  - AC's "states it PRINTS/EMITS the guide, not writes files": the `rootUsage` line uses "print" and the body uses "Print … to stdout" — no "write"/"scaffold" verbs. The `init` line keeps "scaffold," correctly contrasting the two verbs.
  - Print site (Task 2-3, reconciled here per the task's "Confirm `runSetup` prints `setupUsage`"): `cmd/mint/setup.go:39` prints `setupUsage` on `flag.ErrHelp` to stdout and returns 0; otherwise `setupguide.Guide()` to stdout (`setup.go:46`). The const and print site are consistent.

TESTS:
- Status: Adequate
- Coverage:
  - `cmd/mint/usage_test.go:34` — `parseSetupFlags` added to `TestParseFlags_HelpSurfacesErrHelp` `parsers` map; pins `-h`/`--help` → `flag.ErrHelp` for setup (AC3). Signature `func(a []string) error { return parseSetupFlags(a) }` matches the actual `parseSetupFlags(args []string) error`.
  - `cmd/mint/usage_test.go:57` — `setup` added to `TestRunVerb_Help_ExitsZero` `runs` map: `runSetup([]string{"--help"}, io.Discard, io.Discard)` returns 0 (AC3). Signature matches the actual `runSetup(args []string, stdout, stderr io.Writer) int`.
  - `cmd/mint/usage_test.go:160` — `{"setup", setupUsage, []string{"--help"}}` row added to `TestUsageTexts_CoverTheirFlagSets`; the `HasPrefix(tc.usage, "usage: mint ")` synopsis assertion (line 167) now bites on setup (AC4 synopsis pin).
  - `cmd/mint/usage_test.go:171` — `"setup"` added to the `rootUsage`-command coverage slice; a dropped `setup` line fails the test (AC4 rootUsage pin).
  - `cmd/mint/usage_test.go:181-200` — new `TestRootUsage_SetupDescriptionPrintsNotWrites`: isolates the setup line, requires "print"/"emit", forbids "write"/"scaffold". Directly pins the spec distinction (matches the planned test `"it states the rootUsage setup description prints/emits the guide, not writes files"`).
  - `cmd/mint/usage_test.go:207-218` — new `TestRootUsage_CarriesNoConfigReference`: asserts `rootUsage` carries neither `setupguide.MarkerConfigReference` nor any of four known config keys (`diff_exclude`, `tag_prefix`, `ai_command`, `max_diff_lines`). Pins the "mint help stays frozen, no config reference" guard (AC5; planned test `"it keeps mint help free of any config reference"`). Verified `MarkerConfigReference` exists and is exported at `internal/setupguide/setupguide.go:59`.
- Notes:
  - Not under-tested: every planned test for 2-4 is present, and each maps to an acceptance criterion. The two new guard tests pin the two spec-load-bearing facts (prints-not-writes; no-config-reference) that a plain coverage table would not catch.
  - Not over-tested: assertions are focused. `TestRootUsage_CarriesNoConfigReference` checks four representative keys rather than the full schema — proportionate for a guard (it is a tripwire against retrofitting the config table, not a completeness proof; the SoT drift test owns completeness). No redundant happy-path duplication.
  - The `parsers`/`runs` maps are non-deterministically ordered, but each entry is asserted independently (no cross-entry coupling), so map iteration order is immaterial — correct table-driven style.
  - `TestRun_TopLevelHelp_ExitsZero` was (per the task's "author's discretion") not extended with a `mint setup` row — acceptable: `TestRunVerb_Help_ExitsZero` already covers `mint setup --help` exit-0 via the `runs` map.

CODE QUALITY:
- Project conventions: Followed.
  - CLAUDE.md "Error & exit idioms" / cmd-layer exception: the curated-help/usage text is the documented seam-3 exception; `setupUsage` is a cmd-layer const written to stdout from `cmd/mint`, exactly the existing register. No business-logic output bypasses the presenter.
  - Test idioms: external assertions on exact rendered text and exact exit codes; `t.Parallel()` on the pure tests; table-driven where the shape fits. Conforms to the "assert exact rendered lines … drift is a contract break" rule.
  - The heavy-WHY comment convention is honoured: `setupUsage`'s doc comment (usage.go:87-90) explains the no-flags/runs-anywhere contract and that the cwd-safety lives in the guide, true to as-built.
- SOLID principles: Good. The change is additive and single-responsibility (one const + one command line + test extensions); no coupling introduced.
- Complexity: Low. No new control flow; const + slice/map additions.
- Modern idioms: Yes. Idiomatic Go table tests, `errors.Is(err, flag.ErrHelp)` already in place at the print site.
- Readability: Good. The two new guard tests carry clear intent-stating doc comments tying them to the spec decision.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] cmd/mint/usage_test.go:213 — `TestRootUsage_CarriesNoConfigReference` forbids a hand-maintained list of four config keys; the list never grows with the schema and a future key that is a common English word could false-positive. Decide whether to drive the forbidden-key set from `config.MetadataRows()` keys so the guard tracks the schema. Low value — the marker check (line 210) is the primary guard and the key list is a belt-and-braces sample — so this is a judgement call, not a clear win.
