TASK: Save the finished chunk notes when a chunked run fails (token-aware-diff-sizing-4-10, tick-c85b15)

ACCEPTANCE CRITERIA:
- Release of `v1.4.0` in `/Users/lee/Code/app`, 6-part wave failing for good at part 3 after parts 1, 2, 4 finished, writes `{user cache dir}/mint/chunk-notes/-Users-lee-Code-app/v1.4.0.md`; under `abort` the failure names the full path after the AI's verbatim output
- The file holds parts 1, 2, 4's notes byte for byte, chunk order, blank line between; part 5 (in flight when part 3 failed) finishing with notes sits in part 5's place
- Every part finishes and the merge fails (call fails / classify SIZE / prompt over cap): file is every part's notes in chunk order, blank-line separated, nothing else
- A halved slice's finished parts sit in its place; a halved single-pass whose parts partly finished writes a file
- `on_notes_failure = fallback`: file written as under abort, commit-subject list ships
- No part returned notes: no file, no path named; single-pass failing without halving: no file
- A later failing chunked run for the same tag overwrites the file
- A later dry run writing the notes cache leaves the file in place
- Ctrl-C with chunk calls in flight and parts finished: no file
- README `release` section documents the file: where, what, failure names its path, released with `mint release --notes-file <path>` / `mint release regenerate <version> --notes-file <path>`

STATUS: complete

SPEC CONTEXT: §11.4 places the saved file at `{user cache dir}/mint/chunk-notes/{repo path, separators as dashes}/{tag}.md`, beside but outside the dry-run cache's pruning, holding finished chunks' notes in chunk order separated by a blank line, written only when at least one chunk finished, overwritten by a later failure of the same tag, with the failure naming its full path. §11.1–§11.3 make any final chunk failure or merge failure trigger the save; §11.2 keeps in-flight calls' notes; §11.5 saves nothing on Ctrl-C; §12.1 orders the attended failure as ✗ line, verbatim output, then the saved-file pointer; §18.2 requires README coverage under `release`.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notescache/chunknotes.go:30-55 — `NewUserChunkNotes` resolves `{UserCacheDir}/mint/chunk-notes` lazily; `Save` writes `{base}/{repoScope(root)}/{tag}.md` via `fsutil.WriteFile` (temp+rename, overwrites)
  - internal/notes/chunk.go:57-66 — `writeParts` routes both a failed wave and a failed merge (call failure, SIZE → ErrMergeTooLarge at :86, over-cap at :70-72) through `keepChunkNotes`
  - internal/notes/chunk.go:168-193, :226-231 — the wave records an in-flight call's notes even after the wave has ended, keeps halved parts in their slice's place (`halve`, :253-275), and returns no parts/notes when ctx ended
  - internal/notes/chunknotes.go:48-58, :68-77, :81-93 — `keepChunkNotes` skips when no part finished; `saveFailedChunkNotes` skips on `context.Canceled`/nil saver/failed save; `joinChunkNotes` keeps each part byte for byte with one blank line between
  - internal/notes/generate.go:227-241 — single-pass failure without SIZE returns without saving; a SIZE single-pass goes through `generateHalved` → `writeParts`
  - internal/engine/chunknotes.go:29-41 and internal/engine/release.go:549-576 — keeper wired into the generation sinks; save happens inside the generation, before StageFailed and before the floor/fallback routing
  - internal/engine/release.go:1442-1452 — `StageFailure.SavedNotes` from `notes.SavedChunkNotesPath(cause)` (found through ResolveFailure's `%w` abort wrap)
  - internal/presenter/plain.go:212-214, internal/presenter/pretty.go:586-588 — path rendered after the verbatim Output
  - cmd/mint/main.go:193, :305 — production wiring with `os.UserCacheDir` for release and regenerate
  - README.md:131 — documentation inside the `release` section (README.md:83-140)
- Notes: Prune (internal/notescache/cache.go:170-183) only touches `.json` entries under `mint/cache/{scope}`, a sibling tree, so the saved file is outside it. Cancel during the wave, the merge call, or the merge's classify call all reach `saveFailedChunkNotes` as `context.Canceled` (or return before it with nil notes) and save nothing.

TESTS:
- Status: Adequate
- Coverage:
  - internal/notes/savenotes_test.go:105-137 — the 6-part scenario: part 3 fails for good after 1, 2, 4 finish, part 5 in flight finishes and sits in its place, part 6's later failure ignored
  - internal/notes/savenotes_test.go:139-162 — byte-for-byte join including no-trailing-newline and trailing-blank-line notes
  - internal/notes/savenotes_test.go:164-205 — all three merge-failure cases, file is every part's notes with no marker
  - internal/notes/savenotes_test.go:229-293 — halved slice in place; halved single-pass partly finishing
  - internal/notes/savenotes_test.go:295-330 — no part finished, single-pass without halving: nothing saved, no path named
  - internal/notes/savenotes_test.go:332-415 and internal/engine/notes_savedchunknotes_test.go:244-279 — Ctrl-C mid-wave, during merge, during merge classify
  - internal/engine/notes_savedchunknotes_test.go:131-242 — abort names full path with output unchanged; merge failure; fallback writes file and ships subject list; overwrite; dry-run cache write leaves file
  - internal/notescache/chunknotes_test.go:22-110 — exact `-Users-lee-Code-app/v1.4.0.md` path, overwrite, prune isolation, user-cache-dir base, unresolvable dir
  - internal/presenter/stagefailed_savednotes_test.go:23-75 — path line follows the verbatim output in plain and pretty, absent when empty
  - cmd/mint/release_seam_test.go:260-289 — the cmd seam passes the store through to the engine
- Notes: Some overlap across layers (overwrite and prune isolation tested at both notescache and engine level), but each layer pins a distinct seam; not bloat.

CODE QUALITY:
- Project conventions: Followed — file write via `fsutil.WriteFile`, output only through the presenter's StageFailure payload, cancel propagated unchanged, external test packages with `t.Parallel()` and `t.TempDir()`
- SOLID principles: Good — storage (`notescache.ChunkNotes`) behind a consumer-side `ChunkNotesSaver` interface; content assembly in `notes`; engine only wires
- Complexity: Low
- Modern idioms: Yes (`slices.DeleteFunc`, `errors.As`, `%w` chains)
- Readability: Good; comments on the new types and functions hold against the code
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
