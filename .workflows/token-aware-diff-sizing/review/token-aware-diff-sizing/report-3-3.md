TASK: Regenerate one version from a notes file (token-aware-diff-sizing-3-3, tick-08089c)

ACCEPTANCE CRITERIA:
- Given a released version, `mint release regenerate <version> --notes-file notes.md --target both -y` asks no source question and makes no AI call. The changelog section and the provider release receive the file's contents byte-identical. With `--target release` only the provider release is written, and with `--target changelog` only the changelog (§13.1, §13.3)
- Given the oldest version, which has no prior tag, the supplied body is written, not `Initial release.` (§13.1, §13.3)
- Given an attended run, the plan shown before the gate does not name the source as `fresh`, `tag` or `release` (§13.1)
- Given an attended run, the body shows at regenerate's review gate, offering accept, edit and abort with no `r`, not the accept/abort confirm the `tag` and `release` sources show. Choosing edit changes the body written, not the file on disk, and choosing abort writes nothing. Under `-y` the gate auto-accepts inside the presenter (§13.2, §13.3)
- Given a `changelog` or `both` target and either an untracked notes file in the repo or a tracked one with uncommitted edits, staged or unstaged (§13.2): the clean-tree check passes; the changelog commit contains only `CHANGELOG.md`; afterwards the file is as it was (still untracked, or carrying the same edits, staged if they were staged and unstaged if not); any other dirty path still fails the check as built
- Given a relative path, it resolves against the directory mint was run from. The body written is the file's contents as read at startup (§13.1, §13.2)
- `mint release regenerate --help` lists `--notes-file P` with the description `notes source: the finished notes in file P (one version only)` (§13.1, §18.3)
- The README's `release regenerate` section documents `--notes-file` (§18.2): a row in its flag table; that it is a notes source beside fresh, tag and release that runs no AI; that it serves one version only

STATUS: complete

SPEC CONTEXT: §13.1 makes `--notes-file <path>` a fourth regenerate notes source beside fresh, tag and release. It takes a path only and resolves it against mint's run directory, and one file answers one version. §13.2 (with the 2026-10-19 corrigenda) has the file read once at startup. It is kept out of the clean-tree check and every commit, and is left untouched afterwards, staged state included, whether it is untracked or tracked with edits. §13.3 (with its corrigendum) has a supplied body skip every AI call and win over the first-release body. It shows at regenerate's own review gate (accept/edit/abort, no `r`), not the reuse confirm, auto-accepts under `-y`, and flows byte-identical to every sink. §18.2/§18.3 require the README row and prose and the curated usage line.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - Flag parse: cmd/mint/regenerate_flags.go:98 (flag), :131-132 (set-ness), :170 and :210 (`--notes-file`/`-notes-file` passed through as a separate-token value, so the path is not taken as a second positional)
  - Startup read before any repo/config/branch resolution: cmd/mint/main.go:150 (`engine.ReadNotesFile` on the raw path, so `os.ReadFile` resolves it against the process cwd)
  - Threading: cmd/mint/regenerate_run.go:30-37 (`regenerateRunFlags` carries `NotesFile`), cmd/mint/main.go:244 (`runReq.Supplied`)
  - Source takeover: internal/engine/regenerate_interactive.go:164-166 (a supplied body forces `SourceOf(regenerateSourceSupplied)`, so `resolveSource` never prompts), :270-275 (`produceRegenerateBody` returns the held body and never calls `ProduceBody`. That rules out both the AI and `RegenerateFreshBody`'s `Initial release.`)
  - Plan label: internal/engine/regenerate_interactive.go:413-414 (`notes file`)
  - Gate: internal/engine/regenerate_write.go:239-240 (supplied/floor → `reviewGate` with `presenter.NotesReviewGateNoRegen()`), internal/presenter/gate.go:177-181
  - Keep-out: internal/engine/regenerate_interactive.go:197-204 (`notesExclusion` only for a changelog-writing target, fed to the clean-tree gate), internal/engine/regenerate_write.go:263, :304, :313 and :347-358 (changelog commit through `record.CommitStaged` with keepOut)
  - Usage: cmd/mint/usage.go:58
  - README: README.md:153 (flag-table row), README.md:161 (prose), README.md:175 (example)
- Notes: The task's Context pointed at `newBatchBodyProducer` and `regenerateSourceAxis` as the dispatch points. The build instead short-circuits body production in the engine on `req.Supplied`, with an unexported `regenerateSourceSupplied` the cmd layer cannot name. This is sound and arguably better than the plan's wording: no producer path can be reached for a supplied body, and `--source` stays refused beside it. The relative-path handling is consistent between the read (`os.ReadFile`) and the exclusion (`repoRelativePath` → `filepath.Abs`), since both resolve against the process cwd. The body read once at startup is the only content consumed afterwards.

TESTS:
- Status: Adequate
- Coverage:
  - Source, AI and both targets: internal/engine/regenerate_notesfile_test.go:113 (both target: no source question, no producer call, changelog section and provider body byte-identical). :141 (release-only writes no commit/push and leaves CHANGELOG.md untouched; changelog-only dispatches no provider write)
  - Oldest version: :191 (supplied body wins over a producer that returns `record.FirstReleaseBody`)
  - Plan label: :216 (plan steps exactly `source: notes file`, `release: v1.4.0`)
  - Gate: :242 (`Use these notes?` with keys y/n/e and the supplied body shown). :269 (edit changes both sinks; the file is unchanged). :298 (abort: non-zero, no commit/push, CHANGELOG.md untouched, no provider write). :319 (real presenter under `-y` renders `notes: accepted (-y)` and no `source:` line)
  - Keep-out: :361 (exact `status --porcelain -- :(top,literal,exclude)notes.md` argv; as-built vs pathspec-from-file commit carrying only `:(top,literal)CHANGELOG.md`; the notes path never named outside its exclusion; no reset; file content unchanged). :415 (another dirty path still fails the check)
  - Relative path and startup read: :456 (engine), and cmd/mint/regenerate_notesfile_test.go:192, which drives the real `regenerate()` wiring. The file is rewritten after startup and the `gh release edit` stdin still carries the startup body.
  - Parsing: cmd/mint/regenerate_flags_test.go:294 (separate-token, single-dash and `=` forms on either side of the positional) and cmd/mint/regenerate_run_test.go:123 (threading)
  - Usage: cmd/mint/usage_test.go:180 (coverage) and :255 (word-for-word line)
- Notes: In the :361 table, the "untracked file" and "tracked file with unstaged edits" subtests script identical git output and assert identical things, because the FakeRunner cannot tell the two states apart and the code has no branch between them. The second case adds no coverage, but it costs nothing and fails nothing. The README criterion has no test, which is acceptable for prose.

CODE QUALITY:
- Project conventions: Followed. Subprocesses go through the runner and mutations through the Mutator. The gate's `-y` handling stays inside the presenter. External test packages, `t.Parallel()` (except the two `t.Chdir` tests, which are documented) and exact-argv assertions are used throughout.
- SOLID principles: Good. The supplied body is a source value consumed at one point (`produceRegenerateBody`), and gate selection stays in `gateRegenerate`'s switch.
- Complexity: Low
- Modern idioms: Yes (`slices.DeleteFunc` on a fresh literal slice in `NotesReviewGateNoRegen`)
- Readability: Good
- Issues: None that cross the reporting bar. The comments the change touched (regenerate_write.go:29-34 and :215-220, regenerate_interactive.go:125-131, regenerate_flags.go:85-90 and :169-172) hold against the code.

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "afterwards the file is as it was: still untracked, or carrying the same edits, staged if they were staged and unstaged if not" — the suite scripts git through FakeRunner, so it proves only the argv: `git -C root commit -m` when the notes file has no staged edits, else `commit --pathspec-from-file=- --pathspec-file-nul` with stdin `:(top,literal)CHANGELOG.md\0`. To settle it, run `mint release regenerate <v> --notes-file notes.md --target changelog -y` against a scratch repo with a bare origin, three times: once with notes.md untracked, once with a tracked notes.md carrying unstaged edits, and once with staged edits. After each run, check `git status --porcelain -- notes.md` and `git diff --cached -- notes.md` against their pre-run values, and confirm that `git show --name-only HEAD` lists only CHANGELOG.md.
