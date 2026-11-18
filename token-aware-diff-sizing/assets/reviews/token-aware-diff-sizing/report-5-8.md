TASK: A dry run under `fallback` previews the commit-subject list (token-aware-diff-sizing-5-8, tick-e6ed72)

ACCEPTANCE CRITERIA:
- Given `mint release --dry-run` under `on_notes_failure = "fallback"` whose single-pass call fails with output, no choice shows, and the previewed notes are this release's commit-subject list. One warning carries `AI call failed` with the AI's captured output verbatim beneath it (§12.1)
- Given that dry run at a terminal without `-y`, with `-y`, or with stdin not a terminal, nothing is asked (§12.1)
- Given a dry run whose chunked run failed with parts finished, the chunk-notes file is written as on a real run. The warning names its full path with `mint release --notes-file <path>`, and does not name `mint release regenerate` (§12.1, §12.4)
- Given a dry run whose chunk notes could not be saved, the warning names no file and no command (§11.4)

STATUS: complete

SPEC CONTEXT: §12.1 — a dry run at the floor asks nothing whatever the mode; under `fallback` it previews the commit-subject list as the body that would ship, with the fallback warning (cause phrase, AI output verbatim beneath), naming a saved file with `mint release --notes-file <path>` only, since the version is not out; the chunk-notes file is saved as on a real run. §12.4 — the regenerate command is for once the version is out. §11.4 plus its 2026-10-19 corrigendum — a failed save adds one warning with the write error verbatim and the run goes on as when no file was saved (no path named). The 2026-10-19 §12.1 corrigendum gives the real run's warning both commands; this task is the dry-run counterpart.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/release.go:580-588 — on a resolved body, the stage closes, `chunkNotes.warnUnsaved(p)` runs first, then `stage.warnDryRunFallback()` on a dry run or `stage.warnFallback(versionKey)` on a real run
  - internal/engine/notesstage.go:122-128 — `warnDryRunFallback` appends only `release them with mint release --notes-file <path>` (built by `notesFileCommand`, shell-quoted) for a saved file
  - internal/engine/notesstage.go:130-139 — shared `warnFallbackReleasing`: cause phrase via `failureMessage`, `; falling back to the commit-subject list`, the saved path only when `notes.SavedChunkNotesPath` finds one, output via `notesFailureOutput`
  - internal/engine/release.go:600-602 — the dry run closes through `finishDryRun` after `ShowNotes`, before any review gate
- Notes:
  - The fallback body comes from `notes.ResolveFailure` → `resolveFallbackBody` (internal/notes/resolve.go:54-71), the `lastTag..HEAD` subject list, and the run never reaches the floor's choice because the selector returns it as `KindFallback` with a nil error. That is why no mode gets asked.
  - The chunk-notes store is wired whether or not the run is a dry run (cmd/mint/main.go:304-305, :326), so a dry run's failed wave saves exactly as a real run's does.
  - A save failure leaves `SavedChunkNotesPath` empty, so the warning names no file. The unsaved-save warning comes first, as §11.4's corrigendum orders it.
  - The README paragraph at README.md:113 was extended to describe the dry-run variant accurately. The plan/notes comment this commit removed had become false once the two paths' warnings diverged.

TESTS:
- Status: Adequate
- Coverage (internal/engine/notes_fallback_dryrun_test.go):
  - TestRelease_DryRunFallback_PreviewsTheCommitSubjectListWithTheWarningAndAsksNothing (:44-74). This covers criteria 1 and 2. A table over three modes (a terminal without -y, -y at a terminal, and stdin not a terminal) asserts:
    - zero prompt gates, no ✗ line, the shown body equal to the commit-subject list, and a dry-run RunFinished
    - exactly one warning, `AI call failed; falling back to the commit-subject list`, with the login output verbatim
    - the warning's position between the notes completion and ShowNotes
    - no mutation
  - TestRelease_DryRunFallback_ChunkedFailureSavesTheChunkNotesAndNamesOnlyTheNotesFileRelease (:76-97). This covers criterion 3. It asserts the saved file's exact contents (the marker plus part 1's notes), and that the one warning names the path with `mint release --notes-file <path>`. It also explicitly checks that no warning contains `regenerate`, and that nothing was mutated.
  - TestRelease_DryRunFallback_UnsavedChunkNotesNameNoFileAndNoCommand (:99-117). This covers criterion 4. Under -y with a store whose save fails, it asserts the exact ordered warnings: the unsaved-save warning with the write error, then the fallback warning with no path and no command.
- Notes: Each test would fail if its behaviour broke. Calling `warnFallback` on a dry run fails the chunked test's exact-match and its regenerate check. A dry-run prompt fails `promptGates`. A skipped save fails `readSaved`. The three-mode table maps directly to criterion 2 and is not redundant.

CODE QUALITY:
- Project conventions: Followed — output only through the presenter, with the AI output carried verbatim in `Warning.Output`. Tests are external-package, parallel and table-driven where the shape fits, and assert exact presenter payloads.
- SOLID principles: Good — the two public warning variants differ only in the release command, supplied as a closure to the one shared formatter.
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
