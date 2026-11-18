TASK: Document The Intentional Cross-Package Twin Of The Single-StageFailed Extraction Helpers (tick-df920b / notes-failure-output-ugly-and-uninformative-4-1)

ACCEPTANCE CRITERIA:
- Both helpers retain identical behaviour and signatures; no call site changes.
- onlyStageFailure and onlyStageFailureEvent each carry a comment naming the other as the intentional twin and stating the cross-package reason (different test packages) the duplication cannot be collapsed into one func.
- The comments are accurate to as-built (no claim of a single shared helper).
- All project gates pass: go build ./..., gofmt -l . prints nothing, go vet ./..., go test -race ./..., golangci-lint run reports 0 issues.

STATUS: Complete

SPEC CONTEXT:
This is the final analysis-cycle (T4) duplication note from the notes-failure-output-ugly-and-uninformative work unit. Two byte-identical StageFailed-extraction test helpers were authored independently across T2-3 and T3-2 and sit in different compilation units: onlyStageFailure in white-box package engine (notesfailurewiring_internal_test.go) and onlyStageFailureEvent in external package engine_test (release_priortag_test.go). The spec's resolution is explicitly NOT to consolidate — a single shared func would require either exporting a test helper into the production package (forbidden by the no-production-surface-pollution invariant) or relocating tests across the package boundary (out of scope, risky for the white-box wiring proof). The task is documentation only: cross-reference the twins so a future RecordingPresenter event-shape or "exactly one StageFailed" contract change is steered to both sites.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/notesfailurewiring_internal_test.go:48-73 (onlyStageFailure, package engine; new doc lines 50-56)
  - internal/engine/release_priortag_test.go:780-806 (onlyStageFailureEvent, package engine_test; new doc lines 782-789)
- Notes:
  - Commit 1d3149b is comment-only. The diff against both files adds ONLY doc-comment lines; the function signatures, bodies, and surrounding code are byte-for-byte unchanged.
  - Both signatures are identical: (t *testing.T, rec *presentertest.RecordingPresenter) presenter.StageFailure. Confirmed against as-built source.
  - Both bodies are behaviourally identical: t.Helper(), loop rec.Events, match Kind == presentertest.KindStageFailed, t.Fatalf on a second match, capture rec.Events[i].StageFailed into found, t.Fatalf when found == nil, return *found. The only differences are the documented-and-expected ones: function name, local capture var (f vs sf), and trivial fatal-message wording ("StageFailed" vs "StageFailed event"). The "byte-identical twin" framing in the comments is accurate to intent (same logic), and the comments do not overclaim character-for-character sameness.
  - No production (non-test) source changed: `git show 1d3149b --name-only` lists only the two *_test.go files plus tick/manifest bookkeeping. Verified no .go file outside *_test.go is in the commit.
  - No call site changes: all three call sites (notesfailurewiring_internal_test.go:36, :45, :176 for onlyStageFailure; release_priortag_test.go:771 for onlyStageFailureEvent) are untouched by the diff.
  - Package split verified directly: notesfailurewiring_internal_test.go is `package engine`; release_priortag_test.go is `package engine_test`. The cross-package reason stated in both comments is real.

TESTS:
- Status: Adequate (N/A by design)
- Coverage: No new test required — this documents existing test scaffolding and introduces zero behaviour. The existing engine suite continues to exercise both helpers via their unchanged call sites, which is the proof that behaviour is untouched.
- Notes: Tests were not executed (out of scope for review and forbidden by task rules). Behaviour-preservation is established structurally: the diff adds only comment lines, so the suite that compiled and ran before still compiles and runs identically.

CODE QUALITY:
- Project conventions: Followed. Matches CLAUDE.md "Comments" contract — heavy WHY-comments stating the constraint and the reasoning the code cannot show, kept true to as-built. The "Edit BOTH together" line encodes the maintenance contract; no false scope/phase claim. golang-documentation idiom (doc comment opens with the identifier name) is preserved on both helpers.
- SOLID principles: Good. No structural change; the deliberate, documented duplication is the correct trade-off given the forbidden alternatives.
- Complexity: Low. Comment-only change.
- Modern idioms: Yes. Unchanged helper code already uses index-range iteration and pointer capture to avoid copying the loop variable.
- Readability: Good. Both comments are symmetric: each names its twin, gives the twin's file, states the three reasons consolidation is impossible (invisibility across the package boundary, export-to-production forbidden, relocation out of scope), and closes with the "edit both together" trigger condition. Wording is mirrored without being a blind copy (each correctly describes its own side of the boundary — "this copy is in white-box package engine and is invisible to the external package" vs "this copy is in the external engine_test package and cannot see the unexported package engine helper").
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None. The comments are symmetric, accurate to as-built, and make no false consolidation claim; signatures/bodies/call sites are unchanged; no production source touched. All acceptance criteria met.
