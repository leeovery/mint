# Review Tracking: token-aware-diff-sizing - Input Review

## Findings

### 1. What follows `c` or `e` at a single-version regenerate's floor

**Source**: discussion, "Floor Wording and Guidance" (Journey: "A chosen body is not AI output, so it goes on to the review gate a non-AI body gets as built … accept, edit, abort; no `r`"; Initial Decision, "After `c` or `e`"); "Remediation at the Floor" (Initial: "Single-version regenerate asks too" and "An empty `e` is refused"); "Notes-File Backdoor" (2026-10-17 revision: on regenerate a supplied body gets "the gate without `r`")
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §12.3 Regenerate at the floor

**Problem**:
An operator at a terminal regenerating one version can hit the floor when the AI call fails. They are then offered commit subjects (`c`), the editor (`e`) or abort (`a`). The specification never says what happens to the chosen body before it overwrites the version's published notes. On release, two rules cover this: the chosen body goes to the non-AI review gate (accept, edit, abort, no `r`), and an empty `e` is refused. The regenerate floor says only "the same choice". An implementer has three ways to guess wrong:
- They reuse regenerate's existing notes-review gate, `r` included. Pressing `r` on commit subjects or hand-written notes re-runs the AI call that just failed. If it fails again, a failed `r` aborts the regenerate, and the operator loses the body they chose.
- They write the body straight through. The version's provider release or changelog is replaced with commit subjects the operator never saw. They find out only when they read the published page.
- They don't carry the empty-`e` refusal over. An editor closed without writing, the usual way to back out, blanks the version's published notes.

**Proposal**:
Add to the regenerate bullet: an empty `e` is refused and returns to the choice, as on release, and the chosen body goes on to regenerate's own review gate without `r` (accept, edit, abort). The record settles this:
- The reason the record gives for sending a chosen body to the non-AI gate is "a chosen body is not AI output". That holds whichever verb runs the floor.
- On regenerate, the record already gives a non-AI body (a supplied `--notes-file` body) "the gate without `r`".
- The empty-`e` refusal exists because accepting an empty body would ship an empty provider release. Regenerate writes the provider release, so the same grounds apply.

The other options are dead:
- Offering `r`: there is no AI output to redo, and a failed `r` aborts.
- Skipping the gate: the record's own reason the floor suits regenerate is that it "already runs attended with its own review gate", which is where the operator sees what will replace the published notes.

**Current**:
- **`regenerate <version>`, attended:** the same choice, whatever `on_notes_failure` holds, finishing in the same run. `c` gives that version's commit subjects, `e` opens the editor (on the saved chunk notes when a chunk or the merge failed), and `a` aborts, which is as-built regenerate's one outcome and stays the default. Regenerate already runs attended with its own review gate, and stopping would only send the operator to re-run with `--notes-file`. Unattended, it stops with guidance, as release does under `abort`.

**Proposed Text**:
- **`regenerate <version>`, attended:** the same choice, whatever `on_notes_failure` holds, finishing in the same run. `c` gives that version's commit subjects, `e` opens the editor (on the saved chunk notes when a chunk or the merge failed), and `a` aborts, which is as-built regenerate's one outcome and stays the default. As on release, an empty `e` is refused and returns to the choice, and the chosen body goes on to regenerate's own review gate without `r`: accept, edit, abort, since there is no AI output to redo. Regenerate already runs attended with its own review gate, and stopping would only send the operator to re-run with `--notes-file`. Unattended, it stops with guidance, as release does under `abort`.

**Resolution**: Pending
**Notes**:

---

## Observations

- A saved chunk-notes file from an earlier failure of a tag stays on disk when a later failure of the same tag finishes no chunk and so saves no file. That later failure names no file, so nothing points at the stale one. The sources address overwriting and the no-file case, but not this combination.
