# Review Tracking: Headline First Release Notes - Integrity

## Findings

### 1. A failed acceptance pass or hold re-run lets Phase 3 close with the bar not cleared

**Severity**: Important
**Plan Reference**: Phase 3 Acceptance (the reopening criterion); headline-first-release-notes-3-9 (Outcome); headline-first-release-notes-3-10 (Outcome; Acceptance Criteria 6)
**Category**: Task Template Compliance
**Move**: settled
**Change Type**: update-task

**Problem**:
Tasks 3-9 and 3-10 can each close with tuning reopened, and nothing in the plan says who does the reopened round. Suppose the acceptance pass misses a line: false claims at 27 against at most 25. Task 3-10's Outcome counts "tuning has reopened" as success, so the implementer closes it. Every Phase 3 task is then done, the task loop finds nothing left, and implementation concludes. The rewritten default prompt has been in `DefaultPrompt` since Phase 1, so it ships with a count worse than today's. §10.1's bar exists to rule out that regression, and Phase 3's own Goal and Acceptance say the bar is cleared on the frozen wording.

Task 3-9 has the same gap. If it closes on a failed re-run check, it hands 3-10 a wording already known to hold shipping, and no open task re-freezes the next wording or re-checks it.

Phase 3's Acceptance also names only two triggers for reopening: a count in the acceptance pass, and a hold failure in a re-run check. It leaves out the one hold failure the acceptance pass itself can show, a maintenance line on a release with a change users meet (task 3-10's fifth criterion). §10.2 reopens tuning for that failure as for any other on the frozen wording.

**Proposal**:
Each task that checks the frozen wording stays open through the rounds a failure reopens, and closes only once one frozen wording clears:

- Task 3-9 closes once a frozen wording clears every re-run check.
- Task 3-10 closes once a pass on such a wording clears every count, with no body that holds shipping.
- A re-frozen wording is held to task 3-8's checks, as Phase 3's Acceptance holds every frozen wording to them.
- Phase 3's reopening criterion names the acceptance pass's own hold failure, in §10.2's terms.

The record settles this in three places:

- §10.1 ships the prompt only when every count clears and every failure that holds shipping has been cleared.
- The corrigendum on a missed count rejected ending the attempt. It has the pass run again on the wording frozen after it.
- §10.2 reopens tuning on any failure on the frozen wording.

Nobody can know ahead how many rounds there will be. So the closing condition goes on the tasks that already own the frozen-wording checks, not on a task per round.

**Current**:

*Phase 3 Acceptance (planning.md), the reopening criterion:*

- [ ] Given a failure on the frozen wording, whether a count that misses its line in the acceptance pass or a failure §10.3 says holds shipping in a re-run check, when it is found, then tuning reopens. The wording frozen after it gets its own acceptance pass and runs the chunked-path and nothing-users-meet checks again on the same cuts. The same frozen wording is never run again to clear a count, and a check passed only on wording that tuning later changed counts as not cleared (§10.2).

*headline-first-release-notes-3-9, Outcome:*

**Outcome**: Every failure §10.3 says holds shipping on the chunked path and on the release with nothing users meet is checked on the frozen wording itself and found absent, or tuning has reopened.

*headline-first-release-notes-3-10, Outcome:*

**Outcome**: One acceptance pass of the frozen wording shows every count at its line, or tuning has reopened for a new frozen wording and its own pass.

*headline-first-release-notes-3-10, Acceptance Criteria 6:*

- [ ] Given a count that misses its line, tuning reopens. The wording frozen after it gets its own acceptance pass and its own re-run hold checks on the same cuts, and the same frozen wording is never run again to clear a count (§10.2)

**Proposed Text**:

*Phase 3 Acceptance (planning.md), the reopening criterion:*

- [ ] Given a failure on the frozen wording, whether a count that misses its line in the acceptance pass or a failure §10.3 says holds shipping, in the acceptance pass or in a re-run check, when it is found, then tuning reopens. The wording frozen after it gets its own acceptance pass and runs the chunked-path and nothing-users-meet checks again on the same cuts. The same frozen wording is never run again to clear a count, and a check passed only on wording that tuning later changed counts as not cleared. Phase 3 closes only on one frozen wording that clears every count and every check (§10.2).

*headline-first-release-notes-3-9, Outcome:*

**Outcome**: Every failure §10.3 says holds shipping on the chunked path and on the release with nothing users meet is checked on the frozen wording itself and found absent. A failure reopens tuning, and the task stays open until a wording frozen after it, held to task 3-8's checks, clears every re-run check on the same cuts. That wording is the one the acceptance pass (task 3-10) runs on.

*headline-first-release-notes-3-10, Outcome:*

**Outcome**: An acceptance pass on the frozen wording shows every count at its line, and no body among the 52 holds shipping. A pass that falls short reopens tuning, and the task stays open: the wording frozen after it is held to task 3-8's checks, runs task 3-9's hold checks again on the same cuts, and gets its own pass. The task closes only on a pass that clears, on a wording whose re-run hold checks have all cleared.

*headline-first-release-notes-3-10, Acceptance Criteria 6:*

- [ ] Given a count that misses its line, or a body among the 52 that holds shipping, tuning reopens. The wording frozen after it gets its own acceptance pass and its own re-run hold checks on the same cuts, and the same frozen wording is never run again to clear a count. The task does not close on a pass that falls short (§10.2)

**Resolution**: Pending
**Notes**:

---

## Observations

- Tasks 3-5 and 3-6 each judge "the wording tuning hands to the freeze", but 3-6 and 3-7 go on tuning after them. Task 3-7's every-count criterion is the one that holds at the hand-off.
- Phase 2's first Acceptance criterion says that on the default prompt "a change in doubt in a part's slice is listed", which reads as model output in a run that only records stdin. Task 2-3 states it as what the instructions say, which is what that run can check.
