# Plan: Dry-Run Autostash Mutation

## Phase 1: Apply Change

Make `--dry-run --autostash` provably mutation-free: skip the stash in dry-run and bypass the clean-tree gate when `DryRun && AutoStash`, preserving the byte-identical preview.

#### Tasks
status: approved

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| dry-run-autostash-mutation-1-1 | Gate autostash on dry-run; bypass clean-tree gate for dry-run+autostash | Non-autostash dry run still aborts on dirty tree; real autostash run unchanged; bypass conditioned on `DryRun && AutoStash`, not `DryRun` alone |
