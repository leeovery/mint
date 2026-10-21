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

## Summary

### Key Insights

### Open Threads

### Current State
