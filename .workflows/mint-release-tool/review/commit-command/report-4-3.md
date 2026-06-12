TASK: 4-3 — e graceful-degrade when no editor is launchable (commit-command)

ACCEPTANCE CRITERIA:
- A not-launchable signal from 3-1 under e triggers a warn + re-render (engine re-calls ShowMessage with the unedited body then Prompt with the same y/n/e/r gate), NOT fail-loud
- The unedited message is preserved verbatim through the re-render (e treated as a no-op)
- No editor is launched when the signal is not-launchable
- The behaviour is distinct from the 3-5 fallback fail-loud — e degrades gracefully because a message already exists
- The gate remains usable after the warn — y/n/e/r all still offered (a subsequent y commits the unchanged message)
- The not-launchable signal is consumed from 3-1, not re-derived
- e is interactive-only — this path is not reachable under -y/non-TTY

STATUS: Complete

SPEC CONTEXT:
Spec section "$EDITOR Fallback — Path Semantics → Editor resolution" states: when no editor in
git's chain resolves to a launchable program, behaviour depends on whether a message candidate
already exists. Fallback path (no message yet) → fail loud. e gate action (a message already
exists) → graceful degrade: warn the editor could not launch and re-render the Continue? gate
with the unedited message preserved (treat e as a no-op), consistent with "e is a refinement step
that can never produce an empty commit." This task is the graceful-degrade consumer of the same
3-1 not-launchable signal that 3-5 consumes for fail-loud on the fallback path.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/commit/run.go:537-577 (ChoiceEdit branch of reviewLoop), warn constants
  run.go:69-70, OpenEditor surfacing internal/commit/editor_open.go:51-95, ResolveEditor +
  ErrNoEditor sentinel internal/commit/editor.go:32-70.
- Notes: The ChoiceEdit branch calls OpenEditor (which consumes 3-1 ResolveEditor) and on
  err matches errors.Is(oerr, ErrNoEditor) || errors.Is(oerr, runner.ErrCommandNotFound) — the
  two not-launchable shapes (resolution failure and missing-binary-at-launch). On match it emits
  p.Warn(Warning{Label:"editor", Message:"could not launch editor, keeping the message"}) and
  `continue`s, leaving body unchanged; the loop top re-renders ShowMessage(unedited) → Prompt with
  the same hand-built commitReviewGate() (y/n/e/r). All seven acceptance criteria satisfied:
  the signal is consumed (not re-derived — no parallel launchability check); the warn-then-loop is
  distinct from runEditorFallback's errNoMessageSource fail-loud (the mirror consumer); no editor
  launches on the resolution-failure shape (OpenEditor returns before RunInteractive); the gate
  literal is unchanged so y/n/e/r remain offered; e-under-(-y/non-TTY) is unreachable because the
  presenter auto-accepts (ChoiceYes) under -y and returns ErrNotInteractive on non-TTY, both
  short-circuiting before ChoiceEdit (documented run.go:509-511, a presenter-contract property).
  No drift from plan.

TESTS:
- Status: Adequate
- Coverage: internal/commit/run_edit_nolaunch_test.go covers all six listed micro-acceptance
  tests across two not-launchable shapes:
  * TestRun_EditNotLaunchable_WarnsAndReRenders — exact event ordering RunStarted, ShowMessage,
    Prompt, Warn, ShowMessage, Prompt, RunFinished; asserts exactly 1 Warn and 0 StageFailed.
  * TestRun_EditNotLaunchable_PreservesUneditedMessageVerbatim — re-rendered ShowMessage body and
    committed stdin both byte-equal the original generated message.
  * TestRun_EditNotLaunchable_NoEditorLaunched — er.launches == 0 (resolution-failure shape).
  * TestRun_EditNotLaunchable_DistinctFromFallbackFailLoud — Run returns nil and failLoudMessage
    never surfaces (contrast with run_failloud_test.go's fallback fail-loud).
  * TestRun_EditNotLaunchable_GateRemainsUsable — re-rendered gate offers y/n/e and a subsequent y
    commits the unchanged message.
  * TestRun_EditMissingBinary_WarnsAndReRenders — the OTHER signal (ErrCommandNotFound at launch):
    editor resolves and launches once, then warn + re-render + commit, no fail-loud.
  Tests run through the real OpenEditor/ResolveEditor/RunInteractive path over a production-shaped
  editorRunner (records actual launches), and through git_safe to the commit sink — so "no editor
  launched" and "verbatim committed body" are genuinely exercised, not stubbed. The two
  not-launchable shapes (resolution-fail vs missing-binary) are correctly distinguished by launch
  count (0 vs 1).
- Notes: Not under-tested — both signal shapes, ordering, verbatim preservation, distinct-from-
  fail-loud, and gate usability are all covered. Not over-tested — each test targets a distinct
  acceptance criterion; the shared seedEditNoEditorThenAccept helper avoids setup duplication.
  The interactive-only criterion is not re-tested here (correctly — it is a presenter-contract
  property owned by 4-1 and the presenter package, not re-asserted per consumer).

CODE QUALITY:
- Project conventions: Followed. Idiomatic sentinel matching via errors.Is; warn strings hoisted to
  a named const block; reuses the presenter.Warn seam consistent with the engine's existing
  "could not launch editor" precedent (internal/engine/editor.go:114). No re-derivation of git's
  resolution chain (delegates to `git var GIT_EDITOR`).
- SOLID principles: Good. The not-launchable decision (fail-loud vs graceful-degrade) is the
  consumer's per 3-1's design; this consumer owns only its own branch and shares the OpenEditor
  roundtrip with the fallback path (single responsibility, no duplication).
- Complexity: Low. One added case-arm branch with a single errors.Is guard and a continue.
- Modern idioms: Yes — errors.Is sentinel matching, structured Warning value.
- Readability: Good. Intent is documented inline (the mirror relationship to 3-5's fail-loud is
  spelled out) without obscuring the code path.
- Issues: None.

BLOCKING ISSUES:
- None

NON-BLOCKING NOTES:
- None
