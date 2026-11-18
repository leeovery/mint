# Review Tracking: Headline First Release Notes - Input Review

## Findings

### 1. The maintenance line has no rule for when the model is unsure

**Source**: discussion, Summary → Key Insights 5 ("What sits at the top of the notes costs most … Every rule that puts something at the top — the headline, the Breaking section — is biased to omission when unsure"); A Release With Nothing Users Meet → Decision ("notes that list the release's internal changes instead cost what today's notes cost") and Journey (the merge drops a part's line beside other parts' sections); No Clear Headline → Decision (When in doubt, leave it out: a missing headline costs no more than today's notes, a wrong one misstates the release in its most-read line, and on an unattended run nobody catches it). A Release With Nothing Users Meet was decided after the Key Insights were written, and the omission bias was never carried to it.
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §5.1 Only what users meet (the "A release with nothing users meet" bullet)

**Problem**:
The maintenance line is the one body in this feature that says something about the whole release: "no user-facing changes". On a release that is mostly tests, CI or tooling but carries one small change users meet, such as a fix or a speed-up they would notice, nothing tells the model which way to lean when it can't tell whether that change counts. If it writes the maintenance line, the tag, the `CHANGELOG.md` section and the provider release say in their only line that nothing changed for users, and the change they meet is never listed. Agent sessions run `mint release -y`, so nobody reads the notes before they ship. The user who was waiting for the fix reads notes that say there isn't one. On the chunked path it is worse and harder to see. A part that returns its nothing-line for a slice holding such a change loses that change without trace, because the merge drops the line beside other parts' sections and the merged notes look complete. The headline and the 💥 Breaking section each lean to omission when unsure. This line, which also stands at the top of the notes, does not.

**Proposal**:
When the model is unsure whether a change is one users meet, it lists the change in its section, and neither the release's maintenance line nor a part's nothing-line is written. Disposed as the session's call: it applies the discussion's cross-cutting principle (Key Insight 5: every rule that puts something at the top of the notes is biased to omission when unsure) to ground A Release With Nothing Users Meet never covered, so it lands in that section first. What leaned: the maintenance line is the whole body; listing an uncertain change costs what today's notes cost, while a false line drops a change users meet and misstates the release in its only line. The alternative, no doubt rule, leaves the worst-ranked misstatement to the model's threshold. The clause applies only where the line is in play, so it moves nothing in the user-facing filter or E1's internal-items count. Proposed Text trimmed at dispose to the rule and its one-line reason.

**Current**:
- **A release with nothing users meet.** Where every change in the release is one this section leaves out, the notes are one plain line saying the release is maintenance with no user-facing changes, in the shape of the empty-diff stub (`Maintenance release — no notable source changes`), with no headline and no sections. On the chunked path only the merge can say this of the release: a part whose slice holds nothing users meet returns one line saying its part holds nothing for the notes, and the merge writes the maintenance line only when every part returned one (§8). The notes are never empty: the transport rejects an empty body as a failed call.

**Proposed Text**:
- **A release with nothing users meet.** Where every change in the release is one this section leaves out, the notes are one plain line saying the release is maintenance with no user-facing changes, in the shape of the empty-diff stub (`Maintenance release — no notable source changes`), with no headline and no sections. On the chunked path only the merge can say this of the release: a part whose slice holds nothing users meet returns one line saying its part holds nothing for the notes, and the merge writes the maintenance line only when every part returned one (§8). When unsure whether a change is one users meet, the change is listed in its section and neither line is written: a line left out costs what today's notes cost, while a false one drops a change users meet and tells every reader, in the release's only line, that nothing changed for them. The notes are never empty: the transport rejects an empty body as a failed call.

**Resolution**: Routed
**Notes**: Landed in the discussion's A Release With Nothing Users Meet decision (written this sitting), then in §5.1.

---

### 2. The breaking bullet's remedy rule departs from the decided rule

**Source**: No source decides this. Checked: the discussion's Breaking Changes (Options: E1's tested rule, "the one sentence says what breaks and what to do"; Journey: Portal v0.11.1 "flagged on both repeats with the remedy named"; Decision: "Each bullet says in its one sentence what breaks and what to do"), True To The Release (Decision: the three accuracy rules), Picking The Headline (the diff the sole source of truth), and the Summary. The rule first appears in an earlier review cycle's tracking file, which is not a source.
**Category**: Unsourced decision
**Move**: route
**Affects**: §4.2 💥 Breaking (the "What breaks, and what to do" bullet); §10.2 Tuning and acceptance (the third listed difference from new-v3)

**Problem**:
The specification makes the remedy in a breaking bullet conditional: "What to do comes only from what the diff shows, such as a new name, a replacement, or a setting that keeps the old behaviour. Where the diff shows none, the bullet says what breaks and stops there." It also lists this as one of the places where what ships departs from E1's measured wording. The discussion decides the rule without a condition: each bullet says what breaks and what to do. Upgrading users meet the difference in the opening lines of the notes. Take a command removed with nothing in the diff to replace it. Under the specification's rule the bullet names the removal and gives no step. Under the discussion's rule every bullet carries a step, and on that break the model can only supply one from outside the diff. The two records now disagree about what the most-read lines of a breaking release say. Anyone who next revisits the Breaking rule from the discussion, whether a later prompt revision or the agent-authored release notes item, restores the unconditional rule, and the invented steps come back with it.

**Proposed Text**:

**Resolution**: Routed
**Notes**: Settled by derivation from True To The Release and the section's own asymmetry; landed as a dated revision of the discussion's Breaking Changes decision. §4.2 and §10.2 already carry the same rule, so the specification needed no edit.

---

## Observations

- A failed chunked run whose finished parts all returned their nothing-line saves a file with nothing left after tidying. The operator then writes the maintenance line by hand to release it. This is rare: a release large enough to chunk that has nothing users meet, and whose merge also failed.
