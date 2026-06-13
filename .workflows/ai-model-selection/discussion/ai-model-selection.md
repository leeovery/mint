# Discussion: AI Model Selection

## Context

Mint's default `ai_command` is `claude -p`, which inherits whatever default model the operator's Claude CLI is configured for — likely an overpowered, slower, more expensive model than mint's two AI tasks (release notes, commit messages) actually need. The immediate want is to **pin a model in the shipped default** so behaviour is predictable regardless of the operator's CLI settings — probably via the alias form (`claude -p --model sonnet`) rather than a full model ID, since baked-in full IDs go stale as new model versions ship.

That small change opened a larger design question: **should the two verbs use different models at all?** The commit-command spec currently mandates a single shared top-level `ai_command` with **explicitly no per-verb override** ("promote to a `[commit]` key only if a real need appears"). This discussion is the test of whether that real need has appeared.

A related "ideal world" framing was raised in discovery: rather than configuring a raw command string, pick *which AI* you're using and set the model as config, with mint knowing how to invoke each AI (a driver-based pattern).

Code-health angle: the default command string is duplicated across three sites — `internal/config/config.go:75`, `internal/ai/transport.go:45`, `internal/initgen/initgen.go:39` — plus test pins and both specs. One coupling to keep in view: the transport's per-attempt timeout is 60s and timeouts are **fatal (not retried)**, so slower models interact directly with that deadline.

### References

- Seed: `seeds/2026-06-11-ai-model-selection.md` (inbox:idea)
- Discovery: `discovery/session-001.md`
- Prior decision being revisited: commit-command spec — `ai_command` is a shared engine key, no per-verb override (KB)
- Touch sites: `internal/config/config.go:75`, `internal/ai/transport.go:45`, `internal/initgen/initgen.go:39`

## Discussion Map

A living index of subtopics tracked during the discussion. Grows as the conversation branches, converges as decisions land.

### States

- **pending** (`○`) — identified but not yet explored
- **exploring** (`◐`) — actively being discussed
- **converging** (`→`) — narrowing toward a decision
- **decided** (`✓`) — decision reached with rationale documented

### Map

  Discussion Map — AI Model Selection (8 subtopics — all decided)

  ┌─ ✓ Pin A Model In The Shipped Default [decided]
  │  └─ ✓ Alias Form Vs Full Model ID [decided]
  ├─ ✓ Per-Verb Model Differentiation [decided]
  │  └─ ✓ Config Shape: Top-Level Shared + Per-Verb Override [decided]
  ├─ ✓ Timeout × Model-Choice Coupling [decided]
  ├─ ✓ Driver-Based AI Config — Dropped [decided]
  ├─ ✓ Single Source Of Truth For The Default Command [decided]
  └─ ✓ Init Scaffolds The New Config Keys [decided]

---

*Subtopics are documented below as they reach `decided` or accumulate enough exploration to capture.*

---

## Pin A Model In The Shipped Default

### Context

`claude -p` inherits whatever model the operator's Claude CLI defaults to — so output quality, cost, and latency silently depend on an external, mutable setting mint doesn't control. Pinning a model in the shipped default removes that dependence.

### Decision

Pin via the **alias form** (`--model sonnet`), **not a full model ID** — full IDs baked into the binary go stale every model release and would force a rebuild just to track versions; the alias tracks the current version automatically.

**Shared default model = Sonnet** (confirmed) — strong enough for the salience-heavy notes task and comfortably inside the 60s deadline; Opus reserved for explicit per-verb opt-in.

**Not a breaking change in practice — no users yet (decided).** Moving the shipped default from bare `claude -p` to `claude -p --model sonnet` *would* silently switch the model for zero-config users — but mint is a brand-new project with no users yet, so there is nothing to break: **no release-note callout and no runtime signal are needed.** (Had operators existed, the mitigation would have been a release note; the *proper* surfacing — operator choice at setup — rides on the deferred interactive `mint init` feature.) The only real migration cost is internal — mint's own test pins that assert the old default (see Notes for Spec/Planning).

## Per-Verb Model Differentiation

### Context

The crux. Should release notes and commit messages run different models, and does that justify promoting `ai_command` from shared-only to per-verb-overridable (the "real need" the commit spec deferred to)?

### Journey

- **First collapse.** Haiku was ruled out by preference for both verbs (user doesn't trust it for either), eliminating the "Sonnet release / Haiku commit" pairing. That reduced the whole 2×2 to one fork: **releases on Sonnet or Opus?** — because commit → Sonnet was near-settled (Haiku rejected, Opus overkill for a single bounded staged diff). Honest read on Haiku: probably fine for the *bounded* commit task, wrong for the *salience-synthesis* notes task — but moot once the user rejected it outright.
- **Experimentation is not the justification.** "Try them and see" is already possible *today* with the single shared `ai_command` — edit one key, cut a few commits. So wanting to experiment does NOT by itself justify per-verb config; the real justification is wanting *different* models pinned *simultaneously*.
- **The real need surfaced.** The user argued per-verb is worth it "in any case" — a user might want a different AI entirely, or a different command, per verb.
- **Mechanism insight.** That generality is best served by a **raw per-verb command string**, NOT a driver. A driver only supports AIs mint codes for; a full command string supports any AI / model / flags with zero per-AI machinery (the transport is already content-agnostic). The user's "different AIs" instinct argues *for* per-verb override and *against* the driver in one move.

### Decision

**Per-verb `ai_command` override is warranted — promote it.** (Confidence: high, user-confirmed.) Mechanism: a **full command string per verb**, not a model knob or driver. Resolution order: `[verb].ai_command` → top-level shared `ai_command` → shipped default. Commit leans Sonnet, release leans Sonnet (Opus only on observed need — see timeout coupling).

#### Config shape (child) — decided

**Keep the top-level shared key as the baseline; per-verb keys are optional overrides** (confidence: high, user-confirmed). Applies to *both* `ai_command` and the new timeout key: a top-level shared default (the shipped-default home) + optional `[release]`/`[commit]` overrides. Resolution order: `[verb].<key>` → top-level shared `<key>` → shipped default.

Why not verb-only defaults:

- **"Set once for all verbs" is the common case** — one model for both; per-verb is the exception. The shared key repoints every verb in one line (e.g. swap to a different AI). Verb-only would force editing each verb's key.
- **Established schema** — `ai_command` is already a top-level shared engine key; per-verb override is purely additive, no churn to the "shared keys at top" principle.
- **Clincher — single source of truth.** Verb-only defaults bake the shipped default *once per verb* — more duplication, fighting the cleanup we want. A single top-level shipped default keeps it canonical.

`ai_command` and timeout become the first keys living at *both* levels with fallback — a small, deliberate new pattern. `max_diff_lines`/`diff_exclude` stay shared-only until their own real need appears.

**`regenerate` is not a separate verb (decided).** `mint release regenerate --fresh` re-runs the release-notes task, so it resolves through `[release]`'s `ai_command`/timeout — no `[regenerate]` table. Regenerating with a different model than you released with would be odd, and it shares release's salience needs and Opus-on-big-diff timeout exposure. So the per-verb config space is exactly two tables: `[release]` (covers release + regenerate) and `[commit]`. (Confidence: high, user-confirmed.)

**Mechanical consequences (for spec/planning).** `[commit]` simply mirrors `[release]` — same two override keys (`ai_command`, `timeout`), same resolution, no commit-specific asymmetry. The keys must be added to both verb shape structs with `typeErrorMessages` entries, else strict decoding (`DisallowUnknownFields`) rejects them. This formally **reverses the commit spec's "Deliberately NOT added for commit … promote to a `[commit]` key only if a real need appears"** — the real need has appeared; that spec owes the reconciliation (cross-spec hand-off).

**Resolution value semantics (decided — supersedes the earlier "empty/zero fails loud" phrasing).** The `verb → shared → default` chain treats non-normal values *differently per key*:

- **`ai_command`** — blank / whitespace / invalid / missing at a layer **drops through** to the next layer. The shipped default is the floor, so resolution always yields a valid command; `ai_command` is never empty, and the transport's old empty→re-default / empty→fail-loud path becomes unreachable (even a top-level `ai_command = ''` falls to the shipped default).
- **`timeout`** — **zero is an explicit, honored value meaning "no time limit"** (disables the per-attempt deadline); it stops the fall-through and **must be documented**, including the trade-off that the operator is opting into an AI call that can run unbounded — a conscious, operator-chosen exception to "fail loud, never hang." **Missing or invalid (e.g. negative) drops through**; positive is used as-is; floor = the shipped 60s default. A wrong *type* still surfaces as a strict decode error at Load (existing schema behaviour), distinct from a value-invalid drop-through. The transport must learn `timeout = 0` ⇒ no deadline, replacing its current non-positive→60s re-default.

## Timeout × Model-Choice Coupling

### Context

The transport's per-attempt timeout is 60s and a timeout is **fatal — not retried** (the single retry covers empty/error/refusal *content* only). Slower models (Opus) on big diffs hit this deadline directly: the release verb is both where Opus pays off most (salience synthesis over a whole change map) and where it's most likely to time out (large input), and that failure aborts the release.

### Decision

**Add `timeout` as a config key with a per-verb override** (confidence: high, user-confirmed), in the same *layering* shape as `ai_command`: a top-level shared default (the current 60s value) + optional `[release]`/`[commit]` override. Rationale: per-verb model freedom and per-verb timeout freedom must travel together — otherwise we've made it trivial to pin a model that reliably blows the fatal deadline. Key naming and per-attempt semantics are an implementation detail left to spec/planning.

**Correction — `timeout` is NET-NEW to the config schema, not a relocated default.** Unlike `ai_command` (an existing top-level config key), `timeout` today exists *only* as `defaultTimeout` in `transport.go:64` and is **never populated from config** — every wiring site constructs `ai.Config{AICommand: cfg.AICommand}` with `Timeout` left zero, relying on the transport's own self-default. So this work introduces a **brand-new top-level shared `timeout` key plus the two per-verb overrides**, all of which need full new-key treatment: schema struct field, `typeErrorMessages` entry, `defaults()` seeding, and absent-vs-zero/duration decoding. "Mirror the `ai_command` shape" describes the *layering*, not the effort — `ai_command` is a de-dup/move; `timeout` is genuinely new surface. (This is the one place the otherwise-code-grounded discussion assumed symmetry that doesn't hold.)

**Coupling is the operator's responsibility (decided).** Resolution is per-key *independent* — `ai_command` and `timeout` each fall back to their own shared default. So a verb that overrides the command to a slow model but *not* the timeout silently inherits the 60s shared default — the exact fatal-deadline exposure the override exists to prevent. Mint does **not** protect against this: no auto-bump, no warning, no paired-defaults requirement. "That's the operator's worry." If you slow the model, you raise the timeout yourself; the supported pattern is documented (README/spec) but not enforced. Mint ships the current 60s as the shared default. (Confidence: high, user-confirmed.)

## Driver-Based AI Config — Dropped

### Context

The seed's "ideal world": configure *which AI* + a model alias, with mint knowing how to invoke each AI (driver pattern). Flagged as likely YAGNI in discovery.

### Decision

**Dropped** (confidence: high) — not merely deferred-on-YAGNI, but *superseded*. The raw per-verb command string already delivers the multi-AI / multi-command generality the driver was meant for, with less machinery and no provider registry to maintain. The driver's only residual value is ergonomics (typing `haiku` vs the full command), which doesn't justify the cost today. Revisit only as sugar over the command string if a future user juggles several AIs frequently.

## Single Source Of Truth For The Default Command

### Context (verified against code, 2026-06-13)

- **Config is optional, not required.** `config.Load` returns `defaults()` when no `.mint.toml` exists (`config.go:285`); when a file exists, the decode target is pre-seeded with defaults and only present keys override (`config.go:291`). Mint already runs a "defaults, overridden by the project file" model.
- **Every key has a built-in default**, clustered in `internal/config/config.go` (`defaultTagPrefix`, `defaultCommitPrefix`, `defaultPublish`, `defaultChangelog`, `defaultOnNotesFailure`, `defaultMaxDiffLines`, `defaultAICommand`).
- **The scattering that matters:** `defaultAICommand = "claude -p"` is **duplicated** in `ai/transport.go:45`, and `defaultTimeout = 60s` lives **only** in the transport. Other `default*` consts (`defaultEditor "vi"`, git retry/backoff, the presenter leaf glyph) are *operational internals, not config keys* — correctly local, left alone.

### Decision

**`internal/config` is the single source of truth for config defaults; the project `.mint.toml` overrides.** (Confidence: high, user-confirmed.) The model is Laravel-shaped with one inversion made explicit: the **compiled Go constants are the defaults (base layer)** and the **file is the override** — there is no separate defaults *file* (Go idiom: defaults are compiled). Two layers only.

Idiomatic Go shape:

- One `defaults() Config` constructor in `internal/config` — the canonical defaults.
- `Load` decodes the project file *over* a defaults-seeded target (present keys win); missing file → `defaults()`. (Already how mint works.)
- **Layered per-verb lookup centralized in config** via typed accessor methods that resolve `verb override → shared top-level → default` — e.g. `cfg.AICommandFor(verb)` / `cfg.TimeoutFor(verb)`. The fallback chain lives in exactly one place; consumers just ask for the resolved value.
- **The transport carries no defaults** — it runs the concrete command/timeout config resolves and hands it; config's floor always supplies a valid command, so the transport never re-defaults. `transport.go:45`'s duplicate `defaultAICommand` is deleted; `timeout` is introduced as a *net-new* config key (today only `transport.go:64`'s `defaultTimeout`, never config-populated — see the Timeout subtopic correction). The transport also learns `timeout = 0` ⇒ no deadline (see resolution value semantics).
- **`initgen` pulls its template values from `config`** rather than re-typing literals.
- No reflection, no global `config()`/`env()` service-locator — a typed `Config` value passed explicitly with accessor methods. The `ai`↔`config` decoupling survives (config never imports ai; consumers map `config.Config` → `ai.Config`).

**Env-var overrides are out of scope** — the override layer is the file; an environment-variable third layer (`MINT_AI_COMMAND`-style) is a separate, addable feature, not built here.

## Init Scaffolds The New Config Keys

### Context

New config keys must appear in `internal/initgen`'s commented template (project convention) and be user-facing-documented. Current template: top-level `ai_command = 'claude -p'` + `max_diff_lines`; an uncommented `[release]`; a fully-commented `[commit]` (only `context`/`prompt`).

### Decision

- **Top-level**: bump the scaffolded default to `ai_command = 'claude -p --model sonnet'` (the pinned default *value*, sourced from the config constant — not re-typed), and add the new shared `timeout` key at its 60s default. Both uncommented, matching the other defaulted shared keys.
- **Per-verb overrides shown commented** under both `[release]` and `[commit]` — `# ai_command = …` and `# timeout = …` — so the override pattern is discoverable (optional → commented, per the template's own convention).
- **Config comments stay model-agnostic (user constraint).** Comments describe what a key does and never name a specific model (no sonnet/opus/haiku, no "use a stronger model" steer). The timeout hint is framed around *command latency*, not a model — e.g. "raise if your `ai_command` runs slowly." The pinned default *value* still carries `--model sonnet` (that's the decided default, not a comment).
- **F9 resolved**: no concrete, model-tied timeout number in the scaffold; the mitigation hint is generic and model-free.
- **README (F6) in scope**: document the new keys, the `verb → shared → default` resolution order, and the new shipped default value (a factual statement, not a recommendation). No breaking-change callout needed — mint has no users yet (see *Pin A Model In The Shipped Default*).

Confidence: high. Exact comment wording is a planning/impl detail.

## Notes for Spec/Planning

Factual completeness items carried over from the final review — no open decisions, recorded so spec/planning don't rediscover them:

- **Transport wiring sites (3).** The resolved per-verb command *and* timeout must be threaded where today only `ai.Config{AICommand: cfg.AICommand}` is constructed (Timeout left zero): `internal/engine/release.go`, `internal/commit/run.go`, and `internal/engine/regenerate_fresh.go`. `regenerate_fresh.go` is a *distinct* construction site that must deliberately resolve through `[release]` (per "regenerate rides on `[release]`"), not its own table — an easy miss.
- **Test-pin migration.** Changing the shipped default and removing the transport's `defaultAICommand` will break every test that asserts the exact default command/argv (`claude -p` with no `--model`), plus the initgen "full template loads cleanly" test. Project test idioms assert exact argv / rendered lines, so these are known, bounded edits — enumerate in planning.
- **Cross-spec reconciliation sequencing.** Reversing the commit spec's "Deliberately NOT added for commit" is not just a spec-doc edit: the as-built `Commit` struct doc comment (`internal/config/config.go`) encodes the old contract, and CLAUDE.md requires comments stay true to as-built in the same change. Spec must decide whether the commit-spec revision lands in *this* work unit or is handed to a separate commit-spec pass, and what blocks on what.

## Summary

### Key Insights

1. The per-verb question reduced to a single fork (releases on Sonnet vs Opus) once Haiku was ruled out and commit→Sonnet settled.
2. Experimentation is already possible via the single shared `ai_command` — so "I want to try models" is not what justifies per-verb config; wanting *different* models pinned *simultaneously* is.
3. A raw per-verb command string is *more* general than a driver — it argues for per-verb override and against the driver simultaneously.
4. Per-verb model freedom couples to the fatal 60s timeout — model and timeout overrides must travel together.

### Open Threads

- **Deferred to spec/planning** (deliberate hand-offs, not unresolved decisions): the new keys' exact names, the `timeout` TOML representation/units (int seconds vs string duration), and the exact model-agnostic comment wording. See *Notes for Spec/Planning* for the mechanical carry-overs (the three wiring sites, test-pin migration, cross-spec sequencing).
- **Interactive `mint init` setup** (prompting for model/AI per verb during scaffolding) — logged as a separate idea for later triage (init UX, broader than models, pulls in interactive-prompt machinery + the fail-loud/non-TTY invariant). The in-scope counterpart — init's static template surfacing the new keys — stays here as the `Init Scaffolds The New Config Keys` subtopic.
- **Env-var override layer** (Laravel `.env`-style third layer) — explicitly out of scope; addable later as a third layer if a need appears.

### Current State

- **Decided (all 8 subtopics)**: shared default = Sonnet, alias form; per-verb `ai_command` override (raw command string); per-verb timeout override (coupling is the operator's responsibility); keep top-level shared `ai_command`/timeout + per-verb overrides (config shape); `regenerate` rides on `[release]`; driver dropped; single source of truth = `internal/config` owns defaults, project file overrides, layered accessors resolve per-verb, transport carries no defaults; init scaffolds the new keys with model-agnostic comments + README documents keys/resolution; per-key resolution value semantics (ai_command empty→fall-through; timeout zero=no-limit, invalid→fall-through). Shipped-default change is **not** a breaking change in practice (brand-new project, no users yet) — no release-note/runtime signal; only internal test pins need updating.
- **Routed out**: interactive `mint init` setup → logged as a separate idea (the proper home for surfacing the model choice to operators). Env-var override layer (Laravel `.env`-style third layer) explicitly out of scope — two layers only (compiled defaults ← project file); addable later if wanted.

## Triage

(none)
