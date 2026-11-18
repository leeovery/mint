TASK: `.mint.toml` never reaches release notes (token-aware-diff-sizing-7-3)

ACCEPTANCE CRITERIA:
- Given no `diff_exclude` at any level and a release range changing `.mint.toml` and `src/export.go`, the notes call carries the diff of `src/export.go` alone, and the Change Map does not name `.mint.toml` (§5.3)
- Given a range whose only change is `.mint.toml`, the release gets the maintenance-release body with no AI call (§5.3, §7.1)
- Given `mint release regenerate <version>` or `mint release regenerate --all` from a fresh source, over a version whose range changed `.mint.toml`, that version's notes diff and Change Map leave it out (§5.3)
- Given a staged change to `.mint.toml`, `mint commit`'s AI call carries its diff (§5.3)
- Given `version_file = "VERSION"` with no `version_pattern`, a regenerated version whose range changed `VERSION` leaves it out of its notes diff; given a `version_pattern`, a changed version file stays in, as built (§5.3)
- Where the shared `diff_exclude` row in `mint setup`'s config reference names the built-in release-notes exclusions, it names `.mint.toml` beside `CHANGELOG.md` (§5.3, §18.1)

STATUS: issues_found

SPEC CONTEXT: §5.3 makes `.mint.toml` a built-in, non-configurable always-exclude beside `CHANGELOG.md` for every release-notes diff and Change Map, on forward releases and regenerate alike (exclusion is path-based). The reason is that the exclusion check (§6.6) commits `.mint.toml` inside the release it answered for, and without the exclude that tooling change would reach the release's notes and any later regenerate. Commit's AI diff keeps no built-in excludes, so a config change still reaches its commit message. The plain-mode `version_file` exclusion is unchanged. §7.1 routes an empty post-exclusion diff to the maintenance body. §18.1/§18.3 keep the SoT row and code comments true in the same change.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/assemble.go:29-32: `builtinExcludePathspecs` holds `":(exclude)" + record.ChangelogFileName` and `":(exclude)" + config.FileName`.
  - internal/notes/assemble.go:117-128: `excludePathspecs` puts the built-ins first, then the globs, then the plain-mode version_file entry.
  - internal/notes/assemble.go:183-191: `AssembleRange` uses the shared pathspecs and runs at root.
  - internal/notes/changemap.go:48-66: both Change Map git calls use the same `excludePathspecs`.
  - internal/engine/regenerate_fresh.go:120-133 and internal/engine/release.go:1016: both the forward and the regenerate generator build their Assembler from `notesExcludeConfig`.
  - internal/commit/source.go:116-130: commit maps only the configured globs and has no built-ins.
  - internal/config/metadata.go:120: the shared `diff_exclude` row now names "the built-in CHANGELOG.md and .mint.toml release-notes exclusions".
- Notes: The exclude lives inside `Assembler.excludePathspecs`, so every Assembler carries it, whatever the call site: forward, `r`, the re-assembly after Exclude answers (internal/notes/select.go:249-259), regenerate single-version and `--all`. No release-notes path can bypass it. Git does the matching, and the calls run at the repo root, so the pattern matches exactly the root `.mint.toml` that `config.Load` reads. The exclusion check works from the post-exclusion diff (internal/notes/select.go:253), so it never lists `.mint.toml`. The comments the task named (`ExcludeConfig`, `NewAssembler`, `AssembleDiff`, `AssembleRange`, `BuildChangeMap`/`BuildChangeMapForRange`, `Config.DiffExclude`, `resolveBody`, degenerate.go, commit/generate.go, commit/source.go, regenerate_fresh.go header) no longer call CHANGELOG.md the one built-in. The README and the setup-guide prose state no built-in list, so nothing there became false.

TESTS:
- Status: Adequate
- Coverage:
  - AC1: internal/engine/release_builtin_exclude_test.go:13 runs with an empty config and checks the exact argv on the notes diff and on both Change Map calls (`:(exclude)CHANGELOG.md`, `:(exclude).mint.toml`, nothing else). It also checks the prompt carries the `src/export.go` diff.
  - AC2: release_builtin_exclude_test.go:40 covers an empty post-exclusion diff with the `.mint.toml` exclude in the argv. It checks for zero `claude` invocations and that the tag annotation carries the maintenance body.
  - AC3 single version: release_builtin_exclude_test.go:70 (engine) and cmd/mint/regenerate_all_test.go:451, whose shared `assertFreshGitCallsExclude` now includes `.mint.toml`.
  - AC3 `--all`: cmd/mint/regenerate_all_test.go:466 checks the exact three fresh git calls.
  - AC4: internal/commit/generate_test.go:1057 checks the bare `git diff --cached -- .` argv and that the staged `.mint.toml` diff is in the prompt.
  - AC5: release_builtin_exclude_test.go:90, a table covering plain mode (VERSION excluded) and embedded mode (kept).
  - AC6: internal/setupguide/setupguide_test.go:957 checks the rendered `| diff_exclude | top-level |` row names both files.
  - The unit argv helpers (assemble_test.go `wantArgs`, changemap_test.go `wantNameStatusArgs`/`wantNumstatArgs`, range_test.go `wantRangeArgs`) were updated, so the built-in pair is pinned at the unit level too.
- Notes: Each test would fail if the `.mint.toml` entry were dropped, since the exact-argv assertions follow the project's idiom. The "prompt does not name `.mint.toml`" checks (release_builtin_exclude_test.go:35, :85) cannot fail under the FakeRunner, because git's exclusion is not simulated. They are harmless; the argv assertions carry the proof.

CODE QUALITY:
- Project conventions: Followed. Git does the exclusion through `RunInDir(root, …)`; the change is drift-safe because the names come from `config.FileName` and `record.ChangelogFileName`; the SoT row was edited at the source.
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: Three test names and messages still say CHANGELOG.md is the only exclude (see FINDINGS).

BLOCKING ISSUES:
- None

FINDINGS:
- [in-scope] [contained] internal/engine/regenerate_fresh_test.go:161 — This task changed the assertion two lines above to expect `:(exclude)CHANGELOG.md` plus `:(exclude).mint.toml`, but the failure message still reads "want CHANGELOG.md exclude only". Two test names make the same claim: `TestAssembler_AssembleDiff_AbsentDiffExclude_ExcludesOnlyChangelog` (internal/notes/assemble_test.go:272) and `TestAssembler_BuildChangeMap_AbsentDiffExclude_ExcludesOnlyChangelog` (internal/notes/changemap_test.go:350). This task updated both bodies, and their own comments now say "the only excludes are the built-ins". Fix: change the message to "want the built-in excludes only", and rename both tests to `..._ExcludesOnlyTheBuiltIns`. — FAILS: if a regression drops the `.mint.toml` exclude, line 161 fails while stating "CHANGELOG.md exclude only" as the expectation. That is exactly the regressed argv, so it points the fixer at the wrong target. The two names also advertise the contract this task removed, against CLAUDE.md's rule that comments stay true to as-built.

UNSETTLED:
- None
