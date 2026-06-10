TASK: cli-presentation-2-4 — ShowPlan renders structured steps: pretty bulleted block, plain semicolon-joined one-liner (tick-278cf9)

ACCEPTANCE CRITERIA:
- ShowPlan is on the Presenter interface and recorded by RecordingPresenter with full structured payload.
- Plain renders one "plan: …" line with steps joined by "; " as "{verb} {target}".
- Pretty renders a "Plan" header plus one bulleted "• {verb}  {target}" line per step, verbs padded to a column.
- A single-step plan renders correctly in both modes (no dangling separator).
- An empty plan produces no dangling separators (plain) and no orphan bullet/header (pretty).
- A step with empty target renders just {verb} (no trailing space/separator) in both modes.

STATUS: Complete

SPEC CONTEXT: spec:61,71 — ShowPlan carries STRUCTURED steps (verb+target), not pre-formatted text; per-event table (spec:225); worked examples (spec:159-163 pretty, spec:236 plain). Both modes format from the same structured steps; abbreviations are engine-supplied targets, not a distinct terse field.

IMPLEMENTATION:
- Status: Implemented (no drift)
- Location: presenter.go:241-269 (PlanStep/Plan types; interface :87); plain.go:269-288 (ShowPlan + renderPlainStep); pretty.go:579-626 (ShowPlan + planVerbColumn/padVerb, planIndent); presentertest/recording.go:27,52-53,87,157-159.
- Notes: Plain: empty plan -> "plan:\n"; empty target -> verb only; steps joined "; ". Pretty: empty plan -> block omitted; column = longest verb + 2 (dynamic, matches worked example); empty target -> no trailing pad. Both consume same []PlanStep (no separate terse field). Narration -> out only. Worked-example byte alignment verified ("• commit   CHANGELOG.md").

TESTS:
- Status: Adequate
- Coverage: plain core/single-step/empty/empty-target/byte-purity (plain_test.go:797,815,834,851,893); pretty core block/alignment property/single-step/empty/empty-target/colour-downgrade/colour-on (pretty_test.go:616,640,671,691,704,753,777); recorder round-trip (presenter_test.go:486); same-payload-both-modes (:519); golden transcript (golden_transcript_test.go:77-99).
- Notes: Every AC incl. all three edge cases tested in both modes plus recorder. Behaviour-focused. Colour-on/off pair justified. Not over-tested.

CODE QUALITY:
- Project conventions: Followed — small focused helpers, fmt "%-*s" padding, consistent writef.
- SOLID principles: Good — additive interface change, payload-driven rendering.
- Complexity: Low.
- Modern idioms: Yes — strings.Join, fmt width verb, range.
- Readability: Good.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
