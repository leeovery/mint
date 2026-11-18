TASK: Route AI-generation failure to the editor fallback (commit-command 3-3)

ACCEPTANCE CRITERIA:
- Any transport failure sentinel — ai.ErrGenerationFailed (after the transport's one bad-content retry), ai.ErrTimeout, ai.ErrCommandMissing — routes to the editor fallback, not abort
- ai.ErrTimeout and ai.ErrCommandMissing fall back immediately (never retried)
- save-as-accept semantics reused unchanged from 3-2 (non-empty save stages-then-commits; empty/abort true no-op)
- editor opens with an empty/template buffer — no synthetic stub, no re-show of a prior/partial message
- transport-failure route distinguished from the oversized-skip route (only notes.ErrDiffTooLarge takes the oversized path; no oversized note on this path)
- the transport's one bad-content retry is consumed, not re-implemented
- the -y/non-TTY and not-launchable fail-loud (3-5) and the r gate action (Phase 4) are NOT implemented here

STATUS: Complete

SPEC CONTEXT:
Spec "Commit Message Format & Prompt -> The $EDITOR fallback" (case 2) and "$EDITOR Fallback — Path Semantics" mandate that an AI generation failure after the engine's one retry falls back to $EDITOR rather than aborting (release's harsh notes-failure model is wrong for a routine local commit). The fallback opens an empty/template buffer (no synthetic stub), and the editor save IS the accept event (non-empty save -> stage-then-commit; empty/quit -> true no-op). This must be kept distinct from the oversized-skip (only notes.ErrDiffTooLarge), which short-circuits before L2 and carries an "opening editor" note. The transport's typed sentinels (internal/ai/transport.go:26-41) are: ErrGenerationFailed (bad content surviving the single retry), ErrTimeout and ErrCommandMissing (both reported immediately, never retried). Regeneration failure routes to the same entry point but r itself is Phase 4.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/commit/run.go:307-334 — generate-error routing: notes.ErrDiffTooLarge -> oversized branch (with note); isAITransportFailure(err) -> runEditorFallback (no note); any other error -> surface abort. context.Canceled is NOT a transport sentinel (transport.go:164-172) so it falls through to the surface abort, correctly not routing to the editor.
  - internal/commit/run.go:686-698 — isAITransportFailure predicate matches all three sentinels via errors.Is; named so Phase 4's r reuses the same entry point.
  - internal/commit/run.go:402-446 — runEditorFallback (shared with --no-ai/oversized): the fail-loud guard (3-5) runs first, then OpenEditor against the caller-supplied buffer (passed "" here -> empty), then save-as-accept via the shared commitAccept tail.
  - internal/commit/run.go:331 — the AI-failure branch calls runEditorFallback(ctx, deps, root, "") — empty initial buffer, no stub.
  - internal/commit/generate.go:127-130 — transport error wrapped with %w, preserving the sentinel so isAITransportFailure (errors.Is) matches end-to-end.
- Notes: The routing is purely additive over the shipped 3-1/3-2/2-3 pieces — editor resolution, save-as-accept, and stage-then-commit ordering are all consumed unchanged via runEditorFallback -> commitAccept. The transport-failure branch carries NO oversized note, the oversized branch carries one; the discriminator is correct and matches the spec. The transport itself is called once (failTransport / real ai.Transport) — commit never re-runs it. No drift from the plan.

TESTS:
- Status: Adequate
- Location: internal/commit/run_aifail_test.go (all eight micro-acceptance tests, one-to-one with the task's Tests list); supporting contract coverage in internal/ai/transport_test.go.
- Coverage:
  - ErrGenerationFailed routes to editor (not abort), commits saved body — TestRun_AIFailure_GenerationFailed_RoutesToEditor
  - ErrTimeout falls back immediately — TestRun_AIFailure_Timeout_RoutesToEditor
  - ErrCommandMissing falls back immediately — TestRun_AIFailure_CommandMissing_RoutesToEditor
  - save-as-accept reused: non-empty save under -a does `git add -u` THEN commit, ordered (assertAddBeforeCommit) — TestRun_AIFailure_NonEmptySaveUnderAll_AddsTrackedThenCommits
  - empty/aborted editor is a true no-op (table: whitespace-only save + aborted launch), no add/commit, non-zero error — TestRun_AIFailure_EmptySave_TrueNoOp
  - editor opens empty/template (onLaunch captures the temp-file at launch == "") — TestRun_AIFailure_EditorBufferIsEmptyTemplate
  - oversized route distinct: notes.ErrDiffTooLarge takes the oversized branch, discriminated by the oversized Warn note — TestRun_AIFailure_OversizedDiff_RoutesViaOversizedBranch
  - transport's retry consumed, not re-run: transport.Generate called exactly once (tr.calls == 1) — TestRun_AIFailure_TransportNotReRun
  - Bonus (beyond the listed Tests, justified): context.Canceled does NOT route to the editor and keeps the surface abort — TestRun_AIFailure_Cancelled_DoesNotRouteToEditor. This guards a real spec boundary (a Ctrl-C cancel is not an AI failure; transport.go propagates it unchanged) and is not redundant.
- Notes: Tests are genuinely end-to-end — editorDeps wires the real git.NewMutator (git_safe) over the editorRunner CommandRunner double and exercises the real runEditorFallback/commitAccept path; only the Transport is faked to inject the sentinel, and the editorRunner simulates a real $EDITOR file roundtrip. The commit/add assertions filter by git verb (gitVerbInvocations) and check Stdin == saved, so they are not vacuous. Not over-tested: the buffer-is-empty and transport-not-re-run checks are split because they assert distinct concerns (no-stub vs no-re-run). Timeout/CommandMissing share a seedAIFailFallback shape but each verifies its own sentinel routes immediately — not redundant given the spec calls out the never-retried distinction per sentinel.

CODE QUALITY:
- Project conventions: Followed. Error wrapping uses %w throughout (generate.go:129, run.go); typed sentinels matched with errors.Is per golang-error-handling; testify not required and not forced (table test uses plain t.Run/t.Fatalf, consistent with golang-testing). Seams injected via Deps, mirroring engine.ReleaseDeps.
- SOLID principles: Good. isAITransportFailure is a single named routing predicate (SRP); runEditorFallback is the one shared degradation path the three triggers converge on (DRY — no parallel implementation); the Transport interface keeps commit decoupled from the ai concretion (DIP).
- Complexity: Low. The generate-error routing is a flat three-way branch (oversized / transport-failure / other) with clear, documented ordering.
- Modern idioms: Yes. errors.Is sentinel matching, %w wrapping, deferred-staging via the shared accept tail.
- Readability: Good. The routing rationale (skip vs failure, note vs no-note, cancel-is-not-failure) is documented inline at run.go:307-334 and the predicate at run.go:686-698.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
