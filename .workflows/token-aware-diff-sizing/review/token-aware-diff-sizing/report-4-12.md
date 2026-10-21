TASK: Warn when the chunk notes cannot be saved (token-aware-diff-sizing-4-12, tick-b0803a)

ACCEPTANCE CRITERIA:
- Given a chunked run that fails with parts finished under `on_notes_failure = abort`, and a chunk-notes write that fails, the ✗ line, its cause phrase and the AI's captured output beneath it are what they would be had the write succeeded (§9.6, §11.4)
- Given that run, one warning says the finished chunk notes could not be saved, with the write error verbatim beneath it, and the failure names no chunk-notes path (§11.4)
- Given a user cache dir that cannot be resolved, the same one warning shows, with the resolution error verbatim beneath it, and no path is named (§11.4)
- Given `on_notes_failure = fallback` and a write that fails, the same one warning shows and the commit-subject list ships (§11.4, §12.1)
- Given a wave in which no chunk finished, no such warning shows, whether or not the user cache dir resolves, since no file was due (§11.4)

STATUS: complete

SPEC CONTEXT: §11.4 "When the file cannot be written" (and its 2026-10-19 corrigendum): an unresolvable user cache dir or a failed write leaves the failure the AI's (✗ line, cause phrase, verbatim output unchanged, per §9.6/§11.2); mint adds one warning that the finished chunk notes could not be saved, with the error verbatim beneath it; the run then proceeds as the no-file branch: no path named, the floor's `e` opens empty, guidance says to write the notes and release with `--notes-file`. §11.4's "No file when no chunk finished" means no save is due, so no warning. §12.1's fallback row ships the commit-subject list with its one fallback warning, naming a file only where one was saved.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/chunknotes.go:29-41 — `saver()` records the store's error in `saveErr` and the saved text only on success
  - internal/engine/chunknotes.go:54-64 — `warnUnsaved` emits the single `notes` warning, `Message: "the finished chunk notes could not be saved"`, `Output: saveErr.Error()`, then clears `saveErr` so a later call (review-gate `reportKept`, :77-82) cannot repeat it
  - internal/engine/release.go:562-568 — abort path: the ✗ line is built from the AI's cause (`stageFailure`, release.go:1442-1452) before the warning; `SavedNotes` comes from `notes.SavedChunkNotesPath(cause)`, which is "" because `saveFailedChunkNotes` (internal/notes/chunknotes.go:68-77) returns the failure unwrapped when the save errs
  - internal/engine/release.go:576 — attended floor receives `chunkNotes.saved` (empty after a failed save), so `e` opens empty
  - internal/engine/release.go:580-582 — fallback path warns after `stage.done()`, ahead of the fallback warning, which names no file since the cause carries no saved path (notesstage.go:130-139)
  - internal/notescache/chunknotes.go:30-38, 42-46 — `NewUserChunkNotes` resolves the cache dir lazily at save time and returns the resolution error unwrapped; write errors are wrapped with `%w`
  - cmd/mint/main.go:193, :305 — production wiring is `notescache.NewUserChunkNotes(os.UserCacheDir)` for both release and regenerate, replacing the earlier silent nil-on-unresolvable store
  - internal/notes/chunknotes.go:48-58 — a wave with no finished part returns before any save, so `saveErr` is never set and no warning fires
- Notes: Every warning is emitted after the stage's ✗/✓ line, which respects the presenter's stream discipline (no Warn while the blocking stage's spinner runs). The save runs once on the generator's goroutine after the wave (internal/notes/chunk.go:56-65), so the unsynchronised `saveErr` write has no race. A cancel skips the save (chunknotes.go:69), so a Ctrl-C produces no warning. The README (README.md:131) documents the behaviour accurately.

TESTS:
- Status: Adequate
- Coverage:
  - AC1/AC2: internal/engine/notes_unsavedchunknotes_test.go:68-101 ("the write fails" subtest) asserts the StageFailure message `AI call failed (part 2 of 2)` and output `usageLimitOutput`, which match the saved-file case's assertions at internal/engine/notes_savedchunknotes_test.go:134-145; it also asserts `SavedNotes == ""`, exactly one warning equal to the expected `Warning` with the write error verbatim, and that the warning falls after StageFailed and before Unwound. internal/engine/notes_floor_nextmoves_test.go:115-128 confirms that the unattended next moves name no path (`writeNotesMove`)
  - AC3: same test, "the user cache dir cannot be resolved" subtest, goes through the real `NewUserChunkNotes` with a failing resolver; internal/notescache/chunknotes_test.go also pins the verbatim resolution error at the store level
  - AC4: notes_unsavedchunknotes_test.go:103-127 asserts the ordered warnings [unsaved, fallback with no path] and that the tag annotation body is the commit-subject list; :129-149 guards against a second warning when the fallback run is then declined at the review gate (where `reportKept` calls `warnUnsaved` again)
  - AC5: notes_unsavedchunknotes_test.go:151-186 runs every-part-fails under both a resolving and an unresolvable cache dir, and asserts zero warnings and no file
  - Floor continuations are covered by phase-5 tests: dry run (notes_floor_dryrun_test.go:114-128) and an attended `e` opening empty over an earlier file (notes_floor_edit_test.go:112-135)
- Notes: Each assertion would fail if the warning were dropped, duplicated, carried a path, or replaced the AI's ✗ line. The tests are focused and none is redundant.

CODE QUALITY:
- Project conventions: Followed. Output goes through `presenter.Warn` only, the diagnostic travels verbatim in `Warning.Output`, tests use the external package with `t.Parallel()`, and exact `Warning` values are asserted
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes
- Readability: Good. The comments on `chunkNotesKeeper`, `warnUnsaved`, `NewUserChunkNotes` and `ReleaseDeps.ChunkNotes` all match the code as built
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
