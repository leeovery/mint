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

---

## Working Notes

[Optional - capture in-progress discussion if needed]
