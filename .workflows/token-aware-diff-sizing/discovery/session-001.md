# Discovery Session 001

Date: 2026-06-26
Work unit: token-aware-diff-sizing

## Description (as of session)

Token-aware diff sizing plus graceful handling of oversized diffs for mint's notes engine, replacing the hard line-count fail with a budget-aware guard and a chunk/summarise fallback.

## Seed

- seeds/2026-06-25-token-aware-diff-sizing.md (inbox:idea)

## Imports

(none)

## Map State at Start

(n/a — single-topic work)

## Exploration

The work originated from an inbox idea about how mint handles oversized release diffs. Two facets framed the shape:

1. **Hard dead-end UX.** When the post-exclusion diff exceeds `max_diff_lines` (default 50000), `mint release` under the default `on_notes_failure=abort` dies with a message that names the cause ("diff too large") and the counts, but gives no remediation hint — nothing points the user at the available escape hatches (raise `max_diff_lines`, add `diff_exclude` globs, set `on_notes_failure=fallback`, or `--no-ai`). The failure reads as a wall.

2. **Line-count is a crude token proxy.** The size guard counts diff lines, but the thing that actually has to fit is the assembled prompt (diff + L1 context + system prompt) against the model's context window. So even an under-limit diff can overflow the model and fail the AI call anyway — the guard catches the extreme case but doesn't guarantee "this fits."

Desired direction (for later phases, not decided here): make the size guard budget/token-aware so the ceiling reflects the real model budget; and for genuinely large releases, degrade gracefully — surface the escape hatches on failure, and/or break the diff into chunks, summarise each, and map-reduce the partials into the final notes — instead of a hard abort. Noted prior art already parked in the tree: `internal/notes/size.go` comments reference a deferred "Change Map + trimmed diff" escalation.

Scope note carried forward: the guard is shared via `notes.CheckDiffSize` / `notes.ErrDiffTooLarge` and consumed by three callers — release, regenerate (per-release skip "diff too large"), and commit (a generate-SKIP) — so any change must account for all three.

Shape settled quickly: the user confirmed this is a single coherent feature, not an epic. The quick "tell the user how to escape the dead-end" fix and the larger token-aware-sizing-plus-chunking redesign read as two facets of one notes-sizing deliverable rather than independent topics.

## Edits

(none)

## Topics Identified

(none)

## Conclusion

Routed to research.
