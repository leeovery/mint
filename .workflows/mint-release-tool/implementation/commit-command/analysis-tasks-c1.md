---
topic: commit-command
cycle: 1
total_proposed: 5
---
# Analysis Tasks: Commit-Command (Cycle 1)

## Task 1: Single-source the emptiness verdict from the exclusion-filtered diff so the AI is never invoked on an empty post-exclusion diff
status: approved
severity: high
sources: architecture

**Problem**: The empty-staging preflight (`wouldStageNothing`, internal/commit/run.go:697-713) and the AI's L1 source diff (`sourceDiff`/`stagedDiff`/`trackedWorktreeDiff`/`untrackedFiles`, internal/commit/generate.go:144-174) are two independently-authored queries answering the same question ("is there anything to commit?") but applying DIFFERENT filters. The preflight runs name-only git commands WITHOUT the `diff_exclude` `:(exclude)` pathspecs (e.g. `git diff --cached --name-only`), while the L1 content is built WITH `excludePathspecs(cfg.DiffExclude)`. When every staged/changed file matches a `diff_exclude` glob, preflight sees a non-empty set and passes, then `sourceDiff` returns an empty post-exclusion diff. `CheckDiffSize("", n)` returns nil (`countDiffLines("") == 0`), the transport validates only the AI's response not its input, and there is no empty-post-exclusion guard anywhere in the generate path — so `transport.Generate` is called with a blank diff ("instructions\n\n"). This violates the spec invariant "never invoke the AI on an empty diff" (Staging Model) and wastes an AI call on noise the user explicitly excluded. This gap was previously noted as deferred/out-of-scope during implementation.

**Solution**: Make the emptiness decision derive from the SAME exclusion-filtered source the AI consumes, so the two computations cannot drift. Either apply `excludePathspecs(cfg.DiffExclude)` to the `wouldStageNothing` probes so preflight measures the post-exclusion would-be-staged set, or move the emptiness verdict into the generator/L1 itself: compute the post-`diff_exclude` would-be-staged diff once and treat empty-after-exclusion as the same fail-loud "nothing to commit" case keyed on the actual tree state. The post-`diff_exclude` would-be-staged content is the single source of truth; both "fail loud on empty" and "generate from this" must read it.

**Outcome**: An all-excluded staged/changed set is recognised as "nothing to commit" and fails loud with the exact spec empty-staging message, mutating nothing; `transport.Generate` is never reached with an empty/blank diff. Preflight and the AI's L1 diff can no longer disagree because they read one exclusion-filtered source.

**Do**:
1. Read internal/commit/run.go:697-713 (`wouldStageNothing` and the preflight callers), internal/commit/generate.go:108-174 (`GenerateWithContext`, `sourceDiff`/`stagedDiff`/`trackedWorktreeDiff`/`untrackedFiles`), internal/commit/generate.go around `excludePathspecs`, and internal/notes/size.go:34-40 (`CheckDiffSize`/`countDiffLines`) to confirm exactly where exclusion is and is not applied.
2. Locate the spec's "never invoke the AI on an empty diff" / empty-staging invariant and the exact empty-staging fail-loud message string in the specification (Staging Model section) so the new guard surfaces the identical message the existing empty-staging path uses.
3. Choose the single-source approach: prefer applying `excludePathspecs(cfg.DiffExclude)` to the `wouldStageNothing` name-only probes so the same pathspecs gate both decisions; if the L1 diff is already computed before the AI call, alternatively add a post-exclusion-empty check in the generate path that routes to the same fail-loud "nothing to commit" outcome. Do not introduce a third query — reuse the exclusion-filtered computation.
4. Ensure the empty-after-exclusion case fails loud BEFORE any AI invocation and mutates nothing (consistent with the existing empty-staging contract: no add, no commit, no push).
5. Keep behaviour for the non-excluded empty case and the non-empty case unchanged.

**Acceptance Criteria**:
- When every staged/changed file matches a `diff_exclude` glob, the command fails loud with the exact spec empty-staging message and performs no git mutation (no add/commit/push).
- `transport.Generate` is never called when the post-`diff_exclude` would-be-staged diff is empty.
- The emptiness verdict and the AI's L1 source diff are computed from the same exclusion-filtered source (no remaining name-only probe that omits `excludePathspecs` while the AI path applies it).
- Existing behaviour for genuinely non-empty staged sets and for the already-handled empty-staging case is unchanged.

**Tests**:
- New test: a repo whose only staged changes are files matching a `diff_exclude` pattern fails loud with the empty-staging message and records zero git mutations; assert the fake transport's `Generate` was not invoked.
- New test: the same all-excluded scenario on the worktree/deferred-staging path likewise fails loud and does not call the AI.
- Regression: a repo with at least one non-excluded staged change still reaches `Generate` and proceeds normally; a normally-empty staging set still fails loud as before.

## Task 2: Rename/redocument the Committer seam to reflect it is the lock-resilient sink for ALL commit mutations (stage, commit, push)
status: approved
severity: medium
sources: architecture

**Problem**: The `Committer` interface doc (internal/commit/run.go:142-156) says "the bare commit's `git commit -F -` flows through it" and the `Deps.Committer` field doc (internal/commit/run.go:171-174) says "The commit — the bare path's ONLY mutation — flows through it." As shipped, three distinct git mutations route through `Committer.Mutate`: `git add -u`/`git add -A` (`stageForMode`, run.go:836-852), `git commit -F -` (`createCommit`), and `git push` (`pushAfterCommit`, run.go:907-924). The seam's contract is correct (every mutation needs the `git_safe` lock wrapper) but the name and docs under-describe it, framing it as commit-only when it is the lock-resilient MUTATION seam for the whole verb. This narrow description reads correctly in its original Phase-1 bare-commit context but is misleading now that staging/push are layered on — it invites a future reader to assume staging/push bypass the wrapper or to add a parallel push/stage path. The release engine's equivalent is named `Mutator` for exactly this reason, and the doc already says it mirrors `engine.ReleaseDeps.Mutator`.

**Solution**: Rename `Committer` → `Mutator` (mirroring `engine.ReleaseDeps.Mutator`), or at minimum rewrite the interface and field docs to state it is the lock-resilient sink for ALL of commit's git mutations (stage, commit, push), not solely the commit. The interface method is already the generic `Mutate(ctx, stdin, name, args...)`, so only the type name and the two doc comments need to match the as-built scope.

**Outcome**: The seam's name and documentation accurately describe its as-built responsibility — the single lock-resilient sink through which staging, commit, and push all flow — eliminating the commit-only mischaracterisation and aligning naming with the engine's `Mutator`.

**Do**:
1. Read internal/commit/run.go:142-156 (interface + doc), :171-174 (Deps field + doc), :836-852 (`stageForMode`), `createCommit`, and :907-924 (`pushAfterCommit`) to confirm all three mutations route through this seam.
2. Decide rename vs doc-only: prefer rename `Committer` → `Mutator` to match `engine.ReleaseDeps.Mutator`. If renamed, update the interface type name, the `Deps.Committer` field name and its references, all call sites, the cmd-layer wiring in cmd/mint/main.go (the `git.NewMutator(r)` assignment to the Deps field), and every test that constructs `commit.Deps` (the per-file Deps builders set this field).
3. Rewrite both doc comments to state the seam is the lock-resilient sink for all of commit's git mutations — staging (`git add`), commit (`git commit -F -`), and push (`git push`) — each requiring the `git_safe` lock wrapper.
4. If choosing doc-only (no rename), leave the type name but make the two comments explicitly enumerate stage/commit/push.
5. Run go vet, golangci-lint, and the full commit test suite to confirm no broken references.

**Acceptance Criteria**:
- The interface doc and the `Deps` field doc both state the seam handles staging, commit, and push (not commit only).
- If renamed, the type is `Mutator` and all references (interface, Deps field, cmd-layer wiring, tests) compile and pass.
- No behavioural change: the same three mutations still route through the seam with the lock wrapper.
- go vet, golangci-lint, and `go test ./internal/commit/...` pass clean.

**Tests**:
- Existing commit test suite passes unchanged (rename is mechanical / docs are non-behavioural).
- No new behavioural test required; correctness is preserved by the existing end-to-end coverage that already exercises stage, commit, and push through this seam.

## Task 3: Do not emit the "opening editor" note on the unattended oversized path that fails loud
status: approved
severity: low
sources: standards

**Problem**: On the oversized-diff path `Run` emits `p.Warn("diff too large to summarise — opening editor")` (internal/commit/run.go:286-289) and THEN calls `runEditorFallback`, whose no-message-source guard (run.go:392-394) fails loud with "no AI message and no interactive editor available" when `deps.Yes` is true or stdin is non-interactive. The user therefore sees a note promising "opening editor" immediately followed by a fail-loud saying no editor is available — no editor is ever opened. The spec describes this note as the clear signal when routing TO the editor and says the "-y/non-TTY forbidden-combo check then applies"; it does not anticipate the note firing on a run that will never reach the editor. The AI-failure trigger (run.go:296-298) correctly carries NO note, so only the oversized trigger has this wrinkle. The fail-loud tests (`assertFailLoudNoMutation` in run_failloud_test.go) assert the StageFailed message and no-mutation but do not assert the absence of the contradictory oversized Warn, so the inconsistent sequence is currently unverified. Behaviour is spec-conformant on substance (it fails loud, mutates nothing, surfaces the exact message); only the emitted-then-contradicted note is a UX drift.

**Solution**: Only emit the oversized note when the fallback will actually open an editor. Run the no-message-source guard (`deps.Yes || !deps.StdinInteractive`) for the oversized branch BEFORE emitting the oversized note — either gate the `p.Warn` on the attended condition, or move the unattended guard ahead of the Warn so an unattended oversized run fails loud with the single spec fail-loud message and no preceding "opening editor" claim.

**Outcome**: An unattended oversized run (`-y` or non-TTY) fails loud with the single spec message and emits no contradictory "opening editor" note; the attended oversized path still emits the note before genuinely opening the editor.

**Do**:
1. Read internal/commit/run.go:286-298 (oversized note + AI-failure trigger) and run.go:392-394 (`runEditorFallback` no-message-source guard) to confirm the exact note string and guard condition.
2. Restructure so the oversized note is emitted only on the attended path: either wrap the `p.Warn("diff too large to summarise — opening editor")` in the `!(deps.Yes || !deps.StdinInteractive)` condition, or hoist the unattended fail-loud guard so it returns before the note for oversized runs.
3. Keep the attended oversized behaviour identical (note then editor opens) and keep the exact fail-loud message unchanged.
4. Confirm the AI-failure trigger continues to carry no note (no regression there).

**Acceptance Criteria**:
- On the `-y` / non-TTY oversized path, no "diff too large to summarise — opening editor" Warn is recorded; the run fails loud with the exact spec fail-loud message and mutates nothing.
- On the attended (TTY, non-`-y`) oversized path, the note is still emitted and the editor still opens.
- The AI-failure trigger still emits no note.

**Tests**:
- New fail-loud test: assert that on the `-y` / non-TTY oversized path no oversized Warn is recorded by the presenter, in addition to the existing StageFailed-message and no-mutation assertions.
- Regression: an attended oversized run still records the "opening editor" note and routes to the editor.

## Task 4: Consolidate the duplicated commit test-suite scaffolding (invocation-filter helpers and per-file Deps builders)
status: approved
severity: medium
sources: duplication

**Problem**: Isolated per-file authoring produced two clusters of copy-pasted test scaffolding that constitute a copy-paste-drift surface. (a) Nine invocation-filter helpers implement the same two filtering shapes over a recorded invocation slice: "keep only Name==git" appears byte-for-byte 3× (`gitInvocations` run_test.go:73, `gitInvocationsGen` generate_test.go:96, `editorGitInvocations` run_noai_test.go:37) and "keep only git calls whose Args[0]==<verb>" appears 6× (`addInvocations` staging_defer_test.go:16, `editorAddInvocations` run_noai_test.go:47, `commitInvocations` run_test.go:743, `editorCommitInvocations` run_noai_test.go:57, `pushInvocations` run_push_test.go:17, `editorPushInvocations` run_editor_push_test.go:17), each differing only by the verb literal and the source (raw runner vs `editorRunner.fake`). (b) Seven per-file `commit.Deps` builders have converged on the same shape: `aiFailDeps` (run_aifail_test.go:23), `oversizedDeps` (run_oversized_test.go:102), and `regenFailDeps` (run_regen_fallback_test.go:56) are IDENTICAL; `editDeps` (run_edit_test.go:19) and `noAIDeps` (run_noai_test.go:19) are subset variants; `failLoudDeps` (run_failloud_test.go:88) is the superset exposing every varying field; `newCommitDeps` (run_test.go:61) is a genuinely different bare-path shape. The shared `git.NewMutator(er, git.WithBackoff(func(int){}))` wiring and `StdinInteractive:true` default are copy-pasted six times, so a change to the editorRunner Committer wiring or the invocation-matching logic must be made in many places.

**Solution**: Add two shared invocation-filter helpers in the commit test package: `gitInvocationsOf(invs []runner.Invocation) []runner.Invocation` (subsumes the three Name==git copies) and `gitVerbInvocations(invs []runner.Invocation, verb string) []runner.Invocation` (subsumes the six verb filters), both taking an already-extracted `[]runner.Invocation` so one helper serves both raw-runner and `editorRunner.fake` call sites; rewrite the nine wrappers as one-line delegations or delete them where the call site can call the shared helper directly. For the Deps builders, keep one editor-path builder by promoting `failLoudDeps` (the superset) to a shared `editorDeps(...)` (options-struct variant preferred to avoid a boolean-triple parameter list), and have `aiFailDeps`/`oversizedDeps`/`regenFailDeps`/`editDeps`/`noAIDeps` become thin one-line wrappers (or be deleted in favour of direct calls) that fix only their per-scenario fields. Keep `newCommitDeps` (bare-path, FakeRunner + no editor) separate.

**Outcome**: The two filtering shapes live in one helper each and the editor-path Deps wiring lives in one builder; a future change to invocation matching or to the editorRunner Committer wiring touches one place instead of nine/six. Test behaviour and assertions are unchanged.

**Do**:
1. Read the nine helper definitions and the seven Deps builders at the cited file:line locations to confirm the exact signatures, the `editorRunner.fake.Invocations()` vs `r.Invocations()` access patterns, and the per-scenario field differences.
2. Add `gitInvocationsOf` and `gitVerbInvocations` to run_test.go (or a small `testutil`/shared test file in package commit), each accepting an already-extracted `[]runner.Invocation`.
3. Rewrite each of the nine wrappers to a one-line delegation (extracting `.Invocations()` at the call boundary), or delete the wrapper and update its call sites to call the shared helper directly — whichever keeps each test readable.
4. Promote `failLoudDeps` to a shared `editorDeps` constructor covering all varying fields (mode, root, yes, stdinInteractive, noAI, transport); prefer an options struct over a long positional/boolean parameter list. Centralise the `git.NewMutator(er, git.WithBackoff(func(int){}))` wiring and the `StdinInteractive:true` default in it.
5. Reduce `aiFailDeps`/`oversizedDeps`/`regenFailDeps`/`editDeps`/`noAIDeps` to thin wrappers over `editorDeps` (or inline them), preserving each scenario's exact field values (e.g. `noAI=true` for the --no-ai path, dropped Transport where applicable). Leave `newCommitDeps` untouched.
6. Run `go test ./internal/commit/...`, go vet, and golangci-lint; confirm all tests still pass with unchanged assertions.

**Acceptance Criteria**:
- Two shared invocation-filter helpers exist; no test file contains a re-rolled "Name==git" or "Args[0]==<verb>" filter loop (the nine wrappers are delegations or removed).
- A single editor-path Deps builder centralises the `git.NewMutator(...WithBackoff...)` wiring and `StdinInteractive` default; the previously-identical/subset builders are thin wrappers or inlined, with each scenario's distinct fields preserved.
- `newCommitDeps` (bare path) remains a separate builder.
- `go test ./internal/commit/...`, go vet, and golangci-lint pass clean; no test assertion changed meaning.

**Tests**:
- The existing commit test suite is the test: it must pass unchanged after consolidation (this is a refactor of test scaffolding, not of behaviour).
- Spot-check that at least one raw-runner call site and one `editorRunner.fake` call site both route through `gitInvocationsOf`/`gitVerbInvocations` to confirm the single helper serves both sources.

## Task 5: Extract a single commitAccept helper to single-source the stage→commit→push accept tail
status: approved
severity: low
sources: duplication

**Problem**: The two accept entry points end with the same ordered sequence: `stageForMode` → (surface on err) → `createCommit` → (surface on err) → `pushAfterCommit` → `p.RunFinished(...)` → return pushErr. The gate-accept tail (internal/commit/run.go:333-349, in `Run`) and the save-as-accept tail (internal/commit/run.go:422-438, in `runEditorFallback`) are the same five-step tail with the same error-surfacing and the same "RunFinished always fires, return pushErr" contract; only the committed body differs (`finalBody` vs `saved`). The individual steps are already well-extracted, so the remaining repetition is the orchestration glue — but it encodes the spec's load-bearing "stage→commit→push, mutate-nothing-until-accept, never-unwind" invariant, and having it written twice means a future change to that ordering (or to RunFinished firing) must be kept in sync across both accept paths.

**Solution**: Extract a single `commitAccept(ctx, deps, root, body string) error` helper that runs `stageForMode` → `createCommit` → `pushAfterCommit` → `RunFinished` (with the same error-surfacing) and returns pushErr, and call it from both the gate-accept branch and the save-as-accept branch. This collapses the duplicated tail to one call site each and single-sources the stage→commit→push ordering invariant.

**Outcome**: The stage→commit→push accept ordering, the error-surfacing on stage/commit failure, the always-fire `RunFinished`, and the return-pushErr contract live in exactly one function; both accept paths call it with their respective body. No behavioural change.

**Do**:
1. Read internal/commit/run.go:333-349 (gate-accept tail in `Run`) and run.go:422-438 (save-as-accept tail in `runEditorFallback`) to confirm the two tails are identical apart from the body argument.
2. Define `commitAccept(ctx, deps, root, body string) error` that performs: `stageForMode` (surface and return on err), `createCommit` with `body` (surface and return on err), `pushAfterCommit`, then `p.RunFinished(...)` (always), returning the push error.
3. Replace both inlined tails with a call to `commitAccept`, passing `finalBody` from the gate-accept branch and `saved` from the save-as-accept branch.
4. Preserve the exact error-surfacing presenter calls and the RunFinished arguments used today.
5. Run `go test ./internal/commit/...`, go vet, and golangci-lint.

**Acceptance Criteria**:
- A single `commitAccept` helper contains the stage→commit→push→RunFinished→return-pushErr sequence; neither `Run` nor `runEditorFallback` inlines that tail any longer.
- Both accept paths produce identical observable behaviour to before (same mutations, same ordering, same error surfacing, RunFinished always fires, pushErr returned) — verified by the existing accept-path and push tests.
- `go test ./internal/commit/...`, go vet, and golangci-lint pass clean.

**Tests**:
- Existing accept-path tests (gate-accept and save-as-accept) and push tests pass unchanged — they already assert the stage→commit→push ordering, RunFinished firing, and the warn-don't-unwind push-failure contract for both paths, which now both flow through `commitAccept`.
