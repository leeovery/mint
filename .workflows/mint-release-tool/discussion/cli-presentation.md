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

  Discussion Map — CLI Presentation (8 subtopics — 8 decided)

  ┌─ ✓ Render-Mode Detection Model [decided]
  ├─ ✓ What The Pretty Layer Actually Shows [decided]
  ├─ ✓ Plain / Token-Efficient Mode Contract [decided]
  ├─ ✓ Spinners & Long-Running Progress [decided]
  ├─ ✓ -y/--yes Orthogonality [decided]
  ├─ ✓ Presentation Seam / Architecture [decided]
  ├─ ✓ Per-Verb Presentation [decided]
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

**Detection — `isatty(stdout)` only. No override flags, no sniffing:**
- `isatty(stdout)` → `pretty`; non-TTY → `plain`. That is the *entire* mechanism — there is **no `--pretty`/`--plain`/`--no-color` override**. (The only run flag near this area is `-y`, which is gating, not rendering — orthogonal.)
- **"Human vs agent" reduces to "is stdout a terminal?"** — an agent never announces itself; its harness captures stdout through a **pipe** (not a terminal) → plain. A human's terminal → pretty. The OS reports what's connected for free.
- **Decided mechanism: `golang.org/x/term.IsTerminal(int(os.Stdout.Fd()))`** — *not* the `Stat().Mode() & os.ModeCharDevice` check that `tick` and `portal` use. The stat check is subtly wrong: `/dev/null` **is** a character device, so `mint release > /dev/null` would mis-detect as a TTY and emit ANSI. `x/term.IsTerminal` does a real terminal ioctl, correctly returning false for `/dev/null`, pipes, and regular files. One small, already-transitively-present dependency for a correct answer.
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

### Decision

- **Three orthogonal axes**: styling = f(`isatty(stdout)`), gating = f(`-y`), output stream = fixed (narration→stdout, errors/warnings→stderr). A human with `-y` at a terminal still gets full styling.
- **One uniform rule for every interactive stop**: a gate needs **`isatty(stdin)`**. If **stdin is not a TTY** and **`-y` was not passed**, mint **fails loud** ("not a TTY — pass `-y` to run unattended") rather than blocking on stdin. Output styling (stdout TTY) and gate availability (stdin TTY) are checked **independently** — a run can legitimately have a piped stdout (plain) but a TTY stdin, or vice versa; each is judged on its own fd.

### The complete gate inventory (resolves "are there other gates?" — yes, and they all obey the one rule)

Enumerated against the engine's decided CLI surface; **no per-gate special-casing**:

1. **Release notes-review** (`[a]/[e]/[r]/[q]`) — the main gate. `-y` skips → notes ship as generated.
2. **`regenerate` interactive questions** (no flags → asks source, asks target) — these are prompts; `-y` (or supplying the flags) skips them. Non-TTY stdin without `-y`/flags → fail loud.
3. **`regenerate` confirm** ("shows plan, confirms") — `-y` skips the confirm.
4. **`regenerate --all` per-version review gate** (fresh) — gated per version; `-y` runs the batch unattended. Same TTY-or-`-y` rule per version.
5. **`regenerate [a]/[e]/[r]/[q]`** on a fresh single version — same as #1.
6. **`mint init`** — **no gate**: it is non-clobbering by design (skips existing files with a notice) and `--force` overwrites without prompting. So there is nothing for `-y` to skip and no stdin dependency. Confirmed: `init` never blocks on input.

`reuse` regenerate is deterministic (a simple confirm, no review gate) — covered by #3. That's the whole universe; the rule is uniform across all of them.

Confidence: high — gate inventory is complete against the decided CLI surface; the single TTY-or-`-y` rule covers every case.

---

## Presentation Seam / Architecture

### Context

The structural backbone the topic exists to define: how the engine and the presentation layer relate, so that mode selection, colour, and spinners live in exactly one place and the seven-stage release spine stays oblivious to them. `tick` gave a template — a `Formatter` interface with concrete per-mode impls behind a factory — but `tick`'s methods are **data-shaped** (`FormatTaskList(tasks)`) because it renders data structures. mint renders a **process**, so the seam must be shaped differently.

### Decision

**An event/step-oriented `Presenter` interface the engine calls at lifecycle points.** The engine emits *semantic events* ("stage X started", "here's the plan", "warn: hook failed"); the presenter decides *how they look*. **Pinned method set** (names/signatures are the decided surface; only cosmetic Go-idiom tweaks — receiver names, error-vs-bool return — are left to impl):

```go
type Presenter interface {
    // run framing
    RunStarted(project, version string)            // 🌿 mint · {project} › releasing v{X}
    RunFinished(project, version, url string)      // success footer (url "" if publish=false)

    // the seven-stage spine
    StageStarted(stage string)                     // pretty: starts spinner on this line
    StageDetail(stage, detail string)              // optional sub-detail before completion
    StageSucceeded(stage, detail string, d time.Duration)
    StageFailed(stage string, err error)
    Unwound(summary string)                        // ↩ what auto-unwind undid

    // information surfaces
    ShowPlan(plan Plan)                            // bulleted plan block
    ShowNotes(version, body string)                // boxed notes / delimited block
    Warn(label, msg string)                        // ⚠ non-fatal (e.g. post_release)

    // interaction (pretty only; plain never reaches here — see -y rule)
    ReviewGate() GateChoice                        // [a]/[e]/[r]/[q] → accept|edit|regen|abort
    RegenContext() string                          // one-line context for [r]
}
```

- **Two implementations behind the interface — `pretty` and `plain`** — selected **once at startup** from the TTY check (see detection below). Nothing downstream re-checks the TTY.
- **The engine never touches colour, spinners, or TTY state.** It calls `Presenter` methods only. This mirrors the engine's existing seams (`CommandRunner` for git/gh/claude, `Publisher` for releases) — the same dependency-inversion discipline, now for output.
- **Applies to every verb.** `release`, `regenerate`, `init`, `version` all emit through the same `Presenter` (see the per-verb rendering subtopic for how each maps onto these methods) — this is *how* "consistent presentation across all verbs" is achieved structurally, not per-verb styling code.
- **Testability** (the whole Go rationale): a recording/fake `Presenter` asserts which events fired and with what payload, independent of rendering. A non-interactive presenter is used under `-y`; `ReviewGate`/`RegenContext` are never called when gates are skipped.
- **Spinner ownership**: the `pretty` impl owns the spinner internally (started by `StageStarted`, cleared by `StageSucceeded`/`StageFailed`); the engine is unaware a spinner exists. `plain` renders the same events as terse lines. Full lifecycle decided in the spinners subtopic below.

### Journey

Considered cloning `tick`'s data-shaped `Formatter` directly, but mint has no task-list-equivalent to format — it has a running process. Reshaping the seam from "format this data" to "react to this event" keeps the decoupling principle (one interface, per-mode impls, factory by mode) while fitting what mint actually does. The engine-emits-events / presenter-renders split is the payoff: the spine is testable and rendering-agnostic.

Confidence: high — split, event shape, and the method surface are all decided. Only Go-idiom cosmetics (error vs bool returns, exact `Plan`/`GateChoice` struct fields) remain, and those are mechanical.

---

## Library Selection (Charm vs lighter)

### Context

The discovery how-question: which Go packages provide styling and spinners — a charm/lipgloss stack vs lighter colour libs. Grounded by two sibling Go tools the user already runs: `tick` (uses lipgloss-style formatting) and `portal`.

### Decision

- **`lipgloss` for all `pretty`-mode styling** — colour, the 🌿 brand line, status glyphs, the notes box/border. It is *pure string styling* (no event loop), so it composes with the `Presenter` seam, and it auto-downgrades colour when piped. Idiomatic and already in the user's toolchain.
- **Spinner: `github.com/briandowns/spinner`** (decided). Explicit `Start()`/`Stop()` + mutable suffix maps 1:1 onto `StageStarted`→`StageSucceeded/Failed`, and it handles the fiddly bits (line-clearing, carriage returns, cleanup on stop) that a hand-roll gets wrong. Rejected `huh/spinner` — it's action-wrapping (`.Action(fn).Run()`), which would invert our seam (presenter running the engine's work instead of the engine driving the presenter). Lipgloss styles the suffix text; briandowns drives the frames.
- **NOT Bubble Tea / no alt-screen / no full-screen TUI.** Print-style linear narration only.
- **`plain` mode pulls in no UI library** — just `fmt` lines. That's the point of token-efficiency.

### Journey — why not Bubble Tea / Portal's stack

`portal` is a full-screen Bubble Tea TUI (`tea.NewProgram(tea.WithAltScreen())`, a `Model`/`Update`/`View` state machine with pages and modals). Inspected as a candidate baseline and **rejected for mint**: it's built for an interactive picker that *owns the screen*, whereas mint narrates a linear process and exits. A full TUI would also fight the `Presenter` seam (Bubble Tea wants to own the event loop; mint's engine drives and calls the presenter) and the dual pretty/plain requirement. We take **lipgloss** (the styling layer) from that ecosystem and leave the TUI runtime. Note: `portal` and `tick` both detect TTY with the same `os.Stdout.Stat() & ModeCharDevice` check we locked — three-for-three.

Confidence: high — stack decided: `lipgloss` (styling) + `briandowns/spinner` (spinner) + `golang.org/x/term` (TTY), no Bubble Tea. Exact lipgloss colour values are cosmetic and tweakable; the library choices are fixed.

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
| `ShowNotes` | rounded box `╭─ Release notes · v{X} ─╮ … ╰─╯` | `--- release notes v{X} ---` … `--- end notes ---` |
| review gate | `[a] accept  [e] edit  [r] regenerate  [q] abort` + `› ` prompt | (not shown — non-TTY ⇒ `-y` required ⇒ gate skipped; emits `notes: accepted (-y)`) |
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

  ╭─ Release notes · v1.4.0 ─────────────────────╮
  │ Faster cold starts and a calmer log.         │
  │                                              │
  │ ✨ Features                                   │
  │   • Parallel warm-up halves boot time        │
  │ 🐛 Fixes                                      │
  │   • Stop double-flush on SIGTERM             │
  ╰──────────────────────────────────────────────╯

  [a] accept   [e] edit   [r] regenerate   [q] abort
  › a

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

Confidence: high — look-and-feel baseline confirmed by the user. Exact colour values and micro-spacing are cosmetic and tweakable later (and the Claude Code TUI can't render colour in code blocks anyway); the structure — glyphs, the brand line, the notes box, plain's one-line-per-transition contract — is decided.

---

## Per-Verb Presentation (release · regenerate · init · version)

### Context

The topic's scope is *every* verb, not just `release`. The seam (one `Presenter`, all verbs) makes this mostly free, but each verb maps onto the events differently — pin that mapping now rather than discovering it at impl.

### Decision

- **`release`** — the canonical case fully worked above (RunStarted → stages → ShowPlan → ShowNotes → ReviewGate → RunFinished).

- **`version`** — special: it emits a **machine value**, so it bypasses the narration. **`plain`: bare `0.3.1` on stdout** (clean, scriptable — `v=$(mint version)`). **`pretty`: `🌿 mint 0.3.1`**. No stages, no spinner. This is the one verb whose stdout is a capturable payload (consistent with the render-mode decision).

- **`init`** — emits a **per-file result** through the presenter (no spinner; the work is instant):
  - `pretty`:
    ```
    🌿 mint · init
      ✓ created  .mint.toml
      • skipped  release (exists — use --force)
    ```
  - `plain`:
    ```
    init: created .mint.toml
    init: skipped release (exists)
    ```
  - `--force` overwrites → `✓ regenerated`. No gate (decided in the gate inventory).

- **`regenerate <version>`** — same notes-box + `ReviewGate` rendering as `release` (fresh), scoped to one version; `reuse` is a simple confirm + a single `✓ v1.4.0 release re-created` line.

- **`regenerate --all`** — **per-version progress + a final skip summary** (the engine's skip-and-continue batch semantics):
  - `pretty`:
    ```
    🌿 mint · regenerate (all, oldest→newest)
      ✓ v1.2.0  regenerated
      ✓ v1.3.0  regenerated
      ⤬ v1.3.1  skipped — diff too large (max_diff_lines)
      ✓ v1.4.0  regenerated
    
    27 regenerated · 3 skipped (v1.3.1, v0.9.4, v0.2.0)
    ```
  - `plain`:
    ```
    regenerate: v1.2.0 regenerated
    regenerate: v1.3.1 skipped (diff too large)
    summary: 27 regenerated, 3 skipped: v1.3.1, v0.9.4, v0.2.0
    ```
  - Per-version review gate (fresh) obeys the uniform `-y` rule; `-y` runs the whole batch unattended.

These reuse the existing `Presenter` methods (`RunStarted`/`StageSucceeded`-style lines / `Warn` for skips / `RunFinished` for the summary) — no verb-specific presenter, just different call sequences.

Confidence: high.

---

## Summary

### Key Insights

1. **mint's narration IS its output.** No separate data payload (bar `mint version`), so narration → stdout, and stderr is reserved for errors/warnings (kept only for redirect-visibility). "All process" is fine — when the narration is the product, the narration is stdout.
2. **Mirror `tick`'s adapter model** — one logical output, rendered by an adapter chosen by audience: `isatty(stdout)` → pretty, else plain. No override flags; piping is the force-plain path. Proven in a sibling tool the user already trusts.
3. **Two modes suffice** (pretty + plain). Structured json/toon is YAGNI here because mint renders a process, not a queryable data structure.
4. **Three orthogonal axes**: styling (TTY) · gating (`-y`) · output stream. Independence is the design's backbone.
5. **Engine emits events; presenter renders.** An event-oriented `Presenter` seam (not tick's data-shaped `Formatter`) keeps the seven-stage spine oblivious to colour/spinners/TTY — mirroring the `CommandRunner`/`Publisher` seams — and is how "consistent across all verbs" is achieved structurally.
6. **No render-mode flags.** Mode is purely `isatty(stdout)`; piping is the natural "force plain". Minimal surface by design.

### Open Threads

- **Dry-run note reuse / caching** — **routed out** to the `mint-release-tool` discussion (engine/dry-run behaviour, not presentation) as an addendum to its dry-run-semantics decision, so the in-progress spec picks it up. The idea: cache the dry-run-generated note and reuse it on the real run to guarantee *what was previewed is what ships* (determinism), with invalidation keyed on the post-exclude diff + version + prompt. Resolved here; decided in spec.
- *(none open — the discovery how-question on libraries is now decided; dry-run caching is routed to the engine spec.)*

### Current State

- **All 8 subtopics decided.** Render-mode detection (stdout `x/term.IsTerminal`, two modes, narration→stdout, no flags) · pretty + plain worked examples · spinner lifecycle · uniform `-y`/stdin-TTY gate rule with a complete gate inventory · event-oriented `Presenter` seam (pinned method set) · per-verb rendering (release/regenerate/init/version) · library stack (lipgloss + briandowns/spinner + x/term, no Bubble Tea). Only cosmetic colour/spacing values remain tweakable at impl. Ready for spec.
