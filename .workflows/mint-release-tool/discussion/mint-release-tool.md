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
  ├─ ✓ Tag format, prefix & pre-releases [decided]
  ├─ ✓ Safety & preflight gates [decided]
  ├─ ◐ Hook mechanism [exploring]
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

**Lineage — old `VERSION_STRATEGY` → new model** (all three absorbed, none lost):
- old `none` → default (no `version_file`); tag is truth.
- old `file` (plain `release.txt`) → `version_file = "release.txt"` (no pattern).
- old `embedded` (sed-replace `RELEASE_VERSION="x.y.z"` in a source file) → `version_file` + `version_pattern = 'RELEASE_VERSION="{version}"'`.
The behavioural change: these are now write-only mirrors, not read sources.

### Notes / deferred (Version)

- **Brew formula version bump is NOT mint's job.** The formula's version + sha256 are bumped downstream by the tap's auto-update CI reacting to the GitHub release mint creates. Most repos mint releases aren't formulas anyway. If a project ever wants mint to actively trigger it (`repository_dispatch`), that's a **post-release hook**, not engine code. Tracked as a child of Tag/push/publish.

Confidence: high.

---

## Tag format, prefix & pre-releases

### Context

Grew out of the version decision (review finding F1). "Latest `vX.Y.Z` tag" left several tag shapes undefined: pre-release/RC tags, prefixed/component tags, the `v` prefix itself, and 4th/build segments. Pinning the exact recognised grammar matters because mis-parsing a tag silently re-releases an existing version.

### Decision

**Standard: strict SemVer 2.0.0, three numeric segments only.** mint recognises exactly `MAJOR.MINOR.PATCH`. Anything else (`release-1.2`, `1.2`, `1.2.0.4`) is not a mint version — the project's problem, not ours.

**`tag_prefix` config, default `"v"`.** Industry default leans `v` (GitHub convention; Go *requires* it on module tags), but it's a per-project preference so it's overridable to `""` or anything else. mint reads the prefix off a tag, parses the semver, and writes the prefix back when tagging. One elegant consequence: the same knob covers component/monorepo prefixes — e.g. `tag_prefix = "pkg-name/v"`.

**Recognised tag grammar:** `^{tag_prefix}(\d+)\.(\d+)\.(\d+)$`. Tags not matching are ignored entirely.

**"Latest" = highest semver, globally** (resolves F2). Among all tags matching the grammar, pick the numerically highest version — *not* `git describe`'s nearest-reachable-from-HEAD, which diverges on branches and hotfix lines. Tag-as-truth requires the true maximum.

**Tag completeness** (resolves F3): preflight's fetch includes `--tags`, so mint always sees the complete tag set even after a fresh/partial clone. mint is a local interactive tool (not a CI job), so the CI `--no-tags` shallow scenario barely applies anyway.

### Explicitly rejected (YAGNI)

- **Pre-release / RC tags** (`1.2.0-rc.1`) — valid SemVer, but the user doesn't cut RC releases, so mint won't even *parse* them, let alone produce them. (Consequence accepted: a repo whose only tags are RC tags would read as `0.0.0`. Not a real scenario for these projects.) Re-addable later if a project needs it.
- **4th / build segment** (`1.2.0.4`) — not SemVer; breaks brew and tag-as-truth. SemVer's build metadata (`1.2.0+build5`) is precedence-ignored and not wanted. Docker image build numbers are stamped at image-build time in CI (`semver + git-sha`/run-number), off mint's released version — not baked into the release tag. mint stays strictly 3-part.

Confidence: high.

---

## Safety & preflight gates

### Context

Stage 2 of the spine: the "is it safe to release?" checks. All cheap and reversible, all run before any mutation or hooks. The guiding principle — releasing is high-consequence, so force a conscious, known-good starting state. The user has been bitten by *both* failure modes: blocked unnecessarily (annoyance) and the risk of releasing something stale/unreviewed (danger). The design favours safety, with escape hatches for the annoyance.

### Decision — the preflight gate set

Run in order; cheap local checks first, then network checks. Nothing irreversible until all pass.

1. **Git repo present**, anchored at repo root.
2. **On the release branch** — default-on, **auto-derived** from `origin/HEAD` (so it just resolves `main`/`master` with zero config). Override via `release_branch` in config; `--any-branch` escape hatch for the rare deliberate off-branch release. Rationale: we shouldn't release feature branches; auto-derivation means it protects with no config burden.
3. **Clean working tree (strict)** — `git status --porcelain` must be empty (gitignored files exempt, so build outputs don't trip it). Blocks on uncommitted/unstaged tracked changes *and* non-ignored untracked files. Rationale: a release is a big, consequential act — a clean slate forces the user to consciously check what's going out.
   - **`--autostash` opt-in flag** (not default): stashes (`--include-untracked`) before the run and restores after, **including on abort/failure**. Deliberately opt-in, not default, because the release mutates the tree (hook commits, changelog, version file) and popping unrelated WIP on top can conflict — a nasty failure mode to bake in by default. Opt-in = user asserts it's safe.
4. **Target tag is free** — computed `vX.Y.Z` must not exist locally or on the remote. Closes the double-release / re-run footgun (old script never checked this).
5. **Remote sync** — `git fetch`, then **abort (never auto-pull)** if local is *behind* or *diverged* from the release branch's upstream. Being *ahead* is fine and expected (those are the commits being released). Rationale: auto-pulling silently drags in unseen remote commits and releases them — integrating remote work must be a conscious act, not a side effect. Clear message on abort ("N commits behind origin/main — pull and review, then re-run").
6. **`gh` installed + authenticated** — gated only when actually publishing a GitHub release, and *before* the tag, so a missing/unauthenticated `gh` never strands a pushed tag with no release. (`claude` CLI is NOT a preflight gate — AI notes are optional with graceful fallback; see AI release notes subtopic.)

### Notes / deferred (Preflight)

- The exact membership above resolves the open "which tools gate the run" question: `gh` (conditional on publish), `git` (implied), `claude` optional.
- Repo-root anchoring with the global-binary + shim model (where mint sets its working dir; behaviour in submodules/worktrees) is an implementation detail flagged for spec, not re-litigated here.

Confidence: high.

---

## Summary

### Key Insights

*(to be filled as the discussion progresses)*

### Open Threads

*(to be filled)*

### Current State

- Nothing decided yet — discussion just initialized from the design handoff.
