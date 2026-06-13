# Discussion: AI Model Selection

## Context

Mint's default `ai_command` is `claude -p`, which inherits whatever default model the operator's Claude CLI is configured for — likely an overpowered, slower, more expensive model than mint's two AI tasks (release notes, commit messages) actually need. The immediate want is to **pin a model in the shipped default** so behaviour is predictable regardless of the operator's CLI settings — probably via the alias form (`claude -p --model sonnet`) rather than a full model ID, since baked-in full IDs go stale as new model versions ship.

That small change opens a larger design question worth talking through: **should the two verbs use different models at all?** Release notes are salience-heavy (judge what matters across a whole change map, grouping, audience-facing tone → Sonnet/Opus). Commit messages are frequent and latency-sensitive (summarise one bounded staged diff → Haiku may suffice and is noticeably faster). The commit-command spec currently mandates a single shared top-level `ai_command` with **explicitly no per-verb override** ("promote to a `[commit]` key only if a real need appears"). This idea may be that real need surfacing — so the config shape is on the table.

A related "ideal world" framing was raised in discovery: rather than configuring a raw command string, pick *which AI* you're using and set the model as config, with mint knowing how to invoke each AI (a driver-based pattern). The user explicitly flagged this as likely YAGNI today (no desire for multiple AIs) — it stays in the option space to weigh and probably defer.

There's also a code-health angle: the default command string is duplicated across three sites — `internal/config/config.go:75`, `internal/ai/transport.go:45`, `internal/initgen/initgen.go:39` — plus test pins and both specs. Changing the default touches all of them in step; worth deciding whether the default should have a single source of truth.

One coupling to keep in view: the transport's per-attempt timeout is 60s and timeouts are **fatal (not retried)**, so slower models (Opus) interact directly with that deadline — model choice and timeout policy are coupled.

### References

- Seed: `seeds/2026-06-11-ai-model-selection.md` (inbox:idea)
- Discovery: `discovery/session-001.md`
- Touch sites: `internal/config/config.go:75`, `internal/ai/transport.go:45`, `internal/initgen/initgen.go:39`

## Discussion Map

A living index of subtopics tracked during the discussion. Grows as the conversation branches, converges as decisions land.

### States

- **pending** (`○`) — identified but not yet explored
- **exploring** (`◐`) — actively being discussed
- **converging** (`→`) — narrowing toward a decision
- **decided** (`✓`) — decision reached with rationale documented

### Map

  Discussion Map — AI Model Selection (6 subtopics, all pending)

  ┌─ ○ Pin a model in the shipped default [pending]
  │  └─ ○ Alias form vs full model ID [pending]
  ├─ ○ Per-verb model differentiation [pending]
  │  └─ ○ Config shape: shared vs [commit] override vs per-verb [pending]
  ├─ ○ Driver-based AI config (pick-AI-then-model) [pending]
  ├─ ○ Single source of truth for the default command [pending]
  └─ ○ Timeout × model-choice coupling [pending]

---

*Subtopics are documented below as they reach `decided` or accumulate enough exploration to capture.*

---

## Summary

### Key Insights

*(to be filled as the discussion develops)*

### Open Threads

*(to be filled)*

### Current State

- Nothing decided yet — discussion just opened.

## Triage

(none)
