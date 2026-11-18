TASK: cli-presentation-1-3 — Startup render-mode selection (TTY detection, no sniffing)

ACCEPTANCE CRITERIA:
- SelectMode(true, true) -> ModePlain (--plain on a TTY still selects plain).
- SelectMode(false, true) -> ModePretty; SelectMode(false, false) -> ModePlain.
- LANG/LC_ALL/TERM/CI/NO_COLOR set must not change selected mode for any (plainFlag, isTTY) combo.
- IsTerminal is the sole TTY signal and uses term.IsTerminal(int(f.Fd())); selection path consults no env var.
- Mode resolved at a single call site; no second TTY/flag check downstream.

STATUS: Complete

SPEC CONTEXT: Spec section "Render-Mode Detection & Output Streams" fixes precedence (--plain -> plain; else isatty(stdout) -> pretty; non-TTY -> plain), mandates term.IsTerminal(int(os.Stdout.Fd())), bans sniffing LANG/LC_*/TERM/CI/NO_COLOR, and requires TERM=dumb to still select pretty (colour downgrade is lipgloss's job, Task 1-5). Mode selected once at startup; nothing downstream re-checks.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/presenter/mode.go:12-21 — Mode type + ModePlain (iota zero) / ModePretty.
  - mode.go:45-53 — SelectMode pure core, exact precedence.
  - mode.go:62-64 — IsTerminal wraps term.IsTerminal(int(f.Fd())).
  - mode.go:70-72 — DetectMode startup wiring.
  - go.mod:9 — golang.org/x/term v0.44.0 present.
- Notes: No-sniffing ban honoured — env-var references (LANG/TERM/CI/NO_COLOR/Getenv) appear only in *_test.go asserting they are ignored, never in production. Explicit no-sniffing comments at mode.go:39-44, 55-61. Single decision point: DetectMode is the only production resolver; gating.go:35 reuses the SAME IsTerminal primitive for the independent stdin axis (not a second render-mode check); wiring.go:64-75 NewForStartup is the single construction site threading the chosen Mode downstream. Bonus String() (mode.go:24-33) for diagnostics — minor, in scope.

TESTS:
- Status: Adequate
- Coverage (internal/presenter/mode_test.go):
  - :13-56 TestSelectModeAppliesPrecedence — all four combos incl. (true,true)->plain, (false,true)->pretty, (false,false)->plain; table-driven, t.Parallel().
  - :62-89 TestSelectModeIgnoresEnvironment — env set, all four combos unchanged.
  - :94-104 TestIsTerminalOnNonTTY — /dev/null reports false (deterministic).
  - :108-121 TestDetectModeOnNonTTYSelectsPlain — end-to-end OS probe -> plain for both flag values.
  - :126-145 TestDetectModeIgnoresEnvironment — env set, DetectMode still plain.
- Notes: Every criterion has a direct assertion. Env tests correctly omit t.Parallel() (t.Setenv incompatible). Not over-tested. isTTY=true path intentionally exercised purely via SelectMode.

CODE QUALITY:
- Project conventions: Followed (table-driven named subtests, black-box package, /dev/null + t.Cleanup).
- SOLID principles: Good — pure core / OS edge / wiring cleanly separated.
- Complexity: Low.
- Modern idioms: Yes (iota const block, golang.org/x/term, Stringer).
- Readability: Good.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
