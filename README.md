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

### Setup

mint is an AI tool for commits & releases. To configure it for your project, pass the prompt below to your AI of choice — Claude, Codex, or whatever you run (we find Opus-level models do the best work here):

```bash
Run `mint setup` and follow what it prints.
```

`mint setup` emits a version-matched setup guide that inspects your project and proposes a config — it is the source of truth, so this README does not reproduce it here. (This assumes mint is already installed; see [Install](#install).)

## Commands

### `init`

Scaffold mint into a repo: writes a minimal `.mint.toml` (empty body plus a short header comment pointing to the GitHub docs and `mint setup`) and a `release` shim at the git-resolved repo root. Idempotent: existing files are skipped unless `--force`.

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
| `--notes-file P` | release with the finished notes in file P (no AI) |
| `--autostash` | stash/restore unrelated WIP around the run |
| `--any-branch` | bypass the release-branch gate |
| `--plain` | force plain (un-styled) output |

At the notes review gate, a single keypress (no Enter needed): **`y`** accept (Enter also accepts), **`n`** abort, **`e`** edit in `$EDITOR`, **`r`** regenerate, Ctrl-C aborts cleanly.

`--notes-file P` releases a finished body you already hold: mint reads P once at startup (a relative path resolves against the directory you ran mint from; there is no stdin form) and ships its contents byte-identical as the notes, in place of every other body, first-release and empty-diff ones included. It makes no AI call, so the size cap, chunking, the dry-run cache and `on_notes_failure` never apply. An attended run still shows the body at the notes review gate first (`y`, `n`, `e`; no `r`, since there is no AI output to redo); `-y` accepts it unattended. P may sit in the repo, untracked or a tracked file with uncommitted edits: mint keeps it out of the clean-tree check, out of `--autostash`'s stash and out of every commit the release makes (the `pre_tag` artifact commit and the bookkeeping commit alike), and leaves it as it was, edits you had staged still staged and unstaged ones unstaged. Removing it afterwards is up to you.

When the notes fail under `on_notes_failure = abort` (the default), whatever the cause (a failed call, a timeout, the AI tool not installed, chunking exhausted), an attended release is offered the commit-subject list, writing the notes in the editor, or an abort, while an unattended one stops (`-y`). At the terminal, beneath the ✗ line, the AI's own output and the saved part-notes path where a file was written, a single keypress chooses: **`c`** releases with this release's commit-subject list (`git log --format=%s {lastTag}..HEAD`, the maintenance-release body when it is empty), **`e`** opens `$EDITOR` on the chunk notes this run saved, where it saved a file, and otherwise empty, **`a`** aborts (Enter also aborts). `e` edits a copy: the saved file keeps the chunk notes as the calls returned them, and an empty or whitespace-only body returns to the choice, as does an editor that cannot be launched. `c` and `e` take the body to the notes review gate as a non-AI body (`y`, `n`, `e`; no `r`), and the release goes on as usual. An abort, like an editor that fails or stdin closing at the choice, ends the run before any tag and unwinds what the run made, the `pre_tag` artifact commit included. A failed `r` at the review gate and a Ctrl-C during the notes abort without the choice.

A `--dry-run` generates and caches the notes (~1 hour); a real run within the window reuses the previewed bytes instead of calling the AI again.

Notes of any size go through the AI. When the composed notes prompt (instructions, Change Map, diff and closing reminder) fits in `max_prompt_bytes` (default 1,500,000 bytes), mint makes one call. Over it, mint skips that call and **chunks** the notes: the diff's files are packed in path order into chunk calls that each fit the cap, every one carrying the release's instructions (the default prompt with `context`, or your `[release].prompt` override), the whole-release Change Map and which part it is; then exactly one merge call turns every part's notes into the release's notes. The merge output ships byte-identical, reaches the review gate as AI notes (`r` included), and is what the tag, changelog and provider release carry. Chunking has no setting and asks nothing. `release regenerate` from a fresh source chunks the same way, per version.

When a chunked run fails, through a part that fails for good or a merge that fails, mint keeps the part notes that did finish in `{user cache dir}/mint/chunk-notes/{repo path, separators as dashes}/{tag}.md` (the user cache dir is `~/Library/Caches` on macOS, `~/.cache` on Linux), beside the dry-run cache but outside its pruning, so the file outlives the run. It holds each finished part's notes exactly as the AI wrote them, in part order, with a blank line between one part's notes and the next; notes from calls still running when the run failed are kept with the rest. When any part has no notes, the file opens with a marker line, then a blank line: `**Notes missing:** no notes were written for {files} — add them before releasing.` It lists every file with no notes, including files in parts that never started, and marks `(partly)` a file cut into pieces of which only some have notes. When every part finished and only the merge failed, the file carries no marker. The failure names the file's full path, and the file is written whatever `on_notes_failure` holds. A later failure for the same tag overwrites it. When the file cannot be written, or the user cache dir cannot be resolved, the failure stays the AI's and names no file, and mint warns that the finished chunk notes could not be saved, with the error beneath the warning. A run in which no part finished writes no file, nor does a failed single call that was never split into parts, and a Ctrl-C saves nothing. Tidy the notes, filling in the files the marker lists and removing the marker line, before releasing them with `--notes-file`: `mint release --notes-file <path>` while the version is not out, or `mint release regenerate <version> --notes-file <path>` once it is.

```bash
mint release                        # patch release, interactive
mint release -m -y                  # minor release, unattended
mint release --set-version 2.0.0    # explicit version
mint release -d                     # preview the full plan, change nothing
mint release -y --notes-file /tmp/notes.md  # finished notes, no AI
```

### `release regenerate`

Regenerate the notes for an *existing* release and rewrite the chosen surface(s): the provider release body, `CHANGELOG.md`, or both. The **source** (where the notes come from) and the **target** (what gets written) are independent — any source can write any surface.

```bash
mint release regenerate <version> [options]
mint release regenerate --all [options]
```

| Flag | Description |
|---|---|
| `--source SOURCE` | where notes come from: `fresh` (re-diff + AI, default), `tag` (annotation body), or `release` (existing provider release body) |
| `--notes-file P` | notes source: the finished notes in file P (one version only) |
| `--target SURFACE` | surface(s) to write: `release`, `changelog`, or `both` |
| `--all` | regenerate every version, oldest → newest |
| `-y, --yes` | skip the confirmation / per-version review gate |
| `--plain` | force plain (un-styled) output |

The two axes are symmetric and independent: `--source` picks *where the notes come from*, `--target` picks *what gets written*. The `tag` and `release` sources run no AI — they read existing notes verbatim, which makes them a fast way to *backfill a `CHANGELOG.md`* from tags or releases you already have. `--source release` needs a resolvable provider (e.g. a GitHub origin). Under `-y` a `--target` is required (mint never guesses which live surface to rewrite unattended). In an `--all` run, a version with no usable source (a tag with no annotation, a release with no body, or a diff that fails AI generation) is skipped and reported — the other versions still land.

`--notes-file P` is a fourth notes source beside `fresh`, `tag` and `release`: the finished body in file P, such as notes you wrote by hand for an old version. It runs no AI. Giving it settles the source, so mint asks no source question; it reads P once at startup (a relative path resolves against the directory you ran mint from) and writes its contents byte-identical to the chosen target(s), the oldest version included. One file answers one version, so it serves `regenerate <version>` only. An attended run shows the body at the review gate first (`y` accept, `n` abort, `e` edit; no `r`, since there is no AI output to redo); an edit changes what is written, never P itself. On a `changelog` or `both` target, P may sit in the repo, untracked or a tracked file with uncommitted edits: mint keeps it out of the clean-tree check and the changelog commit, and leaves it as it was.

A `fresh` regenerate whose chunked run fails saves its finished chunk notes exactly as a forward release does (see [`release`](#release)): to `{user cache dir}/mint/chunk-notes/{repo path, separators as dashes}/{tag}.md`, keyed by the version's own tag, with the same missing-notes marker, and with the same warning when the file cannot be written. A single-version run names the file's full path in its failure. Under `--all`, each failed version saves its own file and is skipped, and the batch carries on. A version whose single call failed, or whose run finished no part, saves no file. Tidy a saved file, then put it on its version with `mint release regenerate <version> --notes-file <path>`, one version at a time.

Targets differ in what they touch: a `changelog` (or `both`) target **commits `CHANGELOG.md` and pushes it**; a `release` target updates the provider release in place with **no git changes**. Regenerated changelog entries keep each version's *original* release date, not today's. `both` is non-atomic — the changelog is committed and pushed first, then the provider release. There are only two surfaces, so `both` is how you write more than one; there is no partial multi-select beyond `release`, `changelog`, or `both`.

```bash
mint release regenerate 1.4.0                                    # interactive: asks source then target
mint release regenerate v1.4.0 --source tag                     # tag body → GitHub release, no AI
mint release regenerate --all --source tag --target changelog   # build the whole changelog from tag annotations
mint release regenerate --all --source release --target changelog # …or from the published GitHub releases
mint release regenerate --all --source fresh --target both      # re-diff every version, rewrite releases + changelog
mint release regenerate 1.4.0 --notes-file notes.md --target both -y  # finished notes, no AI
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

When the AI can't produce a message (`--no-ai`, a transport failure, or a composed prompt over `max_prompt_bytes`), mint opens `$EDITOR` (resolved via git's own chain: `GIT_EDITOR`, `core.editor`, `$VISUAL`, `$EDITOR`) and the save becomes the accept. Unattended runs with no message source fail loud instead.

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

`mint init` writes a **minimal** `.mint.toml` at the repo root: an empty body plus a short header comment that points to the GitHub docs (this human config reference) and to `mint setup` (AI-assisted setup). The file is fully optional — every key has a compiled default, so an empty file is valid and changes nothing; add a key only to set a value that differs from its default.

```toml
# .mint.toml — mint configuration. This file is fully OPTIONAL: every default is
# compiled into mint, so an empty file is valid and changes nothing. Add a key only
# to set a value that differs from the default.
#
# Config reference (every key, level, and default): https://github.com/leeovery/mint
# AI-assisted setup (inspects your project and proposes config): run `mint setup`
```

The per-key reference tables below are the authoritative human config reference — every key, its level, and its default.

### Shared engine keys

`ai_command` and `timeout` live at **both** the shared (top) level and per-verb (`[release]` / `[commit]`); the rest are shared-only. Each of the two resolves per-key independently through `[verb].<key>` → top-level shared `<key>` → shipped default — so a verb override repoints only that key for that verb, and an unset override falls through to the shared value, then to the compiled default.

| Key | Default | Description |
|---|---|---|
| `ai_command` | `claude -p --model sonnet --safe-mode --tools "" --strict-mcp-config --no-session-persistence --system-prompt "Follow the instructions you are given and output only what they ask for."` | the AI invocation: prompt on stdin, message on stdout; split into program + arguments by POSIX shell quoting rules and run directly, never through a shell (see [Quoting in `ai_command`](#quoting-in-ai_command)); resolves `[verb]` → shared → default (see [The AI Transport](#the-ai-transport)) |
| `timeout` | `180` on release, `60` on commit | per-attempt AI deadline in seconds; `0` = no limit; resolves `[verb]` → shared → default. Raise it if your `ai_command` runs slowly (see [The AI Transport](#the-ai-transport)) |
| `max_prompt_bytes` | `1500000` | cap, in bytes, on the composed prompt of any single AI call: instructions, Change Map, diff and closing reminder together; no call mint makes is bigger. Must be positive — `0` is rejected, never "no cap" |
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
| `on_notes_failure` | `abort` | `abort` fails loud: an attended release is offered the commit-subject list, writing the notes in the editor, or an abort, while an unattended one stops (`-y`); `fallback` ships the commit-subject list (the maintenance-release body when it is empty) |
| `version_file` | | write the new version into this file (omit = tag-only release) |
| `version_pattern` | | line to replace inside `version_file` (omit = the whole file is the version) |
| `ai_command` | shared | optional per-verb override of the AI command for `release` only; resolves `[release]` → shared → default |
| `timeout` | shared, else `180` | optional per-verb override of the per-attempt AI deadline (seconds) for `release` only; resolves `[release]` → shared → default |

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
| `ai_command` | shared | optional per-verb override of the AI command for `commit` only; resolves `[commit]` → shared → default |
| `timeout` | shared, else `60` | optional per-verb override of the per-attempt AI deadline (seconds) for `commit` only; resolves `[commit]` → shared → default |

Both verbs share the two-knob model: `context` *injects into* the default prompt; `prompt` *replaces* it. Unknown or mistyped keys fail loud at load; mint never silently ignores config.

## The AI Transport

Mint owns the prompt; the command is just transport. `ai_command` is any executable that reads a finished prompt on **stdin** and writes the message body to **stdout**:

```toml
ai_command = 'claude -p --model opus'          # a different model, without the lean flags
ai_command = 'llm -m gpt-4o'                   # any CLI with the same stdin/stdout contract
```

Both `ai_command` and `timeout` can be set shared (top-level) or per-verb under `[release]` / `[commit]`, each resolving `[verb]` → shared → default.

The transport applies a per-attempt deadline — `timeout` seconds, default `180` for release and `60` for commit — retries bad output (empty/non-zero exit) exactly once, and routes failures by cause: release follows `on_notes_failure`; commit drops to the `$EDITOR` fallback. A timeout is **fatal**: it is reported immediately and never retried (the single retry covers bad *content* only). A Ctrl-C is a clean abort, never a retry.

Setting `timeout = 0` disables the per-attempt deadline entirely: the AI call then runs **unbounded**. This is a deliberate, operator-chosen exception to mint's "fail loud, never hang" posture — you are opting a slow command into an open-ended run and you own that trade-off.

`ai_command` and `timeout` resolve independently per verb, so overriding one does **not** touch the other. If you pin a slower model for a verb, raise that verb's `timeout` in the **same** `[release]` / `[commit]` table — mint does **not** auto-bump the timeout, warn, or require the pair, so a slow command left under its default (`180` for release, `60` for commit) will hit the fatal per-attempt deadline. Overriding both keys together for a slow verb is the supported pattern; it is your responsibility, not enforced.

### The shipped default

With no `ai_command` set at any level, every AI call mint makes runs:

```text
claude -p --model sonnet --safe-mode --tools "" --strict-mcp-config --no-session-persistence --system-prompt "Follow the instructions you are given and output only what they ask for."
```

An `ai_command` you set replaces this command whole, lean flags included: to change only the model, copy it and edit `--model`.

It is Claude Code in lean form, so a call carries mint's prompt and nothing else:

- `--model sonnet` pins the model, by alias, so zero-config behaviour does not depend on the model your Claude CLI happens to default to.
- `--safe-mode` switches off CLAUDE.md files (the repo's and your global one), skills, plugins, hooks and MCP servers. Auth and model selection keep working.
- `--tools ""` drops the built-in tool definitions.
- `--strict-mcp-config` loads only MCP configuration passed on the command line, and none is passed.
- `--no-session-persistence` stops each one-shot call being saved to disk as a resumable session.
- `--system-prompt` replaces Claude Code's own system prompt. It names no task, because the same command serves every call; each call's own prompt carries its job.

`--bare` is not used: it forces API-key auth and breaks the subscription login.

What the lean form drops, it drops on purpose. CLAUDE.md files and Claude Code's own system prompt no longer reach mint's calls, so instructions kept there that shaped release notes or commit messages stop doing so. Conventions you want applied belong in the `[release]` and `[commit]` `context` and `prompt` keys, which ride in every release-notes and commit-message call.

A Claude CLI too old for one of these flags rejects it as an unknown option and exits non-zero, which lands on the ordinary AI-failure path. mint does not probe the version or fall back to a plainer command. Upgrade claude, or set your own `ai_command`.

### Quoting in `ai_command`

Every `ai_command` — shared, `[release]`, `[commit]` and the shipped default — is split into a program name and its arguments by POSIX shell quoting rules. The program then runs directly; no shell is ever started.

- Unquoted whitespace separates words.
- Single quotes keep everything up to the next single quote literally.
- Double quotes keep everything up to the next unescaped double quote. Inside them, a backslash escapes `"`, `\`, `$` and the backtick, and is otherwise literal.
- Outside quotes, a backslash makes the next character literal.
- A backslash followed by a newline is removed, as a line continuation.
- Quoted and unquoted parts written together form one word: `--system-prompt="Be terse"` is the single argument `--system-prompt=Be terse`.
- A quoted empty string, `""` or `''`, is one empty argument.
- Nothing else is shell syntax. `$`, backticks, `*`, `~`, `|`, `;`, `&`, `<` and `>` are ordinary characters within a word: no variable expansion, command substitution, globbing, pipes or redirection.

Write the value as a TOML literal (single-quoted) string so backslashes and double quotes reach the splitter as written; use a multi-line literal (`'''…'''`) for a line continuation:

```toml
ai_command = 'claude -p --model sonnet --tools "" --system-prompt "Be terse."'
ai_command = '''
claude -p \
  --model opus'''
```

A command with no quotes or backslashes splits exactly as it always has, on runs of whitespace.

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
