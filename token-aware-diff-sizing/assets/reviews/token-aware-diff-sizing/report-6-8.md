TASK: A failed re-merge aborts with every chunk's notes saved (token-aware-diff-sizing-6-8, tick-f5c44c)

ACCEPTANCE CRITERIA:
- Given chunked notes from a 6-part run of tag `v1.4.0` in the repo at `/Users/lee/Code/app`, and an `r` whose re-merge call fails with output and is classified OTHER, the ✗ line reads `AI call failed (merging)` with that call's captured output verbatim beneath it. mint writes `{user cache dir}/mint/chunk-notes/-Users-lee-Code-app/v1.4.0.md`, holding the six parts' notes exactly as the chunk calls returned them, in chunk order, separated by a blank line, with no missing-notes marker. The failure names its full path with `mint release --notes-file <path>`
- Given a re-merge call that times out, the ✗ line reads `AI timed out (merging)`, and the file is saved and named the same way
- Given a nudge that brings the re-merge prompt over the cap, no merge call is sent and the ✗ line reads `part notes too large to merge`. Given a re-merge call classified SIZE, no further call follows and the ✗ line reads the same. Either way the file is saved and named
- Given chunked notes and stdin closing at the `r`'s nudge prompt, no AI call is made, and the file is saved and named the same way
- Given any of these failures, the run then unwinds to the starting commit, taking a pre-tag artifact commit with it, makes no tag and exits 1, and the saved file still sits at the path the failure named, holding the same notes. Under `on_notes_failure = "fallback"` it does the same: no commit-subject list ships and no `c` / `e` / `a` choice shows
- Given chunked notes reused from the notes cache, whether a dry run or an earlier real run cached them, and an `r` whose re-merge fails, the file holds the cached chunk notes and is named the same way
- Given the chunk-notes file cannot be written, the ✗ line, its cause phrase and its captured output are unchanged. One warning says the finished chunk notes could not be saved, with the write error verbatim beneath it, and the failure names no path
- Given a Ctrl-C while the re-merge call runs, the run aborts as a cancel, and no chunk-notes file is written

STATUS: complete

SPEC CONTEXT: §15.1 has a failed `r` on chunked notes save the per-chunk notes first, name the file with `mint release --notes-file <path>`, and abort before any tag whatever `on_notes_failure` holds. The corrigenda count stdin closing at the nudge prompt as a failed `r` (it unwinds) and a Ctrl-C during the re-merge as a cancel (nothing saved). §11.3 counts a merge as failed on any call failure, a SIZE answer or an over-cap prompt. §10.1 says a merge's SIZE answer is never halved. §8.7 sends no over-cap merge. §12.4 and its corrigendum give `part notes too large to merge` no location suffix and other phrases `(merging)`. §11.4 fixes the file's location, contents, marker rule and the write-failure warning. §15.2 has the cache entry hold the per-chunk notes whichever run wrote it.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/remerge.go:52-58 — `Remerge`: an over-cap nudged merge prompt returns `ErrMergeTooLarge` without a call. Otherwise it goes through `sendMerge`.
  - internal/notes/chunk.go:82-91 — `sendMerge`: a SIZE-classified failure wraps `ErrMergeTooLarge` with no halving. Any other failure becomes `callError{location: "merging"}`, which `CauseText` (internal/notes/resolve.go:164-189) renders as `AI call failed (merging)` / `AI timed out (merging)`. `ErrMergeTooLarge` returns before the suffix is applied, so it carries no suffix.
  - internal/notes/chunknotes.go:64-77 — `KeepPartNotes` / `saveFailedChunkNotes`: joins the held part notes byte-for-byte in chunk order with a blank line between (`joinChunkNotes`, :81-93) and adds no marker. A cancel, a nil save or a failed save keeps nothing and returns the failure unchanged.
  - internal/engine/chunknotes.go:66-73 — `keepRegen`: saves only while the keeper holds merge inputs, i.e. the notes at the gate are chunked, whether live, reused from the cache (internal/engine/release.go:1033-1036) or written by an earlier `r` turned chunked.
  - internal/engine/chunknotes.go:75-92 — `reportKept` / `reviewGate`: on any review-gate error, warns once on a failed save (`warnUnsaved`, :54-64, which clears `saveErr`), then names a saved file with `notesFileMove("mint release", saved)`.
  - internal/engine/release.go:1867-1886 — `regenerateBody`: the closed-input path (`abort(regen.keepFailed(err))`) and the regeneration-failure path (`surface(p, "regenerate", regen.keepFailed(rerr))`) both save before any ✗ line, so `StageFailure.SavedNotes` carries the path.
  - internal/engine/release.go:607-613 — the review-gate error goes straight to `Unwind`. `on_notes_failure` and the floor are never consulted.
  - internal/notescache/chunknotes.go:42-55 plus cmd/mint/main.go:193 and :305 — the production store at `{os.UserCacheDir}/mint/chunk-notes/{repo path with dashes}/{tag}.md`.
- Notes: A Ctrl-C during the re-merge: the transport returns `context.Canceled`, `classify` reads it as OTHER without a call (internal/notes/classify.go:53-57), and `saveFailedChunkNotes` sees the cancel through the `callError` wrap and saves nothing. Later refactors (the `regenRun`/`RegenRun` binding and the `command` parameter on `reportKept`) moved the code past the task commit's shape without loss. The engine binds the re-merge and the save to the same keeper inputs, so they cannot drift apart.

TESTS:
- Status: Adequate
- Coverage: internal/engine/release_remergefail_test.go covers every criterion:
  - OTHER with output: ✗ text, verbatim output, `SavedNotes`, file contents, the next move naming `mint release --notes-file`, and the next moves ordered before the unwind (:123-138).
  - Timeout (:140-151).
  - Over-cap nudge (0 re-merge calls, 0 classify calls) and SIZE (exactly 1 nudged call, so no halving round, since halved chunk calls would also carry the nudge), both `part notes too large to merge` (:153-186).
  - Stdin closed at the nudge: AI calls equal the first wave's 6 plus its merge, and the file is saved, named and unwound (:188-207).
  - The unwind, no tag, the file surviving, no `git log` subject list, no floor gate and no warnings, across all four failure kinds under both `abort` and `fallback` (:209-254).
  - Cache reuse from a dry run and from an aborted real run (:256-318).
  - A failed save: the `StageFailure` is unchanged with no `SavedNotes`, there is one verbatim warning after the ✗ line and before the unwind, and no next moves (:320-361).
  - Ctrl-C: the error is a cancel, no file, no next moves (:363-382).
  - The literal `-Users-lee-Code-app/v1.4.0.md` path format is pinned in internal/notescache/chunknotes_test.go:28-33.
- Notes: The tests are focused. The 4×2 unwind matrix maps directly onto the "any of these failures … under fallback" criterion. Every assertion would fail if its behaviour broke.

CODE QUALITY:
- Project conventions: Followed. Output goes through the presenter, the write error travels verbatim in `Warning.Output`, sentinels are matched with `errors.Is`/`errors.As`, a cancel propagates unchanged and is never saved, tests use an external package with `t.Parallel()`, and test doubles are FakeRunner and RecordingPresenter.
- SOLID principles: Good. The keeper owns saving and reporting. `reviewRegen.keep` injects the save into the gate loop without the loop knowing about chunk notes.
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
