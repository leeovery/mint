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

## Stage 1 — Version Determination & Tag Grammar

### Source of truth: git tags, always

The current version is the **highest** SemVer tag in the repository (stripped of its prefix). There is no file-based or embedded version source — brew installs from tags, so the tag *is* the real version; any file copy is derived state. With no matching tags, the current version is `0.0.0`.

- **"Latest" = the numerically highest matching version, globally** — not `git describe`'s nearest-reachable-from-HEAD (which diverges on branches and hotfix lines). Tag-as-truth requires the true maximum across all tags.
- Preflight's fetch includes `--tags`, so mint always sees the complete tag set even after a fresh/partial clone.

### Recognised tag grammar

- **Strict SemVer 2.0.0, three numeric segments only:** `MAJOR.MINOR.PATCH`. Anything else (`release-1.2`, `1.2`, `1.2.0.4`, `1.2.0-rc.1`, `1.2.0+build5`) is **not** a mint version and is ignored entirely.
- **Recognised pattern:** `^{tag_prefix}(\d+)\.(\d+)\.(\d+)$`. Tags not matching are ignored when computing "latest".
- **`tag_prefix` config, default `"v"`** — mint reads the prefix off existing tags, parses the semver, and writes the prefix back when tagging. Overridable to `""` or anything else. The same knob covers component/monorepo prefixes, e.g. `tag_prefix = "pkg-name/v"`.

### Bump selection

The next version is computed from the current version by a bump flag:

- `-p` / `--patch` — **default** when no flag is given
- `-m` / `--minor`
- `-M` / `--major`
- `--set-version X.Y.Z` — explicit version escape hatch (e.g. a deliberate 1.x → 2.0.0 jump)

**First release handles itself** with no special-casing: with no tags the current version is `0.0.0`, so `mint release` → `0.0.1`, `mint release -m` → `0.1.0`, `mint release -M` → `1.0.0`.

### `--set-version` rules

- **Mutually exclusive with bump flags.** `--set-version` combined with `-p`/`-m`/`-M` is an **error** ("can't combine `--set-version` with a bump flag") — no silent precedence. (`--set-version` alone = explicit; a bump flag alone = computed; neither = default patch.)
- **Must be valid 3-part semver AND strictly greater than the current latest tag.** A backwards/equal jump is rejected by default *even if the target tag is free*, because a lower version sorts below "latest" and corrupts tag-as-truth. (This sits on top of the free-tag preflight check, which catches an equal/existing tag.)
- **Forward-only today; no downgrade override.** A `--force`-style "re-tag an old line" escape is YAGNI and deliberately not built now.

### Optional version-file projection

When a project needs the version written *into the repo*, mint mirrors the new version into a file during the **Record** stage (Stage 5). The file is always a **write-only mirror kept in sync** — never a source of truth.

- `version_file` — path to write; **omit = tag-only** (no projection).
- `version_pattern` — e.g. `RELEASE_VERSION="{version}"`; **omit = the whole file *is* the version** (plain mode).

**Legacy strategy mapping** (the old `VERSION_STRATEGY` model collapses into this; all absorbed, none lost):
- old `none` → no `version_file` (tag is truth).
- old `file` (plain `release.txt`) → `version_file = "release.txt"`, no pattern.
- old `embedded` (sed-replace into a source file) → `version_file` + `version_pattern = 'RELEASE_VERSION="{version}"'`.

The behavioural change vs. legacy: these are now write-only mirrors, not read sources.

### Explicitly rejected (YAGNI)

- **Pre-release / RC tags** (`1.2.0-rc.1`) — not parsed or produced. (Accepted consequence: a repo whose only tags are RC tags reads as `0.0.0` — not a real scenario here.)
- **4th / build segments** (`1.2.0.4`, `1.2.0+build5`) — not SemVer 3-part; break brew and tag-as-truth. Docker/CI build numbers are stamped at image-build time off mint's released version, never baked into the release tag.

---

## Stage 2 — Preflight & Safety Gates

### Principle

Releasing is high-consequence, so mint forces a conscious, known-good starting state. All preflight checks are cheap and reversible, and all run before any mutation or hooks. The design favours safety, with explicit escape hatches for the cases where a gate would merely annoy.

### The gate set (run in order — cheap local checks first, then network)

Nothing irreversible happens until all applicable gates pass.

1. **Git repo present**, anchored at the repo root (resolved via `git rev-parse --show-toplevel`; mint runs from root).
2. **On the release branch** — default-on, **auto-derived from `origin/HEAD`** (resolves `main`/`master` with zero config). Override via `release_branch` in config. Escape hatch: `--any-branch` for a deliberate off-branch release.
3. **Clean working tree (strict)** — `git status --porcelain` must be empty. Gitignored files are exempt (build outputs don't trip it); blocks on uncommitted/unstaged tracked changes *and* non-ignored untracked files. Escape hatch: **`--autostash`** (opt-in, not default) stashes (`--include-untracked`) before the run and restores after, **including on abort/failure**. Opt-in because the release mutates the tree (hook commits, changelog, version file) and popping unrelated WIP on top can conflict — opting in is the user asserting it's safe.
4. **Target tag is free** — the computed `{tag_prefix}X.Y.Z` must not exist locally or on the remote. Closes the double-release / re-run footgun.
5. **Remote sync** — `git fetch`, then **abort (never auto-pull)** if local is *behind* or *diverged* from the release branch's upstream. Being *ahead* is fine and expected (those are the commits being released). Auto-pulling would silently drag in unseen remote commits and release them; integrating remote work must be a conscious act. Clear abort message, e.g. "N commits behind origin/main — pull and review, then re-run".
6. **`gh` installed + authenticated** — gated **only when actually publishing** a GitHub release, and **before the tag**, so a missing/unauthenticated `gh` never strands a pushed tag with no release.

### Tool gating summary

- **`git`** — implied/required.
- **`gh`** — gated conditionally, only when publishing.
- **`claude` CLI is NOT a preflight gate** — AI notes are optional with graceful fallback (see AI release notes).

### Project preflight hook

After mint's built-in preflight checks pass, the project's optional `preflight` hook runs (for project-specific gates/validation) — before any mutation. A non-zero exit aborts the release cleanly. (Detailed in the Hooks section.)

---

## Working Notes
