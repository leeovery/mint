TASK: Write the notes in `$EDITOR` from the release floor (token-aware-diff-sizing-5-2, tick-ec08bd)

ACCEPTANCE CRITERIA:
- Given an attended release under `abort` whose notes fail, the floor's choice offers `c`, `e` and `a`, with `a` still the default (§12.2)
- Given a chunked run that failed and saved its chunk notes, `e` opens the editor holding the saved file's contents, its missing-notes marker included where it has one. Given every part finished and the merge failed, the editor holds every part's notes, as saved (§11.3, §11.4, §12.2)
- Given a single-pass call that failed, so no file was saved, `e` opens the editor empty (§12.2)
- Given a chunked run whose chunk notes could not be saved, with a file from an earlier failure of the same tag at the path, `e` opens the editor empty (§11.4)
- Given the operator saves a non-empty body, it shows at the review gate `Release with these notes?`, which offers release, abort and edit with no `r`. Accepting it releases with the saved text byte-identical in the tag message, changelog entry and provider release (§12.2)
- Given the operator edits the opened chunk notes and saves, the file at the saved path still holds the chunk notes exactly as the wave saved them, whether the release then ships or is aborted at the review gate (§11.4, §12.2)
- Given the operator closes the editor leaving the body empty or whitespace-only, the review gate does not show, and the `c` / `e` / `a` choice shows again. Choosing `c` from it then gives the commit-subject list (§12.2)
- Given no editor can be launched, because the resolved editor is not installed, mint warns that it could not launch the editor and the `c` / `e` / `a` choice shows again (§12.2)
- Given a launched editor that exits non-zero, the failure is surfaced and the run aborts before any tag, exiting 1, and a pre_tag artifact commit is unwound, as on `a` (§6.6, §12.2)
- The README's description of the floor's choice covers `e`: it opens the editor on the saved chunk notes where this run saved a file, otherwise empty, and an empty body returns to the choice (§18.2)
- The surfaces task 5-1 described the choice on say an attended release under `abort` whose notes fail is offered the commit-subject list, writing the notes in the editor, or an abort: `on_notes_failure`'s README row; its description as `mint setup`'s config reference renders it; `mint setup`'s AI-transport paragraph, which still names no command flag (§14.2, §18.1, §18.2)

STATUS: issues_found

SPEC CONTEXT: §12.2 adds `e` to the attended release floor as a write-from-scratch route through release's editor seam. Corrigenda 2026-10-19 settle that `e` edits a copy (the saved file at §11.4's path keeps the chunk notes as the calls returned them), that a missing editor warns and returns to the choice while any other editor failure is surfaced and aborts, and that every abort at the floor unwinds the pre-tag artifact commit (§6.6, CLAUDE.md "Surgical unwind, then PONR"). An empty or whitespace-only `e` is refused because closing the editor empty is how an operator backs out, and git would accept an empty tag message. §11.4: when the chunk notes cannot be written, the run goes on as if no file was saved, so `e` opens empty. The chosen body goes to the non-AI review gate (y/n/e, no `r`).

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/notesfloor.go:129-141 — `notesFloorGate` declares c / e ("write in $EDITOR") / a, default a
  - internal/engine/notesfloor.go:148-184 — `notesFloor` loop: `e` runs `editBody(ctx, deps.Editor, savedNotes)`; `ErrEditorReturnToGate` re-presents the choice (line 165-166); any other editor error is surfaced as an `edit` StageFailed and returned (167-169); an empty or whitespace-only body re-presents the choice (170-173); a non-empty body is returned verbatim (175)
  - internal/engine/notesfloor.go:188-192 — `releaseNotesFloor` returns `notes.KindFloor`
  - internal/engine/chunknotes.go:29-41 — the keeper's saver records the saved text in `k.saved` only on a successful save, so the editor's copy is the exact string written to disk (marker included, since `keepChunkNotes` prepends the marker before calling the saver, internal/notes/chunknotes.go:53-57) and stays "" when the save failed, whatever an earlier run left at the path
  - internal/engine/release.go:576-579 — the floor is handed `chunkNotes.saved`; any floor error goes through `Unwind`, so an editor failure resets the pre_tag artifact commit and returns `abort(reason)` (exit 1, internal/engine/engine.go:37)
  - internal/engine/release.go:609, 1842-1847 — `KindFloor` takes `FirstReleaseReviewGate` ("Release with these notes?", release/abort/edit, no `r`)
  - internal/engine/editor.go:90-131 — `EditorLauncher.Edit` edits a `mint-notes-*.md` temp copy, never the saved path
  - README.md:109 (release section), README.md:256 (`on_notes_failure` row), internal/config/metadata.go:134 (`on_notes_failure` SoT row), internal/setupguide/setupguide.go:179-182 (AI-transport paragraph)
- Notes: Behaviour matches every criterion and the §12.2 corrigenda. Production wiring supplies both seams (cmd/mint/main.go:192-193). Spinner bracketing at the floor is safe: `ResumeSpinner` is a no-op when nothing was suspended (internal/presenter/pretty.go:457-460), so the editor hand-off after the notes ✗ line restarts nothing.

TESTS:
- Status: Adequate
- Coverage:
  - AC1: `assertFloorFollowsFailure` (internal/engine/notes_floor_test.go:51-73) pins keys c/e/a and default a; `TestRelease_NotesFloorEdit_TheChoiceNamesWritingInTheEditor` pins the action labels
  - AC2: `TestRelease_NotesFloorEdit_OpensOnTheChunkNotesThisRunSaved` — part-failed case wants `partOneSaved` (marker included, notes_savedchunknotes_test.go:113), merge-failed case wants every part's notes; both also compare against the file on disk
  - AC3: `TestRelease_NotesFloorEdit_SinglePassFailureOpensEmpty`
  - AC4: `TestRelease_NotesFloorEdit_UnsavedChunkNotesOpenEmptyOverAnEarlierFile` — pre-saves an earlier file, makes the dir read-only so the atomic rename fails, asserts the one warning, the earlier file untouched, and an empty editor. It would fail if the floor read the path from disk
  - AC5: `TestRelease_NotesFloorEdit_SavedBodyReleasesByteIdenticalAfterTheReviewGate` — review gate question and keys, then tag annotation, CHANGELOG section and `gh release create` stdin each equal `editedBody`
  - AC6: `TestRelease_NotesFloorEdit_LeavesTheSavedChunkNotesAsTheWaveSavedThem` — released and aborted-at-review-gate cases
  - AC7: `TestRelease_NotesFloorEdit_EmptyBodyReturnsToTheChoice` — empty and whitespace-only; floor shown twice, one review gate after `c`, tag body is the commit-subject list
  - AC8: `TestRelease_NotesFloorEdit_NoEditorInstalledWarnsAndReturnsToTheChoice` — the real `EditorLauncher` over a FakeRunner seeded not-found; exact warning, choice re-shown after it, no review gate, no tag
  - AC9: `TestRelease_NotesFloorEdit_FailedEditorAbortsAndUnwindsThePreTagCommit` — exit 1, stages notes then edit, edit message verbatim, no tag, `git reset --hard <start>`, unwound summary
  - AC10/AC11: internal/config/notes_floor_docs_test.go (`TestREADME_ReleaseSectionDescribesTheFloorsEdit`, README row, metadata row) and internal/setupguide/setupguide_test.go:848-874 (config-reference row; AI-transport paragraph with no flag)
- Notes: Tests are behaviour-level, one per criterion, with no redundant variants. Each would fail if its behaviour broke.

CODE QUALITY:
- Project conventions: Followed. All output goes through the presenter, the editor goes through `runner.RunInteractive` behind the Editor seam, failures are surfaced verbatim, and the unwind runs on every abort before the push
- SOLID principles: Good. One `notesFloor` loop is parameterised by gate and subjects, and release and regenerate both reuse it
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: The Editor interface's method doc describes a missing editor as an abort, which the floor (and the review gate) contradicts. See FINDINGS.

BLOCKING ISSUES:
- None

FINDINGS:
- [in-scope] [contained] internal/engine/release.go:65-67 — The `Edit` method doc says "A non-nil error means no edit could be obtained (e.g. no editor on PATH); the engine surfaces it and aborts". This task widened that interface doc to cover "an `e` choice" at the floor as well as the review gate. A missing editor actually returns `ErrEditorReturnToGate` (internal/engine/editor.go:113-118), and both consumers re-present their gate without surfacing or aborting (internal/engine/notesfloor.go:165-166, internal/engine/release.go:1802-1808). Replace the sentence with: "A non-nil error means no edit could be obtained: ErrEditorReturnToGate (no editor could be launched, already reported) sends the engine back to the gate or choice it came from; any other error is surfaced and aborts." — FAILS: the seam's own contract tells an implementer of a new Editor that every error aborts the run. It never mentions the `ErrEditorReturnToGate` sentinel that both gate loops branch on. A new implementation that reports a missing editor with a plain error would therefore turn the floor's AC8 path (warn and return to the choice) into an abort and unwind. The comment also breaks CLAUDE.md's "true to as-built" comment rule.

UNSETTLED:
- None
