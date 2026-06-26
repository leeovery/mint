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

## Triage

(none)
