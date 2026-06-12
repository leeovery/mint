TASK: Assemble Conventional Commits prompt (L3) — tick-033497 / commit-command-1-2

ACCEPTANCE CRITERIA:
- Composed input is prompt + staged diff, in that order, nothing else — ordering holds under the [commit].prompt override (override replaces prompt segment only; diff stays in same trailing position, never dropped/reordered).
- Default prompt requests a Conventional Commits type: description subject (imperative, concise) with optional wrapped body for the why.
- Default prompt instructs the AI to infer the type from the diff.
- Default prompt instructs scope omitted by default (no (scope) guessing).
- Default prompt forbids any commit_prefix/branding and any preamble/meta-commentary.
- [commit].context (when set) injected into the default prompt without replacing it; absent context = default unchanged.
- [commit].prompt (when set) fully overrides the default while mint still supplies the diff; unreadable override fails loud (no fallback).
- No machine-parseable output wrapper requested (AI returns message directly).

STATUS: Complete

SPEC CONTEXT:
Spec "Commit Message Format & Prompt" + "AI Engine — Three-Layer Split" (Prompt boundary). L3 owns prompt assembly; L2 receives the finished prompt. Commit's default differs from release's: Conventional Commits 1.0.0 (type(scope): description, but scope off by default), AI infers type, no commit_prefix/branding, two-knob model ([commit].context inject + [commit].prompt full override). L2 still appends the diff under override. Pattern mirrors release's notes/ComposePrompt (internal/notes/prompt.go).

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/commit/prompt.go:31 (DefaultPrompt), :79 (ComposePrompt), :109 (ResolveInstructions), :128 (injectContext), :137 (injectOneTimeContext)
  - internal/config/config.go:177 (Commit struct), :546 (ResolveCommitPrompt — point-of-use override read, fail-loud)
  - internal/commit/generate.go:119-125 (wired: ResolveInstructions → injectOneTimeContext → ComposePrompt → L2 transport)
- Notes: Every acceptance criterion is met.
  - ComposePrompt is pure: strings.Join({instructions, diff}, "\n\n") — diff always trailing, including under override (override only replaces the instructions arg; ComposePrompt is order-blind to its source). Matches the criterion exactly.
  - DefaultPrompt carries all required rules verbatim: "type: description", "infer the type", "Omit the scope", imperative + concise, optional wrapped body for the why, no branding/prefix/emoji, "Return the commit message directly" (no machine wrapper), "no preamble and no meta-commentary".
  - ResolveInstructions precedence is correct: override (file) > default+context > default verbatim. Override read delegates to config.ResolveCommitPrompt, which fails loud on unreadable/missing (config.go:553) — no silent fallback. Context ignored under override (no default to inject into), which is the spec-correct behaviour.
  - Mirrors release's internal/notes/prompt.go shape (consumed pattern) cleanly; release's variant takes a 3-part input (instructions, changeMap, diff) — commit's 2-part input is the correct simplification (no Change Map at L3 for commit).

TESTS:
- Status: Adequate
- Coverage (prompt_test.go, 15 test funcs + generate_test.go integration):
  - Ordering instructions-then-diff: TestComposePrompt_OrdersInstructionsThenDiff (assertOrder).
  - "Nothing else": TestComposePrompt_ContainsOnlyTheTwoParts strips both parts and asserts only whitespace residue — a strong guard against smuggled content.
  - Default-prompt rules: TestDefaultPrompt_CarriesEveryRule (9 named subtests) + four targeted tests (subject/body, infer type, scope omitted, forbids branding/preamble) + no-machine-wrapper test.
  - Context inject (not replace) + default rules survive: TestResolveInstructions_Context_InjectedNotReplacing.
  - Absent context byte-identical to default: TestResolveInstructions_AbsentContext_LeavesDefaultUnchanged.
  - Full override replaces default + default rules absent: TestResolveInstructions_PromptFile_FullyOverridesDefault.
  - Override still flows through Compose with trailing diff: TestResolveInstructions_PromptOverride_StillFlowsThroughComposeWithDiff (assertOrder over override, diff).
  - Prompt wins over context: TestResolveInstructions_PromptTakesPrecedenceOverContext.
  - Unreadable override fails loud, no fallback: TestResolveInstructions_UnreadablePromptOverride_FailsLoudNoFallback.
  - One-time regenerate context (injectOneTimeContext, the private helper) covered at integration in generate_test.go:902 (on-top-of-persisted) and :933 (empty == plain Generate, byte-identical prompt).
  - Every "Tests:" bullet in the task maps to a named test.
- Notes:
  - Not under-tested: all 10 listed test cases present plus the one-time-context layer.
  - Not over-tested: TestDefaultPrompt_CarriesEveryRule overlaps the four targeted default-prompt tests (e.g. "Conventional Commits", "imperative", "scope", "no preamble" are asserted in both). This is mild redundancy but each targeted test pins a distinct acceptance criterion with its own failure message, so the duplication is defensible rather than bloat. No excessive mocking; tests assert behaviour (substrings/ordering), not implementation structure.

CODE QUALITY:
- Project conventions: Followed. Doc comments are full-sentence and explain WHY (Go style); exported identifiers documented; package doc states the L3 boundary precisely. Test naming and t.Parallel() usage match golang-testing conventions; testify not needed here (plain stdlib assertions are fine for substring/order checks).
- SOLID principles: Good. Clean SRP split — ComposePrompt is pure (order/join only, no IO), ResolveInstructions owns the IO/precedence, injectContext is the shared idiom for both persisted and one-time context (DRY via the header parameter). The pure/IO seam is exactly what makes ComposePrompt trivially testable.
- Complexity: Low. ResolveInstructions is two guards; the rest are one-liners.
- Modern idioms: Yes. strings.Join, fmt.Errorf with %w wrapping at both boundaries.
- Readability: Good. Intent is self-documenting; the diff-always-trailing invariant is stated where it matters.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/commit/prompt_test.go:96-108 — TestDefaultPrompt_CarriesEveryRule duplicates substrings already asserted by the four targeted default-prompt tests (Conventional Commits, imperative, scope, no preamble, no meta-commentary). Optionally fold the targeted tests into the table (or drop the overlapping table rows) to remove the double-coverage; low value, the current shape is acceptable.
- [quickfix] internal/commit/prompt_test.go:50-91 — No test pins the exact "\n\n" separator between instructions and diff. ContainsOnlyTheTwoParts proves "nothing else but whitespace" and assertOrder proves ordering, so the blank-line delimiter the doc comment promises (prompt.go:76-78) is only indirectly covered. Add one assertion that ComposePrompt(a,b) == a+"\n\n"+b if the exact separator is contractual.
