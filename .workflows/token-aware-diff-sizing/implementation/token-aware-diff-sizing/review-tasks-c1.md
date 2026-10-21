# Review Tasks: token-aware-diff-sizing (Cycle 1)

## Task 1: Make the chunk-wave deadline test deterministic
severity: medium
sources: test-surface-2

**Problem**: `TestGenerator_OverCap_WaveLongerThanTimeout_EachCallOwnDeadline_NotesWritten` (internal/notes/chunkwave_test.go:354, added in 2-6) pins one 2-6 criterion: each chunk call and the merge has its own per-attempt deadline, and no deadline spans the wave (§11.6). It does so through real wall-clock margins. `pacedAIRunner` sleeps 200ms while honouring a 500ms per-attempt deadline. The test then requires every call's deadline to sit at least 400ms past the call's runner entry. The test was run as 20 concurrent race-built `notes.test` processes on 10 cores, and it failed 2 of 300 runs. One failure was `ai command timed out: context deadline exceeded` and the other was `call 4 was given 353.810875ms by its deadline`. So the `go test -race ./...` gate intermittently goes red on a loaded machine or CI runner when there is no defect.

**Solution**: Replace the margins with orderings that hold by construction. Do not re-tune callTime and timeout. Re-tuned margins stay probabilistic and slow the suite, and no run can show they stopped flaking. What the criterion actually tests is which deadline each call gets, and an ordering pins that exactly.

- **The runner**: it keeps sleeping callTime but stops honouring its context, so no scheduler stall can time a call out. This holds because `ai.Transport.attempt` returns a successful `RunWith` result without re-checking the context. It records each call's absolute deadline and whether the call is a chunk call or the merge.
- **The checks kept**: the checks that are already deterministic stay.
  - The body is the merge output.
  - All seven calls carry a deadline.
  - The wave runs longer than the timeout: three serial steps of callTime exceed the timeout.
  - No deadline lies more than the timeout past its call's runner entry.
- **The check replaced**: the lower bound that jitter can cross (`allotted < timeout-100*time.Millisecond`) is removed. In its place, the merge's deadline must lie at least callTime past every chunk call's deadline. That gap holds by construction, because the merge starts only after every chunk call has slept callTime. A deadline shared across the wave gives equal deadlines and fails it.

Because the runner now ignores its context, the notes being written no longer tells a per-call deadline from a wave-wide one. The deadline ordering does that, and the test's comment says so.

**Outcome**: The test passes under any CPU load as long as every call gets a fresh per-attempt deadline. It fails when a deadline spans the wave.
