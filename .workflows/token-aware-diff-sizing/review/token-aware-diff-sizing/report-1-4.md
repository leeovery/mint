TASK: 180-second shipped release timeout (token-aware-diff-sizing-1-4, tick-fd9376)

ACCEPTANCE CRITERIA:
- Given no `timeout` at any level (no `.mint.toml`, or one setting no `timeout`), a hung release-notes call fails as `AI timed out` after 180 seconds, on a forward release and on `mint release regenerate <version>` from a fresh source (§4.4)
- Given the same config, a hung `mint commit` AI call fails after 60 seconds and goes to the editor fallback as built (§4.4)
- `[release].timeout = 90`, or a shared `timeout = 90` with no `[release].timeout`, gives release-notes calls 90s, not 180; a shared `timeout = 90` with no `[commit].timeout` gives `mint commit` 90s too (§4.4)
- `mint setup`'s config reference shows `[release].timeout` with a 180-second default and the shared `timeout` row naming both shipped defaults (§18.2)
- The README's `[release]` table shows `timeout` with a 180-second default, the shared `timeout` row names both, and every README line stating the default names both, including the `default 60` mentions in "The AI Transport" (§18.2)
- `mint setup`'s AI-transport paragraph names the 180s release and 60s commit timeouts where it said `default 60` (§18.1)

STATUS: issues_found

SPEC CONTEXT: §4.4 gives `mint release` its own shipped per-attempt timeout of 180s, resolving `[release].timeout` → shared `timeout` → 180s; regenerate rides on `[release]`; commit keeps 60s. §4.5 grounds it: a call at the 1.5 MB cap should take about a minute, so a 60s floor would time out calls the cap allows. §18.1/§18.2 require the setup guide paragraph, the SoT rows behind `mint setup`'s config reference, and the README to name both defaults. §18.3 requires code comments stating the single 60s default to be rewritten in the same change.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/config/config.go:91-97 — `DefaultReleaseTimeout = 180s`, `DefaultCommitTimeout = 60s` replace the single `DefaultTimeout`
  - internal/config/config.go:642-668 — `TimeoutFor` selects the per-verb override and the per-verb floor together; chain `[verb] → shared → floor` unchanged, as are the explicit-0 / negative value rules
  - internal/config/config.go:289-307 — `defaults()` no longer seeds the shared `Timeout`, so the zero-config path now reaches the per-verb floor instead of being pinned to 60s at the shared level (the trap the task's Context flagged); no production code reads `cfg.Timeout` directly outside `TimeoutFor`, so removing the seed is safe
  - internal/aitransport/aitransport.go:41-46 — sources `cfg.TimeoutFor(verb)`; internal/engine/release.go:1321 and internal/engine/regenerate_fresh.go:152 pass `config.VerbRelease`, internal/commit/run.go:822 passes `config.VerbCommit`
  - internal/config/metadata.go:119, :138, :153 — SoT Default cells: shared `180 on release, 60 on commit`, `[release]` `shared, else 180`, `[commit]` `shared, else 60`, all derived from the constants via `seconds()` (:159)
  - internal/setupguide/setupguide.go:165-166 — AI-transport paragraph reads "default 180 for release, 60 for commit", spliced from the constants
  - README.md:240, :260, :279, :295, :299 — shared row, both per-verb rows, and both "The AI Transport" prose lines name both defaults; no other README line states a timeout default
- Notes: The hang → `AI timed out` path is the existing transport behaviour (context.WithTimeout per attempt in internal/ai/transport.go `attempt`, DeadlineExceeded → `ErrTimeout`, rendered as "AI timed out" at internal/notes/resolve.go:178); the task only changes the deadline value reaching it. The §18.3 comment sites (config.go `Timeout` fields, `defaults()`, `fileShape`, `TimeoutFor`, `resolveTimeout`; internal/ai/transport.go `Config` and `NewTransport`; internal/aitransport/aitransport.go `New`) are all updated and true to as-built.

TESTS:
- Status: Adequate
- Coverage:
  - internal/aitransport/aitransport_test.go `TestNew_AppliesEachVerbsResolvedDeadline` — loads real config files and measures the deadline the runner receives: no file → 180s release / 60s commit; a file with no timeout → same; `[release].timeout = 90` → 90s; shared `timeout = 90` → 90s for both verbs. Covers criteria 1–3 end to end through `config.Load`
  - Per-site wiring guards: internal/engine/release_aitransport_internal_test.go `TestAITransport_ZeroConfigAppliesShippedReleaseDeadline`, internal/engine/regenerate_fresh_aitransport_internal_test.go `TestResolveFreshTransport_ZeroConfigAppliesShippedReleaseDeadline`, internal/commit/run_aitransport_internal_test.go `TestCommitTransport_ZeroConfigAppliesShippedCommitDeadline` — each would fail if a site passed the wrong verb
  - internal/config/config_test.go — `TestDefaultTimeouts_ShippedPerVerbValues`, `TestLoad_AbsentFile_LeavesSharedTimeoutUnset`, and the existing TimeoutFor tests re-pointed at a per-verb `shippedTimeout(verb)` expectation (negative drops, absent-falls-through, command-override independence, release-override-does-not-bleed)
  - internal/config/metadata_test.go — shared row pinned to "180 on release, 60 on commit"; per-verb rows pinned to "shared, else 180/60"
  - internal/setupguide/setupguide_test.go — `TestGuide_AITransportParagraphNamesBothShippedTimeouts` (and asserts no `default 60;` remains), `TestGuide_ConfigReferenceNamesShippedTimeouts`
  - internal/config/readme_tripwire_test.go `TestREADME_TimeoutDefaultsNameBothVerbs` — positive strings for every row and the prose line, plus stale-phrase negatives
- Notes: The 5-second tolerance on `DeadlineRemaining` is measured against `time.Until` immediately after `WithTimeout`, so it is not flaky under `-race`. No over-testing: the aitransport table proves resolution; the three site tests only prove each site's verb choice.

CODE QUALITY:
- Project conventions: Followed — external test packages, table-driven, `t.Parallel()`, `t.TempDir()`; config remains the single owner of the default; transport stays config-agnostic
- SOLID principles: Good
- Complexity: Low — `TimeoutFor` gains one paired assignment
- Modern idioms: Yes
- Readability: Good
- Issues: One stale comment in a file this task edited (see FINDINGS)

BLOCKING ISSUES:
- None

FINDINGS:
- [in-scope] [contained] internal/ai/transport_test.go:19 — the `generousTimeout` comment still says tests exist "so tests never wait the production ~60s". This task rewrote the identical claim in internal/ai/transport.go:128 to "never wait out a production deadline" and edited this same test file (line 516), but missed this line. Replace "never wait the production ~60s." with "never wait out a production deadline." — FAILS: the comment states a single 60s production deadline, which the code now falsifies (release-notes calls get 180s); it is a single-60s-default comment of the kind §18.3 and CLAUDE.md's "true to as-built" rule require rewritten. Comment-only remedy, non-blocking.

UNSETTLED:
- None
