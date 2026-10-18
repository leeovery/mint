# Review Tracking: token-aware-diff-sizing - Input Review

## Findings

### 1. A size cap of zero or less

**Source**: Gap in the sources. `discussion/token-aware-diff-sizing.md`: "Size Cap Key, Default and the Release Timeout" and "Proactive Size Ceiling" fix the key, its unit and its default. No section names a valid range.
**Category**: Gap/Ambiguity
**Move**: settled
**Affects**: §4.1 The key

**Problem**:
Nothing says what `max_prompt_bytes = 0`, or a negative value, does. In mint, `timeout = 0` already means "no deadline", so an operator (or an agent configuring mint from the setup guide) could reasonably write 0 to mean "no cap". An implementer who copies the `timeout` handling would build exactly that. Built that way, the one guard against claude's 10 MB stdin refusal and against silently truncating providers is gone, and a release too big for the model's window sends one whole call instead of chunking. Built literally, every release stops at the floor with `no room for the diff under max_prompt_bytes` and every `mint commit` opens the editor. Either way the operator finds out from broken runs, not when config loads.

**Proposal**:
Reject a value of zero or less when config loads, in the existing load-error shape, naming the key. Three things in the record decide it:
- The cap bounds every call mint sends, sets the size every chunk is cut to, and is the only guard where reactive detection is blind.
- The record turns down any off switch for chunking: there is "no way to switch it off", and the cap "stays the only size setting".
- The record already rejects malformed `ai_command` values at load: fail loud, before any step runs.

A literal zero can carry no call, so no informed user would choose it. Reading zero as "no cap" would be the off switch the record turned down. The other answer that would fit, honouring 0 as "no cap" to match `timeout`, is ruled out by those same cap rules.

**Proposed Text**:
In §4.1, after the paragraph beginning "As a new key it takes the strict-config path":

> A value of zero or less is rejected when config loads, before any step runs, in the same load-error shape as a malformed `ai_command`: `invalid .mint.toml: max_prompt_bytes must be a positive number of bytes`. Zero never means "no cap", the way `timeout = 0` means no deadline: the cap sizes every chunk and keeps every call under claude's stdin limit. An operator who wants a cap mint never reaches sets a large one.

**Resolution**: Routed
**Notes**: Landed in the discussion as a new subtopic, Size Cap Range (zero or less rejected at load; zero never means no cap). Specification §4.1 carries the Proposed Text.

---

### 2. Unattended exclusion suggestions when the notes fail

**Source**: `discussion/token-aware-diff-sizing.md`:
- "Exclusion Suggestions Before the Notes", unattended: "the suggestions print with the notes, for whoever reads the output to act on".
- "Exclusion Check Mechanics": "Unattended follow-up — after the notes, each suggestion prints…".
- "Chunk Boundaries and the Single-File Floor", 2026-10-17 revision: "under `-y` nobody answers it, so a flagged big file is read in pieces and the printed suggestion names the glob".

All three assume the notes were written.
**Category**: Gap/Ambiguity
**Move**: settled
**Affects**: §6.7 Unattended and dry runs; §16.3 Follow-ups

**Problem**:
On an unattended release (`-y`, CI, an agent session), the exclusion check's suggestions print only after the notes. When the notes then fail and the run stops at the floor, there are no notes for them to follow, so as specified they never print.

An agent whose release stops for size gets the generic advice to "exclude generated paths with `[release].diff_exclude`", while mint already holds the exact glob, its reason and the line that would exclude it. A re-run repeats the same check and the same stop, so the suggestion never reaches the reader who needs it. Under `on_notes_failure = "fallback"` it is also unstated whether the suggestions follow the commit-subject list.

**Proposal**:
The unattended suggestions print on every unattended run that produced them, whatever happens to the notes:
- after the notes, when they are written
- after the commit-subject list, when `fallback` ships it
- with the floor's next moves, when the run stops there.

The record gives the suggestions one purpose, "for whoever reads the output to act on", and gives the unattended floor the job of naming next moves. A run that stops is exactly where the reader acts. The other answer that would fit is to print them only after notes that were written, which keeps a stopped run's output shorter. It drops the glob on the runs that need it most.

**Proposed Text**:
In §6.7, at the end of the Unattended bullet:

> They print whatever the notes' outcome: after the commit-subject list when `on_notes_failure = "fallback"` ships it, and with the next moves when the run stops at the floor (§12.4), where the reader is about to act.

In §16.3, after the list:

> On an unattended run that stops at the floor, the exclusion suggestions print with the floor's next moves instead (§6.7).

**Resolution**: Routed
**Notes**: Landed in the discussion as a new subtopic, Unattended Suggestions When the Notes Fail, with the earlier "after the notes" restatements amended. Specification §6.7 and §16.3 carry the Proposed Text.

---

## Observations

- No source asks for condensing to be deterministic or to collapse the largest runs first. That is the builder's call, and any competent implementer would build it that way.
- The no-room guidance blames only outsized operator text or a halving round. A positive cap set below the fixed part, with no `[release].prompt` or `context` set, would get the shorten-your-text guidance. That config is implausible.
- The source measures that a 200k-token window holds about 400 KB of diff. The README could give that as a starting value for operators who lower the cap.
- The source accepts that `--all` repeats the same captured output under each skipped version's warning. The spec doesn't say so, but its rule that every skip warning carries its output already settles it.
