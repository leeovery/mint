# Analysis Tasks: Token Aware Diff Sizing (Cycle 4)

## Task 1: Run the notes diff and Change Map at the repo root, so an Exclude answer or a keep-list glob covers what the check counted
severity: medium
sources: architecture

**Problem**: The exclusion check matches globs in Go, root-relative. `pathspecMatches` (internal/notes/pathspec.go:9-15) is checked against git's diff headers, which are always root-relative. Three pieces of the check depend on it: `usableSuggestions`, a pause's `matches` count, and `diff_keep`'s `coveredBy` (internal/notes/exclusioncheck.go:228-263). The notes Assembler, though, runs its diff (internal/notes/assemble.go:185-186) and both Change Map reads (internal/notes/changemap.go:48-57) with a plain `Run` from the process cwd, over `-- .` and `:(exclude)` pathspecs. From a subdirectory, git reads every one of those pathspecs relative to the cwd.

Take an operator who runs `mint release` at a terminal from `sub/`:
- The check lists `sub/tests/t_test.go`.
- The AI suggests `sub/tests/`, and the pause counts the file under it.
- The operator answers Exclude. The glob is written to `[release].diff_exclude` and committed.
- `checkExclusions` (internal/notes/select.go:249-259) re-assembles with `:(exclude)sub/tests/`. git reads that as `sub/sub/tests/`, so it matches nothing.

The re-assembled diff, the Change Map, every chunk call and any `r` still carry the test files. The notes ship describing files the operator chose to leave out, and the operator learns this only by reading them, at the review gate or after the release is out. The architecture agent reproduced it in a scratch repo: run from `sub/`, `git diff --name-only v1..HEAD -- . ':(exclude)sub/tests/'` still lists the file, while `':(exclude)tests/'` drops it. A `diff_keep` entry and a `diff_exclude` entry likewise disagree from a subdirectory about which paths they cover. The same cwd also cuts a subdirectory run's notes down to that subtree, and leaves out the rest of the release. Regenerate's fresh source builds the same Assembler (internal/engine/regenerate_fresh.go:121), so a `mint release regenerate` from a subdirectory reads the recorded answers cwd-relative too, where §6.1 has answers given on forward releases carry into regenerated notes.

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

**Acceptance Criteria**:
- [ ] Take a prior-tag `mint release` in a repo whose root is R. Its notes diff (`git diff --src-prefix=a/ --dst-prefix=b/ {lastTag}..HEAD -- . {excludes}`) and both Change Map reads (`git diff --name-status …` and `git diff --numstat …`, over the same range and pathspecs) run with R as their working directory, whatever directory mint was started from. Their argv are byte-identical to today's (§5.1, §8.2)
- [ ] `mint release regenerate <version>` from the fresh source runs its `{PreviousTag}..{Tag}` diff and both Change Map reads at R the same way, with today's argv. So does an `r` at its review gate (§5.1, §5.3, §6.1)
- [ ] At a terminal, the exclusion check suggests `sub/tests/` and the operator answers Exclude. The re-assembled notes diff and the Change Map built after it carry `:(exclude)sub/tests/` and run at R, so git reads the glob against the same root-relative paths the pause counted. An `r` at the review gate re-reads the diff at R with the glob still excluded (§5.1, §6.5)
- [ ] Every existing exact-argv pin on the notes diff and Change Map, in internal/notes and internal/engine, holds unchanged. All five gates in CLAUDE.md pass

**Do**:
- internal/notes/assemble.go: the constructor becomes `NewAssembler(r, root, exclude)` (:102) and the Assembler holds the root. `AssembleRange`'s diff (:185-186) runs through `a.runner.RunInDir(ctx, a.root, nil, "git", args...)`.
- internal/notes/changemap.go: `BuildChangeMapForRange`'s `--name-status` read (:51) and `--numstat` read (:57) run the same way.
- Those three are every git read the Assembler makes. `rg -n '\.Run\(ctx' internal/notes -g '!*_test.go'` returns 4 lines: the three, plus `rangeSubjects`' `git log --format=%s` (internal/notes/resolve.go:22). That one takes no pathspec and stays on `Run`.
- The model is commit's `diffSourceText` (internal/commit/generate.go:242-249).
- Production callers pass the `root` they already hold, the `git rev-parse --show-toplevel` root from `gitrepo.ResolveRoot` (internal/engine/release.go:361 for release, cmd/mint/main.go:158 for regenerate): `resolveBody` (internal/engine/release.go:1016) and `freshGenerator` (internal/engine/regenerate_fresh.go:121).
- The constructor change reaches every call site. `rg -n 'notes\.NewAssembler\(' internal` returns 94 lines across 21 files: the two production sites, 91 test sites across 18 internal/notes test files, and internal/engine/release_regen_sizecheck_test.go:52. Every one moves to the new signature.
- The two "cwd-relative like the other engine git calls" comments sit at internal/notes/assemble.go:149 and internal/notes/changemap.go:17. The Solution's :150 predates cycle 4's comment corrections.
- The tests read the working directory from `runner.Invocation.Dir`, which `FakeRunner.RunInDir` records (internal/runner/fake_runner.go:118-119). The engine's existing Exclude-answer pins (internal/engine/release_exclusionanswer_test.go, `notesDiffCalls` at :86) read argv only.

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

**Acceptance Criteria**:
- [ ] A release fails before the push. The unwind's `git reset --hard <start>` then fails, with git writing `fatal: Unable to create '.git/index.lock': File exists.` to stderr. The run warns "unwind incomplete" and names the manual reset, as today, and that stderr sits verbatim beneath the warning as its Output (§6.6)
- [ ] The same failed reset, in a run whose exclusion answers were committed, warns once. The warning names the reset and then the `git cherry-pick` step that restores the answers commit, as today, and its Output is git's stderr. Today's pins at release_answersunwindfailure_test.go:251 and :281 expect `exit status 1` here (§6.6)
- [ ] An unwind whose `git tag -d <tag>` fails leaves git's stderr beneath the warning that names the manual tag delete (§6.6)
- [ ] With a `--notes-file` path kept out of the reset, a failed `git reset --soft <start>` leaves git's stderr beneath the warning that names the soft reset and the restore. If the soft reset succeeds and the `git restore --source=<start> --staged --worktree …` after it fails, the restore's stderr sits beneath the warning that names the restore (§13.2)
- [ ] A `mint release regenerate` whose changelog push fails, and whose reset of the changelog commit then fails, leaves git's stderr beneath the warning ending "to drop the leftover changelog commit" (§13.2)
- [ ] A failed git step that wrote no stderr shows the error's text beneath its warning, as each of these steps does today (§6.6)
- [ ] These stay as they are: a failed unstage of `.mint.toml` and a failed answers re-commit still show git's stderr, and a failed removal or rewrite of `.mint.toml`'s content still shows the filesystem error's text. Every warning's Message and every Unwound summary reads as today. Every other existing test stays green, and all five gates in CLAUDE.md pass

**Do**:
- The rule's two copies today: `rg -n 'Stderr != ""|output == ""' internal/engine -g '!*_test.go'` returns 2 lines. One is `gitOutput` (internal/engine/release.go:1166), the other `unstageEdit`'s inline copy (internal/engine/unwind.go:257). `gitOutput` delegates to the helper for the `*record.GitError` carrier, and its three callers (release.go:1105, :1111, :1146) stay as they are. `unstageEdit` (unwind.go:256-259) calls the helper.
- The complete set of unwind warnings: `rg -n 'warnUnwindIncomplete\(' internal/engine -g '!*_test.go'` returns 8 lines, the definition (unwind.go:320) and 7 call sites:
  - unwind.go:146, `recommitAnswersIfKept`: already passes `recommit`'s output, built by `gitOutput`. Unchanged.
  - unwind.go:184, `deleteTagIfMade`: keeps the Mutate `Result` and passes the helper's output.
  - unwind.go:206, `resetCommitsIfMade`: passes `resetTo`'s output.
  - unwind.go:260, `unstageEdit`: the helper replaces the inline copy.
  - unwind.go:268 and :274, `restoreEditContent`: filesystem steps, keep `err.Error()`.
  - internal/engine/regenerate_write.go:398, `resetAndAbort`: passes `resetTo`'s output.
- `resetTo` (unwind.go:294-314) returns `(manual, output string, err error)`, the shape of `answersCommit.recommit` (release.go:1138). Each of its three Mutates keeps its `Result` and builds the output through the helper: the hard reset (:297), the soft reset (:307) and the restore (:310). `rg -n 'resetTo\(' internal/engine -g '!*_test.go'` returns 3 lines, the definition and its two callers (unwind.go:201, regenerate_write.go:397).
- Test seams already in the suite: `gitFailing(stderr)` (internal/engine/release_answersfailure_test.go:21) seeds a failure that carries stderr, and `scriptedMutateFailure()` (internal/engine/unwind_test.go:237-242) seeds one with none. The keep-out and regenerate reset failures are driven today in internal/engine/notesfile_reset_test.go (:266, :313), and the tag-delete failure in unwind_test.go (:292). All of these assert the Message only.
