TASK: cli-presentation-3-5 — Skip gate under -y with rendered auto-accept echo in both modes

ACCEPTANCE CRITERIA:
- Under -y, Prompt does NOT render the menu/prompt and does NOT read from the input reader.
- Under -y, Prompt returns the gate's declared default (y).
- Plain emits 'notes: accepted (-y)' for the notes gate (and reuse confirm) to stdout.
- Pretty emits a concise accept line (e.g. '✓ notes  accepted (-y)') to stdout.
- The reuse confirm is auto-accepted under -y exactly like the notes gate (with its echo).
- The auto-accept echo is on stdout only (not stderr).

STATUS: Complete

SPEC CONTEXT: "Gating & -y Orthogonality" (spec:109) — -y skips the gate, menu not drawn. "The Presenter Seam" (spec:72) — auto-accept is a rendered event, not engine-printed. "The Plain Layer" (spec:212/247) — exact 'notes: accepted (-y)'. spec:114 — reuse confirm skipped under -y like notes gate.

IMPLEMENTATION:
- Status: Implemented
- Location: gate.go:80-106 (Gate + Subject/AcceptEcho), :136-172 (NotesReviewGate/ReuseConfirmGate set Subject="notes", AcceptEcho="accepted"); plain.go:42,100-103,345-358 (yes field, WithYes, Prompt yes-branch first emits "{Subject}: {AcceptEcho} (-y)", returns Default, no read/menu); pretty.go:270-273,745-757 (WithYes, yes-branch emits "  ✓ {Subject}  {AcceptEcho} (-y)"); wiring.go:64-75 (NewForStartup threads WithYes in both modes).
- Notes: yes-branch checked FIRST, before stdinInteractive/reader. Echo word and subject travel in gate payload (proven by source/target gates reusing mechanism). Threading via construction state keeps Prompt(Gate) seam stable.

TESTS:
- Status: Adequate
- Coverage (gate_skip_test.go): menu-not-drawn both modes; returns default without reading stdin (failingReader); plain echo exact "notes: accepted (-y)\n"; pretty exact "  ✓ notes  accepted (-y)\n"; reuse confirm auto-accepted; stdout only; plus byte-purity, colour-on glyph styling, not-hardcoded payload proof (source/github gate), constructor Subject assertions, interactive-path-unchanged regression guards; golden transcript (:118-138).
- Notes: failingReader proves stdin never read. not-hardcoded test is the standout. No over-testing.

CODE QUALITY:
- Project conventions: Followed — WithYes mirrors WithInteractiveStdin/WithSpinnerFactory; shared readChoice/parseChoice untouched.
- SOLID principles: Good — Gate pure data model; echo payload carried in data so one Prompt renders every variant (open/closed).
- Complexity: Low.
- Modern idioms: Yes — %s passthrough.
- Readability: Good.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
