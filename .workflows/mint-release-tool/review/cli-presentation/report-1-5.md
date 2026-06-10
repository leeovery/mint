TASK: cli-presentation-1-5 — Pretty presenter renders the minimal stage sequence via lipgloss (tick-c09dc8)

ACCEPTANCE CRITERIA:
- Satisfies Presenter, writes to injected out writer.
- Minimal sequence (top brand → `  ✓ {stage} {detail}` → bottom brand) in order.
- Colour-on emits ANSI around glyph/text with layout preserved; colour-downgrade emits no ANSI while ✓/🌿/indent survive.
- Styling exclusively via lipgloss, no NO_COLOR/TERM check in the pretty path.
- Brand leaf from payload, defaulting to 🌿; rendered top and bottom.
- Start-of-run renders engine-supplied info.Action, never a hardcoded "releasing".

STATUS: Complete

SPEC CONTEXT: "The Pretty Layer" (brand lines, glyphs, stage-line shape), "Library Selection" (lipgloss for all pretty styling, Bubble Tea banned), "Render-Mode Detection" (colour-incapable TTY stays pretty and leans on lipgloss auto-downgrade). Implementation matches on every point; no drift.

IMPLEMENTATION:
- Status: Implemented. go.mod:7 adds github.com/charmbracelet/lipgloss v1.1.0.
- Location: internal/presenter/pretty.go:178 (`var _ Presenter`), :228 constructor, :318 writef, :350 RunStarted, :460 StageSucceeded, :881/:899 RunFinished, injectable lipgloss.NewRenderer(out) :242 + WithProfile/SetColorProfile :194, leafOrDefault :339.
- Notes: File extended well beyond the 1-5 slice by later phases (spinner, gate, notes, version, init), but the 1-5 surface (brand lines, stage success, colour seam, payload-driven action/leaf) is intact and correct. StageStarted now the Phase-4 spinner form — 1-5 ACs do not constrain StageStarted, so expected evolution, not drift. Grep confirms the only NO_COLOR/TERM hit is the doc comment at :41 documenting the absence of sniffing.

TESTS: Adequate. All five task-specified tests exist in pretty_test.go, plus leaf-payload coverage:
- :76 minimal sequence (order + verbatim detail)
- :113 colour-on emits ANSI (ESC 0x1b, indent before styled glyph, padded name + detail)
- :142 colour-downgrade emits no ANSI (no 0x1b; ✓, 🌿, layout survive)
- :167 elapsed only on blocking stages (table-driven)
- :300 start line uses engine action (releasing/regenerating, table-driven)
- :328/:355 brand leaf from payload (verbatim; empty → 🌿)
- :27 runtime Presenter-interface proof
Tests assert through the public out buffer; no-colour assertions key on absence of 0x1b ESC byte; WithProfile forces determinism. Not over-tested — each maps to a distinct criterion.

CODE QUALITY:
- Project conventions: Followed (functional-options constructor, injectable renderer/writer/input seams, verbatim engine data, small focused helpers).
- SOLID principles: Good — single responsibility per method, DI via Presenter seam, colour-capability delegated to lipgloss.
- Complexity: Low.
- Modern idioms: Yes — functional options, compile-time interface assertion, %-*s width formatting.
- Readability: Good (doc comments verbose but accurate).
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
