---
status: complete
created: 2026-06-09
cycle: 2
phase: Plan Integrity Review
topic: CLI Presentation
---

# Review Tracking: CLI Presentation - Integrity

## Findings

### 1. Start-of-run brand line hardcodes the `release` verb ("releasing v{X}") but is reused verbatim by `regenerate`

**Severity**: Important
**Plan Reference**: Phase 1 — cli-presentation-1-1 (`RunInfo` payload), cli-presentation-1-5 (pretty `RunStarted` rendering); Phase 4 — cli-presentation-4-2 (regenerate reuses `RunStarted`)
**Category**: Task Self-Containment / Internal Consistency
**Change Type**: update-task (cli-presentation-1-1 and cli-presentation-1-5; coordination note in cli-presentation-4-2)

**Details**:
The end-of-run line was given an explicit verb-shaped treatment: cli-presentation-4-4 dispatches the closing line by an engine-supplied `Verb`/shape discriminator so `release` shows the URL footer, `regenerate` shows a url-less summary, and `init`/`version` show nothing. The **start-of-run** line received no equivalent treatment.

cli-presentation-1-5 renders the pretty start-of-run brand line with the literal verb-action text `🌿 mint · {project}  ›  releasing v{X}` (and cli-presentation-1-4 renders plain `mint: releasing {project} v{X}`). The `RunInfo` payload defined in cli-presentation-1-1 carries only `Project` and `Version` — there is no field for the verb-action word. Yet cli-presentation-4-2 instructs the implementer to render `regenerate` blocks by reusing `RunStarted` "exactly as `release` does." An implementer following 4-2 would therefore emit `🌿 mint · {project}  ›  releasing v{X}` (literal "releasing") for a `regenerate` run — which is wrong — or would be forced to invent an unplanned payload extension to vary the action text. Either way the implementer must make a design decision the plan should have settled, which is exactly what an integrity review must catch.

The spec's event-payload principle resolves the ambiguity cleanly: the engine supplies, in each event's payload, every datum the rendering consumes — so the start-of-run action text (like the engine-supplied brand leaf and the end-of-run verb shape) should arrive on `RunInfo`, not be hardcoded as "releasing". This keeps the start-of-run line consistent with the verb-shaped end-of-run line (4-4) and with the brand-leaf provenance already adopted in 1-5/4-3/4-4. The fix is to add an engine-supplied action/header field to `RunInfo` in 1-1 (where the payload is owned), render it instead of the literal "releasing" in 1-4/1-5, and add a coordination note in 4-2 so its `RunStarted` reuse supplies the regenerate action text rather than "releasing".

This finding adjusts cli-presentation-1-1 and cli-presentation-1-5 (the two highest-leverage points: payload definition and pretty rendering). The plain rendering (cli-presentation-1-4) consumes the same new field; its existing wording is already illustrative ("Exact wording is refinable"), so no separate edit is required there beyond rendering the supplied action — captured in the 1-1 acceptance criteria and the 4-2 note below.

**Current** (cli-presentation-1-1 — relevant `Do` bullets and one Acceptance Criterion):

> **Do**:
> - Create the Go module if not present (`go mod init` for the `mint` module) and create package directory `internal/presenter/`.
> - In `internal/presenter/presenter.go` declare `type Presenter interface { ... }` with the minimal methods (illustrative signatures — exact surface is settled here, at implementation):
>   - `RunStarted(info RunInfo)` — start-of-run brand/header line (e.g. carries `Project`, `Version`).
>   - `StageStarted(s StageStart)` — carries the stage `Name` and a `Blocking bool` flag (engine knowledge: it knows when it is about to invoke a long/slow command). Plain uses the flag to decide whether to emit a start line; pretty always shows progress.

> **Acceptance Criteria**:
> - [ ] `internal/presenter/presenter.go` declares a `Presenter` interface with the five minimal methods (start-of-run, `StageStarted`, `StageSucceeded`, `StageFailed`, end-of-run).

**Proposed** (cli-presentation-1-1 — same bullets, with the action field added):

> **Do**:
> - Create the Go module if not present (`go mod init` for the `mint` module) and create package directory `internal/presenter/`.
> - In `internal/presenter/presenter.go` declare `type Presenter interface { ... }` with the minimal methods (illustrative signatures — exact surface is settled here, at implementation):
>   - `RunStarted(info RunInfo)` — start-of-run brand/header line. `RunInfo` carries `Project`, `Version`, and an **engine-supplied action word** (`Action string`, e.g. `releasing` for `release`, `regenerating` for `regenerate`) so the start-of-run line is verb-shaped from the payload rather than hardcoding `releasing`. Per the event-payload principle the engine supplies the action word; the presenter renders it and never re-derives the verb. (This mirrors the verb-shaped end-of-run line owned by cli-presentation-4-4 and the engine-supplied brand leaf adopted in cli-presentation-1-5.)
>   - `StageStarted(s StageStart)` — carries the stage `Name` and a `Blocking bool` flag (engine knowledge: it knows when it is about to invoke a long/slow command). Plain uses the flag to decide whether to emit a start line; pretty always shows progress.

> **Acceptance Criteria**:
> - [ ] `internal/presenter/presenter.go` declares a `Presenter` interface with the five minimal methods (start-of-run, `StageStarted`, `StageSucceeded`, `StageFailed`, end-of-run).
> - [ ] `RunInfo` carries an engine-supplied `Action` word (e.g. `releasing`/`regenerating`) so the start-of-run line is verb-shaped from the payload; no presenter code hardcodes the literal `releasing`.

**Notes**: The `Action` field is the minimal spec-grounded extension; the worked examples (`release`) all use `releasing`, so the default/illustrative value stays `releasing` and existing tests are unaffected. (If preferred, the same data could be carried as the existing verb discriminator that cli-presentation-4-4 adds to the end-of-run payload, reused here — but adding it to `RunInfo` at the start-of-run definition point keeps 1-1 self-contained.)

---

### 1a. Pretty `RunStarted` must render the engine-supplied action word, not the literal "releasing"

**Severity**: Important
**Plan Reference**: Phase 1 — cli-presentation-1-5 (pretty `RunStarted`)
**Category**: Task Self-Containment / Internal Consistency
**Change Type**: update-task (cli-presentation-1-5)

**Details**:
Paired with finding 1: cli-presentation-1-5's `Do` and Context describe the brand line with the literal action word "releasing". Now that `RunInfo` carries an engine-supplied `Action` (finding 1), the rendering must consume it so the same `RunStarted` correctly renders `🌿 mint · {project}  ›  {action} v{X}` for `release` (`releasing`) and `regenerate` (`regenerating`) alike, exactly as 4-2 reuses it.

**Current** (cli-presentation-1-5 — first `Do` bullet of the method list):

> - `RunStarted` → top brand line `🌿 mint · {project}  ›  releasing v{X}` (flush-left). The brand leaf is **engine-supplied** (carried on the start-of-run/end-of-run payload, defaulting to `🌿`) rather than hardcoded, honouring the spec's "leaf ties to `commit_prefix`" note and the event-payload principle; render the supplied leaf, do not re-derive it.

**Proposed** (cli-presentation-1-5 — same bullet):

> - `RunStarted` → top brand line `🌿 mint · {project}  ›  {action} v{X}` (flush-left), where `{action}` is the **engine-supplied** action word from `RunInfo` (`releasing` for `release`, `regenerating` for `regenerate`) — render the supplied action, do not hardcode `releasing` (the same start-of-run event is reused for `regenerate` per cli-presentation-4-2). The brand leaf is likewise **engine-supplied** (carried on the start-of-run/end-of-run payload, defaulting to `🌿`) rather than hardcoded, honouring the spec's "leaf ties to `commit_prefix`" note and the event-payload principle; render the supplied leaf, do not re-derive it.

**Notes**: Add a corresponding acceptance criterion to 1-5 if desired: "The start-of-run brand line renders the engine-supplied `Action` word from `RunInfo` (not a hardcoded `releasing`)." The colour/downgrade and worked-example tests for the default `release` (`releasing`) value remain valid.

---

### 1b. Coordination note: cli-presentation-4-2 must supply the regenerate action word via `RunInfo`

**Severity**: Important
**Plan Reference**: Phase 4 — cli-presentation-4-2 (regenerate reuses `RunStarted`)
**Category**: Internal Consistency
**Change Type**: add-to-task (cli-presentation-4-2)

**Details**:
Paired with findings 1 and 1a: 4-2 currently says regenerate "reuses `RunStarted` ... exactly as `release` does," which — with the unfixed start-of-run line — would render "releasing" for a regenerate run. With the `Action` field added to `RunInfo` (finding 1), 4-2 should make explicit that the engine supplies `regenerating` (not `releasing`) as the per-block start-of-run action, so the reuse is correct rather than rendering the release verb. This is engine-supplied data, not new presenter logic — consistent with 4-2's existing "the presenter renders whatever ... the engine drives" stance.

**Current** (cli-presentation-4-2 — first `Do` bullet):

> - Confirm `regenerate` needs **no new rendering events** for the per-version blocks — each block reuses `RunStarted`/`StageStarted`/`StageSucceeded`/`StageFailed`/`ShowPlan`/`ShowNotes`/`Prompt` exactly as `release` does. The engine emits one block per version; the presenter renders them linearly in emit order (which the engine has ordered oldest→newest). Do **not** add per-version ordering logic to the presenter — the engine supplies the order. Add a doc note that block ordering is engine-owned.

**Proposed** (cli-presentation-4-2 — same bullet):

> - Confirm `regenerate` needs **no new rendering events** for the per-version blocks — each block reuses `RunStarted`/`StageStarted`/`StageSucceeded`/`StageFailed`/`ShowPlan`/`ShowNotes`/`Prompt` exactly as `release` does. The engine emits one block per version; the presenter renders them linearly in emit order (which the engine has ordered oldest→newest). Do **not** add per-version ordering logic to the presenter — the engine supplies the order. Add a doc note that block ordering is engine-owned. The per-block `RunStarted` carries the engine-supplied `Action` word `regenerating` (the `RunInfo.Action` field established in cli-presentation-1-1/1-5) so the start-of-run brand line reads `🌿 mint · {project}  ›  regenerating v{X}` (pretty) / `mint: regenerating {project} v{X}` (plain) — the presenter renders the supplied action, it does not hardcode `releasing`. Add a test that a regenerate block's start-of-run line uses `regenerating`, not `releasing`.

**Resolution**: Fixed
**Notes**:

---
