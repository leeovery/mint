TASK: cli-presentation-4-6 — Spinner stop/resume around the engine-driven $EDITOR hand-off (SuspendSpinner/ResumeSpinner hooks)

ACCEPTANCE CRITERIA:
- The presenter exposes engine-callable SuspendSpinner()/ResumeSpinner() hooks (on the interface; recorded/no-op in RecordingPresenter).
- In pretty, SuspendSpinner stops the active spinner (releasing the terminal) and ResumeSpinner restarts it on the same stage line.
- No frames emitted between suspend and resume (the editor session is animation-free).
- When no spinner is active, both hooks are safe no-ops.
- In plain, both hooks are no-ops (no output, no error).
- Repeated edit passes each stop/resume cleanly with no orphaned or duplicated spinner (one-at-a-time after N cycles).
- The presenter does NOT invoke $EDITOR — the hooks only suspend/resume the presenter's own animation; the engine owns the hand-off.

STATUS: Complete

SPEC CONTEXT: "Spinner Lifecycle (pretty only)" (spec:261-267) — one at a time; "$EDITOR takes over the terminal — the spinner is stopped before handing off, resumed after." "Library Selection" (spec:273) — no Bubble Tea/alt-screen. Phase-3 render-only contract: the engine owns the e/r loop and $EDITOR hand-off; presenter never detects/invokes the editor.

IMPLEMENTATION:
- Status: Implemented
- Location: presenter.go:140-162 (Suspend/ResumeSpinner on interface, documented); pretty.go:417-445 (hooks), :155-174 (activeSpinnerText/spinnerSuspended/suspendedText state), :394-401 (stopSpinner clears suspend state); plain.go:166-169 (no-ops); presentertest/recording.go:201-210 (records payload-less kinds), :31-32,60-63.
- Notes: SuspendSpinner Stop()s directly (not via stopSpinner, which clears the flag), remembers start text, nils handle, sets spinnerSuspended; true no-op when nil. ResumeSpinner recreates one spinner from suspendedText on same line, clears flag; no-op when nothing suspended. "Completes while suspended" handled: stopSpinner clears spinnerSuspended even when handle already nil, so a later Resume doesn't resurrect a completed/failed stage. No alt-screen.

TESTS:
- Status: Adequate
- Coverage (pretty_suspend_test.go + guards): suspend→resume same line; no-frames-during-window (snapshots tracker during window); no-active safe no-op; plain no-ops; repeated cycles (3, peak<=1, drained to 0); completes-while-suspended success AND failure (no resurrect); hooks emit no narration; interface has both hooks (nopPresenter + exhaustive call test presenter_test.go:30-56); "no editor invocation" via whole-package import guard (prompt_render_only_test.go:60-64 scans all non-test sources incl. pretty.go).
- Notes: Every AC has a behaviour-level test plus two extra completion-while-suspended edges. Spy tracker, no real goroutine. Not over-tested.

CODE QUALITY:
- Project conventions: Followed — external _test package, injected spy seam, plain no-ops + recorder record/no-op per pattern.
- SOLID principles: Good — single-responsibility control signals; Liskov holds (no-op-when-inactive contract).
- Complexity: Low.
- Modern idioms: Yes — guard-clause returns, interface-typed handle.
- Readability: Good — subtle flag-ordering point documented (pretty.go:387-393, 411-416).
- Issues: None blocking.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES:
- [quickfix] internal/presenter/pretty_suspend_test.go:170 — TestStageFailedWhileSuspendedClearsSuspendedState (and its success twin) asserts only spinner state, not the completion line. Optional cheap strengthening: assert "✗ notes" still emits after a suspended-then-failed sequence (the non-suspended ✗-line path is already covered elsewhere, so purely additive).
