TASK: T3-1 — Consolidate Duplicated Abort-Chain Test Helpers and Concise-Phrase Assertions in the Engine Internal-Test Package (tick-a2d3fd)

ACCEPTANCE CRITERIA:
1. Exactly one abort-chain wrapping helper exists in the engine internal-test package; the literals "v1.0.0" and OnNotesFailure: "abort" and the abort-chain doc comment appear in exactly one place.
2. The three former helpers (resolveAbortAround, wrapAsAbort, resolveAbortError) are gone, with all former callers routed through the shared helper.
3. The concise-phrase three-part assertion exists in exactly one helper, called from both TestFailureMessage_CollapsesForwardAbortChain and TestFailureMessage_CollapsesRegenerateShortChain.
4. No production (non-test) source files change.
5. All project gates pass.

STATUS: Complete

SPEC CONTEXT: This is an analysis-cycle (Phase 3) de-duplication task — pure test-code consolidation, no production change. The engine internal-test package had authored the same abort-chain wrapping helper three times (resolveAbortAround, wrapAsAbort, resolveAbortError — byte-identical bodies calling notes.ResolveFailure in abort mode with the magic literals "v1.0.0" and OnNotesFailure: "abort") and the AC#2 concise-phrase guard (no ':', no "failed", no leading "notes") twice. The goal: collapse to one wrapping helper and one assertion helper so a ResolveFailure signature change or a rule tightening edits one site, while all existing engine tests keep asserting identical behaviour.

IMPLEMENTATION:
- Status: Implemented (clean consolidation, no leftover duplicate)
- Location:
  - internal/engine/engine_failure_testhelpers_test.go (new) — wrapNotesAbort (line 24) and assertConcisePhrase (line 36), with the single consolidated abort-chain doc comment (lines 19-23) and the magic literals at line 26.
  - internal/engine/failuremessage_internal_test.go — resolveAbortError deleted; callers at lines 41, 75 route through wrapNotesAbort; the two inline triplets (former lines 63-71 and 90-98) replaced by assertConcisePhrase at lines 47 and 66; unused strings/config/runner imports dropped.
  - internal/engine/notesfailureoutput_internal_test.go — resolveAbortAround deleted; caller at line 32 routes through wrapNotesAbort; config/runner imports dropped.
  - internal/engine/notesfailurewiring_internal_test.go — wrapAsAbort deleted; callers at lines 83, 105, 142 route through wrapNotesAbort; config/runner imports dropped.
- Notes:
  - (a) CONFIRMED. wrapNotesAbort is defined exactly once (engine_failure_testhelpers_test.go:24). The literals "v1.0.0" and OnNotesFailure: "abort" appear in the failure-surface helpers in exactly one place (line 26, plus a self-referential mention in the file's header comment line 6). The other "v1.0.0" hits across the package (regenerate_*_test.go, release_*_test.go) are unrelated version-resolution fixtures, not abort-chain construction. The abort-chain doc-comment phrase "never invoked in abort mode" appears exactly once (line 23).
  - (b) CONFIRMED. grep for resolveAbortAround|wrapAsAbort|resolveAbortError across internal/engine returns nothing — all three definitions and their duplicated doc comments are gone; every former caller (6 sites across 3 files) now calls wrapNotesAbort.
  - (c) CONFIRMED. The three-part concise-phrase assertion (strings.Contains ":", strings.Contains "failed", strings.HasPrefix "notes") exists only in assertConcisePhrase (engine_failure_testhelpers_test.go:38-46). It is called from TestFailureMessage_CollapsesForwardAbortChain (line 47, inside the table loop) and TestFailureMessage_CollapsesRegenerateShortChain (line 66, replacing the standalone triplet). The exact-phrase equality check (got != want) is retained alongside in both tests, so the helper adds the structural guard without weakening the value assertion.
  - (d) CONFIRMED. The commit (e5486b0) touches only _test.go files plus workflow bookkeeping (.tick/tasks.jsonl, manifest.json). diff-filter for non-test *.go production files returns none.

TESTS:
- Status: Adequate (the consolidation IS test code; behaviour is preserved, not extended)
- Coverage: Behaviour is identical to pre-T3-1. wrapNotesAbort preserves the t.Helper() + fatal-on-nil + return-err contract of all three former helpers (the bodies were byte-identical, so a single merged body is exact). assertConcisePhrase holds the same three checks with the same t.Errorf messages as the former inline triplets. All five failureMessage tests, the notesFailureOutput extraction test, and the wiring tests retain their per-case exact-phrase / population assertions; only the wrapping and the structural-guard duplication were factored out. t.Parallel() is preserved in every caller and subtest.
- Notes: No over-testing or under-testing introduced — this is a faithful extraction. The merged wrapNotesAbort doc comment correctly generalises across the three former contexts (failureMessage chain collapse, notesFailureOutput carrier traversal, wiring), naming the *ai.GenerationError carrier and the longest errors.As chain. Note (correctness-neutral): the assertConcisePhrase t.Errorf messages are hard-coded as "failureMessage = %q, ..." even though the helper is now also indirectly the assertion vehicle reached from contexts where the value under test is conceptually the same display Message — acceptable, as both call sites do assert a failureMessage result.

CODE QUALITY:
- Project conventions: Followed. External-vs-internal: the package is the white-box internal test (package engine) as required for white-box failureMessage/notesFailureOutput/surfaceAndUnwind proofs. Helpers use t.Helper(). The new file name engine_failure_testhelpers_test.go reads naturally as the shared-helper home (one of the two sanctioned placements in the task's Do step). Imports are minimal and all used in each touched file.
- SOLID principles: Good. Single shared helper per concern; no premature abstraction.
- Complexity: Low. Both helpers are flat.
- Modern idioms: Yes. t.Context(), runner.NewFakeRunner(), t.Helper(), t.Parallel() all idiomatic.
- Readability: Good. The consolidated doc comments state the WHY (magic-literal/rule single-site ownership, abort-chain shape, errors.As traversal) consistent with the codebase's heavy WHY-comment style, and remain true to as-built.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.

GATES NOTE: Gate execution (go build / gofmt / go vet / go test -race / golangci-lint) is outside this verifier's remit (no test/command execution permitted; Bash used only for git inspection and the report rename). Adequacy is judged by reading: the import sets in all four files are consistent with what is used (no dangling unused imports, which would fail go build/vet), the merged helper body is a byte-faithful union of three identical bodies, and no production code changed. AC#5 (gates green) is asserted by the commit's standing as the completed Phase-3 head; this review does not independently re-run it.
