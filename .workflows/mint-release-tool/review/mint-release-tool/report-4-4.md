TASK: mint-release-tool-4-4 — Autostash escape hatch (--autostash) stash/restore with unwind ordering

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented (all 7 ACs). internal/engine/autostash.go:44-78 (push/pop/warn helpers); wired in release.go:345-349 (stash before gate + deferred pop) + cmd/mint/flags.go. Unwind-then-pop ordering achieved structurally — autostashPush runs at Stage 2 before clean-tree gate; pop is a defer that fires after Release returns, by which point the surgical unwind has reset to clean start. No-WIP keys off "No local changes to save"; push error treated as nothing-stashed. All stash/pop via deps.Mutator.Mutate (4-1).

TESTS:
- Status: Adequate. release_autostash_test.go: stash-before-gate + dirty passes, pop after success, abort unwind-before-pop (reset-precedes-pop, tag-d-precedes-pop index assertions), pop-conflict-after-success keeps stash + warns, no-WIP no-op, no-flag dirty still aborts + no stash, abort-path pop conflict. Exact-match index assertions.

CODE QUALITY:
- Followed conventions (accept-interfaces seam reuse, Warn seam mirroring warnPublishFailed, Mutator). SOLID good — three small single-purpose helpers; deferred-pop keeps ordering explicit. Low complexity, documented invariants.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/engine/release.go:345-349 — --autostash is not gated on !opts.DryRun. Under --dry-run --autostash with dirty WIP, the spine stashes at Stage 2 and the deferred pop restores on finishDryRun return; net repo state unchanged but it briefly mutates the tree via real git stash push/pop, in tension with the spec's "dry run NEVER reaches the Mutator". Cross-task interaction (dry-run is 4-7 scope) — decide whether dry-run should skip the stash entirely.
- [do-now] internal/engine/autostash.go:40-43 — the doc comment says a push error is "treated as nothing-stashed (stashed=false)"; consider noting this also means a genuinely-stashed-then-errored push would skip the pop. Doc precision only.
