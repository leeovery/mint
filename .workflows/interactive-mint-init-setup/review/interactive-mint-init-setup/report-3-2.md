TASK: interactive-mint-init-setup-3-2 — Remove the now-subsumed scaffold value-drift pins and sever the config import from the initgen test package

ACCEPTANCE CRITERIA:
- [x] `TestMintTOML_AICommandValueEqualsConfigConstant` and `TestMintTOML_TimeoutValueEqualsConfigConstant` removed from `initgen_test.go`.
- [x] `initgen_test.go` import block no longer imports `mint/internal/config`, `time`, or `strconv`.
- [x] No orphaned helper remains — every surviving helper is referenced by a surviving test.
- [x] `internal/initgen` (production + test) has no `config` import — the deliberate `initgen`↛`config` seam holds.
- [x] Phase 1 SoT pins (1-5) untouched and remain the carrier of the ai_command/timeout default-value drift discipline — nothing left unpinned.
- [x] `ReleaseShim()` / `shim_test.go` and `engine/init_test.go` unchanged.
- [ ] All standard gates pass — NOT executed here (test execution is out of scope for this verifier; verified by static inspection only).

STATUS: Complete

SPEC CONTEXT:
Spec "`initgen` scope of change" → "The scaffold-value drift-pin moves to the SoT": today initgen's drift tests pin the scaffold's literal default values (ai_command, timeout) equal to config.DefaultAICommand / config.DefaultTimeout. The minimal template (after Task 3-1) carries no default values to pin, so that value-drift discipline is subsumed by the new SoT drift test — the SoT `default` column becomes the drift-pinned carrier. "No default value is left unpinned by the change." CLAUDE.md records that `internal/initgen` is the pure generator and deliberately does NOT import `config`. This task is the Phase 3 half of the Phase-1 hand-off (1-5 ADDED the subsuming SoT pin; the two pins coexisted until Phase 3).

IMPLEMENTATION:
- Status: Implemented (correctly).
- Location:
  - `internal/initgen/initgen_test.go` — final form is 87 lines: package `initgen_test`, imports only `strings`, `testing`, and `mint/internal/initgen` (lines 3-8). Contains the two new minimal-shape tests (`TestMintTOML_BodyHasNoActiveOrCommentedKeys` L16, `TestMintTOML_HeaderCarriesBothPointers` L33) plus helpers `carriesConfigKey` (L51) and `isBareKeyRune` (L77). Both removed pins are gone; `activeTopLevelValue`, `valueAfterEquals`, `isConfigLineForKey`, `looksLikeConfigLine`, and all other commented-template helpers are gone (grep returns nothing).
  - `internal/initgen/initgen.go` — does not import `config` (the only `config` occurrence is the doc comment at L22 stating "initgen does NOT import config"). Minimal `MintTOML()` body (L38-46) references no config constant.
- Verification:
  - (a) Old value-drift pins removed: `grep -rn "AICommandValueEqualsConfigConstant\|TimeoutValueEqualsConfigConstant" internal/initgen/` returns nothing. PASS.
  - (b) Forbidden imports severed: `grep -n "mint/internal/config\|\"time\"\|\"strconv\"\|\"bufio\"\|\"os\"\|path/filepath" internal/initgen/initgen_test.go` returns nothing. The surviving import block (L3-8) is exactly `strings`, `testing`, `mint/internal/initgen` — and the minimal template references no config constant. PASS.
  - (c) No orphaned helper: `carriesConfigKey` referenced at L22; `isBareKeyRune` referenced at L68. No removed-helper names survive anywhere in the package. PASS.
  - (d) Subsumption is real — cross-checked `internal/config`: the SoT metadata default column carries both values, drift-pinned:
      * `internal/config/metadata.go:129` — `{Key: "timeout", Level: LevelShared, Default: strconv.Itoa(int(DefaultTimeout / time.Second)), ...}` — exactly the derivation the removed initgen `time`/`strconv` imports performed.
      * `internal/config/metadata_test.go:304-318` — ties the (shared, ai_command) SoT `Default` cell to `config.DefaultAICommand`.
      * `internal/config/metadata_test.go:344-359` — ties the (shared, timeout) SoT `Default` cell to `int(config.DefaultTimeout / time.Second)`.
      * Canonical constants pinned: `internal/config/config.go:91` `DefaultAICommand = "claude -p --model sonnet"`; `internal/config/config.go:103` `DefaultTimeout = 60 * time.Second`; canonical-value tests at `config_test.go:858` and `config_test.go:1935`.
    Removing the two initgen pins leaves NO default value unpinned — the SoT carrier is present and active. PASS.
  - Scope guard: `git show --stat 59d527c` shows only `internal/initgen/initgen.go` and `internal/initgen/initgen_test.go` changed (plus workflow bookkeeping `.tick/tasks.jsonl`, `manifest.json`). `shim.go`, `shim_test.go`, and `engine/init_test.go` are byte-for-byte untouched. PASS.
- Notes: Tasks 3-1 and 3-2 were committed together in commit 59d527c ("Sequenced 3-1+3-2 in one cycle"), which is the planned-permitted sequencing (the plan's Edge Cases for 3-1 noted the two pins are knowingly left RED at the 3-1 boundary and an executor may sequence 3-1 then 3-2 in one cycle to keep both green). The combined commit means there is no intermediate red state on the mainline; this is consistent with the plan.

TESTS:
- Status: Adequate (for a removal/cleanup increment).
- Coverage: This task carries no new behavioural test by design — its contract is verified by the gates and by the absence of the removed code. The two surviving tests (`TestMintTOML_BodyHasNoActiveOrCommentedKeys`, `TestMintTOML_HeaderCarriesBothPointers`) belong to Task 3-1 and continue to pin the minimal shape; they are unaffected by 3-2's removals and still reference the retained helpers.
- Subsumption coverage is carried entirely by `internal/config` (metadata_test.go drift pins + config_test.go canonical-value pins), which this task correctly did NOT touch or duplicate.
- Notes: No under-testing — the import-severing/pin-removal is fully observable via grep + compile. No over-testing — the task added no redundant test; the SoT carrier is the single owner of the value-drift discipline, avoiding a duplicate pin. The carriesConfigKey predicate (L51-73) correctly distinguishes a commented config line from prose containing `=` or a URL via the strict bare-key check (L67-71), which is the edge case the plan flagged.

CODE QUALITY:
- Project conventions: Followed. External test package (`package initgen_test`), `t.Parallel()` on both tests, no real subprocess/IO. The `initgen`↛`config` seam (CLAUDE.md Layout) is restored — production and test now have zero `config` dependency. Doc comment at initgen.go:22-25 is true-to-as-built (describes the subsumption of the value-drift pins to the SoT).
- SOLID principles: Good. Single-responsibility helpers; the drift discipline now has a single owner (the SoT), not two coexisting carriers.
- Complexity: Low. The test file is 87 lines with two focused tests and two small pure predicates.
- Modern idioms: Yes — `strings`-based predicates, no reflection or external deps.
- Readability: Good. WHY-comments on both tests and both helpers explain the minimal-shape contract and the bare-key guard.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
