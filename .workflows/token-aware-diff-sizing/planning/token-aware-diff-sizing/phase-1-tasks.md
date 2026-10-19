# Phase 1: Lean default AI command and quote-aware parsing — 5 tasks

## token-aware-diff-sizing-1-1

### Task 1-1: Quote-aware `ai_command` splitting

**Problem**: As built, the transport splits `ai_command` into a program and arguments on whitespace, so no argument can carry a space and an empty argument disappears. The lean default command this phase ships (§2.1) needs both: one empty argument after `--tools` and a multi-word `--system-prompt`. An operator command that quotes an argument is mangled the same way today.

**Solution**: Split every `ai_command` into a program name and its arguments by POSIX shell quoting rules. That covers the shared top level, `[release]`, `[commit]` and the shipped default. The program still runs directly, never through a shell. The README documents the rules where `ai_command` is.

**Outcome**: A quoted, escaped or empty argument reaches the AI tool exactly as written. Nothing in the command is read as shell syntax. A command with no quotes or backslashes runs the same program and arguments as before.

**Acceptance Criteria**:
- [ ] Given an `ai_command` of `tool --system-prompt="Be terse" '$HOME' a\ b "" ''`, at any level (shared, `[release]` for a forward release or regenerate's fresh source, `[commit]` for `mint commit`), the program run is `tool` and its arguments are, in order: `--system-prompt=Be terse`, the literal `$HOME`, `a b`, and two empty arguments (§3.1)
- [ ] Given `tool 'a "b" \c $d'`, the tool receives the single argument `a "b" \c $d`: single quotes keep everything up to the next single quote literally (§3.1)
- [ ] Given `tool "a\"b" "c\\d" "e\$f" "g\x"`, the tool receives `a"b`, `c\d`, `e$f` and `g\x`. Inside double quotes a backslash escapes `"`, `\`, `$` and the backtick, and is otherwise kept as a literal character (§3.1)
- [ ] Given a command written across two lines with a backslash ending the first line outside quotes (`claude -p \`, then `--model opus`), the program is `claude` with the arguments `-p`, `--model` and `opus`. The backslash and the newline are removed as a line continuation (§3.1)
- [ ] Given `tool $HOME *.go ~/x a|b c;d e&f <in >out`, the program `tool` receives exactly those eight words, character for character. `$`, backticks, `*`, `~`, `|`, `;`, `&`, `<` and `>` are ordinary characters within a word. No variable expansion, command substitution, globbing, pipe or redirection happens, and no shell is started (§3.1)
- [ ] Given an `ai_command` with no quotes or backslashes, such as `claude -p --model opus` with runs of spaces and tabs between the words, the program and arguments run are exactly those the whitespace split ran before the change (§3.3)
- [ ] The README states `ai_command`'s quoting rules where `ai_command` is documented (§18.2)

**Do**:
- The quote-aware split replaces the transport's whitespace split of the command (`parseCommand` in `internal/ai/transport.go`). The program still runs directly through the `runner.CommandRunner` seam, never through a shell (§3.1).
- The README documents the quoting rules where `ai_command` is: the shared engine keys table and "The AI Transport" section (§18.2).

**Context**:
> The rules (§3.1):
> - Unquoted whitespace separates words.
> - Single quotes keep everything up to the next single quote literally.
> - Double quotes keep everything up to the next unescaped double quote. Inside them, a backslash escapes `"`, `\`, `$` and the backtick, and is otherwise literal.
> - Outside quotes, a backslash makes the next character literal.
> - A backslash followed by a newline is removed, as a line continuation.
> - Quoted and unquoted parts written together form one word.
> - A quoted empty string, `""` or `''`, is one empty argument.
> - Nothing else is shell syntax.
>
> Compatibility (§3.3): quotes never worked, so a working `ai_command` today contains none, and its words are unchanged under the new rules. Only a backslash in an existing command could change meaning. None of the user's repos sets `ai_command`.
>
> Task 1-3 rejects a command these rules cannot split (an unterminated quote, a trailing lone backslash, an empty first word) when config loads, using the same rules. CLAUDE.md seam #5 keeps `internal/ai` from importing `internal/config`, and config does not import ai. Whatever does the split must therefore be reachable from both without breaking that.
>
> §3.3 and §18.3 name the comments that describe a whitespace split and go stale with this change. They sit on `internal/ai/transport.go` (the `Config` command field, `Generate`, `parseCommand`), `internal/config/config.go` (the `AICommand` field, `AICommandFor`) and `internal/aitransport/aitransport.go` (`New`). CLAUDE.md keeps comments true to as-built in the same commit.
>
> The values in these criteria are the decoded `ai_command` string, not its TOML spelling. The README's examples write `ai_command` as a single-quoted TOML literal string, in which a backslash and a double quote are literal.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §3.1 The rules; §3.3 Compatibility; §18.2 The README (`ai_command`'s quoting rules); §18.3 CLAUDE.md and code comments

## token-aware-diff-sizing-1-2

### Task 1-2: Lean shipped default AI command

**Problem**: The shipped default `claude -p --model sonnet` carries Claude Code's own overhead into every call before mint's prompt begins: its system prompt, its tool definitions, and the repo's and the user's CLAUDE.md files. That was measured at 105,994 input tokens on a one-word prompt, against 628 for the lean form (§2.1). Every AI call mint makes, and every call the later phases add, uses this one command (§2.2). The overhead is also invisible to mint, so it eats into what a call can carry.

**Solution**: Make the lean command the shipped default `ai_command`. It applies wherever `ai_command` resolution falls through to the shipped default, and an explicit `ai_command` at any level is untouched. The README documents the lean command in full, what it drops on purpose, and where conventions belong now. The setup guide's AI-transport paragraph names it.

**Outcome**: A repo that sets no `ai_command` runs the lean command for release notes, regenerate's fresh notes and commit messages. A repo that sets one runs exactly what it set. The README, `mint setup`'s config reference and its AI-transport paragraph all show the lean command as the default.

**Acceptance Criteria**:
- [ ] Given a repo with no `.mint.toml`, when `mint release` writes notes, the program run is `claude` with exactly these arguments, in order (§2.1, §2.4):
  - `-p`, `--model`, `sonnet`
  - `--safe-mode`, then `--tools` followed by one empty argument
  - `--strict-mcp-config`, `--no-session-persistence`
  - `--system-prompt`, then `Follow the instructions you are given and output only what they ask for.` as one argument
- [ ] Given a `.mint.toml` that sets no `ai_command` at any level, `mint release regenerate <version>` from a fresh source and `mint commit` run that same program and arguments (§2.2, §2.4)
- [ ] Given an explicit `ai_command`, such as `[commit].ai_command = 'claude -p --model opus'` or a shared `ai_command = 'llm -m gpt-4o'`, each verb that resolves to it runs exactly that command, with none of the lean flags added (§2.4)
- [ ] Given a claude too old for a lean flag, one that exits non-zero rejecting `--safe-mode` as an unknown option, mint runs no version probe and no other command form. The AI tool's only invocations are the lean command's first attempt and the transport's one retry (§2.5)
- [ ] `mint setup`'s config reference shows the shared `ai_command` row defaulting to the lean command. Its AI-transport paragraph names the lean command where it named `claude -p --model sonnet` (§18.1)
- [ ] The README (§2.3, §18.2):
  - shows the lean command as the shared `ai_command` default, and no line still calls `claude -p --model sonnet` the shipped default
  - documents the lean command in full
  - says it drops CLAUDE.md files and Claude Code's own system prompt on purpose
  - says conventions an operator wants applied belong in the `[release]` and `[commit]` `context` and `prompt` keys

**Do**:
- The default is `config.DefaultAICommand` in `internal/config/config.go`, the single canonical source. The config-metadata SoT's shared `ai_command` Default cell is sourced from it, so `mint setup`'s config reference follows (§18.1).
- The setup guide's AI-transport paragraph is the "The AI transport" text in `overview()` in `internal/setupguide/setupguide.go` (§18.1).
- The README changes go in the shared engine keys table and "The AI Transport" section (§18.2).

**Context**:
> What each flag does (§2.1):
> - `--safe-mode` switches off CLAUDE.md files (the repo's and the user's global), skills, plugins, hooks and MCP servers. Auth and model selection keep working.
> - `--tools ""` drops the built-in tool definitions.
> - `--strict-mcp-config` loads only the MCP configuration passed on the command line, and none is passed.
> - `--no-session-persistence` stops claude saving each one-shot call to disk as a resumable session.
> - `--system-prompt` replaces Claude Code's own system prompt.
>
> The model is unchanged: Sonnet, in alias form. `--bare` is not used: it forces API-key auth and breaks the subscription login (§2.1). The system prompt names no task, because the same command serves every AI call mint makes, and each call's own prompt carries its job (§2.2).
>
> What it drops is intended (§2.3). mint owns the prompt and the command is only transport, so a call carries mint's instructions and nothing else. Instructions kept in CLAUDE.md that shaped release notes or commit messages stop doing so. The `context` and `prompt` keys ride in every release-notes and commit-message call.
>
> A claude too old for a flag rejects it as an unknown option and exits non-zero, landing on the normal generation-failure path (§2.5). mint adds no version probe and no fallback to the plain command, because either would put claude-specific knowledge into mint. Commit showing that message verbatim is task 1-5. Release already renders the AI's captured output beneath its ✗ line.
>
> The empty `--tools` argument and the multi-word system prompt reach claude intact only through task 1-1's quote-aware split (§2.1).
>
> The setup guide paragraph's other stale parts change elsewhere, each carried by its phase's acceptance: `default 60` in task 1-4, the cap and chunking in Phase 2, the commit-subject list as the one non-AI body in Phase 3, and the floor in Phase 5. Phase 5's floor guidance later points operators at the README's lean-command documentation for their own `ai_command` (§18.2).

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §2.1 The command; §2.2 One command for every call; §2.3 What it drops, on purpose; §2.4 Reach; §2.5 A claude too old for the flags; §18.1 The setup guide; §18.2 The README

## token-aware-diff-sizing-1-3

### Task 1-3: Reject a malformed `ai_command` at config load

**Problem**: An `ai_command` the quoting rules cannot split has no program and arguments to run. The three cases are an unterminated quote, a trailing lone backslash and an empty first word. Left to call time, the mistake would read as an AI failure and end in a non-AI body or the editor, with nothing naming the config slip (§3.2).

**Solution**: Reject a malformed `ai_command` at any level when config loads, before any step runs. The error takes the existing `invalid .mint.toml: …` shape and names the key and the cause. A blank or whitespace-only value is not malformed: it still counts as unset and falls through to the next level, as built.

**Outcome**: A malformed `ai_command` at any level stops every command that loads config at once, with a message naming the level and what is wrong. A blank value keeps falling through, so nothing that loads today stops loading.

**Acceptance Criteria**:
- [ ] Given a `[release].ai_command` with an unterminated quote, such as `claude --system-prompt "Be terse`, each of `mint release`, `mint release regenerate <version>` and `mint commit` fails before any step runs with `invalid .mint.toml: release.ai_command has an unterminated quote`, and makes no AI call (§3.2)
- [ ] Given a shared `ai_command` that ends in a lone backslash, such as `claude -p \`, loading fails with `invalid .mint.toml: ai_command ends in a lone backslash` (§3.2)
- [ ] Given a `[commit].ai_command` of `'' --flag`, whose first word is empty, loading fails with `invalid .mint.toml: commit.ai_command names no program`. A non-blank value whose split yields no words at all, such as a lone backslash-newline, fails with `names no program` too (§3.2, Corrigendum 2026-10-19 on §3.2)
- [ ] The message names the level the bad value sits at and its cause, for every combination. The key reads `ai_command`, `release.ai_command` or `commit.ai_command`. The cause reads `has an unterminated quote` (single or double), `ends in a lone backslash` or `names no program` (§3.2)
- [ ] Given a malformed `[commit].ai_command` and nothing else wrong, `mint release` still fails at load with the `commit.ai_command` message, though release never runs that command (§3.2)
- [ ] Given a well-formed command whose empty argument is not its first word, such as `claude --tools ""`, config loads (§3.2)
- [ ] Given a blank or whitespace-only `ai_command` at any level, config loads and that level counts as unset (§3.2, §2.4):
  - a blank `[commit].ai_command` under a shared `ai_command = 'llm -m gpt-4o'` makes `mint commit` run `llm -m gpt-4o`
  - with every level blank or unset, the shipped default runs

**Do**:
- The check runs in `config.Load` (`internal/config/config.go`) beside the existing `validateHooks` and `validateOnNotesFailure` checks. Its errors take the same `invalid .mint.toml: …` shape those rejections use (§3.2).
- A non-blank value counts as malformed exactly when task 1-1's split rules cannot split it, or split it into no words or an empty first word (§3.1, §3.2).

**Context**:
> Why at load (§3.2): a malformed command is never left to fail at call time. There it would read as an AI failure, reach the failure classifier (which runs the same command), take the OTHER path, and end in a non-AI body or the editor.
>
> Blank values (§3.2, Corrigendum 2026-10-19): a blank or whitespace-only `ai_command` is not malformed. As built, `AICommandFor` in `internal/config/config.go` trims each candidate and skips a blank one, so resolution falls through `[verb] → shared → shipped default`. That stays. "Names no program" covers a non-blank value whose split yields no words or an empty first word.
>
> The three levels a `.mint.toml` can set are the shared top-level `ai_command`, `[release].ai_command` and `[commit].ai_command`. The error's key reads `ai_command` for the shared level and `release.` / `commit.` plus the key for the verb tables, as the spec's three messages show.
>
> The values in these criteria are the decoded `ai_command` string, not its TOML spelling.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §3.2 Malformed commands fail at load; §2.4 Reach; Corrigenda (2026-10-19, §3.2)

## token-aware-diff-sizing-1-4

### Task 1-4: 180-second shipped release timeout

**Problem**: Release-notes calls share commit's 60-second shipped per-attempt deadline. Phase 2 lets a release-notes call grow to the 1.5 MB cap, and a call at the cap should take about a minute (§4.5), so a 60-second floor would time out calls the cap allows. The cap's default and the release timeout's default are chosen together, so that a call at the cap finishes well inside the timeout (§4.4).

**Solution**: Give release-notes calls their own shipped per-attempt default of 180 seconds, resolving `[release].timeout` → shared `timeout` → 180s. Regenerate rides on it, as it does on `[release].ai_command`. `mint commit` keeps the shipped 60s. The README tables, the config-metadata rows behind `mint setup`'s config reference, and the setup guide's AI-transport paragraph all name both defaults.

**Outcome**: With no timeout configured, a hung release-notes call fails after 180 seconds and a hung commit call after 60. An explicit timeout at either level still wins. Every surface that states the shipped timeout names both values.

**Acceptance Criteria**:
- [ ] Given no `timeout` at any level, whether there is no `.mint.toml` or one that sets no `timeout`, a release-notes call that hangs fails as `AI timed out` after 180 seconds. That holds on a forward release and on `mint release regenerate <version>` from a fresh source (§4.4)
- [ ] Given the same config, a `mint commit` AI call that hangs fails after 60 seconds and goes to the editor fallback as built (§4.4)
- [ ] Given `[release].timeout = 90`, or a shared `timeout = 90` with no `[release].timeout`, a hung release-notes call fails after 90 seconds, not 180. A shared `timeout = 90` with no `[commit].timeout` gives `mint commit` 90 seconds too (§4.4)
- [ ] `mint setup`'s config reference shows the `[release].timeout` row with a 180-second default, and the shared `timeout` row naming both shipped defaults: 180 seconds on release and 60 on commit (§18.2)
- [ ] The README's `[release]` table shows `timeout` with a 180-second default, and its shared `timeout` row names both shipped defaults. Every README line that states the timeout's default names both, including the `default 60` mentions in "The AI Transport" section (§18.2)
- [ ] `mint setup`'s AI-transport paragraph names the 180s release and 60s commit timeouts where it said `default 60` (§18.1)

**Do**:
- Resolution lives in `config.TimeoutFor` (`internal/config/config.go`). `aitransport.New` sources it at all three wiring sites, and regenerate's fresh source passes `config.VerbRelease` (§4.4).
- The SoT rows are in `config.MetadataRows()` (`internal/config/metadata.go`), and `mint setup`'s config reference renders from them (§18.1, §18.2).
- The README changes go in its shared engine keys table, its `[release]` table and "The AI Transport" section. The setup guide paragraph is the "The AI transport" text in `overview()` in `internal/setupguide/setupguide.go` (§18.1, §18.2).

**Context**:
> §4.4: `mint release` gets its own shipped per-attempt timeout default of 180 seconds. `mint commit` keeps the shipped 60s, because its calls stay small and a hung one should not take minutes to fail.
>
> §4.5: on the lean command, 1,500,000 bytes is about 675k tokens. Reading the prompt is fast enough that a call's time is mostly the notes being written. A call at the cap should take about a minute.
>
> As built, the zero-config path (no `.mint.toml`) seeds the shared `timeout` with 60 seconds in `defaults()` (`internal/config/config.go`). So that case resolves through the shared level today, not through `TimeoutFor`'s floor.
>
> The spec leaves the as-built per-level value rules alone: an explicit `timeout = 0` means no per-attempt deadline and stops resolution at that level, and a negative value drops through to the next level.
>
> §18.3 names the comments that state a single 60s default, which go stale with this change. They sit on `internal/config/config.go` (`DefaultTimeout`, the `Timeout` fields, `defaults()`, `fileShape`, `TimeoutFor`, `resolveTimeout`), `internal/ai/transport.go` (`Config`, `NewTransport`) and `internal/aitransport/aitransport.go` (`New`, which also names the old default command). CLAUDE.md keeps comments true to as-built in the same commit.
>
> The README tripwire test checks only that each schema key name appears somewhere in the README, so it does not catch a stale default (§18.2).

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §4.4 The release timeout; §4.5 The default's grounds; §18.1 The setup guide; §18.2 The README; §18.3 CLAUDE.md and code comments

## token-aware-diff-sizing-1-5

### Task 1-5: Commit's AI failure carries the AI's verbatim output

**Problem**: As built, any AI generation failure on `mint commit` drops the AI's captured output. The attended warning and the unattended failure carry only mint's own line. Release renders that output verbatim under its ✗ line, so commit breaks the rule that subprocess failure diagnostics travel verbatim (§17.1). It matters for any commit failure: an expired login, a usage limit, or a claude too old for the lean flags this phase ships. That last one fails every commit call, and only claude's own message says why (§2.5).

**Solution**: Carry the AI's captured output verbatim beneath commit's AI-failure warning on an attended run, and beneath its fail-loud line on an unattended one. A failed `r` regeneration at the review gate carries it the same way. The editor-fallback triggers that are not AI failures keep their lines and carry no AI output.

**Outcome**: Whenever an AI call fails on `mint commit`, whoever reads the output sees what the tool said beneath mint's line, whether or not anyone is at the terminal.

**Acceptance Criteria**:
- [ ] Given an attended `mint commit` whose AI tool exits non-zero on both attempts with a message, such as a claude rejecting `--safe-mode` as an unknown option, the run shows `could not generate a message, opening editor` with the tool's captured output verbatim beneath it, then opens the editor as built (§17.1, §2.5)
- [ ] Given the same failure on an unattended run (`-y`, or a non-TTY stdin), the run fails with `no AI message and no interactive editor available`, with the tool's captured output verbatim beneath it (§17.1)
- [ ] Given a tool that writes its message to stderr behind an empty or whitespace-only stdout, that message shows beneath the line on both runs, as it does beneath release's ✗ line (§17.1)
- [ ] Given `r` at the review gate whose regeneration fails the same way, the warning shows the regeneration call's captured output verbatim beneath it before the editor opens (§17.1)
- [ ] Given `--no-ai`, a diff over the size limit, or an all-excluded diff, the editor-fallback lines are unchanged and carry no AI output (§17.1):
  - an over-limit diff still shows `diff too large to summarise — opening editor`, and an all-excluded diff `all changes are excluded from the AI context — opening editor`
  - an unattended run fails with `no AI message and no interactive editor available` and nothing beneath it

**Do**:
- The work lives in `internal/commit/run.go`: the AI-failure branch of `Run`, the failed regeneration in `reviewLoop`, and the no-message-source failure in `runEditorFallback` (§17.1).

**Context**:
> §17.1: the unattended run is the one with nobody present to re-run it, so it needs the cause most.
>
> As built, the transport keeps a failed call's two streams in `ai.GenerationError` (`Stdout`, `Stderr`; `internal/ai/transport.go`), which wraps `ai.ErrGenerationFailed`. The transport captures output only on a generation failure, so a timeout or a missing binary carries none. Release composes the two streams for its ✗ line in `notesFailureOutput` (`internal/engine/release.go`). `internal/engine` and `internal/commit` are sibling packages, and neither imports the other (CLAUDE.md layout).
>
> CLAUDE.md's error idioms: failure diagnostics from subprocesses travel verbatim in `StageFailure.Output` / `Warning.Output`, never summarised or reworded. The presenter renders either beneath its line.
>
> `r` at the gate is interactive-only, so a failed regeneration is always on an attended run.
>
> What commit does not get (§17.2): no classify call, because every commit failure already ends in the editor, and no chunking, exclusion check or message file. Its shipped timeout stays 60s (task 1-4).
>
> The over-limit trigger is `max_diff_lines` today. Phase 2 replaces it with `max_prompt_bytes`, and the note text stays `diff too large to summarise — opening editor` (§4.3).

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §17.1 The AI's output, verbatim; §17.2 What commit does not get; §2.5 A claude too old for the flags; §4.3 Per consumer
