# Specification: Headline First Release Notes

## Specification

### 1. Purpose and Scope

#### 1.1 What this changes

A release's notes are one body that ships byte-identical on three surfaces: the annotated tag, the `CHANGELOG.md` section under mint's `## [x.y.z] - date` header, and the provider release body. This feature changes what that body says and in what order, wherever the fix lives: in mint's default notes prompt (`DefaultPrompt`, `internal/notes/prompt.go`), or in the wording mint adds to every notes call (the closing reminder, the chunk and piece framings, the merge framing). It covers:

- a one-sentence headline above the sections that names the release's main body of work (§3);
- a 💥 Breaking section, and each change placed by comparing it with the previous release (§4);
- notes limited to what users meet, and true to the release (§5);
- ordering within sections by weight to the user, with the Change Map turned from a ranking signal into a coverage check (§6);
- shared wording that is format-neutral, so the headline reaches only repos on the default prompt (§7), and the chunked path's split of the headline between part calls and the merge (§8).

Every rule answers to the notes themselves: good, readable, coherent, useful, accurate and well ordered, for someone who uses the software. §10 makes that bar measurable.

#### 1.2 What does not change

- **The AI's inputs.** Every notes call still carries the resolved instructions, the Change Map, the post-exclusion diff (or one part's slice of it) and the closing reminder. Nothing is added. Commit history stays out, and the diff stays the only source of truth for what changed. None of the failures this feature addresses needs mint to hand the AI more (§5.2).
- **How the Change Map is built.** Its git calls, detection and rendering stay as they are (§6.2).
- **Bodies pass through verbatim.** mint never parses, reformats or adds to a notes body, and never writes a headline (§3.4).
- **The provider release title** stays the bare tag (§9).
- **The body selector's precedence and every body that skips the AI.** None of them goes through the notes prompt (§3.4).
- **The transport and the chunked path's mechanics.** Validation, the single bad-content retry, timeouts, part sizing, halving and the saved part notes all stay as they are.
- **`mint commit`.** Its prompt and closing reminder are separate from these and out of scope.

#### 1.3 Out of scope

- The `diff_exclude` pathspec bug. Globs go to git as plain `:(exclude)` pathspecs, so `**/testdata/**` misses a `testdata/` at the repo root. It is a mint bug and is handled on its own.
- The largest releases never reaching the AI. Token-aware diff sizing closes this.
- `CHANGELOG.md` drifting after a regenerate.
- Any route for a headline supplied by an operator or an agent (§3.5). That belongs to the agent-authored release notes roadmap item.

#### 1.4 Evidence

The rules rest on two records:

- [Release-notes audit](../../imports/release-notes-audit.md): 31 AI-written releases across nine projects, all written on today's default prompt and compared with their code. The calls in it are judgement, not a controlled measurement.
- [E1 report](../../experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/report.md): today's prompt against a rewritten candidate on 26 held-out releases, two repeats each, judged blind. Its frozen candidate is the starting wording (§10).

#### 1.5 Decisions this replaces

Once shipped, this feature replaces three earlier decisions for the default prompt and the merge framing:

- from the mint-release-tool specification: the six-section default notes format, checking the notes criterion by eye with no formal rubric, and the Change Map as the ranking signal (with release-notes-quality's "rank importance from the Change Map");
- from token-aware diff sizing: the merge call "ranking by the Change Map".

### 2. The Notes Format

#### 2.1 Shape

The default prompt asks for this body and nothing else:

```text
{headline: one plain sentence, only where the release has one (§3)}

💥 Breaking
- {one-line bullet}

✨ Added
- {one-line bullet}

🔧 Changed
- …
```

- **The headline** is optional. Where there is one, it is the body's first line, followed by one blank line and then the first section (§3).
- **The sections** always run in this fixed order: 💥 Breaking, ✨ Added, 🔧 Changed, ⚠️ Deprecated, 🗑️ Removed, 🐛 Fixed, 🔒 Security. A section with no items is left out. The order is the same in every release, whatever its main work. The headline already lifts the main work above every section, so reordering the sections would answer the same problem twice and cost readers an order they can rely on across versions. What belongs in each section is §4.
- **Nothing else.** The headline is the only thing that may appear above the first section: no summary paragraph, no TL;DR, no label, no other preamble. The default prompt's "start directly with the first section header" gives way to this headline-or-first-section rule, which still forbids every other preamble. The notes carry no wrapper, markers or machine-parseable labels, and no commentary or sign-off. The one body without sections is §5.1's maintenance line.

#### 2.2 Bullets (unchanged)

- Each bullet is one line and one sentence: the change in plain language, with its user-facing effect after an em dash where that helps. No second sentence, no bold lead-in, no sub-bullets, no implementation detail, and no rationale for how it was built.
- The shipped prompt's three altitude examples stay as they are.
- One bullet per notable change, never one per file, hunk or commit. The combine rule stands as worded: "Combine closely-related changes into a single bullet." Closely related means the small tweaks of one change, such as three commits that fix one gate. It never means a feature's separate capabilities: each piece a user can use or meet on its own is a notable change and gets its own bullet. What would reopen the combine rule's wording: evidence on real releases of the model collapsing a large feature into one bullet. Version bumps and other bookkeeping churn are ignored. Most releases are a handful of bullets, and the notes are not padded.
- Deprecated and Security stay opportunistic. An item goes in them only on an explicit marker in the diff (a `@deprecated` annotation, or an obvious auth, cryptography, input-validation or CVE-bump surface), and is never inferred.

#### 2.3 Each change once

Every change is listed once, in one section. The only restatement in the body is the headline's summary of the main work, which overlaps that work's pieces in the sections by design (§3.1).

### 3. The Headline

#### 3.1 Shape

The headline is one sentence that sums up the release's main body of work. It is held to the bullets' altitude: terse, in plain language, naming the change and what it gives its user. It is never a synthesis of everything in the release, and never a narrative thread that ties the changes together. That narrative is what grew into the essay-length TL;DR the format carried until 886c6100 (2026-06-13) removed it for length. The headline wins the lead back without bringing the essay back.

- **It leads with what the release gives its user.** That means what users can now do, or no longer suffer, in the terms they meet it, never in files, formats or internals. It does not describe the scope of the work: no scale or praise words ("major", "overhaul", "powerful"), and no inventory of the work's pieces. The sections carry the scale.
- **It can name any kind of change.** The main body of work may be a feature, a rework, a fix or a large epic. The headline names it whatever section its pieces sit in.
- **It is plain text.** No bold, no emoji, and no label such as "Highlights:" or "TL;DR:". Its position and the blank line beneath it set it apart (§2.1).
- **It does not relieve the sections.** Every notable change is listed below the headline exactly as it would be without one, the pieces of the main work included. The prompt says so, so that the model does not thin the sections because the top line already summed them up. The headline and the sections overlap by design: summary, then detail. In the common release, one body of work built from many pieces plus some sideline changes, the main work takes several bullets, not one. So the headline cannot be a lifted bullet. Where the main work is a single bullet, the overlap costs one repeated line.

A release whose main work is notes for diffs of any size, in this shape:

```text
Release notes for diffs of any size — a diff too big for the model is split into parts, and their notes are merged into one coherent set.

✨ Added
- Oversized diffs are chunked into parts that each fit `max_prompt_bytes`, with one merge call writing the release's notes.
- A call the model rejects as too big is retried as smaller parts, and mint tells you the largest size that worked.
- …

🐛 Fixed
- …
```

#### 3.2 When a release has one

- **A main body of work gets a headline.** A main body of work is one feature, rework or fix that anyone asked "what is this release?" would name. The test, run against the notes' own bullets: most of the notable changes serve it, or it alone clearly outweighs all the other changes put together. The largest of several separate changes is not a main body of work.
- **Two co-equal bodies are named together.** Where two bodies of work of equal weight each pass the test, the one sentence names both. That is at most two, still in one sentence, and still led by the benefit. A headline names a second subject only when it is a co-equal body of work: never two pieces of one body, and never a main body plus a lesser piece. A release with more than two co-equal bodies is not designed for: the one-sentence limit stands, and the model handles the case as best it can.
- **No headline where one bullet already states the whole release** and nothing else stands beside it. There the headline would only repeat that bullet. The test is whether a single bullet states the whole release, not how many bullets there are. Where the main work is spread over several bullets and none of them names the whole, the headline stays even when the notes hold nothing else, because it is the one line that says what the release is. A main feature written as a single bullet beside unrelated changes keeps its headline too, since there is a rest to set it apart from.
- **No headline on a grab-bag.** A release of small unrelated changes has no main body of work, however large the largest of them, so there is nothing to sum up. Its notes start with the first section. A theme stitched across such changes is the narrative thread §3.1 rules out.
- **When in doubt, leave it out.** The AI writes a headline only when one body of work (or two, as above) clearly stands out. Otherwise the notes start with the first section. A missing headline costs no more than today's notes do. A wrong one misstates the release in its most-read line, and on an unattended run (`mint release -y`, as agent sessions run it) nobody reads the notes before they ship. Accepted cost: some releases that deserved a headline go without one. On an attended run the review gate's existing correctives remain: `e` to edit, and `r` to regenerate with a one-time nudge such as "lead with the auth work".

#### 3.3 How the headline is picked

The AI picks the headline by what the release changes for its user, read from the diff. A new folder or a large file is not, of itself, the release's main thing, and the Change Map does not pick the headline (§6.2). The inputs are those of §1.2, with no commit history. There is a known limit: the session that built a feature knows its headline better than mint's diff-only view does. That is the ground of §3.5, and this feature does not close it.

#### 3.4 Bodies that carry no headline

mint never writes or adds a headline: the AI writes one only through the notes prompt. The body selector has seven kinds (`rg -n '// Kind[A-Za-z]+:' internal/notes/select.go` → 7 lines), and only the AI-written body goes through the notes prompt. mint and its AI add no headline to the other six. A body the operator writes, a `--notes-file` body or notes written in the editor, carries one only if the operator writes it:

- the first release's fixed `Initial release.` body;
- the maintenance stub, for an empty post-exclusion diff;
- the commit-subject body under `--no-ai`;
- the commit-subject body when the AI fails under `on_notes_failure = fallback`;
- a `--notes-file` body, shipped as the operator wrote it;
- the body the operator chooses at the notes floor after the AI fails on an attended run (the commit subjects, or notes written in the editor).

Two more cases have their own sections. A full `[release].prompt` override owns its format and gets a headline only if it asks for one (§7.2). The saved part notes of a failed chunked run carry no headline (§8).

#### 3.5 Room for a supplied headline

This feature designs no route for a headline supplied by an operator or an outer agent. The headline, one plain sentence above the first section, is the slot any later route fills. The byte-identical rule is read as protecting what the AI or the operator wrote from mint's own edits and additions. mint never writes or edits a headline of its own. Nothing here rules out any of the three routes a later design could take: a steer handed to mint's AI, a finished `--notes-file` body, or a line the operator supplies, placed above the AI's body. Which route to take is the agent-authored release notes item's call. The review gate's `r` nudge already shows how a steer works: a one-time line appended to the resolved instructions, which flows into every AI call of that generation and is never written to config.

### 4. Section Placement

#### 4.1 Judged against the previous release

Every change is placed by comparing it with the previous release, never with earlier states inside this release. The previous release is the baseline a reader upgrading from it shares.

- **✨ Added**: something the previous release did not have. Its pieces are Added too, even where the diff shows them adjusted, capped or fixed along the way. A bug in something new in this release is never Fixed, and a tweak to it is never Changed.
- **🔧 Changed**: a rework or redesign of something the previous release already had. A genuinely new capability that ships inside the rework is Added.
- **🐛 Fixed**: something that behaved wrongly in the previous release and now behaves correctly, however the diff or its comments frame it. Behaving wrongly covers wrong output or placement, a crash or error, dropped or lost data, and something not working. This includes a rework whose purpose is the fix, and something an earlier release lost that this one brings back.
- **🗑️ Removed**: something the previous release had that is gone. A removal that breaks existing use goes under 💥 Breaking only (§4.2).
- **⚠️ Deprecated** and **🔒 Security** keep their opportunistic rule (§2.2).

#### 4.2 💥 Breaking

- **Only there.** A breaking change goes under 💥 Breaking only, and is not repeated in another section. The section comes first among the sections (§2.1), after the headline where there is one. Like every section, it is left out when empty.
- **What qualifies.** Something users already do stops working, or gives a different result they must act on. Examples: a removed or renamed command, flag, option, setting or key binding; a default changed so that existing setups behave differently in a way that matters; a changed output or file format that scripts or tools read; data or configuration from the previous release no longer read.
- **What does not.** A change that only adds, keeps more, allows more or improves is not breaking.
- **When unsure, it is not breaking**, and it goes in its usual section. The section sits at the top of the notes, so a false alarm stands in the opening lines. This is the same asymmetry as the headline's (§3.2): a missed flag costs what today's notes already cost, while a false one misstates the release where every reader looks.
- **What breaks, and what to do.** Each bullet says both in its one sentence. What to do comes only from what the diff shows, such as a new name, a replacement, or a setting that keeps the old behaviour. Where the diff shows none, the bullet says what breaks and stops there.

A reader upgrading must meet what breaks before anything else, which is why the section exists. The narrow definition keeps it from crying wolf at the top of the notes.

### 5. What the Notes Claim

#### 5.1 Only what users meet

- **In:** what a user of the software meets. That is what the software does, its commands, options, configuration, output, errors, performance and compatibility.
- **Out:** how the software is built and maintained. That covers CI and release tooling, linting, build scripts, test suites and harnesses, internal packages, modules and refactors, internal flags and plumbing commands, data-migration numbering, contributor docs, and repo housekeeping such as restructuring the README. A new internal package is not a feature: the notes describe what it lets users do, or leave it out.
- **The test is whether a user meets or notices the change**, not whether the code behind it is internal. A speed-up users notice belongs to them and is listed. Where the shipped files are the product (prompts, skills or templates the software ships and uses), changes to those files count.
- **In the user's terms.** Every change is described as users see and use it, never in the code's names, the mechanism, or a design document's jargon.
- **A release with nothing users meet.** Where every change in the release is one this section leaves out, the notes are one plain line saying the release is maintenance with no user-facing changes, in the shape of the empty-diff stub (`Maintenance release — no notable source changes`), with no headline and no sections. On the chunked path only the merge can say this of the release: a part whose slice holds nothing users meet returns one line saying its part holds nothing for the notes, and the merge writes the maintenance line only when every part returned one (§8). When unsure whether a change is one users meet, the change is listed in its section and neither line is written: a line left out costs what today's notes cost, while a false one drops a change users meet and tells every reader, in the release's only line, that nothing changed for them. The notes are never empty: the transport rejects an empty body as a failed call.

#### 5.2 True to the release

- **No behaviour is claimed from documentation, READMEs, comments, examples or tests alone.** These files describe behaviour, and editing them does not change it. The exception is files that are the product (§5.1), where a change does change it. A change to tests alone is not a fix.
- **Work built in this release is never presented as a change to, or fix of, the previous release.** §4.1 places it.
- **Nothing the previous release already had is presented as new.** That holds whether the diff shows it in unchanged context lines, in removed lines, or in code the diff only moves or renames.

**No new inputs.** The previous release's state is already in front of the AI, in the diff's removed and context lines. E1's judges called only 3 of the 47 accuracy residuals in the rewritten notes input-absent (`jq -c '.residuals["true to the release"]' .workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/data/scores.json` → `{"verdict":"needs more prompt work","input_absent":3,"instances":47}`). Three of their "the previous release already had it" calls were spot-checked against the inputs, and each fact sat in the diff's removed lines. So the fix is how the prompt gets the diff read, not what mint sends. The wording carries it, held to §10.

### 6. Ordering and the Change Map

#### 6.1 Ordering within sections

Within each section, changes are ordered by what they mean to the user, most important first. Weight is how much of the product a change touches, how often users meet it, and how much it changes their experience. A change to something users do every day outranks a change to a diagnostic or rarely used command. The pieces of the release's main work lead their sections. Lines, files, commits, and how easy a change is to name are not weight. The audit traced today's inversions within sections to exactly those stand-ins (granularity and nameability), which is why the rule names weight in the user's terms.

#### 6.2 The Change Map is coverage, not ranking

- **It stays in every notes call, built as today.** That holds for single-pass, chunk, piece and merge calls alike, with no change to its git calls, detection or rendering.
- **Its role** is a check on where the changes landed, which the AI uses so that nothing large is missed. It is never a measure of importance, for the headline (§3.3) and for ordering (§6.1) alike. Every change is described from the diff. The map is metadata, never content.
- **The default prompt's ranking rule goes.** "New structure (a whole new package or directory) is the strongest signal — weight it above raw size" is replaced by the coverage role. The merge framing's "rank by the Change Map above" goes too (§7.1).
- **mint's own descriptions follow the role.** Wherever mint's text calls the map a salience or priority signal, the description changes to coverage: the prompt's description of the map, and the doc comments on how it is built, composed and generated (`rg -n -i 'salience|prioriti' internal/notes --glob '!*_test.go' | wc -l` → 11).
- **Detection is not fixed.** New-directory detection reads only a path's first segment (`grep -n "func leadingDir" -A 6 internal/notes/changemap.go` → returns `path[:i]` at the first `/`), so a new package under an existing `internal/`, `src/` or `skills/` never registers. The audit found the novelty line fired in 6 of 31 releases and pointed the wrong way each time. The largest-file line is often a test, a deletion or a dev script. Making detection find new packages at any depth would make it fire on exactly the internal packages §5.1 keeps out of the notes, and it would still flag novelty rather than what a release changes for its user.

### 7. Shared Wording and Prompt Overrides

#### 7.1 mint's shared wording stays format-neutral

Some text is mint's own, and goes on every repo's notes calls whatever the instructions are, including under a full `[release].prompt` override:

- **The closing reminder.** It closes the single-pass prompt, every chunk and piece call, and the merge call (`rg -n 'OutputReminder' internal/notes/chunk.go internal/notes/prompt.go | grep -v '//'` → `chunk.go:543`, `chunk.go:567`, `prompt.go:83`, `prompt.go:101`).
- **The chunk and piece framings, and the merge framing.** Each tells the model to write "in the format above" (`rg -n 'in the format above' internal/notes/chunk.go | wc -l` → 3).

All of it stays format-neutral. Every headline rule, and every rule in §2–§6, lives in the default prompt alone.

- **The reminder** becomes: "Now output ONLY the release notes themselves, in the format above, exactly as they should be published: no preamble, no commentary, no explanation, no code fences, nothing before or after the notes." Its job is the same on every repo: the body is the notes and nothing else. It is the hardening against models narrating before the notes, which has been observed in real `mint commit` runs. "The format above" means whichever instructions govern. On the default prompt, that is a format that opens with the headline.
- **The wording is not E1's.** E1's reminder named the headline ("the one-sentence headline if this release has one, then the sections"). On an override repo, that line would tell the model to put a headline above sections its format may not have. Whether the model keeps the headline under the neutral wording is measured under §10. What would reopen this: evidence that the model keeps a headline only when the last thing it reads names one.
- **The merge framing** drops "rank by the Change Map above" (§6.2), so ranking belongs to the governing format. Its other instructions stay: combine related changes across parts into single bullets, and drop duplicates. It gains one, for parts that held nothing (§8).
- **The chunk and piece framings** gain format-neutral instructions of their own (§8).

#### 7.2 What reaches which repo

- **The default prompt, with or without `[release].context`.** These repos get the headline and every rule in §2–§6. `[release].context` is appended under its labelled header and leaves the default's rules intact.
- **A full `[release].prompt` override.** This is the operator's format. It gets no headline unless the operator's prompt asks for one, and nothing mint adds to its calls asks for one. The only change that reaches these calls is the shared wording in §7.1.

### 8. The Chunked Path

A release whose diff is too large for one call is written in parts and then merged. Each chunk or piece call carries the release's instructions, the whole-release Change Map, a framing that names its part, its slice of the diff, and the closing reminder. The merge call reads every part's notes in part order and writes the release's notes.

- **Part calls write no headline. The merge alone writes it**, judging the whole release from every part's notes and the whole-release Change Map. A part sees one slice. It cannot know whether that slice is the release's main work or a footnote, and the merge is the only call that sees the whole.
- **The part instruction is format-neutral (§7.1).** The chunk and piece framings ask the part to leave out anything that sums up the release as a whole, because the merge writes that. On the default prompt this means the headline. Under an override whose format has no such line, it asks nothing. They also tell a part whose slice holds nothing the notes would list to return one line saying so. That line is about the part, never the release, so the part is never empty: the transport rejects an empty body as a failed call, which fails the part.
- **The merge leaves those lines out.** Where every part returned one, it writes what the governing format gives a release with nothing to list: on the default prompt, §5.1's maintenance line. The merge framing says this in format-neutral terms.
- **The saved part notes carry no headline.** When the merge fails, the file's rule stands as it is: each finished part's notes byte for byte, in chunk order, with a blank line between parts, and a `**Notes missing:**` line on top when any slice has none (`grep -n "func joinChunkNotes" -B 2 internal/notes/chunknotes.go` → "keeps each part's notes byte for byte, with one blank line between one part's notes and the next"). This feature changes what the part calls return, not the file's rule. So the file holds the parts' sections, and any part's line saying it held nothing for the notes, which the operator removes while tidying, as with the `**Notes missing:**` line. mint's guidance on releasing a saved file tells the operator to delete these lines as well as the marker. A release shipped from the file, by `--notes-file` or picked up with `e`, carries a headline only if the operator writes one while tidying. That is the doubt rule (§3.2) on a path where no call saw the whole.
- **Every regenerate goes through the merge.** On chunked notes, `r` re-runs only the merge. A merge re-run from saved part notes also goes through the merge. So the headline, and any one-time nudge such as "lead with the auth work", stay with the call that writes the headline.

### 9. The Provider Release Title

The provider release title stays the bare tag on the forward release and on both regenerate paths (`rg -n 'CreateRelease\(ctx, tag, tag' internal/engine/release.go` → line 786; `rg -n 'DispatchRelease\(' internal/engine/regenerate_batch.go internal/engine/regenerate_write.go` → `res.Tag, res.Tag` at `regenerate_batch.go:361`, `req.Tag, req.Tag` at `regenerate_write.go:200`). The headline lives only in the body, where it renders as the first line under the title.

Lifting the headline into the title is ruled out for four reasons:

- Titles would vary with whether the AI wrote a headline. The bodies of §3.4, override repos and every release the doubt rule leaves without one have no headline, so the releases list would mix summaries with bare numbers.
- mint would have to read into a body it otherwise passes through untouched.
- Both regenerate paths re-publish with a title, so regenerating would retitle releases that are already out.
- The gain is confined to the releases list, because the release page already shows the headline as the first line under the title.

The bare-tag title also keeps `mint release regenerate --source tag`'s rebuild of the provider release from the tag annotation body deterministic and free of parsing.

### 10. The Shipping Bar

#### 10.1 The bar

The rewritten default prompt ships when every count is at least as good as today's prompt's, each count on its own, and every failure §10.3 says holds shipping has been cleared. The counts are taken on E1's 26 held-out releases, two repeats each (52 bodies), judged blind with E1's rubric and judges, and scored as E1 scored them. Today's prompt was measured on the same inputs:

| Count | Bar: today's prompt | E1's frozen candidate |
|---|---|---|
| Clean lead | at least 21 of 52 | 38 |
| Wrong-subject headline | none | 0 |
| Inversions | at most 30 | 16 |
| Misfiled | at most 84 | 78 |
| Duplicates | at most 29 | 19 |
| Internal items | at most 77 | 28 |
| False claims | at most 25 | 34 |
| In-release-as-change | at most 22 | 13 |
| Breaking changes flagged | at least 1 of 10 | 8 |
| Missing items | at most 9 | 13 |
| Format slips | at most 5 | 5 |

Measured with `jq -c '.field_counts' .workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/data/secondary.json` → current `inversions 30, misfiled 84, duplicates 29, internal 77, false_claims 25, in_release_as_change 22, missing 9, format_slips 5`; new `16, 78, 19, 28, 34, 13, 13, 5`. Also `jq -c '[.lead.current.clean, .lead.new.clean, .breaking.current.flagged, .breaking.new.flagged]' …/data/secondary.json` → `[21,38,1,8]`, and `jq -c '.classes["lead and headline"].new_wrong_subject' …/data/scores.json` → `0`.

- **No netting.** E1's own rule scored false claims and in-release-as-change as one class. That class sat level at 47 while false claims rose from 25 to 34. Here each count clears its own line.
- **The headline's line.** Today's notes have no headline to compare against, so for the headline "at least as good" means no wrong-subject headline.
- **Why this bar.** The rules are decided, and E1 shows them moving the notes the right way on nearly every count. The judges preferred the new notes on 21 of 26 releases, and the user, reading blind, on 4 of 6. The one shipping outcome that would be wrong is a regression. This bar rules out every regression while the remaining gap is wording. E1's pre-registered thresholds and their successor-experiment branch do not gate shipping. E1's measurement carries on as the acceptance check.

#### 10.2 Tuning and acceptance

- **Starting wording.** E1's frozen candidate, `new-v3` ([prompt](../../experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3.md), SHA-256 `c1f149842689ba5f6b7338579514258a0fa23416c04421253eacfaae62630a0f`), is the starting point. Where it and §2–§6 differ, what ships follows this specification. Its rules differ here:
  - It looks for what the previous release already had only in unchanged context lines and in code the diff moves or renames; §5.2 adds the diff's removed lines (`rg -n 'unchanged context lines|removed lines' .workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3.md` → line 98 only, no removed-lines hit).
  - It has no rule for a release with nothing users meet; §5.1's maintenance line is added (`rg -n -i 'maintenance|user-facing changes' .workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3.md` → no output).
  - It asks every breaking bullet to "Say what breaks and what to do"; §4.2 takes what to do only from what the diff shows (`rg -n 'what to do' .workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3.md` → line 87).
  - It bars a lesser second subject only where "And" joins it; §3.2 bars any second subject that is not a co-equal body of work (`rg -n '"And" in a headline' .workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3.md` → line 44).

  Its internal-efficiency leave-out line is a tuning point (§10.3). Two more parts of what ships are not new-v3's: the closing reminder is §7.1's format-neutral wording, not new-v3's headline-naming one, and the chunk, piece and merge framings follow §7.1 and §8. E1 never exercised the chunked path.
- **Tuning** happens during implementation, on E1's releases. The wording stays general: no prompt text names, quotes or paraphrases a change from the 31 sampled releases, and any example added is invented (the three shipped altitude examples are exempt). That generality, together with a bar that asks for no worse than today rather than a tuned threshold, is the guard against wording fitted to the releases it is tested on.
- **Acceptance** is one pass of the wording as it ships, frozen when tuning ends, over the 26 held-out releases, two repeats each: the rewritten default prompt, closed by §7.1's format-neutral reminder. It is otherwise run as E1 ran it: E1's harness, the same inputs, the same AI command and model, E1's rubric and judge grouping, and E1's scoring. It sits outside mint's test suite, which never spawns a real AI command.

#### 10.3 Where the tuning goes

- **False claims** are furthest from the bar (34 against at most 25), and then **missing items** (13 against at most 9). Both must come back to the bar while every other count holds. E1's candidate dropped a speed-up users notice under its "internal efficiency a user would not notice" line, which is why §5.1 draws the line at what users notice.
- **The neutral reminder** (§7.1) is untested. The first runs check that the headline survives it.
- **The chunked path** is untested: E1 ran every release as one call. The first runs also take held-out releases through the part calls and the merge, and check that the part notes carry no headline and that the merge writes the headline §3.2 calls for. A headline in any part's notes, or a merged headline that names something other than the release's main work, holds shipping until the wording clears it: today's notes carry neither, so either is a regression. A merge that leaves out a headline §3.2 calls for costs what today's notes cost; it does not hold shipping and is tuned like the known weak spots below. The same runs check the parts that hold nothing users meet, on at least one release cut so that a part's slice holds nothing users meet: that part returns its one line rather than an empty body (§8), and the merge leaves the line out beside other parts' sections and writes §5.1's maintenance line only when every part returned one. Merged notes that carry a part's line or the maintenance line while any part listed changes users meet hold shipping until the wording clears it, and so does a part that returns an empty body and fails the run: today's notes have none of these. A part that lists its internal changes instead costs what today's notes cost, and is tuned like the known weak spots below.
- **A release with nothing users meet** gets a body new-v3 never wrote (§10.2). The first runs also take at least one such release through a single call, and through the part calls and the merge with every part's slice holding nothing users meet, and check that the notes are §5.1's maintenance line. A headline on that release, or a single call or merge that returns an empty body and fails the run, holds shipping until the wording clears it: today's notes have neither. Notes that list the release's internal changes instead cost what today's notes cost, and are tuned like the known weak spots below.
- **Known weak spots** are wording only, and the rules stay unchanged:
  - two-body releases losing their headline, or getting half of one (§3.2);
  - a rework of existing behaviour filed under ✨ Added (36 in today's notes, 34 in E1's candidate);
  - 💥 Breaking misfilings, and false breaks that assert a previous state the diff contradicts.

#### 10.4 Alongside the counts

The read by eye on real releases stays alongside the counts, outside the bar. The counts weigh an error at the top of the notes the same as one at the bottom. Both releases where the user, reading blind, preferred today's notes carried the new prompt's errors in their opening lines.

---

## Working Notes
