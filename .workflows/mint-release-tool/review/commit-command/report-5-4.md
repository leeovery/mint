TASK: 5-4 — Warn-don't-unwind on push failure (tick-803aaf)

ACCEPTANCE CRITERIA:
- On push failure, mint runs no git reset/revert/restore/unstage/amend — commit and pre-existing staging untouched (never-unwind)
- On push failure, one generic warn via presenter.Warn (Label "push"; commit in place; re-run the push) — same warn for rejected/remote-moved/no-upstream/network
- git's stderr carried as Warning.Output, rendered verbatim beneath the warn (consumed Presenter pass-through); out-only, only the one-line warn summary duplicated to stderr
- The warn does not suppress the success close-out and sets no presenter failure state; non-zero exit comes from the engine mapping, not the warn
- No cause classification — a single failure branch for all causes
- No-upstream case surfaces git's own hint via verbatim pass-through, not a mint-authored "set an upstream" message
- Commit stays forward-only and the push is repeatable
- Failure handling wraps the single shared push step (gate-accept 5-2 and editor-accept 5-3 behave identically)
- On push failure the command exits non-zero while the commit remains in place

STATUS: Complete

SPEC CONTEXT: Auto-push Behaviour + "Invariant — mutate nothing until accept; never unwind after". A push can fail for many reasons; mint keeps the commit, emits ONE generic warn, passes git's stderr through verbatim, never unwinds, and exits non-zero. mint does NOT classify the cause; git's specific hint stays visible only via the verbatim pass-through. The push is a best-effort final step whose failure is reported, not repaired. The post-accept never-unwind invariant is absolute — no destructive cleanup path at all.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/commit/run.go:844-861 (pushAfterCommit), :786-803 (commitAccept — the single shared accept tail), :108-129 (errPushFailed sentinel + warn label/message constants), :440-445 (editor save-as-accept routes through the same commitAccept); cmd/mint/main.go:416-422 (exitCode — plain error falls through to exit 1).
- Notes: Strong. pushAfterCommit is the ONE place both accept paths reach (gate-accept via Run→commitAccept; editor save-as-accept via runEditorFallback→commitAccept), so the failure handling is genuinely a single shared step — not duplicated per path. On `git push` failure it (1) emits exactly one presenter.Warn{Label:"push", Message: generic, Output: res.Stderr verbatim}, (2) returns errPushFailed, and (3) does nothing destructive. commitAccept fires RunFinished UNCONDITIONALLY before returning pushErr, so the warn never suppresses the success close-out and sets no failure state. No reset/revert/restore/--amend/unstage/stash/rm exists anywhere in run.go or staging.go (grep confirms — only present in comments describing what is forbidden); the never-unwind invariant is structural, not merely asserted. No cause classification: a single `if err != nil` branch handles all causes with the same warn. The no-upstream hint is left entirely to git's stderr in Output — the constant pushFailWarnMessage ("commit is in place; re-run the push to retry") deliberately omits any "upstream"/"non-fast-forward" phrasing. The argv is exactly `git push` (no -u/upstream args), flowing through the git_safe Mutator seam.
- Exit-code resolution: the spec note flagged that cmd/mint's exitCode() matches only *engine.AbortError (whose abort() constructor is unexported). The implementation chose the documented alternative — errPushFailed is a plain error, so exitCode() falls through to `return 1` (generic non-zero). This satisfies the AC (deterministic non-zero exit signalling the failed push) without touching engine's unexported abort(). Clean, well-documented choice (run.go:108-116).

TESTS:
- Status: Adequate
- Coverage: internal/commit/run_push_fail_test.go covers every listed micro-acceptance: rejected/non-fast-forward keeps commit + generic warn (TestRun_PushFailure_RejectedKeepsCommitAndWarns); exits non-zero with commit in place (ExitsNonZeroCommitInPlace); no-upstream same generic warn, no per-cause message (NoUpstreamSameGenericWarn); network same generic warn, asserted byte-identical to the rejected warn (NetworkSameGenericWarn); stderr verbatim out-only as Warning.Output (StderrVerbatimOutOnly); no destructive git after failure (NeverUnwinds — scans every invocation for reset/revert/restore/rm/checkout/stash and --amend); commit forward-only, exactly one commit + one push, no rewrite (CommitForwardOnly); no-upstream hint only in Output, mint Message free of "upstream"/"set an upstream" while git's hint present in Output (NoUpstreamHintOnlyInOutput); same generic warn for the editor save-as-accept push failure proving the single shared step (EditorAcceptSameGenericWarn). The solePushWarn helper additionally asserts exactly one Warn and NO StageFailed/Unwound accompanies it (the warn alone narrates; exit comes from the sentinel). Success regression (run_push_fail_test.go:366 TestRun_PushSuccess_StillFinishesZeroNoWarn) proves a successful push returns nil, fires RunFinished, and emits no warn.
- Notes: Well-balanced — focused on behaviour (presenter events, recorded git invocations, exit shape) not implementation internals. Not over-tested: the rejected-cause warn is reused as the cross-cause oracle rather than re-asserting the literal message string in each test, which keeps the "one generic warn" property honest without pinning brittle text. The presenter's out-only rendering (the "--- output ---" delimiters and "only the summary duplicated to stderr") is correctly NOT re-tested here — it is owned and tested in the cli-presentation presenter package; this task only verifies it consumes the seam (Warning.Output carries git's verbatim stderr), which matches the spec's "consume the seam as ONE call, do NOT build a new warn renderer."

CODE QUALITY:
- Project conventions: Followed. Consumes the git_safe Mutator seam (not the raw runner) for the push; consumes the presenter Warn seam rather than printing stderr itself; sentinel error is a package-private var with errors-chain-friendly plain text. Matches surface.go's verbatim-Output pass-through convention (surfaceOutput) used by the stage/commit failures.
- SOLID principles: Good. Single responsibility — pushAfterCommit does push-or-nothing + the one failure branch; commitAccept owns the ordered stage→commit→push tail in one place so the invariant cannot drift between the two accept entry points. No classification logic leaks in (open/closed: adding a cause would not change this branch).
- Complexity: Low. pushAfterCommit is a guard + one Mutate + one if-err branch.
- Modern idioms: Yes. errors sentinel + errors.As fall-through in exitCode; context threaded through.
- Readability: Good — arguably exemplary. The doc comments are unusually thorough and pin every spec constraint (never-unwind absolute, no pre-push gate, one generic warn, verbatim Output, exit via sentinel) to the code.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/commit/run_push_fail_test.go:108-361 — No failed-push test asserts RunFinished still fires. The AC "the warn does not suppress the success close-out" is directly asserted only on the SUCCESS path (TestRun_PushSuccess_StillFinishesZeroNoWarn:381). The implementation guarantees it (commitAccept calls RunFinished unconditionally before returning pushErr), but a failed-push test that asserts hasKind(rec, KindRunFinished) AND exactly one push warn would lock the "warn does not suppress close-out" contract on the path where it actually matters. Add one assertion (the hasKind helper already exists at :387) to an existing failed-push test, e.g. RejectedKeepsCommitAndWarns.
