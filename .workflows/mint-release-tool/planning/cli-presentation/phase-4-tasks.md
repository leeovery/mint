---
phase: 4
phase_name: Cross-Verb Rendering, Spinner Lifecycle & Width Robustness
total: 7
---

## cli-presentation-4-1 | approved

### Task cli-presentation-4-1: init renders created/skipped lines in the shared vocabulary (no gate, no brand footer)

**Problem**: `init` must render through the same `Presenter` seam as every other verb — its job is to narrate file-creation outcomes in the shared vocabulary (`✓ created .mint.toml` / `· skipped release (exists, use --force)`), proving "consistent presentation across every verb" structurally rather than with per-verb styling code. Two `init`-specific contracts must be honoured: it has **no interactive gate** (its safety is structural — non-clobbering plus `--force`, not a prompt), and it has **no release-style brand footer** (there is no versioned release, so the `created`/`skipped` lines are themselves the terminal output). Getting the footer wrong (emitting a release-style closing line for `init`) would contradict the verb-shaped end-of-run contract that 4-4 finalises.

**Solution**: Render `init`'s per-file outcomes as shared-vocabulary lines — a created outcome as a `✓`-glyph line (pretty) / `{name}: created` form (plain), and a skipped outcome as a `·`-glyph notice line (pretty) / a terse skipped notice (plain) carrying the engine-supplied reason — driven by an `init`-outcome event the engine emits per file. No new gate is introduced, and no release-style end-of-run footer is emitted for `init`.

**Outcome**: Given a sequence of `init` file outcomes (each created or skipped-with-reason), both modes render one line per outcome in the shared vocabulary — `✓ created {file}` / `· skipped {what} ({reason})` (pretty) and the terse plain equivalents — with no interactive gate drawn and no release-style brand footer at the end; `--force` overwrites narrate as created.

**Do**:
- In `internal/presenter/presenter.go`, add the `init` outcome event. Recommended: a single method `InitResult(r InitOutcome)` with `type InitOutcome struct { Action InitAction; Target string; Reason string }`, where `InitAction` is an enum (`InitCreated`, `InitSkipped`). The engine emits one `InitResult` per file/section it touched (the worked examples are `.mint.toml` created and a `release` section skipped). The presenter does **not** decide created-vs-skipped or know the `--force` semantics — the engine supplies the resolved `Action` and `Reason`. Add the record implementation to `RecordingPresenter`.
- Plain (`internal/presenter/plain.go`) `InitResult`:
  - `InitCreated` → `{target}: created` (e.g. `.mint.toml: created`) to `out`.
  - `InitSkipped` → a terse skipped notice carrying the reason, e.g. `{target}: skipped ({reason})` (mirroring the spec's `skipped release (exists, use --force)` — render the engine-supplied reason verbatim). To `out`.
  - Use only `fmt`/`io`; no ANSI/glyphs.
- Pretty (`internal/presenter/pretty.go`) `InitResult`:
  - `InitCreated` → `  ✓ created {target}` with the green `✓` glyph via the lipgloss renderer (worked example `✓ created .mint.toml`).
  - `InitSkipped` → `  · skipped {target} ({reason})` using the `·` notice glyph (worked example `· skipped release (exists, use --force)`) — note this is the neutral middot, **not** `✓`/`✗`/`⚠`/`↩`; a skip is neither success nor failure.
  - Render the engine-supplied `Reason` verbatim; do not synthesize the `(exists, use --force)` text (it is part of the engine's reason).
- No gate: `init` never calls `Prompt`. This task introduces no prompt and must not draw the review menu. (The non-clobber/`--force` decision is engine logic; the presenter only narrates the outcome the engine resolved.)
- No release-style footer: `init` must **not** emit the release brand footer / `done:` line. Confirm that the end-of-run path is not invoked for `init`, or that when invoked it produces nothing for `init` (the verb-shaped footer is finalised in 4-4; for this task, assert that an `init` run's output ends with the last `InitResult` line and carries no `🌿 released …` / `done: …` line).
- `--force` overwrite is narrated as **created**: when the engine overwrites under `--force` it emits `InitCreated` (not skipped). The presenter has no `--force` knowledge — it renders whatever `Action` the engine supplies. Add a test driving an `InitCreated` that represents the `--force` overwrite case and assert it renders as a created line.

**Acceptance Criteria**:
- [ ] `InitResult` (or equivalent) is on the `Presenter` interface, carries the engine-resolved action + target + reason, and is recorded by `RecordingPresenter`.
- [ ] A created outcome renders `✓ created {target}` (pretty) / `{target}: created` (plain).
- [ ] A skipped outcome renders `· skipped {target} ({reason})` (pretty, middot glyph) / `{target}: skipped ({reason})` (plain), with the engine-supplied reason verbatim.
- [ ] `init` draws no interactive gate (no `Prompt` call, no review menu).
- [ ] `init` emits **no** release-style brand footer / `done:` line — the run ends with the last outcome line.
- [ ] A `--force` overwrite is narrated as a created line (the engine supplies `InitCreated`; the presenter does not special-case `--force`).
- [ ] Mixed runs render created and skipped lines independently, in the order the engine emitted them.

**Tests**:
- `"a created outcome renders the created line"` — `InitOutcome{Action:InitCreated, Target:".mint.toml"}` → pretty `✓ created .mint.toml`, plain `.mint.toml: created`.
- `"a skipped outcome renders the skipped notice with the reason"` — `InitOutcome{Action:InitSkipped, Target:"release", Reason:"exists, use --force"}` → pretty `· skipped release (exists, use --force)`, plain `release: skipped (exists, use --force)`.
- `"all-created run renders only created lines"` — two `InitCreated` outcomes; assert two created lines and no skipped notice.
- `"all-skipped run renders only skipped notices"` — two `InitSkipped` outcomes; assert two skipped lines and no created line.
- `"a mixed run renders created and skipped in emit order"` — created then skipped; assert both lines in that order.
- `"a --force overwrite narrates as created"` — `InitCreated` representing an overwrite renders as a created line (no skipped, no force-specific text beyond what the engine supplied).
- `"init emits no release-style footer"` — drive an `init` run; assert the output contains no `🌿 released …` and no `done: …` line.
- `"init draws no gate"` — drive an `init` run; assert `Prompt` is never called and no `Continue? ›`/menu lines appear.

**Edge Cases**:
- **All created** — every outcome is `InitCreated`; only created lines render.
- **All skipped (exist)** — every outcome is `InitSkipped` with a reason; only skipped notices render.
- **Mixed created + skipped** — both kinds render independently in emit order.
- **`--force` overwrite narrated as created** — the engine emits `InitCreated` for an overwrite; the presenter renders a created line with no `--force` special-casing.
- **No release-style footer emitted** — the run terminates on the last outcome line; no brand footer / `done:` line.

**Context**:
> `init` — process narration in the same vocabulary: `✓ created .mint.toml` / `· skipped release (exists, use --force)`. No gate (non-clobbering).
> Gate inventory: `init` — interactive gate? No — non-clobbering (skips existing with a notice; `--force` to overwrite). Under `-y`: n/a. `init`'s safety is structural (non-clobber + `--force`), not a prompt — which is why it never needed `-y`.
> `init` — has no versioned release — its `created`/`skipped` lines are themselves the terminal output; no release-style brand footer is required.
> Applies to every verb. `release`, `regenerate`, `init`, `version` (and future `commit`) all emit through the same `Presenter`. This is how "consistent presentation across all verbs" is met — structurally, via one interface, not per-verb styling code.
> Event payload principle: the engine supplies, in each event's payload, every datum the renderings consume — the presenter never re-derives engine knowledge (so created-vs-skipped and the `--force` reason come from the engine, not the presenter).

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "Cross-Verb Rendering" (`init`), "Gating & `-y` Orthogonality" (gate inventory — `init` no gate), "Cross-Verb Rendering" (End-of-run line — `init` has no footer), "The `Presenter` Seam" (Event payload principle).

## cli-presentation-4-2 | approved

### Task cli-presentation-4-2: regenerate per-version narration with verb-shaped closing summary (omits url; --all oldest→newest, one block each)

**Problem**: `regenerate` reuses the **same** stage/notes/gate vocabulary as `release`, but narrated **per version**, and `--all` runs **oldest→newest** as one narrated block each. Because `regenerate` does **not publish**, its closing summary is verb-shaped: it omits the `{url}` field that the release footer carries. With `--all`, each version is its own narrated block (stages + notes + gate) and the closing line summarises the **set**. This is the verb where the per-version block boundary, the oldest→newest ordering, the url-omitting closing summary, and the reuse-confirm-vs-fresh-notes gate selection (Phase 3) all come together — proving the shared vocabulary generalises to a multi-block run without a parallel rendering path.

**Solution**: Render `regenerate` by reusing the existing stage/`ShowNotes`/`Prompt` events per version (the engine drives one block per version, in oldest→newest order, choosing the notes-review gate for freshly-generated notes or the reuse-confirm gate for reused notes), and finalise the `regenerate` closing summary — a success-shaped end-of-run line in the shared vocabulary that **omits** the `{url}` field and summarises the set under `--all`. The presenter renders whatever block sequence and closing summary the engine drives; ordering and reuse-vs-fresh are engine knowledge supplied through the events.

**Outcome**: Given an engine-driven `regenerate` run (one block per version, oldest→newest), the presenter renders each version's stages + notes + gate using the shared vocabulary, then a closing summary line that omits the `{url}` field (pretty: a brand-style summary without the URL; plain: a `done:`-style line without the URL) and summarises the set when `--all` produced multiple blocks.

**Do**:
- Confirm `regenerate` needs **no new rendering events** for the per-version blocks — each block reuses `RunStarted`/`StageStarted`/`StageSucceeded`/`StageFailed`/`ShowPlan`/`ShowNotes`/`Prompt` exactly as `release` does. The engine emits one block per version; the presenter renders them linearly in emit order (which the engine has ordered oldest→newest). Do **not** add per-version ordering logic to the presenter — the engine supplies the order. Add a doc note that block ordering is engine-owned.
- Reuse the Phase 3 gate selection per block: a **freshly-generated** notes block uses `NotesReviewGate()` (four-choice `y`/`n`/`e`/`r`); a **reused-notes** block uses `ReuseConfirmGate()` (two-choice `y`/`n`). The presenter renders whichever gate the engine passes to `Prompt` — it does not decide reuse-vs-fresh. Add tests that drive both gate variants within `regenerate` blocks and assert the correct menu renders (four-choice vs two-choice).
- Finalise the `regenerate` closing summary as the verb-shaped end-of-run line (this is the `regenerate` arm of the shared verb-shaped footer that 4-4 generalises; coordinate so the two tasks describe the same mechanism — this task owns the `regenerate` form, 4-4 owns the dispatch and the release/init arms):
  - The end-of-run payload must carry the verb shape so the presenter renders the correct form **without** a URL field for `regenerate`. Recommended: extend the end-of-run payload (`RunFinished`/`RunSummary`) with a `Verb`/shape discriminator and make `URL` optional/empty for `regenerate`.
  - Plain (`plain.go`): a `done:`-style summary **without** the URL — e.g. `done: {project} {versions}` (no trailing URL). Under `--all` the summary names the set (e.g. the count or the version range the engine supplies); render the engine-supplied summary text — do not compute the version set in the presenter.
  - Pretty (`pretty.go`): a brand-style closing summary in the same vocabulary **without** the `· {url}` tail (the release footer is `🌿 released {project} v{X} · {url}`; the regenerate close is the same shape minus the ` · {url}`). Render the engine-supplied summary fields.
  - The `{url}` field is **omitted entirely** — not rendered empty, not rendered as a dangling ` · `. Assert no URL and no dangling separator.
- Honour the Phase 2 suppression rule: on a failed/aborted `regenerate` block the success closing summary is suppressed (the suppression flag from cli-presentation-2-8 already drives this). Do not re-implement suppression; just confirm the regenerate closing summary is the success-only form gated by that flag.
- `--all` with a **single** version is still one block plus the set-summary closing line (the closing line summarises a one-element set without breaking) — assert it does not produce a release-style single-version footer with a URL.

**Acceptance Criteria**:
- [ ] `regenerate` per-version blocks reuse the existing stage/notes/gate events (no new per-block rendering events); the presenter renders blocks in engine emit order.
- [ ] `--all` renders one block per version in the order the engine emitted (oldest→newest) — the presenter does not reorder.
- [ ] A freshly-generated block renders the four-choice notes-review gate; a reused-notes block renders the two-choice reuse confirm — driven by the gate the engine passes.
- [ ] The closing summary omits the `{url}` field entirely (no URL, no dangling ` · `/trailing separator) in both modes.
- [ ] Under `--all` the closing summary summarises the set (engine-supplied summary text), including the single-version `--all` case.
- [ ] A failed/aborted `regenerate` suppresses the success closing summary (reusing the Phase 2 suppression flag).

**Tests**:
- `"a single-version regenerate renders one block then a url-less closing summary"` — one block of stages + notes + gate, then a closing summary with no URL.
- `"--all renders one block per version in oldest→newest emit order"` — three engine-emitted blocks (oldest→newest); assert three blocks rendered in that order.
- `"--all with a single version renders one block and a set summary (no release footer)"` — one block, then the set-summary closing line; assert no `· {url}` and no release-style single-version footer.
- `"the closing summary omits the url field with no dangling separator"` — assert the regenerate close has no URL and no trailing ` · ` in both modes.
- `"a fresh-notes block renders the four-choice gate"` — block driven with `NotesReviewGate()`; assert `y`/`n`/`e`/`r` menu.
- `"a reuse-notes block renders the two-choice confirm"` — block driven with `ReuseConfirmGate()`; assert only `y`/`n`.
- `"a failed regenerate block suppresses the closing summary"` — drive a `StageFailed`/`Unwound` in a block then `RunFinished`; assert no closing summary line.

**Edge Cases**:
- **Single version** — one block plus a url-less closing summary.
- **`--all` multiple versions in oldest→newest order** — one block each in the engine's emit order; the presenter does not reorder.
- **`--all` single version** — one block plus a set-summary closing line; not a release-style single-version footer.
- **Closing summary omits url field** — the `{url}` field is absent entirely, with no dangling separator.
- **Reuse-confirm vs fresh-notes path per block** — the gate variant per block follows the gate the engine passes (two-choice reuse confirm vs four-choice notes review).

**Context**:
> `regenerate` — same stage/notes/gate vocabulary as `release`, narrated per version (`--all` runs oldest→newest, one block each).
> `regenerate` does not publish and has no release URL — it emits a closing summary in the same vocabulary without the `{url}` field; with `--all` (oldest→newest), each version is its own narrated block and the closing line summarises the set.
> Gate inventory: `regenerate` — Yes — interactive source + target prompts, then the notes-review gate (fresh) / a simple confirm (reuse).
> Reuse confirm (regenerate reusing existing notes) — a reduced two-choice `y`/`n` confirm rendered in the same `Continue?` vocabulary (no `e`/`r`); default-yes.
> End-of-run line — success-shaped and verb-shaped. The `🌿 released {project} v{X} · {url}` / `done: {project} v{X} {url}` lines are the release-success form. The closing line follows the verb's payload.
> Failure runs end after the `✗`/`unwound`/`warn` lines: the end-of-run success line is suppressed (it is success-only). (Phase 2 wired the suppression flag.)

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "Cross-Verb Rendering" (`regenerate`; End-of-run line — verb-shaped, regenerate without url), "Gating & `-y` Orthogonality" (gate inventory; reuse confirm).

## cli-presentation-4-3 | approved

### Task cli-presentation-4-3: version payload exception — plain bare value, pretty dressed

**Problem**: `version` is the **one payload verb**: its output is a *value*, not narration. This is the deliberate exception to "narration is the product" — `version` actually has a payload, so the **bare value is the floor** and styling is additive only in pretty. **Plain prints the bare value** (`1.4.0`) so `$(mint version)` and scripts consume it cleanly — no narration prefix, no glyph, no trailing decoration, nothing that would pollute command substitution. **Pretty may dress it** (`🌿 mint v1.4.0`). Getting plain wrong (emitting a glyph, a `version:` prefix, or extra bytes) would break shell consumers, so the "no extra bytes" contract for plain is the load-bearing requirement here.

**Solution**: Render `version` via a dedicated payload event: plain writes the bare value to `out` with a single trailing newline and nothing else (no prefix, no glyph, no decoration), so `$(mint version)` captures exactly the value; pretty dresses it as `🌿 mint v{value}` via the lipgloss renderer. This is the only event whose plain form is a raw payload rather than `key: value` narration.

**Outcome**: Given a `version` payload, plain emits exactly the bare value plus a trailing newline (no `version:` prefix, no glyph, no ANSI, no extra lines), and pretty emits a dressed `🌿 mint v{value}` line; `$(mint version)` in plain captures the bare value with no extraneous bytes.

**Do**:
- In `internal/presenter/presenter.go`, add the payload event. Recommended: `ShowVersion(v Version)` with `type Version struct { Value string }` (the resolved version value, e.g. `1.4.0`). Add the record implementation to `RecordingPresenter`. Document that this is the **payload exception** — the one event whose plain output is a raw value, not `key: value` narration.
- Plain (`internal/presenter/plain.go`) `ShowVersion`: write **exactly** `{value}\n` to `out` — the bare value followed by a single newline. No `version:` prefix, no glyph, no ANSI, no leading/trailing spaces, no second line. This is what `$(mint version)` consumes (command substitution strips the single trailing newline, leaving exactly the value).
- Pretty (`internal/presenter/pretty.go`) `ShowVersion`: write a dressed line, e.g. `🌿 mint v{value}` (worked spec form), via the lipgloss renderer (the leaf glyph ties to the brand). Pretty styling is **additive** — the value must still be present and legible; colour auto-downgrade applies as elsewhere.
- The value is narration's payload → `out` only (never stderr; `version` is not an error/warning).
- `version` has **no gate** and **no release-style end-of-run footer** — it prints its value and that is the terminal output (consistent with the gate inventory and the verb-shaped footer rules; assert no footer/`done:` line follows the version value).
- Be precise about the "no extra bytes" contract for plain: add a test that captures the full plain `out` buffer and asserts it equals exactly `value + "\n"` (byte-for-byte) — no ANSI escape (ESC byte), no `🌿`, no `v` prefix, no `version:` prefix, no extra whitespace or lines. This is the contract shell consumers depend on.

**Acceptance Criteria**:
- [ ] `ShowVersion` (or equivalent) is on the `Presenter` interface and recorded by `RecordingPresenter`.
- [ ] Plain emits **exactly** the bare value plus a single trailing newline — no prefix, no glyph, no ANSI, no extra lines/whitespace.
- [ ] Pretty emits a dressed form (`🌿 mint v{value}`) with the value present; styling is additive only.
- [ ] The plain output is suitable for `$(mint version)` — capturing it yields exactly the value (no extraneous bytes).
- [ ] `version` draws no gate and emits no release-style footer / `done:` line.
- [ ] The value is written to stdout only (absent from stderr).

**Tests**:
- `"plain emits the bare value only"` — `ShowVersion{Value:"1.4.0"}` plain `out` equals exactly `"1.4.0\n"` (byte-for-byte); assert no ESC byte, no `🌿`, no `version:` prefix.
- `"pretty emits the dressed form"` — pretty renders `🌿 mint v1.4.0` (value present, brand glyph present).
- `"command substitution consumes the value cleanly"` — simulate `$(…)` by trimming a single trailing newline from the plain output; assert the result equals `1.4.0` with no extra bytes.
- `"version emits no footer"` — drive a `version` run; assert no `🌿 released …`/`done: …` line follows the value.
- `"version draws no gate"` — assert `Prompt` is never called for `version`.
- `"version writes to stdout only"` — assert stderr is empty after `ShowVersion`.

**Edge Cases**:
- **Plain emits bare value only (no narration/glyph/trailing decoration)** — exactly `value + "\n"`; nothing else.
- **Pretty dressed form** — `🌿 mint v{value}`; styling additive, value still present.
- **Clean command-substitution consumption (no extra bytes)** — `$(mint version)` yields exactly the value; the plain output carries no ANSI, prefix, or extra lines that would pollute the captured value.

**Context**:
> `version` — the one payload verb: its output is a value, not narration. Plain prints the bare value (`1.4.0`) so `$(mint version)`/scripts consume it cleanly; pretty may dress it (`🌿 mint v1.4.0`). This is the deliberate exception to "narration is the product" — `version` actually has a payload, so the bare value is the floor and styling is additive only in pretty.
> Run narration → stdout — stages, the plan, the notes preview, the final summary, and `mint version`'s value. `mint` has no separate data payload, so the narration is its stdout output. (For `version`, the value is the stdout payload.)
> Gate inventory: `version` — interactive gate? No — prints its value.
> `version` has no versioned release footer (it is not a release verb; the verb-shaped end-of-run line is release/regenerate only).

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "Cross-Verb Rendering" (`version` — the one payload verb), "Render-Mode Detection & Output Streams" (Output streams — `mint version`'s value), "Gating & `-y` Orthogonality" (gate inventory — `version` no gate).

## cli-presentation-4-4 | approved

### Task cli-presentation-4-4: Verb-shaped, success-only end-of-run line (release footer with url; regenerate without url; suppressed on failure)

**Problem**: The end-of-run line is **success-shaped and verb-shaped**: the `🌿 released {project} v{X} · {url}` / `done: {project} v{X} {url}` lines are the *release-success* form, but the closing line must follow the verb's payload — `release` carries the `{url}`, `regenerate` omits the `{url}` (it does not publish), and `init`/`version` have no footer at all. It is **success-only**: on any failure/abort run the success line is **suppressed** (Phase 2 wired the suppression flag via cli-presentation-2-8; this task **extends** that flag to verb-shaping). This is the task that dispatches the end-of-run rendering by verb so the right closing form (or none) is emitted, finishing the "verb-shaped end-of-run" contract that 4-1/4-2/4-3 each touched from their verb's side.

**Solution**: Generalise the end-of-run rendering into a verb-shaped dispatch: the end-of-run payload carries a verb/shape discriminator (and the optional `{url}`), and `RunFinished` renders the release footer **with** `{url}` for `release`, the regenerate summary **without** `{url}` for `regenerate`, and **nothing** for `init`/`version` — all gated by the existing success-only suppression flag so failure/abort runs emit no closing line. The presenter never re-derives the verb; the engine supplies the shape and fields.

**Outcome**: `RunFinished` renders the release brand footer with `{url}` for a successful `release`, the regenerate closing summary without `{url}` for a successful `regenerate`, no footer for `init`/`version`, and **nothing** when the success-suppression flag is set (any failure/abort run) — across both modes.

**Do**:
- Extend the end-of-run payload (`RunFinished`/`RunSummary` from Phase 1/2) with the verb shape and optional URL. Recommended: `type RunSummary struct { Verb Verb; Project string; Version string; URL string; SetSummary string }` where `Verb` discriminates `release`/`regenerate`/`init`/`version` (or, more directly, a shape enum: `FooterRelease`/`FooterRegenerate`/`FooterNone`). The engine supplies the shape and the fields; the presenter never infers the verb. Update `RecordingPresenter` to capture the shape.
- Implement the verb-shaped dispatch in both presenters' `RunFinished`, gated **first** by the success-suppression flag (from cli-presentation-2-8):
  1. If the suppression flag is set (a `StageFailed`/`Unwound` fired) → render **nothing** (success-only; no failure-flavoured closing line). This must hold for **every** verb shape.
  2. Else dispatch on the shape:
     - `release` → pretty `🌿 released {project} v{version} · {url}`; plain `done: {project} v{version} {url}`.
     - `regenerate` → the url-less closing summary owned by cli-presentation-4-2 (pretty brand-style summary minus ` · {url}`; plain `done:`-style line without the URL; `SetSummary` for `--all`). Coordinate with 4-2 so both describe the same single mechanism — this task owns the **dispatch**, 4-2 owns the regenerate **content/`--all` set summary**.
     - `init` → **no** footer (renders nothing; the `created`/`skipped` lines from 4-1 are terminal).
     - `version` → **no** footer (the value from 4-3 is terminal).
- The `{url}` is rendered **only** for the `release` shape; for `regenerate` it is omitted entirely (no dangling ` · `); for `init`/`version` no closing line at all. Assert each shape's exact form.
- Do **not** re-implement the suppression detection — reuse the flag set by `StageFailed`/`Unwound` in cli-presentation-2-8 (and not set by `Warn`). This task **reads** that flag and adds the verb-shaped dispatch on the success path. Add a doc note that suppression precedes shaping (a failed `release` emits no footer even though it carries a URL).
- The end-of-run line is narration → `out` (no stderr copy; it is a success line, not an error/warning).
- Add cross-shape tests: a successful release (footer with url), a successful regenerate (summary without url), init/version (no footer), and a failed run of release **and** regenerate (no footer despite carrying url/shape) to lock that suppression wins over shaping.

**Acceptance Criteria**:
- [ ] The end-of-run payload carries a verb/shape discriminator and an optional URL; `RecordingPresenter` captures the shape.
- [ ] A successful `release` renders `🌿 released {project} v{X} · {url}` (pretty) / `done: {project} v{X} {url}` (plain) — with the URL.
- [ ] A successful `regenerate` renders the closing summary **without** the URL (no dangling ` · `) in both modes.
- [ ] `init` and `version` render **no** end-of-run footer.
- [ ] When the success-suppression flag is set (failure or abort), `RunFinished` renders **nothing** for every verb shape — suppression precedes shaping.
- [ ] The presenter never re-derives the verb — the shape comes from the payload; `Warn` alone does not suppress the footer.

**Tests**:
- `"release footer renders with the url"` — `release` shape with `URL` set → pretty `🌿 released acme v1.4.0 · {url}`, plain `done: acme v1.4.0 {url}`.
- `"regenerate close renders without the url"` — `regenerate` shape → closing summary with no URL and no dangling ` · ` in both modes.
- `"init has no footer"` — `init` shape → `RunFinished` renders nothing.
- `"version has no footer"` — `version` shape → `RunFinished` renders nothing.
- `"a failure run suppresses the success line for release"` — `release` shape with the suppression flag set (prior `StageFailed`) → no footer despite the URL.
- `"an abort run suppresses the success line for regenerate"` — `regenerate` shape with the suppression flag set (prior `Unwound`, gate-`n` abort) → no closing summary.
- `"a warn-only run still emits the verb-shaped footer"` — `Warn` then a successful `release` `RunFinished` → footer present (warn does not suppress; reuses cli-presentation-2-6/2-8 semantics).

**Edge Cases**:
- **Release footer with url** — the `release` shape renders the URL.
- **Regenerate close without url** — the `regenerate` shape omits the URL entirely (no dangling separator).
- **init has no footer** — the `init` shape renders nothing at end-of-run.
- **Failure run suppresses success line** — suppression flag set → nothing rendered, regardless of shape/URL.
- **Abort run suppresses success line** — gate-`n` abort (`Unwound` without a prior `StageFailed`) → nothing rendered.

**Context**:
> End-of-run line — success-shaped and verb-shaped. The `🌿 released {project} v{X} · {url}` / `done: {project} v{X} {url}` lines are the release-success form. The closing line follows the verb's payload:
> - `regenerate` does not publish and has no release URL — it emits a closing summary in the same vocabulary without the `{url}` field; with `--all`, the closing line summarises the set.
> - `init` has no versioned release — its `created`/`skipped` lines are themselves the terminal output; no release-style brand footer is required.
> - Failure runs end after the `✗`/`unwound`/`warn` lines: the end-of-run success line is suppressed (it is success-only). Failure/abort is communicated by those lines plus the engine-owned non-zero exit code — there is no failure-flavoured closing brand line.
> Brand lines: bottom `🌿 released {project} v{X} · {url}`.
> Per-event rendering: end of run → pretty `🌿 released {project} v{X} · {url}`; plain `done: {project} v{X} {url}`.
> cli-presentation-2-8 wired the success-line suppression flag (set by `StageFailed`/`Unwound`, not by `Warn`); this task extends it to verb-shaping — suppression precedes shaping.

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "Cross-Verb Rendering" (End-of-run line — success-shaped and verb-shaped), "The Pretty Layer" (Brand lines), "The Plain Layer" (Per-event rendering, end of run).

## cli-presentation-4-5 | approved

### Task cli-presentation-4-5: Pretty spinner lifecycle — single spinner started on StageStarted, replaced in place by ✓/✗; output buffered, printed below ✗

**Problem**: Phases 1–2 rendered the **static** pretty stage line and deferred the spinner *animation* and lifecycle to Phase 4. The spinner is a `pretty`-only concern owned inside the pretty presenter: **one spinner at a time on the current stage line**, started on `StageStarted` and **replaced in place** by the `✓`/`✗` completion line. Underlying command output (git/`claude`/`gh` chatter) is **buffered, not streamed through the spinner line** (so the animation can't be corrupted), and on failure the captured output is printed **below the `✗`** line. `plain` never animates. Wiring this with a lightweight standalone spinner (explicit `Start()`/`Stop()` mapped to `StageStarted`/`StageSucceeded`/`StageFailed`) — **not** Bubble Tea, no alt-screen — is the pretty-mode polish that closes the spinner contract.

**Solution**: Integrate a lightweight standalone spinner (e.g. `briandowns/spinner` or `huh/spinner`) into the pretty presenter: `StageStarted` starts a single spinner on the current stage line (braille frames), and `StageSucceeded`/`StageFailed` stop it and replace it in place with the `✓`/`✗` completion line (reusing the Phase 2 static line rendering). The captured underlying output is buffered (never streamed through the spinner) and printed below the `✗` line on failure (reusing the Phase 2 `StageFailed` body rendering). `plain` remains animation-free (no-op spinner), and short non-spinner stages are unaffected.

**Outcome**: In pretty mode a single spinner runs on the current stage line between `StageStarted` and completion, is replaced in place by `✓` on success or `✗` on failure (with the buffered captured output printed below the `✗`), only one spinner is active at a time across sequential stages, and `plain` emits no animation frames while short stages render exactly as before.

**Do**:
- Add the spinner dependency: a lightweight standalone package with explicit `Start()`/`Stop()` (e.g. `briandowns/spinner` or `huh/spinner` — the exact package is an implementation detail; the seam doesn't care). It must **not** be Bubble Tea, must **not** use an alt-screen, and must **not** own the event loop — the engine drives and calls the presenter. `plain` pulls in **no** UI/spinner library (token-efficiency; the plain presenter's spinner methods are no-ops).
- In `internal/presenter/pretty.go`, hold a single spinner handle on the presenter (one at a time). Map the lifecycle:
  - `StageStarted(s StageStart)`: start the spinner on the current stage line (braille frames `⠋⠙⠹…`, per the spec) showing the dim start text (e.g. `⠋ notes  generating with claude…`). Replace the Phase 1/2 static `StageStarted` dim line with the spinner-driven line in pretty. Only **one** spinner is started; if one is somehow active, stop it first (defensive — there should never be two).
  - `StageSucceeded(s StageSuccess)`: **stop** the spinner and replace it **in place** with the Phase 2 static `✓ {stage}  {detail} ({elapsed})` line (reuse the existing 2-3 rendering for the completion line — do not duplicate it). "In place" = the spinner line becomes the `✓` line (the spinner library's stop-and-replace, or stop + clear-line + print the final line; no alt-screen, no full-screen redraw).
  - `StageFailed(s StageFailure)`: **stop** the spinner and replace it in place with the Phase 2 static `✗ {stage}  {message}` line, then print the **buffered captured output below the `✗`** (reuse the 2-7 `StageFailed` captured-body rendering). The captured output is buffered by `mint` (engine-supplied in the payload) — it is **never** streamed through the spinner line.
- Enforce **one spinner at a time across sequential stages**: when stage A completes its spinner is stopped before stage B's `StageStarted` starts a new one — never two concurrent spinners. Add a test driving `StageStarted(A)`→`StageSucceeded(A)`→`StageStarted(B)`→`StageSucceeded(B)` and asserting at most one spinner is active at any time.
- **Short, non-spinner stages unaffected**: a short stage that the engine renders with only a `StageSucceeded` (no meaningful `StageStarted` spinner, e.g. a fast `version`/`preflight` line in the worked example) still renders its static `✓` line as in Phase 2. Decide and document the trigger for starting a spinner — recommended: start the spinner on `StageStarted` for **blocking** stages (`s.Blocking == true`, the same flag from cli-presentation-2-1 that gates the plain start line), so short stages that emit no blocking `StageStarted` get no spinner and render only their completion line. This keeps the worked example's short stages spinner-free while `notes`/`prep` spin.
- `plain` (`internal/presenter/plain.go`): the spinner lifecycle methods are **no-ops** for animation — `StageStarted`/`StageSucceeded`/`StageFailed` keep their Phase 2 terse-line behaviour and emit **no** animation frames, no `\r`, no ANSI. Add a test asserting plain output contains no spinner frame glyphs (`⠋⠙⠹…`), no carriage returns, and no ESC bytes from animation.
- Keep the captured-output buffering contract: the underlying command output is **not** streamed through the spinner; it is the engine-supplied `Output` on `StageFailure`, printed below `✗` only on failure (on success it is not printed — consistent with Phase 2).
- The spinner is part of the narration on **stdout** (the spec: "the spinner is part of the narration on stdout"). Do not write spinner frames to stderr.

**Acceptance Criteria**:
- [ ] In pretty mode a single spinner starts on a blocking `StageStarted` and is replaced **in place** by the `✓` line on `StageSucceeded`.
- [ ] On `StageFailed` the spinner is replaced in place by the `✗` line and the buffered captured output is printed **below** the `✗` (and only on failure).
- [ ] Only **one** spinner is active at a time across sequential stages — no two concurrent spinners.
- [ ] The spinner uses a lightweight standalone library with explicit `Start()`/`Stop()` — not Bubble Tea, no alt-screen, no full-screen redraw.
- [ ] `plain` emits **no** animation frames, no `\r`, no animation ANSI, and pulls in no spinner/UI library.
- [ ] A short, non-spinner stage (no blocking `StageStarted`) renders only its static completion line, unaffected by the spinner work.
- [ ] The captured underlying output is buffered (never streamed through the spinner) and printed below `✗` only on failure; spinner frames go to stdout, not stderr.

**Tests**:
- `"the spinner is replaced by ✓ on success"` — pretty `StageStarted{Blocking:true}`→`StageSucceeded`; assert the final stage line is the `✓` completion line (spinner stopped, replaced in place).
- `"the spinner is replaced by ✗ on failure"` — pretty `StageStarted{Blocking:true}`→`StageFailed`; assert the `✗` line replaces the spinner.
- `"captured output is printed below ✗ only"` — `StageFailed{Output:"…chatter…"}`; assert the captured body appears below the `✗` line; on a successful stage the captured output is not printed.
- `"one spinner at a time across sequential stages"` — drive two blocking stages in sequence; assert never more than one spinner active (spy on Start/Stop ordering: Start A, Stop A, Start B, Stop B — no Start B before Stop A).
- `"plain emits no animation frames"` — plain `StageStarted{Blocking:true}`→`StageSucceeded`; assert output has no `⠋⠙⠹…` frames, no `\r`, no ESC bytes from animation.
- `"a short non-spinner stage is unaffected"` — pretty `StageSucceeded` for a short stage with no blocking `StageStarted`; assert it renders only the static `✓` line and starts no spinner.
- `"spinner frames go to stdout not stderr"` — assert stderr carries no spinner frames.

**Edge Cases**:
- **Spinner replaced by ✓ on success** — the spinner line becomes the `✓` completion line in place.
- **Spinner replaced by ✗ on failure** — the spinner line becomes the `✗` line in place.
- **Captured output below ✗ only** — buffered output prints below the `✗` on failure; never on success, never streamed through the spinner.
- **One spinner at a time across sequential stages** — no two concurrent spinners; each stage's spinner stops before the next starts.
- **Plain emits no animation frames** — plain remains a single terse line per transition, no frames/`\r`/ANSI.
- **Short non-spinner stage unaffected** — a short stage renders only its static completion line.

**Context**:
> Spinner Lifecycle (pretty only). Spinners are a `pretty`-only concern, owned inside the pretty presenter. `plain` never animates — a stage emits exactly one line on its transition.
> One spinner at a time, on the current stage line. Starts on `StageStarted`, replaced in place by the `✓`/`✗` line on completion. Braille frames (`⠋⠙⠹…`). The spinner is part of the narration on stdout.
> Underlying command output (git/claude/gh chatter) is captured by `mint`, not streamed through the spinner line, so the animation can't be corrupted. On failure, `mint` prints the captured output below the `✗` line.
> A lightweight standalone spinner for stage progress — e.g. `briandowns/spinner` (explicit `Start()`/`Stop()`, maps 1:1 to `StageStarted`/`StageSucceeded`) or charm's `huh/spinner`. The exact package is an implementation detail; the seam doesn't care.
> NOT Bubble Tea / no alt-screen / no full-screen TUI. Print-style linear narration only.
> `plain` mode pulls in no UI library — just `fmt` lines.
> Worked pretty example shows `⠋ notes  generating with claude…` then `✓ notes  generated (1.1s)` — the spinner line replaced by the `✓` line. Short stages (`version`/`preflight`) show only the `✓` line.
> cli-presentation-2-1 established the `Blocking` flag; cli-presentation-2-3 rendered the static `✓` line; cli-presentation-2-7 rendered the captured-output body — this task reuses those for the completion/failure lines and adds only the spinner animation/lifecycle.

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "Spinner Lifecycle (pretty only)", "Library Selection" (lightweight standalone spinner; not Bubble Tea), "The Pretty Layer" (worked example, status glyphs/spinner frames), "The `Presenter` Seam" (spinners pretty-only; StageFailed captured output).

## cli-presentation-4-6 | approved

### Task cli-presentation-4-6: Spinner stop/resume around the engine-driven `$EDITOR` hand-off

**Problem**: When the user chooses `e` at the review gate, the **engine** invokes `$EDITOR`, which **takes over the terminal**. The pretty spinner must be **stopped before** handing off (so it does not animate over or corrupt the editor's screen) and **resumed after** the editor returns. Crucially this is **engine-driven**: per the Phase 3 contract the engine owns the `e`/`r` re-entry loop and the hand-off, so the presenter must expose explicit stop/resume hooks the engine calls around `$EDITOR` — the presenter does not detect the editor itself. This must be safe across **repeated edit passes** (each pass stops and resumes cleanly), a **no-op when no spinner is active**, and a **no-op in plain** (which never animates).

**Solution**: Expose explicit presenter hooks — e.g. `SuspendSpinner()` / `ResumeSpinner()` (or `PauseAnimation()`/`ResumeAnimation()`) — that the engine calls around the `$EDITOR` hand-off: `SuspendSpinner` stops the active pretty spinner (releasing the terminal), `ResumeSpinner` restarts it on the same stage line afterward. Both are no-ops when no spinner is active and no-ops in plain. Repeated edit passes each call the pair cleanly without leaking spinner state.

**Outcome**: The pretty presenter exposes engine-callable stop/resume hooks; calling stop before `$EDITOR` releases the terminal (spinner stopped, no frames during the editor session) and resume after restarts the spinner; both are safe no-ops when no spinner is active and in plain; repeated edit passes each stop/resume without leaking state or leaving an orphaned spinner.

**Do**:
- Add the stop/resume hooks to the `Presenter` interface so the engine can call them around the hand-off. Recommended: `SuspendSpinner()` and `ResumeSpinner()` (document the exact names). They are part of the seam because the engine — which owns the `e`/`r` re-entry loop and the `$EDITOR` invocation (cli-presentation-3-8) — drives the hand-off. Add no-op implementations to `RecordingPresenter`.
- Pretty (`internal/presenter/pretty.go`):
  - `SuspendSpinner()`: if a spinner is currently active (the single handle from cli-presentation-4-5), `Stop()` it and remember the suspended stage context (the stage line it was animating) so `ResumeSpinner` can restart it. Stopping releases the terminal so `$EDITOR` can take it over cleanly (no animation over the editor). If **no** spinner is active, do nothing.
  - `ResumeSpinner()`: if a spinner was suspended, restart it on the same stage line (same frames/text). If none was suspended, do nothing.
  - The suspend/resume must not corrupt the linear scroll (no alt-screen, no clear) — it is a plain stop then start, consistent with the print-style narration.
- Plain (`internal/presenter/plain.go`): `SuspendSpinner()`/`ResumeSpinner()` are **no-ops** (plain never animates; there is nothing to stop or resume). Add a test asserting they produce no output and do not error in plain.
- No active spinner at hand-off: if the engine calls `SuspendSpinner` when no spinner is running (e.g. the edit was triggered from a state where the spinner had already completed), it must be a safe no-op — and the paired `ResumeSpinner` must also no-op (nothing was suspended). Add a test for the no-active-spinner case.
- Repeated edit passes: the engine's `e`/`r` loop (cli-presentation-3-8) may invoke `$EDITOR` multiple times across passes. Each pass calls `SuspendSpinner` then `ResumeSpinner`; the pair must work cleanly every time with no leaked/duplicated spinner (after N suspend/resume cycles there is still at most one spinner, matching the one-at-a-time rule from cli-presentation-4-5). Add a test driving multiple suspend/resume cycles and asserting no orphaned or duplicated spinner.
- This task **does not** invoke `$EDITOR` — that is the engine's job (cli-presentation-3-8 locked the render-only `Prompt` contract; the presenter never calls `$EDITOR`/`claude`). The hooks are called **by the engine** around its own hand-off. Add a doc note reaffirming the presenter still does not invoke the editor; it only suspends/resumes its own animation on engine command.
- Coordinate with cli-presentation-4-5: the single spinner handle and the one-at-a-time invariant established there are what suspend/resume operate on. Do not introduce a second spinner.

**Acceptance Criteria**:
- [ ] The presenter exposes engine-callable `SuspendSpinner()`/`ResumeSpinner()` hooks (on the interface; recorded/no-op in `RecordingPresenter`).
- [ ] In pretty, `SuspendSpinner` stops the active spinner (releasing the terminal) and `ResumeSpinner` restarts it on the same stage line.
- [ ] No frames are emitted between suspend and resume (the editor session is animation-free).
- [ ] When no spinner is active, both hooks are safe no-ops.
- [ ] In plain, both hooks are no-ops (no output, no error) — plain never animates.
- [ ] Repeated edit passes each stop/resume cleanly with no orphaned or duplicated spinner (still one-at-a-time after N cycles).
- [ ] The presenter does **not** invoke `$EDITOR` — the hooks only suspend/resume the presenter's own animation; the engine owns the hand-off.

**Tests**:
- `"suspend stops the spinner before hand-off then resume restarts it"` — pretty: start a blocking-stage spinner, `SuspendSpinner` (assert stopped, no frames), `ResumeSpinner` (assert restarted on the same stage line).
- `"no frames are emitted between suspend and resume"` — assert no spinner frames are written while suspended.
- `"no active spinner at hand-off is a safe no-op"` — `SuspendSpinner`/`ResumeSpinner` with no spinner running: no error, no output, no spinner created.
- `"plain suspend/resume are no-ops"` — plain mode: both hooks produce no output and do not error.
- `"repeated edit passes each stop/resume cleanly"` — N suspend/resume cycles; assert at most one spinner after each cycle and no orphaned/duplicated spinner at the end.
- `"the presenter does not invoke the editor"` — guard test: the suspend/resume path does not reach `os/exec`/`$EDITOR` (the hooks only stop/start the spinner).

**Edge Cases**:
- **Stop before hand-off then resume after** — suspend stops the spinner (terminal released), resume restarts it on the same stage line.
- **No active spinner at hand-off** — both hooks are safe no-ops; no spinner is created on resume.
- **Repeated edit passes each stop/resume cleanly** — multiple cycles leave no orphaned/duplicated spinner; one-at-a-time holds.
- **Plain no-op** — plain never animates; the hooks do nothing.

**Context**:
> `$EDITOR` (note edit) takes over the terminal — the spinner is stopped before handing off, resumed after.
> Regenerate / edit re-entry — the engine owns the loop … Because the engine drives the handoff, it is also the engine that stops the pretty spinner before `$EDITOR` takes over the terminal and resumes after.
> The presenter never calls `$EDITOR` or `claude`; it only re-renders on each pass. (cli-presentation-3-8 locked the render-only `Prompt` contract.)
> One spinner at a time, on the current stage line. (cli-presentation-4-5 established the single spinner handle and the one-at-a-time invariant; suspend/resume operate on that handle.)
> NOT Bubble Tea / no alt-screen — the suspend/resume is a plain stop/start, consistent with linear print-style narration.

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "Spinner Lifecycle (pretty only)" (`$EDITOR` hand-off — stop before, resume after), "Gating & `-y` Orthogonality" (Regenerate / edit re-entry — engine owns the loop and the hand-off), "Library Selection" (no alt-screen).

## cli-presentation-4-7 | approved

### Task cli-presentation-4-7: Width robustness — decorative rules capped at min(terminalWidth, ~50), wrap-never-truncate, fixed stage lines stay fixed

**Problem**: Pretty mode assumes a normal terminal and does **no pervasive width math** — but there is a **single concession**: the **decorative rules** (the `── release notes · v{X} ──` titled rule and its closing `────` rule) must be **capped at `min(terminalWidth, ~50)`** so they can't overflow and wrap into junk. Everything else **wraps naturally — never truncate** (losing release-note text is worse than a wrapped line). Fixed short stage lines **stay fixed** (they're short — no wrapping concerns). Genuinely tiny/weird terminals are a **`--plain` case** — no special handling. cli-presentation-2-5 deliberately deferred the rule-width cap to Phase 4 and rendered a fixed-width rule; this task implements the cap, the wrap-never-truncate guarantee for the notes body, and the undetectable-width fallback.

**Solution**: Implement decorative-rule width capping in the pretty presenter — compute the rule length as `min(detectedTerminalWidth, ruleCap≈50)`, falling back to the cap when width is undetectable — apply it to the notes titled/closing rules (replacing the cli-presentation-2-5 fixed-width rule), and prove the notes body wraps naturally and is **never truncated** while fixed stage lines remain unchanged. Tiny terminals get no special handling (they are the user's `--plain` cue).

**Do**:
- Add terminal-width detection for pretty mode, isolated as a small testable core. Recommended: `func ruleWidth(termWidth int) int { if termWidth <= 0 { return ruleCap }; return min(termWidth, ruleCap) }` with `const ruleCap = 50` (the spec's `~50`; exact value is an implementation detail — document the chosen constant). The startup/render path supplies `termWidth` from the OS (e.g. `term.GetSize(int(os.Stdout.Fd()))`); `termWidth <= 0` (or an error) means undetectable → fall back to the cap. Keep `ruleWidth` pure and unit-tested without a real terminal.
- In `internal/presenter/pretty.go` `ShowNotes` (from cli-presentation-2-5), render the titled rule and closing rule at `ruleWidth(termWidth)` instead of the fixed width — the `──`/`────` rules are sized to `min(terminalWidth, ~50)`. The titled rule still reads `── release notes · v{X} ──` with the trailing dashes filling out to the capped width; the closing rule is the capped-width run of `─`. Do not let the rule exceed the cap even on a very wide terminal, and do not let it overflow a narrow terminal (cap to the terminal width when narrower than ~50).
- **Wrap-never-truncate** for the notes body: the body continues to be written **verbatim** (byte-identical, per cli-presentation-2-5) and must **never** be truncated — long lines wrap naturally (terminal soft-wrap; the presenter does not hard-wrap or clip the body). Add a test with a notes line **longer than the cap and longer than a narrow terminal** asserting the full body bytes are present in the output (nothing dropped/clipped) — i.e. the presenter does not call any truncation/ellipsis on the body.
- **Fixed stage lines stay fixed**: the short stage lines (cli-presentation-2-3) are short and must remain unchanged by width handling — no wrapping logic, no width-dependent padding beyond the existing column alignment. Add a test asserting a stage line renders identically regardless of `termWidth` (narrow vs wide), proving width handling touches only the decorative rules.
- **Width axes**:
  - Terminal narrower than the cap → rule = terminal width (so it fits, no overflow/wrap-into-junk).
  - Terminal wider than the cap → rule = cap (~50; the rule doesn't sprawl across a wide terminal).
  - Undetectable width (detection error / `<= 0`) → rule = cap (the safe fallback; never zero-width, never overflowing).
- **Tiny/weird terminals are a `--plain` case** — implement **no** special handling for genuinely tiny terminals (e.g. a 3-column terminal). The cap-to-terminal-width still applies (the rule shrinks), but there is no bespoke tiny-terminal layout; the documented escape hatch is the user passing `--plain`. Add a doc note and a test asserting no special tiny-terminal branch exists (a tiny width simply yields a tiny rule via the same `min`; the body still wraps and is never truncated).
- This is **pretty-only**: plain has no decorative rules to cap (its `--- release notes v{X} ---` delimiters are fixed terse strings) and no width math — confirm plain is untouched.
- Coordinate with cli-presentation-2-5: that task rendered a fixed-width rule and explicitly deferred the cap here. Replace the fixed width with `ruleWidth(...)`; keep the byte-identical body guarantee intact.

**Acceptance Criteria**:
- [ ] Decorative notes rules (titled + closing) are sized to `min(terminalWidth, ~50)` via a pure, tested `ruleWidth` core.
- [ ] When the terminal is narrower than the cap, the rule equals the terminal width (no overflow); when wider, the rule equals the cap.
- [ ] When terminal width is undetectable (error or `<= 0`), the rule falls back to the cap.
- [ ] The notes body is **never truncated** — a body line longer than the cap/terminal is rendered in full (wraps naturally); no truncation/ellipsis is applied.
- [ ] Fixed short stage lines render identically regardless of terminal width (width handling touches only the decorative rules).
- [ ] No special tiny-terminal handling exists — a tiny width yields a tiny rule via the same `min`, and the body still wraps and is never truncated (the documented escape hatch is `--plain`).
- [ ] Plain mode is untouched (no decorative-rule capping, no width math).

**Tests**:
- `"a terminal narrower than the cap sizes the rule to the terminal width"` — `ruleWidth(30) == 30`; the notes rule renders 30 wide (no overflow).
- `"a terminal wider than the cap sizes the rule to the cap"` — `ruleWidth(200) == 50`; the rule renders at the cap, not 200.
- `"undetectable width falls back to the cap"` — `ruleWidth(0) == 50` and `ruleWidth(-1) == 50`; the rule renders at the cap.
- `"a long notes line wraps and is never truncated"` — notes body with a line longer than the cap and longer than a narrow `termWidth`; assert the full body bytes appear in the output (nothing clipped, no ellipsis).
- `"a fixed stage line is unchanged across widths"` — render a stage `✓` line at `termWidth=20` and `termWidth=200`; assert the stage line bytes are identical.
- `"a tiny terminal remains a --plain case (no special handling)"` — `ruleWidth(3) == 3` (tiny rule via the same `min`); assert no bespoke tiny-terminal branch and the body still wraps/untruncated.
- `"plain is untouched by width handling"` — plain `ShowNotes` renders its fixed `--- release notes v{X} ---` delimiters regardless of `termWidth`.

**Edge Cases**:
- **Terminal narrower than cap** — rule = terminal width (fits, no wrap-into-junk).
- **Terminal wider than cap** — rule = cap (~50; no sprawl).
- **Undetectable width falls back to cap** — detection error/`<= 0` → cap.
- **Long notes line wraps (never truncates)** — the full body is rendered; no clipping/ellipsis (losing note text is worse than a wrapped line).
- **Fixed stage line unchanged** — short stage lines are width-independent.
- **Tiny terminal remains a `--plain` case** — no special handling; the rule shrinks via `min` and the body still wraps untruncated.

**Context**:
> Width robustness (light touch): pretty mode assumes a normal terminal; no pervasive width math. The single concession — decorative rules are capped at `min(terminalWidth, ~50)` so the `── release notes ──`/closing rule can't overflow and wrap into junk. Everything else wraps naturally — never truncate (losing release-note text is worse than a wrapped line). Stage lines stay fixed (they're short). Genuinely tiny/weird terminals are a `--plain` case. Exact rule width is an implementation detail.
> Release notes — no box. A titled `── release notes · v{X} ──` rule, the body verbatim, a closing `────` rule. (The rounded box was dropped: it forced wrap/truncate on arbitrary-width AI notes and read as clutter.)
> Notes body verbatim — the same bytes … No stripping/transforming. (cli-presentation-2-5 guaranteed byte-identity; this task must preserve it while wrapping, never truncating.)
> cli-presentation-2-5 rendered a fixed-width rule and explicitly deferred the `min(terminalWidth, ~50)` cap to Phase 4 — this task implements the cap, the wrap-never-truncate guarantee, and the undetectable-width fallback.

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "The Pretty Layer" (Width robustness; Release notes — no box), "The Plain Layer" (notes delimiters — untouched).
