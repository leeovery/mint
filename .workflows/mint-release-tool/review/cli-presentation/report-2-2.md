TASK: cli-presentation-2-2 — Plain stage narration: start line for long/blocking stages only, completion line per stage

ACCEPTANCE CRITERIA:
- A blocking StageStarted emits a terse start line; a short (Blocking==false) StageStarted emits no start line.
- Every StageSucceeded emits exactly one completion line: {stage}: {detail} when detail present, {stage}: ok when empty.
- A blocking stage's completion line carries an elapsed suffix ((…s)); a short stage's carries none.
- A blocking stage emits start then completion (two lines); a short stage emits one line (completion only).
- Output is terse, lowercase, no ESC byte, no carriage return, no glyphs.

STATUS: Complete

SPEC CONTEXT: "The Plain Layer" per-event table (spec:215-228) and "Spinner Lifecycle" (spec:261-263): plain never animates, start line is plain's spinner-equivalent for long stages. Elapsed gated on Blocking (not Elapsed value); Elapsed==0 legal for blocking; Detail=="" falls back to ok.

IMPLEMENTATION:
- Status: Implemented (with one documented, spec-sanctioned wording choice)
- Location: internal/presenter/plain.go:153-158 (StageStarted), :178-192 (StageSucceeded); shared formatElapsed at pretty.go:921-923.
- Notes: StageStarted early-returns when !Blocking, else writes `{name}: running...`. StageSucceeded falls back detail->"ok", branches on Blocking for elapsed suffix. Elapsed gated on Blocking not value (short+nonzero shows none; blocking+0 shows (0.0s)). Writes to out only. No ANSI/glyphs; import guard protects (plain_test.go:1011). DRIFT (spec-sanctioned): start word is stage-agnostic `running...` (ASCII) vs spec example `generating…` (U+2026) — correct because (a) spec grants wording latitude, (b) StageStart payload carries only Name+Blocking so a stage-specific verb can't be derived without inventing narration (event-payload principle), (c) byte-purity requires ASCII. Documented at plain.go:142-152.

TESTS:
- Status: Adequate
- Coverage (plain_test.go): StageStartedHonoursBlocking (:142); StageSucceededFallsBackToOk (:114); StageSucceededElapsedSuffix (:194, 4 cases incl. blocking-gates-not-value + zero-elapsed); ShortStageEmitsOnlyCompletionLine (:240); LongStageEmitsStartThenCompletion (:255); BlockingStageEmitsNoAnimationBytes (:171, no braille/CR/ESC); EmitsNoANSIGlyphOrAnimationBytes (:781); golden_transcript_test.go exercises blocking start lines end-to-end.
- Notes: Every AC and edge case mapped to a concrete assertion. Exact-byte assertions. Minor justified overlap between suffix table and full-transition tests.

CODE QUALITY:
- Project conventions: Followed — fmt/io-only, terse lowercase, shared formatElapsed (DRY), documented fire-and-forget writef.
- SOLID principles: Good — single responsibility; never derives Blocking from Name.
- Complexity: Low.
- Modern idioms: Yes — early return, %s passthrough (no format-string injection).
- Readability: Good.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
