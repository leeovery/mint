TASK: A failed chunk ends the wave (token-aware-diff-sizing-2-10)

ACCEPTANCE CRITERIA:
- Given a wave in which one chunk call fails, whether after the transport's retry, by timeout or by a missing binary, no merge call is made and no chunk call that had not started is started. Calls already running finish (§11.1, §11.2, §11.6)
- Given that failure under `on_notes_failure = abort`, the run stops before the tag. Its ✗ line carries the failure's cause phrase, with the failed call's captured output verbatim beneath it (§11.2, §12.1)
- Given two chunk calls in flight together that both fail, the ✗ line carries the output of the one that failed first (§11.2)
- Given chunk calls that succeed and a merge call that fails, the notes fail the same way, with the merge call's captured output verbatim beneath the ✗ line (§11.3)
- Given a chunk or merge failure under `on_notes_failure = fallback`, the fallback body ships. Chunk notes never become the body, alone or joined (§11.1, §11.3, §12.1)
- Given a chunk or merge failure on `mint release regenerate <version>` from a fresh source, the regenerate aborts. Under `regenerate --all`, that version is skipped and the batch carries on with the next (§12.3)
- Given a Ctrl-C while chunk calls are in flight, every call in flight is cancelled, no further call starts, and the run aborts cleanly. `on_notes_failure` does not apply, so under `fallback` no fallback body ships (§11.5)

STATUS: complete

SPEC CONTEXT: §11.1: any chunk that ends in failure fails the generation. No merge runs, and merging the slices that did come back is ruled out because the missing slice may hold the headline change. §11.2: the first final failure ends the wave. No new chunk calls start, calls already running finish, and the ✗ line renders that first failure's cause phrase and verbatim output. §11.3: a failed merge fails the notes, and nothing concatenates the chunk notes. §11.5: Ctrl-C cancels the calls in flight, saves nothing and never reaches `on_notes_failure`. §11.6: the timeout applies per call, and a chunk timeout is a final failure. §12.1/§12.3: routing is the same as a single-pass failure. On release that means `on_notes_failure`. A single-version regenerate aborts. Phase 5 later gives the attended single-version regenerate the floor choice, and the unattended one still aborts. `--all` skips the version and carries on.

IMPLEMENTATION:
- Status: Implemented. Later phases (4-9 classifier and halving, 4-10 chunk-notes save, 5-x floor) have reshaped the code, and the behaviour this task asked for still holds.
- Location:
  - internal/notes/chunk.go:168-193 — `wave.run`. `canSend` (chunk.go:195-197) goes false once `w.failure` is set or ctx ends, so no queued part is sent. The loop waits on the cond until `inFlight == 0`, so running calls finish. `ctx.Err()` is returned before `w.failure`, with no parts or notes, so a cancel wins over an earlier chunk failure.
  - internal/notes/chunk.go:209-241 — `wave.send`. Only the first failure to reach the lock is recorded, through `case w.failure != nil`. A failure arriving after the wave ended is not classified (`!w.ended()` at chunk.go:212) and not halved. `inFlight--` and recording the outcome happen in the same locked step, so the freed slot is never reused before the failure is visible.
  - internal/notes/chunk.go:57-66 — `writeParts` merges only when the wave returned nil. Any failure goes to `keepChunkNotes` with an empty body, so no code path joins part notes into a body.
  - internal/notes/chunk.go:82-91 — `sendMerge` wraps a merge failure as a `callError` located at "merging" (a SIZE answer becomes ErrMergeTooLarge). The `*ai.GenerationError` stays on the %w chain.
  - internal/notes/chunknotes.go:68-77 — `saveFailedChunkNotes` saves nothing on `context.Canceled`, which matches §11.5.
  - internal/notes/resolve.go:54-62 — `ResolveFailure` returns a cancel unchanged before checking the mode, so no fallback body is built for it.
  - internal/engine/notesfloor.go:32-38 — `reachesNotesFloor` excludes a cancel. internal/engine/release.go:562-579 renders the StageFailed and unwinds without the floor or the next moves. internal/engine/release.go:2009-2015 `notesFailureOutput` pulls the verbatim output through the chain.
  - internal/engine/regenerate_interactive.go:233-250 — a single-version regenerate aborts unattended. Attended, it gets the Phase 5 floor choice, a planned evolution and not a drift.
  - internal/engine/regenerate_batch.go:317-328 — `--all` treats a cancel as batch-ending (line 320). Any other generation failure becomes a skip, carrying the cause phrase and verbatim output (`notesSkipCause`, regenerate_batch.go:386-392).
- Notes: "First" is measured when the failure is recorded, after any classify call, which is spec §11.2's "first final failure". A chunk whose failure is still being classified keeps its slot counted in flight, so no extra call starts in its place. internal/engine/unwind.go:121 detaches the unwind from cancellation, so a Ctrl-C still resets a pre_tag artifact commit cleanly.

TESTS:
- Status: Adequate
- Coverage:
  - AC1: internal/notes/chunkfail_test.go:54 runs a table over three causes: a GenerationError after the retry, a timeout, and a missing binary. With six parts and four in flight, part 2 fails. The test asserts no merge, exactly 4 starts, and `end N` for parts 1, 3 and 4. `waveTransport` logs `cancelled N` when its ctx is cancelled, so a regression that cancels in-flight calls would fail this test.
  - AC2: internal/engine/notes_chunkfail_test.go:126 covers a chunk failure, a chunk timeout, a merge failure and a merge returning empty notes, under `abort`. It asserts no tag, push or provider mutation, the exact AI call count, the StageFailed message (cause phrase plus location), and the verbatim Output.
  - AC3: internal/notes/chunkfail_test.go:118. Part 3 fails first, part 1 fails later, and the error's GenerationError output is part 3's. The ✗ line takes its output from that same chain through `notesFailureOutput`, and AC2's engine test covers that path.
  - AC4: internal/notes/chunkfail_test.go:179 (one merge, merge output on the chain) and the merge rows of the engine table.
  - AC5: internal/engine/notes_chunkfail_test.go:163 checks that the tag annotation body equals the commit-subject list, and that the CHANGELOG section holds neither part's notes. It runs for both chunk and merge failures.
  - AC6: internal/engine/notes_chunkfail_test.go:204 (single-version regenerate aborts with the phrase and output, and no provider write) and :236 (`--all` skips v2, dispatches v1 and v3, and the skip warning and summary carry the phrase and output).
  - AC7: internal/notes/chunkfail_test.go:234 (cancel with four in flight leaves 0 in flight, 4 started, no merge, and returns context.Canceled). chunkfail_test.go:317 shows a cancel after a chunk already failed still returns the cancel. internal/engine/notes_chunkfail_test.go:299 runs under `fallback` and asserts the `git log --format=%s` fallback never runs, the AI makes only 2 calls, and nothing is mutated.
- Notes: Two notes-level tests synchronise with a fixed 50ms grace: internal/notes/chunkfail_test.go:95 and :138. The second grace now also covers part 3's in-memory classify call, which Phase 4 added between the transport returning and the wave recording the failure. If either grace ever falls short, the test fails loudly rather than passing silently. I did not count this as a finding. The engine table's "merge call returns empty notes" row also exercises the Phase 4 cause phrase. That overlaps with the cause-text tests but is a legitimate case for the "fails the same way" criterion, not bloat.

CODE QUALITY:
- Project conventions: Followed. The AI goes through the consumer-defined Transport, `context.Canceled` propagates unchanged and is never routed to a fallback, and subprocess output travels verbatim in StageFailure.Output and Warning.Output. Tests use external packages, `t.Parallel()` and exact phrase and output assertions.
- SOLID principles: Good. The wave owns concurrency and the end-of-wave rule, the selector and ResolveFailure own routing, and the engine owns rendering.
- Complexity: Acceptable. `wave.send` has a five-way switch, and each branch is commented or named.
- Modern idioms: Yes (`slices`, `sync.Cond`, `errors.Is`/`As`, `%w: %w`).
- Readability: Good. The doc comments on `run`, `send` and `ResolveFailure` match the code.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
