# Research: Release-Notes Quality

Can AI release-note quality be lifted beyond what a raw textual diff allows — especially for large releases where big diffs summarise to mush? This thread explores enriching the *input* to the AI (AST / semantic structure / other signals) rather than further tuning the prompt and output format.

## Starting Point

What we know so far:
- **Prompted by**: Suspicion that AI release-note quality can be lifted beyond a raw textual diff, especially for large releases where big diffs summarise to mush (echoes the first discussion's `max_diff_lines` cost/quality note).
- **Already knows**: The first discussion tuned the AI prompt and output format but always fed the AI a raw textual diff. This thread explores enriching the *input* instead. Framed explicitly as speculative — an open "what is possible" question.
- **Starting point**: Technical feasibility — feeding AST or richer semantic structure of the change (or some other signal) to produce more accurate, meaningful notes.
- **Constraints**: none

## Prior Context (from knowledge base)

What's already settled that this thread builds on:

- **First discussion — AI release-notes quality (parked child).** Settled the *prompt/output* side: diff-exclusion tiers (`CHANGELOG.md` always; `version_file` strategy-aware; `diff_exclude` globs), `max_diff_lines` (default 50000) reframed as a **cost + quality** guard ("a huge diff is slow, costly, and summarises to mush"), and a default format (TL;DR one-liner + emoji-headed sections, bolded notable features, no-preamble rule) plus two-knob prompt config (`notes_context` inject / `notes_prompt` override). **All of this always feeds the AI a raw textual diff.** This research thread is the parked child that asks whether enriching the *input* does better.
- **Commit-command engine boundary — names this thread directly.** The AI message engine is a **three-layer split**: L1 = context builder (git-aware, produces "the content to describe"), L2 = AI message engine (**git-unaware, content-agnostic** — "context in, message out"), L3 = per-verb glue. The load-bearing property: *"The input being a diff is incidental; L2 just sees 'content.'"* Explicitly: *"this is what makes the separate **release-notes-quality** research thread cheap: enriching the input (AST/semantic signal instead of a raw diff) swaps L1's output with **zero change to L2**. The boundary was chosen partly because it absorbs that future work."*

**Implication for this research:** the architectural home is already carved out. Anything this thread proposes lives inside **L1 (the context builder)** — it changes *what content is assembled*, not the engine, prompt-ownership model, or sinks. That's the lens to keep.

---
