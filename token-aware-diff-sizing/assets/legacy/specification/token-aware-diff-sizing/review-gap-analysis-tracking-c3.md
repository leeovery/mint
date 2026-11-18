# Review Tracking: token-aware-diff-sizing - Gap Analysis

## Findings

### 1. The halving narration says "2 parts", which halving never produces

**Source**: Specification analysis
**Category**: Contradiction
**Move**: settled
**Priority**: Important
**Affects**: §16.1 Pretty mode: one notes line, updated (the example block and the single-pass halving bullet). Colliding reading: §10.1 When it applies, the halved target and the re-cut rule.

**Problem**:
Halving aims each new call at half the size of the call that failed, counting everything in that call: instructions, Change Map, framing, diff and reminder. Every smaller call still carries the whole fixed part, so the room left for diff in each one is less than half the failed call's diff. Re-cutting the slice therefore always gives three parts or more, never two. A single-pass call of 1.4 MB with a 200 KB fixed part gets a 700 KB target, 500 KB of room each, and 1.2 MB of diff to place, which makes three parts.

The narration says otherwise. The progress example reads `part 4 was too big for the model, retrying it as 2 smaller parts`, and the single-pass bullet gives the exact line `too big for the model in one call, retrying in 2 parts`. An implementer can resolve this in one of two ways, and both cost the operator something:

- **Hardcode "2".** The operator watching the run sees `retrying in 2 parts` followed straight away by a count of three or four parts. That looks like a bug in the progress line, at the point of a long run where they are deciding whether mint is working or hung.
- **Make halving match the narration** by splitting the slice's diff into two halves. Each half then carries the full fixed part, so each call sits above half the failed size. The bound "three halvings take a slice to an eighth of the size that first failed" no longer holds. With a large Change Map, the calls never get much smaller than the fixed part, and a release that the specified halving would have rescued stops at the floor instead.

**Proposal**:
The narration states the number of parts the re-cut actually produced, both for a halved single-pass call and for a halved part. The halving rule decides this: the target counts the whole call, and the re-cut uses the first wave's packing rules. The part count is whatever packing gives, and that is always more than two. The split line already counts parts, and the sentence about a halved part's split growing the total already treats the count as variable.

The alternative that also fits is to drop the count (`retrying in smaller parts`). That loses information the split line shows anyway a moment later.

**Current**:
```text
⠋ Writing notes: part 4 was too big for the model, retrying it as 2 smaller parts
```

- A single-pass call that halves (§10.1) shows `Writing notes: too big for the model in one call, retrying in 2 parts`.

**Proposed Text**:
```text
⠋ Writing notes: part 4 was too big for the model, retrying it as 3 smaller parts
```

- A single-pass call that halves (§10.1) shows `Writing notes: too big for the model in one call, retrying in {n} parts`. Here, and when a part halves, n is the number of parts the re-cut yields (§10.1).

**Resolution**: Approved
**Notes**:

---

### 2. A halving round with no room says the cap is too small, while its next move says to lower the cap

**Source**: Specification analysis
**Category**: Source defect
**Move**: route
**Priority**: Important
**Affects**: §12.4 The ✗ line and the next moves (cause table)

**Problem**:
The specification knows two ways a call can be left with no room for diff, and it treats them as different causes:

- **Operator text** fills the cap: an outsized `[release].prompt` or `context`. The fix is to shorten that text.
- **A halving round** is aimed below the call's fixed part: the model rejected even a call about twice the fixed part's size. The next moves group this case with "SIZE with no halving left" and tell the operator to lower `max_prompt_bytes` or switch to the lean command.

The cause table gives both cases one phrase: `no room for the diff under max_prompt_bytes`. For the halving case, that phrase says the opposite of the case's own advice. It reads as "the cap is too small to leave room", which points at raising the cap.

The harm is worst at a terminal. Attended runs show no next moves, only the ✗ line, the AI's output and the choice. The operator sees `no room for the diff under max_prompt_bytes (part 3 of 6)` above the model's own "prompt is too long" message. They raise `max_prompt_bytes`, re-run the release, wait through the whole wave again, and fail the same way. Unattended, the output prints the phrase and then a move telling the reader to lower that same setting, so the two lines contradict each other.

The discussion's Floor Wording and Guidance gives both no-room causes one phrase, `no room for the diff under max_prompt_bytes`, while its next moves send a halving round's no-room to the size guidance; the source carries the defect, so the phrase is the discussion's to correct.

**Current**:
| SIZE with no halving left | `too large for the model, even in smaller parts` |
| No room for diff (§8.7) | `no room for the diff under max_prompt_bytes` |

**Resolution**: Routed
**Notes**: Derived and landed in the discussion as a dated revision of Floor Wording and Guidance's Decision: a halving round with no room reads `too large for the model, even in smaller parts`, and the no-room phrase names operator text only. Specification §12.4's cause table re-aligned.

---

## Observations

- Under `-y`, the presenter's gate seam auto-accepts a gate's default. If the exclusion check's pauses were prompted at all under `-y`, the default Keep would be written to `diff_keep` and committed. The rule that an unattended check writes nothing fixes the product behaviour; how it is kept is the builder's.
- The final-failure list that ends a wave does not name a halving round with no room. Its general rule, that anything finishing a chunk for good ends the wave, covers it.
- The rule that `--notes-file` with `--all` or `--source` is a usage error appears in both the flag's description and its refusals list.
