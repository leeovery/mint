---
status: in-progress
created: 2026-06-09
cycle: 3
phase: Traceability Review
topic: CLI Presentation
---

# Review Tracking: CLI Presentation - Traceability

## Summary

Cycle-3 bidirectional traceability analysis of the CLI Presentation plan (4 phases, 29 tasks) against `specification/cli-presentation/specification.md`, re-run after the cycle-1 and cycle-2 fixes were applied. The full specification, planning file, and all four `phase-{1,2,3,4}-tasks.md` detail files were re-read in full.

**Cycle-1/2 fixes re-verified as applied and consistent:**

- **Brand-leaf provenance (1-5 / 4-3 / 4-4)** — the leaf is an engine-supplied payload datum (`RunInfo`/`RunResult`/`Version` `Leaf`/`Brand`, defaulting to `🌿`) across all three brand-bearing tasks; no drift.
- **Version `Leaf` field (4-3)** — `type Version struct { Value string; Leaf string }`; plain ignores it, pretty renders it; consistent with 1-5.
- **Interim notes-rule width (2-5)** — fixed-width rule at the `~50` cap constant, `min(terminalWidth, cap)` narrowing deferred to 4-7; faithful to the spec's `min(terminalWidth, ~50)`.
- **Reuse-confirm `-y` echo subject (3-5)** — `ReuseConfirmGate()` sets `Subject: notes` → `notes: accepted (-y)`; faithful to the spec's "analogous auto-accept echo … same `Continue?` vocabulary."
- **Source/target enumerated choice (3-7)** — modelled as a declared `GateChoice` set so the shared line-read/"unrecognised re-prompts" core applies; conservative, not invented.
- **Start-of-run verb-shaped Action word** — applied to **1-1** (declares `RunInfo.Action`) and **1-5** (pretty renders `{action}`, AC forbids hardcoded `releasing`) and **4-2** (regenerate start-of-run uses `regenerating`). **However, the symmetric fix was not propagated to the plain presenter task (1-4)** — see Finding 1.

**Direction 1 (Spec → Plan, completeness):** Every specification section maps to plan coverage with implementer-grade detail; re-verified section by section (Scope & Output Modes; Render-Mode Detection & Output Streams; The `Presenter` Seam; Gating & `-y` Orthogonality; The Pretty Layer; The Plain Layer; Spinner Lifecycle; Library Selection; Cross-Verb Rendering; Dependencies). Negatives ("no `json`/`toon`", "no `--pretty`", "no colour flag/`--no-color`", "no env sniffing") are correctly honoured by absence. The three spec error/warning event types that route a one-line summary to stderr — `StageFailed` (1-6/2-7), `Warn` (2-6), and the forbidden-combination failure (3-6) — are each covered; there is no fourth error type in the spec. The `2>&1` combined-capture note is a consequence of the 1-6 stream wiring, not a separate buildable requirement. The future `commit` verb appears only inside spec quotations, never as a task — correctly out of scope.

**Direction 2 (Plan → Spec, fidelity / anti-hallucination):** Every task's Problem, Solution, Do, Acceptance Criteria, Tests, and Edge Cases trace to specific spec text. Implementation-detail choices not literally in the spec (package/type/dependency selections, `ruleCap = 50`, EOF-returns-error, the `SuspendSpinner`/`ResumeSpinner` hooks, `Gate.Subject`, enumerated source/target gates) are consistently framed as recommendations the spec licenses ("the exact package is an implementation detail"; "`~50`"; "never silently accept"; engine-driven `$EDITOR` hand-off; "auto-accept is a rendered event"). No hallucinated content found.

**Net result:** One finding — an internal inconsistency in the start-of-run Action-word fix: the plain presenter task (1-4) still hardcodes `releasing`, contradicting 1-1's acceptance criterion and the plan's own 4-2 requirement (and the spec's "applies to every verb"). This is the plain-side mirror of the gap that cycle 1/2 corrected on the pretty side (1-5).

## Findings

### 1. Plain presenter (1-4) hardcodes `releasing` in the start-of-run line, contradicting the engine-supplied Action word adopted in 1-1/1-5/4-2

**Type**: Incomplete coverage (internal inconsistency with a spec-grounded decision)
**Spec Reference**: "Cross-Verb Rendering" (`regenerate` — same vocabulary as `release`, "Applies to every verb"); "The `Presenter` Seam" (Event payload principle — the engine supplies every datum each rendering consumes); "The Plain Layer" (Per-event rendering — start of run → `mint: releasing {project} v{X}`, the release-shaped instance of the verb-shaped line)
**Plan Reference**: Phase 1, task cli-presentation-1-4 (Outcome, the `RunStarted` Do step, the Context per-event line); contradicts cli-presentation-1-1 acceptance criterion ("no presenter code hardcodes the literal `releasing`") and cli-presentation-4-2 (which requires the plain start-of-run line to read `mint: regenerating {project} v{X}`)
**Change Type**: update-task

**Details**:
The cycle-1/2 fix made the start-of-run line verb-shaped from an engine-supplied `RunInfo.Action` word: task 1-1 declares the field and asserts "no presenter code hardcodes the literal `releasing`", task 1-5 renders `{action}` in the pretty brand line (with an acceptance criterion forbidding a hardcoded `releasing`), and task 4-2 requires the plain start-of-run line to render `regenerating` for `regenerate`.

The plain presenter task 1-4, however, was not updated to match. Its `RunStarted` Do step (line 149) still reads `RunStarted → mint: releasing {project} v{X}`, its Outcome (line 144) names the literal `mint: releasing {project} v{X}` as the line it emits, and it carries no acceptance criterion requiring the action word to come from `RunInfo.Action`. As written, 1-4 would hardcode `releasing`, which (a) directly contradicts 1-1's acceptance criterion, (b) makes 4-2 unsatisfiable without re-opening 1-4, and (c) breaks the spec's "applies to every verb" by leaving the plain start-of-run line release-only. This is the exact plain-side mirror of the pretty-side gap that cycle 1/2 fixed in 1-5; it must be applied symmetrically. The spec's `mint: releasing …` is the *release instance* of the line (the worked examples are all `release`), not a literal the plain renderer should bake in.

The fix updates 1-4's `RunStarted` Do step, Outcome, Context, and Acceptance Criteria to render the engine-supplied `RunInfo.Action` (defaulting to the worked `releasing` for `release`, `regenerating` for `regenerate`), exactly mirroring 1-5. No new payload or scope is introduced — `RunInfo.Action` already exists (1-1).

**Current** (task cli-presentation-1-4 — the affected fragments):

> **Outcome**: Given an event stream, `PlainPresenter` emits the spec's plain lines (`mint: releasing {project} v{X}`, `{stage}: {detail}`/`{stage}: ok`, `done: {project} v{X} {url}`) with zero ANSI/glyph/animation bytes and no UI-library import.

> **Do**:
> - Create `internal/presenter/plain.go` with `type PlainPresenter struct { out io.Writer; err io.Writer }` and a constructor `NewPlainPresenter(out, err io.Writer) *PlainPresenter`. (The `err` writer is wired fully in Task cli-presentation-1-6; accept it now so the constructor is stable.)
> - Implement the minimal methods using only `fmt.Fprintf(p.out, …)`:
>   - `RunStarted` → `mint: releasing {project} v{X}`.

> **Acceptance Criteria**:
> - [ ] `PlainPresenter` satisfies `Presenter` and writes narration to the injected `out` writer.
> - [ ] A minimal sequence (`RunStarted` → `StageSucceeded` → `RunFinished`) produces the expected terse lines in order.

> **Context**:
> > Per-event rendering (plain): start of run → `mint: releasing {project} v{X}`; `StageSucceeded` → `{stage}: ok` / `{stage}: {detail}`; `StageFailed` → `{stage}: FAILED - {message}`; end of run → `done: {project} v{X} {url}`.

**Proposed** (task cli-presentation-1-4 — the affected fragments, updated):

> **Outcome**: Given an event stream, `PlainPresenter` emits the spec's plain lines (`mint: {action} {project} v{X}` — e.g. `mint: releasing acme v1.4.0` — `{stage}: {detail}`/`{stage}: ok`, `done: {project} v{X} {url}`) with the start-of-run action word taken from the engine-supplied `RunInfo.Action` (not hardcoded), and with zero ANSI/glyph/animation bytes and no UI-library import.

> **Do**:
> - Create `internal/presenter/plain.go` with `type PlainPresenter struct { out io.Writer; err io.Writer }` and a constructor `NewPlainPresenter(out, err io.Writer) *PlainPresenter`. (The `err` writer is wired fully in Task cli-presentation-1-6; accept it now so the constructor is stable.)
> - Implement the minimal methods using only `fmt.Fprintf(p.out, …)`:
>   - `RunStarted` → `mint: {action} {project} v{X}`, where `{action}` is the **engine-supplied** action word from `RunInfo.Action` (`releasing` for `release`, `regenerating` for `regenerate`) — render the supplied action, do **not** hardcode `releasing` (the same start-of-run event is reused for `regenerate` per cli-presentation-4-2, which requires the plain line `mint: regenerating {project} v{X}`). This mirrors the pretty brand line in cli-presentation-1-5 and honours 1-1's "no presenter code hardcodes the literal `releasing`."

> **Acceptance Criteria**:
> - [ ] `PlainPresenter` satisfies `Presenter` and writes narration to the injected `out` writer.
> - [ ] A minimal sequence (`RunStarted` → `StageSucceeded` → `RunFinished`) produces the expected terse lines in order.
> - [ ] The plain start-of-run line renders the engine-supplied `RunInfo.Action` word (e.g. `mint: releasing {project} v{X}` for `release`); no plain presenter code hardcodes the literal `releasing` (so `regenerate` renders `mint: regenerating {project} v{X}` per cli-presentation-4-2).

> **Context**:
> > Per-event rendering (plain): start of run → `mint: releasing {project} v{X}` (the release-shaped instance — the `{action}` word is engine-supplied and verb-shaped, so `regenerate` renders `mint: regenerating {project} v{X}`); `StageSucceeded` → `{stage}: ok` / `{stage}: {detail}`; `StageFailed` → `{stage}: FAILED - {message}`; end of run → `done: {project} v{X} {url}`.
> > Applies to every verb — `release`, `regenerate`, `init`, `version` all emit through the same `Presenter`; the start-of-run action word is therefore verb-shaped from `RunInfo.Action` (cli-presentation-1-1), not a release-only literal.

Add a corresponding test to the Tests list:

> - `"the start-of-run line renders the engine-supplied action word"` — `RunStarted{Action:"regenerating", Project:"acme", Version:"1.4.0"}` → `mint: regenerating acme v1.4.0`; `Action:"releasing"` → `mint: releasing acme v1.4.0` (no hardcoded `releasing`).

**Resolution**: Pending
**Notes**:

---
