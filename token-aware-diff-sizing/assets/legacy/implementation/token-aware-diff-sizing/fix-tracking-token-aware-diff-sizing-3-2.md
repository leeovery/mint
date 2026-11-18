## Attempt 1

ISSUES:
- /Users/leeovery/Code/mint/internal/record/commit.go:46 (artifact commit) and :91 (bookkeeping commit): adding `-- :(top,literal,exclude)<notes>` to `git commit` puts git into `--only` mode. It then commits the work-tree content of every tracked path the pathspec matches, which is every path except the notes file, instead of committing the index. I checked this on git 2.56 in scratch repos:
  (a) A file marked `git update-index --assume-unchanged` with local edits passes the clean-tree check today. Under the pathspec it lands in both the artifact commit and the bookkeeping commit, and its assume-unchanged bit is cleared (`git ls-files -v` goes from `h` to `H`). The current `git commit -m` takes neither. `git add -A -- <exclusion>` alone still respects the bit; only the commit pathspec sweeps it in.
  (b) An unstaged edit to a tracked file made after the clean-tree check rides the bookkeeping commit; today it stays an unstaged edit. Two ways this happens: a preflight hook (release.go runs it after the gate) rewrites a tracked lockfile and no pre_tag hook is set, or an operator edits files while the attended review gate waits.
  This fires in the headline case too. An untracked `notes.md`, or one with only unstaged edits, never rides an index commit, so the commit-level exclusion buys nothing there and only changes behaviour. These commits go out in the atomic push and are never unwound after it.
  FIX: Keep the exclusion on the status probes, the `add -A` and the stash push. Take it off both `git commit` calls. When keepOut is set, commit exactly the paths the current index commit would carry, minus the notes file:
    1. Read the paths: `m.Run(ctx, "git", withExclusion(keepOut, "-C", dir, "diff", "--cached", "--name-only", "-z", "--no-renames")...)`.
    2. If the list is empty, make no commit. Never run `commit` with empty stdin: I checked that an empty `--pathspec-from-file` commits the whole index, the notes file's staged edits included.
    3. Otherwise commit with `m.Mutate(ctx, list, "git", "--literal-pathspecs", "-C", dir, "commit", "-m", subject, "--pathspec-from-file=-", "--pathspec-file-nul")`. Passing the list as `[]byte` stdin means a lock retry re-pipes it in full (seam 2). `--literal-pathspecs` keeps glob characters literal.
  A zero exclusion keeps today's `git -C dir commit -m subject`. The `add -A` (artifact) or `add <paths>` (bookkeeping) has just synced every listed path, so `--only` reads the bytes the index already holds. An assume-unchanged path is never listed, because its index entry equals HEAD. I checked on git 2.56: the assume-unchanged edit and its bit stay out, the notes file stays `MM` (staged and unstaged edits both kept), and a hook's deleted file and a file named `odd [x]*.txt` are both committed.
  Tests: pin the diff read plus the commit argv and stdin in internal/record/commit_test.go and internal/engine/release_notesfile_git_test.go (lines 106, 253, 304), and add an empty-list case that makes no commit. Keep the zero-exclusion tests asserting the current argv. Needs git 2.25 or later for `commit --pathspec-from-file`. Rewrite the CommitDirtyTree doc sentence "The commit names the exclusion so it takes every other path…" (commit.go:30-31) to match the new mechanism.
  ALTERNATIVE: (a) Gate it: a commit only needs any exclusion when the notes file has staged edits (`git -C dir diff --cached --quiet -- :(top,literal)<path>` exits 1). The headline untracked case and unstaged-only edits then keep today's argv exactly, and the path list above is used only for the staged case. That is the smallest change in behaviour, at the cost of one more probe and a branch. (b) Commit through a temporary index (GIT_INDEX_FILE pointing at a copy of the index with the notes entry reset to HEAD). That keeps exact index behaviour even for a partially staged path, but needs env support on `Mutator.Mutate`, a seam change. I recommend the primary fix (one code path, checked against real git), or (a) if keeping the common case's argv unchanged matters more.
  CONFIDENCE: medium

- /Users/leeovery/Code/mint/internal/engine/release.go:840-845 `notesFilePath`: production always passes an empty WorkDir, so `filepath.Join("", p)` now cleans every relative path (checked: `./notes.md` becomes `notes.md`, `link/../notes.md` becomes `notes.md`, `a//b.md` becomes `a/b.md`). Two effects:
  - `readNotesFile` reads the cleaned path. When `link` is a symlinked directory, that is a different file from the one the OS would open.
  - The read error names the cleaned path instead of the P the operator typed. §13.4 fixes the message as `reading --notes-file P: <cause>`, and task 3-5 will pin it.
  Before this task, the raw flag value was read.
  FIX: Return `opts.NotesFile` unchanged when `opts.WorkDir` is empty: `if opts.WorkDir == "" || opts.NotesFile == "" || filepath.IsAbs(opts.NotesFile) { return opts.NotesFile }`. Add an engine test that releases with `NotesFile: "./missing-notes.md"` and an empty WorkDir; it resolves against the package dir, so no chdir is needed. Assert that the surfaced error reads `reading --notes-file ./missing-notes.md: …`.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/engine/release.go:240-241: false as built. Unwind's `git reset --hard` (internal/engine/unwind.go:134) is a git step the run takes, and it still wipes a tracked notes file's edits until task 3-4 lands.
  OLD: `	// never applies. A file inside the repo is left out of every git step the run
	// takes, so it ends the run exactly as it started, staged edits still staged.`
  NEW: `	// never applies. A file inside the repo is left out of the clean-tree check, the
	// stash and every commit the run makes, so a successful run leaves it as it
	// started, staged edits still staged.`
- /Users/leeovery/Code/mint/internal/preflight/preflight.go:82-83: the first half restates the code, and "the one path the clean-tree gate does not count" is false, because gitignored paths are not counted either.
  OLD: `//
// keepOut is passed to CheckCleanTree: the one path the clean-tree gate does not count.`
  NEW: (empty — delete both lines)
- /Users/leeovery/Code/mint/internal/preflight/preflight.go:102-103: "as before" refers to history.
  OLD: `// build outputs never trip the gate. A keepOut path is left out of the probe, so
// its changes never count; every other path counts as before.`
  NEW: `// build outputs never trip the gate. A keepOut path is left out of the probe, so
// its changes never count.`
- /Users/leeovery/Code/mint/internal/record/commit.go:132: restates the code.
  OLD: `// withExclusion appends keepOut's pathspec, if any, to a git argument list.`
  NEW: (empty — delete)
- /Users/leeovery/Code/mint/internal/engine/release_notesfile_git_test.go:38-39: misdescribes the matched prefix. The prefix is want's first prefixLen args, which callers often end before the first pathspec (prefixLen 3 stops the bookkeeping commit at `commit`, before `-m`).
  OLD: `// assertGitArgs fails unless exactly one git invocation starts with want's leading
// words up to its first pathspec, and it carries want exactly.`
  NEW: `// assertGitArgs fails unless exactly one git invocation starts with want's first
// prefixLen args, and that invocation carries want exactly.`

NOTES:
- Real-git behaviour I confirmed independently (git 2.56, scratch repos under /Volumes/Scratch/tmp/claude-501/-Users-leeovery-Code-mint/46a4a797-50cf-4b31-b04c-16a0d451f4b9/scratchpad):
  - The exclude-only `:(top,literal,exclude)` pathspec covers the whole tree for `status` and `stash push` run from a subdirectory.
  - The stash leaves a tracked notes file `MM` (staged and unstaged edits both intact). Pop restores the other changes as it does today, with staged changes coming back unstaged.
  - A notes-only stash prints "No local changes to save".
  - An untracked notes file alone in a new directory is hidden from the collapsed `?? docs/` entry. With a sibling file the entry still reports dirty, and `add -A`/`stash` split the two correctly.
- Gates re-run clean: build, gofmt, vet, `go test -race` on the touched packages, golangci-lint 0 issues.
- README (README.md:107) and the Release NotesFile doc promise "left as it was". Until task 3-4 changes the unwind's reset, that holds only on a successful run.
- `internal/engine/regenerate_write.go:321` uses the same add-then-`git commit -m` shape as CommitBookkeeping. Task 3-3 should reuse whatever replaces the commit-level pathspec here, or it will sweep other paths the same way.
- On macOS's case-insensitive filesystem, `--notes-file NOTES.md` naming `notes.md` reads fine, but the literal exclusion does not match, so the clean-tree check refuses the run. That fails safe; with `--autostash` the file is stashed and restored instead.
- `ReleaseOptions.WorkDir` is a field only tests set, consistent with the existing `Now`/`NotesBody` test overrides.
- The executor disclosed a TDD slip and covered it with mutation checks (disable the exclusion, watch the tests fail).
- Nothing arrived beyond the enumerated inputs.

## Attempt 2

ISSUES:
- /Users/leeovery/Code/mint/internal/engine/release.go:327 (`keepOut := notesExclusion(root, notesPath)`), feeding /Users/leeovery/Code/mint/internal/record/commit.go:44 and /Users/leeovery/Code/mint/internal/engine/autostash.go:54. A notes file inside the repo that is untracked and gitignored still gets an exclusion pathspec. Two steps then fail where they worked before this task. I reproduced both on git 2.56 with `.gitignore` = `notes.md` or `tmp/`, in /Volumes/Scratch/tmp/claude-501/-Users-leeovery-Code-mint/46a4a797-50cf-4b31-b04c-16a0d451f4b9/scratchpad/r32g and /Volumes/Scratch/tmp/claude-501/-Users-leeovery-Code-mint/46a4a797-50cf-4b31-b04c-16a0d451f4b9/scratchpad/r32i.
  (a) Pre-tag artifact commit. `git -C root add -A -- :(top,literal,exclude)notes.md` exits 1 with "The following paths are ignored by one of your .gitignore files: notes.md". Every exclude form does this (`:!x`, `:(exclude)x`, `top`, `literal`), and `--ignore-errors` does not help. CommitDirtyTree then returns "staging dirty tree…", so any release whose pre_tag hook changes files aborts and unwinds.
  (b) `--autostash` with other WIP. `git stash push --include-untracked -- <exclusion>` exits 128 after it has already saved stash@{0}, and it leaves the operator's unstaged changes staged. autostashPush reads the error as nothing stashed, so the clean-tree gate aborts on the WIP. The operator is left with a stray stash entry and a changed index.
  Without the pathspec, `add -A` and `stash push --include-untracked` skip an ignored file silently and both succeed. A gitignored scratch path such as `tmp/notes.md` or an ignored `RELEASE_NOTES.md` is a likely place for an agent to write notes. A tracked file that matches an ignore pattern is not affected: I checked, and `add -A` with the exclusion exits 0.
  FIX: Return the zero Exclusion when the notes file is an ignored untracked path, since git already leaves it out of every step. After `notesExclusion` yields a non-zero rel, probe with `deps.Runner.Run(ctx, "git", "-C", root, "ls-files", "-z", "--others", "--ignored", "--exclude-standard", "--", ":(top,literal)"+rel)`. Non-empty stdout means ignored, so use "". Empty stdout means keep the exclusion. Surface a probe error under "preflight". I checked the probe on real git for `notes.md`, `tmp/notes.md` (an ignored directory) and `odd [x]*.md`: each is listed when ignored, and a non-ignored or tracked path gives empty output. Put the probe where regenerate (task 3-3) can share it. Add engine tests in release_notesfile_git_test.go:
  - an ignored notes file with a pre_tag hook that changes files: the status probe, `add -A` and the commit run the bare argv, with no `--` pathspec;
  - an ignored notes file with `--autostash`: a bare `stash push --include-untracked`.
  The existing in-repo seeds need one more scripted read, returning "", for the probe.
  ALTERNATIVE: `git -C root check-ignore -q -- <rel>`, where exit 0 means ignored and exit 1 means not. It also works, and it treats glob characters literally (checked). But exit 1 arrives as a runner error and has to be told apart from 128 by ExitCode. `ls-files` always exits 0 and answers in its output, so I recommend it.
  CONFIDENCE: medium

- /Users/leeovery/Code/mint/internal/engine/release_notesfile_test.go:291-305 (`TestRelease_NotesFile_RelativePathResolvesAgainstTheRunDirectory`). Production never sets WorkDir. So the AC6 path production takes, a cwd-relative NotesFile resolved by `filepath.Abs` in `notesExclusion` (release.go:847), runs only in this chdir test, and the test asserts nothing about the exclusion. The parallel WorkDir tests (release_notesfile_git_test.go:333-378) always hand `notesExclusion` an absolute path, so `filepath.Abs` never resolves a relative one there. Suppose `notesExclusion` computed the path against root (`notes.md` instead of `sub/notes.md`), the exact trap the task context names. Every test would still pass: the scripted sequence only needs the exclusion to be non-empty. Meanwhile the real `sub/notes.md` would be swept into the artifact commit by `add -A`.
  FIX: In that test, after the release, add `assertGitArgs(t, f, 2, []string{"status", "--porcelain", "--", excludePathspec("sub/notes.md")})` and `assertCommitKeepsNotesOut(t, f, root, "sub/notes.md", "🌿 Release "+nextTag, stagedChangelog)`. Both helpers live in the same package (release_notesfile_git_test.go).
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/git/exclusion.go:11-12 — false in general. From a subdirectory, an exclude-only pathspec scopes `ls-files` and `grep` to the cwd subtree, not the whole tree (checked on git 2.56). The type is exported and task 3-3 will reuse it.
  OLD: `// working directory and glob characters in the name match only themselves. With no
// other pathspec beside it, git applies the exclusion to the whole tree.`
  NEW: `// working directory and glob characters in the name match only themselves. With no
// other pathspec beside it, the command covers what it would with no pathspec at all,
// less the path.`
- /Users/leeovery/Code/mint/internal/record/commit.go:57-58 — the cardinality claim is now false. With a keepOut path, CommitBookkeeping runs three git invocations: add, diff --cached, commit.
  OLD: `// `🌿 Release v0.0.1`). Both git invocations target dir (the repo root) via
// `git -C {dir}` so they are independent of the process working directory.`
  NEW: `// `🌿 Release v0.0.1`). Every git invocation targets dir (the repo root) via
// `git -C {dir}` so it is independent of the process working directory.`
- /Users/leeovery/Code/mint/internal/record/commit.go:107-112 — the comment implies the named paths are committed from the index. A pathspec commit takes their work-tree content. On git 2.56, a partially staged path in the list was committed with its work-tree bytes.
  OLD: `// commitStaged commits the index in dir with subject, reporting whether a commit was
// made. With a keepOut path it commits exactly the other staged paths, named on
// stdin: a bare exclusion pathspec on `git commit` would instead commit the work-tree
// content of every other tracked path, sweeping in unstaged and assume-unchanged
// edits. The list is never empty when the commit runs, since an empty pathspec file
// commits the whole index, keepOut's staged edits included.`
  NEW: `// commitStaged commits the index in dir with subject, reporting whether a commit was
// made. With a keepOut path it names the other staged paths on stdin instead: a bare
// exclusion pathspec on `git commit` would commit the work-tree content of every other
// tracked path, sweeping in unstaged and assume-unchanged edits. The named paths are
// still committed from the work tree, which matches the index only for paths staged
// just before the call. The list is never empty when the commit runs, since an empty
// pathspec file commits the whole index, keepOut's staged edits included.`
- /Users/leeovery/Code/mint/internal/engine/release.go:840 — restates the code, and drops its empty-WorkDir pass-through.
  OLD: `// notesFilePath is the --notes-file path resolved against opts.WorkDir.`
  NEW: (empty — delete)
- /Users/leeovery/Code/mint/internal/engine/release.go:868 — restates the code.
  OLD: `// realPath resolves p through symlinks, falling back to p when it cannot be resolved.`
  NEW: (empty — delete)

NOTES:
- What I checked on real git 2.56, in scratch repos under /Volumes/Scratch/tmp/claude-501/-Users-leeovery-Code-mint/46a4a797-50cf-4b31-b04c-16a0d451f4b9/scratchpad/:
  - A tracked NEXT_RELEASE.md with staged and unstaged edits stays `MM` with both contents intact through every step: the stash push run from a subdirectory, the gate, the bookkeeping commit through `--pathspec-from-file`, and `stash pop`.
  - Other WIP comes back as before.
  - An assume-unchanged edit keeps its `h` bit and stays out of the commit.
  - The artifact commit takes a modified file, a deleted file and `odd [x]*.txt`, and leaves an untracked notes.md untracked.
- `commit --pathspec-from-file` needs git 2.25 or later, and the README states no minimum git version. That only matters if older gits are meant to be supported.
- The bookkeeping commit with keepOut set now takes the work-tree content of any other already-staged path, where before it took the staged content. That differs only for a path partially staged after the clean-tree check: by a preflight hook with no pre_tag hook, or by the operator during the review gate. The comment correction above names this trap. I don't think it needs a code change.
- README.md:107 and the NotesFile doc promise the file is "left as it was". As the executor reports, until task 3-4 changes the unwind's `reset --hard` (/Users/leeovery/Code/mint/internal/engine/unwind.go:134), that holds only on a successful run. The README criterion asks for this wording, so I took it as intended.
- Minor test depth:
  - `TestCommitBookkeeping_Exclusion_NothingElseStaged_FailsWithoutCommitting` asserts only that an error came back, not its `nothing staged to commit` text.
  - The `listing staged paths` failure branch in commitStaged (/Users/leeovery/Code/mint/internal/record/commit.go:121-124) has no test.
- `ReleaseOptions.WorkDir` is a field only tests set, like the existing `Now`/`NotesBody` overrides. Issue 2 exists because this field makes the tests take a different branch from production.
- Nothing arrived beyond the enumerated inputs. To see the previous round's findings, I read the fix-tracking file in the repo, /Users/leeovery/Code/mint/.workflows/token-aware-diff-sizing/implementation/token-aware-diff-sizing/fix-tracking-token-aware-diff-sizing-3-2.md.

## Attempt 3

ISSUES:
- /Users/leeovery/Code/mint/internal/record/commit.go:129: `git --literal-pathspecs ... commit` makes git set `GIT_LITERAL_PATHSPECS=1` in its own environment, and every commit hook it spawns (pre-commit, commit-msg, post-commit) inherits it. A hook that runs `git diff --cached --name-only -- '*.go'` (or any glob pathspec) then matches nothing and silently skips its check. This happens on both the artifact commit and the bookkeeping commit, but only when the notes file is in the repo. In every other case the same commits run with the variable unset. Reproduced: /Volumes/Scratch/tmp/claude-501/-Users-leeovery-Code-mint/46a4a797-50cf-4b31-b04c-16a0d451f4b9/scratchpad/scripts/hookenv.sh prints `GIT_LITERAL_PATHSPECS=1` and `staged *.go = []` with the flag, and `unset` and `[a.go ]` without it. So the operator's hooks behave differently on release commits depending on where the notes file sits, and nothing shows it.
  FIX: Drop `--literal-pathspecs`. Instead, prefix each NUL-separated entry from the `diff --cached --name-only -z` read with `:(top,literal)` before piping it as `--pathspec-from-file=- --pathspec-file-nul` stdin. Pathspec-from-file parses per-entry magic: verified that `:(top,literal)odd [x]*.go` committed only that file and left a staged `oddx.go` uncommitted. Keep the empty-list guard (`staged.Stdout == ""` → no commit). Tests to update:
  - /Users/leeovery/Code/mint/internal/record/commit_test.go:314 `pathspecCommitArgs` loses the flag. The expected stdin at :338 and :407 becomes the prefixed list, separate from the scripted read output.
  - /Users/leeovery/Code/mint/internal/engine/release_notesfile_git_test.go:78 `assertCommitKeepsNotesOut` needs the new argv and a prefixed expected stdin. Today `stagedChangelog`/`stagedArtifacts` serve as both the scripted read and the expected stdin, and that has to split.
  - /Users/leeovery/Code/mint/internal/engine/release_notesfile_git_test.go:349: the no-artifact-commit guard's `argsOf(f, "--literal-pathspecs", ...)` prefix would match nothing after the fix and pass while checking nothing. Point it at `-C root commit -m <artifact subject>`.
  CONFIDENCE: high

NOTES:
- README line 107 says mint "leaves it as it was" with no condition. Until task 3-4 lands, the unwind's `git reset --hard` (/Users/leeovery/Code/mint/internal/engine/unwind.go:134) still wipes a tracked notes file's uncommitted edits on a failed run. The `ReleaseOptions.NotesFile` doc correctly limits its claim to a successful run. The README wording is what the acceptance criterion asks for, and 3-4 makes it true.
- `ReleaseOptions.WorkDir` (/Users/leeovery/Code/mint/internal/engine/release.go:244-246) is set only by tests; production leaves it empty, so paths resolve against the process directory. `Now` and `NotesBody` are existing test overrides of the same kind on this struct.
- `--notes-file CHANGELOG.md` (or the configured version file) is nonsense input. mint writes the file and runs `git add` on it by name, and the bookkeeping commit then fails with `nothing staged to commit` and unwinds. It fails loudly; no change asked.
- Checked against real git: both claims in the `commitStaged` doc comment hold — a bare exclusion pathspec on commit sweeps in assume-unchanged edits, and an empty pathspec file commits the whole index. No comment corrections needed.

## Attempt 4

ISSUES:
- /Users/leeovery/Code/mint/internal/record/commit.go:114-133 (`commitStaged`, reached from :47 and :94). With any in-repo notes file, both commits now go through `git commit --pathspec-from-file`. A pathspec commit is a `--only` commit: git builds a temporary index, and the commit hooks run against it (`GIT_INDEX_FILE=.git/next-index-*.lock`, seen from inside a hook). A pre-commit hook that rewrites a staged file and re-stages it changes only that temporary index. The real index keeps the pre-hook content. That is how formatter hooks work, for example lint-staged with prettier on `*.md`, and CHANGELOG.md is markdown. Reproduced with the built binary: /Volumes/Scratch/tmp/claude-501/-Users-leeovery-Code-mint/46a4a797-50cf-4b31-b04c-16a0d451f4b9/scratchpad/rev4/scripts/restage-hook-repro.sh.
  - With an untracked `notes.md` at the root and `mint release -y --notes-file notes.md`, the release succeeds. Afterwards CHANGELOG.md is left `MM`, and `git diff --cached` shows a staged revert of the hook's rewrite.
  - The same hook with the notes file outside the repo, which takes the bare `git commit -m`, leaves the tree clean.
  - So the task's headline case leaves the operator a dirty index on a file mint wrote. The operator sees it as `git status` noise after a "successful" release, and the next `mint release` refuses at the clean-tree gate. The pre_tag artifact commit has the same exposure.
  - The pathspec route is only needed when the notes file has staged edits. An untracked notes file, or one with only unstaged edits, never rides an index commit: `add -A` already excludes it.
  FIX: Gate the pathspec route on the notes file having staged edits. In `commitStaged`, when `keepOut != ""`, first read `m.Run(ctx, "git", "-C", dir, "diff", "--cached", "--name-only", "-z", "--no-renames", "--", ":(top,literal)"+string(keepOut))`.
  - If it is empty, run the as-built `git -C dir commit -m subject` and return true.
  - If not, keep the current route: the exclusion read, the empty-list guard, and the `literalPathspecs` stdin.
  The headline case (AC1) and unstaged-only edits then run exactly the as-built commit argv. Only the staged-edit case of AC2 keeps the temporary-index behaviour, and that case needs it. Add one line to the `commitStaged` doc naming the remaining trap: in the pathspec route git runs the commit hooks against a temporary index, so a hook that re-stages its rewrite leaves the real index behind. Put the positive literal pathspec on `git.Exclusion` (for example a `Match()` beside `Pathspec()`) rather than spelling out `:(top,literal)` a third time. release.go:859 and `literalPathspecs` already build it.
  Tests:
  - In internal/record/commit_test.go, add a "keepOut has no staged edits" case for `CommitDirtyTree` and one for `CommitBookkeeping`. Each asserts the probe, then the bare `-C dir commit -m subject`. Put the probe in front of the existing pathspec-route cases.
  - In internal/engine/release_notesfile_git_test.go, the untracked `notes.md` cases (`seedInRepoNotesFileGit`, `firstReleaseTail`, the pre_tag case) script the probe as "" and expect the bare commit. Split the `NEXT_RELEASE.md` row into an unstaged variant (bare commit) and a staged variant (probe returns `NEXT_RELEASE.md\x00`, then the pathspec route via `assertCommitKeepsNotesOut`).
  ALTERNATIVE: Do one unfiltered `git -C dir diff --cached --name-only -z --no-renames` read. If `string(keepOut)` is not among the entries, run the bare commit. Otherwise drop it from the list and pipe the rest as now. That is one read instead of two, but Go-side string equality replaces git's pathspec matching. I recommend the probe, because it keeps the exclusion semantics in git.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/engine/release.go:850-853 — this says the exclusion keeps the file out of "the run's git steps", but the unwind's `git reset --hard` (/Users/leeovery/Code/mint/internal/engine/unwind.go:134) is one of those steps and does not use it.
  OLD: `// notesExclusion is the exclusion that keeps the notes file at path out of the run's
// git steps: the zero Exclusion when no file was given, it lies outside the repo, or
// git ignores it. An ignored file needs none, since git already leaves it out, and an
// exclusion naming it makes `add -A` and `stash push` fail.`
  NEW: `// notesExclusion is the zero Exclusion when no file was given, the file lies outside
// the repo, or git ignores it. An ignored file needs none, since git already leaves it
// out, and an exclusion naming it makes `add -A` and `stash push` fail.`

NOTES:
- What I checked end to end with the built mint against real git 2.56. All cases pass:
  - An untracked `notes.md` with a pre_tag hook: the artifact commit holds only `bundle.js`, the bookkeeping commit only CHANGELOG.md, and `notes.md` is still untracked with the same bytes. Hooks see `GIT_LITERAL_PATHSPECS` unset and their `*.js` glob matches, so the round-3 fix holds.
  - A tracked `NEXT_RELEASE.md` in `MM` state, run from `sub/` with `--notes-file ../NEXT_RELEASE.md` and `--autostash`, with other unstaged and untracked work: the index blob and the work-tree bytes of `NEXT_RELEASE.md` are unchanged, the other work is stashed and restored, and no commit carries the notes file.
  - The notes file as the only change with `--autostash`: no stash entry is made and no `stash pop` runs (the git trace shows none).
  - The notes file plus another dirty path, from the root or from a subdirectory: the clean-tree gate fails.
  - A pre_tag hook that changes nothing: no artifact commit.
  - A gitignored `tmp/notes.md` with `--autostash` and other WIP: the bare argv, and the run succeeds.
- `internal/engine/regenerate_write.go:318-321` has the same add-then-`git commit -m` shape as `CommitBookkeeping`, and `record.commitStaged` is unexported. Task 3-3 owns regenerate's exclusion. If 3-3 writes its own copy instead of reusing `commitStaged` (with the gate above), it inherits whichever commit route it copies. The executor's BANK entry says the same. I did not bank it, because nothing is duplicated yet.
- README.md:107 and the `NotesFile` doc say the file is left "as it was" with no condition. Until task 3-4 changes the unwind's `reset --hard`, that holds only on a successful run. The README wording is what the acceptance criterion asks for.
- `ReleaseOptions.WorkDir` is set only by tests. Production resolves the path against the process directory, and the chdir test (`release_notesfile_test.go:291`) now pins the exclusion on that production branch.
- `TestRelease_NotesFile_EmptyWorkDir_ReadsThePathAsGiven` (release_notesfile_git_test.go) names its constant `wantPrefix` but checks it with `strings.Contains`. This is not a failure mode, since the guard still checks the path.
- `commit --pathspec-from-file` needs git 2.25 or later. The README states no minimum git version.
- Inputs: nothing arrived beyond the enumerated inputs. For context on earlier rounds I read the fix-tracking file in the repo, /Users/leeovery/Code/mint/.workflows/token-aware-diff-sizing/implementation/token-aware-diff-sizing/fix-tracking-token-aware-diff-sizing-3-2.md. The verdict rests on my own checks.
