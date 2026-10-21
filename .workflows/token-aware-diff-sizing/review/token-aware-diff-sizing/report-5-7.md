TASK: A dry run under `abort` stops with the next moves (token-aware-diff-sizing-5-7, tick-47f942)

ACCEPTANCE CRITERIA:
- Given `mint release --dry-run` at a terminal without `-y`, under `on_notes_failure = abort`, whose notes fail, nothing is asked and no choice shows. mint renders the ✗ line, the AI's captured output verbatim and the next moves, and the run exits 1 (§12.1)
- Given that dry run, the next moves include the `--notes-file` route and `--no-ai`, and not `on_notes_failure = "fallback"`. Given the same dry run with `-y`, or with stdin not a terminal, they also include `on_notes_failure = "fallback"` under `[release]` (§12.1, §12.4)
- Given a dry run whose chunked run failed with parts finished, the chunk-notes file is written as on a real run, at `{user cache dir}/mint/chunk-notes/{repo path, separators as dashes}/{tag}.md`. Its full path shows, and the `--notes-file` move names `mint release --notes-file <that path>` (§11.4, §12.1, §12.4)
- Given a dry run that stops because a call timed out, the moves add raise `[release].timeout`. Given one that stops because a slice's halving ran out, they add lower `max_prompt_bytes`, the lean-command pointer and `[release].diff_exclude`. Each cause adds the moves an unattended real run shows for it (§12.1, §12.4)
- Given a dry run whose chunk notes cannot be saved, the warning that they could not be saved shows with the write error verbatim, as on a real run. The `--notes-file` move says to write the notes and names no path (§11.4, §12.4)

STATUS: complete

SPEC CONTEXT: §12.1 (with its 2026-10-19 corrigendum): a dry run at the floor asks nothing whatever the mode; under `abort` it renders the ✗ line, the AI's verbatim output, the saved chunk-notes pointer and the cause-fitted next moves, then ends as a failed run. The next moves follow the dry run's own mode, so the `fallback` move shows only when the dry run is itself `-y` or non-TTY. The chunk-notes file is saved as on a real run. §11.4 fixes the file's location and the unsaved-file warning (write error verbatim, no path named). §12.4 defines the Always moves (`--notes-file` route via `mint release` while the version is not out, `--no-ai`, `fallback` unattended only) plus the per-cause moves.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/notesfloor.go:24-26 — `asksAtTerminal` gates the floor's choice on `attended() && !opts.DryRun`, so a dry run never prompts
  - internal/engine/notesfloor.go:32-38 — `reachesNotesFloor` no longer excludes dry runs, so a dry-run generation failure reaches the next-moves branch
  - internal/engine/notesfloor.go:44-53 — `notesFloorMoves` adds the `fallback` move only when `!d.attended()` (the run's own mode, independent of DryRun); the `--notes-file` route goes through `notesFileMove("mint release", savedNotes)` (path, or the write-the-notes form when none was saved); cause moves via the shared `causeMoves`
  - internal/engine/release.go:562-575 — the failure branch: StageFailed (Recoverable=false on a dry run, SavedNotes from the cause), `chunkNotes.warnUnsaved`, NextMoves, suggestions, then `Unwind`, which returns `abort(reason)` (exit 1) and emits no Unwound when nothing was made
  - internal/engine/chunknotes.go:29-41 — the chunk-notes saver is wired identically on dry and real runs; cmd/mint/main.go:304-305 and :326 wire `notescache.NewUserChunkNotes(os.UserCacheDir)` regardless of `--dry-run`
- Notes: The dry run reuses the unattended real run's path end to end (same moves builder, same saver, same unsaved warning), so every cause's moves carry over by construction. README.md:111 documents the dry-run behaviour accurately.

TESTS:
- Status: Adequate
- Coverage: internal/engine/notes_floor_dryrun_test.go covers each criterion:
  - AC1: `TestRelease_DryRunNotesFloor_AttendedAsksNothingAndPrintsTheMovesWithoutFallback` (line 12) — attended dry run; asserts the ✗ line phrase and verbatim output, Recoverable=false, zero prompt gates, exact moves, AbortError exit 1, no tag
  - AC2: same test pins the exact move list `[writeNotesMove, noAIMove]` (absence of fallback enforced by slice equality); `TestRelease_DryRunNotesFloor_UnattendedAddsTheFallbackMove` (line 35) runs both `-y` at a terminal and non-TTY
  - AC3: `TestRelease_DryRunNotesFloor_ChunkedFailureSavesTheChunkNotesAndNamesThem` (line 62) — reads the saved file's contents, asserts `StageFailure.SavedNotes` is the full path, and the tidy move names `mint release --notes-file <path>`
  - AC4: `TestRelease_DryRunNotesFloor_TimeoutAddsRaiseTheTimeout` (line 80, attended) and `TestRelease_DryRunNotesFloor_HalvingRunsOutAddsTheSizeMoves` (line 93, `-y`)
  - AC5: `TestRelease_DryRunNotesFloor_UnsavedChunkNotesWarnAndNameNoPath` (line 114) — exactly one warning with the write error verbatim in Output, and the path-free write-the-notes move
- Notes: `assertStopsWithNextMoves` (internal/engine/notes_floor_nextmoves_test.go:38) also pins ordering (moves after the ✗ line, only Unwound after) and absence of floor gates. Each test would fail if the dry-run gating, the mode-dependent fallback move, or the save wiring regressed. No redundant tests; six focused cases.

CODE QUALITY:
- Project conventions: Followed — gates through the presenter, no engine-side prompting on a dry run, attended/unattended decided from the threaded `Yes`/`StdinInteractive` signals as CLAUDE.md seam 4 prescribes; external `_test` package, `t.Parallel()`, exact-move assertions
- SOLID principles: Good — one moves builder serves real and dry runs; mode logic isolated in `attended`/`asksAtTerminal`
- Complexity: Low
- Modern idioms: Yes
- Readability: Good — comments on `asksAtTerminal`, `reachesNotesFloor` and `notesFloorMoves` hold against the code
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
