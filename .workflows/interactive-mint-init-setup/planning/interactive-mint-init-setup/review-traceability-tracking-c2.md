---
status: complete
created: 2026-06-16
cycle: 2
phase: Traceability Review
topic: Interactive Mint Init Setup
---

# Review Tracking: Interactive Mint Init Setup - Traceability

## Result

**CLEAN** — no findings. The plan is a faithful, complete two-directional translation of the specification.

This is review cycle 2. Cycle 1 was clean for traceability; since then the integrity review added two cross-phase dependency edges (2-2 blocked_by 1-2; 4-3 blocked_by 1-1). Those edges are structural sequencing, not content, and do not alter the spec↔plan content mapping. Re-running the full two-directional trace against the current 14 authored tasks confirms the plan remains a faithful translation.

## Direction 1: Specification → Plan (completeness)

Every specification element has plan coverage with sufficient implementer-level depth:

| Specification element | Plan coverage |
|---|---|
| `mint setup` subcommand — pure stdout string emitter, no IO beyond stdout | Phase 2 — tasks 2-1 (emitter), 2-3 (runner) |
| What it emits — (1) procedure, (2) etiquette, (3) config reference | 2-1 (procedure + etiquette prose) + 2-2 (config reference render) |
| Stable section markers + structural test (5 marked sections) | 2-1 (pipeline, etiquette, minimalism, existing-config, config-reference markers + grep-the-markers test) |
| Runs unconditionally — no git/cwd guard | 2-3 (no `git rev-parse` / repo-root resolution before emitting) |
| Help-surface wiring (rootUsage line, setupUsage, classifyCommand/run dispatch, coverage test) | 2-3 (dispatch route + runSetup) + 2-4 (rootUsage line, setupUsage const, extended coverage test) |
| `mint help` stays frozen, gains only the setup line, carries no config reference | 2-4 (frozen-text + no-config-reference guard test) |
| Install handling (assumes installed; agent asks to install; no auto-install) | 4-2 (entry point assumes installed, must not imply auto-install) — guide is binary-emitted so the install-ask is correctly an entry-point/README concern |
| Generated config: strip to minimal (empty body + short header) | Phase 3 — task 3-1 |
| Header dual pointers (GitHub docs + `mint setup`) as cold-arrival recovery net | 3-1 (both pointers pinned by test) |
| initgen scope: only `MintTOML()` changes; `ReleaseShim()` + its tests untouched | 3-1 |
| initgen test impact — remove the commented-template assertions; add minimal-shape tests | 3-1 (twelve named removals + new minimal-shape/header-pointer tests) |
| Scaffold-value drift-pin moves to the SoT (no default left unpinned) | 1-5 (adds the subsuming SoT pin) + 3-2 (removes the stranded initgen pins) |
| Config-metadata SoT (one row per key · level · default · description) | 1-1 |
| `default` column representation convention (blank / auto / `[]` / shared / hooks-blank) | 1-2 |
| Drift test — total bijection over leaf keys, derived mechanically from the schema | 1-3 (mechanical derivation via reflection) + 1-4 (bijection test) |
| Bijection contract: dual-level ai_command/timeout per level; recurse-don't-count containers | 1-1, 1-3, 1-4 (all three carry the per-(level,key) + recurse-don't-count rule) |
| Render targets and layering | 2-2 (`mint setup`), 3-1 (`mint init`), 2-4 (`mint help` — no reference), 4-1 (README — human reference) |
| Emitted guide — setup procedure (6 ordered steps incl. cwd-confirm, read-config-reference-early) | 2-1 |
| mint's pipeline / stage model (required content section) | 2-1 (full pipeline `preflight → notes → pre_tag → tag + push (PONR) → publish → post_release`) |
| Hook detection & mapping (best-fit, flag, never silently skip, pre_tag-as-array) | 2-1 |
| Cross-cutting principle — agent as collaborator | 2-1 |
| AI model per verb — config representation (same → shared; different → per-verb) | 2-1 |
| diff_exclude scope (release-notes noise, surfaced interactively) | 2-1 |
| Emitted guide — AI etiquette (ask interactively, confirm, never remove without permission) | 2-1 |
| Emitted guide — minimalism (only set what varies; read defaults from the reference) | 2-1 |
| Emitted guide — existing .mint.toml: diff, discuss, upgrade | 2-1 |
| Release shim role mention in the guide | 2-1 |
| README — entry point (tiny prompt routing to `mint setup`) | 4-2 |
| README — "any AI" framing (light, no machinery; Opus-level steer) | 4-2 |
| README — config reference verification + reconcile Configuration/Commands (a/b/c) | 4-1 (intro rewrite, embedded-block replace/drop, Commands init line) |
| README tripwire (optional — every schema key name appears) | 4-3 |
| Definition of done — drift test, structural test, updated initgen tests, help-contract coverage, README tripwire | Distributed across 1-4, 2-1, 3-1/3-2, 2-4, 4-3 |
| Acceptance — prose quality (one-time manual run, four representative repos) | "Manual Acceptance (not a phase)" section, naming the same four repos |

Coverage depth is sufficient throughout — each task carries the spec's essence (decisions, constraints, edge cases) inline, so an implementer would not need to return to the specification.

## Direction 2: Plan → Specification (fidelity / anti-hallucination)

Every task's content traces back to a specific specification section. Items examined that add specificity beyond the spec's literal prose, with their traces:

- **"25 rows" / "14 [release] keys" (1-1)** — a mechanical consequence of the spec's "bijection total over leaf keys" applied to the actual `config` schema (4 shared + 14 release + 3 hooks + 4 commit). The plan explicitly forbids hard-coding the count in production and defers to the drift test as the real guard. Not an invented requirement — derived from the verified schema. Traces to "Drift test → the bijection contract".
- **Concrete default values in 1-2/1-5 (`v`, `🌿`, `true`, `abort`, `50000`, `claude -p --model sonnet`, `60`)** — verified against `internal/config/config.go`. The spec mandates the SoT carry the "real compiled default... not an illustrative example", so the real values belong in the plan. Traces to "Config-metadata source of truth (SoT) → default column".
- **Emission-surface decision (presenter vs cmd-layer) in 2-1/2-3** — framed as an implementer flag against CLAUDE.md seam 3, not a fabricated spec requirement. The spec leaves "how the string reaches stdout" open ("pure emitter… no IO beyond stdout"); the plan surfaces the choice rather than inventing a mandate. Traces to "The mint setup subcommand" (pure emitter) + project seam context.
- **Package placements (`internal/setupguide`, SoT inside `internal/config`)** — the spec explicitly designates package layout a planning detail. Traces.
- **Optional tripwire (4-3) marked OPTIONAL** — matches the spec's explicit "optional" framing. Traces.
- **Pipeline timing claims in 2-1 ("post_release runs after publish", PONR at tag+push)** — taken verbatim from the spec's "mint's pipeline / stage model" decided content; cross-checked against the engine. Traces.

No content was found that lacks a specification anchor. No invented edge cases, no made-up acceptance criteria, no technical approaches absent from the spec.

## Findings

None.

---

_All 14 authored tasks (phases 1–4) and the Manual Acceptance section were read in full and traced in both directions against the specification. No missing spec content; no hallucinated plan content._
