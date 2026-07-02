# Discussion: Token-Aware Diff Sizing & Graceful Oversized-Diff Handling

## Context

Mint's notes engine guards oversized release diffs with a raw line count (`CheckDiffSize` in `internal/notes/size.go`, default `max_diff_lines = 50000`). Under the default `on_notes_failure = abort`, an over-ceiling diff dies with a bare "diff too large" message and no remediation hint. Two facets are in scope:

1. **UX dead-end** — the abort names the cause + counts but points at none of the escape hatches (raise `max_diff_lines`, add `diff_exclude` globs, `on_notes_failure = fallback`, `--no-ai`). A big release reads as a wall.
2. **Correctness gap** — line count is a crude token proxy. The thing that must fit is the *assembled prompt* (instructions + Change Map + diff + reminder) against the model window. An under-limit diff can still overflow and fail the AI call.

The guard is shared via `notes.CheckDiffSize` / `notes.ErrDiffTooLarge` and consumed by three callers — **release**, **regenerate** (per-version skip on `--all`, forward-like on single), **commit** (generate-SKIP → `$EDITOR`). Any change must account for all three. Confirmed a single coherent feature, not an epic.

### Research standing position (to ratify / resolve here)

Research (`research/token-aware-diff-sizing.md`, incl. two deep-dives + three review passes) has matured most of the design into *emerging directions the discussion phase must ratify*:

- **mint CANNOT fit-predict** — it can't see the model window, the tokenizer, *or* the `ai_command` CLI's own wrapper overhead (Claude Code's system prompt + tool defs, empirically ~800k tokens in one live case). So any proactive ceiling is a crude **policy** cap, never a fit predictor.
- **Trigger: reactive-primary.** An **AI-as-error-classifier** (ask the same AI to map its own captured error to one of a closed code set) is the standing front-runner trigger — provider-agnostic, dissolves the string-match portability wall. A crude **proactive byte-cap** survives only as a **backstop** for silent-truncating providers (Ollama) + claude's 10 MB stdin cap.
- **Response: a graceful-degradation ladder** — full single-pass → **map-reduce** (parallel map + ONE serial reduce, *not* refine) → concat partials → existing `on_notes_failure` floor. Change Map is the size-independent **spine** handed to every map + the reduce to preserve salience.
- **Config: auto-degrade silently by default.** A new `[release]`-scoped axis (`oversize = chunk|off`, name TBD), orthogonal to `on_notes_failure`.
- **Scope: chunking is a release-notes concept.** Commit stays as-is; regenerate `--all` inherits chunking uniformly (skip stays only as its floor); forward release + single regenerate chunk cleanly.
- **Overhead-dominated failures are NOT mint-fixable** — chunking shrinks mint's prompt but every chunk re-pays the overhead; mint can only surface + advise. `--bare` is off the table (breaks subscription OAuth, live-confirmed).

### References

- Seed: `seeds/2026-06-25-token-aware-diff-sizing.md`
- Research: `research/token-aware-diff-sizing.md`
- Discovery: `discovery/session-001.md`
- Code: `internal/notes/size.go`, `prompt.go`, `changemap.go`, `generate.go`; `internal/ai/transport.go`; `internal/commit/generate.go`; `internal/engine/release.go`

## Discussion Map

### States

- **pending** (`○`) — identified but not yet explored
- **exploring** (`◐`) — actively being discussed
- **converging** (`→`) — narrowing toward a decision
- **decided** (`✓`) — decision reached with rationale documented

### Map

  Discussion Map — Token-Aware Diff Sizing (6 subtopics, all pending)

  ┌─ ○ Detection trigger — proactive vs reactive [pending]
  │  ├─ ○ AI-as-error-classifier as primary trigger [pending]
  │  ├─ ○ Closed code-set: minimal vs rich granularity [pending]
  │  ├─ ○ Classifier prompt hardening (untrusted error text) [pending]
  │  └─ ○ Proactive byte-cap backstop + line→byte ceiling; token-budget/registry survival [pending]
  ├─ ○ Degradation ladder — the response [pending]
  │  ├─ ○ Map-reduce vs refine (parallel map + serial reduce) [pending]
  │  ├─ ○ Stitching / salience (Change Map as spine) [pending]
  │  ├─ ○ Chunk boundaries + single-file floor [pending]
  │  ├─ ○ Concat floor / abort as last rung [pending]
  │  └─ ○ Overhead-dominated failures (surface + advise) [pending]
  ├─ ○ Config surface — oversize knob [pending]
  │  ├─ ○ New axis vs on_notes_failure value; naming [pending]
  │  └─ ○ Default (silent auto-degrade) + scope [pending]
  ├─ ○ Scope across three consumers [pending]
  ├─ ○ Remediation UX / escape-hatch guidance [pending]
  │  ├─ ○ Attended vs unattended default [pending]
  │  └─ ○ Commit-message fallback reuse [pending]
  └─ ○ Implementation reality — parallelism [pending]
     ├─ ○ Bounded fan-out / failure aggregation [pending]
     ├─ ○ Progress narration model [pending]
     ├─ ○ Timeout semantics across N calls [pending]
     └─ ○ Cancellation / partial-coverage fork [pending]

---

*Subtopics are documented below as they reach `decided` or accumulate enough exploration to capture.*

---

## Summary

### Key Insights

*(captured as the discussion progresses)*

### Open Threads

*(captured as the discussion progresses)*

### Current State

- Nothing decided yet. Research standing positions above are candidates to ratify.

## Triage

(none)
