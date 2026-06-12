TASK: 1-4 — Wire bare `mint commit` generate-and-commit thread (commit-command-1-4)

ACCEPTANCE CRITERIA:
- Bare `mint commit` generates a Conventional Commits message from the staged diff (AI infers type)
- Scope is off by default in the produced message
- The commit is created via the consumed git_safe wrapper, not the raw runner
- The commit message text is the generated body verbatim — no commit_prefix/branding
- The bare path is staged-only — no git add is run
- No -a/-A, -p, --no-ai, or gate e/r behaviour is implemented (deferred)
- The thread is exercised end-to-end with the fake runner + recording presenter (no real git/claude)

STATUS: Complete

SPEC CONTEXT:
Spec (commit-command/specification.md) frames `mint commit` as a thin standalone verb consuming the shared three-layer AI engine. Task 1-4 is the walking-skeleton vertical seam: L3 glue (1-3) + prompt composer (1-2) + [commit] config (1-1) threaded into a runnable bare `mint commit`. Relevant sections: Commit Flow / Lifecycle (read-only build → generate → commit via git_safe), Commit Message Format & Prompt (Conventional Commits 1.0.0, AI infers type, scope off by default, NO commit_prefix/branding), and CLI Surface. Deferred to later tasks: -a/-A staging (P2), --no-ai/$EDITOR (P3), gate e/r (P4), -p push (P5), empty-index preflight (1-6).

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/commit/run.go — Run orchestrator (generate → review gate → commitAccept → createCommit); createCommit (run.go:760) pipes the body via `git commit -F -` through deps.Mutator (git_safe), NOT the raw runner.
  - internal/commit/generate.go — L3 Generator: sourceDiff (`git diff --cached` for StagedOnly), notes.CheckDiffSize guard, ResolveInstructions + ComposePrompt, transport.Generate; body returned WHOLE (generate.go:96, 109-132).
  - internal/commit/source.go — single per-mode source descriptor; stagedBaseArgs `diff --cached -- .` for the bare path.
  - internal/commit/prompt.go — DefaultPrompt (Conventional Commits, infer type, omit scope, no branding) + ComposePrompt + ResolveInstructions.
  - internal/commit/surface.go — StageFailed narration + projectName.
  - internal/commit/staging.go — StagingMode (StagedOnly zero value = bare path, no git add).
  - cmd/mint/commit_flags.go + cmd/mint/main.go:300-354 — production wiring: ExecRunner read seam + git.NewMutator(r) commit sink; Transport left nil → real ai.Transport.
  - Touchpoints confirmed present: config.Commit/AICommand/ResolveCommitPrompt (internal/config/config.go), git.NewMutator/Mutate/WithBackoff (internal/git/mutator.go).
- Notes:
  - createCommit passes the body as []byte; git.Mutator.invoke (mutator.go:184) builds a FRESH bytes.NewReader per attempt, so a lock-retry re-pipes the full payload — the drained-reader hazard (a retried commit committing empty) is correctly avoided.
  - The bare path's StagedOnly default short-circuits stageForMode (run.go:737-739, no `git add`), satisfying staged-only.
  - run.go now legitimately spans later phases (P3 --no-ai/editor fallback, P4 e/r, P5 push). For the 1-4 deliverable specifically, the deferred behaviours are gated behind flags/branches (deps.NoAI, deps.Staging, deps.Push) that the bare path never enters — no drift into 1-4's scope.

TESTS:
- Status: Adequate
- Coverage: run_test.go drives the whole thread end-to-end over ONE FakeRunner + RecordingPresenter + scripted transport (no real git/claude), with a compile-time assertion that *git.Mutator satisfies commit.Mutator (run_test.go:25). Each acceptance criterion has a focused test:
  - Generates conventional message from staged diff: TestRun_BareCommit_GeneratesAndCommitsConventionalMessage, TestRun_NonEmptyStagedIndex_ProceedsToGeneration.
  - AI-inferred type appears / scope off by default: TestRun_InferredTypeAppears_NoScopeByDefault (asserts `fix:` prefix + no `(` in subject); prompt-level TestDefaultPrompt_InstructsInferTypeFromDiff / _InstructsScopeOmittedByDefault.
  - Commit via git_safe: TestRun_CommitCreatedViaGitSafe seeds a stale-lock contention then success and asserts TWO commit attempts — proving the lock-resilient retry ran (a raw runner would surface the first failure). This is the strongest possible "is it git_safe" assertion.
  - No branding: TestRun_MessageCarriesNoBranding (verbatim body + no 🌿); prompt-level TestDefaultPrompt_ForbidsCommitPrefixAndBranding.
  - Staged-only / no git add: TestRun_BarePathRunsNoGitAdd asserts EXACTLY 3 git calls (preflight diff + L1 diff + commit) and no `add`.
  - Body verbatim: TestRun_GeneratedBodyUsedVerbatim (multi-line body byte-for-byte + `-F -` argv).
  - Failure/abort guards: generate-failure aborts pre-commit, gate-no is a true no-op (no StageFailed), non-TTY-without-y / ErrInputClosed handling, ordering (ShowMessage before Prompt before commit), narration through the recording presenter. generate_test.go additionally exercises the L1/guard/compose/transport pieces in isolation.
- Notes:
  - Not under-tested: each acceptance criterion maps to a distinct behavioural assertion; the git_safe claim is proven via observable retry behaviour, not a mock-call count, so the test would fail if the sink were switched to the raw runner.
  - Not over-tested: tests assert observable outcomes (commit stdin, git argv sequence, presenter event kinds) rather than internal call shapes. Some shared scaffolding (seedDiffThenCommit, newCommitDeps) keeps setup DRY. The empty-index preflight tests (TestRun_EmptyStagedIndex_*, TestRun_NotAGitRepository_*) belong to 1-6 but co-reside here harmlessly and do not bloat 1-4's surface.

CODE QUALITY:
- Project conventions: Followed. Mutator/Transport seams defined at the consumer (decoupled from git/ai concretions), production wiring kept thin in main.go, errors wrapped with %w preserving sentinels for errors.Is, table-free focused tests with t.Parallel(), helper-driven assertions. Consistent with golang-* skill guidance (interfaces at consumer, error sentinels, dependency injection via Deps struct).
- SOLID principles: Good. Clear single responsibilities (Generator = L3 glue, source.go = per-mode source SoT, surface.go = narration, run.go = ordering). Mutator/Transport interface segregation; dependency inversion via injected Deps.
- Complexity: Low for the 1-4 path (linear Run → generate → reviewLoop → commitAccept). run.go as a whole is large because it now carries P3/P4/P5 branches, but each branch is well-isolated and documented.
- Modern idioms: Yes. errors.Is sentinels, strings.Builder, []byte stdin for retry-safe piping, append-copy in sourceArgs to avoid aliasing.
- Readability: Good. Doc comments are unusually thorough and accurately describe the invariants (mutate-nothing-until-accept, git_safe sink).
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/commit/surface.go:15-37 — surface and surfaceOutput are near-identical; surface could delegate to surfaceOutput with an empty output ("") to remove the duplicated StageFailed/return pair. Mechanical, single file, no behaviour change.
