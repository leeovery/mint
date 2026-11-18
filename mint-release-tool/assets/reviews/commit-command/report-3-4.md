TASK: Route oversized diff (max_diff_lines) to the editor fallback with note (commit-command-3-4 / tick-7910fb)

ACCEPTANCE CRITERIA:
- An over-limit (diff_exclude-filtered) diff is detected at L1, before any L2 call — no ai_command/claude invocation
- diff_exclude is applied first, so excluded noise alone cannot push a diff over the limit
- The note 'diff too large to summarise — opening editor' is emitted via the consumed Presenter
- The oversized case routes to the editor fallback with save-as-accept reused from 3-2
- The oversized outcome is treated as a generate-skip, distinct from the generate-failure (3-3) — no oversized note on the failure path, no L2 call on the oversized path
- Boundary: a diff at max_diff_lines passes to L2 (inclusive boundary — exactly maxLines passes); only over-limit triggers the fallback
- The line count is consumed (notes.CheckDiffSize + notes.ErrDiffTooLarge via commit's own L1 glue per 1-3), diff_exclude filtering is the L1 glue's :(exclude) pathspecs — not re-implemented

STATUS: Complete

SPEC CONTEXT:
Commit's spec (Commit Message Format & Prompt -> The $EDITOR fallback, case 3 + Detection ordering) states max_diff_lines is evaluated at L1 after diff_exclude and before any L2 call. An over-limit diff is a generate-SKIP (like --no-ai), not a generate-FAILURE — it short-circuits L2 and routes to the $EDITOR fallback with the verbatim note 'diff too large to summarise — opening editor'. The -y/non-TTY forbidden-combo (3-5) then applies: an unattended run hitting the fallback has no message source and fails loud. Line-counting mechanics are settled in the shared engine (notes.CheckDiffSize, inclusive boundary) and reused; only the fall-back-rather-than-abort branch is commit-specific.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/commit/generate.go:115-117 — the consumed L1 size guard (notes.CheckDiffSize, wrapped notes.ErrDiffTooLarge), applied AFTER sourceDiff (diff_exclude :(exclude) pathspecs at L1) and BEFORE transport.Generate. No re-implementation — sentinel and counter are consumed from internal/notes.
  - internal/commit/run.go:318-323 — the oversized branch: errors.Is(err, notes.ErrDiffTooLarge) emits the note (gated on !deps.editorUnavailable()) then routes to runEditorFallback with an empty buffer. Distinct from the isAITransportFailure branch (run.go:330-332) which carries no note.
  - internal/commit/run.go:55-58 — oversizedNoteMessage constant = the spec string verbatim (em dash U+2014).
  - internal/commit/run.go:240-241 — editorUnavailable() = deps.Yes || !deps.StdinInteractive, the single shared predicate so the note and the fallback's fail-loud guard (run.go:412) cannot drift.
- Notes:
  - The guard is mode-agnostic (StagedOnly / All / AddAll all flow through sourceDiff -> CheckDiffSize), so an over-ceiling -a/-A would-be-staged diff is caught at L1 too.
  - One subtlety handled correctly and beyond the literal acceptance list: the note is suppressed on an unattended run (editorUnavailable) so it never promises an editor that will never open. The fallback then fails loud via the same predicate. This is the 3-5 hand-off done cleanly here rather than left as a gap.

TESTS:
- Status: Adequate
- Coverage (every task micro-test maps to real test code):
  - over-limit skips L2 + routes to editor: run_oversized_test.go:125 (TestRun_Oversized_SkipsL2_RoutesToEditor) — transport.calls()==0, one editor launch, commit carries saved body. Driven through the REAL Generator over a FakeRunner so the real CheckDiffSize fires.
  - emits the verbatim note: run_oversized_test.go:154 (TestRun_Oversized_EmitsNote) — byte-for-byte equality against the em-dash string, not substring.
  - diff_exclude before the count: run_oversized_test.go:183 (TestRun_Oversized_DiffExcludeAppliedBeforeCount) — asserts the :(exclude) pathspec is issued at L1 and a within-ceiling post-exclusion diff reaches L2 with no fallback/note.
  - inclusive boundary at-limit -> L2: run_oversized_test.go:219 (TestRun_Oversized_AtLimitPassesToL2). Over-limit -> fallback: run_oversized_test.go:252 (TestRun_Oversized_OverLimitTriggersFallback). Strict complements at max=3 (3 lines vs 4 lines). Reinforced at the unit/guard level by internal/notes/size_test.go:21,33.
  - reuses save-as-accept from --no-ai unchanged: run_oversized_test.go:281 (stage-then-commit under -a, add-before-commit ordering asserted), :318 (empty/aborted save = true no-op, table over whitespace-only + aborted), :371 (empty buffer / no synthetic stub).
  - distinct from generation failure: run_oversized_test.go:399 (both halves — oversized carries note + skips L2; AI-failure carries NO oversized note) and run_aifail_test.go:283 (the branch boundary: a wrapped ErrDiffTooLarge routes through the oversized branch, note as discriminator).
  - L1 guard before transport + errors.Is distinguishability: generate_test.go:273, :299, :394, plus the -a/-A source variants generate_test.go:556 and :774.
  - unattended UX fix (3-5 hand-off): run_oversized_failloud_test.go:17 — over-limit under -y and non-TTY both fail loud, suppress the note, and mutate nothing.
- Notes:
  - Not under-tested: the inclusive boundary, the diff_exclude-first ordering, the skip-vs-failure distinction, the save-as-accept reuse, and the unattended fail-loud are all exercised, several at both the unit (Generator) and end-to-end (Run) levels.
  - Not over-tested: the two-level coverage is justified — the unit tests pin the guard's contract, the Run tests pin the orchestration branch; they assert different things. No redundant assertions. The oversized path does NOT re-test push integration (-p), correctly relying on the --no-ai editor-fallback push tests since runEditorFallback/commitAccept is reused verbatim — avoiding over-testing.

CODE QUALITY:
- Project conventions: Followed. Consumes the shared notes.CheckDiffSize/ErrDiffTooLarge rather than re-implementing (golang-design-patterns / DRY); errors.Is sentinel routing with %w wrapping (golang-error-handling); table tests + t.Parallel + behaviour-over-implementation assertions (golang-testing); the local Transport seam keeps the consumer decoupled (golang-structs-interfaces).
- SOLID principles: Good. Single shared editorUnavailable() predicate prevents the note-gate and the fallback fail-loud guard from drifting; runEditorFallback is the one convergence point for all three no-AI cases.
- Complexity: Low. The oversized branch is a flat errors.Is check + a gated Warn + a delegate call.
- Modern idioms: Yes — errors.Is, %w, strings.Builder, NUL-split enumeration.
- Readability: Good. Doc comments are thorough and accurately state the skip-vs-failure distinction and the note-suppression rationale.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
