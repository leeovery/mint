TASK: ai-model-selection-2-2 — Make ai.Config.Timeout carry absent-vs-explicit-zero and apply the deadline conditionally

ACCEPTANCE CRITERIA:
- ai.Config.Timeout is a *time.Duration (nil = not threaded; &0 = explicit no-deadline; &positive = the deadline).
- defaultTimeout const deleted; literal 60 * time.Second no longer appears in internal/ai/transport.go.
- Explicit 0 (no deadline): transport runs on the parent context, does NOT call context.WithTimeout, no instant timeout.
- Positive value: transport uses context.WithTimeout(ctx, value); a genuine deadline fires as ErrTimeout.
- Parent-context cancellation on the no-deadline path propagates as context.Canceled unchanged (not swallowed, not mapped to a sentinel).
- No negative collapsed into the 0 no-deadline branch.
- "No deadline" reachable only via explicit &0 (mapped to nil internal deadline), never via nil/forgotten Config.Timeout; boundary→internal mapping is explicit (no pointer copy), pinned by a test, documented in the comment.
- Config.Timeout / NewTransport / Generate-attempt comments corrected to conditional-deadline / no-self-default as-built.
- Build/gofmt/vet/test-race/golangci-lint all pass.

STATUS: Complete

SPEC CONTEXT: specification.md "Resolution value semantics" mandates the config→ai.Config boundary preserve absent-vs-explicit-zero for timeout — the load-bearing invariant that "no deadline" be reachable ONLY by an operator's explicit 0, never by a wiring site omitting the field (a forgotten field running unbounded would invert "fail loud, never hang"). The transport must apply the deadline conditionally: 0 ⇒ skip context.WithTimeout (run on parent ctx); positive ⇒ WithTimeout; a residual negative must not collapse into the 0 no-deadline branch. The "Migration & mechanical carry-overs" section enumerates the four doc-comment corrections required in the same change.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/ai/transport.go — Config.Timeout *time.Duration (lines 64-67); Transport.deadline *time.Duration internal carrier (lines 77-81); NewTransport mapping (lines 105-122); attempt conditional (lines 191-204).
- Notes: Faithful to the task's prescribed mechanism. The boundary type is *time.Duration; the internal Transport.deadline is a SEPARATE carrier with INVERSE polarity (nil = no deadline), and NewTransport MAPS rather than copies the pointer (lines 116-120: only *cfg.Timeout != 0 yields a non-nil deadline). The explicit-&0 case correctly becomes deadline = nil → parent-context path. The nil-by-omission case is handled strictly via panic (lines 106-111) with a descriptive programmer-error message — a sanctioned choice in the task and idiomatic per golang-design-patterns ("Panic is for bugs, not expected errors"; production never reaches it because all sites source config.TimeoutFor). The negative case is NOT collapsed: any non-zero value (including a stray negative) maps to a per-attempt deadline (lines 117-120), so a negative fires immediately via WithTimeout rather than disabling the deadline. defaultTimeout const and the timeout <= 0 re-default block are deleted; grep confirms no "defaultTimeout" or "60 * time.Second" remains in internal/ai/. attempt (lines 191-204) runs on the parent ctx when t.deadline == nil and only calls context.WithTimeout when t.deadline != nil, with defer cancel(). classifyFatal (lines 210-229) is unchanged, so context.Canceled still propagates unchanged on both paths. All four WHY-comments (Config.Timeout, NewTransport, Transport.deadline, Generate/attempt) are corrected to the conditional/no-self-default as-built and accurately describe the inverse-polarity mapping.

TESTS:
- Status: Adequate
- Coverage: internal/ai/transport_test.go. Every acceptance criterion has a focused behaviour test:
  - Explicit-0 skips WithTimeout / no instant timeout — TestTransport_Generate_ExplicitZeroTimeoutSkipsWithTimeoutRunsOnParentContext (lines 363-400): real NewExecRunner + a sleep-0.2 script with Timeout &0; asserts the body returns (a 0-duration WithTimeout would have fired instantly). Strong end-to-end proof of the central behaviour.
  - Positive value fires as ErrTimeout — TestTransport_Generate_RealDeadlineKillIsNonRetriedTimeout (lines 319-361): real exec deadline kill with a tiny positive Timeout against sleep 5.
  - Parent-cancel on the no-deadline path propagates unchanged — TestTransport_Generate_NoDeadlinePathPropagatesParentCancellationUnchanged (lines 402-426): Timeout &0, seeded context.Canceled; asserts errors.Is(context.Canceled), no sentinel match, exactly one invocation (not retried).
  - Negative not collapsed into no-deadline — TestTransport_Generate_NegativeTimeoutIsNotNoDeadline (lines 428-454): Timeout &(-1s) against sleep 5 yields ErrTimeout, proving the WithTimeout path (not the unbounded parent-ctx path).
  - Nil is a wiring bug, not a silent no-deadline — TestNewTransport_NilTimeoutIsWiringBugNotSilentNoDeadline (lines 456-478): asserts NewTransport panics on nil Timeout, pinning the contract.
  - ptrTo helper added (lines 26-28) with a clear comment on why ptrTo(time.Duration(0)) is needed over ptrTo(0). All pre-existing content/retry/missing-tool/cancel tests were migrated to the *time.Duration field (generousTimeout via ptrTo).
- Notes: Not under-tested — each criterion maps to a dedicated test; the two real-deadline edge cases (zero-skip and negative-fires) use NewExecRunner so a genuine exec deadline, not an injected wrapper, is exercised, which is the only way to actually prove WithTimeout was/was not applied. Not over-tested — no redundant assertions; the no-retry invariant counts are reused only where they pin distinct routing. The explicit-0 and cancel-on-no-deadline tests share setup shape but assert different behaviours (body-returns vs cancel-propagation), so neither is redundant.

CODE QUALITY:
- Project conventions: Followed. Honours CLAUDE.md seams (subprocess via runner.CommandRunner, no os/exec; tests use FakeRunner/SeedSequence and the real NewExecRunner only for the genuine-deadline proofs). WHY-comments corrected true-to-as-built in the same change, per CLAUDE.md and the spec's same-change obligation. Sentinel/error idioms untouched. External _test package, t.Parallel() throughout, t.TempDir() roots.
- SOLID principles: Good. Single responsibility preserved — boundary type vs internal carrier are cleanly separated; the mapping concentrates the absent/zero/positive decision in one place (NewTransport).
- Complexity: Low. The conditional in attempt is a single nil-check; the mapping is a single non-zero branch.
- Modern idioms: Yes. *time.Duration boundary distinguishing absent from explicit-zero is the idiomatic make-illegal-states-unrepresentable approach; the panic is the idiomatic programmer-error guard for an unreachable-in-production invariant.
- Readability: Good. The inverse-polarity relationship between Config.Timeout and Transport.deadline is the one genuinely subtle point, and it is documented explicitly in three comments (Config.Timeout, Transport.deadline, NewTransport), so intent is clear.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
