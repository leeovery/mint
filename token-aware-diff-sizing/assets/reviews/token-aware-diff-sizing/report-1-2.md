TASK: Lean shipped default AI command (token-aware-diff-sizing-1-2)

ACCEPTANCE CRITERIA:
- Given a repo with no `.mint.toml`, when `mint release` writes notes, the program run is `claude` with exactly: `-p`, `--model`, `sonnet`, `--safe-mode`, `--tools`, "", `--strict-mcp-config`, `--no-session-persistence`, `--system-prompt`, `Follow the instructions you are given and output only what they ask for.` (one argument)
- Given a `.mint.toml` that sets no `ai_command` at any level, `mint release regenerate <version>` from a fresh source and `mint commit` run that same program and arguments
- Given an explicit `ai_command` (e.g. `[commit].ai_command = 'claude -p --model opus'`, shared `ai_command = 'llm -m gpt-4o'`), each verb that resolves to it runs exactly that command, with none of the lean flags added
- Given a claude too old for a lean flag (exits non-zero rejecting `--safe-mode`), mint runs no version probe and no other command form; every AI-tool invocation is the lean command; on `mint commit` the only invocations are the lean first attempt and the transport's one retry
- `mint setup`'s config reference shows the shared `ai_command` row defaulting to the lean command; its AI-transport paragraph names the lean command where it named `claude -p --model sonnet`
- The README shows the lean command as the shared `ai_command` default with no line still calling `claude -p --model sonnet` the shipped default; documents the lean command in full; says it drops CLAUDE.md files and Claude Code's own system prompt on purpose; says conventions belong in the `[release]` and `[commit]` `context` and `prompt` keys

STATUS: complete

SPEC CONTEXT: §2.1 fixes the lean command string and the purpose of each flag (no `--bare`: it breaks subscription login). §2.2: one command serves every AI call (single-pass, chunk, merge, classifier, exclusion check, commit), so the system prompt names no task. §2.3: dropping CLAUDE.md and Claude Code's system prompt is intended; conventions move to `context`/`prompt`. §2.4: applies only where resolution falls through to the shipped default; explicit `ai_command` at any level is untouched. §2.5: an old claude fails through the normal generation-failure path; no version probe, no fallback to the plain form. §18.1/§18.2: setup guide AI-transport paragraph and config reference (SoT-rendered), and README per-key table + "The AI Transport" section, document the lean default.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/config/config.go:89 — `DefaultAICommand` is the lean command, the single canonical source; comment at :72-88 states the flag rationale, the `sonnet` pin, the no-`--bare` reason and the quote-aware-split dependency, all true to the code
  - internal/config/config.go:583-606 — `AICommandFor` resolution `[verb] → shared → DefaultAICommand` unchanged, so an explicit value at any level is returned verbatim (§2.4)
  - internal/config/metadata.go:117 — shared `ai_command` SoT row's Default sourced from `DefaultAICommand`; internal/setupguide/setupguide.go:428-446 renders it verbatim into `mint setup`'s config reference
  - internal/setupguide/setupguide.go:161-165 — AI-transport paragraph interpolates `config.DefaultAICommand` and states what the lean form drops and where conventions belong
  - README.md:239 — shared engine keys table row shows the lean command as the default
  - README.md:299-324 — "The shipped default" subsection: full command, each flag explained, no-`--bare` reason, the on-purpose drop of CLAUDE.md and Claude Code's system prompt, conventions routed to `[release]`/`[commit]` `context`/`prompt`, and the old-claude behaviour (no probe, no fallback)
  - Production wiring: all three sites build through `aitransport.New` — internal/engine/release.go:1321, internal/engine/regenerate_fresh.go:152 (reached from cmd/mint/regenerate_all.go:117 for single-version and batch fresh sources), internal/commit/run.go:822 — so the default reaches release, regenerate's fresh source and commit with no per-site literal
- Notes: No leftover source/doc line presents the plain `claude -p --model sonnet` as the default. Remaining occurrences are CHANGELOG.md:102 (a historical release entry), README.md:342 (a quoting example, not described as a default) and internal/notes/prompt.go:49 (an altitude example bullet in the notes prompt) — none is a default claim. No version probe or plain-form fallback exists anywhere in the transport path.

TESTS:
- Status: Adequate
- Coverage:
  - AC1: internal/engine/lean_command_test.go:51 `TestRelease_ZeroConfig_RunsLeanClaudeCommand` — no `.mint.toml`, real transport via `newDeps`, asserts every `claude` invocation's argv equals the literal `leanClaudeArgs` (:17-22) and stdin is non-empty
  - AC2: internal/engine/lean_command_test.go:115 (regenerate fresh, `.mint.toml` with only `[release].context`, nil transport → production `aitransport.New`, the same call cmd/mint makes) and internal/commit/lean_command_test.go:44 (commit, `.mint.toml` with only `[commit].context`, exactly one lean invocation with stdin)
  - AC3: internal/engine/lean_command_test.go:73 (shared `llm -m gpt-4o` runs as `llm` with exactly `-m gpt-4o`; claude never invoked) and internal/commit/lean_command_test.go:67 (`[commit].ai_command = 'claude -p --model opus'` runs with exactly `-p --model opus`, no lean flags appended); regenerate's `[release]` override is covered at internal/engine/regenerate_fresh_aitransport_internal_test.go:24
  - AC4: internal/engine/lean_command_test.go:96 (every claude invocation across the failure, including the classifier call, is the lean argv — a `--version` probe or plain-form fallback would fail it) and internal/commit/lean_command_test.go:87 (exactly 2 claude invocations, both lean)
  - AC5: internal/setupguide/setupguide_test.go:375 pins the lean literal in the AI-transport paragraph and rejects the plain backticked form; config reference chain: internal/config/config_test.go:789 pins `DefaultAICommand` to the literal, internal/config/metadata_test.go:326 ties the SoT cell to the constant, internal/setupguide/setupguide_test.go:547 asserts every SoT row (including its Default) renders
  - AC6: internal/config/readme_ai_command_test.go:42 (table row), :52 (no "default" line carries the plain form), :67 (AI Transport section carries the full command, CLAUDE.md, system prompt, `[release]`, `[commit]`, `context`, `prompt`)
  - Pre-existing default-command assertions across engine and commit tests were moved to `leanClaudeArgs` rather than deleted, keeping their override-vs-default coverage
- Notes: Literal argv/command pins are written out per package by design so drift in the constant or in the split fails; not over-tested. The zero-config and config-without-`ai_command` cases for commit and regenerate overlap, but the criteria name both situations separately.

CODE QUALITY:
- Project conventions: Followed — single SoT constant, config-metadata row derived from it, transport built only via `aitransport.New`, tests use FakeRunner + RecordingPresenter, external test packages, `t.Parallel()`, exact-argv assertions
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes (`slices.Equal`, `t.Context()`)
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
