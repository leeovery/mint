TASK: 2-2 — Compute the would-be-staged diff read-only per mode (tick-011ac6)

ACCEPTANCE CRITERIA:
- -a produces a diff of tracked modifications + deletions, excluding untracked files
- -A produces a diff that includes untracked files (plus tracked mods + deletions)
- Deletions captured in the diff under both -a and -A
- The index is unmutated after the would-be-staged computation (no git add ran; pre-existing user staging untouched)
- diff_exclude globs applied to the would-be-staged diff (excluded files never reach the prompt) — commit's own L1 glue per 1-3 (:(exclude) pathspec)
- max_diff_lines guard applied to the would-be-staged diff via commit's own L1 glue consuming notes.CheckDiffSize/ErrDiffTooLarge (per 1-3)
- StagedOnly continues to use git diff --cached unchanged (Phase 1 path intact)
- All diff computation is read-only via the consumed CommandRunner/fake; no git add invoked

STATUS: Complete

SPEC CONTEXT:
Commit's core invariant is "mutate nothing until gate-accept." Under -a/-A the message must be generated from the would-be-committed diff computed READ-ONLY, no git add, index byte-for-byte unchanged (spec: Commit Flow/Lifecycle, Staging Model, AI Engine three-layer split). -a = git commit -a (tracked mods + deletions, no untracked); -A = git add -A (everything incl untracked); both must capture deletions. Phase 1 hardwired L1 to `git diff --cached`; this task extends commit's L3 source selection so the L1 source is chosen by the 2-1 StagingMode while reusing the SAME exclusion (:(exclude)), size guard (notes.CheckDiffSize), compose, and L2 transport — only the source differs.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/commit/source.go — single source-of-truth per-mode source selection: sourceSpec/sourceKind, stagedBaseArgs/trackedBaseArgs/untrackedBaseArgs, sourcesForMode (StagingMode->[]sourceSpec), sourceArgs (base + excludePathspecs tail).
  - internal/commit/generate.go:156-292 — sourceDiff/renderSource/diffSourceText/untrackedAdditions/untrackedAdditionDiff/excludePathspecs. All reads via runner.RunInDir(ctx, root, nil, "git", ...). Size guard (generate.go:115) + compose unchanged across modes.
  - internal/commit/preflight.go — wouldStageNothing consumes the SAME sourcesForMode descriptor (emptiness verdict cannot drift from the L1 source).
  - Threading: cmd/mint/commit_flags.go (resolveStagingMode) -> Deps.Staging -> run.go:294 (checkSomethingToCommit) and run.go:669/682 (NewGenerator).
- Notes:
  - -a source = `git diff HEAD -- .` — captures tracked mods + deletions (staged and unstaged) and excludes untracked; correct for git commit -a semantics. No ls-files call on the -a path.
  - -A source = tracked `git diff HEAD -- .` + untracked enumeration `git ls-files --others --exclude-standard -z -- .`, each untracked file rendered as `git diff --no-index -- /dev/null <file>`. Avoids git add and git add --intent-to-add entirely, so the index is never touched. The -z + NUL-split correctly handles C-quotable filenames (non-ASCII/quotes/backslashes).
  - --no-index exit-1 ambiguity (differ vs access-error) resolved by stdout-non-empty discriminator (generate.go:267-279) — sound; a genuine access error surfaces rather than silently dropping a file.
  - StagedOnly path is byte-identical to Phase 1 (`git diff --cached -- .`).
  - Read-only is structural: every source goes through diffSourceText/untrackedAdditions/untrackedAdditionDiff, none of which can emit a mutating verb; the only `git add` in the package is stageForMode (run.go:712, task 2-3, on accept).

TESTS:
- Status: Adequate
- Coverage (internal/commit/generate_test.go + source_test.go + staging_excluded_test.go):
  - AllMode_UsesTrackedWorktreeDiffExcludingUntracked (asserts `git diff HEAD -- .` and that ls-files is never called) -> AC1.
  - AddAllMode_IncludesUntrackedPlusTrackedDiff (tracked diff + ls-files -z + per-file --no-index; both reach the prompt) -> AC2.
  - AllMode_CapturesTrackedDeletion / AddAllMode_CapturesTrackedDeletion (deletion hunk reaches prompt under both modes) -> AC3.
  - AllMode_RunsNoGitAdd / AddAllMode_RunsNoGitAdd (hasGitAdd false) -> AC4. hasGitAdd matches arg0 == "add", so it catches add -u / add -A / add --intent-to-add alike.
  - AllMode_DiffExcludeAndSizeGuardApply, AddAllMode_UntrackedRespectsDiffExclude (:(exclude) on both halves), AddAllMode_SizeGuardAppliesToCombinedDiff (notes.ErrDiffTooLarge on combined, transport never called) -> AC5, AC6.
  - StagedOnly_StillUsesGitDiffCached (exactly one git call, `git diff --cached -- .`) -> AC7.
  - source_test.go (white-box) proves preflight probe argv == L1 source argv + --name-only (diff) / verbatim ls-files prefix (untracked), and that wouldStageNothing and sourceDiff agree per mode incl. the AddAll short-circuit — locks the "one source, cannot drift" invariant structurally.
  - staging_excluded_test.go drives the same exclusion behaviour end-to-end through Run for StagedOnly/-a/-A (all-excluded -> fail loud, no AI, no add, no commit).
  - Edge cases: AddAllMode_UnusualFilenamesPassThroughRaw (-z, NUL split, two entries); AddAllMode_SurfacesNoIndexAccessError (exit-1 empty stdout) and AddAllMode_SurfacesNoIndexOtherNonZeroExit (exit-129) cover both --no-index failure branches; SurfacesL1GitError covers a failing source read.
- Notes: Not under-tested — every AC and the spec edge cases (deletions, untracked exclusion, read-only, exclude+guard on the would-be-staged diff, raw filenames) have a focused test, and an L1 git failure never masquerades as an empty diff. Not over-tested — source_test.go (structural single-sourcing) and generate_test.go (Generator behaviour) test different layers and do not duplicate. Assertions are behaviour-facing (argv, prompt content, transport-call count, presence of git add), not implementation internals.

CODE QUALITY:
- Project conventions: Followed. Seam-based git access (runner.CommandRunner via RunInDir), table-driven parallel tests, testify-free explicit assertions consistent with the package, errors wrapped with %w preserving errors.Is matching (notes.ErrDiffTooLarge vs ai.* sentinels stay distinguishable).
- SOLID principles: Good. source.go is the single dispatch point both consumers (preflight + generate) derive from; sourceSpec/sourceKind cleanly separate the diff-source vs untracked-list rendering; Generator depends on the injected runner/Transport seams (DIP).
- Complexity: Low. sourceDiff is a fold over sourcesForMode; renderSource a 2-way switch; the only subtlety (--no-index exit-1 discrimination) is isolated and documented.
- Modern idioms: Yes. strings.Builder, strings.Split on NUL, context threaded throughout.
- Readability: Good — arguably heavy doc-comment density, but the comments are load-bearing (they encode the cannot-drift and read-only invariants) rather than noise.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
