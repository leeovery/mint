## Attempt 1

ISSUES:
- `/Users/leeovery/Code/mint/internal/notes/select.go:207-215`: the check call runs before `ResolveInstructions`. Before this task, a release whose `[release].prompt` names a missing file (or whose `context` file fails to stat) failed with no AI call. Now it makes the check call first, which can run up to the release timeout and may be retried, and only then fails on a local file read. Once task 7-7 lands, the same run will also pause the operator for each suggestion and commit their answers, and §6.6 keeps that commit through the unwind. All of that would happen before a failure that was knowable before any AI call. The instructions do not depend on the diff, and the check's exclusions never change them, so nothing needs them resolved after the check.
  FIX: move the `instructions, err := ResolveInstructions(s.root, cfg.Release)` block (and its error return) above the `if s.check != nil { … }` block. Add a test in `/Users/leeovery/Code/mint/internal/notes/exclusioncheck_test.go` next to `TestSelector_ExclusionCheckCancelled_ReturnsTheCancelBeforeAnyNotesCall`: give `[release].prompt` a file that does not exist under `root`, then assert that `SelectBody` returns the error and the transport recorded zero calls.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `/Users/leeovery/Code/mint/internal/engine/release_realruncache_test.go:63`: the diff edited this comment, but it still says the AI is skipped on the reuse path, and the exclusion check's AI call is now made there.
  OLD: the AI is skipped), then the record/tag/push mutation tail.
  NEW: the notes call is skipped), then the record/tag/push mutation tail.

NOTES:
- `/Users/leeovery/Code/mint/internal/notes/exclusioncheck.go` `fileChange` handles `new file mode`, `deleted file mode` and `rename from`, but not `copy from`. `headerPath` (`/Users/leeovery/Code/mint/internal/notes/chunk.go:300`) keys a copy to its destination, so under `diff.renames = copies` a copied file would be listed as `modified`. Default git config does not detect copies, so this is minor.
- The check still makes its call when the keep list covers every path, with an empty list in the prompt. The task assigns that skip to 7-6, so this is in scope as delivered.
- `/Users/leeovery/Code/mint/internal/engine/release_realruncache_test.go:110` ("The AI was NEVER invoked — reuse skipped it.") is also stale now, since the check call is made. It sits as unchanged context beside an edited line, so it is outside the correction scope, but it is worth fixing in the same pass.
- Two edited comments have awkward line wrapping but are not wrong: the stream-discipline note in `/Users/leeovery/Code/mint/internal/presenter/presenter.go:71-73` (a short "the completion event" line) and the `SelectBodyWithReuse` paragraph in `/Users/leeovery/Code/mint/internal/engine/release.go:1004-1005` (one over-long line).
- Nothing came with the dispatch beyond the enumerated inputs.
