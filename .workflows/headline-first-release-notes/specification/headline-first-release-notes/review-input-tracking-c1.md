# Review Tracking: Headline First Release Notes - Input Review

## Findings

### 1. The combine rule has no stated condition for reopening

**Source**: discussion, Headline Shape → Journey ("Will the sections really carry the pieces?"): "What would reopen it: evidence of the model collapsing a big feature into one bullet on real releases, which would put the combine rule's own wording in question."
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §2.2 Bullets (unchanged)

**Problem**:
If the model folds a large feature's separate capabilities into one bullet, readers of the biggest releases get a headline plus one bullet that restates it, and the capabilities that make up the release are never listed. Someone scanning ✨ Added for a capability they heard shipped won't find it. The specification locks the combine sentence "as worded" and gives no way out, so tuning that sees the collapse can only adjust the wording around that sentence. It has no grounds to question the sentence itself, even though the record names that sentence as what such evidence calls into question. The specification records the matching reopen condition for the neutral reminder but drops this one.

**Proposal**:
Carry the discussion's reopen condition into the bullet that fixes the combine rule, phrased the same way as the reminder's "What would reopen this". The source states it, so the only open question is where it goes: in the combine rule's own bullet, where the "stands as worded" lock sits.

**Current**:
- One bullet per notable change, never one per file, hunk or commit. The combine rule stands as worded: "Combine closely-related changes into a single bullet." Closely related means the small tweaks of one change, such as three commits that fix one gate. It never means a feature's separate capabilities: each piece a user can use or meet on its own is a notable change and gets its own bullet. Version bumps and other bookkeeping churn are ignored. Most releases are a handful of bullets, and the notes are not padded.

**Proposed Text**:
- One bullet per notable change, never one per file, hunk or commit. The combine rule stands as worded: "Combine closely-related changes into a single bullet." Closely related means the small tweaks of one change, such as three commits that fix one gate. It never means a feature's separate capabilities: each piece a user can use or meet on its own is a notable change and gets its own bullet. What would reopen the combine rule's wording: evidence on real releases of the model collapsing a large feature into one bullet. Version bumps and other bookkeeping churn are ignored. Most releases are a handful of bullets, and the notes are not padded.

**Resolution**: Approved
**Notes**: The discussion states the reopen condition (Headline Shape → Journey); landed in the combine rule's bullet.

---

### 2. A ban on prompt text drawn from the sampled releases that no source decides

**Source**: No source decides this. Checked: the discussion's Shipping Bar (Journey and Decision), Headline Shape, and the Summary. The only guard against overfitting the record names is in the Shipping Bar Journey: "the bar's guard against that is that it asks for no worse than today rather than a tuned threshold, and the rules it carries are general." The specification's wording matches the constraint E1's experiment design put on its own prompt revisions (E1 `design.md`, line 72). That document is not a source of this specification.
**Category**: Unsourced decision
**Move**: route
**Affects**: §10.2 Tuning and acceptance (the Tuning bullet)

**Problem**:
The specification constrains both the shipped default prompt and the tuning that produces it: "no prompt text names, quotes or paraphrases a change from the 31 sampled releases, and any example added is invented (the three shipped altitude examples are exempt)". The discussion never made this call for shipping. It decided only that the rules are general and that the bar asks for no worse than today. E1 imposed the constraint on experiment revisions, and nothing in the record carries it over to implementation. It matters to users either way. Without it, wording can be fitted to the 26 acceptance releases, pass every count, and then do worse on everyone else's repos than the counts promise. With it, the implementer is bound by a rule no decision made, and has no recorded basis for the altitude-example exemption.

**Proposed Text**:

**Resolution**: Pending
**Notes**:

---

### 3. Nothing checks the headline on the chunked path before it ships

**Source**: discussion, Chunked Path → Decision (part calls write no headline, the merge alone writes it; no measurement named); Shipping Bar → Decision (measured on E1's held-out releases); Prompt Overrides And Reminder → Journey (the untested neutral reminder "is a measurement, not a decision — it rides with the next run of the prompt"); No Clear Headline → Options (omitting the headline "drops the headline on the releases where it helps most — the biggest ones"). The specification itself records that E1 never exercised the chunked path.
**Category**: Gap/Ambiguity
**Move**: settled
**Affects**: §10.3 Where the tuning goes

**Problem**:
The releases big enough to be written in parts are the ones with the most main work, where the headline matters most. They depend on wording that no run has tested: the new instruction in the chunk and piece framings telling parts to leave out the release-wide summary, and the merge writing the headline from the parts' notes. E1 ran every release as one call, so the bar's counts say nothing about this path. The specification marks the neutral reminder as untested and has the first runs check it, but it plans no check for the chunked wording. Two failures can then reach users unseen. A merge can drop the headline on the largest releases. Or part calls can keep writing their own headlines, which a failed merge's saved part-notes file stacks one after another. With `mint release -y` nobody reads the notes before they reach the tag, the changelog and the provider release.

**Proposal**:
Give the chunked path the same treatment the record gives the other untested shared wording. The first tuning runs also send held-out releases through the part calls and the merge, and check that part notes carry no headline and that the merge writes the headline the release calls for. Like the reminder's check, this sits outside the bar. What leaned: the discussion's handling of the neutral reminder (untested shared wording is checked in the next runs, not decided or gated), and its point that the headline earns most on the biggest releases. Alternatives that also fit: adding chunked runs to the bar as gating counts, ruled out in practice because today's prompt has no measured chunked counts to hold them to, so "at least as good as today" has no baseline there; or leaving the chunked path to the by-eye read on real releases, which leaves the largest releases, often run unattended, unchecked until after they ship. How to force the held-out releases into parts within E1's harness is left to the builder.

**Proposed Text**:
- **The chunked path** is untested: E1 ran every release as one call. The first runs also take held-out releases through the part calls and the merge, and check that the part notes carry no headline and that the merge writes the headline §3.2 calls for. Like the reminder's check, it sits outside the bar.

**Resolution**: Routed
**Notes**: A call the discussion never made; landed in the discussion as the new subtopic Checking The Chunked Path (settled by derivation from Prompt Overrides And Reminder and No Clear Headline), Summary Open Threads and Current State updated; §10.3 re-aligned to it.

---

## Observations

- The §8 bullet headed "Every regenerate goes through the merge" claims more than the record does. The record says a regenerate on chunked notes and a merge re-run from saved part notes go through the merge; a single-pass regenerate goes through the single-pass call.
- §4.1's list of what "behaving wrongly" covers (wrong output or placement, a crash or error, dropped or lost data, something not working) spells out the Fixed rule further than the discussion records. It reads as an elaboration, not a new rule.
