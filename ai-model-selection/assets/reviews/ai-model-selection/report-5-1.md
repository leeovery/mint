TASK: ai-model-selection-5-1 — Extract the duplicated context-deadline CommandRunner spy into a shared test double

ACCEPTANCE CRITERIA:
- A single exported deadline-recording CommandRunner spy exists in the runner package alongside FakeRunner; no deadlineRunner/deadlineCommitRunner struct definition remains in aitransport, engine, or commit test files.
- A single shared *time.Duration pointer helper replaces the three durationPtr redeclarations; no local durationPtr definition remains in those three files.
- The shared double implements the full CommandRunner surface (RunWith/Run/RunInteractive/RunInDir) and records argv plus the context-deadline-present flag exactly as the current copies do.
- No production (non-test) code path changes; only test files and the runner package's test-double surface are touched.
- All gates pass: go build, gofmt -l . prints nothing, go vet, go test -race, golangci-lint run 0 issues.

STATUS: Complete

SPEC CONTEXT:
The spec (specification.md lines 95-102) makes the deadline-present-vs-absent distinction load-bearing: a resolved timeout of 0 must reach the transport as a deadline-free context (skip context.WithTimeout entirely), while a positive value must reach it as a context carrying a deadline. The "no deadline only ever reachable by explicit 0, never by a wiring site omitting the field" invariant is what the timeout-vs-no-deadline transport proofs in aitransport/engine/commit exist to defend. The spy under review is the only test instrument that can observe this, because runner.FakeRunner discards the context. This task is pure test-support de-duplication arising from analysis cycle 2 — no spec behaviour is altered.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/runner/deadline_recording_runner.go:17-67 — exported DeadlineRecordingRunner + DurationPtr helper.
  - Consumers re-pointed: internal/aitransport/aitransport_test.go:95, internal/engine/release_aitransport_internal_test.go:35/66/89, internal/engine/regenerate_fresh_aitransport_internal_test.go:37/68/91, internal/commit/run_aitransport_internal_test.go:36/67/92.
- Notes:
  - Verbatim port confirmed against the parent commit (67327e5~1): field recording (name/args/_, hadDeadline = ctx.Deadline()), the stdin drain, the fixed body "a non-empty body\n", and the RunInteractive/RunInDir no-ops are identical to the originals. Return shape and recorded fields unchanged.
  - No leftover duplicates: grep across the tree for deadlineRunner / deadlineCommitRunner / `func durationPtr` / `durationPtr(` returns zero hits. All three former struct sites and all three local durationPtr helpers are deleted; both engine-package copies collapse to the single shared reference.
  - Necessary, justified adaptation: the originals read the spy's unexported fields directly (same-package access); the shared double crossed a package boundary, so observation now goes through exported Name()/Args()/HadDeadline() accessors. This is a behaviour-preserving change the task's "exported" mandate requires, not drift.
  - DurationPtr is correctly placed and matches the original one-liner; all three former call sites now route through runner.DurationPtr.
  - Production-only-untouched criterion holds: the only non-_test.go file added is internal/runner/deadline_recording_runner.go, which is a test-double surface alongside FakeRunner (the established precedent for shipping a cross-package test double in a production file). No engine/commit/aitransport/ai production path changed (commit 67327e5 touches only test files + the runner double + bookkeeping).

TESTS:
- Status: Adequate
- Coverage:
  - internal/runner/deadline_recording_runner_test.go directly exercises the extracted spy: table-driven over deadline-bearing vs plain context, asserting HadDeadline() true/false, argv recording (Name/Args), and the fixed non-empty body. TestDurationPtr pins the pointer helper (non-nil, dereferences to the input). This is exactly the proof the task asked for ("proving the extracted spy behaves identically to the originals").
  - The four downstream transport-wiring proofs (aitransport, engine release + regenerate-fresh, commit) continue to assert exact argv and exact deadline-present flags through the shared double — verified the assertions remain exact (e.g. aitransport_test.go:112-114 HadDeadline()==wantDead with timeout=0→false / positive→true).
- Notes:
  - Not over-tested: the runner-package test is two focused cases plus one helper test; downstream proofs were not duplicated, they were re-pointed. No redundant assertions.
  - Not under-tested: both deadline branches (the spec-critical 0 vs positive distinction), argv, and the body are all covered. The test would fail if the spy stopped recording the deadline (the whole point of its existence).

CODE QUALITY:
- Project conventions: Followed. Lives in internal/runner next to FakeRunner (CLAUDE.md: runner is the ONLY subprocess surface, FakeRunner is its double) — correct home. External test package (runner_test), t.Parallel() throughout, table-driven, t.Context(). Exact-argv assertions per the test idioms. Single explanatory WHY-comment on the double; the copy-pasted per-site comment blocks were removed and replaced by tailored one-liners at each call site.
- SOLID principles: Good. Single responsibility (record argv + deadline presence); satisfies the full CommandRunner interface with a compile-time assertion (var _ CommandRunner) — interface conformance is pinned.
- Complexity: Low. Trivial recorder; no branching beyond the nil-stdin guard.
- Modern idioms: Yes. context.WithDeadline + t.Context() in tests, _, ok := ctx.Deadline() idiom preserved.
- Readability: Good. Godoc on the type and every accessor; the WHY-comment states precisely why it exists (FakeRunner discards the context) and why it ships in a production file (cross-package _test.go invisibility).
- Issues: None blocking. Thread-safety: the double carries no mutex, but this matches the FakeRunner precedent (also mutex-free) and every call site constructs a fresh &runner.DeadlineRecordingRunner{} per (sub)test and never shares it across goroutines, so the unsynchronised fields are safe as used.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/runner/deadline_recording_runner.go:17 — DeadlineRecordingRunner overwrites name/args/hadDeadline on every call and has no concurrency guard; FakeRunner shares this limitation. If a future test ever drives one instance from multiple goroutines (or asserts a full call history rather than last-call), decide whether to add a sync.Mutex and/or accumulate an []Invocation slice like FakeRunner does. No action needed for current usage (one fresh instance per subtest, single call each).
- [do-now] internal/runner/deadline_recording_runner.go:64 — DurationPtr's godoc reads "returns a pointer to d" where d is the parameter name shadowing the receiver name used elsewhere in the file; consider renaming the param to `dur` (or rephrasing to "to the given duration") so the doc reads cleanly in isolation. Pure wording, no logic impact.
