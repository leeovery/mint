TASK: cli-presentation-3-6 — Forbidden-combination fail-loud (non-TTY stdin without -y) surfaced through Presenter and to stderr

ACCEPTANCE CRITERIA:
- When -y absent and stdin non-interactive, Prompt does NOT read stdin and does NOT draw the menu — fails immediately (no blocking).
- Pretty renders a styled ✗ failure line ('not a TTY — pass -y to run unattended'); plain renders a terse failure line — both to stdout.
- The one-line failure summary is also written to stderr in both modes.
- Prompt returns a non-nil error; the presenter sets no exit code.
- Render mode still chosen on stdout independently — non-TTY stdin + TTY stdout still renders failure in pretty.
- Precedence: -y -> auto-accept (no failure); -y absent + non-TTY stdin -> fail; -y absent + TTY stdin -> interactive.

STATUS: Complete

SPEC CONTEXT: "Gating & -y Orthogonality" — fail loud ("not a TTY — pass -y to run unattended") rather than block; surfaces through Presenter (styled pretty / terse plain, render mode from stdout independently) and the one-line summary also to stderr. Exit-code ownership stays with engine/main.

IMPLEMENTATION:
- Status: Implemented
- Location: prompt.go:29 (exported ErrNotInteractive sentinel); plain.go:345-358 (precedence yes->non-interactive->interactive), :371-375 (failNotInteractive → out+err, returns sentinel, no stdin read); pretty.go:745-757, :772-777 (styled ✗ to out, unstyled summary to err); gate.go:29-45 (gateFailLabel "gate", ASCII + em-dash messages); gating.go + wiring.go:64-75 (DetectStartupSignals resolves stdin axis independently; NewForStartup threads WithInteractiveStdin).
- Notes: Precedence exactly spec order. Fail path never constructs/reads bufferedReader so cannot block. "gate" label is the mechanism, not gate.Subject. Render mode from renderer bound to stdout — orthogonality preserved. Constructor default stdinInteractive=true (explicit in literals) keeps interactive tests green.

TESTS:
- Status: Adequate
- Coverage (gate_forbidden_test.go): no-read-on-fail both modes (failingReader, err!=nil, choice=="", reader untripped); plain exact "gate: FAILED - not a TTY; pass -y to run unattended\n"; pretty styled (ESC+glyph) + exact Ascii shape; stderr summary both modes; render-mode independence (TrueColor styles despite non-TTY stdin); errors.Is sentinel; precedence (yes bypasses, interactive keeps interactive, constructor default); byte-purity. Production seam wiring_test.go:489-530 drives real NewForStartup with /dev/null stdin end-to-end.
- Notes: Every AC mapped. Mode-invariant property asserted once via gateDrivers table; mode-specific rendering in dedicated cases. Not over-tested.

CODE QUALITY:
- Project conventions: Followed — sentinel lowercase no-trailing-punctuation matched via errors.Is; centralised write-error discard; constants avoid hardcoded literals.
- SOLID principles: Good — single construction seam owns axis threading; presenters share parseChoice/readChoice; gating axis separated from render-mode axis.
- Complexity: Low — failNotInteractive 3-line guard; Prompt flat 3-branch precedence.
- Modern idioms: Yes — errors.Is, exported sentinel, builder setters, table-driven mode-invariant tests.
- Readability: Good.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
