# Discussion: CLI Presentation

## Context

`mint` is the reusable Go release tool extracted from per-project bash release scripts (see the decided `mint-release-tool` topic). This discussion covers its **presentation layer** — the styled-but-restrained UI applied consistently across *every* verb (`release`, `regenerate`, `init`, `version`, and the future `commit`), not only the release run.

The shape settled in discovery:

- **Interactive terminal**: brand + title, colour, and progress spinners while git and the `claude` CLI work.
- **Non-TTY (piped/redirected)**: degrades to token-efficient plain text, so an AI or agent consuming the output isn't paying for ANSI noise.
- **Detection**: render mode is driven by **TTY detection**, not environment sniffing.
- **`-y`/`--yes` is orthogonal**: it only skips interactive gate stops. A human at a terminal with `-y` still gets the styled UI.
- **Open how-question** (deferred to later phases): which Go packages provide styling and spinners — a charm/lipgloss-style stack vs lighter colour libraries.

### References

- [mint-release-tool discussion](../discussion/mint-release-tool.md) — the engine + lifecycle this presentation layer wraps

## Discussion Map

  Discussion Map — CLI Presentation (7 subtopics — 3 decided · 3 converging · 1 exploring)

  ┌─ ✓ Render-Mode Detection Model [decided]
  ├─ ✓ What The Pretty Layer Actually Shows [decided]
  ├─ → Plain / Token-Efficient Mode Contract [converging]
  ├─ → Spinners & Long-Running Progress [converging]
  ├─ ◐ -y/--yes Orthogonality [exploring]
  ├─ ✓ Presentation Seam / Architecture [decided]
  └─ → Library Selection (Charm Vs Lighter) [converging]

  *(Dry-run note reuse / caching was routed out to the `mint-release-tool` discussion — engine behaviour, not presentation. See its dry-run-semantics addendum.)*

---

*Subtopics are documented below as they reach `decided` or accumulate enough exploration to capture.*

---

## Render-Mode Detection Model

### Context

The seed mandates: render mode is driven by **TTY detection, not environment sniffing**. Interactive terminal → styled (brand, colour, spinners); non-TTY (an AI/agent consuming the output) → token-efficient plain text. The job here was to make "TTY detection" operationally precise, since `mint` is a **local interactive tool, not a CI job**, and `mint release` emits no capturable data payload — its output *is* the run narration.

### Decision (REVISED — grounded in the sibling `tick` tool's proven model)

**Two modes only — `pretty` + `plain`.** No structured (`json`/`toon`) mode: `tick` has one because it renders *data structures* (task lists); mint renders a *process*, so an agent reads the narration, it doesn't parse it. Structured output is YAGNI, addable later.
- **`pretty`** (human): brand, colour, spinners, formatted stages.
- **`plain`** (agent): terse token-efficient text — no ANSI, no animation, no banner.

**Detection — `isatty(stdout)` only. No override flags, no sniffing:**
- `isatty(stdout)` → `pretty`; non-TTY → `plain`. That is the *entire* mechanism — there is **no `--pretty`/`--plain`/`--no-color` override**. (The only run flag near this area is `-y`, which is gating, not rendering — orthogonal.)
- **"Human vs agent" reduces to "is stdout a terminal?"** — exactly `tick`'s mechanism (`stat(stdout).Mode() & os.ModeCharDevice != 0` on `os.Stdout`). An agent never announces itself; its harness captures stdout through a **pipe** (not a char device) → `false` → plain. A human's terminal is a char device → `true` → pretty. Same binary, same path; the OS reports what's connected for free. mint mirrors this (the stat check, or `term.IsTerminal(int(os.Stdout.Fd()))`).
- **No `CI=true`/`TERM` guessing** — the environment sniffing the seed forbids.
- **The two edge cases are features, not compromises** (confirmed desirable by the user): a human who pipes/redirects (`mint … > out.txt`, `… | grep`) gets `plain` — exactly what you want when capturing output (no ANSI junk), and this *is* the "force plain" path. An agent on a pseudo-terminal getting `pretty` is rare and harmless. There is deliberately no force-pretty.

**Stream split — narration is the product, so it's stdout:**
- **Run narration → stdout** — stages, the plan, the notes preview, the final summary, and `mint version`'s value. mint has no separate data payload, so the narration *is* its stdout output.
- **Errors + warnings → stderr** — the one real job stderr keeps: *visibility under redirection*. `mint release > run.log` sends stdout to the file, but a failure on stderr still hits the terminal and can't silently vanish.
- **Exit code** signals success/failure for scripts (they check `$?`, not stream parsing).
- An **agent captures combined output (`2>&1`)** by default, so it sees narration *and* errors regardless of the split — the split costs the agent nothing and buys humans redirect-visibility.

**No colour flag.** Colour is intrinsic to `pretty` and absent from `plain` — there is no `--no-color` and no `NO_COLOR` handling. Don't want colour? Pipe/redirect the output — any non-terminal stdout gives `plain`. Mode ∈ {pretty, plain}, full stop — no third "no-colour-but-styled" state, and no render-mode flags at all. (`NO_COLOR` support is addable later if anyone ever asks; YAGNI now.)

### Journey

Three course-corrections, each from the user:

1. **Initial framing assumed a payload-vs-chrome stdout/stderr split** where a human might do `mint release > notes.txt`. Wrong — mint emits no capturable release payload; it performs side effects and shows the notes-review gate interactively.
2. **First revision over-corrected to "all chrome → stderr, detect on stderr."** The review (F8) caught the tension: if everything meaningful is on stderr and stdout is empty, an agent capturing stdout gets near-silence.
3. **Resolved by reading `tick`** (the user's sibling CLI, which they already trust): it treats its rendered output as the product → **stdout**, detects on stdout, and reserves stderr for errors/`--verbose`. mint adopts the same stance. The "git/wget put progress on stderr" pattern exists to protect a *real* stdout payload mint doesn't have — copying it would be cargo-culting.

The model collapsed to: **one logical output, rendered by an adapter chosen by audience (auto via stdout TTY, override via flag); narration→stdout, errors→stderr.**

Confidence: high.

---

## -y/--yes Orthogonality

### Context

The seed: `-y/--yes` is orthogonal to styling — it only skips interactive gate stops; a human at a terminal with `-y` still gets the styled UI. Three independent concerns: **styling** (TTY), **gating** (`-y`), **output stream** (stdout/stderr).

### Decided so far

- **Three orthogonal axes**: styling = f(TTY), gating = f(`-y`), output stream = fixed (chrome→stderr, payload→stdout). A human with `-y` at a terminal still gets full styling.
- **The one forbidden combination errors, never hangs**: if **stdin is not a TTY** and **`-y` was not passed**, the notes-review gate (`[a]/[e]/[r]/[q]`) can't be answered — mint **fails loud** ("not a TTY — pass `-y` to run unattended") rather than blocking on stdin. Render mode is about *output* (stderr TTY); the gate is about *input* (stdin TTY) — both checked independently.

Still exploring: whether any other gates exist beyond notes-review that interact with `-y`.

---

## Presentation Seam / Architecture

### Context

The structural backbone the topic exists to define: how the engine and the presentation layer relate, so that mode selection, colour, and spinners live in exactly one place and the seven-stage release spine stays oblivious to them. `tick` gave a template — a `Formatter` interface with concrete per-mode impls behind a factory — but `tick`'s methods are **data-shaped** (`FormatTaskList(tasks)`) because it renders data structures. mint renders a **process**, so the seam must be shaped differently.

### Decision

**An event/step-oriented `Presenter` interface the engine calls at lifecycle points.** The engine emits *semantic events* ("stage X started", "here's the plan", "warn: hook failed"); the presenter decides *how they look*. Illustrative method set (exact surface settled at spec/impl):

```
StageStarted(name) · StageSucceeded(name) · StageFailed(name, err)
Warn(msg) · ShowPlan(plan) · ShowNotes(body) · Prompt(gate) → choice
```

- **Two implementations behind the interface — `pretty` and `plain`** — selected **once at startup** from `isatty(stdout)`. Nothing downstream re-checks the TTY.
- **The engine never touches colour, spinners, or TTY state.** It calls `Presenter` methods only. This mirrors the engine's existing seams (`CommandRunner` for git/gh/claude, `Publisher` for releases) — the same dependency-inversion discipline, now for output.
- **Applies to every verb.** `release`, `regenerate`, `init`, `version` all emit through the same `Presenter`, which is *how* the "consistent presentation across all verbs" goal is met structurally (not per-verb styling code).
- **Testability** (the whole Go rationale): assert which events fired and with what payload, independent of rendering. A `plain` impl is trivially assertable; a fake/recording presenter verifies engine behaviour without parsing styled text.
- **Spinners are a `pretty`-only concern** owned inside the pretty presenter (e.g. a spinner spans the gap between `StageStarted` and `StageSucceeded/Failed`); `plain` renders the same events as terse lines. Lifecycle detail deferred to the spinners subtopic.

### Journey

Considered cloning `tick`'s data-shaped `Formatter` directly, but mint has no task-list-equivalent to format — it has a running process. Reshaping the seam from "format this data" to "react to this event" keeps the decoupling principle (one interface, per-mode impls, factory by mode) while fitting what mint actually does. The engine-emits-events / presenter-renders split is the payoff: the spine is testable and rendering-agnostic.

Confidence: high (on the split and the event shape; exact method signatures are spec/impl detail).

---

## Library Selection (Charm vs lighter)

### Context

The discovery how-question: which Go packages provide styling and spinners — a charm/lipgloss stack vs lighter colour libs. Grounded by two sibling Go tools the user already runs: `tick` (uses lipgloss-style formatting) and `portal`.

### Decision

- **`lipgloss` for all `pretty`-mode styling** — colour, the 🌿 brand line, status glyphs, the notes box/border. It is *pure string styling* (no event loop), so it composes with the `Presenter` seam, and it auto-downgrades colour when piped. Idiomatic and already in the user's toolchain.
- **A lightweight standalone spinner** for stage progress — `briandowns/spinner` (explicit `Start()`/`Stop()`, maps 1:1 to `StageStarted`/`StageSucceeded`) or charm's `huh/spinner`. Exact pick is an impl detail; the seam doesn't care.
- **NOT Bubble Tea / no alt-screen / no full-screen TUI.** Print-style linear narration only.
- **`plain` mode pulls in no UI library** — just `fmt` lines. That's the point of token-efficiency.

### Journey — why not Bubble Tea / Portal's stack

`portal` is a full-screen Bubble Tea TUI (`tea.NewProgram(tea.WithAltScreen())`, a `Model`/`Update`/`View` state machine with pages and modals). Inspected as a candidate baseline and **rejected for mint**: it's built for an interactive picker that *owns the screen*, whereas mint narrates a linear process and exits. A full TUI would also fight the `Presenter` seam (Bubble Tea wants to own the event loop; mint's engine drives and calls the presenter) and the dual pretty/plain requirement. We take **lipgloss** (the styling layer) from that ecosystem and leave the TUI runtime. Note: `portal` and `tick` both detect TTY with the same `os.Stdout.Stat() & ModeCharDevice` check we locked — three-for-three.

Confidence: high on the stack (lipgloss + standalone spinner, no Bubble Tea); specific spinner package is impl detail.

---

## What The Pretty Layer Shows / Plain Contract (worked examples)

### Context

The user asked for **concrete logged examples** so the implementer isn't guessing the look-and-feel. Both modes consume the *same* `Presenter` events; only rendering differs. Baseline below — refinable at implementation, but the intent is fixed.

### Baseline renderings (per event)

| Event | `pretty` | `plain` |
|---|---|---|
| start of run | `🌿 mint · {project}  ›  releasing v{X}` brand line | `mint: releasing {project} v{X}` |
| `StageStarted` | dim line with spinner: `⠋ notes  generating with claude…` | (no line until transition) |
| `StageSucceeded` | `✓ {stage}  {detail} ({elapsed})`, glyph green | `{stage}: ok` / `{stage}: {detail}` |
| `StageFailed` | `✗ {stage}  {message}`, glyph red | `{stage}: FAILED - {message}` (also stderr) |
| auto-unwind | `↩ unwound  {what it undid} — repo clean` | `unwound: {what}; repo clean` |
| `Warn` | `⚠ {label}  {message}`, amber | `{label}: WARN - {message}` (also stderr) |
| `ShowPlan` | a `Plan` block, bulleted | `plan: {semicolon-joined one-liner}` |
| `ShowNotes` | titled rule: `── release notes · v{X} ──` body, closing `────` rule (no box) | `--- release notes v{X} ---` … `--- end notes ---` |
| review gate | vertical menu (`y accept [default]` / `n abort` / `e edit` / `r regenerate`) then `Continue? › ` prompt; Enter ⇒ `y` | (not shown — non-TTY ⇒ `-y` required ⇒ gate skipped; emits `notes: accepted (-y)`) |
| end of run | `🌿 released {project} v{X} · {url}` | `done: {project} v{X} {url}` |

### Full `pretty` run

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

`pretty` failure + auto-unwind, and a post-release warn:

```
  ✗ tag/push   push rejected: remote moved
  ↩ unwound    removed tag v1.4.0, reset 2 release commit(s) — repo clean

  ⚠ post_release  hook failed (tag is already published): scripts/notify.sh exited 1
```

### Same run in `plain` (agent, `-y`)

```
mint: releasing acme v1.4.0
version: v1.3.2 -> v1.4.0 (minor)
preflight: ok (clean, on main, tag free, in sync)
plan: commit changelog+version; tag v1.4.0; push --atomic; publish github
prep: pre_tag ok (2.3s)
notes: generated (1.1s)
--- release notes v1.4.0 ---
Faster cold starts and a calmer log.

Features:
- Parallel warm-up halves boot time
Fixes:
- Stop double-flush on SIGTERM
--- end notes ---
notes: accepted (-y)
record: CHANGELOG.md + bin/acme
tag/push: v1.4.0 pushed (atomic)
publish: github release created
done: acme v1.4.0 https://github.com/acme/acme/releases/tag/v1.4.0
```

`plain` failure:

```
tag/push: FAILED - push rejected: remote moved
unwound: removed tag v1.4.0, reset 2 commits; repo clean
```

### Spinner lifecycle (resolves the spinners subtopic)

- One spinner at a time, on the **current** stage line (stderr-independent — it's part of the narration on stdout). Starts on `StageStarted`, replaced in place by the `✓`/`✗` line on completion. Braille frames (`⠋⠙⠹…`).
- **Underlying command output** (git/claude/gh chatter) is captured by mint, not streamed through the spinner line, so the animation can't be corrupted. On failure mint prints the captured output below the `✗` line.
- **`$EDITOR` (note edit)** takes over the terminal — spinner is stopped before handing off, resumed after.
- **`plain` never animates** — a stage emits exactly one line on its transition.

### Decisions locked (pretty layer)

The **pretty** half is decided; the **plain** contract below stays converging until walked separately.

- **Brand lines** — `🌿 mint · {project}  ›  releasing v{X}` (top) and `🌿 released {project} v{X} · {url}` (bottom). Leaf ties to the engine `commit_prefix` brand.
- **Status glyphs** — `✓` success (green) · `✗` failure (red) · `⚠` warn (amber) · `↩` auto-unwind. Spinner frames `⠋⠙⠹…`.
- **Stage lines** — two-space indent, glyph, stage name padded to a column, terse detail. Brand lines flush-left; everything else indented under them. Symmetry/consistency is the bar — no ad-hoc indentation.
- **Release notes** — **no box.** A titled `── release notes · v{X} ──` rule, the body verbatim, a closing `────` rule. Dropped the rounded box: it forced wrap/truncate on arbitrary-width AI notes and read as clutter.
- **Review gate** — a vertical menu, **options above the question**, `[default]` next to its action, prompt last:

  ```
    y  accept & proceed [default]
    n  abort
    e  edit in $EDITOR
    r  regenerate

  Continue? › 
  ```

  **Enter ⇒ `y`** (accept & proceed — the 99% path). `n` ⇒ abort (full auto-unwind, per the engine discussion). `e` ⇒ `$EDITOR`; `r` ⇒ regenerate-with-context.

- **`-y` alignment** — `-y` is a *yes* flag: it answers this `Continue?` gate `yes` unattended — identical outcome to pressing Enter. Reinforces the orthogonality model (gating = f(`-y`)).

**Cross-ref / reconciliation needed:** this **revises the engine discussion's** "Interactive confirmation & notes review" gate, which documented keys as `[a] accept / [e] edit / [r] regenerate / [q] abort`. Semantics are unchanged (same four choices, auto-unwind on abort) — only the *rendering* changes: default-yes `Continue?` instead of explicit accept, `n` instead of `q`. The in-progress `mint-release-tool` spec should reconcile the two surfaces (presentation owns the rendering; engine owns the four semantic choices).

Confidence: high on the pretty layer (brand, glyphs, stage shape, no-box notes, gate rendering); plain-mode verbosity still to confirm in its own subtopic.

---

## Summary

### Key Insights

1. **mint's narration IS its output.** No separate data payload (bar `mint version`), so narration → stdout, and stderr is reserved for errors/warnings (kept only for redirect-visibility). "All process" is fine — when the narration is the product, the narration is stdout.
2. **Mirror `tick`'s adapter model** — one logical output, rendered by an adapter chosen by audience: `isatty(stdout)` → pretty, else plain; explicit flag overrides. Proven in a sibling tool the user already trusts.
3. **Two modes suffice** (pretty + plain). Structured json/toon is YAGNI here because mint renders a process, not a queryable data structure.
4. **Three orthogonal axes**: styling (TTY) · gating (`-y`) · output stream. Independence is the design's backbone.
5. **Engine emits events; presenter renders.** An event-oriented `Presenter` seam (not tick's data-shaped `Formatter`) keeps the seven-stage spine oblivious to colour/spinners/TTY — mirroring the `CommandRunner`/`Publisher` seams — and is how "consistent across all verbs" is achieved structurally.
6. **No render-mode flags.** Mode is purely `isatty(stdout)`; piping is the natural "force plain". Minimal surface by design.

### Open Threads

- **Dry-run note reuse / caching** — **routed out** to the `mint-release-tool` discussion (engine/dry-run behaviour, not presentation) as an addendum to its dry-run-semantics decision, so the in-progress spec picks it up. The idea: cache the dry-run-generated note and reuse it on the real run to guarantee *what was previewed is what ships* (determinism), with invalidation keyed on the post-exclude diff + version + prompt. Resolved here; decided in spec.
- Library selection was flagged in discovery as a deferred how-question.

### Current State

- Render-mode detection **decided** (stdout-based, tick-aligned, two modes, narration→stdout, no flags). Presentation seam **decided** (event-oriented `Presenter`, two impls). `-y` orthogonality mostly decided. Remaining: what the pretty layer shows, the concrete plain-mode text contract, spinners, library selection.
