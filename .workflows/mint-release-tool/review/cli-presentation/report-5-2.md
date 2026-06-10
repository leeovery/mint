TASK: cli-presentation-5-2 — Neutralise the hardcoded plain blocking-stage start verb

ACCEPTANCE CRITERIA:
- Plain StageStarted emits "{name}: running...\n" for blocking stages and remains silent for non-blocking stages.
- No occurrence of the stage-specific verb "generating" remains in plain start narration or its doc comment.
- The start line is still byte-pure ASCII (uses "...", not U+2026) and passes the byte-purity guard.
- `go test ./internal/presenter/...` and `go vet ./...` pass.

STATUS: Complete

SPEC CONTEXT: Event-payload principle (spec:64-66) — engine supplies every datum; presenter never re-derives engine knowledge or hardcodes stage-specific logic. StageStarted carries only Name+Blocking; plain uses Blocking to decide whether to emit a start line, must not invent stage-specific narration. The spec's worked example uses notes ("notes: generating…") but the spec also names the pre_tag build hook (prep) as blocking, for which "generating" is wrong. Wording is refinable; byte-purity is fixed.

IMPLEMENTATION:
- Status: Implemented
- Location: plain.go:153-158 (format string now "%s: running...\n"); doc comment plain.go:142-152 rewritten to describe a stage-agnostic synthesised verb.
- Notes: Minimal, spec-permitted edit. Non-blocking early-return unchanged (short stages stay silent). ASCII "..." preserved (not U+2026). No new payload field — line stays presenter-synthesised. No "generating" remains in plain narration/doc; surviving references are spinner.go:64 (pretty-only) and spec examples. golden_transcript_test.go:34 comment consistent; plain golden doesn't drive a blocking StageStarted.

TESTS:
- Status: Adequate
- Coverage: plain_test.go:142-165 (StageStartedHonoursBlocking — table: blocking=false→"", blocking=true→"notes: running...\n"); :255-266 (LongStageEmitsStartThenCompletion — ordered pair); :778-790 (EmitsNoANSIGlyphOrAnimationBytes — drives blocking StageStarted + assertBytePureASCII, so a U+2026 would fail).
- Notes: Both acceptance axes (verb text + silence) plus byte-purity covered. Not over-tested. Could not run go test/go vet (no shell); assessed by reading with exact-equality assertions.

CODE QUALITY:
- Project conventions: Followed — key:value vocabulary, writef pattern, doc-comment style.
- SOLID principles: Good — removing stage-specific assumption tightens the dependency-inverted seam.
- Complexity: Low.
- Modern idioms: Yes.
- Readability: Good — rewritten doc comment explains the why (event-payload principle).
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
