TASK: Next moves for a timeout, a missing binary or another failure (token-aware-diff-sizing-5-5)

ACCEPTANCE CRITERIA:
- Given an unattended release under `abort` that stops because a single-pass, chunk or merge call timed out, the next moves add raise `[release].timeout` (§11.6, §12.4)
- Given one that stops because the `ai_command` program is not installed, the moves add install the AI tool, or set `ai_command` (§12.4)
- Given one whose call failed with output and was classified OTHER, with no `ai_command` set at any level, the moves are the always-moves only, beneath the AI's verbatim output (§12.4)
- Given the same failure with `[release].ai_command = "llm -m gpt-4o"`, or with a shared `ai_command` and none under `[release]`, the moves add the lean-command pointer: use the lean command documented in the README. It names none of the lean command's flags (§9.7, §12.4)
- Given only `[commit].ai_command` set, the release resolves to the shipped default, and no lean-command pointer shows (§2.4, §12.4)
- Given a failure whose captured output is empty, which reads `AI returned empty notes`, the moves follow the OTHER rules (§12.4)

STATUS: complete

SPEC CONTEXT: §12.4 makes the unattended floor's next moves cause-fitted: Timeout adds raise `[release].timeout`; Missing binary adds install the AI tool, or set `ai_command`; OTHER (with or without output) adds nothing of its own because the AI's verbatim message names the cause, except that a resolved release `ai_command` other than the shipped default adds the lean-command pointer (§9.7: a repo command whose overhead alone overflows the window fails its classify call the same way and lands on OTHER). §2.4/§1.4: resolution is `[release]` -> shared -> shipped default with blanks as unset; mint names no claude flags. §9.5: a failed classify call or empty output still takes OTHER, with the original call's output.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/engine/notesfloor.go:79 (leanCommandMove), internal/engine/notesfloor.go:86-112 (causeMoves: CauseTimeout at :102-103, CauseCommandMissing at :104-105, CauseGenerationFailed with `cfg.AICommandFor(config.VerbRelease) != config.DefaultAICommand` at :106-109); appended after the always-moves in notesFloorMoves (internal/engine/notesfloor.go:44-53) and regenerateNotesFloorMoves (internal/engine/notesfloor.go:58-61); called from internal/engine/release.go:571 with the loaded cfg. Cause sorting: internal/notes/resolve.go:140-156; both OTHER phrases ("AI call failed" / "AI returned empty notes") share CauseGenerationFailed (internal/notes/resolve.go:181-196), so the empty-output case follows the OTHER rules by construction.
- Notes: Resolution goes through config.AICommandFor (internal/config/config.go:583), so blank-as-unset and `[commit]`-only overrides fall through to the shipped default as required. A failed classify call returns the original failure (internal/notes/classify.go:41-47, 66-68), so a classify-call timeout cannot be misreported as a Timeout move. The lean-pointer move cites the README section "The shipped default", which exists (README.md:301), and names no claude flag. README.md:111 documents the timeout, missing-tool and OTHER/lean-command moves.

TESTS:
- Status: Adequate
- Coverage: internal/engine/notes_floor_causemoves_test.go covers single-pass timeout and missing binary with phrase plus moves (:28-56); chunk and merge timeouts with located phrases and the saved-notes route (:58-90); OTHER with output and with whitespace-only output under no ai_command, `[commit]`-only ai_command, and blank ai_command at both levels, asserting the always-moves only and the verbatim StageFailure.Output (:108-142); OTHER under `[release].ai_command`, shared ai_command with none under `[release]`, and shared with blank `[release]`, asserting the lean pointer and no claude flags (:144-174); a guard that a timeout under a repo-own command gets no lean pointer (:176-187). assertStopsWithNextMoves (internal/engine/notes_floor_nextmoves_test.go:38-64) pins exact move order, ordering after the ✗ line, exit 1 and no floor gate. The test double's transport returns the same failure for the classify call, so the OTHER-via-failed-classify path is exercised. cmd/mint/regenerate_floor_test.go:78-112 confirms the moves are fitted to the loaded config end to end.
- Notes: Each test would fail if its move were dropped, reordered or leaked to the wrong cause. No redundant cases.

CODE QUALITY:
- Project conventions: Followed (external test package, t.Parallel throughout, table-driven, exact rendered move text asserted, output through presenter.NextMoves)
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
- None
