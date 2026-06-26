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

---

## Triage

(none)
