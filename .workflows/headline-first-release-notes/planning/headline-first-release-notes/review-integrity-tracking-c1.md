# Review Tracking: Headline First Release Notes - Integrity

## Findings

### 1. The nothing-users-meet checks make an accepted outcome a pass/fail criterion

**Severity**: Important
**Plan Reference**: Phase 3, headline-first-release-notes-3-3 (Outcome; Acceptance Criteria 2 and 3) and headline-first-release-notes-3-4 (Outcome; Acceptance Criteria 1 and 2)
**Category**: Acceptance Criteria Quality
**Move**: settled
**Change Type**: update-task

**Problem**:
Tasks 3-3 and 3-4 would hold the freeze, and with it shipping, on an outcome the specification says does not hold shipping. Task 3-4 requires the nothing-users-meet release's notes to be the maintenance line on both paths, and task 3-3 requires the part to return its one line. In each task the fourth criterion, like §10.3, says notes or parts that list their internal changes instead cost what today's notes cost, are tuned like the known weak spots, and do not hold shipping. Suppose the tuned wording still lists that release's internal changes. The implementer then cannot close 3-4 against its own criteria. They either keep tuning and hold up 3-8's freeze for an outcome the specification already accepts, or they close the task with two criteria unmet. Task 3-4's first criterion also pins the outcome to "the wording from Phases 1–2". A first run that fails there leaves the criterion false for good, even after tuning clears the failure. §10.3's own wording is a check ("check that the notes are §5.1's maintenance line"), whose hold-shipping failures are a headline, an empty body that fails the run, and the parts' lines kept in place of the maintenance line. Phase 3's Acceptance already uses that check form.

**Proposal**:
Phrase these criteria as §10.3 does: the runs check for the maintenance line and the part's one line, and only the outcomes §10.3 lists hold shipping. The fourth criterion of each task stays as it is and carries the internal-changes outcome. Each Outcome states the hold-shipping failures cleared on the tuned wording and the internal-changes outcome tuned, not held. §10.3 fixes which outcomes hold shipping and which are tuned, so the record allows only this answer.

**Current**:

*headline-first-release-notes-3-3, Outcome:*

**Outcome**: On the tuned wording, the part returns its one line, and the merged notes carry neither that line nor the maintenance line. The release is kept with the parts it was cut into.

*headline-first-release-notes-3-3, Acceptance Criteria 2 and 3:*

- [ ] Given that part, it returns one line saying its part holds nothing for the notes, never an empty body. A part that returns an empty body and fails the run holds shipping until the wording clears it (§8, §10.3)
- [ ] Given the same release's merge, the merged notes leave the part's line out beside the other parts' sections and do not carry the maintenance line. Merged notes that carry a part's line or the maintenance line while any part listed changes users meet hold shipping until the wording clears it (§5.1, §8, §10.3)

*headline-first-release-notes-3-4, Outcome:*

**Outcome**: On the tuned wording, that release's notes are the maintenance line on both paths, and the release is kept with its cut for the checks on the frozen wording.

*headline-first-release-notes-3-4, Acceptance Criteria 1 and 2:*

- [ ] Given the wording from Phases 1–2 and at least one release whose every change is one §5.1 leaves out, when the first runs take it through a single call, its notes are one plain line in the shape of `Maintenance release — no notable source changes`, with no headline and no sections (§5.1, §10.3)
- [ ] Given the same release taken through the part calls and the merge, cut so that every part's slice holds nothing users meet, every part returns its one line and the merged notes are §5.1's maintenance line (§5.1, §8, §10.3)

**Proposed Text**:

*headline-first-release-notes-3-3, Outcome:*

**Outcome**: The runs show whether that part returns its one line and whether the merge leaves it out. On the tuned wording, no part returns an empty body that fails the run, and the merged notes carry neither a part's line nor the maintenance line while any part listed changes users meet. A part that still lists its internal changes in place of its line is tuned like the known weak spots and does not hold shipping. The release is kept with the parts it was cut into.

*headline-first-release-notes-3-3, Acceptance Criteria 2 and 3:*

- [ ] Given that part, the runs check that it returns one line saying its part holds nothing for the notes rather than an empty body. A part that returns an empty body and fails the run holds shipping until the wording clears it (§8, §10.3)
- [ ] Given the same release's merge, the runs check that the merged notes leave the part's line out beside the other parts' sections and do not carry the maintenance line. Merged notes that carry a part's line or the maintenance line while any part listed changes users meet hold shipping until the wording clears it (§5.1, §8, §10.3)

*headline-first-release-notes-3-4, Outcome:*

**Outcome**: The runs show, on both paths, whether that release's notes are the maintenance line. On the tuned wording, none of the outcomes that hold shipping remains: no headline on that release, no single call, part or merge that returns an empty body and fails the run, and no merged notes that keep the parts' lines in place of the maintenance line. Notes that still list the release's internal changes are tuned like the known weak spots and do not hold shipping. The release is kept with its cut for the checks on the frozen wording.

*headline-first-release-notes-3-4, Acceptance Criteria 1 and 2:*

- [ ] Given the wording from Phases 1–2 and at least one release whose every change is one §5.1 leaves out, when the first runs take it through a single call, the runs check that its notes are one plain line in the shape of `Maintenance release — no notable source changes`, with no headline and no sections (§5.1, §10.3)
- [ ] Given the same release taken through the part calls and the merge, cut so that every part's slice holds nothing users meet, the runs check that every part returns its one line and that the merged notes are §5.1's maintenance line (§5.1, §8, §10.3)

**Resolution**: Fixed
**Notes**: Applied as proposed under the auto finding gate; tick records for 3-3 and 3-4 updated and verified.

---

### 2. The hold-check task asks for a check only the later acceptance pass can make

**Severity**: Important
**Plan Reference**: Phase 3, headline-first-release-notes-3-9 (Outcome; Acceptance Criteria; Context) and its row in planning.md's Phase 3 task table
**Category**: Dependencies and Ordering
**Move**: settled
**Change Type**: remove-from-task

**Problem**:
Task 3-9 runs before 3-10 in natural order, but its third criterion can only be checked by 3-10's acceptance pass. In 3-9's own runs, the only single call is on the release with nothing users meet. So "no single call writes the maintenance line for a release with a change users meet" can only fail among the acceptance pass's 52 bodies. To close 3-9, the implementer has two options. They can run the acceptance pass inside 3-9, and then 3-10 becomes a second pass on the same frozen wording, which §10.2 and the plan both rule out ("the same frozen wording is never run again to clear a count"). Or they can close 3-9 with the criterion unchecked. Task 3-10's fifth criterion already carries this check on the 52 bodies.

**Proposal**:
Remove the third criterion from 3-9 and narrow its Outcome to what its runs cover: the chunked path and the release with nothing users meet, the checks §10.2 says a single-call pass cannot exercise. Add a pointer in its Context to 3-10, which owns the single-call check. Drop the same clause from 3-9's Edge Cases cell in planning.md. Phase 3's Acceptance keeps its rule-form criterion, since that one spans both tasks. Two things settle this: 3-10 already holds the check, and none of 3-9's runs contains a single call on a release with a change users meet.

**Current**:

*headline-first-release-notes-3-9, Outcome:*

**Outcome**: Every failure §10.3 says holds shipping is checked on the frozen wording itself and found absent, or tuning has reopened.

*headline-first-release-notes-3-9, Acceptance Criteria:*

- [ ] Given the frozen wording, when the releases the first runs took through the part calls and the merge run again, cut into the same parts: no part's notes carry a headline; no merged headline names anything other than the release's main work; no part returns an empty body and fails the run; and no merged notes carry a part's nothing-to-list line or the maintenance line while any part listed changes users meet (§8, §10.3)
- [ ] Given the frozen wording, when the release with nothing users meet runs again through a single call, and through the part calls and the merge cut as the first runs cut it, its notes carry no headline, no single call, part or merge returns an empty body and fails the run, and the merged notes do not keep the parts' lines in place of the maintenance line (§5.1, §10.3)
- [ ] In every run on the frozen wording, the acceptance pass and these re-run checks included, no single call writes the maintenance line for a release with a change users meet (§10.3)
- [ ] Given a hold check cleared only on a tuning draft, when tuning later changed the wording, the check counts as not cleared until it passes on the frozen wording (§10.2)
- [ ] Given a failure in any re-run check, tuning reopens. The wording frozen after it gets its own acceptance pass, and runs the chunked-path and nothing-users-meet checks again on the same cuts (§10.2)
- [ ] Given an outcome on the frozen wording that §10.3 says costs what today's notes cost (a merge that leaves out a headline §3.2 calls for, or a part or a nothing-users-meet release that lists its internal changes), it does not hold shipping (§10.3)

*headline-first-release-notes-3-9, Context, last paragraph:*

> The acceptance pass on the frozen wording is task 3-10.

*planning.md, Phase 3 task table, headline-first-release-notes-3-9 row:*

| headline-first-release-notes-3-9 | Hold checks re-run on the frozen wording | the same releases cut as the first runs cut them (§10.2), no part's notes carry a headline (§10.3), no merged headline names anything other than the release's main work (§10.3), no part returns an empty body and fails the run (§10.3), no merged notes carry a part's line or the maintenance line while any part listed changes users meet (§10.3), the nothing-users-meet release through a single call and through the part calls and merge gets no headline/no empty body/no parts' lines in place of the maintenance line (§10.3), no single call writes the maintenance line for a release with a change users meet (§10.3), a check passed only on wording that tuning later changed counts as not cleared (§10.2 and Corrigenda), a failure reopens tuning and the wording frozen after it gets its own acceptance pass and re-run checks on the same cuts (§10.2 and Corrigenda) |

**Proposed Text**:

*headline-first-release-notes-3-9, Outcome:*

**Outcome**: Every failure §10.3 says holds shipping on the chunked path and on the release with nothing users meet is checked on the frozen wording itself and found absent, or tuning has reopened.

*headline-first-release-notes-3-9, Acceptance Criteria:*

- [ ] Given the frozen wording, when the releases the first runs took through the part calls and the merge run again, cut into the same parts: no part's notes carry a headline; no merged headline names anything other than the release's main work; no part returns an empty body and fails the run; and no merged notes carry a part's nothing-to-list line or the maintenance line while any part listed changes users meet (§8, §10.3)
- [ ] Given the frozen wording, when the release with nothing users meet runs again through a single call, and through the part calls and the merge cut as the first runs cut it, its notes carry no headline, no single call, part or merge returns an empty body and fails the run, and the merged notes do not keep the parts' lines in place of the maintenance line (§5.1, §10.3)
- [ ] Given a hold check cleared only on a tuning draft, when tuning later changed the wording, the check counts as not cleared until it passes on the frozen wording (§10.2)
- [ ] Given a failure in any re-run check, tuning reopens. The wording frozen after it gets its own acceptance pass, and runs the chunked-path and nothing-users-meet checks again on the same cuts (§10.2)
- [ ] Given an outcome on the frozen wording that §10.3 says costs what today's notes cost (a merge that leaves out a headline §3.2 calls for, or a part or a nothing-users-meet release that lists its internal changes), it does not hold shipping (§10.3)

*headline-first-release-notes-3-9, Context, last paragraph:*

> The acceptance pass on the frozen wording is task 3-10. It checks that no single call among its 52 bodies writes the maintenance line for a release with a change users meet, the one single-call failure §10.3 says holds shipping in any run.

*planning.md, Phase 3 task table, headline-first-release-notes-3-9 row:*

| headline-first-release-notes-3-9 | Hold checks re-run on the frozen wording | the same releases cut as the first runs cut them (§10.2), no part's notes carry a headline (§10.3), no merged headline names anything other than the release's main work (§10.3), no part returns an empty body and fails the run (§10.3), no merged notes carry a part's line or the maintenance line while any part listed changes users meet (§10.3), the nothing-users-meet release through a single call and through the part calls and merge gets no headline/no empty body/no parts' lines in place of the maintenance line (§10.3), a check passed only on wording that tuning later changed counts as not cleared (§10.2 and Corrigenda), a failure reopens tuning and the wording frozen after it gets its own acceptance pass and re-run checks on the same cuts (§10.2 and Corrigenda) |

**Resolution**: Pending
**Notes**:

---

### 3. The no-new-inputs criterion leaves the framings out of the chunk and merge calls

**Severity**: Important
**Plan Reference**: Phase 3, headline-first-release-notes-3-5 (Acceptance Criteria 2)
**Category**: Acceptance Criteria Quality
**Move**: settled
**Change Type**: update-task

**Problem**:
Task 3-5 guards against new inputs by listing what every notes call carries, and the list omits the framings. Read as written, a chunk or piece call carries only the instructions, the Change Map, a slice and the reminder, and the merge call only the instructions, the Change Map, the part notes and the reminder. Tasks 2-1 and 2-2 put a framing in each of those calls, and Phase 3 tunes those framings. A tester who writes the criterion as a test asserts calls the code cannot produce, and the test fails. Alternatively, the tester reads the framing as the "new input" the criterion forbids. Either way the plan contradicts itself on what a chunk or merge call holds.

**Proposal**:
Name each call's framing among the call's own material, as §8 and tasks 2-1 and 2-2 describe the calls. The criterion's purpose, no commit history and no new input, stays as it is.

**Current**:

- [ ] Given the tuned wording, every notes call still carries only the resolved instructions, the Change Map, the call's own material (the diff, one part's slice, or every part's notes for the merge) and the closing reminder: no commit history and no new input (§1.2, §5.2)

**Proposed Text**:

- [ ] Given the tuned wording, every notes call still carries only the resolved instructions, the Change Map, the call's own material (the diff for a single call; the framing that names its part and that part's slice for a chunk or piece call; the merge framing and every part's notes for the merge call) and the closing reminder: no commit history and no new input (§1.2, §5.2, §8)

**Resolution**: Pending
**Notes**:

---

## Observations

- If run as-is, E1's `score.py` and `secondary.py` overwrite `data/scores.json` and `data/secondary.json`, which hold the measurement behind §10.1's bar column, and `blind.py` overwrites `data/lee-read.md`. Task 3-10's Context names this, and where the new pass writes its outputs is the builder's call.
- Tasks 3-1 and 3-6 say certain evidence "reopens" §7.1's reminder decision or §2.2's combine rule "rather than being tuned around". Neither says the implementer stops and puts the reopened decision to Lee, though that follows from the decision being the specification's.
