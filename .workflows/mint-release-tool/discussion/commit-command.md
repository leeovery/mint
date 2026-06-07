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

  Discussion Map — Commit Command (10 subtopics — 10 decided)

  ┌─ ✓ Scope & relationship to the release pipeline (the framing fork) [decided]
  ├─ ✓ Commit flow / lifecycle (the stages) [decided]
  ├─ ✓ Staging model & `--all` (what gets committed) [decided]
  ├─ ✓ AI message generation (engine boundary, content source) [decided]
  ├─ ✓ Commit message format & prompt (Conventional Commits) [decided]
  ├─ ✓ Interactive review gate (reuse of notes-review) [decided]
  ├─ ✓ Auto-push behaviour [decided]
  ├─ ✓ Preflight & safety for commit [decided]
  ├─ ✓ Config schema additions (verb-namespaced shape) [decided]
  └─ ✓ CLI surface & flags [decided]

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

Key property (refined during the Gate subtopic): **mint mutates nothing until the user accepts
the gate.** Everything before accept is read-only — including the `-a`/`-A` staging, which is
*deferred to the accept path*. This is what makes abort a true no-op.

1. **Preflight (minimal)** — git repo present; **something to commit** (for `-a`/`-A`, the
   would-be-staged changes; else the existing index). Computed read-only. Empty → fail loud.
2. **Build context (L1)** — filtered diff of what *would* be committed (default: `git diff
   --cached`; with `-a`/`-A`: the would-be-staged working-tree diff, computed **without**
   mutating the index), with `diff_exclude` + `max_diff_lines`.
3. **Generate (L2)** — the commit message (skipped under `--no-ai`; fallback → Format subtopic).
4. **Review gate** — same `Continue?` rendering as release, interactive only (→ Gate subtopic).
5. **On accept** — apply `-a`/`-A` staging now (if given), then `git commit` (via `git_safe`).
6. **Push (optional)** — only if `-p`/`--push` (or config) (→ Auto-push subtopic).

**Reversibility:** no point-of-no-return / atomic-push semantics — a commit is local and
reversible. Before accept, nothing has been mutated (clean abort). After accept, a completed
commit is never unwound by mint (partial-failure model under Auto-push, reviewer F6/F11).

Confidence: high.

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
- **Staging is deferred to gate-accept** (see Gate subtopic). With `-a`/`-A`, mint computes the
  would-be-committed diff *read-only* for message generation, and only runs `git add` after the
  user accepts. So aborting an `-a`/`-A` run leaves the index exactly as it was — mint never
  leaves a half-staged worktree behind.
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

### Decision

- **Push is opt-in via `-p` / `--push`** (default: no push). May also have a config default
  (→ Config subtopic). `-p` is free on this verb (release uses `-p` for `--patch`; per-verb
  flag meanings, like git subcommands) — **cross-verb `-p` collision noted for CLI surface**.
- **Push failure → keep the commit, warn clearly, do NOT unwind (reviewer F6/F11).** On a
  failed push (rejected, remote moved, no upstream, network), mint leaves the commit in place
  and reports clearly with the fix (re-run the push). Rationale: a push is a trivially
  repeatable manual fix, whereas unwinding the commit is *messy and risky* — the user may have
  had files staged before running `mint commit`, and resetting/unstaging could clobber that
  pre-existing state. So push is **not** treated as an atomic point-of-no-return with unwind;
  it's a best-effort final step whose failure is reported, not repaired.
- **Upstream handling:** defer to git — `mint commit -p` runs a normal `git push` (current
  branch → its configured upstream). No upstream set → git's own failure, surfaced via the
  warn-clearly rule above ("commit is in place; set an upstream and push"). mint adds no
  special upstream logic.

### Invariant established — *mutate nothing until accept; never unwind after*

The push-failure decision plus the gate-abort refinement give one coherent rule:

- **Before gate-accept, mint mutates nothing** — staging (`-a`/`-A`) is deferred to accept, so
  abort returns the user to their exact pre-`mint` state (their own prior staging untouched).
- **After accept, mint never unwinds a completed commit** — on a failed push it leaves the
  commit and reports clearly; it never unstages, resets, or rewrites.

This is the deliberate opposite of `mint release`'s auto-unwind model. The reason is the same
staging-safety concern: a local commit verb must never risk the user's working/staged state.
There is no destructive cleanup path at all — failures either left nothing behind (pre-accept)
or leave a clean forward-only commit the user can act on manually (post-accept).

Confidence: high.

---

## Preflight & safety for commit

### Context

Release's preflight is a strict gate set (clean tree, on release branch, remote in sync,
tag free, gh auth). A commit is a frequent, low-stakes, *local* act — most of those gates
are actively wrong for it. This pins commit's minimal preflight (reviewer F2/F9).

### Decision — commit's preflight is minimal

Commit runs only:

1. **Git repo present** — anchored at the repo root (same resolution as release).
2. **Something to commit** — after staging; empty → fail loud (decided in Staging).

**Gates commit deliberately DROPS (and why):**

- **Clean-working-tree — dropped.** Commit exists *to* operate on a dirty tree; the release
  gate is the direct opposite of commit's purpose. (Resolves the F2 head-on collision.)
- **On-release-branch — dropped.** Commits legitimately happen on feature branches all day.
- **Remote-in-sync (behind/diverged) — dropped.** You commit while behind origin constantly;
  blocking that would be absurd. (Resolves F9.)
- **No pre-push gate even with `-p`.** Consistent with the auto-push decision — mint doesn't
  gate the commit on push-ability; it attempts the push and *reports* failure. No remote-sync
  precheck.

This makes commit's safety posture the inverse of release's: release forces a known-good,
clean, in-sync starting state because it's high-consequence; commit assumes a messy in-progress
tree because that's its entire reason to exist.

Confidence: high.

---

## Commit message format & prompt

### Context

Release's default format (TL;DR + emoji-headed sections) is wrong for a commit. This pins
commit's default output format and its `--no-ai` behaviour. L3 owns this prompt (per the
engine-boundary decision); it's just a different default from release's.

### Decision — default format = Conventional Commits

- **Conventional Commits 1.0.0** (the formal standard, conventionalcommits.org) is the default
  output: `type(scope): description` subject line (imperative, concise), optional blank line +
  wrapped body for the *why*. Chosen because the user's own repos already use conventional
  commits (`discussion(...)`, `chore(...)`, `docs(...)`).
- **AI infers the `type`** (feat/fix/chore/docs/…) from the diff — central to the format and
  reliably inferable.
- **Scope off by default** — scope conventions are project-specific and the AI guesses them
  inconsistently; better omitted than wrong. (Re-enabling/guiding scope is a prompt/config
  affordance if ever wanted.)
- **Two-knob override**, mirroring release: a commit-specific context-inject knob + a full
  prompt-override knob (exact key names → Config subtopic). Same "mint owns the prompt;
  `ai_command` is transport" model.

### `--no-ai` fallback (reviewer F4)

- **`--no-ai` = behave like plain `git commit`** — drop to `$EDITOR` with an empty/template
  message and let the user write it. No AI, no synthetic stub. This is the natural fallback for
  a commit verb (unlike release, whose `--no-ai` builds a commit-subject-list body).
- **Same path on AI generation failure** — if the AI errors/returns nothing usable (after the
  engine's one retry), fall back to the `$EDITOR` path rather than abort. Low-stakes and
  friendly; the user is at the terminal anyway.

### Decided in passing

- **commit does NOT use release's `commit_prefix` (🌿) (reviewer F7).** A conventional-commit
  message is plain `type(scope): …`; forcing a 🌿 emoji onto *every* commit is undesirable.
  `commit_prefix` stays a release-only concern (release commit + tag subject). Commit messages
  carry no mint branding in their text.

### `max_diff_lines` exceeded → `$EDITOR` fallback (reviewer F5)

When the staged diff exceeds `max_diff_lines`, commit does **not** abort (release's
notes-failure model is too harsh for a routine large commit). Instead it falls back to the
same **`$EDITOR` path** as `--no-ai` / AI-failure, with a clear note ("diff too large to
summarise — opening editor"). One consistent degradation path for all three "no AI message"
cases: deliberate skip, generation failure, oversized diff. (`diff_exclude` still applies first,
so excluded noise doesn't push a diff over the limit.)

Confidence: high.

---

## `$EDITOR` fallback path semantics

### Context

Three decided paths converge on "drop to `$EDITOR`": `--no-ai`, AI-generation failure, and
oversized diff (all under Commit message format). The final review (set 002) found this path
was never walked against the deferred-staging model, the gate, and the `-y`/non-TTY posture
(F1–F4). This section pins it.

### Decision — F1: the `$EDITOR` fallback requires a TTY

`$EDITOR` is inherently interactive. When a fallback fires under **`-y` or non-TTY stdin**
(e.g. `mint commit -Apy --no-ai`, or `-Apy` when the AI fails / the diff is oversized), mint
**fails loud** ("no AI message and no interactive editor available") — it never hangs or commits
an empty message. This extends the gate's forbidden-combo philosophy (unattended + needs-human →
fail loud) to the editor path. Rationale: an unattended run with no message source is
contradictory — `--no-ai` unattended has nothing to commit with, and an unattended user wants
the AI anyway. **No `-m/--message` escape** (kept minimal — anyone needing unattended-with-own-
message uses plain `git commit`; mint commit is for *minted* messages).

Confidence: high.

---

## Interactive review gate

### Context

Whether/how commit reviews the message before it lands, reusing the cli-presentation
`Continue?` gate. Two reviewer concerns: the gate's abort semantics for commit (F3) and the
default posture given commit's higher invocation frequency (F10).

### Decision — reuse the `Continue?` gate, ON by default

Reuses the cli-presentation gate rendering (`y`/`n`/`e`/`r`, Enter ⇒ accept). Choice mapping
for commit:

- **`y` / accept** → stage (if `-a`/`-A`) then commit; then push if `-p`.
- **`n` / abort** → do nothing. **No auto-unwind needed** — nothing has been mutated yet
  (staging deferred to accept), so abort is a true no-op back to the pre-`mint` state.
  (Resolves F3 — commit's abort has nothing to roll back, unlike release's.)
- **`e` / edit** → edit the message in `$EDITOR`, used verbatim.
- **`r` / regenerate with context** → re-run the AI with a one-time context line. This *is* the
  "context injection" affordance from the user's original commit shell function.

**Posture: gate ON by default (F10).** Interactive runs show the message + `Continue?`; `-y`
skips it (auto-accept); the shared forbidden-combo rule applies (non-TTY stdin + no `-y` →
fail loud). Chosen for consistency with release + the presentation model, and because seeing
the minted message before it sticks is the point. The frequent one-liner stays fast via `-y`
(`mint commit -Apy`).

- **Considered — gate OFF by default** (commit immediately, review opt-in): faster for the
  frequent case, but commits messages unseen — the exact pain release's gate was built to kill.
  Rejected; `-y` already covers the unattended case explicitly.

### The gate-abort refinement (key design correction)

Originally the flow staged `-a`/`-A` *before* the gate. The user flagged that aborting would
then leave a mint-altered worktree — wrong: "abort" should mean the whole run is abandoned with
no trace. Fix: **mint mutates nothing until accept** (staging deferred). This is now a
cross-cutting property — see Commit flow, Staging, and the never-unwind invariant under
Auto-push.

Confidence: high.

---

## Config schema additions

### Context

What new `.mint.toml` keys commit needs — and, prompted by it, the right config *shape* now
that mint is multi-verb. The user clarified "consistent" means *the best implementation,
made coherent* — not "copy release's existing flat layout." So the shape itself is in play.

### Journey — flat `commit_*` → verb-namespaced tables

First pass added flat `commit_context`/`commit_prompt` keys to mirror release's flat
`notes_*` keys. On reflection (user-prompted), flat-with-prefixes crowds the namespace as
verbs multiply and hides which keys are shared vs verb-specific. With two verbs live and a
third (future) flagged, the better shape is **verb-namespaced tables + shared engine keys**.

### Decision — shared engine keys at top + a table per verb

```toml
# Engine-level — shared by every verb
ai_command     = "claude -p"
diff_exclude   = ["skills/**/knowledge.cjs", "*.min.js"]
max_diff_lines = 50000

[release]
tag_prefix       = "v"
commit_prefix    = "🌿"
release_branch   = "main"
changelog        = true
publish          = true
context          = "..."      # was notes_context
prompt           = "..."      # was notes_prompt
on_notes_failure = "abort"
# version_file, version_pattern, provider, ...

[commit]
context = "Conventional Commits; dev-workflow toolkit."   # inject into the commit prompt
prompt  = ".mint/commit-prompt.md"                        # full prompt override

[hooks]
pre_tag = "npm ci && npm run build"
```

Why this is the better implementation:

- **Shared vs verb-specific is structural, not inferred from prefixes** — `ai_command` /
  `diff_exclude` / `max_diff_lines` sit at the top *because* they serve every verb.
- **The verbs become symmetric** — both `[release]` and `[commit]` carry plain `context` /
  `prompt`; the table disambiguates, so no `notes_`/`commit_` prefixing is needed.
- **Scales** — a future `mint <verb>` is a new table, not more flat prefixes.

**Reused (shared) keys:** `ai_command`, `diff_exclude`, `max_diff_lines` — same values serve
both verbs. **Commit-specific:** `[commit].context`, `[commit].prompt`.

**Deliberately NOT added:** no push config (push is flag-only `-p`, never a default); no
`on_notes_failure` analogue for commit (failure path is always the `$EDITOR` fallback); no
scope toggle or per-verb `ai_command`/`max_diff_lines` override (steer via `[commit].context`/
`prompt`; promote to a `[commit]` key only if a real need appears).

### Cost / reconciliation owed

This **revises release's already-concluded flat config layout** — `notes_context` →
`[release].context`, `notes_prompt` → `[release].prompt`, and every flat release key moves
under `[release]`; the shared engine keys lift to the top. Cheap now (no code exists), far
cheaper than after implementation. Recorded as a **spec hand-off** (see Summary) — the
in-progress release spec absorbs it, the same way it owes the cli-presentation reconciliations.

Confidence: high.

---

## CLI surface & flags

### Context

Consolidation of every flag named across the discussion into commit's surface, plus the
dry-run question (reviewer F8) and whether a one-time context flag / a shim are warranted.

### Decision — the surface

```
mint commit [flags]

  -a, --all          stage tracked changes first (git commit -a semantics)
  -A, --add-all      stage everything incl. untracked first (git add -A)
  -p, --push         push after committing (no push without this; no config default)
      --no-ai        skip AI; drop to $EDITOR
  -y, --yes          skip the review gate (auto-accept)
      --plain        plain output — global presentation flag, all verbs
```

Bundles: `mint commit -Ap` (add-all + push, gate shown) · `mint commit -Apy` (unattended).
`-p` = push is per-verb (release's `-p` = `--patch`); **cross-verb `-p` divergence is
intentional and acceptable** (git subcommands carry their own flag meanings).

### Resolved

- **No `--dry-run` (reviewer F8).** Dropped consciously. The review gate already *is* the
  preview-then-bail affordance (see the message, `n` aborts with zero mutation), and a commit
  is cheap to `--amend`. Release needs dry-run because it previews a whole irreversible
  pipeline; commit has no such pipeline.
- **No `--context` one-time-context flag.** The original shell function had it, but the user
  has never used it. Interactive `r` (regenerate-with-context) at the gate plus the
  `commit_context` config cover the need. Dropped (YAGNI).
- **No `commit` shim.** `release` gets a per-project shim for muscle memory + `mint` delegation;
  `commit` is invoked directly as `mint commit` (the user aliases it personally if desired).

Confidence: high.

---

## Summary

### Key Insights

1. **Designed clean, not retrofitted.** No code exists, so the shared AI machinery is designed
   up front to serve both verbs — commit is not squeezed into release-note generation.
2. **The AI engine is content-agnostic (the load-bearing seam).** A three-layer split confines
   git to Layer 1 (context builder), keeps Layer 2 a pure "context in, message out" engine, and
   puts per-verb prompt/source/sinks in Layer 3. Because L2 never knows it's a diff, the future
   release-notes-quality work (AST/semantic input) swaps L1 with zero L2 change.
3. **Commit is the inverse of release on safety.** Release forces a clean, in-sync starting
   state (high-consequence); commit assumes a messy tree (that's its purpose) and drops the
   clean-tree / branch / remote-sync gates entirely.
4. **Mutate nothing until accept; never unwind after.** Staging (`-a`/`-A`) is deferred to
   gate-accept, so abort is a true no-op back to the user's pre-`mint` state; a completed commit
   is never unwound (failed push → keep commit, warn). One coherent rule replacing release's
   auto-unwind — chosen to never risk the user's working/staged state.
5. **One degradation path for "no AI message":** `--no-ai`, AI failure, and oversized diff all
   fall back to the normal `$EDITOR` commit — never abort.
6. **Multi-verb forces config namespacing.** Two live verbs (third flagged) tip the config from
   flat-with-prefixes to shared-engine-keys + a table per verb — symmetric, structural, scalable.

### Open Threads

- None outstanding for commit itself — all 10 subtopics decided.
- The separate **release-notes-quality** research topic remains; the content-agnostic engine
  boundary was deliberately shaped to absorb whatever it concludes (enriched L1 input).

### Spec hand-offs (reconciliation owed by the in-progress release spec)

1. **Config restructure → verb-namespaced shape.** Adopt shared engine keys at top +
   `[release]` / `[commit]` / `[hooks]` tables. Migrate release's flat keys under `[release]`
   (`notes_context`→`[release].context`, `notes_prompt`→`[release].prompt`, etc.). See Config
   subtopic.
2. **Shared AI engine = the three-layer split.** The release spec should express notes
   generation through the same L1/L2/L3 layering so commit and release literally share L1
   (context builder + `diff_exclude`/`max_diff_lines`) and L2 (the engine).
3. **Gate semantics already owed by release** (cli-presentation's `[a]/[q]`→`Continue?`
   reconciliation) apply to commit's gate too — commit consumes the same rendering.

### Current State

- **All 10 subtopics decided.** Commit is fully shaped: a thin standalone verb on a shared,
  content-agnostic AI engine; staged-diff input with `-a`/`-A` staging deferred to accept;
  Conventional Commits output with `$EDITOR` fallback; minimal preflight; on-by-default review
  gate (mutate-nothing-until-accept); opt-in `-p` push with no-unwind failure handling; flat
  `mint commit` surface (no dry-run/context-flag/shim); verb-namespaced config. Ready for
  specification, pending the release-spec reconciliations above.
