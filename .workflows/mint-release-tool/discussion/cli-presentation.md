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

  Discussion Map — CLI Presentation (7 subtopics — 7 decided)

  ┌─ ✓ Render-Mode Detection Model [decided]
  ├─ ✓ What The Pretty Layer Actually Shows [decided]
  ├─ ✓ Plain / Token-Efficient Mode Contract [decided]
  ├─ ✓ Spinners & Long-Running Progress [decided]
  ├─ ✓ -y/--yes Orthogonality [decided]
  ├─ ✓ Presentation Seam / Architecture [decided]
  └─ ✓ Library Selection (Charm Vs Lighter) [decided]

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

**Detection — `--plain` override, else `isatty(stdout)`. No sniffing:** *(AMENDED — see Journey course-correction 4)*
- Precedence: **`--plain` passed → `plain`**; otherwise `isatty(stdout)` → `pretty`, non-TTY → `plain`. A flag is an explicit *instruction*, not a guess — the ban is on *sniffing* (`LANG`/`LC_*`/`TERM`/`CI`), which still stands. (The only other run flag near this area is `-y`, which is gating, not rendering — orthogonal.)
- **`--plain` only — no `--pretty` (YAGNI).** Force-*plain* has real demand (a UTF-8/braille-incapable TTY that would garble the spinner/emoji; or just wanting clean output without piping). Force-*pretty* doesn't: a real terminal already gets pretty, and mint is "a local interactive tool, not a CI job," which kills the coloured-CI-logs case. Addable later if anyone asks.
- **This resolves the terminal-capability gap (review-002 F6).** Pretty mode assumes UTF-8/braille; rather than build locale-capability detection (which *is* the banned sniffing) and a fallback glyph set, a broken-glyph terminal is the user's cue to pass `--plain`. mint never self-degrades.
- **Cross-ref:** `--plain` is a new **global/presentation flag** (applies to every verb, like the `Presenter` itself), distinct from the per-verb engine flags. The in-progress `mint-release-tool` spec's CLI surface should record it alongside `-y`.
- **"Human vs agent" reduces to "is stdout a terminal?"** — exactly `tick`'s mechanism (`stat(stdout).Mode() & os.ModeCharDevice != 0` on `os.Stdout`). An agent never announces itself; its harness captures stdout through a **pipe** (not a char device) → `false` → plain. A human's terminal is a char device → `true` → pretty. Same binary, same path; the OS reports what's connected for free. mint mirrors this (the stat check, or `term.IsTerminal(int(os.Stdout.Fd()))`).
- **No `CI=true`/`TERM` guessing** — the environment sniffing the seed forbids.
- **The two edge cases are features, not compromises** (confirmed desirable by the user): a human who pipes/redirects (`mint … > out.txt`, `… | grep`) gets `plain` — exactly what you want when capturing output (no ANSI junk), and this *is* the "force plain" path. An agent on a pseudo-terminal getting `pretty` is rare and harmless. There is deliberately no force-pretty.

**Stream split — narration is the product, so it's stdout:**
- **Run narration → stdout** — stages, the plan, the notes preview, the final summary, and `mint version`'s value. mint has no separate data payload, so the narration *is* its stdout output.
- **Errors + warnings → stderr** — the one real job stderr keeps: *visibility under redirection*. `mint release > run.log` sends stdout to the file, but a failure on stderr still hits the terminal and can't silently vanish.
- **Exit code** signals success/failure for scripts (they check `$?`, not stream parsing). **Ownership (resolves review-003 F3): the engine, not the `Presenter`.** The presenter is render-only — it reacts to events and has no say in process status. The engine/`main` knows the run outcome (success / pre-push failure + unwind / post-push warn) and sets the exit code independent of rendering. Mentioned here only because the stream contract touches it; **routed to the engine/spec** (same disposition as the dry-run caching thread).
- An **agent captures combined output (`2>&1`)** by default, so it sees narration *and* errors regardless of the split — the split costs the agent nothing and buys humans redirect-visibility.

**No separate colour flag.** Colour is intrinsic to `pretty` and absent from `plain` — there is no `--no-color`. Don't want colour? **`--plain`** (or pipe/redirect — any non-terminal stdout gives `plain`). Mode ∈ {pretty, plain} at *mint's* level, full stop — no third "no-colour-but-styled" state. `NO_COLOR` env handling stays YAGNI — it's sniffing, and `--plain` is the explicit equivalent.

**Colour-incapable terminal — rely on lipgloss's own downgrade (resolves review-003 F2).** A real-but-colour-incapable TTY (`TERM=dumb`, etc.) is still selected as `pretty` by `isatty(stdout)`. We **lean into lipgloss's built-in colour auto-downgrade**: it emits no colour codes there while keeping layout and glyphs. That is *not* a third mint mode — mint still offers exactly pretty/plain; the styling library behaving correctly underneath is in-scope and free. The "no third state" rule constrains *mint*, not lipgloss internals. (Glyph/UTF-8 incapability is a separate axis — that's the `--plain` escape; colour incapability is handled for free.)

### Journey

Four course-corrections, each from the user:

1. **Initial framing assumed a payload-vs-chrome stdout/stderr split** where a human might do `mint release > notes.txt`. Wrong — mint emits no capturable release payload; it performs side effects and shows the notes-review gate interactively.
2. **First revision over-corrected to "all chrome → stderr, detect on stderr."** The review (F8) caught the tension: if everything meaningful is on stderr and stdout is empty, an agent capturing stdout gets near-silence.
3. **Resolved by reading `tick`** (the user's sibling CLI, which they already trust): it treats its rendered output as the product → **stdout**, detects on stdout, and reserves stderr for errors/`--verbose`. mint adopts the same stance. The "git/wget put progress on stderr" pattern exists to protect a *real* stdout payload mint doesn't have — copying it would be cargo-culting.
4. **Reintroduced a single `--plain` override.** An earlier pass had dropped *all* render-mode flags (auto-detect only). The terminal-capability gap (review-002 F6 — a TTY that can't render UTF-8/braille still gets pretty and garbles glyphs) reopened it: the alternative was capability *sniffing* (the thing we banned) plus a fallback glyph set. A `--plain` flag is the explicit, no-sniff escape hatch — and it doubles as the discoverable form of "pipe to force plain." `--pretty` stayed out (YAGNI). Net: auto-detect by default, one explicit override.

The model collapsed to: **one logical output, rendered by an adapter chosen by audience (auto via stdout TTY, single `--plain` override); narration→stdout, errors→stderr.**

Confidence: high.

---

## -y/--yes Orthogonality

### Context

The seed: `-y/--yes` is orthogonal to styling — it only skips interactive gate stops; a human at a terminal with `-y` still gets the styled UI. Three independent concerns: **styling** (TTY), **gating** (`-y`), **output stream** (stdout/stderr).

### Decision

- **Three orthogonal axes**: styling = f(`--plain` else TTY), gating = f(`-y`), output stream = fixed (chrome→stderr, payload→stdout). A human with `-y` at a terminal still gets full styling; `--plain` drops styling without touching gating.

**Gate inventory (resolves review-002 F1)** — every verb walked for interactive stops, not just notes-review:

| Verb | Interactive gate? | Under `-y` |
|---|---|---|
| `release` | **Yes** — the `Continue?` notes-review gate (also confirms the plan) | answers `yes` |
| `regenerate` | **Yes** — interactive *source* + *target* prompts, then the notes-review gate (fresh) / a simple confirm (reuse) | uses flags/defaults, auto-accepts |
| `init` | **No** — non-clobbering (skips existing with a notice; `--force` to overwrite) | n/a |
| `version` | **No** — prints its value | n/a |
| `commit` (future) | out of scope — separate feature | — |

Two gating verbs (`release`, `regenerate`). `init`'s safety is **structural** (non-clobber + `--force`), not a prompt — which is why it never needed `-y`.

- **Generalised forbidden-combination rule (was only stated for notes-review)**: for **any** interactive gate, if **stdin is not a TTY** and **`-y` was not passed**, mint **fails loud** ("not a TTY — pass `-y` to run unattended") rather than blocking on stdin. Render mode is about *output* (stdout TTY); a gate is about *input* (stdin TTY) — checked independently. `-y` answers every gate.

**Gate input handling (resolves review-002 F3)** — for the `Continue?` prompt:
- **Line-read** (type the letter, press Enter) — not raw single-keypress; no termios raw-mode complexity.
- **Empty line (just Enter) = default = accept.** The default fires *only* on a deliberate empty Enter.
- **Case-insensitive** (`N` = `n`).
- **Unrecognised key** (`x`, or old muscle-memory `a`/`q`) → **re-prompt**, never silently accept. Keeps the destructive-adjacent default safe — garbage never proceeds.

**Regenerate / edit re-entry (resolves review-002 F7)** — after `e` (edit in `$EDITOR`) or `r` (regenerate-with-context), flow **loops back to the same `Continue?` gate** with the refreshed notes, until `y`/`n`. Rendering is **linear — re-prints the notes block + gate below** (it scrolls; no screen-clearing or alt-screen, consistent with the no-Bubble-Tea print model).

Confidence: high.

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
| `StageStarted` | dim line with spinner: `⠋ notes  generating with claude…` | (blank for short stages; long/blocking stages emit a terse start line, e.g. `notes: generating…`) |
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

`plain` failure:

```
tag/push: FAILED - push rejected: remote moved
unwound: removed tag v1.4.0, reset 2 commits; repo clean
```

### Decisions locked (plain layer)

The **plain** contract is decided:

- **`key: value` lines**, lowercase, **one per stage on completion** — no animation, no glyphs, no colour.
- **Start line for long/blocking stages only (resolves review-003 F1)** — a stage that blocks on something slow (AI **notes** generation, a `pre_tag` build hook) also emits a terse start line (`notes: generating…` → `notes: generated (1.1s)`), so a **live-tail** consumer (`mint release | tee log`, or a streaming agent) isn't staring at silence through a multi-second wait. **Short stages stay one-line-on-completion** — no start line. This is plain's one-line equivalent of the pretty spinner; the captured-log target is unaffected (one extra line per long stage).
- **Stage terseness** confirmed as-is (e.g. `preflight: ok (clean, on main, tag free, in sync)`) — terse but human-legible; not pared further.
- **Notes block** delimited by plain rules: `--- release notes v{X} ---` … `--- end notes ---`, so a reader can slice it out reliably.
- **Notes body is verbatim** — the same bytes as pretty/tag/changelog/release, **emoji headers shown if present** (`✨ Features`, `🐛 Fixes`). No stripping/transforming: it would contradict the engine's "use the body whole" rule and break "what previews is what ships." The few extra tokens are negligible.
- **`-y` echo** — when the gate is skipped under `-y`, emit `notes: accepted (-y)` so the auto-accept is visible in the captured log.
- **Errors/warnings** still also go to **stderr** (per the detection model), in addition to appearing in the plain narration — redirect-visibility.

Only the **delimiters and stage narration** differ between modes; the **notes body is byte-identical** in pretty and plain.

Confidence: high.

### Spinner lifecycle (resolves the spinners subtopic)

- One spinner at a time, on the **current** stage line (stderr-independent — it's part of the narration on stdout). Starts on `StageStarted`, replaced in place by the `✓`/`✗` line on completion. Braille frames (`⠋⠙⠹…`).
- **Underlying command output** (git/claude/gh chatter) is captured by mint, not streamed through the spinner line, so the animation can't be corrupted. On failure mint prints the captured output below the `✗` line.
- **`$EDITOR` (note edit)** takes over the terminal — spinner is stopped before handing off, resumed after.
- **`plain` never animates** — a stage emits exactly one line on its transition.

Confidence: high — lifecycle confirmed (frame style, captured-output handling, `$EDITOR` stop/resume, no-animate-in-plain all signed off).

### Decisions locked (pretty layer)

The **pretty** half is decided.

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

- **Width robustness (light touch, resolves review-002 F8)** — pretty mode assumes a normal terminal; we do **not** reintroduce width math everywhere (the whole reason the notes box was dropped). The single concession: **decorative rules are capped at `min(terminalWidth, ~50)`** so the `── release notes ──` / closing rule can't overflow and wrap into junk. Everything else **wraps naturally — never truncate** (losing release-note text is worse than a wrapped line). Stage lines stay fixed (they're short). Genuinely tiny/weird terminals are a `--plain` case, same stance as glyph capability. Exact rule width is impl detail.
- **`-y` alignment** — `-y` is a *yes* flag: it answers this `Continue?` gate `yes` unattended — identical outcome to pressing Enter. Reinforces the orthogonality model (gating = f(`-y`)).

**Cross-ref / reconciliation needed:** this **revises the engine discussion's** "Interactive confirmation & notes review" gate, which documented keys as `[a] accept / [e] edit / [r] regenerate / [q] abort`. Semantics are unchanged (same four choices, auto-unwind on abort) — only the *rendering* changes: default-yes `Continue?` instead of explicit accept, `n` instead of `q`. The in-progress `mint-release-tool` spec should reconcile the two surfaces (presentation owns the rendering; engine owns the four semantic choices).

Confidence: high on the pretty layer (brand, glyphs, stage shape, no-box notes, gate rendering).

### Cross-verb rendering (resolves review-002 F2)

The worked examples above are all `mint release`, but the seam applies to every verb. The non-release verbs:

- **`init`** — process narration in the same vocabulary: `✓ created .mint.toml` / `· skipped release (exists, use --force)`. No gate (non-clobbering).
- **`regenerate`** — same stage/notes/gate vocabulary as `release`, narrated per version (`--all` runs oldest→newest, one block each).
- **`version`** — the **one payload verb**: its output is a *value*, not narration. **Plain prints the bare value** (`1.4.0`) so `$(mint version)` / scripts consume it cleanly; **pretty may dress it** (`🌿 mint v1.4.0`). This is the deliberate exception to "narration is the product" — `version` actually has a payload, so the bare value is the floor and styling is additive only in pretty.

All four verbs emit through the same `Presenter`; consistency is structural (one interface), not per-verb styling code.

---

## Summary

### Key Insights

1. **mint's narration IS its output.** No separate data payload (bar `mint version`), so narration → stdout, and stderr is reserved for errors/warnings (kept only for redirect-visibility). "All process" is fine — when the narration is the product, the narration is stdout.
2. **Mirror `tick`'s adapter model** — one logical output, rendered by an adapter chosen by audience: `isatty(stdout)` → pretty, else plain; explicit flag overrides. Proven in a sibling tool the user already trusts.
3. **Two modes suffice** (pretty + plain). Structured json/toon is YAGNI here because mint renders a process, not a queryable data structure.
4. **Three orthogonal axes**: styling (`--plain` else TTY) · gating (`-y`) · output stream. Independence is the design's backbone — and the forbidden combo (non-TTY stdin + no `-y` at any gate) fails loud, never hangs.
5. **Engine emits events; presenter renders.** An event-oriented `Presenter` seam (not tick's data-shaped `Formatter`) keeps the seven-stage spine oblivious to colour/spinners/TTY — mirroring the `CommandRunner`/`Publisher` seams — and is how "consistent across all verbs" is achieved structurally.
6. **One render-mode flag — `--plain`.** Default is `isatty(stdout)`; `--plain` is the single explicit override (force-plain), which doubles as the escape hatch for UTF-8-incapable terminals so mint never needs capability sniffing or a fallback glyph set. No force-pretty (YAGNI). Minimal surface by design.

### Open Threads

- **Dry-run note reuse / caching** — **routed out** to the `mint-release-tool` discussion (engine/dry-run behaviour, not presentation) as an addendum to its dry-run-semantics decision, so the in-progress spec picks it up. The idea: cache the dry-run-generated note and reuse it on the real run to guarantee *what was previewed is what ships* (determinism), with invalidation keyed on the post-exclude diff + version + prompt. Resolved here; decided in spec.

### Spec hand-offs (reconciliation owed by the in-progress `mint-release-tool` spec)

- **Gate rendering vs engine discussion** — this discussion redesigned the notes-review gate to a default-yes `Continue?` (`y`/`n`/`e`/`r`, Enter⇒accept); the engine discussion documented `[a]/[e]/[r]/[q]`. Same four semantic choices, different rendering. The spec must adopt the presentation rendering and drop the stale `[a]/[q]` keys.
- **`--plain` in the CLI surface** — a new global/presentation flag (all verbs), to be recorded in the spec's CLI surface alongside `-y`.

### Current State

- **All 7 subtopics decided.** Render-mode detection (`--plain` override else `isatty(stdout)`, two modes, narration→stdout), the pretty layer (brand/glyphs/stage shape, no-box notes, default-yes `Continue?` gate, light width-cap), the plain contract (terse `key: value`, verbatim notes body), spinners (lifecycle + standalone non-Bubble-Tea spinner), `-y` orthogonality (full gate inventory, input handling, regenerate re-entry), the event-oriented `Presenter` seam, and library selection (lipgloss + standalone spinner). Ready for specification, pending the two spec hand-offs above.
