---
phase: 1
phase_name: Config-metadata SoT + drift test
total: 5
---

## interactive-mint-init-setup-1-1 | approved

### Task interactive-mint-init-setup-1-1: Define the config-metadata SoT table (rows + typed level)

**Problem**: mint has no single, structured, in-binary record of its config metadata. The per-key meaning currently lives only in the `internal/initgen` commented template (a drift surface this feature is removing) and the README. Downstream surfaces (`mint setup`'s config reference) and the drift test need one authoritative table — one row per config key carrying `key · level · default · description` — that is schema-adjacent and renderable.

**Solution**: Add a structured source-of-truth (SoT) table of config metadata: a typed level enum, a row type with the four columns, and a single exported constructor/accessor returning the full ordered slice of rows. One row per `(level, key)` pair — including the dual-level `ai_command`/`timeout` keys (one row each at top-level, `[release]`, `[commit]`), the `[release.hooks]` keys, and NO row for the `release`/`commit`/`hooks` container fields. This task establishes the rows, their levels, keys, and descriptions; the `default` column representation convention is applied in task 1-2 and the literal default values pinned in task 1-5 (leave `default` cells as placeholder/empty for now, but include the field on the row type).

**Outcome**: A Go value (e.g. `config.MetadataRows()` returning `[]config.MetadataRow`) exists in the binary listing all 25 config keys as `(level, key)` rows with descriptions, queryable by tests and renderable by `mint setup` in a later phase.

**Do**:
- Decide package placement and record the decision in this task (see Context — recommend option (a): SoT lives inside `internal/config`). Author the SoT in `internal/config` so the task 1-3 reflection can read the UNEXPORTED `fileShape`/`releaseShape`/`commitShape`/`hooksShape` struct tags directly without an exported reflection seam.
- Add a typed level: e.g. `type MetadataLevel int` with values `LevelShared`, `LevelRelease`, `LevelReleaseHooks`, `LevelCommit` (exported, so `mint setup` and tests can name them). Give it a `String()` returning the TOML rendering used in the config reference (`""`/top-level for shared, `[release]`, `[release.hooks]`, `[commit]`) so the render target and the drift test agree on level identity.
- Add a row type `MetadataRow` with exported fields `Key string`, `Level MetadataLevel`, `Default string`, `Description string`.
- Add an exported accessor (e.g. `func MetadataRows() []MetadataRow`) returning the ordered slice of all rows. Order rows by level then by schema field order (top-level shared, then `[release]`, then `[release.hooks]`, then `[commit]`) for stable rendering.
- Enumerate the 25 rows with their `Key`, `Level`, and a one-line `Description` each (leave `Default` empty here — 1-2 sets representation, 1-5 pins literal values):
  - Shared (4): `ai_command`, `max_diff_lines`, `timeout`, `diff_exclude`.
  - `[release]` (14): `tag_prefix`, `commit_prefix`, `release_branch`, `publish`, `changelog`, `provider`, `context`, `prompt`, `on_notes_failure`, `fallback`, `version_file`, `version_pattern`, `ai_command`, `timeout`.
  - `[release.hooks]` (3): `preflight`, `pre_tag`, `post_release`.
  - `[commit]` (4): `context`, `prompt`, `ai_command`, `timeout`.
- Write the SoT in a new file (e.g. `internal/config/metadata.go`) and its test in `internal/config/metadata_test.go` (external `package config_test`, mirroring the existing `config_test`/`initgen_test` style). NOTE: task 1-3's reflection helper must read unexported struct tags, so that helper will live in an INTERNAL test file (`package config`) or a non-test internal file — see task 1-3.

**Acceptance Criteria**:
- [ ] `config.MetadataRows()` returns exactly 25 rows (sanity check — do NOT hard-code `25` in production code; derive nothing from a count, the drift test in 1-4 is the real guard).
- [ ] `ai_command` and `timeout` each appear as three distinct rows: one at `LevelShared`, one at `LevelRelease`, one at `LevelCommit`.
- [ ] No row exists for the keys `release`, `commit`, or `hooks` (the container fields emit no metadata row).
- [ ] Every row carries a non-empty `Description`.
- [ ] `MetadataLevel.String()` renders `[release]`, `[release.hooks]`, `[commit]` for those levels and the top-level/shared form for `LevelShared`.
- [ ] All standard gates pass (`go build`, `gofmt -l`, `go vet`, `go test -race`, `golangci-lint`).

**Tests**:
- `"it returns one row per (level, key) pair for all 25 config keys"`
- `"it emits ai_command at the shared, release, and commit levels as three distinct rows"`
- `"it emits timeout at the shared, release, and commit levels as three distinct rows"`
- `"it emits no row for the release, commit, or hooks container keys"`
- `"it carries a non-empty description on every row"`
- `"it renders [release], [release.hooks], and [commit] level strings"`

**Edge Cases**:
- `ai_command`/`timeout` are dual-level (in fact tri-level: shared + `[release]` + `[commit]`) — they must NOT be collapsed to a single row; the row identity is the `(level, key)` pair.
- The `release`/`commit`/`hooks` container fields are NOT keys and emit zero rows — the inverse of the dual-level case (a container maps to zero rows; a dual-level key maps to one row per level). The recursion that supplies the level for nested leaf keys is the drift-test reflection's job (task 1-3), not the SoT's — the SoT just states the levels explicitly.

**Context**:
> The SoT is the single in-binary source of config metadata, schema-adjacent, with columns key · level · default · description (spec "Config-metadata source of truth (SoT)"). It renders into `mint setup` (Phase 2) and is drift-tested against the real schema (Phase 1 task 1-4).
>
> Verified against `internal/config/config.go`: the decode-shape structs are `fileShape` (top-level: `AICommand` toml:"ai_command", `MaxDiffLines` toml:"max_diff_lines", `Timeout` toml:"timeout", `DiffExclude` toml:"diff_exclude", plus containers `Release` toml:"release" and `Commit` toml:"commit"), `releaseShape` (14 leaf keys listed above plus `Hooks` toml:"hooks"), `commitShape` (`context`, `prompt`, `ai_command`, `timeout`), and `hooksShape` (`preflight`, `pre_tag`, `post_release`). These structs and tags are UNEXPORTED.
>
> PACKAGE-LAYOUT DECISION (carry into 1-3, recorded here for the implementer): CLAUDE.md bans `internal/initgen` from importing `config`, but imposes NO such ban on the new SoT. Because task 1-3's reflection must read the UNEXPORTED struct tags of `fileShape`/`releaseShape`/`commitShape`/`hooksShape`, there are two viable placements:
> - Option (a) — RECOMMENDED: the SoT and its derivation/drift test live INSIDE `internal/config`, reflecting over the private shapes directly (an internal `package config` test file, or an unexported internal helper, can `reflect.TypeOf(fileShape{})` and read tags). Lighter, lower-risk, no new exported API surface, no new package.
> - Option (b): a new package holds the SoT, and `config` grows an EXPORTED reflection seam (e.g. an exported function returning the leaf `(level, key)` set, or exported copies of the shapes). Heavier — adds exported surface purely to satisfy the test.
> Recommend option (a). This is a decision for the implementer; the tradeoff is stated, not silently fixed.
>
> Description content is prose (a planning detail) but must be the one-line meaning of each key — source it from the existing per-field doc comments in `config.go` and the README per-key tables (e.g. `tag_prefix` — "prefix applied to the version when forming the git tag"; `release_branch` — "branch the release must run on; empty auto-derives from origin/HEAD"; `on_notes_failure` — "policy when AI notes fail: abort or fallback"). Keep descriptions DRY of default values (the `default` column carries those — see task 1-2 and 1-5; the minimalism guidance and the no-restate-defaults rule in the spec's "minimalism" section depend on this separation).

**Spec Reference**: `.workflows/interactive-mint-init-setup/specification/interactive-mint-init-setup/specification.md` — "Config-metadata source of truth (SoT)", "Drift test (the anti-drift enforcement)", "Render targets and layering".

## interactive-mint-init-setup-1-2 | approved

### Task interactive-mint-init-setup-1-2: Apply the decided default-column representation convention to the SoT rows

**Problem**: Many config keys have no concrete scalar default (empty-string defaults, sentinel-auto defaults, an empty collection, per-verb inherit defaults, activate-only hooks). The SoT `default` column must represent these UNAMBIGUOUSLY — the minimalism guidance in the emitted guide depends on the agent being able to tell "this is auto-derived" from "this has no value" from "this inherits the shared value". A blank cell everywhere would conflate distinct meanings.

**Solution**: Populate the `Default` cell of each SoT row from task 1-1 using the decided representation convention, matching the convention the README per-key tables already use so the two human/agent surfaces stay mutually consistent. This task sets the REPRESENTATION (the convention's symbolic cells); task 1-5 pins the two literal real-default cells (`ai_command`/`timeout` at the shared level) against the `config` constants.

**Outcome**: Every SoT row's `Default` cell carries the correct convention token: blank for empty-string defaults, `auto` for sentinel-auto, `[]` for the empty collection, `shared` for per-verb inherit overrides, and blank/`—` for hooks keys; the two shared-level literal-default cells are left for task 1-5.

**Do**:
- For the shared scalar/literal-default keys, set the cells task 1-5 will pin: `ai_command` and `timeout` at `LevelShared` carry the REAL compiled defaults (pinned in 1-5 — leave a clear seam, e.g. set them from the `config` constants now if convenient, but 1-5 is the task that adds the pinning TEST). `max_diff_lines` (shared) carries its literal default `50000`. `diff_exclude` (shared) carries `[]` (empty collection).
- For `[release]` keys with a concrete scalar default: `tag_prefix` → `v`, `commit_prefix` → `🌿`, `publish` → `true`, `changelog` → `true`, `on_notes_failure` → `abort`.
- For `[release]` empty-string defaults → BLANK cell: `context`, `prompt`, `fallback`, `version_file`, `version_pattern`.
- For `[release]` sentinel-auto defaults → the word `auto` (distinct from blank): `release_branch` (empty means auto-derive from origin/HEAD), `provider` (empty means auto-detect from the remote host).
- For per-verb inherit overrides → the word `shared`: `[release].ai_command`, `[release].timeout`, `[commit].ai_command`, `[commit].timeout` (each means "inherit the shared top-level value unless overridden").
- For `[commit].context` and `[commit].prompt` → BLANK cell (empty-string defaults, mirroring `[release]`).
- For `[release.hooks]` keys (`preflight`, `pre_tag`, `post_release`) → no compiled default; cell is blank or `—` (pick one and apply consistently; the description carries when each hook runs).
- Pick a single sentinel-blank representation (recommend the literal empty string `""` rendered as a visibly-blank cell, and `—` reserved for the hooks keys if you want hooks distinguishable from empty-string-default keys — but blank for both is also acceptable per spec; document the choice in the test).
- Add table-driven test cases in `metadata_test.go` asserting the `Default` cell of each row matches the convention.

**Acceptance Criteria**:
- [ ] `context`, `prompt`, `fallback`, `version_file`, `version_pattern` (`[release]`) and `context`, `prompt` (`[commit]`) carry a blank `Default` cell.
- [ ] `release_branch` and `provider` carry `Default` == `auto` (distinct from the blank cells above).
- [ ] `diff_exclude` carries `Default` == `[]`.
- [ ] `[release].ai_command`, `[release].timeout`, `[commit].ai_command`, `[commit].timeout` carry `Default` == `shared`.
- [ ] `[release.hooks]` rows (`preflight`, `pre_tag`, `post_release`) carry no default value (blank or `—`, applied consistently).
- [ ] `tag_prefix`==`v`, `commit_prefix`==`🌿`, `publish`==`true`, `changelog`==`true`, `on_notes_failure`==`abort`, `max_diff_lines`==`50000`.
- [ ] The blank-for-empty-string cells are DISTINGUISHABLE from the `auto` cells (a test asserts `release_branch`/`provider` are NOT blank).
- [ ] All standard gates pass.

**Tests**:
- `"it renders empty-string-default keys as a blank default cell"`
- `"it renders sentinel-auto defaults (release_branch, provider) as auto, distinct from blank"`
- `"it renders the empty diff_exclude collection as []"`
- `"it renders per-verb ai_command and timeout inherit overrides as shared"`
- `"it renders hooks keys with no default (blank or em-dash) consistently"`
- `"it renders concrete scalar defaults verbatim (tag_prefix v, publish true, on_notes_failure abort)"`

**Edge Cases**:
- The blank-vs-`auto` distinction is load-bearing: `release_branch` and `provider` default to `""` in the Go struct, but their `""` MEANS "auto", so they render `auto`, NOT blank — whereas `context`/`prompt`/`fallback`/`version_file`/`version_pattern` also default to `""` but render blank (genuinely "no value"). The agent must tell these apart (spec: "distinct from a plain blank, so the agent can tell 'auto' from 'no value'").
- The four per-verb `ai_command`/`timeout` rows render `shared` (inherit) NOT the real default value — the real default lives ONLY on the shared-level row (task 1-5). This is the inverse of the shared rows.

**Context**:
> Spec "`default` column representation": empty-string defaults → blank; sentinel-empty "auto" defaults (`release_branch`, `provider`) → `auto`; empty collection (`diff_exclude`) → `[]`; per-verb override inherit defaults (`[release]`/`[commit]` `ai_command`/`timeout`) → `shared`; `[release.hooks]` keys → no default (blank/`—`). The spec states these representations are part of the DECIDED behaviour, not a planning-only rendering choice, because the minimalism guidance depends on the column being unambiguous.
>
> Verified against `internal/config/config.go` `defaults()`: `defaultTagPrefix="v"`, `defaultCommitPrefix="🌿"`, `defaultPublish=true`, `defaultChangelog=true`, `defaultOnNotesFailure="abort"`, `defaultMaxDiffLines=50000`. `ReleaseBranch` and `Provider` default to `""` (sentinel-auto). `Context`/`Prompt`/`Fallback`/`VersionFile`/`VersionPattern` default to `""`. The per-verb `AICommand`/`Timeout` overrides default to nil (no override → inherit shared) per `AICommandFor`/`TimeoutFor`.
>
> AMBIGUITY noted for the implementer: the spec offers "blank OR `—`" for the hooks cells. Either satisfies the contract; pick one and apply it consistently across all three hooks rows, and pin the choice in the test. If you reserve `—` for hooks (so hooks are visually distinct from empty-string-default keys) you gain a small clarity win, but plain blank is equally spec-compliant.

**Spec Reference**: `.workflows/interactive-mint-init-setup/specification/interactive-mint-init-setup/specification.md` — "Config-metadata source of truth (SoT)" → "`default` column representation".

## interactive-mint-init-setup-1-3 | approved

### Task interactive-mint-init-setup-1-3: Mechanically derive the schema leaf-key set from the decode-shape structs via reflection

**Problem**: The drift test (task 1-4) must compare the SoT against the schema's ACTUAL key set — not against a second hand-maintained list, which would only prove two copies of the same hand-list agree. The authoritative `(level, key)` set must be derived MECHANICALLY from the `config` decode-shape structs' `toml` tags so the drift test catches a genuine schema↔SoT divergence when a struct field is added, renamed, or removed.

**Solution**: Write a reflection helper that walks the `fileShape`/`releaseShape`/`commitShape`/`hooksShape` structs by their `toml` tags and produces the set of leaf `(level, key)` pairs: every `toml`-tagged LEAF field (scalar or collection) yields one pair at its level; the three sub-table CONTAINER fields (`fileShape.Release` toml:"release", `fileShape.Commit` toml:"commit", `releaseShape.Hooks` toml:"hooks") yield ZERO pairs and the traversal RECURSES into them, the nested struct's tag supplying the level for its leaf rows.

**Outcome**: A helper (e.g. `schemaLeafKeys() []leafKey` where `leafKey` carries `{Level MetadataLevel, Key string}`) returns exactly 25 leaf pairs derived purely from the struct tags, with the dual-level `ai_command`/`timeout` appearing once per level and no pair for the `release`/`commit`/`hooks` containers.

**Do**:
- Place the helper where it can read the UNEXPORTED shapes (per the 1-1 package decision, option (a)): an internal `package config` file (helper can be unexported and test-only, e.g. in a `_test.go` file in `package config`, or an unexported non-test helper if other phases will reuse it). It must be able to `reflect.TypeOf(fileShape{})` etc. and read `Field(i).Tag.Get("toml")`.
- Implement a recursive walk starting from `reflect.TypeOf(fileShape{})`:
  - For each field, read the `toml` tag (the key name). Skip fields with no `toml` tag (none exist today, but be defensive).
  - Determine whether the field is a CONTAINER (its type is one of the sub-shape structs: `releaseShape`, `commitShape`, `hooksShape`) or a LEAF. Recommended detection: if the field's underlying type is a struct that itself carries `toml`-tagged fields, treat it as a container and recurse; otherwise it is a leaf. (A robust alternative: maintain the container→level mapping explicitly — `release`→`LevelRelease`, `commit`→`LevelCommit`, `hooks`→`LevelReleaseHooks` — and recurse on a struct-kind field. Document the chosen detection.)
  - For a container field: emit NO leaf pair; recurse into the nested struct, mapping its tag to the level (`release` → `LevelRelease`, `commit` → `LevelCommit`, `hooks` → `LevelReleaseHooks`). The level for the CURRENT recursion is determined by the container tag — top-level fields are `LevelShared`.
  - For a leaf field (scalar like `*string`/`string`/`*int`/`*bool`/`HookValue`, or a collection like `[]string`): emit one `(currentLevel, tagKey)` pair.
- Return the accumulated slice of leaf pairs. Do NOT deduplicate across levels — `ai_command` at `LevelShared`, `LevelRelease`, and `LevelCommit` are three distinct pairs and all must appear.
- Add tests proving the derived set has the expected shape (these are derivation-correctness tests, separate from the bijection test in 1-4): the count is 25, the three container keys are absent, `ai_command`/`timeout` appear at all three levels, the three hooks keys appear at `LevelReleaseHooks`.

**Acceptance Criteria**:
- [ ] `schemaLeafKeys()` (or equivalent) returns exactly 25 `(level, key)` pairs derived from the struct tags — no hand-maintained key list.
- [ ] No pair has key `release`, `commit`, or `hooks` (containers emit zero pairs).
- [ ] The traversal recurses into `fileShape.Release`, `fileShape.Commit`, `releaseShape.Hooks` and tags their leaf children at `LevelRelease`, `LevelCommit`, `LevelReleaseHooks` respectively.
- [ ] `ai_command` and `timeout` each appear exactly once at each of `LevelShared`, `LevelRelease`, `LevelCommit` (three pairs each).
- [ ] The three `[release.hooks]` leaf keys (`preflight`, `pre_tag`, `post_release`) appear at `LevelReleaseHooks`.
- [ ] The helper reads `toml` tags only — it does not consult the SoT (no coupling between the two sides of the future bijection).
- [ ] All standard gates pass.

**Tests**:
- `"it derives all leaf (level, key) pairs from the decode-shape struct tags"`
- `"it emits no pair for the release, commit, or hooks container fields (recurse-don't-count)"`
- `"it tags release leaf keys at the release level via the recursed container tag"`
- `"it tags hooks leaf keys at the release.hooks level"`
- `"it emits ai_command and timeout at all three levels as distinct pairs"`
- `"it derives the key set from struct tags, independent of the SoT"`

**Edge Cases**:
- recurse-don't-count: `fileShape.Release`/`fileShape.Commit`/`releaseShape.Hooks` are `toml`-tagged but are table CONTAINERS — they must emit zero pairs and the walk recurses into them. This is the INVERSE of the dual-level rule (a container → zero pairs + recursion; a dual-level key → one pair per level).
- nested-tag-supplies-level: the level for a leaf inside `[release]` comes from the `release` container tag, the leaf inside `[release.hooks]` from the `hooks` tag nested under `release`, the leaf inside `[commit]` from the `commit` tag — the helper must thread the level down through recursion, not infer it from the leaf field name.
- Defensive: a leaf field with an empty/`-` `toml` tag (none today) should be skipped, not crash — guard the tag read.
- `HookValue` fields have underlying type `any` (interface) — they are LEAVES, not containers; ensure the container-detection logic does not mistake an interface-typed field for a sub-shape.

**Context**:
> Spec "Drift test" → "What counts as one 'key' (the bijection contract)": the authoritative key set is derived mechanically from the decode-shape structs' `toml` tags (`fileShape`, `releaseShape`, `commitShape`, `hooksShape`), NOT a hand-maintained list. The bijection is total over LEAF keys: every `toml`-tagged leaf field has exactly one matching SoT row at its level. Sub-table container fields are recursed, not counted — `fileShape.Release` (toml:"release"), `fileShape.Commit` (toml:"commit"), `releaseShape.Hooks` (toml:"hooks") emit no row and the traversal recurses into them, the nested struct's tag supplying the level. This recurse-don't-count rule is the inverse of the dual-level case.
>
> Verified against `internal/config/config.go`: `fileShape` has 6 fields (4 leaf-tagged + `Release`/`Commit` containers); `releaseShape` has 15 fields (14 leaf-tagged + `Hooks` container); `commitShape` has 4 leaf fields; `hooksShape` has 3 leaf fields. The sub-shape Go types are `releaseShape`, `commitShape`, `hooksShape`. All structs and tags are unexported, which is WHY the helper must live in `package config` (the 1-1 package-layout decision, option (a)).
>
> The reflection should compare against the field SET, not field order, so adding/removing/renaming a struct field changes the derived set and (via task 1-4) fails the build.

**Spec Reference**: `.workflows/interactive-mint-init-setup/specification/interactive-mint-init-setup/specification.md` — "Drift test (the anti-drift enforcement)" → "What counts as one 'key' (the bijection contract)".

## interactive-mint-init-setup-1-4 | approved

### Task interactive-mint-init-setup-1-4: Drift test — total bijection over leaf keys (SoT ↔ derived schema set)

**Problem**: Centralising config metadata only earns its keep if the SoT cannot drift from the schema the binary actually accepts. Without a build-failing test, a future schema change (new key, renamed key, removed key) silently desyncs the `mint setup` config reference from reality — exactly the drift this feature exists to kill, mirroring the existing `initgen`↔`config` drift discipline.

**Solution**: Add a Go test that proves a TOTAL BIJECTION over leaf `(level, key)` pairs between the SoT rows (task 1-1/1-2) and the mechanically-derived schema leaf-key set (task 1-3). Every schema leaf has exactly one matching SoT row at its level; every SoT row maps back to exactly one schema leaf. The test fails the build on any divergence.

**Outcome**: A build-failing test (`TestMetadataSoT_BijectsSchemaLeafKeys` or similar) that catches a schema leaf with no SoT row, an SoT row with no schema leaf (removed/renamed key), and a duplicate SoT row for one `(level, key)`; each of the three dual-level rows is matched independently per level.

**Do**:
- Build the SoT pair set from `MetadataRows()`: project each row to its `(Level, Key)` pair.
- Build the schema pair set from `schemaLeafKeys()` (task 1-3).
- Assert SET EQUALITY on `(level, key)` pairs in BOTH directions, reporting the specific offenders:
  - every schema leaf pair has a matching SoT row — report any schema pair missing from the SoT;
  - every SoT row has a matching schema leaf — report any SoT pair missing from the schema (catches removed/renamed keys still listed in the SoT);
- Assert NO DUPLICATE SoT rows: build a multiset/count map over SoT `(level, key)` pairs and fail if any pair appears more than once (a duplicate would let a real divergence hide behind a stray row).
- Match strictly on the `(level, key)` PAIR — NOT on bare key name. `ai_command` at `LevelShared`, `LevelRelease`, and `LevelCommit` are three independent matches; a test case proves each of the three dual-level rows matches its own level's schema leaf independently (e.g. removing the `[commit]` `ai_command` row fails the bijection even though shared and `[release]` `ai_command` rows still exist).
- Model the test in the style of the existing `internal/initgen/initgen_test.go` drift pins (build-failing, names the offender) — but in `internal/config` (package placement per 1-1).
- Emit a clear failure message listing the diverging pairs (level + key) so a future schema edit points the implementer straight at the missing/extra SoT row.

**Acceptance Criteria**:
- [ ] The test passes against the current schema (25 pairs match exactly, bijective).
- [ ] Removing a leaf field from any decode-shape struct (without updating the SoT) FAILS the test naming the now-orphaned SoT row.
- [ ] Adding a leaf field to any decode-shape struct (without updating the SoT) FAILS the test naming the unmatched schema leaf.
- [ ] An SoT row whose `(level, key)` has no schema leaf FAILS the test (removed/renamed key still in the SoT).
- [ ] A duplicate SoT row for one `(level, key)` FAILS the test.
- [ ] Each of the three dual-level rows (`ai_command`, `timeout` × shared/release/commit) matches independently per level — dropping any one fails the bijection while the others still match.
- [ ] All standard gates pass.

**Tests**:
- `"it bijects the SoT rows against the derived schema leaf-key set on the current schema"`
- `"it fails when a schema leaf has no matching SoT row (added/renamed key)"`
- `"it fails when an SoT row has no matching schema leaf (removed/renamed key)"`
- `"it fails on a duplicate SoT row for one (level, key) pair"`
- `"it matches each dual-level ai_command/timeout row independently per level"`

**Edge Cases**:
- The bijection is over `(level, key)` PAIRS, never bare key names — a key present at one level but not another must surface as a per-level mismatch (e.g. an SoT that lists `timeout` at shared and `[release]` but forgot `[commit]` fails, even though the bare name `timeout` exists in the SoT).
- The "added/renamed/removed key" cases cannot be asserted by literally editing the struct in the test (the struct is fixed); instead, prove the bijection MECHANISM catches divergence — e.g. by constructing a deliberately-broken SoT pair set (drop one pair, add a phantom pair, duplicate a pair) and asserting the comparison helper reports it. Factor the comparison into a pure helper taking two pair sets so it can be unit-tested against synthetic divergent inputs without mutating the real schema or SoT.
- Duplicate detection must be on the full `(level, key)` pair, not the key alone — otherwise the three legitimate `ai_command` rows would read as duplicates.

**Context**:
> Spec "Drift test (the anti-drift enforcement)": a Go test fails the build if the SoT and the canonical schema disagree (a key in the schema but missing from the SoT, or vice versa). It mirrors the existing `initgen`↔`config` drift discipline. The bijection is TOTAL over leaf keys and matched per `(level, key)` pair: `ai_command`/`timeout` at both shared and `[release]`/`[commit]` are distinct rows matched per level, not collapsed; `[release.hooks]` keys are their own rows; container fields are recursed not counted.
>
> The existing drift discipline to mirror lives in `internal/initgen/initgen_test.go` (e.g. `TestMintTOML_AICommandValueEqualsConfigConstant`, `TestMintTOML_TimeoutValueEqualsConfigConstant`) — build-failing tests that pin a value/shape against the canonical source and name the offender. Mirror that style: a focused, clearly-named, build-failing test with an offender-naming message.
>
> Factoring the comparison into a pure two-set helper is what makes the divergence cases testable without mutating the real schema — the real schema is always in sync (that is the point), so the synthetic-divergence inputs prove the GUARD works.

**Spec Reference**: `.workflows/interactive-mint-init-setup/specification/interactive-mint-init-setup/specification.md` — "Drift test (the anti-drift enforcement)" → "What counts as one 'key' (the bijection contract)"; "Definition of done" → "Drift test".

## interactive-mint-init-setup-1-5 | approved

### Task interactive-mint-init-setup-1-5: Pin the subsumed scaffold default values on the SoT default column

**Problem**: Today `initgen`'s drift tests pin the scaffold's literal default values (`ai_command`, `timeout`) equal to `config.DefaultAICommand` / `config.DefaultTimeout` so no re-typed literal can drift from the canonical constant. The strip-to-minimal change (Phase 3) removes the scaffold's default values, so that value-drift discipline must be SUBSUMED by the SoT: the SoT `default` column becomes the drift-pinned carrier of those values. No default value may be left unpinned by the change.

**Solution**: Add build-failing tests that pin the SoT `Default` cells for the shared-level `ai_command` and `timeout` rows against the canonical `config` constants — `ai_command`'s cell equals `config.DefaultAICommand`, and `timeout`'s cell equals the integer-seconds form DERIVED from `config.DefaultTimeout` (`int(config.DefaultTimeout / time.Second)`), never a re-typed `60`. This mirrors and replaces the existing `initgen` value-drift pins, which themselves are removed from `initgen` in Phase 3 (NOT here).

**Outcome**: The SoT shared-level `ai_command`/`timeout` `Default` cells are tied to `config.DefaultAICommand` / `config.DefaultTimeout` by build-failing tests, so a future change to either constant that is not reflected in the SoT fails the build — the value-drift discipline previously carried by `initgen` now lives on the SoT.

**Do**:
- Ensure the shared-level `ai_command` row's `Default` cell is sourced from / equal to `config.DefaultAICommand` (the constant `"claude -p --model sonnet"`). If task 1-2 set the cell from a literal, change it to derive from the constant.
- Ensure the shared-level `timeout` row's `Default` cell renders the integer SECONDS derived from `config.DefaultTimeout` — `int(config.DefaultTimeout / time.Second)` formatted as a decimal string (`"60"`), NOT a re-typed `60`. `config.DefaultTimeout` is a `time.Duration` constant (`60 * time.Second`); the cell is the seconds integer as it appears in TOML.
- Add a build-failing drift test pinning the `ai_command` shared-row `Default` to `config.DefaultAICommand`.
- Add a build-failing drift test pinning the `timeout` shared-row `Default` to `strconv.Itoa(int(config.DefaultTimeout / time.Second))` (import `time` and `strconv` in the test, mirroring `initgen_test.go`'s `TestMintTOML_TimeoutValueEqualsConfigConstant`).
- Do NOT touch `internal/initgen` here — the existing `initgen` value-drift pins (`TestMintTOML_AICommandValueEqualsConfigConstant`, `TestMintTOML_TimeoutValueEqualsConfigConstant`) are removed in Phase 3 when `MintTOML()` is stripped. Phase 1 only ADDS the subsuming SoT pin; the two pins coexist until Phase 3.

**Acceptance Criteria**:
- [ ] The shared-level `ai_command` SoT row's `Default` cell equals `config.DefaultAICommand`.
- [ ] The shared-level `timeout` SoT row's `Default` cell equals `int(config.DefaultTimeout / time.Second)` rendered as a decimal string (`"60"`), derived from the constant, not a re-typed literal.
- [ ] A build-failing test fails if the `ai_command` cell drifts from `config.DefaultAICommand`.
- [ ] A build-failing test fails if the `timeout` cell drifts from the seconds derived from `config.DefaultTimeout`.
- [ ] The `timeout` cell is the integer-seconds form (`"60"`), NOT the duration string (`"1m0s"`) and NOT a re-typed `60`.
- [ ] `internal/initgen` is untouched in this task (its existing value-drift pins still pass).
- [ ] All standard gates pass.

**Tests**:
- `"it pins the shared ai_command default cell to config.DefaultAICommand"`
- `"it pins the shared timeout default cell to the seconds derived from config.DefaultTimeout"`
- `"it renders the timeout default as integer seconds, not the duration string"`

**Edge Cases**:
- `timeout` must be pinned as integer SECONDS derived from the duration constant (`int(config.DefaultTimeout / time.Second)` → `60`), NOT a re-typed `60` literal and NOT the `time.Duration` String form (`"1m0s"`). This mirrors `initgen_test.go`'s existing `want := int(config.DefaultTimeout / time.Second)` derivation — the whole point is no second copy of the literal.
- This task pins ONLY the two shared-level real-default cells. The `[release]`/`[commit]` `ai_command`/`timeout` rows render `shared` (inherit — task 1-2), so they are deliberately NOT pinned to a literal default value; pinning them to a literal would contradict the inherit representation.
- `max_diff_lines` (default `50000`) is a shared literal default too — it has no `config`-EXPORTED constant today (`defaultMaxDiffLines` is unexported). Its cell value is set in task 1-2 as the literal `50000`; this task's subsuming pin targets only `ai_command`/`timeout` (the two `initgen` pinned). Note in the test comment that `max_diff_lines` is pinned to the unexported `defaultMaxDiffLines` only if the SoT lives in `package config` (option (a)) and can read it; otherwise it stays a literal in the SoT (the spec's subsumption scope is `ai_command`/`timeout`, which were the only `initgen` value-drift pins).

**Context**:
> Spec "`initgen` scope of change" → "The scaffold-value drift-pin moves to the SoT": today `initgen`'s drift tests pin the scaffold's literal default values (`ai_command`, `timeout`) equal to `config.DefaultAICommand` / `config.DefaultTimeout`. The minimal template carries no default values to pin, so that value-drift discipline is subsumed by the new SoT drift test — the SoT `default` column becomes the drift-pinned carrier. No default value is left unpinned.
>
> Spec "Definition of done": "The scaffold-value drift-pin is subsumed by the SoT drift test." The removal of the `initgen` pin itself happens in Phase 3 ("Updated `initgen` tests"), NOT Phase 1 — Phase 1 only adds the subsuming SoT pin.
>
> Verified against `internal/config/config.go`: `DefaultAICommand = "claude -p --model sonnet"` (exported string const), `DefaultTimeout = 60 * time.Second` (exported `time.Duration` const). Verified against `internal/initgen/initgen_test.go`: `TestMintTOML_AICommandValueEqualsConfigConstant` pins the scaffold `ai_command` to `config.DefaultAICommand`; `TestMintTOML_TimeoutValueEqualsConfigConstant` pins the scaffold `timeout` to `int(config.DefaultTimeout / time.Second)` via `strconv.Itoa(want)`. Mirror that exact derivation for the SoT pin.

**Spec Reference**: `.workflows/interactive-mint-init-setup/specification/interactive-mint-init-setup/specification.md` — "`initgen` scope of change" → "The scaffold-value drift-pin moves to the SoT"; "Definition of done".
