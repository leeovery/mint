TASK: Ask the AI which changed paths don't belong in the notes (token-aware-diff-sizing-7-5)

ACCEPTANCE CRITERIA:
- Given a forward release on the AI path whose range modifies `src/export.go` and adds `docs/design/export.md`, the AI tool receives one call before any notes call. Its prompt lists those two paths, each with its change status and the size of its diff, carries none of the diff's content, and asks for suggested globs, each with a short reason, or an explicit "none" (§6.2, §6.4, §7.2)
- Given shared `diff_exclude = ["*.lock"]`, `[release].diff_exclude = ["specs/"]`, `[release].diff_keep = ["docs/api/"]`, `version_file = "VERSION"` with no `version_pattern`, and a range that also changes `app.lock`, `specs/export.md`, `docs/api/export.md`, `.mint.toml`, `CHANGELOG.md` and `VERSION`, none of those six paths is listed (§5.2, §5.3, §6.2)
- Given changed files whose diffs are 3,100,000 bytes and 640 bytes, both are listed as ordinary entries, with sizes `3.1 MB` and `640 B` (§6.2, §16.1)
- Given `[release].ai_command = "claude -p --model opus"` and `[release].timeout = 300`, the check runs `claude` with `-p --model opus` under a 300-second per-attempt deadline. With neither key set at any level, it runs the lean shipped default under the 180-second release default (§2.2, §4.4, §6.2)
- Given the AI answers that nothing looks out of place, the release goes on to its notes, written from the same diff as with no check (§6.4)
- Given a dry run that cached the release's notes, the following real run makes the check call before it asks `Cached notes found for this diff — use them?` (§6.1, §7.2)
- Given a release that runs the check, in pretty mode the notes line shows `Checking for files that don't belong in the notes…` while the call runs, then narrates the notes as it would without the check. In plain mode the check prints as its own line before the notes' lines (§16.1, §16.2)
- Given AI notes at the review gate, choosing `r` makes no check call (§15.1)
- Given a run whose notes were written after halving, the lower-the-cap hint never suggests the check call's size (§10.3)
- The README's `release` section says that before the AI writes a release's notes, mint asks the AI which changed paths look like they don't belong in them (§18.2)

STATUS: complete

SPEC CONTEXT: §6 introduces one AI call before a forward release's notes, carrying only the changed-path list (no diff), each path not covered by the built-in always-excludes (CHANGELOG.md, .mint.toml), the plain-mode version_file exclusion, either diff_exclude level, or [release].diff_keep, listed with its change status and diff size in the §16.1 decimal format. §7.2 places it first on the AI path, before cache reuse and the size check. §2.2/§4.4: it uses the release's ai_command and timeout (180s release default). §15.1: `r` never re-runs it. §10.3 (corrigendum): only diff-carrying calls count toward the lower-the-cap hint. §16.1/§16.2/§16.4: a presenter event, pretty on the notes line, plain as its own line. §18.2: README documents it with `mint release`.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/exclusioncheck.go:123-159 (`Check`: paths from the post-exclusion diff, keep-list filter at :125, prompt composed at :129, narration around the call at :134/:136, "none" at :144)
  - internal/notes/exclusioncheck.go:273-302 (`diffPaths`/`fileChange`: path, change status from the diff's extended headers, size = the file section's bytes)
  - internal/notes/exclusioncheck.go:306-321, :342-361 (list rendering with `bytesize.Format`; prompt asking for globs with a short reason or exactly "none")
  - internal/notes/select.go:209-237, :249-259 (instructions resolved, then `checkExclusions`, then the reuse hook, then `GenerateFromDiff`)
  - internal/engine/release.go:1016-1022 (one `aiTransport(deps, cfg)` shared by the generator and the check; `WithExclusionCheck`), :1317-1322 (`aitransport.New(..., config.VerbRelease)`)
  - internal/engine/release.go:1340-1350 (`perRunRegenerator` drives `r` through the generator, never the selector, so `r` makes no check call)
  - internal/engine/notesstage.go:43-49 (`Checking`/`Checked` -> `NotesChecking`/`NotesChecked`); `CallSucceeded` (:78) is fed only by the generator's progress, so the check call never enters the lower-the-cap hint
  - internal/presenter/presenter.go:82-87, internal/presenter/pretty.go:519-527, internal/presenter/plain.go:243-249, internal/presenter/presentertest/recording.go:207-215
  - internal/notes/pathspec.go:15-30 (Go-side git pathspec matcher for keep-list coverage)
  - README.md:115 (the `release` section's paragraph on the check)
- Notes: The built-in, version_file and diff_exclude exclusions come from git via the Assembler's `:(exclude)` pathspecs (internal/notes/assemble.go:117-128), so the check lists only what the notes diff carries; keep is applied in Go. The task's Context pointed at the Change Map's `--name-status` for change status; the implementation reads it from the diff's own extended headers instead. That is a sound divergence: the check runs before the Change Map is built, it needs no extra git call, and added/deleted/renamed/modified all come out right.

TESTS:
- Status: Adequate
- Coverage:
  - Criterion 1: internal/engine/release_exclusioncheck_test.go:69 (two claude calls, check first; exact list lines; no diff content; asks for globs, reason, "none"); unit level internal/notes/exclusioncheck_test.go:88 asserts the whole prompt byte-for-byte
  - Criterion 2: release_exclusioncheck_test.go:105 asserts the git argv carries every `:(exclude)` (CHANGELOG.md, .mint.toml, *.lock, specs/, VERSION) and that the kept path present in the diff is not listed
  - Criterion 3: release_exclusioncheck_test.go:149 and exclusioncheck_test.go:137
  - Criterion 4: release_exclusioncheck_test.go:167 (table: [release] override args + ~300s deadline; unset -> lean args + ~180s)
  - Criterion 5: release_exclusioncheck_test.go:220 (notes call carries the whole diff unchanged; tag body is the notes)
  - Criterion 6: release_exclusioncheck_test.go:239 (NotesChecked precedes the cache prompt; no notes call); exclusioncheck_test.go:379 (selector order check -> reuse -> notes)
  - Criterion 7: release_exclusioncheck_test.go:270 (events) and :285 (plain lines); presenter/notescheck_test.go:26, :46, :82 (pretty spinner texts, restore, plain line, byte-purity)
  - Criterion 8: release_exclusioncheck_test.go:315
  - Criterion 9: release_exclusioncheck_test.go:381 (fixture guarantees the check prompt is larger than every succeeded diff call)
  - Criterion 10: internal/config/readme_exclusion_check_test.go:8
- Notes: Some overlap between the notes-package unit tests and the engine tests (list contents, decimal sizes), but each layer proves something the other cannot: exact prompt bytes at the unit level, wiring and argv at the engine level. Each test would fail if its behaviour broke.

CODE QUALITY:
- Project conventions: Followed (AI behind the consumer-defined `notes.Transport`; transport built once via `aitransport.New` with VerbRelease; narration only through presenter events; engine never touches spinners; external test packages, t.Parallel, FakeRunner/RecordingPresenter, exact argv assertions)
- SOLID principles: Good (`CheckNarration` is a narrow interface the notes stage satisfies; copy-on-With builders match `Selector`'s existing pattern)
- Complexity: Low
- Modern idioms: Yes (`strings.Lines`, `slices.DeleteFunc`/`ContainsFunc`, range-over-int in tests)
- Readability: Good; doc comments on `Check`, `ExclusionCheck`, `checkExclusions` and the presenter events match the code as built
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
