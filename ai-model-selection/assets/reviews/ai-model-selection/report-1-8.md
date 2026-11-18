TASK: ai-model-selection-1-8 — Prove per-key resolution independence across ai_command and timeout

ACCEPTANCE CRITERIA:
- Overriding [release].ai_command only leaves TimeoutFor(VerbRelease) at the 60s floor.
- Overriding [release].timeout only leaves AICommandFor(VerbRelease) at the shipped default floor.
- The same independence holds for VerbCommit (both directions).
- A per-verb command override combined with a shared top-level timeout resolves the command to the override and the timeout to the shared value (independent chains).
- A [release] override does not perturb VerbCommit resolution (each verb reads its own table).
- go build / gofmt / go vet / go test -race / golangci-lint all pass.

STATUS: Complete

SPEC CONTEXT:
specification.md "Resolution value semantics" states resolution is per-key INDEPENDENT — ai_command and timeout each fall back through their own verb → shared → default chain. "Timeout × model-choice coupling" makes the failure mode explicit: a verb overriding ai_command to a slower model but not timeout silently inherits the shared 60s default; that is operator responsibility, but the *mechanism* (each key reads only its own field, never leaks into the other) must hold. "Acceptance criteria — resolution behaviors" lists "Per-key independence — overriding ai_command on a verb leaves that verb's timeout resolving through the shared/floor (and vice-versa)" as a first-class testable behavior. The task is explicitly a verification-only TDD cycle: no new production code expected unless a test reveals coupling.

IMPLEMENTATION:
- Status: Implemented (verification-only — no production change, as planned)
- Location: tests in internal/config/config_test.go:2586-2765 (4 new test functions); accessors under test at internal/config/config.go:538 (AICommandFor) and :596 (TimeoutFor); Verb enum at internal/config/verb.go.
- Notes: Commit 690609e touched only config_test.go (181 insertions) plus workflow bookkeeping — zero production lines, confirming the cycle converged clean (no coupling found, no accessor fix required). The accessors are structurally independent by construction: AICommandFor reads only c.Release.AICommand / c.Commit.AICommand / c.AICommand / DefaultAICommand; TimeoutFor reads only the *time.Duration timeout fields. They share no field, so there is no path for one to perturb the other — the tests confirm this empirically rather than relying on inspection.

TESTS:
- Status: Adequate
- Coverage: All six planned test scenarios are present and map 1:1 to the acceptance criteria:
  * TestResolution_OverrideCommandOnly_LeavesTimeoutAtFloor (config_test.go:2586) — table over VerbRelease + VerbCommit; asserts the command override wins AND TimeoutFor stays at the 60s floor (non-nil pointer, *got == DefaultTimeout). Covers Cases A and C.
  * TestResolution_OverrideTimeoutOnly_LeavesCommandAtFloor (:2635) — table over both verbs; asserts TimeoutFor == 120s AND AICommandFor == DefaultAICommand floor. Covers Cases B and D.
  * TestResolution_PerVerbCommandOverrideWithSharedTimeout_EachKeyOwnChain (:2684) — the combined proof: per-verb command override + shared top-level timeout=90; command resolves to the per-verb value (first layer), timeout to the shared value (second layer), proving neither chain leaks and the shared layer is consulted independently per key. Both verbs.
  * TestResolution_ReleaseOverride_DoesNotPerturbCommitResolution (:2734) — cross-verb isolation: [release] overrides both keys; both commit keys fall to their floors; includes a sanity assertion that the release override itself still resolves (so the check proves isolation, not a no-op config).
- Both directions, both verbs are covered. The combined-and-shared case proves the second-layer (shared) is consulted per-key independently — a property none of the 1-4/1-7 single-accessor tests exercise.
- Would-fail-if-broken: yes. If AICommandFor accidentally read a timeout field (or vice-versa), or if the per-verb selector keyed off the wrong table, at least one of these assertions on the *other* key's resolved value would fail. The combined case would also catch a regression where overriding one key short-circuits the other's shared-layer consultation.
- Not over-tested / non-redundant with 1-4 and 1-7: the prior tests exercise each accessor in ISOLATION (e.g. TestAICommandFor_ReadsOnlyAICommandCandidates at :1687 proves AICommandFor preserves a raw padded value; TestTimeoutFor_AbsentPerVerb_FallsToSharedThenFloor at :2434 walks TimeoutFor's own chain). None of them call BOTH accessors against a single-key-override config to assert the untouched key is unperturbed. The 1-8 tests are the first combined cross-key proof — a distinct property, not a re-run of 1-4/1-7. The table cases are minimal (two rows: one per verb) with no redundant assertions.

CODE QUALITY:
- Project conventions: Followed. External config_test package; t.Parallel() at function and subtest level; t.TempDir() roots; table-driven with named subtests; exact-value assertions (CLAUDE.md "assert exact ... rendered lines"); pointer-nil guarded before deref; comparisons against the exported config.DefaultTimeout / config.DefaultAICommand constants rather than re-typed literals (so the tests track the canonical source).
- SOLID principles: Good (tests; SRP — each function proves one independence property).
- Complexity: Low. Straight table loops, single concern per case.
- Modern idioms: Yes. Idiomatic Go table tests; verb iteration via []config.Verb where shared.
- Readability: Good. Each test carries a WHY-comment naming the exact independence property and the layer each key is expected to resolve through.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/config/config_test.go:2684 — the combined case fixes the override command to "haiku-cmd" and the shared timeout to 90s for both verbs; a symmetric combined case (override the per-verb TIMEOUT while only a SHARED ai_command is set, asserting timeout→per-verb and command→shared) would mirror the proof in the opposite direction. The existing four functions already cover both directions across the matrix, so this is optional symmetry, not a gap — decide whether the extra mirror earns its keep.
