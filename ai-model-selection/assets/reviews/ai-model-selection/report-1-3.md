TASK: ai-model-selection-1-3 — Add the per-verb ai_command override to the schema and strict decoding

ACCEPTANCE CRITERIA:
- [release].ai_command and [commit].ai_command decode without strict-decode rejection.
- Both are carried raw onto the config structs (absent → nil; present → the literal string verbatim, blank or not).
- An integer assigned to either key surfaces the mapped friendly message naming the key and string type at Load.
- A boolean assigned to either still fails loud (library fallback text, key still visible) — no silent accept.
- An unknown sibling key inside [release] / [commit] is still rejected naming the key and table.
- Build/gofmt/vet/test/lint gates all pass.

STATUS: Complete

SPEC CONTEXT:
specification.md "Config schema: per-verb ai_command override" promotes ai_command from shared-only to a key living at both levels with fallback (verb → shared → shipped default). "Strict-decoding requirement" mandates the new per-verb keys be added to both verb shape structs with typeErrorMessages entries, else DisallowUnknownFields rejects them. This task is scoped to decode-and-carry only; resolution (AICommandFor) is Task 1-4. The *string choice is required so absent (nil) is distinguishable from an explicit blank — Task 1-4's blank-skip depends on it. The Commit doc-comment reconciliation is explicitly deferred to Phase 3 (task note + spec "Cross-spec reconciliation").

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/config/config.go:349 (commitShape.AICommand *string `toml:"ai_command"`)
  - internal/config/config.go:366 (releaseShape.AICommand *string `toml:"ai_command"`)
  - internal/config/config.go:225 (Release.AICommand *string carried field)
  - internal/config/config.go:251 (Commit.AICommand *string carried field)
  - internal/config/config.go:474,478 (typeErrorMessages: releaseShape.AICommand / commitShape.AICommand)
  - internal/config/config.go:681 (resolveRelease copies shape.AICommand verbatim)
  - internal/config/config.go:705-712 (resolveCommit converted to explicit field-by-field copy, mirroring resolveRelease)
- Notes:
  - Both shape fields use *string exactly as the task directs — absent (nil) vs explicit blank preserved.
  - resolveCommit was correctly converted from the old direct Commit(shape) conversion to a field-by-field copy; the task flagged that the *string/*time.Duration shape divergence breaks field-identity, and the implementation handles it (no struct-conversion compile break).
  - defaults() (config.go:286-312) correctly does NOT seed per-verb ai_command — absent baseline stays nil, matching "Do NOT seed defaults() for these per-verb keys."
  - typeErrorMessages strings match the task verbatim: "release.ai_command must be a string" / "commit.ai_command must be a string".
  - DisallowUnknownFields is untouched (config.go:412); the new keys are recognised struct fields, so siblings remain rejected — no loosening.

TESTS:
- Status: Adequate
- Coverage (internal/config/config_test.go):
  - TestLoad_ReleaseAICommand_CarriedRawOntoConfig (1388) — decodes [release].ai_command, asserts non-nil pointer + verbatim value.
  - TestLoad_CommitAICommand_CarriedRawOntoConfig (1410) — commit-verb mirror.
  - TestLoad_AbsentPerVerbAICommand_DistinctFromExplicitBlank (1431) — absent → nil (both verbs) AND explicit "" → non-nil pointer to "". This is the load-bearing *string distinction; covered in both sub-cases.
  - TestLoad_IntegerReleaseAICommand_RejectedNamingKeyAndStringType (1709) — integer → mapped message naming "release.ai_command" and "string".
  - TestLoad_IntegerCommitAICommand_RejectedNamingKeyAndStringType (1730) — commit mirror.
  - TestLoad_BooleanPerVerbAICommand_StillFailsLoud (1750) — boolean fails loud, key visible, deliberately does NOT over-assert the mapped message (correctly honours the documented go-toml/v2 quirk pinned by TestLoad_CommitBooleanValue_StillFailsLoud).
  - TestLoad_UnknownSiblingKey_StillRejectedAfterAddingAICommand (1786) — unknown sibling alongside a valid ai_command in [release]/[commit] still rejected naming key + table.
- Notes:
  - Every acceptance criterion and every named test from the task is present and verifies behaviour (would fail if the field were dropped, mis-typed, or if strict decoding were loosened).
  - Not over-tested: each test pins one distinct behaviour; no redundant happy-path duplication. The boolean test correctly restrains its assertion to fail-loud + key-visibility rather than over-asserting an unmappable message.
  - Edge cases from the task (genuine type mismatch as strict decode error; unknown siblings still rejected) are both directly covered.

CODE QUALITY:
- Project conventions: Followed. *string absent-vs-zero idiom mirrors the existing *bool/*int handling; resolveCommit now mirrors resolveRelease's explicit copy; typeErrorMessages entries follow the established "<key> must be a <type>" pattern; tests are external package (config_test), t.Parallel throughout, t.TempDir roots, exact-message assertions — all matching CLAUDE.md test idioms. The stdlib if/t.Errorf style (not testify) is the established convention for this file.
- SOLID principles: Good. Single source of truth for the carried value; resolution deferred to the accessor (single responsibility split honoured).
- Complexity: Low. Pure decode-and-copy; no new branching.
- Modern idioms: Yes.
- Readability: Good. WHY-comments on the shape and struct fields accurately state the *string absent-vs-blank contract and that blank-skipping is the accessor's job, true to as-built.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
