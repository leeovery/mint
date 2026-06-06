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

## Edits

(none)

## Topics Identified

(none)

## Conclusion

(none)
