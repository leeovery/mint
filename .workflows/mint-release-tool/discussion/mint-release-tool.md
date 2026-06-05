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
  ├─ ✓ Hook mechanism [decided]
  ├─ ✓ Hook points (which stages) [decided]
  ├─ ✓ Hook contract & commit interplay [decided]
  ├─ ◐ AI release notes [exploring]
  │  ├─ ✓ Diff-exclude globs ("mint ignore") [decided]
  │  └─ ◐ Prompt control: override & context injection [exploring]
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

## Hook mechanism

### Context

Hooks are mint's escape valve for steps that are *specific to one project* and that mint cannot know about generically. Critically, the discussion first **shrank** what hooks are for: anything universal-but-optional (version-file writing, diff-exclude globs) gets absorbed into mint as built-in config, because mint already owns those concerns and they're fragile string-work better tested once in Go. After that absorption, the only genuine hook use case across all the user's repos is *one*: `agentic-workflows` runs `npm ci && npm run build` to rebuild its knowledge bundle before tagging. So the hook system only needs to serve "run my own command at this lifecycle point."

### Journey

Started as a three-way fork — script files (`.release/hooks/pre-tag`) vs inline config commands vs both. The user collapsed it: an inline command string can simply *call* a script (`pre_tag = "./.release/hooks/build.sh"`), so "script vs inline" was never a real choice — inline strings subsume scripts. That removes a whole mechanism, a `.release/hooks/` convention to learn, and a second place to look. It mirrors the npm-scripts / GitHub-Actions `run:` model, which is familiar and proven.

### Decision

**Hooks are a config table of shell commands, keyed by lifecycle point.** One mechanism only.

```toml
[hooks]
pre_tag = "npm ci && npm run build"        # single string — the 90% case
# or:
pre_tag = ["npm ci", "npm run build"]      # array of strings, run in order
```

- **Value is a string *or* an array of strings.** Array entries run sequentially; the **first non-zero exit aborts the release**. (String-or-array accepted for ergonomics: string for one command, array for readable multi-step without quoting a giant `&&` chain.)
- **Executed through a shell** (`sh -c "<entry>"`) so `&&`, pipes, env vars, and `./script.sh` invocations all work. Run from the **repo root**.
- **mint injects `MINT_*` env vars** (new version, tag, dry-run flag, etc.) so commands/scripts have context. Exact var set decided in the Hook contract subtopic.
- **Script files are not a separate mechanism** — they're just something a command string can call. Complex/conditional logic lives in a script; the config points at it.

### Why not a `.release/hooks/` directory convention

No capability is lost by dropping it: testable/complex logic still lives in a script file, the config just references it. One mechanism, one place to look. `mint init` may still scaffold an example script + reference, but the directory is not load-bearing.

Confidence: high.

---

## Hook points (which stages)

### Context

Which lifecycle stages expose a hook. Kept minimal — only points with a real or near-certain use case, since adding a point later is trivial under the config-table mechanism.

### Decision

Three points, mapped to the spine:

- **`preflight`** — runs after mint's built-in preflight checks; for project-specific gates/validation. Before any mutation.
- **`pre_tag`** — stage 3 project prep (build/generate artifacts; the knowledge bundle). Dirties the tree → mint commits per the interplay rule below.
- **`post_release`** — stage 7 follow-ups after the GitHub release (notifications, tap `repository_dispatch`, etc.).

**No `post_tag`** (between tag/push and publish) — no use case; YAGNI.

Confidence: high.

---

## Hook contract & commit interplay

### Context

How mint invokes a hook, how it passes context, what a non-zero exit means, and how hook-produced changes reconcile with the clean-tree-before-tag invariant (review finding F5).

### Decision — commit interplay

**After a `pre_tag` hook runs, mint commits whatever it left dirty** (standard message, e.g. `chore(release): pre-tag artifacts for v1.4.0`). Consequences:

- Simple hooks never touch git — they just build; mint handles the commit.
- "Commit only if the bundle changed" falls out for free: changed → tree dirty → mint commits; unchanged → tree clean → nothing committed.
- A hook that wants a *custom* commit can do its own and hand mint back a clean tree — mint then sees nothing to do.

Either way mint never tags a dirty tree, and hook authors aren't forced to know git.

### Decision — failure behaviour (asymmetric across the point of no return)

- **`preflight` / `pre_tag`** run *before* the tag is pushed → **non-zero exit aborts the whole release cleanly** (no tag, no damage).
- **`post_release`** runs *after* the tag is live → it **cannot abort**; a non-zero exit just **warns** ("post_release hook failed; tag is already published"). Same principle as a failed `gh release create`.

### Decision — invocation & context

- Each hook entry runs via `sh -c` from the **repo root**.
- mint injects env vars: `MINT_NEW_VERSION` (`1.4.0`), `MINT_PREVIOUS_VERSION` (`1.3.2`), `MINT_VERSION_TAG` (`v1.4.0`), `MINT_BUMP` (`patch`/`minor`/`major`), `MINT_DRY_RUN` (`0`/`1`). Set may grow as stages need it.

### Deferred

- **Hooks under `--dry-run`:** lean is mint *skips* hooks by default (they have side effects) and reports that it skipped them. Final call made in the dry-run semantics subtopic (review F9).

Confidence: high.

---

## AI release notes — skeleton

### Context

Stage 4: generate a release-notes body from the diff since the last release. The body is reused in three places — the tag message, the CHANGELOG entry, and the GitHub release. Generate once, use everywhere. This section covers the structural/robustness half; the *quality* half (prompt, diff-excludes, context injection) is its own children below. Resolves review finding F7.

### Decision — skeleton

**A. Diff base.** Diff `last_tag..HEAD` (changes since the last release).
- **First release (no prior tag):** no base to diff and diffing the whole repo is useless to an AI → mint **skips the AI and uses a fixed body, "Initial release."**

**B. Engine.** Default the `claude` CLI (`claude -p`): mint composes the prompt, pipes it to the command's stdin, reads the body from stdout, with a timeout (~60s) so a hung call can't stall a release. The **command is overridable** via config (`ai_command`, default `claude -p`) — mint always *owns the prompt*; the command is just transport. Cheap future-proofing (swap binary/model) that keeps prompt-control working.

**C. Failure behaviour — fail loud by default.** Notes are generated at stage 4, *before* the tag (stage 6), so aborting on failure leaves **nothing tagged/pushed** — no mess to clean up. This is *why* blocking is safe and correct.
- **Config `on_notes_failure`, default `abort`** — if the AI can't produce a body (missing tool, timeout, error, diff exceeds `max_diff_lines`), mint **fails loudly and tags nothing**. An empty/garbage release is worse than a failed command (the user has been bitten by having to delete/hand-edit bad releases).
- **`fallback` mode** (opt-in) — proceed with a non-AI body instead of aborting. Fallback body defaults to the commit-subject list since last tag (lower-stakes since it only applies in this mode); can be a fixed configurable string.
- **`--no-ai`** is a *deliberate* skip, not a failure → always uses the fallback body, never aborts.

### Decided in passing

- **mint owns CHANGELOG generation** (confirmed direction) — it's the Record stage, and the AI body feeds the changelog entry. So the abort/fallback decision *also* protects the changelog: no body → no entry → no tag. Mechanics designed in the Changelog & version recording subtopic.

Confidence: high on skeleton.

---

## AI release notes — quality (children, parked)

### Diff-exclude globs — decided

- **`diff_exclude`** — a config array of globs kept out of the diff sent to the AI (knowledge bundle, minified output, lockfiles, generated code). Implemented via git's `:(exclude)` pathspec — git does the filtering. Connection: the same artifact a `pre_tag` hook builds is what gets excluded here.
- **Config array, not a `.mintignore` file** — consistent with the "everything in one config file, one place to look" principle (same call as hooks). These are *tracked, committed* generated files, so they're deliberately not in `.gitignore`. A `.mintignore` only earns its place if exclude sets grow large/gitignore-like — YAGNI today, addable later.
- **`max_diff_lines`, default 50000.** Reframed: not a model-context limit (modern windows are ~1M tokens) but a **cost + quality** guard — a huge diff is slow, costly, and summarises to mush. Lines are a cheap proxy for tokens (~10–20 tokens/line). Excluded paths don't count toward it. Exceeding it = a notes failure → abort-or-fallback per `on_notes_failure`. Fully overridable.

### Prompt control — exploring

- **Goal:** notes meaningfully *better* than today's output. Needs (a) ability to **override** the prompt entirely, and/or (b) **inject project context**. Per-project. Designing now — first pinning down what "better" means (the spec for mint's default prompt), then the override/inject mechanics.

---

## Summary

### Key Insights

*(to be filled as the discussion progresses)*

### Open Threads

*(to be filled)*

### Current State

- Nothing decided yet — discussion just initialized from the design handoff.
