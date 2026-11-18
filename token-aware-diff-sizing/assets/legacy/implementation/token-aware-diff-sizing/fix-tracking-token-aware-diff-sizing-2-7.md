## Attempt 1

ISSUES:
- /Users/leeovery/Code/mint/internal/notes/chunkpieces_test.go:186-220 — No test checks that pieces are filled or that they sit in path order. Both regressions were reproduced on a scratch copy of the repo, and every test in internal/notes still passed:
  (a) Changing packPieceBodies' fill check (chunk.go:232) to `if current.Len() > 0 {` puts every hunk or line in its own piece. The 60 KB single-hunk fixture goes from 5 piece calls to 938, and the 12-hunk fixture from 6 to 12. A piece is a chunk call, and §8.3 says each chunk is filled until the next item would not fit. The whole-file path already has this check (chunk_test.go:380-385); the piece path, which Phase 4's halving re-cut will reuse, has none. A regression here would multiply AI calls with no test failing.
  (b) Moving every file's pieces after all the whole-file chunks (collecting them in cutParts and appending them at the end) also passes. That breaks §8.3's path order (big/huge.go's pieces land after gamma/c.go), even though the executor's report claims "the same path-order spot".
  FIX: (a) In TestGenerator_OverCap_PiecesOfHunksThatFit_CutOnlyBetweenHunks and TestGenerator_OverCap_HunkOverPieceRoom_CutBetweenLines, cap the piece count by the bytes: `len(bodies) <= 2*len(diff)/pieceRoomOverFixed + 1`. Under this cap a piece's body room is well over half of pieceRoomOverFixed, so this holds for the current code (6 and 5 pieces, against a bound of 9) and fails both broken versions (12 and 938). (b) In the table test TestGenerator_OverCap_FileOverChunkRoom_SentAsPieces, assert the part order from chunksInPartOrder: the first part's slice is exactly alpha/a.go's diff, the next len(pieces) parts are big/huge.go's pieces, and the last part's slice is exactly gamma/c.go's diff.
  ALTERNATIVE: For (a), copy chunk_test.go:380-385's pattern (`len(piece prompt) + len(next piece's first hunk or line) > cap`). It checks fill more tightly, but it needs a margin: framing is measured at the diff's byte length, so real prompts come in a few bytes under what was measured. Under the line-cut fixture, piece 2 plus its next 65-byte line is 10 bytes under the cap, so the strict check would fail on correct code. I recommend the count bound because it does not depend on that allowance.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/notes/chunk.go:140 — the claim "at most maxPromptBytes" is false when a piece has no room for diff. cutPieces (chunk.go:201-202) then sends the file whole, over the cap; I reproduced this with a 61 KB file sent as one non-piece part under a cap 50 bytes above the fixed part.
  OLD: // cutParts cuts files, in order, into chunk calls of at most maxPromptBytes: whole
  NEW: // cutParts cuts files, in order, into chunk calls: whole

NOTES:
- characterBoundary (chunk.go:258-266) walks back with no limit over bytes that look like UTF-8 continuation bytes (0x80-0xBF). On valid UTF-8 it stops within 3 bytes. Only a stretch of such bytes as long as the whole room (about 1.5 MB at the default cap) would shrink pieces to one byte each, so it is not reachable in practice. Capping the walk-back at utf8.UTFMax-1 would make it impossible.
- Measuring the framing at the diff's byte length rather than len(files) adds a few bytes of slack per chunk: a chunk can stop while the next item would have fit by about 10 bytes, as in the line-cut fixture above. This does not matter at real caps and keeps the cap guarantee, but it is why the strict fill check in the ALTERNATIVE needs a margin.
- packPieceBodies repeats packChunks' fill rule, and with items ≤ room it gives the same output. I did not raise it: two copies that can each be correct on their own do not clear the floor.
- The dispatch carried nothing beyond the enumerated inputs.

## Attempt 2

ISSUES:
- /Users/leeovery/Code/mint/internal/notes/chunkpieces_test.go:138-141: no test checks the framing bound behind the piece cap guarantee. That bound is at /Users/leeovery/Code/mint/internal/notes/chunk.go:145-150: room is measured with framing numbered at the diff's byte length. This task introduced it because pieces can outnumber files. Every fixture runs with 8 or fewer parts, so the real part and piece numbers have the same digit count as any measured number from 1 to 9. I tested this on a scratch copy. Replacing the bound with `maxParts := len(files)` (the measurement before this task) or with `maxParts := 1` still passes the whole internal/notes suite. Under either change, a release with ten or more parts sends piece prompts a few bytes over max_prompt_bytes. A large file cut into ten or more pieces is the normal case this task creates, and the gap is in AC1's "None is bigger than the cap".
  FIX: Add one row to the table in TestGenerator_OverCap_FileOverChunkRoom_SentAsPieces (after line 140): `{name: "pieces enough for ten or more parts", diff: minifiedFileDiff("big/huge.go", 150_000, 0)},`. That gives 13 parts. The minified line fills each piece to within 3 bytes of its room, so a framing measured one digit short pushes the prompt over the cap, and the existing per-piece cap check at line 159 catches it. Checked on a scratch copy: the row passes on the current code, including the header count, the framing, the byte-for-byte join and the part order. It fails both mutations: pieces come out at 18,111 bytes against an 18,109 cap.
  CONFIDENCE: high

NOTES:
- Fix round 1 checked. Changing packPieceBodies' fill check (chunk.go:232) to `if current.Len() > 0 {` now fails both fill bounds: 12 pieces against a limit of 8, and 924 against 9. The order checks at chunkpieces_test.go:177-191 fail if pieces move after the whole-file chunks, because parts[1] would then be gamma/c.go. The comment at chunk.go:140 matches the earlier correction.
- Other changes I tried on the scratch copy, each failing at least one test: a raw byte cut with no character-boundary walk-back; piece room measured with whole-file framing; always splitting hunks into lines; starting a new piece for a long line instead of filling the current one.
- chunk.go:140 now breaks after "whole", leaving a short line. That is a matter of line wrapping only; the comment is not wrong.
- The tests never send two oversized files in one release. Piece numbering restarts per file (chunk.go:216), which is clear from reading the code, and no acceptance criterion asks for it.
- The fix-tracking file on disk (/Users/leeovery/Code/mint/.workflows/token-aware-diff-sizing/implementation/token-aware-diff-sizing/fix-tracking-token-aware-diff-sizing-2-7.md) holds attempt 1's review. I read it for context only and did not treat it as input. The dispatch carried nothing beyond the inputs it listed.
