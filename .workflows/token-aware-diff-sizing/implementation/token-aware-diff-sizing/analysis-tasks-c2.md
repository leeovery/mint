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

**Acceptance Criteria**:
- [ ] A release run with `--notes-file notes/what's-new.md`, a file inside the repo, fails before the push, and the unwind's soft reset fails too. The warning reads ``automatic recovery failed; manually reset HEAD back to {sha}, leaving notes/what's-new.md as it is (`git reset --soft {sha}` then `git restore --source={sha} --staged --worktree -- ':(top,literal,exclude)notes/what'\''s-new.md'`)``. The pathspec is written the way `cmdline.Quote` writes it (§12.4, §13.2)
- [ ] The same release, where the soft reset succeeds and the restore fails, warns ``automatic recovery failed; manually restore the index and work tree to {sha}, leaving notes/what's-new.md as it is (`git restore --source={sha} --staged --worktree -- ':(top,literal,exclude)notes/what'\''s-new.md'`)`` (§12.4, §13.2)
- [ ] A regenerate to a changelog target with `--notes-file notes/what's-new.md`, whose reset after its changelog commit fails, warns the same reset step with the same quoted pathspec, followed by ` to drop the leftover changelog commit` (§13.2)
- [ ] For a notes file named `notes/'draft'.md`, reading the printed restore step back by POSIX shell rules (`cmdline.Split`) gives the pathspec `:(top,literal,exclude)notes/'draft'.md` as one whole word, so the step leaves that file out and never names `notes/draft.md` (§13.2)
- [ ] The prose `leaving {path} as it is` names the notes file raw, a `'` in it included (§12.4)
- [ ] A notes file whose name holds no `'` (`NEXT_RELEASE.md`, `notes.md`, a name with a space) gets manual steps byte-identical to today's. The pinned expectations at internal/engine/notesfile_reset_test.go:268 and :330 stand unchanged (§12.4)

**Do**:
- The change is `restoreLine` at internal/engine/unwind.go:305: write `pathspec[1]` through `cmdline.Quote` in place of `" '" + pathspec[1] + "'"`. `pathspec[0]`, the bare `--`, stays as it is, and so does the `keeping` prose at :306. Both of `resetTo`'s manual steps embed `restoreLine` (:308, :311), and both callers reach `resetTo`: `resetCommitsIfMade` (unwind.go:201) and `resetAndAbort` (internal/engine/regenerate_write.go:397). This one line therefore covers the release unwind and regenerate's changelog reset. unwind.go already imports `mint/internal/cmdline` for `unstageEdit` (:260).
- The complete set, measured: I searched every `*.go` file in the repo for a single quote spliced into a string by hand, using the ripgrep pattern `" '"|"' "|\+ "'"|"'" \+`. It returns 7 lines. Two are production code: internal/cmdline/quote.go:17, which is `Quote`'s own body, and internal/engine/unwind.go:305. The other five are expected strings in internal/engine/notesfile_quoting_test.go (:45, :63, :89, :109, :125), and they already match what `cmdline.Quote` writes. So unwind.go:305 is the last printed step that quotes by hand.
- The existing keep-out reset pins are `TestUnwind_KeepOut_ResetStepFails_WarnsTheManualStepsLeft` (internal/engine/notesfile_reset_test.go:263) and `TestRegenerateWrite_KeepOut_RecoveryResetFails_WarnsTheManualStepsLeft` (:310). Each takes its keep-out as a `git.Exclusion(...)` and fails the reset with `scriptedMutateFailure()`. Their expected strings do not change.

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

**Acceptance Criteria**:
- [ ] The README's `release` section has a new paragraph beside its chunking paragraph. It says that a failed notes call, whether the single-pass call, a chunk call or the merge call, is classified by one more call through the release's `ai_command`. Only a generation failure whose AI output holds text on either stream is classified. A timeout, a missing AI tool, empty output, a classify prompt over `max_prompt_bytes` and a classify call that fails all go straight to the floor (§9.1, §9.2, §9.5)
- [ ] The same paragraph says what a SIZE answer does on the single-pass call or a chunk call: mint re-sends that slice as calls aimed at half the failed call's composed size. The rounds come one after another, at most three per slice, and a further SIZE fails the slice. SIZE on the merge call is a failed merge, and any other answer goes to the floor (§9.3, §10.1, §10.2)
- [ ] The same paragraph says that when halving wrote the notes, mint then warns `set max_prompt_bytes = N`, N being the exact size in bytes of the largest diff-carrying call that succeeded. Under `regenerate --all` the warning prints once, at the end of the batch, however the batch ends. The "lower-the-cap hint" at README.md:121 then names a hint the page defines (§10.3, §16.3)
- [ ] The same paragraph says that `r` on chunked notes re-runs only the merge over the part notes already written, with the nudge, and that a change a part's notes missed is not re-read. README.md:127's "(`r` included)" then says what `r` does there (§15.1)
- [ ] The AI Transport section's routing clause at README.md:293 says that a release's failed notes call reaches `on_notes_failure` only once classification and halving have not written the notes. Commit's `$EDITOR` fallback and the rest of the sentence read as they do today (§9.1, §12.1)
- [ ] Behaviour is unchanged. Only README.md changes, every README pin in internal/config stays green as it stands, and every gate passes (§18, §18.2)

**Do**:
- Where the work lives:
  - The new paragraph goes in README.md's `### release` section, beside the chunking paragraph at :127. The paragraph after that one, :129, covers the chunk-notes file.
  - The routing rewording at :293, in `## The AI Transport`, touches only the clause "release follows `on_notes_failure`".
  - README.md:109, :111, :121 and :127 stay as written. The new paragraph supplies the behaviour they refer to.
- What the tree builds, so the paragraph states it truly:
  - **Classify.** `classify` (internal/notes/classify.go:49-70) asks only about an `ai.GenerationError` whose `Output()` is non-empty. A whitespace-only stream counts as empty (internal/ai/transport.go:97-105). It sends no classify prompt over `max_prompt_bytes`, and a failed classify call reads as OTHER.
  - **Halving.** A single-pass SIZE goes through `generateHalved` (internal/notes/chunk.go:37-48), aiming at half the call's size, and that counts as the slice's first halving. A chunk call's slice is re-cut at `len(prompt)/2` (chunk.go:217-219), and the bound is `maxHalvings = 3` (chunk.go:112).
  - **What "serially" means.** A slice's rounds follow one another, each starting only after its call answered SIZE. A round's new parts still join the wave's queue and go out with its other calls, at most `maxChunkCallsInFlight` (4) in flight (chunk.go:116, `halve` at :253-261). "Serially" must not read as one call at a time.
  - **Merge.** In `sendMerge` (chunk.go:82-91), SIZE gives `ErrMergeTooLarge`, which reads `part notes too large to merge`. Any other answer is a failed merge, `(merging)`.
  - **The hint.** The warning is `hintLowerCap` (internal/engine/notesstage.go:270-280), label `max_prompt_bytes`. A release prints it after the notes (internal/engine/release.go:592), and `--all` prints it at the batch's end on both exits (internal/engine/regenerate_batch.go:204, :218). Only diff-carrying calls feed it, through `CallSucceeded` (internal/notes/generate.go:234, chunk.go:230).
  - **`r`.** `r` on chunked notes is `Remerge` (internal/notes/remerge.go:47-57).
- The pins:
  - The `### release` section's pinned phrases (internal/config/notes_floor_docs_test.go, internal/config/readme_exclusion_check_test.go) sit in paragraphs this task does not edit.
  - README.md:293 carries the pinned substring "`timeout` seconds, default `180` for release and `60` for commit" (internal/config/readme_tripwire_test.go:229). The reworded clause leaves it intact.
  - The `## The AI Transport` section's pins (internal/config/readme_ai_command_test.go:71-79) hold text elsewhere in that section.
