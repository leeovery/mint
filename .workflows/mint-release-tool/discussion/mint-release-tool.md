# Discussion: Mint Release Tool

## Context

Build `mint`, a reusable configuration-driven Go release tool that replaces the
per-project `release` bash scripts copy-pasted (and drifting) across ~8 repos. It
extracts the generic release engine — AI release notes via `claude`, semver bump,
`git_safe` lock handling, CHANGELOG generation, annotated tag + atomic push, `gh`
release creation — into one reusable binary. Distributed as a public dual-arch
formula in the existing `leeovery/homebrew-tools` tap, with per-project TOML config
and a hook system for project-specific steps. Each project keeps a tiny `release`
shim that delegates to the global `mint`; `mint init` scaffolds config, shim, and
example hooks.

Several decisions are already settled in discovery/handoff and are **not** up for
re-litigation here unless something forces it:

- **Language: Go** — for testability of the fragile logic behind a single
  `CommandRunner` interface (mock git/gh/claude).
- **Name: `mint`** (global binary); local shim stays `release` for muscle memory.
- **Distribution: new public dual-arch formula** in `leeovery/homebrew-tools`,
  source in its own repo, reusing the tap's auto-update action.
- **Per-project shim + `mint init` activation.**
- **The 552-line `agentic-workflows/release` is the behavioral spec / test oracle.**

The open forks discovery deliberately deferred to discussion: **hook mechanism**
(scripts vs inline config vs both) and **config format** (TOML vs YAML). Beyond
those, this discussion shapes the pipeline lifecycle, config schema, CLI surface,
`mint init` behaviour, and the testability/parity strategy.

### References

- [Design handoff](../imports/release-tool-design-handoff.md) — decisions, open forks, and the verbatim 552-line reference script
- [Discovery session 001](../discovery/session-001.md)

## Discussion Map

### States

- **pending** (`○`) — identified but not yet explored
- **exploring** (`◐`) — actively being discussed
- **converging** (`→`) — narrowing toward a decision
- **decided** (`✓`) — decision reached with rationale documented

### Map

  Discussion Map — Mint Release Tool (8 subtopics · 8 pending)

  ┌─ ○ Hook mechanism [pending]
  ├─ ○ Config format & schema [pending]
  ├─ ○ Pipeline lifecycle & hook points [pending]
  ├─ ○ CommandRunner & testability strategy [pending]
  ├─ ○ Parity with bash oracle [pending]
  ├─ ○ CLI surface & flags [pending]
  ├─ ○ `mint init` scaffolding [pending]
  └─ ○ Distribution & versioning of mint itself [pending]

---

*Subtopics are documented below as they reach `decided` or accumulate enough exploration to capture.*

---

## Summary

### Key Insights

*(to be filled as the discussion progresses)*

### Open Threads

*(to be filled)*

### Current State

- Nothing decided yet — discussion just initialized from the design handoff.
