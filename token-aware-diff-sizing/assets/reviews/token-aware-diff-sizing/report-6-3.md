TASK: `r` on single-pass notes re-runs from the size check (token-aware-diff-sizing-6-3, tick-1e4b73)

ACCEPTANCE CRITERIA:
- Given single-pass notes and an `r` whose nudged prompt stays within the cap, the AI tool receives exactly one single-pass call: the first attempt's prompt with the nudge appended to its instructions, as built (§7.2, §15.1)
- Given single-pass notes whose composed prompt was 1,499,800 bytes, and a nudge that brings the `r`'s composed prompt over 1,500,000 bytes, no single-pass call is sent. The AI tool receives chunk calls, none bigger than the cap, then one merge call, each carrying the nudge in the release's instructions. The merge's output is shown as the new notes and the review gate asks again (§4.1, §8.6, §15.1)
- Given such an `r` whose diff holds a file too big for one chunk call's room, that file's piece calls carry the nudge too (§8.4, §15.1)
- Given notes an earlier `r` turned into a chunked run, a further `r` makes exactly one call, a merge over the parts that `r` produced, carrying only the new nudge (§15.1)
- Given a real run that cached its single-pass notes, an `r` with an empty nudge makes its AI call and does not ask `Cached notes found for this diff — use them?` (§7.2, §15.1)
- Given `mint release regenerate <version>` from a fresh source, or a version under `regenerate --all` at a terminal without `-y`, with single-pass notes, `r` makes one single-pass call when its nudged prompt fits the cap, and becomes a chunked run carrying the nudge when it does not (§1.3, §15.1)

STATUS: complete

SPEC CONTEXT: §15.1 (with the 2026-10-19 corrigendum) has `r` on single-pass notes re-run the AI path from the size check onward (§7.2 steps 3-5). The nudge goes into the release's instructions, so it counts toward the size check, is part of every call's fixed part, and every chunk, piece and halving-round call and the merge carry it. The `r` skips the exclusion check and cache reuse (§7.2 steps 1-2). An `r` the ladder turns chunked leaves chunked notes, so a further `r` re-merges only (§15.1, task 6-1). Fresh-source regenerate, single-version and per version under `--all` at a terminal without `-y`, follows the same rule (§1.3, §15.1 "On regenerate"). §4.1: no call is ever bigger than the cap.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/generate.go:225-241: `generateFromDiffWithContext` appends the nudge (`appendOneTimeContext`) before `ComposePrompt`, so the size check at :227 measures the nudged prompt. Over the cap it calls `generateChunked` with the nudged `instructions`. Within the cap it makes one single-pass call. `chunkedBody` keeps merge inputs with `releaseInstructions` (nudge left out), so a later re-merge carries only its own nudge.
  - internal/notes/chunk.go:29, :69, :199-201, :359-370: the cutter holds the nudged instructions. Room per call is the cap minus `fixedPart`, which includes them. Chunk, piece and merge prompts are all composed from `cut.instructions`.
  - internal/engine/chunknotes.go:115-125: `RegenRun.regenerator` re-merges when the keeper holds merge inputs. Otherwise it runs the verb's size-check generation over a generator wired to the run's sinks (`NotesSinks.wire`, internal/engine/regenerate_fresh.go:83-85). That wiring makes a chunked `r` hand its merge inputs to `chunkNotesKeeper.keepMerged`, so the next `r` re-merges.
  - internal/engine/release.go:1340-1350: the forward `perRunRegenerator` routes the size-check generation through `GenerateWithContext`. It never touches `SelectBodyWithReuse` or `cacheReuse` (release.go:1037, :1255), so an `r` cannot offer cached notes. The shared Assembler keeps the exclusion answers (internal/notes/select.go:249-258), so the `r` re-assembles the same post-exclusion diff without re-running the check.
  - internal/engine/regenerate_fresh.go:98-112: `RegenerateFreshRegenerator` routes through `GenerateFromRangeWithContext`. It is wired for single-version runs (internal/engine/regenerate_interactive.go:262; cmd/mint/regenerate_run.go:100-105) and per version under `--all` (internal/engine/regenerate_batch.go:463-479; cmd/mint/regenerate_all.go:70, :131-138). The `--all` gate is skipped only under `-y` (regenerate_batch.go:464).
- Notes: No drift. The `r` re-assembles the diff and Change Map before the size check, which is the as-built AI path and not a re-run of the exclusion check, so §15.1 holds.

TESTS:
- Status: Adequate
- Coverage:
  - AC1: internal/engine/release_regen_sizecheck_test.go:187 asserts exactly two prompts and that the `r`'s prompt equals the first prompt with the nudge spliced into its instructions.
  - AC2: release_regen_sizecheck_test.go:211 pins the first prompt at exactly 1,499,800 bytes and the nudged single-pass size over the cap. `assertChunkedRegen` (:148) checks that every call before the last is a chunk call within the cap, opening with the nudged instructions, and that the last call is a diff-free merge carrying the nudge with the parts in order. The test also checks the shown bodies, two review-gate asks and the tag body.
  - AC3: release_regen_sizecheck_test.go:248 uses a single near-cap file and asserts at least two piece calls, each within the cap and carrying the nudge, plus the nudge on the merge.
  - AC4: release_regen_sizecheck_test.go:284 compares the re-merge prompt byte-for-byte to the chunked `r`'s merge with only the new nudge (`assertReMergeWithOnlyTheNewNudge`, :176), and checks that the re-merge carries no diff and that git diff assemblies number 2.
  - AC5: release_regen_sizecheck_test.go:312 wires the real-run cache, asserts the cache entry holds the first notes, the empty-nudge `r` prompt equals the first prompt, and no `Cached notes found…` gate is asked.
  - AC6: internal/engine/regenerate_regen_sizecheck_test.go:62, :81 and :101 cover single-version fresh runs within the cap, over the cap, and a further `r` re-merging (exact notes-git invocation count). :121 and :140 cover `--all` with Yes=false, within the cap and over the cap followed by a re-merge.
- Notes: Each test would fail if the behaviour broke. A wrongly sent single-pass call fails both the `partFraming` check and `sizeCheckTransport`'s scripted single-pass budget. A cache consult would show up in `promptGates`. Extra `r` calls break the exact git and merge counts. The tests stay focused: one per criterion and consumer, with no redundant variations.

CODE QUALITY:
- Project conventions: Followed. Every AI call goes through the `notes.Transport` seam, every git call through the runner, tests use `package engine_test` with `t.Parallel()`, and exact prompt bytes are asserted.
- SOLID principles: Good. `RegenRun.regenerator` holds the one `r` rule shared by forward and regenerate, and each verb injects only its size-check generation (`regenerateFunc`).
- Complexity: Low
- Modern idioms: Yes
- Readability: Good. The doc comments on `GenerateWithContext`, `GenerateFromRangeWithContext`, `generateFromDiffWithContext`, `perRunRegenerator` and `RegenRun.regenerator` match the code.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
