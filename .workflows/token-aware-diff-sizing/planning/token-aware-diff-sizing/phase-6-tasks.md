# Phase 6: Chunked notes at the review gate and in the dry-run cache — 10 tasks

## token-aware-diff-sizing-6-1

### Task 6-1: `r` on chunked notes re-runs only the merge

**Problem**: Chunked notes are AI notes, so the attended review gate offers `r`, regenerate with a one-time nudge, as it does for single-pass notes. The review gate is where an operator rejects a weak stitch (§15.1). Until this task, `r` on chunked notes re-runs the whole AI path: every chunk call again, then the merge. A nudge asks to change emphasis or ordering, and the merge is where those are decided; the per-chunk notes do not change with it. Re-running the wave spends minutes and many calls on a change one call can make.

**Solution**: On chunked notes at the release review gate, `r` makes one AI call, the merge. It reuses the per-chunk notes the run already produced, in chunk order, and adds the operator's nudge, which rides in the release's instructions. Like the first merge, it carries the release's instructions, the Change Map, the merge framing and the closing reminder. Its output is re-shown and becomes the body at the review gate, byte-identical. Each further `r` re-merges the same per-chunk notes with only its own nudge. Notes that a halved single-pass call turned into a chunked run are chunked notes, and re-merge the same way.

**Outcome**: Redirecting chunked notes at the review gate costs one AI call, as `r` costs on a single-pass release.

**Acceptance Criteria**:
- [ ] Given an attended release whose notes were written as 6 parts and a merge, when the operator chooses `r` at the review gate and enters the nudge `lead with the export feature`, the AI tool receives exactly one call. It carries the release's instructions with the nudge in them, the Change Map, the six parts' notes in chunk order, the merge framing and the closing reminder, and none of the diff. No chunk call and no single-pass call is made (§8.6, §15.1)
- [ ] Given that call returns a body, it is shown as the new notes and the review gate `Release with these notes?` asks again. Accepting it ships that body byte-identical as the tag message, changelog entry and provider release (§8.6, §15.1)
- [ ] Given a second `r` with the nudge `group the fixes`, its one call carries the same six parts' notes the wave produced and only `group the fixes`: neither the first nudge nor the first re-merge's output (§15.1)
- [ ] Given notes written by a single-pass call that was classified SIZE and re-cut into 3 parts, `r` makes exactly one call, a merge over those 3 parts' notes carrying the nudge (§10.1, §15.1)

**Do**:
- `r` reaches the AI through the per-run regenerator the release review gate consults: `perRunRegenerator`, which `regenerateBody` calls from `reviewGate` (`internal/engine/release.go`).
- The nudge rides in the release's instructions as `appendOneTimeContext` (`internal/notes/generate.go`) appends it as built (§15.1, Corrigendum 2026-10-19).
- The re-merge is Phase 2's merge call, through the release-notes transport with the release's `ai_command` and timeout (§2.2, §11.6).

**Context**:
> §15.1: on chunked notes, `r` re-runs only the merge call, reusing the per-chunk notes already produced and adding the operator's nudge. That is one call, as `r` costs on a single-pass release, and not a re-run of the whole wave. The residual this cannot fix, a change a chunk's own notes missed, is rare.
>
> §8.6: the merge call carries the release's instructions (the default prompt with `context`, or the `[release].prompt` override, which replaces both), the Change Map, every chunk's notes in chunk order, framing that asks for one set of release notes (combining related changes across parts, dropping duplicates and ranking by the Change Map), and the closing reminder. Its output passes through byte-identical. §8.7: chunk notes are held in memory for the run; they persist only in a failure's saved file and in the dry-run cache.
>
> As built:
> - `perRunRegenerator` binds `generator.GenerateWithContext`, which re-assembles the diff and re-runs the whole AI path with the nudge appended. A wired `deps.Regenerator` overrides it, the test seam.
> - `resolveBody` hands `Release` the body, its `notes.Kind` and the generator; `gateForKind` gives `notes.KindNormalAI` the four-choice gate with `r`.
> - `regenerateBody` reads the nudge through `AskLine(regenContextPrompt)`. An empty answer is legal and appends nothing. It re-shows the result with `ShowNotes`, and `reviewGate` asks again.
> - The doc comments on the `Regenerator` interface, `reviewGate`, `regenerateBody` and `perRunRegenerator` describe `r` as re-running the AI path the body took. §18.3 keeps comments true to a changed contract in the same change.
>
> In this phase, task 6-2 brings the merge-only `r` to regenerate, 6-3 covers `r` on single-pass notes, 6-5 keeps a merge-only `r`'s narration as built, 6-6 carries the per-chunk notes through the notes cache, and 6-8 handles a re-merge that fails.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §2.2 One command for every call; §8.6 What each call is told; §8.7 Calls that cannot be made; §10.1 When it applies; §11.6 Timeouts across a wave; §15.1 `r` on chunked notes; §18.3 CLAUDE.md and code comments; Corrigenda (2026-10-19, §15.1)

## token-aware-diff-sizing-6-2

### Task 6-2: Regenerate's `r` on chunked notes re-runs only the merge

**Problem**: Regenerate's fresh source has its own review gate, and its `r` works the same way as release's (§15.1). A single-version fresh regenerate, or a fresh version under `regenerate --all` at a terminal without `-y`, can have chunked notes. Until this task, `r` on them re-runs that version's whole wave.

**Solution**: On chunked notes at regenerate's review gate, `r` makes one AI call, the merge, over that version's own per-chunk notes with the operator's nudge riding in the release's instructions. Its output is re-shown and becomes the body at regenerate's review gate. Under `--all`, each version's `r` re-merges that version's own chunk notes. Each `r` re-merges the same chunk notes with only its own nudge, and notes a halved single-pass call turned into parts re-merge the same way.

**Outcome**: Redirecting a regenerated version's chunked notes costs one AI call, as it does on a forward release.

**Acceptance Criteria**:
- [ ] Given `mint release regenerate v1.4.0` from a fresh source whose notes were written as 5 parts and a merge, choosing `r` at regenerate's review gate with a nudge makes exactly one AI call. It carries the release's instructions with the nudge in them, v1.4.0's Change Map, its five parts' notes in chunk order, the merge framing and the closing reminder. No chunk call and no single-pass call is made (§8.6, §15.1)
- [ ] Given that call returns a body, it is shown as the new notes and regenerate's review gate `Use these notes?` asks again. Accepting it writes that body byte-identical to the chosen target (§15.1)
- [ ] Given a second `r` on v1.4.0 with another nudge, its one call carries the same five parts' notes and only the second nudge (§15.1)
- [ ] Given `regenerate --all --source fresh` at a terminal without `-y`, where v1.2.0 and v1.3.0 were both chunked, `r` at v1.3.0's review gate makes one merge call carrying v1.3.0's part notes and none of v1.2.0's (§15.1)
- [ ] Given a version whose single-pass call was classified SIZE and re-cut into parts, `r` at its review gate makes one merge call over those parts' notes (§10.1, §15.1)

**Do**:
- Regenerate's `r` reaches the AI through the per-run regenerator `gateRegenerate` hands its review gate (`internal/engine/regenerate_write.go`): `RegenerateFreshRegenerator` (`internal/engine/regenerate_fresh.go`), built for a single version by `newRegenerateRegeneratorProducer` (`cmd/mint/regenerate_run.go`) and per version under `--all` by `newBatchRegeneratorProducer` (`cmd/mint/regenerate_all.go`).
- The re-merge goes through the release `ai_command` and timeout, as every regenerate AI call does (`resolveFreshTransport` with `config.VerbRelease`) (§1.3, §2.2).

**Context**:
> §15.1: a single-version regenerate from a fresh source has its own review gate, and its `r` works the same way: on chunked notes it re-runs only the merge call. `regenerate --all` run at a terminal without `-y` shows each fresh version at the same review gate, and its `r` follows the same rules for that version.
>
> §7.2: regenerate neither offers notes from the dry-run cache nor writes to it, so a version's chunk notes come only from that version's own wave in this run.
>
> As built:
> - `RegenerateFreshBody` and `RegenerateFreshRegenerator` each build their own generator through `freshGenerator`. The body reaches the engine through `ProduceBody`, in `RegenerateRun` (`internal/engine/regenerate_interactive.go`) and in `processOneVersion` (`internal/engine/regenerate_batch.go`), and `ProduceBody` returns only the body.
> - Under `-y` the batch skips the per-version gate (`gatePerVersion`), and a non-TTY run without `-y` fails the gate with `ErrNotInteractive`, so a batch's `r` is reachable only at a terminal without `-y`.
> - Regenerate's review gate is `presenter.NotesReviewGate()`, asking `Use these notes?` with accept, abort, edit and regenerate.
> - The doc comments on `RegenerateFreshRegenerator`, `freshGenerator`, `regenerateRegenerator` and `RegenerateWriteRequest.Regenerator` describe an `r` that re-runs the identical fresh AI path. §18.3 keeps comments true to a changed contract in the same change.
>
> Task 6-1 built the merge-only `r` on the forward release. Task 6-10 handles a regenerate `r` that fails.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §1.3 The consumers; §2.2 One command for every call; §7.2 The AI path, in order; §8.6 What each call is told; §10.1 When it applies; §15.1 `r` on chunked notes; §18.3 CLAUDE.md and code comments

## token-aware-diff-sizing-6-3

### Task 6-3: `r` on single-pass notes re-runs from the size check

**Problem**: On single-pass notes, `r` re-runs the AI path with the nudge appended (§15.1). As built, the regeneration composes one prompt and sends it, whatever its size. A nudge can push a prompt that fit the cap over it, and no call mint makes is ever bigger than the cap (§4.1). An `r` the ladder turns into a chunked run also leaves chunked notes behind, which a further `r` should treat as chunked notes.

**Solution**: `r` on single-pass notes re-runs the AI path from the size check onward (§7.2), with the nudge counted in the composed prompt the size check measures:
- within the cap, one single-pass call with the nudge appended, as built
- over the cap, a chunked run that sends no over-cap call. The nudge rides in the release's instructions, so every chunk and piece call and the merge carry it.

Notes an `r` turned into a chunked run are chunked notes, so a further `r` re-runs only the merge (task 6-1). Starting at the size check, the `r` never offers cached notes. Regenerate's fresh `r`, single-version or under `--all`, re-runs from the size check the same way.

**Outcome**: A nudge that grows the prompt past the cap gets its notes written in parts instead of an over-cap call, and every part hears the nudge.

**Acceptance Criteria**:
- [ ] Given single-pass notes and an `r` whose nudged prompt stays within the cap, the AI tool receives exactly one single-pass call: the first attempt's prompt with the nudge appended to its instructions, as built (§7.2, §15.1)
- [ ] Given single-pass notes whose composed prompt was 1,499,800 bytes, and a nudge that brings the `r`'s composed prompt over 1,500,000 bytes, no single-pass call is sent. The AI tool receives chunk calls, none bigger than the cap, then one merge call, each carrying the nudge in the release's instructions. The merge's output is shown as the new notes and the review gate asks again (§4.1, §8.6, §15.1)
- [ ] Given such an `r` whose diff holds a file too big for one chunk call's room, that file's piece calls carry the nudge too (§8.4, §15.1)
- [ ] Given notes an earlier `r` turned into a chunked run, a further `r` makes exactly one call, a merge over the parts that `r` produced, carrying only the new nudge (§15.1)
- [ ] Given a real run that cached its single-pass notes, an `r` with an empty nudge makes its AI call and does not ask `Cached notes found for this diff — use them?` (§7.2, §15.1)
- [ ] Given `mint release regenerate <version>` from a fresh source, or a version under `regenerate --all` at a terminal without `-y`, with single-pass notes, `r` makes one single-pass call when its nudged prompt fits the cap, and becomes a chunked run carrying the nudge when it does not (§1.3, §15.1)

**Do**:
- The `r` re-enters Phase 2's size check and chunked generation on the release-notes generation path that `GenerateWithContext` (forward release) and `GenerateFromRangeWithContext` (regenerate's fresh source) reach, `generateFromDiffWithContext` (`internal/notes/generate.go`) (§7.2, §15.1).
- The nudge is appended to the instructions as `appendOneTimeContext` appends it, so it is part of every call's fixed part (§15.1, Corrigendum 2026-10-19).

**Context**:
> §15.1: on single-pass notes, `r` re-runs the same AI path as built, with the nudge appended, from the size check onward, so the ladder applies to it as to the first attempt. Its call is a release-notes generation call like any other, so the cap and the classifier apply by their own terms: a nudge that pushes the prompt over the cap sends the release to chunking rather than one over-cap call. It never re-runs the exclusion check, which asks about paths, not notes; Phase 7's acceptance pins that.
>
> Corrigendum 2026-10-19 on §15.1: the nudge rides in the release's instructions, so every chunk, piece and halving-round call and the merge carry it. §15.1 counts the nudge in the prompt the size check measures, which makes it part of every call's fixed part.
>
> §7.2, the AI path in order: the exclusion check (forward releases, Phase 7), cache reuse, the size check, then one single-pass call within the cap or chunked generation over it.
>
> As built, `r` never consults the notes cache: `cacheReuse` is wired only into `SelectBodyWithReuse` at the first generation (`internal/engine/release.go`, `internal/notes/select.go`). `resolveBody` writes the cache on real runs as well as dry runs.
>
> In this phase, task 6-4 classifies and halves a failing `r` call, task 6-5 narrates an `r` that becomes a chunked run, and task 6-9 saves its finished notes when it fails.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §1.3 The consumers; §4.1 The key; §7.2 The AI path, in order; §8.4 A file too big for one call: pieces; §8.6 What each call is told; §15.1 `r` on chunked notes; Corrigenda (2026-10-19, §15.1)

## token-aware-diff-sizing-6-4

### Task 6-4: A failed single-pass `r` call is classified and halved

**Problem**: A nudge can take an `r`'s prompt past what the model accepts while it stays within the cap. The `r` call is a release-notes generation call like any other, so the classifier applies to it by its own terms (§9.1, §15.1). Without that, an under-cap overflow at `r` aborts the run where halving would still have written the notes.

**Solution**: A single-pass `r` call that fails with a generation failure gets a classify call, as a first attempt's does, through the release command and timeout. On SIZE, the `r` becomes a chunked run at half the failed call's composed size, nudge included, and every halving-round call carries the nudge. A part of a chunked run made from an `r` that answers SIZE halves as a first attempt's part does, within the same bound of three halvings. A timeout or a missing binary gets no classify call, and every outcome other than SIZE fails the `r` (tasks 6-7, 6-9). Notes an `r` wrote after halving show the lower-cap hint as a first attempt's do. Regenerate's fresh `r`, single-version or under `--all`, is classified and halved the same way.

**Outcome**: An `r` whose nudge tips the prompt past the model's limit still gets its notes written, as the first attempt would have.

**Acceptance Criteria**:
- [ ] Given single-pass notes and an `r` whose call fails with captured output after the transport's retry, one classify call follows through the release command and timeout, carrying that `r` call's captured output (§9.1, §9.2, §15.1)
- [ ] Given that classify answer is `SIZE`, the AI tool receives chunk calls, each no bigger than half the failed `r` call's composed size, nudge included, then one merge call, and each carries the nudge. The merge's output is shown as the new notes and the review gate asks again (§10.1, §15.1)
- [ ] Given a part of that run, or of an `r` that went to chunking over the cap, whose call is classified SIZE, its slice is re-cut at half that call's composed size, each new call carrying the nudge, and the notes are still merged. A slice of the `r`'s run is halved at most three times, counted from its first SIZE answer, so a fourth SIZE fails the `r` (§10.1, §10.2, §15.1)
- [ ] Given the classify answer is `OTHER`, or an `r` call that times out, which gets no classify call, no chunk call follows and the `r` fails (§9.1, §9.3, §15.1)
- [ ] Given an `r` whose notes were written after halving, the lower-cap hint shows after the re-shown notes and before the review gate asks again. It names the largest single-pass, chunk or piece call that succeeded in the run, the first attempt's calls included, as `max_prompt_bytes = {bytes}` (§10.3, §15.1, §16.3)
- [ ] Given `mint release regenerate <version>` from a fresh source whose `r` call is classified SIZE, the `r` is halved the same way, and the hint shows before regenerate's review gate asks again. Under `regenerate --all` at a terminal without `-y`, a version's `r` halves the same way, shows no hint of its own, and its calls count toward the one hint the batch prints when it ends (§1.3, §10.3, §15.1)

**Do**:
- The `r` re-enters Phase 4's classifier and halving on the release-notes generation path in `internal/notes`, the ladder a first attempt runs; the classify call goes through the release `ai_command` and timeout (§9.2, §15.1).

**Context**:
> §15.1: the `r` call is a release-notes generation call like any other, so the classifier applies by its own terms, and the ladder applies to it as to the first attempt. Corrigendum 2026-10-19 on §15.1: the nudge rides in the release's instructions, so every halving-round call carries it.
>
> §10.1: the new target is half the composed size of the call that failed (instructions, Change Map, framing, diff and reminder together), so the nudge in the instructions counts. A single-pass call that fails SIZE becomes a chunked run at half its size. §10.2: a slice is halved at most three times, counted from its first SIZE answer.
>
> §10.3 and its corrigenda: only calls that carry diff count toward the hint (the single-pass, chunk and piece calls), and the value is that call's exact byte count. A single-version run prints the hint after its notes, ahead of the gate; under `regenerate --all` it prints once when the batch ends, naming the largest such call across the batch. The hint names the largest such call that succeeded in the run, so a first attempt's call that succeeded counts.
>
> §9.5: a Ctrl-C during the classify call or a halving round's call propagates as a cancel. Phase 4's tasks 4-2 to 4-8 and 4-14 built the classifier, halving, its bound and the hint for a first attempt. In this phase, task 6-5 narrates the halving, task 6-7 unwinds a failed `r`, and task 6-9 saves its finished notes.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §1.3 The consumers; §9.1 The trigger; §9.2 The call; §9.3 The codes; §9.5 Edges; §10.1 When it applies; §10.2 The bound; §10.3 Telling the operator; §15.1 `r` on chunked notes; §16.3 Follow-ups; Corrigenda (2026-10-19, §10.3 and §15.1)

## token-aware-diff-sizing-6-5

### Task 6-5: An `r` that becomes a chunked run narrates as one

**Problem**: As built, `r` narrates nothing while its call runs. An `r` that becomes a chunked run, over the cap at its size check or through halving (tasks 6-3, 6-4), can take minutes, and a wait that long with no sign of progress reads as hung (§16.1). In plain mode, CI and agents would see nothing until it ends (§16.2).

**Solution**: An `r` that becomes a chunked run narrates as any chunked run does: the halving line where its single-pass call halved, the split line counting parts done, a halving line for any part that halves, then merging, then `✓ Notes written ({n} parts, merged) ({elapsed})`. The split line's size is the `r`'s own composed single-pass prompt, nudge included, as its size check measured it. Plain mode prints a line per event, one per finished part included. An `r` that makes one call, the merge-only `r` on chunked notes or a single-pass `r` that stays one call, narrates as `r` does as built. Regenerate's `r`, single-version or under `--all`, narrates the same way.

**Outcome**: Whoever watches an `r` that grew into a chunked run sees it advance part by part, as they would a first attempt.

**Acceptance Criteria**:
- [ ] Given an `r` whose composed single-pass prompt, nudge included, is 1,520,000 bytes and goes to chunking as 2 parts under the default cap, in pretty mode the notes line reads `Writing notes: 1.5 MB release, in 2 parts · {k} of 2 done` as parts finish, then `Writing notes: merging 2 parts`, and ends `✓ Notes written (2 parts, merged) ({elapsed})` (§7.2, §15.1, §16.1)
- [ ] Given an `r` whose single-pass call is classified SIZE and re-cut into 3 parts, the notes line reads `Writing notes: too big for the model in one call, retrying in 3 parts`, then moves to the split line with the `r`'s composed single-pass size as its parts finish, then reads `Writing notes: merging 3 parts`, and ends `✓ Notes written (3 parts, merged) ({elapsed})` (§16.1)
- [ ] Given a chunked `r` whose part 2 of 4 is classified SIZE and re-cut into 2 parts, the notes line reads `Writing notes: part 2 was too big for the model, retrying it as 2 smaller parts`, and the split line then counts out of 5 (§16.1)
- [ ] Given those runs in plain mode, each update prints as its own line as it happens, one per finished part included (§16.2)
- [ ] Given the merge-only `r` on chunked notes, or a single-pass `r` that stays one call, no `Writing notes:` line and no `Notes written` line shows: the `r` narrates as built (§16.1)
- [ ] Given `mint release regenerate <version>` from a fresh source, or a version under `regenerate --all` at a terminal without `-y`, whose `r` becomes a chunked run, it narrates the same way, and an `r` there that makes one call narrates as built (§1.3, §16.1)

**Do**:
- The narration is the chunked run's presenter events from Phase 2 and Phase 4 (tasks 2-8 and 4-5, `internal/presenter`); the engine never touches spinners or TTY state (§16.4).

**Context**:
> Corrigendum 2026-10-19 on §16.1: §16.1's ground is that a chunked run can take minutes and a spinner that sits that long reads as hung, and its wording names a chunked run with no first-attempt limit; §15.1 has the ladder apply to an `r` as to the first attempt. A one-call `r` waits as long as a single-pass `r` does as built, so it keeps the as-built narration.
>
> §16.1: sizes are written in decimal units, 1 MB being 1,000,000 bytes, one decimal place below 10 and a whole number from 10 up, so 1,520,000 bytes reads `1.5 MB`. The ✓ line keeps the elapsed time every blocking stage's success line carries. Parts are numbered by their place in chunk order, a halved part's new parts taking its place, and the total grows by the parts a re-cut adds.
>
> As built:
> - `regenerateBody` (`internal/engine/release.go`) reads the nudge, calls the regenerator and re-shows the notes, with no stage narration of its own.
> - A first attempt's notes narrate as a blocking stage opened by `emitBlockingStageStarted`, in `Release`, in `RegenerateRun` (`internal/engine/regenerate_interactive.go`) and in `processOneVersion` (`internal/engine/regenerate_batch.go`). The `Presenter.StageStarted` stream-discipline note (`internal/presenter/presenter.go`) keeps other narration out from between a blocking start and its completion, unless wrapped in `SuspendSpinner`/`ResumeSpinner`.
> - Plain's synthesised narration is byte-pure printable ASCII (`internal/presenter/bytepurity_test.go`).

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §1.3 The consumers; §7.2 The AI path, in order; §15.1 `r` on chunked notes; §16.1 Pretty mode: one notes line, updated; §16.2 Plain mode: a line per event; §16.4 How it is built; Corrigenda (2026-10-19, §16.1)

## token-aware-diff-sizing-6-6

### Task 6-6: Chunked notes come back from the notes cache with their chunk notes

**Problem**: The notes cache spares a real run the AI work an earlier run for the same inputs already did, and a chunked run is where it spares the most: a whole wave (§15.2). As built, a cache entry holds only the body. Chunked notes reused from it come back with no per-chunk notes, so `r` on them cannot re-run only the merge; it would re-run the very wave the cache exists to spare.

**Solution**: Cache a chunked body together with its per-chunk notes, in the same entry and under the same key as single-pass notes: the post-exclusion diff, the version and the resolved instructions. Notes a halved single-pass call turned into a chunked run are cached the same way. The entry is written on real runs as well as dry runs, as built, and holds the per-chunk notes whichever run wrote it. The following real run offers the cached notes through the cached-notes prompt as built; using them makes no AI call, and `r` on them re-runs only the merge. The per-chunk notes share the entry's lifecycle: outside the repo, pruned with it, stale with it after the ~1h TTL, with no cleanup of their own. A single-pass entry carries no chunk notes. Regenerate neither reads nor writes the cache.

**Outcome**: A real run that follows a chunked dry run, or a chunked run aborted at the review gate, gets those notes back without a wave and can redirect them for one merge call.

**Acceptance Criteria**:
- [ ] Given `mint release --dry-run` whose notes were written as 4 parts and a merge, the following real run for the same diff, version and instructions within the TTL asks `Cached notes found for this diff — use them?`. Choosing use shows the dry run's body byte-identical, and no chunk, merge or single-pass call is made (§7.2, §15.2)
- [ ] Given those reused notes at the review gate, `r` with a nudge makes exactly one AI call: a merge carrying the dry run's four parts' notes in chunk order and the nudge (§15.1, §15.2)
- [ ] Given a real run whose notes were written as parts and a merge and which was then aborted at the review gate, the next real run for the same inputs offers its notes, and `r` on them makes only the merge call, over that run's part notes (§15.2)
- [ ] Given a cached chunked entry older than the TTL, the real run does not offer it and writes the notes afresh (§15.2)
- [ ] Given a run that writes a cached chunked entry, the repo's work tree and index are unchanged by the write. Given a later run with a changed diff that writes its own entry, nothing of the earlier entry, its chunk notes included, remains in the repo's cache directory (§15.2)
- [ ] Given cached notes from a single-pass run, reusing them and choosing `r` re-runs the AI path from the size check, as on live single-pass notes (§15.1, §15.2)
- [ ] Given `mint release regenerate <version>` from a fresh source, single-version or under `--all`, whose notes are chunked, no cached notes are offered and no cache entry is written (§7.2)

**Do**:
- The cache key stays `notescache.Key` over the post-exclusion diff, the version and the resolved instructions (§15.2).
- The per-chunk notes are held by the cache entry itself (`notescache.Entry`, `internal/notescache/cache.go`), written where `resolveBody` writes the body and returned where `cacheReuse` reuses it (`internal/engine/release.go`) (§15.2).

**Context**:
> §15.2: the notes cache keys on inputs, not output, so a dry run's chunked notes are cached and offered back to the real run exactly like single-pass notes, sparing it the whole wave. A chunked body is cached together with its per-chunk notes, so `r` on reused chunked notes re-runs only the merge, one call, as on a live run.
>
> Corrigendum 2026-10-19 on §15.2: an entry holds the per-chunk notes whichever run wrote it, dry or real, so `r` on reused chunked notes re-runs only the merge whoever cached them. Outside the cache entry the per-chunk notes live only in memory, and only a failure's saved file persists.
>
> As built:
> - `Entry` holds `Body` and `WrittenAt`. `Lookup` treats an entry older than `TTL` (one hour) as absent. `Store.Prune` removes only the `.json` entries in the repo's cache directory other than the kept key. The cache lives under `{user cache dir}/mint/cache`, one directory per repo (`newNoteCache`, `cmd/mint/main.go`).
> - `resolveBody` writes every freshly generated normal-AI body, on real runs and dry runs alike, skips the write for a reused body, and prunes the repo's other entries. A dry run reuses a live entry silently; a real run asks through `reuseConfirmGate`, `use cached` or `regenerate`, and `-y` auto-accepts use.
> - `SelectBodyWithReuse` consults the reuse hook before any AI call (`internal/notes/select.go`). The engine's `NoteCacheWriter.Write` takes only a body.
> - The package doc and the `Entry` doc in `internal/notescache/cache.go`, and the `NoteCache` seam docs in `internal/engine/release.go`, describe a body-only entry. §18.3 keeps comments true to a changed contract in the same change.
>
> §11.4: the saved chunk-notes file sits beside the cache but outside its pruning (Phase 4); it is not the cache entry. Task 6-8 saves a reused entry's chunk notes when its re-merge fails.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §7.2 The AI path, in order; §11.4 The saved chunk-notes file; §15.1 `r` on chunked notes; §15.2 The dry-run cache; §18.3 CLAUDE.md and code comments; Corrigenda (2026-10-19, §15.2)

## token-aware-diff-sizing-6-7

### Task 6-7: Every abort at the review gate unwinds

**Problem**: The review gate sits after the pre-tag artifact commit, so however it ends a release before the push, the run should unwind as any failure before the push does (§15.1). As built, `Release` routes only a decline through the surgical unwind. A failed `r`, stdin closing at the `r`'s nudge prompt or at the gate's keypress, and a launched editor that fails at the gate's `e` all return with the artifact commit still on the branch. That breaks the invariant that every pre-push mutation is unwound on failure.

**Solution**: Every way the review gate ends a release before the push unwinds as any failure before the push does: back to the starting commit, taking the pre-tag artifact commit with it. That covers:
- a decline, as built
- every failed `r`: the merge-only `r`, a single-pass `r` that stays one call, one the ladder turned into a chunked run, and stdin closing at the `r`'s nudge prompt
- a launched editor that fails at the gate's `e`
- stdin closing at the gate's keypress
- a Ctrl-C while an `r` runs.

A missing editor still warns and returns to the gate. A failed `r` aborts whatever `on_notes_failure` holds: under `fallback` the run still unwinds before any tag, and the commit-subject list never ships. A failed `r` never reaches the floor.

**Outcome**: However a release ends at the review gate, the branch is back at the commit it started from, with no tag.

**Acceptance Criteria**:
- [ ] Given a release whose pre_tag hook changed files, so the run made the pre-tag artifact commit, and an `r` whose single-pass call fails with output and is classified OTHER, the ✗ line reads `AI call failed` with that call's captured output verbatim beneath it. The run then resets to the starting commit, the artifact commit gone, narrates the unwind, makes no tag and exits 1 (§6.6, §12.4, §15.1)
- [ ] Given the same release, an `r` on chunked notes whose re-merge fails, or an `r` the ladder turned into a chunked run that fails, unwinds the same way and exits 1 (§15.1)
- [ ] Given the same release and stdin closing at the `r`'s nudge prompt, no AI call is made, and the run unwinds the same way and exits 1 (§15.1)
- [ ] Given the same release and a launched editor that exits non-zero at the review gate's `e`, the failure is surfaced, and the run unwinds the same way and exits 1. Given stdin closing at the review gate's keypress instead, the run unwinds the same way and exits 1 (§15.1)
- [ ] Given the review gate's `e` with no editor that can be launched, mint warns that it could not launch the editor and the review gate asks again with the body unchanged; nothing is reset (§15.1)
- [ ] Given a Ctrl-C while an `r`'s AI call runs, the run aborts as a cancel and unwinds the same way, making no tag (§9.1, §15.1)
- [ ] Given `on_notes_failure = "fallback"` and a failed `r`, the run unwinds the same way and exits 1 before any tag: no commit-subject list ships, no fallback warning prints, and no `c` / `e` / `a` choice shows (§12.1, §15.1)

**Do**:
- Every way `reviewGate` ends a release before the push goes through the surgical unwind, `Unwind` (`internal/engine/unwind.go`), from `Release`'s handling of the review gate's result (`internal/engine/release.go`), which as built unwinds only a decline (§15.1, Corrigenda 2026-10-19).

**Context**:
> §15.1 and its corrigenda: every way the review gate ends a release before the push unwinds as any failure before the push does, taking the pre-tag artifact commit with it. An `r` that ends because stdin closed at its nudge prompt is a failed `r`, as built. That is settled by CLAUDE.md's "Surgical unwind, then PONR" invariant (every pre-push mutation is tracked and unwound on failure), by §6.6 ("A failure before the push unwinds as built, back to the starting commit and taking the pre-tag artifact commit with it"), and by the §12.2 corrigendum, which unwinds every abort at the floor on the same ground. Regenerate's review gate lands before its changelog commit, so an abort there has nothing to reset.
>
> §15.1: `on_notes_failure` does not apply to a failed `r`, so `fallback` never ships its body in place of what the operator asked for. §12.1: a failed `r` is not the floor; it aborts.
>
> As built:
> - `reviewGate` returns `errGateAborted` on a decline; a launched-but-failed editor through `surface(p, "edit", …)`; closed input at the keypress as an abort from `ReviewDecision`; and `regenerateBody`'s failures: closed input at its `AskLine` read, a nil regenerator, and a failed regeneration surfaced through `surface(p, "regenerate", …)`, which renders `failureMessage`'s cause phrase and `notesFailureOutput`'s captured output.
> - A missing editor makes `EditorLauncher.Edit` (`internal/engine/editor.go`) warn `could not launch editor "…": not found` and return `ErrEditorReturnToGate`, and `reviewGate` asks again.
> - `Release` routes only `errGateAborted` through `Unwind` and returns every other gate error as-is.
> - `Unwind` resets to the starting commit when the run made commits, deletes a tag mint made, narrates `Unwound`, is a no-op when nothing was made, and detaches from cancellation. A cancelled context observed before the push already routes through it (`internal/engine/release_cancellation_test.go`).
> - A dry run returns before the review gate.
> - The comment at `Release`'s review-gate handling, that every other review-gate error is returned as-is, and `reviewGate`'s doc on how its aborts are handled go stale with this change. §18.3 keeps comments true to a changed contract in the same change.
>
> Phase 8 later re-commits the exclusion check's answers on top of this unwind. Tasks 6-8 and 6-9 save chunk notes before the abort; task 6-10 covers regenerate's failed `r`.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §6.6 Recording the answers; §9.1 The trigger; §12.1 What applies, by mode; §12.4 The ✗ line and the next moves; §15.1 `r` on chunked notes; §18.3 CLAUDE.md and code comments; Corrigenda (2026-10-19, §12.2 and §15.1)

## token-aware-diff-sizing-6-8

### Task 6-8: A failed re-merge aborts with every chunk's notes saved

**Problem**: The re-merge a merge-only `r` makes (task 6-1) can fail like any merge: a usage limit the wave drew down, an expired login, a timeout, a SIZE answer, or a nudge that pushes its prompt over the cap (§11.3). A failed `r` aborts the run whatever `on_notes_failure` says (§15.1). Without a saved file, the abort throws away every chunk's notes, and the operator's only way back to AI notes is to re-run the whole wave.

**Solution**: Any failure of the re-merge fails the `r`:
- a nudged merge prompt over the cap sends no call and fails as `part notes too large to merge`
- a SIZE answer on the re-merge is not halved, and fails the same way
- any other failure takes its own cause phrase, located at the merge.

Before the run aborts, mint saves every chunk's notes to the chunk-notes file, with no missing-notes marker, and names it with `mint release --notes-file <path>`. Stdin closing at the nudge prompt on chunked notes fails the `r` before any call, and saves the file the same way. The run then aborts before any tag whatever `on_notes_failure` holds, unwinding as task 6-7 does, and never reaches the floor, so `fallback`'s commit-subject list never ships in place of the re-merge. The saved file outlives the unwind. Chunked notes reused from the notes cache save their cached chunk notes the same way. A save that fails warns with the write error verbatim, and the failure names no path.

**Outcome**: A failed re-merge leaves the operator every chunk's notes in a named file, ready to tidy and release with `--notes-file`, instead of losing the wave.

**Acceptance Criteria**:
- [ ] Given chunked notes from a 6-part run of tag `v1.4.0` in the repo at `/Users/lee/Code/app`, and an `r` whose re-merge call fails with output and is classified OTHER, the ✗ line reads `AI call failed (merging)` with that call's captured output verbatim beneath it. mint writes `{user cache dir}/mint/chunk-notes/-Users-lee-Code-app/v1.4.0.md`, holding the six parts' notes exactly as the chunk calls returned them, in chunk order, separated by a blank line, with no missing-notes marker. The failure names its full path with `mint release --notes-file <path>` (§11.3, §11.4, §12.4, §15.1)
- [ ] Given a re-merge call that times out, the ✗ line reads `AI timed out (merging)`, and the file is saved and named the same way (§11.3, §12.4)
- [ ] Given a nudge that brings the re-merge prompt over the cap, no merge call is sent and the ✗ line reads `part notes too large to merge`. Given a re-merge call classified SIZE, no further call follows and the ✗ line reads the same. Either way the file is saved and named (§8.7, §10.1, §11.3, §12.4)
- [ ] Given chunked notes and stdin closing at the `r`'s nudge prompt, no AI call is made, and the file is saved and named the same way (§15.1)
- [ ] Given any of these failures, the run then unwinds to the starting commit, taking a pre-tag artifact commit with it, makes no tag and exits 1, and the saved file still sits at the path the failure named, holding the same notes. Under `on_notes_failure = "fallback"` it does the same: no commit-subject list ships and no `c` / `e` / `a` choice shows (§11.4, §12.1, §15.1)
- [ ] Given chunked notes reused from the notes cache, whether a dry run or an earlier real run cached them, and an `r` whose re-merge fails, the file holds the cached chunk notes and is named the same way (§15.1, §15.2)
- [ ] Given the chunk-notes file cannot be written, the ✗ line, its cause phrase and its captured output are unchanged. One warning says the finished chunk notes could not be saved, with the write error verbatim beneath it, and the failure names no path (§11.4)
- [ ] Given a Ctrl-C while the re-merge call runs, the run aborts as a cancel, and no chunk-notes file is written (§11.5, §15.1)

**Do**:
- The file is Phase 4's chunk-notes file, at its path, in its format and with its write-failure warning (tasks 4-10 and 4-12) (§11.4).
- The abort goes through the unwind task 6-7 gives every failed `r` (§15.1).

**Context**:
> §15.1, when `r` fails on chunked notes: mint first saves the per-chunk notes and names the file with the command that releases it, `mint release --notes-file <path>`. `on_notes_failure` does not apply, so `fallback` never ships its body in place of the re-merge the operator asked for. The abort lands before any tag, so the saved notes can still go out through `--notes-file`.
>
> §11.3: a merge counts as failed on any failure of the merge call, a SIZE answer from the classifier, or a merge prompt over the cap. There is no concatenation rung. §10.1: a SIZE answer on the merge is not halved, since halving a merge would be the recursive-collapse reduce that is ruled out. §8.7: a merge prompt over the cap is not sent.
>
> §12.4 and its corrigendum: `part notes too large to merge` takes no location suffix, since it already names the merge; the other phrases end with where the failure happened, `(merging)` for the merge.
>
> §11.4: the marker opens the file only when a slice has no notes; on a re-merge every slice has them. The file lives outside the repo, so the unwind's reset leaves it in place, and a later failure of the same tag overwrites it.
>
> Corrigenda 2026-10-19 on §15.1:
> - an `r` that ends because stdin closed at its nudge prompt is a failed `r`, as built. As built, `regenerateBody` returns on closed input at its `AskLine` read, before calling the regenerator.
> - a Ctrl-C during an `r`, the re-merge call included, is a cancel, not a failed `r`, and saves no chunk notes, as a Ctrl-C mid-wave saves none (§11.5). The run's notes-cache entry still holds the chunk notes (§15.2).
>
> Task 6-6 carries the per-chunk notes in the cache entry. Task 6-10 saves a regenerated version's file.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §8.7 Calls that cannot be made; §10.1 When it applies; §11.3 A failed merge; §11.4 The saved chunk-notes file; §11.5 Ctrl-C mid-wave; §12.1 What applies, by mode; §12.4 The ✗ line and the next moves; §15.1 `r` on chunked notes; §15.2 The dry-run cache; Corrigenda (2026-10-19, §11.4, §12.4 and §15.1)

## token-aware-diff-sizing-6-9

### Task 6-9: A failed `r` the ladder made chunked aborts with its finished notes saved

**Problem**: A single-pass `r` that the ladder turns into a chunked run, over the cap at its size check or through halving (tasks 6-3, 6-4), can fail partway through its wave. A failed `r` aborts whatever `on_notes_failure` says (§15.1). Without a saved file, the parts that finished are lost with the abort.

**Solution**: When an `r` the ladder made chunked fails, mint first saves the finished chunk notes of that `r`'s wave, as a first attempt's failed wave saves them, and names the file with `mint release --notes-file <path>`. Slices with no notes get the missing-notes marker. A wave in which no chunk finished saves and names no file, and a single-pass `r` that fails without chunking saves none. The run then aborts before any tag whatever `on_notes_failure` holds, unwinding as task 6-7 does, and never reaches the floor. A Ctrl-C during the `r` cancels the run and saves nothing.

**Outcome**: An `r` that grew into a chunked run and then failed leaves its finished parts in a named file, as a first attempt's failed wave does.

**Acceptance Criteria**:
- [ ] Given single-pass notes and an `r` whose nudged prompt is over the cap and goes as 4 parts, where part 3's call fails for good with output, classified OTHER, and parts 1, 2 and 4 finish, the ✗ line reads `AI call failed (part 3 of 4)` with that call's captured output verbatim beneath it. mint saves the chunk-notes file holding parts 1, 2 and 4's notes in chunk order, opening with the missing-notes marker that lists part 3's files, and the failure names its full path with `mint release --notes-file <path>` (§11.2, §11.4, §12.4, §15.1)
- [ ] Given an `r` whose single-pass call is classified SIZE and re-cut into 3 parts, where part 2 fails for good and parts 1 and 3 finish, the file is saved and named the same way, with parts 1 and 3's notes and a marker listing part 2's files (§10.1, §11.4, §15.1)
- [ ] Given such an `r` whose parts all finish and whose merge fails, the file holds every part's notes and no marker (§11.3, §11.4)
- [ ] Given such an `r` whose wave fails before any part finishes, no file is saved and the failure names none (§11.4)
- [ ] Given a single-pass `r` that fails without chunking, classified OTHER, timed out, or with its program not installed, no file is saved and none is named (§15.1)
- [ ] Given any of these failures, the run aborts before any tag and exits 1 whatever `on_notes_failure` holds, and no `c` / `e` / `a` choice shows (§12.1, §15.1)
- [ ] Given a Ctrl-C while the `r`'s chunk calls run, with some parts finished, the run aborts as a cancel: no file is saved, no further AI call is made, and under `on_notes_failure = "fallback"` no commit-subject list ships (§9.1, §11.5)
- [ ] Given the chunk-notes file cannot be written, the ✗ line, its cause phrase and its captured output are unchanged. One warning says the finished chunk notes could not be saved, with the write error verbatim beneath it, and the failure names no path (§11.4)

**Do**:
- The file, its marker and its write-failure warning are Phase 4's (tasks 4-10, 4-11 and 4-12) (§11.4).
- The abort goes through the unwind task 6-7 gives every failed `r` (§15.1).

**Context**:
> §15.1, when a single-pass `r` the ladder turned into a chunked run fails: mint first saves the per-chunk notes and names the file with `mint release --notes-file <path>`. `on_notes_failure` does not apply, and the abort lands before any tag, so the saved notes can still go out through `--notes-file`.
>
> §11.2: a chunk failure that is final ends the wave: an OTHER answer, a failed classify call, a failure with no output to classify, a timeout, a missing binary, or a SIZE answer with no halving left. Calls already running finish, and the notes they produce are saved with the rest.
>
> §11.4: the file holds each finished chunk's notes in chunk order, separated by a blank line, exactly as the calls returned them. When any slice has no notes, it opens with the missing-notes marker, and a file whose pieces only partly failed is marked `(partly)`. A wave in which no chunk finished saves no file, because a file holding only the marker would pass `--notes-file`'s empty check and ship.
>
> §11.5: a Ctrl-C mid-wave cancels every call in flight and aborts cleanly; no chunk notes are saved and `on_notes_failure` does not apply.
>
> The ladder makes an `r` chunked only from single-pass notes, so the chunk notes it saves are its own wave's. Phase 4's tasks 4-9 to 4-12 built the first attempt's saved file.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §9.1 The trigger; §10.1 When it applies; §11.2 The first final failure ends the wave; §11.3 A failed merge; §11.4 The saved chunk-notes file; §11.5 Ctrl-C mid-wave; §12.1 What applies, by mode; §12.4 The ✗ line and the next moves; §15.1 `r` on chunked notes; Corrigenda (2026-10-19, §11.4 and §15.1)

## token-aware-diff-sizing-6-10

### Task 6-10: A failed regenerate `r` saves the version's chunk notes before it aborts

**Problem**: A failed `r` aborts a regenerate (§15.1). A regenerate `r` on chunked notes, or one the ladder turned into a chunked run, holds chunk notes the abort would throw away. The version is already out, so saved notes go onto it only through `mint release regenerate <version> --notes-file <path>` (§13). Under `regenerate --all`, a failed `r` ends the batch as a failed gate does as built; it is not the batch's floor, so the version is not skipped and the batch does not carry on.

**Solution**: When a fresh regenerate's `r` fails where the notes were chunked or the ladder made the `r` chunked, mint first saves the chunk notes to that version's file, keyed by its own tag, and names it with that version's `mint release regenerate <version> --notes-file <path>`, then aborts the regenerate. The file holds what a release's failed `r` saves: every chunk's notes after a failed re-merge, or the finished parts with the missing-notes marker. Stdin closing at the nudge prompt on chunked notes is a failed `r` too. Under `--all` at a terminal without `-y`, the failed `r` saves that version's file, names it with that version's command and ends the batch, as a failed gate does as built. A single-pass `r` that fails without chunking saves no file. A save that fails warns with the write error verbatim and names no path.

**Outcome**: A regenerate `r` that fails on chunked notes leaves that version's notes in a named file, with the command that puts them on the version.

**Acceptance Criteria**:
- [ ] Given `mint release regenerate v1.4.0` from a fresh source with chunked notes, and an `r` whose re-merge fails, mint writes `{user cache dir}/mint/chunk-notes/{repo path, separators as dashes}/v1.4.0.md`, holding every part's notes with no missing-notes marker. The failure names its full path with v1.4.0's `mint release regenerate <version> --notes-file <path>`. The regenerate aborts and exits 1, writing nothing to the changelog or the provider release (§11.3, §11.4, §15.1)
- [ ] Given an `r` on v1.4.0's single-pass notes that the ladder made chunked, whose wave fails for good after some parts finished, the file holds those parts' notes and the missing-notes marker, and is named the same way (§11.4, §15.1)
- [ ] Given stdin closing at the `r`'s nudge prompt on v1.4.0's chunked notes, no AI call is made, and the file is saved and named the same way before the regenerate aborts (§15.1)
- [ ] Given a single-pass `r` on v1.4.0 that fails without chunking, the regenerate aborts and no file is saved or named (§15.1)
- [ ] Given `regenerate --all --source fresh` at a terminal without `-y` over v1.2.0, v1.3.0 and v1.4.0, where v1.2.0 was regenerated and an `r` on v1.3.0's chunked notes fails, mint saves v1.3.0's file and names it with v1.3.0's `mint release regenerate <version> --notes-file <path>`. The batch then ends, as a failed gate does, and exits 1: v1.4.0 is not processed and v1.3.0 is not reported as skipped (§12.3, §15.1)
- [ ] Given the version's chunk-notes file cannot be written, the ✗ line, its cause phrase and its captured output are unchanged. One warning says the finished chunk notes could not be saved, with the write error verbatim beneath it, and the failure names no path (§11.4)

**Do**:
- The failed `r` reaches regenerate's review gate through `gateRegenerate` (`internal/engine/regenerate_write.go`), which the single-version and `--all` paths share (§15.1).
- The file, its marker and its write-failure warning are Phase 4's, keyed by the version's own tag (tasks 4-10 to 4-13) (§11.4).

**Context**:
> §15.1 on regenerate: a failed `r` aborts the regenerate. If the notes were chunked, or the ladder turned the `r` into a chunked run, mint first saves the per-chunk notes and names the file; the version is already out, so the saved notes go onto it with `mint release regenerate <version> --notes-file <path>`. Under `regenerate --all` at a terminal without `-y`, a failed `r` saves that version's per-chunk notes, names the file with that version's command, and ends the batch, as a failed gate does as built. It is not the batch's floor, so the batch does not skip the version and carry on.
>
> Corrigenda 2026-10-19 on §15.1: a regenerate's `r` lands before its changelog commit, so it has nothing to reset. An `r` that ends because stdin closed at its nudge prompt is a failed `r`, as built.
>
> §13.1: one file answers one version, so each version's file goes on with its own command.
>
> As built:
> - A gate error at a version propagates out of `processOneVersion` and ends the batch with no summary (`internal/engine/regenerate_batch.go`). Skip-and-continue covers only a `ProduceBody` failure and a body-less deterministic source.
> - Regenerate's review gate runs through `gateRegenerate` on the single-version and `--all` paths alike, before any changelog commit or provider write.
>
> Task 4-14's batch-end lower-cap hint prints however the batch ends, a failed `r` included. Tasks 6-8 and 6-9 save the same file on a release's failed `r`.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §11.3 A failed merge; §11.4 The saved chunk-notes file; §12.3 Regenerate at the floor; §13.1 What it is; §15.1 `r` on chunked notes; Corrigenda (2026-10-19, §11.4 and §15.1)
