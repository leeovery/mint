TASK: cli-presentation-7-1 — Consolidate the prompt/gate test drivers onto the shared gate_helpers_test.go construction seam (tick-4831fb)

ACCEPTANCE CRITERIA:
- Only one "drive one Prompt across both modes" mode-driver table remains; the two redundant ones removed.
- Exactly one pretty-prompt single-Prompt driver; drivePrettyPrompt no longer exists as an independent Ascii-pinned reimplementation.
- The remaining prompt drivers build via the plainGate/prettyGate seam, not inline construction.
- The forced colour profile is an explicit parameter; the render-only screen-control guard still runs under TrueColor.
- No production (.go non-test) file in internal/presenter is modified.

STATUS: Complete

SPEC CONTEXT: Phase 7 test-quality cleanup (source: duplication). Underlying gate/Prompt render-only contract established in Phases 1-6. Sole job: collapse copy-paste driver drift without losing asserted properties. Production out of scope.

IMPLEMENTATION:
- Status: Implemented
- Location: gate_helpers_test.go:54 (plainGate), :68 (prettyGate), :104 (gateDriver), :119 (gateDrivers) — single canonical seam + table; pretty_gate_test.go:20 (drivePrettyPromptProfile, single pretty-prompt driver on prettyGate); prompt_test.go:23,51,74,97,116,136,159,199,218 (consume gateDrivers()/d.prompt; hint test builds plain via plainGate); prompt_render_only_test.go:108,126,162,215,259,278.
- Notes: Grep for promptModeDriver(s)/promptDriver(s)/drivePlainPrompt/drivePrettyPrompt returns zero. Exactly one mode-driver table type (gateDriver). drivePrettySpy/drivePretty are unrelated render helpers. Colour profile explicit at every layer. Production untouched — all construction APIs already existed.

TESTS:
- Status: Adequate (test code; assessment = coverage preserved, seam sound)
- Coverage: every mode-invariant prompt property still runs against both presenters via gateDrivers(); mode-specific rendering stays in dedicated tests (plain hint, pretty menu/[default]/downgrade); screen-control guard runs pretty arm under TrueColor (assertNoScreenControl with SGR present).
- Notes: No coverage lost (pure seam swap). Explicit profile parameter eliminates prior silent drift. Read-only assessed.

CODE QUALITY:
- Project conventions: Followed — *_helpers_test.go layout, named-field gateOpts, doc comments.
- SOLID principles: Good — single construction seam; gateDriver composes; thin specialisation.
- Complexity: Low.
- Modern idioms: Yes — table-driven subtests, io.Reader injection, functional options.
- Readability: Good.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
