## Attempt 1

ISSUES:
- /Users/leeovery/Code/mint/internal/notes/chunk.go:165 vs :208. There is a data race on `slot.part`. `send` passes `slot.part` by value to `w.cut.recut` outside `w.mu`, and that copy reads every field, `piece`/`pieces` included. Meanwhile `halve`, running under the lock on another goroutine, renumbers `s.part.piece, s.part.pieces` on every slot of the same file, including slots whose calls are still in flight. Any two in-flight pieces of one file that are both classified SIZE race, whatever order their answers arrive in. That is the normal case for this feature: a cap set above the model's window makes every piece overflow together. I reproduced it on a copy of the module. With the existing helpers (`piecesGeneration` over `hunkedFileDiff("big/huge.go", 12, 5_000)`) and the first two big/huge.go pieces sent made to overflow, `go test -race` reports `WARNING: DATA RACE`: a write in `(*wave).halve` chunk.go:208 against a previous read in `(*wave).send` chunk.go:165. The suite misses it only because every current test overflows a single piece.
  FIX: Capture the part under the lock when the call launches and pass it in, so `send` never reads the shared slot's part outside the mutex. In `run`, `go w.send(ctx, slot, slot.part, w.prompt(slot))`. Change `send`'s signature to `send(ctx context.Context, slot *waveSlot, part chunkPart, prompt string)` and call `w.cut.recut(part, len(prompt)/2)`. `halve` keeps using `slot` (pointer identity, under the lock). Add a regression test in /Users/leeovery/Code/mint/internal/notes/halveparts_test.go in which two pieces of one file are both classified SIZE: `halvingTransport.overflows` returns true for the first two distinct big/huge.go slices sent. `go test -race` then guards it.
  ALTERNATIVE: Make `halve` renumber only slots still in `w.queue`, so in-flight and finished slots are never written. That also removes the race, but it leaves an unlocked read of shared state that the next edit can turn back into a race. I recommend passing the part copy in.
  CONFIDENCE: high
- /Users/leeovery/Code/mint/internal/notes/chunk.go:198-209. No test covers the piece renumbering in `halve`. I replaced the early return's condition with `if true` so renumbering never runs, and every test in ./internal/notes and ./internal/engine still passed. Without renumbering, pieces composed after a re-cut would name stale or repeated numbers ("piece 1 of 2" twice beside "piece 3 of 6"), contradicting §8.4 ("says which piece of how many it is"), and nothing would notice. My probe confirmed the current renumbering is correct: one of six pieces halved into four gives "piece 4..7 of 9", and later parts read "of 10".
  FIX: In `TestGenerator_PieceClassifiedSize_RecutIntoSmallerPiecesUnderTheFilesHeader` (halveparts_test.go:362), or in the new two-pieces test from ISSUE 1, assert that the re-cut pieces carry consecutive piece numbers in file order and name the grown total M = original pieces − 1 + re-cut pieces. Read the numbers with `pieceOf` on the `halved` prompts after `inPartOrder`. For example, the i-th re-cut piece names piece (failedIndex + i) of M, and every piece of the file composed after the halving names "of M".
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/notes/chunk.go:23-25 — a halving round with no room now returns ErrNoRoomForDiff mid-wave, after calls were sent, so "before any call" is false
  OLD: // over its bound is sent: ErrNoRoomForDiff before any call, ErrMergeTooLarge in
// place of the merge. A merge call whose failure is classified SIZE is never halved:
// it fails as ErrMergeTooLarge, carrying the call's own failure.
  NEW: // over its bound is sent: ErrNoRoomForDiff in place of a call with no room for diff,
// ErrMergeTooLarge in place of the merge. A merge call whose failure is classified
// SIZE is never halved: it fails as ErrMergeTooLarge, carrying the call's own failure.
- /Users/leeovery/Code/mint/internal/notes/size.go:13 — this task now also returns the sentinel for a merge call that was sent and classified SIZE
  OLD: // ErrMergeTooLarge reports a merge prompt over max_prompt_bytes, left unsent.
  NEW: // ErrMergeTooLarge reports a merge prompt over max_prompt_bytes, left unsent, or a
// merge call whose failure is classified SIZE.
- /Users/leeovery/Code/mint/internal/notes/size.go:10-11 — a halving round with no room now returns it after calls were sent, and its room is under the halved target, not max_prompt_bytes
  OLD: // ErrNoRoomForDiff reports a chunked run sending no call at all, because one of
	// its calls has no room under max_prompt_bytes for the diff it must carry.
  NEW: // ErrNoRoomForDiff reports a chunked run's call left unsent, because it has no
	// room under its bound for the diff it must carry.
- /Users/leeovery/Code/mint/internal/notes/halveparts_test.go:23 — garbled clause: it is missing "for", so it reads as though the slice reports
  OLD: // A chunk call whose slice overflows reports true fails with an overflow's output;
  NEW: // A chunk call whose slice overflows reports true for fails with an overflow's output;
- /Users/leeovery/Code/mint/internal/engine/notes_halveparts_test.go:59-61 — the helper never checks ordering against the merge; it only counts one merge
  OLD: // assertCorePartHalved fails unless the core/b.go part overflowed once, its classify
// call read SIZE, and its slice was re-sent in calls each at most half its size, all
// before one merge.
  NEW: // assertCorePartHalved fails unless the core/b.go part overflowed once, its classify
// call read SIZE, its slice was re-sent in calls each at most half its size, and one
// merge call was made.

NOTES:
- For now, when halving runs out of room, `w.failure` is set to the bare `ErrNoRoomForDiff`, which drops the overflowing call's captured output and location (chunk.go:179-180). The executor explicitly left this for 4-7/4-9, matching the task's Context (§10.2: "task 4-7 builds both limits").
- `g.generate` still classifies in-flight calls that fail after a final failure has ended the wave. §11.2 ("makes no further classify calls") is task 4-9's. The wave's `failure` field is the natural gate for it.
- Heads-up for 4-7: the halving target `len(prompt)/2` is written twice, at generate.go:223 (single-pass) and chunk.go:165 (wave). A single-pass SIZE already counts as a slice's first halving, so the wave's depth count must start at one for slices born of it, or that run gets one more round than §10.2's three.
- Probe scratch copies were made in the scratchpad and removed afterwards. The repo is unchanged.
- Nothing arrived beyond the enumerated inputs.
