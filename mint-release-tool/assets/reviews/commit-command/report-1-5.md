TASK: 1-5 — Integrate the Continue? review gate (tick-fd3c5e)

ACCEPTANCE CRITERIA:
- Interactive run renders the message via ShowMessage (Title "commit message", body byte-verbatim) then the Continue? gate via Prompt — never ShowNotes
- Enter accepts (Enter => y) and the commit is created
- y accepts and the commit is created
- n aborts as a true no-op — nothing staged/committed, repo unchanged
- -y: engine still calls Prompt; presenter skips render/read internally, emits "message: accepted (-y)", returns Default (ChoiceYes), commit proceeds
- Non-TTY stdin without -y fails loud: Prompt returns ErrNotInteractive (errors.Is), engine maps to non-zero exit, no commit, no hang, no added message
- ErrInputClosed (EOF mid-input) handled via errors.Is, surfaced by the engine, no commit
- Gate is a hand-built Gate literal (Subject "message", AcceptEcho "accepted", Default ChoiceYes, spec action labels); NotesReviewGate()/ReuseConfirmGate() not reused
- Gate placed before the commit mutation (nothing mutated pre-accept)

STATUS: Complete

SPEC CONTEXT:
spec "Interactive Review Gate" / "Commit Flow / Lifecycle" / "Invariant — mutate nothing until accept". The gate is ON by default, AI-path-only, renders the minted message before it sticks. Choice mapping: y/accept -> stage(if -a/-A)+commit+push; n/abort -> true no-op (staging deferred to accept). -y skips the gate (presenter-internal auto-accept); the shared forbidden-combo rule (non-TTY stdin + no -y -> fail loud) applies. Phase 1 wires y/n + Enter=>y and the -y/non-TTY posture; e/r are Phase 4. The load-bearing invariant: mutate nothing until accept, so abort returns the user to their exact pre-mint state.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/commit/run.go:463-624 reviewLoop — owns the ShowMessage -> Prompt render and the gate-outcome translation
  - internal/commit/run.go:518 ShowMessage(Title "commit message", Body body) renders before the gate
  - internal/commit/run.go:520 p.Prompt(commitReviewGate()) presents the gate
  - internal/commit/run.go:533-536 ChoiceYes -> accept; ChoiceNo -> decline (true no-op)
  - internal/commit/run.go:521-530 Prompt-error branch: ErrNotInteractive wrapped with %w, NO StageFailed (presenter pre-rendered); every other error (ErrInputClosed) surfaced via StageFailed
  - internal/commit/run.go:626-649 commitReviewGate() — hand-built literal: Subject "message", AcceptEcho "accepted", Default ChoiceYes, action labels
  - internal/commit/run.go:342-367 Run wires reviewLoop before commitAccept; line 359 returns errGateAborted on decline (before any git add/commit)
  - internal/commit/run.go:85-93,106 errGateAborted/errEditorNoOp/errNoMessageSource sentinels (plain errors -> exitCode() fallthrough = exit 1)
  - cmd/mint/main.go:350-352 commit.Run err -> exitCode(err); cmd/mint/main.go:416-422 exitCode falls through to 1 for any non-AbortError
- Notes:
  - Correctly consumes the cli-presentation seam: gate rendering, line-read, -y echo, forbidden-combo are NOT re-implemented in commit. The -y echo "message: accepted (-y)" is produced by the presenter from the literal's Subject+AcceptEcho (internal/presenter/gate.go:92-101) — verified the seam reads Subject as the LHS and AcceptEcho as "accepted".
  - The literal now declares y/n/e/r (e added in 4-1, r in 4-4) — exceeds the strict Phase 1 y/n scope, but the task description explicitly anticipated and sanctioned this evolution ("e/r actions are Phase 4"); it is not drift.
  - Subject is deliberately "message" (NOT NotesReviewGate's "notes"); doc comment at run.go:626-635 explains the reuse-avoidance reason. Matches the acceptance criterion exactly.
  - Exit-code wiring matches the task's "Exit-code note": errGateAborted is a plain error, so cmd/mint's exitCode() (matches only *engine.AbortError) falls through to deterministic exit 1. No unexported engine.abort() dependency. Correct route.

TESTS:
- Status: Adequate
- Coverage (all seven planned tests present in internal/commit/run_test.go):
  - TestRun_GateEnterAccepts_CreatesCommit (355) — unscripted recorder returns gate.Default(ChoiceYes); asserts commit stdin == body verbatim
  - TestRun_GateYesAccepts_CreatesCommit (375) — NextChoices ChoiceYes; asserts commit created with verbatim body
  - TestRun_GateNoAborts_MutatesNothing (396) — NextChoices ChoiceNo; asserts zero commit invocations AND no StageFailed (clean decline)
  - TestRun_DashYAutoAccepts_CallsPromptAndCommits (423) — asserts KindPrompt IS recorded (skip is presenter-internal) AND commit created
  - TestRun_NonTTYWithoutDashY_FailsLoudNoCommit (446) — PromptResult -> ErrNotInteractive; asserts non-nil err, errors.Is(ErrNotInteractive) preserved, zero commits, NO StageFailed
  - TestRun_InputClosed_SurfacedNoCommit (477) — PromptResult -> ErrInputClosed; asserts errors.Is preserved, zero commits, StageFailed IS emitted (engine surfaces it)
  - TestRun_MessageThenGateThenCommit_Ordering (508) — asserts ShowMessage index < Prompt index, and a commit only via the gate path
  - Plus TestRun_GateLiteral_CommitSubjectAndChoices (540) — asserts Subject "message", AcceptEcho "accepted", Default ChoiceYes, and the exact y/n/e/r key order
- Harness: real Generator + real git.Mutator over a single FakeRunner with a scripted transport (newCommitDeps, run_test.go:61) — an end-to-end thread, not a mock-of-itself. The RecordingPresenter faithfully models the seam contract (recording.go:215-226): unscripted Prompt returns gate.Default, exactly mirroring the real presenter's Enter and -y behaviour; PromptResult overrides for error paths. This makes the Enter-accept and -y tests valid stand-ins for the real presenter outcomes.
- Not under-tested: every acceptance criterion has a direct assertion; both no-op paths assert zero commit invocations; the two error sentinels are distinguished by their differing StageFailed expectation (the spec's "presenter pre-rendered vs engine-surfaced" split).
- Not over-tested: assertions are focused; no redundant happy-path duplication; gate-literal shape is asserted once in its own test rather than repeated.

CODE QUALITY:
- Project conventions: Followed. Sentinels preallocated and lowercase/no-punctuation (golang-error-handling); wrapped with %w (run.go:527,590); matched via errors.Is throughout; the single-handling rule respected (surface-or-return, never both narrate-and-rewrap).
- SOLID: Good. reviewLoop owns gate orchestration only; rendering/input/-y-echo live behind the Presenter seam (dependency inversion via the interface in Deps). The Mutator/Transport/Runner seams keep the orchestrator decoupled from concretions.
- Complexity: Acceptable. reviewLoop's switch has grown to carry e/r (Phase 4), but each case is self-contained and the gate-integration core (y/n/Prompt-error) is a small, clear slice.
- Modern idioms: Yes. errors.Is, %w wrapping, table-free focused tests, t.Parallel().
- Readability: Good. Doc comments are unusually thorough and pin each branch to its spec clause and to the mirror/opposite consumer (e.g. the ErrNotInteractive-vs-ErrInputClosed split, the editorUnavailable lock-step).
- Issues: none blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] cmd/mint/main.go:350-352 — No process-boundary test drives runCommit through a gate-decline or non-TTY path to assert exitCode()==1. The exit mapping is exercised only indirectly (engine tests assert a non-nil error; exitCode()'s fallthrough is shared infra). A single cmd-level test asserting the commit-abort -> exit 1 contract would lock the wiring the task's "Exit-code note" calls out; decide whether the shared exitCode() coverage already suffices.
