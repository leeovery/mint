# Research: Release-Notes Quality

Can AI release-note quality be lifted beyond what a raw textual diff allows — especially for large releases where big diffs summarise to mush? This thread explores enriching the *input* to the AI (AST / semantic structure / other signals) rather than further tuning the prompt and output format.

## Starting Point

What we know so far:
- **Prompted by**: Suspicion that AI release-note quality can be lifted beyond a raw textual diff, especially for large releases where big diffs summarise to mush (echoes the first discussion's `max_diff_lines` cost/quality note).
- **Already knows**: The first discussion tuned the AI prompt and output format but always fed the AI a raw textual diff. This thread explores enriching the *input* instead. Framed explicitly as speculative — an open "what is possible" question.
- **Starting point**: Technical feasibility — feeding AST or richer semantic structure of the change (or some other signal) to produce more accurate, meaningful notes.
- **Constraints**: none

---
