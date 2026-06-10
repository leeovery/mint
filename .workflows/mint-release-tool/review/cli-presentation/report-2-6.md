TASK: cli-presentation-2-6 — Warn renders structured label + message in both modes and to stderr (tick-434222)

ACCEPTANCE CRITERIA:
- Warn is on the Presenter interface, carries structured Label+Message, recorded by RecordingPresenter.
- Plain renders {label}: WARN - {message}; pretty renders ⚠ {label}  {message} (amber under colour).
- The warning is written to both stdout narration and stderr in both modes.
- The presenter never parses a label out of a single string — Label and Message are separate inputs.
- Multiple warnings each render independently (no collapsing/de-dup), in order.
- A warning does not suppress the success end-of-run line.
- An empty message still renders the label-prefixed form without crashing or inventing content.

STATUS: Complete

SPEC CONTEXT: "Render-Mode Detection & Output Streams" (errors/warnings → stderr in addition to narration, spec:22,49); status glyph ⚠ amber (spec:128); worked failure example (spec:198); per-event table (spec:224). Warn decoupled from failure handling.

IMPLEMENTATION:
- Status: Implemented
- Location: presenter.go:75 (interface), :413-420 (Warning struct, separate Label/Message); plain.go:255-258 (Warn → out and err); pretty.go:556-571 (Warn + warnText, amber to out, unstyled copy to err); presentertest/recording.go:144-145,25,48.
- Notes: Label/Message written via %s as separate args — no parsing. Warn sets no failure state (terminalFailure untouched), so RunFinished still renders success line — independence is structural. Empty message handled: plain "x: WARN - " (documented trailing space), pretty "  ⚠ x" no trailing-whitespace. Asymmetry deliberate and documented. Pretty err copy unstyled (errf writes plain text), documented at pretty.go:541-555.

TESTS:
- Status: Adequate
- Coverage: interface/payload (presenter_test.go:104,118); plain to both streams (plain_test.go:402), empty message (:419), two warnings ordered both streams (:436), warn doesn't suppress done (:455), byte-pure (:471); pretty amber+ANSI (pretty_test.go:801), exact layout downgrade (:824), empty message (:843), stream split (:864), two warnings (:889), doesn't suppress brand line (:911).
- Notes: "never parses combined string" verified by feeding a Message containing ": " and asserting whole string lands after the prefix. Edge cases covered symmetrically in both modes. Not over-tested.

CODE QUALITY:
- Project conventions: Followed — doc-commented Warn, single-purpose warnText, lipgloss-only styling.
- SOLID principles: Good — additive method, small payload, isolated empty-message branch.
- Complexity: Low.
- Modern idioms: Yes — shared pure helper.
- Readability: Good — doc comments capture stream contract, failure-independence, empty-message decision.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
