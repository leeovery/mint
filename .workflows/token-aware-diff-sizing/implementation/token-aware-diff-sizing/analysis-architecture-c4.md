AGENT: architecture
FINDINGS:
- FINDING: From a subdirectory, an Exclude answer takes nothing out of the notes: the check matches globs root-relative, but git applies them relative to the cwd
  SEVERITY: medium
  FAILURE: Say an operator runs `mint release` at a terminal from inside `sub/`. The plan supports running from a subdirectory: `--notes-file` resolves against the run directory (§13.1), and TestRelease_NotesFileInRepo_RelativePathFromASubdirectory_ExcludesTheFileItNames pins that case. Here is what happens:
    1. Git writes diff headers relative to the repo root, so the check's list shows `sub/tests/t_test.go`.
    2. The AI suggests `sub/tests/`. `pathspecMatches` checks it against that root-relative path, and the pause asks "Exclude sub/tests/? (…; 1 file, …)".
    3. The operator answers Exclude. mint writes the glob into `[release].diff_exclude` and commits it.
    4. `checkExclusions` adds `:(exclude)sub/tests/` to the Assembler and re-assembles the diff with a plain `Run` from `sub/`. Git reads that pathspec relative to the cwd, as `sub/sub/tests/`, so it matches nothing.
  The re-assembled diff, the Change Map, every chunk call and any `r` all still carry the test files. The notes ship describing files the operator chose to leave out, after a pause that counted them as excluded. The operator finds out only by reading the notes, at the review gate or after the release is out.
  A `[release].diff_keep` entry is matched in Go root-relative, while every `diff_exclude` entry reaches git cwd-relative, so from a subdirectory the two lists disagree on which paths they cover.
  Reproduced in a scratch repo, from `sub/`:
    - `git diff --name-only v1..HEAD -- . ':(exclude)sub/tests/'` still lists `sub/tests/t_test.go`;
    - `':(exclude)tests/'` drops it.
  FILES: internal/notes/assemble.go:186-187, internal/notes/assemble.go:107-110, internal/notes/assemble.go:150, internal/notes/changemap.go:17, internal/notes/changemap.go:48-57, internal/notes/pathspec.go:9-15, internal/notes/exclusioncheck.go:228-241, internal/notes/exclusioncheck.go:249-263, internal/notes/select.go:249-259, internal/engine/release.go:1016, internal/engine/regenerate_fresh.go:121, internal/git/exclusion.go:13-19, internal/commit/generate.go:185-192, internal/commit/generate.go:244
  DESCRIPTION: This feature added Go-side pathspec matching to a pipeline whose git half still runs from the process cwd. Three pieces of the exclusion check depend on that matching:
    - which suggestions it keeps (`usableSuggestions`);
    - what each pause counts (`matches`);
    - what the keep list covers (`coveredBy`).
  `pathspecMatches` is documented as reporting "whether git matches path to spec". That holds only when git runs at the repo root.
  The rest of the feature already guards against this:
    - `git.Exclusion` is root-anchored (`:(top,literal,exclude)`) "so it means the same file from any working directory";
    - the answers commit, `IsIgnored` and `notesExclusion` all run `git -C root`;
    - commit's own diff source runs at the root through `RunInDir`, under CLAUDE.md seam 1.
  The notes Assembler is the one read-side path that uses `-- .` over a plain `Run`, which is exactly the case seam 1 forbids: "a plain `Run` from a subdirectory silently scopes to the subtree". Its diff and Change Map now carry the check's answers. Its comments justify it as running "cwd-relative like the other engine git calls", and the feature's other git calls no longer do. The cwd also confines a subdirectory run's notes to that subtree. That scoping predates this feature, but the Exclude and keep promises this feature makes are what now quietly break on it.
  RECOMMENDATION: Give the Assembler the repo root: `NewAssembler(r, root, exclude)`. Both production sites, `resolveBody` and `freshGenerator`, already have `root` in scope.
    - Run AssembleRange's diff and BuildChangeMapForRange's two `git diff` calls through `RunInDir(ctx, a.root, nil, "git", …)`, as commit's `sourceDiff` does. `-- .` then covers the whole tree, and every `:(exclude)` glob is read root-relative: the built-ins, both `diff_exclude` levels and the Exclude answers. Those are the semantics `pathspecMatches` mirrors and the recorded `.mint.toml` globs assume.
    - Rewrite the two "run cwd-relative like the other engine git calls" comments (assemble.go:150, changemap.go:17) to say the calls run at the repo root.
    - Pin it in the assembler tests: assert the recorded `Dir` on the FakeRunner's diff and Change Map invocations.
COMMENT_CORRECTIONS:
- internal/notes/assemble.go:10-11 — the package doc says the size check is "deliberately not here", but the package holds the max_prompt_bytes check (generate.go:227) and the chunk sizing it drives
  OLD: // bookkeeping file; embedded mode keeps the real source). The max_prompt_bytes size
// check is a later layer — deliberately not here.
  NEW: // bookkeeping file; embedded mode keeps the real source).
- internal/notes/generate.go:149 — "in later tasks" is workflow vocabulary, and the downstream write has shipped
  OLD: // (no parsing/splitting) for the downstream provider/changelog write in later tasks.
  NEW: // (no parsing/splitting) for the downstream provider/changelog write.
SUMMARY: The changes since cycle 3 add no new seam: the request's `Yes` field is deleted, the -y guard test now supplies -y, and the setup guide routes `on_notes_failure` to release only. One medium finding remains: the exclusion check matches globs in Go as root-relative, but the notes Assembler runs its diff and Change Map from the process cwd, so from a subdirectory an Exclude answer silently excludes nothing. Two stale comments in internal/notes are collected for direct correction.
