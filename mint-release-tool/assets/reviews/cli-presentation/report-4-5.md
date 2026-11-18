TASK: cli-presentation-4-5 — Pretty spinner lifecycle (single spinner started on StageStarted, replaced in place by check/cross; output buffered, printed below cross)

ACCEPTANCE CRITERIA:
- In pretty mode a single spinner starts on a blocking StageStarted and is replaced in place by the ✓ line on StageSucceeded.
- On StageFailed the spinner is replaced in place by the ✗ line and the buffered captured output is printed below the ✗ (only on failure).
- Only one spinner active at a time across sequential stages — no two concurrent spinners.
- The spinner uses a lightweight standalone library with explicit Start()/Stop() — not Bubble Tea, no alt-screen, no full-screen redraw.
- plain emits no animation frames, no \r, no animation ANSI, and pulls in no spinner/UI library.
- A short, non-spinner stage (no blocking StageStarted) renders only its static completion line.
- The captured underlying output is buffered (never streamed through the spinner) and printed below ✗ only on failure; spinner frames go to stdout, not stderr.

STATUS: Complete

SPEC CONTEXT: "Spinner Lifecycle (pretty only)" (spec:261) — one at a time, started on StageStarted, replaced in place by ✓/✗; underlying output buffered, printed below ✗ on failure. "Library Selection" (spec:269) — lightweight standalone spinner, NOT Bubble Tea; plain pulls in no UI library.

IMPLEMENTATION:
- Status: Implemented
- Location: spinner.go:26-87 (StageSpinner seam, spinnerFactory, braille CharSets[11], briandowns wrapper to out); pretty.go:367-376 (StageStarted blocking-only, defensive stopSpinner then create+Start single spinner), :394-401 (stopSpinner one-at-a-time, nil-safe), :460-469 (StageSucceeded stopSpinner+static ✓), :509-519 (StageFailed stopSpinner+✗+err summary+writeNotesBody only when Output non-empty); plain.go:153-158,166-169 (terse start line; Suspend/Resume no-ops); pretty.go:287-299 (WithSpinnerFactory seam).
- Notes: Spinner suffix is dim s.Name ("⠋ notes") not richer "generating with claude…" — correct (StageStart carries only Name+Blocking; event-payload principle). "In place" = briandowns Stop() then static line, no alt-screen. Suspend/Resume machinery belongs to 4-6; presence not scope creep.

TESTS:
- Status: Adequate
- Coverage (pretty_spinner_test.go): started/stopped→✓ (:81-100); start→stop→✗ (:106-122); body below ✗ on failure + not on success (:128-158); one-at-a-time across two stages (:164-181, maxActive<=1); defensive stop on double-start (:188-204); short stage no spinner (:210-223); non-blocking renders nothing (:230-241); real factory frames not to stderr (:247-258); spinner_deps_test.go:14-26 (bubbletea not reachable); plain_test.go:171-189 (no braille/CR/ESC); plain_test.go:1011-1015 (plain.go imports no UI lib); golden_transcript_test.go:101-205.
- Notes: spySpinner tracks active/maxActive deterministically. Each test distinct. Not over/under-tested.

CODE QUALITY:
- Project conventions: Followed — seam+injectable factory mirrors builder style; real spinner writes to out only.
- SOLID principles: Good — minimal StageSpinner interface (segregation/inversion); stopSpinner centralises invariant.
- Complexity: Low.
- Modern idioms: Yes — function-type factory, nil-safe stop, compile-time interface assertion.
- Readability: Good.
- Issues: None blocking.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES:
- [idea] internal/presenter/spinner.go:70-78 — spinner suffix is the bare stage Name; the spec worked example shows richer "generating with claude…". Achieving it needs an engine-supplied start-detail field on StageStart (cross-cuts the engine event schema). Decide whether to extend the payload or accept Name as the payload-faithful rendering.
- [idea] internal/presenter/plain_test.go:1011-1015 + spinner.go:7 — the plain UI-library guard is source-import-level; because plain and the briandowns spinner share one Go package, any binary importing the plain presenter still links briandowns/spinner. Decide whether to split the spinner into a sub-package to make "plain pulls in no UI library" a real link-level property, or document the guard as source-level by design.
