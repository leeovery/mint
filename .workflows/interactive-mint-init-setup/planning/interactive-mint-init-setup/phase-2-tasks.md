---
phase: 2
phase_name: mint setup subcommand — guide emitter, sections, help wiring
total: 4
---

## interactive-mint-init-setup-2-1 | approved

### Task 2-1: Author the embedded setup-guide body with stable section markers

**Problem**: The pivot away from an interactive `mint init` puts all setup interactivity into an AI guide the binary emits. There is no in-binary text yet that teaches an AI agent how to configure mint — what mint's pipeline/hook model is, the etiquette of proposing changes, the minimalism rule, how to handle an existing `.mint.toml`, and the ordered inspect-and-map procedure. Without this body there is nothing for `mint setup` to print.

**Solution**: Build a pure string emitter (in the spirit of `internal/initgen`) that returns the embedded static setup-guide body as one version-matched string. Each required section emits a stable, test-detectable marker (a fixed anchor) so a structural test can prove section presence without coupling to body prose. This task authors every section EXCEPT the config-reference table render, which Task 2-2 supplies — but it defines the config-reference marker and the assembly seam where that rendered table is spliced in.

**Outcome**: A new package (working name `internal/setupguide`, a planning detail — confirm the package name against repo conventions at implementation time) exposes a pure function returning the guide body string. The string carries all required content sections, each preceded by its stable marker. A structural test greps the markers (not prose) and passes. The config-reference section's marker is present with a placeholder/seam that Task 2-2 fills.

**Do**:
- Create the new emitter package (proposed `internal/setupguide`, sibling to `internal/initgen`). Mirror the `initgen` style: a package doc comment stating it is a PURE string emitter performing NO IO, and a single exported function (proposed `Guide() string`) returning the embedded static body. Do NOT import `config` from the prose-authoring path — but note Task 2-2 will need the SoT for the config-reference table, so design the assembly so the config-reference section is composed from a value Task 2-2 produces (e.g. `Guide()` concatenates the prose sections with the rendered config-reference table; decide whether `Guide()` calls a Task-2-2 render helper internally or accepts the rendered table — recommend `Guide()` internally calls the render helper so callers get one finished string).
- Define a single, documented set of stable section markers as package constants — one per required section. Propose HTML-comment-style anchors that will not appear incidentally in prose, e.g. `<!-- mint:section:pipeline -->`, `<!-- mint:section:etiquette -->`, `<!-- mint:section:minimalism -->`, `<!-- mint:section:existing-config -->`, `<!-- mint:section:config-reference -->`. The exact marker text is a planning detail; the contract is: a fixed greppable anchor per required section, decoupled from body prose. Emit each marker on its own line immediately before its section so the marker's presence is independent of the surrounding wording.
- Author the prose for each required CONTENT section (markers in brackets):
  1. Pipeline/stage model `[pipeline marker]` — the ordered stages `preflight → notes → pre_tag → tag + push (PONR) → publish → post_release`; preflight runs before any release work and failure aborts; pre_tag runs after notes, before the tag, and accepts an ARRAY of ordered commands; post_release runs after publish; the tag + atomic push is the point of no return. Include the one-line release-shim role mention: what `./release` is and that `mint init` creates it.
  2. AI etiquette `[etiquette marker]` — ask interactively using whatever user-question tool the agent has (phrased AI-agnostically: "if you're Claude, use your Ask-User tool; otherwise, if you have any tool for asking the user questions, use it"); confirm the user is comfortable and make clear exactly what is being changed before writing; never remove anything without explicit permission; surface the `diff_exclude` patterns inside the interactive confirmation.
  3. Minimalism `[minimalism marker]` — add (activate) a key ONLY to set a non-default value; if the default is fine, omit the key entirely (this is NOT skipping/disabling — defaults are compiled into the binary and apply whether or not the key appears; the whole file is optional); for every activated key, state the project-specific reason at the confirmation step so over-configuration is visible; read defaults from the config reference, never guess or restate default values (DRY).
  4. Existing-config/upgrade branch `[existing-config marker]` — if a `.mint.toml` already exists, bring it into context and discuss; never silently overwrite; offer work-with-existing (targeted, key-by-key) vs start-fresh-reusing-values; config upgrade/migration: detect removed/renamed keys that would fail `DisallowUnknownFields` loudly at the next `mint` run, values that no longer fit, and new keys worth considering — "setup" doubles as "upgrade my config to this mint version".
  5. Config reference `[config-reference marker]` — emit the marker and the assembly seam where Task 2-2's rendered `key · level · default · description` table lands. This task authors the marker + any introductory framing line; Task 2-2 supplies the table rows.
- Author the ordered inspect-and-map PROCEDURE content (it threads through the marked sections; it does not require its own marker unless a marker aids the structural test — author's discretion, but the five markers above are the contract): (1) confirm the working directory is the intended repo root before inspecting or writing anything (the in-instructions safety net that replaces `mint setup`'s missing cwd guard); (2) learn mint — read the README and internalise the minimalist philosophy; (3) read the config reference as an explicit ordered EARLY step performed BEFORE any inspect/edit — internalise the embedded config-reference section in this same output (do not fetch a separate artifact); (4) inspect-and-map: existing release process → hooks (the centrepiece), noise dirs → `diff_exclude`, version file → `version_file`/`version_pattern`, AI model per verb → `ai_command` shared-or-per-verb, provider/release_branch only if auto-detect would be wrong; (5) propose → explain → approve per the etiquette; (6) sanity-check — strict-schema `DisallowUnknownFields` loud-fail backstop plus a natural "verify the config loads" step.
- Weave in hook-detection guidance: propose a best-fit mapping and flag it; never silently skip a step ("if it's in the customer's release script, it's important"); when a step doesn't fit, surface it honestly (the outcome may legitimately be "mint isn't suitable here"); explain mint's model so a technical user can collaborate; `pre_tag`-as-array widens what fits (a linear multi-step build/test sequence maps to a `pre_tag` array).
- Weave in the AI-model-per-verb mapping: "same" → shared top-level `ai_command`; "different" → `[release].ai_command` + `[commit].ai_command`. And `diff_exclude` scope: release-notes noise not generated code; `.gitignore`'d paths are already absent; real targets are tracked process/meta/doc files (`.workflows/`, `.claude/`, agent dirs, `docs/`, lockfiles); surface interactively inside the confirmation step.
- Add the structural marker test (proposed `internal/setupguide/setupguide_test.go`, external `package setupguide_test`): assert `Guide()` contains each of the five section markers exactly. Key the test on the marker CONSTANTS, not literal prose substrings. Add a guard proving a section's prose without its marker would fail (e.g. assert the marker count is exactly one per section, or document that the test greps marker constants so removing a marker fails even if prose stays).

**Acceptance Criteria**:
- [ ] A new pure emitter package returns the embedded setup-guide body as one static string and performs no IO.
- [ ] The body carries all required content: pipeline/stage model (with the shim role mention), AI etiquette, minimalism rule, existing-config/upgrade branch, the ordered inspect-and-map procedure (cwd-confirm, learn-mint, read-config-reference-early, inspect-and-map, propose/explain/approve, sanity-check), hook-detection guidance, AI-model-per-verb mapping, and `diff_exclude` scope.
- [ ] Each of the five required sections (pipeline/hook model, etiquette, minimalism, existing-config/upgrade, config reference) is preceded by a stable, greppable marker defined as a package constant, decoupled from body prose.
- [ ] The config-reference section carries its marker plus the assembly seam Task 2-2 fills; the prose-authoring path does not hand-write config metadata.
- [ ] A structural test greps the marker constants and proves each section's presence; a section present without its marker would fail the test.
- [ ] All standard gates pass (`go build ./...`, `gofmt -l .`, `go vet ./...`, `go test -race ./...`, `golangci-lint run`).

**Tests**:
- `"it emits the pipeline-model section marker"`
- `"it emits the etiquette section marker"`
- `"it emits the minimalism section marker"`
- `"it emits the existing-config/upgrade section marker"`
- `"it emits the config-reference section marker"`
- `"it keys section detection on the marker constants, not prose, so a section without its marker fails"`
- `"it mentions the release shim role (what ./release is and that mint init creates it)"`
- `"it carries the cwd-confirm safety step as the first ordered procedure step"`

**Edge Cases**:
- Markers must be greppable anchors decoupled from body prose — a wording change in any section must NOT break the structural test, and removing a marker MUST break it.
- A section present in prose but with its marker absent must fail the structural test (the test keys on markers, never representative prose substrings).
- The config-reference marker exists in this task even though the table rows are supplied by Task 2-2 — the marker and seam are this task's responsibility so 2-2 splices into a stable anchor.

**Context**:
> The spec (`The mint setup subcommand` → `Stable section markers`) decides that markers exist and the structural test keys on them; the exact marker text is a planning detail. Required marked sections: pipeline/hook model, etiquette, minimalism, the existing-config/upgrade branch, and the config reference.
>
> The spec (`Emitted guide — setup procedure`, `mint's pipeline / stage model`, `Hook detection & mapping`, `Cross-cutting principle`, `AI model per verb`, `diff_exclude scope`, `Emitted guide — AI etiquette`, `Emitted guide — minimalism`, `Emitted guide — existing .mint.toml`) is the source of every content requirement above. Read those sections in full; the prose is a planning detail but the required content is decided.
>
> EMISSION-SURFACE DECISION (flagged, not silently fixed): CLAUDE.md seam 3 says every byte of user output goes through `presenter.Presenter`, with the cmd layer's usage/error lines as the only exception. The spec calls `mint setup` a pure string emitter performing no IO beyond stdout. Two options for HOW the guide reaches stdout: (A) a cmd-layer stdout write (`fmt.Fprint(os.Stdout, ...)`), consistent with the existing curated-help/usage texts which already write to stdout from the cmd layer as the documented exception — `mint setup` is conceptually the same kind of static, requested-action text emission; (B) route through the presenter to honour seam 3 strictly. RECOMMENDATION: option A (cmd-layer stdout write), because the curated-help register is the precedent and the spec frames `mint setup` as a pure emitter in the spirit of `initgen` (a string producer with the cmd layer doing the write), and routing static help-class text through the presenter would be a novel use of that seam. This task only authors the string; the actual write site is Task 2-3 — surface the same decision there. Leave the final call to the implementer; do not hard-wire a presenter dependency into this emitter package.
>
> Confirm at implementation time: the new package name/location against repo conventions; the `initgen` emitter style (package doc comment, single exported string function, fully static, no IO).

**Spec Reference**: `.workflows/interactive-mint-init-setup/specification/interactive-mint-init-setup/specification.md` — sections `The mint setup subcommand`, `Emitted guide — setup procedure`, `mint's pipeline / stage model (a required content section)`, `Hook detection & mapping`, `Cross-cutting principle — agent as collaborator`, `AI model per verb — config representation`, `diff_exclude scope`, `Emitted guide — AI etiquette`, `Emitted guide — minimalism (only set what varies)`, `Emitted guide — existing .mint.toml: diff, discuss, upgrade`, `Release shim and mint init unchanged` (shim role mention), `Definition of done` (Structural test).

## interactive-mint-init-setup-2-2 | approved

### Task 2-2: Render the config-reference section from the Phase 1 SoT

**Problem**: The stripped `.mint.toml` template no longer carries in-repo config docs, so the agent's config reference must come from `mint setup`. That reference must be the drift-tested config-metadata SoT (Phase 1), not a hand-written table — otherwise it can drift from what the binary actually accepts. Task 2-1 created the config-reference marker and seam but did not fill the table.

**Solution**: Render the config-reference section's `key · level · default · description` table from the Phase 1 SoT (`config.MetadataRows()` returning rows with exported `Key`, `Level`, `Default`, `Description`, and a typed `config.MetadataLevel` whose `String()` renders the TOML form). The render reads the SoT and never re-derives or hand-writes metadata. Dual-level `ai_command`/`timeout` render as distinct rows per level; default-column tokens are carried verbatim from the SoT.

**Outcome**: The config-reference section of the emitted guide contains a table with one line per SoT row, each line carrying the row's key, level (rendered via `MetadataLevel.String()` — `[release]`, `[release.hooks]`, `[commit]`, shared), the verbatim default-column token, and the description. `ai_command` and `timeout` each appear as distinct rows for shared, `[release]`, and `[commit]`. A test proves the render reflects the SoT (including the dual-level distinct rows and the verbatim default tokens) rather than a hand-list.

**Do**:
- BEFORE writing code, read `internal/config` to confirm the actual Phase 1 SoT signatures: the exported accessor name (documented as `config.MetadataRows() []config.MetadataRow`), the `MetadataRow` exported fields (`Key`, `Level`, `Default`, `Description`), and the `MetadataLevel` type with its `String()` rendering of the TOML form. If any signature differs from the documented API, adapt the render to the actual surface and note the divergence. (Phase 1 is planned but not yet implemented at authoring time — confirm against the shipped code.)
- Add a render helper (proposed in `internal/setupguide`, e.g. `configReference() string` or `renderConfigReference(rows []config.MetadataRow) string`) that iterates the SoT rows in their SoT order and emits one table line per row. The line carries: the key, the level via `row.Level.String()`, the default token verbatim from `row.Default` (do NOT transform, re-default, or re-derive — blank stays blank, `auto` stays `auto`, `[]` stays `[]`, `shared` stays `shared`, hooks-blank stays blank/`—`), and the description. Choose a stable column layout (markdown table or aligned columns — a planning detail; recommend a markdown table for agent-readability and so the render is unambiguous).
- Splice the rendered table into the config-reference section seam from Task 2-1 (under the `[config-reference marker]`), so `Guide()` returns one finished string with the table in place. This is where the setupguide package gains its `config` import (the prose-authoring path stayed config-free; the render path is the single place that reads the SoT).
- Do NOT collapse the dual-level rows: `ai_command` and `timeout` must render as three distinct rows each (shared, `[release]`, `[commit]`) exactly as the SoT carries them, so the agent sees the "same → shared, different → per-verb" choice in the reference.
- Add a render test (in `internal/setupguide/setupguide_test.go`, external package): assert the rendered config-reference section contains a line for every `config.MetadataRows()` row (drive the assertion FROM `config.MetadataRows()` so adding/removing a SoT row is reflected without editing the test's expectations — i.e. assert a per-row presence, not a frozen string). Assert the three dual-level keys each appear once per level. Assert the default tokens are carried verbatim (e.g. find the `diff_exclude` row renders `[]`, a sentinel-auto row renders `auto`, a per-verb override row renders `shared`, an empty-default row renders blank). Assert the render reads `config.MetadataRows()` and does not re-derive metadata (structurally — the test drives expectations from the SoT, so a re-derived table that diverged from the SoT would fail).

**Acceptance Criteria**:
- [ ] The config-reference section renders one table line per `config.MetadataRows()` row, carrying key · level · default · description.
- [ ] Level is rendered via `MetadataLevel.String()` (TOML form: `[release]`, `[release.hooks]`, `[commit]`, shared).
- [ ] `ai_command` and `timeout` each render as three distinct rows (shared, `[release]`, `[commit]`); the render never collapses dual-level keys.
- [ ] Default-column tokens (blank / `auto` / `[]` / `shared` / hooks-blank) are carried verbatim from the SoT — never transformed, re-defaulted, or re-derived.
- [ ] The render reads the SoT (`config.MetadataRows()`) and re-derives no metadata; the test drives expectations from the SoT so a divergent re-derived table would fail.
- [ ] The rendered table is spliced under the config-reference marker so `Guide()` returns one finished string; the structural marker test from Task 2-1 still passes.
- [ ] All standard gates pass.

**Tests**:
- `"it renders a config-reference line for every SoT row"`
- `"it renders ai_command as distinct shared, [release], and [commit] rows"`
- `"it renders timeout as distinct shared, [release], and [commit] rows"`
- `"it carries the empty-collection default token [] verbatim for diff_exclude"`
- `"it carries the sentinel-auto default token verbatim for release_branch/provider"`
- `"it carries the per-verb inherit default token shared verbatim for per-verb ai_command/timeout"`
- `"it carries a blank default cell for empty-string-default keys"`
- `"it renders the level via MetadataLevel.String() in TOML form"`

**Edge Cases**:
- Dual-level `ai_command`/`timeout` must render distinctly per level — three rows each — never collapsed to one.
- Default-column representations are carried verbatim from the SoT; the render performs zero metadata logic (no re-deriving defaults, no re-typing tokens).
- The render reads the SoT, never a hand-maintained list — the test drives expectations from `config.MetadataRows()` so SoT additions/removals propagate without test edits, and a re-derived table that diverged would fail.

**Context**:
> Phase 1 delivers the SoT consumed here and must NOT be re-derived: `config.MetadataRows() []config.MetadataRow` (exported fields `Key`, `Level`, `Default`, `Description`) and a typed `config.MetadataLevel` with `String()` rendering the TOML form (`[release]`, `[release.hooks]`, `[commit]`, shared). The default-column representation convention (blank / `auto` / `[]` / `shared` / hooks-blank) is already encoded in the SoT rows. CONFIRM these signatures by reading `internal/config` at implementation time — Phase 1 is planned but not yet implemented.
>
> The spec (`Config-metadata source of truth (SoT)` → `default column representation`) decides the default-column tokens are part of the decided behaviour, not a rendering choice: empty-string defaults → blank; sentinel-auto (`release_branch`, `provider`) → `auto`; empty collection (`diff_exclude`) → `[]`; per-verb inherit (`[release]`/`[commit]` `ai_command`/`timeout`) → `shared`; `[release.hooks]` keys → no default value (blank or `—`). The render carries these verbatim — this task does not implement the convention (Phase 1 does), it renders what the SoT carries.
>
> The spec (`Render targets and layering`) decides `mint setup` is the SoT's single in-binary render target. The spec (`The mint setup subcommand` → What it emits, item 3) decides the config reference is rendered from the SoT "so the agent reads option meanings from a drift-tested table rather than from template comments."

**Spec Reference**: `.workflows/interactive-mint-init-setup/specification/interactive-mint-init-setup/specification.md` — sections `Config-metadata source of truth (SoT)`, `Render targets and layering`, `The mint setup subcommand` (What it emits, item 3), `Definition of done` (Drift test, Structural test).

## interactive-mint-init-setup-2-3 | approved

### Task 2-3: Wire the commandSetup dispatch route and the runSetup runner

**Problem**: The guide emitter (Tasks 2-1/2-2) produces the setup-guide string, but nothing in `cmd/mint` invokes it — there is no `mint setup` command. The dispatch enum, the `classifyCommand`/`run` switch, the verb runner, and the unknown-command message all need the new route so `mint setup` actually emits the guide.

**Solution**: Add a `commandSetup` route to the `cmd/mint` dispatch (`commandKind` enum, `classifyCommand`, the `run` switch, and the unknown-command default message), and a `runSetup` verb runner mirroring the `runInit` idiom — flag parsing with `flag.ErrHelp` → stdout/exit 0 — but running UNCONDITIONALLY with NO `gitrepo`/repo-root resolution before emitting (it must print even outside a work tree). `runSetup` emits the `Guide()` string to stdout.

**Outcome**: `mint setup` emits the guide to stdout and exits 0 from any directory (no `git rev-parse`/cwd guard); `mint setup --help` prints `setupUsage` to stdout and exits 0 (not the usage-error exit 2); an unrecognised flag is a usage error (exit 2); the unknown-command message lists `setup` among the wired commands.

**Do**:
- READ `cmd/mint/main.go` and `cmd/mint/init.go` first to confirm exact names: the `commandKind` enum + its iota constants, `classifyCommand`, the `run` switch, the unknown-command `fmt.Fprintln(os.Stderr, ...)` message, and the `runInit` flag-parsing idiom.
- Add a `commandSetup` constant to the `commandKind` enum in `cmd/mint/main.go` (with a doc comment in the codebase's heavy-WHY style: a top-level verb that emits the AI setup guide to stdout, drives no gate, calls no RunFinished, and — unlike `init`/`release`/`commit` — needs NO git repo / never resolves the repo root, so it runs unconditionally anywhere).
- Add a `setup` branch to `classifyCommand` in `cmd/mint/main.go`: `if args[0] == "setup" { return commandSetup, args[1:] }`, placed alongside the existing `init`/`version`/`commit` branches (before the `release` check). Keep `classifyCommand` pure — no execution, no parsing.
- Add a `case commandSetup:` to the `run` switch in `cmd/mint/main.go`, dispatching to `runSetup(rest)`. Note `runSetup` needs no `ctx` (it spawns nothing to interrupt — like `commandVersion`'s `runVersion`, which takes only the IO descriptors); decide the signature accordingly. Recommend `runSetup(stdout io.Writer) int` (or `runSetup(rest []string) int` writing to `os.Stdout`) — match the `runVersion`/`runInit` shape the rest of the file uses; if `runSetup` must parse `--help` it needs `rest`, so recommend `runSetup(rest []string) int`.
- Update the unknown-command default message in `run` to include `setup` in the list of wired commands (currently: "only `mint release`, `mint release regenerate`, `mint init`, `mint version`, and `mint commit` are wired") — add `mint setup` to that list.
- Create `cmd/mint/setup.go` with `runSetup` (mirror `runInit` in `cmd/mint/init.go` for the flag-parse + `flag.ErrHelp` idiom, but WITHOUT any `gitrepo.ResolveRoot`/presenter/engine wiring): parse the flag set (proposed `parseSetupFlags` — `mint setup` takes no flags beyond the implicit `--help`/`-h`; register the flag set so `-h`/`--help` surfaces `flag.ErrHelp`, discard the flag set's default usage dump with `fs.SetOutput(io.Discard)`); on `flag.ErrHelp` print `setupUsage` to stdout and return 0; on any other parse error print `mint: %v` to stderr and return `usageExitCode`; otherwise write the guide string to stdout and return 0.
- EMISSION SURFACE (carry the same decision flagged in Task 2-1): write the guide either via a cmd-layer `fmt.Fprint(os.Stdout, setupguide.Guide())` (option A, recommended — consistent with the curated-help/usage cmd-layer stdout writes which are the documented exception to seam 3) or through a presenter. Implement option A unless the implementer chooses otherwise; do NOT introduce a presenter/engine dependency for a pure text emit. `runSetup` must perform NO IO beyond writing to stdout — no git, no config load, no repo-root resolution.
- Add focused dispatch tests (in `cmd/mint`, external `package main` test file or the existing test files): `classifyCommand([]string{"setup"})` returns `commandSetup` with empty rest; `run([]string{"setup"})` exits 0 and the unknown-command path is not hit; `run([]string{"setup", "extra-unknown-flag"})`-style invalid flag exits `usageExitCode`. (The `--help` exit-0 and coverage assertions are extended in Task 2-4's usage tests — keep this task's tests to dispatch/runner behaviour.)

**Acceptance Criteria**:
- [ ] `commandKind` gains a `commandSetup` constant with a WHY-comment noting it runs unconditionally (no repo / no `git rev-parse`).
- [ ] `classifyCommand([]string{"setup", ...})` returns `commandSetup` with the remaining args; `classifyCommand` stays pure.
- [ ] The `run` switch routes `commandSetup` to `runSetup`; `mint setup` emits the guide to stdout and exits 0 from any directory with no repo-root resolution.
- [ ] `mint setup --help` prints `setupUsage` to stdout and exits 0 (via `flag.ErrHelp`), NOT the usage-error exit 2.
- [ ] An unrecognised flag to `mint setup` is a usage error (exit 2, message on stderr).
- [ ] The unknown-command default message in `run` lists `mint setup` among the wired commands.
- [ ] `runSetup` performs no IO beyond writing to stdout (no git, no config, no repo-root resolution).
- [ ] All standard gates pass.

**Tests**:
- `"it classifies setup as commandSetup with the remaining args"`
- `"it emits the guide to stdout and exits 0 for mint setup"`
- `"it runs mint setup anywhere — no git rev-parse / repo-root resolution before emitting"`
- `"it exits 0 (not 2) for mint setup --help via flag.ErrHelp"`
- `"it exits with the usage error code for an unrecognised mint setup flag"`
- `"it lists mint setup in the unknown-command message"`

**Edge Cases**:
- Runs anywhere — no `git rev-parse` / repo-root resolution before emitting (unlike `runInit`, which resolves the repo root). The cwd-confirm safety lives in the emitted instructions (Task 2-1), not in a cmd-layer guard.
- `mint setup --help` exits 0 (the requested-action path), not the usage-error exit 2.
- The unknown-command message must be updated to mention `setup` so a typo's diagnostic lists every wired command.

**Context**:
> The spec (`The mint setup subcommand` → `Runs unconditionally — no git/cwd guard`) decides `mint setup` must print even outside a work tree (setup instructions are commonly read before `cd`-ing in). Safety lives in the instructions; `mint init` (run during setup) remains the loud-fail backstop outside a work tree. This is the explicit divergence from `runInit`'s `git rev-parse --show-toplevel` resolution.
>
> The spec (`The mint setup subcommand` → `Help-surface wiring`) decides dispatch wiring is a new `commandSetup` route in `classifyCommand`/`run`. The exact verb name `setup` is the spec's working name (the exact command name is a planning detail) — use `setup` unless a stronger name emerges.
>
> Real codebase shape (verify at implementation time): `cmd/mint/main.go` carries the `commandKind` enum (iota, zero value `commandUnknown`), `classifyCommand` (pure string match on `args[0]`), the `run` switch dispatching each verb, and the unknown-command `fmt.Fprintln(os.Stderr, "mint: unknown command (only ... are wired)")`. `runVersion` takes only IO descriptors and no ctx (it spawns nothing) — the `runSetup` precedent for a ctx-free runner. `runInit` (in `cmd/mint/init.go`) is the flag-parse + `flag.ErrHelp` → stdout/exit-0 idiom to mirror, MINUS the repo-root/presenter/engine wiring.
>
> EMISSION-SURFACE DECISION (flagged, not silently fixed): same tradeoff as Task 2-1 — cmd-layer stdout write (option A, recommended, consistent with the existing curated-help/usage cmd-layer writes that are the documented exception to CLAUDE.md seam 3) vs routing through the presenter (option B). Implement A unless the implementer decides otherwise. Surface this in the task so it is a conscious choice, not a silent default.

**Spec Reference**: `.workflows/interactive-mint-init-setup/specification/interactive-mint-init-setup/specification.md` — sections `The mint setup subcommand` (Runs unconditionally — no git/cwd guard; Help-surface wiring — dispatch), `Definition of done`.

## interactive-mint-init-setup-2-4 | approved

### Task 2-4: Thread mint setup through the curated-help surface and extend the coverage test

**Problem**: `mint setup` now dispatches and emits (Task 2-3), but it is invisible in mint's hand-written help surface: `mint help` does not list it, there is no `setupUsage` text for `mint setup --help`, and the usage-coverage test does not pin it. Without this wiring the command ships undocumented and unpinned.

**Solution**: Add the `setup` command-list line to `rootUsage` with a curated one-line description, add a curated `setupUsage` text for `mint setup --help` (opening with the `usage: mint ...` synopsis line, noting no flags beyond `--help` and that it runs anywhere/no repo required), and extend the existing usage-coverage test to pin `mint setup` (the `rootUsage` line plus `setupUsage`). `mint help` stays the frozen curated text — it gains ONLY the `setup` line and carries NO config reference.

**Outcome**: `mint help` / `mint --help` lists `setup` with a curated description that states it PRINTS/EMITS the guide (not that it writes files); `mint setup --help` prints `setupUsage` (synopsis + one-line summary) to stdout with exit 0; the coverage test fails if `setup` is dropped from `rootUsage` or if `setupUsage` loses its synopsis line; `mint help` carries no config reference and is otherwise byte-identical to before plus the one `setup` line.

**Do**:
- READ `cmd/mint/usage.go` and `cmd/mint/usage_test.go` first to match the curated register and the test's exact maps/slices.
- In `cmd/mint/usage.go`, add a `setup` line to the `rootUsage` command list (alongside `release`/`commit`/`init`/`version`), with a curated one-line description in the existing register. Use the spec's wording: "print the AI-assisted setup guide for configuring mint" (exact wording adjustable, but it must state the command PRINTS/EMITS the guide, NOT that it writes files). Align the column to the existing entries. Do NOT add any config reference to `rootUsage` — `mint help` stays frozen apart from this one line.
- In `cmd/mint/usage.go`, add a curated `setupUsage` const (mirroring `initUsage`'s shape): it MUST open with a `usage: mint setup` synopsis line (the coverage test asserts `strings.HasPrefix(usage, "usage: mint ")`); a one-line summary in the same register noting `mint setup` takes no flags beyond `--help` and runs anywhere (no repo required). Keep it short — `mint setup` has no real flag surface, so the body documents the no-flags / runs-anywhere facts.
- Confirm `runSetup` (Task 2-3) prints `setupUsage` on `flag.ErrHelp` — this task owns the const; Task 2-3 owns the print site. If the const did not exist when 2-3 was implemented, reconcile here (the print site references `setupUsage`).
- Extend `cmd/mint/usage_test.go`:
  - `TestParseFlags_HelpSurfacesErrHelp`: add `"setup": func(a []string) error { _, err := parseSetupFlags(a); return err }` to the `parsers` map so `-h`/`--help` is pinned to surface `flag.ErrHelp` for setup.
  - `TestRunVerb_Help_ExitsZero`: add `"setup": func() int { return runSetup([]string{"--help"}) }` to the `runs` map so `mint setup --help` is pinned to exit 0. (Adjust the call to match `runSetup`'s actual signature from Task 2-3.)
  - `TestUsageTexts_CoverTheirFlagSets`: add a row for setup — `{"setup", setupUsage, []string{...}}` (the flags slice can be empty if `mint setup` registers no long flags beyond help; the `strings.HasPrefix(tc.usage, "usage: mint ")` synopsis assertion still applies and pins the synopsis line). Add `"setup"` to the `rootUsage`-command coverage slice (`[]string{"release", "regenerate", "commit", "init", "version"}` → add `"setup"`) so a dropped `setup` line fails the test.
  - `TestRun_TopLevelHelp_ExitsZero`: optionally add a `mint setup` exit-0 assertion if not covered by `TestRunVerb_Help_ExitsZero` (the latter already covers it via the `runs` map — author's discretion).
- Add a guard test that `mint help`/`rootUsage` carries NO config reference (e.g. assert `rootUsage` does not contain a config-reference marker or a known config key table heading) — pin that `mint help` stays frozen apart from the `setup` line and never gains the config table.

**Acceptance Criteria**:
- [ ] `rootUsage` lists `setup` with a curated one-line description stating the command PRINTS/EMITS the guide (not that it writes files), column-aligned to the existing entries.
- [ ] A curated `setupUsage` const exists, opens with the `usage: mint setup` synopsis line, and notes no flags beyond `--help` and runs-anywhere/no-repo.
- [ ] `mint setup --help` prints `setupUsage` to stdout and exits 0 (pinned via the extended `TestRunVerb_Help_ExitsZero` and `TestParseFlags_HelpSurfacesErrHelp`).
- [ ] `TestUsageTexts_CoverTheirFlagSets` is extended: a setup row (synopsis pinned) and `setup` added to the `rootUsage`-command coverage slice, so dropping the `setup` line fails the test.
- [ ] `mint help`/`rootUsage` carries NO config reference and is otherwise the frozen curated text plus the one `setup` line; a guard test pins the no-config-reference fact.
- [ ] All standard gates pass.

**Tests**:
- `"it surfaces flag.ErrHelp for mint setup -h and --help"` (extended `TestParseFlags_HelpSurfacesErrHelp`)
- `"it exits 0 for mint setup --help"` (extended `TestRunVerb_Help_ExitsZero`)
- `"it lists setup in the rootUsage command list"` (extended `TestUsageTexts_CoverTheirFlagSets`)
- `"it opens setupUsage with the usage: mint synopsis line"` (extended `TestUsageTexts_CoverTheirFlagSets`)
- `"it keeps mint help free of any config reference"` (new guard test)
- `"it states the rootUsage setup description prints/emits the guide, not writes files"`

**Edge Cases**:
- `setupUsage` MUST open with the `usage: mint ` synopsis line — the coverage test's `strings.HasPrefix(tc.usage, "usage: mint ")` assertion applies to every usage text, including setup.
- `mint help` gains ONLY the `setup` line and carries NO config reference (humans get config reference from GitHub docs / README; the agent gets it from `mint setup`). `mint help` is NOT retrofitted into a dynamic renderer — it stays frozen curated text.
- The coverage test must pin `setup` in BOTH the per-verb usage register (the `setupUsage` row) AND the `rootUsage`-command slice, so dropping either fails the build.

**Context**:
> The spec (`The mint setup subcommand` → `Help-surface wiring (the curated-help contract)`) decides: a `rootUsage` command-list line for `setup` with the curated description "print the AI-assisted setup guide for configuring mint" (states it PRINTS/EMITS the guide, not that it writes files); a curated `setupUsage` for `mint setup --help` (stdout, exit 0, via `flag.ErrHelp`; one-line summary in the same register, noting no flags beyond `--help` and runs anywhere/no repo required); the existing usage-coverage test extended to pin `mint setup`.
>
> The spec decides `mint help` stays the FROZEN curated text — it gains only the `setup` command line, is NOT retrofitted into a dynamic renderer, and carries NO config reference: "humans get config reference from the GitHub docs / README; the agent gets it from `mint setup`." (`mint setup deliberately does not carry the config reference` in `mint help`.)
>
> Real codebase shape (verify at implementation time): `cmd/mint/usage.go` carries `rootUsage` (the command list) and per-verb consts (`releaseUsage`, `regenerateUsage`, `commitUsage`, `initUsage`) — `initUsage` is the closest shape for `setupUsage` (a short body, synopsis-first). `cmd/mint/usage_test.go` has four test functions: `TestParseFlags_HelpSurfacesErrHelp` (a `parsers` map keyed by verb), `TestRunVerb_Help_ExitsZero` (a `runs` map), `TestUsageTexts_CoverTheirFlagSets` (a table of `{name, usage, flags}` rows + a `rootUsage`-command slice `[]string{"release", "regenerate", "commit", "init", "version"}` + the `HasPrefix(usage, "usage: mint ")` synopsis assertion), and `TestRun_TopLevelHelp_ExitsZero`. Extend each as listed in Do.

**Spec Reference**: `.workflows/interactive-mint-init-setup/specification/interactive-mint-init-setup/specification.md` — sections `The mint setup subcommand` (Help-surface wiring — the curated-help contract; `mint help` stays the frozen curated text), `Render targets and layering` (mint help → curated text, no config reference), `Definition of done` (Help-contract coverage test).
