TASK: Push after an editor save-as-accept commit (commit-command 5-3 / tick-50726c)

ACCEPTANCE CRITERIA:
- A non-empty editor save (save-as-accept) commits then pushes when -p is armed
- mint commit -Ap --no-ai runs end-to-end: stage (git add -A) -> commit -> push, no AI call, editor opened by mint itself
- The editor-path push is the SAME single shared push step as the gate-path push (5-2) — no parallel/second push call
- The push runs only after the staging+commit ordering completes
- The push reuses 5-2's plain git push via git_safe (no upstream args, no special upstream logic)
- The AI-failure (3-3) and oversized (3-4) editor drops also commit-then-push on a non-empty save when -p is armed
- The empty/aborted-save no-push case is NOT handled here (5-5); the failure warn is NOT here (5-4)

STATUS: Complete

SPEC CONTEXT:
specification.md "$EDITOR Fallback — Path Semantics" -> "The editor save IS the accept event": a non-empty save is a full accept, so "-p push then runs as normal" (mint commit -Ap --no-ai stages, commits, and pushes); an empty/aborted save is a true no-op (no push even with -p). "Auto-push Behaviour" mandates a plain `git push` deferring all upstream handling to git, warn-don't-unwind on failure. This task wires the editor save-as-accept path into the SAME single shared push step 5-2 built — no parallel push — reached only after the stage->commit ordering finishes.

IMPLEMENTATION:
- Status: Implemented (consume-and-reuse, no new push machinery — exactly as the task framed it)
- Location:
  - internal/commit/run.go:402-446 runEditorFallback — non-empty save (line 445) routes through commitAccept, the SAME shared accept tail the gate path uses (run.go:367).
  - internal/commit/run.go:786-803 commitAccept — the single ordered stage(788)->commit(794)->push(797) tail; pushAfterCommit is called once here, so BOTH accept paths share one push call site (no parallel push).
  - internal/commit/run.go:844-861 pushAfterCommit — gated on deps.Push; plain `git push` (argv exactly ["push"], no upstream args) via the git_safe Mutator; warn-don't-unwind owned by 5-4.
  - cmd/mint/commit_flags.go:67-78 / cmd/mint/main.go:347 — -p/--push (and -Ap/-Apy bundling) threaded into Deps.Push end-to-end.
- Notes: The reuse is structural and verifiable: the editor path adds zero push code; it routes the saved buffer into commitAccept, which is the literal same function the gate path calls. The "no second push" guarantee is therefore guaranteed by construction, not just by test. AI-failure (run.go:330-332) and oversized (run.go:318-323) both also funnel into runEditorFallback, so they inherit the identical commitAccept->pushAfterCommit path. No drift from plan or spec.

TESTS:
- Status: Adequate
- Coverage (internal/commit/run_editor_push_test.go):
  - TestRun_NoAI_PushArmed_EditorSaveCommitsThenPushes — non-empty save commits then pushes; exactly 1 push; commitIdx < pushIdx (push strictly after commit). [AC 1, 4]
  - TestRun_NoAI_PushArmed_AddAll_EndToEnd — mint commit -Ap --no-ai: asserts NO `claude` call, then add -A < commit < push, each exactly once, all via the git_safe Mutator. [AC 2, 4]
  - TestRun_NoAI_PushArmed_SingleSharedPushStep — exactly one push recorded (no parallel/second push). [AC 3]
  - TestRun_NoAI_PushArmed_PlainPushNoUpstreamArgs — push argv exactly ["push"], empty stdin (plain push, no upstream args). [AC 5]
  - TestRun_AIFailure_PushArmed_EditorSaveCommitsThenPushes — AI-failure (3-3) editor drop with -p commits then pushes, push after commit. [AC 6, AI-failure half]
  - TestRun_NoAI_PushUnarmed_EditorSaveCommitsNoPush — disarmed -p: commit happens, zero pushes (the no-op-when-disarmed half of this task's push step).
  - Test wiring is sound: editorDeps (run_failloud_test.go:108-120) builds the REAL git.NewMutator over the editorRunner, and pushes are asserted on the same embedded FakeRunner — proving the push flows through the genuine git_safe seam, not a side channel. Ordering helpers (editorGitIndexOf) read the real recorded git order. Tests verify behaviour (recorded git invocations + order), not implementation internals.
- Notes (not under-tested in substance; one named-AC test absent):
  - The acceptance criteria name the OVERSIZED (3-4) editor drop alongside AI-failure ("...and oversized (3-4) editor drops also commit-then-push..."), but there is no -p-armed oversized test (only the AI-failure variant exists). The behaviour IS covered by construction — the oversized branch (run.go:318-322) routes through the identical runEditorFallback->commitAccept->pushAfterCommit path the AI-failure test exercises — so this is a completeness gap on a named AC, not a behavioural risk. Non-blocking.
  - Not over-tested: each test pins one distinct property (then-pushes / end-to-end-AddAll / single-step / plain-argv / AI-failure-variant / disarmed-no-op); no redundant happy-path clones, no excessive mocking.

CODE QUALITY:
- Project conventions: Followed. git_safe Mutator used for the push (golang-safety / git_safe posture); table-free focused tests with intent-naming (golang-testing/golang-naming); errPushFailed is a documented sentinel carrying no user text, exit-only (golang-error-handling).
- SOLID principles: Good. Single shared accept tail (commitAccept) is the textbook DRY/SRP move — the two accept entry points converge on one ordering+push owner, so the "no parallel push" and "stage->commit->push order" invariants live in exactly one place and cannot drift.
- Complexity: Low. runEditorFallback is a linear guard->open->classify->commitAccept; the push gate is a single `if !deps.Push { return nil }`.
- Modern idioms: Yes. errors.Is sentinel routing, raw-bytes stdin for retry-safe re-pipe, context threaded throughout.
- Readability: Good — arguably exemplary. The doc comments on runEditorFallback (run.go:370-401), commitAccept (768-785) and pushAfterCommit (805-843) state the reuse, the ordering guarantee, and the warn-don't-unwind contract precisely.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/commit/run_editor_push_test.go — add an oversized-diff (3-4) editor-drop test with -p armed asserting commit-then-push on a non-empty save, mirroring TestRun_AIFailure_PushArmed_EditorSaveCommitsThenPushes. The oversized drop is named explicitly in this task's acceptance criteria but only the AI-failure editor-entry variant has a -p test; the behaviour is covered structurally (same commitAccept tail) but the named-AC case is unasserted.
