TASK: cli-presentation-3-8 — Render-only Prompt contract: engine owns the e/r re-entry loop, linear re-render

ACCEPTANCE CRITERIA:
- Prompt returning e produces no presenter side effect — no $EDITOR/subprocess/generation call.
- Prompt returning r produces no presenter side effect — no claude/regeneration invocation.
- A simulated engine loop (ShowNotes -> Prompt, re-entering on e/r) renders linearly: append-only/cumulative across passes.
- The output contains no screen-clearing or alt-screen control sequences (no ESC[2J, no alt-screen, no cursor-home overwrite).
- The loop ends when Prompt returns y or n; e/r continue it.
- The render-only contract is documented on Prompt and guarded by a test (no editor/regeneration dependency in the prompt path).
- Pretty spinner stop/resume around $EDITOR is NOT implemented here (deferred to Phase 4).

STATUS: Complete

SPEC CONTEXT: "Gating & -y Orthogonality" (spec:107) — Prompt returns a single choice and is render-only; engine owns e/r work then re-calls ShowNotes + Prompt, looping until y/n; linear (scrolls, no clear/alt-screen). "Library Selection" (spec:273) — no Bubble Tea/alt-screen. "Spinner Lifecycle" (spec:267) — $EDITOR hand-off deferred to Phase 4.

IMPLEMENTATION:
- Status: Implemented (contract-and-test task plus doc comment and import guard)
- Location: presenter.go:105-139 (Prompt render-only doc); plain.go:345-358 / pretty.go:745-757 (Prompt via shared readChoice, no subprocess); prompt.go (parseChoice/readChoice pure); gate.go (ChoiceEdit/ChoiceRegen display strings only); prompt_render_only_test.go:60-64 + import_guard_helpers_test.go (assertImportsExclude via go/parser ImportsOnly).
- Notes: Grep confirms no non-test source imports os/exec or syscall; os.StartProcess absent. Bare "os" imports are stdio/TTY only. $EDITOR/claude references are comments/labels (invisible to ImportsOnly guard). SuspendSpinner/ResumeSpinner are pure engine-driven control signals that don't detect/invoke $EDITOR — no breach. No drift.

TESTS:
- Status: Adequate
- Coverage (prompt_render_only_test.go): EditHasNoPresenterSideEffect (:107), RegenHasNoPresenterSideEffect (:125), EngineLoopRendersLinearlyAcrossPasses (:214, exactly-once + ordering + no screen-control), EngineLoopEndsOnYes (:258), EngineLoopEndsOnNo (:277), PromptPathImportsNoSubprocessDependency (:60, scanned==0 defence).
- Notes: Screen-control guard runs pretty under TrueColor so lipgloss emits SGR while guard passes — proves guard rejects only clear/alt-screen/home, not all ESC. simulateEngineLoop constructs presenter ONCE (accumulates like a real run); maxPasses=16 guards non-termination. Not over-tested.

CODE QUALITY:
- Project conventions: Followed — import guard mirrors UI-library guard, reuses shared assertImportsExclude.
- SOLID principles: Good — render-only seam is clean DI boundary; shared parse/loop core.
- Complexity: Low.
- Modern idioms: Yes — go/parser ImportsOnly, termenv profile injection.
- Readability: Good.
- Issues: None material.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES:
- [idea] internal/presenter/prompt_render_only_test.go:22-25 — subprocessMarkers scans only os/exec and syscall; a hypothetical os.StartProcess (bare "os", legitimately imported) would slip past. Consider adding an AST-level guard flagging os.StartProcess call expressions to close the residual gap. Low value today (no such call exists).
