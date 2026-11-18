TASK: The check runs only on a forward release with paths to raise (token-aware-diff-sizing-7-6)

ACCEPTANCE CRITERIA:
- Given `mint release --no-ai`, no check call is made and no check line shows, and the body is the commit-subject list (§6.1, §14.1)
- Given `mint release --notes-file notes.md`, no check call is made and no check line shows, and the supplied body ships (§6.1, §13.3)
- Given a first release, no check call is made, and the body is `Initial release.` (§6.1, §7.1)
- Given a range whose only changes are to `CHANGELOG.md`, `.mint.toml` or paths a `diff_exclude` level matches, no check call is made, and the release gets the maintenance-release body (§6.1, §7.1)
- Given `[release].diff_keep = ["docs/api/"]`, shared `diff_exclude = ["*.lock"]`, and a range changing only `docs/api/export.md` and `app.lock`, no check call is made and no check line shows, and the notes call carries `docs/api/export.md`'s diff (§5.2, §6.1)
- Given `mint release regenerate <version>` or `mint release regenerate --all` from a fresh source, attended or under `-y`, no check call is made and no check line shows for any version (§6.1)
- Given `mint commit`, no check call is made (§6.1)

STATUS: complete

SPEC CONTEXT: §6.1 confines the exclusion check to `mint release` on the AI path with at least one uncovered changed path: skipped under `--no-ai`, `--notes-file`, a first release, an empty post-exclusion diff, or when every changed path is covered by an exclusion or the keep list. Regenerate (any source, single or `--all`) and `mint commit` never run it. §7.1 fixes the body precedence (supplied > first release > empty diff > --no-ai > AI path); §7.2 puts the check first on the AI path, forward releases only; §5.2 says a keep-listed path is never raised; §13.3 says a supplied body skips the check and every AI call.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/select.go:183-243 — `SelectBodyWithReuse` returns before the check for a supplied body (184-186), a first release (188-190), a degenerate post-exclusion diff (197-199) and `--no-ai` (201-207); the check runs only at 214 via `checkExclusions` (249-259), and is a no-op when no check is wired (250-252).
  - internal/notes/exclusioncheck.go:123-128 — `Check` drops keep-covered paths and returns with no call and no narration when none remain; the post-exclusion diff already omits built-in, shared and `[release]` `diff_exclude` paths, so "every path covered by an exclusion or the keep list" collapses to this early return.
  - internal/engine/release.go:1020-1022 — the one `NewSelector(...).WithExclusionCheck(...)` site (the only non-test caller of both `NewSelector` and `WithExclusionCheck` in the tree), on the forward release's `resolveBody`.
  - internal/engine/regenerate_fresh.go:66, 99-101, 120-123 — the fresh source builds its body and its `r` regenerator through `freshGenerator` directly, never a Selector, so no check is reachable on regenerate.
  - internal/commit — no reference to `ExclusionCheck` or the check narration in non-test code; commit generates through its own generator.
- Notes: Precedence matches §7.1 exactly; the check sits after `--no-ai` and the degenerate guard and before cache reuse, as §7.2 orders it. No drift.

TESTS:
- Status: Adequate
- Coverage:
  - internal/engine/release_exclusioncheck_skip_test.go:23 (`--no-ai`: no check call or narration, no claude call, tag body is the commit-subject list), :53 (`--notes-file` with a keep list configured: no check, no AI work, supplied body byte-identical in tag/CHANGELOG/provider), :72 (first release: no check, `Initial release.`), :94 (table over built-ins, shared `diff_exclude`, `[release].diff_exclude`: asserts the exact diff pathspecs, no check, maintenance-release body), :150 (keep `docs/api/` + shared `*.lock`: pathspecs asserted, no check, exactly one notes call carrying `docs/api/export.md`'s diff, AI body ships).
  - cmd/mint/regenerate_exclusioncheck_test.go:58 and :91 — single version and `--all`, each attended and `-y`, with a config whose lists leave the changed paths uncovered (so a wired check would fire), asserting claude ran exactly once per version with that version's diff and no check narration.
  - internal/commit/commit_diff_exclude_test.go:167 — commit makes exactly one AI call, over the staged diff, with no check narration.
  - Unit-level backing: internal/notes/exclusioncheck_test.go:176 (keep list covering every path makes no call and narrates nothing).
- Notes: Each test would fail if its skip broke — the regenerate and keep-list cases deliberately leave a path uncovered or seed a single claude outcome, so a stray check call changes the call count or consumes the notes answer. No redundant or over-mocked tests worth acting on.

CODE QUALITY:
- Project conventions: Followed (FakeRunner/RecordingPresenter doubles, external test packages, t.Parallel, exact argv on git pathspecs)
- SOLID principles: Good — the selector owns the precedence, the check owns its own uncovered-path filter, wiring is confined to one forward-release site
- Complexity: Low
- Modern idioms: Yes (slices.DeleteFunc/ContainsFunc, copy-on-With builders)
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
