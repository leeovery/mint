# Consolidation Tasks: Token Aware Diff Sizing (Phase 7)

## Task 1: Build the release-notes built-in excludes from the names their owning packages export
placement: phase 7
severity: duplication

**Problem**: The file name mint reads as its config, `.mint.toml`, is now written out in three places: `internal/config`'s unexported `configFileName` (config.go:42), used by `Load` and its error messages; `internal/engine/init.go`'s own `configFileName` (:85), used by `mint init`; and, since 7-3, the literal `":(exclude).mint.toml"` in `builtinExcludePathspecs` (`internal/notes/assemble.go:22-28`). The same list types `":(exclude)CHANGELOG.md"` by hand although `record.ChangelogFileName` is exported (`internal/record/changelog.go:20`). §5.3 rests on one rule: the file mint reads as its config, and the changelog it projects, never reach release notes. If either name is changed at its owner but not in notes, the notes keep excluding the old name, and every config edit (Phase 8's answers commit included) or changelog projection reaches the notes AI diff and Change Map of every release and regenerate. Nothing fails: `release_builtin_exclude_test.go` pins the literal argv, so it passes against the stale name, and the drift shows only in published notes.
**Solution**: Export the config file name from `internal/config` (one constant, used by `Load` and its error messages), have `internal/engine/init.go` use it in place of its own copy, and build `builtinExcludePathspecs` from `":(exclude)" + record.ChangelogFileName` and `":(exclude)" + <the exported config name>`. notes already imports both config and record, and config imports only cmdline, so no import cycle arises. The argv stays byte-identical, so the existing exact-argv tests pass unchanged, and a later rename changes the argv at every site at once (the finder's proposed shape).
**Outcome**: Each built-in excluded file name has one owner, and Phase 8's write and `git add` sites (8-1, 8-2) can take the config name from the same export. Every notes git call, config error and `mint init` outcome is byte-identical to today's, and every existing test passes with its literals unchanged.

**Acceptance Criteria**:
- [ ] A forward release whose range touches `.mint.toml`, `CHANGELOG.md` and source runs its notes diff, `--name-status` and `--numstat` calls with argv byte-identical to today's, `:(exclude)CHANGELOG.md :(exclude).mint.toml` first among the pathspecs, and the notes prompt names neither file; `release_builtin_exclude_test.go` and every other exact-argv test pass with their literals unchanged (§5.3)
- [ ] Regenerate's fresh source over a tag range excludes the same two files with the same argv, beside the plain-mode `version_file` exclusion where one applies (§5.3)
- [ ] A release range whose only change is `.mint.toml` still gets the maintenance-release body with no AI call (§5.3)
- [ ] `mint init` writes `.mint.toml` at the repo root and reports it created, or skipped with `exists, use --force`, exactly as today; the init tests pass unchanged
- [ ] A `.mint.toml` that fails to read, parse or validate fails with the same message as today, naming `.mint.toml`; the config tests pass unchanged
- [ ] Rule: every non-test Go site that uses either name as a path (config loading and its errors, `mint init`'s write, the notes built-in exclude, record's write and staging) takes it from the owning constant: `.mint.toml` from `internal/config`'s export, `CHANGELOG.md` from `record.ChangelogFileName`. Neither `internal/engine/init.go` nor `internal/notes/assemble.go` holds either name as a code literal
- [ ] The five gates pass: `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues)

**Do**:
- The measured set: `rg -n --type go -g '!*_test.go' '"[^"]*(CHANGELOG\.md|\.mint\.toml)[^"]*"' internal cmd` gives 11 lines. Four use the names as paths: the owners `internal/config/config.go:42` and `internal/record/changelog.go:20`, and the two copies this task removes, `internal/engine/init.go:85` and `internal/notes/assemble.go:28`. The other seven are display text, not paths, and stay as written: comments at `internal/engine/init.go:68` and `internal/presenter/presenter.go:310`, config-metadata descriptions at `internal/config/metadata.go:120` and `:130`, and narration labels at `internal/engine/release.go:1524`, `:1540` and `internal/engine/regenerate_interactive.go:362`.
- `internal/config/config.go:41-42`: export the constant (the finder's example name is `config.FileName`), keeping its value `.mint.toml`, and move every reference in the package onto it. `rg -n 'configFileName' internal/config` gives 13 lines: the declaration and 12 uses, which are `Load`'s path join at :383 and the error messages from :390 to :857. No test references the unexported name.
- `internal/engine/init.go:82-87`: drop `configFileName` from the const block, keep `shimFileName`, and build the `:118` target's `Name` from the config export. The engine package already imports config (`release.go:42`), so this adds no package edge.
- `internal/notes/assemble.go:28`: build `builtinExcludePathspecs` as `":(exclude)" + record.ChangelogFileName` then `":(exclude)" + <the config export>`, in that order, so the argv stays byte-identical. notes already imports both packages (`select.go:7-8`), and config imports only `cmdline`, so no cycle arises.
- Test files keep their literal argv and file names. Those literals are what make a later rename fail loudly.

## Task 2: Bring the README's exclusion-check text up to what 7-8 and 7-13 built
placement: phase 7
severity: drift

**Problem**: The README's `release` section describes the exclusion check as 7-5, 7-7 and 7-12 left it, not as built. README.md:115 describes the check's list as each path with its change and size, with no word of the folder condensing 7-13 added. README.md:117 says a pause shows "the glob and the AI's reason", not the matched files' count, total diff size and keep-list names 7-8 added (the warning that an Exclude takes kept files out too, §5.2), and its no-pause sentence omits a suggestion whose matches earlier answers in the run already decide. README.md:119 lists four failure causes and omits the fifth, a list that will not fit in `max_prompt_bytes` even condensed, so an operator who sees `couldn't check for files that don't belong in the notes` with nothing beneath it on a large release finds no explanation. §18.2 requires the check and its pauses documented as built. `internal/config/readme_exclusion_check_test.go:29` pins the stale pause sentence word for word, so the tripwire asserts the outdated text. No Phase 8 task touches these sentences.
**Solution**: Documentation only, no behaviour change (the finder's proposed shape):
- README.md:115: say that once the list would take more than a tenth of `max_prompt_bytes`, it condenses by folder, a folder's paths summed up as a count and a total diff size.
- README.md:117: after the glob and reason, say the pause shows how many changed files the notes still carry that the glob matches, the total size of their diffs, and any of them on the keep list or kept earlier in the run, which an Exclude takes out too; extend the no-pause sentence to a suggestion whose matches earlier answers in the run already decide.
- README.md:119: add the list that will not fit in `max_prompt_bytes` even condensed to the failure causes.
- Update the pinned pause sentence in `readme_exclusion_check_test.go` in the same change, and pin the new failure cause in `TestREADME_ReleaseSectionDescribesTheFailedCheck`.

**Outcome**: The README's `release` section describes the exclusion check's list, pauses and failure causes as built, its tripwire pins the current text, and no code behaviour changes.

**Acceptance Criteria**:
- [ ] In the README's `release` section, the paragraph on the check's call says that once the list would take more than a tenth of `max_prompt_bytes`, it condenses by folder, with a folder's paths summed up as a count and a total diff size (§6.2, §6.3)
- [ ] The pause paragraph says that a pause shows, after the glob and the AI's reason, how many changed files the notes still carry that the glob matches, the total size of their diffs, and any of them on `[release].diff_keep` or kept earlier in the run, and that an Exclude takes those kept files out too (§6.5, §5.2)
- [ ] The no-pause sentence also covers a suggestion whose matches earlier answers in the run already decide, alongside a suggestion that matches no changed path, or only paths a `diff_exclude` level or `[release].diff_keep` already covers (§6.5)
- [ ] The failure paragraph adds a list that will not fit in `max_prompt_bytes` even condensed to the check's failure causes. It still says mint warns `couldn't check for files that don't belong in the notes`, with the AI's output beneath it where there is any, and carries on with no suggestions (§6.8)
- [ ] `TestREADME_ReleaseSectionDescribesTheExclusionPauses` pins the new pause sentence through its count, size and keep-list clause. Today's pin is still a substring of an extended sentence, so the new pin must reach past "the AI's reason", and it fails against today's README (§18.2)
- [ ] `TestREADME_ReleaseSectionDescribesTheFailedCheck` pins the new failure cause and fails against today's README. Every other pin in `readme_exclusion_check_test.go` passes unchanged (§18.2)
- [ ] Rule: no Go file changes except `internal/config/readme_exclusion_check_test.go`, and the five gates pass

**Do**:
- Edit `README.md:115`, `:117` and `:119`, matching the code as built:
  - the list's condensing: `internal/notes/condense.go:10-11` (the share is `maxPromptBytes / 10`) and `internal/notes/exclusioncheck.go:277-305` (a folder line reads `{folder}/: {count} files, {size}`)
  - the pause question: `internal/engine/release.go:1085-1109` (`{n} files, {size}`, then `on the keep list: …`; `Kept` counts both the configured keep list and this run's Keep answers, per `internal/notes/exclusioncheck.go:201-214`)
  - the skip: `internal/notes/exclusioncheck.go:160-176` and `:193-199`
  - the over-cap failure: `internal/notes/exclusioncheck.go:113-117`, where no call is made and nothing prints beneath the warning
- `internal/config/readme_exclusion_check_test.go:29`: replace the pinned pause sentence with the new one. In `TestREADME_ReleaseSectionDescribesTheFailedCheck` (:69-82), add a pin for the new failure cause.
