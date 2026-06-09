# Plan: CLI Presentation

## Phases

### Phase 1: Presenter Seam & Render-Mode Skeleton
status: approved
approved_at: 2026-06-09

**Goal**: Establish the `Presenter` interface, startup render-mode selection, and minimal `pretty`/`plain` implementations rendering a thin end-to-end stage flow driven by a fake recording engine.

**Why this order**: Walking skeleton. It threads the dual-implementation seam, TTY-based mode selection, and the stdout/stderr stream split through one minimal flow — proving the architecture at the cheapest moment. Every later phase plugs rendering into this seam, so it is the strongest foundation and has no prerequisites of its own.

**Acceptance**:
- [ ] A `Presenter` interface exists with a minimal event set (start-of-run, `StageStarted`, `StageSucceeded`, `StageFailed`, end-of-run)
- [ ] Mode is selected once at startup: `--plain` forces `plain`; otherwise `isatty(stdout)` → `pretty`, non-TTY → `plain`; nothing downstream re-checks the flag or TTY
- [ ] No environment sniffing — `LANG`/`LC_*`/`TERM`/`CI`/`NO_COLOR` are not consulted for mode selection
- [ ] A recording/fake presenter lets tests assert which events fired and with what payload, independent of rendering
- [ ] `pretty` and `plain` each render a minimal stage sequence end-to-end (start of run → a stage success → end of run) from the same event stream
- [ ] Narration is written to stdout and the stream split is wired and tested
- [ ] `plain` emits no ANSI, glyphs, or animation and pulls in no UI library; `pretty` styles via `lipgloss` and relies on its colour auto-downgrade when piped or on a colour-incapable TTY

#### Tasks
status: approved
approved_at: 2026-06-09

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| cli-presentation-1-1 | Define the Presenter interface with the minimal event set | none |
| cli-presentation-1-2 | Recording/fake presenter for event assertions | zero events recorded, multiple stages in sequence |
| cli-presentation-1-3 | Startup render-mode selection (TTY detection, no sniffing) | --plain on a TTY still selects plain, piped/redirected stdout selects plain, LANG/LC_*/TERM/CI/NO_COLOR set but ignored |
| cli-presentation-1-4 | Plain presenter renders the minimal stage sequence | no ANSI/glyph/animation bytes in output, no UI-library import |
| cli-presentation-1-5 | Pretty presenter renders the minimal stage sequence via lipgloss | colour auto-downgrade emits no colour codes on non-colour-capable TTY, layout/glyphs preserved under downgrade |
| cli-presentation-1-6 | Wire and verify the stdout/stderr stream split | failure summary on stderr and in stdout narration, narration absent from stderr on success |

### Phase 2: Run Narration — Stages, Plan, Notes, Warnings, Unwind
status: approved
approved_at: 2026-06-09

**Goal**: Render the full non-interactive release-run vocabulary in both modes — staged completions with detail and elapsed, the plan block, the verbatim notes block, warnings, failures with captured underlying output, and auto-unwind.

**Why this order**: This is the bulk of what a run looks like and builds directly on the Phase 1 seam. Completing the captured-log narration before interactive input keeps the input-handling risk isolated to a later phase. It has no dependency on gating.

**Acceptance**:
- [ ] `StageStarted` carries a long/blocking flag; `plain` emits a start line only for long/blocking stages while short stages render a single completion line; `pretty` shows stage progress (full spinner lifecycle hardened in Phase 4)
- [ ] `StageSucceeded` renders its engine-supplied detail; engine-supplied elapsed renders on long/blocking stages only — the presenter never times stages
- [ ] `ShowPlan` renders structured steps (verb + target) — `pretty` as a bulleted `Plan` block, `plain` as a semicolon-joined `plan: …` one-liner — both from the same structured payload, no separate terse field
- [ ] `ShowNotes` renders the notes body byte-identical across modes (emoji headers preserved, no stripping/transforming): titled rule + body + closing rule in `pretty`, `--- release notes v{X} ---` … `--- end notes ---` delimiters in `plain`
- [ ] `Warn` carries structured `label` + `message`; both modes render label-prefixed (`⚠ {label}  {message}` / `{label}: WARN - {message}`) and warnings also go to stderr
- [ ] `StageFailed` carries the message and captured underlying-command output; both modes render the captured output (`plain` wrapped in sliceable delimiters); the one-line `FAILED` summary additionally goes to stderr while the multi-line captured body does not
- [ ] `Unwound` is a first-class event with its own glyph (`↩`) and rendering in both modes; the success end-of-run line is suppressed on failure/abort runs

### Phase 3: Interactive Gating — Prompt, Input Model & `-y` Orthogonality
status: approved
approved_at: 2026-06-09

**Goal**: Implement the render-only `Prompt` gate with the line-read input model, gate-declared choice sets, `-y` skip with auto-accept echo, the forbidden-combination fail-loud, and the `regenerate` source/target prompts.

**Why this order**: Interactive input carries a distinct risk profile (stdin TTY detection, input parsing, re-prompt loop, the engine-owned re-entry handoff) and depends on the notes/gate rendering established in Phase 2. Render mode (stdout) remains selected independently of gating (stdin), per the orthogonality contract.

**Acceptance**:
- [ ] `Prompt(gate)` renders whatever choice set the gate declares and returns one choice — the four-choice (`y`/`n`/`e`/`r`) notes-review gate and the two-choice (`y`/`n`) reuse confirm both render from this one method
- [ ] `pretty` renders the vertical menu (options above the question, `[default]` beside its action, prompt last); Enter ⇒ `y` (default-yes)
- [ ] Input is line-read and case-insensitive; an empty Enter = accept default; unrecognised input re-prompts and never silently accepts
- [ ] Under `-y` the gate is skipped (not drawn-and-auto-pressed); a rendered auto-accept event echoes in both modes (`plain`: `notes: accepted (-y)`; `pretty`: a concise accept line)
- [ ] Forbidden combination (stdin not a TTY and `-y` not passed) fails loud, surfaced through the `Presenter` as a rendered failure (styled in `pretty`, terse in `plain`) and also to stderr
- [ ] `regenerate` source/target prompts use the same line-read model, render terse `key: value` in `plain`, skip under `-y` using provided flags/defaults with an auto-accept echo, and obey the forbidden-combination rule
- [ ] `Prompt` is render-only — it returns the choice and never invokes `$EDITOR` or `claude`; the engine owns the `e`/`r` re-entry loop and re-rendering on each `ShowNotes`/`Prompt` pass is linear (scrolls, no screen-clearing or alt-screen)

### Phase 4: Cross-Verb Rendering, Spinner Lifecycle & Width Robustness
status: approved
approved_at: 2026-06-09

**Goal**: Complete consistent rendering across all verbs (`init`, `regenerate`, `version`, and verb-shaped end-of-run lines), the `pretty` spinner lifecycle including `$EDITOR` hand-off, and the light-touch width robustness.

**Why this order**: Hardening and refinement across the full verb surface plus glyph/width polish, built on the complete event vocabulary (Phase 2) and gating (Phase 3). It closes out the "consistent presentation across every verb" goal and the pretty-mode polish.

**Acceptance**:
- [ ] `init` renders `created`/`skipped` lines in the shared vocabulary (no gate, no release-style brand footer)
- [ ] `regenerate` narrates per version using the shared stage/notes/gate vocabulary; `--all` runs oldest→newest as one narrated block each, with a closing line that summarises the set and omits the `{url}` field
- [ ] `version` is the payload exception: `plain` prints the bare value (`1.4.0`) for clean `$(mint version)` consumption; `pretty` may dress it (`🌿 mint v1.4.0`)
- [ ] The end-of-run line is success-shaped and verb-shaped (release brand footer with `{url}`; `regenerate` summary without `{url}`); it is suppressed on failure runs
- [ ] Pretty spinner lifecycle: one spinner at a time on the current stage line, started on `StageStarted` and replaced in place by the `✓`/`✗` line on completion; underlying command output is buffered (not streamed through the spinner) and printed below `✗` on failure; `plain` never animates
- [ ] The spinner is stopped before `$EDITOR` takes over the terminal and resumed after (engine-driven hand-off)
- [ ] Width robustness: decorative rules capped at `min(terminalWidth, ~50)`; everything else wraps naturally and never truncates; fixed short stage lines stay fixed
