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
- **Bodies pass through verbatim.** mint never parses, reformats or adds to a notes body. The headline exists only where the AI writes it (§3.4).
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
- **Nothing else.** The headline is the only thing that may appear above the first section: no summary paragraph, no TL;DR, no label, no other preamble. The default prompt's "start directly with the first section header" gives way to this headline-or-first-section rule, which still forbids every other preamble. The notes carry no wrapper, markers or machine-parseable labels, and no commentary or sign-off.

#### 2.2 Bullets (unchanged)

- Each bullet is one line and one sentence: the change in plain language, with its user-facing effect after an em dash where that helps. No second sentence, no bold lead-in, no sub-bullets, no implementation detail, and no rationale for how it was built.
- The shipped prompt's three altitude examples stay as they are.
- One bullet per notable change, never one per file, hunk or commit. The combine rule stands as worded: "Combine closely-related changes into a single bullet." Closely related means the small tweaks of one change, such as three commits that fix one gate. It never means a feature's separate capabilities: each piece a user can use or meet on its own is a notable change and gets its own bullet. Version bumps and other bookkeeping churn are ignored. Most releases are a handful of bullets, and the notes are not padded.
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
- **Two co-equal bodies are named together.** Where two bodies of work of equal weight each pass the test, the one sentence names both. That is at most two, still in one sentence, and still led by the benefit. "And" in a headline joins only two co-equal bodies, never two pieces of one body and never a main body plus a lesser piece. A release with more than two co-equal bodies is not designed for: the one-sentence limit stands, and the model handles the case as best it can.
- **No headline where one bullet already states the whole release** and nothing else stands beside it. There the headline would only repeat that bullet. The test is whether a single bullet states the whole release, not how many bullets there are. Where the main work is spread over several bullets and none of them names the whole, the headline stays even when the notes hold nothing else, because it is the one line that says what the release is. A main feature written as a single bullet beside unrelated changes keeps its headline too, since there is a rest to set it apart from.
- **No headline on a grab-bag.** A release of small unrelated changes has no main body of work, however large the largest of them, so there is nothing to sum up. Its notes start with the first section. A theme stitched across such changes is the narrative thread §3.1 rules out.
- **When in doubt, leave it out.** The AI writes a headline only when one body of work (or two, as above) clearly stands out. Otherwise the notes start with the first section. A missing headline costs no more than today's notes do. A wrong one misstates the release in its most-read line, and on an unattended run (`mint release -y`, as agent sessions run it) nobody reads the notes before they ship. Accepted cost: some releases that deserved a headline go without one. On an attended run the review gate's existing correctives remain: `e` to edit, and `r` to regenerate with a one-time nudge such as "lead with the auth work".

#### 3.3 How the headline is picked

The AI picks the headline by what the release changes for its user, read from the diff. A new folder or a large file is not, of itself, the release's main thing, and the Change Map does not pick the headline (§6.2). The inputs are those of §1.2, with no commit history. There is a known limit: the session that built a feature knows its headline better than mint's diff-only view does. That is the ground of §3.5, and this feature does not close it.

#### 3.4 Bodies that carry no headline

The headline exists only where the notes prompt writes it. mint never writes or adds one. The body selector has seven kinds (`rg -n '// Kind[A-Za-z]+:' internal/notes/select.go` → 7 lines), and only the AI-written body goes through the notes prompt. The other six carry no headline:

- the first release's fixed `Initial release.` body;
- the maintenance stub, for an empty post-exclusion diff;
- the commit-subject body under `--no-ai`;
- the commit-subject body when the AI fails under `on_notes_failure = fallback`;
- a `--notes-file` body, shipped as the operator wrote it;
- the body the operator chooses at the notes floor after the AI fails on an attended run (the commit subjects, or notes written in the editor).

Two more cases have their own sections. A full `[release].prompt` override owns its format and gets a headline only if it asks for one (§7.2). The saved part notes of a failed chunked run hold sections only (§8).

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
- **What breaks, and what to do.** Each bullet says both in its one sentence.

A reader upgrading must meet what breaks before anything else, which is why the section exists. The narrow definition keeps it from crying wolf at the top of the notes.

---

## Working Notes
