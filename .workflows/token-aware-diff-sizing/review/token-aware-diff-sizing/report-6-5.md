TASK: An `r` that becomes a chunked run narrates as one (token-aware-diff-sizing-6-5, tick-fda44b)

ACCEPTANCE CRITERIA:
- An `r` whose composed single-pass prompt, nudge included, is 1,520,000 bytes and goes to chunking as 2 parts under the default cap: in pretty mode the notes line reads `Writing notes: 1.5 MB release, in 2 parts · {k} of 2 done` as parts finish, then `Writing notes: merging 2 parts`, and ends `✓ Notes written (2 parts, merged) ({elapsed})`
- An `r` whose single-pass call is classified SIZE and re-cut into 3 parts: `Writing notes: too big for the model in one call, retrying in 3 parts`, then the split line with the `r`'s composed single-pass size as parts finish, then `Writing notes: merging 3 parts`, ending `✓ Notes written (3 parts, merged) ({elapsed})`
- A chunked `r` whose part 2 of 4 is classified SIZE and re-cut into 2 parts: `Writing notes: part 2 was too big for the model, retrying it as 2 smaller parts`, and the split line then counts out of 5
- In plain mode, each update prints as its own line as it happens, one per finished part included
- The merge-only `r` on chunked notes, or a single-pass `r` that stays one call, shows no `Writing notes:` line and no `Notes written` line: it narrates as built
- `mint release regenerate <version>` from a fresh source, or a version under `regenerate --all` at a terminal without `-y`, whose `r` becomes a chunked run narrates the same way; an `r` there that makes one call narrates as built

STATUS: complete

SPEC CONTEXT: §16.1 (with its 2026-10-19 corrigenda) has a chunked run hold one notes line and update it as the run moves: the split line takes the composed single-pass prompt the size check measured, a single-pass call that halves shows the halving line and then the split line, a part that halves grows the total, and the ✓ line keeps the elapsed time. §16.2 gives plain mode one line per event. §15.1 has a single-pass `r` re-run the AI path from the size check onward, with the nudge in the instructions, so the ladder can turn it into a chunked run. The merge-only `r` makes one call. The §16.1 corrigendum on `r` says a chunked `r` narrates as any chunked run does, and a one-call `r` keeps the as-built narration. §16.4 says the updates are presenter events and the engine never touches spinners or TTY state. §1.3 extends all of this to regenerate's fresh source and `--all`.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/notesstage.go:159 `regenStage`: the `r`'s notes.Progress. `narrating` (:185) stamps `began`, clears `succeeded` per `r`, and closes an opened line with `mergedCompletion(r.parts)` (:293) only on success, before regenerateBody's ShowNotes. `chunked` (:200) opens the blocking "notes" stage lazily, timed from the `r`'s start. `PartsDone` (:206) and `Halved` (:211) open the stage and then emit NotesProgress/NotesHalving. `Merging` (:217) emits NotesMerging.
  - internal/engine/engine.go:161 `startBlockingStageSince` emits StageStarted{Blocking:true}, then StageSucceeded with Elapsed measured from the `r`'s start.
  - Wiring: Release (internal/engine/release.go:607-608), RegenerateRun (internal/engine/regenerate_interactive.go:255, reaching internal/engine/regenerate_write.go:255 via reviewFresh), and the batch's processOneVersion (internal/engine/regenerate_batch.go:336, then gatePerVersion → gateRegenerate → reviewFresh). In each, the same `regens` is both the generator's Progress sink (through chunkNotes.regenRun) and the narrating wrapper.
- Notes:
  - Split size: the split line's size comes from `len(prompt)` where `prompt := ComposePrompt(instructions, …)` (internal/notes/generate.go:226), and `instructions` already carries the nudge. The size is therefore the `r`'s composed single-pass prompt, nudge included, as the criteria require.
  - Merge-only `r`: Remerge goes through sendMerge, not g.merge, so it reports no progress and opens no line. A single-pass `r` that succeeds reports only CallSucceeded. Both narrate as built.
  - Halved single-pass `r`: the line opens at the halving event, after the failed single-pass call. Opening any earlier would break the as-built narration of a one-call `r`, so this ordering is right.
  - Concurrency: the wave's progress calls run under its mutex, and the wave returns only once nothing is in flight. `narrating`'s read of `succeeded`/`parts` after Regenerate returns is therefore ordered after every write.
  - §16.4: the engine only emits presenter events and never touches a spinner.

TESTS:
- Status: Adequate
- Coverage (internal/engine/regen_narration_test.go):
  - Release, pretty events:
    - Over the cap at the size check: exact NotesProgress{1_520_000, 2, k} for k=0..2, Merging(2), then the merged completion with Blocking (:151).
    - Single-pass SIZE re-cut into 3: Halving(0,3), progress at the overflowed call's size, Merging(3), completion (:169).
    - Part 2 of 4 re-cut: Halving(2,n), and progress counts out of the grown total (:184).
  - Plain mode: the same three runs printed line by line through the real PlainPresenter, including `1.5 MB` and the per-part lines (:275, :293, :313).
  - One-call `r`s: the merge-only `r` and the single-pass `r` in one call produce no second notes stage and no progress, halving or merging events after the first notes are shown (:352).
  - Regenerate: single-version and `--all` (no `-y`), both the halved chunked `r` and the one-call `r` (:387, :400, :412, :423).
  - Pretty text rendering of these events (`Writing notes: …`, `✓ Notes written (n parts, merged) (elapsed)`) is pinned by the presenter-level tests in internal/presenter/notesprogress_test.go.
  - The tests would fail if the stage were not opened, the size excluded the nudge, the total failed to grow, or the line closed after the notes were shown.
- Notes: The tests are focused and map one-to-one onto the criteria; none is redundant. AC3's test computes the re-cut count from the run instead of pinning it to 2, but it does verify the grown-total mechanism the criterion names.

CODE QUALITY:
- Project conventions: Followed. Output goes only through presenter events (seam 3, §16.4), and the elapsed time is measured by the engine. Tests use external packages, t.Parallel and the recording/plain presenters.
- SOLID principles: Good. regenStage owns the `r`'s narration and the presenters own the rendering. `mergedCompletion` is shared between the first attempt and the `r`, so the two completion lines cannot drift apart.
- Complexity: Low
- Modern idioms: Yes
- Readability: Good. The comments on regenStage, narrating and chunked match the code as built.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
