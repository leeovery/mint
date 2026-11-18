TASK: Reject a malformed `ai_command` at config load (token-aware-diff-sizing-1-3)

ACCEPTANCE CRITERIA:
- Given a `[release].ai_command` with an unterminated quote, such as `claude --system-prompt "Be terse`, each of `mint release`, `mint release regenerate <version>` and `mint commit` fails before any step runs with `invalid .mint.toml: release.ai_command has an unterminated quote`, and makes no AI call (§3.2)
- Given a shared `ai_command` that ends in a lone backslash, such as `claude -p \`, loading fails with `invalid .mint.toml: ai_command ends in a lone backslash` (§3.2)
- Given a `[commit].ai_command` of `'' --flag`, loading fails with `invalid .mint.toml: commit.ai_command names no program`; a non-blank value whose split yields no words at all (a lone backslash-newline) fails with `names no program` too (§3.2, Corrigendum 2026-10-19)
- The message names the level and cause for every combination: key `ai_command` / `release.ai_command` / `commit.ai_command`; cause `has an unterminated quote` (single or double) / `ends in a lone backslash` / `names no program` (§3.2)
- Given a malformed `[commit].ai_command` and nothing else wrong, `mint release` still fails at load with the `commit.ai_command` message (§3.2)
- Given a well-formed command whose empty argument is not its first word, such as `claude --tools ""`, config loads (§3.2)
- Given a blank or whitespace-only `ai_command` at any level, config loads and that level counts as unset: a blank `[commit].ai_command` under shared `llm -m gpt-4o` makes `mint commit` run `llm -m gpt-4o`; with every level blank or unset, the shipped default runs (§3.2, §2.4)

STATUS: issues_found

SPEC CONTEXT: §3.1 defines the POSIX-quoting split every `ai_command` goes through (top level, `[release]`, `[commit]`, shipped default). §3.2 rejects a command that cannot be split (unterminated quote, trailing lone backslash, empty first word) at config load, in the existing `invalid .mint.toml: <key> <cause>` shape, so it never surfaces at call time as an AI failure that reaches the classifier and the OTHER path. The 2026-10-19 corrigendum keeps a blank/whitespace-only value as unset (falls through `[verb] → shared → shipped default` per §2.4's reliance on as-built `AICommandFor`), and extends "names no program" to a non-blank value whose split yields no words.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/config/config.go:428 — `validateAICommands(shape)` runs in `Load` beside `validateHooks` (:422) and `validateOnNotesFailure` (:425), before `Config` is built
  - internal/config/config.go:844 — `validateAICommands` walks all three levels (`ai_command`, `release.ai_command`, `commit.ai_command`) regardless of verb, skips nil and `strings.TrimSpace`-blank values, and formats `invalid .mint.toml: <key> <cause>`
  - internal/config/config.go:865 — `aiCommandDefect` maps `cmdline.ErrUnterminatedQuote` → `has an unterminated quote`, `cmdline.ErrTrailingBackslash` → `ends in a lone backslash`, and zero words or an empty first word → `names no program`
  - internal/cmdline/split.go — `Split` (task 1-1) is the single source of the split rules; a backslash-newline outside quotes is dropped without opening a word, so `"\\\n"` yields no words as the corrigendum requires
  - Load sites that surface the error before any step: internal/engine/release.go:370 (release, `surface(p, "config", err)`), internal/commit/run.go:304 (commit, before `RunStarted`), cmd/mint/main.go:163 (regenerate, before version resolution, preflight or any AI call)
- Notes: Blank detection (`strings.TrimSpace`) and the splitter's word separator (`unicode.IsSpace`) agree, so a value skipped as blank at load is exactly one `AICommandFor` (config.go:583) skips at resolution, and every non-blank value `AICommandFor` can return has a non-empty first word. The `err != nil` fallback in `aiCommandDefect` is unreachable with the current `Split` but keeps the switch total. No drift from the plan or spec.

TESTS:
- Status: Adequate (one named arm unpinned — see FINDINGS)
- Coverage:
  - internal/config/ai_command_validation_test.go — `TestLoad_MalformedAICommand_RejectedNamingLevelAndCause` crosses all 3 levels with 6 values (unterminated double quote, unterminated single quote, trailing backslash, `'' --flag`, lone `""`, backslash-newline) and asserts the exact error string: covers AC2, AC3 and AC4's every-combination claim. A fixture guard test proves the TOML basic string decodes to backslash+newline. `TestLoad_WellFormedAICommandWithLaterEmptyArgument_Loads` covers AC6 at all three levels. Two blank-value tests cover AC7 at the accessor.
  - internal/engine/release_malformed_aicommand_test.go:17 — `mint release` with a malformed `release.ai_command` and with a malformed `commit.ai_command` (AC1 release arm, AC5): asserts the exact ✗ config message, only `git rev-parse --show-toplevel` ran, and no non-git command (no AI call).
  - internal/commit/run_malformed_aicommand_test.go:17 — `mint commit` with release/shared/commit-level malformed values through the real transport: exact error, one StageFailed, zero invocations (AC1 commit arm, AC2). `TestRun_BlankCommitAICommand_RunsSharedCommand` and `TestRun_EveryAICommandLevelBlank_RunsShippedDefault` prove AC7's two bullets end to end (the `llm -m gpt-4o` and lean `claude` invocations are asserted by argv).
  - Not covered: the `mint release regenerate <version>` arm of AC1 (see FINDINGS).
- Notes: Not over-tested — the 18-case matrix is what AC4's "every combination" asks for; the engine/commit tables share rows with other tasks' load rejections, which is consolidation, not redundancy.

CODE QUALITY:
- Project conventions: Followed — `invalid .mint.toml: …` shape matches sibling validators, lowercase messages, external test packages, `t.Parallel()`, `t.TempDir()`, exact-string assertions, FakeRunner/RecordingPresenter doubles
- SOLID principles: Good — the split rules live only in `internal/cmdline`; config maps its sentinels to load-error causes
- Complexity: Low
- Modern idioms: Yes (Go 1.22+ per-iteration loop vars in the nested `t.Run` closures)
- Readability: Good — comments on `validateAICommands` and `aiCommandDefect` hold against the code
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- [in-scope] [contained] cmd/mint/main.go:163 — no test drives `mint release regenerate <version>` against a malformed `ai_command`, though AC1 names it beside release and commit, and regenerate loads config at its own site (this one, rendering through `fmt.Fprintf(os.Stderr, …)`) rather than through `engine.Release`'s or `commit.Run`'s load. Add a cmd/mint test that calls `regenerate(t.Context(), rec, f, req)` (the seam at cmd/mint/main.go:149, already used by cmd/mint/regenerate_notesfile_test.go and cmd/mint/regenerate_floor_test.go) with a `.mint.toml` holding `[release]\nai_command = 'claude --system-prompt "Be terse'`, the FakeRunner seeded only for `git rev-parse --show-toplevel`, and assert a non-zero exit and that `git rev-parse --show-toplevel` is the only recorded invocation (no tag read, no AI call) — FAILS: a regression that moved regenerate's `config.Load` behind version resolution or the source/target prompts, or dropped its error check, leaves the suite green while AC1's regenerate arm ("fails before any step runs … and makes no AI call") is broken.

UNSETTLED:
- None
