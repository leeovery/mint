TASK: At most four chunk calls in parallel (token-aware-diff-sizing-2-6, tick-56fa24)

ACCEPTANCE CRITERIA:
- Given an over-cap release that packs into six chunk calls, four are in flight together, and at no point are more than four in flight (§8.5)
- Given an over-cap release that packs into three chunk calls, all three are in flight together (§8.1, §8.5)
- No config key or flag changes the bound of four (§8.5)
- Given chunk calls that finish in a different order from chunk order, the merge call carries their notes in chunk order (§8.6)
- Given a wave, the merge call starts only after every chunk call has finished (§8.1)
- Given a per-attempt timeout, and a wave whose every call finishes inside it but whose rounds together run longer than it, the notes are written. Each chunk call and the merge call has its own per-attempt deadline, and no deadline spans the wave (§11.6)

STATUS: complete

SPEC CONTEXT: §8.1 sets the map-reduce shape: chunk calls run in parallel and one merge call is the single serial step. §8.5 says at most four chunk calls run at once, and the bound is fixed rather than a setting, so a large release never lands its whole wave on the machine and the provider's limits at the same moment. §8.6 says the merge carries every chunk's notes in chunk order. §11.6 says the per-attempt timeout applies to each call on its own, with no overall deadline across the wave.

IMPLEMENTATION:
- Status: Implemented. The task's original WaitGroup/semaphore `generateParts` (commit 01e3915) was later replaced by the `wave` type, which keeps the same bound and adds halving (2-7) and wave-end (2-10). The current code is reviewed below.
- Location:
  - internal/notes/chunk.go:116: `const maxChunkCallsInFlight = 4`. It is unexported and has one reader, `canSend` at internal/notes/chunk.go:196. A grep of cmd/mint, internal/config and internal/engine finds no concurrency key or flag.
  - internal/notes/chunk.go:168-193 (`wave.run`): dispatch is gated by `canSend` (`inFlight < maxChunkCallsInFlight`). It waits on a sync.Cond and returns only once `inFlight == 0`. The inner loop exits only with in-flight at 0 or a free slot.
  - internal/notes/chunk.go:209-241 (`wave.send`): `inFlight--` and the outcome are recorded in one locked step. The classify call runs inside the slot, so classify calls also count against the four.
  - internal/notes/chunk.go:187-192: notes are read out of `w.slots`, which is in chunk order whatever order the calls finished in. internal/notes/chunk.go:57-66 (`writeParts`) calls the merge only after `run` returns.
  - Deadlines: the only `context.WithTimeout` in production code is the per-attempt one at internal/ai/transport.go:286 (I grepped all non-test .go files for WithTimeout/WithDeadline). The wave and merge run on the run's ctx (internal/notes/chunk.go:121-122 and :74).
  - internal/notes/generate.go:19-20 states the concurrent-use contract on `Transport`. `ai.Transport` holds only immutable fields, and `ExecRunner` is a stateless struct.
  - internal/runner/fake_runner.go:49, :134: FakeRunner gained a mutex. Recording a call and consuming its SeedSequence outcome happen in one critical section, and `Invocations()` returns a clone.
- Notes: I found no lost wakeups, deadlocks or data races. Broadcast runs under the lock and conditions are re-checked under the lock. A Ctrl-C waits for in-flight calls, which honour ctx, then returns ctx.Err(). Progress calls happen only under `w.mu`, so the "calls never overlap" contract in internal/notes/generate.go:25-26 holds. No drift from the plan.

TESTS:
- Status: Adequate
- Coverage (internal/notes/chunkwave_test.go):
  - :174, six calls: waits for exactly four in flight, then asserts max-in-flight == 4, six chunk calls and one merge. A bound of 1 or unbounded fan-out would fail it.
  - :198, three calls: all three in flight together. A serial implementation would fail it.
  - :215, no setting: a tripwire over `config.MetadataRows()` keys and descriptions for concurrency wording. It does not scan flags; reading settles the flag half, since the constant is unexported with one reader.
  - :226, out of order: parts are forced to end in order 3, 2, 1, deterministically through ended/gate channels. The test asserts the merge carries the notes in chunk order.
  - :275, merge last: 13 events, the merge last, all six ends before it.
  - :354, timeouts: a real ai.Transport with a 500ms per-attempt timeout over a paced runner (200ms calls), two rounds plus the merge (about 600ms). It asserts the notes are written, elapsed > timeout, and all 7 calls got a fresh deadline of about 500ms. A deadline spanning the wave or the whole generation would cut the second-round or merge calls' allotment below timeout-100ms and fail it.
  - internal/runner/fake_runner_test.go:289 proves concurrent calls each consume one SeedSequence outcome exactly once.
- Notes: Every test would fail if its behaviour broke. The waits are bounded by `waitLimit`, so a broken generator fails rather than hangs. Over-testing is mild: :275 reuses :174's setup to assert ordering. Not a defect.

CODE QUALITY:
- Project conventions: Followed. External test package, t.Parallel throughout, fakes rather than real subprocesses, and the AI goes through the Transport seam.
- SOLID principles: Good. The wave owns scheduling, the transport owns the per-attempt deadline and retry, and Progress stays a narrow interface.
- Complexity: Acceptable. The sync.Cond loop is compact, and is needed because halving grows the queue mid-wave.
- Modern idioms: Yes (slices.Clone/Index/Replace, wg.Go, min/max builtins).
- Readability: Good. The WHY-comments on `wave`, `run` and `send` match the code as built.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
