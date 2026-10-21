TASK: Name the cause on a notes failure's ✗ line (token-aware-diff-sizing-4-1)

ACCEPTANCE CRITERIA:
- Single-pass call exiting non-zero on both attempts with `Not logged in · Please run /login` on stdout → ✗ line `AI call failed`, that output verbatim beneath
- Single-pass failure with stdout empty and a message on stderr → `AI call failed`, stderr message verbatim beneath
- Single-pass empty/whitespace body on both attempts, only whitespace on stderr → `AI returned empty notes`, no output beneath
- Single-pass timeout → `AI timed out`; uninstalled program → `AI tool not installed`; neither carries a location
- Chunked run of 6 parts, part 3 fails with output → `AI call failed (part 3 of 6)` with output beneath; timeout → `AI timed out (part 3 of 6)`; empty notes → `AI returned empty notes (part 3 of 6)`; a failed piece call reads the same with its own part number
- Chunk calls succeed, merge fails with output → `AI call failed (merging)` with the merge's output beneath; merge timeout → `AI timed out (merging)`
- Chunked run failing as `no room for the diff under max_prompt_bytes` or `part notes too large to merge` (merge prompt over cap) → that phrase with no location
- `mint release regenerate <version>` from a fresh source → same phrase, location and output
- No release-notes failure, forward or regenerate, reads `AI returned empty/invalid notes after retry`

STATUS: complete

SPEC CONTEXT: §12.4's cause-phrase table replaces the single misnaming phrase `AI returned empty/invalid notes after retry` with per-cause phrases (`AI call failed` / `AI returned empty notes` split on whether either captured stream has non-whitespace text; timeout and missing binary as built). The 2026-10-19 corrigendum fixes the suffix rule: the suffix names the call the failure belongs to — `(part k of n)` or `(merging)` — and `part notes too large to merge` plus failures no single call owns (no room for diff) take none. §9.6 keeps the AI's captured output verbatim beneath the ✗ line. §1.3 makes regenerate <version> fresh share the phrases.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/resolve.go:164-189 — CauseText: maps each ClassifyFailure cause to its phrase; NoRoomForDiff and MergeTooLarge return bare (lines 171-174); every other cause takes ` (<location>)` when a *callError is in the chain (lines 184-187)
  - internal/notes/resolve.go:191-197 — generationFailurePhrase: `AI call failed` when the chain's *ai.GenerationError has non-empty Output(), else `AI returned empty notes` (also covers a bare ai.ErrGenerationFailed with no carrier)
  - internal/notes/chunk.go:93-108 — callError carrying the call's location; partCallError builds `part k of n`
  - internal/notes/chunk.go:88 — failed merge call wrapped as callError `merging`; chunk.go:86 merge SIZE answer and chunk.go:71 over-cap merge prompt produce ErrMergeTooLarge with no callError, so no suffix
  - internal/notes/chunk.go:237,239 — the wave's first final part failure wrapped via partCallError
  - internal/engine/release.go:2042-2055 — failureMessage delegates to notes.CauseText; release.go:2009-2015 notesFailureOutput renders the same *ai.GenerationError's Output(), so the phrase choice and the output beneath read the one carrier and cannot disagree
  - internal/ai/transport.go:97-105 — GenerationError.Output decides emptiness on each stream's trimmed view, matching the "empty or whitespace-only" rule
- Notes: Single-pass failures (internal/notes/generate.go:241) wrap only with "generating notes: %w" — no callError, so no location. Every notes ✗ site (surface, surfaceAndUnwind, release.go:565, regenerate_interactive.go:237, notesfloor.go) funnels through stageFailure → failureMessage/failureOutput, so forward release and regenerate <version> share the phrases. The old phrase is absent from all Go code, README, CLAUDE.md and the setup guide (only historical .workflows documents mention it). ClassifyFailure (resolve.go:140-156) was introduced by later tasks; the 4-1 behaviour is preserved through it.

TESTS:
- Status: Adequate
- Coverage:
  - internal/engine/notes_causephrase_test.go:54 and :80 — single-pass table (stdout output, stderr-only output, whitespace on both, timeout, missing program) asserting exact Message and Output on both the forward release and RegenerateRun fresh (AC 1-4, 8)
  - internal/engine/release_priortag_test.go:743 and internal/engine/notes_failure_no_line_ceiling_test.go:24 — production transport via FakeRunner with a non-zero claude exit on both attempts, asserting `AI call failed` and the verbatim stdout end-to-end
  - internal/notes/causetext_test.go:87 — part 3 of 6 with output / timeout / empty notes / missing binary; :134 piece call names its own part; :154 merge with output / timeout / empty; :181 no-room and over-cap merge carry no location; :14 single-pass phrases on both chain shapes; :62 old phrase never returned (AC 5-7, 9)
  - internal/engine/notes_chunkfail_test.go:126 and :204 — chunked engine runs (chunk fails with output, chunk times out, merge fails with output, merge returns empty) asserting exact Message and Output on forward release and regenerate (AC 5, 6, 8)
  - internal/engine/notes_cannotcall_test.go:89 and :203 — no-room and merge-too-large ✗ lines on forward release and regenerate
- Notes: Each test fails if the phrase, suffix or output wiring breaks. TestCauseText_NoOldPhraseForAnyGenerationFailure is implied by the exact-match tests but pins AC 9 directly at small cost.

CODE QUALITY:
- Project conventions: Followed — sentinels matched via errors.Is/errors.As through %w chains; display derived payload-side via the single failureMessage seam; the AI's output passes through verbatim
- SOLID principles: Good — the phrase derivation stays in notes.CauseText; location is carried by a small typed wrapper rather than parsed from error text
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
