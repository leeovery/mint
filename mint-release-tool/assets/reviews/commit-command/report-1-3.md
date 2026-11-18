TASK: Bind staged-diff source through L1/L2 (L3 glue) — tick-3a7287 (commit-command-1-3)

ACCEPTANCE CRITERIA:
- L1 source is `git diff --cached` (staged-only) for the bare-commit path
- diff_exclude globs remove excluded files before generation (never reach the prompt) — mapped from cfg.DiffExclude to :(exclude) pathspecs on the `git diff --cached` invocation
- max_diff_lines guard applied after diff_exclude, before any L2 call, via consumed notes.CheckDiffSize(diff, cfg.MaxDiffLines) — not re-counted in commit code
- L2's transport, validation, and one retry are consumed (ai.NewTransport behind commit's own 1-method Transport interface), not reimplemented
- Returned body used whole (no parsing/splitting), suitable for the commit sink
- Failure surfaces as a distinguishable typed failure via errors.Is — notes.ErrDiffTooLarge vs ai.ErrGenerationFailed / ai.ErrTimeout / ai.ErrCommandMissing
- No -a/-A would-be-staged source implemented (deferred to Phase 2)
- All git/AI calls go through the consumed CommandRunner/fake

STATUS: Complete

SPEC CONTEXT:
The commit verb consumes a three-layer AI engine. L1 (context builder, git-aware) produces the content to describe, parameterised by source — commit uses the staged diff (`git diff --cached`) — and applies diff_exclude + max_diff_lines (the genuinely shared git logic). L2 (AI message engine, content-agnostic) takes a finished prompt, runs the transport, validates, retries once, returns the body. L3 (per-verb glue) picks the L1 source, supplies the prompt, decides sinks. Spec (AI Engine — Three-Layer Split; Commit's binding to the engine; Detection ordering for the oversized case) pins: max_diff_lines is evaluated at L1 after diff_exclude and before any L2 call; an over-limit diff short-circuits L2 entirely (a generate-SKIP, not a failure). The typed-failure distinction (oversized vs generation-failure) is what lets Phase 3 route oversized → $EDITOR fallback separately. This task is the L3 binding; failure ROUTING ($EDITOR / --no-ai) is Phase 3 — here only surface the outcome.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/commit/generate.go — Generator struct + NewGenerator (54-68); Generate / GenerateWithContext (96-132) orchestrate sourceDiff → notes.CheckDiffSize → ResolveInstructions+ComposePrompt → transport.Generate in exactly the spec order; Transport seam defined at the consumer (29-31); excludePathspecs maps cfg.DiffExclude → :(exclude) (286-292).
  - internal/commit/source.go — single source-of-truth for the per-mode argv: stagedBaseArgs() = `diff --cached -- .` (57-59); sourcesForMode (89-101); sourceArgs appends the exclude tail (106-108).
  - internal/commit/run.go — production wiring: generateMessage (668-671) / regenerateMessage (681-684) build the Generator; commitTransport (705-710) returns the injected test seam when set, else ai.NewTransport(deps.Runner, ai.Config{AICommand: cfg.AICommand}) — production leaves deps.Transport nil and gets the real L2.
- Notes:
  - Every AC verified. L1 = `git diff --cached -- .` (stagedBaseArgs); diff_exclude → :(exclude) pathspecs in config order, appended after `-- .`, with NO inherited CHANGELOG.md/version_file tiers (excludePathspecs carries only the configured globs); size guard is the CONSUMED notes.CheckDiffSize (no re-count in commit code), applied after the exclude-filtered diff returns and before transport.Generate; body returned byte-identical (Generate returns transport body verbatim — no trim/split); typed failures preserved via %w (errors.Is matches notes.ErrDiffTooLarge and the three ai sentinels); all git reads go through runner.RunInDir(ctx, g.root, …) and the AI call through the consumed ai.Transport (itself over the same CommandRunner seam).
  - DRIFT (benign, expected): AC#7 ("No -a/-A would-be-staged source") was the task-TIME Phase-1 constraint. The current code has progressed past Phase 1 — sourcesForMode now also serves All (-a) and AddAll (-A) modes (Phase 2). This is legitimate downstream evolution, NOT a regression: the StagedOnly default path (the zero value) remains byte-identical to Phase 1 (`git diff --cached -- .`), proven by TestGenerator_Generate_StagedOnly_StillUsesGitDiffCached and TestGenerator_Generate_DoesNotComputeWouldBeStagedDiff. The per-mode dispatch is colocated with the preflight emptiness probe (source.go) so the preview and the emptiness check provably read one exclusion-filtered source — a structural invariant, well executed.
  - cwd-pinning to the repo root (RunInDir(g.root)) for every L1 read is a correct, well-documented refinement (the `-- .` selector and ls-files are cwd-relative; pinning makes preview and accept-time mutation agree regardless of invocation dir).

TESTS:
- Status: Adequate
- Coverage (internal/commit/generate_test.go):
  - Staged-diff source via `git diff --cached -- .` — TestGenerator_Generate_ObtainsStagedDiffViaGitDiffCached (exact-argv).
  - diff_exclude → :(exclude) in config order, no inherited CHANGELOG.md — DiffExcludeMapsToExcludePathspecs; excluded path never reaches the prompt — ExcludedFilesNeverReachThePrompt.
  - max_diff_lines after exclude, before L2, short-circuits transport (calls()==0), surfaces notes.ErrDiffTooLarge — MaxDiffLinesGuardAppliedBeforeTransport; inclusive-boundary at-ceiling passes — GuardCountsPostExclusionDiff.
  - Body returned whole byte-identical — ReturnsValidatedBodyWhole.
  - Composed prompt (instructions before diff) + commit's own knobs — FeedsComposedPromptWithDefaultInstructionsAndDiff, AppliesCommitPromptKnobs.
  - Typed distinguishability — SurfacesDiffTooLargeDistinctFromGenerationFailure + SurfacesTransportFailuresTyped (one subtest per ai sentinel, each asserting NOT-ErrDiffTooLarge).
  - Consumed one-retry via the REAL ai.Transport over a FakeRunner (empty 1st claude attempt, good 2nd) asserting exactly 1 git + 2 claude calls — ConsumesL2OneRetryNotReimplemented. Strong: this proves the retry lives in L2, not commit.
  - Staged-only (no -a/-A flags) — DoesNotComputeWouldBeStagedDiff, StagedOnly_StillUsesGitDiffCached.
  - L1 git error surfaces, transport never reached — SurfacesL1GitError.
  - Compile-time seam conformance: `var _ commit.Transport = (*ai.Transport)(nil)` (19) pins that the real L2 satisfies the consumer seam without coupling production code to ai concretions.
  - Plus thorough Phase-2 -a/-A coverage (tracked-vs-HEAD, untracked --no-index rendering, -z raw-path handling, no-git-add read-only guarantee, --no-index error discrimination, combined-diff size guard).
- Tests verify behaviour through the runner seam (exact argv, invocation counts, prompt content) rather than implementation internals; a broken feature would fail (e.g. dropping the guard, re-running the transport, or trimming the body each have a dedicated failing assertion).
- Notes: Mild redundancy between MaxDiffLinesGuardAppliedBeforeTransport and SurfacesDiffTooLargeDistinctFromGenerationFailure — both seed an oversized diff and assert errors.Is(notes.ErrDiffTooLarge); the latter only adds the "NOT ai.ErrGenerationFailed" leg, which SurfacesTransportFailuresTyped already covers from the opposite direction. Acceptable (each documents a distinct AC facet); not harmful over-testing.

CODE QUALITY:
- Project conventions: Followed. Consumer-defined 1-method Transport interface (golang-structs-interfaces — accept-interfaces-at-the-consumer), DI via NewGenerator, error wrapping with %w preserving sentinels (golang-error-handling), table-driven subtests + t.Parallel + t.Context (golang-testing), runner seam for all externals (golang-safety). Matches the notes-engine consumed pattern it mirrors.
- SOLID principles: Good. Single responsibility per function (sourceDiff selects, renderSource dispatches, diffSourceText/untrackedAdditions render, size guard + compose + transport are separate steps). The Transport seam at the consumer is textbook interface segregation + dependency inversion. sourcesForMode + the shared sourceArgs tail give one open/closed extension point for source modes.
- Complexity: Low. Generate is a flat 4-step pipeline; the per-mode switch is a single small dispatch table.
- Modern idioms: Yes. strings.Builder for concatenation, strings.Split on NUL for -z paths, errors.Is throughout, append(append([]string{}, …)…) to avoid aliasing the base slice in sourceArgs.
- Readability: Good — arguably exemplary doc comments explaining the WHY (read-only computation, cwd-pinning rationale, the --no-index exit-1 discriminator, the -z load-bearing reasoning). Comments are dense but accurate and load-bearing.
- Issues: None blocking. One minor organizational note (below).

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/commit/generate_test.go:273,394 — SurfacesDiffTooLargeDistinctFromGenerationFailure overlaps MaxDiffLinesGuardAppliedBeforeTransport (both assert errors.Is(notes.ErrDiffTooLarge) on an oversized diff); consider folding the "NOT ai.ErrGenerationFailed" assertion into the guard test and dropping the standalone case, or keep as a documented distinctness witness. Decision (whether the explicit distinctness witness earns its keep) — hence idea, not quickfix.
- [quickfix] internal/commit/generate.go:286-292 — excludePathspecs lives in generate.go while every other per-mode argv builder (stagedBaseArgs, trackedBaseArgs, untrackedBaseArgs, sourceArgs, sourcesForMode) lives in source.go, the declared "single source-of-truth for per-mode git source selection." Moving excludePathspecs into source.go beside sourceArgs (its only caller) colocates the full argv-assembly surface. Mechanical move at a known location, no logic change.
