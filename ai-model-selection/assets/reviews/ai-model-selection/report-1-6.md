TASK: ai-model-selection-1-6 — Add per-verb timeout overrides to the schema and strict decoding

ACCEPTANCE CRITERIA:
- [release].timeout and [commit].timeout decode (integer seconds) without strict-decode rejection.
- Both preserve absent (nil) vs explicit 0 on the carried config (per-verb absent-vs-zero distinction).
- An integer is carried as that many seconds; a negative integer is carried raw (drop is 1-7's job).
- A non-integer per-verb timeout is rejected at Load with a mapped friendly message naming the key and integer/seconds type.
- An unknown sibling key in [release] / [commit] is still rejected.
- Gates pass (build, gofmt, vet, test -race, golangci-lint).

STATUS: Complete

SPEC CONTEXT: specification.md "Config schema: timeout key" + "Resolution value semantics" establish timeout as net-new config surface mirroring ai_command's layering. The int-seconds representation makes a non-integer TOML value a strict decode (type) error at Load, a negative an integer-typed value-invalid drop-through, and absent-vs-zero a nil pointer vs 0. This task (1-6) makes the per-verb keys decode, carry through with absent-vs-zero preserved, and fail loud on wrong type; resolution semantics (negative-drop, zero-honour, fall-through) are 1-7.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/config/config.go:350 (commitShape.Timeout *int `toml:"timeout"`)
  - internal/config/config.go:367 (releaseShape.Timeout *int `toml:"timeout"`)
  - internal/config/config.go:226 (Release.Timeout *time.Duration) / :262 (Commit.Timeout *time.Duration)
  - internal/config/config.go:475 + :479 (typeErrorMessages entries for releaseShape.Timeout / commitShape.Timeout)
  - internal/config/config.go:657-663 (resolveTimeout: seconds → *time.Duration, nil preserved, sign preserved)
  - internal/config/config.go:682 (resolveRelease wires Timeout via resolveTimeout)
  - internal/config/config.go:705-712 (resolveCommit is now an explicit field-by-field copy; the old direct Commit(shape) conversion is gone, as required by 1-3 and forced by the *int→*time.Duration boundary)
- Notes: Matches Task 1-5's integer-seconds *int representation. Per-verb values carried as *time.Duration with nil = absent, non-nil = explicit (including 0 and negatives raw), exactly per the task's "Do" list. No per-verb default seeded (defaults() seeds only top-level Timeout), so the per-verb absent baseline is nil — correct. WHY-comments on both struct fields and resolveTimeout/resolveCommit are accurate to as-built and state the 1-6/1-7 boundary explicitly (satisfies CLAUDE.md comment contract; the Commit doc comment no longer encodes the old "no per-verb override" stance).

TESTS:
- Status: Adequate
- Coverage (all seven task-listed tests present and targeted):
  - TestLoad_ReleaseTimeout_CarriedAsSeconds (config_test.go:2083) — [release].timeout=120 carries 120s.
  - TestLoad_CommitTimeout_CarriedAsSeconds (:2104) — [commit].timeout=45 carries 45s.
  - TestLoad_AbsentPerVerbTimeout_DistinctFromExplicitZero (:2126) — absent → nil for both verbs; explicit 0 → non-nil pointer to 0 for both. Directly proves the absent-vs-zero distinction.
  - TestLoad_NegativePerVerbTimeout_CarriedRaw (:2182) — -5/-30 carried raw as negative durations.
  - TestLoad_TypeMismatch_MappedFriendlyMessages (:744, rows :758-759) — [release].timeout="slow" and [commit].timeout="slow" each assert the exact mapped message naming the key + "integer (seconds)".
  - TestLoad_UnknownSiblingKeyAfterTimeout_StillRejected (:2211) — unknown sibling alongside timeout in [release] and [commit] still rejected by name.
- Notes: Tests assert exact carried durations and exact (substring) messages per project test idioms; t.Parallel throughout; black-box config_test package. Would fail if the field were dropped, mistyped, the conversion lost the sign, or strict decoding loosened. Not over-tested — each test pins a distinct acceptance criterion with no redundant happy-path duplication. The 1-7 TimeoutFor resolution tests (:2250 onward) live in the same file but belong to 1-7; they do not bloat 1-6's coverage.

CODE QUALITY:
- Project conventions: Followed. Strict-decode + typeErrorMessages pattern mirrors max_diff_lines exactly; *int file-shape / pointer-carry idiom is consistent with publish/changelog/max_diff_lines. resolveCommit converted to explicit copy (no struct-identity conversion) as the schema demands.
- SOLID principles: Good. resolveTimeout is a single-responsibility boundary converter reused by top-level, release, and commit (DRY); no duplication of the seconds→duration logic.
- Complexity: Low. resolveTimeout is a two-branch function; resolveRelease/resolveCommit are flat field copies.
- Modern idioms: Yes. Pointer-for-absent-vs-zero is the idiomatic Go choice here; time.Duration(*seconds)*time.Second is correct.
- Readability: Good. WHY-comments precisely delineate the 1-6 vs 1-7 responsibility split and the absent/zero/negative semantics.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
