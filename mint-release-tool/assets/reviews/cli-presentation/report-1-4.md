TASK: cli-presentation-1-4 — Plain presenter renders the minimal stage sequence

ACCEPTANCE CRITERIA:
- PlainPresenter satisfies Presenter and writes narration to the injected out writer.
- Minimal sequence (RunStarted -> StageSucceeded -> RunFinished) produces the expected terse lines in order.
- Start-of-run line renders engine-supplied RunInfo.Action; no hardcoded "releasing" (regenerate renders "mint: regenerating {project} v{X}").
- Output contains no ESC (0x1b), no braille/emoji glyphs, no CR animation — verified by scanning captured bytes.
- Plain code path imports no UI library (verified by import inspection / dependency assertion).
- Short (non-blocking) StageStarted emits no start line; blocking StageStarted does.

STATUS: Complete

SPEC CONTEXT: "The Plain Layer" fixes terse lowercase key:value lines, one per stage on completion, start line for long/blocking stages only, `mint: {action} {project} v{X}` start-of-run with a verb-shaped engine-supplied action, `done: {project} v{X} {url}` end-of-run. "Library Selection" makes byte-purity and no-UI-library the non-negotiable bars.

IMPLEMENTATION:
- Status: Implemented (extended additively by later phases)
- Location: internal/presenter/plain.go
  - NewPlainPresenter (78-80) / NewPlainPresenterWithInput (86-91): out, err io.Writer.
  - Compile-time `var _ Presenter = (*PlainPresenter)(nil)` (71).
  - RunStarted (138-140): `mint: %s %s v%s` using info.Action — engine-supplied.
  - StageStarted (153-158): start line only when s.Blocking; short stages return early.
  - StageSucceeded (178-192): `{stage}: {detail}` with `ok` fallback.
  - RunFinished (428-451): `done: {project} v{X} {url}`, URL omitted cleanly when empty.
  - writef -> fmt.Fprintf(p.out, ...) (121-123); imports only stdlib fmt/io/bufio/os/strings — no UI library.
- Notes: File fully built out (Prompt, Warn, Unwound, ShowPlan, ShowNotes, ShowVersion, InitResult, verb dispatch) — expected post-convergence; later phases extend the same file without regressing the 1-4 path.

TESTS:
- Status: Adequate
- Coverage (plain_test.go, helpers bytepurity_test.go / import_guard_helpers_test.go):
  - TestPlainPresenterSatisfiesInterface (54-63) — AC1.
  - TestPlainPresenterRendersMinimalSequence (68-82) — exact ordered sequence, AC2.
  - TestPlainPresenterStartLineUsesEngineAction (87-110) — releasing AND regenerating, AC3.
  - TestPlainPresenterEmitsNoANSIGlyphOrAnimationBytes (781-791) + TestPlainPresenterBlockingStageEmitsNoAnimationBytes (171-189) — AC4.
  - TestPlainPresenterImportsNoUILibrary (1011-1015) — go/parser import scan with scanned==0 guard, AC5.
  - TestPlainPresenterStageStartedHonoursBlocking (142-165) — both branches, AC6.
- Not under-tested: every AC has a dedicated behaviour-level test. Not over-tested: UI-library guard correctly scans only plain.go.

CODE QUALITY:
- Project conventions: Followed — io.Writer injection seam, concrete constructor + compile-time interface assertion, table-driven t.Parallel subtests.
- SOLID principles: Good — single rendering responsibility; presenter never re-derives engine knowledge.
- Complexity: Low.
- Modern idioms: Yes — fmt.Fprintf, builder-style setters, exhaustive verb switch.
- Readability: Good.
- Issues: None blocking.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
