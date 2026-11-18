# Consolidation Findings: Token Aware Diff Sizing (Phase 6)

## Findings

### F1: Release and regenerate each implement the `r` regenerator rule, and regenerate holds the merge inputs in two places
- **Class**: duplication
- **Failure**: Both verbs follow one rule for `r`: re-merge the merge inputs held for the notes at the gate, else regenerate from the size check through the run's sinks, keeping a chunked result's merge inputs for the next `r`. The rule is written once per verb. On the regenerate side the merge inputs also live in two holders that are kept equal by hand. One is the closure-local `merged` that RegenerateFreshRegenerator re-merges from. The other is the keeper's `merged`, which `keepRegen` saves from when an `r` fails. A change to one verb's copy, or a write to one holder and not the other, compiles and passes every existing test. The operator only sees it when a failed `r` saves the wrong part notes or none, or when `r` on chunked notes re-runs the whole wave instead of one merge call. The copies have already drifted during this phase. Until 6-10, regenerate's `r` ignored the `run.ChunkNotes` it was handed and never updated the keeper, so a failed regenerate `r` on notes the ladder made chunked saved nothing. Until 6-9, release's `r` had no WithChunkNotes. Today 6-10's forwarding callback keeps the two regenerate holders in step; nothing else makes them agree.
- **Evidence**:
  - internal/engine/release.go:1154-1168: perRunRegenerator, copy 1. It reads `chunkNotes.merged` directly (1163) and re-merges through the unwired `generator` (1164). The size-check path goes through `regenerating` (1161).
  - internal/engine/regenerate_fresh.go:98-122: RegenerateFreshRegenerator, copy 2. `merged := run.Merged` (99) is the closure-local holder. The WithMergeInputs callback sets it and forwards to `run.MergeInputs` (100-105). It is read at 112-113, and the re-merge goes through the wired generator.
  - internal/engine/release.go:88-94: `RegenRun.Merged` is a snapshot pointer, taken when the RegenRun is built.
  - internal/engine/chunknotes.go:83-85: `regenRun` copies `k.merged` into that snapshot.
  - internal/engine/chunknotes.go:65-70: `keepRegen` saves from `k.merged` only.
  - internal/engine/regenerate_interactive.go:267 and internal/engine/regenerate_batch.go:477 take the snapshot. cmd/mint/regenerate_all.go:131-138 and cmd/mint/regenerate_run.go:101-106 pass it to RegenerateFreshRegenerator.
  - Tests that construct RegenRun:
    - cmd/mint/regenerate_remerge_test.go:73,83 (`engine.RegenRun{Merged: &notes.MergeInputs{}}`)
    - cmd/mint/regenerate_all_test.go:287,299,311, cmd/mint/regenerate_run_test.go:113,116 and internal/engine/regenerate_fresh_test.go:365,412,442 (`engine.RegenRun{}`)
- **Proposed shape**: One engine-side helper owns the rule, and the keeper becomes the only holder.
  - The helper could be `func (k *chunkNotesKeeper) regenerator(generator *notes.Generator, progress notes.Progress, cfg config.Config, fromSizeCheck func(ctx context.Context, g *notes.Generator, oneTimeContext string) (string, error)) Regenerator`. When `k.merged` is set it re-merges `*k.merged`; otherwise it runs `fromSizeCheck` over `generator` wired to `k.sinks(progress)` (see F2).
  - perRunRegenerator keeps its `deps.Regenerator` and nil-generator guards and calls the helper with `g.GenerateWithContext(ctx, lastTag, cfg, c)`. This drops it from six parameters.
  - RegenerateFreshRegenerator keeps its FirstRelease short-circuit and its `regenerating fresh notes for %s` wrap, and calls the helper with `g.GenerateFromRangeWithContext(ctx, res.DiffRange(), cfg, c)`.
  - RegenRun stops carrying a snapshot. It carries either the keeper (unexported) and the progress the `r` reports to, or a reader `Merged func() *notes.MergeInputs` bound to the keeper, built only by `chunkNotesKeeper.regenRun`. The reader keeps the cmd tests' construction close to how they build it today.
  - A zero RegenRun behaves as an empty keeper with no store, so the `RegenRun{}` tests are unaffected. The two cmd tests that seed held merge inputs change only how they build them; test semantics are unchanged.
  - The closure-local `merged`, the forwarding wrapper and the `run.MergeInputs != nil` guard go.
  - One re-merge generator remains, which removes the current difference: release re-merges through the unwired generator and regenerate through the wired one. That difference has no effect today because Remerge reports no progress and keeps nothing.
- **Bank**:
  - 6-8 reviewer, two entries: regenerate holds its `r` merge inputs in a closure-local `merged` while release holds them in chunkNotesKeeper. The failure these entries describe (the keeper never updated) was absorbed by 6-10's forward, but the two holders they describe remain.
  - 6-10 reviewer: release and regenerate `r` regenerators are two implementations of one rule; one helper would make the keeper the single holder.

### F2: Four generation sites chain the three notes sinks by hand
- **Class**: duplication
- **Failure**: Every notes generation that runs inside a notes stage must do three things: report to its progress, save a failed chunked run's finished notes, and pass on a written run's merge inputs. Each site chains the three `With…` calls by hand. Release passes the keeper's methods one at a time; regenerate passes a NotesSinks built by `chunkNotesKeeper.sinks`. A site that leaves one out still compiles and runs.
  - Without WithChunkNotes, a failed chunked run's finished part notes are lost. The operator is told no file was saved and has to re-run the wave.
  - Without WithMergeInputs, the next `r` re-runs the whole wave instead of one merge call, and release writes a chunked cache entry without its part notes, so a reusing run's `r` re-runs the wave too.
  - This happened twice in this phase. Release's `r` site had no WithChunkNotes until 6-9, and regenerate's `r` ignored the `run.ChunkNotes` it was handed until 6-10.
- **Evidence**:
  - internal/engine/release.go:1009: `selecting`, the body generation.
  - internal/engine/release.go:1161: `regenerating`, the `r`.
  - internal/engine/regenerate_fresh.go:65: RegenerateFreshBody.
  - internal/engine/regenerate_fresh.go:100-105: RegenerateFreshRegenerator.
  - internal/engine/chunknotes.go:44-46: `sinks`. regenerate_interactive.go:236, regenerate_batch.go:319 and chunknotes.go:84 use it; release does not.
  - internal/engine/regenerate_fresh.go:75-79: the NotesSinks type.
- **Proposed shape**: One engine method lists the sinks once, for example `func (s NotesSinks) wire(g *notes.Generator) *notes.Generator { return g.WithProgress(s.Progress).WithChunkNotes(s.ChunkNotes).WithMergeInputs(s.MergeInputs) }`. Release builds its sinks through `chunkNotes.sinks(stage)` and `chunkNotes.sinks(progress)` as regenerate already does, and every site reads `sinks.wire(generator)`. A new sink then reaches every generation site at once. Once F1 lands, the two `r` sites merge into F1's helper, which wires through this method; the body sites at release.go:1009 and regenerate_fresh.go:65 stay.
- **Bank**: 6-9 reviewer: three call sites repeat the WithProgress/WithChunkNotes/WithMergeInputs chain on a notes generator. There are four sites now, counting regenerate_fresh.go:100.

### F3: The review gate's `r` wiring is assembled at three sites
- **Class**: duplication
- **Failure**: Around an `r`, every review gate must do four things:
  - narrate an `r` that becomes chunked through the regenStage,
  - hand on the lower-cap hint once the notes are shown,
  - save the held part notes when the `r` fails,
  - after the abort, warn of a save that failed or name the saved file with the verb's own command.

  Release builds a `reviewRegen` inline and calls `reportKept` after the gate. Regenerate splits the same parts into three RegenerateWriteRequest fields: a Regenerator that is already narrated, RegenShown, and chunkNotes. RegenerateRun and gatePerVersion each set those fields, and reviewFresh puts them back together and reports. A site that leaves a part out still compiles, and reviewFresh reads a nil chunkNotes as "keep nothing". The result is a failed `r` that saves no part notes and names no file, or a halved `r` with no lower-cap hint, and no test fails. In this phase, 6-8 gave release `keep` and `reportKept`, and regenerate went without them until 6-10.
- **Evidence**:
  - internal/engine/release.go:593-600: the inline `reviewRegen{regenerator: regens.narrating(...), shown: regens.shown, keep: chunkNotes.keepRegen}`, then `reportKept(p, err, "mint release")`.
  - internal/engine/regenerate_interactive.go:260, 267-268, 271: the regens, Regenerator, RegenShown and chunkNotes fields.
  - internal/engine/regenerate_batch.go:338-339, 465 and 477-479: the same three fields.
  - internal/engine/regenerate_write.go:136-147: the Regenerator, RegenShown and chunkNotes fields.
  - internal/engine/regenerate_write.go:250-260: reviewFresh rebuilds the reviewRegen, with nil-tolerant keep and report.
  - internal/engine/regenerate_write.go:269-274: the `deps.Regenerator` override is applied after narration, so regenerate does not narrate it. Release does narrate it (release.go:594 with 1155-1157).
- **Proposed shape**: Two helpers, used by both release and reviewFresh.
  - One builds the gate's `r` from its parts, for example `func (r *regenStage) review(regenerator Regenerator, k *chunkNotesKeeper) reviewRegen` returning `{regenerator: r.narrating(regenerator), shown: r.shown, keep: k.keepRegen}`.
  - The other runs the gate and then the report: `reviewGate`, then `k.reportKept(p, err, command)`.
  - RegenerateWriteRequest carries the un-narrated regenerator plus the regenStage and keeper, in place of the narrated Regenerator, RegenShown and chunkNotes fields. RegenerateRun and gatePerVersion then pass only their stage and keeper.
  - The helper fixes one order for the `deps.Regenerator` override relative to narration. Only that test seam sees the change.
  - F1 and F3 share the keeper and can be one task.

## Comment Corrections

- internal/engine/release.go:585-592: "The gate stays positioned BEFORE any mutation" is false, since a pre_tag artifact commit can precede the gate, and the Unwind at line 600 is there to reset it. The rest of the block after the first sentence restates the code below it.
  OLD: 	// The review gate is the FINAL gate of a real release: accepting it proceeds straight
	// through Record → tag → push → publish (there is no further confirmation), which is
	// why ReleaseReviewGate's wording says "release", not "accept". It may first EDIT (the
	// `e` choice) or REGENERATE (`r`, normal-AI only) the body; capture the returned final
	// body and thread IT to every downstream sink. The gate stays positioned BEFORE any
	// mutation (before Record). gateForKind selects the variant (y/n/e vs y/n/e/r); the
	// per-run regenerator re-merges the held part notes of chunked notes, else binds
	// this run's lastTag + cfg to the generator so an `r` re-runs the AI path.
  NEW: 	// The review gate is the FINAL gate of a real release: accepting it proceeds straight
	// through Record → tag → push → publish (there is no further confirmation), which is
	// why ReleaseReviewGate's wording says "release", not "accept".

- internal/engine/release.go:1554-1557: "fails loud and clean before any mutation" is false at the release gate, where a pre_tag artifact commit can come first; the comment also carries a task id. The variable's name and message already say what it is.
  OLD: // errEditorUnavailable is the cause surfaced when the `e` choice is taken but no
// Editor seam was wired — a misconfiguration that should never reach production
// (task 2-13 wires the editor). It is surfaced rather than panicked so the spine
// fails loud and clean before any mutation.
  NEW:

- internal/engine/release.go:1560-1564: same false "clean before any mutation" claim and a task id. The variable's name and message already say what it is.
  OLD: // errRegeneratorUnavailable is the cause surfaced when the `r` choice is taken but
// no Regenerator seam was wired — a misconfiguration that should never reach
// production (the gate offers `r` only for KindNormalAI, which task 2-16 wires with
// a Regenerator). It is surfaced rather than panicked so the spine fails loud and
// clean before any mutation.
  NEW:

- internal/engine/release.go:1680-1681: "surfaced before any mutation" is false on the release path, where a pre_tag artifact commit can come first and the caller unwinds it.
  OLD: // no extra context), then hands that line to the Regenerator. Each failure mode is
// fail-loud and surfaced before any mutation:
  NEW: // no extra context), then hands that line to the Regenerator. Each failure mode is
// fail-loud:
