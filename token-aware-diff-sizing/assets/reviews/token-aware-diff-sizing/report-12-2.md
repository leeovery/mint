TASK: The unwind's reset, restore and tag-delete warnings carry git's stderr (token-aware-diff-sizing-12-2)

ACCEPTANCE CRITERIA:
- A release fails before the push; the unwind's `git reset --hard <start>` fails with git writing `fatal: Unable to create '.git/index.lock': File exists.`; the run warns "unwind incomplete", names the manual reset, and that stderr sits verbatim beneath the warning as its Output (§6.6)
- The same failed reset in a run whose exclusion answers were committed warns once, naming the reset then the `git cherry-pick` restore step; its Output is git's stderr (pins at release_answersunwindfailure_test.go:251 and :281 move off `exit status 1`) (§6.6)
- A failed `git tag -d <tag>` leaves git's stderr beneath the warning naming the manual tag delete (§6.6)
- With a `--notes-file` path kept out, a failed `git reset --soft <start>` leaves git's stderr beneath the warning naming the soft reset and restore; a failed `git restore --source=<start> --staged --worktree …` after a successful soft reset leaves the restore's stderr beneath the warning naming the restore (§13.2)
- A `mint release regenerate` whose changelog push fails and whose reset then fails leaves git's stderr beneath the warning ending "to drop the leftover changelog commit" (§13.2)
- A failed git step that wrote no stderr shows the error's text beneath its warning (§6.6)
- Unchanged: a failed unstage of `.mint.toml` and a failed answers re-commit still show git's stderr; a failed removal/rewrite of `.mint.toml`'s content still shows the filesystem error's text; every warning's Message and every Unwound summary reads as today; every other test stays green and all five gates pass

STATUS: complete

SPEC CONTEXT: §6.6 (Recording the answers) requires the unwind's failed re-commit, reset or tag delete to name its manual step "with git's own stderr verbatim beneath it". §13.2 makes the forward unwind's reset and regenerate's changelog reset leave a `--notes-file` path untouched, which is why resetTo has the soft-reset-then-restore form. CLAUDE.md's error idiom says subprocess diagnostics (git stderr) travel verbatim in `Warning.Output`.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/release.go:1175-1180 — `failedGitOutput(stderr string, err error) string`, the single rule: stderr if non-empty, else `err.Error()`
  - internal/engine/release.go:1165-1171 — `gitOutput` delegates to `failedGitOutput` for the `*record.GitError` carrier; its three callers (release.go:1105, :1111, :1146) are unchanged
  - internal/engine/unwind.go:183-184 — `deleteTagIfMade` keeps the Mutate Result and passes `failedGitOutput(res.Stderr, err)`
  - internal/engine/unwind.go:201-206 — `resetCommitsIfMade` passes resetTo's output
  - internal/engine/unwind.go:251-258 — `unstageEdit`'s inline copy replaced by the helper
  - internal/engine/unwind.go:260-274 — `restoreEditContent` keeps `err.Error()` for its two filesystem steps
  - internal/engine/unwind.go:291-311 — `resetTo` returns `(manual, output string, err error)`; each of the hard reset (:294-295), soft reset (:304-305) and restore (:307-308) builds output through the helper
  - internal/engine/regenerate_write.go:397-398 — `resetAndAbort` passes resetTo's output
- Notes: The Mutator returns the Result alongside the error on both the non-lock path and the exhausted-retry path (internal/git/mutator.go:165, :177), so `res.Stderr` is populated where git wrote any. Every Message string is byte-unchanged in the task commit (720351a); only the Output argument moved. No other inline copy of the rule remains in non-test engine code (the only other `.Stderr` read, release.go:1994, is the hook-output path and is not part of this rule).

TESTS:
- Status: Adequate
- Coverage:
  - internal/engine/unwind_gitoutput_test.go:41-60 — the lock case: a `git reset --hard` failing with index.lock stderr on every attempt of a 3-attempt Mutator budget; asserts the single warning is the manual reset with the lock stderr verbatim as Output, 3 reset attempts, and the "manual cleanup required" summary (AC1)
  - internal/engine/unwind_gitoutput_test.go:62-144 — hard reset, tag delete, keep-out soft reset and keep-out restore, each crossed with a with-stderr failure (`gitFailing`) and a no-stderr failure (`scriptedMutateFailure`), asserting the full Warning (Label, Message, Output) and the Unwound summary (AC3, AC4, AC6)
  - internal/engine/unwind_gitoutput_test.go:146-204 — regenerate's changelog reset: hard reset with and without stderr, keep-out soft reset with stderr, each asserting the full warning ending "to drop the leftover changelog commit" (AC5, AC6)
  - internal/engine/release_answersunwindfailure_test.go:251 and :281 — now pin `Output: resetStderr`, through `assertUnwoundWithOneWarning`, which checks exactly one warning (AC2)
  - The unchanged behaviours stay pinned by existing tests: re-commit stderr at release_answersunwindfailure_test.go:115; unstage stderr in release_answersfailure_test.go (`unstageStderr` at :272, :300, :329) (AC7)
- Notes: Each new assertion compares the whole `presenter.Warning`, so it would fail on a reverted Output and also on any drift in Message. No existing test still pins `exit status` or the no-stderr text as an unwind Output, so nothing left in the suite contradicts the change.

CODE QUALITY:
- Project conventions: Followed. Mutations still go through `git.Mutator`, output still goes through `presenter.Warn`, and git stderr travels verbatim in `Warning.Output` as CLAUDE.md requires
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes
- Readability: Good. The changed comments (failedGitOutput, gitOutput, resetTo, warnUnwindIncomplete) match the code
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "Every other existing test stays green, and all five gates in CLAUDE.md pass" — run `go build ./...`, `gofmt -l .`, `go vet ./...`, `go test -race ./...` and `golangci-lint run` on the delivered tree
