---
phase: 5
phase_name: Auto-push — `-p` with Warn-Don't-Unwind
total: 5
---

## commit-command-5-1 | approved

### Task commit-command-5-1: Parse the `-p`/`--push` flag (flag-only, no config default)

**Problem**: `mint commit` must support pushing after a completed commit, but push is **opt-in via `-p`/`--push` only** — *"we never push without the `-p` flag"*. There is deliberately **no push config default** (the spec's "Deliberately NOT added for commit" excludes any push config key). Without the flag parsed and the orchestrator carrying a clear push-armed boolean, the later push tasks (5-2, 5-3) have no signal to gate the push step on, and there is a risk of inadvertently reading or inventing a config-driven default — which the spec forbids.

**Solution**: Add the `-p`/`--push` boolean flag to the `mint commit` subcommand (consumed `cmd/mint` surface, reserved across earlier phases) and resolve it into a single push-armed value the orchestrator (`internal/commit` `Run`) carries — e.g. a `push bool` on the run options. `-p` absent → push disarmed (the default, no push). `-p` present → push armed. **No push config key is defined, read, or defaulted** anywhere — push is purely flag-driven. The flag must compose cleanly inside the `-Ap` and `-Apy` bundles (it is just another boolean alongside `-A`, `-a`, `-y`, `--no-ai`).

**Solution note**: This task owns **only** flag parsing and the armed/disarmed value the rest of Phase 5 consumes. It does NOT run any push (5-2/5-3), warn on failure (5-4), or suppress push on empty/aborted runs (5-5) — it just arms the boolean. Do NOT add a `[commit].push` (or any push) config key, and do NOT read one — the `[commit]` config read (1-1) must remain push-free. The cross-verb `-p` divergence (release's `-p` = `--patch`, commit's `-p` = `--push`) is intentional and acceptable — note it but it needs **no** special handling here (each subcommand owns its own flag set; mint does not reconcile or alias them). `-p` arms push but the push only ever fires after a successful commit — that gating is 5-2/5-3, not this task.

**Outcome**: `mint commit` with `-p` absent carries push **disarmed** (no push will ever be attempted). `mint commit -p` (and the long form `--push`) carries push **armed**. `-p` composes inside `-Ap` (add-all + push, gate shown) and `-Apy` (add-all + push + auto-accept) — the headline ergonomic bundles parse with no conflict. **No push config key exists or is read** — the only way to arm push is the flag, and push is never armed by default. The flag arms the intent only; whether a push actually runs is decided post-commit by 5-2/5-3.

**Do**:
- In the `mint commit` subcommand (`cmd/mint`, the surface 1-4 added and earlier phases reserved), register `-p`/`--push` as a boolean flag.
- In the commit orchestrator (`internal/commit` `Run`), resolve the flag into a single push-armed value on the run options (e.g. `push bool`), so 5-2/5-3 gate the post-commit push on one boolean.
- `-p` absent → `push == false` (the default — no push). `-p`/`--push` present → `push == true` (push armed).
- **Do NOT** define, read, or default any push config key. The `[commit]` config read (consumed from 1-1) stays push-free; there is no `[commit].push`. The only source of the armed value is the flag.
- Ensure the flag composes in the `-Ap` and `-Apy` bundles — it is an independent boolean alongside `-A`/`-a`/`-y`/`--no-ai`, with no inter-flag conflict (unlike `-a`/`-A`, `-p` conflicts with nothing).
- Note (no code): the cross-verb `-p` divergence (release `-p` = `--patch`) is intentional and acceptable — the commit subcommand owns its own `-p` = `--push`; do not add aliasing or reconciliation logic.
- Do NOT implement the push itself, push-failure warn, or empty/aborted suppression (5-2..5-5).
- Tests assert the armed value via the run options / recording presenter + fake runner: `-p` arms, absent disarms, `-Ap`/`-Apy` parse and arm, and no config read path supplies a push default.

**Acceptance Criteria**:
- [ ] `-p`/`--push` present → push is armed (`push == true`) on the run options.
- [ ] `-p`/`--push` absent → push is disarmed (`push == false`) — the default, no push.
- [ ] `--push` long form parses identically to `-p`.
- [ ] `-p` composes in the `-Ap` and `-Apy` bundles with no flag conflict.
- [ ] **No push config key is defined, read, or defaulted** anywhere (the `[commit]` read stays push-free).
- [ ] Push is never armed by default — the flag is the sole source of the armed value.
- [ ] No push execution / failure-warn / empty-suppression behaviour is implemented (deferred to 5-2..5-5).

**Tests**:
- `"-p arms push on the run options"`
- `"--push (long form) arms push identically to -p"`
- `"absent -p leaves push disarmed (the default)"`
- `"-Ap parses and arms both add-all and push"`
- `"-Apy parses and arms add-all, push, and auto-accept"`
- `"no push config key is read or defaults push on (push armed only by the flag)"`

**Edge Cases**:
- `-p` absent → no push.
- `-p` present → arms push.
- No push config key exists or is read.
- Composes in `-Ap` and `-Apy` bundles.
- Push never armed by default.

**Context**:
> Auto-push Behaviour: "Push is opt-in via `-p` / `--push` (default: no push). Flag-only — no config default ('we never push without the `-p` flag'). `-p` is per-verb (release uses `-p` for `--patch`); the cross-verb `-p` divergence is intentional and acceptable (git subcommands carry their own flag meanings)." CLI Surface & Flags: "`-p, --push   push after committing (no push without this; no config default)`. Bundles: `mint commit -Ap` (add-all + push, gate shown) · `mint commit -Apy` (unattended). `-p` = push is per-verb (release's `-p` = `--patch`); the cross-verb `-p` divergence is intentional and acceptable." Config Schema — Deliberately NOT added for commit: "No push config — push is flag-only `-p`, never a default." The actual push, the failure warn, and the empty/aborted suppression are 5-2..5-5; this task is flag parsing + the armed boolean only.

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "Auto-push Behaviour", "CLI Surface & Flags", "Config Schema → Deliberately NOT added for commit".

## commit-command-5-2 | approved

### Task commit-command-5-2: Push after a successful gate-accept commit

**Problem**: With `-p` armed (5-1), mint must actually push — but **only after a successful commit**, and only via the accept paths. The gate-accept path (Phase 1's `y`/Enter and `-y` auto-accept, plus Phase 2's stage-then-commit and Phase 4's accept after `e`/`r` refinement) produces a completed commit; when `-p` is armed, a normal `git push` (current branch → its configured upstream) must run **after** that commit succeeds. Push runs **only** after a successful commit: `-p` with no commit (e.g. the commit step itself never completed) performs no push. mint adds **no special upstream logic** — it defers all upstream handling to git. Without this, the headline `mint commit -Apy` never pushes, and `-Ap` commits but leaves the user to push manually.

**Solution**: Add a single shared **push step** to the commit orchestrator (`internal/commit` `Run`) that runs after a successful commit when `push` is armed (5-1). On the gate-accept path — after the consumed stage-then-commit completes successfully (1-4/2-3 `git_safe` commit, including `-y` auto-accept and the post-`e`/`r` accept) — if `push` is armed, run a **normal `git push`** via the consumed `git_safe` lock-resilient wrapper: a plain `git push` with **no** upstream arguments and **no** branch/remote computation, so git pushes the current branch to its configured upstream using git's own resolution. The push is gated strictly on commit success: if the commit did not complete, no push runs. This task implements the success path; the failure-warn behaviour is 5-4 and the empty/aborted suppression is 5-5.

**Solution note**: The push must be a **single shared step both accept paths reach** (gate-accept here; editor save-as-accept in 5-3) — design it as one routine the orchestrator calls after a successful commit, NOT two parallel push implementations. The `git push` runs through the consumed **`git_safe`** wrapper (a push is a mutating, lock-relevant git op). mint runs a **plain `git push`** — do NOT add `--set-upstream`, `-u`, remote/branch arguments, current-branch detection, or any upstream inference; the spec is explicit that mint adds **no special upstream logic** and **defers to git** (a missing upstream is git's own failure, surfaced by 5-4's warn/pass-through). This task implements only the **happy path** (commit succeeds → push runs → push succeeds): the one-generic-warn-on-failure is 5-4; the never-unwind invariant is 5-4; suppressing push when nothing was committed (gate `n`, empty editor save) is 5-5. The push fires for the gate-accept commit **including `-y` auto-accept** and including the accept that follows a Phase 4 `e`/`r` refinement (all are the same gate-accept commit path).

**Outcome**: With `-p` armed and the gate-accept path taken (interactive `y`/Enter, `-y` auto-accept, or accept after an `e`/`r` refinement), once the commit succeeds via `git_safe`, mint runs a **plain `git push`** via `git_safe` — current branch → its configured upstream, with mint supplying **no** upstream/remote/branch arguments and **no** special upstream logic (git resolves the destination). The push runs strictly **after** the commit succeeds. If the commit did not succeed (the commit step produced no commit), **no push runs**. The push runs after a `-y` auto-accept commit exactly as after an interactive accept — same single shared push step.

**Do**:
- In the commit orchestrator (`internal/commit` `Run`), after the consumed gate-accept stage-then-commit completes (1-4's `git_safe` commit, ordered after 2-3's staging; reached by interactive `y`/Enter, `-y` auto-accept, or post-`e`/`r` accept), add a **push step** gated on the 5-1 `push` armed boolean.
- Implement the push as a **single shared routine** (e.g. an unexported `pushAfterCommit` the orchestrator calls) so the editor save-as-accept path (5-3) reuses the **same** step — no parallel push implementation.
- The push runs **only after the commit succeeds**: sequence it strictly after the `git_safe` commit returns success. If the commit step did not complete a commit, do not call the push step.
- Run a **plain `git push`** via the consumed **`git_safe`** wrapper: no upstream args, no `-u`/`--set-upstream`, no remote/branch computation, no current-branch detection — git resolves "current branch → configured upstream" itself. mint adds **no special upstream logic**.
- Ensure the push fires on the `-y` auto-accept commit (the consumed 1-5/2-3 `-y` path) exactly as on an interactive accept — both reach the same shared push step.
- Do NOT implement the failure warn / pass-through / never-unwind (5-4) or the empty/aborted suppression (5-5) here — assume the push succeeds. (5-4 layers failure handling over this same step.)
- All git invocation goes through the consumed `CommandRunner`/`git_safe`; tests script a successful commit then assert a plain `git push` (no upstream args) is recorded via `git_safe`, that it runs after the commit, that an unarmed `-p` records no push, and that a run where the commit did not complete records no push.

**Acceptance Criteria**:
- [ ] With `-p` armed, after a successful gate-accept commit a **plain `git push`** runs via the consumed `git_safe`.
- [ ] The `git push` carries **no** upstream/remote/branch arguments and no `-u`/`--set-upstream` — git resolves current branch → configured upstream (no special upstream logic in mint).
- [ ] The push runs **strictly after** the commit succeeds (ordered after the `git_safe` commit).
- [ ] With `-p` unarmed, **no push** runs after the commit.
- [ ] If the commit did not complete (no commit produced), **no push** runs.
- [ ] The push fires after a `-y` auto-accept commit exactly as after an interactive accept.
- [ ] The push is a **single shared step** (reusable by 5-3), not a gate-path-only implementation.
- [ ] No failure-warn / never-unwind / empty-suppression behaviour is implemented here (deferred to 5-4/5-5).

**Tests**:
- `"with -p armed, a plain git push runs via git_safe after a successful gate-accept commit"`
- `"the git push carries no upstream/remote/branch arguments (no special upstream logic)"`
- `"the push runs strictly after the commit succeeds"`
- `"with -p unarmed, no push runs after the commit"`
- `"-p without a successful commit performs no push"`
- `"the push runs after a -y auto-accept commit too"`
- `"the push goes through git_safe, not the raw runner"`

**Edge Cases**:
- Push runs only after the commit succeeds.
- Normal `git push` (current branch → configured upstream), no special upstream logic.
- `-p` without a successful commit performs no push.
- Push runs after a `-y` auto-accept commit too.

**Context**:
> Auto-push Behaviour: "Upstream handling: defer to git. `mint commit -p` runs a normal `git push` (current branch → its configured upstream). No upstream set → git's own failure, surfaced via the warn-clearly rule … mint adds no special upstream logic." Commit Flow / Lifecycle: "On accept — apply `-a`/`-A` staging now (if given), then `git commit` (via `git_safe`). Push (optional) — only if `-p`/`--push` (flag-only, no config default)." Interactive Review Gate: "`y` / accept → stage (if `-a`/`-A`) then commit; then push if `-p`." Scope: "`git_safe` lock-resilient git" is a reused shared primitive. The push must be a single shared step that the editor save-as-accept path (5-3) also reaches; the failure warn (5-4) and empty/aborted suppression (5-5) layer over this same step.

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "Auto-push Behaviour", "Commit Flow / Lifecycle", "Interactive Review Gate".

## commit-command-5-3 | approved

### Task commit-command-5-3: Push after an editor save-as-accept commit

**Problem**: Push must run via **every** accept path, not just the gate. The Phase 3 `$EDITOR` fallback's **save-as-accept** (`--no-ai`, AI-generation failure, oversized diff) is a full accept: a non-empty editor save stages (`-a`/`-A`) then commits. The spec is explicit that `-p` push **then runs as normal** after such a save — so `mint commit -Ap --no-ai` must stage, commit, **and** push end-to-end. This push must be the **same single shared push step** the gate-accept path uses (5-2), reached **only after** the staging+commit ordering completes — not a second, parallel push call bolted onto the editor path. Without this, `-p` would silently no-op on the editor accept path, breaking the "push via every accept path" guarantee.

**Solution**: Wire the **single shared push step** (5-2's `pushAfterCommit` routine) into the editor save-as-accept path (Phase 3's 3-2 non-empty-save accept, which 3-3/3-4 reuse). After a **non-empty** editor save applies the mode's deferred staging and commits via `git_safe` (consumed 3-2 ordering: stage → commit), if `push` is armed (5-1) the orchestrator calls the **same** push step 5-2 added — running a plain `git push` via `git_safe`. The push fires strictly **after** the staging+commit ordering completes, mirroring the gate-accept sequencing. This reuses 5-2's routine verbatim; the only difference is *why* the commit happened (editor save vs gate accept), not *what* the push does.

**Solution note**: This task adds **no new push implementation** — it routes the editor save-as-accept commit into the **same** shared push step 5-2 built (no parallel push call). The editor save-as-accept *resolution/open/stage/commit ordering* is consumed from Phase 3 (3-2, reused by 3-3/3-4); the `git push` via `git_safe` is consumed from 5-2. The push runs **only after** the consumed stage-then-commit ordering finishes (a non-empty save is a full accept). An **empty/aborted** editor save is a true no-op with **no commit** → **no push** even with `-p` — but that suppression is 5-5; this task assumes a **non-empty** save (a real commit) and wires the push onto it. The failure warn / never-unwind is 5-4 and applies to this path's push identically (same shared step). Do NOT special-case the editor path's push — it is the gate path's push reached from a different accept event.

**Outcome**: `mint commit -Ap --no-ai` (on a TTY, launchable editor, non-empty would-be-staged set) runs **end-to-end**: opens the editor (3-1/3-2), and on a **non-empty save** stages `git add -A` (2-3/3-2), commits via `git_safe` with the saved message, **then** runs a plain `git push` via `git_safe` (the **same** 5-2 push step). The push fires **only after** the staging+commit ordering completes. The editor-path push is the **same single shared step** as the gate-path push — there is **no** second/parallel push call. The same holds for the AI-failure (3-3) and oversized (3-4) editor drops when `-p` is armed: a non-empty save commits then pushes.

**Do**:
- In the editor save-as-accept path (Phase 3's 3-2 non-empty-save accept branch, reused by 3-3/3-4), after the consumed **stage → commit** ordering completes successfully, call the **same** shared push step 5-2 added (e.g. `pushAfterCommit`) when `push` is armed (5-1). Do NOT add a second push implementation.
- Sequence strictly: editor non-empty save → apply mode staging (consumed 2-3/3-2) → `git_safe` commit → **then** the shared push step. The push runs **only after** the staging+commit ordering finishes.
- Reuse 5-2's plain-`git push`-via-`git_safe` routine **verbatim** (no upstream args, no special upstream logic) — the push is identical regardless of which accept event triggered the commit.
- Verify the headline `mint commit -Ap --no-ai` flows end-to-end: stage (add-all) → commit → push, with no AI call (`--no-ai`), the editor opened by mint itself (3-2), and a single recorded push.
- Do NOT handle the **empty/aborted** editor save here (no commit → no push) — that suppression is 5-5; this task assumes a non-empty save. Do NOT implement the failure warn / never-unwind (5-4) — it applies to this push via the same shared step.
- Tests use the fake runner (scripted non-empty editor save) + recording presenter: assert `mint commit -Ap --no-ai` records stage → commit → push in order, the push is the **same** `git_safe` push step (no parallel push call recorded), and the push runs only after the staging+commit ordering; assert the AI-failure (3-3) and oversized (3-4) editor drops with `-p` also commit then push on a non-empty save.

**Acceptance Criteria**:
- [ ] A **non-empty** editor save (save-as-accept) commits **then** pushes when `-p` is armed.
- [ ] `mint commit -Ap --no-ai` runs end-to-end: stage (`git add -A`) → commit → push, with no AI call and the editor opened by mint itself.
- [ ] The editor-path push is the **same single shared push step** as the gate-path push (5-2) — **no** parallel/second push call.
- [ ] The push runs **only after** the staging+commit ordering completes.
- [ ] The push reuses 5-2's plain `git push` via `git_safe` (no upstream args, no special upstream logic).
- [ ] The AI-failure (3-3) and oversized (3-4) editor drops also commit-then-push on a non-empty save when `-p` is armed.
- [ ] The empty/aborted-save no-push case is NOT handled here (deferred to 5-5); the failure warn is NOT implemented here (5-4).

**Tests**:
- `"a non-empty editor save commits then pushes when -p is armed"`
- `"mint commit -Ap --no-ai runs stage, commit, push end-to-end"`
- `"the editor-path push reuses the single shared push step (no parallel push call recorded)"`
- `"the push runs only after the staging+commit ordering completes"`
- `"the editor-path push is a plain git push via git_safe (no upstream args)"`
- `"an AI-failure editor drop with -p commits then pushes on a non-empty save"`

**Edge Cases**:
- Non-empty editor save commits then pushes.
- `mint commit -Ap --no-ai` end-to-end (stage, commit, push).
- Reuses the single push step (no parallel push call).
- Push runs only after the staging+commit ordering completes.

**Context**:
> $EDITOR Fallback — Path Semantics: "Staging applies on save. Same 'stage on accept' rule, where *save* is the accept: the editor opens against the real (unstaged) state; only on a non-empty save does mint apply `-a`/`-A` staging, then commit. `-p` push then runs as normal (a non-empty save is a full accept, so `mint commit -Ap --no-ai` stages, commits, and pushes). Mutate-nothing-until-accept holds. Empty/aborted editor = true no-op. No staging applied, no commit, no push (even with `-p`)." Auto-push Behaviour: "`mint commit -p` runs a normal `git push` (current branch → its configured upstream) … mint adds no special upstream logic." The push here is the same single shared step as the gate-accept push (5-2); the empty/aborted no-push case is 5-5 and the failure warn is 5-4.

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "$EDITOR Fallback — Path Semantics → The editor save *is* the accept event", "Auto-push Behaviour".

## commit-command-5-4 | approved

### Task commit-command-5-4: Warn-don't-unwind on push failure

**Problem**: A push can fail for many reasons (rejected / non-fast-forward, remote moved, no upstream set, network). The spec's **post-accept never-unwind invariant** says: on a failed push, mint **keeps the commit**, emits **one generic warn** (the commit is in place; re-run the push to retry), passes **git's own stderr through verbatim** beneath the warn, and **never** unstages, resets, or rewrites. mint does **not classify** the cause — rejected, remote-moved, no-upstream, and network **all** get the **same one warn**; git's specific hint (set-upstream, non-fast-forward, etc.) stays visible only through the verbatim pass-through. Without this, a push failure could either be swallowed silently, trigger a dangerous unwind of the user's commit (clobbering pre-existing staged state), or grow per-cause messaging the spec explicitly rejects.

**Solution**: Wrap the **single shared push step** (5-2's routine, reused by 5-3) so that when the `git push` returns a non-zero/failure result, mint: (1) leaves the commit **completely untouched** — no `git reset`, no `git revert`, no unstage, no rewrite, no destructive cleanup of any kind; (2) emits **one generic warn** via the consumed Presenter — the commit is in place and the push can simply be re-run to retry; (3) renders **git's own stderr verbatim** beneath the warn (the consumed Presenter stderr pass-through), so git's specific hint stays visible; (4) does **not** classify the cause — there is a single failure branch for all causes (rejected, remote-moved, no-upstream, network), no per-cause messages or branching. The commit stays **forward-only** and the push is **repeatable** by hand.

**Solution note**: This applies to the **same single shared push step** for both accept paths (gate-accept 5-2 and editor save-as-accept 5-3) — implement the failure handling **once** around that one step, not per path. The warn rendering and the **stderr pass-through** are **consumed** from the Presenter seam (the same warn/render primitives release uses) — do NOT build a new warn renderer or re-format git's stderr (it is passed through **verbatim**). The *"set an upstream and push"* phrasing in the spec is **illustrative of git's pass-through**, NOT a mint-authored per-cause message — mint emits one generic warn and lets git's stderr supply the specific hint; do NOT author per-cause text or detect "no upstream" specially. **Never** add any unwind/reset/unstage/rewrite — the post-accept never-unwind invariant is absolute (the staging-safety concern: the user may have had files staged before `mint commit`, and resetting could clobber that). There is **no** destructive cleanup path at all. The empty/aborted-run no-push suppression is 5-5 (a failed push presupposes a commit happened, so it is distinct from "nothing was committed").

**Outcome**: When the `git push` fails for **any** cause — rejected/non-fast-forward, remote moved, no upstream set, network — mint: keeps the commit exactly in place (no reset/revert/unstage/rewrite — the commit is **forward-only** and the push is **repeatable**); emits **one generic warn** that the commit is in place and the push can be re-run; and prints **git's stderr verbatim** beneath the warn so git's own hint (e.g. its set-upstream suggestion, or its non-fast-forward message) is visible. mint does **not** classify the cause — the no-upstream case surfaces git's own hint via the **pass-through**, not a mint-authored message; all four causes hit the **same** single warn. The user's pre-existing staged/working state is never touched.

**Do**:
- Around the **single shared push step** (5-2's `pushAfterCommit`, reused by 5-3), branch on the `git push` result: **success** → done (5-2/5-3 happy path); **failure** (non-zero exit) → the warn-don't-unwind handling below.
- On push failure:
  - **Never unwind**: run **no** `git reset`, `git revert`, `git restore`, unstage, or any commit rewrite/amend — leave the commit and the index/working tree exactly as they are after the commit. There is **no** destructive cleanup path. Assert the commit (and any pre-existing user staging) is byte-for-byte untouched after a failed push.
  - **One generic warn**: emit a single warn via the consumed Presenter — the commit is in place and the push is repeatable (re-run to retry). The **same** warn fires for every cause; do NOT branch the warn text on the cause.
  - **Verbatim stderr pass-through**: render git's own stderr **verbatim** beneath the warn, via the consumed Presenter stderr pass-through. Do NOT reformat, summarise, or parse git's stderr — git's specific hint (set-upstream, non-fast-forward, etc.) stays visible only through the pass-through.
  - **No cause classification**: a single failure branch handles rejected, remote-moved, no-upstream, and network identically — no detection of "no upstream" or any other cause, no per-cause message. The *"set an upstream and push"* line is git's (via pass-through), not mint's.
- The push remains **repeatable**: mint reports failure but performs no cleanup, so re-running the push by hand is the documented fix. On a push failure the overall command **exits non-zero** (the push step failed) so scripted/CI callers can detect it — but the **commit stays in place** (forward-only, never unwound); the non-zero status signals only the failed push, not a failed commit.
- Apply this handling to the **one** shared push step so both the gate-accept (5-2) and editor save-as-accept (5-3) pushes get identical warn/never-unwind behaviour.
- Tests use the fake runner (scripted `git push` failure with canned stderr) + recording presenter: assert exactly **one** warn (same text across rejected / remote-moved / no-upstream / network scenarios), the canned stderr appears **verbatim** beneath it, **no** `git reset`/`revert`/`restore`/unstage/amend is recorded, the commit remains, and the no-upstream scenario surfaces git's hint via pass-through (no mint-authored per-cause message).

**Acceptance Criteria**:
- [ ] On push failure, mint runs **no** `git reset`/`revert`/`restore`/unstage/amend — the commit and pre-existing staging are untouched (never-unwind).
- [ ] On push failure, mint emits **one generic warn** (commit is in place; re-run the push) — the **same** warn for rejected, remote-moved, no-upstream, and network causes.
- [ ] git's own stderr is rendered **verbatim** beneath the warn (consumed Presenter pass-through) — not reformatted, summarised, or parsed.
- [ ] mint performs **no cause classification** — a single failure branch for all causes, no per-cause messages.
- [ ] The no-upstream case surfaces git's own hint **via the verbatim pass-through**, not a mint-authored "set an upstream" message.
- [ ] The commit stays **forward-only** and the push is **repeatable** (re-running the push by hand is the fix).
- [ ] The failure handling wraps the **single shared push step**, so gate-accept (5-2) and editor-accept (5-3) pushes behave identically on failure.
- [ ] On a push failure the overall command **exits non-zero** (deterministic — signalling only the failed push) while the commit remains in place.

**Tests**:
- `"a rejected (non-fast-forward) push keeps the commit and emits the generic warn"`
- `"a push failure exits non-zero while leaving the commit in place"`
- `"a no-upstream push failure emits the same generic warn (no per-cause message)"`
- `"a network push failure emits the same generic warn"`
- `"git's stderr is passed through verbatim beneath the warn"`
- `"a failed push runs no git reset/revert/restore/unstage/amend (never unwind)"`
- `"the commit remains in place and is forward-only after a failed push"`
- `"the no-upstream hint comes from git's pass-through, not mint-authored text"`
- `"the same warn fires for the editor-accept push failure (single shared step)"`

**Edge Cases**:
- One generic warn for all causes (rejected, remote moved, no upstream, network).
- git's stderr passed through verbatim beneath the warn.
- No cause classification.
- Never unstages/resets/rewrites the commit.
- Commit stays forward-only and the push is repeatable.
- No-upstream surfaces git's own hint via pass-through.

**Context**:
> Auto-push Behaviour: "Push failure → keep the commit, warn clearly, do NOT unwind. On a failed push (rejected, remote moved, no upstream, network), mint leaves the commit in place and reports clearly with the fix (re-run the push) … Push is not an atomic point-of-no-return with unwind; it is a best-effort final step whose failure is reported, not repaired." "mint does not classify push-failure causes. On *any* push failure … it emits one generic warn — the commit is in place; re-run the push to retry — with git's own stderr passed through verbatim beneath it, so git's specific hint (set-upstream, non-fast-forward, etc.) stays visible. The *'set an upstream and push'* line is illustrative of git's pass-through, not a mint-authored per-cause message. One rule for all causes: keep the commit, surface git's output, tell the user the commit is safe and the push is repeatable." Invariant — mutate nothing until accept; never unwind after: "After accept, mint never unwinds a completed commit — on a failed push it leaves the commit and reports clearly; it never unstages, resets, or rewrites … There is no destructive cleanup path at all." The warn rendering and stderr pass-through are consumed from the Presenter; the failure handling wraps the single shared push step (5-2/5-3).

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "Auto-push Behaviour", "Invariant — mutate nothing until accept; never unwind after".

## commit-command-5-5 | approved

### Task commit-command-5-5: Suppress push on empty/aborted runs; confirm no pre-push/remote-sync gate

**Problem**: Push must run **only after a successful commit** — so a run that committed **nothing** must perform **no push even with `-p`**. Two such cases exist: a gate **`n` abort** (nothing committed — the bare/gate path's true no-op) and an **empty/aborted editor save** (Phase 3's editor true no-op — no staging, no commit, no push even with `-p`). Separately, the spec drops the remote-sync gates for commit: there is **no pre-push gate even with `-p`** and **no remote-sync precheck** — mint attempts the push and *reports* failure rather than gating the commit on push-ability, which is what lets `mint commit -Apy` run **unattended end-to-end**. Without this task, `-p` could fire on an aborted run (pushing a phantom/unrelated commit, or erroring with nothing to push), or a phantom pre-push gate could block the unattended bundle.

**Solution**: Confirm and lock down two properties around the shared push step (5-2/5-3). (1) **Push is gated on commit success**: because the push step runs only after a successful commit (5-2/5-3), a gate `n` abort and an empty/aborted editor save — both of which produce **no commit** — reach **no push**, even with `-p` armed. This task adds explicit guards/assertions that the abort and empty-editor branches return their true no-op **before** the push step is reachable. (2) **No pre-push / remote-sync gate**: verify mint runs **no** remote-sync precheck, behind/diverged check, or any pre-push gate before attempting the push — consistent with the Preflight & Safety drops — so `mint commit -Apy` runs **unattended end-to-end** (no interactive gate, no precheck blocks the push attempt; failure is reported by 5-4, not pre-gated).

**Solution note**: This task is mostly **invariant-confirmation + guard placement**, not new push machinery — the push step is 5-2/5-3 and the failure warn is 5-4. It ensures the **abort/empty-editor no-op branches are reached before the push step** (the consumed gate-`n` no-op from 1-5/2-3 and the consumed empty/aborted-editor no-op from 3-2 must short-circuit the entire accept-and-push tail). It also confirms **no pre-push or remote-sync gate exists** — the dropped gates (clean-working-tree, on-release-branch, remote-in-sync, pre-push) from Preflight & Safety must stay dropped; do NOT add any remote-sync precheck or push-ability gate. `mint commit -Apy` must run unattended: `-y` auto-accepts the gate (consumed 1-5), staging+commit run, the push runs, and any push failure is the 5-4 warn (not an interactive block). The push-failure case (a commit happened but the push failed) is 5-4's concern and is distinct from this task's no-commit-so-no-push suppression.

**Outcome**: A gate **`n` abort** with `-p` armed performs **no push** — nothing was committed, so the shared push step is never reached (the run is a true no-op). An **empty/aborted editor save** with `-p` armed performs **no push** — Phase 3's editor true no-op (no staging, no commit) short-circuits before the push step. **No pre-push or remote-sync gate runs** at any point (no behind/diverged check, no push-ability precheck) — consistent with the dropped gates. `mint commit -Apy` runs **unattended end-to-end**: `-y` skips the gate, the commit and push run with no interactive prompt, and **no remote-sync precheck blocks the push attempt** (a failing push is reported by 5-4's warn, never pre-gated).

**Do**:
- In the commit orchestrator (`internal/commit` `Run`), ensure the push step (5-2/5-3) is **only** reachable on a path that produced a **successful commit**:
  - **Gate `n` abort** (consumed 1-5/2-3 no-op): the abort branch returns the true no-op **before** any commit or push — assert no `git push` is recorded on a gate `n` run with `-p` armed.
  - **Empty/aborted editor save** (consumed 3-2 no-op): the empty/quit-editor branch returns the true no-op (no staging, no commit) **before** the push step — assert no `git push` is recorded on an empty-editor-save run with `-p` armed.
- **Confirm no pre-push / remote-sync gate**: verify the orchestrator runs **no** remote-sync precheck, behind/diverged check, or push-ability gate before the push — the push is attempted directly (5-2/5-3) and failure is reported (5-4), never pre-gated. Do NOT add any such gate; the Preflight & Safety drops (clean-working-tree, on-release-branch, remote-in-sync, no pre-push gate) stay dropped.
- Verify `mint commit -Apy` runs **unattended end-to-end**: `-y` auto-accepts (consumed 1-5), add-all stages (2-3), the commit runs (1-4), the push runs (5-2) — with **no** interactive prompt and **no** remote-sync precheck blocking the attempt. A push failure on this run is the 5-4 warn (the run does not hang or pre-gate).
- Tests use the recording presenter (scripted `n` / empty-editor-save / `-Apy`) + fake runner: assert gate `n` + `-p` records no push, empty editor save + `-p` records no push, no remote-sync/pre-push check is recorded before the push, and `-Apy` records stage → commit → push with no interactive gate.

**Acceptance Criteria**:
- [ ] A gate **`n` abort** with `-p` armed performs **no push** (nothing committed — true no-op, push step never reached).
- [ ] An **empty/aborted editor save** with `-p` armed performs **no push** (Phase 3 editor no-op short-circuits before the push step).
- [ ] **No pre-push or remote-sync gate** runs at any point (no behind/diverged or push-ability precheck).
- [ ] **No remote-sync precheck blocks the push attempt** — the push is attempted directly and failure is reported (5-4), not pre-gated.
- [ ] `mint commit -Apy` runs **unattended end-to-end** (auto-accept → stage → commit → push) with no interactive prompt.
- [ ] The Preflight & Safety drops (clean-working-tree, on-release-branch, remote-in-sync, no pre-push gate) remain dropped — no such gate is added.

**Tests**:
- `"gate n abort with -p performs no push (nothing committed)"`
- `"an empty/aborted editor save with -p performs no push"`
- `"no pre-push or remote-sync gate runs before the push attempt"`
- `"no remote-sync precheck is recorded blocking the push"`
- `"mint commit -Apy runs unattended end-to-end (stage, commit, push, no interactive gate)"`
- `"the push attempt is made directly with no behind/diverged precheck"`

**Edge Cases**:
- Gate `n` with `-p` performs no push (nothing committed).
- Empty/aborted editor save with `-p` performs no push.
- No pre-push or remote-sync gate runs.
- `mint commit -Apy` runs unattended end-to-end.
- No remote-sync precheck blocks the push attempt.

**Context**:
> Auto-push Behaviour: "Push is opt-in via `-p` / `--push` (default: no push)." Commit Flow / Lifecycle: "Push (optional) — only if `-p`/`--push`." Interactive Review Gate: "`n` / abort → do nothing. No auto-unwind needed — nothing has been mutated yet (staging deferred to accept), so abort is a true no-op." $EDITOR Fallback — Path Semantics: "Empty/aborted editor = true no-op. No staging applied, no commit, no push (even with `-p`). Nothing was mutated, so there is nothing to clean up." Preflight & Safety — Gates commit deliberately DROPS: "Remote-in-sync (behind/diverged) — dropped. You commit while behind origin constantly; blocking that would be absurd. No pre-push gate even with `-p`. Consistent with the auto-push decision — mint doesn't gate the commit on push-ability; it attempts the push and *reports* failure. No remote-sync precheck." Posture: "The frequent one-liner stays fast via `-y` (`mint commit -Apy`)." This task is invariant-confirmation: the push step (5-2/5-3) is reached only after a successful commit, and no pre-push/remote-sync gate is added.

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "Auto-push Behaviour", "Interactive Review Gate", "$EDITOR Fallback — Path Semantics", "Preflight & Safety → Gates commit deliberately DROPS".
