TASK: A failed `r` the ladder made chunked aborts with its finished notes saved (token-aware-diff-sizing-6-9, tick-5560e1)

ACCEPTANCE CRITERIA:
- An `r` over the cap that goes as 4 parts, part 3 failing for good (OTHER, with output), parts 1, 2 and 4 finishing: ✗ line `AI call failed (part 3 of 4)` with the call's output verbatim; the saved file holds parts 1, 2 and 4 in chunk order, opening with the missing-notes marker listing part 3's files; the failure names the full path with `mint release --notes-file <path>` (§11.2, §11.4, §12.4, §15.1)
- An `r` whose single-pass call is classified SIZE and re-cut into 3 parts, part 2 failing for good: saved and named the same way, parts 1 and 3's notes, marker listing part 2's files (§10.1, §11.4, §15.1)
- Such an `r` whose parts all finish and whose merge fails: the file holds every part's notes and no marker (§11.3, §11.4)
- Such an `r` whose wave fails before any part finishes: no file saved, none named (§11.4)
- A single-pass `r` failing without chunking (OTHER, timeout, program missing): no file saved, none named (§15.1)
- Any of these: aborts before any tag, exit 1, whatever `on_notes_failure` holds; no `c` / `e` / `a` choice (§12.1, §15.1)
- Ctrl-C while the `r`'s chunk calls run, some parts finished: aborts as a cancel, no file saved, no further AI call, and under `fallback` no commit-subject list ships (§9.1, §11.5)
- Chunk-notes file cannot be written: ✗ line, cause phrase and output unchanged; one warning with the write error verbatim; no path named (§11.4)

STATUS: complete

SPEC CONTEXT: §15.1 says a single-pass `r` re-runs the AI path from the size check, so the ladder can turn it into a chunked run, either over the cap or through halving. When such an `r` fails, mint first saves the per-chunk notes and names them with `mint release --notes-file <path>`. `on_notes_failure` does not apply, and the abort unwinds before any tag. §11.2: the first final failure ends the wave, and calls already in flight finish and are saved. §11.4 covers the file's contents and marker, no file when no chunk finished, and the write-failure warning. §11.5 and the §15.1 corrigendum: a Ctrl-C is a cancel and saves nothing.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/release.go:1340-1350 — perRunRegenerator builds the `r` over chunkNotes.regenRun(progress), so the size-check generation (GenerateWithContext) is wired to the keeper's sinks, including the chunk-notes saver
  - internal/engine/chunknotes.go:97-99 (regenRun → NotesSinks with ChunkNotes: k.saver()), :115-125 (RegenRun.regenerator: merge-only while merge inputs are held, else the wired size-check generation)
  - internal/notes/chunk.go:57-66 — writeParts hands both a wave failure and a merge failure to keepChunkNotes. This is the first attempt's path, reused unchanged
  - internal/notes/chunknotes.go:48-58 (keepChunkNotes: no file when no part finished; marker when any part has none), :68-77 (saveFailedChunkNotes: a cancel or a failed save keeps nothing)
  - internal/notes/chunk.go:184-186 — a cancelled wave returns ctx.Err() with no parts, so a Ctrl-C saves nothing
  - internal/engine/release.go:1882-1886 — a Regenerator failure is surfaced as the "regenerate" ✗ line. Because keepRegen (chunknotes.go:68-73) returns the failure unchanged when no merge inputs are held, nothing is saved twice
  - internal/engine/chunknotes.go:77-92 — reportKept emits the write-failure warning after the ✗ line, then names a saved file with notesFileMove("mint release", path)
  - internal/engine/release.go:609-614 — any review-gate error, a failed `r` included, goes straight to Unwind. The floor and `on_notes_failure` are never consulted
- Notes: The landed commit 420106e was a one-line wiring change (WithChunkNotes on the `r`'s generator). A later refactor moved that wiring into chunkNotesKeeper.regenRun, where it still holds. The comments on perRunRegenerator, RegenRun.regenerator, keepRegen and reportKept are accurate against the current code. There is no drift from the plan or the spec.

TESTS:
- Status: Adequate
- Coverage (internal/engine/release_regenchunkfail_test.go):
  - AC1 :192 — exact StageFailure (name, `AI call failed (part 3 of 4)`, loginOutput, SavedNotes path), exact file contents (marker for c/3.go plus parts 1, 2 and 4), next moves naming `mint release --notes-file`, emitted before the unwind. All four parts are dispatched before any outcome is recorded (the wave holds its lock while sending, and the in-flight limit is 4), so part 4 is deterministically in flight and saved
  - AC2 :207 — halved `r`: single pass overflows, classified SIZE; part 2 fails with login output, classified OTHER; message `(part 2 of 3)`, exact file contents with core/b.go marker
  - AC3 :221 — merge failure for both the over-cap and the halved `r`: `(merging)` message, every part's notes, no marker
  - AC4 :263 — every part fails: no file under the chunk-notes base, no SavedNotes, no next moves
  - AC5 :276 — single-pass OTHER, timeout and missing binary: correct cause phrases, no file named
  - AC6 :309 — 3 configs (unset, abort, fallback) × 4 failure shapes: unwound to start, no tag, no mutation, ExitCode == 1 (assertAbortNonZero fatals on a non-AbortError first, so the check is effective), no floor gate, no `git log` subject list
  - AC7 :370 — parts 1, 2 and 4 finish before part 3 cancels. The test checks errors.Is(context.Canceled), no AI call after the cancel (a stray classify or merge would show), no file or path, no subject list under fallback, and an unwind to start
  - AC8 :409 — failing store: exact StageFailure with no SavedNotes, the single warning with the write error verbatim placed after the ✗ line and before the unwind, no next moves
- Notes: Each test would fail if its behaviour broke. The AC6 matrix does not repeat the timeout, missing-binary and no-part-finished shapes, but the abort route after the review gate does not depend on the cause, so the four representative shapes cover it. The tests stay focused on behaviour; nothing is redundant beyond the deliberate unset-versus-explicit-abort config pair.

CODE QUALITY:
- Project conventions: Followed (external test package, t.Parallel throughout, FakeRunner and RecordingPresenter, exact assertions on ✗ lines and moves, Unwind detaches from the cancelled ctx at internal/engine/unwind.go:121)
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
