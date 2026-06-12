TASK: Route --no-ai to the editor with save-as-accept (tick-d5adfe, commit-command 3-2)

ACCEPTANCE CRITERIA:
- --no-ai skips L3 generate — no ai_command/claude invocation recorded
- mint opens the editor itself (a direct editor launch is recorded), not via git commit
- The editor open is a reusable file-roundtrip routine (write initial buffer -> launch resolved editor argv via RunInteractive -> read back) that accepts a caller-supplied initial buffer; message NOT via stdin
- The editor buffer starts empty/template — no synthetic stub message
- A non-empty save applies the mode's -a/-A staging then commits, in that order (git_safe)
- Under the default mode, a non-empty save commits the existing index unchanged (no git add)
- An empty save is a true no-op — no staging, no commit, no mutation
- An aborted/quit editor is a true no-op
- 'Empty' is whitespace-only; no #-comment stripping
- No separate Continue? gate rendered on the --no-ai path
- No -p push implemented, but the save-accept path does not preclude it

STATUS: Complete

SPEC CONTEXT:
Per the commit-command spec ("$EDITOR Fallback — Path Semantics" + "Commit Message Format & Prompt"), --no-ai is one of three "no AI message" cases that drop to $EDITOR with an empty/template buffer, behaving like plain git commit but reconciled with mint's deferred-staging model. The editor SAVE is the accept event (no separate Continue? gate — the gate is AI-path-only): a non-empty save applies the deferred -a/-A staging then commits in that order; an empty/aborted editor is a true no-op (mutate-nothing-until-accept). mint opens the editor itself rather than delegating to git commit because staging must be deferred to the save event. The -y/non-TTY and not-launchable fail-loud are the sibling task 3-5; empty-staging is the 2-4 preflight reached before the editor.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/commit/editor_open.go:51-95 — OpenEditor, the reusable file-roundtrip routine: ResolveEditor (3-1) -> writeEditorTempFile(initial) -> strings.Fields argv + temp path appended -> r.RunInteractive -> read-back. Returns (saved, exitedNormally, err). Never stages/commits.
  - internal/commit/run.go:303-305 — the deps.NoAI branch: short-circuits BEFORE generateMessage and the review loop, routing straight to runEditorFallback(ctx, deps, root, "") with an empty initial buffer.
  - internal/commit/run.go:402-446 — runEditorFallback: no-message-source guard first (3-5), then OpenEditor; aborted (ok=false) and whitespace-only saves return errEditorNoOp (true no-op); a non-empty save calls commitAccept.
  - internal/commit/run.go:786-803 — commitAccept: stageForMode (git add) THEN createCommit (git commit -F -) THEN pushAfterCommit, in that order; the single shared accept tail used by both accept paths.
  - internal/commit/run.go:459-461 — isEmptyEditorBuffer = strings.TrimSpace == "" (the whitespace-only rule, shared with the `e` action; no #-comment stripping since no synthetic stub exists).
  - internal/commit/run.go:730-747 — stageForMode maps All->`git add -u`, AddAll->`git add -A`, StagedOnly->no add.
  - cmd/mint/commit_flags.go:62,78 + cmd/mint/main.go:338 — --no-ai parsed and threaded into Deps.NoAI end-to-end.
- Notes:
  - Empty-staging preflight (checkSomethingToCommit, run.go:294) runs BEFORE the NoAI branch, matching the spec's "2-4 preflight reached before the editor."
  - Message travels via the file only; the argv appends the temp path as the final arg and splits multi-word editor commands (e.g. "code --wait"). No stdin path for the buffer.
  - -p push is NOT implemented here but is NOT precluded: commitAccept already calls pushAfterCommit (gated on deps.Push), so 5-3 reuses the same accept tail unchanged. Correct per the task's "does not implement but does not preclude."
  - The save-as-accept routine cleanly mirrors engine.EditorLauncher.Edit (temp file + Fields argv + RunInteractive + Suspend/Resume bracket) without importing the engine's release-coupled policy — exactly as the task directed.

TESTS:
- Status: Adequate
- Coverage:
  - internal/commit/run_noai_test.go covers every acceptance criterion end-to-end through commit.Run:
    - SkipsAI_NoTransportCall (transport nil + no claude invocation; also asserts no Prompt gate)
    - OpensEditorViaMint_NotGitCommit (exactly 1 RunInteractive launch on the resolved editor; the only git commit is `-F -`, not a bare interactive commit)
    - EditorBufferIsEmptyTemplate (captures temp-file contents at launch == "")
    - NonEmptySaveUnderAll_AddsTrackedThenCommits (`git add -u` then commit, assertAddBeforeCommit ordering, body verbatim)
    - NonEmptySaveUnderAddAll_AddsEverythingThenCommits (`git add -A` then commit, ordering)
    - NonEmptySaveUnderDefault_CommitsIndexUnchanged (no git add; index committed)
    - WhitespaceOnlySave_TrueNoOp (table: "" and "  \n\t\n  " — both no add/no commit, non-zero abort)
    - AbortedEditor_TrueNoOp (launchErr non-not-found -> no add/no commit, non-zero abort)
    - NoContinueGate (no Prompt, no ShowMessage)
  - internal/commit/editor_open_test.go unit-tests the routine in isolation: write-initial/launch-resolved/read-back, multi-word arg splitting + path appended, caller-supplied initial pre-fill (the 4-1 capability), missing-editor surfaces ErrNoEditor, not-found binary surfaces ErrCommandNotFound, aborted editor reports ok=false without error, Suspend/Resume bracketing, and that the buffer never travels via stdin.
- Notes:
  - Edge cases from the task are all present: order-of-staging (assertAddBeforeCommit), whitespace-only-as-empty (table), aborted=no-op, default-mode-index-unchanged, pre-fill capability. The whitespace rule is tested at both empty-string and mixed-whitespace.
  - Not over-tested: the unit tests (editor_open_test.go) and the integration tests (run_noai_test.go) target different seams — routine mechanics vs Run wiring — with little redundant overlap. The launch-count==1 / commit==1 assertions are load-bearing (they prove no double-launch / no bare git commit delegation), not redundant.
  - Whitespace-only and aborted no-op tests assert the non-zero abort (err != nil) as well as no-mutation, so they would fail if the no-op silently committed or returned success — good failure sensitivity.

CODE QUALITY:
- Project conventions: Followed. errors.Is sentinel discrimination (ErrNoEditor, ErrCommandNotFound, errEditorNoOp), %w wrapping, table-driven parallel tests with testify-free t.Errorf/t.Fatalf consistent with the rest of the package, deferred temp-file cleanup, deferred ResumeSpinner. Naming idiomatic (OpenEditor, runEditorFallback, isEmptyEditorBuffer, commitAccept).
- SOLID principles: Good. OpenEditor has a single responsibility (open + roundtrip, never stage/commit) explicitly documented and enforced; commitAccept is the single shared mutation tail so the STAGE->COMMIT->PUSH order lives in exactly one place and cannot drift between the gate-accept and save-as-accept paths. The (saved, exitedNormally, err) return cleanly separates the three caller decisions (no-op / fail-loud / accept).
- Complexity: Low. The NoAI branch is a 1-line short-circuit; runEditorFallback is a flat guard-then-decide sequence; no nested control flow of note.
- Modern idioms: Yes. strings.Fields for argv splitting, os.CreateTemp, context-threaded runner seam, sentinel errors via errors.New + %w.
- Readability: Good. Doc comments are thorough (arguably verbose) and accurately describe the contract, including the deferred-staging reconciliation and the 3-3/3-4/4-1 reuse the routine anticipates.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/commit/editor_open.go:79-88 — The (ok=false, nil) "aborted/quit" path treats ANY non-not-found RunInteractive error as a benign quit/abort no-op. A genuinely broken editor invocation (e.g. a transient exec/IO failure that is not ErrCommandNotFound) is therefore silently swallowed as "user aborted" and produces a true no-op rather than surfacing. This matches the spec's save-as-accept intent (a launched-but-failed editor == abort) and is covered by TestOpenEditor_AbortedEditor_ReportsNotNormalExit, so it is a deliberate design choice — flagged only as a decision point should distinguishing "user quit" from "editor crashed" ever matter.
- [do-now] internal/commit/run.go:347 — Comment says a successful fallback commit "exits 0", but commitAccept can return errPushFailed (non-zero) on a -p push failure even after a successful commit; tighten the comment to "exits per the accept tail (0 on commit success, non-zero only on a failed -p push)" to avoid implying the fallback path is always 0.
