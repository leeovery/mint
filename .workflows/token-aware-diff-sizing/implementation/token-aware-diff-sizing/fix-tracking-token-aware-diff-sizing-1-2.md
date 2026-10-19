## Attempt 1

ISSUES:
- README.md:252-253. The AI Transport section's TOML example still offers `ai_command = 'claude -p --model opus'          # pin a different model`. Since the default changed, that line also drops all five lean flags. An operator who copies it to change model silently gets back Claude Code's ~106K-token system prompt, tool definitions and CLAUDE.md files. They only notice through cost or latency, or through CLAUDE.md conventions reappearing in their notes and commit messages. Under Phase 2's per-call cap it also turns into the invisible overhead that §9.7 describes. This contradicts the subsection directly below it, which says the lean form drops CLAUDE.md on purpose and that conventions belong in `context`/`prompt`. The README's lean documentation is also what §12.4/§18.2 point operators at when they write their own `ai_command`.
  FIX: Change the example's trailing comment to `# a different model, without the lean flags`. In `### The shipped default` (README.md:265-286), add one sentence after the code block: "An `ai_command` you set replaces this command whole, lean flags included: to change only the model, copy it and edit `--model`."
  ALTERNATIVE: Rewrite the opus example as the full lean command with `--model opus`. It is self-contained, but the TOML block gets an unwieldy ~200-character line. I recommend the sentence-plus-relabel approach.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/setupguide/setupguide.go:7-10. The package doc says every prose-authoring helper is config-free and config is imported in exactly one path, but `overview()` now reads `config.DefaultAICommand`. Delete the claim.
  OLD: // write site). It imports `config` in exactly ONE path — renderConfigReference()
// reads the config-metadata source of truth (config.MetadataRows()) to render
// the config-reference table; every prose-authoring helper stays config-free, so
// the SoT remains the single source of key metadata.
  NEW: // write site).
- /Users/leeovery/Code/mint/internal/engine/release_configconsolidation_test.go:7-9. The file header says this file covers the zero-config default, but the diff removed `TestRelease_AICommand_DefaultDrivesTransport` from this file; that coverage is now in lean_command_test.go.
  OLD: //   - ai_command rewiring: the documented top-level ai_command key MUST drive the
//     notes AI invocation (its name + args reach the runner). The pinned default
//     (config.DefaultAICommand) still applies on a zero-config run.
  NEW: //   - ai_command rewiring: the documented top-level ai_command key MUST drive the
//     notes AI invocation (its name + args reach the runner).

NOTES:
- The setup guide has the same trap as the README issue, outside this task's §18.1 scope. Procedure step 4 (internal/setupguide/setupguide.go:204-209) tells the agent to write a shared or per-verb `ai_command` for the model the user wants. An agent will likely write `claude -p --model opus` and lose the lean flags. §18.1 lists only the AI-transport paragraph, the diff_exclude advice, the PONR line and the config reference as edits, so I did not raise it as an issue. It may deserve a spec corrigendum, or a mention when Phase 5's floor-guidance work points operators at the lean command.
- internal/setupguide/setupguide.go:165: the rewrap left one emitted guide line at 107 characters ("as FATAL — reported at once, never retried. On failure, release follows [release].on_notes_failure (abort"). The surrounding prose wraps at about 80. This is cosmetic, and the paragraph's tail gets rewritten in task 1-4 and Phases 2/3/5 anyway.
- The engine helpers `invokedWith`/`stdinOf` match on space-joined command lines, so a word-split system prompt with quotes stripped would still match there. The new dedicated tests use exact `slices.Equal`, so this gap is covered for the lean command.
- Leaving internal/notes/prompt.go:49 alone is correct. It is a sample release-notes bullet, not a statement of the default.
- Nothing rode the dispatch beyond the enumerated inputs.
