TASK: Pieces for a file too big for one call (token-aware-diff-sizing-2-7)

ACCEPTANCE CRITERIA:
- Given an over-cap release with a file whose diff alone exceeds a chunk call's room, that file is sent as piece calls. None is bigger than the cap, and each carries only that file's diff, never another file's (§8.4)
- Given that file's hunks each fit a piece, every cut falls between hunks (§8.4)
- Given a hunk bigger than a piece's room, it is cut between lines (§8.4)
- Given a single line longer than the room, such as a minified bundle, it is cut at a character boundary, never inside a multi-byte character (§8.4)
- Each piece opens with the file's diff header, and its framing names the file and which piece of how many it is (§8.4, §8.6)
- Given a file sent as pieces, the pieces' diff content joined in order, with the file's header kept once, reproduces the file's diff byte for byte (§8.4)
- Given a release whose other files are packed into chunks, the pieces run alongside those chunk calls under the four-at-once bound, and their notes reach the merge in chunk order like any other chunk's (§8.4, §8.5, §8.6)

STATUS: complete

SPEC CONTEXT: §8.3 gives a chunk call room for diff equal to the cap minus its fixed part (instructions, Change Map, framing, reminder), with files packed in path order. §8.4 splits a file whose diff alone exceeds that room into piece calls: cuts at hunk boundaries, else line boundaries, a single over-room line cut at a character boundary; each piece carries the file's diff header and names which piece of how many; nothing is dropped. §8.5 bounds a wave at four calls in flight. §8.6 has a piece's framing name the file and its piece number, and has the merge carry every part's notes in chunk order. §10.1 has halving re-cut a failed slice by the same packing and piece rules.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/chunk.go:377-406 (`cutParts`: files that fit the chunk room are packed via `packChunks`. The pending fitting files are flushed before a piece set, so a piece is never packed with another file. An over-room file is cut by `cutPieces` in its path-order place)
  - internal/notes/chunk.go:439-463 (`cutPieces`: the room comes from the piece framing at `numberBound` and the header's length is subtracted. Hunks that fit stay whole segments. A hunk over the room becomes line segments via `strings.SplitAfter`. Each piece's slice is `header + body` and carries `piece i of len(bodies)`)
  - internal/notes/chunk.go:469-503 (`packPieceBodies`: packs segments greedily. An over-room line fills the rest of the current piece and as many more as it needs, each cut at `characterBoundary`. A character longer than the room returns ErrNoRoomForDiff)
  - internal/notes/chunk.go:507-515 (`characterBoundary`, which steps back from n past UTF-8 continuation bytes)
  - internal/notes/chunk.go:519-526 (`splitFileHeader`), internal/notes/chunk.go:530-540 (`splitBefore`)
  - internal/notes/chunk.go:548-560 (piece framing: "part k of n: piece i of m of the diff for {file}")
  - internal/notes/chunk.go:367-371 (`fixedPart` numbers the framing at `numberBound`, the diff's byte length, so a piece fits the cap whatever place it takes)
  - internal/notes/chunk.go:168-202, 116 (pieces are ordinary wave slots under `maxChunkCallsInFlight = 4`. Notes are kept per slot in chunk order and passed to `composeMergePrompt` at :562-568)
  - internal/notes/chunk.go:408-415, 253-275 (`recut` of a piece re-cuts its body under the same header. `halve` renumbers that file's pieces by their place)
- Notes: Traced every AC against the code.
  - Cap: a piece's prompt is at most its fixed part (framing at the numeric bound, same file path) plus header plus body, and the body is at most bodyRoom, so the prompt stays at or under the target.
  - Byte-for-byte: every segment, and every part of a segment cut across pieces, is written to exactly one body, in order. Header plus the concatenated bodies is the file.
  - One file per piece: the fitting files are flushed before `cutPieces`, and a piece's slice holds only `header + body`.
  - Piece count: a file over the chunk room always yields at least two pieces, because the piece framing is strictly longer than the whole-file framing.
  - Path names: headers are forced to `a/`/`b/` (`--src-prefix=a/ --dst-prefix=b/`, internal/notes/assemble.go:184), so `headerPath` names the real path in the framing.
  No drift from the plan.

TESTS:
- Status: Adequate
- Coverage:
  - internal/notes/chunkpieces_test.go:131-195 (table over four shapes: many hunks, one oversized hunk, one long line, and a line long enough for 10 or more pieces, which exercises two-digit numbering against the cap). For each piece it asserts:
    - the piece is at or under the cap
    - the slice holds exactly one `diff --git ` header
    - the framing names `piece i of m` and the file
    - the slice opens with the header
    - the pieces joined, header kept once, equal the file's diff
    It also asserts placement: alpha's chunk, then the pieces, then gamma's chunk.
  - :197-216 every piece body starts with `@@ -` when the hunks fit (a cut inside a hunk would fail this), plus a greedy-fill bound
  - :218-237 every piece body ends with `\n` when one hunk is over the room, plus a greedy-fill bound
  - :239-270 eight lead offsets against an 8-byte `a€😀` run guarantee that a byte-count cut would land inside a 3- or 4-byte character; every body must be valid UTF-8, piece 1 must end mid-line, and the pieces must join to the diff
  - :290-332 pieces and two chunk calls are held, four reach flight together, and the merge carries each part's notes, keyed to its framing's part number, under `Notes for part k of n` in order
  - internal/notes/chunk_internal_test.go:9-24 and :57-73 cover ErrNoRoomForDiff when the piece framing plus header leaves no room, and when the room is smaller than a two-byte character
- Notes: Each test would fail if the behaviour it names broke. No redundant or implementation-detail assertions.

CODE QUALITY:
- Project conventions: Followed (external test package for behaviour tests, internal tests only for cutter edges, `t.Parallel()` throughout, table-driven where the shape fits, WHY-comments true to the code, no process-artifact references)
- SOLID principles: Good (cutting, packing, character boundary and framing are separate functions; the wave is agnostic to whether a slot is a chunk or a piece)
- Complexity: Acceptable (`packPieceBodies` has the most branches and stays readable)
- Modern idioms: Yes (`slices`, `strings.CutPrefix`, `min`, `range` over int)
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
