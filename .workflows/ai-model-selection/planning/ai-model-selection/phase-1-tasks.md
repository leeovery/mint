---
phase: 1
phase_name: Config resolution layer
total: 8
---

## ai-model-selection-1-1 | approved

### Task ai-model-selection-1-1: Pin the shipped default AI command to `claude -p --model sonnet`

**Problem**: The shipped default command is `claude -p`, which inherits whatever default model the operator's Claude CLI is set to — an external, mutable setting mint does not control, so output quality, cost, and latency vary silently. The default must pin a model so behaviour is predictable for zero-config operators. This also establishes the single canonical constant that every other site (transport, init template, README) will derive from rather than re-typing the literal.

**Solution**: Change the `defaultAICommand` constant in `internal/config/config.go` from `"claude -p"` to `"claude -p --model sonnet"`, and export it so later phases can source the value from `config` rather than re-typing it (Phase 3's init template requires "sourced from the config constant, not re-typed"). Update the constant's WHY-comment and the `Config.AICommand` doc comment to state the new pinned default and the canonical-source role.

**Outcome**: With no `.mint.toml`, `config.Load` returns `AICommand == "claude -p --model sonnet"`; the value is exported from `config` as the single canonical source; all config-package callers resolve to the new pinned default.

**Do**:
- In `internal/config/config.go`, rename the unexported `defaultAICommand` const to an exported name (`DefaultAICommand`) and change its value to `"claude -p --model sonnet"`. Update every in-package reference (`defaults()` ~line 219, `resolveAICommand` ~line 417) to the exported name.
- Rewrite the const's WHY-comment (~lines 69-74) so it states the pinned `--model sonnet` default and notes this is the CANONICAL source every other site derives from (the transport's duplicate and initgen's literal are removed/sourced from here in later phases). Pin the alias rationale: the alias `sonnet` tracks the current version automatically rather than baking a full versioned ID that goes stale.
- Update the `Config.AICommand` doc comment (~lines 91-95) so it reflects the new default value. Do NOT yet rewrite the parts of that comment describing transport re-defaulting of empty values — that wording changes in Task 1-4 when the accessor takes over blank-skipping. Keep this task's comment edit limited to the default-value change.
- Update the existing config test pins that assert the old `"claude -p"` default: `TestLoad_AbsentAICommand_DefaultsToClaudeP` (~line 837), and the `ai_command = "claude -p"` line inside the `TestLoad_FullyValidFile_LoadsWithoutError` fixture (~line 801) is a VALID explicit value and may stay as-is (it asserts a load succeeds, not the default) — only the default-assertion test must change. Rename the default-assertion test to reflect the new value.

**Acceptance Criteria**:
- [ ] The exported constant equals `"claude -p --model sonnet"`.
- [ ] `config.Load` on an empty dir returns `cfg.AICommand == "claude -p --model sonnet"`.
- [ ] The constant is exported (referenceable from another package) so Phase 3's initgen can source it without re-typing.
- [ ] The const WHY-comment and `Config.AICommand` doc comment state the new default value and the canonical-source role.
- [ ] `go build ./...`, `gofmt -l .` (prints nothing), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues) all pass.

**Tests**:
- `"it returns claude -p --model sonnet as the default ai_command when no .mint.toml exists"`
- `"it carries an explicit top-level ai_command through verbatim (the pinned default is the floor, not a forced value)"` — existing `TestLoad_ExplicitAICommand_Honoured` continues to pass unchanged.

**Edge Cases**: none (per the task table).

**Context**:
> Spec — Pinned default model: "The shipped default command becomes `claude -p --model sonnet` (today: `claude -p`)." Alias form, not a full model ID — "the alias tracks the current version automatically." Default model is Sonnet; Opus is reserved for explicit per-verb opt-in. "Not a breaking change in practice … mint is a brand-new project with no users yet … No release-note callout and no runtime signal are required."
> Spec — De-duplication target: "`defaultAICommand = "claude -p"` is currently duplicated across `internal/config/config.go`, `internal/ai/transport.go`, and `internal/initgen/initgen.go` … After this work the value lives canonically in `internal/config` and the other sites derive from it." Exporting the constant in this task is the enabling step; the transport-side deletion is Phase 2 and the initgen sourcing is Phase 3.
> Planning Phase 1 goal: "pin the shipped default command to `claude -p --model sonnet` as the canonical constant that all other sites will derive from." A canonical constant other packages derive from must be exported — hence the rename to an exported identifier here, before Phase 3 consumes it.

**Spec Reference**: `.workflows/ai-model-selection/specification/ai-model-selection/specification.md` — Pinned default model; Single source of truth for config defaults (De-duplication target).

## ai-model-selection-1-2 | approved

### Task ai-model-selection-1-2: Define the typed closed verb enum (release, commit; no regenerate)

**Problem**: The layered accessors `AICommandFor(verb)` / `TimeoutFor(verb)` (Tasks 1-4, 1-7) need a `verb` parameter. A raw string would let a typo silently fall through to the shared baseline, and would leave a "regenerate" value reachable — but regenerate must route through `[release]`, not its own table. A typed, closed enum with exactly two values makes the accessor's domain exhaustive by construction and the regenerate routing un-missable.

**Solution**: Define a typed, closed enum in `internal/config` with exactly two exported constant values — one per verb table (`[release]`, `[commit]`) — and no `regenerate` value and no externally-constructible/zero-value member that would fall through. The type is a named string-or-int alias kept unconstructible from outside via unexported underlying representation where practical; expose the two constants for callers.

**Outcome**: `config` exports a `Verb` type with exactly two values (e.g. `VerbRelease`, `VerbCommit`); there is no `regenerate` value; the type is the parameter Tasks 1-4 and 1-7 accept; a test enumerates the closed set and proves there is no third value.

**Do**:
- In `internal/config/config.go` (or a small sibling file in the same package), define an exported `Verb` type. Use a named type over an unexported underlying int (e.g. `type Verb int`) so the zero value is one of the two real verbs OR — preferred — so that callers must use a named constant; choose the representation that makes "no unrecognized verb case to handle" true by construction. Define exactly two exported constants, `VerbRelease` and `VerbCommit`.
- Add a WHY-comment on the type stating: exactly two values (one per verb table), NO `regenerate` value (regenerate rides on `[release]` so `internal/engine/regenerate_fresh.go` can only pass `VerbRelease`), and that the closed enum gives compile-time safety against string-typo fall-through and makes the accessor domain exhaustive — there is no "unrecognized verb" branch.
- Do NOT add the accessors here (they arrive in 1-4 / 1-7). This task defines only the type and constants plus a test pinning the closed set. If the chosen representation makes a zero-value verb meaningful, ensure tests assert which verb that is and that it is one of the two real verbs (no silent third/empty member).

**Acceptance Criteria**:
- [ ] An exported `Verb` type exists in `internal/config` with exactly two exported constants (`VerbRelease`, `VerbCommit`).
- [ ] There is no `regenerate` (or other third) enum value.
- [ ] A test enumerates the two values and asserts no additional reachable member maps to a distinct table (no unknown/zero-value verb silently distinct from the two).
- [ ] The type's WHY-comment records the no-regenerate rationale and the exhaustive-by-construction guarantee.
- [ ] `go build ./...`, `gofmt -l .` (prints nothing), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues) all pass.

**Tests**:
- `"it defines exactly two verb values, release and commit"`
- `"it has no regenerate verb value"` — assert by enumerating the closed set; no third constant exists (a compile-time/structural proof, e.g. a table listing the only two values the accessor tests will exercise).
- `"the verb type's zero value resolves to a real verb table, never an unknown/third one"` (only if the chosen representation has a meaningful zero value).

**Edge Cases**:
- No `regenerate` value (per the task table) — regenerate must route through `[release]`.
- No unknown / zero-value verb falling through to the shared baseline — the domain is exhaustive by construction, so there is no "unrecognized verb" case for the accessors to handle.

**Context**:
> Spec — Single source of truth for config defaults: "The `verb` parameter is a typed, closed enum defined in `internal/config` — not a raw string — with exactly two values, one per verb table (`[release]`, `[commit]`). A typed enum gives compile-time safety (no string typos silently falling through to the shared baseline) and makes the regenerate routing un-missable: there is no `regenerate` value, so `internal/engine/regenerate_fresh.go` can only pass the release verb. The accessor's domain is therefore exhaustive by construction — there is no 'unrecognized verb' case to handle. Exact type and constant names are a planning/impl detail."
> Spec — Verb config space: "exactly two tables: `[release]` and `[commit]` … `regenerate` is not a separate verb … there is no `[regenerate]` table."

**Spec Reference**: `.workflows/ai-model-selection/specification/ai-model-selection/specification.md` — Single source of truth for config defaults (typed verb enum); Config schema: per-verb `ai_command` override (Verb config space).

## ai-model-selection-1-3 | approved

### Task ai-model-selection-1-3: Add the per-verb `ai_command` override to the schema and strict decoding

**Problem**: Today `ai_command` lives only at the top level. To let each verb run a different AI command, `[release].ai_command` and `[commit].ai_command` must become decodable per-verb override keys. Strict decoding (`DisallowUnknownFields`) will reject them as unknown keys unless they are added to both verb shape structs, and a wrong TYPE on them must surface a friendly, mapped error rather than the raw decoder text.

**Solution**: Add an optional `ai_command` field to both `releaseShape` and `commitShape` (as `*string`, so absent is distinguishable from an explicit empty/blank value — both Task 1-4 cases), carry the raw values onto the `Release`/`Commit` structs verbatim, and add `typeErrorMessages` entries for `release.ai_command` and `commit.ai_command`. Resolution of the override chain is Task 1-4; this task only makes the keys decode, carry through verbatim, and fail loud on a wrong type.

**Outcome**: `[release].ai_command` and `[commit].ai_command` decode and are carried raw onto the config structs (nil when absent); strict decoding accepts them; a genuine TOML type mismatch on either surfaces a mapped friendly message at `Load`; unknown sibling keys in those tables are still rejected.

**Do**:
- Add `AICommand *string` to `releaseShape` (~line 250) with `toml:"ai_command"` and to `commitShape` (~line 245) with `toml:"ai_command"`. Use `*string` (not plain string) because Task 1-4's blank-skip needs absent (nil) distinct from explicit empty/blank.
- Add raw-carrying fields onto the `Release` and `Commit` structs so the resolver in 1-4 can read them. Carry the pointer's value verbatim (nil → the field stays at the absent sentinel; non-nil → the literal string, including blank/whitespace — blank-skipping is the accessor's job in 1-4, NOT here). Wire them through `resolveRelease` (~line 431) and `resolveCommit` (~line 459). Note `resolveCommit` currently does a direct `Commit(shape)` conversion — adding a `*string` shape field that maps to a carried field on `Commit` breaks field-identity, so convert field-by-field (mirror `resolveRelease`'s explicit copy).
- Add `typeErrorMessages` entries (~line 362): `"releaseShape.AICommand": "release.ai_command must be a string"` and `"commitShape.AICommand": "commit.ai_command must be a string"`. (Note the go-toml/v2 quirk pinned by `TestLoad_CommitBooleanValue_StillFailsLoud`: a boolean-into-string mismatch emits no struct-field path, so it falls back to the library description but still fails loud — mirror that expectation, do not over-assert the mapped message for the boolean case.)
- Do NOT seed `defaults()` for these per-verb keys — the absent baseline is "no override" (nil), which falls through in 1-4. The top-level shared `ai_command` keeps its `defaults()` seeding.

**Acceptance Criteria**:
- [ ] `[release].ai_command` and `[commit].ai_command` decode without strict-decode rejection.
- [ ] Both are carried raw onto the config structs (absent → nil/absent sentinel; present → the literal string verbatim, blank or not).
- [ ] An integer assigned to either key surfaces the mapped friendly message naming the key and `string` type at `Load`.
- [ ] A boolean assigned to either still fails loud (library fallback text, key still visible) — no silent accept.
- [ ] An unknown sibling key inside `[release]` / `[commit]` is still rejected naming the key and table (existing strict-decode behaviour intact).
- [ ] `go build ./...`, `gofmt -l .` (prints nothing), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues) all pass.

**Tests**:
- `"it decodes [release].ai_command and carries it raw onto the config"`
- `"it decodes [commit].ai_command and carries it raw onto the config"`
- `"it distinguishes an absent per-verb ai_command (nil) from an explicit blank one"` — write `[release]\nai_command = ""` and assert the carried value is the explicit empty string, not the absent sentinel.
- `"it rejects an integer [release].ai_command naming the key and string type"`
- `"it rejects an integer [commit].ai_command naming the key and string type"`
- `"it still fails loud for a boolean per-verb ai_command (key visible)"`
- `"it still rejects an unknown sibling key in [release] / [commit] after adding ai_command"`

**Edge Cases**:
- Genuine TOML type mismatch still a strict decode error at `Load` (per the task table) — a non-string value fails loud, distinct from a value-level fall-through.
- Unknown sibling keys still rejected — adding a recognised per-verb key must not loosen `DisallowUnknownFields` for the rest of the table.

**Context**:
> Spec — Config schema: per-verb `ai_command` override: "`[release].ai_command` and `[commit].ai_command` … The new per-verb keys must be added to both verb shape structs with `typeErrorMessages` entries, otherwise strict decoding (`DisallowUnknownFields`) rejects them. `[commit]` simply mirrors `[release]` — same override keys, same resolution, no commit-specific asymmetry."
> As-built: `releaseShape`/`commitShape` use the decode-onto-pre-seeded-defaults idiom; `*string` is the established absent-vs-explicit pattern (cf. `MaxDiffLines *int`, `Publish *bool`). `typeErrorMessages` maps the Go struct-field path (e.g. `commitShape.Context`) to the friendly message; `translateTypeError` matches `field + " "` in the DecodeError text. The boolean-into-string go-toml quirk is documented in `TestLoad_CommitBooleanValue_StillFailsLoud`.
> NOTE — `Commit` struct doc comment: this task ADDS `[commit].ai_command`, which begins contradicting the existing `Commit` doc comment ("Deliberately NOT added for commit"). The CLAUDE.md same-change comment-reconciliation obligation for that doc comment is scoped to Phase 3 by the plan (it is bundled with the README/initgen surfacing) — do NOT edit the `Commit` doc comment here beyond what is mechanically required to compile; the full reconciliation is Phase 3's acceptance item. Leave a focused note only if compilation forces a touch.

**Spec Reference**: `.workflows/ai-model-selection/specification/ai-model-selection/specification.md` — Config schema: per-verb `ai_command` override; Cross-spec reconciliation (commit spec).

## ai-model-selection-1-4 | approved

### Task ai-model-selection-1-4: Add the layered `AICommandFor(verb)` accessor with multi-layer trim-and-skip

**Problem**: Consumers need a single resolved AI command per verb without re-implementing the fallback chain or the blank-skip logic. Resolution must be `[verb].ai_command → top-level shared ai_command → shipped default`, trimming each candidate and skipping it when blank/whitespace at EVERY layer — so a blank per-verb value falls to shared, a blank shared falls to the floor, and the result is never empty. The transport's old single blank-re-default only ever saw one already-resolved value; that blank-detection must move into config and apply at all layers, folding away the now-insufficient `resolveAICommand` helper.

**Solution**: Add an exported `AICommandFor(verb Verb) string` method on `Config` that walks the per-key chain, trimming each candidate with `strings.TrimSpace` and skipping it when empty, falling to the shared top-level `AICommand`, then to the shipped `DefaultAICommand` floor. Fold the existing `resolveAICommand` (today nil-vs-present only) into / replace it with this accessor so blank-skipping lives in exactly one place across all layers — but keep `Load` carrying the raw top-level `ai_command` verbatim (an explicit blank shared value must reach the accessor as blank, so the accessor — not `Load` — applies the floor).

**Outcome**: `cfg.AICommandFor(VerbRelease)` / `cfg.AICommandFor(VerbCommit)` always return a non-empty command; a blank `[verb].ai_command` falls to shared; a blank shared falls to the floor; a top-level `ai_command = ''` resolves to the shipped default; `resolveAICommand`'s old job is subsumed by the accessor (one place for blank-skipping).

**Do**:
- Add `func (c Config) AICommandFor(verb Verb) string` to `internal/config/config.go`. Build the candidate order: the per-verb override for `verb` (read `c.Release`'s or `c.Commit`'s carried `ai_command` field from Task 1-3 — select by verb), then `c.AICommand` (shared top-level), then `DefaultAICommand` (floor). For each candidate, `strings.TrimSpace`; return the FIRST candidate whose trimmed form is non-empty. Return the RAW (untrimmed) candidate value — preserve the operator's exact string (the transport whitespace-splits it; do not collapse internal spacing) — trim only for the empty-check. The floor is always non-empty, so the method never returns "".
- Decide the top-level blank handling: `Load` must carry an explicit top-level `ai_command = ''` as the empty string (not re-defaulted at Load) so the accessor's trim-and-skip is what falls it through to the floor. Today `resolveAICommand` re-defaults only nil (absent) and carries an explicit empty verbatim — that already carries `''` through, so `Load` keeps seeding `defaults()` (absent → pinned default) while an explicit `''` overwrites to empty and reaches the accessor as empty. Confirm this and remove/repoint `resolveAICommand` so the single blank-skip lives in the accessor: either delete `resolveAICommand` and have `Load` assign `shape.AICommand` via a tiny absent→default helper (keeping the pinned default for the ABSENT case) OR fold its nil-default into the accessor path. Whichever is chosen, blank/whitespace detection must exist in exactly ONE place (the accessor), and an ABSENT top-level key must still resolve to the pinned default for any code reading `cfg.AICommand` directly during the transition.
- Update the `Config.AICommand` doc comment (and remove the stale "the transport re-defaults an explicit empty value" wording now that blank-skipping is config's job) and the `resolveAICommand` comment if the helper survives, so comments match as-built.
- Verify per-key independence is preserved by construction: `AICommandFor` reads only `ai_command` candidates and never consults `timeout`.

**Acceptance Criteria**:
- [ ] `cfg.AICommandFor(verb)` returns `[verb].ai_command` when that override is present and non-blank.
- [ ] A blank/whitespace `[verb].ai_command` falls through to the shared top-level `ai_command`.
- [ ] A blank/whitespace shared `ai_command` falls through to the shipped default floor.
- [ ] A top-level `ai_command = ''` (with no per-verb override) resolves to `DefaultAICommand`.
- [ ] The accessor NEVER returns an empty string for either verb under any input.
- [ ] The returned command preserves the operator's raw string (trim is used only for the empty-check, not to mutate the value).
- [ ] Blank/whitespace detection exists in exactly one place; `resolveAICommand`'s old single-layer behaviour is subsumed (helper deleted or repointed).
- [ ] `go build ./...`, `gofmt -l .` (prints nothing), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues) all pass.

**Tests**:
- `"it resolves a present non-blank [release].ai_command override"`
- `"it resolves a present non-blank [commit].ai_command override"`
- `"it falls a blank [verb].ai_command through to the shared top-level ai_command"`
- `"it falls a whitespace-only shared ai_command through to the shipped default floor"`
- `"it resolves a top-level ai_command = '' to the shipped default"`
- `"it never returns an empty ai_command for either verb"`
- `"it resolves both verbs to the pinned default when no .mint.toml exists"` (zero-config floor).
- `"it preserves the operator's raw command string (no internal-spacing collapse)"`

**Edge Cases**:
- Blank/whitespace `[verb].ai_command` falls to shared (per the task table).
- Blank/whitespace shared falls to floor.
- Top-level `ai_command = ''` falls to shipped default.
- Accessor never yields empty — the floor guarantees a valid command, which is what makes the transport's old empty→re-default / empty→fail-loud path unreachable (removed in Phase 2).

**Context**:
> Spec — Resolution value semantics (`ai_command`): "Blank / whitespace / invalid / missing at a layer drops through to the next layer. The shipped default is the floor, so resolution always yields a valid command — `ai_command` is never empty. Even a top-level `ai_command = ''` falls through to the shipped default. Blank/whitespace detection lives in the config accessor, applied at every layer. `AICommandFor` trims each candidate and skips it when empty … This multi-layer trim-and-skip replaces the transport's old single blank-re-default … the whitespace-blank detection moves out of the transport into config. The existing `resolveAICommand` helper (today only nil-vs-present) is folded into / replaced by the accessor so blank-skipping happens in exactly one place across all layers."
> Spec — Resolution order: "`[verb].ai_command → top-level shared ai_command → shipped default`."
> Spec — Single source of truth: accessors "resolve `verb override → shared top-level → default` … The fallback chain lives in exactly one place; consumers just ask for the resolved value." No reflection / no global service-locator — a typed `Config` value with accessor methods.
> As-built: `resolveAICommand` (~line 413) currently returns `*v` when non-nil else `defaultAICommand`; it does NOT trim. The transport's `NewTransport` (~lines 80-83) does the single `strings.TrimSpace(command) == "" → defaultAICommand` blank-default that this accessor supersedes (transport-side deletion is Phase 2).

**Spec Reference**: `.workflows/ai-model-selection/specification/ai-model-selection/specification.md` — Resolution value semantics (`ai_command`); Resolution order; Single source of truth for config defaults.

## ai-model-selection-1-5 | approved

### Task ai-model-selection-1-5: Add the net-new top-level shared `timeout` key to the schema

**Problem**: `timeout` today exists only as `defaultTimeout` in the transport and is never populated from config — every wiring site leaves `ai.Config.Timeout` zero and relies on the transport's self-default. To let operators tune the per-attempt deadline (and to let a slow verb raise it), `timeout` must become a real top-level shared config key. It needs full new-key treatment, and — critically — it must distinguish ABSENT from an explicit ZERO (zero means "no deadline", a conscious, honored value), so a plain decode field cannot be used.

**Solution**: Add a top-level shared `timeout` key to the schema, decoded into a pointer field so absent (nil) is distinguishable from an explicit `0`. Seed the shipped 60s default in `defaults()`. Add a `typeErrorMessages` entry. Carry the resolved/absent value onto `Config` such that the accessor in Task 1-7 can apply the absent-vs-zero-vs-negative semantics. Resolution (the value semantics) is Task 1-7; this task adds the schema field, the default seed, strict-decode acceptance, and the absent-vs-zero plumbing.

**Outcome**: A top-level `timeout` key decodes; absent vs explicit `0` is distinguishable on the config; `defaults()` seeds 60s; a genuine TOML type mismatch fails loud at `Load`; zero-config still yields the 60s value through Task 1-7's accessor.

**Do**:
- REPRESENTATION DECISION — use **integer seconds** (`timeout = 60`), decoded as `*int` in `fileShape`. Justification (grounded in as-built decoding style): every existing scalar config key with an absent-vs-explicit-zero need uses a `*T` pointer in the shape (`MaxDiffLines *int`, `Publish *bool`, `Changelog *bool`) and re-defaults nil at resolve time; `max_diff_lines` is already a bare integer key with exactly this `*int` absent-vs-zero idiom and friendly type-error mapping. Integer seconds reuses that idiom verbatim: nil = absent, `0` = explicit zero, negative = value-invalid. It also gives the spec's stated behaviour for FREE — "a non-integer TOML value is a strict decode (type) error at `Load`" — because `*int` decoding rejects a non-int at decode time, exactly like `max_diff_lines`. A string-duration representation would require a custom decode, would make `"fast"`/`"-5s"` decode as a valid string (deferring all value-invalid detection to resolution time, more bespoke code), and has no precedent in this schema. Integer seconds is the minimal, idiom-consistent choice; the unit (seconds) is documented in the init template / README (Phase 3). Carry this choice into Tasks 1-6 and 1-7.
- Add `Timeout *int` to `fileShape` (~line 233) with `toml:"timeout"`.
- Add a `Timeout` field to `Config` to carry the resolved top-level value. Because the absent-vs-zero distinction must survive to Task 1-7's accessor, carry it as a `*time.Duration` (or `*int` seconds) on `Config` — pick the representation that lets `TimeoutFor` see nil(absent) vs 0(explicit) vs positive vs the dropped-negative. Recommended: store the resolved top-level as `*time.Duration` on `Config` (nil = absent/floor-eligible-at-accessor; non-nil = the operator's explicit value, including `0` and any negative, with negative-drop handled in 1-7). Convert seconds→`time.Duration` at the boundary (`time.Duration(n) * time.Second`).
- Seed `defaults()` (~line 203): the SHIPPED default is 60s. Add a `DefaultTimeout` exported constant in `config` set to `60 * time.Second` (canonical, mirroring the transport's `defaultTimeout` which Phase 2 deletes). Decide whether `defaults()` seeds the `*` field with a pointer to 60s OR whether 60s is applied as the accessor floor in 1-7. To keep "absent top-level resolves to 60s" working for any direct reader and to mirror how `defaults()` seeds the other top-level keys, seed the canonical 60s — but ensure an explicit top-level `0` in the file still overwrites that seed to the explicit-zero value (the `*int` decode-onto-pre-seed must let an explicit `0` win). If pre-seeding the pointer makes absent-vs-zero ambiguous, instead leave `defaults()` carrying the floor only and have the accessor apply 60s when the chain is exhausted — document which mechanism is chosen and prove absent-vs-zero with a test.
- Add `typeErrorMessages` entry: `"fileShape.Timeout": "timeout must be an integer (seconds)"`.
- Update the package doc comment's enumeration of shared top-level keys (~lines 5-11) to include `timeout` alongside `ai_command`, `diff_exclude`, `max_diff_lines`.

**Acceptance Criteria**:
- [ ] A top-level `timeout = 90` decodes and is carried onto `Config` as 90 seconds.
- [ ] An absent top-level `timeout` is distinguishable from an explicit `timeout = 0` on the config (nil vs explicit-zero plumbing intact).
- [ ] With no `.mint.toml`, the resolved top-level timeout is the shipped 60s (via the seed or the accessor floor — whichever mechanism is chosen).
- [ ] A non-integer `timeout` (e.g. `timeout = "fast"`) is rejected at `Load` with a mapped friendly message naming the key and integer/seconds type.
- [ ] A `DefaultTimeout` exported constant equals `60 * time.Second`.
- [ ] `go build ./...`, `gofmt -l .` (prints nothing), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues) all pass.

**Tests**:
- `"it decodes a top-level timeout and carries it as seconds"`
- `"it distinguishes an absent top-level timeout from an explicit timeout = 0"`
- `"it resolves the shipped 60s default when no .mint.toml exists"`
- `"it rejects a non-integer top-level timeout naming the key and integer type"`
- `"it carries an explicit top-level timeout = 0 through as a distinguishable explicit zero (not coerced to the default)"`

**Edge Cases**:
- Absent vs explicit zero distinguished (per the task table) — the `*` field is the mechanism.
- Genuine TOML type mismatch a strict decode error at `Load` — integer-seconds makes a non-int a decode-time error (chosen representation rationale above).
- Zero-config resolves to 60s.

**Context**:
> Spec — Config schema: `timeout` key: "`timeout` is NET-NEW config surface, not a relocated default … This work therefore introduces a brand-new top-level shared `timeout` key … all needing full new-key treatment: schema struct field (top-level + both verb shapes), `typeErrorMessages` entry, `defaults()` seeding at the current 60s value, absent-vs-zero / duration decoding. Shipped default = 60s (the transport's current per-attempt deadline value), seeded in `internal/config`."
> Spec — Deferred to planning (the representation/units): "Whatever representation is chosen must preserve the value semantics … distinguishing absent from an explicit zero, honoring zero, and treating negative/unparseable values as value-invalid. Int seconds — a non-integer TOML value is a strict decode (type) error at `Load`; a negative integer is a value-invalid drop-through; absent vs zero is a nil pointer vs `0`." This task adopts integer seconds; the rationale (idiom-consistency with `max_diff_lines`/the `*T` pattern, free strict-decode-on-non-int, no bespoke string-duration parser) is recorded in Do above and carried into Tasks 1-6 and 1-7.
> As-built: `max_diff_lines` is the precedent — `MaxDiffLines *int` in `fileShape`, seeded in `defaults()` via `defaultMaxDiffLines`, resolved by `resolveMaxDiffLines` (nil → default, else explicit), with `typeErrorMessages["fileShape.MaxDiffLines"]` mapping the friendly message. `timeout` mirrors this exactly, plus the explicit-zero honoring (which `max_diff_lines` does not need).

**Spec Reference**: `.workflows/ai-model-selection/specification/ai-model-selection/specification.md` — Config schema: `timeout` key (including the Deferred-to-planning representation note and the int-seconds value-semantics bullet).

## ai-model-selection-1-6 | approved

### Task ai-model-selection-1-6: Add per-verb `timeout` overrides to the schema and strict decoding

**Problem**: Per-verb model freedom and per-verb timeout freedom must travel together: a verb pinned to a slower model needs to raise its own deadline without changing the shared one. So `[release].timeout` and `[commit].timeout` must become decodable per-verb override keys with the same absent-vs-explicit-zero distinction as the top-level key, and strict decoding must accept them while a wrong type still fails loud.

**Solution**: Add an optional integer-seconds `timeout` field (`*int`, matching Task 1-5's chosen representation) to both `releaseShape` and `commitShape`, carry the absent-vs-explicit-zero-preserving value onto the `Release`/`Commit` structs, and add `typeErrorMessages` entries for `release.timeout` and `commit.timeout`. Resolution (the value semantics across layers) is Task 1-7; this task makes the keys decode, carry through with absent-vs-zero preserved, and fail loud on a wrong type.

**Outcome**: `[release].timeout` and `[commit].timeout` decode (integer seconds); absent vs explicit `0` is distinguishable per-verb; strict decoding accepts them; a non-integer per-verb timeout fails loud at `Load` naming the key; unknown sibling keys still rejected.

**Do**:
- Add `Timeout *int` to `releaseShape` (~line 250, `toml:"timeout"`) and to `commitShape` (~line 245, `toml:"timeout"`), matching Task 1-5's integer-seconds choice.
- Carry the per-verb value onto `Release` and `Commit` so Task 1-7's accessor can read it with absent (nil) vs explicit `0` vs negative all distinguishable — carry as `*time.Duration` (nil = absent; non-nil = the operator's explicit value, converting seconds→`time.Duration` at the boundary; preserve `0` and negatives raw for 1-7 to interpret). Wire through `resolveRelease` / `resolveCommit` (the same explicit field-by-field copy `resolveRelease` already uses; `resolveCommit` must drop the direct `Commit(shape)` conversion — already required by Task 1-3).
- Add `typeErrorMessages` entries: `"releaseShape.Timeout": "release.timeout must be an integer (seconds)"` and `"commitShape.Timeout": "commit.timeout must be an integer (seconds)"`.
- Do NOT seed per-verb timeout defaults — the absent baseline is "no override" (nil), which falls through to shared/floor in 1-7.

**Acceptance Criteria**:
- [ ] `[release].timeout` and `[commit].timeout` decode (integer seconds) without strict-decode rejection.
- [ ] Both preserve absent (nil) vs explicit `0` on the carried config (the per-verb absent-vs-zero distinction).
- [ ] An integer is carried as that many seconds; a negative integer is carried raw (negative-drop is 1-7's job, not this task's).
- [ ] A non-integer per-verb `timeout` (e.g. `"slow"`) is rejected at `Load` with a mapped friendly message naming the key and integer/seconds type.
- [ ] An unknown sibling key in `[release]` / `[commit]` is still rejected.
- [ ] `go build ./...`, `gofmt -l .` (prints nothing), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues) all pass.

**Tests**:
- `"it decodes [release].timeout as seconds and carries it"`
- `"it decodes [commit].timeout as seconds and carries it"`
- `"it distinguishes an absent per-verb timeout (nil) from an explicit [verb].timeout = 0"`
- `"it carries a negative per-verb timeout raw (drop is the accessor's job)"`
- `"it rejects a non-integer [release].timeout naming the key and integer type"`
- `"it rejects a non-integer [commit].timeout naming the key and integer type"`
- `"it still rejects an unknown sibling key in [release] / [commit] after adding timeout"`

**Edge Cases**:
- Per-verb absent vs explicit zero distinguished (per the task table).
- Type mismatch still fails loud — non-integer per-verb timeout is a strict decode error at `Load`, distinct from a value-level drop-through.
- Unknown sibling keys still rejected.

**Context**:
> Spec — Config schema: `timeout` key: "a top-level shared default plus optional `[release]` / `[commit]` overrides, with resolution `[verb].timeout → top-level shared timeout → shipped default`." The new key needs the schema struct field in BOTH verb shapes plus `typeErrorMessages` entries (the same strict-decoding requirement as per-verb `ai_command`).
> Spec — Config schema: per-verb `ai_command` override: "`ai_command` and `timeout` become the first keys living at both levels with fallback — a small, deliberate new pattern."
> Representation carried from Task 1-5: integer seconds (`*int` in the shapes), for idiom-consistency with `max_diff_lines` and to get strict-decode-on-non-int for free. Absent vs explicit `0` is the `*` pointer; negative is carried raw for the accessor (1-7) to drop.
> As-built: `resolveRelease` already copies fields explicitly; `resolveCommit`'s current `Commit(shape)` direct conversion must become field-by-field once the shape carries `*` fields that map to differently-typed carried fields (also flagged in Task 1-3).

**Spec Reference**: `.workflows/ai-model-selection/specification/ai-model-selection/specification.md` — Config schema: `timeout` key; Config schema: per-verb `ai_command` override (both-levels-with-fallback pattern).

## ai-model-selection-1-7 | approved

### Task ai-model-selection-1-7: Add the layered `TimeoutFor(verb)` accessor with value semantics

**Problem**: Consumers need a single resolved timeout per verb without re-implementing the fallback chain or the non-normal-value rules. Unlike `ai_command`, timeout's chain treats values specially: an explicit `0` means "no deadline" and is HONORED — it stops the fall-through and is NOT treated as missing; a negative/invalid value DROPS through to the next layer; a positive value is used as-is; the floor is the shipped 60s. Getting these rules wrong either makes "no deadline" unreachable or lets a forgotten/negative value silently disable the deadline.

**Solution**: Add an exported accessor on `Config` that resolves `[verb].timeout → top-level shared timeout → 60s floor`, honoring an explicit `0` (returns it and stops the chain), dropping a negative/value-invalid candidate to the next layer, using a positive value as-is, and bottoming out at the 60s floor. Choose a return type that lets the transport (Phase 2) distinguish an operator's explicit `0` ("no deadline") from any other value — per the spec's absent-vs-explicit-zero invariant at the `config → ai.Config` boundary.

**Outcome**: `cfg.TimeoutFor(VerbRelease)` / `cfg.TimeoutFor(VerbCommit)` resolve the per-verb chain with: explicit `0` honored and stopping fall-through; negative dropped to the next layer; positive used as-is; floor at 60s; and the return distinguishes explicit-`0` from a fallen-through value so Phase 2's boundary can keep "no deadline" reachable only by an operator's explicit `0`.

**Do**:
- Add `func (c Config) TimeoutFor(verb Verb) <ReturnType>` to `internal/config/config.go`. RETURN TYPE: return `*time.Duration` (or a tiny wrapper) so the Phase 2 `config → ai.Config` boundary can distinguish an explicit `0` ("no deadline") from a positive value and from the floor — the spec mandates "no deadline" be reachable ONLY by an operator's explicit `0`, never by a forgotten/zero-by-omission field. A plain `time.Duration` zero would re-introduce the ambiguity the spec forbids. (If a wrapper type is chosen, define it in `config` and expose whether it is the explicit-zero/no-deadline case.) Document the boundary contract in the accessor comment so Phase 2 wires it correctly.
- Build the candidate chain for `verb`: per-verb override (carried in Task 1-6), then shared top-level (carried in Task 1-5), then the 60s floor (`DefaultTimeout`). For each candidate in order:
  - absent (nil) → skip to the next candidate;
  - explicit `0` → HONOR it: return the "no deadline" / explicit-zero result and STOP (do not fall through);
  - negative (value-invalid) → DROP through to the next candidate (do NOT honor, do NOT treat as zero);
  - positive → return it as-is.
  If all candidates are exhausted (all absent, or the only present ones were negative-and-dropped), return the 60s floor.
- Per-key independence: `TimeoutFor` reads only `timeout` candidates, never `ai_command`.
- Update comments so the value semantics (zero-honored, negative-dropped, positive-as-is, 60s floor) are documented at the accessor. Reference that the transport (Phase 2) must skip `context.WithTimeout` when the resolved value is the explicit-`0`/no-deadline case — but do NOT modify the transport here (Phase 2).
- Ensure the negative-drop interacts correctly with the floor: a verb with `timeout = -1` and no shared override resolves to 60s (negative dropped, floor applied), NOT to "no deadline" and NOT to a negative.

**Acceptance Criteria**:
- [ ] `cfg.TimeoutFor(verb)` returns a present positive `[verb].timeout` as-is.
- [ ] An explicit `[verb].timeout = 0` is HONORED as "no deadline" and STOPS the fall-through (a present shared/floor value is NOT consulted).
- [ ] A negative `[verb].timeout` DROPS through to the shared top-level, then to the 60s floor (never honored, never collapsed into the zero/no-deadline case).
- [ ] An absent `[verb].timeout` falls to the shared top-level; an absent shared falls to the 60s floor.
- [ ] A negative shared `timeout` (with absent per-verb) drops to the 60s floor.
- [ ] The return type distinguishes an explicit-`0`/no-deadline result from a positive/floor result (so Phase 2's boundary can keep "no deadline" reachable only by explicit `0`).
- [ ] With no `.mint.toml`, both verbs resolve to the 60s floor.
- [ ] `go build ./...`, `gofmt -l .` (prints nothing), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues) all pass.

**Tests**:
- `"it resolves a present positive [verb].timeout as-is"`
- `"it honors an explicit [verb].timeout = 0 as no-deadline and stops fall-through"` — with a present shared timeout, prove the shared value is NOT used.
- `"it drops a negative [verb].timeout through to the shared timeout"`
- `"it drops a negative [verb].timeout with no shared override through to the 60s floor"`
- `"it falls an absent [verb].timeout through to the shared timeout, then to the 60s floor"`
- `"it drops a negative shared timeout to the 60s floor"`
- `"it resolves both verbs to the 60s floor when no .mint.toml exists"`
- `"its return distinguishes an explicit-zero (no deadline) from a positive/floor value"`

**Edge Cases**:
- Explicit `0` honored and stops fall-through, not treated as missing (per the task table).
- Negative drops through to floor.
- Unparseable/value-invalid drops through — with integer-seconds, "unparseable" (a non-integer) is already a Load-time decode error (Task 1-5/1-6), so at the accessor the only value-invalid case is a NEGATIVE integer; this is the spec's "where value-invalid is detected differs by representation" — for int-seconds it is the negative-integer drop-through here, with non-int already rejected at Load.
- Positive used as-is.
- A negative must NOT be collapsed into the `0` no-deadline branch (it drops; only an explicit `0` disables the deadline).

**Context**:
> Spec — Resolution value semantics (`timeout`): "Zero is an explicit, honored value meaning 'no time limit' — it disables the per-attempt deadline and stops the fall-through (it is not treated as missing). This is a conscious, operator-chosen exception to 'fail loud, never hang' … It must be documented … Missing or invalid (e.g. negative) drops through to the next layer; positive is used as-is; the floor is the shipped 60s default. A wrong type still surfaces as a strict decode error at `Load` … The config→`ai.Config` boundary must preserve absent-vs-explicit-zero for `timeout` … Invariant: 'no deadline' must only ever be reachable by an operator's explicit `0`, never by a wiring site omitting the field … Planning picks the mechanism (e.g. give the boundary field a type that distinguishes nil from explicit-`0`, such as `*time.Duration` / a small wrapper …)."
> Spec — int-seconds representation (chosen in Task 1-5): "a negative integer is a value-invalid drop-through; absent vs zero is a nil pointer vs `0`." So at the accessor, the value-invalid case is the negative integer (non-int was already rejected at Load).
> Spec — Acceptance criteria — resolution behaviors: "`timeout = 0` honored — resolves as 'no deadline' and stops fall-through (not treated as missing) … Negative/invalid `timeout` drops through — to the 60s floor; a positive value is used as-is."
> NOTE — the transport's conditional `WithTimeout` (skip on `0`) and the three wiring sites consuming this accessor are PHASE 2, not this task. This task delivers the accessor and its return type only; do not touch `internal/ai` or the wiring sites.

**Spec Reference**: `.workflows/ai-model-selection/specification/ai-model-selection/specification.md` — Resolution value semantics (`timeout`); Acceptance criteria — resolution behaviors; Config schema: `timeout` key (int-seconds value semantics).

## ai-model-selection-1-8 | approved

### Task ai-model-selection-1-8: Prove per-key resolution independence across `ai_command` and `timeout`

**Problem**: Resolution is per-key INDEPENDENT — `ai_command` and `timeout` each fall through their own `verb → shared → default` chain. A regression where overriding one key perturbs the other's resolution (e.g. an accessor accidentally reading the wrong field, or a shared-shape coupling) would silently break the feature's core promise: a verb that overrides only its command still inherits the shared/floor timeout, and vice-versa. The two accessors built in 1-4 and 1-7 must be proven independent in combination.

**Solution**: Add focused tests that exercise both accessors together against configs that override exactly one key on a verb, asserting the OTHER key still resolves through its own shared/floor chain — both directions, both verbs. This is a verification-only TDD cycle (no new production code expected); if a test reveals coupling, fix the accessor that leaked.

**Outcome**: Tests prove that overriding `[verb].ai_command` leaves `cfg.TimeoutFor(verb)` resolving through the shared/floor, and overriding `[verb].timeout` leaves `cfg.AICommandFor(verb)` resolving through the shared/floor — for both `VerbRelease` and `VerbCommit`.

**Do**:
- Add tests in `internal/config/config_test.go` (external `config_test` package, `t.Parallel()`, `t.TempDir()`, table-driven where the shape fits — per project test idioms).
- Case A (override command only): write `[release]\nai_command = "haiku-cmd"` (no per-verb timeout, no shared timeout). Assert `AICommandFor(VerbRelease)` is the override AND `TimeoutFor(VerbRelease)` is the 60s floor (unperturbed).
- Case B (override timeout only): write `[release]\ntimeout = 120` (no per-verb ai_command, no shared ai_command override). Assert `TimeoutFor(VerbRelease)` is 120s AND `AICommandFor(VerbRelease)` is the shipped default floor (unperturbed).
- Case C (override command only on commit) and Case D (override timeout only on commit): mirror A/B for `VerbCommit`.
- Add a combined case: a verb overrides its command, while a SHARED top-level timeout is set — assert the command resolves to the per-verb value and the timeout resolves to the SHARED value (proving neither chain leaks into the other and the shared layer is consulted independently).
- Optionally add the cross-verb independence check: a `[release]` override does not affect `cfg.AICommandFor(VerbCommit)` / `cfg.TimeoutFor(VerbCommit)` (each verb reads only its own table).

**Acceptance Criteria**:
- [ ] Overriding `[release].ai_command` only leaves `TimeoutFor(VerbRelease)` at the 60s floor.
- [ ] Overriding `[release].timeout` only leaves `AICommandFor(VerbRelease)` at the shipped default floor.
- [ ] The same independence holds for `VerbCommit` (both directions).
- [ ] A per-verb command override combined with a shared top-level timeout resolves the command to the override and the timeout to the shared value (independent chains).
- [ ] A `[release]` override does not perturb `VerbCommit` resolution (each verb reads its own table).
- [ ] `go build ./...`, `gofmt -l .` (prints nothing), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues) all pass.

**Tests**:
- `"overriding ai_command on release leaves release timeout at the 60s floor"`
- `"overriding timeout on release leaves release ai_command at the shipped default"`
- `"overriding ai_command on commit leaves commit timeout at the 60s floor"`
- `"overriding timeout on commit leaves commit ai_command at the shipped default"`
- `"a per-verb command override with a shared timeout resolves each key through its own chain"`
- `"a [release] override does not perturb commit resolution"`

**Edge Cases**:
- Per-key independence both directions, both verbs (per the task table) — the test matrix covers command-only and timeout-only overrides for release and commit.

**Context**:
> Spec — Resolution value semantics: "Resolution is per-key independent — `ai_command` and `timeout` each fall back through their own `verb → shared → default` chain."
> Spec — Acceptance criteria — resolution behaviors: "Per-key independence — overriding `ai_command` on a verb leaves that verb's `timeout` resolving through the shared/floor (and vice-versa)."
> Spec — Timeout × model-choice coupling: "Because resolution is per-key independent, a verb that overrides `ai_command` to a slower model but not `timeout` silently inherits the 60s shared default … Mint does not protect against this" — these tests pin exactly that (independent, unprotected) behaviour as correct.
> Project test idioms: external `config_test` package, table-driven, `t.Parallel()`, `t.TempDir()`; behaviour-level proofs over unit minutiae.

**Spec Reference**: `.workflows/ai-model-selection/specification/ai-model-selection/specification.md` — Resolution value semantics (per-key independence); Acceptance criteria — resolution behaviors; Timeout × model-choice coupling.
