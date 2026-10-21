TASK: Warn when `fallback` ships the commit-subject list (token-aware-diff-sizing-5-6, tick-e87790)

ACCEPTANCE CRITERIA:
- Given `mint release -y` under `on_notes_failure = "fallback"` whose single-pass call fails with output, no choice shows, and the commit-subject list ships as the release's notes. One warning carries `AI call failed` with the AI's captured output verbatim beneath it, and names no file (§12.1, §12.4)
- Given an attended run under `fallback` whose notes fail, no `c` / `e` / `a` choice shows. The same one warning prints, and the list then shows at the review gate, which offers release, abort and edit with no `r` (§12.1)
- Given a notes call that times out under `fallback`, the one warning carries `AI timed out` with no output beneath it, and the list ships (§12.1, §12.4)
- Given a 6-part chunked run under `fallback` whose part 3 fails for good with output, after other parts finished and were saved at P, the one warning carries `AI call failed (part 3 of 6)`, part 3's captured output verbatim, P, and `mint release regenerate <version> --notes-file P` and `mint release --notes-file P`, with this release's version in the first (§12.1, §12.4)
- Given a chunked run under `fallback` whose merge prompt is over the cap, the one warning carries `part notes too large to merge`, the saved path and both commands (§11.3, §12.1, §12.4)
- Given that 6-part run attended, where the operator aborts at the review gate, the run unwinds as built, and the file at P still holds the saved part notes (§11.4, §12.1)
- Given a chunked run under `fallback` whose chunk notes could not be saved, the warning names no file and neither command (§11.4)

STATUS: complete

SPEC CONTEXT: §12.1's `fallback` row: no choice; the commit-subject list ships with one warning whatever the cause — the §12.4 cause phrase, the AI's captured output verbatim beneath it (§9.6), and where a chunk-notes file was saved (§11.4) its path and both releasing commands. The 2026-10-19 §12.1 corrigendum requires both commands (regenerate for once the release is out, `mint release --notes-file` for a run that ends before the push) because the warning prints at the notes stage, before the review gate and push. The §11.4 corrigendum: when the file cannot be written, the run continues as if none was saved (no path named) after the separate could-not-be-saved warning. §11.4: the file sits outside the repo, so the unwind never touches it.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/select.go:87-93 — `Selector.WithFallbackNotice` copy-builder; internal/notes/select.go:265-274 — `resolveNormalPathFailure` hands the original generator failure (with its `chunkNotesSavedError` wrap intact) to the notice only once `ResolveFailure` has produced the fallback body, so cancels and abort-mode failures are never noticed.
  - internal/engine/release.go:1020-1022 — the release's selector is wired `.WithFallbackNotice(stage.fellBack)`.
  - internal/engine/notesstage.go:82-84 — `fellBack` records the cause; internal/engine/notesstage.go:113-139 — `warnFallback` / `warnFallbackReleasing` build one `presenter.Warning{Label: "notes"}`: `failureMessage(cause)` (the §12.4 phrase with its `(part k of n)` / `(merging)` suffix via `notes.CauseText`) + "; falling back to the commit-subject list", plus, where `notes.SavedChunkNotesPath(cause)` is non-empty, the path and both commands (shell-quoted through `notesFileCommand`); `Output` is `notesFailureOutput(cause)`, the AI's captured output verbatim.
  - internal/engine/release.go:580-588 — the warning prints on the success branch after `stage.done()` closes the blocking stage (so no `Warn` lands mid-spinner) and after `chunkNotes.warnUnsaved`, before `ShowPlan`/`ShowNotes`; the version passed is `versionKey`, the bare `x.y.z` that `mint release regenerate` accepts (same form the regenerate floor's moves use).
  - internal/engine/release.go:1842-1847 — `gateForKind` gives `KindFallback` the y/n/e gate with no `r`; no floor is reached because the selector returns no error under `fallback`.
  - README.md:113 — the fallback paragraph documents the warning and both commands.
- Notes: Under a review-gate abort, `chunkNotesKeeper.reportKept` reads the gate's own error, which carries no saved path, so the abort adds no second pointer — consistent with the corrigendum's "naming both keeps the one warning §12.1 prescribes without adding output to the unwind". The saved file lives under the user cache dir, outside the repo; nothing in `Unwind` touches it.

TESTS:
- Status: Adequate
- Coverage:
  - Criterion 1: internal/engine/notes_fallback_warning_test.go:69 — `-y`, no floor gate, no ✗ line, exactly one warning equal to `AI call failed` + output (exact Message equality also proves no file is named), warning between the notes stage's end and the notes shown, tag annotation is the commit-subject list.
  - Criterion 2: internal/engine/notes_fallback_warning_test.go:92 — attended: no floor gate, the same one warning, list shown, single review gate with keys exactly y/n/e, list tagged.
  - Criterion 3: internal/engine/notes_fallback_warning_test.go:121 — `AI timed out`, empty Output, list tagged.
  - Criterion 4: internal/engine/notes_fallback_warning_test.go:223 — six parts under a 20,000-byte cap, part 3 failing with output; exact warning with `AI call failed (part 3 of 6)`, the output, the saved path and both commands with `1.2.4` in the regenerate command.
  - Criterion 5: internal/engine/notes_fallback_warning_test.go:242 — merge prompt over the cap; exact warning with `part notes too large to merge`, no output, path and both commands.
  - Criterion 6: internal/engine/notes_fallback_warning_test.go:266 — attended, declined at the review gate: abort non-zero, no tag, `git reset --hard <start>` (artifact commit unwound), one save, file at the path equals what was saved.
  - Criterion 7: internal/engine/notes_unsavedchunknotes_test.go:103 — failing store: the could-not-be-saved warning then the fallback warning with no path or commands, list tagged; internal/engine/notes_unsavedchunknotes_test.go:129 — the same when declined at the review gate, no repeat warning.
  - Selector seam: internal/notes/fallbacknotice_test.go:24 and :49 — the notice fires once with the AI's failure under fallback, and not when the AI succeeds or under abort.
- Notes: Assertions are exact-payload (Message/Output/Label) per the project's test idioms; no redundant cases.

CODE QUALITY:
- Project conventions: Followed — output only through `presenter.Warning`; failure output travels verbatim in `Warning.Output`; the warning prints only once the blocking stage has ended; external test packages, `t.Parallel()`, `t.TempDir()`, FakeRunner/RecordingPresenter.
- SOLID principles: Good — the notes package stays presentation-free (a notice callback); the engine's notes stage owns the narration; the dry-run variant (task 5-8) shares `warnFallbackReleasing` through a single injected command phrase.
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
