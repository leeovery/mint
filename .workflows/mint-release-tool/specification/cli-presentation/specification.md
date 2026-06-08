# Specification: CLI Presentation

## Specification

## Scope & Output Modes

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

## Render-Mode Detection & Output Streams

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

## The `Presenter` Seam (Architecture)

The presentation layer is structured as an **event/step-oriented `Presenter` interface** that the engine calls at lifecycle points. The engine emits *semantic events* ("stage X started", "here's the plan", "warn: hook failed"); the presenter decides *how they look*. This is shaped around the *process* `mint` runs, not around data structures.

**Illustrative method set** (exact surface settled at implementation):

```
StageStarted(name) · StageSucceeded(name) · StageFailed(name, err)
Warn(msg) · ShowPlan(plan) · ShowNotes(body) · Prompt(gate) → choice
```

**Decisions:**

- **Two implementations behind the interface — `pretty` and `plain`** — selected **once at startup** (`--plain` if passed, else `isatty(stdout)`). Nothing downstream re-checks the TTY or the flag.
- **The engine never touches colour, spinners, or TTY state.** It calls `Presenter` methods only. This mirrors the engine's existing dependency-inversion seams (`CommandRunner` for git/gh/claude, `Publisher` for releases) — the same discipline, now for output.
- **Applies to every verb.** `release`, `regenerate`, `init`, `version` (and future `commit`) all emit through the same `Presenter`. This is *how* "consistent presentation across all verbs" is met — structurally, via one interface, not per-verb styling code.
- **Testability** (the core Go rationale): assert which events fired and with what payload, independent of rendering. A `plain` impl is trivially assertable; a fake/recording presenter verifies engine behaviour without parsing styled text.
- **Spinners are a `pretty`-only concern** owned inside the pretty presenter (a spinner spans the gap between `StageStarted` and `StageSucceeded`/`StageFailed`); `plain` renders the same events as terse lines.

## Gating & `-y` Orthogonality

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

**Forbidden-combination rule (applies to any interactive gate):** if **stdin is not a TTY** and **`-y` was not passed**, `mint` **fails loud** ("not a TTY — pass `-y` to run unattended") rather than blocking on stdin. `-y` answers every gate.

**Gate input handling** (for the `Continue?` prompt):

- **Line-read** (type the letter, press Enter) — not raw single-keypress; no termios raw-mode complexity.
- **Empty line (just Enter) = default = accept.** The default fires only on a deliberate empty Enter.
- **Case-insensitive** (`N` = `n`).
- **Unrecognised key** (`x`, or old muscle-memory `a`/`q`) → **re-prompt**, never silently accept. Garbage never proceeds — keeps the destructive-adjacent default safe.

**Regenerate / edit re-entry:** after `e` (edit in `$EDITOR`) or `r` (regenerate-with-context), flow **loops back to the same `Continue?` gate** with the refreshed notes, until `y`/`n`. Rendering is **linear** — it re-prints the notes block + gate below (it scrolls; no screen-clearing or alt-screen).

## The Pretty Layer

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

## The Plain Layer

The `plain` presenter renders the same `Presenter` events as terse, token-efficient text — no animation, no glyphs, no colour. Only the **delimiters and stage narration** differ from pretty; the **notes body is byte-identical** in both modes.

**Contract:**

- **`key: value` lines, lowercase, one per stage on completion.**
- **Start line for long/blocking stages only.** A stage that blocks on something slow (AI **notes** generation, a `pre_tag` build hook) also emits a terse start line (`notes: generating…` → `notes: generated (1.1s)`), so a live-tail consumer (`mint release | tee log`, a streaming agent) isn't staring at silence through a multi-second wait. **Short stages stay one-line-on-completion** — no start line. This is plain's equivalent of the pretty spinner; the captured-log target gains only one extra line per long stage.
- **Stage terseness** as-is (e.g. `preflight: ok (clean, on main, tag free, in sync)`) — terse but human-legible, not pared further.
- **Notes block** delimited by plain rules: `--- release notes v{X} ---` … `--- end notes ---`, so a reader can slice it out reliably.
- **Notes body verbatim** — the same bytes as pretty/tag/changelog/release, **emoji headers shown if present** (`✨ Features`, `🐛 Fixes`). No stripping/transforming.
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

## Spinner Lifecycle (pretty only)

Spinners are a `pretty`-only concern, owned inside the pretty presenter. `plain` never animates — a stage emits exactly one line on its transition.

- **One spinner at a time, on the current stage line.** Starts on `StageStarted`, replaced in place by the `✓`/`✗` line on completion. Braille frames (`⠋⠙⠹…`). The spinner is part of the narration on stdout.
- **Underlying command output** (git/claude/gh chatter) is captured by `mint`, not streamed through the spinner line, so the animation can't be corrupted. On failure, `mint` prints the captured output below the `✗` line.
- **`$EDITOR` (note edit)** takes over the terminal — the spinner is stopped before handing off, resumed after.

## Library Selection

- **`lipgloss` for all `pretty`-mode styling** — colour, the 🌿 brand line, status glyphs, the titled notes rule. It is *pure string styling* (no event loop), so it composes with the `Presenter` seam, and it **auto-downgrades colour when piped** (also relied on for colour-incapable TTYs — see Render-Mode Detection).
- **A lightweight standalone spinner** for stage progress — e.g. `briandowns/spinner` (explicit `Start()`/`Stop()`, maps 1:1 to `StageStarted`/`StageSucceeded`) or charm's `huh/spinner`. The exact package is an implementation detail; the seam doesn't care.
- **NOT Bubble Tea / no alt-screen / no full-screen TUI.** Print-style linear narration only. A full TUI would fight the `Presenter` seam (Bubble Tea wants to own the event loop; `mint`'s engine drives and calls the presenter) and the dual pretty/plain requirement.
- **`plain` mode pulls in no UI library** — just `fmt` lines. That is the point of token-efficiency.

## Cross-Verb Rendering

The worked examples are all `mint release`, but the `Presenter` seam applies to every verb. All four verbs emit through the same `Presenter`; consistency is structural (one interface), not per-verb styling code.

- **`init`** — process narration in the same vocabulary: `✓ created .mint.toml` / `· skipped release (exists, use --force)`. No gate (non-clobbering).
- **`regenerate`** — same stage/notes/gate vocabulary as `release`, narrated per version (`--all` runs oldest→newest, one block each).
- **`version`** — the **one payload verb**: its output is a *value*, not narration. **Plain prints the bare value** (`1.4.0`) so `$(mint version)`/scripts consume it cleanly; **pretty may dress it** (`🌿 mint v1.4.0`). This is the deliberate exception to "narration is the product" — `version` actually has a payload, so the bare value is the floor and styling is additive only in pretty.

---

## Working Notes

[Optional - capture in-progress discussion if needed]
