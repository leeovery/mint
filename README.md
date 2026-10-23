<div align="center">

# 🌿 Mint

**AI-minted releases and commits**

A Go CLI that cuts releases and writes commits with AI-written notes,
<br>and shows you every word before anything leaves your machine.

[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8.svg)](https://go.dev)

[Install](#install) · [Quick Start](#quick-start) · [Commands](#commands) · [Release Notes in Depth](#release-notes-in-depth) · [Configuration](#configuration) · [The AI Transport](#the-ai-transport) · [Safety Model](#safety-model)

</div>

---

Mint replaces the release script every project grows. `mint release` bumps the version, has the AI write the release notes from the diff, updates the changelog, tags, pushes and publishes the GitHub release, and stops to show you the notes before anything is pushed. `mint commit` writes a Conventional Commits message from the diff you're about to commit, and commits when you accept it.

## Why Mint?

Release scripts accrete: bump the version file, regenerate the changelog, remember the tag prefix, push the tag *and* the branch, create the GitHub release, don't forget the build hook. Every project grows its own slightly broken copy. Mint folds all of it into one binary and one optional config file.

- **Notes from the code that changed.** The AI reads the release's diff, not its commit subjects, so the notes say what changed for the people who use it. You accept, edit or regenerate them before anything ships.
- **Releases of any size.** A release too big for one AI call is written in parts and merged into one set of notes. If the model turns a call down as too big, mint retries it in smaller parts and tells you the size that worked.
- **Notes about the product, not how it was made.** Before writing, mint asks the AI which changed files don't belong in release notes: specs, design docs, test fixtures. You answer once per suggestion, and mint remembers the answer in `.mint.toml`.
- **A failed AI call never strands a release.** At a terminal you choose: ship the commit subjects, write the notes yourself, or abort. Unattended, mint stops and says what to do next. Notes from the parts that finished are saved, ready to release with `--notes-file`.
- **One point of no return.** Branch and tag go up in a single `git push --atomic`. Decline or fail before it, and mint unwinds the commits and tag it made and restores your stashed work. The one thing it keeps is your answers about which files to leave out of the notes.
- **Lean AI calls.** The default command runs Claude Code with CLAUDE.md files, skills, plugins, hooks, MCP servers and tools switched off, so each call carries mint's prompt and nothing else. Any command that reads a prompt on stdin and writes on stdout works.
- **Lock-safe git.** Every git write retries past a held `.git` lock and clears stale ones, so an editor or background agent holding the index can't break a release.
- **Unattended means unattended.** `-y` runs end to end or fails with one clear message. Mint never hangs on a hidden prompt, and never commits an empty message because nobody was there.

## Install

You need git. The default AI command runs [Claude Code](https://claude.com/claude-code) (`claude`), and publishing GitHub releases needs the [GitHub CLI](https://cli.github.com) (`gh`), signed in.

**Homebrew**

```bash
brew install leeovery/tools/mint
```

**From source** (Go 1.26 or later)

```bash
go build -o mint ./cmd/mint
```

## Quick Start

```bash
mint init                 # add .mint.toml and the ./release shim to your repo
mint release              # patch release: review the notes, then tag, push and publish
mint release -m           # minor bump
mint release -d           # dry run: preview the version and notes, change nothing in the repo
mint commit -a            # stage tracked changes, write the message, review, commit
mint commit -Apy          # stage everything, accept, push: no questions asked
```

### Setup

To configure mint for your project, give your AI of choice (Claude, Codex, or whatever you run) this prompt. Opus-level models do the best work here.

```bash
Run `mint setup` and follow what it prints.
```

`mint setup` prints a setup guide matched to the installed version: the agent inspects your project and proposes a config for you to approve. The guide is the source of truth for setup, so this README doesn't repeat it.

## Commands

| Command | What it does |
|---|---|
| [`mint init`](#init) | Adds `.mint.toml` and a `release` shim to the repo |
| [`mint release`](#release) | Cuts a release: version, AI notes, changelog, tag, atomic push, GitHub release |
| [`mint release regenerate`](#release-regenerate) | Rewrites the notes of a release that's already out, on GitHub, in `CHANGELOG.md`, or both |
| [`mint commit`](#commit) | Writes a commit message from your diff and commits when you accept it |
| [`mint setup`](#setup-1) | Prints the setup guide for an AI agent to follow |
| [`mint version`](#version) | Prints mint's version |
| [`mint help`](#help) | Lists the commands |

### `init`

Adds mint to a repo: a minimal `.mint.toml` (an empty body under a short header that points to this README and to `mint setup`) and a `release` shim, both at the repo root. `./release` runs `mint release` with whatever arguments you give it, and says how to install mint where it's missing. Existing files are left alone unless you pass `--force`.

```bash
mint init [--force] [--plain]
```

| Flag | What it does |
|---|---|
| `--force` | overwrite existing files |
| `--plain` | force plain (un-styled) output |

### `release`

Cuts a release. An attended run asks you twice: once to confirm the version, once to accept the notes.

1. **Confirm the version.** `Release v1.4.0 → v1.4.1 (patch)?`, or `Release v0.0.1 (first release)?` the first time.
2. **Check the repo.** Mint fetches tags, then checks the tree is clean, you're on the release branch, the tag is free here and on `origin`, and the branch is in sync with `origin`. Then the `preflight` hook runs.
3. **Build.** The `pre_tag` hook runs, and whatever it leaves in the tree is committed as `chore(release): pre-tag artifacts for v1.4.1`.
4. **Write the notes.** Mint checks for [files that don't belong in the notes](#files-that-dont-belong-in-the-notes), then the AI writes the notes from the diff since the last tag.
5. **Review.** The notes review gate shows you the notes.
6. **Record.** The new version goes into `version_file` and the notes into `CHANGELOG.md`, committed together as `🌿 Release v1.4.1`.
7. **Tag and push.** Mint checks `gh` is signed in (when publishing), tags, and pushes the branch and the tag in one `git push --atomic`. This is the point of no return.
8. **Publish.** The GitHub release is created, then the `post_release` hook runs.

```bash
mint release [-p | -m | -M | --set-version X.Y.Z] [options]
```

| Flag | What it does |
|---|---|
| `-p, --patch` | patch bump (default) |
| `-m, --minor` | minor bump |
| `-M, --major` | major bump |
| `--set-version V` | explicit version `X.Y.Z`, higher than the latest (can't be combined with a bump flag) |
| `-d, --dry-run` | preview the run: print the plan and the notes, change nothing in the repo |
| `-y, --yes` | accept the version and the notes without asking |
| `--no-ai` | skip the AI; release with the commit-subject list |
| `--notes-file P` | release with the finished notes in file P (no AI; can't be combined with `--no-ai`) |
| `--autostash` | stash unrelated work before the run and restore it after |
| `--any-branch` | release from the current branch, whatever `release_branch` says |
| `--plain` | force plain (un-styled) output |

At the notes review gate: **`y`** accept (Enter also accepts), **`n`** abort, **`e`** edit in your editor, **`r`** regenerate, with an optional line of extra context for the AI. At a terminal each is a single keypress; with `--plain` or piped output, type the key and press Enter. Ctrl-C aborts. Notes the AI didn't write (`--notes-file`, `--no-ai`, the commit-subject list) get `y`, `n` and `e`, since there's no AI output for `r` to redo.

Under `--no-ai` the notes are the commit subjects since the last tag (`git log --format=%s`), less mint's own answers commits. Without `--notes-file`, a first release ships `Initial release.` and a release whose diff is empty once exclusions are taken out ships `Maintenance release — no notable source changes`, with or without the AI.

```bash
mint release                        # patch release, interactive
mint release -m -y                  # minor release, unattended
mint release --set-version 2.0.0    # explicit version
mint release -d                     # preview the full plan, change nothing
mint release -y --notes-file /tmp/notes.md  # finished notes, no AI
```

How mint writes notes for big releases, what it does with files that don't belong in the notes, and what happens when the AI fails are covered in [Release Notes in Depth](#release-notes-in-depth).

### `release regenerate`

Rewrites the notes of a release that's already out, on the GitHub release, in `CHANGELOG.md`, or both. Where the notes come from (the **source**) and what gets written (the **target**) are independent: any source can write any target.

```bash
mint release regenerate <version> [options]
mint release regenerate --all [options]
```

| Flag | What it does |
|---|---|
| `--source SOURCE` | where the notes come from: `fresh`, `tag` or `release` |
| `--notes-file P` | take the notes from file P instead (one version only; can't be combined with `--source`) |
| `--target SURFACE` | what to write: `release`, `changelog` or `both` |
| `--all` | regenerate every version, oldest to newest |
| `-y, --yes` | skip the questions and the review gate (needs `--target`) |
| `--plain` | force plain (un-styled) output |

| Source | Where the notes come from |
|---|---|
| `fresh` | the AI, from the version's diff against the version before it (the default) |
| `tag` | the tag's annotation body, as it is; no AI |
| `release` | the GitHub release's body, as it is; no AI |
| `--notes-file P` | the finished notes in file P, written byte for byte; no AI |

| Target | What it writes |
|---|---|
| `release` | creates or updates the GitHub release, titled with the tag; no git changes |
| `changelog` | rewrites the version's `CHANGELOG.md` entry, keeping its original release date, then commits (`docs(changelog): regenerate notes for v1.4.0`) and pushes it |
| `both` | the changelog first, then the GitHub release; not atomic |

The `tag` and `release` sources make a fast way to backfill a `CHANGELOG.md` from tags or releases you already have. A tag's annotation body is the message below its subject line, so a one-line tag message counts as no body. `--source release` needs a GitHub remote. The version can be given with or without the tag prefix (`1.4.0` or `v1.4.0`).

Without `--source` or `--target`, an attended run asks for them (`Source?`, then `Target?`): type the answer and press Enter, or press Enter alone for `fresh` and `release`. Under `-y` a `--target` is required, since mint never guesses which live surface to rewrite unattended. An attended run shows the notes at the review gate before writing: `y`, `n`, `e` and `r` for fresh notes, `y` and `n` for a tag or release body, `y`, `n` and `e` for a `--notes-file` body. An edit changes what's written, never the file itself.

Before writing, a `changelog` target checks the tree is clean, you're on the release branch and in sync with `origin`; a `release` target checks `gh` is signed in. No hooks run. A `fresh` source reads the shared and `[release]` `diff_exclude` lists and `[release]`'s `ai_command` and `timeout`. The oldest version has nothing to diff against, so `fresh` gives it `Initial release.`; a `--notes-file` body is the way to give it real notes. Regenerate has no `--dry-run` and never runs the exclusion check. With a `changelog` or `both` target, `--notes-file` P may sit in the repo, untracked or with uncommitted edits: mint keeps it out of the clean-tree check and the changelog commit, and leaves it as it was.

```bash
mint release regenerate 1.4.0                                    # interactive: asks source, then target
mint release regenerate 1.4.0 --source tag --target release      # tag body → GitHub release, no AI
mint release regenerate --all --source tag --target changelog    # build the whole changelog from tag annotations
mint release regenerate --all --source release --target changelog  # …or from the published GitHub releases
mint release regenerate --all --source fresh --target both       # re-diff every version, rewrite releases + changelog
mint release regenerate 1.4.0 --notes-file notes.md --target both -y  # finished notes, no AI
```

What `--all` does with versions it can't regenerate, and how it rebuilds `CHANGELOG.md`, is covered in [Regenerating every version](#regenerating-every-version).

### `commit`

Writes an AI Conventional Commits message from the diff you'd commit, shows it at the review gate, and commits when you accept. The diff is the index as staged, or with `-a`/`-A` the changes those flags will stage. Nothing is staged or committed until you accept, so declining leaves the index exactly as it was.

```bash
mint commit [-a | -A] [-p] [-y] [--no-ai] [--plain]
```

| Flag | What it does |
|---|---|
| `-a, --all` | stage tracked changes at accept (`git commit -a` semantics) |
| `-A, --add-all` | stage everything, untracked files included, at accept (`git add -A`) |
| `-p, --push` | push after the commit (mint never pushes without it) |
| `-y, --yes` | accept the message without asking |
| `--no-ai` | skip the AI; write the message in your editor |
| `--plain` | force plain (un-styled) output |

Short flags bundle: `-Ap`, `-Apy` and `-ay` all work. `-a` and `-A` can't be combined, since `-A` already covers `-a`.

At the gate: **`y`** accept (Enter also accepts), **`n`** abort, **`e`** edit in your editor (back to the gate after), **`r`** regenerate, with an optional line of extra context. At a terminal each is a single keypress; with `--plain` or piped output, type the key and press Enter. Ctrl-C aborts.

When there's no AI message to show (`--no-ai`, a failed or timed-out AI call, a prompt over `max_prompt_bytes`, a diff that `diff_exclude` empties, or a failed `r`), mint opens your editor on an empty message, and saving it is the accept. An empty save aborts. The editor is git's: `GIT_EDITOR`, `core.editor`, `$VISUAL`, `$EDITOR`, then `vi`. An unattended run with no message to commit fails instead.

`-p` runs a plain `git push` after the commit. If the push fails, mint keeps the commit, shows git's own error, and exits `1`.

```bash
mint commit                # commit the index as staged
mint commit -a             # stage tracked changes too
mint commit -Ap            # stage everything, commit, push
mint commit --no-ai        # skip the AI, write it yourself
```

### `setup`

Prints a setup guide for an AI agent to follow: what mint does, every config key with its default, and a procedure for inspecting your project and proposing a `.mint.toml`. It matches the installed version, takes no flags beyond `--help`, and runs anywhere, no repo needed. See [Setup](#setup).

### `version`

Prints mint's own version. `mint --version` does the same.

### `help`

`mint help` (or `-h`, `--help`) lists the commands, and `mint <command> --help` shows a command's flags.

## Release Notes in Depth

### Big releases

When the notes prompt (mint's instructions, the Change Map summarising every changed file, the diff and a closing reminder) fits in `max_prompt_bytes` (default 1,500,000 bytes), mint makes one call. Over it, mint skips that call and writes the notes in parts. The diff's files are packed, in path order, into calls that each fit the cap, and a file too big for one part is cut into pieces of its own. Every part's call carries the release's instructions (the default prompt with `[release].context`, or your `[release].prompt`), the whole release's Change Map, and which part it is. One merge call then turns the parts' notes into the release's notes. There's no setting for this and nothing to answer, and `release regenerate` from a fresh source does the same, per version.

The merged notes ship byte for byte and reach the review gate as AI notes. `r` there re-runs only the merge, over the part notes already written, with the line of context you give it: a change a part's notes missed isn't read again.

A model can have a limit of its own below `max_prompt_bytes`. When a notes call fails and the AI tool printed something, mint asks the AI, in one more call carrying that output, whether the failure means the call was too big for the model. If it does, mint retries that slice of the diff as calls half the size (`too big for the model in one call, retrying in 2 parts`, or `part 2 was too big for the model, retrying it as 2 smaller parts`), up to three times per slice. When the smaller calls wrote the notes, mint warns with a cap the model accepts:

```text
set higher than this model accepts; set max_prompt_bytes = 612000, the largest call that succeeded
```

Under `release regenerate --all` that warning prints once, at the end of the batch. A merge call the model rejects, a timeout, a missing AI tool, or any other failure goes to [When the notes fail](#when-the-notes-fail).

### Files that don't belong in the notes

Before the AI writes a release's notes, mint asks the AI which changed paths look like they don't belong in them: design documents, specs, test changes and other files about how the software is made rather than what it does. The call carries no diff, only the changed paths the notes diff carries, less any `[release].diff_keep` covers, each with its kind of change and the size of its diff. A list that would take more than a tenth of `max_prompt_bytes` is summed up by top-level folder (`vendor/: 4,312 files, 38 MB`). The call runs with the release's `ai_command` and timeout, and with mint's own prompt: `[release].context` and `[release].prompt` don't reach it. While it runs, the notes line reads `Checking for files that don't belong in the notes…`. It runs once per release, so `r` at the review gate never repeats it: the check asks about paths, not notes. It doesn't run for `--notes-file`, `--no-ai`, a first release or an empty diff, and `release regenerate` never runs it.

The AI answers with suggested globs, each with a short reason, or `none`. It never excludes anything itself.

**At a terminal**, an attended release then pauses once on each suggestion that matches a changed path nothing already covers, in the order the AI gave them. Each pause shows the glob and the AI's reason, how many changed files the notes still carry that it matches, the total size of their diffs, and any of them on `[release].diff_keep` or kept earlier in the run, which an Exclude takes out too. A suggestion that only matches files already excluded, kept, or decided earlier in the run gets no pause. Press **`e`** to exclude the glob's files from this release's notes, or **`k`** to keep them (Enter also keeps), so no file leaves the notes unless you choose it.

Once the last pause is answered, mint records the answers in `.mint.toml`'s `[release]` table, an Exclude joining `diff_exclude` and a Keep joining `diff_keep`, and keeps the file's comments and layout as they were. It commits `.mint.toml` alone as `chore(release): release-notes path answers for {tag}`, whatever `commit_prefix` says, and one line says so before the notes are written. The commit goes out with the release's push, so later releases from any clone read the answers and never ask about those paths again. Recorded answers survive an abort or a failure before the push: the unwind takes everything else the run made and re-commits the answers on the commit you started from. Where `.mint.toml` is untracked and git ignores it, mint writes the answers but neither stages nor commits them, since that would take `git add -f` on a file you keep out of the repo, and the line says so. Later releases from this clone read them; other clones are asked again.

Ctrl-C, or stdin closing, at a pause aborts the release before any tag and unwinds what the run made, the `pre_tag` artifact commit included. Nothing is recorded until the last pause is answered.

**Unattended** (`-y`), a release pauses on nothing and excludes nothing: flagged files stay in the diff, the notes read them like any other, and `.mint.toml` is left as it is. After the notes, mint prints each suggestion with the AI's reason, how many changed files it matches and the total size of their diffs, any of those files on `[release].diff_keep`, and the `diff_exclude` line that would exclude it, to set under `[release]`. That line holds the key's current entries with the suggested glob appended, so it replaces the existing line and keeps the earlier exclusions. The suggestions print whatever happens to the notes, including when the run stops on a notes failure.

**A `--dry-run`** asks nothing about the suggestions and writes nothing, so its notes read every flagged file. At a terminal without `-y` it shows each suggestion as a pause would, without asking; an unattended dry run (`-y`, or stdin not a terminal) prints them as an unattended release does, `diff_exclude` line included.

When the check fails, because its call fails, times out or finds no AI tool, because its answer can't be read, or because the list of changed paths won't fit in `max_prompt_bytes` even summed up (then no call is made), mint warns `couldn't check for files that don't belong in the notes`, with the AI's output beneath it where there is any, and the release carries on to its notes with no suggestions. A real AI problem then shows up at the notes call. Ctrl-C during the check cancels the run.

### When the notes fail

What happens when the AI can't write a release's notes, whatever the cause (a failed call, a timeout, the AI tool not installed, a release too big even in parts), depends on `[release].on_notes_failure`.

**Under `abort`** (the default), an attended release is offered the commit-subject list, writing the notes in the editor, or an abort, while an unattended one stops with next moves fitted to the cause. At a terminal mint shows the ✗ line, the AI's own output and, where it saved any, the path of the [saved part notes](#saved-part-notes), then asks `Release without AI notes?`:

- **`c`** releases with the commit subjects since the last tag, less mint's own answers commits (the maintenance-release body when none are left).
- **`e`** opens your editor on the chunk notes this run saved, where it saved a file, and otherwise empty. You edit a copy: the saved file keeps the notes as the AI wrote them. An empty or whitespace-only body returns to the choice, as does an editor that can't be launched.
- **`a`** aborts (Enter also aborts).

`c` and `e` take the body to the review gate (`y`, `n`, `e`; no `r`), and the release goes on as usual. An abort, an editor that fails, or stdin closing at the choice ends the run before any tag and unwinds what it made, the `pre_tag` artifact commit included. A failed `r` at the review gate and a Ctrl-C during the notes abort without the choice.

An unattended release (`-y`) asks nothing. Beneath the ✗ line and the AI's output it prints its next moves, unwinds, and exits `1` before any tag. The moves always include:

- the `--notes-file` route: where the run saved part notes, tidy them and release with `mint release --notes-file <path>`; where it saved none, write the notes and release with `--notes-file`
- `--no-ai`, to release with the commit-subject list
- `on_notes_failure = "fallback"` under `[release]`, to ship the commit-subject list instead of stopping

Then the moves that clear the cause:

| Cause | Next moves |
|---|---|
| The model rejected even the smallest parts | Lower `max_prompt_bytes`; use the lean command (see [The shipped default](#the-shipped-default)) if your `ai_command` carries a large overhead of its own; exclude generated paths with `[release].diff_exclude` |
| Mint's prompt left no room for the diff | Shorten `[release].prompt` when it's set (it replaces `context`), else `[release].context`; with neither set, the cap is below mint's own instructions, so raise `max_prompt_bytes` |
| Part notes too large to merge | None beyond the three above: a lower cap cuts the release into more parts and doesn't shrink what the merge holds |
| Timeout | Raise `[release].timeout`, which each AI call gets on its own |
| AI tool missing | Install it, or set `ai_command` |
| Anything else | The AI's own output names the cause; where the release's `ai_command` isn't the shipped default, the moves also point at the lean command, for a command whose overhead alone can overflow the model's window |

A `--dry-run` asks nothing in any mode: it prints the next moves and exits `1`. At a terminal without `-y` it leaves out the `on_notes_failure = "fallback"` move, since the real run there offers the choice. Part notes it saved stay for the real run to release with `--notes-file` instead of calling the AI again.

**Under `fallback`**, attended or not, there's no choice: the commit-subject list replaces the AI notes and the run goes on, with a warning that names the cause and the AI's output beneath it. Where the run saved part notes, the warning names the file and both commands that release it: `mint release regenerate <version> --notes-file <path>`, which puts them on the version once this release is out, and `mint release --notes-file <path>`, for a run that ends before the push, such as an abort at the review gate. An attended run then shows the list at the review gate (`y`, `n`, `e`). A `--dry-run` previews the list as the notes that would ship, with the same warning, naming a saved file with `mint release --notes-file <path>` only.

**`release regenerate` doesn't read `on_notes_failure`**, so it never ships the commit-subject list over a version's notes on its own. When the notes of an attended `regenerate <version>` from a `fresh` source fail, mint shows the ✗ line, the AI's output and any saved part-notes path, then asks `Regenerate without AI notes?` with the same keys: **`c`** the version's commit subjects (less its own `Release` commit and mint's answers commits, so the list is the one its release would have shipped), **`e`** the editor, opening on any saved part notes, **`a`** abort (Enter also aborts). An abort writes nothing and exits `1`. Unattended (`-y`, or stdin not a terminal), it asks nothing, writes nothing, and prints next moves: the `--notes-file` route, with `mint release regenerate <version> --notes-file <path>`, and the same moves for the cause as a release. There's no `--no-ai` or `fallback` move: both belong to the forward release. Under `--all`, a version whose notes fail is skipped instead (see [Regenerating every version](#regenerating-every-version)).

### Saved part notes

When a release written in parts fails, through a part that fails for good or a merge that fails, mint keeps the part notes that finished in:

```text
{user cache dir}/mint/chunk-notes/{repo path, separators as dashes}/{tag}.md
```

The user cache dir is `~/Library/Caches` on macOS, and `$XDG_CACHE_HOME` (else `~/.cache`) on Linux. The file sits beside the dry-run cache but outside its pruning, so it outlives the run. It holds each finished part's notes exactly as the AI wrote them, in part order, a blank line between one part's notes and the next. When any part has no notes, the file opens with a marker line listing every file without notes, files in parts that never started included, and marking `(partly)` a file cut into pieces of which only some have notes:

```markdown
**Notes missing:** no notes were written for `api/a.go` (partly), `api/b.go` — add them before releasing.
```

When every part finished and only the merge failed, there's no marker. The failure names the file's full path, the file is written whatever `on_notes_failure` says, and a later failure for the same tag overwrites it. Nothing is saved when no part finished, when the notes were one call that was never split, or on Ctrl-C. A failed `r` on part notes saves them too. When the file can't be written, mint warns that the finished chunk notes could not be saved, with the error beneath, and the failure names no file.

A part whose slice held nothing for the notes wrote one line saying so, and the file keeps it as it keeps any part's notes. To release a saved file, fill in the files the marker lists, delete the marker line and any part's line saying it held nothing for the notes, then release it with `--notes-file`: `mint release --notes-file <path>` while the version isn't out, or `mint release regenerate <version> --notes-file <path>` once it is. Where every part's notes are that line, the release holds nothing users meet: release the maintenance line, `Maintenance release — no notable source changes`, in their place rather than an empty file, which `--notes-file` refuses. Mint checks for neither the marker nor a part's line: a file that still has one ships with it.

A `fresh` regenerate saves its part notes the same way, keyed by the version's own tag.

### Finished notes with `--notes-file`

`--notes-file P` releases a finished body you already hold. Mint reads P once at startup (a relative path resolves against the directory you ran mint from; there's no stdin form) and ships its contents byte for byte as the notes, in place of every other body, first-release and empty-diff ones included. An empty file is refused. There's no AI call, so the size cap, parts, the exclusion check, the dry-run cache and `on_notes_failure` never apply. An attended run still shows the body at the review gate (`y`, `n`, `e`); `-y` accepts it.

P may sit in the repo, untracked or a tracked file with uncommitted edits. Mint keeps it out of the clean-tree check, out of `--autostash`'s stash and out of every commit the release makes (the `pre_tag` artifact commit and the bookkeeping commit alike), and leaves it as it was: edits you had staged stay staged, unstaged ones unstaged. Removing it afterwards is up to you.

### Dry runs and cached notes

`-d` runs the release up to the review gate without changing the repo. It fetches tags and calls the AI (the exclusion check and the notes), but runs no hooks, asks nothing, and makes no commit, stash or tag; then it prints the plan and the notes that would ship. With `--autostash` it previews a dirty tree without stashing; without it, a dirty tree fails the clean-tree check as a real run would.

The previewed notes are cached for an hour, outside the repo. A real run within the hour whose diff, version and prompt match asks `Cached notes found for this diff — use them?` (`y` uses them, `r` writes new ones; `-y` uses them) instead of calling the AI for the notes again. A real run that stops before the push leaves its notes cached the same way. Since a dry run skips the hooks, a `pre_tag` that changes files, or an Exclude you answer in the real run, changes the diff, and the real run writes fresh notes.

### Regenerating every version

`release regenerate --all` regenerates every version, oldest to newest: every tag that matches `tag_prefix` followed by a plain `X.Y.Z`. A version with no usable notes (a tag with no annotation body, a release with no body, or notes the AI couldn't write) is skipped with a warning naming the cause, and the other versions still land. A skipped `fresh` version whose part notes were saved names the file and its own `mint release regenerate <version> --notes-file <path>`. The end-of-run summary lists every skip, and the run exits `0`. Declining a version at the review gate, a failed GitHub write, or Ctrl-C ends the batch there.

With a `changelog` or `both` target, `--all` rebuilds `CHANGELOG.md` whole, then commits (`docs(changelog): regenerate release notes`) and pushes it once, at the end: the standard Keep a Changelog preamble, then one entry per version, newest first, each with its original release date. A custom preamble, and entries for anything that isn't a release tag (such as `## [Unreleased]`), are dropped; a skipped version keeps the entry it had. With `both`, each version's GitHub release is written as the batch goes, and the changelog last.

## Configuration

`mint init` writes a **minimal** `.mint.toml` at the repo root: an empty body under a short header that points to this README and to `mint setup`. The file is optional: every key has a compiled default, so an empty file is valid and changes nothing. Add a key only to set a value that differs from its default.

```toml
# .mint.toml — mint configuration. This file is fully OPTIONAL: every default is
# compiled into mint, so an empty file is valid and changes nothing. Add a key only
# to set a value that differs from the default.
#
# Config reference (every key, level, and default): https://github.com/leeovery/mint
# AI-assisted setup (inspects your project and proposes config): run `mint setup`
```

The tables below are the config reference: every key, its level, and its default. Unknown keys and mistyped values fail at load, so a typo never goes unnoticed.

### Shared engine keys

`ai_command` and `timeout` live at **both** the shared (top) level and per verb (`[release]`, `[commit]`). `max_prompt_bytes` is shared-only. `diff_exclude` has a shared list plus a `[release]` and a `[commit]` list, and each verb's list adds to the shared one rather than replacing it. `ai_command` and `timeout` each resolve on their own, `[verb]` → shared → shipped default, so a verb override repoints only that key for that verb, and an unset override falls through to the shared value, then to the default.

| Key | Default | Description |
|---|---|---|
| `ai_command` | `claude -p --model sonnet --safe-mode --tools "" --strict-mcp-config --no-session-persistence --system-prompt "Follow the instructions you are given and output only what they ask for."` | the AI invocation: prompt on stdin, message on stdout; split into program and arguments by POSIX shell quoting rules and run directly, never through a shell (see [Quoting in `ai_command`](#quoting-in-ai_command)); a blank value counts as unset |
| `timeout` | `180` on release, `60` on commit | per-attempt AI deadline in seconds; `0` means no limit; a negative value counts as unset. Raise it if your `ai_command` runs slowly (see [The AI Transport](#the-ai-transport)) |
| `max_prompt_bytes` | `1500000` | cap, in bytes, on the composed prompt of any single AI call: no call mint makes is bigger. Must be positive: `0` is rejected, never "no cap" |
| `diff_exclude` | `[]` | pathspec globs kept out of every AI diff: what neither release notes nor commit messages should read (lockfiles, build output) |

The release-notes diff always leaves out `CHANGELOG.md`, `.mint.toml`, and `version_file` when it holds nothing but the version (no `version_pattern`). Commit messages read all three.

### `[release]`

| Key | Default | Description |
|---|---|---|
| `tag_prefix` | `v` | tag name prefix (`v1.4.0`) |
| `commit_prefix` | `🌿` | brand prefix on mint's bookkeeping commit and the tag message |
| `release_branch` | auto | branch releases must run on (default: derived from `origin/HEAD`; set it where that isn't set) |
| `publish` | `true` | create the GitHub release; `false` stops at tag and push |
| `changelog` | `true` | maintain `CHANGELOG.md` |
| `provider` | auto | publishing driver (default: detected from the remote host); an unsupported value skips publishing with a warning |
| `context` | | project guidance added to the notes prompt: the text itself, or the path of a file under the repo root |
| `prompt` | | path, from the repo root, to a file that replaces the notes prompt; `context` is then ignored |
| `on_notes_failure` | `abort` | `abort` fails loud: an attended release is offered the commit-subject list, writing the notes in the editor, or an abort, while an unattended one stops (`-y`); `fallback` ships the commit-subject list (the maintenance-release body when it is empty). See [When the notes fail](#when-the-notes-fail) |
| `version_file` | | write the new version into this file (omit = tag-only release) |
| `version_pattern` | | the line inside `version_file` to rewrite, with `{version}` where the version sits (`VERSION = "{version}"`); a pattern that matches nothing aborts the release before the tag (omit = the whole file is the version) |
| `ai_command` | shared | override of the AI command for every release AI call and regenerate's `fresh` source; resolves `[release]` → shared → default |
| `timeout` | shared, else `180` | override of the per-attempt AI deadline (seconds) for release and regenerate; resolves `[release]` → shared → default |
| `diff_exclude` | `[]` | pathspec globs kept out of the release-notes diff only, added to the shared `diff_exclude` list; commit messages still read these paths (`.workflows/`, `.claude/`, `docs/`) |
| `diff_keep` | `[]` | pathspec globs whose paths the exclusion check never raises; they add nothing to the release-notes diff and never override an exclusion, so a path both kept and excluded stays out of the notes |

### `[release.hooks]`

| Hook | Runs |
|---|---|
| `preflight` | after mint's own repo checks, before any commit; failure aborts the release |
| `pre_tag` | after preflight, before the notes, changelog and tag; whatever it leaves in the tree is committed as `chore(release): pre-tag artifacts for {tag}` |
| `post_release` | after the push lands, whether or not a GitHub release was created; failure warns, with the hook's stderr, and keeps the release |

Each hook takes a command or an array of commands, run in order from the repo root through `sh -c`, stopping at the first that fails. Every hook gets `MINT_NEW_VERSION` (`1.4.1`), `MINT_PREVIOUS_VERSION` (`1.4.0`, or `0.0.0` on a first release), `MINT_VERSION_TAG` (`v1.4.1`), `MINT_BUMP` (`patch`, `minor`, `major` or `explicit`) and `MINT_DRY_RUN` in its environment. A `--dry-run` runs no hooks and says which it skipped.

`pre_tag` runs before the notes, so the notes read the tree it leaves: its artifact commit is in the notes diff, and its subject is in the commit-subject list. It runs before the changelog and version file are updated, so it sees the previous version there (the new one is in `MINT_NEW_VERSION`). Files git ignores are never committed. A failing `pre_tag` leaves what it wrote in the tree for you to clean up.

```toml
[release.hooks]
pre_tag = ["npm ci", "npm run build", "npm test"]
post_release = "./scripts/announce.sh"
```

### `[commit]`

| Key | Default | Description |
|---|---|---|
| `context` | | project guidance added to the commit-message prompt (text only) |
| `prompt` | | path, from the repo root, to a file that replaces the commit-message prompt; `context` is then ignored |
| `ai_command` | shared | override of the AI command for `commit` only; resolves `[commit]` → shared → default |
| `timeout` | shared, else `60` | override of the per-attempt AI deadline (seconds) for `commit` only; resolves `[commit]` → shared → default |
| `diff_exclude` | `[]` | pathspec globs kept out of the commit-message diff only, added to the shared `diff_exclude` list; release notes still read these paths |

Both verbs share the two-knob model: `context` *adds to* the default prompt; `prompt` *replaces* it.

### Upgrading

Two keys have gone, and naming either fails at load with a message saying what replaced it:

- `max_diff_lines` is replaced by `max_prompt_bytes`, a per-call size cap in bytes.
- `[release].fallback` is gone: every non-AI body is now the commit-subject list. Hand-written notes go in with `--notes-file`.

## The AI Transport

Mint owns the prompt; the command is only the transport. `ai_command` is any executable that reads a finished prompt on **stdin** and writes the message body to **stdout**:

```toml
ai_command = 'claude -p --model opus'          # a different model, without the lean flags
ai_command = 'llm -m gpt-4o'                   # any CLI with the same stdin/stdout contract
```

Both `ai_command` and `timeout` can be set shared (top-level) or per verb under `[release]` and `[commit]`, each resolving `[verb]` → shared → default.

Each call gets a per-attempt deadline: `timeout` seconds, default `180` for release and `60` for commit. Bad output (empty or whitespace-only, or a non-zero exit) is retried exactly once. A timeout or a missing AI tool is never retried, since the retry covers bad *content* only, and a Ctrl-C is a clean abort, never a retry. A failure then takes its verb's path: a release's notes go to [When the notes fail](#when-the-notes-fail) unless smaller parts wrote them (see [Big releases](#big-releases)); a commit opens your editor.

Setting `timeout = 0` turns the deadline off: the call runs **unbounded**. This is a deliberate, operator-chosen exception to mint's "fail loud, never hang" posture: you opt a slow command into an open-ended run and own that trade-off.

`ai_command` and `timeout` resolve independently, so overriding one doesn't touch the other. If you pin a slower model for a verb, raise that verb's `timeout` in the **same** `[release]` or `[commit]` table. Mint doesn't bump the timeout, warn, or require the pair, so a slow command left under its default (`180` for release, `60` for commit) will hit the deadline.

### The shipped default

With no `ai_command` set at any level, every AI call mint makes runs:

```text
claude -p --model sonnet --safe-mode --tools "" --strict-mcp-config --no-session-persistence --system-prompt "Follow the instructions you are given and output only what they ask for."
```

An `ai_command` you set replaces this command whole, lean flags included: to change only the model, copy it and edit `--model`.

It's Claude Code in lean form, so a call carries mint's prompt and nothing else:

- `--model sonnet` pins the model, by alias, so zero-config behaviour doesn't depend on the model your Claude CLI happens to default to.
- `--safe-mode` switches off CLAUDE.md files (the repo's and your global one), skills, plugins, hooks and MCP servers. Auth and model selection keep working.
- `--tools ""` drops the built-in tool definitions.
- `--strict-mcp-config` loads only MCP configuration passed on the command line, and none is passed.
- `--no-session-persistence` stops each one-shot call being saved to disk as a resumable session.
- `--system-prompt` replaces Claude Code's own system prompt. It names no task, because the same command serves every call; each call's own prompt carries its job.

`--bare` isn't used: it forces API-key auth and breaks the subscription login.

What the lean form drops, it drops on purpose. CLAUDE.md files and Claude Code's own system prompt no longer reach mint's calls, so instructions kept there that shaped release notes or commit messages stop doing so. Conventions you want applied belong in the `[release]` and `[commit]` `context` and `prompt` keys, which ride in every release-notes and commit-message call.

A Claude CLI too old for one of these flags rejects it as an unknown option and exits non-zero, which lands on the ordinary AI-failure path. Mint doesn't probe the version or fall back to a plainer command: upgrade claude, or set your own `ai_command`.

### Quoting in `ai_command`

Every `ai_command` (shared, `[release]`, `[commit]` and the shipped default) is split into a program name and its arguments by POSIX shell quoting rules. The program then runs directly; no shell is ever started.

- Unquoted whitespace, newlines included, separates words.
- Single quotes keep everything up to the next single quote literally.
- Double quotes keep everything up to the next unescaped double quote. Inside them, a backslash escapes `"`, `\`, `$` and the backtick, and is otherwise literal.
- Outside quotes, a backslash makes the next character literal.
- A backslash followed by a newline is removed, as a line continuation.
- Quoted and unquoted parts written together form one word: `--system-prompt="Be terse"` is the single argument `--system-prompt=Be terse`.
- A quoted empty string, `""` or `''`, is one empty argument.
- Nothing else is shell syntax. `$`, backticks, `*`, `~`, `#`, `|`, `;`, `&`, `<` and `>` are ordinary characters within a word: no variable expansion, command substitution, globbing, comments, pipes or redirection.

Write the value as a TOML literal (single-quoted) string so backslashes and double quotes reach the splitter as written. A multi-line literal (`'''…'''`) spreads a long command over lines:

```toml
ai_command = 'claude -p --model sonnet --tools "" --system-prompt "Be terse."'
ai_command = '''
claude -p \
  --model opus'''
```

A command with no quotes or backslashes splits exactly as it always has, on runs of whitespace. Every level is checked at load, whichever verb runs, and a value that can't be split fails there: `ai_command has an unterminated quote`, `ends in a lone backslash`, or `names no program`.

## Safety Model

- **Mutate nothing until accept.** `mint commit` works out the would-be-committed diff read-only; staging (`git add`) happens only after you accept. Declining is a true no-op.
- **Surgical unwind before the point of no return.** Everything `mint release` does before the atomic push is tracked. On any failure or abort before the push (Ctrl-C included), mint resets its own commits, deletes its tag and restores your autostash, never touching your work, and reports exactly what it undid. The one exception is the exclusion check's answers: once recorded, their commit stays on the branch, re-committed on the starting commit, and the unwind says it kept it; where git ignores an untracked `.mint.toml`, their uncommitted write is left in place.
- **One atomic push.** Branch and tag go up in a single `git push --atomic`; there's no moment where the tag exists without its commit.
- **Never unwind after the push.** A failed GitHub release or `post_release` hook only warns, and the release stands. A failed `mint commit -p` push keeps the commit.
- **Lock-resilient mutations.** Every `git` mutation retries a held `.git` lock and clears provably stale ones.
- **Fail loud, never hang.** A question with no terminal to ask at and no `-y` fails with one clear line telling you to pass `-y`. Unattended runs with no message to commit abort.

## Output

Mint renders styled output at a terminal and plain, byte-pure lines when piped (or under `--plain`). Failures and warnings are also written to stderr for scripting. Exit codes: `0` success, `1` runtime failure or abort, `2` usage error.

## Contributing

`CLAUDE.md` has the house rules. Every change passes the gates:

```bash
go build ./...
gofmt -l .            # must print nothing
go vet ./...
go test -race ./...
golangci-lint run     # must report 0 issues
```

Tests never run real `git`, `claude` or editors: subprocesses are scripted through the runner's fake, and output is recorded through the presenter's.

## License

MIT
