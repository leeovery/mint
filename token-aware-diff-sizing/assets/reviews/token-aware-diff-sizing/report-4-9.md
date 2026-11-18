TASK: The first final failure ends the wave (token-aware-diff-sizing-4-9, tick-6bb015)

ACCEPTANCE CRITERIA:
- Given a wave whose part 2 is classified OTHER, no part that had not started is started and no further classify call is made. Calls already in flight finish (§11.2)
- Given part 2 instead ends with a classify call that fails, a failure with no output, a timeout, a missing binary, a SIZE answer with no halving left, or a halving round with no room, the wave ends the same way (§10.1, §11.2)
- Given part 2 is classified SIZE with halving left, the wave carries on and part 2's halved parts are sent (§11.2)
- Given part 2's final failure while part 5's call is in flight, and part 5 then fails with output, no classify call is made for part 5 (§11.2)
- Given part 3 classified SIZE, none of whose halved parts has started when part 5 fails for good, none of them is started. Given part 3's classify call is still running when part 5 fails for good and then answers SIZE, no halved part is sent for it either (§11.2)
- Given a 6-part wave whose part 2 is classified SIZE and re-cut into 3 parts, and whose part first packed as 5, now part 7 of 8, is then classified OTHER, the ✗ line reads `AI call failed (part 7 of 8)` with that part's captured output verbatim beneath it, never part 2's (§11.2, §12.4, §16.1)
- Given the wave ends on an unattended release under `on_notes_failure = abort`, the run stops before the tag, and under `fallback` the commit-subject list ships. An unattended single-version regenerate stops, and under `regenerate --all` the version is skipped and the batch carries on (§11.1, §12.1, §12.3)

STATUS: complete

SPEC CONTEXT: §11.2 makes a chunk failure end the wave once it finishes that chunk for good: anything except a SIZE answer with halving left (OTHER, a failed classify call, no output, timeout, missing binary, SIZE with no halving left or no room per §10.1/§8.7). After it, no new chunk calls (halved parts included) and no further classify calls. In-flight calls finish, and their notes are saved (task 4-10). The ✗ line carries the first final failure's cause phrase and captured output. §11.6 makes a chunk timeout a final failure with no overall deadline. §12.4/§16.1 number parts by their place in chunk order within the current total. §12.1/§12.3 route the resulting failure by mode.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/chunk.go:168-193 (`wave.run`): the send loop stops on `canSend` (chunk.go:195-197, which is false once `w.failure != nil`), then waits for every in-flight call before returning the first failure with parts and notes.
  - internal/notes/chunk.go:209-241 (`wave.send`): a failed call is classified only while `!w.ended()` (chunk.go:212). Outcomes are recorded under the lock in the order success, wave already ended (neither recorded nor halved, chunk.go:232-233), halving with room (chunk.go:234-235), SIZE with no halving left or no room becoming `ErrTooLargeForModel` (chunk.go:236-237), anything else (chunk.go:238-239). `w.failure` is set only while nil, so the first final failure wins. Its location is taken from the slot's place in the current `w.slots` at record time, so a re-cut renumbers it correctly.
  - internal/notes/chunk.go:244-248 (`wave.ended`).
  - internal/notes/classify.go:53-70: a timeout or missing binary (not a `*ai.GenerationError`) and empty output skip the classify call. A classify call that fails reads as OTHER while the original call's failure is kept.
  - internal/notes/resolve.go:164-189 (`CauseText`): the phrase plus `(part k of n)` from the `callError` location. Rendered output comes from the wrapped `*ai.GenerationError` through `notesFailureOutput` (internal/engine/release.go:2009-2015).
- Notes: Every criterion holds on reading. A SIZE answer that arrives after the end, from a classify call that was already running, falls to the `w.failure != nil` branch before the halving branch, so no re-cut is queued or reported. Halved parts already queued are blocked by `canSend`. The comments on `run`, `send`, `ended` and `wave` match the code.

TESTS:
- Status: Adequate
- Coverage:
  - AC1: internal/notes/waveend_test.go:155 (`TestGenerator_Wave_PartClassifiedOther_StartsNoNewPartAndClassifiesNoMore`) checks that parts 5 and 6 never start, parts 1, 3 and 4 finish, there is exactly one classify call and no merge, and the cause is part 2's.
  - AC2: internal/notes/waveend_test.go:200 is a table covering a failed classify call, no output, a timeout, a missing binary and a halving round with no room. Each case asserts four starts, the expected classify count, the cause phrase with location, and that `errors.Is` reaches part 2's own failure. SIZE with no halving left is covered at internal/notes/waveend_test.go:299 (four SIZE answers, then a held part 1 failing after the end gets no classify call, and there is no merge).
  - AC3: internal/notes/waveend_test.go:359 checks that part 2's halved parts are sent, parts 5 and 6 are sent, and the run merges.
  - AC4: internal/notes/waveend_test.go:393.
  - AC5: internal/notes/waveend_test.go:434 (halved parts queued but never started; also asserts part 5 was renumbered and its output is the one carried) and internal/notes/waveend_test.go:478 (a late SIZE answer from part 3's classify call: no re-cut is reported through `recutLog` and no halved part is started).
  - AC6: internal/engine/notes_waveend_test.go:115. The fixture asserts part 5 was sent as `part 7 of 8`, then checks that the StageFailure message is `AI call failed (part 7 of 8)` and its Output is part 5's text, not part 2's.
  - AC7: internal/engine/notes_waveend_test.go:115 (unattended abort: AbortError and no `git tag -a`), :140 (fallback ships the subject list as the tag body), :172 (unattended single regenerate stops with no provider write), :200 (`--all` skips v2 and dispatches v1 and v3).
- Notes: Each test maps to one criterion with no redundant variations. Removing the `!w.ended()` guard, the `w.failure != nil` branch or the `canSend` failure check would fail at least one test. Ordering in the notes-level tests relies on 50 ms grace sleeps (`recordGrace`), which the package already uses (chunkfail_test.go, savenotes_test.go).

CODE QUALITY:
- Project conventions: Followed. Tests use external test packages with `t.Parallel()`, assert the exact cause phrase and verbatim output, and spawn no real subprocesses. Sentinels are wrapped with `%w`.
- SOLID principles: Good
- Complexity: Acceptable. `send` carries one switch whose case order encodes the precedence (success, ended, halve, too large, other), and the comments state it.
- Modern idioms: Yes (slices.Index/Replace/Clone, sync.Cond).
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
