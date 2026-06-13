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

  Discussion Map — AI Model Selection (8 subtopics — 5 decided · 1 converging · 2 pending)

  ┌─ → Pin A Model In The Shipped Default [converging]
  │  └─ ✓ Alias Form Vs Full Model ID [decided]
  ├─ ✓ Per-Verb Model Differentiation [decided]
  │  └─ ✓ Config Shape: Top-Level Shared + Per-Verb Override [decided]
  ├─ ✓ Timeout × Model-Choice Coupling [decided]
  ├─ ✓ Driver-Based AI Config — Dropped [decided]
  ├─ ○ Single Source Of Truth For The Default Command [pending]
  └─ ○ Init Scaffolds The New Config Keys [pending]

---

*Subtopics are documented below as they reach `decided` or accumulate enough exploration to capture.*

---

## Pin A Model In The Shipped Default

### Context

`claude -p` inherits whatever model the operator's Claude CLI defaults to — so output quality, cost, and latency silently depend on an external, mutable setting mint doesn't control. Pinning a model in the shipped default removes that dependence.

### Decision (converging)

Pin via the **alias form** (`--model sonnet`), **not a full model ID** — full IDs baked into the binary go stale every model release and would force a rebuild just to track versions; the alias tracks the current version automatically. *(Alias-vs-ID: decided.)*

Shared default model leans **Sonnet** — strong enough for the salience-heavy notes task and comfortably inside the 60s deadline; Opus reserved for explicit per-verb opt-in. *(Specific shared model: converging, confirm pending.)*

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

## Timeout × Model-Choice Coupling

### Context

The transport's per-attempt timeout is 60s and a timeout is **fatal — not retried** (the single retry covers empty/error/refusal *content* only). Slower models (Opus) on big diffs hit this deadline directly: the release verb is both where Opus pays off most (salience synthesis over a whole change map) and where it's most likely to time out (large input), and that failure aborts the release.

### Decision

**Add a per-verb timeout override** (confidence: high, user-confirmed), mirroring the `ai_command` shape: a top-level shared default (the current 60s value) + optional `[release]`/`[commit]` override. Rationale: per-verb model freedom and per-verb timeout freedom must travel together — otherwise we've made it trivial to pin a model that reliably blows the fatal deadline. Key naming and per-attempt semantics are an implementation detail left to spec/planning.

## Driver-Based AI Config — Dropped

### Context

The seed's "ideal world": configure *which AI* + a model alias, with mint knowing how to invoke each AI (driver pattern). Flagged as likely YAGNI in discovery.

### Decision

**Dropped** (confidence: high) — not merely deferred-on-YAGNI, but *superseded*. The raw per-verb command string already delivers the multi-AI / multi-command generality the driver was meant for, with less machinery and no provider registry to maintain. The driver's only residual value is ergonomics (typing `haiku` vs the full command), which doesn't justify the cost today. Revisit only as sugar over the command string if a future user juggles several AIs frequently.

## Summary

### Key Insights

1. The per-verb question reduced to a single fork (releases on Sonnet vs Opus) once Haiku was ruled out and commit→Sonnet settled.
2. Experimentation is already possible via the single shared `ai_command` — so "I want to try models" is not what justifies per-verb config; wanting *different* models pinned *simultaneously* is.
3. A raw per-verb command string is *more* general than a driver — it argues for per-verb override and against the driver simultaneously.
4. Per-verb model freedom couples to the fatal 60s timeout — model and timeout overrides must travel together.

### Open Threads

- Shared default model (Sonnet) and top-level-vs-verb-only config shape — converging, confirm pending.
- **Interactive `mint init` setup** (prompting for model/AI per verb during scaffolding) — logged as a separate idea for later triage (init UX, broader than models, pulls in interactive-prompt machinery + the fail-loud/non-TTY invariant). The in-scope counterpart — init's static template surfacing the new keys — stays here as the `Init Scaffolds The New Config Keys` subtopic.

### Current State

- **Decided**: per-verb `ai_command` override (raw command string); per-verb timeout override; **keep top-level shared `ai_command`/timeout + per-verb overrides** (config shape); driver dropped; alias form over full model ID.
- **Converging**: shared default model = Sonnet (shape confirmed; specific model not yet).
- **Pending**: single source of truth for the default command; init scaffolds the new keys.
- **Routed out**: interactive `mint init` setup → logged as a separate idea.

## Triage

(none)
