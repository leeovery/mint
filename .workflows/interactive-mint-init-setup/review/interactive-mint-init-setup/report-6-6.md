TASK: interactive-mint-init-setup-6-6 — Add an end-to-end run("setup") dispatch test

ACCEPTANCE CRITERIA:
- A test drives run([]string{"setup"}) end to end and asserts stdout carries the emitted guide and the exit code is 0.
- The test would fail if the commandSetup arm's stdout/stderr arguments were swapped or the route were mis-wired.
- The test does not re-test runSetup's internal rendering already covered elsewhere.

STATUS: Complete

SPEC CONTEXT:
The spec (specification.md) defines `mint setup` as a pure string emitter: a thin top-level verb that prints the embedded AI setup guide to stdout, runs unconditionally (no git/cwd guard), and threads through the existing curated-help surface. The feature is built from three independently-tested seams — classifyCommand routing ("setup" -> commandSetup, dispatch_test.go:80-91), the runSetup emitter (setup_test.go:16-29), and the run() switch arm (main.go:105-108). The analysis-cycle-2 finding (analysis-tasks-c2.md Task 6) flagged that nothing exercised the COMPOSITION of these three — an argument swap (stdout/stderr) or a mis-wired route would pass every existing test. The remediation adds a single run()-level dispatch test in the spirit of the existing TestRun_TopLevelHelp_ExitsZero.

IMPLEMENTATION:
- Status: Implemented
- Location: cmd/mint/usage_test.go:91-105 (TestRun_Setup_EmitsGuideToStdoutAndExitsZero) + the captureStdStreams helper at cmd/mint/usage_test.go:113-143.
- Notes: The test calls run([]string{"setup"}) (the FULL dispatch entry, not runSetup directly), redirecting the process-global os.Stdout/os.Stderr onto separate pipes so it can prove WHICH stream the output landed on. It asserts exit code 0, that stdout contains the stable exported marker setupguide.MarkerPipeline, and — critically — that stderr does NOT contain that marker. This is exactly the composition proof the finding requested.

  Wiring under test (main.go:105-108): the commandSetup arm calls `runSetup(rest, os.Stdout, os.Stderr)`. Because run() writes the guide straight to the process os.Stdout (not an injected writer), a run()-level test cannot inject buffers — hence the os.Pipe redirection via captureStdStreams. The helper restores the originals via t.Cleanup and drains both pipes. Correctly NOT t.Parallel() (documented at usage_test.go:90) because it swaps the process-global descriptors.

TESTS:
- Status: Adequate
- Coverage:
  (a) End-to-end run("setup") to stdout, asserting guide-on-stdout + exit 0 — COVERED (usage_test.go:94-101).
  (b) Composition proof catching a stdout/stderr arg swap or mis-wired route — COVERED. The separate-pipe capture is the key design choice: if the commandSetup arm's two writer args were swapped, the guide would land on the stderr pipe; the stdout-contains-marker assertion (line 99) would fail AND the stderr-does-NOT-contain-marker assertion (line 102) would fail. A mis-wired route (e.g. commandSetup falling through to default/unknown) would yield exit 2 and no marker on stdout, failing lines 94-101. The classifyCommand route, the switch arm, and runSetup are all exercised in one call. Verified the bite is real: classifyCommand maps "setup" -> commandSetup (main.go:434-436), and the switch arm is the only producer of the guide on this path.
  (c) Does NOT duplicate emitter internals — CONFIRMED. It asserts a single stable marker (setupguide.MarkerPipeline), not byte-for-byte guide content. The byte-identical content contract is held by setup_test.go:16-29 (TestRunSetupEmitsGuideAndExitsZero, `out.String() != setupguide.Guide()`). Clean separation: composition here, content there.
- Notes: Marker is an exported, stable, greppable constant in the documented `<!-- mint:section:NAME -->` namespace (setupguide.go:45-60), specifically designed so prose changes never break it and the section's removal always does — an ideal anchor for a composition assertion (resilient to prose drift, still bites on a missing section). No under-testing (route + arm + emitter + stream-direction all proven) and no over-testing (one marker, not the full body; relies on setup_test.go for content).

CODE QUALITY:
- Project conventions: Followed. No real git/claude/editor is spawned — correct, because the setup path constructs no runner and resolves no repo (main.go:106-107 comment; setup.go:23-34), so the FakeRunner/RecordingPresenter idiom does not apply here (there is no subprocess or presenter seam on this path to fake). The test uses os.Pipe to capture the process-global descriptors, which is the only faithful way to test run()'s direct os.Stdout/os.Stderr writes; this matches the suite's existing run()-level idiom (TestRun_TopLevelHelp_ExitsZero) and the no-os.Exit testability noted in main.go:44-48.
- SOLID principles: Good. captureStdStreams is a single-responsibility test helper, reusable, with a clear doc comment explaining WHY it exists (run writes straight to os.Stdout, not an injected writer).
- Complexity: Low. The test body is a linear call-then-assert; the helper's drain closures are straightforward.
- Modern idioms: Yes. t.Helper(), t.Cleanup() for restoration, io.Copy to drain, strings.Builder. Errors from os.Pipe/io.Copy are surfaced via t.Fatalf rather than ignored.
- Readability: Good. The test and helper carry thorough WHY-comments matching the codebase's heavy-comment discipline (CLAUDE.md), including the explicit rationale for NOT using t.Parallel() and for the separate-pipe capture biting on an arg swap.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None. The test meets all three acceptance criteria, holds the FakeRunner/RecordingPresenter idiom (n/a on a seamless path) by spawning nothing real, and the separate-stream capture genuinely bites on an arg-order swap as required.
