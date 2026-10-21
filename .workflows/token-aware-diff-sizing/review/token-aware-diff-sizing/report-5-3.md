TASK: Next moves when an unattended release stops (token-aware-diff-sizing-5-3, tick-e530ca)

ACCEPTANCE CRITERIA:
- Given `mint release -y` under `on_notes_failure = abort` whose single-pass call fails with output, mint prints the ✗ line with the AI's captured output verbatim beneath it, then the next moves (write the notes and release with `--notes-file`; `--no-ai`, for the commit-subject list; `on_notes_failure = "fallback"` under `[release]`, to ship the commit-subject list rather than stop). No prompt shows, and the run exits 1 before any tag
- Given that `-y` run with stdin a terminal, and with stdin not a terminal, the output and the exit are the same
- Given a chunked run that failed and saved its chunk notes at P, the `--notes-file` move says to tidy the saved notes at P, filling in any files its marker lists, and release with `mint release --notes-file P`
- Given a chunked run whose chunk notes could not be saved, the move says to write the notes and release with `--notes-file`, and names no path
- Given an attended release under `abort` whose notes fail, no next moves print; the choice shows instead
- The README's `release` section and `mint setup`'s AI-transport paragraph say that an unattended release under `abort` whose notes fail stops with next moves fitted to the cause. The setup guide paragraph names no command flag

STATUS: complete

SPEC CONTEXT: §1.1 names the dead end (an abort with no way forward) this feature removes. §12.1: an unattended (`-y` or non-TTY) release under `abort` renders the ✗ line, the AI's verbatim output and the cause-fitted next moves, then stops, never prompting; an attended one gets the c/e/a choice. §12.4: the Always moves are the `--notes-file` route (tidy the saved file at its path and release with `mint release --notes-file <path>`, or, with no file, write the notes and release with `--notes-file`), `--no-ai` on forward release, and `on_notes_failure = "fallback"` on an unattended forward release only. §11.4 + corrigendum: a failed save keeps the AI's failure unchanged, adds one warning, and the run continues as with no file (no path in the guidance). §18.1/§18.2: the setup guide stays flag-free; README documents the behaviour in the `release` section.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/release.go:562-579 — on a notes failure, `reachesNotesFloor` + `asksAtTerminal` decide between the floor's choice and the stop; the stop renders StageFailed (with output and SavedNotes from `stageFailure`), the unsaved-notes warning, then `p.NextMoves(deps.notesFloorMoves(...))`, then unwinds (AbortError exit 1, before any tag)
  - internal/engine/notesfloor.go:32-38 — `reachesNotesFloor` (cancel and non-generation errors get no moves)
  - internal/engine/notesfloor.go:44-53 — `notesFloorMoves`: notes-file route, `--no-ai`, and the fallback move only when `!d.attended()`, then cause moves
  - internal/engine/notesfloor.go:65-70 — `notesFileMove`: tidy-at-path (path raw in prose, shell-quoted in the command via `notesFileCommand`) or write-from-scratch with no path
  - internal/notes/chunknotes.go:68-77 — a failed save returns the failure without a saved path, so `SavedChunkNotesPath` is "" and the write move names no path
  - internal/presenter/presenter.go:100-102, 562-567 — `NextMoves` event and payload; internal/presenter/plain.go:217-222 (`{stage}: NEXT - {move}` lines), internal/presenter/pretty.go:591-600 (`next moves:` heading, indented list), both to out only; internal/presenter/presentertest/recording.go:232-235 records it
  - README.md:109 and :111 — `release` section: the unattended stop "with next moves fitted to the cause", the three Always moves, the saved/unsaved `--notes-file` wording, and that an attended run gets the choice and prints none
  - internal/setupguide/setupguide.go:180-182 — AI-transport paragraph: "an unattended one stops with next moves fitted to the cause", no flag named
- Notes: Behaviour matches every criterion. The saved-file pointer also prints beneath the ✗ line on the unattended stop (StageFailed carries SavedNotes in every mode), consistent with §12.4's merge example. The unwind narration and exclusion suggestions print after the moves, consistent with §16.3.

TESTS:
- Status: Adequate
- Coverage:
  - internal/engine/notes_floor_nextmoves_test.go:66-95 — single-pass `-y` failure with output, subtests for stdin a terminal and not: asserts the ✗ line's message and verbatim output, exactly the three moves in order, moves after the ✗ line and followed only by the unwind, no floor gate, AbortError exit 1, and no `git tag -a` (AC1, AC2)
  - :97-107 — chunked run with a failed merge and a saved file: tidy move with the saved path and `mint release --notes-file <path>` (AC3)
  - :115-128 — failing ChunkNotesSaver: write move with no path, one warning (AC4)
  - :130-142 — attended run: no NextMoves events, the floor's choice follows the ✗ line (AC5)
  - :144-158 — unattended cancel prints no moves (the `reachesNotesFloor` cancel branch)
  - internal/presenter/nextmoves_test.go — exact plain and pretty rendering, plain byte purity, empty moves render nothing, NextMoves leaves the success line standing (no failure state)
  - internal/config/notes_floor_docs_test.go:70-78 — README `release` section carries the phrase; internal/setupguide/setupguide_test.go (`TestGuide_AITransportParagraphNamesTheFloorChoiceWithoutAFlag`) — paragraph carries it and names none of `-y`, `--yes`, `--no-ai`, `--notes-file`, `--dry-run` (AC6)
- Notes: Each test would fail if its behaviour broke (exact move slices, event order, gate absence, exit code). No redundant or over-mocked tests.

CODE QUALITY:
- Project conventions: Followed — output only through the presenter as a payload-driven event (engine authors the move text, presenter renders verbatim); unattended decision made from the threaded `Yes`/`StdinInteractive` deps, never inside the presenter; exit via AbortError; tests use FakeRunner and RecordingPresenter, external test packages, `t.Parallel()`
- SOLID principles: Good — move composition (engine), rendering (presenter) and save-path plumbing (notes) are separate
- Complexity: Low
- Modern idioms: Yes
- Readability: Good — comments on `notesFloorMoves`, `notesFileMove`, `notesFileCommand` and `reachesNotesFloor` hold against the code
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
