---
status: complete
created: 2026-06-13
cycle: 2
phase: Traceability Review
topic: AI Model Selection
---

# Review Tracking: AI Model Selection - Traceability

## Summary

Fresh bidirectional traceability analysis (cycle 2) of the AI Model Selection
plan against its specification, performed after cycle 1's single integrity fix
(Task 1-7's `TimeoutFor` return type pinned to `*time.Duration`). The full
specification, the planning file, all 17 leaf tasks (read from both the tick
store and the mirrored `phase-{1,2,3}-tasks.md` files), and the as-built
transport/wiring sources were re-read from scratch.

- **Direction 1 (Specification → Plan, completeness)**: Every specification
  element — every decision, requirement, edge case, constraint, schema field,
  integration point, and validation rule — is represented in the plan with
  implementer-level depth. No missing content.
- **Direction 2 (Plan → Specification, fidelity)**: Every task's Problem,
  Solution, Do steps, Acceptance Criteria, Tests, and Edge Cases trace to a
  named specification section. No hallucinated requirements, edge cases,
  acceptance criteria, or technical approaches.

**Cycle-1 fix confirmed**: Task 1-7 (`tick-f32dd3` / `ai-model-selection-1-7`)
fixes `func (c Config) TimeoutFor(verb Verb) *time.Duration` in its Do step,
Acceptance Criteria, and Tests. This is consistent with Task 2-2 (which changes
`ai.Config.Timeout` to `*time.Duration`) and with Tasks 2-3/2-4/2-5 (which assign
the accessor's return directly with no conversion). The integrity chain holds.

**Result: CLEAN — 0 findings.**

## Coverage Map (Specification → Plan)

| Specification section | Plan coverage |
|---|---|
| Overview (three goals: pin model, per-verb `ai_command`+parallel `timeout`, config as single source of truth) | 1-1; 1-3/1-4/1-5/1-6/1-7; 1-1/1-2/1-4/1-7/2-1/2-2/3-1 |
| Scope boundaries / non-goals (no driver, no env-var layer, no interactive init, no coupling protection) | Negative constraints honored — nothing builds them; 3-2 explicitly excludes them from README; coupling-protection non-goal honored by 1-8 (independence pinned correct) + 3-2 (unenforced pattern documented) |
| Pinned default model (`claude -p --model sonnet`, alias-not-full-ID, Sonnet default / Opus opt-in / Haiku ruled out, not-a-breaking-change, no release-note/runtime callout) | 1-1 (const + alias rationale + Sonnet/Opus/not-breaking context); propagated 2-1, 2-6, 3-1, 3-2 |
| Config schema: per-verb `ai_command` override (both-levels-with-fallback, resolution order, full-command-string mechanism, two-tables-only, regenerate-not-a-verb, `[commit]` mirrors `[release]`, strict-decoding requirement) | 1-2 (enum / two-tables / no-regenerate), 1-3 (schema + strict decode + typeErrorMessages), 1-4 (accessor / resolution order) |
| Config schema: `timeout` key (NET-NEW full new-key treatment, deferred TOML representation, shipped 60s seeded in config, int-vs-string detection-site detail, value-semantics preservation) | 1-5 (top-level + int-seconds representation decision + seed + typeErrorMessages), 1-6 (per-verb), 1-7 (accessor value semantics) |
| Resolution value semantics — `ai_command` (blank/whitespace/invalid/missing drops, never-empty floor, transport old empty→re-default path unreachable+removed, accessor-owns-blank-skip at every layer, fold `resolveAICommand`) | 1-4 (accessor); transport removal 2-1 |
| Resolution value semantics — `timeout` (zero-honored / stops fall-through, missing/negative drops, positive as-is, conditional `WithTimeout` / skip on 0, config→`ai.Config` absent-vs-explicit-zero boundary invariant, no-negative-collapse) | 1-7 (accessor + `*time.Duration` return), 2-2 (transport conditional deadline + boundary type + negative-not-collapsed) |
| Timeout × model-choice coupling — operator's responsibility (no auto-bump, no warning, no paired-defaults requirement, 60s shared default ships) | 1-8 (per-key independence pinned as correct/unprotected), 3-2 (override-both documented as supported-but-unenforced) |
| Single source of truth (one `defaults()` constructor, layered typed accessors, typed closed verb enum, transport carries no defaults, `initgen` pulls from config, no reflection/global service-locator, `ai`↔`config` decoupling, de-dup target) | 1-1 (export const), 1-2 (enum), 1-4/1-7 (accessors), 2-1/2-2 (transport no defaults), 3-1 (initgen sources constant) |
| Init template scaffolding (top-level bumped + sourced-from-constant, shared `timeout` at 60s, commented per-verb overrides under both tables, model-agnostic latency-framed comments, no model-tied timeout number) | 3-1 |
| README documentation (both-level keys, resolution order, default-as-fact, `timeout = 0` no-limit trade-off, override-both unenforced pattern, no out-of-scope mechanisms, no breaking-change callout) | 3-2 |
| Cross-spec reconciliation (in-repo `Commit` field + struct doc comment in-scope same-change; external commit-command spec document deferred to separate pass) | 3-3 (in-repo comment reconciliation), out-of-scope external-doc boundary respected; 1-3 defers the full reconciliation to 3-3 while touching only what compilation forces |
| Acceptance criteria — resolution behaviors (per-key independence, `ai_command` never empties, `timeout = 0` honored, negative/invalid drops, regenerate routes through `[release]`, pinned default zero-config for both verbs+60s) | 1-8 (independence), 1-4 (never-empties + zero-config), 1-7 (timeout=0 / negative drops + zero-config), 2-5 (regenerate routes `[release]`), zero-config also in 1-1/2-3/2-4/2-5 |
| Migration & mechanical carry-overs (3 transport wiring sites incl. easy-miss regenerate, test-pin migration, transport doc-comment migration same-change) | 2-3 / 2-4 / 2-5 (the three sites), 2-6 (test-pin sweep), 2-1 (command-side comments `Config.AICommand`/`NewTransport`/`parseCommand`) + 2-2 (timeout-side comments `Config.Timeout`/`NewTransport`/`Generate`/`attempt`) |

### Transport doc-comment migration (spec's most granular enumeration) → plan

| Spec-enumerated comment | Plan task |
|---|---|
| `Config.AICommand` — "(default `claude -p` when empty)" | 2-1 |
| `Config.Timeout` — "A zero or negative Timeout falls back to the production default." | 2-2 |
| `NewTransport` — "An empty AICommand resolves to `claude -p` … non-positive Timeout resolves to the ~60s production default …" | 2-1 (command clause) + 2-2 (timeout clause) |
| `Generate` / `attempt` — "Each attempt gets its own deadline via `context.WithTimeout(ctx, t.timeout)`" (→ conditional) | 2-2 |

All four covered, with the command/timeout halves correctly split across 2-1/2-2.

## Direction 2 notes (anti-hallucination)

Every place the plan adds specificity beyond the spec's literal text was checked;
all trace to spec-sanctioned planning latitude:

- **Int-seconds `timeout` representation (1-5)** — the spec explicitly defers the
  representation to planning ("Deferred to planning: the key's exact TOML
  representation/units (int seconds vs string duration)") and enumerates the
  int-seconds value semantics ("a non-integer TOML value is a strict decode (type)
  error at `Load`; a negative integer is a value-invalid drop-through; absent vs
  zero is a nil pointer vs `0`"). Choosing int-seconds, with the idiom-consistency
  rationale against `max_diff_lines`, is a legitimate planning decision carried
  coherently into 1-6 and 1-7 — not invented scope.
- **`*time.Duration` accessor return / verb enum representation (1-2, 1-7, 2-2)** —
  the spec states "Exact type and constant names are a planning/impl detail" for
  the verb enum and "Planning picks the mechanism (e.g. give the boundary field a
  type that distinguishes nil from explicit-`0`, such as `*time.Duration` / a small
  wrapper)" for the timeout boundary. The chosen mechanisms sit squarely within the
  stated latitude. (Cycle-1 fix confirmed consistent across 1-7/2-2/2-3/2-4/2-5.)
- **`120` per-verb timeout scaffold example (3-1)** — flagged in-task as
  "illustrative, not a recommendation"; the spec says "Exact comment wording is a
  planning/impl detail." Traces correctly.

As-built references embedded in the tasks were spot-verified against the current
tree and are accurate: `defaultAICommand = "claude -p"` and
`defaultTimeout = 60 * time.Second` exist in `internal/ai/transport.go`; all three
wiring sites (`internal/engine/release.go`, `internal/commit/run.go`,
`internal/engine/regenerate_fresh.go`) construct `ai.Config{AICommand: cfg.AICommand}`
with `Timeout` left zero, exactly as the plan describes. The `Commit`
"Deliberately NOT added for commit" comment and the README config-table layout
referenced by 3-2/3-3 are consistent with the spec's as-built notes.

No task contains a Problem, Solution, acceptance criterion, test, or edge case
that lacks a corresponding specification section. No requirement, behavior, or
edge case appears in the plan that the specification never identified.

## Findings

None.

## Resolution

No findings to resolve. The plan remains a faithful, complete bidirectional
translation of the specification at cycle 2; the cycle-1 integrity fix is intact
and consistent throughout the dependent tasks.
