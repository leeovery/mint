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

  Discussion Map — Commit Command (10 subtopics — 4 decided · 1 exploring · 5 pending)

  ┌─ ✓ Scope & relationship to the release pipeline (the framing fork) [decided]
  ├─ ✓ Commit flow / lifecycle (the stages) [decided]
  ├─ ✓ Staging model & `--all` (what gets committed) [decided]
  ├─ ✓ AI message generation (engine boundary, content source) [decided]
  ├─ ○ Commit message format & prompt (conventional vs emoji sections) [pending]
  ├─ ○ Interactive review gate (reuse of notes-review) [pending]
  ├─ ◐ Auto-push behaviour [exploring]
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

## AI message generation (engine boundary)

### Context

The framing decision said the AI message-generation concern is a shared engine both
verbs consume. This subtopic pins *where the git boundary sits* inside that engine —
the architectural seam that feeds the spec.

### Decision — a three-layer split, with git confined to Layer 1

- **Layer 1 — Context builder (git-aware).** Produces the content to describe.
  Parameterised by *source*: release uses `tag..HEAD`; commit uses the staged diff
  (`git diff --cached`). Applies `diff_exclude` globs + the `max_diff_lines` guard —
  identical logic for both verbs, so this is the genuinely shared git piece. (Different
  git *providers* are a separate axis, handled by the existing driver/provider setup,
  not by this layer.)
- **Layer 2 — AI message engine (git-UNAWARE, content-agnostic).** Inputs: an assembled
  prompt + the content + `ai_command`. Runs the transport, validates (non-empty / not an
  error / not a refusal), one retry, fail-loud per policy, returns the message body.
  **Knows nothing about git, diffs, tags, or commits — pure "context in, message out."**
- **Layer 3 — Per-verb glue.** Picks the L1 source, supplies the prompt + default format
  (release notes vs commit message), and decides the sinks. Where the verbs differ.

**The engine is content-agnostic — this is the load-bearing property.** The input being
a diff is incidental; L2 just sees "content." It doesn't matter whether that content is
a textual diff, an AST/semantic breakdown, or a human-written note — same engine. This
is what makes the separate **release-notes-quality** research thread cheap: enriching
the *input* (AST/semantic signal instead of a raw diff) swaps L1's output with **zero
change to L2**. The boundary was chosen partly *because* it absorbs that future work.

### Journey / rationale

- Confines git to one layer, mirroring the dependency-inversion discipline already locked
  for release (`CommandRunner` / `Publisher` / `Presenter` are all seams the engine never
  touches directly). A git-aware engine would be the lone exception that breaks the pattern.
- L2 is trivially testable — a string + a fake `ai_command`, assert message + retry/fail
  behaviour, no git fixtures.
- **Composition is still allowed.** Keeping the underlying pieces separate doesn't forbid
  a convenience wrapper (a local or exported function) that composes L1→L2→sink for a
  call site's ergonomics. Separation is about the *underlying pieces*, not banning a tidy
  facade over them.

### Prompt boundary (consistent with settled release model)

L3 owns prompt assembly; L2 receives the finished prompt. Mirrors release's settled
"mint always owns the prompt; `ai_command` is just transport" with the two-knob model
(context-inject + full-override). Commit gets its own *default* commit-message prompt and
its own context/override knobs — specifics deferred to the Commit message format & Config
subtopics.

### Decided in passing

- **Commit's content source = the staged diff** (`git diff --cached`). Working-tree/`-a`
  interplay (what gets staged before we diff) → Staging subtopic.

Confidence: high.

---

## Commit flow / lifecycle

### Context

Release is a seven-stage spine ending in an irreversible atomic push. Commit is a far
shorter local act. This pins the stage sequence so the other subtopics have a spine to
hang off.

### Decision — the commit flow

1. **Preflight (minimal)** — git repo present; **something to commit** after staging
   (see Staging). Push-related gates only if pushing. (Exact gate subset → Preflight subtopic.)
2. **Stage** — apply the staging flag (`-a`/`-A`) if given; otherwise use the index as-is.
3. **Build context (L1)** — filtered staged diff (`git diff --cached`, with `diff_exclude`
   + `max_diff_lines`).
4. **Generate (L2)** — the commit message (skipped under `--no-ai`; fallback → Format subtopic).
5. **Review gate** — same `Continue?` rendering as release, interactive only (→ Gate subtopic).
6. **Commit** — `git commit` with the message (via `git_safe`).
7. **Push (optional)** — only if `-p`/`--push` (or config) (→ Auto-push subtopic).

**Reversibility:** no point-of-no-return / atomic-push semantics — a commit is local and
reversible until pushed. *Open:* the partial-failure model (commit OK, push fails) is NOT
auto-unwind like release; what mint does/says on a failed push is tracked under Auto-push
(reviewer F6/F11).

Confidence: high on the shape; push-failure detail open.

---

## Staging model & `--all`

### Context

What goes into the commit. The user's actual habit is `git add .` (which **includes new
files**), but the natural flag to "copy from git" — `-a` — is git's `commit -a`, which is
**tracked-only** (no untracked). That mismatch is the whole decision. (mint runs from the
repo root, so `git add .` ≡ `git add -A` for its purposes — both sweep the whole tree
including untracked.)

| Command | Modified tracked | Deleted tracked | New/untracked |
|---|---|---|---|
| `git commit -a` / `git add -u` | ✅ | ✅ | ❌ |
| `git add .` (from root) / `git add -A` | ✅ | ✅ | ✅ |

### Options Considered

- **A — Faithful `-a` only.** `mint commit -a` = `git commit -a` (tracked-only); untracked
  requires a manual `git add .` first. Predictable, never sweeps stray files, but doesn't
  replace the user's `git add . && commit` one-liner.
- **B — Two faithful flags.** `-a`/`--all` = `git commit -a` (tracked); `-A`/`--add-all` =
  `git add -A` then commit (everything incl. untracked). Both letters map to git flags the
  user already knows; the "everything" sweep is explicit/opt-in.

### Decision — B (two faithful flags)

- **Default = staged-only.** Commit the index exactly as staged. Respects deliberate staging;
  mint never decides *what* goes in unless asked.
- **`-a` / `--all` = `git commit -a`** — tracked modifications + deletions, no untracked.
  Muscle-memory faithful.
- **`-A` / `--add-all` = `git add -A` then commit** — everything including untracked. This is
  the user's `git add .` habit in one shot.
- **Flags bundle:** `mint commit -Ap` = add-all + push, with a minted message — the headline
  ergonomic target.
- **Empty staging** (nothing to commit after staging) → **fail loud** ("nothing to commit"),
  never invoke the AI on an empty diff (reviewer F1 — the analogue of release's first-release
  guard). `-A`/`-a` that stage nothing land here too.

### Journey

The original local commit shell function did **not** do its own `git add` (commit-only). The
user consciously chose to *add* the staging affordance to mint — a deliberate enhancement over
the original, not a port. The `git add .` habit (untracked included) is what tipped the choice
from A to B: a faithful `-a` alone would silently drop new files and surprise the user, so the
explicit `-A` covers the everything-case without overloading `-a`.

Confidence: high.

---

## Auto-push behaviour

### Context

The command optionally pushes after committing. The user's target invocation `mint commit -Ap`
implies push is a **flag**, opt-in — not default.

### Working direction (exploring)

- **Push is opt-in via `-p` / `--push`** (default: no push). Possibly a config default too
  (→ Config subtopic). `-p` is free on this verb (release uses `-p` for `--patch`; per-verb
  flag meanings, like git subcommands) — **cross-verb `-p` collision noted for CLI surface**.
- **Open (reviewer F6/F11):** is auto-push commit's point-of-no-return, with its own
  pre/post-push failure handling, or fire-and-forget? "Reversible until pushed" implies push
  *is* the line, so its failure behaviour deserves an explicit decision. To resolve here.
- **Open:** push target / upstream handling (current branch → its tracking remote; what if no
  upstream is set?). To resolve here.

Confidence: direction clear (opt-in `-p`); failure model + upstream handling open.

---

## Summary

### Key Insights

*(captured as the discussion progresses)*

### Open Threads

*(captured as the discussion progresses)*

### Current State

- Discussion initialized; all subtopics pending. Seeded from the discovery shape and
  the settled release + presentation discussions.
