TASK: cli-presentation-7-2 — Converge the startup wiring so one entry point consumes StartupSignals and threads all axes (Mode, terminal width, -y, StdinInteractive)

ACCEPTANCE CRITERIA:
- One production startup entry point consumes StartupSignals and threads Mode, width, -y, StdinInteractive; StartupSignals no longer referenced only by its own test.
- -y is a parameter into the startup seam; stdin-interactive is detected from the stdin descriptor, not derived from Mode.
- A presenter built through the converged seam with a non-interactive stdin handle and -y unset reaches the forbidden-combination fail-loud path.
- All wiring stays inside internal/presenter; no main/cmd package introduced.
- stdin/stdout/stderr handles are parameters; no os globals reached internally; unit-testable with non-TTY handles.

STATUS: Complete

SPEC CONTEXT: "Gating & -y Orthogonality" (spec:82-98) — render axis (stdout TTY) and gating axis (stdin TTY) orthogonal; fail loud (spec:98) when stdin non-TTY and -y unset, surfaced through Presenter and to stderr. This task makes one production seam thread both axes so that path is reachable from production wiring.

IMPLEMENTATION:
- Status: Implemented
- Location: wiring.go:64-75 (NewForStartup(plainFlag, yes bool, stdout, stderr, stdin *os.File) — consumes DetectStartupSignals :65, branches on signals.Mode, threads WithTermWidth(detectTermWidth(stdout)) on pretty, WithYes(yes) + WithInteractiveStdin(signals.StdinInteractive) on both); gating.go:64-69 (DetectStartupSignals, now consumed in production); plain.go:345-375 + pretty.go:745-776 (Prompt yes-first then !stdinInteractive → failNotInteractive → ErrNotInteractive + summary to out and err).
- Notes: -y a plain bool threaded only, no derivation from Mode. stdin-interactive from signals.StdinInteractive (stdout never feeds it). All three handles *os.File parameters; no os globals in NewForStartup. New(mode, out, err) kept as lower-level seam. No main/cmd package (grep confirms no production caller outside package). Deferral comments trimmed.

TESTS:
- Status: Adequate
- Coverage: wiring_test.go:475 (mode + writer wiring), :499 (AC#3 non-TTY stdin + -y unset → errors.Is ErrNotInteractive, summary on err via os.Pipe — proves threading), :540 (-y true auto-confirms, proves WithYes threaded + precedes stdin check); gating_test.go:40-90, :155-173 (four stdout/stdin combos at detector level, neither derived).
- Notes: Real char devices not synthesisable in CI; the two non-TTY-stdout combos exercised through the seam, independence pinned at detector level + seam threads verbatim. Idiomatic split, acceptable substitute. Focused, not over-tested.

CODE QUALITY:
- Project conventions: Followed — *os.File parameters, OS probes isolated from pure logic, builder setters return concrete type.
- SOLID principles: Good — single converged construction site; New kept as lower-level seam.
- Complexity: Low.
- Modern idioms: Yes — min() builtin, errors.Is sentinel, *os.File as io.Writer.
- Readability: Good — axis orthogonality explicit.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES:
- [idea] internal/presenter/wiring_test.go:469-530 — the "all four TTY combinations against the seam" bullet is satisfied indirectly (detector-level independence + seam threading) because real TTYs aren't CI-creatable. If a pty-based test helper is ever introduced, consider one NewForStartup case driving a real TTY stdout/stdin end-to-end; decide whether the pty dependency is worth it.
