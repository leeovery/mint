TASK: Build the release-notes built-in excludes from the names their owning packages export (token-aware-diff-sizing-7-14, tick-636b7b)

ACCEPTANCE CRITERIA:
- A forward release whose range touches `.mint.toml`, `CHANGELOG.md` and source runs its notes diff, `--name-status` and `--numstat` calls with argv byte-identical to today's, `:(exclude)CHANGELOG.md :(exclude).mint.toml` first among the pathspecs, and the notes prompt names neither file; `release_builtin_exclude_test.go` and every other exact-argv test pass with their literals unchanged (§5.3)
- Regenerate's fresh source over a tag range excludes the same two files with the same argv, beside the plain-mode `version_file` exclusion where one applies (§5.3)
- A release range whose only change is `.mint.toml` still gets the maintenance-release body with no AI call (§5.3)
- `mint init` writes `.mint.toml` at the repo root and reports it created, or skipped with `exists, use --force`, exactly as today; the init tests pass unchanged
- A `.mint.toml` that fails to read, parse or validate fails with the same message as today, naming `.mint.toml`; the config tests pass unchanged
- Rule: every non-test Go site that uses either name as a path takes it from the owning constant (`.mint.toml` from `internal/config`'s export, `CHANGELOG.md` from `record.ChangelogFileName`); neither `internal/engine/init.go` nor `internal/notes/assemble.go` holds either name as a code literal
- The five gates pass: `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues)

STATUS: complete

SPEC CONTEXT: §5.3 makes `.mint.toml` a built-in, non-configurable always-exclude beside `CHANGELOG.md` for every release-notes diff (forward release and regenerate's fresh source), path-based via git `:(exclude)` pathspecs; the plain-mode `version_file` exclusion is unchanged. The task is a pure single-owner refactor: the names stay byte-identical, only their source moves to the owning packages so a later rename cannot leave the notes excluding a stale name.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/config/config.go:43-44 — `const FileName = ".mint.toml"` (exported; the unexported `configFileName` is gone, no reference to it remains anywhere in the repo's Go files, README.md or CLAUDE.md)
  - internal/config/config.go:383, 390, 419, 458, 462, 512, 516, 518, 787, 828, 836, 858 — `Load`'s path join and every error message use `FileName`; message text is unchanged (`fmt.Errorf("invalid %s: ...", FileName, ...)` renders `invalid .mint.toml: ...` as before)
  - internal/engine/init.go:83 — the const block now holds only `shimFileName`; internal/engine/init.go:114 — the config target's `Name` is `config.FileName` (import added at :28; the engine package already imported config elsewhere, so no new package edge)
  - internal/notes/assemble.go:29-32 — `builtinExcludePathspecs` is `":(exclude)" + record.ChangelogFileName` then `":(exclude)" + config.FileName`, in that order, so the argv is byte-identical (`:(exclude)CHANGELOG.md`, `:(exclude).mint.toml`)
- Notes:
  - Commit 26df8a6 touches exactly these three Go files and no tests, so every test literal is unchanged by construction.
  - The rule criterion holds. Searching all non-test Go files under internal/ and cmd/ for either name finds the two owning constants (internal/record/changelog.go:20, internal/config/config.go:44). Every other hit is display or doc text the task names as staying: metadata descriptions at internal/config/metadata.go:120 and :130, narration labels at internal/engine/release.go:1648 and :1664, the gate label at internal/engine/regenerate_interactive.go:357, comments, and the initgen, setupguide and usage prose. Every path use reads the constants: config `Load` and its errors, config/appendrelease.go:36-61, the init write, the notes exclude, the record writes at internal/record/changelog.go:81 and internal/record/rebuild.go:93, the record staging at internal/record/commit.go:245, the regenerate staging at internal/engine/regenerate_write.go:348-349, and the Phase 8 answers sites at internal/engine/release.go:1093-1145. internal/engine/init.go and internal/notes/assemble.go hold neither name as a code literal; the only mention left in init.go is the comment at :69, which the task keeps as written.
  - No import cycle: notes already imported config and record in select.go and resolve.go, and config imports only cmdline.

TESTS:
- Status: Adequate
- Coverage:
  - internal/engine/release_builtin_exclude_test.go:13-38 pins the forward-release diff, `--name-status` and `--numstat` argv with `:(exclude)CHANGELOG.md :(exclude).mint.toml` immediately after `-- .`. It uses `assertNotesGitCallsExclude` (internal/engine/release_diffexclude_test.go:26-39), which asserts the full ordered argv tail.
  - :40-68 covers the `.mint.toml`-only range: the maintenance body, zero claude invocations and the exact diff argv.
  - :70-88 covers regenerate's fresh source with the same argv.
  - :90-124 covers the plain-mode `version_file` exclusion beside the built-ins and the embedded-mode keep.
  - internal/engine/init_test.go pins `.mint.toml` as the written path and the rendered Target for created, skipped (`exists, use --force`) and force outcomes.
  - Config and commit tests pin `invalid .mint.toml: ...` message literals, for example internal/config/fallback_removed_test.go:12 and internal/commit/run_malformed_aicommand_test.go:28-58.
- Notes: The task deliberately adds no tests. The existing literal-pinned argv, message and init assertions are what fail loudly if a later rename changes an owner constant, which is the behaviour the task needs.

CODE QUALITY:
- Project conventions: Followed. The comments stay true to the shipped code. The `FileName` doc still holds, and the `builtinExcludePathspecs` comment still describes what the list does.
- SOLID principles: Good. Each name has one owner.
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "The five gates pass: `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues)" — run the five gate commands from the project root. Reading shows the concatenated argv, error messages and init target are byte-identical, so the tests named in the other criteria should pass unchanged, but only running them confirms it.
