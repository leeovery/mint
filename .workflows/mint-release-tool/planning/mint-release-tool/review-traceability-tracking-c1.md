---
status: in-progress
created: 2026-06-09
cycle: 1
phase: Traceability Review
topic: Mint Release Tool
---

# Review Tracking: Mint Release Tool - Traceability

## Findings

### 1. Core `--dry-run` semantics (no-mutation + plan-print) owned by no task

**Type**: Missing from plan
**Spec Reference**: "Dry-Run (`-d` / `--dry-run`) → Semantics" — "**Does:** read-only preflight, compute the version, **generate the AI notes preview** (Change Map + diff → AI), and **print the full plan** — the commits it would make, the tag, and what it would publish. **Skips:** all mutations (commit / tag / push / provider release) **and all hooks** … — and **reports** that hooks were skipped." (Also "CLI Surface & Flags → `mint release` flags" lists `-d, --dry-run`.)
**Plan Reference**: N/A (gap); adjacent tasks mint-release-tool-3-11, mint-release-tool-4-7, mint-release-tool-4-8
**Change Type**: add-task

**Details**:
The spec's dry-run has three behaviours: (a) skip all *hooks* and report; (b) skip all *mutations* (commit/tag/push/provider release) and print the full plan (commits, tag, publish target); (c) cache the notes preview for the real run. Only (a) and (c) are owned by a task:

- Task **3-11** owns the hook-skip dimension only and explicitly disclaims the rest: "The `--dry-run` flag's broader plan-printing/no-mutation behaviour is wired in its own phase; this task covers the hook dimension of dry-run." It also notes "Dry-run also skips mutations generally — that broader behaviour is owned by the dry-run phase."
- Task **4-7** scopes itself to the cache *write* half and merely *relies on* mutation-skipping: "The dry run continues to skip hooks (Phase 3) and all mutations; the cache write is the only new side effect." It does not establish the mutation-skip or the plan-print.
- Task **4-8** scopes itself to the cache *read/reuse* half.
- Tasks **1-11** and **2-16** (the forward-path wiring) both explicitly defer `-d/--dry-run` to "later phases."

No task registers the `-d/--dry-run` flag's core behaviour, skips the commit/tag/push/provider-release mutations, or prints the full plan (the commits it would make, the tag, and what it would publish). Every task that touches dry-run points at "its own phase" for this behaviour, but that owner does not exist anywhere in the 6-phase plan. Without it, an implementer following the plan would build hook-skip and note-caching but never the actual no-mutation/plan-print dry run the spec defines — `mint release --dry-run` would attempt the real mutations the caching tasks assume are already suppressed.

The natural home is Phase 4 (Robustness — already owns the rest of dry-run via 4-7/4-8), as a task sequenced *before* 4-7 so the cache-write tasks can rely on a real no-mutation guard. Note caching (4-7/4-8) and hook-skip+`MINT_DRY_RUN` (3-11) already exist and should remain; this task adds only the missing core flag behaviour and references them.

**Proposed**:

Add to Phase 4 (place before mint-release-tool-4-7 in the task table; suggested id mint-release-tool-4-7 with subsequent caching tasks renumbered, or insert as mint-release-tool-4-6a / a new ordered entry — the orchestrator picks the final id):

Phase 4 task-table row:

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| mint-release-tool-4-7a | `--dry-run` core: read-only run, skip all mutations, print the full plan | all mutations skipped (no commit/tag/push/provider release), full plan printed (commits + tag + publish target), notes preview generated, hooks skipped + reported (reuses 3-11), `MINT_DRY_RUN=1` |

Phase 4 detail file (`phase-4-tasks.md`) task:

### Task mint-release-tool-4-7a: `--dry-run` core — read-only run, skip all mutations, print the full plan

**Problem**: The `-d`/`--dry-run` flag's core behaviour is owned by no task. The hook-skip dimension (task 3-11) and the note-caching (tasks 4-7/4-8) both exist and both explicitly defer the "broader plan-printing/no-mutation behaviour" to a dry-run-owning task that does not exist; the forward-path wiring (tasks 1-11, 2-16) likewise defers `--dry-run` to "later phases." Without this task, `mint release --dry-run` would run the read-only preflight and notes but then perform the real commit/tag/push/provider-release mutations the caching tasks (4-7/4-8) assume are already suppressed, and it would never print the plan the spec promises.

**Solution**: Implement the `--dry-run` flag's core semantics in the release orchestrator: run the read-only preflight, compute the version, generate the AI notes preview, and **print the full plan** (the commits mint would make, the tag, and what it would publish), while **skipping every mutation** — the release-bookkeeping commit, any `pre_tag` artifact commit, the annotated tag, `git push --atomic`, and the provider release. Hooks are already skipped + reported and `MINT_DRY_RUN=1` is set by task 3-11; the note preview is cached by task 4-7. This task adds the no-mutation guard and the plan print, and registers the flag's behaviour, so 4-7/4-8 sit on a real dry-run.

**Solution note**: This task owns the no-mutation guard and the plan-print only. The hook-skip + `MINT_DRY_RUN` env (3-11) and the note caching write/reuse (4-7/4-8) already exist — do NOT reimplement them; this task makes the dry run real so those tasks' "skips all mutations" assumption holds.

**Outcome**: `mint release --dry-run` runs the read-only preflight, computes the next version, generates the notes preview, and prints the full plan (the commits it would make, the tag, and the publish target) via the Presenter, then exits having made **no** commit, tag, push, or provider release. Hooks are skipped and reported (3-11); the notes preview is cached (4-7). The repo is byte-for-byte unchanged after a dry run.

**Do**:
- Register the `-d`/`--dry-run` flag's behaviour on `mint release` in the orchestrator (the flag surface was reserved in task 1-11). Thread a `dryRun` state through the spine.
- Run the **read-only** stages normally: preflight gates (Phase 1), version determination (incl. `--set-version` validation, task 4-6), and notes generation/preview (Phase 2 engine).
- **Skip every mutation** when `dryRun` is set: do not create the release-bookkeeping commit (task 1-9/3-7), the `pre_tag` artifact commit (task 3-3, already prevented because 3-11 skips the hook), the annotated tag (task 1-10), `git push --atomic` (task 1-10), or the provider release (`Publisher.CreateRelease`, task 1-8). Guard at each mutation point so a dry run never reaches the lock-resilient mutation wrapper (task 4-1).
- **Print the full plan** via the Presenter: the commit(s) mint would make (and their subjects), the tag it would create, and what it would publish (provider target, or that publishing is downgraded/disabled). Reuse the plan-summary the gate already shows (task 2-12) plus the would-do mutations.
- Confirm hooks are skipped and reported and `MINT_DRY_RUN=1` (task 3-11) under this flag — this task does not re-implement that, it relies on it.
- Confirm the notes preview is generated and cached (task 4-7) under this flag.
- Do NOT skip or alter the notes-review gate's orthogonality (task 4-8 already specifies `-y` skips on both runs; an interactive dry run still shows the plan/notes).

**Acceptance Criteria**:
- [ ] `mint release --dry-run` makes no commit, no tag, no push, and no provider release.
- [ ] `--dry-run` prints the full plan: the commit(s) it would make, the tag, and the publish target.
- [ ] The read-only preflight runs and the version is computed under `--dry-run`.
- [ ] The AI notes preview is generated under `--dry-run` (and cached per task 4-7).
- [ ] Hooks are skipped and reported and `MINT_DRY_RUN=1` under `--dry-run` (reusing task 3-11).
- [ ] The repo is unchanged after a dry run (no mutation reaches the lock-resilient wrapper).

**Tests**:
- `"--dry-run makes no commit, tag, push, or provider release"`
- `"--dry-run prints the full plan (commits, tag, publish target)"`
- `"--dry-run runs the read-only preflight and computes the version"`
- `"--dry-run generates the notes preview"`
- `"--dry-run skips hooks and reports them (via task 3-11) with MINT_DRY_RUN=1"`
- `"the repo is unchanged after a dry run"`

**Edge Cases**:
- All mutations skipped (commit / tag / push / provider release).
- Full plan printed (commits + tag + publish target).
- Notes preview still generated (and cached).
- Hooks skipped + reported; `MINT_DRY_RUN=1`.

**Context**:
> Dry-Run semantics: "Does: read-only preflight, compute the version, generate the AI notes preview (Change Map + diff → AI), and print the full plan — the commits it would make, the tag, and what it would publish. Skips: all mutations (commit / tag / push / provider release) and all hooks (they have side effects) — and reports that hooks were skipped." CLI flags: `-d, --dry-run`. The hook-skip + `MINT_DRY_RUN` env is task 3-11; the note caching is tasks 4-7/4-8; this task adds the no-mutation guard and the plan-print so those sit on a real dry run.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Dry-Run (`-d` / `--dry-run`) → Semantics", "CLI Surface & Flags → `mint release` flags".

**Resolution**: Pending
**Notes**:

---
