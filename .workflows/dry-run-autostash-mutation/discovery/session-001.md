# Discovery Session 001

Date: 2026-06-18
Work unit: dry-run-autostash-mutation

## Description (as of session)

Gate the autostash on dry-run so `--dry-run --autostash` no longer briefly
mutates the working tree via a real git stash, keeping a dry run provably
mutation-free. The skip-vs-accept decision (skip the stash in dry-run, or
accept the transient stash/pop given the net-unchanged outcome) is small and
binary, to be settled in scoping.

## Seed

- seeds/2026-06-11-dry-run-autostash-mutation.md (inbox:idea)

## Imports

(none)

## Map State at Start

(n/a — single-topic work)

## Exploration

Origin is an inbox idea from the mint-release-tool review (Recommendation #30).
At `internal/engine/release.go:345-349` the `--autostash` path is not gated on
`!opts.DryRun`, so `--dry-run --autostash` against a dirty working tree briefly
mutates the tree via a real `git stash push` / `pop`. The net result is
unchanged, but the transient mutation is in tension with the spec guarantee
that a dry run never reaches the Mutator and the repo is unchanged after a dry
run.

Shaping settled the work type quickly. There is nothing to investigate — the
location and mechanism are already pinned — and no malfunction in observable
behaviour, so neither bugfix nor feature fits. The change is small, targeted,
and mechanical (gate the autostash on `!opts.DryRun`), which makes it a
quick-fix. The one open question — skip the stash entirely in dry-run (making
dry-run provably mutation-free) vs. accept the transient stash/pop — is binary
and small enough to settle in the scoping phase rather than reshaping the work.
No further topics surfaced; single-topic.

## Edits

(none)

## Topics Identified

(none)

## Conclusion

(none)
