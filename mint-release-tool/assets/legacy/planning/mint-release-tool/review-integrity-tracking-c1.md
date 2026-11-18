---
status: complete
created: 2026-06-09
cycle: 1
phase: Plan Integrity Review
topic: Mint Release Tool
---

# Review Tracking: Mint Release Tool - Integrity

## Summary

The plan is of high structural quality. All 70 tasks across 6 phases comply fully with the canonical task template (Problem / Solution / Outcome / Do / Acceptance Criteria / Tests, plus Edge Cases, Context, and Spec Reference where relevant). Vertical slicing is consistent — every task is a single, independently-verifiable TDD cycle. Self-containment is strong: each task pulls the relevant specification detail into its Context block and names the seams/tasks it builds on, so an implementer can execute any task without reading others. Phase structure follows a clean Foundation → Core → Robustness → Extension → Consolidation progression with clear per-phase acceptance criteria. Dependencies rely on natural intra-phase creation order (per the tick convention) with the explicit 4-7a→4-7/4-8 edges added by the traceability cycle; no circular dependencies or mis-ordered convergence points were found.

One minor finding: the Phase 4 phase-level acceptance criteria in `planning.md` were not updated when task 4-7a (the `--dry-run` core no-mutation / plan-print task) was added in the traceability cycle, so the phase's acceptance no longer fully represents what the phase delivers.

## Findings

### 1. Phase 4 acceptance criteria omit the `--dry-run` core no-mutation / plan-print behaviour (now task 4-7a)

**Severity**: Minor
**Plan Reference**: `planning.md` — Phase 4 ("Robustness…"), **Acceptance** list (and Goal line)
**Category**: Phase Structure (each phase has clear, complete acceptance criteria)
**Change Type**: add-to-task (phase acceptance criteria)

**Details**:
The traceability cycle added task `mint-release-tool-4-7a` — the core `--dry-run` behaviour: run read-only, skip every mutation (commit/tag/push/provider release), and print the full plan. The task and its task-table row were added to Phase 4, but the Phase 4 **phase-level acceptance criteria** in `planning.md` were not updated to reflect this new, user-facing capability. The existing dry-run acceptance bullet (line 126) covers only the *note-caching* dimension; the Phase 4 Goal (line 118) names "dry-run note caching" but not the no-mutation/plan-print core. As a result, a reader checking the phase against its acceptance criteria would not see the headline 4-7a capability gated at the phase level, even though the phase now delivers it. This is a documentation/consistency gap in the phase contract, not a gap in the task content itself (task 4-7a is complete and correct). Adding a phase acceptance bullet keeps the phase's acceptance an accurate, complete statement of what it ships.

**Current**:
```markdown
**Goal**: The forward pipeline is production-hardened — lock-resilient git on every mutation, surgical auto-unwind on pre-PONR failure, the `--autostash`/`--any-branch`/`--set-version` escape hatches, dry-run note caching for deterministic preview→ship, and full provider auto-detection with safe downgrade.

**Why this order**: This is the hardening layer over the now-complete forward pipeline. It refines failure and edge behaviour rather than adding new user-facing capabilities, so it belongs after the forward path's capabilities are all in place and before the separate regenerate command.

**Acceptance**:
- [ ] All git mutations are wrapped in lock resilience (retry on a contended `.git` lock; clear a provably-stale lock)
- [ ] Pre-PONR failures auto-unwind surgically (delete the tag created, reset the N commits) to the exact clean starting state and report what was undone; post-PONR never unwinds — publish failure warns and points to the heal path
- [ ] `--autostash` stashes `--include-untracked` before the run and restores after unwind, leaving the stash intact and warning on pop conflict; `--any-branch` bypasses the branch gate; `--set-version X.Y.Z` validated (mutually exclusive with bump flags, valid 3-part, strictly greater than latest)
- [ ] `--dry-run` generates the notes preview and caches it; the real run reuses on a key match (hash of post-`diff_exclude` diff + computed version + prompt/`context`), regenerates + reports on miss, with ~1h TTL, gitignored and never committed; the review gate is unaffected
- [ ] Provider is auto-detected from the remote host (`github.com` → GitHub); an unknown `provider` value, an unmatched host, or no remote with `publish = true` warns loudly and downgrades to tag + push only — never silently assumes GitHub, never strands a pushed tag
```

**Proposed**:
```markdown
**Goal**: The forward pipeline is production-hardened — lock-resilient git on every mutation, surgical auto-unwind on pre-PONR failure, the `--autostash`/`--any-branch`/`--set-version` escape hatches, the `--dry-run` core (read-only run, no mutations, full plan printed) plus dry-run note caching for deterministic preview→ship, and full provider auto-detection with safe downgrade.

**Why this order**: This is the hardening layer over the now-complete forward pipeline. It refines failure and edge behaviour rather than adding new user-facing capabilities, so it belongs after the forward path's capabilities are all in place and before the separate regenerate command.

**Acceptance**:
- [ ] All git mutations are wrapped in lock resilience (retry on a contended `.git` lock; clear a provably-stale lock)
- [ ] Pre-PONR failures auto-unwind surgically (delete the tag created, reset the N commits) to the exact clean starting state and report what was undone; post-PONR never unwinds — publish failure warns and points to the heal path
- [ ] `--autostash` stashes `--include-untracked` before the run and restores after unwind, leaving the stash intact and warning on pop conflict; `--any-branch` bypasses the branch gate; `--set-version X.Y.Z` validated (mutually exclusive with bump flags, valid 3-part, strictly greater than latest)
- [ ] `--dry-run` runs the read-only preflight, computes the version, generates the notes preview, and prints the full plan (the commits it would make, the tag, and the publish target) while skipping every mutation (commit/tag/push/provider release) and all hooks (reported skipped) — the repo is unchanged after a dry run
- [ ] `--dry-run` caches the notes preview; the real run reuses on a key match (hash of post-`diff_exclude` diff + computed version + prompt/`context`), regenerates + reports on miss, with ~1h TTL, gitignored and never committed; the review gate is unaffected
- [ ] Provider is auto-detected from the remote host (`github.com` → GitHub); an unknown `provider` value, an unmatched host, or no remote with `publish = true` warns loudly and downgrades to tag + push only — never silently assumes GitHub, never strands a pushed tag
```

**Resolution**: Fixed
**Notes**: Applied — Phase 4 Goal updated to name the --dry-run core, and the dry-run acceptance split into a no-mutation/plan-print bullet plus the caching bullet.

---
