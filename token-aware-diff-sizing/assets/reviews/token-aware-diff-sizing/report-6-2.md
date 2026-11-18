TASK: Regenerate's `r` on chunked notes re-runs only the merge (token-aware-diff-sizing-6-2)

ACCEPTANCE CRITERIA:
- Given `mint release regenerate v1.4.0` from a fresh source whose notes were written as 5 parts and a merge, choosing `r` at regenerate's review gate with a nudge makes exactly one AI call. It carries the release's instructions with the nudge in them, v1.4.0's Change Map, its five parts' notes in chunk order, the merge framing and the closing reminder. No chunk call and no single-pass call is made (§8.6, §15.1)
- Given that call returns a body, it is shown as the new notes and regenerate's review gate `Use these notes?` asks again. Accepting it writes that body byte-identical to the chosen target (§15.1)
- Given a second `r` on v1.4.0 with another nudge, its one call carries the same five parts' notes and only the second nudge (§15.1)
- Given `regenerate --all --source fresh` at a terminal without `-y`, where v1.2.0 and v1.3.0 were both chunked, `r` at v1.3.0's review gate makes one merge call carrying v1.3.0's part notes and none of v1.2.0's (§15.1)
- Given a version whose single-pass call was classified SIZE and re-cut into parts, `r` at its review gate makes one merge call over those parts' notes (§10.1, §15.1)

STATUS: complete

SPEC CONTEXT: §15.1 gives a single-version fresh regenerate its own review gate whose `r` behaves like release's: on chunked notes it re-runs only the merge call over the per-chunk notes already produced, with the operator's nudge. `regenerate --all` at a terminal without `-y` applies the same rule per version. §8.6 fixes what the merge call carries (instructions, Change Map, every chunk's notes in order, merge framing, closing reminder). §10.1 makes a SIZE-classified single-pass call a chunked run at half size, so its notes are chunked notes too. §7.2 says regenerate neither reads nor writes the dry-run cache, so a version's part notes come only from its own wave in this run. §1.3/§2.2 send every regenerate AI call through the `[release]` command and timeout. §18.3 requires comments on a changed contract to be rewritten in the same change.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/chunknotes.go:109-125 — `RegenRun.regenerator`: while `run.Merged()` returns merge inputs, an `r` makes one `Remerge` call. Otherwise it runs the verb's size-check generation.
  - internal/engine/chunknotes.go:43-45, 97-103 — the keeper records the merge inputs of the chunked run that wrote the body (`keepMerged`), and `regenRun` reads them live (`heldMerged`).
  - internal/notes/remerge.go:47-57 — `Remerge` composes the merge prompt from the stored release instructions with only this call's one-time context appended, plus the stored Change Map and part notes. A prompt over the cap is never sent.
  - internal/notes/remerge.go:59-70 and internal/notes/generate.go:225-240 — both the over-cap chunked path and the SIZE-halved path hand `chunkedBody` the release instructions without the nudge. So every re-merge carries only its own nudge, and AC5's halved notes re-merge the same way.
  - internal/engine/regenerate_fresh.go:98-112 — `RegenerateFreshRegenerator` builds the `r` rule over `freshGenerator`. That generator uses `resolveFreshTransport` with `config.VerbRelease` (regenerate_fresh.go:148-153), so the re-merge uses the release `ai_command` and timeout.
  - internal/engine/regenerate_interactive.go:230-231, 262 — single version: one keeper per run, wired into body production and into `ProduceRegenerator`.
  - internal/engine/regenerate_batch.go:316-317, 475 — under `--all`, a new keeper is built for each version inside `processOneVersion`, so v1.3.0's `r` can only reach v1.3.0's merge inputs.
  - cmd/mint/regenerate_run.go:100-105 and cmd/mint/regenerate_all.go:131-138 — production producers bind `RegenerateFreshRegenerator` over the passed `RegenRun` for the fresh source.
  - internal/engine/regenerate_write.go:250-257 — `reviewFresh` runs `presenter.NotesReviewGate()` with the narrated regenerator. On regenerate, `Remerge` makes no progress calls, so a single re-merge call narrates nothing (consistent with `regenStage`'s doc at notesstage.go:154-158).
- Notes: The doc comments the task named (`RegenerateFreshRegenerator`, `freshGenerator`, `regenerateRegenerator`, `RegenerateWriteRequest.Regenerator`) and `GenerateFromRangeWithContext` (internal/notes/generate.go:154-166) now describe the merge-only `r` correctly. Regenerate never touches the notes cache, as §7.2 requires.

TESTS:
- Status: Adequate
- Coverage:
  - internal/engine/regenerate_remerge_test.go:144 (AC1): asserts 7 calls (five chunk calls, the merge, one re-merge) and that the 7th call is the re-merge. It also asserts that the re-merge prompt equals the first merge prompt with the nudge appended to the instructions. That equality pins the Change Map, framing, part notes and reminder against the first merge. It asserts no diff in the re-merge, parts in order via `assertPartsInOrder`, and no extra git calls (3, i.e. no re-diff).
  - :174 (AC2): asserts both bodies are shown in order, the review gate is asked twice, and the release body written is byte-identical. The bodies carry trailing whitespace to catch trimming.
  - :192 (AC3): asserts the second re-merge is the first merge plus only the second nudge, with neither the first nudge nor the first re-merge's output in it.
  - :221 (AC4): `--all` without `-y`, each version with its own transport and git. Asserts v1.3.0's re-merge equals its own merge plus the nudge, carries no "v1.2.0 notes", and that the written bodies are correct.
  - :280 (AC5): the halved single-pass case makes 7 calls (single-pass, classify, three chunks, merge, re-merge). The re-merge is over 3 parts.
  - cmd/mint/regenerate_remerge_test.go:68, :78: both production producers re-merge in exactly one call through the `[release]` ai_command (argv `release-ai --notes`, not the shared or `[commit]` command), carrying the nudge and no diff.
- Notes: Each test maps to one acceptance criterion, and the scaffolding is shared through `regenerateV140`/`mergeTransport`. There are no redundant tests. Each test would fail if the re-merge fell back to the AI path (extra chunk or git calls), lost its stored inputs (prompt equality), or carried an old nudge.

CODE QUALITY:
- Project conventions: Followed. AI goes through the notes.Transport seam, and the production transport is built only through `aitransport.New` with VerbRelease. Output goes through the presenter. Tests use external packages, `t.Parallel()`, FakeRunner and RecordingPresenter, and assert exact argv.
- SOLID principles: Good. One `r` rule (`RegenRun.regenerator`) serves release and regenerate. The keeper is the only holder of merge inputs.
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
