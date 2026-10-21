TASK: An unattended regenerate stops with next moves (token-aware-diff-sizing-5-11)

ACCEPTANCE CRITERIA:
- Given `mint release regenerate v1.2.0 --source fresh --target release -y` whose notes fail with output, mint prints the ✗ line with the AI's captured output verbatim beneath it, then the next moves. It writes nothing to any target, shows no prompt, and exits 1. The same holds with stdin not a terminal and no `-y` (§12.3)
- Given `on_notes_failure = "fallback"`, the run stops the same way, and the commit-subject list is written to no target (§12.3)
- Given a chunked run that saved its chunk notes at P, the `--notes-file` move says to tidy the saved notes at P, filling in any files its marker lists, and run `mint release regenerate <version> --notes-file P` for this version (§11.4, §12.4)
- Given no saved file, including when the save failed, the move says to write the notes and release with `--notes-file`, and names no path (§11.4, §12.4)
- Given any of these runs, no move names `--no-ai` or `on_notes_failure` (§12.4)
- Given a regenerate that stops because a call timed out, the moves add raise `[release].timeout`. Given one that stops because a slice's halving ran out, they add the size moves a release shows: lower `max_prompt_bytes`, the lean-command pointer and `[release].diff_exclude` (§1.3, §12.4)

STATUS: complete

SPEC CONTEXT: §12.3 has regenerate ignore `on_notes_failure`; an attended single-version run gets the c/e/a choice, an unattended one (`-y` or non-TTY) stops with guidance as release does under `abort`, never shipping the commit-subject list. §12.4 makes the `--notes-file` route always present (tidy the saved file at its path and run `mint release regenerate <version> --notes-file <path>` once the version is out; otherwise write the notes and release with `--notes-file`), excludes `--no-ai` and the `fallback` move from regenerate, and fits cause moves to the same `[release]` keys (§1.3: regenerate rides on `[release]`). §11.4 and its 2026-10-19 corrigendum: no file is named when no chunk finished or the save failed; a failed save adds one warning with the write error verbatim and the run goes on as when no file was saved.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/regenerate_interactive.go:233-246 — on a body-production failure, `offersChoice` is `reachesNotesFloor(err) && deps.attended()`; when false the ✗ line is emitted (StageFailed, Recoverable=false), the unsaved-notes warning follows (`chunkNotes.warnUnsaved`), and a floor failure prints `regenerateNotesFloorMoves` before `abort(err)` (exit 1). Nothing reads `on_notes_failure`, so `fallback` stops the same way.
  - internal/engine/notesfloor.go:55-61 — `regenerateNotesFloorMoves` leads with `notesFileMove("mint release regenerate "+version, savedNotes)` then `causeMoves(failure, cfg)`; no `--no-ai` or `on_notes_failure` move.
  - internal/engine/notesfloor.go:65-70 — saved path gives "tidy the saved notes at P, filling in any files its marker lists, and release with mint release regenerate <version> --notes-file P" (path shell-quoted where needed); no path gives "write the notes to a file and release with ... --notes-file <file>".
  - internal/engine/notesfloor.go:86-112 — shared cause moves: timeout -> "raise [release].timeout"; too-large-for-model -> lower `max_prompt_bytes`, lean-command pointer, `[release].diff_exclude`.
  - cmd/mint/main.go:246,251 — `VersionKey` (bare x.y.z, which `version.ParseSemVer` accepts as the regenerate argument) and the loaded `cfg` are threaded into the request.
- Notes: `failure.SavedNotes` comes from `notes.SavedChunkNotesPath`, which is "" when the save failed (`saveFailedChunkNotes` returns the bare failure, internal/notes/chunknotes.go:72-75), so the failed-save case falls to the no-path move. A cancel (`reachesNotesFloor` false) prints no moves. The fresh producer (internal/engine/regenerate_fresh.go:66) calls the generator directly and never routes through `notes.ResolveFailure`, so no fallback body can be built. README.md:165 describes the as-built behaviour accurately.

TESTS:
- Status: Adequate
- Coverage: internal/engine/regenerate_floor_nextmoves_test.go covers every criterion. `assertRegenStopsWithNextMoves` (lines 59-86) checks exit 1, a single notes NextMoves event equal to the expected list, no `--no-ai`/`on_notes_failure` move, ✗ before moves with nothing after, zero Prompt calls, and no provider dispatch or push. Both unattended modes (-y at a TTY; non-TTY without -y) run for the output-verbatim and fallback cases (lines 88-128); the fallback test also proves `git log --format=%s v1.1.0..v1.2.0` never runs on the deps runner. Saved file after a failed part and after a failed merge (130-150). No saved file for single-pass failure, a wave with no finished part, and a failed save with its one warning (152-181). Timeout (183-197), both halving-runs-out shapes (199-220), and an OTHER failure with and without a repo-own `ai_command` (222-245). Ctrl-C prints no moves (247-266), and an attended run gets the choice instead (268-281). cmd/mint/regenerate_floor_test.go:54-114 pins exit code 1 and no gate under `-y` through the cmd layer, and that the loaded `.mint.toml` reaches the cause moves.
- Notes: Each test would fail if its behaviour broke: an exact-list move comparison, an exact-argv subject-list check, and event ordering. No redundant cases.

CODE QUALITY:
- Project conventions: Followed. Output goes through presenter events, and the unattended branch is the engine's decision from the startup `Yes`/`StdinInteractive` signals (CLAUDE.md seam 4). The AI output travels verbatim in `StageFailure.Output`. Tests use the FakeRunner and RecordingPresenter, run in parallel, and are table-driven.
- SOLID principles: Good. The move builders are shared with release (`notesFileMove`, `causeMoves`), and only the verb-specific "always" list differs.
- Complexity: Low
- Modern idioms: Yes
- Readability: Good. The doc comments on `RegenerateRun`, `regenerateNotesFloorMoves` and `RegenerateRunRequest.Config` hold against the code.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
