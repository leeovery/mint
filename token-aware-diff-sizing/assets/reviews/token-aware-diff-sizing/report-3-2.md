TASK: Keep the notes file out of the release's git steps (token-aware-diff-sizing-3-2, tick-303b9c)

ACCEPTANCE CRITERIA:
- Given a tree that is clean apart from an untracked `notes.md` at the repo root, `mint release -y --notes-file notes.md` passes the clean-tree check and releases. Afterwards `notes.md` is still untracked with its contents unchanged, and no commit the release made contains it (§13.2)
- Given `--notes-file NEXT_RELEASE.md` naming a tracked file with uncommitted edits, staged or unstaged: the clean-tree check passes; neither the pre-tag artifact commit nor the release bookkeeping commit contains those edits; afterwards the file carries the same edits, staged if they were staged and unstaged if not (§13.2)
- Given the notes file and one other untracked or modified path, the clean-tree check still fails as built (§13.2)
- Given `--autostash`, the notes file (untracked, or tracked with uncommitted edits) and other uncommitted changes, the other changes are stashed and restored as built. The notes file stays in the work tree through the run with its contents and staged state unchanged. Given the notes file is the only uncommitted change, nothing is stashed and no `git stash pop` runs (§13.2)
- Given a pre_tag hook that changes files, the artifact commit holds the hook's changes and not the notes file. Given a pre_tag hook that changes nothing, no artifact commit is made, even though the notes file is untracked or carries uncommitted edits (§13.2)
- Given mint run from a subdirectory with a relative `--notes-file` path inside the repo, the file that path names is the one left out of the clean-tree check, the stash and the commits (§13.1, §13.2)
- The README's `--notes-file` documentation with `mint release` says the file may sit in the repo, untracked or with uncommitted edits. It is kept out of the clean-tree check, `--autostash` and every commit the release makes, and left as it was (§13.2, §18.2)

STATUS: complete

SPEC CONTEXT: §13.2 puts the --notes-file file "outside everything mint does": read once at startup, not counted by the clean-tree check, left out of --autostash's stash, never staged or committed by any git step (the pre-tag artifact commit's `git add -A` included), and left in the work tree untouched. The 2026-10-19 corrigenda extend this to a tracked file with uncommitted edits and require its staged/unstaged split to survive a successful run. §13.1: a relative path resolves against the directory mint was run from. §18.2: the README documents --notes-file with its command. The unwind's reset (task 3-4) and regenerate (task 3-3) are other tasks.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/git/exclusion.go:6-19 — `git.Exclusion`, a repo-root-relative path whose `Pathspec()` yields `-- :(top,literal,exclude)<path>` (nil for the zero value, so a run without the flag is argv-identical to as-built)
  - internal/engine/release.go:354-368 — the file is read before root resolution (before any stash, check or gate), then `notesExclusion` derives the exclusion
  - internal/engine/release.go:912-955 — `notesFilePath` (WorkDir-relative, process cwd in production), `notesExclusion` (zero for no file, a file outside the repo, or a git-ignored file, probed with `git -C root ls-files --others --ignored --exclude-standard -- :(top,literal)<rel>`), `repoRelativePath` (resolves the directory and root through symlinks so a subdirectory or symlinked cwd names the right root-relative path)
  - internal/preflight/preflight.go:82-113 — `RunLocalGates`/`CheckCleanTree` append the exclusion to `git status --porcelain`; any other entry still fails the gate
  - internal/engine/autostash.go:54-61 — `git stash push --include-untracked -- <exclusion>`; when the notes file is the only change git prints "No local changes to save", so no pop is deferred (release.go:450-454)
  - internal/record/commit.go:33-52 — `CommitDirtyTree` excludes the path from the porcelain probe and `git add -A`, so a hook that changed nothing makes no artifact commit
  - internal/record/commit.go:80-102, 155-190 — `CommitBookkeeping` and the shared `CommitStaged`: when the notes file has no staged edits, the as-built `git commit -m`; when it has, the other staged paths (read with the exclusion) are committed by name through `--pathspec-from-file`, leaving the notes file's index entry staged. An empty remainder makes no commit (artifact) or fails with `ErrNothingStaged` (bookkeeping)
  - internal/engine/release.go:471, 522, 655 — the exclusion threads into preflight, the pre_tag artifact commit and the bookkeeping commit
  - README.md:107 — documents the in-repo file, untracked or tracked with uncommitted edits, kept out of the clean-tree check, the stash and every release commit (artifact and bookkeeping), and left with its staged/unstaged split
- Notes: The answers commit (`record.CommitFile`, which commits the whole index) is not reached on a --notes-file run, since `SelectBody` returns the supplied body before the exclusion check (internal/notes/select.go:184-185), so "every commit the release makes" holds. The status and stash probes run from the process cwd, but the exclusion is `top`-anchored and those commands imply a whole-tree positive pathspec beside an exclude-only one, so CLAUDE.md seam 1 (cwd-relative pathspecs run at the root) is respected in substance. All mutations go through `git.Mutator` (seam 2). No drift from the plan.

TESTS:
- Status: Adequate
- Coverage:
  - internal/engine/release_notesfile_git_test.go:217 — untracked notes.md, tracked NEXT_RELEASE.md unstaged, and tracked NEXT_RELEASE.md staged: exact status argv with the exclusion, `add CHANGELOG.md` only, the as-built commit or the pathspec commit naming only CHANGELOG.md, the file never named as a bare git argument, file contents unchanged, RunFinished
  - :258 — another untracked or modified path still fails the clean-tree check, with no mutation
  - :298 / :323 — --autostash stashes with the exclusion and pops; with the notes file the only change, nothing stashed and no pop runs
  - :348 — pre_tag hook that changes files (with and without staged notes edits) gets an artifact commit of the other paths only; a hook that changes nothing gets no artifact commit
  - :422 — subdirectory, climbing `../` and symlinked-cwd cases name the right root-relative path
  - :469, :506, :527, :588 — outside-the-repo and git-ignored files run as built; the ignore probe's argv (literal glob characters) and its failure path
  - internal/record/commit_test.go:343-544 — CommitDirtyTree/CommitBookkeeping/CommitStaged exclusion routes, empty-remainder and probe/read failures, exact argv and stdin pathspecs
  - internal/preflight/preflight_test.go:85, :122 and internal/git/exclusion_test.go:10 — the gate and Pathspec unit pins
- Notes: Each test fails if its exclusion is dropped from its git step. Per the project's test idiom (no real git), they pin argv, so git's own handling of the pathspec is recorded under UNSETTLED. Not over-tested: the engine tests prove behaviour across the run, and the record/preflight tests pin the branch logic (staged-edit probe, empty remainder, failures) that the engine tests do not reach.

CODE QUALITY:
- Project conventions: Followed (Runner/Mutator seams, sentinel ErrNothingStaged with %w wrapping, external test packages, t.Parallel, exact-argv assertions)
- SOLID principles: Good. `git.Exclusion` is the one owner of the pathspec form, and each git step takes it as a parameter
- Complexity: Low
- Modern idioms: Yes (filepath.IsLocal, slices.Equal)
- Readability: Good. WHY-comments state the contracts (why a bare exclusion pathspec on `git commit` would sweep in unstaged edits, why an ignored file gets no exclusion) and match the code
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "Afterwards `notes.md` is still untracked with its contents unchanged, and no commit the release made contains it" — reading settles the argv (the file is never staged by name, and `add -A` carries the exclusion). That real git honours `:(top,literal,exclude)` on `git status --porcelain` and `git add -A` needs observing: run the release against a scratch repo with an untracked notes.md and inspect `git status` and `git show --stat` for each release commit
- "afterwards the file carries the same edits, staged if they were staged and unstaged if not" — needs `git commit -m … --pathspec-from-file=- --pathspec-file-nul` (`--only` semantics) observed in a real repo with NEXT_RELEASE.md partly staged: after the release, `git diff --cached` and `git diff` on the file should match their pre-run output
- "The notes file stays in the work tree through the run with its contents and staged state unchanged. Given the notes file is the only uncommitted change, nothing is stashed and no `git stash pop` runs" — needs real `git stash push --include-untracked -- :(top,literal,exclude)<path>` and the later `git stash pop`: check the file's staged and unstaged state is unchanged afterwards, that the other WIP is restored, and that git prints "No local changes to save" when the notes file is the only change
- "the file that path names is the one left out of the clean-tree check, the stash and the commits" — the root-relative path derivation is settled by reading and tests. Settling the rest means running `mint release --notes-file <rel>` from a subdirectory of a real repo, to confirm that a top-anchored exclude-only pathspec on `git status` and `git stash push` still covers the whole tree from there
