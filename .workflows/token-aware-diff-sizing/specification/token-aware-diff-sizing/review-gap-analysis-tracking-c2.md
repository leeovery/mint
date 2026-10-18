# Review Tracking: token-aware-diff-sizing - Gap Analysis

## Findings

### 1. Regenerate's failures are said to go to `on_notes_failure`

**Source**: Specification analysis
**Category**: Contradiction
**Move**: settled
**Priority**: Important
**Affects**: §7.2 The AI path, in order (step 4); §8.7 Calls that cannot be made; §9.1 The trigger (timeout and missing-binary bullets); §9.3 The codes (OTHER row); §9.7 An explicit `ai_command` that carries its own overhead; §11.1 One failed chunk fails the generation; §11.3 A failed merge. Colliding reading: §12.1 and §12.3, regenerate never reads `on_notes_failure`.

**Problem**:
Regenerate never reads `on_notes_failure`. At a terminal, a failed single-version regenerate offers the `c` / `e` / `a` choice. Unattended, it stops with guidance. `--all` skips the version. All of this holds whatever the key is set to, so a published version's notes are never replaced by a commit-subject list.

The passages that say where a failed notes call goes do not follow that rule. Each of these is said to fall to `on_notes_failure`: a failed single-pass call, a timeout, a missing binary, an OTHER answer, an explicit command whose overhead overflows the window, a call with no room for diff, a failed chunk and a failed merge. Every one of those paths also serves regenerate's fresh source. The OTHER row goes further and says `on_notes_failure` applies "exactly as today". But today's `abort` offers no choice, and today's `fallback` ships the fixed body this feature drops.

An implementer who builds the shared failure path from these passages routes regenerate through the key. Take a repo set to `fallback` where the login expires during `mint release regenerate v1.4.0`. The run ships the commit-subject list over v1.4.0's published notes and changelog entry, when it should have offered the choice. Someone notices when they open the old release page and find commit subjects where the notes were.

**Proposal**:
Each passage names the floor instead of the key. The floor is the specification's own name for where a failed notes path ends, and its rules split by verb: a release reads `on_notes_failure`, and a regenerate has its own floor. The decision that regenerate never reads the key fixes where these failures go, and the other sections about regenerate already follow it. "Exactly as today" is dropped, since the floor's choice and the commit-subject list both change what `on_notes_failure` does today.

What leaned: one term that already carries the per-verb split. The alternative that also fits is to name both verbs at each passage ("`on_notes_failure` on a release, regenerate's own floor on a regenerate"). That repeats, at eight sites, a split the floor's rules already make once.

**Current**:
§7.2, step 4:
> 4. **Under the cap: one single-pass call**, as built. If it fails, the failure classifier decides what happens next (§9). SIZE halves (§10); anything else falls to `on_notes_failure` (§12).

§8.7, "No room for diff" bullet, last sentence:
> mint then sends no call, and the run falls to `on_notes_failure` (§12) with guidance naming which of the two it was.

§9.1, timeout bullet, opening:
> - **A timeout** never starts the ladder. It falls to `on_notes_failure` as today, because a timeout is not evidence of size, and a wave fired at a slow service multiplies the wait.

§9.1, missing-binary bullet:
> - **A missing binary** falls to `on_notes_failure` as today.

§9.3, OTHER row:
> | OTHER | No split. `on_notes_failure` applies exactly as today (§12). |

§9.7, second sentence:
> Its classify call fails the same way, so the run takes the OTHER path with the AI's verbatim message, then `on_notes_failure`.

§11.1, second paragraph, last sentence:
> The run then goes to `on_notes_failure` like any other failure (§12).

§11.3, last sentence:
> mint then saves every chunk's notes (§11.4), names the file, and goes to `on_notes_failure` (§12).

**Proposed Text**:
§7.2, step 4:
> 4. **Under the cap: one single-pass call**, as built. If it fails, the failure classifier decides what happens next (§9). SIZE halves (§10); anything else falls to the floor (§12).

§8.7, "No room for diff" bullet, last sentence:
> mint then sends no call, and the run falls to the floor (§12) with guidance naming which of the two it was.

§9.1, timeout bullet, opening:
> - **A timeout** never starts the ladder. It falls to the floor (§12), because a timeout is not evidence of size, and a wave fired at a slow service multiplies the wait.

§9.1, missing-binary bullet:
> - **A missing binary** falls to the floor (§12).

§9.3, OTHER row:
> | OTHER | No split. The run falls to the floor (§12). |

§9.7, second sentence:
> Its classify call fails the same way, so the run takes the OTHER path with the AI's verbatim message, then the floor.

§11.1, second paragraph, last sentence:
> The run then goes to the floor like any other failure (§12).

§11.3, last sentence:
> mint then saves every chunk's notes (§11.4), names the file, and goes to the floor (§12).

**Resolution**: Approved
**Notes**:

---

### 2. Regenerate would offer the dry-run cache's notes, the ones it was asked to replace

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Move**: settled
**Priority**: Important
**Affects**: §7.2 The AI path, in order (closing sentence); context in §15.2 The dry-run cache

**Problem**:
A release's notes path runs in this order: the exclusion check, then cache reuse, then the size check and the call. Regenerate's fresh source is said to follow the same path minus only the exclusion check, so it gets the cache step too. The cache is keyed on the post-exclusion diff, the version and the resolved instructions, and an entry lives about an hour.

The most common regenerate comes right after a release, when the operator reads the published notes and wants better ones. If the release commit changed only excluded files, such as the changelog and `.mint.toml`, the regenerate has the same range, version and instructions as the release's dry run. Its key then matches the entry that dry run left. So `mint release regenerate v1.4.0` asks whether to use the cached notes, and those are the notes being replaced. Under `-y` it takes them, writes v1.4.0's identical notes back, and reports success. The operator sees no change on the release page and cannot tell why.

Writing the cache does harm too. The cache keeps one entry per project, so a regenerate's write evicts the preview a dry run left for a pending release. That release's real run then pays for its whole chunked wave again.

**Proposal**:
Regenerate's fresh source skips the cache step as well: it never offers cached notes and never writes them.

What determined it:
- The cache exists to carry a release's dry-run preview to the real run that follows, and regenerate has no dry run.
- A regenerate asks for new notes on a version that is already out, so a cached body can only hand back what it was asked to replace.
- A regenerate's write can only evict a pending release's preview.

Reading without writing keeps the first cost, and writing without reading keeps the second, so no other option survives.

**Current**:
Regenerate's fresh source follows the same path without step 1, per version under `--all`.

**Proposed Text**:
Regenerate's fresh source follows the same path without steps 1 and 2, per version under `--all`. It runs no exclusion check, and, as built, it neither offers notes from the dry-run cache nor writes to it: the cache carries a release's dry-run preview to the real run that follows, and a regenerate is asked for new notes on a version already out.

**Resolution**: Approved
**Notes**:

---

## Observations

- The floor's `e` on a saved chunk-notes file: an editor working on the file itself would empty it when the operator backs out by clearing the buffer. Editing a copy, as the existing editor seam's temp file does, keeps the file for the next `e` and for `--notes-file`.
- A merge-too-large ✗ line, read literally, ends `part notes too large to merge (merging)`, where the location suffix repeats the phrase.
- A no-room failure caused by operator text happens before any packing, so the `(part k of n)` suffix has no part to name.
- The follow-ups list gives the exclusion suggestions "under `-y`", while the unattended rule also covers non-TTY runs. A non-TTY run without `-y` stops at its first gate anyway.
- The after-halving hint names only the cap, though a command's own overhead can also cause halving. The lean-command pointer would fit beside it when `ai_command` is explicit.
- A non-zero exit with nothing on either stream renders `AI returned empty notes`, which reads oddly for a tool that crashed silently.
- The saved file's `{tag}.md` name needs the tag made path-safe when the tag prefix carries a slash.
