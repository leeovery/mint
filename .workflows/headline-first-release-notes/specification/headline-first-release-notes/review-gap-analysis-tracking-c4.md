# Review Tracking: Headline First Release Notes - Gap Analysis

## Findings

### 1. A new option on an existing command fits both Added and Changed

**Source**: Specification analysis
**Category**: Contradiction
**Move**: settled
**Priority**: Important
**Affects**: §4.1 Judged against the previous release (the 🔧 Changed bullet, against the ✨ Added bullet above it)

**Problem**:
✨ Added holds anything the previous release did not have. 🔧 Changed now also takes "any smaller change to how it works that is not a fix" to something the previous release had. A new flag, option or subcommand on an existing command fits both definitions: the option is new, and it changes how an existing command works. The one sentence that settles this kind of case says a genuinely new capability is Added, but only when it "ships inside the rework", so it does not reach a smaller addition. A prompt written from these definitions lets the model file "the release command now takes a JSON output flag" under Changed. Readers who scan ✨ Added to see what they can now do miss the new capability, and someone looking for a feature they heard had shipped won't find it in Added. The judges count it as misfiled, and misfiled has 6 to spare (78 against at most 84). The known weak spot already runs the other way (reworks filed as Added), so tuning against that weak spot pushes more changes toward Changed. Any release that adds to an existing command shows the problem.

**Proposal**:
A genuinely new capability is Added whatever it rides on, whether inside a rework or as an addition to something the previous release had. Changed's smaller changes are changes to how existing behaviour works, as its own examples (a speed-up, a changed default) show. The record settles it. Added is defined by what the previous release did not have, and the new option is exactly that. The rework sentence already files a new capability under Added even when it ships inside reworked existing work. If a smaller addition filed it under Changed, the same capability would land in different sections depending on whether the thing it was added to happened to be reworked. The bullet rule makes each piece a user can use on its own a notable change, so what gets placed is the capability, not the thing it was added to. That leaves Changed with no live case for a new capability. Where a change only widens what an existing option accepts, whether that counts as a new capability is left to the wording.

**Current**:
- **🔧 Changed**: a rework or redesign of something the previous release already had, and any smaller change to how it works that is not a fix, such as a speed-up users notice or a changed default that is not breaking. A genuinely new capability that ships inside the rework is Added.

**Proposed Text**:
- **🔧 Changed**: a rework or redesign of something the previous release already had, and any smaller change to how it works that is not a fix, such as a speed-up users notice or a changed default that is not breaking. A genuinely new capability is Added, whether it ships inside a rework or is added to something the previous release had, such as a new option on an existing command.

**Resolution**: Pending
**Notes**:

---

## Observations

- §4.2 ends by saying a reader upgrading must meet what breaks "before anything else", but §2.1 and §4.2's own first bullet put the section after the headline. Read together, the sentence means before every other section.
- §7.1 says every rule in §2–§6 lives in the default prompt alone. §5.1's sentence on the part line and the merge actually lives in the chunk, piece and merge framings, in format-neutral words (§8), and §7.1's last bullet points there.
- §3.3 has the headline picked from the diff. On the chunked path the merge reads the parts' notes instead, as §8 states.
- The closing reminder ("ONLY the release notes themselves … no commentary") ends every chunk call, including one whose framing asks a part with nothing to list for a line saying so. A part that follows the reminder over the framing returns an empty body, and §10.3 already holds shipping on that.
- §5.1's Out list names contributor docs. It doesn't name user-facing documentation changes (a new guide, corrected usage docs) either way, and its In list (what the software does) reads them as out.
- §10.2's list of where new-v3's rules differ does not name §3.2's "once the other is set aside". new-v3 (lines 41–42) reads "Two bodies of EQUAL weight that each pass the test", so the list may be one entry short. Checking this is claims verification's job, not this phase's.
