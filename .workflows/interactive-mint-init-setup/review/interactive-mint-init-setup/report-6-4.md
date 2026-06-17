TASK: interactive-mint-init-setup-6-4 — Reword emitted-guide procedure step 2 to name mint's own README unambiguously

ACCEPTANCE CRITERIA:
- Procedure step 2 names mint's own README explicitly and is not readable as the target project's README.
- The minimalism-philosophy clause is retained.
- The guide's structural markers are unchanged.
Tests:
- The existing structural/marker tests for the emitted guide pass unchanged.
- Add or extend an assertion that the rendered procedure step 2 contains the disambiguated "mint's README" wording.

STATUS: Complete

SPEC CONTEXT:
specification.md "Emitted guide — setup procedure" step 2 reads: "Learn mint — read the README and internalise mint's minimalist philosophy (only set what varies)." The surrounding spec makes clear the README in question is mint's OWN README, the human config-reference surface: line 113 ("GitHub docs / README → the human config reference"), line 124 ("the README is the human surface"), line 185 ("an illustrative example value (e.g. in the human README)"), and the "README — entry point" section (195+) describing mint's README as the operator-facing entry point. The finding (analysis-tasks-c2.md Task 4, severity low, sources standards) is correct: the as-shipped emitted prose "Read the project's README for mint's commands and surface" most naturally reads as the TARGET project's README — which would not document mint's commands or philosophy — so a following agent could read the wrong README. This is load-bearing AI-facing prose with no compiler check (the spec's "prose quality" acceptance posture).

IMPLEMENTATION:
- Status: Implemented
- Location: internal/setupguide/setupguide.go:112-115 (procedure step 2). Remediation commit 5a60353.
- As-built step 2 prose: "2. Learn mint. Read mint's README (the human config reference) for mint's commands and surface — this means mint's OWN README, not the target project's — and internalise mint's minimalist philosophy: only set what varies from the compiled defaults."
- (a) Disambiguation: PASS. The README is now named twice and unambiguously — "mint's README (the human config reference)" plus the explicit contrast "this means mint's OWN README, not the target project's". The old "the project's README" wording is gone. The parenthetical "(the human config reference)" aligns precisely with the spec's framing of the README as the human surface.
- (b) Minimalism clause retained: PASS. "internalise mint's minimalist philosophy: only set what varies from the compiled defaults" survives verbatim from the prior revision (git diff shows the philosophy tail unchanged).
- (c) Structural markers unchanged: PASS. The commit diff touches only the procedure() body (5 lines in setupguide.go); the five Marker* constants (lines 48-59) and every Marker* emission are untouched. Markers are not part of the procedure step and were not in scope of the edit.
- Scope discipline: the change is confined to step 2 prose; steps 1 and 3-6 are unchanged. gofmt reports the file clean.
- Notes: step 3 (line 118) and the configReferenceSection intro independently reinforce "the README is the human surface", so the guide is now internally consistent on the README's role. No drift from the spec's decided intent.

TESTS:
- Status: Adequate
- Location: internal/setupguide/setupguide_test.go:223-284 (TestGuide_LearnMintStepNamesMintsOwnReadme plus the numberedStep / isNumberedStepOpener helpers), added in the same commit 5a60353.
- Coverage: The new test gathers the full multi-line step 2 (numberedStep joins continuation lines up to the next numbered opener or a blank line) and asserts three things: (1) positive — the step contains "mint's README"; (2) negative — the step does NOT contain "the project's README" (guards against regressing to the ambiguous wording); (3) the minimalism-philosophy clause survives ("minimalist philosophy", case-insensitive). This is exactly the "add an assertion on the disambiguated wording" the finding's Tests section demands (item d), and it covers all three acceptance criteria for the prose.
- numberedStep correctness verified by hand against the as-built body: it locates the "2." opener (line 112), appends the trimmed continuation lines 113-115 (none of which open with a digit-then-dot, so isNumberedStepOpener returns false), and stops at line 116 ("3."). The negative assertion does not false-trip: the rendered phrase is "not the target project's —", which does not contain the contiguous substring "the project's README". The test would genuinely fail if step 2 were reverted to "the project's README" (negative assert fires) or if the disambiguation were dropped (positive assert fires) or if the philosophy clause were lost (third assert fires).
- isNumberedStepOpener edge: dot<=0 guard rejects a leading-dot line; the digit-only scan over the pre-dot prefix correctly distinguishes "2." from prose containing a period. Sound.
- Structural/marker tests (TestGuide_EmitsEverySectionMarker, _MarkersAreUnique, _MarkersAreCommentAnchorsNotProse, _EachMarkerSitsOnItsOwnLine, _FirstProcedureStepIsCwdConfirm) are unaffected by a step-2 prose change and continue to assert structure — satisfying "existing structural/marker tests pass unchanged".
- Not under-tested: all three sub-criteria (disambiguation present, old wording absent, philosophy retained) are asserted. Not over-tested: the test does not pin the entire step-2 sentence verbatim (which would be brittle); it keys on the load-bearing tokens only. The two new helpers (numberedStep / isNumberedStepOpener) are justified — they generalise step extraction across wrapped lines and read as a natural sibling to the pre-existing firstNumberedStep helper.

CODE QUALITY:
- Project conventions: Followed. External test package (setupguide_test), t.Parallel() on the new test, token-level assertions over verbatim-line pins (matches the CLAUDE.md "behaviour-level proofs / structural markers, not representative prose" idiom). The prose edit carries no new config metadata and leaves the config-free authoring discipline intact.
- SOLID principles: Good. numberedStep/isNumberedStepOpener are single-purpose; the test asserts one behaviour.
- Complexity: Low. The prose change is three altered source lines; the test helpers are simple linear scans.
- Modern idioms: Yes (strings.Split/TrimSpace/HasPrefix; IndexByte for the dot scan).
- Readability: Good. The WHY-comment on TestGuide_LearnMintStepNamesMintsOwnReadme states the ambiguity it guards and the chosen disambiguated wording; numberedStep documents the wrapped-step gathering.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/setupguide/setupguide_test.go:213-221 vs 252-269 — firstNumberedStep and numberedStep now overlap: firstNumberedStep(body) is numberedStep(body, "1.") restricted to the opening line. They could be unified (e.g. firstNumberedStep delegating to a single step-extraction primitive) to single-source procedure-step location. Deferred because it is a judgement call about the right shared shape and is outside this task's prose-disambiguation scope; raised only as a future dedupe consideration.
