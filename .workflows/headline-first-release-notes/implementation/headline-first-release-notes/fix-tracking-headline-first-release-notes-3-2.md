## Attempt 1

ISSUES:
- **Issue 1: the merged headlines were judged by the executor, not by E1's judges.** The task's Do and spec §10.2 (lines 273 and 308) require E1's judging. The executor's report says the judging "was mine, not E1's judge agents". The final-round bodies it judged are under `/Volumes/Scratch/tmp/claude-501/-Users-leeovery-Code-mint/4763aed1-90af-4f78-a0d3-ff4af7ffd2f0/scratchpad/c2/runs/w4a/*/body.md` and `/Volumes/Scratch/tmp/claude-501/-Users-leeovery-Code-mint/4763aed1-90af-4f78-a0d3-ff4af7ffd2f0/scratchpad/c2/runs/w4b/*/body.md`. The tuner judged its own tuning on exactly the call AC2 turns on. Task 3-9 re-runs these checks "with … the same judging", so the deviation carries forward unless it is fixed here. I spot-checked against E1's judgements and the audit: agentic-workflows v0.7.80, v0.8.12 and v0.5.12, and dex-engine v0.2.2. They look like `match` or `incomplete`, never `wrong-subject`. Tick v0.3.0 (w4a: "a consistent keyed layout, and `tick show --field`…") and switchboard v0.1.5 could go either way, which is the reason independent judges are required.
  FIX: The orchestrator, which can dispatch sub-agents, dispatches E1's six read-only judges with E1's grouping. That grouping is in `/Users/leeovery/Code/mint/.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/design.md:130`:
  - agentic-workflows (7)
  - dex-engine (3)
  - portal (3)
  - switchboard (3)
  - agntc and mint (5)
  - tick, stitch-cli and agentic-skills (5)

  Each judge receives:
  - `rubric.md`; the Headline criterion is at lines 28–34
  - repo paths and tag ranges from `sample.tsv`
  - the audit section for its group
  - the pre-registered headline expectation
  - the w4a and w4b merged bodies, with their part notes so the part-headline check is confirmed too

  The judges return headline verdicts as text, and the orchestrator records them. Any `wrong-subject` reopens tuning within this task. Clean verdicts clear AC2 by the method the spec names.
  ALTERNATIVE: Record the deviation in the task record and defer the judged verdict to 3-9. This is cheaper now, but 3-3 and 3-4 would tune on top of an unverified base, and any wrong-subject headline would surface only after more wording has changed. I recommend the fix.
  CONFIDENCE: medium

- **Issue 2: the kept cuts and the harness that replays them exist only in this session's scratchpad.** That is `/Volumes/Scratch/tmp/claude-501/-Users-leeovery-Code-mint/4763aed1-90af-4f78-a0d3-ff4af7ffd2f0/scratchpad/c2/`: `cuts/` (17 MB, 26 files); `main.go`, `export.go`, `overlay.json`, `run.py` and `check.py`; and `runs/` (147 MB). AC4 requires each release to be kept with its cut so 3-9 cuts it into the same parts. A scratchpad that may not outlive the session does not meet that, and 3-9 may run in another session. The executor was barred from writing to `.workflows/` and flagged this itself.
  FIX: The orchestrator persists the work, following E1's own split (E1 `design.md:125` keeps bulky inputs in the gitignored `.workflows/.cache/…` with their SHA-256s recorded in tracked metadata):
  1. Move `cuts/*.json` to `/Users/leeovery/Code/mint/.workflows/.cache/headline-first-release-notes/implementation/headline-first-release-notes/chunked-path/cuts/`.
  2. Add a tracked index beside the task record. Per release it lists: the cut file's SHA-256, `max_prompt_bytes`, the part count, `diff_sha256`, `change_map_sha256` and `cut_on_instructions_sha256`. Since the Phases 1–2 wording is in git history, the cut can be rebuilt if the cache is lost.
  3. Track the harness sources in a `.workflows/…/implementation/…` directory, as E1's `harness/main.go` already is under `.workflows`. Go's `./...` skips dot-directories, so the gates are unaffected. Rewrite `overlay.json`'s absolute scratchpad paths to the new location.
  4. Keep the w4a/w4b `part-*.md`, `body.md`, `merge.md` and `meta.json` as the evidence Issue 1's judges read. The prompts can go to the cache.
  CONFIDENCE: high

NOTES:
- The new summary rule ("…sums up the release, or this part, as a whole…") sits right before the rule asking a part with nothing to list for one line about this part. A spec review flagged this kind of tension at release level (`/Users/leeovery/Code/mint/.workflows/headline-first-release-notes/specification/headline-first-release-notes/review-input-tracking-c2.md:16`). It did no harm in 208 final-round parts: nothing-to-list lines came back as asked and no part returned an empty body. Task 3-3's checks should still watch for an empty-body failure.
- "Since the merge writes that" now also covers "this part, as a whole", which the merge does not write. The wording is imprecise, but no harm showed in the final-round runs.
- The merge bullet "judge its main body of work by the merged bullets, with the Change Map beside them" (`prompt.go:111-112`) follows §8. It sits one bullet below "the Change Map does not pick the headline". 3-9's wrong-subject check covers any drift toward ranking by the map.
- `meta.json` does not record the model each run resolved to; the executor checked the `sonnet` alias once (`modelUsage`). When the harness is persisted, capturing the resolved model per run would make 3-9's runs comparable without a separate probe.
- The harness uses unexported internals through the overlay (`cutRun`, `newCutter`, `splitFileDiffs`, `writeParts`, `chunkPart` fields). If a later change renames them, 3-9's replay fails at build time, so the break is visible.
- The new `DefaultPrompt` bullets also reach single-call prompts. Task 3-1's single-call checks ran on earlier wording, so 3-9's re-run on the frozen wording is what covers them, as the executor noted.
- Several final-round nothing-to-list lines add an explanation ("— its diff only changes tests"). That is 3-3's scope.
- Nothing arrived beyond the enumerated inputs.
