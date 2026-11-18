TASK: Commit messages exclude the `[commit].diff_exclude` level (token-aware-diff-sizing-7-2, tick-79bb8f)

ACCEPTANCE CRITERIA:
- Given shared `diff_exclude = ["*.lock"]`, `[commit].diff_exclude = ["tmp/"]`, and staged changes to `app.lock`, `tmp/run.log` and `src/export.go`, `mint commit`'s AI call carries the diff of `src/export.go` alone, and accepting the message commits all three files (§5.1)
- Given `[commit].diff_exclude = ["tmp/"]`, `mint commit -A`, a tracked change to `src/export.go` and an untracked `tmp/new.log`, the AI call carries `src/export.go`'s change and not `tmp/new.log`, and accepting stages and commits both (§5.1)
- Given `[commit].diff_exclude = ["tmp/"]` and a release range changing `tmp/run.log` and `src/export.go`, the release's notes diff and Change Map carry both (§5.1)
- Given `[commit].diff_exclude = ["tmp/"]` and only `tmp/run.log` staged, `mint commit` makes no AI call, prints `all changes are excluded from the AI context — opening editor` with no AI output, and opens the editor; saving a message commits `tmp/run.log` (§5.1, §17.1)
- Given `[commit].diff_exclude = "tmp/"`, a string rather than an array, any command that loads config fails before any step runs with a friendly type error in the `invalid .mint.toml: …` shape that names the `[commit]` key and its array-of-strings type (§5.1)
- `mint setup`'s config reference and the README's `[commit]` table list `diff_exclude` at the `[commit]` level with an empty default, described as globs that add to the shared list for commit messages only (§5.1, §18.1, §18.2)
- CLAUDE.md's config seam says `diff_exclude` has shared, `[release]` and `[commit]` levels whose lists add rather than override (§18.3)

STATUS: complete

SPEC CONTEXT: §5.1 splits `diff_exclude` into shared, `[release]` and `[commit]` levels. Each verb's AI diff excludes the shared list plus its own level's list, and the lists add rather than override. Release notes additionally carry the built-in always-excludes (§5.3), and commit carries no built-ins. Commit's preflight stays blind to every level, so a commit whose every file is excluded still commits, with the message written in the editor. §17.1 keeps the all-excluded editor route's line unchanged and free of AI output. §18.1–§18.3 require the setup guide's config reference (rendered from the SoT), the README's per-key tables and CLAUDE.md's config seam to describe the three additive levels.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/config/config.go:346 — `commitShape.DiffExclude []string \`toml:"diff_exclude"\``
  - internal/config/config.go:265 — `Commit.DiffExclude`; copied through in `resolveCommit` at internal/config/config.go:755
  - internal/config/config.go:486 — `typeErrorMessages["commitShape.DiffExclude"] = "commit.diff_exclude must be an array of strings"`. go-toml v2.3.1 renders the mismatch as `… into struct field config.commitShape.DiffExclude of type []string` (unmarshaler.go:189), so `translateTypeError`'s `field+" "` substring match hits it, and no other map key is a substring of it.
  - internal/config/metadata.go:154 — the `([commit], diff_exclude)` SoT row, default `[]`. The bijection count `wantPairs = 27` (internal/config/metadata_bijection_test.go:119) matches the 4+15+3+5 rows.
  - internal/commit/generate.go:126 — `g.sourceDiff(ctx, slices.Concat(cfg.DiffExclude, cfg.Commit.DiffExclude))`. Both the first generation (run.go:747) and the gate's `r` (run.go:760) go through `GenerateWithContext`, so both apply the `[commit]` list.
  - internal/commit/source.go:112-130 — `sourceArgs`/`excludePathspecs` add the tail to every source, including the `-A` untracked `ls-files` enumeration. `probeArgs` (internal/commit/preflight.go:137) appends no tail, so the preflight stays blind to every level.
  - Release side: `notesExcludeConfig` (internal/engine/regenerate_fresh.go:127-129) concatenates only `cfg.DiffExclude` and `cfg.Release.DiffExclude`. The only production reader of `cfg.Commit.DiffExclude` is internal/commit/generate.go:126.
  - README.md:280 — `[commit]` table row. README.md:235 — the shared-keys paragraph names the three additive levels.
  - internal/setupguide/setupguide.go:313-327 — the diff_exclude scope section names the `[commit]` level. The config reference renders the SoT row.
  - CLAUDE.md:37 — config seam rewritten as required.
- Notes: The comments the task named (`Config.Commit`, `Commit`, the commit `Generator`, `excludePathspecs`) now describe the shared plus `[commit]` lists. No drift from the plan.

TESTS:
- Status: Adequate
- Coverage:
  - Criterion 1: internal/commit/commit_diff_exclude_test.go:28. Asserts the unfiltered preflight probe argv, the L1 argv `diff --cached -- . :(exclude)*.lock :(exclude)tmp/` (shared list first), that the prompt carries the export diff and neither excluded path, that `git commit -F -` gets the message, and that no `git add` runs.
  - Criterion 2: internal/commit/commit_diff_exclude_test.go:77. Asserts exact argv for the tracked diff and for the untracked `ls-files` enumeration (both carry `:(exclude)tmp/`), `git add -A` on accept, and that `tmp/new.log` is absent from the prompt.
  - Criterion 3: internal/engine/release_diffexclude_test.go:120. Asserts exact argv on the notes diff and both Change Map calls (built-in excludes only, no `tmp/`), and that the notes prompt carries both paths.
  - Criterion 4: internal/commit/commit_diff_exclude_test.go:124. Asserts zero transport calls, the exact warning struct with an empty `Output`, one editor launch, and one commit carrying the saved message.
  - Criterion 5: internal/config/commit_diff_exclude_test.go:59 pins the exact `invalid .mint.toml: commit.diff_exclude must be an array of strings` message. "Before any step runs" holds through the existing config-error wiring, which every verb shares (internal/commit/run.go:304-307, internal/engine/release.go:370-373, cmd/mint/main.go:163-167).
  - Criterion 6: internal/config/commit_diff_exclude_test.go:77 covers the SoT row, internal/config/commit_diff_exclude_test.go:93 the README `[commit]` table row, and internal/setupguide/setupguide_test.go:931 and :943 the config reference row and the scope text.
  - Decoding across all three levels: internal/config/commit_diff_exclude_test.go:12.
  - Each test would fail if the feature broke: either the L1 argv tail or the release argv would change.
- Notes: No over-testing. The tests are behaviour-level and assert exact argv and exact rendered lines, per the project's test idioms. Criterion 7 (CLAUDE.md wording) has no test; it is prose, and reading confirms it at CLAUDE.md:37.

CODE QUALITY:
- Project conventions: Followed. The key takes the strict-config path (shape field, `typeErrorMessages` entry, SoT row, README row), every git call goes through `RunInDir`, the tests use the FakeRunner and RecordingPresenter with exact-argv assertions, and comments were updated in the same change.
- SOLID principles: Good
- Complexity: Low. The change is one `slices.Concat` at the single call into the existing exclusion tail.
- Modern idioms: Yes (`slices.Concat`)
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
