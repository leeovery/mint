TASK: A pause counts the files the notes still carry (token-aware-diff-sizing-7-8, tick-90c5ed)

ACCEPTANCE CRITERIA:
- Given a release changing `docs/design/a.md` (a 12,000-byte diff), `docs/design/b.md` (3,000 bytes), `docs/api.md` (2,000 bytes), `docs/old/c.md` and `docs/VERSION`, with `[release].diff_keep = ["docs/api.md"]`, `[release].diff_exclude = ["docs/old/"]` and `version_file = "docs/VERSION"` with no `version_pattern`, the pause on `docs/` shows 3 files totalling `17 KB` and names `docs/api.md` as on the keep list. Neither `docs/old/c.md` nor `docs/VERSION` is counted (§6.5)
- Given that release also changes `README.md`, and the suggestions are `docs/design/`, answered Exclude, then `*.md`, the pause on `*.md` counts `README.md` and `docs/api.md`, and neither `docs/design/` file (§6.5)
- Given that release also changes `docs/guide.md`, and the suggestions are `docs/design/`, answered Keep, then `docs/`, the pause on `docs/` counts `docs/design/a.md`, `docs/design/b.md`, `docs/guide.md` and `docs/api.md`, and names the two `docs/design/` files and `docs/api.md` as on the keep list (§6.5)
- Given the suggestions `docs/`, answered Exclude or Keep, then `docs/design/`, no pause shows for `docs/design/` (§6.5)
- Given the suggestions `docs/design/`, answered Keep, then `docs/design/a.md`, no pause shows for `docs/design/a.md`: its only match is now on the keep list (§5.2, §6.5)

STATUS: complete

SPEC CONTEXT: §6.5 (with the 2026-10-19 corrigenda on the count and the skip) has each pause show how many changed files the glob matches and the total of their diff sizes, each measured as §6.2 measures a path, over the paths the notes diff still carries at that pause: paths taken out by the built-ins, the plain-mode version_file, either diff_exclude level or an earlier Exclude answer are not counted; kept paths are counted and named, an earlier Keep answer's glob counting as on the keep list. A suggestion is skipped when earlier answers cover every uncovered changed path it matches, so kept files alone raise no pause. §5.2: keep never overrides an exclusion. §16.1 corrigendum: sizes in decimal units (17,000 bytes reads `17 KB`).

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/exclusioncheck.go:57-64 — `Matches` (Paths, DiffBytes, Kept)
  - internal/notes/exclusioncheck.go:123-125 — `files` is every path of the post-exclusion notes diff (kept paths included); `paths` drops the keep list for the call and suggestion filtering
  - internal/notes/exclusioncheck.go:176-197 — `pause`: skips a suggestion via `leavesUndecided`, hands the pause `decided.matches(...)`, records each answer
  - internal/notes/exclusioncheck.go:199-241 — `runAnswers` (keep list cloned with `slices.Clip` and grown by Keep answers; Exclude globs), `leavesUndecided`, `matches` (skips paths an earlier Exclude covers, sums `diffBytes`, marks kept paths)
  - internal/notes/exclusioncheck.go:273-279 — `diffPaths`: `diffBytes: len(file)`, the same measure `checkList` writes on the check's list (line 310)
  - internal/notes/select.go:192, 214, 253 — the diff the check sees is the assembler's post-exclusion diff (built-ins, both diff_exclude levels, plain-mode version_file pathspec from internal/notes/assemble.go:53-63)
  - internal/engine/release.go:1210-1234 — `exclusionPauseGate` / `describeMatches`: `N file(s), {bytesize.Format}` plus `; on the keep list: a, b`
- Notes: Each criterion traces through the code. Criterion 1: docs/old/c.md and docs/VERSION never reach `files`, since git drops them through the `:(exclude)` pathspecs; 2,000+12,000+3,000 = 17,000 renders `17 KB` (internal/bytesize/bytesize.go:10-26). Criterion 2: `matches` skips paths `a.excluded` covers. Criterion 3: `a.keep` holds the Keep answer's glob, so the docs/design/ files land in Kept. Criteria 4 and 5: `leavesUndecided` treats Exclude globs, Keep globs and the configured keep list as decided. `slices.Clip` stops a Keep answer from appending into `cfg.Release.DiffKeep`'s backing array. A paused suggestion can never show "0 files", because `leavesUndecided` requires at least one match. The unattended and dry-run report paths (`reportAll`, line 169-174) compute matches against the configured keep list only, which matches the task's note that runs giving no answers reflect the config alone.

TESTS:
- Status: Adequate
- Coverage:
  - internal/notes/exclusionpausecount_test.go:59-72 — criterion 1 at the Matches level: 3 paths, 17,000 bytes, Kept docs/api.md
  - internal/engine/release_exclusionpausecount_test.go:14-52 — criterion 1 end to end: asserts the notes diff and both Change Map git calls carry `:(exclude)docs/old/` and `:(exclude)docs/VERSION` (the fake git cannot drop them itself, so the argv assertion is the right proof), and the exact rendered question `... (documentation; 3 files, 17 KB; on the keep list: docs/api.md)`
  - internal/notes/exclusionpausecount_test.go:74-87 — criterion 2 (an Exclude answer removes its files from the later count)
  - internal/notes/exclusionpausecount_test.go:89-106 — criterion 3 (a Keep answer's files count and are named as kept)
  - internal/notes/exclusionpausecount_test.go:108-130 — criterion 4, table over Exclude and Keep
  - internal/notes/exclusionpausecount_test.go:132-141 — criterion 5
  - internal/notes/exclusionpausecount_test.go:143-156 — the inverse guard: a suggestion with an undecided match still pauses, and its count drops the excluded file
  - internal/engine/release_exclusionpausecount_test.go:54-66 — the singular `1 file`
- Notes: Each test would fail if its behaviour broke: dropping the `a.excluded` filter fails criterion 2, ignoring Keep answers in Kept fails criterion 3, and ignoring the keep list in `leavesUndecided` fails criterion 5. The tests are focused, with no redundant variations. The existing pause tests were updated to the new question text (release_exclusionpause_test.go:78-79, 172, 199), which is correct.

CODE QUALITY:
- Project conventions: Followed (external test packages, t.Parallel, exact rendered-line assertions, exact git argv assertions, bytes through internal/bytesize, output through the presenter gate)
- SOLID principles: Good — the count stays in internal/notes, and the wording stays in the engine's gate construction
- Complexity: Low
- Modern idioms: Yes (slices.ContainsFunc/DeleteFunc/Clone/Clip)
- Readability: Good
- Issues: None. The comments on Matches, runAnswers, leavesUndecided, Check and describeMatches hold against the code.

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
