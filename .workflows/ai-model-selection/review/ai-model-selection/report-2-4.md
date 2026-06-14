TASK: ai-model-selection-2-4 — Thread resolved command + timeout through the commit wiring site

ACCEPTANCE CRITERIA:
- commitTransport constructs ai.Config with AICommand: cfg.AICommandFor(config.VerbCommit) and Timeout: cfg.TimeoutFor(config.VerbCommit).
- A [commit].ai_command override drives the commit-message AI invocation's binary+args (not the bare shared/default).
- A zero-config mint commit still resolves to claude -p --model sonnet.
- The commit timeout is sourced from cfg.TimeoutFor(VerbCommit) (never left zero-by-omission).
- The commitTransport WHY-comment (and related comments at ~204/~707) no longer claim the transport re-defaults an empty value to claude -p.
- All gates pass.

STATUS: Complete

SPEC CONTEXT:
Spec "Migration & mechanical carry-overs (Transport wiring sites)" enumerates three sites that must thread BOTH the resolved per-verb command and timeout where today only ai.Config{AICommand: cfg.AICommand} is built (Timeout left zero); internal/commit/run.go is the commit site. "Resolution value semantics" + "config→ai.Config boundary" mandate the invariant: "no deadline" must only ever be reachable by an operator's explicit 0, never by a wiring site omitting the field; all three sites must source the timeout from the accessor. Per-verb [commit].ai_command must win over shared/floor (verb → shared → floor); zero-config resolves to the pinned claude -p --model sonnet + 60s floor. Cross-spec reconciliation requires the Commit struct doc comment be updated to the per-verb-override contract in the same change.

IMPLEMENTATION:
- Status: Implemented (convergent with Phase 4 task 4-1)
- Location: internal/commit/run.go:777-782 (commitTransport), :781 (aitransport.New(deps.Runner, cfg, config.VerbCommit)); helper internal/aitransport/aitransport.go:40-45; accessors internal/config/config.go:538 (AICommandFor), :596 (TimeoutFor).
- Notes: As-built, commitTransport delegates to the shared aitransport.New helper rather than inlining ai.NewTransport(...) as the task's "Do" steps literally described. This is the Phase 4 task 4-1 consolidation, which the task brief explicitly anticipated ("Phase 4 task 4-1 consolidates the three sites into a shared helper — check this site uses the commit verb"). The site correctly passes config.VerbCommit; the helper sources BOTH AICommandFor(verb) and TimeoutFor(verb) and assigns the *time.Duration directly to ai.Config.Timeout. Behaviour is identical to the inline form — convergent evolution, not drift.
- Zero-by-omission is structurally impossible: ai.Config.Timeout is *time.Duration (internal/ai/transport.go:66); NewTransport fails loud on nil; TimeoutFor never returns nil (60s floor). A forgotten field would surface as nil and panic, not silently run unbounded — the spec invariant holds.
- Comments true to as-built: commitTransport WHY-comment (run.go:768-776) describes per-verb resolution and the never-zero-by-omission contract; related comments at :202-209 (Deps.Transport) and :705-712 (generateMessage) reference cfg.AICommandFor(VerbCommit)/cfg.TimeoutFor(VerbCommit). No remaining "re-defaults to claude -p" / "whitespace-splits and re-defaults" / bare "cfg.AICommand drives it" wording in run.go (grep clean). Commit struct doc comment (config.go:230-251) encodes the per-verb override contract — cross-spec reconciliation done in-change.
- deps.Transport != nil test-seam short-circuit preserved (run.go:778-780).
- Mutate-nothing-until-accept invariant unaffected: commitTransport only constructs a transport (no git mutation); the only commit-path read here is the read-only git diff --cached; git add/commit remain in the accept tail.

TESTS:
- Status: Adequate
- Coverage:
  - run_aitransport_test.go (black-box, real transport over FakeRunner, deps.Transport nil):
    - TestRun_AICommand_CommitVerbOverrideDrivesTransport — [commit].ai_command="mybot gen --json" over shared "sharedbot gen"; asserts mybot gen --json invoked with prompt on stdin, sharedbot NOT invoked, claude -p --model sonnet NOT invoked, body reaches commit sink verbatim. Proves per-verb override wins (verb → shared → floor) AND exact argv.
    - TestRun_AICommand_NoCommitOverrideFallsToShared — shared-only ai_command="mybot gen"; asserts mybot gen drives, default not invoked.
    - TestRun_AICommand_DefaultDrivesTransport — zero-config; asserts claude -p --model sonnet invoked with prompt on stdin.
  - run_aitransport_internal_test.go (white-box, DeadlineRecordingRunner spy):
    - TestCommitTransport_SourcesCommandFromCommitAccessor — [commit].ai_command override binary+args asserted on the spy.
    - TestCommitTransport_PositiveTimeoutThreadsDeadline — [commit].timeout=90s threads a deadline (spy.HadDeadline()).
    - TestCommitTransport_ExplicitZeroTimeoutThreadsNoDeadline — [commit].timeout=0 threads NO deadline (honours "no deadline").
  - All four tests the task enumerated are present; the timeout structural test is preferred over a timing-based test (deterministic), as the task specified.
- Notes: Both required dimensions asserted — exact argv (per-verb override wins, falls to shared, zero-config floor) AND timeout threading (positive deadline, explicit-zero no-deadline). The explicit-zero no-deadline case exceeds the literal acceptance list and correctly covers the spec's most failure-prone timeout semantic. Not over-tested: black-box covers the real production thread end-to-end; white-box covers the deadline observability the FakeRunner cannot (it discards the context). The two layers are complementary, not redundant. The default-argv pins coordinate with Task 2-6 (claude -p --model sonnet) as the task required.

CODE QUALITY:
- Project conventions: Followed. External test packages (commit_test) for black-box; internal package (commit) only where white-box access to commitTransport is genuinely needed; t.Parallel() throughout; FakeRunner + RecordingPresenter idioms; exact-argv assertions; behaviour-level proofs (override wins, default not invoked). Seam discipline intact — transport built over the runner, config never imports ai (aitransport is the seam-clean home).
- SOLID principles: Good. Single construction expression centralized in aitransport.New; commitTransport retains only its site-specific deps short-circuit.
- Complexity: Low. commitTransport is a guard + one delegating call.
- Modern idioms: Yes. *time.Duration boundary distinguishes absent / explicit-0 / positive; direct assignment, no conversion.
- Readability: Good. WHY-comments state the contract (per-key chain, never-zero-by-omission) and match as-built.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
