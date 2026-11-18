TASK: mint-release-tool-2-12 — Interactive review gate semantics (y/n/e)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. engine/release.go reviewGate loop (y/n/e/r mapping), gateForKind variant select, regenerateBody, editBody; wired BEFORE Record/dry-run boundary, gate-no routed to surgical Unwind. engine.go FirstReleaseReviewGate y/n/e literal + ReviewDecision maps presenter errors to *AbortError. -y skip+echo lives in presenter (plain.go) — cross-spec boundary; Prompt records exactly one KindPrompt. Edited body threaded verbatim to all sinks. No rendering in engine.

TESTS:
- Status: Adequate. engine_test.go: y/n/e gate literal, ReviewDecision scripted choice + single KindPrompt, both prompt-error sentinels → non-zero abort. release_test.go: bare-Enter accept, explicit y, -y always-prompts-once no engine echo, gate-no abort no mutation, edit-verbatim-to-all-sinks then yes, edit-then-no, editor error/nil/return-to-gate, unexpected-choice abort, gate-variant selection. assertNoMutation + rev-parse HEAD count==1.

CODE QUALITY:
- Followed conventions (accept-interfaces seams, presenter-owned rendering, typed *AbortError w/ exit code). SOLID good — reviewGate owns semantics only. Acceptable complexity (switch-in-loop), good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/engine/release_test.go:1228-1241 — assertNoMutation does not check for `git ... commit` invocations, so an abort path that erroneously made a bookkeeping/artifact commit before the gate wouldn't be caught by this helper directly (gate-no tests cover it indirectly via rev-parse HEAD count==1). Add a `case strings.Contains(line, " commit -m")` arm to make the "no commit" half explicit at the helper level.
