TASK: cli-presentation-2-5 — ShowNotes renders byte-identical body with per-mode delimiters

ACCEPTANCE CRITERIA:
- ShowNotes is on the Presenter interface and recorded by RecordingPresenter.
- Plain wraps body in `--- release notes v{X} ---` … `--- end notes ---`; pretty wraps in titled rule + closing rule (no box).
- Body bytes written by plain and pretty are identical (byte-for-byte), proven by extracting and comparing.
- Emoji headers (✨ Features, 🐛 Fixes) preserved verbatim in both modes — no stripping.
- A body line that looks like a delimiter is written verbatim, not treated as a real delimiter.
- A multi-line body with internal blank lines preserves those blank lines exactly.
- No truncation of the body in either mode.

STATUS: Complete

SPEC CONTEXT: spec:132 (Pretty "no box": titled rule, body verbatim, closing rule), 203 ("notes body byte-identical in both modes"), 210-211 (Plain delimiters; body verbatim; emoji shown; "non-negotiable"), 226. Body byte-identity is the "what previews is what ships" invariant; only delimiters differ.

IMPLEMENTATION:
- Status: Implemented (correct)
- Location: presenter.go:29-39 (shared writeNotesBody helper), :88-92 (interface), :271-303 (Notes struct); plain.go:306-310 (ShowNotes); pretty.go:645-650 (ShowNotes), :659-673 (notesTitledRule/notesClosingRule); presentertest/recording.go:88,164-166.
- Notes: Both presenters route body through single shared writeNotesBody with unchanged Notes.Body — the structural guarantee of byte-identity; only delimiter writes differ. No stripping/transform/indent/truncation. Empty body writes nothing between delimiters. Delimiters positional (not content-matched), so delimiter-like body lines are safe. Emoji survive (byte-purity guard scans synthesised narration, not engine body). Pretty rule width via ruleWidth(p.termWidth); default termWidth 0 maps to ruleCap, preserving 2-5's fixed-cap rendering.

TESTS:
- Status: Adequate
- Coverage: plain delimiters (plain_test.go:920); pretty titled/closing + no box (pretty_test.go:1179); byte-identical body across modes incl. unmutated vs source (pretty_test.go:1306); emoji preserved (plain_test.go:937, pretty_test.go:1202); delimiter-like body line verbatim (plain_test.go:969, pretty_test.go:1234); empty body bare delimiters (plain_test.go:953, pretty_test.go:1218); multi-line blank lines preserved (plain_test.go:991, pretty_test.go:1251); no truncation (pretty_width_test.go:142); interface/recorder round-trip (presenter_test.go:145); colour-downgrade + colour-on (pretty_test.go:1269/1288); golden transcript (golden_transcript_test.go:108).
- Notes: Test-side rule helpers re-derive from exported RuleCapForTest with same clamp, guarded by TestRuleCapForTestMirrorsProductionRuleCap. Not over-tested.

CODE QUALITY:
- Project conventions: Followed (verbatim-body discipline, payload-driven, documented discarded writes).
- SOLID principles: Good — single shared writeNotesBody is the one source of truth for body bytes (DRY + the mechanism enforcing the invariant).
- Complexity: Low.
- Modern idioms: Yes — io.WriteString, fmt.Fprintf, rune-count display width, strings.Repeat with min-1 clamp.
- Readability: Good — invariant documented at helper, interface, both implementations.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
