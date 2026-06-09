---
phase: 3
phase_name: $EDITOR Fallback — Unified No-AI Degradation Path
total: 5
---

## commit-command-3-1 | approved

### Task commit-command-3-1: Resolve the editor via git's resolution order

**Problem**: All three "no AI message" cases (`--no-ai`, AI-generation failure, oversized diff) drop to an editor, and the editor must be the *same one `git commit` would open*. That means mint cannot just read `$EDITOR` — it must follow git's own resolution order (`GIT_EDITOR` → `core.editor` → `$VISUAL` → `$EDITOR` → git's built-in default). Critically, `$EDITOR` being unset is **not** by itself an error on a TTY, because git's default still applies. Without a shared resolver, the fallback tasks (3-2/3-3/3-4) and the unattended fail-loud task (3-5) have no single, correct way to find an editor — and Phase 4's `e` gate action would have to re-derive the same logic.

**Solution**: A standalone editor-resolution helper (in commit's package, e.g. `internal/commit`'s `ResolveEditor`) that returns the launchable editor command following git's precedence: `GIT_EDITOR` wins over everything; then `core.editor` (read via `git config`/`git var`); then `$VISUAL`; then `$EDITOR`; then git's built-in default. It returns the resolved editor command/argv, or a distinguished **not-launchable signal** when no candidate in the chain resolves to a launchable program. The resolver only *resolves and validates launchability* — it does not open the editor, apply staging, or commit; consumers (3-2..3-5, and Phase 4's `e`) decide what to do with the result.

**Solution note**: The cleanest, most git-faithful way to get this exact precedence is to ask git itself — `git var GIT_EDITOR` returns the very editor `git commit` would launch, already honouring `GIT_EDITOR` → `core.editor` → `$VISUAL` → `$EDITOR` → built-in default. Prefer delegating to `git var GIT_EDITOR` (via the consumed `CommandRunner`) over hand-rolling the precedence chain, so mint stays byte-for-byte aligned with git and inherits future git changes for free. The "not-launchable" determination is about whether the resolved program can actually be executed (e.g. the named binary is absent / not on `PATH`), not about `$EDITOR` being unset. This task builds the resolver **only** — it does NOT build the fallback drop (3-2), failure routing (3-3), oversized routing (3-4), the unattended fail-loud (3-5), or Phase 4's `e`/`r` gate actions. Build it cleanly as shared logic Phase 4's `e` action will reuse.

**Outcome**: Given `GIT_EDITOR` set, the resolver returns it regardless of `core.editor`/`$VISUAL`/`$EDITOR`. Given `GIT_EDITOR` unset but `core.editor` set, it returns `core.editor` over `$VISUAL`/`$EDITOR`. Given only `$VISUAL` and `$EDITOR` set, it returns `$VISUAL`. Given `$EDITOR` **unset** (and nothing higher in the chain), it returns git's built-in default — an unset `$EDITOR` is **not** an error on a TTY. When **no** candidate in the chain resolves to a launchable program, it returns a distinguished **not-launchable** signal (a typed sentinel/error the callers branch on), not a launch attempt and not a panic.

**Do**:
- In commit's package (e.g. `internal/commit`), implement `ResolveEditor(...) (editor, error)` (or returning a small result + a distinguished not-launchable sentinel) that yields the editor command `git commit` would open.
- Prefer delegating the precedence to git itself: run `git var GIT_EDITOR` via the consumed `CommandRunner` and use its output as the resolved editor — this already encodes `GIT_EDITOR` → `core.editor` → `$VISUAL` → `$EDITOR` → built-in default. (If a non-git formulation is used instead, it MUST reproduce this exact precedence, including reading `core.editor` from `git config` and falling to git's built-in default — but the `git var` route is the recommended, git-faithful one.)
- Treat an **unset `$EDITOR`** as a normal path to git's default, not an error: resolution succeeds and returns the default editor. Do not surface "unset `$EDITOR`" as a failure.
- Determine **launchability** of the resolved program: if no candidate in the chain resolves to a launchable program (e.g. the resolved binary cannot be found/executed), return a **distinguished not-launchable signal** (typed error/sentinel) that callers can branch on — distinct from a successfully-resolved editor. The resolver does not itself decide fail-loud vs graceful-degrade (3-5 / Phase 4 decide that from the signal).
- Do NOT open/launch the editor here, do NOT apply staging or commit, and do NOT implement any of the three fallback drops or the unattended fail-loud — this task is resolution + launchability signalling only.
- All git interrogation (`git var`, any `git config` read) goes through the consumed `CommandRunner` seam; tests script the precedence inputs and the not-launchable case via the fake runner / env.

**Acceptance Criteria**:
- [ ] `GIT_EDITOR` set resolves to it over `core.editor`, `$VISUAL`, and `$EDITOR`.
- [ ] `core.editor` set (no `GIT_EDITOR`) resolves to it over `$VISUAL` and `$EDITOR`.
- [ ] `$VISUAL` set (no `GIT_EDITOR`/`core.editor`) resolves to it over `$EDITOR`.
- [ ] `$EDITOR` **unset** (and nothing higher) resolves to git's built-in default — **not** an error on a TTY.
- [ ] When no candidate in the chain is launchable, the resolver returns a **distinguished not-launchable signal** (not a launch attempt, not a panic).
- [ ] The resolver does not open the editor, stage, or commit — resolution/launchability only.
- [ ] All git interrogation goes through the consumed `CommandRunner`/fake.

**Tests**:
- `"GIT_EDITOR wins over core.editor, $VISUAL, and $EDITOR"`
- `"core.editor wins over $VISUAL and $EDITOR when GIT_EDITOR is unset"`
- `"$VISUAL wins over $EDITOR when nothing higher is set"`
- `"an unset $EDITOR resolves to git's built-in default and is not an error"`
- `"no launchable editor in the chain returns the not-launchable signal"`
- `"the resolver does not launch, stage, or commit"`

**Edge Cases**:
- `GIT_EDITOR` wins over all.
- `core.editor` over `$VISUAL`/`$EDITOR`.
- `$VISUAL` over `$EDITOR`.
- Unset `$EDITOR` falls to git default (not an error on TTY).
- None in chain launchable → not-launchable signal.

**Context**:
> $EDITOR Fallback — Path Semantics: "Editor resolution (applies to *every* editor mint opens — both this fallback path and the gate's `e` action). mint resolves which editor to launch using git's own resolution order (`GIT_EDITOR` → `core.editor` → `$VISUAL` → `$EDITOR` → git's built-in default), so it opens whatever `git commit` would open. `$EDITOR` being unset is therefore *not* by itself an error on a TTY — git's default still applies. mint opens the editor itself (rather than delegating to `git commit`) because staging must be deferred until the save-as-accept event." "When no editor in the chain resolves to a launchable program, behaviour depends on whether a message candidate already exists: Fallback path (no message yet) → fail loud … `e` gate action (a message already exists) → graceful degrade." This task builds only the shared *resolution* and the not-launchable *signal*; the consumers decide fail-loud vs graceful-degrade. Phase 4's `e` gate action reuses this resolver.

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "$EDITOR Fallback — Path Semantics → Editor resolution".

## commit-command-3-2 | approved

### Task commit-command-3-2: `--no-ai` drops to the editor with save-as-accept

**Problem**: `--no-ai` must skip AI entirely and let the user write the message in their editor, behaving like plain `git commit` — but reconciled with mint's deferred-staging model. The editor save **is** the accept event on this path (there is no separate `Continue?` gate — the gate is AI-path-only), so a non-empty save must apply the deferred `-a`/`-A` staging (from 2-3) **then** commit, while an empty/aborted editor must be a **true no-op** (no staging, no commit, nothing mutated). Crucially, mint must open the editor **itself** (not delegate to `git commit`), because staging is deferred until the save event. Without this, `--no-ai` either commits unseen, stages at the wrong time, or leaves a half-staged tree on abort.

**Solution**: Add the `--no-ai` flag to the `mint commit` subcommand and, when set, **skip L3 generate (1-3)** and route straight to the editor fallback: resolve the editor via 3-1, open it **directly from mint** against the real (unstaged) state with an empty/template buffer, and treat the save as the accept event. On a **non-empty** save, apply the mode's deferred staging (2-3: `-a` → `git add -u`, `-A` → `git add -A`, default → no `git add`) **then** commit via the consumed `git_safe` with the saved message — in that order. On an **empty save or an aborted/quit editor**, do nothing: no staging, no commit, no mutation — a true no-op. No synthetic stub message is inserted into the buffer.

**Solution note**: The editor *resolution* is consumed from 3-1; the deferred-staging-then-commit ordering is consumed from 2-3; the `git_safe` commit sink is consumed from 1-4. This task wires `--no-ai` to **skip generation** and use the editor-as-accept path. It establishes the **save-as-accept** semantics that 3-3 (AI-failure) and 3-4 (oversized) reuse unchanged. There is **no** separate `Continue?` gate on this path — do NOT render the AI-path gate here. The `-y`/non-TTY forbidden-combo fail-loud and the not-launchable-editor fail-loud are 3-5 (this task assumes a TTY with a launchable editor); build the happy path here and let 3-5 layer the unattended/no-source fail-loud over it. `-p` push is Phase 5 — the spec says `-p` push *does* run after an editor save-as-accept, so this save-accept path must **not preclude** push (it is a full accept), but this task does **not** implement push. Empty-staging under `--no-ai` is still the 2-4 preflight concern reached before the editor (no editor opens on an empty would-be-staged set) — this task assumes a non-empty would-be-staged set.

**Outcome**: `mint commit --no-ai` (on a TTY, with a launchable editor and a non-empty would-be-staged set) opens the user's editor **directly via mint** (resolved by 3-1) with an empty/template buffer and **no AI call**. On a **non-empty save**: under `-a` mint runs `git add -u` then commits; under `-A` runs `git add -A` then commits; under the default mode commits the **existing index unchanged** (no `git add`) — all with the saved text as the commit message, staging-before-commit ordered, via `git_safe`. On an **empty save or quit/abort**: mint does nothing — no `git add`, no commit, no mutation; the index is exactly the pre-`mint` state. The editor is opened by **mint itself**, never delegated to `git commit`.

**Do**:
- Add `--no-ai` as a boolean flag on the `mint commit` subcommand (`cmd/mint`, the surface 1-4 added; reserved in earlier phases). When set, the orchestrator (`internal/commit` `Run`) **skips L3 generate (1-3)** entirely — no `ai_command`/`claude` invocation.
- Route to the editor fallback: resolve the editor via 3-1 (`ResolveEditor`), then **open it directly from mint** (via the consumed `CommandRunner`) against the real, unstaged state with an **empty/template** buffer. Do **not** delegate to `git commit` to open the editor — mint opens it itself because staging is deferred to the save-as-accept event. Insert **no synthetic stub** message.
- Treat the **save as the accept event** (no separate `Continue?` gate — the gate governs the AI-generated message only):
  - **Non-empty save** (the buffer has a non-whitespace message body) ⇒ accept: apply the mode's deferred staging (consumed 2-3: `All` → `git add -u`; `AddAll` → `git add -A`; `StagedOnly` → no `git add`) **then** `git commit` via the consumed `git_safe` with the saved message — strictly stage-then-commit.
  - **Empty save** (buffer empty / whitespace only) **or aborted/quit editor** ⇒ true no-op: no `git add`, no commit, no mutation. The index is exactly the pre-`mint` state.
- Determine "empty" the way git does for the editor flow: a buffer with no non-comment, non-whitespace content is empty ⇒ abort. (Mirror plain `git commit`'s empty-message-aborts behaviour.)
- Establish save-as-accept as a small reusable routine the AI-failure (3-3) and oversized (3-4) tasks call into unchanged — the only difference there is *why* the editor opened, not *what* save/abort does.
- Do NOT render the AI-path `Continue?` gate on this path. Do NOT implement `-p` push (Phase 5) — but do not structure the save-accept so push is impossible to add (a non-empty save is a full accept). Do NOT implement the `-y`/non-TTY or not-launchable fail-loud (3-5).
- Tests use the fake runner (scripted editor exit + saved buffer) + recording presenter: assert no AI call under `--no-ai`, non-empty save stages (per mode) then commits in order, empty/quit save mutates nothing, default mode commits the index with no `git add`, and the editor is launched by mint (a `CommandRunner` editor launch is recorded) — not via `git commit`.

**Acceptance Criteria**:
- [ ] `--no-ai` skips L3 generate — no `ai_command`/`claude` invocation is recorded.
- [ ] mint opens the editor **itself** (a direct editor launch is recorded), not via `git commit`.
- [ ] The editor buffer starts empty/template — **no synthetic stub** message is inserted.
- [ ] A **non-empty save** applies the mode's `-a`/`-A` staging **then** commits, in that order (via `git_safe`).
- [ ] Under the **default mode**, a non-empty save commits the existing index unchanged (no `git add`).
- [ ] An **empty save** is a true no-op — no staging, no commit, no mutation.
- [ ] An **aborted/quit editor** is a true no-op — no staging, no commit, no mutation.
- [ ] No separate `Continue?` gate is rendered on the `--no-ai` path.
- [ ] No `-p` push is implemented, but the save-accept path does not preclude it.

**Tests**:
- `"--no-ai skips the AI (no ai_command/claude invocation)"`
- `"--no-ai opens the editor via mint itself, not via git commit"`
- `"the editor buffer is empty/template with no synthetic stub"`
- `"a non-empty save under -a runs git add -u then commits, in that order"`
- `"a non-empty save under -A runs git add -A then commits, in that order"`
- `"a non-empty save under the default mode commits the index unchanged (no git add)"`
- `"an empty save is a true no-op (no staging, no commit)"`
- `"an aborted/quit editor is a true no-op (no staging, no commit)"`
- `"no Continue? gate is rendered on the --no-ai path"`

**Edge Cases**:
- Non-empty save = accept: applies `-a`/`-A` staging then commits in order.
- Empty save = no staging / no commit / no mutation.
- Aborted/quit editor = true no-op.
- Default-mode commits index unchanged on save.
- Editor opened by mint itself, not delegated to `git commit`.

**Context**:
> Commit Message Format → The `$EDITOR` fallback: "Three cases converge on dropping to `$EDITOR` with an empty/template message (behaving like plain `git commit`): 1. `--no-ai` — skip AI entirely; the user writes the message. No synthetic stub." $EDITOR Fallback — Path Semantics: "mint opens the editor itself (rather than delegating to `git commit`) because staging must be deferred until the save-as-accept event." "The editor save *is* the accept event. On the fallback path the editor replaces the `Continue?` gate (git-like): No separate `Continue?` gate. The gate governs the *AI-generated* message only; the fallback path uses the editor itself as the review. A non-empty save = accept; quit/empty = abort. (This reconciles '`--no-ai` behaves like plain `git commit`' with 'gate ON by default' — the gate is AI-path-only.) Staging applies on save. Same 'stage on accept' rule, where *save* is the accept: the editor opens against the real (unstaged) state; only on a non-empty save does mint apply `-a`/`-A` staging, then commit. `-p` push then runs as normal … Mutate-nothing-until-accept holds. Empty/aborted editor = true no-op. No staging applied, no commit, no push (even with `-p`). Nothing was mutated, so there is nothing to clean up." The `-y`/non-TTY and not-launchable fail-loud are 3-5; `-p` push is Phase 5 (runs after this save-accept but is built later).

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "Commit Message Format & Prompt → The `$EDITOR` fallback", "$EDITOR Fallback — Path Semantics".

## commit-command-3-3 | approved

### Task commit-command-3-3: Route AI-generation failure to the editor fallback

**Problem**: When the AI errors or returns nothing usable **after the engine's one retry**, commit must not abort (release's harsh notes-failure model is wrong for a routine local commit — the user is at the terminal anyway). Instead it falls back to `$EDITOR`, reusing the same save-as-accept semantics as `--no-ai`. This failure case must be **distinguished from the oversized-skip** (3-4): an oversized diff is a generate-*skip* that never calls L2, whereas this is a generate-*failure* after L2 ran and retried. And no synthetic stub message is inserted — the buffer opens empty/template just as on the `--no-ai` path. Without this routing, a transient AI failure would abort an otherwise-fine commit.

**Solution**: In the orchestrator, when L3 generate (1-3) surfaces a **generation-failure** typed outcome (the engine's transport failed / returned unusable output **after** its one retry — the consumed L2 behaviour from 1-3), route to the **same editor fallback** as `--no-ai` (3-2): resolve the editor (3-1), open it from mint with an empty/template buffer (no stub), and apply the **identical save-as-accept** semantics (non-empty save ⇒ stage-then-commit; empty/abort ⇒ true no-op). This is treated as *any other* AI failure — one consistent rule: any failed AI generation lands at the editor. The routing distinguishes this **generation-failure** outcome from the **oversized-skip** outcome (3-4), which short-circuits before L2 and is handled by its own task with a note.

**Solution note**: The save-as-accept routine, editor resolution, and staging-then-commit ordering are all **consumed** from 3-2/3-1/2-3 — this task adds only the *routing* of the generation-failure outcome into that path. The "after the engine's one retry" semantics are **consumed** from the shared L2 (1-3 already surfaces a distinguishable typed failure) — do NOT re-implement the retry. Keep the **generation-failure** branch distinct from the **oversized-skip** branch (3-4) so each can be reasoned about and tested separately (1-3 was authored to make them distinguishable). The `-y`/non-TTY and not-launchable fail-loud (3-5) apply to this path identically — but they are built in 3-5; this task is the TTY happy-path routing. Regeneration (`r`) failure also routes here, but `r` is a Phase 4 gate action — do NOT build `r` here; just ensure the failure-routing entry point is the shared one Phase 4 can reuse.

**Outcome**: When L3 generate fails (transport error / unusable output) **after** the engine's one retry, mint does **not** abort — it drops to the editor fallback with the **same** save-as-accept behaviour as `--no-ai`: editor opened by mint (resolved via 3-1) with an empty/template buffer and **no synthetic stub**; a non-empty save stages (per mode) then commits; an empty/aborted save is a true no-op. This **generation-failure** route is distinct from the **oversized-skip** route (3-4) — the failure happened after L2 ran and retried, not before any L2 call.

**Do**:
- In the orchestrator (`internal/commit` `Run`), branch on the L3 generate (1-3) outcome: when it is the **generation-failure** typed outcome (engine transport failed or returned unusable output after the consumed L2 one-retry), route to the editor fallback rather than aborting.
- Reuse the **save-as-accept** routine established in 3-2 verbatim: resolve the editor (3-1), open it from mint with an empty/template buffer (**no synthetic stub**), non-empty save ⇒ apply the mode's deferred staging (2-3) then `git_safe` commit; empty/abort ⇒ true no-op.
- **Distinguish from oversized-skip**: the generation-failure branch must be separate from the `max_diff_lines` oversized-skip branch (3-4). Oversized is a generate-*skip* (short-circuits before any L2 call, emits the "diff too large" note); generation-failure is a generate-*failure* (L2 ran, retried, failed) and carries **no** oversized note. Do not conflate the two.
- Do NOT re-implement the engine's one retry — it is consumed (1-3 surfaces the post-retry typed failure). Do NOT insert a synthetic stub or re-show any prior/partial message (there is no "re-show the prior message" path — the buffer opens empty/template).
- Do NOT build the `r` (regenerate) gate action (Phase 4); just make the failure-routing entry point a shared one Phase 4's `r`-failure can reuse. Do NOT build the `-y`/non-TTY or not-launchable fail-loud (3-5).
- Tests use the fake runner scripting an L2 failure after retry: assert the run does **not** abort, the editor is opened by mint with an empty buffer, save-as-accept behaves as 3-2 (non-empty save stages+commits, empty/abort no-ops), and the path is taken for generation-failure but **not** for an oversized diff (which goes to 3-4).

**Acceptance Criteria**:
- [ ] A generation-failure after the engine's one retry routes to the editor fallback — **not** abort.
- [ ] The save-as-accept semantics are **reused unchanged** from 3-2 (non-empty save stages-then-commits; empty/abort is a true no-op).
- [ ] The editor opens with an empty/template buffer — **no synthetic stub** and no re-show of a prior/partial message.
- [ ] The **generation-failure** route is distinguished from the **oversized-skip** route (3-4) — no oversized note on this path.
- [ ] The engine's one retry is **consumed**, not re-implemented.
- [ ] The `-y`/non-TTY and not-launchable fail-loud (3-5) are not implemented here; the `r` gate action (Phase 4) is not implemented here.

**Tests**:
- `"an AI generation failure after the one retry routes to the editor, not abort"`
- `"the editor fallback reuses save-as-accept unchanged (non-empty save stages then commits)"`
- `"an empty/aborted editor on the failure path is a true no-op"`
- `"the editor opens empty/template with no synthetic stub on the failure path"`
- `"the generation-failure route is distinct from the oversized-skip route"`
- `"the engine one-retry is consumed (commit code does not re-run the transport)"`

**Edge Cases**:
- Failure after the engine's one retry routes to editor, not abort.
- Distinguished from oversized-skip.
- Save-as-accept semantics reused unchanged.
- No synthetic stub message inserted.

**Context**:
> Commit Message Format → The `$EDITOR` fallback: "2. AI generation failure — if the AI errors or returns nothing usable after the engine's one retry, fall back to `$EDITOR` rather than abort. Low-stakes; the user is at the terminal anyway." $EDITOR Fallback — Path Semantics: "Regeneration failure routes here too. If the user presses `r` (regenerate-with-context) at the gate and the regeneration fails after its one retry, mint treats it as any other AI failure → the `$EDITOR` fallback. One consistent rule: any failed AI generation lands at the editor. No special 're-show the prior message' path." Detection ordering for the oversized case: "An over-limit diff short-circuits L2 entirely — it is a generate-*skip* (like `--no-ai`), not a generate-*failure*." So generation-failure (L2 ran + retried) and oversized-skip (no L2 call) are distinct. The `r` gate action is Phase 4; `-y`/non-TTY/not-launchable fail-loud is 3-5.

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "Commit Message Format & Prompt → The `$EDITOR` fallback", "$EDITOR Fallback — Path Semantics → Regeneration failure routes here too".

## commit-command-3-4 | approved

### Task commit-command-3-4: Route oversized diff (max_diff_lines) to the editor fallback with note

**Problem**: A diff exceeding `max_diff_lines` must **not** abort the commit (release's notes-failure model is too harsh for a routine large commit). Instead, commit falls back to `$EDITOR` with a clear note — *"diff too large to summarise — opening editor"*. This is detected at **L1**, after `diff_exclude` filtering and **before any L2 call**, so excluded noise (bundles, lockfiles, minified output) can **never** push an otherwise-fine diff over the limit. It is a generate-*skip* (like `--no-ai`), **not** a generate-*failure* (3-3) — it short-circuits L2 entirely. Without this, a large commit would either abort or burn an AI call on a diff that can't be summarised.

**Solution**: In commit's L3 glue / orchestrator, when the consumed L1 `max_diff_lines` guard reports the (already `diff_exclude`-filtered) diff is **over** the limit, treat it as a **generate-skip**: short-circuit **before any L2 call**, emit the note *"diff too large to summarise — opening editor"* via the consumed Presenter, and route to the **same editor fallback** (3-2) with save-as-accept. The `diff_exclude` filtering is applied **first** (consumed L1 ordering), so excluded files are removed before the line count is taken. The boundary is **at-limit passes, over-limit skips** (a diff exactly at `max_diff_lines` is not over the limit; only strictly exceeding it triggers the fallback).

**Solution note**: The `max_diff_lines` line-counting mechanics and the `diff_exclude`-then-count ordering are **consumed** from the shared engine / L1 (settled in the engine spec and applied at L1 per 1-3) — do NOT re-implement the line count or the filter. This task adds only commit's **fall-back-rather-than-abort branch**: detect the over-limit *skip* outcome at L1, emit the note, and route to the editor fallback (3-2 save-as-accept). It is a generate-**skip** (no L2 call), explicitly **distinct** from the generate-**failure** (3-3, which *did* call L2). The editor resolution (3-1), save-as-accept routine (3-2), and staging-then-commit ordering (2-3) are consumed unchanged. The `-y`/non-TTY and not-launchable fail-loud (3-5) apply to this path identically but are built in 3-5 — including that an unattended run hitting the oversized fallback has no message source and fails loud (the spec calls this out explicitly for the oversized case).

**Outcome**: When the `diff_exclude`-filtered diff exceeds `max_diff_lines`, mint **skips L2 entirely** (no `ai_command`/`claude` call), emits *"diff too large to summarise — opening editor"*, and drops to the editor fallback with the **same** save-as-accept behaviour as `--no-ai` (3-2): editor opened by mint, empty/template buffer, non-empty save ⇒ stage-then-commit, empty/abort ⇒ true no-op. `diff_exclude` is applied **first**, so excluded noise cannot push a diff over the limit. The boundary is exact: a diff **at** `max_diff_lines` is summarised normally (passes); only a diff **over** the limit triggers the fallback. This **oversized-skip** is distinct from the **generation-failure** route (3-3) — no L2 call was made.

**Do**:
- In commit's L3 glue / orchestrator (`internal/commit`, where 1-3 wired L1→L2), detect the consumed L1 `max_diff_lines` **over-limit** outcome on the (already `diff_exclude`-filtered) diff. The detection is **at L1, after `diff_exclude`, before any L2 call** — consumed ordering from 1-3; this task branches on its result.
- On over-limit, treat it as a **generate-skip** (like `--no-ai`): do **not** call L2 (no `ai_command`/`claude`), and emit the note *"diff too large to summarise — opening editor"* via the consumed Presenter.
- Route to the editor fallback (3-2): resolve the editor (3-1), open it from mint with an empty/template buffer, and apply the **identical save-as-accept** semantics (non-empty save ⇒ stage-then-commit per mode; empty/abort ⇒ true no-op).
- Ensure `diff_exclude` is applied **first** (consumed L1 ordering), so excluded files are removed **before** the line count — assert that a diff which is over-limit *only because of* excluded noise is **not** treated as oversized once exclusion is applied.
- Mark this outcome as a **generate-skip**, explicitly distinct from the generate-**failure** of 3-3: oversized short-circuits before L2 (no AI call, carries the oversized note); failure happened after L2 ran (no oversized note). Do not conflate them.
- Honour the **boundary**: a diff exactly **at** `max_diff_lines` passes to L2 normally; only a diff **strictly over** the limit triggers the fallback (at-limit vs over-limit).
- Do NOT re-implement the line count or the `diff_exclude` filter (consumed). Do NOT build the `-y`/non-TTY or not-launchable fail-loud (3-5) — but note 3-5 will make the unattended oversized fallback fail loud.
- Tests use the fake runner scripting an over-limit and an at-limit `diff_exclude`-filtered diff: assert over-limit skips L2 (no AI call), emits the note, and routes to save-as-accept (3-2); at-limit proceeds to L2 normally; an over-limit-only-due-to-excluded-noise diff is not treated as oversized.

**Acceptance Criteria**:
- [ ] An over-limit (`diff_exclude`-filtered) diff is detected at **L1, before any L2 call** — **no** `ai_command`/`claude` invocation.
- [ ] `diff_exclude` is applied **first**, so excluded noise alone cannot push a diff over the limit.
- [ ] The note *"diff too large to summarise — opening editor"* is emitted via the consumed Presenter.
- [ ] The oversized case routes to the editor fallback with **save-as-accept** reused from 3-2.
- [ ] The oversized outcome is treated as a **generate-skip**, distinct from the generate-**failure** (3-3) — no oversized note on the failure path, no L2 call on the oversized path.
- [ ] **Boundary**: a diff **at** `max_diff_lines` passes to L2; only a diff **over** the limit triggers the fallback.
- [ ] The line count and `diff_exclude` filter are **consumed**, not re-implemented.

**Tests**:
- `"an over-limit diff skips L2 (no AI call) and routes to the editor"`
- `"the oversized fallback emits 'diff too large to summarise — opening editor'"`
- `"diff_exclude is applied before the line count (excluded noise alone does not trigger oversized)"`
- `"a diff exactly at max_diff_lines passes to L2 normally"`
- `"a diff over max_diff_lines triggers the fallback"`
- `"the oversized case reuses save-as-accept from --no-ai unchanged"`
- `"the oversized skip is distinct from a generation failure (skip carries the note, no L2 call)"`

**Edge Cases**:
- Detected at L1 after `diff_exclude` and before any L2 call.
- `diff_exclude` applied first so excluded noise can't push over limit.
- Emits "diff too large to summarise — opening editor".
- Treated as generate-skip, not generate-failure.
- At-limit vs over-limit boundary.

**Context**:
> Commit Message Format → The `$EDITOR` fallback: "3. `max_diff_lines` exceeded — commit does **not** abort (release's notes-failure model is too harsh for a routine large commit). Fall back to `$EDITOR` with a clear note (*'diff too large to summarise — opening editor'*). `diff_exclude` still applies first, so excluded noise doesn't push a diff over the limit." "Detection ordering for the oversized case. `max_diff_lines` is evaluated at **L1**, after `diff_exclude` filtering and **before any L2 call**. An over-limit diff short-circuits L2 entirely — it is a generate-*skip* (like `--no-ai`), not a generate-*failure* — and routes straight to the `$EDITOR` fallback. The `-y`/non-TTY forbidden-combo check (below) then applies exactly as for `--no-ai`: an unattended run that hits the oversized fallback has no message source and **fails loud**. (The line-counting mechanics of `max_diff_lines` themselves are settled in the shared engine spec and reused; only commit's fall-back-rather-than-abort branch is commit-specific.)" The unattended oversized fail-loud is wired in 3-5; the save-as-accept routine and editor resolution are consumed from 3-2/3-1.

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "Commit Message Format & Prompt → The `$EDITOR` fallback (case 3 + Detection ordering)".

## commit-command-3-5 | approved

### Task commit-command-3-5: Fail loud when the fallback has no message source

**Problem**: The editor fallback is inherently interactive — it requires a TTY and a launchable editor to produce a message. When a fallback fires with **no message source** — under `-y` (auto-accept, no human), under **non-TTY stdin**, or when **no editor in the chain is launchable** on a TTY — there is nothing to commit with and mint must **fail loud** (*"no AI message and no interactive editor available"*). It must **never hang** (no blocking read with no terminal) and **never commit an empty message**. This extends the gate's forbidden-combo philosophy (unattended + needs-human → fail loud) to the editor path, and it applies **identically** across all three converging cases (`--no-ai`, AI-failure, oversized). There is **no `-m`/`--message` escape** — `mint commit` is for *minted* messages; unattended-with-own-message uses plain `git commit`.

**Solution**: A single fail-loud guard on the editor-fallback entry point (the path 3-2/3-3/3-4 converge on), evaluated **before** attempting to open any editor. It fires when: (a) `-y` is set, or (b) stdin is non-TTY (the consumed forbidden-combo condition from the Presenter), or (c) the editor resolves to the **not-launchable** signal from 3-1 on a TTY. In all three cases — and across all three converging fallback triggers — mint **fails loud** with *"no AI message and no interactive editor available"*, performs **no staging**, creates **no commit**, never hangs, and never commits an empty message. No `-m`/`--message` flag is added.

**Solution note**: The `-y`/non-TTY forbidden-combo *condition* is **consumed** from the Presenter (1-5 already establishes the non-TTY-without-`-y` fail-loud philosophy for the gate) — this task extends that philosophy to the editor path, reusing the consumed TTY/`-y` determination rather than re-implementing isatty detection. The **not-launchable** editor signal is **consumed** from 3-1. This guard sits in front of the save-as-accept open (3-2): it must fire **before** mint tries to launch the editor, so a `-y`/non-TTY/not-launchable run never blocks on an editor that cannot interact. It applies identically to all three triggers (`--no-ai`, generation-failure, oversized) because they converge on the same entry point. Note the distinction with Phase 4's `e` gate action: on the **fallback path there is no message yet** → fail loud (this task); on the **`e` gate action a message already exists** → graceful degrade (Phase 4, not built here). Do NOT add a `-m`/`--message` escape. Do NOT implement `-p` push (Phase 5).

**Outcome**: When any of the three fallbacks fires under `-y` (e.g. `mint commit -Apy --no-ai`, or `-Apy` when the AI fails or the diff is oversized), or under **non-TTY stdin**, or when **no editor in the chain is launchable** on a TTY, mint **fails loud** with *"no AI message and no interactive editor available"* — it does **not** open or hang on an editor, performs **no staging**, creates **no commit**, and never commits an empty message. The behaviour is **identical** across `--no-ai`, AI-failure, and oversized triggers. There is **no `-m`/`--message` escape** — the only unattended-with-own-message path is plain `git commit`.

**Do**:
- At the editor-fallback **entry point** (the shared path 3-2/3-3/3-4 converge on), before attempting to open any editor, evaluate the no-message-source guard:
  - **`-y` set** → fail loud (unattended, no human to write a message).
  - **Non-TTY stdin** (no `-y`) → fail loud — reuse the **consumed** forbidden-combo condition the Presenter already exposes (per 1-5); do NOT re-implement isatty detection.
  - **On a TTY, editor not launchable** → fail loud — branch on the **not-launchable** signal from 3-1 (no message candidate exists on the fallback path, so there is nothing to fall back to).
- The failure message is *"no AI message and no interactive editor available"*, surfaced via the consumed Presenter (and stderr).
- The guard must run **before** any editor launch attempt, so the run **never hangs** on a blocking editor/read with no terminal, and **never commits an empty message**.
- Apply the guard **identically** to all three converging triggers (`--no-ai`, generation-failure, oversized) — they share one entry point and one fail-loud rule.
- In all fail-loud cases: **no `git add`**, **no commit**, no mutation (consistent with mutate-nothing-until-accept — the fallback never reached an accept).
- Do **not** add a `-m`/`--message` flag or any escape hatch — the spec is explicit there is none. Do NOT implement Phase 4's `e`-action graceful-degrade (that path *has* a message; it is not this guard) or `-p` push (Phase 5).
- Tests use the fake runner / scripted TTY + `-y` + not-launchable signal: assert each of the three converging triggers fails loud under `-y`, under non-TTY stdin, and (on a TTY) under a not-launchable editor — with no editor launch, no staging, no commit, no hang, and no `-m` flag accepted.

**Acceptance Criteria**:
- [ ] A fallback under `-y` fails loud with *"no AI message and no interactive editor available"* — no editor launch, no staging, no commit.
- [ ] A fallback under **non-TTY stdin** fails loud (consumed forbidden-combo condition) — no editor launch, no staging, no commit.
- [ ] A fallback on a TTY with **no launchable editor** fails loud (3-1 not-launchable signal) — there is no message to fall back to.
- [ ] The guard applies **identically** across all three triggers (`--no-ai`, generation-failure, oversized).
- [ ] The run **never hangs** (guard fires before any editor launch / blocking read) and **never commits an empty message**.
- [ ] No `-m`/`--message` escape hatch is added.
- [ ] The `-y`/non-TTY condition is **consumed** from the Presenter (not re-implemented); the not-launchable signal is **consumed** from 3-1.

**Tests**:
- `"--no-ai under -y fails loud with 'no AI message and no interactive editor available'"`
- `"AI failure under -y fails loud (no editor, no staging, no commit)"`
- `"oversized diff under -y fails loud (no editor, no staging, no commit)"`
- `"a fallback under non-TTY stdin fails loud with no editor launch"`
- `"a fallback on a TTY with no launchable editor fails loud (no message to fall back to)"`
- `"the fail-loud applies identically across --no-ai, AI-failure, and oversized"`
- `"the run never hangs and never commits an empty message"`
- `"no -m/--message escape hatch is accepted"`

**Edge Cases**:
- `-y` + fallback fails loud.
- Non-TTY stdin + fallback fails loud.
- No launchable editor on TTY fails loud (no message to fall back to).
- Applies identically across all three converging cases.
- Never hangs, never commits an empty message.
- No `-m` escape hatch.

**Context**:
> $EDITOR Fallback — Path Semantics: "Requires a TTY. `$EDITOR` is inherently interactive. When a fallback fires under `-y` or non-TTY stdin (e.g. `mint commit -Apy --no-ai`, or `-Apy` when the AI fails / the diff is oversized), mint **fails loud** (*'no AI message and no interactive editor available'*) — it never hangs and never commits an empty message. This extends the gate's forbidden-combo philosophy (unattended + needs-human → fail loud) to the editor path. An unattended run with no message source is contradictory: `--no-ai` unattended has nothing to commit with, and an unattended user wants the AI anyway. **There is no `-m`/`--message` escape** — anyone needing unattended-with-own-message uses plain `git commit`; `mint commit` is for *minted* messages." "When no editor in the chain resolves to a launchable program … Fallback path (no message yet) → fail loud, same as the no-TTY/`-y` case above — there is no message to fall back to. `e` gate action (a message already exists) → graceful degrade …" The `e`-action graceful-degrade is Phase 4; the not-launchable signal is consumed from 3-1; the `-y`/non-TTY condition is consumed from the Presenter (1-5).

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "$EDITOR Fallback — Path Semantics → Requires a TTY; Editor resolution (no launchable program → fallback path fails loud)".
