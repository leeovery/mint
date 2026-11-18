TASK: T4-2 — Make GenerationError.Error()/ExitCode Honest About Its Dual Provenance (tick-37eba5)

ACCEPTANCE CRITERIA:
1. GenerationError.Error() never renders "(exit 0)": the empty/whitespace-body path (ExitCode == 0) produces a distinct, non-contradicting string; the non-zero-exit path is unchanged.
2. The ExitCode field doc describes both legitimate provenances (non-zero exit OR zero on the empty-body path).
3. Error() strings stay lowercase with no trailing punctuation.
4. No change to routing, the carried Stdout/Stderr/ExitCode, Unwrap(), or classification logic; errors.Is(err, ErrGenerationFailed) still matches the carrier from both sites; context.Canceled passthrough unchanged.
5. The display Message path (notes.CauseText / failureMessage → concise phrase) is unaffected; AC#2 concise-phrase tests still pass.
6. All project gates pass.

STATUS: Complete

SPEC CONTEXT:
Spec "Fix 1" upgrades ai.ErrGenerationFailed into the typed carrier *ai.GenerationError that wraps the sentinel and carries claude's captured stdout/stderr. The carrier has two construction sites (transport.go:220 non-zero exit; transport.go:231 empty/whitespace body after a clean zero-exit retry). The Invariants section (#1 errors.Is still matches, #2 context.Canceled passthrough, #3 content-agnostic) are load-bearing. This task is a low-severity diagnostic-honesty follow-up: on the empty-body site ExitCode is 0, so the original Error() rendered the self-contradicting "ai generation failed (exit 0)". The display Message for ErrGenerationFailed is resolved upstream by notes.CauseText via errors.Is (→ "AI returned empty/invalid notes after retry") WITHOUT rendering cause.Error(), so Error() is a diagnostic/log surface only.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/ai/transport.go:63-67 (ExitCode field doc), :70-81 (Error()).
- Notes:
  - Error() now branches: ExitCode == 0 → "ai generation failed (empty body)"; ExitCode != 0 → fmt.Sprintf("ai generation failed (exit %d)", e.ExitCode). AC#1 satisfied — the zero-exit path can never render "(exit 0)".
  - ExitCode field doc (lines 63-66) now states both shapes: "the non-zero process exit status on the non-zero-exit path, OR zero on the empty/whitespace-body path — a clean (zero-exit) attempt whose body alone (not the exit) classified it as bad content." AC#2 satisfied.
  - Both new strings are lowercase, no trailing punctuation — matches the project error idiom (CLAUDE.md "messages lowercase, no trailing punctuation"; .claude/skills/golang-error-handling SKILL.md rule 3). AC#3 satisfied.
  - A WHY-comment (lines 71-76) was added to Error() explaining the dual provenance and that Error() is a diagnostic surface only — consistent with the codebase's heavy WHY-comment convention.
  - Diff confirms NO change to the two construction sites (transport.go:220, :231 untouched), Unwrap() (:85, returns ErrGenerationFailed unchanged), classifyFatal (:272), isValid (:300), parseCommand, attempt, Generate routing, or carried Stdout/Stderr/ExitCode values. context.Canceled passthrough in classifyFatal is untouched. AC#4 satisfied.
  - Grep across internal/ and cmd/ confirms NO other code consumes the GenerationError.Error() string — only ErrGenerationFailed.Error() (the sentinel "ai generation failed", unchanged) and the carrier are referenced. The string change is isolated to the diagnostic/log surface.
  - notes.CauseText (internal/notes/resolve.go:116-129) maps ErrGenerationFailed via errors.Is to "AI returned empty/invalid notes after retry" and explicitly derives the phrase "WITHOUT rendering the wrapped cause.Error()". engine failureMessage delegates to it. So the Error() change cannot affect the display Message. AC#5 satisfied — the AC#2 concise-phrase tests (internal/notes/causetext_test.go:27, resolve_test.go:233, engine/failuremessage_internal_test.go:34,62, engine/release_priortag_test.go:747, engine/notesfailurewiring_internal_test.go:75) assert the sentinel-keyed phrase and are unaffected.

TESTS:
- Status: Adequate
- Location: internal/ai/transport_test.go:732-797 (two new tests).
- Coverage:
  - TestGenerationError_Error_DistinguishesDualProvenance (table-driven, three cases): empty-body (ExitCode 0) → "ai generation failed (empty body)"; ExitCode 1 → "ai generation failed (exit 1)"; ExitCode 137 → "ai generation failed (exit 137)". Each case also asserts the empty-body variant never contains "exit 0" (guards the exact self-contradiction this task removes). Covers AC#1 and AC#3 (verbatim string match pins lowercase + no trailing punctuation).
  - TestGenerationError_ZeroExitCarrierStillMatchesErrGenerationFailed: regression — errors.Is(&GenerationError{ExitCode: 0}, ErrGenerationFailed) holds, proving the zero-exit carrier still routes via Unwrap. Covers AC#4 routing-preservation.
  - The ExitCode-1 carry behaviour ("(exit 1)") is asserted both here and indirectly by the pre-existing carrier tests (CarriesCapturedOutput... at :645-666 assert genErr.ExitCode == 1). The pre-existing empty/whitespace carrier tests (:545-615) already prove the empty-body construction site populates ExitCode 0 and routes via errors.Is, so the new tests build on a proven path.
- Notes:
  - Not under-tested: both provenances, the negative ("never exit 0"), and the routing regression are covered. The 137 case is a small, justified extra proving the %d format is faithful for arbitrary non-zero codes, not just 1.
  - Not over-tested: no redundant assertions, no unnecessary mocking (the Error() tests construct the carrier literally — correct, since Error() is a pure method with no transport dependency). The strings.Contains "exit 0" guard is a distinct assertion (negative property) from the exact-match, not a duplicate.
  - Tests assert behaviour (the rendered diagnostic string and errors.Is routing), not implementation details. They would fail if the feature regressed (e.g. reverting to the single fmt.Sprintf would make the empty-body case render "(exit 0)" and fail both the exact-match and the Contains guard).

CODE QUALITY:
- Project conventions: Followed. Lowercase/no-punctuation error idiom honoured; WHY-comments added and true to as-built; external test package (ai_test) with t.Parallel() throughout and a table-driven shape; carrier/Unwrap contract preserved (CLAUDE.md AI-seam invariant #5; spec Invariant #1).
- SOLID principles: Good. Single, focused change to one method + one field doc; no new responsibilities introduced.
- Complexity: Low. One added if-branch; cyclomatic cost trivial.
- Modern idioms: Yes. Branch-then-Sprintf is idiomatic Go.
- Readability: Good. Intent is explicit in both the comment and the test rationale.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None. (The gates — go build / gofmt / go vet / go test -race / golangci-lint — were not executed per the no-execution rule; this verification is by reading. The change is string-only on a pure method with an added if-branch and an unused-import-free test addition (strings is now used), so no gate risk is evident from the source.)
