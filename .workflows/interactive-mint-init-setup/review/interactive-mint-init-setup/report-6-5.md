TASK: interactive-mint-init-setup-6-5 — Make MetadataLevel.String() distinguish an invalid level from the shared scope

ACCEPTANCE CRITERIA (from analysis-tasks-c2.md Task 5):
- Each of the four declared MetadataLevel constants produces its existing String() output (LevelShared still ""); behaviour for valid levels is unchanged.
- An out-of-range MetadataLevel value no longer produces the same output as LevelShared — it panics or yields an unmistakable sentinel.
- The render path cannot turn an invalid level into a valid shared-level cell.
- Tests: table test of each declared level's String() (incl. LevelShared == ""); a test exercising an out-of-range level proving fail-loud behaviour, distinguishable from LevelShared.

STATUS: Complete

SPEC CONTEXT:
The finding (analysis-tasks-c2.md:107-130) is an architecture note, not a spec line. The config-metadata SoT (metadata.go header) treats MetadataLevel as a "closed, typed identity that the render target and the drift test agree on." The latent bug: String() returned "" for BOTH LevelShared (a legitimate scope) and the default branch (any out-of-range int), and setupguide.LevelCell maps "" — and only "" — to the "top-level" placeholder, so a corrupted/uninitialised level would silently masquerade as the genuine shared/top-level cell in the agent-facing config table. Latent today (every SoT row uses a declared constant), it would become a real masking bug if a fifth level were added without a String() case. The spec's broader posture (CLAUDE.md "Fail loud, never hang") and the error-handling skill both bear on the panic-vs-sentinel choice.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/config/metadata.go:78-91 (String); default branch at :88-89 returns fmt.Sprintf("MetadataLevel(%d)", int(l)). Rationale comment at :61-77. Render seam unchanged at internal/setupguide/setupguide.go:355-360 (LevelCell).
- Notes:
  (a) SATISFIED. LevelShared still returns "" (explicit case at :86-87); the four declared cases are unchanged. The default branch (:88-89) now returns a non-empty, self-describing sentinel "MetadataLevel(N)" — an out-of-range level can no longer silently map onto the shared/top-level cell.
  (b) SATISFIED. LevelCell (setupguide.go:355-360) takes the "top-level" branch ONLY when level.String() == "". A sentinel like "MetadataLevel(4)" is non-empty, so it returns the sentinel verbatim; the render seam can no longer collapse an invalid level onto the shared cell. LevelCell itself needed no change — single-sourcing the level identity in config.String() is what made the seam correct.
  (c) SATISFIED and idiomatic. The implementer chose the sentinel over panic. This is the correct reading of the Go skills: golang-error-handling best-practice 9 ("NEVER use panic for expected error conditions — reserve for truly unrecoverable states") and references/error-handling.md ("Panic MUST only be used for truly unrecoverable states"). A Stringer is expected to be safe to call, and a production panic in the render path would crash `mint setup` outright — strictly worse than an unmistakable cell. The sentinel form "MetadataLevel(N)" is exactly what golang.org/x/tools/cmd/stringer emits (confirmed against golang-naming SKILL.md: Stringer is the canonical -er interface), so the choice is idiomatic and self-documenting. The WHY-comment at :70-77 records this reasoning truthfully as-built, per CLAUDE.md's comment contract.
  - Note on the finding's framing: the finding suggested panic "matching the 'fail loud on an impossible enum value' posture the bijection walk already takes when it Fatalf's." That posture is TEST-side (t.Fatalf in metadata_drift_test.go:95,111,133) — a test failing loud is not the same as a production panic in a render-path Stringer. The implementer correctly did not transplant a test-time fail-loud into a production Stringer; the sentinel preserves the "loud and unmistakable" intent without the crash risk. The chosen path honours the acceptance criterion's explicit "panics OR yields an unmistakable sentinel" disjunction.

TESTS:
- Status: Adequate
- Coverage:
  - TestMetadataLevel_String (metadata_test.go:114-137): table test pinning each declared level's String() output, including LevelShared == "". Covers the "behaviour for valid levels is unchanged" criterion.
  - TestMetadataLevel_String_OutOfRangeIsDistinctFromShared (metadata_test.go:147-165): exercises config.LevelCommit + 1 (== MetadataLevel(4), since LevelShared=0..LevelCommit=3). Three assertions: (1) got != LevelShared.String() — the load-bearing distinctness claim; (2) got != "" — the masking-prevention claim; (3) got == "MetadataLevel(4)" — pins the conventional Stringer sentinel form so a regression swapping it for an empty/collidable token is caught.
- Notes:
  - (d) latent-until-5th-level concern is GUARDED at the behavioural level: the out-of-range test proves that ANY undeclared level (the case a future 5th-level-without-a-String-case would hit) renders distinctly from shared and is non-empty — so the masking bug the finding describes can no longer occur silently. The test asserts the contract ("distinct from shared, non-empty"), not just the literal value, so it remains meaningful as the enum grows.
  - Not over-tested: the three assertions are non-redundant — distinctness, non-emptiness, and exact-form each catch a different regression class (assertion 1 catches a collapse-to-shared, 2 catches a collapse-to-empty, 3 catches a swap to some other non-empty-but-non-conventional token). No bloat.
  - Not under-tested: both acceptance-criteria test bullets are covered. The render-seam criterion (b) is exercised indirectly — LevelCell's "" branch is the only path to "top-level", and the unit test proves an invalid level never yields "". An explicit LevelCell(out-of-range) assertion would be belt-and-braces but is not required; the seam's correctness is a direct consequence of String() never returning "" for an invalid level (single-sourced). See non-blocking note.
  - The test would fail if the feature broke: reverting the default branch to `return ""` fails all three assertions.

CODE QUALITY:
- Project conventions: Followed. Heavy WHY-comment (metadata.go:61-77) states the contract and the panic-vs-sentinel reasoning, true to as-built (CLAUDE.md comment contract). Sentinel sourced via fmt.Sprintf with int(l), matching the stringer convention named in the comment. Test idioms honoured: external package (config_test), t.Parallel() throughout, table-driven valid-level test, exact-output assertions.
- SOLID principles: Good. String() retains single responsibility (render the canonical TOML form); the closed-enum claim is now enforced at the one place that owns level identity, and LevelCell consumes it without re-deriving.
- Complexity: Low. A four-case switch plus a default; no added branching elsewhere.
- Modern idioms: Yes. fmt.Sprintf sentinel matching golang.org/x/tools/cmd/stringer output; idiomatic per golang-error-handling (no panic in a safe-to-call Stringer) and golang-naming (Stringer).
- Readability: Good. Intent is self-evident from the switch plus the rationale comment.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/setupguide/setupguide_test.go — Consider an explicit LevelCell(config.LevelCommit + 1) assertion proving the render seam returns the sentinel (not "top-level") for an out-of-range level. Currently the seam's correctness for invalid levels is proven only transitively (via String() never returning "" for an invalid level). A direct seam-level test would pin acceptance criterion (b) at the render boundary itself and survive a hypothetical future refactor of LevelCell's empty-check. Decision: whether the transitive proof is sufficient given the single-sourcing, or worth a direct guard. Non-blocking — the current coverage already makes the masking bug impossible.
