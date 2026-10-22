# Review Tracking: Headline First Release Notes - Gap Analysis

## Findings

### 1. The worked headline breaks the "and" rule

**Source**: Specification analysis
**Category**: Contradiction
**Move**: settled
**Priority**: Important
**Affects**: §3.1 Shape (the worked headline example); §3.2 When a release has one (the "and" rule)

**Problem**:
The headline rules say "and" in a headline joins only two co-equal bodies of work, never two pieces of one body (§3.2, beside the two-co-equal-bodies rule). The spec's only worked headline joins two steps of one feature with "and": "…is split into parts, and their notes are merged into one coherent set". Whoever writes the prompt's headline guidance, or judges headlines while tuning, is left with two incompatible models. The rule forbids the example's shape, and the example permits chaining a feature's pieces with "and", which is the inventory of pieces the headline is meant to avoid. Readers would get headlines that walk through how a feature works piece by piece instead of naming what the release gives them.

**Proposal**:
Correct the example and keep the rule. The rule is a decided constraint and the example only illustrates the headline's shape. Rewording one clause removes the "and" without changing what the example says.

**Current**:
```text
Release notes for diffs of any size — a diff too big for the model is split into parts, and their notes are merged into one coherent set.
```

**Proposed Text**:
```text
Release notes for diffs of any size — a diff too big for the model is split into parts whose notes merge into one coherent set.
```

**Resolution**: Pending
**Notes**:

---

### 2. Operator-written bodies are said to carry no headline

**Source**: Specification analysis
**Category**: Contradiction
**Move**: settled
**Priority**: Important
**Affects**: §3.4 Bodies that carry no headline

**Problem**:
The list of bodies that skip the AI says all six "carry no headline". Two of them are written by the operator: a `--notes-file` body, and notes written in the editor at the notes floor. The chunked-path section (§8) says a release shipped from the saved part notes, by `--notes-file` or `e`, carries a headline if the operator writes one while tidying. The supplied-headline section (§3.5) also keeps a finished `--notes-file` body open as a later way to supply a headline. Read as written, the list says an operator's own body may not open with a headline. Anyone who writes mint's user- or agent-facing guidance from it would tell operators and agents to start a `--notes-file` body at the first section, which closes off the route the spec keeps open.

**Proposal**:
The list means that mint and its AI add no headline to these bodies. A body the operator writes carries one exactly when the operator writes it. Two things settle this: the byte-identical rule, which is read as protecting what the operator wrote, and the chunked path's statement about notes the operator tidies.

**Current**:
The other six carry no headline:

**Proposed Text**:
mint and its AI add no headline to the other six. A body the operator writes, a `--notes-file` body or notes written in the editor, carries one only if the operator writes it:

**Resolution**: Pending
**Notes**:

---

### 3. A breaking change with nothing in the diff for the user to do

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Move**: settled
**Priority**: Important
**Affects**: §4.2 💥 Breaking

**Problem**:
Every Breaking bullet must say both what breaks and what to do. Some breaks leave the user nothing to do that the diff shows: a command removed with no replacement, or a data format that is no longer read and has no migration in the diff. Held to "say what to do", the model makes up a step, such as a replacement command, a migration or a setting that does not exist. That is a false claim in the opening lines of the notes, where the spec itself says an error costs most, and false claims are already the count furthest from the shipping bar. An upgrading user finds out when the step they were told to take does not work.

**Proposal**:
What to do comes only from what the diff shows: a new name, a replacement, or a setting that keeps the old behaviour. Where the diff shows none, the bullet says what breaks and stops there. Two rules settle this: nothing is claimed beyond what the release's diff shows, and the Breaking section's own asymmetry says a false line at the top costs more than a missing one. One alternative also fits: the bullet could say outright that nothing replaces the removed thing. That helps users when it is true, but a diff-only view cannot prove that something does not exist, so the claim can itself be false.

**Current**:
- **What breaks, and what to do.** Each bullet says both in its one sentence.

**Proposed Text**:
- **What breaks, and what to do.** Each bullet says both in its one sentence. What to do comes only from what the diff shows, such as a new name, a replacement, or a setting that keeps the old behaviour. Where the diff shows none, the bullet says what breaks and stops there.

**Resolution**: Pending
**Notes**:

---

### 4. A release where nothing changes for users

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §5.1 Only what users meet; §2.1 Shape

**Problem**:
The notes now leave out everything users don't meet: CI, tests, refactors, internal packages, contributor docs and repo housekeeping. The body may hold only an optional headline and sections. Nothing says what the notes are when every change in a release is of the left-out kind, as with a release cut after test-only or tooling-only work. With no rule, the model does one of three things. It lists the internal changes anyway, which is the error the shipping bar counts as internal items. It writes a line the format forbids, such as "No user-facing changes in this release." Or it returns an empty body. The user notices on the first such release. They see notes that break the new rules, or a stray commentary line on the tag, changelog and provider release, or an AI call that gives nothing to ship.

**Proposal**:
The notes are a single 🔧 Changed bullet saying the release is maintenance only and changes nothing users meet, with no headline. Three things lean this way. The format allows nothing in the body except a headline and sections, so a sections-only body is the one shape that needs no exception. The rule that notes hold only what users meet, together with the bar's internal-items count, rules out listing the internal changes. And a headline needs a main body of work that users meet, which this release does not have. Two alternatives also fit. One is a single plain line in the style of the empty-diff maintenance stub, which reads like that body but needs an exception to the nothing-else rule. The other is to list the internal changes as an exception for this case only, which the bar counts against.

**Proposed Text**:
- **A release with nothing users meet.** Where every change in the release is one this section leaves out, the notes are a single 🔧 Changed bullet saying the release is maintenance only and changes nothing users meet. There is no headline (§3.2).

**Resolution**: Pending
**Notes**:

---

### 5. The acceptance pass must measure the wording that ships

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Move**: settled
**Priority**: Important
**Affects**: §10.2 Tuning and acceptance (Acceptance); §10.1 The bar

**Problem**:
The acceptance pass is the one run that decides whether the new notes ship. It has two problems as written. First, it is described as a pass of "the frozen candidate". Everywhere else, the spec uses that name only for E1's untuned new-v3 wording, which already misses the bar on false claims and missing items. Second, it is run "as E1 ran it", and E1's harness ended every call with E1's own reminder, which names the headline, not with the format-neutral reminder mint ships. Read literally, the bar could be passed or failed on wording that is not what ships. Users on the default prompt would then get notes that never passed the bar. For example, if the neutral reminder makes the model drop headlines, they would get fewer headlines than the bar recorded, and nobody would see this until releases ship.

**Proposal**:
Acceptance measures the wording as it ships, frozen when tuning ends: the rewritten default prompt, closed by the format-neutral reminder. The record settles this. The bar is stated as the condition for the rewritten default prompt to ship, and the spec names the format-neutral reminder, not E1's, as part of what ships. A pass on any other wording measures notes that no user will get.

**Current**:
- **Acceptance** is one pass of the frozen candidate over the 26 held-out releases, two repeats each. It is run as E1 ran it: E1's harness, the same inputs, the same AI command and model, E1's rubric and judge grouping, and E1's scoring. It sits outside mint's test suite, which never spawns a real AI command.

**Proposed Text**:
- **Acceptance** is one pass of the wording as it ships, frozen when tuning ends, over the 26 held-out releases, two repeats each: the rewritten default prompt, closed by §7.1's format-neutral reminder. It is otherwise run as E1 ran it: E1's harness, the same inputs, the same AI command and model, E1's rubric and judge grouping, and E1's scoring. It sits outside mint's test suite, which never spawns a real AI command.

**Resolution**: Pending
**Notes**:

---

### 6. What a failed chunked-path check does to shipping

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Move**: settled
**Priority**: Important
**Affects**: §10.3 Where the tuning goes (the chunked-path bullet)

**Problem**:
The first tuning runs send releases through the part calls and the merge. They check that part notes carry no headline and that the merge writes the headline the release calls for. This check "sits outside the bar", and nothing says what a failed check does to shipping. If part calls still write headlines, a large release's notes can open on a summary of one slice, a headline that names part of the release as if it were the whole. The bar allows none of these on single-pass notes. A failed merge's saved notes would also carry stray summary lines partway through the file. If "outside the bar" is read as "not blocking", the release ships that way. Users first see it on a release big enough to be split, which is usually among the largest and most-read releases.

**Proposal**:
A failed check holds shipping when the failure is a regression, and not otherwise. A headline in any part's notes, or a merged headline that names something other than the release's main work, holds shipping until the part instruction's wording fixes it. Today's notes never carry a headline, so either failure is a regression. Ruling out regressions is the whole purpose of the bar, and the bar allows no wrong-subject headline. A merge that leaves out a headline the release calls for does not hold shipping. That costs what today's notes cost, which is the accepted cost of leaving the headline out when in doubt, and it is tuned like the known weak spots. Two alternatives also fit. Holding shipping on either failure is stricter, but it delays shipping over an outcome no worse than today. Holding on neither lets a regression ship on the largest releases.

**Current**:
- **The chunked path** is untested: E1 ran every release as one call. The first runs also take held-out releases through the part calls and the merge, and check that the part notes carry no headline and that the merge writes the headline §3.2 calls for. Like the reminder's check, it sits outside the bar.

**Proposed Text**:
- **The chunked path** is untested: E1 ran every release as one call. The first runs also take held-out releases through the part calls and the merge, and check that the part notes carry no headline and that the merge writes the headline §3.2 calls for. Like the reminder's check, it sits outside the bar. A headline in any part's notes, or a merged headline that names something other than the release's main work, still holds shipping until the part instruction's wording clears it: today's notes carry no headline, so either is a regression. A merge that leaves out a headline §3.2 calls for does not hold shipping, since that costs what today's notes cost, and it is tuned like the known weak spots below.

**Resolution**: Pending
**Notes**:

---

## Observations

- The list of what every notes call carries says "the diff or one part's slice of it", but the merge call gets the parts' notes instead. This is wording only, since the inputs themselves do not change.
- "Every regenerate goes through the merge" refers to the review gate's `r` on chunked notes, not to `mint release regenerate`. The next sentence makes this clear.
- "Piece" calls are named but never defined. The chunked mechanics are unchanged, so nothing depends on it.
- The spec says the headline "cannot be a lifted bullet", but also that when the main work is a single bullet the overlap "costs one repeated line". Whether that headline may repeat the bullet word for word is left to the wording.
- The headline must be "plain text", but the spec does not say whether a command name in backticks counts as formatting.
- A release with more than two co-equal bodies of work already gets no headline under the leave-it-out-when-in-doubt rule. The phrase "as best it can" is consistent with that.
- The spec does not say whether a security fix with an explicit marker goes under Security or Fixed. Security is the obvious home.
- The spec does not say outright whether tuning may run on the 26 held-out releases. Its guard sentence treats wording fitted to the tested releases as an accepted risk.
