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

`mint` adopts a `mint <verb>` command namespace from the outset. The release command is `mint release`; the per-project `release` shim delegates to `mint release`. This **specification covers the release pipeline only**. `mint` is now an **epic**: the release pipeline is one specification within it, with sibling specifications for the CLI presentation layer and the `mint commit` verb. This spec is standalone and self-contained; cross-spec touch-points are recorded under Dependencies.

### In scope (this build)

The complete release pipeline end-to-end: version determination → preflight safety gates → project-prep hooks → AI release notes (with interactive review) → record (changelog + version file) → annotated tag + atomic push → publish (GitHub release + post-release hooks); plus the regenerate/heal command, the TOML config schema, the CLI surface, and `mint init` scaffolding.

### Out of scope (consciously deferred)

- **`mint commit`** — a sibling verb specified separately (shares the AI engine, config model, and presentation layer; not part of this spec).
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
   - **Submodule / worktree resolution:** `git rev-parse --show-toplevel` resolves to the innermost enclosing repository or linked worktree from the invocation directory, and mint operates on *that* repository. A **submodule** is treated as its own repo (its own tags and its own `.mint.toml`); a **linked worktree** resolves to the worktree root and shares the main repository's tag/ref store. mint always anchors to and runs from that resolved root.
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

## Hooks

### Purpose & scope

Hooks are mint's escape valve for steps **specific to one project** that mint cannot know about generically. Anything universal-but-optional (version-file writing, diff-exclude globs) is deliberately absorbed into mint as built-in, tested config rather than left to hooks. The guiding test: *if mint already owns the data/concern, it's core; hooks are only for genuinely bespoke project steps.*

### Mechanism: one mechanism only

Hooks are a **config table of shell commands keyed by lifecycle point** (`[release.hooks]` in `.mint.toml`). There is no separate `.release/hooks/` directory convention — a command string can simply *call* a script, so scripts are just something a string invokes, not a second mechanism.

```toml
[release.hooks]
pre_tag = "npm ci && npm run build"        # single string — the 90% case
# or:
pre_tag = ["npm ci", "npm run build"]      # array of strings, run in order
```

- **Value is a string *or* an array of strings.** Array entries run sequentially; the **first non-zero exit aborts** (for pre-PONR hooks). String for one command; array for readable multi-step without quoting a giant `&&` chain.
- **Executed through a shell** (`sh -c "<entry>"`) so `&&`, pipes, env vars, and `./script.sh` invocations all work.
- **Run from the repo root.**
- **Complex/conditional logic lives in a script file** that the config points at; `mint init` may scaffold an example script + reference, but the directory is not load-bearing.

### Hook points (three, mapped to the spine)

- **`preflight`** — runs *after* mint's built-in preflight checks (Stage 2), for project-specific gates/validation. Before any mutation.
- **`pre_tag`** — Stage 3 project prep (build/generate artifacts, e.g. a knowledge bundle). Dirties the tree → mint commits per the interplay rule below.
- **`post_release`** — Stage 7 follow-ups after the provider release (notifications, tap `repository_dispatch`, etc.).

**No `post_tag`** point (between tag/push and publish) and **no `pre_notes`/`post_notes`** points — no use case; YAGNI. Adding a point later is trivial under the config-table mechanism.

### Commit interplay (`pre_tag`)

After a `pre_tag` hook runs, **mint commits whatever it left dirty** (message `chore(release): pre-tag artifacts for {tag}`). Consequences:

- Simple hooks never touch git — they just build; mint handles the commit.
- "Commit only if the bundle changed" falls out for free: changed → tree dirty → mint commits; unchanged → tree clean → nothing committed.
- A hook that wants a *custom* commit can do its own and hand mint back a clean tree — mint then sees nothing to commit.

Either way, mint never tags a dirty tree, and hook authors aren't forced to know git.

### Failure behaviour (asymmetric across the point of no return)

- **`preflight` / `pre_tag`** run *before* the tag is pushed → a non-zero exit **aborts the whole release cleanly** (no tag, no damage; mint auto-unwinds any local mutations).
- **`post_release`** runs *after* the tag is live → it **cannot abort**; a non-zero exit just **warns** ("post_release hook failed; tag is already published"). Same principle as a failed `gh release create`.

### Invocation & context (injected env vars)

Each hook entry runs via `sh -c` from the repo root. mint injects:

| Variable | Example | Meaning |
|---|---|---|
| `MINT_NEW_VERSION` | `1.4.0` | the version being released |
| `MINT_PREVIOUS_VERSION` | `1.3.2` | the prior latest version |
| `MINT_VERSION_TAG` | `v1.4.0` | the full tag (with prefix) |
| `MINT_BUMP` | `patch` / `minor` / `major` / `explicit` | the bump kind; `explicit` when `--set-version` was used |
| `MINT_DRY_RUN` | `0`/`1` | dry-run flag |

The set may grow as later stages need it.

### Dry-run behaviour

Under `--dry-run`, mint **skips hooks** (they have side effects) and reports that they were skipped. (Confirmed in the dry-run semantics section.)

---

## Stage 4 — AI Release Notes

Generate a release-notes body from the diff since the last release. The same body is reused for every output surface (tag annotation, CHANGELOG, provider release) — generate once, use everywhere.

### Source of truth: the diff alone

mint generates notes from the **release diff and nothing else**. Commit messages / history are deliberately **not** ingested in any form:

- They are the **path, not the destination** — a commit may add code a later commit removes; the final diff correctly shows neither.
- They are **unreliable and entirely user-controlled** — mint may not have authored them (`mint commit` adoption is optional), so they may be hand-written or bare `WIP`. There is no floor on commit-message quality to build on.
- The conditional machinery to exploit them **isn't worth it** — the signal is unreliable and shrinks further as squash/rebase collapse history.

The diff is the one signal always true regardless of merge strategy or commit discipline, so it is the sole input. Quality is lifted by making the diff **more legible** (the Change Map, below) — never by adding other data.

### Engine layering (context assembly vs AI transport)

Notes generation separates two concerns, mirroring the engine's other seams (`CommandRunner`, `Publisher`, `Presenter`):

- **Context assembly (git-aware):** builds what the AI describes — the `last_tag..HEAD` diff, `diff_exclude` filtering, the `max_diff_lines` guard, and the Change Map preamble. All git-aware, release-specific logic lives here.
- **AI transport (content-agnostic):** takes an assembled prompt + content + `ai_command`, runs the call, validates (non-empty / not-an-error / not-a-refusal), retries once, and returns the body. It knows nothing about git, diffs, or tags — pure "content in, message out."

The boundary keeps the transport trivially testable (a string + a fake `ai_command`) and means quality work (enriching the Change Map / context) changes only the assembly side, never the transport. This is the same shared AI engine the sibling `mint commit` verb consumes — with a different context source (its staged diff) and prompt.

### Diff base

- Diff **`last_tag..HEAD`** (changes since the last release).
- **First release (no prior tag):** there's no base to diff and diffing the whole repo is useless to an AI → mint **skips the AI and uses a fixed body, "Initial release."**
- **Computed at the post-hook HEAD:** because `pre_tag` hooks (Stage 3) commit before notes generate (Stage 4), HEAD already includes the hook-artifact commit. This is intended — `diff_exclude` filters hook artifacts (e.g. the bundle) out *by path* regardless of being freshly committed, so the AI never sees bundle churn. Anything a hook commits that *isn't* excluded legitimately appears in the notes.

### Engine

- **Default `claude -p`.** mint composes the prompt, pipes it to the command's stdin, and reads the body from stdout, with a **timeout (~60s)** so a hung call can't stall a release.
- **Command overridable** via `ai_command` (default `claude -p`). mint always *owns the prompt*; the command is just transport — cheap future-proofing (swap binary/model) that keeps prompt-control working.

### Diff exclusion (two tiers + strategy-aware version file)

The diff sent to the AI is filtered via git's `:(exclude)` pathspec (git does the filtering):

- **Built-in always-exclude — `CHANGELOG.md`** (non-configurable). Pure mint output, never meaningful source. Excluded in both forward and regenerate paths.
- **`version_file` — NOT blanket-excluded (strategy-aware):**
  - *Forward path:* nothing to exclude — notes generate (Stage 4) *before* the version write (Stage 5), so the file is inherently unchanged at notes time. (The whole concern is therefore **regenerate-only**.)
  - *Regenerate, plain mode* (whole file is the version, e.g. `release.txt`): **exclude** the file — pure bookkeeping.
  - *Regenerate, embedded mode* (`version_pattern` in a real source file like `main.go`): **do not exclude** — it's source we want in notes. The lone version-line bump is negligible and neutralised by the default prompt's "ignore version-number bumps" instruction, not by hiding real code.
- **`diff_exclude` (project artifacts) — configurable array of globs**, on top of the above (knowledge bundle, minified output, lockfiles, generated code). These are *tracked, committed* generated files (deliberately not in `.gitignore`), which is why explicit exclusion is needed. A release diff is commit-to-commit so it can only contain tracked files; gitignored files never appear. A file that is gitignored yet *force-added* is nonetheless tracked, so it can still appear in the diff — this edge is deliberate and not special-cased. Kept in config (not a `.mintignore` file) per the "one config, one place to look" principle; `.mintignore` is YAGNI, addable later if exclude sets grow large.

### `max_diff_lines` guard

- **Default 50000.** Not a context limit but a **cost + quality** guard — a huge diff is slow, costly, and summarises to mush. Lines are a cheap token proxy (~10–20 tokens/line). **Excluded paths don't count toward it.** Exceeding it = a notes failure → abort-or-fallback per `on_notes_failure`. Fully overridable.

### Degenerate release (empty / all-excluded diff)

The mirror image of the `max_diff_lines` ceiling — a guard at the *small* end. If the post-`diff_exclude` diff is **empty or whitespace-only** (a re-tag with no source change; a release where every changed file fell under `diff_exclude`; or pure churn with nothing notable), mint **does not call the AI** — an empty diff is the one input it will reliably hallucinate on. It writes a minimal, honest entry: the version header + a short stub line (e.g. *"Maintenance release — no notable source changes"*). No hallucination, no hard error, no skipped entry — a no-op release still produces a truthful record.

One coherent family of "don't run the AI on a bad-sized diff" guards: too-big → fallback/abort per `on_notes_failure`; too-small/empty → stub, no AI.

### Change Map (diff-derived salience preamble)

The motivating failure — "glosses over the big feature on big releases" — is a **salience** problem (misallocated attention), not a missing-data problem; feeding *more* raw diff makes it worse. The fix is a computed **Change Map** that mint assembles (cheap git commands) and **prepends to the AI input**, telling the AI what to prioritize.

- **Structural novelty (primary signal):** new / removed / renamed paths — *especially new directories or packages appearing*. "A whole new `auth/` package showed up" is the strongest language-agnostic headline signal there is, and is qualitatively different from churn — a new subsystem is a headline even at modest line count, whereas a large refactor of existing code may not be. Weighted **above** raw magnitude, in both ordering and how the prompt is told to read it.
- **Magnitude (secondary signal):** per-area churn ranking, as supporting context ("400 lines here, 3 there").
- **Granularity — directory/area rollup by default**, with individually-notable files called out (new top-level entries, the single largest file). A flat list of every changed file is itself mush on big releases — the exact case this targets — so rollup is the salience-preserving form.
- **Computed after `diff_exclude`** (the map runs *after* exclusion, never before). Bulk noise is already removed, so post-exclude magnitude is largely trustworthy.
- **Prompt discipline (carries the diff-always-wins rule):** the prompt says **rank** importance using the Change Map but **describe** changes from the diff. The map is salience *metadata*, not content — the AI must never narrate a file as a feature merely because it's large or new.

**The AI input is therefore: the Change Map preamble, then the post-`diff_exclude` (and `max_diff_lines`-capped) diff — nothing else.**

### Big-diff handling — deferred (documented escalation)

Ship **single-pass**: the whole (`max_diff_lines`-capped) diff + Change Map, which already injects salience within one pass — the cheap win. **Hierarchical summarisation** (per-area summarise-then-synthesize) is **parked, not built for v1** — the documented escalation if real big-release output still reads as mush. An intermediate lever (Change Map + a *trimmed* diff rather than falling back at the cap) is noted for the same future. Revisit only on observed need.

### Failure behaviour — fail loud by default

Notes generate at Stage 4, *before* the tag (Stage 6), so aborting leaves nothing tagged/pushed — which is *why* blocking is safe.

- **`on_notes_failure`, default `abort`** — if the AI can't produce a body (missing tool, timeout, error, diff exceeds `max_diff_lines`, or a bad/empty generation that survives one retry), mint **fails loudly and tags nothing**. An empty/garbage release is worse than a failed command.
- **`fallback` mode (opt-in)** — proceed with a non-AI body instead of aborting. Fallback body defaults to the commit-subject list since the last tag; can be a fixed configurable string.
- **`--no-ai`** is a *deliberate* skip, not a failure → always uses the fallback body, never aborts.

### Output format & validation

- **The AI returns the notes directly in presentation format** — no machine-parseable wrapper labels. mint uses the body **whole** for every sink; no parsing, no splitting, no per-sink reassembly.
- **Validation is sanity, not structure:** non-empty, not an error/refusal/whitespace. On a bad/empty generation → **one automatic retry** → still bad → notes failure → `on_notes_failure`. A **timeout is not retried** — it goes straight to `on_notes_failure` (retrying a hung call only risks a second full timeout); the single retry covers empty/error/refusal *content* only, bounding worst-case latency at one ~60s timeout.
- The interactive review gate (next section) is the human backstop for *style*.

### Default notes format mint ships (anchored on Keep a Changelog)

The format anchors on the **Keep a Changelog** convention — its category taxonomy and principles (*"a changelog is for humans, not machines"*; *"a changelog is not a commit log"*) — rendered in **mint's emoji skin**. "Their meaning, mint's skin." This **refines** the emoji-headed-sections decision; it does not override it (it pins a fixed taxonomy *behind* the emoji).

- A **TL;DR one-liner** at the top (may be multi-line) — what the release is really about. This is mint's value-add over a raw changelog: a unified **cross-change narrative** synthesized from the *whole* diff (what beats regex/one-line-per-commit tools, which structurally can't see the whole release). Retained, sitting above the categorized sections.
- **Emoji-headed sections keyed to the Keep a Changelog taxonomy** — the canonical bucket set is `Added / Changed / Deprecated / Removed / Fixed / Security`, rendered with emoji headers (e.g. `✨ Added`, `🔧 Changed`, `🐛 Fixed`, `🗑️ Removed`). A *fixed, standard* taxonomy forces the AI to classify every change, and classification is itself prioritization — which helps the salience problem. Empty sections are omitted entirely.
- **Unit of entry = the notable item**, not the file / hunk / commit. The AI reads the whole diff, extracts notable items, and files each under its category. A change that adds a feature *and* fixes a bug yields two items in two sections — multi-category coverage falls out naturally.
- **Notable features bolded + described** (celebrated, not buried in a flat list).
- **Diff-inferability tiers the categories.** `Added / Changed / Fixed / Removed` are readable from a diff. `Deprecated` and `Security` are intent-laden and often invisible in a raw diff → kept in the vocabulary but treated as **opportunistic**: emit only on an *explicit textual marker* (a `@deprecated`/deprecation annotation; an obvious security surface — auth, crypto, input validation, a CVE-referencing dependency bump). Light prompt guidance, not detection machinery; expected to stay empty most releases. The deliberate escape hatch for diff-invisible intent is the **`[release].context` inject knob** — the user tells mint rather than mint guessing.
- **Strict "no preamble, no meta-commentary"** rule so prompt artifacts can never leak.
- Default prompt instructs the AI to **ignore version-number bumps** and other trivial bookkeeping churn.

The same body flows to every sink; in the changelog it sits under the `## [x.y.z] - date` header. *(Source confidence: the taxonomy/principles are firm; the exact emoji↔category mapping and prompt wording are explicitly ship-and-refine.)*

### Prompt control — two knobs (no third "themes" concept)

1. **`[release].context`** (string or file) — *injects* project-specific guidance into mint's default prompt (e.g. "dev-workflow toolkit; emphasise user-facing changes"). The common case.
2. **`[release].prompt`** (file path) — *full override* of the prompt; mint still supplies the diff. Total control.

A "theme/variant" is not a separate feature — it's just a `[release].prompt` override file. `mint init` can scaffold an example prompt. No built-in theme enum (YAGNI).

### Success criterion & verification

The quality bar for the notes is: **on a big release, the headline feature leads the notes.** The mechanism aimed at it is the Change Map (new-structure-as-headline, novelty over magnitude) plus forced Keep a Changelog classification. There is **no formal rubric or harness** — the user assesses output quality by eye on real releases (mint dogfoods itself; agentic-workflows, Portal, etc. are live cases) and tunes the prompt when the headline-leads criterion isn't met. Consistent with the "best effort, tune over time" posture.

---

## Body Distribution: Tag vs Changelog vs Provider Release

The single notes body feeds three surfaces. mint **writes** all three but **reads** only one — the tag annotation.

### What each surface carries

- **Tag annotation = subject `{commit_prefix} Release {tag}` + the FULL notes body** (default `commit_prefix` is 🌿). **Annotated** (not lightweight): signable, offline, in-repo, **immutable**. This is the **single source mint ever reads** — `regenerate --reuse` reads the annotation body via one deterministic git call (`git for-each-ref … contents:body`), no parsing.
- **CHANGELOG.md = a write-only projection** of the full body, under the `## [x.y.z] - date` header. mint *writes* it but **never reads** it.
- **Provider release (GitHub today) = a write-only projection** of the full body.

### Optionality stack

| Surface | Optional? | Control |
|---|---|---|
| **Annotated tag** | **Mandatory** | always created, always carries a body — the floor and source of truth |
| **Provider release** | Optional | `publish` (default `true`) |
| **CHANGELOG.md** | Optional | `changelog` (default `true`) |
| **AI notes** | Optional | `--no-ai` / no AI → tag body falls back to a commit-subject / changed-files list, so the tag is never empty |

With `changelog = false` nothing durable is lost — the tag still holds the full notes.

### Source-of-truth model

- The **tag is the immutable record of what shipped**. CHANGELOG + provider release are **mutable projections**.
- `regenerate --fresh` rewrites the **mutable** surfaces only; the tag is **never** rewritten (immutable history).
- `regenerate --reuse` always sources from the tag — deterministic, parse-free, config-independent.

**Trade accepted:** the full notes are duplicated in the tag *and* the changelog when both exist. Worth it for changelog-optionality, an always-present offline record, and parse-free healing.

### Design history (why the body is whole, not split)

An earlier design had the tag carry a Summary/TL;DR only (full body deemed redundant given a CHANGELOG) and had the AI emit machine-parseable `## Summary` / `## Notes` labels so mint could split the TL;DR out. Once the CHANGELOG became optional, the tag had to become the single source of truth carrying the full body — so nothing splits anymore and the machine labels became vestigial and were removed. The current design: AI returns presentation-format notes, mint uses the body whole for every sink.

---

## Stage 5 — Record (Changelog & Version Recording)

Persist the release into the repo: the CHANGELOG entry and the optional version-file projection, then build the commit graph leading to the tag.

### Changelog mechanics

mint **owns** CHANGELOG generation (Keep a Changelog format). mint is a **generator, not a human-maintained changelog**, so it follows Keep a Changelog's *entry structure* but not its human-workflow conventions:

- **A complete `## [x.y.z] - YYYY-MM-DD` section** (the full notes body), written atomically at release time, inserted **above the most recent existing `## [` block**.
- **No `[Unreleased]` section.** KaC's `[Unreleased]` exists for humans accruing notes between releases; mint writes the finished version section directly, so it doesn't apply.
- **Newest on top** — the new version section is prepended **below the file's Keep a Changelog header preamble, above all prior version sections**.
- **First release** — if `CHANGELOG.md` doesn't exist, mint creates it with the standard Keep a Changelog header preamble first, then the first version section.
- **Idempotent by version key** — a re-run or amended release for an *existing* version **replaces that version's section in place** rather than appending a duplicate.
- **Skipped entirely when `changelog = false`.**

### Version-file projection

When `version_file` is configured, mint writes the new version into it (per `version_pattern`, or the whole file in plain mode). See Stage 1 for the strategy mapping.

- **`version_pattern` mismatch** (configured pattern matches nothing in the file) → **abort during Record, before the tag** (fail-loud, same family as a notes failure). Never silently skip the version write.

### Commit graph (up to two commits, then tag)

1. **Hook artifacts** (only if a `pre_tag` hook dirtied the tree) → their **own** commit: `chore(release): pre-tag artifacts for {tag}`. Kept separate because it's *project content* (e.g. a rebuilt knowledge bundle), semantically distinct from release bookkeeping.
2. **Release bookkeeping** — the CHANGELOG entry **and** the version-file projection folded into **one** commit: `{commit_prefix} Release {tag}` (subject uses the configurable `commit_prefix`, default 🌿). (The legacy script made three commits per release — needlessly noisy.)
3. **Annotated tag** points at the release-bookkeeping commit.
4. **`git push --atomic`** sends both commits + tag together — the single point of no return.

### No-op safety

No empty commits — if the changelog yields no net change, or the version file already holds the target version, mint skips that commit.

---

## Stages 6–7 — Tag, Push & Publish

### Lock-resilient git

mint wraps **all** its git mutations in lock resilience (retry on a contended `.git` lock; clear a provably-stale lock). This carries forward the legacy `git_safe` behaviour as a built-in — tested once in Go, applied everywhere. A background agent/editor holding the index lock won't blow up a release.

### Point of no return

`git push --atomic origin HEAD {tag}` is the **single point of no return** — commits + tag go up together or not at all.

### Failure model

| When it fails | What mint does |
|---|---|
| **Before the push** (hook, notes, changelog, version, tag creation) | Everything mint did is local-only. mint **auto-unwinds its own mutations** — deletes the tag it made, resets the release commit(s) — returning the repo to the exact clean starting state. mint knows precisely what it created (N commits + 1 tag), so the unwind is surgical, and it reports what it undid. Next run starts clean. **Not configurable (YAGNI).** |
| **Push succeeds but provider release create fails** (e.g. transient network) | The tag is already public, so mint **never unwinds** (that would be destructive history rewriting). mint **warns** and points to the heal path: `regenerate --reuse` recreates the provider release from the tag annotation body (deterministic, parse-free). |
| **`post_release` hook fails** | **Warn only** — after the point of no return, the tag is already published. |

The auto-unwind is the same path taken when the user answers **no** at the `Continue?` review gate (see Interactive Review) — it includes any `pre_tag` hook-artifact commit. One mental model: *nothing mint did this run survives unless the release completes.*

### Publishing: provider driver abstraction

Publishing the release is **first-class but provider-abstracted** — not hardcoded to `gh`, and **not a `post_release` hook**.

- **Not a hook** — a hook would reintroduce the copy-paste disease mint cures (every repo re-deriving `gh release create --notes … --verify-tag`) and would break heal/regenerate (the reuse path recreates the provider release, so mint must own it).
- **Behind a small `Publisher` interface** (`CreateRelease` / `UpdateRelease`). mint **auto-detects the provider from the remote host** (`github.com` → GitHub driver via `gh`), overridable by the `provider` config.
- **GitHub is the only driver implemented now.** The seam means GitLab (`glab`), Gitea, etc. can drop in later with zero rework — extra drivers are YAGNI; the *interface* is the cheap future-proofing.
- Config is provider-neutral: **`publish`** (default `true`; `false` = tag + push only) plus optional **`provider`** override. An unknown/unsupported `provider` *value* (a recognised key, e.g. `provider = "gitlab"` when only GitHub is implemented) is **not** a fail-loud config error — mint **warns loudly and downgrades to tag + push only** (publish skipped), so a typo can't silently vanish. Fail-loud config validation still applies to unknown *keys* and bad *types*.
- The interface shape and auto-detection mechanics are routine Go, left to implementation.

### Post-release: tap / formula update

The brew formula's version/sha bump is **downstream CI** reacting to the GitHub release mint creates — **not mint's job**. Most repos mint releases aren't formulas anyway. If a project ever wants mint to actively trigger it (`repository_dispatch`), that's a **`post_release` hook** — already supported by the hook system, no engine code.

---

## Interactive Confirmation & Notes Review

The biggest live pain with the legacy script is that release notes go out *unseen* — no chance to review or edit before they're public. Notes are generated at Stage 4 (before any mutation / the point of no return), so there is a natural zero-risk window to review them.

### Default interactive flow (before any mutation)

```
1. Plan summary + computed version  → shown
2. Notes generated + validated      → shown in full
3. Gate (Continue?):  [y] accept (default — Enter)   [n] abort   [e] edit   [r] regenerate with context
```

- **`y` accept** (default; a bare Enter accepts) → proceed to Record → tag → push.
- **`n` abort** → **full auto-unwind**: identical to the pre-push failure path — mint rolls back everything it made this run, including any `pre_tag` hook-artifact commit, returning to the exact clean starting state. The hook re-runs next time (idempotent build). A user-abort and a pre-push git failure are treated identically.
- **`e` edit** → opens the notes in `$EDITOR` for real manual editing. **The saved text is used verbatim — no re-parse, no validation.** A human edit is trusted; structural validation only ever applied to untrusted AI output (which has no machine labels anyway). No mangle-loop, no possible trap.
- **`r` regenerate with context** → mint asks for a one-time context line, appends it to the prompt, re-runs the AI, and shows the result again (loops until happy). The "nudge it just this once" affordance — without permanently editing `[release].context`.

**Exact gate rendering** (the default-yes `Continue?` prompt, menu layout, and line-read input handling) is owned by the **CLI Presentation specification** (a cross-spec dependency); this section owns the four semantic choices and their effects.

### Non-interactive

- **`-y` / `--yes`** skips the whole gate (uses notes as generated) for scripted/CI use.
- A config toggle to disable the gate can be added later if it ever annoys (YAGNI now).

This eliminates the "notes went out unseen / I had to fix the release afterward" pain entirely.

---

## Dry-Run (`-d` / `--dry-run`)

### Semantics

- **Does:** read-only preflight, compute the version, **generate the AI notes preview** (Change Map + diff → AI), and print the full plan — the commits it would make, the tag, and what it would publish.
- **Skips:** all mutations (commit / tag / push / provider release) **and all hooks** (they have side effects) — and **reports** that hooks were skipped.

### Dry-run note caching

When `--dry-run` generates the notes preview, mint **caches it so the subsequent real run reuses it** — guaranteeing *what was previewed is what ships*. (Motivating workflow: an agent runs `mint release -y --dry-run` to surface the notes to a human, then `mint release -y` for real.)

- **The win is determinism, not cost.** AI generation is stochastic; regenerating on the real run risks shipping notes that differ from the preview. Reuse removes that risk. Skipping the second AI call is a bonus.
- **Activation is automatic.** The dry-run writes the note to the cache; the real run reuses it on a key match. No flag — the key-based invalidation makes automatic reuse safe, and it serves the motivating workflow transparently.
- **Cache key = hash of (post-`diff_exclude` diff + computed version + prompt / `[release].context`)** — not HEAD sha, since a `pre_tag` hook can change the tree between runs. **Miss → regenerate**, and say so ("diff changed since dry-run preview — regenerating notes"). mint never silently ships a stale note that no longer matches the release.
- **Re-review is unaffected.** A cached note does **not** skip the notes-review gate: an interactive real run still shows it (re-showing identical notes is cheap, and avoids assuming an out-of-band approval mint can't verify); `-y` still skips the gate on both runs. Reuse guarantees determinism; the review gate stays orthogonal.
- **Location:** a **gitignored cache** (e.g. `.mint/cache/`) or system temp, keyed by repo, **never committed**, with a **short TTL** backstop (default **~1 hour** — long enough for the dry-run→real-run handoff, short enough that a forgotten preview can't resurrect) so a stale preview can't be reused.

---

## Regenerate / Backfill Notes (non-destructive)

mint can regenerate release notes for *existing* releases — one release, or every release in a batch backfill ("rewrite all of a repo's release history"). It is **non-destructive**: it touches only the mutable surfaces and never rewrites tags.

### What's mutable vs permanent

- **Mutable (regenerate targets these):** the **provider release body** (GitHub today) and **`CHANGELOG.md`** — editable documents with no history consequence. mint re-diffs `vX-1..vX`, regenerates notes, and updates either or both, for one release or all. This is how you cleanly "rewrite release history": regenerate every version's provider release + rebuild `CHANGELOG.md`, touching no tags. ~95% of the visible value.
- **Permanent (excluded by default):** **tag annotations are git history.** "Rewriting" a tag means delete + re-create + force-push — destructive, and it breaks anyone who pulled. If ever built it would be a loud, explicit, opt-in-only escape (`--rewrite-tags`), strongly discouraged. **Not in scope now.**

### Two-axis contract (source × target)

Regenerate has two independent axes plus scope, all leaving tags untouched:

**Axis 1 — source of notes:**
- **`--reuse`** — read the **tag annotation body** (the single source of truth; deterministic git read, no parsing, config-independent). No AI, no re-diff — can't drift.
- **fresh** (default) — re-diff `vX-1..vX` (with the same `diff_exclude` tiers + Change Map as the forward path) and re-run the AI for genuinely better notes.

**Axis 2 — target surface(s): `--target release | changelog | both`.**
- **`--reuse` ⇒ release-only.** Its source *is* the notes record, so "reuse → write changelog" would write a file from itself (a no-op) → mint **errors on `--reuse --target changelog`**.
- **fresh ⇒ release, changelog, or both.**
- **`--target changelog`/`both` when `changelog = false` → error** (fail-loud: "changelog is disabled in config"). mint never silently creates a CHANGELOG the project opted out of.
- **Canonical spelling is `--target <surface>`** (one flag, a value) — not separate `--release`/`--changelog` flags.

**Composition table:**

| Goal | Source | Target |
|---|---|---|
| Heal a failed publish | reuse | release |
| Clean up a legacy/bad release | fresh | both |
| Refresh public release text only | fresh | release |
| Rebuild a changelog entry only | fresh | changelog |
| Mass-heal missing provider releases | reuse `--all` | release |
| Full history rewrite | fresh `--all` | both |

### Version argument & diff base resolution

- **`<version>` argument** may be given **with or without `tag_prefix`** (`regenerate v1.4.0` ≡ `regenerate 1.4.0`); mint normalises it. A `<version>` with **no matching tag** → **fail loud** ("no tag `vX.Y.Z` found").
- **Fresh diff base = `vX-1..vX`** (previous tag → target tag). For the **oldest release** (no predecessor tag — a single regenerate of the first release, or the first version in an `--all` backfill), there is no `vX-1`, so mint mirrors the forward path's first-release rule: **no AI, fixed body "Initial release."**

### Interactive by default, flags to skip

- `mint release regenerate <ver>` with **no flags → interactive**: asks source, asks target, shows the plan, confirms.
- **fresh** regeneration runs the same **notes-review gate** (see Interactive Review) before writing — backfilled notes are reviewable before they overwrite live surfaces.
- **reuse** is deterministic (no new notes) → a **simple confirm**, no review gate.
- Flags skip the questions but still confirm unless `-y`.

### Batch `--all` semantics

- **Ordering: oldest → newest** (lets mint rebuild `CHANGELOG.md` in natural order).
- **Partial failure: skip-and-continue, summarise at the end** — *not* abort-the-batch (a single huge release tripping `max_diff_lines` shouldn't kill the others). This consciously overrides the single-version `on_notes_failure = abort` default; mint reports e.g. *"27 regenerated, 3 skipped: vX (diff too large), …"* so the user re-runs the stragglers.
- **Review gates per version by default** (consistent with "notes never go out unseen"); **`-y`** is the opt-out to run fully unattended.
- **Re-runnable, no resume state.** `--reuse --all` (mass-heal from tags) is fully deterministic; `--fresh --all` re-generates (stochastic but harmless).

### Preflight subset per verb

Preflight is a *gate set*; each command runs only the relevant subset (general rule: *calls `gh` → gh-auth; commits + pushes → clean-tree / branch / remote-sync; cuts a new tag → tag-free*):

- **`regenerate --reuse`** (release-only, no git mutation) → **gh-auth only** (it must run that — a dead `gh` auth is the usual reason you're healing).
- **`regenerate` fresh → changelog / both** (commits + pushes) → **gh-auth + clean-tree + branch + remote-sync**; **not** tag-free (tags exist, untouched); no version compute.

---

## CLI Surface & Flags

### Commands

```
mint release [bump] [options]                  cut a release   (shim `release` → `mint release`)
mint release regenerate <version> [flags]      regenerate notes for an existing release
mint release regenerate --all [flags]          backfill every version (oldest → newest)
mint init                                      scaffold .mint.toml (+ release shim)
mint version                                   print mint's own version
```

- **Bare `mint release` is the cut action** (not `mint release cut`). `release` is a command with a default action *and* subcommands (well-trodden, e.g. `git stash` = `git stash push`). The per-project shim is `release`, so `./release -m` maps cleanly to `mint release -m`.
- **`regenerate` is a subcommand of `release`**, not a top-level `notes` verb — `notes` is the wrong noun and ages badly.
- `mint version` / `mint --version` print mint's own version (standard convention).

### `mint release` flags

```
-p, --patch          default when no bump flag is given
-m, --minor
-M, --major
    --set-version X.Y.Z   explicit version (mutually exclusive with bump flags)
-d, --dry-run
    --no-ai          deliberate skip → fallback body
    --autostash      stash/restore unrelated WIP around the run
    --any-branch     bypass the release-branch gate
-y, --yes            skip the confirmation + notes-review gate (scripted/CI use)
```

### `regenerate` flags

```
--reuse              source = tag annotation body (no AI); implies --target release
--fresh              source = re-diff + AI (default)
--target release|changelog|both    surface(s) to write (default: asked interactively)
--all                every version, oldest → newest; per-version gates unless -y
-y, --yes            skip confirmation + per-version review gate
```

No flags → fully interactive (asks source, asks target, shows plan, confirms). See Regenerate / Backfill Notes for semantics.

### Config Format & Schema

#### Format & location

- **Format: TOML.** Go-native, typed/validated with real error messages, supports comments, and `[table]`-style sections read cleanly. (YAML's indentation + type-coercion footguns aren't worth it.)
- **Location: `.mint.toml` at the repo root.** mint resolves the root via `git rev-parse --show-toplevel`, looks for `.mint.toml` there, and runs from root.
- **Fully optional.** Zero config = sensible defaults everywhere (tag-only release, `claude -p` notes, auto-derived release branch). `mint init` scaffolds a documented file.
- **Typed validation, fail-loud** on unknown keys / bad types, with clear messages.

#### Shape: shared engine keys + a table per verb

mint is multi-verb, so config is **verb-namespaced**: keys shared by every verb sit at the top level; each verb has its own table. This shape is adopted across the epic (it supersedes an earlier flat `notes_*` / `[hooks]` layout) — shared-vs-verb-specific becomes structural rather than inferred from prefixes, the verbs stay symmetric, and a future verb is a new table rather than more prefixes.

```toml
# Engine-level — shared by every verb
ai_command     = "claude -p"                      # default
diff_exclude   = ["skills/**/knowledge.cjs", "*.min.js"]
max_diff_lines = 50000

[release]
tag_prefix       = "v"             # default "v"
commit_prefix    = "🌿"            # default 🌿 — release commit + tag subject; cosmetic
release_branch   = "main"          # default: auto-derived from origin/HEAD
version_file     = "bin/tool"      # optional; omit = tag-only
version_pattern  = 'RELEASE_VERSION="{version}"'   # omit = whole file is the version
changelog        = true            # default true; false = no CHANGELOG.md projection
publish          = true            # default true; false = tag + push only
provider         = "github"        # optional; default auto-detected from remote host
on_notes_failure = "abort"         # abort | fallback
context          = "This is a dev-workflow toolkit; emphasise user-facing changes."  # inject into the notes prompt
prompt           = ".mint/notes-prompt.md"         # optional full prompt override

[release.hooks]
preflight    = "scripts/check.sh"
pre_tag      = ["npm ci", "npm run build"]
post_release = "scripts/notify.sh"
```

- **Shared engine keys (top level):** `ai_command`, `diff_exclude`, `max_diff_lines` — they serve every verb, so they sit above the tables.
- **`[release]` keys:** all release-specific config. `context` (the notes context-injection knob — formerly `notes_context`) and `prompt` (the full prompt override — formerly `notes_prompt`) are plain names; the table disambiguates them, so no `notes_` prefix is needed.
- **Hooks nest under their owning verb** as `[release.hooks]` (top-level is strictly shared-engine, so a top-level `[hooks]` would contradict that rule). Release owns `preflight` / `pre_tag` / `post_release`.
- **Other verbs add their own `[verb]` table** (e.g. a future `mint commit` gets `[commit]`), defined in that verb's own specification. This release spec specifies only the shared-engine keys and the `[release]` / `[release.hooks]` tables.

### Global / presentation flags

- **`--plain`** — a **global presentation flag** (applies to every verb): forces token-efficient plain output instead of styled output. Detection model (`--plain` else `isatty(stdout)`) and rendering are specified by the **CLI Presentation specification** (a cross-spec dependency); listed here so the full invocation surface is visible in one place.

---

## `mint init` Scaffolding

`mint init` activates mint in a project by dropping in **two files**:

1. **`.mint.toml`** — a **commented template**: the common keys with their defaults, plus optional keys (`version_file`, `[release.hooks]`, `[release].context` / `prompt`, …) present-but-commented with a one-line explanation each. The project tunes mint by uncommenting rather than reading docs.
2. **The `release` shim** — a tiny executable committed to the repo so `./release` works for anyone who clones. It execs `mint release "$@"`; if mint isn't installed it prints the install hint (`brew install leeovery/tools/mint`) and exits non-zero.

### Behaviour

- **Idempotent / non-clobbering** — an existing `.mint.toml` or `release` is **skipped with a notice**; **`--force`** regenerates.
- **No hook/prompt files scaffolded by default** — the commented config shows hook examples inline, and a `[release].prompt` override file is only *mentioned* in a comment (not created).
- **No project auto-detection** — mint does **not** sniff `package.json`/etc. to pre-fill a build hook; that guesswork can surprise. A clean commented template is more honest. Deferred, addable later.

---

## Dependencies

Prerequisites that must exist before implementation can begin:

### Partial Requirement

| Dependency | Why Blocked | Minimum Scope Needed |
|------------|-------------|----------------------|
| **CLI Presentation specification** | The release run emits semantic events to a `Presenter` and shows the interactive notes-review gate through it; this spec defers gate *rendering* and the `--plain` flag's behaviour to that spec. The engine cannot render output or prompt the gate without the Presenter contract. | The **`Presenter` interface** (the event/method surface the engine calls) and the **notes-review gate rendering contract** (the `Continue?` default-yes mapping of the four semantic choices). Concrete `pretty`/`plain` renderers and `--plain` detection can land in parallel — the engine builds and tests against the interface (a fake/recording presenter). |

### Notes

- **Everything else builds independently.** Version/tag logic, preflight gates, hooks, AI-notes context assembly + transport, the Change Map, record/changelog/version-file, tag/push/publish, regenerate, config parsing, and `mint init` have no cross-spec blocker — they sit behind the engine's own seams (`CommandRunner`, `Publisher`) and the shared config model, all owned by this spec.
- **`mint commit` is not a dependency** — it *consumes* the AI engine, config model, and Presenter that this spec (and the presentation spec) establish. Release is the producer, so it does not wait on commit.
- **Runtime tools** (`git`, `gh`, `claude`) are invoked behind the `CommandRunner` seam and are runtime prerequisites, not build/spec dependencies; the spec already defines graceful handling (preflight `gh` gate; optional/fallback `claude`).
- **Downstream tap/formula auto-update CI** reacts to the GitHub release mint creates — it is post-release, not a build dependency.

---

## Working Notes
