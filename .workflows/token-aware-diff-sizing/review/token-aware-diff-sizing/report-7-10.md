TASK: An unattended release prints the suggestions after the notes (token-aware-diff-sizing-7-10)

ACCEPTANCE CRITERIA:
- Given `mint release -y` where the AI suggests `docs/design/`, with the reason "design documents", matching 2 changed files whose diffs total 15,000 bytes, no pause and no `-y` auto-accept echo shows for it, and the notes calls carry the `docs/design/` diffs (§6.7)
- Given that run's notes are written, after the notes mint prints the suggestion's glob and reason, its 2 files totalling `15 KB`, and the `[release].diff_exclude` line that would exclude `docs/design/` (§6.5, §6.7)
- Given `.mint.toml` sets `[release].diff_exclude = ["specs/"]`, the printed line sets `diff_exclude` under `[release]` to `specs/` followed by `docs/design/`. Given the key unset, it sets `diff_exclude` to `docs/design/` alone. Given a second surviving suggestion, `tmp/`, in the same run, its line holds `specs/` followed by `tmp/` and never `docs/design/` (§6.6, §6.7)
- Given one of the matched files is on `[release].diff_keep`, the printed suggestion names it as on the keep list (§6.5, §6.7)
- Given `mint release -y`, the output is the same whether stdin is a terminal or not (§6.7)
- Given `on_notes_failure = "fallback"` and a notes call that fails, the suggestions print after the commit-subject list that ships (§6.7)
- Given `on_notes_failure = abort` and a notes call that fails, the suggestions print with the floor's next moves, and the run exits 1 (§6.7, §12.4, §16.3)
- Given a run whose notes were written after halving, the suggestions and the lower-the-cap hint both print after the notes (§16.3)
- Given any of these runs, `.mint.toml` is unchanged (§6.7)
- The README's `release` section says an unattended run pauses on nothing and excludes nothing, and prints each suggestion after the notes with the `[release].diff_exclude` line that would exclude it (§18.2)

STATUS: complete

SPEC CONTEXT: §6.7 (with its two 2026-10-19 corrigenda) gives an unattended release no pause and no write: each surviving suggestion prints after the notes with its reason, what it matches as the §6.5 pause would show it (file count, total diff size, any files on the keep list), and the `[release].diff_exclude` line an Exclude answer would leave. That means the loaded entries with the glob appended, worked out per suggestion so no line carries another suggestion's glob. They print whatever the notes' outcome: after AI notes, after a `fallback` commit-subject list, and with the floor's next moves on a stop. §16.3 puts them beside the lower-the-cap hint as follow-ups after the notes. §6.4 says the AI never excludes anything itself. §18.2 asks for README coverage. CLAUDE.md seams 3 and 4: output goes through payload-driven presenter events, and whether an unattended run is offered a gate at all is the engine's decision, from the threaded Yes/StdinInteractive signals.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/release.go:1063-1073: `exclusionCheck` routes an attended real run to pauses, an unattended run (`!deps.attended()`) to `WithReport(stage.suggested)`, and an attended dry run to the preview
  - internal/engine/notesfloor.go:18-26: `attended()` is `!d.Yes && d.StdinInteractive`, so `-y` decides unattended regardless of stdin (criterion 5)
  - internal/notes/exclusioncheck.go:152-158 and 169-174: the report branch hands each usable suggestion its matches against the keep list alone and returns no excluded globs, so the selector re-assembles nothing (internal/notes/select.go:253-258) and the notes diff keeps the flagged files
  - internal/engine/notesstage.go:86-111: `suggested` collects the suggestion with `describeMatches` (internal/engine/release.go:1224-1234: "N files, size; on the keep list: …"). `showSuggestions` builds each line from the loaded `cfg.Release.DiffExclude` through `diffExcludeLine`, whose `slices.Clip` forces `append` to allocate, so no suggestion's glob leaks into another's line
  - internal/engine/release.go:590-593: after ShowNotes, then the lower-cap hint, then the suggestions. This covers AI notes, the `fallback` commit-subject list and dry runs
  - internal/engine/release.go:569-574: at the floor (unattended), NextMoves then the suggestions, then Unwind (exit 1)
  - internal/presenter/presenter.go:103-106, 569-575, 594-600: new `ExclusionSuggestions` event and payload
  - internal/presenter/plain.go:224-229 and internal/presenter/pretty.go:602-613: both renderers write to out only
  - README.md:121, in the `release` section
- Notes: The task's "as built" context said the release engine got neither the `-y` decision nor the stdin signal. The implementation threads both into `ReleaseDeps` (internal/engine/release.go:182-185, wired at cmd/mint/main.go:326-328), which follows CLAUDE.md seam 4. This is a sound divergence, not a loss. The report path never wires `WithRecord`, so nothing can write `.mint.toml` or stage it on an unattended run.

TESTS:
- Status: Adequate
- Coverage:
  - internal/engine/release_exclusionsuggestions_test.go:158-179 (criterion 1): no pause gates recorded, so no Prompt and no echo; the single notes call carries both `docs/design/*` diff headers; the notes git calls carry no answered exclude
  - :181-195 (criterion 2): the exact payload, including `2 files, 15 KB`, straight after ShowNotes
  - :197-234 (criterion 3): key unset and key set, with two suggestions; `tmp/`'s line never carries `docs/design/`
  - :236-258: the printed line loads back through `config.Load`
  - :260-276 (criterion 4): keep-list naming
  - :278-303 (criterion 5): identical event kinds, suggestions and gates with StdinInteractive true and false under Yes. This would fail if `attended()` ignored Yes
  - :305-325 (criterion 6): fallback; suggestions straight after the commit-subject ShowNotes
  - :327-348 (criterion 7): floor; suggestions straight after NextMoves; AbortError exit 1; no notes shown; no tag
  - :350-379 (criterion 8): notes, then the hint, then the suggestions
  - criterion 9: `releaseWith` (:97-118) asserts `.mint.toml` is byte-identical and never `git add`ed in every run that uses it
  - Unit layers: internal/engine/diffexcludeline_internal_test.go (escaping, no aliasing), internal/notes/exclusionreport_test.go (report excludes nothing; none or a failed call reports nothing), internal/presenter/exclusionsuggestions_test.go (exact plain and pretty lines, out only, empty prints nothing)
  - criterion 10: internal/config/readme_exclusion_check_test.go:39-52 pins the README sentences
- Notes: None

CODE QUALITY:
- Project conventions: Followed. Output goes through a payload-driven presenter event (seam 3), the gate decision comes from deps signals (seam 4), and the tests are external packages with `t.Parallel()`, exact payloads, FakeRunner and RecordingPresenter
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes (`slices.Clip`/`slices.Clone` to avoid aliasing)
- Readability: Good; the comments on `exclusionCheck`, `showSuggestions`, `diffExcludeLine`, `WithReport` and `Check` hold against the code
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
