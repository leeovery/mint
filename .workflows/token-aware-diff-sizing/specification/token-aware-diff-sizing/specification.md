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

That overhead is real and invisible to mint: the plain `claude -p` form carries 105,994 input tokens before mint's prompt begins (§2.1). The tool cannot predict fit either. claude's own local estimate counts about 4 bytes a token, while diff content runs about 2.2 (§4.5), so claude's view of a prompt can fall half short of what the API counts. In one observed failure claude reported the request at ~1,335,248 tokens against a 1,000,000 limit, with only ~532,481 of them the conversation and the rest put down to system prompt, tools and attachments. Most of that rest was the estimate's gap, not overhead. The same split on a diff through the lean command (§2.1), whose overhead is 628 tokens, still puts ~514k tokens on overhead: `{ git log --all -p --no-color -- '*.go' | head -c 2600000; printf '\n\nReply with the single word: ok\n'; } | claude -p --model sonnet --safe-mode --tools "" --strict-mcp-config --no-session-persistence --system-prompt "Follow the instructions you are given and output only what they ask for." --output-format json` → `"api_error_status":400`, `"terminal_reason":"prompt_too_long"`, `"duration_ms":1586`, `"total_cost_usd":0`, result `Prompt is too long · the request is ~1162407 tokens (limit 1000000) but this conversation is only ~648221 tokens — the rest is system prompt, tool definitions, and attachment content.` (2026-10-18).

So mint estimates no tokens and keeps no model registry. Instead it does three things:

- caps the size of every call it makes, as a size policy rather than a fit prediction (§4)
- trims the shipped default command's overhead at source (§2)
- reacts to the AI's actual outcome when a call fails (§9).

#### 1.3 The consumers

| Consumer | Its share of this feature |
|---|---|
| `mint release` | All of it. |
| `mint release regenerate <version>`, fresh source | The size cap, chunking, failure classification, halving, the floor's choices, and `--notes-file` as a notes source. No exclusion check. |
| `mint release regenerate --all` | The size cap, chunking, failure classification and halving, per version. Skip-and-continue stays only as its floor and names the real cause, with the AI's verbatim output. No exclusion check, no `--notes-file`. |
| `mint commit` | The lean default command, quote-aware parsing, the size cap (over it, the message is written in `$EDITOR` as today), a `[commit]` exclusion level, and the AI's verbatim output on an AI failure. No chunking, failure classification, exclusion check or message file. |

Regenerate rides on `[release]` config throughout, as built, except `on_notes_failure`, which it never reads (§12.3). Its `tag` and `release` sources run no AI and are unchanged.

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

CLAUDE.md files and Claude Code's own system prompt no longer reach mint's calls. Instructions kept there that shaped release notes or commit messages stop doing so. This is intended: mint owns the prompt and the command is only transport, so a call carries mint's instructions and nothing else. Conventions an operator wants applied belong in the `[release]` and `[commit]` `context` and `prompt` keys, which ride in every release-notes and commit-message call. The change is documented alongside the lean default (§18).

#### 2.4 Reach

The lean command applies wherever `ai_command` resolution falls through to the shipped default. An explicit `ai_command` at any level is untouched.

#### 2.5 A claude too old for the flags

A claude version that predates a flag rejects it as an unknown option and exits non-zero. That lands on the normal generation-failure path:

- The failure classifier runs the same command and fails the same way, so the release takes the OTHER path (§9) with claude's own message rendered verbatim.
- On `mint commit`, the failure carries that message verbatim, in the warning before the editor opens or in an unattended run's failure (§17).

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

A blank or whitespace-only `ai_command` is not malformed. As built, it counts as unset and resolution falls through to the next level (§2.4), and that stays. "Names no program" covers a non-blank value whose split yields no words or an empty first word, such as `'' --flag`.

A malformed command is never left to fail at call time. There it would read as an AI failure, reach the failure classifier (which runs the same command), take the OTHER path, and end in a non-AI body or the editor.

#### 3.3 Compatibility

Quotes never worked, so a working `ai_command` today contains none, and its words are unchanged under the new rules. Only a backslash in an existing command could change meaning.

None of the user's repos sets `ai_command` (`grep -HE '^[[:space:]]*ai_command[[:space:]]*=' ~/Code/*/.mint.toml` → no output).

The code comments that describe a whitespace split are rewritten to the new rule, in the same change. They sit on the transport's command parsing (the `ai.Config` command field, `Generate` and `parseCommand`), on config's `ai_command` field and its `AICommandFor` accessor, and on `aitransport.New`: `grep -rn -i 'whitespace-split\|whitespace split\|by whitespace\|strings.Fields split' internal/ai internal/config internal/aitransport --include='*.go' | grep -v _test.go` → `internal/ai/transport.go:92`, `:171`, `:306`; `internal/config/config.go:129`, `:535`; `internal/aitransport/aitransport.go:30` (2026-10-18).

### 4. The Per-Call Size Cap

#### 4.1 The key

`max_prompt_bytes` replaces `max_diff_lines`:

- an integer count of bytes, defaulting to 1,500,000
- at the shared top level only, so one cap covers both verbs.

It bounds the composed prompt mint sends in any single AI call, measured in bytes: instructions, Change Map, diff and closing reminder together. The AI tool's own overhead is invisible to mint and sits outside the cap. No call mint makes is ever bigger than the cap.

As a new key it takes the strict-config path: a schema entry, a friendly type error, a config-metadata row and the README tables.

A value of zero or less is rejected when config loads, before any step runs, in the same load-error shape as a malformed `ai_command`: `invalid .mint.toml: max_prompt_bytes must be a positive number of bytes`. Zero never means "no cap", the way `timeout = 0` means no deadline: the cap sizes every chunk and keeps every call under claude's stdin limit. An operator who wants a cap mint never reaches sets a large one.

#### 4.2 What the cap does

The cap is a size policy, not a fit prediction. A prompt under it can still overflow the model; that is the failure classifier's case (§9) and halving's (§10).

It has three jobs:

- **It decides when the single call is skipped.** Release notes whose single-pass prompt would exceed the cap go straight to chunking. Under the cap, mint makes the single call.
- **It sizes every chunk call** (§8).
- **It guards where reactive detection is blind.** claude refuses stdin over 10 MB with a separate error. Some providers silently truncate an oversized prompt and return plausible output, so no failure ever reaches the classifier.

#### 4.3 Per consumer

- **`mint release`, and `regenerate <version>` from a fresh source.** A single-pass prompt over the cap goes straight to chunking (§7, §8).
- **`regenerate --all`.** The same, per version. Skip-and-continue remains only as its floor (§12).
- **`mint commit`.** A composed prompt over the cap opens `$EDITOR` as today, with the existing `diff too large to summarise — opening editor` note. Commit has no chunking. The same holds for `r` at commit's review gate: a nudge that pushes the regeneration's composed prompt over the cap sends no call, and the editor opens with that note, empty, as it does after a failed regeneration.

#### 4.4 The release timeout

The cap's default and the release timeout's default are chosen together, so that a call at the cap finishes well inside the timeout.

`mint release` gets its own shipped per-attempt timeout default of 180 seconds. It resolves `[release].timeout` → shared `timeout` → 180s, so an explicit value at either level still wins. Regenerate rides on it, as it does on `[release].ai_command`. `mint commit` keeps the shipped 60s, because its calls stay small and a hung one should not take minutes to fail.

#### 4.5 The default's grounds

Measured with the lean command on 120,000 bytes of Go diff, with a one-word ask appended and `--output-format json` (2026-10-18): `git log --all -p --no-color -- '*.go' | head -c 120000` → 54,654 input tokens, 2.2s API time.

That is about 2.2 bytes per token, so 1,500,000 bytes is about 675k tokens. It stays under a 1,000,000-token window even at about 1.8 bytes per token for denser content, and far below claude's 10 MB stdin limit. Reading the prompt is fast enough that a call's time is mostly the notes being written. A call at the cap should take about a minute.

The default leans high because the two ways of getting it wrong differ:

- **Too high for the model is loud and corrects itself.** claude uploads the prompt and the API refuses it with a 400 within a couple of seconds, at no token cost (§9.5). mint halves and still writes the notes, then hints that the cap should come down (§10). An operator on a smaller window lowers the cap.
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

The check is one AI call through the existing transport, with the release command and timeout. It carries no diff, only the release's changed-path list. Each entry is a path not already covered by the built-in always-excludes, the plain-mode `version_file` exclusion (§5.3), the shared or `[release]` `diff_exclude`, or `diff_keep`, listed with its change status and the size of its diff. These are the uncovered changed paths §6.4 and §6.5 refer to. Big files are listed like any other path; the check passes no separate verdict on them.

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

Each surviving suggestion gets a pause through the presenter's gate seam, in order. A suggestion is skipped when earlier answers in this run already cover every uncovered changed path it matches, so a suggestion whose only remaining matches are on the keep list raises no pause (§5.2). The pause shows:

- the glob and the AI's reason
- how many changed files it matches, and the total size of their diffs, each measured as §6.2 measures a path. The files counted are the changed paths this release's notes diff still carries at that pause: a path the built-in always-excludes, either `diff_exclude` level or an earlier Exclude answer in this run has taken out is not counted, and a path on the keep list is
- any of those files on the keep list, a glob an earlier Keep answer in this run gave included.

It offers two choices, each picked by its initial letter, as the floor's are (§12.2):

- **`e`, Exclude:** the glob joins `[release].diff_exclude`, and this release's notes are written without those files.
- **`k`, Keep:** the glob joins `[release].diff_keep` and is never raised again.

The default is keep, which a bare Enter picks, so no file leaves the notes without the operator choosing it.

#### 6.6 Recording the answers

Answers are held until the last pause, then written together into `.mint.toml`'s `[release]` table. The entry is appended to an existing key, the key is added to an existing table, the table is added, or the file is created holding only that table, as needed. A `[release]` table written inline (`release = { … }`) or with dotted keys (`release.version_file = …`) takes the answer in that form, inside the existing table, since TOML allows the table no second definition. Everything else in the file is left as written: other keys, comments and formatting. Answers go to `[release]` only, never the shared list, so commit messages still see those files.

mint then stages `.mint.toml` by path and commits it through the mutator, with the fixed subject `chore(release): release-notes path answers for {tag}`. The prefix is constant, like the pre-tag artifact commit's, not `commit_prefix`. The commit sits on the release branch and goes out with the release's push. The recording is narrated: once the answers commit lands, one line says the answers were written to `.mint.toml`'s `[release]` table and committed, a presenter event shown before the notes line resumes (§16.1), and plain mode prints it as its own line. Its wording is the implementer's.

Answers live in `.mint.toml` rather than in state on one machine, so a teammate, CI or an agent session elsewhere is not asked the same question again.

- **Ctrl-C during the pauses** cancels the run and records nothing.
- **Stdin closing at a pause** aborts the run before any tag, unwinding as any failure before the push does, and records nothing, since answers are written only once the last pause is answered. It is never read as the default, keep.
- **Abort or failure after answering:** the answers commit stays. The answer is about the repo, not this release, and unwinding it would bring the same question back next time. This is the one deliberate exception to the surgical unwind. A failure before the push unwinds as built, back to the starting commit and taking the pre-tag artifact commit with it. The unwind then re-commits the answers on top of the starting commit. The unwind's narration names the answers commit as kept, so the commit left at the top of the branch does not read as an unwind that failed.
- **The answers write or commit failing:** a failure writing `.mint.toml`, or staging or committing it, is a failure before the push. It is surfaced and routed through the unwind as a failed pre-tag artifact commit is, with the write error, or git's own stderr, verbatim beneath its line, and the run aborts before any tag, unwinding as built. No answers commit was made, so none is kept: the unwind leaves `.mint.toml` as it was before the answers were written, and nothing is recorded.
- **The unwind's re-commit failing:** one warning names the manual step that restores the answers, as a failed reset or tag delete during the unwind names its manual step, with git's own stderr verbatim beneath it. The unwind's narration then does not name the answers commit as kept, and its summary reports manual cleanup required rather than a clean repo.
- **The unwind's reset failing:** the answers commit is still on the branch, above the commits the reset could not remove, so no re-commit is attempted. The warning that names the reset as the manual step, as built, names after it the step that restores the answers, since that reset takes the answers commit with it. The unwind's narration does not name the answers commit as kept, and its summary reports manual cleanup required, as built.
- **Everything excluded:** when the answers leave no path in the diff, the release gets the maintenance-release body, as any empty post-exclusion diff does.
- **Commit-subject bodies:** the commit-subject list (§14) leaves out subjects starting `chore(release): release-notes path answers for `. So the floor's `c` and `fallback` never list mint's own answers commit, and neither does `c` on a later regenerate.
- **`--autostash`:** the answers are written to the committed `.mint.toml`. A stashed edit to the same file that conflicts on restore leaves the stash entry intact, as any conflicting restore does.

#### 6.7 Unattended and dry runs

- **Unattended** (`-y`, non-TTY, an agent run): no pause, and nothing is written. After the notes, each suggestion prints with its reason, what it matches as the pause would show it (how many changed files and the total size of their diffs, and any of those files on the keep list, §6.5), and the `[release].diff_exclude` line that would exclude it, for whoever reads the output to act on. That line is the key as an Exclude answer would leave it (§6.6): `diff_exclude = [...]` under `[release]`, holding the entries the key holds now with the suggested glob appended, or the glob alone where the key is unset. Each suggestion's line is worked out against the config as loaded, so it never carries another suggestion's glob. A flagged file stays in the diff and is read like any other (§8). They print whatever the notes' outcome: after the commit-subject list when `on_notes_failure = "fallback"` ships it, and with the next moves when the run stops at the floor (§12.4), where the reader is about to act.
- **Dry run:** nothing is asked or written. The suggestions follow the dry run's own mode, as the real run it previews would show them: a dry run at a terminal without `-y` shows them as they would appear at the pause, and an unattended dry run (`-y` or non-TTY) prints them after the notes as an unattended run does, the `[release].diff_exclude` line included.

#### 6.8 When the check fails

The check counts as failed in five cases:

- a generation failure
- a timeout
- a missing binary
- an answer mint cannot read (§6.4)
- a list that will not fit (§6.2).

In each case the release carries on with no suggestions. mint prints one warning line, `couldn't check for files that don't belong in the notes`, with the AI's captured output verbatim beneath it where there is any. Without that line, "nothing looked out of place" and "the check never ran" would look the same, and that is how a design file slips into the notes unnoticed.

Any real AI problem surfaces at the notes call, which runs the same command through the normal failure path (§9). The failure classifier never runs for the check itself. A Ctrl-C during the check cancels the run.

### 7. Routing a Release's Notes

#### 7.1 Where the body comes from

The first match wins:

1. **`--notes-file`** (§13): the supplied body. No AI, no exclusion check, no cap.
2. **A first release:** `Initial release.`, as built.
3. **An empty post-exclusion diff:** the maintenance-release body, as built.
4. **`--no-ai`:** the commit-subject list (§14).
5. **The AI path** (§7.2).

#### 7.2 The AI path, in order

1. **The exclusion check**, on forward releases only (§6). Exclusions it records change the diff that the later steps see. If they leave the diff empty, the maintenance-release body applies (§6.6).
2. **Cache reuse.** A real run that follows a dry run offers the cached notes, as built (§15).
3. **The size check.** The composed single-pass prompt is measured against `max_prompt_bytes`. It holds the instructions, the Change Map (condensed if it would take more than its share, §6.3), the diff and the closing reminder.
4. **Under the cap: one single-pass call**, as built. If it fails, the failure classifier decides what happens next (§9). SIZE halves (§10); anything else falls to the floor (§12).
5. **Over the cap: chunked generation** (§8). The single-pass call is skipped.

The size check comes after the exclusion check, so it measures the diff the exclusions leave. Take a release over the cap only because of a committed 1.9 MB transcript: once the operator excludes the transcript at the pause, the release is measured under the cap and gets an ordinary single-pass call.

Regenerate's fresh source follows the same path without steps 1 and 2, per version under `--all`. It runs no exclusion check, and, as built, it neither offers notes from the dry-run cache nor writes to it: the cache carries a release's dry-run preview to the real run that follows, and a regenerate is asked for new notes on a version already out.

### 8. Chunked Generation

#### 8.1 The shape: map-reduce

Chunk calls run in parallel, then exactly one merge call produces the final notes. Refine is out: sequential calls each handed the notes so far are serial by construction and biased toward early chunks, and the steer is parallel first, never a chain of serial AI calls. The merge is the one serial step. Concatenating the chunk notes is neither the plan nor a fallback (§11).

#### 8.2 The Change Map in every call

Every chunk call and the merge call receive the same whole-release Change Map:

- Each chunk knows the whole release's shape while reading only its slice.
- The merge can rank across all of it.

The default prompt's salience rules depend on the map: one bullet per notable change, related changes combined, ranked using the Change Map.

A release that deletes a vendored tree or moves thousands of files gets one `Removed:` or `Renamed:` line per path, enough to crowd out any diff. So when the Change Map would take more than its share of the cap (§6.3), its long runs of removed or renamed files condense by folder, for example to `Removed: 4,312 files under vendor/`. The notes still learn the tree went, which is all they would say about it anyway.

This applies to every call that carries the map, single-pass included. A map that fits renders exactly as today.

#### 8.3 Packing chunks

A chunk call's room for diff is the cap minus its fixed part: the instructions, Change Map, chunk framing and closing reminder.

Files are packed in path order. That keeps each area's files together (the Change Map's area is a path's leading directory), and each folder's. Each chunk is filled until the next file's diff would not fit in its room.

#### 8.4 A file too big for one call: pieces

A file whose diff alone exceeds a chunk call's room is split into pieces. Each piece is a chunk call of its own, sent in parallel with the others and stitched by the merge like any other chunk notes.

- Cuts fall at hunk boundaries, else at line boundaries. A single line longer than the room, such as a minified bundle, is cut at a character boundary.
- Each piece carries the file's diff header and says which piece of how many it is.

Nothing in the diff is dropped or stood in for. The exclusion check has already asked whether each file belongs (§6), so a file still in the diff is wanted, and it is read.

#### 8.5 Concurrency

At most four chunk calls run at once. The bound is fixed, not a setting. Unbounded fan-out would put the whole wave on the machine, and on the provider's rate and usage limits, at the same moment.

#### 8.6 What each call is told

- **A chunk call** carries:
  - the release's instructions: the default prompt with `context`, or the `[release].prompt` override, which replaces both (as built)
  - the Change Map
  - framing that names its part of the release (part k of n; for a piece, the file and which piece of how many)
  - its slice
  - the closing reminder.

  It returns notes for its part, in the release's notes format.
- **The merge call** carries:
  - the release's instructions
  - the Change Map
  - every chunk's notes, in chunk order
  - framing that asks for one set of release notes, combining related changes across parts, dropping duplicates and ranking by the Change Map
  - the closing reminder.

  Its output is the release's notes body, passed through byte-identical like a single-pass body.

mint's framing is fixed text of its own; the exact wording is the implementer's.

#### 8.7 Calls that cannot be made

- **No room for diff.** Even with the Change Map condensed, a call's fixed part can leave no room for diff. That happens with an outsized `[release].prompt` or `context` (operator text mint never trims), with a `max_prompt_bytes` below the fixed part of mint's own instructions when neither key is set, or with a halving round aimed below the fixed part (§10). mint then sends no call, and the run falls to the floor (§12) with guidance naming which of these it was.
- **A merge prompt over the cap** is not sent, and counts as a failed merge (§11).

Chunk notes are held in memory for the run. They persist only in a failure's saved file (§11) and in the dry-run cache (§15).

#### 8.8 Always on

Chunking has no setting. There is no `oversize` key and no way to switch it off. Chunking only happens when a release cannot go in one call, so switching it off would change one thing: such releases would get no AI notes. Nobody wants that as a standing policy. A release that should not get AI notes already has `--no-ai` and `--notes-file` (§13).

Chunking needs no consent prompt for its extra calls; it handles the release without asking. A key is cheap to add and costly to remove: under strict config, a repo carrying a removed key fails to load. So one can be added later if a real case appears, such as an API-key setup where the extra calls cost money. `max_prompt_bytes` (§4) stays the only size setting.

### 9. Failure Classification

#### 9.1 The trigger

When a release-notes generation call fails, mint asks the same AI what its own error means, and only a size answer starts the degradation ladder. The release-notes generation calls are the single-pass call, a chunk call and the merge call.

The classifier was chosen for two reasons:

- It is provider-agnostic by construction, because the AI interprets its own error.
- It never fires a wave of calls at a failure that chunking cannot fix. On a rate limit, a one-off server error or an expired login, splitting would multiply one failure into many.

Accepted cost: one extra AI call per failure.

The classifier runs only when the call fails with a generation failure: a non-zero exit, or an empty body, surviving the transport's single retry. The transport already tells the other failures apart, and none of them reach the classifier:

- **A timeout** never starts the ladder. It falls to the floor (§12), because a timeout is not evidence of size, and a wave fired at a slow service multiplies the wait. The size cap absorbs "too slow because too big" before any call is sent (§4.4). The rule is revisited if timeouts show up on releases that sit under any sensible cap.
- **A missing binary** falls to the floor (§12).
- **A cancel** (`context.Canceled`) propagates unchanged and never falls to `on_notes_failure`.

The classifier is release-notes only, for forward release and regenerate. `mint commit` makes no classify call, because every commit failure already ends in the editor (§17). The exclusion check never classifies its own failures either (§6.8).

#### 9.2 The call

The classify call goes through the existing transport, with the release's `ai_command` and timeout; there is no new AI seam. Its prompt carries:

- the two codes, each with a one-line gloss: SIZE for a request too large for the model's context window or input limit, and OTHER for anything else
- the failed call's captured stdout and stderr, both streams, fenced as captured tool output, explicitly not instructions
- an instruction to answer with exactly one code.

Each stream longer than 4,000 bytes keeps its first and last 2,000 bytes, cut at character boundaries, with a line saying how many bytes were omitted between them. That keeps the call small, a few kilobytes, and holds both ends of the output, where causes sit. Under a cap smaller than the classify prompt, the call is not sent, since no call mint makes is ever bigger than the cap (§4.1, §8.7). The failure then takes the OTHER path, as when the classify call fails (§9.5).

The stakes of untrusted text here are bounded. The answer only picks a branch, an injected answer can at most misclassify, and under the lean command (§2) the call has no tools to act through.

#### 9.3 The codes

| Code | Reaction |
|---|---|
| SIZE | The ladder: halving for a single-pass or chunk call (§10). On the merge call, a failed merge (§11). |
| OTHER | No split. The run falls to the floor (§12). |

Only about three reactions exist however many codes are defined: split, retry or back off, and surface and stop. No code beyond SIZE has a reaction worth its cost for the subscription case. The set extends additively. A RATE_LIMIT code, with a wait-and-retry that would be a deliberate exception to "consumers never re-retry", can be added if API-key setups hit throttling mid-wave. Anything unrecognised already falls to OTHER.

#### 9.4 Reading the answer

mint strips surrounding whitespace, quotes, backticks and trailing punctuation from the answer, then compares what remains case-insensitively against the codes. It is a listed code only when what remains is exactly `SIZE` or exactly `OTHER`. Anything else maps to OTHER: a sentence, both codes, an unknown word.

There is no firmer re-ask. The transport owns validation and the single retry, and consumers never re-retry. Worst case: the error surfaces where chunking might have helped.

#### 9.5 Edges

- **No error text.** When both captured streams are empty or whitespace-only, mint skips the classify call and takes the OTHER path directly. Classifying nothing could only be a guess, and a guessed SIZE would fire the whole wave at a failure chunking cannot fix.
- **The classify call fails.** A generation failure, timeout or missing binary on the classify call takes the OTHER path. The output rendered is still the original call's, never the classify call's. When the tool's overhead alone overflows the window, the classify call fails the same way, and that is the correct stop: splitting cannot fix it.
- **A cancel during the classify call** propagates unchanged and never falls to `on_notes_failure`.
- **The transport's single retry stays**, for every call including the classify call. So an under-cap overflow sends the identical call twice before the classifier looks. On claude the repeat costs one more round trip: a diff the model cannot take passes claude's local check, is uploaded, and the API refuses it with a 400 in a second or two at no token cost (`{ git log --all -p --no-color -- '*.go' | head -c 2600000; printf '\n\nReply with the single word: ok\n'; } | claude -p --model sonnet --safe-mode --tools "" --strict-mcp-config --no-session-persistence --system-prompt "Follow the instructions you are given and output only what they ask for." --output-format json` → `"api_error_status":400`, `"terminal_reason":"prompt_too_long"`, `"duration_ms":1586`, `"total_cost_usd":0`, result `Prompt is too long · the request is ~1162407 tokens (limit 1000000) but this conversation is only ~648221 tokens — the rest is system prompt, tool definitions, and attachment content.` (2026-10-18)). claude refuses locally only when its own estimate, about 4 bytes a token, already puts the prompt over the window. The cap makes under-cap overflows rare, so making the retry rule depend on which command calls is not worth the saving. This is revisited if under-cap overflows become routine, for example on smaller-window models run with the cap left at the default.

#### 9.6 Alongside the verbatim error

The classifier adds to the AI's verbatim output; it never replaces it. Its code drives control flow (split or stop) and selects the ✗ line's cause phrase and mint's guidance (§12). The AI's captured output still renders verbatim beneath the ✗ line, as subprocess diagnostics always do. The AI's own text is often the most specific remediation available, for example `Not logged in · Please run /login`, or claude's token breakdown on an overflow.

#### 9.7 An explicit `ai_command` that carries its own overhead

A repo whose own `ai_command` carries an overhead that alone overflows the window gets no chunk small enough to fit. Its classify call fails the same way, so the run takes the OTHER path with the AI's verbatim message, then the floor. mint's guidance points at the documented lean command rather than naming claude flags itself (§12).

### 10. Halving After an Under-Cap Overflow

#### 10.1 When it applies

A prompt under the cap can still overflow the model. That happens when an explicit `ai_command` points at a smaller-window model, when an explicit command carries overhead of its own, or when the cap is set above the real window. The failed call was already under the cap, so the cap gives the chunker nothing smaller to aim at.

So when the classifier answers SIZE for a diff-carrying call (the single-pass call or a chunk call), mint halves:

- The new target is half the composed size of the call that failed: instructions, Change Map, framing, diff and reminder together.
- The slice that call carried is re-cut at the new target, by the first wave's packing and piece rules (§8.3, §8.4). A single-pass call that fails SIZE therefore becomes a chunked run at half its size.
- Each call in a halved round has its own room for diff, which is whatever its fixed part leaves. A round aimed below the fixed part has no room, and no call is sent (§8.7).
- A SIZE answer on the merge call is not halved. Halving a merge would be the recursive-collapse reduce that is ruled out, so it is a failed merge instead (§11).

#### 10.2 The bound

A slice is halved at most three times, counted from its first SIZE answer, which takes it to an eighth of the size that first failed. A fourth SIZE fails that slice, and a failed slice fails the generation (§11). A cap wrong for its model by more than eight times is not a slight miscalibration.

Halving's rounds are serial, a deliberate exception to the parallel-first shape. The exception is accepted because the path is rare when the cap fits the model, and its cost lands where the misconfiguration is.

#### 10.3 Telling the operator

- **The notes were written after halving.** After the notes, mint says the cap is set higher than this model accepts, and suggests the size of the largest call that succeeded in the run. That turns a quiet absorption into something the operator can fix. The suggestion is that call's exact size in bytes, written as the value to set, such as `max_prompt_bytes = 1260000`, never rounded: a size rounded up would sit above the largest call the model accepted. Only calls that carry diff count: the single-pass, chunk and piece calls, which are what overflowed and what the cap sizes. The merge, classify and exclusion-check calls do not. Under `regenerate --all` the hint prints once, at the end of the batch, naming the largest such call that succeeded across the batch, since the whole batch runs under one cap and one command. It prints however the batch ends once any version's notes were written after halving, including a batch that stops early at a declined gate, a failed `r` or a Ctrl-C. The hint records no state, so printing it on the way out leaves the cancel clean.
- **Halving ran out.** mint cannot tell a small window from a command with a large overhead of its own: in both, even small pieces come back SIZE while the small classify call fits. So the floor's guidance names both causes: the model accepts less than the cap (lower `max_prompt_bytes`), or the command carries a large overhead of its own (use the documented lean command, §2) (§12).

### 11. Failures in a Chunked Run

#### 11.1 One failed chunk fails the generation

A chunk call (or piece) ends in failure in either of these cases:

- after its transport retry, its classify call and, on SIZE, its halving rounds (§10)
- on a timeout or a missing binary.

When any chunk ends in failure, no merge runs. mint saves the notes of the chunks that did finish (§11.4) and names the file in the failure. The run then goes to the floor like any other failure (§12).

Merging what came back is out. The missing slice can be the release's headline feature. Notes that read as complete while quietly leaving out the biggest change are the very failure this feature exists to end, and under `-y` they would ship unreviewed.

#### 11.2 The first final failure ends the wave

A chunk failure ends the wave when it finishes that chunk for good, which is anything except a SIZE answer with halving left:

- an OTHER answer
- a failed classify call
- a failure with no output to classify
- a timeout, which gets no classify call
- a missing binary
- a SIZE answer with no halving rounds left.

After it, mint starts no new chunk calls and makes no further classify calls. Calls already running finish, and the notes they produce are saved with the rest. A SIZE answer with halving left does not end the wave; that chunk halves (§10).

Once one chunk has failed for good, the generation has already failed, so classifying further failures could only confirm it. A usage limit or an expired login landing mid-wave would otherwise send up to four calls per chunk at a service already refusing them. Letting in-flight calls finish costs no extra requests, and what they return makes the attended floor's `e` cheap (§12).

The ✗ line renders that first final failure's cause phrase and its captured output verbatim (§12).

#### 11.3 A failed merge

The merge can fail for reasons other than size. It is the last call after a parallel wave, so it is the one most likely to meet a usage limit the wave itself drew down, an expired login, a timeout, or an error that survives the transport's retry.

There is no concatenation rung. Gluing the chunk notes together would ship a visibly broken body: repeated section headers, near-duplicate bullets, no ranking. That is worst exactly when nobody is reviewing.

A merge counts as failed in three cases:

- any failure of the merge call
- a SIZE answer from the classifier
- a merge prompt over the cap (§8.7).

mint then saves every chunk's notes (§11.4), names the file, and goes to the floor (§12).

#### 11.4 The saved chunk-notes file

- **Where:** `{user cache dir}/mint/chunk-notes/{repo path, separators as dashes}/{tag}.md`. That is outside the repo, beside the dry-run notes cache but outside its pruning, because the file must outlive the run. A later failure of the same tag overwrites it. The failure names its full path.
- **Contents:** each finished chunk's notes in chunk order, separated by a blank line, exactly as the chunk calls returned them.
- **The marker:** when any slice has no notes, the file opens with one bold line, then a blank line before the notes:

  ```text
  **Notes missing:** no notes were written for {files} — add them before releasing.
  ```

  It lists every file with no notes. A file whose pieces only partly failed is marked `(partly)`. The failure guidance says the same thing.

The marker lives in the file itself because the file is what gets picked up later. That might be by the operator through `e`, or by an agent session releasing with `--notes-file`, possibly after the terminal output is gone. Accepted cost: the marker line ships if the file is released unread. An attended release shows it at the review gate.

**No file when no chunk finished.** mint saves and names a file only when at least one chunk's notes exist. A wave can fail before any chunk finishes, for example when a login expires or a usage limit lands at the start. Then the failure names no file, the floor's `e` opens empty, and there is no `--notes-file` pointer. A file holding only the marker would otherwise pass `--notes-file`'s empty check and ship as the release's notes.

**When the file cannot be written.** When the user cache dir cannot be resolved, or the write fails, the failure stays the AI's: its ✗ line, cause phrase and captured output are unchanged. mint adds one warning that the finished chunk notes could not be saved, with the write error verbatim beneath it. The run then goes on as when no file was saved: the failure names no path, the floor's `e` opens empty, and the guidance says to write the notes and release with `--notes-file`.

The file keeps the wave's work. At a terminal the floor's `e` opens it (§12). `--notes-file` releases it (§13), either on the release itself or, for a version already out, through regenerate.

#### 11.5 Ctrl-C mid-wave

A Ctrl-C cancels every call in flight and aborts cleanly. No chunk notes are saved, and `on_notes_failure` does not apply. An interrupted chunked run behaves like an interrupted single-pass one: it stops, and a re-run starts over. mint keeps no resume state.

#### 11.6 Timeouts across a wave

The per-attempt timeout applies to each call on its own: every chunk call, the merge and the classify call. A wave therefore takes roughly as long as its rounds of chunk calls plus the merge.

There is no overall deadline across the calls. One would be a second timing rule whose right value grows with the release, which is exactly what the per-call cap exists to avoid predicting. A chunk call's timeout is a final failure that ends the wave (§11.2). A timeout on a single-pass call is covered in §9.1.

### 12. The Remediation Floor

The floor is where the ladder ends without AI notes. With the lean default and chunking it is reached far less often, but it is still reached: chunking exhausted, a login expired, a usage limit hit. By then the release is fully prepared, so a re-run would repeat every step to change one input.

#### 12.1 What applies, by mode

On `mint release`, the floor depends on the mode and `on_notes_failure`. Regenerate does not read `on_notes_failure` and has its own floor (§12.3).

| Situation | What happens |
|---|---|
| Attended, `on_notes_failure = abort` | The ✗ line and the AI's verbatim output, the saved chunk-notes pointer where a file exists (§11.4), then a one-keypress choice (§12.2). |
| Unattended (`-y` or non-TTY), `abort` | The ✗ line, the AI's verbatim output and the cause-fitted next moves (§12.4), then the run stops. Never a prompt. |
| `on_notes_failure = fallback` | No choice. The commit-subject list (§14) ships, with one warning whatever the cause: the cause phrase (§12.4), the AI's captured output verbatim beneath it, and, where a chunk-notes file was saved (§11.4), its path and both commands that release it: `mint release regenerate <version> --notes-file <path>`, which puts it on the version once this release is out, and `mint release --notes-file <path>`, for a run that ends before the push, such as an abort at the review gate. |

A failed `r` is not the floor; it aborts (§15).

**A dry run at the floor** asks nothing, whatever the mode. Under `abort` it renders the ✗ line, the AI's verbatim output, the saved chunk-notes pointer where a file exists (§11.4) and the cause-fitted next moves (§12.4), as an unattended run does, then ends as a failed run. The next moves follow the dry run's own mode: the `on_notes_failure = "fallback"` move shows only when the dry run is itself unattended (`-y` or non-TTY), since at a terminal without `-y` the real run offers the choice rather than stopping. Under `fallback` it previews the commit-subject list as the body that would ship, with the fallback warning, which names a saved file with `mint release --notes-file <path>` (§12.4), since the version is not out. The chunk-notes file is saved as on a real run: it sits outside the repo beside the dry-run cache, and the real run can release it with `--notes-file` instead of repeating the wave.

#### 12.2 The choice

At a terminal under `abort`, one choice through the presenter's gate seam:

- **`c`**: this release's commit-subject list (§14).
- **`e`**: write the notes in `$EDITOR`. It opens on the saved chunk notes when a chunk or the merge failed and a file exists; otherwise it opens empty. This is a new write-from-scratch route: release's editor seam had only edited an already-produced body.
- **`a`**: abort. This is the default.

The chosen body goes on to the review gate a non-AI body gets as built: accept, edit, abort, with no `r`, since there is no AI output to redo. The release then continues as usual.

**An empty `e` is refused.** When the operator closes the editor leaving an empty or whitespace-only body, which is the usual way to back out, mint returns to the `c` / `e` / `a` choice. An empty body would otherwise reach the review gate, where accepting it ships an empty tag message, changelog entry and provider release. Git accepts one: `printf '' | git tag -a v1 -F -` exits 0.

**`e` edits a copy.** The editor opens on a copy of the saved chunk notes, as release's editor seam edits a temp copy of the body it is given. The file at the path keeps the chunk notes as the calls returned them (§11.4), whatever the operator writes, and the edited text lives in the run like any edit at the review gate.

**An editor that cannot run.** When no editor can be launched, mint warns and returns to the `c` / `e` / `a` choice. Any other editor failure, such as a launched editor exiting non-zero, is surfaced and aborts. Both are what the review gate does as built.

**Every abort at the floor unwinds.** The floor sits after the pre-tag artifact commit, so however it ends a release before the push, whether `a`, an editor failure, or stdin closing at the choice, the run unwinds as any failure before the push does (§6.6): back to the starting commit, taking the pre-tag artifact commit with it.

#### 12.3 Regenerate at the floor

Regenerate does not read `on_notes_failure`, as built, and this feature does not make it. As built, a failed regenerate aborts. `fallback` stays release's: it exists so a release goes out without AI notes rather than not at all, and a regenerate blocks nothing, since its version is already out with notes. Shipping the commit-subject list over those notes whenever an AI call failed would replace them with a worse body. An operator who wants the subjects on a version picks `c`.

- **`regenerate <version>`, attended:** the same choice, whatever `on_notes_failure` holds, finishing in the same run. `c` gives that version's commit subjects, `e` opens the editor (on the saved chunk notes when a chunk or the merge failed), and `a` aborts, which is as-built regenerate's one outcome and stays the default. As on release, an empty `e` is refused and returns to the choice, and the chosen body goes on to regenerate's own review gate without `r`: accept, edit, abort, since there is no AI output to redo. Regenerate already runs attended with its own review gate, and stopping would only send the operator to re-run with `--notes-file`. Unattended, it stops with guidance, as release does under `abort`.
- **`regenerate --all`:** skip-and-continue stays its floor, whatever `on_notes_failure` holds. Each skipped version names its real cause phrase (§12.4), in its warning and in the end-of-run summary, never `diff too large`. Its skip warning carries the failed call's captured output verbatim beneath it, as the ✗ line does; in a chunked run, that of the first final failure (§11.2). As built the skip warning carries no output at all (`grep -n 'Label: "skipped "' internal/engine/regenerate_batch.go` → `382:	p.Warn(presenter.Warning{Label: "skipped " + tag, Message: reason})`), so an expired login across a batch would otherwise read only as `AI call failed`. A version whose wave left chunk notes saves its own file (§11.4), named in its skip warning and in the summary, to be fed back with that version's own `mint release regenerate <version> --notes-file <path>`. A version whose single-pass call failed, or whose wave produced nothing, names no file.

#### 12.4 The ✗ line and the next moves

The ✗ line's cause phrase comes from the cause. The AI's captured output still renders verbatim beneath it (§9.6).

| Cause | Phrase |
|---|---|
| Timeout | `AI timed out` (as built) |
| Missing binary | `AI tool not installed` (as built) |
| SIZE with no halving left, or a halving round with no room (§10.1) | `too large for the model, even in smaller parts` |
| OTHER, with output | `AI call failed` |
| OTHER, no output | `AI returned empty notes` |
| No room for diff caused by operator text, or by a cap below mint's own fixed part (§8.7) | `no room for the diff under max_prompt_bytes` |
| Merge too large: its prompt over the cap (§8.7), or a SIZE answer on the merge call (§11.3) | `part notes too large to merge` |

In a chunked run the phrase ends with where the failure happened: `(part 3 of 6)` or `(merging)`. The suffix names the call the failure belongs to, so two cases take none. `part notes too large to merge` already names the merge. A failure no single call owns has no place to name: no room for any part under operator text or the cap, or a single-pass call whose first halving round has no room. These phrases replace `AI returned empty/invalid notes after retry`, which misnamed every generation failure. Release-notes failures no longer use `diff too large`.

The next moves are cause-fitted. Unattended runs print them; attended runs show the saved-file pointer and the choice.

- **Always:**
  - The `--notes-file` route. Where a chunk-notes file was saved, it says to tidy the saved notes at its path, filling in any files the marker lists, and release with them. The command is `mint release --notes-file <path>` while the version is not out, or `mint release regenerate <version> --notes-file <path>` once it is: after a regenerate, or a forward release that shipped under `fallback`. Where no file was saved, it says to write the notes and release with `--notes-file`.
  - On forward release, `--no-ai`, for the commit-subject list.
  - On an unattended forward release only, `on_notes_failure = "fallback"` under `[release]`, to ship the commit-subject list rather than stop.
- **Size** (SIZE with no halving left on a single-pass or chunk call; no room caused by a halving round):
  - lower `max_prompt_bytes`, since the model accepts less than the cap
  - if `ai_command` carries a large overhead of its own, use the lean command documented in the README (§2), without mint naming claude flags itself
  - exclude generated paths with `[release].diff_exclude`.
- **No room caused by operator text:** shorten the operator text the call carries: `[release].prompt` when the release sets it, since an override replaces the default prompt and `context` with it, otherwise `[release].context`.
- **No room with neither key set:** raise `max_prompt_bytes`, since the cap is below the fixed part of mint's own instructions.
- **Timeout:** raise `[release].timeout`.
- **Missing binary:** install the AI tool, or set `ai_command`.
- **OTHER:** the AI's own message above names the cause. When the resolved `ai_command` is not the shipped default, add the lean-command pointer.

A merge too large, by the cap or by the model, takes only the Always moves, led by its saved part notes. Lowering `max_prompt_bytes` cuts the release into more parts and does not shrink what the merge has to hold.

A failed merge's guidance reads, for example: `notes couldn't be merged; part notes saved to <path> — tidy them and release with --notes-file <path>`.

### 13. `--notes-file`

#### 13.1 What it is

`--notes-file <path>` hands mint a finished notes body. It replaces a workaround: write the notes into `[release].fallback`, release with `--no-ai`, then remove them, at the cost of an extra clean-up commit on main for every such release. With the flag it becomes: write a file, release with it.

It takes a file path only. There is no stdin form: the review gate reads its keypresses from stdin, and an agent can always write a file. The path resolves against the directory mint was run from.

- **Release:** `mint release --notes-file <path>`.
- **Regenerate:** `mint release regenerate <version> --notes-file <path>`, a notes source beside fresh, tag and release. It puts finished notes onto a version already out, the saved chunk notes (§11.4) among them. One file answers one version, so giving it with `--all`, or with `--source`, is a usage error.
- **`mint commit`** gets no message-file flag. It keeps its two message sources, the AI and `$EDITOR`. An agent with its own message gains little from `mint commit` beyond its staging and push flags, `git commit -F` already serves it, and commit's `-A` staging is where an excluded file would be riskiest.

Usage lines, one per verb's usage text:

- release: `--notes-file P   release with the finished notes in file P (no AI)`
- regenerate: `--notes-file P   notes source: the finished notes in file P (one version only)`

#### 13.2 The file is outside everything mint does

- mint reads the file once at startup, before any stash, check or gate, and holds the body in memory.
- The file does not count against the clean-tree check.
- It is left out of `--autostash`'s stash.
- It is never staged or committed by any git step mint runs, including the pre-tag artifact commit's `git add -A`.
- Afterwards it sits in the work tree untouched. Cleaning it up is the caller's business.
- All of this holds whether the file is untracked or a tracked file with uncommitted edits, such as a `NEXT_RELEASE.md` drafted afresh each cycle. A reset during an unwind, the forward release's surgical unwind or regenerate's reset after its changelog commit, leaves the file as it was: its content, and whether its edits were staged. So the operator's draft survives a failed run. After a successful run, edits the operator had staged are likewise still staged, and unstaged ones unstaged. mint removes only what it created.

Keeping the file out of git is mint's job, not only the caller's. The obvious first try is an agent writing `notes.md` at the repo root and running `mint release -y --notes-file notes.md`. Without these exclusions, the clean-tree check would refuse the untracked file, `--autostash` would sweep it away before mint read it, and the pre-tag artifact commit would stage it into a release commit.

#### 13.3 How a supplied body moves through the run

- **Precedence:** a supplied body wins over everything, including the first-release and empty-diff bodies (§7.1). Those exist to produce an honest body when there is nothing for an AI to summarise, which a supplied body does not need.
- **Skipped:** the exclusion check, the size cap, chunking and every AI call. `on_notes_failure` never applies, since it governs only an AI failure.
- **The review gate** is still shown on an attended run, so the operator sees what will ship. It offers accept, edit and abort, without `r`, since there is no AI output to redo. Under `-y` it auto-accepts inside the presenter, as usual. This holds on both verbs: `mint release regenerate <version> --notes-file P` shows regenerate's own review gate, as its fresh source does and as the floor's `c` and `e` bodies do (§12.3), not the simple confirm the tag and release sources run.
- **The body** passes into the tag, changelog and provider release byte-identical, exactly like an AI body. In the tag, the annotation is the bookkeeping subject, a blank line, then the body as written, with git cleaning nothing (`--cleanup=verbatim`), so a `#` heading or a trailing space survives. The annotation ends in a newline: one is added after a body that ends without one, as every AI body does, because git appends a signed tag's signature directly after the message's last byte, and a signature that does not start on its own line is not found by `git verify-tag` and is read back as part of the body. That line terminator is the annotation's, like the subject line and blank line before the body, and not part of the notes.

#### 13.4 Refusals

- **With `--no-ai`:** a usage error, exit `2`, because the two flags give two answers to where the notes come from. The same holds on regenerate with `--all` or `--source`.
- **An empty or whitespace-only file:** refused before any step runs with `--notes-file P is empty`, exit `1`.
- **A missing or unreadable file:** fails before any step runs with `reading --notes-file P: <cause>`, exit `1`.
- **An empty path,** such as `--notes-file ""` from an unset variable, names no file. It fails as a missing file does, before any step runs, exit `1`. It never counts as the flag left off, which would send the run down the AI path the caller meant to skip.

### 14. One Non-AI Body: Dropping `[release].fallback`

#### 14.1 The commit-subject list everywhere

`[release].fallback`, a fixed notes body shared by `--no-ai` and `on_notes_failure = fallback`, is dropped. Every route that ends without AI notes gives this release's commit-subject list:

- `--no-ai`
- `on_notes_failure = fallback`
- the floor's `c` (§12.2).

Hand-written notes come through `--notes-file` (§13).

The commit-subject list is `git log --format=%s` over the release's range, as built: `{lastTag}..HEAD` on a forward release, and the version's own range on regenerate. It leaves out mint's own answers commit (§6.6). On regenerate it also leaves out that version's release bookkeeping commit (`{commit_prefix} Release {tag}`, the commit the tag sits on), where one was made. It is the commit the tag sits on when that commit's subject ends in ` Release {tag}`, whatever prefix comes before it, so a version released under an earlier `commit_prefix` is matched too; a tag on any other commit leaves the list whole. That commit is made after the notes, so the forward release's list never held it; with it left out, regenerate's list for a version is the one that version's forward release would have shipped. The pre-tag artifact commit stays, as it does on the forward range, where it lands before the notes. An empty list gives the maintenance-release body, so the tag body is never empty, as built.

`on_notes_failure` keeps its two values. `fallback` still reads as falling back to a non-AI body.

With one non-AI body there is no longer a mismatch where `c` differs from `--no-ai` in a repo with a fixed fallback string. The key's one observed use was hand-written notes for a release the AI could not take, which is `--notes-file`'s job now.

#### 14.2 Removing the key

Under strict config, a `.mint.toml` still setting `[release].fallback` fails at load, with a removed-key message alongside the `[hooks]` placement one:

```text
invalid .mint.toml: release.fallback has been removed — every non-AI body is the commit-subject list; supply hand-written notes with --notes-file
```

The key's config-metadata row and its README table entries go with it. So does the fixed string in `on_notes_failure`'s own description: its config-metadata row says `fallback uses the commit-subject / fixed-string body`, and its README row adds "or `fallback` string". Both come to say `fallback` ships the commit-subject list. The setup guide's prose follows (§18).

No repo of the user's sets the key, so none needs a change before upgrading: `grep -lE '^[[:space:]]*fallback[[:space:]]*=' ~/Code/*/.mint.toml` → no output (2026-10-18).

### 15. Chunked Notes at the Review Gate and in the Cache

#### 15.1 `r` on chunked notes

Chunked notes are AI notes, so the attended review gate offers `r`, regenerate with a one-time nudge, as it does for single-pass notes. The review gate is where an operator rejects a weak stitch.

On chunked notes, `r` re-runs only the merge call. It reuses the per-chunk notes already produced and adds the operator's nudge. That is one call, as `r` costs on a single-pass release, and not a re-run of the whole wave.

A nudge asks to change emphasis or ordering, and the merge is where emphasis and ordering are decided; the per-chunk notes do not change with it. The residual this cannot fix, a change a chunk's own notes missed, is rare.

On single-pass notes, `r` re-runs the same AI path as built, with the nudge appended: from the size check onward (§7.2), so the ladder applies to it as to the first attempt. Its call is a release-notes generation call like any other, so the cap (§4.1) and the classifier (§9.1) apply by their own terms: a nudge that pushes the prompt over the cap sends the release to chunking rather than one over-cap call. The nudge rides in the release's instructions (§8.6), so when the ladder turns the `r` into a chunked run, every chunk, piece and halving-round call and the merge carry it. It never re-runs the exclusion check, which asks about paths, not notes, and whose answers were recorded before the first attempt.

**When `r` fails.** A failed `r` aborts the run whatever `on_notes_failure` says, as built. On a release the abort unwinds as any failure before the push does (§6.6): back to the starting commit, taking the pre-tag artifact commit with it. An `r` that ends because stdin closed at its nudge prompt is a failed `r` too, as built, and unwinds the same way. On chunked notes, and on a single-pass `r` the ladder turned into a chunked run:

- mint first saves the per-chunk notes and names the file with the command that releases it, `mint release --notes-file <path>` (§11.4, §13).
- `on_notes_failure` does not apply, so `fallback` never ships its body in place of the re-merge the operator asked for.
- The abort lands before any tag, so the saved notes can still go out through `--notes-file` (§13).

A Ctrl-C during an `r`, the merge-only `r`'s re-merge call included, is a cancel, not a failed `r`: it saves no chunk notes, as a Ctrl-C mid-wave saves none (§11.5).

**Every abort at the review gate unwinds.** The review gate sits after the pre-tag artifact commit, so however it ends a release before the push, whether a decline (as built), a failed `r`, a launched editor that fails at the gate's `e`, or stdin closing at the gate's keypress, the run unwinds as any failure before the push does (§6.6): back to the starting commit, taking the pre-tag artifact commit with it. A missing editor still warns and returns to the gate, as built. Regenerate's review gate lands before its changelog commit, so an abort there has nothing to reset.

**On regenerate.** A single-version regenerate from a fresh source has its own review gate, and its `r` works the same way. On chunked notes it re-runs only the merge call. On single-pass notes it re-runs the AI path from the size check onward. A failed `r` aborts the regenerate. If the notes were chunked, or the ladder turned the `r` into a chunked run, mint first saves the per-chunk notes and names the file (§11.4). The version is already out, so the saved notes go onto it with `mint release regenerate <version> --notes-file <path>` (§13).

`regenerate --all` run at a terminal without `-y` shows each fresh version at the same review gate, and its `r` follows the same rules for that version: only the merge on chunked notes, the AI path from the size check onward on single-pass notes. A failed `r` saves that version's per-chunk notes where the notes were chunked or the ladder made the `r` a chunked run, names the file with that version's `mint release regenerate <version> --notes-file <path>` (§11.4, §13), and ends the batch, as a failed gate does as built. It is not the batch's floor, so the batch does not skip the version and carry on.

#### 15.2 The dry-run cache

The notes cache keys on inputs, not output: the post-exclusion diff, the version and the resolved instructions. A dry run's chunked notes are therefore cached and offered back to the real run exactly like single-pass notes, sparing the real run the whole wave.

A chunked body is cached together with its per-chunk notes. So `r` on reused chunked notes re-runs only the merge, one call, as on a live run.

The per-chunk notes share the cache entry's lifecycle and need no cleanup of their own. The entry is:

- outside the repo
- pruned to the latest entry per project on every write
- stale after the ~1h TTL
- keyed on the version, so once the release is tagged its key cannot match again.

The cache is written on real runs as well as dry runs, as built, and an entry holds the per-chunk notes whichever run wrote it. So a real run aborted at the review gate leaves chunked notes the next run can reuse, and `r` on them re-runs only the merge. Outside the cache entry the per-chunk notes live only in memory and end with the run, and only a failure's saved file persists (§11.4).

### 16. Progress Narration

#### 16.1 Pretty mode: one notes line, updated

A chunked run can take minutes: the exclusion check, rounds of chunk calls a few at a time, maybe halving, then the merge. A spinner that sits that long looks the same whether mint is working or hung.

So a chunked run keeps the single notes line and updates it as the run moves:

```text
⠋ Checking for files that don't belong in the notes…
⠋ Writing notes: 3.1 MB release, in 6 parts · 2 of 6 done
⠋ Writing notes: part 4 was too big for the model, retrying it as 3 smaller parts
⠋ Writing notes: merging 8 parts
✓ Notes written (8 parts, merged) (64.2s)
```

- The check line shows on every release that runs the check (§6.1), single-pass included.
- The split line names the release's size and its part count, then counts parts done out of the total. The size is the composed single-pass prompt the size check measured against the cap (§7.2), since that measurement is what split the run.
- Sizes are written in decimal units, 1 MB being 1,000,000 bytes, so the default cap reads 1.5 MB. Below 10 a size takes one decimal place and from 10 up a whole number: `3.1 MB`, `38 MB`, `850 KB`, and plain bytes under 1 KB (`640 B`). The exclusion check's list (§6.2, §6.3) and its pauses (§6.5) write sizes the same way.
- A single-pass call that halves (§10.1) shows `Writing notes: too big for the model in one call, retrying in {n} parts`. Here, and when a part halves, n is the number of parts the re-cut yields (§10.1). The run is then a chunked run, so as its parts finish the line moves to the split line, `Writing notes: {size} release, in {n} parts · {k} of {n} done`, with the same composed single-pass size, and plain mode prints one line per finished part.
- When halving splits a part, the total grows by the parts it adds. The split line's part count grows with it, since one total serves both places: `in 8 parts · 5 of 8 done`. Parts are numbered by their place in chunk order, a halved part's new parts taking its place, so a later part's number moves up by the parts added, and `part {k}` on the halving line and `(part k of n)` on the ✗ line always count within the current total.
- The ✓ line keeps the elapsed time every blocking stage's success line carries, after its own text: `✓ Notes written ({n} parts, merged) ({elapsed})`. A chunked run is the longest wait mint has, so the time tells the most there.
- A single-pass run otherwise narrates as today.
- An `r` at the review gate (§15.1) that becomes a chunked run, over the cap at its size check or through halving, narrates as any chunked run does, and in plain mode prints its lines as any chunked run does (§16.2). An `r` that makes one call, the merge-only `r` on chunked notes or a single-pass `r` that stays one call, narrates as `r` does as built.
- The spinner suspends for the check's pauses (§6.5) and for the floor's choice (§12.2), as it does for any gate. After the last pause, the line recording the answers (§6.6) shows before the notes line resumes.

There is no per-part detail beyond the count. Which folders a part covers, or each part's own status, would be noise unless something fails, and on a failure the ✗ line already names the part (§12.4).

#### 16.2 Plain mode: a line per event

CI and agents get no live line, so in plain mode each of those updates becomes its own line as it happens, one per finished part included. A long run never goes quiet.

#### 16.3 Follow-ups

The follow-ups print after the notes:

- the lower-the-cap hint (§10.3)
- the exclusion suggestions under `-y` (§6.7)

On an unattended run that stops at the floor, the exclusion suggestions print with the floor's next moves instead (§6.7).

The failed-check warning (§6.8) is not a follow-up: it prints when the check fails, before the notes are written.

#### 16.4 How it is built

The updates are new presenter events in the existing model. Spinners are a pretty-only concern spanning a stage's start and its success or failure, and plain mode renders the same events as terse lines. Each presenter renders the new events in its own way. The engine never touches spinners or TTY state.

### 17. `mint commit`'s AI-Failure Output

#### 17.1 The AI's output, verbatim

As built, any AI generation failure on `mint commit` drops the AI's captured output. Release, by contrast, renders it verbatim under the ✗ line. Commit breaks the rule that subprocess failure diagnostics travel verbatim, as `regenerate --all`'s skip warning does (§12.3), and it matters for any commit failure: an expired login, a usage limit, a claude too old for the lean flags (§2.5).

- **Attended:** the AI-failure warning, `could not generate a message, opening editor`, carries the AI's captured output verbatim beneath it, then the editor opens as built.
- **Unattended** (`-y` or non-TTY): when the run fails loud because the AI failed, the failure carries the AI's captured output verbatim beneath its `no AI message and no interactive editor available` line. The unattended run is the one with nobody present to re-run it, so it needs the cause most.

The other triggers into commit's editor fallback carry no AI output, and their lines are unchanged:

- `--no-ai`
- an oversized prompt (§4.3)
- an all-excluded diff.

#### 17.2 What commit does not get

- **No classify call.** The classifier's only distinct reaction is the degradation ladder, which commit does not have: every commit failure already ends in the editor, so a classify call would only delay the editor by one AI call.
- **No chunking, exclusion check or message file** (§1.3).

Commit does get the lean default command (§2), quote-aware parsing (§3), the size cap (§4.3) and the `[commit]` exclusion level (§5.1). Its shipped timeout stays 60s (§4.4).

### 18. Documentation and the Setup Guide

Docs stay true to what is built: CLAUDE.md, the README, the setup guide and code comments all change in the same change as the behaviour they describe.

#### 18.1 The setup guide

`mint setup` prints a version-matched, self-contained guide for an AI agent configuring mint. It is edited only where this feature makes it untrue, and it stays flag-free by design. An agent guide for running releases is parked on the roadmap (§1.4).

- **The AI-transport paragraph** is rewritten to cover:
  - the lean shipped default (§2)
  - the per-call size cap (§4)
  - chunking (§8)
  - the floor (§12)
  - the release timeout's 180s default beside commit's 60s (§4.4)
  - the commit-subject list as the one non-AI body (§14).

  Its `claude -p --model sonnet`, its `default 60`, and its "fall back to the commit-subject / fallback body" go.
- **The `diff_exclude` advice.** The etiquette section ("diff_exclude scope") and procedure step 4 send `.workflows/`, `.claude/`, agent directories, `docs/` and lockfiles to a plain `diff_exclude`. After the split (§5.1), that would hide them from commit messages too. The advice is updated to name the levels: noise that only release notes should skip goes in `[release].diff_exclude`, and the shared list keeps only what neither verb should read, such as lockfiles and build output.
- **The point-of-no-return line.** The pipeline section's "Everything before it is unwindable; nothing after it is" gains its one exception: after an abort or failure before the push, the exclusion check's answers commit stays, re-committed on the starting commit (§6.6).
- **The config reference** renders from the config-metadata source of truth, as built, so new, renamed and removed keys reach it with no change to the guide's own prose. A row whose description names behaviour this feature changes is edited in the source of truth itself, `on_notes_failure`'s among them (§14.2).

#### 18.2 The README

- **Per-key tables** gain `max_prompt_bytes`, `[release].diff_exclude`, `[commit].diff_exclude` and `[release].diff_keep`. They lose `max_diff_lines` and `[release].fallback`. They show `[release].timeout`'s 180s default. The shared `timeout` row names both shipped defaults, 180 seconds on release and 60 on commit, since both verbs resolve through it (§4.4). Its config-metadata row says the same, so the setup guide's config reference follows, and any README line that states the timeout's default names both. The tripwire test checks only that each distinct schema key name appears somewhere in the README (`grep -n 'strings.Contains(readme, key)' internal/config/readme_tripwire_test.go` → `68:		if !strings.Contains(readme, key) {`). It catches a missing `max_prompt_bytes` or `diff_keep`. It does not catch a missing `[release].diff_exclude` or `[commit].diff_exclude` row, because the README already names `diff_exclude` (`grep -c 'diff_exclude' README.md` → `1`).
- **The unwind promises.** The feature list's "Everything before the `git push --atomic` is unwindable … leaving the repo exactly as it found it" and the surgical-unwind bullet name the one exception: after an abort or failure before the push, the exclusion check's answers commit stays on the branch, re-committed on the starting commit (§6.6).
- **The lean default command** is documented in full. That includes what it drops on purpose, CLAUDE.md files and Claude Code's system prompt (§2.3), and that conventions belong in the `[release]` and `[commit]` `context` and `prompt` keys. It is the lean command the floor's guidance points operators at for their own `ai_command` (§12.4).
- **`ai_command`'s quoting rules** (§3) are documented where `ai_command` is.
- **`--notes-file`** on release and regenerate (§13), the exclusion check and its pauses (§6), chunking and the saved chunk-notes file (§8, §11.4) are documented with the commands they belong to.

#### 18.3 CLAUDE.md and code comments

- CLAUDE.md's config seam says `max_diff_lines`/`diff_exclude` "stay shared-only at the top level". It is rewritten: `max_prompt_bytes` is shared-only, and `diff_exclude` has shared, `[release]` and `[commit]` levels that add rather than override. Any other line naming a renamed or removed symbol is kept true the same way.
- The load-bearing invariant "Surgical unwind, then PONR" says every pre-push mutation is tracked and unwound on failure. It is rewritten to carry the one exception: the exclusion check's answers commit, which the unwind re-commits on the starting commit (§6.6).
- `--notes-file`'s usage lines (§13.1) join the curated usage texts, where a test pins coverage.
- Code comments that state a contract this feature changes are rewritten in the same commit. These include the whitespace split (§3.3), the line-count guard, the single 60s default, and the fixed fallback string.

---

## Working Notes

## Corrigenda

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §3.2 listed "an empty first word" as malformed without saying whether a blank or whitespace-only `ai_command` is now rejected — corrected: a blank value is not malformed; it counts as unset and resolution falls through as built, and "names no program" covers a non-blank value whose split yields no words or an empty first word — settled by: §2.4's reliance on the as-built fall-through chain, §3.3's compatibility promise that a working command is unchanged, and the as-built `AICommandFor` blank-skipping (`internal/config/config.go:525`).

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §13.3 said a supplied body's review gate "is still shown on an attended run" without saying whether `regenerate <version> --notes-file` shows one — corrected: it shows regenerate's own review gate (accept, edit, abort, no `r`), not the tag and release sources' simple confirm — settled by: §13.3's rationale "so the operator sees what will ship", which no verb limits, and §12.3's precedent sending regenerate's non-AI floor bodies through regenerate's review gate without `r`.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §14.1 said regenerate's commit-subject list over "the version's own range" leaves out only the answers commit, which would put the version's release bookkeeping commit into its notes — corrected: regenerate's list also leaves out that version's bookkeeping commit (`{commit_prefix} Release {tag}`), so it matches what the forward release's list held; the pre-tag artifact commit stays, as on the forward range — settled by: measurement — the bookkeeping commit is made after the notes (`internal/engine/release.go:568`) while the pre-tag hook's artifact commit lands before them (`internal/engine/release.go:464`, notes at `:472`), so the forward `{lastTag}..HEAD` list holds the artifact commit and never the bookkeeping one.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §16.1 said "The split line names the release's size" without saying what is measured or how a size is written — corrected: the size is the composed single-pass prompt the size check measured against the cap; sizes are written in decimal units (1 MB = 1,000,000 bytes), one decimal place below 10 and a whole number from 10 up, KB below 1 MB and bytes below 1 KB, and the exclusion check's list and pauses write sizes the same way — settled by: derivation — the cap and every call size in the feature are composed-prompt bytes (§4.1, §10.1) and the split is decided by that measure (§7.2); the spec's two renderings, `3.1 MB` (§16.1) and `38 MB` (§6.3), both fit the one-decimal-below-10 rule, and decimal units keep the 1,500,000-byte default reading as 1.5 MB.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §4.3 said a `mint commit` prompt over the cap "opens `$EDITOR` as today" without saying what `r` at commit's review gate does when its nudge pushes the regeneration's composed prompt over the cap — corrected: no call is sent, and the editor opens empty with the `diff too large to summarise — opening editor` note, as after a failed regeneration — settled by: §4.1 (no call mint makes is ever bigger than the cap), §1.3 and §4.3 stating commit's over-cap route without limiting it to the first generation, §15.1 applying the cap to release's `r` on release's own terms, and the as-built routing of a failed commit regeneration to the empty editor fallback (`internal/commit/run.go:413`).

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §16.1 showed the chunked run ending `✓ Notes written (8 parts, merged)` without saying whether that line keeps the elapsed time every blocking stage's success line carries — corrected: it keeps it, after its own text, `✓ Notes written ({n} parts, merged) ({elapsed})` — settled by: the discussion's Narration Wording decision (2026-10-19 revision), user agreed.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §13.2 stated the `--notes-file` exclusions only through the untracked case and left open a tracked file with uncommitted edits, which an unwind's `git reset --hard` would wipe — corrected: the exclusions hold for a tracked file with uncommitted edits too, and a reset during the forward release's surgical unwind or regenerate's post-commit reset leaves the file with the content it had — settled by: derivation — §13.2's bullets are stated about "the file" with no tracked/untracked limit under a heading that puts it "outside everything mint does" and promises it "sits in the work tree untouched"; refusing a tracked file would contradict the unconditional clean-tree bullet; the surgical unwind removes only what mint created (CLAUDE.md "Surgical unwind, then PONR"), and §6.6's `--autostash` rule never discards the operator's work.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §13.2 said the notes file "sits in the work tree untouched" and an unwind's reset "leaves the file with the content it had" without saying whether edits the operator had staged in a tracked notes file stay staged — corrected: the file keeps both its content and its staged state, after a successful run and after an unwind alike — settled by: derivation — "untouched" under a heading that puts the file "outside everything mint does" leaves no room for mint to stage or unstage it, and the surgical unwind removes only what mint created (CLAUDE.md "Surgical unwind, then PONR"); the operator's index state is not mint's.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §13.4 refused an empty file and a missing or unreadable one without saying what `--notes-file ""`, an empty path value, does — corrected: an empty path names no file and fails as a missing file, before any step runs, exit `1`; it never counts as the flag left off — settled by: derivation — §13.1 says the flag "takes a file path only" and §13.4 already fails a path that names no readable file; reading an empty value as an absent flag would take the AI path, unreviewed under `-y`, that the caller gave the flag to skip, against §13.4's own ground that the flag answers where the notes come from.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §9.2 said the 4,000-byte stream budget "keeps the call small under any cap", which is false for a cap below the classify prompt's own size, and left open whether such a call is sent — corrected: under a cap smaller than the classify prompt the call is not sent, and the failure takes the OTHER path — settled by: derivation — §4.1 (no call mint makes is ever bigger than the cap), §8.7 (a call that cannot be made under the cap is not sent), and §9.5 (a classify call that fails takes the OTHER path, with the original call's output rendered); §9.2 fixes the trimming rule, so trimming further would be a second rule the record never made.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §10.3 said the hint "suggests the size of the largest call that succeeded in the run" without saying how the size is written — corrected: it is that call's exact byte count, written as the value to set (`max_prompt_bytes = 1260000`), never rounded — settled by: derivation — `max_prompt_bytes` is an integer count of bytes (§4.1), the hint exists so the operator can fix the cap (§10.3), and a decimal-rounded size can sit above the largest call the model accepted; the §16.1 corrigendum's decimal format covers narration (the split line, the check's list and pauses), not a value to paste into config.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §10.3 did not say how the lower-the-cap hint behaves under `regenerate --all`, where several versions can halve — corrected: it prints once, at the end of the batch, naming the largest call that succeeded across the batch — settled by: derivation — the hint advises on one setting and the batch runs under one cap and one command, so per-version hints would repeat one remedy with differing sizes the operator would have to reconcile; "in the run" reads as the batch, and the batch already reports in one place at its end (`batchSummary`, `internal/engine/regenerate_batch.go:401`).

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §11.4 said mint writes the chunk-notes file and "The failure names its full path" without saying what happens when the user cache dir cannot be resolved or the write fails — corrected: the failure stays the AI's, with its ✗ line, cause phrase and captured output unchanged; one warning says the chunk notes could not be saved, with the write error verbatim; the run goes on as when no file was saved — settled by: derivation — §9.6 and §11.2 keep the AI's failure and verbatim output on the ✗ line, so a write error cannot replace them; naming no file silently would hide that finished notes were lost, against CLAUDE.md's "Fail loud" and verbatim-diagnostics idioms; §11.4's no-file branch and §12.4's "Where no file was saved" guidance already define the run without a file.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §10.3's hint names "the largest call that succeeded in the run" without saying which calls count, so a merge, classify or exclusion-check call could set the suggested value — corrected: only calls that carry diff count, the single-pass, chunk and piece calls — settled by: derivation — §10.1 confines halving to diff-carrying calls, those are the calls that overflowed and the ones the cap sizes (§4.2, §8.3), and a merge's size carries notes prose of no measured density (§4.5 measures diff only), so naming it could suggest a value at which chunk calls overflow again.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §10.3's `regenerate --all` hint prints "at the end of the batch" without saying whether a batch that stops early prints it — corrected: it prints however the batch ends once any version's notes were written after halving, a declined gate or a failed `r` included — settled by: derivation — §10.3 ties the hint to the notes being written after halving, not to how the run ends, and its ground, that a quiet absorption becomes something the operator can fix, holds for a version already regenerated before the batch stopped; a single-version run prints the hint after its notes (§16.3), ahead of the gate where a decline would land.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §12.4 said "In a chunked run the phrase ends with where the failure happened" without saying which phrases take the suffix — corrected: the suffix names the call the failure belongs to; `part notes too large to merge` takes none, since it already names the merge, and a failure no single call owns takes none (no room for any part under operator text, or a single-pass call's first halving round with no room) — settled by: derivation — the suffix exists to say where the failure happened, which these phrases either already say or have no single call to point at (§8.7 sends no call when there is no room); Phase 2's acceptance already pins both §8.7 phrases bare.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §16.1 said "When halving splits a part, the total grows by the parts it adds" without saying whether the split line's part count grows too, or how parts are numbered after a re-cut — corrected: the split line's count grows with the total (`in 8 parts · 5 of 8 done`), and parts are numbered by their place in chunk order, a halved part's new parts taking its place — settled by: derivation — the split line's template `in {n} parts · {k} of {n} done` uses one total in both places; §8.6 and §10.1 place a halved slice's parts in that slice's position in chunk order; numbering by place keeps every `part {k}` and `(part k of n)` within the total the line shows.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §16.1 gave a halved single-pass call the line `too big for the model in one call, retrying in {n} parts`, then `merging`, without saying what shows while its parts finish — corrected: the run is then a chunked run, so the line moves to the split line with the same composed single-pass size and counts parts done, and plain mode prints one line per finished part — settled by: derivation — §10.1 says such a call "becomes a chunked run at half its size", §16.1 says a chunked run "keeps the single notes line and updates it as the run moves" so a long run never sits on one line, and §16.2 gives plain mode a line per finished part; the split line's size is the composed single-pass prompt the size check measured, which exists for this run too.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §10.3's `regenerate --all` hint "prints however the batch ends" without saying whether a Ctrl-C counts — corrected: it does; the hint prints on the way out of a cancelled batch once a version's notes were written after halving — settled by: derivation — a single-version run has already printed its hint after its notes, before any later Ctrl-C, and the batch only defers that same hint to its end; the hint records nothing, so §11.5's clean abort (no saved notes, no `on_notes_failure`) holds, and losing it would let the next batch halve silently, the absorption §10.3 exists to end.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §12.1 said a dry run under `abort` renders "the cause-fitted next moves (§12.4), as an unattended run does" without saying whether a dry run at a terminal without `-y` shows the `on_notes_failure = "fallback"` move, which §12.4 gives "On an unattended forward release only" — corrected: the next moves follow the dry run's own mode; the `fallback` move shows only when the dry run is itself unattended (`-y` or non-TTY) — settled by: derivation — §12.1 defines unattended as `-y` or non-TTY and §12.4 limits the move to that mode; the move exists "to ship the commit-subject list rather than stop", and the real run an attended dry run previews offers the choice rather than stopping.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §12.4 said "shorten whichever of `[release].prompt` or `context` is outsized" without saying which key the move names when the release sets both — corrected: the move names each of the two the release sets, the larger first, and with one set names that one — settled by: derivation — when both fill the fixed part together neither alone is the outsized one, mint never trims operator text (§8.7) and cannot tell which the operator would rather cut, so naming only one could point away from the text the operator would shorten.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §12.1's `fallback` warning named a saved file with "the `mint release regenerate <version> --notes-file <path>` command that puts it on the version", though the warning prints at the notes stage, before the review gate and the push, and a run that then aborts or fails before the push leaves no version for that command — corrected: the warning names both commands, `mint release regenerate <version> --notes-file <path>` for once the release is out and `mint release --notes-file <path>` for a run that ends before the push — settled by: derivation — §12.4 picks the command by whether the version is out, which is not yet known when the warning prints; an attended operator who reads the warning and aborts at the review gate to release the saved notes instead needs the release command, and naming both keeps the one warning §12.1 prescribes without adding output to the unwind.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §12.4's earlier corrigendum said the operator-text move "names each of `[release].prompt` and `[release].context` that the release sets, the larger first", and §8.6 said a chunk call carries "the default prompt or the `[release].prompt` override, with `context`" — corrected: a set `[release].prompt` replaces the default prompt and `context` with it, so a call carries one of the two; the move names `[release].prompt` when the release sets it, otherwise `[release].context`, and §8.6 says the override replaces both — settled by: measurement — `ResolveInstructions` (`internal/notes/prompt.go`) returns the override without reading `context`, the README's two-knob line reads "`context` *injects into* the default prompt; `prompt` *replaces* it", and no section of this specification changes that model.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §8.7 gave no room for diff two causes, outsized operator text or a halving round aimed below the fixed part, leaving out a `max_prompt_bytes` below the fixed part of mint's own instructions when neither `[release].prompt` nor `context` is set — corrected: that is a third cause; it renders `no room for the diff under max_prompt_bytes` with no location, and its move is to raise `max_prompt_bytes` — settled by: derivation — any positive cap loads (§4.1) and the default prompt and closing reminder alone take a few kilobytes, so the case is reachable; the phrase already names the cap, no single call owns the failure (§12.4's suffix rule), there is no operator text to shorten, and lowering the cap, the size move, would make it worse, so raising it is the one move that clears it.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §12.2 said the floor's `e` "opens on the saved chunk notes" without saying whether the operator's edits are written back to the file at the path — corrected: `e` edits a copy; the file keeps the chunk notes as the calls returned them, and the edited text lives in the run like any edit at the review gate — settled by: derivation — release's editor seam edits a temp copy and returns the text (`EditorLauncher.Edit`, `internal/engine/editor.go`), §11.4 defines the file's contents as each chunk's notes "exactly as the chunk calls returned them", and no step of the record rewrites the file once saved; an edit discarded by a later abort is what the review gate's own `e` does as built.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §12.2 and §12.3 settled only an empty `e`, leaving out an editor that cannot be launched or fails — corrected: no editor launched warns and returns to the `c` / `e` / `a` choice; any other editor failure is surfaced and aborts — settled by: derivation — the floor's `e` runs release's editor seam, which as built warns and returns `ErrEditorReturnToGate` for a missing editor and returns any other launch failure for the caller to surface and abort (`internal/engine/editor.go`), and the review gate re-presents itself or aborts on exactly those (`reviewGate`, `internal/engine/release.go`); the floor's choice is the gate it returns to.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §14.1 said regenerate's list leaves out "that version's release bookkeeping commit (`{commit_prefix} Release {tag}`, the commit the tag sits on), where one was made" without saying how it is recognised when `commit_prefix` has changed since the version was released — corrected: it is the commit the tag sits on when that commit's subject ends in ` Release {tag}`, whatever prefix comes before it; a tag on any other commit leaves the list whole — settled by: derivation — `record.BookkeepingSubject` builds the subject from the `commit_prefix` read at run time, so today's prefix cannot recognise an older version's commit, and the commit is made only when the changelog or version file changed (`record.BookkeepingWillCommit`), so the tag's position alone cannot say one was made; the tag's position and the ` Release {tag}` suffix together identify it under any prefix.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §12.2's earlier corrigendum said an editor failure at the floor "is surfaced and aborts", as "the review gate does as built", without saying whether the abort unwinds the pre-tag artifact commit, which the review gate's own non-decline errors as built do not — corrected: every way the floor ends a release before the push, `a`, an editor failure or stdin closing at the choice, unwinds as any failure before the push does, taking the pre-tag artifact commit with it — settled by: the record — CLAUDE.md's "Surgical unwind, then PONR" invariant (every pre-push mutation is tracked and unwound on failure), §6.6 ("A failure before the push unwinds as built, back to the starting commit and taking the pre-tag artifact commit with it"), and the as-built notes stage, whose failure under `abort` already goes through `surfaceAndUnwind` (`internal/engine/release.go`); the artifact commit lands at the pre_tag stage, before the notes and so before the floor.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §15.1 said "A failed `r` aborts the run whatever `on_notes_failure` says, as built" without saying whether that abort unwinds the pre-tag artifact commit, which as built it does not: `Release` routes only a review-gate decline through `Unwind` and returns a failed `r` as-is — corrected: on a release the abort unwinds as any failure before the push does, back to the starting commit, taking the pre-tag artifact commit with it — settled by: the record — CLAUDE.md's "Surgical unwind, then PONR" invariant (every pre-push mutation is tracked and unwound on failure), §6.6 ("A failure before the push unwinds as built, back to the starting commit and taking the pre-tag artifact commit with it"), and the §12.2 corrigendum, which settled every abort at the floor as an unwind on the same grounds; the review gate sits after the pre_tag stage, and a regenerate's `r` lands before its changelog commit, so it has nothing to reset.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §16.1 said "a chunked run keeps the single notes line and updates it as the run moves" and §15.1 let a single-pass `r` become a chunked run, without saying what an `r` shows while it works — corrected: an `r` that becomes a chunked run, over the cap at its size check or through halving, narrates as any chunked run does, in pretty and plain mode alike; an `r` that makes one call, the merge-only `r` or a single-pass `r` that stays one call, narrates as `r` does as built — settled by: derivation — §16.1's ground is that a chunked run can take minutes and a spinner that sits that long reads as hung, and its wording names a chunked run with no first-attempt limit; §15.1 has the ladder apply to an `r` "as to the first attempt"; a one-call `r` waits as long as a single-pass `r` does as built, so it keeps the as-built narration.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §15.1 said a single-pass `r` "re-runs the same AI path as built, with the nudge appended" and can be sent to chunking, without saying which calls of that chunked run carry the nudge — corrected: the nudge rides in the release's instructions, so every chunk, piece and halving-round call and the merge carry it — settled by: derivation — as built the nudge is appended to the instructions (`appendOneTimeContext`, `internal/notes/generate.go`), §8.6 has the release's instructions ride in every chunk call and the merge, and §15.1 counts the nudge in the prompt the size check measures, which makes it part of every call's fixed part.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §15.2 said "A chunked body is cached together with its per-chunk notes" and also "In a live run the per-chunk notes live only in memory and end with the run", though a real run writes the notes cache too — corrected: an entry holds the per-chunk notes whichever run wrote it, dry or real, so `r` on reused chunked notes re-runs only the merge whoever cached them; outside the cache entry the per-chunk notes live only in memory, and only a failure's saved file persists — settled by: measurement and derivation — `resolveBody` caches every freshly generated normal-AI body whatever the mode (`internal/engine/release.go`), and the cache is wired on real runs (`newNoteCache`, `cmd/mint/main.go`); §15.2's "A chunked body is cached together with its per-chunk notes" and "`r` on reused chunked notes re-runs only the merge" carry no mode limit, the cache keys on inputs, not on which run wrote it, and the entry's lifecycle bullets hold for any entry.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §15.1's earlier corrigendum said a failed `r` on a release unwinds, without saying whether an `r` that ends because stdin closed at its nudge prompt counts as a failed `r` — corrected: it does, as built, and unwinds the same way, taking the pre-tag artifact commit with it — settled by: the record — as built the `r` step treats closed input at its nudge read as one of its own failures, beside a nil regenerator and a failed regeneration (`regenerateBody`, and the `r` case's "Any failure (closed input, nil seam, regenerate error) is surfaced and aborts", `internal/engine/release.go`); CLAUDE.md's "Surgical unwind, then PONR" invariant and the §12.2 corrigendum, which unwinds stdin closing at the floor's choice on the same ground.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): the §12.2 corrigendum recorded that "the review gate's own non-decline errors as built do not" unwind, and the specification neither brought them under the unwind nor said they stay as built — corrected: every way the review gate ends a release before the push unwinds as any failure before the push does, taking the pre-tag artifact commit with it: a decline (as built), a failed `r`, a launched editor that fails at the gate's `e`, and stdin closing at the gate's keypress; a missing editor still warns and returns to the gate; regenerate's review gate lands before its changelog commit and has nothing to reset — settled by: the record — CLAUDE.md's "Surgical unwind, then PONR" invariant (every pre-push mutation is tracked and unwound on failure), §6.6 ("A failure before the push unwinds as built, back to the starting commit and taking the pre-tag artifact commit with it"), and the §12.2 and §15.1 corrigenda, which settled the floor's aborts and a failed `r` as unwinds on the same ground; as built `Release` (`internal/engine/release.go`) unwinds only a decline (`errGateAborted`) and returns an `e` failure surfaced through `surface(p, "edit", …)` and closed input from `ReviewDecision` without unwinding.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §15.1 said a failed `r` on chunked notes first saves the per-chunk notes, and §11.5 that a Ctrl-C "mid-wave" saves none, without saying whether a Ctrl-C during a merge-only `r`'s re-merge call, when no wave runs and every chunk's notes are complete, saves the file — corrected: a Ctrl-C during an `r`, the re-merge call included, is a cancel, not a failed `r`, and saves no chunk notes — settled by: derivation — §11.5 saves nothing on a Ctrl-C and keeps no resume state, and an interrupted chunked run "behaves like an interrupted single-pass one"; CLAUDE.md's AI seam holds that a cancel is not an AI failure and propagates unchanged (§9.5); §15.1 saves on a failed `r`, and the run's notes-cache entry still holds the chunk notes for the next run with the same inputs (§15.2).

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §6.5 said the pause shows "how many changed files it matches, and their total size" without saying which size is meant or which matched files count — corrected: the size is the total of the matched files' diff sizes, each measured as §6.2 measures a path; the files counted are the changed paths this release's notes diff still carries at that pause, so a path the built-in always-excludes, either `diff_exclude` level or an earlier Exclude answer in this run has taken out is not counted, and a path on the keep list is, a glob an earlier Keep answer in this run gave counting as on the keep list — settled by: derivation — §6.2 lists each path with "the size of its diff" and §6.3's condensed line in the same list, `vendor/: 4,312 files, 38 MB`, uses that measure; Exclude can take out only a file the notes still carry, so a file already out would misstate what the answer removes; §6.5 names "any of those files on the keep list" among the matches, so kept files are counted, and a Keep answer makes its glob join `[release].diff_keep` (§6.5); §5.2 has keep never override an exclusion, which is why the pause shows kept files.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §6.7 said that on an unattended run "each suggestion prints with its reason, what it matches, and the `[release].diff_exclude` line that would exclude it" without saying what "what it matches" holds — corrected: it is what the pause shows of the matches: how many changed files the glob matches and the total size of their diffs, and any of those files on the keep list (§6.5) — settled by: derivation — §6.5's pause is the only definition the record gives of what a suggestion shows about its matches, and §6.7 has a dry run show suggestions "as they would at the pause"; a full path list would run to thousands of lines for a folder glob such as `vendor/`, and the keep-list files are the warning the operator at the pause gets before excluding a kept file (§5.2).

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §6.7 said "**Dry run:** the suggestions show as they would at the pause; nothing is asked or written", while an unattended run prints them after the notes with the `[release].diff_exclude` line, without saying which rule an unattended dry run (`--dry-run -y`, or a non-TTY dry run) follows — corrected: the suggestions follow the dry run's own mode; a dry run at a terminal without `-y` shows them as they would appear at the pause, and an unattended dry run prints them after the notes as an unattended run does, the `[release].diff_exclude` line included; nothing is asked or written either way — settled by: derivation — a dry run previews the real run, and a real `-y` or non-TTY run never pauses; the §12.1 corrigendum settles the dry run at the floor the same way, by the dry run's own mode; an agent previewing with `mint release --dry-run -y` acts on the `[release].diff_exclude` line the real run will print.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §6.5 named the pause's two choices, Exclude and Keep, with keep the default, but not the keys that pick them — corrected: each is picked by its initial letter, `e` for Exclude and `k` for Keep, and a bare Enter picks the default, keep — settled by: derivation — §12.2 names each floor choice by its initial letter (`c`, `e`, `a`); each gate declares its own choice set and default (`internal/presenter/gate.go`, the `Gate` type), so the pause's `e` clashes with no other gate's `e`; as built, a deliberate empty Enter fires the gate's `Default`.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §6.5 said "A suggestion is skipped when earlier answers in this run already cover everything it matches", which, read with the corrigendum that counts kept files among a pause's matches, would still pause a suggestion whose only remaining matches are on the keep list — corrected: a suggestion is skipped when earlier answers in this run already cover every uncovered changed path it matches, so a suggestion whose only remaining matches are on the keep list raises no pause; kept files are counted and shown only at a pause that uncovered matches raise — settled by: derivation — §5.2 has a path `diff_keep` covers "never raised by the exclusion check"; §6.4 keeps a suggestion only while it matches at least one uncovered changed path, and an Exclude or Keep answer makes its glob join `[release].diff_exclude` or `[release].diff_keep` (§6.5), covering the paths it matches; the kept files a pause shows are the warning that comes with uncovered matches before Exclude takes a kept file out (§5.2), not a reason to ask about kept files alone.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §6.2 said the check lists each changed path "not already covered by the built-in always-excludes, the shared or `[release]` `diff_exclude`, or `diff_keep`", which leaves out the plain-mode `version_file` exclusion that §5.3 keeps apart from the built-ins, so a plain-mode version file changed in the range would be listed and could raise a pause counting no file the notes carry — corrected: a path the plain-mode `version_file` exclusion covers is not listed either, and the listed paths are the uncovered changed paths §6.4 and §6.5 refer to — settled by: derivation — the check asks which paths the notes should not read, and the notes diff never carries that file (§5.3, `versionFileExcludePathspec` in `internal/notes/assemble.go`); §6.1 skips the check when every changed path is "already covered by an exclusion or the keep list", any exclusion; an answer about that path would change nothing in the notes.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §6.5 and §6.6 settled "**Ctrl-C during the pauses** cancels the run and records nothing" but said nothing of stdin closing at a pause — corrected: stdin closing at a pause aborts the run before any tag, unwinding as any failure before the push does, and records nothing, since answers are written only once the last pause is answered; it is never read as the default, keep — settled by: derivation — the corrigenda to §12.2 and §15.1 treat stdin closing at the floor's choice and at the review gate's keypress as an abort that unwinds, on CLAUDE.md's "Surgical unwind, then PONR" invariant; CLAUDE.md seam 4 has `ErrInputClosed` surfaced by the caller and its "Fail loud, never hang" idiom forbids carrying on as if answered; §6.6 holds answers until the last pause, so none exists to keep, and reading closed input as keep would record a `[release].diff_keep` glob, "never raised again", that the operator never chose.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §6.7 said each unattended suggestion prints with "the `[release].diff_exclude` line that would exclude it" without saying what the line holds when `[release].diff_exclude` already has entries — corrected: the line is the key as an Exclude answer would leave it, `diff_exclude = [...]` under `[release]`, holding the entries the key holds now with the suggested glob appended, or the glob alone where the key is unset; each suggestion's line is worked out against the config as loaded, so it never carries another suggestion's glob — settled by: derivation — §6.6 has an Exclude answer append to an existing key, never overwrite, so the line that would exclude the glob is the key as that append leaves it; TOML rejects a key defined twice in one table and the config loads strictly (`config.Load`), so a line holding the glob alone, pasted beside an existing key, would break the next run's load, while the full line replaces the existing one and keeps the earlier exclusions.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §6.6 left open what happens when writing `.mint.toml`, or staging or committing it, fails — corrected: it is a failure before the push, surfaced with the write error or git's error verbatim beneath its line, as a failed pre-tag artifact commit is; the run aborts before any tag and unwinds as built, leaving `.mint.toml` as it was before the answers were written, and nothing is recorded — settled by: derivation — the answers commit is a pre-push mutation, and every other pre-push commit mint makes, the pre-tag artifact commit and the bookkeeping commit, is surfaced and routed through the surgical unwind on failure (`surfaceAndUnwind`, `internal/engine/release.go`); §6.6 already has "a failure before the push" unwind as built; its keep exception covers an answers commit that exists, and a failed write or commit made none; CLAUDE.md's "Surgical unwind, then PONR" (mint removes only what it created) has the unwind take back mint's own write.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §6.6 said "The unwind then re-commits the answers on top of the starting commit" and names the answers commit as kept, without covering that re-commit failing after the reset — corrected: one warning names the manual step that restores the answers, with git's error verbatim beneath it; the narration then does not name the answers commit as kept, and the unwind's summary reports manual cleanup required rather than a clean repo — settled by: derivation — as built, a failed reset or tag delete during the unwind warns with the exact manual step and git's error verbatim (`warnUnwindIncomplete`, `internal/engine/unwind.go`), and the summary drops `; repo clean` for `; manual cleanup required` (`summaryTail`) so the narration never contradicts the warning; naming the commit kept when it is not would be the false claim that rule exists to prevent.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §6.6 had the unwind reset to the starting commit and "then re-commits the answers on top of the starting commit", and covered the re-commit failing, but not the reset before it failing — corrected: when the reset fails, no re-commit is attempted, since the answers commit is still on the branch; the as-built warning naming the reset as the manual step names after it the step that restores the answers, because that reset takes the answers commit with it; the narration does not name the answers commit as kept, and the summary reports manual cleanup required, as built — settled by: derivation — a re-commit on a branch the reset never moved would duplicate the answers commit or commit nothing; the as-built reset warning (`warnUnwindIncomplete` via `resetCommitsIfMade`, `internal/engine/unwind.go`) gives a manual `git reset --hard {start}` that drops the answers along with mint's other commits, and following it would bring the same questions back on the next release, which §6.6's exception exists to prevent; the re-commit-failing correction already has a warning name the step that restores the answers, with no "kept" claim and `; manual cleanup required` (`summaryTail`).

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §6.6's bullets on the answers write or commit failing and on the unwind's re-commit failing said the failure is surfaced "with the write error or git's error verbatim beneath its line, as a failed pre-tag artifact commit is" and the warning comes "with git's error verbatim beneath it, as a failed reset or tag delete during the unwind warns", but neither cited site shows git's own message: the artifact commit's ✗ line carries the wrapped runner error with nothing beneath it, and the unwind's warning carries the exit-code text — corrected: what travels verbatim beneath the line is the write error, or git's own stderr; the cited sites are the precedent for the route (surfaced and unwound) and the manual-step warning, not for the output — settled by: CLAUDE.md's "Error & exit idioms": "Failure diagnostics from subprocesses (hook output, git stderr) travel VERBATIM in `StageFailure.Output`/`Warning.Output`"; the README's `-p` push warning passes "git's own stderr" through verbatim; the runner keeps git's stderr in `runner.Result`, not in the error (`internal/runner/exec_runner.go`).

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §6.6 said "The entry is appended to an existing key, the key is added to an existing table, the table is added, or the file is created holding only that table" without covering a `[release]` table the operator wrote inline (`release = { … }`) or with dotted keys (`release.version_file = …`), both of which `config.Load` accepts — corrected: such a table takes the answer in that form, inside the existing table, since TOML allows the table no second definition — settled by: derivation — TOML 1.0 forbids adding keys to an inline table from outside it and rejects a `[release]` header for a table dotted keys already defined, so adding a table would write a file the next run's strict load rejects, while rewriting the table into header form breaks "Everything else in the file is left as written"; "the key is added to an existing table" covers any form of that table.

> **Corrigendum 2026-10-19** (from `planning/token-aware-diff-sizing`): §6.6 had mint write and commit `.mint.toml` after the last pause, and §16.1 listed the notes stage's lines, with no line for recording the answers — corrected: the recording is narrated: once the answers commit lands, one line says the answers were written to `.mint.toml`'s `[release]` table and committed, a presenter event shown before the notes line resumes, and plain mode prints it as its own line; its wording is the implementer's — settled by: derivation — every commit mint makes before the tag is narrated as built: the pre-tag artifact commit inside the pre_tag stage's ✓ line, the bookkeeping commit in the record stage's line (`recordSentence`, `internal/engine/release.go`); a silent commit of the operator's config would leave them to find it in `git log`; CLAUDE.md seam 3 has the presenter render only events the engine emits, so the line is a new event.

> **Corrigendum 2026-10-20** (from `implementation/token-aware-diff-sizing`): §13.3 said the body "passes into the tag, changelog and provider release byte-identical" without saying how the tag annotation is cleaned or terminated; git's default `strip` cleanup drops a body's `#` lines and trailing whitespace, and `--cleanup=verbatim` alone leaves a body without a final newline glued to a signed tag's signature — corrected: the annotation is the bookkeeping subject, a blank line and the body as written under `--cleanup=verbatim`, ending in a newline that is added only when the body lacks one; that terminator belongs to the annotation, like the subject and blank line, not to the notes — settled by: measurement against git 2.56.0 (`man git-tag`: strip is the default and removes commentary; with `tag.gpgSign=true` a body without a final newline gives "no signature found" from `git verify-tag` and returns the signature block from `%(contents:body)`, while a newline-terminated body signs and round-trips) and the byte-identical-bodies invariant, which no cleanup mode but verbatim keeps.
