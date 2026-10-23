## Attempt 1

ISSUES:
- README.md:318: the closing sentence "Mint checks for neither line: a file that still has one ships with it." no longer clearly names the marker. The old text said "Mint doesn't check for the marker: a file that still has it ships with it." In the new paragraph, "neither line" comes right after the maintenance-line sentence, so its nearest antecedents are "that line" (the part's line) and "the maintenance line", not the marker line and the part's line. The failure: an operator reads it as "mint doesn't check for the part's line or the maintenance line", leaves the marker in, and the `**Notes missing:**` line ships in the published release notes. That would only be noticed after the push. This paragraph is the task's whole deliverable and the only text that guides tidying, so the warning needs to stay explicit.
  FIX: Name both lines in the closing sentence: "Mint checks for neither the marker nor a part's line: a file that still has one ships with it." Optionally add that phrase to the `want` checked in `TestREADME_SavedPartNotesReleasing_DeletesThePartsNothingToListLinesWithTheMarker` (/Users/leeovery/Code/mint/internal/config/readme_saved_part_notes_test.go:27), so the explicit warning can't be dropped silently again.
  ALTERNATIVE: Move the warning to right after the "then release it with `--notes-file` … once it is." sentence, before the maintenance-line sentence, so "neither line" binds to the two lines just named. Rewording in place is the smaller edit and does not depend on sentence order.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/config/readme_saved_part_notes_test.go:9-10 — the cardinality claim ("the one text that …") restates spec reasoning, and an additive README edit elsewhere would make it false.
  OLD: // savedPartNotesReleasing is the README's Saved part notes paragraph on releasing a
// saved file, whitespace-flattened so a pinned phrase matches across line wraps. It is
// the one text that tells the operator to delete the marker line.
  NEW: // savedPartNotesReleasing is the README's Saved part notes paragraph on releasing a
// saved file, whitespace-flattened so a pinned phrase matches across line wraps.

NOTES:
- The `r` re-merge case builds `MergeInputs` by JSON round-trip, the same way /Users/leeovery/Code/mint/internal/notescache/chunked_test.go `mergeInputsOf` does. The fields are unexported, so this is the only way an external test package can build one. The production `r` path hands these inputs to `KeepPartNotes` at /Users/leeovery/Code/mint/internal/engine/chunknotes.go:72, so testing at the `KeepPartNotes` level covers it.
- The `nothingToList` constant is a sample of what the AI might write rather than a fixed string. That fits the chunk framing (/Users/leeovery/Code/mint/internal/notes/chunk.go:554), which asks for the line in free text.
- I re-ran the gates on the affected packages: `go test -race` on config, notes, engine and setupguide passed. A grep found no other text that tells the operator to delete the marker, including the `mint setup` guide.
- Nothing arrived beyond the enumerated inputs.
