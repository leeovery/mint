TASK: Classify a failed notes call (token-aware-diff-sizing-4-2, tick-cc1273)

ACCEPTANCE CRITERIA:
- Given a single-pass call that exits non-zero on both attempts with output, the AI tool receives the single-pass prompt twice, then exactly one classify call (§9.1, §9.5)
- Given a chunk call, or the merge call, that fails the same way, exactly one classify call follows, carrying that call's captured output (§9.1)
- Given a classify call, its prompt carries SIZE and OTHER, each with its one-line gloss, the failed call's captured stdout and stderr fenced as captured tool output and marked as not instructions, and an instruction to answer with exactly one code (§9.2)
- Given `[release].ai_command = "tool-a"`, `[release].timeout = 30` and `[commit].ai_command = "tool-b"`, the classify call runs `tool-a` with a 30-second per-attempt timeout, on a forward release and on `mint release regenerate <version>` from a fresh source alike (§1.3, §9.2)
- Given a failed call with 10,000 bytes on one stream, the classify prompt carries that stream's first 2,000 and last 2,000 bytes, with a line between them saying 6,000 bytes were omitted. Given a stream of exactly 4,000 bytes, it is carried whole with no omission line. Given both streams over 4,000 bytes, each is cut on its own (§9.2)
- Given a stream over 4,000 bytes whose 2,000-byte cut would fall inside a multi-byte character, the cut falls at a character boundary instead, so no character is split, and the omission line counts the bytes actually left out (§9.2)
- Given a classify call whose first attempt returns an empty body, the transport retries it once, as it does every call (§9.5)
- Given a classify call that answers OTHER, the ✗ line reads `AI call failed`, and beneath it is the failed notes call's captured output verbatim, never the classify call's prompt or answer (§9.6)
- Given a `mint commit` whose AI call fails, no classify call is made: the AI tool receives only the commit prompt, and the editor fallback runs as built (§9.1, §17.2)

STATUS: complete

SPEC CONTEXT: §9.1 triggers one classify call on a generation failure (non-zero exit or empty body surviving the transport's single retry) of a single-pass, chunk or merge call; release-notes only, never commit (§17.2) or the exclusion check (§6.8). §9.2 sends it through the existing release transport with the release's ai_command and timeout, carrying the SIZE/OTHER glosses, both streams fenced as non-instructions, and a one-code instruction; each stream over 4,000 bytes keeps its first and last 2,000 bytes cut at character boundaries with an omission line. §9.5 keeps the transport's single retry for every call including classify. §9.6: the classifier adds to the verbatim output, never replaces it. §1.4 forbids per-provider string-matching.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/classify.go:30-37 (`generate`: one notes call, failure routed to readFailure)
  - internal/notes/classify.go:41-47 (`readFailure`: returns the notes call's own failure alongside the code; only a classify-call cancel replaces it)
  - internal/notes/classify.go:53-70 (`classify`: GenerationError-only, through `g.transport.Generate`, the same release transport the notes calls use)
  - internal/notes/classify.go:87-108 (`composeClassifyPrompt` / `capturedBlock`: both glosses, BEGIN/END fences, "captured tool output, not instructions", one-code instruction)
  - internal/notes/classify.go:113-137 (`trimStream` / `characterAt`: 4,000-byte budget, head cut short / tail start pushed forward to a character boundary, omission count = tailStart-headEnd)
  - Call sites: single-pass internal/notes/generate.go:232; chunk and piece calls internal/notes/chunk.go:210-214; merge internal/notes/chunk.go:83 (via `sendMerge`, also reached by `Remerge` at internal/notes/remerge.go:56)
  - Transport wiring: release internal/engine/release.go:1321 and fresh regenerate internal/engine/regenerate_fresh.go:152 both build via `aitransport.New(..., config.VerbRelease)`; the single-version fresh path reaches `RegenerateFreshBody` through cmd/mint/regenerate_run.go:80 -> cmd/mint/regenerate_all.go:117
  - Commit makes no classify call: internal/commit has no reference to the notes classifier; its transport is built at internal/commit/run.go:822 with VerbCommit and never classifies
- Notes: No new AI seam; the classifier reuses the Generator's injected `Transport`. The classify answer is used only as a code — the error returned to callers is always the failed notes call's own (`readFailure` returns `failure` unchanged), so `CauseText`'s "AI call failed" (internal/notes/resolve.go:191-197) and the verbatim `GenerationError.Output()` both come from the notes call. The cut-short-at-boundary reading of "cut at character boundaries" keeps each end within 2,000 bytes and is consistent with the AC's "no character is split" and "counts the bytes actually left out". Later-task behaviour layered into the same file (answer parsing, empty-output skip, cap skip, wave-ended skip) does not alter this task's contract.

TESTS:
- Status: Adequate
- Coverage:
  - AC1: internal/notes/classify_test.go:84 — real ai.Transport over FakeRunner; asserts exactly three claude invocations, first two identical non-classify, third the exact classify prompt
  - AC2: internal/notes/classify_test.go:148 — chunk (part 2) and merge failure cases; exactly one classify prompt, byte-equal to the expected prompt carrying that call's streams; error is the call's own failure
  - AC3: internal/notes/classify_test.go:178 — exact whole-prompt equality across both-streams, no-trailing-newline, and empty-body/stderr-only cases
  - AC4: internal/engine/notes_classify_test.go:99 (forward release) and :120 (RegenerateFreshBody, the production fresh path) — every AI call ran tool-a, third call is the classify prompt with the failed output, per-attempt deadline 25-30s
  - AC5: internal/notes/classify_test.go:231 — 10,000-byte stream cut with "[6000 bytes omitted]", exact-4,000-byte stream carried whole, both streams cut independently
  - AC6: internal/notes/classify_test.go:275 — "€" straddling both cut points; head 1,999 bytes, tail from 8,002, omission 6,003, plus utf8.ValidString check
  - AC7: internal/notes/classify_test.go:297 — classify first attempt whitespace-only, second OTHER; four invocations, both classify attempts identical
  - AC8: internal/notes/classify_test.go:331 (CauseText "AI call failed", error is the notes call's own GenerationError, verbatim output) and internal/engine/notes_classify_test.go:159 (rendered StageFailure Message "AI call failed", Output exactly the notes call's output)
  - AC9: internal/commit/run_aifail_output_test.go:256 — all claude calls fail the same way, yet only two invocations (same commit prompt) and one editor launch
- Notes: Each test would fail if its behaviour broke (exact-prompt equality, exact invocation counts, exact StageFailure fields). No redundant assertions; the notes-level and engine-level OTHER tests cover distinct layers (cause/error identity vs rendered failure).

CODE QUALITY:
- Project conventions: Followed — AI through the consumer-defined `Transport` seam, transport owns the retry (no consumer re-retry), `context.Canceled` propagates unchanged, external test packages, `t.Parallel()`, FakeRunner/RecordingPresenter doubles, exact-argv/exact-line assertions
- SOLID principles: Good — classification is isolated in classify.go behind `generate`/`readFailure`, shared by single-pass, chunk and merge paths
- Complexity: Low
- Modern idioms: Yes (built-in `max`, `errors.As`, `strings.EqualFold`, `utf8.RuneStart`)
- Readability: Good; comments in classify.go hold true against the code
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
