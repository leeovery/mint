TASK: interactive-mint-init-setup-2-3 — Wire the commandSetup dispatch route and the runSetup runner

ACCEPTANCE CRITERIA (from plan):
- [ ] commandKind gains a commandSetup constant with a WHY-comment noting it runs unconditionally (no repo / no git rev-parse).
- [ ] classifyCommand([]string{"setup", ...}) returns commandSetup with remaining args; classifyCommand stays pure.
- [ ] The run switch routes commandSetup to runSetup; mint setup emits the guide to stdout and exits 0 from any directory with no repo-root resolution.
- [ ] mint setup --help prints setupUsage to stdout and exits 0 (via flag.ErrHelp), NOT usage-error exit 2.
- [ ] An unrecognised flag to mint setup is a usage error (exit 2, message on stderr).
- [ ] The unknown-command default message in run lists mint setup among the wired commands.
- [ ] runSetup performs no IO beyond writing to stdout (no git, no config, no repo-root resolution).
- [ ] All standard gates pass.

STATUS: Complete

SPEC CONTEXT:
Spec "The mint setup subcommand" → "Runs unconditionally — no git/cwd guard" decides mint setup is a pure text emitter that must print even outside a work tree (setup instructions are read before cd-ing in); safety lives in the emitted instructions, and mint init remains the loud-fail backstop. This is the explicit divergence from runInit's `git rev-parse --show-toplevel`. Spec "Help-surface wiring" decides dispatch is a new commandSetup route in classifyCommand/run, with mint setup --help routed to stdout/exit 0 via flag.ErrHelp. Emission-surface decision (flagged in Tasks 2-1/2-3): option A, a cmd-layer fmt.Fprint to stdout, consistent with the curated-help/usage cmd-layer writes that are the documented exception to CLAUDE.md seam 3 — not a presenter.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - cmd/mint/main.go:406-412 — commandSetup enum constant with a heavy-WHY comment stating it drives no gate, calls no RunFinished, needs no git repo, never resolves the repo root / issues no git rev-parse, and that the cwd-confirm safety lives in the emitted guide. Matches AC1.
  - cmd/mint/main.go:434-436 — classifyCommand setup branch (`if args[0] == "setup" { return commandSetup, args[1:] }`), placed alongside init/version/commit before the release check. classifyCommand stays pure (no execution, no parsing). Matches AC2.
  - cmd/mint/main.go:105-108 — run switch case commandSetup dispatching to runSetup(rest, os.Stdout, os.Stderr), with a comment noting it takes only IO descriptors and no ctx (like version). Matches AC3.
  - cmd/mint/main.go:118 — unknownCommandMessage const now enumerates `mint setup` among the wired commands. Matches AC6.
  - cmd/mint/setup.go:17-21 — parseSetupFlags: flag.NewFlagSet("setup", ContinueOnError) + fs.SetOutput(io.Discard); no flags registered so -h/--help surfaces flag.ErrHelp and any other flag is a parse error.
  - cmd/mint/setup.go:35-48 — runSetup: on flag.ErrHelp prints setupUsage to stdout, returns 0; on any other parse error prints "mint: %v" to stderr, returns usageExitCode; otherwise writes setupguide.Guide() to stdout, returns 0. No gitrepo.ResolveRoot, no runner, no config load, no presenter. Matches AC3/AC4/AC5/AC7.
- Notes:
  - Signature divergence from plan recommendation (NOT a defect — an improvement): the plan recommended `runSetup(rest []string) int` writing to os.Stdout. The implementation chose `runSetup(args []string, stdout, stderr io.Writer) int`. This is the better call: injecting io.Writer lets setup_test.go assert on in-memory buffers with t.Parallel(), where runVersion's *os.File signature forces the process-global-stream capture dance. The run() switch passes os.Stdout/os.Stderr so production behaviour is identical to the recommendation. Aligned with the spirit of the seam-3 exception (cmd-layer stdout write).
  - Emission surface: option A honoured (cmd/mint/setup.go:46 fmt.Fprint(stdout, setupguide.Guide())). No presenter, no os/exec, no git. The choice is documented inline at setup.go:31-34. Conforms to the flagged spec decision and to CLAUDE.md seam-3's documented cmd-layer-help exception.
  - Verified (a) classifyCommand + run route commandSetup; (b) runSetup emits and runs anywhere with no repo resolution (no gitrepo import in setup.go, no runner constructed); (c) --help → flag.ErrHelp → stdout/exit 0; (d) unknown-command message mentions setup; (e) tests prove these without spawning git.

TESTS:
- Status: Adequate
- Coverage:
  - cmd/mint/dispatch_test.go:80-91 — classifyCommand("setup") → commandSetup with empty rest; "setup --help" carries remaining args through. (AC2; plan test "it classifies setup as commandSetup with the remaining args".)
  - cmd/mint/setup_test.go:16-29 TestRunSetupEmitsGuideAndExitsZero — runSetup writes setupguide.Guide() byte-for-byte to stdout, nothing to stderr, exit 0. (AC3; plan test "it emits the guide to stdout and exits 0".)
  - cmd/mint/setup_test.go:36-47 TestRunSetupRunsAnywhere_NoRepoResolution — driven from a fresh t.TempDir() that is deliberately NOT git-init-ed; still emits and exits 0. Proves the no-repo-resolution divergence behaviourally. (AC3/AC7 edge case; plan test "it runs mint setup anywhere".)
  - cmd/mint/setup_test.go:52-67 TestRunSetupHelpExitsZero — both -h and --help print setupUsage to stdout, exit 0, nothing on stderr. (AC4; plan test "it exits 0 (not 2) for mint setup --help".)
  - cmd/mint/setup_test.go:72-85 TestRunSetupUnknownFlagIsUsageError — --nope exits usageExitCode, diagnostic on stderr, nothing on stdout. (AC5; plan test "it exits with the usage error code for an unrecognised flag".)
  - cmd/mint/setup_test.go:90-98 TestParseSetupFlagsHelpSurfacesErrHelp — parser surfaces flag.ErrHelp for -h/--help.
  - cmd/mint/setup_test.go:103-109 TestRunListsSetupInUnknownCommandMessage — unknownCommandMessage contains "mint setup". (AC6; plan test "it lists mint setup in the unknown-command message".)
  - cmd/mint/usage_test.go:91-105 TestRun_Setup_EmitsGuideToStdoutAndExitsZero — full run([]string{"setup"}) dispatch end-to-end on separate stdout/stderr pipes; asserts the guide marker lands on stdout (not stderr), exit 0. This is the seam test the plan's Do requested ("run([]string{"setup"}) exits 0 and the unknown-command path is not hit") and additionally guards against the arg-order swap (guide on stderr).
- Notes:
  - No real git/claude/editor spawned anywhere — runSetup constructs no runner, and the anywhere test relies on t.Chdir to a non-repo temp dir, so the no-repo property is proved by absence of failure rather than by mocking. Conforms to the "tests never spawn real git" idiom.
  - Marker-keyed assertion in the run-level test (setupguide.MarkerPipeline) avoids fragile prose coupling — good.
  - Slight under-coverage (non-blocking): the plan's Do mentioned a run-level `run([]string{"setup", "extra-unknown-flag"})` → usageExitCode assertion. The unknown-flag → usageExitCode behaviour IS covered at the runner level (TestRunSetupUnknownFlagIsUsageError) and the run→runSetup seam is covered by the happy-path run() test, so the error path through run() is not independently pinned. The runner-level coverage plus the proven seam make this adequate; an explicit run-level usage-error assertion would be belt-and-braces, not a gap.
  - Not over-tested: each test pins one distinct behaviour; the -h/--help loop and the dispatch table are the only multiplicities and both cover genuinely distinct inputs.

CODE QUALITY:
- Project conventions: Followed. cmd-layer stays thin (flag parse + dispatch + exit-code mapping, no business logic). The pure string lives in internal/setupguide; the cmd layer only writes it. Mirrors runInit/runVersion idioms. External-package tests where they were already external (dispatch_test, usage_test use package main as the existing files do; setup_test is package main matching the cmd convention for run-level tests). Lowercase error message ("mint: %v") consistent with the cmd register. No os/exec, no presenter, no git in runSetup — seam 1/2/3 respected (seam 3 via its documented cmd-layer-help exception, explicitly chosen and commented).
- SOLID principles: Good. Single responsibility — parseSetupFlags parses, runSetup orchestrates the emit, setupguide.Guide() produces. Dependency inversion via io.Writer injection.
- Complexity: Low. runSetup is one parse + a two-branch error split + the emit. classifyCommand gains one pure string branch.
- Modern idioms: Yes. errors.Is(err, flag.ErrHelp), io.Discard for the flag dump, io.Writer injection.
- Readability: Good. The WHY-comments (main.go:406-412 enum doc, setup.go:23-34 runSetup doc) state the no-repo divergence and the seam-3 emission-surface choice as contracts, true to as-built — matching the codebase's heavy-WHY comment discipline.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] cmd/mint/usage_test.go (TestRun-level error path) — add a run([]string{"setup", "--nope"}) assertion that the full dispatch returns usageExitCode and writes the diagnostic to stderr, mirroring the existing happy-path run-level seam test. The behaviour is proven at the runner level (setup_test.go:72) but not through the run() switch; a one-case addition closes the symmetry the plan's Do sketched.
