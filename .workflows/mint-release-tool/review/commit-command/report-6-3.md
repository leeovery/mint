TASK: 6-3 — Do not emit the "opening editor" note on the unattended oversized path that fails loud (tick-3efc2a, Phase 6: Analysis Cycle 1)

ACCEPTANCE CRITERIA:
1. On the -y / non-TTY oversized path, no "diff too large to summarise — opening editor" Warn is recorded; the run fails loud with the exact spec fail-loud message and mutates nothing.
2. On the attended (TTY, non--y) oversized path, the note is still emitted and the editor still opens.
3. The AI-failure trigger still emits no note.

STATUS: Complete

SPEC CONTEXT:
The $EDITOR fallback (specification.md, "Commit Message Format & Prompt" → case 3, and "$EDITOR Fallback — Path Semantics") describes the oversized note "diff too large to summarise — opening editor" as the clear signal when routing TO the editor, after which the -y/non-TTY forbidden-combo check applies. The spec mandates that an unattended run hitting the oversized fallback "has no message source and fails loud" with the single message "no AI message and no interactive editor available". The drift the task fixes: the note was firing on a run that would never reach the editor — promising an editor that the very next fail-loud guard refuses to open. Substance was already spec-conformant (fails loud, mutates nothing, exact message); only the contradictory emitted-then-denied note was a UX drift.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/commit/run.go:230-242 (new editorUnavailable predicate), run.go:318-323 (oversized branch gates the note on !deps.editorUnavailable()), run.go:402-414 (runEditorFallback guard reuses the same predicate).
- Notes: The task offered two options (wrap the Warn in the attended condition, or hoist the unattended guard). The implementation took a cleaner third path: extracted a single Deps.editorUnavailable() predicate (d.Yes || !d.StdinInteractive) consumed by BOTH the note gate (line 319, negated) and the fallback's no-message-source guard (line 412). This is the strongest possible form of the fix — the two consumers cannot drift because "will the editor open?" has exactly one definition. The oversized note now fires only when !editorUnavailable() (attended), so an unattended oversized run reaches runEditorFallback and fails loud at line 413 with no preceding note. The AI-failure branch (run.go:330-332) was already noteless and is unchanged. The predicate correctly covers only the two pre-resolution conditions (-y, non-TTY); the third no-message-source case (no launchable editor in git's chain) is necessarily detected post-resolution at run.go:422-423, which only happens on attended runs — the doc comment at run.go:230-242 states this accurately.

TESTS:
- Status: Adequate
- Coverage:
  - AC1 (unattended → no note + fail loud + no mutation): internal/commit/run_oversized_failloud_test.go:17 TestRun_Oversized_Unattended_NoNote_FailsLoud — table over both {UnderYes} and {NonTTYStdin}; asserts assertFailLoudNoMutation (exact spec message once, no launch, no add, no commit) AND scans warnEvents for the oversized note, failing if present. This is the precise gap the task identified (the older fail-loud tests assert message+no-mutation but not note-absence).
  - AC2 (attended → note emitted + editor opens): run_oversized_test.go:154 TestRun_Oversized_EmitsNote (exactly one Warn, byte-for-byte note), :252 TestRun_Oversized_OverLimitTriggersFallback (one launch + note), :125 TestRun_Oversized_SkipsL2_RoutesToEditor (launch recorded). All run on the default attended deps (StdinInteractive true, no -y).
  - AC3 (AI-failure noteless): run_oversized_test.go:426 TestRun_Oversized_DistinctFromGenerationFailure/AIFailurePathCarriesNoOversizedNote — within-ceiling diff whose transport fails routes to the editor and asserts no oversized Warn fires.
  - Regression also covered by the existing fail-loud trigger tables in run_failloud_test.go which include the Oversized trigger under both -y and non-TTY.
- Notes: Not over-tested. The new test overlaps the existing fail-loud trigger tables only on the message/no-mutation assertion, but adds the unique note-absence check (the actual subject of this task) which those tables do not have — so it is genuinely new coverage, not a duplicate. The note string is asserted via the verbatim oversizedNote const (em dash U+2014), matching the run.go const. A test would fail if the note leaked onto the unattended path (AC1) or disappeared from the attended path (AC2). Edge cases from the spec (clean exact message, no editor launch, no git add/commit) are all asserted.

CODE QUALITY:
- Project conventions: Followed. Table-driven subtests with t.Parallel(), exact byte-for-byte string asserts via shared consts, FakeRunner-driven git thread, RecordingPresenter — all consistent with golang-testing and the surrounding commit package.
- SOLID principles: Good. The single-predicate extraction is a textbook DRY/single-source-of-truth move that removes a real drift hazard between two consumers.
- Complexity: Low. One small boolean method plus one negated guard at the note site; control flow is unchanged otherwise.
- Modern idioms: Yes. errors.Is for sentinel routing, predicate as a value-receiver method on Deps.
- Readability: Good. The doc comment on editorUnavailable (run.go:230-242) explicitly names both consumers and why they share the predicate; the oversized branch comment (run.go:309-317) explains the emitted-then-contradicted hazard being avoided.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
