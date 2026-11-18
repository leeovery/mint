TASK: Route r regeneration-failure to the $EDITOR fallback (tick-30658f, suffix 4-5)

ACCEPTANCE CRITERIA:
- An r regeneration failure after the engine's one retry routes to the 3-3 editor fallback
- It reuses the 3-3 entry point — no parallel failure handler is introduced
- There is no special re-show-prior-message path — the fallback editor opens empty/template (no stub, no pre-r message pre-filled)
- The save-as-accept semantics are unchanged (non-empty save => stage-then-commit per mode; empty/abort => true no-op)
- The engine's one retry is consumed (1-3/L2), not re-implemented
- The -y/non-TTY fail-loud (3-5) is moot — r is interactive-only; no separate unattended branch is added

STATUS: Complete

SPEC CONTEXT:
specification.md "$EDITOR Fallback — Path Semantics → Regeneration failure routes here too" and "Interactive Review Gate → Choice mapping (r)": any failed AI generation lands at the editor; pressing r and failing after the one retry is treated as any other AI failure → the $EDITOR fallback, with NO re-show of the pre-r message. Moot under -y/non-TTY because r is an interactive-only gate action.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/commit/run.go:578-616 — reviewLoop's ChoiceRegen branch. On a regeneration failure it calls isAITransportFailure(gerr) and, when true, returns the internal routing sentinel errRegenerateFallback (line 606). A non-transport regen error keeps the defensive surface-abort (line 611).
  - internal/commit/run.go:342-351 — Run intercepts errRegenerateFallback (errors.Is) and calls runEditorFallback(ctx, deps, root, "") — empty initial buffer, the SAME entry point first-generation AI failures use (line 331). Sentinel is intercepted before it can surface (no user-facing message).
  - internal/commit/run.go:131-140 — errRegenerateFallback documented as an internal-only routing sentinel; Run owns the fallback call so a successful fallback commit exits 0 (not turned into errGateAborted) and never double-stages.
  - internal/commit/run.go:694-698 — isAITransportFailure is the single shared routing predicate (ErrGenerationFailed/ErrTimeout/ErrCommandMissing); its doc explicitly notes it is named so Phase 4's r-failure reuses the SAME entry point.
  - internal/commit/run.go:402-446 — runEditorFallback is unchanged and shared; no parallel handler added.
  - internal/commit/generate.go:109-132 — regenerateMessage → GenerateWithContext wraps the transport error with %w ("generating commit message: %w", line 129), so the sentinel survives errors.Is and isAITransportFailure matches a regen failure exactly as it matches a first-gen failure. The transport's own one retry is consumed here (line 127), not re-run.
- Notes: Entry-point reuse is genuine — the r-failure and the first-generation-failure paths converge on the identical isAITransportFailure predicate and the identical runEditorFallback(..., "") call. No re-show path: the initial buffer is the empty string in both. The -y/non-TTY case is structurally moot: under -y the presenter auto-accepts and never returns ChoiceRegen, and on a non-TTY Prompt returns ErrNotInteractive (run.go:526-528), so the regen branch is unreachable — no separate unattended branch was added.

TESTS:
- Status: Adequate
- Coverage: internal/commit/run_regen_fallback_test.go — one test per acceptance criterion, each with a real editor roundtrip (editorRunner writes/reads a real temp file and records launches; sequencedTransport scripts a successful initial generate then a failing regeneration):
  - TestRun_RegenFailure_RoutesToEditorFallback — r failure routes to the editor; exactly one RunInteractive launch; saved body committed; no StageFailed.
  - TestRun_RegenFailure_ReusesThe33EntryPoint — `git var GIT_EDITOR` resolution (the 3-1 step of the shared fallback) observed before the single launch; correct editor launched.
  - TestRun_RegenFailure_EditorBufferIsEmptyTemplate — captures temp-file contents at launch via onLaunch; asserts the pre-r message ("...BEFORE the user pressed r") is NOT pre-filled (buffer == "").
  - TestRun_RegenFailure_NonEmptySaveUnderAll_AddsTrackedThenCommits — under -a, `git add -u` then `git commit -F -` carrying the saved body verbatim, in that order (assertAddBeforeCommit).
  - TestRun_RegenFailure_EmptySave_TrueNoOp — table {whitespace-only save, aborted editor}: editor launched, no add, no commit, non-zero abort.
  - TestRun_RegenFailure_EngineOneRetryConsumed — transport.Generate called exactly twice (initial + one failing regen), proving commit does not re-run the retry.
  - Failure-sentinels exercised across tests include ErrGenerationFailed and ErrTimeout (the routing predicate's members).
- Notes: Maps 1:1 onto the six micro-acceptance test names in the task. Not under-tested (empty-template, empty/abort no-op, retry-consumption, and stage-then-commit ordering all covered). Not over-tested — each test asserts a distinct property; the empty-save case is a tight 2-row table rather than duplicated tests. Tests verify behaviour (launch count, git argv order, committed body, transport call count), not internals.

CODE QUALITY:
- Project conventions: Followed. Sentinel errors are package vars with %w wrapping (golang-error-handling); errors.Is used for routing; the routing predicate is a small named function reused across both failure sites (DRY, no parallel handler); test doubles live in _test.go with a single shared editorDeps options builder.
- SOLID principles: Good. The fallback entry point (runEditorFallback) has one owner; reviewLoop signals via a sentinel and Run owns the side-effecting fallback call — the regeneration (decision) is cleanly separated from the fallback (action), and the doc on errRegenerateFallback explains exactly why Run must own the call (exit-0 + no double-stage).
- Complexity: Low. The r-failure addition is two errors.Is branches; no new control structure of note.
- Modern idioms: Yes (errors.Is/%w, table-driven subtests, t.Parallel).
- Readability: Good. Doc comments on errRegenerateFallback, reviewLoop's ChoiceRegen branch, and isAITransportFailure each state the spec rule (one consistent rule: any failed AI generation lands at the editor; no re-show).
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
