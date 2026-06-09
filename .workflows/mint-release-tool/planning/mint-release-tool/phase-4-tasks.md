---
phase: 4
phase_name: Robustness — Lock Resilience, Recovery, Dry-Run Caching & Publisher Resolution
total: 10
---

## mint-release-tool-4-1 | approved

### Task mint-release-tool-4-1: Lock-resilient git wrapper (git_safe built-in)

**Problem**: Phases 1–3 call git directly for every mutation (commit, tag, push, reset, stash) with no lock resilience. A background agent, editor, or concurrent git process can be holding the `.git` index/ref lock (`index.lock`, `HEAD.lock`, `<ref>.lock`) at the moment mint mutates — and a transient contended lock would currently blow up a release for no good reason. The legacy `release` bash script carried a `git_safe` helper that retried on a contended lock and cleared a provably-stale lock; that behaviour must be carried forward into mint as a built-in, tested once in Go and applied to **every** git mutation path. Crucially it must NOT clear a live/fresh lock (that would corrupt a real concurrent operation), and when retries are exhausted it must surface a clear error rather than hanging or silently proceeding.

**Solution**: A lock-resilient git mutation wrapper that sits on top of the Phase 1 `CommandRunner` seam. When a git mutation fails with a lock-contention error, the wrapper distinguishes a **provably-stale** lock (clear it, then retry) from a **live/fresh** lock (do not clear; retry with backoff while the holder may still finish). On a bounded number of retries the mutation either eventually succeeds or the wrapper surfaces the original lock error. This wrapper becomes the single chokepoint through which all mint git mutations flow.

**Solution note**: This task builds and tests the wrapper mechanism once, and routes every existing mutation call site through it. It does NOT change *what* mutations happen or their ordering — only that each goes through the lock-resilient path. The surgical unwind (4-2) and its trigger wiring (4-3) and autostash (4-4) are separate tasks; they will themselves perform git mutations and so must also flow through this wrapper.

**Outcome**: Every git mutation mint performs (commit, annotated tag, `push --atomic`, any reset, tag delete, stash/pop) runs through the lock-resilient wrapper. A contended `.git` lock that clears within the retry budget results in eventual success. A **provably-stale** lock (the lock file exists but no live holder owns it — e.g. no live PID / the lock is older than a staleness threshold) is cleared and the mutation retried. A **live/fresh** lock is NOT cleared — mint retries with backoff and, if the lock persists past the retry budget, surfaces the lock error via the Presenter rather than deleting a lock a live process owns. Read-only git invocations (status, fetch, for-each-ref, rev-parse, diff) are unaffected.

**Do**:
- In `internal/git` (the package wrapping git invocation over the `CommandRunner`), add a lock-resilient mutation entrypoint, e.g. `RunMutation(ctx, args...)` (or a `Mutate` wrapper around the existing runner call), used by every mutation path.
- **Detect lock contention** from the failed git result: match git's lock-error signature on stderr (e.g. `Unable to create '…/.git/…lock': File exists` / `Another git process seems to be running`). Only the mutation paths use this; read-only calls keep calling the runner directly.
- **Stale vs live classification** (do NOT clear a live lock):
  - Resolve the offending lock path from the repo root (`.git/index.lock`, `.git/<ref>.lock`, etc.).
  - Treat a lock as **provably stale** only when there is positive evidence no holder is alive — e.g. the lock file's mtime is older than a staleness threshold (a fresh lock is younger than that threshold and is treated as live). Do not assume staleness from mere existence.
  - A **live/fresh** lock (younger than the threshold) is never cleared; mint just retries with backoff.
- **Retry with bounded budget + backoff**: retry the mutation a small bounded number of times with short backoff between attempts. On each attempt, re-classify (a lock may go stale, or the holder may finish and release it).
- **Clear a stale lock** by removing the lock file (through a filesystem op, not a git command), then retry the mutation once more within the budget.
- **Exhausted retries → surface error**: when the lock persists (live) past the retry budget, return the git failure (carrying the lock stderr) so the caller aborts; the Presenter reports the contended-lock abort. Never hang indefinitely; never silently skip the mutation.
- **Route every mutation through the wrapper**: update the Phase 1–3 call sites — release-bookkeeping commit, `pre_tag` artifact commit, annotated tag creation, `git push --atomic`, and the (best-effort) reset — to call the wrapper instead of the runner directly. (4-2 replaces the best-effort reset with the surgical unwind, which also uses this wrapper.)
- Tests use `FakeRunner` to script a lock-error result on the first attempt then success on a retry; assert: contended-then-cleared succeeds; a stale lock (old mtime) is cleared (filesystem removal observed) then the mutation retried; a fresh lock is NOT cleared and is retried; exhausted retries surface the error via `RecordingPresenter`; read-only calls bypass the wrapper.

**Acceptance Criteria**:
- [ ] A contended `.git` lock that frees within the retry budget results in eventual mutation success (retry then succeed).
- [ ] A provably-stale lock (old mtime / no live holder) is cleared (lock file removed) and the mutation retried.
- [ ] A live/fresh lock (recent mtime) is NOT cleared; mint retries with backoff and never deletes it.
- [ ] Exhausted retries surface the lock error (caller aborts; Presenter reports), never hanging or silently skipping.
- [ ] Every git mutation path (commit, tag, push, reset, tag-delete, stash/pop) flows through the wrapper; read-only git calls do not.
- [ ] All git invocation (including the retries) goes through the `CommandRunner`/`FakeRunner`; the staleness check and lock removal are the only direct filesystem touches.

**Tests**:
- `"a contended lock that clears within the budget retries then succeeds"`
- `"a provably-stale lock is cleared and the mutation retried"`
- `"a live/fresh lock is not cleared and is retried with backoff"`
- `"exhausted retries surface the lock error and abort"`
- `"a read-only git call does not go through the lock wrapper"`
- `"every mutation call site (commit, tag, push, reset) routes through the wrapper"`

**Edge Cases**:
- Contended `.git` lock → retry then succeed.
- Provably-stale lock → cleared.
- Live/fresh lock → not cleared.
- Retries exhausted → surface error.
- Applied to every mutation path.

**Context**:
> Lock-resilient git: "mint wraps all its git mutations in lock resilience (retry on a contended `.git` lock; clear a provably-stale lock). This carries forward the legacy `git_safe` behaviour as a built-in — tested once in Go, applied everywhere. A background agent/editor holding the index lock won't blow up a release." Phase scope: "Wrap ALL mint git mutations (retry on a contended `.git` lock; clear a provably-stale lock; do NOT clear a live/fresh lock; exhausted retries → surface error). Legacy `git_safe` as a built-in, tested once in Go, applied to every mutation path." The exact staleness threshold and backoff schedule are implementation detail; the load-bearing rule is stale-vs-live discrimination so a live lock is never destroyed.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stages 6–7 → Lock-resilient git".

## mint-release-tool-4-2 | approved

### Task mint-release-tool-4-2: Surgical pre-PONR auto-unwind (delete tag + reset N commits + report)

**Problem**: Phases 1–3 used a **best-effort, non-surgical reset** to recover from a pre-PONR failure or gate abort, deferring proper hardening to here. A best-effort reset is blunt: it doesn't precisely target what mint created and risks either under-cleaning (leaving a stray tag/commit) or over-cleaning. But mint knows *exactly* what it created this run — N commits (zero, one hook-artifact commit, or one hook-artifact + one bookkeeping commit) plus at most one annotated tag — so recovery can be **surgical**: delete the exact tag it made and reset exactly the N commits it made, returning to the exact clean starting state, and **report** what it undid. This is the invariant "nothing mint did this run survives unless the release completes." It is deliberately not configurable (YAGNI), and post-PONR it must never run (that would rewrite published history).

**Solution**: A surgical unwind operation that captures the exact starting ref state at the run's beginning (the starting HEAD commit and the fact that the target tag did not yet exist) and, when invoked pre-PONR, deletes the exact tag mint created (if it got as far as creating it) and resets HEAD back to the captured starting commit (dropping exactly the N commits mint made), then reports each undone item via the Presenter. It records nothing if mint made no mutations. All git operations flow through the lock-resilient wrapper (4-1). It is hard-wired (no config knob) and is a no-op / refuses to run post-PONR.

**Solution note**: This task builds the surgical unwind operation and proves it against the full range of pre-PONR mutation states (0/1/2 commits; tag created or not). The *wiring* of triggers — the gate `n` abort and pre-push failures — into this operation is task 4-3, which replaces those triggers' current best-effort reset. Autostash's "unwind first, then pop stash" ordering is task 4-4. The post-PONR warn-only publish-failure path (no unwind) already exists from Phase 1; this task only asserts the unwind refuses to run post-PONR.

**Outcome**: Given the run's captured starting state (starting HEAD + target tag absent), the unwind: (a) if mint created the annotated tag, deletes exactly that tag (local; it was never pushed pre-PONR); (b) resets HEAD back to the exact starting commit, dropping exactly the commits mint made this run (0, 1, or 2); (c) returns the repo to the exact clean starting state; (d) reports each undone item via the Presenter (e.g. "Reverted: deleted tag v1.4.0, reset 2 release commits"). With zero mutations made it is a no-op and reports nothing meaningful. It is not configurable. After the point of no return it never unwinds.

**Do**:
- Capture **starting state** at the start of the mutating portion of the run (after preflight, before any commit/tag): record the starting HEAD commit sha and confirm the target tag does not yet exist. Thread this captured state to the unwind.
- Track, as the run proceeds, **what mint created**: the count of commits made (0 = nothing dirtied/recorded; 1 = bookkeeping only, or hook-artifact only; 2 = hook-artifact + bookkeeping) and whether the annotated tag was created. This is precise bookkeeping mint already has — derive it from the spine steps that ran, not by inference.
- Implement the unwind, e.g. `Unwind(ctx, start StartState, made MadeState)`:
  - **Delete the tag** if `made.tagCreated`: `git tag -d {tag}` (local only — pre-PONR the tag was never pushed) through the lock-resilient wrapper (4-1).
  - **Reset commits**: `git reset --hard {start.HEAD}` through the lock-resilient wrapper, returning HEAD to the exact starting commit and discarding mint's release commits and their working-tree changes (changelog, version-file projection, hook artifacts). (Use the captured starting sha as the reset target, not a relative `HEAD~N`, so the result is provably the exact starting state.)
  - **Report** via the Presenter exactly what was undone — the deleted tag (if any) and the number of commits reset — so the user sees the rollback.
- **Zero-mutation case**: if `made` shows no tag and no commits, the unwind is a no-op (nothing to delete or reset); report nothing or a benign "nothing to undo".
- **Not configurable**: no config knob gates this; it is hard-wired pre-PONR recovery.
- **Post-PONR guard**: the unwind must only ever be invoked pre-PONR; assert (and document) that after `git push --atomic` succeeds, mint never calls it. (The trigger wiring in 4-3 enforces the call sites; this task ensures the operation itself is the pre-PONR recovery and not reachable post-PONR.)
- Tests via `FakeRunner` + `RecordingPresenter`: assert the exact git argv for tag-delete and reset; cover zero/one/two-commit states and tag-created-vs-not; assert the report names each undone item; assert reset targets the captured starting sha.

**Acceptance Criteria**:
- [ ] The unwind resets HEAD to the exact captured starting commit, dropping exactly the N commits mint made (0, 1, or 2).
- [ ] If mint created the annotated tag, the unwind deletes exactly that tag (local); if not, it deletes no tag.
- [ ] After unwind the repo is at the exact clean starting state it began from.
- [ ] The unwind reports each undone item (deleted tag and/or reset commit count) via the Presenter.
- [ ] With zero mutations the unwind is a no-op.
- [ ] The unwind is not configurable (no knob) and is only ever a pre-PONR operation; it never runs after the atomic push succeeds.
- [ ] All unwind git operations flow through the lock-resilient wrapper (4-1) and the `CommandRunner`/`FakeRunner`.

**Tests**:
- `"unwind with two commits and a tag deletes the tag and resets both commits"`
- `"unwind with one bookkeeping commit and a tag resets to the starting commit"`
- `"unwind with a tag not yet created deletes no tag"`
- `"unwind with zero mutations is a no-op"`
- `"unwind resets to the exact captured starting sha (not HEAD~N)"`
- `"unwind reports each undone item via the Presenter"`
- `"the unwind is never invoked after the atomic push succeeds"`

**Edge Cases**:
- Zero commits made.
- One commit.
- Two commits (hook-artifact + bookkeeping).
- Tag created vs not-yet-created.
- Reports each undone item.
- Post-PONR never unwinds.

**Context**:
> Failure model (before the push): "Everything mint did is local-only. mint auto-unwinds its own mutations — deletes the tag it made, resets the release commit(s) — returning the repo to the exact clean starting state. mint knows precisely what it created (N commits + 1 tag), so the unwind is surgical, and it reports what it undid. Next run starts clean. Not configurable (YAGNI)." Invariant: "Everything before stage 6 is local-only and recoverable… mint auto-unwinds every mutation it made this run, returning the repo to the exact clean state it started from." And: "After the point of no return, mint never unwinds (that would mean rewriting published history)." Phase scope: "Replace the best-effort reset — mint knows precisely what it created (N commits + 1 tag), so it deletes the exact tag and resets the exact N commits, returns to the exact clean starting state, and REPORTS what it undid; not configurable; post-PONR never unwinds." Commit graph (Phase 3, task 3-8): up to two commits — hook-artifact then bookkeeping — then the tag.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Release Lifecycle → Invariants", "Stages 6–7 → Failure model".

## mint-release-tool-4-3 | approved

### Task mint-release-tool-4-3: Gate-abort & pre-push failure route through surgical unwind

**Problem**: The surgical unwind (4-2) exists but is not yet wired to its triggers. Two distinct events must route to it and be treated **identically**: (1) the user answers `n` (abort) at the interactive review gate (Phase 2), and (2) any **pre-push failure** in the local-only stages (a hook abort, notes failure, changelog/version-file failure, tag-creation failure — Phase 1/3). Both currently use the Phase-1/2 best-effort reset, which this task replaces with the surgical unwind. The push-succeeds-but-publish-fails path must NOT unwind — the tag is already public, so it stays warn-only with a heal-path pointer (`regenerate --reuse`). Getting these routings wrong either leaves stray local state (under-recovery) or destroys a published tag (catastrophic over-recovery).

**Solution**: Replace the best-effort reset at both pre-PONR trigger sites with a call to the surgical unwind (4-2), passing the run's captured starting state and made-state. The gate `n` path and every pre-push failure path converge on the identical unwind, producing an identical clean-state result. The post-PONR publish-failure path is left exactly as Phase 1 built it: warn only, point to `regenerate --reuse`, no unwind.

**Solution note**: This task is wiring only — it does not change the unwind operation (4-2) or the lock wrapper (4-1). It removes the now-obsolete best-effort reset from the gate-abort path (Phase 2) and the pre-push failure path (Phase 1/3), substituting the surgical unwind. Autostash adds a stash-pop *after* this unwind on the abort/failure path (task 4-4), layered on top of this wiring.

**Outcome**: Answering `n` at the review gate triggers the surgical unwind (delete the tag if created, reset the N commits), returning to the exact clean starting state and reporting what was undone. Any pre-push failure (hook non-zero abort, notes failure, record failure, tag-creation failure) triggers the identical surgical unwind with the identical clean-state result — `n` and a pre-push git failure are indistinguishable in outcome. A failure *after* the atomic push succeeds (provider release create fails) does NOT unwind: mint warns the tag is published and points to `regenerate --reuse`; the pushed tag and commits remain.

**Do**:
- In the release orchestrator, locate the two pre-PONR recovery sites:
  - The **gate `n` abort** path (Phase 2 / task 2-15): replace its best-effort reset with a call to the surgical unwind (4-2), passing the captured starting state and made-state.
  - The **pre-push failure** path(s) (Phase 1/3): wherever a local-only stage fails before `git push --atomic` (preflight/`pre_tag` hook non-zero, notes failure routed by `on_notes_failure`, changelog/version-file Record failure, annotated-tag creation failure), replace the best-effort reset with the same surgical unwind call.
- **Treat `n` and a pre-push failure identically**: both call the same `Unwind` with the same inputs; assert the resulting repo state and Presenter report are identical for the two triggers.
- **Leave the post-PONR path unchanged**: the push-succeeds / publish-create-fails case (Phase 1 / task 1-11) stays warn-only — mint warns "the tag is already published" and points to the heal path (`regenerate --reuse` recreates the provider release from the tag annotation body); it must NOT call the unwind.
- Remove the now-dead best-effort reset helper from the pre-PONR sites (it is fully superseded by the surgical unwind).
- Tests via `FakeRunner` + `RecordingPresenter`: assert `n` at the gate invokes the surgical unwind and lands clean; assert a scripted pre-push failure (e.g. tag-create failure) invokes the identical unwind and lands in the identical state; assert a scripted publish-create failure after a successful push does NOT unwind and instead warns + points to the heal path.

**Acceptance Criteria**:
- [ ] Answering `n` at the review gate triggers the surgical unwind and returns to the exact clean starting state with a report of what was undone.
- [ ] Any pre-push failure (hook abort, notes failure, record failure, tag-creation failure) triggers the identical surgical unwind.
- [ ] The `n` abort and a pre-push failure produce identical clean-state results and reports.
- [ ] A publish-create failure after a successful atomic push does NOT unwind; mint warns the tag is published and points to `regenerate --reuse`.
- [ ] The obsolete best-effort reset is removed from the pre-PONR trigger sites.
- [ ] All recovery git operations flow through the surgical unwind (4-2) / lock wrapper (4-1) and the `CommandRunner`/`FakeRunner`.

**Tests**:
- `"answering n at the review gate triggers the surgical unwind to a clean state"`
- `"a pre-push tag-creation failure triggers the surgical unwind"`
- `"n abort and a pre-push failure produce identical clean-state results"`
- `"a publish failure after a successful push warns only and does not unwind"`
- `"the publish-failure warning points to the regenerate --reuse heal path"`

**Edge Cases**:
- Gate `n` → surgical unwind.
- Pre-push git failure → surgical unwind.
- Push succeeds + publish fails → warn only (no unwind).
- Identical clean-state result for `n` and failure.

**Context**:
> Interactive Review (`n` abort): "full auto-unwind: identical to the pre-push failure path — mint rolls back everything it made this run, including any `pre_tag` hook-artifact commit, returning to the exact clean starting state. The hook re-runs next time (idempotent build). A user-abort and a pre-push git failure are treated identically." Failure model: before the push → surgical auto-unwind; "Push succeeds but provider release create fails… The tag is already public, so mint never unwinds (that would be destructive history rewriting). mint warns and points to the heal path: `regenerate --reuse` recreates the provider release from the tag annotation body." Phase scope: "Wire the gate `n` abort (Phase 2) and any pre-push failure (Phase 1/3) to the surgical unwind (4-2), replacing their best-effort reset; the push-succeeds-publish-fails path stays warn-only + heal-path pointer (no unwind); `n` and a pre-push failure are treated identically."

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stages 6–7 → Failure model", "Interactive Confirmation & Notes Review → Default interactive flow (`n` abort)".

## mint-release-tool-4-4 | approved

### Task mint-release-tool-4-4: --autostash stash/restore with unwind ordering

**Problem**: The clean-tree preflight gate (Phase 1) blocks a release when the working tree has uncommitted tracked changes or non-ignored untracked files. `--autostash` is the opt-in escape hatch: stash the unrelated WIP (`--include-untracked`) before the run and restore it after. Because the release itself mutates the tree (hook commits, changelog, version file), restoring on abort/failure has a precise required ordering: mint must **first** auto-unwind its own commits/tag (4-2) back to the clean starting state, **then** pop the stash on top — otherwise the pop lands against mint's release commits instead of the original baseline. And if the pop conflicts, mint must **leave the stash intact and warn** — it must never discard the user's WIP. It is opt-in (not default) because popping unrelated WIP on top of a release can conflict, and opting in is the user asserting it's safe.

**Solution**: A `--autostash` flag that, when set, runs `git stash push --include-untracked` before the preflight clean-tree gate is enforced (so the gate sees a clean tree), and arranges restoration on every exit path. On success, mint pops the stash after the release completes. On abort/failure (pre-PONR), mint first runs the surgical unwind (4-2) to return to the clean starting state, then pops the stash on top. A pop conflict leaves the stash intact and warns. If there was no WIP to stash, the flag is a no-op. All stash/pop operations flow through the lock-resilient wrapper (4-1).

**Solution note**: This builds on the surgical unwind (4-2) and its trigger wiring (4-3) — autostash adds the stash-before / pop-after bracket around the run and inserts the pop *after* the unwind on the abort/failure path. It does not change the clean-tree gate's logic (Phase 1); it changes what the tree looks like *before* the gate runs (stashed → clean). Opt-in: without `--autostash`, the gate still blocks a dirty tree (Phase 1 behaviour unchanged).

**Outcome**: With `--autostash` and a dirty tree, mint stashes the WIP (including untracked files) before the clean-tree gate, so the gate passes. On a successful release, mint pops the stash afterward, restoring the WIP. On abort or pre-push failure, mint first surgically unwinds its own commits/tag to the clean starting state, then pops the stash on top — so the WIP lands back against the original baseline, not mint's release commits. If the pop conflicts, mint leaves the stash intact and warns (the WIP is never discarded). With no WIP to stash, `--autostash` is a no-op. Without the flag, a dirty tree still aborts at the clean-tree gate.

**Do**:
- Add the `--autostash` flag to `mint release`.
- **Stash before the gate**: when `--autostash` is set, run `git stash push --include-untracked` (through the lock-resilient wrapper, 4-1) **before** the clean-tree preflight gate is evaluated, so the gate observes a clean tree. Record whether anything was actually stashed (an empty stash — nothing to save — means no pop is needed later).
- **No-WIP no-op**: if the stash recorded nothing (clean tree already), `--autostash` makes no change and pops nothing.
- **Restore on success**: after a successful release (post-publish), pop the stash (`git stash pop`) through the wrapper, restoring the WIP.
- **Restore on abort/failure — strict ordering**: on any pre-PONR abort or failure, **first** run the surgical unwind (4-2, wired by 4-3) to return to the clean starting state, **then** pop the stash. The unwind-then-pop ordering is load-bearing: popping before the unwind would apply the WIP against mint's release commits.
- **Pop conflict → keep stash + warn**: if `git stash pop` reports a conflict, do NOT drop the stash; leave it intact and warn via the Presenter (e.g. "could not restore stashed changes cleanly — your work is preserved in `git stash`; resolve manually"). Never `git stash drop` on conflict. (A clean pop drops the stash entry as normal.)
- Tests via `FakeRunner` + `RecordingPresenter`: clean restore after success; abort path asserts unwind runs *before* the pop (invocation order); pop-conflict result leaves the stash intact and warns; `--include-untracked` is used; no-WIP run pops nothing; without `--autostash` a dirty tree still aborts at the gate.

**Acceptance Criteria**:
- [ ] `--autostash` runs `git stash push --include-untracked` before the clean-tree gate, so a dirty tree passes the gate.
- [ ] On a successful release the stash is popped afterward, restoring the WIP.
- [ ] On abort/failure mint first runs the surgical unwind, then pops the stash (unwind-then-pop ordering).
- [ ] A pop conflict leaves the stash intact and warns; the WIP is never discarded (no `git stash drop` on conflict).
- [ ] With no WIP to stash, `--autostash` is a no-op (nothing stashed, nothing popped).
- [ ] Without `--autostash`, a dirty tree still aborts at the clean-tree gate (Phase 1 behaviour preserved).
- [ ] All stash/pop operations flow through the lock-resilient wrapper (4-1) and the `CommandRunner`/`FakeRunner`.

**Tests**:
- `"--autostash stashes including untracked files before the clean-tree gate"`
- `"a dirty tree passes the gate when --autostash is set"`
- `"the stash is popped after a successful release"`
- `"on abort the surgical unwind runs before the stash pop"`
- `"a pop conflict leaves the stash intact and warns"`
- `"--autostash with no WIP is a no-op"`
- `"a dirty tree without --autostash still aborts at the gate"`

**Edge Cases**:
- Clean restore after success.
- Restore after abort (unwind then pop).
- Pop conflict → stash kept + warn (WIP never discarded).
- Untracked files stashed.
- No WIP → no-op.

**Context**:
> Clean working tree gate: "Escape hatch: `--autostash` (opt-in, not default) stashes (`--include-untracked`) before the run and restores after, including on abort/failure. Opt-in because the release mutates the tree (hook commits, changelog, version file) and popping unrelated WIP on top can conflict — opting in is the user asserting it's safe. Restore ordering: on abort/failure, mint first auto-unwinds its own commits/tag (back to the clean starting state), then pops the stash on top; if the pop conflicts, mint leaves the stash intact and warns — it never discards the user's WIP." Phase scope: "stash `--include-untracked` before the run, restore after; on abort/failure mint FIRST auto-unwinds its own commits/tag (4-2), THEN pops the stash; pop conflict → leave stash intact + warn; opt-in (not default)."

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 2 → Clean working tree (strict) / `--autostash`".

## mint-release-tool-4-5 | approved

### Task mint-release-tool-4-5: --any-branch branch-gate bypass

**Problem**: The on-release-branch preflight gate (Phase 1, auto-derived from `origin/HEAD`, overridable via `release_branch`) aborts when mint is run off the release branch — the right default for safety. But there are deliberate off-branch releases (a hotfix line, a one-off). `--any-branch` is the conscious escape hatch that bypasses *only* the branch gate for that run. Without the flag the gate must still abort off-branch (the Phase 1 default is unchanged); the flag must not weaken any other gate.

**Solution**: A `--any-branch` flag that, when set, skips the on-release-branch preflight gate (Phase 1) while leaving every other gate (clean tree, tag-free, remote sync, `gh` auth) intact. Without the flag, the branch gate runs exactly as before.

**Solution note**: This is a narrow flag that gates a single Phase 1 check. It touches only the branch gate; clean-tree, tag-free, remote-sync, and `gh`-auth gates are unaffected. It composes with `--autostash` (4-4) and the other escape hatches without interaction.

**Outcome**: Run off the release branch with `--any-branch`, the branch gate is bypassed and the release proceeds (subject to all other gates). Run off-branch **without** `--any-branch`, the branch gate still aborts (Phase 1 behaviour). On the release branch, `--any-branch` has no effect (the gate would pass anyway). No other gate is affected by the flag.

**Do**:
- Add the `--any-branch` flag to `mint release`.
- In the preflight sequence, when `--any-branch` is set, **skip the on-release-branch gate** (the Phase 1 / task 1-5 branch check) — do not evaluate it. All other gates still run.
- When `--any-branch` is not set, the branch gate runs unchanged (abort off-branch).
- Optionally report via the Presenter that the branch gate was bypassed by `--any-branch` (so an off-branch release is visible in the plan summary), but do not alter any other gate's behaviour.
- Tests via `FakeRunner` + `RecordingPresenter`: off-branch + flag → branch gate skipped, release proceeds (other gates run); off-branch without flag → branch gate aborts; on-branch + flag → no effect (gate passes regardless); assert the other gates (clean tree, tag-free, remote sync) still run under `--any-branch`.

**Acceptance Criteria**:
- [ ] Off the release branch with `--any-branch`, the branch gate is bypassed and the release proceeds (other gates still evaluated).
- [ ] Off the release branch without `--any-branch`, the branch gate still aborts (Phase 1 behaviour preserved).
- [ ] On the release branch, `--any-branch` has no effect.
- [ ] `--any-branch` does not weaken any other gate (clean tree, tag-free, remote sync, `gh` auth still run).
- [ ] All checks flow through the `CommandRunner`/`FakeRunner`; bypass is observable via the Presenter.

**Tests**:
- `"off-branch with --any-branch bypasses the branch gate and proceeds"`
- `"off-branch without --any-branch still aborts at the branch gate"`
- `"on-branch with --any-branch has no effect"`
- `"--any-branch does not skip the clean-tree, tag-free, or remote-sync gates"`

**Edge Cases**:
- Off-branch + flag → passes.
- Off-branch without flag → still aborts.
- On-branch + flag → no effect.

**Context**:
> On the release branch gate: "default-on, auto-derived from `origin/HEAD`… Override via `release_branch` in config. Escape hatch: `--any-branch` for a deliberate off-branch release." CLI flags: "`--any-branch` bypass the release-branch gate." Phase scope: "bypass the release-branch preflight gate for a deliberate off-branch release; without the flag the gate still aborts off-branch (Phase 1)."

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 2 → On the release branch", "CLI Surface & Flags → `mint release` flags".

## mint-release-tool-4-6 | approved

### Task mint-release-tool-4-6: --set-version explicit version validation (MINT_BUMP=explicit)

**Problem**: The bump flags (`-p`/`-m`/`-M`) compute the next version from the current latest tag (Phase 1). `--set-version X.Y.Z` is the explicit escape hatch for a deliberate jump (e.g. 1.x → 2.0.0). It carries strict rules: it is **mutually exclusive** with bump flags (combining them is an error — no silent precedence); the value must be a **valid 3-part semver**; and it must be **strictly greater than the current latest tag** (a backwards or equal jump is rejected *even if the target tag is free*, because a lower version sorts below "latest" and corrupts tag-as-truth). It is forward-only — no downgrade override (YAGNI). When used, it injects `MINT_BUMP=explicit` into the hook environment. Without these guards, `--set-version` could silently downgrade tag-as-truth or collide with a bump flag.

**Solution**: Extend the Phase 1 version-determination path with an explicit `--set-version X.Y.Z` flag that bypasses bump computation and instead validates the supplied version: it errors if combined with any bump flag; errors if not valid 3-part semver; errors if not strictly greater than the current latest tag. On success it becomes the next version and sets the bump kind to `explicit`, so the `MINT_BUMP` hook env (Phase 3) is `explicit`.

**Solution note**: This task adds the flag and its validation on top of Phase 1's version determination and Phase 3's `MINT_BUMP` env. It does not change how the current latest tag is resolved (Phase 1) or how the tag-free preflight gate works (the strictly-greater rule sits *on top of* tag-free — it rejects backwards/equal even when the target tag is free). No downgrade/`--force` override is built (YAGNI).

**Outcome**: `mint release --set-version 2.0.0` (current latest `1.4.3`) sets the next version to `2.0.0` and `MINT_BUMP=explicit`. `--set-version` combined with `-p`/`-m`/`-M` errors ("can't combine `--set-version` with a bump flag"). A malformed or non-3-part value (`2.0`, `2.0.0.1`, `2.0.0-rc.1`, `v2.0.0` if prefix not stripped, `abc`) errors. A value equal to the latest tag is rejected; a value less than the latest is rejected — both even if the target tag is free. A value strictly greater is accepted. On a first release (latest `0.0.0`), any valid 3-part version > `0.0.0` is accepted (e.g. `--set-version 1.0.0`).

**Do**:
- Add the `--set-version X.Y.Z` flag to `mint release`.
- **Mutual exclusivity**: if `--set-version` is given together with any of `-p`/`--patch`, `-m`/`--minor`, `-M`/`--major`, error with "can't combine `--set-version` with a bump flag" and do not proceed. (`--set-version` alone = explicit; a bump flag alone = computed; neither = default patch.)
- **Semver validation**: parse the value as strict 3-part SemVer (`MAJOR.MINOR.PATCH`, numeric segments only — reuse the Phase 1 tag-grammar parser). Reject anything non-3-part (`2.0`, `2.0.0.1`), pre-release/build (`2.0.0-rc.1`, `2.0.0+b5`), or non-numeric. The value is supplied without `tag_prefix` (mint adds the prefix when tagging); decide and document whether a leading prefix is stripped or rejected (prefer normalising/stripping consistent with the regenerate `<version>` normalisation, but at minimum reject ambiguity loudly).
- **Strictly-greater check**: compare the parsed version against the current latest tag (Phase 1's resolved latest, `0.0.0` if none). Reject when equal or less ("`--set-version X.Y.Z` must be greater than the current latest version `A.B.C`"). This is independent of, and on top of, the tag-free preflight gate — reject backwards/equal even if the target tag does not exist.
- **No downgrade override**: do not provide a `--force`-style bypass (YAGNI).
- **Set bump kind to `explicit`**: when `--set-version` succeeds, the run's bump kind is `explicit`, so the Phase 3 `MINT_BUMP` hook env var is `explicit` (not `patch`/`minor`/`major`).
- Tests via `FakeRunner` (for tag listing) + `RecordingPresenter` (for error reporting): combined-with-bump error; malformed-value errors; equal-to-latest reject; less-than-latest reject; greater-than-latest accept; first-release accept; assert `MINT_BUMP=explicit` is injected into the hook env.

**Acceptance Criteria**:
- [ ] `--set-version` combined with any bump flag errors with "can't combine `--set-version` with a bump flag".
- [ ] A non-3-part / pre-release / build / non-numeric value errors (strict SemVer 2.0.0 3-part only).
- [ ] A value equal to the current latest tag is rejected, even if the target tag is free.
- [ ] A value less than the current latest tag is rejected, even if the target tag is free.
- [ ] A value strictly greater than the current latest tag is accepted and becomes the next version.
- [ ] On a first release (latest `0.0.0`) any valid 3-part version > `0.0.0` is accepted.
- [ ] A successful `--set-version` sets the bump kind to `explicit`, so `MINT_BUMP=explicit` is injected.
- [ ] No downgrade/`--force` override exists.

**Tests**:
- `"--set-version combined with -p/-m/-M errors"`
- `"a non-3-part --set-version value errors (2.0, 2.0.0.1, 2.0.0-rc.1)"`
- `"--set-version equal to the latest tag is rejected even if the tag is free"`
- `"--set-version less than the latest tag is rejected"`
- `"--set-version strictly greater than the latest is accepted"`
- `"--set-version on a first release (latest 0.0.0) is accepted"`
- `"a successful --set-version injects MINT_BUMP=explicit"`

**Edge Cases**:
- Combined with `-p`/`-m`/`-M` → error.
- Malformed / non-3-part semver → error.
- Equal to latest → reject.
- Less than latest → reject.
- Greater → accepted.
- First release (latest `0.0.0`).
- `MINT_BUMP=explicit` injected.

**Context**:
> Bump selection: "`--set-version X.Y.Z` — explicit version escape hatch (e.g. a deliberate 1.x → 2.0.0 jump)." `--set-version` rules: "Mutually exclusive with bump flags. `--set-version` combined with `-p`/`-m`/`-M` is an error ('can't combine `--set-version` with a bump flag') — no silent precedence… Must be valid 3-part semver AND strictly greater than the current latest tag. A backwards/equal jump is rejected by default even if the target tag is free, because a lower version sorts below 'latest' and corrupts tag-as-truth. (This sits on top of the free-tag preflight check, which catches an equal/existing tag.) Forward-only today; no downgrade override. A `--force`-style 're-tag an old line' escape is YAGNI and deliberately not built now." Hook env: "`MINT_BUMP` … `explicit` when `--set-version` was used." Tag grammar: strict SemVer 2.0.0, three numeric segments only.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 1 → Bump selection / `--set-version` rules", "Hooks → Invocation & context (injected env vars)".

## mint-release-tool-4-7 | approved

### Task mint-release-tool-4-7: Dry-run note cache write & key computation

**Problem**: An agent commonly runs `mint release -y --dry-run` to surface the notes to a human, then `mint release -y` for real. AI generation is stochastic, so regenerating on the real run risks shipping notes that differ from the previewed ones. To guarantee "what was previewed is what ships", the dry-run must **cache** the notes it generates, keyed so the subsequent real run can safely reuse them. The key must be the hash of (post-`diff_exclude` diff + computed version + prompt/`[release].context`) — NOT the HEAD sha, because a `pre_tag` hook can change the tree between the (hook-skipped) dry run and the (post-hook) real run. The cache must live in a gitignored or system-temp location, be repo-scoped, carry a ~1h TTL stamp, and never be committed.

**Solution**: On a `--dry-run`, after the notes preview is generated (Phase 2's notes engine), compute the cache key from the post-`diff_exclude` diff, the computed version, and the resolved prompt/`context`, and write the generated note body plus a TTL timestamp to a repo-scoped, gitignored (or system-temp) cache. The dry run continues to skip hooks (Phase 3) and all mutations; the cache write is the only new side effect of `--dry-run`, and it is never committed.

**Solution note**: This task is the **write** half — dry-run produces and stores the cached note keyed correctly. The **read/reuse** half (real-run reuse on key match, miss-regenerate, TTL expiry, gate orthogonality) is task 4-8. This task does not change dry-run's skip-hooks/report behaviour (Phase 3 / task 3-11) — it adds the cache write on top.

**Outcome**: Under `--dry-run`, mint generates the notes preview and writes the note body to a cache keyed by `hash(post-diff_exclude diff + computed version + prompt/context)`, in a gitignored (e.g. `.mint/cache/`) or system-temp directory, scoped to the repo, with a TTL stamp (default ~1h). The cache file is never committed (the location is gitignored or outside the repo). Dry-run still skips all hooks and reports they were skipped, and performs no commit/tag/push. The key deliberately excludes the HEAD sha (a `pre_tag` hook can change the tree between runs), and includes the resolved prompt/`context` so a prompt change invalidates the key.

**Do**:
- After the dry-run generates the notes preview (Phase 2 engine, with Change Map + post-`diff_exclude` diff), **compute the cache key**: hash the concatenation/canonical form of
  - the **post-`diff_exclude` diff** (the same filtered diff fed to the AI — *not* HEAD sha),
  - the **computed next version**,
  - the **resolved prompt / `[release].context`** (whichever applies — `[release].prompt` full override file contents, or the default prompt plus injected `[release].context`).
- **Choose a cache location**: a gitignored repo path (e.g. `.mint/cache/`) or system temp, **scoped to the repo** (e.g. keyed/namespaced by the resolved repo root so caches don't collide across repos). Ensure the location is gitignored or outside the repo so it is **never committed** (if using a repo path, ensure `.mint/` or `.mint/cache/` is gitignored — write/ensure the ignore entry, or document the expectation, consistent with "gitignored cache… never committed").
- **Write the cache entry**: store the generated note **body** and a **TTL stamp** (the write time, so 4-8 can enforce the ~1h default TTL). Key the entry by the computed hash.
- **Only on `--dry-run`**: the cache write happens during a dry run. (A real run that *generates* fresh notes on a cache miss may also refresh the cache — but that is 4-8's concern; this task establishes the dry-run write path and the key/location contract.)
- **Preserve dry-run semantics**: hooks are still skipped and reported (Phase 3 / task 3-11); no commit/tag/push occurs; the cache write is the sole new side effect.
- Tests via `FakeRunner` (scripting the diff + AI command) + `RecordingPresenter`, with a temp cache dir: assert a dry run writes a cache entry; assert the key changes when the post-`diff_exclude` diff changes, when the version changes, and when the prompt/`context` changes; assert the key is invariant to HEAD sha changes that don't change the filtered diff; assert a TTL stamp is written; assert the location is repo-scoped and gitignored/temp; assert dry-run still skips hooks.

**Acceptance Criteria**:
- [ ] A `--dry-run` writes the generated note body to a cache entry.
- [ ] The cache key is the hash of (post-`diff_exclude` diff + computed version + resolved prompt/`context`), not the HEAD sha.
- [ ] The key changes when the filtered diff, the version, or the prompt/`context` changes.
- [ ] A TTL stamp (write time) is recorded with the entry (for the ~1h default TTL enforced in 4-8).
- [ ] The cache is repo-scoped and lives in a gitignored path (e.g. `.mint/cache/`) or system temp, and is never committed.
- [ ] Dry-run still skips all hooks and reports them skipped, and performs no commit/tag/push.
- [ ] All command execution flows through the `CommandRunner`/`FakeRunner`; the cache is the only filesystem side effect.

**Tests**:
- `"a dry-run writes the generated note to the cache"`
- `"the cache key changes when the post-diff_exclude diff changes"`
- `"the cache key changes when the computed version changes"`
- `"the cache key changes when the prompt/context changes"`
- `"the cache key is invariant to a HEAD sha change that doesn't change the filtered diff"`
- `"a TTL stamp is written with the cache entry"`
- `"the cache location is repo-scoped and gitignored/temp"`
- `"a dry-run with caching still skips hooks"`

**Edge Cases**:
- Cache dir gitignored / temp.
- Key includes context/prompt.
- TTL stamp written.
- Repo-scoped key.
- Dry-run still skips hooks.

**Context**:
> Dry-run note caching: "When `--dry-run` generates the notes preview, mint caches it so the subsequent real run reuses it — guaranteeing what was previewed is what ships… The win is determinism, not cost… Cache key = hash of (post-`diff_exclude` diff + computed version + prompt / `[release].context`) — not HEAD sha, since a `pre_tag` hook can change the tree between runs… Location: a gitignored cache (e.g. `.mint/cache/`) or system temp, keyed by repo, never committed, with a short TTL backstop (default ~1 hour…)." Phase scope (4-7): "`--dry-run` generates the notes preview and WRITES it to a gitignored (e.g. `.mint/cache/`) or system-temp cache, keyed by hash of (post-`diff_exclude` diff + computed version + prompt/`[release].context`), repo-scoped, ~1h TTL stamp, never committed." Dry-run semantics: still skips hooks and reports.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Dry-Run → Dry-run note caching".

## mint-release-tool-4-8 | approved

### Task mint-release-tool-4-8: Real-run cache reuse, miss-regenerate & TTL/gate orthogonality

**Problem**: The dry-run write (4-7) is only valuable if the real run reuses it. The real run must, on a cache key **match**, reuse the cached note (determinism — what was previewed is what ships, skipping the second AI call). On a **miss** (the post-`diff_exclude` diff, version, or prompt changed since the preview), it must regenerate and **say so** ("diff changed since dry-run preview — regenerating notes") — never silently ship a stale note that no longer matches. An **expired TTL** (older than ~1h) must also force regeneration. Reuse must be **automatic** (no flag). Two subtle interactions: a non-excluded `pre_tag` source change must correctly MISS (the real post-hook diff differs from the hook-skipped preview), while an excluded hook artifact must still REUSE (the key is the *post-`diff_exclude`* diff). And the review gate stays **orthogonal**: a cached note still shows at an interactive gate, and `-y` still skips the gate on both runs.

**Solution**: On the real run, before invoking the AI, compute the same cache key (4-7) and look it up. A live (non-expired) match reuses the cached body and skips the AI call. A miss or expired entry regenerates via the Phase 2 engine and reports the regeneration. Reuse is automatic (no flag). The key being the post-`diff_exclude` diff means an excluded hook artifact still matches (reuse) while a non-excluded source change misses (regenerate). The notes-review gate is untouched: the (reused or regenerated) body flows to the gate as normal, shown interactively and skipped under `-y`.

**Solution note**: This is the **read/reuse** half complementing 4-7's write. It consumes the key computation and cache location from 4-7. It does not change the review gate (Phase 2) — it only ensures a reused note is still subject to the same gate. It does not change `pre_tag` hook execution (Phase 3) — it relies on the key being post-`diff_exclude` to get reuse-vs-miss right across the hook-skipping dry run and the post-hook real run.

**Outcome**: On a real run, mint computes the cache key and, on a live match, reuses the cached note **without calling the AI** (deterministic preview→ship). On a key miss it regenerates and reports "diff changed since dry-run preview — regenerating notes". On an expired TTL (older than ~1h) it regenerates regardless of key match. A `pre_tag` hook that changed a **non-excluded** source path makes the real diff differ from the preview → key miss → regenerate. A `pre_tag` hook that only changed an **excluded** artifact leaves the post-`diff_exclude` diff identical → key match → reuse. The review gate is unaffected: an interactive real run still shows the (reused) note at the gate; `-y` skips the gate on both the dry and real runs. Reuse needs no flag.

**Do**:
- On the real run, **before invoking the AI**, recompute the cache key (4-7's hash of post-`diff_exclude` diff + computed version + prompt/`context`) and look up the cache entry.
- **Live match → reuse**: if an entry exists for the key and its TTL stamp is within the ~1h window, use the cached note body and **skip the AI call**. Report (quietly) that the previewed note is being reused, consistent with the determinism guarantee.
- **Miss → regenerate + report**: if no entry matches the key, regenerate via the Phase 2 notes engine and report "diff changed since dry-run preview — regenerating notes". Never ship the stale cached note.
- **Expired TTL → regenerate**: if an entry matches the key but its stamp is older than the ~1h TTL, treat it as absent — regenerate. (Optionally refresh/evict the stale entry.)
- **Automatic activation**: no flag enables reuse; the key-based invalidation makes automatic reuse safe.
- **Hook-interaction correctness** (falls out of the key being post-`diff_exclude`):
  - A `pre_tag` hook changing a **non-excluded** path → the real (post-hook) post-`diff_exclude` diff differs from the dry-run (hook-skipped) one → key **miss** → regenerate (correct: what ships differs from the preview).
  - A `pre_tag` hook changing only an **excluded** artifact → the post-`diff_exclude` diff is identical → key **match** → **reuse** (correct).
- **Gate orthogonality**: the reused or regenerated body flows into the notes-review gate exactly as a freshly generated one would. An interactive real run **still shows** the note at the gate (re-showing identical notes is cheap and avoids assuming an out-of-band approval). `-y` **still skips** the gate on both runs. Reuse guarantees determinism; the gate stays orthogonal.
- Tests via `FakeRunner` (scripting the AI command + diffs) + `RecordingPresenter`, with a temp cache pre-seeded by a dry-run write (4-7): key match → no AI call, reused body; key miss → AI called + "regenerating" report; expired TTL → AI called; non-excluded `pre_tag` change → miss; excluded artifact → reuse; interactive real run shows the cached note at the gate; `-y` skips the gate on both runs.

**Acceptance Criteria**:
- [ ] On a live key match the real run reuses the cached note and does NOT call the AI.
- [ ] On a key miss the real run regenerates and reports "diff changed since dry-run preview — regenerating notes".
- [ ] On an expired TTL (older than ~1h) the real run regenerates regardless of key match.
- [ ] Reuse is automatic — no flag is required.
- [ ] A non-excluded `pre_tag` source change causes a key miss (regenerate); an excluded-only hook artifact still matches (reuse).
- [ ] An interactive real run still shows the (reused) note at the notes-review gate; `-y` still skips the gate on both runs.
- [ ] All command execution flows through the `CommandRunner`/`FakeRunner`; gate behaviour is verified via `RecordingPresenter`.

**Tests**:
- `"a key match reuses the cached note without calling the AI"`
- `"a key miss regenerates and reports the diff-changed message"`
- `"an expired TTL regenerates even on a key match"`
- `"reuse activates automatically with no flag"`
- `"a non-excluded pre_tag change causes a key miss and regenerates"`
- `"an excluded-only hook artifact still matches and reuses"`
- `"a cached note is still shown at the interactive review gate"`
- `"-y skips the review gate on both the dry and real runs"`

**Edge Cases**:
- Key match → reuse (no AI call).
- Diff-changed miss → regenerate + report.
- Expired TTL → regenerate.
- Non-excluded `pre_tag` change → correct miss.
- Excluded hook artifact → still reuse.
- Cached note still shown at gate.
- `-y` still skips.

**Context**:
> Dry-run note caching: "Activation is automatic. The dry-run writes the note to the cache; the real run reuses it on a key match. No flag — the key-based invalidation makes automatic reuse safe… Miss → regenerate, and say so ('diff changed since dry-run preview — regenerating notes'). mint never silently ships a stale note that no longer matches the release. Interaction with `pre_tag` hooks: because the key is the post-`diff_exclude` diff, it is invariant to hook artifacts that fall under `diff_exclude`… so reuse holds even though dry-run skips hooks. If a `pre_tag` hook changes a non-excluded (real source) path, the dry-run (hook-skipped) and real (post-hook) diffs genuinely differ → the key correctly misses and the real run regenerates… Re-review is unaffected. A cached note does not skip the notes-review gate: an interactive real run still shows it… `-y` still skips the gate on both runs. Reuse guarantees determinism; the review gate stays orthogonal… a short TTL backstop (default ~1 hour…) so a stale preview can't be reused." Phase scope (4-8) restates these as the read/reuse, miss, TTL, hook-interaction, and gate-orthogonality requirements.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Dry-Run → Dry-run note caching".

## mint-release-tool-4-9 | approved

### Task mint-release-tool-4-9: Provider auto-detection from remote host

**Problem**: Phases 1–2 hardwired the GitHub driver when `publish = true`. Publishing is meant to be **provider-abstracted**, with the provider **auto-detected from the remote host**: a `github.com` remote (SSH or HTTPS) selects the GitHub driver (via `gh`) behind the `Publisher` interface, overridable by the `provider` config value. This removes the hardwiring so the detection seam is real and a future driver (GitLab, Gitea) can drop in without engine rework. Both SSH (`git@github.com:owner/repo.git`) and HTTPS (`https://github.com/owner/repo.git`) github.com URLs must detect GitHub.

**Solution**: A provider resolver that inspects the remote's host and selects a `Publisher` driver: `github.com` (matched from both SSH and HTTPS remote URL forms) → the GitHub driver. An explicit `provider` config value overrides auto-detection. The resolution sits behind the `Publisher` interface (Phase 1), replacing the hardwired GitHub selection.

**Solution note**: This task implements the *successful* detection/override path — `github.com` (and an explicit `provider = "github"`) resolve to the GitHub driver. The **downgrade** path — an unknown `provider` value, a non-github.com host, or no remote, with `publish = true` — is task 4-10. This task replaces the Phase 1/2 hardwiring with the detection seam; 4-10 adds the safe-downgrade fallback on top.

**Outcome**: When `publish = true` (default), mint resolves the provider from the remote host: a `github.com` remote — whether SSH (`git@github.com:owner/repo.git`, `ssh://git@github.com/…`) or HTTPS (`https://github.com/owner/repo.git`) — selects the GitHub driver via `gh`, behind the `Publisher` interface. An explicit `provider` config value overrides detection (e.g. `provider = "github"` forces the GitHub driver). The previously-hardwired GitHub selection is replaced by this resolver. The `gh` install/auth gate (Phase 1, conditional on actually publishing) still runs only when a driver is selected and publishing proceeds.

**Do**:
- Add a provider resolver, e.g. `ResolvePublisher(remoteURL, providerConfig) (Publisher, error/sentinel)`, that:
  - Reads the remote URL (the release remote, `origin` by default) via the `CommandRunner` (`git remote get-url origin` or `git config --get remote.origin.url`).
  - **Parses the host** from both URL forms: HTTPS (`https://github.com/owner/repo.git`) and SCP-like SSH (`git@github.com:owner/repo.git`) and `ssh://git@github.com/owner/repo.git`. Extract the host component robustly across these forms.
  - **Host `github.com` → GitHub driver** (the Phase 1 GitHub `Publisher` via `gh`).
- **Explicit override**: if `provider` is set in config to a recognised value (`"github"`), use that driver regardless of the detected host (the override wins over detection).
- **Behind the `Publisher` interface**: the resolver returns a `Publisher`; the orchestrator calls `CreateRelease`/`UpdateRelease` through the interface, never a concrete type — so a future driver slots in with no caller change.
- **Replace the hardwiring**: remove the Phase 1/2 "always GitHub when publish=true" selection and route through the resolver.
- **Defer downgrade to 4-10**: the resolver's non-github.com / unknown-value / no-remote outcomes are handled by 4-10; this task wires the success and override cases and exposes the unresolved outcome for 4-10 to act on.
- Tests via `FakeRunner` (scripting `git remote get-url` outputs) + `RecordingPresenter`: HTTPS github.com → GitHub driver; SSH `git@github.com:` → GitHub driver; `ssh://git@github.com/` → GitHub driver; explicit `provider = "github"` overrides detection; the resolved driver is used behind the `Publisher` interface; the `gh` gate still gates the selected driver.

**Acceptance Criteria**:
- [ ] An HTTPS `github.com` remote resolves to the GitHub driver.
- [ ] An SSH `github.com` remote (`git@github.com:…` and `ssh://git@github.com/…`) resolves to the GitHub driver.
- [ ] An explicit `provider` config value overrides auto-detection.
- [ ] Resolution is behind the `Publisher` interface — the orchestrator never references a concrete driver type.
- [ ] The Phase 1/2 hardwired GitHub selection is replaced by the resolver.
- [ ] All git invocation (remote URL read) flows through the `CommandRunner`/`FakeRunner`.

**Tests**:
- `"an https github.com remote resolves to the GitHub driver"`
- `"an SSH git@github.com remote resolves to the GitHub driver"`
- `"an ssh:// github.com remote resolves to the GitHub driver"`
- `"an explicit provider config value overrides detection"`
- `"the resolved driver is invoked behind the Publisher interface"`

**Edge Cases**:
- github.com remote → GitHub driver.
- SSH github.com URL → GitHub driver.
- Explicit provider config overrides detection.
- Detection behind Publisher interface.

**Context**:
> Publishing: provider driver abstraction: "Behind a small `Publisher` interface (`CreateRelease`/`UpdateRelease`). mint auto-detects the provider from the remote host (`github.com` → GitHub driver via `gh`), overridable by the `provider` config. GitHub is the only driver implemented now. The seam means GitLab (`glab`), Gitea, etc. can drop in later with zero rework — extra drivers are YAGNI; the interface is the cheap future-proofing… The interface shape and auto-detection mechanics are routine Go, left to implementation." Phase scope (4-9): "auto-detect the provider from the remote host (`github.com` → GitHub driver via `gh`) behind the Publisher interface; overridable by the `provider` config value; SSH and HTTPS github.com URLs both detect GitHub." Config: `provider` is optional; default auto-detected from remote host.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stages 6–7 → Publishing: provider driver abstraction".

## mint-release-tool-4-10 | approved

### Task mint-release-tool-4-10: Safe downgrade to tag+push on unresolved provider

**Problem**: When `publish = true` (default) but no driver can be resolved, mint must never silently assume GitHub and must never strand a pushed tag waiting on a release it can't create. Three unresolved cases all share one behaviour: (1) an unknown but recognised `provider` *value* (e.g. `provider = "gitlab"` when only GitHub is implemented); (2) auto-detection with no matching driver — a non-github.com host (GitHub Enterprise, GitLab, Gitea, an unmatchable SSH host) or no remote at all, with `publish = true` and no explicit provider. In all of these mint must **warn loudly and downgrade to tag + push only** (publish skipped) — never silently. Because the `gh` gate runs only when actually publishing, a downgrade means it was never gated, so the pushed tag is never stranded. `publish = false` is the separate **silent** tag+push (no warn). (Fail-loud validation for unknown config *keys* / bad *types* is Phase 6 — here it is only the provider *value* / no-driver downgrade.)

**Solution**: Extend the provider resolver (4-9) so that an unresolved provider — unknown recognised value, non-github.com host, or no remote — under `publish = true` with no explicit working driver downgrades the run to **tag + push only**: it warns loudly via the Presenter that publishing is skipped, and proceeds without selecting a `Publisher` (no `gh` gate, no provider release). `publish = false` remains a silent tag+push (no warn). The downgrade leaves the tag+push intact (it is post-PONR-safe — publishing was simply never attempted).

**Solution note**: This task completes the resolver started in 4-9 by handling its unresolved outcomes. It is scoped to the provider *value* / no-driver downgrade only — fail-loud validation of unknown config *keys* and bad *types* (and `mint init`) is Phase 6 and explicitly out of scope here. It does not change the success/override path (4-9) or the `gh` gate's conditional nature (Phase 1) — it relies on the gate running only when a driver is actually selected.

**Outcome**: With `publish = true` (default) and an unresolvable provider — an unknown recognised `provider` value (`provider = "gitlab"`), a non-github.com host (GHE/GitLab/Gitea/unmatchable SSH), or no remote at all, and no explicit working driver — mint warns loudly ("provider could not be resolved; downgrading to tag + push only, publish skipped") and downgrades: it tags and pushes but skips the provider release. The `gh` gate does not run (it gates only an actually-selected driver), so the pushed tag is never stranded waiting on a release mint can't create. `publish = false` is a silent tag+push (no warning — publishing was deliberately off). mint never silently assumes GitHub.

**Do**:
- Extend the resolver (4-9) so its unresolved outcomes are handled rather than erroring or defaulting to GitHub:
  - **Unknown recognised `provider` value** (e.g. `provider = "gitlab"` — a value mint recognises as a provider name but has no driver for): warn loudly and downgrade. (This is NOT a fail-loud config error — that family, for unknown *keys* / bad *types*, is Phase 6.)
  - **Auto-detection, no matching driver**: a non-github.com host (GHE, GitLab, Gitea, an unmatchable SSH host) or **no remote at all**, with `publish = true` and no explicit working `provider`: warn loudly and downgrade.
- **Downgrade behaviour**: when downgrading, do **not** select a `Publisher`; the run becomes **tag + push only** — the annotated tag and `git push --atomic` still happen; the provider release is skipped. Warn via the Presenter naming the reason (unknown value / unmatched host / no remote) so a typo or misconfiguration can't silently vanish.
- **`gh` gate is never reached on downgrade**: because the Phase 1 `gh` install/auth gate gates only an actually-selected publishing driver, a downgrade means it never gated — so the tag+push is never stranded waiting on an impossible release. Assert this ordering (no `gh` gate on a downgrade path).
- **`publish = false` is silent**: when publishing is explicitly off, mint does a tag+push with **no warning** (it is not a downgrade — the user opted out). Distinguish this from the warn-and-downgrade case.
- **Never silently assume GitHub**: an unresolved provider must never fall through to the GitHub driver.
- Tests via `FakeRunner` (scripting remote URLs) + `RecordingPresenter`: unknown `provider = "gitlab"` → warn + downgrade (no publish, no `gh` gate, tag+push intact); GHE/GitLab/Gitea host → warn + downgrade; no remote → warn + downgrade; unmatchable SSH host → warn + downgrade; `publish = false` → silent tag+push (no warn); assert the `gh` gate is not invoked on any downgrade path; assert no GitHub driver is silently used.

**Acceptance Criteria**:
- [ ] An unknown recognised `provider` value (e.g. `"gitlab"`) with `publish = true` warns loudly and downgrades to tag + push only.
- [ ] Auto-detection with no matching driver — non-github.com host (GHE/GitLab/Gitea/unmatchable SSH) or no remote — with `publish = true` and no explicit driver warns loudly and downgrades.
- [ ] On a downgrade the tag and push still happen; the provider release is skipped; the `gh` gate is never reached (so the tag is not stranded).
- [ ] `publish = false` is a silent tag + push (no warning).
- [ ] mint never silently assumes GitHub for an unresolved provider.
- [ ] Fail-loud validation of unknown config *keys* / bad *types* is NOT introduced here (deferred to Phase 6); only the provider value / no-driver downgrade is implemented.
- [ ] All git invocation flows through the `CommandRunner`/`FakeRunner`; warnings recorded via `RecordingPresenter`.

**Tests**:
- `"an unknown provider value warns and downgrades to tag+push only"`
- `"a non-github.com host (GHE/GitLab/Gitea) warns and downgrades"`
- `"no remote warns and downgrades to tag+push only"`
- `"an unmatchable SSH host warns and downgrades"`
- `"publish=false does a silent tag+push with no warning"`
- `"the gh gate is not reached on a downgrade path so the tag is not stranded"`
- `"an unresolved provider never silently uses the GitHub driver"`

**Edge Cases**:
- Unknown provider value → warn + downgrade.
- GHE/GitLab/Gitea host → warn + downgrade.
- No remote → warn + downgrade.
- Unmatchable SSH → warn + downgrade.
- `publish = false` → silent tag+push (no warn).
- `gh` gate skipped so tag never stranded.

**Context**:
> Publishing: provider driver abstraction: "An unknown/unsupported `provider` value (a recognised key, e.g. `provider = "gitlab"` when only GitHub is implemented) is not a fail-loud config error — mint warns loudly and downgrades to tag + push only (publish skipped), so a typo can't silently vanish. Fail-loud config validation still applies to unknown keys and bad types. Auto-detection with no matching driver — a non-`github.com` remote (GitHub Enterprise, GitLab, Gitea, an unmatchable SSH host) or no remote at all, with `publish = true` (the default) and no explicit `provider` — is treated the same as an unsupported value: mint warns loudly and downgrades to tag + push only. It never silently assumes GitHub, and (because the `gh` gate runs only when actually publishing) never strands a pushed tag waiting on a release it can't create." Config: "`publish` (default `true`; `false` = tag + push only)." Phase scope (4-10) adds: "`publish = false` is a silent tag+push (no warn). Note: fail-loud config validation for unknown KEYS / bad TYPES is Phase 6 — here it's only the provider VALUE / no-driver downgrade."

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stages 6–7 → Publishing: provider driver abstraction".
