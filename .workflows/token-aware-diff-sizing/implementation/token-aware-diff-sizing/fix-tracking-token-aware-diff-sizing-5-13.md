## Attempt 1

ISSUES:
- /Users/leeovery/Code/mint/README.md:151 (and :153) still describe the `--all` skip as it was before this task. Line 151 sets "A single-version run names the file's full path in its failure" against "Under `--all`, each failed version saves its own file and is skipped". That reads as though `--all` names no path, which this task makes untrue. Line 153 lists what the skip warning and summary entry name (the cause and the AI's output) and leaves out the saved file and its command, which is the very behaviour this task adds. An operator or agent session reading the README to recover a batch's chunk notes is pointed at the path template only. Spec §18 requires the README to change in the same change as the behaviour it describes.
  FIX: In README.md:151, replace "Under `--all`, each failed version saves its own file and is skipped, and the batch carries on." with "Under `--all`, each failed version saves its own file and is skipped, its skip warning and its end-of-run summary entry naming that file with the version's own `mint release regenerate <version> --notes-file <path>`, and the batch carries on." Line 153 stays true after that edit.
  ALTERNATIVE: Instead, append to line 153's last sentence: ", and where its run saved chunk notes, both also name the file and that version's `mint release regenerate <version> --notes-file <path>`". That leaves line 151's contrast reading as if `--all` names nothing, so I recommend the line-151 edit.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/engine/regenerate_batch.go:250 — the list after the colon reads as the complete set of no-file cases, but it is incomplete: a failure from outside the generation calls (the `reasonNotesFailed` path) also saves none.
  OLD: // savedNotesPointer names the chunk notes failure saved and the regenerate of version
// that puts them on it, or "" when failure saved none: a single-pass failure, a wave
// that finished no chunk, or a save that failed.
  NEW: // savedNotesPointer names the chunk notes failure saved and the regenerate of version
// that puts them on it, or "" when failure saved none.
- /Users/leeovery/Code/mint/internal/engine/regenerate_batch.go:241 — the second clause restates the four-line body.
  OLD: // cause is the skip's text in its warning and in the end summary: the reason, then
// the saved chunk notes' pointer when there is one.
  NEW: // cause is the skip's text in its warning and in the end summary.
- /Users/leeovery/Code/mint/internal/engine/regenerate_chunknotes_test.go:181 — "tag's" names no parameter (the helper takes phrase, version and path), and the trailing clause restates the body.
  OLD: // savedSkipCause is tag's skip cause, its cause phrase phrase, when its chunk notes
// were saved at path: the path, then version's own regenerate that puts them on it.
  NEW: // savedSkipCause is version's skip cause, its cause phrase phrase, when its chunk
// notes were saved at path.

NOTES:
- `skippedVersion.SavedNotes` (regenerate_batch.go:235-238) holds the rendered pointer sentence, while `presenter.StageFailure.SavedNotes` and `notes.SavedChunkNotesPath` deal in bare paths. The field comment says so, but a later reader using the same name may expect a path. This does not need changing.
- The skip's wording ("part notes saved to P — tidy them and put them on it with …") follows the release fallback warning (notesstage.go:80) rather than `notesFileMove`'s next-moves wording, which adds "filling in any files its marker lists". The spec's §12.3 requirement for the skip is only the file and the command, so this is acceptable. The marker inside the file still flags the missing slices.
- I ran the gates independently: `go build ./...`, `gofmt -l .` (empty), `go vet ./internal/engine/`, `go test -race ./... -count=1` (all ok) and `golangci-lint run ./internal/engine/...` (0 issues).
- Nothing arrived in the dispatch beyond the enumerated inputs.
