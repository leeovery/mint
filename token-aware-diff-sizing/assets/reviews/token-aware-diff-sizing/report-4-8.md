TASK: Failures that skip classification (token-aware-diff-sizing-4-8)

ACCEPTANCE CRITERIA:
- Given a single-pass call that returns an empty or whitespace-only body with nothing but whitespace on stderr on both attempts, no classify call is made and no chunk call follows. The ✗ line reads `AI returned empty notes` (§9.5, §12.4)
- Given a failed call with text only on stdout, or only on stderr, a classify call is made (§9.5)
- Given a `max_prompt_bytes` smaller than the classify prompt a failed notes call would produce, no classify call is sent and no chunk call follows. The ✗ line reads `AI call failed` with the original call's captured output verbatim beneath it (§9.2, §12.4)
- Given a classify call that fails on both attempts, times out, or whose program is not found, no chunk call follows. The ✗ line reads `AI call failed` with the original notes call's captured output verbatim beneath it, never the classify call's (§9.5)
- Given a single-pass, chunk or merge call that times out, or whose program is not installed, no classify call is made, and the ✗ line reads `AI timed out` or `AI tool not installed` (§9.1, §12.4)
- Given a Ctrl-C while the classify call runs, or while a halving round's call runs, the run aborts as a cancel: no further AI call is made, and under `on_notes_failure = fallback` no fallback body ships (§9.1, §9.5, §11.5)

STATUS: complete

SPEC CONTEXT: §9.1 limits the classifier to generation failures (non-zero exit or empty body surviving the transport retry); a timeout or missing binary goes straight to the floor and a cancel propagates unchanged, never reaching on_notes_failure. §9.5: no error text on either stream skips classify and takes OTHER; a classify call that fails, times out or finds no binary takes OTHER with the original call's output rendered. §9.2 plus the 2026-10-19 corrigendum: a classify prompt over max_prompt_bytes is not sent and the failure takes OTHER (no further trimming). §12.4 phrase table: `AI call failed` (OTHER with output), `AI returned empty notes` (OTHER, no output), `AI timed out`, `AI tool not installed`, with a `(part k of n)` / `(merging)` suffix in a chunked run. §11.5: a Ctrl-C cancels every call in flight, saves no chunk notes, and on_notes_failure does not apply.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/classify.go:30-37 (`generate`: one notes call, failure handed to readFailure)
  - internal/notes/classify.go:41-47 (`readFailure`: returns the original failure with classify's code, or the classify call's cancel in its place)
  - internal/notes/classify.go:55-57 (no GenerationError, e.g. ErrTimeout/ErrCommandMissing/cancel, or `Output() == ""`, which is empty when both streams are whitespace-only per internal/ai/transport.go:97-105 -> OTHER with no call)
  - internal/notes/classify.go:58-61 (classify prompt over maxPromptBytes -> not sent, OTHER)
  - internal/notes/classify.go:62-68 (classify cancel returned; any other classify failure -> OTHER, original failure kept)
  - internal/notes/generate.go:232-241 (single-pass: only codeSize halves; everything else returns the original failure, or the cancel)
  - internal/notes/chunk.go:82-91 (merge goes through the same generate/readFailure path)
  - internal/notes/chunk.go:209-241 (wave.send: chunk-call failures go through readFailure; a cancel reads codeOther so it never halves)
  - internal/notes/chunk.go:184-186 (wave.run returns ctx.Err() with no parts once cancelled; canSend at :195-197 stops new calls)
  - internal/notes/chunknotes.go:68-70 (a cancel saves no chunk notes)
  - internal/notes/resolve.go:55-57 (ResolveFailure returns a cancel unchanged in fallback mode too) and internal/engine/notesfloor.go:32-38 (a cancel never reaches the floor)
  - internal/notes/resolve.go:191-197 (the phrase depends on whether the original GenerationError's Output() holds text)
- Notes: Each acceptance path traces correctly through the code. The single-pass, chunk and merge calls all go through the same readFailure, so the skip rules apply the same way to every release-notes call. The production exec runner wraps ctx.Err() on a cancelled command (internal/runner/exec_runner.go:106), so a Ctrl-C reaches classify as context.Canceled, not as a retryable non-zero exit. The cap check measures the composed, trimmed classify prompt against the same max_prompt_bytes, as the corrigendum requires. I found no drift from the spec or the plan.

TESTS:
- Status: Adequate
- Coverage:
  - AC1/AC2: internal/engine/notes_classifyskip_test.go:79 drives the real ai.Transport through a FakeRunner across four cases: empty body with whitespace stderr, whitespace body with whitespace stderr, text only on stdout, text only on stderr. It asserts the exact AI call count (2 = both attempts, no classify or chunk call; 3 = attempts plus classify), the third call is the classify prompt, and the exact ✗ message and Output. A broken `Output() == ""` guard would use the seeded fourth reply and fail the count.
  - AC3: internal/engine/notes_classifyskip_test.go:154 sets a 6,000-byte cap with a classify fake that would answer SIZE if called. It asserts exactly one prompt (no classify, no chunk), checks the single-pass prompt fits the cap as a fixture check, and asserts `AI call failed` with `failure.Output()` verbatim. internal/notes/classify_test.go:353 pins the exact boundary: a cap equal to the classify prompt sends it, one byte short does not.
  - AC4: internal/engine/notes_classifyskip_test.go:189 covers a classify GenerationError, ErrTimeout and ErrCommandMissing. It asserts exactly the notes call plus one classify call, and that the ✗ line shows the notes call's output, not the classify call's.
  - AC5: internal/engine/notes_classifyskip_test.go:231 covers a 3x2 matrix (single-pass/chunk/merge x timeout/missing binary). It asserts zero classify calls and the exact phrase with its `(part 2 of 2)` / `(merging)` suffix.
  - AC6: internal/engine/notes_classifyskip_test.go:333 (Ctrl-C during the single-pass call's and the merge call's classify), :397 (during a chunk call's classify, with the other chunk held in flight) and :418 (during a halving round's chunk calls), all under fallback. assertInterrupted (:304) checks errors.Is Canceled, no mutation, and no `git log --format=%s` (the fallback body was never built). Each also asserts no AI call after the cancel: the last call is the classify call, the call count is exactly 3, or there are no merge calls and exactly one classify call. The wave's start-no-more-after-cancel mechanism is also pinned at internal/notes/chunkfail_test.go:234.
- Notes: The tests are focused, and every case maps to a criterion. The 3x2 matrix in AC5 is the criterion's own matrix, so it is not over-testing. Each test would fail if the behaviour it names broke.

CODE QUALITY:
- Project conventions: Followed. AI goes through the consumer-defined Transport; context.Canceled propagates unchanged and is never routed to a fallback (CLAUDE.md seam 5). Consumers do not re-retry. Errors are matched with errors.Is/As. Tests use external packages, t.Parallel and table-driven cases.
- SOLID principles: Good. `classify` owns the skip decisions in one place, and `readFailure` owns the cancel substitution.
- Complexity: Low
- Modern idioms: Yes
- Readability: Good. The doc comments on generate/readFailure/classify (internal/notes/classify.go:29, :39-40, :49-52) match what the code does.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
