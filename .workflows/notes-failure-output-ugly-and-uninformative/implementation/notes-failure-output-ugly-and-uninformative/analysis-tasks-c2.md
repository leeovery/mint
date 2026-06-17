---
topic: notes-failure-output-ugly-and-uninformative
cycle: 2
total_proposed: 2
---
# Analysis Tasks: notes-failure-output-ugly-and-uninformative (Cycle 2)

## Task 1: Document the intentional cross-package twin of the single-StageFailed extraction helpers
status: pending
severity: medium
sources: duplication

**Problem**: Two byte-identical "extract the single recorded StageFailed payload, fail if not exactly one" test helpers exist across the engine test packages, authored independently across two task boundaries (T2-3 and T3-2):
- `onlyStageFailure` — `package engine` (white-box), internal/engine/notesfailurewiring_internal_test.go:50-66
- `onlyStageFailureEvent` — `package engine_test` (external), internal/engine/release_priortag_test.go:782-798

Both take `(t *testing.T, rec *presentertest.RecordingPresenter)`, loop `rec.Events`, match `rec.Events[i].Kind == presentertest.KindStageFailed`, `t.Fatalf` on a second match, capture `rec.Events[i].StageFailed` into a found pointer, `t.Fatalf` when none was recorded, and return `*found`. The only differences are the function name, the local capture variable name (`f` vs `sf`), and trivial wording in the fatal messages. They can silently diverge if the RecordingPresenter event shape or the "exactly one StageFailed" contract changes — a change must be made in two places that look unrelated.

**Solution**: A single shared func is NOT viable here: the two helpers live in different compilation units (`package engine` vs `package engine_test`). An unexported helper in `package engine` test files is not visible to `package engine_test` files, and the only ways to truly share one source are to export a test helper into the production package (forbidden — no production-surface pollution) or to relocate tests across package boundaries (out of scope and risky for the white-box wiring proof). Therefore keep both copies but make the duplication INTENTIONAL and DOCUMENTED: add a short cross-reference comment to each helper naming its twin and the reason for the split (different test package), so a future editor knows both must move together. Do not change either helper's behaviour or signature.

**Outcome**: Each helper carries a comment pointing at its twin in the other package and stating the cross-package reason the duplication is deliberate. A reader (or a future change to the RecordingPresenter event shape) is steered to both sites rather than discovering the divergence after a silent drift. No behaviour change; all engine tests still pass.

**Do**:
1. Open internal/engine/notesfailurewiring_internal_test.go and locate `onlyStageFailure` (around line 48-66). Extend its doc comment to note that an intentional byte-identical twin, `onlyStageFailureEvent`, lives in the external `engine_test` package (internal/engine/release_priortag_test.go) and exists only because the two call sites are in different test packages — both must be edited together if the "exactly one StageFailed" contract or the RecordingPresenter event shape changes.
2. Open internal/engine/release_priortag_test.go and locate `onlyStageFailureEvent` (around line 780-798). Extend its doc comment with the symmetric cross-reference back to `onlyStageFailure` in the internal `package engine` test file, stating the same cross-package reason.
3. Keep the comments WHY-comments true to as-built: state the cross-package constraint and the "edit both together" contract; do not claim a consolidation that did not happen.
4. Run the gates: `go build ./...`, `gofmt -l .` (must print nothing), `go vet ./...`, `go test -race ./internal/engine/...`, `golangci-lint run` (0 issues).

**Acceptance Criteria**:
- Both helpers retain identical behaviour and signatures; no call site changes.
- `onlyStageFailure` and `onlyStageFailureEvent` each carry a comment naming the other as the intentional twin and stating the cross-package reason (different test packages) the duplication cannot be collapsed into one func.
- The comments are accurate to as-built (no claim of a single shared helper).
- All project gates pass: `go build ./...`, `gofmt -l .` prints nothing, `go vet ./...`, `go test -race ./...`, `golangci-lint run` reports 0 issues.

**Tests**:
- No new test needed (this is documentation of existing test scaffolding); the existing engine test suite (`go test -race ./internal/engine/...`) must continue to pass unchanged, proving the helpers' behaviour is untouched.

## Task 2: Make GenerationError.Error()/ExitCode honest about its dual provenance
status: pending
severity: low
sources: architecture

**Problem**: `GenerationError` (internal/ai/transport.go:56-69) carries `ExitCode int` documented as "the non-zero process exit status that classified this as bad content", and `Error()` renders `fmt.Sprintf("ai generation failed (exit %d)", e.ExitCode)`. The type has two construction sites: the non-zero-exit path (transport.go:208), where ExitCode is genuinely non-zero, and the empty/whitespace-body-after-retry path (transport.go:219), reached on a CLEAN (zero-exit) attempt whose body merely failed `isValid`. On the second path ExitCode is 0, so `Error()` renders the self-contradicting string "ai generation failed (exit 0)" — a zero exit labelled as the exit that classified the failure, and the field's "non-zero" annotation is false for one of the type's two legitimate inputs. This is diagnostic-quality only (notes.CauseText / failureMessage intercept the carrier and resolve the concise phrase before `Error()` is reached, so it never becomes the display Message), but `Error()` still surfaces in logs and the defensive failureMessage fallback, and the field contract is internally inconsistent.

**Solution**: Make the carrier honest about both shapes without touching routing or the carried Stdout/Stderr. Have `Error()` distinguish the two provenances: when `ExitCode == 0` render an empty-body variant (e.g. "ai generation failed (empty body)") instead of "(exit 0)"; when `ExitCode != 0` keep the existing "(exit %d)" string. Widen the `ExitCode` field doc to acknowledge both legitimate shapes — a non-zero exit that classified the failure, OR zero on the empty/whitespace-body path where the body alone classified it. Do NOT alter the two construction sites' routing, the carried Stdout/Stderr/ExitCode values, `Unwrap()`, or any classification logic. Keep the `Error()` string lowercase with no trailing punctuation per the project error idiom.

**Outcome**: `Error()` reports an accurate diagnostic for both construction paths — never "exit 0" — and the `ExitCode` field doc truthfully describes its dual provenance. The display Message path (CauseText/failureMessage → concise phrase) is unchanged, errors.Is(ErrGenerationFailed) routing is unchanged, and the content-agnostic transport invariants are preserved. Logs and the defensive fallback no longer carry a self-contradicting string.

**Do**:
1. In internal/ai/transport.go, update the `ExitCode` field doc (around line 63) to state that it is the non-zero process exit status that classified the failure on the non-zero-exit path, OR zero on the empty/whitespace-body path where the invalid body itself classified the failure.
2. Update `Error()` (around line 67-69) to branch on `ExitCode`: for `ExitCode != 0` keep `fmt.Sprintf("ai generation failed (exit %d)", e.ExitCode)`; for `ExitCode == 0` return a variant that does not claim an exit code (e.g. "ai generation failed (empty body)"). Keep both strings lowercase, no trailing punctuation.
3. Leave the two construction sites (transport.go:208 and transport.go:219), `Unwrap()`, classifyFatal, isValid, and all carried field values untouched.
4. Run the gates: `go build ./...`, `gofmt -l .` (must print nothing), `go vet ./...`, `go test -race ./internal/ai/...` and `go test -race ./...`, `golangci-lint run` (0 issues).

**Acceptance Criteria**:
- `GenerationError.Error()` never renders "(exit 0)": the empty/whitespace-body path (ExitCode == 0) produces a distinct, non-contradicting string; the non-zero-exit path is unchanged.
- The `ExitCode` field doc describes both legitimate provenances (non-zero exit OR zero on the empty-body path).
- `Error()` strings stay lowercase with no trailing punctuation.
- No change to routing, the carried Stdout/Stderr/ExitCode, `Unwrap()`, or classification logic; `errors.Is(err, ErrGenerationFailed)` still matches the carrier from both sites; context.Canceled passthrough unchanged.
- The display Message path (notes.CauseText / failureMessage → concise phrase) is unaffected; AC#2 concise-phrase tests still pass.
- All project gates pass: `go build ./...`, `gofmt -l .` prints nothing, `go vet ./...`, `go test -race ./...`, `golangci-lint run` reports 0 issues.

**Tests**:
- Add/extend a transport test asserting `(&GenerationError{Stdout: "", Stderr: "", ExitCode: 0}).Error()` returns the empty-body variant (no "(exit 0)") and that `(&GenerationError{ExitCode: 1}).Error()` still returns "ai generation failed (exit 1)".
- A regression assertion that `errors.Is((&GenerationError{ExitCode: 0}).err-form..., ErrGenerationFailed)` (via Unwrap) still holds for the zero-exit carrier, proving routing is preserved.
