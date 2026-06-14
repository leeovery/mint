TASK: ai-model-selection-4-2 — Rewrite forward-looking phase/task comment narration in config/verb to match as-built code

ACCEPTANCE CRITERIA:
- None of the enumerated comments in internal/config/verb.go or internal/config/config.go describe an already-shipped behaviour as future, deferred, or owned by a later task/phase.
- The WHY/contract content of each comment is preserved; only the task/phase tense is removed.
- The genuinely-historical "Phase 1" notes and the out-of-scope "Phase 6 provider validation" carve-outs remain unchanged and accurate.
- No code behaviour changes — comment-only edits.
- All gates pass (build, gofmt, go vet, go test -race, golangci-lint).

STATUS: Complete

SPEC CONTEXT: The ai-model-selection feature pins the shipped default to `claude -p --model sonnet`, promotes ai_command/timeout to per-verb-overridable keys resolving [verb] → top-level shared → shipped default, and establishes internal/config as the single source of truth for defaults (removing the transport's self-default and sourcing initgen's scaffold from config constants). CLAUDE.md's Comments contract requires WHY-comments be TRUE TO AS-BUILT with no scope/phase claims contradicting shipped code. This task removes the forward-looking tense from comments that described work completed in this same change as still-future.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/config/config.go, internal/config/verb.go (commit f547e57, comment-only).
- Notes: All 8 enumerated sites verified rewritten to present/as-built tense:
  - verb.go:4-5 — "the layered accessors (AICommandFor / TimeoutFor) accept" — "arriving in later tasks" removed.
  - config.go:79-82 (DefaultAICommand) — "the transport carries no self-default of its own, and initgen's scaffold literal is sourced from this constant" — "in later phases" removed.
  - config.go:89-92 (DefaultTimeout) — "the transport carries no defaultTimeout literal of its own" — "which Phase 2 deletes" removed.
  - config.go:580-590 (TimeoutFor doc) — "ai.Config.Timeout is also *time.Duration and takes this accessor's return by DIRECT ASSIGNMENT ... that conditional lives in the transport, NOT in this accessor" — "Phase 2's ..." / "lives in Phase 2, NOT here" removed; the WHY (conditional belongs in transport as a call-wiring concern) preserved.
  - config.go:139-140, 208-209, 258, 653-654 (negative-drop/floor) — all now "TimeoutFor drops a/the negative to the floor"; no "1-7's job" / "Task 1-7's TimeoutFor accessor" remains.
  - config.go:101-104, 198-201, 247-250, 695-697 (resolver/[commit] table) — resolver and [commit] table described as present; "the resolver (1-4) needs" / "arrive in later phases" removed.
  - As-built claims cross-checked against code: transport no longer self-defaults (internal/ai/transport_test.go:32 confirms), initgen sources from config.DefaultAICommand/DefaultTimeout with build-failing drift tests (internal/initgen/initgen.go:16-17, initgen_test.go:57-93).
  - Grep of both files for residual phase/task narration found only allowed carve-outs: lines 23 & 168 (Phase 6 provider-value/typed-validation, out-of-scope), 42, 285, 381 (historical Phase 1 notes), 171 (Phase 2 origin label), 237 & 788 (point-of-use "assembly (1-2)" references). None describe an already-shipped behaviour as future/deferred/owned-by-a-later-task.
  - "No code behaviour changes" confirmed: git diff of config.go/verb.go shows only comment-line edits (zero non-comment +/- lines).

TESTS:
- Status: Adequate (no new tests required — comment-only change, per the task's own Tests section).
- Coverage: The as-built claims now asserted in comments are independently pinned by existing tests — internal/ai/transport_test.go (transport no longer self-defaults) and internal/initgen/initgen_test.go:57-93 (scaffold values equal config.DefaultAICommand / DefaultTimeout, build-failing on drift). These guarantee the rewritten comments stay true to as-built.
- Notes: Correct call to add no tests; a documentation-tense change has no behavioural surface to assert beyond the unchanged suite.

CODE QUALITY:
- Project conventions: Followed. CLAUDE.md Comments contract ("TRUE TO AS-BUILT", "Never leave scope/phase claims that contradict the shipped code") is now upheld in both files. golang-documentation conventions respected (WHY-comments, contract-first).
- SOLID principles: N/A (comment-only).
- Complexity: Low (no logic touched).
- Modern idioms: N/A.
- Readability: Improved — comments now read as present fact rather than mixing tense with planning vocabulary; intent is clearer to a future reader who has no plan context.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/config/config.go:171 — "Context and Prompt are the Phase 2 notes-engine prompt-control knobs" carries a phase-origin label ("Phase 2") that is not forward-looking but, like the enumerated sites, references a plan-phase a future reader has no map for. Consider whether to drop the "Phase 2" qualifier (the sentence reads fine as "Context and Prompt are the notes-engine prompt-control knobs"). Not a contract violation (it does not claim shipped behaviour is future); left out of the task's fix list deliberately, so this is a judgment call on consistency, not a defect.
