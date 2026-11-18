# Review Tracking: token-aware-diff-sizing - Input Review

## Findings

### 1. What a failed `r` on single-pass notes goes through

**Source**: No source decides this. Checked in `discussion/token-aware-diff-sizing.md`:
- "Regenerate on Chunked Notes": the Decision covers chunked notes only, where `r` re-runs only the merge. Its "When the merge fails during `r`" entry says only that a failed `r` aborts, as on a single-pass release.
- "AI-as-Error-Classifier as Primary Trigger", "Under-Cap Overflow: Halving" and "Proactive Size Ceiling": these set the trigger, halving and the cap for the notes call. None mentions `r`.
- "Remediation at the Floor" and "Floor Wording and Guidance": these say nothing about `r`.
- "Exclusion Suggestions Before the Notes" and "Exclusion Check Mechanics": these place the check before the notes and say nothing about `r`.
**Category**: Unsourced decision
**Move**: route
**Affects**: §15.1 `r` on chunked notes (also §12.1's "A failed `r` is not the floor; it aborts")

**Problem**:
The specification says this about single-pass notes: "`r` re-runs the same AI path as built, with the nudge appended: from the size check onward (§7.2), so the ladder applies to it as to the first attempt. It never re-runs the exclusion check."

No source makes that call. The record decides two things about `r`: on chunked notes it costs one merge call, and a failed `r` aborts whatever `on_notes_failure` says. It never says whether the new degradation path reaches a single-pass `r`, meaning the size check against `max_prompt_bytes`, the classify call on failure, and halving into a chunked wave on a SIZE answer.

The operator sees the difference whenever a nudged `r` fails:
- **Under the specification's rule:** the failure costs an extra classify call. A SIZE answer turns the one-call `r` into a halved, parallel chunked wave and then a merge. That means minutes of calls against the usage window before the run shows notes or aborts, and the ✗ line takes the classifier's cause phrase.
- **Under the other reading:** the run aborts straight away with the AI's verbatim output.

The spec also never says what happens to the chunk notes when a single-pass `r` has turned into a wave and a chunk then fails. Its save-the-chunk-notes rule for a failed `r` covers only `r` on chunked notes.

Skipping the exclusion check on `r` is unsourced too. It is the side any operator would choose: the answers are already recorded, and re-asking would bring back pauses already answered.

**Proposed Text**:

**Resolution**: Routed
**Notes**: Derived from the record's own rules for release-notes calls (no call over the cap, the classifier on any failed generation call, halving) and landed in the discussion as a new subtopic, `r` on Single-Pass Notes: a failed `r` aborts and saves any chunk notes the ladder produced first, and `r` never re-runs the exclusion check. Specification §15.1 re-aligned.

---

## Observations

- §1.2's exact figures (~1,335,248 tokens requested, ~532,481 of them mint's) do not appear in the discussion, which gives ~532k for mint's prompt and ~803k of overhead. They are consistent background figures, not a decision.
- §6.6's `--autostash` bullet appears in no source. It says the answers go to the committed `.mint.toml`, and a stashed edit that conflicts on restore keeps its stash entry. This follows from the stash running at startup, before the check, so it reads as a consequence rather than a choice.
- Under `regenerate --all`, an expired login or usage limit hits each version separately. Each version gets its own notes call, retry and classify call, because the stop-the-wave rule is per version's wave. The sources accept repeated per-version output but never consider stopping the batch. These failures come back fast, so the cost is mostly noise.
