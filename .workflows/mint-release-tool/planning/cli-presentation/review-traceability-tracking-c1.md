---
status: in-progress
created: 2026-06-09
cycle: 1
phase: Traceability Review
topic: CLI Presentation
---

# Review Tracking: CLI Presentation - Traceability

## Summary

Bidirectional traceability analysis of the CLI Presentation plan (4 phases, 29 tasks) against `specification/cli-presentation/specification.md`.

**Direction 1 (Spec → Plan, completeness):** Every spec section maps to plan coverage with implementer-grade detail:

- Scope & Output Modes (two modes; three orthogonal axes) → Phase 1 (modes + stream split), Phase 3 (gating axis).
- Render-Mode Detection (precedence, `isatty`, no env sniffing incl. `NO_COLOR`, `--plain`-only, no `--pretty`, no colour flag, colour-incapable handled free via lipgloss) → Tasks 1-3, 1-5.
- Output Streams (narration→stdout, errors/warnings→stderr, one-line summary duplicated, captured body not, exit-code is engine's) → Tasks 1-6, 2-6, 2-7, 3-6.
- Presenter Seam (interface, event-payload principle, every illustrative method, the testability/engine-never-touches-colour/applies-to-every-verb/spinner-pretty-only decisions) → Tasks 1-1, 1-2, 2-1…2-8, 3-1, 4-5.
- Gating & `-y` Orthogonality (gate inventory, forbidden-combination, line-read input model, engine-owned re-entry loop, pretty-under-`-y`, `Prompt` carries choice set, source/target prompts) → Tasks 3-1…3-8.
- Pretty Layer (brand lines, status glyphs, stage lines, notes-no-box, vertical review menu, width robustness, `-y` alignment, worked examples incl. failure+unwind+warn) → Tasks 1-5, 2-3, 2-5, 2-8, 3-4, 4-4, 4-5, 4-7.
- Plain Layer (key:value contract, blocking-only start line, notes delimiters, byte-identical body w/ emoji headers, `-y` echo, per-event table, worked examples) → Tasks 1-4, 2-2, 2-4, 2-5, 3-5, 4-1…4-4.
- Spinner Lifecycle → Tasks 4-5, 4-6.
- Library Selection (lipgloss for pretty, standalone spinner, NOT Bubble Tea/no alt-screen, plain pulls in no UI lib) → Tasks 1-4, 1-5, 4-5, 3-8.
- Cross-Verb Rendering (`init`, `regenerate`, `version` payload exception, verb-shaped end-of-run, failure suppression) → Tasks 4-1, 4-2, 4-3, 4-4.
- Dependencies (none required; reconciliations owed *by* engine spec — review-gate keys, `--plain` CLI-surface registration; exit-code ownership) → correctly treated as out-of-scope reverse-direction items; `plainFlag` consumed as an injected input in Task 1-3 (flag registration is engine-spec scope per the spec's own Dependencies note).

**Direction 2 (Plan → Spec, fidelity / anti-hallucination):** Every task's Problem, Solution, acceptance criteria, tests, and edge cases trace to specific spec text. Spot checks:

- `NO_COLOR` in the no-sniffing list (1-3) ← spec line 42 names it as out-of-scope sniffing.
- Package/type choices (`internal/presenter`, `golang.org/x/term`, `briandowns/spinner`, `lipgloss`) framed as recommendations ← spec "exact package is an implementation detail."
- `ruleCap = 50` (4-7) ← spec "`~50`."
- EOF-returns-error (3-3), one-spinner-at-a-time spy assertions (4-5), suspend/resume hooks (4-6), `Gate.Subject` for `-y` echo (3-5) — all faithful derivations of stated spec requirements (never silently accept; one spinner at a time; engine-driven `$EDITOR` hand-off; auto-accept is a rendered event), not invented behaviours.

No hallucinated content found. No missing scoped content found.

## Findings

### 1. Brand-leaf provenance (`commit_prefix` tie) under-specified in the pretty brand-line tasks

**Type**: Incomplete coverage
**Spec Reference**: "The Pretty Layer" — Brand lines: "The leaf ties to the engine's `commit_prefix` brand." (spec line 125); "The `Presenter` Seam" — Event payload principle ("the engine supplies, in each event's payload, every datum the renderings consume — the presenter never re-derives engine knowledge").
**Plan Reference**: Task cli-presentation-1-5 (renders the `🌿` brand lines); also surfaces in Tasks 4-3 (`🌿 mint v{value}`) and 4-4 (`🌿 released …` footer).
**Change Type**: add-to-task

**Details**:
The spec states the brand leaf "ties to the engine's `commit_prefix` brand," and the event-payload principle forbids the presenter from re-deriving engine knowledge. Task 1-5 hardcodes the literal `🌿` leaf in the brand-line rendering and the other brand-bearing tasks (4-3, 4-4) inherit that literal, with no task deciding whether the leaf glyph is a constant or an engine-supplied payload datum tied to `commit_prefix`. This is the one spec datum whose plan provenance is left implicit: every worked example shows the literal `🌿`, so rendering it as a constant is a defensible reading, but the spec's explicit `commit_prefix` tie plus the payload principle mean an implementer could reasonably expect the leaf to arrive in the `RunInfo`/`RunResult` payload (so a project that customises `commit_prefix` gets a matching leaf) rather than being hardcoded. Surfacing the decision in Task 1-5 removes the ambiguity without expanding scope.

This is the only traceability gap found; it is low-severity (the literal-leaf reading is faithful to every worked example) and is raised so the user can confirm the intended provenance.

**Current** (Task cli-presentation-1-5, `**Context**` block — relevant excerpt):
> Brand lines — Top: `🌿 mint · {project}  ›  releasing v{X}`; Bottom: `🌿 released {project} v{X} · {url}`. Status glyphs: `✓` success (green) · `✗` failure (red) · `⚠` warn (amber) · `↩` auto-unwind. Stage lines: two-space indent, glyph, stage name padded to a column, terse detail.

**Proposed** (Task cli-presentation-1-5, `**Context**` block — append after the brand-lines line; add a matching one-line note to `**Do**` for `RunStarted`):

Append to `**Context**`:
> Brand-leaf provenance: the spec notes "the leaf ties to the engine's `commit_prefix` brand." Per the event-payload principle the engine supplies every datum the rendering consumes, so the leaf glyph should arrive in the start-of-run / end-of-run payload (e.g. a `Leaf`/`Brand` field on `RunInfo`/`RunResult`, defaulting to `🌿`) rather than being hardcoded in the presenter — the presenter renders the engine-supplied leaf so a customised `commit_prefix` brand stays consistent. Every worked example uses the default `🌿`; if the user prefers a fixed constant leaf, the field can be omitted and the literal `🌿` rendered — this finding exists to confirm which.

Append to `**Do**` (under the `RunStarted` bullet):
- The brand leaf is engine-supplied (carried on the start-of-run/end-of-run payload, defaulting to `🌿`) rather than hardcoded, honouring the spec's "leaf ties to `commit_prefix`" note and the event-payload principle; render the supplied leaf, do not re-derive it.

Add to `**Acceptance Criteria**`:
- [ ] The brand leaf is rendered from the engine-supplied payload datum (defaulting to `🌿`), not re-derived/hardcoded in the presenter.

**Resolution**: Pending
**Notes**: Low-severity / confirmatory. If the user prefers the leaf as a fixed presenter constant, this finding can be resolved as "no change" — the literal-leaf reading is consistent with every worked example in the spec.

---
