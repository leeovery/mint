TASK: Quote the repo root and path in the unstage manual step (token-aware-diff-sizing-8-10, tick-8c4b48)

ACCEPTANCE CRITERIA:
- In a repo whose root contains a space (`…/My App`), the answers commit fails and the unwind's unstage of `.mint.toml` fails too: the `unwind incomplete` warning reads ``automatic recovery failed; manually unstage .mint.toml (`git -C '…/My App' reset -q {sha} -- .mint.toml`)``, the root single-quoted as `cmdline.Quote` writes it, with git's stderr verbatim beneath it (§6.6).
- In a root made only of `[A-Za-z0-9_@%+=:,./-]`, as every `t.TempDir` root is, the step reads as today with the root bare: the warnings pinned at `internal/engine/release_answersfailure_test.go:278-282` and `:309-313` hold unchanged (§6.6).

STATUS: complete

SPEC CONTEXT: §6.6 "Recording the answers": a failure writing, staging or committing the exclusion answers is a pre-push failure routed through the unwind, which leaves `.mint.toml` as it was before the answers. When an unwind step itself fails, one warning names the manual step that finishes the recovery, with git's own stderr verbatim beneath it. The task makes that manual step copy-paste-safe in a POSIX shell for repo roots that need quoting, matching the answers re-commit step, which already quotes the root.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/engine/unwind.go:256 (`unstageEdit` builds the step from `cmdline.Quote(edit.root)` and `cmdline.Quote(edit.path)`); import at internal/engine/unwind.go:41; the executed argv at internal/engine/unwind.go:252 still passes the raw root and path, which is correct because argv is never shell-parsed. It follows the pattern of `recommit` at internal/engine/release.go:1143-1145.
- Notes: `cmdline.Quote` (internal/cmdline/quote.go:10) returns a word made only of `[A-Za-z0-9_@%+=:,./-]` unchanged, so criterion 2 holds by construction. The stderr beneath the step travels through `failedGitOutput` (internal/engine/release.go:1175), so it is verbatim. The other printed unwind steps that the task's measured set names (tag delete, hard reset, cherry-pick restore) carry no repo root. `restoreEditContent`'s "remove {full}, which mint created" is a prose path, not a command. No root-bearing step is left unquoted. The task's commit (ed9523c) changes exactly the one site that the measured set identifies.

TESTS:
- Status: Adequate
- Coverage: The new test `TestRelease_AnswersCommitFailing_WhenUnstagingFailsInARootWithASpace_WarnsTheStepWithTheRootQuoted` (internal/engine/release_answersfailure_test.go:297-324) runs the answers-commit failure in `{TempDir}/My App` with the unstage failing. It pins the exact warning, with the root wrapped in literal single quotes, and the stderr as Output. It would fail if the root went back to bare. It hard-codes the quoting rather than calling `cmdline.Quote`, so it checks the format independently of the helper. A t.TempDir base contains no single quote, so the literal `'…'` form is exact. Criterion 2's two pinned warnings, now at release_answersfailure_test.go:284-291 and :344-351 after the insertion shifted them, still expect the bare `run.root` and are unchanged. The refactor of `recordAnswersFailing` into a delegating `recordAnswersFailingIn` (lines 57-91) leaves every existing caller's behaviour as it was.
- Notes: `edit.path` is always `config.FileName` (`.mint.toml`), which needs no quoting, so path quoting cannot be observed here. That is a property of the input, not a gap.

CODE QUALITY:
- Project conventions: Followed. The output goes through the Presenter `Warn`, the mutation through `git.Mutator`, git's stderr travels verbatim in `Warning.Output`, and the test is an external `engine_test` package with `t.Parallel()`, a `FakeRunner` and an exact pinned line.
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
