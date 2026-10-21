TASK: Choose the commit-subject list or abort at the release floor (token-aware-diff-sizing-5-1, tick-77fef6)

ACCEPTANCE CRITERIA:
- Attended `mint release`, `on_notes_failure` unset or `abort`, single-pass call fails with output: ✗ line with the AI's captured output verbatim beneath it, then the `c` / `a` choice, `a` the default (§12.1, §12.2)
- Chunked run that failed and saved its chunk notes: the saved file's full path shows after the output and before the choice (§11.4, §12.1)
- Timeout or AI tool not installed: the same choice shows beneath that cause's ✗ line (§9.1, §12.1)
- Pretty mode: no spinner animates while the choice waits for a keypress (§16.1)
- `c`: body is `git log --format=%s {lastTag}..HEAD`; shows at the review gate `Release with these notes?` with release/abort/edit and no `r`; accepting records, tags, pushes, publishes with the list byte-identical in tag message, changelog and provider release; usual success line; exit 0 (§12.2, §14.1)
- `c` with an empty or whitespace-only list: the maintenance-release body (§14.1)
- `a`, bare Enter, or stdin closing at the choice: run ends before any tag, exit 1, pre_tag artifact commit unwound (§6.6, §12.2)
- `mint release -y` under `abort` whose notes fail: no choice, no input read, exit 1 before any tag (§12.1)
- AI notes at the review gate and an `r` whose call fails: aborts before any tag, no choice (§12.1, §15.1)
- Ctrl-C while the notes call runs: aborts as a cancel, no choice (§9.1, §11.5)
- README `release` section, `on_notes_failure`'s README row, `on_notes_failure`'s SoT description (as `mint setup` renders it), and `mint setup`'s AI-transport paragraph (flag-free) say an attended release under `abort` is offered the commit-subject list or an abort while an unattended one stops (§14.2, §18.1, §18.2)

STATUS: complete

SPEC CONTEXT: §12 makes the floor the end of the ladder without AI notes, reached after preflight and the pre_tag artifact commit, so a re-run would repeat every step. §12.1 gives the attended `abort` run a one-keypress choice beneath the ✗ line, the verbatim output and the saved chunk-notes pointer; unattended runs stop with next moves; a failed `r` (§15.1) and a cancel (§9.1, §11.5) never reach it. §12.2: `c` the commit-subject list, `a` abort as the default, the chosen body going to the non-AI review gate (no `r`), every abort unwinding as any pre-push failure does. §14.1: the commit-subject list is the one non-AI body, the maintenance-release body when empty. §16.1: the spinner suspends for the floor's choice. Task 5-2 has since added `e` to the same gate, which the as-built code and tests reflect.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/release.go:562-579 — a notes failure is classified by `reachesNotesFloor`; an attended real run (`deps.asksAtTerminal(opts)`) gets a Recoverable `StageFailed` (✗ line, `Output`, `SavedNotes`) and then `releaseNotesFloor`; any other run gets the non-recoverable failure, next moves where it is a generation failure, and `Unwind`
  - internal/engine/notesfloor.go:17-26 — `attended()` / `asksAtTerminal()` from the threaded `Yes` / `StdinInteractive` signals
  - internal/engine/notesfloor.go:32-38 — `reachesNotesFloor` excludes `context.Canceled` and admits only `notes.ClassifyFailure`-known generation failures (timeout and missing binary included, internal/notes/resolve.go:140-156)
  - internal/engine/notesfloor.go:129-141 — the gate: `c` / `e` / `a`, Default `ChoiceAbort`
  - internal/engine/notesfloor.go:148-184 — the choice loop; `c` builds the body through `subjects`, `a` returns `errFloorAborted`, a Prompt error (ErrInputClosed) is returned for the caller to unwind
  - internal/engine/notesfloor.go:188-192 — `releaseNotesFloor` uses `notes.NoAIBody` over `{lastTag}..HEAD` (internal/notes/resolve.go:65-71, `subjectBody` falling back to `StubBody()` on an empty or whitespace-only list, resolve.go:99-105) and returns `notes.KindFloor`
  - internal/engine/release.go:1842-1847 — `gateForKind` gives `KindFloor` the `FirstReleaseReviewGate()` (`Release with these notes?`, y/n/e, internal/engine/engine.go:83-101)
  - internal/presenter/gate.go:28-31 — `ChoiceCommitSubjects` ("c") and `ChoiceAbort` ("a") added to the gate vocabulary
  - internal/presenter/pretty.go:576-579, internal/presenter/plain.go:203-204 — `StageFailed` stops the spinner and leaves `terminalFailure` unset for a Recoverable failure, so a `c` run keeps its success line
  - cmd/mint/main.go:296-304, 316-329 — `Yes` and `StdinInteractive` threaded into `ReleaseDeps`
  - Docs: README.md:109 (release section), README.md:256 (`on_notes_failure` row), internal/config/metadata.go:134 (SoT description), internal/setupguide/setupguide.go:179-182 (AI-transport paragraph, no command flag)
- Notes: A floor abort with no commit made emits no `Unwound`, the same as as-built `n` at the review gate; ErrInputClosed at the choice is returned unrendered through `abort(err)` (exit 1), matching `ReviewDecision`'s contract. With no `r` on the KindFloor gate, the nil `generator` reaching `perRunRegenerator` is never invoked (release.go:1344-1346 returns nil for it).

TESTS:
- Status: Adequate
- Coverage:
  - AC1: internal/engine/notes_floor_test.go:93-123 (unset and explicit `abort`; ✗ message, verbatim output, keys c/e/a, default a, no tag)
  - AC2: notes_floor_test.go:125-138 (SavedNotes on the failure, choice follows it) plus internal/presenter/stagefailed_savednotes_test.go:23-58 (path renders after the output in both presenters)
  - AC3: notes_floor_test.go:140-165 (timeout, tool not installed)
  - AC4: notes_floor_test.go:68-72 (no StageStarted/ResumeSpinner between the ✗ line and the gate) plus internal/presenter/recoverable_failure_test.go:89-107 (zero animating spinners at the gate's read in pretty mode)
  - AC5: notes_floor_test.go:167-211 (exact `git log --format=%s v1.2.3..HEAD` argv, review gate question and y/n/e keys, byte-identical tag annotation, changelog and `gh release create` stdin, Recoverable failure, RunFinished last, nil error)
  - AC6: notes_floor_test.go:213-242 (empty and whitespace-only logs give `StubBody()`)
  - AC7: notes_floor_test.go:257-303 (`a`, bare Enter via Default, ErrInputClosed; reset --hard to the starting commit, unwound summary, no tag, no subject list built)
  - AC8: notes_floor_test.go:305-325 (`Yes` set: no floor gate, no AskLine, non-Recoverable failure, no tag)
  - AC9: notes_floor_test.go:327-349; AC10: notes_floor_test.go:351-366
  - Wiring: cmd/mint/release_seam_test.go:227-247 (attended run through the cmd seam gets the floor, exit 1 on the default abort) and :211-225 (non-TTY gets none)
  - Docs: internal/config/notes_floor_docs_test.go:25-53, internal/setupguide/setupguide_test.go:848-875 (including a flag-absence check on the transport paragraph)
- Notes: Each test would fail if its behaviour broke. The tests are focused, with no redundant variants.

CODE QUALITY:
- Project conventions: Followed. The choice goes through `Prompt(gate)`, `-y` and TTY detection stay in the presenter with the engine reading the startup signals (seam 4), git reads go through the runner, the body passes through verbatim, and the tests are external packages using FakeRunner and RecordingPresenter with exact argv.
- SOLID principles: Good. `notesFloor` is shared with regenerate through the injected `subjects` closure; the release wrapper only supplies the range and the Kind.
- Complexity: Low
- Modern idioms: Yes
- Readability: Good. The comments in the changed code hold true against it.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
