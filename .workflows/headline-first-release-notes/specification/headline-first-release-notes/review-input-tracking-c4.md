# Review Tracking: Headline First Release Notes - Input Review

## Findings

### 1. A change to mint's saved-file guidance that no source decides

**Source**: No source decides this. Checked: the discussion's Chunked Path (Context: the saved file is for "the operator to tidy and release with `--notes-file`, or to pick up with `e`"; Decision 2026-10-22 revised: the saved file "holds the parts' sections and any such line, which the operator removes while tidying, as with the notes-missing line"; Decision Initial; the sibling check, "this decision changes what the part calls return, not the file's rule, which stands"), A Release With Nothing Users Meet (Journey and Decision), Checking The Chunked Path, Scope (the fix lives in the notes prompt or the signals mint hands the AI), and the Summary (Open Threads, Current State). None of them says mint's guidance for releasing a saved file changes. The sentence first appears in an earlier review cycle's tracking file, which is not a source.
**Category**: Unsourced decision
**Move**: route
**Affects**: §8 The Chunked Path (the saved-part-notes bullet)

**Problem**:
The specification commits mint to rewording its user-facing guidance on releasing a saved part-notes file: "mint's guidance on releasing a saved file tells the operator to delete these lines as well as the marker." That guidance is outside the prompt. It covers the README's steps for the saved file, and possibly the next-move line mint prints when a chunked run's merge fails. The discussion never decided this. It decides only that a part with nothing for the notes returns one line, that the line lands in the saved file when the merge fails, and that the operator removes it while tidying, as they remove the notes-missing marker. The two readings give the operator different things. If mint's checklist still names only the marker, an operator or agent who follows it to the letter ships the part's line. The tag, `CHANGELOG.md` and the provider release then carry a sentence saying one part held nothing, in the middle of the release's sections. If the guidance does change, the feature reaches past the default prompt and the shared wording into README prose and failure narration. The specification's own account of its reach doesn't cover that: the fix lives in the prompt or in mint's shared wording, and the saved part notes stay as they are. Either way the build rests on a call the record never made, and whoever next revisits the Chunked Path decision from the discussion won't find it there.

**Proposed Text**:

**Resolution**: Routed
**Notes**: Settled by derivation from the Chunked Path revision (the operator removes these lines as with the marker) and mint's existing marker guidance; landed in that revision entry (written this sitting). §8 already carries the same sentence.

---

## Observations

- The merge framing keeps "combine related changes across parts into single bullets" without the combine rule's narrow reading (the tweaks of one change, never a feature's separate capabilities). The discussion names it as one of the pulls toward folding a feature into one bullet, and the combine rule's reopen condition already covers that.
- The 🔧 Changed definition's "any smaller change to how it works that is not a fix" goes further than the discussion's Section Placement decision, which names only a rework. It is derived with one live side: the change must be listed, its usual section is the only home, and Added and Fixed are ruled out. The discussion records only the rework wording.
