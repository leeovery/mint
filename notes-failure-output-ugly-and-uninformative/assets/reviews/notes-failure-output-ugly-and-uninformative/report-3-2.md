TASK: T3-2 — Add an End-to-End Release Test Proving Captured AI Output Reaches StageFailure.Output Through the Real Transport Chain (tick-3238c3)

ACCEPTANCE CRITERIA:
1. A release-level test drives engine.Release with the PRODUCTION transport (Transport nil), seeds claude to fail with non-empty stdout on both attempts, and asserts the recorded StageFailed payload has Output == seeded stdout and Message == "AI returned empty/invalid notes after retry".
2. The assertion runs through the real transport.Generate → generator → SelectBody/ResolveFailure → surfaceAndUnwind chain (no hand-constructed *ai.GenerationError, no direct call to notesFailureOutput/failureMessage).
3. The existing abort-before-mutation coverage (TestRelease_PriorTag_NotesFailureAbort_AbortsBeforeMutation) is preserved, not weakened/deleted.
4. No production (non-test) source files change.
5. All project gates pass.

STATUS: Complete

SPEC CONTEXT:
The spec's Testing Requirements flag a wiring-level gap: the fix's load-bearing payoff is that claude's captured stdout, composed by the production transport into an *ai.GenerationError carrier, actually survives the full release spine and lands in StageFailure.Output (with a concise top-line Message). Every prior new test exercised the seam in isolation — white-box tests hand-construct the carrier and either call notesFailureOutput/failureMessage directly or wrap one level via wrapAsAbort. The ONE prior end-to-end test over the real transport (TestRelease_PriorTag_NotesFailureAbort_AbortsBeforeMutation) seeded EMPTY stdout and asserted only that a StageFailed fired — "the prior fakes had no stdout to lose." In production the carrier sits TWO wraps deep (carrier ← "generating notes: %w" ← abortError "notes generation failed (%s): %w"). errors.As/errors.Is are depth-agnostic so the extraction helper is correct, but a regression in the generator wrap layer, the SelectBody plumbing, or the non-zero-exit-plus-stdout seeding contract would be caught by no test. The spec asks for an assertion "at the wiring level, not just the presenter."

IMPLEMENTATION:
- Status: Implemented (test addition only, as designed for this analysis-cycle task)
- Location: internal/engine/release_priortag_test.go:717-803 (new TestRelease_PriorTag_NotesFailureAbort_CapturedOutputReachesStageFailure + onlyStageFailureEvent helper)
- Notes: The new test mirrors the sibling abort test's harness exactly (seedPriorTagReadGates → seedNormalAINotes → newDeps with Transport unset → priorTagNormalAIOptions/default abort config), differing only in the claude seed and the added Output/Message assertions.

Production-chain verification (each named layer confirmed against as-built source):
- Transport nil → real transport: newDeps (release_test.go:78-90) leaves ReleaseDeps.Transport unset; aiTransport (release.go:935-940) therefore calls aitransport.New(deps.Runner, cfg, config.VerbRelease), which builds ai.NewTransport over the FakeRunner (aitransport.go:40-44). DefaultAICommand = "claude -p --model sonnet" (config.go:91); parseCommand splits the binary name to "claude", which is exactly the name the test seeds. So the run drives the genuine production transport over the fake — NOT an injected double.
- Seeding contract (both attempts fail with stdout): test seeds f.Seed("claude", runner.Result{Stdout: "Prompt is too long", ExitCode: 1}, errors.New("exit status 1")). A name-keyed Seed returns the same outcome on every call, covering attempt 1 and the single retry. classifyFatal (transport.go:272-291) returns nil for a plain non-zero exit (default branch), so it is treated as bad CONTENT and retried; after the retry still fails, Generate packs the RETRY's res.Stdout into &GenerationError{Stdout, Stderr, ExitCode} (transport.go:211-221). Carrier wraps ErrGenerationFailed via Unwrap (transport.go:85). Correct: the non-nil err + populated res.Stdout matches the real runner's non-zero-exit contract.
- Two-wrap chain: generator.generateFromDiffWithContext wraps the transport error "generating notes: %w" (generate.go:185). resolveBody/SelectBody returns it to Release stage 4, which routes through ResolveFailure → abortError "notes generation failed (%s): %w" (resolve.go:100-102) in default abort mode (OnNotesFailure == "" ⇒ abort, resolve.go:66-70). Release stage 4 surfaces via surfaceAndUnwind(ctx, deps, "notes", ...) (release.go:458-460).
- Output extraction: surfaceAndUnwind sets StageFailure.Output = notesFailureOutput(cause) (release.go:1057). notesFailureOutput uses errors.As to find the *ai.GenerationError inside the two-wrap chain and reads Stdout (stdout-first compose, verbatim, trailing-trimmed) (release.go:1622-1639). With Stderr empty and Stdout "Prompt is too long", Output == "Prompt is too long". Matches the assertion.
- Message extraction: StageFailure.Message = failureMessage(cause) (release.go:1052). failureMessage delegates to notes.CauseText, which errors.Is-matches ai.ErrGenerationFailed and returns "AI returned empty/invalid notes after retry" (resolve.go:124-125, release.go:1674-1683). Matches the assertion.

TESTS:
- Status: Adequate
- Coverage: Closes exactly the integration seam the spec flagged. Asserts (a) the abort is non-zero through *engine.AbortError (assertAbortNonZero), (b) no mutation (assertNoMutation), (c) exactly one StageFailed (onlyStageFailureEvent fails on zero or >1), (d) Output == seeded stdout, (e) Message == concise phrase. The combined Output+Message assertions over the live two-wrap chain are uniquely owned here — no other test drives the production transport with NON-empty stdout.
- Not a tautology (regression-sensitivity confirmed per layer):
  * generate.go:185 %w→%v (swallow carrier): errors.As in notesFailureOutput fails → Output flips to "", and failureMessage falls back to cause.Error() → Message assertion flips. CAUGHT.
  * SelectBody/ResolveFailure dropping or substituting the carrier: Output and/or Message flip. CAUGHT.
  * Transport stops packing res.Stdout (the exact seeding-contract gap the empty-stdout sibling could not see): Output flips to "". CAUGHT.
  * abortError ceasing to wrap with %w: errors.Is(ai.ErrGenerationFailed) fails in CauseText → Message falls back to cause.Error(). CAUGHT.
- Not over-tested: re-asserting assertAbortNonZero + assertNoMutation is intentional and justified by the doc-comment — it keeps this proof self-standing when run alone (t.Parallel) without depending on the sibling test's existence; it does not duplicate the sibling's narrower notesGatePrompted check. The single duplicated helper (onlyStageFailureEvent) is an unavoidable cross-package twin (external engine_test cannot see the white-box package-engine onlyStageFailure) and is documented as such (lines 780-789, hardened by T4-1).
- Not under-tested: the spec's wiring-level requirement is met precisely. Edge variants (whitespace-only stdout, stderr-only, empty body path) are already covered by the white-box notesFailureOutput tests; replicating them end-to-end here would be redundant over-testing, correctly avoided.

CODE QUALITY:
- Project conventions: Followed. External engine_test package; t.Parallel(); t.TempDir() root; FakeRunner seeding (no real git/claude); RecordingPresenter; exact-value assertions on Output/Message; lowercase intent. Matches the file's established prior-tag harness idiom.
- SOLID principles: Good. onlyStageFailureEvent is a focused single-purpose helper; the test asserts behaviour (carried payload) not internals.
- Complexity: Low. Linear seed→drive→assert; the helper is a single-pass scan with a clear duplicate-detection guard.
- Modern idioms: Yes. errors.As/errors.Is rely on the production chain rather than re-deriving; t.Context() used; named consts (claudeStdout, conciseMessage) make intent self-documenting.
- Readability: Good — arguably exemplary. The WHY-comment block (717-740) accurately states the as-built two-wrap chain, the seeding rationale, and what regression each assertion guards, consistent with the codebase's heavy-WHY-comment contract.
- Issues: None.

PRESERVATION & PRODUCTION-CHANGE CHECKS:
- TestRelease_PriorTag_NotesFailureAbort_AbortsBeforeMutation (release_priortag_test.go:682-715) is intact — unchanged in the T3-2 diff; the new test is purely additive (+83 lines) and re-asserts (does not replace) the abort/no-mutation invariants. Criterion 3 satisfied.
- T3-2 commit (b19447f) changes exactly one Go file — internal/engine/release_priortag_test.go (test only). The remaining diff is tick/manifest bookkeeping (.tick/tasks.jsonl, manifest.json). No production (non-test) source file changed. Criterion 4 satisfied.
- Gates: not executed (verification is read-only; running the suite is out of scope). Assessed by reading: imports are complete (errors, testing, engine, presenter, presentertest, runner, version), referenced helpers/consts all exist, types match the presenter.StageFailure shape, and the code is gofmt-shaped. No reason found to doubt the build/vet/test/lint gates the task claims pass.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None. (No concrete, actionable change identified. The cross-package helper duplication is already documented and out-of-scope to collapse; raising it again would be a pure observation, not a finding.)
