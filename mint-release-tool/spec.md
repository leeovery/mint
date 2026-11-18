# Mint Release Tool

## 1. CLI Presentation

#### Specification

#### Scope & Output Modes

`mint`'s presentation layer is the styled-but-restrained UI applied **consistently across every verb** (`release`, `regenerate`, `init`, `version`, and the future `commit`) — not a release-only concern. It exists to make output good for a human at a terminal while staying token-efficient for an AI/agent consuming the output.

**Two output modes only — `pretty` and `plain`:**

- **`pretty`** (human): brand line, colour, spinners, formatted stages.
- **`plain`** (agent): terse, token-efficient text — no ANSI, no animation, no banner.

No structured (`json`/`toon`) mode. `mint` renders a *process* (an agent reads the narration; it does not parse it), not a queryable data structure, so structured output is out of scope — addable later if a need appears.

**Three orthogonal axes** govern behaviour independently:

| Axis | Driven by |
|---|---|
| **Styling** | `--plain` if passed, else `isatty(stdout)` |
| **Gating** | `-y`/`--yes` (skips interactive gate stops) |
| **Output stream** | Fixed — narration → stdout, errors/warnings → stderr |

Their independence is the backbone of the design: a human with `-y` at a terminal still gets full styling; `--plain` drops styling without touching gating; the stream split is fixed regardless of mode.

#### Render-Mode Detection & Output Streams

**Mode selection — by TTY detection, never environment sniffing:**

- **Precedence:** `--plain` passed → `plain`; otherwise `isatty(stdout)` → `pretty`, non-TTY → `plain`.
- Detection uses the OS-reported stream type: `term.IsTerminal(int(os.Stdout.Fd()))` (equivalently `os.Stdout.Stat().Mode() & os.ModeCharDevice != 0`). A human's terminal is a char device → `pretty`; an agent's harness captures stdout through a pipe → `plain`. Same binary, same path.
- **No sniffing of `LANG`/`LC_*`/`TERM`/`CI`.** A flag is an explicit instruction; the ban is on guessing from the environment.
- The two edge cases are intended behaviour, not compromises: a human who pipes/redirects (`mint … > out.txt`, `… | grep`) gets `plain` — exactly what's wanted when capturing output, and this *is* the force-plain path. An agent on a pseudo-terminal getting `pretty` is rare and harmless.

**`--plain` is the only render-mode flag (force-plain). No `--pretty`:**

- `--plain` is a **global/presentation flag** applying to every verb (recorded in the CLI surface alongside `-y`), distinct from per-verb engine flags.
- Force-plain has real demand: a UTF-8/braille-incapable TTY that would garble spinner/emoji glyphs, or simply wanting clean output without piping. `--plain` is the explicit, no-sniff escape hatch — and doubles as the discoverable form of "pipe to force plain."
- This is how the **terminal-capability gap** is resolved: rather than build locale-capability detection (which is the banned sniffing) plus a fallback glyph set, a broken-glyph terminal is the user's cue to pass `--plain`. `mint` never self-degrades on glyph capability.
- Force-pretty is out of scope (YAGNI): a real terminal already gets pretty, and `mint` is a local interactive tool, not a CI job.

**No colour flag.** Colour is intrinsic to `pretty` and absent from `plain` — there is no `--no-color` and no third "no-colour-but-styled" state. Don't want colour? Use `--plain` (or pipe/redirect). `NO_COLOR` env handling is out of scope (it's sniffing; `--plain` is the explicit equivalent).

**Colour-incapable terminal handled for free.** A real-but-colour-incapable TTY (`TERM=dumb`, etc.) is still selected as `pretty` by `isatty(stdout)`; `mint` leans on lipgloss's built-in colour auto-downgrade, which emits no colour codes there while keeping layout and glyphs. This is not a third `mint` mode — the styling library behaving correctly underneath is in scope and free. (Glyph/UTF-8 incapability is the separate `--plain` axis; colour incapability is automatic.)

**Output streams — narration is the product, so it's stdout:**

- **Run narration → stdout** — stages, the plan, the notes preview, the final summary, and `mint version`'s value. `mint` has no separate data payload, so the narration *is* its stdout output.
- **Errors + warnings → stderr** — for visibility under redirection. `mint release > run.log` sends stdout to the file, but a failure on stderr still reaches the terminal and cannot silently vanish. Errors/warnings appear in *both* the narration and on stderr.
- An agent capturing combined output (`2>&1`) by default sees narration and errors regardless of the split.
- **Exit code** signals success/failure for scripts. Ownership of the exit code is the **engine/`main`, not the `Presenter`** — the presenter is render-only and has no say in process status. (Noted here only because the stream contract touches it; the exit-code behaviour itself belongs to the engine specification.)

#### The `Presenter` Seam (Architecture)

The presentation layer is structured as an **event/step-oriented `Presenter` interface** that the engine calls at lifecycle points. The engine emits *semantic events* ("stage X started", "here's the plan", "warn: hook failed"); the presenter decides *how they look*. This is shaped around the *process* `mint` runs, not around data structures.

**Illustrative method set** (exact surface settled at implementation):

```
StageStarted(name) · StageSucceeded(name) · StageFailed(name, err)
Warn(msg) · Unwound(summary) · ShowPlan(plan) · ShowNotes(body) · Prompt(gate) → choice
```

**Event payload principle.** The method names above are illustrative; the exact Go signatures are settled at implementation. The fixed requirement is that **the engine supplies, in each event's payload, every datum the renderings consume** — the presenter never re-derives engine knowledge or hardcodes stage-specific logic. Specifically:

- **`StageStarted` carries whether the stage is long/blocking** (engine knowledge — it knows when it is about to invoke `claude` or run a build hook). The plain presenter uses this flag to decide whether to emit a start line; the pretty presenter always shows a spinner. The presenter does not hold a hardcoded list of long stage names.
- **`StageSucceeded` carries its detail string and the elapsed time**, both measured/supplied by the engine (the presenter does not time stages). Pretty renders `({elapsed})` on **long/blocking stages only** (the same stages flagged on `StageStarted`); short stages render detail without elapsed.
- **`Warn` carries a structured `label` and `message`** (e.g. `post_release` + the failure text). Both renderings are label-prefixed (`⚠ {label}  {message}` / `{label}: WARN - {message}`); the presenter does not parse a label out of a single string.
- **`StageFailed` carries the error message *and* the captured underlying-command output** (the git/`claude`/`gh` chatter `mint` buffered instead of streaming). The presenter renders it — never engine-printed — in **both** modes: pretty below the `✗` line, plain below the `{stage}: FAILED - {message}` line wrapped in a sliceable delimiter (e.g. `--- output ---` … `--- end output ---`, mirroring the notes-block delimiting so a reader/agent can extract it). The captured output is narration → **stdout**; per the stream contract the one-line `FAILED`/error summary additionally goes to **stderr** for redirect-visibility (the multi-line captured body is not duplicated to stderr).
- **`Unwound` is a first-class event** (not a `StageFailed`) carrying the "what it undid" summary; it has its own glyph (`↩`) and renderings in both modes.
- **`ShowPlan` carries structured plan steps** (each a verb + target), not pre-formatted text. Pretty renders them as a bulleted block; plain joins them into a `plan: …; …` one-liner. Each presenter formats from the same structured steps — there is no separate verbose/terse payload, and the abbreviations shown in the worked examples are illustrative wording, not a distinct terse field.
- **Gate auto-accept under `-y` is a rendered event, not engine-printed text.** When `-y` skips the gate, the engine emits an event the presenter renders (plain: `notes: accepted (-y)`; pretty: see the Gating section), preserving "narration → presenter."

**Decisions:**

- **Two implementations behind the interface — `pretty` and `plain`** — selected **once at startup** (`--plain` if passed, else `isatty(stdout)`). Nothing downstream re-checks the TTY or the flag.
- **The engine never touches colour, spinners, or TTY state.** It calls `Presenter` methods only. This mirrors the engine's existing dependency-inversion seams (`CommandRunner` for git/gh/claude, `Publisher` for releases) — the same discipline, now for output.
- **Applies to every verb.** `release`, `regenerate`, `init`, `version` (and future `commit`) all emit through the same `Presenter`. This is *how* "consistent presentation across all verbs" is met — structurally, via one interface, not per-verb styling code.
- **Testability** (the core Go rationale): assert which events fired and with what payload, independent of rendering. A `plain` impl is trivially assertable; a fake/recording presenter verifies engine behaviour without parsing styled text.
- **Spinners are a `pretty`-only concern** owned inside the pretty presenter (a spinner spans the gap between `StageStarted` and `StageSucceeded`/`StageFailed`); `plain` renders the same events as terse lines.

#### Gating & `-y` Orthogonality

`-y`/`--yes` is orthogonal to styling and to the output stream: it only skips **interactive gate stops**. A human at a terminal with `-y` still gets full styling. Gating is about *input* (is stdin a TTY?); render mode is about *output* (is stdout a TTY?) — checked independently.

**Gate inventory** (every verb walked for interactive stops):

| Verb | Interactive gate? | Under `-y` |
|---|---|---|
| `release` | **Yes** — the `Continue?` notes-review gate (also confirms the plan) | answers `yes` |
| `regenerate` | **Yes** — interactive *source* + *target* prompts, then the notes-review gate (fresh) / a simple confirm (reuse) | uses flags/defaults, auto-accepts |
| `init` | **No** — non-clobbering (skips existing with a notice; `--force` to overwrite) | n/a |
| `version` | **No** — prints its value | n/a |
| `commit` (future) | out of scope — separate feature | — |

Two gating verbs (`release`, `regenerate`). `init`'s safety is **structural** (non-clobber + `--force`), not a prompt — which is why it never needed `-y`.

**Forbidden-combination rule (applies to any interactive gate):** if **stdin is not a TTY** and **`-y` was not passed**, `mint` **fails loud** ("not a TTY — pass `-y` to run unattended") rather than blocking on stdin. `-y` answers every gate. This failure **surfaces through the `Presenter`** (rendered as a failure — styled in `pretty`, terse in `plain` — since render mode is selected on stdout independently of the stdin problem) and, per the stream contract, also goes to **stderr**.

**Gate input handling** (for the `Continue?` prompt):

- **Line-read** (type the letter, press Enter) — not raw single-keypress; no termios raw-mode complexity.
- **Empty line (just Enter) = default = accept.** The default fires only on a deliberate empty Enter.
- **Case-insensitive** (`N` = `n`).
- **Unrecognised key** (`x`, or old muscle-memory `a`/`q`) → **re-prompt**, never silently accept. Garbage never proceeds — keeps the destructive-adjacent default safe.

**Regenerate / edit re-entry — the engine owns the loop:** `Prompt(gate)` returns a single `choice` (`y`/`n`/`e`/`r`) and is render-only. On `e`/`r` the **engine** does the work — invoking `$EDITOR` (edit) or re-running generation via `claude` (regenerate-with-context) — then re-calls `ShowNotes` with the refreshed body and `Prompt` again, looping until `y`/`n`. The presenter never calls `$EDITOR` or `claude`; it only re-renders on each pass. Rendering is **linear** — each pass re-prints the notes block + gate below (it scrolls; no screen-clearing or alt-screen). Because the engine drives the handoff, it is also the engine that stops the pretty spinner before `$EDITOR` takes over the terminal and resumes after.

**Pretty under `-y`:** `-y` **skips the gate** rather than auto-pressing it — identical outcome to pressing Enter, but the menu is not drawn (consistent with "the gate is skipped"). "Full styling under `-y`" means the rest of the run is styled; it does not mean the interactive menu is shown then auto-answered. The auto-accept is rendered via the gate auto-accept event (pretty: a concise accept line in the same vocabulary, e.g. `✓ notes  accepted (-y)`; plain: `notes: accepted (-y)`).

**`Prompt(gate)` carries its choice set.** A *gate* is described by the choices it offers; `Prompt` renders whatever choice set the gate declares and returns one of them. This lets one method render every gate variant:

- **Notes-review gate** (release; regenerate-fresh) — the four-choice `y`/`n`/`e`/`r` `Continue?` menu specified in the Pretty Layer; default-yes; the engine owns the `e`/`r` re-entry loop.
- **Reuse confirm** (regenerate reusing existing notes) — a reduced **two-choice `y`/`n`** confirm rendered in the same `Continue?` vocabulary (no `e`/`r`, since there are no freshly-generated notes to edit or regenerate); default-yes. Plain skips it under `-y` exactly like the notes gate, with an analogous auto-accept echo.

**Regenerate source/target prompts.** Before the notes/confirm gate, `regenerate` has interactive *source* and *target* selection prompts. They render through the `Presenter` using the **same line-read input model** as `Continue?` (type the value, press Enter; case-insensitive; unrecognised input re-prompts, never silently proceeds), with plain rendering them as terse prompt lines in the same `key: value` vocabulary. Under `-y` they are **skipped using the provided flags/defaults**, with an auto-accept echo in the same vocabulary as the notes gate (so the chosen source/target are visible in a captured log). The forbidden-combination rule applies to them as to any interactive gate: non-TTY stdin without `-y` fails loud.

#### The Pretty Layer

The `pretty` presenter renders the run as styled, linear narration (print-style, no alt-screen). The look-and-feel below is the fixed intent; exact spacing/wording is refinable at implementation.

**Brand lines:**
- Top: `🌿 mint · {project}  ›  releasing v{X}`
- Bottom: `🌿 released {project} v{X} · {url}`
- The leaf ties to the engine's `commit_prefix` brand. Brand lines are flush-left; everything else indents under them.

**Status glyphs:**
- `✓` success (green) · `✗` failure (red) · `⚠` warn (amber) · `↩` auto-unwind. Spinner frames `⠋⠙⠹…`.

**Stage lines:** two-space indent, glyph, stage name padded to a column, terse detail. Symmetry/consistency is the bar — no ad-hoc indentation.

**Release notes — no box.** A titled `── release notes · v{X} ──` rule, the body verbatim, a closing `────` rule. (The rounded box was dropped: it forced wrap/truncate on arbitrary-width AI notes and read as clutter.)

**Review gate — vertical menu, options above the question, `[default]` next to its action, prompt last:**

```
    y  accept & proceed [default]
    n  abort
    e  edit in $EDITOR
    r  regenerate

  Continue? › 
```

**Enter ⇒ `y`** (accept & proceed — the 99% path). `n` ⇒ abort (full auto-unwind, owned by the engine). `e` ⇒ `$EDITOR`; `r` ⇒ regenerate-with-context.

**Width robustness (light touch):** pretty mode assumes a normal terminal; no pervasive width math. The single concession — **decorative rules are capped at `min(terminalWidth, ~50)`** so the `── release notes ──`/closing rule can't overflow and wrap into junk. Everything else **wraps naturally — never truncate** (losing release-note text is worse than a wrapped line). Stage lines stay fixed (they're short). Genuinely tiny/weird terminals are a `--plain` case. Exact rule width is an implementation detail.

**`-y` alignment:** `-y` answers this `Continue?` gate `yes` unattended — identical outcome to pressing Enter.

**Full `pretty` run (worked example):**

```
🌿 mint · acme  ›  releasing v1.4.0

  ✓ version    v1.3.2 → v1.4.0 (minor)
  ✓ preflight  clean · on main · tag free · in sync with origin

  Plan
    • commit   CHANGELOG.md + bin/acme
    • tag      v1.4.0 (annotated)
    • push     --atomic → origin
    • publish  GitHub release

  ✓ prep       pre_tag: npm ci && npm run build (2.3s)
  ⠋ notes      generating with claude…
  ✓ notes      generated (1.1s)

  ── release notes · v1.4.0 ───────────────────────
  Faster cold starts and a calmer log.

  ✨ Features
    • Parallel warm-up halves boot time
  🐛 Fixes
    • Stop double-flush on SIGTERM
  ─────────────────────────────────────────────────

    y  accept & proceed [default]
    n  abort
    e  edit in $EDITOR
    r  regenerate

  Continue? › 

  ✓ record     CHANGELOG.md + bin/acme
  ✓ tag/push   v1.4.0 pushed (atomic)
  ✓ publish    github release created

🌿 released acme v1.4.0 · https://github.com/acme/acme/releases/tag/v1.4.0
```

**Pretty failure + auto-unwind, and a post-release warn:**

```
  ✗ tag/push   push rejected: remote moved
  ↩ unwound    removed tag v1.4.0, reset 2 release commit(s) — repo clean

  ⚠ post_release  hook failed (tag is already published): scripts/notify.sh exited 1
```

#### The Plain Layer

The `plain` presenter renders the same `Presenter` events as terse, token-efficient text — no animation, no glyphs, no colour. Only the **delimiters and stage narration** differ from pretty; the **notes body is byte-identical** in both modes.

**Contract:**

- **`key: value` lines, lowercase, one per stage on completion.**
- **Start line for long/blocking stages only.** A stage that blocks on something slow (AI **notes** generation, a `pre_tag` build hook) also emits a terse start line (`notes: generating…` → `notes: generated (1.1s)`), so a live-tail consumer (`mint release | tee log`, a streaming agent) isn't staring at silence through a multi-second wait. **Short stages stay one-line-on-completion** — no start line. This is plain's equivalent of the pretty spinner; the captured-log target gains only one extra line per long stage.
- **Stage terseness** as-is (e.g. `preflight: ok (clean, on main, tag free, in sync)`) — terse but human-legible, not pared further.
- **Notes block** delimited by plain rules: `--- release notes v{X} ---` … `--- end notes ---`, so a reader can slice it out reliably.
- **Notes body verbatim** — the same bytes as pretty/tag/changelog/release, **emoji headers shown if present** (`✨ Features`, `🐛 Fixes`). No stripping/transforming. This is non-negotiable, not stylistic: stripping or transforming the body would contradict the engine's "use the body whole" rule and break the "what previews is what ships" invariant. The emoji headers superficially cut against plain's token-efficiency goal, but the extra tokens are negligible and preview/ship parity wins — plain mode must not be "optimised" by stripping them.
- **`-y` echo** — when the gate is skipped under `-y`, emit `notes: accepted (-y)` so the auto-accept is visible in the captured log.
- **Errors/warnings** also go to **stderr** (per the stream contract), in addition to appearing in the plain narration.

**Per-event rendering (pretty vs plain):**

| Event | `pretty` | `plain` |
|---|---|---|
| start of run | `🌿 mint · {project}  ›  releasing v{X}` brand line | `mint: releasing {project} v{X}` |
| `StageStarted` | dim line with spinner: `⠋ notes  generating with claude…` | (blank for short stages; long/blocking stages emit a terse start line, e.g. `notes: generating…`) |
| `StageSucceeded` | `✓ {stage}  {detail} ({elapsed})`, glyph green | `{stage}: ok` / `{stage}: {detail}` |
| `StageFailed` | `✗ {stage}  {message}`, glyph red | `{stage}: FAILED - {message}` (also stderr) |
| auto-unwind | `↩ unwound  {what it undid} — repo clean` | `unwound: {what}; repo clean` |
| `Warn` | `⚠ {label}  {message}`, amber | `{label}: WARN - {message}` (also stderr) |
| `ShowPlan` | a `Plan` block, bulleted | `plan: {semicolon-joined one-liner}` |
| `ShowNotes` | titled rule + body + closing rule (no box) | `--- release notes v{X} ---` … `--- end notes ---` |
| review gate | vertical menu then `Continue? › ` prompt; Enter ⇒ `y` | (not shown — non-TTY ⇒ `-y` required ⇒ gate skipped; emits `notes: accepted (-y)`) |
| end of run | `🌿 released {project} v{X} · {url}` | `done: {project} v{X} {url}` |

**Same run in `plain` (agent, `-y`):**

```
mint: releasing acme v1.4.0
version: v1.3.2 -> v1.4.0 (minor)
preflight: ok (clean, on main, tag free, in sync)
plan: commit changelog+version; tag v1.4.0; push --atomic; publish github
prep: pre_tag ok (2.3s)
notes: generated (1.1s)
--- release notes v1.4.0 ---
Faster cold starts and a calmer log.

✨ Features
- Parallel warm-up halves boot time
🐛 Fixes
- Stop double-flush on SIGTERM
--- end notes ---
notes: accepted (-y)
record: CHANGELOG.md + bin/acme
tag/push: v1.4.0 pushed (atomic)
publish: github release created
done: acme v1.4.0 https://github.com/acme/acme/releases/tag/v1.4.0
```

**Plain failure:**

```
tag/push: FAILED - push rejected: remote moved
unwound: removed tag v1.4.0, reset 2 commits; repo clean
```

#### Spinner Lifecycle (pretty only)

Spinners are a `pretty`-only concern, owned inside the pretty presenter. `plain` never animates — a stage emits exactly one line on its transition.

- **One spinner at a time, on the current stage line.** Starts on `StageStarted`, replaced in place by the `✓`/`✗` line on completion. Braille frames (`⠋⠙⠹…`). The spinner is part of the narration on stdout.
- **Underlying command output** (git/claude/gh chatter) is captured by `mint`, not streamed through the spinner line, so the animation can't be corrupted. On failure, `mint` prints the captured output below the `✗` line.
- **`$EDITOR` (note edit)** takes over the terminal — the spinner is stopped before handing off, resumed after.

#### Library Selection

- **`lipgloss` for all `pretty`-mode styling** — colour, the 🌿 brand line, status glyphs, the titled notes rule. It is *pure string styling* (no event loop), so it composes with the `Presenter` seam, and it **auto-downgrades colour when piped** (also relied on for colour-incapable TTYs — see Render-Mode Detection).
- **A lightweight standalone spinner** for stage progress — e.g. `briandowns/spinner` (explicit `Start()`/`Stop()`, maps 1:1 to `StageStarted`/`StageSucceeded`) or charm's `huh/spinner`. The exact package is an implementation detail; the seam doesn't care.
- **NOT Bubble Tea / no alt-screen / no full-screen TUI.** Print-style linear narration only. A full TUI would fight the `Presenter` seam (Bubble Tea wants to own the event loop; `mint`'s engine drives and calls the presenter) and the dual pretty/plain requirement.
- **`plain` mode pulls in no UI library** — just `fmt` lines. That is the point of token-efficiency.

#### Cross-Verb Rendering

The worked examples are all `mint release`, but the `Presenter` seam applies to every verb. All four verbs emit through the same `Presenter`; consistency is structural (one interface), not per-verb styling code.

- **`init`** — process narration in the same vocabulary: `✓ created .mint.toml` / `· skipped release (exists, use --force)`. No gate (non-clobbering).
- **`regenerate`** — same stage/notes/gate vocabulary as `release`, narrated per version (`--all` runs oldest→newest, one block each).
- **`version`** — the **one payload verb**: its output is a *value*, not narration. **Plain prints the bare value** (`1.4.0`) so `$(mint version)`/scripts consume it cleanly; **pretty may dress it** (`🌿 mint v1.4.0`). This is the deliberate exception to "narration is the product" — `version` actually has a payload, so the bare value is the floor and styling is additive only in pretty.

**End-of-run line — success-shaped and verb-shaped.** The `🌿 released {project} v{X} · {url}` / `done: {project} v{X} {url}` lines are the *release-success* form. The closing line follows the verb's payload:

- **`regenerate`** does not publish and has no release URL — it emits a closing summary in the same vocabulary without the `{url}` field; with `--all` (oldest→newest), each version is its own narrated block and the closing line summarises the set.
- **`init`** has no versioned release — its `created`/`skipped` lines are themselves the terminal output; no release-style brand footer is required.
- **Failure runs** end after the `✗`/`unwound`/`warn` lines: the **end-of-run success line is suppressed** (it is success-only). Failure/abort is communicated by those lines plus the engine-owned non-zero exit code — there is no failure-flavoured closing brand line.

#### Dependencies

Prerequisites that must exist before implementation can begin:

##### Required

_None._ The presentation layer is **foundational**, not dependent. It defines the `Presenter` seam that every verb consumes; via dependency inversion the engine depends on this layer, not the reverse. The pretty/plain implementations are rendering logic driven by semantic events and can be built and tested standalone with a fake/recording engine driving the events.

##### Notes

- **Built independently of the engine.** The `Presenter` interface and its two implementations need no engine code to exist — a recording presenter and fixed event sequences are enough to build and assert rendering. The worked release examples are illustrative; the presenter renders whatever events it is handed.
- **Third-party libraries** (`lipgloss`, a standalone spinner package) are implementation dependencies, not spec prerequisites.
- **Reconciliations owed *by* the `mint-release-tool` (engine) specification** — reverse direction; this spec is the source of truth for rendering:
  - **Review-gate rendering** — this spec's default-yes `Continue?` gate (`y`/`n`/`e`/`r`, Enter ⇒ accept) supersedes the engine discussion's `[a] accept / [e] edit / [r] regenerate / [q] abort`. Same four semantic choices (auto-unwind on abort); only the rendering changes. The engine spec must adopt this rendering and drop the stale `[a]`/`[q]` keys. The engine owns the four semantic choices; presentation owns how they look.
  - **`--plain` global flag** — a new presentation flag applying to every verb, to be recorded in the engine spec's CLI surface alongside `-y`.
- **Exit-code ownership** stays with the engine/`main`, not the `Presenter` — recorded here only because the stream contract touches it.

---

#### Working Notes

[Optional - capture in-progress discussion if needed]

## 2. Mint Release Tool

#### Specification

#### Overview & Scope

##### Purpose

`mint` is a reusable, configuration-driven Go release tool that replaces the per-project `release` bash scripts that have been copy-pasted (and have drifted) across ~8 repositories. It extracts the generic release engine into one reusable binary: AI-generated release notes, semver bump, lock-resilient git handling, CHANGELOG generation, annotated tag + atomic push, and GitHub release creation.

##### Settled foundations (not re-litigated)

- **Language: Go** — chosen for testability of the fragile logic (git/`gh`/`claude` invocation) behind a single `CommandRunner` interface that can mock those external commands.
- **Name: `mint`** for the global binary; each project keeps a tiny local shim named `release` for muscle memory.
- **Distribution:** a new public dual-arch Homebrew formula in the existing `leeovery/homebrew-tools` tap. `mint`'s source lives in its own repository, reusing the tap's existing auto-update CI action. Install via `brew install leeovery/tools/mint`.
- **Activation model:** each project carries a committed `release` shim that delegates to the globally-installed `mint`; `mint init` scaffolds the per-project config and shim.

##### Command namespace

`mint` adopts a `mint <verb>` command namespace from the outset. The release command is `mint release`; the per-project `release` shim delegates to `mint release`. This **specification covers the release pipeline only**. `mint` is now an **epic**: the release pipeline is one specification within it, with sibling specifications for the CLI presentation layer and the `mint commit` verb. This spec is standalone and self-contained; cross-spec touch-points are recorded under Dependencies.

##### In scope (this build)

The complete release pipeline end-to-end: version determination → preflight safety gates → project-prep hooks → AI release notes (with interactive review) → record (changelog + version file) → annotated tag + atomic push → publish (GitHub release + post-release hooks); plus the regenerate/heal command, the TOML config schema, the CLI surface, and `mint init` scaffolding.

##### Out of scope (consciously deferred)

- **`mint commit`** — a sibling verb specified separately (shares the AI engine, config model, and presentation layer; not part of this spec).
- **Testing / parity strategy** — deferred to planning/implementation. The legacy 552-line `agentic-workflows/release` bash script is treated as a **feature reference / capability checklist, not a byte-parity test oracle**; the clean-slate design intentionally diverges from it.
- **YAGNI items addable later:** pre-release/RC tag parsing & production, `--rewrite-tags` (destructive tag rewriting), a `.release/hooks/` directory convention, built-in note "themes", project auto-detection in `mint init`, a dry-run hook-run toggle, a notes-review disable toggle, and a `.mintignore` file.

---

#### Release Lifecycle (the spine)

A `mint release` run proceeds through seven stages, in strict order. This spine is the contract that hooks, config, and recovery all hang off.

| # | Stage | What happens | Reversible? |
|---|-------|-------------|-------------|
| 1 | **Version** | Determine the current version (from git tags) and compute the next (patch/minor/major or explicit). | Yes — read-only |
| 2 | **Preflight** | Safety gates: clean tree, on release branch, target tag free, remote in sync, required tools present & authenticated. | Yes — read-only checks |
| 3 | **Project prep (hooks)** | Run the project's `pre_tag` hook (build/generate artifacts). May dirty the tree; mint commits artifacts. | Yes — local only |
| 4 | **Release notes** | Generate the notes body from the diff via the AI engine; interactive review gate. | Yes — local only |
| 5 | **Record** | Write the CHANGELOG entry and the optional version-file projection; create release commit(s). | Yes — local only |
| 6 | **Make official** | Create the annotated tag and `git push --atomic` (commits + tag together). | **No — point of no return** |
| 7 | **Publish** | Create the provider release (GitHub today) + run `post_release` hooks. | Post-PONR — warn-only on failure |

##### Invariants

- **Everything before stage 6 is local-only and recoverable.** If any stage 1–5 fails (or the user aborts at the review gate), mint auto-unwinds every mutation it made this run, returning the repo to the exact clean state it started from.
- **`git push --atomic` (stage 6) is the single point of no return.** Commits and tag go up together or not at all.
- **After the point of no return, mint never unwinds** (that would mean rewriting published history). Failures in stage 7 warn and point to the heal path.
- One mental model: *nothing mint did this run survives unless the release completes.*

The per-stage details are specified in their own sections below.

---

#### Stage 1 — Version Determination & Tag Grammar

##### Source of truth: git tags, always

The current version is the **highest** SemVer tag in the repository (stripped of its prefix). There is no file-based or embedded version source — brew installs from tags, so the tag *is* the real version; any file copy is derived state. With no matching tags, the current version is `0.0.0`.

- **"Latest" = the numerically highest matching version, globally** — not `git describe`'s nearest-reachable-from-HEAD (which diverges on branches and hotfix lines). Tag-as-truth requires the true maximum across all tags.
- Preflight's fetch includes `--tags`, so mint always sees the complete tag set even after a fresh/partial clone.

##### Recognised tag grammar

- **Strict SemVer 2.0.0, three numeric segments only:** `MAJOR.MINOR.PATCH`. Anything else (`release-1.2`, `1.2`, `1.2.0.4`, `1.2.0-rc.1`, `1.2.0+build5`) is **not** a mint version and is ignored entirely.
- **Recognised pattern:** `^{tag_prefix}(\d+)\.(\d+)\.(\d+)$`. Tags not matching are ignored when computing "latest".
- **`tag_prefix` config, default `"v"`** — mint reads the prefix off existing tags, parses the semver, and writes the prefix back when tagging. Overridable to `""` or anything else. The same knob covers component/monorepo prefixes, e.g. `tag_prefix = "pkg-name/v"`.

##### Bump selection

The next version is computed from the current version by a bump flag:

- `-p` / `--patch` — **default** when no flag is given
- `-m` / `--minor`
- `-M` / `--major`
- `--set-version X.Y.Z` — explicit version escape hatch (e.g. a deliberate 1.x → 2.0.0 jump)

**First release handles itself** with no special-casing: with no tags the current version is `0.0.0`, so `mint release` → `0.0.1`, `mint release -m` → `0.1.0`, `mint release -M` → `1.0.0`.

##### `--set-version` rules

- **Mutually exclusive with bump flags.** `--set-version` combined with `-p`/`-m`/`-M` is an **error** ("can't combine `--set-version` with a bump flag") — no silent precedence. (`--set-version` alone = explicit; a bump flag alone = computed; neither = default patch.)
- **Must be valid 3-part semver AND strictly greater than the current latest tag.** A backwards/equal jump is rejected by default *even if the target tag is free*, because a lower version sorts below "latest" and corrupts tag-as-truth. (This sits on top of the free-tag preflight check, which catches an equal/existing tag.)
- **Forward-only today; no downgrade override.** A `--force`-style "re-tag an old line" escape is YAGNI and deliberately not built now.

##### Optional version-file projection

When a project needs the version written *into the repo*, mint mirrors the new version into a file during the **Record** stage (Stage 5). The file is always a **write-only mirror kept in sync** — never a source of truth.

- `version_file` — path to write; **omit = tag-only** (no projection).
- `version_pattern` — e.g. `RELEASE_VERSION="{version}"`; **omit = the whole file *is* the version** (plain mode).

**Legacy strategy mapping** (the old `VERSION_STRATEGY` model collapses into this; all absorbed, none lost):
- old `none` → no `version_file` (tag is truth).
- old `file` (plain `release.txt`) → `version_file = "release.txt"`, no pattern.
- old `embedded` (sed-replace into a source file) → `version_file` + `version_pattern = 'RELEASE_VERSION="{version}"'`.

The behavioural change vs. legacy: these are now write-only mirrors, not read sources.

##### Explicitly rejected (YAGNI)

- **Pre-release / RC tags** (`1.2.0-rc.1`) — not parsed or produced. (Accepted consequence: a repo whose only tags are RC tags reads as `0.0.0` — not a real scenario here.)
- **4th / build segments** (`1.2.0.4`, `1.2.0+build5`) — not SemVer 3-part; break brew and tag-as-truth. Docker/CI build numbers are stamped at image-build time off mint's released version, never baked into the release tag.

---

#### Stage 2 — Preflight & Safety Gates

##### Principle

Releasing is high-consequence, so mint forces a conscious, known-good starting state. All preflight checks are cheap and reversible, and all run before any mutation or hooks. The design favours safety, with explicit escape hatches for the cases where a gate would merely annoy.

##### The gate set (run in order — cheap local checks first, then network)

Nothing irreversible happens until all applicable gates pass.

1. **Git repo present**, anchored at the repo root (resolved via `git rev-parse --show-toplevel`; mint runs from root).
   - **Submodule / worktree resolution:** `git rev-parse --show-toplevel` resolves to the innermost enclosing repository or linked worktree from the invocation directory, and mint operates on *that* repository. A **submodule** is treated as its own repo (its own tags and its own `.mint.toml`); a **linked worktree** resolves to the worktree root and shares the main repository's tag/ref store. mint always anchors to and runs from that resolved root.
2. **On the release branch** — default-on, **auto-derived from `origin/HEAD`** (resolves `main`/`master` with zero config). Override via `release_branch` in config. Escape hatch: `--any-branch` for a deliberate off-branch release.
3. **Clean working tree (strict)** — `git status --porcelain` must be empty. Gitignored files are exempt (build outputs don't trip it); blocks on uncommitted/unstaged tracked changes *and* non-ignored untracked files. Escape hatch: **`--autostash`** (opt-in, not default) stashes (`--include-untracked`) before the run and restores after, **including on abort/failure**. Opt-in because the release mutates the tree (hook commits, changelog, version file) and popping unrelated WIP on top can conflict — opting in is the user asserting it's safe. **Restore ordering:** on abort/failure, mint **first auto-unwinds its own commits/tag** (back to the clean starting state), **then** pops the stash on top; if the pop conflicts, mint **leaves the stash intact and warns** — it never discards the user's WIP.
4. **Target tag is free** — the computed `{tag_prefix}X.Y.Z` must not exist locally or on the remote. Closes the double-release / re-run footgun.
5. **Remote sync** — `git fetch`, then **abort (never auto-pull)** if local is *behind* or *diverged* from the release branch's upstream. Being *ahead* is fine and expected (those are the commits being released). Auto-pulling would silently drag in unseen remote commits and release them; integrating remote work must be a conscious act. Clear abort message, e.g. "N commits behind origin/main — pull and review, then re-run".
6. **`gh` installed + authenticated** — gated **only when actually publishing** a GitHub release, and **before the tag**, so a missing/unauthenticated `gh` never strands a pushed tag with no release.

##### Tool gating summary

- **`git`** — implied/required.
- **`gh`** — gated conditionally, only when publishing.
- **`claude` CLI is NOT a preflight gate** — AI notes are optional with graceful fallback (see AI release notes).

##### Project preflight hook

After mint's built-in preflight checks pass, the project's optional `preflight` hook runs (for project-specific gates/validation) — before any mutation. A non-zero exit aborts the release cleanly. (Detailed in the Hooks section.)

---

#### Hooks

##### Purpose & scope

Hooks are mint's escape valve for steps **specific to one project** that mint cannot know about generically. Anything universal-but-optional (version-file writing, diff-exclude globs) is deliberately absorbed into mint as built-in, tested config rather than left to hooks. The guiding test: *if mint already owns the data/concern, it's core; hooks are only for genuinely bespoke project steps.*

##### Mechanism: one mechanism only

Hooks are a **config table of shell commands keyed by lifecycle point** (`[release.hooks]` in `.mint.toml`). There is no separate `.release/hooks/` directory convention — a command string can simply *call* a script, so scripts are just something a string invokes, not a second mechanism.

```toml
[release.hooks]
pre_tag = "npm ci && npm run build"        # single string — the 90% case
# or:
pre_tag = ["npm ci", "npm run build"]      # array of strings, run in order
```

- **Value is a string *or* an array of strings.** Array entries run sequentially; the **first non-zero exit aborts** (for pre-PONR hooks). String for one command; array for readable multi-step without quoting a giant `&&` chain.
- **Executed through a shell** (`sh -c "<entry>"`) so `&&`, pipes, env vars, and `./script.sh` invocations all work.
- **Run from the repo root.**
- **Complex/conditional logic lives in a script file** that the config points at; `mint init` may scaffold an example script + reference, but the directory is not load-bearing.

##### Hook points (three, mapped to the spine)

- **`preflight`** — runs *after* mint's built-in preflight checks (Stage 2), for project-specific gates/validation. Before any mutation.
- **`pre_tag`** — Stage 3 project prep (build/generate artifacts, e.g. a knowledge bundle). Dirties the tree → mint commits per the interplay rule below.
- **`post_release`** — Stage 7 follow-ups after the provider release (notifications, tap `repository_dispatch`, etc.).

**No `post_tag`** point (between tag/push and publish) and **no `pre_notes`/`post_notes`** points — no use case; YAGNI. Adding a point later is trivial under the config-table mechanism.

##### Commit interplay (`pre_tag`)

After a `pre_tag` hook runs, **mint commits whatever it left dirty** (message `chore(release): pre-tag artifacts for {tag}`). Consequences:

- Simple hooks never touch git — they just build; mint handles the commit.
- "Commit only if the bundle changed" falls out for free: changed → tree dirty → mint commits; unchanged → tree clean → nothing committed.
- A hook that wants a *custom* commit can do its own and hand mint back a clean tree — mint then sees nothing to commit.

Either way, mint never tags a dirty tree, and hook authors aren't forced to know git.

##### Failure behaviour (asymmetric across the point of no return)

- **`preflight` / `pre_tag`** run *before* the tag is pushed → a non-zero exit **aborts the whole release cleanly** (no tag, no damage; mint auto-unwinds any local mutations).
- **`post_release`** runs *after* the tag is live → it **cannot abort**; a non-zero exit just **warns** ("post_release hook failed; tag is already published"). Same principle as a failed `gh release create`.

##### Invocation & context (injected env vars)

Each hook entry runs via `sh -c` from the repo root. mint injects:

| Variable | Example | Meaning |
|---|---|---|
| `MINT_NEW_VERSION` | `1.4.0` | the version being released |
| `MINT_PREVIOUS_VERSION` | `1.3.2` | the prior latest version |
| `MINT_VERSION_TAG` | `v1.4.0` | the full tag (with prefix) |
| `MINT_BUMP` | `patch` / `minor` / `major` / `explicit` | the bump kind; `explicit` when `--set-version` was used |
| `MINT_DRY_RUN` | `0`/`1` | dry-run flag |

The set may grow as later stages need it.

##### Dry-run behaviour

Under `--dry-run`, mint **skips hooks** (they have side effects) and reports that they were skipped. (Confirmed in the dry-run semantics section.)

---

#### Stage 4 — AI Release Notes

Generate a release-notes body from the diff since the last release. The same body is reused for every output surface (tag annotation, CHANGELOG, provider release) — generate once, use everywhere.

##### Source of truth: the diff alone

mint generates notes from the **release diff and nothing else**. Commit messages / history are deliberately **not** ingested in any form:

- They are the **path, not the destination** — a commit may add code a later commit removes; the final diff correctly shows neither.
- They are **unreliable and entirely user-controlled** — mint may not have authored them (`mint commit` adoption is optional), so they may be hand-written or bare `WIP`. There is no floor on commit-message quality to build on.
- The conditional machinery to exploit them **isn't worth it** — the signal is unreliable and shrinks further as squash/rebase collapse history.

The diff is the one signal always true regardless of merge strategy or commit discipline, so it is the sole input. Quality is lifted by making the diff **more legible** (the Change Map, below) — never by adding other data.

##### Engine layering (context assembly vs AI transport)

Notes generation separates two concerns, mirroring the engine's other seams (`CommandRunner`, `Publisher`, `Presenter`):

- **Context assembly (git-aware):** builds what the AI describes — the `last_tag..HEAD` diff, `diff_exclude` filtering, the `max_diff_lines` guard, and the Change Map preamble. All git-aware, release-specific logic lives here.
- **AI transport (content-agnostic):** takes an assembled prompt + content + `ai_command`, runs the call, validates (non-empty / not-an-error / not-a-refusal), retries once, and returns the body. It knows nothing about git, diffs, or tags — pure "content in, message out."

The boundary keeps the transport trivially testable (a string + a fake `ai_command`) and means quality work (enriching the Change Map / context) changes only the assembly side, never the transport. This is the same shared AI engine the sibling `mint commit` verb consumes — with a different context source (its staged diff) and prompt.

##### Diff base

- Diff **`last_tag..HEAD`** (changes since the last release).
- **First release (no prior tag):** there's no base to diff and diffing the whole repo is useless to an AI → mint **skips the AI and uses a fixed body, "Initial release."**
- **Computed at the post-hook HEAD:** because `pre_tag` hooks (Stage 3) commit before notes generate (Stage 4), HEAD already includes the hook-artifact commit. This is intended — `diff_exclude` filters hook artifacts (e.g. the bundle) out *by path* regardless of being freshly committed, so the AI never sees bundle churn. Anything a hook commits that *isn't* excluded legitimately appears in the notes.

##### Engine

- **Default `claude -p`.** mint composes the prompt, pipes it to the command's stdin, and reads the body from stdout, with a **timeout (~60s)** so a hung call can't stall a release.
- **Command overridable** via `ai_command` (default `claude -p`). mint always *owns the prompt*; the command is just transport — cheap future-proofing (swap binary/model) that keeps prompt-control working.

##### Diff exclusion (two tiers + strategy-aware version file)

The diff sent to the AI is filtered via git's `:(exclude)` pathspec (git does the filtering):

- **Built-in always-exclude — `CHANGELOG.md`** (non-configurable). Pure mint output, never meaningful source. Excluded in both forward and regenerate paths.
- **`version_file` — NOT blanket-excluded (strategy-aware):**
  - *Forward path:* nothing to exclude — notes generate (Stage 4) *before* the version write (Stage 5), so the file is inherently unchanged at notes time. (The whole concern is therefore **regenerate-only**.)
  - *Regenerate, plain mode* (whole file is the version, e.g. `release.txt`): **exclude** the file — pure bookkeeping.
  - *Regenerate, embedded mode* (`version_pattern` in a real source file like `main.go`): **do not exclude** — it's source we want in notes. The lone version-line bump is negligible and neutralised by the default prompt's "ignore version-number bumps" instruction, not by hiding real code.
- **`diff_exclude` (project artifacts) — configurable array of globs**, on top of the above (knowledge bundle, minified output, lockfiles, generated code). These are *tracked, committed* generated files (deliberately not in `.gitignore`), which is why explicit exclusion is needed. A release diff is commit-to-commit so it can only contain tracked files; gitignored files never appear. A file that is gitignored yet *force-added* is nonetheless tracked, so it can still appear in the diff — this edge is deliberate and not special-cased. Kept in config (not a `.mintignore` file) per the "one config, one place to look" principle; `.mintignore` is YAGNI, addable later if exclude sets grow large.

**Exclusion is path-based, never commit-based.** The forward path diffs a HEAD that *predates* mint's release-bookkeeping commits; regenerate diffs a tag range (`vX-1..vX`) that *already contains* them. A git range diff operates on paths/content and **cannot subtract commits**, so even though mint's bookkeeping commit carries a recognisable `commit_prefix` (cosmetic only), it cannot be dropped from the range — path exclusion is what reproduces the forward path's *source* view on the regenerate path.

##### `max_diff_lines` guard

- **Default 50000.** Not a context limit but a **cost + quality** guard — a huge diff is slow, costly, and summarises to mush. Lines are a cheap token proxy (~10–20 tokens/line). **Excluded paths don't count toward it.** Exceeding it = a notes failure → abort-or-fallback per `on_notes_failure`. Fully overridable.

##### Degenerate release (empty / all-excluded diff)

The mirror image of the `max_diff_lines` ceiling — a guard at the *small* end. If the post-`diff_exclude` diff is **empty or whitespace-only** (a re-tag with no source change; a release where every changed file fell under `diff_exclude`; or pure churn with nothing notable), mint **does not call the AI** — an empty diff is the one input it will reliably hallucinate on. It writes a minimal, honest entry: the version header + a short stub line (e.g. *"Maintenance release — no notable source changes"*). No hallucination, no hard error, no skipped entry — a no-op release still produces a truthful record.

One coherent family of "don't run the AI on a bad-sized diff" guards: too-big → fallback/abort per `on_notes_failure`; too-small/empty → stub, no AI.

##### Notes-path precedence

When multiple guards could apply to one run, mint resolves them in this order:

1. **First release** (no prior tag) → fixed body **"Initial release."**, no AI. Wins over everything below — there is no diff base, so `--no-ai`, `on_notes_failure`, and the degenerate check don't apply.
2. **Degenerate diff** (empty / all-excluded post-`diff_exclude`) → stub entry, no AI.
3. **`--no-ai`** (deliberate skip) → fallback body (commit-subject list), never aborts.
4. **Normal AI path** → generate + validate; failures route to `on_notes_failure`.

`on_notes_failure` governs only the normal AI path — steps 1–3 never call the AI, so they can't trigger it.

##### Change Map (diff-derived salience preamble)

The motivating failure — "glosses over the big feature on big releases" — is a **salience** problem (misallocated attention), not a missing-data problem; feeding *more* raw diff makes it worse. The fix is a computed **Change Map** that mint assembles (cheap git commands) and **prepends to the AI input**, telling the AI what to prioritize.

- **Structural novelty (primary signal):** new / removed / renamed paths — *especially new directories or packages appearing*. "A whole new `auth/` package showed up" is the strongest language-agnostic headline signal there is, and is qualitatively different from churn — a new subsystem is a headline even at modest line count, whereas a large refactor of existing code may not be. Weighted **above** raw magnitude, in both ordering and how the prompt is told to read it.
- **Magnitude (secondary signal):** per-area churn ranking, as supporting context ("400 lines here, 3 there").
- **Granularity — directory/area rollup by default**, with individually-notable files called out (new top-level entries, the single largest file). A flat list of every changed file is itself mush on big releases — the exact case this targets — so rollup is the salience-preserving form.
- **Computed after `diff_exclude`** (the map runs *after* exclusion, never before). Bulk noise is already removed, so post-exclude magnitude is largely trustworthy.
- **Prompt discipline (carries the diff-always-wins rule):** the prompt says **rank** importance using the Change Map but **describe** changes from the diff. The map is salience *metadata*, not content — the AI must never narrate a file as a feature merely because it's large or new.

**The AI input is therefore: the Change Map preamble, then the post-`diff_exclude` (and `max_diff_lines`-capped) diff — nothing else.**

##### Big-diff handling — deferred (documented escalation)

Ship **single-pass**: the whole (`max_diff_lines`-capped) diff + Change Map, which already injects salience within one pass — the cheap win. **Hierarchical summarisation** (per-area summarise-then-synthesize) is **parked, not built for v1** — the documented escalation if real big-release output still reads as mush. An intermediate lever (Change Map + a *trimmed* diff rather than falling back at the cap) is noted for the same future. Revisit only on observed need.

##### Failure behaviour — fail loud by default

Notes generate at Stage 4, *before* the tag (Stage 6), so aborting leaves nothing tagged/pushed — which is *why* blocking is safe.

- **`on_notes_failure`, default `abort`** — if the AI can't produce a body (missing tool, timeout, error, diff exceeds `max_diff_lines`, or a bad/empty generation that survives one retry), mint **fails loudly and tags nothing**. An empty/garbage release is worse than a failed command.
- **`fallback` mode (opt-in)** — proceed with a non-AI body instead of aborting. Fallback body defaults to the commit-subject list since the last tag; can be a fixed configurable string.
- **`--no-ai`** is a *deliberate* skip, not a failure → always uses the fallback body, never aborts.

##### Output format & validation

- **The AI returns the notes directly in presentation format** — no machine-parseable wrapper labels. mint uses the body **whole** for every sink; no parsing, no splitting, no per-sink reassembly.
- **Validation is sanity, not structure:** non-empty, not an error/refusal/whitespace. On a bad/empty generation → **one automatic retry** → still bad → notes failure → `on_notes_failure`. A **timeout is not retried** — it goes straight to `on_notes_failure` (retrying a hung call only risks a second full timeout); the single retry covers empty/error/refusal *content* only, bounding worst-case latency at one ~60s timeout.
- The interactive review gate (next section) is the human backstop for *style*.

##### Default notes format mint ships (anchored on Keep a Changelog)

The format anchors on the **Keep a Changelog** convention — its category taxonomy and principles (*"a changelog is for humans, not machines"*; *"a changelog is not a commit log"*) — rendered in **mint's emoji skin**. "Their meaning, mint's skin." This **refines** the emoji-headed-sections decision; it does not override it (it pins a fixed taxonomy *behind* the emoji).

- A **TL;DR one-liner** at the top (may be multi-line) — what the release is really about. This is mint's value-add over a raw changelog: a unified **cross-change narrative** synthesized from the *whole* diff (what beats regex/one-line-per-commit tools, which structurally can't see the whole release). Retained, sitting above the categorized sections.
- **Emoji-headed sections keyed to the Keep a Changelog taxonomy** — the canonical bucket set is `Added / Changed / Deprecated / Removed / Fixed / Security`, rendered with emoji headers (e.g. `✨ Added`, `🔧 Changed`, `🐛 Fixed`, `🗑️ Removed`). A *fixed, standard* taxonomy forces the AI to classify every change, and classification is itself prioritization — which helps the salience problem. Empty sections are omitted entirely.
- **Unit of entry = the notable item**, not the file / hunk / commit. The AI reads the whole diff, extracts notable items, and files each under its category. A change that adds a feature *and* fixes a bug yields two items in two sections — multi-category coverage falls out naturally.
- **Notable features bolded + described** (celebrated, not buried in a flat list).
- **Diff-inferability tiers the categories.** `Added / Changed / Fixed / Removed` are readable from a diff. `Deprecated` and `Security` are intent-laden and often invisible in a raw diff → kept in the vocabulary but treated as **opportunistic**: emit only on an *explicit textual marker* (a `@deprecated`/deprecation annotation; an obvious security surface — auth, crypto, input validation, a CVE-referencing dependency bump). Light prompt guidance, not detection machinery; expected to stay empty most releases. The deliberate escape hatch for diff-invisible intent is the **`[release].context` inject knob** — the user tells mint rather than mint guessing.
- **Strict "no preamble, no meta-commentary"** rule so prompt artifacts can never leak.
- Default prompt instructs the AI to **ignore version-number bumps** and other trivial bookkeeping churn.

The same body flows to every sink; in the changelog it sits under the `## [x.y.z] - date` header. *(Source confidence: the taxonomy/principles are firm; the exact emoji↔category mapping and prompt wording are explicitly ship-and-refine.)*

##### Prompt control — two knobs (no third "themes" concept)

1. **`[release].context`** (string or file) — *injects* project-specific guidance into mint's default prompt (e.g. "dev-workflow toolkit; emphasise user-facing changes"). The common case.
2. **`[release].prompt`** (file path) — *full override* of the prompt; mint still supplies the diff. Total control.

A "theme/variant" is not a separate feature — it's just a `[release].prompt` override file. `mint init` can scaffold an example prompt. No built-in theme enum (YAGNI).

##### Success criterion & verification

The quality bar for the notes is: **on a big release, the headline feature leads the notes.** The mechanism aimed at it is the Change Map (new-structure-as-headline, novelty over magnitude) plus forced Keep a Changelog classification. There is **no formal rubric or harness** — the user assesses output quality by eye on real releases (mint dogfoods itself; agentic-workflows, Portal, etc. are live cases) and tunes the prompt when the headline-leads criterion isn't met. Consistent with the "best effort, tune over time" posture.

---

#### Body Distribution: Tag vs Changelog vs Provider Release

The single notes body feeds three surfaces. mint **writes** all three but **reads** only one — the tag annotation.

##### What each surface carries

- **Tag annotation = subject `{commit_prefix} Release {tag}` + the FULL notes body** (default `commit_prefix` is 🌿). **Annotated** (not lightweight): signable, offline, in-repo, **immutable**. This is the **single source mint ever reads** — `regenerate --reuse` reads the annotation body via one deterministic git call (`git for-each-ref … contents:body`), no parsing.
- **CHANGELOG.md = a write-only projection** of the full body, under the `## [x.y.z] - date` header. mint *writes* it but **never reads** it.
- **Provider release (GitHub today) = a write-only projection** of the full body.

##### Optionality stack

| Surface | Optional? | Control |
|---|---|---|
| **Annotated tag** | **Mandatory** | always created, always carries a body — the floor and source of truth |
| **Provider release** | Optional | `publish` (default `true`) |
| **CHANGELOG.md** | Optional | `changelog` (default `true`) |
| **AI notes** | Optional | `--no-ai` / no AI → tag body falls back to a commit-subject / changed-files list, so the tag is never empty |

With `changelog = false` nothing durable is lost — the tag still holds the full notes.

##### Source-of-truth model

- The **tag is the immutable record of what shipped**. CHANGELOG + provider release are **mutable projections**.
- `regenerate --fresh` rewrites the **mutable** surfaces only; the tag is **never** rewritten (immutable history).
- `regenerate --reuse` always sources from the tag — deterministic, parse-free, config-independent.

**Trade accepted:** the full notes are duplicated in the tag *and* the changelog when both exist. Worth it for changelog-optionality, an always-present offline record, and parse-free healing.

##### Design history (why the body is whole, not split)

An earlier design had the tag carry a Summary/TL;DR only (full body deemed redundant given a CHANGELOG) and had the AI emit machine-parseable `## Summary` / `## Notes` labels so mint could split the TL;DR out. Once the CHANGELOG became optional, the tag had to become the single source of truth carrying the full body — so nothing splits anymore and the machine labels became vestigial and were removed. The current design: AI returns presentation-format notes, mint uses the body whole for every sink.

---

#### Stage 5 — Record (Changelog & Version Recording)

Persist the release into the repo: the CHANGELOG entry and the optional version-file projection, then build the commit graph leading to the tag.

##### Changelog mechanics

mint **owns** CHANGELOG generation (Keep a Changelog format). mint is a **generator, not a human-maintained changelog**, so it follows Keep a Changelog's *entry structure* but not its human-workflow conventions:

- **A complete `## [x.y.z] - YYYY-MM-DD` section** (the full notes body), written atomically at release time, inserted **above the most recent existing `## [` block**.
- **No `[Unreleased]` section.** KaC's `[Unreleased]` exists for humans accruing notes between releases; mint writes the finished version section directly, so it doesn't apply.
- **Newest on top** — the new version section is prepended **below the file's Keep a Changelog header preamble, above all prior version sections**.
- **First release** — if `CHANGELOG.md` doesn't exist, mint creates it with the standard Keep a Changelog header preamble first, then the first version section.
- **Idempotent by version key** — writing an *existing* version's section **replaces it in place** rather than appending a duplicate. This applies on the **regenerate path** (and amended releases), since the forward `mint release` path's tag-free preflight gate blocks re-running an already-released version before Record is reached.
- **Skipped entirely when `changelog = false`.**

##### Version-file projection

When `version_file` is configured, mint writes the new version into it (per `version_pattern`, or the whole file in plain mode). See Stage 1 for the strategy mapping.

- **`version_pattern` mismatch** (configured pattern matches nothing in the file) → **abort during Record, before the tag** (fail-loud, same family as a notes failure). Never silently skip the version write.
- **`version_pattern` multiple matches** → mint **replaces all occurrences** (carrying forward the legacy `sed`-replace semantics), keeping the file internally consistent rather than leaving stale copies of the old version.

##### Commit graph (up to two commits, then tag)

1. **Hook artifacts** (only if a `pre_tag` hook dirtied the tree) → their **own** commit: `chore(release): pre-tag artifacts for {tag}`. Kept separate because it's *project content* (e.g. a rebuilt knowledge bundle), semantically distinct from release bookkeeping.
2. **Release bookkeeping** — the CHANGELOG entry **and** the version-file projection folded into **one** commit: `{commit_prefix} Release {tag}` (subject uses the configurable `commit_prefix`, default 🌿). (The legacy script made three commits per release — needlessly noisy.)
3. **Annotated tag** points at the release-bookkeeping commit.
4. **`git push --atomic`** sends both commits + tag together — the single point of no return.

##### No-op safety

No empty commits — if the changelog yields no net change, or the version file already holds the target version, mint skips that commit.

---

#### Stages 6–7 — Tag, Push & Publish

##### Lock-resilient git

mint wraps **all** its git mutations in lock resilience (retry on a contended `.git` lock; clear a provably-stale lock). This carries forward the legacy `git_safe` behaviour as a built-in — tested once in Go, applied everywhere. A background agent/editor holding the index lock won't blow up a release.

##### Point of no return

`git push --atomic origin HEAD {tag}` is the **single point of no return** — commits + tag go up together or not at all.

##### Failure model

| When it fails | What mint does |
|---|---|
| **Before the push** (hook, notes, changelog, version, tag creation) | Everything mint did is local-only. mint **auto-unwinds its own mutations** — deletes the tag it made, resets the release commit(s) — returning the repo to the exact clean starting state. mint knows precisely what it created (N commits + 1 tag), so the unwind is surgical, and it reports what it undid. Next run starts clean. **Not configurable (YAGNI).** |
| **Push succeeds but provider release create fails** (e.g. transient network) | The tag is already public, so mint **never unwinds** (that would be destructive history rewriting). mint **warns** and points to the heal path: `regenerate --reuse` recreates the provider release from the tag annotation body (deterministic, parse-free). |
| **`post_release` hook fails** | **Warn only** — after the point of no return, the tag is already published. |

The auto-unwind is the same path taken when the user answers **no** at the `Continue?` review gate (see Interactive Review) — it includes any `pre_tag` hook-artifact commit. One mental model: *nothing mint did this run survives unless the release completes.*

##### Publishing: provider driver abstraction

Publishing the release is **first-class but provider-abstracted** — not hardcoded to `gh`, and **not a `post_release` hook**.

- **Not a hook** — a hook would reintroduce the copy-paste disease mint cures (every repo re-deriving `gh release create --notes … --verify-tag`) and would break heal/regenerate (the reuse path recreates the provider release, so mint must own it).
- **Behind a small `Publisher` interface** (`CreateRelease` / `UpdateRelease`). mint **auto-detects the provider from the remote host** (`github.com` → GitHub driver via `gh`), overridable by the `provider` config.
- **GitHub is the only driver implemented now.** The seam means GitLab (`glab`), Gitea, etc. can drop in later with zero rework — extra drivers are YAGNI; the *interface* is the cheap future-proofing.
- Config is provider-neutral: **`publish`** (default `true`; `false` = tag + push only) plus optional **`provider`** override. An unknown/unsupported `provider` *value* (a recognised key, e.g. `provider = "gitlab"` when only GitHub is implemented) is **not** a fail-loud config error — mint **warns loudly and downgrades to tag + push only** (publish skipped), so a typo can't silently vanish. Fail-loud config validation still applies to unknown *keys* and bad *types*.
- **Auto-detection with no matching driver** — a non-`github.com` remote (GitHub Enterprise, GitLab, Gitea, an unmatchable SSH host) or no remote at all, with `publish = true` (the default) and no explicit `provider` — is treated the **same as an unsupported value**: mint **warns loudly and downgrades to tag + push only**. It never silently assumes GitHub, and (because the `gh` gate runs only when actually publishing) never strands a pushed tag waiting on a release it can't create.
- The interface shape and auto-detection mechanics are routine Go, left to implementation.

##### Post-release: tap / formula update

The brew formula's version/sha bump is **downstream CI** reacting to the GitHub release mint creates — **not mint's job**. Most repos mint releases aren't formulas anyway. If a project ever wants mint to actively trigger it (`repository_dispatch`), that's a **`post_release` hook** — already supported by the hook system, no engine code.

---

#### Interactive Confirmation & Notes Review

The biggest live pain with the legacy script is that release notes go out *unseen* — no chance to review or edit before they're public. Notes are generated at Stage 4 (before any mutation / the point of no return), so there is a natural zero-risk window to review them.

##### Default interactive flow (before any mutation)

```
1. Plan summary + computed version  → shown
2. Notes generated + validated      → shown in full
3. Gate (Continue?):  [y] accept (default — Enter)   [n] abort   [e] edit   [r] regenerate with context
```

- **`y` accept** (default; a bare Enter accepts) → proceed to Record → tag → push.
- **`n` abort** → **full auto-unwind**: identical to the pre-push failure path — mint rolls back everything it made this run, including any `pre_tag` hook-artifact commit, returning to the exact clean starting state. The hook re-runs next time (idempotent build). A user-abort and a pre-push git failure are treated identically.
- **`e` edit** → opens the notes in the user's editor for real manual editing. **The saved text is used verbatim — no re-parse, no validation.** A human edit is trusted; structural validation only ever applied to untrusted AI output (which has no machine labels anyway). No mangle-loop, no possible trap. **Editor resolution:** `$VISUAL` then `$EDITOR`, falling back to a sensible default (`vi`); if no editor can be launched, mint reports the problem and returns to the gate rather than crashing.
- **`r` regenerate with context** → mint asks for a one-time context line, appends it to the prompt, re-runs the AI, and shows the result again (loops until happy). The "nudge it just this once" affordance — without permanently editing `[release].context`.

**On the no-AI notes paths** (first-release fixed body, degenerate stub, or `--no-ai` fallback — see Notes-path precedence), the gate offers only **`y` / `n` / `e`**: the AI-dependent **`r` regenerate is omitted**, since there is no AI invocation to nudge (and offering it under `--no-ai` would contradict the flag). To regenerate with the AI on a `--no-ai` run, the user re-runs without `--no-ai` rather than `r` silently overriding it. `y`/`n` are body-agnostic and `e` edits verbatim regardless of source, so only `r` is affected.

**Exact gate rendering** (the default-yes `Continue?` prompt, menu layout, and line-read input handling) is owned by the **CLI Presentation specification** (a cross-spec dependency); this section owns the four semantic choices and their effects.

##### Non-interactive

- **`-y` / `--yes`** skips the whole gate (uses notes as generated) for scripted/CI use.
- A config toggle to disable the gate can be added later if it ever annoys (YAGNI now).

This eliminates the "notes went out unseen / I had to fix the release afterward" pain entirely.

---

#### Dry-Run (`-d` / `--dry-run`)

##### Semantics

- **Does:** read-only preflight, compute the version, **generate the AI notes preview** (Change Map + diff → AI), and print the full plan — the commits it would make, the tag, and what it would publish.
- **Skips:** all mutations (commit / tag / push / provider release) **and all hooks** (they have side effects) — and **reports** that hooks were skipped.

##### Dry-run note caching

When `--dry-run` generates the notes preview, mint **caches it so the subsequent real run reuses it** — guaranteeing *what was previewed is what ships*. (Motivating workflow: an agent runs `mint release -y --dry-run` to surface the notes to a human, then `mint release -y` for real.)

- **The win is determinism, not cost.** AI generation is stochastic; regenerating on the real run risks shipping notes that differ from the preview. Reuse removes that risk. Skipping the second AI call is a bonus.
- **Activation is automatic.** The dry-run writes the note to the cache; the real run reuses it on a key match. No flag — the key-based invalidation makes automatic reuse safe, and it serves the motivating workflow transparently.
- **Cache key = hash of (post-`diff_exclude` diff + computed version + prompt / `[release].context`)** — not HEAD sha, since a `pre_tag` hook can change the tree between runs. **Miss → regenerate**, and say so ("diff changed since dry-run preview — regenerating notes"). mint never silently ships a stale note that no longer matches the release.
- **Interaction with `pre_tag` hooks:** because the key is the *post-`diff_exclude`* diff, it is **invariant to hook artifacts that fall under `diff_exclude`** (the normal case — build outputs like a generated bundle are excluded), so reuse holds even though dry-run skips hooks. If a `pre_tag` hook changes a **non-excluded** (real source) path, the dry-run (hook-skipped) and real (post-hook) diffs genuinely differ → the key **correctly misses and the real run regenerates** — which is right, since what would ship differs from the preview.
- **Re-review is unaffected.** A cached note does **not** skip the notes-review gate: an interactive real run still shows it (re-showing identical notes is cheap, and avoids assuming an out-of-band approval mint can't verify); `-y` still skips the gate on both runs. Reuse guarantees determinism; the review gate stays orthogonal.
- **Location:** a **gitignored cache** (e.g. `.mint/cache/`) or system temp, keyed by repo, **never committed**, with a **short TTL** backstop (default **~1 hour** — long enough for the dry-run→real-run handoff, short enough that a forgotten preview can't resurrect) so a stale preview can't be reused.

---

#### Regenerate / Backfill Notes (non-destructive)

mint can regenerate release notes for *existing* releases — one release, or every release in a batch backfill ("rewrite all of a repo's release history"). It is **non-destructive**: it touches only the mutable surfaces and never rewrites tags.

##### What's mutable vs permanent

- **Mutable (regenerate targets these):** the **provider release body** (GitHub today) and **`CHANGELOG.md`** — editable documents with no history consequence. mint re-diffs `vX-1..vX`, regenerates notes, and updates either or both, for one release or all. This is how you cleanly "rewrite release history": regenerate every version's provider release + rebuild `CHANGELOG.md`, touching no tags. ~95% of the visible value.
- **Permanent (excluded by default):** **tag annotations are git history.** "Rewriting" a tag means delete + re-create + force-push — destructive, and it breaks anyone who pulled. If ever built it would be a loud, explicit, opt-in-only escape (`--rewrite-tags`), strongly discouraged. **Not in scope now.**

##### Two-axis contract (source × target)

Regenerate has two independent axes plus scope, all leaving tags untouched:

**Axis 1 — source of notes:**
- **`--reuse`** — read the **tag annotation body** (the single source of truth; deterministic git read, no parsing, config-independent). No AI, no re-diff — can't drift.
- **fresh** (default) — re-diff `vX-1..vX` (with the same `diff_exclude` tiers + Change Map as the forward path) and re-run the AI for genuinely better notes.

**Axis 2 — target surface(s): `--target release | changelog | both`.**
- **`--reuse` ⇒ release-only.** Its source *is* the notes record, so "reuse → write changelog" would write a file from itself (a no-op) → mint **errors on `--reuse --target changelog`**.
- **fresh ⇒ release, changelog, or both.**
- **`--target changelog`/`both` when `changelog = false` → error** (fail-loud: "changelog is disabled in config"). mint never silently creates a CHANGELOG the project opted out of.
- **Canonical spelling is `--target <surface>`** (one flag, a value) — not separate `--release`/`--changelog` flags.

**Composition table:**

| Goal | Source | Target |
|---|---|---|
| Heal a failed publish | reuse | release |
| Clean up a legacy/bad release | fresh | both |
| Refresh public release text only | fresh | release |
| Rebuild a changelog entry only | fresh | changelog |
| Mass-heal missing provider releases | reuse `--all` | release |
| Full history rewrite | fresh `--all` | both |

##### Version argument & diff base resolution

- **`<version>` argument** may be given **with or without `tag_prefix`** (`regenerate v1.4.0` ≡ `regenerate 1.4.0`); mint normalises it. A `<version>` with **no matching tag** → **fail loud** ("no tag `vX.Y.Z` found").
- **Fresh diff base = `vX-1..vX`** (previous tag → target tag). For the **oldest release** (no predecessor tag — a single regenerate of the first release, or the first version in an `--all` backfill), there is no `vX-1`, so mint mirrors the forward path's first-release rule: **no AI, fixed body "Initial release."**
- **Argument presence:** a bare `regenerate` with **neither `<version>` nor `--all`** is an error ("specify a version or `--all`"); supplying **both** `<version>` and `--all` is also an error (mutually exclusive).
- **`--reuse` with no annotation body:** a tag not created by mint may be lightweight (no annotation) or carry an empty/whitespace body (`git for-each-ref … contents:body` returns empty). `--reuse` against such a tag is a **fail-loud error in single mode** ("tag `vX.Y.Z` has no annotation body — use `--fresh`"); in **`--all` mode** it is **skipped and reported** (consistent with batch skip-and-continue), never written as an empty release body.

##### Interactive by default, flags to skip

- `mint release regenerate <ver>` with **no flags → interactive**: asks source, asks target, shows the plan, confirms.
- **fresh** regeneration runs the same **notes-review gate** (see Interactive Review) before writing — backfilled notes are reviewable before they overwrite live surfaces.
- **reuse** is deterministic (no new notes) → a **simple confirm**, no review gate.
- Flags skip the questions but still confirm unless `-y`.
- **Fresh + `-y` requires an explicit `--target`.** Since `-y` skips the interactive target prompt and the fresh path has no default surface, `regenerate --fresh -y` (and `--fresh --all -y`) **without `--target`** is a fail-loud error ("`--target` is required with `--fresh -y`"). mint never guesses which live surface(s) to rewrite unattended. (`--reuse` is unaffected — it implies `--target release`.)

##### Batch `--all` semantics

- **Ordering: oldest → newest** (lets mint rebuild `CHANGELOG.md` in natural order).
- **Partial failure: skip-and-continue, summarise at the end** — *not* abort-the-batch (a single huge release tripping `max_diff_lines` shouldn't kill the others). This consciously overrides the single-version `on_notes_failure = abort` default; mint reports e.g. *"27 regenerated, 3 skipped: vX (diff too large), …"* so the user re-runs the stragglers. Skip-and-continue covers **per-version** failures only; **config-level** errors are validated **up front and abort the whole batch before it starts** — e.g. `--target changelog`/`both` with `changelog = false` (a static config fact, not a per-version condition) fails immediately rather than being skipped per version.
- **Review gates per version by default** (consistent with "notes never go out unseen"); **`-y`** is the opt-out to run fully unattended.
- **Re-runnable, no resume state.** `--reuse --all` (mass-heal from tags) is fully deterministic; `--fresh --all` re-generates (stochastic but harmless).

##### Preflight subset per verb

Preflight is a *gate set*; each command runs only the relevant subset (general rule: *calls `gh` → gh-auth; commits + pushes → clean-tree / branch / remote-sync; cuts a new tag → tag-free*):

- **`regenerate --reuse`** (release-only, no git mutation) → **gh-auth only** (it must run that — a dead `gh` auth is the usual reason you're healing).
- **`regenerate` fresh → changelog / both** (commits + pushes) → **gh-auth + clean-tree + branch + remote-sync**; **not** tag-free (tags exist, untouched); no version compute.

##### Write path (commit, push & recovery)

Regenerate's write mechanics mirror the forward path's principles, adapted to the fact that **no tag is ever cut** (tags are immutable):

- **Commit graph (changelog target).** A `--target changelog`/`both` run writes at most **one** CHANGELOG commit (hooks don't run on regenerate, so there is no hook-artifact commit). Subject: **`docs(changelog): regenerate notes for {tag}`** for a single version, or **`docs(changelog): regenerate release notes`** for an `--all` rebuild. It does **not** reuse the forward `{commit_prefix} Release {tag}` subject — nothing is being released.
- **Push form.** Plain **`git push origin HEAD`** (no tag, so not the forward path's `--atomic … {tag}`).
- **Provider target: create-or-update is automatic.** For `--target release`/`both`, mint **probes whether a provider release exists at `{tag}`** and dispatches `UpdateRelease` if it does, `CreateRelease` if it doesn't. The user never picks — "refresh existing text" and "mass-heal missing releases" are the same command, resolved per version (so an `--all` batch transparently mixes updates and creates).
- **Point of no return & recovery.** The changelog commit's **`git push` is the point of no return**, mirroring the forward model:
  - **Abort at the review gate** (fresh) or any **pre-push failure** → mint resets the local CHANGELOG commit, returning to the clean starting state (no tag is involved, so the unwind is just the commit reset).
  - **A provider create/update failure after the changelog push** → **warn only** (the changelog is already pushed; the provider release re-heals with `--target release`). Same post-PONR principle as the forward path.
  - **`--target both` is not atomic across surfaces:** mint writes the changelog (commit + push) first, then the provider release; a provider failure after the changelog push is the warn-only case above, not a rollback.
- **`--all` rebuild strategy.** An `--all` `--target changelog`/`both` run **rebuilds `CHANGELOG.md` whole** — regenerating the file from the Keep a Changelog preamble + every version's section, newest-on-top — rather than editing in place (whole-file rebuild is what "rebuild the changelog" means, and it repairs ordering/stray-section drift). A **single-version** regenerate uses the idempotent in-place section replace (per Stage 5). The batch makes **one** CHANGELOG commit + push **at the end** (after all per-version notes are generated/reviewed), not one per version — avoiding N noisy commits for a large backfill.

---

#### CLI Surface & Flags

##### Commands

```
mint release [bump] [options]                  cut a release   (shim `release` → `mint release`)
mint release regenerate <version> [flags]      regenerate notes for an existing release
mint release regenerate --all [flags]          backfill every version (oldest → newest)
mint init                                      scaffold .mint.toml (+ release shim)
mint version                                   print mint's own version
```

- **Bare `mint release` is the cut action** (not `mint release cut`). `release` is a command with a default action *and* subcommands (well-trodden, e.g. `git stash` = `git stash push`). The per-project shim is `release`, so `./release -m` maps cleanly to `mint release -m`.
- **`regenerate` is a subcommand of `release`**, not a top-level `notes` verb — `notes` is the wrong noun and ages badly.
- `mint version` / `mint --version` print mint's own version (standard convention).

##### `mint release` flags

```
-p, --patch          default when no bump flag is given
-m, --minor
-M, --major
    --set-version X.Y.Z   explicit version (mutually exclusive with bump flags)
-d, --dry-run
    --no-ai          deliberate skip → fallback body
    --autostash      stash/restore unrelated WIP around the run
    --any-branch     bypass the release-branch gate
-y, --yes            skip the confirmation + notes-review gate (scripted/CI use)
```

##### `regenerate` flags

```
--reuse              source = tag annotation body (no AI); implies --target release
--fresh              source = re-diff + AI (default)
--target release|changelog|both    surface(s) to write (default: asked interactively)
--all                every version, oldest → newest; per-version gates unless -y
-y, --yes            skip confirmation + per-version review gate
```

No flags → fully interactive (asks source, asks target, shows plan, confirms). See Regenerate / Backfill Notes for semantics.

##### Config Format & Schema

###### Format & location

- **Format: TOML.** Go-native, typed/validated with real error messages, supports comments, and `[table]`-style sections read cleanly. (YAML's indentation + type-coercion footguns aren't worth it.)
- **Location: `.mint.toml` at the repo root.** mint resolves the root via `git rev-parse --show-toplevel`, looks for `.mint.toml` there, and runs from root.
- **Fully optional.** Zero config = sensible defaults everywhere (tag-only release, `claude -p` notes, auto-derived release branch). `mint init` scaffolds a documented file.
- **Typed validation, fail-loud** on unknown keys / bad types, with clear messages.

###### Shape: shared engine keys + a table per verb

mint is multi-verb, so config is **verb-namespaced**: keys shared by every verb sit at the top level; each verb has its own table. This shape is adopted across the epic (it supersedes an earlier flat `notes_*` / `[hooks]` layout) — shared-vs-verb-specific becomes structural rather than inferred from prefixes, the verbs stay symmetric, and a future verb is a new table rather than more prefixes.

```toml
# Engine-level — shared by every verb
ai_command     = "claude -p"                      # default
diff_exclude   = ["skills/**/knowledge.cjs", "*.min.js"]
max_diff_lines = 50000

[release]
tag_prefix       = "v"             # default "v"
commit_prefix    = "🌿"            # default 🌿 — release commit + tag subject; cosmetic
release_branch   = "main"          # default: auto-derived from origin/HEAD
version_file     = "bin/tool"      # optional; omit = tag-only
version_pattern  = 'RELEASE_VERSION="{version}"'   # omit = whole file is the version
changelog        = true            # default true; false = no CHANGELOG.md projection
publish          = true            # default true; false = tag + push only
provider         = "github"        # optional; default auto-detected from remote host
on_notes_failure = "abort"         # abort | fallback
context          = "This is a dev-workflow toolkit; emphasise user-facing changes."  # inject into the notes prompt
prompt           = ".mint/notes-prompt.md"         # optional full prompt override

[release.hooks]
preflight    = "scripts/check.sh"
pre_tag      = ["npm ci", "npm run build"]
post_release = "scripts/notify.sh"
```

- **Shared engine keys (top level):** `ai_command`, `diff_exclude`, `max_diff_lines` — they serve every verb, so they sit above the tables.
- **`[release]` keys:** all release-specific config. `context` (the notes context-injection knob — formerly `notes_context`) and `prompt` (the full prompt override — formerly `notes_prompt`) are plain names; the table disambiguates them, so no `notes_` prefix is needed.
- **Hooks nest under their owning verb** as `[release.hooks]` (top-level is strictly shared-engine, so a top-level `[hooks]` would contradict that rule). Release owns `preflight` / `pre_tag` / `post_release`.
- **Other verbs add their own `[verb]` table** (e.g. a future `mint commit` gets `[commit]`), defined in that verb's own specification. This release spec specifies only the shared-engine keys and the `[release]` / `[release.hooks]` tables.

##### Global / presentation flags

- **`--plain`** — a **global presentation flag** (applies to every verb): forces token-efficient plain output instead of styled output. Detection model (`--plain` else `isatty(stdout)`) and rendering are specified by the **CLI Presentation specification** (a cross-spec dependency); listed here so the full invocation surface is visible in one place.

---

#### `mint init` Scaffolding

`mint init` activates mint in a project by dropping in **two files**:

1. **`.mint.toml`** — a **commented template**: the common keys with their defaults, plus optional keys (`version_file`, `[release.hooks]`, `[release].context` / `prompt`, …) present-but-commented with a one-line explanation each. The project tunes mint by uncommenting rather than reading docs.
2. **The `release` shim** — a tiny executable committed to the repo so `./release` works for anyone who clones. It execs `mint release "$@"`; if mint isn't installed it prints the install hint (`brew install leeovery/tools/mint`) and exits non-zero.

##### Behaviour

- **Idempotent / non-clobbering** — an existing `.mint.toml` or `release` is **skipped with a notice**; **`--force`** regenerates.
- **No hook/prompt files scaffolded by default** — the commented config shows hook examples inline, and a `[release].prompt` override file is only *mentioned* in a comment (not created).
- **No project auto-detection** — mint does **not** sniff `package.json`/etc. to pre-fill a build hook; that guesswork can surprise. A clean commented template is more honest. Deferred, addable later.

---

#### Dependencies

Prerequisites that must exist before implementation can begin:

##### Partial Requirement

| Dependency | Why Blocked | Minimum Scope Needed |
|------------|-------------|----------------------|
| **CLI Presentation specification** | The release run emits semantic events to a `Presenter` and shows the interactive notes-review gate through it; this spec defers gate *rendering* and the `--plain` flag's behaviour to that spec. The engine cannot render output or prompt the gate without the Presenter contract. | The **`Presenter` interface** (the event/method surface the engine calls) and the **notes-review gate rendering contract** (the `Continue?` default-yes mapping of the four semantic choices). Concrete `pretty`/`plain` renderers and `--plain` detection can land in parallel — the engine builds and tests against the interface (a fake/recording presenter). |

##### Notes

- **Everything else builds independently.** Version/tag logic, preflight gates, hooks, AI-notes context assembly + transport, the Change Map, record/changelog/version-file, tag/push/publish, regenerate, config parsing, and `mint init` have no cross-spec blocker — they sit behind the engine's own seams (`CommandRunner`, `Publisher`) and the shared config model, all owned by this spec.
- **`mint commit` is not a dependency** — it *consumes* the AI engine, config model, and Presenter that this spec (and the presentation spec) establish. Release is the producer, so it does not wait on commit.
- **Runtime tools** (`git`, `gh`, `claude`) are invoked behind the `CommandRunner` seam and are runtime prerequisites, not build/spec dependencies; the spec already defines graceful handling (preflight `gh` gate; optional/fallback `claude`).
- **Downstream tap/formula auto-update CI** reacts to the GitHub release mint creates — it is post-release, not a build dependency.

---

#### Working Notes

## 3. Commit Command

#### Specification

#### Overview

`mint commit` is a sibling verb to `mint release` that produces an AI-generated commit message from the diff, built into the `mint` binary. It is a **thin standalone verb**: it does NOT ride the release lifecycle spine. Its core act is small and local: stage (optionally) → generate a message → optionally review → commit → optionally push.

The guiding design stance: **there is no code yet.** The shared AI machinery is designed up front to serve both verbs cleanly — `commit` is not retrofitted onto release-note generation, nor does it require reworking any settled release decision.

#### Scope

**What `commit` reuses (shared primitives):**
- The `Presenter` seam — pretty/plain rendering, `-y` orthogonality, `--plain`, the `Continue?` review-gate rendering (defined in `cli-presentation`).
- The AI engine — transport `claude -p`, mint-owned prompt, fail-loud + retry, `--no-ai` skip (the three-layer engine, below).
- `git_safe` lock-resilient git.
- The TOML config model (verb-namespaced shape, below).
- `diff_exclude` globs and the `max_diff_lines` guard apply to commit's diff exactly as they apply to release's — we don't feed excluded files (bundles, lockfiles, minified output) into message generation.

**What `commit` does NOT touch:**
- Version detection, tags, changelog, publish/provider, the regenerate command.
- No point-of-no-return / atomic-push semantics — a commit is inherently local and reversible until pushed.

**Safety posture:** the inverse of release's. Release forces a known-good, clean, in-sync starting state because it is high-consequence; commit assumes a messy, in-progress working tree because operating on one is its entire reason to exist.

#### AI Engine — Three-Layer Split

The AI message-generation concern is a shared engine that both `release` and `commit` consume. The git boundary sits in a single layer:

- **Layer 1 — Context builder (git-aware).** Produces the content to describe. Parameterised by *source*: release uses `tag..HEAD`; commit uses the staged diff (`git diff --cached`). Applies `diff_exclude` globs and the `max_diff_lines` guard — identical logic for both verbs, so this is the genuinely shared git piece. (Different git *providers* are a separate axis, handled by the existing driver/provider setup, not by this layer.)
- **Layer 2 — AI message engine (git-unaware, content-agnostic).** Inputs: an assembled prompt + the content + `ai_command`. Runs the transport, validates the output (non-empty / not an error / not a refusal), retries once, fails loud per policy, and returns the message body. **Knows nothing about git, diffs, tags, or commits — pure "context in, message out."**
- **Layer 3 — Per-verb glue.** Picks the L1 source, supplies the prompt + default format (release notes vs commit message), and decides the sinks. This is where the verbs differ.

**Content-agnostic is the load-bearing property.** The input being a diff is incidental; L2 only ever sees "content." It does not matter whether that content is a textual diff, an AST/semantic breakdown, or a human-written note — same engine. This is what lets the separate release-notes-quality work enrich L1's *input* with zero change to L2.

**Composition is permitted.** Keeping L1/L2/L3 as separate underlying pieces does not forbid a convenience wrapper (a local or exported function) that composes L1→L2→sink for a call site's ergonomics. Separation governs the *underlying pieces*, not a tidy facade over them.

**Prompt boundary.** L3 owns prompt assembly; L2 receives the finished prompt. This mirrors release's settled model — mint always owns the prompt; `ai_command` is just transport — with the two-knob model (context-inject + full-override). Commit gets its own default commit-message prompt and its own context/override knobs (see Config).

##### Commit's binding to the engine
- **Layer 1 source:** the staged diff (`git diff --cached`), or the would-be-staged diff under `-a`/`-A` computed read-only (see Staging).
- **Layer 3 glue:** supplies the Conventional Commits default prompt/format, the `[commit]` context/override knobs, and the commit sinks (`git commit`, optional push).

#### Commit Flow / Lifecycle

The core invariant that shapes the whole flow: **mint mutates nothing until the user accepts the gate.** Everything before accept is read-only — including the `-a`/`-A` staging, which is deferred to the accept path. This is what makes abort a true no-op.

The stages:

1. **Preflight (minimal)** — git repo present; *something to commit* (for `-a`/`-A`, the would-be-staged changes; otherwise the existing index). Computed read-only. Empty → fail loud.
2. **Build context (L1)** — filtered diff of what *would* be committed (default: `git diff --cached`; with `-a`/`-A`: the would-be-staged working-tree diff, computed **without** mutating the index), with `diff_exclude` + `max_diff_lines` applied.
3. **Generate (L2)** — the commit message (skipped under `--no-ai`; fallback path covered under Message Format).
4. **Review gate** — the same `Continue?` rendering as release, interactive only (see Review Gate).
5. **On accept** — apply `-a`/`-A` staging now (if given), then `git commit` (via `git_safe`).
6. **Push (optional)** — only if `-p`/`--push` (flag-only, no config default) (see Auto-push).

**Reversibility:** no point-of-no-return / atomic-push semantics — a commit is local and reversible. Before accept, nothing has been mutated (clean abort). After accept, a completed commit is never unwound by mint (partial-failure model under Auto-push).

#### Staging Model

What goes into the commit. The design tension: the user's habit is `git add .` (which **includes new files**), but the natural "copy from git" flag `-a` maps to git's `commit -a`, which is **tracked-only**. Two faithful flags resolve the mismatch.

| Command | Modified tracked | Deleted tracked | New/untracked |
|---|---|---|---|
| `git commit -a` / `git add -u` | ✅ | ✅ | ❌ |
| `git add .` (from root) / `git add -A` | ✅ | ✅ | ✅ |

(mint runs from the repo root, so `git add .` ≡ `git add -A` for its purposes.)

**Decision — two faithful flags:**

- **Default = staged-only.** Commit the index exactly as staged. Respects deliberate staging; mint never decides *what* goes in unless asked.
- **`-a` / `--all` = `git commit -a`** — tracked modifications + deletions, no untracked. Muscle-memory faithful.
- **`-A` / `--add-all` = `git add -A` then commit** — everything including untracked. This is the user's `git add .` habit in one shot.
- **`-a` and `-A` are mutually exclusive.** Supplying both (`mint commit -aA`) is a conflicting-flags error → **fail loud** before any work (*"`-a` and `-A` cannot be combined; `-A` already includes `-a`'s changes"*). Consistent with the fail-loud posture for contradictory input — mint never silently picks a winner.
- **Staging is deferred to gate-accept.** With `-a`/`-A`, mint computes the would-be-committed diff *read-only* for message generation, and only runs `git add` after the user accepts. Aborting an `-a`/`-A` run leaves the index exactly as it was — mint never leaves a half-staged worktree behind.
- **Flags bundle:** `mint commit -Ap` = add-all + push with a minted message — the headline ergonomic target.

**Empty-staging handling — fail loud, mirroring git's messaging:**

- Empty staging (nothing to commit after staging) → **fail loud**; never invoke the AI on an empty diff. `-A`/`-a` that stage nothing land here too.
- Distinguish the two empty cases exactly as git does. **Which message fires is determined by the *actual* tree state after the requested staging mode, not by the flag passed:**
  - **Genuinely clean tree** (the chosen mode had nothing it could ever stage, and the index is empty) → *"nothing to commit, working tree clean"*. (E.g. `mint commit -A` on a pristine tree.)
  - **Changes exist but the chosen mode staged none** → guide the user — mint's flavour of git's `no changes added to commit` — naming the modes that *would* help:
    - Bare `mint commit` with unstaged changes → *"no changes staged — use `-a`/`--all`, `-A`/`--add-all`, or `git add`"*.
    - `mint commit -a` when the only changes are **untracked** (so tracked-only `-a` staged nothing) → point specifically at `-A`/`--add-all` (the mode that would include them), e.g. *"no tracked changes to stage — use `-A`/`--add-all` to include untracked files"*.

**Rationale:** the original commit shell function did not do its own `git add` (commit-only); the staging affordance is a deliberate enhancement in mint, not a port. The `git add .` habit (untracked included) is what tipped the choice to two flags — a faithful `-a` alone would silently drop new files, so the explicit `-A` covers the everything-case without overloading `-a`.

#### Commit Message Format & Prompt

L3 owns this prompt; it is simply a different default from release's.

**Default format = Conventional Commits 1.0.0** (the formal standard, conventionalcommits.org):
- `type(scope): description` subject line — imperative, concise; optional blank line + wrapped body for the *why*.
- Chosen because the user's own repos already use conventional commits (`discussion(...)`, `chore(...)`, `docs(...)`).
- **AI infers the `type`** (feat/fix/chore/docs/…) from the diff — central to the format and reliably inferable.
- **Scope off by default** — scope conventions are project-specific and the AI guesses them inconsistently; better omitted than wrong. (Re-enabling/guiding scope is a prompt/config affordance if ever wanted.)
- **Two-knob override**, mirroring release: a commit-specific context-inject knob + a full prompt-override knob (key names in Config). Same "mint owns the prompt; `ai_command` is transport" model.

**No mint branding in the message text.** Commit does **not** use release's `commit_prefix` (🌿) — a conventional-commit message is plain `type(scope): …`, and forcing a 🌿 onto every commit is undesirable. `commit_prefix` stays a release-only concern (release commit + tag subject).

##### The `$EDITOR` fallback — one degradation path for all "no AI message" cases

Three cases converge on dropping to `$EDITOR` with an empty/template message (behaving like plain `git commit`):

1. **`--no-ai`** — skip AI entirely; the user writes the message. No synthetic stub.
2. **AI generation failure** — if the AI errors or returns nothing usable after the engine's one retry, fall back to `$EDITOR` rather than abort. Low-stakes; the user is at the terminal anyway.
3. **`max_diff_lines` exceeded** — commit does **not** abort (release's notes-failure model is too harsh for a routine large commit). Fall back to `$EDITOR` with a clear note (*"diff too large to summarise — opening editor"*). `diff_exclude` still applies first, so excluded noise doesn't push a diff over the limit.

**Detection ordering for the oversized case.** `max_diff_lines` is evaluated at **L1**, after `diff_exclude` filtering and **before any L2 call**. An over-limit diff short-circuits L2 entirely — it is a generate-*skip* (like `--no-ai`), not a generate-*failure* — and routes straight to the `$EDITOR` fallback. The `-y`/non-TTY forbidden-combo check (below) then applies exactly as for `--no-ai`: an unattended run that hits the oversized fallback has no message source and **fails loud**. (The line-counting mechanics of `max_diff_lines` themselves are settled in the shared engine spec and reused; only commit's fall-back-rather-than-abort branch is commit-specific.)

(The detailed semantics of the `$EDITOR` path — TTY requirement, save-as-accept, regeneration failure routing — are specified next.)

#### `$EDITOR` Fallback — Path Semantics

The three "no AI message" cases (`--no-ai`, AI-generation failure, oversized diff) all drop to `$EDITOR`. This path is reconciled with the deferred-staging model, the gate, and the `-y`/non-TTY posture.

**Requires a TTY.** `$EDITOR` is inherently interactive. When a fallback fires under `-y` or non-TTY stdin (e.g. `mint commit -Apy --no-ai`, or `-Apy` when the AI fails / the diff is oversized), mint **fails loud** (*"no AI message and no interactive editor available"*) — it never hangs and never commits an empty message. This extends the gate's forbidden-combo philosophy (unattended + needs-human → fail loud) to the editor path. An unattended run with no message source is contradictory: `--no-ai` unattended has nothing to commit with, and an unattended user wants the AI anyway. **There is no `-m`/`--message` escape** — anyone needing unattended-with-own-message uses plain `git commit`; `mint commit` is for *minted* messages.

**Editor resolution (applies to *every* editor mint opens — both this fallback path and the gate's `e` action).** mint resolves which editor to launch using **git's own resolution order** (`GIT_EDITOR` → `core.editor` → `$VISUAL` → `$EDITOR` → git's built-in default), so it opens whatever `git commit` would open. `$EDITOR` being unset is therefore *not* by itself an error on a TTY — git's default still applies. mint opens the editor itself (rather than delegating to `git commit`) because staging must be deferred until the save-as-accept event.

When **no editor in the chain resolves to a launchable program**, behaviour depends on whether a message candidate already exists:
- **Fallback path (no message yet)** → fail loud, same as the no-TTY/`-y` case above — there is no message to fall back to.
- **`e` gate action (a message already exists)** → graceful degrade: warn that the editor could not be launched and re-render the `Continue?` gate with the **unedited** message preserved (treat `e` as a no-op). Consistent with "`e` is a refinement step that can never produce an empty commit."

**The editor save *is* the accept event.** On the fallback path the editor replaces the `Continue?` gate (git-like):

- **No separate `Continue?` gate.** The gate governs the *AI-generated* message only; the fallback path uses the editor itself as the review. A non-empty save = accept; quit/empty = abort. (This reconciles "`--no-ai` behaves like plain `git commit`" with "gate ON by default" — the gate is AI-path-only.)
- **Staging applies on save.** Same "stage on accept" rule, where *save* is the accept: the editor opens against the real (unstaged) state; only on a non-empty save does mint apply `-a`/`-A` staging, then commit. **`-p` push then runs as normal** (a non-empty save is a full accept, so `mint commit -Ap --no-ai` stages, commits, and pushes). Mutate-nothing-until-accept holds.
- **Empty/aborted editor = true no-op.** No staging applied, no commit, no push (even with `-p`). Nothing was mutated, so there is nothing to clean up.

**Regeneration failure routes here too.** If the user presses `r` (regenerate-with-context) at the gate and the regeneration fails after its one retry, mint treats it as any other AI failure → the `$EDITOR` fallback. One consistent rule: any failed AI generation lands at the editor. No special "re-show the prior message" path. (Under `-y`/non-TTY this is moot — `r` is an interactive-only gate action.)

#### Interactive Review Gate

Commit reuses the cli-presentation `Continue?` gate rendering (`y`/`n`/`e`/`r`, Enter ⇒ accept), **ON by default**.

**Choice mapping for commit:**

- **`y` / accept** → stage (if `-a`/`-A`) then commit; then push if `-p`.
- **`n` / abort** → do nothing. **No auto-unwind needed** — nothing has been mutated yet (staging deferred to accept), so abort is a true no-op back to the pre-`mint` state.
- **`e` / edit** → open the editor (resolved via the shared chain under Fallback Semantics → Editor resolution) pre-filled with the current message; on save, **return to the `Continue?` gate** with the edited message shown, used verbatim (no AI reprocessing). This follows the cli-presentation seam's loop-back contract — `e` re-renders the gate, it is *not* save-as-accept (only the fallback editor is save-as-accept). From the re-rendered gate the user may accept (`y`), abort (`n`), edit again (`e`), or regenerate (`r`). An empty save under `e` discards the edit and re-renders the gate with the **prior message preserved** — `e` is a refinement step, never a message source, so it can never produce an empty commit.
- **`r` / regenerate with context** → re-run the AI with a one-time context line. This *is* the "context injection" affordance from the user's original commit shell function. After `r`, mint prompts for a single free-text context line (rendered via the Presenter's line-read — the same input model as the gate); Enter submits. The line is injected as one-time context into the regeneration prompt and is **not persisted**. An empty line regenerates with no injected context (a plain re-roll). Regeneration runs the engine's normal one retry; failure routes to the `$EDITOR` fallback (per Fallback Semantics).

**Posture — gate ON by default.** Interactive runs show the message + `Continue?`; `-y` skips it (auto-accept); the shared forbidden-combo rule applies (non-TTY stdin + no `-y` → fail loud). Chosen for consistency with release and the presentation model, and because seeing the minted message before it sticks is the point. The frequent one-liner stays fast via `-y` (`mint commit -Apy`).

- **Considered and rejected — gate OFF by default** (commit immediately, review opt-in): faster for the frequent case, but commits messages unseen — the exact pain the gate exists to kill. `-y` already covers the unattended case explicitly.

**The gate-abort refinement (key design correction).** Originally the flow staged `-a`/`-A` *before* the gate, which meant aborting would leave a mint-altered worktree — wrong; "abort" must mean the whole run is abandoned with no trace. The fix — **mint mutates nothing until accept** (staging deferred) — is the cross-cutting property that runs through the lifecycle, staging, and the never-unwind invariant.

#### Auto-push Behaviour

- **Push is opt-in via `-p` / `--push`** (default: no push). **Flag-only — no config default** ("we never push without the `-p` flag"). `-p` is per-verb (release uses `-p` for `--patch`); the cross-verb `-p` divergence is intentional and acceptable (git subcommands carry their own flag meanings).
- **Push failure → keep the commit, warn clearly, do NOT unwind.** On a failed push (rejected, remote moved, no upstream, network), mint leaves the commit in place and reports clearly with the fix (re-run the push). Rationale: a push is a trivially repeatable manual fix, whereas unwinding the commit is messy and risky — the user may have had files staged before running `mint commit`, and resetting/unstaging could clobber that pre-existing state. Push is **not** an atomic point-of-no-return with unwind; it is a best-effort final step whose failure is reported, not repaired.
- **Upstream handling:** defer to git. `mint commit -p` runs a normal `git push` (current branch → its configured upstream). No upstream set → git's own failure, surfaced via the warn-clearly rule (*"commit is in place; set an upstream and push"*). mint adds no special upstream logic.
- **mint does not classify push-failure causes.** On *any* push failure (rejected, remote moved, no upstream, network) it emits **one generic warn** — the commit is in place; re-run the push to retry — with **git's own stderr passed through verbatim** beneath it, so git's specific hint (set-upstream, non-fast-forward, etc.) stays visible. The *"set an upstream and push"* line is illustrative of git's pass-through, not a mint-authored per-cause message. One rule for all causes: keep the commit, surface git's output, tell the user the commit is safe and the push is repeatable.

#### Invariant — *mutate nothing until accept; never unwind after*

The push-failure decision plus the gate-abort refinement give one coherent rule:

- **Before gate-accept, mint mutates nothing** — staging (`-a`/`-A`) is deferred to accept, so abort returns the user to their exact pre-`mint` state (their own prior staging untouched).
- **After accept, mint never unwinds a completed commit** — on a failed push it leaves the commit and reports clearly; it never unstages, resets, or rewrites.

This is the deliberate opposite of `mint release`'s auto-unwind model. The reason is the staging-safety concern: a local commit verb must never risk the user's working/staged state. There is **no destructive cleanup path at all** — failures either left nothing behind (pre-accept) or leave a clean forward-only commit the user can act on manually (post-accept).

#### Preflight & Safety

A commit is a frequent, low-stakes, *local* act — most of release's strict gates are actively wrong for it. Commit's preflight is minimal.

**Commit runs only:**

1. **Git repo present** — anchored at the repo root (same resolution as release).
2. **Something to commit** — after staging; empty → fail loud (see Staging).

**Gates commit deliberately DROPS (and why):**

- **Clean-working-tree — dropped.** Commit exists *to* operate on a dirty tree; the release gate is the direct opposite of commit's purpose.
- **On-release-branch — dropped.** Commits legitimately happen on feature branches all day.
- **Remote-in-sync (behind/diverged) — dropped.** You commit while behind origin constantly; blocking that would be absurd.
- **No pre-push gate even with `-p`.** Consistent with the auto-push decision — mint doesn't gate the commit on push-ability; it attempts the push and *reports* failure. No remote-sync precheck.

This makes commit's safety posture the inverse of release's: release forces a known-good, clean, in-sync starting state because it is high-consequence; commit assumes a messy in-progress tree because that is its entire reason to exist.

#### Config Schema

With mint now multi-verb, the config shape is **verb-namespaced tables + shared engine keys** (not flat-with-prefixes). Shared engine keys sit at the top *because* they serve every verb; each verb gets its own table.

```toml
# Engine-level — shared by every verb
ai_command     = "claude -p"
diff_exclude   = ["skills/**/knowledge.cjs", "*.min.js"]
max_diff_lines = 50000

[release]
tag_prefix       = "v"
commit_prefix    = "🌿"
release_branch   = "main"
changelog        = true
publish          = true
context          = "..."      # was notes_context
prompt           = "..."      # was notes_prompt
on_notes_failure = "abort"
# version_file, version_pattern, provider, ...

[commit]
context = "Conventional Commits; dev-workflow toolkit."   # inject into the commit prompt
prompt  = ".mint/commit-prompt.md"                        # full prompt override

[release.hooks]                                           # hooks nest under the owning verb
pre_tag = "npm ci && npm run build"
```

**Commit's config surface:**
- **Reused (shared) keys:** `ai_command`, `diff_exclude`, `max_diff_lines` — same values serve both verbs.
- **Commit-specific keys:** `[commit].context` (context-inject knob) and `[commit].prompt` (full prompt override). Both optional, typed, fail-loud — consistent with the existing config model.

**Hooks nest under the owning verb.** Top-level is strictly shared-engine, so a top-level `[hooks]` would contradict the "top-level = shared by every verb" rule. Commit defines **no** hook points (release owns `preflight`/`pre_tag`/`post_release`, mapped to its spine), so there is no `[commit.hooks]` today; it is the natural slot if commit ever gains hooks.

**Deliberately NOT added for commit:**
- No push config — push is flag-only `-p`, never a default.
- No `on_notes_failure` analogue — commit's failure path is always the `$EDITOR` fallback.
- No scope toggle, no per-verb `ai_command`/`max_diff_lines` override — steer via `[commit].context`/`prompt`; promote to a `[commit]` key only if a real need appears.

**Reconciliation owed by the release spec (cross-spec hand-off).** This verb-namespaced shape *revises* release's already-concluded flat config layout (`notes_context` → `[release].context`, `notes_prompt` → `[release].prompt`, `[hooks]` → `[release.hooks]`, every flat release key moves under `[release]`, shared engine keys lift to the top). Cheap now (no code exists). Commit's spec **depends on** that restructured shape; the migration itself is the release spec's to absorb (formalised in Dependencies).

#### CLI Surface & Flags

```
mint commit [flags]

  -a, --all          stage tracked changes first (git commit -a semantics)
  -A, --add-all      stage everything incl. untracked first (git add -A)
  -p, --push         push after committing (no push without this; no config default)
      --no-ai        skip AI; drop to $EDITOR
  -y, --yes          skip the review gate (auto-accept)
      --plain        plain output — global presentation flag, all verbs
```

**Bundles:** `mint commit -Ap` (add-all + push, gate shown) · `mint commit -Apy` (unattended).

`-p` = push is per-verb (release's `-p` = `--patch`); the cross-verb `-p` divergence is intentional and acceptable (git subcommands carry their own flag meanings).

**Resolved (consciously dropped):**

- **No `--dry-run`.** The review gate already *is* the preview-then-bail affordance (see the message, `n` aborts with zero mutation), and a commit is cheap to `--amend`. Release needs dry-run because it previews a whole irreversible pipeline; commit has no such pipeline.
- **No `--context` one-time-context flag.** The original shell function had it, but the user has never used it. Interactive `r` (regenerate-with-context) at the gate plus the `[commit].context` config cover the need. Dropped (YAGNI).
- **No `commit` shim.** `release` gets a per-project shim for muscle memory + `mint` delegation; `commit` is invoked directly as `mint commit` (the user aliases it personally if desired).

#### Dependencies

Prerequisites that must exist before implementation can begin:

##### Required

| Dependency | Why Blocked | What's Unblocked When It Exists |
|------------|-------------|--------------------------------|
| **CLI Presentation** (`cli-presentation` spec) | Commit renders *all* output and its review gate through the `Presenter` seam — pretty/plain by `isatty`/`--plain`, `-y` auto-accept, the `Continue?` gate rendering, and the shared non-TTY forbidden-combo rule. None of commit's interactive flow can be built without this seam. | The entire commit presentation surface: gate rendering, pretty/plain modes, `--plain`/`-y` handling, and the fail-loud forbidden-combo behaviour. |

##### Partial Requirement

| Dependency | Why Blocked | Minimum Scope Needed |
|------------|-------------|---------------------|
| **Mint Release Tool** (`mint-release-tool` spec) | Commit consumes the shared, content-agnostic AI engine and the verb-namespaced config — both established/restructured by the release spec. L2 (the engine), L1's `diff_exclude`/`max_diff_lines` logic, `git_safe`, and the `[commit]` config table cannot be built until these shared pieces exist in their reconciled form. | The shared AI engine (L1 context builder + L2 message engine), `git_safe` lock-resilient git, and the **verb-namespaced config restructure** (shared engine keys at top + per-verb tables, hooks nested under their verb). Commit does **not** depend on the release spine, version detection, tags, changelog, or publish. |

##### Notes

- **Build order:** CLI Presentation → Mint Release Tool (establishes engine, config, consumes Presenter) → Commit. Commit is the last of the three to be implementable because it reuses all of the shared primitives.
- **Designed clean, not retrofitted:** the three-layer engine split is *designed* in commit's discussion but is owed to the release spec as a reconciliation. Commit's L3 glue (Conventional Commits prompt, `[commit]` knobs, commit sinks) is the only engine-related code unique to commit; it can be written as soon as L1/L2 exist.
- **Config reconciliation is the release spec's to absorb:** commit only depends on the *result* (the verb-namespaced shape). It introduces no migration work of its own.
- **Gate-rendering reconciliation flows through:** the `Continue?` gate rendering commit consumes is the subject of an open reconciliation owed by the release spec (cli-presentation's `[a]/[q]`→`Continue?`, replacing the stale release gate keys). Commit inherits whatever that reconciled rendering becomes — it introduces no gate-rendering work of its own, but its review gate is only as settled as that reconciliation.

---

#### Working Notes

[Optional - capture in-progress discussion if needed]
