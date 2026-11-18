# Consolidation Findings: Token Aware Diff Sizing (Phase 7)

## Findings

### F1: The release-notes built-in exclude retypes `.mint.toml` (and `CHANGELOG.md`) instead of using the owning packages' names
- **Class**: duplication
- **Failure**: Spec §5.3 relies on one rule: the file mint reads as its config never reaches release notes. That is why Phase 8's answers commit, which changes only `.mint.toml`, stays out of every release's notes and Change Map and out of any later regenerate. After this phase the name exists in three places. `internal/config` holds it as an unexported constant for loading, `internal/engine/init.go` declares its own copy for `mint init`, and 7-3 added a third as a literal in the notes built-in pathspecs. Phase 8 is about to add more sites: 8-1 writes the file, and 8-2 stages it by path. If the config file is renamed, or a second name is supported, in config and init but not in notes, the notes keep excluding the old name. The new config file then reaches the notes AI diff and Change Map. Every attended release that records answers gets a tooling line in its published notes, and so does every regenerate of that version. Nothing fails at compile time or in tests: `release_builtin_exclude_test.go` pins the literal `:(exclude).mint.toml` argv, so it passes against the stale name. It would only be noticed in the published notes. `CHANGELOG.md` sits in the same literal list even though `record` exports the name, so it carries the same risk.
- **Evidence**:
  - internal/notes/assemble.go:22-28: `builtinExcludePathspecs = []string{":(exclude)CHANGELOG.md", ":(exclude).mint.toml"}`. The `.mint.toml` entry is new in 7-3; the `CHANGELOG.md` entry was moved here from the old `changelogExcludePathspec`.
  - internal/config/config.go:41-42: `const configFileName = ".mint.toml"` (unexported), used by Load and every config error message (383-857).
  - internal/engine/init.go:82-85: its own `configFileName = ".mint.toml"`, consumed at :118.
  - internal/record/changelog.go:20: the exported `ChangelogFileName = "CHANGELOG.md"`.
  - internal/engine/release_builtin_exclude_test.go:30,56: the tests pin the literal argv.
  - Import graph: notes already imports both config (select.go) and record (select.go's `record.FirstReleaseBody`), and config imports only cmdline, so the change adds no import cycle.
- **Proposed shape**:
  - Export the config file name from `internal/config` (for example `config.FileName`) and use it for Load and its error messages.
  - Have `internal/engine/init.go` use it instead of its own constant.
  - Build `builtinExcludePathspecs` as `":(exclude)" + record.ChangelogFileName` and `":(exclude)" + config.FileName`.
  - The argv is byte-identical, so the existing tests stay green unchanged. A later rename then changes the argv and fails those tests loudly instead of drifting silently. Phase 8's write and `git add` sites can use the same export.
- **Bank**:
  - 7-3 executor: the `.mint.toml` filename is written out separately in config loading, `mint init` and the release-notes built-in exclusion.
  - 7-3 reviewer: the built-in excluded filenames are retyped as literals in notes, away from the packages that own them (`config.configFileName`, `record.ChangelogFileName`).

### F2: The README's exclusion-check text was written before 7-8 and 7-13 and now misdescribes the pause, the list and the failure causes
- **Class**: drift
- **Failure**: The sibling tasks describe the same surface differently. 7-10's unattended paragraph spells out what a suggestion matches: the count, the total diff size, and any matched files on `[release].diff_keep`. The pause paragraph from 7-7 says a pause shows only "the glob and the AI's reason". 7-8 then added the count, size and kept-file names to the pause, plus the skip when earlier answers already decide every match, and left the README as it was. 7-13 added folder condensing to the check's list, and a fifth failure case: the list does not fit in `max_prompt_bytes` even condensed. Both came after 7-12 wrote the README paragraph listing the failure causes. This leaves operators and agents reading the release section with three gaps:
  - They are not told that a pause names the kept files an Exclude would also remove. Keep never overrides an exclusion (§5.2), and those names are what tell the operator so.
  - They are not told why a later suggestion raised no pause.
  - On a very large release they get `couldn't check for files that don't belong in the notes` with nothing beneath it, a cause the README never lists, so the docs cannot explain why the check never ran.

  §18 requires the exclusion check and its pauses to be documented as built. Only a docs-versus-behaviour review would catch this. `readme_exclusion_check_test.go` pins the stale pause sentence word for word, so it asserts the outdated text. No Phase 8 task touches these sentences; 8-2 only adds the recording sentence beside them.
- **Evidence**:
  - README.md:115: the check's list is described as "each with its change and the size of its diff", with no mention of condensing by folder.
  - internal/notes/exclusioncheck.go:279-295: `checkList` condenses by folder past `condenseShare`, and `composeCheckPrompt` describes the folder summaries.
  - README.md:117: the pause shows "the glob and the AI's reason". The no-pause sentence lists only `diff_exclude` levels and `[release].diff_keep`.
  - internal/engine/release.go:1085-1109: the pause question carries `describeMatches`, which gives the file count, the decimal size and "on the keep list: …".
  - internal/notes/exclusioncheck.go:160-176 and :195-200: `pause` skips a suggestion when `leavesUndecided` finds every match already decided by earlier Keep or Exclude answers.
  - README.md:119: the failure causes are the call failing, a timeout, no AI tool, and an unusable answer. The list that will not fit is missing.
  - internal/notes/exclusioncheck.go:113-117: a prompt over `maxPromptBytes` calls `CheckFailed("")` without making the call.
  - internal/config/readme_exclusion_check_test.go:29: pins "…, showing the glob and the AI's reason".
- **Proposed shape**:
  - README.md:117:
    - After "showing the glob and the AI's reason", add how many changed files the notes still carry that it matches, the total size of their diffs, and any of them on `[release].diff_keep` or kept earlier in the run. Say an Exclude takes those out too.
    - Extend the no-pause sentence to cover a suggestion whose matches earlier answers in the run already decide.
  - README.md:115: say that once the list would take more than a tenth of `max_prompt_bytes`, it condenses by folder, with a folder's paths summed up as a count and a total diff size.
  - README.md:119: add "or because the list will not fit in `max_prompt_bytes` even condensed" to the causes.
  - Update the pinned sentence in readme_exclusion_check_test.go:29 at the same time. If wanted, add a pin for the new failure cause in `TestREADME_ReleaseSectionDescribesTheFailedCheck`.
  - This is documentation only, and no code behaviour changes.
- **Bank**:
  - 7-8 reviewer: the README pause paragraph says nothing of the count, the size, the kept-file names, or the skip that earlier answers cause.

## Comment Corrections

- internal/engine/regenerate_fresh.go:7: the forward Stage-4 path now runs the exclusion check first, and the fresh path never does, so it is no longer "ONE substitution".
  OLD:
  ```
  // The fresh path is the forward Stage-4 AI path with ONE substitution: the diff base.
  ```
  NEW:
  ```
  // The fresh path is the forward Stage-4 AI path less the exclusion check, over its own diff base.
  ```

- internal/notes/assemble.go:108-111: `Assembler.Exclude` (7-9) appends the check's Exclude-answered globs after the configured ones, so "one :(exclude)<glob> per configured diff_exclude glob (in config order)" is false for every diff and Change Map assembled after a pause. The order is visible in the code below, so delete the enumeration.
  OLD:
  ```
  // diff and Change Map git calls — the UNION of the exclusion tiers in a fixed order:
  // FIRST the built-in always-excludes, THEN one :(exclude)<glob> per configured
  // diff_exclude glob (in config order), THEN — only in PLAIN mode — the strategy-aware
  // :(exclude)<version_file> (embedded mode / no version_file appends nothing here). git
  ```
  NEW:
  ```
  // diff and Change Map git calls — the UNION of the exclusion tiers. git
  ```

- internal/notes/assemble.go:148-151: AssembleDiff's doc repeats the same composition, with "per configured diff_exclude glob". It is the very call `checkExclusions` makes after `Exclude`.
  OLD:
  ```
  // excludePathspecs is `:(exclude)CHANGELOG.md :(exclude).mint.toml` followed by one
  // `:(exclude)<glob>` per configured diff_exclude glob, and — only in PLAIN mode — the strategy-aware
  // `:(exclude)<version_file>` (see excludePathspecs), run cwd-relative like the other
  // engine git calls. GIT
  ```
  NEW:
  ```
  // It runs cwd-relative like the other engine git calls. GIT
  ```

- internal/notes/changemap.go:18-21: BuildChangeMap's doc repeats the composition with "per configured diff_exclude glob". The forward Change Map is built after `Exclude`.
  OLD:
  ```
  // AFTER the SAME exclusion set the diff uses — the built-in always-excludes
  // (CHANGELOG.md, .mint.toml) plus one :(exclude)<glob> per configured diff_exclude glob plus, in PLAIN mode, the
  // strategy-aware :(exclude)<version_file> (the shared excludePathspecs) rides on BOTH —
  // so excluded churn never appears in the map:
  ```
  NEW:
  ```
  // AFTER the SAME exclusion set the diff uses (the shared excludePathspecs, on BOTH),
  // so excluded churn never appears in the map:
  ```

- internal/notes/changemap.go:45-46: BuildChangeMapForRange's doc says the set is "the built-ins + configured globs + strategy version_file". It now also holds Exclude-answered globs.
  OLD:
  ```
  // SAME post-exclusion set the diff uses (the shared excludePathspecs: the built-ins +
  // configured globs + strategy version_file), so the map is computed AFTER exclusion
  ```
  NEW:
  ```
  // SAME post-exclusion set the diff uses (the shared excludePathspecs), so the map is
  // computed AFTER exclusion
  ```
