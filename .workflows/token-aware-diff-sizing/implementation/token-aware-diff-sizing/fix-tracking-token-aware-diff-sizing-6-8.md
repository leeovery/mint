## Attempt 1

ISSUES:
- /Users/leeovery/Code/mint/internal/engine/release_remergefail_test.go:204: the "no AI call" assertion (`run.regen.calls() != 0 || run.tr.classifs != 0`) can never fail in this scenario. `gateAbortTransport.Generate` sends a prompt to `regen` only when it starts with `nudgedInstructions(exportNudge)`. Stdin closing means no nudge is ever read, so any re-merge would carry an empty one-time context. `appendOneTimeContext(instr, "")` returns `instr` unchanged, so `composeMergePrompt` opens `DefaultPrompt\n\n<changeMap>`. That prompt falls to the `!strings.Contains(prompt, "diff --git ")` branch, which returns `sixPartMergeBody` without touching either counter. Suppose `regenerateBody` called `Regenerate(ctx, "")` before returning on the AskLine error. The file would still be saved and named, the run would still unwind, and every assertion in this test would pass, even though acceptance criterion 4 ("no AI call is made") is broken.
  FIX: Count every transport call. Add a `calls int` field to `gateAbortTransport` (release_reviewgate_unwind_test.go:25) and increment it at the top of `Generate` under the lock it already holds. Then assert `run.tr.calls == len(sixPartPaths)+1` in this test, which is the first run's six chunk calls plus its merge. The existing `regen`/`classifs` check can then go.
  ALTERNATIVE: Wrap `run.tr` in a counting `notes.Transport` defined in release_remergefail_test.go, so 6-7's helper is untouched. The cost is one more small type. The field is the smaller change, and it is additive, so 6-7's tests are unaffected.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/engine/release_remergefail_test.go:105: the helper never checks the ✗ line. Callers check `SavedNotes` themselves, and the stdin-closed test uses this helper where no ✗ line exists.
  OLD: // assertSavedAndNamed fails unless the six parts' notes sit in the file at path, the
// ✗ line names it, and the next moves name it with the release command, ahead of the
// unwind.
  NEW: // assertSavedAndNamed fails unless the six parts' notes sit in the file at path and
// the next moves name it with the release command, ahead of the unwind.
- /Users/leeovery/Code/mint/internal/engine/chunknotes.go:48: "the failed stage" is wrong for the release.go:564 caller. There the notes stage ended with ✓ after `fallback` shipped, and the save that failed belonged to that stage.
  OLD: // warnUnsaved warns once, with the save's error verbatim beneath, when finished chunk
// notes were due to be saved and could not be. Call it only once the line that ends
// the failed stage is shown, so the warning follows it.
  NEW: // warnUnsaved warns once, with the save's error verbatim beneath, when finished chunk
// notes were due to be saved and could not be. Call it only once the line that ends
// the stage whose save failed is shown, so the warning follows it.

NOTES:
- `reportKept` (internal/engine/chunknotes.go:74) hard-codes `mint release` and stage `regenerate`. When 6-10 wires `keep` on regenerate's gate, it needs `mint release regenerate <version> --notes-file <path>`, so either this helper takes the command or that task adds its own reporter.
- The executor's decisions hold. Reusing `notesFileMove` (including the "filling in any files its marker lists" clause) matches the spec's own failed-merge example in §12.4. Showing no next move when the save fails matches 6-7's single-pass failed `r`, which shows none, and the "names no path" criterion.
- `perRunRegenerator` now takes six parameters. That is a deliberate trade: `RegenRun.Merged` gave way to the shared keeper pointer the gate-time save needs. Regenerate still uses `RegenRun`, and its doc comment is still accurate.
- `KeepPartNotes` (internal/notes/chunknotes.go:64) does not guard against empty part notes. `MergeInputs` only comes from a written chunked run or its cache entry, so every part has notes and nothing is reachable today.
- Nothing arrived beyond the enumerated inputs.

## Attempt 2

ISSUES:
- /Users/leeovery/Code/mint/internal/engine/chunknotes.go:60 (`k.saveErr = nil`), together with /Users/leeovery/Code/mint/internal/engine/release.go:599 (`chunkNotes.reportKept(p, err)`).
  - What this task changed: `reportKept` now runs `warnUnsaved` on every review-gate abort, not only on a failed `r`. The clear is the only thing that stops a duplicate.
  - How the duplicate happens without the clear:
    1. A run's chunk wave fails and its chunk-notes save fails.
    2. The notes stage warns, either at release.go:551 (attended abort mode, then the floor's `c`) or at release.go:564 (`fallback` shipped).
    3. The operator declines at the y/n/e review gate.
    4. `reportKept` emits a second identical "the finished chunk notes could not be saved" warning before the unwind.
  - No test reaches that path. Every test that uses `failingChunkNotes`, `failingChunkNotesSaver` or `unresolvableUserCacheDir` either accepts at the gate, stops at the floor, or saves during the `r` (the only `ChoiceNo` among them is the 6-8 cache-seeding run, which saves nothing). Deleting the clear would ship a duplicated presenter line, a break in the exact-rendered-lines contract, with every test still green.
  FIX: Add a decline variant beside `TestRelease_ChunkNotesCannotBeWritten_Fallback_WarnsThenFallsBackNamingNoFile` in /Users/leeovery/Code/mint/internal/engine/notes_unsavedchunknotes_test.go:103.
  - Setup: `newChunkNotesRun(t, "fallback", twoPartTransport(failsSecondPart))`, `run.deps.ChunkNotes = failingChunkNotes{err: errChunkNotesWrite}`, `run.f.SeedSequence("git", ScriptedOut(subjects))` for the `log --format=%s`, and `run.rec.NextChoices = []presenter.Choice{presenter.ChoiceNo}`. There is no pre_tag hook, so the unwind needs no seeding.
  - Assert the run aborts non-zero.
  - Assert `warnings(run.rec)` equals exactly `[unsavedChunkNotesWarning(errChunkNotesWrite.Error()), fallbackWarning("AI call failed (part 2 of 2)", usageLimitOutput)]`.
  ALTERNATIVE: Drive attended abort mode instead (floor `c`, then review `n`) and assert one unsaved warning. This covers the release.go:551 site rather than :564; either site proves the clear. The fallback variant is recommended because it reuses an existing setup unchanged.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/engine/release_remergefail_test.go:34 — the comment restates the type's name and its one method.
  OLD: // countedRegen counts the calls made to regen.
  NEW:

NOTES:
- Fix round 1 holds. `gateAbortTransport.calls` increments at the top of `Generate`, so any stray `Regenerate(ctx, "")` on the closed-input path adds an 8th call, and `run.tr.calls == len(sixPartPaths)+1` fails as it should.
- The Ctrl-C test guards the cancel check. The scripted regen returns `context.Canceled`, `sendMerge`'s `callError` keeps it in the chain, and dropping the `errors.Is(failure, context.Canceled)` guard in `saveFailedChunkNotes` would write the file that `assertNoSavedChunkNotes` rejects.
- The over-cap nudge is missing from the AC5 matrix (`TestRelease_FailedReMerge_UnwindsToTheStartLeavingTheSavedFile`), which runs four of the five failure kinds under both configs. Its own test only checks that an Unwound follows the next moves. The risk is low: after `Remerge` returns, the path through regenerateBody, `surface`, `reportKept` and `Unwind` is the same for every error, SIZE included, and SIZE is in the matrix.
- `reportKept` (/Users/leeovery/Code/mint/internal/engine/chunknotes.go:74) hard-codes `mint release` and stage `regenerate`. Task 6-10 needs `mint release regenerate <version> --notes-file <path>`, so either this helper takes the command or that task adds its own.
- The executor's two decisions hold. Reusing `notesFileMove`, including its marker clause, matches §12.4's own failed-merge example. Showing no next move when the save fails matches 6-7's single-pass failed `r` and the "names no path" criterion.
- I found the fix-tracking file for this task in the working tree myself. Nothing arrived beyond the enumerated inputs.
