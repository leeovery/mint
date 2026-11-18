## Attempt 1

ISSUES:
- Concurrent chunk calls race inside the shared `runner.FakeRunner` in existing engine and cmd tests, so the race gate fails. Those tests run a chunked release through the real `ai.Transport` over one `FakeRunner`. The two chunk calls now enter it at once from the `wg.Go` workers (/Users/leeovery/Code/mint/internal/notes/chunk.go:52-55). They write `f.invocations` unguarded (/Users/leeovery/Code/mint/internal/runner/fake_runner.go:98) and read and write `f.sequences` unguarded in `outcome` (fake_runner.go:140, :142). I reproduced it on every package run with -count of 5 or more. The race detector names these tests:
  - TestRelease_OverCap_ChunkedNotesShipByteIdenticalToEverySink (/Users/leeovery/Code/mint/internal/engine/release_chunked_test.go:31)
  - TestRelease_OverCap_ChunkedNotesReachTheReviewGateAsAINotesWithNoPromptBeforeThem (release_chunked_test.go:76)
  - TestNewBatchBodyProducer_Fresh_OverCapVersionIsChunkedOverItsRange (/Users/leeovery/Code/mint/cmd/mint/regenerate_all_test.go:233)
  Detection is intermittent: two workers that both go through `context.WithTimeout` on the same parent sometimes get a happens-before edge from the parent's cancelCtx mutex. That explains why the executor's runs passed. Because the tests use t.Parallel, a full-suite run also marks unrelated sibling tests as failed. The task context had flagged that "runner.FakeRunner holds no lock".
  FIX: Make `FakeRunner` safe for concurrent use.
  - Add `mu sync.Mutex` to the struct and hold it in `Seed`, `SeedSequence`, `SeedNotFound`, `RunWith`, `RunInDir` and `RunInteractive`. The recording append and the `outcome` lookup and consumption must sit inside one critical section, so two calls cannot consume the same sequence entry.
  - Drain stdin before taking the lock.
  - Have `Invocations()` return `slices.Clone(f.invocations)` under the lock.
  - Add a test in /Users/leeovery/Code/mint/internal/runner/fake_runner_test.go: N goroutines call `RunWith` on a `SeedSequence` of N entries. Assert N invocations and that each outcome is consumed exactly once. The gate runs it under -race.
  - Update the FakeRunner doc so "recorded in order" reads as arrival order, which is what it means once calls run concurrently.
  - Add a contract line to the `notes.Transport` doc (/Users/leeovery/Code/mint/internal/notes/generate.go:10-17) saying `Generate` is called concurrently and must be safe for concurrent use. The types cannot express this, and the gap is how this race got in.
  The engine and cmd assertions are symmetric over the two chunk replies, and the merge is always the third `claude` call, so those tests need no change.
  CONFIDENCE: high
- `TestGenerator_OverCap_ChunksFinishingOutOfOrder_MergeCarriesNotesInChunkOrder` is flaky. It failed 28 of 300 runs under -race (`go test -race -count=300 -run ...ChunksFinishingOutOfOrder... ./internal/notes/`), and it also failed in one full-suite run. Every ordering appeared, from `[end 3 end 1 end 2]` to `[end 1 end 2 end 3]`.
  The cause: the test's gate closes `finished[part]` in its `defer` (/Users/leeovery/Code/mint/internal/notes/chunkwave_test.go:223), but `waveTransport.Generate` records `"end N"` only after the gate returns (chunkwave_test.go:53-57). Part k is released before part k+1 has recorded its end, so the precondition check at :245-247 races. The comment at :221 ("so they end 3, 2, 1") states the guarantee the code does not provide.
  FIX: Release the next part only after this part's end is recorded. Add an `ended func(part int)` field to `waveTransport`, called right after the `"end N"` record (nil means no-op). In this test, drop the `defer close(finished[part])` from the gate and set `wt.ended = func(part int) { close(finished[part]) }`. The finish order is then deterministically 3, 2, 1 at the transport boundary, and the :221 comment becomes true. Re-run with `-race -count=300` to confirm zero failures.
  ALTERNATIVE: Have the test record its own finish order inside the gate, before the close, and assert on that instead of the transport's end events. That is smaller, but it proves the gates' order rather than the transport's return order, so it is the weaker check. I recommend the hook.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/notes/chunk.go:42-43 — "the transport's per-attempt deadline is the only one" is a claim about every deadline in the call path, and it turns false the day any caller passes a ctx with a deadline. What this code needs to say is only that the wave adds no deadline of its own.
  OLD: // runs on ctx itself: the transport's per-attempt deadline is the only one, so the
// wave as a whole has no deadline. A failure is reported for the lowest-numbered
// failed part.
  NEW: // runs on ctx itself: the wave sets no deadline of its own, so a timeout is per call.
// A failure is reported for the lowest-numbered failed part.

NOTES:
- Failure behaviour changed from the serial version, which stopped at the first error. Now every chunk call runs even after one fails, and the error returned is the lowest-numbered part's. The executor marked this as off-script and left it to task 2-10. Task 2-10's ACs ("no chunk call that had not started is started"; "the ✗ line carries the output of the one that failed first", which is time order, not part order) will replace both this behaviour and the "lowest-numbered" sentence in the `generateParts` doc. It is acceptable as an interim state inside the phase.
- The slot acquisition `slots <- struct{}{}` (chunk.go:51) does not select on `ctx.Done()`. After a Ctrl-C, the remaining slices still get workers, each calling `Generate` on a cancelled ctx. `exec.Cmd.Start` refuses a done ctx, so no process spawns and nothing hangs. Task 2-10's Ctrl-C AC ("no further call starts") will need a ctx check at this point.
- `TestGenerator_ChunkBound_NoConfigKeyControlsIt` is a regex scan of `config.MetadataRows()`. It passed before the change, would miss a key whose description used other words, and does not cover flags; the executor says so. The real proof of AC3 is the unexported constant. It is a weak guard but not a false one.
- `TestGenerator_OverCap_WaveLongerThanTimeout_EachCallOwnDeadline_NotesWritten` relies on wall-clock margins: 300ms headroom per call and a 100ms window on the measured allotment. It passed 40 runs in isolation and 30 each at -cpu=1 and 2 under -race. I did not reproduce a flake, but it is the test most exposed to a loaded CI host.
- `generateParts` composes each chunk prompt itself from `(i+1, len(slices))`. Task 2-7's pieces need different framing (which file, piece k of m), so that task will probably pass prepared prompts in rather than raw slices.
- Nothing arrived beyond the enumerated inputs.
