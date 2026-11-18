TASK: One `r` regenerator for both verbs, with the keeper the only holder of its merge inputs (token-aware-diff-sizing-6-11)

ACCEPTANCE CRITERIA:
- A release whose notes a chunked run wrote, live or reused from a chunked cache entry: `r` with a nudge makes one merge call over the held part notes, carrying the nudge and no diff (§15.1, §15.2)
- A release on single-pass notes: `r` re-runs the AI path from the size check with the nudge, and once the ladder has made that `r` a chunked run, a further `r` re-merges that `r`'s part notes in one call (§15.1)
- A release `r` the ladder made chunked whose part fails for good: the finished parts' notes are saved, the abort names the file, and the run aborts before any tag (§15.1, §11.4)
- A single-version regenerate on chunked fresh notes: each `r` re-merges the same part notes in one call carrying only its own nudge; under `regenerate --all`, an `r` re-merges only that version's part notes (§15.1)
- A single-version regenerate on single-pass fresh notes whose first `r` the ladder made chunked and wrote: a second `r` whose re-merge fails, or whose nudge prompt sees stdin close, saves the first `r`'s part notes and names the file with `mint release regenerate <version> --notes-file <path>` (§15.1, §11.4)
- A fresh regenerator built over a zero `RegenRun`: `r` re-runs the fresh AI path over the version's `{PreviousTag}..{Tag}` range with the nudge; on the oldest release it returns the fixed first-release body with no AI call and no diff (§15.1)
- The cmd layer's single-version and batch producers, handed a `RegenRun` holding merge inputs: `r` makes one re-merge call through the release `ai_command`, carrying the nudge and no diff (§15.1)
- Rule: `rg -n '\.WithProgress\(' internal cmd -g '!*_test.go'` matches one line, in the `NotesSinks` wiring method, and `rg -n '\.Remerge\(' internal cmd -g '!*_test.go'` matches one line, in the `r` helper
- Rule: the existing tests pass with their semantics untouched. The only test edits are in the two cmd tests at cmd/mint/regenerate_remerge_test.go:73 and :83, which change how they build the held merge inputs. All five CLAUDE.md gates pass

STATUS: complete

SPEC CONTEXT: §15.1 — `r` on chunked notes re-runs only the merge call over the held part notes with the nudge (one call, never the whole wave); on single-pass notes `r` re-runs the AI path from the size check so the ladder applies; a failed `r` (including stdin closing at the nudge) on chunked notes, or on an `r` the ladder made chunked, saves the part notes and names the file (release: `mint release --notes-file`, regenerate: `mint release regenerate <version> --notes-file`), and a release aborts before any tag. Regenerate single-version and `--all` follow the same rule per version. §15.2 — a chunked cache entry carries its part notes so `r` on reused chunked notes is merge-only. §11.4 — the saved chunk-notes file. This task is a behaviour-preserving consolidation: one sink-wiring method, one `r` rule, one holder of the merge inputs.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/regenerate_fresh.go:82-85 — `NotesSinks.wire`, the single chain of `WithProgress`/`WithChunkNotes`/`WithMergeInputs`
  - internal/engine/regenerate_fresh.go:66 — `RegenerateFreshBody` wires through `sinks.wire`
  - internal/engine/release.go:1019 — release selection builds its sinks through `chunkNotes.sinks(stage).wire(generator)`
  - internal/engine/chunknotes.go:105-125 — `regenerateFunc` and `RegenRun.regenerator`, the one `r` rule: re-merge while the reader returns inputs, else the verb's size-check generation over the generator wired to the run's sinks
  - internal/engine/chunknotes.go:94-103 — `chunkNotesKeeper.regenRun` binds `Merged` to `k.heldMerged` (a method value on the keeper pointer, so the read is live) and the sinks to the keeper
  - internal/engine/release.go:91-99 — `RegenRun.Merged` is now `func() *notes.MergeInputs`; a nil reader reads as none (chunknotes.go:118)
  - internal/engine/release.go:1340-1350 — `perRunRegenerator` keeps the `deps.Regenerator` override and nil-generator return, then calls the shared rule with `GenerateWithContext` over `lastTag`
  - internal/engine/regenerate_fresh.go:98-112 — `RegenerateFreshRegenerator` keeps the first-release short-circuit and the `regenerating fresh notes for %s` wrap, calling the shared rule with `GenerateFromRangeWithContext` over `res.DiffRange()`; the closure-local `merged`, the forwarding `WithMergeInputs` callback and its nil guard are gone
- Notes:
  - Rule greps checked by reading: `\.WithProgress\(` matches exactly one non-test line (regenerate_fresh.go:84, inside `wire`), and `\.Remerge\(` matches exactly one (chunknotes.go:120, inside `RegenRun.regenerator`).
  - `RegenRun{...}` with a non-zero `Merged` is built in exactly one production place, chunknotes.go:98. The only other constructions are the zero values in tests and the two cmd tests the task names.
  - Divergence from the plan's wording: the plan called for "one `chunkNotesKeeper` helper", but the rule is a method on `RegenRun`. This is sound. `RegenerateFreshRegenerator` receives only a `RegenRun` (cmd/mint/regenerate_all.go:136), so the rule has to be reachable from the run, and the plan's own Do section allows it ("the helper therefore reaches the keeper through that run's reader and sinks"). Both verbs share one rule and one generator for the re-merge, which is what the task intends.
  - Behaviour equivalence: release now re-merges through the wired generator rather than the unwired one. `Remerge` → `sendMerge` → `generate` (internal/notes/remerge.go:51-57, chunk.go:82-91, classify.go:30-37) reads no progress, chunk-notes or merge-inputs sink, so nothing changes. On the regenerate side the live reader replaces the snapshot plus forwarding callback. The keeper's `keepMerged` sink is the one writer, and the cache-reuse assignment at release.go:1035 writes the same field, so `r` and `keepRegen` (chunknotes.go:68-73) read the same holder.

TESTS:
- Status: Adequate
- Coverage: The task is a refactor, and the existing tests carry each criterion:
  - Criterion 1: release_remerge_test.go:66, :157; release_chunkedcache_test.go:147, :171
  - Criterion 2: release_regen_sizecheck_test.go:187, :211, :284
  - Criterion 3: release_regenchunkfail_test.go:192, :207, :309
  - Criterion 4: regenerate_remerge_test.go:144, :192, :221
  - Criterion 5: regenerate_regenfail_test.go:392, both subtests (re-merge fails; stdin closes at the second nudge)
  - Criterion 6: regenerate_fresh_test.go:405, :438
  - Criterion 7: cmd/mint/regenerate_remerge_test.go:68, :78
  The single-holder change is observable: if the reader went back to a snapshot, the second `r` in release_regen_sizecheck_test.go:284 and regenerate_regen_sizecheck_test.go:101/:140 would re-run the ladder instead of re-merging, and those tests would fail.
- Notes: Per the implementation commit's stat, the only test edits are the two `RegenRun{Merged: ...}` constructions at cmd/mint/regenerate_remerge_test.go:73 and :83, as the task allows. They now pass a reader returning `&notes.MergeInputs{}`. The named zero-value constructions are unchanged; later tasks shifted their lines in regenerate_fresh_test.go to 371/418/448. No redundant tests were added.

CODE QUALITY:
- Project conventions: Followed (no new subprocess, output or config surface; the WHY-comments on `RegenRun`, `regenRun`, `RegenRun.regenerator`, `perRunRegenerator` and `RegenerateFreshRegenerator` match the as-built code)
- SOLID principles: Good — the sink chain and the `r` rule each live in one place, and each verb supplies only its size-check generation through `regenerateFunc`
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "Rule: the existing tests pass with their semantics untouched. ... All five CLAUDE.md gates pass" — run `go build ./...`, `gofmt -l .`, `go vet ./...`, `go test -race ./...` and `golangci-lint run` over the tree and confirm all are clean
