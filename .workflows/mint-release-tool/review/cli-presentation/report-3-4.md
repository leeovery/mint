TASK: cli-presentation-3-4 — Pretty Prompt vertical-menu rendering (options above question, [default] beside action, prompt last)

ACCEPTANCE CRITERIA:
- Pretty renders options above the question and the {Prompt} › line last, with a blank line between.
- The [default] marker appears beside the action whose key equals gate.Default (and only that one).
- A four-choice notes gate renders four option lines (y/n/e/r); a two-choice reuse confirm renders only y/n.
- After unrecognised input the full menu is redrawn before the next read — verified by counting renders.
- The menu is built from the gate's declared choices (no hardcoded list); reordering/changing the gate changes the rendered menu.
- Under colour downgrade, structure and [default]/› markers are present with no ANSI colour codes.

STATUS: Complete

SPEC CONTEXT: "The Pretty Layer" (Review gate — vertical menu): options indented above the question, [default] beside its action, "Continue? › " prompt last, no trailing newline. "Gating & -y Orthogonality" — one Prompt(gate) renders whatever set the gate declares. Linear (scrolls, no alt-screen); unrecognised re-prompts via full redraw. lipgloss with auto-downgrade.

IMPLEMENTATION:
- Status: Implemented
- Location: pretty.go:797-803 (renderGate), wired into Prompt :745-757 via shared readChoice loop; constants menuIndent/defaultMarker/promptMarker :687-700; defaultSuffix :808-813.
- Notes: renderGate iterates gate.Choices in declared order, blank line, then "  {Question} › " with NO trailing newline — matches worked example. Menu from g.Choices, no hardcoded list. [default] via defaultSuffix comparing Key to g.Default. Redraw-on-bad-input free (renderGate passed as render closure to readChoice). Only key and "› " dim-styled; structure survives downgrade. No NO_COLOR/TERM sniffing. Scope bounded: no -y skip (3-5), no forbidden-combo (3-6); plain Prompt remains terse form.

TESTS:
- Status: Adequate
- Coverage (pretty_gate_test.go): RendersOptionsAboveQuestion, DefaultMarkerOnDefaultLineOnly, NonYDefaultMarksDeclaredDefault, TwoChoiceRendersOnlyYN, RedrawnAfterBadInput (count-based), MenuBuiltFromDeclaredChoices, ColourDowngradePreservesStructure (no ESC), ColourOnEmitsANSIButKeepsStructure.
- Notes: Asserts rendered bytes. Index/order guards reflow; count guards redraw. Not over-tested.

CODE QUALITY:
- Project conventions: Followed — injectable lipgloss renderer, layout-vs-colour separation, shared core.
- SOLID principles: Good — renderGate render-only, loop/parse factored into shared free function.
- Complexity: Low.
- Modern idioms: Yes — range/fmt, typed Choice.
- Readability: Good — no-trailing-newline contract documented.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
