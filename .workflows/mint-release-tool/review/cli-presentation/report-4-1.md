TASK: cli-presentation-4-1 — init renders created/skipped lines in the shared vocabulary (no gate, no brand footer) (tick-b948d8)

ACCEPTANCE CRITERIA:
- InitResult is on the Presenter interface, carries engine-resolved action + target + reason, recorded by RecordingPresenter.
- A created outcome renders ✓ created {target} (pretty) / {target}: created (plain).
- A skipped outcome renders · skipped {target} ({reason}) (pretty, middot) / {target}: skipped ({reason}) (plain), reason verbatim.
- init draws no interactive gate (no Prompt call, no review menu).
- init emits no release-style brand footer / done: line — run ends with the last outcome line.
- A --force overwrite is narrated as a created line (engine supplies InitCreated; presenter does not special-case --force).
- Mixed runs render created and skipped lines independently, in emit order.

STATUS: Complete

SPEC CONTEXT: "Cross-Verb Rendering" (spec:276-288) — init narrates in shared vocabulary, no gate (non-clobbering, spec:280, gate inventory spec:92); end-of-run (spec:287) — init's created/skipped lines are terminal, no footer. Event-payload principle: engine supplies created-vs-skipped and the --force reason.

IMPLEMENTATION:
- Status: Implemented
- Location: presenter.go:179 (InitResult on interface), :194-220 (InitAction enum + String()), :222-239 (InitOutcome), :470-474 (VerbInit no-footer arm); plain.go:389-395 (InitResult), :437-439 (RunFinished VerbInit no-op); pretty.go:815-845 (initSkipGlyph + InitResult), :890-892 (RunFinished no-op); presentertest/recording.go:33,64-65,91,212-217.
- Notes: Plain `{target}: created`/`{target}: skipped ({reason})`; pretty `✓ created {target}`/`· skipped {target} ({reason})` (middot U+00B7). Word order inverted per mode per spec. Reason verbatim via %s; created path never reads Reason. No --force knowledge: overwrite arrives as InitCreated, renders created line unconditionally. No gate (never calls Prompt). VerbInit/VerbVersion RunFinished arms render nothing (defensive). Suppression-precedes-shaping preserved.

TESTS:
- Status: Adequate
- Coverage (init_test.go): String() (:18), payload round-trip (:37); core created/skipped plain (:54,:69) pretty (:117,:133); out-only (:83), byte-purity (:102), colour-downgrade (:147), colour-on (:167,:190); all-created (:209), all-skipped (:229), mixed plain (:249) pretty (:265); --force as created both modes (:282); no footer/no gate plain (:306) pretty (:333); recorder payload (:356).
- Notes: Every AC + edge covered with exact-string assertions. Justified minor overlap. Behaviour-focused.

CODE QUALITY:
- Project conventions: Followed — typed enum + String() "unknown", engine-supplied rendering, glyph-only styling.
- SOLID principles: Good — focused single-responsibility method; engine/presenter split clean.
- Complexity: Low.
- Modern idioms: Yes — iota enum, exhaustive switch with documented no-op arm, table-driven tests.
- Readability: Good — doc comments justify middot/word-order/no-special-casing.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
