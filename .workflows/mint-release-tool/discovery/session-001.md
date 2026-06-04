# Discovery Session 001

Date: 2026-06-04
Work unit: mint-release-tool

## Description (as of session)

Build `mint`, a reusable configuration-driven Go release tool that replaces the
per-project `release` bash scripts (copy-pasted and drifting across ~8 repos).
Distributed via the existing `leeovery/homebrew-tools` tap, with per-project TOML
config and a hook system for project-specific steps. Each project keeps a tiny
`release` shim that delegates to the global `mint` binary; `mint init` scaffolds
config, shim, and example hooks into a project.

## Seed

(none)

## Imports

- imports/release-tool-design-handoff.md

## Map State at Start

(n/a — single-topic work)

## Exploration

The user brought a thorough design handoff document (`release-tool-design-handoff.md`)
for productizing a release tool. The problem: a `release` bash script was copy-pasted
across ~8 repos and diverged over months. The goal is to extract the generic engine
into one reusable Go binary, distributed via Homebrew, with per-project config and a
hook system for the one project-specific step seen so far (a knowledge-base bundle build
in `agentic-workflows`).

Several decisions are already reached in the handoff: language is Go (for testability of
the fragile logic — `git_safe` lock handling, changelog insertion, version extraction, AI
diff guards — behind a single `CommandRunner` interface); distribution as a new public
dual-arch formula in the `leeovery/homebrew-tools` tap with source in its own repo; a
per-project `release` shim plus `mint init` activation. The 552-line `agentic-workflows`
script is the superset and serves as the behavioral spec / test oracle for the rewrite.

Open forks remain (for discussion/spec, not discovery): the hook mechanism (hook scripts
vs inline config commands vs both), and config format (TOML assumed, confirm vs YAML).

Shape conversation: this is clearly something to **build** (a tool), not a fix or a
pattern to define — so not bugfix/quick-fix/cross-cutting. The real fork was single
feature vs epic, given the multiple distinct concerns (release engine, config schema, hook
system, brew distribution, `mint init` scaffolding) and the productization/multi-repo
framing. The user reflected that it's a brand-new project and "depends on how we break it
up," but agreed it's relatively straightforward and fits inside a feature, with the option
to promote to an epic later if scope fragments. Settled as a **single feature**: one
coherent deliverable with a clear behavioral oracle, tightly-coupled concerns that
planning will phase out, and a handful of design forks to settle in discussion/spec.

## Edits

(none)

## Topics Identified

(none)

## Conclusion

(none)
