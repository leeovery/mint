TASK: cli-presentation-3-2 — Stdin-TTY detection independent of stdout render-mode detection

ACCEPTANCE CRITERIA:
- Stdin-TTY detection uses the same IsTerminal primitive applied to the stdin descriptor — no second/duplicate TTY mechanism.
- The stdin-interactive signal is a distinct value from the stdout-derived Mode; gating path consumes the stdin signal, never the stdout signal.
- All four combinations independently representable.
- No environment variable read in the stdin-detection or gating-decision path.
- Both signals resolved once at startup; neither derived from the other.

STATUS: Complete

SPEC CONTEXT: "Scope & Output Modes", "Gating & -y Orthogonality", "Render-Mode Detection & Output Streams" — three orthogonal axes; gating = input (stdin TTY), render = output (stdout TTY); term.IsTerminal with hard ban on env sniffing; forbidden-combination rule depends on a stdin signal distinct from the stdout-derived Mode.

IMPLEMENTATION:
- Status: Implemented
- Location: gating.go:25 (StdinIsInteractive pure core), :34 (DetectStdinTTY reuses IsTerminal), :44 (StartupSignals struct, two distinct fields), :64 (DetectStartupSignals resolves both once); mode.go:62 (shared IsTerminal primitive); wiring.go:64 (NewForStartup threads Mode onto render path, StdinInteractive onto gating field).
- Notes: AC1 same primitive, only descriptor differs (grep confirms IsTerminal sole TTY call). AC2 StdinInteractive separate field, no cross-feed. AC3 four combos exercised (TestAxesAreIndependent). AC4 no env reads (grep Getenv/LookupEnv/os.Environ returns nothing); orthogonality+no-sniffing comment present. AC5 resolved once, neither derived; NewForStartup single production site.

TESTS:
- Status: Adequate
- Coverage (gating_test.go): StdinIsInteractiveMirrorsSignal (:14), AxesAreIndependent (:40, four combos), StdinIsInteractiveIgnoresEnvironment (:96), DetectStdinTTYOnNonTTY (:115), DetectStdinTTYIgnoresEnvironment (:130), DetectStartupSignalsResolvesBothAxesIndependently (:155); integration in wiring_test.go (:499, :540) proves the stdin signal reaches the forbidden-combination/gating consumer.
- Notes: Black-box, table-driven, t.Parallel, deterministic /dev/null. Both no-sniff paths (core + wiring) justified, not redundant.

CODE QUALITY:
- Project conventions: Followed — golang-testing conventions; pure-core-plus-thin-wiring mirrors Phase 1 SelectMode/DetectMode.
- SOLID principles: Good — single responsibility, pure core decoupled from I/O, descriptors injected.
- Complexity: Low.
- Modern idioms: Yes — *os.File injection, struct-of-signals, t.Cleanup.
- Readability: Good — doc comments state orthogonality + no-sniffing invariants.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
