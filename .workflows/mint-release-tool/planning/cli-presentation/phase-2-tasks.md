---
phase: 2
phase_name: Run Narration — Stages, Plan, Notes, Warnings, Unwind
total: 8
---

## cli-presentation-2-1 | approved

### Task cli-presentation-2-1: Extend StageStarted/StageSucceeded payloads with long/blocking flag, detail, and engine-supplied elapsed

**Problem**: Phase 1 stubbed the stage payloads with the fields the skeleton needed, but the full run vocabulary depends on three engine-supplied data points being firmly in the contract before any rendering task can consume them: whether a stage is long/blocking (drives whether plain emits a start line and whether pretty/plain render elapsed), the stage detail string, and the engine-measured elapsed time. The spec's event-payload principle is non-negotiable here — the presenter never times stages or holds a hardcoded list of long stage names; every datum arrives in the payload. Locking the payload shapes first prevents the rendering tasks (2-2, 2-3) from being built against a moving contract.

**Solution**: Finalise the `StageStart` and `StageSucceed`-style payload structs in `internal/presenter` so they carry the `Blocking` long/blocking flag, the `Detail` string, and the engine-supplied `Elapsed time.Duration`, with documented semantics for the three edge cases (short stage carries no elapsed, blocking flag present but zero elapsed, empty detail). No rendering changes — this task stabilises the data contract and its tests assert the field shapes and zero-value semantics.

**Outcome**: `StageStart` carries `Name` + `Blocking`; `StageSuccess` carries `Name` + `Detail` + `Elapsed` + `Blocking`; the zero-value semantics (no elapsed on a short stage, zero `Elapsed` even when `Blocking`, empty `Detail`) are documented and locked by tests, so the rendering tasks can rely on the engine supplying everything.

**Do**:
- In `internal/presenter/presenter.go` (or wherever the Phase 1 payload structs live), confirm/extend `StageStart` to carry `Name string` and `Blocking bool` (the engine knows when it is about to invoke `claude` or a build hook — the presenter never infers this from the name).
- Confirm/extend the stage-success payload (`StageSuccess`) to carry `Name string`, `Detail string`, `Elapsed time.Duration`, and `Blocking bool`. The `Blocking` flag here mirrors the one on `StageStart` for the same stage so pretty/plain can decide whether to render `({elapsed})`.
- Add doc comments fixing the zero-value semantics relied on by 2-2/2-3:
  - A **short stage** (`Blocking == false`) carries no meaningful elapsed — renderers must not print elapsed for it regardless of the `Elapsed` value.
  - `Elapsed == 0` is a legal value even when `Blocking == true` (a long stage that completed instantly or whose timing rounded to zero) — renderers decide how to format zero (see 2-2/2-3); the payload does not special-case it.
  - `Detail == ""` is legal — renderers fall back to `ok`/detail-less forms (see 2-2/2-3); the payload does not invent a default.
- Keep this file rendering-free and dependency-free beyond stdlib `time`. Do not add formatting helpers here — formatting belongs to the per-mode presenters.
- Update the `RecordingPresenter` (from cli-presentation-1-2) if the field set changed so it still captures the full payload.

**Acceptance Criteria**:
- [ ] `StageStart` carries `Name` and `Blocking`; the stage-success payload carries `Name`, `Detail`, `Elapsed`, and `Blocking`.
- [ ] Doc comments state the three zero-value semantics: short stage carries no elapsed, `Elapsed == 0` is legal under `Blocking == true`, empty `Detail` is legal.
- [ ] The package compiles and imports nothing beyond stdlib (`time`); no formatting/rendering logic added.
- [ ] `RecordingPresenter` captures the full extended payload for both events (no field dropped).
- [ ] No method or struct derives engine knowledge — there is no stage-name list and no timing performed in the presenter package.

**Tests**:
- `"StageStart carries name and blocking flag"` — construct `StageStart{Name:"notes", Blocking:true}` and assert fields round-trip through the recorder.
- `"StageSuccess carries detail, elapsed, and blocking"` — construct with `Detail:"generated", Elapsed:1100*time.Millisecond, Blocking:true` and assert all captured.
- `"a short stage success carries no meaningful elapsed"` — construct `StageSuccess{Blocking:false}`; the contract test asserts renderers are free to ignore `Elapsed` (documents intent; the actual omission is verified in 2-2/2-3).
- `"zero elapsed is legal on a blocking stage"` — `StageSuccess{Blocking:true, Elapsed:0}` is a valid value (no panic, no special-casing in the payload layer).
- `"empty detail is legal"` — `StageSuccess{Detail:""}` is a valid value.

**Edge Cases**:
- **Short stage carries no elapsed** — `Blocking == false` means elapsed is not rendered; the payload does not enforce a zero, the renderers honour the flag.
- **Long flag present but zero elapsed** — `Blocking == true` with `Elapsed == 0` is legal and must not be treated as "no elapsed"; formatting of zero is the renderer's call.
- **Detail empty** — `Detail == ""` is legal; the payload supplies no default; renderers fall back (2-2/2-3).

**Context**:
> Event payload principle: the engine supplies, in each event's payload, every datum the renderings consume — the presenter never re-derives engine knowledge or hardcodes stage-specific logic.
> `StageStarted` carries whether the stage is long/blocking (engine knowledge — it knows when it is about to invoke `claude` or run a build hook). The plain presenter uses this flag to decide whether to emit a start line; the pretty presenter always shows a spinner. The presenter does not hold a hardcoded list of long stage names.
> `StageSucceeded` carries its detail string and the elapsed time, both measured/supplied by the engine (the presenter does not time stages). Pretty renders `({elapsed})` on long/blocking stages only (the same stages flagged on `StageStarted`); short stages render detail without elapsed.
> Phase 1 established the payload structs (`StageStart`, `StageSuccess`) with `Blocking`/`Elapsed`/`Detail` fields designed to allow this extension without churn — this task finalises and documents their semantics.

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "The `Presenter` Seam (Architecture)" (Event payload principle), "The Plain Layer", "The Pretty Layer".

## cli-presentation-2-2 | approved

### Task cli-presentation-2-2: Plain stage narration — start line for long/blocking stages only, completion line per stage

**Problem**: In `plain` mode a live-tail consumer (`mint release | tee log`, a streaming agent) must not stare at silence through a multi-second wait while AI notes generate or a build hook runs — yet short stages must stay one-line-on-completion for token efficiency. This is plain's equivalent of the pretty spinner: a terse start line for long/blocking stages only, plus exactly one completion line per stage. The Phase 1 plain presenter honoured the `Blocking` flag minimally; this task fixes the full plain stage vocabulary and its detail/`ok` fallback.

**Solution**: Extend `PlainPresenter`'s `StageStarted`/`StageSucceeded` rendering so a start line is emitted **only** when `Blocking == true` (e.g. `notes: generating…`), short stages emit nothing on start, and every stage emits a completion line on success — `{stage}: {detail}` when detail is present, `{stage}: ok` when detail is empty. Elapsed handling on the completion line follows the blocking flag (long stages may show `({elapsed})` per the worked example, e.g. `notes: generated (1.1s)`; short stages show no elapsed).

**Outcome**: Given a stage event stream, `PlainPresenter` emits a start line for blocking stages only, no start line for short stages, and one completion line per stage that reads `{stage}: {detail}` or `{stage}: ok`, with elapsed appended only for blocking stages — all terse, lowercase, ANSI/glyph-free.

**Do**:
- In `internal/presenter/plain.go`, implement `StageStarted(s StageStart)`: when `s.Blocking` is true, write a terse start line to `out` (e.g. `{name}: generating…` for the worked notes example — keep wording terse and generic; the spec wording is illustrative). When `s.Blocking` is false, write **nothing**.
- Implement `StageSucceeded(s StageSuccess)` to write exactly one completion line to `out`:
  - detail present → `{stage}: {detail}`.
  - detail empty → `{stage}: ok`.
  - When `s.Blocking` is true, append the elapsed in the worked-example form ` (1.1s)` after the detail/`ok` text (e.g. `notes: generated (1.1s)`, `prep: pre_tag ok (2.3s)`). When `s.Blocking` is false, append no elapsed.
- Format elapsed consistently and compactly (e.g. seconds with one decimal, mirroring the worked examples `(2.3s)`/`(1.1s)`). Keep the formatting helper local to the plain presenter.
- Use only `fmt`/`io` — no ANSI, no glyphs, no UI library (honour the Phase 1 plain constraints).
- Keep these to `out` only — stage success/start are narration, never stderr.

**Acceptance Criteria**:
- [ ] A blocking `StageStarted` emits a terse start line; a short (`Blocking == false`) `StageStarted` emits no start line.
- [ ] Every `StageSucceeded` emits exactly one completion line: `{stage}: {detail}` when detail present, `{stage}: ok` when detail empty.
- [ ] A blocking stage's completion line carries an elapsed suffix (`(…s)`); a short stage's completion line carries no elapsed suffix.
- [ ] A blocking stage emits a start line **then** a completion line (two lines total); a short stage emits one line total (completion only).
- [ ] Output is terse, lowercase, and contains no ESC byte, no `\r`, and no glyphs.

**Tests**:
- `"a short stage emits only a completion line"` — drive `StageStarted{Blocking:false}` + `StageSucceeded`; assert exactly one line and no start line.
- `"a long stage emits a start line then a completion line"` — drive `StageStarted{Blocking:true}` + `StageSucceeded{Blocking:true, Elapsed:1100ms}`; assert two lines, start then completion, completion includes `(1.1s)`.
- `"a completion with no detail renders {stage}: ok"` — `StageSucceeded{Name:"x", Detail:""}` → `x: ok`.
- `"a completion with detail renders {stage}: {detail}"` — `StageSucceeded{Name:"preflight", Detail:"ok (clean, on main, tag free, in sync)"}` round-trips verbatim.
- `"a short stage completion carries no elapsed even if Elapsed is set"` — `StageSucceeded{Blocking:false, Elapsed:5s}` produces no `(…s)` suffix.

**Edge Cases**:
- **Short stage emits no start line** — `Blocking == false` on `StageStarted` produces no output; only the completion line appears.
- **Long stage emits start then completion** — `Blocking == true` produces two lines, in order.
- **Completion with no detail renders `{stage}: ok`** — empty `Detail` falls back to `ok`, never to an empty value.

**Context**:
> Start line for long/blocking stages only. A stage that blocks on something slow (AI notes generation, a `pre_tag` build hook) also emits a terse start line (`notes: generating…` → `notes: generated (1.1s)`), so a live-tail consumer isn't staring at silence through a multi-second wait. Short stages stay one-line-on-completion — no start line. This is plain's equivalent of the pretty spinner.
> Per-event rendering (plain): `StageStarted` → (blank for short stages; long/blocking stages emit a terse start line, e.g. `notes: generating…`); `StageSucceeded` → `{stage}: ok` / `{stage}: {detail}`.
> Worked plain example lines: `prep: pre_tag ok (2.3s)`, `notes: generated (1.1s)`, `preflight: ok (clean, on main, tag free, in sync)`.
> Exact wording is refinable at implementation; the blocking-only start line, the one-completion-line-per-stage rule, and the `ok` fallback are the fixed requirements. The full pretty spinner animation is out of scope (Phase 4) — plain never animates.

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "The Plain Layer" (Contract, Per-event rendering), "Spinner Lifecycle" (plain never animates).

## cli-presentation-2-3 | approved

### Task cli-presentation-2-3: Pretty stage narration — stage line with detail and conditional elapsed

**Problem**: The `pretty` stage line is the styled human form of a stage completion — two-space indent, green `✓` glyph, stage name padded to a column, terse detail, and `({elapsed})` **only** on long/blocking stages. The Phase 1 skeleton rendered a minimal `✓` line; this task fixes the full pretty completion line including the conditional elapsed and the detail-only form. The spinner *animation* between `StageStarted` and completion is explicitly out of scope here (Phase 4 owns the spinner lifecycle) — this task renders the static stage line.

**Solution**: Extend `PrettyPresenter`'s `StageSucceeded` rendering to produce the styled completion line `  ✓ {stage}  {detail}` with the stage name padded to a column, appending ` ({elapsed})` only when `Blocking == true`, and rendering a detail-only line when no elapsed applies. Styling (green glyph, layout) goes through the injected `lipgloss` renderer with colour auto-downgrade preserved from Phase 1.

**Outcome**: Given stage-success events, `PrettyPresenter` emits `  ✓ {stage}  {detail}` lines with consistent column padding, `({elapsed})` appended on long/blocking stages and omitted on short stages, the green glyph under colour and the glyph-plus-layout preserved under colour downgrade.

**Do**:
- In `internal/presenter/pretty.go`, implement `StageSucceeded(s StageSuccess)` to render a single stage line to `out`:
  - Two-space indent, green `✓` glyph (via the lipgloss renderer), stage name padded to a fixed column (mirroring the worked example where `version`/`preflight`/`prep`/`notes`/`record`/`tag/push`/`publish` align), then the terse `s.Detail`.
  - When `s.Blocking == true`, append ` ({elapsed})` after the detail (e.g. `  ✓ prep       pre_tag: npm ci && npm run build (2.3s)`, `  ✓ notes      generated (1.1s)`). When `s.Blocking == false`, append no elapsed (e.g. `  ✓ preflight  clean · on main · tag free · in sync with origin`).
  - When `s.Detail == ""`, render the glyph + padded name with no trailing detail (detail-only/`ok`-style line) — and still honour the elapsed rule (elapsed appended only if blocking).
- Format elapsed compactly mirroring the worked examples (`(2.3s)`, `(1.1s)`). Keep the formatter local to the pretty presenter (or shared with plain via an internal helper, implementer's choice — document if shared).
- Use the injected `*lipgloss.Renderer` for colour; do not add any `NO_COLOR`/`TERM` check (Phase 1 established lipgloss auto-downgrade).
- Do **not** implement the spinner animation, frame ticking, or in-place replacement — that is Phase 4. Render the static completion line only. `StageStarted` may keep its Phase 1 static dim line; this task does not need to change it (the spinner lifecycle is Phase 4).
- Stage lines stay fixed-width short (no wrapping concerns); decorative-rule width capping is a Phase 4 concern, not here.

**Acceptance Criteria**:
- [ ] A long/blocking stage success renders `  ✓ {stage}  {detail} ({elapsed})` with the elapsed appended.
- [ ] A short stage success renders `  ✓ {stage}  {detail}` with **no** elapsed.
- [ ] A stage with empty detail renders a detail-only line (glyph + padded name, no trailing detail text), honouring the elapsed rule.
- [ ] The stage name is padded to a consistent column so successive stage lines align.
- [ ] Under colour-on, the glyph/line carries ANSI colour codes; under colour-downgrade, no colour codes are emitted while the `✓` glyph, indent, and column padding survive.
- [ ] No spinner animation or in-place line replacement is implemented in this task (deferred to Phase 4).

**Tests**:
- `"a long stage shows elapsed"` — `StageSucceeded{Name:"notes", Detail:"generated", Elapsed:1100ms, Blocking:true}` → line ends with `(1.1s)`.
- `"elapsed is omitted on a short stage"` — `StageSucceeded{Name:"preflight", Detail:"...", Blocking:false}` → line has no `(…s)` suffix even if `Elapsed` is non-zero.
- `"a detail-only line renders glyph + name with no trailing detail"` — `StageSucceeded{Name:"x", Detail:""}` renders `✓ x` with padding and no detail text.
- `"stage names are padded to a common column"` — render two stages with different-length names; assert their detail columns align.
- `"colour downgrade preserves glyph and layout"` — force no-colour profile; assert no SGR codes yet `✓`, indent, and padding remain.

**Edge Cases**:
- **Elapsed omitted on short stage** — `Blocking == false` → no `({elapsed})`, regardless of the `Elapsed` value.
- **Detail-only line** — empty `Detail` renders glyph + padded name with no trailing text (not an empty detail slot artefact).
- **Long stage shows elapsed** — `Blocking == true` → `({elapsed})` appended after the detail.

**Context**:
> Stage lines: two-space indent, glyph, stage name padded to a column, terse detail. Symmetry/consistency is the bar — no ad-hoc indentation.
> `StageSucceeded` (pretty) → `✓ {stage}  {detail} ({elapsed})`, glyph green. Pretty renders `({elapsed})` on long/blocking stages only (the same stages flagged on `StageStarted`); short stages render detail without elapsed.
> Worked example: `✓ preflight  clean · on main · tag free · in sync with origin` (short, no elapsed); `✓ prep       pre_tag: npm ci && npm run build (2.3s)` and `✓ notes      generated (1.1s)` (long, with elapsed).
> Spinners are a `pretty`-only concern owned inside the pretty presenter (a spinner spans the gap between `StageStarted` and `StageSucceeded`/`StageFailed`) — the full spinner lifecycle is hardened in Phase 4. This task renders the static stage line, not the animation.

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "The Pretty Layer" (Status glyphs, Stage lines, worked example), "The `Presenter` Seam" (Event payload principle for elapsed), "Spinner Lifecycle" (deferred to Phase 4).

## cli-presentation-2-4 | approved

### Task cli-presentation-2-4: ShowPlan renders structured steps — pretty bulleted block, plain semicolon-joined one-liner

**Problem**: The plan is a list of structured steps (each a verb + target), not pre-formatted text — the engine must not pre-render it, and there is no separate verbose/terse payload. Both modes format from the **same** structured steps: pretty as a bulleted `Plan` block, plain as a `plan: …; …` one-liner. This is the first new `Presenter` method introduced in Phase 2, so it must be added to the interface, both implementations, and the recording presenter.

**Solution**: Add a `ShowPlan(plan Plan)` method to the `Presenter` interface carrying structured steps (a slice of `{Verb, Target}` steps), implement it in both presenters (pretty bulleted block; plain semicolon-joined one-liner), and record it in `RecordingPresenter`. Both renderings derive from the same `[]PlanStep` — no second pre-formatted field.

**Outcome**: Given a structured plan payload, pretty renders a `Plan` header with one bulleted `• {verb}  {target}` line per step, and plain renders a single `plan: {verb} {target}; {verb} {target}; …` line — both from the identical structured payload, handling single-step, empty, and empty-target cases cleanly.

**Do**:
- In `internal/presenter/presenter.go`, define `type PlanStep struct { Verb string; Target string }` and `type Plan struct { Steps []PlanStep }` (or `ShowPlan(steps []PlanStep)` — choose one; document the shape). Add `ShowPlan(plan Plan)` to the `Presenter` interface. Add the no-op/record implementation to `RecordingPresenter`.
- Plain (`internal/presenter/plain.go`) `ShowPlan`: write a single line `plan: {step1}; {step2}; …` to `out`, where each step renders as `{verb} {target}` (e.g. `plan: commit changelog+version; tag v1.4.0; push --atomic; publish github`). Join with `; `. When a step's `Target` is empty, render just the `{verb}` for that step (no trailing space). When there are no steps, render `plan:` with an empty body (or `plan: ` — document the exact form; prefer `plan:` with nothing after, no dangling separators).
- Pretty (`internal/presenter/pretty.go`) `ShowPlan`: render a `Plan` header line (indented, per the worked example) followed by one bulleted line per step: `    • {verb}   {target}` with the verb padded to a column for alignment (mirroring `• commit   CHANGELOG.md + bin/acme` / `• tag      v1.4.0 (annotated)`). When a step's `Target` is empty, render `• {verb}` with no trailing target. When there are no steps, render the `Plan` header with no bullet lines (or omit the header entirely — document the choice; prefer omitting the block when there are no steps so an empty plan produces no orphan header).
- Both renderings consume the **same** `[]PlanStep`; there is no separate terse field and no engine-supplied pre-formatted string.
- Plan is narration → `out` only, never stderr.

**Acceptance Criteria**:
- [ ] `ShowPlan` is on the `Presenter` interface and recorded by `RecordingPresenter` with its full structured payload.
- [ ] Plain renders one `plan: …` line with steps joined by `; ` as `{verb} {target}` from the structured steps.
- [ ] Pretty renders a `Plan` header plus one bulleted `• {verb}  {target}` line per step, verbs padded to a column.
- [ ] A single-step plan renders correctly in both modes (one bullet / one item, no dangling separator).
- [ ] An empty plan (no steps) produces no dangling separators in plain and no orphan bullet/header in pretty (documented empty-plan form).
- [ ] A step with an empty target renders just `{verb}` (no trailing space/separator) in both modes.

**Tests**:
- `"plain joins steps into a semicolon one-liner"` — steps `commit changelog+version`, `tag v1.4.0`, `push --atomic`, `publish github` → `plan: commit changelog+version; tag v1.4.0; push --atomic; publish github`.
- `"pretty renders a bulleted Plan block"` — same steps → `Plan` header + four `• {verb}  {target}` lines, verbs aligned.
- `"a single-step plan renders one item with no separator"` — one step; plain has no `;`, pretty has one bullet.
- `"an empty plan renders no steps and no dangling separators"` — zero steps; plain `plan:` (no trailing `; `), pretty has no orphan bullet (and no orphan header per the documented choice).
- `"a step with an empty target renders just the verb"` — `{Verb:"publish", Target:""}` → plain `…publish` (no trailing space), pretty `• publish` (no trailing target).

**Edge Cases**:
- **Single step** — exactly one item; no separator in plain, one bullet in pretty.
- **Empty plan with no steps** — no dangling `; `, no orphan bullet/header.
- **Step with empty target** — render `{verb}` alone with no trailing space or separator.

**Context**:
> `ShowPlan` carries structured plan steps (each a verb + target), not pre-formatted text. Pretty renders them as a bulleted block; plain joins them into a `plan: …; …` one-liner. Each presenter formats from the same structured steps — there is no separate verbose/terse payload, and the abbreviations shown in the worked examples are illustrative wording, not a distinct terse field.
> Pretty worked example:
> ```
>   Plan
>     • commit   CHANGELOG.md + bin/acme
>     • tag      v1.4.0 (annotated)
>     • push     --atomic → origin
>     • publish  GitHub release
> ```
> Plain worked example: `plan: commit changelog+version; tag v1.4.0; push --atomic; publish github`.
> The worked-example abbreviations (`changelog+version`, `github`) are illustrative wording supplied by the engine in the step targets — not a distinct terse payload the presenter computes.
> Plan is narration → stdout.

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "The `Presenter` Seam" (ShowPlan payload), "The Pretty Layer" (Plan block), "The Plain Layer" (Per-event rendering, ShowPlan).

## cli-presentation-2-5 | approved

### Task cli-presentation-2-5: ShowNotes renders byte-identical body with per-mode delimiters

**Problem**: The release-notes body is the one piece of output that must be **byte-identical** across both modes — the spec calls this non-negotiable: stripping or transforming the body would contradict the engine's "use the body whole" rule and break the "what previews is what ships" invariant. Only the surrounding **delimiters** differ (pretty: titled rule + closing rule, no box; plain: `--- release notes v{X} ---` … `--- end notes ---`). Emoji headers must be preserved. This adds a second new `Presenter` method.

**Solution**: Add a `ShowNotes(notes Notes)` method carrying the version and the verbatim body, implement it in both presenters with per-mode delimiters wrapping the body, and assert byte-identity of the body bytes between modes. The body is written through unchanged — no stripping, no transforming, no re-wrapping.

**Outcome**: Given a notes payload, pretty renders `── release notes · v{X} ──` + body + `────` closing rule, plain renders `--- release notes v{X} ---` + body + `--- end notes ---`, and the body bytes between the two renderings are identical (emoji headers, blank lines, and delimiter-like body lines all preserved).

**Do**:
- In `internal/presenter/presenter.go`, define `type Notes struct { Version string; Body string }` (or `ShowNotes(version, body string)` — document the shape) and add `ShowNotes(notes Notes)` to the `Presenter` interface. Add the record implementation to `RecordingPresenter`.
- Plain (`internal/presenter/plain.go`) `ShowNotes`: write `--- release notes v{X} ---`, then the body **verbatim** (exactly the bytes supplied, no trailing/leading manipulation beyond a single newline separating the delimiter lines from the body), then `--- end notes ---`. Use only `fmt`/`io`.
- Pretty (`internal/presenter/pretty.go`) `ShowNotes`: write a titled rule `── release notes · v{X} ──` (a lipgloss-styled rule, **no box**), the body **verbatim**, and a closing `────` rule. Decorative-rule width capping (`min(terminalWidth, ~50)`) is a Phase 4 concern — for this task render a fixed-width rule; note the cap is deferred. The body must wrap naturally and never truncate (do not implement any truncation).
- The body bytes must be written **identically** in both modes — write the exact `notes.Body` with no per-mode stripping, emoji removal, case-folding, or re-wrapping. If a shared helper writes the body, both presenters must call it with the unchanged body.
- Notes are narration → `out` only, never stderr.
- Handle the edge cases: empty body (delimiters still render, body section empty — document whether an empty line appears between delimiters; prefer delimiters back-to-back with no spurious blank line); body containing lines that look like delimiters (e.g. a body line `--- end notes ---`) is written verbatim and **not** escaped or treated as a delimiter (the delimiters are positional, not content-matched).

**Acceptance Criteria**:
- [ ] `ShowNotes` is on the `Presenter` interface and recorded by `RecordingPresenter`.
- [ ] Plain wraps the body in `--- release notes v{X} ---` … `--- end notes ---`; pretty wraps it in a titled rule + closing rule (no box).
- [ ] The body bytes written by plain and pretty are **identical** (byte-for-byte), proven by extracting the body region from each and comparing.
- [ ] Emoji headers (`✨ Features`, `🐛 Fixes`) in the body are preserved verbatim in both modes — no stripping.
- [ ] A body line that itself looks like a delimiter is written verbatim and not treated as a real delimiter.
- [ ] A multi-line body with internal blank lines preserves those blank lines exactly.
- [ ] No truncation of the body in either mode.

**Tests**:
- `"plain wraps the body in plain delimiters"` — assert `--- release notes v1.4.0 ---` opener and `--- end notes ---` closer around the body.
- `"pretty wraps the body in a titled rule and closing rule with no box"` — assert the `── release notes · v1.4.0 ──`-style opener and a closing rule; assert no box-drawing border characters surround the body.
- `"the body is byte-identical across modes"` — render the same `Notes` in both modes, extract the body region from each, assert equality byte-for-byte.
- `"emoji headers are preserved"` — body containing `✨ Features`/`🐛 Fixes` renders those bytes unchanged in both modes.
- `"a delimiter-like body line is rendered verbatim"` — body containing a literal `--- end notes ---` line is written through and the real closing delimiter still follows it.
- `"an empty body renders delimiters with no spurious content"` — empty `Body`; assert opener and closer present and no invented body text.
- `"a multi-line body with blank lines preserves the blanks"` — body with internal `\n\n` round-trips exactly.

**Edge Cases**:
- **Empty body** — delimiters still render; no spurious blank line or invented content between them (documented form).
- **Body with emoji headers** — `✨`/`🐛` headers preserved verbatim in both modes (non-negotiable, not stylistic).
- **Body containing delimiter-like lines** — a body line resembling `--- end notes ---` is written verbatim and not mistaken for the real delimiter (delimiters are positional).
- **Multi-line body with blank lines** — internal blank lines preserved exactly.

**Context**:
> Only the delimiters and stage narration differ from pretty; the notes body is byte-identical in both modes.
> Notes body verbatim — the same bytes as pretty/tag/changelog/release, emoji headers shown if present (`✨ Features`, `🐛 Fixes`). No stripping/transforming. This is non-negotiable, not stylistic: stripping or transforming the body would contradict the engine's "use the body whole" rule and break the "what previews is what ships" invariant. The emoji headers superficially cut against plain's token-efficiency goal, but the extra tokens are negligible and preview/ship parity wins — plain mode must not be "optimised" by stripping them.
> Plain: notes block delimited by plain rules: `--- release notes v{X} ---` … `--- end notes ---`, so a reader can slice it out reliably.
> Pretty: a titled `── release notes · v{X} ──` rule, the body verbatim, a closing `────` rule. The rounded box was dropped: it forced wrap/truncate on arbitrary-width AI notes and read as clutter.
> Width robustness — decorative rules capped at `min(terminalWidth, ~50)`; everything else wraps naturally — never truncate. (The cap is hardened in Phase 4; this task renders a fixed-width rule and must not truncate the body.)

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "The Plain Layer" (Notes block, Notes body verbatim), "The Pretty Layer" (Release notes — no box), "The `Presenter` Seam".

## cli-presentation-2-6 | approved

### Task cli-presentation-2-6: Warn renders structured label + message in both modes and to stderr

**Problem**: A warning carries a structured `label` (e.g. `post_release`) and a `message` — the presenter must **not** parse a label out of a single string. Both modes render label-prefixed (`⚠ {label}  {message}` / `{label}: WARN - {message}`), and per the stream contract warnings go to stderr in addition to appearing in the narration. A warning can fire on an otherwise-successful run (a post-release hook failure does not fail the release), so it must not be coupled to failure handling.

**Solution**: Add a `Warn(w Warning)` method carrying structured `Label` and `Message`, implement it in both presenters with the label-prefixed rendering, and route it to both `out` (narration) and `err` (stderr) per the stream contract.

**Outcome**: Given a warning payload, pretty renders an amber `⚠ {label}  {message}` line and plain renders `{label}: WARN - {message}`, both written to stdout narration **and** stderr, with multiple sequential warnings each rendered independently and a warning on a successful run not suppressing the success end-of-run line.

**Do**:
- In `internal/presenter/presenter.go`, define `type Warning struct { Label string; Message string }` and add `Warn(w Warning)` to the `Presenter` interface. Add the record implementation to `RecordingPresenter`.
- Plain (`internal/presenter/plain.go`) `Warn`: render `{label}: WARN - {message}` to **both** `out` and `err`. Use only `fmt`/`io`.
- Pretty (`internal/presenter/pretty.go`) `Warn`: render an amber-styled `⚠ {label}  {message}` line (via the lipgloss renderer) to `out`; also write the warning to `err`. For the stderr copy, follow the established stream rule from Phase 1 (the one-line summary goes to stderr) — render the warning line to err as well (styling on the err copy follows the same renderer; document if the err copy is plainer).
- The presenter must **not** split or parse a single combined string into label/message — both arrive as separate fields.
- An empty `Message` renders `{label}: WARN - ` (plain) / `⚠ {label}  ` (pretty) — the label still prefixes; document the exact trailing form (prefer no dangling separator artefacts beyond the fixed prefix).
- `Warn` is independent of `StageFailed`/`Unwound` — it does not suppress the success end-of-run line and does not set any failure state (suppression is 2-8's concern, driven by `StageFailed`/`Unwound`, not `Warn`).

**Acceptance Criteria**:
- [ ] `Warn` is on the `Presenter` interface, carries structured `Label`+`Message`, and is recorded by `RecordingPresenter`.
- [ ] Plain renders `{label}: WARN - {message}`; pretty renders `⚠ {label}  {message}` (amber under colour).
- [ ] The warning is written to **both** stdout narration and stderr in both modes.
- [ ] The presenter never parses a label out of a single string — `Label` and `Message` are separate inputs.
- [ ] Multiple warnings in sequence each render independently (no collapsing/de-duplication), in order.
- [ ] A warning does not suppress the success end-of-run line (a warn-only run still ends with the success line).
- [ ] An empty message still renders the label-prefixed form without crashing or inventing content.

**Tests**:
- `"plain renders the label-prefixed WARN line to stdout and stderr"` — `Warning{Label:"post_release", Message:"hook failed: scripts/notify.sh exited 1"}` → `post_release: WARN - hook failed: scripts/notify.sh exited 1` in both buffers.
- `"pretty renders an amber warn line to stdout and stderr"` — assert `⚠ post_release  …` on stdout (amber under colour) and the warning present on stderr.
- `"multiple warnings render in sequence"` — two `Warn` calls produce two independent lines in order in both buffers.
- `"a warn on a successful run does not suppress the end-of-run line"` — drive `Warn` then `RunFinished`; assert the success line still renders.
- `"an empty message renders the label prefix without inventing content"` — `Warning{Label:"x", Message:""}` → `x: WARN - ` (plain) with no extra text.

**Edge Cases**:
- **Empty message** — the label still prefixes; no invented message text, no crash.
- **Multiple warnings in sequence** — each rendered independently and in order; none collapsed.
- **Warn on an otherwise-successful run** — does not flip the run to failure and does not suppress the success end-of-run line.

**Context**:
> `Warn` carries a structured `label` and `message` (e.g. `post_release` + the failure text). Both renderings are label-prefixed (`⚠ {label}  {message}` / `{label}: WARN - {message}`); the presenter does not parse a label out of a single string.
> Per-event rendering: `Warn` → pretty `⚠ {label}  {message}`, amber; plain `{label}: WARN - {message}` (also stderr).
> Errors + warnings → stderr — for visibility under redirection. Errors/warnings appear in both the narration and on stderr.
> Pretty failure example includes a post-release warn: `⚠ post_release  hook failed (tag is already published): scripts/notify.sh exited 1` — a warn can occur on an otherwise-successful run (a post-release hook failure does not fail the release).
> Status glyph: `⚠` warn (amber).

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "The `Presenter` Seam" (Warn payload), "The Pretty Layer" (Status glyphs, warn example), "The Plain Layer" (Per-event rendering, Errors/warnings), "Render-Mode Detection & Output Streams" (Output streams).

## cli-presentation-2-7 | approved

### Task cli-presentation-2-7: StageFailed renders captured underlying output; FAILED summary to stderr, captured body not

**Problem**: When a stage fails, `mint` has buffered the underlying command's output (git/`claude`/`gh` chatter) rather than streaming it, so it can render it cleanly — and it must render it itself (never engine-printed) in **both** modes: pretty below the `✗` line, plain below the `{stage}: FAILED - {message}` line wrapped in sliceable delimiters mirroring the notes block. Critically, the stream split is subtle: the one-line `FAILED`/error summary additionally goes to stderr for redirect-visibility, but the **multi-line captured body is not duplicated to stderr**. Phase 1 wired the one-line summary to stderr; this task adds the captured-body rendering to stdout and locks the no-duplication rule.

**Solution**: Extend `StageFailed` rendering in both presenters to render the captured underlying output (`Output` field from the Phase 1 payload) — pretty below the `✗ {stage}  {message}` line, plain below the `{stage}: FAILED - {message}` line wrapped in `--- output ---` … `--- end output ---` delimiters — writing the captured body to `out` only, while the one-line FAILED summary continues to go to both `out` and `err`.

**Outcome**: Given a `StageFailed` with captured output, both modes render the FAILED summary line plus the captured body to stdout; the one-line summary also appears on stderr; the multi-line captured body does **not** appear on stderr; and when there is no captured output the FAILED line renders alone with no empty delimiter block.

**Do**:
- Confirm the Phase 1 `StageFailure` payload carries `Name`, `Message`, and `Output string` (the captured underlying-command output). No interface change needed if Phase 1 included `Output`; otherwise add it.
- Plain (`internal/presenter/plain.go`) `StageFailed`:
  - Write `{stage}: FAILED - {message}` to **both** `out` and `err` (the one-line summary; Phase 1 established the stderr copy).
  - When `Output` is non-empty, write the captured body to `out` **only**, wrapped in sliceable delimiters mirroring the notes block: `--- output ---`, the captured body verbatim, `--- end output ---`.
  - When `Output` is empty, write **no** delimiter block (the FAILED line stands alone).
  - Do not write the captured body to `err`.
- Pretty (`internal/presenter/pretty.go`) `StageFailed`:
  - Render the red `✗ {stage}  {message}` line to `out`, and write the one-line summary to `err` (per Phase 1).
  - When `Output` is non-empty, render the captured body below the `✗` line to `out` only (no box; the captured body is verbatim text — styling is minimal/dim, document the treatment). When empty, render no body block.
  - Do not write the captured body to `err`.
- The captured body, like the notes body, is written **verbatim** — no stripping/transforming. A captured body line that itself looks like a delimiter (`--- end output ---`) is written verbatim and not treated as the real delimiter (delimiters are positional, mirroring 2-5).
- Multi-line captured output preserves internal newlines and blank lines exactly.

**Acceptance Criteria**:
- [ ] Both modes render the one-line FAILED summary to stdout **and** stderr.
- [ ] Both modes render the captured `Output` to stdout: plain wrapped in `--- output ---` … `--- end output ---`, pretty below the `✗` line.
- [ ] The multi-line captured body is **not** written to stderr in either mode.
- [ ] When `Output` is empty, no delimiter/body block is rendered (the FAILED line stands alone) — no empty `--- output --- / --- end output ---` pair.
- [ ] A captured-body line resembling a delimiter is rendered verbatim and not mistaken for the real delimiter.
- [ ] Multi-line captured output preserves internal newlines/blank lines exactly.

**Tests**:
- `"plain renders the FAILED line plus a delimited output block to stdout"` — `StageFailure{Name:"tag/push", Message:"push rejected: remote moved", Output:"...git chatter..."}` → stdout has `tag/push: FAILED - push rejected: remote moved` then `--- output ---` … `--- end output ---`.
- `"the FAILED summary appears on stderr without the captured body"` — assert stderr contains the one-line summary and does **not** contain the captured body bytes or the output delimiters.
- `"pretty renders the captured output below the ✗ line on stdout"` — assert the `✗` line then the captured body on stdout; stderr has only the summary.
- `"empty captured output renders the FAILED line alone"` — `Output:""` → no `--- output ---` block in plain, no body block in pretty.
- `"a delimiter-like captured line is rendered verbatim"` — `Output` containing a literal `--- end output ---` line is written through and the real closing delimiter still follows.
- `"multi-line captured output preserves newlines"` — multi-line `Output` round-trips exactly within the delimiters.

**Edge Cases**:
- **Empty captured output** — FAILED line renders alone; no empty delimiter pair, no body block.
- **Captured output containing delimiter-like lines** — written verbatim; delimiters are positional, not content-matched.
- **Multi-line captured output** — internal newlines and blank lines preserved exactly.
- **FAILED summary on stderr without captured body** — the one-line summary is duplicated to stderr; the multi-line captured body is not.

**Context**:
> `StageFailed` carries the error message and the captured underlying-command output (the git/`claude`/`gh` chatter `mint` buffered instead of streaming). The presenter renders it — never engine-printed — in both modes: pretty below the `✗` line, plain below the `{stage}: FAILED - {message}` line wrapped in a sliceable delimiter (e.g. `--- output ---` … `--- end output ---`, mirroring the notes-block delimiting so a reader/agent can extract it). The captured output is narration → stdout; per the stream contract the one-line `FAILED`/error summary additionally goes to stderr for redirect-visibility (the multi-line captured body is not duplicated to stderr).
> Per-event rendering: `StageFailed` → pretty `✗ {stage}  {message}`, glyph red; plain `{stage}: FAILED - {message}` (also stderr).
> Underlying command output (git/claude/gh chatter) is captured by `mint`, not streamed through the spinner line, so the animation can't be corrupted. On failure, `mint` prints the captured output below the `✗` line.
> Status glyph: `✗` failure (red).

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "The `Presenter` Seam" (StageFailed payload), "The Pretty Layer" (Status glyphs, failure example), "The Plain Layer" (Per-event rendering), "Render-Mode Detection & Output Streams" (Output streams), "Spinner Lifecycle" (captured output buffered).

## cli-presentation-2-8 | approved

### Task cli-presentation-2-8: Unwound first-class event with ↩ glyph; suppress success end-of-run line on failure/abort

**Problem**: Auto-unwind is a **first-class event** — not a `StageFailed` — carrying the "what it undid" summary, with its own glyph (`↩`) and renderings in both modes. It can follow a stage failure (a push rejected → unwind) or an abort with no prior failure (the user answered `n` at the gate → full auto-unwind). Separately, the success end-of-run line is success-only: on any failure/abort run it must be **suppressed** — failure is communicated by the `✗`/`unwound` lines plus the engine-owned non-zero exit code, with no failure-flavoured closing brand line. This task adds the `Unwound` method and wires the suppression of the success end-of-run line (`RunFinished`).

**Solution**: Add an `Unwound(u Unwind)` method carrying the summary, implement its `↩`-glyph rendering in both modes, and make the success end-of-run line (`RunFinished`) suppressed once a terminal failure/abort signal has occurred — driven by `StageFailed` and/or `Unwound` having fired, not by `Warn`. The full spinner lifecycle and per-verb end-of-run shaping remain out of scope (Phase 4); this task covers only the `Unwound` rendering and the suppression of the success line.

**Outcome**: Given an `Unwound` event, pretty renders `↩ unwound  {what} — repo clean` and plain renders `unwound: {what}; repo clean`; and after a `StageFailed` or `Unwound` has fired, a subsequent `RunFinished` does **not** emit the success end-of-run line.

**Do**:
- In `internal/presenter/presenter.go`, define `type Unwind struct { Summary string }` (the "what it undid" text) and add `Unwound(u Unwind)` to the `Presenter` interface. Add the record implementation to `RecordingPresenter`.
- Plain (`internal/presenter/plain.go`) `Unwound`: render `unwound: {summary}` to `out`. Per the worked example the summary already includes the `; repo clean` tail (`unwound: removed tag v1.4.0, reset 2 commits; repo clean`) — render the engine-supplied summary verbatim; do not synthesize the "repo clean" tail (it is part of the engine's summary). Document this: the presenter does not append "repo clean" itself.
- Pretty (`internal/presenter/pretty.go`) `Unwound`: render `↩ unwound  {summary}` to `out` with the `↩` glyph styled via the lipgloss renderer (the spec colours it as the auto-unwind glyph). The worked example shows `↩ unwound  removed tag v1.4.0, reset 2 release commit(s) — repo clean` — render the engine-supplied summary verbatim.
- Implement success-line suppression: track in the presenter whether a terminal failure/abort signal has fired — set the flag in `StageFailed` and in `Unwound`. In `RunFinished`, if the flag is set, **suppress** the success end-of-run line (emit nothing for the closing brand/`done:` line). The flag is **not** set by `Warn` (a warn-only run still ends with the success line — consistent with 2-6).
  - Document the rationale: failure/abort runs end after the `✗`/`unwound`/`warn` lines; the end-of-run success line is success-only; there is no failure-flavoured closing brand line. Failure is signalled by those lines plus the engine-owned non-zero exit code (the presenter never sets the exit code).
- Unwound is narration → `out`. (It is not an error/warning; the stderr-duplication rule applies to the `FAILED` summary and warnings, not to `Unwound` — render it to `out` only, consistent with the per-event table which lists no stderr copy for auto-unwind.)
- Scope note: do **not** implement per-verb end-of-run shaping (regenerate summary without `{url}`, init terminal lines, version payload) — that is Phase 4. This task only suppresses the success line on failure/abort; the verb-specific success forms are unchanged from Phase 1's release form.

**Acceptance Criteria**:
- [ ] `Unwound` is a first-class method on the `Presenter` interface (distinct from `StageFailed`), carries the summary, and is recorded by `RecordingPresenter`.
- [ ] Pretty renders `↩ unwound  {summary}` with the `↩` glyph; plain renders `unwound: {summary}`; the engine-supplied summary (including its "repo clean" tail) is rendered verbatim and the presenter does not synthesize the tail.
- [ ] After a `StageFailed`, a subsequent `RunFinished` suppresses the success end-of-run line in both modes.
- [ ] After an `Unwound` (with no prior `StageFailed` — the abort case), a subsequent `RunFinished` suppresses the success end-of-run line in both modes.
- [ ] On a run where only `Warn` (and no `StageFailed`/`Unwound`) fired, `RunFinished` still emits the success line (warn does not suppress).
- [ ] `Unwound` renders to stdout only (no stderr duplication); the presenter sets no exit code.

**Tests**:
- `"plain renders the unwound line verbatim"` — `Unwind{Summary:"removed tag v1.4.0, reset 2 commits; repo clean"}` → `unwound: removed tag v1.4.0, reset 2 commits; repo clean`.
- `"pretty renders the unwound line with the ↩ glyph"` — assert `↩ unwound  removed tag v1.4.0, reset 2 release commit(s) — repo clean` on stdout.
- `"unwound after a stage failure suppresses the success end-of-run line"` — drive `StageFailed` → `Unwound` → `RunFinished`; assert no success closing line in either mode.
- `"unwound after an abort with no prior failure suppresses the success end-of-run line"` — drive `Unwound` → `RunFinished` (no `StageFailed`); assert no success closing line.
- `"the success end-of-run line is absent whenever Unwound fired"` — assert the closing brand/`done:` line is not emitted once `Unwound` has fired.
- `"a warn-only run still emits the success end-of-run line"` — drive `Warn` → `RunFinished`; assert the success line is present (warn does not suppress).
- `"Unwound writes to stdout only"` — assert the unwound line is on stdout and absent from stderr.

**Edge Cases**:
- **Unwound after a stage failure** — `StageFailed` then `Unwound`; both render, success line suppressed.
- **Unwound after an abort with no prior failure** — `Unwound` alone (gate-`n` abort path) still suppresses the success line.
- **Success end-of-run line absent when Unwound fired** — the closing success line is success-only and must not appear on any failure/abort run.

**Context**:
> `Unwound` is a first-class event (not a `StageFailed`) carrying the "what it undid" summary; it has its own glyph (`↩`) and renderings in both modes.
> Per-event rendering: auto-unwind → pretty `↩ unwound  {what it undid} — repo clean`; plain `unwound: {what}; repo clean`.
> Pretty failure + auto-unwind example:
> ```
>   ✗ tag/push   push rejected: remote moved
>   ↩ unwound    removed tag v1.4.0, reset 2 release commit(s) — repo clean
> ```
> Plain failure example:
> ```
> tag/push: FAILED - push rejected: remote moved
> unwound: removed tag v1.4.0, reset 2 commits; repo clean
> ```
> Failure runs end after the `✗`/`unwound`/`warn` lines: the end-of-run success line is suppressed (it is success-only). Failure/abort is communicated by those lines plus the engine-owned non-zero exit code — there is no failure-flavoured closing brand line.
> `n` ⇒ abort (full auto-unwind, owned by the engine) — the abort path produces an `Unwound` with no prior `StageFailed`.
> Status glyph: `↩` auto-unwind.
> Exit-code ownership stays with the engine/`main`, not the `Presenter`. The per-verb end-of-run shaping (regenerate/init/version) is a Phase 4 concern — this task only suppresses the success line on failure/abort.

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "The `Presenter` Seam" (Unwound first-class event), "The Pretty Layer" (failure + auto-unwind example), "The Plain Layer" (Per-event rendering, plain failure example), "Cross-Verb Rendering" (End-of-run line — success-only suppression), "Gating & `-y` Orthogonality" (abort ⇒ auto-unwind).
