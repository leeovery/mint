# Review Tasks: token-aware-diff-sizing (Cycle 1)

## Task 1: Make the chunk-wave deadline test deterministic
severity: medium
sources: test-surface-2

**Problem**: `TestGenerator_OverCap_WaveLongerThanTimeout_EachCallOwnDeadline_NotesWritten` (internal/notes/chunkwave_test.go:354, added in 2-6) pins one 2-6 criterion: each chunk call and the merge has its own per-attempt deadline, and no deadline spans the wave (§11.6). It does so through real wall-clock margins. `pacedAIRunner` sleeps 200ms while honouring a 500ms per-attempt deadline. The test then requires every call's deadline to sit at least 400ms past the call's runner entry (`allotted < timeout-100*time.Millisecond`, :379). The test was run as 20 concurrent race-built `notes.test` processes on 10 cores, and it failed 2 of 300 runs. One failure was `ai command timed out: context deadline exceeded` (:364) and the other was `call 4 was given 353.810875ms by its deadline` (:380). So the `go test -race ./...` gate intermittently goes red on a loaded machine or CI runner when there is no defect. A repeat at the same load failed none of 300 runs, and 450 runs at one process per core failed none: the flake is rare, and no clean run rules it out.

**Solution**: Replace the margins with orderings that hold by construction. Do not re-tune callTime and timeout. Re-tuned margins stay probabilistic and slow the suite, and no run can show they stopped flaking. What the criterion actually tests is which deadline each call gets, and an ordering pins that exactly.

- **The runner**: it keeps sleeping callTime but stops honouring its context, so no scheduler stall can time a call out. This holds because `ai.Transport.attempt` returns a successful `RunWith` result without re-checking the context. It records each call's absolute deadline and whether the call is a chunk call or the merge.
- **The checks kept**: the checks that are already deterministic stay.
  - The body is the merge output.
  - All seven calls carry a deadline.
  - The wave runs longer than the timeout: three serial steps of callTime exceed the timeout.
  - No deadline lies more than the timeout past its call's runner entry.
- **The check replaced**: the lower bound that jitter can cross (`allotted < timeout-100*time.Millisecond`) is removed. In its place come two orderings:
  - The merge's deadline must lie at least callTime past every chunk call's deadline. That gap holds by construction, because the merge starts only after every chunk call has slept callTime. A deadline shared across the wave gives equal deadlines and fails it.
  - Each second-round chunk call's deadline (the two chunk calls after the first four to enter the runner) must lie at least callTime past the earliest first-round chunk call's deadline. That gap also holds by construction: with at most four calls in flight, a second-round call starts only after a first-round call has slept callTime. A deadline shared across the chunk calls alone, with the merge still given its own, gives equal chunk deadlines and fails it. The old lower bound caught that case, so this check keeps the coverage.

Because the runner now ignores its context, the notes being written no longer tells a per-call deadline from a shared one. The two deadline orderings do that, and the test's comment says so.

**Outcome**: The test passes under any CPU load as long as every call gets a fresh per-attempt deadline. It fails when a deadline spans the wave, or spans the chunk calls.

**Acceptance Criteria**:
- [ ] Given the as-built transport, which takes a fresh per-attempt deadline for every call, the test passes. The body is the merge output, all seven calls carry a deadline, and the wave runs longer than the timeout. No deadline lies more than the timeout past its call's runner entry, the merge's deadline lies at least callTime past each of the six chunk calls' deadlines, and each of the two second-round chunk calls' deadlines lies at least callTime past the earliest first-round chunk call's deadline (§11.6, §8.1, §8.5)
- [ ] With a trial change that gives the calls one deadline spanning the wave and the merge, in place of the per-attempt one, the test fails on a deadline ordering. With the trial reverted it passes (§11.6)
- [ ] With a trial change that gives the six chunk calls one shared deadline while the merge keeps its own per-attempt deadline, the test fails on the second-round ordering. With the trial reverted it passes (§11.6)
- [ ] Rule: nothing in the test can fail on scheduler delay alone. The runner never returns a context error, so no call can surface "ai command timed out", and no check bounds from below the time a call has left at its runner entry (§11.6)
- [ ] Run under the contention that reproduced the flake (a race-built `internal/notes` test binary run as twice as many concurrent processes as cores, `-test.count=15`), the test fails in none of the runs
- [ ] Rule: no production file changes, and the five CLAUDE.md gates pass

**Do**:
- The change lives in `internal/notes/chunkwave_test.go` alone: `pacedAIRunner` (:306-352) and the test (:354-383). The per-attempt `context.WithTimeout` (internal/ai/transport.go:282-290) and the wave that sets no deadline of its own (internal/notes/chunk.go:118-193) are what the test pins, and they stay untouched
- Keep `callTime` at 200ms and `timeout` at 500ms
- `pacedAIRunner.RunWith` sleeps callTime without watching its context and never returns a context error. For each call it records, under its lock, the order of runner entry, the runner entry time, the absolute deadline (or that the call had none), and whether the prompt is a chunk prompt or the merge
- Keep the four deterministic checks: the body is the merge output; seven calls carry a deadline and none lacks one; the wave runs longer than the timeout; no deadline lies more than the timeout past its call's runner entry
- Remove the `allotted < timeout-100*time.Millisecond` lower bound. Add the check that the merge's deadline lies at least callTime past each chunk call's deadline, and the check that each chunk call entering the runner after the first four has a deadline at least callTime past the earliest deadline among those first four
- The runner's comment states that it ignores its context on purpose, so the deadline orderings, not the notes being written, are what tell a per-call deadline from a shared one. The wording is the executor's
