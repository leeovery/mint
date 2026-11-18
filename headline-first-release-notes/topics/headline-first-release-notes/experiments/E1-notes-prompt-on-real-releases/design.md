# E1: Notes Prompt On Real Releases

## Question

On the audited releases, with the AI's inputs unchanged — the same post-exclusion diff, the same Change Map, the same model and command — and only the notes instructions and the closing reminder rewritten, which of the audited failure classes does the rewritten prompt fix; and for each class it does not fix, did the AI have the facts it needed?

The classes are the feature's Scope decision: lead and headline; order within sections; section placement; user-facing only; true to the release; breaking changes.

Feeds: the headline-first-release-notes discussion's open subtopics (Ordering Within Sections with Change Map Signals, Section Placement, User Facing Only, True To The Release, Breaking Changes, and the reminder rewording), and the specification's notes prompt — a class that passes hands the specification its rule as tested; a class that fails for want of input is evidence that mint must hand the AI something new.

## Prediction

Against the current prompt on the same inputs:

- **Lead and headline** — largely fixed. The headline states the whole main work above the sections; expected clean lead in ≥ 85% of new-prompt notes against roughly 55% for the current prompt (the audit: 17 of 30 shipped notes led cleanly). Headline omissions are likelier than wrong subjects, since the prompt biases to leaving it out.
- **User-facing only** — largely fixed: internal packages, CI, release tooling and test harnesses are recognisable from paths; expect a cut of 70% or more.
- **True to the release** — much reduced where the diff shows the cause (a README-only or comment-only edit, a test-only change, behaviour visible in context lines). Residuals concentrate where the fact lies outside the diff: what an earlier release shipped or lost (agntc v0.5.2's restored folders), whether something already existed before the previous tag.
- **Section placement** — partly fixed: rework-versus-new and in-release-versus-previous calls improve with the explicit rule, but hinge on knowing the previous release's state, which the diff only partly shows.
- **Breaking changes** — flagged in 3–4 of the 5 breaking releases; the hardest is portal v0.11.1, whose break (old-format hooks stop firing) needs the reader to infer that no migration ships.
- **Order within sections** — modest improvement; the prompt can de-weight file size and novelty but cannot make weight visible where the diff does not show it.

Predicted residual sorting: true-to-the-release and placement residuals mostly input-absent; ordering residuals mostly input-present.

Secondary (reported, not in the rule): headline shape slips; repeat-to-repeat agreement; Lee's blind preferences on six releases and their agreement with the judges; the tuning set's results.

## Decision rule

Computed on the 26 held-out releases, over every judged body (two repeats per arm, so 52 per arm when every call succeeds), new prompt against current prompt. `harness/score.py` executes it from the judgements; no step takes judgement.

| Class | The new prompt passes when |
|---|---|
| Lead and headline | lead `clean` in ≥ 85% of new bodies; headline verdict `match` in ≥ 80% of new bodies; `wrong-subject` headlines ≤ 1 across all new bodies |
| Order within sections | new inversions ≤ 50% of current's |
| Section placement | new misfiled + duplicates ≤ 40% of current's |
| User-facing only | new internal items ≤ 30% of current's |
| True to the release | new false claims + in-release-as-change ≤ 40% of current's |
| Breaking changes | `flagged` in ≥ 80% of new bodies on the 5 breaking releases (8 of 10) |

Where the current count is 0, the class passes only if the new count is 0. Percentages of body counts round up (85% of 52 → 45).

Guard rails, all three must hold:

- missing items in new bodies ≤ current's × 1.10, rounded up;
- format slips in new bodies ≤ current's;
- the judges prefer the new prompt — mean overall score of a release's new bodies above its current bodies' — in ≥ 70% of releases (19 of 26).

Outcomes:

- **A class passes** → the specification takes that class's rule, as worded in the frozen prompt, as tested.
- **A class fails** → its residual instances in the new bodies (each recorded failure; for lead and headline, each non-`clean` lead and non-`match` headline; for breaking, each body not `flagged`) are sorted by the judges' input-present call. If ≥ 50% are input-absent, the class **needs mint to hand the AI more** — evidence returned to the discussion. Otherwise it **needs more prompt work** — a successor experiment on that class.
- **A guard rail breaks** → the frozen prompt is not taken as tested, whatever the classes show; the report names the regression.
- **Meets the bar** = every class passes and every guard holds.

The verdict is the one line `score.py` prints, of the form "Meets the bar / Short of the bar: rewritten prompt passes …; … need mint to hand the AI more; … need more prompt work[; guard rail broken (…) — not taken as tested]".

## Setup

**Execution shape.** Deterministic harness code drives every AI call; four calls run at once; the session observes and logs. Judging is six ad-hoc foreground sub-agents dispatched in one message. The session close-reads all output after the mechanical scoring.

**Arms.**

- *Current* — mint's shipped `notes.DefaultPrompt` and `notes.OutputReminder`, composed by `notes.ComposePrompt`: byte-for-byte what mint sends today.
- *New* — the candidate instructions `prompts/new-<version>.md` and reminder `prompts/new-<version>-reminder.txt`, joined with the same Change Map and diff in the same order (instructions, Change Map, diff, reminder). The starting candidate is `prompts/new-v0.md` / `new-v0-reminder.txt`: the decided headline rules (Headline Shape, No Clear Headline, Picking The Headline) plus one rule set per failure class — sections judged against the previous release; a 💥 Breaking section first, for changes users must act on; claims only from code, never from docs, comments or tests alone; users' changes only, in users' terms; order by weight to the user; the Change Map recast as a coverage aid, not a measure of importance; the reminder reworded to name the headline as part of the notes. The altitude examples, the combine rule, the bookkeeping rule and the Deprecated/Security rule are carried over unchanged.

**Tuning, then held out.** The five tuning releases (set `tune` in `sample.tsv`): mint v0.0.6, dex-engine v0.2.9, portal v0.12.0, tick v0.3.2, switchboard v0.1.2. Procedure:

1. Run the current arm once (one repeat) on the tuning set, as reference.
2. Run `new-v0` once on the tuning set; close-read each body against the audit's account and the decided rules.
3. Revise to `new-v1`, run, read; up to three revisions in all (`v1`–`v3`). Revisions may change any wording of the instructions or reminder. Every version is kept in `prompts/` and every output in `data/bodies/tune/`.
4. Freeze the last version run (or an earlier one that read better, named with the reason) before any held-out call. The frozen version's name and SHA-256 are written into the report before the held-out run starts.

Constraint on revisions: no prompt text may name, quote or paraphrase any change from the 31 sampled releases; examples, where added, are invented. The three altitude examples carried over from the shipped prompt are exempt — both arms share them. Tuning-set results are reported but never enter the decision rule.

**Held-out run.** Both arms on the 26 held-out releases, two repeats each (104 calls). Each (release, arm) unit runs its two repeats back to back, so the second sends a byte-identical prompt within the cache window; units interleave arms release by release.

**Sample** (`sample.tsv`, frozen): the 31 AI-written releases the audit judged (`.workflows/headline-first-release-notes/imports/release-notes-audit.md`), across nine projects; 5 tuning, 26 held out. Every tag resolves in its repo (checked at design: all 62 `rev-parse --verify` calls succeed). Per release the file carries the headline expectation and the breaking change, both pre-registered from the audit and the discussion's decided rules:

| Release | Set | Headline expectation | Breaking change |
|---|---|---|---|
| agentic-workflows v0.6.0 | holdout | expected | — |
| agentic-workflows v0.8.0 | holdout | expected | — |
| agentic-workflows v0.5.12 | holdout | expected | — |
| agentic-workflows v0.8.3 | holdout | expected | — |
| agentic-workflows v0.8.12 | holdout | expected | — |
| agentic-workflows v0.7.0 | holdout | expected | — |
| agentic-workflows v0.7.80 | holdout | either | — |
| dex-engine v0.2.2 | holdout | expected | — |
| dex-engine v0.2.0 | holdout | expected (two bodies) | — |
| dex-engine v0.1.13 | holdout | expected unless one bullet | — |
| dex-engine v0.2.9 | tune | not expected (grab-bag) | — |
| portal v0.8.0 | holdout | expected | vim navigation keys removed |
| portal v0.11.1 | holdout | expected | old-format hooks stop firing after upgrade |
| portal v0.9.0 | holdout | expected unless one bullet | — |
| portal v0.12.0 | tune | expected | `x /tmp` now read as a search |
| switchboard v0.1.0 | holdout | expected | reserves become opt-in |
| switchboard v0.2.6 | holdout | expected | — |
| switchboard v0.1.5 | holdout | expected | piped output becomes JSON |
| switchboard v0.1.2 | tune | expected | — |
| agntc v0.6.0 | holdout | either | — |
| agntc v0.4.0 | holdout | expected unless one bullet | — |
| agntc v0.5.2 | holdout | expected | — |
| mint v0.0.6 | tune | expected unless one bullet | — |
| mint v0.0.2 | holdout | expected (two bodies) | — |
| mint v0.0.3 | holdout | expected unless one bullet | — |
| tick v0.3.0 | holdout | expected unless one bullet | description format becomes an escaped TOON string |
| tick v0.3.2 | tune | expected | — |
| stitch-cli v1.6.2 | holdout | not expected | — |
| stitch-cli v1.6.6 | holdout | expected (two fixes, one goal) | — |
| agentic-skills v0.2.0 | holdout | expected (two bodies) | — |
| agentic-skills v0.2.2 | holdout | either | — |

"Expected unless one bullet": a headline is expected unless the body's sections hold exactly one bullet and it states the whole release (the narrowed No Clear Headline rule).

**Inputs.** Each release's input is built as mint's regenerate fresh path builds it: `git diff --src-prefix=a/ --dst-prefix=b/ --no-color --no-ext-diff <prev>..<tag> -- .` at the repo root with the built-in `CHANGELOG.md` and `.mint.toml` excludes, the release's shared and `[release]` `diff_exclude` globs as read from `.mint.toml` at the tag, and the plain-mode `version_file` exclude; the Change Map from the same range with `max_prompt_bytes` 1,500,000. `[release].context` and `[release].prompt` are set in none of the nine repos (the audit). The exclusion check does not run (regenerate never runs it).

- Post-exclusion diff sizes: `python3 -I .workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/harness/sizes.py .workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/sample.tsv` → 31 releases, 16,790,711 bytes in total; two over the 1,500,000-byte cap: agentic-workflows v0.6.0 (1,927,664) and switchboard v0.1.0 (1,751,591).
- Both arms run every release as one call, the two over-cap releases included: production would chunk them, and the chunked path is outside this experiment. A release the model refuses as too large drops out of both arms, logged as a deviation.

**Instruments.**

- `harness/main.go` — `e1harness`, built into the mint module by `harness/build.sh` through a `go build -overlay`, importing mint's `internal/notes` (assembler, Change Map, `DefaultPrompt`, `OutputReminder`, `ComposePrompt`), `internal/ai` (the production transport, with its validation and single bad-content retry) and `internal/config` (`DefaultAICommand`, `DefaultMaxPromptBytes`). Reads `.mint.toml` at the tag leniently (only the keys that shape the diff, at either level). Per call it records prompt bytes and SHA-256, the input's SHA-256, the instructions' and reminder's SHA-256, the exclusions, timings and any error.
- mint at `ec5a0479` (`git -C /Users/leeovery/Code/mint rev-parse --short HEAD` → `ec5a0479`); its code is identical to the v0.1.0 release (`git -C /Users/leeovery/Code/mint diff --stat v0.1.0..HEAD -- internal cmd` → empty). Built with `go version` → `go1.27.3 darwin/arm64`.
- AI command: mint's shipped default, `claude -p --model sonnet --safe-mode --tools "" --strict-mcp-config --no-session-persistence --system-prompt "Follow the instructions you are given and output only what they ask for."`, prompt on stdin, run with the working directory at the release's repo (as `mint release` runs it). `claude --version` → `2.1.306 (Claude Code)`; the `sonnet` alias resolves to `claude-sonnet-5-5` (the `modelUsage` key of a `--output-format json` call). Per-attempt deadline 600 s (production's 180 s is raised so the largest prompts cannot time out on one arm and not the other). The CLI version is recorded again at the end of each phase; a change mid-run is a deviation.
- Measured on the instrument, on a 59,312-byte prompt of real diff (dex-engine v0.2.8..v0.2.9 with its excludes, plus a one-line instruction): 24,841 input tokens (about 2.4 bytes per token); sent a second time unchanged, all 24,841 came back as cache reads. Command: `claude -p --model sonnet --safe-mode --tools "" --strict-mcp-config --no-session-persistence --system-prompt "Follow the instructions you are given and output only what they ask for." --output-format json < probe.txt`, the `usage` field.
- `harness/run.py` — the driver: units, repeats in sequence, four units at once, one retry per failed repeat, resumable. Bodies and metadata to `data/bodies/<set>/<arm>/<project>-<tag>-r<n>.{md,meta.json}`; composed prompts and the per-release input (Change Map + diff) to `.workflows/.cache/headline-first-release-notes/experiment/headline-first-release-notes/` (too bulky for git; their SHA-256s are in the metadata).
- `harness/blind.py` — seeded (20261021) blinding: each held-out release's four bodies relabelled A–D in random order for the judges; Lee's six pairs relabelled A/B. Keys stay in the cache until the judgements and Lee's picks are in, then move to `data/`.
- `rubric.md` — the judges' checklist and return format.
- `harness/score.py` — executes the decision rule.

**Judging.** Six foreground general-purpose sub-agents (the session's model, Opus 5.5), dispatched together, grouped as the audit's auditors were: agentic-workflows (7 releases); dex-engine (3); portal (3); switchboard (3); agntc and mint (5); tick, stitch-cli and agentic-skills (5). Each receives `rubric.md`, its releases' repo paths and tag ranges, the audit section for its group, each release's input file, the pre-registered expectations, and the blinded bodies. Judges are read-only and return their JSON as text; the session writes it verbatim to `data/judgements/<group>.json`. A judgement that fails to parse is re-requested from the same group once; a second failure is a deviation.

**Lee's read** (secondary). Six held-out releases, pre-registered: portal v0.11.1, tick v0.3.0, switchboard v0.2.6, agentic-workflows v0.8.0, agntc v0.5.2, mint v0.0.2. For each, repeat 1 of each arm side by side as A/B, with a one-line account of what the release was; Lee picks the better notes against his bar. Delivered as `data/lee-read.md` once the held-out run is done; reported beside the judges, with their agreement; not in the decision rule.

**Environment.** This Mac; the nine repos read-only at their tags (git reads only); the AI calls on Lee's Claude subscriptions through the stock `claude` CLI.

## Controls and biases

- **Concurrent control.** The current arm runs in the same run, on byte-identical inputs, interleaved with the new arm release by release, with the same command, CLI version and settings. The shipped notes and the audit's counts are never the comparison; they are the ground truth's source only.
- **Repeats.** Two per arm per held-out release. The second repeat reads the first's prompt from cache; caching does not change sampling. Repeat-to-repeat agreement is reported.
- **Smoke pass.** The tuning set's first runs are the smoke pass for the harness; a harness defect found there is fixed before the held-out run and logged.
- **Overfitting.** The decision rests only on held-out releases, run cold on a prompt frozen before them. Known bias: the session wrote the prompt having read the audit's account of all 31 releases, held-out included; the prompt's rules are written per failure class, never per release (the no-sample-content constraint above), but the held-out set is not blind to the prompt's author.
- **Judge blinding is partial.** Bodies are relabelled and order-shuffled, and judges never see either prompt; but a headline line reveals the new arm. The checklist's factual items (is this claim true, is this item internal) are less exposed than the overall score; the preference guard rail is the most exposed measure.
- **Judge as model.** The judges are Claude models judging Claude output; their ground truth is the audit's, which is itself judgement. Mitigations: one judge sees all four bodies of a release, so the reference is shared across arms; the session's close read; Lee's six-release read as a check on the judges against his bar.
- **Input-present calls** are the judges' reading of the input file. A judge may call a fact present that only a very careful reader would find; the call is "could a careful reader of the input have got it right".

## What this does not measure

- The chunked path: chunk calls and the merge, where large releases' headlines would be picked. Every release here runs as one call.
- Repos with a `[release].prompt` override or `[release].context`; models other than the shipped default; mint's exclusion check and the inputs it would change.
- Changes to what mint hands the AI — the Change Map's code, commit history, previous notes. Those are the questions a failed class raises, not ones this experiment answers.
- The `diff_exclude` pathspec bug: inputs reproduce it as shipped (switchboard's leaked `testdata/`), identically in both arms.
- Releases outside the audited 31, hand-written releases, and how the prompt fares on future model versions.
- Whether readers other than Lee share the judges' or Lee's preferences.
