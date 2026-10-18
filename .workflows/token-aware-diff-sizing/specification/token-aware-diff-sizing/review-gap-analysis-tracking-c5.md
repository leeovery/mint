# Review Tracking: token-aware-diff-sizing - Gap Analysis

## Findings

### 1. A dry run under `fallback` points its saved notes at regenerate, for a version that is not out

**Source**: Specification analysis
**Category**: Contradiction
**Move**: settled
**Priority**: Important
**Affects**: §12.1 What applies, by mode (the "A dry run at the floor" paragraph). Colliding reading: §12.1's `on_notes_failure = fallback` row, whose warning names `mint release regenerate <version> --notes-file <path>`, against §12.4's Always move, which names `mint release --notes-file <path>` while the version is not out.

**Problem**:
A dry run of a big release under `on_notes_failure = fallback` can end at the floor. A chunk fails mid-wave, mint saves the finished chunks' notes to the chunk-notes file, and the dry run previews the commit-subject list with the fallback warning. That warning is defined for a release that shipped, so it names the saved file with `mint release regenerate <version> --notes-file <path>`. A dry run tags nothing, so the version is not out. The rule for the `--notes-file` pointer gives `mint release --notes-file <path>` while the version is not out, and the dry-run rule itself says the real run can release the saved file with `--notes-file` instead of repeating the wave.

An implementer who renders the fallback warning as defined prints the regenerate command on the dry run. An operator who runs it gets an error, because the version has no tag yet. An operator who runs the real release first, so that there is a version to regenerate, waits through the whole wave again, ships the commit-subject list, and only then puts the saved notes on. Saving the file on a dry run exists to spare that second wave.

**Proposal**:
On a dry run, the fallback warning names the saved file with `mint release --notes-file <path>`. Two rules decide it:
- The `--notes-file` pointer names the release command while the version is not out, and a dry run leaves it not out.
- The dry-run rule says the real run releases the saved file with `--notes-file` instead of repeating the wave.

The regenerate command on a dry run is dead, since it names a version that has no tag.

**Current**:
Under `fallback` it previews the commit-subject list as the body that would ship, with the fallback warning.

**Proposed Text**:
Under `fallback` it previews the commit-subject list as the body that would ship, with the fallback warning, which names a saved file with `mint release --notes-file <path>` (§12.4), since the version is not out.

**Resolution**: Pending
**Notes**:

---

### 2. A failed `r` on a release names the saved notes but not how to release them

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Move**: settled
**Priority**: Important
**Affects**: §15.1 `r` on chunked notes (the "When `r` fails" bullets); context in §12.4's Always move and §15.1's `regenerate --all` paragraph

**Problem**:
At an attended release's review gate, the operator presses `r` on chunked notes to change their emphasis. If that re-merge fails, mint saves every chunk's notes, names the file and aborts. Nothing says the abort tells the operator what to do with the file. A failed `r` is not the floor, and it runs at a terminal, so it gets neither the next moves an unattended floor prints nor the floor's `e`, which is how a saved file gets used at a terminal. The batch regenerate's failed `r` names its file together with the command that releases it. The release's names only the file.

An implementer who prints only the path leaves the operator holding a file with no route. The likely next step is to re-run `mint release`. That runs the exclusion check and the whole chunked wave again, minutes of calls and a draw on the usage window, to rebuild notes already saved on disk. The operator notices while waiting through the second wave.

**Proposal**:
The failed `r`'s abort names the file with `mint release --notes-file <path>`. What determined it:
- The abort lands before any tag, so the version is not out, and the `--notes-file` pointer names the release command while the version is not out.
- `regenerate --all`'s failed `r` already names its file with the command that releases it.

The other reading, the path alone, is dead: an attended operator, who gets no next moves, is left with no route to the file.

**Current**:
- mint first saves the per-chunk notes and names the file (§11.4).

**Proposed Text**:
- mint first saves the per-chunk notes and names the file with the command that releases it, `mint release --notes-file <path>` (§11.4, §13).

**Resolution**: Pending
**Notes**:

---

## Observations

- The three cancel passages (the classifier trigger, the classify-call edge, Ctrl-C mid-wave) still say a cancel never falls to `on_notes_failure`. On regenerate, which never reads that key, "propagates unchanged" is what keeps a Ctrl-C out of the floor's choice and out of `--all`'s skip-and-continue.
- A forward release that ships under `fallback` and then fails before the push is unwound, so the fallback warning it already printed names a regenerate command for a version no longer out.
- The OTHER next move ("the AI's own message above names the cause") points at nothing when the cause is OTHER with no output.
- Whether a halved round's calls share the four-call bound and run in parallel, or go one at a time, is unstated. "Serial rounds" fixes only that each round waits on the failure before it.
- A cap set so low that the default instructions alone fill it reads as no room caused by operator text, whose move names `prompt` and `context` keys the operator never set. The config is implausible.
- Whether a dry run's `fallback` preview of the commit-subject list is written to the dry-run cache is unstated. If cached, a real run whose AI now works would be offered it, or take it under `-y`.
