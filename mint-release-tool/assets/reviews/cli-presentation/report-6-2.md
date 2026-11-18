TASK: cli-presentation-6-2 — Consolidate mode-invariant both-arm assertions through the promptDrivers (now gateDrivers) table

ACCEPTANCE CRITERIA:
- Each consolidated mode-invariant property is asserted exactly once via a t.Run(d.mode, ...) loop rather than two copy-pasted arms.
- Mode-specific rendering assertions (exact plain hint vs pretty menu bytes) remain in their own tests and are not merged.
- The set of properties verified before and after is unchanged — no invariant dropped, none added.
- `go test ./internal/presenter/...` passes; both render modes covered per consolidated property; -run shows each invariant executes once per mode.

STATUS: Complete

SPEC CONTEXT: Spec governs presenter behaviour (render-only Prompt, line-read model, -y skip+echo, forbidden-combination, gate-declared sets) but is silent on test structure. Phase 6 test-quality refactor: removes hand-duplicated plain-then-pretty arms for mode-invariant properties, driving them once through the shared mode-driver table; mode-specific byte-exact tests untouched.

NAMING: Task names promptDrivers(); current tree has gateDrivers() in gate_helpers_test.go — intended end state (task 7-1 merged prompt+gate driver tables onto the single seam). Verified against post-7-1 tree; consolidation present and correct under unified name.

IMPLEMENTATION:
- Status: Implemented
- Location: gate_helpers_test.go:104-138 (gateDriver/gateDrivers table + prompt convenience + gateResult); prompt_test.go (mode-invariant properties driven once per mode: empty-Enter default :20, case-insensitive :39, unrecognised re-prompts :71, a/q re-prompt :94, whitespace :113, repeated-unrecognised :133, EOF :156, reuse-gate-declared-set :215; mode-specific hint check :237 kept plain-only); prompt_render_only_test.go:107-291 (edit/regen no-side-effect + engine-loop linear-render once per mode, TrueColor for screen-control guard); gate_forbidden_test.go:159-170,244-258 (ErrNotInteractive sentinel + default-stdin-interactive via table; byte-exact failure lines remain dedicated).
- Notes: gate_skip_test.go and gate_sourcetarget_test.go correctly no longer reference the table — their dual-arm tests assert mode-specific bytes (instructed to leave separate). No mode-invariant arm pair left hand-duplicated.

TESTS:
- Status: Adequate
- Coverage: deliverable IS test code. Every consolidated property asserted once per mode via named t.Run(d.mode,...) subtests (independently -run selectable). Both modes covered. Property set preserved (each former arm pair → one table body asserting same outcome). Mode-specific byte-exact rendering verified by untouched dedicated tests.
- Notes: Driver run-funcs route construction through plainGate/prettyGate (6-1 helpers) so arms can't drift. No over-testing. Black-box, behaviour-focused.

CODE QUALITY:
- Project conventions: Followed — table-driven named subtests, black-box package, deterministic profile selection.
- SOLID principles: Good — single shared driver/seam; mode-invariant vs mode-specific cleanly separated.
- Complexity: Low.
- Modern idioms: Yes — idiomatic table-driven subtests, explicit profile parameter.
- Readability: Good — each test documents why the property is mode-invariant.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
