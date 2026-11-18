TASK: token-aware-diff-sizing-11-3 — Corrections (run the single-version regenerate -y prompt guard under -y; scope the setup guide's on_notes_failure routing to release)

ACCEPTANCE CRITERIA:
- Run against the current engine, TestRegenerateRun_Yes_AlwaysCallsPrompt hands RegenerateRun deps with Yes and StdinInteractive both true; the recorder captures the source, target and notes gates in that order, the source gate's AcceptEcho equals its default, and the test passes (CLAUDE.md seam 4; the "-y at a terminal" mode of spec §12.3's unattended regenerate)
- With a trial engine-side skip keyed on deps.Yes that returns the default at the source, target or notes gate instead of calling Prompt, TestRegenerateRun_Yes_AlwaysCallsPrompt fails; with the trial reverted it passes, and the trial is never committed
- mint setup's AI-transport paragraph reads "A release's notes still unwritten follow [release].on_notes_failure: …", the rest of that sentence word for word, then "Regenerate never reads on_notes_failure and never ships the commit-subject list on its own.", then "Commit drops to the user's $EDITOR." (§12.3)
- mint setup's `mint release regenerate` entry ends "It rides on the same [release] config, except on_notes_failure." (§1.3)
- The paragraph's classification and halving sentences read as before, and the guide still names no mint flag (§18.1)
- The existing suite stays green with no test edited beyond TestRegenerateRun_Yes_AlwaysCallsPrompt's deps: every pin in internal/setupguide/setupguide_test.go (floorOffer, floorStop, floorMoves, "the commit-subject list, the one non-AI body", "on_notes_failure", the flag-free list) holds unedited, and the five CLAUDE.md gates pass

STATUS: complete

SPEC CONTEXT: Spec §1.3 (line 41): "Regenerate rides on [release] config throughout, as built, except on_notes_failure, which it never reads (§12.3)." §12.1/§12.3: on_notes_failure is release's alone; an attended `regenerate <version>` is offered the c/e/a floor choice whatever the key holds, an unattended one stops with next moves, and `--all` skips the version. §18.1: the setup guide is edited only where the feature makes it untrue and stays flag-free. CLAUDE.md seam 4: -y auto-accept and its echo happen inside the presenter, never as an engine-side branch; the engine's only -y input for a single-version regenerate is ReleaseDeps.Yes, read solely through attended() (internal/engine/notesfloor.go:18-20).

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/regenerate_interactive_test.go:236-241 — `deps := freshRunDeps(rec, f)`, `deps.Yes = true`, `deps.StdinInteractive = true`, passed to RegenerateRun; the request, bare RecordingPresenter{}, the three-gate subject assertion (:246-248) and the AcceptEcho check (:252-255) are unchanged.
  - internal/setupguide/setupguide.go:137 — "It rides on the same [release] config, except on_notes_failure."
  - internal/setupguide/setupguide.go:179-182 — "A release's notes still unwritten follow [release].on_notes_failure: …, the one non-AI body. Regenerate never reads on_notes_failure and never ships the commit-subject list on its own. Commit drops to the user's $EDITOR."
- Notes:
  - Commit b23fa5d touches exactly these two files (8+/4-); no later commit touches internal/setupguide/, regenerate_interactive_test.go, regenerate_write_test.go, regenerate_interactive.go or notesfloor.go. freshRunDeps (regenerate_interactive_test.go:89-91) and regenWriteDeps (regenerate_write_test.go:579-586) are unchanged; no production engine file and no setupguide_test.go line was edited.
  - The new guide sentence is true against the code: the only production reader of OnNotesFailure is notes.ResolveFailure (internal/notes/resolve.go:54-58), called only from Selector.resolveNormalPathFailure (internal/notes/select.go:266), and no regenerate*.go production file references Selector, ResolveFailure or OnNotesFailure. The single-version failure branch (internal/engine/regenerate_interactive.go:233-250) offers the floor only when deps.attended() and otherwise prints next moves and aborts; the batch skips. "Never ships the commit-subject list on its own" holds: the list reaches a regenerate only when an operator picks `c`.
  - The text agrees with README.md:165 ("Regenerate does not read `on_notes_failure` … never ships the commit-subject list over a version's notes on its own") and README.md:295 ("a release's failed notes call reaches `on_notes_failure`").
  - The classification and halving sentences (setupguide.go:175-179) are byte-identical to before, per the commit diff. The added text names no flag. The overview() doc comment (setupguide.go:109-121) still holds.

TESTS:
- Status: Adequate
- Coverage: With Yes and StdinInteractive both true, the run is the "-y at a terminal" mode. The engine's only -y read (attended(), consulted at regenerate_interactive.go:236) sits on the notes-failure branch, which staticBody() never reaches, so the run proceeds to the source, target and notes gates. An engine-side skip keyed on deps.Yes, or on !deps.attended(), at any of the three gates would now fire, drop that gate from the recorder's Prompt events, and fail the `[]string{"source", "target", "notes"}` assertion at :246. Before this change that skip stayed dormant because Yes was false. The guard now does what its name, its doc comment (:219-222) and the file-header bullet (:24-25) claim.
- Notes: All pins in internal/setupguide/setupguide_test.go still match the new text: floorOffer/floorStop/floorMoves (:843-845) appear verbatim in the unchanged middle of the sentence; "the commit-subject list, the one non-AI body" (:830) is intact; "on_notes_failure" (:349) is still present; the flag list (:870) — " -y", "--yes", "--no-ai", "--notes-file", "--dry-run" — matches nothing new. The plan bars edits to setupguide_test.go, so the new regenerate sentence has no test pinning it. That follows the task's stated scope, not a gap in it.

CODE QUALITY:
- Project conventions: Followed (external test package, t.Parallel, FakeRunner/RecordingPresenter doubles; the guide stays flag-free and keeps config defaults spliced from constants)
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "Run against the current engine, TestRegenerateRun_Yes_AlwaysCallsPrompt … and the test passes" — reading settles the deps, the gate order and the AcceptEcho check; the pass itself needs `go test -race ./internal/engine -run TestRegenerateRun_Yes_AlwaysCallsPrompt` to run.
- "With a trial engine-side skip keyed on deps.Yes … TestRegenerateRun_Yes_AlwaysCallsPrompt fails; with the trial reverted it passes" — reading settles that a skip at any of the three gates breaks the :246 subject assertion, and that no trial was committed (b23fa5d touches no production engine file). Confirming it means running the mutation trial and then the reverted run.
- "The existing suite stays green … and the five CLAUDE.md gates pass" — reading settles that no other test was edited and every setupguide pin matches the new text; whether the suite is green and the gates pass needs `go build ./...`, `gofmt -l .`, `go vet ./...`, `go test -race ./...` and `golangci-lint run` to be run.
