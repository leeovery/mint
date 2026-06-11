TASK: mint-release-tool-2-5 — Default notes prompt & Keep-a-Changelog emoji-skin format

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/notes/prompt.go: DefaultPrompt const (TL;DR, emoji KaC sections, omit empty, bold+describe, notable-item unit, ignore version bumps, no preamble, salience discipline, Deprecated/Security opportunistic-on-marker), ComposePrompt (pure join), ResolveInstructions + helpers (readPromptOverride/resolveContext/injectContext). Wired at single resolution point in generate.go + select.go. Precedence: prompt override wins, missing override is error, context string-or-file. No output parsing.

TESTS:
- Status: Adequate. Covers ordering (sentinel-based), "nothing else" residue-strip, every default-prompt rule (table), no-wrapper, default-verbatim, context-string/file inject (path not leaked), full override (default rules absent), override-still-flows, prompt-precedence-over-context, missing-prompt-file error. Behaviour-focused.

CODE QUALITY:
- Followed conventions (%w + contextual messages, errors.Is fs.ErrNotExist, small functions, doc comments). SOLID good — pure composer vs IO resolver. Low complexity, modern idioms, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/notes/prompt_test.go:151 — add a test asserting resolveContext's literal-string fall-back when [release].context names an existing directory (prompt.go:159-160 info.IsDir() branch is untested).
- [quickfix] internal/notes/prompt_test.go — add a test that a context value whose stat fails for a reason other than not-exist surfaces an error (prompt.go:157 branch); low-value; downgrade to skip if hard to provoke portably.
