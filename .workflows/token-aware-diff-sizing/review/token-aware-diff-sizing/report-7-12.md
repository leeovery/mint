TASK: A failed check warns and the release carries on (token-aware-diff-sizing-7-12, tick-b4fafc)

ACCEPTANCE CRITERIA:
- Given the check's call exits non-zero after the transport's retry, with `Not logged in · Please run /login` on stdout, mint warns `couldn't check for files that don't belong in the notes` with that output verbatim beneath it, before the notes are written, and makes no classify call for the check (§6.8, §9.1)
- Given the check's call times out, or the AI tool is not installed, the same warning shows and the release carries on to its notes (§6.8)
- Given an answer that is neither "none" nor yields one usable suggestion, such as one whose every glob matches no uncovered changed path, the same warning shows with the AI's answer verbatim beneath it; no pause shows and no second check call is made (§6.4, §6.8)
- Given a failed check, the release carries on to its notes with no pause, and no suggestion prints after the notes, under `-y` included (§6.8)
- Given a check that failed because the AI tool is not installed, the notes call that follows fails the same way through the release's own notes failure path, with its own ✗ line, `AI tool not installed` (§6.8, §9.1)
- Given a Ctrl-C during the check's call, the run is cancelled before any notes call, and no warning about the check shows (§6.8)

STATUS: incomplete

SPEC CONTEXT: §6.8 counts the check as failed on a generation failure, a timeout, a missing binary, an unreadable answer (§6.4) or a list that will not fit (§6.2, task 7-13). In each case the release carries on with no suggestions after one warning line, `couldn't check for files that don't belong in the notes`, with the AI's captured output verbatim beneath it. The classifier never runs for the check (§9.1). A Ctrl-C cancels the run. §16.3: the warning prints when the check fails, before the notes are written, and is not a follow-up. §16.1: the pretty notes line stays live and updates through a chunked run, because a line that sits for minutes looks the same whether mint is working or hung.

IMPLEMENTATION:
- Status: Implemented, with one pretty-mode defect (below)
- Location:
  - internal/notes/exclusioncheck.go:134-151: `Checking`/call/`Checked`. A `context.Canceled` is returned unchanged (137-139). Any other transport error goes to `CheckFailed(failureOutput(err))` and returns nil, nil (140-143). "none" is matched case-insensitively after trimming (144). An answer with no usable suggestion goes to `CheckFailed(answer with trailing whitespace trimmed)` (147-151). The pauses and report run only after those returns, so a failed check never pauses or reports.
  - internal/notes/exclusioncheck.go:161-167 `failureOutput`: `GenerationError.Output()` when the error carries it, otherwise "" (timeout, missing binary).
  - internal/engine/notesstage.go:51-59 `CheckFailed`: `Warn{Label: "notes", Message: "couldn't check for files that don't belong in the notes", Output: output}`, bracketed by `SuspendSpinner`/`ResumeSpinner`.
  - internal/engine/release.go:1017-1022: the check gets the raw `aiTransport` (release.go:1317-1322). The classifier lives in the Generator's failure path, so it cannot run for the check.
  - internal/notes/select.go:214-217: the check runs before the reuse hook and the notes call, and a cancel ends the selection before any notes call.
- Notes: The engine side matches every criterion. The warning reaches the presenter as intended, but in pretty mode the bracket around it does not bring the notes line back (see FINDINGS).

TESTS:
- Status: Adequate for the engine contract. Under-tested where the bracket meets the pretty presenter.
- Coverage:
  - AC1: internal/engine/release_exclusioncheckfail_test.go:84-113 runs the real transport over the FakeRunner. It asserts two check invocations (attempt plus retry), exactly one later claude call and none of them a classify prompt, one warning carrying `notLoggedIn` verbatim, the warning opening the notes stage after the check (Suspend/Warn/Resume first, then more events), and the notes in the tag.
  - AC2: :115-142 covers `ErrTimeout` and `ErrCommandMissing` as subtests. It asserts the warning with empty output, one notes call that is not a classify call, and the notes body shipped.
  - AC3: :144-166 runs attended with globs matching nothing. It asserts the answer verbatim as output, no pauses, one check call and one notes call.
  - AC4: :168-205 covers attended, `-y` and non-TTY. It asserts no pauses and no `ExclusionSuggestions` block, and that the notes ship.
  - AC5: :207-236 seeds every claude call with `ErrCommandNotFound`. It asserts one warning, one notes call, a `StageFailed` reading `notes` / `AI tool not installed`, and the warning before the ✗.
  - AC6: :238-265 cancels inside the check call. It asserts `context.Canceled`, no notes prompts and no warning.
  - Unit tests in internal/notes/exclusioncheck_test.go:217-364 cover the narration order for each failure kind, the unreadable-answer shapes (with pauses and with report), "none" variants, and a cancel narrating no failure.
- Notes: Each engine test would fail if its behaviour broke. Nothing is over-tested; the subtests vary only the failure kind. The gap: every engine test records events on `RecordingPresenter`, and no pretty presenter test drives `SuspendSpinner` → `Warn` → `ResumeSpinner` inside a blocking stage. That is why the defect below goes unseen.

CODE QUALITY:
- Project conventions: Followed. Output goes through the presenter, subprocess output travels verbatim in `Warning.Output`, a cancel propagates unchanged, and the consumer does not retry again.
- SOLID principles: Good. The notes package narrates through the narrow `CheckNarration` interface, and the engine maps it to presenter events.
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: The Suspend/Warn/Resume bracket in `CheckFailed` does not resume the pretty spinner (FINDINGS).

BLOCKING ISSUES:
- In pretty mode, a failed check kills the notes stage's live line for the rest of the stage. The single-pass notes call runs with no spinner, and a chunked run's split, halving and merging updates (§16.1) never show until the ✓ line. Remedy: the FINDINGS entry at internal/engine/notesstage.go:52-58.

FINDINGS:
- [in-scope] [spreading] internal/engine/notesstage.go:52-58: `CheckFailed` brackets its `Warn` with `SuspendSpinner`/`ResumeSpinner` so the notes line comes back afterwards, but the pretty presenter undoes the bracket. `SuspendSpinner` stops the spinner and sets `spinnerSuspended = true` (internal/presenter/pretty.go:437-445). `PrettyPresenter.Warn` then calls `p.stopSpinner()` (internal/presenter/pretty.go:691), which sets `spinnerSuspended = false` (internal/presenter/pretty.go:415). `ResumeSpinner` then returns without restarting anything (internal/presenter/pretty.go:458-460). With `activeSpinner` nil, every later `NotesProgress`/`NotesHalving`/`NotesMerging` is dropped by `setSpinnerText` (internal/presenter/pretty.go:548-551). The fix has more than one defensible shape. One is to make `Warn` stop a live spinner without clearing a pending suspend, so a bracketed Warn resumes. That needs a check of the other Warn-inside-a-bracket paths (the editor and gate brackets), where resuming would now also happen. The other is a dedicated presenter event for the check failure that renders without ending the line. Whichever shape is chosen, it should land with a pretty presenter test, for example beside internal/presenter/notescheck_test.go:46: drive StageStarted(Blocking) → NotesChecking → NotesChecked → SuspendSpinner → Warn → ResumeSpinner → NotesProgress, and assert a live spinner whose text becomes the `Writing notes: … in N parts · 0 of N done` line. — FAILS: in pretty mode at a TTY, any release whose check fails prints the ⚠ line and then shows nothing while the notes are written. A single call can take minutes, and a chunked run's progress never appears until `✓ Notes written (…)`, which is the "looks hung" state §16.1 exists to prevent. Plain mode and the RecordingPresenter-based engine tests do not see this, since plain's Suspend/Resume are no-ops and the recorder only checks event order.

UNSETTLED:
- None
