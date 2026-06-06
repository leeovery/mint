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

  Discussion Map — CLI Presentation (7 subtopics · 7 pending)

  ┌─ ○ Render-mode detection model [pending]
  ├─ ○ What the styled layer actually shows [pending]
  ├─ ○ Plain / token-efficient mode contract [pending]
  ├─ ○ Spinners & long-running progress [pending]
  ├─ ○ -y/--yes orthogonality [pending]
  ├─ ○ Presentation seam / architecture [pending]
  └─ ○ Library selection (charm vs lighter) [pending]

---

*Subtopics are documented below as they reach `decided` or accumulate enough exploration to capture.*

---

## Summary

### Key Insights

*(to be populated)*

### Open Threads

- Library selection was flagged in discovery as a deferred how-question.

### Current State

- Discussion just started; all subtopics pending.
