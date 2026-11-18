TASK: `[release].diff_keep` config key (token-aware-diff-sizing-7-4)

ACCEPTANCE CRITERIA:
- Given `[release].diff_keep = ["docs/api/"]` and a release range changing `docs/api/export.md` and `src/export.go`, the notes call carries the same diff and Change Map as with no `diff_keep` set (§5.2)
- Given `[release].diff_keep = ["docs/api/"]` with `[release].diff_exclude = ["docs/"]`, or with shared `diff_exclude = ["docs/"]`, a change to `docs/api/export.md` stays out of the notes diff and Change Map (§5.2)
- Given `[release].diff_keep = "docs/api/"` (a string), any command that loads config fails before any step runs with a friendly type error in the `invalid .mint.toml: …` shape naming the `[release]` key and its array-of-strings type (§5.2)
- Given `diff_keep` at the top level or under `[commit]`, loading fails with the as-built unknown-key message naming the key and where it sits (§5.2)
- `mint setup`'s config reference and the README's `[release]` table list `diff_keep` at the `[release]` level with an empty default, described as globs whose paths the exclusion check never raises, which add nothing to the diff and never override an exclusion (§5.2, §18.1, §18.2)

STATUS: complete

SPEC CONTEXT: §5.2 defines `[release].diff_keep` as an array of pathspec globs, default empty, whose paths the exclusion check (§6) never raises; it does nothing else — adds nothing to the diff, never overrides an exclusion — and takes the strict-config path (schema field, friendly type error, metadata row, README row). §18.1 has the setup guide's config reference render from the metadata SoT; §18.2 adds the README `[release]` row and notes the README tripwire catches a missing `diff_keep`. Consumers (check list, skip rule, suggestion filter, pause counts, Phase 8 Keep append) belong to later tasks.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/config/config.go:364 — `DiffKeep []string \`toml:"diff_keep"\`` in `releaseShape`
  - internal/config/config.go:224 — `Release.DiffKeep`; doc comment at :206-208
  - internal/config/config.go:726 — copied through in `resolveRelease`
  - internal/config/config.go:481 — `typeErrorMessages` entry `"releaseShape.DiffKeep": "release.diff_keep must be an array of strings"`
  - internal/config/metadata.go:140 — SoT row at `LevelRelease`, Default `[]`
  - README.md:262 — `[release]` table row
- Notes: The only production reader of `Release.DiffKeep` besides config is the exclusion check (internal/engine/release.go:1064, `notes.NewExclusionCheck(transport, cfg.Release.DiffKeep, stage)`) plus the Phase 8 answers writer (internal/config/appendrelease.go) — the notes-diff pathspec assembly never reads it, so "adds nothing to the diff / never overrides an exclusion" holds structurally. Keep matching is in-process (internal/notes/exclusioncheck.go `coveredBy`), so no extra git invocation appears on the notes range. Unknown-key rejection for top-level / `[commit]` falls out of `DisallowUnknownFields` with no new code, which is the as-built path the criterion asks for. initgen correctly untouched (template carries no per-key entries).

TESTS:
- Status: Adequate
- Coverage:
  - internal/config/release_diff_keep_test.go:12 — decode table: absent → nil, keep-only, beside an exclusion (asserts `DiffExclude` untouched)
  - internal/config/release_diff_keep_test.go:49 — exact type-error string for a scalar `diff_keep`
  - internal/config/release_diff_keep_test.go:65 — exact unknown-key messages at top level and in `[commit]`
  - internal/config/release_diff_keep_test.go:97 — SoT row default and description
  - internal/config/release_diff_keep_test.go:113 — README `[release]` section carries the row
  - internal/setupguide/setupguide_test.go:974 — rendered config reference carries the `[release]` row with `[]` default and the SoT description
  - internal/engine/release_diffkeep_test.go:47 — AC1: runs the release with and without `diff_keep`, asserts identical notes-range git argv (diff + both Change Map calls, count pinned at 3) and an identical notes prompt that still carries both paths
  - internal/engine/release_diffkeep_test.go:74 — AC2: both `[release]` and shared `diff_exclude = ["docs/"]` variants, asserting exact argv (via `invokedWith` exact command-line match) on the notes diff and both Change Map calls carry `:(exclude)docs/` and nothing from the keep list
  - Bijection/census/drift tests updated to 27 pairs with the new `([release], diff_keep)` census entry
- Notes: The AC2 test's "prompt does not contain docs/api/export.md" check is near-tautological against a scripted git that returns post-exclusion output, but the load-bearing assertion is the exact pathspec argv, which would fail if keep globs leaked into the git call. Not over-tested: each test targets one criterion.

CODE QUALITY:
- Project conventions: Followed — strict-config path per CLAUDE.md seam 6 (schema field, `typeErrorMessages`, `MetadataRows()` row, README row); external test packages, `t.Parallel()`, table-driven, exact error strings and exact git argv asserted
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: None. The `Release.DiffKeep` doc comment (config.go:206-208) is true to as-built.

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
