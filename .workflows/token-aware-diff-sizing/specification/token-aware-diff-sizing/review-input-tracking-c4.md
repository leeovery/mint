# Review Tracking: token-aware-diff-sizing - Input Review

## Findings

### 1. The halving progress line's part count

**Source**: No source decides this. Checked in `discussion/token-aware-diff-sizing.md`:
- "Narration Wording": the Decision fixes the line as "`Writing notes: too big for the model in one call, retrying in 2 parts` when a single-pass call halves". Its Journey reasons "Halving a part into two adds a part".
- "Progress Narration for a Chunked Run": the mock the user agreed reads `part 4 was too big for the model, retrying it as 2 smaller parts`.
- "Halving Bound and the Lower-the-Cap Hint": sets the target at half the composed size of the call that failed and re-cuts by the first wave's packing rules. It decides no narration wording.
**Category**: Unsourced decision
**Move**: route
**Affects**: §16.1 Pretty mode: one notes line, updated (the example block and the single-pass halving bullet)

**Problem**:
The specification says this about the halving line: "A single-pass call that halves (§10.1) shows `Writing notes: too big for the model in one call, retrying in {n} parts`. Here, and when a part halves, n is the number of parts the re-cut yields (§10.1)." Its example reads `part 4 was too big for the model, retrying it as 3 smaller parts`.

No source document makes that call. The record's own decision on this user-facing line still says "retrying in 2 parts", and it reasons as if halving splits a part in two. The halving rule the record also decided points the other way. Every halved call still carries the whole fixed part, so a re-cut always yields three parts or more. Built from the record's wording, an operator watching a long run would see `retrying in 2 parts` and then a count of three or four. At the moment they are judging whether mint is working or hung, the progress line would look broken. The specification and the record currently disagree on what this line says, and only the record can settle it.

**Proposed Text**:

**Resolution**: Pending
**Notes**:

---

### 2. `r` at single-version regenerate's review gate on chunked notes

**Source**: `discussion/token-aware-diff-sizing.md`. The sources do not address this case. These sections set the ground:
- "Regenerate on Chunked Notes": scoped to "On an attended release". Its deciding factor is that a nudge changes emphasis and ordering, which the merge decides, so `r` should cost one call, not the whole wave.
- "`r` on Single-Pass Notes": the ladder from the size check onward. A failed `r` aborts after saving the chunk notes.
- "Remediation at the Floor": single-version regenerate "already runs attended with its own review gate".
- "Scope Across the Three Consumers": single regenerate takes chunking through the shared generator.
- "Notes-File Backdoor" (2026-10-17 revision): a version already out takes finished notes through `mint release regenerate <version> --notes-file <path>`.
**Category**: Gap/Ambiguity
**Move**: settled
**Affects**: §15.1 `r` on chunked notes (context: §1.3's regenerate fresh-source row)

**Problem**:
A single-version regenerate from a fresh source now gets chunked notes for a big version, and its review gate offers `r` on AI notes. The rules for `r` are written for a release:
- On chunked notes, `r` re-runs only the merge, and a failed `r` saves the chunk notes first.
- Those notes can still go out because "the abort lands before any tag".
- The consumer table lists regenerate's share of the feature (the size cap, chunking, classification, halving, the floor's choices, `--notes-file`) and leaves `r` out.

Suppose an implementer builds regenerate's `r` from that. An operator regenerates a big version and presses `r` to lead with a different change. They wait through the whole wave again, every chunk call a few at a time and then the merge, and it draws on their usage window. A release's `r` costs one call. If that `r` fails, the saved notes have no stated route onto a version that is already out.

**Proposal**:
Regenerate's `r` follows the release's rules:
- On chunked notes, it re-runs only the merge.
- On single-pass notes, it re-runs the AI path from the size check onward.
- A failed `r` aborts after saving the chunk notes. The saved file goes onto the version with regenerate's own `--notes-file`.

What determined it: the record chose merge-only `r` because a nudge changes emphasis and ordering, and the merge decides those. That reason has nothing to do with which verb runs the gate. Regenerate's fresh source runs the same chunked path. The Notes-File Backdoor revision already routes a version that is out through `mint release regenerate <version> --notes-file <path>`. The other reading has regenerate's `r` re-run the whole wave for an emphasis nudge. No operator would choose that. It is the cost the record's decision on `r` set out to avoid.

**Proposed Text**:
Appended to §15.1, after the "When `r` fails" bullets:

**On regenerate.** A single-version regenerate from a fresh source has its own review gate, and its `r` works the same way. On chunked notes it re-runs only the merge call. On single-pass notes it re-runs the AI path from the size check onward. A failed `r` aborts the regenerate. If the notes were chunked, or the ladder turned the `r` into a chunked run, mint first saves the per-chunk notes and names the file (§11.4). The version is already out, so the saved notes go onto it with `mint release regenerate <version> --notes-file <path>` (§13).

**Resolution**: Pending
**Notes**:

---

## Observations

- The record notes that a 200k-token window holds about 400 KB of diff at most ("Size Cap Key, Default and the Release Timeout"). The specification says only that an operator on a smaller window lowers the cap. The README entry for `max_prompt_bytes` could give that figure.
- The headline-first roadmap note says the chunked merge inherits whatever notes format exists when headline-first ships. The parked item in §1.4 does not carry that.
