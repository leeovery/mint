# Specification: Commit Command

## Specification

## Overview

`mint commit` is a sibling verb to `mint release` that produces an AI-generated commit message from the diff, built into the `mint` binary. It is a **thin standalone verb**: it does NOT ride the release lifecycle spine. Its core act is small and local: stage (optionally) → generate a message → optionally review → commit → optionally push.

The guiding design stance: **there is no code yet.** The shared AI machinery is designed up front to serve both verbs cleanly — `commit` is not retrofitted onto release-note generation, nor does it require reworking any settled release decision.

## Scope

**What `commit` reuses (shared primitives):**
- The `Presenter` seam — pretty/plain rendering, `-y` orthogonality, `--plain`, the `Continue?` review-gate rendering (defined in `cli-presentation`).
- The AI engine — transport `claude -p`, mint-owned prompt, fail-loud + retry, `--no-ai` skip (the three-layer engine, below).
- `git_safe` lock-resilient git.
- The TOML config model (verb-namespaced shape, below).
- `diff_exclude` globs and the `max_diff_lines` guard apply to commit's diff exactly as they apply to release's — we don't feed excluded files (bundles, lockfiles, minified output) into message generation.

**What `commit` does NOT touch:**
- Version detection, tags, changelog, publish/provider, the regenerate command.
- No point-of-no-return / atomic-push semantics — a commit is inherently local and reversible until pushed.

**Safety posture:** the inverse of release's. Release forces a known-good, clean, in-sync starting state because it is high-consequence; commit assumes a messy, in-progress working tree because operating on one is its entire reason to exist.

## AI Engine — Three-Layer Split

The AI message-generation concern is a shared engine that both `release` and `commit` consume. The git boundary sits in a single layer:

- **Layer 1 — Context builder (git-aware).** Produces the content to describe. Parameterised by *source*: release uses `tag..HEAD`; commit uses the staged diff (`git diff --cached`). Applies `diff_exclude` globs and the `max_diff_lines` guard — identical logic for both verbs, so this is the genuinely shared git piece. (Different git *providers* are a separate axis, handled by the existing driver/provider setup, not by this layer.)
- **Layer 2 — AI message engine (git-unaware, content-agnostic).** Inputs: an assembled prompt + the content + `ai_command`. Runs the transport, validates the output (non-empty / not an error / not a refusal), retries once, fails loud per policy, and returns the message body. **Knows nothing about git, diffs, tags, or commits — pure "context in, message out."**
- **Layer 3 — Per-verb glue.** Picks the L1 source, supplies the prompt + default format (release notes vs commit message), and decides the sinks. This is where the verbs differ.

**Content-agnostic is the load-bearing property.** The input being a diff is incidental; L2 only ever sees "content." It does not matter whether that content is a textual diff, an AST/semantic breakdown, or a human-written note — same engine. This is what lets the separate release-notes-quality work enrich L1's *input* with zero change to L2.

**Composition is permitted.** Keeping L1/L2/L3 as separate underlying pieces does not forbid a convenience wrapper (a local or exported function) that composes L1→L2→sink for a call site's ergonomics. Separation governs the *underlying pieces*, not a tidy facade over them.

**Prompt boundary.** L3 owns prompt assembly; L2 receives the finished prompt. This mirrors release's settled model — mint always owns the prompt; `ai_command` is just transport — with the two-knob model (context-inject + full-override). Commit gets its own default commit-message prompt and its own context/override knobs (see Config).

### Commit's binding to the engine
- **Layer 1 source:** the staged diff (`git diff --cached`), or the would-be-staged diff under `-a`/`-A` computed read-only (see Staging).
- **Layer 3 glue:** supplies the Conventional Commits default prompt/format, the `[commit]` context/override knobs, and the commit sinks (`git commit`, optional push).

## Commit Flow / Lifecycle

The core invariant that shapes the whole flow: **mint mutates nothing until the user accepts the gate.** Everything before accept is read-only — including the `-a`/`-A` staging, which is deferred to the accept path. This is what makes abort a true no-op.

The stages:

1. **Preflight (minimal)** — git repo present; *something to commit* (for `-a`/`-A`, the would-be-staged changes; otherwise the existing index). Computed read-only. Empty → fail loud.
2. **Build context (L1)** — filtered diff of what *would* be committed (default: `git diff --cached`; with `-a`/`-A`: the would-be-staged working-tree diff, computed **without** mutating the index), with `diff_exclude` + `max_diff_lines` applied.
3. **Generate (L2)** — the commit message (skipped under `--no-ai`; fallback path covered under Message Format).
4. **Review gate** — the same `Continue?` rendering as release, interactive only (see Review Gate).
5. **On accept** — apply `-a`/`-A` staging now (if given), then `git commit` (via `git_safe`).
6. **Push (optional)** — only if `-p`/`--push` (flag-only, no config default) (see Auto-push).

**Reversibility:** no point-of-no-return / atomic-push semantics — a commit is local and reversible. Before accept, nothing has been mutated (clean abort). After accept, a completed commit is never unwound by mint (partial-failure model under Auto-push).

## Staging Model

What goes into the commit. The design tension: the user's habit is `git add .` (which **includes new files**), but the natural "copy from git" flag `-a` maps to git's `commit -a`, which is **tracked-only**. Two faithful flags resolve the mismatch.

| Command | Modified tracked | Deleted tracked | New/untracked |
|---|---|---|---|
| `git commit -a` / `git add -u` | ✅ | ✅ | ❌ |
| `git add .` (from root) / `git add -A` | ✅ | ✅ | ✅ |

(mint runs from the repo root, so `git add .` ≡ `git add -A` for its purposes.)

**Decision — two faithful flags:**

- **Default = staged-only.** Commit the index exactly as staged. Respects deliberate staging; mint never decides *what* goes in unless asked.
- **`-a` / `--all` = `git commit -a`** — tracked modifications + deletions, no untracked. Muscle-memory faithful.
- **`-A` / `--add-all` = `git add -A` then commit** — everything including untracked. This is the user's `git add .` habit in one shot.
- **Staging is deferred to gate-accept.** With `-a`/`-A`, mint computes the would-be-committed diff *read-only* for message generation, and only runs `git add` after the user accepts. Aborting an `-a`/`-A` run leaves the index exactly as it was — mint never leaves a half-staged worktree behind.
- **Flags bundle:** `mint commit -Ap` = add-all + push with a minted message — the headline ergonomic target.

**Empty-staging handling — fail loud, mirroring git's messaging:**

- Empty staging (nothing to commit after staging) → **fail loud**; never invoke the AI on an empty diff. `-A`/`-a` that stage nothing land here too.
- Distinguish the two empty cases exactly as git does:
  - Genuinely clean tree → "nothing to commit, working tree clean".
  - Dirty-but-unstaged tree (bare `mint commit`, nothing in the index) → guide the user — mint's flavour of git's `no changes added to commit`, e.g. *"no changes staged — use `-a`/`--all`, `-A`/`--add-all`, or `git add`"*.

**Rationale:** the original commit shell function did not do its own `git add` (commit-only); the staging affordance is a deliberate enhancement in mint, not a port. The `git add .` habit (untracked included) is what tipped the choice to two flags — a faithful `-a` alone would silently drop new files, so the explicit `-A` covers the everything-case without overloading `-a`.

---

## Working Notes

[Optional - capture in-progress discussion if needed]
