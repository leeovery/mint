# Review Tracking: Headline First Release Notes - Input Review

## Findings

### 1. The chunked path's part rules predate the maintenance line

**Source**: discussion, Chunked Path → Decision ("The saved part-notes file therefore holds sections only") and Journey ("it asks the part to leave out anything that sums up the release as a whole, which the merge writes"); A Release With Nothing Users Meet → Decision ("A chunked release's part with nothing users meet returns the same line, and the merge writes it only when every part's notes are that line") and Context ("Path-order packing can put a run of test files or docs in a part of its own"). The Chunked Path decision was reached before the maintenance line existed and still describes parts as returning sections and nothing else.
**Category**: Source defect
**Move**: route
**Affects**: §8 The Chunked Path (the part-instruction bullet and the saved-part-notes bullet); §3.4 Bodies that carry no headline (closing paragraph); §5.1 Only what users meet (the chunked sentence of "A release with nothing users meet")

**Problem**:
On a release written in parts, a part whose slice is only tests, docs or tooling returns the maintenance line (the "no user-facing changes" line in the shape of the empty-diff stub). Path-order packing makes such a part ordinary on large releases. Two rules for parts were set when parts returned sections and nothing else, and both now misdescribe what users get:

- The saved part notes are said to hold sections only. When the merge fails, the file keeps each part's notes byte for byte, so it can carry a maintenance line between other parts' Added and Fixed lists. Anything built on "sections only", such as the narration and docs on tidying the file, gives the operator no reason to look for that line. A release shipped from the file with `--notes-file` then publishes to the tag, `CHANGELOG.md` and the provider release a line saying the release has no user-facing changes, in the middle of its feature list.
- The part framing tells every part to leave out anything that sums up the release as a whole, and the maintenance line says exactly that about the release. A part with nothing users meet is told both to write the line and to leave it out. If it lists its internal changes instead, the merge reads them as notes for the largest releases. If it returns nothing, the transport rejects the empty body, the part fails, and the chunked run fails with it, so an unattended release of the largest kind stops with no notes.

**Proposed Text**:

**Resolution**: Pending
**Notes**:

---

### 2. The first chunked runs never check the maintenance line on parts and the merge

**Source**: discussion, A Release With Nothing Users Meet → Decision (a part with nothing users meet returns the line; the merge writes it only when every part's notes are that line) and Context (path-order packing can put a run of test files or docs in a part of its own); Checking The Chunked Path → Decision and Journey (the first tuning runs take held-out releases through the parts and the merge; "on this path only the failures today's notes could not have are regressions", and those hold shipping; leaving the path to the read by eye was rejected because the largest releases, often unattended, go unchecked until after they ship). No source decides a check for the maintenance line on this path.
**Category**: Gap/Ambiguity
**Move**: settled
**Affects**: §10.3 Where the tuning goes (the chunked-path bullet)

**Problem**:
On a release written in parts, the merge has to recognise a part's maintenance line, drop it when other parts listed real changes, and write it only when every part returned it. No run has exercised this: E1's wording had no maintenance-line rule, and E1 never split a release. The first chunked runs check only the headline. If the merge keeps the line, the largest releases ship notes that open with, or carry among their sections, a line saying the release has no user-facing changes. If it writes the line in place of the sections, every change the parts found is lost. If a part returns nothing instead, the run fails. These releases are often cut unattended with `mint release -y`, so nobody sees the notes before they reach the tag, `CHANGELOG.md` and the provider release. Readers find notes that contradict themselves, or a release that never got notes.

**Proposal**:
The first chunked runs also check the parts that hold nothing users meet, and the failures today's notes cannot have hold shipping, as they do for the headline. Revised at dispose so the check reads true under the part rule finding 1 lands (a part with nothing for the notes returns one line about its part, never an empty body): the check names the part's line and the release's maintenance line separately rather than calling them the same line. What determined it: the record's rule for this path (Checking The Chunked Path: untested shared behaviour is checked in the first runs; failures today's notes could not have are regressions and hold shipping). Today's parts never return an empty body or a nothing-here line, so a kept line, a line in place of sections, and an empty part are failures today's notes cannot have; a part that lists its internal changes does what today's parts do and is tuned. The runs must include a part whose slice holds nothing users meet; how the harness cuts a release to get one is the builder's. Alternatives weighed: tuning without holding shipping (lets a regression ship on the largest releases), or the read by eye (rejected for this path).

**Proposed Text**:
§10.3, the chunked-path bullet, appended:
The same runs check the parts that hold nothing users meet, on at least one release cut so that a part's slice holds nothing users meet: that part returns its one line rather than an empty body (§8), and the merge leaves the line out beside other parts' sections and writes §5.1's maintenance line only when every part returned one. Merged notes that carry a part's line or the maintenance line while any part listed changes users meet hold shipping until the wording clears it, and so does a part that returns an empty body and fails the run: today's notes have none of these. A part that lists its internal changes instead costs what today's notes cost, and is tuned like the known weak spots below.

**Resolution**: Routed
**Notes**: The session's call over the path's own rule; landed in the discussion by amending Checking The Chunked Path (Journey and Decision, written this sitting), then in §10.3.

---

## Observations

- §9 states no condition for reopening the bare-tag title. The discussion records what would have moved it: valuing scanning the releases list by title over consistent titles.
- §10.3 says false claims are furthest from the bar but not what the discussion found them to be: behaviour the previous release already had, presented as new or changed, or a claim drawn from a README edit with no code behind it. Several sat in bullets worded with a before ("moved from", "now", "no longer").
- E1 ran its two held-out releases over the 1.5 MB cap as single calls, and acceptance repeats that. On those two it measures single-pass notes that mint would in fact write in parts; the chunked check is what covers that path.
