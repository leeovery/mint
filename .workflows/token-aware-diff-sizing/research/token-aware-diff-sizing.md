# Research: Token-Aware Diff Sizing & Graceful Oversized-Diff Handling

Explores how mint's notes engine should measure and react to large release diffs. Today the size guard counts diff lines against `max_diff_lines` and, under the default `on_notes_failure=abort`, dies with a bare "diff too large" message and no remediation hint. Two facets are in scope: a UX dead-end (no escape-hatch guidance on failure) and a correctness gap (line count is a crude token proxy — an under-limit diff can still overflow the model). This research explores feasibility of a budget/token-aware guard plus graceful degradation (escape-hatch messaging and/or chunk → summarise → map-reduce).

## Starting Point

What we know so far:

- **Prompted by:** An inbox idea about how mint handles oversized release diffs. Two problems:
  1. **Hard dead-end UX.** When the post-exclusion diff exceeds `max_diff_lines` (default 50000), `mint release` under default `on_notes_failure=abort` dies with `notes generation failed (diff too large): diff exceeds max_diff_lines (N > M)`. It names the cause + counts but gives no remediation hint — nothing points at the escape hatches (raise `max_diff_lines`, add `diff_exclude` globs, set `on_notes_failure=fallback`, or `--no-ai`).
  2. **Line-count is a crude token proxy.** The guard counts diff lines, but what must actually fit is the assembled prompt (diff + L1 context + system prompt) against the model's context window. An under-limit diff can still overflow the model and fail the AI call.
- **Already knows / desired direction (not decided):** Make the size guard budget/token-aware so the ceiling reflects the real model budget (estimate tokens across system prompt + context + diff). For genuinely large releases, degrade gracefully — surface escape hatches on failure, and/or chunk the diff, summarise each chunk, map-reduce the partials into final notes — instead of a hard abort. Prior art parked in tree: `internal/notes/size.go` comments reference a deferred "Change Map + trimmed diff" escalation (trim the diff to the ceiling instead of failing).
- **Starting point:** technical feasibility (token budgeting + chunk/summarise fallback) plus the UX of the failure path.
- **Constraints:** The guard is shared via `notes.CheckDiffSize` / `notes.ErrDiffTooLarge` and consumed by three callers — release, regenerate (per-release skip "diff too large"), and commit (a generate-SKIP). Any change to how size is measured/handled must account for all three. Confirmed a single coherent feature, not an epic.

---

## Code Grounding (as-built, 2026-06-26)

Read before the session so threads are concrete, not hypothetical.

**The guard.** `internal/notes/size.go` — `CheckDiffSize(diff string, maxLines int) error`. Pure function: counts diff lines (`countDiffLines`, trailing-newline-stable) and returns wrapped `ErrDiffTooLarge` when `got > maxLines` (INCLUSIVE boundary — exactly maxLines passes). It **rejects**, it does not trim. `maxLines` resolved by the orchestrator from `config.MaxDiffLines` (default `DefaultMaxDiffLines = 50000`). Input is already post-exclusion.

**The prompt that must actually fit.** `internal/notes/prompt.go` — `ComposePrompt(instructions, changeMap, diff)` joins, in order: `instructions` + `changeMap` + `diff` + `OutputReminder`, blank-line separated. So the real payload = resolved instructions (default ~50-line `DefaultPrompt`, or `[release].prompt` override, plus optional `[release].context`) + Change Map + the whole diff + reminder. The guard only measures the diff — not the instructions, Change Map, or reminder. (Note: `ComposePrompt`'s doc comment already *calls* the diff "capped", but nothing caps it today — aspirational language pointing at the parked trim idea.)

**The Change Map — already exists, cheap, size-independent.** `internal/notes/changemap.go` — `BuildChangeMapForRange` runs two cheap git calls (`--name-status`, `--numstat`) and renders a compact salience preamble: structural novelty (new dirs, renames, removals), churn-by-area rollup, notable files. It is METADATA, bounded by file *count* not diff *size*. This is the natural backbone for any degrade path: the parked "Change Map + trimmed diff" escalation = keep the full (small) map, trim/replace the (large) diff.

**Three consumers of the shared guard:**
- **Release (forward)** — `internal/notes/generate.go` `generateFromDiffWithContext` → `CheckDiffSize`; failure surfaced typed, `on_notes_failure` routing decided by caller (`internal/engine/release.go`).
- **Regenerate** — fresh single (`regenerate_fresh.go`, rides on `[release]`) surfaces like forward; batch `--all` (`regenerate_batch.go`) catches `ErrDiffTooLarge` and does a per-version skip-and-continue (non-terminal `Warn`).
- **Commit** — `internal/commit/generate.go` `CheckDiffSize(diff, cfg.MaxDiffLines)`; over-ceiling is a generate-SKIP (`ErrDiffTooLarge`), distinct from a transport failure → routes to `$EDITOR` fallback, never `StageFailed`.

**Relevant config (as-built):** `max_diff_lines` — shared-only top-level, default 50000 (`ai-model-selection` parked it shared-only "until a real need appears"). `diff_exclude` — shared-only globs. `[release].on_notes_failure` — `abort | fallback` (default `abort`); `[release].fallback` — fixed body string shared by `fallback` and `--no-ai`. Shipped AI default is now `claude -p --model sonnet` (per `ai-model-selection`).

**Sibling shipped work (do NOT re-tread):** `notes-failure-output-ugly-and-uninformative` already (a) carries claude's verbatim captured output (e.g. `Prompt is too long`) below the ✗ line via a typed carrier error, and (b) collapsed the redundant failure message. So the *rendering* of a failure is solved; what this unit adds on the UX facet is **remediation guidance** (escape hatches), a layer on top.

## Open Threads / Key Tensions

- **The model-opacity tension (crux for "token-aware").** `ai_command` is a raw command string deliberately supporting *any* AI (`ai-model-selection` dropped the driver pattern precisely so mint needn't know the AI). mint therefore does **not** know the configured model's context-window size, nor a reliable tokenizer for it. So "make the ceiling reflect the real model budget" runs straight into: mint can't introspect the budget. Open question for the session — does token-awareness mean (a) a better *byte/char*-based proxy than line count (no model knowledge needed), (b) an optional configured token budget the operator sets, (c) leaning on the AI's own "Prompt is too long" as the real signal and degrading *reactively*, or some mix? This is the first thing to put to the user.

### Thread: AI/model registry for context budget (user proposal, 2026-06-26)

**User proposal.** Keep a maintained library/registry mapping known AI + model → context-window budget. The operator picks a known entry (declares "I'm using Claude" / "Codex", plus the model), mint looks up the budget and becomes genuinely token-aware. Plus an override / add-your-own escape hatch for custom or unlisted models. Optionally seed the library by researching current AIs/models; "kept as a repository it can always be updated."

**Feasibility: yes, mechanically possible.** But it lands on contested ground and carries hazards research must resolve:

- **Distinction from the dropped driver (the hinge).** `ai-model-selection` *explicitly dropped* a driver/provider-registry — but that registry was about *how to invoke* each AI (command construction). This proposal is a registry of *budget metadata only*; invocation stays the raw `ai_command` string. So it is narrower and arguably orthogonal — NOT a straight reopen. Weigh this distinction deliberately; it decides whether this is a new idea or a reopened argument.
- **Drift hazard (two descriptions of the AI).** Today `ai_command` is the sole description of "what AI runs." A separate budget-model picker is a second description that can drift (`ai_command = claude -p --model opus` but picker still "sonnet"). Needs a reconciliation rule or the budget silently lies. Echoes `ai-model-selection`'s per-key drift worry (timeout vs command).
- **Staleness + no update channel.** A baked-in limits table goes stale every model release — the *exact* problem that drove `ai-model-selection` to pick the `--model sonnet` alias over a full versioned ID. Worse here: mint is a compiled binary with no network calls and no editable data-file channel today. "Always updatable repository" needs a concrete mechanism: rebuild-per-release, an external editable data file, or a network fetch (a large new seam mint deliberately lacks).
- **Budget is only the denominator.** Knowing the window (e.g. 200k) still leaves estimating the *prompt's* token count (numerator) — without the model's tokenizer that is a bytes/4 heuristic. Registry buys the budget, not the count.
- **Proactive vs reactive value.** The budget's job is to decide *when to degrade*. Reactive degradation (option c) gets the exact truth from the AI's "Prompt is too long" for free. Registry's value = *proactive* avoidance (don't fire a doomed slow/expensive call). Is that worth a maintained registry vs letting it fail then degrade?
- **Reframe.** A registry is essentially ergonomic sugar over a single operator-set budget number (option b: `context_budget` / `max_prompt_tokens`). Pick "claude-sonnet" → mint fills the number. Same "sugar over the raw command string" shape `ai-model-selection` deferred for the driver. Minimal-machinery core = one budget config key; registry is a convenience layer on top.

**External survey (declined as a deep-dive, 2026-06-26):** the factual question — current AIs/models' context windows, how stably/officially those limits are published, whether reliably discoverable, and how each signals prompt-overflow — stays in conversation for now. Still the key evidence that would tell us whether a maintained registry is viable; revisit if the registry path firms up.

### Thread: separate "the ceiling" from "what happens at it" (emerging framing, 2026-06-26)

The registry conflates two questions that are worth pulling apart:

- **(A) The ceiling** — *when* do we decide a diff is too big? Spectrum: line count (today) → byte/char count → estimated token budget → registry-derived per-model token budget. Increasing precision, increasing machinery + AI-coupling.
- **(B) The response** — *what happens* when it is too big? Spectrum: hard fail (today) → hard fail + escape-hatch guidance → graceful degradation (trim diff / Change-Map-only / chunk → summarise → map-reduce).

The registry is purely an answer to **(A)** — and it only earns its keep if **proactive** prediction matters (avoid firing a doomed slow/expensive call). If the real value is in **(B)** done well — especially **reactive** degradation triggered by the AI's own "Prompt is too long" — then (A)'s precision matters far less, and the opacity tension largely dissolves. Open question for the user: which is the actual goal — predicting the failure, or handling it gracefully?

**User steer (2026-06-26):** leaning toward (B) via chunking, to the point that "all the token-budget stuff goes out the window" if recursive splitting works. Registry/budget demoted to a *fallback* framing, not the spine. Not ready for the external deep-dive until the shape firms up.

### Thread: recursive bisection / chunk-and-stitch (leading direction, 2026-06-26)

**The idea (user).** Don't predict — split. If the diff is too big (or the AI rejects it), split it, summarise each piece, stitch the pieces into one coherent set of notes. If a piece still fails, split again — recurse on "simple math" until each piece fits. Refinements the user volunteered: split at sensible boundaries (files, not mid-line); maybe overlap (hand a bit of the diff back); maybe even **ask the AI itself** for acceptable split boundaries given the file list.

**Why it's attractive.** Sidesteps opacity entirely on the reactive path — no need to know the model's budget; the AI's failure *is* the signal. Bounded recursion (~log(size/limit)). Provider-agnostic if "any generation failure → split & retry."

**The one hard problem — stitching ≠ concatenation (the crux the user named).** Release notes are defined by *cross-cutting salience*: `DefaultPrompt` demands ONE bullet per notable change (not per file/hunk/commit), closely-related changes *combined*, ranked by the Change Map. Chunk-by-file destroys exactly that: a feature spanning 5 files across 3 chunks becomes 3 fragmented/duplicated bullets, each chunk blind to whether its slice is the headline or a footnote. So the "reduce" step is **not** a glue-join — it needs a second AI pass that re-synthesizes across the partial outputs (dedup, merge, re-rank). That reduce input is small (summaries, not diffs), so it almost always fits.
- **Map-reduce vs refine** are the two stitch shapes: (i) map each chunk → partial notes, then one reduce call over all partials + global Change Map; (ii) refine — sequential, each call gets notes-so-far + next chunk. (i) parallelizable but needs the extra reduce pass; (ii) no reduce pass but sequential/slow and order-biased. Naive concat is rejected (breaks salience/dedup).

**The Change Map is the spine that makes this survivable (key connection).** It already exists, is cheap, and is *size-independent* (built from name-status/numstat, bounded by file count). Hand the SAME global Change Map to every map call AND the reduce call → every chunk knows the whole release's shape and ranking even while seeing only its slice. This is a strong reuse-of-existing-machinery argument and arguably *why the Change Map was built*. Its area-grouping could also *inform* chunk boundaries (keep an area's files together).

**Hazards / open edges:**
- **Single-file floor.** Bisection by file boundary bottoms out at one file; if that *one* file's diff exceeds the limit (giant generated file, lockfile, vendored blob, the 867KB `.workflows/`/`.tick/` artifact case), file-splitting can't terminate. Forced choice: trim within the file, drop it (often it's exactly `diff_exclude` noise), or coarse-summarise. Recursion does not *always* converge via file-splitting alone.
- **Cost/latency inversion.** Today = ONE call. Chunking = N map calls + 1 reduce, each with its own fatal per-attempt `timeout` exposure. The user's "if it fails, it fails quickly" applies to the *detection*, but the *recovery* is slow/expensive. Acceptable as the exceptional big-release path, but the tradeoff must be explicit.
- **Architectural seam.** The transport is content-agnostic and must stay so — it knows nothing of diffs/chunking. So the chunk/split/retry orchestration lives in the *notes engine* above the transport, re-composing smaller prompts and re-calling. Reactive trigger choice: string-match "Prompt is too long" (provider-coupled, Claude-specific) vs "any generation failure → split" (provider-agnostic, but wasteful on non-size failures).
- **Determinism/testing tension.** Chunk count and call count become input-dependent and (if AI-driven boundaries) non-deterministic — harder to reason about and to pin in mint's exact-argv/exact-output test idiom. (Distinct from the "byte-identical body" invariant, which is about not reformatting a generated body, not generation determinism.)
- **"Ask the AI to split" — surprisingly sound.** You don't pass the full diff to get boundaries — you pass the *file inventory* (paths + line counts, ~the Change Map's data), which is tiny and always fits. The AI groups files into chunks "that fit your limits" — offloading BOTH the boundary intelligence (keep related files together) AND the budget knowledge to the one entity that actually knows its own window. Costs an extra round-trip and inherits AI non-determinism, but as a boundary heuristic it dominates naive line-bisection.

### Thread: prior art — is this a solved problem? (from training, NOT verified — deep-dive deferred)

User asked directly. From general knowledge (flagged for later verification; the external survey was declined for now):
- **Commit-based changelog tools sidestep the problem entirely.** git-cliff, release-please, semantic-release, conventional-changelog generate notes from *commit messages / conventional commits* — tiny input, never near a context window. mint is unusual in feeding the *diff* to an AI; that's the source of the size problem AND of mint's richer output.
- **AI PR-summary / code-review tools DO hit this and use map-reduce.** CodeRabbit, GitHub Copilot PR summaries, "What the Diff", etc. feed diffs to LLMs; common strategies are per-file summarise-then-aggregate (map-reduce), truncation, and file filtering. So map-reduce over diffs is an *established* pattern — the release-notes-specific *salience-preserving stitch* is the part that's ours to solve.
- **The pattern is canonical in LLM frameworks.** LangChain's "map_reduce" and "refine" document chains exist for exactly "input bigger than context." Nothing exotic here.
- **Token counting.** tiktoken (OpenAI), Anthropic's `count_tokens` endpoint. Context windows ARE publicly documented (e.g. Claude ~200k, GPT-4o ~128k, Gemini 1M+) but (a) change over time and (b) use different tokenizers per provider — so a portable estimate is ~chars/4, and exact counts need a provider call mint doesn't make. ⇒ confirms the registry/budget would be an *approximation*, and that a maintained table tracks moving targets.

### Thread: simple operator-set token budget (minimal (A), still on the table)

A single optional config key (`max_prompt_tokens` / `context_budget`) the operator sets, estimated via chars/4 over the *whole* composed prompt (instructions + Change Map + diff + reminder), not just the diff. Self-contained, no registry, no AI introspection; operator owns the number and can edit it. Could be the *trigger* for chunking rather than a hard fail. Public model-window data makes a sane default guessable but moving. Demoted by the user's chunking steer, but it is the cheapest "smarter ceiling than line count" and composes with chunking (it decides *when* to split).

---

## Triage

(none)
