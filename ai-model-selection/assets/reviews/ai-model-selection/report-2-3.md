TASK: ai-model-selection-2-3 — Thread resolved command + timeout through the release wiring site

ACCEPTANCE CRITERIA:
- `aiTransport` constructs `ai.Config` with `AICommand: cfg.AICommandFor(config.VerbRelease)` and `Timeout: cfg.TimeoutFor(config.VerbRelease)`.
- A `[release].ai_command` override drives the release notes AI invocation's binary+args (not the bare shared/default).
- A zero-config run still resolves to `claude -p --model sonnet` for release notes.
- The release timeout is sourced from `cfg.TimeoutFor(VerbRelease)` (never zero-by-omission).
- The `aiTransport` WHY-comment (and the related seam comment) no longer claim the transport re-defaults an empty value to `claude -p`.
- Gates pass (build/gofmt/vet/test -race/golangci-lint).

STATUS: Complete

SPEC CONTEXT:
specification.md "Migration & mechanical carry-overs" lists the three transport-wiring sites that must thread BOTH the per-verb command and timeout; the config schema defines per-verb `ai_command`/`timeout` with resolution order `[verb] → shared → floor`. The release notes call must honour a `[release]` override and never leave Timeout zero-by-omission. Phase 4 task 4-1 later consolidates the three sites into a shared `internal/aitransport.New(r, cfg, verb)` helper.

IMPLEMENTATION:
- Status: Implemented (and superseded by the Phase 4 consolidation, correctly)
- Location:
  - internal/engine/release.go:933-938 — `aiTransport` returns `aitransport.New(deps.Runner, cfg, config.VerbRelease)` for the production branch; the `deps.Transport != nil` test-seam short-circuit is preserved unchanged (release.go:934-936).
  - internal/aitransport/aitransport.go:40-45 — `New` constructs `ai.Config{AICommand: cfg.AICommandFor(verb), Timeout: cfg.TimeoutFor(verb)}`. With `VerbRelease` passed, this is exactly the task's required expression.
  - internal/config/config.go:596-624 — `TimeoutFor` returns a non-nil, positive `*time.Duration` floor (DefaultTimeout = 60s) when nothing overrides, so the field can never be nil/zero-by-omission through this path; explicit `0` is honoured as "no deadline". Returned type is `*time.Duration`, assigned directly to `ai.Config.Timeout` with no conversion (per the task note on Task 2-2's type change).
  - internal/config/config.go:87 — DefaultAICommand = "claude -p --model sonnet"; AICommandFor floors to it on a zero-config run.
- Notes: The task targeted the inline `ai.NewTransport(...)` call, but Phase 4 task 4-1 already routed this site through the shared `aitransport.New` helper with `config.VerbRelease`. The release verb constant is correctly used. The acceptance criteria are satisfied via the helper — the contract lives in one place, which is the intended Phase 4 end state. No drift.

COMMENTS (WHY-comment carry-over):
- internal/engine/release.go:923-932 (`aiTransport` doc) — rewritten: states production goes through `aitransport.New` with `config.VerbRelease`, the `[release] → shared → floor` chain, and the never-zero-by-omission timeout contract. No "re-defaults an empty value to `claude -p`" / "whitespace-splits" claim remains.
- internal/engine/release.go:132-141 (`Transport` seam doc) — updated to "the release verb's resolved command + timeout (cfg.AICommandFor / cfg.TimeoutFor over the `[release] → shared → floor` chain) — config owns the default and the blank-skip / no-deadline semantics." The stale claim is gone.
- Repo-wide grep for `re-default` / `whitespace-split` in release.go returns nothing — no stale claim left behind.

TESTS:
- Status: Adequate (well-balanced; behaviour-level end-to-end plus focused structural proofs, no redundancy)
- Coverage:
  - internal/engine/release_configconsolidation_test.go:
    - TestRelease_AICommand_ReleaseVerbOverrideDrivesTransport (:74) — a `[release].ai_command` override drives the argv over the real prior-tag normal-AI path; asserts `stdinOf(..., "mybot", "gen", "--json")` non-empty AND that neither the shared `sharedbot gen` nor the `claude` default was invoked. Proves per-verb override WINS (the criterion the task's first test asked for).
    - TestRelease_AICommand_NoReleaseOverrideFallsToShared (:112) — no `[release]` override → shared `mybot gen` drives the call (the second requested test).
    - TestRelease_AICommand_DefaultDrivesTransport (:142) — zero-config run invokes `claude -p --model sonnet` with the composed prompt on stdin (the migrated Task 2-6 default-argv assertion, coordinated to the new pinned floor).
    - TestRelease_AICommand_ConfigValueDrivesTransport (:31) — shared top-level key drives the transport (the original mirror retained, still valid).
  - internal/engine/release_aitransport_internal_test.go (white-box, package engine):
    - TestAITransport_SourcesCommandFromReleaseAccessor (:24) — `aiTransport` directly, asserts the `[release].ai_command` override binary+args reach the runner via DeadlineRecordingRunner.
    - TestAITransport_ExplicitZeroTimeoutThreadsNoDeadline (:56) — `[release].timeout = 0` threads a deadline-free context (the focused timeout-at-the-seam proof the task preferred for determinism). The comment correctly notes reaching Generate at all proves the field was threaded (NewTransport panics on nil), and no-deadline proves it came from the accessor.
    - TestAITransport_PositiveTimeoutThreadsDeadline (:81) — a positive resolved timeout threads a real per-attempt deadline.
  - internal/aitransport/aitransport_test.go: TestNew_SourcesCommandAndDeadlineFromVerbAccessor (:28) — table-driven across both verbs, proving the shared helper sources command + deadline from the per-verb accessor (covers the VerbRelease row this site relies on).
- Notes: Every acceptance criterion has a matching test. Timeout is proven both ways (explicit-0 → no deadline; positive → deadline), which is the right pair, not over-testing. The default-argv assertion exactly matches the new floor `claude -p --model sonnet`, coordinated with Task 2-6 as the task instructed. Tests assert exact argv and exact behaviour per the project's test idioms. No redundant or implementation-detail-only assertions; the white-box internal tests are justified (timeout is not observable on the command line, so the DeadlineRecordingRunner seam is necessary).

CODE QUALITY:
- Project conventions: Followed. AI goes through the consumer-facing transport; the helper imports ai+config+runner with no cycle (config does not import ai). The `deps.Transport` test seam is preserved (CLAUDE.md seam #5 / test idioms). WHY-comments kept true to as-built (CLAUDE.md "Comments" rule) — the now-false claims were removed in the same change.
- SOLID principles: Good. Single construction expression centralised in `aitransport.New`; `aiTransport` retains only the site-specific injected-seam short-circuit (the deps wrapper type differs per site), which correctly stays local.
- Complexity: Low. `aiTransport` is a two-line branch; `New` is a single constructor.
- Modern idioms: Yes. Direct `*time.Duration` threading, closed `config.Verb` enum, value-semantics resolution loop in TimeoutFor.
- Readability: Good. Comments state the contract and the resolution chain precisely.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
