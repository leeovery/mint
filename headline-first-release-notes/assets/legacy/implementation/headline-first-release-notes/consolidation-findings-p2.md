# Consolidation Findings: headline-first-release-notes (Phase 2)

## Findings

### F1: The README tells operators to ship the maintenance line as a copied literal, and its pin checks that copy, not stubBody
- **Class**: duplication
- **Failure**: Task 2-5 added guidance for the case where every part's saved notes are the nothing-to-list line: release `Maintenance release — no notable source changes` instead. It wrote the README sentence and the test that pins it as two literals. The line mint actually ships comes from `stubBody`. Its doc says it is "tunable in one place", and since phase 1 the default prompt asks the AI for its maintenance line in the shape of `stubBody`. Phase 3 tunes the wording around this line. If `stubBody` changes, the empty-diff stub and the AI's maintenance line change together, but the README keeps telling operators to type the old line. The pin stays green because it compares the README with its own copy. An operator who follows the README then ships, on the tag, the CHANGELOG and the provider release, a maintenance line that differs byte for byte from the one mint writes for every other release with nothing users meet. Nothing flags it. The only sign is two differently worded maintenance entries in the CHANGELOG.
- **Evidence**:
  - `README.md:318`: the literal in the guidance on releasing a saved file.
  - `internal/config/readme_saved_part_notes_test.go:43`: the pin's own literal.
  - `internal/notes/degenerate.go:5-13`: `stubBody` and its "tunable in one place" doc.
  - `internal/notes/degenerate.go:45-46`: `StubBody()`.
  - `internal/notes/prompt.go:183`: the default prompt splices `stubBody` in.
  - `README.md:129`: an older, unpinned copy of the same line, describing what mint ships for an empty diff.
- **Proposed shape**: Build the pin's expected sentence from the constant. Concatenate `notes.StubBody()` where the literal now sits between the backticks of the second `want` string. This is how `internal/config/readme_ai_command_test.go:46` already builds its pin from `config.DefaultAICommand`. `config_test` is an external test package, so importing `mint/internal/notes` (which imports `config`) creates no cycle. The README is unchanged and the test passes today. After that, retuning `stubBody` fails this pin until `README.md:318` is updated to match. `README.md:129` can be tied to the constant in the same change. That is optional, because that copy predates this phase.

## Spec Defects

### S1: A corrigendum says the printed next moves never mention the marker, but the release floor's move does
- **Claim**: Corrigendum 2026-10-22, the third one, on §8's guidance: "the next moves never mention the marker (`internal/engine/notesfloor.go:69`, `internal/engine/regenerate_batch.go:259`)".
- **Observed**: `internal/engine/notesfloor.go:69` prints "tidy the saved notes at …, filling in any files its marker lists, and release with …". `internal/engine/regenerate_batch.go:259` does not mention the marker. Neither move tells the operator to delete the marker line. `README.md:318` is still the only text that does (`grep -n "delete the marker line" README.md` → 318).
- **Read**: The spec is stale. The evidence is misquoted, but the conclusion holds: §8's guidance is the README paragraph, and phase 2's README-only change matches that. The corrigendum should say the next moves "never tell the operator to delete the marker line". No code change is owed.
