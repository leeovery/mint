TASK: Task 2-3 — Wire the composed Output into both notes StageFailed surfacing paths (notes-failure-output-ugly-and-uninformative-2-3)

ACCEPTANCE CRITERIA (from plan):
1. surfaceAndUnwind AND surface both set StageFailure.Output = notesFailureOutput(cause); a generation-failed carrier populates Output at BOTH sites.
2. The two paths render identically for the same cause (forward release vs regenerate).
3. timeout / command-missing / diff-too-large produce empty StageFailure.Output (✗ line stands alone with the concise phrase).
4. resetAndAbort unchanged (no Output population); batch reportSkip/classifyNotesFailure unchanged.
5. padStage and StageFailed column layout unchanged; only pretty_test.go message-text assertions change; gate_forbidden_test.go and askline_test.go byte-for-byte untouched.
6. The wiring test asserts StageFailure.Output POPULATION via RecordingPresenter, not the rendered stream.
7. Gates pass.

STATUS: Complete

SPEC CONTEXT:
This is the consumption end of the three-task phase. Fix 1 (Phase 1) made *ai.GenerationError a typed carrier holding claude's captured Stdout/Stderr. Task 2-1 added notes.CauseText + the failureMessage concise-phrase branch (Fix 2). Task 2-2 added notesFailureOutput(cause) — the errors.As extraction/composition helper. Task 2-3 is the wiring: feed notesFailureOutput(cause) into StageFailure.Output at the two notes StageFailed surfacing helpers so claude's captured output finally reaches the screen below the ✗ line. Spec Scope & Affected Surfaces names exactly two in-scope sites (surfaceAndUnwind forward release, surface regenerate single-version/interactive) and explicitly puts resetAndAbort and the batch reportSkip path out of scope. Fix 3 keeps padStage; Acceptance Criteria #3 demands the two paths render identically.

IMPLEMENTATION:
- Status: Implemented (matches plan and spec exactly)
- Location:
  - internal/engine/release.go:1050-1058 — surfaceAndUnwind now sets Output: notesFailureOutput(cause) alongside Name + Message: failureMessage(cause). WHY-comment correctly states it is called for every stage (pre_tag/notes/record/preflight/tag) and is NOT special-cased, because the helper returns "" for any non-carrier cause.
  - internal/engine/release.go:1646-1658 — surface (regenerate) sets the same Output: notesFailureOutput(cause). WHY-comment explains the identical-render guarantee vs the forward path.
  - internal/engine/release.go:1622-1639 — notesFailureOutput (Task 2-2, unchanged here) is the helper being wired; confirmed it uses errors.As(*ai.GenerationError) and composes stdout-then-stderr.
  - internal/engine/release.go:1674-1683 — failureMessage (Task 2-1) supplies the concise phrase via notes.CauseText.
- Notes: Verifications against the plan's eight explicit checks:
  (a) Both sites set Output: notesFailureOutput(cause) alongside Name and Message: failureMessage(cause). CONFIRMED.
  (b) Generation-failed carrier populates Output at both sites; identical for the same cause. CONFIRMED by code and by TestSurface_PopulatesOutputIdenticallyToForwardPath (asserts Output AND Message equality across paths).
  (c) timeout/command-missing/diff-too-large produce empty Output (helper returns "" for non-carrier causes). CONFIRMED — notesFailureOutput returns "" when errors.As fails; phrases match notes.CauseText (release.go: "AI timed out", "AI tool not installed", "diff too large").
  (d) resetAndAbort UNCHANGED — internal/engine/regenerate_write.go:353 still builds StageFailure{Name, Message: failureMessage(cause)} with NO Output. Git confirms regenerate_write.go was not touched by the work unit. Batch reportSkip/classifyNotesFailure (regenerate_batch.go) NOT touched by the work unit (git confirms). CONFIRMED.
  (e) padStage and StageFailed layout UNCHANGED — internal/presenter/pretty.go not touched by the work unit (git confirms). Only one line changed in pretty_test.go across the entire work unit (the message literal in TestPrettyPresenterFailedRegenerateSuppressesClose, "claude failed" → "AI returned empty/invalid notes after retry"). gate_forbidden_test.go and askline_test.go are byte-for-byte untouched — git diff across the whole work unit shows neither file in any commit's file list. CONFIRMED.
  (f) TestPrettyPresenterStageFailedRendersCapturedOutputBelowGlyphLine (pretty_test.go:1115, tag/push case) left untouched — the work-unit pretty_test.go diff is a single line at ~615, nowhere near 1115. CONFIRMED.
  (g) The wiring test asserts StageFailure.Output POPULATION via RecordingPresenter (inspecting sf.Output / sf.Message on the recorded event), not the rendered stream. CONFIRMED — notesfailurewiring_internal_test.go reads recorded StageFailure fields, never a rendered string.
  (h) surfaceAndUnwind is NOT stage-name special-cased — it calls notesFailureOutput(cause) unconditionally for every stage. CONFIRMED.

TESTS:
- Status: Adequate (well-balanced; covers both required paths and all four causes; no over-testing)
- Location: internal/engine/notesfailurewiring_internal_test.go (new, 192 lines), plus the updated literal in internal/presenter/pretty_test.go:618.
- Coverage:
  - TestSurfaceAndUnwind_PopulatesOutputWithCapturedStdout — forward release path: carrier wrapped via wrapNotesAbort (the real abortError chain), stdout "Prompt is too long\n"; asserts Output == "Prompt is too long" (trailing newline trimmed) AND Message == concise phrase. Matches plan test #1.
  - TestSurface_PopulatesOutputIdenticallyToForwardPath — regenerate path via the shorter fmt.Errorf("generating notes: %w", carrier) chain; asserts Output and Message AND cross-path equality vs the forward path. Directly proves Acceptance Criteria #3 (identical render). Matches plan test #2.
  - TestNotesSurfacing_NonCarrierCausesYieldEmptyOutput — table over timeout / command-missing / diff-too-large, run through BOTH surfaceAndUnwind and surface; asserts Output == "" and the exact concise Message on both. Folds plan tests #3/#4/#5 into one focused table and covers both paths. Good consolidation, not over-testing.
  - TestResetAndAbort_NoOutputForGitFailure — drives resetAndAbort with committed=false (no Mutator call) and a wrapped git push error; asserts Output == "" and Message == cause.Error() fallback. Matches plan test #6, proving resetAndAbort inherited Fix 2 but not Fix 1 output.
- Test-double / no-real-subprocess hygiene: RecordingPresenter for assertions; surfaceAndUnwind driven with empty MadeState so Unwind no-ops (confirmed at unwind.go:111/130 — TagCreated false + Commits<=0 issue no Mutate and no Unwound event), so the Mutator is never invoked and exactly one StageFailed is recorded. resetAndAbort driven with committed=false for the same reason. wrapNotesAbort uses runner.NewFakeRunner() in abort mode (never invokes git). All idiomatic per project test conventions.
- onlyStageFailure helper correctly enforces "exactly one StageFailed" and carries a thorough WHY-comment about the intentional white-box/external twin (onlyStageFailureEvent). The duplication is justified and documented (package-boundary constraint), not a DRY violation to flag.
- Not under-tested: both required paths (surfaceAndUnwind + surface) covered; all four causes covered; identical-render contract explicitly asserted; resetAndAbort out-of-scope behaviour pinned; the empty-Output cases cover the per-cause Output contract.
- Not over-tested: stream-placement / composition-rule internals are NOT re-asserted here (correctly delegated to Task 2-2's helper tests and the pinned presenter test). The table consolidates the three non-carrier causes rather than spawning near-duplicate tests. No redundant assertions, no excess mocking.

CODE QUALITY:
- Project conventions: Followed. Output goes through presenter.StageFailure only; no fmt.Print/os.Stdout. errors.As traversal matches the AI-seam pattern. WHY-comments are present, accurate, and true to as-built (CLAUDE.md comment contract). External-vs-white-box test split is respected; the white-box file lives in package engine deliberately (it drives unexported surface/surfaceAndUnwind/resetAndAbort).
- SOLID: Good. Single display-derivation seam (failureMessage) and single extraction seam (notesFailureOutput) reused at both sites; no special-casing; the change is two field additions, nothing more.
- Complexity: Low. The wiring is a one-line field addition at each of two sites.
- Modern idioms: Yes. errors.As chain traversal, t.Context(), t.Parallel(), table-driven sub-tests.
- Readability: Good. Comments at both call sites explain why the helper is unconditional and why the two paths render identically.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None. The implementation is minimal, scoped exactly to the two in-scope sites, leaves all out-of-scope sites (resetAndAbort, batch reportSkip, padStage, pretty.go, gate_forbidden_test.go, askline_test.go, the tag/push presenter test) untouched, and the tests cover the acceptance criteria and edge cases without redundancy.
