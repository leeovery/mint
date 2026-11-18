TASK: Run the notes diff and Change Map at the repo root, so an Exclude answer or a keep-list glob covers what the check counted (token-aware-diff-sizing-12-1)

ACCEPTANCE CRITERIA:
- A prior-tag `mint release` in a repo rooted at R runs its notes diff (`git diff --src-prefix=a/ --dst-prefix=b/ {lastTag}..HEAD -- . {excludes}`) and both Change Map reads (`--name-status`, `--numstat`) with R as their working directory, whatever directory mint was started from, with argv byte-identical to before (§5.1, §8.2)
- `mint release regenerate <version>` from the fresh source runs its `{PreviousTag}..{Tag}` diff and both Change Map reads at R the same way, with unchanged argv; so does an `r` at its review gate (§5.1, §5.3, §6.1)
- At a terminal, the check suggests `sub/tests/` and the operator answers Exclude: the re-assembled diff and the Change Map built after it carry `:(exclude)sub/tests/` and run at R; an `r` at the review gate re-reads the diff at R with the glob still excluded (§5.1, §6.5)
- Every existing exact-argv pin on the notes diff and Change Map, in internal/notes and internal/engine, holds unchanged. All five gates in CLAUDE.md pass

STATUS: complete

SPEC CONTEXT: §5.1 says `diff_exclude` entries at every level are git pathspec globs and git does the matching. Release notes are a public list of what a release holds, so the notes diff must cover the whole tree. The exclusion check records its globs root-relative (they are matched in Go against git's root-relative diff headers). An Exclude answer written to `[release].diff_exclude` must therefore be read root-relative by the re-assembled diff, the Change Map, every later generation and an `r`. §6.1 carries recorded answers into regenerate's fresh source. CLAUDE.md seam 1 requires read-side git with cwd-relative pathspecs (`-- .`) to run at the root through `RunInDir(root, ...)`.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/assemble.go:88-92 (Assembler gains `root`), :101-103 (`NewAssembler(r, root, exclude)`), :185 (AssembleRange diff through `a.runner.RunInDir(ctx, a.root, nil, "git", args...)`)
  - internal/notes/changemap.go:51 (`--name-status`) and :57 (`--numstat`), both through `RunInDir(ctx, a.root, nil, ...)`
  - internal/engine/release.go:1016 (`resolveBody` passes the `gitrepo.ResolveRoot` root from release.go:361)
  - internal/engine/regenerate_fresh.go:121 (`freshGenerator` passes `root`, which is threaded from cmd/mint/main.go:158 through cmd/mint/regenerate_all.go:117/:136 and cmd/mint/regenerate_run.go:79/:100)
- Notes:
  - The task commit (f8f536d) leaves argv byte-identical: only the runner method changes, from `Run` to `RunInDir`.
  - The Assembler makes no other git reads. Across internal/notes production code, the only runner calls are the three RunInDir sites above plus `rangeSubjects`' `git log --format=%s` (internal/notes/resolve.go:22). That call takes no pathspec, and the plan deliberately leaves it on `Run`.
  - The Exclude-answer path, `checkExclusions` (internal/notes/select.go:249-259), calls `s.assembler.Exclude` and then `AssembleDiff` on the same root-anchored Assembler. The release gate's `r` reuses the generator `resolveBody` returns, which wraps that same Assembler. The regenerate `r` builds its own Assembler through `freshGenerator` with the same root.
  - Both stale "cwd-relative like the other engine git calls" comments are rewritten (assemble.go:148-149, changemap.go:17). A search of internal/notes, internal/engine and cmd/mint production code finds no remaining claim that these reads run cwd-relative.
  - Production `ExecRunner.RunInDir` (internal/runner/exec_runner.go:60-65) appends nil env to `os.Environ()`, so the host environment is still inherited. No behaviour is lost by moving off `Run`.

TESTS:
- Status: Adequate
- Coverage:
  - internal/notes/assemble_root_test.go:15-87 is table-driven over AssembleDiff, AssembleRange, BuildChangeMap and BuildChangeMapForRange. For each read it asserts `Dir == root`, `Name == "git"` and the exact argv, with a `sub/tests/` glob in play.
  - internal/engine/notes_reporoot_test.go:68-90 covers AC1 and AC3 end to end. An attended prior-tag release gets an Exclude answer for `sub/tests/` and then an `r`. The test asserts all seven notes reads in order: the initial diff, then diff, name-status and numstat after the answer, then diff, name-status and numstat for the `r`. Each read has exact argv carrying `:(exclude)sub/tests/` after the answer and `Dir` equal to the root that `rev-parse --show-toplevel` resolved (seeded by release_priortag_test.go:49).
  - internal/engine/notes_reporoot_test.go:92-127 covers AC2. A fresh-source `RegenerateRun` with `[release].diff_exclude = ["sub/tests/"]` gets an `r`. The test asserts both passes (diff, name-status and numstat each) at root with exact range argv.
  - Under the old `Run`, `FakeRunner.Run` records `Dir` as "" (internal/runner/fake_runner.go:104-111), so every one of these assertions would fail if the reads moved back off the root.
  - AC4 pins: in internal/notes, the test-file changes in f8f536d are constructor-signature updates only, with no argv edits. The one engine test-site change (internal/engine/release_regen_sizecheck_test.go:52) is a constructor update too. No existing test compares a notes-diff invocation's `Dir` or its whole Invocation struct, and the wrapper runners that override only `Run` (in internal/engine and cmd/mint tests) intercept other git calls, never the notes reads.
- Notes: The tests are not over-tested. The notes-level table pins each Assembler read, and the two engine tests pin production wiring for the forward and regenerate paths without overlapping.

CODE QUALITY:
- Project conventions: Followed. The change follows CLAUDE.md seam 1 and mirrors commit's `diffSourceText` RunInDir pattern. Tests are in external test packages, use t.Parallel and assert exact argv and Dir.
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes
- Readability: Good. The NewAssembler doc (assemble.go:94-100) states why root exists: the pathspecs are read cwd-relative and the globs are root-relative.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "All five gates in CLAUDE.md pass" — settling this means running `go build ./...`, `gofmt -l .`, `go vet ./...`, `go test -race ./...` and `golangci-lint run` over the tree.
