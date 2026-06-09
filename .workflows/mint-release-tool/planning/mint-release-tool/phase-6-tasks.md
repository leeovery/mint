---
phase: 6
phase_name: Config Schema & `mint init` Scaffolding
total: 8
---

## mint-release-tool-6-1 | approved

### Task mint-release-tool-6-1: Typed config schema structs (full verb-namespaced shape)

**Problem**: Across Phases 1–4 the config loader grew organically — each phase read only the keys it needed (`tag_prefix`/`commit_prefix`/`release_branch`/`publish` in Phase 1; `ai_command`/`max_diff_lines`/`on_notes_failure`/`context`/`prompt` in Phase 2; `[release.hooks]`/`version_file`/`version_pattern`/`diff_exclude` in Phase 3; `provider` in Phase 4). There is no single typed struct describing the *complete* `.mint.toml` shape, so there is nothing to validate the whole file against (Phase 6's later tasks) and nothing the loader can decode once, up front, with all defaults applied uniformly.

**Solution**: Define the complete typed config schema as Go structs mirroring the verb-namespaced TOML shape — shared engine keys at the top level (`ai_command`, `diff_exclude`, `max_diff_lines`), a `[release]` table, and a nested `[release.hooks]` table — and a `Load`/`Defaults` path that applies every default on zero-config (file absent / empty / comment-only) and defaults each key individually when only part of a table is present.

**Solution note**: This task establishes the consolidated *struct shape + defaults*. The fail-loud validation of unknown keys (6-2) and bad types (6-3) and the rewiring of the earlier per-key reads through this schema (6-4) are separate tasks — do not implement validation or rewiring here, only the typed shape and complete default application.

**Outcome**: A single `Config` struct (with `Release` and `Release.Hooks` sub-structs) holds every documented key with its correct Go type. Loading an absent, empty, or comment-only `.mint.toml` yields all defaults; a file that sets only some keys in a table leaves the unset keys at their defaults; setting only top-level keys leaves `[release]` and `[release.hooks]` fully defaulted.

**Do**:
- In `internal/config`, consolidate the accreted fields into one canonical schema. Suggested shape:
  - Top level (shared engine): `AICommand string` (`ai_command`), `DiffExclude []string` (`diff_exclude`), `MaxDiffLines int` (`max_diff_lines`).
  - `Release struct` (`[release]`): `TagPrefix string`, `CommitPrefix string`, `ReleaseBranch string`, `VersionFile string`, `VersionPattern string`, `Changelog bool`, `Publish bool`, `Provider string`, `OnNotesFailure string`, `Context string`, `Prompt string`.
  - `Release.Hooks struct` (`[release.hooks]`): `Preflight`, `PreTag`, `PostRelease` — each a string-or-array value (model as a dedicated `HookValue` type that decodes from either a TOML string or `[]string`; the actual string-vs-array decoding/validation is 6-3, but the field type must support both now).
- Apply the full default set, matching the schema block in the spec:
  - `ai_command` → `"claude -p"`; `diff_exclude` → empty slice (no extra excludes); `max_diff_lines` → `50000`.
  - `release.tag_prefix` → `"v"`; `commit_prefix` → `"🌿"`; `release_branch` → `""` (sentinel: auto-derive from `origin/HEAD`); `version_file` → `""` (omit = tag-only); `version_pattern` → `""` (omit = whole file is the version); `changelog` → `true`; `publish` → `true`; `provider` → `""` (sentinel: auto-detect from remote host); `on_notes_failure` → `"abort"`; `context` → `""`; `prompt` → `""`.
  - `release.hooks.*` → empty/absent (no hook).
- Apply defaults so that the bool-zero-value trap is avoided (`changelog` and `publish` both default `true`): pre-seed the struct with defaults before decoding, or decode bools into `*bool` and resolve nil → default. An *explicit* `changelog = false` / `publish = false` must be honoured, and an explicit empty `tag_prefix = ""` must be preserved (not re-defaulted), consistent with Phase 1's loader.
- Handle the zero-config family identically: file absent, empty file, and comment-only file all return the full default struct with no decode error and no spurious values.
- Keep using the TOML decoder chosen in Phase 1 (`github.com/pelletier/go-toml/v2` or `github.com/BurntSushi/toml`) — do not add a second TOML dependency.

**Acceptance Criteria**:
- [ ] A single `Config` struct exists covering all top-level, `[release]`, and `[release.hooks]` keys with correct Go types.
- [ ] Absent `.mint.toml` → every key at its documented default.
- [ ] Empty file and comment-only file → identical to absent (all defaults, no error).
- [ ] A partial `[release]` table → the keys present override, the rest default individually.
- [ ] A file with only top-level keys → `[release]` and `[release.hooks]` fully defaulted.
- [ ] `changelog = false` and `publish = false` are honoured (not lost to the bool zero-value vs. default ambiguity).
- [ ] An explicit `tag_prefix = ""` is preserved, not re-defaulted to `"v"`.
- [ ] Hook fields accept both a string and an array of strings at the schema level.

**Tests**:
- `"it returns all defaults when .mint.toml is absent"`
- `"it returns all defaults for an empty or comment-only file"`
- `"it defaults unset keys individually within a partial [release] table"`
- `"it fully defaults [release] and [release.hooks] when only top-level keys are set"`
- `"it honours an explicit changelog = false and publish = false"`
- `"it preserves an explicit empty tag_prefix"`
- `"it decodes a hook value given as a string and as an array"`

**Edge Cases**:
- File absent → all defaults.
- Empty / comment-only file → all defaults.
- Partial table present → unset keys default individually.
- Only top-level keys present → tables fully defaulted.

**Context**:
> Config "Shape: shared engine keys + a table per verb … keys shared by every verb sit at the top level; each verb has its own table." The spec's schema block (verbatim defaults): `ai_command = "claude -p"`, `diff_exclude = [...]`, `max_diff_lines = 50000`; `[release]` `tag_prefix = "v"` (default "v"), `commit_prefix = "🌿"`, `release_branch` "default: auto-derived from origin/HEAD", `version_file` "optional; omit = tag-only", `version_pattern` "omit = whole file is the version", `changelog = true` ("false = no CHANGELOG.md projection"), `publish = true` ("false = tag + push only"), `provider` "optional; default auto-detected from remote host", `on_notes_failure = "abort"` (`abort | fallback`), `context`, `prompt` "optional full prompt override"; `[release.hooks]` `preflight`/`pre_tag`/`post_release`. "Shared engine keys (top level): `ai_command`, `diff_exclude`, `max_diff_lines`." "Hooks nest under their owning verb as `[release.hooks]`." Hook "Value is a string *or* an array of strings." "Fully optional. Zero config = sensible defaults everywhere." This task builds the typed shape + defaults only; unknown-key/bad-type validation are 6-2/6-3, rewiring is 6-4.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Config Format & Schema → Shape: shared engine keys + a table per verb", "Hooks → Mechanism".

## mint-release-tool-6-2 | approved

### Task mint-release-tool-6-2: Fail-loud validation — unknown keys

**Problem**: The spec mandates "Typed validation, fail-loud on unknown keys / bad types, with clear messages." Without unknown-key rejection, a typo (`tag_prefx`, `publsh`) or a misplaced key silently has no effect — the user thinks they configured something they didn't, and a release behaves unexpectedly. A particularly important case: a top-level `[hooks]` table must be rejected, because the top level is strictly shared-engine and hooks must nest under `[release.hooks]`.

**Solution**: Add strict unknown-key validation to the loader: any key not present in the schema — at the top level, inside `[release]`, or inside `[release.hooks]` — is rejected with a clear message naming the offending key and the table it appeared in; a top-level `[hooks]` table is rejected with guidance to nest it under `[release.hooks]`.

**Solution note**: This task covers unknown *keys* only. Bad *types* (6-3) and the `on_notes_failure` enum check (6-3) are separate. The known carve-out from Phase 4 — an unknown `provider` *value* warns + downgrades rather than erroring — concerns a *value*, not a key, and is preserved in 6-4; it is out of scope here.

**Outcome**: A `.mint.toml` containing any key not in the schema fails to load with a clear, actionable error that names the bad key and its table (e.g. "unknown key `tag_prefx` in `[release]`"). A top-level `[hooks]` table is rejected with a message pointing the user to `[release.hooks]`. A file with only valid keys loads cleanly.

**Do**:
- In `internal/config`, enable strict decoding so unknown keys surface rather than being ignored:
  - If using `pelletier/go-toml/v2`: decode with `Decoder.DisallowUnknownFields()` and translate the resulting `*toml.StrictMissingError` (which carries the offending key path) into mint's clear message.
  - If using `BurntSushi/toml`: use `toml.Decode`'s returned `MetaData.Undecoded()` to enumerate keys that were present in the file but matched no struct field, and reject if non-empty.
- Produce a message that names both the key and its table, e.g. `unknown key "foo" in [release]` / `unknown top-level key "bar"` / `unknown key "baz" in [release.hooks]`. Prefer the full dotted key path so the table is unambiguous.
- Explicitly reject a **top-level `[hooks]` table**: because `hooks` is not a valid top-level key, strict decoding already rejects it, but give it a *targeted* message — e.g. `[hooks] is not valid at the top level — nest hooks under [release.hooks]` — since this is the documented contradiction (top-level is strictly shared-engine). Detect the `[hooks]` table specifically and emit the guidance variant rather than the generic unknown-key message.
- Ensure validation runs on the consolidated schema from 6-1 (top level, `[release]`, `[release.hooks]` all checked) and that a fully-valid file still loads with no error.
- Tests use temp dirs with crafted `.mint.toml` files; no other Phase 6 task is required.

**Acceptance Criteria**:
- [ ] An unknown top-level key is rejected with a message naming the key.
- [ ] An unknown key inside `[release]` is rejected with a message naming the key and the `[release]` table.
- [ ] An unknown key inside `[release.hooks]` is rejected with a message naming the key and the `[release.hooks]` table.
- [ ] A top-level `[hooks]` table is rejected with guidance to nest under `[release.hooks]`.
- [ ] A typo'd key (e.g. `tag_prefx`) is surfaced clearly (named in the error), not silently ignored.
- [ ] A file containing only valid keys loads without error.

**Tests**:
- `"it rejects an unknown top-level key naming the key"`
- `"it rejects an unknown key in [release] naming the table"`
- `"it rejects an unknown key in [release.hooks] naming the table"`
- `"it rejects a top-level [hooks] table and points to [release.hooks]"`
- `"it surfaces a typo'd key clearly rather than ignoring it"`
- `"it loads a fully valid file without error"`

**Edge Cases**:
- Unknown top-level key.
- Unknown `[release]` key.
- Unknown `[release.hooks]` key.
- Top-level `[hooks]` table rejected (must nest under `[release.hooks]`).
- Typo'd key surfaced clearly.

**Context**:
> "Typed validation, fail-loud on unknown keys / bad types, with clear messages." "Hooks nest under their owning verb as `[release.hooks]` (top-level is strictly shared-engine, so a top-level `[hooks]` would contradict that rule)." On the value-vs-key distinction (preserved, not erased): "An unknown/unsupported `provider` *value* … is not a fail-loud config error — mint warns loudly and downgrades … Fail-loud config validation still applies to unknown *keys* and bad *types*." That `provider`-value carve-out is a *value* concern handled in 6-4; this task is unknown *keys* only.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Config Format & Schema → Typed validation, fail-loud", "Shape … Hooks nest under their owning verb".

## mint-release-tool-6-3 | approved

### Task mint-release-tool-6-3: Fail-loud validation — bad types

**Problem**: A key with the wrong TOML value type (`max_diff_lines = "lots"`, `publish = "yes"`, `changelog = "true"`, `diff_exclude = "*.min.js"` as a bare scalar instead of an array) must fail loudly rather than silently coercing or being dropped. Hook values are the one place where two types are *both* valid (string or array), and `on_notes_failure` is a closed enum whose out-of-range values must also be rejected. Without this, a mistyped value either errors with an opaque decoder message or, worse, silently disappears.

**Solution**: Add bad-type validation: reject a key whose TOML value type doesn't match its schema type with a clear message; accept hook values as either string or array (both valid); and validate `on_notes_failure` against its allowed enum (`abort` / `fallback`).

**Outcome**: `max_diff_lines` as a string, `publish`/`changelog` as a string, and `diff_exclude` as a scalar each fail to load with a clear, key-named type error. A hook value given as a string *or* as an array of strings both load successfully. `on_notes_failure` outside `{abort, fallback}` fails loudly; the two valid values load.

**Do**:
- In `internal/config`, ensure type mismatches surface as clear errors rather than silent drops or opaque decoder output:
  - The chosen TOML decoder raises a typed error when a value can't be unmarshalled into the target Go type (e.g. a string into an `int`/`bool`, or a scalar into a `[]string`). Catch that error and re-wrap it into a mint message that names the key and the expected type, e.g. `max_diff_lines must be an integer` / `publish must be a boolean` / `changelog must be a boolean` / `diff_exclude must be an array of strings`.
  - For `diff_exclude` specifically, a bare scalar (`diff_exclude = "*.min.js"`) must error — the field is `[]string`; the decoder will reject the scalar, which this task surfaces with the clear message.
- Implement the **string-or-array** hook decoding on the `HookValue` type from 6-1: decode a TOML string into a single-entry form and a TOML array into the multi-entry form; both are valid. Anything else (e.g. a hook value that is an integer or a table) is a bad type and is rejected with a clear message naming the hook key. Add a custom `UnmarshalTOML`/`UnmarshalText` (per the decoder's interface) on `HookValue` to accept both shapes.
- Validate the `on_notes_failure` **enum** after decoding: it must be exactly `"abort"` or `"fallback"`; any other string → fail loud with a clear message listing the valid values (e.g. `on_notes_failure must be one of: abort, fallback`). (This is a value-level constraint on a correctly-typed string, but it lives with type/shape validation as a closed-set check; it is *not* the `provider` warn-downgrade case — `on_notes_failure` is fail-loud.)
- Order vs. unknown-key validation (6-2): both run during the single up-front load; either failing aborts the load. The exact ordering is not load-bearing, but both must be active together.

**Acceptance Criteria**:
- [ ] `max_diff_lines` given as a string → fail loud, message names the key and expects an integer.
- [ ] `publish` given as a string → fail loud (boolean expected); same for `changelog`.
- [ ] `diff_exclude` given as a scalar (not an array) → fail loud (array of strings expected).
- [ ] A hook value given as a string loads successfully; given as an array of strings loads successfully.
- [ ] A hook value of a non-string/non-array type (e.g. integer) → fail loud naming the hook key.
- [ ] `on_notes_failure` set to an invalid value (e.g. `"retry"`) → fail loud listing the valid values.
- [ ] `on_notes_failure = "abort"` and `on_notes_failure = "fallback"` both load.

**Tests**:
- `"it rejects max_diff_lines given as a string"`
- `"it rejects publish given as a string"`
- `"it rejects changelog given as a string"`
- `"it rejects diff_exclude given as a scalar instead of an array"`
- `"it accepts a hook value as a string"`
- `"it accepts a hook value as an array of strings"`
- `"it rejects a hook value of an invalid type"`
- `"it rejects an invalid on_notes_failure enum value"`
- `"it accepts on_notes_failure = abort and = fallback"`

**Edge Cases**:
- Scalar where array expected (`diff_exclude`).
- String where bool expected (`publish` / `changelog`).
- String where int expected (`max_diff_lines`).
- Hook value string vs array — both valid.
- `on_notes_failure` invalid enum value → fail loud.

**Context**:
> "Typed validation, fail-loud on unknown keys / bad types, with clear messages." Hook "Value is a string *or* an array of strings. Array entries run sequentially … String for one command; array for readable multi-step." `on_notes_failure = "abort"  # abort | fallback` — a closed two-value enum. The `provider` warn-downgrade carve-out applies only to `provider` *values* (6-4); `on_notes_failure` is a normal fail-loud config key.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Config Format & Schema → Typed validation, fail-loud", "Hooks → Mechanism", "Stage 4 → Failure behaviour (`on_notes_failure`)".

## mint-release-tool-6-4 | approved

### Task mint-release-tool-6-4: Route earlier-phase as-needed loaders through the validated schema

**Problem**: Phases 1–4 each read their own keys from `.mint.toml` as needed, with their own defaults, and without consolidated validation — so an unknown key or a bad type isn't caught until (or unless) the relevant phase happens to read that key, and never up front. The spec wants the whole file validated **once, before pipeline work begins**, while every earlier-phase key still resolves with its prior default. The critical preservation: an unknown `provider` *value* must still only *warn + downgrade* (Phase 4 behaviour) — it must not be converted into a fail-loud config error by this consolidation.

**Solution**: Back the Phase 1–4 per-key reads with the single consolidated typed schema (6-1) plus its validation (6-2 unknown keys, 6-3 bad types), so the loader validates the entire file once up front and every consumer reads from the one validated `Config`; the `provider`-value warn-and-downgrade path (Phase 4) is left untouched and continues to fire at publish-resolution time, not as a config error.

**Solution note**: This is a *consolidation/rewiring* task, not new behaviour. It does not change any default, does not move the `provider` downgrade into config validation, and does not add new keys. Its test value is in proving (a) the whole file fails loud up front on unknown keys / bad types before any pipeline work, (b) all earlier-phase keys still resolve with prior defaults, and (c) the `provider`-value downgrade still warns rather than erroring.

**Outcome**: A single `config.Load(root)` returns the fully-validated `Config` (or a fail-loud error) and is the one source every consumer reads from. An unknown key or bad type aborts the run *before* version determination / preflight / notes work begins. Every Phase 1–4 key resolves with the exact default it had before. A `provider` set to an unsupported but well-typed value (e.g. `"gitlab"`) loads cleanly and still produces the Phase 4 warn-and-downgrade-to-tag+push at publish resolution.

**Do**:
- Make `internal/config.Load(root)` the single entry point that: reads `.mint.toml` (absent/empty/comment-only → defaults per 6-1), decodes into the consolidated schema, runs unknown-key validation (6-2) and bad-type/enum validation (6-3), and returns the validated `Config` or a clear error.
- Replace each earlier phase's narrow read with a read off the consolidated `Config`:
  - Phase 1: `tag_prefix`, `commit_prefix`, `release_branch`, `publish`.
  - Phase 2: `ai_command`, `max_diff_lines`, `on_notes_failure`, `context`, `prompt`.
  - Phase 3: `[release.hooks]` (`preflight`/`pre_tag`/`post_release`), `version_file`, `version_pattern`, `diff_exclude`.
  - Phase 4: `provider`.
  Each must keep its prior default exactly. If earlier phases introduced a separate minimal loader, collapse it into this one (or have it delegate) so there is a single decode + validation pass.
- Sequence the validation **before pipeline work**: in the release orchestrator (`cmd/mint` / the release runner from Phase 1's task 1-11), `Load` runs first and a config error aborts before version determination, preflight, hooks, or notes. Verify by asserting that a bad config never reaches the version/preflight stages.
- **Preserve the `provider`-value carve-out (critical)**: `provider` is a normal string key in the schema (any string is a valid *type*); an *unsupported value* is **not** rejected here. The Phase 4 publish-resolution logic continues to own the "unknown/unmatched provider → warn loudly + downgrade to tag + push only" behaviour. Add a test asserting that `provider = "gitlab"` loads without a config error and that the downgrade still occurs at publish resolution.
- Confirm the single up-front validation covers the *whole* file in one pass (one decode), not lazily per consumer.

**Acceptance Criteria**:
- [ ] `config.Load` validates the entire file once up front; consumers read from the returned `Config`.
- [ ] Every Phase 1–4 key resolves with its prior default (no default changed by the consolidation).
- [ ] An unknown key or bad type aborts the run *before* version determination / preflight / notes work begins.
- [ ] A well-typed but unsupported `provider` value (e.g. `"gitlab"`) loads without a config error.
- [ ] That unsupported `provider` value still triggers the Phase 4 warn-and-downgrade-to-tag+push at publish resolution (not converted to a config error).
- [ ] There is a single decode + validation pass, not a lazy per-consumer read.

**Tests**:
- `"it validates the whole file once and aborts before pipeline work on a bad key"`
- `"all earlier-phase keys resolve with their prior defaults through the consolidated loader"`
- `"an unknown provider value loads without a config error"`
- `"an unknown provider value still warns and downgrades to tag+push at publish resolution"`
- `"a bad type aborts before version determination / preflight"`

**Edge Cases**:
- All earlier-phase keys resolve with prior defaults.
- Unknown `provider` VALUE warns + downgrades (not a hard error).
- Whole-file validation runs once up front.
- A bad key fails before pipeline work begins.

**Context**:
> "The schema accretes naturally across earlier phases (each consumes its own keys), so this phase consolidates complete validation." Phase 4 carve-out (must survive): "An unknown/unsupported `provider` *value* (a recognised key, e.g. `provider = "gitlab"` when only GitHub is implemented) is **not** a fail-loud config error — mint **warns loudly and downgrades to tag + push only** (publish skipped), so a typo can't silently vanish. Fail-loud config validation still applies to unknown *keys* and bad *types*." "Auto-detection with no matching driver … is treated the same as an unsupported value: mint warns loudly and downgrades to tag + push only." Fail-loud (6-2/6-3) is for unknown KEYS and bad TYPES only.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Config Format & Schema → Typed validation, fail-loud", "Stages 6–7 → Publishing: provider driver abstraction".

## mint-release-tool-6-5 | approved

### Task mint-release-tool-6-5: `.mint.toml` commented-template generation

**Problem**: `mint init` activates mint in a project by dropping a documented `.mint.toml` the user tunes by uncommenting — "tunes mint by uncommenting rather than reading docs." There is no template content yet. It must present common keys with their defaults, optional keys present-but-commented with a one-line explanation each, every scaffolded key must validate under the 6-1/6-2/6-3 schema once uncommented, and it must do **no** project auto-detection (no `package.json` sniffing) — a clean, honest commented template.

**Solution**: A pure template generator returning the commented `.mint.toml` content — common keys shown with their default values, optional keys (`version_file`, `version_pattern`, `[release.hooks]`, `[release].context`/`prompt`, …) present-but-commented with a one-line explanation, and a comment merely *mentioning* a `[release].prompt` override file rather than creating one.

**Solution note**: This task produces the *template string* only (a deterministic generator with no filesystem or git work). Writing it to disk, idempotency/`--force`, and the `release` shim are the `mint init` command (6-7) and shim (6-6) tasks. There is no auto-detection anywhere in this content.

**Outcome**: A generator (e.g. `initgen.MintTOML() string`) returns the documented template. Uncommenting any scaffolded key yields a file that loads cleanly under `config.Load` (6-4) — every example value is valid for its key's type and (where applicable) enum. The content contains no project sniffing and only *mentions* a prompt-override file path in a comment.

**Do**:
- In a generation package (e.g. `internal/initgen`), implement `MintTOML() string` returning the template. Suggested structure, mirroring the spec's schema block:
  - A short header comment explaining the file and that uncommenting enables a setting.
  - **Common keys shown with defaults** (active or clearly-defaulted), e.g. `ai_command = "claude -p"`, `max_diff_lines = 50000`, and under `[release]`: `tag_prefix = "v"`, `commit_prefix = "🌿"`, `changelog = true`, `publish = true`, `on_notes_failure = "abort"`. (Showing defaults is informational; a project can leave them as-is.)
  - **Optional keys present-but-commented**, each with a one-line explanation:
    - `# diff_exclude = ["skills/**/knowledge.cjs", "*.min.js"]  # tracked generated files to keep out of the notes diff`
    - `# release_branch = "main"  # default: auto-derived from origin/HEAD`
    - `# version_file = "bin/tool"  # write the version into a file; omit = tag-only`
    - `# version_pattern = 'RELEASE_VERSION="{version}"'  # omit = the whole file is the version`
    - `# provider = "github"  # default: auto-detected from the remote host`
    - `# context = "..."  # inject project guidance into the notes prompt`
    - `# prompt = ".mint/notes-prompt.md"  # full prompt override file (create it yourself; not scaffolded)`
    - a commented `[release.hooks]` block showing `preflight`/`pre_tag` (string and array forms)/`post_release` examples, each with a one-line explanation.
  - The `[release].prompt` line is **only a mention in a comment** — `mint init` does **not** create the prompt file.
- **No auto-detection**: do not read `package.json` or any project file; the examples are static. State this intent in a code comment.
- **Validity guarantee**: every scaffolded key, once uncommented, must satisfy 6-1/6-2/6-3 — correct types, only-known keys, hooks under `[release.hooks]` (never a top-level `[hooks]`), and `on_notes_failure` within its enum. Write a test that programmatically uncomments the template (strip a leading `# ` from commented config lines) and feeds the result through `config.Load`, asserting it loads without error.
- Keep the generator pure (returns a string); no IO here.

**Acceptance Criteria**:
- [ ] The generator returns a commented `.mint.toml` template with common keys shown at their defaults.
- [ ] Optional keys (`version_file`, `version_pattern`, `[release.hooks]`, `context`, `prompt`, `diff_exclude`, `release_branch`, `provider`) are present-but-commented, each with a one-line explanation.
- [ ] Once uncommented, the whole template loads cleanly through `config.Load` (no unknown-key, bad-type, or enum error).
- [ ] No project auto-detection — no `package.json`/file sniffing anywhere in generation.
- [ ] A `[release].prompt` override file is only *mentioned* in a comment, not created (the generator emits no second file).
- [ ] Hooks appear under a `[release.hooks]` block — never a top-level `[hooks]` table.

**Tests**:
- `"the template includes common keys at their defaults"`
- `"the template includes optional keys present-but-commented with explanations"`
- `"the uncommented template loads cleanly through config.Load"`
- `"the template performs no project auto-detection"`
- `"the prompt-override file is only mentioned in a comment, not generated"`
- `"hooks are shown under [release.hooks], not a top-level [hooks]"`

**Edge Cases**:
- Every scaffolded key validates once uncommented.
- Optional keys present-but-commented with a one-line explanation each.
- No project auto-detection (no `package.json` sniffing).
- Prompt-override file only mentioned in a comment, not created.

**Context**:
> `mint init` drops "a commented template: the common keys with their defaults, plus optional keys (`version_file`, `[release.hooks]`, `[release].context` / `prompt`, …) present-but-commented with a one-line explanation each. The project tunes mint by uncommenting rather than reading docs." "No hook/prompt files scaffolded by default — the commented config shows hook examples inline, and a `[release].prompt` override file is only *mentioned* in a comment (not created)." "No project auto-detection — mint does not sniff `package.json`/etc. to pre-fill a build hook; that guesswork can surprise. A clean commented template is more honest." The schema block (spec "Config Format & Schema") supplies exact key names, defaults, and example values.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "`mint init` Scaffolding", "Config Format & Schema → Shape".

## mint-release-tool-6-6 | approved

### Task mint-release-tool-6-6: `release` shim generation

**Problem**: Each project carries a tiny committed `release` script so `./release` works for anyone who clones, delegating to the globally-installed `mint` — this is the muscle-memory activation model. If `mint` isn't installed, the shim must fail helpfully (point at the brew install) rather than with a cryptic "command not found". There is no shim content yet.

**Solution**: A generator returning the executable `release` shim content: it execs `mint release "$@"`; if `mint` is absent it prints the install hint (`brew install leeovery/tools/mint`) and exits non-zero; and the file is written with an executable mode.

**Solution note**: This task produces the *shim content* (a string) and defines its file mode. Writing it to disk with idempotency/`--force` is the `mint init` command (6-7). The shim delegates to `mint release`, which is built across Phases 1–5 — this task does not implement the release pipeline.

**Outcome**: A generator (e.g. `initgen.ReleaseShim() string`) returns a small POSIX shell script that, when `mint` is on `PATH`, execs `mint release "$@"` (forwarding all args, e.g. `./release -m`); when `mint` is absent, prints `brew install leeovery/tools/mint` and exits with a non-zero status. The intended file mode is executable (e.g. `0755`).

**Do**:
- In `internal/initgen`, implement `ReleaseShim() string` returning the script. Suggested content:
  ```sh
  #!/usr/bin/env sh
  if ! command -v mint >/dev/null 2>&1; then
    echo "mint is not installed. Install it with: brew install leeovery/tools/mint" >&2
    exit 1
  fi
  exec mint release "$@"
  ```
  - Use `exec` so the shim replaces itself with `mint release` (clean signal/exit-code propagation).
  - Forward all arguments with `"$@"` so `./release -m`, `./release --set-version 2.0.0`, etc. map cleanly to `mint release …`.
  - Detect mint via `command -v mint` (portable) and, when absent, print the exact install hint `brew install leeovery/tools/mint` to stderr and `exit 1` (non-zero).
- Expose the intended **file mode** the writer (6-7) will use — executable, e.g. a constant `ShimMode = 0o755`. The actual `os.WriteFile`/`chmod` happens in 6-7; this task owns the content and the documented mode.
- Keep the generator pure (returns a string); no IO here.
- Test the content directly: assert it contains the shebang, the `command -v mint` guard, the exact install-hint string `brew install leeovery/tools/mint`, a non-zero `exit`, and `exec mint release "$@"` forwarding args. (Optionally, where the test environment allows, run the script with a stubbed `PATH` to assert the absent-mint branch exits non-zero and prints the hint — but the string assertions are the primary, host-independent check.)

**Acceptance Criteria**:
- [ ] The generator returns a POSIX `sh` shim with a shebang.
- [ ] When `mint` is present, the shim execs `mint release "$@"` forwarding all arguments.
- [ ] When `mint` is absent, the shim prints the `brew install leeovery/tools/mint` hint and exits non-zero.
- [ ] The shim uses `exec` so it replaces itself with `mint release` (clean exit-code propagation).
- [ ] The intended file mode is executable (e.g. `0755`), exposed for the writer in 6-7.

**Tests**:
- `"the shim contains a shebang and execs mint release with forwarded args"`
- `"the shim guards on command -v mint"`
- `"the shim prints the brew install hint and exits non-zero when mint is absent"`
- `"the shim mode constant is executable (0755)"`

**Edge Cases**:
- Shim file mode executable.
- `mint` present → exec `mint release "$@"` passing args.
- `mint` absent → brew install hint + non-zero exit.

**Context**:
> "The `release` shim — a tiny executable committed to the repo so `./release` works for anyone who clones. It execs `mint release "$@"`; if mint isn't installed it prints the install hint (`brew install leeovery/tools/mint`) and exits non-zero." Activation model: "each project carries a committed `release` shim that delegates to the globally-installed `mint`; `mint init` scaffolds the per-project config and shim." "The per-project shim is `release`, so `./release -m` maps cleanly to `mint release -m`." Install: "`brew install leeovery/tools/mint`."

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "`mint init` Scaffolding", "Settled foundations → Activation model", "CLI Surface & Flags".

## mint-release-tool-6-7 | approved

### Task mint-release-tool-6-7: `mint init` command (drops both files, idempotent/non-clobbering, --force)

**Problem**: The template (6-5) and shim (6-6) content exist but nothing writes them. `mint init` is the activation command: it drops **both** files — `.mint.toml` and the executable `release` shim — at the repo root, and it must be safe to re-run (an existing file is skipped with a notice, never clobbered) with a `--force` escape to regenerate. It must write at the repo root resolved via `git rev-parse --show-toplevel`, scaffold no hook/prompt files, and do no project auto-detection.

**Solution**: The `mint init` command: resolve the repo root via the `CommandRunner`, then write `.mint.toml` (6-5 content) and the `release` shim (6-6 content, executable mode) at the root; skip any file that already exists with a notice through the Presenter; `--force` overwrites both; scaffold nothing else.

**Outcome**: In a repo with neither file, `mint init` creates both at the repo root (the shim executable). If one exists, only the missing one is created and the existing one is reported as skipped. If both exist, both are skipped with notices and nothing is overwritten. `mint init --force` regenerates both. No hook scripts, prompt files, or auto-detected content are produced.

**Do**:
- Add `mint init` to the CLI (`cmd/mint`), with a `--force` flag. Wire it through the same `CommandRunner` and `Presenter` seams used by the rest of the engine so it is testable with `FakeRunner` + `RecordingPresenter`.
- Resolve the repo root via the existing repo-root resolver (Phase 1 task 1-4, `git rev-parse --show-toplevel` through the `CommandRunner`); write both files at that root, never the invocation directory.
- For each of the two targets — `{root}/.mint.toml` (content from `initgen.MintTOML()`, 6-5) and `{root}/release` (content from `initgen.ReleaseShim()`, 6-6, mode `0755`):
  - If the file **does not exist** → write it (and `chmod` the shim executable).
  - If the file **exists** and `--force` is **not** set → skip it and emit a notice via the Presenter (e.g. `".mint.toml already exists — skipped (use --force to overwrite)"`). The two targets are independent: one existing does not block the other being created.
  - If `--force` is set → write (overwrite) regardless.
- Report per-file outcomes (created / skipped / overwritten) through the Presenter so a test can assert exactly what happened.
- **Scaffold nothing else**: no `.release/hooks/` directory, no prompt-override file, no example hook scripts, no project auto-detection. The commented template (6-5) is the only documentation surface.
- Emit a clear summary (e.g. which files were created vs skipped) via the Presenter.
- Test with a temp-dir repo root (the `FakeRunner` returns the temp dir for `git rev-parse --show-toplevel`) and assert: both-absent → both created (shim mode executable); one-present → only the other created + a skip notice; both-present → both skipped, contents unchanged; `--force` → both rewritten.

**Acceptance Criteria**:
- [ ] With neither file present, `mint init` creates both `.mint.toml` and `release` at the repo root.
- [ ] The written `release` shim has an executable file mode.
- [ ] If only one file exists, the other is created and the existing one is skipped with a notice.
- [ ] If both exist, both are skipped with notices and neither is overwritten.
- [ ] `mint init --force` regenerates (overwrites) both files.
- [ ] Files are written at the `git rev-parse --show-toplevel` root, not the invocation directory.
- [ ] No hook scripts, prompt files, or auto-detected content are scaffolded.

**Tests**:
- `"it creates both files at the repo root when neither exists"`
- `"the written release shim is executable"`
- `"it skips an existing file with a notice and creates only the missing one"`
- `"it skips both with notices when both exist (no overwrite)"`
- `"--force regenerates both files"`
- `"it writes at the repo root, not the invocation directory"`
- `"it scaffolds no hook or prompt files"`

**Edge Cases**:
- Neither file exists → both created.
- One exists → only the other created + notice.
- Both exist → both skipped with notices.
- `--force` regenerates.
- Files written at repo root.
- No hook/prompt files scaffolded.

**Context**:
> `mint init` "activates mint in a project by dropping in two files: `.mint.toml` … and the `release` shim." Behaviour: "Idempotent / non-clobbering — an existing `.mint.toml` or `release` is skipped with a notice; `--force` regenerates." "No hook/prompt files scaffolded by default." "No project auto-detection." Location: config "Location: `.mint.toml` at the repo root. mint resolves the root via `git rev-parse --show-toplevel`." CLI: `mint init    scaffold .mint.toml (+ release shim)`. The `.release/hooks/` directory convention is explicitly YAGNI ("there is no separate `.release/hooks/` directory convention").

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "`mint init` Scaffolding", "Config Format & Schema → Location", "CLI Surface & Flags".

## mint-release-tool-6-8 | approved

### Task mint-release-tool-6-8: `mint version` / `mint --version`

**Problem**: The CLI surface lists `mint version` (and the `--version` flag) to print mint's own version — standard CLI convention, and the user-facing way to confirm which mint is installed (relevant because the `release` shim delegates to whatever `mint` is on `PATH`). Phase 1 reserved the `version` surface but did not finalise it; it must be completed here so both the subcommand and the flag print the identical version string.

**Solution**: Finalise `mint version` (subcommand) and `mint --version` (global flag) so both print mint's own version, sourced from a single version constant/variable, yielding an identical string from either entry point.

**Solution note**: This is mint's *own* tool version (the binary), distinct from the *release* version computed from git tags (Stage 1). This task does not touch release-version logic.

**Outcome**: `mint version` and `mint --version` both print mint's own version (e.g. `mint 1.4.0` or the bare version string), and the two outputs are identical because they read the same source. The value is defined in one place (a package-level `Version` variable, suitable for build-time injection via `-ldflags -X`).

**Do**:
- Define mint's own version in one place — a package-level variable, e.g. `var Version = "dev"` in the `cmd/mint` (or a small `internal/buildinfo`) package — overridable at build time via `-ldflags "-X ...Version=<v>"`. Both entry points read this single source so they can never diverge.
- Implement the **`version` subcommand** (`mint version`) to print the version string. Use the same package's parsing/dispatch established in Phase 1 (where the `version` surface was reserved); finalise its action here.
- Implement the **`--version` global flag** (`mint --version`) to print the identical string and exit (standard convention: handled before/independently of any subcommand).
- Ensure both produce the **exact same** output string (define a single `versionString()` helper both call) so a test can assert equality directly.
- Keep it independent of the release pipeline — printing the tool version requires no git/repo resolution and must work outside a git repo.
- Test: capture output of the `version` subcommand and of the `--version` flag through the same CLI entry; assert each prints the configured version and that the two are byte-identical. (Route output through the established output seam / Presenter or a writer the test can capture.)

**Acceptance Criteria**:
- [ ] `mint version` prints mint's own version.
- [ ] `mint --version` prints mint's own version.
- [ ] Both print the **identical** string (same single source).
- [ ] The version value is defined in one place and is build-time injectable (`-ldflags -X`).
- [ ] Printing the version works without a git repo (no repo-root resolution required).

**Tests**:
- `"the version subcommand prints mint's own version"`
- `"the --version flag prints mint's own version"`
- `"the subcommand and the flag print an identical string"`
- `"it prints the version outside a git repository"`

**Edge Cases**:
- `mint version` subcommand.
- `mint --version` flag.
- Both print the identical version string.

**Context**:
> CLI Surface: `mint version    print mint's own version`. "`mint version` / `mint --version` print mint's own version (standard convention)." This is mint's *own* version, not the release version (which is sourced from git tags, Stage 1: "the tag is the real version"). Phase 1 "reserved `mint version` and the `release` subcommand surface" (task 1-11 note) but only finalises `mint release`; this task finalises `mint version` / `--version`.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "CLI Surface & Flags → Commands".
