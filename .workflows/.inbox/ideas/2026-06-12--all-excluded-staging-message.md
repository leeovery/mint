# Clearer empty-staging message when everything is `diff_exclude`-d

When every staged (or changed) file in a `mint commit` run matches a `[commit].diff_exclude` glob, commit does the right thing on substance — it recognises the post-exclusion would-be-staged diff as empty, fails loud, and never invokes the AI on a blank diff (this correctness gap was closed in `commit-command-6-1`, which made the preflight emptiness probes apply the same `:(exclude)` pathspecs the L1 diff uses). The remaining wrinkle is purely the *wording* of the failure.

The empty-staging message is selected by tree state via `git status --porcelain`, and porcelain does **not** apply `diff_exclude`. So when the only staged change is an excluded file, porcelain still reports a non-empty tree, and `emptyStagingError` falls into its "changes exist but the chosen mode staged none" branch. A bare/`StagedOnly` run therefore surfaces:

> `no changes staged — use -a/--all, -A/--add-all, or git add`

…which is misleading: the user *did* stage — their staged change is simply entirely diff-excluded, so none of `-a`/`-A`/`git add` would help. Under `-a` the analogous output is `no tracked changes to stage — use -A/--add-all to include untracked files`, with the same mismatch (the tracked change exists but is excluded).

The idea: detect the "all would-be-staged content is `diff_exclude`-d" case specifically and surface a dedicated message that points at the `[commit].diff_exclude` config (the actual reason nothing reached the diff) rather than at staging flags. Something to the effect of "all staged/tracked changes are excluded by `diff_exclude`" so the user knows to adjust the exclude config, not their staging.

This is low severity and UX-only — the load-bearing invariants (no AI on an empty diff, no git mutation, fail loud) all hold today. It was deliberately left out of `commit-command-6-1` and `commit-command-2-4`, which reused the existing empty-staging sentinels (`errNothingToCommit` / `errNoChangesStaged` / `errNoTrackedChanges`) under an explicit "do not invent a new message string" constraint. Implementing this would introduce a new sentinel/message and a way to distinguish "tree non-empty but post-exclusion would-be-staged set empty" from the ordinary "nothing staged" case.

Relevant code: the preflight cluster in `internal/commit/preflight.go` (`checkSomethingToCommit`, `wouldStageNothing`, `emptyStagingError`, the `*ProbeArgs` builders), the shared per-mode source builders in `internal/commit/source.go`, and `excludePathspecs` in `internal/commit/generate.go`. Surfaced by the commit-command cycle-1 architecture analysis and the `commit-command-6-1` review.
