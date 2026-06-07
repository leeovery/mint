# Discussion: Commit Command

## Context

`mint commit` is a sibling verb to `mint release` — an AI-generated commit message
from the diff, built into the `mint` binary rather than living as a per-machine
shell function. It was **parked in the first discussion** as "its own separate
feature," with the `mint <verb>` namespace adopted precisely to leave room for it;
the pivot of mint from a single feature to an **epic** is the trigger that was
flagged for promoting it. This discussion designs it.

**Shape (from discovery):** the command wraps the user's existing AI-commit shell
function — an AI-generated commit message from the (staged) diff, with `--all`,
`--no-ai`, one-time context injection, and auto-push. "Minting a commit" fits the
brand. The user wants it **built into mint**, sharing:

- **the AI engine** — `claude -p` transport, mint-owned prompt, fail-loud + retry,
  `--no-ai` skip (see the release discussion's *AI release notes — skeleton*);
- **the `.mint.toml` config** — typed, optional, fail-loud (see *Config format & schema*);
- **the styled presentation layer** — the event-oriented `Presenter` seam, `pretty`
  vs `plain` by `isatty(stdout)`/`--plain`, `-y` orthogonality, the review gate
  rendering (see the *cli-presentation* discussion).

The integration details — **how much it reuses the release pipeline versus stands
alone** — were deliberately left for this discussion. That's the framing fork below.

### What's settled elsewhere (not re-litigated here unless commit forces it)

- The `mint <verb>` namespace, the `Presenter` seam + pretty/plain + `--plain`/`-y`,
  the AI-engine skeleton (`ai_command`, prompt-ownership, fail-loud, retry, `--no-ai`),
  lock-resilient git (`git_safe`), and the TOML config model all exist already.
  `commit` *consumes* these; it should not redesign them.

### References

- [mint-release-tool discussion](mint-release-tool.md) — the engine, AI-notes skeleton, config schema, lock-resilient git, lifecycle spine `commit` may or may not reuse
- [cli-presentation discussion](cli-presentation.md) — the `Presenter` seam, pretty/plain, review-gate rendering, `--plain`/`-y` that `commit` inherits
- [Discovery session 002](../discovery/session-002.md) — where `commit` was promoted to its own epic topic

## Discussion Map

### States

- **pending** (`○`) — identified but not yet explored
- **exploring** (`◐`) — actively being discussed
- **converging** (`→`) — narrowing toward a decision
- **decided** (`✓`) — decision reached with rationale documented

### Map

  Discussion Map — Commit Command (10 subtopics — 1 decided · 1 exploring · 8 pending)

  ┌─ ✓ Scope & relationship to the release pipeline (the framing fork) [decided]
  ├─ ○ Commit flow / lifecycle (the stages) [pending]
  ├─ ○ Staging model & `--all` (what gets committed) [pending]
  ├─ ◐ AI message generation (engine boundary, diff base) [exploring]
  ├─ ○ Commit message format & prompt (conventional vs emoji sections) [pending]
  ├─ ○ Interactive review gate (reuse of notes-review) [pending]
  ├─ ○ Auto-push behaviour [pending]
  ├─ ○ Preflight & safety for commit [pending]
  ├─ ○ Config schema additions [pending]
  └─ ○ CLI surface & flags [pending]

---

*Subtopics are documented below as they reach `decided` or accumulate enough
exploration to capture. These seeds are a starting point, not a fixed agenda — the
map grows and converges as we go. The framing fork (scope vs the release pipeline)
is the natural place to start, since most other subtopics cascade from it.*

---

## Scope & relationship to the release pipeline

### Context

`mint release` is a seven-stage spine ending in an irreversible `git push --atomic`
(tag + commits). `mint commit` is a far smaller act: stage → generate a message →
maybe review → commit → maybe push. The fork: how much machinery does commit share
with release, and does it ride the release spine or stand on its own?

The decisive framing: **there is no code yet.** Nothing is being "squeezed into"
release-note generation — we design the shared pieces clean to serve both verbs from
the start. So this is not "retrofit commit onto release"; it's "design the engine
boundary correctly the first time."

### Options Considered

- **A — Thin standalone verb** borrowing primitives (AI engine, Presenter, git_safe)
  but with its own diff logic, prompt, gate wiring. Release spine stays release-only.
- **B — Shared "AI-message-from-diff" core** that both verbs call, differing by diff
  source + prompt + sinks.

### Decision

**A in spirit, B applied narrowly — but designed fresh, not extracted.** `commit` is
a thin standalone verb that does NOT ride the release spine. It shares the genuinely
common primitives, and the AI message-generation concern is designed *up front* as a
shared engine both verbs consume (rather than retrofitted later). The guiding caution
(user): don't assume the *existing release design* needs changing — it doesn't; we're
designing the new shared seam, not reworking settled release decisions.

**What commit reuses:** the `Presenter` seam (pretty/plain, `-y`, `--plain`), the AI
engine (transport `claude -p`, mint-owned prompt, fail-loud + retry, `--no-ai`),
`git_safe` lock-resilient git, and the TOML config model.

**What commit does NOT touch:** version detection, tags, changelog, publish/provider,
the regenerate command. No point-of-no-return / atomic-push semantics — commit is
inherently local and reversible until pushed.

**Core behaviour:** take what's staged → generate a commit message for it → optionally
commit → optionally push (flags govern commit/push). A `-a`/add-staging flag is noted
but deferred to the Staging subtopic (possible scope-creep flag).

### Decided in passing

- **`diff_exclude` and `max_diff_lines` apply to commit too.** We don't want to feed an
  excluded file (bundle, lockfile, minified output) into commit-message generation any
  more than into release notes. The exclusion + size-guard logic is shared. (Whether
  commit reuses release's *config values* verbatim or can override them → Config subtopic.)

Confidence: high on the framing; the engine *boundary* (git-aware vs context-agnostic)
is the live question under AI message generation.

---

## Summary

### Key Insights

*(captured as the discussion progresses)*

### Open Threads

*(captured as the discussion progresses)*

### Current State

- Discussion initialized; all subtopics pending. Seeded from the discovery shape and
  the settled release + presentation discussions.
