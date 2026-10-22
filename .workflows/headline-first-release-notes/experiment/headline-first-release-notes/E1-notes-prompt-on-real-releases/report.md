# E1: Notes Prompt On Real Releases — Report

## Results

### Run environment (2026-10-22)

- Harness rebuilt at mint `aa559a96`; mint's code is unchanged from the design's `ec5a0479` (`git diff --stat ec5a0479..HEAD -- internal cmd go.mod go.sum` → empty).
- `claude --version` → `2.1.307 (Claude Code)`; the `sonnet` alias still resolves to `claude-sonnet-5-5` (`modelUsage` key of a one-line `--output-format json` call).

### Tuning (not in the decision rule)

Bodies under `data/bodies/tune/<arm>/`; call metadata beside each body.

**Round v0** (2026-10-22 07:46; `prompts/new-v0.md` SHA-256 `487ec8b6…fa570`, reminder `ee31f054…2ce`). Current arm and `new-v0`, one repeat each on the five tuning releases; 10 calls, 0 failures, 5–8 s each (run log: `.workflows/.cache/…/run-tune-v0.log`).

Close read against the audit's account:

| Release | Current | new-v0 |
|---|---|---|
| mint v0.0.6 | no headline; README restructure as a bullet; setup guide's reference table as its own bullet | headline names `mint setup` for what it lets the user do (expected → match); README bullet gone |
| dex-engine v0.2.9 (grab-bag) | test-only "park note states the cause" claimed as a fix; article fallback under Added | headline written on the largest of the separate fixes (not expected → unexpected); test-only claim gone; article fallback (a fix) under Changed |
| portal v0.12.0 | comment-only "hook locations listed correctly" claimed as a fix; `x /tmp` change unflagged in Changed | headline right; `x /tmp` under 💥 Breaking; comment-only claim gone; malformed-`hooks.json` fix leads Fixed |
| tick v0.3.2 | the guard (description cap) opens the notes; the 64 KiB fix under Changed; `.worktreeinclude` (dev-only) under Added | headline names the long-line fix but adds `tick doctor` as a second subject and words the fix in file terms (a lesser piece promoted to co-equal); the fix under Fixed; `.worktreeinclude` gone; "doctor reads the file once" (internal) still present |
| switchboard v0.1.2 | two in-release review fixes listed as user fixes; DST day-boundary fix under Changed | headline right; in-release fixes gone; retention default raised to 400 days filed under 💥 Breaking (keeps more — not breaking); DST fix still under Changed |

Revisions taken into v1: a headline only for a body of work that clearly outweighs the rest put together, and a second subject only when it is of equal weight; the headline in the user's terms, not files or formats; 💥 Breaking narrowed to existing use that stops working or gives a different result to act on (a default that only keeps more or adds is not breaking); Fixed defined by behaviour that was wrong in the previous release, however the diff frames it; internal efficiency a user would not notice added to what is left out.

**Round v1** (2026-10-22 07:48; `new-v1` only, one repeat; 5 calls, 0 failures; run log `run-tune-v1.log`).

| Release | new-v1 |
|---|---|
| mint v0.0.6 | as v0: headline right, no internals |
| dex-engine v0.2.9 | no headline (match); article fallback now under Fixed; the test-only "parked reason reports the cause" claim is back (v0 had dropped it) |
| portal v0.12.0 | headline and 💥 Breaking right; the comment-only "`hook list` shows a single location" claim is back (v0 had dropped it); grouped-view groundwork now presented as a Fixed bullet |
| tick v0.3.2 | headline names the long-line fix alone, still in store terms ("very long lines in the task store"); description cap and migrate's title rule under 💥 Breaking (arguable); four `tick doctor` bullets lead Changed above parse errors every command shows |
| switchboard v0.1.2 | headline right; retention change back under Changed; DST fix now under Fixed; token redaction under Security |

The false claims that came back between v0 and v1 on the same inputs point to sampling variance on the truth rule rather than a wording effect. Revisions taken into v2: a per-bullet check — find the code change behind each bullet and drop it when every changed line behind it is a comment, documentation or a test; ordering weight to include how often users meet a change (everyday commands above diagnostic or rarely used ones).

## Deviations

- 2026-10-22 07:46 — Claude Code updated itself from 2.1.306 (recorded in the design) to 2.1.307 before the run started. Every call in this run uses 2.1.307, both arms alike; the model behind `--model sonnet` is unchanged (`claude-sonnet-5-5`).
