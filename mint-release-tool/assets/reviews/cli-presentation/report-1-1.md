TASK: cli-presentation-1-1 — Define the Presenter interface with the minimal event set (tick-98e6cd)

ACCEPTANCE CRITERIA:
- [x] internal/presenter/presenter.go declares a Presenter interface with the (at minimum five) minimal methods.
- [x] RunInfo carries an engine-supplied Action word (releasing/regenerating); no presenter code hardcodes the literal "releasing".
- [x] StageStarted and StageSucceeded carry the Blocking/long-stage flag; StageSucceeded carries engine-supplied Elapsed; StageFailure carries both Message and captured Output.
- [x] The package compiles and imports no UI library and no I/O beyond what type definitions require.
- [x] No method derives engine knowledge (no stage-name lists, no timing) — all such data arrives via payloads.

STATUS: Complete

SPEC CONTEXT:
Spec "The Presenter Seam (Architecture)" defines an event/step-oriented interface called at lifecycle points, governed by the event-payload principle: the engine supplies every datum the rendering consumes; the presenter holds no hardcoded stage-name lists and times no stages. The spec explicitly anticipates the fuller vocabulary (Warn, Unwound, ShowPlan, ShowNotes, Prompt) arriving in Phases 2–3, so payloads must extend without churn.

IMPLEMENTATION:
- Status: Implemented, and correctly extended by later phases as the task's own Context anticipated.
- Location: internal/presenter/presenter.go — interface :56-182, RunInfo :336-341, StageStart :348-354, StageSuccess :371-384, StageFailure :390-394, RunResult :497-509. go.mod:1 declares module `mint`.
- Notes:
  - All five walking-skeleton methods present: RunStarted (:63), StageStarted (:65), StageSucceeded (:67), StageFailed (:70), RunFinished (:181). The interface has since grown (Warn/Unwound/ShowPlan/ShowNotes/ShowVersion/Prompt/Suspend/Resume/InitResult) — later-phase additions documented at :50-55, not scope creep against 1-1. The original five payload struct shapes remain intact.
  - RunInfo.Action (:339) is the engine-supplied verb word; presenter.go contains no hardcoded "releasing" literal (only doc-comment examples). Negative guard at pretty_test.go:491.
  - StageStart.Blocking (:353), StageSuccess.Blocking (:383)+Elapsed (:380, time.Duration), StageFailure.Message+Output (:392-393) all present. Zero-value semantics documented (:361-384).
  - presenter.go imports only stdlib `io` and `time` (:6-9) — satisfies the no-UI/minimal-I/O criterion.
  - No engine knowledge derived: Blocking travels on the payload, elapsed is engine-supplied, no stage-name list exists.

TESTS:
- Status: Adequate.
- Location: internal/presenter/presenter_test.go; supporting guards in plain_test.go, pretty_test.go, import_guard_helpers_test.go.
- Coverage: interface satisfiability (presenter_test.go:39, :41); Blocking/Elapsed shapes (:172, :183); StageFailure Message+Output (:203); Action round-trip RunInfo{Action:"regenerating"} (:218). "No hardcoded releasing" guarded behaviourally at pretty_test.go:300/482/491 and plain_test.go:87. "No UI library" guarded by TestPlainPresenterImportsNoUILibrary (plain_test.go:1011) and the Bubble-Tea dependency guard (spinner_deps_test.go:14).
- Balance: Not under-tested — every criterion has an assertion. Not over-tested — minor recorder round-trip overlap serves a distinct seam.

CODE QUALITY:
- Project conventions: Followed. Compile-time interface checks present; InitAction.String() honours fmt.Stringer; constructors return concrete structs; zero-value usefulness deliberate and documented.
- SOLID principles: Good — interface is the DI seam; enums additive/open-closed.
- Complexity: Low.
- Modern idioms: Yes (iota enums, Stringer, time.Duration).
- Readability: Good.
- Issues: Interface-size observation only — golang-structs-interfaces advises 1–3 methods; Presenter has 14, but this is an intentional, spec-mandated single lifecycle seam; guidance does not cleanly apply. No action.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
