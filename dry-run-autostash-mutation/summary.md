# Specification: Dry-Run Autostash Mutation

## Change Description

`mint release --dry-run --autostash` against a dirty working tree currently runs a
real `git stash push --include-untracked` (and a deferred `git stash pop`) through
`git.Mutator` — both reach the lock-resilient mutation sink and briefly mutate the
working tree. This violates the `DryRun` contract that *"a dry run NEVER reaches the
lock-resilient Mutator and the repo is byte-for-byte unchanged"*
(`internal/engine/release.go`).

Decision settled in scoping: **skip the stash entirely in dry-run** (Option A), making
a dry run provably mutation-free. Because `--autostash`'s only job is to make the tree
clean so the clean-tree preflight gate passes, skipping the stash means the clean-tree
gate must also be **bypassed when `DryRun && AutoStash`** — otherwise a dirty-tree dry
run would newly abort at that gate, a regression. This mirrors the existing
`--any-branch` gate-skip pattern. Observable behaviour is unchanged: the dry-run preview
stays byte-identical (same transcript, same generated notes, same cache key — the notes
diff is `git diff {lastTag}..HEAD`, a commit-to-commit range that never includes
working-tree WIP, so the stash never affected it).

## Scope

- `internal/engine/release.go`
  - Gate the autostash block (the `if opts.AutoStash { autostashPush / defer autostashPop }`
    sequence, ~lines 369-373) on `!opts.DryRun` so neither push nor pop runs in dry-run.
  - Pass a `skipCleanTree` condition (`opts.DryRun && opts.AutoStash`) into `runPreflight`,
    which threads it to `preflight.RunLocalGates`.
  - Update the as-built WHY-comments in the same change: the `AutoStash` and `DryRun`
    field docs (`ReleaseOptions`), the Stage-2 autostash block comment, and the `Release`
    function doc — each must state that the stash is skipped (and the clean-tree gate
    bypassed) under dry-run.
- `internal/preflight/preflight.go`
  - Add a `skipCleanTree bool` parameter to `RunLocalGates` (mirroring the existing
    `anyBranch` parameter): when true, `CheckCleanTree` is not evaluated. Update the
    `RunLocalGates` doc comment to document the new parameter.
- `internal/engine/release.go` — `runPreflight` signature gains the `skipCleanTree`
  parameter and forwards it to `RunLocalGates`; update its doc comment.
- `internal/engine/autostash.go` — header comment may note the push/pop is not invoked
  under dry-run (keep comments true to as-built).

## Exclusions

- **No change to the real-run (`--autostash` without `--dry-run`) path.** It still
  stashes before the clean-tree gate and pops afterward, on success and on abort,
  exactly as today.
- **No change to the non-autostash dry-run path.** A dirty-tree dry run *without*
  `--autostash` still aborts at the clean-tree gate (that abort correctly tells the user
  their tree is dirty) — the gate bypass is conditioned on `DryRun && AutoStash`, not
  on `DryRun` alone.
- **No change to `--any-branch`, remote-sync, tag-free, or gh gates** — only the
  clean-tree gate is conditionally skipped, and only for the dry-run+autostash combo.
- **No new flags, config keys, or presenter events.** The autostash push is already
  silent; skipping it produces no transcript change.
- **No change to the notes diff, cache key, or message body** (already proven
  WIP-independent).

## Verification

- `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race ./...`,
  `golangci-lint run` (0 issues) — the project gates.
- New/updated engine test: `--dry-run --autostash` against a dirty tree issues **zero**
  `git stash push` / `git stash pop` mutations (assert exact argv via `FakeRunner`) and
  still completes the full preview — no clean-tree abort, dry-run summary rendered.
- New/updated preflight test: `RunLocalGates(..., skipCleanTree=true)` does **not** run
  the clean-tree probe (`git status --porcelain` absent from recorded calls) while the
  branch and tag-free gates still run; `skipCleanTree=false` is unchanged.
- Regression guards intact: real `--autostash` (non-dry) still stashes/pops; non-autostash
  dry run on a dirty tree still aborts at the clean-tree gate.
- WHY-comments updated in the same change and true to as-built (per CLAUDE.md): no
  comment claims the stash runs in dry-run, and the `DryRun` "never reaches the Mutator"
  guarantee now holds for the `--autostash` combo.
