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

## Reading

**Execution first.** The run is clean on everything the rule reads: both arms on byte-identical inputs, the same CLI build (2.1.307) and model on every call, 104 of 104 held-out calls returned with no retry, the two over-cap releases answered as single calls in both arms, and all 104 judgements parsed with every field. The one instrument drift (CLI 2.1.306 → 2.1.307) landed before the first call and hit both arms alike. What the results indict is the prediction, not the harness — with the judge caveat at the end of this section.

**Against the prediction, class by class.**

- *Breaking changes* — predicted 3–4 of 5 releases flagged; 4 of 5 were, on both repeats (8 of 10 bodies, against 1 of 10 for the current prompt). Portal v0.11.1, predicted hardest, is flagged on both repeats with the remedy named (re-register or remove the old keys). The miss is switchboard v0.1.5: both new bodies state the change — "print JSON whenever output isn't a terminal, so scripts and agents need no `--json` flag" — under 🔧 Changed, worded as a convenience. The v3 rules that a change which "allows more, or improves" is not breaking, and "when unsure … it is not", fit that reading of it. The 💥 section is also a new place to be wrong: nine Breaking→Changed misfilings (dex-engine v0.2.0's auto-migrated changes, mint v0.0.2's cache move, mint v0.0.3's default model), and false breaks that assert a previous state the diff contradicts (portal v0.8.0 "kill moved from `K` to `k`", when `k` already killed; agentic-workflows v0.6.0 "refuses any `--key` flag", when no such flag existed). mint v0.0.2's new notes open on a 💥 Breaking bullet the judges filed as Changed — a false alarm in the notes' first line.
- *Lead and headline* — predicted ≥ 85% clean; 73% (38 of 52), up from 40% (21 of 52). The predicted bias held exactly: no wrong-subject and no unexpected headline in 52 bodies; every miss is an omission (8 bodies: dex-engine v0.2.2, mint v0.0.2, stitch-cli v1.6.6, tick v0.3.0) or a headline naming one piece or one of two bodies (6 bodies: agentic-workflows v0.5.12, v0.8.3, dex-engine v0.2.0). Headline presence agreed between repeats on all 26 releases, where the tuning rounds flipped the same decisions on and off; the v3 main-body test made the call repeatable. The shortfall is recall. Two of the four omitted releases are pre-registered as two bodies or two fixes serving one goal (mint v0.0.2, stitch-cli v1.6.6), and dex-engine v0.2.0's incomplete headlines name one of its two bodies: the v3 rules that stopped the grab-bag headline in tuning ("the largest of several separate changes is NOT a main body"; "and" only for co-equal bodies) read here as also pushing the two-body release to no headline, or to half of one.
- *User-facing only* — predicted a cut of 70% or more; 64% (77 → 28 internal items). Close to the bar, short of it.
- *Order within sections* — predicted modest; inversions fell 47% (30 → 16), one short of the ≤ 15 bar. Better than predicted.
- *True to the release* — predicted much reduced; level (47 → 47). In-release-as-change fell 22 → 13; false claims rose 25 → 34. The rise is concentrated: portal v0.8.0 (1 → 4), agntc v0.6.0 (2 → 4), agentic-workflows v0.6.0 (3 → 5), switchboard v0.1.0 (0 → 2) (from the judgements against `data/blind-key.json`; **exploratory**). Nearly every false claim in either arm is one of two kinds: behaviour presented as new or changed that the previous release already had (agntc's `NO_COLOR` handling and `init` tree preview, switchboard's per-account reserve, portal's `k`/`r` bindings, agentic-workflows' Triage rerouting), or a claim drawn from a README edit with no code behind it (portal v0.8.0's case-sensitive tags, switchboard v0.1.5's config keys). The v2 per-bullet code check and the v3 line on tests did not stop either kind, and several of the new arm's false claims sit in bullets whose wording states a before ("moved from", "now", "no longer") — the claim shape the prompt's "check the before" rule names.
- *Section placement* — predicted partly fixed; misfiled barely moved (84 → 78), duplicates fell (29 → 19). The dominant misfiling is the same in both arms: a rework of existing behaviour filed under ✨ Added (36 current, 34 new). Changed→Fixed fell 23 → 14: the behaviour-based definition of Fixed took two-fifths of that error out.
- *Missing items* — the guard broke, 9 → 13. Two of the four extra are agentic-workflows v0.7.80's faster filtered store reads, missing from both new repeats; the v1 rule leaving out "internal efficiency changes a user would not notice" fits that omission, a change the judges counted as one users meet. The other two are single (agentic-workflows v0.7.0's docs page, dex-engine v0.2.2's routine hand-off).

**Input present or absent.** The prediction sorted true-to-the-release and placement residuals as mostly input-absent. The judges call 3 of 47 true-to-the-release residuals absent (switchboard v0.1.5's config keys: README rows for code that shipped in v0.1.4 and is not in the diff — the one case of the kind the prediction named) and 0 of 97 placement residuals. The session spot-checked three of the judges' "previous release already had it" calls against the input files (**exploratory**, not in the rule), and each fact sits in the diff's removed lines: portal v0.8.0's input removes `key.NewBinding(key.WithKeys("k"), key.WithHelp("k", "kill"))` (line 7420); agntc v0.6.0's removes a comment saying picocolors strips colour for `NO_COLOR`; switchboard v0.1.0's removes a README line giving every account a `reserve` setting. On this evidence the prediction's sorting is wrong: the previous release's state was in front of the AI as removed and context lines, and the prompt did not get it used. Caveat: the judges' other input-present calls are unverified, and "present" means a careful reader of a diff of up to 1.9 MB could have found it — whether one call can be made to read that carefully is what a prompt-work successor would find out.

**Readers.** The judges prefer the new notes on 21 of 26 releases (mean overall 2.85 → 3.62); Lee on 4 of his 6, agreeing with the judges on 5. The disagreement is portal v0.11.1, where Lee picked the current notes against the judges' 4/4 for new and 2/2 for current. Read side by side: the new r1 headline is right and its 💥 section states the old-hook break with the remedy, but its first section opens on two Breaking bullets in mechanism terms ("keyed by a per-pane token instead of `<session>:<window>.<pane>`"), a fix sits under Added, and the release's main fix is fourth in 🐛 Fixed below its own pieces; the current r1's first bullet states the release in one plain line ("Resume hooks now follow the pane itself — … hooks survive pane moves and session renames"), misfiled under Added and silent on the break. On mint v0.0.2, where Lee and the judges both picked current, the new notes have no headline and lead with the false 💥 cache-move bullet. Both of the releases where Lee preferred current carry the new arm's failures in the notes' opening lines; the rule's counts weigh an error at the top the same as one at the bottom.

## Conclusion

The pre-registered rule, executed by `harness/score.py` on the 26 held-out releases (`data/scores.json`):

- **Breaking changes passes** — 8 of 10 new bodies flagged against a bar of 8.
- **Lead and headline, order within sections, section placement, user-facing only and true to the release fail.** Their residual instances sort by the judges' input-present call: input-absent 0 of 28, 0 of 16, 0 of 97, 0 of 28 and 3 of 47 — each below 50%, so each class **needs more prompt work**: a successor experiment on that class. No class sorts to "needs mint to hand the AI more".
- **A guard rail breaks** — missing items 13 in the new bodies against a ceiling of 10 (current 9 × 1.10, rounded up). The rule's branch: the frozen prompt is not taken as tested, whatever the classes show — the Breaking class's pass included. The regression it names: the new prompt drops more user-facing changes than the current one. The other two guards hold (format slips 5 against 5; judges prefer new on 21 of 26 releases against a bar of 19).
- **Meets the bar: no.**

Verdict: "Short of the bar: rewritten prompt passes breaking changes; lead and headline, order within sections, section placement, user-facing only, true to the release need more prompt work; guard rail broken (missing items) — not taken as tested".

## Reproduce

From the mint repo root, with `D` = `.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases` and `C` = `.workflows/.cache/headline-first-release-notes/experiment/headline-first-release-notes`:

1. Build the harness: `bash D/harness/build.sh` → `C/e1harness` (an overlay build of `D/harness/main.go` into the mint module; prints the SHA it built against).
2. Input sizes: `python3 -I D/harness/sizes.py D/sample.tsv`.
3. Tuning: `python3 -I D/harness/run.py --set tune --arms current,new --new-version v0`; then `--arms new --new-version v1`, `--arms new --new-version v2`, and `--arms new --new-version v3 --repeats 2`. Bodies to `D/data/bodies/tune/`.
4. Held-out: `python3 -I D/harness/run.py --set holdout --arms current,new --new-version v3 --repeats 2` (resumable). Bodies and call metadata to `D/data/bodies/holdout/`; composed prompts and per-release inputs (Change Map + diff) to `C/prompts/` and `C/inputs/`.
5. Blind: `python3 -I D/harness/blind.py --new-version v3` → `C/blind/<release>/A–D.md`, `C/blind-key.json`, `C/lee-key.json`, and `D/data/lee-read.md`.
6. Judge: six general-purpose agents, one per group in the design's Judging section, each given `D/rubric.md`, its releases' repo paths and tag ranges, its group's section of `.workflows/headline-first-release-notes/imports/release-notes-audit.md`, `C/inputs/<release>.txt`, the expectations in `D/sample.tsv`, and `C/blind/<release>/`. Each hand-back to file: `python3 -I C/extract_judgement.py <agent transcript .jsonl> D/data/judgements/<group>.json`.
7. Unblind once every judgement and Lee's picks are in: copy `C/blind-key.json` and `C/lee-key.json` to `D/data/`.
8. Score: `python3 -I D/harness/score.py` → `D/data/scores.json` and the verdict line; `python3 -I D/harness/secondary.py` → `D/data/secondary.json`.

The model calls are stochastic: a re-run repeats the procedure, not the bodies.

## Corrections

### 2026-10-22

- Results → Secondary measures, the Breaking line: the two new-arm bodies not flagged are switchboard v0.1.5, both repeats (output piped off a terminal becomes JSON), not tick v0.3.0. tick v0.3.0's two new bodies each carry a 💥 Breaking section and are flagged. The judges' one `flagged` call on the current arm is switchboard v0.1.0 r2. The counts (current 1 of 10, new 8 of 10) and the decision-rule table are unaffected.
