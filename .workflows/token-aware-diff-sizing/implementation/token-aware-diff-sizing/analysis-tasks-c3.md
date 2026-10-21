# Analysis Tasks: Token Aware Diff Sizing (Cycle 3)

## Task 1: Drop the single-version regenerate request's unread `Yes` field
severity: dead-code
sources: architecture

**Problem**: `RegenerateRunRequest.Yes` (internal/engine/regenerate_interactive.go:139) is set by cmd/mint's `regenerateRunFlags` (cmd/mint/regenerate_run.go:36) and by the engine tests' `runReq` helper (internal/engine/regenerate_interactive_test.go:53-61). `RegenerateRun` never reads it. The engine's only read of a request's `Yes` is `BatchRegenerateRequest.Yes`, in `gatePerVersion` (internal/engine/regenerate_batch.go:464).

This feature made attended versus unattended an engine decision for a single-version regenerate's notes floor. That decision comes from `ReleaseDeps.Yes` and `StdinInteractive` through `deps.attended()` (internal/engine/notesfloor.go:18-20, regenerate_interactive.go:241). Phase 5's comment corrections deleted the field's doc comment, which had become false, and left a bare `Yes bool` beside `Config`. The struct's doc still lists "the -y flag" among its inputs.

Picture a maintainer who wires -y onto the request alone, taking it for the live sibling of the batch field. A run at a terminal is then attended. When its notes generation fails, the floor offers its `c` / `e` / `a` choice, and the -y presenter auto-answers Abort. The operator gets an abort with no next moves and no `--notes-file` pointer, where §12.3 requires the next moves. The code compiles and no test catches it.

The suite already treats the field as live:
- `regenChunkNotes.regenerate` (internal/engine/regenerate_chunknotes_test.go:68) and `TestRegenerateRun_Unattended_WaveEnds_StopsNamingTheFailureThatEndedIt` (internal/engine/notes_waveend_test.go:181) both pass `yes=true`. Their runs are unattended only because their deps leave `StdinInteractive` false.
- `regenerateUnattended` (internal/engine/regenerate_floor_nextmoves_test.go:44-45) copies `run.deps.Yes` into the request.
- `TestRegenerateRunFlags_NotesFile` (cmd/mint/regenerate_run_test.go:141-143) asserts `regenerateRunFlags(...).Yes`, a guard on a field nothing reads.

**Solution**: Delete `RegenerateRunRequest.Yes`, its assignment in `regenerateRunFlags`, and "the -y flag" from the struct's doc comment. `ReleaseDeps.Yes` is then the only place a single-version regenerate learns -y. The tests change only where the deletion forces it:
- drop the `yes` parameter from `runReq` and from every call site;
- drop the `run.Yes` assertion from `TestRegenerateRunFlags_NotesFile`, keeping its NotesFile and Target checks.

Tests that need an unattended run already get it from the deps, through `run.deps.Yes` or by leaving `StdinInteractive` false, so their assertions hold. Behaviour does not change, because the engine never reads the field. This follows cycle 1 Task 1's settled direction that -y and the stdin signal reach the engine through `ReleaseDeps` on a tested seam, and the finder's proposed shape.

**Outcome**: A single-version regenerate has one home for -y, `ReleaseDeps.Yes`, which the notes floor reads. The request carries no same-named field that looks live and does nothing.

## Task 2: Corrections
severity: corrections
sources: standards

**Problem**: The setup guide's AI-transport paragraph says "On failure, release follows [release].on_notes_failure" (internal/setupguide/setupguide.go:175). This feature made that untrue with the classify-and-halve ladder. A failed notes call is classified first. A single call or chunk call the model rejects as too big is re-sent in smaller parts, up to three halvings. When that writes the notes, mint warns `set max_prompt_bytes = N`. `on_notes_failure` is reached only once the ladder has not written the notes (§9.1, §10.1-10.3, and §12's definition of the floor as "where the ladder ends without AI notes").

The guide's reader is an AI agent. The guide tells it the guide is self-contained and that it should explain mint's model to the user, and its step 4 points the agent at non-Claude and per-verb `ai_command` choices, the setups most likely to overflow under the default cap. That agent tells the user a model overflow ends at abort or fallback. The user then meets things nobody mentioned: `too big for the model in one call, retrying in N parts` narration, extra classify and halving calls, and a lower-the-cap warning. Cycle 2's Task 2 fixed the same claim at README.md:295, so the README and the guide disagree. §18.1 requires the guide to be edited wherever this feature makes it untrue. `grep -rn 'release follows' --include='*.go' --include='*.md' --exclude-dir=.workflows .` returns this one line.

**Solution**:
- internal/setupguide/setupguide.go:175: replace "On failure, release follows [release].on_notes_failure: abort loud," with "When a release-notes call fails with output and does not time out, mint asks the model, in one more call through ai_command, whether that call was too big for it. A single call or chunk call it was too big for is re-sent in smaller parts, up to three halvings per call, and when that writes the notes mint suggests a lower max_prompt_bytes. Notes still unwritten follow [release].on_notes_failure: abort loud,". The rest of the sentence, from "where an attended release is offered" through "Commit drops to the user's $EDITOR.", stays word for word, and the raw string's lines wrap as their neighbours do.
- The new text follows README.md:295's routing clause and the README's ladder paragraph from cycle 2 Task 2. It stays flag-free (§18.1). Every existing pin in internal/setupguide/setupguide_test.go still holds unchanged: `on_notes_failure`; "an attended release is offered the commit-subject list, writing the notes in the editor, or an abort"; "an unattended one stops with next moves fitted to the cause"; "the commit-subject list, the one non-AI body"; and the no-flag checks.
