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

#### Tasks
status: approved
approved_at: 2026-06-09

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| cli-presentation-2-1 | Extend StageStarted/StageSucceeded payloads with long/blocking flag, detail, and engine-supplied elapsed | short stage carries no elapsed, long flag present but zero elapsed, detail empty |
| cli-presentation-2-2 | Plain stage narration — start line for long/blocking stages only, completion line per stage | short stage emits no start line, long stage emits start then completion, completion with no detail renders `{stage}: ok` |
| cli-presentation-2-3 | Pretty stage narration — stage line with detail and conditional elapsed | elapsed omitted on short stage, detail-only line, long stage shows elapsed |
| cli-presentation-2-4 | ShowPlan renders structured steps — pretty bulleted block, plain semicolon-joined one-liner | single step, empty plan with no steps, step with empty target |
| cli-presentation-2-5 | ShowNotes renders byte-identical body with per-mode delimiters | empty body, body with emoji headers, body containing delimiter-like lines, multi-line body with blank lines |
| cli-presentation-2-6 | Warn renders structured label + message in both modes and to stderr | empty message, multiple warnings in sequence, warn on an otherwise-successful run |
| cli-presentation-2-7 | StageFailed renders captured underlying output; FAILED summary to stderr, captured body not | empty captured output, captured output containing delimiter-like lines, multi-line captured output, FAILED summary on stderr without captured body |
| cli-presentation-2-8 | Unwound first-class event with `↩` glyph; suppress success end-of-run line on failure/abort | unwound after a stage failure, unwound after an abort with no prior failure, success end-of-run line absent when Unwound fired |

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

#### Tasks
status: approved
approved_at: 2026-06-09

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| cli-presentation-3-1 | Gate model with declared choice set rendered/returned by Prompt | four-choice notes-review gate, two-choice reuse confirm, gate declaring a non-y default, choice outside the declared set |
| cli-presentation-3-2 | Stdin-TTY detection independent of stdout render-mode detection | stdin non-TTY while stdout TTY, stdout non-TTY while stdin TTY, both non-TTY, no environment sniffing |
| cli-presentation-3-3 | Line-read input model — case-insensitive parse, empty-Enter default, re-prompt loop | empty Enter selects default, uppercase N maps to n, unrecognised x/a/q re-prompts (never accepts), whitespace-only line, repeated unrecognised then valid choice, EOF on stdin |
| cli-presentation-3-4 | Pretty Prompt vertical-menu rendering — options above question, [default] beside action, prompt last | two-choice confirm renders only y/n, [default] marker on declared default action, prompt redrawn after bad input |
| cli-presentation-3-5 | `-y` skip with rendered auto-accept echo in both modes | menu not drawn under -y, plain `notes: accepted (-y)`, pretty concise accept line, reuse-confirm auto-accepted, returns default without reading stdin |
| cli-presentation-3-6 | Forbidden-combination fail-loud (non-TTY stdin without `-y`) surfaced through Presenter and to stderr | non-TTY stdin without -y fails (no stdin block), styled failure in pretty, terse failure in plain, also to stderr, render mode still chosen on stdout |
| cli-presentation-3-7 | Regenerate source/target prompts reuse the line-read model, skip under `-y`, obey forbidden-combination | unrecognised input re-prompts, plain terse `key: value` lines, -y echoes chosen source/target, non-TTY without -y fails loud, flag/default used when skipped |
| cli-presentation-3-8 | Render-only Prompt contract — engine owns the `e`/`r` re-entry loop, linear re-render | `e` returns no presenter side effect, `r` returns no presenter side effect, repeated ShowNotes+Prompt passes scroll (no screen-clear/alt-screen), loop ends on y/n |

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

#### Tasks
status: approved
approved_at: 2026-06-09

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| cli-presentation-4-1 | init renders created/skipped lines in the shared vocabulary (no gate, no brand footer) | all created, all skipped (exist), mixed created+skipped, --force overwrite narrated as created, no release-style footer emitted |
| cli-presentation-4-2 | regenerate per-version narration with verb-shaped closing summary (omits url; --all oldest→newest, one block each) | single version, --all multiple versions in oldest→newest order, --all single version, closing summary omits url field, reuse-confirm vs fresh-notes path per block |
| cli-presentation-4-3 | version payload exception — plain bare value, pretty dressed | plain emits bare value only (no narration/glyph/trailing decoration), pretty dressed form, clean command-substitution consumption (no extra bytes) |
| cli-presentation-4-4 | Verb-shaped, success-only end-of-run line (release footer with url; regenerate without url; suppressed on failure) | release footer with url, regenerate close without url, init has no footer, failure run suppresses success line, abort run suppresses success line |
| cli-presentation-4-5 | Pretty spinner lifecycle — single spinner started on StageStarted, replaced in place by ✓/✗; output buffered, printed below ✗ | spinner replaced by ✓ on success, replaced by ✗ on failure, captured output below ✗ only, one spinner at a time across sequential stages, plain emits no animation frames, short non-spinner stage unaffected |
| cli-presentation-4-6 | Spinner stop/resume around the engine-driven `$EDITOR` hand-off | stop before hand-off then resume after, no active spinner at hand-off, repeated edit passes each stop/resume cleanly, plain no-op |
| cli-presentation-4-7 | Width robustness — decorative rules capped at min(terminalWidth, ~50), wrap-never-truncate, fixed stage lines stay fixed | terminal narrower than cap, terminal wider than cap, undetectable width falls back to cap, long notes line wraps (never truncates), fixed stage line unchanged, tiny terminal remains a --plain case |

### Phase 5: Analysis (Cycle 1)

**Goal**: Address findings from Analysis (Cycle 1).

#### Tasks

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| cli-presentation-5-1 | Extract shared byte-purity ASCII-scan test helper | nine inline copies replaced with one helper, two multi-stream sites assert both out and err, each call retains its per-site context string, negative path (ESC byte) still fails |
| cli-presentation-5-2 | Neutralise the hardcoded plain blocking-stage start verb | blocking stages emit `{name}: running...`, non-blocking stages stay silent, no "generating" remains in narration or doc comment, ASCII ellipsis preserved (byte-purity guard passes) |
| cli-presentation-5-3 | Collapse the four pretty constructors into one with functional options | single NewPrettyPresenter with WithProfile/WithErr/WithInput options, three-option combo reachable in one call, removed constructors and WithInput setter gone, production wiring + tests retargeted, rendered output byte-for-byte unchanged |
| cli-presentation-5-4 | State the ASCII/case-fold precondition on SourceGate/TargetGate | SourceGate and TargetGate doc comments state ASCII-enumerated-values-only precondition with reason, documentation-only (no code change), existing gate tests still pass |

### Phase 6: Analysis (Cycle 2)

**Goal**: Address findings from Analysis (Cycle 2).

#### Tasks

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| cli-presentation-6-1 | Extract shared gate-test construction+capture helpers | plainGate/prettyGate parameterised by -y/interactive-stdin toggles, three gate test files call helpers instead of inlining, arming toggle passed explicitly at every call site, no behavioural change to assertions |
| cli-presentation-6-2 | Consolidate mode-invariant both-arm assertions through promptDrivers table | mode-invariant property asserted once via t.Run(d.mode, ...), mode-specific byte-exact rendering tests kept separate, property set unchanged before/after, reuse Task 1 helpers without hard dependency |
| cli-presentation-6-3 | Consolidate decorative notes-rule expectation and the ANSI-strip helper into one shared pretty test helper | notes-rule prefix literal + fill/clamp arithmetic appear once, decorative rule width sourced once (no scattered 50), stripANSI/ruleDisplayWidth in one shared file referenced not redefined, exact rendered rule still verified |
| cli-presentation-6-4 | Add golden full-worked-example transcript test per render mode | plain -y full worked example transcript, pretty full worked example with spy/no-op spinner + fixed profile, asserts composition (spacing, block ordering, gate echo column, footer) not individual events, non-tautological (mutated composition detail fails) |

### Phase 7: Analysis (Cycle 3)

**Goal**: Address findings from Analysis (Cycle 3).

#### Tasks

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| cli-presentation-7-1 | Consolidate the prompt/gate test drivers onto the shared gate_helpers_test.go construction seam | one mode-driver table remains (two redundant ones removed), exactly one pretty-prompt driver (drivePrettyPrompt no longer an Ascii-pinned reimplementation), remaining prompt drivers build via plainGate/prettyGate seam, forced colour profile an explicit parameter, render-only screen-control guard still runs under TrueColor (SGR escapes present, no clear/alt-screen), no production .go file in internal/presenter modified, go vet reports no unused helpers/types |
| cli-presentation-7-2 | Converge the startup wiring so one entry point consumes StartupSignals and threads all axes | one entry point consumes StartupSignals threading Mode/width/-y/StdinInteractive, -y a parameter and stdin-interactive detected from descriptor (not from Mode), non-TTY stdin + -y unset reaches forbidden-combination fail-loud path, -y true auto-confirms without reading stdin, all four stdout/stdin TTY combinations match independently-resolved signals, no main/cmd package introduced, stdin/stdout/stderr are parameters (no os globals), no unused StartupSignals/DetectStartupSignals symbols |
