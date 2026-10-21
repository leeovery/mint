# A failed `pre_tag` hook leaves its files in the tree, and the next release fails the clean-tree check

When a `[release.hooks].pre_tag` hook exits non-zero, `mint release` aborts before the notes, as it should, but whatever the hook wrote to the working tree before failing stays there, uncommitted. A build that got halfway, generated assets, or a test run's output files are all left behind. The unwind reports nothing to undo, because mint made no commit of its own before the hook failed.

The user feels this on the next attempt. They fix whatever broke the build and run `mint release` again, and it stops at the clean-tree check, because the previous run's half-built files make the tree dirty. They have to notice the files, work out that the failed hook made them, and clean them up by hand before mint will run. This was seen live in the docs review: after a failing `pre_tag`, `git status` showed `?? artifact.txt`, and the next run failed the clean-tree gate.

The safety model describes the release as unwinding everything it did before the push, so a user can reasonably expect a failed release to leave the repo as it found it. Here the hook, not mint, wrote the files, which is why the surgical unwind leaves them alone. From the user's side it's one run that failed and left their tree dirty. A pre_tag hook that commits on its own has a related edge: those commits fall outside what mint counts as its own.

It happens whenever a `pre_tag` hook fails after writing tracked or untracked files that git doesn't ignore. Ignored outputs don't dirty the tree, so they don't trip the check.

This came up in the docs review after the token-aware-diff-sizing feature, which now documents the behaviour in the README's `[release.hooks]` section ("A failing `pre_tag` leaves what it wrote in the tree for you to clean up"). The hook run and its artifact commit are in `internal/engine/release.go` (around lines 1579-1581), and the unwind's no-commits case is in `internal/engine/unwind.go` (around lines 197-199).
