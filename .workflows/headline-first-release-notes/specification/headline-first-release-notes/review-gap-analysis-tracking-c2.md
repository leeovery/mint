# Review Tracking: Headline First Release Notes - Gap Analysis

## Findings

### 1. The ship condition names the counts alone

**Source**: Specification analysis
**Category**: Contradiction
**Move**: settled
**Priority**: Important
**Affects**: §10.1 The bar (its opening sentence); §10.3 Where the tuning goes (the chunked-path bullet)

**Problem**:
The bar says the rewritten prompt ships when every count is at least as good as today's, and those counts come from 52 single-pass bodies. The chunked-path checks name more failures that hold shipping: a headline in a part's notes, a merged headline about something other than the release's main work, a part's line or the maintenance line kept in merged notes beside real changes, and a part that returns an empty body and fails the run. None of these can show up in the 52 bodies. Someone who runs acceptance by the bar's own sentence ships as soon as the counts clear, with any of those failures still open. Users would then meet them on the largest releases, which are often cut unattended: a headline about one slice partway down the notes, a line in the published notes saying a part held nothing, or a run that stops with no notes.

**Proposal**:
The ship condition names both: the counts, and every failure the tuning section says holds shipping. The record settles it. The Checking The Chunked Path decision makes those failures hold shipping, and the Shipping Bar exists to rule out every regression, which single-pass counts cannot do on the chunked path. The wording points at every hold in the tuning section, not only the chunked ones, so it stays true if more holds are added.

**Current**:
The rewritten default prompt ships when every count is at least as good as today's prompt's, each count on its own.

**Proposed Text**:
The rewritten default prompt ships when every count is at least as good as today's prompt's, each count on its own, and every failure §10.3 says holds shipping has been cleared.

**Resolution**: Approved
**Notes**: Landed in §10.1.

---

### 2. Nothing produces the maintenance line before it ships

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §10.3 Where the tuning goes (a new bullet after the chunked-path bullet); §5.1 Only what users meet (a release with nothing users meet)

**Problem**:
This feature adds one new body for a whole release: the maintenance line written when every change is one users never meet. No run produces it before shipping. new-v3 never had the rule, and every release E1 sampled has changes users meet, so neither tuning nor acceptance ever calls for it. The chunked check cuts a release so that one part holds nothing users meet, but not every part, so the merge is never asked to write it either. If the shipped wording leaves the model returning an empty body for such a release, the transport rejects it and the run fails. A release cut after test-only or tooling-only work, run unattended with `mint release -y` as agent sessions run it, then stops with no notes, where today's prompt ships notes. If the model writes a headline instead, the tag, the `CHANGELOG.md` section and the provider release open on a line that names internal work. Either problem is first seen on a real release.

**Proposal**:
The first runs take at least one release with nothing users meet through a single call. They also take it through the part calls and the merge, with every part's slice holding nothing users meet, and check that the notes are the maintenance line. A headline on that release holds shipping until the wording clears it, and so does a single call or merge that returns an empty body and fails the run. Notes that list the release's internal changes cost what today's notes cost, and are tuned.

What leaned is the record's own rule for this kind of ground, from Checking The Chunked Path. Wording whose effect is unmeasured is checked in the first runs. A failure today's notes cannot have is a regression and holds shipping, while doing what today's notes do is tuned. The part check already applies this rule; this extends it to the whole-release case that check leaves out. Today's notes never fail on such a release and never carry a headline, so an empty body and a headline are both regressions. Listing internal changes is what today's prompt already does.

Two alternatives also fit. Holding shipping on listed internal changes too is stricter, but delays shipping over an outcome no worse than today. Having no check means the one whole-release body this feature adds ships without ever having been generated. How the harness finds or cuts such a release is the builder's.

**Proposed Text**:
§10.3, a new bullet after the chunked-path bullet:
- **A release with nothing users meet** gets a body new-v3 never wrote (§10.2). The first runs also take at least one such release through a single call, and through the part calls and the merge with every part's slice holding nothing users meet, and check that the notes are §5.1's maintenance line. A headline on that release, or a single call or merge that returns an empty body and fails the run, holds shipping until the wording clears it: today's notes have neither. Notes that list the release's internal changes instead cost what today's notes cost, and are tuned like the known weak spots below.

**Resolution**: Routed
**Notes**: The session's call over the path's own rule; landed in the discussion's A Release With Nothing Users Meet decision (written this sitting), then as a new §10.3 bullet.

---

### 3. The steps for releasing a saved file name only the marker

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Move**: settled
**Priority**: Important
**Affects**: §8 The Chunked Path (the saved-part-notes bullet)

**Problem**:
When a chunked run fails, the saved file can now hold a part's line saying that part held nothing for the notes, sitting between other parts' sections. The operator is meant to remove it while tidying, the same way they remove the `**Notes missing:**` marker. Nothing says what mint tells them to do. mint's current steps for releasing a saved file, in the README's saved part notes section, form a checklist that names only the marker: fill in the files it lists, delete the marker line, then release with `--notes-file`. The same section warns that a file still carrying the marker ships with it. An operator or agent who follows that checklist ships the part's line. The tag, `CHANGELOG.md` and the provider release then carry a sentence saying a part held nothing, in the middle of the release's sections, where readers see it.

**Proposal**:
mint's guidance on releasing a saved file tells the operator to delete these lines as well as the marker. Which surfaces carry it (the README's steps, the next-move line) is the builder's call.

What leaned: the record has the operator remove these lines "as with the notes-missing line", and mint's guidance spells out removing the marker. A checklist that leaves a line out makes it read as content to keep. That matters most for an unattended agent, which works from what mint tells it.

Alternatives that also fit: leave the guidance as it is and rely on the line plainly not being notes. Against that, the marker plainly isn't notes either, and the guidance still names it.

**Current**:
So the file holds the parts' sections, and any part's line saying it held nothing for the notes, which the operator removes while tidying, as with the `**Notes missing:**` line.

**Proposed Text**:
So the file holds the parts' sections, and any part's line saying it held nothing for the notes, which the operator removes while tidying, as with the `**Notes missing:**` line. mint's guidance on releasing a saved file tells the operator to delete these lines as well as the marker.

**Resolution**: Approved
**Notes**: Landed in §8's saved-part-notes bullet; the surfaces are the builder's.

---

## Observations

- The merge is said to judge the whole release "from every part's notes and the whole-release Change Map", next to the rule that the Change Map does not pick the headline. In the merge the map is coverage, and the merge framing is format-neutral, so it cannot restate the map as a ranking.
- Under a full override whose format says nothing about a release with nothing to list, what the merge writes when every part returned its line is the model's own choice. It is rare, and it is the operator's format to define.
- The nothing-else rule calls the maintenance line "the one body without sections", which describes the release's body. A part's nothing-line is a part's body without sections, and the framing asks for it, not the default prompt.
- The part instruction says that on the default prompt "anything that sums up the release as a whole" means the headline. The maintenance line is the other such summary, and the maintenance-line rule already leaves it to the merge.
- "Replaces three earlier decisions" sits over a list of four or five entries. The count groups them by decision, not by source.
