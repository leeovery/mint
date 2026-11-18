TASK: A Ctrl-C ends a `regenerate --all` batch instead of skipping versions (token-aware-diff-sizing-2-11)

ACCEPTANCE CRITERIA:
- A `--source fresh` batch over v1, v2, v3 (release target) whose `ProduceBody` returns `context.Canceled` for v2: `RegenerateAll` returns an error matching `context.Canceled` under `errors.Is`. `ProduceBody` is never called for v3 and no v3 release is dispatched.
- In that same run, no `skipped …` warn is emitted for any version, no `RunFinished` event is recorded, and v2's notes stage closes as a failed `notes` stage (a `StageFailed` named `notes`), the same way single-version regenerate renders a cancelled notes stage.
- A cancel that the producer wraps in its own error is treated the same as a bare one (`regenerating fresh notes for <tag>: %w`), so the batch still ends, with no skip warn and no `RunFinished`.
- A `RegenerateAllValidated` batch with a changelog target, and one with a both target, cancelled at v2: returns an error matching `context.Canceled`; the end-of-batch rebuild never runs, `CHANGELOG.md` is not rewritten, no `git commit`/`git push` runs, no `record` stage failure is rendered.
- On every target, the cancel's error is an `*engine.AbortError` with exit code 1, so `mint release regenerate --all` exits 1 through `exitCode` and never exits 0.
- A non-cancel notes failure (wrapped `ai.ErrGenerationFailed` for v2) is still skipped as before: a `skipped <v2 tag>` warn, v3 processed, `RunFinished` listing the skip; existing skip-and-continue tests stay green unchanged.

STATUS: complete

SPEC CONTEXT: §11.5 — a Ctrl-C cancels every call in flight and aborts cleanly; no chunk notes are saved and no floor applies. §12.3 — `regenerate --all` keeps skip-and-continue as its floor for notes failures. CLAUDE.md seam 5 — `context.Canceled` propagates unchanged and is never routed to a fallback/floor. The batch was the one caller that converted a cancel into a skip; this task brings it in line with the forward release and single-version regenerate.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/engine/regenerate_batch.go:317-322 (cancel branch: `errors.Is(err, context.Canceled)` → `surface(p, "notes", err)` before the skip path); doc updates at internal/engine/regenerate_batch.go:186-190 (RegenerateAll), :262-277 (processOneVersion return contract and covered-failures list), :306-309 (comment above ProduceBody)
- Notes: The cancel is tested before `notesSkipCause`/`reportSkip`, so it never reaches the skip path. `surface` renders `StageFailed(stageFailure("notes", err))` and returns `abort(err)` (internal/engine/release.go:2022-2025), the same payload single-version regenerate renders for a cancel (internal/engine/regenerate_interactive.go:230-241, where `reachesNotesFloor` is false for a cancel so it emits no floor and no next moves). `RegenerateAll` returns `nil, err` at :203-206 with no RunFinished; `RegenerateAllValidated` returns at :151-154 before `rebuildBatchChangelog`; `exitCode` maps `*AbortError` to its ExitCode (cmd/mint/main.go:481-487). The production fresh producer wraps with `%w` on both paths (internal/engine/regenerate_fresh.go:68, :108) and the exec runner reports `ctx.Err()` on a killed child (internal/runner/exec_runner.go:106), so a real Ctrl-C reaches the branch as `context.Canceled`. The cancel path skips `chunkNotes.warnUnsaved`, which is correct: `saveFailedChunkNotes` never saves on a cancel (internal/notes/chunknotes.go:69), so there is no save error to warn about. `hint.tried(stage)` running ahead of the cancel check and the lower-cap hint on the error path belong to a later task's "however it ends" contract, not this one.

TESTS:
- Status: Adequate
- Coverage: `TestRegenerateAll_CancelEndsBatch` (internal/engine/regenerate_batch_skip_test.go:351) covers bare and producer-wrapped cancels on the release target: error matches `context.Canceled`, `*AbortError` with ExitCode 1, no `skipped ` warn, no RunFinished, first StageFailed named `notes`, ProduceBody called for exactly [v1, v2], dispatch for exactly [v1], nil bodies. `TestRegenerateAllValidated_CancelSkipsChangelogRebuild` (internal/engine/regenerate_batch_changelog_test.go:431) covers the changelog and both targets: CHANGELOG.md byte-unchanged, no git add/commit/push in the FakeRunner invocations (the Mutator is wired over the same FakeRunner, as `committedSubject` in that file relies on), and no StageFailed other than `notes`. `TestRegenerateAll_NotesFailureStillSkipsWithWarn` (internal/engine/regenerate_batch_skip_test.go:397) proves a non-cancel failure is still skipped and continues. Each test would fail if the branch were removed: v3 would be produced and dispatched, a skip warn and RunFinished would be recorded, and on changelog targets the rebuild would rewrite the file and commit.
- Notes: The skip warn in the non-cancel test now reads `AI returned empty notes` rather than the criterion's `notes generation failed`. That changed after this task, when §12.3's cause-phrase skip reasons landed. It is deliberate and sound, and the skip-and-continue behaviour the criterion protects still holds. The task commit (7cec4ec) only added tests and left the existing skip tests unchanged. The release-target cancel goes through `RegenerateAll` directly rather than `RegenerateAllValidated`; the validated entry passes the error straight through at :151-154, so nothing goes unproven. The tests are not over-tested: the shared `assertCancelEndedBatch` helper keeps the assertions in one place, with no duplication across the three cases.

CODE QUALITY:
- Project conventions: Followed (sentinel matched with `errors.Is`; cancel propagated unchanged per seam 5; failure rendered through the presenter via `surface`; tests use FakeRunner + RecordingPresenter, external package, `t.Parallel()`, table-driven)
- SOLID principles: Good
- Complexity: Low (one guarded early return)
- Modern idioms: Yes
- Readability: Good
- Issues: None. The added comments match the code and carry no task ids; the "5-7"/"5-13" references in the file come from earlier work.

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
