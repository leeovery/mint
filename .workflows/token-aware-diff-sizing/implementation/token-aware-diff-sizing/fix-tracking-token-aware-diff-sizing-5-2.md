## Attempt 1

ISSUES:
- `/Users/leeovery/Code/mint/internal/setupguide/setupguide.go:173`: the AI-transport paragraph was only partly reflowed. Line 173 is now 113 columns, while the rest of the guide's prose wraps at about 83. The raw string prints as-is, so `mint setup` shows one long line in that paragraph (line 71 of the output, the only prose line over 90 apart from the interpolated default command), and it wraps raggedly in an 80-column terminal. The tests flatten whitespace, so none catches it.
  FIX: Rewrap lines 171–174 to the file's width, with the wording unchanged:
  ```
  abort loud, where an attended release is offered the commit-subject list, writing
  the notes in the editor, or an abort, and an unattended one stops, or fall back to
  the commit-subject list, the one non-AI body. Commit drops to the user's $EDITOR.
  If you pin a slower model for a verb, raise that verb's timeout in the SAME table —
  mint does not auto-bump it.`
  ```
  The existing guide tests compare whitespace-flattened text and still pass.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `/Users/leeovery/Code/mint/internal/engine/release.go:59` — the touched comment says the seam's return gets "no re-validation", but the notes floor now refuses an empty or whitespace-only return. The block also still carries a task id and a claim about tests.
  OLD: // Editor is the engine's edit seam: an `e` choice hands a notes body to Edit and
// uses whatever it returns VERBATIM (no re-parse, no
// re-validation — a human edit is trusted). The interface is intentionally tiny so
// production can wire the real $EDITOR resolution (task 2-13) behind it while the
// gate-loop tests drive a scripted fake. It is defined HERE (the consumer) rather
// than where it is implemented, per the accept-interfaces convention.
  NEW: // Editor is the engine's edit seam: an `e` choice hands a notes body to Edit and
// uses the text it returns VERBATIM, never re-parsed — a human edit is trusted.
- `/Users/leeovery/Code/mint/internal/engine/chunknotes.go:8` — the edit attaches "so the run can warn" to `saved` as well, but `saved` is what the floor's editor opens on, not something the warning uses.
  OLD: // chunkNotesKeeper saves a failed chunked run's finished chunk notes for one repo's
// tag and remembers what this run saved, or why a save failed, so the run can warn
// once its notes stage has ended.
  NEW: // chunkNotesKeeper saves a failed chunked run's finished chunk notes for one repo's
// tag, remembering what it saved and why a save failed.
- `/Users/leeovery/Code/mint/internal/engine/notes_floor_edit_test.go:43` — the comment gives a selection rule ("offers the edit choice") that the code does not apply. The code selects `Subject == "notes"` gates that have `ChoiceYes`.
  OLD: // reviewGates returns every recorded gate that offers the edit choice without
// being the floor's choice.
  NEW: // reviewGates returns every recorded notes review gate.

NOTES:
- `TestRelease_NotesFloorEdit_UnsavedChunkNotesOpenEmptyOverAnEarlierFile` saves the earlier file through a real `notescache.ChunkNotes`, then swaps `deps.ChunkNotes` for `failingChunkNotes`, which has no link to that file. The "earlier file at the path" setup therefore has no effect, and the test is equivalent to "a save that failed opens empty". It still checks the right outcome. A test closer to the real case would keep the real store and make its write fail (for example, a read-only scope directory) with the earlier file in place.
- The failing-editor test checks the ✗ stage names (`notes`, `edit`) but not the `edit` StageFailed's Message (`launching editor "<name>": exit status 1`), even though that message is deterministic. No test pins the floor gate's new Action label `write in $EDITOR` either. `assertFloorFollowsFailure` checks keys and the default only.
- At the floor, a successful `e` prints nothing before ShowPlan/ShowNotes. The review gate's `e` brackets its edit with an `edit` blocking stage (`applying edited notes…` / ✓). With no spinner live at the floor, the launcher's Suspend/Resume calls do nothing, so there is no functional problem. The floor's `c` likewise prints no ✓ of its own.
- These comments were not touched by the diff but are now incomplete or false: `Editor.Edit`'s method doc (`release.go:66-68`, "no editor on PATH; the engine surfaces it and aborts", which is false for `ErrEditorReturnToGate`), `editBody`'s doc (`release.go:1691-1693`, "so the gate surfaces", plus a task id), and `NewEditorLauncher`'s doc (`editor.go:65-67`, "wires it once", though `cmd/mint/main.go` wires it at lines 191 and 313). They are outside this task's scope.
- `notesFloor` now nests for → switch → case → switch. Task 5-9 adds the same `e` handling to regenerate's floor (missing editor, empty refusal, surface on other failures). Pulling the `e` branch into a shared helper when 5-9 lands would keep the empty-body refusal rule in one place.
- Nothing arrived beyond the enumerated inputs.
