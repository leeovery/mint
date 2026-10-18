# Specification: Token Aware Diff Sizing

## Specification

### 1. Purpose and Scope

#### 1.1 The problem

Mint writes release notes by sending a composed prompt to the configured AI command: instructions, the Change Map, the post-exclusion diff and a closing reminder. Big releases fail in two ways as built:

- **A dead end.** When a post-exclusion diff is over `max_diff_lines` (default 50,000 lines), the release aborts under the default `on_notes_failure = abort` with `diff too large`, and nothing points to a way forward.
- **The wrong measure.** A line count says little about what the AI receives, because one minified or generated line can be any size. A diff under the ceiling can still overflow the model. That failure renders as `AI returned empty/invalid notes after retry`, which misnames a size overflow.

The goal is AI-written notes for a release of any size. For the releases that still cannot get them, the run ends somewhere useful: choices at a terminal, guidance everywhere else.

#### 1.2 The premise: mint cannot predict fit

Whether a prompt fits depends on three things mint cannot see:

- the model's context window
- its tokenizer
- the AI tool's own per-call overhead: its system prompt, tool definitions and attachments.

That overhead can dwarf mint's own prompt. In one observed failure the request was ~1,335,248 tokens against a 1,000,000 limit, and mint's prompt was only ~532,481 of them. About 803k tokens belonged to the tool itself.

So mint estimates no tokens and keeps no model registry. Instead it does three things:

- caps the size of every call it makes, as a size policy rather than a fit prediction (§4)
- trims the shipped default command's overhead at source (§2)
- reacts to the AI's actual outcome when a call fails (§9).

#### 1.3 The consumers

| Consumer | Its share of this feature |
|---|---|
| `mint release` | All of it. |
| `mint release regenerate <version>`, fresh source | The size cap, chunking, failure classification, halving, the floor's choices, and `--notes-file` as a notes source. No exclusion check. |
| `mint release regenerate --all` | The size cap, chunking, failure classification and halving, per version. Skip-and-continue stays only as its floor and names the real cause. No exclusion check, no `--notes-file`. |
| `mint commit` | The lean default command, quote-aware parsing, the size cap (over it, the message is written in `$EDITOR` as today), a `[commit]` exclusion level, and the AI's verbatim output on an AI failure. No chunking, failure classification, exclusion check or message file. |

Regenerate rides on `[release]` config throughout, as built. Its `tag` and `release` sources run no AI and are unchanged.

#### 1.4 Out of scope

- Token estimation, an operator-set token budget and a model-window registry: §1.2 rules them out.
- Failure codes beyond SIZE and OTHER, including a rate-limit wait-and-retry. The code set extends additively if API-key setups hit throttling mid-wave (§9).
- Per-provider knowledge in mint: no string-matching of provider error messages, no probing of CLI versions, no fallback to an older command form.
- Three items parked on the roadmap:
  - headline-first release notes, a format that states what a release is about
  - agent-authored release notes, with mint handing an outer agent its instructions and Change Map
  - an agent guide for running releases.

### 2. The Lean Default AI Command

#### 2.1 The command

The shipped default `ai_command` becomes:

```text
claude -p --model sonnet --safe-mode --tools "" --strict-mcp-config --no-session-persistence --system-prompt "Follow the instructions you are given and output only what they ask for."
```

- `--safe-mode` switches off CLAUDE.md files (the repo's and the user's global), skills, plugins, hooks and MCP servers. Auth and model selection keep working.
- `--tools ""` drops the built-in tool definitions.
- `--strict-mcp-config` loads only the MCP configuration passed on the command line, and none is passed.
- `--no-session-persistence` stops claude saving each one-shot call to disk as a resumable session.
- `--system-prompt` replaces Claude Code's own system prompt.

The model is unchanged: Sonnet, in alias form.

The empty `--tools` argument and the multi-word system prompt reach claude intact only through quote-aware parsing (§3).

Measured overhead, on the same one-word prompt with `usage` read from `--output-format json`:

- The plain form: `echo "Reply with the single word: ok" | claude -p --model sonnet --output-format json --no-session-persistence` → 105,994 input tokens (2026-10-18).
- The lean form, final wording: `echo "Reply with the single word: ok" | claude -p --model sonnet --safe-mode --tools "" --strict-mcp-config --no-session-persistence --system-prompt "Follow the instructions you are given and output only what they ask for." --output-format json` → 628 input tokens, answered `ok` on the subscription login (2026-10-18).

`--bare` is not used: it forces API-key auth and breaks the subscription login (`Not logged in · Please run /login`).

#### 2.2 One command for every call

The system prompt names no task, because the same command serves every AI call mint makes:

- the release-notes calls: single-pass, chunk and merge
- the failure classifier
- the exclusion check
- `mint commit`.

Each call's own prompt carries its job.

#### 2.3 What it drops, on purpose

CLAUDE.md files and Claude Code's own system prompt no longer reach mint's calls. Instructions kept there that shaped release notes or commit messages stop doing so. This is intended: mint owns the prompt and the command is only transport, so a call carries mint's instructions and nothing else. Conventions an operator wants applied belong in the `[release]` and `[commit]` `context` and `prompt` keys, which ride in every call. The change is documented alongside the lean default (§18).

#### 2.4 Reach

The lean command applies wherever `ai_command` resolution falls through to the shipped default. An explicit `ai_command` at any level is untouched.

#### 2.5 A claude too old for the flags

A claude version that predates a flag rejects it as an unknown option and exits non-zero. That lands on the normal generation-failure path:

- The failure classifier runs the same command and fails the same way, so the release takes the OTHER path (§9) with claude's own message rendered verbatim.
- On `mint commit`, the failure opens the editor and carries that message verbatim (§17).

mint adds no version probe and no fallback to the plain command. Either would put claude-specific knowledge into mint.

### 3. Quote-Aware `ai_command` Parsing

#### 3.1 The rules

mint splits every `ai_command` into a program name and its arguments by POSIX shell quoting rules. That covers the top level, `[release]`, `[commit]` and the shipped default:

- Unquoted whitespace separates words.
- Single quotes keep everything up to the next single quote literally.
- Double quotes keep everything up to the next unescaped double quote. Inside them, a backslash escapes `"`, `\`, `$` and the backtick, and is otherwise literal.
- Outside quotes, a backslash makes the next character literal.
- A backslash followed by a newline is removed, as a line continuation.
- Quoted and unquoted parts written together form one word: `--system-prompt="Be terse"` is the single argument `--system-prompt=Be terse`.
- A quoted empty string, `""` or `''`, is one empty argument.
- Nothing else is shell syntax. `$`, backticks, `*`, `~`, `|`, `;`, `&`, `<` and `>` are ordinary characters within a word: no variable expansion, globbing, pipes or redirection.

mint still runs the program directly through the runner seam, never through a shell.

#### 3.2 Malformed commands fail at load

An `ai_command` at any level that cannot be split is rejected when config loads, before any step runs. Three cases count as malformed:

- an unterminated quote
- a trailing lone backslash
- an empty first word.

The error takes the existing load-error shape and names the key, as `on_notes_failure` and hook values are rejected today:

- `invalid .mint.toml: release.ai_command has an unterminated quote`
- `invalid .mint.toml: ai_command ends in a lone backslash`
- `invalid .mint.toml: commit.ai_command names no program`

A malformed command is never left to fail at call time. There it would read as an AI failure, reach the failure classifier (which runs the same command), take the OTHER path, and end in a non-AI body or the editor.

#### 3.3 Compatibility

Quotes never worked, so a working `ai_command` today contains none, and its words are unchanged under the new rules. Only a backslash in an existing command could change meaning.

None of the user's repos sets `ai_command` (`grep -HE '^[[:space:]]*ai_command[[:space:]]*=' ~/Code/*/.mint.toml` → no output).

The code comments that describe a whitespace split are rewritten to the new rule, in the same change. They sit on the transport's command parsing and on config's `ai_command` field.

### 4. The Per-Call Size Cap

#### 4.1 The key

`max_prompt_bytes` replaces `max_diff_lines`:

- an integer count of bytes, defaulting to 1,500,000
- at the shared top level only, so one cap covers both verbs.

It bounds the composed prompt mint sends in any single AI call, measured in bytes: instructions, Change Map, diff and closing reminder together. The AI tool's own overhead is invisible to mint and sits outside the cap. No call mint makes is ever bigger than the cap.

As a new key it takes the strict-config path: a schema entry, a friendly type error, a config-metadata row and the README tables.

#### 4.2 What the cap does

The cap is a size policy, not a fit prediction. A prompt under it can still overflow the model; that is the failure classifier's case (§9) and halving's (§10).

It has three jobs:

- **It decides when the single call is skipped.** Release notes whose single-pass prompt would exceed the cap go straight to chunking. Under the cap, mint makes the single call.
- **It sizes every chunk call** (§8).
- **It guards where reactive detection is blind.** claude refuses stdin over 10 MB with a separate error. Some providers silently truncate an oversized prompt and return plausible output, so no failure ever reaches the classifier.

#### 4.3 Per consumer

- **`mint release`, and `regenerate <version>` from a fresh source.** A single-pass prompt over the cap goes straight to chunking (§7, §8).
- **`regenerate --all`.** The same, per version. Skip-and-continue remains only as its floor (§12).
- **`mint commit`.** A composed prompt over the cap opens `$EDITOR` as today, with the existing `diff too large to summarise — opening editor` note. Commit has no chunking.

#### 4.4 The release timeout

The cap's default and the release timeout's default are chosen together, so that a call at the cap finishes well inside the timeout.

`mint release` gets its own shipped per-attempt timeout default of 180 seconds. It resolves `[release].timeout` → shared `timeout` → 180s, so an explicit value at either level still wins. Regenerate rides on it, as on all `[release]` config. `mint commit` keeps the shipped 60s, because its calls stay small and a hung one should not take minutes to fail.

#### 4.5 The default's grounds

Measured with the lean command on 120,000 bytes of Go diff, with a one-word ask appended and `--output-format json` (2026-10-18): `git log --all -p --no-color -- '*.go' | head -c 120000` → 54,654 input tokens, 2.2s API time.

That is about 2.2 bytes per token, so 1,500,000 bytes is about 675k tokens. It stays under a 1,000,000-token window even at about 1.8 bytes per token for denser content, and far below claude's 10 MB stdin limit. Reading the prompt is fast enough that a call's time is mostly the notes being written. A call at the cap should take about a minute.

The default leans high because the two ways of getting it wrong differ:

- **Too high for the model is loud and corrects itself.** claude rejects an oversized prompt instantly, before sending it. mint halves and still writes the notes, then hints that the cap should come down (§10). An operator on a smaller window lowers the cap.
- **Too low is silent.** A release one call could have read whole gets chunked and merged notes instead.

#### 4.6 Removing `max_diff_lines`

Under strict config, a `.mint.toml` still setting `max_diff_lines` fails at load. Like the `[hooks]` placement error, it gets a specific message naming its replacement: `invalid .mint.toml: max_diff_lines has been replaced by max_prompt_bytes, a per-call size cap in bytes`.

mint has no users outside the user's own repos. One of them still sets the key, and that line comes out before it upgrades: `grep -lE '^[[:space:]]*max_diff_lines[[:space:]]*=' ~/Code/*/.mint.toml` → `/Users/leeovery/Code/agentic-workflows/.mint.toml`.

### 5. Per-Verb Exclusion Lists

#### 5.1 Three levels of `diff_exclude`

`diff_exclude` gains `[release]` and `[commit]` levels beside the shared top-level list. Each verb's AI diff excludes the shared list plus its own level's list:

| Diff | Excludes |
|---|---|
| Release notes (forward release, regenerate's fresh source) | the built-in always-excludes (§5.3), shared `diff_exclude`, `[release].diff_exclude` |
| `mint commit`'s AI diff | shared `diff_exclude`, `[commit].diff_exclude` |

The lists add, unlike `ai_command` and `timeout`, where a per-verb value overrides the shared one. The shared list holds what neither verb should read, such as lockfiles and build output, and a per-verb list should not have to repeat it.

The split exists because the two verbs serve different readers:

- Release notes are a public list of what a release holds.
- A commit message is for the developers and agents reading that piece of work, and should cover its tests and specs.

With one shared list, hiding a test or spec folder from release notes also hid it from commit messages.

As built, the following continue to hold:

- Entries at every level are git pathspec globs, and git does the matching.
- The Change Map is built over the same post-exclusion set as the diff, so an excluded path never appears in it.
- `mint commit`'s preflight ignores `diff_exclude` at every level. A commit whose every file is excluded still commits, with its message written in the editor.

The new keys take the strict-config path: schema entries, friendly type errors, config-metadata rows and README tables.

#### 5.2 The keep list

`[release].diff_keep` is an array of globs in the same pathspec form, default empty. A path it covers is never raised by the exclusion check (§6). It does nothing else: it adds nothing to the diff and never overrides an exclusion. It takes the strict-config path.

#### 5.3 Built-in always-excludes

`.mint.toml` joins `CHANGELOG.md` as a built-in, non-configurable always-exclude from every release-notes diff. That holds for forward releases and regenerate alike, since exclusion is path-based.

A change to mint's config is about how releases are made, not what a release holds. The exclusion check commits `.mint.toml` inside the release it answered for (§6.6). Without the exclude, that change would put a tooling line into the release's notes, and into any later regenerate of that version.

The plain-mode `version_file` exclusion is unchanged. `mint commit`'s AI diff has no built-in excludes, as built, so a config change still reaches its commit message.

### 6. The Exclusion Check

Before a release's notes are written, mint asks the AI which changed paths look like they do not belong in release notes. At a terminal it then asks the operator about each one.

It exists to catch slips that have already shipped: design files and test changes documented in notes they were never meant for. Those slips happened on normal-sized releases, so the check runs on every release whose notes the AI writes, not only big ones.

#### 6.1 When it runs

The check runs on `mint release`, before the size check, which measures what remains (§7). It is skipped in these cases:

- under `--no-ai`
- with `--notes-file` (§13)
- on a first release
- when no path is left to raise: an empty post-exclusion diff, or every changed path already covered by an exclusion or the keep list.

`mint release regenerate` never runs it, for any source, single version or `--all`. Regenerate applies the `[release]` exclusion and keep lists as they stand, so answers given on forward releases carry into regenerated notes. A check there would have it edit and commit `.mint.toml` mid-run, and could pause `--all` at every version. A regenerate that writes only the provider release would otherwise need no clean tree and commit nothing. Accepted cost: noise in an old version that was never asked about is not flagged.

`mint commit` gets no check either. A commit message should describe the tests and specs in the work, and a check there would be asking a different question, closer to "did you mean to stage this?".

#### 6.2 The call

The check is one AI call through the existing transport, with the release command and timeout. It carries no diff, only the release's changed-path list. Each entry is a path not already covered by the built-in always-excludes, the shared or `[release]` `diff_exclude`, or `diff_keep`, listed with its change status and the size of its diff. Big files are listed like any other path; the check passes no separate verdict on them.

When the list would take more than a tenth of the cap, it condenses by folder (§6.3), for example to `vendor/: 4,312 files, 38 MB`. The AI suggests folder-level globs in the main, so a folder-grouped list keeps what it needs. Skipping the check would lose it on exactly the releases where flagging a folder matters most, and splitting the list across calls would split the judgment. A list that cannot get under the cap even condensed counts as a failed check (§6.8).

#### 6.3 Condensing by folder

Two texts condense by folder: the check's file list, and the Change Map (§8). The rule is the same for both:

- A text condenses once it would take more than a tenth of the size cap, which is 150,000 bytes at the default.
- Condensing replaces a long run of entries under one folder with one summary line for that folder, until the text is under its share.
- A text under its share renders exactly as it does today.

The result must be deterministic for the same input and must collapse the largest runs first. Which folders collapse, and in what order, is otherwise the implementer's.

#### 6.4 Reading the answer

The AI returns suggested globs, each with a short reason, or an explicit "none".

- mint keeps a suggestion only if it matches at least one uncovered changed path, matched the way git matches `diff_exclude`.
- An answer that is neither "none" nor yields one usable suggestion is a failed check (§6.8).

The AI never excludes anything itself.

#### 6.5 At a terminal: one pause per suggestion

Each surviving suggestion gets a pause through the presenter's gate seam, in order. A suggestion is skipped when earlier answers in this run already cover everything it matches. The pause shows:

- the glob and the AI's reason
- how many changed files it matches, and their total size
- any of those files on the keep list.

It offers two choices:

- **Exclude:** the glob joins `[release].diff_exclude`, and this release's notes are written without those files.
- **Keep:** the glob joins `[release].diff_keep` and is never raised again.

The default is keep, so no file leaves the notes without the operator choosing it.

#### 6.6 Recording the answers

Answers are held until the last pause, then written together into `.mint.toml`'s `[release]` table. The entry is appended to an existing key, the key is added to an existing table, the table is added, or the file is created holding only that table, as needed. Everything else in the file is left as written: other keys, comments and formatting. Answers go to `[release]` only, never the shared list, so commit messages still see those files.

mint then stages `.mint.toml` by path and commits it through the mutator, with the fixed subject `chore(release): release-notes path answers for {tag}`. The prefix is constant, like the pre-tag artifact commit's, not `commit_prefix`. The commit sits on the release branch and goes out with the release's push.

Answers live in `.mint.toml` rather than in state on one machine, so a teammate, CI or an agent session elsewhere is not asked the same question again.

- **Ctrl-C during the pauses** cancels the run and records nothing.
- **Abort or failure after answering:** the answers commit stays. The answer is about the repo, not this release, and unwinding it would bring the same question back next time. This is the one deliberate exception to the surgical unwind. A failure before the push unwinds as built, back to the starting commit and taking the pre-tag artifact commit with it. The unwind then re-commits the answers on top of the starting commit.
- **Everything excluded:** when the answers leave no path in the diff, the release gets the maintenance-release body, as any empty post-exclusion diff does.
- **Commit-subject bodies:** the commit-subject list (§14) leaves out subjects starting `chore(release): release-notes path answers for `. So the floor's `c` and `fallback` never list mint's own answers commit, on the forward run or on a later regenerate.
- **`--autostash`:** the answers are written to the committed `.mint.toml`. A stashed edit to the same file that conflicts on restore leaves the stash entry intact, as any conflicting restore does.

#### 6.7 Unattended and dry runs

- **Unattended** (`-y`, non-TTY, an agent run): no pause, and nothing is written. After the notes, each suggestion prints with its reason, what it matches, and the `[release].diff_exclude` line that would exclude it, for whoever reads the output to act on. A flagged file stays in the diff and is read like any other (§8).
- **Dry run:** the suggestions show as they would at the pause; nothing is asked or written.

#### 6.8 When the check fails

The check counts as failed in five cases:

- a generation failure
- a timeout
- a missing binary
- an answer mint cannot read (§6.4)
- a list that will not fit (§6.2).

In each case the release carries on with no suggestions. mint prints one warning line, `couldn't check for files that don't belong in the notes`, with the AI's captured output verbatim beneath it where there is any. Without that line, "nothing looked out of place" and "the check never ran" would look the same, and that is how a design file slips into the notes unnoticed.

Any real AI problem surfaces at the notes call, which runs the same command through the normal failure path (§9). The failure classifier never runs for the check itself. A Ctrl-C during the check cancels the run.

---

## Working Notes
