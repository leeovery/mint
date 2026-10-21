TASK: Halving narration (token-aware-diff-sizing-4-5, tick-1be62f)

ACCEPTANCE CRITERIA:
- Given a single-pass call classified SIZE whose re-cut yields 3 parts, in pretty mode the notes line reads `Writing notes: too big for the model in one call, retrying in 3 parts`. As the parts finish it moves to the split line with the composed single-pass size, such as `Writing notes: 1.2 MB release, in 3 parts · 1 of 3 done`, then reads `Writing notes: merging 3 parts`, and the run ends `✓ Notes written (3 parts, merged) ({elapsed})`. In plain mode each finished part prints its own line (§16.1, §16.2)
- Given a chunked run of 6 parts whose part 4 is classified SIZE and re-cut into 3 parts, the notes line reads `Writing notes: part 4 was too big for the model, retrying it as 3 smaller parts`. Its split line then counts out of 8, such as `Writing notes: 3.1 MB release, in 8 parts · 5 of 8 done`, it then reads `Writing notes: merging 8 parts`, and the run ends `✓ Notes written (8 parts, merged) ({elapsed})` (§16.1)
- Given that run, part 4's new parts are numbered 4, 5 and 6, and the parts first packed as 5 and 6 are numbered 7 and 8 from then on. A later re-cut of the part first packed as 5 into 2 parts reads `Writing notes: part 7 was too big for the model, retrying it as 2 smaller parts`, and the run then merges 9 parts (§16.1)
- Given that run with no further halving, where the part first packed as 6 fails with output and is classified OTHER, the ✗ line reads `AI call failed (part 8 of 8)` (§12.4, §16.1)
- Given the same runs in plain mode, each halving update prints as its own line as it happens, alongside the per-part lines (§16.2)
- Given a run with no halving, its narration is as Phase 2 built it (§16.1)
- Given `mint release regenerate <version>` from a fresh source, or a version under `regenerate --all`, whose notes halve, that version's notes line narrates the same way (§1.3, §16.1)

STATUS: complete

SPEC CONTEXT: §16.1 keeps one updated notes line for a chunked run; a single-pass SIZE re-cut shows `too big for the model in one call, retrying in {n} parts`, a part re-cut shows `part {k} was too big for the model, retrying it as {n} smaller parts`. Corrigenda 2026-10-19: the split line's count grows with the total (one total in both places), parts are numbered by place in chunk order with a halved part's new parts taking its place, and a halved single-pass run moves to the split line with the same composed single-pass size. §16.2: plain prints each update as its own line. §16.4: new presenter events, engine never touches spinners/TTY. §10.1: the re-cut target is half the failed call's composed size. §12.4: the ✗ line names the failed part. §1.3: regenerate fresh and --all share halving.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/presenter/presenter.go:95-97 (NotesHalving on the Presenter interface; listed in the stream-discipline exception at :72 and :88-93), :528-540 (NotesHalving payload and the shared halvingText wording)
  - internal/presenter/pretty.go:535-539 (rewrites the running notes line via setSpinnerText, a no-op with no spinner at :549-555)
  - internal/presenter/plain.go:256-259 (own line, `notes: ` prefix, ASCII)
  - internal/presenter/presentertest/recording.go:42, :90-91, :123, :222-225 (KindNotesHalving recorded with its payload)
  - internal/notes/generate.go:31-34 (Progress.Halved contract), :237-240 (single-pass SIZE routes to generateHalved)
  - internal/notes/chunk.go:41-48 (generateHalved reports Halved(0, n) in place of the split; the wave's PartsDone then carries the single-pass promptBytes), :253-261 (wave.halve reports Halved(at+1, len(halves)) under the wave lock, then replaces the slot in place so later parts renumber), :231 (PartsDone counts out of len(w.slots), the grown total), :237/:239 (partCallError names the slot's current place and total, so the ✗ line counts within the grown total)
  - internal/engine/notesstage.go:65-68 (notesStage relays to NotesHalving), :211-215 (regenStage relays for an `r`)
  - internal/notes/resolve.go:184-187 (CauseText appends the callError location, giving `AI call failed (part k of n)`), consumed by failureMessage at internal/engine/release.go:2051
- Notes: Numbering-by-place holds: wave.halve uses slices.Index at the moment of the re-cut and slices.Replace splices the new parts into that position, so a part first packed as 5 sits at index 7 after part 4's 3-way re-cut, and a later Halved/partCallError reads the current index. Single-pass halving emits no Done-0 split, matching "in place of the split". Regenerate fresh and --all reach the same Generator through NotesSinks.wire (internal/engine/regenerate_fresh.go:83-85) with startNotesStage as the Progress, so they narrate identically. The engine emits presenter events only; no spinner or TTY handling outside the presenter.

TESTS:
- Status: Adequate
- Coverage:
  - Presenter rendering (internal/presenter/notesprogress_test.go:175-289): exact pretty spinner texts and ✓ line for the single-pass 3-part halving and the 6-to-8 part halving (AC1, AC2 wording verbatim, including `in 8 parts · 5 of 8 done` and `(64.2s)`), exact plain output for both with byte-purity assertions (AC5), no spinner started without a running notes line. Phase-2 no-halving narration retained at :28-92 (AC6).
  - RecordingPresenter payload recording (internal/presenter/presentertest/recording_test.go:192-212).
  - Generator reports (internal/notes/halvenarration_test.go): single-pass re-cut then each part counted out of its parts at the single-pass size then merge (:163-189); a 6-part run whose part 4 re-cuts, counts before the re-cut out of 6 and after out of the grown total, merge out of the grown total (:191-228, AC2); a later re-cut of the part first packed as 5 named `part 4+added` and the merge counting the doubly grown total (:230-265, AC3); the part first packed as 6 failing OTHER after part 4's re-cut yields CauseText `AI call failed (part total of total)` (:267-301, AC4).
  - End-to-end through the engine (internal/engine/notes_halvenarration_test.go): release single-pass halving event sequence and completion (:103-119), the same in plain (:121-151), a chunk re-cut with counts out of the grown total, merge and completion (:170-227), its plain line (:229-263), regenerate fresh (:272-294) and a halved version under --all between two unhalved versions (:296-327, AC6/AC7).
  - Each test would fail if its behaviour broke: exact strings and payloads are compared, not just presence.
- Notes: Engine/notes tests derive the part counts from the transport's observed re-cut rather than hard-coding 3 or 8, which is correct given the counts depend on packing; the exact 3/8 wording is pinned at the presenter layer. Layering (presenter strings, generator reports, engine wiring) is non-redundant.

CODE QUALITY:
- Project conventions: Followed (presenter is the only output surface; payload-driven event; plain narration byte-pure ASCII; RecordingPresenter extended; external test packages, t.Parallel throughout)
- SOLID principles: Good (one wording helper shared by both renderers; Progress interface extended by one method with a noProgress default)
- Complexity: Low
- Modern idioms: Yes (slices.Index/Replace)
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
