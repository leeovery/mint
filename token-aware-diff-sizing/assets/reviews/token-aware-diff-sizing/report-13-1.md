TASK: token-aware-diff-sizing-13-1 (tick-1156d4) — Make the chunk-wave deadline test deterministic

ACCEPTANCE CRITERIA:
- Given the as-built transport, which takes a fresh per-attempt deadline for every call, the test passes. The body is the merge output, all seven calls carry a deadline, and the wave runs longer than the timeout. No deadline lies more than the timeout past its call's runner entry, the merge's deadline lies at least callTime past each of the six chunk calls' deadlines, and each of the two second-round chunk calls' deadlines lies at least callTime past the earliest first-round chunk call's deadline (§11.6, §8.1, §8.5)
- With a trial change that gives the calls one deadline spanning the wave and the merge, in place of the per-attempt one, the test fails on a deadline ordering. With the trial reverted it passes (§11.6)
- With a trial change that gives the six chunk calls one shared deadline while the merge keeps its own per-attempt deadline, the test fails on the second-round ordering. With the trial reverted it passes (§11.6)
- Rule: nothing in the test can fail on scheduler delay alone. The runner never returns a context error, so no call can surface "ai command timed out", and no check bounds from below the time a call has left at its runner entry (§11.6)
- Run under the contention that reproduced the flake (a race-built internal/notes test binary run as twice as many concurrent processes as cores, -test.count=15), the test fails in none of the runs
- Rule: no production file changes, and the five CLAUDE.md gates pass

STATUS: complete

SPEC CONTEXT: §11.6 — the per-attempt timeout applies to each call on its own (every chunk call, the merge, the classify call); there is no overall deadline across a wave, so a wave takes roughly its rounds of chunk calls plus the merge. §8.1 — chunk calls run in parallel, then exactly one merge. §8.5 — at most four chunk calls in flight, a fixed bound. The test pins that each call gets a fresh deadline and that none spans the wave or the chunk calls.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/chunkwave_test.go:307-312 — runner doc comment: ignores its context on purpose, so no stall can time a call out
  - internal/notes/chunkwave_test.go:320-326 — pacedCall records entry time, absolute deadline, whether one was present, and chunk-vs-merge
  - internal/notes/chunkwave_test.go:328-345 — RunWith stamps entry, reads the deadline, records under the lock, then time.Sleep(callTime) with no context select; the only error path is io.ReadAll over the transport's strings.Reader
  - internal/notes/chunkwave_test.go:359-365 — test doc comment: every check holds by construction; the deadline orderings, not the notes, tell a per-call deadline from a shared one
  - internal/notes/chunkwave_test.go:369-370 — timeout stays 500ms, callTime 200ms
  - internal/notes/chunkwave_test.go:380-406 — kept checks: merge-output body, elapsed > timeout, every call has a deadline, allotted <= timeout (upper bound only), 6 chunk calls + 1 merge
  - internal/notes/chunkwave_test.go:408-413 — merge deadline >= each chunk deadline + callTime
  - internal/notes/chunkwave_test.go:414-424 — chunk deadlines sorted; the two latest must lie >= callTime past the earliest
- Notes:
  - Production code untouched: commit 608ccc0c and the follow-up 7257efb4 each touch only internal/notes/chunkwave_test.go. The pinned production shape was checked: internal/ai/transport.go:282-291 (attempt applies context.WithTimeout per attempt and returns RunWith's result without re-checking the context) and :229-230 (a valid body returns immediately); internal/notes/chunk.go:168-197 (the wave sets no deadline of its own; a slot frees at :225 only after Generate returns).
  - By-construction proof, from reading: deadline_i = t_i + timeout, where t_i is the WithTimeout call and t_i <= entered_i, so allotted <= timeout. The merge's t_m comes after every chunk call returned, which is >= t_c + callTime, so merge gap >= callTime. A second-round call takes the slot of a first-round call k after k slept, so t_s >= t_k + callTime >= min first-round t + callTime. The earliest of the six deadlines is therefore a first-round one, and at least two deadlines (the second-round ones) sit >= callTime past it, so the two latest do too. ctx.Deadline() keeps WithTimeout's monotonic reading, so Sub/Compare are monotonic, and time.Sleep guarantees at least callTime. Elapsed is >= 3 x callTime = 600ms > 500ms. None of these checks depends on a margin.
  - Sound divergence from the Do wording: the Do said to identify second-round calls by "order of runner entry". The executor instead sorts by deadline and checks the two latest (comment at :414-417). Recording order, and even entry order, can misplace a first-round call that stalls for callTime or longer before it records. The sort-based check is implied by per-call deadlines under any interleaving, and it still fails when all chunk deadlines are equal. This is an improvement, not a loss.
  - The runner comment no longer carries the "orderings, not notes, tell per-call from shared" sentence. 7257efb4 moved it to the test's doc comment (:361-365), which the Solution section endorses ("the test's comment says so"). The Do left the wording to the executor.
  - Considered and set aside: awaitGeneration's 5s waitLimit is a hang guard shared by every wave test. It sits more than 8x above the ~600ms nominal wave and is not a deadline-allotment check, so it is not reported.

TESTS:
- Status: Adequate
- Coverage: The one named test, TestGenerator_OverCap_WaveLongerThanTimeout_EachCallOwnDeadline_NotesWritten, carries all four kept checks and both new orderings. A wave-spanning deadline makes all seven deadlines equal, which fails :410 (and :421). A chunk-shared deadline with the merge keeping its own makes the six chunk deadlines equal, which fails :421, while the merge gap (>= 2 x callTime) and the allotted bound still pass. That is the coverage the removed lower bound used to give.
- Notes: Focused, with no redundant assertions. The second-round failure message calls the two latest-deadline calls "second-round chunk call 1/2". Under a passing implementation the check can't fire, and under the failure it targets all deadlines are equal, so the label misleads nobody.

CODE QUALITY:
- Project conventions: Followed (external test package, t.Parallel, FakeRunner-seeded git, real ai.Transport over a fake CommandRunner, lock-guarded recording)
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes (slices.SortFunc with time.Time.Compare)
- Readability: Good
- Issues: None. Comments at :307-312, :320, :359-365 and :414-417 were checked against the code and against transport.go:282-291. All hold.

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "Given the as-built transport, which takes a fresh per-attempt deadline for every call, the test passes" — reading settles that every listed check is present and holds by construction. A run of `go test -race ./internal/notes -run TestGenerator_OverCap_WaveLongerThanTimeout_EachCallOwnDeadline_NotesWritten` would confirm the pass.
- "With a trial change that gives the calls one deadline spanning the wave and the merge, in place of the per-attempt one, the test fails on a deadline ordering. With the trial reverted it passes" — needs the trial applied and the test run, to observe the :410 merge-ordering failure. Reading predicts it: all seven deadlines are equal and the runner ignores its context, so nothing earlier fails, though a generator-level ctx trial under heavy load could error first.
- "With a trial change that gives the six chunk calls one shared deadline while the merge keeps its own per-attempt deadline, the test fails on the second-round ordering. With the trial reverted it passes" — needs the trial applied and the test run, to observe the :421 failure with :410 passing.
- "Run under the contention that reproduced the flake (a race-built internal/notes test binary run as twice as many concurrent processes as cores, -test.count=15), the test fails in none of the runs" — needs `go test -race -c ./internal/notes` and 2 x nproc concurrent runs of the binary with `-test.run TestGenerator_OverCap_WaveLongerThanTimeout_EachCallOwnDeadline_NotesWritten -test.count=15`, with zero failures observed.
- "Rule: no production file changes, and the five CLAUDE.md gates pass" — the no-production-change half is settled by reading (608ccc0c and 7257efb4 touch only internal/notes/chunkwave_test.go). The gates half needs `go build ./...`, `gofmt -l .`, `go vet ./...`, `go test -race ./...` and `golangci-lint run` run on the tree.
