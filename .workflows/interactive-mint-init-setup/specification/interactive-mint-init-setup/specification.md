# Specification: Interactive Mint Init Setup

## Specification

### Overview

`mint init` stays a pure static-file generator — it grows **no** interactive surface. Interactivity is offloaded to an **AI setup guide emitted by the binary**: a new thin `mint setup` subcommand prints an embedded static instruction string that teaches an AI agent (Claude or any AI) how to configure mint for a project — what mint does, what each config option means, what to inspect in the project (existing release process → hooks, noise dirs → `diff_exclude`, version file, AI model per verb), and how to propose and confirm changes interactively. The agent does the research and runs the natural-language setup session; **mint itself never prompts**.

This supersedes the original framing ("make `mint init` interactive"). The "augment only what varies" principle survives as the guide's minimalism rule, but there is no mint-side overlay or wizard — the agent does the tailoring.

### Goal (end state)

A `.mint.toml` that is **up to date with the installed mint version** and **contains the project's relevant config — whether sourced from an existing file or built fresh**. "Setup" therefore doubles as "upgrade my config to this mint version."

### Why binary-emit is the keystone

Emitting the instructions from the binary means they:
- **Cannot drift** from the schema that binary implements — drift-testable, exactly like the existing `initgen`↔`config` drift tests.
- **Cannot version-skew** — an old mint emits old-but-correct instructions, so there is no N-versions doc maintenance.
- Are **reachable** wherever mint is installed (no clone/fetch step).
- Are **human-readable**, so a user with no agentic AI or no web access can still read and follow them.

This turns the deliverable into a **thin Go feature** (subcommand + embedded string + drift test) that slots into mint's existing generator + cmd + test patterns.

### Non-goals / out of scope

- **No mint-side interactive wizard.** mint never prompts during init; the fail-loud/non-interactive (`-y`/non-TTY hang) concern is dissolved because mint never reads stdin for setup.
- **No ecosystem-aware static `diff_exclude` defaults** (option B — explicitly rejected). A wrong default is worse than none (`docs/**` is wrong for a docs tool); the guide surfaces patterns interactively instead. Accepted tradeoff: the non-AI path gets no `diff_exclude` help.
- **No in-init AI calls / no AI in `engine.Init`.** The in-init AI `diff_exclude` scan is deferred (sketch preserved in the discussion); init stays AI-free.
- **No auto-install.** Setup assumes mint is installed; if `mint setup` isn't found the agent asks the user to install — mint does not install itself (a binary install is a far larger blast radius than editing a config).
- **Deferred to possible future work:** per-verb `timeout` auto-write for slow models; a Claude-Code skill wrapper.

### The `mint setup` subcommand

A new top-level verb (working name `mint setup`; the exact command name is a planning detail) that emits the AI setup guide. It is a **pure string emitter** in the spirit of `initgen` — it prints an embedded static instruction string and performs no IO beyond writing to stdout.

**What it emits** (one embedded string, all version-matched to the installed binary):
1. The setup **procedure** (the inspect-and-map flow — see the guide-content sections).
2. The AI **etiquette** rules.
3. The **config reference** — rendered from the config-metadata source of truth (the `key · level · default · description` table) so the agent reads option meanings from a drift-tested table rather than from template comments.

**Stable section markers.** Although the guide *prose* is a planning detail, each required section emits a **stable, test-detectable marker** (a fixed section heading/anchor) so the structural test can prove a section's presence without coupling to the body prose. The required sections with markers: pipeline/hook model, etiquette, minimalism, the existing-config/upgrade branch, and the config reference. The structural test greps for these markers, not for representative prose substrings (the exact marker text is a planning detail; that markers exist and the test keys on them is decided).

**Runs unconditionally — no git/cwd guard.** Unlike `mint init` (which resolves the repo root via `git rev-parse --show-toplevel` and fails loudly outside a work tree), `mint setup` is a pure text emitter: it must print even when the operator is not yet inside the target repo (setup instructions are commonly read before `cd`-ing in). Safety lives in the *instructions* instead — the emitted guide tells the agent to confirm it is in the intended repo root before inspecting or writing, and `mint init` (run during setup) remains the loud-fail backstop outside a work tree.

**Help-surface wiring (the curated-help contract).** `mint setup` threads through mint's existing hand-written help surface exactly like every other verb:
- A `rootUsage` command-list line for `setup`.
- A curated `setupUsage` text for `mint setup --help` (printed to stdout, exit 0, via the `flag.ErrHelp` path).
- Dispatch wiring in `classifyCommand` / `run` (a new `commandSetup` route).
- The coverage test the help contract requires (the existing test that pins every verb's usage coverage).

`mint help` stays the **frozen curated text** — it gains only the `setup` command line and is **not** retrofitted into a dynamic renderer. `mint help` deliberately does **not** carry the config reference: humans get config reference from the GitHub docs / README; the agent gets it from `mint setup`.

**Install handling.** The README entry point assumes mint is installed and links the install. If `mint setup` is not found, the agent asks the user to install mint — mint does **not** auto-install itself.

### Generated config: strip to minimal

The generated `.mint.toml` is **stripped to bare essentials** — no inline key comments, no commented example overrides. The pre-pivot template carried the only in-repo config docs, so stripping would have hurt; post-pivot the binary is the doc source (`mint setup` SoT table for the agent; GitHub docs / README for humans), so the comments are pure duplication — exactly the drift surface mint already fights. Stripping removes that drift surface and yields a clean minimal file.

**Micro-choice (decided): empty body + a short header comment.** Because every default is compiled into the binary, an empty file is valid and honest. The header comment points to:
- the **GitHub docs** for the config reference, and
- **`mint setup`** for AI-assisted setup.

This header pointer is also the **recovery net** for the cold-arrival case — an agent (or human) that opens `.mint.toml` directly, without having read `mint setup`, still finds where the config reference lives.

### `initgen` scope of change

- **Only `MintTOML()` changes** — it returns the new minimal string (empty body + header). `ReleaseShim()` and its tests are **untouched**.
- The existing `initgen` seams are respected: `initgen` deliberately does **not** import `config` and is drift-pinned. (Package layout for the new config-metadata SoT is a planning detail.)
- **`initgen` test impact (not a light edit).** The current `initgen` test suite is built around the commented-template contract; the tests that assert that shape are **removed**, not edited — they assert content the minimal template no longer carries (common keys active at defaults, optional keys present-but-commented, per-verb override comments, `pre_tag` shown in both string and array forms, prompt-override-mentioned-in-comment-only, hooks-only-under-`[release.hooks]`, and uncommented-loads-cleanly). **New** `initgen` tests assert the minimal shape instead: the body is empty (no active or commented keys) and the **header carries both pointers** (GitHub docs + `mint setup`) — the header is load-bearing (the cold-arrival recovery net), so its pointers are pinned by a test.
- **The scaffold-value drift-pin moves to the SoT.** Today `initgen`'s drift tests pin the scaffold's literal default values (`ai_command`, `timeout`) equal to `config.DefaultAICommand` / `config.DefaultTimeout`. The minimal template carries **no** default values to pin, so that value-drift discipline is **subsumed by the new SoT drift test** — the SoT `default` column becomes the drift-pinned carrier of those values. No default value is left unpinned by the change.

### Release shim and `mint init` unchanged

- `mint init` still emits **both** files at the git-resolved repo root, unchanged in behaviour: `.mint.toml` (now minimal) **and** the executable `release` shim. The non-clobbering / `--force` / idempotent disposition is unchanged; strip-to-minimal is a **config-content-only** change.
- Shim tests stay as-is.
- The `mint setup` guide gives the shim a **one-line role mention** — what `./release` is, and that `mint init` creates it — so the agent's picture of the release pipeline is complete.

### Config-metadata source of truth (SoT)

A single structured table of config metadata lives **in the binary**, schema-adjacent — one row per config key with columns **key · level · default · description**:
- **key** — the TOML key name (e.g. `ai_command`, `tag_prefix`, `pre_tag`).
- **level** — where it lives (top-level shared, `[release]`, `[release.hooks]`, `[commit]`).
- **default** — the **real** compiled default (e.g. `claude -p --model sonnet`, `60`, `v`, `true`), not an illustrative example.
- **description** — the one-line meaning.

This SoT is the **single in-binary source of config metadata**. It **renders into the `mint setup` output** as the config reference the agent reads — replacing the now-stripped template comments. (Package layout / exact rendering is a planning detail.)

**`default` column representation.** Many keys have no concrete scalar default, so the column adopts the **same representation convention the README per-key tables already use** (keeping the two surfaces mutually consistent):
- **Empty-string defaults** (`context`, `prompt`, `fallback`, `version_file`, `version_pattern`) → an empty/blank cell.
- **Sentinel-empty "auto" defaults** (`release_branch`, `provider`, where `""` *means* auto-derive/auto-detect) → the word **`auto`** (distinct from a plain blank, so the agent can tell "auto" from "no value").
- **Empty collection** (`diff_exclude`) → **`[]`**.
- **Per-verb override "inherit-the-shared" defaults** (`[release].ai_command`/`timeout`, `[commit].ai_command`/`timeout`) → the word **`shared`**.
- **`[release.hooks]` keys** (`preflight`, `pre_tag`, `post_release`) — activate-only, no compiled default → no default value; the description carries when each runs (the `default` cell is blank or `—`).

The minimalism guidance depends on this column being unambiguous, so these representations are part of the decided behaviour, not a planning-only rendering choice.

### Drift test (the anti-drift enforcement)

The SoT is **drift-tested against the real `config` schema** — a Go test that fails the build if the SoT and the canonical schema disagree (a key present in the schema but missing from the SoT, or vice versa). This is the core value of centralising the metadata: the config reference the agent reads **cannot drift** from what the binary actually accepts, even though `mint setup` is the single in-binary render target. It mirrors the existing `initgen`↔`config` drift discipline.

**What counts as one "key" (the bijection contract).** The schema's key set is not a flat list of distinct names, so the test matches on **(level, key) pairs**, one SoT row per pair:
- `ai_command` and `timeout` exist at **both** the shared top level **and** under `[release]` and `[commit]` — these are **distinct rows** (mirroring the README's four-row model), matched per level, not collapsed to one.
- The `[release.hooks]` keys (`preflight`, `pre_tag`, `post_release`) are their own rows at the `[release.hooks]` level.
- The authoritative key set is **derived mechanically from the `config` decode-shape structs' `toml` tags** (`fileShape`, `releaseShape`, `commitShape`, `hooksShape`) — **not** a hand-maintained list — so the test catches a genuine schema↔SoT divergence rather than comparing two copies of the same hand-list. The bijection is total: every `toml`-tagged schema field has exactly one matching SoT row at its level, and every SoT row maps back to a schema field. (The exact reflection helper / package layout is a planning detail; the *contract* — derive from the schema, match per (level, key), bijective — is decided.)

### Render targets and layering

- **`mint setup`** → renders the SoT config table (the **agent's** machine-readable config reference).
- **`mint init`** → minimal config (empty body + header).
- **`mint help`** → curated text, **no** config reference (stays frozen).
- **GitHub docs / README** → the **human** config reference.

The SoT earns its keep via the drift test even with a single in-binary render target.

### Emitted guide — setup procedure

The `mint setup` output carries an **inspect-and-map procedure** the agent follows. Hook/release-process detection is the centrepiece.

Ordered flow:
1. **Confirm the working directory** is the intended repo root before inspecting or writing anything (this is the in-instructions safety net that replaces `mint setup`'s missing cwd guard).
2. **Learn mint** — read the README and internalise mint's minimalist philosophy (*only set what varies*).
3. **Read the config reference** — an **explicit, ordered early step**, performed **before** any inspect/edit. Because the stripped template no longer lists keys in-repo, the flow depends on the agent holding the config reference from `mint setup`'s SoT table. Making this a required ordered step closes the cold-arrival gap; the minimal file's header pointer is the recovery net if the agent ever arrives at the file without it.
4. **Inspect the project and map findings to config:**
   - existing **release process** → mint's **hooks** (the centrepiece — see below);
   - **noise dirs** → `diff_exclude`;
   - **version file** → `version_file` / `version_pattern`;
   - **AI model per verb** → `ai_command` (shared or per-verb — see representation below);
   - **provider / release branch** → only if auto-detect would be wrong.
5. **Propose → explain → approve** — present the proposed config, explain each choice, get the user's approval (per the etiquette rules).
6. **Sanity-check** — unknown/renamed keys fail loudly at the next `mint` run via the strict schema (`DisallowUnknownFields`); a natural "verify the config loads" step is the backstop.

### mint's pipeline / stage model (a required content section)

The emitted instructions **must carry mint's pipeline/stage model** — the ordered stages and which hook fires where — otherwise the agent cannot explain or map a release process accurately. This content is drift-sensitive: it must match the engine.

The pipeline, with the three hook phases bracketing it:

```
preflight → notes → pre_tag → tag + push (PONR) → publish → post_release
```

- **`preflight`** — runs before any release work; failure aborts the release.
- **`pre_tag`** — runs after notes, before the tag; **accepts an array** of ordered commands.
- **`post_release`** — runs after the release is published.

The tag + atomic push is the **point of no return (PONR)**.

### Hook detection & mapping

- **Propose a best-fit mapping and flag it to the user.** Never silently skip a step — "if it's in the customer's release script, it's important."
- When a step **doesn't fit**, surface it honestly. The outcome may legitimately be "mint isn't suitable here" or "you'll need to adjust your process" — acceptable, not a failure to paper over.
- The agent's job is to **explain mint's model clearly** (the pipeline + where each hook slots in) so a technical user can collaborate on a workaround, an adaptation, or a clean fit. The instructions **facilitate that conversation**; they don't force a mapping.
- `pre_tag`-as-array widens what fits: a linear multi-step build/test sequence maps to a `pre_tag` array, so the genuinely-unmappable set narrows to *needs a step where mint has no hook* / non-linear / mid-pipeline approval-gate cases.

### Cross-cutting principle — agent as collaborator (not auto-configurer)

Generalises beyond hooks: the guide makes the agent a knowledgeable collaborator — **explain mint's model, propose, flag, never silently drop or clobber, help the user fit their process or recognise a genuine misfit.** Not a magic one-shot configurer.

### AI model per verb — config representation

The agent asks in natural language (model choice → same for both verbs, or different → `custom` escape for any non-Claude command). What lands in `.mint.toml`:
- **"same"** → write the shared top-level `ai_command` (today's shape).
- **"different"** → write `[release].ai_command` + `[commit].ai_command` as per-verb overrides.

### `diff_exclude` scope

`diff_exclude` is for **release-notes noise, not generated code.** `.gitignore`'d paths (`node_modules`, `vendor`) are already absent from the diff. The real targets are tracked process/meta/doc files — `.workflows/`, `.claude/`, agent dirs, `docs/`, lockfiles. The guide surfaces these patterns **interactively** (inside the confirmation step), since the right set is project-specific.

### Emitted guide — AI etiquette

The guide instructs the agent to:
- **Ask interactively** using whatever user-question tool it has — phrased AI-agnostically: "if you're Claude, use your Ask-User tool; otherwise, if you have any tool for asking the user questions, use it."
- **Confirm the user is comfortable**, and make **clear exactly what is being changed/updated** before writing.
- **Never remove anything without explicit permission.**
- Surface the `diff_exclude` patterns inside this interactive confirmation.

### Emitted guide — minimalism (only set what varies)

Mirror risk to hook-mapping: a thoroughness-rewarded agent populates every inferable key and hands back a bloated `.mint.toml`, defeating the good-defaults design that motivated the pivot. The guide must prevent this:

- **Add (activate) a key ONLY to set a non-default value.** If the default is fine, **omit the key entirely** — that is NOT 'skipping' or 'disabling': mint's defaults are compiled into the binary and apply whether or not the key appears in the file. The whole `.mint.toml` is optional. (Post-strip, the generated file is empty-body, so "minimal" means the agent adds only the keys it has a project-specific reason to set — there are no pre-commented keys to leave alone.)
- **For every key the agent does activate, state the project-specific reason** at the confirmation step — so over-configuration is *visible* (each active key carries a justification), not silent.
- **Read defaults from the config reference, never guess.** The `mint setup` SoT table shows each key's real default (the `default` column). The agent judges "is the default fine?" against that real default — it must not infer a default from an illustrative example value (e.g. in the human README), and the guide never restates default values (DRY).

### Emitted guide — existing `.mint.toml`: diff, discuss, upgrade

Re-running setup on a configured repo is a likely motion. mint's generator is strictly non-clobbering (`mint init` skips an existing file unless `--force`); the agent needs the same instinct, made explicit in the setup output:

- If a `.mint.toml` already exists, **bring it into context and discuss** — never silently overwrite. The agent surfaces what's there and asks.
- Offer the choice: **work with the existing config** (targeted, key-by-key, reviewed changes) **or start fresh reusing the existing values** to build a clean current file — always the user's call; nothing removed without explicit permission.
- **Config upgrade / migration (emergent capability):** the agent has the current canonical reference (the SoT via `mint setup`) plus the user's file, so it can detect **drift from the current mint version** — removed/renamed keys (which would otherwise fail `DisallowUnknownFields` loudly at the next `mint` run), values that no longer fit, new keys worth considering — and bring an old config current. "Setup" doubles as "upgrade my config to this mint version."

### README — entry point

The README carries the **tiny entry-point prompt** that points operators at the binary-emitted guide — roughly: *"mint is an AI tool for commits & releases; run `mint setup` and follow what it prints."* The README does **not** restate the guide; it routes to it (the binary is the version-matched source).

### README — "any AI" framing (light, no machinery)

Frame the entry point as: *"to set up, pass the following prompt to your AI of choice — Claude, Codex, …"*, with a light steer like *"we find Opus-level models do the best work here."* There is **no fidelity-floor machinery** — this is a convenience; if the user picks a weak AI, that's their call. The strict-schema loud-fail and a natural "verify the config loads" step remain as sensible backstops, not defensive engineering.

### README — config reference verification

The README **stays manual narrative** (updated per-feature) and is the **human** config reference surface. As part of this work it is **verified to declare every config key + its default**. An optional cheap **tripwire test** may be added — assert that every schema key name appears somewhere in the README. README descriptions may lightly duplicate the SoT — **accepted**: the README is the human GitHub-browsing surface, while the machine/agent surfaces (`mint help` + `mint setup`) are the ones held to a single SoT.

- **Reconcile the existing "Configuration" section with strip-to-minimal.** The README's current `## Configuration` intro (*"`mint init` writes a commented `.mint.toml`"*) and the embedded full commented-template TOML block both become false after the strip. As part of this work: (a) correct the framing — `mint init` now writes a **minimal** `.mint.toml` (empty body + header pointer); (b) **replace** the embedded full-template block with the new minimal template (empty body + header), or drop it — the per-key reference tables already below it (`Shared engine keys` / `[release]` / `[release.hooks]` / `[commit]`) are the authoritative human config reference and the surface the tripwire test checks; (c) correct the Commands-section line (the `mint init … writes a commented .mint.toml …` entry) the same way.

### Definition of done

The deliverable is mostly a thin Go feature, so most of "done" lands under the normal gates (`go build` / `gofmt` / `go vet` / `go test -race` / `golangci-lint`):

- **Drift test** — config-metadata SoT ↔ the actual `config` schema (fails the build if they disagree).
- **Structural test on `mint setup` output** — asserts it emits the required sections: pipeline/hook model, etiquette, minimalism, the if-exists/upgrade branch, and the config table. Detection is by the **stable section markers** the guide emits (see "The `mint setup` subcommand" → Stable section markers), not by matching free prose.
- **Updated `initgen` tests** — the commented-template assertions are removed; new tests assert the minimal shape (empty body + header pointers to GitHub docs + `mint setup`). The scaffold-value drift-pin is subsumed by the SoT drift test. `ReleaseShim()` tests untouched. (See "`initgen` scope of change" for the full list.)
- **Help-contract coverage test** — the existing usage-coverage test extended to pin `mint setup` (rootUsage line + curated `setupUsage`).
- **README tripwire (optional)** — assert every schema key name appears in the README.

### Acceptance — prose quality (the one part with no compiler)

Whether the emitted prompt actually drives a good config can't be unit-tested without spawning an AI (forbidden by the test culture). The acceptance bar is a **one-time manual run** against representative repos, eyeballing that each yields a sensible config:
- a fresh JS project,
- a Go project,
- a repo with an existing release script,
- a repo with an existing `.mint.toml`.

---

## Working Notes

### Reconciliation: F8 minimalism vs strip-to-minimal (confirmed with user)

The discussion's **F8 minimalism** language was written against the pre-strip commented template ("leave the key commented"; comments as "inert documentation"; "don't treat the commented example value `# timeout = 120` as the default"). The **strip-to-minimal** decision replaces that template with an empty body + header, and the **SoT config table** carries a real `default` column. Reconciled in the "minimalism (only set what varies)" section by: (1) restating the rule as "add a key only to set a non-default value; otherwise omit it" rather than "leave it commented"; (2) replacing the example-value warning with "read the real default from the SoT table's `default` column." The minimalism *principle* (absent = at default, not off; don't over-configure) is unchanged. User confirmed this reconciliation during construction.
