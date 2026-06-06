# Discovery Session 002

Date: 2026-06-06
Work unit: mint-release-tool

## Description (as of session)

Build mint, a reusable configuration-driven Go release tool that replaces the
per-project release bash scripts, distributed via Homebrew with per-project
config and a project-specific hook system.

## Seed

(none)

## Imports

- imports/release-tool-design-handoff.md

## Map State at Start

1 topic — 1 decided.

## Exploration

A fresh thread opened on **mint's CLI presentation / output layer** — a surface
the first discussion never covered. The original discussion settled *what* the
interactive flow does (the `[a]/[e]/[r]/[q]` notes-review gate, the plan summary,
abort/unwind messaging) but never *how mint presents itself* on screen: colour,
the 🌿 leaf branding, a title, and progress/spinners while git and the `claude`
CLI run.

The user wants a "styled-but-restrained" feel — more than plain text, not overly
rich. Concretely: colours, spinners, a nice brand and title, around the existing
plan + notes flow. This presentation should be **consistent across all mint
commands** (release, regenerate, init, version), not just the release run.

Render mode crystallised around **who's on the other end**, decided by **TTY
detection** rather than env-sniffing:
- **Interactive terminal (TTY)** → full styled UI.
- **Non-TTY (piped / redirected / no real terminal)** → token-efficient,
  stripped-down output, suited to an AI/agent caller consuming the output.

The `-y`/`--yes` flag is **orthogonal to styling** — it only skips the
interactive gate stops; it does *not* imply non-TTY. So a human running `-y` at
a terminal still gets the same styled UI, just without the gates. (The user
explicitly walked back an earlier "detect if it's an AI via env var" idea —
TTY detection is the signal, not environment checking.)

Open question this thread raises but does not resolve: **which Go packages**
provide the styling/spinners (e.g. a charm/lipgloss-style stack vs lighter
colour libraries). That's a how/feasibility question for a later phase, not
shaped here.

A second thread reintroduced **`mint commit`**. It was parked in the first
discussion as "its own separate feature," with the `mint <verb>` namespace
adopted to leave room — and the pivot from feature to epic is the exact trigger
that was flagged for promoting it. Shape: a sibling command that wraps the
user's existing AI-commit shell function — AI-generated commit message from the
diff, with `--all`, `--no-ai`, context injection, and auto-push; "minting a
commit" fitting the brand. The user wants it **built into mint** (sharing the AI
engine, `.mint.toml` config, and the presentation layer above), with the
integration details deliberately left for its own discussion. Discussion-shaped:
the user knows the rough shape, the open work is design/integration decisions.

A third thread: **better AI release-note generation**. Motivated by the user's
current agentic-workflows release script — the notes aren't always great,
**especially on bigger releases** (large diffs summarise to mush, echoing the
first discussion's `max_diff_lines` cost/quality note). The first discussion
tuned the *prompt and format* but always fed the AI a raw textual diff; this
thread is about enriching the **input** — speculatively via AST / semantic
structure of the change, or some other signal — to lift quality. The user framed
it explicitly as speculative ("a suspicion we can do better"), an open
what's-possible question → research-shaped.

## Edits

(none)

## Topics Identified

### cli-presentation

- Routing: discussion
- Why: user has a clear shape in mind (styled-but-restrained, TTY-driven render modes, gate-skip orthogonal) — a design decision space, not an open feasibility question.

### commit-command

- Routing: discussion
- Why: user knows the rough shape (wraps an existing AI-commit function, built into mint); the open work is design/integration decisions, not investigation.

### release-notes-quality

- Routing: research
- Why: framed explicitly as speculative ("a suspicion we can do better"), an open what-is-possible question about enriching AI input beyond a raw diff.

## Conclusion

3 topic(s) added. Map now has 4 topics.
