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

  Discussion Map — Mint Release Tool (10 subtopics — 2 decided · 1 exploring · 7 pending)

  ┌─ ✓ Release lifecycle spine [decided]
  ├─ ✓ Version detection & bump [decided]
  ├─ ◐ Safety & preflight gates [exploring]
  ├─ ○ Hook mechanism [pending]
  ├─ ○ AI release notes [pending]
  ├─ ○ Changelog & version recording [pending]
  ├─ ○ Tag, push & publish [pending]
  │  └─ ○ Post-release: tap / formula update [pending]
  ├─ ○ Config format & schema [pending]
  ├─ ○ CLI surface & flags [pending]
  └─ ○ `mint init` scaffolding [pending]

---

*Subtopics are documented below as they reach `decided` or accumulate enough exploration to capture. Approach: clean-slate design, working top-to-bottom through the lifecycle spine. The old bash script is a feature checklist, not a design to copy.*

---

## Release lifecycle spine

### Context

The lifecycle is the contract everything else hangs off — hooks, config, init, and the testability strategy all reference these stages. Designed clean-slate; the old script's ordering is not authoritative.

### Decision

A release run has seven stages, in order:

1. **Version** — determine current version and compute the next (patch/minor/major).
2. **Preflight** — safety gates: clean working tree, required tools present & authenticated. Nothing irreversible past this point until stage 6.
3. **Project prep (hooks)** — project-specific build/prep steps that may produce and commit artifacts.
4. **Release notes** — AI-generated from the diff.
5. **Record** — changelog entry, version file (where applicable).
6. **Make official** — annotated tag + atomic push. This is the point of no return.
7. **Publish** — GitHub release + project-specific follow-ups.

Provisional confidence: high on the spine; per-stage details still to be designed top-to-bottom. The user's bar: robust + usable across all their projects, configurable where ambiguous.

---

## Version detection & bump

### Context

How mint determines the current version and computes the next one — the first stage of every release. The old script conflated "where to *read* the version" with "where to *write* it" via three strategies (file / embedded / none). Clean-slate design separates these concerns.

### Decision

**Source of truth = git tags, always.** The current version is the latest `vX.Y.Z` semver tag (stripped of `v`); no tags → `0.0.0`. Rationale: brew installs from tags, so the tag *is* the real version — a file copy is derived state and a needless fork. This collapses the old three strategies into one rule.

**Bump selection** (carried over from the old script — the user likes it, it's intuitive):
- `-p` / `--patch` — default when no flag given
- `-m` / `--minor`
- `-M` / `--major`
- `-d` / `--dry-run` — preview without making changes

**First release handles itself** — no special-casing. With no tags, current is `0.0.0`, so `mint` → `0.0.1`, `mint -m` → `0.1.0`, `mint -M` → `1.0.0`. The user picks via the normal bump flag.

**Escape hatch:** `--version X.Y.Z` to set an explicit version (e.g. a deliberate 1.x → 2.0.0 jump). Preferred over a positional `mint 2.0.0` — the flag is unambiguous and self-documenting.

**Optional version-file projection:** when a project needs the version *written into the repo* (a bash script with `RELEASE_VERSION="x.y.z"`, or a plain `release.txt` read at runtime), mint mirrors the new version into a file during the Record stage. Config:
- `version_file` — path; omit = tag-only (no projection).
- `version_pattern` — e.g. `RELEASE_VERSION="{version}"`; omit = whole file *is* the version (plain mode).

Truth still always comes from the tag; the file is a kept-in-sync mirror, never the source.

### Notes / deferred

- **Brew formula version bump is NOT mint's job.** The formula's version + sha256 are bumped downstream by the tap's auto-update CI reacting to the GitHub release mint creates. Most repos mint releases aren't formulas anyway. If a project ever wants mint to actively trigger it (`repository_dispatch`), that's a **post-release hook**, not engine code. Tracked as a child of Tag/push/publish.

Confidence: high.

---

### Key Insights

*(to be filled as the discussion progresses)*

### Open Threads

*(to be filled)*

### Current State

- Nothing decided yet — discussion just initialized from the design handoff.
