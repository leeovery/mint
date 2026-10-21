TASK: Release notes exclude the `[release].diff_exclude` level (token-aware-diff-sizing-7-1, tick-4694f0)

ACCEPTANCE CRITERIA:
- Given shared `diff_exclude = ["*.lock"]`, `[release].diff_exclude = ["specs/"]`, and a release range changing `app.lock`, `specs/export.md` and `src/export.go`, the notes call carries the diff of `src/export.go` alone, and its Change Map names neither `app.lock` nor `specs/export.md` (§5.1)
- Given `[release].diff_exclude = ["specs/"]` and no shared list, `specs/` paths stay out of the notes diff and Change Map. Given both lists, a path only the shared list matches stays out too: the `[release]` list adds to the shared list and never replaces it (§5.1)
- Given a range whose only changes match `[release].diff_exclude`, the release gets the maintenance-release body with no AI call (§5.1, §7.1)
- Given `mint release regenerate <version>` from a fresh source, or `mint release regenerate --all` from a fresh source, each version's notes diff and Change Map leave out the paths `[release].diff_exclude` matches (§5.1, §6.1)
- Given `[release].diff_exclude = ["specs/"]` and a staged change to `specs/export.md`, `mint commit`'s AI call carries that change's diff (§5.1)
- Given `[release].diff_exclude = "specs/"`, a string rather than an array, any command that loads config fails before any step runs with a friendly type error in the `invalid .mint.toml: …` shape that names the `[release]` key and its array-of-strings type (§5.1)
- `mint setup`'s config reference and the README's `[release]` table list `diff_exclude` at the `[release]` level with an empty default, described as globs that add to the shared list for release notes only (§5.1, §18.1, §18.2)
- `mint setup`'s "diff_exclude scope" etiquette section and its procedure step 4 send noise only release notes should skip, such as `.workflows/`, `.claude/`, agent directories and `docs/`, to `[release].diff_exclude`, and keep the shared list for what neither verb should read, such as lockfiles and build output (§18.1)
- Neither the README's shared engine keys section nor CLAUDE.md's config seam says `diff_exclude` is shared-only (§18.2, §18.3)

STATUS: complete

SPEC CONTEXT: §5.1 splits `diff_exclude` into shared, `[release]` and `[commit]` levels whose lists add (never override): the release-notes diff (forward release and regenerate's fresh source) excludes the built-ins, the shared list and `[release]`'s; commit's AI diff excludes shared plus `[commit]` only. Entries stay git pathspec globs matched by git, and the Change Map is built over the same post-exclusion set. §6.1/§7.2: regenerate applies the `[release]` lists as they stand, with no exclusion check. §7.1: an empty post-exclusion diff gets the maintenance-release body. §18.1-§18.3: setup guide advice, README tables/shared-keys prose and CLAUDE.md's config seam stop calling `diff_exclude` shared-only; the new key takes the strict-config path (schema field, typeErrorMessages entry, MetadataRows row, README row).

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/config/config.go:363 (`releaseShape.DiffExclude`), :223 (`Release.DiffExclude`), :725 (resolveRelease copy), :480 (typeErrorMessages entry `release.diff_exclude must be an array of strings`); comments updated at :105-113, :142-147, :203-204
  - internal/config/metadata.go:139 (SoT row: `[release]`, default `[]`, "pathspec globs kept out of the release-notes diff only, added to the shared diff_exclude list")
  - internal/engine/regenerate_fresh.go:127-133 (`notesExcludeConfig`: `slices.Concat(cfg.DiffExclude, cfg.Release.DiffExclude)`), consumed by both production Assembler construction sites: internal/engine/release.go:1016 (forward, resolveBody) and internal/engine/regenerate_fresh.go:121 (freshGenerator, serving single-version and --all)
  - internal/notes/assemble.go:74-77 (ExcludeConfig.Globs comment), :117-128 (excludePathspecs, shared by the diff and both Change Map calls via internal/notes/changemap.go:49)
  - internal/commit/generate.go:126 (commit's AI diff reads `cfg.DiffExclude` + `cfg.Commit.DiffExclude`, never `cfg.Release.DiffExclude`)
  - internal/setupguide/setupguide.go:224-226 (procedure step 4), :313-331 (diff_exclude scope)
  - README.md:235 (shared engine keys prose), :242 (shared row), :261 (`[release]` row)
  - CLAUDE.md:37 (config seam names the three additive levels)
- Notes: The plan's `freshExcludeConfig` was renamed to `notesExcludeConfig` and is now shared by the forward and regenerate paths, so both read one definition of the release-notes exclusion set. That is a sound divergence from the task wording, not a loss. `slices.Concat` allocates a new slice, so the later `Assembler.Exclude` (assemble.go:106-108) cannot alias into the config's backing array. Ordering is built-ins, then shared, then `[release]`, then plain-mode version_file, which matches the as-built order the task states. The go-toml v2.3.1 type-mismatch text ("struct field config.releaseShape.DiffExclude of type []string") matches the `releaseShape.DiffExclude ` substring lookup, and the lookup cannot cross-match `fileShape.DiffExclude` or `commitShape.DiffExclude`.

TESTS:
- Status: Adequate
- Coverage:
  - AC1/AC2: internal/engine/release_diffexclude_test.go — `TestRelease_ReleaseDiffExclude_AddsToTheSharedListOnTheNotesDiffAndChangeMap` checks the exact argv on the notes diff and both Change Map git calls (`:(exclude)*.lock`, `:(exclude)specs/` after the built-ins), and that the notes prompt (exclusion-check call filtered out) carries the src/export.go diff. `TestRelease_ReleaseDiffExclude_AppliesWithNoSharedList` covers the `[release]`-only case.
  - AC3: `TestRelease_ReleaseDiffExclude_OnlyExcludedChangesGiveTheMaintenanceBody` checks that no claude invocation runs and that the tag annotation is the maintenance body.
  - AC4: internal/engine/regenerate_fresh_test.go:611, :629 (RegenerateFreshBody and the `r` regenerator); cmd/mint/regenerate_all_test.go `TestNewBatchBodyProducer_Fresh_LeavesOutReleaseDiffExcludePaths` (--all) and `TestNewRegenerateBodyProducer_Fresh_LeavesOutReleaseDiffExcludePaths` (single version), both asserting the full git argv sequence.
  - AC5: internal/commit/generate_test.go `TestGenerator_Generate_ReleaseDiffExcludeNeverReachesTheCommitDiff` checks the bare `diff --cached -- .` argv and that the prompt carries the specs diff.
  - AC6: internal/config/release_diff_exclude_test.go `TestLoad_ReleaseDiffExcludeScalar_RejectedNamingTheReleaseKeyAndArrayType` checks the exact error string. The decode table covers absent, release-only and both lists.
  - AC7: `TestMetadataRows_ReleaseDiffExcludeRow`, `TestREADME_ReleaseTableListsDiffExclude`, and internal/setupguide/setupguide_test.go `TestGuide_ConfigReferenceListsReleaseDiffExclude`.
  - AC8: `TestGuide_DiffExcludeScopeSplitsTheLevels` and `TestGuide_ProcedureStepFourRoutesNoiseByLevel`.
  - AC9: `TestREADME_SharedEngineKeysDoNotCallDiffExcludeSharedOnly`. CLAUDE.md:37 verified by reading.
- Notes: The engine tests' "prompt does not name app.lock / specs/export.md" checks pass by construction, because the fake runner never returns those paths. The argv assertions on all three git calls carry the proof, consistent with the project's "git does the matching, assert exact argv" convention. No under- or over-testing that matters.

CODE QUALITY:
- Project conventions: Followed (strict-config path complete; exclusion stays git-side via `:(exclude)` pathspecs; reads run at root via RunInDir; external test packages, t.Parallel, exact argv assertions)
- SOLID principles: Good — one function defines the release-notes exclusion set for both consumers
- Complexity: Low
- Modern idioms: Yes (`slices.Concat`)
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
