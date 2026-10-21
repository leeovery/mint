TASK: A SIZE answer re-runs a single-pass call in halved parts (token-aware-diff-sizing-4-3, tick-f5a6c9)

ACCEPTANCE CRITERIA:
- Given a single-pass call of S bytes that fails with output, and a classify answer that reduces to `SIZE` (`SIZE`, ` size `, `"SIZE"`, `` `Size` ``, `SIZE.`), no second single-pass call is made; the notes are written by chunk calls each no bigger than half of S, then one merge call, whose output ships byte-identical as the notes body (§8.6, §9.4, §10.1)
- Those chunk calls each carry the release's instructions, the same whole-release Change Map, part-k-of-n framing, its slice and the closing reminder; files are packed in path order, and together the calls carry every file's diff exactly once (§8.2, §8.3, §8.6, §10.1)
- A file whose diff alone exceeds a halved call's room is sent as pieces cut by the piece rules, each carrying the file's diff header and none bigger than half of S (§8.4, §10.1)
- A classify answer of `OTHER`, a sentence such as `This looks like a SIZE problem`, both codes such as `SIZE OTHER`, or an unknown word such as `LIMIT` makes no chunk call and no second classify call; the notes fail with the ✗ line `AI call failed` and the original call's captured output verbatim beneath it (§9.3, §9.4)
- `mint release regenerate <version>` from a fresh source, or a version under `regenerate --all`, whose single-pass call fails and is classified SIZE, has that version's notes written the same way over its own range (§1.3)

STATUS: complete

SPEC CONTEXT: §10.1 says a prompt under the cap can still overflow the model (a smaller-window model, an explicit command with its own overhead, a cap set above the real window). A SIZE answer on a diff-carrying call halves: the new target is half the failed call's composed size, and its slice is re-cut by the first wave's packing (§8.3) and piece (§8.4) rules, so a single-pass SIZE becomes a chunked run at half its size. §9.4 says to strip surrounding whitespace/quotes/backticks and trailing punctuation, then compare case-insensitively; only exact SIZE or OTHER is a listed code, anything else is OTHER, and there is no re-ask. §8.2/§8.6 say every chunk and merge call carries the same whole-release Change Map, and the merge output passes through byte-identical. §1.3 says regenerate's fresh source and `--all` share halving, per version.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/generate.go:226-241 — the single-pass prompt within the cap goes through g.generate. On success it returns the body. On codeSize it calls generateHalved with len(prompt) (the composed instructions + Change Map + diff + reminder) and the original failure. Any other failure returns `generating notes: %w` over the call's own error. No second single-pass call is made on any branch.
  - internal/notes/classify.go:30-47 — generate/readFailure: one transport call, then at most one classify call. The failure returned is always the notes call's own, except that a cancel during classify replaces it.
  - internal/notes/classify.go:53-70 — classify: only a GenerationError with output is asked about, only when the prompt fits the cap. A failed classify call reads as OTHER, and a cancel propagates.
  - internal/notes/classify.go:74-85 — readCode: strips leading whitespace, quotes, curly quotes and backticks, and trailing ones plus unicode punctuation, then uses strings.EqualFold against "SIZE". Everything else is codeOther, which matches §9.4 (leading punctuation is not stripped, as the spec words it).
  - internal/notes/chunk.go:41-48 — generateHalved: cutRun at promptBytes/2, Halved(0, n) reported in place of the split, then writeParts with halvings=1. The merge stays bounded by the cap (chunk.go:68-80).
  - internal/notes/chunk.go:377-406 (cutParts), 280-294 (splitFileDiffs, stable path-order sort), 419-433 (packChunks), 439-463 (cutPieces, header repeated per piece), 542-560 (composeChunkPrompt + part/piece framing) — the first wave's packing and piece rules, reused unchanged and aimed at the halved target. fixedPart (chunk.go:369-371) numbers the framing at a bound, so each call stays within target wherever it lands.
  - internal/notes/remerge.go:62-70 — chunkedBody returns the merge body unchanged (byte-identical) and hands over merge inputs.
  - internal/engine/regenerate_fresh.go:59-71 and cmd/mint/regenerate_all.go:117 — both the fresh single-version and `--all` producers run GenerateFromRange, which reaches the same generateFromDiffWithContext core over res.DiffRange(), so halving applies per version over its own range.
- Notes: No drift. The halved run reuses the release's instructions (including one-time context) and the same Change Map computed for the single-pass call, and runs through the same bounded wave (four in flight).

TESTS:
- Status: Adequate
- Coverage:
  - AC1: internal/notes/halve_test.go:77 is table-driven over `SIZE`, ` size `, `"SIZE"`, `` `Size` ``, `SIZE.`, plus `SIZE\n` and `'size'`. It asserts exactly one single-pass call, one classify call, at least 2 chunk calls each with 2*len ≤ S, one merge, and a byte-identical body (a trailing-whitespace body catches any trimming). internal/engine/notes_halve_test.go:74 proves the merge output lands byte-identical in the tag annotation and the CHANGELOG through a full Release.
  - AC2: internal/notes/halve_test.go:117 feeds files out of path order with a `[release].context`. It asserts part k-of-n numbering, the instructions + Change Map prefix, the closing-reminder suffix, the half-S bound, and that the slices joined in part order equal every file's diff once in path order.
  - AC3: internal/notes/halve_test.go:162 runs a 60 KB hunked file under the halved room. It asserts ≥ 2 pieces, each ≤ S/2, piece i-of-m framing, every piece opening with the file header (pieceBodies, chunkpieces_test.go:118), and byte-for-byte reassembly.
  - AC4: internal/notes/halve_test.go:198 covers OTHER, the sentence, both codes, LIMIT, an empty-after-stripping answer and a failing classify call. It asserts zero chunk/merge calls, exactly one classify call, CauseText "AI call failed", and the original GenerationError output. The rendered ✗ line plus verbatim output beneath it is pinned end-to-end by internal/engine/notes_classify_test.go:159.
  - AC5: internal/engine/notes_halve_test.go:113 (fresh, asserting the version's own `v1.0.0..v1.1.0` diff argv) and :132 (`--all`, middle version halved and written with the merge body).
- Notes: Each test would fail if its behaviour broke. A cap-sized target instead of S/2 would yield no split or oversized calls, a second single-pass call is counted, and a lax readCode would trip the OTHER table. No redundant or implementation-detail tests.

CODE QUALITY:
- Project conventions: Followed (transport seam, sentinel wrapping with %w, external test packages, t.Parallel, exact-output assertions)
- SOLID principles: Good — classification, cutting and the wave are separate units, and generateHalved reuses cutRun/writeParts rather than duplicating the chunked path
- Complexity: Low
- Modern idioms: Yes (strings.EqualFold, TrimLeft/RightFunc, slices helpers, min/max builtins)
- Readability: Good; the changed comments (generate.go:101-106, chunk.go:37-40, classify.go:49-52, 72-73) hold against the code
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
