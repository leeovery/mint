TASK: cli-presentation-3-1 — Gate model with declared choice set rendered/returned by Prompt

ACCEPTANCE CRITERIA:
- Gate carries an ordered []GateChoice (key + action label) and a Default key that is a member of the choice set.
- Prompt(gate Gate) (Choice, error) is on the Presenter interface and recorded by RecordingPresenter.
- NotesReviewGate() declares exactly y/n/e/r in that order with default y and the spec's action labels.
- ReuseConfirmGate() declares exactly y/n in that order with default y and no e/r.
- A gate can declare a non-y default; Gate.Has/Gate.Keys operate on the declared set, not a hardcoded list.
- No code in the presenter package hardcodes the y/n/e/r choice set — choices read from the gate value.

STATUS: Complete

SPEC CONTEXT: "Gating & -y Orthogonality" — one Prompt method renders every gate variant (4-choice notes-review, 2-choice reuse confirm), both default-yes, same "Continue?" vocabulary; gate described by the choices it offers; engine owns e/r re-entry; presenter render-only.

IMPLEMENTATION:
- Status: Implemented (with correct forward-layering from later tasks)
- Location: gate.go:9-27 (Choice type + constants), :52-57 (GateChoice), :80-106 (Gate struct), :112-130 (Has/Keys), :136-152 (NotesReviewGate y/n/e/r), :158-172 (ReuseConfirmGate y/n); presenter.go:105-139 (Prompt on interface, render-only doc); presentertest/recording.go:90,110-117,182-193 (capture + NextChoices/PromptResult/Default fallback).
- Notes: Field named "Question" not "Prompt" to avoid colliding with Presenter.Prompt (documented gate.go:71-74) — deliberate naming improvement, not drift. Subject/AcceptEcho/SourceGate/TargetGate/fail-loud constants belong to later tasks, correctly layered atop 3-1; do not weaken any 3-1 criterion. Default-is-member invariant documented (gate.go:103-106) but unenforced — acceptable, satisfied by construction.

TESTS:
- Status: Adequate
- Coverage (gate_test.go): NotesReviewGateDeclaresFourChoices (:16), ReuseConfirmGateDeclaresTwoChoices (:47), GateCanDeclareNonYesDefault (:67), HasRejectsChoiceOutsideDeclaredSet (:86), PromptIsOnInterfaceAndRecorderCapturesGate (:98), RecorderReturnsScriptedChoice (:130). Criterion 6 enforced structurally — grep finds y/n/e/r literal only in doc comment + example; plainKeyHint builds from g.Keys(); render-only no-subprocess guard at prompt_render_only_test.go:60.
- Notes: Line-read default tests (gate_test.go:149,165) belong to 3-3, correctly attributed. Behaviour-focused, not over-tested.

CODE QUALITY:
- Project conventions: Followed — typed Choice, presentertest subpackage, thorough doc comments.
- SOLID principles: Good — pure data model, render-only Prompt seam, clean recorder precedence chain.
- Complexity: Low.
- Modern idioms: Yes — typed Choice, explicit constants.
- Readability: Good — Question-vs-Prompt rename a clarity win.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES:
- [idea] internal/presenter/gate.go:103-106 — the documented "Default must be a member of Choices" invariant is unenforced. Consider a validation helper / constructor-time check, or leave as a by-construction contract. Not required by 3-1.
