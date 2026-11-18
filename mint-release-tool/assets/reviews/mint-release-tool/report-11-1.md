TASK: 11-1 — Gate Regenerate gh-auth on the Resolved Publisher, Not the Bare Target (type: bug)

ACCEPTANCE CRITERIA:
- Downgraded run (nil publisher) + provider-writing target → RegeneratePreflight does NOT run CheckGhAuth.
- Resolved-publisher run + provider-writing target → RegeneratePreflight still runs CheckGhAuth exactly as today.
- CommitsAndPushes bucket (clean-tree + on-branch + remote-sync) selected SOLELY from target.writesChangelog(), unaffected by publisher presence.
- Downgrade behaviour now identical across forward (engine.Release) and regenerate (RegenerateRun / RegenerateAllValidated): warn-and-downgrade, gh-auth skipped.
- Changelog-only (--target changelog) gate selection unchanged regardless of publisher presence.

STATUS: Complete

SPEC CONTEXT:
Spec "Preflight subset per verb" (line 547) and the publishing/downgrade rules (lines 128, 429-430) are decisive. The `gh` gate is "gated only when actually publishing a GitHub release" (line 128). A non-github / no-remote origin "warns loudly and downgrades to tag + push only" and — explicitly — "because the gh gate runs only when actually publishing — never strands a pushed tag waiting on a release it can't create" (line 430). The bug was that regenerate's gh-auth gate was keyed off the bare target (writesProvider()) rather than whether publishing actually resolves, so a downgraded regenerate run warned "provider skipped" and THEN still aborted on a dead gh auth — the opposite of the spec intent and of the forward spine (release.go:570 guards CheckGhAuth behind `if publisher != nil`). The fix realigns regenerate with the spec and the forward path.

IMPLEMENTATION:
- Status: Implemented (correct, complete, no drift)
- Location:
  - internal/engine/regenerate.go:67-72 — selector now `regenerateGateSet(target RegenerateTarget, publisherResolved bool)` computing `CallsProvider: target.writesProvider() && publisherResolved`; `CommitsAndPushes: target.writesChangelog()` unchanged. Doc comment (lines 52-66) updated to state the gh-auth gate requires BOTH a provider-writing target AND a resolved publisher, citing the forward spine's `if publisher != nil` guard, and noting CommitsAndPushes is unaffected by publisher presence.
  - internal/engine/regenerate_interactive.go:168 (RegenerateRun) — passes `publisher != nil`; the publisher param is already in scope. Comment at 162-167 updated to describe the publisher-presence guard.
  - internal/engine/regenerate_batch.go:131 (RegenerateAllValidated) — passes `publisher != nil`; comment at 125-130 updated.
- Notes:
  - The selector is the SINGLE owner — grep confirms only the two preflight call sites and the definition reference it; no other caller needs updating (Do step 5 satisfied).
  - The RegenerateGateSet struct and RegeneratePreflight gate ordering are untouched (regenerate.go:43-50, 87-114) — only the SELECTION input changed, as required.
  - Chose the boolean-parameter form (`publisherResolved bool`) over a sibling selector. Clean and minimal; the call sites pass `publisher != nil` so the engine selector stays free of the publish.Publisher type, preserving its "pure mapping with no knowledge of cmd-layer enums" contract.
  - Typed-nil trap checked and clear: cmd-layer resolveRegeneratePublisher (main.go:221) returns engine.ResolvePublisher's result directly, which yields a genuine nil interface on downgrade (asserted by TestResolvePublisher_Downgrade_WarnsReturnsNilNoError, regenerate_nilpublisher_test.go:125). The downstream provider-write nil-guards (regenerate_batch.go:297, RegenerateWrite) already use the same `publisher == nil` test, so the gate selection and the write skip agree on the same nil.

TESTS:
- Status: Adequate (well-targeted, no over/under-testing)
- Coverage:
  - Nil-publisher → gate skipped: TestRegenerateRun_DowngradedReuse_SkipsGhAuthGate (regenerate_interactive_test.go:582) and TestRegenerateAllValidated_DowngradedReuse_SkipsGhAuthGate (regenerate_batch_preflight_test.go:128). Both seed a FAILING gh-auth recorder (ExitCode 1) and a genuine `var pub publish.Publisher` nil interface, then assert the run completes (would abort if the gate ran) AND `!ghAuthRan(f)`. This is the strong form — it proves both that the run survives and that the gh probe was never issued.
  - Non-nil publisher → gate runs: TestRegenerateRun_ResolvedRelease_RunsGhAuthGate (interactive_test.go:610) and TestRegenerateAllValidated_ResolvedRelease_RunsGhAuthGate (batch_preflight_test.go:159) assert ghAuthRan(f) with a resolved fake publisher.
  - Downgraded reuse/release flow completes without aborting on gh-auth: covered by the two DowngradedReuse tests above (the failing-gh recorder is the "must not be reached" guard the task asked for).
  - Existing RegeneratePreflight gate/ordering tests stay green: regenerate_preflight_test.go drives the gate IMPLEMENTATION directly via constructed gate sets (releaseGateSet/changelogGateSet/bothGateSet) — untouched by the selection change, exactly as the task intended.
  - CommitsAndPushes unaffected by publisher: TestRegenerateAllValidated_ChangelogOnly_GateSelectionUnaffectedByPublisher (batch_preflight_test.go:191) runs a changelog-only batch with a NIL publisher and asserts the full commit/push bucket STILL runs and gh-auth does NOT — directly pinning that the changelog bucket is selected solely from the target.
  - The "both" selection is exercised via bothGateSet() in the preflight tests and the gate-before-mutation ordering tests; CallsProvider for "both" with a resolved publisher is covered transitively by the resolved-release path plus the changelog-only proof that the changelog half is publisher-independent.
- Notes:
  - Tests assert observable behaviour (which git/gh argv the FakeRunner recorded via ghAuthRan/cleanTreeRan/etc.), not the internal boolean — the right level. No redundant assertions, no excessive mocking.
  - The downgrade tests deliberately use a genuine nil interface (not a typed-nil concrete pointer), with an inline comment explaining why — this matches the production downgrade value and would catch a typed-nil regression.

CODE QUALITY:
- Project conventions: Followed. Doc comments are thorough and explain the WHY (the forward-spine parity, the nil-guard agreement). Boolean param is named publisherResolved with a documented derivation. Matches the surrounding engine style.
- SOLID principles: Good. Selector remains a single-responsibility pure mapping; the publisher-presence fact is threaded in by the callers that own it rather than the selector reaching for the publish.Publisher type.
- Complexity: Low. One extra `&& publisherResolved` term and a threaded bool; no new branches.
- Modern idioms: Yes. Idiomatic Go; `publisher != nil` at the call sites is the standard interface-presence check.
- Readability: Good. The two call-site comments and the selector doc comment all cross-reference the forward spine's `if publisher != nil` guard, so the parity is self-documenting.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
