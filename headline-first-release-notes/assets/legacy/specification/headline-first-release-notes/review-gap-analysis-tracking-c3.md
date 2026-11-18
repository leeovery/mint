# Review Tracking: Headline First Release Notes - Gap Analysis

## Findings

### 1. Small changes to existing behaviour have no section

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Move**: settled
**Priority**: Important
**Affects**: §4.1 Judged against the previous release (the 🔧 Changed bullet); the rules that send changes there sit in §4.2 (What does not; When unsure) and §5.1 (the speed-up users notice)

**Problem**:
The notes define 🔧 Changed as a rework or redesign of something the previous release already had. The same rules also send smaller changes to existing behaviour into a section without naming one. A speed-up users notice must be listed. A change that keeps more, allows more or improves, or a changed default that isn't breaking, goes in "its usual section". No section definition takes them. Added is for what the previous release didn't have, Fixed is for what behaved wrongly, and Changed covers only reworks. A prompt written from these definitions gives the model nowhere to put a faster command or a new default. The model then files it under Added or Fixed, telling upgrading users that something is new or was broken when it was neither, or drops it. Dropped changes add to missing items, which is the count second furthest from the bar. Readers see this in any release that adjusts an existing feature without reworking it.

**Proposal**:
Changed also covers any smaller change to how something from the previous release works, as long as the change isn't a fix. The spec's own rules settle this. A speed-up users notice is listed. A changed default that isn't breaking, and an improvement, both go in their usual section. The Added rule says a tweak to something new is never Changed, which assumes a tweak to something existing is. The fork has one live side. Added is ruled out because the previous release had the thing. Fixed is ruled out because nothing behaved wrongly. Leaving the change out breaks the rule that changes users notice are listed.

**Current**:
- **🔧 Changed**: a rework or redesign of something the previous release already had. A genuinely new capability that ships inside the rework is Added.

**Proposed Text**:
- **🔧 Changed**: a rework or redesign of something the previous release already had, and any smaller change to how it works that is not a fix, such as a speed-up users notice or a changed default that is not breaking. A genuinely new capability that ships inside the rework is Added.

**Resolution**: Approved
**Notes**: Landed in §4.1.

---

### 2. Two co-equal bodies can never both pass the headline test

**Source**: Specification analysis
**Category**: Contradiction
**Move**: settled
**Priority**: Important
**Affects**: §3.2 When a release has one (the two co-equal bodies bullet, against the main-body test in the bullet before it)

**Problem**:
A body of work earns a headline when most of the notable changes serve it, or when it alone clearly outweighs all the other changes put together. Two co-equal bodies are named together when each one passes that test. Taken literally, two bodies of equal weight can never both pass. They can't each be served by most of the changes. Neither can clearly outweigh all the rest, because the rest includes the other body, which weighs the same. Whoever writes the prompt's headline guidance will carry this contradiction into it. A model that reads it literally gives every two-body release no headline. A model that reads it loosely tends to pick one body, so the release's most-read line leaves out half of what the release is. Both outcomes are already a known weak spot (two-body releases losing their headline, or getting half of one), and the rule as written pushes toward them.

**Proposal**:
Each body takes the test with the other body set aside. The spec settles that the two-body rule is meant to fire: it decides that two co-equal bodies are named together, at most two, in one sentence led by the benefit, and it tunes against two-body releases losing their headline. The literal reading would make that decided rule dead text, so it is the dead side. Between the two phrasings that work, setting the other body aside keeps the per-body test the spec already states, and it is the stricter of the two, which fits the leave-it-out-when-in-doubt rule. The alternative that also fits is to run the test on the pair together: most of the notable changes serve one or the other, or the two together clearly outweigh everything else. That version is slightly more permissive at the margin.

**Current**:
Where two bodies of work of equal weight each pass the test, the one sentence names both.

**Proposed Text**:
Where two bodies of work of equal weight each pass the test once the other is set aside, the one sentence names both.

**Resolution**: Routed
**Notes**: The session's pick between two readings; landed as a dated revision of the discussion's No Clear Headline decision, then in §3.2.

---

### 3. The lean to listing reads as a rule for every release

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Move**: settled
**Priority**: Important
**Affects**: §5.1 Only what users meet (a release with nothing users meet)

**Problem**:
The maintenance-line rule says that when unsure whether a change is one users meet, the change is listed in its section and neither the maintenance line nor a part's nothing-line is written. The wording reads as a doubt rule for every change in every release. Its reason covers only the maintenance line: a false one tells every reader, in the release's only line, that nothing changed for them. A prompt writer looking for guidance on borderline changes will find only this sentence, and may lift it into the general leave-out guidance. If so, every release's notes lean toward listing borderline internal work, such as a refactor or an internal flag the model can't rule out users noticing. Readers then see internal items in ordinary releases, which is exactly what the users-meet rule exists to remove. The bar wouldn't catch it, because the internal-items count has wide room (28 against at most 77). Nothing else in the spec gives ordinary releases a lean either way.

**Proposal**:
Limit the sentence to the decision it serves: whether the release, or a part's slice, holds any change users meet. The spec's own reason settles this, since it weighs only what a wrong maintenance line or nothing-line costs. A lean to listing in every release would trade internal items for fewer missing items on every release. Nothing in the spec makes that trade, so the scoped reading is the only one the spec supports. Under it, a borderline change in an ordinary release is judged by the users-meet test alone, as it is today.

**Current**:
When unsure whether a change is one users meet, the change is listed in its section and neither line is written: a line left out costs what today's notes cost, while a false one drops a change users meet and tells every reader, in the release's only line, that nothing changed for them.

**Proposed Text**:
When unsure whether the release, or a part's slice, holds any change users meet, the change in doubt is listed in its section and neither line is written: a line left out costs what today's notes cost, while a false one drops a change users meet and tells every reader, in the release's only line, that nothing changed for them.

**Resolution**: Routed
**Notes**: Record-determined by the rule's own reason; the same wording was repaired in the discussion's A Release With Nothing Users Meet decision (written this sitting) and in §5.1.

---

## Observations

- The maintenance line is described as a line "in the shape of" the empty-diff stub, not a fixed string, so its exact words may vary between releases unless the prompt gives one line to write.
- "No code fences" in the new closing reminder could be read under a full override as barring fenced blocks inside the notes, not just a fence around them.
- The spec has mint's descriptions of the Change Map follow its new role. It says nothing about any user-facing text that describes the default notes format itself. Updating that text is the builder's job.
