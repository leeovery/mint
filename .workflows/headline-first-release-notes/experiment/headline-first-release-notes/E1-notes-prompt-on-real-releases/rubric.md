# E1 Judge Rubric

You are judging AI-written release notes for real releases of a project. For each release you are given several sets of notes, labelled A, B, C, D. They were written by different setups whose identity you are not told; do not try to work it out. Judge every set by the same standard, against what the release really was.

You are read-only: never modify any repository or file. Read only what this rubric and your brief point you at.

## Inputs, per release

- **The release** — project, repo path, the previous tag and this tag.
- **The ground truth** — the audit's account of the release in `.workflows/headline-first-release-notes/imports/release-notes-audit.md` (your brief names the section). It says what the release's main work was, what else shipped, and the defects found in the notes that shipped at the time. Those defects belong to the old shipped notes, not to the sets you judge — use them as a list of things to check, never as findings.
- **What the AI was given** — the input file your brief names: the Change Map then the diff between the two tags, after the release's own exclusions. This is everything the AI saw apart from its instructions.
- **The pre-registered expectations** — whether a headline is expected (below) and the release's breaking change, if any.
- **The notes** — one file per label.

Before scoring, settle your own reference list for the release from the ground truth, checked against the repo where you need to (`git -C <repo> log`, `git -C <repo> diff <prev>..<tag> -- <path>`, `git -C <repo> show <tag>:<path>`): the main body (or two co-equal bodies) of work; the notable user-facing changes in rough order of weight to a user; the internal or dev-only changes; the breaking change. Use the same reference for every label.

## What each set of notes looks like

An optional headline — one plain sentence above the first section — then sections with emoji headers (✨ Added, 🔧 Changed, ⚠️ Deprecated, 🗑️ Removed, 🐛 Fixed, 🔒 Security; possibly a 💥 Breaking section first), with one-line bullets. Some sets have no headline. A single plain one-sentence line above the first section is a headline, scored under **Headline**; anything else above the first section is a format slip.

## Score, per set of notes

**Lead** — does the first line (the headline if there is one, else the first bullet) state the release's main work?
- `clean` — it names the main body of work as a whole (both bodies, where there are two co-equal ones).
- `partial` — it names a piece of the main work, or one of two co-equal bodies, or the right thing framed so a user would miss what it is.
- `no` — it names a side item, a dev-only change, a guard instead of the fix, or anything that is not the main work.

**Headline** — present or not, its text, and its verdict against the release's expectation:
- `expected` — present and naming the main work (both bodies where two) → `match`; absent → `omitted`; present but naming only one of two co-equal bodies → `incomplete`; present but naming as the release's main thing something that is not (a side piece, a dev tool, a guard, a detail) → `wrong-subject`.
- `expected-unless-one-bullet` — if the set's sections hold exactly one bullet and it states the whole release, then absent → `match` and present → `unexpected`; otherwise score as `expected`.
- `not-expected` — absent → `match`; present → `unexpected` (or `wrong-subject` if it names the wrong thing).
- `either` — absent → `match`; present and right → `match`; present and naming one of two → `incomplete`; present and wrong → `wrong-subject`.

Also list the headline's shape slips, if any: `multi-sentence`, `scale-word` ("major", "overhaul", "powerful", "significant" and the like), `inventory` (lists more than two of the work's pieces), `bold`, `label` (a prefix such as "Highlights:").

**Inversions** — within each section, a bullet placed above a bullet in the same section that clearly matters more to a user. List each out-of-place bullet once. "Clearly" means a reader who knows the release would not hesitate; close calls are not inversions.

**Misfiled** — a bullet in the wrong section, judged against the PREVIOUS release: something new in this release under Changed or Fixed; a rework of existing behaviour under Added; a fix of something broken in the previous release under Added or Changed; a regression fix presented as a new feature; a breaking change not where the set's own scheme puts breaking changes.

**Duplicates** — the same change stated twice across the sections (overlap between the headline and the sections is by design and never a duplicate). List each extra statement once.

**Internal** — a bullet about something a user of the software never meets: CI, lint, release tooling, build scripts, test suites or harnesses, internal packages or refactors, internal flags or plumbing commands, migration numbering, contributor docs, repo housekeeping such as a README restructure with no product change. Where the files ARE the product (prompts, skills, templates the software ships), changes to them are not internal.

**False claims** — a bullet claiming behaviour the release's code does not change: invented from a documentation, README, comment or test edit; behaviour that was already true in the previous release; a feature shipped in an earlier release presented as new; a fix for something that never shipped broken.

**In-release as change** — a bullet presenting work built inside this release as a change to, or fix of, the previous release (a cap, adjustment or fix to something that did not exist before this release).

**Breaking** — for a release with a pre-registered breaking change: `flagged` (the change is stated and a user would know existing use breaks — marked as breaking, or worded plainly so), `unflagged` (stated but not as breaking), `absent` (not mentioned). For other releases `n/a`.

**Missing** — notable user-facing changes in your reference list that the set leaves out entirely. Do not count internal items.

**Format slips** — bullets of more than one sentence, bold lead-ins, sub-bullets, text above the first section other than a one-sentence headline, commentary or sign-off, code fences, empty sections, sections out of the order listed above (a breaking section may stand first). One entry per slip.

**Overall** — 1 to 5: how well the set meets the bar of notes that are good, readable, coherent, useful, accurate and well ordered, for someone who uses this software. Score each set on its own, then check the scores across the release's sets are consistent with each other.

**Input present** — for every failure you record (each entry in the lists above; a lead that is not `clean`; a headline verdict that is not `match`; a breaking change not `flagged`) say whether the facts needed to avoid it were in what the AI was given (the input file): `true` if a careful reader of that input could have got it right, `false` if getting it right needs something the input does not hold — the project's history before the previous tag, what earlier releases shipped, intent stated only in commit messages, issues or planning documents excluded from the diff. When `false`, name the missing fact in a few words.

## Return format

Return ONLY a JSON array, one object per set of notes, nothing before or after it:

```json
[
  {
    "release": "portal-v0.11.1",
    "label": "A",
    "lead": "clean",
    "lead_input_present": null,
    "headline": {"present": true, "text": "…", "verdict": "match", "input_present": null, "shape_slips": []},
    "inversions": [{"text": "bullet text", "outranks_by": "bullet it sits above that matters more", "input_present": true, "missing_fact": null}],
    "misfiled": [{"text": "…", "section": "Added", "should_be": "Fixed", "input_present": true, "missing_fact": null}],
    "duplicates": [{"text": "…", "duplicates": "…", "input_present": true, "missing_fact": null}],
    "internal": [{"text": "…", "why": "…", "input_present": true, "missing_fact": null}],
    "false_claims": [{"text": "…", "why": "…", "input_present": true, "missing_fact": null}],
    "in_release_as_change": [{"text": "…", "why": "…", "input_present": false, "missing_fact": "…"}],
    "breaking": "n/a",
    "breaking_input_present": null,
    "missing": [{"text": "…", "input_present": true, "missing_fact": null}],
    "format_slips": [{"text": "…", "kind": "multi-sentence"}],
    "overall": 4,
    "notes": "one or two sentences on what most shaped the overall score"
  }
]
```

`release` is the `<project>-<tag>` stem your brief gives. `lead_input_present`, `headline.input_present` and `breaking_input_present` are `null` when there is no failure to classify.
