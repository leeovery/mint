TASK: A failed single-pass `r` call is classified and halved (token-aware-diff-sizing-6-4, tick-c12b39)

ACCEPTANCE CRITERIA:
1. Given single-pass notes and an `r` whose call fails with captured output after the transport's retry, one classify call follows through the release command and timeout, carrying that `r` call's captured output (§9.1, §9.2, §15.1)
2. Given that classify answer is `SIZE`, the AI tool receives chunk calls, each no bigger than half the failed `r` call's composed size, nudge included, then one merge call, and each carries the nudge. The merge's output is shown as the new notes and the review gate asks again (§10.1, §15.1)
3. Given a part of that run, or of an `r` that went to chunking over the cap, whose call is classified SIZE, its slice is re-cut at half that call's composed size, each new call carrying the nudge, and the notes are still merged. A slice of the `r`'s run is halved at most three times, counted from its first SIZE answer, so a fourth SIZE fails the `r` (§10.1, §10.2, §15.1)
4. Given the classify answer is `OTHER`, or an `r` call that times out, which gets no classify call, no chunk call follows and the `r` fails (§9.1, §9.3, §15.1)
5. Given an `r` whose notes were written after halving, the lower-cap hint shows after the re-shown notes and before the review gate asks again. It names the largest single-pass, chunk or piece call that succeeded in the run, the first attempt's calls included, as `max_prompt_bytes = {bytes}` (§10.3, §15.1, §16.3)
6. Given `mint release regenerate <version>` from a fresh source whose `r` call is classified SIZE, the `r` is halved the same way, and the hint shows before regenerate's review gate asks again. Under `regenerate --all` at a terminal without `-y`, a version's `r` halves the same way, shows no hint of its own, and its calls count toward the one hint the batch prints when it ends (§1.3, §10.3, §15.1)

STATUS: complete

SPEC CONTEXT: §15.1 makes a single-pass `r` re-run the AI path from the size check onward. Its call is a release-notes generation call, so the cap and the classifier apply on their own terms, and the nudge rides in the instructions of every chunk, piece, halving-round and merge call (2026-10-19 corrigendum). §9.1/§9.2: only a generation failure that survives the transport's single retry is classified, through the release `ai_command` and timeout; a timeout or missing binary never is. §10.1/§10.2: on SIZE the slice is re-cut at half the failed call's composed size, at most three times per slice counted from its first SIZE answer. §10.3 and its corrigenda: the hint names the exact byte count of the largest succeeded diff-carrying call in the run. It prints after the notes and ahead of the gate on a single-version run, and once at the batch's end under `--all`.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/generate.go:131-137, 167-173, 200-242: `GenerateWithContext` and `GenerateFromRangeWithContext`, the `r` entry points, share `generateFromDiffWithContext` with the first attempt. A single-pass failure goes through `g.generate`, then `readFailure`/`classify`, and a SIZE answer goes to `generateHalved` at `len(prompt)/2`. `prompt` is composed from `appendOneTimeContext(...)`, so the nudge counts in the composed size and rides in every chunk and merge call through `cut.instructions`.
  - internal/notes/chunk.go:41-48 (`generateHalved`, which starts the run at halvings=1), 209-241 (`wave.send`: classify, then recut at `len(prompt)/2`, bounded by `maxHalvings`=3 at chunk.go:112), 82-91 (merge SIZE becomes ErrMergeTooLarge, never halved).
  - internal/notes/classify.go:53-70: only an `*ai.GenerationError` with output is classified, so a timeout or missing binary gets no classify call. The call goes through the generator's transport under `cfg.MaxPromptBytes`.
  - Release command and timeout: release.go:1017-1018 builds the one generator over `aiTransport(deps, cfg)` (`aitransport.New(..., config.VerbRelease)`, release.go:1317-1322), and the `r` reuses that generator through `perRunRegenerator` (release.go:1340-1350) and `RegenRun.regenerator` (chunknotes.go:115-125). Regenerate goes through `freshGenerator`/`resolveFreshTransport` with VerbRelease (regenerate_fresh.go:98-153), wired in production with a nil transport (cmd/mint/regenerate_all.go:131-138, cmd/mint/regenerate_run.go:100-105).
  - Hint: `regenStage` (internal/engine/notesstage.go:159-235) folds each `r` call into the run's `notesStage.largestCall` through `CallSucceeded`, so the first attempt's calls count. It records `halved`/`parts` per `r`. `shown()` hands `suggestedCap(...)` to the hint and resets the per-`r` state. `regenerateBody` calls `regen.shown()` straight after `ShowNotes` (release.go:1888-1893), before the gate loop re-prompts.
  - Single-version regenerate: regenerate_interactive.go:255-266 passes `regens` and `LowerCap`, and `reviewFresh` wires them (regenerate_write.go:250-257).
  - `--all`: regenerate_batch.go:336-338 builds `regens` with `hint.regenWritten` (no per-version hint, and `LowerCap` is unset in `gatePerVersion`). `hint.tried(stage)` folds the `r` calls in after the gate. `RegenerateAll` prints the one hint at the end and on every error exit (regenerate_batch.go:203-205, 218).
- Notes: The halving bound counts the single-pass SIZE as the first halving (writeParts halvings=1), so a part of a halved `r` fails on the fourth SIZE overall. That matches "counted from its first SIZE answer". Per-`r` halving state is reset on each successful `r` in `shown()`, and a failed `r` ends the run, so no stale state carries into a later `r`.

TESTS:
- Status: Adequate
- Coverage (internal/engine/regen_halve_test.go):
  - AC1: `TestRelease_RegenCallFailsWithOutput_ClassifyRunsTheReleaseCommandAndTimeoutOnTheRegensOutput` (223) goes through the real `aitransport` over a deadline-recording runner. It covers both of the `r`'s attempts (the retry), then exactly one classify call on `tool-a` within about 30s, carrying the `r`'s output.
  - AC2: `TestRelease_RegenClassifiedSize_HalvesAtHalfTheNudgedCallAndShowsTheMergeAtTheGateAgain` (268), with `assertRegenHalved` (148): every chunk call is at most half the failed nudged call and carries the nudge, there is one nudged merge, the merge output is shown, the gate asks twice, and the merge output reaches the tag.
  - AC3: `TestRelease_RegensHalvedPartClassifiedSize_...` (290), `TestRelease_RegenOverTheCapWhosePartIsClassifiedSize_...` (309), and the bound test `TestRelease_RegensSliceClassifiedSizeAFourthTime_FailsTheRegen` (328): four overflowed calls, ErrTooLargeForModel, no merge.
  - AC4: table `TestRelease_RegenThatCannotBeHalved_FailsWithNoChunkCall` (350). The timeout case answers classify with SIZE, so a stray classify call would surface as chunk calls.
  - AC5: `TestRelease_RegenWrittenAfterHalving_HintsTheLargestSucceededCallOfTheRun...` (399), where the first attempt's call is the largest; `...AfterItsPartHalved_...` (415); `TestRelease_RegenNotHalved_NoHint` (431); `TestRelease_ReMergeAfterARegenThatHalved_HintsOnlyAfterTheHalvedRegen` (460), which covers the per-`r` reset; and `...HintNamesAnEarlierRegensCallWhenItIsTheLargest` (575). `assertLowerCapHint` pins the exact Warning text and that the previous event is ShowNotes and the next is the gate's Prompt.
  - AC6: `TestRegenerateRun_RegenClassifiedSize_HalvesAndHintsBeforeTheGateAsksAgain` (492), `TestRegenerateAll_RegenClassifiedSize_HalvesWithNoHintOfItsOwnAndCountsTowardTheBatchEndHint` (536), and `TestRegenerateAll_..._BatchEndHintNamesAnEarlierRegensCallWhenItIsTheLargest` (590).
- Notes: Each test would fail if its behaviour broke. With halving gone, the `r` aborts and `t.Fatalf` fires. With the wrong target, the half-size assertions fail. Without the nudge, the prefix assertions fail. Without the hint plumbing, the hint count or position assertions fail. A missing binary on an `r` is not exercised here, but it is not in the ACs and takes the same `classify` branch the Phase 4 tests cover. The tests are not over-tested: each covers a distinct criterion or edge.

CODE QUALITY:
- Project conventions: Followed. External test package, `t.Parallel()` throughout, exact presenter Warning asserted, the AI goes through the consumer-defined `notes.Transport`, production transports are built only through `aitransport.New`, and output goes only through the presenter.
- SOLID principles: Good. `regenStage` is a narrow `notes.Progress` adapter. The `r` rule lives once in `RegenRun.regenerator` and is shared by release, single-version regenerate and `--all`.
- Complexity: Low
- Modern idioms: Yes
- Readability: Good. The comments on `regenStage`, `RegenRun`, `batchLowerCap.regenWritten`, `RegenerateAll` and `Generator.Generate` step 4 hold against the code.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
