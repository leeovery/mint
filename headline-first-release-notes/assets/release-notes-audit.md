# Release-Notes Audit — 31 Released Notes Against Their Code

*Evidence gathered 2026-10-21 for the headline-first-release-notes discussion.*

## Method

Every project on this machine with a `.mint.toml` was surveyed for tags since 2026-06-13, the day the current notes format (one-line bullets, no TL;DR) landed. Twelve repos besides mint carry one; about 340 tags fall in the window; none of the twelve sets `[release].prompt` or `[release].context`, so their notes come from the default prompt this feature changes.

Six read-only auditors took a cross-section of 31 AI-written releases, weighted toward the largest by post-exclusion diff (where salience matters most) plus a few small ones: agentic-workflows (7), dex-engine (4), portal (4), switchboard (4), agntc (3), mint (3), tick (2), stitch-cli (2), agentic-skills (2). Each auditor judged what a release was really about from the full commit history and planning docs — context mint's AI never has — then where the notes put it, the order within sections, misfiled, missing and misrepresented items, what the Change Map's novelty and largest-file lines pointed at, and the headline the decided rules would give it.

Caveats: the ground-truth calls and the counts below are the auditors' judgement, not a controlled measurement. Releases whose notes were not AI-written were skipped: dex-engine v0.1.0, portal v0.11.0, agntc v0.5.0, switchboard v0.2.0 and v0.2.4 (hand-written, mostly because the diff was over the AI's limit), and tick v0.2.8 (no notes). One auditor (agntc and mint) judged "new folder" at package level rather than by mint's first-segment rule; the collated counts below use mint's actual rule.

## Cross-project synthesis

**Did the notes lead with the release's main work?** Of the 30 releases with a main body of work (dex-engine v0.2.9 is a grab-bag): led cleanly in 17, partly in 7 (a piece of the work opens the notes, or the right item with the wrong framing), not at all in 6 (a side item, a dev-only harness, a guard instead of the fix, or the docs site first).

**Fragmentation is near-universal.** The main body of work was split across several bullets — typically 4 to 14, across two to four sections — in all but one release (stitch-cli v1.6.2, a single feature). In almost none did any one line state the whole. Items were stated twice within the same notes in five releases (dex-engine v0.2.2, v0.2.0; tick v0.3.0, v0.3.2; mint v0.0.3). The model never collapsed a large feature into a single bullet; it over-splits.

**The fixed section order buries reshaping work.** Where the main work is a reshaping of existing behaviour — an engine replatform, a redesign, a format rewrite, a bug fixed by rework — it lands in 🔧 Changed, below ✨ Added, so small new items outrank it: agentic-workflows v0.6.0, v0.5.12, v0.8.3; portal v0.8.0, v0.11.1; tick v0.3.0, v0.3.2; agentic-skills v0.2.0. Twice the model broke the fixed order to put the main work first (agentic-workflows v0.8.12, stitch-cli v1.6.6).

**Inversions within sections are frequent but modest** — present in roughly half the sample. Bullet count and rank follow commit or file granularity and how easy an item is to name (a new verb, a flag, a column, an exit code), not its weight to the user; README prose is weighted as content.

**The Change Map's structural signals barely work.** mint's new-directory detection reads only a path's first segment, so a new package under an existing `internal/`, `src/`, `skills/` or `laravel/skills/` is never flagged. It fired in 6 of 31 releases: twice on a pre-existing `design/` folder (it calls a folder new when every changed path under it was added, without checking the folder existed), once on a docs site that then led the notes (agentic-workflows v0.6.0), once on README demo assets (switchboard v0.1.0), once on a 6-line wrapper the notes rightly ignored (mint v0.0.2), and once on the smaller of two co-equal bodies, which it promoted (agentic-skills v0.2.0). The area rollup collapses to one or two top-level folders. The largest-file line was often a test file, a deleted file or a dev-only script. Separately, the prompt's own rule ("new structure … is the strongest signal") read against the diff turned each new internal package into a feature bullet in mint v0.0.6.

**A headline under the decided rules** would apply in about 23 of 31. The only-the-main-thing exemption ruled it out in five or six releases (dex-engine v0.1.13, portal v0.9.0, agntc v0.4.0, mint v0.0.3, v0.0.6; tick v0.3.0 arguably) — and four of the six auditors independently flagged that this is where the main work was most fragmented: the notes held nothing but the main thing, in 5 to 10 pieces, none naming it. Their proposed test: the exemption applies only when one bullet already states the whole release. Genuine no-headline cases: dex-engine v0.2.9 (grab-bag), stitch-cli v1.6.2 (one user-facing bullet). Hand-written evidence: Lee's switchboard v0.2.0 notes open with a headline (though it inventories the pieces); his v0.2.4 grab-bag has none.

**Failures beyond salience** (release counts approximate):
- **False or stale claims shipped** (~7): a behaviour change invented from a README edit (tick v0.3.0), a fix invented from a comment-only edit (portal v0.12.0), a test-only change presented as a fix (dex-engine v0.2.9), last release's features re-announced because the README now documents them (switchboard v0.1.5), already-true behaviour presented as new (mint v0.0.3), an existing gate presented as new (agentic-workflows v0.8.0), a never-shipped bug presented as fixed (agntc v0.4.0). Root cause: from the diff alone the AI cannot tell newly built behaviour from newly documented behaviour, comments or tests.
- **Work new in this release presented as a change or fix to the last** (~7): dex-engine v0.2.0, v0.1.13; portal v0.9.0, v0.12.0; agntc v0.4.0; switchboard v0.2.6; agentic-workflows v0.8.0 (refactors as fixes).
- **Internal and dev-only changes presented to users** (~19 of 31): CI, lint, release tooling, internal packages, engine verbs, migration numbers, test harnesses, internal flags.
- **Section misfiling** (~15): fixes under Added or Changed, redesigns of existing views under Added, a performance project as three Added items, regression fixes as new features.
- **Breaking changes buried or unflagged** (5): portal v0.11.1 (old hooks stop firing), v0.8.0 (vim keys removed); tick v0.3.0 (description format); switchboard v0.1.5 (JSON when piped), v0.1.0 (reserves opt-in).
- **Notable items missing** (~6) and **mechanism phrasing, spec jargon, multi-sentence bullets** (portal v0.8.0, switchboard v0.2.6).

**Found outside notes design:**
- **An exclusion bug in mint.** `diff_exclude` globs are passed to git as plain `:(exclude)` pathspecs, and in git's default pathspec mode `**/testdata/**` does not match a repo-root `testdata/`: on switchboard v0.0.6..v0.1.0, `git diff --name-only v0.0.6 v0.1.0 -- . ':(exclude)**/testdata/**' | grep -c '^testdata/'` → 172, against 0 with `':(exclude,glob)**/testdata/**'` — the auditor puts the leak at 45% of that release's prompt. portal's `**/*_test.go` likewise missed a top-level test file.
- **The largest releases never reached the AI.** In five of the six groups the largest release in the window shipped hand-written notes because its diff was over the AI's limit — the gap the token-aware diff sizing work (shipped to main, not yet released) closes.
- **CHANGELOG drift** in stitch-cli: regenerated sections no longer match the tag bodies (v1.6.2, v1.6.3).

---

# Per-project reports

## agentic-workflows — release-notes audit (7 releases)

**Sample:** the five largest post-exclusion (v0.6.0, v0.8.0, v0.5.12, v0.8.3, v0.8.12) plus v0.7.0 (medium, a new feature) and v0.7.80 (small, mixed fixes). Nothing skipped — every candidate had AI-written notes. Sizes use the current `.mint.toml` exclusions (until v0.8.3 the config also excluded the `knowledge.cjs` bundle, ≤292 lines per release; ranking unchanged).

### v0.6.0 (prev v0.5.13) — 393 files, +19205 −9555
- **Ground truth:** one main body — ~75 commits move the workflows onto a new engine (`skills/workflow-engine`, ~10.6k lines) and retire the manifest CLI. Secondary: a docs site (#461, 1.4k) and knowledge-base chat setup (#420, 1.2k).
- **Led with it?** No — the first bullet is the docs site; the engine is 🔧 Changed 1, bullet 4 of 22.
- **Order within sections:** ✨ Added puts the docs site above the knowledge-base setup work (similar size, changes product behaviour).
- **Collapsed or split?** Changed 1, 3, 4 plus Removed 1.
- **New-folder signal:** the only new folder `docs/` pointed at the secondary work, and the AI led with it. Largest file the deleted `workflow-manifest/scripts/manifest.cjs` (1177) points at the main work.
- **Headline:** "Workflow state, menus and dashboards now run on one deterministic engine in place of the manifest CLI and per-skill scripts."
- **Other:** six 🐛 Fixed and three 🔒 Security bullets from fixes made during the rebuild sit below the main work.

### v0.8.0 (prev v0.7.80) — 271 files, +11930 −2481
- **Ground truth:** one main body — a Claude Code mod draws gate menus as clickable buttons (#1289/#1290/#1292/#1294/#1303, ~7.8k lines), built on menus the engine now draws.
- **Led with it?** Yes, ✨ Added 1 ("Buttons above the prompt").
- **Order within sections:** 🔧 Changed — "Dispatches to background agents…" (#1304, 334 lines) above per-conversation folders (#1312, 1876) and render vocabulary (#1310, 1059).
- **Collapsed or split?** ~six bullets (Added 1, Changed 1, 2, 6, 7, the Fixed bullets); the mod's own changes #1294 and #1303 folded away.
- **Misfiled / missing / misrepresented:** ✨ Added 2 "A settings-only sign-off gate on the investigation findings" is wrong — the gate already existed; only its rendering moved to the engine; "settings-only" means nothing. Both 🐛 Fixed bullets are internal refactors (#1301 and others) dressed as fixes. The tmux-label migration gets its own bullet.
- **New-folder signal:** `design/` flagged as new though it already held 30+ design docs (false novelty). Largest file a test `skills/workflow-gates/tests/register.test.ts` (3666). The AI ignored both.
- **Headline:** "Workflow menus now appear as clickable buttons above the prompt, drawn by a Claude Code mod that switches itself on at first start."

### v0.5.12 (prev v0.5.11) — 74 files, +701 −12427
- **Ground truth:** one main body — Deep Discovery (PR0–PR5 plus #398). Size almost entirely deletions: the explorer HTML (5.4k) and shipped design docs (6.6k).
- **Led with it?** Partly — ✨ Added 1 is one piece (briefs); the core "discovery conversations go deeper" is 🔧 Changed 1, bullet 3 of 8; the feature is never named.
- **Collapsed or split?** Five bullets across Added and Changed.
- **Missing:** migration 045 (moves session logs).
- **New-folder signal:** none; the map's novelty list is 38 "Removed:" lines; largest file the deleted `workflow-explorer.html` (5436) — housekeeping, though the AI kept it in 🗑️ Removed.
- **Headline:** "Discovery now works through real decisions before harvesting topics, and hands each topic a brief so its research or discussion starts where discovery left off."

### v0.8.3 (prev v0.8.2) — 189 files, +6257 −6082
- **Ground truth:** one main body — the knowledge base moves into the engine and fills its vectors in the background (#1429/#1430/#1431, ~11.4k of 12.3k lines); the rest ~80 small prose fixes.
- **Led with it?** Partly — ✨ Added 1 is one piece (background vector fill); the framing statement is 🔧 Changed 1, bullet 6 of 25.
- **Order within sections:** the largest fix (#1396, 43 lines) last in 🐛 Fixed below 2–10-line fixes; Changed 4 (boot indexes by keyword, main work) below the two #1431 config bullets; Changed 10 (#1393, 130 lines) below Changed 8 (#1404, 6 lines).
- **Collapsed or split?** ~seven bullets across three sections.
- **Misfiled / misrepresented:** engine verbs as user features (`engine commit --migrations`, engine-rendered screens); 🗑️ Removed 2 and 3 are migration bookkeeping (062/063/064), not removed features.
- **New-folder signal:** none. Largest file the deleted `src/knowledge/index.js` (3062) — main work.
- **Headline:** "The knowledge base now runs inside the engine and fills its vectors in the background, so phase starts and closes never wait on the embedding service."

### v0.8.12 (prev v0.8.11) — 202 files, +3417 −6592
- **Ground truth:** one main body — the "entry layer" programme (design/entry-layer.md, #1470–#1479): phase-entry skills fold into their phases, four continue skills merge into one, the start loads lighter.
- **Led with it?** Yes, 🔧 Changed 1 — reached by putting 🔧 Changed before ✨ Added, breaking the fixed section order.
- **Order within sections:** Changed 7 "framework loads once" (#1478, 55 lines) above Changed 8 the lighter start (#1479, 489); ✨ Added 1 the b/back row above the promote work (#1482, 687).
- **Collapsed or split?** Badly split, ~12 bullets.
- **Misfiled / misrepresented:** fix commit #1482 split across Added and Fixed; `engine topic incorporations`, migrations 068–070 and `--no-session-log` are internal.
- **New-folder signal:** `design/` again a false "new" folder (design/entry-layer.md). Largest file a deleted spec-entry `gateway.cjs` (343) — main work.
- **Headline:** "Each phase now starts inside its own skill and one continue menu covers all linear work, so a phase start reads its state once."

### v0.7.0 (prev v0.6.70) — 64 files, +4437 −252
- **Ground truth:** one main body — the product roadmap (#915–#944); two small unrelated items: engine-drawn menus (#943), voice rules (#945, 7 lines).
- **Led with it?** Yes, ✨ Added 1.
- **Order within sections:** ✨ Added — the #930 integrations (167 lines) get three bullets ahead of knowledge-base indexing (#926, 675); 🔧 Changed — the roadmap's cancel-revert behaviour below the unrelated menu change.
- **Collapsed or split?** Seven bullets.
- **Missing / misrepresented:** the docs page (#944) missing; the voice-rule change is agent-prompt guidance presented as user-facing.
- **New-folder signal:** nothing registered — new `skills/workflow-roadmap/` invisible at leading-segment granularity; `design/` also modified, so not flagged. Largest file `roadmap.cjs` (1034) — main work.
- **Headline:** "A product roadmap now sits above work units, for staging capabilities into horizons and pulling slices into delivery." Nearly repeats ✨ Added 1; allowed because the notes hold other items.

### v0.7.80 (prev v0.7.79) — 13 files, +538 −318
- **Ground truth:** user-facing work is one main body — rate-limit and quota handling (#1309, 419 lines). The largest commit is a dev-only eval harness (#1307, 96k lines inside excluded `tests/`).
- **Led with it?** No — ✨ Added 1 (the only Added) is the dev-only eval harness, inferred from one `docs/contributing.md` line.
- **Collapsed or split?** Rate-limit work across Changed 1, Changed 2, Fixed 1.
- **Misrepresented:** a contributor tool presented as a user feature.
- **New-folder signal:** none; largest file `src/knowledge/index.js` (341) — uninformative.
- **Headline:** borderline — "Embedding requests now wait out a provider's rate limit and stop at once with a clear message when the quota is spent." Leaving it out also defensible.

### Project summary
- Main work led cleanly in 3 of 7 (v0.8.0, v0.7.0, and v0.8.12 only by breaking section order); partly in 2 (v0.5.12, v0.8.3 — a piece opens ✨ Added, the framing at 🔧 Changed 1); not in 2 (v0.6.0 opens with the docs site, v0.7.80 with a dev-only harness).
- Main cause: the main work usually lands in 🔧 Changed — here it is usually a reshaping of existing behaviour (engine replatform, knowledge base into the engine, entry layer, deeper discovery) — and the fixed section order puts it below ✨ Added, so small items that happen to be new outrank it. v0.8.12 shows the model working around this by reordering sections. A headline fixes this directly.
- Splitting every time: all six single-body releases spread the main work over 4–12 bullets across 2–4 sections. "Combine closely related changes" works at commit level at best, never at the level of a whole body of work.
- Inversions frequent but modest (v0.8.0, v0.8.3, v0.8.12, v0.7.0). Bullet count follows commit or file granularity, not weight.
- The new-folder signal does not work here: grouping by leading path segment makes every new skill under `skills/` invisible (workflow-engine, workflow-roadmap, workflow-gates). False novelty: a folder is called new when every changed path under it was Added, without checking the folder already existed (`design/` in v0.8.0, v0.8.12). When genuinely right it pointed at secondary work (`docs/` in v0.6.0, which then led).
- The largest file is usually a deleted file or a test (v0.6.0, v0.8.3, v0.8.12; v0.8.0's `register.test.ts`); nested `skills/*/tests/` escape the root-only `tests/` exclude.
- Headline applies in 6 of 7 (v0.7.80 borderline); none is a grab-bag; v0.7.0 shows the headline nearly duplicating ✨ Added 1.
- Beyond ordering: internal details as user features (engine verbs, migration numbers, flags — v0.8.3, v0.8.12; a contributor eval tool — v0.7.80; refactors filed as 🐛 Fixed — v0.8.0); something old presented as new (v0.8.0 sign-off gate); fixed section order broken (v0.8.12); size misleads as a weight signal (v0.5.12 third-largest only from deleted docs and HTML).

---

## dex-engine — release-notes audit (4 releases)

**Skipped:** v0.1.0 (+57,175 −1,650, the largest): hand-written fixed body shipped `--no-ai` because the diff was over the AI ceiling (commits b1f34cf, 0a5192a, 4d175e0). v0.0.1 has no previous tag.

All tags here date from 2026-08-18 on. Exclusions applied: `design/`, `CHANGELOG.md`, `.mint.toml`.

**How the Change Map behaves here.** `newDirectories` looks only at the first path segment, and all code sits under `src/` and `tests/`. The "New package/dir" line fired in 0 of 4 releases, including v0.2.0's brand-new `src/dex_engine/directives/` package. "Churn by area" shrinks to `tests/` > `src/` > `instance/`. The largest file was a test file every time. The map gave the AI almost no useful signal.

### v0.2.2 (prev v0.2.1) — 69 files, +10,704 −752
- **Ground truth:** one main body of work — captured links now land what they actually hold: GitHub routes #172, table/code/math extraction #176, X articles #175, X video transcription #174 (~9.1k lines together), with migrations 15–18 healing existing instances. Second thread: reruns keep a good copy (#177, 1.6k lines), plus small collision fixes (#171, #173).
- **Led with it?** Partly. ✨ Added 1 of 7 is the GitHub piece; the theme only appears after five Added bullets, and nothing names it as a whole.
- **Order within sections:** mild — X video (#174, 2.3k lines) below X article (1.8k); the capture-name fix (287 lines) among the features.
- **Collapsed or split?** Heavily split, 14 bullets: GitHub as Added 1 and Fixed 1; #176 across Added 2 and 3; migrations their own bullet apart from the fixes they heal; rerun-keeps-landing as four Changed bullets (1, 2, 4, 5).
- **Misfiled / missing / misrepresented:** fixes under Added (capture-name suffix "Fixes #150"; the X-article renderer that was stripping blocks); a fix under Changed (media slots, one unit's file overwriting another's); internal ledger detail presented to users ("`enrich compact` now preserves a requeued unit's last landing record…").
- **New-folder signal:** none fired. Largest file `tests/pipeline/test_run.py` (899 lines) points at the secondary rerun work.
- **Headline:** "Captured links now land what they actually point at with nothing lost in extraction, and existing instances re-read what earlier versions got wrong."

### v0.2.0 (prev v0.1.17) — 83 files, +7,032 −552
- **Ground truth:** two co-equal bodies — the lens (#161–#163, #167) replaces scope as how an instance reads what's shared into it; directives (#159, #164) are new engine-shipped tasks carrying existing instances over to it.
- **Led with it?** Yes — ✨ Added 1 (directives) and 2 (lens); the machinery leads ahead of the change the owner sees.
- **Order within sections:** Changed 2 "Harvest no longer narrows by lens" (#167, 44 lines) above the exclude purge, owner-asking routing and the directive-pending refusal (Changed 6, a real behaviour gate); the dex-inbox origin-URL fix (Changed 5) also above that refusal.
- **Collapsed or split?** ~12 bullets; the directive command appears three times (Added 1, 3, 4).
- **Misfiled / missing / misrepresented:** "Harvest no longer narrows by lens" describes a correction made inside this release (the lens is new in v0.2.0; the commit says harvest follows the subject "as it did before the lens") — the net change is the door's scope check replaced by a lens verdict after harvest, which the bullet half-says. Added 4 dev-only ("wired through the standard entry-point/shim/README plumbing"). The origin-URL change is a fix under Changed.
- **New-folder signal:** none fired (`directives/` sits under `src/`). Largest file `tests/directives/test_directive_1.py` (717 lines) happens to point at the main work.
- **Headline:** "Each instance now reads what's shared into it through a lens of its own, and engine-shipped directives bring existing instances over after the next pull."

### v0.1.13 (prev v0.1.12) — 39 files, +4,589 −224
- **Ground truth:** only the main thing — all five PRs (#126–#130) are the instance map: compiled `state/map.json`, rendered `wiki/index.md`, custody through `enrich place`, MCP map tools, migration 10.
- **Led with it?** Yes, ✨ Added 1 of 5 (`dex-map`).
- **Order within sections:** good.
- **Collapsed or split?** Split into 10 bullets; none names the whole.
- **Misfiled / missing / misrepresented:** two in-release changes dressed as changes to the previous release — Changed 4 "The MCP `graph` tool's default view is capped" (tool and cap both new in #128); Fixed "`dex-exclude` now recompiles the map…" (the map did not exist before).
- **New-folder signal:** none. Largest file `tests/test_instance_map.py` (747 lines), on target.
- **Headline:** none by the only-the-main-thing rule — but see the summary.

### v0.2.9 (prev v0.2.8) — 15 files, +852 −59 (small/medium)
- **Ground truth:** grab-bag — the article-fallback fix keeping headings and lists (#213, 560 lines) is the largest piece; two shared-unit ownership fixes (#209, #212).
- **Led with it?** No — Changed precedes Fixed, so a minor message tweak (Changed 1) heads the notes; the largest item is Fixed 1.
- **Misfiled / missing / misrepresented:** Fixed 5 "The park note for an unfetchable fetch URL now includes the cause" is false — no source changed; the commit made a test assert the cause. A test change presented as a user fix.
- **New-folder signal:** none. Largest file `tests/drivers/test_web.py` (290 lines), points at the article fix.
- **Headline:** none (grab-bag).

### Patterns
- The main work led every time there was one (v0.2.2, v0.2.0, v0.1.13). Inversions were mild: v0.2.0 Changed 2; v0.2.2 X video below X article; v0.2.9 a minor Changed bullet heading the notes because of fixed section order — a structural cause no within-section ranking fixes.
- The new-folder signal is blind where code lives under `src/` and `tests/`: first-segment matching never fires and the area rollup is mush. Good ordering came from the AI's own reading of the diff.
- Fragmentation is the biggest failure: the main body was split every time (14, ~12, 10 bullets), nothing stated the whole, items repeated (directive command three times; GitHub in Added and Fixed). The gap a headline fills.
- In-release changes reported as changes or fixes against the previous release (v0.2.0 harvest; v0.1.13 graph cap and exclude) — likely from WHY-comments and docs written against an old state ("never narrows", "no longer") plus edits to existing files read as a delta from the last tag.
- Misfiling between sections: fixes as Added or Changed (v0.2.2 ×3, v0.2.0 ×1).
- Test-only and dev-only changes presented as user changes: v0.2.9 park note, v0.2.0 plumbing, v0.2.2 `enrich compact`.
- A headline applies in 2 of 4 (v0.2.2, v0.2.0); v0.2.9 rightly none.
- Rule question: v0.1.13 is "only the main thing" spread over 10 bullets, none naming it. "A headline would only repeat it" assumes the main thing already has its own bullet; here a headline ("Each instance now has a compiled map of its topics, entities and links that the wiki index and chat tools read from") adds the missing frame. The exclusion could narrow to notes where one bullet already states the whole release.

---

## portal — release-notes audit (4 releases)

**Sample:** v0.8.0, v0.11.1, v0.9.0 (the three largest AI-written), v0.12.0 (medium). **Skipped v0.11.0** (largest, 30,292 lines): hand-written notes for the `--no-ai` path (commit 3c1933949 "chore(release): hand-write the v0.11.0 notes for the --no-ai path"). Measured with the `.mint.toml` excludes as `:(exclude)` pathspecs plus `CHANGELOG.md` and `.mint.toml`.

### v0.8.0 (prev v0.7.7) — 51 files, +10,549 −1,337
- **Ground truth:** one main body — a visual redesign of the picker ("spectrum-tui-design": 87 impl, 86 spec commits; the spec calls it "a visual reskin"): colour layer, light/dark detection on a canvas Portal paints itself, header/wordmark, `?` help modal, cold-start loading screen, new keymap.
- **Led with it?** Partly. ✨ Added 1 of 2 is a piece ("Automatic light and dark canvas"). The only bullet naming the redesign is 🔧 Changed 1 of 4, third overall. A reader sees a colour-mode feature, not "the whole UI changed".
- **Order within sections:** no clear inversion; Added-first format puts pieces above the whole.
- **Collapsed or split?** Five bullets (canvas, loading screen, reskin, keymap, background restore); the new `?` help modal folded into the Changed keymap bullet.
- **Misfiled / missing / misrepresented:** the help modal is new (belongs in Added); "README has been restructured" is docs churn presented as a product change; spec jargon leaks ("Modern Vivid theme", "owned, mode matched canvas", "honest loading screen", raw hex codes); bullets run to two or three sentences against the one-sentence rule. The 1,244-line screenshot harness (`cmd/capturetool`, `internal/capture`) correctly omitted.
- **New-folder signal:** mute in mint (`leadingDir` keys on the first path segment; nothing under `internal/` or `cmd/` counts as new). At package level the new dirs would be `cmd/capturetool`, `internal/capture` (dev tooling) and `internal/tui/theme` (main work) — two of three tooling. Largest file `internal/tui/model.go` (3,141 lines) points at the main work.
- **Headline:** "The picker is redesigned, with a new colour scheme that follows your terminal's light or dark background."
- **Other:** removing the vim navigation keys is breaking and sits mid-list unflagged.

### v0.11.1 (prev v0.11.0) — 127 files, +4,936 −1,152
- **Ground truth:** one main body — the "resume-hooks-silently-lost" bugfix (411 impl commits plus an investigation). Hooks could vanish, so restored panes came back to a bare shell; the fix keys each hook to a per-pane token, refuses a nonexistent pane, adds a lock, makes the cleanup sweep conservative. The rest small: two TUI fixes, a CI split, a large test-helper refactor.
- **Led with it?** No. ✨ Added 1–2 are small items (a fourth column in `hook list`; refusing bad names on rename). The core fix is 🔧 Changed 1 of 8, framed as "survive … more reliably", never as fixing silently lost hooks.
- **Order within sections:** Changed — "`hook rm` exits non-zero" (2) above the lock (3) and conservative sweep (4); the sweep was half the root cause.
- **Collapsed or split?** ~seven bullets, none stating the fix.
- **Misfiled / missing / misrepresented:** the main fix under Changed, not Fixed. Missing: `hook set` now fails when its pane doesn't exist (one of two root causes, the degenerate `:.` key). Missing and serious: per spec §8 no migration ships, so every hook saved in the old format stops firing after upgrade; Changed 4 says only that such entries are "kept forever unless removed by hand". Dev-only bullets in Changed: the Homebrew CI job and golangci-lint coverage.
- **New-folder signal:** mute under first-segment keying. Package level: `hooksweep`, `nanoid` (main work) plus `commandertest`, `hookstest`, `harnesstest`, `shellquote` (helpers). Largest file `internal/tmux/tmux.go` (360 lines), loosely related.
- **Headline:** "Resume hooks now stay tied to their pane through renames and rearrangement, so a restored pane resumes its work instead of opening a bare shell."
- **Other:** exclusions leak — `**/*_test.go` doesn't match top-level `main_panic_test.go` (110 lines); non-`_test.go` test-helper packages pass through: 2,838 of 6,088 lines (47%) of the AI's diff was test scaffolding.

### v0.9.0 (prev v0.8.5) — 44 files, +5,298 −144
- **Ground truth:** only the main thing — "restore-host-terminal-windows" (every commit): mark sessions in the picker, press Enter, each opens in its own terminal window; `portal spawn` is the CLI equivalent.
- **Led with it?** In topic, but ✨ Added 1 leads with the `portal spawn` CLI; the front door (multi-select in the picker) is Added 2. The spec defines the feature as "Adds a multi-select mode to the Sessions page".
- **Order within sections:** the secondary CLI above the picker flow — a new verb is easy to name, and `cmd/spawn.go` is a big new file.
- **Collapsed or split?** One feature → seven bullets (four Added, one Changed, two Fixed).
- **Misfiled / missing / misrepresented:** both 🐛 Fixed bullets describe the new feature's own designed partial-failure contract and a bug fixed during implementation — nothing shipped before was broken. 🔧 Changed "`portal attach` gained an internal `--spawn-ack` flag" is internal plumbing presented as a user change.
- **New-folder signal:** mute in mint. Package level `internal/spawn`, `internal/spawntest` (correct). Largest file `internal/tui/model.go` (680 lines).
- **Headline:** none under only-the-main-thing — but the rule's premise fails: the notes split the feature so far that no line says "open several sessions at once, each in its own window"; a headline would add, not repeat.

### v0.12.0 (prev v0.11.1) — 25 files, +1,320 −190
- **Ground truth:** one main body — session search from the shell, "open-with-forced-filter" (207 impl commits): `x /term`, completion, a directory column; plus small hooks/doctor fixes from PRs #8–#13.
- **Led with it?** Yes, ✨ Added 1 of 3. Changed 1 (`x /tmp` now read as a search) the right breaking-change call-out.
- **Order within sections:** Fixed — `doctor --fix` under-report (1) above "a `hooks.json` that fails to parse is no longer written over" (2), which prevents losing every hook entry.
- **Collapsed or split?** Reasonable: three Added bullets.
- **Misfiled / missing / misrepresented:** 🐛 Fixed 3 ("`hook list` could show the wrong location…") is invented — `cmd/hooks.go` changed by one comment (commit fd0e01903 "docs: three claims the tree falsifies"). 🐛 Fixed 4 ("grouping guesses kept separate…") is internal groundwork for the directory column, presented as a bug fix.
- **New-folder signal:** none at any level. Largest file `internal/tui/model.go` (326 lines).
- **Headline:** "Open a session from the shell by searching its name with `x /term`." — nearly repeats Added 1, because the notes already lead well.

### Patterns
- Main work led once in four (v0.12.0). v0.9.0: topic led via the secondary entry point. v0.8.0, v0.11.1: a fragment or side item first; the whole-feature bullet third.
- Fragmented in all three large releases (5, ~7, 7 bullets); no bullet states the whole. The fixed Added → Changed → Fixed order compounds it: Added holds pieces or side items, the main work lands in Changed (v0.8.0, v0.11.1).
- Inversions within sections in three of four (v0.11.1, v0.9.0, v0.12.0). Likely causes: how easy an item is to name and see (a new verb, an exit code, a column) and file size, not user weight.
- The new-folder signal does nothing here: `newDirectories` and `areaOf` use the first path segment, so the "New package/dir" section was empty in all four; the size rollup is `internal/` vs `cmd/`; "Largest file" was `internal/tui/model.go` three times of four. At package level it would point at dev tooling or test helpers as often as at the main work.
- Headline: three of four (v0.8.0, v0.11.1, v0.12.0 — the last nearly repeating Added 1). v0.9.0 ruled out as only-the-main-thing, but that exemption should apply only when one bullet already states the whole.
- Failure classes beyond ordering: (1) a fix invented from a comment-only edit (v0.12.0); (2) work on a feature new in the same release filed as Fixed (v0.9.0); (3) internal/dev changes presented as user changes — `--spawn-ack`, CI, lint, README, an internal refactor (all four); (4) a real fix under Changed (v0.11.1); (5) a breaking upgrade buried — old hooks stop firing (v0.11.1), vim keys removed (v0.8.0); (6) spec jargon reaching users (v0.8.0); (7) exclusion leaks diluting the diff (47% of v0.11.1's diff test scaffolding).

---

## switchboard — release-notes audit (4 releases)

**Sample:** all 20 tags fall after 2026-06-13. The two largest post-exclusion — v0.2.0 (887 files, ±89.6k lines, 6.6 MB) and v0.2.4 (315 files, ±11.5k, 3.7 MB) — skipped: each diff was "past what the AI takes", so the notes were hand-written into a `[release] fallback` and shipped `--no-ai` (commits ec7aa07f, bebf9d66). Audited: the next three largest (v0.1.0, v0.2.6, v0.1.5) plus medium v0.1.2. Current `.mint.toml` excludes plus CHANGELOG.md and .mint.toml; the Change Map modelled on mint's current `changemap.go`; prompt and Change Map code unchanged across the sampled tags.

### v0.1.0 (prev v0.0.6) — 336 files, +23621 −1666
- **Ground truth:** one main body — the dashboard redesign (milestone 5): Accounts view, Sessions, Runway, themes, charts, flip cards, live request stream, router history/events (PRs #53–#64, ~20k lines). Reserves becoming opt-in (#66, 74 lines) a small behaviour-changing extra.
- **Led with it?** Yes, ✨ Added 1 of 12 ("A redesigned dashboard: a card per account…").
- **Order within sections:** mild — themes (#56, 2752 lines, a new package plus a picker) 6th, below "Card charts" (#62, 1561) and "Flip a card" (#58, 1547); the `?` help panel above the `/history` and `/stream` endpoints and router events.
- **Collapsed or split?** ~10 Added plus 3 Changed — defensible at this size, but no single statement of the redesign.
- **Misfiled / missing / misrepresented:** "Animated demos and stills for the README… with the new `capturetool`" is dev tooling in ✨ Added (#54 "test: a capture harness", #64); "README rewritten" listed as a change; reserves becoming opt-in alters default routing for existing users, unmarked in 🔧 Changed.
- **New-folder signal:** new top-level `art/`, `demo/`, `testdata/` — all README-demo assets; the real new packages `internal/capture`, `internal/theme` invisible (first path segment only). Largest file `internal/dashboard/runway.go` (605), a piece of the main work. The signal pointed at minor dev stuff, which the AI included but ranked last.
- **Headline:** "The dashboard is redesigned to show at a glance where each account stands and where its sessions run."
- **Other:** exclusion leak — `':(exclude)**/testdata/**'` does not match a repo-root `testdata/` in git's default (non-glob) pathspec mode; `:(exclude,glob)` does. 172 `testdata/vhs` files (ANSI frames and tapes, 782 KB) reached the AI — 45% of the 1.75 MB prompt.

### v0.2.6 (prev v0.2.5) — 81 files, +9060 −1234
- **Ground truth:** one main body — performance: finished days stored as per-day digests and rows, so the List, session pages, the Log's look-back and turns open fast (PRs #135, #136, #138–#140, ~11.9k lines). Only other item: a small Runway tweak (#137, 145 lines).
- **Led with it?** Technically (✨ Added 1 of 3), but phrased as mechanism ("Finished days are now digested into small per-day files (session states, a day head, and rows)"); the user benefit, speed, first appears in bullet 2.
- **Order within sections:** 🔧 Changed — the small Runway "COMING UP no longer numbers accounts" (#137) 3rd, above main-work bullet 4 ("reads each day back from its end").
- **Collapsed or split?** 9 of 10 bullets across Added, Changed, Fixed. The List opening fast (#135, 3207 lines, the second-largest PR) never named.
- **Misfiled / misrepresented:** a performance change filed as three ✨ Added items; "Session pages and Log rows are built from compact stored records, and request costs are summed as token counts and priced when shown" pure implementation; two 🐛 Fixed bullets (the Log's new-line count after a filter change; closing the find dropping its day summaries) appear to fix the filter read-back (#140) and day heads (#138) introduced in this same release.
- **New-folder signal:** none (21 new files in existing packages); rollup "internal: 10198, testdata: 96" (root `testdata/vhs` leaked again). Largest file `internal/ledger/sessionfrom.go` (775) — main work.
- **Headline:** "The sessions list, session pages and the Log's earlier days now open fast, no longer replaying whole days of requests."

### v0.1.5 (prev v0.1.4) — 87 files, +8105 −850
- **Ground truth:** one main body — "stage 3", the data verbs: `sessions`, `events`, history's blocks, the status document's fields and JSON output off a terminal, on a new `internal/views` package (#89–#93, #95). Supporting: router probing around the clock (#94), summaries covering every account (#96).
- **Led with it?** Partly — `sessions` is ✨ Added 1 of 8, but the unifying point (all of it readable by agents) buried; JSON-by-default at 🔧 Changed 1.
- **Order within sections:** `events` (#91, 985 lines) above the `history` totals (#93, 3030, the largest PR); two README-only bullets (5th, 6th) above router probing (#94, a real behaviour change) and the status document (#90, 696); "Day summaries now cover every account" (#96, 1422) 4th of 6 in Changed.
- **Collapsed or split?** ~7 bullets.
- **Misfiled / missing / misrepresented:** duplicates of the previous release — "Set `plan`… and `[prices]`" and "`week_starts` config key" were already in v0.1.4's notes and the config code shipped in v0.1.4; in v0.1.5 only the README documented them (#95), and the AI read the new README text as new features. Piped output switching from text to JSON is breaking (the commit body says so) but unflagged. Router probing under Added rather than Changed.
- **New-folder signal:** `internal/views` (18 files, 3573 lines) invisible; rollup "internal 8830, README.md 125". Largest file `internal/cli/historyblocks.go` (435) — main work.
- **Headline:** "Everything the router records can now be read from the command line, as text for you or JSON for an agent."

### v0.1.2 (prev v0.1.1) — 48 files, +3388 −430
- **Ground truth:** one main body — the request ledger read back and priced, through `requests` and `history` (#78) and day summaries (#77). Retention changes (#80) secondary. Two review-fix PRs (#79, #82) mostly fixed code not yet released.
- **Led with it?** Yes, ✨ Added 1–3 of 6.
- **Order within sections:** no meaningful inversions.
- **Collapsed or split?** Four bullets (`requests`, `history`, pricing, summaries) — reasonable; pre-release review fixes correctly left out.
- **Misfiled:** "Day boundaries follow the local clocks…" in 🔧 Changed is a fix to the shipped dashboard (midnights an hour early where clocks go forward at midnight, #82); the token-redaction fix arguably 🔒 Security.
- **New-folder signal:** `internal/readings` (123 lines) invisible. Largest file `internal/ledger/summary.go` (543), a supporting piece shown as Added 4.
- **Headline:** "The request ledger can now be read back, every request listed and each day's use priced at API rates."

### Project summary
- Main work led in 4 of 4; this repo's failures are scatter and phrasing — main work split into 4–10 bullets across sections (v0.2.6 spreads one performance project over Added, Changed, Fixed), and its first bullet sometimes mechanism rather than benefit (v0.2.6). A headline applies to all four; none a grab-bag or only-the-main-thing.
- Inversions mild but in 3 of 4 (v0.1.0 themes; v0.1.5 events above history, README-only items above real behaviour; v0.2.6 the Runway tweak). Likely causes: new commands and visible novelty ranking above size; README prose weighted as content.
- The new-folder signal is blind in a Go `internal/` layout — never flagged `internal/capture`, `internal/theme`, `internal/views`, `internal/readings`. The only time it fired (v0.1.0) it pointed at dev-only demo assets, one meant to be excluded. The area rollup collapses to "internal: ~97–99%". Only "largest file" carried signal, on the main work in 4 of 4.
- Beyond ordering: (1) docs read as features — README additions became duplicate Added bullets for features shipped the release before (v0.1.5); (2) implementation detail and performance work filed as Added (v0.2.6); (3) fixes to code new in the same release listed as user fixes (v0.2.6, likely); (4) dev tooling as a feature (v0.1.0 `capturetool`); (5) breaking default changes unflagged (v0.1.5 JSON when piped, v0.1.0 reserves); (6) exclusion leak — `**/testdata/**` misses a repo-root `testdata/` (v0.1.0 45% of the prompt; v0.2.6 too) — a mint pathspec bug: use `:(exclude,glob)`.
- The largest releases never reach the AI: v0.2.0 and v0.2.4 exceeded the size cap and shipped hand-written fallbacks.
- Hand-written evidence: Lee's v0.2.0 notes open with a headline ("The dashboard, redesigned to be read at a glance: six tabs, …"), but it inventories the pieces, which the design's rules forbid. His v0.2.4 notes, a grab-bag of dashboard tweaks, have none — matching the no-headline rule.

---

## agntc and mint — release-notes audit (6 releases)

All sizes post-exclusion, with the `diff_exclude` in force at each tag plus CHANGELOG.md and .mint.toml.

**Skipped:** agntc v0.5.0 (172 files, +24359 −20134), the largest — not AI-written: the v2 PR (e4fea393) hand-wrote an `[Unreleased]` section, b6c5d967 dropped it from CHANGELOG.md, and the v0.5.0 tag body is that section verbatim, `**Breaking:**` lead paragraph included.

*Auditor's note on the new-folder signal (collator):* this auditor judged "new folder" at package level (`src/terminal/`, `internal/setupguide`). mint's Change Map flags only brand-new top-level directories (`leadingDir`, first path segment), so none of those would have been flagged by the map itself; where the notes promoted a new package, the prompt's own wording ("new structure (a whole new package or directory) is the strongest signal") read against the diff is the likelier driver.

### agntc v0.6.0 (prev v0.5.3) — 70 files, +9305 −2424
- **Ground truth:** one main body — agntc's own terminal UI (PRs #8, #10, #16); the README rewrite with screenshots (#11, #12) secondary.
- **Led with it?** Yes, ✨ Added 1 of 7 ("A redesigned terminal UI across every command…").
- **Order within sections:** mild inversion — Added 2, a UI detail ("Spinners and questions sit under the row…"), above the `list` dashboard (list-render.ts +311, a rewrite of list.ts).
- **Collapsed or split?** Heavily split: 12 of 13 bullets across Added (6), Changed (5), Removed (1).
- **Misfiled / missing / misrepresented:** the `list` dashboard and detail view are redesigns of existing views filed under Added; the README rewrite under Added; the dependency swap in Removed is internal.
- **New-folder signal (package level):** `src/terminal/`, `src/terminal/prompts/`; largest file `src/terminal/block.ts` (+862) — on the main work.
- **Headline:** "Every agntc command now runs through its own redesigned terminal UI." Borderline only-the-main-thing: Added 1 already does the headline's job.

### agntc v0.4.0 (prev v0.3.3) — 20 files, +7597 −1363
- **Ground truth:** only the main thing — the "update-output-overhaul" spec (clone dedup, streamed progress, tag wording, gating message); 142 commits, all this unit.
- **Led with it?** Yes — Added 1 (clone once per group), the spec's "structural pivot".
- **Order within sections:** no inversions.
- **Collapsed or split?** Seven bullets over Added, Changed, Fixed.
- **Misfiled / missing / misrepresented:** streamed progress and the re-add directive are changes to existing output (the spec calls the directive a "reword") filed as Added. Changed 3 ("A group can split…") is not a change — per-member status preserved under grouping. Fixed 1 ("stale pre-update version") never shipped — the old footer printed no current version (`v0.3.3:src/summary.ts:302`); a design decision in the new footer dressed as a fix.
- **New-folder signal:** none; largest file `tests/commands/update.test.ts` (+3770 −956; tests not excluded then) — neutral.
- **Headline:** none (only the main thing) — the rule bites hardest here, where the work spreads over three sections.

### agntc v0.5.2 (prev v0.5.1) — 64 files, +4099 −560
- **Ground truth:** one main body — packs' support folders, which 0.5.0 and 0.5.1 dropped ("0.4 installed 26 folders, 0.5.1 installs 23"), with an `update` safety net; secondary strand: git retries and one-line errors.
- **Led with it?** Yes, Added 1.
- **Order within sections:** Changed 1 a trivial support-folder follow-on (`--skill` names the folders it leaves behind); remote-read retries plus the SSH "Permission denied" retry at Changed 7 of 8, below the cosmetic "name the member once instead of repeating its key" (Changed 4).
- **Collapsed or split?** Across Added 1–3, Changed 1–2, Fixed 1 and 3.
- **Misfiled / missing / misrepresented:** support folders are a regression fix presented as a new feature; the `list` detail count ("5 skills" for 2 skills and 3 folders) a fix under Added. Missing: `update` brings back the folders 0.5.x installs lost (a test asserts exactly this).
- **New-folder signal:** none; new file `src/dropped-folders.ts`; largest file `tests/integration/add-and-update.test.ts` (all support-folder tests) — on the main work.
- **Headline:** "Packs install their support folders again, and `update` stops rather than delete folders a repo still holds."

### mint v0.0.6 (prev v0.0.5) — 21 files, +3074 −627
- **Ground truth:** one main body — `mint setup`'s guide, with `mint init` cut to a minimal pointer file (interactive-mint-init-setup, 123 commits).
- **Led with it?** Yes, Added 1.
- **Order within sections:** Added 2–4 are internals; the user-facing `mint init` change at Changed 1.
- **Collapsed or split?** Added 1, Changed 1, Changed 2 (README).
- **Misfiled / missing / misrepresented:** 4 of 7 bullets dev-only — the `MetadataRows()` source of truth, the `internal/setupguide` package, the `internal/configtest` test seam, the exported `DefaultMaxDiffLines`.
- **New-folder signal (package level):** `internal/setupguide`, `internal/configtest`; largest file `setupguide_test.go` (+591). The AI turned each new package into its own feature bullet — evidence of the "new structure is the strongest signal" rule promoting internals.
- **Headline:** none once internals are dropped (only the main thing remains).

### mint v0.0.2 (prev v0.0.1) — 59 files, +2135 −1528 (no `diff_exclude` in force)
- **Ground truth:** two co-equal bodies — the presenter redesign (hotkey bar, flush-left layout, single-keypress gates; `pretty.go` +321 −165) and dry run as a pure preview plus the shared notes cache it feeds (~9 commits).
- **Led with it?** No — Added 1 is the version-confirm gate; "Pretty output was redesigned" at Changed 4.
- **Order within sections:** Changed 1, a prompt tweak (`prompt.go` +58 −53), above the dry-run preview (Changed 2) and the output redesign (Changed 4).
- **Collapsed or split?** Both split: redesign over Added 2, Changed 3, 4, 6; dry run over Added 3, Changed 2 and 5, Fixed 2.
- **Missing:** "second Ctrl-C terminates immediately" (fixed an unkillable gate).
- **New-folder signal:** none; the new top-level `release` wrapper (6 lines) correctly ignored; largest file `pretty.go` — the notes ranked the redesign 4th anyway.
- **Headline:** "Dry runs are now a true preview whose notes the real run can reuse, and release output is redesigned around single-keypress gates."

### mint v0.0.3 (prev v0.0.2) — 26 files, +3334 −229
- **Ground truth:** only the main thing — AI model selection: per-verb `ai_command` and `timeout`, a pinned Sonnet default, a configurable deadline (100 of ~105 commits).
- **Led with it?** Yes, Added 1.
- **Order within sections:** no real inversions; the pinned default (what every zero-config user gets) at Changed 1.
- **Collapsed or split?** Five bullets with duplicates — `timeout = 0` in both Added 2 and Changed 3; "configurable" twice.
- **Misfiled / missing / misrepresented:** Changed 2's "a timeout is fatal and never retried" was already true in v0.0.2 (`transport.go:108`) — the bullet comes from new README text. Changed 4 mentions mint's own `.mint.toml` excludes (dev-only; only CHANGELOG.md was auto-excluded then).
- **New-folder signal (package level):** `internal/aitransport` (45 lines of plumbing); largest file `config_test.go` (+1346) — plumbing; the notes rightly ignored it.
- **Headline:** none (only the main thing).

### Summary
- The main work led in 5 of 6 (exception mint v0.0.2: a small gate led, the largest change at Changed 4). "Lost" is mostly not the first bullet; it is **fragmentation** in all 6 (3 to 12 bullets across Added, Changed, Fixed; no single line portrays it) and **dilution** (internal and minor bullets around the main work).
- Inversions in 3 of 6 (v0.6.0 mild, v0.5.2, v0.0.2), following neither size nor the new-folder signal; in v0.5.2 the main theme's crumbs come first in a section and push a secondary theme's significant change (retries) to the bottom.
- New-folder signal (package level) hit the main work in v0.6.0 and v0.0.6, pointed at plumbing in v0.0.3, absent in three. Its real damage: v0.0.6, where each new internal package became a feature bullet.
- Headline applies in 3 of 6 (v0.6.0 borderline, v0.5.2, mint v0.0.2 two co-equal); ruled out in 3 (v0.4.0, v0.0.3, v0.0.6 — all only-the-main-thing). That rule suppresses the headline exactly where the main work is most fragmented; a sharper test: does the first bullet already name the whole body of work?
- Failure classes beyond ordering: (1) section misfiling — regression fixes as Added (v0.5.2), redesigns of existing views as Added (v0.6.0, v0.4.0), docs as Added (v0.6.0); (2) internals presented as features (v0.0.6 ×4, v0.0.3, v0.6.0); (3) unchanged behaviour or never-shipped bugs presented as changes or fixes (v0.4.0 Fixed and Changed 3, v0.0.3 Changed 2) — from the diff alone the AI cannot tell newly built behaviour from newly documented behaviour or spec-style comments; (4) missing items (v0.0.2 Ctrl-C fix; v0.5.2 restoring what 0.5.x lost — regression context invisible without history); (5) duplicate bullets (v0.0.3).

---

## tick, stitch-cli, agentic-skills — release-notes audit (6 releases)

Only two of six clearly open with the main work; five of six split it into many bullets.

**Skipped:** tick v0.2.8 (largest, 59,642 lines) — tag body only "🔖 Release v0.2.8", no notes, no `.mint.toml` at that tag. Took v0.3.0.

### tick v0.3.0 (prev v0.2.9) — 59 files, +12,408 −2,108
- **Ground truth:** one main body — 446 of 454 commits are free-text-round-trip: every TOON document rebuilt so a standard TOON reader parses it; free text survives read-edit-write (`--field`, `--`, migrate trim, note index). Otherwise a README expansion.
- **Led with it?** No. ✨ Added 1–2 are pieces (`--`, `show --field`); the format rewrite at 🔧 Changed 3 of 9; no bullet says TOON output now parses.
- **Order:** Changed 1 (the false `--ready`/`--blocked` claim) above the TOON rewrite; Added 1 (the `--` parsing fix) above `show --field`, a new 298-line command.
- **Collapsed or split?** ~14 bullets across three sections; the `changed` table stated twice (Added 3, Changed 2). Folded away entirely: the description is now an escaped, quoted TOON string instead of an indented block — the change that breaks agent parsers; no bullet mentions it.
- **Misrepresented:** "`--ready`/`--blocked` now also match `in_progress` tasks" is false — only README text changed; the query already said `t.status IN ('open','in_progress')` at v0.2.9 (`query_helpers.go:53`).
- **New-folder signal:** none (everything under existing `internal/`). Largest file `internal/cli/conformance_test.go` (1,896) — on theme but a test file; unused by the notes.
- **Headline:** "Tick's TOON output now reads with any standard TOON parser, and free text read from `tick show` writes back unchanged."
- **Other:** the notes hold nothing but the main thing, but only in pieces — the exemption must mean some bullet already states the whole.

### tick v0.3.2 (prev v0.3.1) — 46 files, +3,166 −280
- **Ground truth:** one main body — the bugfix "oversized text field bricks the store": a JSONL line over 64 KiB made every command fail. Fix: a shared line reader with no length limit, plus doctor consistency, a description cap, safer rebuilds.
- **Led with it?** No — ✨ Added 1 "Descriptions are now capped at 50,000 characters" is the guard, not the fix; the fix at 🔧 Changed 1 and again at 🐛 Fixed 1.
- **Order:** four doctor bullets before "Parse errors now name the failing line" (shown by every command); one of the four ("reads `tasks.jsonl` once, shares the result across all checks") internal.
- **Collapsed or split?** ~11 bullets; core fix stated twice ("Task lines of any length now load…" / "Store operations no longer break once a task's line grows past 64 KiB").
- **Misfiled:** the fix under Changed; an internal doctor refactor presented as user-facing.
- **New-folder signal:** none — `internal/jsonl/` is new but nested, unseen by the map; the only new top-level entry `.worktreeinclude` (dev-only). Largest file `internal/cli/doctor_line_reading_test.go` (275) points at doctor, a secondary piece the notes over-weight.
- **Headline:** "One task with a very large description can no longer lock every tick command out of the store."

### stitch-cli v1.6.2 (prev v1.6.1) — 6 files, +48 −358 (largest in the window)
- **Ground truth:** the only user-facing item is the `ai.model` setting (~40 lines); the rest a maintainer chore — the repo's own `release` script replaced by mint (355 deletions).
- **Led with it?** Yes, ✨ Added 1 of 1, accurate.
- **Misrepresented:** the 🔧 Changed bullet describes the repo's own release tooling — dev-only, presented to stitch users.
- **New-folder signal:** none. Largest file `release` (361 lines, dev-only) — the size signal pointed the wrong way; fixed section order kept the feature on top.
- **Headline:** none — the only user item is one Added bullet.
- **Other:** CHANGELOG.md no longer matches the tag body (regenerated): splits the tooling item in two and adds a maintainer-aimed `brew install` instruction.

### stitch-cli v1.6.6 (prev v1.6.5) — 7 files, +206 −74
- **Ground truth:** one squashed PR "fix: stop CI builds failing on the commit hook and existing ECR tags" — two co-equal fixes for one goal: the commit-msg hook (POSIX sh, exits under `$CI`, lets the commit through with no terminal) and trunk build templates (skip versions already in ECR, `--no-verify` on tracking commits).
- **Led with it?** Partly — 🔧 Changed 1 is the POSIX-sh hook framed as a change; the CI-builds-were-failing story never told.
- **Order:** sections emitted out of the fixed order — 🔧 Changed before ✨ Added.
- **Collapsed or split?** Five bullets across three sections for one PR.
- **Misfiled:** both the POSIX-sh rewrite and the ECR skip are fixes (the commit body says so), filed under Changed and Added. "Bumped to template version 1.0.6" is bookkeeping the prompt says to ignore.
- **New-folder signal:** none. Largest file `lib/common/hooks.sh` (116) — a fair pointer.
- **Headline:** "CI trunk builds no longer fail on the commit-msg hook or on a version already pushed to ECR."

### agentic-skills v0.2.0 (prev v0.1.4) — 127 files, +9,662 −13,529
- **Ground truth:** two co-equal bodies — a new Go plugin (8 skills, 5,744 lines, 1 PR) and a rewrite of all 20 Laravel skills (16,927 lines changed, 5 PRs).
- **Led with it?** Go plugin at ✨ Added 1; the Laravel rewrite at 🔧 Changed 1 (sixth bullet) — three Laravel sub-items before the rewrite itself.
- **Order:** Added 2 (Go quality references, a piece of Added 1) and Added 3–5 (tenancy harness, packages skill, reference files — pieces of the rewrite) above the rewrite.
- **Collapsed or split?** The Laravel rewrite over ~8 bullets across Added, Changed, Removed.
- **Misfiled:** rewrite pieces as Added; "Releases now run through mint" and removing the "GitHub Actions release workflow and committed local Claude settings file" are dev-only.
- **New-folder signal:** `go/` new at top level — points at the Go plugin, the smaller body by ~3×, and promoted it and its sub-piece. Largest file `laravel/skills/laravel-validation/references/validation-testing.md` (1,443 lines, mostly deletion).
- **Headline:** "Go projects get their own skills plugin, and all 20 Laravel skills are rewritten around current conventions."

### agentic-skills v0.2.2 (prev v0.2.1) — 12 files, +446 −9
- **Ground truth:** two co-equal pieces in one PR "Add placeholder event-sourcing and workflow skills" — two new Laravel skills plus cross-links.
- **Led with it?** Yes, ✨ Added 1.
- **Order:** Added 2 (event-sourcing's three PHP reference files) above Added 3 (the workflows skill, co-equal with event sourcing).
- **Collapsed or split?** Event sourcing two bullets, workflows one.
- **Misrepresented:** both skills say they "hold only the decisions made so far" (the PR calls them placeholders); the notes present them as complete.
- **New-folder signal:** none at top level — `laravel-event-sourcing/`, `laravel-workflows/` new but nested. Largest file event-sourcing `SKILL.md` (177).
- **Headline:** "Laravel projects get first skills for event-sourced models and long-running Durable Workflow processes." Borderline.

### Summary
- Leading: clearly led 2 of 6 (stitch v1.6.2, agentic v0.2.2); partly 2 (v1.6.6 right piece wrong framing; v0.2.0 only the Go half); not 2 (both tick — a piece of the work in v0.3.0, a secondary guard in v0.3.2).
- Inversions in 4 of 6 (tick v0.3.0, v0.3.2, agentic v0.2.0, v0.2.2). Causes: splitting the main work (pieces scatter; a piece outranks the whole or its co-equal); the fixed section order (rewrite pieces under Added land above the rewrite in Changed; a guard above the fix); file-level novelty (a new flag or file outranks the format rewrite).
- Splitting in 5 of 6; stated twice in two (v0.3.2, v0.3.0).
- New-folder signal fires only for a new top-level directory (`leadingDir`, first path segment, June and current code alike): silent in 5 of 6; cannot fire for new Go packages under `internal/` or new skills under `laravel/skills/`. The once it fired (v0.2.0) it promoted the smaller of two co-equal bodies. Largest-file signal pointed at test files (both tick), a mostly-deleted doc (v0.2.0), a dev-only script (v1.6.2).
- Headline applies 5 of 6 (exception v1.6.2: one user bullet). v0.3.0 shows the only-the-main-thing exemption needs wording: only when one bullet already states the whole.
- Beyond ordering: a false bullet invented from a README edit (tick v0.3.0); a missing breaking change (TOON description format, v0.3.0); fixes misfiled, and the fixed section order broken (v1.6.6); dev-only items as user changes (v1.6.2, v0.2.0, v0.3.2); a version bump as a bullet (v1.6.6); placeholders oversold (v0.2.2); CHANGELOG drift from the tag body (stitch v1.6.2, v1.6.3).

---

