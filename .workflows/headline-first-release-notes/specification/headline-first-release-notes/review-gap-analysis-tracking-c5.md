# Review Tracking: Headline First Release Notes - Gap Analysis

## Findings

### 1. A false maintenance line from a single call is only counted, never held

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Move**: settled
**Priority**: Important
**Affects**: §10.3 Where the tuning goes (the "A release with nothing users meet" bullet), which §10.1's ship condition reaches; the cost of a false line is stated in §5.1 (a release with nothing users meet); the merged-notes hold on the same failure sits in §10.3's chunked-path bullet

**Problem**:
The new prompt can write the one-line maintenance body ("no user-facing changes") for a release that does have a change users meet, such as a mostly-tooling release that carries one small fix. Users of that release then read on the tag, the `CHANGELOG.md` section and the provider release that nothing changed for them, and the fix they were waiting for is never listed. On an unattended `mint release -y` nobody sees the notes before they ship. The spec counts this as the worst error the maintenance line can make, and it holds shipping when merged notes carry the line beside parts that listed real changes. When a single call writes it, nothing holds shipping, and every body the bar measures is written by a single call. The bar's counts come from E1's rubric, which had no maintenance line. So a false line shows up only as that release's missing items, netted against every other missing item across the 52 bodies. On a release with one or two changes, a false line costs the same as dropping one or two bullets. Whoever runs acceptance can find a false maintenance line in the bodies and still ship, because the counts clear and no listed hold names it.

**Proposal**:
A single call that writes the maintenance line for a release with a change users meet holds shipping until the wording clears it, in any run including the acceptance pass, as the same failure already does in merged notes. What determined it: the bar's own reading of "at least as good" for something today's notes never contain. Today's notes have no headline, so the headline's line is no wrong-subject headline. Today's notes never write a maintenance line, which is the spec's stated ground for the merged-notes hold, so "at least as good" means no false one. The spec also gives a false line the same kind of harm the zero line on headlines guards against: it misstates the release in its only line, and it drops a change users meet. The bar is meant to rule out every regression, and a count that nets this against single dropped bullets does not rule it out.

Two alternatives also fit:
- A "False maintenance line: none" row in the bar's table has the same effect. Against it: the table holds E1's measured counts, and E1 has no figure for this.
- Leaving it to the counts, as false breaks are left to the misfiled count, is the other option. Against it: a false break misstates one line among the sections, while a false maintenance line replaces every section.

**Current**:
- **A release with nothing users meet** gets a body new-v3 never wrote (§10.2). The first runs also take at least one such release through a single call, and through the part calls and the merge with every part's slice holding nothing users meet, and check that the notes are §5.1's maintenance line. A headline on that release, or a single call or merge that returns an empty body and fails the run, holds shipping until the wording clears it: today's notes have neither. Notes that list the release's internal changes instead cost what today's notes cost, and are tuned like the known weak spots below.

**Proposed Text**:
- **A release with nothing users meet** gets a body new-v3 never wrote (§10.2). The first runs also take at least one such release through a single call, and through the part calls and the merge with every part's slice holding nothing users meet, and check that the notes are §5.1's maintenance line. A headline on that release, or a single call or merge that returns an empty body and fails the run, holds shipping until the wording clears it: today's notes have neither. Notes that list the release's internal changes instead cost what today's notes cost, and are tuned like the known weak spots below. A single call that writes the maintenance line for a release with a change users meet also holds shipping until the wording clears it, in any run including the acceptance pass, as it does in merged notes: today's notes never write that line, and the counts would weigh it only by the changes it drops.

**Resolution**: Pending
**Notes**:

---

## Observations

- §10.1 and §10.2 both say how the counts are taken. §10.1 names E1's rubric, judges and scoring, and §10.2 adds E1's harness, inputs, AI command and model. The two agree today, and §10.2 is the fuller statement.
- "Clean lead" and "wrong-subject headline" get their meaning from E1's rubric. The spec defines only the second, and only on the chunked path, as a headline that names something other than the release's main work.
