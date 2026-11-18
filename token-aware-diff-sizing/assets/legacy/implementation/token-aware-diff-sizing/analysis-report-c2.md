# Analysis Report: Token Aware Diff Sizing (Cycle 2)

## Stats

- Total findings: 3
- Deduplicated findings: 3
- Proposed tasks: 2

## Summary

The three agents raised three separate findings that do not overlap, all graded low. The standards agent re-checked every behaviour decision against the specification and its corrigenda and found them all as built. Two findings become proposals. They survive the low-severity filter because each one fits a pattern this plan already built:
- `resetTo`'s restore step is the last printed paste-ready command that does not quote through `cmdline.Quote`. Phases 5 and 8 settled that rule for every printed step.
- The README leaves out four built behaviours on the notes-failure ladder and misstates a fifth. That falls under §18's rule that docs stay true to as-built, the same kind of gap phase 7's Task 2 closed.

The third finding asks to inject regenerate's chunk-notes store. It is discarded because it reverses a settled direction with no new grounds. Two stale comments in config.go are collected below for direct application.

## Comment Corrections

- internal/config/config.go:22-24 — the package doc lists Load's value checks and leaves out the malformed-ai_command rejection this feature added (`validateAICommands`, config.go:427, :842)
  OLD: // command string nor an array of command strings is rejected naming the hook key;
// max_prompt_bytes must be positive; and on_notes_failure is constrained to the closed
// set abort | fallback.
  NEW: // command string nor an array of command strings is rejected naming the hook key;
// max_prompt_bytes must be positive; an ai_command at any level must split into a
// program and its arguments; and on_notes_failure is constrained to the closed set
// abort | fallback.
- internal/config/config.go:447-449 — translateStrict's list of special-cased keys leaves out release.fallback, which unknownKeyMessage also special-cases (config.go:551-552)
  OLD: // table so the offender is unambiguous — and special-cases a TOP-LEVEL [hooks] table
// (the documented contradiction) with targeted guidance to nest under
// [release.hooks], and the removed top-level max_diff_lines with its replacement.
  NEW: // table so the offender is unambiguous — and special-cases a TOP-LEVEL [hooks] table
// (the documented contradiction) with targeted guidance to nest under
// [release.hooks], and the removed max_diff_lines and release.fallback with what
// replaced them.

## Discarded Findings

- Regenerate's chunk-notes wiring has no test (architecture, low). Discarded because it reverses a settled direction without grounds. Cycle 1's approved Task 1 settled, in both its Solution and its Do, that "`regenerate(...)` keeps its hard-wired chunk-notes store, since no test reaches it". The walk approved that clause with the release half. The finding's recommendation, passing the store to `regenerate(...)` as a parameter, is that clause reversed. It does not extend or complete it.
  - It offers no measurement: it agrees that no test reaches the field, which is the fact the direction was derived from.
  - It cites no spec entry and no project rule.
  - It shows no defect in the shipped tree, since both regenerate paths are wired with the store today (cmd/mint/main.go:193).
  - What it offers is a second reading of the same facts: the wiring-drop failure the release half was accepted for applies to regenerate too. Cycle 1's architecture finding already recommended "Give `regenerate(...)` the same store injection" beside that failure, and the walk settled against it.
