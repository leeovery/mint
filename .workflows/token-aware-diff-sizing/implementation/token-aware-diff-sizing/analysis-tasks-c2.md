# Analysis Tasks: Token Aware Diff Sizing (Cycle 2)

## Task 1: Quote the keep-out restore step with `cmdline.Quote`
severity: low
sources: duplication

**Problem**: When a reset back to the starting commit fails and the run has a `--notes-file` keep-out, `resetTo` (internal/engine/unwind.go:294-314) prints a manual restore step for the operator to paste. Two failures reach it: the release unwind (`resetCommitsIfMade`, unwind.go:201) and regenerate's changelog reset (`resetAndAbort`, internal/engine/regenerate_write.go:397). It builds that step's pathspec with its own quoting, `" '" + pathspec[1] + "'"` (unwind.go:305). This handles a space but not a `'` in the path.

Take a notes file named `notes/what's-new.md`. The pasted step leaves the shell inside an unterminated quote. Take `notes/'draft'.md`. The quotes balance, but the pathspec now excludes `notes/draft.md`, so the restore rewrites the very notes file the keep-out exists to protect, and a tracked notes file's uncommitted edits are lost. Either way, the printed step fails while the repo is half-unwound, the one moment the operator depends on it. No test uses a path with a `'` in it.

Every other printed paste-ready step quotes its shell words through `cmdline.Quote`:
- `notesFileCommand` (internal/engine/notesfloor.go:76);
- `recommit` (internal/engine/release.go:1143-1145);
- `unstageEdit` (unwind.go:260).

Phase 8's Task 2 left `restoreLine` out only as out of that phase's scope.

**Solution**: Build `restoreLine` from `cmdline.Quote(pathspec[1])` in place of the hand-written quotes, as `unstageEdit` and `recommit` do. `pathspec[0]`, the bare `--`, stays as it is. The prose `leaving {path} as it is` stays raw, as phase 5 kept display-only paths raw.

`Quote` wraps any word holding `(` in single quotes, and every keep-out pathspec holds one in `:(top,literal,exclude)`. So for every path without a `'`, the step prints byte-identical to today, and the pins at internal/engine/notesfile_reset_test.go:268 and :330 stand. A path holding a `'` gets it written `'\''`.

Derivation: the rule phase 5 settled from §12.4, that a printed command is one the reader is told to run and so has to survive being pasted into a shell, and phase 8's extension of it to the unwind's steps.

**Outcome**: Every manual step the release unwind and regenerate's changelog reset print quotes its shell words one way. The keep-out restore step then restores everything but the notes file, whatever that file is named.

## Task 2: Document the classify-and-halve ladder, the lower-the-cap hint and the merge-only `r` in the README
severity: drift
sources: standards

**Problem**: The README's `release` section never describes what happens when a notes call fails on size under the cap, although it refers to that ladder in four places:
- "chunking exhausted" (README.md:109);
- "the model rejected even the smallest parts" (:111);
- "beside the lower-the-cap hint" (:121), a hint the page defines nowhere;
- "(`r` included)" on chunked notes (:127), with no word that `r` there re-runs only the merge over the same part notes.

The AI Transport section's "release follows `on_notes_failure`" (:293) leaves out the step a SIZE answer takes first, so on the release path it is no longer accurate.

Take an operator whose model accepts less than the cap: an explicit `ai_command` on a smaller-window model, or a cap raised above the window. They see narration like `too big for the model in one call, retrying in N parts` (internal/presenter/presenter.go:537) and an extra classify call for each failed notes call. After the notes they get the warning `max_prompt_bytes: set higher than this model accepts; set max_prompt_bytes = N, the largest call that succeeded` (internal/engine/notesstage.go:276-279). Their reference explains none of it. One who presses `r` hoping the AI re-reads a part whose notes missed a change gets the same part notes merged again. §18 requires the README to stay true to what is built, and §18.2 has chunking documented with `mint release`.

**Solution**: Add a paragraph beside the chunking paragraph in the README's `release` section. It states the ladder as §9, §10.3 and §15.1 specify and the code builds:
- A failed notes call is classified by one more call through the release's command. A failed call here is the single-pass call, a chunk call or the merge call, and only a generation failure whose AI output holds text on either stream counts. A timeout, a missing AI tool, empty output, a classify prompt over the cap, or a classify call that fails all go straight to the floor.
- A SIZE answer on the single-pass call or a chunk call re-sends that slice as calls aimed at half the failed call's composed size, serially, at most three times per slice. A further SIZE fails the slice.
- SIZE on the merge counts as a failed merge, and any other answer goes to the floor.
- When halving wrote the notes, mint then warns `set max_prompt_bytes = N`, N being the exact size in bytes of the largest diff-carrying call that succeeded. Under `regenerate --all` the warning prints once, at the end of the batch, however the batch ends.
- `r` on chunked notes re-runs only the merge over the part notes already written, with the nudge. A change a part's notes missed is not re-read.

Then reword README.md:293's routing clause: a release's failed notes call reaches `on_notes_failure` only once classification and halving have not written the notes.

The README's existing pins in internal/config hold no text these edits change. Behaviour is unchanged.

**Outcome**: The README's floor, unattended-suggestions and chunking paragraphs refer only to behaviour the page describes. Its transport section routes a release's failures the way the code does.
