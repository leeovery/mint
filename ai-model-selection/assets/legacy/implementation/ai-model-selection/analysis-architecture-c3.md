AGENT: architecture
FINDINGS:
- FINDING: DurationPtr test helper bundled into the deadline-recording-runner file, weakening cohesion
  SEVERITY: low
  FILES: internal/runner/deadline_recording_runner.go:64-67
  DESCRIPTION: DurationPtr (a generic *time.Duration constructor) ships in deadline_recording_runner.go,
    a file whose stated purpose is the cross-package CommandRunner spy. The two are unrelated: DurationPtr
    has nothing to do with recording a deadline on a runner — it is a generic pointer helper the config
    *time.Duration test call sites need. It was co-located here purely because this is the one production
    (non-_test.go) file shared across the aitransport/engine/commit test packages, but the file name and
    doc topic do not describe it. A reader searching for the duration helper would not look in a
    deadline-recording-runner file, and a future reader of this file is surprised to find a value helper
    sitting beside a CommandRunner implementation. This is a minor seam/cohesion nit, not a defect: the
    placement is correct in spirit (cross-package production-file home, mirroring FakeRunner), only the
    grouping is off.
  RECOMMENDATION: Leave the placement decision (production file, shared home) as-is, but consider a more
    neutrally-named host file (e.g. testsupport.go / ptr.go) for cross-package test helpers, or at minimum
    a file-level doc line acknowledging it carries an unrelated shared helper. Low priority — do not churn
    if it would ripple imports.

- FINDING: timeout has no TOML→transport end-to-end integration test at the engine/commit level (ai_command does)
  SEVERITY: low
  FILES: internal/engine/release_configconsolidation_test.go:31-161, internal/engine/release_aitransport_internal_test.go:56-98, internal/commit/run_aitransport_test.go:1-184
  DESCRIPTION: The ai_command key is proven end-to-end from a real .mint.toml through config.Load to the
    invoked argv (TestRelease_AICommand_ConfigValueDrivesTransport / the [release] and [commit] override
    variants write an actual config file and assert the resolved binary runs). The parallel timeout path
    has no equivalent: every timeout proof at the wiring level (aiTransport / commitTransport /
    resolveFreshTransport white-box tests) constructs a SYNTHETIC config.Config literal and never loads a
    TOML file, and the config_test.go TimeoutFor tests cover resolution but stop at the accessor return —
    no test threads `timeout = N` from a written .mint.toml all the way to a deadline-bearing (or
    deadline-free) context at the runner. The seam is covered link-by-link (config.Load→TimeoutFor proven
    in config; TimeoutFor→runner-deadline proven white-box), so the composition holds and this is not a
    correctness risk; it is an asymmetry in integration coverage relative to ai_command, which got the
    full chain. Worth a single end-to-end timeout assertion (e.g. a real-run release with `[release].timeout = 0`
    asserting the AI call's context carries no deadline) to make the cross-task wiring un-missable the way
    ai_command's is.
  RECOMMENDATION: Add one TOML-driven engine-or-commit integration test that writes a timeout key (an
    explicit 0 and/or a positive value) and asserts the deadline presence on the runner call (via
    DeadlineRecordingRunner threaded as deps.Runner), closing the asymmetry with ai_command's existing
    end-to-end proof. Optional — the per-link coverage already proves the chain.
SUMMARY: The architecture is sound: the dedicated aitransport package is a well-justified single home for
  the shared construction expression, the closed config.Verb enum makes regenerate routing un-missable by
  construction, and the *time.Duration boundary cleanly preserves absent-vs-explicit-zero with a fail-loud
  nil guard and a single contained polarity-mapping in NewTransport. The two findings are both low severity
  — a file-cohesion nit for a co-located helper and an integration-coverage asymmetry where timeout lacks
  ai_command's full TOML→transport proof — neither indicating a structural defect.
