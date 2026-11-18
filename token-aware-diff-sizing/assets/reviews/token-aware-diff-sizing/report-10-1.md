TASK: Quote the keep-out restore step with `cmdline.Quote` (token-aware-diff-sizing-10-1, tick-3e30c1)

ACCEPTANCE CRITERIA:
- A release run with `--notes-file notes/what's-new.md` fails before the push and the unwind's soft reset fails too; the warning reads ``automatic recovery failed; manually reset HEAD back to {sha}, leaving notes/what's-new.md as it is (`git reset --soft {sha}` then `git restore --source={sha} --staged --worktree -- ':(top,literal,exclude)notes/what'\''s-new.md'`)``, the pathspec written the way `cmdline.Quote` writes it (§12.4, §13.2)
- The same release, soft reset succeeds and restore fails, warns ``automatic recovery failed; manually restore the index and work tree to {sha}, leaving notes/what's-new.md as it is (`git restore --source={sha} --staged --worktree -- ':(top,literal,exclude)notes/what'\''s-new.md'`)`` (§12.4, §13.2)
- A regenerate to a changelog target with `--notes-file notes/what's-new.md`, whose reset after its changelog commit fails, warns the same reset step with the same quoted pathspec, followed by ` to drop the leftover changelog commit` (§13.2)
- For `notes/'draft'.md`, reading the printed restore step back by POSIX shell rules (`cmdline.Split`) gives `:(top,literal,exclude)notes/'draft'.md` as one whole word, never naming `notes/draft.md` (§13.2)
- The prose `leaving {path} as it is` names the notes file raw, a `'` in it included (§12.4)
- A notes file with no `'` (`NEXT_RELEASE.md`, `notes.md`, a name with a space) gets manual steps byte-identical to today's; the pinned expectations in internal/engine/notesfile_reset_test.go stand unchanged (§12.4)

STATUS: complete

SPEC CONTEXT: §13.2 requires a reset during an unwind (the release's surgical unwind, or regenerate's reset after its changelog commit) to leave the `--notes-file` file as it was. When that reset fails, the manual step mint prints is the operator's only recovery path. §12.4, as phase 5 settled it and phase 8 extended it to the unwind's steps, makes every printed command paste-ready for a POSIX shell, so its shell words go through `cmdline.Quote`. Prose that only names a file for display stays raw.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/engine/unwind.go:302 (`restoreLine` now writes `pathspec[0] + " " + cmdline.Quote(pathspec[1])`). The `keeping` prose at internal/engine/unwind.go:303 is still `string(target.KeepOut)`, raw. Both manual steps embed `restoreLine` (internal/engine/unwind.go:305, :308). There are two callers of `resetTo`: `resetCommitsIfMade` (internal/engine/unwind.go:201) and `resetAndAbort` (internal/engine/regenerate_write.go:397, which appends " to drop the leftover changelog commit" at :398).
- Notes: The change is the one line the task prescribed (commit c3a3bd0). `git.Exclusion.Pathspec()` (internal/git/exclusion.go:14-19) always returns `["--", ":(top,literal,exclude)"+path]`. Because of the `(`, `Quote` always wraps the second word, so for any path without a `'` the printed output is byte-identical to the old hand-written quotes. I re-ran the task's ripgrep pattern over every `*.go` file. It now finds only internal/cmdline/quote.go:17 (`Quote`'s own body) and the five expected strings in internal/engine/notesfile_quoting_test.go (:45, :63, :89, :109, :125). No hand-quoted printed step is left in production code.

TESTS:
- Status: Adequate
- Coverage:
  - Criteria 1, 2 and 5: `TestUnwind_KeepOut_ResetStepFails_QuotesThePathspecForTheShell` (internal/engine/notesfile_reset_test.go:343). For each of `notes/what's-new.md`, `notes/'draft'.md` and `release notes.md`, it checks the exact full Warn message in both failure modes (soft reset fails, and restore fails). The expected prose uses the raw `tt.file`.
  - Criterion 3: `TestRegenerateWrite_KeepOut_RecoveryResetFails_QuotesThePathspecForTheShell` (:428) checks the full regenerate message, including the `'\''` escape and the " to drop the leftover changelog commit" suffix.
  - Criterion 4: `TestUnwind_KeepOut_RestoreStepSplitsBackToTheKeepOutPathspec` (:400) runs the backticked restore step through `cmdline.Split` and checks the exact argv, with the pathspec as one word. Under the old hand-written quoting this split would give `:(top,literal,exclude)notes/draft.md`, so the test fails if the fix regresses.
  - Criterion 6: the existing pins `TestUnwind_KeepOut_ResetStepFails_WarnsTheManualStepsLeft` (:266, expected string at :271) and `TestRegenerateWrite_KeepOut_RecoveryResetFails_WarnsTheManualStepsLeft` (:313, expected string at :332-334) are unchanged in commit c3a3bd0. They sit 3 lines lower than the task's :268/:330 only because of the new imports. The space row in the new table covers the space case.
- Notes: The criteria call for a "release run with --notes-file". The tests instead drive `engine.Unwind` / `engine.RegenerateWrite` with a `git.Exclusion` keep-out, which is the shape the task's Do section names for the existing pins. Wiring from `--notes-file` to `KeepOut` is covered by the `TestRelease_NotesFileInRepo_*` tests in the same file. The tests are not over-tested: each new test covers a different criterion.

CODE QUALITY:
- Project conventions: Followed (it reuses `cmdline.Quote`, matching `unstageEdit` at internal/engine/unwind.go:256; tests are external-package, use `t.Parallel()`, script the FakeRunner, and check exact rendered lines)
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes (`slices.Equal` in the split-back test)
- Readability: Good
- Issues: None. The `resetTo` doc comment (internal/engine/unwind.go:283-290) still holds after the change.

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
