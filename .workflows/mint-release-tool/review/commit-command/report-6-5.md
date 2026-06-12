TASK: commit-command 6-5 — Extract a single commitAccept helper to single-source the stage→commit→push accept tail (tick-2a281f)

ACCEPTANCE CRITERIA:
- A single `commitAccept` helper contains the stage→commit→push→RunFinished→return-pushErr sequence; neither `Run` nor `runEditorFallback` inlines that tail any longer.
- Both accept paths produce identical observable behaviour to before (same mutations, same ordering, same error surfacing, RunFinished always fires, pushErr returned) — verified by the existing accept-path and push tests.
- `go test ./internal/commit/...`, go vet, and golangci-lint pass clean.

STATUS: Complete

SPEC CONTEXT:
The spec's load-bearing commit invariant is "mutate nothing until accept; never unwind after" (specification.md §Invariant, §Commit Flow, §Auto-push). Both accept entry points — gate-accept (`y`/Enter/`-y` at the Continue? gate) and the editor save-as-accept (the --no-ai / AI-failure / oversized fallback, where a non-empty save IS the accept) — must run the identical ordered tail: apply deferred `-a`/`-A` staging FIRST, then `git commit`, then the optional `-p` push, with push-failure handled warn-don't-unwind (keep the commit, one generic warn, git's stderr verbatim, no destructive cleanup). This task is a pure DRY refactor: collapse the two byte-identical inlined tails into one helper so the ordering/RunFinished/return-pushErr contract lives once and cannot drift.

IMPLEMENTATION:
- Status: Implemented (clean pure refactor, no behavioural change)
- Location: internal/commit/run.go:786-803 (`commitAccept` definition); call sites at run.go:367 (gate-accept, in `Run`) and run.go:445 (save-as-accept, in `runEditorFallback`).
- Notes:
  - The extraction commit (c137341) shows both inlined tails removed and replaced with single `commitAccept(ctx, deps, root, body)` calls — `finalBody` from the gate path, `saved` from the editor path. Neither `Run` nor `runEditorFallback` inlines the tail any longer (criterion 1 met).
  - The helper runs stageForMode → createCommit → pushAfterCommit → RunFinished (always) → return pushErr, exactly the ordered sequence the spec mandates; the body is the sole parameter (the only per-caller difference), which is correct.
  - Two follow-up commits landed on top of the extraction and are out of scope for 6-5 but confirm the single-source held: ef9a5a8 routed stage/commit failures through `surfaceOutput` (git stderr pass-through) and gave stageForMode/createCommit a `runner.Result` return; 842a219 added `Verb: presenter.VerbCommit` to the RunFinished call. Both changes were made in ONE place (the helper) — exactly the maintainability benefit the task targeted, demonstrated in practice.
  - No drift from the spec: post-accept never-unwind is preserved (no reset/revert/restore), push stays a no-op when disarmed, RunFinished fires even on push failure.

TESTS:
- Status: Adequate (the task is a refactor; the existing accept-path + push suites are the correct verification and exercise both call sites)
- Coverage:
  - Gate-accept path through commitAccept: run_test.go (GateEnterAccepts/GateYesAccepts/DashYAutoAccepts create the commit; RunFinished recorded in TestRun_NarratesThroughRecordingPresenter), run_push_test.go (push fires strictly after commit, plain `git push` argv, unarmed = no push, no-commit = no push, -y auto-accept push, push via git_safe), staging_defer_test.go (stage→commit ordering per mode), run_push_fail_test.go (warn-don't-unwind + RunFinished still fires + no destructive git).
  - Editor save-as-accept path through commitAccept: run_editor_push_test.go (EditorSaveCommitsThenPushes, AddAll end-to-end add<commit<push ordering, single shared push step, plain push argv, AI-failure drop then push, unarmed = no push), run_noai_test.go (non-empty save stages-then-commits per mode; whitespace-only/aborted = true no-op no mutation), run_edit_*_test.go (RunFinished recorded on accept).
  - Push-failure contract (the always-fire RunFinished + return-pushErr the helper owns) is asserted on BOTH paths in run_push_fail_test.go, including the success regression (TestRun_PushSuccess_StillFinishesZeroNoWarn) confirming RunFinished fires and no warn on a clean push.
- Notes: Not under-tested — both call sites, all three staging modes, armed/unarmed push, and the warn-don't-unwind failure case are covered with explicit ordering assertions. Not over-tested — the suites pre-date this refactor and assert observable behaviour (git invocation order, single push, recorded presenter events), not the helper's internal structure, so they correctly verify "identical behaviour after extraction" without coupling to the implementation. No new test was needed or added, which is right for a behaviour-preserving DRY extraction.

CODE QUALITY:
- Project conventions: Followed. The helper matches the file's established idiom — leading doc comment enumerating the ordered steps, `p := deps.Presenter` derived locally, errors surfaced via the existing surface/surfaceOutput helpers, every git mutation routed through the git_safe Mutator seam (never the raw runner).
- SOLID principles: Good. Single-responsibility helper that composes the already-well-extracted steps (stageForMode/createCommit/pushAfterCommit); no new coupling introduced.
- Complexity: Low. Linear, two guard returns, no branching beyond the staging/commit error checks.
- Modern idioms: Yes. Idiomatic Go error wrapping and early returns.
- Readability: Good. The doc comment makes the single-source intent and the always-fire-RunFinished / return-pushErr contract explicit.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
