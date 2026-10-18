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

---

## Working Notes
