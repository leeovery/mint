# Review Tracking: token-aware-diff-sizing - Gap Analysis

## Findings

### 1. The progress example merges fewer parts than its own halving left

**Source**: Specification analysis
**Category**: Contradiction
**Move**: settled
**Priority**: Important
**Affects**: §16.1 Pretty mode: one notes line, updated (the example block). Colliding reading: §16.1's bullet "When halving splits a part, the total grows by the parts it adds", and §8.6's merge call, which carries every chunk's notes.

**Problem**:
The progress example follows one chunked run. It starts at 6 parts, retries part 4 as 3 smaller parts, and then reads `merging 6 parts` and `Notes written (6 parts, merged)`. The rule beside the example says the total grows by the parts a halving adds. So by the merge that run has 8 parts, and the merge call takes the notes of all 8.

An implementer who copies the example's lines makes the merge line and the done line count the first wave's parts. The operator watches the count reach `8 of 8 done`, then reads `merging 6 parts`. That looks as though two parts' notes were dropped before the merge, which is exactly the silent loss this feature exists to end. The operator sees it at the point where they are deciding whether to trust the notes.

**Proposal**:
The example's merge line and done line read 8. Two things in the specification decide this. The total grows by the parts a halving adds, so part 4 retried as 3 takes 6 parts to 8. The merge carries every chunk's notes, so it merges 8. Counting the first wave's parts at the merge is dead. It contradicts the rule on how the total grows, and it shows the operator a smaller count than the run just finished.

**Current**:
```text
⠋ Writing notes: merging 6 parts
✓ Notes written (6 parts, merged)
```

**Proposed Text**:
```text
⠋ Writing notes: merging 8 parts
✓ Notes written (8 parts, merged)
```

**Resolution**: Approved
**Notes**:

---

### 2. `r` at `regenerate --all`'s per-version review gate on chunked notes

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §15.1 `r` on chunked notes (the "On regenerate" paragraph); context in §1.3's `regenerate --all` row and §12.3 Regenerate at the floor

**Problem**:
At a terminal without `-y`, `mint release regenerate --all` shows each fresh version's notes at a review gate that offers `r`. It is the same notes-review gate a single-version regenerate uses: as built, the batch's per-version step reaches `gateRegenerate` with that version's fresh AI regenerator, and skips the gate only under `-y`. Big versions in the batch now get chunked notes. The `r` rules for chunked notes cover a release and a single-version regenerate: `r` re-runs only the merge, and a failed `r` first saves the per-chunk notes and names the file. Nothing covers the batch's gate.

An implementer who leaves the batch's `r` on the version's fresh AI path, which is what it re-runs as built, makes a nudge on a chunked version re-run that version's whole wave. Every chunk call runs again, a few at a time, then the merge. That means minutes of waiting and a draw on the usage window, all for a change of emphasis. If that `r` fails, a failed gate ends the batch as built, and the version's chunk notes are gone, with no file to feed back. The operator notices when an `r` on one version of a long backfill sits through a full wave. They also notice when a failed `r` ends the batch with nothing saved.

**Proposal**:
The batch's per-version gate follows the single-version rules for `r`. On chunked notes it re-runs only the merge. On single-pass notes it re-runs the AI path from the size check onward. A failed `r` first saves that version's per-chunk notes, where the notes were chunked or the ladder made the `r` a chunked run. It names the file with that version's `mint release regenerate <version> --notes-file <path>`, and then ends the batch, as a failed gate does as built.

What determined it:
- The record chose a merge-only `r` because a nudge changes emphasis and ordering, and the merge decides both. That reason does not depend on which verb runs the gate, and the batch runs the same chunked path for each version.
- Saving before the abort keeps the wave's work. The record gives that reason for both a release and a single-version regenerate.
- A failed `r` is not the floor. So the batch's skip-and-continue floor does not take it, and the as-built end of the batch stands.

What leaned on the failed-`r` outcome: the rule that a failed `r` is not the floor, plus the as-built behaviour of a failed gate. The alternative that fits is to treat a failed `r` as a skip: the version keeps its published notes, the summary names it with its saved file, and the batch carries on. That matches the batch's resilience, but it makes a failed `r` a floor on one verb only.

**Proposed Text**:
Appended to §15.1, after the "On regenerate" paragraph:

> `regenerate --all` run at a terminal without `-y` shows each fresh version at the same review gate, and its `r` follows the same rules for that version: only the merge on chunked notes, the AI path from the size check onward on single-pass notes. A failed `r` saves that version's per-chunk notes where the notes were chunked or the ladder made the `r` a chunked run, names the file with that version's `mint release regenerate <version> --notes-file <path>` (§11.4, §13), and ends the batch, as a failed gate does as built. It is not the batch's floor, so the batch does not skip the version and carry on.

**Resolution**: Routed
**Notes**: Landed in the discussion as a new subtopic, `r` at `regenerate --all`'s Review Gates (merge-only `r` on chunked notes; a failed `r` saves that version's part notes, names its regenerate `--notes-file` command and ends the batch). Specification §15.1 carries the Proposed Text.

---

### 3. The documented `timeout` default stays 60 seconds while releases default to 180

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Move**: settled
**Priority**: Important
**Affects**: §18.2 The README (per-key tables bullet); context in §4.4 The release timeout and §18.1 The setup guide (the config reference)

**Problem**:
The feature splits the shipped per-attempt timeout by verb: 180 seconds on release and 60 on commit. Both verbs resolve through the shared `timeout` key first. The doc changes give `[release].timeout` its 180-second default and drop the setup guide's "default 60". They say nothing about the shared `timeout` row, and that row's default is the one that split. Left as it is, the README's table and the config reference that `mint setup` renders from the config metadata keep saying the timeout defaults to 60 seconds.

An operator whose big release ends `AI timed out` looks the setting up, reads that it defaults to 60, and sets `timeout = 120` to double the wait. That cuts every release call's deadline from 180 seconds to 120, so the next run times out sooner, on the same calls. An agent configuring a repo from the setup guide reads the same wrong figure.

**Proposal**:
The shared `timeout` row names both shipped defaults: 180 seconds on release, 60 on commit. Its config-metadata row says the same, so the setup guide's config reference follows. Any README line that states the timeout's default names both.

What determined it: the record gives each verb its own shipped default and resolves both through the shared key, and docs stay true to what is built (§18). A single figure on the shared row is therefore untrue for one of the two verbs, so the row states both. How the cell lays the two figures out is the builder's.

**Current**:
They show `[release].timeout`'s 180s default.

**Proposed Text**:
They show `[release].timeout`'s 180s default. The shared `timeout` row names both shipped defaults, 180 seconds on release and 60 on commit, since both verbs resolve through it (§4.4). Its config-metadata row says the same, so the setup guide's config reference follows, and any README line that states the timeout's default names both.

**Resolution**: Approved
**Notes**:

---

## Observations

- The split line's `3.1 MB release` does not say which size it reports, the post-exclusion diff or the composed prompt. Either tells the operator the run is big.
- After a halving grows the total, whether the split line's `in 6 parts` changes too, or only the `of N` count, is unstated.
- A `--dry-run -y` run meets both the dry-run rule (suggestions shown as at the pause) and the unattended rule (printed after the notes with the `diff_exclude` line). Either one shows the suggestions and writes nothing.
- On `regenerate --all`, the lower-the-cap hint would print once for each version that halved. Printing it once at the end of the batch would read cleaner.
