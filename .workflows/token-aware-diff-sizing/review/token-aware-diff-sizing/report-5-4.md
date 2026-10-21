TASK: Next moves for size and no-room failures (token-aware-diff-sizing-5-4)

ACCEPTANCE CRITERIA:
- Unattended release under `abort` stopping because a slice's halving ran out (single-pass call or chunk): next moves add lower `max_prompt_bytes`; if `ai_command` carries a large overhead, use the lean command documented in the README; exclude generated paths with `[release].diff_exclude`
- Same three moves when a halving round had no room (single-pass call or a part)
- The lean-command move points at the README and names none of the lean command's flags (`--safe-mode`, `--tools`, ...)
- No room with `[release].prompt` set (with or without `context`): add shorten `[release].prompt`, never name `[release].context`; with only `context` set: shorten `[release].context`
- No room with neither key set and a cap below mint's own fixed part: add raise `max_prompt_bytes`, no move says lower it
- Merge too large (prompt over the cap, or merge call classified SIZE) with part notes saved at P: always-moves only, led by the saved part notes; no move names `max_prompt_bytes`, the lean command or `[release].diff_exclude`

STATUS: complete

SPEC CONTEXT: §12.4 makes the next moves cause-fitted: the Size moves (lower cap, lean-command pointer, `[release].diff_exclude`) for SIZE with no halving left on a single-pass/chunk call and for a halving round with no room; no room under operator text shortens `[release].prompt` when set (corrigendum 2026-10-19: a set prompt replaces the default prompt and `context`), else `[release].context`; no room with neither key set raises the cap (corrigendum on §8.7); a merge too large takes only the always-moves, led by the saved part notes. §9.7/§10.3 require the lean-command pointer to name no claude flags. The AC's merge example text (`notes couldn't be merged; ...`) is superseded by the 2026-10-20 corrigendum, which makes the guidance the next moves led by `tidy the saved notes at <path>, ...` — the implementation follows the corrigendum.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/notesfloor.go:44-53 (`notesFloorMoves`: always-moves, then `causeMoves`)
  - internal/engine/notesfloor.go:58-61 (`regenerateNotesFloorMoves` shares `causeMoves`)
  - internal/engine/notesfloor.go:79 (`leanCommandMove`: points at the README's "The shipped default", names no flags; README.md:301 holds that heading)
  - internal/engine/notesfloor.go:86-112 (`causeMoves`: `CauseNoRoomForDiff` -> `noRoomMove`; `CauseMergeTooLarge` -> none; `CauseTooLargeForModel` -> the three size moves in AC order)
  - internal/engine/notesfloor.go:117-126 (`noRoomMove`: prompt over context, else raise the cap — mirrors `ResolveInstructions` at internal/notes/prompt.go:125-138)
  - internal/engine/release.go:571 (unattended/dry-run floor prints the moves with the failure and cfg)
- Notes: Traced the cause routing end to end. Halving exhaustion and a halving round with no room both arrive as `ErrTooLargeForModel`: the single-pass first round via `generateHalved` (internal/notes/chunk.go:42-44) and a part's re-cut via `wave.send`'s `code == codeSize` arm, which catches both `slot.halvings >= maxHalvings` and a failed `recut` (internal/notes/chunk.go:217-237). `ErrNoRoomForDiff` arises only from the first wave's cut (internal/notes/chunk.go:29-31, 378-381, 441-443), so it is only ever the operator-text/cap cause. A merge prompt over the cap and a SIZE merge both yield `ErrMergeTooLarge` (internal/notes/chunk.go:70-71, 85-86), and `ClassifyFailure` checks it before `ai.ErrGenerationFailed` (internal/notes/resolve.go:140-156). The saved-file wrapper `chunkNotesSavedError` unwraps (internal/notes/chunknotes.go:41), so the sentinel survives to `causeMoves`. No drift from the spec as corrected.

TESTS:
- Status: Adequate
- Coverage (internal/engine/notes_floor_sizemoves_test.go):
  - AC1/AC2 single-pass: `TestRelease_NotesFloorSizeMoves_HalvingRunsOutAddsTheSizeMoves` (line 34) runs both `halvingOutCases()` (internal/engine/notes_halvebound_test.go:66-73): a first part halved three times (chunk exhaustion) and a single-pass call whose first halving round has no room; asserts the ✗ phrase and the exact move list.
  - AC2 part: `TestRelease_NotesFloorSizeMoves_PartHalvingRoundWithNoRoomAddsTheSizeMoves` (line 60) asserts one overflow, the `(part 1 of ` location, and the exact moves.
  - AC3: `TestRelease_NotesFloorSizeMoves_LeanCommandMovePointsAtTheREADMEWithoutClaudeFlags` (line 91) scans every move for six claude flags; the README pointer is pinned by the exact-match constant (line 17).
  - AC4/AC5: `TestRelease_NotesFloorSizeMoves_NoRoomNamesTheTextToShortenOrRaisesTheCap` (line 118) covers prompt set, prompt+context set, context only, neither set under a 1000-byte cap; `assertStopsWithNextMoves` compares with `slices.Equal` (internal/engine/notes_floor_nextmoves_test.go:48), so "does not name context" and "no lower move" are enforced by exact equality.
  - AC6: `TestRelease_NotesFloorSizeMoves_MergeTooLargeTakesOnlyTheAlwaysMoves` (line 177) covers both merge causes and asserts the moves are exactly the saved-notes route, `--no-ai` and fallback.
  - README docs pinned by `TestREADME_ReleaseSectionNamesTheSizeStopsSettings` (internal/config/notes_floor_docs_test.go).
- Notes: Each test would fail if the cause routing or move text broke. No bloat worth reporting.

CODE QUALITY:
- Project conventions: Followed (output through `presenter.NextMoves`; sentinels matched with `errors.Is` via `ClassifyFailure`; external test package, `t.Parallel()`, exact-output assertions)
- SOLID principles: Good — `causeMoves` is the single cause-to-moves mapping shared by release and regenerate
- Complexity: Low
- Modern idioms: Yes
- Readability: Good; comments at notesfloor.go:40-43, 81-85, 114-116 hold true against the code
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
