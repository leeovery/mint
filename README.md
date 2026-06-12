<div align="center">

# 🌿 Mint

**AI-minted releases and commits**

A Go CLI that cuts releases and writes commits with AI-generated notes,
<br>wrapped in git-safe automation that reviews everything before it mutates anything.

[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8.svg)](https://go.dev)

[Install](#install) · [Quick Start](#quick-start) · [Commands](#commands) · [Configuration](#configuration) · [The AI Transport](#the-ai-transport) · [Safety Model](#safety-model)

</div>

---

Mint replaces the per-project release script. `mint release` runs the whole pipeline (version bump, AI release notes, changelog, tag, atomic push, provider release) with a review gate before anything is written. `mint commit` mints a Conventional Commits message from your diff, shows it to you, and commits on accept.

Everything interactive is reviewable, everything unattended fails loud, and nothing touches your repo until you say yes.

## Why Mint?

Release scripts accrete: bump the version file, regenerate the changelog, remember the tag prefix, push the tag *and* the branch, create the GitHub release, don't forget the build hook. Every project grows its own slightly-broken copy.

Mint folds that into one binary with one config file:

- **AI does the writing.** Release notes are generated from the actual diff and commit history; commit messages from the staged changes. You review, edit, or regenerate at a gate; the AI never gets the last word.
- **One atomic point of no return.** Everything before the `git push --atomic` is unwindable: if anything fails pre-push, mint surgically unwinds its own commits, tag, and stash, and only its own, leaving the repo exactly as it found it.
- **Mutations are lock-safe.** Every git mutation retries past a contended `.git` lock and clears provably stale ones, so a background agent or editor holding the index lock can't blow up a release.
- **Unattended means unattended.** `-y` runs end to end or fails loud. Mint never hangs on a hidden prompt and never commits an empty message because nobody was there.

## Install

**Homebrew**

```bash
brew install leeovery/tools/mint
```

**From source**

```bash
go build -o mint ./cmd/mint
```

## Quick Start

```bash
mint init                 # drop .mint.toml + the release shim into your repo
mint release              # cut a patch release: notes → review → tag → push → publish
mint release -m           # minor bump
mint commit -a            # stage tracked changes, mint a commit message, review, commit
mint commit -Apy          # stage everything, auto-accept, push: fully unattended
```

## Commands

### `init`

Scaffold mint into a repo: writes a commented `.mint.toml` (every key shown at its default, optional keys commented with one-line explanations) and a `release` shim at the git-resolved repo root. Idempotent: existing files are skipped unless `--force`.

```bash
mint init [--force] [--plain]
```

| Flag | Description |
|---|---|
| `--force` | regenerate (overwrite) existing files |
| `--plain` | force plain (un-styled) output |

### `release`

Cut a release. The pipeline: preflight gates (clean tree, on the release branch, tag free, remote in sync, provider auth) → resolve the new version → generate AI release notes from the diff → **notes review gate** → changelog update + bookkeeping commit → `pre_tag` hook → annotated tag → **atomic push** (branch + tag in one `git push --atomic`) → provider release → `post_release` hook.

```bash
mint release [-p | -m | -M | --set-version X.Y.Z] [options]
```

| Flag | Description |
|---|---|
| `-p, --patch` | patch bump (default) |
| `-m, --minor` | minor bump |
| `-M, --major` | major bump |
| `--set-version V` | explicit version `X.Y.Z` (mutually exclusive with bump flags) |
| `-d, --dry-run` | read-only run: print the plan, make no changes |
| `-y, --yes` | skip the confirmation/notes-review gate |
| `--no-ai` | skip the AI notes path; use the commit-subject fallback body |
| `--autostash` | stash/restore unrelated WIP around the run |
| `--any-branch` | bypass the release-branch gate |
| `--plain` | force plain (un-styled) output |

At the notes review gate, a single keypress (no Enter needed): **`y`** accept (Enter also accepts), **`n`** abort, **`e`** edit in `$EDITOR`, **`r`** regenerate, Ctrl-C aborts cleanly.

A `--dry-run` generates and caches the notes (~1 hour); a real run within the window reuses the previewed bytes instead of calling the AI again.

```bash
mint release                        # patch release, interactive
mint release -m -y                  # minor release, unattended
mint release --set-version 2.0.0    # explicit version
mint release -d                     # preview the full plan, change nothing
```

### `release regenerate`

Regenerate the notes for an *existing* release and rewrite the chosen surface(s): the provider release body, `CHANGELOG.md`, or both.

```bash
mint release regenerate <version> [options]
mint release regenerate --all [options]
```

| Flag | Description |
|---|---|
| `--reuse` | source = the tag annotation body (no AI); implies `--target release` |
| `--fresh` | source = re-diff + AI (default) |
| `--target SURFACE` | surface(s) to write: `release`, `changelog`, or `both` |
| `--all` | regenerate every version, oldest → newest |
| `-y, --yes` | skip the confirmation / per-version review gate |
| `--plain` | force plain (un-styled) output |

```bash
mint release regenerate 1.4.0                       # interactive: asks source/target
mint release regenerate v1.4.0 --reuse              # tag body → GitHub release, no AI
mint release regenerate --all --target changelog    # rebuild the whole changelog
```

### `commit`

Mint an AI-generated Conventional Commits message from the would-be-committed diff, review it at the gate, and create the commit. Nothing is staged or committed until you accept; a decline leaves the index byte-for-byte untouched.

```bash
mint commit [-a | -A] [-p] [-y] [--no-ai] [--plain]
```

| Flag | Description |
|---|---|
| `-a, --all` | stage tracked changes at accept (`git commit -a` semantics) |
| `-A, --add-all` | stage everything incl. untracked at accept (`git add -A`) |
| `-p, --push` | push after a successful commit (mint never pushes without this flag) |
| `-y, --yes` | auto-accept the review gate |
| `--no-ai` | skip AI generation; write the message in `$EDITOR` |
| `--plain` | force plain (un-styled) output |

Short flags bundle: `-Ap`, `-Apy`, `-ay` all work.

At the gate, a single keypress (no Enter needed): **`y`** accept (Enter also accepts), **`n`** abort, **`e`** edit in `$EDITOR` (loops back to the gate), **`r`** regenerate with a one-time context line, Ctrl-C aborts cleanly.

When the AI can't produce a message (`--no-ai`, a transport failure, or a diff over `max_diff_lines`), mint opens `$EDITOR` (resolved via git's own chain: `GIT_EDITOR`, `core.editor`, `$VISUAL`, `$EDITOR`) and the save becomes the accept. Unattended runs with no message source fail loud instead.

A failed `-p` push never unwinds the commit: mint warns once with git's own stderr passed through verbatim, keeps the commit, and exits non-zero.

```bash
mint commit                # commit the index as staged
mint commit -a             # stage tracked changes too
mint commit -Ap            # stage everything, commit, push
mint commit --no-ai        # skip the AI, write it yourself
```

### `version`

Print mint's own version. `mint --version` is equivalent.

### `help`

`mint help` lists the commands; every verb also takes `-h`/`--help`.

## Configuration

`mint init` writes a commented `.mint.toml` at the repo root. The file is fully optional: every key defaults sensibly.

```toml
# --- Engine-level keys (shared by every mint verb) ---

ai_command = 'claude -p'
max_diff_lines = 50000

# diff_exclude = ['skills/**/knowledge.cjs', '*.min.js']

[release]
tag_prefix = 'v'
commit_prefix = '🌿'
changelog = true
publish = true
on_notes_failure = 'abort'

# release_branch = 'main'
# version_file = 'bin/tool'
# version_pattern = 'RELEASE_VERSION="{version}"'
# provider = 'github'
# context = 'Emphasise user-facing changes.'
# prompt = '.mint/notes-prompt.md'

# [release.hooks]
# preflight = 'scripts/check.sh'
# pre_tag = 'npm run build'
# post_release = 'scripts/notify.sh'

# [commit]
# context = 'Reference the ticket number if the branch carries one.'
# prompt = '.mint/commit-prompt.md'
```

### Shared engine keys

| Key | Default | Description |
|---|---|---|
| `ai_command` | `claude -p` | the AI invocation: prompt on stdin, message on stdout (see [The AI Transport](#the-ai-transport)) |
| `max_diff_lines` | `50000` | diffs over this (post-exclusion) skip the AI |
| `diff_exclude` | `[]` | pathspec globs kept out of every AI diff (lockfiles, generated code) |

### `[release]`

| Key | Default | Description |
|---|---|---|
| `tag_prefix` | `v` | tag name prefix (`v1.4.0`) |
| `commit_prefix` | `🌿` | brand prefix on mint's bookkeeping commit |
| `release_branch` | auto | branch releases must run on (default: derived from `origin/HEAD`) |
| `publish` | `true` | create the provider (GitHub) release |
| `changelog` | `true` | maintain `CHANGELOG.md` |
| `provider` | auto | publishing driver (default: detected from the remote host) |
| `context` | | project guidance injected into the notes prompt |
| `prompt` | | path to a full notes-prompt override file |
| `on_notes_failure` | `abort` | `abort` fails loud; `fallback` uses the commit-subject list (or `fallback` string) |
| `fallback` | | fixed fallback body, used verbatim by `on_notes_failure = 'fallback'` and `--no-ai` |
| `version_file` | | write the new version into this file (omit = tag-only release) |
| `version_pattern` | | line to replace inside `version_file` (omit = the whole file is the version) |

### `[release.hooks]`

| Hook | Runs |
|---|---|
| `preflight` | before any release work; failure aborts |
| `pre_tag` | after notes, before the tag (string or array of commands) |
| `post_release` | after the release is published |

### `[commit]`

| Key | Default | Description |
|---|---|---|
| `context` | | project guidance injected into the commit-message prompt |
| `prompt` | | path to a full commit-prompt override file |

Both verbs share the two-knob model: `context` *injects into* the default prompt; `prompt` *replaces* it. Unknown or mistyped keys fail loud at load; mint never silently ignores config.

## The AI Transport

Mint owns the prompt; the command is just transport. `ai_command` is any executable that reads a finished prompt on **stdin** and writes the message body to **stdout**:

```toml
ai_command = 'claude -p'                       # the default
ai_command = 'claude -p --model sonnet'        # pin a model
ai_command = 'llm -m gpt-4o'                   # any CLI with the same stdin/stdout contract
```

The transport applies a 60s per-attempt deadline, retries bad output (empty/non-zero exit) exactly once, and routes failures by cause: release follows `on_notes_failure`; commit drops to the `$EDITOR` fallback. A Ctrl-C is a clean abort, never a retry.

## Safety Model

- **Mutate nothing until accept.** `mint commit` computes the would-be-committed diff read-only; staging (`git add`) happens only after you accept. Declining is a true no-op.
- **Surgical unwind before the point of no return.** Everything `mint release` does before the atomic push is tracked; on any pre-push failure (including Ctrl-C) mint removes *its own* commits, tag, and autostash (never your work) and reports exactly what it undid.
- **One atomic push.** Branch and tag go up in a single `git push --atomic`; there is no window where the tag exists without its commit.
- **Never unwind after success.** A failed post-commit push or `post_release` hook warns, with the tool's own output passed through verbatim, and keeps the work.
- **Lock-resilient mutations.** Every `git` mutation retries contended `.git` locks and clears provably stale ones.
- **Fail loud, never hang.** Non-TTY without `-y` is a hard error. Unattended runs with no message source abort with one clear line.

## Output

Mint renders styled output on a TTY and plain byte-pure lines when piped (or under `--plain`). Failures are mirrored to stderr for scripting. Exit codes: `0` success, `1` runtime failure or user abort, `2` usage error.

## License

MIT
