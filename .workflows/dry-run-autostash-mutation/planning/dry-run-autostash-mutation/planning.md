# Plan: Dry-Run Autostash Mutation

## Phase 1: Apply Change

Make `--dry-run --autostash` provably mutation-free: skip the stash in dry-run and bypass the clean-tree gate when `DryRun && AutoStash`, preserving the byte-identical preview.

#### Tasks
status: approved

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| dry-run-autostash-mutation-1-1 | Gate autostash on dry-run; bypass clean-tree gate for dry-run+autostash | Non-autostash dry run still aborts on dirty tree; real autostash run unchanged; bypass conditioned on `DryRun && AutoStash`, not `DryRun` alone |

### Phase 2: Analysis (Cycle 1)

Address findings from Analysis (Cycle 1).

#### Tasks

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| dry-run-autostash-mutation-2-1 | Consolidate the redundant skipCleanTree preflight tests | Surviving test must still prove the porcelain probe is absent, branch and tag-free gates run, and the run passes; alternative path (keep dirty-tree test) only if it adds a distinguishing assertion over an identical fixture |

### Phase 3: Analysis (Cycle 2)

Address findings from Analysis (Cycle 2).

#### Tasks

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| dry-run-autostash-mutation-3-1 | Reuse seedDryRunFirstRelease for the dry-run+autostash dirty-tree test instead of hand-copying the read-gate timeline | Parameterised helper must keep all other seeded lines and order unchanged; existing callers pass skipCleanTree=false byte-for-byte; surviving test still asserts zero porcelain probe, zero stash push/pop, no mutation tail, no gh, clean RunFinished |
