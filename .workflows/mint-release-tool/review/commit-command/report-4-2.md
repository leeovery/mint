TASK: 4-2 — `e` empty-save discards edit; gate re-renders with prior message preserved

ACCEPTANCE CRITERIA:
- An empty `e` save discards the edit and re-renders with the prior message preserved unchanged — engine re-calls ShowMessage (prior body) then Prompt (same y/n/e/r gate)
- A whitespace-only `e` save is treated as empty (per the 3-2 editor contract) and discarded the same way
- A quit/abort with no content under `e` is treated as empty and preserves the prior message
- Repeated `e` then empty-save preserves the message current before that `e` (last non-empty candidate, back to the original generated message)
- `e` is never a message source — no empty-e-save path commits; `e` can never produce an empty commit
- An empty `e` save is NOT treated as an abort/no-op (distinct from the Phase 3 fallback) — the gate re-renders, the run continues
- The emptiness rule is consumed from 3-2 (whitespace-only / no content; no #-comment stripping), not re-defined

STATUS: Complete

SPEC CONTEXT:
specification.md — "Interactive Review Gate → Choice mapping (e / edit)": "An empty save under `e` discards the edit and re-renders the gate with the prior message preserved — `e` is a refinement step, never a message source, so it can never produce an empty commit." The "$EDITOR Fallback — Path Semantics" section establishes the contrasting Phase 3 rule (under the fallback, no message exists yet, so an empty/aborted save IS a true no-op abort). Task 4-2 is the empty-save counterpart to 4-1's non-empty loop-back; both share the engine-owned ShowMessage→Prompt gate loop and the single 3-2 whitespace-only emptiness rule.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/commit/run.go:537-577 — the `e` (ChoiceEdit) branch of reviewLoop. The decisive line is run.go:574-576: `if ok && !isEmptyEditorBuffer(saved) { body = saved }`. On an empty/whitespace-only save OR an aborted editor (ok=false), `body` is left UNCHANGED, so the unconditional loop-back re-renders the prior message via ShowMessage→Prompt (run.go:518-520). This single guard implements discard + preserve for all three empty shapes (empty string, whitespace-only, quit/abort) at once.
  - internal/commit/run.go:459-460 — isEmptyEditorBuffer is `strings.TrimSpace(saved) == ""`, the single 3-2 whitespace-only rule, shared verbatim with the Phase 3 runEditorFallback consumer (run.go:433). No second emptiness definition; no #-comment stripping (correct — the `e` buffer carries only the real message).
  - internal/commit/editor_open.go:79-88 — OpenEditor maps a launched-but-failed editor (quit/abort, e.g. `:cq`) to ("", false, nil), which the `e` branch reads as ok=false → discard. A missing binary (ErrCommandNotFound) is distinguished and surfaced (graceful-degrade is 4-3, correctly out of scope here).
- Notes: The implementation is unified and minimal — the `e` branch does not special-case the three empty variants; they all fall through the one `ok && !isEmpty` guard. "Repeated e then empty-save preserves the last non-empty candidate" is an emergent property of `body` being a single carried-forward local (a non-empty save overwrites it; an empty save leaves it), needing no extra code. No drift from the plan or spec.

TESTS:
- Status: Adequate
- Location: internal/commit/run_edit_empty_test.go (seven tests, one per acceptance-criteria test name)
- Coverage:
  - TestRun_EditEmptySaveDiscardsEditReRendersWithPriorMessage — asserts the exact event order (ShowMessage→Prompt→Suspend/Resume→ShowMessage→Prompt) and that BOTH ShowMessage bodies are the prior generated body; final `y` commits the prior body verbatim. Directly verifies AC1.
  - TestRun_EditWhitespaceOnlySaveTreatedAsEmpty — table-driven over two whitespace shapes ("   \n", "  \n\t\n  "); each preserves the prior body. Verifies AC2 and the 3-2-rule reuse (AC7).
  - TestRun_EditQuitWithNoContentPreservesPriorMessage — launchErr=errExitOne models the quit/abort (ok=false); re-render + prior-body commit. Verifies AC3.
  - TestRun_EditRepeatedThenEmptySavePreservesLastNonEmptyCandidate — e(save X)→e(empty)→y; asserts second editor pre-fill is X (not the original) and the commit body is X. Verifies AC4 (last non-empty candidate).
  - TestRun_EditEmptySaveOnFirstEditPreservesGeneratedMessage — e(empty)→y; commit body is the original generated message. Verifies AC4's "back to the original generated message" leg.
  - TestRun_EditEmptySaveNeverCommits — exactly one commit (on the final `y`) and zero `git add`; the empty-save iteration commits nothing. Verifies AC5.
  - TestRun_EditEmptySaveIsNotAnAbort — Run returns nil, a second gate renders still offering y/n/e, and the commit is reached. Verifies AC6 (re-render, not abort) — the right observable for "not an abort" given an abort would surface a non-nil error and stop before re-render/commit.
- Notes: Tests verify behaviour through the recorder (event kinds, ShowMessage bodies, gate choice sets via Has) and the git sink (commit stdin, add invocations) — not implementation internals. The editorRunner harness (editor_open_test.go:24-75) faithfully models per-launch saves and records preFills, so the "pre-fill is the prior candidate" assertions are real evidence, not tautologies. Balanced: not under-tested (every AC and both empty edge variants covered, including the distinction from the Phase 3 no-op via a nil-vs-error assertion), and not over-tested (the seven tests target distinct facets; the few overlapping prior-body-commit assertions are each anchoring a different scenario, not redundant re-checks of one path).

CODE QUALITY:
- Project conventions: Followed. Table-driven whitespace test, t.Parallel throughout, testify-free behavioural assertions consistent with the package. The hand-built commitReviewGate literal (Subject "message") is correctly reused, and the editor-emptiness rule is genuinely shared rather than duplicated.
- SOLID principles: Good. reviewLoop owns the gate-loop ordering (single responsibility); isEmptyEditorBuffer is the one emptiness authority (DRY — no second rule); OpenEditor owns launch/abort classification, keeping the `e` branch declarative.
- Complexity: Low. The empty-save case adds zero new branches — it is the fall-through of the existing `ok && !isEmpty` adopt-guard. The whole discard-and-preserve behaviour rides on one carried-forward `body` local.
- Modern idioms: Yes. strings.TrimSpace for emptiness, errors.Is for sentinel routing, an unconditional `for` with explicit early returns.
- Readability: Good. The run.go:561-573 comment block precisely names the adopt-vs-discard split and the Phase-3 distinction; intent is self-evident at the call site.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
