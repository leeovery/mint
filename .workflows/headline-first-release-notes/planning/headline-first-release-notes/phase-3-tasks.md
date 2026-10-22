# Phase 3: Tuning, then acceptance on the frozen wording — 10 tasks

## headline-first-release-notes-3-1

### Task 3-1: Headline holds under the neutral reminder on single calls

**Problem**: Every notes call now closes with §7.1's format-neutral reminder, which does not name the headline. E1's frozen candidate was measured with a reminder that did ("the one-sentence headline if this release has one, then the sections"), so whether the model still writes a headline under the neutral wording is untested (§7.1, §10.3). A single call can now also return §5.1's maintenance line, a body new-v3 never wrote. Written for a release with a change users meet, it drops that change and tells every reader, in the release's only line, that nothing changed for them (§5.1, §10.3).

**Solution**: The first runs take E1's held-out releases through single calls on the wording from Phases 1–2, closed by §7.1's reminder, and check that the notes carry a headline where §3.2 calls for one. Where headlines drop, the default prompt's wording is tuned, and the reminder stays as §7.1 words it. Evidence that the model keeps a headline only when the last thing it reads names one reopens §7.1's decision instead of being tuned around. A single call that writes the maintenance line for a release with a change users meet holds shipping until the wording clears it.

**Outcome**: The single-call runs show whether the headline survives the neutral reminder. The reminder stays word for word §7.1's, and the tuned wording writes the maintenance line on a single call only for a release with nothing users meet.

**Acceptance Criteria**:
- [ ] Given the wording from Phases 1–2, when the first runs take E1's held-out releases through single calls, every prompt ends with exactly "Now output ONLY the release notes themselves, in the format above, exactly as they should be published: no preamble, no commentary, no explanation, no code fences, nothing before or after the notes.", and the runs show, release by release, whether the notes carry a headline where §3.2 calls for one (§7.1, §10.3)
- [ ] Given single-call runs in which releases §3.2 calls a headline for come back without one, when the wording is tuned for it, only the default prompt's wording changes. The reminder stays §7.1's sentence word for word, and nothing in mint's shared wording names a headline (§7.1, §10.3)
- [ ] Given runs whose evidence shows the model keeps a headline only when the last thing it reads names one, the evidence reopens §7.1's reminder decision and is not tuned around: no wording that names the headline is added to the reminder, or to anything else mint adds to every notes call (§7.1)
- [ ] Given a release §3.2 calls a headline for that a single call leaves without one, it is the accepted cost and does not hold shipping. The bar's line for the headline is that none names the wrong subject (§3.2, §10.1)
- [ ] Given a single call in any run, on a release with a change users meet, when it returns §5.1's maintenance line, that holds shipping until the wording clears it (§10.3)

**Do**:
- The wording lives in `DefaultPrompt` in `internal/notes/prompt.go`, and tuning for the headline changes it alone. `OutputReminder` in the same file stays §7.1's sentence (§7.1).
- E1's held-out releases are the `holdout` rows of `sample.tsv` in E1's record, `.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/` (§10.2).
- A run that checks a failure §10.3 says holds shipping, this task's maintenance-line check included, uses mint's shipped default `ai_command` (`config.DefaultAICommand`, `internal/config/config.go`), as E1 ran it, never a release repo's own. Whether a release holds a change users meet is judged as E1 judged: with E1's `rubric.md` and judges, reading the release's repo and diff, plus the audit's account (`.workflows/headline-first-release-notes/imports/release-notes-audit.md`) where the release is one of the 31 (§10.2).

**Context**:
> §7.1: the reminder's job is the same on every repo: the body is the notes and nothing else. "The format above" means whichever instructions govern; on the default prompt, that is a format that opens with the headline. E1's reminder (`prompts/new-v3-reminder.txt` in E1's record) named the headline. On an override repo, that line would tell the model to put a headline above sections its format may not have.
>
> §3.2: when in doubt, leave it out. A missing headline costs no more than today's notes do, while a wrong one misstates the release in its most-read line. The headline reaches the bar through the acceptance pass's clean-lead count (at least 21 of 52; E1's candidate scored 38) and its wrong-subject count (none) (§10.1, task 3-10).
>
> §10.3: today's notes never write the maintenance line, and the counts would weigh it only by the changes it drops. So a single call that writes it for a release with a change users meet holds shipping in any run, the acceptance pass included.
>
> §10.2 and its corrigenda: a check cleared only on wording that tuning later changed has not been cleared, and the hold checks run again on the frozen wording (task 3-9). The wording stays general: no prompt text names, quotes or paraphrases a change from the 31 sampled releases, and any example added is invented (the three shipped altitude examples are exempt).

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §7.1 mint's shared wording stays format-neutral; §3.2 When a release has one; §5.1 Only what users meet (a release with nothing users meet); §10.1 The bar; §10.2 Tuning and acceptance; §10.3 Where the tuning goes (the neutral reminder; a release with nothing users meet); Corrigenda (2026-10-22)

## headline-first-release-notes-3-2

### Task 3-2: Part calls write no headline and the merge writes it

**Problem**: E1 ran every release as one call, so the chunked path's split of the headline has never run on a real release. Part calls are to write none, and the merge alone writes it, judging the whole release from every part's notes and the whole-release Change Map (§8, §10.2). A headline in any part's notes, or a merged headline that names something other than the release's main work, is a regression, since today's notes carry neither (§10.3).

**Solution**: The first runs take held-out releases through the part calls and the merge on the wording from Phases 1–2, and check every part's notes and every merged body. Where they fail, the chunk, piece and merge framings and the default prompt are tuned by wording only, with the framings kept format-neutral. Each release is kept with the parts it was cut into, so the checks on the frozen wording cut it the same way.

**Outcome**: On the tuned wording, held-out releases written in parts carry no headline in any part's notes, and a merged headline only where it names the release's main work. Each release is kept with its cut.

**Acceptance Criteria**:
- [ ] Given the wording from Phases 1–2, when the first runs take held-out releases through the part calls and the merge, every part's notes are checked for a headline. A part's notes that carry one hold shipping until the wording clears it (§8, §10.3)
- [ ] Given the merged notes of the same runs, a merged headline that names something other than the release's main work holds shipping until the wording clears it (§10.3)
- [ ] Given a release §3.2 calls a headline for, when its merge writes none, that costs what today's notes cost: it does not hold shipping, and it is tuned like the known weak spots (§10.3)
- [ ] Given a release the first runs took through the part calls and the merge, it is kept with the parts it was cut into, so the checks on the frozen wording cut it into the same parts (§10.2)
- [ ] Given the tuned framings, every chunk and piece call still asks the part, in format-neutral terms, to leave out anything that sums up the release as a whole, because the merge writes that. The merge call still asks for one set of notes for the whole release, in the format above. No framing names a headline (§7.1, §8)
- [ ] Given a full `[release].prompt` override whose release is written in parts on the tuned wording, nothing mint adds to its part or merge calls names or asks for a headline (§7.2)

**Do**:
- The framings are `chunkPart.framing` (its chunk and piece branches) and `mergeFraming` in `internal/notes/chunk.go`. The headline rules live in `DefaultPrompt` (`internal/notes/prompt.go`) alone (§7.1, §8).
- E1's held-out releases are the `holdout` rows of `sample.tsv` in E1's record, `.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/` (§10.2).
- These checks use mint's shipped default `ai_command` (`config.DefaultAICommand`, `internal/config/config.go`), as E1 ran it, never a release repo's own. What they judge is judged as E1 judged, whether a merged headline names the release's main work included: with E1's `rubric.md` and judges, reading the release's repo and diff, plus the audit's account (`.workflows/headline-first-release-notes/imports/release-notes-audit.md`) (§10.2).

**Context**:
> §8: a part sees one slice. It cannot know whether that slice is the release's main work or a footnote, and the merge is the only call that sees the whole. Every regenerate goes through the merge, so the headline, and any one-time nudge, stay with the call that writes the headline.
>
> E1's harness sends every release as one call (§10.2: E1 never exercised the chunked path). mint's cutter sizes each part with the instructions and the framing counted (`cutter.fixedPart`, `internal/notes/chunk.go`). Once tuning changes the wording's length, the same `max_prompt_bytes` can cut a release into different parts.
>
> A part whose slice holds nothing users meet is task 3-3, and a release with nothing users meet is task 3-4. These checks run again on the frozen wording in task 3-9: a check cleared only on wording that tuning later changed has not been cleared (§10.2, Corrigenda).
>
> §10.2: no prompt text names, quotes or paraphrases a change from the 31 sampled releases, and any example added is invented.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §8 The Chunked Path; §7.1 mint's shared wording stays format-neutral; §7.2 What reaches which repo; §3.2 When a release has one; §10.2 Tuning and acceptance; §10.3 Where the tuning goes (the chunked path); Corrigenda (2026-10-22)

## headline-first-release-notes-3-3

### Task 3-3: A part with nothing users meet returns its line, and the merge leaves it out

**Problem**: A part whose slice holds nothing users meet now returns one line saying its part holds nothing for the notes. The merge leaves those lines out, and writes §5.1's maintenance line only when every part returned one (§5.1, §8). None of this has run on a real release. A part that returns an empty body fails the part and the run, and merged notes that carry a part's line or the maintenance line beside parts that listed changes users meet misstate the release. Today's notes have none of these (§10.3).

**Solution**: Among the held-out releases the first runs take through the part calls and the merge, at least one is cut so that one part's slice holds nothing users meet. The runs check that this part returns its line rather than an empty body, and that the merge leaves the line out beside the other parts' sections and writes no maintenance line. Where they fail, the framings and the default prompt are tuned by wording only, and the framings stay format-neutral. The release is kept with its cut.

**Outcome**: On the tuned wording, the part returns its one line, and the merged notes carry neither that line nor the maintenance line. The release is kept with the parts it was cut into.

**Acceptance Criteria**:
- [ ] Given the wording from Phases 1–2, when the first runs take held-out releases through the part calls and the merge, at least one of them is cut so that one part's slice holds nothing users meet while other parts hold changes users meet (§10.3)
- [ ] Given that part, it returns one line saying its part holds nothing for the notes, never an empty body. A part that returns an empty body and fails the run holds shipping until the wording clears it (§8, §10.3)
- [ ] Given the same release's merge, the merged notes leave the part's line out beside the other parts' sections and do not carry the maintenance line. Merged notes that carry a part's line or the maintenance line while any part listed changes users meet hold shipping until the wording clears it (§5.1, §8, §10.3)
- [ ] Given a part whose slice holds nothing users meet that lists its internal changes instead of returning its line, it costs what today's notes cost: it does not hold shipping, and it is tuned like the known weak spots (§10.3)
- [ ] Given the release cut for this check, it is kept with the parts it was cut into, so the checks on the frozen wording cut it into the same parts (§10.2)
- [ ] Given the tuned framings, the chunk and piece framings still ask a part whose slice holds nothing the notes would list to return one line saying so. The merge framing still asks for those lines to be left out and, where every part returned one, for what the governing format gives a release with nothing to list. None of them names a headline or the maintenance line (§7.1, §8)

**Do**:
- The framings are `chunkPart.framing` and `mergeFraming` in `internal/notes/chunk.go`. The doubt rule for a part's slice and §5.1's maintenance-line rule live in `DefaultPrompt` (`internal/notes/prompt.go`) alone (§5.1, §7.1).
- E1's held-out releases are the `holdout` rows of `sample.tsv` in E1's record, `.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/` (§10.2).
- These checks use mint's shipped default `ai_command` (`config.DefaultAICommand`, `internal/config/config.go`), as E1 ran it, never a release repo's own. Whether a part's slice holds a change users meet, and whether any part listed one, is judged as E1 judged: with E1's `rubric.md` and judges, reading the release's repo and diff, plus the audit's account (`.workflows/headline-first-release-notes/imports/release-notes-audit.md`) (§10.2).

**Context**:
> §5.1: on the chunked path only the merge can say the release holds nothing users meet. When unsure whether a part's slice holds any change users meet, the change in doubt is listed in its section and neither line is written: a line left out costs what today's notes cost, while a false one drops a change users meet. The transport rejects an empty body as a failed call, which fails the part.
>
> §8: when the merge fails, the saved part notes keep a part's line as the part wrote it, and the README's Saved part notes guidance tells the operator to delete it (Phase 2).
>
> mint's cutter sizes each part with the instructions and the framing counted (`cutter.fixedPart`, `internal/notes/chunk.go`), so a cut made on one wording can move at the same `max_prompt_bytes` once the wording's length changes.
>
> A release where every part's slice holds nothing users meet is task 3-4. These checks run again on the frozen wording in task 3-9 (§10.2, Corrigenda). §10.2: no prompt text names, quotes or paraphrases a change from the 31 sampled releases, and any example added is invented.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §8 The Chunked Path; §5.1 Only what users meet (a release with nothing users meet); §7.1 mint's shared wording stays format-neutral; §10.2 Tuning and acceptance; §10.3 Where the tuning goes (the chunked path); Corrigenda (2026-10-22)

## headline-first-release-notes-3-4

### Task 3-4: A release with nothing users meet gets the maintenance line

**Problem**: §5.1 gives a release whose every change is one the notes leave out a body new-v3 never wrote: one plain line saying the release is maintenance with no user-facing changes, with no headline and no sections (§10.2). No real release has been written that way, either by a single call or by the merge once every part returned its nothing-to-list line. A headline on that release, an empty body that fails the run, or merged notes that keep the parts' lines in place of the maintenance line, are regressions, since today's notes have none of them (§10.3).

**Solution**: The first runs take at least one release with nothing users meet through a single call, and through the part calls and the merge with every part's slice holding nothing users meet, and check that the notes are §5.1's maintenance line. Where they fail, the default prompt and the framings are tuned by wording only. The maintenance rule stays in the default prompt, and the merge framing gains no fallback line of its own. The release is kept with its cut.

**Outcome**: On the tuned wording, that release's notes are the maintenance line on both paths, and the release is kept with its cut for the checks on the frozen wording.

**Acceptance Criteria**:
- [ ] Given the wording from Phases 1–2 and at least one release whose every change is one §5.1 leaves out, when the first runs take it through a single call, its notes are one plain line in the shape of `Maintenance release — no notable source changes`, with no headline and no sections (§5.1, §10.3)
- [ ] Given the same release taken through the part calls and the merge, cut so that every part's slice holds nothing users meet, every part returns its one line and the merged notes are §5.1's maintenance line (§5.1, §8, §10.3)
- [ ] Given either path, each of these holds shipping until the wording clears it: a headline on that release; a single call, a part or a merge that returns an empty body and fails the run; merged notes that keep the parts' lines in place of the maintenance line (§10.3)
- [ ] Given notes on that release that list its internal changes instead of the maintenance line, on either path, they cost what today's notes cost: they do not hold shipping, and they are tuned like the known weak spots (§10.3)
- [ ] Given the release, it is kept with the parts it was cut into, so the checks on the frozen wording cut it into the same parts (§10.2)
- [ ] Given the tuned wording, the maintenance-line rule lives in the default prompt alone. On a full `[release].prompt` override, nothing mint adds to a merge call names the maintenance line or supplies a nothing-to-list body of its own (§7.1, §7.2, §8)

**Do**:
- §5.1's rule lives in `DefaultPrompt` (`internal/notes/prompt.go`). The merge framing's nothing-to-list instruction is in `mergeFraming` (`internal/notes/chunk.go`) and names neither the maintenance line nor a body of its own (§7.1, §8).
- These checks use mint's shipped default `ai_command` (`config.DefaultAICommand`, `internal/config/config.go`), as E1 ran it, never a release repo's own. Whether the release, and each part's slice, holds a change users meet is judged as E1 judged: with the `rubric.md` and judges of E1's record (`.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/`), reading the release's repo and diff, plus the audit's account (`.workflows/headline-first-release-notes/imports/release-notes-audit.md`) where the release is one of the 31 (§10.2).

**Context**:
> §5.1: the line follows the shape of the empty-diff stub, `stubBody` in `internal/notes/degenerate.go`. That stub stays the body for an empty post-exclusion diff, which never goes through the AI (§3.4). The notes are never empty: the transport rejects an empty body as a failed call. When unsure whether the release holds any change users meet, the change in doubt is listed and the line is not written.
>
> §8 and its corrigendum: under a full override whose format gives no body for a release with nothing to list, the merge framing supplies no fallback. A fallback line would carry §5.1's rule into mint's shared wording, which §7.1 keeps format-neutral.
>
> mint's cutter sizes each part with the instructions and the framing counted (`cutter.fixedPart`, `internal/notes/chunk.go`), so a cut made on one wording can move at the same `max_prompt_bytes` once the wording's length changes.
>
> A single call that writes the maintenance line for a release with a change users meet holds shipping in any run (task 3-1). These checks run again on the frozen wording in task 3-9 (§10.2, Corrigenda). §10.2: no prompt text names, quotes or paraphrases a change from the 31 sampled releases, and any example added is invented.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §5.1 Only what users meet (a release with nothing users meet); §8 The Chunked Path; §7.1 mint's shared wording stays format-neutral; §7.2 What reaches which repo; §3.4 Bodies that carry no headline; §10.2 Tuning and acceptance; §10.3 Where the tuning goes (a release with nothing users meet); Corrigenda (2026-10-22)

## headline-first-release-notes-3-5

### Task 3-5: False claims tuned back to the bar

**Problem**: On E1's 52 held-out bodies, the frozen candidate wrote 34 false claims against today's 25, the count furthest from the bar (§10.1, §10.3). E1's own rule scored false claims and in-release-as-change as one class, which sat level at 47 while false claims rose, so netting hid the regression (§10.1). E1's judges called only 3 of the 47 accuracy residuals input-absent: the previous release's state was in the diff, and the prompt did not get it read (§5.2).

**Solution**: Tune the default prompt's wording on E1's releases until false claims come back to the bar while every other count holds its line. §5.2's rules stay as specified, nothing is added to what the AI is sent, and the wording stays general.

**Outcome**: The wording handed to the freeze brings false claims back to the bar, counted on their own, with every other count at its line, §5.2's rules unchanged and nothing added to any notes call.

**Acceptance Criteria**:
- [ ] Given the tuned wording, when a release's notes are written in one call through an `ai_command` that records its stdin, the instructions still carry §5.2's rules as specified. No behaviour is claimed from documentation, READMEs, comments, examples or tests alone, outside files that are the product. A change to tests alone is not a fix. Work built in this release is never presented as a change to, or fix of, the previous release. Nothing the previous release had is presented as new, whether the diff shows it in unchanged context lines, in removed lines, or in code it only moves or renames (§5.2, §10.3)
- [ ] Given the tuned wording, every notes call still carries only the resolved instructions, the Change Map, the call's own material (the diff, one part's slice, or every part's notes for the merge) and the closing reminder: no commit history and no new input (§1.2, §5.2)
- [ ] Given tuning runs on E1's releases, false claims are counted on their own and in-release-as-change separately, never one netted against the other (§10.1)
- [ ] Given the wording tuning hands to the freeze, its tuning runs show false claims back at the bar, at most 25 as §10.1 counts them, while every other §10.1 count holds its line (§10.1, §10.3)
- [ ] Given the tuned wording, no prompt text names, quotes or paraphrases a change from the 31 sampled releases, and any example it adds is invented (§10.2)

**Do**:
- The wording lives in `DefaultPrompt` in `internal/notes/prompt.go` alone (§7.1). Its truthfulness rules are Phase 1's: new-v3's passage, with §5.2's removed lines added (§10.2).
- What each call carries is composed by `ComposePrompt` (`internal/notes/prompt.go`), and by `composeChunkPrompt` and `composeMergePrompt` (`internal/notes/chunk.go`), and stays as composed (§1.2).
- E1's releases are the rows of `sample.tsv` in E1's record, `.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/` (§10.2).

**Context**:
> §5.2, no new inputs: the previous release's state is already in front of the AI, in the diff's removed and context lines. Three of the judges' "the previous release already had it" calls were spot-checked against the inputs, and each fact sat in the diff's removed lines. So the fix is how the prompt gets the diff read, not what mint sends.
>
> §10.1: today's prompt, measured on the same inputs, wrote 25 false claims and 22 in-release-as-change items; E1's candidate wrote 34 and 13. These are the counts `jq -c '.field_counts' …/data/secondary.json` reads from E1's record.
>
> §10.3: false breaks that assert a previous state the diff contradicts are a known weak spot (task 3-7). Missing items, the next furthest from the bar, are task 3-6. The acceptance pass on the frozen wording clears the count (task 3-10).
>
> §10.2: tuning happens during implementation, on E1's releases. The wording's generality, together with a bar that asks for no worse than today rather than a tuned threshold, guards against wording fitted to the releases it is tested on. The three shipped altitude examples are exempt from the invented-example rule.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §5.2 True to the release; §1.2 What does not change; §10.1 The bar (no netting); §10.2 Tuning and acceptance; §10.3 Where the tuning goes (false claims)

## headline-first-release-notes-3-6

### Task 3-6: Missing items tuned back to the bar, with a speed-up users notice listed

**Problem**: On E1's 52 held-out bodies, the frozen candidate left out 13 notable user-facing changes against today's 9, the count next furthest from the bar (§10.1, §10.3). Among them, it dropped a speed-up users notice under its "internal efficiency changes a user would not notice" leave-out line, which §10.2 names as a tuning point. §5.1 draws the line at what users notice: a speed-up users notice belongs to them and is listed.

**Solution**: Tune new-v3's internal-efficiency leave-out line, as carried into the default prompt, to §5.1's line at what users notice, so a speed-up users notice is listed. Tune the rest of the default prompt's wording on E1's releases until missing items come back to the bar while every other count holds. §5.1's meets-or-notices rule and the combine rule's wording stay as specified.

**Outcome**: The wording handed to the freeze lists a speed-up users notice and brings missing items back to the bar with every other count at its line. §5.1's rule and the combine rule's wording are unchanged.

**Acceptance Criteria**:
- [ ] Given a release whose diff carries a speed-up users notice, when its notes are written on the tuned wording, they list it (§5.1, §10.3)
- [ ] Given the tuned wording, when a release's notes are written in one call through an `ai_command` that records its stdin, the instructions keep §5.1's test as specified: whether a user meets or notices a change, not whether the code behind it is internal, with the in and out lists unchanged. No leave-out line drops a change users notice for being internal (§5.1, §10.2)
- [ ] Given the wording tuning hands to the freeze, its tuning runs show missing items back at the bar, at most 9 as §10.1 counts them, while every other §10.1 count holds its line (§10.1, §10.3)
- [ ] Given the tuned wording, the combine rule reads "Combine closely-related changes into a single bullet." word for word. Evidence on real releases of the model collapsing a large feature into one bullet reopens §2.2's combine rule rather than being tuned around (§2.2)
- [ ] Given the tuned wording, no prompt text names, quotes or paraphrases a change from the 31 sampled releases, and any example it adds is invented (§10.2)

**Do**:
- The line lives in `DefaultPrompt` in `internal/notes/prompt.go` (§7.1). Its starting point is new-v3's out list, line 113 of `prompts/new-v3.md` in E1's record (`.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/`): "internal efficiency changes a user would not notice", carried into the default prompt in Phase 1 (§10.2).
- E1's releases are the rows of `sample.tsv` in E1's record (§10.2).

**Context**:
> §5.1: the test is whether a user meets or notices the change, not whether the code behind it is internal. §4.1 files a speed-up users notice, to something the previous release had, under 🔧 Changed.
>
> §2.2: "closely related" means the small tweaks of one change, such as three commits that fix one gate. It never means a feature's separate capabilities: each piece a user can use or meet on its own is a notable change and gets its own bullet.
>
> §10.1: today's prompt, measured on the same inputs, left out 9 items and carried 77 internal items; E1's candidate left out 13 and carried 28. Moving the leave-out line leaves the internal-items count held to its own line (at most 77).
>
> §10.3: false claims are task 3-5. The acceptance pass on the frozen wording clears the count (task 3-10). §10.2: the three shipped altitude examples are exempt from the invented-example rule.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §5.1 Only what users meet; §2.2 Bullets (unchanged) (the combine rule); §4.1 Judged against the previous release; §10.1 The bar; §10.2 Tuning and acceptance (the internal-efficiency tuning point); §10.3 Where the tuning goes (missing items)

## headline-first-release-notes-3-7

### Task 3-7: Known weak spots tuned by wording alone

**Problem**: E1's candidate left three known weak spots in its notes (§10.3). Two-body releases lost their headline or got half of one (§3.2). A rework of existing behaviour went under ✨ Added: 36 times in today's notes, 34 in E1's candidate (§4.1). 💥 Breaking was misfiled, and some false breaks asserted a previous state the diff contradicts (§4.2). The chunked-path and nothing-users-meet checks add three outcomes that cost what today's notes cost and are tuned the same way. None of them holds shipping, but each costs readers, and the counts weigh an error at the top of the notes the same as one at the bottom (§10.4).

**Solution**: Tune the default prompt's wording, and the framings where the outcome is on the chunked path, against these outcomes on E1's releases and on the releases the first runs cut, by wording alone. The rules of §3.2, §4.1 and §4.2 stay as specified, and every §10.1 count holds its line.

**Outcome**: The wording handed to the freeze tunes the weak spots by wording alone, with the rules unchanged and every count at its line. None of these outcomes holds shipping.

**Acceptance Criteria**:
- [ ] Given the tuned wording, when a release's notes are written in one call through an `ai_command` that records its stdin, the instructions still carry, as specified: §3.2's two-body rule (two bodies of equal weight, each passing the test with the other set aside, named in one benefit-led sentence, and never a second subject that is not a co-equal body); §4.1's placement of a rework of something the previous release had under 🔧 Changed; and §4.2's narrow 💥 Breaking rule, with "when unsure, it is not breaking" (§3.2, §4.1, §4.2, §10.3)
- [ ] Given runs on the tuned wording in which a two-body release goes without its headline or gets half of one, a rework of existing behaviour is filed under ✨ Added, or a change is misfiled into or out of 💥 Breaking, a false break that asserts a previous state the diff contradicts included, each is tuned by wording and none holds shipping (§10.3)
- [ ] Given runs in which a merge leaves out a headline §3.2 calls for, a part whose slice holds nothing users meet lists its internal changes, or a release with nothing users meet lists its internal changes, each is tuned like the weak spots above and none holds shipping (§10.3)
- [ ] Given the wording tuning hands to the freeze, its tuning runs show every §10.1 count at its line (§10.1)
- [ ] Given the tuned wording, the framings stay format-neutral and name neither a headline nor the maintenance line. No prompt text names, quotes or paraphrases a change from the 31 sampled releases, and any example it adds is invented (§7.1, §10.2)

**Do**:
- The rules live in `DefaultPrompt` (`internal/notes/prompt.go`) alone. The chunk, piece and merge framings are `chunkPart.framing` and `mergeFraming` in `internal/notes/chunk.go` (§7.1, §8).
- E1's releases are the rows of `sample.tsv` in E1's record, `.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/`. The chunked releases and their cuts are the ones the first runs kept (tasks 3-2 to 3-4) (§10.2).

**Context**:
> §3.2 and §4.2 carry the same asymmetry, and it stays as a rule: when in doubt, leave the headline out, and when unsure, a change is not breaking. A missed headline or flag costs what today's notes cost, while a false one misstates the release where every reader looks. Accepted cost: some releases that deserved a headline go without one.
>
> §10.1: the lines most exposed here are misfiled (at most 84; E1's candidate 78), clean lead (at least 21; 38), breaking changes flagged (at least 1 of 10; 8) and false claims (at most 25; 34, task 3-5).
>
> §10.4: both releases where the user, reading blind, preferred today's notes carried the new prompt's errors in their opening lines. The read by eye on real releases stays alongside the counts, outside the bar (task 3-10).
>
> The hold checks run again on the frozen wording in task 3-9. §10.2: the three shipped altitude examples are exempt from the invented-example rule.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §10.3 Where the tuning goes (known weak spots; the chunked path; a release with nothing users meet); §3.2 When a release has one; §4.1 Judged against the previous release; §4.2 💥 Breaking; §7.1 mint's shared wording stays format-neutral; §10.1 The bar; §10.2 Tuning and acceptance; §10.4 Alongside the counts

## headline-first-release-notes-3-8

### Task 3-8: Frozen wording carries every rule and no sampled release

**Problem**: Shipping is decided on the wording as it ships, frozen when tuning ends (§10.2). Tuned on E1's releases, the wording could fit itself to them, which the generality rule guards against. Every tuning change also risks dropping a rule of §2–§6, or carrying one into mint's shared wording (§7.1, §10.2).

**Solution**: Freeze the wording when tuning ends: the default prompt, the closing reminder, and the chunk, piece and merge framings, as mint ships them. Check that it names, quotes or paraphrases no change from the 31 sampled releases, that every example beyond the three shipped altitude examples is invented, that the default prompt still carries every rule of §2–§6, and that the shared wording stays format-neutral. Every mint gate passes on it.

**Outcome**: One frozen wording, as mint ships it, carries every rule and nothing from a sampled release, with every mint gate passing. It is the wording every acceptance run uses.

**Acceptance Criteria**:
- [ ] Given the frozen wording, when it is read against E1's 31 sampled releases, no prompt text names, quotes or paraphrases a change from any of them, and every example in it other than the three shipped altitude examples is invented (§10.2)
- [ ] Given the frozen wording, when a release's notes are written in one call on the default prompt through an `ai_command` that records its stdin, the instructions carry every rule of §2–§6 as specified. That covers §2's format, section order, bullet rules (the three altitude examples as shipped, and "Combine closely-related changes into a single bullet." word for word) and each change listed once; §3's headline rules; §4's placement and 💥 Breaking rules; §5's limit to what users meet, maintenance line and truthfulness rules; and §6's ordering by weight and the Change Map as a coverage check (§10.2, §10.3)
- [ ] Given the frozen wording, every single-call, chunk, piece and merge prompt ends with exactly §7.1's reminder, and the reminder, the chunk and piece framings and the merge framing name neither a headline nor the maintenance line. On a full `[release].prompt` override, the instructions are its prompt file verbatim, and nothing mint adds asks for a headline (§7.1, §7.2, §8)
- [ ] Given the frozen wording and a release, the prompt an acceptance run sends for that release is the one mint composes for it: every acceptance run uses the wording as mint ships it (§10.2)
- [ ] Given a change to the wording after it is frozen, tuning has reopened: no acceptance pass or hold check run on the earlier wording counts for the changed one (§10.2)
- [ ] With the wording frozen, `go build ./...`, `gofmt -l .` (printing nothing), `go vet ./...`, `go test -race ./...` and `golangci-lint run` (0 issues) all pass, and mint's test suite spawns no real AI command: the acceptance pass and the hold checks run outside it (§10.2)

**Do**:
- The frozen wording is `DefaultPrompt` and `OutputReminder` in `internal/notes/prompt.go`, and `chunkPart.framing` and `mergeFraming` in `internal/notes/chunk.go` (§7.1, §8, §10.2).
- The 31 sampled releases are the rows of `sample.tsv` in E1's record (`.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/`), which are the audit's releases (`.workflows/headline-first-release-notes/imports/release-notes-audit.md`) (§1.4, §10.2).
- The three shipped altitude examples are the default prompt's "Single-keypress review gates — press y/n/e/r to decide, no Enter needed.", "Failures no longer print twice when stdout and stderr share a terminal." and "Pin the AI model with `ai_command = "claude -p --model sonnet"`." (§2.2).

**Context**:
> §10.2: tuning happens during implementation, on E1's releases. The wording's generality, together with a bar that asks for no worse than today rather than a tuned threshold, guards against wording fitted to the releases it is tested on. §7.1: every headline rule, and every rule of §2–§6, lives in the default prompt alone.
>
> §10.2 and its corrigenda: acceptance is one pass of the wording as it ships, frozen when tuning ends. A check cleared only on wording that tuning later changed has not been cleared. The hold checks run again on the frozen wording (task 3-9), and the acceptance pass runs on it (task 3-10).
>
> E1's harness is built into the mint module at the checked-out SHA through `go build -overlay` (`harness/build.sh` in E1's record). Its `current` arm composes with `notes.ComposePrompt(notes.DefaultPrompt, …)`, and its `new` arm reads instructions and a reminder from files (`harness/main.go`). mint's notes tests pin the default prompt's rules by substring (`TestDefaultPrompt_CarriesEveryRule`, `internal/notes/prompt_test.go`).

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §10.2 Tuning and acceptance; §7.1 mint's shared wording stays format-neutral; §7.2 What reaches which repo; §8 The Chunked Path; §2 The Notes Format; §3 The Headline; §4 Section Placement; §5 What the Notes Claim; §6 Ordering and the Change Map; §1.4 Evidence; Corrigenda (2026-10-22)

## headline-first-release-notes-3-9

### Task 3-9: Hold checks re-run on the frozen wording

**Problem**: §10.1 ships the rewritten prompt only once every failure §10.3 says holds shipping has been cleared, and what ships is the frozen wording (§10.2). The acceptance pass is a single-call pass, so it cannot exercise the part calls and the merge, or the chunked run of a release with nothing users meet. A check cleared on a tuning draft says nothing of wording that tuning later changed (§10.2, Corrigenda).

**Solution**: Run the chunked-path checks and the nothing-users-meet checks again on the frozen wording, on the releases the first runs used, cut as the first runs cut them, with the same AI command and the same judging. A failure reopens tuning, and the wording frozen after it gets its own acceptance pass and its own re-run checks on the same cuts.

**Outcome**: Every failure §10.3 says holds shipping is checked on the frozen wording itself and found absent, or tuning has reopened.

**Acceptance Criteria**:
- [ ] Given the frozen wording, when the releases the first runs took through the part calls and the merge run again, cut into the same parts: no part's notes carry a headline; no merged headline names anything other than the release's main work; no part returns an empty body and fails the run; and no merged notes carry a part's nothing-to-list line or the maintenance line while any part listed changes users meet (§8, §10.3)
- [ ] Given the frozen wording, when the release with nothing users meet runs again through a single call, and through the part calls and the merge cut as the first runs cut it, its notes carry no headline, no single call, part or merge returns an empty body and fails the run, and the merged notes do not keep the parts' lines in place of the maintenance line (§5.1, §10.3)
- [ ] In every run on the frozen wording, the acceptance pass and these re-run checks included, no single call writes the maintenance line for a release with a change users meet (§10.3)
- [ ] Given a hold check cleared only on a tuning draft, when tuning later changed the wording, the check counts as not cleared until it passes on the frozen wording (§10.2)
- [ ] Given a failure in any re-run check, tuning reopens. The wording frozen after it gets its own acceptance pass, and runs the chunked-path and nothing-users-meet checks again on the same cuts (§10.2)
- [ ] Given an outcome on the frozen wording that §10.3 says costs what today's notes cost (a merge that leaves out a headline §3.2 calls for, or a part or a nothing-users-meet release that lists its internal changes), it does not hold shipping (§10.3)

**Do**:
- The releases and their cuts are the ones the first runs kept (tasks 3-2 to 3-4), and the frozen wording is task 3-8's (§10.2).
- Every re-run check uses mint's shipped default `ai_command` (`config.DefaultAICommand`, `internal/config/config.go`), as E1 ran it, never a release repo's own. What it judges is judged as E1 judged: with the `rubric.md` and judges of E1's record (`.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/`), reading the release's repo and diff, plus the audit's account (`.workflows/headline-first-release-notes/imports/release-notes-audit.md`) where the release is one of the 31. That covers whether a merged headline names the release's main work, and whether a release, or a part's slice, holds a change users meet (§10.2).

**Context**:
> Corrigenda 2026-10-22: §10.2 runs the checks set in the first runs again on the frozen wording, and a failure there reopens tuning and a fresh acceptance pass. Every such check, first runs and frozen-wording runs alike, uses the acceptance pass's AI command and model, and is judged with E1's rubric and judges. §10.4 keeps the read by eye outside the bar, so it cannot judge a check that holds shipping.
>
> §10.3: today's notes have none of these failures, so each is a regression.
>
> mint's cutter sizes each part with the instructions and the framing counted (`cutter.fixedPart`, `internal/notes/chunk.go`). The frozen wording's length can differ from the drafts the first runs ran on, so the same `max_prompt_bytes` can cut a release into different parts.
>
> The acceptance pass on the frozen wording is task 3-10.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §10.2 Tuning and acceptance (acceptance; the re-run checks); §10.3 Where the tuning goes (the chunked path; a release with nothing users meet); §10.1 The bar; §8 The Chunked Path; §5.1 Only what users meet (a release with nothing users meet); §10.4 Alongside the counts; Corrigenda (2026-10-22)

## headline-first-release-notes-3-10

### Task 3-10: Acceptance pass clears every count on the frozen wording

**Problem**: The rewritten default prompt ships only when every §10.1 count is at least as good as today's prompt's, each on its own. The counts are taken on E1's 26 held-out releases with two repeats each, judged blind with E1's rubric and judges, and scored as E1 scored them (§10.1). One pass of the frozen wording generates bodies from one prompt, so nothing is blinded unless another prompt's bodies sit beside them (§10.2, Corrigenda).

**Solution**: Run the frozen wording once over the 26 held-out releases, two repeats each, as E1 ran it: E1's harness, the same inputs, the same AI command and model, E1's rubric and judge grouping, and E1's scoring. Judge each release's two bodies blind beside E1's two stored current-prompt bodies for that release, four per release under shuffled labels as E1's `harness/blind.py` lays them out. Score only the frozen wording's 52 bodies against §10.1's fixed bar column. A count that misses its line reopens tuning.

**Outcome**: One acceptance pass of the frozen wording shows every count at its line, or tuning has reopened for a new frozen wording and its own pass.

**Acceptance Criteria**:
- [ ] Given the frozen wording, the rewritten default prompt closed by §7.1's reminder, when the acceptance pass runs it once over E1's 26 held-out releases with two repeats each, it does so with E1's harness, the same inputs, and the same AI command and model, and yields 52 bodies (§10.2)
- [ ] Given those 52 bodies, when they are judged, each release's two frozen-wording bodies are judged beside E1's two stored current-prompt bodies for that release, four per release under shuffled labels as E1's `harness/blind.py` lays them out, with E1's rubric and judge grouping (§10.2)
- [ ] Given the judgements, only the frozen wording's bodies are scored, as E1 scored them, against §10.1's bar column, which stays fixed whatever the current-prompt bodies score in the new judging. Each count clears its own line: clean lead at least 21 of 52; wrong-subject headlines none; inversions at most 30; misfiled at most 84; duplicates at most 29; internal items at most 77; false claims at most 25; in-release-as-change at most 22; breaking changes flagged at least 1 of 10; missing items at most 9; format slips at most 5 (§10.1, §10.2)
- [ ] Given the scores, false claims and in-release-as-change each clear their own line, never netted as one class. For the headline, at least as good means no wrong-subject headline. E1's pre-registered thresholds and its successor-experiment branch do not gate shipping (§10.1)
- [ ] Given a body among the 52 that is the maintenance line, on a release with a change users meet, judged as E1 judged, it holds shipping until the wording clears it (§10.3)
- [ ] Given a count that misses its line, tuning reopens. The wording frozen after it gets its own acceptance pass and its own re-run hold checks on the same cuts, and the same frozen wording is never run again to clear a count (§10.2)
- [ ] Given the acceptance pass, the read by eye on real releases runs alongside the counts and outside the bar, and the pass runs outside mint's test suite (§10.4, §10.2)

**Do**:
- E1's record is `.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/`. Its harness is `harness/` (`build.sh`, `main.go`, `run.py`, `blind.py`, and the scoring in `score.py` and `secondary.py`), its rubric `rubric.md`, and its judge grouping the Judging section of `design.md`. The held-out releases are the `holdout` rows of `sample.tsv`, and the stored current-prompt bodies are the 52 under `data/bodies/holdout/current/` (§10.2).
- The AI command is mint's shipped default `ai_command` (`config.DefaultAICommand`, `internal/config/config.go`), as E1 ran it (§10.2).
- The counts are the ones §10.1 reads from E1's scored data: `.field_counts` in `data/secondary.json` (inversions, misfiled, duplicates, internal, false claims, in-release-as-change, missing, format slips), the lead's clean count and the breaking flagged count in the same file, and `.classes["lead and headline"].new_wrong_subject` in `data/scores.json` (§10.1).
- The frozen wording is task 3-8's (§10.2).

**Context**:
> §10.1, why this bar: the rules are decided, and E1 shows them moving the notes the right way on nearly every count. The judges preferred the new notes on 21 of 26 releases, and the user, reading blind, on 4 of 6. The one shipping outcome that would be wrong is a regression, and this bar rules out every regression while the remaining gap is wording.
>
> §10.2: the acceptance pass is a single-call pass, and the checks it cannot exercise run again on the frozen wording in task 3-9. Corrigenda 2026-10-22: a count that misses its line reopens tuning, as a failure §10.3 says holds shipping does, since a rerun of unchanged wording would clear a count by resampling, not by its wording. The bar column is today's measured counts, fixed in its table.
>
> E1's scripts write into E1's record: `run.py` writes bodies under `data/bodies/`, `blind.py` writes `data/lee-read.md`, and `score.py` and `secondary.py` write `data/scores.json` and `data/secondary.json`. Those files hold the bar column's measurement and the stored current-prompt bodies the judging reuses (§10.1, §10.2).
>
> §10.4: the counts weigh an error at the top of the notes the same as one at the bottom, which is why the read by eye stays alongside them.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §10.1 The bar; §10.2 Tuning and acceptance (acceptance); §10.3 Where the tuning goes (a release with nothing users meet); §10.4 Alongside the counts; §7.1 mint's shared wording stays format-neutral; Corrigenda (2026-10-22)
