TASK: Quote-aware `ai_command` splitting (token-aware-diff-sizing-1-1, tick-242ea4)

ACCEPTANCE CRITERIA:
- Given `tool --system-prompt="Be terse" '$HOME' a\ b "" ''` at any level (shared, `[release]` for a forward release or regenerate's fresh source, `[commit]` for `mint commit`), the program is `tool` and its arguments are `--system-prompt=Be terse`, the literal `$HOME`, `a b`, and two empty arguments (§3.1)
- Given `tool 'a "b" \c $d'`, the tool receives the single argument `a "b" \c $d` (§3.1)
- Given `tool "a\"b" "c\\d" "e\$f" "g\x"`, the tool receives `a"b`, `c\d`, `e$f`, `g\x`; inside double quotes a backslash escapes `"`, `\`, `$`, backtick and is otherwise literal (§3.1)
- A command across two lines with a backslash ending the first line outside quotes (`claude -p \` / `--model opus`) yields `claude` with `-p`, `--model`, `opus` (§3.1)
- Given `tool $HOME *.go ~/x a|b c;d e&f <in >out`, `tool` receives exactly those eight words; no expansion, substitution, globbing, pipe, redirection, and no shell (§3.1)
- An `ai_command` with no quotes or backslashes (runs of spaces and tabs) runs exactly the program and arguments the whitespace split ran before (§3.3)
- The README states `ai_command`'s quoting rules where `ai_command` is documented (§18.2)

STATUS: complete

SPEC CONTEXT: §3.1 defines the POSIX-quoting split rules (unquoted whitespace separates; single quotes literal; double quotes with the four backslash escapes; outside-quote backslash literal; backslash-newline continuation; joined parts form one word; quoted empty is one empty argument; nothing else is shell syntax) and requires the program to run directly through the runner seam. §3.2 (task 1-3) rejects unsplittable commands at load with the same rules, so the splitter must be reachable from both `internal/ai` and `internal/config` without either importing the other. §3.3 says unquoted commands are unchanged and names the stale whitespace-split comments in transport.go, config.go and aitransport.go. §18.2 requires the README to document the rules where `ai_command` lives.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/cmdline/split.go:27-122 — `Split` plus the `splitter` state machine (`singleQuoted` :65, `doubleQuoted` :78, `escaped` :98, `endWord` :115); sentinels `ErrUnterminatedQuote` / `ErrTrailingBackslash` at :17/:20
  - internal/ai/transport.go:333-342 — `parseCommand` now delegates to `cmdline.Split`, wraps a split error with `%w`; internal/ai/transport.go:215-218 — `Generate` returns that plain error before any attempt
  - Stale-comment rewrites: internal/ai/transport.go:109-113 (Config.AICommand), :192-194 (Generate), :329-332 (parseCommand); internal/config/config.go:115-124 (AICommand field), :577-579 (AICommandFor); internal/aitransport/aitransport.go:28-31 (New)
  - README.md:239 (shared engine keys row links the quoting section) and README.md:326-348 ("Quoting in `ai_command`" under "The AI Transport")
- Notes: `internal/cmdline` imports only the standard library, so both `internal/ai` and `internal/config` (config.go:866) reach it without breaking seam #5. Splitting on `unicode.IsSpace` makes an unquoted, backslash-free valid-UTF-8 command split exactly as `strings.Fields` did (TOML guarantees valid UTF-8, so the `[]rune` conversion cannot alter a config-sourced value). Double-quoted backslash handling covers exactly `"`, `\`, `$`, backtick and newline; an escaped closing quote correctly leaves the quote open. The production runner execs `exec.CommandContext(ctx, name, args...)` (internal/runner/exec_runner.go:41), so no shell is involved. All three wiring sites (engine/release.go:1321, engine/regenerate_fresh.go:152, commit/run.go:822) go through `aitransport.New` and therefore `parseCommand`. The `grep` named in §3.3 returns no remaining whitespace-split claims in internal/ai, internal/config or internal/aitransport; the remaining "split on whitespace" comments are on `$EDITOR` handling (internal/commit/editor_open.go:66, internal/engine/editor.go), which is not `ai_command` and stays a whitespace split.

TESTS:
- Status: Adequate
- Coverage:
  - internal/cmdline/split_test.go:12-94 — table covers every criterion's exact input (AC1 :22, AC2 :27, AC3 :32 plus the backtick escape, AC4 :37, AC5 :57 plus a backtick word), plus backslash-newline inside a word, inside double quotes and inside single quotes (literal), escaped quotes outside quotes, quoted whitespace, and the lean default command
  - internal/cmdline/split_test.go:96-118 — AC6 compared directly against `strings.Fields` over runs of spaces/tabs, other `unicode.IsSpace` runes (incl. U+0085), empty and blank inputs
  - internal/cmdline/split_test.go:120-148 — the unsplittable inputs return the right sentinel and nil words
  - internal/ai/transport_test.go:182 (`TestTransport_Generate_SplitsAICommandByShellQuoting`) — the AC1 command reaches the runner as name + exact args; :210 (`TestTransport_Generate_UnsplittableAICommandRunsNothing`) — zero invocations and no AI sentinel on a split failure
  - internal/aitransport/aitransport_test.go:126 (`TestNew_SplitsQuotedCommandAtEveryLevel`) — AC1 through shared (both verbs), `[release]` and `[commit]`; regenerate's fresh source rides `VerbRelease`, whose wiring is pinned by the existing internal/engine/regenerate_fresh_aitransport_internal_test.go
- Notes: Each test would fail if the corresponding rule broke. No redundant or implementation-detail assertions.

CODE QUALITY:
- Project conventions: Followed (external test packages, table-driven, `t.Parallel()`, sentinels with `errors.Is`, `%w` wrapping with a lowercase message, runner seam untouched, comments rewritten in the same change)
- SOLID principles: Good — a small single-purpose package shared by the transport and config validation
- Complexity: Low — one rune-at-a-time state machine with one method per quoting context
- Modern idioms: Yes
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
