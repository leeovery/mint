# Consolidation Findings: token-aware-diff-sizing (Phase 1)

## Findings

### F1: One rule for composing a failed AI call's captured output, on ai.GenerationError
- **Class**: duplication
- **Failure**: Release and commit each have their own copy of the rule that turns a failed AI call's two captured streams into the text shown beneath mint's line. Under that rule a stream counts only if it is non-blank after trimming; stdout comes before stderr, the two are joined by one "\n", and trailing whitespace is trimmed. The two copies are byte-identical today. Nothing ties them together:
  - `internal/engine/notesfailureoutput_internal_test.go` pins release's copy.
  - `internal/commit/run_aifail_output_test.go` pins commit's copy.
  - Each passes alone, and there is no compile-time link.

  Later phases rework the ✗ line around `notesFailureOutput` (phases 2, 4, 5, 6 and 7 all route captured output through it). If release's rule changes there (stream order, separator, blank-stream handling), commit keeps the old rule with no error and no failing test. A commit user then sees the same tool failure laid out differently from release's ✗ line. The case that matters most is a claude too old for the lean flags: it fails every call, and only its own output says why. Nobody notices until someone compares the two verbs' output. Commit's doc comment says the two rules match, and nothing checks that claim, so it turns false on the first divergence.
- **Evidence**:
  - `internal/engine/release.go:1640-1673`: `notesFailureOutput`, the composition rule and its doc. Its callers are `surfaceAndUnwind` at `:1086` and `surface` at `:1689`. Its `unicode` import at `:37` is used only at `:1672`.
  - `internal/commit/run.go:797-812`: `aiFailureOutput`, the same rule written as a loop. Its doc at `:797-799` names `notesFailureOutput` as the rule it follows. Its callers are `Run` at `:393` and `reviewLoop` at `:678`. Its `unicode` import at `:33` is used only at `:811`.
  - `internal/ai/transport.go:44-57`: `GenerationError`. Its doc at `:53-56` says composing the streams "is a downstream concern, so the transport keeps them separate and un-merged here".
  - `internal/ai/transport.go:231-232`: `Generate`'s inline comment, "the transport never trims — composition is a downstream concern".
- **Proposed shape**:
  - Add one method beside the carrier in `internal/ai/transport.go`, for example `func (e *GenerationError) Output() string`. It holds the composition rule, and the rule's doc moves there from `release.go:1650-1655`. The `Stdout`/`Stderr` fields stay raw and untrimmed, so later consumers that need the separate streams still get them.
  - `notesFailureOutput` and `aiFailureOutput` each shrink to `errors.As` plus `genErr.Output()`. `notesFailureOutput` keeps its name, so `notesfailureoutput_internal_test.go` stays green unchanged.
  - Optionally, one `ai` helper that takes an `error` could replace both `errors.As` wrappers.
  - The `unicode` imports in `run.go` and `release.go` go with the copies.
  - Three comments change:
    - `GenerationError`'s doc (`transport.go:53-56`) and the `Generate` inline comment (`:231-232`): the fields are carried raw and `Output` composes them.
    - `aiFailureOutput`'s cross-reference to `notesFailureOutput` is removed.
  - Behaviour is unchanged on both verbs, and the existing tests stay green.
- **Bank**: four entries (two from the executor, two from the reviewer, all from task 1-5) describe this same duplication. Each asks for one method on `ai.GenerationError` that both commit and release call. All four are confirmed against the final state: both copies are still present and still identical, and no test ties them together.

## Comment Corrections

- internal/commit/surface.go:19-25: the phase changed "stderr" to "output" here because the editor fallback now passes the AI's captured output (`run.go:479`), but the comment still lists only the stage/commit mutation failures as callers and gives git-diagnostic reasons, which no longer cover every use. It also repeats what the push path does elsewhere.
  OLD: // surfaceOutput is surface with the failed command's captured output passed through
// VERBATIM as StageFailure.Output. The mutation failures use it (stage/commit) because
// git's own diagnostics — a pre-commit hook's rejection output above all — are the only
// actionable explanation of WHY the mutation failed; the bare exit-status message alone
// renders as an unexplained failure. An empty output degrades to exactly surface's
// rendering, so callers pass whatever output they captured unconditionally. (The push
// path narrates through Warn with the same verbatim-Output convention.)
  NEW: // surfaceOutput is surface with the failed command's captured output passed through
// VERBATIM as StageFailure.Output. An empty output degrades to exactly surface's
// rendering, so callers pass whatever output they captured unconditionally.
