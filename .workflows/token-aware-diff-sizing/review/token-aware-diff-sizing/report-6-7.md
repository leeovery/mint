TASK: Every abort at the review gate unwinds (token-aware-diff-sizing-6-7, tick-e0b078)

ACCEPTANCE CRITERIA:
- Given a release whose pre_tag hook changed files, so the run made the pre-tag artifact commit, and an `r` whose single-pass call fails with output and is classified OTHER, the ✗ line reads `AI call failed` with that call's captured output verbatim beneath it. The run then resets to the starting commit, the artifact commit gone, narrates the unwind, makes no tag and exits 1 (§6.6, §12.4, §15.1)
- Given the same release, an `r` on chunked notes whose re-merge fails, or an `r` the ladder turned into a chunked run that fails, unwinds the same way and exits 1 (§15.1)
- Given the same release and stdin closing at the `r`'s nudge prompt, no AI call is made, and the run unwinds the same way and exits 1 (§15.1)
- Given the same release and a launched editor that exits non-zero at the review gate's `e`, the failure is surfaced, and the run unwinds the same way and exits 1. Given stdin closing at the review gate's keypress instead, the run unwinds the same way and exits 1 (§15.1)
- Given the review gate's `e` with no editor that can be launched, mint warns that it could not launch the editor and the review gate asks again with the body unchanged; nothing is reset (§15.1)
- Given a Ctrl-C while an `r`'s AI call runs, the run aborts as a cancel and unwinds the same way, making no tag (§9.1, §15.1)
- Given `on_notes_failure = "fallback"` and a failed `r`, the run unwinds the same way and exits 1 before any tag: no commit-subject list ships, no fallback warning prints, and no `c` / `e` / `a` choice shows (§12.1, §15.1)

STATUS: complete

SPEC CONTEXT: §15.1 "Every abort at the review gate unwinds" and its 2026-10-19 corrigenda: the review gate sits after the pre-tag artifact commit, so a decline, a failed `r` (including stdin closing at its nudge prompt), a launched editor failing at `e`, and stdin closing at the gate's keypress all unwind as any pre-push failure does (§6.6), back to the starting commit. A missing editor still warns and returns to the gate. A failed `r` aborts whatever `on_notes_failure` holds and never reaches the floor (§12.1). A Ctrl-C during an `r` is a cancel, not a failed `r`, and propagates unchanged (§9.1). Regenerate's review gate precedes its changelog commit, so it has nothing to reset (task 6-10's territory).

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/release.go:609-614 — `Release` now routes every error from the review gate (via `chunkNotes.reviewGate`) through `Unwind(ctx, deps, start, made, err)`; the as-built `errors.Is(err, errGateAborted)` filter that returned every other gate error as-is is gone.
  - internal/engine/release.go:1739-1774 — `reviewGate` doc updated: the stale "the gate sits BEFORE any mutation, so the unwind finds nothing to reset" claim removed; "The gate never unwinds or emits an Unwound; any reset is the caller's" added.
  - internal/engine/unwind.go:26-30 — WIRING comment updated to "every review-gate abort and every pre-push failure".
  - Pre-existing and unchanged, relied on: missing editor -> `ErrEditorReturnToGate` -> `continue` (release.go:1802-1808, editor.go:113-118); `Unwind` detaches from cancellation (unwind.go:121), so a Ctrl-C during `r` still resets.
- Notes: The change covers every exit of `reviewGate`, including ones the task does not list (ErrNotInteractive from a non-TTY prompt, the unexpected-choice contract violation), which is what "every way the review gate ends a release before the push" requires. Nothing under `made` is lost: the answers commit re-commit and tag handling flow through the same `Unwind`. Passing an already-`*AbortError` cause into `Unwind` double-wraps it, but `exitCode()` (cmd/mint/main.go:481) reads the outer ExitCode (1) and nothing prints the error text, so it has no visible effect. The dry-run boundary (release.go:600-602) still returns before the gate.

TESTS:
- Status: Adequate
- Coverage (internal/engine/release_reviewgate_unwind_test.go): one test per criterion, each on a prior-tag release whose pre_tag hook made the artifact commit (`gateAbortRelease`), with a shared `assertUnwoundToStart` that checks a non-zero abort, `git reset --hard <startingSHA>`, Unwound summary "reset 1 commit; repo clean", no `git tag -a`, no push/publish, and no RunFinished after the Unwound.
  - AC1: TestRelease_FailedSinglePassRegen_ShowsTheCallsOutputAndUnwindsThePreTagCommit (:114) asserts one classify call, the single StageFailed message "AI call failed" with the call's Stdout verbatim, StageFailed before Unwound, then the unwind.
  - AC2: TestRelease_FailedChunkedRegen_UnwindsThePreTagCommit (:136) covers both the merge-only re-merge failing and a SIZE-classified `r` the ladder chunked whose part calls time out.
  - AC3: TestRelease_StdinClosedAtTheRegenNudge_MakesNoCallAndUnwindsThePreTagCommit (:182) asserts ErrInputClosed, only the first attempt's AI call, then the unwind.
  - AC4: TestRelease_FailedEditorAtTheReviewGate_SurfacesAndUnwindsThePreTagCommit (:205) (StageFailed "edit", then unwind) and TestRelease_StdinClosedAtTheReviewGateKeypress_UnwindsThePreTagCommit (:227).
  - AC5: TestRelease_MissingEditorAtTheReviewGate_WarnsAndAsksAgainWithNothingReset (:251) drives the real `EditorLauncher` with a not-found binary and asserts the exact warning, two gate asks, the unchanged body in the tag annotation, no reset and no Unwound. It is non-parallel because of t.Setenv, and its comment says so.
  - AC6: TestRelease_CtrlCDuringAnRegensCall_AbortsAsACancelAndUnwindsThePreTagCommit (:296) asserts context.Canceled is preserved, zero classify calls, and the unwind.
  - AC7: TestRelease_FailedRegenUnderFallback_UnwindsWithoutTheFallbackOrTheFloor (:321) asserts no `git log --format=%s <prior>..HEAD` (the exact argv `notes/resolve.go:22` issues), no warnings, and no floor gates.
- Notes: Each test would fail if `Release` went back to returning that gate error as-is, because the `git reset --hard` and Unwound assertions would fail. The AC4 editor case injects a `fakeEditor` error rather than a launcher non-zero exit. That is sufficient because the engine treats every non-ReturnToGate editor error identically. The suite is not bloated: one focused case per criterion, with shared assertions in one helper.

CODE QUALITY:
- Project conventions: Followed. Recovery goes through the single `Unwind` seam, output goes through the presenter, and the WHY-comments were updated in the same change as CLAUDE.md "Comments" requires.
- SOLID principles: Good
- Complexity: Low. The change removes a branch.
- Modern idioms: Yes
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
