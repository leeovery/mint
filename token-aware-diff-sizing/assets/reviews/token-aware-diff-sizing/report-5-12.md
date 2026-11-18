TASK: A `regenerate --all` skip names its real cause (token-aware-diff-sizing-5-12, tick-165339)

ACCEPTANCE CRITERIA:
- Given `regenerate --all --source fresh` where v1.1.0's single-pass call fails with `Not logged in · Please run /login` on stdout and is classified OTHER, v1.1.0's skip warning reads `AI call failed` with that output verbatim beneath it. The end-of-run summary lists v1.1.0 with `AI call failed`, and the other versions are regenerated (§9.6, §12.3)
- Given that batch at a terminal without `-y`, or under `-y`, with `on_notes_failure` unset, `abort` or `fallback`, the version is skipped the same way, and no `c` / `e` / `a` choice shows (§12.3)
- Given a version whose 6-part wave fails for good at part 3, its skip warning reads `AI call failed (part 3 of 6)` with part 3's captured output verbatim beneath it, and its summary entry names the same phrase. Given part 5, already in flight, then fails too, the output beneath is still part 3's (§11.2, §12.3, §12.4)
- Given versions whose notes time out, find no binary, run out of halving, have no room for diff, or whose merge is too large, each skip names its phrase in its warning and in the summary: `AI timed out`, `AI tool not installed`, `too large for the model, even in smaller parts`, `no room for the diff under max_prompt_bytes`, `part notes too large to merge` (§12.3, §12.4)
- No skip warning or summary entry for a notes failure reads `diff too large` (§12.3)

STATUS: issues_found

SPEC CONTEXT: §12.3 keeps skip-and-continue as `--all`'s floor whatever `on_notes_failure` holds; each skip names the §12.4 cause phrase (with its `(part k of n)` / `(merging)` location in a chunked run) in its warning and in the end summary, never `diff too large`, and carries the failed call's captured output verbatim beneath it, as the ✗ line does (§9.6, §17.1). In a chunked run the output is the first final failure's (§11.2): once one part fails for good, a later in-flight failure is neither recorded nor classified. §12.3 also states regenerate never reads `on_notes_failure`; that key belongs to release alone.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/regenerate_batch.go:383-392 — `notesSkipCause` replaces the removed `classifyNotesFailure`. A known notes failure takes `notes.CauseText`'s phrase, which carries the location suffix, plus `notesFailureOutput`'s captured output. These are the same two derivations the ✗ line uses: `failureMessage` (internal/engine/release.go:2051) and `failureOutput`→`notesFailureOutput` (internal/engine/release.go:2006-2015). Any other failure keeps the generic `notes generation failed` with no output.
  - internal/engine/regenerate_batch.go:318-328 — the producer's error reaches the skip path. A cancel is surfaced and ends the batch; any other error becomes a `skippedVersion` whose Reason is the cause phrase.
  - internal/engine/regenerate_batch.go:434-437 — `reportSkip` now emits `presenter.Warning{Label: "skipped "+tag, Message: skip.cause(), Output: output}`. Both presenters render `Warning.Output` beneath the line (internal/presenter/plain.go:305-314, internal/presenter/pretty.go:678-699).
  - internal/engine/regenerate_batch.go:445-454 — `batchSummary` renders `tag (cause)`, with the same phrase as the warning.
  - The `reasonDiffTooLarge` constant is removed, and the only `"diff too large"` strings left in production are commit's (internal/notes/size.go:7, internal/commit/run.go:56). The release-notes path never produces `ErrDiffTooLarge`, and if it did, `ClassifyFailure` would map it to the generic reason.
  - Mode independence (AC2) holds structurally. `ReleaseDeps` carries no config. The production batch producer calls `engine.RegenerateFreshBody` directly (cmd/mint/regenerate_all.go:117), which never reads `OnNotesFailure`. The key's only production reader is `notes.ResolveFailure` (internal/notes/resolve.go:58), which is reached only from the release selector (internal/notes/select.go:266). The batch loop has no floor-choice path.
  - First-final-failure output (AC3) holds in the wave itself. internal/notes/chunk.go:212 skips classifying once the wave has ended, and internal/notes/chunk.go:232-233 drops any later failure, so `notesFailureOutput` reads part 3's `GenerationError` through the `callError` chain.
- Notes: The implementation drifts from the plan in one way: `skippedVersion.cause()` appends the saved chunk-notes pointer. That belongs to task 5-13 and does not change this task's phrase.

TESTS:
- Status: Adequate
- Coverage:
  - AC1: internal/engine/regenerate_batch_skipcause_test.go `TestRegenerateAll_SinglePassCallFails_SkipNamesTheCauseWithTheAIsOutputBeneath` drives the real `RegenerateFreshBody` with an OTHER classify. `assertSkippedV2` pins the exact Warning (label, message, output), the exact summary and the v1/v3 dispatches.
  - AC2: `TestRegenerateAll_NotesFailure_SkipsWhateverTheModeOrOnNotesFailure` covers {terminal without -y, -y} × {"", abort, fallback}. It sets `on_notes_failure` on the config the real producer receives, and asserts no gate offering `ChoiceCommitSubjects` and no NextMoves.
  - AC3: `TestRegenerateAll_WaveFailsForGoodAtPartThree_SkipNamesThePartWithItsOutputBeneath` holds part 3 until part 5 is in flight, then has part 5 fail after part 3 is classified. It asserts `AI call failed (part 3 of 6)` with `partThreeOutput` beneath.
  - AC4: timeout and missing binary are covered in `TestRegenerateAll_NotesFailure_EachCauseNamesItsPhrase`. Halving running out (both the `(part 1 of n)` case and the bare no-room-at-first-halving case) is in internal/engine/notes_halvebound_test.go `TestRegenerateAll_FreshHalvingRunsOut_SkipsThatVersionAndCarriesOn`. No room for diff (context and cap-below-instructions cases) and merge too large are in internal/engine/notes_cannotcall_test.go `TestRegenerateAll_FreshNotesCallCannotBeMade_SkipsThatVersionAndCarriesOn`. All of these go through `assertSkippedV2`, so warning and summary are both pinned. Chunk-call and merge failures with output are in internal/engine/notes_chunkfail_test.go `TestRegenerateAll_FreshChunkedRunFails_SkipsThatVersionAndCarriesOn`.
  - AC5: internal/engine/notes_failure_no_line_ceiling_test.go `TestRegenerateAll_NotesFailureSkip_NeverReadsDiffTooLarge` includes the `notes.ErrDiffTooLarge` sentinel itself and asserts neither the warning nor the summary contains `diff too large`.
- Notes: Each test would fail if its phrase, its output or the summary entry regressed. The AC3 test orders part 5's failure after part 3's recording with a 50ms sleep. That is not a defect, since the wave records part 3 under its lock immediately after the classify call returns.

CODE QUALITY:
- Project conventions: Followed. Diagnostics travel verbatim in `Warning.Output`, and the phrase derivation is shared with the ✗ line rather than re-derived.
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: There is a stale claim in a test-file comment the work edited (see FINDINGS).

BLOCKING ISSUES:
- None

FINDINGS:
- [in-scope] [contained] internal/engine/regenerate_batch_skip_test.go:22 — The file header (lines 19-22) says batch skip-and-continue is "consciously OVERRIDING the single-version on_notes_failure=abort default". `TestRegenerateAll_SkipOverridesOnNotesFailureAbort`'s doc (lines 174-177) and its failure message (line 195, "skip-and-continue must override on_notes_failure=abort") say the same. Regenerate never reads `on_notes_failure`: the only production reader is `notes.ResolveFailure` (internal/notes/resolve.go:58), reached only from release's selector (internal/notes/select.go:266), and internal/engine/regenerate_interactive.go:234 states "on_notes_failure is release's alone". The test also never sets the key. The work edited line 20 of this same paragraph and the matching production comment at internal/engine/regenerate_batch.go:185, but left these lines. Fix: reword them to say the batch skips and continues whatever `on_notes_failure` holds, which it never reads, and drop the "pins task 5-12" process reference at line 19. — FAILS: a reader is told `on_notes_failure` governs regenerate, which contradicts §12.3 and the code's own comment at regenerate_interactive.go:234, and a test claims to prove an override of a setting it never sets.
- [out-of-scope] [contained] internal/engine/regenerate_batch.go:18-20 — The file header still says processOneVersion returns "a per-version result + error the loop currently propagates) so task 5-12 can wrap each iteration in skip-and-continue". The loop already records skips (internal/engine/regenerate_batch.go:207-210), and the paragraph is written in process-artifact terms ("this task", "task 5-13"). These lines were not touched by this work. Fix: describe the as-built split, where a skip is recorded and the loop continues while an error ends the batch, with no task ids. — FAILS: the header of the file that implements skip-and-continue describes it as future work, against CLAUDE.md's "Never leave scope/phase claims that contradict the shipped code".

UNSETTLED:
- None
