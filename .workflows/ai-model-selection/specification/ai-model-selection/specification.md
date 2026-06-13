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

---

## Working Notes

[Optional - capture in-progress discussion if needed]
