TASK: cli-presentation-2-1 — Extend StageStarted/StageSucceeded payloads with long/blocking flag, detail, and engine-supplied elapsed (tick-ff9cb4)

ACCEPTANCE CRITERIA:
- StageStart carries Name and Blocking; StageSuccess carries Name, Detail, Elapsed, and Blocking.
- Doc comments state the three zero-value semantics: short stage carries no elapsed, Elapsed==0 legal under Blocking==true, empty Detail legal.
- The package compiles and imports nothing beyond stdlib (time); no formatting/rendering logic added.
- RecordingPresenter captures the full extended payload for both events (no field dropped).
- No method or struct derives engine knowledge — no stage-name list, no timing in the presenter package.

STATUS: Complete

SPEC CONTEXT: "The Presenter Seam (Architecture)" — Event payload principle (spec:64-67): engine supplies every datum the renderings consume; StageStarted carries blocking flag, StageSucceeded carries detail + engine-measured elapsed; presenter never times stages or hardcodes stage-name lists. Implementation matches exactly.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/presenter/presenter.go:343-384 (StageStart, StageSuccess structs); presentertest/recording.go:82-83,128-135,215-216 (capture).
- Notes: StageStart (348-354) Name+Blocking with "no hardcoded list of long stages here" doc. StageSuccess (371-384) Name, Detail, Elapsed time.Duration, Blocking. Three zero-value semantics documented as numbered list (361-370) matching AC2 verbatim. presenter.go imports only stdlib io and time; time.Duration is a field TYPE only, no time.Now/Since timing (grep-confirmed). No stage-name list/switch. RecordingPresenter stores whole structs so extended fields captured automatically. AC3 correctly scoped to presenter.go (contract file), not whole package.

TESTS:
- Status: Adequate
- Coverage: name+blocking (presenter_test.go:172-181, round-trip :368-386); detail/elapsed/blocking (:183-201, round-trip :391-421; recording_test.go:15-50); edge 1 short no-elapsed (:428-444); edge 2 zero elapsed under blocking (:450-465); edge 3 empty detail (:469-479).
- Notes: Contract/shape tests at the correct level (rendering lives in 2-2/2-3). Mild intentional overlap between field-level and round-trip tests serving distinct guards across two packages. Acceptable.

CODE QUALITY:
- Project conventions: Followed (doc comments, idiomatic naming, time.Duration).
- SOLID principles: Good — contract-only file, interface segregation preserved.
- Complexity: Low.
- Modern idioms: Yes.
- Readability: Exemplary — zero-value semantics documented as numbered list mirroring rendering tasks' needs.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
