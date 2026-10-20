## Attempt 1

ISSUES:
- /Users/leeovery/Code/mint/internal/engine/release_reviewgate_unwind_test.go:194 and :202. In `TestRelease_StdinClosedAtTheRegenNudge_MakesNoCallAndUnwindsThePreTagCommit`, the `tr.regenCalls() == 0` check passes no matter what. The transport is built with `nudge: ""`, so it only counts prompts that start with `notes.DefaultPrompt + "\n\n\n\n"`. No real call has that prefix. `appendOneTimeContext` (internal/notes/generate.go:248-252) adds nothing for an empty context, so an `r` call with an empty context is byte-identical to the first attempt and falls to the `default` case. The criterion "no AI call is made" therefore has no real check. Example: if the closed-input path ever called the AI anyway (an empty-context regenerate, or a call before `AskLine`), the counter would stay at 0.
  FIX: Count every transport call and assert there was exactly one, the first attempt. The simplest version uses the existing `freshTransport` (regenerate_fresh_test.go:30): `tr := &freshTransport{body: firstSinglePass}`, then `if got := tr.calls(); got != 1 { t.Errorf("AI calls = %d, want only the first attempt's", got) }`. Drop the `regenCalls` assertion and the `gateAbortTransport` from this test.
  ALTERNATIVE: Add a total-call counter to `gateAbortTransport.Generate` and assert it is 1. This keeps one transport type across the file, but it adds a field that only this test uses. I recommend the `freshTransport` version.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/engine/release.go:1594-1596. "the caller routes every error it returns through the shared auto-unwind" is false: `reviewGate` has two other callers in regenerate (regenerate_write.go:236, 238), and neither unwinds.
  OLD:
// The gate never unwinds: the caller routes every error it returns through the
// shared auto-unwind, which authors the Unwound and resets whatever mint made (a
// pre_tag artifact commit lands before the gate).
  NEW:
// The gate never unwinds or emits an Unwound; any reset is the caller's.
- /Users/leeovery/Code/mint/internal/engine/release.go:587-591. Most of this comment restates the code: the `return Unwind(...)` call, and the dry-run return a few lines above. The only fact the code cannot show is why this is `Unwind` and not `surfaceAndUnwind`. Each line is indented with two tabs.
  OLD:
		// Every way the gate ends the run — a decline, a failed `r`, a failed editor,
		// closed input, a cancel — takes the surgical unwind a pre-push failure takes,
		// resetting any pre_tag artifact commit tracked in made. reviewGate has already
		// surfaced whatever needed a StageFailed. A dry run returned above and never
		// reaches this gate.
  NEW:
		// Unwind, not surfaceAndUnwind: reviewGate has already surfaced whatever
		// needed a StageFailed.

NOTES:
- The ladder case in `TestRelease_FailedChunkedRegen_UnwindsThePreTagCommit` (release_reviewgate_unwind_test.go:157-167) checks the unwind but not that the chunked run actually happened. The fixture matches regen_halve_test.go's halving setup (10_000-byte two-file diff, SIZE answer), so the chunked path does run. Adding `errors.Is(err, ai.ErrTimeout)` would prove it inside the test, since only chunk calls get a timeout. Not blocking: the code under test routes every error the same way.
- release_reviewgate_unwind_test.go:128 and :318 read `tr.classifs` directly, while `regenCalls()` takes the lock. It is race-safe because `Release` has returned. It is only inconsistent.
- `reviewGate` already returns an `*AbortError`, and `Unwind` wraps it again with `abort(reason)`, giving `abort(abort(cause))`. Before this change the decline path passed the raw `errGateAborted`. `errors.Is`, the exit code and `exitCode()` are unchanged, and cmd/mint does not print engine errors. The only effect is a doubled "aborting (exit 1):" prefix in `Error()`. The floor path (notesfloor.go:202, then release.go:550) already does the same.
- Some stale comments outside this diff now contradict the edited ones. None are in scope, but they could go in the same pass:
  - release.go:579-580: "The gate stays positioned BEFORE any mutation (before Record)". The pre_tag artifact commit lands before the gate.
  - `regenerateBody`'s doc: "surfaced before any mutation".
  - `errEditorUnavailable` and `errRegeneratorUnavailable`: "before any mutation".
  - The `r` case comment in `reviewGate`: "Any failure (closed input, nil seam, regenerate error) is surfaced". Closed input is aborted without a StageFailed.
- Nothing arrived in the dispatch beyond the enumerated inputs.
