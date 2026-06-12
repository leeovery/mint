TASK: 3-1 — Resolve the editor via git's resolution order (commit.ResolveEditor)

ACCEPTANCE CRITERIA:
- GIT_EDITOR set resolves to it over core.editor, $VISUAL, and $EDITOR
- core.editor set (no GIT_EDITOR) resolves to it over $VISUAL and $EDITOR
- $VISUAL set (no GIT_EDITOR/core.editor) resolves to it over $EDITOR
- $EDITOR unset (and nothing higher) resolves to git's built-in default — NOT an error on a TTY
- When no candidate in the chain is launchable, the resolver returns a distinguished not-launchable signal (not a launch attempt, not a panic)
- The resolver does not open the editor, stage, or commit — resolution/launchability only
- All git interrogation goes through the consumed CommandRunner/fake

STATUS: Complete

SPEC CONTEXT:
Spec ($EDITOR Fallback — Path Semantics → Editor resolution, lines 121-125): mint must open whatever `git commit` would open, following git's OWN resolution order (GIT_EDITOR → core.editor → $VISUAL → $EDITOR → built-in default). An unset $EDITOR is not by itself an error on a TTY. When no editor in the chain resolves to a launchable program, downstream behaviour splits (fallback path fails loud; `e` gate action graceful-degrades) — but this task builds ONLY the resolver and its distinguished not-launchable signal, not the downstream routing. The task design note pins the git-faithful approach: delegate to `git var GIT_EDITOR` via the consumed CommandRunner rather than hand-roll the chain, and explicitly NOT reuse engine.ResolveEditor (whose $VISUAL→$EDITOR→vi order omits GIT_EDITOR/core.editor).

IMPLEMENTATION:
- Status: Implemented
- Location: internal/commit/editor.go:32 (ErrNoEditor sentinel), :54-70 (ResolveEditor)
- Notes:
  - Delegates to a single read-only `git var GIT_EDITOR` through the consumed runner.CommandRunner (editor.go:55). git applies the full precedence, so mint inherits git's chain verbatim — the most git-faithful realisation of every precedence acceptance criterion (GIT_EDITOR > core.editor > $VISUAL > $EDITOR > built-in default) in one call. Correct per the design note.
  - Two not-launchable paths both collapse to ErrNoEditor: a non-zero `git var` exit (wrapped with %w so errors.Is matches and the cause is preserved, editor.go:60), and a clean exit whose stdout trims to empty (editor.go:64-66). Both return an empty editor string — never a launch, never a panic. Matches the criterion.
  - Unset $EDITOR is correctly NON-erroring: git returns its built-in default on stdout, the resolver passes it through untouched. No special-casing in mint, which is exactly right.
  - Resolution-only: ResolveEditor neither launches (no RunInteractive), stages, nor commits. Launching is correctly deferred to OpenEditor (editor_open.go) — out of this task's scope (3-2+).
  - Correctly does NOT reuse engine.ResolveEditor (internal/engine/editor.go:45-53), whose VISUAL→EDITOR→vi precedence is wrong for the git-faithful contract. The package doc comment (editor.go:9-14) records why. No drift.
  - Return value is trimmed but otherwise as-is (may carry args like "code --wait"); splitting into argv is left to the launcher (OpenEditor uses strings.Fields). Correct separation.
  - Design-note nuance, NOT drift: the note frames "not launchable" as detected AT LAUNCH via errors.Is(err, runner.ErrCommandNotFound) from RunInteractive. As-built, the resolver detects it earlier at resolution time (failed/blank `git var`), and the launch-time ErrCommandNotFound check correctly lives in OpenEditor (editor_open.go:82). Both signals converge on the surfaced not-launchable error. This is consistent with the spec's contract ("git var GIT_EDITOR fails or yields nothing"); the note's launch-time wording is one mechanism, not the only one.

TESTS:
- Status: Adequate
- Location: internal/commit/editor_test.go
- Coverage (all six task test cases present, each as a focused test):
  - GIT_EDITOR wins → TestResolveEditor_GitEditorWins (:22) + asserts the single call was `git var GIT_EDITOR` (assertGitVarGitEditor, :187).
  - core.editor wins → TestResolveEditor_CoreEditorWins (:43).
  - $VISUAL wins, args preserved → TestResolveEditor_VisualWins (:62), uses "code --wait" to lock arg preservation.
  - unset $EDITOR → git default, not an error → TestResolveEditor_UnsetEditorFallsToGitDefault (:82).
  - no launchable editor → not-launchable signal → TestResolveEditor_NoLaunchableEditor (:102) asserts errors.Is(err, ErrNoEditor) and empty string; TestResolveEditor_BlankGitVarYieldsSentinel (:125) additionally covers the clean-exit-but-blank branch (newline-only and whitespace-only), which is the other not-launchable trigger.
  - does not launch/stage/commit → TestResolveEditor_DoesNotLaunchStageOrCommit (:158) asserts no `git add`, no `git commit`, and exactly one invocation.
- Tests assert behaviour (delegation, returned value, sentinel via errors.Is, no extra invocations) rather than implementation internals. Each precedence test scripts the `git var` stdout to the tier git would pick and asserts pass-through — the correct way to test delegation, since precedence is git's job not mint's.
- Honest delegation note: because FakeRunner.Seed keys on command NAME only ("git"), the four precedence tests are structurally identical (script a string, assert it returns). Their distinct value is documentation of the precedence contract, not four independent code paths — they all exercise the same happy-path line. This is acceptable (it pins the six named acceptance tests one-to-one) and not over-testing; the blank/empty and failure branches that DO differ are each separately covered.
- Not under-tested: the trim-before-empty-check edge (editor.go:63) is explicitly guarded by the "\n" and "   " sub-cases. The wrapped-cause path (%w) is verified via errors.Is in the failure test.

CODE QUALITY:
- Project conventions: Followed. Exported sentinel + errors.Is contract (golang-error-handling), single read-only runner call through the seam (golang-cli / dependency-inversion), table sub-tests with t.Parallel() and t.Helper() (golang-testing). Doc comments are thorough and explain WHY (the git-faithful delegation, the deliberate non-reuse of engine.ResolveEditor).
- SOLID principles: Good. Single responsibility (resolve only), depends on the runner.CommandRunner abstraction not a concrete runner.
- Complexity: Low. One git call, two guard branches, one return. Trivial cyclomatic complexity.
- Modern idioms: Yes. fmt.Errorf with %w, strings.TrimSpace, errors.Is-friendly sentinel.
- Readability: Good. Self-documenting; intent and rationale are explicit.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
