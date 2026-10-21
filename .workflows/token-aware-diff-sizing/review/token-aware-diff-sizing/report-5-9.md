TASK: Regenerate's floor offers the same choice (token-aware-diff-sizing-5-9)

ACCEPTANCE CRITERIA:
- Attended `mint release regenerate v1.2.0` from a fresh source whose notes fail, with `on_notes_failure` unset, `abort` or `fallback`: the ✗ line with the AI's captured output verbatim beneath it, the saved chunk-notes path where this run saved a file, then the choice of `c`, `e`, `a` with `a` the default; in pretty mode no spinner animates while it waits
- `c` over `v1.1.0..v1.2.0` (`feat: add export`, `fix: handle empty input`, tag on the latter) gives those two subjects, shown at regenerate's review gate (accept/edit/abort, no `r`); accepting writes it to the chosen target byte-identical
- `e` after a chunked run that saved its chunk notes opens the editor on the saved file's contents; after a single-pass failure, or a run whose chunk notes could not be saved, it opens empty, even over an earlier failure's file at the path
- An editor saved non-empty shows at the review gate with no `r`, and the file still holds the chunk notes as the wave saved them; an empty or whitespace-only body skips the review gate and shows the choice again
- No editor launchable: warns and shows the choice again. A launched editor exiting non-zero: surfaced, nothing written, exit 1
- `a` or bare Enter: nothing written, exit 1
- A failed `r` at regenerate's review gate aborts, no choice shows
- A Ctrl-C during the notes call aborts as a cancel, no choice shows
- The README's `release regenerate` section describes the attended floor (`c` the version's commit subjects, `e` the editor on any saved chunk notes, `a` abort the default) whatever `on_notes_failure` holds, and says an unattended one stops with guidance

STATUS: complete

SPEC CONTEXT: §12.3 gives a single-version attended regenerate the release floor's one-keypress choice whatever `on_notes_failure` holds (regenerate never reads it; `fallback` stays release's). `a` stays the default because aborting is the as-built outcome. The chosen body goes to regenerate's own review gate without `r` (§13.3 corrigendum). The §12.2 corrigenda govern `e`: it edits a copy, so the saved file keeps the chunk notes as returned. An empty `e` returns to the choice, an unlaunchable editor warns and returns, and any other editor failure is surfaced and aborts. §11.4 defines when a saved file exists, including the save-failure branch that opens `e` empty. §15.1 has a failed `r` abort the regenerate. §9.1/§11.5 have a cancel propagate and never reach the floor. §16.1 suspends the spinner for the floor's choice. §18.2 asks for the README text.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/regenerate_interactive.go:229-253: the notes-stage failure branch. `offersChoice := reachesNotesFloor(err) && deps.attended()` (236), StageFailed with `Recoverable` (237-239), the unsaved-notes warning (240), then `regenerateNotesFloor` (247), and the body is routed as `regenerateSourceFloor` (250)
  - internal/engine/notesfloor.go:129-141 (`notesFloorGate`, c/e/a with `ChoiceAbort` the default); 148-184 (the shared `notesFloor` loop: `e` via `editBody` → `deps.Editor`, `ErrEditorReturnToGate` and an empty/whitespace body `continue`, any other editor error surfaced as `edit` and returned); 196-205 (`regenerateNotesFloor`, `c` via `notes.VersionNoAIBody` over `req.DiffRange`, every error wrapped in `abort`)
  - internal/engine/notesfloor.go:32-38: `reachesNotesFloor` excludes `context.Canceled` before classifying, so a Ctrl-C never reaches the floor
  - internal/engine/chunknotes.go:33-40: `saved` holds the bytes this run's `Save` wrote, set only on a successful save. `e` therefore opens on exactly the file's contents, and opens empty after a single pass or a failed save, never reading an earlier file at the path
  - internal/engine/regenerate_write.go:239-240: `regenerateSourceFloor` shares the supplied body's `NotesReviewGateNoRegen` review gate
  - internal/presenter/pretty.go:577-589: StageFailed stops the spinner and renders the verbatim Output and the SavedNotes line before the gate is prompted
  - cmd/mint/main.go:143, 192-195, 247: the startup stdin signal, `Yes`/`StdinInteractive` threaded into deps, `NewEditorLauncher` as the editor seam, `DiffRange` threaded for `c`
  - README.md:165: the paragraph in the `release regenerate` section
- Notes: The fresh producer (`RegenerateFreshBody` → `GenerateFromRange`) never routes through `ResolveFailure` (the only `OnNotesFailure` reader is `internal/notes/resolve.go:58`, reached from the forward `Selector` at `internal/notes/select.go:266`), so `fallback` cannot pre-empt the floor. `lowerCap` stays 0 on a failed generation because `suggestedCap` needs merged parts, so no spurious lower-the-cap hint shows over a floor body. A non-generation failure (a tag/release-source read error) falls through to the abort, as it should. No drift from the plan.

TESTS:
- Status: Adequate
- Coverage: internal/engine/regenerate_floor_test.go maps one test per criterion:
  - `on_notes_failure` across "", abort and fallback, plus the ✗ line, output and floor shape, with no StageStarted/ResumeSpinner between the failure and the gate (109-135)
  - saved path before the choice (137-148)
  - `c` through `git log --format=%s v1.1.0..v1.2.0`, a review gate keyed y/n/e and a byte-exact provider body (150-166)
  - `e` on a part failure and on a merge failure, checked against the file's own contents (182-213)
  - single-pass opens empty (215-230)
  - save failure over an earlier file opens empty and leaves that file (232-259)
  - an edited body reaches the gate and leaves the file as saved (261-276)
  - empty and whitespace-only bodies loop back to the choice (278-312)
  - the real `EditorLauncher` with the editor not found warns and loops (314-334)
  - the editor exiting non-zero is surfaced as `edit` and writes nothing (336-358)
  - `a` and bare Enter (360-387)
  - a failed `r` (389-405)
  - a cancel (407-428)

  cmd/mint/regenerate_floor_test.go:17-52 proves the production wiring end to end: the stdin signal, `DiffRange` and the subjects on the gh write.
- Notes: Each test would fail if its behaviour broke. The save-failure test also guards against the editor reading the file at the path, which the single-pass test alone could not catch. No redundant or implementation-detail tests.

CODE QUALITY:
- Project conventions: Followed. The choice goes through `Presenter.Prompt`, `e` goes through the `Editor` seam, git runs through `runner.CommandRunner`, and errors are wrapped as `*AbortError`. External test packages use `t.Parallel()`, FakeRunner and RecordingPresenter.
- SOLID principles: Good. `notesFloor` is shared by release and regenerate, parameterised on the gate question and the subjects builder.
- Complexity: Low
- Modern idioms: Yes
- Readability: Good. The changed comments (the `RegenerateRun` doc, the failure-branch comment, the `regenerate_fresh.go` FAILURE ROUTING note, the `regenerate_write.go` source-gating note, the `regenerateSourceFloor` doc) hold against the code.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
