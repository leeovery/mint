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
- `TestRegenerateRun_NotesFile_YesAutoAcceptsTheReviewGateInsideThePresenter` (internal/engine/regenerate_notesfile_test.go:337) sets `req.Yes = true`. Its `notes: accepted (-y)` line comes from the presenter it builds with -y (`presenter.NewForStartup(true, true, ...)`, :335), not from the request.

**Solution**: Delete `RegenerateRunRequest.Yes`, its assignment in `regenerateRunFlags`, and "the -y flag" from the struct's doc comment. `ReleaseDeps.Yes` is then the only place a single-version regenerate learns -y. The tests change only where the deletion forces it:
- drop the `yes` parameter from `runReq` and from every call site;
- drop the `run.Yes` assertion from `TestRegenerateRunFlags_NotesFile`, keeping its NotesFile and Target checks.

Tests that need an unattended run already get it from the deps, through `run.deps.Yes` or by leaving `StdinInteractive` false, so their assertions hold. Behaviour does not change, because the engine never reads the field. This follows cycle 1 Task 1's settled direction that -y and the stdin signal reach the engine through `ReleaseDeps` on a tested seam, and the finder's proposed shape.

**Outcome**: A single-version regenerate has one home for -y, `ReleaseDeps.Yes`, which the notes floor reads. The request carries no same-named field that looks live and does nothing.

**Acceptance Criteria**:
- [ ] Take `mint release regenerate <version>` run under -y at a terminal, where the notes generation fails. It prints the ✗ line, the AI's verbatim output and regenerate's cause-fitted next moves, then asks nothing, writes nothing and exits 1, as it does today. The -y reaches the floor through `ReleaseDeps.Yes` alone (§12.1, §12.3, §12.4)
- [ ] The same failure at a terminal without -y is offered the `c` / `e` / `a` choice, as today (§12.2, §12.3)
- [ ] The same failure with stdin not a terminal and no -y stops with the next moves and asks nothing, as today (§12.3)
- [ ] For `1.4.0 --notes-file notes.md --target both -y`, `regenerateRunFlags` gives a request whose NotesFile is `notes.md` and whose target is both. Without `--notes-file`, NotesFile is empty (§13.1)
- [ ] `RegenerateRunRequest` has no `Yes` field, and its doc comment no longer lists "the -y flag" among its inputs. `BatchRegenerateRequest.Yes`, its read in `gatePerVersion`, and the cmd layer's `regenerateRequest.Yes` all stay as they are
- [ ] Behaviour is unchanged. Every existing test stays green with its assertions as they stand, apart from the edits the deletion forces, which are listed under Do. All five gates in CLAUDE.md pass

**Do**:
- Production, two edits:
  - internal/engine/regenerate_interactive.go:139: delete `Yes    bool`. In the struct's doc comment (:100-103), drop "the -y flag" from the list of inputs.
  - cmd/mint/regenerate_run.go:36: delete `Yes:       req.Yes,` from `regenerateRunFlags`. `regenerateRequest.Yes` itself stays, because three places still read it: `ReleaseDeps.Yes` at cmd/mint/main.go:194, the `-y` without `--target` check at cmd/mint/regenerate_validate.go:33, and the batch request at cmd/mint/regenerate_all.go:59.
- The `runReq` helper (internal/engine/regenerate_interactive_test.go:51-64) loses its `yes bool` parameter and its `Yes: yes,` line. Its doc comment drops the "+ Yes flag" the deletion makes false.
- The complete set of `runReq` call sites, measured with `rg -n '\brunReq\(' internal/engine`: 37 lines, the definition plus 36 call sites across 20 test files. Every one drops its third argument:
  - 32 pass `false`.
  - 3 pass `true`: internal/engine/regenerate_chunknotes_test.go:68, internal/engine/notes_waveend_test.go:181 and internal/engine/regenerate_interactive_test.go:240.
  - 1 passes `run.deps.Yes`: internal/engine/regenerate_floor_nextmoves_test.go:45. That helper's `run.deps.Yes = yes` (:39) stays, because it is what makes the "-y at a terminal" mode unattended.
- The other forced test edits:
  - internal/engine/regenerate_notesfile_test.go:337: delete `req.Yes = true`. The test's -y is its presenter's (:335), so its assertions hold.
  - cmd/mint/regenerate_run_test.go:141-143: delete the `run.Yes` assertion. The table's args, `-y` included, and the NotesFile and Target checks stay.
- Leave alone: `batchReq`'s `yes` parameter (internal/engine/regenerate_batch_test.go:59-67) and every `deps.Yes` / `run.deps.Yes` assignment in the engine tests. They set the fields the engine reads.

## Task 2: Corrections
severity: corrections
sources: standards

**Problem**: The setup guide's AI-transport paragraph says "On failure, release follows [release].on_notes_failure" (internal/setupguide/setupguide.go:175). This feature made that untrue with the classify-and-halve ladder. A failed notes call is classified first. A single call or chunk call the model rejects as too big is re-sent in smaller parts, up to three halvings. When that writes the notes, mint warns `set max_prompt_bytes = N`. `on_notes_failure` is reached only once the ladder has not written the notes (§9.1, §10.1-10.3, and §12's definition of the floor as "where the ladder ends without AI notes").

The guide's reader is an AI agent. The guide tells it the guide is self-contained and that it should explain mint's model to the user, and its step 4 points the agent at non-Claude and per-verb `ai_command` choices, the setups most likely to overflow under the default cap. That agent tells the user a model overflow ends at abort or fallback. The user then meets things nobody mentioned: `too big for the model in one call, retrying in N parts` narration, extra classify and halving calls, and a lower-the-cap warning. Cycle 2's Task 2 fixed the same claim at README.md:295, so the README and the guide disagree. §18.1 requires the guide to be edited wherever this feature makes it untrue. `grep -rn 'release follows' --include='*.go' --include='*.md' --exclude-dir=.workflows .` returns this one line.

**Solution**:
- internal/setupguide/setupguide.go:175: replace "On failure, release follows [release].on_notes_failure: abort loud," with "When a release-notes call fails with output and does not time out, mint asks the model, in one more call through ai_command, whether that call was too big for it. A single call or chunk call it was too big for is re-sent in smaller parts, up to three halvings per call, and when that writes the notes mint suggests a lower max_prompt_bytes. Notes still unwritten follow [release].on_notes_failure: abort loud,". The rest of the sentence, from "where an attended release is offered" through "Commit drops to the user's $EDITOR.", stays word for word, and the raw string's lines wrap as their neighbours do.
- The new text follows README.md:295's routing clause and the README's ladder paragraph from cycle 2 Task 2. It stays flag-free (§18.1). Every existing pin in internal/setupguide/setupguide_test.go still holds unchanged: `on_notes_failure`; "an attended release is offered the commit-subject list, writing the notes in the editor, or an abort"; "an unattended one stops with next moves fitted to the cause"; "the commit-subject list, the one non-AI body"; and the no-flag checks.

**Outcome**: The `mint setup` guide routes a failed release-notes call the way the code does and the README describes: classification first, halving on a too-big answer with the lower-the-cap suggestion, and `on_notes_failure` only for notes the ladder did not write.

**Acceptance Criteria**:
- [ ] `mint setup`'s AI-transport paragraph says this: when a release-notes call fails with output and does not time out, mint asks the model, in one more call through ai_command, whether that call was too big for it (§9.1, §9.2)
- [ ] The same paragraph says that a single call or chunk call the model found too big is re-sent in smaller parts, up to three halvings per call. It also says that when the smaller parts write the notes, mint suggests a lower max_prompt_bytes (§10.1, §10.2, §10.3)
- [ ] The paragraph sends only notes still unwritten to [release].on_notes_failure. The rest of that sentence, from "where an attended release is offered" through "Commit drops to the user's $EDITOR.", reads word for word as it does today (§12, §12.1)
- [ ] The paragraph names none of mint's flags: no ` -y`, `--yes`, `--no-ai`, `--notes-file` or `--dry-run` (§18.1)
- [ ] `grep -rn 'release follows' --include='*.go' --include='*.md' --exclude-dir=.workflows .` returns nothing (§18.1)
- [ ] Only internal/setupguide/setupguide.go changes. Every pin in internal/setupguide/setupguide_test.go and cmd/mint/setup_test.go stays green as written, and all five gates in CLAUDE.md pass (§18.1)

**Do**:
- The edit lives in `overview()`'s raw string literal (internal/setupguide/setupguide.go:122-181), under `### The AI transport`. The replaced clause starts mid-line at :175, after "cap opens $EDITOR instead.", and runs to "abort loud," at the start of :176. Lines :176-178 carry the kept rest of the sentence and are rewrapped only as the new text forces. The closing "If you pin a slower model…" lines (:179-180) stay as they are.
- The new text holds no backtick, so it sits inside the raw string with no `` ` + "`" + ` `` splice.
- The pins it must keep satisfying, all of which flatten whitespace, so a rewrap cannot break them, except where noted:
  - `TestGuide_OverviewCarriesAITransport` (setupguide_test.go:340): `on_notes_failure`, "stdin" / "stdout" and `FATAL`, matched on the raw body.
  - `TestGuide_AITransportParagraphCoversThePerCallCapAndChunking` (:389): `max_prompt_bytes`, "default 1500000 bytes", "chunk calls", "one merge call" and "Chunking has no setting".
  - `TestGuide_AITransportParagraphNamesTheOneNonAIBody` (:824): "the commit-subject list, the one non-AI body", with no "fallback body" and no `--no-ai`.
  - `TestGuide_AITransportParagraphNamesTheFloorChoiceWithoutAFlag` (:859): the `floorOffer`, `floorStop` and `floorMoves` phrases (:842-846), with no flag from its list.
  - `TestGuide_AITransportParagraphNamesLeanDefault` (:375): the lean command, rendered from `config.DefaultAICommand` at :161, which this edit does not touch.
  - `TestRunSetupEmitsGuideAndExitsZero` (cmd/mint/setup_test.go:16) compares stdout to `setupguide.Guide()` itself, so it follows the new text. The repo has no golden copy of the guide.
