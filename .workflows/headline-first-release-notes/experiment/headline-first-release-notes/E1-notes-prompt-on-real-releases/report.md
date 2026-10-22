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

All six judgements landed (agentic-workflows: 28 objects): 104 objects, four labels per held-out release, every rubric field present. Keys moved into `data/blind-key.json` and `data/lee-key.json` after the last judgement and Lee's picks were in (2026-10-22).

### Decision-rule scores (held-out, 52 bodies per arm)

`python3 -I .workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/harness/score.py` → `data/scores.json`:

| Class | Current | New | Threshold for new | Pass |
|---|---|---|---|---|
| Lead and headline | clean lead 21/52 | clean lead 38/52; headline `match` 38/52; `wrong-subject` 0 | clean lead ≥ 45; `match` ≥ 42; `wrong-subject` ≤ 1 | no |
| Order within sections | 30 inversions | 16 | ≤ 15 | no |
| Section placement | 113 (misfiled 84 + duplicates 29) | 97 (78 + 19) | ≤ 45.2 | no |
| User-facing only | 77 internal items | 28 | ≤ 23.1 | no |
| True to the release | 47 (false claims 25 + in-release-as-change 22) | 47 (34 + 13) | ≤ 18.8 | no |
| Breaking changes | flagged 1/10 | flagged 8/10 | ≥ 8 | **yes** |

| Guard rail | Current | New | Holds |
|---|---|---|---|
| Missing items ≤ current × 1.10, rounded up (10) | 9 | 13 | **no** |
| Format slips ≤ current | 5 | 5 | yes |
| Judges prefer new in ≥ 19 of 26 releases | — | 21 of 26 | yes |

Residual sorting for the failed classes (new-arm instances the judges called input-absent / all): lead and headline 0/28; order 0/16; placement 0/97; user-facing 0/28; true to the release 3/47 (the three absent calls: switchboard v0.1.5's config keys that shipped in v0.1.4). Every failed class therefore sorts to "needs more prompt work".

Verdict line printed by `score.py`: "Short of the bar: rewritten prompt passes breaking changes; lead and headline, order within sections, section placement, user-facing only, true to the release need more prompt work; guard rail broken (missing items) — not taken as tested".

### Secondary measures (not in the rule)

`python3 -I .workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/harness/secondary.py` → `data/secondary.json`:

- Lead: current clean 21 / partial 30 / no 1; new clean 38 / partial 12 / no 2.
- New-arm headline verdicts: match 38, incomplete 6, omitted 8, unexpected 0, wrong-subject 0. The 6 `incomplete` are three releases × both repeats — agentic-workflows v0.5.12, v0.8.3 and dex-engine v0.2.0, each naming one piece or one of two bodies. The 8 `omitted` are four releases × both repeats — dex-engine v0.2.2, mint v0.0.2, stitch-cli v1.6.6, tick v0.3.0. Headline shape slips in new bodies: 7 (judges' `shape_slips`, mostly `inventory`).
- Headline presence agreed between the new arm's two repeats on all 26 releases.
- False claims alone: current 25 (in 20 bodies), new 34 (in 23 bodies). In-release-as-change: current 22, new 13. Internal: 77 → 28. Inversions: 30 → 16. Duplicates: 29 → 19. Misfiled: 84 → 78.
- Missing items: current 9, new 13. The extra four sit on agentic-workflows v0.7.80 (+2), v0.7.0 (+1) and dex-engine v0.2.2 (+1).
- Mean overall score: current 2.85, new 3.62. Releases where the judges' mean favours new: 21 of 26; not: agentic-workflows v0.6.0, agntc v0.6.0, dex-engine v0.2.2, mint v0.0.2, switchboard v0.1.5.
- Breaking (on the 5 breaking releases): current flagged 1 / unflagged 9; new flagged 8 / unflagged 2 (tick v0.3.0 ×2 — new arm wrote no 💥 Breaking section there).

**Lee's blind read** (`data/lee-picks.json` against `data/lee-key.json`): new preferred on 4 of 6 — tick v0.3.0, switchboard v0.2.6, agentic-workflows v0.8.0, agntc v0.5.2; current preferred on portal v0.11.1 and mint v0.0.2. Agreement with the judges' release-level preference: 5 of 6 (they differ on portal v0.11.1, where the judges scored new 4/4 against current 2/2).

## Deviations

- 2026-10-22 07:46 — Claude Code updated itself from 2.1.306 (recorded in the design) to 2.1.307 before the run started. Every call in this run uses 2.1.307, both arms alike; the model behind `--model sonnet` is unchanged (`claude-sonnet-5-5`).
- 2026-10-22 07:58 — The six judges, dispatched together in one message as designed, ran as background agents (this session's agent tool launches them asynchronously; it offers no foreground switch). The session waits for all six before scoring, so the judging itself is unchanged.
- 2026-10-22 07:57 — Lee's side-by-side read (`data/lee-read.md`) went out when the held-out run finished, as designed, in parallel with the judges.
