TASK: cli-presentation-4-3 — version payload exception: plain bare value, pretty dressed (ShowVersion)

ACCEPTANCE CRITERIA:
- ShowVersion is on the Presenter interface and recorded by RecordingPresenter.
- Plain emits exactly the bare value plus a single trailing newline — no prefix, glyph, ANSI, or extra lines/whitespace.
- Pretty emits a dressed form (🌿 mint v{value}) with the value present; styling additive only.
- The plain output is suitable for $(mint version) — capturing yields exactly the value.
- version draws no gate and emits no release-style footer / done: line.
- The value is written to stdout only (absent from stderr).

STATUS: Complete

SPEC CONTEXT: "Cross-Verb Rendering" (spec:282) — version is "the one payload verb"; plain prints bare value for $(mint version), pretty may dress it, styling additive. Gate inventory (spec:93) — no gate. Output streams (spec:48) — value to stdout.

IMPLEMENTATION:
- Status: Implemented
- Location: presenter.go:104 (ShowVersion on interface), :317-324 (Version struct {Value, Leaf}); plain.go:322-324 (writef("%s\n", v.Value) — bare value + single newline, %s never interpreted as format); pretty.go:714-718 (leafOrDefault default 🌿, "{leaf} mint v{value}", dim lipgloss, "v" prefix pretty-only); presentertest/recording.go:171-173,29,56-57,89.
- Notes: Leaf reuses leafOrDefault (consistent with brand lines); 1-5 brand-leaf resolution (payload-carried) correctly followed. No footer/gate special-casing — version simply never triggers RunFinished/Prompt. No drift.

TESTS:
- Status: Adequate
- Coverage (version_test.go): plain bare value == "1.4.0\n" (:32); no-framing guard (no ESC/🌿/"version:"/leading "v", :47); $() command-substitution trim (:71); plain stdout-only (:86); pretty dressed "🌿 mint v1.4.0\n" (:103); colour-on ANSI + value survive (:117); colour-downgrade bare text (:137); payload leaf + fallback 🌿 (:154); pretty stdout-only (:173); no footer/gate plain+pretty (:194); no Prompt event, exactly [ShowVersion] (:223); recorder round-trip (:241).
- Notes: Every AC + edge covered incl. byte-exact plain contract. Behaviour-focused. Leading-"v" guard subsumed by exact-equality but documents shell hazard cheaply.

CODE QUALITY:
- Project conventions: Followed — reuses writef/leafOrDefault, %s avoids format-string injection.
- SOLID principles: Good — single responsibility; additive payload field.
- Complexity: Low.
- Modern idioms: Yes — lipgloss additive styling with downgrade.
- Readability: Good — "no extra bytes" contract called out at impl site.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
