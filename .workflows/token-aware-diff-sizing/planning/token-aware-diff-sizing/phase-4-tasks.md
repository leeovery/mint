# Phase 4: Failure classification, halving and saved chunk notes — 14 tasks

## token-aware-diff-sizing-4-1

### Task 4-1: Name the cause on a notes failure's ✗ line

**Problem**: Every release-notes generation failure renders as `AI returned empty/invalid notes after retry`. That misnames a failure the AI explained in its own output, such as an expired login or a size overflow (§1.1, §12.4). In a chunked run the ✗ line also does not say which call failed: a chunk, a piece or the merge.

**Solution**: Give each notes failure its §12.4 cause phrase:
- a generation failure with text on either captured stream reads `AI call failed`
- a generation failure whose captured stdout and stderr are both empty or whitespace-only reads `AI returned empty notes`
- a timeout keeps `AI timed out`, and a missing binary keeps `AI tool not installed`.

In a chunked run the phrase ends with the call the failure belongs to: `(part k of n)` for a chunk or piece call, `(merging)` for the merge call. Three cases take no suffix:
- a single-pass failure
- `part notes too large to merge`, which already names the merge
- `no room for the diff under max_prompt_bytes`, which no single call owns.

The AI's captured output still renders verbatim beneath the line. The forward release and `mint release regenerate <version>` share the phrases.

**Outcome**: A notes failure's ✗ line says what went wrong and, in a chunked run, which call it went wrong in. No release-notes failure reads the old phrase.

**Acceptance Criteria**:
- [ ] Given a single-pass call that exits non-zero on both attempts with `Not logged in · Please run /login` on stdout, the ✗ line reads `AI call failed` with that output verbatim beneath it (§9.6, §12.4)
- [ ] Given a single-pass call that fails on both attempts with stdout empty and a message on stderr, the ✗ line reads `AI call failed` with the stderr message verbatim beneath it (§12.4)
- [ ] Given a single-pass call that returns an empty or whitespace-only body on both attempts, with nothing but whitespace on stderr, the ✗ line reads `AI returned empty notes` with no output beneath it (§12.4)
- [ ] Given a single-pass call that times out, the ✗ line reads `AI timed out`. Given an `ai_command` whose program is not installed, it reads `AI tool not installed`. Neither carries a location (§12.4)
- [ ] Given a chunked run of 6 parts whose part 3 call fails on both attempts with output, the ✗ line reads `AI call failed (part 3 of 6)` with that call's captured output verbatim beneath it. A part 3 call that times out reads `AI timed out (part 3 of 6)`, and one that returns empty notes with nothing on stderr reads `AI returned empty notes (part 3 of 6)`. A failed piece call reads the same way with its own part number (§12.4)
- [ ] Given chunk calls that succeed and a merge call that fails on both attempts with output, the ✗ line reads `AI call failed (merging)` with the merge call's captured output verbatim beneath it. A merge call that times out reads `AI timed out (merging)` (§12.4)
- [ ] Given a chunked run that fails as `no room for the diff under max_prompt_bytes`, or as `part notes too large to merge` because the merge prompt is over the cap, the ✗ line reads that phrase with no location (§12.4)
- [ ] Given `mint release regenerate <version>` from a fresh source whose notes fail in any of these ways, its ✗ line reads the same phrase and location, with the same output beneath (§1.3, §12.4)
- [ ] No release-notes failure, forward or regenerate, reads `AI returned empty/invalid notes after retry` (§12.4)

**Do**:
- The phrases replace the as-built ones in `notes.CauseText` (`internal/notes/resolve.go`), which `failureMessage` (`internal/engine/release.go`) renders as the ✗ line's message on the forward release and regenerate alike.

**Context**:
> §12.4, the cause phrases:
>
> | Cause | Phrase |
> |---|---|
> | Timeout | `AI timed out` (as built) |
> | Missing binary | `AI tool not installed` (as built) |
> | SIZE with no halving left, or a halving round with no room | `too large for the model, even in smaller parts` |
> | OTHER, with output | `AI call failed` |
> | OTHER, no output | `AI returned empty notes` |
> | No room for diff caused by operator text | `no room for the diff under max_prompt_bytes` |
> | Merge too large: its prompt over the cap, or a SIZE answer on the merge call | `part notes too large to merge` |
>
> §12.4 and its corrigendum: the suffix names the call the failure belongs to. `part notes too large to merge` already names the merge, and a failure no single call owns has no place to name: no room for any part under operator text, or a single-pass call whose first halving round has no room (task 4-7).
>
> §9.6: the classifier adds to the AI's verbatim output and never replaces it. The AI's own text is often the most specific remediation available, for example `Not logged in · Please run /login`, or claude's token breakdown on an overflow.
>
> This task comes before the classifier, so every generation failure here takes what becomes the OTHER path, and its phrase follows from whether its captured output has text. Task 4-2 adds the classify call, and tasks 4-3, 4-4 and 4-7 add the SIZE reactions and `too large for the model, even in smaller parts`.
>
> As built:
> - `notes.CauseText` maps `ai.ErrGenerationFailed` to the old phrase, and also names the cause inside `abortError`'s message.
> - The transport returns `*ai.GenerationError` for a generation failure, carrying the retry attempt's `Stdout` and `Stderr` as separate fields. `notesFailureOutput` (`internal/engine/release.go`) composes them for `StageFailure.Output`, deciding emptiness on each stream's trimmed view.
> - A timeout or a missing binary carries no captured output.
> - The single-version regenerate notes failure reaches the ✗ line through `surface`, which uses the same `failureMessage` and `notesFailureOutput`.
> - `regenerate --all`'s skip reason does not use `CauseText` (`classifyNotesFailure`, `internal/engine/regenerate_batch.go`). Phase 5 makes each skip name its cause phrase.
>
> Phase 2 added the two §8.7 phrases (task 2-9) and made a chunked run's ✗ line carry the first failure's captured output (task 2-10). The `n` in `(part k of n)` is the part count Phase 2's narration shows (task 2-8), in which each piece counts as one part. Task 4-5 keeps the numbering right once halving adds parts.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §1.1 The problem; §1.3 The consumers; §9.6 Alongside the verbatim error; §12.4 The ✗ line and the next moves; Corrigenda (2026-10-19, §12.4)

## token-aware-diff-sizing-4-2

### Task 4-2: Classify a failed notes call

**Problem**: A failed notes call's output can mean the prompt overflowed the model, which smaller calls would fix. It can equally mean an expired login, a usage limit or a one-off server error, where splitting would multiply one failure into many (§9.1). mint keeps no per-provider knowledge and never string-matches provider error messages (§1.4), so it cannot tell the two apart on its own.

**Solution**: When a single-pass, chunk or merge call ends in a generation failure, make one classify call. A generation failure is a non-zero exit, or an empty body, that survived the transport's single retry. The classify call goes through the existing transport with the release's `ai_command` and timeout. Its prompt carries:
- the two codes, each with a one-line gloss: SIZE for a request too large for the model's context window or input limit, and OTHER for anything else
- the failed call's captured stdout and stderr, both streams, fenced as captured tool output and explicitly not instructions
- an instruction to answer with exactly one code.

A stream longer than 4,000 bytes is sent as its first and last 2,000 bytes, cut at character boundaries, with a line saying how many bytes were omitted between them. The transport's single retry applies to the classify call as to every call. The classifier adds to the AI's verbatim output and never replaces it. `mint commit` makes no classify call.

**Outcome**: Every release-notes generation failure gets one small classify call for the next tasks to branch on, and the failure the operator sees is still the failed call's own output.

**Acceptance Criteria**:
- [ ] Given a single-pass call that exits non-zero on both attempts with output, the AI tool receives the single-pass prompt twice, then exactly one classify call (§9.1, §9.5)
- [ ] Given a chunk call, or the merge call, that fails the same way, exactly one classify call follows, carrying that call's captured output (§9.1)
- [ ] Given a classify call, its prompt carries SIZE and OTHER, each with its one-line gloss, the failed call's captured stdout and stderr fenced as captured tool output and marked as not instructions, and an instruction to answer with exactly one code (§9.2)
- [ ] Given `[release].ai_command = "tool-a"`, `[release].timeout = 30` and `[commit].ai_command = "tool-b"`, the classify call runs `tool-a` with a 30-second per-attempt timeout, on a forward release and on `mint release regenerate <version>` from a fresh source alike (§1.3, §9.2)
- [ ] Given a failed call with 10,000 bytes on one stream, the classify prompt carries that stream's first 2,000 and last 2,000 bytes, with a line between them saying 6,000 bytes were omitted. Given a stream of exactly 4,000 bytes, it is carried whole with no omission line. Given both streams over 4,000 bytes, each is cut on its own (§9.2)
- [ ] Given a stream over 4,000 bytes whose 2,000-byte cut would fall inside a multi-byte character, the cut falls at a character boundary instead, so no character is split, and the omission line counts the bytes actually left out (§9.2)
- [ ] Given a classify call whose first attempt returns an empty body, the transport retries it once, as it does every call (§9.5)
- [ ] Given a classify call that answers OTHER, the ✗ line reads `AI call failed`, and beneath it is the failed notes call's captured output verbatim, never the classify call's prompt or answer (§9.6)
- [ ] Given a `mint commit` whose AI call fails, no classify call is made: the AI tool receives only the commit prompt, and the editor fallback runs as built (§9.1, §17.2)

**Do**:
- The classify call goes through the release-notes `Transport` the notes calls already use, built by `aitransport.New` for the release verb. There is no new AI seam (§9.2).

**Context**:
> §9.1: the classifier was chosen for two reasons. It is provider-agnostic by construction, because the AI interprets its own error. And it never fires a wave of calls at a failure chunking cannot fix. Accepted cost: one extra AI call per failure.
>
> §9.2: each stream's 4,000-byte budget keeps the call small, a few kilobytes, and holds both ends of the output, where causes sit. The stakes of untrusted text are bounded: the answer only picks a branch, an injected answer can at most misclassify, and under the lean command the call has no tools to act through. §9.2 fixes what the prompt carries; its wording is the implementer's.
>
> §9.5: the transport's single retry stays for every call, so an under-cap overflow sends the identical call twice before the classifier looks. On claude the repeat costs one more round trip: the API refuses an oversized prompt with a 400 in a second or two at no token cost.
>
> §17.2: every commit failure already ends in the editor, so a classify call would only delay the editor by one AI call. The exclusion check (Phase 7) never classifies its own failures either (§6.8).
>
> Later tasks in this phase complete the classifier:
> - task 4-3 reads the answer and reacts to SIZE on a single-pass call; until it lands, every answer takes the OTHER path
> - task 4-4 reacts to SIZE on a chunk, piece or merge call
> - task 4-8 routes the failures that get no classify call (no error text, a timeout, a missing binary, a classify prompt over the cap, a cancel) and a classify call that fails
> - task 4-9 stops classifying once a wave has failed for good.
>
> CLAUDE.md seam 5: the transport owns validation and the single bad-content retry, and consumers never re-retry. `context.Canceled` propagates unchanged. As built, `*ai.GenerationError` carries the retry attempt's `Stdout` and `Stderr` as separate fields, and the notes tests inject a recording fake `Transport` (`internal/notes/generate.go`).

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §1.3 The consumers; §1.4 Out of scope; §9.1 The trigger; §9.2 The call; §9.5 Edges; §9.6 Alongside the verbatim error; §17.2 What commit does not get

## token-aware-diff-sizing-4-3

### Task 4-3: A SIZE answer re-runs a single-pass call in halved parts

**Problem**: A single-pass prompt under the cap can still overflow the model. That happens when an explicit `ai_command` points at a smaller-window model, when an explicit command carries overhead of its own, or when the cap is set above the real window (§10.1). The failed call was already under the cap, so the cap gives the chunker nothing smaller to aim at, and the release ends without notes that smaller calls could have written.

**Solution**: Read the classify answer. Strip surrounding whitespace, quotes, backticks and trailing punctuation, then compare what remains case-insensitively against the codes. It is a listed code only when exactly `SIZE` or exactly `OTHER` remains; anything else is OTHER, and there is no firmer re-ask.

On SIZE for the single-pass call, halve. The new target is half the failed call's composed size: instructions, Change Map, diff and closing reminder together. The diff is re-cut at that target by the first wave's packing and piece rules, so the single-pass call becomes a chunked run at half its size, ending in one merge whose output ships byte-identical. On OTHER, nothing splits and the notes fail.

**Outcome**: A release whose single call overflowed the model still gets AI-written notes, from calls sized off the one that failed. Any other failure stops without a wave.

**Acceptance Criteria**:
- [ ] Given a single-pass call of S bytes that fails with output, and a classify answer that reduces to `SIZE`, such as `SIZE`, ` size `, `"SIZE"`, `` `Size` `` or `SIZE.`, no second single-pass call is made. The notes are written by chunk calls each no bigger than half of S, then one merge call, and the merge's output ships byte-identical as the notes body (§8.6, §9.4, §10.1)
- [ ] Given those chunk calls, each carries the release's instructions, the same whole-release Change Map, part-k-of-n framing, its slice and the closing reminder. Files are packed in path order, and together the calls carry every file's diff exactly once (§8.2, §8.3, §8.6, §10.1)
- [ ] Given a file whose diff alone exceeds a halved call's room, it is sent as pieces cut by the piece rules, each carrying the file's diff header and none bigger than half of S (§8.4, §10.1)
- [ ] Given a classify answer of `OTHER`, a sentence such as `This looks like a SIZE problem`, both codes such as `SIZE OTHER`, or an unknown word such as `LIMIT`, no chunk call is made and no second classify call is sent. The notes fail with the ✗ line `AI call failed` and the original call's captured output verbatim beneath it (§9.3, §9.4)
- [ ] Given `mint release regenerate <version>` from a fresh source, or a version under `regenerate --all`, whose single-pass call fails and is classified SIZE, that version's notes are written the same way, over its own range (§1.3)

**Do**:
- The re-cut uses the packing and piece rules Phase 2 built (tasks 2-4 and 2-7), aimed at the halved target where the first wave aims at the cap (§10.1).

**Context**:
> §9.3: only about three reactions exist however many codes are defined: split, retry or back off, and surface and stop. SIZE starts the ladder, and OTHER falls to the floor with no split. Anything unrecognised already falls to OTHER.
>
> §9.4: the transport owns validation and the single retry, and consumers never re-retry. Worst case: the error surfaces where chunking might have helped.
>
> §10.1: each call in a halved round has its own room for diff, which is whatever its fixed part leaves: the instructions, Change Map, framing and closing reminder. A round aimed below the fixed part has no room and sends no call. Task 4-7 handles that case, together with the bound of three halvings.
>
> §8.2: every chunk call and the merge call receive the same whole-release Change Map, so a halved call's fixed part carries the map the first wave's calls carry.
>
> A halved run is a chunked run, so the four-at-once bound (task 2-6) applies to its calls. In this phase, task 4-4 halves a failed chunk or piece call and refuses to halve the merge, task 4-5 narrates the re-cut, and task 4-6 suggests a lower cap once the notes are written after halving. Phase 6 sends a single-pass `r` at the review gate through this same ladder.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §1.3 The consumers; §8.2 The Change Map in every call; §8.3 Packing chunks; §8.4 A file too big for one call: pieces; §8.6 What each call is told; §9.3 The codes; §9.4 Reading the answer; §10.1 When it applies

## token-aware-diff-sizing-4-4

### Task 4-4: A SIZE answer on a chunk halves its slice

**Problem**: A chunk or piece call under the cap can overflow the model just as a single-pass call can (§10.1). Without a reaction, one overflowing part fails the whole generation, although its slice re-cut smaller would fit. The merge is different: halving it would be the recursive-collapse reduce that is ruled out (§8.1, §10.1).

**Solution**: When a chunk or piece call fails and its classify answer is SIZE, halve its slice:
- the new target is half that call's composed size: instructions, Change Map, framing, diff and closing reminder together
- the slice the call carried is re-cut at the target by the packing and piece rules, and each new call has whatever room its fixed part leaves
- the halved parts are chunk calls under the four-at-once bound, and their notes take that slice's place in chunk order at the merge
- a halved part that is classified SIZE in turn is halved in a further round, sent after that answer.

A SIZE answer with halving left does not end the wave. A SIZE answer on the merge call is never halved: it fails the merge as `part notes too large to merge`.

**Outcome**: A chunked run whose individual parts overflow the model still ends in one set of merged notes, and a merge too large for the model fails cleanly instead of recursing.

**Acceptance Criteria**:
- [ ] Given a chunked run whose part 3 call, of C bytes, fails with output and is classified SIZE, part 3's slice is re-sent as chunk calls each no bigger than half of C, each carrying the release's instructions, the same whole-release Change Map, its framing, its slice and the closing reminder. Between them they carry every file's diff that part 3 carried, exactly once (§8.2, §10.1)
- [ ] Given part 3 classified SIZE while other parts are in flight or not yet started, the wave carries on: the other parts are sent and finish as before (§11.2)
- [ ] Given a wave that includes halved parts, never more than four chunk calls are in flight at once, the halved parts included (§8.5, §10.1)
- [ ] Given a piece call classified SIZE, its piece is re-cut into smaller pieces by the piece rules, each carrying the file's diff header (§8.4, §10.1)
- [ ] Given one of part 3's halved parts also classified SIZE, it is halved in turn, to half its own composed size, in a further round sent after that answer (§10.1, §10.2)
- [ ] Given the halved parts and every other part succeed, exactly one merge call follows. It carries every part's notes in chunk order, with part 3's halved parts in part 3's place, and its output ships byte-identical (§8.3, §8.6, §10.1)
- [ ] Given every part succeeds and the merge call fails with output and is classified SIZE, no further call is made and the merge is not halved. The ✗ line reads `part notes too large to merge`, with no location, and the merge call's captured output verbatim beneath it (§9.6, §10.1, §11.3, §12.4)
- [ ] Given `mint release regenerate <version>` from a fresh source, or a version under `regenerate --all`, whose chunk call is classified SIZE, that version's slice halves the same way (§1.3)

**Do**:
- The re-cut uses the packing and piece rules Phase 2 built (tasks 2-4 and 2-7), aimed at the halved target (§10.1).

**Context**:
> §10.1: a SIZE answer for a diff-carrying call, the single-pass call or a chunk call, halves. A SIZE answer on the merge call is not halved: halving a merge would be the recursive-collapse reduce that is ruled out, so it is a failed merge instead.
>
> §10.2: halving's rounds are serial, a deliberate exception to the parallel-first shape. The exception is accepted because the path is rare when the cap fits the model, and its cost lands where the misconfiguration is. A slice is halved at most three times from its first SIZE answer, and a round aimed below its fixed part sends no call; task 4-7 builds both limits.
>
> §11.2: a SIZE answer with halving left does not end the wave; that chunk halves. Every other chunk failure is final and ends it (task 4-9).
>
> §11.3: a failed merge saves every chunk's notes (task 4-10) and goes to the floor. Concatenating the chunk notes is never a fallback (§8.1).
>
> Task 4-3 built the answer reading and the single-pass re-cut that this task extends to parts. Task 4-5 narrates a part's re-cut, which grows the part total and renumbers the parts after it.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §1.3 The consumers; §8.1 The shape: map-reduce; §8.2 The Change Map in every call; §8.3 Packing chunks; §8.4 A file too big for one call: pieces; §8.5 Concurrency; §8.6 What each call is told; §9.6 Alongside the verbatim error; §10.1 When it applies; §10.2 The bound; §11.2 The first final failure ends the wave; §11.3 A failed merge; §12.4 The ✗ line and the next moves

## token-aware-diff-sizing-4-5

### Task 4-5: Halving narration

**Problem**: Halving re-cuts a failed call and sends more calls, adding parts and time the operator cannot see (§16.1). A notes line still counting toward the old total would read as stuck or wrong once a part is re-cut, and a part number on the halving line or the ✗ line would point at the wrong part. Plain output for CI and agents would go quiet on the re-cut (§16.2).

**Solution**: Narrate each re-cut on the notes line:
- a single-pass call that halves shows `Writing notes: too big for the model in one call, retrying in {n} parts`
- a part that halves shows `Writing notes: part {k} was too big for the model, retrying it as {n} smaller parts`.

In both, n is the number of parts the re-cut yields. The part total grows by the parts each re-cut adds, and the split line's count grows with it, since one total serves both places. Parts are numbered by their place in chunk order, a halved part's new parts taking its place, so every later part's number moves up by the parts added. `part {k}` on the halving line and `(part k of n)` on the ✗ line always count within the current total. Plain mode prints each update as its own line. The updates are new presenter events, and the engine never touches spinners or TTY state.

**Outcome**: Whoever watches a run sees when a call was too big for the model and how the run re-cut it, and every count and part number stays right to the end.

**Acceptance Criteria**:
- [ ] Given a single-pass call classified SIZE whose re-cut yields 3 parts, in pretty mode the notes line reads `Writing notes: too big for the model in one call, retrying in 3 parts`. As the parts finish it moves to the split line with the composed single-pass size, such as `Writing notes: 1.2 MB release, in 3 parts · 1 of 3 done`, then reads `Writing notes: merging 3 parts`, and the run ends `✓ Notes written (3 parts, merged) ({elapsed})`. In plain mode each finished part prints its own line (§16.1, §16.2)
- [ ] Given a chunked run of 6 parts whose part 4 is classified SIZE and re-cut into 3 parts, the notes line reads `Writing notes: part 4 was too big for the model, retrying it as 3 smaller parts`. Its split line then counts out of 8, such as `Writing notes: 3.1 MB release, in 8 parts · 5 of 8 done`, it then reads `Writing notes: merging 8 parts`, and the run ends `✓ Notes written (8 parts, merged) ({elapsed})` (§16.1)
- [ ] Given that run, part 4's new parts are numbered 4, 5 and 6, and the parts first packed as 5 and 6 are numbered 7 and 8 from then on. A later re-cut of the part first packed as 5 into 2 parts reads `Writing notes: part 7 was too big for the model, retrying it as 2 smaller parts`, and the run then merges 9 parts (§16.1)
- [ ] Given that run with no further halving, where the part first packed as 6 fails with output and is classified OTHER, the ✗ line reads `AI call failed (part 8 of 8)` (§12.4, §16.1)
- [ ] Given the same runs in plain mode, each halving update prints as its own line as it happens, alongside the per-part lines (§16.2)
- [ ] Given a run with no halving, its narration is as Phase 2 built it (§16.1)
- [ ] Given `mint release regenerate <version>` from a fresh source, or a version under `regenerate --all`, whose notes halve, that version's notes line narrates the same way (§1.3, §16.1)

**Do**:
- The updates are new presenter events in the existing model (`internal/presenter`): the pretty presenter renders them on the running notes line, the plain presenter as lines of their own, and `presentertest.RecordingPresenter` records them. The engine never touches spinners or TTY state (§16.4).

**Context**:
> §16.1's example of a chunked run that halves:
>
> ```text
> ⠋ Writing notes: 3.1 MB release, in 6 parts · 2 of 6 done
> ⠋ Writing notes: part 4 was too big for the model, retrying it as 3 smaller parts
> ⠋ Writing notes: merging 8 parts
> ✓ Notes written (8 parts, merged) (64.2s)
> ```
>
> §16.1 and its corrigendum: the split line's template `in {n} parts · {k} of {n} done` uses one total in both places, and a halved slice's parts sit in that slice's position in chunk order (§8.6, §10.1). Numbering by place keeps every `part {k}` and `(part k of n)` within the total the line shows. Each piece counts as one part (task 2-8). A single-pass run that halves is then a chunked run, so as its parts finish the line moves to the split line with the same composed single-pass size (Corrigendum 2026-10-19 on §16.1). There is no per-part detail beyond the count; on a failure, the ✗ line names the part (task 4-1).
>
> Phase 2's task 2-8 built the split, progress, merge and completion lines and their presenter events. As built, the `Presenter.StageStarted` doc's stream-discipline note forbids other narration between a blocking start and its completion, so the updates have to render without breaking the running line. Plain's synthesised narration is byte-pure printable ASCII (`internal/presenter/bytepurity_test.go`), and its exact wording is the implementer's.
>
> Tasks 4-3 and 4-4 produce the re-cuts this task narrates.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §1.3 The consumers; §10.1 When it applies; §12.4 The ✗ line and the next moves; §16.1 Pretty mode: one notes line, updated; §16.2 Plain mode: a line per event; §16.4 How it is built; Corrigenda (2026-10-19, §16.1)

## token-aware-diff-sizing-4-6

### Task 4-6: Suggest a lower cap after halving

**Problem**: Halving absorbs a cap set higher than the model accepts. The notes get written, but every later release pays again for the failed call, its retry, the classify call and the re-cut, and the operator never learns that the cap is wrong (§10.3).

**Solution**: When a run's notes were written after halving, print a follow-up after the notes. It says the cap is set higher than this model accepts and suggests the size of the largest diff-carrying call that succeeded in the run, written as the value to set: `max_prompt_bytes = {bytes}`. The single-pass, chunk and piece calls count; the merge and classify calls never do. The value is the call's exact byte count, never rounded and never in the narration's decimal units. A single-version run prints the hint after its notes, ahead of the review gate. A run whose halving ran out gets no hint, since its notes were not written.

**Outcome**: An operator whose cap is too high for the model learns a value that worked, ready to paste into `.mint.toml`.

**Acceptance Criteria**:
- [ ] Given an attended release whose single-pass call is classified SIZE and whose halved parts all succeed, the largest of them 612,345 bytes, mint shows the hint after the notes and before the review gate. It says the cap is set higher than this model accepts and shows `max_prompt_bytes = 612345` (§10.3, §16.3)
- [ ] Given a chunked run whose parts of 1,400,000 and 1,260,000 bytes succeed, and whose third part is classified SIZE and re-cut into parts that succeed, the hint shows `max_prompt_bytes = 1400000`: the largest diff-carrying call that succeeded, halved or not (§10.3)
- [ ] Given a run whose merge call is bigger than every chunk and piece call, the hint names the largest chunk or piece call. Neither the merge call nor a classify call ever sets the value (§10.3)
- [ ] The value is the call's exact byte count with no separators, rounding or units: a call of 1,260,001 bytes is written `max_prompt_bytes = 1260001`, never `1.3 MB` or `1260000` (§10.3)
- [ ] Given a run whose notes were written with no halving, no hint shows (§10.3)
- [ ] Given a run that halved but whose notes then failed, as when halving runs out or another part fails for good, no hint shows (§10.3)
- [ ] Given `mint release regenerate <version>` from a fresh source whose notes were written after halving, the hint shows after its notes and before regenerate's review gate (§1.3, §10.3)

**Context**:
> §10.3 and its corrigenda: the hint turns a quiet absorption into something the operator can fix. A size rounded up would sit above the largest call the model accepted, so the value is exact. Only the calls that carry diff count, because those are what overflowed and what the cap sizes; a merge's size carries notes prose of no measured density, so naming it could suggest a value at which chunk calls overflow again. A single-version run prints the hint after its notes, ahead of the gate where a decline would land.
>
> §4.5: the cap's default leans high because the two ways of getting it wrong differ. Too high for the model is loud and corrects itself: mint halves, still writes the notes, then hints that the cap should come down. Too low is silent.
>
> §16.3: the follow-ups print after the notes: this hint, and the exclusion suggestions under `-y` (Phase 7). The exclusion check's call (Phase 7) carries no diff, so it never counts toward the hint either. The narration's decimal sizes (task 2-8) are for narration, not for a value to paste into config.
>
> Under `regenerate --all` the hint prints once, at the end of the batch, never after a single version's notes (task 4-14).
>
> As built:
> - The forward release shows its notes with `p.ShowNotes` after the plan and before `reviewGate` (`internal/engine/release.go`).
> - Regenerate shows its notes in `gateRegenerate` (`internal/engine/regenerate_write.go`), which the single-version and `--all` paths share.
> - Regenerate's body producer reaches the engine as `ProduceBody`, which returns only the body.
>
> CLAUDE.md seam 3: every byte of user output goes through the presenter.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §1.3 The consumers; §4.5 The default's grounds; §10.1 When it applies; §10.3 Telling the operator; §16.1 Pretty mode: one notes line, updated; §16.3 Follow-ups; Corrigenda (2026-10-19, §10.3)

## token-aware-diff-sizing-4-7

### Task 4-7: Halving runs out

**Problem**: Halving cannot go on forever. A cap wrong for its model by more than eight times is not a slight miscalibration (§10.2). A command whose own overhead overflows the window gets SIZE back however small the calls become (§9.7, §10.3). And a halving round can aim below its calls' fixed part, leaving no room for diff at all (§8.7, §10.1).

**Solution**: Count halvings per slice from its first SIZE answer, carrying the count into the parts a halved slice becomes, a single-pass call's included. A slice is halved at most three times, which takes it to an eighth of the size that first failed. A fourth SIZE fails the slice, and a failed slice fails the generation with no merge. A halving round whose target is below its calls' fixed part has no room: it sends no call, and the slice fails.

Both render `too large for the model, even in smaller parts`, with the location of the call whose SIZE answer ended the slice. A single-pass call whose first halving round has no room belongs to no part, so its phrase takes no location. Both route as any notes failure does.

**Outcome**: A run that halving cannot save stops after at most three re-cuts of any slice, with a ✗ line that names a size failure.

**Acceptance Criteria**:
- [ ] Given a single-pass call of S bytes classified SIZE, then one of its halved parts classified SIZE, then one of that part's halved parts classified SIZE, the third round's calls are each no bigger than an eighth of S. Given one of those is classified SIZE too, no further re-cut and no merge call is made, and the notes fail (§10.2, §11.1)
- [ ] Given a chunked run of 6 parts whose part 3 is classified SIZE, where each re-cut yields 2 parts and the first of them is classified SIZE again, the slice is halved three times and the fourth SIZE fails it. No merge call is made, and the ✗ line reads `too large for the model, even in smaller parts (part 3 of 9)` with that call's captured output verbatim beneath it (§10.2, §11.1, §12.4, §16.1)
- [ ] Given a chunk call of 200,000 bytes whose fixed part is 120,000 bytes, classified SIZE, its halved target of 100,000 bytes leaves no room for diff. No call is sent for that round, and the ✗ line reads `too large for the model, even in smaller parts` with that part's location and its call's captured output verbatim beneath it (§8.7, §10.1, §12.4)
- [ ] Given a single-pass call of 200,000 bytes whose instructions, Change Map and closing reminder take 120,000 bytes, classified SIZE, its first halving round has no room. No chunk call is sent, and the ✗ line reads `too large for the model, even in smaller parts` with no location and the single-pass call's captured output verbatim beneath it (§8.7, §10.1, §12.4)
- [ ] Given halving runs out on an unattended release under `on_notes_failure = abort`, the run stops before the tag. Under `fallback`, the commit-subject list ships (§12.1)
- [ ] Given halving runs out on an unattended `mint release regenerate <version>` from a fresh source, it stops with the same ✗ line and writes nothing. Under `regenerate --all`, that version is skipped and the batch carries on (§12.3)

**Context**:
> §10.2: a slice is halved at most three times, counted from its first SIZE answer, which takes it to an eighth of the size that first failed. A fourth SIZE fails that slice, and a failed slice fails the generation.
>
> §10.3: when halving runs out, mint cannot tell a small window from a command with a large overhead of its own. In both, even small pieces come back SIZE while the small classify call fits. Phase 5's floor guidance names both causes: lower `max_prompt_bytes`, or use the documented lean command. No lower-cap hint shows (task 4-6).
>
> §8.7: a halving round aimed below the fixed part sends no call, and the run falls to the floor with guidance naming which of the two no-room causes it was. Phase 5 tells this case apart from outsized operator text, which renders `no room for the diff under max_prompt_bytes` (task 2-9).
>
> §12.4's corrigendum: the suffix names the call the failure belongs to. A single-pass call's first halving round with no room has no single call to point at, so it takes none; a later round with no room belongs to the part whose SIZE answer it was.
>
> §11.1, §11.2: when any chunk ends in failure, no merge runs. A slice that runs out of halving is a final failure that ends the wave (task 4-9), and the finished chunks' notes are saved (task 4-10).
>
> Tasks 4-3 and 4-4 built the halving this task bounds; task 4-5 numbers parts by their place in chunk order, which the location counts within.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §8.7 Calls that cannot be made; §9.7 An explicit `ai_command` that carries its own overhead; §10.1 When it applies; §10.2 The bound; §10.3 Telling the operator; §11.1 One failed chunk fails the generation; §11.2 The first final failure ends the wave; §12.1 What applies, by mode; §12.3 Regenerate at the floor; §12.4 The ✗ line and the next moves; §16.1 Pretty mode: one notes line, updated; Corrigenda (2026-10-19, §12.4)

## token-aware-diff-sizing-4-8

### Task 4-8: Failures that skip classification

**Problem**: Some failures leave nothing to classify, or no way to classify it (§9.5): a failed call with no error text, a classify prompt that cannot fit under the cap, and a classify call that itself fails. A timeout or a missing binary is already named by the transport and is no evidence of size (§9.1). A guessed SIZE on any of them would fire a wave at a failure chunking cannot fix, and a classify call's own output would hide the original cause. A Ctrl-C is not a failure at all.

**Solution**: Route these without a classify answer:
- both captured streams empty or whitespace-only: no classify call, and the OTHER path; text on either stream is enough to classify
- a cap smaller than the classify prompt: the call is not sent, and the OTHER path
- a classify call that fails, times out or finds no binary: the OTHER path
- in both of the last two cases, the output rendered is still the original call's
- a notes call that times out or finds no binary: no classify call, and its own phrase
- a Ctrl-C during the classify call or a halving round's call: the cancel propagates and never reaches `on_notes_failure`.

**Outcome**: Only a failure with text to read and a classify call that answered can start halving. Every other failure stops with the original call's own output, and a Ctrl-C always aborts.

**Acceptance Criteria**:
- [ ] Given a single-pass call that returns an empty or whitespace-only body with nothing but whitespace on stderr on both attempts, no classify call is made and no chunk call follows. The ✗ line reads `AI returned empty notes` (§9.5, §12.4)
- [ ] Given a failed call with text only on stdout, or only on stderr, a classify call is made (§9.5)
- [ ] Given a `max_prompt_bytes` smaller than the classify prompt a failed notes call would produce, no classify call is sent and no chunk call follows. The ✗ line reads `AI call failed` with the original call's captured output verbatim beneath it (§9.2, §12.4)
- [ ] Given a classify call that fails on both attempts, times out, or whose program is not found, no chunk call follows. The ✗ line reads `AI call failed` with the original notes call's captured output verbatim beneath it, never the classify call's (§9.5)
- [ ] Given a single-pass, chunk or merge call that times out, or whose program is not installed, no classify call is made, and the ✗ line reads `AI timed out` or `AI tool not installed` (§9.1, §12.4)
- [ ] Given a Ctrl-C while the classify call runs, or while a halving round's call runs, the run aborts as a cancel: no further AI call is made, and under `on_notes_failure = fallback` no fallback body ships (§9.1, §9.5, §11.5)

**Context**:
> §9.5: classifying nothing could only be a guess, and a guessed SIZE would fire the whole wave at a failure chunking cannot fix. When the tool's overhead alone overflows the window, the classify call fails the same way, and that is the correct stop: splitting cannot fix it.
>
> §9.1: a timeout is not evidence of size, and a wave fired at a slow service multiplies the wait. The size cap absorbs "too slow because too big" before any call is sent (§4.4).
>
> Corrigendum 2026-10-19 on §9.2: no call mint makes is ever bigger than the cap (§4.1), and a call that cannot be made under the cap is not sent (§8.7). The 4,000-byte stream budget is fixed, so the classify prompt is not trimmed further to fit.
>
> §11.2: in a chunked run each of these failures is final and ends the wave (task 4-9).
>
> CLAUDE.md seam 5: `context.Canceled` propagates unchanged, and a cancel is never routed to a fallback. As built, the transport reports a timeout (`ai.ErrTimeout`) and a missing binary (`ai.ErrCommandMissing`) at once without retrying, and propagates a cancel unchanged. Phase 2's task 2-10 made a Ctrl-C mid-wave cancel every call in flight.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §4.1 The key; §8.7 Calls that cannot be made; §9.1 The trigger; §9.2 The call; §9.5 Edges; §11.5 Ctrl-C mid-wave; §12.4 The ✗ line and the next moves; Corrigenda (2026-10-19, §9.2)

## token-aware-diff-sizing-4-9

### Task 4-9: The first final failure ends the wave

**Problem**: Once one chunk has failed for good, the generation has already failed, so classifying further failures could only confirm it (§11.2). A usage limit or an expired login landing mid-wave would otherwise send up to four calls per chunk at a service already refusing them: the call, its retry, the classify call and its retry. And the ✗ line has to report the failure that ended the run, never an earlier SIZE answer that halved.

**Solution**: A chunk failure ends the wave when it finishes that chunk for good, which is anything except a SIZE answer with halving left:
- an OTHER answer
- a failed classify call
- a failure with no output to classify
- a timeout, which gets no classify call
- a missing binary
- a SIZE answer with no halving left, or whose halving round has no room.

After the first final failure, mint starts no new chunk calls, halved parts included, and makes no further classify calls. Calls already running finish, and a failure among them is not classified. The ✗ line renders the first final failure's cause phrase and its captured output verbatim.

**Outcome**: A wave that has failed asks the AI nothing new, and the ✗ line reports the failure that ended it.

**Acceptance Criteria**:
- [ ] Given a wave whose part 2 is classified OTHER, no part that had not started is started and no further classify call is made. Calls already in flight finish (§11.2)
- [ ] Given part 2 instead ends with a classify call that fails, a failure with no output, a timeout, a missing binary, a SIZE answer with no halving left, or a halving round with no room, the wave ends the same way (§10.1, §11.2)
- [ ] Given part 2 is classified SIZE with halving left, the wave carries on and part 2's halved parts are sent (§11.2)
- [ ] Given part 2's final failure while part 5's call is in flight, and part 5 then fails with output, no classify call is made for part 5 (§11.2)
- [ ] Given part 3 classified SIZE, none of whose halved parts has started when part 5 fails for good, none of them is started. Given part 3's classify call is still running when part 5 fails for good and then answers SIZE, no halved part is sent for it either (§11.2)
- [ ] Given a 6-part wave whose part 2 is classified SIZE and re-cut into 3 parts, and whose part first packed as 5, now part 7 of 8, is then classified OTHER, the ✗ line reads `AI call failed (part 7 of 8)` with that part's captured output verbatim beneath it, never part 2's (§11.2, §12.4, §16.1)
- [ ] Given the wave ends on an unattended release under `on_notes_failure = abort`, the run stops before the tag, and under `fallback` the commit-subject list ships. An unattended single-version regenerate stops, and under `regenerate --all` the version is skipped and the batch carries on (§11.1, §12.1, §12.3)

**Context**:
> §11.2: once one chunk has failed for good, the generation has already failed. Letting in-flight calls finish costs no extra requests, and what they return makes the attended floor's `e` cheap (Phase 5). Their notes are saved with the rest (task 4-10).
>
> §10.1, §8.7: a halving round with no room sends no call, so the SIZE answer behind it has no halving to go on with (task 4-7).
>
> §11.6: a chunk call's timeout is a final failure that ends the wave. There is no overall deadline across the calls.
>
> Phase 2's task 2-10 made every chunk failure end the wave, with no new chunk calls and in-flight calls left to finish, and surfaced the first failure's output. This task keeps that for final failures, while task 4-4 lets a SIZE answer with halving left carry on, and stops classify calls once the wave has ended. Part numbers follow task 4-5's numbering by place in chunk order: the examples name parts as first packed until a re-cut renumbers them.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §8.7 Calls that cannot be made; §10.1 When it applies; §11.1 One failed chunk fails the generation; §11.2 The first final failure ends the wave; §11.6 Timeouts across a wave; §12.1 What applies, by mode; §12.3 Regenerate at the floor; §12.4 The ✗ line and the next moves; §16.1 Pretty mode: one notes line, updated; Corrigenda (2026-10-19, §16.1)

## token-aware-diff-sizing-4-10

### Task 4-10: Save the finished chunk notes when a chunked run fails

**Problem**: A chunked run that fails throws away the notes its finished chunks already produced, the most expensive part of the run (§11.4). Whoever picks it up next, the operator at the floor or an agent session later, has to run the whole wave again or write every note by hand.

**Solution**: When a chunked run fails with at least one chunk's notes finished, or its merge fails, write those notes to `{user cache dir}/mint/chunk-notes/{repo path, separators as dashes}/{tag}.md`:
- each finished chunk's notes exactly as the chunk calls returned them, in chunk order, separated by a blank line
- notes from calls still running when the wave ended are saved with the rest
- a halved slice's finished parts sit in that slice's place.

The failure names the file's full path. The file is written whatever `on_notes_failure` holds, before the run goes to the floor. A wave in which no chunk finished, or a single-pass failure that never chunked, writes no file and names none. A later failure for the same tag overwrites it. It sits beside the dry-run notes cache but outside its pruning. A Ctrl-C mid-wave saves nothing. The README documents the file with `mint release`.

**Outcome**: A failed chunked run leaves its finished notes in a known file outside the repo, ready to tidy and release with `--notes-file`.

**Acceptance Criteria**:
- [ ] Given a release of tag `v1.4.0` in the repo at `/Users/lee/Code/app` whose 6-part wave fails for good at part 3 after parts 1, 2 and 4 finished, mint writes `{user cache dir}/mint/chunk-notes/-Users-lee-Code-app/v1.4.0.md`. Under `on_notes_failure = abort`, the failure names that full path after the AI's verbatim output (§11.1, §11.4, §12.1)
- [ ] Given that run, the notes the file holds are parts 1, 2 and 4's, byte for byte as the calls returned them, in chunk order, with a blank line between one part's notes and the next. Given part 5's call was in flight when part 3 failed for good and finishes with notes, those notes sit in part 5's place (§11.2, §11.4)
- [ ] Given every part finishes and the merge fails, because the merge call fails, its classify answer is SIZE, or its prompt is over the cap, the file is every part's notes in chunk order, separated by a blank line, and nothing else (§11.3, §11.4)
- [ ] Given a run in which a slice halved, each finished halved part's notes sit in the halved slice's place in chunk order. Given a single-pass call that halved and whose halved parts partly finished before a final failure, a file is written with those parts' notes (§8.6, §10.1, §11.4)
- [ ] Given `on_notes_failure = fallback`, the file is written as under `abort`, and the commit-subject list ships (§11.1, §12.1)
- [ ] Given a wave in which no part's call returned notes before the run failed, no file is written and the failure names none. Given a single-pass call that fails without halving, no file is written (§11.4)
- [ ] Given a saved file for `v1.4.0` and a later chunked run for `v1.4.0` that fails with parts finished, the file holds only the later run's notes (§11.4)
- [ ] Given a saved file and a later dry run in the same repo that writes the dry-run notes cache, the saved file is still in place (§11.4)
- [ ] Given a Ctrl-C while chunk calls are in flight, with some parts finished, no file is written (§11.5)
- [ ] The README's `release` section documents the saved file: where it is written, what it holds, that the failure names its path, and that `--notes-file` releases it, with `mint release --notes-file <path>` while the version is not out or `mint release regenerate <version> --notes-file <path>` once it is (§11.4, §12.4, §18.2)

**Do**:
- The file sits beside the dry-run notes cache (`{user cache dir}/mint/cache`, built by `newNoteCache` in `cmd/mint/main.go`), in its own `mint/chunk-notes` directory, outside the cache's pruning (§11.4).
- The README documentation goes in its `release` section (§18.2).

**Context**:
> §11.4: the file lives outside the repo, beside the dry-run cache but outside its pruning, because it must outlive the run. It keeps the wave's work: at a terminal the floor's `e` opens it (Phase 5), and `--notes-file` releases it (Phase 3), on the release itself or, for a version already out, through regenerate. There is no file when no chunk finished, because a file holding only the marker (task 4-11) would pass `--notes-file`'s empty check and ship as the release's notes.
>
> §8.7: chunk notes are held in memory for the run. They persist only in a failure's saved file and in the dry-run cache (Phase 6).
>
> §11.5: an interrupted chunked run behaves like an interrupted single-pass one. It stops, and a re-run starts over; mint keeps no resume state.
>
> §12.1 orders the attended failure as the ✗ line, the AI's verbatim output, then the saved chunk-notes pointer. In this phase, task 4-11 adds the missing-notes marker, task 4-12 the warning when the file cannot be written, and task 4-13 regenerate's files. Phase 5 names the saved file in the `fallback` warning with `mint release regenerate <version> --notes-file <path>`, shows it as the attended floor's pointer and in the unattended next moves, and saves it on a dry run as on a real run.
>
> As built:
> - `notescache.repoScope` (`internal/notescache/cache.go`) munges the cleaned repo root's path separators to dashes, so `/Users/lee/Code/mint` becomes `-Users-lee-Code-mint`.
> - `Store.Prune` removes only `.json` entries in the repo's own cache directory.
> - `newNoteCache` resolves `os.UserCacheDir` and disables the dry-run cache when it fails.
> - `fsutil.WriteFile` is the crash-safe temp-file-and-rename write idiom.
> - On the forward release, `ResolveFailure` (`internal/notes/resolve.go`) routes a notes failure to abort or fallback. Under fallback no ✗ line renders.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §8.6 What each call is told; §8.7 Calls that cannot be made; §10.1 When it applies; §11.1 One failed chunk fails the generation; §11.2 The first final failure ends the wave; §11.3 A failed merge; §11.4 The saved chunk-notes file; §11.5 Ctrl-C mid-wave; §12.1 What applies, by mode; §12.4 The ✗ line and the next moves; §18.2 The README

## token-aware-diff-sizing-4-11

### Task 4-11: Mark the files the saved notes are missing

**Problem**: A saved chunk-notes file from a wave that failed partway reads like notes for the whole release, though some files got none (§11.4). It is picked up later, through the floor's `e` or by an agent releasing with `--notes-file`, maybe after the terminal output is gone. Released as it stands, it would quietly leave out the slice that failed, which can be the release's headline change (§11.1).

**Solution**: When any slice has no notes, the file opens with one bold line, then a blank line before the notes:

```text
**Notes missing:** no notes were written for {files} — add them before releasing.
```

The line lists every file with no notes, including files in parts the wave never started or whose calls failed after it ended. A file whose pieces only partly failed is marked `(partly)`. A file in which every slice has notes, as after a failed merge, carries no marker. The README's description of the saved file names the marker.

**Outcome**: Whoever opens the saved file sees at once which files still need notes before it is released.

**Acceptance Criteria**:
- [ ] Given a 3-part wave that fails for good at part 3, which carries `api/handler.go` and `api/routes.go`, after parts 1 and 2 finished, the file's first line is `**Notes missing:** no notes were written for {files} — add them before releasing.`, with `{files}` naming both files. A blank line follows, then parts 1 and 2's notes (§11.4)
- [ ] Given parts the wave never started before it ended, or in-flight calls that failed after it ended, their files are listed in the marker too (§11.2, §11.4)
- [ ] Given a file sent as pieces, some of which finished and some not, the marker lists it marked `(partly)`. A file none of whose pieces finished is listed unmarked (§11.4)
- [ ] Given a slice that halved, files whose halved parts finished are not listed and files whose halved parts did not finish are. A file the re-cut split into pieces, some of which finished, is marked `(partly)` (§10.1, §11.4)
- [ ] Given every part finished and the merge failed, the file has no marker and opens with part 1's notes (§11.3, §11.4)
- [ ] The README's description of the saved file names the marker and says to fill in the files it lists before releasing (§11.4, §18.2)

**Context**:
> §11.4: the marker lives in the file itself because the file is what gets picked up later: by the operator through `e`, or by an agent session releasing with `--notes-file`, possibly after the terminal output is gone. Accepted cost: the marker line ships if the file is released unread. An attended release shows it at the review gate.
>
> §11.1: merging what came back is out, because the missing slice can be the release's headline feature. The marker carries the same warning into the saved file.
>
> Phase 5's next moves tell the operator to tidy the saved notes, filling in any files the marker lists, and release with them (§12.4). How `{files}` separates the paths it lists is the implementer's.
>
> Task 4-10 writes the file, and task 4-13 applies the marker to regenerate's files.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §10.1 When it applies; §11.1 One failed chunk fails the generation; §11.2 The first final failure ends the wave; §11.3 A failed merge; §11.4 The saved chunk-notes file; §12.4 The ✗ line and the next moves; §18.2 The README

## token-aware-diff-sizing-4-12

### Task 4-12: Warn when the chunk notes cannot be saved

**Problem**: Writing the chunk-notes file can fail: the user cache dir may not resolve, or the write itself may fail (§11.4). Naming no file and saying nothing would hide that finished notes were lost. Letting the write error take over the failure would hide why the notes failed in the first place (§9.6).

**Solution**: When the file cannot be written, the failure stays the AI's: its ✗ line, cause phrase and captured output are unchanged. mint adds one warning that the finished chunk notes could not be saved, with the write error verbatim beneath it. The failure names no path, and the run goes on to the floor as when no file was saved.

**Outcome**: The operator learns both why the notes failed and that the finished part notes were not kept.

**Acceptance Criteria**:
- [ ] Given a chunked run that fails with parts finished under `on_notes_failure = abort`, and a chunk-notes write that fails, the ✗ line, its cause phrase and the AI's captured output beneath it are what they would be had the write succeeded (§9.6, §11.4)
- [ ] Given that run, one warning says the finished chunk notes could not be saved, with the write error verbatim beneath it, and the failure names no chunk-notes path (§11.4)
- [ ] Given a user cache dir that cannot be resolved, the same one warning shows, with the resolution error verbatim beneath it, and no path is named (§11.4)
- [ ] Given `on_notes_failure = fallback` and a write that fails, the same one warning shows and the commit-subject list ships (§11.4, §12.1)
- [ ] Given a wave in which no chunk finished, no such warning shows, whether or not the user cache dir resolves, since no file was due (§11.4)

**Context**:
> Corrigendum 2026-10-19 on §11.4: the ✗ line keeps the AI's failure and verbatim output (§9.6, §11.2), so a write error cannot replace them. Naming no file silently would hide that finished notes were lost, against CLAUDE.md's "Fail loud" and verbatim-diagnostics idioms. §11.4's no-file branch and §12.4's "Where no file was saved" guidance already define the run without a file.
>
> As built:
> - `Presenter.Warn` takes a `Warning` whose `Output` renders verbatim beneath the warn line, inside the sliceable `--- output ---` delimiters in plain mode. `Output` is not duplicated to stderr, and a warning does not set failure state.
> - The `Presenter.StageStarted` doc's stream-discipline note forbids a `Warn` between a blocking stage's start and its completion unless the spinner is suspended.
> - `newNoteCache` (`cmd/mint/main.go`) treats an unresolvable `os.UserCacheDir` as the dry-run cache switched off, with no output. The chunk-notes file has no such silent path.
>
> Phase 5 builds the rest of the run without a file: `e` opens empty even when a file from an earlier failure of the same tag sits at the path, the guidance says to write the notes and release with `--notes-file`, the `fallback` warning names no file, and a dry run warns as a real run does. Task 4-13 applies this warning to regenerate.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §9.6 Alongside the verbatim error; §11.2 The first final failure ends the wave; §11.4 The saved chunk-notes file; §12.1 What applies, by mode; §12.4 The ✗ line and the next moves; Corrigenda (2026-10-19, §11.4)

## token-aware-diff-sizing-4-13

### Task 4-13: Regenerate saves each version's chunk notes

**Problem**: Regenerate's fresh source chunks and halves per version, as a forward release does (§1.3), so its waves fail the same ways and would lose the same finished work. A version already out takes saved notes only through `mint release regenerate <version> --notes-file <path>`, and one file answers one version (§11.4, §13.1). Under `--all`, each failed version needs its own file.

**Solution**: Regenerate's fresh source saves a failed chunked run's notes exactly as a forward release does:
- the same path, keyed by the version's own tag
- the same contents and missing-notes marker
- the same warning when the file cannot be written.

A single-version regenerate names the file's full path in its failure. Under `--all`, each failed version saves its own file, is skipped, and the batch carries on. A version whose single-pass call failed, or whose wave produced nothing, saves no file. The README's `release regenerate` section documents the file.

**Outcome**: Every failed chunked regenerate, single-version or batch, leaves that version's finished notes in a file of its own.

**Acceptance Criteria**:
- [ ] Given `mint release regenerate v1.2.0` from a fresh source whose wave fails with parts finished, mint writes `{user cache dir}/mint/chunk-notes/{repo path, separators as dashes}/v1.2.0.md`, holding those parts' notes as a forward release's file would, and the failure names its full path (§11.4)
- [ ] Given `regenerate --all` where the waves of v1.1.0 and v1.3.0 fail with parts finished, each saves its own file, `v1.1.0.md` and `v1.3.0.md`. Each is skipped, and the batch carries on and regenerates the other versions (§11.4, §12.3)
- [ ] Given a version whose single-pass call failed without halving, or whose wave produced no notes, no file is saved for it (§12.3)
- [ ] Given a version's saved file in which some slice has no notes, the file opens with the missing-notes marker, as on a forward release (§11.4)
- [ ] Given a version whose file cannot be written, one warning says its finished chunk notes could not be saved, with the write error verbatim beneath it, and no file is named for it. Under `--all` the batch carries on (§11.4, §12.3)
- [ ] The README's `release regenerate` section says a failed fresh regenerate saves its finished chunk notes, and that `mint release regenerate <version> --notes-file <path>` puts them on the version (§11.4, §13.1, §18.2)

**Context**:
> §12.3: under `--all`, skip-and-continue stays the floor whatever `on_notes_failure` holds. Phase 5 names each version's saved file in its skip warning and in the end-of-run summary, with that version's `mint release regenerate <version> --notes-file <path>`, and gives each skip its cause phrase and captured output. Until then, a skipped version's file is saved but not named.
>
> §13.1: one file answers one version, so `--notes-file` with `--all` is a usage error (Phase 3). Each version's file goes on with that version's own command.
>
> As built:
> - A single-version fresh regenerate surfaces a notes failure through `surface` in `RegenerateRun` (`internal/engine/regenerate_interactive.go`) and aborts.
> - `--all` catches a `ProduceBody` error in `processOneVersion` and records the version as skipped through `reportSkip` (`internal/engine/regenerate_batch.go`).
> - Each version's fresh body comes from `RegenerateFreshBody` (`internal/engine/regenerate_fresh.go`) over that version's range, and `ProduceBody` returns only the body.
> - Both paths narrate each version's notes as a blocking stage, so a warning between its start and its completion has to suspend the spinner (the `Presenter.StageStarted` stream-discipline note).
>
> Tasks 4-10, 4-11 and 4-12 built the file, its marker and its write-failure warning on the forward release.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §1.3 The consumers; §11.4 The saved chunk-notes file; §12.3 Regenerate at the floor; §13.1 What it is; §18.2 The README

## token-aware-diff-sizing-4-14

### Task 4-14: One lower-cap hint for a `regenerate --all` batch

**Problem**: Under `regenerate --all` several versions can halve. A hint after each would repeat one remedy with differing sizes the operator would have to reconcile, although the whole batch runs under one cap and one command (§10.3).

**Solution**: Under `--all`, print no hint after any single version's notes. Print one hint when the batch ends, once any version's notes were written after halving. It names the largest diff-carrying call that succeeded across the batch, whichever version made it, as its exact byte count: `max_prompt_bytes = {bytes}`. It prints however the batch ends, including a batch that stops early at a declined gate, a failed `r` or a Ctrl-C.

**Outcome**: A batch that halved anywhere ends with one cap suggestion that covers every version it ran.

**Acceptance Criteria**:
- [ ] Given `regenerate --all` in which v1.1.0 and v1.3.0 each have notes written after halving, no hint shows after either version's notes, and exactly one hint shows when the batch ends, at a terminal and under `-y` alike (§10.3)
- [ ] Given that batch, where v1.3.0's largest successful single-pass, chunk or piece call is 1,260,000 bytes and v1.1.0's is 700,000, the hint says the cap is set higher than this model accepts and shows `max_prompt_bytes = 1260000`. A larger successful diff-carrying call in a version that needed no halving sets the value instead (§10.3)
- [ ] The value is that call's exact byte count, never rounded and never in the narration's decimal units (§10.3)
- [ ] Given a batch in which no version's notes were written after halving, including one whose only halving was in a version that then failed and was skipped, no hint shows (§10.3)
- [ ] Given a batch in which a version's notes were written after halving, and the operator then declines a gate, at that version or a later one, the hint still prints once as the batch ends. A batch that ends at a failed `r` prints it the same way, and so does one ended by a Ctrl-C, on the way out (§10.3, §11.5)

**Context**:
> Corrigenda 2026-10-19 on §10.3: the hint advises on one setting, and the batch runs under one cap and one command, so per-version hints would repeat one remedy with differing sizes the operator would have to reconcile. "In the run" reads as the batch, and the batch already reports in one place at its end (`batchSummary`). The hint is tied to the notes being written after halving, not to how the run ends, and its ground holds for a version already regenerated before the batch stopped. A Ctrl-C counts too: the hint records nothing, so §11.5's clean abort holds.
>
> Task 4-6 built the single-run hint: its message, the calls that count (single-pass, chunk and piece, never the merge or classify call), and the exact-bytes value.
>
> As built:
> - `RegenerateAll` (`internal/engine/regenerate_batch.go`) closes a batch that runs to the end with one `RunFinished` carrying `batchSummary`. A gate decline propagates out of the loop and ends the batch with no summary.
> - Every per-version gate runs through `gateRegenerate` (`internal/engine/regenerate_write.go`), which shows the notes on the single-version and batch paths alike. Under `-y` the batch skips the gate.
> - A failed `r` at a version's gate ends the batch as built. Phase 6 has it save that version's chunk notes first (§15.1).

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §10.3 Telling the operator; §11.5 Ctrl-C mid-wave; §15.1 `r` on chunked notes; §16.3 Follow-ups; Corrigenda (2026-10-19, §10.3)
