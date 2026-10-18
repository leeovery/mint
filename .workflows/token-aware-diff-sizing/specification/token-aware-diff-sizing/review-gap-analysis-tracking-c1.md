# Review Tracking: token-aware-diff-sizing - Gap Analysis

## Findings

### 1. Under `fallback`, the release ships commit subjects without saying the AI failed or why

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Move**: settled
**Priority**: Important
**Affects**: §12.1 What applies, by mode

**Problem**:
Under `on_notes_failure = fallback`, the release ships the commit-subject list, and the only output specified is a warning about the saved chunk-notes file. Most failures leave no file: any single-pass failure, an expired login, a usage limit, a timeout. For those, nothing says the operator is told the AI failed. Even when a file exists, the warning as specified gives no cause and none of the AI's output.

An implementer building the row as written ships commit-subject notes silently. `fallback` is the setting for unattended releases that must not stop, so a CI pipeline with an expired login would ship commit-subject notes release after release with no sign of the cause. Someone notices only when they read the published notes and wonder where the AI notes went. Even then, nothing in the run's output tells a login failure from a size failure.

**Proposal**:
The fallback warning always appears. It leads with the cause phrase, carries the AI's captured output verbatim beneath it, and adds the saved file and the regenerate command where a file exists.

The spec's own rule decides it: the classifier adds to the AI's verbatim output and never replaces it, as subprocess diagnostics always do. The spec already applies that rule, for the same reason, to two routes that dropped the output: `regenerate --all`'s skip warning, where an expired login across a batch would otherwise read only as `AI call failed`, and `mint commit`'s AI-failure warning. The fallback row is the one AI-failure route still naming no cause.

One alternative fits the row's current wording: warn only when a chunk-notes file was saved. That leaves every single-pass failure silent, which is the dead end this feature exists to end.

**Current**:
| `on_notes_failure = fallback` | No choice. The commit-subject list (§14) ships. A warning names the saved file, where one exists, and the `mint release regenerate <version> --notes-file <path>` command that puts it on the version. |

**Proposed Text**:
| `on_notes_failure = fallback` | No choice. The commit-subject list (§14) ships, with one warning whatever the cause: the cause phrase (§12.4), the AI's captured output verbatim beneath it, and, where a chunk-notes file was saved (§11.4), its path and the `mint release regenerate <version> --notes-file <path>` command that puts it on the version. |

**Resolution**: Routed
**Notes**: Landed in the discussion as a new subtopic, The Fallback Warning, with Floor Wording and Guidance's fallback bullet amended. Specification §12.1's fallback row carries the Proposed Text.

---

### 2. A dry run that reaches the floor

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §12.1 What applies, by mode; §11.4 The saved chunk-notes file

**Problem**:
A dry run of `mint release` runs the whole notes path, including the exclusion check, a chunked wave and the merge, so it can end at the floor. The floor's rules are set only by mode and `on_notes_failure`. Read literally, an attended dry run under `abort` gets the `c` / `e` / `a` choice and then a review gate, for a run that ships nothing. Whatever the operator picks is thrown away when the dry run ends, including notes they spent minutes writing in `$EDITOR`.

An implementer could also treat the chunk-notes file as a write a dry run must not make, and skip it. Then a dry run that failed mid-wave keeps none of its work, and the real run pays for the whole wave again before failing the same way.

The exclusion check settles its own dry-run case (suggestions shown as at the pause, nothing asked or written). The floor has no such rule.

**Proposal**:
A dry run never asks for the floor's choice. Under `abort` it renders what an unattended run renders and ends as a failed run: the ✗ line, the AI's verbatim output, the saved-file pointer and the cause-fitted next moves. Under `fallback` it previews the commit-subject list as the body that would ship, with the fallback warning. The chunk-notes file is saved as on a real run.

What leaned:
- The spec's dry-run rule for the exclusion check is "nothing is asked".
- A dry run produces no release, so a body chosen or written at the floor has nowhere to go.
- The saved file sits outside the repo beside the dry-run cache, which a dry run already writes. Its `--notes-file` pointer lets the real run release the wave's work without re-running it.

Alternatives:
- Offering the choice in a dry run. This side is dead: anything chosen is discarded.
- Saving no file. This keeps the dry run's writes to the notes cache, at the cost of a repeated wave on the real run.

**Proposed Text**:
In §12.1, after "A failed `r` is not the floor; it aborts (§15).":

> **A dry run at the floor** asks nothing, whatever the mode. Under `abort` it renders the ✗ line, the AI's verbatim output, the saved chunk-notes pointer where a file exists (§11.4) and the cause-fitted next moves (§12.4), as an unattended run does, then ends as a failed run. Under `fallback` it previews the commit-subject list as the body that would ship, with the fallback warning. The chunk-notes file is saved as on a real run: it sits outside the repo beside the dry-run cache, and the real run can release it with `--notes-file` instead of repeating the wave.

**Resolution**: Routed
**Notes**: Landed in the discussion as a new subtopic, A Dry Run at the Floor. Specification §12.1 carries the Proposed Text.

---

### 3. A merge call the model rejects for size gets the wrong cause and the wrong advice

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Move**: settled
**Priority**: Important
**Affects**: §12.4 The ✗ line and the next moves; §11.3 A failed merge

**Problem**:
When the merge call overflows the model and the classifier answers SIZE, the spec makes it a failed merge, never halved. It gives that case no cause phrase or guidance of its own.

The nearest row in the cause table, "SIZE with no halving left", renders `too large for the model, even in smaller parts (merging)`. That is untrue, because the merge was never split. Its Size moves tell the operator to lower `max_prompt_bytes`. Lowering the cap cuts the release into more parts, and the merge then has to hold the same release's part notes, no smaller. An operator who follows the advice re-runs the release, waits through the whole wave again, and the merge fails again.

**Proposal**:
A SIZE answer on the merge call shares the over-cap merge's phrase, `part notes too large to merge`. Its next moves are the Always moves, led by the saved part notes and `--notes-file`, which is what the spec's own failed-merge example already says.

What determined it:
- The halving rule makes a merge SIZE a failed merge, not an exhausted halving.
- The failed-merge example's guidance is the saved-notes route.
- Over the cap or over the model's window, the operator's next move is the same.

The alternative that fits is a phrase of its own naming the model, such as `part notes too large for the model to merge`. It costs a second phrase for the same next move.

**Current**:
| Merge prompt over the cap (§8.7) | `part notes too large to merge` |

- **Size** (SIZE with no halving left; no room caused by a halving round):

**Proposed Text**:
| Merge too large: its prompt over the cap (§8.7), or a SIZE answer on the merge call (§11.3) | `part notes too large to merge` |

- **Size** (SIZE with no halving left on a single-pass or chunk call; no room caused by a halving round):

And after the Size moves' list:

> A merge too large, by the cap or by the model, takes only the Always moves, led by its saved part notes. Lowering `max_prompt_bytes` cuts the release into more parts and does not shrink what the merge has to hold.

**Resolution**: Routed
**Notes**: Landed in the discussion as a new subtopic, A Merge the Model Rejects for Size, with Floor Wording and Guidance's cause-phrase bullet amended. Specification §12.4 carries the Proposed Text; the added paragraph sits after the whole next-moves list, before the failed-merge example, so the list stays intact.

---

### 4. The unwind does not say it kept the answers commit

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §6.6 Recording the answers

**Problem**:
After an abort or a pre-push failure that follows the exclusion pauses, mint resets the branch to the starting commit and then re-commits the answers on top. That is the one commit mint leaves behind after an unwind, and nothing says the operator is told.

An unwind line that reports only what was undone leaves an unexplained `chore(release): release-notes path answers for v…` commit at HEAD, from a release the operator just watched abort. The likely reaction is to reset it away as a leftover. That deletes the answers, and the same questions come back on the next release, which is exactly what keeping the commit exists to prevent.

**Proposal**:
The unwind's narration says the answers commit was kept.

What leaned: the spec makes the kept commit the one deliberate exception to the surgical unwind, and the unwind line is the operator's account of what mint undid. An exception the line does not mention reads as an unwind that failed.

Alternatives:
- No mention. This side is dead: it invites the reset that brings the questions back.
- Narrate the answers commit when it is made. This fits as an addition, but the commit's survival only surprises at the abort.

**Proposed Text**:
In §6.6, at the end of the "Abort or failure after answering" bullet:

> The unwind's narration names the answers commit as kept, so the commit left at the top of the branch does not read as an unwind that failed.

**Resolution**: Routed
**Notes**: Landed in the discussion as a new subtopic, Narrating the Kept Answers Commit. Specification §6.6 carries the Proposed Text.

---

### 5. The operator's prompt and context are said to ride in every call, including the classifier

**Source**: Specification analysis
**Category**: Contradiction
**Move**: settled
**Priority**: Important
**Affects**: §2.3 What it drops, on purpose

**Problem**:
The lean-command section says the `[release]` and `[commit]` `context` and `prompt` keys ride in every call. Two other sections define calls that carry neither. The failure classifier's prompt (§9.2) holds the two codes, the failed call's captured output and an instruction to answer with exactly one code. The exclusion check (§6.2) carries only the changed-path list.

Read literally, the first section puts a `[release].prompt` override, which tells the AI to write release notes in a house format, into the classify call. Any answer other than an exact code maps to OTHER. So in a repo that customised its prompt, an overflow that halving would have absorbed stops at the floor instead. Operators would see big releases fail where chunking should have saved them.

**Proposal**:
"Every call" means every call that writes release notes or a commit message: single-pass, chunk, merge and the commit call. The classifier's and the exclusion check's own definitions already say so. Only one reading is live: the other would break the classifier's one-code answer.

**Current**:
Conventions an operator wants applied belong in the `[release]` and `[commit]` `context` and `prompt` keys, which ride in every call.

**Proposed Text**:
Conventions an operator wants applied belong in the `[release]` and `[commit]` `context` and `prompt` keys, which ride in every release-notes and commit-message call.

**Resolution**: Approved
**Notes**:

---

## Observations

- An empty or whitespace-only `ai_command` is not named among the malformed cases. Whether it reads as "names no program" or as unset is the builder's, to match the as-built accessor.
- Saving the chunk-notes file can itself fail (cache dir unwritable). Warning and carrying on as "no file", as the dry-run cache write does, is the obvious fit.
- A failed write or commit of the exclusion answers has no stated outcome.
- The classify call exceeds a positive cap set below roughly 9 KB, against "no call mint makes is ever bigger than the cap". That config is implausible.
- The new `[release]` and `[commit]` `diff_exclude` keys state no default. Empty is the only sensible one.
- The plain-mode `version_file` is not named among the paths the exclusion check leaves out of its list.
- Inside double quotes, the line-continuation bullet and "otherwise literal" disagree on a backslash-newline. POSIX removes it.
- Saved chunk-notes files are never pruned, so one per failed tag builds up in the cache dir.
- The editor failing on the floor's `e` has no stated outcome. The review gate's edit-failure handling is the obvious fit.
- The missing-notes marker lists every file with no notes, which can run to thousands of paths when a wave fails early.
- Whether a nudge on single-pass `r` that re-routes into chunking rides in the chunk calls or only the merge is unstated.
