TASK: ai-model-selection-2-5 — Route the regenerate wiring site through the release verb

ACCEPTANCE CRITERIA:
- `resolveFreshTransport` constructs `ai.Config` with `AICommand: cfg.AICommandFor(config.VerbRelease)` and `Timeout: cfg.TimeoutFor(config.VerbRelease)` — the RELEASE verb, not the bare shared/default.
- A [release].ai_command override drives the fresh-regenerate AI invocation's binary+args (argv asserted to carry the release values).
- A [commit].ai_command override does NOT affect the fresh-regenerate call (regenerate reads [release], never [commit]).
- A zero-config fresh-regenerate still resolves to `claude -p --model sonnet`.
- The fresh-regenerate timeout is sourced from `cfg.TimeoutFor(VerbRelease)` (never left zero-by-omission).
- The `resolveFreshTransport` WHY-comment records the deliberate release-verb routing (and the no-[regenerate]-table rationale) and no longer claims the transport re-defaults an empty value.
- Build/fmt/vet/test/lint gates pass.

STATUS: Complete

SPEC CONTEXT:
specification.md establishes regenerate as NOT a separate verb — `mint release regenerate --fresh` re-runs the release-notes task and resolves through [release] (there is no [regenerate] table). The verb parameter is a typed CLOSED enum in internal/config with exactly two values; the deliberate absence of a `regenerate` value makes the regenerate routing "un-missable" — regenerate_fresh.go can only pass the release verb (Migration & mechanical carry-overs flags this as the "easy miss" third wiring site; Acceptance criteria — resolution behaviors: "Regenerate routes through [release]"). All three wiring sites must source the timeout from the accessor (never zero-by-omission), preserving the absent-vs-explicit-0 invariant.

IMPLEMENTATION:
- Status: Implemented (no drift — planned refactor, see Notes)
- Location: internal/engine/regenerate_fresh.go:133-138 (`resolveFreshTransport`); routes through internal/aitransport/aitransport.go:40-45 (`aitransport.New`); enum at internal/config/verb.go:21-29; accessors at internal/config/config.go:538-561 (`AICommandFor`) and 596-624 (`TimeoutFor`).
- Notes: The production branch calls `aitransport.New(r, cfg, config.VerbRelease)`. The shared helper builds exactly `ai.NewTransport(r, ai.Config{AICommand: cfg.AICommandFor(verb), Timeout: cfg.TimeoutFor(verb)})`. Git history confirms task 2-5 (948ec19) first implemented the literal `ai.NewTransport(...{cfg.AICommandFor(config.VerbRelease), cfg.TimeoutFor(config.VerbRelease)})` exactly as the task prescribed; the later Phase-4 task 4-1 (260ab7f) consolidated all three wiring sites into the `aitransport.New` helper. This is the PLANNED Phase-4 consolidation, not drift — behaviour is byte-identical and the helper still passes `config.VerbRelease`. The closed enum has no `regenerate` value, so VerbRelease is the only verb this site can pass (a wrong verb is a compile error). The injected-transport short-circuit (`transport != nil`) is preserved untouched, as the task required.
- WHY-comment (regenerate_fresh.go:120-132): correctly states the deliberate RELEASE-verb routing, the no-[regenerate]-table rationale, the salience/timeout-exposure justification, the closed-enum un-missability, and that the `[release] → shared → floor` + never-zero-by-omission contract now lives in the helper. The old "NewTransport re-defaults an empty value to `claude -p`" claim is GONE (grep confirms no stale `re-default`/`claude -p`-without-model claim remains in the file).

TESTS:
- Status: Adequate
- Coverage:
  - Production-path [release] override (THE key easy-miss proof): TestRegenerateFreshBody_ReleaseVerbOverrideDrivesAIInvocation (regenerate_fresh_test.go:418) — transport=nil so resolveFreshTransport builds the real transport over the FakeRunner; asserts `stdinOf(t, f, "mybot", "gen", "--json")` non-empty AND `claude -p --model sonnet` never invoked.
  - [commit] non-leak: TestRegenerateFreshBody_CommitOverrideDoesNotDriveAIInvocation (line 460) — [commit].ai_command="wrongbot", no [release]/shared; asserts resolution to the floor `claude -p --model sonnet` and `wrongbot` never invoked.
  - Zero-config floor: TestRegenerateFreshBody_ZeroConfigResolvesToClaudeFloor (line 497) — asserts exact `claude -p --model sonnet` argv.
  - Timeout sourced from RELEASE accessor (white-box): regenerate_fresh_aitransport_internal_test.go — SourcesCommandFromReleaseAccessor (line 24, argv via DeadlineRecordingRunner spy), ExplicitZeroTimeoutThreadsNoDeadline (line 58, [release].timeout=0 ⇒ no deadline), PositiveTimeoutThreadsDeadline (line 83, absent [release].timeout ⇒ shared/floor positive ⇒ real deadline).
  - All five acceptance criteria are covered. The argv assertions carry the RELEASE values (override binary+args; floor `claude -p --model sonnet`), never the bare shared/default — exactly the spec's easy-miss requirement.
- Notes: Tests are honest. `stdinOf` requires an EXACT argv match (name+args joined) and t.Fatals if absent, so a non-empty return proves both exact argv and a prompt delivered. The accessors are total (resolve to the floor even on a bare config.Config{} not run through Load), so freshCfg()-based zero-config/commit tests genuinely exercise the floor. No over-testing: each test pins one distinct behaviour (release routing, commit non-leak, zero-config floor, timeout presence/absence); the timeout split between zero/positive cases is necessary, not redundant. The internal white-box file legitimately exercises the unexported resolveFreshTransport directly for the timeout proofs (FakeRunner discards context, so the DeadlineRecordingRunner spy is required) — this is the right tool, not implementation-detail coupling.

CODE QUALITY:
- Project conventions: Followed. Honors CLAUDE.md seam #5 (AI through internal/ai behind the helper), the typed closed-enum single-source-of-truth, the never-zero-by-omission *time.Duration boundary, and the "comments true to as-built in the same change" rule (stale re-default claim removed). Exact-argv test idiom respected.
- SOLID principles: Good. The shared aitransport.New centralizes the one construction expression across three sites (single responsibility for verb→transport mapping); resolveFreshTransport owns only the seam short-circuit.
- Complexity: Low. resolveFreshTransport is a 5-line nil-check + delegation.
- Modern idioms: Yes. Closed int enum with iota, *time.Duration for absent-vs-zero.
- Readability: Good. The WHY-comment is thorough and accurate; intent (deliberate release routing, no-[regenerate]-table) is explicit.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
