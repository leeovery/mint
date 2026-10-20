# Consolidation Findings: token-aware-diff-sizing (Phase 2)

## Findings

### F1: A Ctrl-C during `regenerate --all` is recorded as a skipped version, and the batch reports success
- **Class**: behaviour
- **Failure**: An operator presses Ctrl-C while `mint release regenerate --all --source fresh` is writing a version's notes. The cancelled `ProduceBody` error goes to `reportSkip`, so that version is listed as `skipped (notes generation failed)`. Every later version then fails straight away on the dead context and is skipped the same way. The batch emits its `RunFinished` summary and `RegenerateAll` returns `collected, nil`. On a `--target release` run mint exits 0. A script or agent reads the interrupted batch as a finished one with a few skips, and a person sees versions they never meant to skip listed in the summary. On a changelog or both target, `rebuildBatchChangelog` then runs on the cancelled context, and the run ends on a `record` stage failure rather than a cancel. This breaks the contract the phase established. Task 2-10 made a chunked wave return `ctx.Err()` over any chunk failure so callers can tell a cancel apart from a failure (spec §11.5: "A Ctrl-C cancels every call in flight and aborts cleanly"). It also made `ResolveFailure` pass `context.Canceled` through unchanged. The forward release and single-version regenerate both honour the contract. The batch is the one caller of the phase's notes stage that still turns a cancel into a skip. The plan's task 4-14 also assumes a Ctrl-C ends a batch ("one ended by a Ctrl-C, on the way out").
- **Evidence**:
  - internal/engine/regenerate_batch.go:286-289: `ProduceBody` error → `reportSkip(p, res.Tag, reasonNotesFailed)`, with no `context.Canceled` check
  - internal/engine/regenerate_batch.go:197-207, 215-220: the loop records the skip and continues, then emits `RunFinished` and returns `collected, nil`
  - internal/engine/regenerate_batch.go:150-157: `RegenerateAllValidated` goes on to `rebuildBatchChangelog` on the same cancelled context
  - cmd/mint/regenerate_all.go:74-77: a nil error exits 0
  - internal/notes/chunk.go:99-101: the wave returns `ctx.Err()` over any earlier chunk failure
  - internal/notes/resolve.go:69-71: the release path passes a cancel through `ResolveFailure`
  - internal/engine/regenerate_interactive.go:207-210: single-version regenerate surfaces the same error as an abort
- **Proposed shape**: In `processOneVersion`, check `errors.Is(err, context.Canceled)` on the `ProduceBody` error before `reportSkip`. Return `RegeneratedVersion{}, nil, surface(p, "notes", err)`, as `RegenerateRun` does. `RegenerateAll` then ends the batch with no `RunFinished`, as a gate decline does. `RegenerateAllValidated` skips the rebuild, and `exitCode` maps the error to 1. Update the comment at regenerate_batch.go:278-285 and the covered-failures list in the `processOneVersion` doc to say a cancel is not skipped. Proving test: a `ProduceBody` that returns `context.Canceled` for v2 makes `RegenerateAll` return an error matching `context.Canceled`. No `ProduceBody` call is made for v3, no `skipped` warn is emitted and no `RunFinished` event is recorded.
- **Bank**: "A Ctrl-C during regenerate --all is treated as a skipped version, not an abort" (token-aware-diff-sizing-2-10, executor, deposited three times)

## Comment Corrections

- internal/commit/run.go:795-796: workflow vocabulary ("Phase 4's `r`") and history; in this epic, Phase 4 means failure classification, so the claim reads wrongly
  OLD: // under its own note. It is a NAMED predicate so Phase 4's `r` (regenerate)
// failure can reuse the SAME routing entry point.
  NEW: // under its own note.
- internal/commit/run.go:54-56: cites a spec section; the rest repeats the literal and what Warn does
  OLD: // The message is the spec string VERBATIM (Commit Message Format & Prompt → The $EDITOR
// fallback, case 3), em dash U+2014; the label is a short informational tag prefixed
// onto the warn line.
  NEW:
- internal/config/config.go:22-23: says the provider-value carve-out is a future "Phase 6 task", but it is already built (the `Provider` field doc at config.go:161-165 describes the downgrade route); also workflow vocabulary
  OLD: // set abort | fallback. The provider-VALUE carve-out (unsupported-but-recognised
// provider names) is a SEPARATE Phase 6 task and is NOT done here.
  NEW: // set abort | fallback.
