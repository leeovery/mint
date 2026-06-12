TASK: commit-command-4-1 — Add the `e` edit action with loop-back to the gate

ACCEPTANCE CRITERIA:
- The gate offers `e` alongside `y`/`n` on an interactive AI-path run (ChoiceEdit added to the hand-built Gate literal).
- `e` opens the editor pre-filled with the current message (not an empty/template buffer).
- The editor is resolved via the consumed 3-1 ResolveEditor — no parallel resolver introduced.
- The $EDITOR hand-off is wrapped in presenter.SuspendSpinner()/ResumeSpinner() (safe no-ops in plain).
- A non-empty save loops back — engine re-calls ShowMessage (refreshed body) then Prompt; NOT save-as-accept (no staging, no commit, no push).
- Prompt renders the gate menu only, never the message (render-only contract).
- The edited message is used verbatim — no L2/AI call and no reprocessing.
- A multi-line edited body is preserved intact through the loop-back.
- The re-rendered gate still offers y/n/e/r; editing again pre-fills with the now-edited message.
- `e` is interactive-only — not reachable under `-y` or non-TTY.

STATUS: Complete

SPEC CONTEXT:
Interactive Review Gate -> Choice mapping (`e` / edit): open the editor (resolved via the shared chain) pre-filled with the current message; on save, RETURN to the Continue? gate with the edited message shown, used verbatim (no AI reprocessing). This is the cli-presentation loop-back contract — `e` re-renders the gate, it is NOT save-as-accept (only the Phase 3 fallback editor is save-as-accept). $EDITOR Fallback -> Editor resolution applies to EVERY editor mint opens (git's own order via `git var GIT_EDITOR`), so `e` consumes the same 3-1 resolver. Empty-save discard is 4-2; not-launchable graceful-degrade is 4-3; `r` is 4-4/4-5.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/commit/run.go:537-577 — the `case presenter.ChoiceEdit` branch in reviewLoop: opens OpenEditor pre-filled with the CURRENT body, adopts a non-empty save verbatim (`body = saved`, line 575), loops back. No staging/commit/push in this branch.
  - internal/commit/run.go:512-624 — reviewLoop is the engine-owned Continue? loop owning the ShowMessage->Prompt render each iteration (render-only contract; line 518 ShowMessage, line 520 Prompt).
  - internal/commit/run.go:636-649 — commitReviewGate() hand-built Gate literal declares ChoiceEdit (line 644) alongside y/n/r; presenter hardcodes no choice set.
  - internal/commit/editor_open.go:51-95 — OpenEditor consumes ResolveEditor (3-1, line 52) and brackets the hand-off with p.SuspendSpinner()/defer p.ResumeSpinner() (lines 76-77).
  - internal/commit/editor.go:54-70 — ResolveEditor (3-1) via `git var GIT_EDITOR`; the single resolver, reused (no parallel resolver introduced).
- Notes: The `e` branch sets `body = saved` directly — it never touches Generate/GenerateWithContext/ComposePrompt, so the edit is genuinely verbatim with no L2 reprocessing. The loop structure (the `for` in reviewLoop) lets `e` re-enter any number of times; only `y` exits to accept and `n` to abort. Run (run.go:342-367) takes the loop's finalBody to commitAccept, so the edited body is what is committed. No drift from the plan: the branch correctly scopes to the non-empty-save case (4-1), with empty-save discard (4-2) and not-launchable degrade (4-3) cohabiting the same branch as their own tasks.

TESTS:
- Status: Adequate
- Coverage (internal/commit/run_edit_test.go — every plan-named test present and behavioural):
  - TestRun_GateOffersEditAlongsideYesNo — asserts the captured gate's declared set via Has(ChoiceYes/No/Edit) (reads the literal, not a hardcoded set).
  - TestRun_EditOpensEditorPreFilledWithCurrentMessage — er.preFills[0] == the generated body (proves real message, not empty buffer).
  - TestRun_EditResolvesEditorVia31ResolveEditor — findEditorResolution asserts a `git var GIT_EDITOR` was recorded (3-1 path, no parallel resolver).
  - TestRun_EditHandoffBracketedBySuspendResume — asserts KindSuspendSpinner precedes KindResumeSpinner around the hand-off.
  - TestRun_EditNonEmptySaveLoopsBack_NotSaveAsAccept — full event-kind ordering (RunStarted, ShowMessage, Prompt, Suspend, Resume, ShowMessage, Prompt, RunFinished); first ShowMessage body = generated, second = edited; exactly 1 commit carrying the edit; zero `git add`.
  - TestRun_EditedMessageUsedVerbatim_NoAIReprocessing — transport.calls() == 1 (no AI re-call) and the commit body equals the edit byte-for-byte.
  - TestRun_MultiLineEditedBodyPreservedThroughLoopBack — multi-line subject+body preserved through the re-render and into the commit.
  - TestRun_ReRenderedGateStillOffersYesNoEdit — both Prompt gates captured; the second still offers y/n/e.
  - TestRun_EditingAgainPreFillsWithNowEditedMessage — e,e,y: preFills[0]=generated, preFills[1]=firstEdit; commit carries secondEdit.
- Notes: Tests assert behaviour through the recording presenter's event stream and the editorRunner's recorded git invocations / temp-file pre-fills — not implementation internals. The interactive-only criterion is verified at the presenter seam (plain.go Prompt returns ChoiceYes under -y / ErrNotInteractive on non-TTY) plus the gate_test.go vocabulary tests, so the ChoiceEdit branch is structurally unreachable in those modes; no commit-package test scripting an impossible state is needed (and none is added — correct). Not over-tested: each test pins a distinct criterion with minimal scripted setup; no redundant happy-path duplication.

CODE QUALITY:
- Project conventions: Followed. git_safe Mutator seam used for all mutations; read-only `git var GIT_EDITOR` on the plain Runner; the gate literal is hand-built (Subject "message") rather than reusing NotesReviewGate (which would mis-echo "notes:"); testify not over-used; table/behavioural test style consistent with the package.
- SOLID principles: Good. reviewLoop owns ordering and dispatch; OpenEditor owns the file-roundtrip; ResolveEditor owns resolution — clean single-responsibility seams. The `e` branch reuses OpenEditor and isEmptyEditorBuffer rather than re-deriving (DRY), and the editor resolver is genuinely the one 3-1 implementation.
- Complexity: Acceptable. reviewLoop's switch is a flat dispatch; the `e` branch is a short, well-commented sequence with a single defensive default.
- Modern idioms: Yes. errors.Is on sentinel chains, deferred ResumeSpinner so the bracket closes even on a launch error, raw-bytes stdin for the commit body.
- Readability: Good — exemplary doc comments that pin each branch to its spec contract and explicitly delineate 4-1 vs 4-2/4-3.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
