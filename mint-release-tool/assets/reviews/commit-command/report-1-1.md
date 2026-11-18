TASK: Read [commit] config table (tick-7bbf71, suffix 1-1)

ACCEPTANCE CRITERIA:
- Absent .mint.toml or absent [commit] table -> Context == "" and Prompt == "" (no error)
- [commit].context read and returned verbatim
- [commit].prompt read and returned as a path for assembly to resolve
- Wrong-typed context or prompt fails loud naming the key
- Configured prompt path missing/unreadable fails loud naming the path (never silently uses default)
- Empty context (no injection) and empty prompt (default) both valid non-error states
- No push/scope/per-verb-engine-override keys introduced for [commit]

STATUS: Complete

SPEC CONTEXT:
Config Schema section (spec lines 186-224): with mint multi-verb, config is verb-namespaced
tables + shared engine keys. Commit's surface is exactly two optional, typed, fail-loud keys:
[commit].context (context-inject knob) and [commit].prompt (full prompt override). Both optional;
absent = empty. Deliberately NOT added: push config, on_notes_failure analogue, scope toggle,
per-verb ai_command/max_diff_lines override. The $EDITOR-fallback spec (lines 103-133) is commit's
only failure path — so a configured-but-unreadable override MUST fail loud, never silently fall
through to the default. The plan task narrows this further: do NOT re-implement the schema; add
commitShape fields to fileShape + matching typeErrorMessages entries; defer the prompt-file read to
assembly (1-2) via ResolveCommitPrompt.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - Commit struct: internal/config/config.go:177-180 (Context, Prompt strings)
  - Config.Commit field: config.go:89
  - commitShape (TOML mirror): config.go:245-248
  - fileShape.Commit wiring: config.go:235
  - resolveCommit (verbatim pass-through): config.go:459-461, wired in Load at config.go:329
  - typeErrorMessages entries: config.go:367-368 ("commit.context must be a string",
    "commit.prompt must be a string")
  - ResolveCommitPrompt (point-of-use file read, fail-loud naming path): config.go:546-557
- Notes:
  - Matches the plan task precisely: only the [commit] table read was added onto the existing
    verb-namespaced config; no schema/shared-key re-implementation, no [release] touched.
  - Both keys are plain strings (no *string), correct because empty IS the documented absent
    default for both (mirrors releaseShape.Context/Prompt) — no absent-vs-zero distinction needed.
  - resolveCommit uses a direct struct conversion Commit(shape); sound because the two structs are
    field-identical (unlike resolveRelease, which must re-default *bool toggles). DRY, no premature
    abstraction.
  - "No push/scope/per-verb-engine keys" is satisfied structurally: commitShape defines only the two
    keys, and strict decoding (DisallowUnknownFields, config.go:307) rejects any extra [commit] key.
  - ResolveCommitPrompt deferral is genuinely consumed (not orphaned): internal/commit/prompt.go:110
    calls it inside ResolveInstructions (task 1-2), confirming the seam works end-to-end.
  - Fail-loud-naming-path holds: ResolveCommitPrompt wraps the os.ReadFile error with %w and the raw
    configured path (config.go:554), so both missing and unreadable surface the path; never returns
    "" silently on a configured-but-unreadable path.

TESTS:
- Status: Adequate
- Coverage: All eight planned micro-acceptance tests present in internal/config/config_test.go:
  - absent [commit] -> empty: TestLoad_AbsentCommitTable_ContextAndPromptEmpty (1189)
  - reads context: TestLoad_ExplicitCommitContext_Honoured (1209)
  - reads prompt as path: TestLoad_ExplicitCommitPrompt_HonouredAsPath (1227)
  - non-string context fails loud naming key: TestLoad_CommitContextNonString_RejectedNamingKey (1266)
  - non-string prompt fails loud naming key: TestLoad_CommitPromptNonString_RejectedNamingKey (1286)
  - missing override path fails loud naming path: TestResolveCommitPrompt_MissingPromptFile_FailsLoudNamingPath (1394)
  - unreadable override path fails loud naming path: TestResolveCommitPrompt_UnreadablePromptFile_FailsLoudNamingPath (1416)
  - empty context+prompt valid: TestLoad_EmptyCommitContextAndPrompt_AreValid (1245)
  Plus the configured-prompt happy path TestResolveCommitPrompt_ConfiguredPrompt_ReturnsFileContents
  (1368) and the extra TestLoad_CommitBooleanValue_StillFailsLoud (1311).
- Notes:
  - Tests verify behaviour, not implementation details: they assert error messages name the KEY and
    the expected "string" type, and name the configured PATH — exactly the acceptance criteria.
  - The extra boolean-value test (1311) is justified, not over-testing: go-toml/v2 emits no
    struct-field path for a boolean-into-string mismatch, so translateTypeError cannot map it to the
    mint-style message and the decoder's positioned description is surfaced instead. The test pins
    that the fail-loud guarantee still holds and the offending key stays visible — a real, distinct
    code path from the integer case. The comment (1314-1320) documents the library quirk well.
  - Each test is focused (one behaviour, minimal setup via writeConfig); no redundant assertions, no
    unnecessary mocking. Not over-tested.
  - The unreadable-file test (1416) strips perms to 0o000 and would silently pass (no error) if run
    as root, since root bypasses file permissions. Standard Go-test pitfall, not introduced here and
    not a behavioural defect; noted below as a non-blocking idea only.

CODE QUALITY:
- Project conventions: Followed. fmt.Errorf("...: %w", err) wrapping (config.go:554); user-facing
  CLI message style ("invalid .mint.toml: ...") consistent with every existing [release] key. The
  "lowercase, no trailing punctuation" error-string rule from golang-error-handling is an
  internal-error convention; mint's config messages are deliberately user-facing actionable CLI
  strings, an established whole-file pattern not changed by this task.
- SOLID principles: Good. Single responsibility per function; resolveCommit is a thin pure mapper;
  the file-read concern is correctly separated into ResolveCommitPrompt at the point of use rather
  than coupled into Load.
- Complexity: Low. resolveCommit is a one-line conversion; ResolveCommitPrompt is a guard + read +
  wrap.
- Modern idioms: Yes. errors.Is/As chain in Load, %w wrapping, *T absent-vs-zero idiom applied
  consistently and correctly omitted where empty is the meaningful default.
- Readability: Good. Doc comments on Commit (177-180), commitShape (241-244), resolveCommit
  (453-458), and ResolveCommitPrompt (536-545) are precise and explain the why (deferred read,
  fail-loud-not-fallback, field-identical conversion).
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/config/config_test.go:1416 — TestResolveCommitPrompt_UnreadablePromptFile silently
  passes when the suite runs as root (0o000 perms ignored). Consider guarding with a skip when
  os.Geteuid() == 0 to make the assertion meaningful in root/CI containers. Decide whether worth it.
- [quickfix] internal/config/config_test.go (commit section) — no unknown-key test exists for the
  [commit] table (e.g. "[commit]\npush = true"), unlike the [release]/[release.hooks]/top-level
  unknown-key tests (config_test.go:646-714). The "no push/scope keys" criterion is enforced by
  strict decode but not pinned by a test; add one [commit] unknown-key test mirroring the [release]
  one to lock the deliberately-excluded-keys guarantee against regression.
