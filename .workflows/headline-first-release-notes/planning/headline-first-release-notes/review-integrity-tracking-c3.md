# Review Tracking: Headline First Release Notes - Integrity

## Findings

### 1. The freeze never checks that the frozen framings still carry §8's instructions

**Severity**: Important
**Plan Reference**: Phase 3 Acceptance (the frozen-wording read criterion); headline-first-release-notes-3-8 (Solution; Acceptance Criteria) and its row in planning.md's Phase 3 task table
**Category**: Acceptance Criteria Quality
**Move**: settled
**Change Type**: add-to-task

**Problem**:
A release too large for one call could ship merged notes that list the same change once per part, or that let the Change Map set the order again, and nothing in Phase 3 would show it. Task 3-7 tunes the chunk, piece and merge framings after 3-2 and 3-3, the last tasks that check what the framings still ask for. The freeze, task 3-8, then checks the framings for one thing only: that they name no headline and no maintenance line. The acceptance pass is a single-call pass and never sees a merge. The hold re-runs in task 3-9 look only for the failures §10.3 says hold shipping: a headline in a part's notes, an empty part, and a part's line or the maintenance line in the wrong place. So if tuning rewords the merge framing so that it no longer drops duplicates, or brings back the Change Map ranking, that framing freezes and ships without anyone seeing it. That cuts against §10.2, which says the framings that ship follow §7.1 and §8, and against Phase 3's Goal, which says tuning changes wording only and the rules stay as specified.

**Proposal**:
Hold the frozen framings to the instructions §7.1 and §8 give them, as task 3-8 already holds the default prompt to §2–§6. Add one criterion to task 3-8, checked on the frozen wording. It restates what tasks 2-1 to 2-4 ask of the framings, and what 3-2 and 3-3 check on the tuned framings. Name the same check in 3-8's Solution, in its table row, and in Phase 3's frozen-wording criterion. Three things settle this, so there is only one answer:
- §10.2: the chunk, piece and merge framings that ship follow §7.1 and §8.
- §7.1: the merge framing keeps "combine related changes across parts into single bullets" and "drop duplicates", drops "rank by the Change Map above", and gains one instruction, for parts that held nothing.
- The plan's own convention: the frozen wording is checked for every rule it has to carry.

**Current**:

*Phase 3 Acceptance (planning.md), the frozen-wording read criterion:*

- [ ] Given the frozen wording, when it is read, then no prompt text names, quotes or paraphrases a change from the 31 sampled releases, every example other than the three shipped altitude examples is invented, and every rule of §2–§6 is still carried (§10.2, §10.3).

*planning.md, Phase 3 task table, headline-first-release-notes-3-8 row:*

| headline-first-release-notes-3-8 | Frozen wording carries every rule and no sampled release | no prompt text names/quotes/paraphrases a change from the 31 sampled releases (§10.2), every example other than the three shipped altitude examples is invented (§10.2), every rule of §2–§6 still carried (§10.2 and §10.3), the closing reminder is §7.1's exact sentence and the shared wording names neither a headline nor the maintenance line (§7.1), the frozen wording is what ships and what every acceptance run uses (§10.2), every mint gate passes and the test suite spawns no real AI command with the acceptance runs outside it (§10.2) |

*headline-first-release-notes-3-8, Solution:*

**Solution**: Freeze the wording when tuning ends: the default prompt, the closing reminder, and the chunk, piece and merge framings, as mint ships them. Check that it names, quotes or paraphrases no change from the 31 sampled releases, that every example beyond the three shipped altitude examples is invented, that the default prompt still carries every rule of §2–§6, and that the shared wording stays format-neutral. Every mint gate passes on it.

*headline-first-release-notes-3-8, Acceptance Criteria, third criterion (the new criterion goes directly after it):*

- [ ] Given the frozen wording, every single-call, chunk, piece and merge prompt ends with exactly §7.1's reminder, and the reminder, the chunk and piece framings and the merge framing name neither a headline nor the maintenance line. On a full `[release].prompt` override, the instructions are its prompt file verbatim, and nothing mint adds asks for a headline (§7.1, §7.2, §8)

**Proposed Text**:

*Phase 3 Acceptance (planning.md), the frozen-wording read criterion:*

- [ ] Given the frozen wording, when it is read, then no prompt text names, quotes or paraphrases a change from the 31 sampled releases, every example other than the three shipped altitude examples is invented, and every rule of §2–§6 is still carried. The chunk and piece framings and the merge framing still carry the instructions §7.1 and §8 give them, and the merge framing still does not rank by the Change Map (§6.2, §7.1, §8, §10.2, §10.3).

*planning.md, Phase 3 task table, headline-first-release-notes-3-8 row:*

| headline-first-release-notes-3-8 | Frozen wording carries every rule and no sampled release | no prompt text names/quotes/paraphrases a change from the 31 sampled releases (§10.2), every example other than the three shipped altitude examples is invented (§10.2), every rule of §2–§6 still carried (§10.2 and §10.3), the chunk and piece framings still leave the release-wide summary to the merge and ask a part with nothing to list for its one line (§8 and §10.2), the merge framing still combines related changes across parts and drops duplicates, leaves the parts' lines out, writes the governing format's nothing-to-list body only when every part returned one, does not rank by the Change Map and gains no other instruction (§6.2, §7.1, §8 and §10.2), the closing reminder is §7.1's exact sentence and the shared wording names neither a headline nor the maintenance line (§7.1), the frozen wording is what ships and what every acceptance run uses (§10.2), every mint gate passes and the test suite spawns no real AI command with the acceptance runs outside it (§10.2) |

*headline-first-release-notes-3-8, Solution:*

**Solution**: Freeze the wording when tuning ends: the default prompt, the closing reminder, and the chunk, piece and merge framings, as mint ships them. Check that it names, quotes or paraphrases no change from the 31 sampled releases, that every example beyond the three shipped altitude examples is invented, that the default prompt still carries every rule of §2–§6, that the chunk, piece and merge framings still carry the instructions §7.1 and §8 give them, and that the shared wording stays format-neutral. Every mint gate passes on it.

*headline-first-release-notes-3-8, Acceptance Criteria, the third criterion unchanged and the new criterion directly after it:*

- [ ] Given the frozen wording, every single-call, chunk, piece and merge prompt ends with exactly §7.1's reminder, and the reminder, the chunk and piece framings and the merge framing name neither a headline nor the maintenance line. On a full `[release].prompt` override, the instructions are its prompt file verbatim, and nothing mint adds asks for a headline (§7.1, §7.2, §8)
- [ ] Given the frozen wording, when a release's notes are written in parts through an `ai_command` that records its stdin, every chunk and piece framing still names its part and asks for release notes for the changes in it, in the format above, to be merged with the other parts' notes. It asks the part to leave out anything that sums up the release as a whole, because the merge writes that, and asks a part whose slice holds nothing the notes would list to return one line saying so. The merge framing still asks for one set of release notes for the whole release, in the format above, combining related changes across parts into single bullets and dropping duplicates. It asks for the parts' nothing-to-list lines to be left out and, only where every part returned one, for what the governing format gives a release with nothing to list. It does not say to rank by the Change Map, and it gains no other instruction (§6.2, §7.1, §8, §10.2)

**Resolution**: Fixed
**Notes**: Applied as proposed under the auto finding gate; planning.md's Phase 3 Acceptance and 3-8 row, and the 3-8 tick record, updated and verified.

---

## Observations

- Tasks 3-2 to 3-4 take their first runs "given the wording from Phases 1–2", but task 3-1 runs before them and may already have tuned the default prompt. Whichever wording those first runs use, the re-runs on the frozen wording (task 3-9) decide.
- Phase 3's Acceptance criterion for re-running the release with nothing users meet names a single call or merge that returns an empty body, but not a part. Task 3-9's second criterion and the chunked-path criterion both cover the part.
