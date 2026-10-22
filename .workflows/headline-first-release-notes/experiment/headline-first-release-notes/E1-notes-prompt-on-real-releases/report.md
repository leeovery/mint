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

**Round v2** (2026-10-22 07:49; `new-v2` only, one repeat; 5 calls, 0 failures; run log `run-tune-v2.log`).

| Release | new-v2 |
|---|---|
| mint v0.0.6 | headline right; one Added bullet for `mint setup` |
| dex-engine v0.2.9 | headline back on the largest fix (unexpected); the test-only parked-reason claim still there; the fallback both under Changed and again as a Fixed bullet; a library name ("trafilatura") in a bullet |
| portal v0.12.0 | headline and 💥 Breaking right; the comment-only claim gone again |
| tick v0.3.2 | headline adds `tick doctor` as a second subject again; the 64 KiB fix leads Fixed; parse errors moved to Fixed |
| switchboard v0.1.2 | headline right; the retention default back under 💥 Breaking |

Across v0–v2 the same three failures flip on and off with no wording change aimed at them: the grab-bag headline (v0 yes, v1 no, v2 yes), the second subject in tick's headline (yes, no, yes), retention as breaking (yes, no, yes). The truth failures flip too (dex test-only claim: no, yes, yes; portal comment-only claim: no, yes, no). One body per version cannot separate wording from sampling. Revisions taken into v3, aimed at making each call less of a coin flip: a countable test for a main body of work (the change most of the notes' bullets serve, or one that outweighs all the others put together; the largest of several separate changes is not one); "and" in a headline joins only two co-equal bodies, never two pieces of one; a doubt rule for 💥 Breaking (when unsure, it is not breaking); a test asserting a behaviour is evidence that the behaviour already existed, not a change. v3 runs two repeats on the tuning set so the choice of the version to freeze rests on more than one body per release.

**Round v3** (2026-10-22 07:50; `new-v3`, two repeats; 10 calls, 0 failures; run log `run-tune-v3.log`).

| Release | new-v3 r1 | new-v3 r2 |
|---|---|---|
| mint v0.0.6 | headline right; no internals | same |
| dex-engine v0.2.9 | no headline (match); fallback under Fixed; test-only parked-reason claim present; 🐛 Fixed emitted before 🔧 Changed | no headline (match); fallback under Fixed; test-only claim absent; 🐛 Fixed before 🔧 Changed |
| portal v0.12.0 | headline and 💥 Breaking right; comment-only claim absent | headline and 💥 Breaking right; comment-only claim present |
| tick v0.3.2 | headline names the long-line fix plus `tick doctor` (a piece of the same fix); 64 KiB fix leads Fixed | same headline shape; section headers written as `## ✨ Added` (markdown heading marks) |
| switchboard v0.1.2 | headline right; retention not flagged breaking; DST fix under Changed | same |

Against v0–v2, v3 holds the two costliest calls on both repeats: no headline on the grab-bag, no false 💥 Breaking on the retention default. The truth failures still split one-and-one. New in v3: sections out of the fixed order on dex-engine (both repeats) and markdown heading marks once on tick — format slips the rubric counts.

**Frozen for the held-out run: `new-v3`** — the last version run, and the one that read best on the calls that cost most (wrong or unwarranted headlines, false breaking flags). `prompts/new-v3.md` SHA-256 `c1f149842689ba5f6b7338579514258a0fa23416c04421253eacfaae62630a0f`; `prompts/new-v3-reminder.txt` SHA-256 `ee31f054f8f0612db4098702406d72bac19a7e8d5295e5b840b2b4e08fe0d2ce` (unchanged from v0). Frozen 2026-10-22 07:52, before any held-out call. `claude --version` at the end of tuning → `2.1.307 (Claude Code)`.

### Held-out generation

Run 2026-10-22 07:51–07:56 (run log `.workflows/.cache/…/run-holdout.log`): 52 units (26 releases × 2 arms), two repeats each, four at once — 104 calls, 0 failures, 0 retries. Bodies in `data/bodies/holdout/current/` and `data/bodies/holdout/new-v3/`, 52 each, with per-call metadata. Longest call 38 s. The two releases over the 1.5 MB cap ran as single calls in both arms without refusal: agentic-workflows v0.6.0 (composed prompt 1,932,945 bytes current / 1,939,515 new) and switchboard v0.1.0 (1,755,023 / 1,761,593). `claude --version` at the end of the phase → `2.1.307 (Claude Code)`.

### Judging

Blinded copies in `.workflows/.cache/…/blind/<release>/A–D.md`; the blind key and Lee's key stay in `.workflows/.cache/…/` (`blind-key.json`, `lee-key.json`) until all judgements and Lee's picks are in. Each judge's hand-back is pulled from its transcript by `.workflows/.cache/…/extract_judgement.py`, parsed as JSON and written to `data/judgements/<group>.json` (content unchanged, re-indented). Landed so far: dex-engine (12 objects), portal (12), tick/stitch-cli/agentic-skills (20), switchboard (12), agntc/mint (20). Lee's picks received (`data/lee-picks.json`: 1 B, 2 B, 3 A, 4 A, 5 A, 6 B), still blind.

## Deviations

- 2026-10-22 07:46 — Claude Code updated itself from 2.1.306 (recorded in the design) to 2.1.307 before the run started. Every call in this run uses 2.1.307, both arms alike; the model behind `--model sonnet` is unchanged (`claude-sonnet-5-5`).
- 2026-10-22 07:58 — The six judges, dispatched together in one message as designed, ran as background agents (this session's agent tool launches them asynchronously; it offers no foreground switch). The session waits for all six before scoring, so the judging itself is unchanged.
- 2026-10-22 07:57 — Lee's side-by-side read (`data/lee-read.md`) went out when the held-out run finished, as designed, in parallel with the judges.
