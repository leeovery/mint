# Analysis Tasks: Token Aware Diff Sizing (Cycle 4)

## Task 1: Run the notes diff and Change Map at the repo root, so an Exclude answer or a keep-list glob covers what the check counted
severity: medium
sources: architecture

**Problem**: The exclusion check matches globs in Go, root-relative. `pathspecMatches` (internal/notes/pathspec.go:9-15) is checked against git's diff headers, which are always root-relative. Three pieces of the check depend on it: `usableSuggestions`, a pause's `matches` count, and `diff_keep`'s `coveredBy` (internal/notes/exclusioncheck.go:228-263). The notes Assembler, though, runs its diff (internal/notes/assemble.go:186-187) and both Change Map reads (internal/notes/changemap.go:48-57) with a plain `Run` from the process cwd, over `-- .` and `:(exclude)` pathspecs. From a subdirectory, git reads every one of those pathspecs relative to the cwd.

Take an operator who runs `mint release` at a terminal from `sub/`:
- The check lists `sub/tests/t_test.go`.
- The AI suggests `sub/tests/`, and the pause counts the file under it.
- The operator answers Exclude. The glob is written to `[release].diff_exclude` and committed.
- `checkExclusions` (internal/notes/select.go:249-259) re-assembles with `:(exclude)sub/tests/`. git reads that as `sub/sub/tests/`, so it matches nothing.

The re-assembled diff, the Change Map, every chunk call and any `r` still carry the test files. The notes ship describing files the operator chose to leave out, and the operator learns this only by reading them, at the review gate or after the release is out. The architecture agent reproduced it in a scratch repo: run from `sub/`, `git diff --name-only v1..HEAD -- . ':(exclude)sub/tests/'` still lists the file, while `':(exclude)tests/'` drops it. A `diff_keep` entry and a `diff_exclude` entry likewise disagree from a subdirectory about which paths they cover. The same cwd also cuts a subdirectory run's notes down to that subtree, and leaves out the rest of the release.

CLAUDE.md seam 1 forbids this case by name: "Read-side git that uses cwd-relative pathspecs (`-- .`, `ls-files`) must run at the repo root via `RunInDir(root, …)`". The feature's other git calls already follow it:
- `git.Exclusion` is root-anchored (internal/git/exclusion.go:13-19);
- the answers commit, `IsIgnored` and `notesExclusion` run `git -C root`;
- commit's diff source uses `RunInDir(g.root)` (internal/commit/generate.go:185-192, :244).

**Solution**: Give the Assembler the repo root: `NewAssembler(r, root, exclude)`. Both production sites already have `root` in scope: `resolveBody` (internal/engine/release.go:1016) and `freshGenerator` (internal/engine/regenerate_fresh.go:121).

`AssembleRange`'s diff and `BuildChangeMapForRange`'s `--name-status` and `--numstat` reads then run through `RunInDir(ctx, a.root, nil, "git", …)`, as commit's diff source does. Their argv stay byte-identical, so the existing exact-argv pins hold. `-- .` then covers the whole tree, and git reads every `:(exclude)` glob root-relative: the built-ins, both `diff_exclude` levels and the Exclude answers.

Two comments say these calls run "cwd-relative like the other engine git calls" (assemble.go:150, changemap.go:17). Both are rewritten to say the calls run at the repo root.

The tests assert the recorded `Dir` on the FakeRunner's diff and Change Map invocations, and pin that an Exclude answer's re-assembly reaches git at the root.

Derivation:
- CLAUDE.md seam 1 above.
- §5.1: "Entries at every level are git pathspec globs, and git does the matching." `.mint.toml` sits at the root, and the check records its globs root-relative.
- §5.1: "Release notes are a public list of what a release holds." The tag covers the whole tree, and the commit-subject list (`git log --format=%s`, internal/notes/resolve.go:22, no pathspec) already covers all of it.

**Outcome**: From any directory in the repo, release and regenerate notes are built over the whole tree. An Exclude answer takes out exactly the files its pause counted, and `diff_keep` covers the same paths `diff_exclude` would. A subdirectory run's notes no longer stop at that subtree.

## Task 2: The unwind's reset, restore and tag-delete warnings carry git's stderr
severity: low
sources: duplication

**Problem**: Under a failed git step, the output should be git's stderr, or the error's text when git wrote none. That rule lives in two places: `gitOutput` (internal/engine/release.go:1164), which reads the `*record.GitError` carrier, and an inline copy in `unstageEdit` (internal/engine/unwind.go:256-259). Every other unwind step discards the Mutate `Result` (`_, err :=`) and passes `err.Error()`. That text is only the runner's `running "git": exited with code N: exit status N` (internal/runner/exec_runner.go:114), since stderr stays in the Result. The steps that do this:
- `resetTo`'s hard reset, soft reset and keep-out restore (unwind.go:294-313), reached from `resetCommitsIfMade` (unwind.go:201-206) and regenerate's `resetAndAbort` (internal/engine/regenerate_write.go:397-398);
- `deleteTagIfMade` (unwind.go:183-184).

Take an operator whose release fails before the push, and whose unwind reset then fails too because an editor holds `.git/index.lock` past the Mutator's retries. The "unwind incomplete" warning names the manual reset. Beneath it, the operator reads `exit status 128` instead of `fatal: Unable to create '.git/index.lock': File exists.`, which is the reason the pasted step will fail again. In the same unwind, a failed unstage or re-commit of the answers shows git's stderr. The diagnosis is lost on the step that leaves mint's commits on the branch, for the operator or an agent reading `-y` output.

The tests pin both behaviours. internal/engine/release_answersunwindfailure_test.go:243 and :273 seed the failed reset with `resetStderr` and then assert `Output: "exit status 1"` (:251, :281). The same file asserts git's stderr for the failed re-commit at :115.

**Solution**: One engine helper holds the rule: git's stderr when it wrote any, the error's text otherwise. `gitOutput` delegates to it for the `*record.GitError` carrier, and `unstageEdit` calls it in place of its inline copy.

`resetTo` keeps each Mutate's `Result` and returns `(manual, output string, err error)`, the shape `answersCommit.recommit` already returns, with the failing step's output built by the helper. `resetCommitsIfMade` and `resetAndAbort` pass that output to `warnUnwindIncomplete`. `deleteTagIfMade` keeps its `Result` and routes it the same way. The two filesystem steps in `restoreEditContent` (unwind.go:268, :274) keep `err.Error()`, since no subprocess is involved.

Tests:
- The pins at release_answersunwindfailure_test.go:251 and :281 move to `resetStderr`.
- New pins seed stderr for a failed tag delete, a failed keep-out soft reset, a failed keep-out restore, and regenerate's failed changelog reset, and assert it verbatim beneath each warning.
- A failed step that wrote no stderr, such as `scriptedMutateFailure()`, still shows the error's text.

Derivation:
- CLAUDE.md: "Failure diagnostics from subprocesses (hook output, git stderr) travel VERBATIM in `StageFailure.Output`/`Warning.Output`".
- `warnUnwindIncomplete`'s own contract (unwind.go:316-319): "the failing step's error rides the Warn's Output verbatim".

**Outcome**: Every git step the release unwind and regenerate's changelog reset run leaves git's own stderr beneath its "unwind incomplete" warning. Where git wrote none, the error's text appears instead. The rule sits in one helper.
