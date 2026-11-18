TASK: A failed regenerate `r` saves the version's chunk notes before it aborts (token-aware-diff-sizing-6-10, tick-a381fa)

ACCEPTANCE CRITERIA:
- `mint release regenerate v1.4.0` from a fresh source with chunked notes, `r` whose re-merge fails: writes `{user cache dir}/mint/chunk-notes/{repo path, separators as dashes}/v1.4.0.md` holding every part's notes with no missing-notes marker; the failure names its full path with v1.4.0's `mint release regenerate <version> --notes-file <path>`; the regenerate aborts and exits 1, writing nothing to the changelog or the provider release
- An `r` on single-pass notes the ladder made chunked, whose wave fails for good after some parts finished: the file holds those parts' notes and the missing-notes marker, named the same way
- Stdin closing at the `r`'s nudge prompt on chunked notes: no AI call, the file is saved and named the same way before the regenerate aborts
- A single-pass `r` that fails without chunking: the regenerate aborts, no file saved or named
- `regenerate --all --source fresh` at a terminal without `-y` over v1.2.0/v1.3.0/v1.4.0, v1.2.0 regenerated, an `r` on v1.3.0's chunked notes fails: v1.3.0's file saved and named with v1.3.0's regenerate command; the batch ends as a failed gate does and exits 1; v1.4.0 not processed; v1.3.0 not reported as skipped
- The chunk-notes file cannot be written: ✗ line, cause phrase and captured output unchanged; one warning that the finished chunk notes could not be saved, write error verbatim beneath; no path named

STATUS: complete

SPEC CONTEXT: §15.1 (On regenerate / --all): a failed `r` aborts the regenerate; on chunked notes, or an `r` the ladder made chunked, mint first saves the per-chunk notes (§11.4) and names the file with `mint release regenerate <version> --notes-file <path>` (§13.1, one file per version). Under `--all` the failed `r` ends the batch as a failed gate does; it is not the batch's skip-and-continue floor (§12.3). §11.3/§11.4 define the file location, contents (finished parts in order, the `**Notes missing:**` marker when a slice has none), the no-file case, and the write-failure branch (corrigendum: ✗ line unchanged, one warning with the error verbatim, no path). Corrigenda on §15.1: stdin closing at the nudge is a failed `r`; regenerate's gate lands before its changelog commit, so nothing resets; a Ctrl-C is a cancel and saves nothing.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/regenerate_write.go:250-257 — `reviewFresh`: the fresh review gate's `r` goes through `regens.review(regenerator, req.chunkNotes)` and `chunkNotes.reviewGate(..., "mint release regenerate "+req.VersionKey)`; reached from `gateRegenerate` (regenerate_write.go:221-246), which both the single-version and `--all` paths share
  - internal/engine/chunknotes.go:66-73 — `keepRegen`: on chunked notes at the gate (`k.merged` set) saves every part's notes via `notes.KeepPartNotes` (no marker; all parts finished)
  - internal/engine/chunknotes.go:75-92 — `reportKept`/`reviewGate`: after any gate error, warns once on a failed save (`warnUnsaved`) and names a saved file with the version's regenerate command via `notesFileMove`
  - internal/engine/chunknotes.go:94-103 — `regenRun`/`heldMerged`: an `r` reads the keeper's merge inputs live, and the ladder-chunked `r`'s generator is wired to the keeper's sinks, so a failed chunked `r` saves finished parts plus marker (`notes.keepChunkNotes`, internal/notes/chunknotes.go:48-58) and a written one's inputs feed the next `r`
  - internal/engine/release.go:1866-1886 — `regenerateBody`: closed input at the nudge runs `regen.keepFailed` before any Regenerate call; a Regenerate failure is kept, then surfaced
  - internal/notes/chunknotes.go:68-77 — `saveFailedChunkNotes`: a cancel, nil save or failed save keeps nothing and returns the failure unchanged
  - internal/engine/regenerate_interactive.go:230,255-266 — single-version wiring keyed by `req.Tag`; internal/engine/regenerate_batch.go:316,336-340,463-479 — batch wiring keyed by `res.Tag`; a gate error propagates out of `processOneVersion` and `RegenerateAll` returns it with no summary (regenerate_batch.go:197-201)
  - cmd/mint/main.go:193 and cmd/mint/regenerate_all.go:131-139 — production wires `notescache.NewUserChunkNotes(os.UserCacheDir)` and `RegenerateFreshRegenerator` over the RegenRun on both paths
- Notes: The command names the bare version key (`mint release regenerate 1.4.0 ...`), matching the regenerate floor's and the `--all` skip pointer's convention; `version.ParseSemVer` (internal/version/version.go:72-83) accepts it with or without the configured prefix, so the named command resolves the same version. The `regens == nil || chunkNotes == nil` branch in `reviewFresh` is reachable only through a direct `RegenerateWrite` call; the only production caller (`RegenerateRun`) always sets both.

TESTS:
- Status: Adequate
- Coverage (internal/engine/regenerate_regenfail_test.go):
  - AC1: TestRegenerateRun_ReMergeFails_SavesEveryPartsNotesAndNamesThemWithTheVersionsRegenerate — exact file content at the path computed independently (`chunkNotesPathIn`), ✗ line message/output/SavedNotes, next moves naming `mint release regenerate 1.4.0 --notes-file <quoted path>`, no warnings, exit-1 abort, no CHANGELOG.md, no commit/push, no provider dispatch
  - AC2: TestRegenerateRun_RegenTheLadderChunkedFailsForGood_SavesTheFinishedPartsWithTheMarker — marker `(partly)` plus parts 1, 3, 4 of 4
  - AC3: TestRegenerateRun_StdinClosedAtTheNudgeOnChunkedNotes_MakesNoCallAndSavesEveryPartsNotes — AI call count stays at the first run's 3, file saved, named, err is ErrInputClosed
  - AC4: TestRegenerateRun_SinglePassRegenFailsWithoutChunking_SavesAndNamesNoFile
  - AC5: TestRegenerateAll_RegenOnAChunkedVersionFails_SavesItsFileNamesItsRegenerateAndEndsTheBatch — v1.3.0's file and command, only v1.2.0 dispatched, v1.4.0 never started, no skip warning, no RunFinished
  - AC6: TestRegenerateRun_FailedRegensChunkNotesCannotBeSaved_KeepsTheFailureAndWarnsWithTheWriteError — table over re-merge, ladder-chunked and closed-nudge; exact StageFailure equality (SavedNotes empty), warning after the ✗ line, only that warning, no next moves
  - Extra edge: TestRegenerateRun_RegenFailsOnChunkedNotesAnEarlierRegenWrote_SavesThatRegensPartNotes — a second `r` after a ladder-chunked `r` wrote notes saves that `r`'s part notes (re-merge failure and closed nudge)
- Notes: Each test would fail if its behaviour broke (exact content, exact argv-free event payloads, exact command text). No redundant cases; the extra edge test pins the live merge-inputs read that the keeper's design depends on. The write-failure branch is exercised on the single-version path only; the batch path reaches the same `chunkNotesKeeper.reviewGate` code, so this is not a gap.

CODE QUALITY:
- Project conventions: Followed — output only through the presenter, sentinel matching via errors.Is/errors.As, cancel propagates without a save, write error carried verbatim in Warning.Output, external test package, t.Parallel throughout, t.TempDir roots
- SOLID principles: Good — the keeper owns save/warn/name for both verbs; the verb supplies only the command string
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: None. Comments in the changed code (chunknotes.go, reviewFresh, RegenerateWriteRequest's regens/chunkNotes fields, RegenRun, RegenerateFreshRegenerator) hold against the code as built.

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
