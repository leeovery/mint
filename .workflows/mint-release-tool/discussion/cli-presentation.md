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

  Discussion Map — CLI Presentation (7 subtopics — 1 decided · 1 exploring · 5 pending)

  ┌─ ✓ Render-Mode Detection Model [decided]
  ├─ ○ What The Styled Layer Actually Shows [pending]
  ├─ ○ Plain / Token-Efficient Mode Contract [pending]
  ├─ ○ Spinners & Long-Running Progress [pending]
  ├─ ◐ -y/--yes Orthogonality [exploring]
  ├─ ○ Presentation Seam / Architecture [pending]
  └─ ○ Library Selection (Charm Vs Lighter) [pending]

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

**Detection — `isatty(stdout)` + explicit override, never sniffing:**
- Default: `isatty(stdout)` → `pretty`; non-TTY → `plain`. That's the entire heuristic. (Matches `tick`: it detects on stdout and defaults TTY→pretty, piped→token-efficient.)
- Explicit override flags `--pretty` / `--plain` win over detection. **No `CI=true`/`TERM` guessing** — exactly the environment sniffing the seed forbids.

**Stream split — narration is the product, so it's stdout:**
- **Run narration → stdout** — stages, the plan, the notes preview, the final summary, and `mint version`'s value. mint has no separate data payload, so the narration *is* its stdout output.
- **Errors + warnings → stderr** — the one real job stderr keeps: *visibility under redirection*. `mint release > run.log` sends stdout to the file, but a failure on stderr still hits the terminal and can't silently vanish.
- **Exit code** signals success/failure for scripts (they check `$?`, not stream parsing).
- An **agent captures combined output (`2>&1`)** by default, so it sees narration *and* errors regardless of the split — the split costs the agent nothing and buys humans redirect-visibility.

**Colour is a toggle *within* `pretty`, not a third mode.** `NO_COLOR` (de-facto standard) and `--no-color` drop ANSI colour but stay in pretty (spinner still runs) — a human who dislikes colour. `plain` already implies no colour. So: mode ∈ {pretty, plain}; colour is a sub-switch of pretty. (Optionally `--color=always` to force colour in captured logs; minor.)

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

## Summary

### Key Insights

1. **mint's narration IS its output.** No separate data payload (bar `mint version`), so narration → stdout, and stderr is reserved for errors/warnings (kept only for redirect-visibility). "All process" is fine — when the narration is the product, the narration is stdout.
2. **Mirror `tick`'s adapter model** — one logical output, rendered by an adapter chosen by audience: `isatty(stdout)` → pretty, else plain; explicit flag overrides. Proven in a sibling tool the user already trusts.
3. **Two modes suffice** (pretty + plain). Structured json/toon is YAGNI here because mint renders a process, not a queryable data structure.
4. **Three orthogonal axes**: styling (TTY) · gating (`-y`) · output stream. Independence is the design's backbone.

### Open Threads

- **Dry-run note reuse / caching** — **routed out** to the `mint-release-tool` discussion (engine/dry-run behaviour, not presentation) as an addendum to its dry-run-semantics decision, so the in-progress spec picks it up. The idea: cache the dry-run-generated note and reuse it on the real run to guarantee *what was previewed is what ships* (determinism), with invalidation keyed on the post-exclude diff + version + prompt. Resolved here; decided in spec.
- Library selection was flagged in discovery as a deferred how-question.

### Current State

- Render-mode detection **decided** (revised to stdout-based, tick-aligned, two modes, narration→stdout). `-y` orthogonality mostly decided. Remaining: what the pretty layer shows, the concrete plain-mode text contract, spinners, presentation seam/architecture, library selection.
