TASK: A dry run shows the suggestions in its own mode, without asking (token-aware-diff-sizing-7-11)

ACCEPTANCE CRITERIA:
- Given `mint release --dry-run` at a terminal without `-y` where the AI suggests `docs/design/`, mint shows the glob, its reason, the matched files' count and total diff size, and any of them on the keep list, as the pause would. It shows them before the notes are written, where the real run would pause, offers no choice and reads no input (§6.7, §7.2)
- Given that dry run, its notes carry the `docs/design/` diff (§6.4, §6.7)
- Given `mint release --dry-run -y`, or a dry run with stdin not a terminal, the suggestions print after the notes as an unattended run prints them, each with the `[release].diff_exclude` line that would exclude it (§6.7)
- Given an unattended dry run whose notes call fails, under `on_notes_failure = "fallback"` the suggestions print after the previewed commit-subject list, and under `abort` they print with the next moves (§6.7, §12.1)
- Given either kind of dry run, `.mint.toml` is unchanged (§6.7)

STATUS: complete

SPEC CONTEXT: §6.7 (with its 2026-10-19 corrigendum) makes a dry run ask and write nothing, its suggestions following the dry run's own mode: an attended dry run shows them as they would appear at the pause; an unattended dry run (`-y` or non-TTY) prints them after the notes as an unattended run does, `[release].diff_exclude` line included. §6.5 defines what a pause shows (glob, reason, matched count and total diff size, kept files). §7.2 puts the check first on the AI path, before cache reuse and the notes, so an attended dry run previews where the real run would pause. §12.1's corrigendum settles the dry run at the floor by its own mode too.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/release.go:1063-1073 — `exclusionCheck` routes by mode: attended real run -> `WithPauses`+`WithRecord`; unattended (real or dry) -> `WithReport(stage.suggested)`; attended dry run (the `default` arm) -> `WithReport(exclusionPauses{...}.preview)`. A dry run never gets pauses or a record, so nothing is asked or written.
  - internal/engine/notesfloor.go:24-26 — `asksAtTerminal` is `attended() && !opts.DryRun`, which sends every dry run off the pause path.
  - internal/engine/release.go:1204-1208 — `preview` renders `exclusionPauseGate(s, m)`, the same gate the real pause prompts with (release.go:1210-1221), via `PreviewGate`, inside a suspended spinner.
  - internal/notes/exclusioncheck.go:152-157, 169-174 — report mode returns no excluded globs, so the notes diff keeps every flagged file; `reportAll` computes each suggestion's matches against the keep list alone (no answers in a dry run), which is what the first pause would show.
  - internal/notes/select.go:214-237 — the check runs before cache reuse and the notes call, so the attended preview lands before the notes are written.
  - internal/engine/release.go:569-573 and 590-593 — an unattended dry run's suggestions print after `ShowNotes` (AI or fallback commit-subject list) and, at the floor under `abort`, straight after `NextMoves`. An attended dry run's `stage.suggestions` stays empty, so nothing prints twice.
  - Presenter: `PreviewGate` on the interface (internal/presenter/presenter.go:111-114), plain (internal/presenter/plain.go:239-241), pretty (internal/presenter/pretty.go:622-632), recorder (internal/presenter/presentertest/recording.go:238-240); marker `dry run: not asked` (internal/presenter/gate.go:53). Neither renderer reads input.
  - README.md:123 documents the dry-run behaviour by mode; internal/config/readme_exclusion_check_test.go pins it.
- Notes: The attended dry run shows every usable suggestion. A real run would skip one that earlier answers cover, but a dry run has no answers, so showing all is correct. Production wiring of `Yes`/`StdinInteractive` (cmd/mint/main.go:327-328, 382-383) feeds the same mode signals the engine uses.

TESTS:
- Status: Adequate
- Coverage:
  - AC1: `TestRelease_AttendedDryRun_ShowsEachSuggestionAsItsPauseWouldBeforeTheNotesAreWritten` (internal/engine/release_exclusiondryrun_test.go:43) pins the exact pause question with and without a kept file, the exclude/keep gate with keep as the default, that the preview comes before the notes call (event index captured inside the transport), and that it sits inside SuspendSpinner/ResumeSpinner. `TestRelease_AttendedDryRun_OffersNoChoiceAndReadsNoInput` (:92) asserts no Prompt, no AskLine and no unattended suggestions event. Presenter tests (internal/presenter/previewgate_test.go) pin the plain and pretty renderings byte-for-byte and assert the input reader is untouched.
  - AC2: `TestRelease_AttendedDryRun_TheNotesCarryTheSuggestedFiles` (:112) asserts both docs/design files' diffs are in the notes prompt and that no extra exclude pathspec reached git.
  - AC3: `TestRelease_UnattendedDryRun_PrintsEachSuggestionAfterTheNotesWithTheLineThatExcludesIt` (:144) runs in both unattended modes, asserting the payload (including `Line`), its position straight after ShowNotes, and no preview or prompt.
  - AC4: `..._Fallback_PrintsTheSuggestionsAfterThePreviewedCommitSubjectList` (:174) and `..._AtTheFloor_PrintsTheSuggestionsWithTheNextMoves` (:203) run in both unattended modes and pin the adjacency and the exit-1 abort.
  - AC5: every test goes through `releaseWith` (internal/engine/release_exclusionsuggestions_test.go:97-118), which asserts `.mint.toml` is byte-identical and never staged. `TestRelease_DryRun_NeitherModeStagesOrCommitsAnything` (:233) adds no `git add`/`commit` across all three modes. If a regression wired pauses into an attended dry run, the recorder's default Keep answer would rewrite `.mint.toml`, so the guard would fail.
- Notes: The tests are focused, with no redundant variations; each would fail if its behaviour broke.

CODE QUALITY:
- Project conventions: Followed. Output goes only through the presenter (seam 3). The engine does not branch on `-y` for gates; mode comes from the threaded `Yes`/`StdinInteractive` (seam 4). Tests are external packages, use `t.Parallel()`, and assert exact rendered lines.
- SOLID principles: Good. The notes package stays mode-agnostic behind the `report` callback; the engine picks the sink.
- Complexity: Low. One switch arm and one three-line method.
- Modern idioms: Yes
- Readability: Good. The comments on `exclusionCheck`, `asksAtTerminal`, `WithReport`, `PreviewGate` and `showSuggestions` match the code as built.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
