# Specification: Mint Release Tool

## Specification

## Overview & Scope

### Purpose

`mint` is a reusable, configuration-driven Go release tool that replaces the per-project `release` bash scripts that have been copy-pasted (and have drifted) across ~8 repositories. It extracts the generic release engine into one reusable binary: AI-generated release notes, semver bump, lock-resilient git handling, CHANGELOG generation, annotated tag + atomic push, and GitHub release creation.

### Settled foundations (not re-litigated)

- **Language: Go** — chosen for testability of the fragile logic (git/`gh`/`claude` invocation) behind a single `CommandRunner` interface that can mock those external commands.
- **Name: `mint`** for the global binary; each project keeps a tiny local shim named `release` for muscle memory.
- **Distribution:** a new public dual-arch Homebrew formula in the existing `leeovery/homebrew-tools` tap. `mint`'s source lives in its own repository, reusing the tap's existing auto-update CI action. Install via `brew install leeovery/tools/mint`.
- **Activation model:** each project carries a committed `release` shim that delegates to the globally-installed `mint`; `mint init` scaffolds the per-project config and shim.

### Command namespace

`mint` adopts a `mint <verb>` command namespace from the outset. The release command is `mint release`; the per-project `release` shim delegates to `mint release`. This is forward-compatible — it leaves room for future verbs (e.g. a later `mint commit`) without restructuring — but **this build ships release functionality only**. `mint` remains a single feature for now. The namespace leaves the door open to promote `mint` to an epic (release + commit + …) later, but that promotion is not made now.

### In scope (this build)

The complete release pipeline end-to-end: version determination → preflight safety gates → project-prep hooks → AI release notes (with interactive review) → record (changelog + version file) → annotated tag + atomic push → publish (GitHub release + post-release hooks); plus the regenerate/heal command, the TOML config schema, the CLI surface, and `mint init` scaffolding.

### Out of scope (consciously deferred)

- **`mint commit`** — a future, separate feature with its own design.
- **Testing / parity strategy** — deferred to planning/implementation. The legacy 552-line `agentic-workflows/release` bash script is treated as a **feature reference / capability checklist, not a byte-parity test oracle**; the clean-slate design intentionally diverges from it.
- **YAGNI items addable later:** pre-release/RC tag parsing & production, `--rewrite-tags` (destructive tag rewriting), a `.release/hooks/` directory convention, built-in note "themes", project auto-detection in `mint init`, a dry-run hook-run toggle, a notes-review disable toggle, and a `.mintignore` file.

---

## Release Lifecycle (the spine)

A `mint release` run proceeds through seven stages, in strict order. This spine is the contract that hooks, config, and recovery all hang off.

| # | Stage | What happens | Reversible? |
|---|-------|-------------|-------------|
| 1 | **Version** | Determine the current version (from git tags) and compute the next (patch/minor/major or explicit). | Yes — read-only |
| 2 | **Preflight** | Safety gates: clean tree, on release branch, target tag free, remote in sync, required tools present & authenticated. | Yes — read-only checks |
| 3 | **Project prep (hooks)** | Run the project's `pre_tag` hook (build/generate artifacts). May dirty the tree; mint commits artifacts. | Yes — local only |
| 4 | **Release notes** | Generate the notes body from the diff via the AI engine; interactive review gate. | Yes — local only |
| 5 | **Record** | Write the CHANGELOG entry and the optional version-file projection; create release commit(s). | Yes — local only |
| 6 | **Make official** | Create the annotated tag and `git push --atomic` (commits + tag together). | **No — point of no return** |
| 7 | **Publish** | Create the provider release (GitHub today) + run `post_release` hooks. | Post-PONR — warn-only on failure |

### Invariants

- **Everything before stage 6 is local-only and recoverable.** If any stage 1–5 fails (or the user aborts at the review gate), mint auto-unwinds every mutation it made this run, returning the repo to the exact clean state it started from.
- **`git push --atomic` (stage 6) is the single point of no return.** Commits and tag go up together or not at all.
- **After the point of no return, mint never unwinds** (that would mean rewriting published history). Failures in stage 7 warn and point to the heal path.
- One mental model: *nothing mint did this run survives unless the release completes.*

The per-stage details are specified in their own sections below.

---

## Working Notes
