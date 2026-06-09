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

## Commit Message Format & Prompt

L3 owns this prompt; it is simply a different default from release's.

**Default format = Conventional Commits 1.0.0** (the formal standard, conventionalcommits.org):
- `type(scope): description` subject line — imperative, concise; optional blank line + wrapped body for the *why*.
- Chosen because the user's own repos already use conventional commits (`discussion(...)`, `chore(...)`, `docs(...)`).
- **AI infers the `type`** (feat/fix/chore/docs/…) from the diff — central to the format and reliably inferable.
- **Scope off by default** — scope conventions are project-specific and the AI guesses them inconsistently; better omitted than wrong. (Re-enabling/guiding scope is a prompt/config affordance if ever wanted.)
- **Two-knob override**, mirroring release: a commit-specific context-inject knob + a full prompt-override knob (key names in Config). Same "mint owns the prompt; `ai_command` is transport" model.

**No mint branding in the message text.** Commit does **not** use release's `commit_prefix` (🌿) — a conventional-commit message is plain `type(scope): …`, and forcing a 🌿 onto every commit is undesirable. `commit_prefix` stays a release-only concern (release commit + tag subject).

### The `$EDITOR` fallback — one degradation path for all "no AI message" cases

Three cases converge on dropping to `$EDITOR` with an empty/template message (behaving like plain `git commit`):

1. **`--no-ai`** — skip AI entirely; the user writes the message. No synthetic stub.
2. **AI generation failure** — if the AI errors or returns nothing usable after the engine's one retry, fall back to `$EDITOR` rather than abort. Low-stakes; the user is at the terminal anyway.
3. **`max_diff_lines` exceeded** — commit does **not** abort (release's notes-failure model is too harsh for a routine large commit). Fall back to `$EDITOR` with a clear note (*"diff too large to summarise — opening editor"*). `diff_exclude` still applies first, so excluded noise doesn't push a diff over the limit.

(The detailed semantics of the `$EDITOR` path — TTY requirement, save-as-accept, regeneration failure routing — are specified next.)

## `$EDITOR` Fallback — Path Semantics

The three "no AI message" cases (`--no-ai`, AI-generation failure, oversized diff) all drop to `$EDITOR`. This path is reconciled with the deferred-staging model, the gate, and the `-y`/non-TTY posture.

**Requires a TTY.** `$EDITOR` is inherently interactive. When a fallback fires under `-y` or non-TTY stdin (e.g. `mint commit -Apy --no-ai`, or `-Apy` when the AI fails / the diff is oversized), mint **fails loud** (*"no AI message and no interactive editor available"*) — it never hangs and never commits an empty message. This extends the gate's forbidden-combo philosophy (unattended + needs-human → fail loud) to the editor path. An unattended run with no message source is contradictory: `--no-ai` unattended has nothing to commit with, and an unattended user wants the AI anyway. **There is no `-m`/`--message` escape** — anyone needing unattended-with-own-message uses plain `git commit`; `mint commit` is for *minted* messages.

**The editor save *is* the accept event.** On the fallback path the editor replaces the `Continue?` gate (git-like):

- **No separate `Continue?` gate.** The gate governs the *AI-generated* message only; the fallback path uses the editor itself as the review. A non-empty save = accept; quit/empty = abort. (This reconciles "`--no-ai` behaves like plain `git commit`" with "gate ON by default" — the gate is AI-path-only.)
- **Staging applies on save.** Same "stage on accept" rule, where *save* is the accept: the editor opens against the real (unstaged) state; only on a non-empty save does mint apply `-a`/`-A` staging, then commit. Mutate-nothing-until-accept holds.
- **Empty/aborted editor = true no-op.** No staging applied, no commit, no push (even with `-p`). Nothing was mutated, so there is nothing to clean up.

**Regeneration failure routes here too.** If the user presses `r` (regenerate-with-context) at the gate and the regeneration fails after its one retry, mint treats it as any other AI failure → the `$EDITOR` fallback. One consistent rule: any failed AI generation lands at the editor. No special "re-show the prior message" path. (Under `-y`/non-TTY this is moot — `r` is an interactive-only gate action.)

## Interactive Review Gate

Commit reuses the cli-presentation `Continue?` gate rendering (`y`/`n`/`e`/`r`, Enter ⇒ accept), **ON by default**.

**Choice mapping for commit:**

- **`y` / accept** → stage (if `-a`/`-A`) then commit; then push if `-p`.
- **`n` / abort** → do nothing. **No auto-unwind needed** — nothing has been mutated yet (staging deferred to accept), so abort is a true no-op back to the pre-`mint` state.
- **`e` / edit** → edit the message in `$EDITOR`, used verbatim.
- **`r` / regenerate with context** → re-run the AI with a one-time context line. This *is* the "context injection" affordance from the user's original commit shell function. (Regeneration failure → `$EDITOR` fallback, per Fallback Semantics.)

**Posture — gate ON by default.** Interactive runs show the message + `Continue?`; `-y` skips it (auto-accept); the shared forbidden-combo rule applies (non-TTY stdin + no `-y` → fail loud). Chosen for consistency with release and the presentation model, and because seeing the minted message before it sticks is the point. The frequent one-liner stays fast via `-y` (`mint commit -Apy`).

- **Considered and rejected — gate OFF by default** (commit immediately, review opt-in): faster for the frequent case, but commits messages unseen — the exact pain the gate exists to kill. `-y` already covers the unattended case explicitly.

**The gate-abort refinement (key design correction).** Originally the flow staged `-a`/`-A` *before* the gate, which meant aborting would leave a mint-altered worktree — wrong; "abort" must mean the whole run is abandoned with no trace. The fix — **mint mutates nothing until accept** (staging deferred) — is the cross-cutting property that runs through the lifecycle, staging, and the never-unwind invariant.

## Auto-push Behaviour

- **Push is opt-in via `-p` / `--push`** (default: no push). **Flag-only — no config default** ("we never push without the `-p` flag"). `-p` is per-verb (release uses `-p` for `--patch`); the cross-verb `-p` divergence is intentional and acceptable (git subcommands carry their own flag meanings).
- **Push failure → keep the commit, warn clearly, do NOT unwind.** On a failed push (rejected, remote moved, no upstream, network), mint leaves the commit in place and reports clearly with the fix (re-run the push). Rationale: a push is a trivially repeatable manual fix, whereas unwinding the commit is messy and risky — the user may have had files staged before running `mint commit`, and resetting/unstaging could clobber that pre-existing state. Push is **not** an atomic point-of-no-return with unwind; it is a best-effort final step whose failure is reported, not repaired.
- **Upstream handling:** defer to git. `mint commit -p` runs a normal `git push` (current branch → its configured upstream). No upstream set → git's own failure, surfaced via the warn-clearly rule (*"commit is in place; set an upstream and push"*). mint adds no special upstream logic.

## Invariant — *mutate nothing until accept; never unwind after*

The push-failure decision plus the gate-abort refinement give one coherent rule:

- **Before gate-accept, mint mutates nothing** — staging (`-a`/`-A`) is deferred to accept, so abort returns the user to their exact pre-`mint` state (their own prior staging untouched).
- **After accept, mint never unwinds a completed commit** — on a failed push it leaves the commit and reports clearly; it never unstages, resets, or rewrites.

This is the deliberate opposite of `mint release`'s auto-unwind model. The reason is the staging-safety concern: a local commit verb must never risk the user's working/staged state. There is **no destructive cleanup path at all** — failures either left nothing behind (pre-accept) or leave a clean forward-only commit the user can act on manually (post-accept).

## Preflight & Safety

A commit is a frequent, low-stakes, *local* act — most of release's strict gates are actively wrong for it. Commit's preflight is minimal.

**Commit runs only:**

1. **Git repo present** — anchored at the repo root (same resolution as release).
2. **Something to commit** — after staging; empty → fail loud (see Staging).

**Gates commit deliberately DROPS (and why):**

- **Clean-working-tree — dropped.** Commit exists *to* operate on a dirty tree; the release gate is the direct opposite of commit's purpose.
- **On-release-branch — dropped.** Commits legitimately happen on feature branches all day.
- **Remote-in-sync (behind/diverged) — dropped.** You commit while behind origin constantly; blocking that would be absurd.
- **No pre-push gate even with `-p`.** Consistent with the auto-push decision — mint doesn't gate the commit on push-ability; it attempts the push and *reports* failure. No remote-sync precheck.

This makes commit's safety posture the inverse of release's: release forces a known-good, clean, in-sync starting state because it is high-consequence; commit assumes a messy in-progress tree because that is its entire reason to exist.

## Config Schema

With mint now multi-verb, the config shape is **verb-namespaced tables + shared engine keys** (not flat-with-prefixes). Shared engine keys sit at the top *because* they serve every verb; each verb gets its own table.

```toml
# Engine-level — shared by every verb
ai_command     = "claude -p"
diff_exclude   = ["skills/**/knowledge.cjs", "*.min.js"]
max_diff_lines = 50000

[release]
tag_prefix       = "v"
commit_prefix    = "🌿"
release_branch   = "main"
changelog        = true
publish          = true
context          = "..."      # was notes_context
prompt           = "..."      # was notes_prompt
on_notes_failure = "abort"
# version_file, version_pattern, provider, ...

[commit]
context = "Conventional Commits; dev-workflow toolkit."   # inject into the commit prompt
prompt  = ".mint/commit-prompt.md"                        # full prompt override

[release.hooks]                                           # hooks nest under the owning verb
pre_tag = "npm ci && npm run build"
```

**Commit's config surface:**
- **Reused (shared) keys:** `ai_command`, `diff_exclude`, `max_diff_lines` — same values serve both verbs.
- **Commit-specific keys:** `[commit].context` (context-inject knob) and `[commit].prompt` (full prompt override). Both optional, typed, fail-loud — consistent with the existing config model.

**Hooks nest under the owning verb.** Top-level is strictly shared-engine, so a top-level `[hooks]` would contradict the "top-level = shared by every verb" rule. Commit defines **no** hook points (release owns `preflight`/`pre_tag`/`post_release`, mapped to its spine), so there is no `[commit.hooks]` today; it is the natural slot if commit ever gains hooks.

**Deliberately NOT added for commit:**
- No push config — push is flag-only `-p`, never a default.
- No `on_notes_failure` analogue — commit's failure path is always the `$EDITOR` fallback.
- No scope toggle, no per-verb `ai_command`/`max_diff_lines` override — steer via `[commit].context`/`prompt`; promote to a `[commit]` key only if a real need appears.

**Reconciliation owed by the release spec (cross-spec hand-off).** This verb-namespaced shape *revises* release's already-concluded flat config layout (`notes_context` → `[release].context`, `notes_prompt` → `[release].prompt`, `[hooks]` → `[release.hooks]`, every flat release key moves under `[release]`, shared engine keys lift to the top). Cheap now (no code exists). Commit's spec **depends on** that restructured shape; the migration itself is the release spec's to absorb (formalised in Dependencies).

## CLI Surface & Flags

```
mint commit [flags]

  -a, --all          stage tracked changes first (git commit -a semantics)
  -A, --add-all      stage everything incl. untracked first (git add -A)
  -p, --push         push after committing (no push without this; no config default)
      --no-ai        skip AI; drop to $EDITOR
  -y, --yes          skip the review gate (auto-accept)
      --plain        plain output — global presentation flag, all verbs
```

**Bundles:** `mint commit -Ap` (add-all + push, gate shown) · `mint commit -Apy` (unattended).

`-p` = push is per-verb (release's `-p` = `--patch`); the cross-verb `-p` divergence is intentional and acceptable (git subcommands carry their own flag meanings).

**Resolved (consciously dropped):**

- **No `--dry-run`.** The review gate already *is* the preview-then-bail affordance (see the message, `n` aborts with zero mutation), and a commit is cheap to `--amend`. Release needs dry-run because it previews a whole irreversible pipeline; commit has no such pipeline.
- **No `--context` one-time-context flag.** The original shell function had it, but the user has never used it. Interactive `r` (regenerate-with-context) at the gate plus the `[commit].context` config cover the need. Dropped (YAGNI).
- **No `commit` shim.** `release` gets a per-project shim for muscle memory + `mint` delegation; `commit` is invoked directly as `mint commit` (the user aliases it personally if desired).

## Dependencies

Prerequisites that must exist before implementation can begin:

### Required

| Dependency | Why Blocked | What's Unblocked When It Exists |
|------------|-------------|--------------------------------|
| **CLI Presentation** (`cli-presentation` spec) | Commit renders *all* output and its review gate through the `Presenter` seam — pretty/plain by `isatty`/`--plain`, `-y` auto-accept, the `Continue?` gate rendering, and the shared non-TTY forbidden-combo rule. None of commit's interactive flow can be built without this seam. | The entire commit presentation surface: gate rendering, pretty/plain modes, `--plain`/`-y` handling, and the fail-loud forbidden-combo behaviour. |

### Partial Requirement

| Dependency | Why Blocked | Minimum Scope Needed |
|------------|-------------|---------------------|
| **Mint Release Tool** (`mint-release-tool` spec) | Commit consumes the shared, content-agnostic AI engine and the verb-namespaced config — both established/restructured by the release spec. L2 (the engine), L1's `diff_exclude`/`max_diff_lines` logic, `git_safe`, and the `[commit]` config table cannot be built until these shared pieces exist in their reconciled form. | The shared AI engine (L1 context builder + L2 message engine), `git_safe` lock-resilient git, and the **verb-namespaced config restructure** (shared engine keys at top + per-verb tables, hooks nested under their verb). Commit does **not** depend on the release spine, version detection, tags, changelog, or publish. |

### Notes

- **Build order:** CLI Presentation → Mint Release Tool (establishes engine, config, consumes Presenter) → Commit. Commit is the last of the three to be implementable because it reuses all of the shared primitives.
- **Designed clean, not retrofitted:** the three-layer engine split is *designed* in commit's discussion but is owed to the release spec as a reconciliation. Commit's L3 glue (Conventional Commits prompt, `[commit]` knobs, commit sinks) is the only engine-related code unique to commit; it can be written as soon as L1/L2 exist.
- **Config reconciliation is the release spec's to absorb:** commit only depends on the *result* (the verb-namespaced shape). It introduces no migration work of its own.
- **Gate-rendering reconciliation flows through:** the `Continue?` gate rendering commit consumes is the subject of an open reconciliation owed by the release spec (cli-presentation's `[a]/[q]`→`Continue?`, replacing the stale release gate keys). Commit inherits whatever that reconciled rendering becomes — it introduces no gate-rendering work of its own, but its review gate is only as settled as that reconciliation.

---

## Working Notes

[Optional - capture in-progress discussion if needed]
