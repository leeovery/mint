TASK: Parse -a/-A flags with mutual-exclusion fail-loud (tick-54f856, suffix 2-1)

ACCEPTANCE CRITERIA:
- -a/--all resolves to the tracked-mods+deletions staging mode
- -A/--add-all resolves to the everything-including-untracked staging mode
- Neither flag resolves to the Phase 1 staged-only default (unchanged behaviour)
- -a and -A together fail loud with '-a and -A cannot be combined; -A already includes -a's changes'
- Bundled short flags pre-expanded before fs.Parse (-aA -> -a -A) so the conflict reaches the guard (not stdlib's generic unknown-flag error)
- Long flags (--all, --add-all, --no-ai, --plain etc.) pass through pre-expansion unexpanded
- The mutual-exclusion failure occurs before any preflight, diff, or AI call — no CommandRunner/ai_command invocation, index untouched
- Both long forms parse identically to short forms
- No -p/--no-ai/gate-e/r behaviour implemented (deferred)

STATUS: Complete

SPEC CONTEXT:
Staging Model (spec lines 59-88) defines two faithful flags: default = staged-only (commit the index as staged), -a/--all = git commit -a (tracked mods + deletions, no untracked), -A/--add-all = git add -A then commit (everything incl untracked). -a and -A are mutually exclusive: supplying both is a conflicting-flags error that must fail loud BEFORE any work, with the exact message "-a and -A cannot be combined; -A already includes -a's changes" — mint never silently picks a winner. CLI Surface (lines 228-243) confirms the flag spellings and the -Ap / -Apy bundle ergonomics. This task owns ONLY flag parsing + the mutual-exclusion guard + the resolved StagingMode value; it computes no diff and applies no staging (deferred to 2-2/2-3).

IMPLEMENTATION:
- Status: Implemented (no drift)
- Location:
  - cmd/mint/commit_flags.go:50-79 (parseCommitFlags) — registers -a/--all, -A/--add-all (paired short/long), pre-expands bundles before fs.Parse, resolves the mode.
  - cmd/mint/commit_flags.go:86-97 (resolveStagingMode) — the mutual-exclusion guard with the spec's exact message; addAll-before-all ordering; default StagedOnly.
  - cmd/mint/commit_flags.go:110-148 (expandShortFlagBundles / shortFlagBundle) — POSIX bundle pre-expansion; only single-`-` tokens whose every char is a DEFINED single-letter flag are expanded; `--` long flags and the `--` terminator pass through verbatim.
  - internal/commit/staging.go:12-24 (StagingMode enum) — StagedOnly (iota zero value) / All / AddAll, correctly making the zero value the Phase 1 default.
  - cmd/mint/main.go:301,350 — parse (with guard) runs before commit.Run; the conflict short-circuits before Deps/git/AI are ever constructed.
- Notes:
  - All nine acceptance criteria are met. The "fails before any git/AI call, index untouched" criterion is satisfied structurally: the guard lives in a pure function (parseCommitFlags(args []string) — no runner/transport in scope) that returns at main.go:301, well before commit.Run at main.go:350 constructs any seam. Nothing can run.
  - Deferred scope respected: -p/--push and --no-ai are now parsed too, but git/dispatch shows they were added by later tasks (5-1, 3-2), not this one (commit 726842d). Task 2-1 added only the -a/-A surface + guard + bundle expansion. No gate e/r behaviour here.
  - Bundle terminator handling (`--`) and the all-chars-defined gate are correct and defensive — `-xz`, lone `-`, and values pass through to fs.Parse unchanged.

TESTS:
- Status: Adequate
- Coverage (cmd/mint/commit_flags_test.go):
  - -a/--all -> All, -A/--add-all -> AddAll, neither -> StagedOnly: TestParseCommitFlags table (lines 34-37, 29).
  - long == short: TestParseCommitFlags + TestParseCommitFlags_LongFlagsPassThroughUnexpanded (lines 204-228), which also pins that --add-all (containing 'a'/'A') is not mistaken for a bundle, and that an unknown long flag (--aA) stays a parse error.
  - conflict fail-loud with the EXACT message across separate (-a -A), bundled (-aA / -Aa), long (--all --add-all), and bundle-with-other (-aA -y) forms: TestParseCommitFlags_ConflictingStaging (lines 131-153), with the message pinned to a const that quotes the spec verbatim (line 123).
  - bundle pre-expansion: TestParseCommitFlags_BundledShortFlags (lines 162-197) incl. -ay/-ya/-Ap/-Apy.
  - StagingMode enum contract: internal/commit/staging_test.go (zero value = StagedOnly; distinct values).
- Notes:
  - The plan's 8th listed test ("conflict fails before any git/AI call — no CommandRunner or ai_command invocation recorded") has no explicit no-git/no-AI assertion. This is not a gap: parseCommitFlags is a pure function with no runner or transport in scope, so there is nothing to record an invocation against — the property holds by construction and an assertion would have nothing to observe. The structural placement (parse before Run) is the real guarantee, and it is exercised end-to-end by TestRunVerb_Help_ExitsZero's "no seam constructed" sibling reasoning. Acceptable as-is.
  - Not over-tested: the three conflict-shaped test functions are distinct concerns (general flag table / push-only / conflict / bundling / long-passthrough), not redundant happy-path variations. The conflict const is shared, not duplicated.
  - Tests assert behaviour (resolved mode, error text) not implementation internals.

CODE QUALITY:
- Project conventions: Followed. Paired short/long BoolVar registration matches the established resolveBump idiom (flags.go); errors are lowercase non-punctuated fmt.Errorf per golang-error-handling; table-driven t.Parallel() tests per golang-testing; names are clear and idiomatic per golang-naming.
- SOLID principles: Good. parseCommitFlags orchestrates; resolveStagingMode owns the guard; expandShortFlagBundles/shortFlagBundle isolate the bundling concern — clean single responsibilities.
- Complexity: Low. shortFlagBundle's guard chain is linear and readable; expandShortFlagBundles is a single pass with an explicit `--` terminator break.
- Modern idioms: Yes. switch-with-no-condition for the guard, range-over-string for chars, pre-sized slices.
- Readability: Good. Doc comments are thorough and explain the WHY (no POSIX bundling in stdlib flag) rather than restating code.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] cmd/mint/commit_flags_test.go:131-153 — TestParseCommitFlags_ConflictingStaging asserts the conflict error but not that no staging side effect / no runner call occurred. Since parseCommitFlags is pure this is informational only; if a future refactor moved the guard into commit.Run, add an assertion that gitInvocations(r) is empty on a -aA run to lock the "before any git/AI" criterion at the integration layer. Concrete edit: a single integration test in internal/commit asserting zero git invocations when Staging conflict is rejected — only worth adding if the guard ever leaves the pure parse layer.
