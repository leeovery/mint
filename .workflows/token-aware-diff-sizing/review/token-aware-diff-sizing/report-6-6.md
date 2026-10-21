TASK: Chunked notes come back from the notes cache with their chunk notes (token-aware-diff-sizing-6-6, tick-88874b)

ACCEPTANCE CRITERIA:
- Given `mint release --dry-run` whose notes were written as 4 parts and a merge, the following real run for the same diff, version and instructions within the TTL asks `Cached notes found for this diff — use them?`. Choosing use shows the dry run's body byte-identical, and no chunk, merge or single-pass call is made (§7.2, §15.2)
- Given those reused notes at the review gate, `r` with a nudge makes exactly one AI call: a merge carrying the dry run's four parts' notes in chunk order and the nudge (§15.1, §15.2)
- Given a real run whose notes were written as parts and a merge and which was then aborted at the review gate, the next real run for the same inputs offers its notes, and `r` on them makes only the merge call, over that run's part notes (§15.2)
- Given a cached chunked entry older than the TTL, the real run does not offer it and writes the notes afresh (§15.2)
- Given a run that writes a cached chunked entry, the repo's work tree and index are unchanged by the write. Given a later run with a changed diff that writes its own entry, nothing of the earlier entry, its chunk notes included, remains in the repo's cache directory (§15.2)
- Given cached notes from a single-pass run, reusing them and choosing `r` re-runs the AI path from the size check, as on live single-pass notes (§15.1, §15.2)
- Given `mint release regenerate <version>` from a fresh source, single-version or under `--all`, whose notes are chunked, no cached notes are offered and no cache entry is written (§7.2)

STATUS: complete

SPEC CONTEXT: §15.2 (with the 2026-10-19 corrigendum): the notes cache keys on inputs (post-exclusion diff, version, resolved instructions), so chunked notes are cached and offered back exactly like single-pass notes. A chunked body is cached together with its per-chunk notes whichever run (dry or real) wrote it, so `r` on reused chunked notes re-runs only the merge. The chunk notes share the entry's lifecycle (outside the repo, pruned to the latest entry per project, stale after ~1h, keyed on the version); outside the entry they live only in memory. §15.1: `r` on chunked notes is one merge call carrying the nudge; on single-pass notes it re-runs the AI path from the size check. §7.2: regenerate's fresh source neither offers nor writes the cache. §18.3: comments stating a changed contract change in the same commit.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notescache/cache.go:48-57 — `Entry` gains `Merge *notes.MergeInputs` (`json:"merge,omitempty"`), nil for single-pass notes; old body-only entries decode with Merge nil
  - internal/notescache/cache.go:104-119 — `Store.Write(repoRoot, key, body, merge)` persists the merge inputs in the same entry, same key, same atomic write; Lookup/TTL/Prune unchanged, so the chunk notes share the entry's lifecycle with no cleanup of their own
  - internal/notes/remerge.go:18-37 — `MergeInputs` JSON round trip (instructions, change map, part notes) over its unexported fields
  - internal/notes/generate.go:229 and :239 — both the over-cap chunked run and the halved single-pass run hand their merge inputs through `chunkedBody` (internal/notes/remerge.go:62-70), so halved notes are cached the same way
  - internal/engine/release.go:1032-1037 — the `reuse` hook marks the body reused and loads the entry's `Merge` into the run's `chunkNotesKeeper`; internal/engine/release.go:1045-1055 — a fresh normal-AI body is written with `chunkNotes.merged` (nil for single-pass), on dry and real runs alike, then the repo's other entries are pruned
  - internal/engine/release.go:1255-1288 — `cacheReuse` hands the live entry to `reuse` on both the silent dry-run path and the real run's `use cached` choice; the `regenerate` choice leaves the keeper untouched
  - internal/engine/chunknotes.go:97-103, :115-125 — the gate's `r` reads the keeper's merge inputs live, so reused chunked notes re-merge (one `Remerge` call) and reused single-pass notes (Merge nil) fall through to the size-check generation
  - Regenerate never references `NoteCache` (no hits in internal/engine/regenerate*.go), and cmd/mint/main.go:186-195 wires no NoteCache into the regenerate deps
  - Fallback bodies return zero `CacheInputs` (internal/notes/select.go:233-237), so a fallback is never cached with stale merge inputs; `keepMergeInputs` fires only on a successful chunked run, so a failed wave leaves `merged` nil
- Notes: The seam and package docs the task named (package doc, `Entry` doc, `ReleaseDeps.NoteCache`, `NoteCache`/`NoteCacheWriter`/`NoteCacheReader`) were rewritten to the new contract in the same commit and hold against the code. No drift from the plan's Do: key unchanged, chunk notes held by `notescache.Entry`, written in `resolveBody`, returned via `cacheReuse`.

TESTS:
- Status: Adequate
- Coverage:
  - AC1: internal/engine/release_chunkedcache_test.go:119 — asserts the reuse question, the shown body equals the dry run's merge output, zero non-check prompts on the real run, and the tag annotation carries the dry run's body
  - AC2: :147 — exactly one non-check prompt, byte-equal to the dry run's merge prompt with the nudge inserted (`withNudge`), parts 1-4 in order (`assertPartsInOrder`); this also proves the MergeInputs JSON round trip end to end
  - AC3: :171 — real run aborted at the review gate (version Yes, review No), next run is offered the entry and its single re-merge is byte-equal to the aborted run's merge prompt plus nudge
  - AC4: :198 — entry written at cacheWriteClock read at TTL+1min: no reuse question, five fresh calls, entry rewritten with the fresh body and stamp, raw JSON holds only the fresh part notes
  - AC5: :234 (work tree snapshot unchanged, no mutation, no `git add`) and :266 (after a changed-diff run the cache dir holds only the later key and no file contains the earlier part notes)
  - AC6: :292 — seeded single-pass entry (Merge nil), reuse then `r`: one single-pass call carrying the nudge over the re-assembled diff (two diff assemblies)
  - AC7: :331 (single-version) and :360 (`--all`) — NoteCache deliberately wired on the deps, still no reuse question and an empty cache base
  - Unit: internal/notescache/chunked_test.go:22 and :53 — chunked entry round-trips its merge inputs; single-pass entry carries none
- Notes: Every criterion has a test that would fail if the behaviour broke. No redundancy: each test pins a distinct criterion. The halved-run-to-cache path has no dedicated test, but it shares the single `chunkedBody` -> keeper -> `Write` path with the chunked run tests exercise, so a break specific to it has no separate route.

CODE QUALITY:
- Project conventions: Followed (consumer-defined seams, external test packages, `t.Parallel()`, `t.TempDir()` roots, exact-prompt assertions, no real subprocesses)
- SOLID principles: Good — the cache stays a store; the engine owns what is put in it; `MergeInputs` keeps its fields unexported behind explicit JSON methods
- Complexity: Low — the change is one extra field threaded through Write, and a reuse callback replacing a bool pointer
- Modern idioms: Yes
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
