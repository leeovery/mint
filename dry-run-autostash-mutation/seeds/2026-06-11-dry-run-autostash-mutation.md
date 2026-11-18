# `--dry-run --autostash` briefly mutates the tree via real git stash

`internal/engine/release.go:345-349` — `--autostash` is not gated on `!opts.DryRun`, so `--dry-run --autostash` with dirty WIP briefly mutates the working tree via a real `git stash push` / `pop`. The net result is unchanged, but it is in tension with the spec's "a dry run never reaches the Mutator / the repo is unchanged after a dry run" guarantee.

Decide whether dry-run should skip the autostash entirely (so dry-run is provably mutation-free), or whether the transient stash/pop is acceptable given the net-unchanged outcome.

Non-blocking. A decision item.

Source: review of mint-release-tool/mint-release-tool (Recommendation #30)
