TASK: dry-run-autostash-mutation-3-1 — Reuse seedDryRunFirstRelease for the dry-run+autostash dirty-tree test instead of hand-copying the read-gate timeline (test-only refactor, tick-e62a83, severity low / duplication)

ACCEPTANCE CRITERIA:
1. seedDryRunFirstRelease carries a trailing skipCleanTree bool; seeds the `status --porcelain` line only when false; all other seeded lines and order unchanged.
2. Inlined SeedSequence block in TestRelease_DryRunAutostash_DirtyTree_NoStashMutation_CompletesPreview is gone, replaced by a single helper call with skipCleanTree=true.
3. Every other caller passes skipCleanTree=false and behaviour byte-for-byte unchanged.
4. The helper's WHY-comment is updated true-to-as-built and explains the conditional clean-tree slot.
5. All project gates pass.
6. Test-only refactor — no new test; existing test still asserts zero status --porcelain, zero stash push/pop, no mutation tail, no gh, final RunFinished with no StageFailed.

STATUS: Complete

SPEC CONTEXT:
The spec (.../specification.md) defines the quick-fix: `mint release --dry-run --autostash` against a dirty tree must skip the real stash AND bypass the clean-tree preflight gate (gate bypass conditioned on DryRun && AutoStash, never DryRun alone). This task 3-1 is a downstream analysis-cycle cleanup: the dry-run+autostash dirty-tree test had hand-copied the seedDryRunFirstRelease read-gate timeline minus the clean-tree probe, creating a drift hazard. The refactor folds that one-line difference into a skipCleanTree parameter on the shared helper. The spec's Verification section requires the engine test to assert zero stash push/pop and a completed preview; that test is unchanged in behaviour — only its seeding source changed.

IMPLEMENTATION:
- Status: Implemented (matches plan exactly)
- Location: internal/engine/release_dryrun_test.go:38-57 (helper), internal/engine/release_autostash_test.go:310 (the refactored call)
- AC1 — VERIFIED. release_dryrun_test.go:38 signature is `seedDryRunFirstRelease(f *runner.FakeRunner, root, releaseBranch, tag string, skipCleanTree bool)`. The four head calls (show-toplevel, symbolic-ref, tag --list, fetch --tags) are seeded first (lines 39-44); the `status --porcelain` clean line is appended only inside `if !skipCleanTree` (lines 45-47); the six tail calls (abbrev-ref HEAD, verify refs/tags, rev-list left-right, ls-remote, rev-parse HEAD, remote get-url) follow (lines 48-55). Order is identical to the pre-refactor inline sequence. Cross-checked against the real read-gate order in internal/preflight/preflight.go (CheckCleanTree at line 100, CheckBranch abbrev-ref at line 116, verify refs/tags at line 139, rev-list at line 184, ls-remote at line 238) and internal/gitrepo/gitrepo.go (show-toplevel line 52, symbolic-ref line 78): the clean-tree probe correctly sits between `fetch --tags` and `abbrev-ref HEAD`, the exact slot the DryRun && AutoStash combo bypasses.
- AC2 — VERIFIED. release_autostash_test.go:310 is now a single `seedDryRunFirstRelease(f, root, "main", "v0.0.1", true)` call. The git show of commit 79a840a confirms the ~11-line inlined `f.SeedSequence("git", ...)` block (which had ScriptedOut(root) … remote get-url, with the clean-tree probe absent) was deleted and replaced by the helper call.
- AC3 — VERIFIED. grep finds exactly five other callers, all in release_dryrun_test.go (lines 75, 114, 152, 265, 295), each passing skipCleanTree=false. With false, the helper produces 4 head + 1 status + 6 tail = 11 calls — identical count and identical seeded values to the original 11-call inline sequence. Byte-for-byte equivalent. (gofmt reformatted the string concat from `"origin/"+releaseBranch` to `"origin/" + releaseBranch` inside the slice literal; the produced value is unchanged.)
- AC6 — VERIFIED (test assertions intact). release_autostash_test.go:318-343 still asserts: no `git stash push --include-untracked` (318), no `git stash pop` (321), no `git status --porcelain` (325), assertNoMutation (329), no gh command (330-334), final event is RunFinished (336-338), and no StageFailed recorded (341-343). None of these assertions were touched by the refactor.
- Notes: The `tag` parameter remains formally unused in code (only referenced in trailing comments such as `refs/tags/{tag}`). This is PRE-EXISTING — it was already an unused parameter before this task — and Go does not flag unused function parameters, so it is not a build/lint concern. Not introduced by this change.

TESTS:
- Status: Adequate (test-only refactor; no new test required by design)
- Coverage: The refactor is itself exercised by all six call sites. The five skipCleanTree=false sites are covered by the existing dry-run suite (TestRelease_DryRun_NoMutation_FirstRelease, _RunsReadOnlyPreflightAndComputesVersion which positively asserts the `status --porcelain` probe DID run at line 124, _PrintsFullPlan, _SkipsHooksAndReports, _RepoFilesUnchanged). The one skipCleanTree=true site is the dry-run+autostash test, which positively asserts the `status --porcelain` probe did NOT run (release_autostash_test.go:325). These two opposing assertions together prove the conditional slot behaves correctly in both modes — a strong guard against the helper regressing.
- Not under-tested: edge of the parameter (true vs false) is exercised on both sides with contradictory positive assertions; a faulty conditional would fail one suite or the other.
- Not over-tested: no redundant scaffolding added; the change removed duplication rather than adding it.
- Notes: None. The test surface is unchanged in intent; only the seeding source was deduplicated.

CODE QUALITY:
- Project conventions: Followed. Per CLAUDE.md test idioms, the read-gate timeline retains exact-argv comments per seeded line and SeedSequence keyed by command name. The WHY-comment discipline is honoured (see below).
- SOLID / DRY: This task's whole purpose is DRY — it removes the byte-for-byte duplicated timeline. ONE definition of the read-gate order now exists; the prior drift hazard is eliminated. Good.
- Complexity: Low. A single `if !skipCleanTree` branch on a slice append; no new control flow of note.
- Modern idioms: Idiomatic Go — slice literal + conditional append + variadic spread into SeedSequence(seq...). Appropriate.
- Readability: Good. The helper's WHY-comment (release_dryrun_test.go:34-37) is updated true-to-as-built: it names skipCleanTree, states it omits the `status --porcelain` probe, identifies that this is the single gate the DryRun && AutoStash combo bypasses, and notes every other caller passes false to keep one definition of the read-gate order. The stale "ten read-only gates" phrasing was corrected to "the read-only gates". The call-site comment at release_autostash_test.go:306-309 was likewise updated to explain skipCleanTree=true.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None. (The pre-existing unused `tag` parameter proposes no action here — it predates this task, is not a Go error, and removing it would be out of scope for a duplication-only cleanup. Recorded as an observation only, not a finding.)

GATES NOT RUN: My operating rules restrict Bash to the output-file rename and forbid executing the test suite, so I did not run `go build` / `gofmt -l` / `go vet` / `go test -race` / `golangci-lint`. AC5 (gates pass) is therefore verified by inspection only: the change is a mechanical test-only refactor with no new control flow, the touched file is `package engine_test` (external test package per convention), and the produced FakeRunner sequences are byte-equivalent for the false path. No gofmt/vet/lint red flags are visible by reading (the slice-literal concat spacing is already gofmt-normalised in HEAD). The orchestrator should run the gate suite once to close AC5 definitively.
