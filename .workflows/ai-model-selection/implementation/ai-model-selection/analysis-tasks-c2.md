---
topic: ai-model-selection
cycle: 2
total_proposed: 1
---
# Analysis Tasks: AI Model Selection (Cycle 2)

## Task 1: Extract the duplicated context-deadline CommandRunner spy into a shared test double
status: pending
severity: medium
sources: duplication

**Problem**: The cycle-1 work introduced a `runner.CommandRunner` spy that records the invoked argv and whether the handed-in context carries a deadline (`_, d.hadDeadline = ctx.Deadline()`), returning a fixed non-empty body. It exists because `runner.FakeRunner` discards the context and so cannot observe the timeout-vs-no-deadline behaviour these tests assert. The identical ~25-line struct plus its four method implementations (`RunWith`/`Run`/`RunInteractive`/`RunInDir`) was authored three independent times: `internal/aitransport/aitransport_test.go:25-54` and `internal/engine/release_aitransport_internal_test.go:26-46` are byte-identical (`type deadlineRunner`), `internal/commit/run_aitransport_internal_test.go:24-53` is the same code renamed to `deadlineCommitRunner`, and `internal/engine/regenerate_fresh_aitransport_internal_test.go` reuses the engine package's copy. The "FakeRunner discards the context, so these proofs use a tiny deadline-recording runner" comment is copy-pasted verbatim across all four sites. Three independent copies of a non-trivial spy is a copy-paste-drift hazard: a change to the seam (a new `CommandRunner` method, or making the spy record stdin) must be made in three places and can silently diverge. Riding along is the one-line helper `func durationPtr(d time.Duration) *time.Duration { return &d }`, independently re-authored in the same three test files (`aitransport_test.go:54`, `release_aitransport_internal_test.go:131`, `run_aitransport_internal_test.go:53`) because `config.Config.Timeout`/`ai.Config.Timeout` are now `*time.Duration`.

**Solution**: Extract the spy once into the existing `runner` package's test-double surface (alongside `FakeRunner`) as an exported `DeadlineRecordingRunner`, since it is generic `CommandRunner` instrumentation with no engine/commit/aitransport knowledge and every caller already imports `runner`. A non-test production-package home is required because Go test helpers in `_test.go` files are not visible across packages — `FakeRunner` is already shipped this way as the precedent. Have all three sites consume the shared double; collapse the two `engine`-package copies to the single shared reference. Fold `durationPtr` into the same shared test-support extraction (e.g. export it next to the shared runner spy) so the new timeout test ergonomics live in one place. Extract the existing spy and helper verbatim — add no new behaviour.

**Outcome**: One exported deadline-recording `CommandRunner` spy (and one shared `*time.Duration` pointer helper) lives in the `runner` package next to `FakeRunner`; the per-package copies in `aitransport`, `engine`, and `commit` are deleted and replaced with references to the shared double. A future change to the `CommandRunner` seam touches exactly one spy implementation. No production behaviour changes and the existing timeout-vs-no-deadline test assertions are unchanged.

**Do**:
1. In the `runner` package (the production-package test-double surface that already exports `FakeRunner`), add an exported `DeadlineRecordingRunner` type that implements the full `CommandRunner` interface (`RunWith`/`Run`/`RunInteractive`/`RunInDir`), recording the invoked argv and `_, hadDeadline = ctx.Deadline()`, returning the same fixed non-empty body the current copies return. Port the existing struct and method bodies verbatim — do not change the recorded fields or return shape.
2. Add an exported pointer helper equivalent to `durationPtr` (e.g. a `DurationPtr` function) in the same shared location.
3. In `internal/aitransport/aitransport_test.go`, delete the local `deadlineRunner` struct/methods and the local `durationPtr`, and update the tests to use the shared `runner` double and helper.
4. In `internal/engine/release_aitransport_internal_test.go` and `internal/engine/regenerate_fresh_aitransport_internal_test.go`, delete the duplicated/shared engine-package copy and the local `durationPtr`; route both to the shared `runner` double and helper.
5. In `internal/commit/run_aitransport_internal_test.go`, delete the `deadlineCommitRunner` struct/methods and the local `durationPtr`; route to the shared `runner` double and helper.
6. Remove the now-orphaned copy-pasted comment blocks; keep a single explanatory comment on the shared double stating why it exists (FakeRunner discards the context).
7. Run the gates: `go build ./...`, `gofmt -l .` (must print nothing), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues).

**Acceptance Criteria**:
- A single exported deadline-recording `CommandRunner` spy exists in the `runner` package alongside `FakeRunner`; no `deadlineRunner`/`deadlineCommitRunner` struct definition remains in `aitransport`, `engine`, or `commit` test files.
- A single shared `*time.Duration` pointer helper replaces the three `durationPtr` redeclarations in the new test files; no local `durationPtr` definition remains in those three files.
- The shared double implements the full `CommandRunner` surface (`RunWith`/`Run`/`RunInteractive`/`RunInDir`) and records argv plus the context-deadline-present flag exactly as the current copies do.
- No production (non-test) code path changes; only test files and the `runner` package's test-double surface are touched.
- All gates pass: `go build ./...`, `gofmt -l .` prints nothing, `go vet ./...`, `go test -race ./...`, `golangci-lint run` reports 0 issues.

**Tests**:
- The existing timeout-vs-no-deadline proofs in `internal/aitransport`, `internal/engine` (release + regenerate-fresh), and `internal/commit` continue to pass unchanged when re-pointed at the shared double — the deadline-present assertions and argv assertions remain exact.
- A test in the `runner` package exercises the shared `DeadlineRecordingRunner`: it records argv and reports `hadDeadline=true` when invoked with a deadline-bearing context and `hadDeadline=false` with a plain context, and returns the fixed non-empty body — proving the extracted spy behaves identically to the originals.
</content>
