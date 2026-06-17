TASK: 2-2 — Add the carrier-output extraction helper composing stdout-then-stderr (notesFailureOutput in internal/engine/release.go, mirroring hookFailureOutput).

ACCEPTANCE CRITERIA (from plan):
- [x] notesFailureOutput returns "" for a cause that does not wrap *ai.GenerationError (timeout, command-missing, diff-too-large, or any plain error).
- [x] Returns the carrier's captured output when the carrier is wrapped inside abortError's chain AND inside the shorter "generating notes: %w" chain (errors.As traversal).
- [x] stdout-only → stdout verbatim (trailing whitespace trimmed); stderr-only → stderr verbatim (trailing whitespace trimmed).
- [x] Both non-empty → stdout, single newline, stderr; internal content verbatim; only composed result's trailing whitespace trimmed.
- [x] Whitespace-only stdout AND whitespace-only stderr → "".
- [x] One whitespace-only + one real → only the real stream, no joining newline.
- [ ] Gates pass: not executed (verifier does not run the suite; assessed by reading only).

STATUS: Complete

SPEC CONTEXT:
Fix 1 step 3 (specification.md lines 46-53) settles the composition rule: trim each stream for the EMPTINESS check only (whitespace-only counts empty); include non-empty streams stdout-first then stderr joined by a single newline; keep included content verbatim; trim the composed result's trailing whitespace; both-empty → "". "Per-cause Output behaviour" (lines 55-58) confirms only the generation-failed carrier carries output — timeout / command-missing / diff-too-large have no claude output. Acceptance Criteria #1 (line 145) restates the rule. The helper is the read-side counterpart of the Phase 1 carrier (*ai.GenerationError with Stdout/Stderr/ExitCode, internal/ai/transport.go:56-85) and is consumed by Task 2-3.

IMPLEMENTATION:
- Status: Implemented, matches the settled rule exactly.
- Location: internal/engine/release.go:1606-1639 (notesFailureOutput), placed directly below hookFailureOutput (1594-1604) so the parallel pattern is visible.
- Verification of the settled rule, clause by clause:
  (a) non-carrier → "": errors.As(cause, &genErr) fails → early return "" (1623-1626). Covered for all four non-carrier shapes by TestNotesFailureOutput_EmptyForNonCarrierCause.
  (b) carrier matched via errors.As through both chains: errors.As traverses %w. Proven against abortError's forward chain (wrapNotesAbort, which routes through notes.ResolveFailure abort mode → abortError "notes generation failed (%s): %w") and the shorter "generating notes: %w" chain. Carrier type/field names confirmed against internal/ai/transport.go (GenerationError{Stdout,Stderr,ExitCode}, Unwrap → ErrGenerationFailed).
  (c) stdout-only / stderr-only verbatim, trailing trimmed: only the non-empty stream is appended, then strings.TrimRightFunc(out, unicode.IsSpace). ✓
  (d) both non-empty → stdout, single "\n", stderr, interior verbatim, only trailing trimmed: strings.Join(streams, "\n") joins exactly the two appended ORIGINAL values, then a single trailing trim. ✓
  (e) emptiness check uses trimmed view: gate is strings.TrimSpace(genErr.Stdout) != "" (and same for Stderr); the appended value is the untrimmed original genErr.Stdout/Stderr. ✓
  (f) both whitespace → "": both gates fail, streams empty, Join → "", trim → "". ✓
  (g) one whitespace + one real → only real, no joining newline: streams has one element, Join adds no separator. ✓
  (h) interior preserved: only TrimRightFunc on the composed result; interior content untouched. ✓
- WHY-comment: present (1606-1621) and explicitly states it reads STDOUT (not stderr like the hook helper) because claude writes its payload to stdout, and that errors.As is used precisely so the carrier matches inside abortError's forward chain OR regenerate's shorter chain. Matches the plan's "Do" instruction.
- Imports confirmed present: errors, strings, unicode, mint/internal/ai (release.go:32/35/37/39).
- No change to hookFailureOutput, presenter, or padStage (out of scope, correctly untouched).
- Note: the commit c61b440 also wired Output: notesFailureOutput(cause) into surfaceAndUnwind (release.go:1057) and surface (release.go:1655), and added notesfailurewiring_internal_test.go. That is Task 2-3's surface; not in scope for 2-2 verification, but observed — no conflict, the helper itself is the 2-2 deliverable and is correct.

TESTS:
- Status: Adequate. Well-balanced; one minor coverage gap (non-blocking, see notes).
- Location: internal/engine/notesfailureoutput_internal_test.go (white-box, package engine).
- Coverage map vs the plan's named test list:
  - "it extracts stdout when the carrier is wrapped in abortError" → TestNotesFailureOutput_ExtractsStdoutThroughAbortChain (stdout "Prompt is too long\n" → "Prompt is too long"). ✓
  - "it extracts through the shorter regenerate chain" → TestNotesFailureOutput_ExtractsThroughRegenerateShortChain. ✓
  - "it composes stdout then stderr joined by a single newline when both present" → TestNotesFailureOutput_ComposesStdoutThenStderr ("out line\nerr line"). ✓
  - "it includes only stdout when stderr is whitespace-only" → TestNotesFailureOutput_IncludesOnlyStdoutWhenStderrWhitespace. ✓
  - "it includes only stderr when stdout is whitespace-only" → TestNotesFailureOutput_IncludesOnlyStderrWhenStdoutWhitespace. ✓
  - "it returns empty when both streams are whitespace-only" → TestNotesFailureOutput_EmptyWhenBothStreamsWhitespace. ✓
  - "it preserves internal content verbatim while trimming trailing whitespace" → TestNotesFailureOutput_PreservesInteriorTrimsTrailing ("line1\n\nline2\n\n" → "line1\n\nline2"). ✓
  - "it returns empty for a non-carrier cause" → TestNotesFailureOutput_EmptyForNonCarrierCause (table: ai.ErrTimeout, ai.ErrCommandMissing, notes.ErrDiffTooLarge, plain error). ✓
  All eight named tests exist and assert the exact expected values. Tests would fail if the helper broke (e.g. wrong field, missing trim, wrong join separator, no errors.As traversal).
- Not over-tested: each test pins one distinct behaviour; no redundant happy-path variations; setup is minimal (direct carrier literals; abort/regenerate chains only where chain-shape traversal is the point).
- Not under-tested (minor gaps, all non-blocking):
  - The both-streams-present composition (TestNotesFailureOutput_ComposesStdoutThenStderr) uses already-trimmed literals ("out line"/"err line"), so it does not exercise the verbatim-interior + single-trailing-trim path WHEN BOTH streams are present (e.g. stdout with an interior blank line joined to a stderr with trailing whitespace). The interior/trailing behaviour is proven for stdout-only (PreservesInteriorTrimsTrailing) and the join is proven separately, so the composed clause-(d) behaviour is covered by inference but not by a single test asserting both at once. Low value gap.

CODE QUALITY:
- Project conventions: Followed. White-box internal test in package engine for an unexported helper; t.Parallel() throughout (top-level and subtests); table-driven for the non-carrier set where the shape fits; exact-value assertions; heavy WHY-comment true to as-built (states the contract, the stdout-vs-stderr divergence, and why errors.As). Consistent with CLAUDE.md test idioms and the hookFailureOutput precedent.
- SOLID: Good. Single responsibility (extract + compose); no leaked state.
- Complexity: Low. Two guarded appends, one Join, one trim; no branching beyond the two emptiness gates.
- Modern idioms: Yes. errors.As for chain traversal, strings.TrimSpace for the emptiness decision, strings.Join for the single-newline composition, strings.TrimRightFunc(unicode.IsSpace) for trailing trim (the plan offered this or TrimRight over a charset; TrimRightFunc is the broader/cleaner choice).
- Readability: Good. The "decide inclusion on the trimmed view, keep the original verbatim values" comment (1628) makes the verbatim-vs-trimmed distinction explicit at the line it matters.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/engine/notesfailureoutput_internal_test.go:59 — TestNotesFailureOutput_ComposesStdoutThenStderr uses pre-trimmed literals, so the both-streams-present case does not assert interior-verbatim + single-trailing-trim together. Add a case with stdout "out\n\nline\n" and stderr "err\n\n" asserting "out\n\nline\nerr" to cover clause (d) directly rather than by inference.
