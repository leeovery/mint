TASK: Regenerate saves each version's chunk notes (token-aware-diff-sizing-4-13)

ACCEPTANCE CRITERIA:
- Given `mint release regenerate v1.2.0` from a fresh source whose wave fails with parts finished, mint writes `{user cache dir}/mint/chunk-notes/{repo path, separators as dashes}/v1.2.0.md`, holding those parts' notes as a forward release's file would, and the failure names its full path (§11.4)
- Given `regenerate --all` where the waves of v1.1.0 and v1.3.0 fail with parts finished, each saves its own file, `v1.1.0.md` and `v1.3.0.md`. Each is skipped, and the batch carries on and regenerates the other versions (§11.4, §12.3)
- Given a version whose single-pass call failed without halving, or whose wave produced no notes, no file is saved for it (§12.3)
- Given a version's saved file in which some slice has no notes, the file opens with the missing-notes marker, as on a forward release (§11.4)
- Given a version whose file cannot be written, one warning says its finished chunk notes could not be saved, with the write error verbatim beneath it, and no file is named for it. Under `--all` the batch carries on (§11.4, §12.3)
- The README's `release regenerate` section says a failed fresh regenerate saves its finished chunk notes, and that `mint release regenerate <version> --notes-file <path>` puts them on the version (§11.4, §13.1, §18.2)

STATUS: issues_found

SPEC CONTEXT: §1.3 gives regenerate's fresh source (single and `--all`) chunking and halving, so its waves fail the way a forward release's do. §11.4 fixes the saved file: `{user cache dir}/mint/chunk-notes/{repo path as dashes}/{tag}.md`, finished chunks' notes in order, the bold missing-notes marker when a slice has none, no file when no chunk finished, and on a write failure the AI's failure unchanged plus one warning carrying the write error verbatim. §12.3 keeps `--all` on skip-and-continue, each failed version saving its own file; a single-pass failure or an empty wave names no file. §13.1 makes `regenerate <version> --notes-file <path>` the way one file goes onto one version.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/chunknotes.go:14-64 — `chunkNotesKeeper` (one per run, keyed by root and tag), its `saver()` and its `warnUnsaved()`. The forward release uses the same keeper (internal/engine/release.go:549).
  - internal/engine/regenerate_fresh.go:59-85 — `RegenerateFreshBody` takes `NotesSinks` and wires `ChunkNotes` into the generator.
  - internal/engine/regenerate_interactive.go:229-250 — single version: a keeper for `req.Tag`. `StageFailed` carries `SavedNotes` via `stageFailure` (internal/engine/release.go:1450), then `warnUnsaved` runs after the ✗ line. The unattended next moves get `failure.SavedNotes`, and the attended floor gets `chunkNotes.saved`.
  - internal/engine/regenerate_batch.go:315-328 — `--all`: one keeper per version for `res.Tag`. Each skip is recorded, `warnUnsaved` runs after the skip warning, and the loop carries on. `savedNotesPointer` (:251) names the file, which is Phase 5's addition on top of this task.
  - internal/notes/chunknotes.go:48-58 — no file when no part finished. The marker line comes first and is followed by a blank line. A failed save returns the failure unchanged.
  - cmd/mint/main.go:193 — production wires `notescache.NewUserChunkNotes(os.UserCacheDir)` into regenerate's deps. cmd/mint/regenerate_all.go:117 passes the sinks into `RegenerateFreshBody` for both paths.
  - README.md:163 — the `release regenerate` section documents the file, its key, the marker, the write-failure warning, the `--all` per-version files, the no-file cases, and `mint release regenerate <version> --notes-file <path>`.
- Notes: Both paths key the file by the canonical tag over the same repo root the forward release uses (`gitrepo.ResolveRoot`). A save is skipped on a cancel. Pretty `Warn` stops a live spinner (internal/presenter/pretty.go:690), so the batch's skip warning and the unsaved warning, both emitted inside the still-open notes stage, keep stream discipline. No drift from the plan.

TESTS:
- Status: Adequate
- Coverage: internal/engine/regenerate_chunknotes_test.go covers every engine-level criterion:
  - AC1 and AC4, single version: the saved file under `v1.2.0` is checked byte for byte against the forward release's constants, both with the marker (part fails) and without it (merge fails), and `StageFailure.SavedNotes` must equal the full path.
  - AC2 and AC4, `--all`: v1.1.0 and v1.3.0 each get their own marked file, and only v1.2.0 is dispatched.
  - AC3: a single-pass failure and an empty wave are each checked on both the single-version and `--all` paths. No file appears under the base, and the skip and summary name none.
  - AC5: the single-version test checks the unchanged ✗ line and output, that no path is named, one exact warning, and its order after the ✗ line. The `--all` test checks the skip warning followed by the unsaved warning, that v1.2.0 and v1.3.0 are still dispatched, and that the summary names no file.
  - The tests are focused, use shared fixtures with the forward-release tests, and are not over-tested.
- Notes: Nothing at the cmd level checks that production regenerate actually wires a chunk-notes store (see FINDINGS).

CODE QUALITY:
- Project conventions: Followed. Output goes only through the presenter, the write error travels verbatim in `Warning.Output`, tests are external, parallel and use temp dirs, and the forward and regenerate paths share one keeper.
- SOLID principles: Good. `ChunkNotesSaver` is a consumer-defined one-method interface, and `NotesSinks` groups the generator's three outputs.
- Complexity: Low
- Modern idioms: Yes
- Readability: Good. The comments in the changed code (keeper, `NotesSinks`, `RegenerateFreshBody`, `processOneVersion`, `reportSkip`) hold true against the code.
- Issues: See FINDINGS

BLOCKING ISSUES:
- None

FINDINGS:
- [in-scope] [contained] cmd/mint/main.go:193 — Regenerate's production chunk-notes store is built inline inside `regenerate()` (cmd/mint/main.go:148), and that function is the seam every cmd regenerate test calls. So no test can inject a store, and none checks that one is wired. The forward release takes the store as a parameter of `release()` (cmd/mint/main.go:310), and `TestRelease_ChunkedRunFails_SavesTheFinishedPartsNotesInTheGivenStore` (cmd/mint/release_seam_test.go:260) checks it end to end. Fix: give `regenerate()` a `chunkNotes engine.ChunkNotesSaver` parameter the same way, have `runRegenerate` pass `notescache.NewUserChunkNotes(os.UserCacheDir)`, have the cmd test callers pass `notescache.NewChunkNotes(t.TempDir())`, and add one cmd test that drives a chunked fresh regenerate failure through `regenerate()` and asserts the saved file. — FAILS: if the `ChunkNotes:` field at main.go:193 is dropped, or set to nil, every test still passes. The keeper's `saver()` then returns nil (internal/engine/chunknotes.go:29-31), and every regenerate chunked failure silently saves no file and gives no warning, losing finished wave work. The engine tests cannot see this because they inject `deps.ChunkNotes` themselves.

UNSETTLED:
- None
