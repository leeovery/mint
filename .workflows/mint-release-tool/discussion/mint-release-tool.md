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

  Discussion Map — Mint Release Tool (21 subtopics — 21 decided)

  ┌─ ✓ Release lifecycle spine [decided]
  ├─ ✓ Version detection & bump [decided]
  ├─ ✓ Tag format, prefix & pre-releases [decided]
  ├─ ✓ Safety & preflight gates [decided]
  ├─ ✓ Hook mechanism [decided]
  ├─ ✓ Hook points (which stages) [decided]
  ├─ ✓ Hook contract & commit interplay [decided]
  ├─ ✓ AI release notes [decided]
  │  ├─ ✓ Diff-exclude globs ("mint ignore") [decided]
  │  └─ ✓ Prompt control: override & context injection [decided]
  ├─ ✓ Tool scope & command namespace (`mint <verb>`) [decided]
  ├─ ✓ Regenerate / backfill notes (non-destructive) [decided]
  ├─ ✓ Body distribution: tag vs changelog vs release [decided]
  ├─ ✓ Changelog & version recording [decided]
  ├─ ✓ Tag, push & publish [decided]
  │  ├─ ✓ Publishing: provider driver abstraction [decided]
  │  └─ ✓ Post-release: tap / formula update [decided]
  ├─ ✓ Config format & schema [decided]
  ├─ ✓ CLI surface & flags [decided]
  ├─ ✓ Interactive confirmation & notes review [decided]
  └─ ✓ `mint init` scaffolding [decided]

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

**Escape hatch:** `--set-version X.Y.Z` to set an explicit version (e.g. a deliberate 1.x → 2.0.0 jump). Preferred over a positional `mint 2.0.0` — the flag is unambiguous and self-documenting. (Named `--set-version`, not `--version`, to avoid the tool-version clash.)

**`--set-version` interaction & validation (review F6):**
- **Mutually exclusive with bump flags** — `--set-version` + `-p`/`-m`/`-M` → error ("can't combine `--set-version` with a bump flag"). No silent precedence. (`--set-version` alone = explicit; bump alone = computed; neither = default patch.)
- **Valid 3-part semver, and strictly greater than the current latest tag** — a backwards jump is rejected by default *even if the tag is free*, because a lower version sorts below "latest" and corrupts tag-as-truth. Sits on top of the free-tag preflight (which catches an equal/existing tag).
- **No downgrade override now** (YAGNI) — add `--force` if a genuine "re-tag an old line" need ever appears. Forward-only today.

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

## Tag, push & publish

### Context

Stages 6–7: create the tag, push, publish the GitHub release. This is where the point of no return sits and where failure handling matters most (review findings F4, F11, F13).

### Decision — lock-resilient git (F13)

mint wraps **all** its git mutations in lock resilience (retry on a contended `.git` lock, clear a provably-stale lock) — the old `git_safe`, carried forward as built-in. Singled out in the handoff as fragile-but-important and a reason to choose Go (tested once, applies everywhere). A background agent/editor holding the index lock won't blow up a release.

### Decision — the point of no return & failure model (F4, F11)

`git push --atomic origin HEAD vX.Y.Z` is the **single point of no return** — commits + tag go up together or not at all.

- **Failure *before* the push** (hook, notes, changelog, version, tag creation) → everything mint did is local only. mint **auto-unwinds its own mutations** — deletes the tag it made and resets the release commit(s) — returning the repo to the exact clean state it started from. mint knows precisely what it created (N commits + 1 tag), so it's surgical; it reports what it undid. Next run starts clean. **Not configurable (YAGNI)** — there's no good reason to want a half-made local release left behind; if you want resilience against AI flakiness, that's what `on_notes_failure = fallback` is for. Auto-unwind only ever concerns the rarer git-side failure.
- **Push succeeds but `gh release create` fails** (e.g. transient network) → tag is already public, so mint **never unwinds** (that would be destructive history rewriting). mint **warns** and points to the heal path: the regenerate command's **reuse mode** recreates the GitHub release from the **tag annotation body** (deterministic, parse-free). (F11 solved — recovery is a first-class command, not a special case.)
- **`post_release` hook fails** → warn only (after the point of no return; already decided).

### Decision — publishing: provider driver abstraction

Publishing the release (the GitHub release today) is **first-class but provider-abstracted**, not hardcoded to `gh` and *not* a hook.

- **Not a `post_release` hook** — that would reintroduce the copy-paste disease mint cures (every repo re-deriving `gh release create --notes … --verify-tag`) *and* break heal/regenerate (the reuse path recreates the provider release; mint must own it).
- **Behind a small `Publisher` interface** (`CreateRelease` / `UpdateRelease`). mint **auto-detects the provider from the remote host** (`github.com` → GitHub driver via `gh`), overridable by `provider` config.
- **GitHub is the only driver implemented now** (all that's needed); the seam means GitLab (`glab`), Gitea, etc. drop in later with zero rework. Extra drivers are YAGNI; the *interface* is the cheap future-proofing.
- Config is provider-neutral: **`publish`** (default true; false = tag + push only) + optional **`provider`** override. Unknown/unsupported provider → tag + push only.
- The interface shape and auto-detection mechanics are routine Go → spec/implementation, not further discussed here.

### Decision — post-release: tap / formula update

Resolved earlier and confirmed: the brew formula's version/sha bump is **downstream CI** reacting to the GitHub release, *not* mint's job. If a project wants mint to actively trigger it (`repository_dispatch`), that's a **`post_release` hook** — already supported by the hook system, no engine code. Child closed.

Confidence: high.

---

## Changelog & version recording (Record stage)

### Context

Stage 5: persist the release into the repo — the CHANGELOG entry and the optional version-file projection — and decide the commit graph relative to the tag (review finding F8).

### Decision — changelog mechanics

mint **owns** CHANGELOG generation (Keep a Changelog format):
- New entry `## [x.y.z] - YYYY-MM-DD` + the full body (Summary + Notes), inserted **above the most recent existing `## [` block**.
- If `CHANGELOG.md` doesn't exist, create it with the standard Keep a Changelog header first.

### Decision — commit graph (up to two commits, then tag)

1. **Hook artifacts** (if a `pre_tag` hook dirtied the tree) → their **own** commit: `chore(release): pre-tag artifacts for vX.Y.Z`. Kept separate because it's *project content* (e.g. the rebuilt knowledge bundle), semantically distinct from release bookkeeping.
2. **Release bookkeeping** — CHANGELOG entry **and** version-file projection folded into **one** commit: `🌿 Release vX.Y.Z` (subject uses the configurable `commit_prefix`, default 🌿 mint leaf). (Old script made three commits per release — needlessly noisy.)
3. **Annotated tag** points at the release commit.
4. **`git push --atomic`** sends both commits + tag together — the single point of no return; anything failing before it is local-only and recoverable.

### Edge cases

- **`version_pattern` mismatch** (configured pattern matches nothing in the file) → **abort during Record, before the tag** (fail-loud, same family as a notes failure). Never silently skip the version write.
- **No-op safety**: no empty commits — if the changelog yields no net change or the version file already holds the version, skip that commit.

Confidence: high.

---

## Body distribution: tag vs changelog vs release

### Context

The notes body feeds three surfaces. The user spotted real redundancy (all three carrying identical full text) and questioned whether the tag should differ. Also raised: how to split a TL;DR out *deterministically* without trusting the AI to obey positional rules, and the fact that a TL;DR may be multi-line.

### Decision — what each surface carries (REVISED after changelog-optional)

> **Reversal:** an earlier decision made the tag carry the **Summary/TL;DR only** (full body deemed redundant given CHANGELOG). That premise assumed a CHANGELOG always exists. Once the **CHANGELOG is optional** (see below), it collapses — so the tag now carries the **full notes body** and becomes the single source of truth.

- **Tag annotation = subject `🌿 Release vX.Y.Z` (configurable `commit_prefix`) + the FULL notes body** (Summary + Notes). Annotated (not lightweight): signable, offline, in-repo, **immutable**. This is the **single source mint ever reads** — `regenerate --reuse` reads the annotation body via one deterministic git call (`git for-each-ref … contents:body`), no parsing.
- **CHANGELOG.md (optional, `changelog = true` default) = a write-only projection** of the full body, under the `## [x.y.z] - date` header. mint *writes* it but **never reads** it. `changelog = false` → no changelog; nothing durable is lost (the tag has the full notes).
- **GitHub release = a write-only projection** of the full body.

**Optionality stack:** the **annotated tag is mandatory** (always created, always carries a body — the floor and source of truth). **Provider release is optional** (`publish`, default true). **CHANGELOG is optional** (`changelog`, default true). **AI notes are optional** — with `--no-ai`/no AI, the tag body falls back to a commit-subject / changed-files list, so the tag is never empty.

**Source-of-truth model:** the tag is the immutable record of *what shipped*. CHANGELOG + GitHub release are mutable projections. `regenerate --fresh` rewrites the **mutable** surfaces only; the tag is **never** rewritten (immutable history). Reuse always sources from the tag — deterministic, parse-free, and config-independent. Trade accepted: full notes duplicated in tag *and* changelog when both exist — worth it for changelog-optionality, an always-present offline record, and parse-free healing.

### Decision — AI output format & validation (machine-labels REMOVED)

> **Cascade simplification:** the original design had the AI emit machine-parseable `## Summary` / `## Notes` labels so mint could *split the TL;DR out for the tag*. Once the tag was revised to carry the **full body**, nothing splits anymore — the labels became vestigial. **Removed.**

- **The AI returns the notes directly in the presentation format** — a short TL;DR lead, then emoji-headed sections (`✨ Features` / `🐛 Fixes` / `🧹 Internal`, empty ones omitted). No machine-parse wrapper labels.
- **mint uses the body whole** for every sink (tag / changelog / GitHub release). No parsing, no splitting, no per-sink reassembly.
- **Validation is now sanity, not structure:** non-empty, not an error/refusal/whitespace. On a bad/empty generation → **one automatic retry** → still bad → **notes failure** → `on_notes_failure` (abort by default). The fail-loud guarantee survives; it just no longer hinges on label-parsing.
- The interactive review gate is the human backstop for *style* (sections missing, tone off) — a far better place to catch presentation issues than a rigid parser.

Confidence: high.

---

## CLI surface & flags

### Context

Consolidation — every command and flag was named across the discussion. Also resolves dry-run semantics (review F9).

### Decision — commands

```
mint release [bump] [options]            cut a release   (shim `release` → `mint release`)
mint release regenerate <version>        fresh regenerate (re-diff + AI), rewrites CHANGELOG + GitHub release
mint release regenerate <ver> --reuse    heal from existing CHANGELOG entry (no AI) — failed-publish recovery
mint release regenerate --all            backfill every version
mint init                                scaffold .mint.toml (+ shim)
mint version                             print mint's own version
```

- **Bare `mint release` is the cut action** (not `mint release cut`) — the shim is `release`, so `./release -m` must map cleanly to `mint release -m`. `release` is a command with a default action *and* subcommands (well-trodden, e.g. `git stash` = `git stash push`). No `cut` verb.
- **Regenerate is a subcommand of `release`**, not a top-level `notes` verb — `notes` is the wrong noun and ages badly once `mint commit` exists.

### Decision — `mint release` flags

```
-p, --patch            default
-m, --minor
-M, --major
    --set-version X.Y.Z   explicit version (renamed from --version to avoid the tool-version clash)
-d, --dry-run
    --no-ai            deliberate skip → fallback body
    --autostash        stash/restore unrelated WIP around the run
    --any-branch       bypass the release-branch gate
-y, --yes              skip the confirmation + notes-review gate (scripted/CI use)
```

`mint version` / `mint --version` print mint's own version (standard convention).

### Decision — dry-run semantics (F9)

- **Does:** read-only preflight, compute version, **generate the AI notes preview**, print the full plan (commits, tag, what would publish).
- **Skips:** all mutations (commit/tag/push/release) **and hooks** (side effects) — reports hooks were skipped.

Confidence: high.

---

## Interactive confirmation & notes review

### Context

The user's biggest live pain with the current script: release notes go out *unseen*, with no chance to review or edit before they're public. Since notes are generated at stage 4 (before any mutation / the point of no return), there's a natural zero-risk window to review them.

### Decision

Default interactive flow before any mutation:

```
1. Plan summary + computed version  → shown
2. Notes generated + validated      → shown in full
3. Gate:  [a] accept   [e] edit   [r] regenerate with context   [q] abort
```

- **`a` accept** → proceed to Record → tag → push.
- **`e` edit** → open the notes in `$EDITOR` for real manual editing; **saved text is used verbatim — no re-parse, no validation** (review F5). A human edit is trusted; structural validation only ever applied to untrusted AI output, which no longer has machine labels anyway. No mangle-loop, no possible trap.
- **`r` regenerate with context** → mint asks for a one-time context line, appends it to the prompt, re-runs the AI, shows the result again (loops until happy). The "nudge it just this once" affordance — without permanently editing `notes_context`.
- **`q` abort** → **full auto-unwind** (review F1): identical to the pre-push failure path — mint rolls back *everything it made this run*, including any `pre_tag` hook-artifact commit, returning to the exact clean starting state. One mental model: *nothing mint did this run survives unless the release completes.* The hook re-runs next time (idempotent build). A user-abort and a pre-push git failure are treated identically.

- **`-y/--yes`** skips the whole gate (uses notes as generated) for scripted/CI use.
- Config toggle to disable the gate can be added later if it ever annoys (YAGNI now).

This eliminates the "notes went out unseen / I had to fix the release afterward" pain entirely.

Confidence: high.

---

## `mint init` scaffolding

### Context

How mint "activates" in a project — the activation step from the handoff.

### Decision

`mint init` drops in **two files**:

1. **`.mint.toml`** — a **commented template**: common keys with defaults, optional keys (version_file, hooks, notes_context, …) present-but-commented with a one-line explanation each. Tune by uncommenting rather than reading docs.
2. **The `release` shim** — a tiny executable committed to the repo so `./release` works for anyone who clones; execs `mint release "$@"`, and if mint isn't installed prints `brew install leeovery/tools/mint` and exits non-zero.

**Behaviour:**
- **Idempotent / non-clobbering** — existing `.mint.toml` or `release` is skipped with a notice; `--force` regenerates.
- **No hook/prompt files scaffolded by default** — the commented config shows hook examples; a `notes_prompt` override file is only mentioned in a comment.
- **No project auto-detection** (e.g. sniffing `package.json` to pre-fill a build hook) — guesswork that can surprise; a clean commented template is more honest. Deferred, addable later.

Confidence: high.

---

## Config format & schema

### Context

Where mint's per-project configuration lives and in what format. Handoff assumed TOML; confirm vs YAML. Also resolves repo-root anchoring (review F12).

### Decision

- **Format: TOML.** Go-native, typed/validated with real error messages, supports comments, and `[hooks]`-style tables read cleanly. YAML's indentation + type-coercion footguns aren't worth it for a config file.
- **Location: `.mint.toml` at the repo root.** mint resolves the root via `git rev-parse --show-toplevel`, looks for `.mint.toml` there, runs from root (F12 — anchoring defined).
- **Fully optional.** Zero config = sensible defaults everywhere (tag-only release, `claude -p` notes, auto-derived release branch). `mint init` scaffolds a documented file.
- **Typed validation, fail-loud** on unknown keys / bad types with clear messages (a Go advantage over the old sourced-bash config that failed silently on a typo).

### The consolidated schema (every key named across the discussion)

```toml
# .mint.toml — all keys optional

tag_prefix      = "v"            # default "v"
commit_prefix   = "🌿"           # default 🌿 (mint leaf) — release commit + tag subject; cosmetic
release_branch  = "main"         # default: auto-derived from origin/HEAD
version_file    = "bin/tool"     # optional; omit = tag-only
version_pattern = 'RELEASE_VERSION="{version}"'   # omit = whole file is the version
changelog       = true           # default true; false = no CHANGELOG.md projection (tag holds full notes)
publish         = true           # default true; false = tag + push only (no provider release)
provider        = "github"       # optional; default auto-detected from the remote host

# AI release notes
ai_command       = "claude -p"   # default
on_notes_failure = "abort"       # abort | fallback
max_diff_lines   = 50000
diff_exclude     = ["skills/**/knowledge.cjs", "*.min.js"]
notes_context    = "This is a dev-workflow toolkit; emphasise user-facing changes."
notes_prompt     = ".mint/notes-prompt.md"   # optional full override

[hooks]
preflight    = "scripts/check.sh"
pre_tag      = ["npm ci", "npm run build"]
post_release = "scripts/notify.sh"
```

Confidence: high.

---

## Tool scope & command namespace (`mint <verb>`)

### Context

Mid-discussion the user proposed making mint multifaceted: `mint release` today, `mint commit` later (wrapping their existing AI-commit shell function — `--all`, `--no-ai`, context injection, auto-push). "Minting a commit" fits the brand. Raises a scope fork: does commit belong in *this* build, and is mint still a single feature?

### Decision

- **Adopt the `mint <verb>` namespace now.** The release command is `mint release`; the per-project shim `release` delegates to `mint release`. Cheap, forward-compatible — leaves room for `mint commit` and future verbs without restructuring later.
- **Defer `mint commit` to its own separate feature/discussion.** A full AI-commit command is a second tool's worth of design (staging logic, its own prompt, flags, push behaviour); folding it in would balloon this build. Out of scope here.
- **mint stays a single feature for now** (release-only). Commit arrives later as a *separate* feature. The namespace leaves the door open to promote mint to an **epic** (release + commit + … as siblings) if/when scope justifies it — discovery already flagged this as the likely trigger. Not promoting yet.

Confidence: high.

---

## Regenerate / backfill notes (non-destructive)

### Context

User wants to be able to regenerate release notes for *existing* releases — even "rewrite all of agentic-workflows' release history." Hinges on what's mutable vs permanent.

### Decision

- **Target only the mutable surfaces.** The **GitHub release body** and **`CHANGELOG.md`** are editable documents with no history consequence. mint can re-diff `vX-1..vX`, regenerate notes, and update both — for one release or all of them (batch backfill). This is how you cleanly "rewrite release history": regenerate every version's GitHub release + rebuild `CHANGELOG.md`, **touching no tags.** ~95% of the visible value.
- **Tag messages are git history — excluded by default.** "Rewriting" a tag means delete + re-create + force-push: destructive, breaks anyone who pulled. If ever built, it's a loud, explicit, opt-in-only escape (`--rewrite-tags`), strongly discouraged. Not in scope now.
- Command mechanics (which versions, invocation, flags) fold into the CLI surface / spec.

### Regenerate contract — two axes (refined: source × target; reviews F4/F7/F8)

Regenerate has **two independent axes** plus scope, all leaving tags untouched (immutable):

**Axis 1 — source of notes:**
- **`--reuse`** — read the **tag annotation body** (the single source of truth; deterministic git read, no parsing, config-independent). No AI, no re-diff. Can't drift.
- **fresh** (default) — re-diff `vX-1..vX` (with the diff-exclusion tiers from F3) + re-run the AI for genuinely better notes.

**Axis 2 — target surface(s):** `--target release | changelog | both`.
- `--reuse` ⇒ **release-only** (its source *is* the notes record; "reuse → write changelog" would write a file from itself — a no-op; mint errors on `--reuse --changelog`).
- fresh ⇒ release, changelog, or both.

**Composition table:**

| Goal | Source | Target |
|---|---|---|
| Heal a failed publish | reuse | release |
| Clean up a legacy/bad release | fresh | both |
| Refresh public release text only | fresh | release |
| Rebuild a changelog entry only | fresh | changelog |
| Mass-heal missing GH releases | reuse `--all` | release |
| Full history rewrite | fresh `--all` | both |

**Interactive by default, flags to skip (F7):**
- `mint release regenerate <ver>` with no flags → interactive: asks source, asks target, shows plan, confirms.
- **fresh** regeneration runs the same **notes-review gate** (`[a]/[e]/[r]/[q]`) before writing — backfilled notes are reviewable before they overwrite live surfaces (the whole point of the gate). **reuse** is deterministic (no new notes) → a simple confirm, no review gate.
- Flags skip the questions but still confirm unless `-y`.

**Batch `--all` semantics (F8):**
- **Ordering: oldest → newest** (lets mint rebuild `CHANGELOG.md` in natural order).
- **Partial failure: skip-and-continue, summarise at the end** — *not* abort-the-batch (a single huge release tripping `max_diff_lines` shouldn't kill 29 good ones). Consciously overrides the single-version `on_notes_failure=abort` default; mint reports `"27 regenerated, 3 skipped: vX (diff too large), …"` so the user re-runs the stragglers.
- **Review gates per version by default** (consistent with "notes never go out unseen"); **`-y/--yes`** is the existing opt-out to run fully unattended. No new flag — gate-by-default, opt out.
- **Re-runnable**, no resume state. `--reuse --all` (mass-heal from tags) is fully deterministic; `--fresh --all` re-generates (stochastic but harmless).

**Preflight subset per verb (F4):** preflight is a *gate set*; each command runs the relevant subset.
- `regenerate --reuse` (release-only, no git mutation) → **gh-auth only** (it *must* run that — a dead `gh` auth is the usual reason you're healing).
- `regenerate` fresh → changelog/both (commits + pushes) → **gh-auth + clean-tree + branch + remote-sync**; **not** tag-free (tags exist, untouched); no version compute.
- General rule: *calls `gh` → gh-auth; commits+pushes → clean-tree/branch/remote-sync; cuts a new tag → tag-free.*

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
- **Computed at the *post-hook* HEAD (review F2):** because `pre_tag` hooks (stage 3) commit before notes generate (stage 4), HEAD already includes the hook-artifact commit. That's intended — `diff_exclude` filters hook artifacts (e.g. the bundle) out *by path*, regardless of being freshly committed, so the AI never sees bundle churn. Consequence: anything a hook commits that *isn't* excluded legitimately appears in the notes (correct — you'd want it described). Ordering kept as-is rather than generating notes pre-hook, which would crudely hide *all* hook output including the meaningful kind.

**No `pre_notes` / `post_notes` hook points (YAGNI).** `pre_tag` already runs before notes, so a pre-notes hook is a redundant second name for the same slot. A post-notes hook has no real use case and would have to mutate the structured body without breaking it — the jobs it might do are covered better by `notes_context`/`notes_prompt` (steering) and the interactive edit/regenerate gate (human intervention). Trivially addable later if a concrete need appears.

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

### Diff exclusion tiers — decided (refined by review F3)

The AI diff has two tiers of exclusion, plus a strategy-aware nuance for the version file:

- **Built-in always-exclude — `CHANGELOG.md`** (non-configurable). It's pure mint output, never meaningful source. Excluded in *both* forward and regenerate paths.
- **`version_file` — NOT blanket-excluded** (strategy-aware):
  - *Forward path:* nothing to exclude — notes generate (stage 4) *before* the version write (stage 5), so the file is inherently unchanged at notes time. This is why the whole concern is **regenerate-only** (post-hoc, where the bookkeeping commits already exist in the `vX-1..vX` range).
  - *Regenerate, plain mode* (whole file is the version, e.g. `release.txt`): exclude the file — pure bookkeeping.
  - *Regenerate, embedded mode* (`version_pattern` in a real source file like `main.go`): **do not** exclude — the file is source we want in notes. The lone `RELEASE_VERSION="…"` line bump is negligible, neutralised by the default prompt's "**ignore version-number bumps**" instruction, not by hiding real code.
- **`diff_exclude` (project artifacts) — configurable, on top** of the above.

**Why regenerate needs this and forward doesn't:** the forward path diffs at a HEAD that predates mint's bookkeeping commits; regenerate diffs a tag range that already contains them. Excluding by path (CHANGELOG always; version_file in plain mode) reproduces the forward path's *source* view. (A 🌿/mint commit prefix is cosmetic only — diffs are range-based and can't subtract commits, so exclusion stays path-based.)

### Diff-exclude globs — decided

- **`diff_exclude`** — a config array of globs kept out of the diff sent to the AI (knowledge bundle, minified output, lockfiles, generated code). Implemented via git's `:(exclude)` pathspec — git does the filtering. Connection: the same artifact a `pre_tag` hook builds is what gets excluded here.
- **Config array, not a `.mintignore` file** — consistent with the "everything in one config file, one place to look" principle (same call as hooks). These are *tracked, committed* generated files, so they're deliberately not in `.gitignore`. A `.mintignore` only earns its place if exclude sets grow large/gitignore-like — YAGNI today, addable later.
- **No `.gitignore`-based exclusion needed (inherent).** A release diff is commit-to-commit (`last_tag..HEAD`), so it can *only* contain tracked/committed files — gitignored files are never committed and thus never appear. `diff_exclude` exists precisely for the opposite case: files that *are* committed but are still noise (the knowledge bundle is tracked, which is why it needs explicit exclusion). Edge case (gitignored-but-force-added) is deliberate and not special-cased.
- **`max_diff_lines`, default 50000.** Reframed: not a model-context limit (modern windows are ~1M tokens) but a **cost + quality** guard — a huge diff is slow, costly, and summarises to mush. Lines are a cheap proxy for tokens (~10–20 tokens/line). Excluded paths don't count toward it. Exceeding it = a notes failure → abort-or-fallback per `on_notes_failure`. Fully overridable.

### Prompt control & default format — decided

Grounded in the *actual* current output (agentic-workflows CHANGELOG + tag messages). Problems observed: flat intertwined list (features, fixes, tests, logged ideas all equal-weight); prompt leakage ("Based on the diff:" preamble bled into v0.4.17/0.4.18); empty descriptions on oversized releases (the fail-loud default fixes this).

**Default format mint ships:**
- A **TL;DR** one-liner at the top — what the release is really about.
- **Emoji-headed sections** — e.g. `✨ Features`, `🐛 Fixes`, `🧹 Internal`. Empty sections omitted; AI may add a sensible section if warranted.
- Notable features **bolded + described** (celebrated, not buried in a flat list).
- Strict **"no preamble, no meta-commentary"** rule so prompt artifacts can never leak.
- Default prompt instructs the AI to **ignore version-number bumps** and other trivial bookkeeping churn (supports the embedded-version-file regenerate case — see diff exclusion tiers).
- Same body flows to all three sinks (tag message, CHANGELOG entry, GitHub release). In the changelog it sits under the `## [x.y.z] - date` header.

**Two-knob configurability (no third "themes" concept):**
1. **`notes_context`** (string or file) — *inject* project-specific guidance into mint's default prompt (e.g. "dev-workflow toolkit; emphasise user-facing changes"). The common case.
2. **`notes_prompt`** (file path) — *full override* of the prompt; mint still supplies the diff. Total control.

A "theme/variant" is **not** a separate feature — it's just a `notes_prompt` override file. `mint init` can scaffold an example prompt to start from. Avoids building/maintaining a built-in theme enum nobody asked for (YAGNI).

Confidence: high.

---

## Summary

### Key Insights

1. **Tag is the single source of truth.** Collapses the old `file`/`embedded`/`none` strategies into one rule; version files become write-only mirrors. Brew installs from tags, so the tag *is* the version.
2. **Shrink hooks before serving them.** Anything universal-but-optional (version-file writing, diff-excludes) is absorbed into mint as *tested Go config*; hooks are reserved for genuinely bespoke project steps. The recurring "is this core or a hook?" test: if mint already owns the data/concern, it's core.
3. **Fail-loud, before the point of no return.** Notes/changelog/version all happen pre-push; failures abort cleanly with nothing tagged. mint auto-unwinds its own local mutations on pre-push failure; it never rewrites pushed history. The single point of no return is `git push --atomic`.
4. **Robustness lives in mint, not in trusting the model.** Sanity-validate AI output (non-empty/not-an-error) + one retry + fail-loud means a bad generation can never silently produce a garbage release; the interactive gate is the human backstop for style. (An earlier `## Summary`/`## Notes` machine-label contract was removed once the tag carried the full body — nothing splits, so the labels were vestigial.)
5. **The annotated tag is the floor and the source of truth.** It's the one mandatory, immutable artifact and the only thing mint ever *reads* (full notes body in the annotation). CHANGELOG + GitHub release are optional, write-only projections. `regenerate --reuse` heals from the tag deterministically (parse-free); even with no AI the tag still carries a commit/file-list fallback body.
6. **One hook mechanism:** a config table of shell command strings keyed by lifecycle point — scripts are just something a string can call.

### Open Threads (deferred / out of scope — consciously)

- **`mint commit`** — deferred to its own separate feature; the `mint <verb>` namespace leaves room. mint may promote to an **epic** (release + commit + …) when scope justifies.
- **Testing / parity strategy** — deferred to spec/planning/implementation. The old bash script is reframed from *test oracle* to *feature reference* (capability checklist, not byte-parity target), since the clean-slate design intentionally diverges. Idiomatic Go + Go skills pulled in at implementation time.
- **YAGNI / addable-later:** pre-release/RC tags (parse + produce), `--rewrite-tags`, inline-vs-script hook duplication, built-in note "themes" (use `notes_prompt` override), project auto-detection in `mint init`, a dry-run hook-run config toggle, a notes-review disable toggle, `.mintignore` file.

### Current State

- **All 20 subtopics decided.** The release pipeline is fully specified end-to-end: version → preflight → hooks → AI notes (with interactive review) → record → tag/push → publish, plus regenerate/heal, config schema, CLI surface, and `mint init`. Ready for specification.
