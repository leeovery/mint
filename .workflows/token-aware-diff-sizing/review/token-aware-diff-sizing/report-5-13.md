TASK: A `regenerate --all` skip names its saved chunk notes (token-aware-diff-sizing-5-13, tick-6e8d8b)

ACCEPTANCE CRITERIA:
- Given `regenerate --all` where v1.1.0's wave fails with parts finished and saves its notes at P1, v1.1.0's skip warning and its entry in the end-of-run summary each name P1 with `mint release regenerate <version> --notes-file P1` for v1.1.0 (§11.4, §12.3)
- Given v1.3.0 in the same batch also saves its notes, at P3, its warning and summary entry name P3 with v1.3.0's own command. Neither version's entry names the other's file (§11.4, §12.3)
- Given a version whose single-pass call failed, or whose wave produced no notes, its skip names no file and no command (§12.3)
- Given a version whose chunk notes could not be saved, its skip names no file, and the warning that they could not be saved shows with the write error verbatim (§11.4)

STATUS: complete

SPEC CONTEXT: §12.3 keeps skip-and-continue as `regenerate --all`'s floor; each skipped version names its real cause phrase and the failed call's output, and a version whose wave left chunk notes saves its own file (§11.4), named in its skip warning and in the end-of-run summary with that version's own `mint release regenerate <version> --notes-file <path>`. A single-pass failure or a wave with nothing finished names no file (§11.4: a marker-only file would pass `--notes-file`'s empty check). The §11.4 corrigendum: when the file cannot be written, one warning carries the write error verbatim and the run goes on as when no file was saved. §13.1: one file answers one version.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/regenerate_batch.go:229-239 — `skippedVersion.SavedNotes` carries the pointer (path plus the version's own command); empty when nothing was saved
  - internal/engine/regenerate_batch.go:242-247 — `cause()` appends `; {SavedNotes}` to the reason only when a file was saved; one source for both the warning and the summary
  - internal/engine/regenerate_batch.go:251-257 — `savedNotesPointer` reads the path from the failure through `notes.SavedChunkNotesPath` and builds `part notes saved to P — tidy them and put them on it with mint release regenerate {version} --notes-file P` through `notesFileCommand` (shell-quoted command, raw prose path)
  - internal/engine/regenerate_batch.go:323-327 — the skip records the pointer per version (each version has its own `chunkNotesKeeper`, so each failure carries only its own path), then `warnUnsaved` follows the skip warning
  - internal/engine/regenerate_batch.go:434-437 (`reportSkip`) and :445-454 (`batchSummary`) — both render `skip.cause()`
- Notes: The command's version argument is the bare key (`1.1.0`), the same one the single-version floor moves use (internal/engine/notesfloor.go:59 via regenerate_interactive.go:243) and which the `<version>` positional accepts with or without the tag prefix (cmd/mint/regenerate_flags.go:52). The single-pass, nothing-finished and save-failed cases all leave `SavedChunkNotesPath` empty (internal/notes/chunknotes.go:48-77: no finished part returns the failure unwrapped; a failed save returns it unwrapped), so they name no file by construction. The "saved to … — …" shape matches the release fallback warning's (internal/engine/notesstage.go:136). README.md:163 was updated to describe the warning and summary entry; its claim holds against the code.

TESTS:
- Status: Adequate
- Coverage:
  - AC1 and AC2: internal/engine/regenerate_chunknotes_test.go:187-216 runs a three-version batch with v1.1.0 and v1.3.0 failing at part 2. It asserts each saved file's contents, the exact summary string, and the exact warning list with each version's own path and bare-key command and the AI output still beneath. Exact equality rules out cross-naming. Removing the pointer or swapping paths would fail it.
  - AC3: internal/engine/regenerate_chunknotes_test.go:249-265 (the `--all` subtests of `TestRegenerate_NoChunkNotesToKeep_SavesNoFile`) covers a single-pass failure and an all-parts-fail wave. It asserts no file on disk, a skip warning whose Message is the bare phrase, and a summary that names no file.
  - AC4: internal/engine/regenerate_chunknotes_test.go:290-309 injects a failing store. It asserts the skip warning carries the bare phrase, followed by the unsaved warning with the write error verbatim, that the batch carries on, and that the summary names no file.
  - Shell-quoting of a saved path with a space in the skip's command is covered separately in internal/engine/notesfile_quoting_test.go:114.
- Notes: The tests are focused. Each acceptance criterion maps to one assertion block, with no redundant variants.

CODE QUALITY:
- Project conventions: Followed. Output goes through `presenter.Warn` and `RunFinished` only. The Summary is engine-computed and rendered verbatim. The save error travels verbatim in `Warning.Output`. The tests are external-package and parallel, and assert exact rendered strings.
- SOLID principles: Good. `cause()` is the single place that composes skip text for both surfaces.
- Complexity: Low
- Modern idioms: Yes
- Readability: Good. The changed comments (internal/engine/regenerate_batch.go:235-237, 241, 249-250, 429-433, 439-444) are accurate as built.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
