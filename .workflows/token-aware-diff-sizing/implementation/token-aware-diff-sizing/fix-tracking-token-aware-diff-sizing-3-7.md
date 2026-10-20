## Attempt 1

ISSUES:
- /Users/leeovery/Code/mint/cmd/mint/main.go:170-174 (with :146-155): `gitrepo.ResolveReleaseBranch` runs in `runRegenerate` before `runRegenerateSingle` ever reads the file. When `release_branch` is not set, it runs `git symbolic-ref --short refs/remotes/origin/HEAD`. That always fails with no origin remote, and it also fails when origin was added with `git remote add` and no `set-head` was run. So AC1's no-origin case prints the branch error and exits 2 instead of the refusal. I reproduced this with the built binary against throwaway repos:
  - **No origin, no config:** `mint release regenerate 1.4.0 --notes-file empty.md` printed `mint: gitrepo: origin/HEAD is unset: running "git": exited with code 128: exit status 128` and exited 2.
  - **9.9.9 with no origin/HEAD:** same error, exit 2.
  - **Same repo with `release_branch = "main"`:** printed `notes: FAILED - --notes-file empty.md is empty` and exited 1.
  - **Broken `.mint.toml`:** regenerate reports the config error with exit 2; `mint release` reports the notes refusal with exit 1.

  So the task's Outcome ("on both verbs, an unusable `--notes-file` is the first and only thing the run reports") holds for release but not for regenerate. The new doc line at main.go:197 ("refused ahead of every check") describes the order the code does not have, so it gets fixed along with this.
  FIX: Read the file in `runRegenerate` straight after `parseRegenerateFlags` succeeds, before `gitrepo.ResolveRoot`, the same order `Release` uses.
  - Build the presenter at that point from the parsed `req.Plain`/`req.Yes`. `validateRegenerateRequest` returns `req` unchanged on success, so the values are the same.
  - Call `engine.ReadNotesFile(p, req.NotesFile, req.NotesFileSet)` there and return `exitCode(err)` on a refusal.
  - Pass `supplied` into `runRegenerateSingle` as a parameter and drop the read at main.go:206-209. The `--all` path is safe: parse already rejects `--notes-file` with `--all`, so `set` is false there and the call returns nil.
  - Move the main.go:197 doc line to `runRegenerate` and phrase it as ahead of the repo/config/branch resolution.
  - For a hermetic test, give the post-parse body of `runRegenerate` the same seams `runRegenerateSingle` already takes: a `runner.CommandRunner` and the presenter as parameters, with the production wrapper building `NewExecRunner`/`NewForStartup`. Then a FakeRunner test with no `release_branch` can assert zero invocations (no `rev-parse`, no `symbolic-ref`), exactly one notes StageFailed, and exit 1. Rework the existing AC1/AC2 tests onto that seam.
  ALTERNATIVE: Read after the presenter is built (main.go:169) and just before `ResolveReleaseBranch` (main.go:170). This is a smaller move and keeps the config-dependent usage errors (changelog-disabled target, `-y` without `--target`, exit 2) ahead of the refusal. AC1 would be met. The cost is that a malformed `.mint.toml` still reports before the refusal, which differs from release, which refuses first. I recommend the primary fix for parity with release and §13.2's "at startup".
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/cmd/mint/regenerate_notesfile_test.go:55 — "each --notes-file mint cannot use" is false, because the helper leaves out the unreadable case (a directory); otherwise it repeats the helper's name and body.
  OLD: // unusableRegenNotesFiles pairs each --notes-file mint cannot use with its refusal line.
  NEW:

NOTES:
- `engine.ReadNotesFile(p, path string, set bool)` takes a positional bool, which is on code-quality.md's anti-pattern list. It comes from `readNotesFile` and the task asked for a wrapper, so I am not raising it as an issue.
- `RegenerateRunRequest` now holds `NotesFile` and `Supplied`, and the two must agree. A caller that sets only `NotesFile` would get the exclusion computed for a file whose body is never used. The one production caller sets both from the same request, and the task specified this shape.
- What I checked beyond the diff: I built the binary and ran it against throwaway repos under /Volumes/Scratch/tmp/claude-501/-Users-leeovery-Code-mint/46a4a797-50cf-4b31-b04c-16a0d451f4b9/scratchpad (`repo-noorigin`, `repo-noorigin2`, `repo-cloned`). In a repo whose non-github origin has origin/HEAD set, both the 1.4.0 and 9.9.9 cases give the notes refusal with exit 1, as intended. The order-mutation check ran in the `mintcopy` copy there. No project files were touched.
- The executor's change to `TestParseRegenerateFlags_EmptyNotesFilePath_CountsAsTheFlagGiven` (renamed, now reads `req.NotesFileSet`) is correct, since the request field it used to read is gone.
- Nothing arrived with the dispatch beyond the enumerated inputs.
