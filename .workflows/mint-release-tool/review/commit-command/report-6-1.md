TASK: 6-1 — Single-source the emptiness verdict from the exclusion-filtered diff so the AI is never invoked on an empty post-exclusion diff

ACCEPTANCE CRITERIA:
- When every staged/changed file matches a `diff_exclude` glob, the command fails loud with the exact spec empty-staging message and performs no git mutation (no add/commit/push).
- `transport.Generate` is never called when the post-`diff_exclude` would-be-staged diff is empty.
- The emptiness verdict and the AI's L1 source diff are computed from the same exclusion-filtered source (no remaining name-only probe that omits `excludePathspecs` while the AI path applies it).
- Existing behaviour for genuinely non-empty staged sets and for the already-handled empty-staging case is unchanged.

STATUS: Complete

SPEC CONTEXT:
Staging Model + Preflight & Safety: "Empty staging (nothing to commit after staging) → fail loud; never invoke the AI on an empty diff." `diff_exclude` globs apply to commit's diff exactly as to release's — excluded files (bundles, lockfiles, minified output) are never fed into message generation. The empty-case message is keyed on the ACTUAL post-mode tree state, not the flag passed: clean tree → "nothing to commit, working tree clean"; changes exist but mode staged none → mint's flavour of git's guidance (bare → "no changes staged — use -a/--all, -A/--add-all, or git add"; -a with only-untracked → "no tracked changes to stage — use -A/--add-all to include untracked files"). The task closes a real prior gap: the preflight previously ran name-only probes WITHOUT the `:(exclude)` pathspecs while L1 applied them, so an all-excluded set passed preflight and reached transport.Generate with a blank diff (CheckDiffSize("",n)==nil validates nothing).

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/commit/preflight.go:66-75 (checkSomethingToCommit now takes diffExclude), :96-109 (wouldStageNothing threads diffExclude through every probe), :118-123 (probeArgs derives from the shared sourceArgs), :167-186 (emptyStagingError keyed on tree state).
  - internal/commit/run.go:294 (preflight called with cfg.DiffExclude before generate).
  - internal/commit/source.go (the shared sourcesForMode / *BaseArgs / sourceArgs single source — restructured by the later 7-1 refactor so probe and L1 derive from ONE descriptor).
  - internal/commit/generate.go:109-132,156-211,286-292 (L1 sources apply the SAME excludePathspecs).
- Notes: The fix is structural, not a parallel check: both the preflight probe (probeArgs) and the L1 source (sourceArgs) build their argv from one excludePathspecs/base, so they CANNOT drift — the probe argv is provably the L1 argv plus `--name-only`. The emptiness gate sits in Run BEFORE generateMessage, so an all-excluded set fails loud before any transport call. No git mutation can occur on this path (return precedes commitAccept). All four acceptance criteria are met. The third criterion ("no remaining name-only probe that omits excludePathspecs") is guaranteed structurally by the shared source descriptor, not merely by convention.

TESTS:
- Status: Adequate
- Coverage (internal/commit/staging_excluded_test.go):
  - TestRun_StagedAllExcluded_FailsLoudNoAINoMutation — staged-only all-excluded: fails loud with noChangesStagedMessage, transport 0 calls, 0 adds, 0 commits.
  - TestRun_PreflightProbeCarriesExcludePathspecs — exact probe argv `diff --cached --name-only -- . :(exclude)*.min.js` (the single-source proof for StagedOnly).
  - TestRun_AllModeAllExcluded_FailsLoudNoAI + ...ProbeCarriesExcludePathspecs — the -a worktree/deferred path (the spec's "same all-excluded scenario on the worktree/deferred-staging path"): fails loud with noTrackedChangesMessage, no AI, exact probe argv `diff HEAD --name-only -- . :(exclude)*.min.js`.
  - TestRun_AddAllModeAllExcluded_FailsLoudNoAI + ...UntrackedProbeCarriesExcludePathspecs — the -A path including the untracked probe: fails loud (defensive clean-tree line), no AI, asserts BOTH tracked and untracked probe argvs carry `:(exclude)*.min.js`.
  - TestRun_StagedNonExcludedChange_ReachesGenerate — the regression: a non-excluded staged change still passes preflight, reaches Generate (transport 1 call), and commits the body verbatim with diff_exclude configured.
  - The "normally-empty staging set still fails loud as before" regression is covered by the pre-existing staging_empty_test.go (clean tree, bare-no-staged, -a-only-untracked, -A-clean), unchanged by this task.
- Notes: Tests drive the REAL Generator + REAL git Mutator over a single FakeRunner with a recording transport — behaviour-level, not implementation mocking. They exercise config.Load end-to-end via a real .mint.toml (writeDiffExclude), so the cfg.DiffExclude thread is genuinely verified, not stubbed. The argv assertions verify the single-source invariant directly and are not redundant with the behavioural fail-loud assertions (different concern: drift-proof argv vs no-AI/no-mutation outcome). Not over-tested: each test asserts one scenario; the per-mode argv tests are separate from the per-mode behaviour tests by design and each adds a distinct guarantee. Message constants asserted (noChangesStagedMessage/noTrackedChangesMessage/nothingToCommitMessage) are the exact spec strings and match the production sentinels in preflight.go:38-41.

CODE QUALITY:
- Project conventions: Followed. Consumer-side seams (Mutator/Transport defined where consumed), git_safe mutation sink, read-only reads on the plain Runner, RunInDir(root) anchoring, sentinel errors + errors.Is, %w wrapping — all consistent with the golang-* skill lens and the surrounding commit package.
- SOLID principles: Good. The shared sourcesForMode/sourceArgs descriptor (single source of truth) is textbook DRY against drift — the exact anti-drift the task demanded. wouldStageNothing's all-specs-empty fold encodes the AddAll composition in one place.
- Complexity: Low. wouldStageNothing is a short-circuiting fold; emptyStagingError is a flat switch; nameOnly is a safe non-aliasing splice (copies head into a fresh slice before append).
- Modern idioms: Yes. strings.Builder, append-to-fresh-slice, NUL-split untracked enumeration.
- Readability: Good. Doc comments are thorough (arguably heavy) and explain the drift invariant precisely.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/commit/preflight.go:89-92 — the wouldStageNothing doc-comment renders the untracked probe as `git ls-files --others --exclude-standard -- . :(exclude)…`, omitting the `-z` flag that untrackedBaseArgs() actually carries (and that source_test.go / staging_excluded_test.go:248 assert). Add `-z` to the comment so the documented argv matches the executed one.
- [do-now] internal/commit/preflight.go:136-153 — stagedProbeArgs/trackedProbeArgs/untrackedProbeArgs are now only referenced by source_test.go (production wouldStageNothing routes through probeArgs after the 7-1 restructure). The doc already flags them as "the single checkable builders" for tests; consider a one-line note that they are test-facing builders so a future reader does not mistake them for live preflight callers (or drop them if the argv tests assert the executed invocations directly).
