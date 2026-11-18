TASK: Task 2-1 — Expose the concise cause phrase and collapse failureMessage for notes/AI failures (notes-failure-output-ugly-and-uninformative-2-1)

ACCEPTANCE CRITERIA:
1. notes.CauseText (exported derivation) returns the exact concise phrase for each of the four sentinels and reports not-known for any other error, matching via errors.Is through both abortError's chain and the shorter "generating notes: %w" chain.
2. failureMessage(cause) returns the concise phrase (no nested %w chain) for all four known causes on BOTH chain shapes; does not begin with the stage label "notes"; does not contain "failed". Incidental "notes" inside the phrase allowed.
3. errors.Is(cause, <sentinel>) still matches after the change — %w chain untouched.
4. resetAndAbort's git-failure cause still renders via the cause.Error() defensive fallback.
5. resolve_test.go's existing abort-message assertions stay green (abortError byte-identical).
6. Gates pass.

STATUS: Complete

SPEC CONTEXT:
Fix 2 collapses the verbose nested %w display chain ("notes generation failed (AI returned empty/invalid notes after retry): generating notes: ai generation failed") to one concise cause phrase. The settled seam is the engine's single display-derivation helper failureMessage(cause), which all StageFailed sites (surface, surfaceAndUnwind, resetAndAbort) funnel through. The phrase is derived from the SENTINEL the cause wraps (via errors.Is, traversing the %w chain), NOT by rendering cause.Error(). The four sentinels (ai.ErrGenerationFailed, ai.ErrTimeout, ai.ErrCommandMissing, notes.ErrDiffTooLarge) are the exhaustive notes-display universe; the default/unmapped branch is a defensive fallback. The %w chain is RETAINED for errors.Is/logs (Sub-decision). Regenerate's fresh path carries a shorter chain than forward release, so the derivation must match the wrapped sentinel, not the chain shape (Note on regenerate's wrap chain). Acceptance Criteria #2 governs the display Message only.

IMPLEMENTATION:
- Status: Implemented (matches plan option (a) exactly)
- Location:
  - internal/notes/resolve.go:104-129 — new exported CauseText(failure error) (string, bool): errors.Is switch over the four sentinels returning the exact concise phrases with true; ("", false) default.
  - internal/notes/resolve.go:131-140 — unexported causeText now delegates to CauseText, falling back to failure.Error() when known==false (preserves abortError's byte-identical output for unmapped causes).
  - internal/notes/resolve.go:100-102 — abortError UNCHANGED ("notes generation failed (%s): %w" with causeText(failure)).
  - internal/engine/release.go:1674-1683 — failureMessage gains a new branch (notes.CauseText) ordered AFTER the *preflight.GateError branch and BEFORE the final cause.Error() fallback.
- Notes: Carrier *ai.GenerationError unwraps to ai.ErrGenerationFailed (internal/ai/transport.go:85), so a carrier wrapped anywhere in the chain resolves via the ErrGenerationFailed case to "AI returned empty/invalid notes after retry" — confirmed. Gate-error ordering is non-conflicting: a *preflight.GateError is never one of the four sentinels, and the gate branch runs first, preserving its behaviour. All callers of causeText/CauseText verified consistent (grep: only resolve.go's abortError uses the unexported form; release.go uses the exported CauseText). The exported phrases are byte-identical to the values pinned in resolve_test.go's TestResolveFailure_VariedCauses_RouteThroughBothModes table (resolve_test.go:231-234), so abortError's message is unchanged.

TESTS:
- Status: Adequate
- Coverage:
  - internal/notes/causetext_test.go:
    - TestCauseText_KnownSentinels_DerivesConcisePhrase — table over all four sentinels; asserts the exact phrase AND known=true, both BARE and WRAPPED behind "generating notes: %w" (proves errors.Is traversal across chain shapes). Covers plan test "it derives the concise phrase for each of the four sentinels via errors.Is".
    - TestCauseText_UnmappedCause_ReportsNotKnown — plain errors.New("boom"); asserts known=false and empty phrase. Covers plan test "it reports an unmapped cause as not-known".
  - internal/engine/failuremessage_internal_test.go (white-box, package engine):
    - TestFailureMessage_CollapsesForwardAbortChain — table over all four sentinels wrapped via wrapNotesAbort (real notes.ResolveFailure abort chain); asserts the concise phrase AND assertConcisePhrase (no ':', no "failed", no leading "notes"). Covers the forward-chain plan test.
    - TestFailureMessage_CollapsesRegenerateShortChain — fmt.Errorf("generating notes: %w", ai.ErrGenerationFailed); asserts the identical concise phrase + assertConcisePhrase. Covers the regenerate shorter-chain plan test.
    - TestFailureMessage_LeavesMatchableChainIntact — asserts errors.Is(cause, ai.ErrGenerationFailed) still holds after failureMessage runs. Covers the chain-intact plan test.
    - TestFailureMessage_FallsBackToCauseErrorForNonAICause — resetAndAbort's shape (wrapped git error, none of the four); asserts failureMessage == cause.Error(). Covers the git-fallback plan test.
  - Shared white-box helpers internal/engine/engine_failure_testhelpers_test.go: wrapNotesAbort (builds the real forward abort chain), assertConcisePhrase (pins AC #2: no ':', no "failed", no leading "notes"). Helpers were consolidated from per-test copies — sensible DRY, single site for the magic literals and the rule.
  - Existing internal/notes/resolve_test.go TestResolveFailure_VariedCauses_RouteThroughBothModes and TestResolveFailure_UnknownCause_AbortFallsBackToFailureMessage use errorContains (substring) on the full abort message; unaffected because causeText still produces identical phrases and abortError's format string is untouched — abort message byte-identical.
- Notes: Coverage maps 1:1 onto the plan's six listed tests. Both chain shapes are exercised at both the notes layer (CauseText) and the engine layer (failureMessage). The "no leading stage label" check is HasPrefix(got, "notes"), which correctly permits the incidental "notes" inside "AI returned empty/invalid notes after retry" (spec explicitly allows this). Not over-tested: the engine tests are white-box but justified — failureMessage is unexported and is the settled display seam; asserting it directly is the right behavioural level. No redundant assertions.

CODE QUALITY:
- Project conventions: Followed. errors.Is sentinel matching with %w chains retained (golang-error-handling). Lowercase, no-trailing-punctuation messages preserved in abortError. External test packages (notes_test) for the public API; package engine white-box only where the seam is unexported, per project precedent. t.Parallel() and table-driven throughout. The heavy WHY-comments are accurate to as-built (CauseText documents the single-source-of-truth role and the errors.Is rationale; failureMessage documents branch ordering and the %w-retained-for-logs invariant).
- SOLID principles: Good. Single source for the concise phrasing (CauseText); causeText delegates, eliminating drift; failureMessage stays the single display-derivation seam. No new types or indirection beyond the minimal exported function (plan option (a), the smaller of the two acceptable shapes).
- Complexity: Low. CauseText is a flat errors.Is switch; causeText is a 4-line delegate; failureMessage gains one branch.
- Modern idioms: Yes. (phrase, known) comma-ok return; errors.Is/errors.As.
- Readability: Good. Intent and contracts are explicit.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
