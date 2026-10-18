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

---

## Working Notes
