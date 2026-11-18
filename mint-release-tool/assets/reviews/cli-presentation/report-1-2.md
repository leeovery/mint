TASK: cli-presentation-1-2 — Recording/fake presenter for event assertions

ACCEPTANCE CRITERIA:
- RecordingPresenter satisfies the Presenter interface (assignable to Presenter).
- Every method call is recorded in call order with its complete payload retrievable.
- A new RecordingPresenter with no calls reports zero events and accessors return empty without panicking (zero-events edge case).
- A sequence of multiple stages, repeated and interleaved with RunStarted/RunFinished, is recorded in exact issue order (multiple-stages edge case).

STATUS: Complete

SPEC CONTEXT:
Spec "The Presenter Seam (Architecture)" names Testability as the core Go rationale — assert which events fired and with what payload, independent of rendering. "Dependencies → Notes" states the layer is built independently of the engine: "a recording presenter and fixed event sequences are enough to build and assert rendering." This task delivers that recorder.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/presenter/presentertest/recording.go — RecordingPresenter struct at :99; Event tagged struct at :79; EventKind enum at :17; the 14 interface methods at :123-222; Kinds() at :227; At() at :238; compile-time guard at :120.
- Notes:
  - Placed in a `presentertest` subpackage (mirrors net/http/httptest), keeping the double off the production package surface and exercising only exported types — documented at file head.
  - Single ordered `Events` slice with per-call append preserves cross-kind interleaving.
  - Tagged-struct Event form chosen over parallel per-kind slices; rationale documented at recording.go:73-78.
  - Zero value usable: nil Events slice, bounds-guarded accessors, no constructor.
  - Scope note (not drift): the recorder satisfies the FULL interface as it stands after phases 2-4 plus Prompt scripting hooks (NextChoices, PromptResult) — breadth is necessary to compile, not creep. Hooks consumed downstream by gate tests.

TESTS:
- Status: Adequate
- Location: internal/presenter/presentertest/recording_test.go
- Coverage:
  - Satisfies-interface: compile-time `var _ presenter.Presenter` in both production (recording.go:120) and test (recording_test.go:13).
  - Single full payload: TestRecordingPresenter_RecordsSingleStageSucceededPayload (Name/Detail/Elapsed/Blocking).
  - Call order across kinds: TestRecordingPresenter_RecordsEventsInCallOrderAcrossKinds.
  - Zero-events edge case: TestRecordingPresenter_ReportsZeroEventsBeforeAnyCall (empty Events/Kinds(), At(0) ok=false, no panic).
  - Multiple-stages edge case: TestRecordingPresenter_RecordsMultipleStagesInIssueOrder (two interleaved cycles, no collapsing/de-dup).
  - At() accessor: TestRecordingPresenter_AtFetchesNthEvent (in-range, out-of-range, negative).
- Notes: Not under-tested — every AC and both edge cases have a dedicated behaviour-asserting test. Not over-tested. Std-lib `testing` consistent with package convention. Minor gap (non-blocking): Prompt precedence resolution and Suspend/ResumeSpinner ordering not directly unit-tested here; exercised indirectly downstream.

CODE QUALITY:
- Project conventions: Followed (httptest-style subpackage, thorough doc comments, std-lib testing, idiomatic naming).
- SOLID principles: Good — single responsibility; depends only on the presenter interface + exported payloads; Liskov substitution compile-proven.
- Complexity: Low — every method a one-line append.
- Modern idioms: Yes — zero-value-ready struct, comma-ok At(), iota enum with KindUnknown sentinel, FIFO reslice.
- Readability: Good.
- Issues: None blocking.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES:
- [quickfix] internal/presenter/presentertest/recording_test.go — add a focused unit test for Prompt answer-resolution precedence (PromptResult overrides NextChoices overrides gate.Default; NextChoices pops FIFO with nil error). Hooks documented (recording.go:104-116, 182-193) and used downstream but never asserted in isolation.
- [quickfix] internal/presenter/presentertest/recording_test.go — add an assertion that SuspendSpinner/ResumeSpinner each append a payload-less Event of the correct Kind in order (recording.go:201-210).
