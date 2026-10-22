# Discussion: Headline First Release Notes

## Context

Release notes should open by naming the release's main feature, with the smaller changes that shipped alongside it listed after. Today a release's main feature can land as one bullet among sideline changes, sorted into its category like any other — mint's own 0.0.6 changelog section is the example: `mint setup`, the release's reason to exist, is the first of four ✨ Added bullets, level with "`internal/configtest` package — a shared test-support seam".

Pulled from the product roadmap (horizon Next), parked there on 2026-10-16 during the token-aware-diff-sizing discussion, deliberately separate from big-release handling. That feature has since shipped on the existing format; its chunked merge pass inherits whatever format exists, so this work's format change reaches it.

### Current state (as built, confirmed 2026-10-21)

- **The default format is category-first.** `DefaultPrompt` (`internal/notes/prompt.go`) asks for Keep a Changelog sections with emoji headers in a fixed order (✨ Added, 🔧 Changed, ⚠️ Deprecated, 🗑️ Removed, 🐛 Fixed, 🔒 Security), empty sections omitted, one-line one-sentence bullets, one bullet per notable change, ranked by the Change Map (new structure weighted above raw size). Ranking happens within categories; nothing surfaces the top-ranked change above them.
- **Two rules forbid anything above the first section.** The prompt says "NO TL;DR, NO summary paragraph, NO preamble — start directly with the first section header" (`rg -n 'NO TL;DR' internal/notes/prompt.go` → line 57). `OutputReminder`, appended after the diff in every notes call (single-pass, chunk and merge alike, and under a full `[release].prompt` override), restates "no preamble … nothing before or after the notes" (`rg -n 'nothing before or after the notes' internal/notes/prompt.go` → line 83). A headline has to be reconciled with both — as written they forbid it as a preamble. The reminder exists because models drift into narrating before the notes (observed in real `mint commit` runs); whatever the headline becomes, that hardening must survive.
- **Bodies are byte-identical.** mint never reformats or injects into the notes body, so a headline has to come from the AI's output, not be added afterwards. *(Amended 2026-10-22 — narrowed by Room For A Supplied Headline: mint never writes or edits a headline of its own; whether a line the operator supplies may be placed above the AI's body is left open for the agent-authored release notes item.)* Nothing in mint parses the notes structure: the transport's validation is minimal (empty or refusal), and the body ships verbatim to the annotated tag, `CHANGELOG.md` (under mint's own `## [x.y.z] - date` header) and the provider release body.
- **The provider release title is the bare tag.** Forward release and both regenerate paths pass the tag as the title (`rg -n 'CreateRelease\(ctx, tag, tag' internal/engine/release.go` → line 786; `rg -n 'DispatchRelease\(' internal/engine/regenerate_batch.go internal/engine/regenerate_write.go` → both pass `res.Tag, res.Tag` / `req.Tag, req.Tag`).
- **The chunked path carries the release's instructions into every call.** Chunk calls and the merge call both carry the default prompt (or the operator's override) plus `context`, the whole-release Change Map and mint's framing; each framing says "in the format above" (`rg -n 'in the format above' internal/notes/chunk.go | wc -l` → 3). The merge framing asks it to combine related changes across parts, drop duplicates and rank by the Change Map — the merge is where the whole release is ranked. A format change to the default prompt therefore reaches chunk calls and the merge.
- **Prompt control.** `[release].prompt` (a file) replaces the default wholesale and ignores `context`; `[release].context` (string or file) is appended to the default under a labelled header. A format change reaches only repos on the default prompt.
- **Today's manual route to a headline.** On an attended release the review gate's `r` regenerates with a one-time nudge — the recorded example is "lead with the auth work". It is the only way to steer emphasis, and only at a terminal; on chunked notes `r` re-runs only the merge. `e` edits the body in `$EDITOR`.
- **Non-AI bodies.** `--no-ai` and the notes-failure fallback ship the commit-subject list; an empty post-exclusion diff ships the fixed stub "Maintenance release — no notable source changes"; `--notes-file` ships a finished body verbatim. None goes through the notes prompt. *(Amended 2026-10-22 — the first release's fixed `Initial release.` body bypasses the prompt too: `rg -n 'KindFirstRelease: no prior tag' internal/notes/select.go` → line 21; No Clear Headline.)*

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
- **Handed to the experiment lab (2026-10-21) — awaiting E1** (notes-prompt-on-real-releases): whether a rewritten notes prompt meets the user's bar on these releases, and which failures need mint to hand the AI something new. Ordering Within Sections (with Change Map Signals), Section Placement, User Facing Only, True To The Release and Breaking Changes return here when its evidence lands.
  - *2026-10-22 — E1 concluded; the wait released.* Verdict by its pre-registered rule: short of the bar ([E1 report](../experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/report.md), `data/scores.json`). On 26 held-out releases × 2 repeats, the frozen rewritten prompt (`new-v3`) against today's: clean lead 21 → 38 of 52, with no wrong-subject or unexpected headline (every miss an omission, or one of two bodies named); inversions 30 → 16; misfiled + duplicates 113 → 97; internal items 77 → 28; breaking changes flagged 1 → 8 of 10 (the one class past its bar); false claims 25 → 34 and in-release-as-change 22 → 13 (true-to-the-release level at 47); missing items 9 → 13, breaking a guard rail, so the frozen prompt is not taken as tested. Judges preferred the new notes on 21 of 26 releases; the user, blind, on 4 of 6 — the two picks for today's notes both carry the new prompt's errors in the opening lines. Sorting the failed classes' residuals by whether the input held what the AI needed: none sorts to "mint must hand the AI more" (input-absent 0–3 per class); the previous release's state sat in the diff's removed and context lines, spot-checked on three false claims. The five subtopics are back with the discussion as ground to decide.
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

The audit's ranking of defects: scatter of the main work and the missing statement of the whole first (the headline's ground); then accuracy — false or stale claims (~7 of 31), in-release work presented as a change or fix to the last release (~7), internal and dev-only changes shown to users (~19), section misfiling (~15), buried breaking changes (5); ordering within sections real but modest. Most accuracy failures share one cause: from the diff alone the AI cannot tell new behaviour from newly documented behaviour, nor what a user meets from what only the code sees. *(Amended 2026-10-22 — E1 found the previous release's state in the diff in all but 3 of 47 accuracy residuals; the cause is the AI not using what the diff shows — True To The Release. The scope decision is unaffected.)* The session argued for taking them in: they ship in the same notes, their fixes are rules in the prompt this feature already rewrites, and they are judged the same way — by eye on real releases — so splitting them off means rewriting that prompt twice. One feature, not an epic. The user agreed: "bring it all in".

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

**Option C closed** (raised by the background review, review-001 F2, which found C's status unrecorded). The decided shape — a plain sentence above the first section — rules out a headed highlight section; it is also how the user's own hand-written switchboard v0.2.0 notes open (Context → Evidence). C's one recorded pro — a headed section is less likely to be dropped by the model as preamble — belongs to rewording the no-preamble rule and the closing reminder (Prompt Overrides And Reminder), not to the headline's typography.

**Will the sections really carry the pieces?** (raised by the background review, review-001 F1). B's case rests on the main feature appearing in the sections as several bullets, but today's rules pull toward combining: the default prompt says "ONE bullet per notable change … Combine closely-related changes into a single bullet … do not pad", and the merge framing on chunked releases says "combine related changes across parts into single bullets, drop duplicates". A model that folds the whole feature into one bullet produces a headline plus a bullet restating it. The session's position, which the user agreed: keep the combine rule as it reads. "Closely-related changes" means the small tweaks of one change — three commits fixing one gate — not a feature's separate capabilities; chunking, halving a too-big call, the exclusion check and `--notes-file` are each something a user can use or meet, so each is a notable change in its own right, and 0.0.6 kept `mint setup`'s pieces apart under today's wording. What the prompt does need is to say the headline does not relieve the sections, so the model doesn't thin them because the top line summarised them. Where a model still folds the feature into one bullet, the cost is one repeated line — the cost already accepted for 0.0.6, so A-versus-B does not come back. What would reopen it: evidence of the model collapsing a big feature into one bullet on real releases, which would put the combine rule's own wording in question.

### Decision

A one-sentence headline above the first section, summarising the release's main body of work; beneath it the categorised sections exactly as today, listing every notable change, the main feature's pieces included. The headline and the sections overlap by design — summary, then detail — and the sections are never thinned to avoid it: the prompt says the headline does not relieve the sections. Today's combine rule stands as worded — it folds the tweaks of one change, not a feature's separate capabilities (resolves review-001 F1).

- **One sentence, at bullet altitude** — terse, plain language, no narrative through-line (the TL;DR failure 886c6100 cut).
- **Leads with what the release gives its user**, not the scope of the work: no superlatives or scale words ("major", "overhaul", "powerful"), no inventory of the pieces — the sections carry the scale.
- **The main thing can be any kind of change** — a feature, a fix, a large epic; the headline names it whatever its category.
- **Plain text**, unbolded, separated from the first section by a blank line.

Deciding factor: in the common release — one body of work built from many pieces plus sideline changes — the main feature is many bullets, not one, so the headline cannot be a lifted bullet; it has to be a summary, and a summary overlaps its detail. Confidence: high on the shape; the exact wording the prompt uses to get it is prompt-tuning, judged by eye on real releases as the original success criterion prescribes. *(Amended 2026-10-22 — the wording is now also held to the Shipping Bar, measured on E1's held-out releases, with the read by eye alongside.)*

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

**The audit narrowed it.** The rule as first written — no headline where the notes hold nothing but the main thing — assumed such notes state that thing in a bullet. The audit (Context → Evidence) found the opposite on exactly these releases: four of six auditors independently flagged that a release made only of its main work is where the work is split into the most pieces, none naming it — portal v0.9.0's open-several-sessions-at-once feature as seven bullets across Added, Changed and Fixed; tick v0.3.0's TOON rewrite as ~14; dex-engine v0.1.13's instance map as 10; agntc v0.4.0's output overhaul as 7 over three sections. There the headline is not repetition — it is the only line that says what the release is. The session proposed narrowing the exemption to notes where one bullet already states the whole release (mint's 0.0.11, 0.0.7; stitch-cli v1.6.2's single user-facing bullet). The user left the call to the session against the bar that matters — notes that are "good, readable, coherent, useful, accurate, well ordered" — and the narrowing stands.

**A wrong headline against no headline** (raised by the background review, review-001 F6). On an unattended release — `mint release -y`, which agent sessions already run — the notes go to the tag, `CHANGELOG.md` and the provider release with no one reading them first. Today a misjudged salience buries the main feature in a list: bad, but quiet. With a headline, the same misjudgement is stated in the line every reader reads — the diff-sizing release opening "Files that don't belong in release notes are flagged before the notes are written": true, and it names a side piece as the release. The costs are asymmetric: a missing headline costs no more than today's notes do; a wrong one misstates the release in its most-read line, uncaught. The session proposed biasing to omission; the user agreed. Accepted cost: some releases that deserved a headline won't get one — prompt tuning, judged by eye on real releases. The attended path keeps its existing correctives (`e`, and `r` with a nudge such as "lead with the auth work").

**A grab-bag release** (settled call, 2026-10-22). A release of small unrelated changes has no main body of work — the audit's case is dex-engine v0.2.9, a set of separate fixes; the user's own hand-written switchboard v0.2.4 grab-bag opens with no headline (Context → Evidence; [Release-notes audit](../imports/release-notes-audit.md), cross-project synthesis). A theme stitched across such changes is the narrative through-line 886c6100 cut, and Headline Shape makes the headline a summary of a main body of work: with none, there is nothing to summarise. E1's tuning measured the call's difficulty on that release: across prompt versions v0–v2 on the same input, the headline flipped on and off — written on the largest of the separate fixes, then not, then again — until v3's test (most of the notes' changes serve it, or it alone outweighs the rest put together; the largest of several separate changes is not a main body) held it out on both repeats. *(The call as first queued gave mint's 0.0.2 as the example; the audit reads 0.0.2 as two co-equal bodies — the presenter redesign, and dry run with the shared notes cache — so it is a two-body release, not a grab-bag.)*

**What E1 measured of the doubt bias** (2026-10-22). On 26 held-out releases × 2 repeats the frozen prompt wrote no wrong-subject and no unexpected headline in 52 bodies, and headline presence agreed between repeats on all 26. Every miss was an omission (8 bodies: dex-engine v0.2.2, mint v0.0.2, stitch-cli v1.6.6, tick v0.3.0) or a headline naming one piece or one of two bodies (6 bodies: agentic-workflows v0.5.12, v0.8.3, dex-engine v0.2.0) — the accepted cost the bias names. The misses cluster on two-body releases: mint v0.0.2 and stitch-cli v1.6.6 are pre-registered as two bodies or two fixes serving one goal, and dex-engine v0.2.0's headlines name one of its two bodies, so the test that keeps the grab-bag headline out also pushed two-body releases to none, or to half of one. The two-body rule stands; its recall is wording, held to the Shipping Bar.

### Decision

#### 2026-10-22 — revised (bodies the notes prompt never writes)
*Trigger: settled call — bodies that never pass through the notes prompt, not covered by the decisions below.*

**Settled by derivation** — not discussed. Determined by the byte-identical bodies invariant (CLAUDE.md, Load-bearing invariants: notes pass through verbatim, nothing injected) and Prompt Overrides And Reminder (the headline rules live in the default prompt alone).

The entries below stand. Added: a body that does not come from the notes prompt carries no headline, and mint never adds one — the commit-subject fallback (`--no-ai`, and a notes failure under `fallback`), the maintenance-release stub for an empty post-exclusion diff, the first release's fixed `Initial release.` body (`rg -n 'KindFirstRelease: no prior tag' internal/notes/select.go` → line 21), and a `--notes-file` body, which ships as the operator wrote it. The headline exists only where the AI writes it. Deciding factor: these bodies never see the headline rules, and mint writing a line into them would be the injection the invariant forbids. Confidence: high.

#### 2026-10-22 — revised
*Trigger: settled call — a grab-bag release, not covered by the decisions below.*

**Settled by derivation** — not discussed. Determined by Headline Shape (the headline summarises a main body of work) and the 886c6100 history (a stitched-together through-line is the TL;DR that was cut).

The three decisions below stand as written. Added: a release of small unrelated changes — no main body of work, however large the largest of them — carries no headline; its notes start with the first section, as today. Deciding factor: with no main body of work there is nothing for the headline to summarise, and a theme line across unrelated changes is the invented narrative the format removed. Confidence: high.

#### 2026-10-21

A release with two bodies of work of equal weight names both in its one headline sentence — at most two, and still one sentence, benefit-led per Headline Shape. Beyond two co-equal bodies of work is not designed for: the one-sentence limit stands, and the case is left to how the model meets it (YAGNI at mint's cadence). Deciding factor: a headline that names one of two equal halves misrepresents the release, and omitting it costs the lead on the largest releases. Confidence: high. (Resolves review-001 F4, which raised the same case independently.)

A release whose notes are one statement of its main thing carries no headline — the notes start with the first section, as today. The test is whether one bullet already states the whole release: where it does and nothing else stands beside it, a headline only repeats it. Where the main thing is spread over several bullets, none naming the whole, the headline is kept even though the notes hold nothing else — it is the line that says what the release is. A single-bullet main feature beside unrelated changes keeps its headline too. Deciding factor: the headline's job is to state the whole and stand it apart; it is dropped only where a bullet already does both, which is the padding the prompt forbids. Confidence: high — the narrowing rests on the audit's fragmented single-feature releases. (Resolves review-001 F3; narrowed the same day on the release-notes audit.)

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

#### 2026-10-22 — revised
*Trigger: settled call — Change Map Signals takes the Change Map out of ranking within the sections as well.*

**Settled by derivation** — not discussed. Determined by the audit's account of the Change Map's signals and E1's measurement with the map demoted (Change Map Signals).

The headline rule below stands: the AI picks the headline by what the release changes for its user, read from the diff; inputs unchanged, no commit history. Changed: the Change Map's new-structure signal no longer keeps a role in ranking within the sections — the Change Map is a where-the-changes-landed check so nothing large is missed, never a measure of importance, for the headline and the ordering alike (Change Map Signals; ordering by user weight, Ordering Within Sections).

#### 2026-10-21

The AI picks the headline by what the release changes for its user, read from the diff. The Change Map's new-structure signal keeps its role in ranking within the sections but does not pick the headline: a new folder is not, of itself, the release's main thing. Inputs are unchanged — diff and Change Map only, no commit history. Deciding factor: the structural signal cannot identify a release's main thing — it is near-blind where code is nested (it reads only a path's first segment) and flags novelty rather than user weight where it fires — while a headline turns any misread from a reordered list into a misstatement in the most-read line. Confidence: medium-high — the rule is clear; whether the prompt wording achieves it is judged by eye on real releases, this one first. (Resolves review-001 F5.)

---

## Prompt Overrides And Reminder

### Context

Two rules in today's prompts forbid anything above the first section (Context → Current state): the default prompt's "NO TL;DR, NO summary paragraph, NO preamble — start directly with the first section header", and the closing reminder appended after the diff on every notes call — "Now output ONLY the release notes themselves, exactly as they should be published: no preamble, no commentary, no explanation, no code fences, nothing before or after the notes." (`rg -n 'const OutputReminder' internal/notes/prompt.go` → line 83). The reminder is mint's text, not the operator's: it closes the single-pass prompt, every chunk and piece call, and the merge call (`rg -n 'OutputReminder' internal/notes/chunk.go internal/notes/prompt.go | grep -v '//'` → `chunk.go:543`, `chunk.go:567`, `prompt.go:83`, `prompt.go:101`), and it applies under a full `[release].prompt` override too — its own comment calls it "part of the compose, not the instructions". The chunk and merge framings are mint's text on every repo's calls in the same way (`rg -n 'in the format above' internal/notes/chunk.go | wc -l` → 3).

Raised by the background review (review-001 F8): "a format change reaches only repos on the default prompt" holds for the default prompt, but any headline wording added to the reminder or the framings would reach override repos whose own format has no headline.

E1's frozen prompt (`new-v3`) changed the reminder to name the headline — "Now output ONLY the release notes themselves, exactly as they should be published: the one-sentence headline if this release has one, then the sections. No preamble, …" (`.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3-reminder.txt`). On an override repo whose house format is, say, one paragraph per release with no sections, that line would tell the model to put a headline above sections the format does not have.

### Options Considered

**A — the reminder names the headline** (E1's tested wording)
- Pros: tested — on E1's held-out run, headline presence agreed between repeats on all 26 releases, with no unexpected or wrong-subject headline in 52 bodies.
- Cons: mint's format leaks into every override repo's calls.

**B — the reminder switches by prompt source** (the headline-naming line on the default prompt, a neutral line under an override)
- Pros: keeps E1's tested wording where the default prompt runs.
- Cons: one output rule split in two, kept in step by hand.

**C — the reminder is format-neutral and points back at the instructions**: "Now output ONLY the release notes themselves, in the format above, exactly as they should be published: no preamble, …" — "the format above" is whichever instructions govern: the default prompt, headline included, or the operator's override.
- Pros: one rule for every repo; the reminder keeps its single job — the body is the notes and nothing else — and the format stays with the instructions.
- Cons: untested — E1 ran only A's wording.

### Journey

The session leaned C: the reminder exists to stop narration before the notes (observed in real `mint commit` runs), a job that is the same on every repo, while the headline is a property of one format. A pushes the default format into overrides; B solves that at the cost of two reminders to keep aligned. C's open risk is the one Headline Shape's Option C named — a model reading "no preamble" as licence to drop the headline. "In the format above" is the answer to it: on the default prompt the format the reminder points at opens with the headline. Whether that holds is a measurement, not a decision — it rides with the next run of the prompt. What would reopen the call: evidence that the model keeps a headline only when the last thing it reads names one. The user agreed.

The same principle settles the rest of the reach question carried in from discovery. The default prompt is where the format lives, so the headline rules — and the rewording of its own no-preamble rule to allow exactly the one headline line and still forbid a summary paragraph or any other preamble — live there alone. `[release].context` appends to the default prompt, so those repos get the headline. A full `[release].prompt` override is the operator's format: it gets no headline unless the operator's prompt asks for one, and nothing mint adds to its calls asks for one.

Sibling check: token-aware-diff-sizing — Chunk Wave Mechanics decides that chunk and merge calls carry the release's instructions, framing naming the call's part, and the closing reminder, each returning notes "in the release's notes format"; a neutral reminder pointing at "the format above" is consistent with it.

### Decision

Everything mint adds to every notes call — the closing reminder, the chunk and piece framings, the merge framing — stays format-neutral; every headline rule lives in the default prompt alone. The reminder becomes "output only the release notes, in the format above, exactly as they should be published", followed by today's no-preamble rules. The default prompt's "start directly with the first section header" gives way to the headline-or-first-section rule, still forbidding any summary paragraph or other preamble. Repos on the default prompt, with or without `[release].context`, get the headline; a full `[release].prompt` override owns its format and gets none unless it asks for one. What the part and merge calls do about a headline is the Chunked Path's call, made within this rule.

Deciding factor: the reminder's job — the body is the notes, nothing else — is the same on every repo, and the headline belongs to one format; one neutral rule that points back at the governing format serves both. Accepted cost: the neutral wording is untested — whether the model keeps the headline under it is checked on the next run of the prompt. Confidence: high on the rule, medium on the wording until measured. (Resolves review-001 F8.)

---

## Shipping Bar

### Context

What the rewritten default prompt has to clear before it ships. E1 (Context → Evidence) pre-registered a bar for taking a rewritten prompt "as tested" — a threshold per failure class plus three guard rails — and its frozen prompt fell short: it passed only Breaking Changes, and missing items broke a guard (`.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/data/scores.json`). The rule's mechanical branch for every failed class is a successor experiment on that class. The user's intent for E1 was that it let the discussion make its decisions, shape the prompt, and move ahead.

Today's prompt on E1's 26 held-out releases, two repeats each (current arm, `data/secondary.json` → `field_counts.current`; lead from `lead.current`; breaking from `breaking.current`): clean lead 21 of 52, inversions 30, misfiled 84, duplicates 29, internal items 77, false claims 25, in-release-as-change 22, missing items 9, format slips 5, breaking flagged 1 of 10. E1's frozen prompt on the same inputs: 38, 16, 78, 19, 28, 34, 13, 13, 5, 8 of 10 — better or level on every count but two, false claims and missing items.

### Options Considered

**1 — At least as good as today's notes on every count**, the wording tuned during implementation on the experiment's releases, with E1's judges as the acceptance check.

**2 — E1's full bar**, which means a further experiment before this discussion closes.

### Journey

The session's position: discussion decides what the notes must do, not their sentences. On the returned subtopics E1's evidence shows the rules move the notes the right way on nearly every count, the judges preferring the new notes on 21 of 26 releases and the user, blind, on 4 of 6; what is short is wording. The one result that would make shipping wrong is a step back in accuracy, and option 1 rules it out. Option 2 spends a full judging round before the specification on wording that is refined on the same releases whichever phase does it. The verdict's successor-experiment branch is set aside for the rules; the measurement carries on as the acceptance check. The user chose 1.

Each count stands on its own. E1 scored false claims and in-release-as-change as one class, which sat level at 47 while false claims rose 25 → 34; netting one against the other would let the rise ship. The headline has no counterpart in today's notes, so "at least as good" means it adds no wrong-subject headline — E1's frozen prompt wrote none in 52 bodies.

Two observations carried with the bar, neither added to it. The user's two blind picks for today's notes (portal v0.11.1, mint v0.0.2) both carry the new prompt's errors in the opening lines, and the counts weigh an error at the top the same as one at the bottom — the original "by eye on real releases" stays alongside the counts. And a prompt tuned on the releases that accept it can fit them; the bar's guard against that is that it asks for no worse than today rather than a tuned threshold, and the rules it carries are general.

Sibling check: mint-release-tool specification — Stage 4's success criterion holds the notes to "on a big release, the headline feature leads the notes", with "no formal rubric or harness — the user assesses output quality by eye on real releases … and tunes the prompt". This bar supersedes the verification half for the rewritten default prompt; it is still true of the code as shipped, and shifts when this feature ships.

### Decision

The rewritten default prompt ships when, on E1's 26 held-out releases — two repeats each, judged blind with E1's rubric and judges and scored as E1 scored them, against today's prompt on the same inputs — every count is at least as good as today's, each on its own:

- clean lead at least 21 of 52; no wrong-subject headline;
- inversions at most 30; misfiled at most 84; duplicates at most 29;
- internal items at most 77;
- false claims at most 25; in-release-as-change at most 22;
- breaking changes flagged at least 1 of 10;
- missing items at most 9; format slips at most 5.

The wording is tuned during implementation on the experiment's releases; the acceptance run is a pass of the frozen candidate over the held-out set. E1's frozen prompt already clears every count but false claims and missing items, so those two are where the tuning goes, holding the rest.

Deciding factor: the rules are decided and moving the right way; the only shipping outcome that would be wrong is a regression, and this bar rules every regression out while the remaining gap is wording. Confidence: high on the bar.

---

## Chunked Path

### Context

A release whose diff is too large for one call is written in parts and merged (Context → Current state): each chunk or piece call carries the release's instructions, the whole-release Change Map, framing naming its part, its slice and the closing reminder, and is told to write "in the format above"; the merge call reads every part's notes in part order and writes the release's notes "in the format above" (`internal/notes/chunk.go`). When the merge fails, mint saves the finished parts' notes — byte for byte, in chunk order, a blank line between parts, opening with a `**Notes missing:**` line when any slice has none (`grep -n "func joinChunkNotes" -B 2 internal/notes/chunknotes.go` → "keeps each part's notes byte for byte, with one blank line between one part's notes and the next") — for the operator to tidy and release with `--notes-file`, or to pick up with `e`.

Raised by the background review (review-001 F7), and carried from discovery as an open question (whether chunk calls name a headline or leave it to the merge): once the format opens with a headline, every part call is told to write one. On a release written in four parts, part two might open "Release notes for diffs of any size…" and part three "Files that don't belong in the notes are flagged before they're written…" — each part crowning its own slice. The merge then reads four competing leads; and a failed merge's saved file holds four headlines stacked one after another, shipped as written if the operator misses them, since mint never edits a body.

### Options Considered

**A — parts write no headline; the merge alone writes it**
- Pros: the merge is the only call that sees the whole release; the saved file never carries competing leads.

**B — parts write headlines and the merge chooses among them**
- Cons: the pick is made from leads written by calls that each saw one slice.

### Journey

Stitching and Salience (token-aware-diff-sizing) framed the chunk problem as "no chunk knows whether its slice is the headline or a footnote"; a headline is exactly the call a part cannot make. The session proposed A as the one defensible shape; the user agreed.

The part instruction has to say it within Prompt Overrides And Reminder's rule that mint's shared text stays format-neutral: it asks the part to leave out anything that sums up the release as a whole, which the merge writes — on the default prompt that is the headline, and under an override whose format has no such line it asks nothing. The saved part-notes file then holds sections only. A release shipped from it carries a headline only if the operator writes one while tidying — the No Clear Headline bias (when in doubt, leave it out) on a path where no call saw the whole. A regenerate on chunked notes (`r` re-runs only the merge) and a merge re-run from saved part notes both go through the merge, so the headline and any one-time nudge ("lead with the auth work") stay with the call that writes it.

Sibling check: token-aware-diff-sizing — The Saved Chunk-Notes File decides its contents are "each finished chunk's notes in chunk order, separated by a blank line, exactly as the chunk calls returned them", with the notes-missing marker on top; this decision changes what the part calls return, not the file's rule, which stands.

### Decision

Part calls write no headline; the merge call alone writes it, judging the whole release from every part's notes and the whole-release Change Map. The part instruction says, in format-neutral terms, to leave out anything that sums up the release as a whole — the merge writes that. The saved part-notes file therefore holds sections only; a release shipped from it carries a headline only if the operator adds one while tidying. Deciding factor: a part sees one slice and cannot know whether it is the release's main work or a footnote, and the merge is the only call that sees the whole. Confidence: high. (Resolves review-001 F7.)

---

## Room For A Supplied Headline

### Context

The session that built a feature knows its headline better than mint's diff-only view (Picking The Headline). The roadmap item **agent-authored release notes** (Next, waiting: "An AI agent running a release can follow mint's notes standard — mint hands it the writing instructions and change summary, or takes a one-line statement of what the release is about"; `node .claude/skills/workflow-engine/scripts/engine.cjs roadmap state` → item `agent-authored-release-notes`) owns that; this feature's brief is only that the format should not foreclose it.

Raised by the background review (review-001 F10). An agent session running `mint release -y` that knows what the release is about has three routes for that one line: hand it to mint's AI as a steer; write the whole notes itself and pass them as a finished `--notes-file` body; or give mint the line and have mint place it above the AI's body. The Context's as-built bullet read the byte-identical rule as "a headline has to come from the AI's output, not be added afterwards" — taken literally, that closes the third route before the item that owns it has weighed it.

### Journey

The session's position: design nothing here, and narrow the sentence to what the invariant protects. The byte-identical rule exists so that mint never rewrites or brands what someone wrote — "no reformatting, no branding injected into message text" (CLAUDE.md, Load-bearing invariants). A line the operator hands over is the operator's content, as a `--notes-file` body is; whether mint should ever compose it with the AI's body is the roadmap item's question, beside the steer and the finished file.

The user favoured the steer — the agent steering mint's AI — doubted it is easy, and placed it outside this work; the roadmap item above already covers it. Ground for that item, noted rather than designed: a one-time steer already exists. The review gate's `r` takes a transient nudge that is appended to the resolved instructions and flows into every AI call of that generation, never written back to config (`grep -n "func appendOneTimeContext" -A 5 internal/notes/generate.go` → appends the nudge after the instructions; the doc comment on `GenerateFromRangeWithContext` → "flows into every AI call this generation makes"). It is reachable only from an attended terminal today; a supplied steer for an unattended run would carry the same kind of line by another way in.

Sibling check: agent-authored release notes (roadmap, waiting) — names the one-line statement as one of its two routes and has not decided between steering and placing; nothing here pre-empts it.

### Decision

This feature designs no route for a supplied headline. The format's headline — one plain sentence above the first section — is the slot any later route fills. The byte-identical rule is read as protecting what the AI or the operator wrote from mint's own edits and additions: mint never writes or edits a headline of its own, and nothing here rules out a steer, a finished notes file, or an operator-supplied line placed above the AI's body. Which of those the agent-authored release notes item takes is its call. Deciding factor: the brief is not to foreclose, and the literal reading of the Context's sentence foreclosed one route on an invariant that does not reach it. Confidence: high. (Resolves review-001 F10.)

---

## Release Title

### Context

A provider's releases page lists each release by its title, and mint titles every provider release with the bare tag — forward release and both regenerate paths pass the tag as the title (`rg -n 'CreateRelease\(ctx, tag, tag' internal/engine/release.go` → line 786; `rg -n 'DispatchRelease\(' internal/engine/regenerate_batch.go internal/engine/regenerate_write.go` → `res.Tag, res.Tag` at `regenerate_batch.go:361`, `req.Tag, req.Tag` at `regenerate_write.go:200`). With the notes opening on a headline, the obvious move is to lift it into the title: `v0.0.12 — Release notes for diffs of any size`.

Raised by the background review (review-001 F9), which named the edges such a change meets:

- **mint would read into the notes.** Nothing in mint parses the body (Context → Current state); a title taken from it would be the first place mint inspects what the AI wrote.
- **Many bodies have no headline to lift**: the commit-subject fallback (`--no-ai` and a notes failure), the maintenance stub, the first release's fixed body (`rg -n 'KindFirstRelease: no prior tag' internal/notes/select.go` → line 21, `fixed "Initial release." body, no AI`), a `--notes-file` body, any repo on its own `[release].prompt`, and every release the doubt rule (No Clear Headline) leaves without one.
- **Regenerate retitles.** Both regenerate paths re-publish the release with a title, so a change would retitle releases already out.

### Options Considered

**A — the title stays the bare tag**
- Pros: one consistent title on every release; mint never reads the body.

**B — the headline is lifted into the title where the body has one**
- Pros: the releases list carries what each release was about.
- Cons: titles depend on whether the AI chose to write a headline, so the list mixes summaries and bare numbers; mint reads into the body.

### Journey

The session leaned A. On the release page the headline already renders as the first line under the title, so B's gain is confined to the releases list, and there it buys an inconsistent list at the price of mint's rule of never reading into the notes. What would have moved it: the user valuing scanning releases by title over consistent titles. The user agreed: keep the bare tag.

Sibling check: mint-release-tool specification — the provider release is a write-only projection of the full body, and `mint release regenerate --source tag` (the flag that specification names `--reuse`, replaced by `--source` in fc1cfe13) recreates it from the tag annotation body "deterministic, parse-free"; a bare-tag title keeps both.

### Decision

The provider release title stays the bare tag on forward release and on both regenerate paths; the headline lives only in the body, where it is the first line under the title. Deciding factor: a headline-derived title would vary with whether the AI wrote a headline and would make mint read into a body it otherwise passes through untouched, for a gain confined to the releases list. Confidence: high. (Resolves review-001 F9.)

---

## Breaking Changes

### Context

Today's format has no place for a breaking change: the default prompt's sections are Keep a Changelog's six (✨ Added, 🔧 Changed, ⚠️ Deprecated, 🗑️ Removed, 🐛 Fixed, 🔒 Security), so a change that stops existing use working lands in whichever of them fits, unflagged. The audit found breaking changes buried in 5 of 31 releases (Context → Evidence). On E1's five breaking held-out releases, today's prompt flagged 1 of 10 bodies.

### Options Considered

The rule E1 tested (`new-v3`): a 💥 Breaking section first among the sections; a change listed there only, not repeated; emitted only where the diff shows something users already do stops working or gives a different result they must act on — a removed or renamed command, flag, option, setting or key binding, a default changed so existing setups behave differently in a way that matters, a changed output or file format that scripts read, data or configuration from the previous release no longer read; a change that only adds, keeps more, allows more or improves is not breaking; when unsure, it is not; the one sentence says what breaks and what to do.

### Journey

E1 measured the rule: 8 of 10 breaking bodies flagged against today's 1 of 10 — the one class past its pre-registered bar. Portal v0.11.1, predicted hardest, was flagged on both repeats with the remedy named. The miss was switchboard v0.1.5 on both repeats — output piped off a terminal becoming JSON, worded as a convenience under 🔧 Changed, which the "allows more or improves" and doubt lines fit. The new section is also a new place to be wrong: nine Breaking→Changed misfilings, and false breaks asserting a previous state the diff contradicts (portal v0.8.0's kill binding, agentic-workflows v0.6.0's `--key` flag); mint v0.0.2's new notes opened on a false 💥 bullet, one of the two releases the user, reading blind, preferred today's notes on.

That last cost is why the narrow definition and the doubt rule stay: the section sits at the top of the notes, so a false alarm stands in the opening lines, which is No Clear Headline's own asymmetry — a missed flag costs what today's notes already cost, a false one misstates the release where every reader looks. The misfilings and false breaks are wording, held to the Shipping Bar.

Sibling check: mint-release-tool specification — its default notes format is Keep a Changelog's six sections in fixed order; this adds 💥 Breaking ahead of them for the rewritten default prompt, superseding that format when this feature ships.

### Decision

**Settled by derivation** — not discussed. Determined by the audit's buried breaking changes, E1's measurement of the rule (8 of 10 flagged against 1 of 10), and No Clear Headline's asymmetry (a false statement in the opening lines costs more than an omission).

Breaking changes get a 💥 Breaking section, first among the sections (after the headline where there is one), and a breaking change is listed there only. It is for existing use that stops working or gives a different result users must act on; a change that only adds, keeps more, allows more or improves is not breaking, and when unsure a change is not breaking and goes in its usual section. Each bullet says in its one sentence what breaks and what to do. The section, like every section, is omitted when empty. Deciding factor: a reader upgrading must meet what breaks before anything else, and the narrow definition keeps the section from crying wolf at the top of the notes. Confidence: high on the rule; its misfilings are wording.

---

## User Facing Only

### Context

Internal and dev-only changes reached users in ~19 of 31 audited releases — CI, lint, release tooling, internal packages, engine verbs, migration numbers, test harnesses, internal flags (Context → Evidence; mint v0.0.6's new internal packages each became a feature bullet). Scope took in both halves: keeping internal changes out, and phrasing in the user's terms rather than mechanism or spec jargon.

### Journey

E1's rules (`new-v3`): include only changes a user meets — what the software does, its commands, options, configuration, output, errors, performance and compatibility; leave out how it is built and maintained — CI and release tooling, linting, build scripts, test suites and harnesses, internal packages, modules and refactors, internal efficiency a user would not notice, internal flags and plumbing commands, data-migration numbering, contributor docs, repo housekeeping; a new internal package is not a feature; describe changes in the user's terms, not the code's names, the mechanism or a design document's jargon. Where the files are the product — prompts, skills, templates the software ships and uses — changing them changes what users meet.

Measured: internal items 77 → 28 on the held-out set, a 64% cut. Its one regression on this ground sat in the missing-items guard: agentic-workflows v0.7.80's faster filtered store reads, missing from both new repeats — a speed-up the judges counted as one users meet, dropped under "internal efficiency changes a user would not notice". The line is therefore whether a user notices the change, not whether the code that made it is internal. The residual internal items are wording, held to the Shipping Bar.

### Decision

**Settled by derivation** — not discussed. Determined by Scope (user-facing only, in the user's terms), the audit's ~19 of 31, and E1's measurement — internal items 77 → 28, with one noticed speed-up dropped as internal.

The notes list only what a user of the software meets — what it does, its commands, options, configuration, output, errors, performance and compatibility; how it is built and maintained stays out. The test is whether a user meets or notices the change, not whether the code behind it is internal: a speed-up users notice is theirs, and where the shipped files are the product (prompts, skills, templates), changes to them count. Every change is described in the user's terms — what they see and do — never in the code's names, the mechanism or a design document's jargon. Deciding factor: the notes are for the people who use the software, and E1 showed the line drawn at the code's internals drops changes users do meet. Confidence: high.

---

## Ordering Within Sections

### Context

Today the default prompt ranks bullets within each section by the Change Map, new structure weighted above raw size (Context → Current state). The audit found inversions within sections in about half its releases: rank followed commit or file granularity and how nameable an item is, not what it meant to users, and README prose was weighted as content (Context → Evidence) — portal v0.9.0's secondary CLI verb above the picker flow that is the feature's front door; tick v0.3.2's guard above the fix it guards.

### Journey

E1's rule (`new-v3`): put the change that matters most to a user first, then the rest in descending weight; weight is what the change means to the user — how much of the product it touches, how often users meet it, how much it changes their experience — not how many files, commits or lines it took, and not how easy it is to name; a change to what users do every day outranks one to a diagnostic or rarely used command; the pieces of the release's main work lead their sections. The everyday-over-diagnostic line came from tuning: on tick v0.3.2, four `tick doctor` bullets led 🔧 Changed above parse errors every command shows.

Measured: inversions 30 → 16 on the held-out set, nearly halved, one short of E1's pre-registered bar; the residual is wording, held to the Shipping Bar. What the Change Map contributes to ordering is its child, Change Map Signals.

### Decision

**Settled by derivation** — not discussed. Determined by the audit's inversions and their causes, and E1's measurement of the weight rule (30 → 16).

Within each section, changes are ordered by what they mean to the user — how much of the product a change touches, how often users meet it, how much it changes their experience — most first; a change to what users do every day outranks one to a diagnostic or rarely used command, and the pieces of the release's main work lead their sections. Lines, files, commits and how easy a change is to name are not weight. Deciding factor: the audit traced the inversions to granularity and nameability standing in for weight, and naming weight in the user's terms halved them. Confidence: high on the rule.

---

## Change Map Signals

*(Child of Ordering Within Sections.)*

### Context

The Change Map is mint's diff-derived preamble ahead of the diff — where the release's changes landed, ordered novelty-first: new packages or directories and renamed or removed paths lead, then a per-area churn rollup, then individually notable files — new top-level entries and the single largest file (the doc comment on `BuildChangeMap`, `internal/notes/changemap.go`: "orders salience NOVELTY-FIRST (weighted above magnitude)"). The default prompt treats it as the ranking signal: "new structure (a whole new package or directory) is the strongest signal — weight it above raw size", and the merge framing on chunked releases says "rank by the Change Map above" (`rg -n 'rank by the Change Map above' internal/notes/chunk.go` → line 573). The design came from the mint-release-tool work, whose success criterion aimed the Change Map at the headline: "new-structure-as-headline, novelty over magnitude".

The audit (Context → Evidence): new-directory detection reads only a path's first segment, so a new package under an existing `internal/`, `src/` or `skills/` never registers; the novelty line fired in 6 of 31 releases and was wrong each time — twice on a pre-existing `design/` folder, on a docs site that then led the notes, on README demo assets, on a 6-line wrapper, and on the smaller of two co-equal bodies, which it promoted; the area rollup collapses to one or two top-level folders; the largest-file line is often a test, a deletion or a dev script. The prompt's own rule turned each new internal package into a feature bullet in mint v0.0.6. Picking The Headline already took the headline away from the signal.

### Journey

E1's prompt (`new-v3`) kept the Change Map in the input unchanged and told the AI what it is for: "The Change Map shows where the changes landed. Use it to make sure nothing large is missed, not as a measure of importance: a new folder may hold tooling, and the largest file is often a test or a deletion. DESCRIBE every change from the DIFF; the Change Map is metadata, never content." With the map demoted and ordering judged by user weight, inversions fell 30 → 16.

Fixing the detection instead — finding new packages at any depth — would make the signal fire on exactly the new internal packages User Facing Only keeps out of the notes, and still flag novelty rather than what a release changes for its user. So the map stays as it is, as coverage. The merge framing's ranking instruction is mint's shared text on every repo's calls; under Prompt Overrides And Reminder it goes format-neutral, and ranking is left to the governing format.

Sibling check: mint-release-tool specification — Stage 4 names the Change Map "new-structure-as-headline, novelty over magnitude" as the mechanism aimed at the headline criterion, and release-notes-quality decided "rank importance from the Change Map; describe changes from the diff"; token-aware-diff-sizing — Chunk Wave Mechanics has the merge "ranking by the Change Map". This decision supersedes the Change Map's ranking role in all three for the rewritten default prompt and the merge framing; the specifications' text shifts when this feature ships.

### Decision

**Settled by derivation** — not discussed. Determined by the audit's account of the Change Map's signals (the novelty line wrong in each of its 6 firings, blind where code is nested), E1's measurement with the map demoted (inversions 30 → 16), User Facing Only (new internal packages are not features), and Prompt Overrides And Reminder (mint's shared text stays format-neutral).

The Change Map stays in every notes call as it is built today — a where-the-changes-landed check the AI uses so nothing large is missed, never a measure of importance; every change is described from the diff. The default prompt's "new structure is the strongest signal" rule goes, replaced by that role. The merge framing drops "rank by the Change Map above"; ranking is the governing format's. The Change Map's detection is not changed. Deciding factor: the signal misled on every release where it fired and is silent where code is nested, and the fix that would make it fire more would point it at internals. Confidence: high. (Revises Picking The Headline's ranking role for the signal.)

---

## Section Placement

### Context

Today's sections run in Keep a Changelog's fixed order (✨ Added, 🔧 Changed, ⚠️ Deprecated, 🗑️ Removed, 🐛 Fixed, 🔒 Security), with no rule for what "Added" or "Fixed" is measured against. The audit (Context → Evidence) found section misfiling in ~15 of 31 releases — fixes under Added or Changed, redesigns of existing views under Added, regression fixes as new features; a main work that reshapes existing behaviour landing in 🔧 Changed below ✨ Added, so small new items outrank it (eight releases; twice the model broke the fixed order to compensate); the main work scattered over two to four sections and stated twice in five. Scope took in misfiling, reshaping work buried under the fixed order, and the main work scattered and stated twice.

### Options Considered

**Fixed order** — every release's sections in one order.

**Order by weight** — the section holding the release's main work first.
- Cons: the notes change shape from release to release, and the burial it answers is now answered by the headline.

### Journey

Headline Shape puts a one-sentence statement of the main work above the sections, so reshaping work under 🔧 Changed is no longer the only place the release's reason to exist appears; reordering the sections to lift it would buy a second answer to the same problem at the cost of an order readers can rely on across versions. Breaking Changes adds 💥 Breaking ahead of the six.

E1's placement rules (`new-v3`) judge every change against the previous release, never against earlier states inside this one: something the previous release did not have is Added, its pieces included, even where the diff shows it adjusted, capped or fixed along the way — never a Fixed or Changed for something new; a rework of something the previous release had is Changed, with a genuinely new capability inside it Added; something that behaved wrongly in the previous release and now behaves correctly is Fixed, however the diff frames it, including a rework whose purpose is the fix and something an earlier release lost that this one restores. Measured: Changed→Fixed misfiling 23 → 14, duplicates 29 → 19, misfiled overall 84 → 78. The dominant misfiling is unmoved in both arms — a rework of existing behaviour filed under ✨ Added (36 today, 34 new) — and is wording, held to the Shipping Bar.

### Decision

**Settled by derivation** — not discussed. Determined by Headline Shape (the headline states the main work above the sections), Breaking Changes (💥 Breaking first), and E1's measurement of the previous-release rules (Changed→Fixed 23 → 14, duplicates 29 → 19).

The sections keep one fixed order — 💥 Breaking, ✨ Added, 🔧 Changed, ⚠️ Deprecated, 🗑️ Removed, 🐛 Fixed, 🔒 Security — empty sections omitted. Every change is placed by comparison with the previous release: what it did not have is Added, pieces and in-release adjustments included; a rework of what it had is Changed, a new capability inside the rework Added; what behaved wrongly in it and now works is Fixed, whatever the diff calls it. Each change is listed once; only the headline restates the main work. Deciding factor: the headline already lifts the main work above any section, so a fixed order costs nothing the reader needs and keeps the notes the same shape release to release; the previous release is the only baseline a reader upgrading from it shares. Confidence: high on the rules; the rework-as-Added misfiling is wording.

---

## True To The Release

### Context

The audit (Context → Evidence) found false or stale claims shipped in ~7 of 31 releases — behaviour invented from a README, comment or test edit, last release's features re-announced, already-true behaviour presented as new, a never-shipped bug presented as fixed — and work built in this release presented as a change or fix to the last in ~7. Scope decided the rules: no behaviour claimed from docs, comments or tests alone; work built in this release not presented as a change or fix to the last; nothing old presented as new. Its stated cause was that from the diff alone the AI cannot tell new behaviour from newly documented behaviour. E1 was asked, among other things, whether these failures need mint to hand the AI something it lacks.

### Journey

E1's rules (`new-v3`): claim only behaviour the code in this diff changes — documentation, READMEs, comments, examples and tests describe behaviour and editing them does not change it (where the files are the product, changing them changes it); do not present behaviour as new when the diff shows it already existed, in unchanged context lines or in code only moved or renamed; comments and docs written during the release describe a "before" that may be an earlier state inside the release; before keeping a bullet, find the code change behind it, and drop it when every changed line behind it is a comment, documentation or a test; a new test asserting a behaviour is evidence it already existed.

Measured on the held-out set: in-release-as-change 22 → 13; false claims 25 → 34 — the class level at 47. The rise concentrates in four releases (portal v0.8.0 1 → 4, agntc v0.6.0 2 → 4, agentic-workflows v0.6.0 3 → 5, switchboard v0.1.0 0 → 2; exploratory, from the judgements against the blind key). Nearly every false claim in either arm is one of two kinds: behaviour presented as new or changed that the previous release already had, or a claim drawn from a README edit with no code behind it; several new-arm false claims sit in bullets worded with a before ("moved from", "now", "no longer"), the shape the "check the before" rule names.

Whether mint must hand the AI more: the judges called 3 of 47 residuals input-absent (switchboard v0.1.5's config keys, README rows for code that shipped in v0.1.4), and the session spot-checked three "the previous release already had it" calls against the inputs — each fact sat in the diff's removed lines (portal v0.8.0's input shows, among its removed lines, the `k` kill binding that its new notes claim was moved from `K` to `k`; agntc v0.6.0's removes a comment saying colour is stripped for `NO_COLOR`; switchboard v0.1.0's removes a README line giving every account a `reserve` setting). The previous release's state was in front of the AI; the prompt did not get it used. So new inputs would not answer the failure; whether wording can make one call read a diff of up to 1.9 MB that carefully is what the tuning under the Shipping Bar finds out.

### Decision

**Settled by derivation** — not discussed. Determined by Scope (the accuracy rules), E1's residual sorting (input-absent 3 of 47, spot-checked against the inputs), and the Shipping Bar (false claims at most 25 and in-release-as-change at most 22, each on its own).

The accuracy rules stand as Scope decided them: no behaviour claimed from documentation, comments or tests alone; work built in this release never presented as a change or fix to the last; nothing the previous release already had presented as new. mint hands the AI nothing new for them — the previous release's state is already in the diff's removed and context lines — and the prompt's wording carries the fix, held to the Shipping Bar: E1's wording already brings in-release-as-change under it, and false claims, which rose, must come back to today's count or below. Deciding factor: the AI had what it needed in all but 3 of 47 cases, so the gap is how the prompt gets the diff read, not what mint sends. Confidence: high on the rules and the inputs; medium on how far wording gets false claims down — the riskiest count under the bar.

---

## Checking The Chunked Path

### Context

Chunked Path decides that part calls write no headline and the merge alone writes it, through a format-neutral instruction in the chunk and piece framings. The Shipping Bar measures the rewritten prompt on E1's held-out releases, and E1 ran every release as one call, the two over the 1.5 MB cap included, so nothing measured has exercised the part instruction or a merge writing the headline. The releases that chunk are the largest, where the headline earns most (No Clear Headline: omitting it "drops the headline on the releases where it helps most — the biggest ones"), and they are often released unattended with `mint release -y`, where nobody reads the notes before the tag, `CHANGELOG.md` and the provider release carry them. Two failures would ship unseen: a merge that drops the headline, or part calls that keep writing their own, which a failed merge's saved part-notes file stacks one after another.

### Options Considered

**A — the first tuning runs check it, holding shipping on a regression**: held-out releases taken through the part calls and the merge; a failure today's notes could not have holds shipping, an omission does not.

**B — gating chunked counts in the bar**
- Cons: today's prompt has no measured chunked counts, so "at least as good as today's" has no baseline on this path for most counts.

**C — leave it to the read by eye on real releases**
- Cons: the largest releases, often unattended, go unchecked until after they ship.

### Journey

Prompt Overrides And Reminder already set how untested shared wording is handled: whether the neutral reminder keeps the headline "is a measurement, not a decision — it rides with the next run of the prompt". The part instruction is the same kind of ground — mint's shared wording, decided as a rule, its effect on the model unmeasured — so it rides the same runs. B needs a baseline the record does not hold; C leaves the path where the headline matters most unchecked until it has shipped. Two of the failures have a baseline all the same: today's notes carry no headline, so a headline in a part's notes, or a merged headline naming something other than the release's main work, is a regression, and the Shipping Bar exists to rule out every regression, already allowing no wrong-subject headline. A merge that leaves a headline out costs what today's notes cost, the accepted cost of When in doubt, leave it out, and is tuned like the other weak spots. Holding shipping on that too would delay it over an outcome no worse than today; holding on neither would let a regression ship on the largest releases. How the harness takes a held-out release through parts is the builder's.

### Decision

**Settled by derivation** — not discussed. Determined by Prompt Overrides And Reminder (untested shared wording is checked on the next run of the prompt), the Shipping Bar (every regression ruled out, no wrong-subject headline) and No Clear Headline (the headline earns most on the biggest releases; an omission costs no more than today's notes).

The first tuning runs under the Shipping Bar also take held-out releases through the part calls and the merge, and check that the part notes carry no headline and that the merge writes the headline the release calls for. A headline in any part's notes, or a merged headline that names something other than the release's main work, holds shipping until the wording clears it: today's notes carry neither, so either is a regression. A merge that leaves out a headline the release calls for does not hold shipping; it is tuned like the other weak spots. Deciding factor: the part instruction is shared wording whose effect on the model is a measurement, as the neutral reminder's is, and on this path only the failures today's notes could not have are regressions. Confidence: high on the check; how it is run is the builder's.

---

## Keeping The Wording General

### Context

The Shipping Bar tunes the rewritten prompt's wording during implementation on E1's releases and accepts it on E1's 26 held-out releases, the same releases a tuned prompt can fit. Its Journey names the guard: the bar "asks for no worse than today rather than a tuned threshold, and the rules it carries are general". The second half needs a line tuning can be held to. Wording drawn from the sampled releases themselves fits those releases by construction: it can pass every count and do no better on the other repos the default prompt ships to.

### Journey

E1 tuned its candidate under exactly such a line: no prompt text may name, quote or paraphrase any change from the 31 sampled releases; examples, where added, are invented; the three altitude examples carried over from the shipped prompt are exempt. Its frozen candidate, the Shipping Bar's starting wording, was written to that line. Carrying it into implementation's tuning is what "the rules it carries are general" asks of wording tuned and accepted on those releases. The altitude examples stay exempt: they are the shipped prompt's own, kept with the bullets exactly as today (Headline Shape), and both of E1's arms carried them.

### Decision

**Settled by derivation** — not discussed. Determined by the Shipping Bar (the rules it carries are general, its guard against a prompt fitted to the releases that accept it) and E1's tuning constraint, under which the starting wording was written.

Tuning the default prompt's wording keeps it general: no prompt text names, quotes or paraphrases a change from the 31 sampled releases, and any example added is invented. The three altitude examples the shipped prompt carries are exempt and stay as they are. Deciding factor: wording drawn from the releases the prompt is tuned and accepted on fits them by construction, which would let the bar pass on wording that does no better elsewhere. Confidence: high.

---

## A Release With Nothing Users Meet

### Context

User Facing Only keeps out everything a user never meets: CI, tests, refactors, internal packages, contributor docs, repo housekeeping. The format allows nothing above the first section but the headline (Prompt Overrides And Reminder). That leaves no answer for a release, or a part of a chunked one, whose every change is of the kind left out, such as a release cut after test-only or tooling-only work. Without one, the model lists the internal changes anyway, writes a line the format forbids, or returns an empty body. The transport rejects an empty body as a failed call (`rg -n 'ai generation failed \(empty body\)' internal/ai/transport.go` → line 80), and on the chunked path that fails the part. Path-order packing can put a run of test files or docs in a part of its own.

### Options Considered

**A — one 🔧 Changed bullet saying the release is maintenance only**
- Cons: misfiles by Section Placement's own rule. Changed is a rework of behaviour the previous release had, and nothing changed.

**B — one plain line in the shape of mint's empty-diff stub, no headline and no sections**
- Pros: matches the body mint already ships for a maintenance release (`rg -n 'const stubBody' internal/notes/degenerate.go` → `"Maintenance release — no notable source changes"`).
- Cons: one named exception to the headline-or-first-section rule.

**C — list the internal changes for this case only**
- Cons: breaks User Facing Only, and counts against the Shipping Bar's internal items.

### Journey

A misfiles and C breaks a decided rule, and an empty body is a failure, not notes. B is the one shape left, and it reads as the maintenance body readers already see from mint when a diff is empty. No main body of work users meet means no headline (No Clear Headline). On the chunked path a part with nothing users meet returns the same line. The merge drops it beside other parts' sections and writes it only when every part returned it, so the release-level answer is the same however the diff was cut.

### Decision

**Settled by derivation** — not discussed. Determined by User Facing Only (internal changes stay out), Section Placement (Changed is a rework of what the previous release had), No Clear Headline (no main body, no headline) and the transport's rejection of an empty body.

Where every change in a release is one User Facing Only leaves out, the notes are one plain line saying the release is maintenance with no user-facing changes, in the shape of the empty-diff stub, with no headline and no sections. It is the one body without sections. A chunked release's part with nothing users meet returns the same line, and the merge writes it only when every part's notes are that line. Deciding factor: it is the only answer that breaks no decided rule and ships notes rather than a failed call. Confidence: high on the rule; the line's wording is the prompt's.

---

## Summary

### Key Insights

1. The user's bar for every call in this feature is the generated note itself: "good, readable, coherent, useful, accurate, well ordered". Mechanism choices are the session's to make against that bar; real releases are the test.
2. The notes' dominant failure is scatter, not absence: the main work is in the notes but split over many bullets and sections with no line stating the whole. The headline is the statement of the whole; the sections are its detail.
3. Most accuracy failures share one cause — the AI reads a diff and cannot, unaided, tell new behaviour from newly documented behaviour, or what a user meets from what only the code sees. *(Amended 2026-10-22 — E1 found the previous release's state in front of the AI in all but 3 of 47 accuracy residuals, in the diff's removed and context lines: the failure is the AI not using what the diff shows, not the diff lacking it — True To The Release.)*
4. The rules move the notes the right way; the wording is what is short. E1's rewritten prompt beat today's on every count but false claims and missing items, judges and the user preferred it, and no failure needed mint to hand the AI anything new — the feature is the default prompt and mint's shared wording, with the Change Map demoted to coverage.
5. What sits at the top of the notes costs most. A wrong headline or a false 💥 Breaking bullet misstates the release where every reader looks; both of the user's blind picks for today's notes carried the new prompt's errors in the opening lines. Every rule that puts something at the top — the headline, the Breaking section — is biased to omission when unsure.

### Open Threads

- Whether the format-neutral closing reminder keeps the headline in — the experiment ran only the wording that names it; measured in the tuning under the Shipping Bar (Prompt Overrides And Reminder).
- False claims are the count furthest from the Shipping Bar (34 against a ceiling of 25), then missing items (13 against 9); where the tuning goes first (True To The Release).
- Two-body releases tend to lose their headline or get half of one under the rules that keep grab-bag headlines out — recall to tune, the rule unchanged (No Clear Headline).
- Whether part calls leave the headline out and the merge writes it — E1 ran no chunked release; checked in the first tuning runs, a part headline or a wrong merged headline holding shipping (Checking The Chunked Path).
- An agent steering the notes, or supplying its one line, belongs to the roadmap's agent-authored release notes item; the review gate's one-time steer is the existing mechanism it would reuse (Room For A Supplied Headline).
- The mint-release-tool specification (the by-eye criterion, the Change Map's ranking role, the six-section format) and token-aware-diff-sizing's merge ranking by the Change Map are superseded once this feature ships (Shipping Bar, Change Map Signals, Breaking Changes).

### Current State

- **Resolved:** the headline's shape, when it appears and how it is picked; part calls write none and the merge alone does; mint's shared wording stays format-neutral and override repos own their format; no route designed for a supplied headline, none foreclosed; the provider title stays the bare tag; 💥 Breaking first; user-facing only; ordering by user weight; the Change Map as coverage only; fixed section order with placement against the previous release; the accuracy rules with no new inputs; a release with nothing users meet gets one maintenance line; the shipping bar — every count at least as good as today's prompt on E1's held-out releases, each on its own, with the tuned wording kept general.
- **Uncertain:** how far wording brings false claims and missing items under the bar, whether the neutral reminder holds the headline, and whether the chunked path leaves the headline to the merge — all settled by measurement, not by decision.
