---
status: complete
created: 2026-06-09
cycle: 2
phase: Traceability Review
topic: Mint Release Tool
---

# Review Tracking: Mint Release Tool - Traceability

Cycle 2 follow-up after cycle 1 applied its one fix (the `--dry-run` core task
`mint-release-tool-4-7a`, plus the matching Phase 4 Goal/acceptance updates).
This cycle re-read the full specification and all 70 tasks across the six phase
detail files in both directions.

## Result

**No findings.** The plan is a faithful, complete translation of the
specification in both directions.

### Direction 1 — Specification → Plan (completeness)

Every specification element has a task home with sufficient implementer-level
depth:

- **Overview / Settled foundations** — Go + `CommandRunner` seam (1-1); brew
  install hint surfaced in the shim (6-6). Distribution/tap auto-update is
  correctly treated as downstream CI, not mint's job.
- **Stage 1 (Version & tag grammar)** — version determination + grammar +
  numeric-max + prefix (1-3); `--set-version` rules (4-6); version-file
  projection plain/embedded (3-5/3-6) with the legacy-strategy mapping.
- **Stage 2 (Preflight)** — repo-root/branch resolution (1-4); local gates
  (1-5); network gates incl. fetch `--tags`, behind/diverged abort, never
  auto-pull (1-6); conditional `gh` gate before the tag (1-8); project
  `preflight` hook (3-2); `--autostash` (4-4); `--any-branch` (4-5).
- **Hooks** — runner mechanism + `sh -c` + repo-root + `MINT_*` env + string|array +
  first-non-zero (3-1); the three points with asymmetric pre/post-PONR failure
  (3-2/3-3/3-4); commit-interplay rule (3-3); dry-run skip+report (3-11).
- **Stage 4 (AI notes)** — transport (2-1); diff assembly + always-exclude
  `CHANGELOG.md` (2-2); `max_diff_lines` (2-3); Change Map (2-4); default prompt
  + KaC emoji skin + context/prompt knobs (2-5); normal-path wiring (2-6);
  `on_notes_failure` (2-7); degenerate stub (2-8); `--no-ai` (2-9); precedence
  (2-10); `diff_exclude` globs (3-9); strategy-aware `version_file` exclusion (3-10).
- **Body distribution** — single body whole to tag/changelog/provider with
  toggles (2-11); annotated tag as sole read source (1-10).
- **Stage 5 (Record)** — first-release body + changelog writer + bookkeeping
  commit (1-9); version-file fold (3-7); up-to-two-commit graph + tag target +
  no-op safety (3-8).
- **Stages 6–7** — annotated tag + atomic push PONR (1-10); lock-resilient git
  (4-1); surgical pre-PONR unwind (4-2) + trigger wiring incl. post-PONR
  warn-only (4-3); Publisher auto-detection (4-9) + safe downgrade (4-10).
- **Interactive review** — y/n/e semantics + `-y` (2-12); editor resolution
  (2-13); `r` regenerate + no-AI gate variant (2-14); `n` auto-unwind (2-15);
  end-to-end wiring (2-16).
- **Dry-run** — core no-mutation/plan-print (4-7a, the cycle-1 fix); cache
  write + key (4-7); reuse/miss/TTL/gate-orthogonality (4-8); hook skip (3-11).
- **Regenerate** — full Phase 5: command/flags (5-1); axis contract (5-2);
  version/diff-base resolution incl. oldest-release first-release rule (5-3);
  per-verb preflight subset (5-4); reuse read (5-5); fresh re-diff + the first
  real exercise of strategy-aware `version_file` exclusion on a range
  containing the bookkeeping commit (5-6); create-or-update probe (5-7);
  single-version in-place changelog write (5-8); write/push/recovery (5-9);
  interactive flow (5-10); `--all` loop (5-11); skip-and-continue + summary
  (5-12); whole-file rebuild + single end commit (5-13).
- **CLI surface** — all `mint release` / `regenerate` flags land across the
  phases; `mint version`/`--version` (6-8); `mint init` (6-7).
- **Config schema** — full verb-namespaced struct + defaults (6-1); unknown-key
  fail-loud incl. top-level `[hooks]` rejection (6-2); bad-type + enum + hook
  string|array (6-3); consolidation preserving the `provider`-value
  warn-downgrade carve-out (6-4).
- **`mint init`** — commented template (6-5); `release` shim (6-6); the
  idempotent/`--force` command (6-7).
- **Dependencies** — Presenter interface built against a recording fake (1-7),
  honouring the cross-spec boundary.

### Direction 2 — Plan → Specification (fidelity / anti-hallucination)

Every task's Problem, Solution, Do, Acceptance Criteria, Tests, and Edge Cases
trace to a named spec section, and each task carries a verbatim **Context**
quote and a **Spec Reference**. No invented requirements, technical approaches,
edge cases, or acceptance criteria were found. The two genuine spec ambiguities
the plan touches are surfaced *inside* the tasks as explicit ambiguity notes for
the executor to confirm, rather than silently resolved into invented scope:

- **5-4** — the spec groups gh-auth under the fresh changelog/both bucket; the
  task encodes the underlying general rule ("calls `gh` → gh-auth") and tests
  both surfaces, flagging the ambiguity.
- **5-13** — the spec does not state how a version *skipped* in the same `--all`
  batch is represented in the whole-file rebuild; the task assumes drop (clean
  rebuild) and surfaces this for resolution.

Both are faithful ambiguity-surfacing, not hallucination.

### Cycle-1 fix verified

`mint-release-tool-4-7a` (`--dry-run` core: read-only run, skip all mutations,
print the full plan) is present in both the Phase 4 task table and the
`phase-4-tasks.md` detail file, sequenced before 4-7/4-8, and references 3-11
(hook skip + `MINT_DRY_RUN`) and 4-7/4-8 (caching) rather than reimplementing
them. Phase 4's Goal and acceptance criteria in `planning.md` were updated to
include the dry-run core. The plan is now 70 tasks. No regression from the
cycle-1 shift was found.

### Out of scope (correctly absent, per the spec and the cycle-2 brief)

- CLI Presentation `Presenter` rendering / gate-rendering / `--plain` (external
  dependency; engine builds against the interface via the recording fake).
- `mint commit` (sibling spec).
- Consciously-deferred YAGNI items: pre-release/RC tags, `--rewrite-tags`,
  `.release/hooks/` directory, built-in note themes, project auto-detection in
  init, dry-run hook-run toggle, notes-review disable toggle, `.mintignore`.

## Findings

None.
