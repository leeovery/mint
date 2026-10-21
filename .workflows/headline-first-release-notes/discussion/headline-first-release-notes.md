# Discussion: Headline First Release Notes

## Context

Release notes should open by naming the release's main feature, with the smaller changes that shipped alongside it listed after. Today a release's main feature can land as one bullet among sideline changes, sorted into its category like any other — mint's own 0.0.6 changelog section is the example: `mint setup`, the release's reason to exist, is the first of four ✨ Added bullets, level with "`internal/configtest` package — a shared test-support seam".

Pulled from the product roadmap (horizon Next), parked there on 2026-10-16 during the token-aware-diff-sizing discussion, deliberately separate from big-release handling. That feature has since shipped on the existing format; its chunked merge pass inherits whatever format exists, so this work's format change reaches it.

### Current state (as built, confirmed 2026-10-21)

- **The default format is category-first.** `DefaultPrompt` (`internal/notes/prompt.go`) asks for Keep a Changelog sections with emoji headers in a fixed order (✨ Added, 🔧 Changed, ⚠️ Deprecated, 🗑️ Removed, 🐛 Fixed, 🔒 Security), empty sections omitted, one-line one-sentence bullets, one bullet per notable change, ranked by the Change Map (new structure weighted above raw size). Ranking happens within categories; nothing surfaces the top-ranked change above them.
- **Two rules forbid anything above the first section.** The prompt says "NO TL;DR, NO summary paragraph, NO preamble — start directly with the first section header" (`rg -n 'NO TL;DR' internal/notes/prompt.go` → line 57). `OutputReminder`, appended after the diff in every notes call (single-pass, chunk and merge alike, and under a full `[release].prompt` override), restates "no preamble … nothing before or after the notes" (`rg -n 'nothing before or after the notes' internal/notes/prompt.go` → line 83). A headline has to be reconciled with both — as written they forbid it as a preamble. The reminder exists because models drift into narrating before the notes (observed in real `mint commit` runs); whatever the headline becomes, that hardening must survive.
- **Bodies are byte-identical.** mint never reformats or injects into the notes body, so a headline has to come from the AI's output, not be added afterwards. Nothing in mint parses the notes structure: the transport's validation is minimal (empty or refusal), and the body ships verbatim to the annotated tag, `CHANGELOG.md` (under mint's own `## [x.y.z] - date` header) and the provider release body.
- **The provider release title is the bare tag.** Forward release and both regenerate paths pass the tag as the title (`rg -n 'CreateRelease\(ctx, tag, tag' internal/engine/release.go` → line 786; `rg -n 'DispatchRelease\(' internal/engine/regenerate_batch.go internal/engine/regenerate_write.go` → both pass `res.Tag, res.Tag` / `req.Tag, req.Tag`).
- **The chunked path carries the release's instructions into every call.** Chunk calls and the merge call both carry the default prompt (or the operator's override) plus `context`, the whole-release Change Map and mint's framing; each framing says "in the format above" (`rg -n 'in the format above' internal/notes/chunk.go | wc -l` → 3). The merge framing asks it to combine related changes across parts, drop duplicates and rank by the Change Map — the merge is where the whole release is ranked. A format change to the default prompt therefore reaches chunk calls and the merge.
- **Prompt control.** `[release].prompt` (a file) replaces the default wholesale and ignores `context`; `[release].context` (string or file) is appended to the default under a labelled header. A format change reaches only repos on the default prompt.
- **Today's manual route to a headline.** On an attended release the review gate's `r` regenerates with a one-time nudge — the recorded example is "lead with the auth work". It is the only way to steer emphasis, and only at a terminal; on chunked notes `r` re-runs only the merge. `e` edits the body in `$EDITOR`.
- **Non-AI bodies.** `--no-ai` and the notes-failure fallback ship the commit-subject list; an empty post-exclusion diff ships the fixed stub "Maintenance release — no notable source changes"; `--notes-file` ships a finished body verbatim. None goes through the notes prompt.

### History: the TL;DR that was built and removed

A headline is not new ground — the format once had one, and lost it for length.

- **The original quality bar is this feature's goal.** The mint-release-tool specification's success criterion for the notes: "on a big release, the headline feature leads the notes", aimed at by the Change Map (new structure as the headline signal) plus forced Keep a Changelog classification, judged by eye on real releases with the prompt tuned when the criterion isn't met.
- **That design carried a TL;DR.** The specification and the release-notes-quality discussion retained "a TL;DR one-liner at the top (may be multi-line) — what the release is really about", framed as mint's value-add: "a unified cross-change narrative synthesised from the whole diff", sitting above the categorised sections. The first default prompt (737e84dc, 2026-06-10) asked for exactly that — "not a list; it is the through-line that ties the changes together" — and told the AI to "bold the notable features and describe them — celebrate them, do not bury them in a flat list".
- **It was removed three days later.** 886c6100 (2026-06-13, "right-size release notes"; `git log -S "NO TL;DR" --format="%h %ad %s" --date=short -- internal/notes/prompt.go` → `886c6100 2026-06-13 fix(presenter,notes): right-size release notes; clearer gate prompt`): notes were essay-length — "a TL;DR narrative paragraph plus bold, multi-sentence, rationale-laden bullets (and ~30s to generate)". The rebalance brought in the one-line one-sentence bullet rule, the worked altitude examples and "NO TL;DR, NO summary paragraph, NO preamble". mint's own 0.0.2 changelog section records it: "The release notes prompt now asks for concise one-line bullets per change with no TL;DR, so changelogs read at a glance."
- **What this leaves.** The removal fixed length and took the only place the main feature could stand apart with it. Whatever this design puts above the sections has to win the headline back without reopening the essay: the failure was a narrative through-line inviting a paragraph, not the existence of a lead.

### Ground from the token-aware-diff-sizing discussion

- Stitching and Salience framed the chunk problem as "no chunk knows whether its slice is the headline or a footnote"; the merge call is where the whole release is ranked.
- When a Chunk Fails rejected merging surviving chunks with a gap because "the missing slice can be the release's headline feature" — notes that read as complete while leaving out the biggest change are named there as "the one the parked headline-first roadmap item targets".

### Evidence: an audit of 31 released notes (2026-10-21)

Mid-discussion the user asked to move forward from evidence: some releases' notes "really don't portray what happened, and definitely not in the right order". Six read-only auditors compared 31 AI-written releases across nine projects on this machine (all on the default prompt, all since the 2026-06-13 format) with their code, judging each release's real main work from full history. Full method, synthesis and per-release reports: [Release-notes audit](../imports/release-notes-audit.md). Judgement-based, not a controlled measurement. In brief:

- **Leading:** of 30 releases with a main body of work, the notes led with it cleanly in 17, partly in 7, not at all in 6.
- **Fragmentation is near-universal:** the main work split over 4–14 bullets across two to four sections in all but one; almost never does a line state the whole; items stated twice in five. The model over-splits — it never collapsed a large feature into one bullet.
- **The fixed section order buries reshaping work:** a main work that reshapes existing behaviour lands in 🔧 Changed below ✨ Added, so small new items outrank it (eight releases); twice the model broke the order to compensate.
- **Inversions within sections** in about half — rank follows commit/file granularity and how nameable an item is, not user weight; README prose weighted as content.
- **The Change Map's structural signals barely work:** first-segment-only novelty fired in 6 of 31, never on the whole main work — on a pre-existing `design/` (false novelty), a docs site that then led, README demo assets, a 6-line wrapper, the smaller of two co-equal bodies. The largest-file line is often a test, a deletion or a dev script. The prompt's "new structure is the strongest signal" turned new internal packages into feature bullets (mint v0.0.6).
- **The headline** under the decided rules applies in about 23 of 31. Four of six auditors independently flagged the only-the-main-thing exemption as suppressing it exactly where the main work is most fragmented.
- **Beyond salience:** false or stale claims shipped (~7 — behaviour invented from README, comment or test edits; old features re-announced); work new in the release presented as a change or fix to the last (~7); internal and dev-only changes presented to users (~19 of 31); section misfiling (~15); breaking changes buried (5); missing items (~6).
- **Outside notes design:** a mint exclusion bug (`diff_exclude` globs passed as plain `:(exclude)` pathspecs; `**/testdata/**` misses a root `testdata/` — 172 files leaked on switchboard v0.1.0, `git diff --name-only v0.0.6 v0.1.0 -- . ':(exclude)**/testdata/**' | grep -c '^testdata/'` → 172, 0 with `:(exclude,glob)`); the largest release in five of six groups never reached the AI (over the limit — the gap token-aware diff sizing closes); CHANGELOG drift after regenerate in stitch-cli.

### Adjacent roadmap item (not pulled)

**Agent-authored release notes** proposes taking a one-line statement of what the release is about from the outer agent — the session that built the feature knows the headline better than mint's diff-only view. Pulled alone, this work designs the format and how mint's own AI fills it; whether a supplied statement overrides the AI's pick is that item's question, and the format should not foreclose it.

### Open questions carried in from discovery

- What a headline looks like in the format — a lead line or short paragraph above the categorised sections, or the main feature pulled out of its category to the top — and how it squares with the one-line-bullet discipline and the no-preamble rule.
- Releases with no single main feature (maintenance, grab-bag) — whether the notes then omit the headline rather than invent one.
- How the AI picks the headline — from the diff and Change Map alone, with the known limit that the building session knows the headline better than the diff does.
- A supplied headline — the format should leave room for it without designing it.
- Reach beyond the default prompt — overrides replace the default wholesale.

### References

- [Discovery session 001](../discovery/sessions/session-001.md)
- [Token-aware diff sizing discussion](../../token-aware-diff-sizing/discussion/token-aware-diff-sizing.md) — Stitching and Salience; When a Chunk Fails; Summary → Open Threads
- [Mint release tool specification](../../mint-release-tool/specification/mint-release-tool/specification.md) — Stage 4: success criterion; default notes format (the TL;DR as specified)
- [Release-notes quality discussion](../../mint-release-tool/discussion/release-notes-quality.md) — the TL;DR retained above the KaC sections; the success criterion
- Commits 737e84dc (first default prompt, with the TL;DR) and 886c6100 (TL;DR removed, one-line bullets)
- `internal/notes/prompt.go` — `DefaultPrompt`, `OutputReminder`, `ComposePrompt`, `ResolveInstructions`
- `internal/notes/chunk.go` — chunk and merge framing
- `CHANGELOG.md` — as-shipped notes in the current format

---

*Subtopics are documented below as they reach `decided` or accumulate enough exploration to capture. Not every subtopic needs its own section — minor items resolved in passing can be folded into their parent. The Discussion Map (which subtopics exist and their states) lives in the manifest, not this file.*

---

## Scope

### Context

The work unit came off the roadmap as headline-first notes alone. Mid-discussion the user widened the frame: the notes are one body used on three surfaces (tag, changelog, provider release), adding a summary is one way to improve it, and "we could expand this into improving it in general" — some releases' notes "really don't portray what happened, and definitely not in the right order", with small stuff at the top and big stuff lost down the list. The audit (Context → Evidence) was run to decide the width from evidence.

### Options Considered

**Headline only** — the roadmap item as pulled.
- Cons: leaves the audit's accuracy failures shipping in the same notes, and a later fix rewrites the same prompt again.

**Headline plus ordering** — the user's stated complaint.
- Cons: same as above for accuracy, which the audit found to be the larger defect after scatter.

**Headline, ordering and the accuracy failures** — everything whose fix lives in the notes prompt or the signals mint hands the AI.

**Pivot to an epic** (one topic per failure class).
- Cons: ceremony for what is one prompt and one output; the Discussion Map carries the parts.

### Journey

The audit's ranking of defects: scatter of the main work and the missing statement of the whole first (the headline's ground); then accuracy — false or stale claims (~7 of 31), in-release work presented as a change or fix to the last release (~7), internal and dev-only changes shown to users (~19), section misfiling (~15), buried breaking changes (5); ordering within sections real but modest. Most accuracy failures share one cause: from the diff alone the AI cannot tell new behaviour from newly documented behaviour, nor what a user meets from what only the code sees. The session argued for taking them in: they ship in the same notes, their fixes are rules in the prompt this feature already rewrites, and they are judged the same way — by eye on real releases — so splitting them off means rewriting that prompt twice. One feature, not an epic. The user agreed: "bring it all in".

### Decision

The feature covers what the notes say and in what order, wherever the fix lives in the notes prompt or in the salience signals mint hands the AI:

- the headline (Headline Shape, No Clear Headline, Picking The Headline);
- ordering within sections, and the salience signals behind it (the Change Map's novelty and largest-file lines, the prompt's new-structure rule);
- section placement — misfiling, reshaping work buried under the fixed section order, the main work scattered and stated twice;
- user-facing only — internal and dev-only changes kept out, phrasing in the user's terms rather than mechanism or spec jargon;
- true to the release — no behaviour claimed from docs, comments or tests alone; work built in this release not presented as a change or fix to the last; nothing old presented as new;
- breaking changes flagged.

Out of scope, recorded where they go: the `diff_exclude` pathspec bug (a mint bug, handled on its own); the largest releases never reaching the AI (closed by token-aware diff sizing, on main and not yet released); CHANGELOG drift after regenerate. One feature, not an epic. The work unit keeps its name. Deciding factor: one prompt, one output, one way of judging it.

---

## Headline Shape

### Context

What the headline looks like in the notes, and how it squares with the format's one-line discipline and the removed TL;DR (Context → History). The line drawn at the outset: the headline is held to the bullets' altitude — one line, one sentence, plain language, the change and its user-facing effect — never a synthesis of everything in the release, because a narrative "what is this release about" is what grew into the essay that 886c6100 cut.

### Options Considered

Worked on mint's own 0.0.6 section, where `mint setup` is the first of four ✨ Added bullets.

**A — headline line above the sections, the headlined change moved out of its category**

```text
`mint setup` — a new command that prints a version-matched guide an AI agent follows to configure mint for a project.

✨ Added

- Config-metadata source of truth — one drift-tested table of every config key, its level, default and description.
- …
```

- Pros: says nothing twice; the top line is the one every reader reads.
- Cons: a reader scanning ✨ Added across versions doesn't find the biggest addition in its list.

**B — headline line above the sections, the change also kept in its category**

```text
`mint setup` — a new command that prints a version-matched guide an AI agent follows to configure mint for a project.

✨ Added

- `mint setup` — prints a version-matched configuration guide, usable from any directory.
- …
```

- Pros: the sections stay a complete record.
- Cons (as first argued): the same thing twice in five lines, against the prompt's "do not pad".

**C — the headline as its own emoji section** (e.g. "⭐ Highlight" with one bullet)

- Pros: keeps the format's visual grammar; a headed section is less likely to be read by the model as a preamble to drop.
- Cons: adds a header line; position is doing the work in every shape.

### Journey

The session's first lean was A, on the premise that the headlined change is one bullet, so keeping it in its category repeats that bullet. The user pushed toward B with an argument that breaks the premise: a release is usually — not always — primarily one thing, and that thing is built from many pieces. The next mint release is the instance: since v0.0.11 the work is almost entirely token-aware diff sizing (`git log v0.0.11..HEAD --format='%s' | sed -E 's/^([a-z-]+)\(([^)/]+).*/\1 \2/' | sort | uniq -c | sort -rn | head -3` → `318 impl token-aware-diff-sizing`, `129 planning token-aware-diff-sizing`, `109 discussion token-aware-diff-sizing`) — splitting a diff too large for the model so the AI can still write one coherent set of notes — with minor bug fixes and documentation updates alongside. In the sections that feature is not one bullet but many: chunking, halving a call the model rejects, the exclusion check, the notes-file route, saved part notes. There is no single bullet to move out; the headline summarises a body of work the sections detail.

So repetition is the design, not padding: the top line is the terse "sales piece" — what was done, short, addressing the feature, the bug or the large epic — and the sections are how the release got there, every piece listed as today. That is what moved the session off A: A only holds for a release whose main feature is a single change (0.0.6), and even there B costs one repeated line.

The next release, worked in the agreed shape:

```text
Release notes for diffs of any size — a diff too big for the model is split into parts, and their notes are merged into one coherent set.

✨ Added

- Oversized diffs are chunked into parts that each fit `max_prompt_bytes`, with one merge call writing the release's notes.
- A call the model rejects as too big is retried as smaller parts, and mint tells you the largest size that worked.
- Before the notes are written, mint flags files that don't belong in them (specs, design docs, tests) and records your answers in `.mint.toml`.
- `--notes-file` releases a finished body you already hold, with no AI call.
- A failed chunked run saves the part notes that finished, so they can be tidied and released.

🐛 Fixed

- …
```

**What the sentence leads with.** The user's framing included "the amount of work that's been done". The session pushed back on that half: asked to convey scale, the model writes an inventory with an adjective on the front — "A major rework of release-notes generation, adding chunking, size-based retries, an exclusion check and a notes-file route" — which is the TL;DR compressed into one long sentence, plus marketing inflation. What sells is what the reader can now do; the list beneath already shows how much work it took, and shows it better than "major". The user agreed: the headline leads with what the release gives its user, and the scale is left to the sections.

**Typography.** Plain, not bold: position and the blank line already set it apart, and in a raw `CHANGELOG.md` the asterisks are noise. Stated as the session's lean; the user agreed.

**Will the sections really carry the pieces?** (raised by the background review, review-001 F1). B's case rests on the main feature appearing in the sections as several bullets, but today's rules pull toward combining: the default prompt says "ONE bullet per notable change … Combine closely-related changes into a single bullet … do not pad", and the merge framing on chunked releases says "combine related changes across parts into single bullets, drop duplicates". A model that folds the whole feature into one bullet produces a headline plus a bullet restating it. The session's position, which the user agreed: keep the combine rule as it reads. "Closely-related changes" means the small tweaks of one change — three commits fixing one gate — not a feature's separate capabilities; chunking, halving a too-big call, the exclusion check and `--notes-file` are each something a user can use or meet, so each is a notable change in its own right, and 0.0.6 kept `mint setup`'s pieces apart under today's wording. What the prompt does need is to say the headline does not relieve the sections, so the model doesn't thin them because the top line summarised them. Where a model still folds the feature into one bullet, the cost is one repeated line — the cost already accepted for 0.0.6, so A-versus-B does not come back. What would reopen it: evidence of the model collapsing a big feature into one bullet on real releases, which would put the combine rule's own wording in question.

### Decision

A one-sentence headline above the first section, summarising the release's main body of work; beneath it the categorised sections exactly as today, listing every notable change, the main feature's pieces included. The headline and the sections overlap by design — summary, then detail — and the sections are never thinned to avoid it: the prompt says the headline does not relieve the sections. Today's combine rule stands as worded — it folds the tweaks of one change, not a feature's separate capabilities (resolves review-001 F1).

- **One sentence, at bullet altitude** — terse, plain language, no narrative through-line (the TL;DR failure 886c6100 cut).
- **Leads with what the release gives its user**, not the scope of the work: no superlatives or scale words ("major", "overhaul", "powerful"), no inventory of the pieces — the sections carry the scale.
- **The main thing can be any kind of change** — a feature, a fix, a large epic; the headline names it whatever its category.
- **Plain text**, unbolded, separated from the first section by a blank line.

Deciding factor: in the common release — one body of work built from many pieces plus sideline changes — the main feature is many bullets, not one, so the headline cannot be a lifted bullet; it has to be a summary, and a summary overlaps its detail. Confidence: high on the shape; the exact wording the prompt uses to get it is prompt-tuning, judged by eye on real releases as the original success criterion prescribes.

---

## No Clear Headline

### Context

Headline Shape makes the headline a summary of the release's main body of work, on the premise that a release is usually — not always — primarily one thing. This subtopic covers the "not always": releases where that body is missing (a grab-bag of small unrelated changes), too small to summarise (one or two changes), or doubled (two bodies of work of equal weight).

### Options Considered

For a release carrying two bodies of work of equal weight:

**1 — Name both in the one sentence**
- Pros: the headline tells the truth about what the release is; the reader gets the lead on exactly the releases where there is most to lead with.
- Cons: a sentence naming two things leans toward the inventory Headline Shape rules out; needs a cap.

**2 — Pick the bigger one; the other lives only in the sections**
- Cons: misleads — the headline claims the release is about one thing when it is about two.

**3 — Omit the headline whenever no single thing dominates**
- Cons: drops the headline on the releases where it helps most — the biggest ones.

### Journey

Worked instance: if headline-first notes is built before mint is next tagged, that release carries two co-equal bodies of work — token-aware diff sizing and this feature. Under option 1 it opens:

```text
Release notes for diffs of any size, now opening with a line that names what the release is about.
```

The session leaned 1, capped at two and still one sentence, on the costs above: 2 misrepresents the release, 3 punishes size. Three co-equal features would fail the one-sentence limit and read as a list; at mint's release cadence that is rare enough not to design for. The user agreed.

**A release that is only its main thing** (raised by the background review, review-001 F3). Small releases have a clear main thing, so the no-main-feature cases don't catch them, yet a headline there restates the whole body. mint's 0.0.11 is two bullets, both about `--dry-run --autostash`; under Headline Shape it would open:

```text
`--dry-run --autostash` previews a release against a dirty working tree and leaves the repo untouched.

✨ Added

- `--dry-run --autostash` now previews a release against a dirty working tree — the real stash is skipped and the repo stays unchanged.

🔧 Changed

- A dirty-tree dry run without `--autostash` still aborts at the clean-tree gate.
```

Three lines saying one thing. 0.0.7 (two bullets, both `mint setup`) is the same shape. The session's position: the headline exists to stand the main thing apart from the rest, so where the notes hold nothing else, it only repeats. Framed as that principle rather than a bullet count, because a count would wrongly drop the case this feature exists for — one big feature written as a single bullet beside four small fixes still has a rest to stand apart from, and still gets its headline. The alternative weighed was consistency — every release opening the same way, even when the top line repeats the only change; the user accepted the position over it.

**A wrong headline against no headline** (raised by the background review, review-001 F6). On an unattended release — `mint release -y`, which agent sessions already run — the notes go to the tag, `CHANGELOG.md` and the provider release with no one reading them first. Today a misjudged salience buries the main feature in a list: bad, but quiet. With a headline, the same misjudgement is stated in the line every reader reads — the diff-sizing release opening "Files that don't belong in release notes are flagged before the notes are written": true, and it names a side piece as the release. The costs are asymmetric: a missing headline costs no more than today's notes do; a wrong one misstates the release in its most-read line, uncaught. The session proposed biasing to omission; the user agreed. Accepted cost: some releases that deserved a headline won't get one — prompt tuning, judged by eye on real releases. The attended path keeps its existing correctives (`e`, and `r` with a nudge such as "lead with the auth work").

### Decision

A release with two bodies of work of equal weight names both in its one headline sentence — at most two, and still one sentence, benefit-led per Headline Shape. Beyond two co-equal bodies of work is not designed for: the one-sentence limit stands, and the case is left to how the model meets it (YAGNI at mint's cadence). Deciding factor: a headline that names one of two equal halves misrepresents the release, and omitting it costs the lead on the largest releases. Confidence: high. (Resolves review-001 F4, which raised the same case independently.)

A release whose notes hold nothing but its main thing carries no headline — the notes start with the first section, as today. The test is whether anything else is in the notes for the headline to stand apart from, not how many bullets the main thing takes: a single-bullet main feature beside unrelated changes keeps its headline. Deciding factor: the headline's job is separation; with nothing to separate from it is repetition, the padding the prompt forbids. Confidence: high. (Resolves review-001 F3.)

When in doubt, leave it out: the AI writes a headline only when one body of work (or two, per above) clearly stands out, and otherwise starts with the first section. Deciding factor: a missing headline costs no more than today's notes; a wrong one misstates the release in its most-read line, and on an unattended run nobody catches it. Accepted cost: occasional omission where a headline was deserved, tuned by eye. Confidence: high on the bias; the prompt wording that sets it is tuning. (Resolves review-001 F6.)

---

## Picking The Headline

### Context

What the AI judges the headline from. mint's notes AI reads the instructions, the Change Map and the post-exclusion diff — nothing else. Commit history is deliberately not an input: the release-notes-quality discussion dropped commit intent and made the diff the sole source of truth. In mint's own repo the planning and discussion trees are in `diff_exclude` (`.mint.toml`: `diff_exclude = ['.workflows', '.claude', '.tick', '.agntc', '.github']`), so the work that makes a feature's weight obvious to the people who built it never reaches the AI. The known limit — the session that built a feature knows the headline better than the diff does — is the supplied-headline question (Room For A Supplied Headline), not this one.

The Change Map orders salience novelty-first: new directories lead, weighted above magnitude (`internal/notes/changemap.go`), and the default prompt tells the AI "new structure (a whole new package or directory) is the strongest signal — weight it above raw size".

### Journey

Raised by the background review (review-001 F5): the session had identified the next release's main work by counting commit scopes, which the AI never sees.

As argued in the conversation, the session went further and claimed mint's strongest signal points the wrong way on the next release: the only directories new since v0.0.11 are two small helpers, `internal/bytesize` and `internal/cmdline` (436 added lines together), while the diff-sizing work lives in the existing notes package (9,749 of 40,888 added lines after exclusion), so the new-structure signal would hand the AI a confident wrong pick that When in doubt, leave it out (No Clear Headline) cannot catch.

*(Corrected 2026-10-21 — that claim was wrong about the mechanism. The Change Map's new-directory detection reads only a path's first segment (`grep -n "func leadingDir" -A 6 internal/notes/changemap.go` → returns `path[:i]` at the first `/`), so a new package under an existing `internal/` is never flagged. For the next release the novelty line is empty: the post-exclusion diff's top-level directories are `cmd` (A, M) and `internal` (A, D, M) — neither all-added. The largest file is `internal/engine/release.go` (877 lines changed). The dex-engine audit found the same: the new-directory line fired in 0 of 4 releases, including one with a brand-new `src/dex_engine/directives/` package, because all code sits under `src/` and `tests/`.)*

Corrected reading: the structural signal cannot pick a headline either way. Where code is nested, as in most repos, it is near-blind — it names no new structure, and its area rollup collapses to one or two top-level folders. Where it does fire, on a brand-new top-level directory, it flags novelty, not what the release changes for its user. The diff-sizing numbers still stand as evidence that the main work can sit in an existing package with nothing structural marking it.

The novelty-first weighting was built to stop a large new package being buried in the list; whatever it contributes stays with ordering within the sections. The headline is judged from the diff by user-facing weight. The alternative — rely on the signals plus the doubt bias — leaves the headline to a signal that is mostly silent. The user agreed (on the original argument; the correction strengthens rather than moves the decision).

Rejected path not reopened: feeding commit history to the AI (dropped in release-notes-quality; the diff stays the sole source of truth).

### Decision

The AI picks the headline by what the release changes for its user, read from the diff. The Change Map's new-structure signal keeps its role in ranking within the sections but does not pick the headline: a new folder is not, of itself, the release's main thing. Inputs are unchanged — diff and Change Map only, no commit history. Deciding factor: the structural signal cannot identify a release's main thing — it is near-blind where code is nested (it reads only a path's first segment) and flags novelty rather than user weight where it fires — while a headline turns any misread from a reordered list into a misstatement in the most-read line. Confidence: medium-high — the rule is clear; whether the prompt wording achieves it is judged by eye on real releases, this one first. (Resolves review-001 F5.)

---

## Summary

### Key Insights

### Open Threads

### Current State
