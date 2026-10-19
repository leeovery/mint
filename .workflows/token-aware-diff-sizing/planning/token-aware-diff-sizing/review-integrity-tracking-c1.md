# Review Tracking: Token Aware Diff Sizing - Integrity

## Findings

### 1. Adding `e` to the release floor leaves three docs saying the floor offers only `c` or `a`

**Severity**: Important
**Plan Reference**: Phase 5 / token-aware-diff-sizing-5-2 (`tick-ec08bd`); also Phase 5's task table row for 5-2 in planning.md
**Category**: Task Self-Containment
**Move**: settled
**Change Type**: update-task

**Problem**:
Task 5-1 pins four surfaces to say an attended release under `abort` whose notes fail is "offered the commit-subject list or an abort". Those surfaces are the README's `release` section, `on_notes_failure`'s README row, `on_notes_failure`'s description in `mint setup`'s config reference, and `mint setup`'s AI-transport paragraph. Task 5-2 then adds `e`, write the notes in `$EDITOR`, to that choice, but updates only the README's `release` section. No task revisits the other three. Once Phase 5 ships:
- the README's `release` section says the floor offers `c`, `e` and `a`
- the README's own `on_notes_failure` row, the config reference and the setup guide still say it offers only the commit-subject list or an abort.

An agent that configures mint from `mint setup` gets a floor with no editor route. Anyone who compares the config reference with the choice the terminal shows sees the mismatch.

**Proposal**:
Task 5-2 updates every surface that describes the choice it extends, as it already does for the README's `release` section. The plan's own convention decides this: each task updates the docs for the behaviour it adds, and task 5-3 updates both the README and the setup paragraph for the next moves it builds. Writing `e` into task 5-1 instead would document a choice that does not exist until 5-2 lands. Three parts of the specification require the edit:
- §18 keeps docs true to what is built.
- §18.1 has the AI-transport paragraph cover the floor.
- §18.1 has a config-reference row whose description names changed behaviour edited in the source of truth, and it names `on_notes_failure`'s row among them.

The Solution, the README criterion, the Do entry, the Spec Reference and the plan table row all change.

**Current**:
Task 5-2, Solution, last sentence:

The README's description of the choice covers `e`.

Task 5-2, Acceptance Criteria, last item:

- [ ] The README's description of the floor's choice covers `e`: it opens the editor on the saved chunk notes where this run saved a file, otherwise empty, and an empty body returns to the choice (§18.2)

Task 5-2, Do, second item:

- The README text goes in the `release` section, with the floor's choice that task 5-1 documents (§18.2).

Task 5-2, Spec Reference:

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §6.6 Recording the answers; §11.2 The first final failure ends the wave; §11.3 A failed merge; §11.4 The saved chunk-notes file; §12.2 The choice; §18.2 The README; Corrigenda (2026-10-19, §11.4 and §12.2)

planning.md, Phase 5 task table, row token-aware-diff-sizing-5-2, Edge Cases cell, last item:

the README's description of the choice covers `e` (§18.2)

**Proposed Text**:
Task 5-2, Solution, last sentence:

The README's description of the choice covers `e`, and so do the other surfaces task 5-1 described the choice on: `on_notes_failure`'s README row, its description in `mint setup`'s config reference, and `mint setup`'s AI-transport paragraph.

Task 5-2, Acceptance Criteria, last item replaced by these two:

- [ ] The README's description of the floor's choice covers `e`: it opens the editor on the saved chunk notes where this run saved a file, otherwise empty, and an empty body returns to the choice (§18.2)
- [ ] The surfaces task 5-1 described the choice on say an attended release under `abort` whose notes fail is offered the commit-subject list, writing the notes in the editor, or an abort (§14.2, §18.1, §18.2):
  - `on_notes_failure`'s README row
  - `on_notes_failure`'s description, as `mint setup`'s config reference renders it
  - `mint setup`'s AI-transport paragraph, which still names no command flag

Task 5-2, Do, second item:

- The README text goes in the `release` section, with the floor's choice that task 5-1 documents, and in `on_notes_failure`'s row in the `[release]` table. The config reference's text is `on_notes_failure`'s row in `config.MetadataRows()` (`internal/config/metadata.go`), and the setup guide's is the "The AI transport" text in `overview()` (`internal/setupguide/setupguide.go`) (§14.2, §18.1, §18.2).

Task 5-2, Spec Reference:

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §6.6 Recording the answers; §11.2 The first final failure ends the wave; §11.3 A failed merge; §11.4 The saved chunk-notes file; §12.2 The choice; §14.2 Removing the key; §18.1 The setup guide; §18.2 The README; Corrigenda (2026-10-19, §11.4 and §12.2)

planning.md, Phase 5 task table, row token-aware-diff-sizing-5-2, Edge Cases cell, last item:

the README's description of the choice, `on_notes_failure`'s README row and config-reference description, and `mint setup`'s AI-transport paragraph cover `e` (§14.2, §18.1, §18.2)

**Resolution**: Pending
**Notes**:

---

## Observations

- Task 4-6's no-hint criterion gives "as when halving runs out" as an example, which cannot be exercised until 4-7 bounds halving; its other example, another part failing for good, carries the criterion.
- The size moves in 5-4, 5-7 and 5-11 name `[release].diff_exclude`, which strict config rejects until Phase 7 lands; this only matters if mint ships between Phase 5 and Phase 7.
- Phase 4's acceptance says the failure names the saved chunk-notes path, but under `fallback` nothing names it until 5-6's warning; task 4-10 scopes naming to `abort`, and Phase 5's acceptance carries the rest.
- Task 5-12's Problem carries the specification's raw `grep` command and output, where the product consequence alone would read cleaner.
- Task 5-9's README criterion says an unattended regenerate "stops with guidance" two tasks before 5-11 builds that guidance; the gap closes within the phase.
