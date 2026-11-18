TASK: ai-model-selection-1-7 — Add the layered TimeoutFor(verb) accessor with value semantics

ACCEPTANCE CRITERIA:
- cfg.TimeoutFor(verb) returns a present positive [verb].timeout as-is.
- An explicit [verb].timeout = 0 is HONORED as "no deadline" and STOPS fall-through (present shared/floor NOT consulted).
- A negative [verb].timeout DROPS through to shared, then to the 60s floor (never honored, never collapsed into zero/no-deadline).
- An absent [verb].timeout falls to shared; an absent shared falls to the 60s floor.
- A negative shared timeout (with absent per-verb) drops to the 60s floor.
- Return type is *time.Duration (never a wrapper or plain time.Duration), distinguishing explicit-0 (pointer to 0) from positive/floor (pointer to positive) — Phase 2's ai.Config.Timeout assigns it directly.
- With no .mint.toml, both verbs resolve to the 60s floor.
- All gates pass (build, gofmt, vet, test -race, golangci-lint).

STATUS: Complete

SPEC CONTEXT:
specification.md "Resolution value semantics (timeout)": zero is explicit/honored = "no deadline" and stops fall-through (NOT treated as missing); missing/invalid (e.g. negative) drops through; positive used as-is; floor is shipped 60s. The config→ai.Config boundary must preserve absent-vs-explicit-zero so "no deadline" is reachable ONLY by an operator's explicit 0, never by a wiring site omitting the field — planning fixed the mechanism as *time.Duration. Int-seconds representation means a non-integer is a Load-time decode error, so at the accessor the only value-invalid case is a negative integer. Transport's conditional WithTimeout and the three wiring sites are Phase 2, explicitly out of scope here.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/config/config.go:596-624 (TimeoutFor); supporting carriers Config.Timeout *time.Duration (config.go:144), Release.Timeout/Commit.Timeout *time.Duration (config.go:226, 262), resolveTimeout (config.go:657-663), defaults() seed (config.go:286-312), Verb enum (verb.go).
- Notes:
  - Candidate chain built exactly per spec: [override, c.Timeout, &floor] (config.go:606). Per-verb override selected exhaustively via the closed two-value Verb enum (VerbCommit → [commit], else [release]) — config.go:597-600.
  - Per-candidate switch (config.go:612-619) implements the four rules precisely: nil → continue; ==0 → return (honor, stop); <0 → continue (drop, not collapsed into zero); default (positive) → return as-is.
  - Floor is a local copy of the const DefaultTimeout (config.go:605) because a const's address cannot be taken; &floor is the last, always-present, positive candidate, making the result total (never nil). The unreachable trailing return &floor (config.go:623) keeps the method honest without relying on loop structure — mirrors AICommandFor.
  - Return type is *time.Duration as the plan fixes it; nil is never returned (floor guarantees non-nil), a pointer to 0 is explicit no-deadline, a pointer to positive is a real deadline — exactly the three-way distinction the spec's boundary invariant requires.
  - Per-key independence holds: the accessor reads only timeout candidates, never ai_command. Mirrors AICommandFor's structure while correctly applying timeout's distinct value rules (no blank-skip; the zero/negative semantics instead).
  - Negative-vs-floor interaction is correct: a verb timeout=-1 with no shared resolves to the 60s floor (negative dropped at both override and shared, floor applied).
  - WHY-comments (config.go:563-595) document the value semantics and the Phase-2 boundary/transport contract (skip WithTimeout on explicit 0) without touching the transport — matches the task's "document, do not modify transport here" instruction. As-built and accurate.
  - No drift: internal/ai and the three wiring sites were not modified by this task (Phase 2 markers present at ai/transport.go and the engine/commit sites confirm wiring is deferred, consistent with scope).

TESTS:
- Status: Adequate
- Coverage (config_test.go):
  - TestTimeoutFor_PresentPositivePerVerbOverride_UsedAsIs (2250) — positive override used as-is, both verbs, with a present shared proving the override wins.
  - TestTimeoutFor_ExplicitZeroPerVerb_HonouredAsNoDeadlineStopsFallThrough (2298) — explicit 0 returns pointer to 0; present shared 30s proven NOT consulted (the key "stops fall-through" assertion).
  - TestTimeoutFor_NegativePerVerb_DropsThroughToShared (2344) — negative drops to the present shared, not collapsed into zero.
  - TestTimeoutFor_NegativePerVerbNoSharedOverride_DropsToSixtySecondFloor (2389) — negative with no shared → 60s floor (negative-drop × floor interaction).
  - TestTimeoutFor_AbsentPerVerb_FallsToSharedThenFloor (2434) — both legs: absent→present shared, and absent+absent→floor.
  - TestTimeoutFor_NegativeSharedAbsentPerVerb_DropsToSixtySecondFloor (2486) — negative shared drops to floor.
  - TestTimeoutFor_NoConfigFile_ResolvesBothVerbsToSixtySecondFloor (2510) — zero-config both verbs → 60s floor.
  - TestTimeoutFor_ReturnDistinguishesExplicitZeroFromPositiveFloor (2531) — compile-time pin of exact *time.Duration signature (config.go:2543) plus runtime proof that explicit 0 → pointer to 0 and floor → pointer to positive. Directly guards the Phase-2 direct-assignment contract.
  - Per-key independence: TestResolution_OverrideCommandOnly_LeavesTimeoutAtFloor (2586), TestResolution_OverrideTimeoutOnly_LeavesCommandAtFloor (2635), TestResolution_PerVerbCommandOverrideWithSharedTimeout_EachKeyOwnChain (2684), TestResolution_ReleaseOverride_DoesNotPerturbCommitResolution (2734) — cover both directions and cross-verb non-bleed.
  - Every plan "Tests:" bullet maps to a test; every acceptance criterion is asserted (including the explicit-0-stops-fall-through proof with a present shared, and the *time.Duration type pin).
  - All assertions distinguish nil from value (Fatalf on nil pointer, then value check) — a regression to nil or to the wrong layer would fail. Edge cases from the spec (zero honored/stops, negative drops not collapsed, positive as-is, floor) are each independently exercised.
- Not over-tested: cases are behaviour-distinct (each maps to a separate rule or layer); table-driven over the two verbs avoids duplication. The compile-time signature pin is a single line, not redundant with the runtime checks (it guards the type, they guard the values). No excessive mocking — Load + TempDir only, per project idiom.

CODE QUALITY:
- Project conventions: Followed. External test package (config_test), table-driven, t.Parallel() throughout, t.TempDir() roots, exact-value assertions. Heavy WHY-comments kept true to as-built per CLAUDE.md (the doc on Config.Timeout / Release.Timeout / Commit.Timeout and the accessor narrate the absent-vs-explicit-zero contract accurately). resolveTimeout mirrors the existing resolveMaxDiffLines *int idiom, adding only the seconds→duration boundary. Accessor mirrors AICommandFor's shape, satisfying the "single place the value semantics live" centralization goal.
- SOLID principles: Good. Single responsibility (resolution only; transport conditional explicitly deferred). The value-semantics live in exactly one place.
- Complexity: Low. One linear loop over three candidates with a three-arm switch; clear, exhaustive code paths.
- Modern idioms: Yes. Closed typed enum for verb, *time.Duration pointer for the absent/zero distinction, tagless switch.
- Readability: Good. Intent is self-evident and the inline comments state the contract precisely.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/config/config.go:606,614 — TimeoutFor returns the caller's stored pointers (override / c.Timeout) directly, so a consumer mutating *result would mutate the Config's stored value via aliasing. The contract is read-only (Config is passed by value but the pointer fields are shared), and Phase 2 assigns the result into ai.Config.Timeout without mutating it, so this is safe as built. If defensive immutability is wanted, decide whether to return a fresh copy of the resolved value rather than the stored pointer; the &floor branch already returns a local copy, so only the override/shared branches alias. Decide whether the read-only contract (already implied by the comments) is sufficient or worth enforcing by copy.
