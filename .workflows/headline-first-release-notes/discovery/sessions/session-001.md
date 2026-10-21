# Discovery Session 001

Date: 2026-10-21
Work unit: headline-first-release-notes

## Description (as of session)

Release notes open by naming the release's main feature, with the smaller changes that shipped alongside it listed after.

## Seed

(none)

## Imports

(none)

## Map State at Start

(n/a — single-topic work)

## Exploration

Pulled from the product roadmap (horizon Next), where it was parked on 2026-10-16 during the token-aware-diff-sizing discussion (`.workflows/token-aware-diff-sizing/discussion/token-aware-diff-sizing.md`, Summary → Open Threads). It was deliberately kept separate from big-release handling: that feature's chunked merge pass "inherits whatever format exists when it ships", and it has since shipped on the existing format.

### The problem as recorded

The default notes prompt (`DefaultPrompt`, `internal/notes/prompt.go`) is category-first: Keep a Changelog sections with emoji headers in a fixed order (✨ Added, 🔧 Changed, ⚠️ Deprecated, 🗑️ Removed, 🐛 Fixed, 🔒 Security), one-line one-sentence bullets, one bullet per notable change, ranked by the Change Map. It carries an explicit rule — "NO TL;DR, NO summary paragraph, NO preamble — start directly with the first section header" — and the closing `OutputReminder` appended after the diff restates "no preamble … nothing before or after the notes". Between them there is no place to declare what a release is about, so a release's main feature can land as one bullet among sideline changes, sorted into its category like any other.

Confirmed against the as-built code on 2026-10-21: the category-first format, the no-TL;DR rule and the reminder's nothing-before-the-notes wording all still stand. Any headline the format gains has to be reconciled with both — as written, they would forbid it as a preamble.

### Ground from the source discussion

- **Salience is cross-cutting today, but unspoken.** The prompt already asks the AI to rank by the Change Map (new structure weighted above raw size). Ranking happens within categories; nothing surfaces the top-ranked change above them. The token-aware discussion's Stitching and Salience section framed the chunk problem as "no chunk knows whether its slice is the headline or a footnote" — the merge call is where the whole release is ranked.
- **The chunked path carries the release's instructions into every call.** Per Chunk Wave Mechanics (as shipped), chunk calls and the merge call both carry the release's own instructions — the default prompt or the operator's `[release].prompt` override, with `context` — plus mint's framing. A format change to the default prompt therefore reaches chunk calls and the merge. Open: whether chunk calls should name a headline at all (each sees only a slice) or whether picking the headline is the merge's job alone, with chunks writing plain partial notes.
- **The missing-slice rule leans on this item.** When a Chunk Fails rejected merging surviving chunks with a gap because "the missing slice can be the release's headline feature" — notes that read as complete while leaving out the biggest change are named there as "the one the parked headline-first roadmap item targets".
- **Today's manual route to a headline.** On an attended release the review gate's `r` regenerates with a one-time nudge — the recorded example is "lead with the auth work". That is the only way an operator can steer emphasis today, and only at a terminal; on chunked notes `r` re-runs only the merge, because emphasis and ordering are decided there.

### Open questions carried in

- **What a headline looks like in the format** — a lead line or short paragraph naming the main feature above the categorised sections, or the main feature pulled out of its category to the top; and how that squares with the one-line-bullet discipline and the no-preamble rule.
- **Releases with no single main feature** — a maintenance or grab-bag release; whether the notes then omit the headline rather than inventing one.
- **How the AI picks the headline** — from the diff and Change Map alone (mint's diff-only view), with the known limit that the session that built a feature knows the headline better than the diff does.
- **A supplied headline.** The adjacent roadmap item, agent-authored release notes (not pulled), proposes taking a one-line statement of what the release is about from the outer agent. Pulled alone, this work designs the format and how mint's own AI fills it; whether a supplied statement overrides the AI's pick is that item's question, and the format should not foreclose it.
- **Reach beyond the default prompt** — an operator's `[release].prompt` override replaces the default wholesale, so a format change reaches only repos on the default. Bodies stay byte-identical (no reformatting by mint), so the headline has to come from the AI's output, not be injected afterwards.

## Edits

(none)

## Topics Identified

(none)

## Conclusion

(none)
