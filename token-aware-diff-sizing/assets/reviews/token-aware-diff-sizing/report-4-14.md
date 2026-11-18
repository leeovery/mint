TASK: One lower-cap hint for a `regenerate --all` batch (token-aware-diff-sizing-4-14)

ACCEPTANCE CRITERIA:
- Given `regenerate --all` in which v1.1.0 and v1.3.0 each have notes written after halving, no hint shows after either version's notes, and exactly one hint shows when the batch ends, at a terminal and under `-y` alike (§10.3)
- Given that batch, where v1.3.0's largest successful single-pass, chunk or piece call is 1,260,000 bytes and v1.1.0's is 700,000, the hint says the cap is set higher than this model accepts and shows `max_prompt_bytes = 1260000`. A larger successful diff-carrying call in a version that needed no halving sets the value instead (§10.3)
- The value is that call's exact byte count, never rounded and never in the narration's decimal units (§10.3)
- Given a batch in which no version's notes were written after halving, including one whose only halving was in a version that then failed and was skipped, no hint shows (§10.3)
- Given a batch in which a version's notes were written after halving, and the operator then declines a gate, at that version or a later one, the hint still prints once as the batch ends. A batch that ends at a failed `r` prints it the same way, and so does one ended by a Ctrl-C, on the way out (§10.3, §11.5)

STATUS: complete

SPEC CONTEXT: §10.3 (with the 2026-10-19 corrigenda): after notes written following halving, mint suggests the largest diff-carrying call (single-pass, chunk, piece; never merge/classify/exclusion-check) that succeeded, as its exact byte count `max_prompt_bytes = N`. Under `regenerate --all` the hint prints once at the end of the batch, naming the largest such call across the batch, and prints however the batch ends (declined gate, failed `r`, Ctrl-C) once any version's notes were written after halving; the hint records no state, so §11.5's clean cancel holds. §16.3 lists the hint as a follow-up after the notes.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/notesstage.go:237-268 — `batchLowerCap` accumulator: `tried` folds a version's largest succeeded call whether or not its notes were written; `written` additionally sets `halved` when that version's notes were written after halving (`s.lowerCap() != 0`, which requires a halving plus a merge); `regenWritten` sets `halved` for an `r` whose notes were written after halving; `lowerCap` returns the batch-wide largest call only once `halved`.
  - internal/engine/notesstage.go:270-280 — `hintLowerCap` renders the value with `strconv.Itoa` (exact bytes, no decimal units).
  - internal/engine/regenerate_batch.go:200-218 — one `batchLowerCap` per batch; the hint is emitted on the early-exit error path (line 204) and before `RunFinished` on the normal path (line 218), and nowhere per version.
  - internal/engine/regenerate_batch.go:319 / 330 / 336 / 338 — a failed or cancelled generation folds its calls only (`tried`); written notes fold through `written`; the gate's `r` stage is handed `hint.regenWritten` rather than a printing callback; `tried(stage)` after the gate folds the `r`'s calls (forwarded into the version stage via `regenStage.CallSucceeded`), including on a failed `r` or decline.
  - internal/engine/regenerate_batch.go:463-479 — `gatePerVersion` leaves `RegenerateWriteRequest.LowerCap` at zero, so `gateRegenerate`'s per-notes hint (internal/engine/regenerate_write.go:230) never fires in the batch.
- Notes: Every exit from the loop (declined gate, failed `r`, cancel during production, gate keypress Ctrl-C which the pretty gate maps to ErrInputClosed, publish failure) passes through the line-204 hint. A skipped version's succeeded calls still count toward the value, which matches "the largest such call that succeeded across the batch"; a skipped version's halving does not arm the hint, per AC4. Production wiring threads the version's notes stage into `RegenerateFreshBody` as its sinks (cmd/mint/regenerate_all.go:117), so the real generator's `Halved`/`CallSucceeded`/`Merged` reach the accumulator. The Ctrl-C path is grounded in `signal.NotifyContext` cancelling the run context (cmd/mint/main.go:74).

TESTS:
- Status: Adequate
- Coverage:
  - internal/engine/regenerate_batch_lowercap_test.go:97 — two halved versions (700,000 and 1,260,000), at a terminal and under `-y`: exactly one hint, value 1260000, followed only by `RunFinished` (so no per-version hint).
  - :129 — a larger 1,300,001-byte call in an unhalved version sets the value (non-round value also proves no rounding; message compared verbatim).
  - :145 — a larger call in a version that failed and was skipped sets the value.
  - :160 — no halving anywhere, and halving only in a version that failed and was skipped: no hint.
  - :186 — decline at the halved version, decline at a later version, failed `r` at a later version, Ctrl-C (context.Canceled) at a later version: one hint, and it is the batch's last event.
  - internal/engine/regen_halve_test.go:536 and :590 — real `RegenerateFreshBody`/`RegenerateFreshRegenerator` in a batch: an `r` that halves prints no hint of its own and arms the batch-end hint; the value is an earlier `r`'s larger call.
- Notes: Tests would fail if the hint printed per version (count != 1), at the wrong position, with the wrong value, or not on the early-exit path. The table-driven early-exit test is focused with no redundant assertions.

CODE QUALITY:
- Project conventions: Followed (output only through the presenter, external test package, `t.Parallel()`, exact rendered-warning assertions, table-driven cases)
- SOLID principles: Good — the accumulator is a small single-purpose type; the `r` path reuses the existing `regenStage` hint callback seam via a method value
- Complexity: Low
- Modern idioms: Yes (builtin `max`)
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
