TASK: Over-cap release notes: chunk calls and one merge (token-aware-diff-sizing-2-4, tick-5d01e7)

ACCEPTANCE CRITERIA:
- No max_prompt_bytes set, composed single-pass prompt <= 1,500,000 bytes (including a diff past 50,000 lines): exactly one single-pass call carrying the single-pass prompt, no chunk or merge call (§4.2, §7.2)
- Diff alone under the cap but composed single-pass prompt over it (e.g. long [release].context): no single-pass call, notes chunked (§7.2)
- Single-pass prompt over the cap: no single-pass call; chunk calls none bigger than the cap, each carrying the release's instructions (default+context, or the [release].prompt override replacing both), the whole-release Change Map, part-k-of-n framing, its slice, and the closing reminder last (§4.1, §8.2, §8.6)
- Over-cap release with both [release].prompt and [release].context: every chunk call and the merge carry the override's contents and none of the context text (§8.6)
- Files that each fit a chunk's room are packed in path order, each chunk filled until the next file's diff would not fit; together the chunks carry every file's diff exactly once (§8.3)
- After the chunk calls succeed, exactly one merge call: instructions, same Change Map, every chunk's notes in chunk order, framing asking for one set of release notes, closing reminder, no diff; output ships byte-identical as tag message, changelog entry and provider release (§8.1, §8.6)
- Chunked notes on an attended run reach the review gate as AI notes with the same choices as single-pass, r included (§15.1)
- regenerate <version> fresh on an over-cap version, or that version under --all, is chunked and merged the same way over its own range (§1.3, §7.2)
- Chunking has no setting and no consent prompt (§8.8)
- README documents chunking with mint release; mint setup's AI-transport paragraph covers the per-call cap and chunking (§18.1, §18.2)

STATUS: complete

SPEC CONTEXT: §7.2 orders the release AI path as exclusion check -> cache reuse -> size check -> one single-pass call under max_prompt_bytes, or chunked generation over it, with regenerate's fresh source running the same path minus the first two steps. §8.1 fixes the shape as map-reduce (parallel chunk calls, exactly one merge; refine and concatenation are out). §8.2/§8.6 put the same whole-release Change Map and the release's instructions in every chunk and merge call. §8.3 packs files in path order, each chunk's room being the cap minus its fixed part. §8.8 makes chunking always on with no key or consent prompt. §4.1: no call is ever bigger than the cap. §18.3 lists the line-count comments that go stale.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/generate.go:226-230 — ComposePrompt over instructions (with any one-time context), Change Map and diff; `len(prompt) > cfg.MaxPromptBytes` routes to generateChunked, otherwise one single-pass call (generate.go:232-235). The old CheckDiffSize line-count guard is gone (no remaining caller or definition).
  - internal/notes/chunk.go:28-35 generateChunked -> cutRun (chunk.go:50-55) -> writeParts (chunk.go:57-66) -> merge (chunk.go:68-80).
  - internal/notes/chunk.go:280-294 splitFileDiffs: one section per file, stable-sorted into path order by headerPath (chunk.go:300-321, rename/copy destination via destinationPath chunk.go:325-339).
  - internal/notes/chunk.go:369-371 fixedPart sizes the call with framing numbered at a bound (the diff's byte length), so a chunk fits whatever place it takes; chunk.go:377-406 cutParts computes room = target - fixedPart and packs fitting files with packChunks (chunk.go:419-433), which closes a chunk only when the next file would not fit. Each file lands in exactly one chunk.
  - internal/notes/chunk.go:542-544 composeChunkPrompt: instructions, Change Map, framing, slice, OutputReminder, in that order. chunk.go:548-560 framing names "part k of n".
  - internal/notes/chunk.go:562-575 composeMergePrompt: instructions, Change Map, merge framing ("write one set of release notes ... combine related changes across parts ..., drop duplicates, and rank by the Change Map"), each part's notes in chunk order, OutputReminder; no diff. chunk.go:68-72 refuses a merge prompt over the cap.
  - Instructions come from ResolveInstructions (internal/notes/prompt.go:125-138), so a [release].prompt override replaces default and context for the single-pass, chunk and merge calls alike.
  - Same transport for every call: forward release builds it once at internal/engine/release.go:1017-1018 (aitransport with VerbRelease, release.go:1321); regenerate fresh at internal/engine/regenerate_fresh.go:120-123/148-153; --all and single-version regenerate both reach engine.RegenerateFreshBody through cmd/mint/regenerate_all.go:98-120 (single-version via cmd/mint/regenerate_run.go:79-80).
  - Docs: README.md:127 (inside `### release`) documents the cap, path-order packing, instructions/Change Map/part framing per chunk, the one merge, byte-identical output, review gate with r, no setting/no prompt, and fresh regenerate per version. internal/setupguide/setupguide.go:168-175 (the "The AI transport" paragraph in overview()) covers max_prompt_bytes and chunking.
  - Stale line-count comments named by §18.3 are rewritten in generate.go, size.go, assemble.go, prompt.go, degenerate.go, regenerate_fresh.go and regenerate_batch.go. A search of internal/ and cmd/ non-test code finds max_diff_lines only in config.go's removed-key message.
- Notes: No config key or flag for chunking exists. The census in internal/config/metadata_census_test.go pins the full key set, and cmd/mint flags/usage carry no chunk/oversize flag. The release path asks nothing between the version gate and the notes review gate.

TESTS:
- Status: Adequate
- Coverage:
  - AC1: internal/notes/chunk_test.go:208 (60,000-line diff under the default cap -> exactly one call equal to the composed single-pass prompt, body passed through); :242 (prompt exactly at the cap -> one single-pass call, pinning the inclusive boundary).
  - AC2: chunk_test.go:257 (context file pushes the prompt over while the diff stays under; the fixture asserts both conditions; no single-pass prompt sent, >=2 chunk calls, one merge).
  - AC3: chunk_test.go:287 (three chunks: part k of 3, each under the cap, opens with instructions+context then the Change Map, framing between map and slice, slice equals the file's diff, ends with the reminder).
  - AC4: chunk_test.go:328 (every call, chunk and merge, opens with the override contents; none carries the context marker).
  - AC5: chunk_test.go:357 (out-of-order git emission restored to path order, exact per-chunk contents, each chunk stopped only when the next file would not fit); :502 (slices joined in part order reproduce the diff exactly, each call under the cap); :389/:456/:479 pin path-order keys for quoted, non-prefixed and renamed headers.
  - AC6: chunk_test.go:526 (one merge, last call, instructions+same Change Map, "one set of release notes" framing, part notes in chunk order, no diff content, reminder last, body byte-identical with leading/trailing whitespace); internal/engine/release_chunked_test.go:31 (over a 20000-byte cap: three notes AI calls each under the cap, merge carries both parts' notes, and the tag annotation body, CHANGELOG section and gh release stdin all equal the merge output byte-for-byte).
  - AC7 + AC9 (no consent): release_chunked_test.go:71 (notes gate offers y/n/e/r; only the version gate and the notes review gate are asked; no AskLine).
  - AC8: internal/engine/regenerate_fresh_test.go:298 (RegenerateFreshBody over v1.0.0..v1.1.0: two chunk calls and one merge, each under the cap, none carrying the whole diff, version's own range diffed); cmd/mint/regenerate_all_test.go:233 (the --all producer: merged body, three claude calls under the cap, first git call is the version's range diff); :367/:382 cover the batch and single-version producers on an over-cap fresh version.
  - AC10: internal/setupguide/setupguide_test.go:389 pins the transport paragraph's cap and chunking text.
- Notes: Each test targets a distinct criterion or a distinct path-order key form. The fixtures check their own preconditions (sizes over or under the cap), so a drifting fixture fails loudly instead of passing vacuously. The fake transports answer by prompt content, not call order, so they hold up under concurrent chunk calls.

CODE QUALITY:
- Project conventions: Followed. External test packages, t.Parallel throughout, FakeRunner and recording transports, exact argv on git calls, byte-identical body assertions. Every external call goes through the runner seam, and the transport is built only via aitransport.New.
- SOLID principles: Good. Splitting, packing, prompt composition and the merge are separate small functions over a cutter value, and the Generator keeps one transport seam.
- Complexity: Acceptable. The size branch in generateFromDiffWithContext is a single comparison, and the packing is linear.
- Modern idioms: Yes (slices, strings.Cut/CutPrefix, min builtin, %w wrapping).
- Readability: Good. The comments on the touched paths (generate.go:94-106, 191-199; chunk.go:17-27; assemble.go:138-141; regenerate_fresh.go:7-13) match the code as built.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
