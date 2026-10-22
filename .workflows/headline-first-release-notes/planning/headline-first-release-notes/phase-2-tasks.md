# Phase 2: The chunked path writes the headline at the merge — 5 tasks

## headline-first-release-notes-2-1

### Task 2-1: Part calls leave the release-wide summary to the merge

**Problem**: A release too large for one call is written in parts, and each chunk or piece call carries the release's full instructions. Once Phase 1's headline rules are in the default prompt, every part call is asked for a headline too. A part sees only one slice: it cannot know whether that slice is the release's main work or a footnote, and only the merge sees the whole (§8). Today's framings tell each part to write notes "in the format above" and nothing keeps it from summing up the release.

**Solution**: Add a format-neutral instruction to the chunk framing and the piece framing: leave out anything that sums up the release as a whole, because the merge writes that. The instruction names no headline. On the default prompt it means the headline. Under an override whose format has no such line, it asks nothing. The rest of each part call stays as it is.

**Outcome**: Every part call, including the smaller calls of a halving, asks the part to leave the release-wide summary to the merge, in words that fit whichever format governs.

**Acceptance Criteria**:
- [ ] Given a repo on the default prompt whose release's notes prompt is over `max_prompt_bytes`, when its notes are written in parts, every chunk call's framing and every piece call's framing ask the part to leave out anything that sums up the release as a whole, because the merge writes that (§8)
- [ ] The same framings never name a headline (§7.1, §8)
- [ ] The same framings still name the call's part (for a piece, also its piece and its file) and ask for release notes for the changes in it, in the format above, to be merged with the other parts' notes (§7.1, §8)
- [ ] Given a single call the model rejects as too big, and given a part the model rejects as too big, when mint retries that slice as smaller calls, every one of the smaller calls carries the same framing (§8)
- [ ] Given a repo on a full `[release].prompt` override whose release is written in parts, its chunk and piece calls carry the same framings, its instructions are its prompt file verbatim, and nothing mint adds to the call names or asks for a headline, the framing and the closing reminder included (§7.2, §8)
- [ ] Every chunk and piece call holds the release's instructions, the whole-release Change Map, the framing that names its part, its slice of the diff and the closing reminder, in that order, and nothing else (§1.2, §8)

**Do**:
- The framings are `chunkPart.framing` in `internal/notes/chunk.go`: the chunk branch and the piece branch, which share `chunkFramingOpening`. `composeChunkPrompt` composes them into each call. The halved calls (`generateHalved`, `wave.halve`, `cutter.recut`) go through the same `framing` (§8).
- The wording is format-neutral. Every headline rule stays in `DefaultPrompt` (`internal/notes/prompt.go`) alone (§7.1).

**Context**:
> §8: part calls write no headline, and the merge alone writes it, judging the whole release from every part's notes and the whole-release Change Map (task 2-2). On the default prompt this instruction leaves out the headline. Under an override whose format has no such line, it asks nothing.
>
> §1.2: the transport, part sizing, halving and the saved part notes stay as they are. The cutter measures each call's fixed part with its framing (`cutter.fixedPart`), so a longer framing is counted there already.
>
> The framings' other new instruction, for a part whose slice holds nothing the notes would list, is task 2-3. Each call's closing reminder is Phase 1's (task 1-12).
>
> §10.3: E1 never exercised the chunked path. Phase 3's first runs take held-out releases through the part calls and the merge. A headline in any part's notes holds shipping until the wording clears it.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §8 The Chunked Path; §7.1 mint's shared wording stays format-neutral; §7.2 What reaches which repo; §1.2 What does not change; §10.3 Where the tuning goes (the chunked path)

## headline-first-release-notes-2-2

### Task 2-2: Merge framing leaves ranking to the governing format

**Problem**: The merge framing tells the model to "rank by the Change Map above". §6.2 makes the map a check on where the changes landed, never a measure of importance, and §1.5 retires token-aware diff sizing's merge "ranking by the Change Map". Left in place, that line overrides the default prompt's ordering by weight to the user (§6.1) on every chunked release, and imposes a ranking on an override's format. The merge is also the one call on the chunked path that writes the headline (§8), so its framing has to leave what it writes to the governing format.

**Solution**: Drop "rank by the Change Map above" from the merge framing. Keep its other instructions: one set of release notes for the whole release, in the format above, combining related changes across parts into single bullets and dropping duplicates. The framing names no headline. The governing format decides what the merge writes, on the default prompt including the headline, from every part's notes and the whole-release Change Map. Every re-merge is composed with the same framing.

**Outcome**: The merge call asks for the release's notes in the governing format with no ranking rule of mint's own, on the run's merge and on every re-merge.

**Acceptance Criteria**:
- [ ] Given a repo on the default prompt whose release is written in parts, when the merge call is made, its framing asks for one set of release notes for the whole release, in the format above, combining related changes across parts into single bullets and dropping duplicates. It no longer says to rank by the Change Map (§6.2, §7.1)
- [ ] The same merge call holds the release's instructions, the whole-release Change Map, the framing, every part's notes in part order and the closing reminder: the Change Map stays in the call (§6.2, §8)
- [ ] The merge framing names no headline (§7.1, §8)
- [ ] Given a repo on a full `[release].prompt` override whose release is written in parts, its merge call carries the same framing, and nothing mint adds to it names or asks for a headline (§7.2)
- [ ] Given chunked notes at the attended review gate of `mint release` or of a fresh-source `mint release regenerate`, when the operator presses `r` and gives a one-time nudge such as "lead with the auth work", only one merge call is made, over the part notes already written, and no chunk or piece call. Its instructions carry the nudge, and it carries the same merge framing (§8, §3.5)
- [ ] Given chunked notes a real run reuses from the dry-run cache, when the operator presses `r` with a one-time nudge, the re-run is the same single merge call over the cached part notes, carrying the nudge and the same framing (§8)

**Do**:
- The framing is `mergeFraming` in `internal/notes/chunk.go`. `composeMergePrompt` composes it, and both the run's merge (`Generator.merge`) and every re-merge (`Generator.Remerge`, `internal/notes/remerge.go`) go through it (§7.1, §8).
- The `r` routes stay as they are: `RegenRun.regenerator` in `internal/engine/chunknotes.go`, and the cached merge inputs a reused entry hands the keeper in `internal/engine/release.go` (`chunkNotes.merged = entry.Merge`) (§8).

**Context**:
> §8: a part sees one slice and cannot know whether it is the release's main work or a footnote, so the merge alone writes the headline, judging the whole release from every part's notes and the whole-release Change Map. On the default prompt the governing format is Phase 1's, with its headline rules (tasks 1-2 to 1-4) and its ordering by weight (task 1-10).
>
> §7.1: ranking belongs to the governing format. The merge framing gains one instruction, for parts that held nothing (task 2-4).
>
> §8: every regenerate goes through the merge. On chunked notes, `r` re-runs only the merge, and a merge re-run from saved part notes goes through it too. So the headline, and any one-time nudge, stay with the call that writes the headline. §3.5: the nudge is a one-time line appended to the resolved instructions, never written to config.
>
> §1.5: this replaces token-aware diff sizing's merge call "ranking by the Change Map". §6.2: the map stays in every notes call, merge included, built as today.
>
> §10.3: Phase 3 checks that the merge writes the headline §3.2 calls for. A merged headline that names something other than the release's main work holds shipping until the wording clears it. A merge that leaves out a headline §3.2 calls for costs what today's notes cost and is tuned, not held.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §8 The Chunked Path; §6.2 The Change Map is coverage, not ranking; §7.1 mint's shared wording stays format-neutral; §7.2 What reaches which repo; §1.5 Decisions this replaces; §3.5 Room for a supplied headline (the `r` nudge); §10.3 Where the tuning goes (the chunked path)

## headline-first-release-notes-2-3

### Task 2-3: A part with nothing to list returns one line saying so

**Problem**: Under the default prompt's rules, a part's slice can hold nothing the notes would list: only changes such as CI, tests or internal refactors, and nothing users meet. Such a part has nothing to write, but the transport rejects an empty body as a failed call, which fails the part and with it the run. Nor may the part write the release's maintenance line: on the chunked path only the merge can say that of the release, because only the merge sees every part (§5.1, §8).

**Solution**: Add a format-neutral instruction to the chunk and piece framings: a part whose slice holds nothing the notes would list returns one line saying so. The line is about the part, never the release, and the framing names neither a headline nor the maintenance line. Extend the default prompt's doubt rule (task 1-8) to a part's slice: when unsure whether a part's slice holds any change users meet, the change in doubt is listed in its section, and neither the part's line nor the maintenance line is written.

**Outcome**: A part with nothing to list returns its one line instead of an empty body. On the default prompt a part lists a change it is unsure about rather than writing that line.

**Acceptance Criteria**:
- [ ] Given a release written in parts, every chunk call's framing and every piece call's framing ask a part whose slice holds nothing the notes would list to return one line saying so. The line speaks of that part, never of the release (§5.1, §8)
- [ ] The same framings ask for that line in place of an empty body, and a part call that returns an empty body still fails the part as any empty body fails a call (§5.1, §8)
- [ ] The same framings name neither a headline nor the maintenance line (§7.1)
- [ ] Given a repo on a full `[release].prompt` override whose release is written in parts, its chunk and piece calls carry the same instruction, and nothing mint adds to them names or asks for a headline (§7.2, §8)
- [ ] Given a repo on the default prompt, the instructions say that when unsure whether a part's slice holds any change users meet, the change in doubt is listed in its section and neither the part's line nor the maintenance line is written (§5.1)
- [ ] Given the override repo, nothing mint adds to its calls carries that doubt rule or speaks of changes users meet: the rule lives in the default prompt alone (§7.1, §7.2)

**Do**:
- The instruction joins both branches of `chunkPart.framing` in `internal/notes/chunk.go`, worded format-neutrally (§7.1, §8).
- The doubt rule for a part's slice lives in `DefaultPrompt` in `internal/notes/prompt.go` alone, extending task 1-8's rule for the release (§5.1, §7.1).

**Context**:
> §5.1: on the chunked path only the merge can say the release holds nothing users meet. A part whose slice holds nothing users meet returns one line saying its part holds nothing for the notes, and the merge writes the maintenance line only when every part returned one (task 2-4). On the default prompt, "nothing the notes would list" is nothing users meet.
>
> §5.1, the doubt rule's asymmetry: a line left out costs what today's notes cost, while a false one drops a change users meet.
>
> §1.2: mint never parses or rewrites a notes body. The merge leaves the line out on its framing's instruction (task 2-4), and the saved part notes keep it as the part wrote it (task 2-5). The instruction to leave the release-wide summary to the merge is task 2-1.
>
> §10.3: Phase 3 runs at least one release cut so that a part's slice holds nothing users meet. A part that returns an empty body and fails the run holds shipping until the wording clears it. A part that lists its internal changes instead costs what today's notes cost and is tuned.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §8 The Chunked Path; §5.1 Only what users meet (a release with nothing users meet); §7.1 mint's shared wording stays format-neutral; §7.2 What reaches which repo; §1.2 What does not change; §10.3 Where the tuning goes (the chunked path)

## headline-first-release-notes-2-4

### Task 2-4: Merge leaves the parts' nothing-to-list lines out

**Problem**: Once a part can return a line saying its slice holds nothing for the notes, the merge receives those lines among the other parts' notes. Carried into the merged notes, a line about one part misstates the release in a body every reader sees. Where every part returned one, the release has nothing to list, and only the merge can say so of the release (§5.1, §8). The merge framing has no instruction for either case.

**Solution**: Give the merge framing its one new instruction, worded format-neutrally: leave the parts' nothing-to-list lines out, and where every part returned one, write what the governing format gives a release with nothing to list. The framing names neither a headline nor the maintenance line and supplies no nothing-to-list body of its own. On the default prompt, the body it points to is §5.1's maintenance line.

**Outcome**: The merge leaves the parts' lines out beside other parts' sections, and writes the governing format's nothing-to-list body only when every part returned one: on the default prompt, the maintenance line.

**Acceptance Criteria**:
- [ ] Given a release written in parts, when the merge call is made, its framing tells the merge to leave out the lines in which parts said they held nothing for the notes (§8)
- [ ] The same framing tells the merge that where every part returned such a line, it writes what the governing format gives a release with nothing to list, and only then, never while any part listed a change (§5.1, §8)
- [ ] The same framing names neither a headline nor the maintenance line, and gives no nothing-to-list body of its own (§7.1, §8)
- [ ] Given a repo on the default prompt, the merge call's instructions carry §5.1's rule for a release with nothing users meet: one plain line in the shape of `Maintenance release — no notable source changes`, with no headline and no sections (§5.1, §8)
- [ ] Given a repo on a full `[release].prompt` override whose release is written in parts, its merge call carries the same framing, and nothing mint adds to it supplies a fallback line or asks for a headline. A merge call there that returns an empty body fails as any call with an empty body fails (§7.2, §8)
- [ ] Beyond the instructions it keeps (one set of notes for the whole release in the format above, related changes across parts combined into single bullets, duplicates dropped), this is the only instruction the merge framing gains (§7.1)

**Do**:
- The instruction joins `mergeFraming` in `internal/notes/chunk.go`, which the run's merge and every re-merge share through `composeMergePrompt` (§7.1, §8).
- The maintenance-line rule the default prompt carries is task 1-8's, in `DefaultPrompt` (`internal/notes/prompt.go`) (§5.1, §7.1).

**Context**:
> §8, as corrected 2026-10-22: under a full `[release].prompt` override whose format gives no body for a release with nothing to list, the merge framing supplies no fallback of its own. The format is the operator's (§7.2), and a fallback line would carry §5.1's rule into mint's shared wording, which §7.1 keeps format-neutral. The merge writes what it can from that format, and an empty body it returns fails the call as any empty body does (§5.1: the transport rejects an empty body as a failed call).
>
> §10.3 holds shipping until the wording clears each of these: merged notes that carry a part's line or the maintenance line while any part listed changes users meet; merged notes that keep the parts' lines in place of the maintenance line; a merge that returns an empty body and fails the run on a release with nothing users meet. Phase 3 checks them, first on tuning drafts and then on the frozen wording.
>
> The part's line itself is task 2-3. The merge framing's kept instructions and the dropped ranking line are task 2-2.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §8 The Chunked Path; §5.1 Only what users meet (a release with nothing users meet); §7.1 mint's shared wording stays format-neutral; §7.2 What reaches which repo; §10.3 Where the tuning goes; Corrigenda (2026-10-22, the merge under a full override)

## headline-first-release-notes-2-5

### Task 2-5: Saved part notes keep the nothing-to-list lines, with README guidance on releasing them

**Problem**: When a chunked run fails after some parts finished, mint saves each finished part's notes byte for byte, for the operator to tidy and release with `--notes-file`. Parts can now return a line saying their slice held nothing for the notes, and the saved file keeps it as it keeps any part's notes. The README tells the operator only to fill in the files the marker lists and delete the marker line, so such a line would ship in the release's notes. Where every part returned one, deleting the lines leaves an empty file, which `--notes-file` refuses (§8).

**Solution**: Keep the saved file's rule as it is, and pin it for a part that returned its line. Change the README's Saved part notes paragraph to tell the operator to delete these lines as well as the marker line, and, where every part's notes are that line, to release §5.1's maintenance line in their place rather than an empty file. The next moves mint prints stay as they are.

**Outcome**: A saved file holds each part's line as the part wrote it, and the README tells the operator to remove those lines before releasing, or to release the maintenance line where nothing else would remain.

**Acceptance Criteria**:
- [ ] Given a chunked run where one part returned its nothing-to-list line, other parts wrote sections, and the merge then fails, whether the run's merge or an `r` re-merge, the saved file holds each finished part's notes byte for byte in chunk order, that line included, with one blank line between one part's notes and the next. mint adds no line of its own, a headline included (§8, §3.4)
- [ ] Given a chunked run where one part returned its nothing-to-list line and another part fails for good, the `**Notes missing:**` marker opens the saved file exactly as it would without the line: the part that returned its line counts as a part with notes, and the marker lists only the files of parts with none (§8)
- [ ] Given the README's Saved part notes section, its paragraph on releasing a saved file tells the operator to delete any part's line saying it held nothing for the notes, as well as the marker line, before releasing the file with `--notes-file` (§8)
- [ ] The same paragraph tells the operator that where every part's notes are that line, the release holds nothing users meet, and to release §5.1's maintenance line in their place rather than an empty file, which `--notes-file` refuses (§5.1, §8)
- [ ] Given a run that stops with saved part notes, the next moves mint prints are unchanged: they tell the operator to tidy the saved notes, filling in any files its marker lists, and release with `--notes-file` (§8)
- [ ] Given notes the operator releases from the saved file, by `--notes-file` or by editing it with `e` at the notes floor, the body ships as the operator left it, with a headline only where the operator wrote one (§8, §3.4)

**Do**:
- The saved file's rule is `keepChunkNotes`, `KeepPartNotes`, `joinChunkNotes` and `notesMissingLine` in `internal/notes/chunknotes.go`, and it stays as it is (§8).
- The guidance is the README's Saved part notes paragraph that tells the operator to delete the marker line (`grep -n "delete the marker line" README.md` → line 318). It is the one text that does (§8).
- The next moves are `notesFileMove` in `internal/engine/notesfloor.go` and `savedNotesPointer` in `internal/engine/regenerate_batch.go`, and they stay as they are (§8).

**Context**:
> §8: this feature changes what the part calls return, not the file's rule. The file holds the parts' sections and any part's line saying it held nothing for the notes, which the operator removes while tidying, as with the marker line. A release shipped from the file carries a headline only if the operator writes one while tidying: the doubt rule (§3.2) on a path where no call saw the whole.
>
> Corrigenda 2026-10-22: the README's Saved part notes paragraph is the guidance, and the printed next moves, which tell the operator to tidy the saved notes, stay as they are. Where every part's notes are that line, deleting the lines alone leaves an empty file, which `--notes-file` refuses (the README's Finished notes with `--notes-file` section), so the guidance names the maintenance line instead.
>
> The part's line is task 2-3, and the merge leaving it out is task 2-4.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §8 The Chunked Path; §5.1 Only what users meet (a release with nothing users meet); §3.4 Bodies that carry no headline; §3.2 When a release has one (the doubt rule); Corrigenda (2026-10-22, the saved-notes guidance and a file of nothing-to-list lines)
