# Specification: AI Model Selection

## Specification

### Overview

Mint invokes an external AI for two tasks — release notes (`mint release`, and its `regenerate` path) and commit messages (`mint commit`) — through a single configurable command. Today that command is `claude -p`, which inherits whatever default model the operator's Claude CLI is set to: an external, mutable setting mint doesn't control, so output quality, cost, and latency vary silently.

This feature does three things:

1. **Pins a model in the shipped default** so behaviour is predictable regardless of the operator's CLI configuration.
2. **Lets each verb run a different AI command** by promoting `ai_command` from a shared-only key to one that also accepts a per-verb override, and introduces a parallel `timeout` key with the same shape (because per-verb model freedom and per-verb timeout freedom must travel together).
3. **Establishes `internal/config` as the single source of truth for config defaults**, removing the default-command duplication scattered across the codebase and centralizing per-verb resolution behind typed accessors.

### Scope boundaries (non-goals)

- **No driver / provider-registry pattern.** Configuring "which AI" plus a model alias, with mint knowing how to invoke each AI, is explicitly dropped (not deferred) — a raw per-verb command string already delivers the multi-AI/multi-command generality with less machinery. The driver's only residual value is ergonomics (typing `haiku` vs the full command), which doesn't justify the cost today; revisit only as sugar over the command string if a future user juggles several AIs frequently.
- **No environment-variable override layer.** A `MINT_AI_COMMAND`-style third layer is out of scope; the override layer is the project file only. Two layers: compiled defaults ← project file.
- **No interactive `mint init` prompting.** Surfacing the model choice to operators during scaffolding is a separate, deferred idea. Only init's *static* commented template surfacing the new keys is in scope here.
- **No coupling protection.** Mint does not auto-bump the timeout, warn, or require paired defaults when a verb overrides the command to a slower model — that is the operator's responsibility (detailed in its own section).

### Pinned default model

The shipped default command becomes `claude -p --model sonnet` (today: `claude -p`).

- **Alias form, not a full model ID.** The default pins `--model sonnet`, not a full versioned model ID. Full IDs baked into the binary go stale every model release and would force a rebuild just to track versions; the alias tracks the current version automatically.
- **Default model is Sonnet.** Sonnet is strong enough for the salience-heavy notes task and comfortably inside the per-attempt deadline. Opus is reserved for explicit per-verb opt-in — never the shipped default. Haiku was ruled out by operator preference for both verbs (honest technical read: probably fine for the *bounded* commit task, wrong for the *salience-synthesis* notes task — but moot once rejected outright). Ruling out Haiku collapsed the model space to a single fork — releases on Sonnet vs Opus — with commit settling on Sonnet (Opus overkill for a single bounded staged diff).
- **Not a breaking change in practice.** Moving the shipped default from bare `claude -p` to `claude -p --model sonnet` would silently switch the model for zero-config operators, but mint is a brand-new project with no users yet — there is nothing to break. **No release-note callout and no runtime signal are required.** The only real migration cost is internal: mint's own test pins that assert the old `claude -p` default (enumerated in the Migration section).

### Config schema: per-verb `ai_command` override

`ai_command` is promoted from a shared-only engine key to a key that lives at **both** levels with fallback:

- **Top-level shared `ai_command`** — the baseline (the shipped-default home). Repoints every verb in one line.
- **Optional per-verb override** — `[release].ai_command` and `[commit].ai_command`.

**Resolution order:** `[verb].ai_command` → top-level shared `ai_command` → shipped default.

**Mechanism is a full command string per verb**, not a model knob or driver. A raw command string supports any AI / model / flags with zero per-AI machinery — the transport is already content-agnostic. This is what makes a verb able to run a *different AI entirely*, not just a different model.

**Why per-verb config is warranted at all.** Experimentation ("try them and see") is already possible today with the single shared `ai_command` — edit one key, cut a few commits — so wanting to experiment does *not* by itself justify per-verb config. The justification is wanting *different* commands pinned *simultaneously*: a different AI / model / flags per verb.

**Why top-level shared baseline + optional per-verb override (not verb-only defaults):**

- "Set once for all verbs" is the common case — one model for both; per-verb is the exception. Verb-only would force editing each verb's key.
- `ai_command` is already a top-level shared engine key; per-verb override is purely additive — no churn to the "shared keys at top" principle.
- Single source of truth: verb-only defaults would bake the shipped default once per verb (more duplication); a single top-level shipped default keeps it canonical.

**Verb config space is exactly two tables: `[release]` and `[commit]`.**

- `regenerate` is **not** a separate verb. `mint release regenerate --fresh` re-runs the release-notes task, so it resolves through `[release]`'s `ai_command` — there is no `[regenerate]` table. (Regenerating with a different model than you released with would be odd, and it shares release's salience needs and timeout exposure.)
- `[commit]` simply mirrors `[release]` — same override keys, same resolution, no commit-specific asymmetry.

**Strict-decoding requirement.** The new per-verb keys must be added to both verb shape structs with `typeErrorMessages` entries, otherwise strict decoding (`DisallowUnknownFields`) rejects them.

**`ai_command` and `timeout` become the first keys living at both levels with fallback** — a small, deliberate new pattern. `max_diff_lines` / `diff_exclude` stay shared-only until their own real need appears.

### Config schema: `timeout` key

A `timeout` config key is added in the **same layering shape** as `ai_command`: a top-level shared default plus optional `[release]` / `[commit]` overrides, with resolution `[verb].timeout` → top-level shared `timeout` → shipped default.

**`timeout` is NET-NEW config surface, not a relocated default.** Unlike `ai_command` (an existing top-level config key being de-duplicated), `timeout` today exists *only* as `defaultTimeout` in the transport (`internal/ai/transport.go`) and is **never populated from config** — every wiring site constructs `ai.Config{AICommand: cfg.AICommand}` with `Timeout` left zero, relying on the transport's own self-default. This work therefore introduces a brand-new top-level shared `timeout` key plus the two per-verb overrides, all needing full new-key treatment:

- schema struct field (top-level + both verb shapes),
- `typeErrorMessages` entry,
- `defaults()` seeding at the current 60s value,
- absent-vs-zero / duration decoding.

"Mirror the `ai_command` shape" describes the **layering**, not the effort — `ai_command` is a de-dup/move; `timeout` is genuinely new surface.

**Shipped default = 60s** (the transport's current per-attempt deadline value), seeded in `internal/config`.

**Why `timeout` exists as config at all (model coupling).** Per-verb model freedom and per-verb timeout freedom must travel together. The transport's per-attempt timeout is fatal — a timeout is not retried (the single retry covers empty/error/refusal *content* only). The release verb is both where a slower model (Opus) pays off most (salience synthesis over a whole change map) and where it is most likely to time out (large input), and that failure aborts the release. Without a per-verb timeout knob, pinning a slower model would make it trivial to reliably blow the fatal deadline.

**Deferred to planning:** the key's exact TOML representation/units (int seconds vs string duration). The decoding must still distinguish absent from zero (see resolution value semantics).

**Whatever representation is chosen must preserve the value semantics.** The TOML representation is deferred, but it must support the rules in *Resolution value semantics*: distinguishing absent from an explicit zero, honoring zero, and treating negative/unparseable values as value-invalid. *Where* an invalid value is detected depends on the representation:

- **Int seconds** — a non-integer TOML value is a strict decode (type) error at `Load`; a negative integer is a value-invalid drop-through; absent vs zero is a nil pointer vs `0`.
- **String duration** — an unparseable (`"fast"`) or negative (`"-5s"`) duration decodes as a valid string, so it is a value-invalid drop-through detected at **resolution time**, not a strict decode error; absent vs zero is a nil pointer vs the zero-duration string (`"0s"`/`"0"`).

Only a genuine TOML *type* mismatch (e.g. a table where a scalar is expected) is a strict decode error in both representations.

### Resolution value semantics

Resolution is per-key **independent** — `ai_command` and `timeout` each fall back through their own `verb → shared → default` chain. The chain treats non-normal values *differently per key*.

**`ai_command`:**

- Blank / whitespace / invalid / missing at a layer **drops through** to the next layer.
- The shipped default is the floor, so resolution **always** yields a valid command — `ai_command` is never empty. Even a top-level `ai_command = ''` falls through to the shipped default.
- Consequently the transport's old "empty → re-default / empty → fail-loud" path becomes **unreachable** and is removed: config's floor always supplies a valid command.
- **Blank/whitespace detection lives in the config accessor, applied at every layer.** `AICommandFor` trims each candidate and skips it when empty — blank `[verb].ai_command` → try shared; blank shared → fall to the floor. This multi-layer trim-and-skip replaces the transport's old single blank-re-default (which only ever saw one already-resolved value); the whitespace-blank detection moves out of the transport into config. The existing `resolveAICommand` helper (today only nil-vs-present) is folded into / replaced by the accessor so blank-skipping happens in exactly one place across all layers.

**`timeout`:**

- **Zero is an explicit, honored value meaning "no time limit"** — it disables the per-attempt deadline and **stops the fall-through** (it is not treated as missing). This is a conscious, operator-chosen exception to "fail loud, never hang": the operator is opting into an AI call that can run unbounded. It **must be documented**, including that trade-off.
- **Missing or invalid (e.g. negative) drops through** to the next layer; **positive is used as-is**; the floor is the shipped 60s default.
- A wrong *type* still surfaces as a strict decode error at `Load` (existing schema behaviour) — distinct from a value-invalid drop-through.
- The transport must learn `timeout = 0` ⇒ no deadline, replacing its current non-positive → 60s re-default.
- **The transport applies the deadline conditionally.** When the resolved timeout is `0`, the transport **skips `context.WithTimeout` entirely** and runs the attempt on the parent context — it must not pass a zero duration to `WithTimeout` (which fires immediately, producing instant timeouts). The current `Timeout <= 0` defensive re-default is therefore split: `== 0` ⇒ no deadline; positive ⇒ `WithTimeout` with that value. Config guarantees the transport receives only a positive value or an explicit `0` (negatives drop through to the 60s floor in config), so no negative reaches the transport; any residual defensive handling of a negative must **not** collapse it into the `0` no-deadline branch.
- **The config→`ai.Config` boundary must preserve absent-vs-explicit-zero for `timeout`.** `ai.Config.Timeout` is today a plain `time.Duration` that every wiring site leaves at its zero value, relying on the transport's now-deleted `timeout <= 0 → 60s` self-default. Once that self-default is gone, a `time.Duration` zero is ambiguous — it is both the operator's explicit "no deadline" and the value a wiring site produces by *forgetting* to thread the resolved timeout. **Invariant: "no deadline" must only ever be reachable by an operator's explicit `0`, never by a wiring site omitting the field** — a forgotten field silently running unbounded would invert "fail loud, never hang" by omission. Planning picks the mechanism (e.g. give the boundary field a type that distinguishes nil from explicit-`0`, such as `*time.Duration` / a small wrapper; or a test-pinned contract that all three sites populate `ai.Config.Timeout` from `cfg.TimeoutFor(verb)`), but the invariant is mandatory and all three wiring sites must source the timeout from the accessor (never zero-by-omission).

### Timeout × model-choice coupling — operator's responsibility

Because resolution is per-key independent, a verb that overrides `ai_command` to a slower model but **not** `timeout` silently inherits the 60s shared default — the exact fatal-deadline exposure the override exists to prevent.

**Mint does not protect against this:**

- no auto-bump of the timeout when a slower command is configured,
- no warning,
- no paired-defaults requirement (overriding the command does not require also overriding the timeout).

If you slow the model, you raise the timeout yourself. The supported pattern — override both keys together for a slow verb — is **documented** (README/spec) but **not enforced**. Mint ships the current 60s as the shared default.

### Single source of truth for config defaults

`internal/config` is the single source of truth for config defaults; the project `.mint.toml` overrides. The model is two layers only — **compiled Go constants are the defaults (base layer)** and the **file is the override**. There is no separate defaults *file* (Go idiom: defaults are compiled).

Required shape:

- **One `defaults() Config` constructor in `internal/config`** — the canonical defaults. (Mint already runs a "defaults, overridden by the project file" model: `Load` returns `defaults()` when no `.mint.toml` exists; when a file exists, the decode target is pre-seeded with defaults and only present keys override.)
- **Layered per-verb lookup centralized in `config`** via typed accessor methods that resolve `verb override → shared top-level → default` — e.g. `cfg.AICommandFor(verb)` / `cfg.TimeoutFor(verb)`. The fallback chain lives in exactly one place; consumers just ask for the resolved value.
- **The `verb` parameter is a typed, closed enum defined in `internal/config`** — not a raw string — with exactly two values, one per verb table (`[release]`, `[commit]`). A typed enum gives compile-time safety (no string typos silently falling through to the shared baseline) and makes the regenerate routing un-missable: there is **no** `regenerate` value, so `internal/engine/regenerate_fresh.go` can only pass the *release* verb. The accessor's domain is therefore exhaustive by construction — there is no "unrecognized verb" case to handle. Exact type and constant names are a planning/impl detail.
- **The transport carries no defaults.** It runs the concrete command/timeout that config resolves and hands it. The duplicate `defaultAICommand` in `internal/ai/transport.go` is **deleted**; since config's floor always supplies a valid command, the transport never re-defaults. `timeout` is introduced as a net-new config key (today only the transport's `defaultTimeout`, never config-populated). The transport also learns `timeout = 0` ⇒ no deadline (see Resolution value semantics).
- **`initgen` pulls its template values from `config`** rather than re-typing literals (the pinned default *value* is sourced from the config constant, not re-typed).
- **No reflection, no global `config()`/`env()` service-locator** — a typed `Config` value passed explicitly, with accessor methods. The `ai`↔`config` decoupling survives: `config` never imports `ai`; consumers map `config.Config` → `ai.Config`.

**De-duplication target.** `defaultAICommand = "claude -p"` is currently duplicated across `internal/config/config.go`, `internal/ai/transport.go`, and `internal/initgen/initgen.go` (plus test pins and both specs). After this work the value lives canonically in `internal/config` and the other sites derive from it. Other `default*` consts (`defaultEditor`, git retry/backoff, the presenter leaf glyph) are operational internals, not config keys — correctly local, left alone.

### Init template scaffolding

`internal/initgen`'s commented template must surface the new keys (project convention: new config keys appear in the template and are user-facing-documented).

- **Top-level (uncommented, matching other defaulted shared keys):**
  - bump the scaffolded `ai_command` to `claude -p --model sonnet` — the pinned default *value*, **sourced from the config constant, not re-typed**;
  - add the new shared `timeout` key at its 60s default.
- **Per-verb overrides shown commented** under both `[release]` and `[commit]` — `# ai_command = …` and `# timeout = …` — so the override pattern is discoverable (optional → commented, per the template's own convention).
- **Config comments stay model-agnostic.** Comments describe what a key does and never name a specific model (no sonnet/opus/haiku, no "use a stronger model" steer). The `timeout` hint is framed around *command latency*, not a model — e.g. "raise if your `ai_command` runs slowly." The pinned default *value* still carries `--model sonnet` (that is the decided default value, not a comment). No concrete, model-tied timeout number appears in the scaffold; the mitigation hint is generic and model-free.

Exact comment wording is a planning/impl detail.

### README documentation

The README documents:

- the new keys (`ai_command` at both levels, `timeout` at both levels),
- the `verb → shared → default` resolution order,
- the new shipped default value (`claude -p --model sonnet`) — stated as a fact, not a recommendation,
- the `timeout = 0` ⇒ "no time limit" semantics, including the unbounded-call trade-off,
- the supported (unenforced) pattern of overriding command and timeout together for a slow verb.

No breaking-change callout is needed — mint has no users yet.

### Cross-spec reconciliation (commit spec)

Promoting per-verb `ai_command` formally **reverses** the commit-command spec's standing decision: "Deliberately NOT added for commit … promote to a `[commit]` key only if a real need appears." The real need has appeared, so that spec owes the reconciliation.

This is not just a spec-doc edit:

- The as-built `Commit` struct doc comment in `internal/config/config.go` encodes the old "no per-verb override" contract, and CLAUDE.md requires comments stay true to as-built **in the same change**.

**In-scope vs deferrable — resolved.** The **code-level** reconciliation is in scope for this work unit: the moment this work adds `[commit].ai_command` / `[commit].timeout`, the `Commit` struct doc comment encoding the old "no per-verb override" contract must be updated in the *same change* (CLAUDE.md requires comments stay true to as-built). Planning must **not** defer the code/comment update — deferring it would ship a comment contradicting shipped code, exactly what CLAUDE.md forbids. Only the **external commit-command spec document** revision (editing that spec's "Deliberately NOT added for commit … promote to a `[commit]` key only if a real need appears" text) may be handed to a separate commit-spec pass. Planning decides only that document-edit sequencing — not whether the in-repo comment update happens here (it does).

### Acceptance criteria — resolution behaviors

The resolution rules imply these individually-testable behaviors (project test idioms assert exact argv / rendered lines, behaviour-level proofs):

- **Per-key independence** — overriding `ai_command` on a verb leaves that verb's `timeout` resolving through the shared/floor (and vice-versa).
- **`ai_command` never empties out** — a top-level `ai_command = ''` falls through to the shipped default; a blank `[verb].ai_command` falls to shared, then to the floor.
- **`timeout = 0` honored** — resolves as "no deadline" and stops fall-through (not treated as missing); the transport skips `WithTimeout`.
- **Negative/invalid `timeout` drops through** — to the 60s floor; a positive value is used as-is.
- **Regenerate routes through `[release]`** — `internal/engine/regenerate_fresh.go` resolves the release command/timeout; argv asserted to carry the release values, not the bare shared/default (the "easy miss" wiring site).
- **Pinned default applies for zero-config** — with no `.mint.toml`, both verbs resolve to `claude -p --model sonnet` and the 60s timeout.

These are the feature's expected-behavior checklist (positive coverage). The migration test-pin breakages (the old `claude -p` argv pins, the initgen full-template-loads test) are enumerated separately below.

### Migration & mechanical carry-overs

These are factual carry-overs with no open decisions — recorded so planning doesn't rediscover them.

**Transport wiring sites (3).** The resolved per-verb command *and* timeout must be threaded where today only `ai.Config{AICommand: cfg.AICommand}` is constructed (with `Timeout` left zero):

- `internal/engine/release.go`,
- `internal/commit/run.go`,
- `internal/engine/regenerate_fresh.go` — a *distinct* construction site that must deliberately resolve through `[release]` (per "regenerate rides on `[release]`"), not its own table. An easy miss.

**Test-pin migration.** Changing the shipped default and removing the transport's `defaultAICommand` will break:

- every test that asserts the exact default command/argv (`claude -p` with no `--model`),
- the initgen "full template loads cleanly" test.

Project test idioms assert exact argv / rendered lines, so these are known, bounded edits — enumerate them in planning.

**Transport doc-comment migration (same-change, per CLAUDE.md).** The transport's WHY-comments hard-encode the contracts this work deletes and must be corrected in the *same change* — the symmetric obligation to the `Commit` struct comment in *Cross-spec reconciliation*. Enumerated so they aren't missed:

- `Config.AICommand` — "(default `claude -p` when empty)";
- `Config.Timeout` — "A zero or negative Timeout falls back to the production default.";
- `NewTransport` — "An empty AICommand resolves to `claude -p` and a non-positive Timeout resolves to the ~60s production default, so the zero Config yields a fully working production transport.";
- `Generate` / `attempt` — "Each attempt gets its own deadline via `context.WithTimeout(ctx, t.timeout)`" (becomes conditional once `timeout = 0` skips `WithTimeout`).

---

## Working Notes

[Optional - capture in-progress discussion if needed]
