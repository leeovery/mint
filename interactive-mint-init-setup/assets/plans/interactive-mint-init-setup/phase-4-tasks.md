---
phase: 4
phase_name: README reconciliation + entry-point + tripwire
total: 3
---

## interactive-mint-init-setup-4-1 | approved

### Task 4-1: Reconcile the Configuration + Commands sections with the minimal template

**Problem**: Phase 3 changed `initgen.MintTOML()` to emit a minimal `.mint.toml` (empty body + a header comment pointing to the GitHub docs and `mint setup`). The README still describes the old behaviour in three places, which are now factually false: the `## Configuration` intro states *"`mint init` writes a commented `.mint.toml` at the repo root"*, the section embeds a full commented-template TOML block (the pre-pivot example file with every key shown active or commented), and the Commands `### init` block says init *"writes a commented `.mint.toml` (every key shown at its default, optional keys commented with one-line explanations)"*. A reader following the README would expect a commented scaffold that the binary no longer produces, and the embedded block duplicates exactly the drift surface the pivot removed.

**Solution**: Edit the README so the Configuration intro and the Commands `### init` line both describe the minimal `.mint.toml` (empty body + a header comment pointing to the GitHub docs and `mint setup`), and either replace the embedded full commented-template TOML block with the new minimal template or drop it entirely. Then run a confirmation pass over the per-key reference tables (`Shared engine keys`, `[release]`, `[release.hooks]`, `[commit]`) to confirm they remain the authoritative human config reference — declaring every config key and its default. This is documentation editing only; there is no compiler for it (verification is the manual-narrative acceptance plus the Task 4-3 tripwire).

**Outcome**: The README's Configuration and Commands sections truthfully describe the shipped minimal-template behaviour, the embedded false template block is gone (replaced or dropped), and the per-key reference tables are confirmed to be the complete human config reference — every schema key present with its default.

**Do**:
- In `/Users/leeovery/Code/mint/README.md`, locate the `## Configuration` section (intro currently around line 176; line numbers drift, find it by the text *"`mint init` writes a commented `.mint.toml`"*). Rewrite the intro so it states `mint init` now writes a **minimal** `.mint.toml` — an **empty body plus a short header comment** that points to the GitHub docs (the human config reference) and to `mint setup` (AI-assisted setup) — and that the file is fully optional because every key has a compiled default.
- Handle the embedded full commented-template TOML block (currently around lines 178-213, the fenced ```toml block listing `ai_command`, `[release]`, `[release.hooks]`, `[commit]` keys). Choose ONE (both are spec-valid — see Context): (a) **replace** it with the new minimal template — the empty-body file plus the dual-pointer header comment, matching what `initgen.MintTOML()` now emits; or (b) **drop** the fenced block entirely. Either way the per-key reference tables immediately below become the authoritative human config reference. If you replace it, the shown header must carry BOTH pointers (GitHub docs + `mint setup`) so the README example matches the binary's actual output.
- In the Commands `### init` block (currently around line 62, the line *"writes a commented `.mint.toml` (every key shown at its default, optional keys commented with one-line explanations)"*), rewrite the description to match: `mint init` writes a **minimal** `.mint.toml` (empty body + header pointer to the GitHub docs and `mint setup`) and the `release` shim, idempotent / skipped-unless-`--force`. Keep the shim and idempotency framing — only the `.mint.toml` description changes.
- Confirmation pass (NOT a rewrite): read the per-key reference tables — `### Shared engine keys` (currently ~219-224), `### [release]` (~228-243), `### [release.hooks]` (~247-251), `### [commit]` (~255-260) — and confirm every config key from the schema appears with its default in the representation convention already used (blank for empty-string defaults, `auto` for sentinel-auto, `[]` for empty collection, `shared` for per-verb inherit, hooks rows describe when each runs). The designer confirmed these tables already cover every key — this step only catches a genuine omission. If (and only if) a key or its default is found missing, add the row; do not restructure tables that are already complete.

**Acceptance Criteria**:
- [ ] The `## Configuration` intro states `mint init` writes a minimal `.mint.toml` — empty body plus a header comment pointing to the GitHub docs and `mint setup` — and no longer contains the phrase "commented `.mint.toml`".
- [ ] The embedded full commented-template TOML block is either replaced with the minimal template (empty body + dual-pointer header) or removed; no fenced TOML block in the Configuration section shows the old commented-with-every-key scaffold.
- [ ] If the block is replaced rather than dropped, the shown header comment carries both pointers (GitHub docs + `mint setup`), matching what `initgen.MintTOML()` emits.
- [ ] The Commands `### init` block describes a minimal `.mint.toml` (empty body + header pointer) and no longer says "commented `.mint.toml` (every key shown at its default, optional keys commented…)".
- [ ] The Configuration intro and the Commands `### init` line AGREE on the framing — both say "minimal `.mint.toml` (empty body + header pointer)".
- [ ] The per-key reference tables (`Shared engine keys`, `[release]`, `[release.hooks]`, `[commit]`) are confirmed to declare every config key and its default; any genuinely missing row is added, otherwise tables are left intact.
- [ ] The README still builds as valid Markdown (no broken fences, no dangling anchor links).

**Tests**:
- This task has no Go test (it is README prose editing with no compiler). Verification is twofold: (1) the manual-narrative acceptance — the README is read end-to-end to confirm the Configuration and Commands sections truthfully describe the shipped minimal template and route to `mint setup`; and (2) the Task 4-3 tripwire test, which asserts every schema key name still appears somewhere in the README after the edits (it lands last, against this reconciled README).
- `"the Configuration intro names the minimal template and both pointers, not a commented file"` (manual read).
- `"the embedded false commented-template block is gone (replaced with the minimal template or dropped)"` (manual read).
- `"the Commands init line agrees with the Configuration intro on the minimal-template framing"` (manual read).
- `"every config key + default still appears across the per-key tables"` (confirmed by the Task 4-3 tripwire once it lands).

**Edge Cases**:
- The embedded-block decision is a genuine fork: replace OR drop are both spec-valid. If you replace, the example must match the binary byte-for-intent (empty body + both header pointers) — a stale half-template (some keys still shown) would reintroduce exactly the drift the pivot removed, so do not leave a partial block.
- The per-key tables are the tripwire surface AND the authoritative human reference: the confirmation pass must NOT delete rows or collapse the dual-level `ai_command`/`timeout` rows (the README intentionally shows them at the shared level and per-verb under `[release]`/`[commit]`) — removing a key's mention would later fail the Task 4-3 tripwire.
- README descriptions are allowed to lightly duplicate the SoT — this is accepted (the README is the human GitHub-browsing surface; the machine surfaces are the ones held to a single SoT). Do not try to thin the tables to avoid duplication.
- Line numbers in the Do steps are approximate and will have drifted — locate each edit by its quoted text, not by line number.

**Context**:
> From the spec ("README — config reference verification"): "Reconcile the existing 'Configuration' section with strip-to-minimal. The README's current `## Configuration` intro (*"`mint init` writes a commented `.mint.toml`"*) and the embedded full commented-template TOML block both become false after the strip. As part of this work: (a) correct the framing — `mint init` now writes a **minimal** `.mint.toml` (empty body + header pointer); (b) **replace** the embedded full-template block with the new minimal template (empty body + header), or drop it — the per-key reference tables already below it (`Shared engine keys` / `[release]` / `[release.hooks]` / `[commit]`) are the authoritative human config reference and the surface the tripwire test checks; (c) correct the Commands-section line (the `mint init … writes a commented .mint.toml …` entry) the same way."
>
> Also: "The README **stays manual narrative** (updated per-feature) and is the **human** config reference surface. As part of this work it is **verified to declare every config key + its default**. … README descriptions may lightly duplicate the SoT — **accepted**: the README is the human GitHub-browsing surface, while the machine/agent surfaces (`mint help` + `mint setup`) are the ones held to a single SoT."
>
> From the generated-config decision: `initgen.MintTOML()` (Phase 3) now returns "empty body + a short header comment" whose header "points to: the **GitHub docs** for the config reference, and **`mint setup`** for AI-assisted setup. This header pointer is also the **recovery net** for the cold-arrival case." The replaced README example (if you replace rather than drop) should reflect that exact shape.
>
> Prose-quality acceptance is a one-time MANUAL run against representative repos (a fresh JS project, a Go project, a repo with an existing release script, a repo with an existing `.mint.toml`) — tracked outside the plan, NOT part of this task.

**Spec Reference**: `/Users/leeovery/Code/mint/.workflows/interactive-mint-init-setup/specification/interactive-mint-init-setup/specification.md` — sections "Generated config: strip to minimal", "README — config reference verification".

## interactive-mint-init-setup-4-2 | approved

### Task 4-2: Add the any-AI entry-point prompt routing operators to mint setup

**Problem**: Post-pivot, the setup experience lives in the binary-emitted `mint setup` guide (Phase 2), not in the README. But the README is where an operator lands first (it is the GitHub-browsing entry point), and right now it has no signpost telling a new user that mint is AI-assisted to set up or that `mint setup` exists. Without an entry-point prompt the operator never discovers the version-matched guide the binary emits.

**Solution**: Add a tiny entry-point prompt to the README that ROUTES operators to `mint setup` — it does not restate the guide (the binary is the version-matched source). Frame it for any AI: "to set up, pass the following prompt to your AI of choice — Claude, Codex, …", with a light steer like "we find Opus-level models do the best work here," and a one-line "what mint is" framing ("mint is an AI tool for commits & releases; run `mint setup` and follow what it prints"). No fidelity-floor machinery — this is a convenience, not defensive engineering.

**Outcome**: A new reader of the README sees a short entry-point prompt that frames mint as an AI-assisted tool, tells them to run `mint setup` and follow what it prints, names that any AI (Claude, Codex, …) can drive it with a light Opus-level steer, and routes to the binary guide without duplicating its contents.

**Do**:
- In `/Users/leeovery/Code/mint/README.md`, add the entry-point prompt. Recommended placement: the Quick Start section (currently ~48-56, the fenced `mint init` / `mint release` / `mint commit` block) is the natural first-contact spot — add a short prose lead-in there, OR add a small dedicated block (e.g. a "Setup" sub-section or a callout) right after Quick Start. Placement is a recommendation, not a hard constraint — pick whichever reads best in the surrounding narrative; do not over-engineer it.
- Write the framing: one line on WHAT mint is in this context (an AI tool for commits & releases) and the routing instruction — run `mint setup` and follow what it prints. Make explicit that the README does NOT reproduce the guide; the binary emits the version-matched source of truth.
- Write the "any AI" framing: phrase it as "to set up, pass the following prompt to your AI of choice — Claude, Codex, …" (or equivalent), with the light steer "we find Opus-level models do the best work here." Keep it light — it is a steer, not a requirement; if the operator picks a weaker AI, that is their call.
- Do NOT add any fidelity-floor machinery, model gate, capability check, or new code/config. The strict-schema loud-fail (`DisallowUnknownFields` at the next `mint` run) and a natural "verify the config loads" step already exist as sensible backstops — mention them at most in passing if it reads naturally; do not introduce them as new defensive engineering.
- Keep the install assumption consistent with the rest of the README: the entry point assumes mint is installed (the Install section is already present) — if `mint setup` is not found, the guide itself tells the agent to ask the user to install; the README need not duplicate that, but the entry point should not imply auto-install.

**Acceptance Criteria**:
- [ ] The README carries an entry-point prompt that names `mint setup` and instructs the operator to run it and follow what it prints.
- [ ] The prompt frames mint as an AI tool for commits & releases and frames setup for "any AI of choice" (naming at least Claude and one other, e.g. Codex).
- [ ] The prompt carries the light Opus-level steer (a recommendation, not a requirement).
- [ ] The README ROUTES to `mint setup` and does NOT restate the guide's contents (no inline copy of the procedure, etiquette, minimalism, or config-reference sections).
- [ ] No fidelity-floor machinery, model gate, or new code/config is introduced by this task.
- [ ] The entry point does not imply auto-install (it assumes mint is installed, consistent with the Install section).
- [ ] The README still builds as valid Markdown (no broken fences or anchors).

**Tests**:
- This task has no Go test (README prose editing, no compiler). Verification is the manual-narrative acceptance — reading the README to confirm the entry point routes to `mint setup`, uses the any-AI/Opus framing, and does not restate the guide. The Task 4-3 tripwire does not check this prose (it only asserts schema key names appear), so this section is verified by manual read.
- `"the README routes a new operator to mint setup and tells them to follow what it prints"` (manual read).
- `"the entry point frames setup for any AI with a light Opus-level steer"` (manual read).
- `"the README does not duplicate the guide's procedure/etiquette/minimalism/config-reference body"` (manual read).
- `"no fidelity-floor machinery or new code is added"` (manual read + the build/gates are unaffected because no code changed).

**Edge Cases**:
- The line between "route to the guide" and "restate the guide" is the crux: keep the entry point to a few lines — what mint is, run `mint setup`, follow what it prints, any-AI + Opus steer. The moment it starts listing the inspect-and-map steps or config keys it has crossed into restating; stop short of that.
- "Light steer, no machinery": resist the pull to add a model-capability gate or a "minimum model" requirement — the spec explicitly rejects fidelity-floor machinery; this is a convenience, and a weak-AI choice is the operator's to make.
- Placement is genuinely flexible (Quick Start lead-in vs a dedicated block) — do not treat the recommendation as a constraint; choose what flows in the surrounding narrative.

**Context**:
> From the spec ("README — entry point"): "The README carries the **tiny entry-point prompt** that points operators at the binary-emitted guide — roughly: *"mint is an AI tool for commits & releases; run `mint setup` and follow what it prints."* The README does **not** restate the guide; it routes to it (the binary is the version-matched source)."
>
> From the spec ("README — 'any AI' framing (light, no machinery)"): "Frame the entry point as: *"to set up, pass the following prompt to your AI of choice — Claude, Codex, …"*, with a light steer like *"we find Opus-level models do the best work here."* There is **no fidelity-floor machinery** — this is a convenience; if the user picks a weak AI, that's their call. The strict-schema loud-fail and a natural 'verify the config loads' step remain as sensible backstops, not defensive engineering."
>
> From "Install handling" / non-goals: "Setup assumes mint is installed; if `mint setup` isn't found the agent asks the user to install — mint does not install itself." So the entry point assumes an installed mint and must not imply auto-install.

**Spec Reference**: `/Users/leeovery/Code/mint/.workflows/interactive-mint-init-setup/specification/interactive-mint-init-setup/specification.md` — sections "README — entry point", "README — 'any AI' framing (light, no machinery)", "Install handling".

## interactive-mint-init-setup-4-3 | approved

### Task 4-3: Add the optional key-presence tripwire test

**Problem**: The README's per-key reference tables are the human config reference surface, but nothing mechanically couples them to the schema. If a future change removes or renames a config key (or adds one) in `internal/config`, the README can silently fall out of sync — the human reference would document a key the binary no longer accepts, or omit a key it now does. The existing Phase 1 SoT drift test guards the SoT↔schema relationship and the machine surfaces, but the human README is currently unguarded.

**Solution**: Add a cheap Go test that asserts every schema key NAME appears somewhere in the README text. The test reads the README from disk, derives the distinct set of schema key names, and fails the build if any name is absent. It dedupes the dual-level `ai_command`/`timeout` to the single distinct key-NAME set (a substring/name-presence check, NOT the (level, key) pairs the Phase 1 drift test uses) — `ai_command` appearing anywhere in the README satisfies the check regardless of how many levels it lives at. A removed/renamed schema key that no longer appears in the README must fail the build, and an added key that the README has not yet documented must also fail.

**Outcome**: A Go test exists (in the `config` test package) that fails the build when any schema key name is missing from the README, locking the human config reference to the actual schema; it passes against the reconciled README from Tasks 4-1 and 4-2.

**Do**:
- Add the test to the `config` test package (`package config_test`) alongside `internal/config/config_test.go` — that package already exists and is where schema-adjacent tests live. (If you instead place it where the Phase 1 SoT lives, keep it in the same package as the SoT for a single key-source — see the key-source flag below.)
- Resolve the README path from the test. The `config` test package is at `internal/config/`, so the README is three directories up: read `../../README.md` relative to the test working directory (Go runs a package's tests with the package directory as the working directory). If a more robust anchor is wanted, use `runtime.Caller` to locate the test file and join `../../README.md` — but the plain relative path is the established-pattern-free simplest option and is acceptable. Read it with `os.ReadFile`; fail the test loudly (with the attempted path) if the README cannot be read, so a moved README surfaces as a clear failure rather than a vacuous pass.
- Derive the distinct schema key-NAME set. FLAG FOR THE IMPLEMENTER (do not silently choose): there are two candidate sources — (1) the `config` decode-shape struct `toml` tags (`fileShape`, `releaseShape`, `commitShape`, `hooksShape` in `internal/config/config.go`, the same structs the Phase 1 drift test reflects over), or (2) the Phase 1 `config.MetadataRows()` SoT (the table the drift test proves bijective with the schema). RECOMMENDATION: derive from the SAME authoritative source the Phase 1 SoT/drift test uses (the decode-shape `toml` tags via the same reflection helper, or `MetadataRows()` — whichever the drift test treats as canonical) so the tripwire, the SoT, and the schema stay one chain. TRADEOFF: tags-direct is the rawest schema truth but duplicates the reflection logic; `MetadataRows()` is one hop removed but is already drift-pinned to the schema so any divergence is already caught upstream — pick `MetadataRows()` if it exists and is exported, else reflect the tags. Whichever source, collapse to the set of DISTINCT key names (so `ai_command` and `timeout`, which appear at multiple levels, contribute one name each) and EXCLUDE the table-container tags (`release`, `commit`, `hooks`) — those are TOML table headers, not config keys, and the README documents them as section headings, not key rows; including them would make the test assert on table names rather than keys.
- For each distinct key name, assert it appears as a substring in the README text. Report ALL missing names in one failure message (not just the first) so a multi-key divergence is fixed in one pass. Keep the check a plain case-sensitive substring of the snake_case key name (e.g. `ai_command`, `tag_prefix`, `pre_tag`) — the README writes keys in backticks verbatim, so substring presence is sufficient and robust.

**Acceptance Criteria**:
- [ ] A Go test in the `config` test package reads the README from disk and fails loudly (naming the attempted path) if it cannot be read.
- [ ] The test derives the distinct set of schema key names from a single authoritative source, with the key-source choice (decode-shape `toml` tags vs `config.MetadataRows()`) documented in a comment explaining why that source was chosen.
- [ ] The dual-level `ai_command` and `timeout` are deduped to one name each (the check is name-presence/substring, NOT the (level, key) pairs the Phase 1 drift test uses).
- [ ] The table-container tags `release`, `commit`, `hooks` are excluded from the key set (they are TOML table headers, not keys).
- [ ] The test asserts every distinct key name appears as a substring of the README and reports ALL missing names in a single failure message.
- [ ] The test PASSES against the reconciled README (Tasks 4-1 and 4-2 landed first).
- [ ] A removed/renamed schema key with no README mention fails the test; an added schema key not yet in the README fails the test (verified by the two negative tests below).
- [ ] All standard gates pass (`go build`, `gofmt -l`, `go vet`, `go test -race`, `golangci-lint`).

**Tests** (this is a true TDD cycle — one test, write-fail-pass):
- `"it asserts every distinct schema key name appears in the README"` — the primary test: derive the distinct key-name set, assert each is a README substring against the real `/Users/leeovery/Code/mint/README.md`.
- `"it dedupes dual-level ai_command/timeout to one name each"` — the derived set contains `ai_command` and `timeout` once, proving the (level, key)-pair model is collapsed to names.
- `"it excludes the release/commit/hooks table-container tags from the key set"` — `release`, `commit`, `hooks` are not in the asserted name set (so the test does not demand those bare strings).
- `"it fails when a schema key name is absent from the README"` — drive the assertion logic against a synthetic README body missing one known key (e.g. feed the same comparison helper a fixture string lacking `tag_prefix`) and prove it reports that name; this proves the tripwire bites for a removed/renamed key.
- `"it fails loudly when the README cannot be read"` — point the reader at a non-existent path and assert the test fails with a clear message naming the attempted path (not a silent/vacuous pass).

**Edge Cases**:
- Dedupe to distinct NAMES, not (level, key) pairs: this is the deliberate difference from the Phase 1 drift test. The README mentions `ai_command` in multiple tables, but the tripwire only needs the name to appear once. Do not import the Phase 1 pair model here.
- Exclude `release` / `commit` / `hooks`: the recurse-don't-count container tags from Phase 1 are TOML table headers, not config keys — the README documents them as `### [release]` / `### [release.hooks]` / `### [commit]` headings, not key rows. Asserting on those bare strings would be testing table names, not keys.
- README-read failure must be loud: a moved or renamed README must fail the test (naming the attempted path), never produce a vacuous pass where an empty body "contains" no keys and silently passes by reading nothing.
- Substring vs whole-word: a plain substring of the snake_case name is sufficient (README writes keys verbatim in backticks). Beware accidental superset overlap only if a short key name is a substring of a longer one — the current schema names (`ai_command`, `timeout`, `max_diff_lines`, `diff_exclude`, `tag_prefix`, `commit_prefix`, `release_branch`, `publish`, `changelog`, `provider`, `context`, `prompt`, `on_notes_failure`, `fallback`, `version_file`, `version_pattern`, `preflight`, `pre_tag`, `post_release`) have no problematic substring overlap, so plain substring is safe; note this in a comment so a future short-name addition is reviewed.

**Context**:
> From the spec ("README — config reference verification"): "An optional cheap **tripwire test** may be added — assert that every schema key name appears somewhere in the README." And the definition of done: "**README tripwire (optional)** — assert every schema key name appears in the README."
>
> This task is the spec's OPTIONAL tripwire. The implementer MAY skip it if they judge the Task 4-1 manual-narrative + per-key-table verification sufficient — but inclusion is RECOMMENDED: it is cheap and permanently locks the human reference to the schema, catching a removed/renamed/added key that the manual pass could miss. Author and attempt it.
>
> From the spec ("Drift test"), for contrast: the Phase 1 drift test matches on "(level, key) pairs, one SoT row per pair," with `ai_command`/`timeout` as distinct rows per level and the container fields recursed-not-counted. THIS tripwire is deliberately coarser — distinct key NAMES, substring presence — because the README is the human surface and lists each key by name in backticks, not by (level, key) coordinate.
>
> Key-source choice is FLAGGED, not decided — see the Do steps. Recommend deriving from the same authoritative source the Phase 1 SoT/drift test treats as canonical (the decode-shape `toml` tags, or `config.MetadataRows()` if exported) so the tripwire stays in one chain with the schema. AMBIGUITY NOTE: at authoring time the Phase 1 SoT (`config.MetadataRows()`) and its reflection helper are not yet in the tree (Phase 1 is approved but not built) — the implementer must confirm the exact exported name/signature when this task is picked up and choose the source accordingly; do not hardcode an assumed helper name.
>
> The config test package is `package config_test` at `internal/config/` (confirmed in `internal/config/config_test.go`); the module path is `mint`. The README lives at the repo root, `../../README.md` from that package's directory. No existing test reads the README, so this establishes that small pattern.

**Spec Reference**: `/Users/leeovery/Code/mint/.workflows/interactive-mint-init-setup/specification/interactive-mint-init-setup/specification.md` — sections "README — config reference verification", "Drift test", "Definition of done" (README tripwire (optional)).
