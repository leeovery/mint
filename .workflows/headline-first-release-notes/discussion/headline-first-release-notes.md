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

### Decision

A one-sentence headline above the first section, summarising the release's main body of work; beneath it the categorised sections exactly as today, listing every notable change, the main feature's pieces included. The headline and the sections overlap by design — summary, then detail — and the sections are never thinned to avoid it.

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

### Decision

A release with two bodies of work of equal weight names both in its one headline sentence — at most two, and still one sentence, benefit-led per Headline Shape. Beyond two co-equal bodies of work is not designed for: the one-sentence limit stands, and the case is left to how the model meets it (YAGNI at mint's cadence). Deciding factor: a headline that names one of two equal halves misrepresents the release, and omitting it costs the lead on the largest releases. Confidence: high. (Resolves review-001 F4, which raised the same case independently.)

---

## Summary

### Key Insights

### Open Threads

### Current State
