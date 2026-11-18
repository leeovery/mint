TASK: commit-command-1-6 — Minimal preflight: empty-index fail-loud (Phase 1)

ACCEPTANCE CRITERIA:
- Not a git repository -> fail loud cleanly (no panic, no AI call, no commit)
- An empty staged index -> fail loud with "nothing to commit, working tree clean"
- No AI/claude is invoked when the staged diff is empty (preflight short-circuits before generate)
- A non-empty staged index -> preflight passes and generation/commit proceed
- Preflight runs before message generation
- The dropped gates (clean-tree, on-branch, remote-sync, pre-push) are NOT implemented
- The flag-aware empty-staging variants are NOT implemented (deferred to Phase 2)
- All checks are read-only via the consumed CommandRunner/fake

STATUS: Complete

SPEC CONTEXT:
Spec "Preflight & Safety": commit's preflight is minimal — only (1) git repo present (anchored at repo root, same resolution as release) and (2) something to commit (empty -> fail loud). Spec explicitly DROPS clean-working-tree, on-release-branch, remote-in-sync, and any pre-push gate ("commit exists to operate on a dirty tree"). Spec "Staging Model -> Empty-staging handling": empty staging fails loud mirroring git's messaging; the AI is never invoked on an empty diff; genuinely-clean-tree yields "nothing to commit, working tree clean". Spec "Commit Flow": preflight is read-only and runs before generate. Task 1-6 is the bare (staged-only) slice; the flag-aware messaging is Phase 2.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/commit/run.go:271-274 — resolveRoot via consumed gitrepo.ResolveRoot, not-a-git-repo surfaces through `surface(p, "preflight", err)` (no panic).
  - internal/commit/run.go:286-296 — checkSomethingToCommit runs AFTER root/config but BEFORE generate (run.go:307), satisfying preflight-before-generate and the no-AI-on-empty short-circuit.
  - internal/commit/run.go:651-659 — resolveRoot delegates to gitrepo.ResolveRoot (`git rev-parse --show-toplevel`), the same consumed primitive release uses.
  - internal/commit/preflight.go:66-109 — checkSomethingToCommit/wouldStageNothing; bare StagedOnly probe is `git diff --cached --name-only -- .` (read-only), empty stdout => empty index.
  - internal/commit/preflight.go:37-41 — errNothingToCommit = "nothing to commit, working tree clean" (git's exact line, returned unwrapped so the verbatim text reaches the presenter).
  - internal/gitrepo/gitrepo.go:51-58 — ResolveRoot returns ErrNotARepository on non-zero exit (clean abort, never a panic).
  - internal/commit/surface.go:15-21 — renders cause.Error() as StageFailed.Message verbatim.
- Notes:
  - The not-a-git-repo and the something-to-commit checks are the ONLY preflight steps. No dropped gate exists in source: grep finds no ResolveReleaseBranch, no symbolic-ref/origin-HEAD, no clean-working-tree, no behind/diverged rev-list, no pre-push probe in internal/commit/*.go (only the documenting comment at run.go:823-826 confirming the drops). Matches the spec's inverse safety posture.
  - Phase-2 evolution noted, not drift: preflight.go now also carries the flag-aware empty-staging messaging (errNoChangesStaged, errNoTrackedChanges, emptyStagingError) and is staging-mode-aware. Task 1-6 forbade implementing those "here" as a Phase-1 boundary; they were legitimately added by the Phase-2 staging tasks (covered by staging_empty_test.go). The bare (StagedOnly) Phase-1 behaviour this task owns is fully present and correct, so the expansion is authorized later work, not a 1-6 violation.
  - Read-only anchoring: every probe runs via RunInDir(root) (preflight.go:192-197), and rootdir_test.go asserts read-side git calls anchor at the repo root — a correctness reinforcement of the spec's root-anchoring requirement.

TESTS:
- Status: Adequate
- Coverage: All five planned test names exist and map 1:1 to the acceptance criteria, in internal/commit/run_test.go:
  - TestRun_NotAGitRepository_FailsLoudNoAINoCommit (run_test.go:587) — seeds rev-parse failure; asserts non-nil error, zero transport calls, zero commits. No panic (run completes with error).
  - TestRun_EmptyStagedIndex_FailsLoudWithGitMessage (run_test.go:619) — empty probe stdout; asserts err.Error() == "nothing to commit, working tree clean" AND StageFailed.Message == same line (verbatim surfacing verified, not just the returned error).
  - TestRun_EmptyStagedIndex_NoAIInvoked (run_test.go:649) — asserts zero transport calls and zero commits on empty index.
  - TestRun_NonEmptyStagedIndex_ProceedsToGeneration (run_test.go:673) — non-empty index reaches generation (transport called once) and commits the body verbatim.
  - TestRun_PreflightRunsBeforeGenerate (run_test.go:699) — asserts the first git call is the `--name-only` preflight read and the second is generation's L1 diff (ordering proven by recorded invocation sequence, not by implementation internals).
- Notes:
  - Tests verify behaviour (error text, transport call count, commit absence, git-call ordering) rather than implementation details — would fail if the feature broke (e.g. if generate ran before preflight, or the AI were called on an empty diff).
  - Not over-tested: the five tests are distinct facets (repo-presence, message text, no-AI, happy path, ordering) with no redundant happy-path duplication. The empty-index pair (FailsLoud + NoAIInvoked) split the message assertion from the no-side-effect assertion — focused, not redundant.
  - Edge cases from the task (empty index message, no AI on empty diff, not-a-git-repo) are each covered.

CODE QUALITY:
- Project conventions: Followed. Error sentinels are lowercase, no trailing punctuation (golang-error-handling rule 3). The deliberate unwrapped return of the user-facing preflight sentinels is a documented exception (preflight.go:22-36) because the spec requires git's verbatim line at the presenter — correct, not a wrapping violation. Tests use the codebase's standard t.Errorf/t.Fatalf + FakeRunner/RecordingPresenter idiom.
- SOLID principles: Good. resolveRoot, checkSomethingToCommit, wouldStageNothing, emptyStagingError, gitOutputEmpty each have a single clear responsibility; the source selection is factored into one source.go so the preflight probe and the L1 diff cannot drift (a genuine DRY win, not premature abstraction).
- Complexity: Low for the 1-6 slice (the bare path is a single read + emptiness check). emptyStagingError's switch is Phase-2 surface but remains shallow and total.
- Modern idioms: Yes. errors.Is-friendly sentinels, context-threaded runner seam, %w wrapping at the git boundary (gitOutputEmpty).
- Readability: Good. Intent is well documented; the read-only / mutate-nothing-until-accept invariant is explicit at the call site (run.go:286-293).
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
