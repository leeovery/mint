TASK: commit-command-4-4 — Add the `r` regenerate-with-context action (line-read + one-time injection)

ACCEPTANCE CRITERIA:
- The gate offers `r` alongside `y`/`n`/`e` on an interactive AI-path run.
- `r` prompts for a single free-text context line via presenter.AskLine; Enter submits; the engine never reads stdin directly.
- A non-empty line is injected one-time into the regeneration prompt (on top of any [commit].context).
- An empty line regenerates with no injected context (a plain re-roll); empty string is a legal AskLine return; no-context decision is engine-side.
- errors.Is(err, presenter.ErrInputClosed) from AskLine (EOF) aborts fail-loud — no regeneration, no commit; presenter renders nothing, engine surfaces it.
- The injected context is not persisted — not to [commit].context/config, and not carried into a subsequent `r`.
- Regeneration runs the engine's one retry (consumed) — not re-implemented.
- The regenerated message returns to the Continue? gate (shown, y/n/e/r still offered); `r` is not an accept.
- `r` is interactive-only — not reachable under -y/non-TTY.
- Regeneration-failure routing (4-5) is not implemented here.

STATUS: Complete

SPEC CONTEXT:
Spec "Interactive Review Gate → Choice mapping (`r`)": `r` re-runs the AI with a one-time context line — the "context injection" affordance from the user's original commit shell function. After `r`, mint prompts for a single free-text context line via the Presenter's line-read (Enter submits), injects it one-time into the regeneration prompt, and is NOT persisted; an empty line is a plain re-roll. Regeneration runs the engine's normal one retry; failure routes to the $EDITOR fallback (that routing is task 4-5). The one-time `r` line is distinct from the persisted [commit].context config knob (1-1). `r` is interactive-only (moot under -y/non-TTY).

IMPLEMENTATION:
- Status: Implemented (matches acceptance criteria and spec)
- Location:
  - internal/commit/run.go:578-616 — the ChoiceRegen branch in reviewLoop: AskLine read, error mapping (ErrNotInteractive wrapped no StageFailed; ErrInputClosed/other surfaced), regenerateMessage call, transport-failure → errRegenerateFallback sentinel (4-5 routing), success → body = regenerated and loop-back.
  - internal/commit/run.go:636-649 — commitReviewGate() declares y/n/e/r with Default ChoiceYes.
  - internal/commit/run.go:673-684 — regenerateMessage builds the same L3 Generator and calls GenerateWithContext(ctx, cfg, oneTimeContext).
  - internal/commit/run.go:73-77 — regenContextPrompt label.
  - internal/commit/generate.go:96-132 — Generate delegates to GenerateWithContext(cfg, ""); GenerateWithContext runs the identical L1→size-guard→resolve→compose→L2 path and layers injectOneTimeContext before composing.
  - internal/commit/prompt.go:88-142 — oneTimeContextHeader, injectContext (shared idiom), injectOneTimeContext (empty = no-op).
- Notes:
  - One-time line is a pure local string passed verbatim through regenerateMessage → GenerateWithContext; never written back to cfg. Persistence is structurally impossible (no write path), confirmed by the config.Load re-read assertion in the test.
  - Injection layers ON TOP of resolved instructions, so persisted [commit].context survives alongside the one-time block; the full-override prompt path (ResolveInstructions returns override) would also receive the one-time block, which is correct (one-time steer applies regardless of how instructions were resolved).
  - The 4-5 failure-routing sentinel (errRegenerateFallback) is already wired (run.go:348-350); not a 4-4 concern but does not harm this task's scope.

TESTS:
- Status: Adequate
- Coverage:
  - run_regen_test.go: gate offers r alongside y/n/e (TestRun_GateOffersRegenAlongsideYesNoEdit); AskLine prompt + event ordering (TestRun_RegenPromptsForContextLineViaAskLine); non-empty line injected one-time, absent from initial prompt (TestRun_RegenNonEmptyLine_InjectedOneTimeIntoPrompt); empty line = byte-identical re-roll (TestRun_RegenEmptyLine_NoInjectedContext); AskLine ErrInputClosed aborts fail-loud, errors.Is preserved, no regeneration/no commit, StageFailed emitted (TestRun_RegenAskLineInputClosed_AbortsFailLoudNoCommit); AskLine ErrNotInteractive aborts, no extra StageFailed (TestRun_RegenAskLineNotInteractive_AbortsNoExtraStageFailed); one-time not carried across re-rolls — r(A) r(B) y, second prompt has B not A (TestRun_RegenContextNotPersisted_SubsequentRegenDoesNotCarryPriorLine); not written to config, with config.Load disk re-read (TestRun_RegenContextNotWrittenToConfig); engine one retry consumed, exactly 2 transport calls (TestRun_RegenConsumesEngineOneRetry); regenerated returns to gate, exactly one commit carrying the regenerated body, no git add on the r iteration (TestRun_RegeneratedMessageReturnsToGate_NotAnAccept).
  - generate_test.go:902-960: unit-level GenerateWithContext — non-empty injects on top of persisted while default rules survive; empty equals plain Generate byte-for-byte.
  - presentertest/recording_inputs_test.go: AskLine records the prompt and pops NextLines FIFO; AskLineResult overrides and scripts errors.
- Notes:
  - Every acceptance criterion has a direct test. The ErrInputClosed fail-loud path asserts all three required outcomes (no regeneration, no commit, sentinel preserved) plus the StageFailed surfacing — well-balanced.
  - The two-layer split (integration via Run + unit via GenerateWithContext) is appropriate, not redundant: the integration tests prove the gate wiring/ordering/persistence, the unit tests pin the prompt-composition contract.
  - Interactive-only (moot under -y/non-TTY) is covered behaviourally by the ErrNotInteractive test rather than a separate "-y never reaches r" test; acceptable because the mootness is enforced by the presenter contract (Prompt auto-accepts under -y, returns ErrNotInteractive on non-TTY) and the AskLine ErrNotInteractive path is the reachable consequence. Not under-tested.
  - Not over-tested: no redundant assertions; transport-call-count assertions are load-bearing (they prove the retry is consumed, not re-run).

CODE QUALITY:
- Project conventions: Followed. Idiomatic Go; errors wrapped with %w preserving sentinels (ErrInputClosed/ErrNotInteractive/ai transport sentinels) for errors.Is; table-free focused tests with t.Parallel(); seam injection via Deps matches the engine's ReleaseDeps shape; presenter is the sole output seam (engine never touches stdin/stdout).
- SOLID principles: Good. Single path for generate (Generate delegates to GenerateWithContext); injectContext is a single shared idiom parameterised by header serving both persisted and one-time blocks; regenerateMessage and generateMessage both build over the same Generator with no duplication.
- Complexity: Low. The ChoiceRegen branch is a flat read → error-map → regenerate → error-map → adopt-and-loop sequence; no nesting beyond the necessary error checks.
- Modern idioms: Yes. errors.Is sentinel checks, %w wrapping, FIFO scripting in the recorder.
- Readability: Good. Doc comments are dense but precise and trace each branch to the spec rule it implements; the one-time-vs-persisted distinction is explicitly documented at every touch point.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
