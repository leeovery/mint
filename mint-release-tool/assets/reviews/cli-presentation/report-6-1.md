TASK: cli-presentation-6-1 — Extract shared gate-test construction+capture helpers

ACCEPTANCE CRITERIA:
- One shared helper file provides plainGate/prettyGate (or equivalently named) constructors parameterised by the -y/interactive-stdin toggles.
- gate_skip_test.go, gate_forbidden_test.go, gate_sourcetarget_test.go no longer hand-inline the buffer-allocation + presenter-construction idiom; they call the helpers.
- No behavioural change to any test assertion; the arming toggles are passed explicitly at every call site.

STATUS: Complete

SPEC CONTEXT: Test-only refactor. Spec §Gating (spec:84-116,212,227,247) defines the gate behaviour the three files exercise (-y skip+echo, forbidden-combination fail-loud with ErrNotInteractive). The refactor preserves all rendered-string assertions verbatim.

IMPLEMENTATION:
- Status: Implemented
- Location: gate_helpers_test.go (new, package presenter_test) — gateOpts struct (:22-46), plainGate (:54-60), prettyGate (:68-79), gateResult/gateDriver/gateDrivers table (:86-138); gate_skip_test.go, gate_forbidden_test.go, gate_sourcetarget_test.go all sites converted.
- Notes: Named gate_helpers_test.go (not the literal gate_test_helpers.go) — a correct deviation since a plain .go file declaring package presenter_test isn't permitted; acceptance allows "equivalently named". Grep confirms no residual inline construction in the three files (only golden_transcript_test.go:123-125 remains, out of scope). Arming toggles explicit gateOpts fields at every site (zero value = default). prettyGate always wires WithErr(errBuf) so stream-split stays assertable.

TESTS:
- Status: Adequate (test infrastructure; adequacy = migrated tests compile and assert same behaviour)
- Coverage: all skip/forbidden/source-target assertions retained byte-for-byte; only construction/capture lines moved into helpers. gateDrivers() consumed by table-driven mode-invariant tests (gate_forbidden_test.go:162,247).
- Notes: No shell; judged by reading — mechanical migration, assertions preserved.

CODE QUALITY:
- Project conventions: Followed — external test package, helper in _test.go, table-driven drivers, lower-camel names.
- SOLID principles: Good — single construction seam; applyToPlain/applyToPretty isolate toggle-threading.
- Complexity: Low.
- Modern idioms: Yes — options-as-struct with zero-value-default.
- Readability: Good (verbose but accurate doc comments).
- Issues: None blocking.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES:
- [quickfix] internal/presenter/gate_helpers_test.go:113-115 — gateDriver.prompt convenience method appears unused by the gate-* files. Confirm it has a consumer; if none, remove it to drop dead test-helper surface.
- [idea] internal/presenter/golden_transcript_test.go:123-125 — the NewPlainPresenterWithInput(...).WithYes(true) construction idiom survives here, outside this task's three-file scope. Decide whether a future pass should route this single full-transcript driver through plainGate or leave it standalone.
