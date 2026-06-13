# Specification: AI Model Selection

## Specification

### Overview

Mint invokes an external AI for two tasks — release notes (`mint release`, and its `regenerate` path) and commit messages (`mint commit`) — through a single configurable command. Today that command is `claude -p`, which inherits whatever default model the operator's Claude CLI is set to: an external, mutable setting mint doesn't control, so output quality, cost, and latency vary silently.

This feature does three things:

1. **Pins a model in the shipped default** so behaviour is predictable regardless of the operator's CLI configuration.
2. **Lets each verb run a different AI command** by promoting `ai_command` from a shared-only key to one that also accepts a per-verb override, and introduces a parallel `timeout` key with the same shape (because per-verb model freedom and per-verb timeout freedom must travel together).
3. **Establishes `internal/config` as the single source of truth for config defaults**, removing the default-command duplication scattered across the codebase and centralizing per-verb resolution behind typed accessors.

### Scope boundaries (non-goals)

- **No driver / provider-registry pattern.** Configuring "which AI" plus a model alias, with mint knowing how to invoke each AI, is explicitly dropped (not deferred) — a raw per-verb command string already delivers the multi-AI/multi-command generality with less machinery.
- **No environment-variable override layer.** A `MINT_AI_COMMAND`-style third layer is out of scope; the override layer is the project file only. Two layers: compiled defaults ← project file.
- **No interactive `mint init` prompting.** Surfacing the model choice to operators during scaffolding is a separate, deferred idea. Only init's *static* commented template surfacing the new keys is in scope here.
- **No coupling protection.** Mint does not auto-bump the timeout, warn, or require paired defaults when a verb overrides the command to a slower model — that is the operator's responsibility (detailed in its own section).

### Pinned default model

The shipped default command becomes `claude -p --model sonnet` (today: `claude -p`).

- **Alias form, not a full model ID.** The default pins `--model sonnet`, not a full versioned model ID. Full IDs baked into the binary go stale every model release and would force a rebuild just to track versions; the alias tracks the current version automatically.
- **Default model is Sonnet.** Sonnet is strong enough for the salience-heavy notes task and comfortably inside the per-attempt deadline. Opus is reserved for explicit per-verb opt-in — never the shipped default.
- **Not a breaking change in practice.** Moving the shipped default from bare `claude -p` to `claude -p --model sonnet` would silently switch the model for zero-config operators, but mint is a brand-new project with no users yet — there is nothing to break. **No release-note callout and no runtime signal are required.** The only real migration cost is internal: mint's own test pins that assert the old `claude -p` default (enumerated in the Migration section).

### Config schema: per-verb `ai_command` override

`ai_command` is promoted from a shared-only engine key to a key that lives at **both** levels with fallback:

- **Top-level shared `ai_command`** — the baseline (the shipped-default home). Repoints every verb in one line.
- **Optional per-verb override** — `[release].ai_command` and `[commit].ai_command`.

**Resolution order:** `[verb].ai_command` → top-level shared `ai_command` → shipped default.

**Mechanism is a full command string per verb**, not a model knob or driver. A raw command string supports any AI / model / flags with zero per-AI machinery — the transport is already content-agnostic. This is what makes a verb able to run a *different AI entirely*, not just a different model.

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

---

## Working Notes

[Optional - capture in-progress discussion if needed]
