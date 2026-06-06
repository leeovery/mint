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

The seed mandates: render mode is driven by **TTY detection, not environment sniffing**. Interactive terminal → styled (brand, colour, spinners); non-TTY (an AI/agent consuming the output) → token-efficient plain text. The job here was to make "TTY detection" operationally precise, since `mint` is a **local interactive tool, not a CI job**, and `mint release` emits no capturable payload — almost all its output is status/chrome.

### Decision

**Stream split (conventional Unix):**
- **Status / progress / chrome → stderr** (brand, title, spinners, "doing X…" lines).
- **Machine payload → stdout** — only `mint version` and any future machine-readable value; stdout stays clean and capturable (`mint version | pbcopy` never sees a spinner).
- **Styling gate keys on `isatty(stderr)`** — chrome lives on stderr, so judge that stream. Each stream is evaluated on its own TTY-ness (the technically-correct rule). Spinners always go to stderr.

**Detection heuristic — TTY + explicit overrides only, never sniffing:**
- Default: `isatty(stderr)` → styled; else plain. That's the entire heuristic.
- Explicit overrides (user intent, *not* sniffing): honour **`NO_COLOR`** (de-facto standard); **`--plain`** forces token-efficient mode even on a TTY; optionally **`--color=always`** to force colour in captured logs.
- **No `CI=true` / `TERM` guessing** — that is precisely the environment sniffing the seed forbids. Mode comes from the TTY or an explicit flag/var, never inferred from the environment.

**Colour vs layout = one switch.** "Plain" = no ANSI *and* no spinner animation *and* no decorative banner — the full token-efficient degradation, because the only consumer of plain mode is a token-sensitive agent and half-measures waste tokens. The lone exception: `NO_COLOR` on a TTY strips colour but keeps the spinner (a human who simply dislikes colour).

### Journey

Initial framing assumed a payload-vs-chrome stdout/stderr split where a human might do `mint release > notes.txt`. The user corrected this: mint doesn't emit a capturable release payload — it performs side effects (commits, tag, push, `gh` release, CHANGELOG) and shows the notes-review gate interactively. So "non-TTY" means *an agent runs mint and reads its output as text*, and the goal is stripping ANSI/animation to save the agent's tokens — not formatting a redirected result. That collapsed the detection model to a single per-stream TTY check.

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

1. **mint has no payload stream to protect** — it's a side-effecting interactive tool, so "styled vs plain" is a single per-stream TTY decision, not a payload/chrome split.
2. **Three orthogonal axes**: styling (TTY) · gating (`-y`) · output stream (fixed). Keeping them independent is the design's backbone.

### Open Threads

- **Dry-run note reuse / caching** — **routed out** to the `mint-release-tool` discussion (engine/dry-run behaviour, not presentation) as an addendum to its dry-run-semantics decision, so the in-progress spec picks it up. The idea: cache the dry-run-generated note and reuse it on the real run to guarantee *what was previewed is what ships* (determinism), with invalidation keyed on the post-exclude diff + version + prompt. Resolved here; decided in spec.
- Library selection was flagged in discovery as a deferred how-question.

### Current State

- Render-mode detection decided. `-y` orthogonality mostly decided (stdin/gate interplay locked). Remaining: what the styled layer shows, plain-mode contract, spinners, presentation seam, library selection.
