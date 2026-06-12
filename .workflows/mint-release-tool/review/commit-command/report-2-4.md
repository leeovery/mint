TASK: Flag-aware empty-staging messaging matrix (tick-193ebe, suffix 2-4)

ACCEPTANCE CRITERIA:
- mint commit -A on a pristine tree -> 'nothing to commit, working tree clean' (keyed on clean tree state, not the -A flag)
- Bare mint commit with unstaged changes but nothing staged -> 'no changes staged — use -a/--all, -A/--add-all, or git add'
- mint commit -a when the only changes are untracked -> 'no tracked changes to stage — use -A/--add-all to include untracked files'
- The message is selected by the actual post-mode tree state, not by the flag passed
- No AI/claude is invoked in any empty-staging case (short-circuits before generate)
- No git add and no commit runs in any empty-staging case
- The 1-6 not-a-git-repo fail-loud and read-only posture preserved; dropped gates remain unimplemented
- No empty case routed to the $EDITOR fallback (Phase 3) — all hard fail-loud

STATUS: Complete

SPEC CONTEXT:
Specification → Staging Model → "Empty-staging handling — fail loud, mirroring git's messaging" and Preflight & Safety → "Something to commit". The empty-staging case fans out under -a/-A: a clean tree yields git's clean-tree line, while a dirty tree whose chosen mode staged nothing yields mint's flavour of git's "no changes added to commit", naming the modes that would help. The discriminator MUST be the actual post-mode tree state, not the flag. The AI must never see an empty diff; no git add / no commit in any empty case; this is a hard fail-loud (not the $EDITOR fallback, which is Phase 3). Extends the 1-6 preflight without re-implementing repo-present detection or the dropped gates (clean-working-tree, on-release-branch, remote-in-sync).

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/commit/preflight.go:37-41 — the three sentinels (errNothingToCommit, errNoChangesStaged, errNoTrackedChanges), verbatim git-style lowercase messages, U+2014 em dashes per spec.
  - internal/commit/preflight.go:66-75 — checkSomethingToCommit: computes would-be-staged emptiness read-only, returns nil when non-empty, else selects the fail-loud cause.
  - internal/commit/preflight.go:96-109 — wouldStageNothing: folds sourcesForMode probes (all-specs-empty), encoding the AddAll tracked-first/short-circuit/else-untracked rule.
  - internal/commit/preflight.go:167-186 — emptyStagingError: keys the message on `git status --porcelain` (clean -> errNothingToCommit; else All -> errNoTrackedChanges, StagedOnly -> errNoChangesStaged, AddAll -> defensive clean-tree fall-back).
  - internal/commit/run.go:294-296 — wired BEFORE the --no-ai branch (line 303) and generate (line 307), so no empty case can reach the AI or the $EDITOR fallback.
  - internal/commit/source.go:89-101 — single sourcesForMode descriptor shared by preflight probes and the L1 diff, making the no-drift invariant structural.
- Notes:
  - Message keyed on tree state, not flag: confirmed — emptyStagingError probes `git status --porcelain` and only then switches on mode; -A always lands on the clean-tree line because an empty -A set implies a clean tree (the AddAll switch arm is documented unreachable and defensively returns the clean-tree message). Matches the AC exactly.
  - diff_exclude threaded through (cfg.DiffExclude at run.go:294 → probeArgs → excludePathspecs), so the emptiness verdict measures the SAME post-exclusion source as the AI's L1 diff; an all-excluded set fails loud here rather than reaching the transport with a blank diff. This is beyond the literal AC but is the correct, spec-aligned behaviour (Scope: diff_exclude applies to commit's diff) and is the right place to enforce it.
  - 1-6 posture preserved: repo-present detection still lives in resolveRoot/gitrepo.ResolveRoot (run.go:271, surfaced as a "preflight" StageFailed); this task extends, not replaces, it. Dropped gates (clean-working-tree, on-release-branch, remote-in-sync) remain unimplemented — none were added. No $EDITOR routing: the preflight returns before both the --no-ai branch and generate's fallback branches.
  - All probes run via RunInDir(root) — anchored at repo root so the cwd-relative `-- .` selector stays whole-tree from any invocation directory. Correct and explained.

TESTS:
- Status: Adequate
- Coverage:
  - internal/commit/staging_empty_test.go covers the full matrix: -A pristine -> clean line (TestRun_AddAllOnPristineTree_ReportsCleanTree); -a pristine -> clean line (TestRun_AllOnPristineTree_ReportsCleanTree); bare unstaged -> no-changes-staged (TestRun_BareCommitUnstagedChangesNothingStaged_PointsAtStagingFlags); -a only-untracked -> point at -A (TestRun_AllWithOnlyUntrackedChanges_PointsAtAddAll); keyed-on-tree-state-not-flag (TestRun_EmptyStagingMessageKeyedOnTreeStateNotFlag, with explicit negative assertions that the message is NOT a flag-driven guidance); no-AI (TestRun_EmptyStaging_NoAIInvoked, transport.calls()==0); no-add/no-commit (TestRun_EmptyStaging_NoGitAddNoCommit).
  - internal/commit/run_test.go covers 1-6 preservation: not-a-git-repo fail-loud, no AI, no commit (TestRun_NotAGitRepository_FailsLoudNoAINoCommit); empty staged index -> clean line + StageFailed narration (TestRun_EmptyStagedIndex_FailsLoudWithGitMessage, _NoAIInvoked).
  - internal/commit/staging_excluded_test.go covers the all-excluded empty cases per mode and asserts the exact probe argv carries `:(exclude)` pathspecs (no-drift with L1).
  - internal/commit/source_test.go proves probe argv = L1 argv + --name-only (TestProbeArgv_*) and that the emptiness verdict agrees with the L1 source per mode incl. the AddAll short-circuit (TestEmptinessVerdictAgreesWithL1Source_PerMode).
- Notes:
  - Every AC has a direct, behaviour-level test that would fail if the feature broke (message text, StageFailed narration, transport-call count, add/commit invocation counts are all asserted).
  - Not over-tested: the run-level matrix tests assert message + narration; the unit-level source/probe tests target the no-drift invariant and argv derivation — distinct concerns, no redundant happy-path duplication. The exclude tests pair a behaviour test with an argv-shape test per mode, which is justified because the no-drift invariant is load-bearing per the spec.
  - The AddAll "unreachable" switch arm in emptyStagingError (preflight.go:179-182) has no direct unit test, but it is genuinely unreachable by construction (an empty -A set implies a clean tree, caught by the earlier `clean` branch), so a test would only exercise dead defensive code. Acceptable.

CODE QUALITY:
- Project conventions: Followed. Error strings are lowercase with no trailing punctuation (golang-error-handling rule); sentinels are pre-allocated package vars and matched/returned by identity, consistent with the sentinel-error idiom. Probes go through the consumed CommandRunner seam (FakeRunner-scriptable), matching the package's read-only idiom.
- SOLID principles: Good. Single source-of-truth for per-mode sources (source.go) cleanly separates the dispatch from both consumers; checkSomethingToCommit/wouldStageNothing/emptyStagingError each hold one responsibility.
- Complexity: Low. wouldStageNothing is a single fold; emptyStagingError is a guard + a 3-arm switch. Clear paths.
- Modern idioms: Yes. Short-circuiting fold, context-threaded runner calls, errors.New sentinels.
- Readability: Good. Intent is explicit and the keyed-on-tree-state-not-flag invariant is documented at the decision site.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/commit/preflight.go:179-182 — the AddAll arm of emptyStagingError is documented unreachable and defensively returns the clean-tree message. Consider whether to keep the defensive branch or collapse AddAll into the default/clean path; decide whether a guard-panic or a comment-only invariant better signals "unreachable by construction". Pure judgment call, no functional defect.
- [quickfix] internal/commit/preflight.go:192-197 — gitOutputEmpty uses strings.TrimSpace to decide emptiness, but the untracked probe runs `ls-files ... -z` (NUL-terminated, no newline). TrimSpace does not strip a trailing NUL, so a non-empty untracked set stays non-empty (correct) and an empty set stays empty (correct) — behaviour is right today. The latent sharp edge: TrimSpace is the wrong primitive for -z output if this helper is ever reused where a lone NUL or NUL-only output must count as empty. A focused guard (treat NUL as whitespace, or check len after trimming NUL) would harden the shared probe; no current bug.
