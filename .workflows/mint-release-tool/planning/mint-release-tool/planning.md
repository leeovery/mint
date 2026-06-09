# Plan: Mint Release Tool

## Phases

### Phase 1: Walking Skeleton — First-Release Cut, End-to-End
status: approved
approved_at: 2026-06-09

**Goal**: A repo with no tags can run `mint release` and produce `0.0.1` end-to-end — version computed from git tags, core preflight gates passed, the fixed "Initial release." body recorded, an annotated tag created, `git push --atomic`, and a GitHub release published — threading through every architectural seam (CommandRunner, config load, the Presenter interface via a recording fake, and the Publisher interface).

**Why this order**: This is the thinnest complete thread through the whole stack and proves the release spine and the point-of-no-return model at the cheapest possible moment. First-release deliberately needs no AI, no hooks, and no version-file projection, so it isolates the architecture itself. It establishes the seams and patterns every later phase consumes, and honours the cross-spec dependency by building the engine against the Presenter interface (recording fake), not a concrete renderer.

**Acceptance**:
- [ ] With no matching tags the current version resolves to `0.0.0`; `mint release` → `0.0.1`, `-m` → `0.1.0`, `-M` → `1.0.0`
- [ ] Tag grammar `^{tag_prefix}(\d+)\.(\d+)\.(\d+)$` (strict 3-part SemVer) parsed; non-matching tags ignored; "latest" is the global numeric maximum; `tag_prefix` default `"v"` read off tags and written back when tagging
- [ ] Core preflight gates run in order — git repo anchored at `git rev-parse --show-toplevel`, on release branch (auto-derived from `origin/HEAD`), clean working tree (porcelain), target tag free locally and on remote, remote sync (abort if behind/diverged) — and abort cleanly on failure
- [ ] First-release path writes the fixed body `"Initial release."` with no AI invocation
- [ ] An annotated tag (subject `{commit_prefix} Release {tag}` + the full body, default prefix 🌿) is created and `git push --atomic origin HEAD {tag}` sends commit(s) + tag together
- [ ] `gh` install+auth is gated only when actually publishing and before the tag; the GitHub `Publisher.CreateRelease` is invoked behind the `Publisher` interface
- [ ] The engine emits semantic events through the `Presenter` interface, verified with a fake/recording presenter; no concrete renderer is required to build or test the engine
- [ ] All external commands (`git`, `gh`, `claude`) run behind the `CommandRunner` seam and are mocked in tests

#### Tasks
status: approved
approved_at: 2026-06-09

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| mint-release-tool-1-1 | Project skeleton & CommandRunner seam | non-zero exit captured with stderr, command-not-found surfaced |
| mint-release-tool-1-2 | Minimal config load (tag_prefix, commit_prefix, release_branch, publish) | file absent → all defaults, only a subset of keys present, comments/blank file |
| mint-release-tool-1-3 | Version determination from git tags | no matching tags → 0.0.0, non-matching tags ignored (1.2, 1.2.0-rc.1, 1.2.0.4, release-1.2), mixed prefixes, double-digit segments sorted numerically |
| mint-release-tool-1-4 | Repo root anchoring & release-branch resolution | not a git repo → abort, origin/HEAD unset, release_branch config override |
| mint-release-tool-1-5 | Local preflight gates (clean tree, on branch, tag-free local) | dirty tracked changes, non-ignored untracked files, gitignored files exempt, not on release branch, tag exists locally |
| mint-release-tool-1-6 | Network preflight gates (fetch --tags, remote sync, tag-free remote) | behind → abort with count, diverged → abort, ahead → pass, no upstream, tag exists on remote |
| mint-release-tool-1-7 | Presenter interface & recording fake | none |
| mint-release-tool-1-8 | Publisher interface & GitHub driver (gh gate when publishing) | publish=false → no gh gate / no publish, gh missing → abort before tag, gh unauthenticated → abort before tag |
| mint-release-tool-1-9 | First-release body & Record (changelog + bookkeeping commit) | CHANGELOG.md absent → create with KaC preamble, no-op changelog → skip commit |
| mint-release-tool-1-10 | Annotated tag & atomic push | push rejected → surfaced, no publish attempted |
| mint-release-tool-1-11 | Release command wiring (end-to-end first-release) | -m → 0.1.0, -M → 1.0.0, default → 0.0.1, publish failure after push → warn only (post-PONR) |

### Phase 2: AI Release Notes Engine, Change Map & Interactive Review
status: approved
approved_at: 2026-06-09

**Goal**: Releases with a prior tag generate a notes body from the `last_tag..HEAD` diff via the layered AI engine (context assembly vs transport), prepend a computed Change Map, distribute the single body whole to the tag annotation, CHANGELOG.md, and provider release, and gate on the interactive `y`/`n`/`e`/`r` notes review.

**Why this order**: This is mint's core value-add and its heaviest seam set, built on the proven spine from Phase 1. It completes the Presenter cross-spec seam by exercising the four semantic review choices through the interface. It must precede hooks, projection, and regenerate because those consume the notes engine and the single-body distribution model established here.

**Acceptance**:
- [ ] Context assembly (diff base `last_tag..HEAD`, always-exclude `CHANGELOG.md`, `max_diff_lines` guard default 50000) is cleanly separated from content-agnostic AI transport (prompt + `ai_command` default `claude -p`, validate non-empty/not-error/not-refusal, one retry, ~60s timeout not retried)
- [ ] A Change Map is computed after exclusion (structural novelty weighted above magnitude, directory/area rollup with notable files called out) and prepended to the AI input
- [ ] Default notes format is Keep-a-Changelog taxonomy in mint's emoji skin with a TL;DR one-liner; empty sections omitted; "no preamble/meta-commentary" and "ignore version bumps" prompt rules applied
- [ ] Notes-path precedence resolved: first-release fixed body > degenerate (empty/all-excluded) stub > `--no-ai` fallback body > normal AI path; `on_notes_failure` (abort default / fallback) governs only the normal path
- [ ] The single body is written whole to the annotated tag (the one read source), the CHANGELOG.md projection, and the provider release — no parsing or per-sink reassembly
- [ ] Interactive gate offers `y`/`n`/`e`/`r` (`r` omitted on no-AI paths); `e` opens `$VISUAL`→`$EDITOR`→`vi` and uses saved text verbatim (returns to gate if no editor launches); `r` appends a one-time context line and re-runs; `-y`/`--yes` skips the gate
- [ ] Answering `n` (abort) triggers a full auto-unwind to the exact clean starting state

#### Tasks
status: approved
approved_at: 2026-06-09

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| mint-release-tool-2-1 | AI transport layer (content-agnostic) | empty/whitespace body → retry then fail, error/refusal text → retry then fail, timeout → not retried, ai_command override |
| mint-release-tool-2-2 | Diff context assembly (last_tag..HEAD, CHANGELOG.md always-excluded) | CHANGELOG.md changes excluded, force-added gitignored file still appears, no source change after exclude |
| mint-release-tool-2-3 | max_diff_lines guard (default 50000) | exactly 50000 passes, over → notes failure, configurable override, excluded paths not counted |
| mint-release-tool-2-4 | Change Map salience preamble | new directory/package headline above magnitude, renamed/removed paths, single largest file called out, all changes in one existing area |
| mint-release-tool-2-5 | Default notes prompt & Keep-a-Changelog emoji-skin format | context inject appended, prompt full-override file, Deprecated/Security opportunistic only on explicit marker |
| mint-release-tool-2-6 | Normal AI notes path wiring (prior-tag release) | body used whole (no parsing), valid generation passes through unchanged |
| mint-release-tool-2-7 | on_notes_failure resolution (abort default / fallback) | abort default tags nothing, fallback → commit-subject list, fallback → fixed configurable string, varied failure causes |
| mint-release-tool-2-8 | Degenerate-diff stub path | all files fell under exclusion, whitespace-only diff, no notable source change, AI never invoked |
| mint-release-tool-2-9 | --no-ai fallback path | never aborts even when AI would fail, commit-subject list body, fixed-string fallback config |
| mint-release-tool-2-10 | Notes-path precedence resolution | first-release wins over --no-ai and degenerate, degenerate wins over --no-ai, on_notes_failure only governs normal path |
| mint-release-tool-2-11 | Single-body distribution to all sinks | changelog=false skips CHANGELOG (tag still carries body), publish=false skips provider, identical body across sinks |
| mint-release-tool-2-12 | Interactive review gate semantics (y/n/e) | bare Enter → accept, e saved text verbatim (no re-validate), -y skips entirely, gate before any mutation |
| mint-release-tool-2-13 | Editor resolution for `e` | $VISUAL set, only $EDITOR set, neither → vi, no launchable editor → report and return to gate |
| mint-release-tool-2-14 | `r` regenerate-with-context (loop) & no-AI gate variant | r omitted on first-release/degenerate/--no-ai paths, multiple r loops, context line not persisted to config |
| mint-release-tool-2-15 | Abort auto-unwind from the gate (`n`) | unwind back to clean state, identical to pre-push failure path, no tag/commit survives |
| mint-release-tool-2-16 | End-to-end prior-tag release wiring | generated body flows to all three sinks, gate accept proceeds to record, gate abort leaves repo clean |

### Phase 3: Project Prep — Hooks, Version-File Projection & Diff Exclusion
status: approved
approved_at: 2026-06-09

**Goal**: Projects can configure `preflight`/`pre_tag`/`post_release` hooks and a version-file projection, and the diff sent to the AI is shaped by exclusion (built-in `CHANGELOG.md`, `diff_exclude` globs, and the strategy-aware `version_file` rule).

**Why this order**: This adds the project-customisation surface on top of the working notes engine and record stage. `pre_tag` feeds the commit graph built in Phase 1; the version-file projection extends Record; diff exclusion refines the notes input from Phase 2 — so all its dependencies already exist.

**Acceptance**:
- [ ] Hooks run via `sh -c` from repo root with injected `MINT_*` env vars; value is string or array (sequential, first non-zero aborts for pre-PONR); `post_release` failure warns only
- [ ] `preflight` hook runs after built-in gates (before mutation); `pre_tag` dirties the tree → mint commits `chore(release): pre-tag artifacts for {tag}` as its own commit; clean tree → no commit
- [ ] Version-file projection runs in Record: plain mode (whole file is the version) and embedded mode (`version_pattern`); pattern-mismatch aborts before the tag; multiple matches all replaced
- [ ] Diff exclusion via git `:(exclude)`: always-exclude `CHANGELOG.md`, `diff_exclude` glob array, and strategy-aware `version_file` handling; exclusion is path-based; `max_diff_lines` excludes excluded paths
- [ ] Commit graph supports up to two commits (hook artifacts then bookkeeping) with no-op safety (no empty commits); `--dry-run` skips all hooks and reports they were skipped

#### Tasks
status: approved
approved_at: 2026-06-09

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| mint-release-tool-3-1 | Hook runner foundation (sh -c, repo root, MINT_* env, string\|array) | string single command, array run in order, first non-zero exit stops the sequence, MINT_BUMP=explicit for --set-version, empty/absent hook → no-op |
| mint-release-tool-3-2 | preflight hook (runs after built-in gates, aborts on non-zero) | runs after built-in gates pass, non-zero → clean abort pre-mutation, absent hook → skipped, array first failure aborts |
| mint-release-tool-3-3 | pre_tag hook execution & artifact commit (commit-interplay rule) | hook dirties tree → own commit, clean tree → no commit, hook makes its own commit + hands back clean → nothing committed, non-zero exit → clean abort, gitignored outputs don't count as dirty |
| mint-release-tool-3-4 | post_release hook (warn-only on failure) | non-zero → warn only, runs after publish, absent → skipped, array continues semantics post-PONR |
| mint-release-tool-3-5 | Version-file projection — plain mode (whole file is the version) | file absent → created, file already holds target version → no-op (no empty commit), trailing newline handling |
| mint-release-tool-3-6 | Version-file projection — embedded mode (version_pattern) | pattern matches nothing → abort before tag, multiple matches → all replaced, already at target version → no-op, {version} placeholder substitution |
| mint-release-tool-3-7 | Bookkeeping commit folds changelog + version-file projection | both changelog and version file change → one commit, version unchanged but changelog changes → still commits, nothing net-changed → no empty commit |
| mint-release-tool-3-8 | Up-to-two-commit graph (hook-artifact then bookkeeping) | hook commit + bookkeeping commit (two), no hook dirt → one commit, neither dirty → zero commits, tag always points at bookkeeping/HEAD |
| mint-release-tool-3-9 | Configurable diff_exclude globs (on top of built-in CHANGELOG.md) | multiple globs, glob matches nothing, force-added gitignored file still excluded by glob, excluded paths not counted toward max_diff_lines, combined with CHANGELOG.md exclusion |
| mint-release-tool-3-10 | Strategy-aware version_file diff exclusion (plain excludes, embedded doesn't) | plain mode → version_file excluded, embedded mode → version_file NOT excluded, no version_file → nothing added, forward path inherently unchanged so no effect, version_file also in diff_exclude |
| mint-release-tool-3-11 | --dry-run skips all hooks and reports skipped | all three hook points skipped + reported, MINT_DRY_RUN=1 injected, no artifact commit when hooks skipped, dry-run note caching out of scope (Phase 4) |

### Phase 4: Robustness — Lock Resilience, Recovery, Dry-Run Caching & Publisher Resolution
status: approved
approved_at: 2026-06-09

**Goal**: The forward pipeline is production-hardened — lock-resilient git on every mutation, surgical auto-unwind on pre-PONR failure, the `--autostash`/`--any-branch`/`--set-version` escape hatches, dry-run note caching for deterministic preview→ship, and full provider auto-detection with safe downgrade.

**Why this order**: This is the hardening layer over the now-complete forward pipeline. It refines failure and edge behaviour rather than adding new user-facing capabilities, so it belongs after the forward path's capabilities are all in place and before the separate regenerate command.

**Acceptance**:
- [ ] All git mutations are wrapped in lock resilience (retry on a contended `.git` lock; clear a provably-stale lock)
- [ ] Pre-PONR failures auto-unwind surgically (delete the tag created, reset the N commits) to the exact clean starting state and report what was undone; post-PONR never unwinds — publish failure warns and points to the heal path
- [ ] `--autostash` stashes `--include-untracked` before the run and restores after unwind, leaving the stash intact and warning on pop conflict; `--any-branch` bypasses the branch gate; `--set-version X.Y.Z` validated (mutually exclusive with bump flags, valid 3-part, strictly greater than latest)
- [ ] `--dry-run` generates the notes preview and caches it; the real run reuses on a key match (hash of post-`diff_exclude` diff + computed version + prompt/`context`), regenerates + reports on miss, with ~1h TTL, gitignored and never committed; the review gate is unaffected
- [ ] Provider is auto-detected from the remote host (`github.com` → GitHub); an unknown `provider` value, an unmatched host, or no remote with `publish = true` warns loudly and downgrades to tag + push only — never silently assumes GitHub, never strands a pushed tag

#### Tasks
status: approved
approved_at: 2026-06-09

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| mint-release-tool-4-1 | Lock-resilient git wrapper (git_safe built-in) | contended .git lock → retry then succeed, provably-stale lock → cleared, live/fresh lock not cleared, retries exhausted → surface error, applied to every mutation path |
| mint-release-tool-4-2 | Surgical pre-PONR auto-unwind (delete tag + reset N commits + report) | zero commits made, one commit, two commits (hook-artifact + bookkeeping), tag created vs not-yet-created, reports each undone item, post-PONR never unwinds |
| mint-release-tool-4-3 | Gate-abort & pre-push failure route through surgical unwind | gate n → surgical unwind, pre-push git failure → surgical unwind, push succeeds + publish fails → warn only (no unwind), identical clean-state result for n and failure |
| mint-release-tool-4-4 | --autostash stash/restore with unwind ordering | clean restore after success, restore after abort (unwind then pop), pop conflict → stash kept + warn (WIP never discarded), untracked files stashed, no WIP → no-op |
| mint-release-tool-4-5 | --any-branch branch-gate bypass | off-branch + flag → passes, off-branch without flag → still aborts, on-branch + flag → no effect |
| mint-release-tool-4-6 | --set-version explicit version validation (MINT_BUMP=explicit) | combined with -p/-m/-M → error, malformed/non-3-part semver → error, equal to latest → reject, less than latest → reject, greater → accepted, first release (latest 0.0.0), MINT_BUMP=explicit injected |
| mint-release-tool-4-7 | Dry-run note cache write & key computation | cache dir gitignored/temp, key includes context/prompt, TTL stamp written, repo-scoped key, dry-run still skips hooks |
| mint-release-tool-4-8 | Real-run cache reuse, miss-regenerate & TTL/gate orthogonality | key match → reuse (no AI call), diff-changed miss → regenerate + report, expired TTL → regenerate, non-excluded pre_tag change → correct miss, excluded hook artifact → still reuse, cached note still shown at gate, -y still skips |
| mint-release-tool-4-9 | Provider auto-detection from remote host | github.com remote → GitHub driver, SSH github.com URL → GitHub driver, explicit provider config overrides detection, detection behind Publisher interface |
| mint-release-tool-4-10 | Safe downgrade to tag+push on unresolved provider | unknown provider value → warn + downgrade, GHE/GitLab/Gitea host → warn + downgrade, no remote → warn + downgrade, unmatchable SSH → warn + downgrade, publish=false → silent tag+push (no warn), gh gate skipped so tag never stranded |

### Phase 5: Regenerate / Backfill (Heal & History Rewrite)
status: approved
approved_at: 2026-06-09

**Goal**: `mint release regenerate <version>` and `--all` non-destructively heal or rewrite the mutable surfaces (provider release body and CHANGELOG.md) from either `--reuse` (tag annotation body) or fresh (re-diff `vX-1..vX` + AI), for one release or a batch, never touching tags.

**Why this order**: Regenerate is a distinct command with its own preflight subset, two-axis source×target contract, and batch semantics. It consumes the notes engine (Phase 2), the Publisher (Phases 1/4), and the record/changelog surfaces (Phases 1–3), so it must follow them.

**Acceptance**:
- [ ] Two-axis contract enforced: `--reuse` reads the tag body, implies `--target release`, and errors on `--target changelog`; fresh re-diffs `vX-1..vX` with the same exclusion tiers + Change Map and targets `release`/`changelog`/`both`
- [ ] `<version>` normalised with or without `tag_prefix`; no matching tag → fail loud; oldest release (no `vX-1`) → fixed body `"Initial release."`; `--reuse` against a tag with no annotation body → error (single) / skip-and-report (`--all`)
- [ ] Argument validation: bare `regenerate` (neither `<version>` nor `--all`) errors; both errors; fresh `-y` without `--target` errors; `--target changelog`/`both` with `changelog = false` aborts up front
- [ ] Per-verb preflight subset applied: `--reuse` → gh-auth only; fresh changelog/both → gh-auth + clean-tree + branch + remote-sync (not tag-free, no version compute)
- [ ] Provider create-or-update is automatic (probe per version); fresh runs the notes-review gate (`-y` skips); `git push origin HEAD` is the PONR with reset-on-abort and warn-only on post-push provider failure; `--target both` is not atomic across surfaces
- [ ] `--all` runs oldest→newest, skip-and-continue with an end summary; whole-file CHANGELOG rebuild with one commit at the end; single-version uses idempotent in-place section replace

### Phase 6: Config Schema & `mint init` Scaffolding
status: approved
approved_at: 2026-06-09

**Goal**: The full verb-namespaced TOML schema is parsed with typed, fail-loud validation, and `mint init` activates mint in a project by scaffolding the commented `.mint.toml` and the `release` shim; `mint version` completes the CLI surface.

**Why this order**: The schema accretes naturally across earlier phases (each consumes its own keys), so this phase consolidates complete validation and ships the activation surface that lets a project adopt mint — a fitting final increment once every key it scaffolds has working behaviour behind it.

**Acceptance**:
- [ ] Shared engine keys (`ai_command`, `diff_exclude`, `max_diff_lines`) at top level plus `[release]` and `[release.hooks]` tables parsed; zero config yields sensible defaults everywhere
- [ ] Typed, fail-loud validation: unknown keys and bad types error with clear messages; an unknown `provider` value warns + downgrades rather than erroring
- [ ] `mint init` drops a commented `.mint.toml` (defaults + present-but-commented optional keys) and an executable `release` shim; idempotent/non-clobbering with a notice; `--force` regenerates; no project auto-detection and no hook/prompt files scaffolded
- [ ] The `release` shim execs `mint release "$@"` and, when mint is absent, prints the `brew install leeovery/tools/mint` hint and exits non-zero
- [ ] `mint version` and `mint --version` print mint's own version
