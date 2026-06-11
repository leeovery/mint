TASK: mint-release-tool-5-12 — Batch --all skip-and-continue & end summary

STATUS: Complete (one non-blocking pretty-mode presentation bug)

IMPLEMENTATION:
- Status: Implemented (skip logic lives in internal/engine/regenerate_batch.go — no separate _skip.go file; tests in regenerate_batch_skip_test.go). RegenerateAllValidated:105 up-front config check, checkBatchTargetConfig:132, RegenerateAll:153 loop catches per-version skip + continues + end summary:174, processOneVersion:209 returns *skippedVersion for body-less reuse:231 + notes failure:247, reportSkip:292 via non-terminal Warn, classifyNotesFailure:300, batchSummary:312. Body-less reuse skip uses same ReadTagBody has-body branch as 5-5 (branches on empty rather than erroring). Config abort reuses ErrChangelogDisabled sentinel. Skip uses Warn (non-terminal) not StageFailed.

TESTS:
- Status: Adequate. regenerate_batch_skip_test.go: diff-too-large skip+continue, body-less reuse skip+report, override of on_notes_failure=abort (oldest-version failure), config abort up-front (no dispatch/block/summary, errors.Is(ErrChangelogDisabled)), end-summary shape (Verb/Project/empty-URL/verbatim Summary, no-skip plain-count variant). changelog_disabled_test.go pins sentinel wording.

CODE QUALITY:
- Followed conventions (sentinel-error idiom, runner/presenter seams, focused tests, dense doc comments). SOLID/DRY good — processOneVersion factored, skip reasons named constants, batchSummary pure. Low complexity.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [bug] internal/engine/regenerate_batch.go:244-248 — on the notes-production-failure skip path the blocking notes spinner is started (emitBlockingStageStarted) but notesDone() is never called and reportSkip→Warn does NOT stop the spinner (presenter/pretty.go:520 Warn never calls stopSpinner; RunFinished:905 also never stops it). For the last/only version that hits diff-too-large, the pretty-mode spinner animates over the `⚠ skipped` and end-summary lines until exit. (Body-less reuse skip is clean — returns before spinner starts.) Invisible to tests (RecordingPresenter has no live spinner). Fix: stop/close the stage before warning on the notes-failure skip (emit a StageFailed for "notes" before reportSkip, or have Warn/the skip path stop any active spinner).
- [quickfix] internal/engine/regenerate_batch.go:227 + cmd/mint/regenerate_all.go:91 — reuse --all reads each tag's annotation body twice for body-bearing tags (ReadTagBody pre-check + ProduceBody→ReadReuseBody). Redundant git call per version. Thread the already-read body into the reuse producer, or have it skip the re-read. [Same as 5-11 note.]
- [do-now] task-input file reference names internal/engine/regenerate_batch_skip.go which does not exist; skip-and-continue logic lives in internal/engine/regenerate_batch.go. Correct the as-built reference.
