## Attempt 1

ISSUES:
- `internal/engine/regenerate_fresh.go:101-105`: the hand-off of an `r`'s merge inputs to `run.MergeInputs` has no test. This is the code that lets a failed `r` on chunked notes that an earlier `r` produced (single-pass notes → a ladder-chunked `r` that succeeded → a later `r` whose re-merge fails, or stdin closing at its nudge) save those part notes. I deleted lines 102-104 in a scratch copy and `go test ./internal/engine/` still passed. In that case the user loses the part notes silently: no file, no `--notes-file` pointer, and nothing in the output says notes were lost. I also wrote the scenario as a scratch test against the code on disk. It passes as built and fails without the hand-off (`reading the saved chunk notes: ... no such file or directory`), so the code works and only the test is missing.
  FIX: Add a test to `internal/engine/regenerate_regenfail_test.go` built on the existing helpers:
  - Setup: `newRegenFailRun(t, &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceRegen}, NextLines: []string{exportNudge, exportNudge}})`, then `run.regenerate(t, singlePassFresh(), tr, freshCfg())`.
  - Transport: `tr := &chunkedRegenTransport{nudge: exportNudge, singlePass: overflows, classify: sizeForOverflow, merge: <a mutex-guarded counter returning regenMergeBody on the first call and failsWithLogin() after>}`.
  - Assertions: `assertAbortExitsOne`; `readSaved(t, run.savedPath())` equals parts 1 to 4 of `regenPartNotes(n, 4)` joined by "\n\n", with no marker; `assertNamedWithRegenerate(t, run.rec, regenRunVersionKey, path)`; `run.assertWroteNothing(t)`.
  - Optional: a stdin-closed variant whose second `r` uses `AskLineResult` returning `presenter.ErrInputClosed` covers the same hand-off on the closed-input route.
  ALTERNATIVE: Remove the duplicate state instead. Have the regenerator read the merge inputs from the source the keeper owns, for example by giving `RegenRun` a `Merged func() *notes.MergeInputs` getter that `chunkNotesKeeper.regenRun` binds to `k.merged`, the way release's `perRunRegenerator` reads `chunkNotes.merged`. The two copies could then never diverge. This changes the exported `RegenRun` shape and its cmd-layer and test call sites, so it is wider than this task needs. I recommend the test.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- internal/engine/regenerate_regenfail_test.go:65 — names a parameter `tr` the function does not have
  OLD: // singlePassFresh is a fresh single-pass run over tr whose `r` re-diffs the range.
  NEW: // singlePassFresh scripts the git reads of a fresh single-pass run and of one `r` that re-diffs its range.

NOTES:
- `reviewFresh` (`internal/engine/regenerate_write.go:250-261`) writes out the `reviewGate` call twice, once per nil and non-nil keeper. Setting `regen.keep` and calling `reportKept` only when `req.chunkNotes != nil` would need one call. This is style only, not a defect.
- `RegenerateWriteRequest.chunkNotes` is an unexported field on an exported struct. That is fine here: only `RegenerateRun` and `gatePerVersion` set it, and direct `RegenerateWrite` callers (tests) get the gate as before.
- The mutation checks ran on throwaway copies in the scratchpad, since deleted. Removing `.WithChunkNotes(run.ChunkNotes)` fails the ladder-chunked and write-failure tests. Dropping `chunkNotes:` from `gatePerVersion` fails the `--all` test. Only the `run.MergeInputs` hand-off goes uncaught. The project tree was not modified.
- The orchestrator's ADDITION was marked as task content, so I reviewed against it as such. Nothing else came with the dispatch beyond the listed inputs.
