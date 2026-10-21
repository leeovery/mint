# Analysis Report: Token Aware Diff Sizing (Cycle 1)

## Stats

- Total findings: 4
- Deduplicated findings: 4
- Proposed tasks: 3

## Summary

After eight consolidation passes, the three agents found no overlapping findings. Each of the four findings stands alone, and the code matches the specification and its corrigenda on every decision the standards agent checked. Three findings become proposals. Release's new cmd-to-engine wiring (`Yes`, `StdinInteractive`, `ChunkNotes`) has no test seam, though regenerate's does. The setup guide's transport paragraph hand-types three shipped defaults that config owns. CLAUDE.md's unwind invariant misses the ignored-`.mint.toml` keep, and that one-sentence fix goes into a Corrections proposal. The fourth finding asks to make the regenerate review gate's run state required. That would reverse phase 6's settled direction with no new grounds, so it is discarded. Four stale comments are collected below for direct application.

## Comment Corrections

- internal/engine/release.go:1999-2007 — the function no longer composes anything (`ai.GenerationError.Output` composes the streams), and the remainder is reasoning about why it is not a copy of `hookFailureOutput`, which the code does not need
  OLD: // notesFailureOutput extracts claude's captured output from a *ai.GenerationError
// carrier and composes it for StageFailure.Output. It mirrors hookFailureOutput's
// PATTERN — a typed-carrier extraction — but NOT its field choice: the hook helper reads
// Result.Stderr, whereas claude writes its payload (e.g. "Prompt is too long") to STDOUT,
// so a literal copy of the hook helper would render nothing. errors.As is used precisely
// because it traverses the %w chain, so the carrier still matches when it sits inside
// abortError's forward chain ("notes generation failed (%s): %w") OR regenerate's shorter
// "generating notes: %w" chain. Any non-carrier cause (timeout, command-missing, or a
// plain error) yields "" so the ✗ line stands alone.
  NEW:
- internal/config/config.go:56-59 — "abort … fails loud and tags nothing" is false now that an attended release under abort is offered the floor's choice and can still tag; the closed-set sentence repeats onNotesFailureValues' comment
  OLD: // defaultOnNotesFailure is the out-of-the-box notes-failure policy: "abort" — when
// the normal AI path fails, mint fails loud and tags nothing. The opt-in alternative
// is "fallback". These two are the closed valid set: an absent key defaults to abort
// and any other non-empty value is rejected by Load (see validateOnNotesFailure).
  NEW: // defaultOnNotesFailure is the notes-failure policy an absent on_notes_failure key
// resolves to.
- internal/engine/regenerate_batch.go:184-186 — single-version regenerate has no "on_notes_failure=abort default" to override, since regenerate never reads on_notes_failure and an attended single-version run gets the floor's choice
  OLD: // per version). A per-version FAILURE is CAUGHT, RECORDED with a human reason, and the
// loop CONTINUES — consciously OVERRIDING the single-version on_notes_failure=abort
// default (one version whose notes fail must not kill the others).
  NEW: // per version). A per-version FAILURE is CAUGHT, RECORDED with a human reason, and the
// loop CONTINUES, so one version whose notes fail does not stop the others.
- internal/engine/regenerate_batch.go:268-270 — the same false claim that single-version regenerate has an on_notes_failure=abort default
  OLD: // Per-version FAILURES are caught here and returned as a *skippedVersion rather than
// surfaced as a terminal abort: this is the skip-and-continue contract that overrides
// the single-version on_notes_failure=abort default. The covered failures are:
  NEW: // Per-version FAILURES are caught here and returned as a *skippedVersion rather than
// surfaced as a terminal abort: skip-and-continue is the batch's floor. The covered failures are:

## Discarded Findings

- The regenerate review gate's run state is still optional (architecture, low). This would reverse a settled direction without grounds. Phase 6's approved Task 2 settled that `RegenerateWriteRequest` carries the `regenStage` and keeper. Its criterion also pins that a direct `RegenerateWrite` with neither runs the gate as built: a failed `r` saves nothing and names no file, and no hint follows. Deleting `reviewFresh`'s nil branch and turning the state into required parameters undoes both. The finding names the same failure consolidation-findings-p6 F3 named, which the walk already settled. It shows no measured defect: both production paths, `RegenerateRun` and `gatePerVersion`, always supply the state. It cites no spec entry and no project rule.
- Injecting the chunk-notes store into `regenerate(...)` (part of the architecture finding on release's wiring). It names no failure. No cmd test reaches a failed chunked run through that seam, so the hard-wired `notescache.NewUserChunkNotes(os.UserCacheDir)` writes nothing from the suite today, and matching release's seam is not a finding on its own. The release half of the finding is proposed as Task 1.
